# Printing Press Retro: youtube

## Session Stats
- API: youtube (YouTube Data API v3)
- Spec source: apis-guru OpenAPI 3.0 (Google Discovery doc-derived), trimmed to 5 read-only paths
- Scorecard: 71/100 (Grade B) after polish (was 74/100 pre-polish; auth-mismatch and type-fidelity gaps dominate)
- Verify pass rate: 100%
- Fix loops: 2 (shipcheck re-run after research.json path correction; narrative example simplification)
- Manual code edits: 3 categories — spec rewrite for auth + inline params, post-generation patch of `--part`/`--max-results` flags onto 5 generator commands, novel-feature scaffolding (~950 LOC across 4 files)
- Features built from scratch: 4 novel commands (search-bulk, videos-transcript, videos-embed, videos-related)

## Findings

### F1. Generator's prevalence-based "global param" filter strips required params on narrow specs (template gap)

- **What happened:** The trimmed YouTube spec has 5 endpoints, and apis-guru's Google Discovery–derived layout inherits 14 query params (including `part`, `maxResults`, `pageToken`, `key`, `fields`, `prettyPrint`) on every endpoint via `$ref`. The generator's prevalence filter strips anything present on 5/5 endpoints. Result: `part` (marked `required: true` in the spec) is filtered from every generated command flag surface. The first real call returned ID-only YouTube results (no titles, no snippets, no thumbnails) because the API request didn't include `part=snippet`.
- **Scorer correct?** N/A — no scorer flagged this. It surfaced during manual testing of the generated commands.
- **Root cause:** The global-param filter in the generator treats prevalence as the sole signal. A param required by the API for the response to be useful gets the same treatment as `prettyPrint` or `callback`.
- **Cross-API check:** Affects any spec where a required param appears on a high fraction of endpoints. Google Discovery-derived specs (`info.x-origin[].format == "google"`) all carry the same global-param block: `key`, `fields`, `prettyPrint`, `quotaUser`, `alt`, `callback`, `oauth_token`, `upload_protocol` plus method-specific ones like YouTube's `part`. The pattern is shared because Google's Discovery framework emits the same parameter group across services.
- **Frequency:** every narrowly-scoped Google API + any spec with cross-endpoint required params (subclass:`required-params-share-prevalence`)
- **Fallback if the Printing Press doesn't fix it:** Claude has to (a) notice the filter warnings during generation, (b) test enough real calls to detect minimal results, (c) hand-patch the generator commands to add the flags back. Reliability ~50% — only catches when shipcheck or live testing reveals the issue. Two CLIs from Google APIs would each pay the same friction.
- **Worth a Printing Press fix?** Yes. The fix has a clean guard.
- **Inherent or fixable:** Fixable.
- **Durable fix:** In the generator's global-param filter, **never filter parameters marked `required: true` in the spec**. Optionally extend the guard to spec-declared `x-pp-keep-as-flag: true` or a built-in allowlist of common pagination params (`maxResults`, `pageToken`, `pageSize`, etc.) so they remain reachable even when prevalent. Hardcoding the YouTube `part` keyword is not the right fix — drive it from the spec.
- **Test:**
  - Positive: a 5-endpoint OpenAPI where one param has `required: true` on every endpoint generates commands that DO expose that param as a flag.
  - Negative: the same spec with all non-required global params (`prettyPrint`, `fields`) continues to filter those as before.
- **Evidence:** Generation warnings showed `filtered global query param "part" from generated commands: present on 5/5 endpoints`. Real `youtube videos-list --id <id>` returned `{"id": "...", "kind": "...", "etag": "..."}` — no snippet/statistics. The trimmed spec had `part: {required: true, in: query}` on every path.
- **Related prior retros:** None matched the global-param-filter pattern.

### F2. apis-guru / Google Discovery-derived specs lack API-key auth scheme (template gap + missing scaffolding)

- **What happened:** The YouTube spec from apis-guru declares only OAuth2 security schemes (`Oauth2`, `Oauth2c`), even though Google APIs also accept `?key=<API_KEY>` as auth — this is the standard read-only path. When my first trimmed spec dropped OAuth2 (the user has API key only, no OAuth client), the generator emitted "Auth: not required" and the CLI sent unauthenticated requests, which Google's edge returned 403 for. I had to rewrite the spec with a hand-authored `securitySchemes.ApiKey` (apiKey in query, name=`key`) and `x-auth-vars` mapping to `YOUTUBE_API_KEY`.
- **Scorer correct?** Partially — the scorecard reports `auth_protocol 4/10` because the spec still declares OAuth2 alongside the synthetic ApiKey scheme. Fixing the inference would also close the scorecard gap.
- **Root cause:** Google's Discovery doc → OpenAPI conversion at apis-guru doesn't model the `?key=` API-key path as a security scheme. The openapi-parser has no path to infer it from the origin metadata.
- **Cross-API check:** Every apis-guru spec where `info.x-origin[0].format == "google"` (i.e., converted from a Google Discovery doc) has the same shape. The catalog already lists `google-cloud-run`; any future Google API (Calendar, Drive, Gmail, Sheets, Maps, YouTube Analytics, etc.) inherits this auth gap.
- **Frequency:** every Google API generated from apis-guru — subclass:`google-discovery-origin`
- **Fallback if the Printing Press doesn't fix it:** Skill instruction in `Pre-Generation Auth Enrichment` documents the manual workaround, but it requires the agent to (a) detect the missing API-key scheme, (b) hand-write the YAML edit script, (c) verify the regen wires env-var lookup. Skill instructions get followed unreliably; ~70%.
- **Worth a Printing Press fix?** Yes. Detection signal is precise (`info.x-origin[0].format == "google"` or host pattern `*.googleapis.com`); fix scope is small.
- **Inherent or fixable:** Fixable.
- **Durable fix:** In the openapi-parser, detect Google-origin specs and auto-inject an `ApiKey` security scheme (`apiKey` in `query`, name `key`) with `x-auth-vars` pointing at a `<UPPER_SLUG>_API_KEY` env var when no API-key scheme already exists. Add top-level `security: [{ApiKey: []}]` only if the user-provided trim removed OAuth schemes (i.e., respect existing OAuth declarations; don't replace).
- **Test:**
  - Positive: generating from any apis-guru Google API spec results in a CLI where `doctor` reports `auth_source: env:<NAME>_API_KEY` and real GETs succeed with the env-var-set key.
  - Negative: non-Google specs (Stripe, Linear, Notion) generate without any synthetic ApiKey injection.
- **Evidence:** The trimmed spec's `components.securitySchemes` was empty after dropping OAuth2. `doctor` reported `Auth: not required`. First real `search-list` returned HTTP 403 "Method doesn't allow unregistered callers." Manual Python script re-added the ApiKey scheme and regen produced a working `auth_source: env:YOUTUBE_API_KEY`.
- **Related prior retros:** None matched. Closest existing issues are #1207 (multi-credential auth template) and #1333 (dual-scheme apiKey-header), both at the multi-scheme level, not the Google-Discovery-origin level.

### F3. `validate-narrative --full-examples` parses shell pipes as command args (scorer bug)

- **What happened:** Recipes in `research.json` like `youtube-pp-cli youtube videos-transcript dQw4w9WgXcQ --lang en --json --select "text" | head -c 2000` failed validation with `unknown shorthand flag: 'c' in -c`. The validator runs the full command line via `exec.Command(args[0], args[1:]...)` — the `|` and everything after end up as args to youtube-pp-cli rather than being interpreted by a shell. Same root cause for `cat <file> | youtube-pp-cli ...` (validator reports `EMPTY: has no subcommand words`) and `youtube-pp-cli ... | jq | xargs -I {} youtube-pp-cli ...` (validator chokes on `-I`).
- **Scorer correct?** No — the scorer is incorrectly treating shell metacharacters as positional args.
- **Root cause:** `validate-narrative` doesn't invoke a shell. It tokenizes the recipe `command` and execs it directly. Pipe, `xargs`, redirect (`<`), etc. all break.
- **Cross-API check:** Affects every printed CLI whose recipes use pipes, redirects, or xargs. The skill's own example for the Cookbook section recommends `--select` chained with `jq` — exactly the shape this validator fails on.
- **Frequency:** every CLI with pipe-using recipes — affects narrative authoring guidance broadly.
- **Fallback if the Printing Press doesn't fix it:** Recipe authors must avoid pipes and redirects in `command` fields. This regresses recipe quality — real-world workflows often pipe to jq, head, xargs. Authors discover the issue at shipcheck time and rewrite recipes; ~50% reliability (some authors will leave broken pipes in and hit it later).
- **Worth a Printing Press fix?** Yes, with low risk.
- **Inherent or fixable:** Fixable. Two clean options: (1) split on top-level `|` like #1271 already proposes for `&&`/`;`, validate only the leading segment, mark trailing pipes `pipe-skipped`; (2) invoke `/bin/sh -c <command>` (with `--dry-run` appended via shell-safe quoting) so the shell handles redirection and pipes natively.
- **Durable fix:** Option 1 is least invasive and consistent with #1271's plan. Implement the `|` split alongside the `&&`/`;` split, but only validate the *first segment* — anything after a top-level `|` is shell-output processing, not another CLI invocation worth verifying. Add `< <file>` redirect handling (strip the redirect, keep the rest) since it's common in recipe-style CLI examples.
- **Test:**
  - Positive: a recipe `pp-cli sync --json | jq '.items[]' | head -10` validates only `pp-cli sync --json` and exits 0.
  - Negative: a recipe with a bogus flag before the pipe (`pp-cli sync --bogus-flag | jq`) still fails.
  - Redirect: a recipe `pp-cli bulk --stdin --json < keywords.txt` validates `pp-cli bulk --stdin --json` and exits 0.
- **Evidence:** validate-narrative output during shipcheck #2: `FAILED [recipes]: youtube-pp-cli ... --select "text" | head -c 2000 → ... exit status 1: Error: unknown shorthand flag: 'c' in -c`. After hand-removing the `| head -c 2000` suffix from research.json, the same recipe passed.
- **Related prior retros:** Adjacent to #1271 (validate-narrative can't handle `&&` chains), which explicitly defers pipe handling as "known limitation with a `pipe-skipped` reason." This finding extends the proposed fix to also handle pipes (the explicit non-goal of #1271 is the gap this finding fills).

## Prioritized Improvements

### P1 — High priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F1 | Global-param prevalence filter strips required params | generator | every Google API + any spec with cross-endpoint required params | ~50% (only caught by shipcheck/live testing) | small | Preserve `required: true`; allowlist common pagination params |

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F2 | Google Discovery-origin specs lack API-key auth scheme | openapi-parser | every apis-guru Google API (subclass: `google-discovery-origin`) | ~70% (skill instruction exists, manual workaround required) | small | Only inject when `info.x-origin[].format == "google"` AND no apiKey scheme exists |
| F3 | validate-narrative parses pipes as args | scorer | every CLI with pipe-using recipes | ~50% (authors discover at shipcheck and rewrite) | small | Split on `|` only at top level; same shell-tokenization as the planned `&&`/`;` split in #1271 |

### Skip
*Empty — all Phase 3 candidates filed.*

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| timedtext deprecation note | SKILL's caption-fetch guidance points at jdepoix's older timedtext approach; InnerTube ANDROID player is the current working path | iteration-noise — the agent discovered the alternative inline; YouTube-specific |
| research.json path mismatch | I authored research.json with `videos transcript` paths before knowing the real generator emits `youtube videos-transcript` | author responsibility — validate-narrative caught it correctly |
| `--top` vs `--max-results` flag confusion | Quickstart referenced `search-list --top 5` but `--top` is a search-bulk flag, not search-list | author error in research.json, caught by validate-narrative |
| Phase 4.95 native code review skipped | I skipped the native /review pass on the generated novel-feature code to keep run length down | one-off decision; not a systemic issue |

## Work Units

### WU-1: Generator preserves `required: true` params from prevalence-based filter (from F1)
- **Priority:** P1
- **Component:** generator
- **Goal:** A required-on-all-endpoints param remains reachable as a command flag instead of being stripped.
- **Target:** Generator's global-param filter (likely in `internal/generator/`; specifically the path that emits the `filtered global query param` warning).
- **Acceptance criteria:**
  - Positive test: a fixture spec with a single `required: true` param appearing on every endpoint generates commands that include the param as a `--<param>` flag. Default value optional but allowed.
  - Negative test: a fixture spec where the same param is `required: false` (or unmarked) on every endpoint continues to be filtered. Other common-global params (`prettyPrint`, `callback`, `fields`, `key`) remain filtered when not required.
  - Edge: when a `required: true` param has the same name across endpoints but different `enum` values (rare but real on Google APIs), the generated flag uses the union of enum values or accepts any string.
- **Scope boundary:** This WU does NOT change the filter's behavior for non-required params, does NOT add a vendor-specific allowlist (a separate WU could add common pagination defaults later), and does NOT touch the warning message format.
- **Dependencies:** None.
- **Complexity:** small.

### WU-2: openapi-parser auto-injects API-key scheme for Google-Discovery specs (from F2)
- **Priority:** P2
- **Component:** openapi-parser
- **Goal:** A spec originated from a Google Discovery document automatically gets a working API-key auth scheme without requiring the agent to hand-edit the spec.
- **Target:** openapi-parser's security-scheme handling. The detection signal is `info.x-origin[0].format == "google"` OR the server URL host matching `*.googleapis.com`.
- **Acceptance criteria:**
  - Positive test: generating from any apis-guru Google API spec (YouTube Data v3, Cloud Run Admin, Calendar, etc.) emits `auth.type: api_key`, env var `<UPPER_SLUG>_API_KEY`, and the generated `doctor` reports `auth_source: env:<UPPER_SLUG>_API_KEY` when the env var is set. A real GET against a public endpoint succeeds with the key in `?key=`.
  - Negative test: a non-Google apis-guru spec (e.g., Stripe, Notion) generates without any synthetic ApiKey injection. Existing OAuth2-only flows remain unchanged on non-Google specs.
  - Edge: a Google spec where the agent has manually pre-added an ApiKey scheme is NOT double-injected.
- **Scope boundary:** This WU does NOT replace existing OAuth2 schemes — it adds ApiKey alongside (or only when no scheme exists). Does NOT cover Google's OAuth2 device-code flow (that belongs to #945). Does NOT touch the SKILL's `Pre-Generation Auth Enrichment` section; that guidance still applies for non-Google specs.
- **Dependencies:** None.
- **Complexity:** small.

### WU-3: validate-narrative splits on `|` and strips redirects (extends #1271) (from F3)
- **Priority:** P2
- **Component:** scorer
- **Goal:** Recipes with pipes, redirects, and xargs validate the CLI portion correctly without choking on shell metacharacters.
- **Target:** `printing-press validate-narrative --full-examples` execution path. The `&&`/`;` split planned in #1271 is the right pattern to extend.
- **Acceptance criteria:**
  - Positive test 1: a recipe `pp-cli sync --json | jq '.items[]' | head -10` validates only `pp-cli sync --json` and exits 0; the report marks the trailing pipe as `pipe-skipped`.
  - Positive test 2: a recipe `pp-cli bulk --stdin --json < keywords.txt` validates `pp-cli bulk --stdin --json` and exits 0; the report marks the redirect as `redirect-stripped`.
  - Negative test: a recipe with a bogus flag before the pipe (`pp-cli sync --bogus-flag | jq`) still fails the validation.
  - Edge: a recipe with both `&&` and `|` (`pp-cli sync && pp-cli list --json | jq '.'`) validates both `pp-cli sync` and `pp-cli list --json` segments.
- **Scope boundary:** This WU does NOT execute the shell pipeline (no `/bin/sh -c`). Does NOT validate `xargs`-substituted invocations beyond the first segment. Does NOT touch the SKILL's recipe-authoring guidance.
- **Dependencies:** Coordinate with #1271 if that work is already in flight — same code path, same tokenization approach. This WU's split-on-pipe behavior should compose cleanly with #1271's split-on-`&&`/`;`.
- **Complexity:** small.

## Anti-patterns
- *None observed during this run that warrant elevation to systemic anti-pattern status.*

## What the Printing Press Got Right
- The 8 quality gates passed cleanly on first generation (`go mod tidy`, `govulncheck`, `go vet`, `go build`, binary, `--help`, `version`, `doctor`).
- `dogfood --live --level quick --write-acceptance` produced the gate marker without manual surgery.
- The polish skill (Phase 5.5) correctly converged in one pass and explicitly recommended `further_polish_recommended: no` for the structural gaps — no spurious polish loops.
- The cobratree runtime walker registered 5 endpoint commands + 4 novel commands as MCP tools without manual wiring once the novel commands were registered on the Cobra tree. Annotations for `mcp:read-only` carried through cleanly.
- The `--research-dir` → `research.json` → README/SKILL rendering pipeline worked correctly once paths matched. Dogfood's sync of `novel_features_built` into README/SKILL/root help blocks fired on the second shipcheck pass after research.json paths were fixed.
- Shipcheck's umbrella reported per-leg pass/fail with elapsed timings — easy to spot the failing legs without grepping output.
