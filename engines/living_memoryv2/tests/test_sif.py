import unittest

from _path import SRC  # noqa: F401
from living_memoryv2.recall import RecallResult
from living_memoryv2.schema import MemoryEnvelope
from living_memoryv2.sif import SIFContextAssembler, SIFContextAssemblerV2


class SIFContextAssemblerTests(unittest.TestCase):
    def test_builds_compact_context_frame_under_budget(self):
        decision = MemoryEnvelope.text(
            "Decision: living_memoryv2 should return compact evidence before full memory inspection.",
            scope={"project_id": "psi"},
            memory_type="decision",
            tags=["context", "mcp"],
            importance=0.98,
            provenance={"source": "unit-test", "path": "notes.md"},
            created_at=2.0,
        )
        background = MemoryEnvelope.text(
            "Background: full documents stay in the vault and should be inspected only by id.",
            scope={"project_id": "psi"},
            tags=["vault"],
            importance=0.4,
            created_at=1.0,
        )
        results = [
            RecallResult(memory=background, score=0.5, confidence=0.6, why_retrieved=["fts"]),
            RecallResult(memory=decision, score=0.9, confidence=0.95, why_retrieved=["critical", "graph"]),
        ]

        frame = SIFContextAssembler(max_tokens=90).assemble("deliver optimized context", results)

        self.assertEqual(frame.format, "lmv2-sif/1")
        self.assertLessEqual(frame.estimated_tokens, 90)
        self.assertEqual(frame.memory_ids[0], decision.id)
        self.assertIn("LMV2_SIF v1", frame.text)
        self.assertIn("[M1]", frame.text)
        self.assertIn(f"id={decision.id}", frame.text)
        self.assertIn("why=critical,graph", frame.text)
        self.assertNotIn('"evidence"', frame.text)
        self.assertNotIn("compress", frame.text.lower())

    def test_truncates_long_text_and_keeps_inspection_hint(self):
        memory = MemoryEnvelope.text(
            "Long context payload. " * 120,
            scope={"project_id": "psi"},
            memory_type="semantic",
            tags=["long"],
            importance=0.9,
            provenance={"source": "large-note.md"},
        )
        result = RecallResult(memory=memory, score=0.8, confidence=0.9, why_retrieved=["fts"])

        frame = SIFContextAssembler(max_tokens=55).assemble("long payload", [result])

        self.assertLessEqual(frame.estimated_tokens, 55)
        self.assertIn(f"inspect_memory({memory.id})", frame.text)
        self.assertIn("...", frame.text)
        self.assertEqual(frame.to_dict()["memory_ids"], [memory.id])

    def test_micro_budget_keeps_id_when_metadata_and_snippet_do_not_fit(self):
        memory = MemoryEnvelope.text(
            "The primary MCP call pattern is living_memory(mode='work') before non-trivial work.",
            scope={"project_id": "psi"},
            memory_type="artifact_section",
            tags=["codex-replay", "agents", "project-memory"],
            importance=0.75,
            provenance={"source": "AGENTS.md", "path": "AGENTS.md"},
        )
        result = RecallResult(memory=memory, score=0.91, confidence=0.95, why_retrieved=["fts", "exact"])

        frame = SIFContextAssembler(max_tokens=40).assemble(
            "What is the primary MCP call pattern for non-trivial work?",
            [result],
        )

        self.assertLessEqual(frame.estimated_tokens, 40)
        self.assertEqual(frame.memory_ids, [memory.id])
        self.assertIn(memory.id, frame.text)
        self.assertEqual(frame.dropped_count, 0)

    def test_sif_v2_builds_structured_sections_without_replacing_v1(self):
        memory = MemoryEnvelope.text(
            "Decision: keep SIF v1 and add SIF v2 structured context.",
            scope={"project_id": "psi"},
            memory_type="decision",
            tags=["decision"],
            importance=0.9,
        )
        result = RecallResult(memory=memory, score=0.88, confidence=0.91, why_retrieved=["fts", "graph"])

        frame = SIFContextAssemblerV2(max_tokens=220).assemble(
            "SIF v2 structured context",
            [result],
            evidence=[
                {
                    "memory_id": memory.id,
                    "memory_type": "decision",
                    "score": 0.88,
                    "confidence": 0.91,
                    "validated_by": ["graph"],
                    "why_retrieved": ["fts", "graph"],
                    "tags": ["decision"],
                    "hint": "Decision: keep SIF v1 and add SIF v2 structured context.",
                    "governance": {"prompt_policy": "inline"},
                }
            ],
            answerability={"status": "supported", "score": 0.9, "reasons": []},
            knowledge={
                "assertions": [
                    {
                        "assertion_id": "kg_1",
                        "fact_id": "fact_1",
                        "subject": "SIF v2",
                        "predicate": "extends",
                        "object": "SIF v1",
                        "source_memory_id": memory.id,
                        "status": "active",
                        "confidence": 0.9,
                    }
                ],
                "entities": [],
            },
        )

        data = frame.to_dict()
        self.assertEqual(data["format"], "lmv2-sif/2")
        self.assertEqual(data["memory_ids"], [memory.id])
        self.assertEqual(data["sections"]["answerability"]["status"], "supported")
        self.assertEqual(data["sections"]["current_facts"][0]["subject"], "SIF v2")
        self.assertEqual(data["sections"]["decisions"][0]["memory_id"], memory.id)
        self.assertIn(memory.id, data["sections"]["inspectable_ids"])


if __name__ == "__main__":
    unittest.main()
