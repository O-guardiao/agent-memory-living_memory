import tempfile
import unittest

from _path import SRC  # noqa: F401
from living_memoryv2.mcp_server import LivingMemoryTools


class FacadeAbstentionTests(unittest.TestCase):
    def test_facade_abstains_when_terms_are_similar_but_no_single_memory_is_relevant(self):
        with tempfile.TemporaryDirectory() as tmp:
            tools = LivingMemoryTools(tmp)
            scope = {"project_id": "psi"}
            tools.remember_text(
                content="Helena is the local-first review workflow for project memory.",
                scope=scope,
                memory_type="semantic",
                tags=["helena", "workflow"],
                importance=0.7,
            )
            tools.remember_text(
                content="TurboQuant is only a future reference for KV-cache compression.",
                scope=scope,
                memory_type="decision",
                tags=["turboquant", "compression"],
                importance=0.7,
            )

            result = tools.living_memory(
                query="Helena TurboQuant payroll reimbursement city policy",
                scope=scope,
                max_tokens=220,
                top_k=4,
            )

            self.assertEqual(result["inspectable_ids"], [])
            self.assertEqual(result["evidence"], [])
            self.assertEqual(result["context"]["memory_ids"], [])
            self.assertIn("abstained", result["status"])

    def test_facade_abstains_when_one_noise_memory_covers_many_query_terms(self):
        with tempfile.TemporaryDirectory() as tmp:
            tools = LivingMemoryTools(tmp)
            scope = {"project_id": "psi"}
            tools.remember_text(
                content=(
                    "Hard noise record. Helena workflow draft with unrelated reimbursement city "
                    "payroll terms and no project decision. This memory intentionally collides "
                    "with benchmark vocabulary but lacks the canonical answer."
                ),
                scope=scope,
                memory_type="episodic",
                tags=["noise", "policy", "context"],
                importance=0.7,
            )

            result = tools.living_memory(
                query="Helena TurboQuant payroll reimbursement city policy",
                scope=scope,
                max_tokens=220,
                top_k=4,
            )

            self.assertEqual(result["inspectable_ids"], [])
            self.assertEqual(result["evidence"], [])
            self.assertIn("abstained", result["status"])

    def test_facade_keeps_direct_match_under_abstention_filter(self):
        with tempfile.TemporaryDirectory() as tmp:
            tools = LivingMemoryTools(tmp)
            scope = {"project_id": "psi"}
            remembered = tools.remember_text(
                content="Current embedding policy: embeddings are optional derived indexes, not required for base recall.",
                scope=scope,
                memory_type="decision",
                tags=["current", "embedding", "policy"],
                importance=0.78,
            )

            result = tools.living_memory(
                query="embedding default policy now",
                scope=scope,
                max_tokens=220,
                top_k=4,
            )

            self.assertIn(remembered["memory_id"], result["inspectable_ids"])
            self.assertEqual(result["status"], "ok")

    def test_facade_prefers_current_temporal_policy_over_future_plan_and_noise(self):
        with tempfile.TemporaryDirectory() as tmp:
            tools = LivingMemoryTools(tmp)
            scope = {"project_id": "psi"}
            now = 5000.0
            future = tools.remember_text(
                content="Future plan: starting next year, enable embeddings by default for every retrieval.",
                scope=scope,
                memory_type="decision",
                tags=["future", "embedding", "policy"],
                relations=[
                    {
                        "subject": "embedding_policy",
                        "predicate": "default",
                        "object": "enabled",
                        "valid_from": now + 1000,
                    }
                ],
                importance=0.9,
            )
            current = tools.remember_text(
                content="Current plan: embeddings are optional derived indexes; base retrieval works without models.",
                scope=scope,
                memory_type="decision",
                tags=["current", "embedding", "policy"],
                relations=[
                    {
                        "subject": "embedding_policy",
                        "predicate": "default",
                        "object": "optional",
                        "valid_from": now - 1000,
                    }
                ],
                importance=0.82,
            )
            tools.remember_text(
                content="embedding default experiment scratchpad for future model-only prototypes",
                scope=scope,
                memory_type="episodic",
                tags=["noise", "policy", "context"],
                importance=0.75,
            )

            result = tools.living_memory(
                query="embedding default policy now",
                scope=scope,
                max_tokens=220,
                top_k=4,
                at_time=now,
            )

            self.assertIn(current["memory_id"], result["inspectable_ids"])
            self.assertNotIn(future["memory_id"], result["inspectable_ids"])
            self.assertEqual(result["status"], "ok")


if __name__ == "__main__":
    unittest.main()
