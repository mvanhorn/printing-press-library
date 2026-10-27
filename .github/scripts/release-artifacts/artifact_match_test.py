import os
import subprocess
import tempfile
import unittest
from pathlib import Path

import artifact_match as am


GITHUB = ("github", "github-contents", "github-intel")


class ArtifactSelectTest(unittest.TestCase):
    def test_old_hyphen_pattern_pulls_in_longer_slugs(self):
        names = [
            "cli-github-linux-amd64",
            "cli-github-contents-linux-amd64",
            "cli-github-intel-darwin-arm64",
            "mcpb-github-darwin-arm64",
            "mcpb-github-contents-darwin-arm64",
        ]
        cli = [name for name in names if am.pattern_matches("cli-github-*", name)]
        self.assertEqual(
            cli,
            [
                "cli-github-linux-amd64",
                "cli-github-contents-linux-amd64",
                "cli-github-intel-darwin-arm64",
            ],
        )

    def test_github_selects_only_its_own_artifacts(self):
        for kind, platforms in am.PLATFORMS.items():
            available = [
                am.artifact_name(kind, slug, platform)
                for slug in GITHUB
                for platform in platforms
            ]
            for slug in GITHUB:
                got = am.select(kind, slug, available)
                expected = sorted(am.artifact_name(kind, slug, platform) for platform in platforms)
                self.assertEqual(got, expected)
                for other in GITHUB:
                    if other == slug:
                        continue
                    self.assertFalse(any(name.startswith(f"{kind}__{other}__") for name in got))

    def test_report_shows_github_family_is_disjoint(self):
        report = am.selection_report(list(GITHUB))
        self.assertIn("slug github\n", report)
        self.assertIn("cli cli__github__* -> 6 selected, 0 foreign\n", report)
        self.assertIn("mcpb mcpb__github__* -> 3 selected, 0 foreign\n", report)
        self.assertIn("cli cli__github-contents__* -> 6 selected, 0 foreign\n", report)
        self.assertIn("cli cli__github-intel__* -> 6 selected, 0 foreign\n", report)
        github_section = report.split("slug github-contents", 1)[0]
        self.assertNotIn("github-contents", github_section)
        self.assertNotIn("github-intel", github_section)

    def test_every_library_prefix_pair_is_disjoint(self):
        slugs = am.library_slugs()
        pairs = am.prefix_pairs(slugs)
        self.assertIn(("github", "github-contents"), pairs)
        self.assertIn(("github", "github-intel"), pairs)
        self.assertIn(("costco", "costco-sameday"), pairs)
        self.assertIn(("openrouter", "openrouter-image"), pairs)
        self.assertIn(("substack", "substack-reader"), pairs)
        involved = sorted({slug for pair in pairs for slug in pair})
        for kind, platforms in am.PLATFORMS.items():
            available = [
                am.artifact_name(kind, slug, platform)
                for slug in involved
                for platform in platforms
            ]
            for slug in involved:
                got = am.select(kind, slug, available)
                self.assertEqual(
                    got,
                    sorted(am.artifact_name(kind, slug, platform) for platform in platforms),
                )


class ReleaseAssetTest(unittest.TestCase):
    def test_upload_glob_keeps_each_prefix_pair(self):
        slugs = am.library_slugs()
        pairs = am.prefix_pairs(slugs)
        involved = sorted({slug for pair in pairs for slug in pair})
        files = [name for slug in involved for name in am.release_asset_names(slug)]
        for slug in involved:
            got = sorted(name for name in files if am.release_upload_matches(slug, name))
            self.assertEqual(got, sorted(am.release_asset_names(slug)))

    def test_bash_upload_glob_matches_the_workflow(self):
        slugs = list(GITHUB)
        files = [name for slug in slugs for name in am.release_asset_names(slug)]
        with tempfile.TemporaryDirectory() as tmp:
            download_dir = Path(tmp) / "dl"
            download_dir.mkdir()
            for name in files:
                (download_dir / name).write_bytes(b"")
            for slug in slugs:
                globs = " ".join(am.UPLOAD_GLOBS)
                script = f"""
set -euo pipefail
shopt -s nullglob
assets=( {globs} )
if [ "${{#assets[@]}}" -eq 0 ]; then
  exit 0
fi
basename -a "${{assets[@]}}"
"""
                result = subprocess.run(
                    ["bash", "-c", script],
                    check=True,
                    text=True,
                    capture_output=True,
                    env={**os.environ, "RUNNER_TEMP": tmp, "SLUG": slug},
                )
                got = sorted(result.stdout.split())
                self.assertEqual(got, sorted(am.release_asset_names(slug)))


class WorkflowContractTest(unittest.TestCase):
    def test_build_workflow_uses_exact_artifact_and_upload_globs(self):
        text = (am.repo_root() / ".github/workflows/build-mcpb.yml").read_text()
        for line in am.ARTIFACT_NAME_YAML.values():
            self.assertIn(line, text)
        for line in am.DOWNLOAD_PATTERN_YAML.values():
            self.assertIn(line, text)
        for line in am.UPLOAD_GLOBS:
            self.assertIn(line, text)
        for line in am.FORBIDDEN_DOWNLOAD_YAML:
            self.assertNotIn(line, text)
        for line in am.RELEASE_FILENAME_YAML:
            self.assertIn(line, text)
        self.assertNotIn("/dl/*", text)

    def test_no_workflow_download_pattern_is_a_slug_hyphen_star(self):
        workflows = am.repo_root() / ".github/workflows"
        offenders = []
        for path in sorted(workflows.glob("*.yml")):
            for match in am.SLUG_HYPHEN_STAR.finditer(path.read_text()):
                offenders.append(f"{path.name}: {match.group(0)}")
        self.assertEqual(offenders, [])


if __name__ == "__main__":
    unittest.main()
