from __future__ import annotations

from dataclasses import dataclass, field
from typing import Iterable

from .scoring import MemoryFeatureScore


@dataclass(frozen=True)
class AnswerabilityAssessment:
    score: float
    status: str
    reasons: list[str] = field(default_factory=list)

    def to_dict(self) -> dict[str, object]:
        return {
            "score": round(self.score, 4),
            "status": self.status,
            "reasons": list(self.reasons),
        }


def assess_answerability(
    query_terms: Iterable[str],
    scores: list[MemoryFeatureScore],
    *,
    phase: str = "work_start",
) -> AnswerabilityAssessment:
    terms = {str(term).lower() for term in query_terms if str(term).strip()}
    if not terms or not scores:
        return AnswerabilityAssessment(score=0.0, status="unsupported", reasons=["no_evidence"])

    best = max(scores, key=lambda item: item.utility)
    lexical = max(float(item.features.get("lexical", 0.0)) for item in scores)
    graph = max(float(item.features.get("graph", 0.0)) for item in scores)
    confidence = max(float(item.features.get("confidence", 0.0)) for item in scores)
    temporal = max(float(item.features.get("time", 0.0)) for item in scores)
    risk = max(float(item.penalties.get("risk", 0.0)) for item in scores)
    stale = min(1.0, sum(float(item.penalties.get("stale", 0.0)) for item in scores) / len(scores))

    score = _clamp(
        0.34 * best.utility
        + 0.22 * lexical
        + 0.14 * graph
        + 0.12 * confidence
        + 0.10 * temporal
        - 0.35 * risk
        - 0.20 * stale
    )
    threshold = _threshold_for_phase(phase)
    reasons: list[str] = []
    if risk >= 0.5:
        reasons.append("risk_too_high")
    if stale >= 0.5:
        reasons.append("stale_or_superseded")
    if lexical <= 0.0 and graph <= 0.0:
        reasons.append("low_query_coverage")
    if score >= threshold and not any(reason in reasons for reason in {"risk_too_high", "low_query_coverage"}):
        reasons.append("sufficient_support")
        return AnswerabilityAssessment(score=score, status="supported", reasons=reasons)
    reasons.append("insufficient_support")
    return AnswerabilityAssessment(score=score, status="unsupported", reasons=reasons)


def _threshold_for_phase(phase: str) -> float:
    if phase == "final_verification":
        return 0.60
    if phase == "tool_preflight":
        return 0.40
    if phase == "planning":
        return 0.35
    return 0.25


def _clamp(value: float) -> float:
    return min(1.0, max(0.0, float(value)))

