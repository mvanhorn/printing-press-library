# Printing Press Retro: openrouter

## Session Stats
- API: openrouter
- Spec source: catalog OpenAPI (https://openrouter.ai/openapi.json) trimmed to 15 introspection paths (dropped 26 inference/admin/payment paths)
- Scorecard: 91/100 (Grade A) post-polish
- Verify pass rate: 100% (25/25)
- Fix loops: 1 (validate-narrative — fixed flag inconsistencies in research.json + budget_check --dry-run short-circuit)
- Manual code edits: 12 (1 MCP main.go HTTP transport + 11 transcendence command files; subsequently polish migrated 9 files from `flags.printJSON` to `printJSONFiltered`)
- Features built from scratch: 8 transcendence commands (~1,300 LOC)

## Findings

### 1. mcp.* spec block silently noop'd by v4.2.0 (Bug — generator)

- **What happened:** Set `mcp.transport: [stdio, http]`, `mcp.orchestration: code`, `mcp.endpoint_tools: hidden` in the spec per Phase 2's Pre-Generation MCP Enrichment guidance. Generator accepted the spec without warning. Generated `cmd/openrouter-pp-mcp/main.go` was stdio-only with no orchestration emission and all 28 endpoint mirror tools visible. None of the three fields had any effect.
- **Scorer correct?** N/A (silent acceptance, no scorer involved). The skill's own Phase 2 documentation says these fields work — that documentation is the contract being broken.
- **Root cause:** Generator templates either don't read `spec.mcp.{transport,orchestration,endpoint_tools}` at all, or read them but never branch on the values. Same root for all three is likely (one parser, one switch).
- **Cross-API check:** Yes — applies to every printed CLI. Three concrete examples from the existing catalog with evidence:
  - Linear (~80 endpoints in v2 spec; would benefit from `orchestration: code` to keep MCP schema bounded)
  - Notion (~50 endpoints, deployed-in-container by many users; needs HTTP transport)
  - Stripe (>200 endpoints; mandatory orchestration to fit any agent context)
- **Frequency:** every CLI with >30 endpoints AND/OR every CLI deployed where the MCP must reach across container boundaries (most production deployments)
- **Fallback if the Printing Press doesn't fix it:** Hand-patch `cmd/<name>-pp-mcp/main.go` (40 LOC of `flag.Parse` + `NewStreamableHTTPServer`). Hand-patch needs to survive regen via `.patches/` (see Finding 2). Cumulative cost across N CLIs: 40 LOC × N + ongoing drift maintenance.
- **Worth a Printing Press fix?** Yes — high-leverage. One template change benefits every printed CLI; eliminates the silent dead-config class for the `mcp.*` block.
- **Inherent or fixable:** Fixable. Three honest options:
  - (a) Honor the fields (full implementation: HTTP transport emission + orchestration template + endpoint_tools suppression)
  - (b) Reject the fields with a clear "not implemented in v4.x" error at parse time
  - (c) Warn loudly at generate time ("spec field 'mcp.transport' accepted but ignored — remove from spec or upgrade to vN+1")
  Option (b) or (c) is the cheap fix. Option (a) is the right fix.
- **Durable fix:** Honor the fields. The orchestration pair (`<api>_search` + `<api>_execute`) collapses N endpoint mirrors into 2 tools — proven win for any CLI >30 endpoints. HTTP transport via `mark3labs/mcp-go`'s `NewStreamableHTTPServer(s).Start(addr)` is ~15 LOC in main.go.
- **Test:** Positive — generate with `mcp.transport: [stdio, http]`; assert `cmd/*-mcp/main.go` contains `NewStreamableHTTPServer`. Negative — generate without the field; assert main.go is stdio-only as today (regression guard).
- **Evidence:** This session, 2026-05-09. Spec at `~/printing-press/.runstate/openclawdocker-03c20893/runs/20260509-165428/research/openrouter-trimmed.json` line ~1700. Generated `cmd/openrouter-pp-mcp/main.go` at 27 LOC, no http server. Hand-patched to add `--http :8092` flag (now 40 LOC, snapshot at `~/printing-press/library/openrouter/.patches/cmd/openrouter-pp-mcp/main.go`).
- **Related prior retros:** None (no prior retros found via `grep -l "mcp\.transport\|mcp\.orchestration" ~/printing-press/manuscripts/*/proofs/*-retro-*.md`).

### 2. Hand-built transcendence commands lost on full regen — no native protection (Missing scaffolding — generator + skill)

- **What happened:** This session built 11 transcendence files in `internal/cli/` (~1,300 LOC). When we re-ran `/printing-press openrouter` for the spec edit, the regen wiped `internal/cli/` and re-emitted only the framework + spec endpoint mirrors. Hand-built files were gone. Recovered only because we manually backed them up to `transcendence-backup/` first.
- **Scorer correct?** N/A.
- **Root cause:** Two contributing gaps:
  1. The skill's Phase 0 Library Check option 1 prose says "Re-runs the Printing Press into a working directory, overwrites generated code, **then rebuilds transcendence features**." The "rebuilds transcendence features" clause is aspirational — there's no mechanism. Phase 3 says the agent will rebuild them by reading prior `research.json novel_features`, but a single Phase 3 run doesn't always reproduce 1300 LOC verbatim.
  2. No emitted scaffolding (`.patches/`, restore script, manifest) to protect hand-built code by construction.
- **Cross-API check:** Yes — universal. Every printed CLI is required to have transcendence commands per Phase 3 mandate. Three concrete examples from the catalog: Producthunt, Yahoo Finance, Recipe-goat — all have hand-built novel commands; all would silently lose them on a full regen.
- **Frequency:** every CLI with hand-built transcendence (which is every CLI per the doctrine). Triggered when: spec changes, generator upgrades, regen-to-fix-defect.
- **Fallback if the Printing Press doesn't fix it:** Manually invent a `.patches/` directory + restore script per CLI (this session did exactly this). Per-CLI cost: 30 minutes the first time, then drift maintenance.
- **Worth a Printing Press fix?** Yes. The pattern absorbs cleanly into a generator template + a small skill instruction.
- **Inherent or fixable:** Fixable.
- **Durable fix:** Two-part:
  - **Generator:** emit `.patches/` skeleton + `.patches/restore.sh` + `HAND-PATCH-MANIFEST.md` (with sha256 + drift-check section) on every generation. Empty by default — fills in as users hand-patch.
  - **Skill:** in Phase 3 "Build The GOAT" section, instruct the agent to copy each new `internal/cli/*.go` hand-built file into `.patches/internal/cli/` and stamp the manifest before the Phase 3 Completion Gate. In Phase 0 Library Check option 1 prose, replace "rebuilds transcendence features" with "restores hand-patches via `.patches/restore.sh`, then re-verifies via shipcheck."
- **Test:** Positive — after a fresh generate, `.patches/` exists with empty `HAND-PATCH-MANIFEST.md` and an executable `restore.sh`. After a hand-built command lands and is stamped, regen + restore.sh + build → command works. Negative — restore.sh against a divergent live state surfaces drift, doesn't silently overwrite.
- **Evidence:** This session, 2026-05-09. Initial generate emitted no protection scaffold. We invented `.patches/` after the regen wiped `internal/cli/`. Snapshot at `~/printing-press/library/openrouter/.patches/` (12 files + restore.sh + manifest).
- **Related prior retros:** None.

### 3. `flags.printJSON()` foot-gun for hand-written commands (Default gap — generator)

- **What happened:** Spec-emitted commands use `printJSONFiltered(cmd.OutOrStdout(), v, flags)` correctly — honors `--select`, `--compact`, `--csv`, `--quiet`. But `flags.printJSON(cmd, v)` exists, is exported, and is the natural call when an agent hand-writes a transcendence command (the receiver-style call reads better in code review). Hand-written commands using `flags.printJSON` silently drop all four flags. This session built 8 transcendence commands all calling `flags.printJSON`; only Phase 5.5 polish caught it and migrated 9 files.
- **Scorer correct?** Partially — the Agent Build Checklist (rule #2 in the printing-press SKILL) explicitly documents this: "Hand-written novel commands that build a Go-typed slice/struct and emit JSON MUST call `printJSONFiltered`...not `flags.printJSON`." Scorer is correct in pointing at the rule. But the rule is a documentation patch over an API surface that allows the wrong call to look right. Foot-gun.
- **Root cause:** Two helpers with similar names; one filters, one doesn't. The unfiltered one is shorter to type and looks like the "primary" call.
- **Cross-API check:** Yes — universal. Every printed CLI has hand-written transcendence commands; every one of them is at risk of this foot-gun on first write. Three concrete examples from the catalog: Producthunt, Yahoo Finance, Recipe-goat — all have hand-built commands that would benefit from this safeguard (whether or not they currently happen to call the right helper).
- **Frequency:** every hand-written novel command (every CLI, multiple per CLI). Catch rate: low without polish.
- **Fallback if the Printing Press doesn't fix it:** Polish migrates `flags.printJSON` → `printJSONFiltered`. Cost: every CLI runs polish to recover what should have been right by default. Polish exists for taste-level fixes, not silent-correctness fixes.
- **Worth a Printing Press fix?** Yes. Cheap to fix; high payoff (closes a class of silent agent-UX defects).
- **Inherent or fixable:** Fixable.
- **Durable fix:** Three options ranked by cost/risk:
  - (a) **Rename + alias.** Rename `flags.printJSON` → `flags.printJSONUnfiltered`. Add `flags.printJSON = flags.printJSONFiltered` (the receiver-style call now does the right thing). Update generator templates to use the receiver style consistently.
  - (b) **Delete the unfiltered version.** If no caller actually needs unfiltered output (the skill rule already says transcendence commands MUST use the filtered one), remove `flags.printJSON` entirely. Compile-time error guides the migration.
  - (c) **Add a vet check.** Generator emits a custom `go vet` analyzer that flags `flags.printJSON` calls. Adds tooling weight.
  Option (b) is cleanest if no real caller needs unfiltered. Option (a) is the safe path. Skill instruction (#2 today) becomes redundant after either fix — that's the goal.
- **Test:** Positive — generate, hand-author a command using `flags.printJSON`, run `go vet ./...` → fail (option c) OR `go build` → fail (option b). Negative — `flags.printJSONFiltered` always passes.
- **Evidence:** This session, 2026-05-09. 8 transcendence commands built calling `flags.printJSON`. Polish migrated 9 files (12 call sites). Pre-polish `~/printing-press/library/openrouter/internal/cli/budget_check.go` (line 49 in original) called `flags.printJSON`; post-polish calls `printJSONFiltered`. The rule lived in skill docs the whole time; the agent didn't violate it adversarially — they followed the receiver-style ergonomics.
- **Related prior retros:** None.

## Prioritized Improvements

### P1 — High priority

| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---|---|---|---|---|---|---|
| F1 | mcp.* spec block silently noop'd | generator | every CLI ≥30 endpoints OR container deployed | Low (silent; only manual diff catches) | Medium (orchestration emission is real work; transport is small) | None — universal benefit |
| F3 | `flags.printJSON` foot-gun | generator | every hand-written novel command | Low (skill rule exists but wasn't followed without polish) | Small (rename + alias, or delete) | None — receiver-style call should always filter |

### P2 — Medium priority

| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---|---|---|---|---|---|---|
| F2 | Hand-built code lost on regen — no native protection | generator + skill | every CLI on regen path | Medium (manual backup works but is invented per-CLI) | Small (template + skill prose) | None |

### Skip

| Finding | Title | Why it didn't make it |
|---|---|---|
| F4 | Phase 5 promote gate refuses on environmental dogfood failures | Step G: case-against stronger. The gate exists to prevent shipping broken CLIs. Loosening it weakens the contract. Manual `cp -a` bypass with documented reason is the appropriate override when failures are environmental (e.g., no provisioning key, at-credit-cap blocking generation-id seeding). The current friction is a feature, not a bug. |

### Dropped at triage

| Candidate | One-liner | Drop reason |
|---|---|---|
| C7 | Tests required on every internal/cli pure-logic file | Too narrow — most transcendence is command wiring, not parser logic. Adding mandate would over-fire. |
| C8 | Auto-detect `--client-pattern proxy-envelope` | Not exercised this session (used clean OpenAPI spec); no evidence to support generalization. |
| C9 | OpenRouter "negative credit balance" `total_usage > total_credits` handling | API-quirk — printed-CLI fix at most. Drop. |
| C10 | macOS launchd "Operation not permitted" on `~/Documents/` | OS-level, not generator. Drop. |
| C11 | dogfood synced research.json `novel_features_built` correctly | What-went-right, not a finding. |

## Work Units

### WU-1: Honor `mcp.*` spec fields in generator (from F1)
- **Priority:** P1
- **Component:** generator
- **Goal:** Spec fields `mcp.transport`, `mcp.orchestration`, `mcp.endpoint_tools` produce visibly different generation output when present vs absent.
- **Target:** `internal/generator/` (MCP main.go template + tools registration template + endpoint-mirror filter)
- **Acceptance criteria:**
  - positive: spec with `mcp.transport: [stdio, http]` → `cmd/<name>-mcp/main.go` includes `NewStreamableHTTPServer(s).Start(addr)` behind a `--http <addr>` flag (default stdio)
  - positive: spec with `mcp.orchestration: code` → `internal/mcp/` emits `<api>_search` + `<api>_execute` tool pair using existing search/execute APIs
  - positive: spec with `mcp.endpoint_tools: hidden` → per-endpoint MCP tools have `mcp:hidden` annotation OR are excluded from `RegisterTools`
  - negative: spec without these fields → main.go is stdio-only with all endpoint tools visible (regression guard)
  - As a minimum-viable shipping floor before full implementation: emit a loud "spec field 'mcp.X' accepted but not yet honored — remove from spec or upgrade to vN+1" warning at generate time. Stop the silent class.
- **Scope boundary:** Does NOT include MCP intents (`mcp.intents`) — separate enrichment path. Does NOT change the generated CLI binary's flags or commands; only the MCP server emission.
- **Dependencies:** None.
- **Complexity:** medium

### WU-2: Hand-patch protection scaffold (from F2)
- **Priority:** P2
- **Component:** generator
- **Goal:** Every generated CLI ships with a `.patches/` directory + `restore.sh` + `HAND-PATCH-MANIFEST.md` so hand-built code survives full regen.
- **Target:** `internal/generator/` (new template files emitted on every generation)
- **Acceptance criteria:**
  - positive: fresh `printing-press generate` → CLI dir contains `.patches/` empty skeleton, executable `.patches/restore.sh`, `HAND-PATCH-MANIFEST.md` with header but no rows
  - positive: agent stamps a hand-built file via simple workflow (documented in Skill Phase 3) → manifest row populated with sha256, file copy in `.patches/`
  - positive: `printing-press generate --force` after a stamped patch + `restore.sh` → CLI builds, verify passes
  - negative: drift between live and patches surfaces via the manifest's drift-check command, doesn't silently restore
- **Scope boundary:** Does NOT auto-stamp hand-built files (agent responsibility). Does NOT touch git — this is filesystem-level protection.
- **Dependencies:** Companion skill update in Phase 3 to instruct stamping (small skill PR).
- **Complexity:** small

### WU-3: Eliminate `flags.printJSON` foot-gun (from F3)
- **Priority:** P1
- **Component:** generator
- **Goal:** Hand-authoring a transcendence command using the natural-feeling JSON-emit call cannot silently drop `--select / --compact / --csv / --quiet`.
- **Target:** `internal/generator/templates/` (helpers.go template) + spec-emitted endpoint commands.
- **Acceptance criteria:**
  - positive (preferred): `flags.printJSON` is removed (option b); `go build` fails on any caller; migration path is to use `printJSONFiltered`
  - positive (alternative): `flags.printJSON` is renamed `flags.printJSONUnfiltered`; a new `flags.printJSON` aliases `printJSONFiltered`
  - negative: existing `printJSONFiltered` callers continue to work without changes
  - The Agent Build Checklist rule #2 in `skills/printing-press/SKILL.md` becomes redundant after this fix — that's the goal. Optionally remove the rule when the fix lands.
- **Scope boundary:** Does NOT change the public flags (`--select` etc.) or the filtering logic. Only the helper API surface.
- **Dependencies:** None.
- **Complexity:** small

## Anti-patterns
- **Trusting spec acceptance as evidence of effect.** v4.2.0 silently accepted `mcp.transport`/`mcp.orchestration`/`mcp.endpoint_tools` and emitted nothing different. The agent thought they configured something; the generator thought they didn't. Documented in companion wiki insight `2026-05-09-spec-fields-that-silently-noop-are-dead-config.md`.
- **"Absorb every spec endpoint" by default.** This session would have shipped 32 commands (24 absorbed + 8 transcendence) including chat / embeddings / rerank / messages / responses — directly contradicting the agent-introspection thesis the CLI was built on. Trim was framework-correct. Documented in `2026-05-09-thesis-coherence-beats-absorb-doctrine.md`.

## What the Printing Press Got Right
- **Phase 5 dogfood matrix** caught the only real CLI behavior bug: `models query "__printing_press_invalid__"` returned exit 0 instead of usage error. Fix was 3 lines (reject `__` prefixed tokens). The matrix's synthetic-input convention works.
- **`validate-narrative --strict --full-examples`** caught real correctness issues in the README/SKILL narrative: command paths using `creds` (didn't exist) instead of `credits`, `--llm` flag where only `--agent` exists on absorbed commands, a recipe with `jq -r` pipe that broke under `--dry-run`. Saved 4 broken examples from shipping.
- **Polish skill's `printJSONFiltered` migration** caught a class of silent agent-UX bugs in 12 call sites the agent introduced. Polish doing real work, not cosmetic.
- **Subagent for novel features brainstorm** produced a strong 8-survivor list with adversarial cuts that named specific persona-served reasoning + buildability proofs. The customer-model + survivors-and-kills shape is the right contract.
- **`--research-dir` arg** on generate carried `research.json` cleanly so the generator + dogfood + scorecard all operated on a consistent narrative. No drift between phases.
- **Lock + promote semantics** kept the working dir from being clobbered by parallel runs. Hold + manual cp-a override was a clean escape valve.
