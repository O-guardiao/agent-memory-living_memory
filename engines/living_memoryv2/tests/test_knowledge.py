import tempfile
import unittest
from pathlib import Path

from _path import SRC  # noqa: F401
from living_memoryv2.graph import GraphStore
from living_memoryv2.knowledge import KnowledgeGraph
from living_memoryv2.mcp_server import LivingMemoryTools
from living_memoryv2.schema import MemoryEnvelope


class KnowledgeGraphTests(unittest.TestCase):
    def test_memory_relations_are_mirrored_to_temporal_knowledge_graph(self):
        with tempfile.TemporaryDirectory() as tmp:
            tools = LivingMemoryTools(tmp)
            memory_id = tools.remember_text(
                content="SIF v2 routes structured context through a knowledge layer.",
                scope={"project_id": "psi"},
                relations=[
                    {
                        "subject": "SIF v2",
                        "predicate": "uses",
                        "object": "Temporal Knowledge Graph",
                        "valid_from": 100.0,
                        "valid_to": 200.0,
                    }
                ],
            )["memory_id"]

            active = tools.query_knowledge(
                subject="SIF v2",
                predicate="uses",
                scope={"project_id": "psi"},
                at_time=150.0,
            )
            expired = tools.query_knowledge(
                subject="SIF v2",
                predicate="uses",
                scope={"project_id": "psi"},
                at_time=250.0,
            )
            graph_edges = tools.query_graph(graph_name="knowledge", subject="SIF v2")

            self.assertEqual(active["count"], 1)
            self.assertEqual(active["assertions"][0]["source_memory_id"], memory_id)
            self.assertEqual(active["assertions"][0]["object"], "Temporal Knowledge Graph")
            self.assertEqual(expired["count"], 0)
            self.assertTrue(any(edge["memory_id"] == memory_id for edge in graph_edges["relations"]))

    def test_fact_ledger_mirrors_supersession_into_knowledge_graph(self):
        with tempfile.TemporaryDirectory() as tmp:
            tools = LivingMemoryTools(tmp)
            old_memory = tools.remember_text(
                content="Current context policy: use SIF v1 only.",
                scope={"project_id": "psi"},
            )["memory_id"]
            new_memory = tools.remember_text(
                content="Current context policy: use SIF v2 structured sections beside SIF v1.",
                scope={"project_id": "psi"},
            )["memory_id"]
            old_fact = tools.vault.facts.add_fact(
                subject="context policy",
                predicate="uses",
                object="SIF v1 only",
                source_memory_id=old_memory,
                scope={"project_id": "psi"},
            )
            new_fact = tools.vault.facts.add_fact(
                subject="context policy",
                predicate="uses",
                object="SIF v2 structured sections beside SIF v1",
                source_memory_id=new_memory,
                scope={"project_id": "psi"},
                supersedes=[old_fact.fact_id],
                reason="newer architecture decision",
            )

            active = tools.query_knowledge(subject="context policy", scope={"project_id": "psi"})
            all_assertions = tools.vault.knowledge.query(subject="context policy", scope={"project_id": "psi"}, status=None)

            self.assertEqual([item["fact_id"] for item in active["assertions"]], [new_fact.fact_id])
            statuses = {item.fact_id: item.status for item in all_assertions}
            self.assertEqual(statuses[old_fact.fact_id], "superseded")
            self.assertEqual(statuses[new_fact.fact_id], "active")

    def test_knowledge_query_respects_scope(self):
        with tempfile.TemporaryDirectory() as tmp:
            graph = GraphStore(Path(tmp) / "graph.sqlite3")
            knowledge = KnowledgeGraph(Path(tmp) / "knowledge.sqlite3", graph)
            knowledge.add_assertion(
                subject="agent-memory",
                predicate="serves",
                object="psi",
                source_memory_id="mem_psi",
                scope={"project_id": "psi"},
            )
            knowledge.add_assertion(
                subject="agent-memory",
                predicate="serves",
                object="finance",
                source_memory_id="mem_finance",
                scope={"project_id": "finance"},
            )

            results = knowledge.query(subject="agent-memory", scope={"project_id": "psi"})

            self.assertEqual([item.source_memory_id for item in results], ["mem_psi"])

    def test_rebuild_indexes_restores_knowledge_from_canonical_log_and_facts(self):
        with tempfile.TemporaryDirectory() as tmp:
            tools = LivingMemoryTools(tmp)
            memory = MemoryEnvelope.text(
                "Owner: Helena",
                scope={"project_id": "psi"},
                relations=[{"subject": "agent-memory", "predicate": "owned_by", "object": "Helena"}],
            )
            memory_id = tools.vault.remember(memory, extract_facts=True)

            tools.vault.knowledge.clear()
            self.assertEqual(tools.vault.knowledge.count()["assertions"], 0)

            tools.vault.rebuild_indexes()
            relation_assertions = tools.vault.knowledge.query(subject="agent-memory", scope={"project_id": "psi"})
            fact_assertions = tools.vault.knowledge.query(subject="owner", object="Helena", scope={"project_id": "psi"})

            self.assertTrue(any(item.source_memory_id == memory_id for item in relation_assertions))
            self.assertTrue(any(item.source_memory_id == memory_id for item in fact_assertions))


if __name__ == "__main__":
    unittest.main()
