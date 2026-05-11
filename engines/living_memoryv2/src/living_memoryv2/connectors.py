from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any

from .schema import MemoryEnvelope


@dataclass(frozen=True)
class ConnectorRecord:
    connector_type: str
    external_id: str
    title: str = ""
    body: str = ""
    source_uri: str = ""
    updated_at: float | None = None
    metadata: dict[str, Any] = field(default_factory=dict)


class ConnectorIngestor:
    def to_memory(
        self,
        record: ConnectorRecord,
        *,
        scope: dict[str, Any],
        tags: list[str] | None = None,
        memory_type: str = "artifact",
    ) -> MemoryEnvelope:
        connector_type = str(record.connector_type).strip().lower()
        if not connector_type:
            raise ValueError("connector_type is required")
        external_id = str(record.external_id).strip()
        if not external_id:
            raise ValueError("external_id is required")
        title = str(record.title or "").strip()
        body = str(record.body or "").strip()
        content = "\n".join(part for part in [f"title: {title}" if title else "", body] if part).strip()
        if not content:
            content = f"{connector_type} record {external_id}"
        metadata = dict(record.metadata or {})
        metadata.update(
            {
                "connector_type": connector_type,
                "external_id": external_id,
                "source_uri": record.source_uri,
                "title": title,
            }
        )
        return MemoryEnvelope.text(
            content,
            scope=scope,
            memory_type=memory_type,
            tags=sorted({*(tags or []), "connector", connector_type}),
            importance=0.62,
            confidence=0.9,
            provenance={
                "source": f"connector:{connector_type}",
                "connector_type": connector_type,
                "external_id": external_id,
                "source_uri": record.source_uri,
                "updated_at": record.updated_at,
            },
            created_at=record.updated_at,
            metadata=metadata,
        )
