from __future__ import annotations

import re
from typing import Any

from .lifecycle import lifecycle_for
from .schema import MemoryEnvelope


INSPECT_ONLY_LABELS = {"credential", "pii", "prompt_attack", "restricted"}

SENSITIVE_LEVELS = {
    "sensitive",
    "confidential",
    "restricted",
    "credential",
    "pii",
}

CREDENTIAL_MARKERS = (
    "api key",
    "apikey",
    "bearer ",
    "chave api",
    "credential",
    "password",
    "secret",
    "segredo",
    "senha",
    "sk-",
    "xoxb-",
)

PROMPT_ATTACK_MARKERS = (
    "disregard previous instructions",
    "exfiltrate",
    "export the memory vault",
    "ignore all previous instructions",
    "ignore previous instructions",
    "reveal secrets",
    "system prompt",
)

PROMPT_ATTACK_TAGS = {
    "prompt-attack",
    "prompt-injection",
    "prompt_attack",
    "prompt_injection",
    "unsafe-media",
    "unsafe_media",
}


def governance_for(memory: MemoryEnvelope) -> dict[str, Any]:
    """Return compact governance metadata for audit/inspect surfaces."""

    lifecycle = lifecycle_for(memory)
    safety_labels = safety_labels_for(memory)
    prompt_policy = "inspect_only" if INSPECT_ONLY_LABELS & set(safety_labels) else "inline"
    trace: dict[str, Any] = {
        "state": lifecycle.state,
        "sensitivity": normalized_sensitivity(memory.sensitivity),
        "safety_labels": safety_labels,
        "prompt_policy": prompt_policy,
        "effective_importance": round(lifecycle.effective_importance(), 4),
    }
    if lifecycle.version:
        trace["version"] = lifecycle.version
    if lifecycle.superseded_by:
        trace["superseded_by"] = lifecycle.superseded_by
    if lifecycle.supersedes:
        trace["supersedes"] = lifecycle.supersedes[:4]
    return trace


def prompt_text_for(memory: MemoryEnvelope) -> str:
    """Return text safe for prompt/SIF surfaces without losing inspectability."""

    governance = governance_for(memory)
    if governance["prompt_policy"] == "inline":
        return memory.text_content()
    labels = ",".join(governance["safety_labels"][:4]) or "sensitive"
    return f"[memory withheld: {labels}; use inspect_memory({memory.id}) with permission]"


def safety_labels_for(memory: MemoryEnvelope) -> list[str]:
    labels: set[str] = set()
    metadata = memory.metadata if isinstance(memory.metadata, dict) else {}
    raw_labels = metadata.get("safety_labels") or metadata.get("safety") or []
    if isinstance(raw_labels, str):
        raw_labels = [raw_labels]
    labels.update(_normalize_label(item) for item in raw_labels if str(item).strip())

    sensitivity = normalized_sensitivity(memory.sensitivity)
    if sensitivity in SENSITIVE_LEVELS:
        labels.add(sensitivity)

    tags = {_normalize_label(tag) for tag in memory.tags}
    if tags & PROMPT_ATTACK_TAGS:
        labels.add("prompt_attack")
    if "credential" in tags:
        labels.add("credential")
    if "pii" in tags:
        labels.add("pii")

    text = memory.text_content().lower()
    if any(marker in text for marker in CREDENTIAL_MARKERS):
        labels.add("credential")
    if _EMAIL_RE.search(text) or _CPF_RE.search(text) or _PHONE_MARKER_RE.search(text):
        labels.add("pii")
    if any(marker in text for marker in PROMPT_ATTACK_MARKERS):
        labels.add("prompt_attack")

    return sorted(label for label in labels if label)


def normalized_sensitivity(value: Any) -> str:
    text = _normalize_label(value or "normal")
    return text or "normal"


def _normalize_label(value: Any) -> str:
    return str(value or "").strip().lower().replace("-", "_").replace(" ", "_")


_EMAIL_RE = re.compile(r"\b[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}\b", re.IGNORECASE)
_CPF_RE = re.compile(r"\b\d{3}\.?\d{3}\.?\d{3}-?\d{2}\b")
_PHONE_MARKER_RE = re.compile(r"\b(cpf|rg|telefone|phone|ssn)\b", re.IGNORECASE)
