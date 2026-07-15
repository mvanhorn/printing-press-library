#!/usr/bin/env python3
"""Fail when a library SKILL.md mentions agent-config filename literals."""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path


AGENT_CONFIG_LITERALS = ("AGENTS.md", "CLAUDE.md", ".cursorrules", ".clinerules")
AGENT_CONFIG_RE = re.compile(
    "|".join(re.escape(literal) for literal in AGENT_CONFIG_LITERALS),
    re.IGNORECASE,
)


def findings(skill_path: Path) -> list[tuple[int, str]]:
    matches: list[tuple[int, str]] = []
    for line_no, line in enumerate(skill_path.read_text(encoding="utf-8").splitlines(), 1):
        match = AGENT_CONFIG_RE.search(line)
        if match:
            matches.append((line_no, match.group(0)))
    return matches


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("skill_path", type=Path)
    args = parser.parse_args()

    failed = False
    for line_no, literal in findings(args.skill_path):
        print(
            f"::error file={args.skill_path},line={line_no},title=Disallowed SKILL.md literal::"
            f"{literal} is an agent-config filename literal and can trip downstream skill-install guards"
        )
        failed = True
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
