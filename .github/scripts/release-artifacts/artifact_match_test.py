"""The release workflow must download one CLI's artifacts and no prefix sibling's."""

from __future__ import annotations

import re
import subprocess
import unittest
from pathlib import Path

import yaml

import artifact_match


WORKFLOW = artifact_match.repo_root() / ".github" / "workflows"
BUILD = WORKFLOW / "build-mcpb.yml"

GITHUB_RELEASE_ASSETS = [
    "github-pp-cli-darwin-amd64",
    "github-pp-cli-darwin-arm64",
    "github-pp-cli-linux-amd64",
    "github-pp-cli-linux-arm64",
    "github-pp-cli-windows-amd64.exe",
    "github-pp-cli-windows-arm64.exe",
    "github-pp-mcp-darwin-arm64.mcpb",
    "github-pp-mcp-windows-amd64.mcpb",
    "github-pp-mcp-windows-arm64.mcpb",
]


def workflow_text() -> str:
    return BUILD.read_text()


def workflow_platforms() -> dict[str, list[str]]:
    data = yaml.safe_load(workflow_text())
    jobs = data["jobs"]
    return {
        "mcpb": list(jobs["bundle"]["strategy"]["matrix"]["platform"]),
        "cli": list(jobs["build-cli"]["strategy"]["matrix"]["platform"]),
    }


def owned_suffixes(text: str) -> list[str]:
    return re.findall(r"(?m)^[ \t]+(pp-(?:cli|mcp)-[A-Za-z0-9.-]+)$", text)


class GithubFamilyTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.slugs = artifact_match.slugs()

    def test_github_selects_only_itself_and_the_others_still_do(self) -> None:
        family = ["github", "github-contents", "github-intel"]
        for slug in family:
            self.assertIn(slug, self.slugs)
            for kind in artifact_match.KINDS:
                got = artifact_match.selected_slugs(kind, slug, self.slugs)
                self.assertEqual(got, [slug], f"{kind} pattern for {slug}")

    def test_previous_glob_mixed_hyphen_prefixes_only(self) -> None:
        self.assertIn(("github", "github-contents", "hyphen"), artifact_match.prefix_pairs(self.slugs))
        self.assertIn(("github", "github-intel", "hyphen"), artifact_match.prefix_pairs(self.slugs))
        old = artifact_match.selected_slugs("cli", "github", self.slugs, previous=True)
        self.assertEqual(old, ["github", "github-contents", "github-intel"])
        old_mcpb = artifact_match.selected_slugs("mcpb", "github", self.slugs, previous=True)
        self.assertEqual(old_mcpb, ["github", "github-contents", "github-intel"])
        # square-current's old glob does not match squarespace: the hyphen is required.
        self.assertEqual(
            artifact_match.selected_slugs("cli", "square", self.slugs, previous=True),
            ["square"],
        )
        for shorter, longer, kind in artifact_match.prefix_pairs(self.slugs):
            for artifact_kind in artifact_match.KINDS:
                exact = artifact_match.selected_slugs(artifact_kind, shorter, self.slugs)
                self.assertEqual(exact, [shorter])
                previous = artifact_match.selected_slugs(
                    artifact_kind, shorter, self.slugs, previous=True
                )
                if kind == "hyphen":
                    self.assertIn(longer, previous)
                else:
                    self.assertNotIn(longer, previous)

    def test_plan_cli_exits_clean_for_the_github_family(self) -> None:
        script = Path(artifact_match.__file__)
        for slug in ("github", "github-contents", "github-intel"):
            completed = subprocess.run(
                ["python3", str(script), "plan", "--slug", slug],
                check=False,
                text=True,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
            )
            self.assertEqual(completed.returncode, 0, completed.stderr)
            self.assertIn(f"cli selected slugs: {slug}\n", completed.stdout)
            self.assertIn(f"mcpb selected slugs: {slug}\n", completed.stdout)
        github = subprocess.run(
            ["python3", str(script), "plan", "--slug", "github"],
            check=True,
            text=True,
            stdout=subprocess.PIPE,
        ).stdout
        self.assertIn(
            "cli previous slugs: github, github-contents, github-intel",
            github,
        )
        self.assertIn("github-pp-cli-linux-amd64", github)
        self.assertNotIn("github-contents-pp-cli", github)


class ReleaseAssetNameTest(unittest.TestCase):
    def test_github_assets_keep_the_names_already_on_releases(self) -> None:
        got = []
        for kind in artifact_match.KINDS:
            for platform in artifact_match.platforms_for(kind):
                got.append(artifact_match.release_asset_name(kind, "github", platform))
        self.assertEqual(got, GITHUB_RELEASE_ASSETS)

    def test_actions_artifact_name_is_not_the_release_asset_name(self) -> None:
        self.assertEqual(
            artifact_match.artifact_name("cli", "github", "linux/amd64"),
            "cli-github__linux-amd64",
        )
        self.assertEqual(
            artifact_match.release_asset_name("cli", "github", "linux/amd64"),
            "github-pp-cli-linux-amd64",
        )


class WorkflowWiringTest(unittest.TestCase):
    def test_upload_and_download_use_the_delimiter(self) -> None:
        text = workflow_text()
        for kind in artifact_match.KINDS:
            self.assertIn(
                f"name: {kind}-${{{{ env.SLUG }}}}__${{{{ env.PLATFORM_TAG }}}}",
                text,
            )
            self.assertIn(
                f"pattern: {kind}-${{{{ steps.slug.outputs.slug }}}}__*",
                text,
            )
        self.assertNotIn("steps.slug.outputs.slug }}-*", text)
        self.assertNotIn("env.SLUG }}-${{", text)
        self.assertIn(
            'out="${RUNNER_TEMP}/out/${slug}-pp-cli-${os}-${arch}${ext}"',
            text,
        )
        self.assertIn(
            'out="${RUNNER_TEMP}/out/${slug}-pp-mcp-${os}-${arch}.mcpb"',
            text,
        )

    def test_matrix_platforms_match_the_asset_list(self) -> None:
        self.assertEqual(workflow_platforms()["cli"], list(artifact_match.CLI_PLATFORMS))
        self.assertEqual(workflow_platforms()["mcpb"], list(artifact_match.MCPB_PLATFORMS))
        self.assertEqual(owned_suffixes(workflow_text()), artifact_match.asset_suffixes())

    def test_publish_compares_asset_names_exactly(self) -> None:
        text = workflow_text()
        self.assertIn('[ "$base" = "${SLUG}-${suffix}" ]', text)
        self.assertLess(
            text.index('[ "$base" = "${SLUG}-${suffix}" ]'),
            text.index('gh release upload "${tag}"'),
        )
        suffixes = owned_suffixes(text)
        for slug, filename, expect in (
            ("github", "github-pp-cli-linux-amd64", True),
            ("github", "github-pp-cli-windows-amd64.exe", True),
            ("github", "github-pp-mcp-darwin-arm64.mcpb", True),
            ("github", "github-contents-pp-cli-linux-amd64", False),
            ("github", "github-intel-pp-mcp-windows-amd64.mcpb", False),
            ("github-contents", "github-contents-pp-cli-darwin-arm64", True),
            ("github-contents", "github-pp-cli-darwin-arm64", False),
            ("github-intel", "github-intel-pp-cli-linux-arm64", True),
            ("costco", "costco-sameday-pp-cli-linux-amd64", False),
            ("costco", "costco-pp-cli-linux-amd64", True),
            ("openrouter", "openrouter-image-pp-mcp-darwin-arm64.mcpb", False),
            ("square", "squarespace-pp-cli-linux-amd64", False),
            ("substack", "substack-reader-pp-cli-darwin-arm64", False),
        ):
            with self.subTest(slug=slug, filename=filename):
                self.assertEqual(_bash_owns(slug, filename, suffixes), expect)

    def test_no_workflow_downloads_with_a_slug_prefix_glob(self) -> None:
        bad_pattern = re.compile(
            r"pattern:\s*\S*\$\{\{[^}\n]+\}\}-\*"
        )
        bad_shell = re.compile(
            r"(?:cli|mcpb)-\$\{(?:slug|SLUG|steps\.slug\.outputs\.slug)\}-\*"
        )
        offenders = []
        for path in sorted(WORKFLOW.glob("*.yml")):
            text = path.read_text()
            if bad_pattern.search(text) or bad_shell.search(text):
                offenders.append(path.name)
        self.assertEqual(offenders, [])

    def test_catalog_slugs_leave_the_delimiter_unambiguous(self) -> None:
        for slug in artifact_match.slugs():
            artifact_match.check_slug(slug)


def _bash_owns(slug: str, filename: str, suffixes: list[str]) -> bool:
    script = """
base=$1
SLUG=$2
shift 2
owned=0
for suffix in "$@"; do
  if [ "$base" = "${SLUG}-${suffix}" ]; then
    owned=1
    break
  fi
done
if [ "$owned" -eq 1 ]; then
  exit 0
fi
exit 1
"""
    completed = subprocess.run(
        ["bash", "-c", script, "bash", filename, slug, *suffixes],
        check=False,
    )
    return completed.returncode == 0


if __name__ == "__main__":
    unittest.main()
