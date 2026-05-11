import tempfile
import unittest
from pathlib import Path

from _path import SRC  # noqa: F401
from living_memoryv2.task_state import TaskStateStore
from living_memoryv2.vault import MemoryVault


class TaskStateStoreTests(unittest.TestCase):
    def test_task_state_round_trips_through_sqlite(self):
        with tempfile.TemporaryDirectory() as tmp:
            store = TaskStateStore(Path(tmp) / "task_state.sqlite3")

            state = store.update(
                "task_alpha",
                scope={"project_id": "psi"},
                patch={
                    "objective": "Ship TaskState without bifurcating living_memoryv2.",
                    "constraints": ["preserve SIF v1", "keep Go shell thin"],
                    "current_plan": ["write tests", "implement canonical store"],
                    "open_subtasks": ["wire facade"],
                    "irreversible_decisions": ["canonical engine is agent-memory/engines/living_memoryv2"],
                    "tool_evidence_ids": ["mem_evidence_1"],
                    "conflict_markers": ["root living_memoryv2 path is deprecated"],
                    "checkpoint_id": "ckpt_design",
                    "checkpoint_sequence": 3,
                    "status": "active",
                },
            )

            reloaded = TaskStateStore(Path(tmp) / "task_state.sqlite3").get("task_alpha")

            self.assertEqual(state.task_id, "task_alpha")
            self.assertEqual(reloaded.objective, "Ship TaskState without bifurcating living_memoryv2.")
            self.assertEqual(reloaded.scope, {"project_id": "psi"})
            self.assertEqual(reloaded.constraints, ["preserve SIF v1", "keep Go shell thin"])
            self.assertEqual(reloaded.current_plan, ["write tests", "implement canonical store"])
            self.assertEqual(reloaded.open_subtasks, ["wire facade"])
            self.assertEqual(reloaded.irreversible_decisions, ["canonical engine is agent-memory/engines/living_memoryv2"])
            self.assertEqual(reloaded.tool_evidence_ids, ["mem_evidence_1"])
            self.assertEqual(reloaded.conflict_markers, ["root living_memoryv2 path is deprecated"])
            self.assertEqual(reloaded.checkpoint_id, "ckpt_design")
            self.assertEqual(reloaded.checkpoint_sequence, 3)

    def test_stale_checkpoint_cannot_overwrite_newer_state(self):
        with tempfile.TemporaryDirectory() as tmp:
            store = TaskStateStore(Path(tmp) / "task_state.sqlite3")
            store.update(
                "task_alpha",
                scope={"project_id": "psi"},
                patch={"objective": "new objective", "checkpoint_sequence": 4},
            )

            with self.assertRaisesRegex(ValueError, "stale checkpoint"):
                store.update(
                    "task_alpha",
                    scope={"project_id": "psi"},
                    patch={"objective": "old objective", "checkpoint_sequence": 2},
                )

            self.assertEqual(store.get("task_alpha").objective, "new objective")
            self.assertEqual(store.get("task_alpha").checkpoint_sequence, 4)

    def test_update_without_explicit_sequence_advances_checkpoint(self):
        with tempfile.TemporaryDirectory() as tmp:
            store = TaskStateStore(Path(tmp) / "task_state.sqlite3")

            first = store.update("task_alpha", patch={"objective": "first"})
            second = store.update("task_alpha", patch={"open_subtasks": ["next"]})

            self.assertEqual(first.checkpoint_sequence, 1)
            self.assertEqual(second.checkpoint_sequence, 2)
            self.assertEqual(second.objective, "first")
            self.assertEqual(second.open_subtasks, ["next"])

    def test_vault_exposes_task_state_store(self):
        with tempfile.TemporaryDirectory() as tmp:
            vault = MemoryVault(Path(tmp))
            vault.task_states.update("task_alpha", patch={"objective": "persist through vault"})

            reloaded = MemoryVault(Path(tmp)).task_states.get("task_alpha")

            self.assertEqual(reloaded.objective, "persist through vault")


if __name__ == "__main__":
    unittest.main()
