import tempfile
import unittest

from _path import SRC  # noqa: F401
from living_memoryv2.consolidation import ConsolidationRunner
from living_memoryv2.mcp_server import LivingMemoryTools


class ConsolidationTests(unittest.TestCase):
    def test_consolidation_proposes_derived_facts_without_writing_them(self):
        with tempfile.TemporaryDirectory() as tmp:
            tools = LivingMemoryTools(tmp)
            source_id = tools.remember_text(
                content="Project: psi_engine_v4_complete\nOwner: Helena\nPolicy should use living_memory facade.",
                scope={"project_id": "psi"},
                memory_type="episodic",
                tags=["meeting-note"],
            )["memory_id"]
            before_count = tools.memory_stats()["memory_count"]

            result = ConsolidationRunner(tools.vault, tools.proposals).propose_from_recent(
                scope={"project_id": "psi"},
                limit=10,
            )

            after_count = tools.memory_stats()["memory_count"]
            self.assertEqual(after_count, before_count)
            self.assertGreaterEqual(result["proposal_count"], 1)
            proposal = tools.proposals.get(result["proposal_ids"][0])
            self.assertEqual(proposal.proposal_type, "derived_fact")
            self.assertEqual(proposal.candidate.provenance["derived_from"], source_id)
            self.assertEqual(proposal.candidate.metadata["review_status"], "pending")


if __name__ == "__main__":
    unittest.main()
