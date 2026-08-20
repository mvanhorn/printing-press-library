# Printing Press Retro: shopify

## Session Stats
- API: shopify
- Spec source: internal YAML (catalog), Shopify Admin GraphQL 2026-04
- Scorecard: 83/100 (Grade A)
- Verify pass rate: 100% (last shipcheck pass)
- Fix loops: 3 (narrative-cleanup → validate-narrative pass → bulk-operations restoration)
- Manual code edits: 4 files (`bulk_operations.go` restored, `orders_tag.go` + `shopifyql.go` + `root.go` wiring added post-promote)
- Features built from scratch: 4 (bulk-operations restoration, orders/customers tag mutations, ShopifyQL analytics, ShopifyQL funnel)
- Reprint baseline: prior CLI at press v3.2.1 (2026-05-02), reprinted under v4.6.1 with `mcp.transport: [stdio, http]` enrichment

## Findings

### 1. `extra_commands` rendered into SKILL.md as real commands but never emitted as Cobra code (Template gap)

- **What happened:** Spec contains `extra_commands: [{name: bulk-operations, description: ...}]`. The generator renders this into SKILL.md's "Command Reference" block as `shopify-pp-cli bulk-operations`, but emits no Cobra code for it. Verify-skill flagged the missing command and shipcheck failed.
- **Scorer correct?** Yes. Verify-skill correctly reported "command path not found in `internal/cli/*.go`". The bug is on the generator's emit side, not the scorer.
- **Root cause:** `internal/spec/spec.go` parses `extra_commands` into a struct field. Only `internal/generator/templates/skill.md.tmpl` consumes it (rendering rows under "Command Reference"). The Cobra emitter never reads the field. The spec-side contract is "this command exists"; the generator-side reality is "documentation-only." The two halves disagree.
- **Cross-API check:** Pattern recurs on every spec that uses `extra_commands`.
- **Frequency:** subclass:internal-yaml-with-extra-commands. Today in catalog: `producthunt` (4 entries), `shopify` (1 entry). Any future internal-YAML spec that wants to name a hand-written command falls in.
- **Fallback if the Printing Press doesn't fix it:** Agents repeatedly hand-write the Cobra files + `root.go` wiring + remember not to break SKILL.md. Verify-skill catches the gap eventually, but only after a wasted shipcheck loop.
- **Worth a Printing Press fix?** Yes. The fix is one of two cheap template edits.
- **Inherent or fixable:** Fixable. Either (a) stop rendering `extra_commands` rows in SKILL.md's Command Reference and move them under "Hand-written commands shipped by this CLI" prose, OR (b) extend the schema so `extra_commands` carries an implementation hint (e.g., `endpoint: tagsAdd`) and emit a Cobra stub when present.
- **Durable fix:** Pick (a) for v1: `skill.md.tmpl` stops listing `extra_commands` under Command Reference. Add a separate "Hand-written extensions" section that renders them as prose with the description, no `Use:` parsing. Verify-skill then doesn't try to resolve them as Cobra paths. Schema option (b) is a follow-up if the catalog grows enough hand-written commands to justify the additional structure.
- **Test:** Positive — regenerate producthunt; `printing-press verify-skill --dir <cli>` exits 0 (no unknown-command errors for `today`/`recent`/`search`). Negative — a spec with `extra_commands` whose CLI also hand-wires a matching Cobra command should still produce a working CLI with the command visible in `--help` (the hand-written file is what registers it, not the spec).
- **Evidence:** Phase 4 shipcheck output: `[unknown-command] shopify-pp-cli bulk-operations: command path not found in internal/cli/*.go (no matching Use: declaration) evidence: Command Reference inline`. Manually restoring `internal/cli/bulk_operations.go` from the prior library and wiring `newBulkOperationsCmd(flags)` into `root.go` resolved it.
- **Related prior retros:** None (no prior retros under `~/printing-press/manuscripts/*/proofs/*-retro-*.md`).
- **Case-against (Step G):** "Maintainers may say `extra_commands` is intentional documentation of commands the spec author plans to hand-write." Plausible, but the rendering surfaces them as canonical CLI commands in the SKILL.md Command Reference block with no marker indicating they require separate implementation. Verify-skill treats Command Reference rows as Cobra contracts; that's the disagreement. Case-for is stronger because the verify-skill failure is mechanical and recurring.

### 2. Novel-features subagent output has no buildability tag; Phase Gate 1.5 doesn't surface hand-coding scope (Skill instruction gap)

- **What happened:** Phase 1.5c.5 subagent brainstormed 8 transcendence features against the absorb-scoring rubric, all scoring 7-9/10. The rubric includes a "buildability proof" line that says each feature must be buildable. The subagent produced a one-line buildability proof per feature (SQL sketch). At Phase Gate 1.5 the prose showcase listed 8 novel features by name + score + group — no signal that 7 of 8 require hand-written Go (50-150 LoC each, plus tests, plus root.go wiring). The user approved. Generation succeeded but only the framework-emitted `sync-status` reference was visible in the `--help` Highlights; no actual command behind it. Validate-narrative failed because rendered narrative quickstart/recipes pointed at unbuilt commands.
- **Scorer correct?** N/A (this is upstream of the scorer).
- **Root cause:** The subagent prompt (in `references/novel-features-subagent.md`) does not require classifying each feature as `auto-emit-from-spec` vs `hand-code-required`. The Phase Gate 1.5 prose showcase in `SKILL.md` reads stub features separately when the manifest tags them `(stub)`, but does not read hand-code-required features separately. Result: the user sees N novel features approved as shipping scope, with no signal of how many lines of hand-written code that commits the agent to write.
- **Cross-API check:** Pattern recurs on every first-print and every reprint where novel features are aspirational. For specs whose spec source can absorb the brainstormed features (rare — would require the API to already cover the cross-entity joins), the friction is lower. For typical APIs (every CLI in the public library I checked), novel features are all hand-coded.
- **Frequency:** every API that runs Phase 1.5c.5 with a spawned subagent — i.e., every printing-press run.
- **Fallback if the Printing Press doesn't fix it:** Agents discover the gap at the Phase 3 Completion Gate (per-row Cobra resolution check), which HALTs and forces either build-now-or-revise-manifest. Either resolution is rework that the earlier Phase Gate 1.5 could have prevented.
- **Worth a Printing Press fix?** Yes.
- **Inherent or fixable:** Fixable. The subagent's output schema gains a buildability tag; the Phase Gate 1.5 prose showcase reads the tag.
- **Durable fix:** Two coordinated edits. (1) `references/novel-features-subagent.md`: extend the survivors-table contract to require `Buildability` column with values `spec-emits` (rare, generator absorbs it) or `hand-code` (Go file + root.go wiring required). Subagent prompt updated to fill the column. (2) `skills/printing-press/SKILL.md` Phase Gate 1.5 prose showcase: read out the hand-code count separately ("M features require hand-written Go after generate; that's the shipping-scope commitment if you approve"). The user can still approve, but with eyes open.
- **Test:** Positive — re-run Phase 1.5c.5 on shopify reprint; subagent output's survivor table has a `Buildability` column with `hand-code` on all 8 rows; gate prose says "8 features require hand-written Go." Negative — a spec where the brainstorm happens to land on cross-entity joins the generator already absorbs (e.g., the generator's per-resource summary command); those rows tag `spec-emits` and the gate prose doesn't count them in the hand-code total.
- **Evidence:** This session — 8 features approved, 0 auto-emitted, validate-narrative + verify-skill failed on a Grade A scorecard run until narrative was scrubbed back. The subagent's final block reads "All eight: read-only, no LLM, no external services" — buildability is implied as obvious, but it isn't surfaced as the contract.
- **Related prior retros:** None.
- **Case-against (Step G):** "Agents and users know 'transcendence' means hand-coded; that's the whole frame of the rubric." This case loses because the rubric's own "Buildability" line is satisfied by SQL sketches, not by a tag that says "the generator won't emit this." The agent reading the rubric naturally interprets "buildable" as "the press will help me build this" — the rubric doesn't say it won't. And for agent-driven runs (no human supervisor at the gate), the absence of an explicit hand-code count means the agent commits scope without the right framing. Case-for is clearly stronger.

### 3. `validate-narrative --full-examples` fails on commands whose base_url has template vars (Scorer accuracy)

- **What happened:** Shopify spec has `base_url: https://{shop}` and `graphql_endpoint_path: /admin/api/{api_version}/graphql.json`. The `endpoint_template_vars: [shop, api_version]` block declares them. When validate-narrative runs `<cli> sync --full --dry-run` as a verification probe, the client constructor exits 1 because `SHOPIFY_SHOP` and `SHOPIFY_API_VERSION` are unset in the verify subprocess. The probe is supposed to confirm the command parses; instead it fails on config resolution before `--dry-run` short-circuits.
- **Scorer correct?** Partially. Validate-narrative correctly flags "this command path doesn't run cleanly," but the failure mode is environmental, not a real narrative defect. A command that requires server config to even build the URL can't be exercised by a placeholder.
- **Root cause:** Validate-narrative's `--full-examples` mode runs the binary under `PRINTING_PRESS_VERIFY=1` with `--dry-run` appended. It does NOT pre-seed the spec's `endpoint_template_vars` with placeholders. The client constructor (generated from the spec) errors before the RunE's dry-run guard fires.
- **Cross-API check:** Affects any CLI whose spec uses templated base_url or path vars. Examples in the embedded catalog and public library: shopify (`{shop}`, `{api_version}`), any GitHub Enterprise spec (`{host}`), AWS regional APIs (`{region}`), Twilio (`{accountSid}` in path), any multi-tenant SaaS (`{tenant}`). At least 4 named cases with evidence from the catalog.
- **Frequency:** subclass:templated-base-url. Every CLI with template vars.
- **Fallback if the Printing Press doesn't fix it:** Agents either skip `sync --full` in the narrative quickstart (the workaround this session used), or hand-set env vars when running validate-narrative, or accept a false validate-narrative FAIL. None scale.
- **Worth a Printing Press fix?** Yes.
- **Inherent or fixable:** Fixable.
- **Durable fix:** `printing-press validate-narrative --full-examples` reads the spec's `endpoint_template_vars` (internal YAML) or OpenAPI server variables (OpenAPI), and seeds each as `<VAR_UPPER>=<var>_placeholder` (or an existing convention) in the subprocess env before running probes. Mirrors what the existing sync dry-run already prints in its POST URL today. The verify env-var convention is already half-implemented (the dry-run output shows `https://shop_placeholder/admin/api/api_version_placeholder/...` when env vars are unset — that path resolution exists). The gap is solely on the validate-narrative invocation side.
- **Test:** Positive — re-run validate-narrative on shopify with `narrative.quickstart` containing `sync --full`; no env vars set externally; probe exits 0. Negative — a CLI with no template vars shouldn't change behavior; spec env vars not declared shouldn't be auto-set (no scope creep).
- **Evidence:** This session's first shipcheck failed with `FAILED [quickstart]: shopify-pp-cli sync --full → full example failed: ... shopify-pp-cli sync --full --dry-run: exit status 1: ... SHOPIFY_API_VERSION not set; SHOPIFY_SHOP not set`.
- **Related prior retros:** None.
- **Case-against (Step G):** "The user should set the env vars themselves before running validate-narrative." This loses because validate-narrative is invoked from inside `shipcheck` automatically — there's no user invocation moment to set env vars. And the subprocess running the binary doesn't inherit anything the user might pre-set unless `printing-press` itself forwards the env. The cleanest fix is at the validate-narrative tool, not at the user's workflow. Case-for is stronger.

## Prioritized Improvements

### P1 — High priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F2 | Novel-features subagent output lacks buildability tag; Phase Gate 1.5 doesn't surface hand-coding scope | skill | every Phase 1.5c.5 spawn | low (Phase 3 Completion Gate catches it but only after rework) | small (subagent prompt + gate prose edits) | none — additive |

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F1 | `extra_commands` rendered as Cobra commands in SKILL.md but never emitted | generator | subclass:internal-yaml-with-extra-commands (~2 catalog specs today; any future hand-written-extension spec) | medium (verify-skill catches it, but each occurrence is a wasted shipcheck loop) | small (template edit) | none — rendering change |
| F3 | validate-narrative `--full-examples` fails on templated base_url because template vars aren't pre-seeded | scorer | subclass:templated-base-url (≥4 known APIs) | low (agents skip the command in narrative or hand-set env) | small (read spec template_vars, set subprocess env) | only auto-set vars the spec actually declares — no scope creep |

### Skip
| Finding | Title | Why it didn't make it (Step B / Step D / Step G) |
|---------|-------|--------------------------------------------------|
| C2 | Force regen wipes hand-written `internal/cli/*.go` files | Step G: case-against stronger. `--force` is a documented destructive contract; the reprint skill is the correct place to handle preservation of hand-written code, not the generator. The `/printing-press-reprint` skill already has a Phase B that detects prior research; extending it to also detect hand-written `internal/cli/*.go` files is a reprint-skill issue, not a press-binary issue. |
| C3 | `classifyAPIError(err)` → `classifyAPIError(err, flags)` signature drift between press v3.2.1 and v4.6.1 broke hand-written CLI code on reprint | Step G case-against: this is the inherent cost of evolving the generator's internal helper surface. A "stable hand-written code ABI" guarantee would freeze press internals indefinitely. The right fix is on the reprint side (codemod) which is reprint-skill or polish-skill territory, not the generator. |
| C4 | `--help` Highlights and SKILL.md "Unique Capabilities" continued to reference `sync-status` as a built feature after dogfood should have synced it out | Step B: only saw it once; need to verify whether dogfood actually wrote `novel_features_built: []` to research.json or whether the regen restored the rendered blocks from a stale narrative copy. Could be a real generator bug or could be my misreading of the second-regen output. Re-raise with reproduction steps if it shows up on the next CLI. |

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| C6 | Binary lost executable bit between first and second `printing-press generate --force` | iteration-noise (could not reproduce cleanly; possible test-harness artifact from running shipcheck twice on the same dir) |
| DC1 | Subagent invocation initially had `$BRIEF_PATH` variable substitution typo in my own Bash | iteration-noise (my mistake, not a press bug) |

## Work Units

### WU-1: Surface hand-coding scope before Phase Gate 1.5 approval (from F2)
- **Priority:** P1
- **Component:** skill
- **Goal:** Agents and users approving the absorb manifest see, explicitly, how many novel features require hand-written Go code after `generate` returns.
- **Target:** `skills/printing-press/references/novel-features-subagent.md` (subagent output contract) and `skills/printing-press/SKILL.md` Phase Gate 1.5 prose showcase rules.
- **Acceptance criteria:**
  - positive test: re-run Phase 1.5c.5 on a fresh CLI; subagent's final survivor table includes a `Buildability` column with `spec-emits` or `hand-code` per row. Phase Gate 1.5 prose showcase reads out "N features require hand-written Go after generate (hand-code: <list>)" separately from absorbed/auto-emitted counts.
  - negative test: a survivor that genuinely maps to a spec endpoint the generator absorbs ships with `Buildability: spec-emits` and is not counted in the hand-code total.
- **Scope boundary:** Does not change what the generator emits. Does not change the rubric's scoring formula. Only changes (a) the subagent's output schema, (b) the gate's prose showcase rules.
- **Dependencies:** none.
- **Complexity:** small.

### WU-2: Stop rendering `extra_commands` under SKILL.md Command Reference (from F1)
- **Priority:** P2
- **Component:** generator
- **Goal:** verify-skill no longer flags `extra_commands` entries as missing Cobra paths.
- **Target:** `internal/generator/templates/skill.md.tmpl` (Command Reference section) and any sibling template that lists `extra_commands` as if they were canonical Cobra commands.
- **Acceptance criteria:**
  - positive test: regenerate producthunt (catalog spec uses `extra_commands: [today, recent, search]`); `printing-press verify-skill --dir <cli>` exits 0 with no unknown-command errors for those names.
  - negative test: a hand-wired Cobra command whose name happens to match an `extra_commands` entry still appears in the CLI's `--help` and in SKILL.md (the rendering change is at the spec-only ingestion point, not at the Cobra walk).
- **Scope boundary:** Does not remove `extra_commands` from the spec schema. A separate "Hand-written extensions" section in SKILL.md may render the descriptions as prose; that's an additive option, not a requirement.
- **Dependencies:** none.
- **Complexity:** small.

### WU-3: validate-narrative seeds spec template vars before probes (from F3)
- **Priority:** P2
- **Component:** scorer
- **Goal:** `printing-press validate-narrative --full-examples` runs `--dry-run` probes successfully against commands whose base_url or path contains template vars declared by the spec.
- **Target:** `internal/cli/validate_narrative.go` (or wherever the verify subprocess env is composed).
- **Acceptance criteria:**
  - positive test: run `validate-narrative --full-examples --research <shopify-research.json> --binary <shopify-cli>` with `SHOPIFY_SHOP` and `SHOPIFY_API_VERSION` unset in the parent env; quickstart entry `shopify-pp-cli sync --full` exits 0 in the probe subprocess and the report shows `ok`, not `failed-example`.
  - negative test: a CLI whose spec declares no template vars sees no env changes (no scope creep). A spec that declares template vars not relevant to the command under test still probes cleanly.
- **Scope boundary:** Only seeds vars the spec actually declares in `endpoint_template_vars` (internal YAML) or OpenAPI server-variables. Does not invent placeholders for arbitrary env vars.
- **Dependencies:** none.
- **Complexity:** small.

## Anti-patterns avoided in this session
- Did not propose hand-edit fixes to the prior shopify CLI as retro findings; those were polished as printed-CLI work and excluded from retro scope.
- Did not file `--force regen wipes hand-written code` as a generator finding; correctly identified as reprint-skill territory.
- Did not propose `extra_commands` should auto-emit Cobra code (more ambitious fix); chose the smaller "stop rendering as Cobra" fix that resolves the verify-skill failure without committing to a richer schema.

## What the Printing Press got right
- v3.2.1 → v4.6.1 regenerate path was clean — all 8 quality gates passed on first generation, every endpoint mirror emitted correctly, cobratree MCP walker auto-mirrored every Cobra command without manual MCP code, and `mcp.transport: [stdio, http]` spec enrichment landed correctly in the MCP server's `main.go`.
- Phase 3 Completion Gate (per-row Cobra resolution check) would have caught F2 if it had fired before validate-narrative; the gate's existence is correct, the upstream surface (F2) is where the friction lives.
- The reprint skill's prior-research discovery logic correctly identified this as a degraded reprint (no `research.json` provenance) and surfaced the trade-off to the user before proceeding.
