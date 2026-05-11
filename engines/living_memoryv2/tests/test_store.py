import tempfile
import unittest
from pathlib import Path

from _path import SRC  # noqa: F401
from living_memoryv2.mobius import MobiusIndex
from living_memoryv2.schema import MemoryEnvelope
from living_memoryv2.store import TemporalStore


class TemporalStoreTests(unittest.TestCase):
    def test_remember_and_inspect_return_exact_text(self):
        with tempfile.TemporaryDirectory() as tmp:
            store = TemporalStore(Path(tmp))
            memory = MemoryEnvelope.text(
                "Marina presentation is on May 15.",
                scope={"project_id": "psi", "user_id": "cinth"},
                tags=["calendar", "marina"],
                importance=0.8,
                created_at=100.0,
            )

            memory_id = store.remember(memory)
            restored = store.inspect(memory_id)

            self.assertEqual(restored.text_content(), "Marina presentation is on May 15.")
            self.assertEqual(restored.scope["project_id"], "psi")
            self.assertEqual(restored.content_hash, memory.content_hash)

    def test_large_inline_text_is_stored_as_resolvable_blob(self):
        with tempfile.TemporaryDirectory() as tmp:
            store = TemporalStore(Path(tmp), blob_threshold=32)
            large_text = "alpha memory " * 20
            memory_id = store.remember(
                MemoryEnvelope.text(
                    large_text,
                    scope={"project_id": "psi"},
                    created_at=101.0,
                )
            )

            raw = store.inspect(memory_id, resolve_blobs=False)
            restored = store.inspect(memory_id, resolve_blobs=True)

            self.assertTrue(raw.modalities[0].uri.startswith("blob://"))
            self.assertIsNone(raw.modalities[0].content)
            self.assertEqual(restored.text_content(), large_text)

    def test_text_search_respects_scope_and_tags(self):
        with tempfile.TemporaryDirectory() as tmp:
            store = TemporalStore(Path(tmp))
            first = MemoryEnvelope.text(
                "OAuth refresh token fix",
                scope={"project_id": "psi"},
                tags=["oauth"],
                created_at=1.0,
            )
            second = MemoryEnvelope.text(
                "Budget report and invoice",
                scope={"project_id": "finance"},
                tags=["money"],
                created_at=2.0,
            )
            store.remember(first)
            store.remember(second)

            results = store.search_text("refresh token", scope={"project_id": "psi"}, tags=["oauth"])

            self.assertEqual([item.id for item in results], [first.id])

    def test_persisted_envelope_does_not_add_summary_or_compression_fields(self):
        with tempfile.TemporaryDirectory() as tmp:
            store = TemporalStore(Path(tmp))
            store.remember(
                MemoryEnvelope.text(
                    "Canonical memory remains exact.",
                    scope={"project_id": "psi"},
                    provenance={"source": "unit-test"},
                )
            )

            persisted = store.jsonl_path.read_text(encoding="utf-8").lower()

            self.assertNotIn("compressed", persisted)
            self.assertNotIn("summary", persisted)
            self.assertIn("canonical memory remains exact", persisted)

    def test_mobius_neighbor_lookup_has_composite_coordinate_index(self):
        with tempfile.TemporaryDirectory() as tmp:
            store = TemporalStore(Path(tmp), mobius_index=MobiusIndex(u_size=8, v_size=4, w_size=3))
            store.remember(
                MemoryEnvelope.text(
                    "Indexed Mobius route",
                    scope={"project_id": "psi"},
                    tags=["routing"],
                    created_at=5.0,
                )
            )

            with store._connection() as conn:
                indexes = {
                    row["name"]
                    for row in conn.execute("PRAGMA index_list(mobius_index)").fetchall()
                }

            self.assertIn("idx_mobius_region_coords", indexes)


if __name__ == "__main__":
    unittest.main()
