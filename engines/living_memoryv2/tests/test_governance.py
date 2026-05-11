import tempfile
import unittest

from _path import SRC  # noqa: F401
from living_memoryv2.governance import governance_for, prompt_text_for
from living_memoryv2.mcp_server import LivingMemoryTools
from living_memoryv2.schema import MemoryEnvelope


class GovernanceTests(unittest.TestCase):
    def test_governance_marks_sensitive_memory_as_inspect_only(self):
        memory = MemoryEnvelope.text(
            "Deployment API key is redacted-test-key and must not be injected into prompts.",
            scope={"project_id": "psi"},
            memory_type="credential",
            sensitivity="restricted",
            tags=["deploy"],
            created_at=100.0,
        )

        governance = governance_for(memory)
        prompt_text = prompt_text_for(memory)

        self.assertEqual(governance["prompt_policy"], "inspect_only")
        self.assertIn("credential", governance["safety_labels"])
        self.assertIn("restricted", governance["safety_labels"])
        self.assertIn(memory.id, prompt_text)
        self.assertNotIn("redacted-test-key", prompt_text)

    def test_facade_redacts_sensitive_prompt_text_but_keeps_inspection_path(self):
        with tempfile.TemporaryDirectory() as tmp:
            tools = LivingMemoryTools(tmp)
            scope = {"project_id": "psi"}
            remembered = tools.remember_text(
                content="Deployment API key is redacted-test-key and belongs only in audited inspection.",
                scope=scope,
                memory_type="credential",
                sensitivity="restricted",
                tags=["deploy", "credential"],
                importance=0.95,
            )

            result = tools.living_memory(
                query="deployment api key",
                scope=scope,
                top_k=1,
                max_tokens=120,
            )
            inspect = tools.inspect_memory(memory_id=remembered["memory_id"])

            self.assertEqual(result["status"], "ok")
            self.assertEqual(result["inspectable_ids"], [remembered["memory_id"]])
            self.assertNotIn("redacted-test-key", result["context"]["sif_text"])
            self.assertEqual(result["evidence"][0]["governance"]["prompt_policy"], "inspect_only")
            self.assertIn("credential", result["evidence"][0]["governance"]["safety_labels"])
            self.assertEqual(result["answerability"]["status"], "unsupported")
            self.assertIn("risk_too_high", result["answerability"]["reasons"])
            self.assertIn("redacted-test-key", inspect["text"])
            self.assertEqual(inspect["governance"]["prompt_policy"], "inspect_only")


if __name__ == "__main__":
    unittest.main()
