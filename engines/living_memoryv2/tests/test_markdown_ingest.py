import tempfile
import unittest

from _path import SRC  # noqa: F401
from living_memoryv2.ingest import split_markdown_sections
from living_memoryv2.mcp_server import LivingMemoryTools


class MarkdownIngestTests(unittest.TestCase):
    def test_split_markdown_sections_keeps_heading_path_and_source_metadata(self):
        text = """# Product Memory

Intro text stays with the first section.

## Benchmarks

Run local validation with living_memoryv2.benchmarks.

### Hard Validation

Hard validation keeps failures as weaknesses.

## Release Readiness

Do not claim production readiness before public benchmarks.
"""
        sections = split_markdown_sections(text, source_path="docs/product.md", max_chars=240)

        self.assertGreaterEqual(len(sections), 4)
        self.assertEqual(sections[0].heading_path, ["Product Memory"])
        self.assertEqual(sections[1].heading_path, ["Product Memory", "Benchmarks"])
        self.assertEqual(sections[2].heading_path, ["Product Memory", "Benchmarks", "Hard Validation"])
        self.assertEqual(sections[1].source_path, "docs/product.md")
        self.assertIn("Run local validation", sections[1].content)
        self.assertLessEqual(max(len(section.content) for section in sections), 240)

    def test_remember_markdown_sections_retrieves_specific_section(self):
        text = """# Living Memory

## Running Benchmarks

Use python -m living_memoryv2.benchmarks to generate JSON reports.

## Release Gaps

Public LoCoMo and LongMemEval comparisons are still missing.
"""
        with tempfile.TemporaryDirectory() as tmp:
            tools = LivingMemoryTools(tmp)
            result = tools.remember_markdown(
                content=text,
                source_path="docs/validation.md",
                scope={"project_id": "psi"},
                tags=["docs"],
                max_section_chars=500,
            )

            self.assertEqual(result["section_count"], 3)
            replay = tools.living_memory(
                query="How do I generate JSON benchmark reports?",
                scope={"project_id": "psi"},
                max_tokens=180,
                top_k=3,
            )
            section_ids = set(result["section_memory_ids"])
            self.assertTrue(section_ids & set(replay["inspectable_ids"]))
            evidence_text = " ".join(item["hint"] for item in replay["evidence"])
            self.assertIn("living_memoryv2.benchmarks", evidence_text)


if __name__ == "__main__":
    unittest.main()
