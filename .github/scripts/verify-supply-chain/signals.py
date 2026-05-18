"""Signal catalog for the supply-chain scan.

Each signal is a pure function that inspects a diff for a specific attack
shape and returns Findings. No I/O. No network. Easy to unit-test.

Signals are tiered by severity:
  block  - hard-fail; exit code 1; PR cannot merge.
  advise - notice only; exit code unchanged; surfaces for review.

The catalog is intentionally regex-driven rather than YAML-parsed: the
shapes we look for (id-token: write, GOPROXY env, replace directives,
postinstall scripts) are line-level signals that survive every common
YAML / JSON quirk and don't require a parser dependency.
"""

from __future__ import annotations

import json
import re
from dataclasses import dataclass
from pathlib import PurePosixPath


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
    lines added in this diff. It is the primary input for signals that
    should only fire on *new* introductions, not on pre-existing state.
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


# Workflows allowed to grant `id-token: write`. Empty in the generator-repo
# mirror; populated here for the published-library repo.
ID_TOKEN_ALLOWLIST = {".github/workflows/npm-publish.yml"}

# The canonical module-path prefix every library CLI must keep.
CANONICAL_MODULE_PREFIX = "github.com/mvanhorn/printing-press-library/library/"


# ---------------------------------------------------------------------------
# R1: pull_request_target + non-default checkout ref (TanStack OIDC theft)
# ---------------------------------------------------------------------------


# Detects pull_request_target as a YAML trigger declaration in any of YAML's
# valid forms:
#   on:
#     pull_request_target:                ← block form
#     pull_request_target:                ← block form with types
#       types: [...]
#   on: pull_request_target               ← inline form
#   on:
#     - pull_request_target               ← list form
#   on: [pull_request_target, push]       ← flow-sequence form
# Anchored so prose mentions inside comments or string values don't false-fire.
_PR_TARGET_TRIGGER_LINE = re.compile(
    r"^\s*(?:-\s+|on\s*:\s*)?pull_request_target(?:\s*:|\s*$)",
    re.MULTILINE,
)
_PR_TARGET_TRIGGER_FLOW = re.compile(
    r"^\s*on\s*:\s*\[[^\]\n]*\bpull_request_target\b",
    re.MULTILINE,
)


def _has_pr_target_trigger(content: str) -> bool:
    return bool(
        _PR_TARGET_TRIGGER_LINE.search(content) or _PR_TARGET_TRIGGER_FLOW.search(content)
    )


_CHECKOUT_USES = re.compile(r"^\s*-\s*uses\s*:\s*actions/checkout", re.MULTILINE)
_DANGEROUS_REF = re.compile(
    r"ref\s*:\s*[^\n#]*"
    r"(github\.event\.pull_request\.head\.(sha|ref)|refs/pull/[^\n]*?/(merge|head))",
)


def _lines_to_scan(change: FileChange) -> list[tuple[int, str]]:
    """Return [(line_no, content)] for lines that should be scanned for new
    additions. Diff-aware semantics:

      - File didn't exist on base (new file)  → every head line is "new".
      - File existed on base                 → only the added_lines diff.

    Lets R1/R2/R4 fire only on newly-introduced patterns. An attacker can't
    silently re-introduce a dangerous pattern that pre-existed on main, and
    unrelated PRs don't get false-flagged because main happens to contain a
    legitimate pattern in some allowlisted location.
    """
    if change.head_content is None:
        return []
    if change.base_content is None:
        return list(enumerate(change.head_content.splitlines(), start=1))
    return list(change.added_lines)


def signal_workflow_trust(change: FileChange) -> list[Finding]:
    """R1. A workflow that combines pull_request_target with a checkout of
    the PR head ref is the TanStack mini-Shai-Hulud attack shape — head
    code runs with the elevated permissions of the base context, including
    secrets and OIDC.

    Fires when:
      - The file's HEAD contains both a pull_request_target trigger AND an
        actions/checkout step (these are the prerequisites — they may exist
        on base unchanged, and that's fine).
      - At least one *newly-introduced* line carries a dangerous ref OR is
        itself the trigger / checkout (so a freshly added attack shape is
        caught regardless of which of the three components landed last).
    """
    if not is_workflow(change.path) or change.head_content is None:
        return []

    content = change.head_content
    if not _has_pr_target_trigger(content):
        return []
    if not _CHECKOUT_USES.search(content):
        return []

    danger_match = _DANGEROUS_REF.search(content)
    if not danger_match:
        return []

    # Diff-aware: fire only if any of the three attack ingredients was
    # introduced by this PR. For a new file every line is "new" so the
    # full-content match implicitly counts.
    relevant_lines = _lines_to_scan(change)
    introduced_here = False
    danger_line: int | None = None
    danger_text: str = danger_match.group(0).strip()
    for line_no, line in relevant_lines:
        if _DANGEROUS_REF.search(line):
            introduced_here = True
            danger_line = line_no
            danger_text = _DANGEROUS_REF.search(line).group(0).strip()
            break
        if _has_pr_target_trigger(line) or _CHECKOUT_USES.search(line):
            introduced_here = True

    if not introduced_here:
        return []

    if danger_line is None:
        # Trigger or checkout was newly added; dangerous ref was already on
        # base. Report at the dangerous-ref position from head_content.
        danger_line = content.count("\n", 0, danger_match.start()) + 1

    return [
        Finding(
            path=change.path,
            line=danger_line,
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


_ID_TOKEN_WRITE = re.compile(r"^\s*id-token\s*:\s*write\s*(?:#.*)?$")


def signal_id_token_outside_allowlist(change: FileChange) -> list[Finding]:
    """R2. id-token: write mints OIDC tokens the publisher uses to push to
    npm, Sigstore, AWS, etc. It should exist only in the workflow(s) that
    actually publish. Anywhere else is a leak vector.

    Diff-aware: fires only when the line is newly introduced (added in this
    PR or part of a new file). Pre-existing grants on base aren't re-flagged.
    """
    if not is_workflow(change.path) or change.head_content is None:
        return []
    if change.path in ID_TOKEN_ALLOWLIST:
        return []

    findings: list[Finding] = []
    for line_no, line_content in _lines_to_scan(change):
        if not _ID_TOKEN_WRITE.match(line_content):
            continue
        findings.append(
            Finding(
                path=change.path,
                line=line_no,
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
        )
    return findings


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
    # bare host pattern: `example.com/path` — anything with a dot before the first slash.
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
            ranges.append((start, j))  # j is 1-indexed close-line-minus-1 (== j in 0-indexed)
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


_GO_ENV_OVERRIDE = re.compile(
    r"^\s*(GOPROXY|GOFLAGS|GONOSUMCHECK|GOSUMDB|GONOSUMDB)\s*:\s*\S"
)


def signal_go_env_override(change: FileChange) -> list[Finding]:
    """R4. Setting GOPROXY / GOFLAGS / GONOSUMCHECK / GOSUMDB inside a
    workflow env block lets an attacker redirect module resolution or
    suppress checksum verification (BufferZoneCorp).

    Diff-aware: fires only on newly-introduced overrides.
    """
    if not is_workflow(change.path) or change.head_content is None:
        return []

    findings: list[Finding] = []
    for line_no, line_content in _lines_to_scan(change):
        match = _GO_ENV_OVERRIDE.match(line_content)
        if not match:
            continue
        var = match.group(1)
        findings.append(
            Finding(
                path=change.path,
                line=line_no,
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


def signal_module_path_drift(change: FileChange) -> list[Finding]:
    """R6. The `module` directive in library/**/go.mod is what the registry
    generator (and downstream `go install`) uses to resolve the published
    binary. A PR that rewrites it on an existing CLI to a non-canonical
    path silently redirects every future install.
    """
    if not is_library_gomod(change.path):
        return []
    if change.head_content is None:
        return []

    head_module = _extract_module_path(change.head_content)
    if head_module is None:
        return []

    base_module = _extract_module_path(change.base_content)

    # New CLI: we still require canonical prefix.
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

    # Existing CLI: ANY module-directive change blocks. Catches both:
    #   1. Drift outside the canonical prefix (the original BufferZoneCorp
    #      shape — module redirected to attacker fork).
    #   2. Within-canonical-prefix renames (e.g., kalshi → kalshi-evil) that
    #      still start with github.com/mvanhorn/printing-press-library/library/
    #      but redirect `go install` to a different slug. Self-evident in PR
    #      review for humans, but worth blocking mechanically since renames
    #      of published CLIs are generator-pipeline operations, not manual
    #      go.mod edits.
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


def _find_line(content: str, needle: str) -> int | None:
    for idx, line in enumerate(content.splitlines(), start=1):
        if needle in line:
            return idx
    return None


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
