import tempfile
import unittest
from pathlib import Path

from _path import SRC  # noqa: F401
from living_memoryv2.agents import ProposalStore
from living_memoryv2.recall import RecallPipeline
from living_memoryv2.schema import MemoryEnvelope
from living_memoryv2.vault import MemoryVault


class ProposalStoreTests(unittest.TestCase):
    def test_agent_proposal_does_not_write_to_vault_until_approved(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            vault = MemoryVault(root / "vault")
            proposals = ProposalStore(root / "proposals.sqlite3")
            candidate = MemoryEnvelope.text(
                "User prefers exact memory over summaries.",
                scope={"project_id": "psi"},
                memory_type="semantic",
                tags=["preference"],
                provenance={"source": "agent"},
            )

            proposal = proposals.propose_memory(
                candidate,
                proposal_type="profile_update",
                rationale="Preference inferred from repeated instruction.",
                created_by="ProfileAgent",
            )
            before = RecallPipeline(vault.store).recall("exact memory summaries", scope={"project_id": "psi"})
            memory_id = proposals.approve(proposal.id, vault, reviewer="human")
            after = RecallPipeline(vault.store).recall("exact memory summaries", scope={"project_id": "psi"})

            self.assertEqual(before, [])
            self.assertEqual(after[0].memory.id, memory_id)
            self.assertEqual(proposals.get(proposal.id).status, "approved")
            self.assertEqual(vault.store.inspect(memory_id).provenance["proposal_id"], proposal.id)

    def test_rejected_proposal_never_enters_vault(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            vault = MemoryVault(root / "vault")
            proposals = ProposalStore(root / "proposals.sqlite3")
            proposal = proposals.propose_memory(
                MemoryEnvelope.text("Rejected fact", scope={"project_id": "psi"}),
                proposal_type="extraction",
                rationale="Weak signal.",
                created_by="ExtractorAgent",
            )

            proposals.reject(proposal.id, reviewer="human", reason="Not enough evidence.")

            self.assertEqual(proposals.get(proposal.id).status, "rejected")
            self.assertEqual(vault.store.list_memories(), [])


if __name__ == "__main__":
    unittest.main()
