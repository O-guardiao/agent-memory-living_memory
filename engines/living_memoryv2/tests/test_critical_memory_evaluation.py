import tempfile
import unittest
from pathlib import Path

from _path import SRC  # noqa: F401
from living_memoryv2.context import ContextAssembler
from living_memoryv2.engram import EngramCache
from living_memoryv2.graph import GraphStore
from living_memoryv2.recall import RecallPipeline
from living_memoryv2.schema import MemoryEnvelope
from living_memoryv2.store import TemporalStore
from living_memoryv2.tool_curriculum import ToolCurriculum, ToolSpec, ToolUseTrace
from living_memoryv2.vault import MemoryVault


class CriticalMemoryEvaluationTests(unittest.TestCase):
    def test_scope_isolation_prevents_cross_project_leak_even_with_hot_engram(self):
        with tempfile.TemporaryDirectory() as tmp:
            store = TemporalStore(Path(tmp))
            psi = MemoryEnvelope.text(
                "The deployment key for psi is psi-only.",
                scope={"project_id": "psi"},
                tags=["secret"],
                importance=0.7,
            )
            other = MemoryEnvelope.text(
                "The deployment key for atlas is atlas-only.",
                scope={"project_id": "atlas"},
                tags=["secret"],
                importance=0.7,
            )
            store.remember(psi)
            store.remember(other)
            pipeline = RecallPipeline(store, engram_cache=EngramCache(cache_threshold=1))

            pipeline.recall("deployment key", scope={"project_id": "psi"}, tags=["secret"])
            results = pipeline.recall("deployment key", scope={"project_id": "atlas"}, tags=["secret"])

            ids = [item.memory.id for item in results]
            self.assertIn(other.id, ids)
            self.assertNotIn(psi.id, ids)

    def test_temporal_graph_recall_does_not_boost_expired_conflicting_relation(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            store = TemporalStore(root / "store")
            graph = GraphStore(root / "graph.sqlite3")
            old_owner = MemoryEnvelope.text(
                "Atlas owner was Igor.",
                scope={"project_id": "psi"},
                relations=[
                    {
                        "subject": "Atlas",
                        "predicate": "owner",
                        "object": "Igor",
                        "valid_to": 100.0,
                    }
                ],
                importance=0.7,
                created_at=1.0,
            )
            current_owner = MemoryEnvelope.text(
                "Atlas owner is Marina.",
                scope={"project_id": "psi"},
                relations=[
                    {
                        "subject": "Atlas",
                        "predicate": "owner",
                        "object": "Marina",
                        "valid_from": 101.0,
                    }
                ],
                importance=0.7,
                created_at=2.0,
            )
            for memory in [old_owner, current_owner]:
                store.remember(memory)
                graph.index_memory(memory)

            results = RecallPipeline(store, graph_store=graph).recall(
                "Atlas owner",
                scope={"project_id": "psi"},
                at_time=150.0,
                top_k=5,
            )

            by_id = {item.memory.id: item for item in results}
            self.assertIn(current_owner.id, by_id)
            self.assertIn("graph", by_id[current_owner.id].why_retrieved)
            self.assertNotIn("graph", by_id[old_owner.id].why_retrieved)

    def test_graph_recall_can_walk_two_hops_to_find_indirect_memory(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            store = TemporalStore(root / "store")
            graph = GraphStore(root / "graph.sqlite3")
            event = MemoryEnvelope.text(
                "Marina owns the presentation plan.",
                scope={"project_id": "psi"},
                relations=[{"subject": "Marina", "predicate": "has_event", "object": "presentation"}],
                importance=0.6,
                created_at=1.0,
            )
            topic = MemoryEnvelope.text(
                "The presentation topic is Mobius memory.",
                scope={"project_id": "psi"},
                relations=[{"subject": "presentation", "predicate": "has_topic", "object": "Mobius"}],
                importance=0.6,
                created_at=2.0,
            )
            for memory in [event, topic]:
                store.remember(memory)
                graph.index_memory(memory)

            results = RecallPipeline(store, graph_store=graph).recall(
                "Mobius",
                scope={"project_id": "psi"},
                graph_hops=2,
                top_k=5,
            )

            by_id = {item.memory.id: item for item in results}
            self.assertIn(topic.id, by_id)
            self.assertIn(event.id, by_id)
            self.assertIn("graph", by_id[event.id].why_retrieved)

    def test_vault_rebuild_preserves_exact_large_memory_payload(self):
        with tempfile.TemporaryDirectory() as tmp:
            vault = MemoryVault(Path(tmp))
            exact_text = "raw-memory-start\n" + ("A" * 70_000) + "\nraw-memory-end"
            memory = MemoryEnvelope.text(
                exact_text,
                scope={"project_id": "psi"},
                memory_type="artifact",
                tags=["large", "exact"],
                importance=0.8,
            )
            memory_id = vault.remember(memory)

            vault.rebuild_indexes()

            rebuilt = vault.store.inspect(memory_id)
            self.assertEqual(rebuilt.text_content(), exact_text)
            self.assertEqual(vault.verify_integrity()["tampered"], [])

    def test_context_assembler_keeps_auditable_sources_under_tight_budget(self):
        with tempfile.TemporaryDirectory() as tmp:
            store = TemporalStore(Path(tmp))
            critical = MemoryEnvelope.text(
                "Critical: never compress canonical memory.",
                scope={"project_id": "psi"},
                memory_type="decision",
                provenance={"source": "architecture-note"},
                importance=0.99,
                created_at=2.0,
            )
            noise = MemoryEnvelope.text(
                "noise " * 500,
                scope={"project_id": "psi"},
                provenance={"source": "noise"},
                importance=0.1,
                created_at=1.0,
            )
            store.remember(critical)
            store.remember(noise)
            results = RecallPipeline(store).recall("compress canonical memory", scope={"project_id": "psi"})
            packets = ContextAssembler(max_tokens=16).assemble(results)

            self.assertEqual(packets[0].memory_id, critical.id)
            self.assertEqual(packets[0].source, "architecture-note")
            self.assertIn("content_hash", packets[0].metadata)
            self.assertLessEqual(sum(packet.estimated_tokens for packet in packets), 16)

    def test_tool_failure_becomes_retrievable_procedural_memory(self):
        with tempfile.TemporaryDirectory() as tmp:
            store = TemporalStore(Path(tmp))
            curriculum = ToolCurriculum(
                [ToolSpec("INSPECT", "memory", "inspect a canonical memory by id", ("memory_id",))]
            )
            curriculum.record_trace(
                ToolUseTrace(
                    query="inspect evidence",
                    selected_tools=["INSPECT"],
                    expected_tools=["INSPECT"],
                    success=False,
                    stage="cross_category",
                    error_type="missing_memory_id",
                    feedback="Agent must carry memory_id from recall before inspect.",
                )
            )
            lesson = curriculum.lesson_memories()[0]
            lesson_memory = MemoryEnvelope.structured(
                lesson,
                scope={"project_id": "psi"},
                memory_type="procedural",
                tags=["tool-use", "inspect"],
                entities=["INSPECT"],
                importance=0.85,
                provenance={"source": "tool_curriculum"},
            )
            store.remember(lesson_memory)

            results = RecallPipeline(store).recall(
                "inspect missing memory_id",
                scope={"project_id": "psi"},
            )

            self.assertEqual(results[0].memory.id, lesson_memory.id)
            self.assertIn("missing_memory_id", results[0].memory.text_content())


if __name__ == "__main__":
    unittest.main()
