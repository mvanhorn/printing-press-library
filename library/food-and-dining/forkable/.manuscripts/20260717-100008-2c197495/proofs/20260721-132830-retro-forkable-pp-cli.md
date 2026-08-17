# Printing Press Retro: forkable

## Session Stats
- API: forkable
- Spec source: browser-sniffed (hand-authored spec from authenticated HAR of forkable.com/mc/ Vue SPA)
- Scorecard: 83/100
- Verify pass rate: 95.7% (45/47)
- Fix loops: 1 (polish pass)
- Manual code edits: 2 files (new `internal/cli/forkable_orders.go`, wired into `root.go`)
- Features built from scratch: 5 write commands (`meal set/set-all/confirm/skip`, `reorder`) — an amendment turning a read-only CLI into a meal-management write surface
- Flow: `/printing-press-amend` (bounced twice) → local build in library → `/printing-press-polish`

## Context

This was an **amendment** session, not a fresh generation. The user asked to add
`place-order`/`reorder` commands to the already-generated (local-only) forkable
CLI. The endpoint discovery was done **statically** from forkable.com's SPA JS
bundle (app.js + 117 route chunks), recovering the real GraphQL mutations
(`replacePiece`, `replaceAllPieces`, `confirmDelivery`, `removeDelivery(-ies)`) —
Forkable has no native `placeOrder` (meals are auto-selected). No live orders were
placed. Write commands were built dry-run-by-default with `--confirm` gating and a
`PRINTING_PRESS_VERIFY=1` short-circuit.

## Findings

### 1. Setup contract corrupted by slash-command arg substitution (Skill instruction gap / rendering-layer)
- **What happened:** When any press skill (`/printing-press`, `/printing-press-amend`) is invoked *with trailing args*, the rendered setup-contract bash block has the user's argument words substituted into the `$1`/`$2`/`$3` shell positional placeholders inside the version-comparison helpers. This session's args ("forkable sniff forkable.com for the order-placing GraphQL mutation...") rendered `_semver_lt()` as `awk -v a="sniff" -v b="forkable.com"` (should be `-v a="$1" -v b="$2"`), `_pp_go_version_norm` as `printf "%d.%d.%d\n", sniff, forkable.com, (for == "" ? 0 : for)` (should be `$1, $2, ($3 == ...)`), and version-check lines as `awk -F= '/^last_check=/{print forkable.com}'` (should be `{print $2}`). Running the contract verbatim would break every semver/toolchain-currency comparison.
- **Scorer correct?** N/A (not a score-penalty finding).
- **Root cause:** The **on-disk `SKILL.md` is correct** — verified `awk -v a="$1" -v b="$2"` (line 149) and `{print $2}` (lines 450, 507, 534). The corruption is introduced by the layer that injects the skill body + args into the model context: it appears to perform a naive positional substitution of the slash-command args into `$1`/`$2`/`$3` tokens *inside the skill body*, not just at the documented ARGUMENTS boundary. This is a rendering/injection-layer bug, so the strict five-component taxonomy (generator/spec-parser/openapi-parser/skill/scorer) does not contain the true root cause.
- **Cross-API check:** Recurs on **every** press-skill invocation that carries trailing args AND whose setup contract uses shell positionals in `awk -v`/`printf` — which is the shared `PRESS_SETUP_CONTRACT` block, i.e. all of `/printing-press`, `/printing-press-amend`, and any sibling that embeds it. Not API-specific at all.
- **Frequency:** every args-bearing invocation of a contract-embedding skill.
- **Fallback if the Printing Press doesn't fix it:** The agent must notice the mangled `awk` (subtle — it still parses, just compares wrong values) and run preflight from ground truth, as done this session. Silent-wrong is the danger: a corrupted `_semver_lt` can misjudge binary currency and either skip a needed upgrade gate or fire a spurious one.
- **Worth a Printing Press fix?** Yes — the skill-side mitigation is durable and cheap: make the contract's helper functions resilient to positional-arg clobbering. The true fix is in the injection layer (outside this repo), but the skill can defend itself.
- **Inherent or fixable:** Fixable. Two mitigations: (a) in the contract, pass values to `awk`/`printf` via environment (`awk -v a="$A" -v b="$B"` where `A`/`B` are named shell vars set just above) or via `awk 'BEGIN{...}' "$1" "$2"` positional-file forms, avoiding bare `$1`/`$2` tokens that the injector rewrites; (b) escape or template the positionals so the injection layer doesn't treat them as substitutable. (a) is self-contained in `SKILL.md` and testable.
- **Durable fix:** Rewrite the `PRESS_SETUP_CONTRACT` version helpers so no `$1`/`$2`/`$3` token appears inside an `awk -v` assignment, `printf` format-arg list, or `awk -F=` action — assign to named shell variables first (`_a="$1"; _b="$2"`) and reference those. This removes the substitution surface the injector clobbers. Separately, file/raise the injection-layer bug wherever that renderer lives (likely the harness, not this repo).
- **Test:** Positive — invoke a contract-embedding skill with a multi-word arg containing tokens that look like values (`foo 1.2.3 bar`); assert the rendered `_semver_lt`/`_pp_go_version_norm` still contain shell-variable references, not the literal arg words. Negative — invoke with no args; assert the contract is byte-identical to today.
- **Evidence:** This session's rendered `/printing-press` and `/printing-press-amend` contract blocks (visible in the conversation) showed `awk -v a="sniff" -v b="forkable.com"` and `printf ... , sniff, forkable.com, (for == ...)`. On-disk SKILL confirmed correct via grep.
- **Related prior retros:** None (first retro in this manuscripts tree).
- **Case-against (Step G):** "Root cause is outside the five components, so it's not a Printing Press fix." Rebuttal: the skill *source* owns the contract text and can be written to survive arg-injection; that is a concrete, testable change to `skills/printing-press/SKILL.md`. The case-against fails because a self-defending contract is strictly better regardless of who owns the injector.

### 2. `/printing-press-amend` has no fast-fail for local-only targets, and no note that it can't run in a forked subagent (Skill instruction gap)
- **What happened:** Three amend attempts this session dead-ended. (a) Two ran as forked subagents, which lack `AskUserQuestion`, so amend's mandatory user checkpoints (U1 transcript-path confirm, U4 scope, U7 PR-draft review) could not fire — the runs bounced with no artifact. (b) The main-session attempt resolved `published_status: local-only` (forkable is 404 in the public library, no `.printing-press-release.json`) but only *after* full Phase 0/1 discovery, and there is no gate that acts on `local-only` — amend's sole artifact is a PR against the published library, so a local-only target has no valid destination. The user had to be told "publish first, or use polish" manually.
- **Scorer correct?** N/A.
- **Root cause:** `printing-press-amend/SKILL.md` resolves `published_status` in Phase 1a step 5 and Phase 1b step 2, but the value is only consumed to route `published` → PR path. There is no early branch that stops on `local-only` with actionable guidance, and no environmental precondition documented (must run in an interactive session with `AskUserQuestion`).
- **Cross-API check:** Recurs for **any** amend invocation whose target is a locally-generated-but-unpublished CLI — a common state, since `/printing-press` leaves CLIs in `$PRESS_LIBRARY` without publishing. The subagent-checkpoint issue recurs for any amend invoked via the Skill/Task fork path rather than the main REPL.
- **Frequency:** local-only targets: common (every unpublished CLI). Subagent invocation: whenever amend is dispatched to a forked agent.
- **Fallback if the Printing Press doesn't fix it:** Agent spends a full discovery pass before hitting the wall, then reconstructs the guidance by hand (as this session did across three attempts).
- **Worth a Printing Press fix?** Yes. Both fixes are localized to the amend skill and prevent wasted discovery passes.
- **Inherent or fixable:** Fixable.
- **Durable fix:** (a) Add a Phase 0/early-Phase-1 gate: immediately after `published_status` resolves, if `local-only`, stop with a clear message — "amend targets published-library CLIs; this CLI is local-only. Publish it first with `/printing-press-publish`, or apply local changes with `/printing-press-polish`." (b) Add a precondition note at the top of amend: the skill requires interactive checkpoints (`AskUserQuestion`) and cannot complete from a forked subagent; if the tool set lacks `AskUserQuestion`, tell the user to re-invoke from their main session.
- **Test:** Positive — invoke amend on a slug absent from the public library; assert it stops at the gate with publish/polish guidance and does no Phase 1 discovery. Negative — invoke on a published slug; assert it proceeds to the PR path unchanged.
- **Evidence:** Two subagent transcripts this session bounced on missing `AskUserQuestion`; the main-session run confirmed 404 via `gh api repos/mvanhorn/printing-press-library/contents/library/food-and-dining/forkable` and `.printing-press-release.json` absent.
- **Related prior retros:** None.
- **Case-against (Step G):** "This is amend-specific, and amend isn't in the five canonical components." Rebuttal: amend is a first-class press skill shipped in this repo (`skills/printing-press-amend/SKILL.md`); a fast-fail gate is a concrete skill edit that saves a full discovery pass on a common target state. Case-against fails.

### 3. dogfood reimplementation-check false-positives on helper-indirected API calls (Scorer bug)
- **What happened:** dogfood reported "3/7 novel features reimplemented" (`served-history`, `preference-drift`, `venue-rotation`). These are false positives: all three call the live API through a shared hand-authored helper (`fetchGraphQL()` in `forkable_gql.go`), but the reimplementation heuristic greps for direct `client.`/`store.` usage *in the command file* and misses the one-hop indirection through a sibling helper in the same package.
- **Scorer correct?** No — scorer bug. The commands do call the real API; they just do it through a centralized helper. The polish pass independently classified these as false positives (documented in the polish proof).
- **Root cause:** `scorer` (dogfood reimplementation check). The detection is a per-command-file grep for `client.`/`store.` tokens; it has no notion of a command delegating API access to a sibling helper function within the same package. Cannot inspect exact source (external run, repo not checked out), so the fix locus is described by behavior.
- **Cross-API check:** Would recur for any CLI that centralizes API access in a hand-authored helper package/file — a reasonable pattern, especially for GraphQL CLIs (one endpoint, many operations) where a shared `fetch`/`mutate` helper is the natural shape. **However, Step B evidence is weak: the local library has only 2 CLIs (forkable, magnite-candidates), and only forkable exhibits the pattern.** Cannot name 3 concrete APIs with evidence.
- **Frequency:** subclass — CLIs with a hand-authored shared API helper. Unknown breadth from local evidence.
- **Fallback if the Printing Press doesn't fix it:** Agent (or polish) manually classifies the finding as a false positive each time — cheap per-instance, but it erodes trust in the reimplementation signal and risks masking a *real* reimplementation next to a false one.
- **Worth a Printing Press fix?** Marginal — filed at P3 because Step B cannot name 3 APIs. A false positive in a scorer heuristic is a verifiable bug from a single instance, so it's worth recording, but the breadth to justify P1/P2 isn't demonstrated.
- **Inherent or fixable:** Fixable. The check could follow intra-package function calls one hop (does the command call a sibling func that itself uses `client.`/`store.`?), or treat presence of a package-level hand-authored API helper as an exemption signal, or key off the `pp:data-source live` marker comment the helper already carries.
- **Durable fix:** Extend the reimplementation heuristic to resolve one level of same-package helper indirection before concluding a command reimplements logic — or honor an explicit `pp:data-source live` / helper-exemption marker.
- **Test:** Positive — a command whose only API access is via a sibling helper func should NOT be flagged as reimplemented. Negative — a command that genuinely inlines a hand-rolled HTTP client (no real `client.`/`store.`/helper call) should still be flagged.
- **Evidence:** Polish proof line: `dogfood "sync uses generic Upsert" / "3/7 novel reimplemented" — false positives (no sync command by design; novel features share cross-file live-fetch helper).`
- **Related prior retros:** None.
- **Case-against (Step G):** "Only one CLI shows this; it's a single-instance observation." Rebuttal: a scorer *false positive* is a correctness bug provable from one instance — the check reports "reimplemented" for a command that demonstrably calls the real API. It doesn't need 3 APIs to be *wrong*; it needs 3 APIs to be *high-priority*. Hence it survives to P3, not higher.

## Prioritized Improvements

### P1 — High priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F1 | Setup contract corrupted by slash-command arg substitution | skill | every args-bearing invocation | low (silent-wrong; subtle to notice) | small | none needed — pure hardening |

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F2 | amend: no fast-fail for local-only + no subagent precondition note | skill | common (unpublished targets; forked invocations) | medium | small | n/a |

### P3 — Low priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F3 | dogfood reimplementation false-positive on helper-indirected calls | scorer | subclass: CLIs with shared API helper | medium (polish catches it) | medium | must not stop flagging genuinely-inlined clients |

### Skip
| Finding | Title | Why it didn't make it |
|---------|-------|-----------------------|
| — | — | (none — the three survivors above all cleared Step G) |

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| C4 | read helper `fetchGraphQL` omitted GraphQL `variables`, so writes needed a parallel `mutateGraphQL` | printed-CLI: normal per-CLI feature-building; a machine-emitted mutation helper would be dead code for read-only CLIs; only 1 GraphQL CLI in library (Step B fails) |

## Work Units

### WU-1: Harden setup-contract version helpers against arg-injection (from F1)
- **Priority:** P1
- **Component:** skill
- **Goal:** The `PRESS_SETUP_CONTRACT` version/currency helpers survive slash-command argument injection intact.
- **Target:** `skills/printing-press/SKILL.md` `PRESS_SETUP_CONTRACT` block (and any sibling skill that embeds the same contract, e.g. `printing-press-amend`).
- **Acceptance criteria:**
  - positive: invoking a contract-embedding skill with a multi-word arg (`foo 1.2.3 bar`) yields a rendered contract whose `_semver_lt`, `_pp_go_version_norm`, and `awk -F=` currency lines still reference shell variables, not the literal arg words.
  - negative: invoking with no args produces a byte-identical contract to today's behavior.
- **Scope boundary:** Does NOT fix the injection-layer renderer itself (that's a harness bug filed separately); only makes the contract text resilient. Does NOT change contract semantics.
- **Dependencies:** none.
- **Complexity:** small.

### WU-2: amend fast-fail for local-only targets + subagent precondition note (from F2)
- **Priority:** P2
- **Component:** skill
- **Goal:** `/printing-press-amend` stops early with actionable guidance when the target is local-only or when it can't run interactive checkpoints.
- **Target:** `skills/printing-press-amend/SKILL.md` — add an early gate after `published_status` resolution, and a top-of-skill precondition note.
- **Acceptance criteria:**
  - positive: amend on a slug absent from the public library stops before Phase 1 discovery with "publish first (`/printing-press-publish`) or polish locally (`/printing-press-polish`)" guidance.
  - positive: amend documents that it requires `AskUserQuestion` and cannot complete from a forked subagent; when that tool is unavailable it tells the user to re-invoke from the main session.
  - negative: amend on a published slug proceeds through the existing PR path unchanged.
- **Scope boundary:** Does NOT add a publish step to amend; only redirects. Does NOT change the PR-path behavior for published CLIs.
- **Dependencies:** none.
- **Complexity:** small.

### WU-3: dogfood reimplementation-check honors same-package helper indirection (from F3)
- **Priority:** P3
- **Component:** scorer
- **Goal:** The reimplementation check stops flagging commands that reach the API through a hand-authored sibling helper.
- **Target:** scorer (dogfood reimplementation check).
- **Acceptance criteria:**
  - positive: a command whose only API access is a call to a sibling package-level helper that itself uses `client.`/`store.` is NOT flagged as reimplemented.
  - negative: a command that inlines a hand-rolled HTTP client with no real `client.`/`store.`/helper delegation is still flagged.
- **Scope boundary:** One hop of same-package indirection (or an explicit exemption marker); not a full call-graph analysis.
- **Dependencies:** none.
- **Complexity:** medium.

## Anti-patterns
- **Quoting a hardcoded count in narrative that tracks a runtime list** — avoided; the amendment's description uses qualitative phrasing.
- **Silent scope downgrade** — avoided; the read-only→write change was surfaced to the user with an explicit safety-model decision (dry-run default + `--confirm`) before building.

## What the Printing Press Got Right
- **`pp:data-source live` marker + hand-authored-file preservation** worked exactly as designed: `forkable_gql.go` and the new `forkable_orders.go` (no generated header) survive `generate --force`, and the amendment slotted in cleanly.
- **Polish's source-of-truth discipline** correctly propagated the read-only→write description change across all 8 surfaces (`.printing-press.json`, `manifest.json`, `tools-manifest.json`, `spec.yaml`, `root.go`, README, SKILL) from `research.json`, and correctly identified its own dogfood false positives rather than "fixing" them.
- **MCP annotation model** — the write commands correctly carry NO `mcp:read-only`, so hosts bucket them as write tools; the runtime Cobra-tree mirror picked them up with zero MCP-specific wiring.
- **verify-mode short-circuit (`PRINTING_PRESS_VERIFY=1`) and `dryRunOK`/`boundCtx`/`usageErr` helpers** gave the new money-flowing write commands a safe, conventional shape out of the box.
