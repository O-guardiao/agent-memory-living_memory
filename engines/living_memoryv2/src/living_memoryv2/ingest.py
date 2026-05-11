from __future__ import annotations

from dataclasses import dataclass
import re


@dataclass(frozen=True)
class MarkdownSection:
    source_path: str
    heading_path: list[str]
    content: str
    section_index: int
    chunk_index: int = 0


def split_markdown_sections(
    text: str,
    *,
    source_path: str,
    max_chars: int = 2400,
) -> list[MarkdownSection]:
    """Split Markdown into heading-aware, bounded sections."""

    if max_chars <= 80:
        raise ValueError("max_chars must be greater than 80")
    lines = text.splitlines()
    stack: list[tuple[int, str]] = []
    sections: list[tuple[list[str], list[str]]] = []
    current_path: list[str] = []
    current_lines: list[str] = []

    def flush() -> None:
        if not current_lines:
            return
        content = "\n".join(current_lines).strip()
        if content:
            sections.append((list(current_path), current_lines.copy()))

    for line in lines:
        match = re.match(r"^(#{1,6})\s+(.+?)\s*$", line)
        if match:
            flush()
            level = len(match.group(1))
            title = _clean_heading(match.group(2))
            stack[:] = [(item_level, item_title) for item_level, item_title in stack if item_level < level]
            stack.append((level, title))
            current_path = [item_title for _item_level, item_title in stack]
            current_lines = [line]
            continue
        current_lines.append(line)
    flush()

    if not sections and text.strip():
        sections.append(([source_path], [text.strip()]))

    output: list[MarkdownSection] = []
    for index, (path, section_lines) in enumerate(sections):
        content = "\n".join(section_lines).strip()
        for chunk_index, chunk in enumerate(_chunk_text(content, max_chars=max_chars)):
            output.append(
                MarkdownSection(
                    source_path=source_path,
                    heading_path=path or [source_path],
                    content=chunk,
                    section_index=index,
                    chunk_index=chunk_index,
                )
            )
    return output


def _chunk_text(text: str, *, max_chars: int) -> list[str]:
    if len(text) <= max_chars:
        return [text]
    paragraphs = re.split(r"\n\s*\n", text)
    chunks: list[str] = []
    current = ""
    for paragraph in paragraphs:
        paragraph = paragraph.strip()
        if not paragraph:
            continue
        if len(paragraph) > max_chars:
            if current:
                chunks.append(current)
                current = ""
            chunks.extend(_split_long_paragraph(paragraph, max_chars=max_chars))
            continue
        candidate = paragraph if not current else current + "\n\n" + paragraph
        if len(candidate) <= max_chars:
            current = candidate
        else:
            chunks.append(current)
            current = paragraph
    if current:
        chunks.append(current)
    return chunks


def _split_long_paragraph(text: str, *, max_chars: int) -> list[str]:
    chunks: list[str] = []
    start = 0
    while start < len(text):
        end = min(len(text), start + max_chars)
        if end < len(text):
            boundary = text.rfind(" ", start, end)
            if boundary > start + max_chars // 2:
                end = boundary
        chunks.append(text[start:end].strip())
        start = end
    return [chunk for chunk in chunks if chunk]


def _clean_heading(text: str) -> str:
    return re.sub(r"\s+", " ", text.strip().strip("#")).strip()
