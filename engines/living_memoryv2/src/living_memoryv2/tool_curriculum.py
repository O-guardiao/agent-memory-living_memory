from __future__ import annotations

from dataclasses import dataclass, field
import re
import time
from typing import Any


@dataclass(frozen=True)
class ToolSpec:
    name: str
    task: str
    description: str
    parameters: tuple[str, ...] = ()
    difficulty: int = 1
    tags: tuple[str, ...] = ()

    def prompt_line(self) -> str:
        params = ", ".join(self.parameters)
        signature = f"{self.name}({params})" if params else f"{self.name}()"
        return f"{signature}: {self.description}"


@dataclass
class ToolUseTrace:
    query: str
    selected_tools: list[str]
    success: bool
    stage: str
    expected_tools: list[str] = field(default_factory=list)
    error_type: str = ""
    feedback: str = ""
    created_at: float = field(default_factory=time.time)
    metadata: dict[str, Any] = field(default_factory=dict)


@dataclass
class IntrospectionFeedback:
    tool_name: str
    failures: int
    error_types: list[str]
    feedback: list[str]


class ToolCurriculum:
    """Confucius-style tool curriculum for inference-time agents."""

    VALID_STAGES = {"warmup", "in_category", "cross_category"}

    def __init__(self, tools: list[ToolSpec] | None = None) -> None:
        self.tools: dict[str, ToolSpec] = {}
        self.traces: list[ToolUseTrace] = []
        for tool in tools or []:
            self.register_tool(tool)

    def register_tool(self, tool: ToolSpec) -> None:
        if not tool.name:
            raise ValueError("tool name is required")
        if tool.difficulty < 1:
            raise ValueError("tool difficulty must be >= 1")
        self.tools[tool.name] = tool

    def select_tools(
        self,
        query: str,
        *,
        task: str | None = None,
        stage: str = "warmup",
        top_k: int = 8,
        distractors: int = 2,
    ) -> list[ToolSpec]:
        if stage not in self.VALID_STAGES:
            raise ValueError(f"unknown curriculum stage: {stage}")
        if top_k <= 0:
            return []

        ranked = sorted(
            self.tools.values(),
            key=lambda tool: self._score_tool(query, tool, task=task),
            reverse=True,
        )

        if stage == "warmup":
            selected = [tool for tool in ranked if task is None or tool.task == task]
            return selected[:top_k]

        relevant = [tool for tool in ranked if task is None or tool.task == task]
        selected = relevant[: max(1, top_k - distractors)]

        if stage == "in_category":
            pool = [tool for tool in relevant if tool not in selected]
        else:
            pool = [tool for tool in ranked if tool not in selected and (task is None or tool.task != task)]

        selected.extend(pool[: max(0, min(distractors, top_k - len(selected)))])
        return selected[:top_k]

    def record_trace(self, trace: ToolUseTrace) -> None:
        if trace.stage not in self.VALID_STAGES:
            raise ValueError(f"unknown curriculum stage: {trace.stage}")
        self.traces.append(trace)

    def introspect_failures(self, *, min_failures: int = 1) -> list[IntrospectionFeedback]:
        failures_by_tool: dict[str, list[ToolUseTrace]] = {}
        for trace in self.traces:
            if trace.success:
                continue
            tool_names = trace.expected_tools or trace.selected_tools
            for tool_name in tool_names:
                failures_by_tool.setdefault(tool_name, []).append(trace)

        feedback: list[IntrospectionFeedback] = []
        for tool_name, traces in failures_by_tool.items():
            if len(traces) < min_failures:
                continue
            error_types = sorted({trace.error_type for trace in traces if trace.error_type})
            notes = [trace.feedback for trace in traces if trace.feedback]
            feedback.append(
                IntrospectionFeedback(
                    tool_name=tool_name,
                    failures=len(traces),
                    error_types=error_types,
                    feedback=notes,
                )
            )
        return sorted(feedback, key=lambda item: (item.failures, item.tool_name), reverse=True)

    def lesson_memories(self, *, min_failures: int = 1) -> list[dict[str, Any]]:
        lessons: list[dict[str, Any]] = []
        for item in self.introspect_failures(min_failures=min_failures):
            tool = self.tools.get(item.tool_name)
            lessons.append(
                {
                    "memory_type": "procedural",
                    "tool_name": item.tool_name,
                    "task": tool.task if tool else None,
                    "lesson": self._lesson_text(item, tool),
                    "failures": item.failures,
                    "error_types": item.error_types,
                    "source": "confucius_style_introspection",
                }
            )
        return lessons

    def _score_tool(self, query: str, tool: ToolSpec, *, task: str | None) -> float:
        query_terms = set(self._terms(query))
        tool_terms = set(self._terms(" ".join([tool.name, tool.task, tool.description, " ".join(tool.tags)])))
        lexical = len(query_terms & tool_terms) / max(1, len(query_terms))
        task_boost = 0.25 if task is not None and tool.task == task else 0.0
        difficulty_penalty = min(tool.difficulty, 10) * 0.01
        failure_boost = min(0.20, self._failure_count(tool.name) * 0.04)
        return lexical + task_boost + failure_boost - difficulty_penalty

    def _failure_count(self, tool_name: str) -> int:
        return sum(
            1
            for trace in self.traces
            if not trace.success and tool_name in (trace.expected_tools or trace.selected_tools)
        )

    def _lesson_text(self, item: IntrospectionFeedback, tool: ToolSpec | None) -> str:
        description = tool.description if tool else "tool not registered"
        errors = ", ".join(item.error_types) if item.error_types else "unknown"
        return (
            f"Tool {item.tool_name} has repeated failures ({item.failures}). "
            f"Purpose: {description}. Common error types: {errors}."
        )

    def _terms(self, text: str) -> list[str]:
        return re.findall(r"[\w]+", text.lower(), flags=re.UNICODE)
