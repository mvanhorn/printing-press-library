# Printing Press Retro: Splitwise (reprint)

## Session Stats
- API: splitwise (REPRINT of the v4.16.0 build onto v4.19.0)
- Spec source: catalog/vendor OpenAPI 3.0.1 (`splitwise/api-docs`), reused from prior run
- Scorecard: 91/100 (A)
- Verify pass rate: 100%
- Fix loops: 0 (shipcheck green first pass)
- Manual code edits: port of 6 prior novel files + root.go rewire + 2 codex-built commands (split, recurring)
- Features built from scratch: 2 new (split, recurring); 6 ported verbatim; 1 framework (search)

This was a near-clean reprint. The findings are all about the **reprint path itself** (the machine-upgrade flow), not the printed CLI, which is healthy.

## Findings

### 1. Phase 5.6 promote routing misfires on from-scratch reprints (skill instruction gap)
- **What happened:** Phase 5.6 routes promotion to Path B (`regen-merge`) whenever the library manifest has `novel_features > 0`. On this reprint, the `regen-merge` dry-run flagged 35 files review-required (29 TEMPLATED-BODY-DRIFT + 4 TEMPLATED-VALUE-DRIFT + 2 TEMPLATED-WITH-ADDITIONS). Every one of those was **4.16→4.19 framework version drift in GENERATED files** (`promoted_*.go`, `sync.go`, `store.go`, `mcp/tools.go`) — not hand-edits. The 2 WITH-ADDITIONS were stale 4.16.0 helpers 4.19.0 dropped (`extractResponseData`). The Phase 5.6 halt condition (`NEEDS_REVIEW > 0`) would have blocked promotion outright.
- **Scorer correct?** N/A (not a score penalty).
- **Root cause:** `skills/printing-press/SKILL.md:4104-4146`. The `NOVEL_COUNT>0 → Path B` rule assumes the fresh tree LACKS the library's novels (pure regen). For a reprint that REBUILDS all novels into the fresh tree, that premise is false: the library has nothing unique to preserve, and `regen-merge` cannot distinguish older-generator drift from hand-edits, so it treats expected version drift as review-required.
- **Cross-API check:** Recurs on every cross-version reprint of a novel-feature CLI. Named with evidence: dice-fm (older binary, novel GraphQL layer), streeteasy (v4.16.0, 16 hand-coded features), splitwise (this run, v4.16.0). Each produces generated-file version drift on reprint onto a newer binary.
- **Frequency:** every cross-version reprint of a CLI with novel features.
- **Fallback if unfixed:** The agent must (a) know to run the regen-merge dry-run (Phase 5.6 mentions it but routes mechanically), (b) recognize the drift is version-drift not hand-edits, and (c) override to Path A swap. An agent following Phase 5.6 mechanically hits the 35-file halt with no guidance and either stalls or, worse, applies regen-merge and preserves stale 4.16.0 framework code.
- **Worth a fix?** Yes — reprints are the high-value machine-upgrade path; the routing should handle them as a first-class case.
- **Durable fix:** Add a reprint-aware branch to Phase 5.6 (skill): when the run is a reprint whose fresh tree reimplements all `novel_features` (novel files present in the fresh tree, classified TEMPLATED-CLEAN/NEW-TEMPLATE-EMISSION by a dry-run), treat generated-file BODY/VALUE drift as expected overwrite and prefer Path A swap (or give `regen-merge` a "treat templated drift as overwrite, only halt on NOVEL-COLLISION" mode). Gate strictly on "fresh tree contains the novels" so it never clobbers a library whose novels are absent from the fresh tree.
- **Test:** positive — reprint a novel CLI built on an older binary; the dry-run shows only generated-file drift + clean novels; promotion proceeds via swap without a manual override. negative — a pure regen that does NOT rebuild novels still routes to Path B and preserves the library's hand-authored files.
- **Evidence:** regen-merge dry-run this run: 35 review-required, all version drift; novel files TEMPLATED-CLEAN; only stale-helper additions. Agent overrode to Path A swap by judgment.
- **Related prior retros:** None for this finding. Adjacent open issues: #2411 (reprint module-path imports), #2181 (library patch clobbered on reprint) — both `related-area` (reprint preservation), different fix.
- **Case-against (Step G):** "Agent escaped via judgment; the skill says 'orchestration must honor preservation,' which is a judgment hook; regen-merge's halt is conservative-correct." Why it fails: the routing is MECHANICAL and the halt is a guaranteed false-positive on every cross-version reprint — the skill gives no branch for "version drift is expected," so the conservative halt becomes a reliable blocker, not a safety net.

### 2. Pinned gosec (@v2.21.4) is incompatible with Go 1.26.3 (skill / quality-gate tooling)
- **What happened:** During Phase 5.5 polish, the pinned `go run github.com/securego/gosec/v2/cmd/gosec@v2.21.4 ./...` failed to compile under Go 1.26.3 (`tokeninternal` array-length error). The failing invocation also silently resolved against a stale prior library copy rather than the working tree. Polish recovered by switching to `gosec@latest`.
- **Scorer correct?** N/A (tooling failure, not a score).
- **Root cause:** `skills/printing-press-polish/SKILL.md:339` and `:691` pin `gosec@v2.21.4` as the no-install fallback. That release predates Go 1.26 and does not compile against its `go/token` internals.
- **Cross-API check:** Toolchain-universal — affects EVERY polish run on Go 1.26.3 that lacks a separately-installed gosec binary, regardless of API. (Go 1.26.3 is the current stable toolchain on this machine.)
- **Frequency:** every polish run on Go 1.26.x without a pre-installed gosec.
- **Fallback if unfixed:** The polish SKILL documents "treat the missing scan as a polish failure → set ship_recommendation: hold." So the literal documented path on current Go is a forced hold (or, as happened here, a silent scan of the wrong directory). Either way the gosec leg is broken until the pin is bumped.
- **Worth a fix?** Yes — one-line pin bump; current and universal breakage.
- **Durable fix:** Bump the pinned gosec to a Go-1.26-compatible release at both call sites in the polish SKILL (and any sibling reference). Optionally float to a recent minor instead of a hard patch pin. Separately, the silent-fallback-to-stale-dir behavior suggests the `go run ... ./...` is sometimes invoked from the wrong cwd — worth confirming the polish gosec step pins cwd to the target CLI dir.
- **Test:** positive — on Go 1.26.x with no installed gosec, the polish gosec leg compiles and writes JSON. negative — an installed gosec binary still takes precedence over the pinned fallback.
- **Evidence:** polish result block this run: "pinned gosec@v2.21.4 fails to compile under Go 1.26.3 (tokeninternal array-length error) and silently resolved a stale prior library copy; switched to gosec@latest."
- **Related prior retros:** None.
- **Case-against (Step G):** "Polish already handles tool failure (hold); gosec will eventually ship a Go-1.26-compatible release; the agent worked around it." Why it fails: the pin is broken on the CURRENT stable Go right now, the SKILL actively instructs using the broken pin, and the failure mode here was a silent wrong-directory scan, not a clean hold — a one-line bump removes a live, universal breakage.

### 3. get-currencies code-keyed rows dropped on sync — RECURS (reinforces open #2327)
- **What happened:** `sync` consumed 153 currencies but stored 0 (`all_items_failed_id_extraction`) because currencies are keyed by `currency_code`, not a numeric `id`. Offline currency lookup is therefore empty; live unaffected.
- **Scorer correct?** N/A.
- **Root cause:** generic resource id-extraction requires an `id`-shaped field; code-keyed resources extract nothing and all rows drop. This is exactly open issue **#2327** ("generator: generic resource id-extraction silently drops code-keyed resources (stored 0 of N)").
- **Disposition:** Do NOT file new. Comment on #2327 with the splitwise/get-currencies recurrence as additional cross-CLI evidence (153→0).
- **Related prior retros:** None in manuscripts; tracked as open issue #2327.

## Prioritized Improvements

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|----------------------|------------|--------|
| 1 | Phase 5.6 promote routing misfires on from-scratch reprints | skill | every cross-version reprint of a novel CLI | low (agent must know to dry-run + override) | medium | gate on "fresh tree contains the novels" |
| 2 | Pinned gosec @v2.21.4 incompatible with Go 1.26.3 | skill | every polish run on Go 1.26.x w/o installed gosec | medium (documented path is hold) | small | prefer installed gosec; bump pin |

### Comment (existing issue)
| Finding | Title | Plan |
|---------|-------|------|
| 3 | get-currencies code-keyed rows dropped (153→0) | Comment on #2327 |

### Skip
| Finding | Title | Why it didn't make it |
|---------|-------|------------------------|
| — | Dead `partialFailureErr` in generated helpers.go | Step G: minor dead-code (dead_code 4/5, not blocking); the partial-failure subsystem is intentionally emitted unconditionally for CLIs that need it; conditional emission adds template complexity for a sub-point penalty. |

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| sync caps expenses at ~100 of 638 | Default --max-pages bounds the page count | printed-CLI (tunable via --max-pages; by-design bounded sync) |
| novel-stub naming collision (newNovel* vs newXCmd) on reprint | Generated stubs collide with older hand-authored novel impls | transitional — new prints use the convention natively; population shrinks as library refreshes; folded as context into Finding 1 |
| incremental sync returned 0 new expenses | Cursor was current from prior run | API-quirk/by-design (correct incremental behavior; the DB-sharing is operator-side) |

## Work Units

### WU-1: Reprint-aware Phase 5.6 promotion routing (from F1)
- **Priority:** P2
- **Component:** skill
- **Goal:** Phase 5.6 promotes a from-scratch reprint (fresh tree reimplements all novels) via Path A swap without halting on expected cross-version generated-file drift.
- **Target:** `skills/printing-press/SKILL.md` Phase 5.6 promote-path section (~lines 4104-4146).
- **Acceptance criteria:**
  - positive: reprinting a novel CLI built on an older binary proceeds to promote without a manual Path-A override; generated-file BODY/VALUE drift is treated as expected overwrite.
  - negative: a pure regen that does not rebuild the novels still routes to Path B (regen-merge) and preserves the library's hand-authored files; a genuine NOVEL-COLLISION still halts.
- **Scope boundary:** does not change regen-merge's classification engine beyond (optionally) a "treat templated drift as overwrite" mode; does not touch fresh-print routing.
- **Dependencies:** none.
- **Complexity:** medium

### WU-2: Bump pinned gosec for Go 1.26 compatibility (from F2)
- **Priority:** P2
- **Component:** skill
- **Goal:** The polish gosec fallback compiles and runs on Go 1.26.x.
- **Target:** `skills/printing-press-polish/SKILL.md:339` and `:691` (pinned `gosec@v2.21.4`).
- **Acceptance criteria:**
  - positive: on Go 1.26.x with no installed gosec, the pinned `go run` gosec leg compiles and writes JSON.
  - negative: an installed gosec binary still takes precedence; the pin only applies as fallback.
- **Scope boundary:** version bump (+ optional cwd-pinning of the scan); not a rewrite of the gosec leg.
- **Dependencies:** none.
- **Complexity:** small

## Anti-patterns
- None observed in the printed CLI. The reprint friction was all in the promotion/tooling flow, not the generated output.

## What the Printing Press Got Right
- **agentcookie soft integration (#2333) landed exactly as intended** — the reprint emitted a clean go.mod with no private require and no replace directive, natively resolving the publish blocker that a hand-migration had papered over. This is the model case for "reprint to capture a machine fix."
- **#2350 numeric-id store fix** keeps resource keys clean (108 distinct friend IDs, no dup-accumulation).
- **Verbatim port of the 6 prior novel files compiled clean against v4.19.0** — framework helper signatures stayed stable across 4.16→4.19, so proven code ported without adaptation.
- **Codex produced correct, well-gated new commands first try** — split's cent-allocation sums exactly; --record gating (print-by-default + IsVerifyEnv short-circuit) was right without correction.
