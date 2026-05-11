import json
from pathlib import Path
import tempfile
import unittest

from _path import SRC  # noqa: F401
from living_memoryv2.benchmarks_external import load_external_cases
from living_memoryv2.calibration import calibration_report


class CalibrationAndExternalBenchmarkTests(unittest.TestCase):
    def test_governance_helpers_are_exported_from_package_surface(self):
        import living_memoryv2

        self.assertIsNotNone(living_memoryv2.FactLedger)
        self.assertIsNotNone(living_memoryv2.RuleBasedFactExtractor)
        self.assertIsNotNone(living_memoryv2.ConnectorIngestor)
        self.assertIsNotNone(living_memoryv2.ConsolidationRunner)
        self.assertIsNotNone(living_memoryv2.calibration_report)
        self.assertIsNotNone(living_memoryv2.load_external_cases)

    def test_calibration_report_returns_ece_risk_coverage_and_unsupported_claims(self):
        cases = [
            {"score": 0.95, "predicted_supported": True, "expected_supported": True},
            {"score": 0.72, "predicted_supported": True, "expected_supported": False},
            {"score": 0.30, "predicted_supported": False, "expected_supported": False},
            {"score": 0.20, "predicted_supported": False, "expected_supported": True},
        ]

        report = calibration_report(cases, bins=2)

        self.assertEqual(report["case_count"], 4)
        self.assertEqual(report["unsupported_claim_count"], 1)
        self.assertGreater(report["ece"], 0.0)
        self.assertEqual(report["risk_coverage"][0]["coverage"], 0.25)
        self.assertIn("accuracy", report["risk_coverage"][0])

    def test_external_fixture_loader_normalizes_locomem_and_longmemeval_shapes(self):
        with tempfile.TemporaryDirectory() as tmp:
            locomo_path = Path(tmp) / "locomo.json"
            long_path = Path(tmp) / "longmemeval.json"
            locomo_path.write_text(
                json.dumps(
                    {
                        "cases": [
                            {
                                "id": "lc-1",
                                "question": "Who owns the policy?",
                                "answer": "Helena",
                                "evidence": ["mem_owner"],
                                "should_answer": True,
                            }
                        ]
                    }
                ),
                encoding="utf-8",
            )
            long_path.write_text(
                json.dumps(
                    [
                        {
                            "question_id": "lm-1",
                            "query": "Current MCP policy?",
                            "gold_answer": "living_memory facade",
                            "supporting_memory_ids": ["mem_policy"],
                            "answerable": True,
                        }
                    ]
                ),
                encoding="utf-8",
            )

            locomo = load_external_cases(locomo_path, suite="locomo")
            longmem = load_external_cases(long_path, suite="longmemeval")

            self.assertEqual(locomo[0].case_id, "lc-1")
            self.assertEqual(locomo[0].expected_answer, "Helena")
            self.assertEqual(locomo[0].evidence_ids, ["mem_owner"])
            self.assertEqual(longmem[0].case_id, "lm-1")
            self.assertEqual(longmem[0].query, "Current MCP policy?")


if __name__ == "__main__":
    unittest.main()
