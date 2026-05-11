from __future__ import annotations

from typing import Any


def calibration_report(cases: list[dict[str, Any]], *, bins: int = 10) -> dict[str, Any]:
    normalized = [_normalize_case(item) for item in cases]
    return {
        "case_count": len(normalized),
        "ece": round(expected_calibration_error(normalized, bins=bins), 6),
        "unsupported_claim_count": sum(
            1 for item in normalized if item["predicted_supported"] and not item["expected_supported"]
        ),
        "risk_coverage": risk_coverage_curve(normalized),
    }


def expected_calibration_error(cases: list[dict[str, Any]], *, bins: int = 10) -> float:
    if not cases:
        return 0.0
    bins = max(1, int(bins))
    total = len(cases)
    error = 0.0
    for index in range(bins):
        lower = index / bins
        upper = (index + 1) / bins
        if index == bins - 1:
            bucket = [item for item in cases if lower <= item["score"] <= upper]
        else:
            bucket = [item for item in cases if lower <= item["score"] < upper]
        if not bucket:
            continue
        accuracy = sum(1 for item in bucket if item["correct"]) / len(bucket)
        confidence = sum(item["score"] for item in bucket) / len(bucket)
        error += (len(bucket) / total) * abs(accuracy - confidence)
    return error


def risk_coverage_curve(cases: list[dict[str, Any]]) -> list[dict[str, Any]]:
    if not cases:
        return []
    ordered = sorted(cases, key=lambda item: item["score"], reverse=True)
    points: list[dict[str, Any]] = []
    for index in range(1, len(ordered) + 1):
        prefix = ordered[:index]
        accuracy = sum(1 for item in prefix if item["correct"]) / index
        points.append(
            {
                "coverage": round(index / len(ordered), 4),
                "accuracy": round(accuracy, 4),
                "threshold": round(prefix[-1]["score"], 4),
            }
        )
    return points


def _normalize_case(item: dict[str, Any]) -> dict[str, Any]:
    score = max(0.0, min(1.0, float(item.get("score") or 0.0)))
    predicted = bool(item.get("predicted_supported"))
    expected = bool(item.get("expected_supported"))
    return {
        "score": score,
        "predicted_supported": predicted,
        "expected_supported": expected,
        "correct": predicted == expected,
    }
