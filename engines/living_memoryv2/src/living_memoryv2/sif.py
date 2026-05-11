from __future__ import annotations

from dataclasses import dataclass
import json
import re
from typing import Any

from .governance import governance_for, prompt_text_for
from .recall import RecallResult


@dataclass
class SIFContextFrame:
    format: str
    query: str
    token_budget: int
    estimated_tokens: int
    memory_ids: list[str]
    text: str
    dropped_count: int = 0

    def to_dict(self) -> dict[str, Any]:
        return {
            "format": self.format,
            "query": self.query,
            "token_budget": self.token_budget,
            "estimated_tokens": self.estimated_tokens,
            "memory_ids": self.memory_ids,
            "dropped_count": self.dropped_count,
            "sif_text": self.text,
        }


@dataclass
class SIFContextFrameV2:
    format: str
    query: str
    token_budget: int
    estimated_tokens: int
    memory_ids: list[str]
    sections: dict[str, Any]
    dropped_count: int = 0

    def to_dict(self) -> dict[str, Any]:
        return {
            "format": self.format,
            "query": self.query,
            "token_budget": self.token_budget,
            "estimated_tokens": self.estimated_tokens,
            "memory_ids": self.memory_ids,
            "dropped_count": self.dropped_count,
            "sections": self.sections,
        }


class SIFContextAssembler:
    """Build a compact, auditable prompt frame from recall results."""

    FORMAT = "lmv2-sif/1"

    def __init__(self, max_tokens: int = 700, chars_per_token: int = 4, preserve_order: bool = False) -> None:
        if max_tokens <= 0:
            raise ValueError("max_tokens must be positive")
        self.max_tokens = max_tokens
        self.chars_per_token = max(1, chars_per_token)
        self.preserve_order = preserve_order

    def assemble(self, query: str, results: list[RecallResult]) -> SIFContextFrame:
        ordered = list(results) if self.preserve_order else sorted(
            results,
            key=lambda item: (
                item.memory.importance >= 0.95,
                item.score,
                item.memory.importance,
                item.memory.confidence,
                item.memory.created_at,
            ),
            reverse=True,
        )
        lines = [self._header_line(query)]
        memory_ids: list[str] = []
        dropped = 0

        for result in ordered:
            index = len(memory_ids) + 1
            added = False
            meta = self._meta_line(index, result)
            prefix = "\n".join(lines + [meta, ""])
            remaining_tokens = self.max_tokens - self.estimate_tokens(prefix)

            if remaining_tokens > 0:
                safe_text = prompt_text_for(result.memory)
                snippet = self._fit_text(safe_text, remaining_tokens)
                if snippet:
                    candidate_lines = lines + [meta, snippet]
                    candidate_text = "\n".join(candidate_lines)
                    if self.estimate_tokens(candidate_text) > self.max_tokens:
                        available_chars = self.max_tokens * self.chars_per_token - len(prefix)
                        snippet = self._fit_chars(safe_text, available_chars)
                        if snippet:
                            candidate_lines = lines + [meta, snippet]
                            candidate_text = "\n".join(candidate_lines)

                    if self.estimate_tokens(candidate_text) <= self.max_tokens:
                        lines = candidate_lines
                        memory_ids.append(result.memory.id)
                        added = True

            if added:
                continue

            micro = self._fit_micro_meta_line(index, result, lines)
            if micro:
                lines.append(micro)
                memory_ids.append(result.memory.id)
                continue

            dropped += 1

        text = "\n".join(lines)
        return SIFContextFrame(
            format=self.FORMAT,
            query=query,
            token_budget=self.max_tokens,
            estimated_tokens=self.estimate_tokens(text),
            memory_ids=memory_ids,
            text=text,
            dropped_count=dropped,
        )

    def estimate_tokens(self, text: str) -> int:
        if not text:
            return 0
        return max(1, (len(text) + self.chars_per_token - 1) // self.chars_per_token)

    def _header_line(self, query: str) -> str:
        base = "LMV2_SIF v1 q="
        max_query_chars = 72
        if self.max_tokens < 80:
            reserved_for_first_id = 52
            max_query_chars = max(
                0,
                min(max_query_chars, self.max_tokens * self.chars_per_token - len(base) - reserved_for_first_id),
            )
        return base + self._one_line(query, max_query_chars)

    def _fit_micro_meta_line(self, index: int, result: RecallResult, lines: list[str]) -> str:
        memory = result.memory
        parts = [f"M{index}", f"id={memory.id}"]
        candidate = " ".join(parts)
        if self.estimate_tokens("\n".join(lines + [candidate])) > self.max_tokens:
            return ""

        extras = [
            f"type={memory.memory_type}",
            f"score={result.score:.2f}",
            f"conf={result.confidence:.2f}",
        ]
        if result.why_retrieved:
            extras.append("why=" + ",".join(result.why_retrieved[:2]))
        source = memory.provenance.get("source")
        if source:
            extras.append("src=" + self._one_line(str(source), 24))
        governance = governance_for(memory)
        if governance["prompt_policy"] != "inline":
            extras.append("gov=" + str(governance["prompt_policy"]))
        extras.append(f"inspect_memory({memory.id})")

        for extra in extras:
            trial = " ".join([*parts, extra])
            if self.estimate_tokens("\n".join(lines + [trial])) > self.max_tokens:
                continue
            parts.append(extra)
            candidate = trial
        return candidate

    def _meta_line(self, index: int, result: RecallResult) -> str:
        memory = result.memory
        parts = [
            f"[M{index}]",
            f"id={memory.id}",
            f"type={memory.memory_type}",
            f"score={result.score:.2f}",
            f"conf={result.confidence:.2f}",
        ]
        if result.why_retrieved:
            parts.append("why=" + ",".join(result.why_retrieved))
        source = memory.provenance.get("source")
        if source:
            parts.append("src=" + self._one_line(str(source), 36))
        if memory.tags:
            parts.append("tags=" + ",".join(memory.tags[:4]))
        governance = governance_for(memory)
        if governance["prompt_policy"] != "inline":
            parts.append("gov=" + str(governance["prompt_policy"]))
        modalities = [item for item in memory.modality_names() if item != "text"]
        if modalities:
            parts.append("mods=" + ",".join(modalities[:4]))
        parts.append(f"inspect_memory({memory.id})")
        return " ".join(parts)

    def _fit_text(self, text: str, token_budget: int) -> str:
        return self._fit_chars(text, token_budget * self.chars_per_token)

    def _fit_chars(self, text: str, char_budget: int) -> str:
        clean = self._one_line(text, max(char_budget, 0))
        if not clean or char_budget <= 0:
            return ""
        if len(clean) <= char_budget:
            return clean
        if char_budget <= 3:
            return ""
        return clean[: char_budget - 3].rstrip() + "..."

    def _one_line(self, text: str, max_chars: int) -> str:
        if not text or max_chars <= 0:
            return ""
        compact = re.sub(r"\s+", " ", text).strip()
        if len(compact) <= max_chars:
            return compact
        if max_chars <= 3:
            return compact[:max_chars]
        return compact[: max_chars - 3].rstrip() + "..."


class SIFContextAssemblerV2:
    """Build a structured MemoryOps context frame beside the stable SIF v1 text."""

    FORMAT = "lmv2-sif/2"

    def __init__(self, max_tokens: int = 700, chars_per_token: int = 4) -> None:
        if max_tokens <= 0:
            raise ValueError("max_tokens must be positive")
        self.max_tokens = max_tokens
        self.chars_per_token = max(1, chars_per_token)

    def assemble(
        self,
        query: str,
        results: list[RecallResult],
        *,
        evidence: list[dict[str, Any]] | None = None,
        answerability: dict[str, Any] | None = None,
        graph_hints: list[dict[str, Any]] | None = None,
        subgraphs: list[dict[str, Any]] | None = None,
        knowledge: dict[str, Any] | None = None,
        task_state: dict[str, Any] | None = None,
    ) -> SIFContextFrameV2:
        memory_ids = list(dict.fromkeys([item.memory.id for item in results]))
        evidence_items = list(evidence or [])
        knowledge_payload = dict(knowledge or {})
        sections: dict[str, Any] = {
            "answerability": dict(answerability or {}),
            "current_facts": self._current_facts(knowledge_payload),
            "decisions": self._decisions(evidence_items),
            "task_state": dict(task_state or {}),
            "evidence": self._evidence(evidence_items),
            "knowledge": knowledge_payload,
            "graph": {
                "hints": list(graph_hints or []),
                "subgraphs": list(subgraphs or []),
            },
            "governance": self._governance(evidence_items),
            "inspectable_ids": list(dict.fromkeys(memory_ids)),
            "abstention": {
                "status": "abstained_no_relevant_memory" if not memory_ids else "not_abstained",
                "reasons": list((answerability or {}).get("reasons") or []),
            },
            "token_roi": {
                "budget": self.max_tokens,
                "policy": "structured evidence only; inspect full memories by id",
            },
        }
        sections, dropped = self._fit_sections(sections)
        encoded = self._encode(sections)
        return SIFContextFrameV2(
            format=self.FORMAT,
            query=query,
            token_budget=self.max_tokens,
            estimated_tokens=self.estimate_tokens(encoded),
            memory_ids=memory_ids,
            sections=sections,
            dropped_count=dropped,
        )

    def estimate_tokens(self, text: str) -> int:
        if not text:
            return 0
        return max(1, (len(text) + self.chars_per_token - 1) // self.chars_per_token)

    def _current_facts(self, knowledge: dict[str, Any]) -> list[dict[str, Any]]:
        facts: list[dict[str, Any]] = []
        for assertion in knowledge.get("assertions") or []:
            if assertion.get("status") != "active":
                continue
            facts.append(
                {
                    "assertion_id": assertion.get("assertion_id"),
                    "fact_id": assertion.get("fact_id"),
                    "subject": assertion.get("subject"),
                    "predicate": assertion.get("predicate"),
                    "object": assertion.get("object"),
                    "source_memory_id": assertion.get("source_memory_id"),
                    "confidence": assertion.get("confidence"),
                    "valid_from": assertion.get("valid_from"),
                    "valid_to": assertion.get("valid_to"),
                }
            )
        return facts

    def _decisions(self, evidence: list[dict[str, Any]]) -> list[dict[str, Any]]:
        decisions: list[dict[str, Any]] = []
        for item in evidence:
            tags = {str(tag).lower() for tag in item.get("tags") or []}
            if item.get("memory_type") != "decision" and "decision" not in tags:
                continue
            decisions.append(
                {
                    "memory_id": item.get("memory_id"),
                    "hint": item.get("hint"),
                    "confidence": item.get("confidence"),
                    "source": item.get("source"),
                }
            )
        return decisions

    def _evidence(self, evidence: list[dict[str, Any]]) -> list[dict[str, Any]]:
        return [
            {
                "memory_id": item.get("memory_id"),
                "memory_type": item.get("memory_type"),
                "score": item.get("score"),
                "confidence": item.get("confidence"),
                "validated_by": item.get("validated_by") or [],
                "why_retrieved": item.get("why_retrieved") or [],
                "hint": item.get("hint"),
            }
            for item in evidence
        ]

    def _governance(self, evidence: list[dict[str, Any]]) -> list[dict[str, Any]]:
        governed: list[dict[str, Any]] = []
        for item in evidence:
            governance = dict(item.get("governance") or {})
            if governance.get("prompt_policy", "inline") == "inline":
                continue
            governed.append(
                {
                    "memory_id": item.get("memory_id"),
                    "prompt_policy": governance.get("prompt_policy"),
                    "risk": governance.get("risk"),
                    "labels": governance.get("labels") or [],
                }
            )
        return governed

    def _fit_sections(self, sections: dict[str, Any]) -> tuple[dict[str, Any], int]:
        fitted = json.loads(self._encode(sections))
        dropped = 0
        shrink_order = [
            ("knowledge", "assertions"),
            ("knowledge", "entities"),
            ("graph", "hints"),
            ("graph", "subgraphs"),
            ("evidence", None),
            ("current_facts", None),
            ("decisions", None),
        ]
        while self.estimate_tokens(self._encode(fitted)) > self.max_tokens:
            changed = False
            for section, nested in shrink_order:
                target = fitted.get(section)
                if nested is not None and isinstance(target, dict):
                    values = target.get(nested)
                    if isinstance(values, list) and values:
                        values.pop()
                        dropped += 1
                        changed = True
                        break
                elif isinstance(target, list) and target:
                    target.pop()
                    dropped += 1
                    changed = True
                    break
            if not changed:
                break
        fitted["token_roi"]["estimated_tokens"] = self.estimate_tokens(self._encode(fitted))
        return fitted, dropped

    def _encode(self, value: Any) -> str:
        return json.dumps(value, sort_keys=True, ensure_ascii=False, separators=(",", ":"))
