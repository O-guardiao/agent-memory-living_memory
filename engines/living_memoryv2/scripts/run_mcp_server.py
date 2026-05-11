from __future__ import annotations

from pathlib import Path
import sys


ROOT = Path(__file__).resolve().parents[1]
SRC = ROOT / "src"
if str(SRC) not in sys.path:
    sys.path.insert(0, str(SRC))

from living_memoryv2.mcp_server import run_stdio  # noqa: E402


if __name__ == "__main__":
    run_stdio()
