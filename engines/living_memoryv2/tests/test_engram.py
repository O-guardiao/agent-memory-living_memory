import tempfile
import unittest
from pathlib import Path

from _path import SRC  # noqa: F401
from living_memoryv2.engram import EngramCache, NGramHasher
from living_memoryv2.recall import RecallPipeline
from living_memoryv2.schema import MemoryEnvelope
from living_memoryv2.store import TemporalStore


class NGramHasherTests(unittest.TestCase):
    def test_hash_is_deterministic_and_normalized(self):
        hasher = NGramHasher(table_size=1009)

        self.assertEqual(hasher.primary_index("Qual Meu Nome"), hasher.primary_index("qual meu nome"))
        self.assertEqual(hasher.all_indices("memoria viva"), hasher.all_indices("memoria viva"))


class EngramCacheTests(unittest.TestCase):
    def test_cache_rejects_hash_collision_with_signature(self):
        cache = EngramCache(table_size=1, cache_threshold=1)
        cache.cache("qual meu nome", ["mem_a"], scope={"project_id": "psi"})

        self.assertEqual(cache.lookup("qual meu nome", scope={"project_id": "psi"}), ["mem_a"])
        self.assertEqual(cache.lookup("onde eu moro", scope={"project_id": "psi"}), [])
        self.assertEqual(cache.stats()["collision_rejections"], 1)

    def test_record_result_auto_caches_after_threshold(self):
        cache = EngramCache(cache_threshold=2)

        self.assertFalse(cache.record_result("query quente", ["mem_a"]))
        self.assertTrue(cache.record_result("query quente", ["mem_a"]))
        self.assertEqual(cache.lookup("query quente"), ["mem_a"])
        self.assertEqual(cache.stats()["auto_cached"], 1)

    def test_scope_and_tags_are_part_of_signature(self):
        cache = EngramCache(cache_threshold=1)
        cache.cache("preferencias", ["mem_a"], scope={"project_id": "psi"}, tags=["profile"])

        self.assertEqual(cache.lookup("preferencias", scope={"project_id": "psi"}, tags=["profile"]), ["mem_a"])
        self.assertEqual(cache.lookup("preferencias", scope={"project_id": "other"}, tags=["profile"]), [])
        self.assertEqual(cache.lookup("preferencias", scope={"project_id": "psi"}, tags=["task"]), [])


class RecallEngramIntegrationTests(unittest.TestCase):
    def test_repeated_query_adds_engram_reason_without_training(self):
        with tempfile.TemporaryDirectory() as tmp:
            store = TemporalStore(Path(tmp))
            memory = MemoryEnvelope.text(
                "Memoria viva deve preservar episodio original e provenance.",
                scope={"project_id": "psi"},
                tags=["architecture"],
                importance=0.7,
                created_at=1.0,
            )
            store.remember(memory)
            cache = EngramCache(cache_threshold=1)
            pipeline = RecallPipeline(store, engram_cache=cache)

            first = pipeline.recall("preservar episodio provenance", scope={"project_id": "psi"})
            second = pipeline.recall("preservar episodio provenance", scope={"project_id": "psi"})

            self.assertEqual(first[0].memory.id, memory.id)
            self.assertEqual(second[0].memory.id, memory.id)
            self.assertIn("engram", second[0].why_retrieved)
            self.assertEqual(cache.stats()["cache_hits"], 1)


if __name__ == "__main__":
    unittest.main()
