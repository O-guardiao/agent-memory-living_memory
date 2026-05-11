import tempfile
import unittest
from pathlib import Path

from _path import SRC  # noqa: F401
from living_memoryv2.lifecycle import lifecycle_for
from living_memoryv2.mcp_server import LivingMemoryTools
from living_memoryv2.recall import RecallPipeline
from living_memoryv2.schema import MemoryEnvelope
from living_memoryv2.store import TemporalStore


class LifecycleGovernanceTests(unittest.TestCase):
    def test_existing_envelopes_default_to_active_lifecycle(self):
        memory = MemoryEnvelope.text(
            "Old schema memory without lifecycle metadata.",
            scope={"project_id": "psi"},
            created_at=10.0,
        )

        lifecycle = lifecycle_for(memory)

        self.assertEqual(lifecycle.state, "active")
        self.assertEqual(lifecycle.effective_importance(memory.importance), memory.importance)
        self.assertEqual(lifecycle.to_trace(), {"state": "active", "effective_importance": memory.importance})

    def test_superseded_memory_is_preserved_but_current_recall_prefers_active_version(self):
        with tempfile.TemporaryDirectory() as tmp:
            store = TemporalStore(Path(tmp))
            scope = {"project_id": "psi"}
            old = MemoryEnvelope.text(
                "Current memory policy used to prefer filesystem notes as the primary agent memory.",
                scope=scope,
                memory_type="decision",
                tags=["memory", "policy"],
                importance=0.99,
                created_at=10.0,
                metadata={
                    "lifecycle": {
                        "state": "superseded",
                        "superseded_by": "policy-v2",
                        "importance_delta": -0.55,
                    }
                },
            )
            current = MemoryEnvelope.text(
                "Current memory policy: preserve canonical memory and use governed recovery through living_memory.",
                scope=scope,
                memory_type="decision",
                tags=["memory", "policy", "current"],
                importance=0.64,
                created_at=20.0,
                metadata={"lifecycle": {"state": "active", "version": "policy-v2"}},
            )
            store.remember(old)
            store.remember(current)

            results = RecallPipeline(store).recall("current memory policy", scope=scope, top_k=2)

            ids = [item.memory.id for item in results]
            self.assertEqual(ids[0], current.id)
            self.assertIn(old.id, ids)
            old_result = next(item for item in results if item.memory.id == old.id)
            self.assertIn("lifecycle", old_result.why_retrieved)
            self.assertLess(old_result.layer_signals["lifecycle"], old_result.memory.importance)

    def test_historical_query_can_recover_superseded_memory(self):
        with tempfile.TemporaryDirectory() as tmp:
            store = TemporalStore(Path(tmp))
            scope = {"project_id": "psi"}
            old = MemoryEnvelope.text(
                "Previous memory policy: filesystem notes were considered the primary long-term memory.",
                scope=scope,
                memory_type="decision",
                tags=["memory", "policy", "previous"],
                importance=0.7,
                created_at=10.0,
                metadata={"lifecycle": {"state": "superseded", "superseded_by": "policy-v2"}},
            )
            current = MemoryEnvelope.text(
                "Current memory policy: living_memory is the primary governed memory facade.",
                scope=scope,
                memory_type="decision",
                tags=["memory", "policy", "current"],
                importance=0.72,
                created_at=20.0,
                metadata={"lifecycle": {"state": "active", "version": "policy-v2"}},
            )
            store.remember(old)
            store.remember(current)

            results = RecallPipeline(store).recall("previous historical memory policy", scope=scope, top_k=2)

            self.assertEqual(results[0].memory.id, old.id)
            self.assertIn(current.id, [item.memory.id for item in results])

    def test_facade_exposes_compact_lifecycle_trace_without_bloating_sif(self):
        with tempfile.TemporaryDirectory() as tmp:
            tools = LivingMemoryTools(tmp)
            scope = {"project_id": "psi"}
            tools.remember_text(
                content="Previous memory policy: filesystem notes were primary.",
                scope=scope,
                memory_type="decision",
                tags=["memory", "policy", "previous"],
                importance=0.92,
                metadata={"lifecycle": {"state": "superseded", "superseded_by": "policy-v2"}},
            )
            current = tools.remember_text(
                content="Current memory policy: living_memory preserves history and ranks active governed memory first.",
                scope=scope,
                memory_type="decision",
                tags=["memory", "policy", "current"],
                importance=0.66,
                metadata={"lifecycle": {"state": "active", "version": "policy-v2"}},
            )

            result = tools.living_memory(
                query="current memory policy",
                scope=scope,
                top_k=2,
                max_tokens=140,
            )

            self.assertEqual(result["inspectable_ids"][0], current["memory_id"])
            self.assertLessEqual(result["context"]["estimated_tokens"], 140)
            self.assertIn("lifecycle", result["evidence"][0])
            self.assertEqual(result["evidence"][0]["lifecycle"]["state"], "active")
            self.assertIn("effective_importance", result["evidence"][0]["lifecycle"])
            self.assertNotIn("superseded_by", result["context"]["sif_text"])


if __name__ == "__main__":
    unittest.main()
