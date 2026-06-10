from __future__ import annotations

from contextlib import contextmanager
from pathlib import Path
import json
import re
import sqlite3
from typing import Any, Iterator

from .mobius import MobiusAddress, MobiusIndex
from .schema import MemoryEnvelope, ModalityRef, content_bytes, stable_json

_SHA256_HEX_RE = re.compile(r"^[0-9a-f]{64}$")


def _is_sha256_hex(value: Any) -> bool:
    return isinstance(value, str) and bool(_SHA256_HEX_RE.match(value))


class TemporalStore:
    def __init__(
        self,
        root: str | Path,
        *,
        blob_threshold: int = 64 * 1024,
        mobius_index: MobiusIndex | None = None,
    ) -> None:
        self.root = Path(root)
        self.root.mkdir(parents=True, exist_ok=True)
        self.blob_dir = self.root / "blobs"
        self.blob_dir.mkdir(parents=True, exist_ok=True)
        self.db_path = self.root / "memory.sqlite3"
        self.jsonl_path = self.root / "memories.jsonl"
        self.blob_threshold = blob_threshold
        self.mobius_index = mobius_index or MobiusIndex()
        self.fts_available = True
        self._init_db()

    def _connect(self) -> sqlite3.Connection:
        conn = sqlite3.connect(self.db_path)
        conn.row_factory = sqlite3.Row
        return conn

    @contextmanager
    def _connection(self) -> Iterator[sqlite3.Connection]:
        conn = self._connect()
        try:
            yield conn
            conn.commit()
        finally:
            conn.close()

    def _init_db(self) -> None:
        with self._connection() as conn:
            conn.execute("PRAGMA journal_mode=DELETE")
            conn.execute(
                """
                CREATE TABLE IF NOT EXISTS memories (
                    id TEXT PRIMARY KEY,
                    created_at REAL NOT NULL,
                    updated_at REAL NOT NULL,
                    memory_type TEXT NOT NULL,
                    scope_json TEXT NOT NULL,
                    tags_json TEXT NOT NULL,
                    entities_json TEXT NOT NULL,
                    importance REAL NOT NULL,
                    confidence REAL NOT NULL,
                    sensitivity TEXT NOT NULL,
                    content_hash TEXT NOT NULL,
                    envelope_json TEXT NOT NULL
                )
                """
            )
            conn.execute(
                """
                CREATE TABLE IF NOT EXISTS mobius_index (
                    memory_id TEXT PRIMARY KEY,
                    iu INTEGER NOT NULL,
                    iv INTEGER NOT NULL,
                    iw INTEGER NOT NULL,
                    region TEXT NOT NULL
                )
                """
            )
            conn.execute(
                """
                CREATE INDEX IF NOT EXISTS idx_mobius_region_coords
                ON mobius_index(region, iu, iv, iw)
                """
            )
            try:
                conn.execute("CREATE VIRTUAL TABLE IF NOT EXISTS memory_fts USING fts5(id UNINDEXED, text)")
            except sqlite3.OperationalError:
                self.fts_available = False
                conn.execute("CREATE TABLE IF NOT EXISTS memory_fts (id TEXT PRIMARY KEY, text TEXT NOT NULL)")
            conn.execute("CREATE INDEX IF NOT EXISTS idx_memories_created ON memories(created_at)")
            conn.execute("CREATE INDEX IF NOT EXISTS idx_memories_importance ON memories(importance)")

    def remember(self, memory: MemoryEnvelope, *, append_log: bool = True) -> str:
        original = MemoryEnvelope.from_dict(memory.to_dict())
        search_text = original.text_content()
        stored = self._externalize_large_payloads(original)
        address = self.mobius_index.address_for(original)
        payload = stable_json(stored.to_dict())
        with self._connection() as conn:
            conn.execute(
                """
                INSERT OR REPLACE INTO memories (
                    id, created_at, updated_at, memory_type, scope_json, tags_json,
                    entities_json, importance, confidence, sensitivity, content_hash, envelope_json
                )
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    stored.id,
                    stored.created_at,
                    stored.updated_at,
                    stored.memory_type,
                    stable_json(stored.scope),
                    stable_json(stored.tags),
                    stable_json(stored.entities),
                    stored.importance,
                    stored.confidence,
                    stored.sensitivity,
                    stored.content_hash,
                    payload,
                ),
            )
            conn.execute("DELETE FROM memory_fts WHERE id = ?", (stored.id,))
            conn.execute("INSERT INTO memory_fts (id, text) VALUES (?, ?)", (stored.id, search_text))
            conn.execute(
                """
                INSERT OR REPLACE INTO mobius_index (memory_id, iu, iv, iw, region)
                VALUES (?, ?, ?, ?, ?)
                """,
                (stored.id, address.iu, address.iv, address.iw, address.region),
            )
        if append_log:
            with self.jsonl_path.open("a", encoding="utf-8") as fh:
                fh.write(payload + "\n")
        return stored.id

    def inspect(self, memory_id: str, *, resolve_blobs: bool = True) -> MemoryEnvelope:
        with self._connection() as conn:
            row = conn.execute("SELECT envelope_json FROM memories WHERE id = ?", (memory_id,)).fetchone()
        if row is None:
            raise KeyError(memory_id)
        memory = MemoryEnvelope.from_dict(json.loads(row["envelope_json"]))
        return self._resolve_blobs(memory) if resolve_blobs else memory

    def resolve_blobs(self, memory: MemoryEnvelope) -> MemoryEnvelope:
        return self._resolve_blobs(memory)

    def search_text(
        self,
        query: str,
        *,
        scope: dict[str, Any] | None = None,
        tags: list[str] | None = None,
        limit: int = 20,
    ) -> list[MemoryEnvelope]:
        expression = self._fts_expression(query)
        if not expression:
            return self.list_memories(scope=scope, tags=tags, limit=limit)
        with self._connection() as conn:
            try:
                if self.fts_available:
                    rows = conn.execute(
                        """
                        SELECT m.envelope_json
                        FROM memory_fts f
                        JOIN memories m ON m.id = f.id
                        WHERE memory_fts MATCH ?
                        ORDER BY bm25(memory_fts)
                        LIMIT ?
                        """,
                        (expression, max(limit * 4, limit)),
                    ).fetchall()
                else:
                    rows = conn.execute(
                        """
                        SELECT m.envelope_json
                        FROM memory_fts f
                        JOIN memories m ON m.id = f.id
                        WHERE lower(f.text) LIKE ?
                        LIMIT ?
                        """,
                        (f"%{query.lower()}%", max(limit * 4, limit)),
                    ).fetchall()
            except sqlite3.OperationalError:
                rows = []
        memories = [MemoryEnvelope.from_dict(json.loads(row["envelope_json"])) for row in rows]
        return self._filter(memories, scope=scope, tags=tags)[:limit]

    def list_memories(
        self,
        *,
        scope: dict[str, Any] | None = None,
        tags: list[str] | None = None,
        min_importance: float | None = None,
        limit: int | None = None,
    ) -> list[MemoryEnvelope]:
        clauses: list[str] = []
        params: list[Any] = []
        if min_importance is not None:
            clauses.append("importance >= ?")
            params.append(min_importance)
        where = " WHERE " + " AND ".join(clauses) if clauses else ""
        sql = f"SELECT envelope_json FROM memories{where} ORDER BY importance DESC, created_at DESC"
        if limit is not None:
            sql += " LIMIT ?"
            params.append(limit * 4)
        with self._connection() as conn:
            rows = conn.execute(sql, params).fetchall()
        memories = [MemoryEnvelope.from_dict(json.loads(row["envelope_json"])) for row in rows]
        filtered = self._filter(memories, scope=scope, tags=tags)
        return filtered[:limit] if limit is not None else filtered

    def address_for(self, memory_id: str) -> MobiusAddress:
        with self._connection() as conn:
            row = conn.execute(
                "SELECT iu, iv, iw, region FROM mobius_index WHERE memory_id = ?",
                (memory_id,),
            ).fetchone()
        if row is None:
            raise KeyError(memory_id)
        return MobiusAddress(iu=row["iu"], iv=row["iv"], iw=row["iw"], region=row["region"])

    def clear_indexes(self) -> None:
        if self.db_path.exists():
            self.db_path.unlink()
        self._init_db()

    def by_mobius_neighbors(
        self,
        address: MobiusAddress,
        *,
        depth: int = 1,
        scope: dict[str, Any] | None = None,
        tags: list[str] | None = None,
        limit: int = 20,
    ) -> list[MemoryEnvelope]:
        neighbors = self.mobius_index.neighbors(address, depth=depth)
        keys = sorted({(item.region, item.iu, item.iv, item.iw) for item in neighbors})
        if not keys:
            return []
        placeholders = ",".join(["(?, ?, ?, ?)"] * len(keys))
        params: list[Any] = [value for key in keys for value in key]
        with self._connection() as conn:
            rows = conn.execute(
                f"""
                SELECT m.envelope_json, mi.iu, mi.iv, mi.iw, mi.region
                FROM mobius_index mi
                JOIN memories m ON m.id = mi.memory_id
                WHERE (mi.region, mi.iu, mi.iv, mi.iw) IN ({placeholders})
                """,
                params,
            ).fetchall()
        memories = [MemoryEnvelope.from_dict(json.loads(row["envelope_json"])) for row in rows]
        return self._filter(memories, scope=scope, tags=tags)[:limit]

    def _filter(
        self,
        memories: list[MemoryEnvelope],
        *,
        scope: dict[str, Any] | None = None,
        tags: list[str] | None = None,
    ) -> list[MemoryEnvelope]:
        required_tags = set(tags or [])
        filtered: list[MemoryEnvelope] = []
        for memory in memories:
            if scope and any(memory.scope.get(key) != value for key, value in scope.items()):
                continue
            if required_tags and not required_tags.issubset(set(memory.tags)):
                continue
            filtered.append(memory)
        return filtered

    def _externalize_large_payloads(self, memory: MemoryEnvelope) -> MemoryEnvelope:
        new_modalities: list[ModalityRef] = []
        for item in memory.modalities:
            if item.content is None:
                new_modalities.append(item)
                continue
            data = content_bytes(item.content)
            if len(data) <= self.blob_threshold:
                new_modalities.append(item)
                continue
            sha = item.sha256 or stable_json(item.content)
            if not _is_sha256_hex(sha):
                import hashlib

                sha = hashlib.sha256(data).hexdigest()
            path = self._blob_path(sha)
            path.parent.mkdir(parents=True, exist_ok=True)
            if not path.exists():
                path.write_bytes(data)
            metadata = dict(item.metadata)
            metadata["blob_format"] = "json" if not isinstance(item.content, str) else "text"
            new_modalities.append(
                ModalityRef(
                    modality=item.modality,
                    content=None,
                    uri=f"blob://{sha}",
                    mime_type=item.mime_type,
                    sha256=sha,
                    size_bytes=len(data),
                    metadata=metadata,
                )
            )
        data = memory.to_dict()
        data["modalities"] = [item.to_dict() for item in new_modalities]
        return MemoryEnvelope.from_dict(data)

    def _resolve_blobs(self, memory: MemoryEnvelope) -> MemoryEnvelope:
        modalities: list[ModalityRef] = []
        for item in memory.modalities:
            if not (item.uri and item.uri.startswith("blob://")):
                modalities.append(item)
                continue
            sha = item.uri.removeprefix("blob://")
            data = self._blob_path(sha).read_bytes()
            metadata = dict(item.metadata)
            blob_format = metadata.pop("blob_format", None)
            if blob_format == "json":
                content: Any = json.loads(data.decode("utf-8"))
            else:
                content = data.decode("utf-8")
            modalities.append(
                ModalityRef(
                    modality=item.modality,
                    content=content,
                    uri=None,
                    mime_type=item.mime_type,
                    sha256=item.sha256,
                    size_bytes=item.size_bytes,
                    metadata=metadata,
                )
            )
        data = memory.to_dict()
        data["modalities"] = [item.to_dict() for item in modalities]
        return MemoryEnvelope.from_dict(data)

    def _blob_path(self, sha: str) -> Path:
        # Guard against path traversal: blob shas are always lowercase hex
        # digests, so a crafted uri like blob://../../etc/passwd must not be
        # allowed to escape the blob directory.
        if not _is_sha256_hex(sha):
            raise ValueError("invalid blob reference")
        return self.blob_dir / sha[:2] / sha

    def _fts_expression(self, query: str) -> str:
        tokens = re.findall(r"[\w]+", query.lower(), flags=re.UNICODE)
        expanded = set(tokens)
        if {"now", "latest", "active"} & expanded:
            expanded.add("current")
        if "current" in expanded:
            expanded.add("now")
        if "run" in expanded:
            expanded.update({"command", "commands"})
        if "missing" in expanded:
            expanded.update({"gap", "gaps"})
        if {"gap", "gaps"} & expanded:
            expanded.add("missing")
        tokens = sorted(expanded)
        return " OR ".join(tokens)
