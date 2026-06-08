import json
import subprocess
import tempfile
import unittest
from pathlib import Path


SCRIPT = Path(__file__).with_name("verify_release_ledger.py")


def run(cmd, cwd, check=True):
    return subprocess.run(cmd, cwd=cwd, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=check)


class ReleaseLedgerVerifierTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.repo = Path(self.tmp.name)
        run(["git", "init"], self.repo)
        run(["git", "config", "user.email", "test@example.com"], self.repo)
        run(["git", "config", "user.name", "Test User"], self.repo)
        run(["git", "checkout", "-b", "main"], self.repo)

    def tearDown(self):
        self.tmp.cleanup()

    def write(self, path, text):
        target = self.repo / path
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(text, encoding="utf-8")

    def seed_existing_cli(self):
        root = "library/social-and-messaging/x-twitter"
        self.write(f"{root}/.printing-press.json", json.dumps({"api_name": "x-twitter"}) + "\n")
        self.write(
            f"{root}/.printing-press-release.json",
            json.dumps(
                {
                    "schema_version": 1,
                    "slug": "x-twitter",
                    "version": "2026.6.1",
                    "released_at": "2026-06-08T00:00:00Z",
                    "source_commit": "base",
                    "changes": [{"title": "Baseline", "commit": "base"}],
                },
                indent=2,
            )
            + "\n",
        )
        self.write(
            f"{root}/CHANGELOG.md",
            "# Changelog\n\nThis file is maintained by printing-press-library release automation. Do not hand-edit release sections in normal PRs.\n\n## 2026.6.1 - 2026-06-08\n\n- Baseline.\n",
        )
        self.write(f"{root}/internal/cli/root.go", 'package cli\n\nvar version = "2026.6.1"\n')
        run(["git", "add", "."], self.repo)
        run(["git", "commit", "-m", "base"], self.repo)

    def verifier(self):
        return run(["python3", str(SCRIPT), "--base-ref", "HEAD~1"], self.repo, check=False)

    def verifier_against(self, base_ref):
        return run(["python3", str(SCRIPT), "--base-ref", base_ref], self.repo, check=False)

    def test_blocks_feature_pr_with_release_manifest_and_changelog_edits(self):
        self.seed_existing_cli()
        root = "library/social-and-messaging/x-twitter"
        self.write(f"{root}/README.md", "# X Twitter\n\nNew feature.\n")
        self.write(
            f"{root}/.printing-press-release.json",
            json.dumps({"schema_version": 1, "slug": "x-twitter", "version": "2026.6.2", "changes": []}) + "\n",
        )
        self.write(f"{root}/CHANGELOG.md", "# Changelog\n\n## 2026.6.2 - 2026-06-08\n\n- New feature.\n")
        run(["git", "add", "."], self.repo)
        run(["git", "commit", "-m", "feature plus manual ledger"], self.repo)

        result = self.verifier()

        self.assertNotEqual(result.returncode, 0)
        self.assertIn("post-merge automation updates them", result.stderr)
        self.assertIn("x-twitter", result.stderr)

    def test_allows_existing_cli_ledger_only_repair(self):
        self.seed_existing_cli()
        root = "library/social-and-messaging/x-twitter"
        self.write(
            f"{root}/.printing-press-release.json",
            json.dumps({"schema_version": 1, "slug": "x-twitter", "version": "2026.6.1", "changes": []}) + "\n",
        )
        run(["git", "add", "."], self.repo)
        run(["git", "commit", "-m", "repair ledger"], self.repo)

        self.assertEqual(self.verifier().returncode, 0)

    def test_allows_blank_skeleton_for_new_cli(self):
        self.seed_existing_cli()
        root = "library/ai/new-cli"
        self.write(f"{root}/.printing-press.json", json.dumps({"api_name": "new-cli"}) + "\n")
        self.write(
            f"{root}/.printing-press-release.json",
            json.dumps(
                {
                    "schema_version": 1,
                    "slug": "new-cli",
                    "version": "",
                    "released_at": "",
                    "source_commit": "",
                }
            )
            + "\n",
        )
        self.write(
            f"{root}/CHANGELOG.md",
            "# Changelog\n\nThis file is maintained by printing-press-library release automation. Do not hand-edit release sections in normal PRs.\n\n",
        )
        run(["git", "add", "."], self.repo)
        run(["git", "commit", "-m", "add new cli"], self.repo)

        self.assertEqual(self.verifier().returncode, 0)

    def test_blocks_new_cli_with_populated_changelog_release(self):
        self.seed_existing_cli()
        root = "library/ai/new-cli"
        self.write(f"{root}/.printing-press.json", json.dumps({"api_name": "new-cli"}) + "\n")
        self.write(
            f"{root}/.printing-press-release.json",
            json.dumps({"schema_version": 1, "slug": "new-cli", "version": "2026.6.1"}) + "\n",
        )
        self.write(f"{root}/CHANGELOG.md", "# Changelog\n\n## 2026.6.1 - 2026-06-08\n\n- Added.\n")
        run(["git", "add", "."], self.repo)
        run(["git", "commit", "-m", "bad new cli ledger"], self.repo)

        result = self.verifier()

        self.assertNotEqual(result.returncode, 0)
        self.assertIn("must leave 'version' blank", result.stderr)
        self.assertIn("must not contain a CalVer release section", result.stderr)

    def test_blocks_runtime_version_bump_with_feature_change(self):
        self.seed_existing_cli()
        root = "library/social-and-messaging/x-twitter"
        self.write(f"{root}/README.md", "# X Twitter\n\nNew feature.\n")
        self.write(f"{root}/internal/cli/root.go", 'package cli\n\nvar version = "2026.6.2"\n')
        run(["git", "add", "."], self.repo)
        run(["git", "commit", "-m", "feature plus version bump"], self.repo)

        result = self.verifier()

        self.assertNotEqual(result.returncode, 0)
        self.assertIn("runtime version changed with normal CLI files", result.stderr)

    def test_allows_stale_feature_branch_after_main_release_automation(self):
        self.seed_existing_cli()
        root = "library/social-and-messaging/x-twitter"
        run(["git", "checkout", "-b", "feature"], self.repo)
        self.write(f"{root}/README.md", "# X Twitter\n\nNew feature.\n")
        run(["git", "add", "."], self.repo)
        run(["git", "commit", "-m", "feature only"], self.repo)

        run(["git", "checkout", "main"], self.repo)
        self.write(
            f"{root}/.printing-press-release.json",
            json.dumps(
                {
                    "schema_version": 1,
                    "slug": "x-twitter",
                    "version": "2026.6.2",
                    "released_at": "2026-06-08T01:00:00Z",
                    "source_commit": "automation",
                    "changes": [{"title": "Automated release", "commit": "automation"}],
                },
                indent=2,
            )
            + "\n",
        )
        self.write(f"{root}/CHANGELOG.md", "# Changelog\n\n## 2026.6.2 - 2026-06-08\n\n- Automated release.\n")
        run(["git", "add", "."], self.repo)
        run(["git", "commit", "-m", "automation release ledger"], self.repo)

        run(["git", "checkout", "feature"], self.repo)
        self.assertEqual(self.verifier_against("main").returncode, 0)

    def test_allows_stale_feature_branch_when_main_bumped_runtime_version(self):
        self.seed_existing_cli()
        root = "library/social-and-messaging/x-twitter"
        run(["git", "checkout", "-b", "feature"], self.repo)
        self.write(
            f"{root}/internal/cli/root.go",
            'package cli\n\nvar version = "2026.6.1"\n\nfunc featureFlag() bool { return true }\n',
        )
        run(["git", "add", "."], self.repo)
        run(["git", "commit", "-m", "feature root change"], self.repo)

        run(["git", "checkout", "main"], self.repo)
        self.write(f"{root}/internal/cli/root.go", 'package cli\n\nvar version = "2026.6.2"\n')
        run(["git", "add", "."], self.repo)
        run(["git", "commit", "-m", "automation version bump"], self.repo)

        run(["git", "checkout", "feature"], self.repo)
        self.assertEqual(self.verifier_against("main").returncode, 0)


if __name__ == "__main__":
    unittest.main()
