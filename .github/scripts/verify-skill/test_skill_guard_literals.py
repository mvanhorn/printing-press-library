#!/usr/bin/env python3
"""Tests for the library-local SKILL.md agent-config literal guard."""

from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

import check_skill_guard_literals


class SkillGuardLiteralsTest(unittest.TestCase):
    def _write_skill(self, text: str) -> Path:
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        path = Path(tmp.name) / "SKILL.md"
        path.write_text(text, encoding="utf-8")
        return path

    def test_clean_skill_passes(self):
        skill = self._write_skill("# Thing\n\nNormal prose, no config files.\n")

        self.assertEqual([], check_skill_guard_literals.findings(skill))

    def test_each_literal_is_caught_case_insensitively(self):
        for literal in check_skill_guard_literals.AGENT_CONFIG_LITERALS:
            with self.subTest(literal=literal):
                skill = self._write_skill(f"# Thing\n\nmentions {literal.lower()} somewhere\n")

                self.assertEqual([(3, literal.lower())], check_skill_guard_literals.findings(skill))

    def test_multiple_literals_report_each_occurrence(self):
        skill = self._write_skill("# Thing\n\nSee AGENTS.md\n\nand CLAUDE.md too\n")

        self.assertEqual(
            [(3, "AGENTS.md"), (5, "CLAUDE.md")],
            check_skill_guard_literals.findings(skill),
        )


if __name__ == "__main__":
    unittest.main()
