import json
import tempfile
import unittest
from pathlib import Path

from _path import SRC  # noqa: F401
from living_memoryv2.benchmarks import (
    main as benchmark_main,
    run_codex_replay_validation,
    run_hard_validation,
    write_report,
)


class HardValidationBenchmarkTests(unittest.TestCase):
    def test_hard_validation_reports_adversarial_weaknesses_without_hiding_failures(self):
        report = run_hard_validation(max_tokens=260, top_k=4, noise_count=180)

        self.assertEqual(report["format"], "living-memoryv2/benchmark-report-1")
        self.assertEqual(report["suite"], "hard_validation_v2")
        self.assertGreaterEqual(report["summary"]["case_count"], 8)
        self.assertIn("weaknesses", report)
        self.assertIn("false_positive_count", report["summary"])
        self.assertIn("false_negative_count", report["summary"])
        self.assertIn("scope_leak_count", report["summary"])
        self.assertIn("stale_hit_count", report["summary"])
        self.assertGreaterEqual(report["summary"]["mean_token_reduction_ratio"], 0.5)

        categories = {case["category"] for case in report["cases"]}
        self.assertIn("prompt_injection", categories)
        self.assertIn("keyword_distractor", categories)
        self.assertIn("future_temporal", categories)
        self.assertIn("budget_stress", categories)

        failing_cases = [case for case in report["cases"] if not case["passed"]]
        self.assertEqual(len(report["weaknesses"]), len(failing_cases))
        for weakness in report["weaknesses"]:
            self.assertIn("case", weakness)
            self.assertIn("failure_modes", weakness)
            self.assertIn("recommended_direction", weakness)

    def test_hard_validation_known_failure_set_is_resolved_at_current_noise_level(self):
        report = run_hard_validation(max_tokens=260, top_k=4, noise_count=180)

        self.assertTrue(report["passes_thresholds"], report["weaknesses"])
        self.assertEqual(report["summary"]["failed"], 0, report["weaknesses"])
        self.assertEqual(report["summary"]["false_negative_count"], 0, report["weaknesses"])
        self.assertEqual(report["summary"]["false_positive_count"], 0, report["weaknesses"])

    def test_codex_replay_validation_uses_real_project_docs(self):
        report = run_codex_replay_validation(max_tokens=360, top_k=5)

        self.assertEqual(report["suite"], "codex_replay_v1")
        self.assertGreaterEqual(report["summary"]["case_count"], 3)
        self.assertTrue(report["passes_thresholds"])
        self.assertGreaterEqual(report["summary"]["accuracy"], 0.66)
        self.assertGreaterEqual(report["summary"]["mean_recall_at_k"], 0.66)
        self.assertGreater(report["summary"]["mean_token_reduction_ratio"], 0.2)
        self.assertTrue(all(case["facade_tokens"] <= case["max_tokens"] for case in report["cases"]))

    def test_cli_can_run_hard_and_replay_suites(self):
        with tempfile.TemporaryDirectory() as tmp:
            hard_output = Path(tmp) / "hard.json"
            replay_output = Path(tmp) / "replay.json"

            hard_exit = benchmark_main(
                ["--suite", "hard", "--output", str(hard_output), "--max-tokens", "240", "--top-k", "4", "--noise-count", "120"]
            )
            replay_exit = benchmark_main(["--suite", "replay", "--output", str(replay_output), "--max-tokens", "360", "--top-k", "5"])

            self.assertIn(hard_exit, {0, 1})
            self.assertEqual(replay_exit, 0)
            self.assertEqual(json.loads(hard_output.read_text(encoding="utf-8"))["suite"], "hard_validation_v2")
            self.assertEqual(json.loads(replay_output.read_text(encoding="utf-8"))["suite"], "codex_replay_v1")

    def test_hard_report_can_be_written(self):
        report = run_hard_validation(max_tokens=240, top_k=4, noise_count=80)
        with tempfile.TemporaryDirectory() as tmp:
            output_path = Path(tmp) / "hard-report.json"
            write_report(report, output_path)
            loaded = json.loads(output_path.read_text(encoding="utf-8"))
            self.assertEqual(loaded["suite"], "hard_validation_v2")


if __name__ == "__main__":
    unittest.main()
