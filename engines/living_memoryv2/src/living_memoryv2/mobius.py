from __future__ import annotations

from dataclasses import dataclass
import hashlib
from typing import Iterable

from .schema import MemoryEnvelope, stable_json


@dataclass(frozen=True, order=True)
class MobiusAddress:
    iu: int
    iv: int
    iw: int
    region: str = "episodic"

    def to_dict(self) -> dict[str, int | str]:
        return {"iu": self.iu, "iv": self.iv, "iw": self.iw, "region": self.region}

    @classmethod
    def from_dict(cls, data: dict[str, int | str]) -> "MobiusAddress":
        return cls(
            iu=int(data["iu"]),
            iv=int(data["iv"]),
            iw=int(data["iw"]),
            region=str(data.get("region") or "episodic"),
        )


class MobiusIndex:
    def __init__(self, u_size: int = 32, v_size: int = 16, w_size: int = 8) -> None:
        if min(u_size, v_size, w_size) <= 0:
            raise ValueError("all dimensions must be positive")
        self.u_size = u_size
        self.v_size = v_size
        self.w_size = w_size

    def address_for(self, memory: MemoryEnvelope) -> MobiusAddress:
        region = self._region_for(memory)
        material = {
            "scope": memory.scope,
            "memory_type": memory.memory_type,
            "modalities": [item.modality for item in memory.modalities],
            "tags": memory.tags,
            "entities": memory.entities,
            "content_hash": memory.content_hash,
        }
        digest = hashlib.sha256(stable_json(material).encode("utf-8")).digest()
        iu = int.from_bytes(digest[0:4], "big") % self.u_size
        iv = self._region_band(region, digest[4])
        iw = int.from_bytes(digest[5:9], "big") % self.w_size
        return MobiusAddress(iu=iu, iv=iv, iw=iw, region=region)

    def neighbors(self, address: MobiusAddress, depth: int = 1) -> list[MobiusAddress]:
        if depth < 0:
            raise ValueError("depth must be non-negative")
        items: set[MobiusAddress] = set()
        for du in range(-depth, depth + 1):
            for dv in range(-depth, depth + 1):
                for dw in range(-depth, depth + 1):
                    if abs(du) + abs(dv) + abs(dw) > depth:
                        continue
                    items.add(self._wrap(address.iu + du, address.iv + dv, address.iw + dw, address.region))
        return sorted(items)

    def _wrap(self, iu: int, iv: int, iw: int, region: str) -> MobiusAddress:
        wraps, new_u = divmod(iu, self.u_size)
        new_v = iv
        if wraps % 2:
            new_v = self.v_size - 1 - new_v
        new_v = self._reflect_axis(new_v, self.v_size)
        return MobiusAddress(
            iu=new_u,
            iv=new_v,
            iw=iw % self.w_size,
            region=region,
        )

    def _reflect_axis(self, value: int, size: int) -> int:
        if size <= 1:
            return 0
        period = 2 * (size - 1)
        position = value % period
        if position >= size:
            position = period - position
        return position

    def _region_for(self, memory: MemoryEnvelope) -> str:
        if memory.memory_type in {"decision", "error", "solution", "procedural", "semantic"}:
            return memory.memory_type
        modalities = {item.modality for item in memory.modalities}
        if len(modalities) > 1 or not modalities.issubset({"text", "structured"}):
            return "multimodal"
        return "episodic"

    def _region_band(self, region: str, byte: int) -> int:
        bands = {
            "episodic": 0,
            "semantic": 2,
            "procedural": 4,
            "decision": 6,
            "error": 8,
            "solution": 10,
            "multimodal": 12,
        }
        start = min(bands.get(region, 0), self.v_size - 1)
        width = max(1, self.v_size // 8)
        return min(self.v_size - 1, start + (byte % width))

    def serialize_many(self, addresses: Iterable[MobiusAddress]) -> list[dict[str, int | str]]:
        return [item.to_dict() for item in addresses]
