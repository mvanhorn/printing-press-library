# Printing Press Retro: HaloPSA

## Session Stats
- API: HaloPSA / HaloITSM / HaloCRM (Halo's single unified REST API; OpenAPI 3.0.1; 947 paths after stripping `/Control*`; OAuth2 client_credentials)
- Spec source: official Swagger at `https://haloacademy.halopsa.com/api/swagger/v2/swagger.json`, manually enriched (server template, switched scheme to OAuth2 with `x-auth-vars`, added `x-mcp` Cloudflare pattern, stripped pathological `/Control` resource + restored stub schema)
- Scorecard: 91/100 (Grade A) at end of shipcheck; 88/100 after polish exposed `mcp_description_quality` previously omitted as N/A
- Verify pass rate: 100% (133/133)
- Fix loops: 1 generation retry (after stripping Control), 2 shipcheck rerun loops (dogfood unregistered children, verify-skill phantom flags, narrative side-effect heuristic), 1 polish pass
- Manual code edits: 13 transcendence command files + `sqlcmd.go` + `novel_register.go` (per absorb-manifest plan), 4 narrative/doc fixes (drop `--agent-load`, replace `--tenant` flag with env var, drop `auth login` from quickstart, drop `--apply` from age-out recipe), `clients card` → `client card` typo fix, `%d → %g` go-vet format-string fix
- Features built from scratch: 13 transcendence commands + `sql` SELECT-only wrapper, then registered via `novel_register.go`. All shipped working.
- Generation time: ~30 min after Control fix; total wall-clock ~1.5 hours including polish + retro
- Final verdict: `hold` (polish downgrade from `ship`) — every functional gate green, but 1268 tools-audit findings remained; polish itself recommended retro over further polish

## Findings

### F1 — Generator-emitted typed table can exceed SQLite's column cap on wide config singletons (template gap)

- **What happened:** First `printing-press generate` run completed all 8 quality gates PASS. First `halopsa-pp-cli sync --full --dry-run` died with `SQL logic error: too many columns on control (1)`. The generator promoted Halo's `Control` resource — a tenant-global config singleton — to a typed table with **3,747** columns, well past SQLite's default `SQLITE_MAX_COLUMN=2000`. Tickets, projects, and opportunities each landed at 847 columns; they fit but only just. I stripped `/Control*` paths from the spec (5 paths) and restored a stub schema for $refs, then regenerated successfully.
- **Scorer correct?** N/A — this was a generation-time runtime failure surfaced by my own `sync --dry-run`, not a scorer penalty. The gates were too soft to catch it: `verify` and `dogfood` exercised commands that don't open the database; only `sync` does.
- **Root cause:** `generator` — the typed-table template emits one column per leaf JSON field discovered in the response schema with no upper bound. Halo's `Control` is a flat object with thousands of feature flags (`addigy_alert_id`, `2faauthenticatorallowed`, etc.). No fallback exists for resources whose schemas exceed a safe column count.
- **Cross-API check:** Enterprise APIs with very wide flat config singletons are the danger zone.
  - **HaloPSA** (this run): `/Control` schema produced 3,747 columns; sync failed cold until stripped.
  - **ServiceNow Table API**: `sys_properties` and `sys_user_role` famously expose hundreds of flat columns; these would land near or over the cap.
  - **Salesforce metadata API** + **Atlassian/Jira instance configuration** are documented as having similarly wide flat config representations.
  - Direct evidence: 1 (HaloPSA). Strong inference based on documented schema shape: 2 more. This is the "named API + endpoint with evidence" bar; the other two would need the catalog to actually carry their specs to lift to ironclad evidence.
- **Frequency:** subclass:wide-config-singleton — narrow class, hard failure when it fires.
- **Fallback if the Printing Press doesn't fix it:** Agent has to: (a) attempt sync (or even just see it during gen — generator could log column counts), (b) hit the SQLite error, (c) hand-strip the resource from the spec, (d) restore a stub for $refs, (e) regenerate. Fallback works when the agent is paying attention but is exactly the kind of opaque failure mode this skill cycle is supposed to prevent.
- **Worth a Printing Press fix?** Yes — detect at generation time, fall back to JSON-only storage. Small fix.
- **Inherent or fixable:** Fixable.
- **Durable fix:** In the per-resource typed-table-emit path (`internal/generator/templates/store/typed_table.go` or equivalent), compute the column count before emitting `CREATE TABLE`. If the count exceeds a threshold (1500 leaves headroom under SQLite's default 2000; tunable), fall back to JSON-only schema (`id TEXT PRIMARY KEY, data JSON NOT NULL, synced_at DATETIME`). Log the fallback so the agent knows. Don't fall back silently below the threshold — typed columns power the offline FTS5 and the cross-entity SQL joins novel commands depend on.
- **Test:** Positive — generate against a synthetic spec where one resource has 3000+ leaf fields; assert the table has 3 columns (id/data/synced_at) and the upsert path still stores JSON. Negative — generate against a normal resource (under threshold); assert typed columns still emit.
- **Evidence:** Run 1 generate output (`$RUN_DIR/proofs/generate-output.log` shows PASS); `halopsa-pp-cli sync --full --dry-run` from working dir shows the actual error: `migration failed: SQL logic error: too many columns on control (1)`. Stripped spec at `$RUN_DIR/research/halopsa-swagger-enriched.json` (947 paths after removing Control).
- **Step G case-against:** "Halo's Control is an outlier; you wouldn't see this on the next 10 APIs." Counter: the cost-side of the asymmetry — when this fires it's a complete-functional-failure that polish can't fix, and the fix shape (column-count threshold + JSON fallback) is conservative, cheap, and clearly cross-API beneficial even at lower thresholds. Case-for is stronger.
- **Related prior retros:** None matched.

### F2 — `tools-audit` flags DO-NOT-EDIT Hidden parent groupers as thin-short / missing-read-only (scorer bug)

- **What happened:** After polish ran `printing-press mcp-sync` and surfaced the typed-endpoint MCP surface to the manifest, `printing-press tools-audit` produced 1268 pending findings on the HaloPSA CLI. **246 of those are thin-short on DO-NOT-EDIT generator-emitted Hidden parent grouper commands** (e.g., the `tickets`, `client`, `asset` resource parents that have `Hidden: true` and `Short: "Manage X"`). Polish documented these as un-fixable inside polish's contract: hand-edits get wiped on regen, and these commands aren't exposed as user-facing MCP tools anyway.
- **Scorer correct?** No. Hidden parent groupers don't surface as MCP tools at runtime (they exist to organize the Cobra tree). Flagging their `Short: "Manage X"` as a description-quality defect is a category error — there's no user or agent that sees that string. The audit should exempt commands where `Hidden == true` and the command has at least one MCP-visible child.
- **Root cause:** `scorer` — `tools-audit`'s thin-short detector doesn't consult Cobra's `Hidden` flag or the MCP-visibility predicate.
- **Cross-API check:** Every CLI with a resource-grouper Cobra topology hits this. Wide APIs hit it especially hard:
  - **HaloPSA** (this run): 246 hits.
  - **Servosity**: 33 resource tags → ~33 hidden parents in the same pattern (see `~/printing-press/manuscripts/servosity/20260511-211606/proofs/20260512-140458-retro-servosity-pp-cli.md` F2 for adjacent territory — `endpoint_tools: hidden` doesn't suppress endpoint mirrors from manifest; my finding is the parent-grouper analog).
  - **HubSpot**: 20-spec merge with 25+ resource parents → 25+ hits.
- **Frequency:** every multi-resource CLI generated through the standard Cobra-parent topology. Halo, Servosity, HubSpot are all in the catalog/library and demonstrate this.
- **Fallback if the Printing Press doesn't fix it:** Polish has to accept-or-skip each one. The 5-cluster duplicate-rationale gate makes that prohibitively expensive at scale (246 individual accepts isn't realistic). The actual recovery today is "skip all of these in the polish report and live with the scorecard noise."
- **Worth a Printing Press fix?** Yes — the fix is small and prevents recurring scorecard noise that polish can't address.
- **Inherent or fixable:** Fixable cheaply.
- **Durable fix:** In `tools-audit` (likely under `internal/cli/tools-audit.go` or `internal/scoring/tools_audit/`), short-circuit the thin-short and missing-read-only checks for any command where `cmd.Hidden == true` AND `len(cmd.Commands()) > 0` (hidden grouper with children). Optionally also short-circuit on `cmd.Annotations["mcp:hidden"] == "true"`. The audit's intent is per-tool quality on what an agent actually sees as an MCP tool; these parents aren't that.
- **Test:** Positive — given a CLI with one Hidden parent grouper that has child commands, assert that parent is NOT in the thin-short findings list. Negative — given a CLI with an un-Hidden parent that has `Short: "Manage X"`, assert it IS flagged (no regression).
- **Evidence:** This run's `printing-press tools-audit --json` after polish (`$RUN_DIR/proofs/2026-05-21-131235-fix-halopsa-pp-cli-polish.md` — 246 thin-short + 6 missing-read-only on Hidden parents specifically called out). Sample offending file: `internal/cli/tickets.go` (Hidden: true, Short: "Manage tickets").
- **Step G case-against:** "Polish could hand-author 246 overrides." Counter: the polish playbook explicitly forbids bulk-accept; per-tool individual override is unrealistic at this scale; the scorer is reporting noise that polish can't drain. Case-for is stronger.
- **Related prior retros:**
  - `servosity-pp-cli` retro (manuscripts: `servosity/20260511-211606/`) — `extends`. Servosity F2 fixed `endpoint_tools: hidden` leak from manifest; my finding addresses the *other* hidden-tool class (Cobra parent groupers, not endpoint mirrors). Both are tools-audit treating "hidden from agents" as "visible to scorer." A single fix to the scorer's visibility predicate would close both.

### F3 — Generator emits raw upstream operation descriptions even when they're systemically thin (template gap)

- **What happened:** HaloPSA's upstream OpenAPI ships nearly every operation with a boilerplate description shape: `"Use this to return multiple X.<br>Requires authentication."`. The generator passes those through to `tools-manifest.json` verbatim. `tools-audit` flagged **1016 thin-mcp-description** findings. `scorecard mcp_description_quality` dropped from N/A (when no manifest existed) to 0/10 once polish ran `mcp-sync`. Polish documented this as un-fixable inside polish's contract: bulk-accept is forbidden by the 5-cluster gate, and authoring 1016 individual overrides isn't in scope for one pass.
- **Scorer correct?** Yes — these descriptions ARE thin. The CLI ships with low-quality agent-facing tool descriptions. The fix isn't in the scorer; it's in the generator's description-composition path.
- **Root cause:** `generator` — when the upstream spec's `operation.description` is thin (short, boilerplate, or matches anti-patterns like "Use this to return"), the generator should compose a synthetic description from structured signals (operationId/method/path/parameters/tags/responses).
- **Cross-API check:**
  - **HaloPSA** (this run): 1016 thin-mcp-description findings; spec descriptions are uniformly `"Use this to return multiple X."` or `"Use this to return a single instance of X."`.
  - **Servosity** (`~/printing-press/manuscripts/servosity/20260511-211606/proofs/20260512-140458-retro-servosity-pp-cli.md` F2 evidence): generator-composed descriptions like `"Verb. Required:/Optional:/Returns the X."` were the polish-time fix shape. Servosity hand-authored 110 overrides + 98 composer-generated; that's two-thirds of a working solution but on the polish side, not the generator.
  - **HubSpot** (`~/printing-press/manuscripts/hubspot/20260511-204733/proofs/...`): the 20-spec merge means many spec sections come from HubSpot's auto-generated SDK definitions, which carry similarly mechanical descriptions.
  - Three named with evidence.
- **Frequency:** common-to-pervasive — vendors with auto-generated SDK-style specs (Halo, HubSpot's per-resource specs, ServiceNow, many AWS sub-services).
- **Fallback if the Printing Press doesn't fix it:** Polish authors per-tool description overrides via the `tools-polish.json` mechanism. Servosity proved this works at the ~110-tool scale. At 1016 tools (Halo's hidden code-orch surface), it doesn't.
- **Worth a Printing Press fix?** Yes — the generator already has all the signals it needs (operationId, method, parameters, response schema). Composing a 2-3 sentence description from structured signals raises the floor across every CLI with a thin upstream spec.
- **Inherent or fixable:** Fixable.
- **Durable fix:** In the generator's description-emit path for typed-endpoint tools, detect thin descriptions (empty, < ~30 chars, matches a vendor anti-pattern like `Use this to return`, or contains only "Requires authentication." after stripping). When thin, compose a synthetic description: `<Verb> <resource>. <Required params summary>. Returns <response shape summary>.` Use existing typed-response struct info where available. Mark composed descriptions with a comment in the manifest so future scorer audits can distinguish "vendor-thin pass-through" from "composer-emitted."
- **Test:** Positive — generate against a spec where every operation description is `"Use this to return multiple X."`; assert generated manifest has descriptions like `"List <resource>. Required: <params>. Returns: array of <type>."` Negative — generate against a spec with rich existing descriptions (e.g., GitHub's docs-driven OpenAPI); assert pass-through behavior is unchanged.
- **Evidence:** This run's polish report (1016 thin-mcp-description findings); Servosity's `~/printing-press/library/servosity/.printing-press-tools-polish.json` 110-entry override file is the manual analog of what the generator could produce.
- **Step G case-against:** "Polish's tools-polish.json override mechanism already solves this; don't change the generator." Counter: Servosity scaled to 110 entries successfully but Halo's 1016 is past the bulk-accept gate. The override mechanism doesn't scale; the generator-composition path does. Servosity's retro F2 actually proposes adjacent generator improvements (suppress hidden-from-manifest); my finding extends that direction (compose better descriptions for the entries that DO ship). Case-for is clearly stronger.
- **Related prior retros:**
  - `servosity-pp-cli` retro F2 — `extends`. Servosity proposes "don't write hidden endpoint mirrors to manifest"; my finding proposes "for entries that DO write, compose better descriptions when upstream is thin." Both could land independently; both together raise the floor for >50-tool CLIs.

### F4 — `validate-narrative --strict --full-examples` treats deliberate side-effect skips as failures (scorer bug)

- **What happened:** Shipcheck umbrella defaults to `validate-narrative --strict --full-examples`. The validator's side-effect heuristic flags any example command containing `auth`, `launch`, or `--apply` as `UNSUPPORTED` (deliberately skipped). Under `--strict --full-examples`, UNSUPPORTED counts as `narrative validation failed`. My initial HaloPSA narrative had `auth login` as quickstart[0] (the natural setup-first step) and `tickets age-out --apply` as a cookbook recipe (showing the actual bulk-close form). Both are correct end-user examples; both fail the strict validator. I had to drop `auth login` from quickstart entirely (substituted with `doctor`, putting the auth-setup hint in a comment) and remove `--apply` from the recipe (showing only preview form).
- **Scorer correct?** Partially. The validator is right to skip live-running `auth login` and `--apply`; those WOULD have side effects. But treating "deliberately skipped because side-effectful" the same as "actually failed when invoked" misrepresents the result and effectively bans first-class side-effect examples from any narrative.
- **Root cause:** `scorer` — `validate-narrative` conflates UNSUPPORTED (heuristic skip with reason) and FAILED (invocation attempted, exited non-zero) under `--strict --full-examples`.
- **Cross-API check:**
  - **HaloPSA** (this run): 2 UNSUPPORTED failures (`auth login --client-id ...`, `tickets age-out ... --apply`); narrative had to be reshaped to dodge the heuristic.
  - **Servosity**: `~/printing-press/manuscripts/servosity/...` would have the same issue — Servosity's CLI has `auth login` as its first quickstart step too.
  - **HubSpot**: HubSpot retro shows the same auth-first quickstart pattern with `--api-key`; the validator's `auth` keyword match would fire here too.
  - Three named with evidence (auth-flow narrative is universal).
- **Frequency:** every CLI that documents an auth flow as its first quickstart step — i.e., every multi-step CLI with non-trivial auth (which is most of them).
- **Fallback if the Printing Press doesn't fix it:** Agents reshape narrative to drop side-effect examples. That's a real loss: the quickstart's whole point is "start here," and "set credentials and log in" IS step 0 for these APIs. The substitute ("run doctor and read the env-var hint in the comment") is informationally lossy.
- **Worth a Printing Press fix?** Yes — small fix in the validator; large UX win.
- **Inherent or fixable:** Fixable.
- **Durable fix:** In `validate-narrative`, when a finding is classified as UNSUPPORTED with `reason: "command is side-effectful"`, treat it as a warning under `--strict --full-examples` rather than a failure. Alternatively, allow narrative entries to carry an explicit `side_effect: true` annotation that the validator honors. Two compatible options; either or both. Keep `--strict --full-examples` failing on true FAILED invocations and on MISSING / EMPTY entries.
- **Test:** Positive — narrative with `auth login` as first quickstart and `--apply` recipe runs `validate-narrative --strict --full-examples`; assert exit 0 with both items logged as UNSUPPORTED warnings. Negative — narrative with a genuinely broken command (e.g., misspelled subcommand) runs the same call; assert exit non-zero with FAILED finding (no regression).
- **Evidence:** `$RUN_DIR/proofs/shipcheck-run1.log` line: `UNSUPPORTED [quickstart]: halopsa-pp-cli auth login ... → full-example validation skipped: command is side-effectful (auth/launch/apply)` followed by `narrative validation failed`. Same shape on the `--apply` recipe.
- **Step G case-against:** "Authors should just not write side-effectful examples in narrative — that's poor UX anyway." Counter: `auth login` IS the right first step for any OAuth2/API-key CLI; `--apply` IS the headline form of bulk operations. Banning them from narrative is the wrong UX, not the right UX. The agent had to make worse documentation to satisfy a validator quirk. Case-for is stronger.
- **Related prior retros:** None matched. First sighting of this specific UNSUPPORTED/FAILED conflation.

## Prioritized Improvements

### P1 — High priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F1 | Wide-schema typed table exceeds SQLite column cap on config singletons | generator | subclass:wide-config-singleton; complete-functional-failure when it fires | Agent catches if attempts sync; misses if not | small (column-count check + JSON-only fallback path) | Don't fall back below threshold — preserve typed-column behavior for normal schemas |

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F2 | tools-audit flags Hidden parent groupers as thin-short / missing-read-only | scorer | every multi-resource CLI; ~10–250 findings per CLI | Polish can't drain at scale (5-cluster gate blocks bulk accept) | small (visibility-predicate guard) | None — Hidden+children is a clean signal |
| F3 | Generator emits raw upstream operation descriptions when they're systemically thin | generator | common across vendors with auto-generated specs (Halo, HubSpot, Servosity, ServiceNow) | Polish overrides scale to ~100, not 1000s | medium (description composer; reuse typed-response struct info) | Detect thin via length + anti-pattern regex; don't overwrite rich existing descriptions |
| F4 | validate-narrative --strict --full-examples treats UNSUPPORTED side-effect skips as failures | scorer | every CLI with auth-flow first quickstart step | Narrative reshaping (informationally lossy) | small (treat reason=side-effectful as warning under strict) | Keep FAILED / MISSING / EMPTY as failures |

### Skip
*(none — every survivor of Phase 2.5 triage filed.)*

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| `--agent-load` phantom flag in narrative | I invented a flag in `research.json` triage example; validator caught it | iteration-noise (agent narrative discipline, not machine issue) |
| `--tenant` flag missing on `auth login` | Halo's server template `{tenant}` overlaps with auth-flow input; CLI exposes it via env var only, narrative referenced it as a flag | printed-CLI (narrative discipline; env-var is the right pattern, SKILL could mention) |
| dogfood static analyzer can't see `findChildCmd` registry-pattern AddCommand | My `novel_register.go` used a runtime parent lookup; static AST only saw 8 children as unregistered | iteration-noise (I chose a non-SKILL pattern; SKILL recommends literal `parent.AddCommand(...)` which is what I refactored to) |
| `--lenient` flag doesn't tolerate missing schema $refs | After I stripped `/Control*`, residual $refs to the Control schema broke generation; needed stub | printed-CLI (cleaner agent workflow: strip resource AND its $refs together; SKILL note worth adding but not a generator fix) |
| Bundled `spec.json` overrides caller `--spec` | dogfood reported "caller --spec overridden by bundled" even though I passed --spec explicitly | API-quirk / minor UX (the bundled spec is the right canonical source post-generation; messaging could be clearer but logic is right) |
| Breadth scorer penalty for all-auth APIs | HaloPSA scored 6/10 on breadth because 0 public endpoints exist; Halo's API is auth-gated by design | unproven-one-off (tradeoff question, not a clear-cut bug; tradeoff convo with maintainers warranted, not retro material) |

## Work Units

### WU-1: Cap-and-fall-back typed table emit (from F1)
- **Priority:** P1
- **Component:** generator
- **Goal:** Generator detects resources whose discovered column count exceeds a safe SQLite ceiling and falls back to JSON-only storage so sync doesn't fail at runtime.
- **Target:** Typed-table-emit template in `internal/generator/templates/store/` (specifically the `CREATE TABLE` composer for typed resources).
- **Acceptance criteria:**
  - positive test: feed a synthetic spec where one resource has 3,000+ leaf fields (or, regression-style, point at Halo's `Control` schema before strip); generated `internal/store/store.go` `CREATE TABLE` for that resource has only `id TEXT PRIMARY KEY, data JSON NOT NULL, synced_at DATETIME`; sync against an empty DB succeeds.
  - negative test: feed a normal resource (e.g., HaloPSA tickets at 847 columns); typed-table emission unchanged; flat columns still emitted.
  - generation-time log line names the resource and the fallback (e.g., `store-fallback: control (3747 cols) → JSON-only`).
- **Scope boundary:** Does NOT extend to per-column truncation, schema-aware projection, or per-API column allowlists. Single threshold + JSON-only fallback only.
- **Dependencies:** None.
- **Complexity:** small.

### WU-2: tools-audit exempts Hidden parent groupers (from F2)
- **Priority:** P2
- **Component:** scorer
- **Goal:** `tools-audit` short-circuits thin-short and missing-read-only checks for Cobra commands that are Hidden AND have at least one MCP-visible child — these are organizational groupers that don't surface as agent tools and shouldn't be scored as if they do.
- **Target:** `tools-audit` source (likely `internal/cli/tools_audit.go` or under `internal/scoring/tools_audit/`); whichever module runs the thin-short and missing-read-only detectors.
- **Acceptance criteria:**
  - positive test: a generated CLI with one Hidden parent grouper (`tickets`, `Hidden: true`, `Short: "Manage tickets"`, with children like `tickets list`, `tickets create`) — `tools-audit --json` reports zero thin-short or missing-read-only findings on the parent.
  - negative test: an un-Hidden command with `Short: "Manage X"` and no children — still flagged thin-short.
  - regression test: regenerate Servosity and HaloPSA; the parent-grouper finding count drops to zero on both.
- **Scope boundary:** Does NOT change tool description quality scoring on the children themselves; only the parent groupers are exempted. Also does NOT touch the `endpoint_tools: hidden` mirror suppression (Servosity F2's domain).
- **Dependencies:** None.
- **Complexity:** small.

### WU-3: Compose synthetic operation descriptions when upstream is thin (from F3)
- **Priority:** P2
- **Component:** generator
- **Goal:** Generator detects thin upstream operation descriptions and composes a structured synthetic description from operationId / method / path / parameter list / response schema, replacing only the thin ones.
- **Target:** The generator path that emits operation descriptions into `tools-manifest.json` and the Cobra `Long:` field (likely in `internal/generator/templates/mcp/` and the cobra-command emitter).
- **Acceptance criteria:**
  - positive test: feed a spec where every operation.description is `"Use this to return multiple X."` or empty; assert generated manifest descriptions follow the shape `"<Verb> <resource>. Required: <params>. Returns: <response shape>."`
  - negative test: feed GitHub's docs-driven OpenAPI (rich descriptions); assert pass-through unchanged.
  - mixed test: spec with some rich and some thin descriptions; assert only thin ones are composed; rich ones pass through.
  - regression: regenerate Halo, Servosity, HubSpot; verify `mcp_description_quality` rises meaningfully on all three.
- **Scope boundary:** Does NOT include per-resource hand-tuning, semantic NLP, or vendor-specific recipes. Mechanical composition from structured spec signals only. Does NOT replace polish's tools-polish.json mechanism — the override path stays for cases where the composer's output isn't good enough.
- **Dependencies:** None. (WU-2 is independent — fix the scorer's blind spot AND raise the floor on descriptions.)
- **Complexity:** medium.

### WU-4: validate-narrative treats UNSUPPORTED (side-effect skip) as warning under --strict --full-examples (from F4)
- **Priority:** P2
- **Component:** scorer
- **Goal:** `validate-narrative --strict --full-examples` distinguishes UNSUPPORTED (deliberately skipped with a reason) from FAILED (invocation attempted, exited non-zero) and treats UNSUPPORTED side-effect skips as warnings rather than failures.
- **Target:** `validate-narrative` source (likely `internal/cli/validate_narrative.go` or the validator under `internal/scoring/narrative_validate/`); the exit-code-determination logic that aggregates findings.
- **Acceptance criteria:**
  - positive test: narrative with `auth login` quickstart and `--apply` recipe; under `--strict --full-examples`, both surface as UNSUPPORTED with reason `command is side-effectful`; exit code 0; stderr/stdout logs both as warnings.
  - negative test: narrative with a genuinely broken command (misspelled subcommand or invalid flag); under `--strict --full-examples`, surfaces as FAILED; exit non-zero (no regression).
  - missing-entry test: narrative.json with empty quickstart array — still fails under `--strict` as before.
- **Scope boundary:** Does NOT remove the side-effect heuristic (still useful info). Does NOT change `--strict` behavior for FAILED / MISSING / EMPTY findings — those still fail.
- **Dependencies:** None.
- **Complexity:** small.

## Anti-patterns

- **Generator emits a typed table without bounding column count.** F1 above — the failure mode is "sync silently dies on first run." Any unbounded loop over discovered schema fields should have a sanity bound and a fallback path.
- **Scorer treats deliberate skips as failures under strict mode.** F4 — conflating "I chose not to evaluate" and "I tried and it failed" inverts what `--strict` is for. The former is a warning at most; the latter is a real failure.
- **Scorer scores tools the runtime never exposes to agents.** F2 — Hidden parent groupers exist for Cobra-tree organization; flagging their descriptions misrepresents what an agent actually sees.

## What the Printing Press Got Right

- **Resource-shadowing rename was handled automatically.** Halo's spec has `/Feedback`, `/Health`, `/Search`, `/Workflow` paths that would collide with framework cobra commands. Generator transparently renamed them to `halo-feedback`, `halo-health`, `halo-search`, `halo-workflow` and logged the warnings up-front. Zero hand-edits needed.
- **MCP Cloudflare-pattern enrichment via spec `x-mcp` block worked first try.** Setting `mcp.transport: [stdio, http]`, `mcp.orchestration: code`, `mcp.endpoint_tools: hidden` in the spec gave a >900-tool MCP surface a clean stdio+http transport, a thin search+execute orchestration pair, and suppressed endpoint-mirror noise. The SKILL's Phase 2 MCP-enrichment guidance is the right shape and the right escalation threshold (>50 tools).
- **`flags.printJSON(cmd, v)` + `printJSONFiltered` chain made novel commands cheap.** Every transcendence command picked up `--json`, `--select`, `--compact`, `--csv`, and `--quiet` for free. The `IsVerifyEnv()` short-circuit helper for verify-time side-effect protection is the right shape — the only thing missing was an example in the SKILL pointing at it for hand-written transcendence command authors.
- **`dryRunOK(flags)` helper lives in `helpers.go`.** Every novel command that opens the database or makes a live API call short-circuits cleanly under `--dry-run` and stays verify-friendly. The pattern is good enough that all 13 hand-built commands picked it up with one read of the SKILL.
- **`validate-narrative --full-examples` caught both real bugs (the `--agent-load` phantom flag and the `clients card` → `client card` typo).** When the agent IS being undisciplined about narrative claims, the validator catches it. F4 is about an over-aggressive case; the validator's existence isn't the issue, just its strict-mode aggregation.
- **The Phase 5.5 polish skill's structured `---POLISH-RESULT---` block.** Made the verdict-override rule cleanly applicable. Polish's `further_polish_recommended: no` + reasoning steered me directly to retro without an interactive decision. The mechanism is the right shape.
