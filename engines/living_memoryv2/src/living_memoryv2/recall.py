from __future__ import annotations

from dataclasses import dataclass, field
import math
import re

from .graph import GraphStore
from .engram import EngramCache
from .lifecycle import lifecycle_for
from .schema import MemoryEnvelope
from .store import TemporalStore


@dataclass
class RecallResult:
    memory: MemoryEnvelope
    score: float
    confidence: float
    why_retrieved: list[str] = field(default_factory=list)
    layer_signals: dict[str, float] = field(default_factory=dict)

    def __post_init__(self) -> None:
        if not self.layer_signals:
            for reason in self.why_retrieved:
                self.layer_signals[reason] = self.score

    def add_reason(self, reason: str) -> None:
        if reason not in self.why_retrieved:
            self.why_retrieved.append(reason)

    def add_signal(self, layer: str, score: float) -> None:
        self.layer_signals[layer] = max(self.layer_signals.get(layer, 0.0), score)


class RecallPipeline:
    def __init__(
        self,
        store: TemporalStore,
        *,
        graph_store: GraphStore | None = None,
        engram_cache: EngramCache | None = None,
        critical_threshold: float = 0.95,
    ) -> None:
        self.store = store
        self.graph_store = graph_store
        self.engram_cache = engram_cache
        self.critical_threshold = critical_threshold

    def recall(
        self,
        query: str,
        *,
        scope: dict[str, object] | None = None,
        tags: list[str] | None = None,
        top_k: int = 10,
        use_engram: bool = True,
        at_time: float | None = None,
        graph_hops: int = 1,
    ) -> list[RecallResult]:
        candidates: dict[str, RecallResult] = {}
        query_terms = self._terms(query)

        for memory in self.store.list_memories(
            scope=scope,
            tags=tags,
            min_importance=self.critical_threshold,
            limit=max(top_k, 10),
        ):
            self._merge(candidates, memory, "critical", 0.70 + memory.importance * 0.25)

        if self.engram_cache is not None and use_engram:
            for memory_id in self.engram_cache.lookup(query, scope=scope, tags=tags, top_k=top_k):
                try:
                    memory = self.store.inspect(memory_id)
                except KeyError:
                    self.engram_cache.invalidate_memory(memory_id)
                    continue
                if not self._matches(memory, scope=scope, tags=tags):
                    continue
                self._merge(candidates, memory, "engram", 0.60 + memory.importance * 0.25)

        fts_memories = self.store.search_text(query, scope=scope, tags=tags, limit=max(top_k * 4, 10))
        for memory in fts_memories:
            self._merge(candidates, memory, "fts", self._lexical_score(query, memory) + 0.35)

        mobius_seed_count = min(max(top_k, 1), 8)
        graph_node_cache: dict[str, set[str]] = self._graph_nodes_for_many(
            [memory.id for memory in fts_memories[:mobius_seed_count]],
            at_time=at_time,
        )
        for seed in fts_memories[:mobius_seed_count]:
            try:
                address = self.store.address_for(seed.id)
            except KeyError:
                continue
            seed_graph_nodes = self._graph_nodes(seed.id, graph_node_cache, at_time=at_time)
            neighbors = self.store.by_mobius_neighbors(
                address,
                depth=1,
                scope=scope,
                tags=tags,
                limit=max(top_k * 2, 10),
            )
            if seed_graph_nodes:
                self._prime_graph_nodes([memory.id for memory in neighbors], graph_node_cache, at_time=at_time)
            for memory in neighbors:
                score = 0.45 + memory.importance * 0.20
                self._merge(candidates, memory, "mobius", score)
                if memory.id != seed.id and self._mobius_graph_validates(
                    seed_graph_nodes,
                    memory.id,
                    graph_node_cache,
                    at_time=at_time,
                ):
                    self._merge(candidates, memory, "mobius_graph", 0.68 + memory.importance * 0.22)

        if self.graph_store is not None:
            for memory_id in self.graph_store.walk_related_memory_ids(
                query_terms,
                limit=max(top_k * 4, 10),
                max_hops=graph_hops,
                at_time=at_time,
            ):
                try:
                    memory = self.store.inspect(memory_id)
                except KeyError:
                    continue
                if not self._matches(memory, scope=scope, tags=tags):
                    continue
                self._merge(candidates, memory, "graph", 0.72 + memory.importance * 0.28)

        if len(candidates) < top_k:
            for memory in self.store.list_memories(scope=scope, tags=tags, limit=100):
                score = self._lexical_score(query, memory)
                if score > 0:
                    self._merge(candidates, memory, "lexical", score)

        for memory_id in list(candidates):
            item = candidates[memory_id]
            lifecycle = lifecycle_for(item.memory)
            if not lifecycle.should_recall(query_terms=query_terms, at_time=at_time):
                candidates.pop(memory_id, None)
                continue
            adjusted_score = lifecycle.adjusted_score(item.score, query_terms=query_terms, at_time=at_time)
            effective_importance = lifecycle.effective_importance(query_terms=query_terms, at_time=at_time)
            if adjusted_score != item.score or lifecycle.state != "active":
                item.score = adjusted_score
                item.confidence = min(1.0, max(0.0, (item.confidence + effective_importance) / 2))
                item.add_reason("lifecycle")
                item.add_signal("lifecycle", effective_importance)

        ordered = sorted(
            candidates.values(),
            key=lambda item: (
                "critical" in item.why_retrieved and lifecycle_for(item.memory).state == "active",
                item.score,
                item.memory.importance,
                item.memory.confidence,
                item.memory.created_at,
            ),
            reverse=True,
        )
        results = ordered[:top_k]
        if self.engram_cache is not None and use_engram:
            self.engram_cache.record_result(
                query,
                [item.memory.id for item in results],
                scope=scope,
                tags=tags,
            )
        return results

    def _merge(
        self,
        candidates: dict[str, RecallResult],
        memory: MemoryEnvelope,
        reason: str,
        score: float,
    ) -> None:
        score = min(1.0, max(0.0, score))
        if memory.id not in candidates:
            candidates[memory.id] = RecallResult(
                memory=memory,
                score=score,
                confidence=min(1.0, (score + memory.confidence) / 2),
                why_retrieved=[reason],
                layer_signals={reason: score},
            )
            return
        current = candidates[memory.id]
        current.score = max(current.score, score)
        current.confidence = max(current.confidence, min(1.0, (score + memory.confidence) / 2))
        current.add_reason(reason)
        current.add_signal(reason, score)

    def _mobius_graph_validates(
        self,
        seed_graph_nodes: set[str],
        memory_id: str,
        cache: dict[str, set[str]],
        *,
        at_time: float | None,
    ) -> bool:
        if self.graph_store is None or not seed_graph_nodes:
            return False
        neighbor_nodes = self._graph_nodes(memory_id, cache, at_time=at_time)
        return bool(seed_graph_nodes & neighbor_nodes)

    def _graph_nodes(
        self,
        memory_id: str,
        cache: dict[str, set[str]],
        *,
        at_time: float | None,
    ) -> set[str]:
        if self.graph_store is None:
            return set()
        if memory_id not in cache:
            cache[memory_id] = self.graph_store.relation_nodes_for_memory(memory_id, at_time=at_time)
        return cache[memory_id]

    def _graph_nodes_for_many(self, memory_ids: list[str], *, at_time: float | None) -> dict[str, set[str]]:
        if self.graph_store is None or not memory_ids:
            return {}
        return self.graph_store.relation_nodes_for_memory_ids(memory_ids, at_time=at_time)

    def _prime_graph_nodes(
        self,
        memory_ids: list[str],
        cache: dict[str, set[str]],
        *,
        at_time: float | None,
    ) -> None:
        if self.graph_store is None:
            return
        missing = [memory_id for memory_id in dict.fromkeys(memory_ids) if memory_id not in cache]
        if not missing:
            return
        cache.update(self.graph_store.relation_nodes_for_memory_ids(missing, at_time=at_time))

    def _lexical_score(self, query: str, memory: MemoryEnvelope) -> float:
        query_terms = set(self._terms(query))
        if not query_terms:
            return 0.0
        haystack = " ".join(
            [
                memory.text_content(),
                " ".join(memory.tags),
                " ".join(memory.entities),
                memory.memory_type,
                self._relation_text(memory),
            ]
        ).lower()
        matched = sum(1 for term in query_terms if term in haystack)
        if matched == 0:
            return 0.0
        coverage = matched / len(query_terms)
        importance_boost = memory.importance * 0.15
        return min(1.0, math.sqrt(coverage) * 0.70 + importance_boost)

    def _terms(self, query: str) -> list[str]:
        raw_terms = re.findall(r"[\w]+", query.lower(), flags=re.UNICODE)
        terms = set(raw_terms)
        if {"now", "latest", "active"} & terms:
            terms.add("current")
        if "current" in terms:
            terms.add("now")
        if "run" in terms:
            terms.update({"command", "commands"})
        if "missing" in terms:
            terms.update({"gap", "gaps"})
        if {"gap", "gaps"} & terms:
            terms.add("missing")
        return sorted(terms)

    def _matches(
        self,
        memory: MemoryEnvelope,
        *,
        scope: dict[str, object] | None = None,
        tags: list[str] | None = None,
    ) -> bool:
        if scope and any(memory.scope.get(key) != value for key, value in scope.items()):
            return False
        required_tags = set(tags or [])
        if required_tags and not required_tags.issubset(set(memory.tags)):
            return False
        return True

    def _relation_text(self, memory: MemoryEnvelope) -> str:
        parts: list[str] = []
        for relation in memory.relations:
            parts.extend(
                str(relation.get(key) or "")
                for key in ("subject", "predicate", "object")
            )
        return " ".join(part for part in parts if part)
