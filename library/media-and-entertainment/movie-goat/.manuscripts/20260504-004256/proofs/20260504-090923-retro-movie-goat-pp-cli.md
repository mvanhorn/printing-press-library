# Printing Press Retro: movie-goat

## Session Stats

- API: movie-goat (TMDb v3 primary + OMDb enrichment, multi-source CLI)
- Spec source: internal YAML (carried forward from prior `20260411-000047` run, auth-shape patched)
- Mode: reprint (printing-press 1.3.2 → 3.8.0)
- Scorecard: 84/100 (Grade A; +1 from polish)
- Verify pass rate: 100% (29/29 in 3 fix loops)
- Live dogfood pass rate: 171/191 (89.5%) at level=full
- Fix loops: 4 (multi/tv-seasons usage, framework Examples, framework JSON envelopes, sync trailing-line)
- Manual code edits: 17+ patches across the printed CLI
- Features built from scratch: 8 novel commands (`tonight`, `ratings`, `marathon`, `career`, `versus`, `watchlist`, `queue`, `collaborators`) + sibling `internal/omdb` package

## Findings

### 1. Generator templates double-print binary in usage errors (Bug)

- **What happened:** `movie-goat-pp-cli multi` (no query) emitted `Usage: movie-goat-pp-cli movie-goat-pp-cli multi <query>` — binary name twice. Same for `tv seasons get` and any other endpoint command with required positional missing.
- **Scorer correct?** N/A (this is a generator emit bug, not a score penalty).
- **Root cause:** Two templates use `cmd.Root().Name(), cmd.CommandPath()` together in the usage error. `cmd.CommandPath()` already begins with the root command name, so concatenation duplicates it.
  - `internal/generator/templates/command_promoted.go.tmpl:95`
  - `internal/generator/templates/command_endpoint.go.tmpl:128`
- **Cross-API check:** Yes, every printed CLI with required positionals on promoted commands or typed endpoint commands ships this defect. That's most CLIs in the catalog.
- **Frequency:** every API.
- **Fallback if the Printing Press doesn't fix it:** Claude rarely catches it without manually invoking the missing-positional error path. Effectively 100% defect rate on CLIs not seen by an agent reasoning through usage errors.
- **Worth a Printing Press fix?** Yes — trivially.
- **Inherent or fixable:** Fixable, two-line change.
- **Durable fix:** In both templates, change the format string and arg list to use only `cmd.CommandPath()`:
  ```go
  return usageErr(fmt.Errorf("{{.Name}} is required\nUsage: %s <%s>", cmd.CommandPath(), "{{.Name}}"))
  ```
- **Test:** Generate any spec with a required positional. Run `<cli> <subcommand>` (no args). Expected: `Usage: <cli> <subcommand> <name>` (one binary). Negative: assert no double-binary in the message.
- **Evidence:** This run patched `internal/cli/promoted_multi.go:31` and `internal/cli/tv_seasons_get.go:33` after dogfood revealed the doubled name. The same exact pattern is in both templates and emits across every CLI.
- **Related prior retros:** None match exactly. The dub retro proposed orthogonal Example-quality work; not the same finding.

### 2. Framework command templates emit no `Example:` field (Template gap)

- **What happened:** Generator-emitted `doctor`, `profile save/use/list/show/delete`, and `feedback list` ship without `Example:` strings. Dogfood help-check requires them and fails for every CLI.
- **Scorer correct?** Yes — examples are a user-facing CLI quality dimension.
- **Root cause:** Three templates lack the field:
  - `internal/generator/templates/doctor.go.tmpl`
  - `internal/generator/templates/profile.go.tmpl` (covers save/use/list/show/delete)
  - `internal/generator/templates/feedback.go.tmpl`
- **Cross-API check:** 100% — every printed CLI has these framework commands.
- **Frequency:** every API.
- **Fallback if the Printing Press doesn't fix it:** Claude doesn't audit framework-command help text by default; ~7 dogfood failures per CLI go uncorrected unless a fix loop fires (cost: ~30s of token-time per CLI to identify and patch).
- **Worth a Printing Press fix?** Yes — single-place template edit, broad impact.
- **Inherent or fixable:** Fixable trivially.
- **Durable fix:** Add canonical `Example:` strings to each framework template, using the binary-name placeholder convention already used elsewhere:
  ```
  Example: `  {{ .BinaryName }} doctor
    {{ .BinaryName }} doctor --json`
  ```
  And similar for profile subcommands (save/use/list/show/delete) and feedback list.
- **Test:** Generate any spec; run `<cli> doctor --help`, `<cli> profile save --help`, `<cli> feedback list --help`; assert each contains `Examples:`. Negative: assert the example commands actually parse (no inline `#` comments — see Skip section).
- **Evidence:** This run patched all 7 commands directly in the printed CLI (auth.go, doctor.go, feedback.go, profile.go).
- **Related prior retros:** None match exactly. food52 retro identified verify-mock issues with `strings.TrimSpace` example formatting; orthogonal.

### 3. Framework command templates ignore `--json` flag (Template gap)

- **What happened:** Many framework commands (`auth status`, `auth logout`, `auth set-token`, `api`, `import`, `profile delete`, `tail` with no resource, `which`, `multi` with no query, `sync` trailing summary) emit human prose unconditionally. The `--json` flag is silently ignored. Dogfood `json_fidelity` test fails on each.
- **Scorer correct?** Yes — `--json` is a contract every command in the CLI advertises (it's a global persistent flag) and an MCP host depending on JSON output relies on consistent shape.
- **Root cause:** Each framework template's RunE uses `fmt.Fprintf` / `fmt.Fprintln` against `cmd.OutOrStdout()` without checking `flags.asJSON`. No JSON envelope path exists.
- **Cross-API check:** 100% — every printed CLI has these framework commands and the same `--json` gap.
- **Frequency:** every API.
- **Fallback if the Printing Press doesn't fix it:** This CLI patched ~9 individual commands by hand. Each future CLI ships with the same defect unless an agent goes through the same manual loop.
- **Worth a Printing Press fix?** Yes — high frequency, broad impact, and any MCP host calling these tools gets human prose where it expects JSON.
- **Inherent or fixable:** Fixable. Each template needs an `if flags.asJSON { return printJSONFiltered(...envelope...) }` branch before the human prose path.
- **Durable fix:** Add the JSON branch to each framework command template:
  - `auth.go.tmpl` — status (`{authenticated, source, config}`), logout (`{cleared, note}`), set-token (`{saved, config_path}`)
  - `api_discovery.go.tmpl` — `{interfaces, note}`
  - `import.go.tmpl` — `{succeeded, failed, skipped}`
  - `profile.go.tmpl` — delete (`{deleted: name}`)
  - `tail.go.tmpl` — no-resource help envelope (`{resources, note}`)
  - `which.go.tmpl` — `{matches: []}` envelope (empty array on no match, exit 0 under --json)
  - sync template — suppress trailing human "Sync complete" line under --json (the `sync_summary` event already carries the same data)
- **Test:** For each command, assert `<cli> <cmd> --json` produces output that `jq .` parses as a single JSON value, exit 0. Negative: human path must remain unchanged when `--json` is absent.
- **Evidence:** This run patched all 9 commands directly in the printed CLI; before patching, dogfood reported `--json` failures for each.
- **Related prior retros:** None match exactly. The dub retro is orthogonal (Example quality, not JSON envelope).

### 4. Live dogfood matrix uses generator's "example-value" placeholder for camelCase ID positionals (Scorer bug + generator gap)

- **What happened:** Five flagship typed-endpoint commands (`movies get`, `people get`, `tv get`, `tv seasons get`, `export`) failed live dogfood happy_path AND json_fidelity because the matrix probed them with literal `example-value` as the positional id. TMDb correctly 404'd; tests recorded as failures.
- **Scorer correct?** Partially. The matrix is doing the right thing (extract example args from `<cli> --help`), but the generator emits `example-value` as the placeholder for ID-typed positionals when the param name is camelCase (e.g., `movieId`, `seriesId`, `personId`).
- **Root cause:** `internal/generator/generator.go:exampleValue` (line 2944) checks `strings.HasSuffix(nameLower, "_id") || nameLower == "id"`. A camelCase param name like `movieId` lowers to `movieid`, which doesn't have an underscore and isn't exactly `id`, so the check falls through to the default `"example-value"`. The fix to recognize camelCase ID suffixes is one regex tweak.
- **Cross-API check:** Yes — every CLI with camelCase ID positionals hits this. From the catalog: notion (page IDs as `pageId`), linear (`issueId`), espn (`teamId`, `eventId`), pagliacci (`storeId`). Spec authors using OpenAPI-generated names get camelCase by convention (TMDb's spec is the same).
- **Frequency:** every API with camelCase ID positionals from OpenAPI specs (most APIs).
- **Fallback if the Printing Press doesn't fix it:** The fixed UUID-shape placeholder still won't match any real API's id format (TMDb is numeric, Stripe is `cus_xxx`-style, etc.), but it would at least look like a placeholder rather than the literal string `example-value` that some scorers might mistake for a hint to substitute. The deeper fix needs a second tier (see below).
- **Worth a Printing Press fix?** Yes for the camelCase recognition (one-line). The deeper fix of "actually substitute a real id at test time" is bigger but proportionally important.
- **Inherent or fixable:** Two-tier fixable.
  - Tier 1 (immediate): in `exampleValue`, also accept `HasSuffix(nameLower, "id") && len(nameLower) > 2` so `movieId`/`seriesId`/`personId` get the UUID-shape placeholder. Doesn't fix the API 404 but normalizes shape.
  - Tier 2 (durable): the live dogfood matrix can run a list-then-get probe. Before testing `<resource> get <id>`, run a list/popular/trending command for that resource, parse the first id from the JSON, and use it as the test fixture. This makes happy_path/json_fidelity actually exercise the get path on real data.
- **Durable fix:** Implement both tiers. Tier 1 is a generator change; Tier 2 is a scorer change in `internal/pipeline/live_dogfood.go:liveDogfoodHappyArgs`.
- **Test:**
  - Tier 1: spec with `movieId` positional → emitted Example uses UUID-shape placeholder, not `example-value`. Negative: snake_case `movie_id` still produces UUID (existing behavior preserved).
  - Tier 2: live dogfood for a resource family (e.g., `movies`) runs `popular --json` first, takes `.results[0].id`, passes it to `get <id>`. Assert the get-test passes (200 OK, valid JSON). Negative: when the resource has no list/popular companion, fall back to skip-with-warn rather than blind `example-value` test.
- **Evidence:** Five tests with literal `example-value` failed in this run. Inspection of `exampleValue` (generator/generator.go:2944) showed the camelCase miss; inspection of `liveDogfoodHappyArgs` (live_dogfood.go) showed the matrix uses the generator's example output as-is.
- **Related prior retros:** food52 retro #20260427-014521 — `aligned`. Proposed canonical mock values for verify positionals; this retro extends the pattern to live dogfood happy_path/json_fidelity. Both retros propose the same general direction (parse `--help` for positionals, supply real fixtures); the food52 retro covers verify-mock subprocesses, this one covers live-dogfood subprocesses. The mechanism (`canonicalargs` registry, paramDefaults from spec) exists already but is not wired into `liveDogfoodHappyArgs`.

### 5. Live dogfood error_path expects non-zero exit on empty search (Scorer bug, design philosophy)

- **What happened:** The matrix's error_path test runs every command with `__printing_press_invalid__` and expects a non-zero exit. Six commands (`movies search`, `people search`, `tv search`, `multi`, `search`, `auth set-token`) failed because they correctly return exit 0 with empty results — canonical Unix UX.
- **Scorer correct?** Partially. Testing exit-on-error is right for mutating commands (DELETE non-existent, POST malformed body); wrong for read-side search/list commands where empty-result-on-no-match is the canonical behavior. Forcing search to exit non-zero would break pipelines like `<cli> search "x" --json | jq '.results[0]'`.
- **Root cause:** `internal/pipeline/live_dogfood.go` error_path test treats all commands uniformly. There's no `kind`-based dispatch (read vs mutation) for the error_path strategy.
- **Cross-API check:** Yes — every printed CLI has search/list commands. From the catalog: notion (search), linear (issue search), espn (event search). All affected.
- **Frequency:** every API with search.
- **Fallback if the Printing Press doesn't fix it:** Each affected CLI ships with 4-6 fake "errors" in dogfood reports. Polish skill currently labels these as known mismatches and skips. Real defect rate undetected; signal weakened.
- **Worth a Printing Press fix?** Yes — accuracy of the scorer matters because polish/promote gates trust it.
- **Inherent or fixable:** Fixable.
- **Durable fix:** In `liveDogfoodCommandResults` (live_dogfood.go), when generating the error_path test, branch on `command.Kind` and `commandSupportsSearch(command.Help)` (heuristic: command has `--query` flag or its `Use:` includes `<query>`). Skip error_path for read+search commands, or substitute "expected non-zero exit" for "expected exit 0 with empty results array under `--json`".
- **Test:** Generate any spec with a search command. Run live dogfood with `__printing_press_invalid__` query. Read+search commands should pass (exit 0 OK). Mutation commands with bogus payloads should still fail (exit non-zero expected).
- **Evidence:** 6 error_path failures in this run, all on search-family commands. Source inspection of `live_dogfood.go` confirms uniform error_path strategy.
- **Related prior retros:** None directly. Recipe-goat retro raised a different verify regex bug (bracketed flag descriptors); orthogonal.

### 6. Sync command emits trailing human line under `--json` (Template gap)

- **What happened:** `<cli> sync --json` emits per-line JSON events (`sync_start`, `sync_progress`, `sync_complete`, `sync_summary`, `sync_warning`) — useful for streaming agents — followed by a human-readable `Sync complete: 37 records across 6 resources (0.1s)` line. Test infra parsing as a single JSON document fails.
- **Scorer correct?** Yes — when `--json` is set, output should be JSON-only.
- **Root cause:** The sync template's RunE prints the human summary unconditionally after the loop completes.
- **Cross-API check:** Yes — every printed CLI has sync.
- **Frequency:** every API.
- **Fallback if the Printing Press doesn't fix it:** Each CLI's `sync --json` output is unparseable as a single JSON value; agents reading `last line of output` get the human line, which is data corruption for downstream tooling. Polish addresses it per-CLI but it recurs.
- **Worth a Printing Press fix?** Yes.
- **Inherent or fixable:** Fixable.
- **Durable fix:** In the sync template, gate the trailing human prose on `!flags.asJSON`. The `sync_summary` event already carries `total_records`, `resources`, `success`, `warned`, `errored`, `duration_ms` — so suppressing the human line under JSON loses no information.
- **Test:** `<cli> sync --json` output must satisfy: split on `\n`; each non-empty line parses as a JSON object; the last non-empty line is `{"event": "sync_summary", ...}`. Negative: human path must still print "Sync complete: ..." when `--json` is absent.
- **Evidence:** Patched in this run's printed CLI sync.go. Same fix would apply to every emitted sync.
- **Related prior retros:** None match.

## Prioritized Improvements

### P1 — High priority

| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---|---|---|---|---|---|---|
| 1 | Generator templates double-print binary in usage errors | generator | every API | low (Claude rarely audits usage strings) | small | none |
| 3 | Framework command templates ignore `--json` flag | generator | every API | low (silent prose under --json corrupts MCP output) | medium (multiple templates) | none |
| 4 | Live dogfood example-value placeholder for camelCase ID positionals | generator + scorer | every API with camelCase IDs | low (false-fail noise drowns real signals) | medium (two-tier: generator regex + scorer probe) | guard Tier 2 on existence of list/popular companion endpoint |

### P2 — Medium priority

| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---|---|---|---|---|---|---|
| 2 | Framework command templates emit no `Example:` field | generator | every API | medium (dogfood help-check catches it but only if run) | small | none |
| 5 | Live dogfood error_path expects non-zero on empty search | scorer | every API with search | medium (false-fails are visible but ignorable) | small | branch on command.Kind + search heuristic |
| 6 | Sync trailing human line under `--json` | generator | every API | low (data-corruption for agents reading last line) | small | none |

### Skip

| Finding | Title | Why it didn't make it |
|---|---|---|
| 7 | Inline `#` shell-comment in Cobra Examples breaks dogfood matrix | Step B: only 1 API (this run) with concrete evidence; other CLIs would need to use `#` comments in Examples to surface the bug, and the convention isn't widespread enough across the catalog to justify a generator/scorer change. Polish skill could add a one-line warning to authoring guidance instead. |

### Dropped at triage

| Candidate | One-liner | Drop reason |
|---|---|---|
| Spec auth.type was `bearer_token` (wrong for v3 keys) | Prior CLI had hand-coded length-check workaround in client.go because v3 TMDb keys are 32-char hex, not Bearer tokens | printed-CLI / spec authoring (the prior agent's spec was wrong; revalidation caught it) |
| `search` reserved resource name collision | Generator clearly errored with rename suggestion; agent renamed to `multi`. Worked. | API-quirk (generator handled correctly) |
| Pre-generation MCP enrichment threshold prompt | Skill prompted at 30-50 band correctly | iteration-noise (worked as designed) |
| Polish caught `collaborators` count/titles + `marathon` unreleased entries | Real CLI bugs in agent-authored novel commands | printed-CLI (specific to logic written this run) |
| `lock promote` rejects level=full with any failure | Rooted in dogfood matrix accuracy | printed-CLI (folds into F4/F5 once those land) |
| LSP/gopls workspace noise during agent-authored Go code | `go build`/`go test` are authoritative, LSP false positives per AGENTS.md | iteration-noise |
| `auth set-token __printing_press_invalid__` accepts any string | TMDB doesn't publish a v3 key format pattern; agent could argue either way | unproven-one-off (industry convention to accept any token string) |
| `which "stale tickets"` no-match | Linear-style query; CLI correctly returns exit 2 with no-match message | API-quirk (test fixture chose Linear-shaped query for movie CLI) |

## Work Units

### WU-1: Generator template polish — Usage line + framework Examples + framework JSON envelopes + sync trailing line (from F1, F2, F3, F6)

- **Priority:** P1 (max of absorbed findings; F1 and F3 are P1).
- **Component:** generator
- **Goal:** Fix four template defects so emitted CLIs have correct Usage strings, Examples on framework commands, JSON envelopes on framework commands, and clean `--json` output from sync.
- **Target:** `internal/generator/templates/`
  - `command_promoted.go.tmpl:95`, `command_endpoint.go.tmpl:128` (F1)
  - `doctor.go.tmpl`, `profile.go.tmpl`, `feedback.go.tmpl` (F2)
  - `auth.go.tmpl`, `api_discovery.go.tmpl`, `import.go.tmpl`, `profile.go.tmpl`, `tail.go.tmpl`, `which.go.tmpl`, sync template (F3, F6)
- **Acceptance criteria:**
  - Positive (F1): generate a CLI with a required-positional endpoint; run `<cli> <subcmd>` (no args). Output's Usage line includes the binary name exactly once.
  - Negative (F1): no regression in commands without required positionals.
  - Positive (F2): every framework command shows an `Examples:` block in `--help` after generation.
  - Positive (F3): every listed framework command's `--json` output is parseable by `jq .` and is a single JSON value.
  - Negative (F3): human path unchanged when `--json` is absent.
  - Positive (F6): `<cli> sync --json` output's last non-empty line is `{"event":"sync_summary",...}`. No trailing human prose.
  - Negative (F6): `<cli> sync` (no flag) still prints "Sync complete: ..." human line.
- **Scope boundary:** Does NOT include the live dogfood matrix changes (those are WU-2). Does NOT touch `--json` paths on endpoint-mirror commands (those already work).
- **Dependencies:** None.
- **Complexity:** medium (multiple template files but each edit is small; can be done in one PR).

### WU-2: Live dogfood matrix accuracy — camelCase ID example values + error_path command-kind dispatch (from F4, F5)

- **Priority:** P1 (F4 is P1).
- **Component:** scorer (primary) + generator (Tier 1 of F4).
- **Goal:** Reduce dogfood matrix false-fail noise so the score signal is trustworthy on every CLI with id-positional commands and search-family commands.
- **Target:**
  - `internal/generator/generator.go:exampleValue` (line 2944) for Tier 1 of F4 (camelCase ID recognition).
  - `internal/pipeline/live_dogfood.go:liveDogfoodHappyArgs` for Tier 2 of F4 (list-then-get probe).
  - `internal/pipeline/live_dogfood.go` error_path generation for F5 (kind-based dispatch).
- **Acceptance criteria:**
  - Positive F4 Tier 1: spec with `movieId` positional → emitted Example uses UUID-shape placeholder (not `example-value`).
  - Positive F4 Tier 2: live dogfood for `<resource> get` runs `<resource> popular` (or `list`) first, takes `.results[0].id`, passes it to the get test. Assert get test passes when the API returns 200.
  - Negative F4 Tier 2: when no list companion exists, fall through to skip-with-warn rather than blind `example-value` test (still better than today).
  - Positive F5: live dogfood for a search command with `__printing_press_invalid__` query returns pass (exit 0 + empty results array under `--json` are both acceptable).
  - Negative F5: live dogfood for a mutating command (write/delete) with bogus body still fails error_path as today.
- **Scope boundary:** Does NOT remove all error_path coverage — only adapts strategy by command kind. Does NOT fix the `<resource>` placeholder used by `export` (separate concern, lower frequency).
- **Dependencies:** None.
- **Complexity:** medium-large (generator regex is small; scorer probe + dispatch is moderate; needs tests).

## Anti-patterns

- Authoring Cobra `Example:` blocks with inline `# comment` annotations — the dogfood matrix's whitespace-split arg parser includes `#` and trailing words as positional arg values, breaking happy_path tests. Convention going forward: either use a separate line comment above the example, or omit the comment entirely.

## What the Printing Press Got Right

- **Reprint-aware research reuse and reconciliation.** The novel-features subagent's Pass 2(d) ingested the prior `research.json`, re-scored 6 prior features, kept 4, reframed 1 (`get` → `ratings`), and dropped 1 (`watch` is now table-stakes). The 3 net-new features (Watchlist, Recommendation Queue, Recurring Collaborators) emerged from persona-driven brainstorming without re-doing all of Phase 1 from scratch.
- **Pre-generation MCP enrichment prompt at the 30-50 band.** Caught the spec at the right moment to add `mcp.transport: [stdio, http]` (and surfaced intents as a real choice the user could weigh; intents weren't needed since cobratree-walked novel commands already cover the multi-step flows).
- **Reserved resource name collision detection.** Generator failed cleanly with `"search" collides with a reserved Printing Press template; rename to e.g. search_resource` instead of producing duplicate symbols. One-rename fix.
- **Auth-shape correction (machine validation worked).** The internal YAML parser supports `auth.type: api_key, in: query, header: api_key`. Once the spec was patched, generation produced clean `?api_key=…` query auth without any post-gen hack — the prior CLI's length-check workaround in client.go is gone.
- **Polish skill's output review caught two real CLI bugs.** `collaborators` count/titles inconsistency (count was per-credit-row but titles deduped) and `marathon` including unreleased franchise entries with runtime=0. Both are agent-authored logic bugs, not generator gaps; polish's diagnose-fix-rediagnose loop is the right place to catch them and it did.
- **Cobratree-walked novel commands automatically appear as MCP tools.** All 8 hand-written transcendence commands became MCP tools at runtime without any spec-level intent definitions. The `mcp:read-only: true` annotation pattern worked exactly as documented.
- **Live scorecard `--live-check` produced production-grade real-API output for all 8 novel features.** Apex on Netflix, Fight Club's full 4-source ratings card, Avengers Collection 614-min plan, Christopher Nolan's directing career with cross-source ratings — all sampled live.
