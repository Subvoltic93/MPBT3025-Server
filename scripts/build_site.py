#!/usr/bin/env python3
"""Render project Markdown files as static pages for GitHub Pages."""

from __future__ import annotations

import os
import re
from pathlib import Path
from urllib.parse import unquote, urlsplit, urlunsplit

from markdown_it import MarkdownIt


ROOT = Path(__file__).resolve().parents[1]
NAV_ITEMS = [
    ("Status", "index.html", "status"),
    ("README", "readme.html", "readme"),
    ("Protocol", "protocol.html", "protocol"),
    ("Research", "research.html", "research"),
    ("Roadmap", "roadmap.html", "roadmap"),
    ("Project files", "project-files.html", "files"),
]
LINK_URL = re.compile(r"(?P<lead>\]\()(?P<url>[^\s)]+)")
H1 = re.compile(r"^#\s+(.+?)\s*#*\s*$", re.MULTILINE)


def output_for(source: Path) -> Path:
    relative = source.relative_to(ROOT)
    key = relative.as_posix().casefold()
    root_pages = {
        "readme.md": Path("readme.html"),
        "research.md": Path("research.html"),
        "roadmap.md": Path("roadmap.html"),
        "protocol.md": Path("protocol.html"),
    }
    if key in root_pages:
        return ROOT / root_pages[key]
    if source.name.casefold() == "readme.md":
        return source.parent / "index.html"
    return source.with_suffix(".html")


def nav_href(page: Path, target: Path) -> str:
    return os.path.relpath(target, page.parent).replace(os.sep, "/")


def page_shell(title: str, body: str, page: Path, current: str) -> str:
    nav = "\n".join(
        f'<a href="{nav_href(page, ROOT / target)}"'
        + (' class="current" aria-current="page"' if current == key else "")
        + f">{label}</a>"
        for label, target, key in NAV_ITEMS
    )
    safe_title = title.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")
    stylesheet = nav_href(page, ROOT / "assets/site.css")
    return f'''<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{safe_title} — MPBT 3025 Server</title>
  <link rel="stylesheet" href="{stylesheet}">
</head>
<body>
<div class="site-shell">
  <header class="masthead">
    <div class="eyebrow">ComStar archives &nbsp; // &nbsp; MPBT 3025 server emulator</div>
    <h1>{safe_title}</h1>
    <div class="report-date">Project documentation</div>
  </header>
  <nav class="site-nav" aria-label="Project pages">{nav}</nav>
  <main><article class="document">{body}</article></main>
  <footer class="site-footer">
    <p><a href="{nav_href(page, ROOT / 'project-files.html')}">Project files and downloads</a></p>
    <p>Not an official EA or BattleTech site · Local project documentation</p>
  </footer>
</div>
</body>
</html>
'''


def rewritten_links(
    text: str,
    source: Path,
    output: Path,
    source_outputs: dict[str, Path],
    local_artifact_refs: list[bool],
) -> str:
    def replace(match: re.Match[str]) -> str:
        raw_url = match.group("url")
        parsed = urlsplit(raw_url)
        if parsed.scheme or parsed.netloc or not parsed.path:
            return match.group(0)
        candidate = (source.parent / unquote(parsed.path)).resolve()
        try:
            relative = candidate.relative_to(ROOT)
        except ValueError:
            relative = None

        target = None
        if relative is not None and relative.suffix.casefold() == ".md":
            target = source_outputs.get(relative.as_posix().casefold())
        elif relative is not None:
            if relative.parts and relative.parts[0] != "runtime" and candidate.is_dir():
                index = candidate / "index.html"
                if index.is_file():
                    target = index
            elif relative.parts and relative.parts[0] != "runtime" and candidate.is_file():
                return match.group(0)

        if target is None:
            local_artifact_refs[0] = True
            return match.group("lead") + "#local-artifacts"
        new_path = os.path.relpath(target, output.parent).replace(os.sep, "/")
        replacement_url = urlunsplit(("", "", new_path, parsed.query, parsed.fragment))
        return match.group("lead") + replacement_url

    return LINK_URL.sub(replace, text)


def main() -> None:
    sources = sorted(path for path in ROOT.rglob("*.md") if ".git" not in path.parts)
    outputs = {source.relative_to(ROOT).as_posix().casefold(): output_for(source) for source in sources}
    markdown = MarkdownIt("default", {"html": True, "typographer": False})
    for source in sources:
        output = outputs[source.relative_to(ROOT).as_posix().casefold()]
        source_text = source.read_text(encoding="utf-8")
        title_match = H1.search(source_text)
        title = title_match.group(1).strip() if title_match else source.stem.replace("-", " ").title()
        local_artifact_refs = [False]
        converted = rewritten_links(source_text, source, output, outputs, local_artifact_refs)
        body = markdown.render(converted)
        if local_artifact_refs[0]:
            body += (
                '<aside class="file-note" id="local-artifacts">Some links in this historical note point to '
                'local client captures or runtime logs that are not included in the published project.</aside>'
            )
        if output == ROOT / "readme.html":
            current = "readme"
        elif output == ROOT / "protocol.html":
            current = "protocol"
        elif output == ROOT / "research.html":
            current = "research"
        elif output == ROOT / "roadmap.html":
            current = "roadmap"
        else:
            current = "files"
        output.parent.mkdir(parents=True, exist_ok=True)
        output.write_text(page_shell(title, body, output, current), encoding="utf-8")
        print(f"{source.relative_to(ROOT)} -> {output.relative_to(ROOT)}")


if __name__ == "__main__":
    main()
