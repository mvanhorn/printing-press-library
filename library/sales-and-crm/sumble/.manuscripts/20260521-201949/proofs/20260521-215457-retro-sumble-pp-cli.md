# Printing Press Retro: Sumble

## Session Stats
- API: sumble (Sumble v6 sales-intelligence API)
- Spec source: hand-authored internal YAML (no OpenAPI exists; modeled from docs.sumble.com)
- Scorecard: 86/100 (Grade A)
- Verify pass rate: PASS (shipcheck 5/5 legs)
- Fix loops: 1 (stack-diff enrich 422)
- Manual code edits: 8 hand-authored novel-command files (credit-economy suite)
- Features built from scratch: 7 (cost-estimate, balance, budget, spend, stale, stack-diff, reconcile)

## Findings

### 1. govulncheck quality gate fails on go1.26 modules (Bug / scorer+generator)
- **What happened:** `generate --validate` and `publish validate` both fail the
  govulncheck gate. The generator emits `go 1.26.3` in go.mod and the gate invokes a
  pinned `golang.org/x/vuln/cmd/govulncheck@v1.3.0` (the latest release). v1.3.0
  requires go>=1.25 and switches to the go1.25.10 toolchain, which then cannot parse
  go1.26 stdlib/packages — every package errors with "requires newer Go version go1.26
  (application built with go1.25)". go build, go vet, go mod tidy all pass.
- **Scorer correct?** No — the gate reports a failure that is not a code vulnerability;
  it is a toolchain-version incompatibility. The CLI has no reachable vuln.
- **Root cause:** The machine controls both inputs: it emits the `go 1.26.3` directive
  AND pins govulncheck@v1.3.0. On a go1.26 host the two are mutually incompatible, so
  the gate fails opaquely (a wall of "requires newer Go version" lines) rather than
  degrading with a clear "toolchain newer than govulncheck supports" message.
- **Cross-API check:** Universal — affects EVERY CLI generated/published on a go1.26
  host, regardless of API shape. Not an API subclass; a toolchain-wide block.
- **Frequency:** every API, every go1.26 user.
- **Fallback if not fixed:** The agent must recognize the failure as environmental and
  hand-wave past a security gate — exactly what a quality gate exists to prevent. And
  `publish validate` hard-stops, so publishing is fully blocked on go1.26 today.
- **Worth a fix?** Yes. The pinned version won't self-resolve when x/vuln ships go1.26
  support; the machine must bump the pin or detect the version skew.
- **Inherent or fixable:** Fixable. Options (disambiguate before implementing):
  (a) bump the govulncheck pin to a go1.26-capable release once available;
  (b) detect "installed go newer than govulncheck's max supported" and emit a clear
      SKIP/WARN with an actionable message instead of an opaque package-load failure;
  (c) run govulncheck under a GOTOOLCHAIN that matches the analyzer.
  Likely (b) as the durable degrade-gracefully behavior, plus (a) for coverage.
- **Durable fix:** Component scorer/generator. Make the govulncheck gate detect a
  toolchain-vs-analyzer version skew and degrade to WARN with a one-line reason, and
  track govulncheck releases so the pin moves to a go1.26-capable version.
- **Test:** On a go1.26 host, `generate --validate` should not hard-fail govulncheck
  with "requires newer Go version"; it should WARN with a clear toolchain-skew message
  (positive). On a go1.25 host with a compatible analyzer, govulncheck still runs and
  blocks on real reachable vulns (negative — don't silently disable the gate).
- **Evidence:** generate, generate --force, and publish validate all emitted the
  identical "go: golang.org/x/vuln@v1.3.0 requires go >= 1.25.0; switching to go1.25.10
  ... requires newer Go version go1.26" failure this session.
- **Step G case-against:** "go1.26 is brand new; x/vuln will catch up; transient."
  Why it fails: the machine PINS v1.3.0 explicitly, so the gap does not self-resolve,
  and it blocks all publishing on go1.26 right now.
- **Related prior retros:** None (first retro on this machine).

### 2. `generate --force` destroys hand-authored Phase 3 files; SKILL claims they survive (Skill instruction gap)
- **What happened:** After Phase 3 (7 hand-authored novel-command files in
  internal/cli/), re-running the Phase 2 command `generate --force` (to re-render the
  README/SKILL from a corrected research.json narrative) deleted all 8 hand-authored
  files. They had to be recreated from scratch.
- **Scorer correct?** N/A (not a score finding).
- **Root cause:** `--force` clears/overwrites the emitted directories, including
  hand-authored files the generator does not emit. The main SKILL's Phase 3 explicitly
  claims the opposite: "put the command body in its own internal/cli/<feature>.go file
  — it survives regen as a whole hand-authored unit" and frames only the root.go
  AddCommand call as wiped-by-force/restored-by-regen-merge. An agent following that
  guidance, then re-running the documented Phase 2 `generate --force`, loses real work.
- **Cross-API check:** Recurs on any CLI where the agent re-runs generate after
  hand-coding Phase 3 features — a common need (refresh README/narrative, retry a spec
  fix). Evidence: sumble lost 8 files this session.
- **Frequency:** most CLIs that iterate on the spec/narrative after Phase 3.
- **Fallback if not fixed:** Agent loses hand-authored work and must recreate it;
  worst case ships a CLI missing approved transcendence features.
- **Worth a fix?** Yes — either the behavior or the claim is wrong, and the
  contradiction is a trap.
- **Inherent or fixable:** Fixable. Options (disambiguate): (a) make `--force` preserve
  files it did not emit (only overwrite generator-owned files); (b) if --force must
  clean, correct the SKILL: Phase 2 should steer post-Phase-3 refreshes to
  `regen-merge` and Phase 3 must stop claiming hand-authored files survive --force.
- **Durable fix:** Component skill (primary): align the Phase 3 survival claim with
  reality and make Phase 2 recommend regen-merge for any regenerate-after-handcode.
  Optionally generator: --force preserves non-emitted files.
- **Test:** Generate, add internal/cli/<feature>.go, re-run `generate --force`; the
  hand-authored file should still exist (if fixing behavior), OR the SKILL no longer
  claims it survives and routes to regen-merge (if fixing docs).
- **Evidence:** `ls internal/cli/ | grep cost_estimate` returned nothing immediately
  after the second `generate --force` this session.
- **Step G case-against:** "Agent should have used regen-merge." Why it fails: the SKILL
  itself tells the agent these files survive regen and documents `generate --force` as
  the Phase 2 command — following the docs causes the loss.
- **Related prior retros:** None.

## Prioritized Improvements

### P1 — High priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|----------------------|------------|--------|
| 1 | govulncheck gate fails on go1.26 | scorer/generator | every go1.26 user | Low (agent must bypass a security gate) | medium | degrade only on detected toolchain-vs-analyzer skew; never silently disable on supported toolchains |

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|----------------------|------------|--------|
| 2 | --force destroys hand-authored Phase 3 files vs SKILL claim | skill | most iterating CLIs | Medium | small | n/a |

### Skip
| Finding | Title | Why it didn't make it |
|---------|-------|------------------------|
| 3 | Nested request bodies emit as opaque JSON-string flags (--filters/--organization), scored MCP Tool Design 5/10 | Step B: only 1 API with evidence, and the exact flatten-fallback trigger (array-typed leaf fields? oneOf?) was not isolated; generator demonstrably flattens simpler nested objects. Needs diagnosis + cross-API evidence before filing. |
| 4 | `lock promote` resolved a different scope's runstate with empty RunID, shipping manifest novel_features=0 | Step B: single-environment evidence, specific to a multi-worktree setup (two working dirs present); impact recoverable (manifest patched, README/SKILL synced correctly by dogfood). Re-surface with multi-environment evidence. |

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| `jobs` reserved-name collision | Resource named `jobs` collided with the built-in async-jobs command; renamed to `postings` | working-as-designed: the parser emitted a clear error and suggested the exact fix |

## Work Units

### WU-1: govulncheck gate degrades gracefully on toolchain-vs-analyzer version skew (from F1)
- **Priority:** P1
- **Component:** scorer
- **Goal:** The govulncheck quality gate must not hard-fail with opaque "requires newer
  Go version" output when the host Go toolchain is newer than the pinned govulncheck can
  analyze; it should WARN with a clear, actionable message and keep blocking on real
  reachable vulns when the analyzer is compatible.
- **Target:** The govulncheck invocation in generate's validation gates and in
  `publish validate` / `publish package`.
- **Acceptance criteria:**
  - positive: on a go1.26 host with govulncheck@v1.3.0, the gate emits a WARN naming the
    toolchain/analyzer skew and does not hard-fail generation or publish validation.
  - negative: on a host where govulncheck can analyze the module, the gate still runs and
    fails on a genuinely reachable vulnerability (not silently disabled).
- **Scope boundary:** Does not change the emitted go.mod go-directive policy; does not
  disable govulncheck on supported toolchains.
- **Dependencies:** None. (Bumping the govulncheck pin to a go1.26-capable release is a
  complementary follow-up once one exists.)
- **Complexity:** medium

### WU-2: Reconcile `--force` behavior with the SKILL's hand-authored-file survival claim (from F2)
- **Priority:** P2
- **Component:** skill
- **Goal:** Eliminate the trap where following the SKILL (hand-author novel commands in
  their own files, then re-run the documented Phase 2 `generate --force`) destroys that
  work. Either preserve non-emitted files across --force, or correct the SKILL to route
  post-Phase-3 regenerations through regen-merge and stop claiming the files survive.
- **Target:** `skills/printing-press/SKILL.md` Phase 2 (generate command) and Phase 3
  (hand-authored-file survival claim); optionally `internal/generator/` --force semantics.
- **Acceptance criteria:**
  - positive: an agent following the SKILL to refresh README/narrative after Phase 3 does
    not lose hand-authored internal/cli/<feature>.go files.
  - negative: routine first-time generation behavior is unchanged.
- **Scope boundary:** Does not change regen-merge's existing AddCommand re-injection.
- **Dependencies:** None.
- **Complexity:** small

## Anti-patterns
- None observed in the agent workflow worth flagging beyond the --force trap above.

## What the Printing Press Got Right
- The reserved-name collision (`jobs`) produced a precise error with the exact fix
  ("Rename the resource — e.g. sumble_jobs"). Zero guesswork.
- Nested request bodies, though emitted as JSON-string flags, assembled the correct
  nested wire body (verified via --dry-run) — functionally correct, just not ergonomic.
- narrative.headline correctly drove the CLI description across root.go/SKILL/goreleaser,
  avoiding the raw-API-blob failure mode.
- dogfood's novel_features_check (planned 7 / found 7) gave a deterministic gate that the
  hand-authored transcendence features were actually wired in.
- scorecard's --live-check sampling caught a real behavioral bug (stack-diff enrich 422)
  that structural checks missed — and it cost zero credits because the call 422'd.
