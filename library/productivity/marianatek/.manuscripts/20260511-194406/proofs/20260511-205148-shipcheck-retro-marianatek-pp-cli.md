# Printing Press Retro: marianatek

## Session Stats
- API: marianatek (Mariana Tek Customer API v1.0.0)
- Spec source: official OpenAPI 3.0.3 (https://docs.marianatek.com/api/customer/v1/schema/, 81 endpoints, 292KB)
- Scorecard: 89/100 (Grade A)
- Verify pass rate: 95% (21/22, watch's infinite poll + conflicts-without-arg-in-verify counted as fails)
- Fix loops: 2 (generator-emitted compile error patched, then narrative-validation broken examples)
- Manual code edits: 5 distinct patches recorded in `.printing-press-patches.json`
- Features built from scratch: 6 transcendence commands (`watch`, `schedule`, `regulars`, `expiring`, `conflicts`, `book-regular`) + 2 already framework-default (`search`, `doctor`)

Production behavioral test passed: reservation <REDACTED-ID> created end-to-end against live API after patches (the live Mariana Tek tenant `kolmkontrast`, using membership-<REDACTED> as payment, response 201).

## Findings

### 1. Generator emits duplicate Go identifiers when a schema has nested + flat aliases (Bug)
- **What happened:** The OpenAPI spec exposed the same property at two paths — `class_session.classroom.name` and `class_session.classroom_name` (flat alias) — and the generator walked both into the same Go variable name `bodyClassSessionClassroomName`. Result: duplicate `var X string` declarations + duplicate `cmd.Flags().StringVar(...)` registrations in four request-body files. Compile broke; govulncheck refused to load packages.
- **Scorer correct?** N/A (failure was a hard compile, not a score penalty).
- **Root cause:** Generator codegen for body flags walks nested schema properties into snake-case-suffixed identifiers without deduplication. Two distinct schema paths that compute to the same identifier produce two `var ...` statements.
- **Cross-API check:** Pattern triggers whenever a vendor spec defines a nested object property AND a sibling property that's a flattened alias for the same field. Common in JSON:API specs (Stripe, Linear, many fitness/wellness APIs that ship both verbose and convenience field names).
- **Frequency:** subclass — APIs with nested+flat property aliases. Hard to count without scanning every catalog spec, but the schema pattern is well-known in REST/JSON:API design.
- **Fallback if the Printing Press doesn't fix it:** Agent has to grep for `redeclared in this block` after every generate, then dedupe. Reliable to catch (compile error is loud) but pure friction.
- **Worth a Printing Press fix?** Yes — the fix is small (a seen-set in the identifier walker) and prevents a hard-stop class of bug.
- **Inherent or fixable:** Fixable. Cleanest: emit a deterministic seen-set scoped to the file; on second emission of an identifier with the same target, either rename via path-disambiguation suffix (`bodyClassSessionClassroomName_alias1`) or merge the two emission sites into a single set (preferred — the flat alias and the nested form usually mean the same thing).
- **Durable fix:** Generator templates that emit body flag declarations should run identifiers through a dedup pass. The dedup choice (rename vs merge) is templated separately so each emission site is small.
- **Test:** Positive — spec with `{nested: {x: T}, nested_x: T}` → single Go variable, one StringVar, body builder writes both paths from the one variable. Negative — spec without alias collision compiles identically to today.
- **Evidence:** Phase 2 `printing-press generate` output. The exact lines patched are recorded in `.printing-press-patches.json#dedup-classroom-name-{vars,flag}`.
- **Related prior retros:** None (no prior retros in local manuscript archive).

### 2. Spec-required body fields balloon into CLI required-flag bloat (Template gap)
- **What happened:** `me reservations-create` emitted ~50 required-flag checks (`are-add-ons-available`, `class-session-capacity`, `class-session-class-type-duration-formatted`, etc.) because the OpenAPI spec marked every nested body field required. The live API actually requires only `{reservation_type, class_session.id, payment_option.id}`. Users hit `Error: required flag "are-add-ons-available" not set` before they could even do a dry-run.
- **Scorer correct?** N/A (no scorecard penalty fired — the verify pass-rate was 95% because `me reservations-create --dry-run` passed and the strict-required-flag path only fires on real submits).
- **Root cause:** Generator faithfully copies the spec's `required:` lists into RunE preconditions. The Customer API's `UserReservation` schema marks response-derived fields (server-computed booleans, IDs, timestamps) as required because they're always present in the *response*. When the schema is reused as the request body, the generator preserves the required list. The result is requiring fields the API ignores or auto-derives.
- **Cross-API check:** Bearer-auth POST endpoints across the catalog with response-shape body schemas:
  - `cloud/digitalocean` — Droplet create endpoints likely reuse response shape
  - `cloud/render` — Service create
  - `commerce/fedex` — Shipment create
  - `developer-tools/firecrawl` — Crawl create
  - `developer-tools/trigger-dev` — Job trigger
  Plus most CRUD-shaped APIs (Stripe, GitHub) that share response/request schemas.
- **Frequency:** every API whose spec marks response-derived fields required and reuses the schema for create-requests. Probably 30-50% of OpenAPI specs.
- **Fallback if the Printing Press doesn't fix it:** Agent reads through the required list, mentally distinguishes server-derived vs writable, manually loosens. Forgetting → CLI is unusable for the endpoint, which is hard to catch in verify because dry-run paths don't exercise the require checks.
- **Worth a Printing Press fix?** Strong yes. This is the kind of "small per-CLI gotcha" that compounds across the catalog and silently degrades dozens of CLIs.
- **Inherent or fixable:** Fixable. The generator already distinguishes readOnly properties in some paths — extend that to the require enforcement. Default heuristic: a body field is required *only if* it has no `readOnly: true` AND no `default:` AND no known server-computed name pattern (`id`, `created_at`, `updated_at`, `*_count`, `is_*_reserved`, `available_*`).
- **Durable fix:** Generator's body-flag template filters required-flag enforcement through a writable-required predicate. Conservative default: when in doubt, treat required spec field as optional CLI flag (the API will return a validation error if missing). Aggressive variant: profile a sample request to see what the API actually rejects.
- **Test:** Positive — for an endpoint whose spec marks both `id` (server-computed) and `email` (user-supplied) required, the generated CLI should require `--email` but not `--id`. Negative — for an endpoint with no required fields, behavior is unchanged.
- **Evidence:** `.printing-press-patches.json#marianatek-required-flag-bloat`. Patched by replacing 550 lines with a 3-rule validator (class-session-id + reservation-type enum + payment-option-id-unless-waitlist).
- **Related prior retros:** None.

### 3. AuthHeader() doesn't auto-prefix `Bearer ` for `oauth_authorization` (Bug)
- **What happened:** The generated `config.AuthHeader()` returns the raw token from `cfg.CustomerOauthAuthorization` without prepending `Bearer `. Result: setting `oauth_authorization = "<token>"` in config.toml (the natural-looking config) produces every API call with `Authorization: <token>` (no scheme), and the API returns 401. The workaround is to set `auth_header = "Bearer <token>"` verbatim, which is non-obvious — users typically look at the field whose name matches the spec security scheme.
- **Scorer correct?** N/A (scorecard's auth dimension scored 10/10 because the *structural* auth setup was correct; the runtime behavior wasn't exercised by verify).
- **Root cause:** Config template for bearer-auth specs emits an `AuthHeader()` body that returns the raw token. The spec's security scheme declares `type: apiKey, in: header, name: Authorization, description: 'OAuth2 Authentication token, with required prefix. Ex: Bearer ...'` — the prefix is in the human-readable description but not in the machine-encoded type. Template doesn't read the description.
- **Cross-API check:** Every bearer-auth OpenAPI spec in the catalog is exposed:
  - `cloud/digitalocean` (Bearer DO_API_TOKEN)
  - `cloud/render` (Bearer RENDER_API_KEY)
  - `developer-tools/firecrawl` (Bearer FIRECRAWL_API_KEY)
  - `ai/openrouter` (Bearer OPENROUTER_API_KEY)
- **Frequency:** every API whose spec security scheme is bearer-style (apiKey-in-header-named-Authorization OR official `type: http, scheme: bearer`). Affects almost every modern REST API in the catalog.
- **Fallback if the Printing Press doesn't fix it:** Agent or user discovers the 401, traces to config, manually edits to `auth_header = "Bearer ..."`. High cost — silent 401 with no obvious config explanation; users assume the token is wrong, not the prefix.
- **Worth a Printing Press fix?** Strong yes. This is a footgun on the most common auth flow.
- **Inherent or fixable:** Fixable. Template change: when the security scheme indicates bearer (either by `type: http, scheme: bearer` OR by a description matching `/bearer/i` OR by header name `Authorization`), the generated `AuthHeader()` prepends `Bearer ` to the raw token unless the token already includes a `Bearer ` / `Token ` prefix.
- **Durable fix:** Patch the config template's `AuthHeader()` to auto-prefix `Bearer ` when the spec security scheme is bearer-shaped. Keep the `auth_header` verbatim path as the escape hatch for non-bearer custom schemes.
- **Test:** Positive — bearer-spec CLI with `oauth_authorization = "abc"` sends `Authorization: Bearer abc`. Negative — `auth_header = "Token xyz"` sends `Authorization: Token xyz` (verbatim). Already-prefixed `oauth_authorization = "Bearer abc"` doesn't double-prefix.
- **Evidence:** `.printing-press-patches.json#marianatek-bearer-prefix`. Patched config.go's AuthHeader(). Regression test added at `internal/config/config_bearer_test.go`.
- **Related prior retros:** None.

### 4. `auth setup` text points at RFC 6750 when the API has no self-serve OAuth (Skill instruction gap)
- **What happened:** Mariana Tek's Customer API requires going through `<vendor-integrations-email>` to obtain OAuth credentials — there is no consumer-facing flow. The generated `auth setup` printed "Get a key at https://tools.ietf.org/html/rfc6750" (the bearer-token RFC), which is correct as a definition but useless as instructions. The real path for end users is to extract the bearer from the iframe widget's session cookie.
- **Scorer correct?** N/A (no scorecard penalty).
- **Root cause:** Generator's `auth setup` template doesn't know about the "no public OAuth registration" pattern. It defaults to citing RFC 6750 when the spec declares bearer auth, but for an end-user-facing CLI, the more useful default is "we don't have a doc page; ask the API maintainer."
- **Cross-API check:** Patterns where end-users can't self-register OAuth credentials:
  - Most fitness/wellness booking platforms (Mariana Tek, Mindbody, ClassPass, Booksy — vendor-only OAuth)
  - Some enterprise SaaS (Workday, NetSuite — admin-only credential issuance)
  - Banking/financial (most require partnership)
  Inverse: most developer-tool APIs (Stripe, GitHub, Linear) DO have self-serve.
- **Frequency:** subclass — vendor-platform / B2B-with-consumer-frontend APIs. Less common in the current pp catalog (skewed developer-tool), but Mariana Tek is the first booking platform and the pattern would recur for any future MindBody/ClassPass-style work.
- **Fallback if the Printing Press doesn't fix it:** Agent rewrites `auth setup` for each affected CLI by hand. Doable but adds 20-40 lines of bespoke instructions per CLI.
- **Worth a Printing Press fix?** Weak yes. Better-than-RFC-6750 default text helps every bearer CLI. A subclass-specific "browser-cookie extraction" template would help fewer but matter more for those users.
- **Inherent or fixable:** Fixable. Make `auth setup` template a small switch on auth profile: standard-self-serve (point at /developers URL if declared), or browser-extraction (print cookie-name pattern + DevTools steps), or admin-only (print "ask your API contact").
- **Durable fix:** Default auth setup template branches on the spec's `info.x-auth-onboarding` (new vendor extension we'd document) OR by heuristic — if no `developers` / `api-key` URL is found and security scheme is bearer, default to a "see your studio admin / contact your account manager" message rather than RFC 6750.
- **Test:** Positive — bearer-auth spec with no `x-auth-onboarding` produces less-bad default text. Negative — bearer-auth spec with `x-auth-onboarding: {flow: self-serve, url: ...}` produces the existing-style instructions.
- **Evidence:** `.printing-press-patches.json#marianatek-browser-auth`. Patched auth.go's setup output + added auth_browser.go with `from-browser` subcommand. Tests added at `internal/cli/auth_browser_test.go`.
- **Related prior retros:** None.

### 5. MCP surface scored poorly because spec wasn't pre-enriched (Skill instruction gap)
- **What happened:** Spec exposed 94 MCP endpoint tools. The generator's pre-generation warning suggested adding `mcp:` enrichment (transport: [stdio, http], orchestration: code, endpoint_tools: hidden — the "Cloudflare pattern") to the spec before generate. We didn't, because (a) the spec was an external OpenAPI we didn't want to fork, and (b) the SKILL's pre-generation enrichment guidance is in a long block that's easy to skip past during a long run. Scorecard ended up: `mcp_token_efficiency 4/10`, `mcp_remote_transport 5/10`, `mcp_tool_design 5/10`, `mcp_surface_strategy 2/10`.
- **Scorer correct?** Yes — the structural critique is accurate. 94 unfiltered tools is too many for an agent to load at server start.
- **Root cause:** SKILL/instruction gap. The pre-generation MCP enrichment guidance is correct on paper but is one prose block among many; for a long run on a large spec, it's easy to skip. The generator already prints a warning at generate-time but doesn't propose a corrective overlay or auto-apply.
- **Cross-API check:** Any API with >30 endpoints. Half the catalog likely:
  - `cloud/digitalocean` (large surface)
  - `commerce/amazon-seller` (huge surface)
  - `commerce/ebay` (huge)
  - `developer-tools/trigger-dev` (medium)
- **Frequency:** every API with >30 endpoints — substantial fraction of the catalog.
- **Fallback if the Printing Press doesn't fix it:** The generator warning exists; if the agent reads it and acts, score is fine. We didn't. Reliability is ~60-70% by my guess (the warning is easy to miss after a long pipeline run).
- **Worth a Printing Press fix?** Yes. The cheapest version: when the generator emits the warning, also emit a tiny `mcp-overlay.yaml` file in the working dir that the agent can pass back into the generator with one flag. Doesn't require forking the spec.
- **Inherent or fixable:** Fixable.
- **Durable fix:** When `mcp endpoint tool count > 50`, the generator: (a) keeps the existing warning, (b) writes `mcp-overlay.yaml` with the recommended Cloudflare pattern to the working directory, and (c) prints a one-liner: `re-run with --spec <original> --spec mcp-overlay.yaml to apply`. The agent now has a concrete one-step path instead of "consider enriching the spec."
- **Test:** Positive — generate against a 94-endpoint spec, overlay file appears, second generate run with overlay → scorecard MCP dimensions >= 7/10. Negative — generate against a 12-endpoint spec, no overlay file appears.
- **Evidence:** Shipcheck scorecard output during this session.
- **Related prior retros:** None.

## Prioritized Improvements

### P1 — High priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F3 | AuthHeader auto-prefix Bearer for bearer-style security schemes | generator | every bearer-auth API in the catalog | low — silent 401 with no clear cause | small (one template) | only when security scheme is bearer-shaped; pass-through when token already prefixed |
| F2 | Generator distinguishes writable-required from response-required body fields | generator | 30-50% of OpenAPI specs (response/request schema reuse) | medium — verify catches dry-run path but not strict-required path on submit | medium (template + readOnly-aware filter) | conservative: when in doubt, treat as optional |

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F1 | Dedup Go identifiers across nested+flat schema aliases | generator | subclass — nested+flat alias pattern | high (compile error is loud) but every occurrence wastes a fix cycle | small (seen-set in identifier walker) | scope dedup to a single emit phase |
| F5 | Auto-emit mcp-overlay.yaml when endpoint count > 50 | generator | every API with >50 endpoints | medium — warning is easy to miss | small (one file write + one-line hint) | only when count crosses threshold |

### P3 — Low priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F4 | `auth setup` default text branches on auth-onboarding profile | generator | subclass — vendor-platform / no-public-OAuth APIs | high — agent can hand-edit per CLI but compounds | small (template branch) + vendor extension `x-auth-onboarding` | only when no obvious developer URL found |

### Skip
None — each survivor cleared Phase 2.5 triage and Phase 3 Step G.

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| Multi-tenant config not supported | Framework assumes one tenant per config; Mariana Tek wants per-brand subdomains | printed-CLI (this is genuinely per-API quirk; most APIs are single-tenant) |
| Chrome MCP filter blocked cookie extraction | Cookie reading is blocked by Anthropic's PII filter | upstream — not a Printing Press concern |
| HAR-capture flow wasn't needed | We had an OpenAPI spec, HAR was redundant | iteration-noise (the user opted into capture before we knew about the spec) |

## Work Units

### WU-1: AuthHeader auto-prefix Bearer (from F3)
- **Priority:** P1
- **Component:** generator
- **Goal:** Bearer-auth CLIs send the wire-correct `Authorization: Bearer <token>` when the user sets `oauth_authorization = "<raw-token>"` in config.
- **Target:** Config template under `internal/generator/` (whichever template emits `internal/config/config.go`'s `AuthHeader()` function).
- **Acceptance criteria:**
  - Positive: bearer-spec CLI with `oauth_authorization = "abc"` sends `Authorization: Bearer abc` on every request.
  - Positive: already-prefixed `oauth_authorization = "Bearer abc"` passes through without double-prefix.
  - Positive: `auth_header = "Token xyz"` sends verbatim (escape hatch preserved).
  - Negative: non-bearer security schemes (apiKey-in-query, custom schemes) are unaffected.
- **Scope boundary:** Does not change the `auth_header` verbatim path. Does not introduce a new config field. Does not change the env-var detection (`CUSTOMER_OAUTH_AUTHORIZATION` → `cfg.CustomerOauthAuthorization`).
- **Dependencies:** none
- **Complexity:** small

### WU-2: Writable-required body-field filter (from F2)
- **Priority:** P1
- **Component:** generator
- **Goal:** Body-flag required-checks in generated request commands only enforce fields the API actually requires the user to provide; server-derived and readOnly fields stop appearing as required CLI flags.
- **Target:** Generator body-flag template (the path that emits the `if !cmd.Flags().Changed(...) && !flags.dryRun { return Errorf("required flag ...") }` block in generated request-body commands).
- **Acceptance criteria:**
  - Positive: a spec with `{id: {readOnly: true}, email: {}}` required → generated CLI requires `--email` only.
  - Positive: a spec where the request body schema is a $ref to the response schema (a common API design) → generator either filters by `readOnly` OR defaults to no-required-checks when the writable subset is unclear.
  - Negative: a spec with all-writable required fields preserves today's behavior.
- **Scope boundary:** Does not modify the request body builder (the part that constructs the JSON), only the precondition checks.
- **Dependencies:** none
- **Complexity:** medium

### WU-3: Identifier dedup across body-flag emission (from F1)
- **Priority:** P2
- **Component:** generator
- **Goal:** Generated body-flag files compile even when the underlying spec aliases a nested property with a flat sibling.
- **Target:** Identifier walker used by body-flag emission templates.
- **Acceptance criteria:**
  - Positive: spec with `{nested: {x: T}, nested_x: T}` produces a single `var bodyNestedX T` and a single `cmd.Flags().StringVar` registration.
  - Positive: the body builder writes both spec paths (`nested.x` and `nested_x`) from the merged variable.
  - Negative: specs without alias collisions produce identical output to today.
- **Scope boundary:** This is the dedup pass only — alias-merge semantics live in the existing body builder.
- **Dependencies:** none
- **Complexity:** small

### WU-4: Auto-emit mcp-overlay.yaml when surface > 50 (from F5)
- **Priority:** P2
- **Component:** generator
- **Goal:** Closes the loop on the pre-generation MCP warning by writing a ready-to-use overlay the agent can apply with one flag.
- **Target:** Generator's pre-generation MCP-surface count + warning code path.
- **Acceptance criteria:**
  - Positive: generate against a spec with >50 endpoint tools → `mcp-overlay.yaml` appears in working dir with `mcp: {transport: [stdio, http], orchestration: code, endpoint_tools: hidden}`.
  - Positive: re-running `generate --spec <original> --spec mcp-overlay.yaml` produces a CLI whose scorecard MCP dimensions score >= 7/10.
  - Negative: generate against a spec with <30 endpoint tools → no overlay file appears.
- **Scope boundary:** Doesn't auto-apply the overlay (preserve user choice). Doesn't change the warning text.
- **Dependencies:** none
- **Complexity:** small

### WU-5: `auth setup` branches on auth-onboarding profile (from F4)
- **Priority:** P3
- **Component:** generator
- **Goal:** The default `auth setup` output is useful for vendor-platform APIs that don't have self-serve OAuth registration.
- **Target:** Auth-setup template in the generator + a new (optional) spec vendor extension `x-auth-onboarding`.
- **Acceptance criteria:**
  - Positive: spec with `info.x-auth-onboarding: {flow: vendor-managed, contact: integrations@example.com}` → `auth setup` prints "contact integrations@..." instead of RFC 6750.
  - Positive: spec with `info.x-auth-onboarding: {flow: browser-extraction, cookie-prefix: mt.token}` → `auth setup` prints DevTools cookie-extraction steps.
  - Negative: spec without the vendor extension → default behavior unchanged.
- **Scope boundary:** This is template-only; doesn't add a runtime cookie-blob parser to every CLI (that's a per-API addition like our `auth_browser.go`).
- **Dependencies:** none
- **Complexity:** small

## Anti-patterns
- Skipping the pre-generation MCP enrichment block during a long pipeline run because it's prose-only guidance buried in a wall of phases. The warning at generate-time is helpful but the agent has already committed to a generate call at that point. Pre-generation enrichment should be either auto-applied or made unavoidable.
- Trusting OpenAPI `required:` blocks faithfully when the schema is reused for both request and response. Vendor specs are written around response shape and the request-required subset is rarely re-declared.
- Treating "no self-serve OAuth" as an edge case. It's standard for vendor-platform APIs (booking, banking, healthcare) and the SKILL/generator should accommodate the pattern instead of defaulting to RFC 6750.

## Additional Finding — discovered after first retro

### 6. `generate --force` AST-merge does not preserve `// PATCH(retro)` blocks (Bug)
- **What happened:** Mid-session attempt to apply MCP enrichment by re-running `printing-press generate --force --spec <original-with-mcp-block> ...` against the library directory. Result: all 7 hand patches were silently overwritten — root.go reverted to the generator-emitted shape (lost the `--tenant` flag), config.go lost the Bearer-prefix patch and the LoadTenant/ListTenants additions, auth.go lost the Mariana-Tek-specific setup text, and the dup-classroom-name compile-error bug returned. Restored from a `cp -r` snapshot.
- **Scorer correct?** N/A (scorer doesn't observe regen).
- **Root cause:** `--force` advertises "preserves hand-edits to generated files via AST-based merge," but in practice the merge appears to drop:
  - new files the agent added next to generated files (`auth_browser.go`, `tenants.go`, `transcendence_test.go`)
  - structural additions inside generated files (new struct fields, new methods on a receiver, new branch arms in switches, new AddCommand calls)
  - even the dedup fixes that lived in the original (the dup-var bug returned)
- **Cross-API check:** Every CLI that has accepted hand-patches (which is most of them, per the AGENTS.md mandate to patch-published-CLIs-locally-when-broken) is exposed. The risk is structurally present across the catalog.
- **Frequency:** every reprint or post-generation `--force` regen on any published CLI that has ever been hand-patched. The published library's own AGENTS.md tells contributors to record patches in `.printing-press-patches.json` — but if regen drops them, the manifest becomes a tombstone, not a preservation mechanism.
- **Fallback if the Printing Press doesn't fix it:** Agents must `cp -r` snapshot before any regen, and manually re-apply every `// PATCH(retro)` block after the regen. Reliable but tedious; one missed restore = silent functional regression.
- **Worth a Printing Press fix?** Strong yes. This is a foundational property of the whole pipeline — without merge preservation, `.printing-press-patches.json` is misleading and reprints are destructive.
- **Inherent or fixable:** Fixable. AST-merge needs to detect `// PATCH(retro ...)` comments as protected blocks and preserve them across regens. Adjacent-file additions (new .go files) should always be preserved unless their name collides with a regenerated file.
- **Durable fix:**
  1. AST-merge treats any function, type, or method whose declaration is immediately preceded by `// PATCH(retro #...)` as protected: re-emit nothing for those names, keep the existing definition verbatim.
  2. Files that did not exist in the prior generation output but exist now are preserved (likely hand-authored additions).
  3. New struct fields and new switch arms inside protected functions are preserved.
- **Test:**
  - Positive: regen a CLI with a `// PATCH(retro #x): new method` decoration — protected function survives intact.
  - Positive: regen a CLI with a hand-authored sibling file (`auth_browser.go` next to generated `auth.go`) — sibling file is preserved.
  - Negative: regen with no PATCH decorations behaves identically to today.
- **Evidence:** Mid-session attempt + restoration documented in `.printing-press-patches.json`'s `upstream_followups` after restore. Snapshot at `/tmp/marianatek-pre-mcp-regen` captured the pre-regen state.
- **Related prior retros:** None (first finding of this class).

This is the most consequential finding in this retro. Without merge preservation, every other generator-level improvement we propose (F1-F5) lands as a regression vector for already-shipped CLIs. **Recommended priority: P1**, and it should land BEFORE any of F1-F5 to avoid breaking published library entries when those fixes ship.

## What the Printing Press Got Right
- The dry-run path on `me reservations-create` showed the correct minimal body shape (`{class_session: {id}, payment_option: {id}, reservation_type}`) the first time we exercised it. That made the production-test root-cause analysis fast: we knew the spec was lying about required fields because the generator's own preview disagreed.
- The OpenAPI spec discovery + JSON:API envelope handling + page-based pagination all worked out-of-the-box; we didn't have to write any of those.
- The framework's local SQLite store, sync pipeline, FTS5 search, and doctor command came for free. Three of the eight planned transcendence commands turned out to already exist (search, doctor) as framework defaults, which let us focus hand-build effort on the genuinely novel ones (watch, regulars, expiring, conflicts, schedule, book-regular).
- The novel-features brainstorm subagent's three-pass cut (customer model → candidates → adversarial cut) produced exactly the 8 features we ended up shipping — and the kills were defensible (smart-cancel and leaderboard scraping really would have been scope creep / out-of-band).
- The shipcheck umbrella's per-leg output made it obvious which leg failed (validate-narrative) and which were clean, so the fix cycle was tight.

## Filing Decision
Local-only retro per user request. No GitHub issues filed. All findings consolidated in this document + `.printing-press-patches.json#upstream_followups` in the promoted library. If/when the CLI is contributed upstream, this retro doc and the patches manifest become the natural attachment for the maintainer review.
