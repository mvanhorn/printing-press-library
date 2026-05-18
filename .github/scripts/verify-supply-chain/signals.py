"""Signal catalog for the supply-chain scan.

Each signal is a pure function that inspects a diff for a specific attack
shape and returns Findings. No I/O. No network. Easy to unit-test.

Signals are tiered by severity:
  block  - hard-fail; exit code 1; PR cannot merge.
  advise - notice only; exit code unchanged; surfaces for review.

Detection strategy:
  - R1/R2/R4 operate on YAML workflow files and parse them structurally
    with pyyaml (pre-installed on ubuntu-latest runners). The earlier
    regex approach kept missing valid YAML forms (block-form `replace`,
    flow-sequence triggers, block-scalar `ref: >-` with the value on the
    next line) and required a patch per quirk. Structural parsing
    eliminates the whole class.
  - R3 (go.mod replace) and R6 (module-path drift) operate on go.mod
    files; a regex-on-text approach is appropriate there.
  - R5 (npm lifecycle scripts) parses npm/package.json as JSON and
    compares head vs base script tables.
"""

from __future__ import annotations

import json
import re
from dataclasses import dataclass
from pathlib import PurePosixPath
from typing import Any

import yaml


# ---------------------------------------------------------------------------
# Types
# ---------------------------------------------------------------------------


@dataclass(frozen=True)
class Finding:
    """A single signal hit at a specific file location."""

    path: str
    line: int | None
    severity: str  # "block" | "advise"
    signal_id: str
    message: str
    remediation: str

    def is_block(self) -> bool:
        return self.severity == "block"


@dataclass(frozen=True)
class FileChange:
    """One file from a PR diff.

    base_content is None when the file did not exist on the base ref (newly
    added). head_content is None when the file was deleted on head. Most
    signals fire only when head_content is non-None — we don't analyze
    deletions.

    added_lines is the list of (1-indexed line number, content) pairs for
    lines added in this diff. Used by R3 (gomod_replace) for line-level
    diff awareness; R1/R2/R4 use structural diff (parse base + head) so
    they don't depend on added_lines.
    """

    path: str
    base_content: str | None
    head_content: str | None
    added_lines: list[tuple[int, str]]


# ---------------------------------------------------------------------------
# Path-scope helpers
# ---------------------------------------------------------------------------


def is_workflow(path: str) -> bool:
    parts = PurePosixPath(path).parts
    return (
        len(parts) >= 3
        and parts[0] == ".github"
        and parts[1] == "workflows"
        and (path.endswith(".yml") or path.endswith(".yaml"))
    )


def is_library_gomod(path: str) -> bool:
    parts = PurePosixPath(path).parts
    return len(parts) >= 4 and parts[0] == "library" and parts[-1] == "go.mod"


def is_npm_package_json(path: str) -> bool:
    return path == "npm/package.json"


# Workflows allowed to grant `id-token: write`. The published-library repo
# uses OIDC Trusted Publishing only in npm-publish.yml.
ID_TOKEN_ALLOWLIST = {".github/workflows/npm-publish.yml"}

# The canonical module-path prefix every library CLI must keep.
CANONICAL_MODULE_PREFIX = "github.com/mvanhorn/printing-press-library/library/"


# ---------------------------------------------------------------------------
# YAML parsing helpers (shared by R1, R2, R4)
# ---------------------------------------------------------------------------


def _parse_workflow(content: str | None) -> Any:
    """Parse a workflow YAML safely. Returns the loaded structure (typically
    a dict) or None if the content is absent or unparseable. A malformed
    workflow would also fail to load in GitHub Actions itself, so silently
    skipping it here is safe."""
    if content is None:
        return None
    try:
        return yaml.safe_load(content)
    except (yaml.YAMLError, TypeError, ValueError):
        return None


def _workflow_on(parsed: Any) -> Any:
    """Extract the workflow's `on:` section.

    YAML 1.1 (pyyaml's default) interprets unquoted `on` as the boolean
    literal True — so `on:` at the top of a workflow becomes the key True
    after parsing. Check both forms; GitHub Actions accepts either."""
    if not isinstance(parsed, dict):
        return None
    if "on" in parsed:
        return parsed["on"]
    if True in parsed:
        return parsed[True]
    return None


def _has_pr_target_trigger(on_node: Any) -> bool:
    """Detect pull_request_target in any of YAML's trigger declaration forms:
    string (`on: pull_request_target`), list (`on: [...]` or
    `on:\\n  - pull_request_target`), or mapping (`on:\\n  pull_request_target:`)."""
    if on_node is None:
        return False
    if isinstance(on_node, str):
        return on_node.strip() == "pull_request_target"
    if isinstance(on_node, list):
        return any(
            isinstance(item, str) and item.strip() == "pull_request_target"
            for item in on_node
        )
    if isinstance(on_node, dict):
        return "pull_request_target" in on_node
    return False


_DANGEROUS_REF_VALUE = re.compile(
    # All forms that resolve to PR-author-controlled content under
    # pull_request_target. github.head_ref is the shorthand alias for
    # event.pull_request.head.ref (Greptile-flagged on PR 1619); the
    # merge_commit_sha points at GitHub's synthesised merge commit which
    # also contains PR code.
    r"github\.event\.pull_request\.head\.(sha|ref)"
    r"|github\.event\.pull_request\.merge_commit_sha"
    r"|github\.head_ref"
    # [^\n] (not [^\s]) so the match survives spaces inside `${{ ... }}`
    # expressions, e.g., refs/pull/${{ github.event.number }}/merge.
    r"|refs/pull/[^\n]*?/(merge|head)"
)


def _is_dangerous_ref_value(value: Any) -> bool:
    """Return True if a checkout step's `ref:` value references the PR head."""
    if not isinstance(value, str):
        return False
    return bool(_DANGEROUS_REF_VALUE.search(value))


def _walk_checkout_refs(parsed: Any) -> list[str]:
    """Walk parsed workflow for actions/checkout steps with dangerous ref
    values. Returns the list of dangerous ref values found (one per offending
    step). Inspects every job and every step recursively but only flags the
    `with.ref` field of `uses: actions/checkout*` steps."""
    if not isinstance(parsed, dict):
        return []
    jobs = parsed.get("jobs")
    if not isinstance(jobs, dict):
        return []
    findings: list[str] = []
    for job in jobs.values():
        if not isinstance(job, dict):
            continue
        steps = job.get("steps")
        if not isinstance(steps, list):
            continue
        for step in steps:
            if not isinstance(step, dict):
                continue
            uses = step.get("uses")
            if not isinstance(uses, str) or not uses.startswith("actions/checkout"):
                continue
            with_block = step.get("with")
            if not isinstance(with_block, dict):
                continue
            ref = with_block.get("ref")
            if _is_dangerous_ref_value(ref):
                findings.append(ref.strip())
    return findings


def _walk_id_token_grants(parsed: Any) -> bool:
    """Return True if the parsed workflow grants `id-token: write` at workflow
    or job level. (GitHub Actions doesn't honor step-level permissions, so we
    skip those.)"""
    if not isinstance(parsed, dict):
        return False
    if _permissions_grant_id_token(parsed.get("permissions")):
        return True
    jobs = parsed.get("jobs")
    if isinstance(jobs, dict):
        for job in jobs.values():
            if isinstance(job, dict) and _permissions_grant_id_token(job.get("permissions")):
                return True
    return False


def _permissions_grant_id_token(perm: Any) -> bool:
    if isinstance(perm, dict):
        value = perm.get("id-token")
        return isinstance(value, str) and value.strip() == "write"
    # A string value of "write-all" grants every permission, including id-token.
    if isinstance(perm, str) and perm.strip() == "write-all":
        return True
    return False


_GO_ENV_KEYS = ("GOPROXY", "GOFLAGS", "GONOSUMCHECK", "GOSUMDB", "GONOSUMDB")


def _walk_go_env_overrides(parsed: Any) -> list[str]:
    """Return the list of GOPROXY/GOFLAGS/etc env-variable names set anywhere
    in the workflow's env blocks (workflow-level, job-level, or step-level)."""
    found: list[str] = []
    if not isinstance(parsed, dict):
        return found
    _collect_go_env_from(parsed.get("env"), found)
    jobs = parsed.get("jobs")
    if isinstance(jobs, dict):
        for job in jobs.values():
            if not isinstance(job, dict):
                continue
            _collect_go_env_from(job.get("env"), found)
            steps = job.get("steps")
            if isinstance(steps, list):
                for step in steps:
                    if isinstance(step, dict):
                        _collect_go_env_from(step.get("env"), found)
    return found


def _collect_go_env_from(env_block: Any, out: list[str]) -> None:
    if not isinstance(env_block, dict):
        return
    for key in env_block:
        if isinstance(key, str) and key in _GO_ENV_KEYS:
            out.append(key)


def _find_line_in(content: str | None, needle: str) -> int | None:
    """Best-effort line number lookup. Returns the 1-indexed line where the
    substring first appears, or None."""
    if not content or not needle:
        return None
    for idx, line in enumerate(content.splitlines(), start=1):
        if needle in line:
            return idx
    return None


# ---------------------------------------------------------------------------
# R1: pull_request_target + PR-head checkout (TanStack OIDC theft)
# ---------------------------------------------------------------------------


def signal_workflow_trust(change: FileChange) -> list[Finding]:
    """R1. A workflow that combines pull_request_target with a checkout of
    the PR head ref is the TanStack mini-Shai-Hulud attack shape — head
    code runs with the elevated permissions of the base context, including
    secrets and OIDC.

    Structural diff: fires when head has the bad combo AND base didn't have
    the same dangerous ref value. (Reformatting the same dangerous YAML in
    a different style is not a new attack — only newly-introduced danger
    fires.)"""
    if not is_workflow(change.path) or change.head_content is None:
        return []

    head = _parse_workflow(change.head_content)
    if not _has_pr_target_trigger(_workflow_on(head)):
        return []

    head_refs = _walk_checkout_refs(head)
    if not head_refs:
        return []

    base = _parse_workflow(change.base_content)
    base_refs: set[str] = set()
    if base is not None and _has_pr_target_trigger(_workflow_on(base)):
        base_refs = set(_walk_checkout_refs(base))

    new_dangerous = [r for r in head_refs if r not in base_refs]
    if not new_dangerous:
        return []

    danger_text = new_dangerous[0]
    line = _find_line_in(change.head_content, danger_text)
    return [
        Finding(
            path=change.path,
            line=line,
            severity="block",
            signal_id="workflow_trust_pr_head_checkout",
            message=(
                "pull_request_target workflow checks out PR head code "
                "(matched: %r). This is the TanStack mini-Shai-Hulud attack "
                "shape — head code runs with base-context secrets and OIDC." % danger_text
            ),
            remediation=(
                "Use `pull_request` instead, or omit the `ref:` override on "
                "actions/checkout so it stays on the base commit. Never run "
                "PR head code under pull_request_target."
            ),
        )
    ]


# ---------------------------------------------------------------------------
# R2: id-token: write outside the publishing allowlist
# ---------------------------------------------------------------------------


def signal_id_token_outside_allowlist(change: FileChange) -> list[Finding]:
    """R2. id-token: write mints OIDC tokens the publisher uses to push to
    npm, Sigstore, AWS, etc. It should exist only in the workflow(s) that
    actually publish. Anywhere else is a leak vector.

    Structural diff: fires only if the head workflow grants id-token: write
    and the base didn't (or didn't exist)."""
    if not is_workflow(change.path) or change.head_content is None:
        return []
    if change.path in ID_TOKEN_ALLOWLIST:
        return []

    head = _parse_workflow(change.head_content)
    if not _walk_id_token_grants(head):
        return []

    base = _parse_workflow(change.base_content)
    if base is not None and _walk_id_token_grants(base):
        return []

    # Pick a needle the file actually contains. The grant can come from
    # either a literal `id-token: write` line at workflow- or job-level, OR
    # from a `permissions: write-all` shorthand at workflow- or job-level.
    # Check for "id-token" first since that's the precise location; fall
    # back to "write-all" when the grant only fires via the shorthand
    # (covers both workflow-level and job-level write-all uniformly).
    if "id-token" in change.head_content:
        needle = "id-token"
    else:
        needle = "write-all"
    line = _find_line_in(change.head_content, needle)
    return [
        Finding(
            path=change.path,
            line=line,
            severity="block",
            signal_id="id_token_outside_allowlist",
            message=(
                "id-token: write is granted in a workflow outside the "
                "publishing allowlist (%s)." % ", ".join(sorted(ID_TOKEN_ALLOWLIST))
            ),
            remediation=(
                "Remove the id-token permission, or move the publishing "
                "logic into a workflow file already on the allowlist. "
                "OIDC scopes are credentials — narrow them."
            ),
        )
    ]


# ---------------------------------------------------------------------------
# R3: replace directives in library go.mod (BufferZoneCorp)
# ---------------------------------------------------------------------------


# Captures the single-line form: `replace <module> [<version>] => <target> [<version>]`
_REPLACE_LINE = re.compile(
    r"^\s*replace\s+\S+(?:\s+v\S+)?\s*=>\s*(?P<target>\S+)"
)

# Captures the inner body of a block-form replace, which appears as
# `<module> [<version>] => <target> [<version>]` *without* a leading `replace`
# keyword. Only valid inside a `replace ( ... )` block — see _replace_block_ranges.
_REPLACE_BLOCK_BODY = re.compile(
    r"^\s*\S+(?:\s+v\S+)?\s*=>\s*(?P<target>\S+)"
)

# Opens a block-form replace: `replace (` possibly followed by whitespace/comment.
_REPLACE_BLOCK_OPEN = re.compile(r"^\s*replace\s*\(\s*(?:\s*//.*)?$")
_REPLACE_BLOCK_CLOSE = re.compile(r"^\s*\)\s*(?:\s*//.*)?$")


def _classify_replace_target(target: str) -> str:
    """Return 'remote' or 'local' for a replace directive target.

    Local: starts with `./`, `../`, or `/` (absolute path).
    Remote: contains a host segment (`example.com/...`) or scheme.
    """
    if target.startswith(("./", "../", "/")):
        return "local"
    if "://" in target:
        return "remote"
    head = target.split("/", 1)[0]
    if "." in head:
        return "remote"
    return "local"  # conservative


def _replace_block_ranges(content: str | None) -> list[tuple[int, int]]:
    """Return [(start_line, end_line)] (1-indexed, inclusive) for each
    `replace ( ... )` block in go.mod content. start_line is the line AFTER
    the opening `replace (`; end_line is the line BEFORE the closing `)`.
    """
    if not content:
        return []
    ranges: list[tuple[int, int]] = []
    lines = content.splitlines()
    i = 0
    while i < len(lines):
        if _REPLACE_BLOCK_OPEN.match(lines[i]):
            start = i + 2  # first body line (1-indexed)
            j = i + 1
            while j < len(lines) and not _REPLACE_BLOCK_CLOSE.match(lines[j]):
                j += 1
            ranges.append((start, j))
            i = j + 1
        else:
            i += 1
    return ranges


def _in_block(line_no: int, ranges: list[tuple[int, int]]) -> bool:
    return any(start <= line_no <= end for start, end in ranges)


def signal_gomod_replace(change: FileChange) -> list[Finding]:
    """R3. New `replace` directives in library/**/go.mod. Tiered:
      - remote target → block (BufferZoneCorp redirect-to-attacker-fork shape).
      - local target  → advise (legitimate vendoring, but still worth a look).

    Catches BOTH go.mod replace syntaxes:
      replace foo => bar v1.0.0                          (single-line form)
      replace (                                          (block form — the
          foo => bar v1.0.0                               inner lines have
      )                                                   no `replace` prefix)
    """
    if not is_library_gomod(change.path):
        return []

    block_ranges = _replace_block_ranges(change.head_content)
    findings: list[Finding] = []
    for line_no, line_content in change.added_lines:
        target: str | None = None
        single_match = _REPLACE_LINE.match(line_content)
        if single_match:
            target = single_match.group("target")
        elif _in_block(line_no, block_ranges):
            body_match = _REPLACE_BLOCK_BODY.match(line_content)
            if body_match:
                target = body_match.group("target")
        if target is None:
            continue
        kind = _classify_replace_target(target)
        if kind == "remote":
            findings.append(
                Finding(
                    path=change.path,
                    line=line_no,
                    severity="block",
                    signal_id="gomod_replace_remote_target",
                    message=(
                        "New `replace` directive in go.mod redirects to a remote "
                        "target (%s). This is the BufferZoneCorp attack shape — "
                        "a published CLI silently pulling from an attacker fork." % target
                    ),
                    remediation=(
                        "Remove the replace directive. If a forked dependency is "
                        "genuinely required, vendor it locally (./third_party/...) "
                        "and record the customization in .printing-press-patches.json."
                    ),
                )
            )
        else:
            findings.append(
                Finding(
                    path=change.path,
                    line=line_no,
                    severity="advise",
                    signal_id="gomod_replace_local_target",
                    message=(
                        "New `replace` directive in go.mod points at a local path (%s). "
                        "Likely legitimate vendoring; flagging for review." % target
                    ),
                    remediation=(
                        "Confirm the local path is checked into the same PR and that "
                        ".printing-press-patches.json records the customization."
                    ),
                )
            )
    return findings


# ---------------------------------------------------------------------------
# R4: GOPROXY / GOFLAGS / GONOSUMCHECK overrides in workflows
# ---------------------------------------------------------------------------


def signal_go_env_override(change: FileChange) -> list[Finding]:
    """R4. Setting GOPROXY / GOFLAGS / GONOSUMCHECK / GOSUMDB inside a
    workflow env block lets an attacker redirect module resolution or
    suppress checksum verification (BufferZoneCorp).

    Structural diff: fires for each go-env key newly set in head that wasn't
    set in base."""
    if not is_workflow(change.path) or change.head_content is None:
        return []

    head_vars = set(_walk_go_env_overrides(_parse_workflow(change.head_content)))
    if not head_vars:
        return []

    base_vars: set[str] = set()
    if change.base_content is not None:
        base_vars = set(_walk_go_env_overrides(_parse_workflow(change.base_content)))

    new_vars = head_vars - base_vars
    if not new_vars:
        return []

    findings: list[Finding] = []
    for var in sorted(new_vars):
        findings.append(
            Finding(
                path=change.path,
                line=_find_line_in(change.head_content, var),
                severity="block",
                signal_id="go_env_override_in_workflow",
                message=(
                    "Workflow sets %s in an env block. This can redirect Go "
                    "module resolution to an attacker proxy or suppress "
                    "checksum verification (BufferZoneCorp attack shape)." % var
                ),
                remediation=(
                    "Remove the env override. If a private GOPROXY is required, "
                    "configure it at the org or runner level under operator review, "
                    "not in a workflow file that PRs can modify."
                ),
            )
        )
    return findings


# ---------------------------------------------------------------------------
# R5: postinstall / preinstall / prepare scripts added to npm/package.json
# ---------------------------------------------------------------------------


_WATCHED_NPM_SCRIPTS = ("preinstall", "postinstall", "prepare")


def signal_npm_lifecycle_script(change: FileChange) -> list[Finding]:
    """R5. Adding postinstall / preinstall / prepare to npm/package.json is
    the Axios attack shape: the lifecycle hook fires on every `npm install`
    or `npx` invocation and runs attacker code in user shells.
    """
    if not is_npm_package_json(change.path) or change.head_content is None:
        return []

    try:
        head_data = json.loads(change.head_content)
    except (json.JSONDecodeError, TypeError):
        return []
    base_data: dict = {}
    if change.base_content:
        try:
            base_data = json.loads(change.base_content)
        except (json.JSONDecodeError, TypeError):
            base_data = {}

    head_scripts = (head_data.get("scripts") or {}) if isinstance(head_data, dict) else {}
    base_scripts = (base_data.get("scripts") or {}) if isinstance(base_data, dict) else {}

    findings: list[Finding] = []
    for name in _WATCHED_NPM_SCRIPTS:
        if name in head_scripts and name not in base_scripts:
            findings.append(
                Finding(
                    path=change.path,
                    line=None,
                    severity="block",
                    signal_id="npm_lifecycle_script_added",
                    message=(
                        "New `%s` script added to npm/package.json. Lifecycle hooks "
                        "fire on every install (Axios / TanStack attack shape)." % name
                    ),
                    remediation=(
                        "Remove the lifecycle hook. Build steps belong in CI before "
                        "publish, not in scripts that run on user machines."
                    ),
                )
            )
    return findings


# ---------------------------------------------------------------------------
# R6: module-path drift on existing library go.mod
# ---------------------------------------------------------------------------


_MODULE_DIRECTIVE = re.compile(r"^\s*module\s+(\S+)", re.MULTILINE)


def _extract_module_path(content: str | None) -> str | None:
    if not content:
        return None
    match = _MODULE_DIRECTIVE.search(content)
    return match.group(1) if match else None


def _find_line(content: str | None, needle: str) -> int | None:
    """Thin alias of _find_line_in kept so R6 callsites don't change."""
    return _find_line_in(content, needle)


def signal_module_path_drift(change: FileChange) -> list[Finding]:
    """R6. The `module` directive in library/**/go.mod is what the registry
    generator (and downstream `go install`) uses to resolve the published
    binary. A PR that rewrites it on an existing CLI silently redirects
    every future install.
    """
    if not is_library_gomod(change.path):
        return []
    if change.head_content is None:
        return []

    head_module = _extract_module_path(change.head_content)
    if head_module is None:
        return []

    base_module = _extract_module_path(change.base_content)

    # New CLI: require canonical prefix.
    if base_module is None:
        if not head_module.startswith(CANONICAL_MODULE_PREFIX):
            return [
                Finding(
                    path=change.path,
                    line=_find_line(change.head_content, head_module),
                    severity="block",
                    signal_id="module_path_noncanonical_on_new_cli",
                    message=(
                        "New library go.mod declares module %s which does not start "
                        "with the canonical prefix %s." % (head_module, CANONICAL_MODULE_PREFIX)
                    ),
                    remediation=(
                        "Use the canonical form: module %s<category>/<slug>." % CANONICAL_MODULE_PREFIX
                    ),
                )
            ]
        return []

    # Existing CLI: ANY module-directive change blocks (drift outside the
    # canonical prefix OR within-canonical rename).
    if head_module != base_module:
        outside_canonical = not head_module.startswith(CANONICAL_MODULE_PREFIX)
        if outside_canonical:
            message = (
                "module directive on an existing library CLI changed from %s "
                "to %s, which is outside the canonical prefix %s. This silently "
                "redirects `go install` for every user."
                % (base_module, head_module, CANONICAL_MODULE_PREFIX)
            )
            signal_id = "module_path_drift_on_existing_cli"
        else:
            message = (
                "module directive on an existing library CLI changed from %s "
                "to %s. Even within the canonical prefix, renaming a published "
                "CLI redirects `go install` for users pinned to the old slug — "
                "this must go through the generator pipeline, not a manual edit."
                % (base_module, head_module)
            )
            signal_id = "module_path_rename_on_existing_cli"
        return [
            Finding(
                path=change.path,
                line=_find_line(change.head_content, head_module),
                severity="block",
                signal_id=signal_id,
                message=message,
                remediation=(
                    "Revert the module directive. Renaming or moving a published CLI "
                    "is a generator-repo operation, not a manual go.mod edit."
                ),
            )
        ]

    return []


# ---------------------------------------------------------------------------
# Signal dispatch
# ---------------------------------------------------------------------------


ALL_SIGNALS = (
    signal_workflow_trust,
    signal_id_token_outside_allowlist,
    signal_gomod_replace,
    signal_go_env_override,
    signal_npm_lifecycle_script,
    signal_module_path_drift,
)


def run_signals(change: FileChange) -> list[Finding]:
    findings: list[Finding] = []
    for sig in ALL_SIGNALS:
        findings.extend(sig(change))
    return findings
