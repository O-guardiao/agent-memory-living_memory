from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime
import re
from typing import Any

from .schema import MemoryEnvelope


@dataclass(frozen=True)
class ExtractedFact:
    subject: str
    predicate: str
    object: str
    source_memory_id: str
    scope: dict[str, Any] = field(default_factory=dict)
    confidence: float = 0.72
    valid_from: float | None = None
    valid_to: float | None = None
    entities: list[str] = field(default_factory=list)
    reason: str = "rule_based_extraction"

    def to_dict(self) -> dict[str, Any]:
        return {
            "subject": self.subject,
            "predicate": self.predicate,
            "object": self.object,
            "source_memory_id": self.source_memory_id,
            "scope": self.scope,
            "confidence": self.confidence,
            "valid_from": self.valid_from,
            "valid_to": self.valid_to,
            "entities": list(self.entities),
            "reason": self.reason,
        }


class RuleBasedFactExtractor:
    KEY_VALUE_RE = re.compile(r"^\s*([A-Za-z][\w \-/]{1,48})\s*:\s*(.+?)\s*$")
    SHOULD_USE_RE = re.compile(r"\b([A-Za-z][\w \-/]{1,48}?)\s+should\s+use\s+(.+?)(?:\.|$)", re.IGNORECASE)
    IS_RE = re.compile(r"\b([A-Za-z][\w \-/]{1,48}?)\s+is\s+(.+?)(?:\.|$)", re.IGNORECASE)
    ISO_DATE_RE = re.compile(r"\b(20\d{2}-\d{2}-\d{2})(?:[T ][0-2]\d:[0-5]\d(?::[0-5]\d)?)?\b")
    ENTITY_RE = re.compile(r"\b[A-Z][A-Za-z0-9_:-]{2,}\b")

    def extract(self, memory: MemoryEnvelope) -> list[ExtractedFact]:
        text = memory.text_content()
        entities = self._entities(memory, text)
        facts: list[ExtractedFact] = []
        for line in text.splitlines():
            match = self.KEY_VALUE_RE.match(line)
            if not match:
                continue
            key = self._clean_subject(match.group(1))
            value = match.group(2).strip()
            if not key or not value:
                continue
            valid_from = self._timestamp(value) if key in {"valid from", "valid_from", "date"} else None
            facts.append(
                ExtractedFact(
                    subject=key.replace("_", " "),
                    predicate="is",
                    object=value,
                    source_memory_id=memory.id,
                    scope=memory.scope,
                    confidence=0.82,
                    valid_from=valid_from,
                    entities=entities,
                )
            )
        for pattern, predicate in ((self.SHOULD_USE_RE, "should_use"), (self.IS_RE, "is")):
            for match in pattern.finditer(text):
                facts.append(
                    ExtractedFact(
                        subject=self._clean_subject(match.group(1)),
                        predicate=predicate,
                        object=match.group(2).strip(),
                        source_memory_id=memory.id,
                        scope=memory.scope,
                        confidence=0.68,
                        entities=entities,
                    )
                )
        return self._dedupe(facts)

    def _entities(self, memory: MemoryEnvelope, text: str) -> list[str]:
        entities = {str(item) for item in memory.entities}
        entities.update(item.group(0) for item in self.ENTITY_RE.finditer(text))
        return sorted(item for item in entities if item.lower() not in {"project", "owner", "status", "policy", "valid"})

    def _clean_subject(self, value: str) -> str:
        return " ".join(str(value).strip().lower().replace("_", " ").split())

    def _timestamp(self, value: str) -> float | None:
        match = self.ISO_DATE_RE.search(value)
        if not match:
            return None
        try:
            return datetime.fromisoformat(match.group(1)).timestamp()
        except ValueError:
            return None

    def _dedupe(self, facts: list[ExtractedFact]) -> list[ExtractedFact]:
        seen: set[tuple[str, str, str, str]] = set()
        deduped: list[ExtractedFact] = []
        for fact in facts:
            key = (fact.subject.lower(), fact.predicate.lower(), fact.object.lower(), fact.source_memory_id)
            if key in seen:
                continue
            seen.add(key)
            deduped.append(fact)
        return deduped
