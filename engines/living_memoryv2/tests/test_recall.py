import tempfile
import unittest
from pathlib import Path

from _path import SRC  # noqa: F401
from living_memoryv2.graph import GraphStore
from living_memoryv2.mobius import MobiusIndex
from living_memoryv2.recall import RecallPipeline
from living_memoryv2.schema import MemoryEnvelope
from living_memoryv2.store import TemporalStore


class RecallPipelineTests(unittest.TestCase):
    def test_recall_combines_fts_tags_and_critical_memories(self):
        with tempfile.TemporaryDirectory() as tmp:
            store = TemporalStore(Path(tmp))
            target = MemoryEnvelope.text(
                "Fix OAuth refresh token before retrying the request.",
                scope={"project_id": "psi"},
                tags=["oauth", "token"],
                memory_type="solution",
                importance=0.7,
                created_at=10.0,
            )
            critical = MemoryEnvelope.text(
                "Never compress canonical user memories.",
                scope={"project_id": "psi"},
                tags=["architecture"],
                memory_type="decision",
                importance=0.98,
                created_at=11.0,
            )
            unrelated = MemoryEnvelope.text(
                "Grocery list for the weekend.",
                scope={"project_id": "home"},
                tags=["personal"],
                importance=0.4,
                created_at=12.0,
            )
            store.remember(target)
            store.remember(critical)
            store.remember(unrelated)

            results = RecallPipeline(store).recall(
                "refresh token retry",
                scope={"project_id": "psi"},
                top_k=5,
            )

            ids = [item.memory.id for item in results]
            self.assertIn(target.id, ids)
            self.assertIn(critical.id, ids)
            self.assertNotIn(unrelated.id, ids)
            self.assertTrue(any("fts" in item.why_retrieved for item in results if item.memory.id == target.id))
            self.assertTrue(any("mobius" in item.why_retrieved for item in results if item.memory.id == target.id))
            self.assertTrue(any("critical" in item.why_retrieved for item in results if item.memory.id == critical.id))

    def test_tag_filter_keeps_recall_focused(self):
        with tempfile.TemporaryDirectory() as tmp:
            store = TemporalStore(Path(tmp))
            oauth = MemoryEnvelope.text("OAuth token detail", scope={"project_id": "psi"}, tags=["oauth"])
            graph = MemoryEnvelope.text("Graph memory detail", scope={"project_id": "psi"}, tags=["graph"])
            store.remember(oauth)
            store.remember(graph)

            results = RecallPipeline(store).recall("detail", scope={"project_id": "psi"}, tags=["graph"])

            self.assertEqual([item.memory.id for item in results], [graph.id])

    def test_critical_memory_is_not_dropped_by_tight_top_k(self):
        with tempfile.TemporaryDirectory() as tmp:
            store = TemporalStore(Path(tmp))
            critical = MemoryEnvelope.text(
                "Critical: never rewrite canonical memories as summaries.",
                scope={"project_id": "psi"},
                memory_type="decision",
                importance=0.99,
                created_at=1.0,
            )
            lexical = MemoryEnvelope.text(
                "refresh token retry refresh token retry",
                scope={"project_id": "psi"},
                importance=0.4,
                created_at=2.0,
            )
            store.remember(critical)
            store.remember(lexical)

            results = RecallPipeline(store).recall("refresh token retry", scope={"project_id": "psi"}, top_k=1)

            self.assertEqual(results[0].memory.id, critical.id)
            self.assertIn("critical", results[0].why_retrieved)

    def test_mobius_neighbor_validated_by_graph_gets_explicit_bridge_signal(self):
        with tempfile.TemporaryDirectory() as tmp:
            store = TemporalStore(Path(tmp) / "store", mobius_index=MobiusIndex(u_size=1, v_size=2, w_size=1))
            graph = GraphStore(Path(tmp) / "graph.sqlite3")
            seed = MemoryEnvelope.text(
                "Mobius routing coordinates recall layer handoff.",
                scope={"project_id": "psi"},
                tags=["mobius", "routing"],
                memory_type="semantic",
                entities=["mobius", "recall"],
                importance=0.7,
            )
            target = MemoryEnvelope.text(
                "Topology handoff keeps context candidates compact before SIF assembly.",
                scope={"project_id": "psi"},
                tags=["topology", "handoff"],
                memory_type="semantic",
                entities=["mobius", "sif"],
                importance=0.65,
            )
            store.remember(seed)
            store.remember(target)
            graph.add_relation(
                subject="mobius",
                predicate="routes",
                object="recall handoff",
                memory_id=seed.id,
                graph_name="topology",
            )
            graph.add_relation(
                subject="mobius",
                predicate="routes",
                object="sif handoff",
                memory_id=target.id,
                graph_name="topology",
            )

            results = RecallPipeline(store, graph_store=graph).recall(
                "mobius recall handoff",
                scope={"project_id": "psi"},
                top_k=5,
            )

            target_result = next(item for item in results if item.memory.id == target.id)
            self.assertIn("mobius", target_result.why_retrieved)
            self.assertIn("mobius_graph", target_result.why_retrieved)
            self.assertGreater(target_result.layer_signals["mobius_graph"], target_result.layer_signals["mobius"])


if __name__ == "__main__":
    unittest.main()
