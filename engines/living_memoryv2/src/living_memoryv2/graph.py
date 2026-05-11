from __future__ import annotations

from contextlib import contextmanager
from dataclasses import dataclass
from pathlib import Path
import re
import sqlite3
import time
from typing import Any, Iterator

from .schema import MemoryEnvelope

DEFAULT_GRAPH_NAME = "semantic"
ALLOWED_GRAPH_NAMES = {
    "semantic",
    "temporal",
    "multimodal",
    "procedural",
    "provenance",
    "topology",
    "facts",
    "conflict",
    "knowledge",
    "default",
}


@dataclass(frozen=True)
class TemporalRelation:
    subject: str
    predicate: str
    object: str
    memory_id: str
    graph_name: str = DEFAULT_GRAPH_NAME
    valid_from: float | None = None
    valid_to: float | None = None
    confidence: float = 1.0
    source: str | None = None
    created_at: float = 0.0

    def to_dict(self) -> dict[str, Any]:
        return {
            "subject": self.subject,
            "predicate": self.predicate,
            "object": self.object,
            "memory_id": self.memory_id,
            "graph_name": self.graph_name,
            "valid_from": self.valid_from,
            "valid_to": self.valid_to,
            "confidence": self.confidence,
            "source": self.source,
            "created_at": self.created_at,
        }


class GraphStore:
    def __init__(self, db_path: str | Path) -> None:
        self.db_path = Path(db_path)
        self.db_path.parent.mkdir(parents=True, exist_ok=True)
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
                CREATE TABLE IF NOT EXISTS relations (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    graph_name TEXT NOT NULL DEFAULT 'semantic',
                    subject TEXT NOT NULL,
                    predicate TEXT NOT NULL,
                    object TEXT NOT NULL,
                    subject_norm TEXT NOT NULL,
                    predicate_norm TEXT NOT NULL,
                    object_norm TEXT NOT NULL,
                    memory_id TEXT NOT NULL,
                    valid_from REAL,
                    valid_to REAL,
                    confidence REAL NOT NULL,
                    source TEXT,
                    created_at REAL NOT NULL,
                    UNIQUE(graph_name, subject_norm, predicate_norm, object_norm, memory_id, valid_from, valid_to)
                )
                """
            )
            self._migrate_graph_name(conn)
            conn.execute("CREATE INDEX IF NOT EXISTS idx_graph_name ON relations(graph_name)")
            conn.execute("CREATE INDEX IF NOT EXISTS idx_graph_subject ON relations(subject_norm)")
            conn.execute("CREATE INDEX IF NOT EXISTS idx_graph_object ON relations(object_norm)")
            conn.execute("CREATE INDEX IF NOT EXISTS idx_graph_predicate ON relations(predicate_norm)")
            conn.execute("CREATE INDEX IF NOT EXISTS idx_graph_memory ON relations(memory_id)")

    def _migrate_graph_name(self, conn: sqlite3.Connection) -> None:
        columns = {row["name"] for row in conn.execute("PRAGMA table_info(relations)").fetchall()}
        if "graph_name" in columns:
            return
        conn.execute("ALTER TABLE relations RENAME TO relations_legacy")
        conn.execute(
            """
            CREATE TABLE relations (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                graph_name TEXT NOT NULL DEFAULT 'semantic',
                subject TEXT NOT NULL,
                predicate TEXT NOT NULL,
                object TEXT NOT NULL,
                subject_norm TEXT NOT NULL,
                predicate_norm TEXT NOT NULL,
                object_norm TEXT NOT NULL,
                memory_id TEXT NOT NULL,
                valid_from REAL,
                valid_to REAL,
                confidence REAL NOT NULL,
                source TEXT,
                created_at REAL NOT NULL,
                UNIQUE(graph_name, subject_norm, predicate_norm, object_norm, memory_id, valid_from, valid_to)
            )
            """
        )
        conn.execute(
            """
            INSERT OR IGNORE INTO relations (
                graph_name, subject, predicate, object, subject_norm, predicate_norm,
                object_norm, memory_id, valid_from, valid_to, confidence, source, created_at
            )
            SELECT
                CASE
                    WHEN valid_from IS NOT NULL OR valid_to IS NOT NULL THEN 'temporal'
                    ELSE 'semantic'
                END,
                subject, predicate, object, subject_norm, predicate_norm,
                object_norm, memory_id, valid_from, valid_to, confidence, source, created_at
            FROM relations_legacy
            """
        )
        conn.execute("DROP TABLE relations_legacy")

    def clear(self) -> None:
        with self._connection() as conn:
            conn.execute("DELETE FROM relations")

    def add_relation(
        self,
        *,
        subject: str,
        predicate: str,
        object: str,
        memory_id: str,
        graph_name: str | None = None,
        valid_from: float | None = None,
        valid_to: float | None = None,
        confidence: float = 1.0,
        source: str | None = None,
        created_at: float | None = None,
    ) -> TemporalRelation:
        normalized_graph_name = self._graph_name(graph_name, valid_from=valid_from, valid_to=valid_to)
        relation = TemporalRelation(
            subject=subject.strip(),
            predicate=predicate.strip(),
            object=object.strip(),
            memory_id=memory_id,
            graph_name=normalized_graph_name,
            valid_from=valid_from,
            valid_to=valid_to,
            confidence=max(0.0, min(1.0, float(confidence))),
            source=source,
            created_at=time.time() if created_at is None else created_at,
        )
        if not relation.subject or not relation.predicate or not relation.object:
            raise ValueError("subject, predicate and object are required")
        with self._connection() as conn:
            conn.execute(
                """
                INSERT OR IGNORE INTO relations (
                    graph_name, subject, predicate, object, subject_norm, predicate_norm, object_norm,
                    memory_id, valid_from, valid_to, confidence, source, created_at
                )
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    relation.graph_name,
                    relation.subject,
                    relation.predicate,
                    relation.object,
                    self._norm(relation.subject),
                    self._norm(relation.predicate),
                    self._norm(relation.object),
                    relation.memory_id,
                    relation.valid_from,
                    relation.valid_to,
                    relation.confidence,
                    relation.source,
                    relation.created_at,
                ),
            )
        return relation

    def index_memory(self, memory: MemoryEnvelope) -> list[TemporalRelation]:
        indexed: list[TemporalRelation] = []
        for item in memory.relations:
            indexed.append(
                self.add_relation(
                    subject=str(item.get("subject", "")),
                    predicate=str(item.get("predicate", "")),
                    object=str(item.get("object", "")),
                    memory_id=memory.id,
                    graph_name=item.get("graph_name"),
                    valid_from=item.get("valid_from"),
                    valid_to=item.get("valid_to"),
                    confidence=item.get("confidence", memory.confidence),
                    source=memory.provenance.get("source"),
                    created_at=memory.created_at,
                )
            )
        return indexed

    def query(
        self,
        *,
        subject: str | None = None,
        predicate: str | None = None,
        object: str | None = None,
        graph_name: str | None = None,
        at_time: float | None = None,
        limit: int = 50,
    ) -> list[TemporalRelation]:
        clauses: list[str] = []
        params: list[Any] = []
        if graph_name is not None:
            clauses.append("graph_name = ?")
            params.append(self._graph_name(graph_name))
        if subject is not None:
            clauses.append("subject_norm = ?")
            params.append(self._norm(subject))
        if predicate is not None:
            clauses.append("predicate_norm = ?")
            params.append(self._norm(predicate))
        if object is not None:
            clauses.append("object_norm = ?")
            params.append(self._norm(object))
        if at_time is not None:
            clauses.append("(valid_from IS NULL OR valid_from <= ?)")
            clauses.append("(valid_to IS NULL OR valid_to >= ?)")
            params.extend([at_time, at_time])
        where = " WHERE " + " AND ".join(clauses) if clauses else ""
        with self._connection() as conn:
            rows = conn.execute(
                f"SELECT * FROM relations{where} ORDER BY confidence DESC, created_at DESC LIMIT ?",
                [*params, limit],
            ).fetchall()
        return [self._row_to_relation(row) for row in rows]

    def related_memory_ids(self, terms: list[str], *, limit: int = 50) -> list[str]:
        return self.walk_related_memory_ids(terms, limit=limit, max_hops=1)

    def relations_for_memory(self, memory_id: str) -> list[TemporalRelation]:
        with self._connection() as conn:
            rows = conn.execute(
                "SELECT * FROM relations WHERE memory_id = ? ORDER BY confidence DESC, created_at DESC",
                (memory_id,),
            ).fetchall()
        return [self._row_to_relation(row) for row in rows]

    def relation_nodes_for_memory(
        self,
        memory_id: str,
        *,
        graph_names: list[str] | tuple[str, ...] | None = None,
        at_time: float | None = None,
    ) -> set[str]:
        return self.relation_nodes_for_memory_ids(
            [memory_id],
            graph_names=graph_names,
            at_time=at_time,
        ).get(memory_id, set())

    def relation_nodes_for_memory_ids(
        self,
        memory_ids: list[str] | tuple[str, ...],
        *,
        graph_names: list[str] | tuple[str, ...] | None = None,
        at_time: float | None = None,
    ) -> dict[str, set[str]]:
        ids = [str(item) for item in dict.fromkeys(memory_ids) if str(item)]
        if not ids:
            return {}
        wanted_graphs = {self._graph_name(item) for item in graph_names or []}
        placeholders = ",".join("?" for _ in ids)
        with self._connection() as conn:
            rows = conn.execute(
                f"SELECT * FROM relations WHERE memory_id IN ({placeholders}) ORDER BY confidence DESC, created_at DESC",
                ids,
            ).fetchall()

        grouped: dict[str, set[str]] = {memory_id: set() for memory_id in ids}
        for row in rows:
            relation = self._row_to_relation(row)
            if wanted_graphs and relation.graph_name not in wanted_graphs:
                continue
            if at_time is not None and not self._relation_is_active_at(relation, float(at_time)):
                continue
            grouped.setdefault(relation.memory_id, set()).add(self._norm(relation.subject))
            grouped.setdefault(relation.memory_id, set()).add(self._norm(relation.object))
        return {memory_id: {node for node in nodes if node} for memory_id, nodes in grouped.items()}

    def walk_related_memory_ids(
        self,
        terms: list[str],
        *,
        limit: int = 50,
        max_hops: int = 2,
        at_time: float | None = None,
        graph_names: list[str] | tuple[str, ...] | None = None,
    ) -> list[str]:
        normalized = [self._norm(term) for term in terms if self._norm(term)]
        if not normalized:
            return []
        graph_filter = [self._graph_name(item) for item in graph_names or []]
        max_hops = max(1, int(max_hops))
        frontier = set(normalized)
        seen_nodes: set[str] = set()
        scores: dict[str, tuple[float, float]] = {}

        with self._connection() as conn:
            for hop in range(max_hops):
                frontier = {item for item in frontier if item and item not in seen_nodes}
                if not frontier:
                    break
                seen_nodes.update(frontier)
                placeholders = ",".join("?" for _ in frontier)
                time_clause = ""
                params: list[Any] = [*frontier, *frontier, *frontier]
                graph_clause = ""
                if graph_filter:
                    graph_placeholders = ",".join("?" for _ in graph_filter)
                    graph_clause = f"AND graph_name IN ({graph_placeholders})"
                    params.extend(graph_filter)
                if at_time is not None:
                    time_clause = """
                    AND (valid_from IS NULL OR valid_from <= ?)
                    AND (valid_to IS NULL OR valid_to >= ?)
                    """
                    params.extend([at_time, at_time])
                rows = conn.execute(
                    f"""
                    SELECT *
                    FROM relations
                    WHERE (
                        subject_norm IN ({placeholders})
                        OR predicate_norm IN ({placeholders})
                        OR object_norm IN ({placeholders})
                    )
                    {graph_clause}
                    {time_clause}
                    LIMIT ?
                    """,
                    [*params, max(limit * 4, limit)],
                ).fetchall()

                next_frontier: set[str] = set()
                hop_weight = max(0.1, 1.0 - (hop * 0.15))
                for row in rows:
                    score = float(row["confidence"]) * hop_weight
                    current = scores.get(row["memory_id"])
                    if current is None or score > current[0]:
                        scores[row["memory_id"]] = (score, float(row["created_at"]))
                    next_frontier.add(row["subject_norm"])
                    next_frontier.add(row["object_norm"])
                frontier = next_frontier

        ordered = sorted(scores.items(), key=lambda item: (item[1][0], item[1][1]), reverse=True)
        return [memory_id for memory_id, _score in ordered[:limit]]

    def answerable_subgraphs(
        self,
        terms: list[str],
        *,
        graph_names: list[str] | tuple[str, ...] | None = None,
        memory_ids: list[str] | tuple[str, ...] | None = None,
        at_time: float | None = None,
        limit: int = 6,
        min_anchor_terms: int = 2,
    ) -> list[dict[str, Any]]:
        anchors = {self._term(term) for term in terms if self._term(term)}
        if not anchors:
            return []
        wanted_graphs = {self._graph_name(item) for item in graph_names or []}
        wanted_memories = set(memory_ids or [])
        clauses: list[str] = []
        params: list[Any] = []
        if wanted_graphs:
            clauses.append("graph_name IN (" + ",".join("?" for _ in wanted_graphs) + ")")
            params.extend(sorted(wanted_graphs))
        if wanted_memories:
            clauses.append("memory_id IN (" + ",".join("?" for _ in wanted_memories) + ")")
            params.extend(sorted(wanted_memories))
        if at_time is not None:
            clauses.append("(valid_from IS NULL OR valid_from <= ?)")
            clauses.append("(valid_to IS NULL OR valid_to >= ?)")
            params.extend([at_time, at_time])
        where = " WHERE " + " AND ".join(clauses) if clauses else ""
        with self._connection() as conn:
            rows = conn.execute(
                f"SELECT * FROM relations{where} ORDER BY confidence DESC, created_at DESC LIMIT ?",
                [*params, max(limit * 12, 50)],
            ).fetchall()

        grouped: dict[str, dict[str, Any]] = {}
        for row in rows:
            relation = self._row_to_relation(row)
            relation_terms = self._relation_terms(relation)
            matched = anchors & relation_terms
            if len(matched) < min_anchor_terms:
                continue
            bucket = grouped.setdefault(
                relation.graph_name,
                {
                    "graph_name": relation.graph_name,
                    "relations": [],
                    "memory_ids": [],
                    "anchor_terms": set(),
                    "answerability": 0.0,
                },
            )
            bucket["relations"].append(relation.to_dict())
            if relation.memory_id not in bucket["memory_ids"]:
                bucket["memory_ids"].append(relation.memory_id)
            bucket["anchor_terms"].update(matched)
            coverage = len(bucket["anchor_terms"]) / max(1, len(anchors))
            bucket["answerability"] = max(bucket["answerability"], round(coverage * relation.confidence, 4))

        ordered = sorted(grouped.values(), key=lambda item: item["answerability"], reverse=True)
        clean: list[dict[str, Any]] = []
        for item in ordered[:limit]:
            item["anchor_terms"] = sorted(item["anchor_terms"])
            item["relations"] = item["relations"][:limit]
            clean.append(item)
        return clean

    def _row_to_relation(self, row: sqlite3.Row) -> TemporalRelation:
        return TemporalRelation(
            subject=row["subject"],
            predicate=row["predicate"],
            object=row["object"],
            memory_id=row["memory_id"],
            graph_name=row["graph_name"],
            valid_from=row["valid_from"],
            valid_to=row["valid_to"],
            confidence=row["confidence"],
            source=row["source"],
            created_at=row["created_at"],
        )

    def _relation_is_active_at(self, relation: TemporalRelation, at_time: float) -> bool:
        starts_before = relation.valid_from is None or relation.valid_from <= at_time
        ends_after = relation.valid_to is None or relation.valid_to >= at_time
        return starts_before and ends_after

    def _graph_name(
        self,
        value: str | None,
        *,
        valid_from: float | None = None,
        valid_to: float | None = None,
    ) -> str:
        if value is None or str(value).strip() == "":
            return "temporal" if valid_from is not None or valid_to is not None else DEFAULT_GRAPH_NAME
        graph_name = self._norm(str(value)).replace(" ", "_")
        if graph_name not in ALLOWED_GRAPH_NAMES:
            raise ValueError(f"unknown graph_name: {value}")
        return graph_name

    def _relation_terms(self, relation: TemporalRelation) -> set[str]:
        return {
            self._term(term)
            for term in re.findall(
                r"[\w]+",
                " ".join([relation.subject, relation.predicate, relation.object, relation.graph_name]).lower(),
                flags=re.UNICODE,
            )
            if self._term(term)
        }

    def _term(self, value: str) -> str:
        term = self._norm(str(value))
        return term if len(term) >= 3 else ""

    def _norm(self, value: str) -> str:
        return " ".join(str(value).strip().lower().split())
