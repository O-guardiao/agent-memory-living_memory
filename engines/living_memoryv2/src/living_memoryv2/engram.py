from __future__ import annotations

from collections import Counter
from dataclasses import dataclass, field
import hashlib
import time
import unicodedata
from typing import Any

from .schema import stable_json


PRIMES = [31, 37, 41, 43, 47, 53, 59, 61, 67, 71, 73, 79]


class NGramHasher:
    """Deterministic suffix n-gram hash for hot recall queries."""

    def __init__(
        self,
        *,
        table_size: int = 100_003,
        n_heads: int = 4,
        ngram_orders: list[int] | None = None,
    ) -> None:
        if table_size <= 0:
            raise ValueError("table_size must be positive")
        if n_heads <= 0 or n_heads > len(PRIMES):
            raise ValueError(f"n_heads must be between 1 and {len(PRIMES)}")
        self.table_size = table_size
        self.n_heads = n_heads
        self.ngram_orders = ngram_orders or [2, 3]

    def normalize(self, text: str) -> str:
        return unicodedata.normalize("NFKC", text or "").strip().lower()

    def tokens(self, text: str) -> list[str]:
        return [token for token in self.normalize(text).split() if token]

    def primary_index(self, text: str) -> int:
        tokens = self.tokens(text)
        if not tokens:
            return 0
        order = min(max(self.ngram_orders), len(tokens))
        return self._hash_ngram(tokens[-order:], head=0)

    def all_indices(self, text: str) -> list[int]:
        tokens = self.tokens(text)
        if not tokens:
            return [0] * self.n_heads
        order = min(max(self.ngram_orders), len(tokens))
        ngram = tokens[-order:]
        return [self._hash_ngram(ngram, head=head) for head in range(self.n_heads)]

    def _hash_ngram(self, tokens: list[str], *, head: int) -> int:
        prime = PRIMES[head]
        value = 0
        for index, token in enumerate(tokens, start=1):
            digest = hashlib.sha256(token.encode("utf-8")).digest()
            token_hash = int.from_bytes(digest[:8], "little")
            value ^= (token_hash * prime * index) % self.table_size
        return value % self.table_size


@dataclass
class EngramEntry:
    query_text: str
    signature: str
    memory_ids: list[str]
    scope: dict[str, Any] = field(default_factory=dict)
    tags: list[str] = field(default_factory=list)
    hit_count: int = 0
    created_at: float = field(default_factory=time.time)
    last_access: float = field(default_factory=time.time)

    def touch(self) -> None:
        self.hit_count += 1
        self.last_access = time.time()


class EngramCache:
    """Hot-query recall cache inspired by Engram, without model training."""

    def __init__(
        self,
        *,
        table_size: int = 100_003,
        n_heads: int = 4,
        cache_threshold: int = 3,
        max_memory_ids: int = 20,
    ) -> None:
        if cache_threshold <= 0:
            raise ValueError("cache_threshold must be positive")
        if max_memory_ids <= 0:
            raise ValueError("max_memory_ids must be positive")
        self.hasher = NGramHasher(table_size=table_size, n_heads=n_heads)
        self.cache_threshold = cache_threshold
        self.max_memory_ids = max_memory_ids
        self.entries: dict[int, list[EngramEntry]] = {}
        self.query_counts: Counter[str] = Counter()
        self.total_lookups = 0
        self.cache_hits = 0
        self.cache_misses = 0
        self.collision_rejections = 0
        self.auto_cached = 0

    def lookup(
        self,
        query: str,
        *,
        scope: dict[str, Any] | None = None,
        tags: list[str] | None = None,
        top_k: int | None = None,
    ) -> list[str]:
        self.total_lookups += 1
        signature = self.signature(query, scope=scope, tags=tags)
        bucket = self.entries.get(self.hasher.primary_index(query), [])
        for entry in bucket:
            if entry.signature == signature:
                entry.touch()
                self.cache_hits += 1
                limit = top_k if top_k is not None else len(entry.memory_ids)
                return entry.memory_ids[:limit]
        if bucket:
            self.collision_rejections += 1
        self.cache_misses += 1
        return []

    def record_result(
        self,
        query: str,
        memory_ids: list[str],
        *,
        scope: dict[str, Any] | None = None,
        tags: list[str] | None = None,
    ) -> bool:
        if not memory_ids:
            return False
        signature = self.signature(query, scope=scope, tags=tags)
        self.query_counts[signature] += 1
        if self.query_counts[signature] < self.cache_threshold:
            return False
        created = self.cache(query, memory_ids, scope=scope, tags=tags)
        if created:
            self.auto_cached += 1
        return created

    def cache(
        self,
        query: str,
        memory_ids: list[str],
        *,
        scope: dict[str, Any] | None = None,
        tags: list[str] | None = None,
    ) -> bool:
        signature = self.signature(query, scope=scope, tags=tags)
        deduped = list(dict.fromkeys(memory_ids))[: self.max_memory_ids]
        if not deduped:
            return False
        entry = EngramEntry(
            query_text=query,
            signature=signature,
            memory_ids=deduped,
            scope=dict(scope or {}),
            tags=sorted(tags or []),
        )
        bucket = self.entries.setdefault(self.hasher.primary_index(query), [])
        for index, existing in enumerate(bucket):
            if existing.signature == signature:
                bucket[index] = entry
                return False
        bucket.append(entry)
        return True

    def invalidate_memory(self, memory_id: str) -> int:
        removed = 0
        for index in list(self.entries):
            bucket = self.entries[index]
            kept: list[EngramEntry] = []
            for entry in bucket:
                if memory_id in entry.memory_ids:
                    entry.memory_ids = [item for item in entry.memory_ids if item != memory_id]
                    removed += 1
                if entry.memory_ids:
                    kept.append(entry)
            if kept:
                self.entries[index] = kept
            else:
                del self.entries[index]
        return removed

    def clear(self) -> None:
        self.entries.clear()
        self.query_counts.clear()
        self.total_lookups = 0
        self.cache_hits = 0
        self.cache_misses = 0
        self.collision_rejections = 0
        self.auto_cached = 0

    def stats(self) -> dict[str, Any]:
        entry_count = sum(len(bucket) for bucket in self.entries.values())
        return {
            "entries": entry_count,
            "buckets": len(self.entries),
            "total_lookups": self.total_lookups,
            "cache_hits": self.cache_hits,
            "cache_misses": self.cache_misses,
            "hit_rate": self.cache_hits / max(1, self.total_lookups),
            "collision_rejections": self.collision_rejections,
            "auto_cached": self.auto_cached,
            "frequent_queries": sum(1 for count in self.query_counts.values() if count >= self.cache_threshold),
        }

    def signature(
        self,
        query: str,
        *,
        scope: dict[str, Any] | None = None,
        tags: list[str] | None = None,
    ) -> str:
        material = {
            "query": self.hasher.normalize(query),
            "scope": scope or {},
            "tags": sorted(tags or []),
        }
        return hashlib.sha256(stable_json(material).encode("utf-8")).hexdigest()
