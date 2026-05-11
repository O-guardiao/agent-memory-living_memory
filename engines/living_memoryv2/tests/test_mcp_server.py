import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

from _path import SRC


class MCPServerTests(unittest.TestCase):
    def test_stdio_mcp_lists_and_calls_memory_tools(self):
        with tempfile.TemporaryDirectory() as tmp:
            proc = self._start_server(tmp)
            try:
                init = self._request(
                    proc,
                    1,
                    "initialize",
                    {
                        "protocolVersion": "2025-03-26",
                        "capabilities": {},
                        "clientInfo": {"name": "unit-test", "version": "0"},
                    },
                )
                self.assertEqual(init["result"]["serverInfo"]["name"], "living-memoryv2")

                self._notify(proc, "notifications/initialized")
                tools = self._request(proc, 2, "tools/list", {})
                tool_names = {tool["name"] for tool in tools["result"]["tools"]}
                self.assertIn("remember_text", tool_names)
                self.assertIn("living_memory", tool_names)
                self.assertIn("recall_context", tool_names)
                self.assertIn("recall_sif", tool_names)
                self.assertIn("inspect_memory", tool_names)
                self.assertIn("verify_integrity", tool_names)

                remembered = self._call_tool(
                    proc,
                    3,
                    "remember_text",
                    {
                        "content": "Codex should use living memory through small evidence packets.",
                        "scope": {"project_id": "psi"},
                        "tags": ["mcp", "context"],
                        "importance": 0.8,
                        "provenance": {"source": "unit-test"},
                    },
                )
                memory_id = remembered["memory_id"]
                self.assertTrue(memory_id.startswith("mem_"))

                context = self._call_tool(
                    proc,
                    4,
                    "recall_context",
                    {
                        "query": "small evidence packets",
                        "scope": {"project_id": "psi"},
                        "max_tokens": 60,
                    },
                )
                self.assertEqual(context["evidence"][0]["memory_id"], memory_id)
                self.assertIn("snippet", context["evidence"][0])
                self.assertLessEqual(context["estimated_tokens"], 60)

                sif = self._call_tool(
                    proc,
                    5,
                    "recall_sif",
                    {
                        "query": "small evidence packets",
                        "scope": {"project_id": "psi"},
                        "max_tokens": 80,
                    },
                )
                self.assertEqual(sif["memory_ids"][0], memory_id)
                self.assertIn("LMV2_SIF v1", sif["sif_text"])
                self.assertLessEqual(sif["estimated_tokens"], 80)

                facade = self._call_tool(
                    proc,
                    6,
                    "living_memory",
                    {
                        "query": "small evidence packets",
                        "scope": {"project_id": "psi"},
                        "max_tokens": 90,
                    },
                )
                self.assertEqual(facade["format"], "living-memoryv2/facade-1")
                self.assertEqual(facade["mode"], "work")
                self.assertEqual(facade["inspectable_ids"][0], memory_id)
                self.assertIn("LMV2_SIF v1", facade["context"]["sif_text"])

                inspected = self._call_tool(proc, 7, "inspect_memory", {"memory_id": memory_id})
                self.assertEqual(inspected["id"], memory_id)
                self.assertIn("small evidence packets", inspected["text"])

                integrity = self._call_tool(proc, 8, "verify_integrity", {})
                self.assertEqual(integrity["tampered"], [])
            finally:
                proc.terminate()
                try:
                    proc.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    proc.kill()
                for pipe in (proc.stdin, proc.stdout, proc.stderr):
                    if pipe is not None and not pipe.closed:
                        pipe.close()

    def _start_server(self, root: str) -> subprocess.Popen:
        env = os.environ.copy()
        env["PYTHONPATH"] = str(SRC)
        env["LIVING_MEMORY_ROOT"] = root
        return subprocess.Popen(
            [sys.executable, "-m", "living_memoryv2.mcp_server"],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            encoding="utf-8",
            env=env,
            cwd=str(Path(__file__).resolve().parents[2]),
        )

    def _request(self, proc: subprocess.Popen, request_id: int, method: str, params: dict) -> dict:
        message = {"jsonrpc": "2.0", "id": request_id, "method": method, "params": params}
        return self._send(proc, message)

    def _notify(self, proc: subprocess.Popen, method: str, params: dict | None = None) -> None:
        message = {"jsonrpc": "2.0", "method": method}
        if params is not None:
            message["params"] = params
        assert proc.stdin is not None
        proc.stdin.write(json.dumps(message) + "\n")
        proc.stdin.flush()

    def _call_tool(self, proc: subprocess.Popen, request_id: int, name: str, arguments: dict) -> dict:
        response = self._request(proc, request_id, "tools/call", {"name": name, "arguments": arguments})
        self.assertNotIn("error", response)
        text = response["result"]["content"][0]["text"]
        return json.loads(text)

    def _send(self, proc: subprocess.Popen, message: dict) -> dict:
        assert proc.stdin is not None
        assert proc.stdout is not None
        proc.stdin.write(json.dumps(message) + "\n")
        proc.stdin.flush()
        line = proc.stdout.readline()
        if not line:
            stderr = proc.stderr.read() if proc.stderr else ""
            raise AssertionError(f"MCP server exited without response. stderr={stderr}")
        return json.loads(line)


if __name__ == "__main__":
    unittest.main()
