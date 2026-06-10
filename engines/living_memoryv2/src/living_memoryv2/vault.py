from __future__ import annotations

from contextlib import contextmanager
from dataclasses import dataclass, field
import json
from pathlib import Path
import sqlite3
import time
from typing import Any, Iterator
import uuid

from .facts import FactLedger
from .graph import GraphStore
from .knowledge import KnowledgeGraph
from .schema import MemoryEnvelope
from .store import TemporalStore
from .task_state import TaskStateStore


@dataclass
class BoxInfo:
    id: str
    name: str
    scope: dict[str, Any]
    created_at: float
    closed_at: float | None
    state: str
    memory_ids: list[str] = field(default_factory=list)


class MemoryBox:
    def __init__(self, vault: "MemoryVault", info: BoxInfo) -> None:
        self.vault = vault
        self.info = info
        self.id = info.id
        self.name = info.name

    def remember(self, memory: MemoryEnvelope) -> str:
        scope = dict(self.info.scope)
        scope.update(memory.scope)
        scope["session_id"] = self.id
        scope["box_id"] = self.id
        data = memory.to_dict()
        data["scope"] = scope
        # Scope is part of the content hash; recompute it so the persisted
        # envelope stays self-consistent and is not flagged as tampered by
        # verify_integrity().
        data["content_hash"] = ""
        scoped = MemoryEnvelope.from_dict(data)
        return self.vault.remember(scoped, box_id=self.id)

    def remember_text(
        self,
        content: str,
        *,
        memory_type: str = "episodic",
        tags: list[str] | None = None,
        entities: list[str] | None = None,
        relations: list[dict[str, Any]] | None = None,
        importance: float = 0.5,
        confidence: float = 1.0,
        provenance: dict[str, Any] | None = None,
    ) -> str:
        return self.remember(
            MemoryEnvelope.text(
                content,
                scope=self.info.scope,
                memory_type=memory_type,
                tags=tags or [],
                entities=entities or [],
                relations=relations or [],
                importance=importance,
                confidence=confidence,
                provenance=provenance or {"source": "memory_box"},
            )
        )

    def close(self) -> None:
        self.vault.close_box(self.id)

    def __enter__(self) -> "MemoryBox":
        return self

    def __exit__(self, exc_type, exc, tb) -> None:
        self.close()


class MemoryVault:
    def __init__(self, root: str | Path) -> None:
        self.root = Path(root)
        self.root.mkdir(parents=True, exist_ok=True)
        self.store = TemporalStore(self.root / "store")
        self.graph = GraphStore(self.root / "graph.sqlite3")
        self.knowledge = KnowledgeGraph(self.root / "knowledge.sqlite3", self.graph)
        self.facts = FactLedger(self.root / "facts.sqlite3", self.graph, knowledge=self.knowledge)
        self.task_states = TaskStateStore(self.root / "task_state.sqlite3")
        self.db_path = self.root / "vault.sqlite3"
        self._init_db()

    @contextmanager
    def _connection(self) -> Iterator[sqlite3.Connection]:
        conn = sqlite3.connect(self.db_path)
        conn.row_factory = sqlite3.Row
        try:
            yield conn
            conn.commit()
        finally:
            conn.close()

    def _init_db(self) -> None:
        with self._connection() as conn:
            conn.execute(
                """
                CREATE TABLE IF NOT EXISTS boxes (
                    id TEXT PRIMARY KEY,
                    name TEXT NOT NULL,
                    scope_json TEXT NOT NULL,
                    created_at REAL NOT NULL,
                    closed_at REAL,
                    state TEXT NOT NULL
                )
                """
            )
            conn.execute(
                """
                CREATE TABLE IF NOT EXISTS box_memories (
                    box_id TEXT NOT NULL,
                    memory_id TEXT NOT NULL,
                    sequence INTEGER NOT NULL,
                    PRIMARY KEY (box_id, memory_id)
                )
                """
            )

    def open_box(self, name: str, *, scope: dict[str, Any] | None = None) -> MemoryBox:
        box_id = "box_" + uuid.uuid4().hex[:20]
        info = BoxInfo(
            id=box_id,
            name=name,
            scope=dict(scope or {}),
            created_at=time.time(),
            closed_at=None,
            state="open",
        )
        with self._connection() as conn:
            conn.execute(
                "INSERT INTO boxes (id, name, scope_json, created_at, closed_at, state) VALUES (?, ?, ?, ?, ?, ?)",
                (info.id, info.name, json.dumps(info.scope, sort_keys=True), info.created_at, None, info.state),
            )
        return MemoryBox(self, info)

    def close_box(self, box_id: str) -> None:
        with self._connection() as conn:
            conn.execute(
                "UPDATE boxes SET state = ?, closed_at = ? WHERE id = ?",
                ("closed", time.time(), box_id),
            )

    def remember(
        self,
        memory: MemoryEnvelope,
        *,
        box_id: str | None = None,
        append_log: bool = True,
        extract_facts: bool = False,
    ) -> str:
        memory_id = self.store.remember(memory, append_log=append_log)
        self.graph.index_memory(memory)
        self.knowledge.index_memory(memory)
        if extract_facts:
            self._extract_facts(memory)
        if box_id is not None:
            with self._connection() as conn:
                row = conn.execute(
                    "SELECT COALESCE(MAX(sequence), 0) AS seq FROM box_memories WHERE box_id = ?",
                    (box_id,),
                ).fetchone()
                conn.execute(
                    "INSERT OR IGNORE INTO box_memories (box_id, memory_id, sequence) VALUES (?, ?, ?)",
                    (box_id, memory_id, int(row["seq"]) + 1),
                )
        return memory_id

    def get_box(self, box_id: str) -> BoxInfo:
        with self._connection() as conn:
            row = conn.execute("SELECT * FROM boxes WHERE id = ?", (box_id,)).fetchone()
            if row is None:
                raise KeyError(box_id)
            memory_rows = conn.execute(
                "SELECT memory_id FROM box_memories WHERE box_id = ? ORDER BY sequence",
                (box_id,),
            ).fetchall()
        return BoxInfo(
            id=row["id"],
            name=row["name"],
            scope=json.loads(row["scope_json"]),
            created_at=row["created_at"],
            closed_at=row["closed_at"],
            state=row["state"],
            memory_ids=[item["memory_id"] for item in memory_rows],
        )

    def memories_in_box(self, box_id: str) -> list[MemoryEnvelope]:
        return [self.store.inspect(memory_id) for memory_id in self.get_box(box_id).memory_ids]

    def verify_integrity(self) -> dict[str, Any]:
        report: dict[str, Any] = {"checked": 0, "tampered": [], "invalid_lines": []}
        if not self.store.jsonl_path.exists():
            return report
        for line_number, line in enumerate(self.store.jsonl_path.read_text(encoding="utf-8").splitlines(), start=1):
            if not line.strip():
                continue
            try:
                memory = MemoryEnvelope.from_dict(json.loads(line))
            except Exception:
                report["invalid_lines"].append(line_number)
                continue
            report["checked"] += 1
            try:
                resolved = self.store.resolve_blobs(memory)
            except Exception:
                report["tampered"].append(memory.id)
                continue
            if resolved.compute_content_hash() != resolved.content_hash:
                report["tampered"].append(memory.id)
        return report

    def rebuild_indexes(self) -> None:
        self.store.clear_indexes()
        self.graph.clear()
        self.knowledge.clear()
        if not self.store.jsonl_path.exists():
            return
        for line in self.store.jsonl_path.read_text(encoding="utf-8").splitlines():
            if not line.strip():
                continue
            memory = MemoryEnvelope.from_dict(json.loads(line))
            self.remember(memory, append_log=False)
        self.facts.rebuild_graph_index()
        for fact in self.facts.list_facts(limit=None):
            self.knowledge.index_fact(fact)

    def _extract_facts(self, memory: MemoryEnvelope) -> None:
        from .extraction import RuleBasedFactExtractor

        for fact in RuleBasedFactExtractor().extract(memory):
            self.facts.add_fact(
                subject=fact.subject,
                predicate=fact.predicate,
                object=fact.object,
                source_memory_id=fact.source_memory_id,
                scope=fact.scope,
                confidence=fact.confidence,
                valid_from=fact.valid_from,
                valid_to=fact.valid_to,
                reason=fact.reason,
            )
