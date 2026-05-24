# Printing Press Retro: setlist-fm

## Session Stats
- API: Setlist.fm
- Spec source: Official Swagger 2.0 at `https://api.setlist.fm/docs/1.0/ui/swagger.json`
- Scorecard: 77/100 (Grade B)
- Verify pass rate: 100% (32/32)
- Dogfood: PASS (15/15 novel features)
- Live dogfood: 39/39 passed
- Fix loops: 2 (Accept header + cache clear)
- Manual code edits: 6 (config BaseURL, config dual env vars, config AuthHeader simplification, root.go rate-limit default, doctor env-var check, client Accept header)
- Features built from scratch: 16 transcendence + 6 ergonomic alias groups + dedicated SQLite schema + hydrated sync

## Findings

### F1. Swagger 2.0 `host` + `basePath` not populating BaseURL default (template gap)
- **What happened:** The spec declares `host: api.setlist.fm` and `basePath: /rest` but the generated config.go defaulted to `https://api.example.com`. Required a manual patch to `config.go` to set `BaseURL: "https://api.setlist.fm/rest"`.
- **Scorer correct?** Yes. The scorecard's auth_protocol score of 0/10 is partially related — the spec's security definitions were empty before enrichment, but the BaseURL fallback is the root cause of the "no servers" warning.
- **Root cause:** `loadSwagger2AsOpenAPI3()` in `internal/openapi/swagger2.go` converts via `openapi2conv.ToV3WithLoader()`. The conversion should populate `doc3.Servers[0].URL` from the Swagger 2.0 `host` + `basePath` fields. Either the conversion library doesn't populate it, or the parser's `resolveServerURLTemplate()` call doesn't handle the converted output. The result is the "no servers defined in spec" warning and `PlaceholderBaseURL` fallback.
- **Cross-API check:** Affects every Swagger 2.0 spec with `host` + `basePath` defined. Swagger 2.0 is still common (Setlist.fm, many legacy APIs).
- **Frequency:** API subclass: Swagger 2.0 specs with `host` field populated.
- **Fallback if the Printing Press doesn't fix it:** Claude manually patches config.go. This happens reliably when the build log calls it out, but the step burns ~2 minutes and is easy to forget.
- **Worth a Printing Press fix?** Yes. The data is right there in the spec.
- **Inherent or fixable:** Fixable. The conversion library likely does populate `Servers` — the parser may just need to check for it after the conversion path, or there's a kin-openapi version issue.
- **Durable fix:** In `internal/openapi/parser.go`, after `loadSwagger2AsOpenAPI3()` returns, verify that `doc.Servers` is populated. If not, construct a server entry from the Swagger 2.0 `host` + `basePath` fields (which are available in the raw JSON before conversion). Guard: only when `isSwagger2SpecJSON()` is true.
- **Test:** Positive: generate from a Swagger 2.0 spec with `host: api.example.com` + `basePath: /v1` → config.go should have `BaseURL: "https://api.example.com/v1"`. Negative: a Swagger 2.0 spec without `host` should still get the placeholder.
- **Evidence:** Build log finding #1: "Swagger 2.0 `host` + `basePath` should populate `cfg.BaseURL` default."

### F2. Generated HTML tags in command descriptions from Swagger 2.0 specs (template gap)
- **What happened:** The 15 auto-generated endpoint commands under `1-0/` had raw HTML tags (`<p>`, `<strong>`, `<em>`) in their `Short` descriptions, inherited from the Swagger spec's `description` fields. The polish pass had to strip them manually from 7 files.
- **Scorer correct?** N/A (not a scorer finding — this is a UX/quality issue).
- **Root cause:** The generator copies spec description fields verbatim into Cobra `Short` strings. Swagger 2.0 descriptions commonly contain HTML (the spec format permits it). The generator does not strip HTML from descriptions.
- **Cross-API check:** Affects any Swagger 2.0 spec that uses HTML in operation descriptions (common).
- **Frequency:** API subclass: Swagger 2.0 specs with HTML-formatted descriptions.
- **Fallback if the Printing Press doesn't fix it:** Claude or polish strips HTML manually. Reliable but tedious.
- **Worth a Printing Press fix?** Yes. Simple text sanitization.
- **Inherent or fixable:** Fixable. Strip HTML tags from description strings at template emission time or in the parser.
- **Durable fix:** In the generator, when emitting `Short:` for Cobra commands, pass the description through a `stripHTMLTags()` function (simple regex `<[^>]+>` replacement, then collapse whitespace). Already precedent: `cliutil.CleanText` exists for similar purposes.
- **Test:** Positive: generate from a spec with `description: "<p>Returns an artist</p>"` → Short should be `"Returns an artist"`. Negative: plain-text descriptions pass through unchanged.
- **Evidence:** Polish pass had to `sed` HTML tags from 7 generated `1-0_*.go` files.

### F3. Auth protocol score 0/10 for x-api-key auth pattern (scorer bug)
- **What happened:** Scorecard scored auth_protocol 0/10 despite the CLI having fully working auth (doctor confirms, live dogfood 39/39). The spec uses Swagger 2.0 `securityDefinitions` with `type: apiKey`, `in: header`, `name: x-api-key` — a valid auth pattern the scorer doesn't recognize.
- **Scorer correct?** No. The scorer expects a specific auth scheme pattern (Bearer/Bot prefix) and scores 0 when the scheme is `apiKey` via `x-api-key` header. The CLI handles auth correctly.
- **Root cause:** The scorecard's auth_protocol check looks for Bearer/Bot token patterns and scores 0 for unknown auth header schemes. `x-api-key` is a common pattern (AWS, Setlist.fm, many others) that doesn't use an Authorization header prefix.
- **Cross-API check:** Affects any API using `x-api-key` header auth (common pattern).
- **Frequency:** API subclass: APIs using `x-api-key` or similar non-Authorization-header auth.
- **Fallback if the Printing Press doesn't fix it:** Score artificially low on every x-api-key API. No functional impact.
- **Worth a Printing Press fix?** Yes. Scorer should recognize x-api-key as a valid auth pattern.
- **Inherent or fixable:** Fixable. The scorer needs to recognize more auth patterns.
- **Durable fix:** In the scorecard's auth_protocol check, recognize `x-api-key` header auth as a valid pattern (not just Bearer/Bot). Check whether the config.go reads from an env var and the doctor validates auth — if both are present, score as a match.
- **Test:** Positive: CLI with x-api-key auth and working doctor should score >= 7/10. Negative: CLI with no auth config should still score 0.
- **Evidence:** Scorecard output: `Auth Protocol 0/10` despite doctor reporting `auth: configured` and live dogfood passing 39/39.

### F4. Generated parent command `1-0` has meaningless name and description (default gap)
- **What happened:** The generated parent command was `1-0` with description "Manage 1 0" — derived from the Swagger spec's `basePath: /rest` and first path segment `1.0`. This is meaningless to users. The polish pass changed it to "Raw Setlist.fm API v1.0 endpoints."
- **Scorer correct?** N/A.
- **Root cause:** The generator derives parent command names from path segments. `/1.0/...` becomes parent `1-0`, and the `Short` description is auto-generated as "Manage 1 0." No semantic awareness that `1.0` is a version prefix, not a resource name.
- **Cross-API check:** Affects any API with versioned path prefixes (`/v1/`, `/v2/`, `/1.0/`, `/api/v3/`).
- **Frequency:** Most APIs — version prefixes in paths are near-universal.
- **Fallback if the Printing Press doesn't fix it:** Claude or polish renames the parent. Reliable.
- **Worth a Printing Press fix?** Yes, but low priority. The fix is to detect version-like path segments and either collapse them into the resource path or emit a sensible description.
- **Inherent or fixable:** Fixable. The profiler can detect version-prefix patterns (regex `v?\d+(\.\d+)*`) and handle them specially.
- **Durable fix:** In the generator's resource naming logic, when the parent command name is a version-like string (matches `v?\d+(\.\d+)*` after slug normalization), either: (a) collapse it into the resource path so commands are `artist get` not `1-0 resource-artist-get`, or (b) emit a description like "API v{version} endpoints" instead of "Manage {slug}."
- **Test:** Positive: spec with paths under `/v2/users/` → parent description should be "API v2 endpoints" or resources should flatten to top-level. Negative: a resource named `v2-auth` that isn't a version prefix should keep its name.
- **Evidence:** Generated parent was `1-0` with `Short: "Manage 1 0"`.

### F5. Dual env-var support for API key not expressible in spec (recurring friction)
- **What happened:** Setlist.fm has two community conventions for the env var: `SETLISTFM_API_KEY` (from setlistfm-js) and `SETLIST_FM_API_KEY` (from setlist-fm-client). The generated config.go only supports one. Required manual patching to accept both.
- **Scorer correct?** N/A.
- **Root cause:** The generator emits a single env var name derived from the API slug. There's no spec extension to declare alternative env var names. Catalog entries support `auth_env_vars` but the spec-level path doesn't.
- **Cross-API check:** Uncommon — most APIs have one canonical env var name. A few (GitHub: `GITHUB_TOKEN` vs `GH_TOKEN`) have two.
- **Frequency:** Rare — ~5% of APIs.
- **Fallback if the Printing Press doesn't fix it:** Claude patches config.go. Reliable.
- **Worth a Printing Press fix?** Low priority. The catalog's `auth_env_vars` already supports ordered lists. The friction is only for APIs not in the catalog.
- **Inherent or fixable:** Already fixable via catalog. For non-catalog APIs, a spec extension like `x-auth-env-var-aliases` could express alternatives, but the frequency doesn't justify the complexity.
- **Durable fix:** Skip — the catalog path already handles this.
- **Evidence:** Build log finding #3: "Doctor's env-var probe is single-name only."

## Prioritized Improvements

### P1 — High priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F1 | Swagger 2.0 host+basePath not populating BaseURL | OpenAPI parser | Swagger 2.0 subclass | Reliable but slow | Small | isSwagger2SpecJSON |
| F3 | Auth protocol score 0/10 for x-api-key pattern | Scorecard | x-api-key APIs | No fallback (score stays wrong) | Small | auth scheme type |

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F2 | HTML tags in Swagger 2.0 descriptions | Generator templates | Swagger 2.0 with HTML | Reliable manual strip | Small | Has HTML tag chars |
| F4 | Meaningless version-prefix parent commands | Generator profiler | Most APIs | Reliable manual rename | Medium | Path matches version regex |

### Skip
| Finding | Title | Why unlikely to recur |
|---------|-------|----------------------|
| F5 | Dual env-var support | Already handled by catalog `auth_env_vars`. Rare enough that non-catalog patch is acceptable. |

## Work Units

### WU-1: Swagger 2.0 BaseURL resolution (from F1)
- **Goal:** When parsing a Swagger 2.0 spec with `host` and `basePath` fields, the generated config.go should default BaseURL to `https://{host}{basePath}`.
- **Target:** `internal/openapi/parser.go` and/or `internal/openapi/swagger2.go`
- **Acceptance criteria:**
  - Positive: Generate from `setlist-fm-swagger.json` (host: api.setlist.fm, basePath: /rest) → config.go has `BaseURL: "https://api.setlist.fm/rest"`
  - Negative: Swagger 2.0 spec without `host` → falls back to PlaceholderBaseURL as before
- **Scope boundary:** Does not change the OpenAPI 3.0 server URL resolution path
- **Dependencies:** None
- **Complexity:** Small

### WU-2: Scorecard x-api-key auth recognition (from F3)
- **Goal:** The scorecard's auth_protocol dimension should recognize x-api-key header auth as a valid pattern.
- **Target:** Scorecard CLI command — the auth_protocol scoring function
- **Acceptance criteria:**
  - Positive: CLI using x-api-key auth with working doctor → auth_protocol >= 7/10
  - Negative: CLI with no auth → still 0/10
- **Scope boundary:** Does not change how auth is generated, only how it's scored
- **Dependencies:** None
- **Complexity:** Small

### WU-3: Strip HTML from Swagger description fields (from F2)
- **Goal:** Generated Cobra `Short` strings should not contain raw HTML tags.
- **Target:** Generator templates or parser — wherever description strings are emitted into Go source
- **Acceptance criteria:**
  - Positive: Spec with `<p>Returns an artist</p>` → Short: `"Returns an artist"`
  - Negative: Plain-text descriptions unchanged
- **Scope boundary:** Only Cobra Short/Long strings, not comments or documentation
- **Dependencies:** None
- **Complexity:** Small

### WU-4: Version-prefix-aware parent command naming (from F4)
- **Goal:** Parent commands derived from version path segments should have meaningful descriptions and ideally flatten to top-level resources.
- **Target:** `internal/openapi/parser.go` resource naming logic
- **Acceptance criteria:**
  - Positive: Spec with paths under `/v2/users/` → parent desc not "Manage v2"
  - Negative: Resource named `v2-auth` keeps its name
- **Scope boundary:** Does not change command structure for non-version paths
- **Dependencies:** Golden fixture updates
- **Complexity:** Medium

## Anti-patterns
- **Stale HTTP response cache silencing bug fixes.** When the `Accept: application/json` header was added, the client still returned XML because old responses were cached. The cache layer sits before the HTTP request in `Get()` and has no way to know that the request shape changed. Consider cache invalidation when client configuration changes (at minimum, different headers should produce different cache keys).

## What the Printing Press Got Right
- **Swagger 2.0 → OpenAPI 3.0 conversion** worked transparently. The spec loaded and parsed without errors despite being Swagger 2.0.
- **All 8 generation gates passed** on the first run. No compilation errors from the generated code.
- **Rate-limit plumbing** was already in the client template. Only the default value needed adjusting.
- **MCP server generation** worked out of the box — 15 tools registered automatically.
- **Dogfood, verify, and scorecard** all ran successfully and caught real issues (dead functions, missing examples, novel feature naming mismatches).
- **The novel_features_built sync** in dogfood correctly identified which transcendence features existed as commands and which didn't, keeping README/SKILL in sync with reality.
- **The verify-skill checker** caught every SKILL.md recipe that used wrong command names (space-separated instead of hyphenated).
