import json
import tempfile
import unittest
from pathlib import Path

from _path import SRC  # noqa: F401
from living_memoryv2.benchmarks import main as benchmark_main
from living_memoryv2.benchmarks import run_local_validation, write_report


class LocalValidationBenchmarkTests(unittest.TestCase):
    def test_local_validation_report_contains_release_relevant_metrics(self):
        report = run_local_validation(max_tokens=420, top_k=6)

        self.assertEqual(report["format"], "living-memoryv2/benchmark-report-1")
        self.assertEqual(report["suite"], "local_validation_v1")
        self.assertGreaterEqual(report["summary"]["case_count"], 6)
        self.assertGreaterEqual(report["summary"]["accuracy"], 0.8)
        self.assertGreaterEqual(report["summary"]["mean_recall_at_k"], 0.8)
        self.assertGreater(report["summary"]["mean_token_reduction_ratio"], 0.2)
        self.assertLess(report["summary"]["mean_latency_ms"], 250)
        self.assertTrue(report["passes_thresholds"])

        case_names = {case["name"] for case in report["cases"]}
        self.assertIn("scope_isolation_project_boundary", case_names)
        self.assertIn("procedural_lesson_from_tool_failure", case_names)
        self.assertIn("abstention_unrelated_query", case_names)

        for case in report["cases"]:
            self.assertLessEqual(case["facade_tokens"], case["max_tokens"])

        scope_case = next(case for case in report["cases"] if case["name"] == "scope_isolation_project_boundary")
        self.assertEqual(scope_case["forbidden_hits"], [])

        temporal_case = next(case for case in report["cases"] if case["name"] == "temporal_current_policy")
        self.assertEqual(temporal_case["forbidden_hits"], [])
        self.assertTrue(temporal_case["passed"])

        abstention_case = next(case for case in report["cases"] if case["name"] == "abstention_unrelated_query")
        self.assertEqual(abstention_case["retrieved_ids"], [])
        self.assertTrue(abstention_case["passed"])

    def test_report_can_be_written_as_reproducible_json(self):
        report = run_local_validation(max_tokens=360, top_k=5)
        with tempfile.TemporaryDirectory() as tmp:
            output_path = Path(tmp) / "report.json"
            write_report(report, output_path)

            loaded = json.loads(output_path.read_text(encoding="utf-8"))
            self.assertEqual(loaded["format"], "living-memoryv2/benchmark-report-1")
            self.assertEqual(loaded["summary"]["case_count"], report["summary"]["case_count"])

    def test_cli_writes_report(self):
        with tempfile.TemporaryDirectory() as tmp:
            output_path = Path(tmp) / "cli-report.json"
            exit_code = benchmark_main(["--output", str(output_path), "--max-tokens", "360", "--top-k", "5"])

            self.assertEqual(exit_code, 0)
            self.assertTrue(output_path.exists())
            loaded = json.loads(output_path.read_text(encoding="utf-8"))
            self.assertTrue(loaded["passes_thresholds"])


if __name__ == "__main__":
    unittest.main()
