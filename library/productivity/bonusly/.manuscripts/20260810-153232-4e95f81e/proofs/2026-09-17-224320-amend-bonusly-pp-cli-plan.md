---
date: 2026-09-17
target_cli: bonusly-pp-cli
amend_run_id: amend-2026-09-17T2243
scope_tier: bugs+features (single feature finding)
findings_count: 1
mode: direct
---

# Amend plan: bonusly-pp-cli — redemption suggestions by point balance

## Source ask (verbatim)

> update the bonusly printing press cli to help suggest redemptions based on number of points the user has

## Feasibility research (performed before scoping)

Bonusly's REST v1 API has **no rewards-catalog endpoint with prices**, confirmed
two ways:

1. The CLI's own `README.md` "Known Gaps" section #1 documents that
   `awards`/`incentives` (the closest thing to a claimable-rewards catalog)
   were live-probed across ~20 path variants each and every one 404'd; the
   command groups were removed entirely
   (`.printing-press-patches/bonusly-remove-broken-commands.json`).
2. The `Redemption` type (`internal/types/types.go`) — sourced from
   `GET /redemptions/{id}` and `GET /users/{id}/redemptions` — has fields
   `id, state, reward_name, claim_url, certificate_url, created_at`. No
   point-cost field exists anywhere in the confirmed schema.

The only real, working "points" datum is `earning_balance` (redeemable
balance) from `GET /users/me`, already surfaced by the `balance` command
(hand-patched in `promoted_balance.go` since the spec-derived
`/users/points_balance` 404s live).

Given this, a literal "tell me what I can afford from the catalog" feature
is not buildable without fabricating price data. Confirmed with the user
(AskUserQuestion) which scoped version to build; user selected:

**Local history + balance suggester** — new `redemptions suggest` command
shows current redeemable balance alongside the user's own historical
redeemed reward names, ranked by frequency/recency, with explicit
disclosure that afford-ability filtering isn't possible.

## Finding

| ID | Kind | Classification | Target | Rationale |
|----|------|-----------------|--------|-----------|
| F1 | add-command | feature | bonusly-pp-cli | Add `redemptions suggest`: live `earning_balance`/`giving_balance` (reusing the same `/users/me` call `balance` makes) + local redemption-history aggregation (reusing the same local-mirror pattern as sibling `redemptions forecast`), ranked by times-redeemed then recency. No price/afford-ability claim is made — disclosed via a `note` field and doc text. |

No findings suppressed (Phase 2 guards: no duplicate/overlapping open or
recently-merged PR found for this feature; PR #1899 was a prior unrelated
amend for user-lookup/auth. Stale-binary check skipped — published
`.printing-press.json` has no `version` field.)

## Implementation plan

New novel command, `pp:data-source auto` (mixed live + local, matching the
existing convention used by `recognition_gap`, `recognition_audit`,
`recognition_values`, `recognition_search_mine`).

### Files to add

- `internal/cli/redemptions_suggest.go` — command implementation:
  - `--dry-run` short-circuit (matches `dryRunOK` pattern).
  - `checkMissingMirrorGuard` first (before any client/network call), same
    ordering as `recognition_gap.go`; prints `"{}"` on missing mirror for
    `--json`/`--agent` (object-shaped output, matching `recognition_gap`'s
    convention — array-shaped siblings print `"[]"`).
  - Live balance fetch: `resolveReadWithStrategyAndResponsePath(..., "auto",
    "balance", false, "/users/me", ...)` — identical call to
    `promoted_balance.go`'s `balance` command. Parses `earning_balance` /
    `giving_balance` from the `{"result": {...}}` envelope (same envelope
    shape `resolveMyUser` already assumes for `/users/me`), with a bare-object
    fallback for defensiveness.
  - Local history: `SELECT reward_name, state, created_at FROM redemptions`
    via `store.OpenWithContext`, same as `redemptions_forecast.go`.
  - Pure helper `rankRedemptionSuggestions([]redemptionHistoryRow)
    []redemptionSuggestion` — aggregates by reward name, counts occurrences,
    tracks most recent `created_at`/`state`, sorts by times-redeemed desc
    then last-redeemed-at desc. Kept pure/no-I/O for unit testing (per
    code-philosophy pure-function preference) — the RunE body is otherwise
    the only place doing I/O, consistent with the rest of this package.
  - `--limit` flag (default 5) caps suggestion count.
  - JSON/agent output: `printJSONFiltered` with
    `{earning_balance, giving_balance, suggestions, note}` — matches the
    `map[string]any` + `printJSONFiltered` shape used by
    `redemptions_forecast.go` / `recognition_gap.go` / `recognition_values.go`
    (not `recognition_search_mine.go`'s bare-array + `printOutputWithFlagsMeta`
    shape, since this command's output is object-shaped).
  - Human output: `newTabWriter` summary + ranked table, same as forecast.
- `internal/cli/redemptions_suggest_test.go`:
  - `TestNovelRedemptionsSuggestHelpWires` — smoke test, identical shape to
    `redemptions_forecast_test.go` / `recognition_gap_test.go`.
  - `TestRankRedemptionSuggestions` — table-driven unit tests for the pure
    ranking function (empty input, frequency ordering, recency tie-break,
    empty reward_name fallback label).

### Files to edit (discoverability + docs; each is an existing
"redemptions forecast" touch point, extended in parallel for "redemptions
suggest")

- `internal/cli/redemptions.go` — wire
  `addNovelCommandIfAbsent(cmd, newNovelRedemptionsSuggestCmd(flags))`.
- `internal/cli/which.go` — add a `whichIndex` entry (capability-query
  index), grouped under "Local state that compounds" like its sibling.
- `internal/cli/root.go` — add a bullet to the root `--help` Highlights
  banner (hardcoded static string, matches existing 6-bullet list style).
- `internal/mcp/tools.go` — add one `command_mirror_capabilities` entry and
  one matching `playbook` entry (MCP agent-context discovery surfaces).
- `README.md` — add a "Unique Features" bullet under "Local state that
  compounds" (same section as forecast's bullet); append one clause to
  Known Gaps item #1 cross-referencing the new command as a workaround.
- `SKILL.md` — add the matching "Unique Features" bullet.

Explicitly NOT touched (established precedent — `redemptions forecast`
follows the same exclusions and neither file lists it):
`internal/cli/root_test.go`'s `TestDeclaredAPISurfaceReachable` (novel/local
commands are not declared-API-surface entries; the test only asserts
`expected ⊆ actual`, so no update is required or possible in a
breaking way), `internal/mcp/tools.go`'s per-resource `endpoints` array
(lists only real endpoint-backed subcommands), README/SKILL's plain
per-resource `### redemptions` listing (novel commands are Unique-Features-only),
and `.printing-press.json` (generation-time manifest; PR #1899, the prior
amend on this same CLI, did not touch it either — established precedent for
this repo).

### Patch manifest

New patch file `.printing-press-patches/bonusly-redemptions-suggest.json`
declaring this whole customization (`files`: the six edited/added Go files
+ README.md + SKILL.md; `call_sites`: the literal `"redemptions suggest"`
command string and `newNovelRedemptionsSuggestCmd` symbol).

## Verification plan

- `go build ./...`, `go vet ./...`, `go test ./...` inside the CLI module.
- `bonusly-pp-cli redemptions suggest --help` renders.
- `bonusly-pp-cli redemptions --help` lists `suggest` alongside `get`,
  `list-mine`, `forecast`.
- `cli-printing-press publish validate --dir <CLI_DIR>` clean.
- `python3 .github/scripts/verify-skill/verify_skill.py --dir <CLI_DIR>` (if
  present in the managed clone) or the packaged equivalent.

## Risks

- No live Bonusly credential available in this environment, so the live
  balance call path is exercised only via `--dry-run`/`--help` and unit
  tests of the pure ranking helper — matches the disclosed testing posture
  of every other `pp:data-source auto`/`live` novel command in this CLI
  (`recognition_gap`, `recognition_search_mine` docs both say "not yet
  verified against a real API response in this build"). This command's
  Unique-Features bullet and `which.go`/`tools.go` entries carry the same
  disclosure.

## Validation results

`go build ./...`, `go vet ./...`, and `go test ./...` all pass inside the
CLI module (including the two new tests). Manual exercise of
`redemptions --help`, `redemptions suggest --help`, and
`redemptions suggest --dry-run` all render correctly.

`cli-printing-press publish validate --dir <CLI_DIR>` reports one failing
check: `phase5` ("phase5 marker source fingerprint does not match the
current CLI source"). **Confirmed pre-existing and unrelated to this
change**: re-ran the identical validate command against a clean checkout of
`upstream/main` (this amend's changes stashed out) and the same check fails
identically, listing dozens of files last touched by PR #1899 (2026-09-02,
the prior amend on this CLI: user-lookup fixes, PAT-only auth) — none of
which this amend touches. The stored fingerprint in
`.manuscripts/20260810-153232-4e95f81e/proofs/phase5-acceptance.json` was
never refreshed after PR #1899 shipped, so it has been stale since that
merge; PR #1899 itself was merged despite this same drift. Refreshing this
marker requires re-running the Phase 5 live-acceptance dogfood against a
real Bonusly credential, which is outside this environment's disclosed
constraints (see README.md "Known Gaps" preamble: "originally generated
without access to a live Bonusly API credential"). Not attempted as part of
this scoped amend; flagging for a future pass with live credentials rather
than silently absorbing an unrelated fix into this PR.

All other checks pass: manifest, transcendence, go mod tidy, module path,
govulncheck, go vet, go build, --help, --version, verify-skill, manuscripts.
