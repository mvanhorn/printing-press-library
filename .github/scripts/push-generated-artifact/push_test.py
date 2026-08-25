import os
from pathlib import Path
import stat
import subprocess
import tempfile
import unittest


SCRIPT = Path(__file__).with_name("push.sh")
REPO_ROOT = Path(__file__).resolve().parents[3]


class PushGeneratedArtifactTest(unittest.TestCase):
    def run_with_failures(
        self,
        *,
        fetch_failures: int = 0,
        push_failures: int = 0,
        max_attempts: int = 5,
    ) -> tuple[subprocess.CompletedProcess[str], list[str]]:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            bin_dir = root / "bin"
            bin_dir.mkdir()
            log = root / "git.log"
            fake_git = bin_dir / "git"
            fake_git.write_text(
                """#!/usr/bin/env bash
set -euo pipefail
printf '%s\\n' "$*" >> "$FAKE_GIT_LOG"
case "$1" in
  fetch)
    state="$FAKE_FETCH_STATE"
    failures="${FAKE_FETCH_FAILURES:-0}"
    ;;
  push)
    state="$FAKE_PUSH_STATE"
    failures="${FAKE_PUSH_FAILURES:-0}"
    ;;
  *) exit 0 ;;
esac
count=0
if [ -f "$state" ]; then
  count=$(<"$state")
fi
count=$((count + 1))
printf '%s\\n' "$count" > "$state"
if [ "$count" -le "$failures" ]; then
  echo "transient $1 failure" >&2
  exit 1
fi
""",
                encoding="utf-8",
            )
            fake_git.chmod(fake_git.stat().st_mode | stat.S_IXUSR)

            env = os.environ.copy()
            env.update(
                {
                    "PATH": f"{bin_dir}:{env['PATH']}",
                    "FAKE_GIT_LOG": str(log),
                    "FAKE_FETCH_STATE": str(root / "fetch-count"),
                    "FAKE_PUSH_STATE": str(root / "push-count"),
                    "FAKE_FETCH_FAILURES": str(fetch_failures),
                    "FAKE_PUSH_FAILURES": str(push_failures),
                    "PUSH_RETRY_MAX_ATTEMPTS": str(max_attempts),
                    "PUSH_RETRY_DELAY_SECONDS": "0",
                }
            )

            result = subprocess.run(
                ["bash", str(SCRIPT), "main"],
                env=env,
                text=True,
                capture_output=True,
                check=False,
            )

            return result, log.read_text(encoding="utf-8").splitlines()

    def test_retries_when_main_moves_between_rebase_and_push(self) -> None:
        result, commands = self.run_with_failures(push_failures=1)

        self.assertEqual(0, result.returncode, result.stderr)
        self.assertEqual(
            [
                "fetch origin main",
                "rebase origin/main",
                "push origin HEAD:main",
                "fetch origin main",
                "rebase origin/main",
                "push origin HEAD:main",
            ],
            commands,
        )

    def test_retries_a_transient_fetch_failure(self) -> None:
        result, commands = self.run_with_failures(fetch_failures=1)

        self.assertEqual(0, result.returncode, result.stderr)
        self.assertEqual(
            [
                "fetch origin main",
                "fetch origin main",
                "rebase origin/main",
                "push origin HEAD:main",
            ],
            commands,
        )

    def test_stops_after_the_retry_budget_is_exhausted(self) -> None:
        result, commands = self.run_with_failures(
            fetch_failures=3,
            max_attempts=3,
        )

        self.assertEqual(1, result.returncode)
        self.assertEqual(["fetch origin main"] * 3, commands)
        self.assertIn("failed after 3 attempts", result.stdout)

    def test_all_main_branch_writers_use_retry_helper(self) -> None:
        workflows = [
            "generate-registry.yml",
            "generate-skills.yml",
            "normalize-patches.yml",
            "update-cli-release-ledger.yml",
        ]

        for name in workflows:
            with self.subTest(workflow=name):
                body = (REPO_ROOT / ".github" / "workflows" / name).read_text(
                    encoding="utf-8"
                )
                self.assertIn(
                    "bash .github/scripts/push-generated-artifact/push.sh main",
                    body,
                )
                self.assertIn(
                    "- '.github/scripts/push-generated-artifact/**'",
                    body,
                )


if __name__ == "__main__":
    unittest.main()
