from __future__ import annotations

from dataclasses import dataclass, field

from .governance import governance_for, prompt_text_for
from .recall import RecallResult


@dataclass
class EvidencePacket:
    memory_id: str
    memory_type: str
    created_at: float
    source: str | None
    snippet: str
    score: float
    confidence: float
    why_retrieved: list[str]
    provenance: dict[str, object] = field(default_factory=dict)
    metadata: dict[str, object] = field(default_factory=dict)
    estimated_tokens: int = 0

    def to_dict(self) -> dict[str, object]:
        return {
            "memory_id": self.memory_id,
            "memory_type": self.memory_type,
            "created_at": self.created_at,
            "source": self.source,
            "snippet": self.snippet,
            "score": self.score,
            "confidence": self.confidence,
            "why_retrieved": self.why_retrieved,
            "provenance": self.provenance,
            "metadata": self.metadata,
            "estimated_tokens": self.estimated_tokens,
        }


class ContextAssembler:
    def __init__(self, max_tokens: int = 4000, chars_per_token: int = 4) -> None:
        if max_tokens <= 0:
            raise ValueError("max_tokens must be positive")
        self.max_tokens = max_tokens
        self.chars_per_token = max(1, chars_per_token)

    def assemble(self, results: list[RecallResult]) -> list[EvidencePacket]:
        ordered = sorted(
            results,
            key=lambda item: (
                item.memory.importance >= 0.95,
                item.score,
                item.memory.importance,
                item.memory.created_at,
            ),
            reverse=True,
        )
        packets: list[EvidencePacket] = []
        used_tokens = 0
        for result in ordered:
            remaining = self.max_tokens - used_tokens
            if remaining <= 0:
                break
            text = prompt_text_for(result.memory)
            snippet, tokens = self._fit_text(text, remaining)
            if not snippet:
                continue
            packet = EvidencePacket(
                memory_id=result.memory.id,
                memory_type=result.memory.memory_type,
                created_at=result.memory.created_at,
                source=result.memory.provenance.get("source"),
                snippet=snippet,
                score=result.score,
                confidence=result.confidence,
                why_retrieved=list(result.why_retrieved),
                provenance=result.memory.provenance,
                metadata={
                    "scope": result.memory.scope,
                    "tags": result.memory.tags,
                    "entities": result.memory.entities,
                    "modalities": result.memory.modality_names(),
                    "content_hash": result.memory.content_hash,
                    "governance": governance_for(result.memory),
                },
                estimated_tokens=tokens,
            )
            packets.append(packet)
            used_tokens += tokens
        return packets

    def estimate_tokens(self, text: str) -> int:
        return max(1, (len(text) + self.chars_per_token - 1) // self.chars_per_token)

    def _fit_text(self, text: str, token_budget: int) -> tuple[str, int]:
        if not text or token_budget <= 0:
            return "", 0
        tokens = self.estimate_tokens(text)
        if tokens <= token_budget:
            return text, tokens
        max_chars = token_budget * self.chars_per_token
        snippet = text[:max_chars].rstrip()
        return snippet, self.estimate_tokens(snippet)
