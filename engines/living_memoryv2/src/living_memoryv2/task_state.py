from __future__ import annotations

from contextlib import contextmanager
from dataclasses import dataclass, field
import json
from pathlib import Path
import sqlite3
import time
from typing import Any, Iterator


TASK_STATE_LIST_FIELDS = {
    "constraints",
    "current_plan",
    "open_subtasks",
    "completed_subtasks",
    "irreversible_decisions",
    "tool_evidence_ids",
    "conflict_markers",
}

TASK_STATE_STRING_FIELDS = {"objective", "status", "checkpoint_id"}


@dataclass
class TaskState:
    task_id: str
    scope: dict[str, Any] = field(default_factory=dict)
    objective: str = ""
    constraints: list[str] = field(default_factory=list)
    current_plan: list[str] = field(default_factory=list)
    open_subtasks: list[str] = field(default_factory=list)
    completed_subtasks: list[str] = field(default_factory=list)
    irreversible_decisions: list[str] = field(default_factory=list)
    tool_evidence_ids: list[str] = field(default_factory=list)
    conflict_markers: list[str] = field(default_factory=list)
    status: str = "active"
    checkpoint_id: str = ""
    checkpoint_sequence: int = 0
    created_at: float = 0.0
    updated_at: float = 0.0

    def __post_init__(self) -> None:
        self.task_id = str(self.task_id).strip()
        if not self.task_id:
            raise ValueError("task_id is required")
        self.scope = dict(self.scope or {})
        for field_name in TASK_STATE_LIST_FIELDS:
            setattr(self, field_name, _string_list(getattr(self, field_name)))
        self.objective = str(self.objective or "")
        self.status = str(self.status or "active")
        self.checkpoint_id = str(self.checkpoint_id or "")
        self.checkpoint_sequence = max(0, int(self.checkpoint_sequence or 0))
        now = time.time()
        self.created_at = float(self.created_at or now)
        self.updated_at = float(self.updated_at or self.created_at)

    def to_dict(self) -> dict[str, Any]:
        return {
            "task_id": self.task_id,
            "scope": self.scope,
            "objective": self.objective,
            "constraints": list(self.constraints),
            "current_plan": list(self.current_plan),
            "open_subtasks": list(self.open_subtasks),
            "completed_subtasks": list(self.completed_subtasks),
            "irreversible_decisions": list(self.irreversible_decisions),
            "tool_evidence_ids": list(self.tool_evidence_ids),
            "conflict_markers": list(self.conflict_markers),
            "status": self.status,
            "checkpoint_id": self.checkpoint_id,
            "checkpoint_sequence": self.checkpoint_sequence,
            "created_at": self.created_at,
            "updated_at": self.updated_at,
        }

    def compact(self, *, max_items: int = 6) -> dict[str, Any]:
        return {
            "task_id": self.task_id,
            "status": self.status,
            "objective": self.objective,
            "constraints": self.constraints[:max_items],
            "current_plan": self.current_plan[:max_items],
            "open_subtasks": self.open_subtasks[:max_items],
            "completed_subtasks": self.completed_subtasks[:max_items],
            "irreversible_decisions": self.irreversible_decisions[:max_items],
            "tool_evidence_ids": self.tool_evidence_ids[:max_items],
            "conflict_markers": self.conflict_markers[:max_items],
            "checkpoint_id": self.checkpoint_id,
            "checkpoint_sequence": self.checkpoint_sequence,
            "updated_at": self.updated_at,
        }

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "TaskState":
        return cls(
            task_id=str(data.get("task_id") or ""),
            scope=dict(data.get("scope") or {}),
            objective=str(data.get("objective") or ""),
            constraints=_string_list(data.get("constraints")),
            current_plan=_string_list(data.get("current_plan")),
            open_subtasks=_string_list(data.get("open_subtasks")),
            completed_subtasks=_string_list(data.get("completed_subtasks")),
            irreversible_decisions=_string_list(data.get("irreversible_decisions")),
            tool_evidence_ids=_string_list(data.get("tool_evidence_ids")),
            conflict_markers=_string_list(data.get("conflict_markers")),
            status=str(data.get("status") or "active"),
            checkpoint_id=str(data.get("checkpoint_id") or ""),
            checkpoint_sequence=int(data.get("checkpoint_sequence") or 0),
            created_at=float(data.get("created_at") or 0.0),
            updated_at=float(data.get("updated_at") or 0.0),
        )


class TaskStateStore:
    def __init__(self, db_path: str | Path) -> None:
        self.db_path = Path(db_path)
        self.db_path.parent.mkdir(parents=True, exist_ok=True)
        self._init_db()

    @contextmanager
    def _connection(self) -> Iterator[sqlite3.Connection]:
        conn = sqlite3.connect(self.db_path)
        conn.row_factory = sqlite3.Row
        try:
            yield conn
            conn.commit()
        finally:
            conn.close()

    def _init_db(self) -> None:
        with self._connection() as conn:
            conn.execute(
                """
                CREATE TABLE IF NOT EXISTS task_states (
                    task_id TEXT PRIMARY KEY,
                    scope_json TEXT NOT NULL,
                    state_json TEXT NOT NULL,
                    checkpoint_sequence INTEGER NOT NULL,
                    created_at REAL NOT NULL,
                    updated_at REAL NOT NULL
                )
                """
            )
            conn.execute("CREATE INDEX IF NOT EXISTS idx_task_states_updated ON task_states(updated_at)")

    def get(self, task_id: str) -> TaskState | None:
        with self._connection() as conn:
            row = conn.execute("SELECT state_json FROM task_states WHERE task_id = ?", (str(task_id),)).fetchone()
        if row is None:
            return None
        return TaskState.from_dict(json.loads(row["state_json"]))

    def update(
        self,
        task_id: str,
        *,
        scope: dict[str, Any] | None = None,
        patch: dict[str, Any] | None = None,
    ) -> TaskState:
        patch = dict(patch or {})
        existing = self.get(task_id)
        if existing is None:
            now = time.time()
            state = TaskState(task_id=task_id, scope=dict(scope or {}), created_at=now, updated_at=now)
        else:
            state = TaskState.from_dict(existing.to_dict())
            if scope:
                state.scope = dict(scope)

        requested_sequence = patch.get("checkpoint_sequence")
        if requested_sequence is not None:
            requested_sequence = int(requested_sequence)
            if existing is not None and requested_sequence < state.checkpoint_sequence:
                raise ValueError(
                    f"stale checkpoint for task_id={state.task_id}: "
                    f"{requested_sequence} < {state.checkpoint_sequence}"
                )
            state.checkpoint_sequence = requested_sequence
        else:
            state.checkpoint_sequence = max(1, state.checkpoint_sequence + 1)

        for field_name in TASK_STATE_STRING_FIELDS:
            if field_name in patch:
                setattr(state, field_name, str(patch.get(field_name) or ""))
        for field_name in TASK_STATE_LIST_FIELDS:
            if field_name in patch:
                setattr(state, field_name, _string_list(patch.get(field_name)))
        if "scope" in patch:
            state.scope = dict(patch.get("scope") or {})
        if not state.checkpoint_id:
            state.checkpoint_id = f"ckpt_{state.checkpoint_sequence}"
        state.updated_at = time.time()
        self._save(state)
        return state

    def count(self) -> int:
        with self._connection() as conn:
            row = conn.execute("SELECT COUNT(*) AS total FROM task_states").fetchone()
        return int(row["total"] if row is not None else 0)

    def _save(self, state: TaskState) -> None:
        payload = json.dumps(state.to_dict(), sort_keys=True, ensure_ascii=False)
        scope_json = json.dumps(state.scope, sort_keys=True, ensure_ascii=False)
        with self._connection() as conn:
            conn.execute(
                """
                INSERT OR REPLACE INTO task_states (
                    task_id, scope_json, state_json, checkpoint_sequence, created_at, updated_at
                )
                VALUES (?, ?, ?, ?, ?, ?)
                """,
                (
                    state.task_id,
                    scope_json,
                    payload,
                    state.checkpoint_sequence,
                    state.created_at,
                    state.updated_at,
                ),
            )


def _string_list(value: Any) -> list[str]:
    if value is None:
        return []
    if isinstance(value, str):
        return [value] if value else []
    if isinstance(value, (list, tuple, set)):
        return [str(item) for item in value if str(item)]
    return [str(value)]
