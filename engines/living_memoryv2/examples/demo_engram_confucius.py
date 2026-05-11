from __future__ import annotations

import json
from pathlib import Path
import shutil
import sys

ROOT = Path(__file__).resolve().parents[1]
SRC = ROOT / "src"
if str(SRC) not in sys.path:
    sys.path.insert(0, str(SRC))

from living_memoryv2 import (  # noqa: E402
    EngramCache,
    MemoryEnvelope,
    RecallPipeline,
    TemporalStore,
    ToolCurriculum,
    ToolSpec,
    ToolUseTrace,
)


def main() -> None:
    runtime = ROOT / ".demo_runtime" / "engram_confucius"
    if runtime.exists():
        shutil.rmtree(runtime)
    runtime.mkdir(parents=True)

    store = TemporalStore(runtime / "store")
    memory = MemoryEnvelope.text(
        "Ao montar contexto para agentes, preserve episodio original, provenance e tool traces.",
        scope={"project_id": "living_memoryv2"},
        tags=["architecture", "agentic"],
        memory_type="decision",
        importance=0.8,
        provenance={"source": "demo_engram_confucius"},
    )
    store.remember(memory)

    engram = EngramCache(cache_threshold=1)
    recall = RecallPipeline(store, engram_cache=engram)
    recall.recall("provenance tool traces agentes", scope={"project_id": "living_memoryv2"})
    second = recall.recall("provenance tool traces agentes", scope={"project_id": "living_memoryv2"})

    curriculum = ToolCurriculum(
        [
            ToolSpec("RECALL", "memory", "retrieve evidence packets from living memory", ("query",)),
            ToolSpec("INSPECT", "memory", "inspect a canonical memory by id", ("memory_id",)),
            ToolSpec("SEARCH", "web", "search external references", ("query",)),
        ]
    )
    tools = curriculum.select_tools(
        "recall evidence about agent traces",
        task="memory",
        stage="cross_category",
        top_k=3,
        distractors=1,
    )
    curriculum.record_trace(
        ToolUseTrace(
            query="inspect previous evidence",
            selected_tools=["INSPECT"],
            expected_tools=["INSPECT"],
            success=False,
            stage="cross_category",
            error_type="missing_memory_id",
            feedback="The agent tried to inspect without carrying the memory_id from recall.",
        )
    )

    output = {
        "recall": [
            {
                "id": item.memory.id,
                "why": item.why_retrieved,
                "text": item.memory.text_content(),
            }
            for item in second
        ],
        "engram_stats": engram.stats(),
        "selected_tools": [tool.prompt_line() for tool in tools],
        "lesson_memories": curriculum.lesson_memories(),
    }
    print(json.dumps(output, indent=2, ensure_ascii=False))


if __name__ == "__main__":
    main()
