import tempfile
import unittest

from _path import SRC  # noqa: F401
from living_memoryv2.mcp_server import LivingMemoryTools


class FactLedgerTests(unittest.TestCase):
    def test_fact_supersession_is_auditable_and_graph_indexed(self):
        with tempfile.TemporaryDirectory() as tmp:
            tools = LivingMemoryTools(tmp)
            old_memory = tools.remember_text(
                content="Current MCP policy: use five separate memory tools.",
                scope={"project_id": "psi"},
                memory_type="decision",
                importance=0.95,
            )["memory_id"]
            new_memory = tools.remember_text(
                content="Current MCP policy: use one living_memory facade.",
                scope={"project_id": "psi"},
                memory_type="decision",
                importance=0.75,
            )["memory_id"]

            old_fact = tools.vault.facts.add_fact(
                subject="mcp policy",
                predicate="uses",
                object="five separate memory tools",
                source_memory_id=old_memory,
                scope={"project_id": "psi"},
                confidence=0.82,
            )
            new_fact = tools.vault.facts.add_fact(
                subject="mcp policy",
                predicate="uses",
                object="one living_memory facade",
                source_memory_id=new_memory,
                scope={"project_id": "psi"},
                confidence=0.94,
                supersedes=[old_fact.fact_id],
                reason="newer verified policy",
            )

            self.assertEqual(tools.vault.facts.get(old_fact.fact_id).status, "superseded")
            self.assertEqual(tools.vault.facts.get(new_fact.fact_id).status, "active")
            conflicts = tools.vault.facts.conflicts_for_fact(old_fact.fact_id)
            self.assertEqual(conflicts[0]["superseding_fact_id"], new_fact.fact_id)

            graph_edges = tools.vault.graph.query(graph_name="conflict", object=old_fact.fact_id)
            self.assertTrue(any(edge.subject == new_fact.fact_id and edge.predicate == "supersedes" for edge in graph_edges))

    def test_added_fact_is_indexed_in_facts_graph_namespace(self):
        with tempfile.TemporaryDirectory() as tmp:
            tools = LivingMemoryTools(tmp)
            memory_id = tools.remember_text(
                content="Owner: Helena",
                scope={"project_id": "psi"},
                memory_type="decision",
            )["memory_id"]

            fact = tools.vault.facts.add_fact(
                subject="owner",
                predicate="is",
                object="Helena",
                source_memory_id=memory_id,
                scope={"project_id": "psi"},
                confidence=0.88,
            )

            graph_edges = tools.vault.graph.query(graph_name="facts", subject="owner", predicate="is")
            self.assertTrue(any(edge.object == "Helena" and edge.memory_id == memory_id for edge in graph_edges))
            self.assertEqual(graph_edges[0].source, "fact_ledger")
            self.assertEqual(fact.source_memory_id, memory_id)

    def test_rebuild_indexes_restores_fact_and_conflict_graph_edges(self):
        with tempfile.TemporaryDirectory() as tmp:
            tools = LivingMemoryTools(tmp)
            old_memory = tools.remember_text(
                content="Current MCP policy: five separate tools.",
                scope={"project_id": "psi"},
                memory_type="decision",
            )["memory_id"]
            new_memory = tools.remember_text(
                content="Current MCP policy: one living_memory facade.",
                scope={"project_id": "psi"},
                memory_type="decision",
            )["memory_id"]
            old_fact = tools.vault.facts.add_fact(
                subject="mcp policy",
                predicate="uses",
                object="five separate tools",
                source_memory_id=old_memory,
                scope={"project_id": "psi"},
            )
            new_fact = tools.vault.facts.add_fact(
                subject="mcp policy",
                predicate="uses",
                object="one living_memory facade",
                source_memory_id=new_memory,
                scope={"project_id": "psi"},
                supersedes=[old_fact.fact_id],
                reason="newer verified policy",
            )

            tools.vault.rebuild_indexes()

            facts_edges = tools.vault.graph.query(graph_name="facts", subject="mcp policy", predicate="uses")
            conflict_edges = tools.vault.graph.query(graph_name="conflict", object=old_fact.fact_id)
            self.assertTrue(any(edge.object == "one living_memory facade" for edge in facts_edges))
            self.assertTrue(any(edge.subject == new_fact.fact_id and edge.memory_id == new_memory for edge in conflict_edges))

    def test_current_facade_downranks_memory_whose_fact_was_superseded(self):
        with tempfile.TemporaryDirectory() as tmp:
            tools = LivingMemoryTools(tmp)
            old_memory = tools.remember_text(
                content="Current MCP policy facade uses five separate tools for memory work.",
                scope={"project_id": "psi"},
                memory_type="decision",
                importance=0.99,
                confidence=0.99,
            )["memory_id"]
            new_memory = tools.remember_text(
                content="Updated policy: current MCP memory work uses the one-call living_memory facade.",
                scope={"project_id": "psi"},
                memory_type="decision",
                importance=0.65,
                confidence=0.95,
            )["memory_id"]
            old_fact = tools.vault.facts.add_fact(
                subject="mcp memory policy",
                predicate="uses",
                object="five separate tools",
                source_memory_id=old_memory,
                scope={"project_id": "psi"},
            )
            tools.vault.facts.add_fact(
                subject="mcp memory policy",
                predicate="uses",
                object="one-call living_memory facade",
                source_memory_id=new_memory,
                scope={"project_id": "psi"},
                supersedes=[old_fact.fact_id],
                reason="replacement policy",
            )

            result = tools.living_memory(
                query="current MCP memory policy facade",
                scope={"project_id": "psi"},
                top_k=1,
                max_tokens=80,
            )

            self.assertEqual(result["inspectable_ids"], [new_memory])
            self.assertEqual(result["evidence"][0]["fact_status"]["active_fact_count"], 1)


if __name__ == "__main__":
    unittest.main()
