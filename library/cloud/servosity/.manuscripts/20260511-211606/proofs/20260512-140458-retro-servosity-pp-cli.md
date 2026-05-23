# Printing Press Retro: Servosity

## Session Stats
- API: servosity (Servosity backup/DR API; single-tenant private SaaS)
- Spec source: Swagger 2.0 fetched from https://api.servosity.com/docs/?format=openapi (255 paths, 328 method-endpoints, 33 resource tags)
- Scorecard: 91/100 (Grade A) after polish-retry (was 90 before polish ran mcp-sync; 88 after thin-mcp-descriptions surfaced; 91 after 110 override entries authored)
- Verify pass rate: 100%
- Fix loops: 1 shipcheck loop (4 fixes applied automatically), 2 polish passes (hold → ship after the main SKILL corrected `phase5-acceptance.json`)
- Manual code edits: 10 novel commands hand-built (the planned transcendence work) + ~20 small polish/fix edits across shipcheck + 4.85 review loops
- Features built from scratch: 10 transcendence commands (attention, drift, stale-backups, backup-facts, find, restore-queue, company show, triage, clear, stale-issues) + store snapshot extension + timewords helper

## Findings

### F1 — `swagger: "2.0"` specs silently mis-parsed by the loader (template gap)

- **What happened:** Fed the Servosity OpenAPI URL to `printing-press generate`. The generator's loader (`internal/openapi/parser.go:880` — `loadOpenAPIDoc`) calls `openapi3.NewLoader().LoadFromData(data)` directly. For a Swagger 2.0 input that *also* shapes overlap enough with OpenAPI 3 to parse paths, kin-openapi silently drops `host`, `basePath`, `schemes`, and `securityDefinitions` (these live in different JSON locations in Swagger 2 vs OpenAPI 3). Result: probe-generated CLI shipped with `BaseURL: "https://api.example.com"` (placeholder), zero env-var declarations, and the `Authorization` header value missing the `Token ` prefix the API requires. All 8 quality gates passed because no live call was attempted with the placeholder. I had to write a 210-line Python conversion script (Swagger 2 → OpenAPI 3 with `x-prefix`, `x-auth-vars`, `x-mcp`) and feed the converted JSON instead.
- **Scorer correct?** N/A — this was not a scorer penalty. The shipcheck gates all passed because verify/dogfood don't exercise the base-URL against a live server.
- **Root cause:** `openapi-parser` — `loadOpenAPIDoc` doesn't detect or convert Swagger 2 input.
- **Cross-API check:** drf-spectacular still emits Swagger 2 in some configurations; some apis.guru entries are Swagger 2; older Django REST framework deployments serve it. The catalog probably has Servosity-shaped APIs in it.
- **Frequency:** subclass:swagger2 — narrow today, but the failure mode (silent placeholder ship) is the worst kind.
- **Fallback if the Printing Press doesn't fix it:** Agent reads `swagger: "2.0"` in the spec, refuses to use the generator's URL path, and either runs a manual conversion or hand-writes the spec in internal YAML. Catches if the agent is paying attention; misses if it isn't.
- **Worth a Printing Press fix?** Yes — the fix is small. Detect `swagger: "2.0"` in the bytes, run `kin-openapi/openapi2conv.ToV3()`, then proceed. About 10 lines plus the import.
- **Inherent or fixable:** Fixable.
- **Durable fix:** In `loadOpenAPIDoc`, sniff the first ~200 bytes for `"swagger":\s*"2.0"`. If matched, parse as Swagger 2, convert with `openapi2conv.ToV3`, then continue. If conversion fails, return a clear error ("convert this spec to OpenAPI 3 before generating") rather than silently falling back.
- **Test:** Positive — pass a Swagger 2 spec, assert servers[].url is populated and securityDefinitions are translated to components.securitySchemes. Negative — pass an OpenAPI 3 spec, assert behavior unchanged.
- **Evidence:** This run; converted spec at `~/printing-press/manuscripts/servosity/20260511-211606/research/servosity-openapi-v3.json` (vs the original `servosity-openapi.json`).
- **Step G case-against:** "Swagger 2 is rare in 2026; manual conversion was a one-time cost." Counter: the failure mode is silent (placeholder base URL ships through every gate). One small generator fix prevents the worst-case outcome. The case-for is clearly stronger.
- **Related prior retros:** None matched the swagger-2 specific failure mode; #1011 (vendor-extension auth detector) is adjacent component but different problem.

### F2 — `endpoint_tools: hidden` doesn't suppress endpoint mirrors from `tools-manifest.json` (template gap)

- **What happened:** Servosity has 328 endpoints. The skill correctly opted into the Cloudflare pattern (`mcp.transport: [stdio, http]`, `mcp.orchestration: code`, `mcp.endpoint_tools: hidden`). At runtime, `RegisterCodeOrchestrationTools` correctly hides the 110 endpoint-mirror tools behind a thin `<api>_search` + `<api>_execute` pair, so the agent surface is clean. But `tools-manifest.json` still lists all 110 endpoint mirrors with thin auto-composed descriptions ("Verb. Required:/Optional:/Returns the X."), which `tools-audit` flags as 110 pending findings and `scorecard mcp_description_quality` scores 3/10. Polish iteration 2 authored a 110-entry override file (98 composer-generated + 12 hand-crafted) and ran `mcp-sync` twice to backfill the manifest; that brought scorer to 10/10 but represents per-CLI effort the generator could absorb.
- **Scorer correct?** Partially. `tools-audit` is correctly counting thin descriptions in the emitted manifest — the manifest is the file the scorer can see. But the entries it's counting are tools the agent *never sees at runtime* (the Cloudflare hidden-tools pattern). The cleaner fix is at the generator template: when `endpoint_tools: hidden`, don't write those entries to `tools-manifest.json` in the first place.
- **Root cause:** `generator` — the MCP manifest-writer template (somewhere under `internal/generator/templates/mcp/`) emits endpoint-mirror entries regardless of the `endpoint_tools` configuration. The runtime registration honors the flag; the manifest writer doesn't.
- **Cross-API check:** Every code-orchestration CLI with >50 endpoints opts in; Servosity is the first I directly observed.
- **Frequency:** subclass:code-orchestration-large-surface — narrow today; the Cloudflare pattern is itself recent. Will grow if more agents opt into the pattern on big APIs.
- **Fallback if the Printing Press doesn't fix it:** Polish authors a manifest-overrides file (as it did here). Real cost per CLI: ~20 minutes of LLM time + the noise on the scorer.
- **Worth a Printing Press fix?** Probably yes, even with one named example. The fix is a template guard (skip endpoint-mirror entries when `mcp.endpoint_tools == "hidden"`). Compounds across every future >50-tool CLI.
- **Inherent or fixable:** Fixable.
- **Durable fix:** In the manifest emit path, skip entries for tools whose Cobra parent is registered as hidden under code-orchestration. The runtime walker already knows which tools are hidden; the manifest writer should consult the same predicate.
- **Test:** Positive — generate a >50-tool CLI with `mcp.endpoint_tools: hidden`, assert `tools-manifest.json` contains only the orchestration tools and the novel commands, not the 100+ endpoint mirrors. Negative — generate without `endpoint_tools: hidden`, assert the manifest still contains the endpoint mirrors.
- **Evidence:** This run; `~/printing-press/library/servosity/.printing-press-tools-polish.json` (the 110 overrides polish landed) and the polish proof at `proofs/2026-05-12-121000-fix-servosity-pp-cli-polish.md`.
- **Step G case-against:** "Only 1 named example today; could be premature." Counter: the polish skill's own inline retro-candidate flagged this same area (it proposed a richer composer; I'm proposing manifest suppression). Two independent agents converged on "this is a generator-template issue." Case-for is stronger.
- **Related prior retros:**
  - #1130 (open, P2) — `aligned`: "Skill: prompt to apply Cloudflare MCP pattern when generator warns >50 endpoints." That issue is the *upstream* gate (prompt the user to opt in). This finding is the *downstream* consequence (the manifest contains entries that shouldn't reach the manifest).
  - #740 (closed) — `extends`: "mcp_token_efficiency dimension scores 0/10 even when Cloudflare-pattern emission is the optimal MCP shape." Same domain (Cloudflare pattern + scorer disagrees with optimal emission), different specific dimension (token efficiency vs description quality).

### F3 — Typed-upsert tests fail NOT NULL on parent-FK columns (DUPLICATE of #1063)

- **What happened:** `go test ./internal/store/` failed three tests:
  - `TestUpsertBatch_PopulatesStartBackupTable`: `constraint failed: NOT NULL constraint failed: start_backup.restic_backups_id`
  - `TestUpsertBatch_PopulatesStartRestoreTable`: same shape on `start_restore.restic_backups_id`
  - `TestUpsertBatch_PopulatesResticBackupsTunnelTable`: same shape on `restic_backups_tunnel.restic_backups_id`

  These are sub-resources of `/restic-backups/{id}/start-backup/`, `/restic-backups/{id}/start-restore/`, `/restic-backups/{id}/tunnel/`. The generated test fixture inserts a row without populating the parent-FK column, but the schema declares it NOT NULL.
- **Scorer correct?** N/A — these are unit tests of generator-emitted code. The tests are correct in their intent; the generator emits a fixture that can't satisfy its own schema.
- **Root cause:** Same as #1063 — exactly. ElevenLabs hit `voices_id` on `edit` / `samples` / `voices_settings`; Servosity hit `restic_backups_id` on `start_backup` / `start_restore` / `restic_backups_tunnel`. The pattern is "dependent resource under a hierarchical path emits NOT NULL parent FK, but the test fixture doesn't supply a value."
- **This is a dedup hit — comment on #1063, not a new issue.**
- **Evidence:** Run `go test ./internal/store/` on this CLI; 3 failures listed above.
- **Cross-API examples now reaching 2 with evidence:** ElevenLabs (`voices_id`, in #1063), Servosity (`restic_backups_id` × 3 tables, this run).

### F4 — SKILL Phase 3 build checklist doesn't warn about `--dry-run` flag shadowing on novel commands (skill instruction gap)

- **What happened:** I built `triage`, `clear`, `stale-issues` (the 3 mutating novel commands) and added a local `--dry-run` flag defaulted to `true` on each one for production safety. Cobra accepted the declaration without complaint, but the local flag silently shadowed the global persistent `--dry-run` (declared at root, default false). My `dryRunOK(flags)` short-circuit reads from `flags.dryRun` (the global), so the local flag never reached it — and verify probes set the global, which my code wasn't checking against. Lost ~15 minutes debugging and refactored to "PLAN mode by default; `--confirm` to mutate" semantics.
- **Scorer correct?** N/A.
- **Root cause:** `skill` — the Phase 3 build checklist documents *some* anti-patterns for novel commands ("MUST NOT use `Args: cobra.MinimumNArgs(N)` or `MarkFlagRequired(...)`") but doesn't warn about the related footgun: declaring a local `--dry-run` flag that shadows the global persistent root flag.
- **Cross-API check:** Any agent building novel commands that need mutation safety. The temptation to declare a local `--dry-run` defaulted to true is natural — it's the obvious way to express "this command defaults to safe." But it's wrong, because the verify probes pass the global `--dry-run`, and `dryRunOK(flags)` reads the global.
- **Frequency:** every API where the agent builds mutating novel commands.
- **Fallback if the Printing Press doesn't fix it:** Agent debugs the shadowing (as I did), refactors. Costs 15-30 minutes per CLI that needs mutating novel commands.
- **Worth a Printing Press fix?** Yes, as a SKILL clarification — cheap and prevents repeated re-discovery.
- **Inherent or fixable:** Fixable.
- **Durable fix:** Add a one-paragraph note in the Phase 3 build checklist's "Verify-friendly RunE template" section: "Do NOT declare a local `--dry-run` flag on novel commands. The global persistent `--dry-run` is set at the root (default false); a local flag with the same name shadows the global silently. For 'PLAN mode by default' semantics on mutating novel commands, use a `--confirm` opt-in flag (default false) and gate mutation on `confirm && !dryRunOK(flags)`. The global `--dry-run` then becomes an extra safety net that forces PLAN mode even when `--confirm` is passed."
- **Test:** A simple cross-CLI sanity check: grep `internal/cli/*.go` in every recently-printed CLI for `BoolVar.*"dry-run"` declared on a child command — should be zero matches. Add the check to `verify-skill` if it isn't already.
- **Evidence:** This run; refactor commits visible in the working dir diff. Specifically the `var dryRun bool` declarations I had to remove from `triage.go`, `clear.go`, `stale_issues.go`.
- **Step G case-against:** "The agent should have known the SKILL already documents the global `--dry-run` flag; this is RTFM-level." Counter: the global flag isn't visible from inside a child command's flag-declaration block, so a careful agent who reads the existing SKILL build checklist literally won't see the warning. The case-for is "add one paragraph to prevent every future agent from re-discovering this."
- **Related prior retros:** None matched directly. #1130 is in skill but a different topic. Filing as a fresh skill clarification.

## Prioritized Improvements

### P2 — Medium priority

| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---|---|---|---|---|---|---|
| F2 | `endpoint_tools: hidden` doesn't suppress endpoint mirrors from `tools-manifest.json` | generator | subclass:code-orchestration-large-surface | Polish authors per-CLI overrides | small | Skip entries from manifest when the Cobra parent is hidden under code-orch |
| F3 | Typed-upsert tests fail NOT NULL on parent-FK columns (servosity adds `restic_backups_id` ×3 to #1063's `voices_id` evidence) | generator | every API with NOT NULL parent-FK sub-resources | Tests fail per-CLI; agent must skip or fix | already filed | (comment on #1063) |

### P3 — Low priority

| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---|---|---|---|---|---|---|
| F1 | `swagger: "2.0"` specs silently mis-parsed by the loader | openapi-parser | subclass:swagger2 | Agent must detect and pre-convert; silent failure if missed | small | None — affects only Swagger 2 input |
| F4 | SKILL Phase 3 build checklist doesn't warn about `--dry-run` flag shadowing on novel commands | skill | every API where agent builds mutating novel commands | Each agent re-debugs (~15-30 min) | small | None — pure doc clarification |

### Skip

| Finding | Title | Why it didn't make it (Step B / Step D / Step G) |
|---|---|---|
| K' | `--read-only` flag on `dogfood --live` for admin-scope tokens | Step B: only 1 API named with evidence (Servosity, this run); the Phase 5 dialogue already routes admin-scope users to Quick via user choice — adding a flag is bloat on a working flow. |
| C | publish-validate phase5 strict count | Step G: the strict count is correct behavior. Agents shouldn't be able to silently override Phase 5 failures by setting status=pass. My acceptance file having `tests_failed: 1` was MY responsibility to correct (I did); the tool's strictness is a feature. |

### Dropped at triage

| Candidate | One-liner | Drop reason |
|---|---|---|
| D | Framework `stale` command collides with domain-specific `stale` | printed-CLI: I renamed to `stale-backups`; per-CLI naming choice, no machine fix |
| F | `/issues/?state=ACTIVE` flag not exposed | API-quirk: Servosity's spec doesn't declare `state`; this is a spec coverage gap on Servosity's side, not a generator issue |
| G | Verify-friendly RunE template — required positional args | already-documented: the SKILL Phase 3 build checklist already warns about `cobra.MinimumNArgs/ExactArgs` |
| H | `--select` example paths sometimes return `{}` | printed-CLI: research.json error (my example used paths that don't match the output structure); not a generator gap |
| I | Display name "Servosity Api" without `x-display-name` | printed-CLI: I set `x-display-name: Servosity` in my converted spec; works as designed when set; the slug-to-title fallback is a known minor issue |
| M | "Existing-tool awareness" — should the SKILL ask about adjacent repos? | per-CLI: user explicitly pointed me at `~/Documents/Dev/servosity-toe` mid-session; this worked, but generalizing the prompt is per-CLI |

## Work Units

### WU-1: Swagger 2.0 detection + auto-conversion in openapi parser (from F1)
- **Priority:** P3
- **Component:** openapi-parser
- **Goal:** When the loader receives a `swagger: "2.0"` spec, auto-convert to OpenAPI 3 instead of silently misparsing.
- **Target:** `internal/openapi/parser.go:880` (`loadOpenAPIDoc`)
- **Acceptance criteria:**
  - positive: `loadOpenAPIDoc` on a Swagger 2.0 JSON file returns an `*openapi3.T` whose `Servers[0].URL` is `<scheme>://<host><basePath>` (from the Swagger 2 fields) and whose `Components.SecuritySchemes` is populated from `securityDefinitions`.
  - negative: `loadOpenAPIDoc` on an OpenAPI 3 spec behaves unchanged (test the existing happy path).
  - error: `loadOpenAPIDoc` on a Swagger 2 spec that fails conversion returns a clear "convert this spec to OpenAPI 3 before generating" error rather than silently dropping fields.
- **Scope boundary:** Does NOT include translating `x-*` Swagger-2-specific extensions (those are spec-author responsibility once converted). Does NOT include propagating Swagger 2 paths' `consumes`/`produces` arrays to operation-level `requestBody.content` / `responses.content` — that's `kin-openapi/openapi2conv`'s responsibility.
- **Dependencies:** None.
- **Complexity:** small (10 lines + import; conversion library does the work).

### WU-2: Suppress hidden endpoint mirrors from tools-manifest.json under code-orchestration (from F2)
- **Priority:** P2
- **Component:** generator
- **Goal:** When the spec declares `mcp.endpoint_tools: hidden`, the emitted `tools-manifest.json` should not include the hidden endpoint-mirror entries.
- **Target:** Generator templates that emit `tools-manifest.json` (under `internal/generator/templates/mcp/`) and the runtime walker that decides which tools are hidden (`internal/mcp/cobratree/`).
- **Acceptance criteria:**
  - positive: generate a >50-tool CLI with `mcp.endpoint_tools: hidden`; assert `tools-manifest.json` contains the orchestration tools (`<api>_search`, `<api>_execute`) + novel commands + framework tools, but NOT the per-endpoint typed-tool mirrors.
  - negative: generate the same spec WITHOUT `mcp.endpoint_tools: hidden`; assert the per-endpoint mirrors ARE in the manifest.
  - scorer: re-run `tools-audit` and `scorecard mcp_description_quality` on the positive case; both should pass without per-CLI overrides.
- **Scope boundary:** Does NOT touch the runtime walker's hidden-tool predicate — that already works correctly. Does NOT change the user-visible behavior of the agent surface; the only effect is on the on-disk manifest.
- **Dependencies:** None.
- **Complexity:** small (one predicate, one template guard).

### WU-3: Comment on #1063 with Servosity's parent-FK NOT NULL evidence (from F3)
- **Priority:** P2 (matching #1063's existing priority)
- **Component:** generator (matching #1063)
- **Goal:** Add Servosity's reproduction to #1063 so the maintainer has a second concrete API to test the fix against.
- **Target:** GitHub comment on issue #1063.
- **Acceptance criteria:**
  - The comment names the three failing tests on Servosity (`start_backup`, `start_restore`, `restic_backups_tunnel`), the missing column (`restic_backups_id`), and the path-derivation source (`/restic-backups/{id}/start-backup/` etc).
  - Provides a reproduction recipe: `printing-press generate --spec <converted-servosity-openapi-v3.json> --output /tmp/servosity-test --force --lenient && go test -C /tmp/servosity-test ./internal/store/`.
- **Scope boundary:** Does NOT propose a new fix — #1063 already has the analysis. The comment just adds evidence.
- **Dependencies:** None.
- **Complexity:** small.

### WU-4: SKILL warning against local `--dry-run` flag declaration on novel commands (from F4)
- **Priority:** P3
- **Component:** skill
- **Goal:** Prevent agents from re-discovering the global-persistent-flag-shadowing footgun when building mutating novel commands.
- **Target:** `skills/printing-press/SKILL.md` Phase 3 "Verify-friendly RunE template" section.
- **Acceptance criteria:**
  - positive: a fresh agent reading the Phase 3 build checklist before authoring a `triage`-shaped command sees the warning AND the suggested "PLAN mode by default; `--confirm` to mutate" recipe.
  - reinforcing test: `verify-skill` could grep `internal/cli/*.go` for `BoolVar.*"dry-run"` on child commands — should be zero matches. This is a stretch goal; the SKILL clarification alone is the minimum.
- **Scope boundary:** Does NOT propose a generator template helper (that's a larger change). Does NOT propose a new linter; just documentation in the existing checklist.
- **Dependencies:** None.
- **Complexity:** small (one paragraph in SKILL.md).

## Anti-patterns

- **Agent-level:** Declaring a local `--dry-run` flag with default=true on a mutating command. The global persistent flag shadows your local declaration silently.
- **Agent-level:** Trusting `kin-openapi` to handle `swagger: "2.0"` input. It accepts the JSON, parses paths, drops everything else.
- **Generator-level:** Emitting endpoint-mirror entries in `tools-manifest.json` even when `endpoint_tools: hidden` is set. The agent never sees these tools at runtime; the manifest noise inflates the scorer penalty and creates phantom overrides work in polish.

## What the Printing Press Got Right

- **`x-prefix` on apiKey schemes** translated the `Token <key>` prefix cleanly once the spec was converted to OpenAPI 3. The generated `applyAuthFormat("Token {token}", ...)` worked first try against live API.
- **`x-mcp` enrichment** — once added to the spec, the Cloudflare pattern emission worked end-to-end. Runtime agent surface is clean.
- **The Phase 1.5 absorb-gate workflow** — the subagent novel-features brainstorm landed 8 high-quality candidates from concrete persona analysis. After examining `servosity-toe` (the real support tooling), 2 more were added with strong evidence. The user's approval gate caught the "trim scope?" decision cleanly.
- **The PRODUCTION SAFETY contract** — every mutating novel command short-circuits on `cliutil.IsVerifyEnv()` AND defaults to PLAN mode. Phase 5 live dogfood was GET-only; the SKILL's prompt for depth let the user enforce "no writes against prod" cleanly.
- **`--force` regenerate with AST merge** — preserved my hand-written `attention.go`, `triage.go`, etc. across two regenerations after I edited research.json. The 10 AddCommand calls in root.go and the 16 hand-edited files all came back untouched.
- **`shipcheck` umbrella** — running all 6 legs in one command made the fix loop tight. The per-leg verdict summary surfaced which legs failed (verify-skill, validate-narrative) and which dimensions needed attention.
- **Polish self-awareness** — polish iteration 1 correctly diagnosed "the remaining gap is in the phase5 acceptance file authored by the main SKILL, not in polish-modifiable code." That clear scope boundary kept polish from inventing scaffolding.
- **The /current-user/ kebab-case path** worked first try when I corrected my own snake_case typo. Generated `/current-user/` matches the spec verbatim.
