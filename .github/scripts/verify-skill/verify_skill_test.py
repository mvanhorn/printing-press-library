"""Tests for verify_skill.py's skill-guard-literals check.

Run from this directory:

    python3 -m unittest verify_skill_test

The check guards against agent-config filename literals (AGENTS.md, CLAUDE.md,
.cursorrules, .clinerules) in a library SKILL.md. Hermes' skills guard flags
those as CRITICAL persistence findings, which hard-block install of the
generated cli-skills/pp-*/SKILL.md mirror. See
docs/plans/2026-06-01-001-fix-hermes-skills-guard-false-positive-plan.md.
"""

import tempfile
import unittest
from pathlib import Path

import verify_skill


def _check(skill_text: str) -> verify_skill.Report:
    with tempfile.TemporaryDirectory() as d:
        cli_dir = Path(d)
        skill = cli_dir / "SKILL.md"
        skill.write_text(skill_text, encoding="utf-8")
        report = verify_skill.Report(cli_dir=str(cli_dir), skill_path=str(skill))
        verify_skill.check_skill_guard_literals(cli_dir, skill, "thing-pp-cli", report)
        return report


class SkillGuardLiteralsTest(unittest.TestCase):
    def test_clean_skill_passes(self):
        report = _check("---\nname: pp-thing\n---\n\n# Thing\n\nNormal prose, no config files.\n")
        self.assertFalse(report.has_real_failures())
        self.assertEqual(report.findings, [])

    def test_agents_md_fails(self):
        report = _check("# Thing\n\nSee AGENTS.md for the contract.\n")
        self.assertTrue(report.has_real_failures())
        self.assertEqual(report.findings[0].check, "skill-guard-literals")
        self.assertIn("AGENTS.md", report.findings[0].detail)
        self.assertIn("SKILL.md:3", report.findings[0].evidence)

    def test_each_literal_is_caught(self):
        for literal in verify_skill.AGENT_CONFIG_LITERALS:
            with self.subTest(literal=literal):
                report = _check(f"# Thing\n\nmentions {literal} somewhere\n")
                self.assertTrue(
                    report.has_real_failures(),
                    f"{literal} should be flagged",
                )

    def test_case_insensitive_match(self):
        # Hermes' scanner matches case-insensitively; so must the guard.
        report = _check("# Thing\n\nsee agents.md in the root\n")
        self.assertTrue(report.has_real_failures())

    def test_multiple_literals_report_each_occurrence(self):
        report = _check("# Thing\n\nSee AGENTS.md\n\nand CLAUDE.md too\n")
        details = " ".join(f.detail for f in report.findings)
        self.assertIn("AGENTS.md", details)
        self.assertIn("CLAUDE.md", details)
        self.assertEqual(len(report.findings), 2)


if __name__ == "__main__":
    unittest.main()
