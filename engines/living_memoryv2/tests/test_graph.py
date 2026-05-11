import tempfile
import unittest
from pathlib import Path

from _path import SRC  # noqa: F401
from living_memoryv2.graph import GraphStore
from living_memoryv2.recall import RecallPipeline
from living_memoryv2.schema import MemoryEnvelope
from living_memoryv2.store import TemporalStore


class GraphStoreTests(unittest.TestCase):
    def test_indexes_memory_relations_with_temporal_validity(self):
        with tempfile.TemporaryDirectory() as tmp:
            graph = GraphStore(Path(tmp) / "graph.sqlite3")
            memory = MemoryEnvelope.text(
                "Marina presentation is on May 15.",
                scope={"project_id": "psi"},
                relations=[
                    {
                        "subject": "Marina",
                        "predicate": "has_event",
                        "object": "presentation",
                        "valid_from": 100.0,
                        "valid_to": 200.0,
                        "confidence": 0.9,
                    }
                ],
                created_at=100.0,
            )

            graph.index_memory(memory)

            active = graph.query(subject="Marina", at_time=150.0)
            expired = graph.query(subject="Marina", at_time=250.0)

            self.assertEqual(len(active), 1)
            self.assertEqual(active[0].memory_id, memory.id)
            self.assertEqual(active[0].predicate, "has_event")
            self.assertEqual(expired, [])

    def test_recall_uses_graph_when_query_matches_relation(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            store = TemporalStore(root / "store")
            graph = GraphStore(root / "graph.sqlite3")
            memory = MemoryEnvelope.text(
                "The important date is May 15.",
                scope={"project_id": "psi"},
                tags=["calendar"],
                relations=[{"subject": "Marina", "predicate": "has_event", "object": "presentation"}],
                importance=0.6,
                created_at=1.0,
            )
            store.remember(memory)
            graph.index_memory(memory)

            results = RecallPipeline(store, graph_store=graph).recall(
                "Marina presentation",
                scope={"project_id": "psi"},
            )

            self.assertEqual(results[0].memory.id, memory.id)
            self.assertIn("graph", results[0].why_retrieved)

    def test_named_graphs_keep_multimodal_and_temporal_relations_separate(self):
        with tempfile.TemporaryDirectory() as tmp:
            graph = GraphStore(Path(tmp) / "graph.sqlite3")
            graph.add_relation(
                subject="artifact:board",
                predicate="has_caption",
                object="one-call facade diagram",
                memory_id="mem_image",
                graph_name="multimodal",
                source="unit-test",
            )
            graph.add_relation(
                subject="embedding_policy",
                predicate="default",
                object="optional",
                memory_id="mem_policy",
                graph_name="temporal",
                valid_from=100.0,
                source="unit-test",
            )

            multimodal = graph.query(graph_name="multimodal")
            temporal = graph.query(graph_name="temporal", at_time=150.0)
            all_relations = graph.query()

            self.assertEqual([item.memory_id for item in multimodal], ["mem_image"])
            self.assertEqual([item.memory_id for item in temporal], ["mem_policy"])
            self.assertEqual({item.graph_name for item in all_relations}, {"multimodal", "temporal"})

    def test_subgraph_requires_anchor_overlap_and_respects_graph_names(self):
        with tempfile.TemporaryDirectory() as tmp:
            graph = GraphStore(Path(tmp) / "graph.sqlite3")
            graph.add_relation(
                subject="artifact:board",
                predicate="has_caption",
                object="local first multimodal memory routing",
                memory_id="mem_image",
                graph_name="multimodal",
            )
            graph.add_relation(
                subject="artifact:board",
                predicate="mentions",
                object="unrelated payroll city reimbursement",
                memory_id="mem_noise",
                graph_name="semantic",
            )

            subgraphs = graph.answerable_subgraphs(
                ["local", "first", "multimodal", "memory", "routing"],
                graph_names=["multimodal"],
                limit=4,
                min_anchor_terms=2,
            )

            self.assertEqual(len(subgraphs), 1)
            self.assertEqual(subgraphs[0]["graph_name"], "multimodal")
            self.assertEqual(subgraphs[0]["memory_ids"], ["mem_image"])
            self.assertGreaterEqual(subgraphs[0]["answerability"], 0.4)


if __name__ == "__main__":
    unittest.main()
