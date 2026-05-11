import unittest

from _path import SRC  # noqa: F401
from living_memoryv2.answerability import assess_answerability
from living_memoryv2.scoring import MemoryFeatureScore


class AnswerabilityTests(unittest.TestCase):
    def test_answerability_marks_supported_pack_when_utility_and_coverage_are_high(self):
        assessment = assess_answerability(
            {"embedding", "policy", "now"},
            [
                MemoryFeatureScore(
                    memory_id="mem_active",
                    utility=0.82,
                    features={"lexical": 0.67, "graph": 0.85, "time": 0.9, "confidence": 0.92},
                    penalties={"risk": 0.0, "stale": 0.0},
                    reasons=["active", "graph_supported"],
                )
            ],
            phase="planning",
        )

        self.assertEqual(assessment.status, "supported")
        self.assertGreaterEqual(assessment.score, 0.35)
        self.assertIn("sufficient_support", assessment.reasons)

    def test_answerability_abstains_when_pack_is_empty(self):
        assessment = assess_answerability({"embedding", "policy"}, [], phase="work_start")

        self.assertEqual(assessment.status, "unsupported")
        self.assertEqual(assessment.score, 0.0)
        self.assertIn("no_evidence", assessment.reasons)

    def test_answerability_abstains_when_governance_risk_is_high(self):
        assessment = assess_answerability(
            {"deployment", "api", "key"},
            [
                MemoryFeatureScore(
                    memory_id="mem_risky",
                    utility=0.58,
                    features={"lexical": 1.0, "confidence": 0.9},
                    penalties={"risk": 0.75, "stale": 0.0},
                    reasons=["active", "risk"],
                )
            ],
            phase="work_start",
        )

        self.assertEqual(assessment.status, "unsupported")
        self.assertIn("risk_too_high", assessment.reasons)


if __name__ == "__main__":
    unittest.main()

