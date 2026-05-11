import unittest

from _path import SRC  # noqa: F401
from living_memoryv2.tool_curriculum import ToolCurriculum, ToolSpec, ToolUseTrace


class ToolCurriculumTests(unittest.TestCase):
    def test_warmup_selects_only_task_tools(self):
        curriculum = ToolCurriculum(
            [
                ToolSpec("CAL", "math", "calculate arithmetic expressions", ("expression",)),
                ToolSpec("WEATHER", "weather", "get city weather by date", ("city", "date")),
            ]
        )

        tools = curriculum.select_tools("calculate the final amount", task="math", stage="warmup")

        self.assertEqual([tool.name for tool in tools], ["CAL"])

    def test_in_category_adds_same_task_distractors(self):
        curriculum = ToolCurriculum(
            [
                ToolSpec("TEMPERATURE", "weather", "get temperature by city and date", ("city", "date")),
                ToolSpec("HUMIDITY", "weather", "get humidity by city and date", ("city", "date")),
                ToolSpec("CAL", "math", "calculate arithmetic expressions", ("expression",)),
            ]
        )

        tools = curriculum.select_tools(
            "temperature in Sao Paulo",
            task="weather",
            stage="in_category",
            top_k=2,
            distractors=1,
        )

        self.assertEqual({tool.task for tool in tools}, {"weather"})
        self.assertEqual(len(tools), 2)

    def test_cross_category_includes_distractor_from_other_task(self):
        curriculum = ToolCurriculum(
            [
                ToolSpec("TEMPERATURE", "weather", "get temperature by city and date", ("city", "date")),
                ToolSpec("HUMIDITY", "weather", "get humidity by city and date", ("city", "date")),
                ToolSpec("CAL", "math", "calculate arithmetic expressions", ("expression",)),
            ]
        )

        tools = curriculum.select_tools(
            "temperature average",
            task="weather",
            stage="cross_category",
            top_k=2,
            distractors=1,
        )

        self.assertIn("weather", {tool.task for tool in tools})
        self.assertIn("math", {tool.task for tool in tools})

    def test_introspection_turns_failures_into_lessons(self):
        curriculum = ToolCurriculum(
            [ToolSpec("QUERY", "search", "search external documents", ("query",))]
        )
        curriculum.record_trace(
            ToolUseTrace(
                query="find source",
                selected_tools=["QUERY"],
                expected_tools=["QUERY"],
                success=False,
                stage="cross_category",
                error_type="missing_parameter",
                feedback="query parameter was empty",
            )
        )

        feedback = curriculum.introspect_failures()
        lessons = curriculum.lesson_memories()

        self.assertEqual(feedback[0].tool_name, "QUERY")
        self.assertEqual(feedback[0].error_types, ["missing_parameter"])
        self.assertEqual(lessons[0]["memory_type"], "procedural")
        self.assertIn("QUERY", lessons[0]["lesson"])


if __name__ == "__main__":
    unittest.main()
