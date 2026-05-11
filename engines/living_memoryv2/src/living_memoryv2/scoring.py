from __future__ import annotations

from dataclasses import dataclass, field
import math
import re
from typing import Any, Iterable

from .lifecycle import CURRENT_QUERY_TERMS, HISTORICAL_QUERY_TERMS, lifecycle_for
from .recall import RecallResult


@dataclass(frozen=True)
class MemoryFeatureScore:
    memory_id: str
    utility: float
    features: dict[str, float] = field(default_factory=dict)
    penalties: dict[str, float] = field(default_factory=dict)
    reasons: list[str] = field(default_factory=list)

    def to_dict(self) -> dict[str, object]:
        return {
            "memory_id": self.memory_id,
            "utility": round(self.utility, 4),
            "features": {key: round(value, 4) for key, value in sorted(self.features.items())},
            "penalties": {key: round(value, 4) for key, value in sorted(self.penalties.items())},
            "reasons": list(self.reasons),
        }


def score_memory_candidate(
    result: RecallResult,
    query_terms: Iterable[str],
    *,
    phase: str = "work_start",
    at_time: float | None = None,
    governance: dict[str, Any] | None = None,
) -> MemoryFeatureScore:
    terms = {str(term).lower() for term in query_terms if str(term).strip()}
    memory = result.memory
    lifecycle = lifecycle_for(memory)
    governance = dict(governance or {})
    reasons = [lifecycle.state]
    memory_terms = _memory_terms(result)

    lexical = _coverage(terms, memory_terms)
    graph = _graph_signal(result)
    time_fit = lifecycle.effective_importance(query_terms=terms, at_time=at_time)
    importance = memory.importance
    confidence = min(1.0, max(0.0, result.confidence))
    activation = _activation(lifecycle.usage_count, lifecycle.last_used_at, at_time)
    phase_fit = _phase_fit(memory.memory_type, set(memory.tags), phase)
    retrieval = min(1.0, max(0.0, result.score))

    stale = _staleness_penalty(lifecycle.state, terms)
    risk = _governance_risk(governance)

    features = {
        "retrieval": retrieval,
        "lexical": lexical,
        "graph": graph,
        "time": time_fit,
        "importance": importance,
        "confidence": confidence,
        "activation": activation,
        "phase": phase_fit,
    }
    penalties = {
        "risk": risk,
        "stale": stale,
        "scope": 0.0,
    }
    utility = (
        0.18 * retrieval
        + 0.18 * lexical
        + 0.14 * graph
        + 0.12 * time_fit
        + 0.12 * importance
        + 0.10 * confidence
        + 0.08 * activation
        + 0.08 * phase_fit
        - 0.35 * risk
        - 0.25 * stale
    )
    if graph > 0:
        reasons.append("graph_supported")
    if risk > 0:
        reasons.append("risk")

    return MemoryFeatureScore(
        memory_id=memory.id,
        utility=_clamp(utility),
        features=features,
        penalties=penalties,
        reasons=reasons,
    )


def _memory_terms(result: RecallResult) -> set[str]:
    memory = result.memory
    material = " ".join(
        [
            memory.text_content(),
            " ".join(memory.tags),
            " ".join(memory.entities),
            memory.memory_type,
        ]
    ).lower()
    return set(re.findall(r"[\w]+", material, flags=re.UNICODE))


def _coverage(query_terms: set[str], memory_terms: set[str]) -> float:
    if not query_terms:
        return 0.0
    return len(query_terms & memory_terms) / len(query_terms)


def _graph_signal(result: RecallResult) -> float:
    reasons = set(result.why_retrieved)
    if "mobius_graph" in reasons:
        return 1.0
    if "graph" in reasons:
        return 0.85
    if result.layer_signals.get("graph"):
        return min(1.0, float(result.layer_signals["graph"]))
    return 0.0


def _activation(usage_count: int, last_used_at: float | None, at_time: float | None) -> float:
    usage = min(0.65, math.log1p(max(0, usage_count)) / 5.0)
    if last_used_at is None or at_time is None:
        return usage
    age = max(0.0, float(at_time) - float(last_used_at))
    recency = math.exp(-age / 86400.0)
    return _clamp(usage + 0.35 * recency)


def _phase_fit(memory_type: str, tags: set[str], phase: str) -> float:
    phase = phase or "work_start"
    if phase == "tool_preflight":
        return 1.0 if memory_type == "procedural" or {"lesson", "tool-use"} & tags else 0.35
    if phase == "planning":
        return 1.0 if memory_type in {"decision", "procedural", "artifact_section"} else 0.55
    if phase == "final_verification":
        return 1.0 if memory_type in {"decision", "semantic", "artifact", "artifact_section"} else 0.45
    if phase == "conflict_resolution":
        return 1.0
    return 0.7


def _staleness_penalty(state: str, terms: set[str]) -> float:
    historical_query = bool(terms & HISTORICAL_QUERY_TERMS)
    current_query = bool(terms & CURRENT_QUERY_TERMS)
    if historical_query:
        return 0.0
    if state == "superseded":
        return 0.6 if current_query else 0.45
    if state == "contradicted":
        return 0.5
    if state == "archival":
        return 0.35
    if state == "historical":
        return 0.25
    return 0.0


def _governance_risk(governance: dict[str, Any]) -> float:
    labels = {str(item).lower() for item in governance.get("safety_labels") or []}
    risk = 0.0
    if governance.get("prompt_policy") == "inspect_only":
        risk = max(risk, 0.72)
    if labels & {"credential", "pii", "prompt_attack", "restricted"}:
        risk = max(risk, 0.75)
    if labels & {"confidential", "sensitive"}:
        risk = max(risk, 0.45)
    return risk


def _clamp(value: float) -> float:
    return min(1.0, max(0.0, float(value)))

