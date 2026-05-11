import unittest

from _path import SRC  # noqa: F401
from living_memoryv2.schema import MemoryEnvelope, ModalityRef


class MemoryEnvelopeTests(unittest.TestCase):
    def test_text_memory_round_trips_without_compression_fields(self):
        memory = MemoryEnvelope.text(
            "OAuth refresh token expires before retry.",
            scope={"project_id": "psi", "user_id": "cinth"},
            memory_type="error",
            tags=["oauth", "token"],
            entities=["refresh_token"],
            importance=0.92,
            confidence=0.88,
            provenance={"source": "unit-test"},
            created_at=1000.0,
        )

        restored = MemoryEnvelope.from_dict(memory.to_dict())

        self.assertEqual(restored.to_dict(), memory.to_dict())
        self.assertEqual(restored.text_content(), "OAuth refresh token expires before retry.")
        serialized = str(restored.to_dict()).lower()
        self.assertNotIn("compress", serialized)
        self.assertTrue(restored.content_hash)

    def test_file_reference_and_structured_memory_keep_modalities(self):
        file_memory = MemoryEnvelope.file_ref(
            uri="file:///tmp/design.png",
            mime_type="image/png",
            scope={"project_id": "psi"},
            tags=["diagram"],
            created_at=2000.0,
        )
        structured = MemoryEnvelope.structured(
            {"decision": "no compression", "phase": 1},
            scope={"project_id": "psi"},
            memory_type="decision",
            created_at=2001.0,
        )

        self.assertEqual(file_memory.modalities[0].modality, "file")
        self.assertEqual(file_memory.modalities[0].uri, "file:///tmp/design.png")
        self.assertIn("no compression", structured.text_content())
        self.assertIsInstance(ModalityRef.from_dict(file_memory.modalities[0].to_dict()), ModalityRef)


if __name__ == "__main__":
    unittest.main()
