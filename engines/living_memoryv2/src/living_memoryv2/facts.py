from __future__ import annotations

from contextlib import contextmanager
from dataclasses import dataclass, field
import json
from pathlib import Path
import sqlite3
import time
from typing import Any, Iterator

from .graph import GraphStore
from .schema import sha256_hex, stable_json


@dataclass(frozen=True)
class FactRecord:
    fact_id: str
    subject: str
    predicate: str
    object: str
    source_memory_id: str
    scope: dict[str, Any] = field(default_factory=dict)
    status: str = "active"
    supersedes: list[str] = field(default_factory=list)
    superseded_by: str | None = None
    reason: str = ""
    valid_from: float | None = None
    valid_to: float | None = None
    confidence: float = 1.0
    created_at: float = 0.0

    def to_dict(self) -> dict[str, Any]:
        return {
            "fact_id": self.fact_id,
            "subject": self.subject,
            "predicate": self.predicate,
            "object": self.object,
            "source_memory_id": self.source_memory_id,
            "scope": self.scope,
            "status": self.status,
            "supersedes": list(self.supersedes),
            "superseded_by": self.superseded_by,
            "reason": self.reason,
            "valid_from": self.valid_from,
            "valid_to": self.valid_to,
            "confidence": self.confidence,
            "created_at": self.created_at,
        }


class FactLedger:
    def __init__(self, db_path: str | Path, graph: GraphStore, knowledge: Any | None = None) -> None:
        self.db_path = Path(db_path)
        self.db_path.parent.mkdir(parents=True, exist_ok=True)
        self.graph = graph
        self.knowledge = knowledge
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
                CREATE TABLE IF NOT EXISTS facts (
                    fact_id TEXT PRIMARY KEY,
                    subject TEXT NOT NULL,
                    predicate TEXT NOT NULL,
                    object TEXT NOT NULL,
                    subject_norm TEXT NOT NULL,
                    predicate_norm TEXT NOT NULL,
                    object_norm TEXT NOT NULL,
                    source_memory_id TEXT NOT NULL,
                    scope_json TEXT NOT NULL,
                    status TEXT NOT NULL,
                    supersedes_json TEXT NOT NULL,
                    superseded_by TEXT,
                    reason TEXT NOT NULL,
                    valid_from REAL,
                    valid_to REAL,
                    confidence REAL NOT NULL,
                    created_at REAL NOT NULL
                )
                """
            )
            conn.execute(
                """
                CREATE TABLE IF NOT EXISTS fact_conflicts (
                    superseding_fact_id TEXT NOT NULL,
                    superseded_fact_id TEXT NOT NULL,
                    reason TEXT NOT NULL,
                    created_at REAL NOT NULL,
                    PRIMARY KEY (superseding_fact_id, superseded_fact_id)
                )
                """
            )
            conn.execute("CREATE INDEX IF NOT EXISTS idx_facts_memory ON facts(source_memory_id)")
            conn.execute("CREATE INDEX IF NOT EXISTS idx_facts_status ON facts(status)")
            conn.execute("CREATE INDEX IF NOT EXISTS idx_facts_spo ON facts(subject_norm, predicate_norm, object_norm)")

    def add_fact(
        self,
        *,
        subject: str,
        predicate: str,
        object: str,
        source_memory_id: str,
        scope: dict[str, Any] | None = None,
        confidence: float = 1.0,
        valid_from: float | None = None,
        valid_to: float | None = None,
        supersedes: list[str] | tuple[str, ...] | None = None,
        reason: str = "",
        created_at: float | None = None,
    ) -> FactRecord:
        subject = str(subject).strip()
        predicate = str(predicate).strip()
        object = str(object).strip()
        source_memory_id = str(source_memory_id).strip()
        if not subject or not predicate or not object or not source_memory_id:
            raise ValueError("subject, predicate, object, and source_memory_id are required")
        supersedes_list = [str(item) for item in supersedes or [] if str(item)]
        created = time.time() if created_at is None else float(created_at)
        fact_id = "fact_" + sha256_hex(
            {
                "subject": self._norm(subject),
                "predicate": self._norm(predicate),
                "object": self._norm(object),
                "source_memory_id": source_memory_id,
                "valid_from": valid_from,
                "valid_to": valid_to,
            }
        )[:24]
        record = FactRecord(
            fact_id=fact_id,
            subject=subject,
            predicate=predicate,
            object=object,
            source_memory_id=source_memory_id,
            scope=dict(scope or {}),
            supersedes=supersedes_list,
            reason=str(reason or ""),
            valid_from=valid_from,
            valid_to=valid_to,
            confidence=max(0.0, min(1.0, float(confidence))),
            created_at=created,
        )
        with self._connection() as conn:
            conn.execute(
                """
                INSERT OR REPLACE INTO facts (
                    fact_id, subject, predicate, object, subject_norm, predicate_norm, object_norm,
                    source_memory_id, scope_json, status, supersedes_json, superseded_by,
                    reason, valid_from, valid_to, confidence, created_at
                )
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    record.fact_id,
                    record.subject,
                    record.predicate,
                    record.object,
                    self._norm(record.subject),
                    self._norm(record.predicate),
                    self._norm(record.object),
                    record.source_memory_id,
                    stable_json(record.scope),
                    record.status,
                    stable_json(record.supersedes),
                    record.superseded_by,
                    record.reason,
                    record.valid_from,
                    record.valid_to,
                    record.confidence,
                    record.created_at,
                ),
            )
            self._index_fact_graph(record)
            for old_fact_id in supersedes_list:
                conn.execute(
                    "UPDATE facts SET status = ?, superseded_by = ? WHERE fact_id = ?",
                    ("superseded", record.fact_id, old_fact_id),
                )
                conn.execute(
                    """
                    INSERT OR REPLACE INTO fact_conflicts (
                        superseding_fact_id, superseded_fact_id, reason, created_at
                    )
                    VALUES (?, ?, ?, ?)
                    """,
                    (record.fact_id, old_fact_id, record.reason, record.created_at),
                )
                self._index_conflict_graph(record, old_fact_id)
        indexed = self.get(record.fact_id)
        if self.knowledge is not None:
            self.knowledge.index_fact(indexed)
        return indexed

    def get(self, fact_id: str) -> FactRecord:
        with self._connection() as conn:
            row = conn.execute("SELECT * FROM facts WHERE fact_id = ?", (str(fact_id),)).fetchone()
        if row is None:
            raise KeyError(fact_id)
        return self._row_to_fact(row)

    def facts_for_memory(self, memory_id: str) -> list[FactRecord]:
        with self._connection() as conn:
            rows = conn.execute(
                "SELECT * FROM facts WHERE source_memory_id = ? ORDER BY confidence DESC, created_at DESC",
                (str(memory_id),),
            ).fetchall()
        return [self._row_to_fact(row) for row in rows]

    def list_facts(
        self,
        *,
        scope: dict[str, Any] | None = None,
        status: str | None = None,
        limit: int | None = 100,
    ) -> list[FactRecord]:
        clauses: list[str] = []
        params: list[Any] = []
        if status:
            clauses.append("status = ?")
            params.append(str(status))
        where = " WHERE " + " AND ".join(clauses) if clauses else ""
        sql = f"SELECT * FROM facts{where} ORDER BY confidence DESC, created_at DESC"
        if limit is not None:
            sql += " LIMIT ?"
            params.append(int(limit))
        with self._connection() as conn:
            rows = conn.execute(sql, params).fetchall()
        facts = [self._row_to_fact(row) for row in rows]
        if scope:
            facts = [fact for fact in facts if all(fact.scope.get(key) == value for key, value in scope.items())]
        return facts[:limit] if limit is not None else facts

    def conflicts_for_fact(self, fact_id: str) -> list[dict[str, Any]]:
        with self._connection() as conn:
            rows = conn.execute(
                """
                SELECT superseding_fact_id, superseded_fact_id, reason, created_at
                FROM fact_conflicts
                WHERE superseding_fact_id = ? OR superseded_fact_id = ?
                ORDER BY created_at DESC
                """,
                (str(fact_id), str(fact_id)),
            ).fetchall()
        return [dict(row) for row in rows]

    def memory_fact_status(self, memory_id: str) -> dict[str, Any]:
        facts = self.facts_for_memory(memory_id)
        active = [fact for fact in facts if fact.status == "active"]
        superseded = [fact for fact in facts if fact.status == "superseded"]
        return {
            "active_fact_count": len(active),
            "superseded_fact_count": len(superseded),
            "fact_ids": [fact.fact_id for fact in facts],
            "active_fact_ids": [fact.fact_id for fact in active],
            "superseded_fact_ids": [fact.fact_id for fact in superseded],
        }

    def count(self) -> int:
        with self._connection() as conn:
            row = conn.execute("SELECT COUNT(*) AS total FROM facts").fetchone()
        return int(row["total"] if row else 0)

    def rebuild_graph_index(self) -> None:
        with self._connection() as conn:
            fact_rows = conn.execute("SELECT * FROM facts ORDER BY created_at ASC").fetchall()
            conflict_rows = conn.execute(
                """
                SELECT superseding_fact_id, superseded_fact_id
                FROM fact_conflicts
                ORDER BY created_at ASC
                """
            ).fetchall()
        facts: dict[str, FactRecord] = {}
        for row in fact_rows:
            fact = self._row_to_fact(row)
            facts[fact.fact_id] = fact
        for fact in facts.values():
            self._index_fact_graph(fact)
        for row in conflict_rows:
            superseding = facts.get(row["superseding_fact_id"])
            if superseding is not None:
                self._index_conflict_graph(superseding, row["superseded_fact_id"])

    def _row_to_fact(self, row: sqlite3.Row) -> FactRecord:
        return FactRecord(
            fact_id=row["fact_id"],
            subject=row["subject"],
            predicate=row["predicate"],
            object=row["object"],
            source_memory_id=row["source_memory_id"],
            scope=json.loads(row["scope_json"]),
            status=row["status"],
            supersedes=json.loads(row["supersedes_json"]),
            superseded_by=row["superseded_by"],
            reason=row["reason"],
            valid_from=row["valid_from"],
            valid_to=row["valid_to"],
            confidence=row["confidence"],
            created_at=row["created_at"],
        )

    def _norm(self, value: str) -> str:
        return " ".join(str(value).strip().lower().split())

    def _index_fact_graph(self, record: FactRecord) -> None:
        self.graph.add_relation(
            subject=record.subject,
            predicate=record.predicate,
            object=record.object,
            memory_id=record.source_memory_id,
            graph_name="facts",
            valid_from=record.valid_from,
            valid_to=record.valid_to,
            confidence=record.confidence,
            source="fact_ledger",
            created_at=record.created_at,
        )

    def _index_conflict_graph(self, record: FactRecord, old_fact_id: str) -> None:
        self.graph.add_relation(
            subject=record.fact_id,
            predicate="supersedes",
            object=old_fact_id,
            memory_id=record.source_memory_id,
            graph_name="conflict",
            confidence=record.confidence,
            source="fact_ledger",
            created_at=record.created_at,
        )
