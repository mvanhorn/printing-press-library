import os
from pathlib import Path
import stat
import subprocess
import tempfile
import unittest


SCRIPT = Path(__file__).with_name("push.sh")
REPO_ROOT = Path(__file__).resolve().parents[3]


class PushGeneratedArtifactTest(unittest.TestCase):
    def test_retries_when_main_moves_between_rebase_and_push(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            bin_dir = root / "bin"
            bin_dir.mkdir()
            log = root / "git.log"
            state = root / "push-count"
            fake_git = bin_dir / "git"
            fake_git.write_text(
                """#!/usr/bin/env bash
set -euo pipefail
printf '%s\\n' "$*" >> "$FAKE_GIT_LOG"
if [ "$1" != "push" ]; then
  exit 0
fi
count=0
if [ -f "$FAKE_GIT_STATE" ]; then
  count=$(<"$FAKE_GIT_STATE")
fi
count=$((count + 1))
printf '%s\\n' "$count" > "$FAKE_GIT_STATE"
if [ "$count" -le "${FAKE_PUSH_FAILURES:-0}" ]; then
  echo "remote moved before push" >&2
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
                    "FAKE_GIT_STATE": str(state),
                    "FAKE_PUSH_FAILURES": "1",
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
                log.read_text(encoding="utf-8").splitlines(),
            )

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
