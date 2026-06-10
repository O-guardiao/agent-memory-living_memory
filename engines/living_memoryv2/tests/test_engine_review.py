import tempfile
import unittest
from pathlib import Path

from _path import SRC  # noqa: F401
from living_memoryv2.agents import ProposalStore
from living_memoryv2.lifecycle import lifecycle_for
from living_memoryv2.schema import MemoryEnvelope, ModalityRef
from living_memoryv2.store import TemporalStore
from living_memoryv2.vault import MemoryVault


class EngineReviewRegressionTests(unittest.TestCase):
    def test_lifecycle_for_tolerates_non_integer_usage_count(self):
        # Malformed metadata must not raise; previously int("many") crashed.
        memory = MemoryEnvelope.text(
            "x",
            scope={},
            metadata={"lifecycle": {"usage_count": "many"}},
        )
        view = lifecycle_for(memory)
        self.assertEqual(view.usage_count, 0)

    def test_lifecycle_for_parses_float_string_usage_count(self):
        memory = MemoryEnvelope.text(
            "x",
            scope={},
            metadata={"lifecycle": {"usage_count": "3.0"}},
        )
        self.assertEqual(lifecycle_for(memory).usage_count, 3)

    def test_blob_reference_path_traversal_is_blocked(self):
        with tempfile.TemporaryDirectory() as tmp:
            store = TemporalStore(Path(tmp) / "store")
            evil = MemoryEnvelope(
                scope={},
                memory_type="episodic",
                modalities=[ModalityRef("text", uri="blob://../../secret.txt")],
            )
            memory_id = store.remember(evil)
            with self.assertRaises(ValueError):
                store.inspect(memory_id)

    def test_legitimate_large_blob_still_round_trips(self):
        with tempfile.TemporaryDirectory() as tmp:
            store = TemporalStore(Path(tmp) / "store")
            big = "B" * (70 * 1024)
            memory_id = store.remember(MemoryEnvelope.text(big, scope={}))
            self.assertEqual(store.inspect(memory_id).text_content(), big)

    def test_box_scoped_memory_is_not_flagged_as_tampered(self):
        with tempfile.TemporaryDirectory() as tmp:
            vault = MemoryVault(Path(tmp))
            with vault.open_box("session", scope={"project_id": "p1"}) as box:
                box.remember_text("box scoped note about quarks")
            report = vault.verify_integrity()
            self.assertEqual(report["tampered"], [])
            self.assertEqual(report["checked"], 1)

    def test_approved_proposal_memory_is_not_flagged_as_tampered(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            vault = MemoryVault(root)
            proposals = ProposalStore(root / "proposals.sqlite3")
            candidate = MemoryEnvelope.text("proposal content xyz", scope={"project_id": "p1"})
            proposal = proposals.propose_memory(
                candidate,
                proposal_type="t",
                rationale="r",
                created_by="me",
            )
            proposals.approve(proposal.id, vault, reviewer="rev")
            self.assertEqual(vault.verify_integrity()["tampered"], [])


if __name__ == "__main__":
    unittest.main()
