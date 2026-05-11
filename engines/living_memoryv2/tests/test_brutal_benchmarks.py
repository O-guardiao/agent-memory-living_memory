import unittest

from _path import SRC  # noqa: F401
from living_memoryv2.benchmarks import run_codex_replay_validation


class BrutalBenchmarkTests(unittest.TestCase):
    def test_tight_replay_top1_still_finds_current_metrics_summary(self):
        report = run_codex_replay_validation(max_tokens=80, top_k=1)

        failures = {case["name"]: case for case in report["cases"] if not case["passed"]}

        self.assertNotIn("replay_current_validation_metrics", failures)
        self.assertEqual(report["summary"]["failed"], 0, report.get("weaknesses"))


if __name__ == "__main__":
    unittest.main()
