import unittest

from _path import SRC  # noqa: F401
from living_memoryv2.context import ContextAssembler
from living_memoryv2.recall import RecallResult
from living_memoryv2.schema import MemoryEnvelope


class ContextAssemblerTests(unittest.TestCase):
    def test_assembler_respects_budget_without_summarizing(self):
        high = MemoryEnvelope.text(
            "Critical architecture decision: keep canonical memory exact.",
            scope={"project_id": "psi"},
            memory_type="decision",
            importance=0.99,
            created_at=2.0,
        )
        low = MemoryEnvelope.text(
            "A very long lower priority memory. " * 50,
            scope={"project_id": "psi"},
            importance=0.2,
            created_at=1.0,
        )
        results = [
            RecallResult(memory=low, score=0.4, confidence=0.4, why_retrieved=["fts"]),
            RecallResult(memory=high, score=0.9, confidence=0.9, why_retrieved=["critical"]),
        ]

        packets = ContextAssembler(max_tokens=40).assemble(results)

        self.assertEqual(packets[0].memory_id, high.id)
        payload = [packet.to_dict() for packet in packets]
        self.assertNotIn("summary", str(payload).lower())
        self.assertNotIn("compress", str(payload).lower())
        self.assertLessEqual(sum(packet.estimated_tokens for packet in packets), 40)

    def test_packet_exposes_source_provenance_for_audit(self):
        memory = MemoryEnvelope.text(
            "Remember exact source in evidence packets.",
            scope={"project_id": "psi"},
            provenance={"source": "unit-test", "tool": "unittest"},
            created_at=3.0,
        )
        result = RecallResult(memory=memory, score=0.8, confidence=0.9, why_retrieved=["fts"])

        packet = ContextAssembler(max_tokens=80).assemble([result])[0].to_dict()

        self.assertEqual(packet["source"], "unit-test")
        self.assertEqual(packet["provenance"]["tool"], "unittest")


if __name__ == "__main__":
    unittest.main()
