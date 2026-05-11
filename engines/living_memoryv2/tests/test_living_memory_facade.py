import os
import tempfile
import unittest

from _path import SRC  # noqa: F401
from living_memoryv2.mcp_server import LivingMemoryTools


class LivingMemoryFacadeTests(unittest.TestCase):
    def test_work_mode_returns_context_graph_and_lessons_in_one_call(self):
        with tempfile.TemporaryDirectory() as tmp:
            tools = LivingMemoryTools(tmp)
            remembered = tools.remember_text(
                content="living_memoryv2 deve expor uma fachada MCP unica para entregar contexto, grafo e pistas de ferramentas.",
                scope={"project_id": "psi"},
                tags=["mcp", "facade"],
                memory_type="decision",
                importance=0.86,
                confidence=0.94,
                provenance={"source": "unit-test"},
                relations=[
                    {
                        "subject": "living_memoryv2",
                        "predicate": "uses",
                        "object": "one-call facade",
                    }
                ],
            )
            tools.register_tool(
                name="pytest",
                task="testing",
                description="Executa testes Python do living_memoryv2.",
                parameters=["path"],
                tags=["tests"],
            )
            tools.record_tool_trace(
                query="rodar testes do living_memoryv2",
                selected_tools=["pytest"],
                success=False,
                stage="warmup",
                expected_tools=["pytest"],
                error_type="missing_path",
                feedback="Sempre informe o caminho dos testes antes de executar pytest.",
            )

            result = tools.living_memory(
                query="fachada MCP unica para contexto e ferramentas",
                scope={"project_id": "psi"},
                max_tokens=160,
                top_k=6,
            )

            self.assertEqual(result["format"], "living-memoryv2/facade-1")
            self.assertEqual(result["mode"], "work")
            self.assertEqual(result["phase"], "work_start")
            self.assertIn("answerability", result)
            self.assertEqual(result["answerability"]["status"], "supported")
            self.assertIn("LMV2_SIF v1", result["context"]["sif_text"])
            self.assertEqual(result["context_v2"]["format"], "lmv2-sif/2")
            self.assertIn("knowledge", result)
            self.assertGreaterEqual(result["knowledge"]["assertion_count"], 1)
            self.assertTrue(any(item["predicate"] == "uses" for item in result["knowledge"]["assertions"]))
            self.assertIn(remembered["memory_id"], result["inspectable_ids"])
            self.assertEqual(result["evidence"][0]["memory_id"], remembered["memory_id"])
            self.assertIn("facade_score", result["evidence"][0])
            self.assertIn("layer_signals", result["evidence"][0])
            self.assertIn("topology", result["evidence"][0])
            self.assertIn("mobius_address", result["evidence"][0]["topology"])
            self.assertTrue(any(item["predicate"] == "uses" for item in result["graph_hints"]))
            self.assertTrue(any("pytest" in item["tool_name"] for item in result["procedural_lessons"]))
            self.assertIn("recall", result["tools_used_internal"])
            self.assertIn("sif_v2", result["tools_used_internal"])
            self.assertIn("knowledge_graph", result["tools_used_internal"])
            self.assertEqual(result["suggested_next_calls"], [])

    def test_capture_mode_persists_final_notes_decisions_lessons_and_relations(self):
        with tempfile.TemporaryDirectory() as tmp:
            tools = LivingMemoryTools(tmp)

            result = tools.living_memory(
                mode="capture",
                scope={"project_id": "psi"},
                notes=["A fachada living_memory deve ser a entrada principal para economizar chamadas MCP."],
                decisions=["Manter as ferramentas antigas, mas privilegiar a fachada de chamada unica."],
                lessons=["Depois de uma tarefa complexa, registrar somente notas pequenas e acionaveis."],
                relations=[
                    {
                        "subject": "living_memoryv2",
                        "predicate": "prefers",
                        "object": "one-call facade",
                        "memory_ref": 0,
                    }
                ],
            )

            self.assertEqual(result["mode"], "capture")
            self.assertEqual(len(result["captured_memory_ids"]), 3)
            self.assertEqual(len(result["relations_added"]), 1)
            decision = tools.inspect_memory(memory_id=result["captured_memory_ids"][1])
            lesson = tools.inspect_memory(memory_id=result["captured_memory_ids"][2])
            self.assertEqual(decision["memory_type"], "decision")
            self.assertEqual(lesson["memory_type"], "procedural")
            self.assertIn("annotation", decision["tags"])
            self.assertIn("lesson", lesson["tags"])

    def test_capture_mode_accepts_common_relation_aliases(self):
        with tempfile.TemporaryDirectory() as tmp:
            tools = LivingMemoryTools(tmp)

            result = tools.living_memory(
                mode="capture",
                scope={"project_id": "psi"},
                notes=["Relation aliases should normalize into the canonical graph schema."],
                relations=[
                    {
                        "from": "deep-research-report.md",
                        "type": "guides",
                        "to": "agent-memory/engines/living_memoryv2",
                        "memory_ref": "last",
                    }
                ],
            )

            self.assertEqual(len(result["relations_added"]), 1)
            relation = result["relations_added"][0]
            self.assertEqual(relation["subject"], "deep-research-report.md")
            self.assertEqual(relation["predicate"], "guides")
            self.assertEqual(relation["object"], "agent-memory/engines/living_memoryv2")

    def test_capture_mode_reports_missing_relation_fields_clearly(self):
        with tempfile.TemporaryDirectory() as tmp:
            tools = LivingMemoryTools(tmp)

            with self.assertRaisesRegex(ValueError, "subject/predicate/object"):
                tools.living_memory(
                    mode="capture",
                    scope={"project_id": "psi"},
                    notes=["Malformed relation should produce an actionable error."],
                    relations=[{"subject": "living_memoryv2"}],
                )

    def test_capture_updates_task_state_and_work_returns_compact_state(self):
        with tempfile.TemporaryDirectory() as tmp:
            tools = LivingMemoryTools(tmp)

            capture = tools.living_memory(
                mode="capture",
                scope={"project_id": "psi"},
                task_id="task_alpha",
                task_state_update={
                    "objective": "Implement TaskState v1 without bifurcating the engine.",
                    "constraints": ["preserve SIF v1", "keep MemoryEnvelope canonical"],
                    "current_plan": ["write tests", "wire facade"],
                    "open_subtasks": ["document contract"],
                    "irreversible_decisions": ["canonical path is agent-memory/engines/living_memoryv2"],
                    "tool_evidence_ids": ["mem_evidence_1"],
                    "checkpoint_id": "ckpt_1",
                    "checkpoint_sequence": 1,
                },
            )

            work = tools.living_memory(
                query="continue TaskState implementation",
                scope={"project_id": "psi"},
                task_id="task_alpha",
                max_tokens=160,
                top_k=3,
            )

            self.assertEqual(capture["task_state"]["task_id"], "task_alpha")
            self.assertEqual(capture["task_state"]["checkpoint_id"], "ckpt_1")
            self.assertEqual(work["task_state"]["objective"], "Implement TaskState v1 without bifurcating the engine.")
            self.assertEqual(work["task_state"]["constraints"], ["preserve SIF v1", "keep MemoryEnvelope canonical"])
            self.assertEqual(work["task_state"]["open_subtasks"], ["document contract"])
            self.assertIn("task_state", work["tools_used_internal"])

    def test_capture_v2_records_agentic_work_and_links_knowledge(self):
        with tempfile.TemporaryDirectory() as tmp:
            tools = LivingMemoryTools(tmp)

            result = tools.living_memory(
                mode="capture",
                capture_type="agentic_work",
                scope={"project_id": "psi"},
                task_id="task_capture_v2",
                spec={
                    "title": "Capture v2 agentic work",
                    "goal": "Let agents write specs, plans, checkpoints, risks, artifacts, and questions.",
                    "acceptance_criteria": ["typed memories", "auto relations"],
                },
                plan={
                    "title": "Implement Capture v2",
                    "steps": ["write failing test", "implement contract", "run benchmark"],
                    "status": "in_progress",
                },
                progress={
                    "completed": ["failing test"],
                    "open": ["implementation"],
                },
                checkpoint={
                    "title": "Capture contract approved",
                    "status": "verified",
                    "evidence_ids": ["mem_external_evidence"],
                },
                artifacts=[
                    {
                        "title": "capture v2 test",
                        "artifact_path": "agent-memory/engines/living_memoryv2/tests/test_living_memory_facade.py",
                        "kind": "test",
                        "evidence_ids": ["mem_external_evidence"],
                    }
                ],
                risks=[{"title": "over-capture", "severity": "medium", "mitigation": "quality report"}],
                open_questions=["Should capture require approval for irreversible decisions?"],
                decisions=[
                    {
                        "content": "Capture v2 is additive and keeps old capture fields stable.",
                        "evidence_ids": ["mem_external_evidence"],
                    }
                ],
                lessons=["Agentic capture should reject ephemeral scratchpad notes."],
            )

            self.assertEqual(result["mode"], "capture")
            self.assertIn("capture_v2", result)
            roles = result["capture_v2"]["captured_by_role"]
            for role in ("spec", "plan", "progress", "checkpoint", "artifact", "risk", "open_question", "decision", "lesson"):
                self.assertIn(role, roles)
                self.assertGreaterEqual(len(roles[role]), 1)
            quality = result["capture_v2"]["quality"]
            self.assertGreaterEqual(quality["durable_items"], 9)
            self.assertEqual(quality["ignored_items"], 0)
            self.assertEqual(quality["task_state_synced"], True)
            self.assertGreaterEqual(quality["relations_added"], 6)
            self.assertIn("capture_v2", result["tools_used_internal"])
            self.assertIn("knowledge_graph", result["tools_used_internal"])

            plan_id = roles["plan"][0]
            spec_id = roles["spec"][0]
            checkpoint_id = roles["checkpoint"][0]
            plan_memory = tools.inspect_memory(memory_id=plan_id)
            self.assertEqual(plan_memory["memory_type"], "plan")
            self.assertIn("agentic", plan_memory["tags"])

            relations = tools.query_knowledge(scope={"project_id": "psi"}, status="active")
            relation_pairs = {
                (item["subject"], item["predicate"], item["object"])
                for item in relations["assertions"]
            }
            self.assertIn((plan_id, "implements", spec_id), relation_pairs)
            self.assertIn((checkpoint_id, "verifies", plan_id), relation_pairs)
            self.assertEqual(result["task_state"]["task_id"], "task_capture_v2")
            self.assertIn("Implement Capture v2", result["task_state"]["objective"])

    def test_work_without_task_id_keeps_task_state_absent(self):
        with tempfile.TemporaryDirectory() as tmp:
            tools = LivingMemoryTools(tmp)

            result = tools.living_memory(
                query="no task id keeps old facade shape",
                scope={"project_id": "psi"},
                max_tokens=160,
                top_k=3,
            )

            self.assertNotIn("task_state", result)
            self.assertNotIn("task_state", result["tools_used_internal"])

    def test_work_mode_honors_micro_context_budget_without_losing_id(self):
        with tempfile.TemporaryDirectory() as tmp:
            tools = LivingMemoryTools(tmp)
            remembered = tools.remember_text(
                content="Project memory says agents should start non-trivial work with living_memory(mode='work').",
                scope={"project_id": "psi"},
                tags=["project-memory", "mcp"],
                memory_type="decision",
                importance=0.9,
                confidence=0.95,
                provenance={"source": "AGENTS.md"},
            )

            result = tools.living_memory(
                query="What is the primary MCP call pattern for non-trivial work?",
                scope={"project_id": "psi"},
                max_tokens=40,
                top_k=1,
            )

            self.assertEqual(result["token_budget"], 40)
            self.assertEqual(result["context"]["token_budget"], 40)
            self.assertLessEqual(result["context"]["estimated_tokens"], 40)
            self.assertIn(remembered["memory_id"], result["inspectable_ids"])
            self.assertIn(remembered["memory_id"], result["context"]["memory_ids"])

    def test_compact_descriptor_mode_keeps_facade_visible_without_removing_handlers(self):
        with tempfile.TemporaryDirectory() as tmp:
            previous = os.environ.get("LIVING_MEMORY_COMPACT_TOOLS")
            os.environ["LIVING_MEMORY_COMPACT_TOOLS"] = "1"
            try:
                tools = LivingMemoryTools(tmp)
                names = {item["name"] for item in tools.tool_descriptors()}
            finally:
                if previous is None:
                    os.environ.pop("LIVING_MEMORY_COMPACT_TOOLS", None)
                else:
                    os.environ["LIVING_MEMORY_COMPACT_TOOLS"] = previous

            self.assertIn("living_memory", names)
            self.assertIn("inspect_memory", names)
            self.assertNotIn("remember_text", names)
            self.assertIn("remember_text", tools.tools())

    def test_work_mode_returns_answerable_subgraphs_from_named_graphs(self):
        with tempfile.TemporaryDirectory() as tmp:
            tools = LivingMemoryTools(tmp)
            remembered = tools.remember_text(
                content="Whiteboard artifact shows local-first multimodal memory routing for agent context.",
                scope={"project_id": "psi"},
                memory_type="artifact",
                tags=["multimodal", "whiteboard"],
                importance=0.8,
                relations=[
                    {
                        "subject": "artifact:whiteboard",
                        "predicate": "has_caption",
                        "object": "local-first multimodal memory routing",
                        "graph_name": "multimodal",
                    },
                    {
                        "subject": "artifact:whiteboard",
                        "predicate": "proves",
                        "object": "agent_context_route",
                        "graph_name": "provenance",
                    },
                ],
            )

            result = tools.living_memory(
                query="local-first multimodal memory routing",
                scope={"project_id": "psi"},
                max_tokens=220,
                top_k=4,
            )

            self.assertIn(remembered["memory_id"], result["inspectable_ids"])
            self.assertIn("subgraphs", result)
            graph_names = {item["graph_name"] for item in result["subgraphs"]}
            self.assertIn("multimodal", graph_names)
            self.assertTrue(all(item["answerability"] > 0 for item in result["subgraphs"]))

    def test_work_mode_accepts_phase_and_returns_debug_feature_trace(self):
        with tempfile.TemporaryDirectory() as tmp:
            tools = LivingMemoryTools(tmp)
            remembered = tools.remember_text(
                content="Planning policy: preserve canonical memories and add only derived indexes.",
                scope={"project_id": "psi"},
                tags=["planning", "policy"],
                memory_type="decision",
                importance=0.86,
                confidence=0.94,
            )

            result = tools.living_memory(
                query="planning policy canonical memories derived indexes",
                scope={"project_id": "psi"},
                phase="planning",
                debug_trace=True,
                max_tokens=220,
                top_k=4,
            )

            self.assertEqual(result["phase"], "planning")
            self.assertEqual(result["answerability"]["status"], "supported")
            self.assertIn("feature_trace", result)
            self.assertTrue(any(item["memory_id"] == remembered["memory_id"] for item in result["feature_trace"]))


if __name__ == "__main__":
    unittest.main()
