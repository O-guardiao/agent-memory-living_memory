from __future__ import annotations

from contextlib import contextmanager
from dataclasses import dataclass, field
import json
from pathlib import Path
import sqlite3
import time
from typing import Any, Iterator

from .graph import GraphStore
from .schema import MemoryEnvelope, sha256_hex, stable_json


@dataclass(frozen=True)
class KnowledgeEntity:
    entity_id: str
    name: str
    entity_type: str = "entity"
    aliases: list[str] = field(default_factory=list)
    scope: dict[str, Any] = field(default_factory=dict)
    source_memory_ids: list[str] = field(default_factory=list)
    confidence: float = 1.0
    status: str = "active"
    created_at: float = 0.0
    updated_at: float = 0.0

    def to_dict(self) -> dict[str, Any]:
        return {
            "entity_id": self.entity_id,
            "name": self.name,
            "entity_type": self.entity_type,
            "aliases": list(self.aliases),
            "scope": dict(self.scope),
            "source_memory_ids": list(self.source_memory_ids),
            "confidence": self.confidence,
            "status": self.status,
            "created_at": self.created_at,
            "updated_at": self.updated_at,
        }


@dataclass(frozen=True)
class KnowledgeAssertion:
    assertion_id: str
    subject: str
    predicate: str
    object: str
    source_memory_id: str
    subject_entity_id: str
    object_entity_id: str | None = None
    object_kind: str = "entity"
    fact_id: str | None = None
    scope: dict[str, Any] = field(default_factory=dict)
    graph_name: str = "knowledge"
    valid_from: float | None = None
    valid_to: float | None = None
    observed_at: float = 0.0
    confidence: float = 1.0
    status: str = "active"
    reason: str = ""
    supersedes: list[str] = field(default_factory=list)
    superseded_by: str | None = None

    def to_dict(self) -> dict[str, Any]:
        return {
            "assertion_id": self.assertion_id,
            "subject": self.subject,
            "predicate": self.predicate,
            "object": self.object,
            "source_memory_id": self.source_memory_id,
            "subject_entity_id": self.subject_entity_id,
            "object_entity_id": self.object_entity_id,
            "object_kind": self.object_kind,
            "fact_id": self.fact_id,
            "scope": dict(self.scope),
            "graph_name": self.graph_name,
            "valid_from": self.valid_from,
            "valid_to": self.valid_to,
            "observed_at": self.observed_at,
            "confidence": self.confidence,
            "status": self.status,
            "reason": self.reason,
            "supersedes": list(self.supersedes),
            "superseded_by": self.superseded_by,
        }


class KnowledgeGraph:
    """Local-first temporal knowledge graph over canonical memories and facts."""

    FORMAT = "living-memoryv2/knowledge-context-1"

    def __init__(self, db_path: str | Path, graph: GraphStore | None = None) -> None:
        self.db_path = Path(db_path)
        self.db_path.parent.mkdir(parents=True, exist_ok=True)
        self.graph = graph
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
                CREATE TABLE IF NOT EXISTS kg_entities (
                    entity_id TEXT PRIMARY KEY,
                    name TEXT NOT NULL,
                    name_norm TEXT NOT NULL,
                    entity_type TEXT NOT NULL,
                    aliases_json TEXT NOT NULL,
                    scope_json TEXT NOT NULL,
                    source_memory_ids_json TEXT NOT NULL,
                    confidence REAL NOT NULL,
                    status TEXT NOT NULL,
                    created_at REAL NOT NULL,
                    updated_at REAL NOT NULL
                )
                """
            )
            conn.execute(
                """
                CREATE TABLE IF NOT EXISTS kg_assertions (
                    assertion_id TEXT PRIMARY KEY,
                    subject TEXT NOT NULL,
                    predicate TEXT NOT NULL,
                    object TEXT NOT NULL,
                    subject_norm TEXT NOT NULL,
                    predicate_norm TEXT NOT NULL,
                    object_norm TEXT NOT NULL,
                    source_memory_id TEXT NOT NULL,
                    subject_entity_id TEXT NOT NULL,
                    object_entity_id TEXT,
                    object_kind TEXT NOT NULL,
                    fact_id TEXT,
                    scope_json TEXT NOT NULL,
                    graph_name TEXT NOT NULL,
                    valid_from REAL,
                    valid_to REAL,
                    observed_at REAL NOT NULL,
                    confidence REAL NOT NULL,
                    status TEXT NOT NULL,
                    reason TEXT NOT NULL,
                    supersedes_json TEXT NOT NULL,
                    superseded_by TEXT
                )
                """
            )
            conn.execute(
                """
                CREATE TABLE IF NOT EXISTS kg_conflicts (
                    superseding_assertion_id TEXT NOT NULL,
                    superseded_assertion_id TEXT NOT NULL,
                    reason TEXT NOT NULL,
                    created_at REAL NOT NULL,
                    PRIMARY KEY (superseding_assertion_id, superseded_assertion_id)
                )
                """
            )
            conn.execute("CREATE INDEX IF NOT EXISTS idx_kg_entities_name ON kg_entities(name_norm)")
            conn.execute("CREATE INDEX IF NOT EXISTS idx_kg_assertions_spo ON kg_assertions(subject_norm, predicate_norm, object_norm)")
            conn.execute("CREATE INDEX IF NOT EXISTS idx_kg_assertions_memory ON kg_assertions(source_memory_id)")
            conn.execute("CREATE INDEX IF NOT EXISTS idx_kg_assertions_fact ON kg_assertions(fact_id)")
            conn.execute("CREATE INDEX IF NOT EXISTS idx_kg_assertions_status ON kg_assertions(status)")

    def clear(self) -> None:
        with self._connection() as conn:
            conn.execute("DELETE FROM kg_conflicts")
            conn.execute("DELETE FROM kg_assertions")
            conn.execute("DELETE FROM kg_entities")

    def upsert_entity(
        self,
        name: str,
        *,
        entity_type: str = "entity",
        aliases: list[str] | tuple[str, ...] | None = None,
        scope: dict[str, Any] | None = None,
        source_memory_id: str | None = None,
        confidence: float = 1.0,
        created_at: float | None = None,
        status: str = "active",
    ) -> KnowledgeEntity:
        name = str(name).strip()
        if not name:
            raise ValueError("entity name is required")
        scope_dict = dict(scope or {})
        now = time.time() if created_at is None else float(created_at)
        entity_id = "ent_" + sha256_hex({"name": self._norm(name), "scope": scope_dict})[:24]
        new_aliases = sorted({self._clean_alias(item) for item in aliases or [] if self._clean_alias(item)} | {name})
        new_sources = [str(source_memory_id)] if source_memory_id else []

        with self._connection() as conn:
            row = conn.execute("SELECT * FROM kg_entities WHERE entity_id = ?", (entity_id,)).fetchone()
            if row is not None:
                existing = self._row_to_entity(row)
                new_aliases = sorted(set(existing.aliases) | set(new_aliases))
                new_sources = sorted(set(existing.source_memory_ids) | set(new_sources))
                now = max(now, existing.updated_at)
                confidence = max(existing.confidence, max(0.0, min(1.0, float(confidence))))
                created = existing.created_at
            else:
                created = now
                confidence = max(0.0, min(1.0, float(confidence)))
            conn.execute(
                """
                INSERT OR REPLACE INTO kg_entities (
                    entity_id, name, name_norm, entity_type, aliases_json, scope_json,
                    source_memory_ids_json, confidence, status, created_at, updated_at
                )
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    entity_id,
                    name,
                    self._norm(name),
                    str(entity_type or "entity"),
                    stable_json(new_aliases),
                    stable_json(scope_dict),
                    stable_json(new_sources),
                    confidence,
                    str(status or "active"),
                    created,
                    now,
                ),
            )
        return self.get_entity(entity_id)

    def get_entity(self, entity_id: str) -> KnowledgeEntity:
        with self._connection() as conn:
            row = conn.execute("SELECT * FROM kg_entities WHERE entity_id = ?", (str(entity_id),)).fetchone()
        if row is None:
            raise KeyError(entity_id)
        return self._row_to_entity(row)

    def add_assertion(
        self,
        *,
        subject: str,
        predicate: str,
        object: str,
        source_memory_id: str,
        scope: dict[str, Any] | None = None,
        object_kind: str = "entity",
        graph_name: str = "knowledge",
        fact_id: str | None = None,
        valid_from: float | None = None,
        valid_to: float | None = None,
        observed_at: float | None = None,
        confidence: float = 1.0,
        status: str = "active",
        reason: str = "",
        supersedes: list[str] | tuple[str, ...] | None = None,
    ) -> KnowledgeAssertion:
        subject = str(subject).strip()
        predicate = str(predicate).strip()
        object_value = str(object).strip()
        source_memory_id = str(source_memory_id).strip()
        if not subject or not predicate or not object_value or not source_memory_id:
            raise ValueError("subject, predicate, object, and source_memory_id are required")
        scope_dict = dict(scope or {})
        observed = time.time() if observed_at is None else float(observed_at)
        object_kind = str(object_kind or "entity")
        subject_entity = self.upsert_entity(
            subject,
            entity_type="subject",
            scope=scope_dict,
            source_memory_id=source_memory_id,
            confidence=confidence,
            created_at=observed,
        )
        object_entity_id = None
        if object_kind == "entity":
            object_entity_id = self.upsert_entity(
                object_value,
                entity_type="object",
                scope=scope_dict,
                source_memory_id=source_memory_id,
                confidence=confidence,
                created_at=observed,
            ).entity_id
        supersedes_list = [str(item) for item in supersedes or [] if str(item)]
        assertion_id = "kg_" + sha256_hex(
            {
                "subject": self._norm(subject),
                "predicate": self._norm(predicate),
                "object": self._norm(object_value),
                "source_memory_id": source_memory_id,
                "fact_id": fact_id,
                "valid_from": valid_from,
                "valid_to": valid_to,
            }
        )[:24]
        record = KnowledgeAssertion(
            assertion_id=assertion_id,
            subject=subject,
            predicate=predicate,
            object=object_value,
            source_memory_id=source_memory_id,
            subject_entity_id=subject_entity.entity_id,
            object_entity_id=object_entity_id,
            object_kind=object_kind,
            fact_id=fact_id,
            scope=scope_dict,
            graph_name=str(graph_name or "knowledge"),
            valid_from=valid_from,
            valid_to=valid_to,
            observed_at=observed,
            confidence=max(0.0, min(1.0, float(confidence))),
            status=str(status or "active"),
            reason=str(reason or ""),
            supersedes=supersedes_list,
        )
        with self._connection() as conn:
            conn.execute(
                """
                INSERT OR REPLACE INTO kg_assertions (
                    assertion_id, subject, predicate, object, subject_norm, predicate_norm, object_norm,
                    source_memory_id, subject_entity_id, object_entity_id, object_kind, fact_id, scope_json,
                    graph_name, valid_from, valid_to, observed_at, confidence, status, reason,
                    supersedes_json, superseded_by
                )
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    record.assertion_id,
                    record.subject,
                    record.predicate,
                    record.object,
                    self._norm(record.subject),
                    self._norm(record.predicate),
                    self._norm(record.object),
                    record.source_memory_id,
                    record.subject_entity_id,
                    record.object_entity_id,
                    record.object_kind,
                    record.fact_id,
                    stable_json(record.scope),
                    record.graph_name,
                    record.valid_from,
                    record.valid_to,
                    record.observed_at,
                    record.confidence,
                    record.status,
                    record.reason,
                    stable_json(record.supersedes),
                    record.superseded_by,
                ),
            )
            for old_assertion_id in supersedes_list:
                conn.execute(
                    "UPDATE kg_assertions SET status = ?, superseded_by = ? WHERE assertion_id = ?",
                    ("superseded", record.assertion_id, old_assertion_id),
                )
                conn.execute(
                    """
                    INSERT OR REPLACE INTO kg_conflicts (
                        superseding_assertion_id, superseded_assertion_id, reason, created_at
                    )
                    VALUES (?, ?, ?, ?)
                    """,
                    (record.assertion_id, old_assertion_id, record.reason, record.observed_at),
                )
        self._index_assertion_graph(record)
        return self.get_assertion(record.assertion_id)

    def get_assertion(self, assertion_id: str) -> KnowledgeAssertion:
        with self._connection() as conn:
            row = conn.execute("SELECT * FROM kg_assertions WHERE assertion_id = ?", (str(assertion_id),)).fetchone()
        if row is None:
            raise KeyError(assertion_id)
        return self._row_to_assertion(row)

    def assertion_ids_for_facts(self, fact_ids: list[str] | tuple[str, ...]) -> list[str]:
        ids = [str(item) for item in fact_ids if str(item)]
        if not ids:
            return []
        placeholders = ",".join("?" for _ in ids)
        with self._connection() as conn:
            rows = conn.execute(
                f"SELECT assertion_id FROM kg_assertions WHERE fact_id IN ({placeholders}) ORDER BY observed_at ASC",
                ids,
            ).fetchall()
        return [row["assertion_id"] for row in rows]

    def index_memory(self, memory: MemoryEnvelope) -> list[KnowledgeAssertion]:
        assertions: list[KnowledgeAssertion] = []
        for relation in memory.relations:
            assertions.append(
                self.add_assertion(
                    subject=str(relation.get("subject", "")),
                    predicate=str(relation.get("predicate", "")),
                    object=str(relation.get("object", "")),
                    source_memory_id=memory.id,
                    scope=memory.scope,
                    graph_name="knowledge",
                    valid_from=relation.get("valid_from"),
                    valid_to=relation.get("valid_to"),
                    observed_at=memory.created_at,
                    confidence=float(relation.get("confidence", memory.confidence)),
                    reason="memory_relation",
                )
            )
        return assertions

    def index_fact(self, fact: Any) -> KnowledgeAssertion:
        supersedes = self.assertion_ids_for_facts(list(getattr(fact, "supersedes", []) or []))
        return self.add_assertion(
            subject=fact.subject,
            predicate=fact.predicate,
            object=fact.object,
            source_memory_id=fact.source_memory_id,
            scope=fact.scope,
            object_kind="literal",
            fact_id=fact.fact_id,
            valid_from=fact.valid_from,
            valid_to=fact.valid_to,
            observed_at=fact.created_at,
            confidence=fact.confidence,
            status=fact.status,
            reason=fact.reason or "fact_ledger",
            supersedes=supersedes,
        )

    def query(
        self,
        *,
        subject: str | None = None,
        predicate: str | None = None,
        object: str | None = None,
        scope: dict[str, Any] | None = None,
        status: str | None = "active",
        at_time: float | None = None,
        memory_ids: list[str] | tuple[str, ...] | None = None,
        limit: int | None = 50,
    ) -> list[KnowledgeAssertion]:
        clauses: list[str] = []
        params: list[Any] = []
        if status:
            clauses.append("status = ?")
            params.append(str(status))
        if subject is not None:
            clauses.append("subject_norm = ?")
            params.append(self._norm(subject))
        if predicate is not None:
            clauses.append("predicate_norm = ?")
            params.append(self._norm(predicate))
        if object is not None:
            clauses.append("object_norm = ?")
            params.append(self._norm(object))
        if memory_ids:
            ids = [str(item) for item in memory_ids if str(item)]
            if ids:
                clauses.append("source_memory_id IN (" + ",".join("?" for _ in ids) + ")")
                params.extend(ids)
        if at_time is not None:
            clauses.append("(valid_from IS NULL OR valid_from <= ?)")
            clauses.append("(valid_to IS NULL OR valid_to >= ?)")
            params.extend([float(at_time), float(at_time)])
        where = " WHERE " + " AND ".join(clauses) if clauses else ""
        sql = f"SELECT * FROM kg_assertions{where} ORDER BY confidence DESC, observed_at DESC"
        if limit is not None:
            sql += " LIMIT ?"
            params.append(max(int(limit) * 4, int(limit)))
        with self._connection() as conn:
            rows = conn.execute(sql, params).fetchall()
        assertions = [self._row_to_assertion(row) for row in rows]
        if scope:
            assertions = [item for item in assertions if all(item.scope.get(key) == value for key, value in scope.items())]
        return assertions[:limit] if limit is not None else assertions

    def assertions_for_memory(self, memory_id: str) -> list[KnowledgeAssertion]:
        return self.query(memory_ids=[memory_id], status=None, limit=None)

    def summary_for_memories(
        self,
        memory_ids: list[str] | tuple[str, ...],
        *,
        scope: dict[str, Any] | None = None,
        at_time: float | None = None,
        limit: int = 8,
    ) -> dict[str, Any]:
        ids = [str(item) for item in dict.fromkeys(memory_ids) if str(item)]
        assertions = self.query(
            scope=scope,
            at_time=at_time,
            memory_ids=ids,
            status="active",
            limit=limit,
        )
        entity_ids = sorted(
            {
                assertion.subject_entity_id
                for assertion in assertions
                if assertion.subject_entity_id
            }
            | {
                assertion.object_entity_id
                for assertion in assertions
                if assertion.object_entity_id
            }
        )
        entities: list[KnowledgeEntity] = []
        for entity_id in entity_ids[:limit]:
            try:
                entity = self.get_entity(entity_id)
            except KeyError:
                continue
            if scope and any(entity.scope.get(key) != value for key, value in scope.items()):
                continue
            entities.append(entity)
        return {
            "format": self.FORMAT,
            "memory_ids": ids,
            "entity_count": len(entities),
            "assertion_count": len(assertions),
            "entities": [entity.to_dict() for entity in entities],
            "assertions": [assertion.to_dict() for assertion in assertions],
        }

    def count(self) -> dict[str, int]:
        with self._connection() as conn:
            entity_row = conn.execute("SELECT COUNT(*) AS total FROM kg_entities").fetchone()
            assertion_row = conn.execute("SELECT COUNT(*) AS total FROM kg_assertions").fetchone()
            active_row = conn.execute("SELECT COUNT(*) AS total FROM kg_assertions WHERE status = 'active'").fetchone()
        return {
            "entities": int(entity_row["total"] if entity_row else 0),
            "assertions": int(assertion_row["total"] if assertion_row else 0),
            "active_assertions": int(active_row["total"] if active_row else 0),
        }

    def _index_assertion_graph(self, assertion: KnowledgeAssertion) -> None:
        if self.graph is None:
            return
        self.graph.add_relation(
            subject=assertion.subject,
            predicate=assertion.predicate,
            object=assertion.object,
            memory_id=assertion.source_memory_id,
            graph_name=assertion.graph_name,
            valid_from=assertion.valid_from,
            valid_to=assertion.valid_to,
            confidence=assertion.confidence,
            source="knowledge_graph",
            created_at=assertion.observed_at,
        )
        for old_assertion_id in assertion.supersedes:
            self.graph.add_relation(
                subject=assertion.assertion_id,
                predicate="supersedes",
                object=old_assertion_id,
                memory_id=assertion.source_memory_id,
                graph_name="conflict",
                confidence=assertion.confidence,
                source="knowledge_graph",
                created_at=assertion.observed_at,
            )

    def _row_to_entity(self, row: sqlite3.Row) -> KnowledgeEntity:
        return KnowledgeEntity(
            entity_id=row["entity_id"],
            name=row["name"],
            entity_type=row["entity_type"],
            aliases=json.loads(row["aliases_json"]),
            scope=json.loads(row["scope_json"]),
            source_memory_ids=json.loads(row["source_memory_ids_json"]),
            confidence=row["confidence"],
            status=row["status"],
            created_at=row["created_at"],
            updated_at=row["updated_at"],
        )

    def _row_to_assertion(self, row: sqlite3.Row) -> KnowledgeAssertion:
        return KnowledgeAssertion(
            assertion_id=row["assertion_id"],
            subject=row["subject"],
            predicate=row["predicate"],
            object=row["object"],
            source_memory_id=row["source_memory_id"],
            subject_entity_id=row["subject_entity_id"],
            object_entity_id=row["object_entity_id"],
            object_kind=row["object_kind"],
            fact_id=row["fact_id"],
            scope=json.loads(row["scope_json"]),
            graph_name=row["graph_name"],
            valid_from=row["valid_from"],
            valid_to=row["valid_to"],
            observed_at=row["observed_at"],
            confidence=row["confidence"],
            status=row["status"],
            reason=row["reason"],
            supersedes=json.loads(row["supersedes_json"]),
            superseded_by=row["superseded_by"],
        )

    def _clean_alias(self, value: Any) -> str:
        return " ".join(str(value).strip().split())

    def _norm(self, value: str) -> str:
        return " ".join(str(value).strip().lower().split())
