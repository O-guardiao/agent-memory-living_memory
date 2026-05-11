from __future__ import annotations

from dataclasses import dataclass, field
import json
from pathlib import Path
from typing import Any


@dataclass(frozen=True)
class ExternalBenchmarkCase:
    case_id: str
    suite: str
    query: str
    expected_answer: str = ""
    evidence_ids: list[str] = field(default_factory=list)
    should_answer: bool = True
    metadata: dict[str, Any] = field(default_factory=dict)

    def to_dict(self) -> dict[str, Any]:
        return {
            "case_id": self.case_id,
            "suite": self.suite,
            "query": self.query,
            "expected_answer": self.expected_answer,
            "evidence_ids": list(self.evidence_ids),
            "should_answer": self.should_answer,
            "metadata": self.metadata,
        }


def load_external_cases(path: str | Path, *, suite: str) -> list[ExternalBenchmarkCase]:
    payload = json.loads(Path(path).read_text(encoding="utf-8"))
    suite = str(suite).strip().lower()
    rows = payload.get("cases") if isinstance(payload, dict) else payload
    if not isinstance(rows, list):
        raise ValueError("external benchmark fixture must be a list or contain a cases list")
    if suite in {"locomo", "dmr", "beam", "halu_eval", "halueval"}:
        return [_from_locomem_like(row, suite=suite) for row in rows]
    if suite in {"longmemeval", "long_mem_eval"}:
        return [_from_longmemeval_like(row, suite=suite) for row in rows]
    return [_from_generic(row, suite=suite) for row in rows]


def _from_locomem_like(row: dict[str, Any], *, suite: str) -> ExternalBenchmarkCase:
    return ExternalBenchmarkCase(
        case_id=str(row.get("id") or row.get("case_id") or row.get("question_id") or ""),
        suite=suite,
        query=str(row.get("question") or row.get("query") or ""),
        expected_answer=str(row.get("answer") or row.get("expected_answer") or ""),
        evidence_ids=[str(item) for item in row.get("evidence") or row.get("evidence_ids") or []],
        should_answer=bool(row.get("should_answer", row.get("answerable", True))),
        metadata={key: value for key, value in row.items() if key not in {"id", "question", "answer", "evidence", "should_answer"}},
    )


def _from_longmemeval_like(row: dict[str, Any], *, suite: str) -> ExternalBenchmarkCase:
    return ExternalBenchmarkCase(
        case_id=str(row.get("question_id") or row.get("id") or row.get("case_id") or ""),
        suite=suite,
        query=str(row.get("query") or row.get("question") or ""),
        expected_answer=str(row.get("gold_answer") or row.get("answer") or row.get("expected_answer") or ""),
        evidence_ids=[str(item) for item in row.get("supporting_memory_ids") or row.get("evidence_ids") or []],
        should_answer=bool(row.get("answerable", row.get("should_answer", True))),
        metadata={
            key: value
            for key, value in row.items()
            if key not in {"question_id", "query", "gold_answer", "supporting_memory_ids", "answerable"}
        },
    )


def _from_generic(row: dict[str, Any], *, suite: str) -> ExternalBenchmarkCase:
    return ExternalBenchmarkCase(
        case_id=str(row.get("case_id") or row.get("id") or row.get("question_id") or ""),
        suite=suite,
        query=str(row.get("query") or row.get("question") or ""),
        expected_answer=str(row.get("expected_answer") or row.get("answer") or row.get("gold_answer") or ""),
        evidence_ids=[str(item) for item in row.get("evidence_ids") or row.get("evidence") or []],
        should_answer=bool(row.get("should_answer", row.get("answerable", True))),
        metadata=dict(row),
    )
