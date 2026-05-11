from __future__ import annotations

import json
from pathlib import Path
import sys
import tempfile

ROOT = Path(__file__).resolve().parents[1]
SRC = ROOT / "src"
if str(SRC) not in sys.path:
    sys.path.insert(0, str(SRC))

from living_memoryv2 import ContextAssembler, MemoryEnvelope, RecallPipeline, TemporalStore


def main() -> None:
    with tempfile.TemporaryDirectory() as tmp:
        store = TemporalStore(Path(tmp))

        store.remember(
            MemoryEnvelope.text(
                "Marina presentation is on May 15.",
                scope={"project_id": "psi", "user_id": "cinth"},
                tags=["calendar", "marina"],
                memory_type="episodic",
                importance=0.82,
                provenance={"source": "demo"},
            )
        )
        store.remember(
            MemoryEnvelope.text(
                "Decision: do not compress canonical memory in phase 1.",
                scope={"project_id": "psi", "user_id": "cinth"},
                tags=["architecture", "compression"],
                memory_type="decision",
                importance=0.99,
                provenance={"source": "demo"},
            )
        )
        store.remember(
            MemoryEnvelope.file_ref(
                uri="file:///workspace/design.png",
                mime_type="image/png",
                scope={"project_id": "psi", "user_id": "cinth"},
                tags=["artifact", "diagram"],
                importance=0.55,
                provenance={"source": "demo"},
            )
        )

        results = RecallPipeline(store).recall(
            "memory compression architecture",
            scope={"project_id": "psi"},
            top_k=5,
        )
        packets = ContextAssembler(max_tokens=120).assemble(results)
        print(json.dumps([packet.to_dict() for packet in packets], indent=2, ensure_ascii=False))


if __name__ == "__main__":
    main()
