import tempfile
import unittest
from pathlib import Path

from _path import SRC  # noqa: F401
from living_memoryv2.schema import MemoryEnvelope
from living_memoryv2.vault import MemoryVault


class MemoryVaultTests(unittest.TestCase):
    def test_box_context_records_session_scope_and_memories(self):
        with tempfile.TemporaryDirectory() as tmp:
            vault = MemoryVault(Path(tmp))

            with vault.open_box("oauth debug", scope={"project_id": "psi", "user_id": "cinth"}) as box:
                first_id = box.remember_text(
                    "Refresh token failed before retry.",
                    memory_type="error",
                    tags=["oauth", "token"],
                    importance=0.8,
                )
                second_id = box.remember(
                    MemoryEnvelope.text(
                        "Fix: refresh before retry.",
                        scope={"project_id": "psi"},
                        memory_type="solution",
                        tags=["oauth"],
                    )
                )

            info = vault.get_box(box.id)
            memories = vault.memories_in_box(box.id)

            self.assertEqual(info.name, "oauth debug")
            self.assertEqual(info.state, "closed")
            self.assertEqual(info.memory_ids, [first_id, second_id])
            self.assertTrue(all(item.scope["session_id"] == box.id for item in memories))
            self.assertTrue(all(item.scope["project_id"] == "psi" for item in memories))

    def test_vault_integrity_detects_tampered_canonical_jsonl(self):
        with tempfile.TemporaryDirectory() as tmp:
            vault = MemoryVault(Path(tmp))
            memory_id = vault.remember(
                MemoryEnvelope.text("Canonical fact", scope={"project_id": "psi"})
            )

            text = vault.store.jsonl_path.read_text(encoding="utf-8")
            vault.store.jsonl_path.write_text(text.replace("Canonical fact", "Tampered fact"), encoding="utf-8")

            report = vault.verify_integrity()

            self.assertIn(memory_id, report["tampered"])

    def test_rebuild_recovers_store_and_graph_from_canonical_log(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            vault = MemoryVault(root)
            memory = MemoryEnvelope.text(
                "Marina presentation is on May 15.",
                scope={"project_id": "psi"},
                relations=[{"subject": "Marina", "predicate": "has_event", "object": "presentation"}],
            )
            memory_id = vault.remember(memory)

            rebuilt = MemoryVault(root)
            rebuilt.rebuild_indexes()

            self.assertEqual(rebuilt.store.inspect(memory_id).text_content(), "Marina presentation is on May 15.")
            self.assertEqual(rebuilt.graph.query(subject="Marina")[0].memory_id, memory_id)


if __name__ == "__main__":
    unittest.main()
