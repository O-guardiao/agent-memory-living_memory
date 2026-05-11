import tempfile
import unittest
from pathlib import Path

from _path import SRC  # noqa: F401
from living_memoryv2.benchmarks import main as benchmark_main
from living_memoryv2.benchmarks import run_multimodal_validation, write_report


class MultimodalValidationBenchmarkTests(unittest.TestCase):
    def test_multimodal_validation_reports_contract_strengths_and_known_gaps(self):
        report = run_multimodal_validation(max_tokens=260, top_k=4)

        self.assertEqual(report["format"], "living-memoryv2/benchmark-report-1")
        self.assertEqual(report["suite"], "multimodal_validation_v1")
        self.assertGreaterEqual(report["summary"]["case_count"], 8)
        self.assertGreaterEqual(report["summary"]["accuracy"], 0.85)
        self.assertLessEqual(report["summary"]["budget_overrun_count"], 0)
        self.assertTrue(report["passes_thresholds"])

        case_names = {case["name"] for case in report["cases"]}
        self.assertIn("image_caption_ocr_one_call", case_names)
        self.assertIn("document_provenance_one_call", case_names)
        self.assertIn("audio_transcript_one_call", case_names)
        self.assertIn("video_scene_temporal_one_call", case_names)
        self.assertIn("raw_image_minimal_entity_bridge", case_names)
        self.assertIn("vision_only_image_no_text_bridge_abstains", case_names)

        image_case = next(case for case in report["cases"] if case["name"] == "image_caption_ocr_one_call")
        self.assertTrue(image_case["modality_hit"])
        self.assertTrue(image_case["subgraph_hit"])

        prompt_case = next(case for case in report["cases"] if case["name"] == "prompt_injection_visual_distractor")
        self.assertTrue(prompt_case["passed"])
        self.assertEqual(prompt_case["forbidden_hits"], [])

        vision_gap = next(case for case in report["cases"] if case["name"] == "vision_only_image_no_text_bridge_abstains")
        self.assertTrue(vision_gap["passed"])
        self.assertEqual(vision_gap["retrieved_ids"], [])
        self.assertEqual(report["weaknesses"], [])

    def test_multimodal_report_can_be_written_as_json(self):
        report = run_multimodal_validation(max_tokens=240, top_k=4)
        with tempfile.TemporaryDirectory() as tmp:
            output_path = Path(tmp) / "multimodal-report.json"
            write_report(report, output_path)

            self.assertTrue(output_path.exists())

    def test_cli_can_run_multimodal_suite(self):
        with tempfile.TemporaryDirectory() as tmp:
            output_path = Path(tmp) / "multimodal-cli.json"
            exit_code = benchmark_main(["--suite", "multimodal", "--output", str(output_path), "--max-tokens", "240", "--top-k", "4"])

            self.assertEqual(exit_code, 0)
            self.assertTrue(output_path.exists())


if __name__ == "__main__":
    unittest.main()
