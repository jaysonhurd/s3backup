#!/usr/bin/env python3
"""Check skill metadata, local Markdown links, and shell syntax without downloads."""

import re
import subprocess
import sys
from pathlib import Path
from urllib.parse import unquote, urlsplit


def prose(text):
    """Exclude fenced examples when looking for links and headings."""
    lines = []
    fence = None
    for line in text.splitlines():
        marker = re.match(r"^\s*(`{3,}|~{3,})", line)
        if marker:
            token = marker.group(1)
            if fence is None:
                fence = token
            elif token[0] == fence[0] and len(token) >= len(fence):
                fence = None
            continue
        if fence is None:
            lines.append(line)
    return "\n".join(lines)


def anchors(text):
    """Collect explicit HTML anchors and GitHub-style heading slugs."""
    text = prose(text)
    result = set(re.findall(r'\b(?:id|name)=[\'"]([^\'"]+)[\'"]', text))
    duplicates = {}
    for heading in re.findall(r"^#{1,6}\s+(.+?)\s*#*\s*$", text, re.M):
        heading = re.sub(r"\[([^]]+)\]\([^)]+\)", r"\1", heading)
        slug = re.sub(r"[^\w\-\s]", "", heading.lower()).replace(" ", "-")
        count = duplicates.get(slug, 0)
        duplicates[slug] = count + 1
        result.add(f"{slug}-{count}" if count else slug)
    return result


def check(root):
    """Return diagnostics for tracked and new, nonignored repository files."""
    tracked = subprocess.check_output(
        ["git", "ls-files", "--cached", "--others", "--exclude-standard", "-z"],
        cwd=root,
    ).decode().split("\0")
    paths = [root / name for name in sorted(set(tracked)) if name]
    errors = []
    for path in paths:
        relative = path.relative_to(root)
        if path.name == "SKILL.md":
            text = path.read_text()
            metadata = re.match(r"\A---\n(.*?)\n---(?:\n|$)", text, re.S)
            if not metadata:
                errors.append(f"{relative}: missing YAML frontmatter")
            else:
                fields = dict(re.findall(r"^(name|description):\s*(.+)$", metadata[1], re.M))
                name = fields.get("name", "").strip()
                description = fields.get("description", "").strip()
                if not re.fullmatch(r"[a-z0-9]+(?:-[a-z0-9]+)*", name) or len(name) > 64:
                    errors.append(f"{relative}: invalid skill name")
                if name != path.parent.name:
                    errors.append(f"{relative}: skill name must match its directory")
                if not description or description in ('""', "''") or "TODO" in description:
                    errors.append(f"{relative}: missing skill description")
        if path.suffix == ".md":
            for target in re.findall(r"!?\[[^]\n]*\]\(([^)\n]+)\)", prose(path.read_text())):
                target = target.strip().removeprefix("<").removesuffix(">")
                link = urlsplit(target)
                if link.scheme or link.netloc:
                    continue
                destination = (path.parent / unquote(link.path)).resolve() if link.path else path
                if not destination.exists():
                    errors.append(f"{relative}: missing local link target {target}")
                elif link.fragment and destination.suffix == ".md":
                    if unquote(link.fragment) not in anchors(destination.read_text()):
                        errors.append(f"{relative}: missing Markdown anchor {target}")
        if path.suffix == ".sh":
            result = subprocess.run(["bash", "-n", str(path)], capture_output=True, text=True)
            if result.returncode:
                errors.append(f"{relative}: {result.stderr.strip()}")
    return errors


if __name__ == "__main__":
    diagnostics = check(Path(__file__).resolve().parents[1])
    if diagnostics:
        print("\n".join(diagnostics), file=sys.stderr)
        sys.exit(1)
    print("Skill metadata, local Markdown links, and shell syntax passed.")
