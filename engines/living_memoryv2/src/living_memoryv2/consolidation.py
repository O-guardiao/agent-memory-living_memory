from __future__ import annotations

from typing import Any

from .agents import ProposalStore
from .extraction import RuleBasedFactExtractor
from .schema import MemoryEnvelope
from .vault import MemoryVault


class ConsolidationRunner:
    def __init__(
        self,
        vault: MemoryVault,
        proposals: ProposalStore,
        *,
        extractor: RuleBasedFactExtractor | None = None,
    ) -> None:
        self.vault = vault
        self.proposals = proposals
        self.extractor = extractor or RuleBasedFactExtractor()

    def propose_from_recent(
        self,
        *,
        scope: dict[str, Any] | None = None,
        limit: int = 50,
    ) -> dict[str, Any]:
        proposal_ids: list[str] = []
        memories = self.vault.store.list_memories(scope=scope, limit=limit)
        existing_signatures = {
            (
                fact.subject.lower(),
                fact.predicate.lower(),
                fact.object.lower(),
                fact.source_memory_id,
            )
            for fact in self.vault.facts.list_facts(scope=scope, limit=max(limit * 4, 100))
        }
        for memory in memories:
            for fact in self.extractor.extract(memory):
                signature = (
                    fact.subject.lower(),
                    fact.predicate.lower(),
                    fact.object.lower(),
                    fact.source_memory_id,
                )
                if signature in existing_signatures:
                    continue
                candidate = MemoryEnvelope.structured(
                    fact.to_dict(),
                    scope=fact.scope,
                    memory_type="semantic_fact",
                    tags=["derived", "fact", "consolidation"],
                    entities=fact.entities,
                    importance=0.64,
                    confidence=fact.confidence,
                    provenance={
                        "source": "background_consolidation",
                        "derived_from": fact.source_memory_id,
                        "extractor": "rule_based",
                    },
                    metadata={
                        "review_status": "pending",
                        "fact_signature": "|".join(signature),
                    },
                )
                proposal = self.proposals.propose_memory(
                    candidate,
                    proposal_type="derived_fact",
                    rationale=f"Derived {fact.subject} {fact.predicate} {fact.object} from {memory.id}.",
                    created_by="consolidation_runner",
                )
                proposal_ids.append(proposal.id)
                existing_signatures.add(signature)
        return {"proposal_count": len(proposal_ids), "proposal_ids": proposal_ids}
