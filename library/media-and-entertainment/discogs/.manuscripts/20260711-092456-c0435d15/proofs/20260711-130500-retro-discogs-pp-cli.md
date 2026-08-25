# Printing Press Retro: Discogs

## Session Stats
- API: discogs (music database / marketplace / collection / wantlist)
- Spec source: hand-authored internal YAML (no trustworthy OpenAPI exists; grounded from the Discogs Postman collection + cswkim MCP TOOLS.md)
- Scorecard: 91/100 (A)
- Verify pass rate: 100%
- Fix loops: 2 (shipcheck) + polish
- Manual code edits: substantial (7 hand-built transcendence commands + sync/limit + store migrations — expected for a local-mirror CLI)
- Features built from scratch: 7 novel commands (fills, portfolio, undervalued, comps, sell-plan, identify, pressings) + sync + limit

## Findings

### 1. Live-dogfood happy_path/json_fidelity ignores `pp:typed-exit-codes` (scorer bug)
- **What happened:** `dogfood --live --level full` fails commands that correctly return a declared typed non-zero exit. All four generated learn-loop commands (`teach`, `teach-pattern`, `teach-playbook`, `playbook amend`) fail happy_path + json_fidelity with exit 2 — despite carrying `pp:typed-exit-codes: "0,2"` that the generator's own authors added with an explicit code comment ("declare it so `cli-printing-press verify` scores the Execute cell honestly"). Adding `pp:typed-exit-codes: "0,3"` to get-by-id endpoint commands (inventory export-get/download, upload-get) that correctly return 404→exit 3 for a synthesized nonexistent id also did NOT make happy_path pass.
- **Scorer correct?** No — scorer bug. `verify` honors `pp:typed-exit-codes`; the live-dogfood matrix does not apply it to happy_path/json_fidelity. The commands behave correctly; the matrix scores them as failures.
- **Root cause:** scorer (dogfood live matrix). happy_path/json_fidelity treat any non-zero exit as failure without consulting the command's `pp:typed-exit-codes` annotation (which `verify` already reads).
- **Cross-API check:** Recurs on **every printed CLI**. The learn loop (teach/recall/playbook family) ships in every print and every one of those commands is intentionally exit-2-on-bare-invocation with `pp:typed-exit-codes: "0,2"`. So every learn-loop CLI carries ≥8 spurious full-dogfood failures. Additionally hits any CLI with get-by-id endpoint commands (extremely common) whenever the matrix probes a synthesized id and the API 404s.
- **Frequency:** every API.
- **Fallback if the Printing Press doesn't fix it:** the agent must hand-diagnose ~8–14 "failures" per run, confirm they're non-defects, and either patch generated framework files (fighting the authors' deliberate design) or fall back to `--level quick` for the acceptance marker. High per-run friction; easy to mis-ship (an agent may band-aid generated files or, worse, force fake args).
- **Worth a Printing Press fix?** Yes. Named cross-CLI evidence (below) is concrete; it's a scorer bug, not a per-CLI quirk.
- **Inherent or fixable:** fixable. The matrix already has the annotation available (verify reads it).
- **Durable fix:** In the live-dogfood matrix, when a command exits non-zero on happy_path/json_fidelity, consult its `pp:typed-exit-codes` annotation (same lookup `verify` uses) and treat a declared code as a pass. This is exactly the mechanism the generator authors assumed existed.
- **Test:** positive — a command annotated `pp:typed-exit-codes:"0,2"` that exits 2 on happy_path scores PASS; negative — a command with no such annotation that exits 2 still scores FAIL.
- **Evidence:** `teach.go:179` annotation + its code comment; empirically, adding `0,3` to the 3 inventory commands did not flip their happy_path (this run, full dogfood 179/194 both before and after the annotation).
- **Named APIs with evidence:** every printed CLI in `$PRESS_LIBRARY` (the learn loop is universal — e.g. `snipes` and any other printed CLI carry the identical teach/playbook commands with `pp:typed-exit-codes:"0,2"`); plus get-by-id: discogs (inventory export-get), and any CLI with `GET /resource/{id}` endpoints (the overwhelming majority).
- **Case-against (Step G):** "Maybe the matrix intentionally ignores typed-exit on happy_path because a happy path should return data (exit 0)." Rebuttal: the generator authors explicitly annotated these commands expecting the annotation to be honored (the code comment names `verify`); the inconsistency between `verify` (honors) and `dogfood` (ignores) is the bug — one scorer contradicts the other on the same annotation. Case-for clearly stronger.
- **Related prior retros:** None (first retro on this machine).

### 2. gosec false-positives ship in generated templates (generator)
- **What happened:** `gosec ./...` reports 17 findings on a freshly generated CLI, 100% in generated framework files, 0 in hand-authored code. All are false-positives: G304 (×8, reading config-resolved / CLI-arg file paths), G201/G202 (×3, internal SQL where only compile-time-constant identifiers are interpolated and values are parameterized), G119 (client redirect policy that *deliberately* strips `Authorization` on cross-host hops), G104/G117 (×5, best-effort deferred ops + a JSON key-name nit).
- **Scorer correct?** N/A (gosec is external, not a PP scorer). The findings are real gosec output but false-positive in context.
- **Root cause:** generator templates (`internal/generator/`) emit these constructs without `#nosec` justifications.
- **Cross-API check:** Recurs on **every printed CLI** — the flagged files (`internal/client/client.go`, `internal/store/store.go`, `internal/learn/*.go`, `internal/config/config.go`, `internal/cli/teach*.go`, `feedback.go`) are all framework files emitted identically in every print.
- **Frequency:** every API.
- **Fallback if the Printing Press doesn't fix it:** each CLI that wants a gosec-clean artifact must hand-add 17 `#nosec` suppressions (documented patches that revert on regen). Recurring, mechanical, fragile.
- **Worth a Printing Press fix?** Yes — universal, mechanical, generator-owned.
- **Inherent or fixable:** fixable. Emit each construct with a justified inline `#nosec <RULE> -- <reason>` in the template.
- **Durable fix:** add justified `#nosec` comments to the generator templates for the recurring false-positive sites (config/CLI file reads, internal constant-identifier SQL, the deliberate auth-strip redirect handler, best-effort deferred ops). Printed CLIs then ship gosec-clean.
- **Test:** positive — `gosec ./...` on a freshly generated CLI reports 0 issues; negative — a genuinely unsafe construct (user-tainted SQL value) is NOT blanket-suppressed.
- **Evidence:** this run — 17 findings, all generated-file, suppressed with justified `#nosec` (documented patch) to reach gosec 0.
- **Named APIs with evidence:** every printed CLI (framework files are byte-identical templates); e.g. `snipes` and any other print carry the same `internal/client/client.go` redirect handler + `internal/store/store.go` SQL sites.
- **Case-against (Step G):** "gosec isn't a ship gate (verify uses govulncheck), and these are false-positives, so who cares." Rebuttal: the value is eliminating a recurring 17-edit manual suppression pass on every CLI whose author wants gosec-clean; a one-time template change removes it forever. Case-for stronger for a mechanical universal fix, though lower urgency than F1 → P2.
- **Related prior retros:** None.

## Prioritized Improvements

### P1 — High priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F1 | Live-dogfood happy_path/json_fidelity must honor `pp:typed-exit-codes` | scorer | every API | Low (agent must hand-diagnose 8–14 non-defect failures each run) | small | Only treat *declared* codes as pass; undeclared non-zero still fails |

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F2 | Emit justified `#nosec` in generated templates for recurring false-positives | generator | every API | Medium (agent can suppress, but 17 fragile edits per CLI) | small | Suppress only the enumerated false-positive sites; never blanket-suppress user-tainted input |

### Skip
| Finding | Title | Why it didn't make it |
|---------|-------|------------------------|
| S1 | Syncable-resource profiler skips path-param-required resources (username/org/project-scoped), so no `sync`/`sql`/`search` framework commands are emitted for user-scoped APIs | Step B: only 1 API with concrete evidence (discogs). Plausible for GitHub/Asana/Linear-style scoped lists but not verified against their specs this run; re-file if a second user-scoped CLI hits it. |

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| pressings `major_formats` type mismatch | Novel command typed `major_formats` as object; API returns array → silent empty | printed-CLI (my hand-authored novel command; fixed by polish; not a generator template) |
| inventory get-by-id 404 happy_path | export-get/download/upload-get fail happy_path with exit 3 | API-quirk / subset of F1 (same root: matrix ignores typed-exit-codes) |
| sync uses generic Upsert only | dogfood WARN on data-pipeline/sync dims | printed-CLI (by-design on-demand snapshot mirror; no conventional sync command) |

## Work Units

### WU-1: Honor `pp:typed-exit-codes` in the live-dogfood happy_path/json_fidelity checks (from F1)
- **Priority:** P1
- **Component:** scorer
- **Goal:** The live-dogfood matrix treats a command's declared typed exit codes as a pass on happy_path/json_fidelity, matching `verify`'s existing behavior.
- **Target:** dogfood live matrix (the happy_path/json_fidelity exit-code classifier).
- **Acceptance criteria:**
  - positive: a command annotated `pp:typed-exit-codes:"0,2"` that exits 2 on happy_path scores PASS; a learn-loop CLI's full dogfood no longer counts teach/teach-pattern/teach-playbook/playbook-amend as failures.
  - negative: a command with no `pp:typed-exit-codes` that exits 2 still scores FAIL.
- **Scope boundary:** Does not change `verify` (already correct); does not touch error_path handling.
- **Dependencies:** none.
- **Complexity:** small.

### WU-2: Ship gosec-clean templates via justified `#nosec` (from F2)
- **Priority:** P2
- **Component:** generator
- **Goal:** A freshly generated CLI reports `gosec ./...` = 0 without per-CLI suppression patches.
- **Target:** generator templates for `internal/client/client.go`, `internal/store/store.go`, `internal/store/learnings.go`, `internal/learn/*.go`, `internal/config/config.go`, `internal/cli/teach*.go`, `internal/cli/feedback.go`.
- **Acceptance criteria:**
  - positive: `gosec ./...` on a fresh print reports Issues: 0; each suppression carries a justification comment.
  - negative: a template that interpolates a user-tainted value into SQL is NOT suppressed.
- **Scope boundary:** Only the enumerated recurring false-positive sites; no blanket file-level `#nosec`.
- **Dependencies:** none.
- **Complexity:** small.

## Anti-patterns
- Band-aiding generated framework files (adding `#nosec` / annotations locally) to satisfy a scorer — reverts on regen and, for F1, would tempt an agent to force fake args that fight the authors' deliberate exit-2 design. Both findings route the fix to the machine instead.

## What the Printing Press Got Right
- The hand-authored internal-YAML path handled a 54-endpoint API with custom auth (`Discogs token={token}` format template), a mandatory `User-Agent` (`required_headers`), and per-condition learn seeds cleanly — generation passed all Go gates first try.
- Polish caught a real silent-failure bug (`pressings` type mismatch) via live-check sampling that shipcheck's structural pass missed — the live-check layer earned its keep.
- The novel-command scaffolds + shared helper set (`printJSONFiltered`, `boundCtx`, `dryRunOK`, NULL-safe patterns) made the 7 hand-built transcendence commands fast to author and clean on review (0 hand-authored gosec/vet findings).
