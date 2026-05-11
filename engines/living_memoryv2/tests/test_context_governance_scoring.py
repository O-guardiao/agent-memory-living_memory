import unittest

from _path import SRC  # noqa: F401
from living_memoryv2.governance import governance_for
from living_memoryv2.recall import RecallResult
from living_memoryv2.schema import MemoryEnvelope
from living_memoryv2.scoring import score_memory_candidate


class ContextGovernanceScoringTests(unittest.TestCase):
    def test_scoring_prefers_current_supported_memory_over_superseded_memory(self):
        query_terms = {"embedding", "policy", "now"}
        active = RecallResult(
            memory=MemoryEnvelope.text(
                "Current embedding policy keeps embeddings optional.",
                scope={"project_id": "psi"},
                memory_type="decision",
                tags=["current", "embedding", "policy"],
                importance=0.78,
                confidence=0.92,
            ),
            score=0.82,
            confidence=0.88,
            why_retrieved=["fts", "graph"],
            layer_signals={"fts": 0.82, "graph": 0.76},
        )
        superseded = RecallResult(
            memory=MemoryEnvelope.text(
                "Old embedding policy required embeddings for all recall.",
                scope={"project_id": "psi"},
                memory_type="decision",
                tags=["old", "embedding", "policy"],
                importance=0.9,
                confidence=0.95,
                metadata={"lifecycle": {"state": "superseded"}},
            ),
            score=0.86,
            confidence=0.9,
            why_retrieved=["fts"],
            layer_signals={"fts": 0.86},
        )

        active_score = score_memory_candidate(active, query_terms, phase="planning", at_time=100.0)
        superseded_score = score_memory_candidate(superseded, query_terms, phase="planning", at_time=100.0)

        self.assertGreater(active_score.utility, superseded_score.utility)
        self.assertGreater(active_score.features["graph"], 0.0)
        self.assertGreater(superseded_score.penalties["stale"], 0.0)
        self.assertIn("active", active_score.reasons)
        self.assertIn("superseded", superseded_score.reasons)

    def test_scoring_uses_governance_to_penalize_inspect_only_memory(self):
        result = RecallResult(
            memory=MemoryEnvelope.text(
                "Deployment API key is redacted-test-key and belongs only in audited inspection.",
                scope={"project_id": "psi"},
                memory_type="credential",
                sensitivity="restricted",
                tags=["deploy", "credential"],
                importance=0.95,
            ),
            score=0.95,
            confidence=0.9,
            why_retrieved=["fts", "critical"],
            layer_signals={"fts": 0.95, "critical": 0.9},
        )

        score = score_memory_candidate(
            result,
            {"deployment", "api", "key"},
            phase="work_start",
            governance=governance_for(result.memory),
        )

        self.assertGreaterEqual(score.penalties["risk"], 0.7)
        self.assertLess(score.utility, 0.7)
        self.assertIn("risk", score.reasons)


if __name__ == "__main__":
    unittest.main()

