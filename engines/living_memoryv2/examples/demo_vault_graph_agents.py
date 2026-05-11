from __future__ import annotations

import json
from pathlib import Path
import sys
import tempfile

ROOT = Path(__file__).resolve().parents[1]
SRC = ROOT / "src"
if str(SRC) not in sys.path:
    sys.path.insert(0, str(SRC))

from living_memoryv2 import ContextAssembler, MemoryEnvelope, MemoryVault, ProposalStore, RecallPipeline


def main() -> None:
    with tempfile.TemporaryDirectory() as tmp:
        root = Path(tmp)
        vault = MemoryVault(root / "vault")
        proposals = ProposalStore(root / "proposals.sqlite3")

        with vault.open_box("calendar session", scope={"project_id": "psi", "user_id": "cinth"}) as box:
            box.remember_text(
                "Marina presentation is on May 15.",
                tags=["calendar", "marina"],
                relations=[{"subject": "Marina", "predicate": "has_event", "object": "presentation"}],
                importance=0.8,
            )

        proposal = proposals.propose_memory(
            MemoryEnvelope.text(
                "User preference: keep canonical memory exact and auditable.",
                scope={"project_id": "psi", "user_id": "cinth"},
                memory_type="semantic",
                tags=["preference", "architecture"],
                importance=0.9,
                provenance={"source": "ProfileAgent"},
            ),
            proposal_type="profile_update",
            rationale="Repeated preference against compression and lossy summaries.",
            created_by="ProfileAgent",
        )
        proposals.approve(proposal.id, vault, reviewer="human")

        results = RecallPipeline(vault.store, graph_store=vault.graph).recall(
            "Marina presentation exact memory",
            scope={"project_id": "psi"},
            top_k=5,
        )
        packets = ContextAssembler(max_tokens=180).assemble(results)
        print(json.dumps([packet.to_dict() for packet in packets], indent=2, ensure_ascii=False))


if __name__ == "__main__":
    main()
