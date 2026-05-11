from __future__ import annotations

from contextlib import contextmanager
from dataclasses import dataclass
import json
from pathlib import Path
import sqlite3
import time
from typing import Iterator
import uuid

from .schema import MemoryEnvelope
from .vault import MemoryVault


@dataclass(frozen=True)
class MemoryProposal:
    id: str
    proposal_type: str
    candidate: MemoryEnvelope
    rationale: str
    created_by: str
    created_at: float
    status: str
    reviewed_by: str | None = None
    reviewed_at: float | None = None
    review_reason: str | None = None
    memory_id: str | None = None


class ProposalStore:
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
                CREATE TABLE IF NOT EXISTS proposals (
                    id TEXT PRIMARY KEY,
                    proposal_type TEXT NOT NULL,
                    candidate_json TEXT NOT NULL,
                    rationale TEXT NOT NULL,
                    created_by TEXT NOT NULL,
                    created_at REAL NOT NULL,
                    status TEXT NOT NULL,
                    reviewed_by TEXT,
                    reviewed_at REAL,
                    review_reason TEXT,
                    memory_id TEXT
                )
                """
            )

    def propose_memory(
        self,
        candidate: MemoryEnvelope,
        *,
        proposal_type: str,
        rationale: str,
        created_by: str,
    ) -> MemoryProposal:
        proposal = MemoryProposal(
            id="prop_" + uuid.uuid4().hex[:20],
            proposal_type=proposal_type,
            candidate=candidate,
            rationale=rationale,
            created_by=created_by,
            created_at=time.time(),
            status="pending",
        )
        with self._connection() as conn:
            conn.execute(
                """
                INSERT INTO proposals (
                    id, proposal_type, candidate_json, rationale, created_by,
                    created_at, status, reviewed_by, reviewed_at, review_reason, memory_id
                )
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    proposal.id,
                    proposal.proposal_type,
                    json.dumps(proposal.candidate.to_dict(), ensure_ascii=False, sort_keys=True),
                    proposal.rationale,
                    proposal.created_by,
                    proposal.created_at,
                    proposal.status,
                    None,
                    None,
                    None,
                    None,
                ),
            )
        return proposal

    def get(self, proposal_id: str) -> MemoryProposal:
        with self._connection() as conn:
            row = conn.execute("SELECT * FROM proposals WHERE id = ?", (proposal_id,)).fetchone()
        if row is None:
            raise KeyError(proposal_id)
        return MemoryProposal(
            id=row["id"],
            proposal_type=row["proposal_type"],
            candidate=MemoryEnvelope.from_dict(json.loads(row["candidate_json"])),
            rationale=row["rationale"],
            created_by=row["created_by"],
            created_at=row["created_at"],
            status=row["status"],
            reviewed_by=row["reviewed_by"],
            reviewed_at=row["reviewed_at"],
            review_reason=row["review_reason"],
            memory_id=row["memory_id"],
        )

    def approve(self, proposal_id: str, vault: MemoryVault, *, reviewer: str) -> str:
        proposal = self.get(proposal_id)
        if proposal.status != "pending":
            raise ValueError(f"proposal {proposal_id} is not pending")
        data = proposal.candidate.to_dict()
        provenance = dict(data.get("provenance") or {})
        provenance.update(
            {
                "proposal_id": proposal.id,
                "proposal_type": proposal.proposal_type,
                "proposed_by": proposal.created_by,
                "approved_by": reviewer,
            }
        )
        data["provenance"] = provenance
        memory = MemoryEnvelope.from_dict(data)
        memory_id = vault.remember(memory)
        self._review(proposal_id, status="approved", reviewer=reviewer, reason=None, memory_id=memory_id)
        return memory_id

    def reject(self, proposal_id: str, *, reviewer: str, reason: str) -> None:
        proposal = self.get(proposal_id)
        if proposal.status != "pending":
            raise ValueError(f"proposal {proposal_id} is not pending")
        self._review(proposal_id, status="rejected", reviewer=reviewer, reason=reason, memory_id=None)

    def _review(
        self,
        proposal_id: str,
        *,
        status: str,
        reviewer: str,
        reason: str | None,
        memory_id: str | None,
    ) -> None:
        with self._connection() as conn:
            conn.execute(
                """
                UPDATE proposals
                SET status = ?, reviewed_by = ?, reviewed_at = ?, review_reason = ?, memory_id = ?
                WHERE id = ?
                """,
                (status, reviewer, time.time(), reason, memory_id, proposal_id),
            )
