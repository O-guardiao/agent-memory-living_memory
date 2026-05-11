import tempfile
import unittest

from _path import SRC  # noqa: F401
from living_memoryv2.connectors import ConnectorIngestor, ConnectorRecord
from living_memoryv2.extraction import RuleBasedFactExtractor
from living_memoryv2.schema import MemoryEnvelope
from living_memoryv2.mcp_server import LivingMemoryTools
from living_memoryv2.vault import MemoryVault


class ExtractionAndConnectorTests(unittest.TestCase):
    def test_rule_based_extractor_finds_entities_dates_and_key_value_facts(self):
        with tempfile.TemporaryDirectory() as tmp:
            vault = MemoryVault(tmp)
            memory = vault.remember(
                MemoryEnvelope.text(
                    "Project: psi_engine_v4_complete\nOwner: Helena\nValid from: 2026-05-10\nPolicy should use living_memory facade.",
                    scope={"project_id": "psi"},
                    memory_type="decision",
                    tags=["policy"],
                )
            )
            envelope = vault.store.inspect(memory)

            facts = RuleBasedFactExtractor().extract(envelope)

            fact_keys = {(fact.subject, fact.predicate, fact.object) for fact in facts}
            self.assertIn(("project", "is", "psi_engine_v4_complete"), fact_keys)
            self.assertIn(("owner", "is", "Helena"), fact_keys)
            self.assertTrue(any(fact.valid_from is not None for fact in facts))
            self.assertTrue(any("Helena" in fact.entities for fact in facts))

    def test_connector_ingestor_normalizes_records_to_memory_envelopes_and_facts(self):
        with tempfile.TemporaryDirectory() as tmp:
            vault = MemoryVault(tmp)
            ingestor = ConnectorIngestor()
            envelope = ingestor.to_memory(
                ConnectorRecord(
                    connector_type="ticket",
                    external_id="TCK-17",
                    title="MCP policy update",
                    body="Owner: Helena\nStatus: active\nMemory policy should use living_memory facade.",
                    source_uri="https://tickets.local/TCK-17",
                    updated_at=1778430000.0,
                    metadata={"priority": "high"},
                ),
                scope={"project_id": "psi"},
            )

            memory_id = vault.remember(envelope, extract_facts=True)
            stored = vault.store.inspect(memory_id)
            facts = vault.facts.facts_for_memory(memory_id)

            self.assertEqual(stored.provenance["connector_type"], "ticket")
            self.assertEqual(stored.provenance["external_id"], "TCK-17")
            self.assertIn("connector", stored.tags)
            self.assertTrue(any(fact.subject == "owner" and fact.object == "Helena" for fact in facts))

    def test_mcp_remember_text_can_opt_into_deterministic_fact_extraction(self):
        with tempfile.TemporaryDirectory() as tmp:
            tools = LivingMemoryTools(tmp)

            remembered = tools.remember_text(
                content="Owner: Helena\nPolicy should use living_memory facade.",
                scope={"project_id": "psi"},
                memory_type="decision",
                extract_facts=True,
            )

            status = tools.fact_status(memory_id=remembered["memory_id"])
            facts = {(fact["subject"], fact["predicate"], fact["object"]) for fact in status["facts"]}
            self.assertIn(("owner", "is", "Helena"), facts)
            self.assertGreaterEqual(status["status"]["active_fact_count"], 1)


if __name__ == "__main__":
    unittest.main()
