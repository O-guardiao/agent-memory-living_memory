from __future__ import annotations

from dataclasses import dataclass, field
import hashlib
import json
import time
from typing import Any

TEXT_BRIDGE_METADATA_KEYS = (
    "text",
    "derived_text",
    "caption",
    "alt_text",
    "ocr_text",
    "detected_text",
    "transcript",
    "description",
    "title",
    "labels",
)


def stable_json(value: Any) -> str:
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":"))


def content_bytes(value: Any) -> bytes:
    if value is None:
        return b""
    if isinstance(value, bytes):
        return value
    if isinstance(value, str):
        return value.encode("utf-8")
    return stable_json(value).encode("utf-8")


def sha256_hex(value: Any) -> str:
    return hashlib.sha256(content_bytes(value)).hexdigest()


@dataclass
class ModalityRef:
    modality: str
    content: Any | None = None
    uri: str | None = None
    mime_type: str | None = None
    sha256: str | None = None
    size_bytes: int | None = None
    metadata: dict[str, Any] = field(default_factory=dict)

    def __post_init__(self) -> None:
        self.modality = str(self.modality or "").strip().lower()
        if not self.modality:
            raise ValueError("modality is required")
        self.metadata = dict(self.metadata or {})
        if self.content is not None:
            data = content_bytes(self.content)
            if self.sha256 is None:
                self.sha256 = hashlib.sha256(data).hexdigest()
            if self.size_bytes is None:
                self.size_bytes = len(data)

    def to_dict(self) -> dict[str, Any]:
        return {
            "modality": self.modality,
            "content": self.content,
            "uri": self.uri,
            "mime_type": self.mime_type,
            "sha256": self.sha256,
            "size_bytes": self.size_bytes,
            "metadata": self.metadata,
        }

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "ModalityRef":
        return cls(
            modality=data["modality"],
            content=data.get("content"),
            uri=data.get("uri"),
            mime_type=data.get("mime_type"),
            sha256=data.get("sha256"),
            size_bytes=data.get("size_bytes"),
            metadata=data.get("metadata") or {},
        )

    def searchable_text(self) -> str:
        parts: list[str] = []
        if self.content is not None:
            if isinstance(self.content, str):
                parts.append(self.content)
            elif isinstance(self.content, bytes):
                pass
            else:
                parts.append(stable_json(self.content))
        if self.uri:
            parts.append(self.uri)
        if self.modality not in {"text", "structured"}:
            parts.append(self.modality)
        if self.mime_type and self.modality not in {"text", "structured"}:
            parts.append(self.mime_type)
        for key in TEXT_BRIDGE_METADATA_KEYS:
            if key not in self.metadata:
                continue
            value = self.metadata.get(key)
            if value is None or value == "":
                continue
            if isinstance(value, (dict, list, tuple)):
                parts.append(stable_json(value))
            else:
                parts.append(str(value))
        return "\n".join(part for part in parts if part)


@dataclass
class MemoryEnvelope:
    scope: dict[str, Any]
    memory_type: str
    modalities: list[ModalityRef]
    id: str = ""
    created_at: float = field(default_factory=lambda: time.time())
    updated_at: float | None = None
    tags: list[str] = field(default_factory=list)
    entities: list[str] = field(default_factory=list)
    relations: list[dict[str, Any]] = field(default_factory=list)
    importance: float = 0.5
    confidence: float = 1.0
    sensitivity: str = "normal"
    provenance: dict[str, Any] = field(default_factory=dict)
    metadata: dict[str, Any] = field(default_factory=dict)
    content_hash: str = ""

    def __post_init__(self) -> None:
        self.scope = dict(self.scope or {})
        self.memory_type = self.memory_type or "episodic"
        self.modalities = [
            item if isinstance(item, ModalityRef) else ModalityRef.from_dict(item)
            for item in self.modalities
        ]
        self.tags = sorted({str(item) for item in self.tags})
        self.entities = sorted({str(item) for item in self.entities})
        self.relations = list(self.relations or [])
        self.provenance = dict(self.provenance or {})
        self.metadata = dict(self.metadata or {})
        self.updated_at = self.created_at if self.updated_at is None else self.updated_at
        self.importance = self._clamp_score(self.importance, "importance")
        self.confidence = self._clamp_score(self.confidence, "confidence")
        if not self.content_hash:
            self.content_hash = self.compute_content_hash()
        if not self.id:
            id_material = {
                "content_hash": self.content_hash,
                "created_at": self.created_at,
                "scope": self.scope,
            }
            self.id = "mem_" + sha256_hex(id_material)[:24]

    @staticmethod
    def _clamp_score(value: float, name: str) -> float:
        try:
            score = float(value)
        except (TypeError, ValueError) as exc:
            raise ValueError(f"{name} must be numeric") from exc
        if score < 0.0 or score > 1.0:
            raise ValueError(f"{name} must be between 0 and 1")
        return score

    @classmethod
    def text(
        cls,
        content: str,
        *,
        scope: dict[str, Any],
        memory_type: str = "episodic",
        tags: list[str] | None = None,
        entities: list[str] | None = None,
        relations: list[dict[str, Any]] | None = None,
        importance: float = 0.5,
        confidence: float = 1.0,
        sensitivity: str = "normal",
        provenance: dict[str, Any] | None = None,
        created_at: float | None = None,
        metadata: dict[str, Any] | None = None,
    ) -> "MemoryEnvelope":
        return cls(
            scope=scope,
            memory_type=memory_type,
            modalities=[ModalityRef("text", content=content, mime_type="text/plain")],
            tags=tags or [],
            entities=entities or [],
            relations=relations or [],
            importance=importance,
            confidence=confidence,
            sensitivity=sensitivity,
            provenance=provenance or {},
            created_at=time.time() if created_at is None else created_at,
            metadata=metadata or {},
        )

    @classmethod
    def file_ref(
        cls,
        *,
        uri: str,
        mime_type: str,
        scope: dict[str, Any],
        modality: str = "file",
        memory_type: str = "artifact",
        tags: list[str] | None = None,
        entities: list[str] | None = None,
        relations: list[dict[str, Any]] | None = None,
        importance: float = 0.5,
        confidence: float = 1.0,
        sensitivity: str = "normal",
        provenance: dict[str, Any] | None = None,
        created_at: float | None = None,
        metadata: dict[str, Any] | None = None,
    ) -> "MemoryEnvelope":
        return cls(
            scope=scope,
            memory_type=memory_type,
            modalities=[ModalityRef(modality, uri=uri, mime_type=mime_type, metadata=metadata or {})],
            tags=tags or [],
            entities=entities or [],
            relations=relations or [],
            importance=importance,
            confidence=confidence,
            sensitivity=sensitivity,
            provenance=provenance or {},
            created_at=time.time() if created_at is None else created_at,
            metadata=metadata or {},
        )

    @classmethod
    def structured(
        cls,
        content: dict[str, Any],
        *,
        scope: dict[str, Any],
        memory_type: str = "semantic",
        tags: list[str] | None = None,
        entities: list[str] | None = None,
        relations: list[dict[str, Any]] | None = None,
        importance: float = 0.5,
        confidence: float = 1.0,
        sensitivity: str = "normal",
        provenance: dict[str, Any] | None = None,
        created_at: float | None = None,
        metadata: dict[str, Any] | None = None,
    ) -> "MemoryEnvelope":
        return cls(
            scope=scope,
            memory_type=memory_type,
            modalities=[ModalityRef("structured", content=content, mime_type="application/json")],
            tags=tags or [],
            entities=entities or [],
            relations=relations or [],
            importance=importance,
            confidence=confidence,
            sensitivity=sensitivity,
            provenance=provenance or {},
            created_at=time.time() if created_at is None else created_at,
            metadata=metadata or {},
        )

    @classmethod
    def multimodal(
        cls,
        modalities: list[ModalityRef | dict[str, Any]],
        *,
        scope: dict[str, Any],
        memory_type: str = "artifact",
        tags: list[str] | None = None,
        entities: list[str] | None = None,
        relations: list[dict[str, Any]] | None = None,
        importance: float = 0.5,
        confidence: float = 1.0,
        sensitivity: str = "normal",
        provenance: dict[str, Any] | None = None,
        created_at: float | None = None,
        metadata: dict[str, Any] | None = None,
    ) -> "MemoryEnvelope":
        if not modalities:
            raise ValueError("at least one modality is required")
        return cls(
            scope=scope,
            memory_type=memory_type,
            modalities=[
                item if isinstance(item, ModalityRef) else ModalityRef.from_dict(item)
                for item in modalities
            ],
            tags=tags or [],
            entities=entities or [],
            relations=relations or [],
            importance=importance,
            confidence=confidence,
            sensitivity=sensitivity,
            provenance=provenance or {},
            created_at=time.time() if created_at is None else created_at,
            metadata=metadata or {},
        )

    def compute_content_hash(self) -> str:
        material = {
            "scope": self.scope,
            "memory_type": self.memory_type,
            "modalities": [item.to_dict() for item in self.modalities],
            "tags": self.tags,
            "entities": self.entities,
            "relations": self.relations,
            "importance": self.importance,
            "confidence": self.confidence,
            "sensitivity": self.sensitivity,
            "provenance": self.provenance,
            "metadata": self.metadata,
        }
        return sha256_hex(material)

    def text_content(self) -> str:
        parts: list[str] = []
        for item in self.modalities:
            text = item.searchable_text()
            if text:
                parts.append(text)
        return "\n".join(part for part in parts if part)

    def modality_names(self) -> list[str]:
        return sorted({item.modality for item in self.modalities})

    def to_dict(self) -> dict[str, Any]:
        return {
            "id": self.id,
            "created_at": self.created_at,
            "updated_at": self.updated_at,
            "scope": self.scope,
            "memory_type": self.memory_type,
            "modalities": [item.to_dict() for item in self.modalities],
            "tags": self.tags,
            "entities": self.entities,
            "relations": self.relations,
            "importance": self.importance,
            "confidence": self.confidence,
            "sensitivity": self.sensitivity,
            "provenance": self.provenance,
            "metadata": self.metadata,
            "content_hash": self.content_hash,
        }

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "MemoryEnvelope":
        return cls(
            id=data.get("id", ""),
            created_at=data.get("created_at") or time.time(),
            updated_at=data.get("updated_at"),
            scope=data.get("scope") or {},
            memory_type=data.get("memory_type") or "episodic",
            modalities=[ModalityRef.from_dict(item) for item in data.get("modalities", [])],
            tags=data.get("tags") or [],
            entities=data.get("entities") or [],
            relations=data.get("relations") or [],
            importance=data.get("importance", 0.5),
            confidence=data.get("confidence", 1.0),
            sensitivity=data.get("sensitivity", "normal"),
            provenance=data.get("provenance") or {},
            metadata=data.get("metadata") or {},
            content_hash=data.get("content_hash", ""),
        )
