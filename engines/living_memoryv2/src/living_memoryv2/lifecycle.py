from __future__ import annotations

from dataclasses import dataclass, field
import math
from typing import Any, Iterable

from .schema import MemoryEnvelope


ACTIVE_STATE = "active"
ALLOWED_STATES = {
    "active",
    "historical",
    "superseded",
    "contradicted",
    "archival",
    "deleted",
}

HISTORICAL_QUERY_TERMS = {
    "anterior",
    "antiga",
    "antigo",
    "audit",
    "auditoria",
    "before",
    "evolution",
    "evolucao",
    "evolução",
    "historical",
    "historico",
    "histórico",
    "history",
    "old",
    "passado",
    "past",
    "previous",
}

CURRENT_QUERY_TERMS = {
    "active",
    "atual",
    "current",
    "latest",
    "now",
    "recente",
}


@dataclass(frozen=True)
class LifecycleView:
    state: str = ACTIVE_STATE
    version: str | None = None
    supersedes: list[str] = field(default_factory=list)
    superseded_by: str | None = None
    valid_from: float | None = None
    valid_to: float | None = None
    usage_count: int = 0
    last_used_at: float | None = None
    importance_delta: float = 0.0
    base_importance: float = 0.5

    @property
    def is_deleted(self) -> bool:
        return self.state == "deleted"

    def active_at(self, at_time: float | None) -> bool:
        if at_time is None:
            return True
        starts_before = self.valid_from is None or self.valid_from <= at_time
        ends_after = self.valid_to is None or self.valid_to >= at_time
        return starts_before and ends_after

    def effective_importance(
        self,
        base_importance: float | None = None,
        *,
        query_terms: Iterable[str] | None = None,
        at_time: float | None = None,
    ) -> float:
        base = self.base_importance if base_importance is None else _clamp(float(base_importance))
        if self.is_deleted:
            return 0.0

        terms = _terms(query_terms)
        historical_query = bool(terms & HISTORICAL_QUERY_TERMS)
        current_query = bool(terms & CURRENT_QUERY_TERMS)
        state_adjustment = {
            "active": 0.0,
            "historical": -0.10,
            "superseded": -0.28,
            "contradicted": -0.22,
            "archival": -0.35,
        }.get(self.state, 0.0)
        if historical_query and self.state in {"historical", "superseded", "archival"}:
            state_adjustment += 0.25
        if historical_query and self.state == "contradicted":
            state_adjustment += 0.10
        if current_query and self.state == "superseded":
            state_adjustment -= 0.12
        if at_time is not None and not self.active_at(float(at_time)) and not historical_query:
            state_adjustment -= 0.50

        usage_bonus = min(0.12, math.log1p(max(0, int(self.usage_count))) * 0.03)
        return _clamp(base + float(self.importance_delta) + state_adjustment + usage_bonus)

    def adjusted_score(
        self,
        score: float,
        *,
        query_terms: Iterable[str] | None = None,
        at_time: float | None = None,
    ) -> float:
        if self.is_deleted:
            return 0.0
        base = _clamp(self.base_importance)
        effective = self.effective_importance(query_terms=query_terms, at_time=at_time)
        terms = _terms(query_terms)
        adjustment = (effective - base) * 0.65
        if self.state == "superseded" and terms & CURRENT_QUERY_TERMS:
            adjustment -= 0.18
        if self.state == "contradicted" and not (terms & HISTORICAL_QUERY_TERMS):
            adjustment -= 0.20
        if self.state in {"historical", "superseded", "archival"} and terms & HISTORICAL_QUERY_TERMS:
            adjustment += 0.16
        return _clamp(float(score) + adjustment)

    def should_recall(self, *, query_terms: Iterable[str] | None = None, at_time: float | None = None) -> bool:
        if self.is_deleted:
            return False
        terms = _terms(query_terms)
        if at_time is not None and not self.active_at(float(at_time)) and not (terms & HISTORICAL_QUERY_TERMS):
            return False
        return True

    def to_trace(
        self,
        *,
        query_terms: Iterable[str] | None = None,
        at_time: float | None = None,
    ) -> dict[str, Any]:
        effective = self.effective_importance(query_terms=query_terms, at_time=at_time)
        trace: dict[str, Any] = {
            "state": self.state,
            "effective_importance": round(effective, 4),
        }
        if self.state != ACTIVE_STATE:
            trace["rank_policy"] = "preserved_downranked"
        if self.version:
            trace["version"] = self.version
        if self.superseded_by:
            trace["superseded_by"] = self.superseded_by
        if self.supersedes:
            trace["supersedes"] = self.supersedes[:4]
        if self.usage_count:
            trace["usage_count"] = self.usage_count
        if at_time is not None and not self.active_at(float(at_time)):
            trace["active_at_query_time"] = False
        return trace


def lifecycle_for(memory: MemoryEnvelope) -> LifecycleView:
    raw = memory.metadata.get("lifecycle") if isinstance(memory.metadata, dict) else None
    data = dict(raw) if isinstance(raw, dict) else {}
    state = _state(data.get("state"))
    return LifecycleView(
        state=state,
        version=_optional_str(data.get("version")),
        supersedes=_string_list(data.get("supersedes")),
        superseded_by=_optional_str(data.get("superseded_by")),
        valid_from=_optional_float(data.get("valid_from")),
        valid_to=_optional_float(data.get("valid_to")),
        usage_count=_optional_int(data.get("usage_count")),
        last_used_at=_optional_float(data.get("last_used_at")),
        importance_delta=_optional_float(data.get("importance_delta")) or 0.0,
        base_importance=memory.importance,
    )


def _state(value: Any) -> str:
    state = str(value or ACTIVE_STATE).strip().lower().replace(" ", "_")
    return state if state in ALLOWED_STATES else ACTIVE_STATE


def _terms(values: Iterable[str] | None) -> set[str]:
    return {str(item).strip().lower() for item in values or [] if str(item).strip()}


def _optional_float(value: Any) -> float | None:
    if value is None or value == "":
        return None
    try:
        return float(value)
    except (TypeError, ValueError):
        return None


def _optional_int(value: Any) -> int:
    if value is None or value == "":
        return 0
    try:
        return max(0, int(value))
    except (TypeError, ValueError):
        try:
            return max(0, int(float(value)))
        except (TypeError, ValueError):
            return 0


def _optional_str(value: Any) -> str | None:
    if value is None:
        return None
    text = str(value).strip()
    return text or None


def _string_list(value: Any) -> list[str]:
    if value is None:
        return []
    if isinstance(value, (list, tuple)):
        return [str(item) for item in value if str(item)]
    return [str(value)]


def _clamp(value: float) -> float:
    return min(1.0, max(0.0, float(value)))
