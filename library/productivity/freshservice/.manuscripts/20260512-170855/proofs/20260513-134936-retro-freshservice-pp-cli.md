# Printing Press Retro: freshservice

## Session Stats
- API: freshservice
- Spec source: internal-YAML (authored from effytech/freshservice_mcp source-code research)
- Scorecard: 84/100 (Grade A)
- Verify pass rate: 100% (35/35 mock-mode subcommands)
- Fix loops: ~6 (during live test discovery, mostly cardinal-bug fixes)
- Manual code edits: ~14 (including 4 CRITICAL infrastructure bugs, 9 novel feature commands, 1 UX resolver)
- Features built from scratch: 10 (1 generator-emitted + 9 hand-authored transcendence commands)

Note: This is the first CLI in the local library; no prior retros to dedup against.

## Findings

### 1. Boolean body fields are omitted from POST bodies unless the user explicitly passes a non-zero value (Bug)

- **What happened:** `freshservice-pp-cli tickets notes create-ticket <TICKET_ID> --body "hi"` posted a *private* note because the API defaults `private: true` server-side when the field is omitted from the JSON body. The generator emitted `if bodyPrivate != false { body["private"] = bodyPrivate }`, so `--private` not being passed meant the field was missing from the wire request and Freshservice applied its server-side default. The user-facing default and the server-side default disagreed, silently, on every note write.

- **Scorer correct?** N/A — this surfaced via real-world user feedback, not a scorer penalty.

- **Root cause:** `internal/generator/generator.go:3271` — `renderBodyMap()` emits the `if x != zeroVal(p.Type) { body[name] = x }` guard for all scalar body params, including booleans. For booleans, `zeroVal()` is `false`, so an unset flag means the field is never present in the body. The spec didn't declare a `default:` keyword for `private` (Freshservice's spec is silent on most server-side defaults), so the generator had no signal to override the omission heuristic.

- **Cross-API check:** Any API where a boolean body field's server-side default disagrees with `false`. The generator's omission-on-zero pattern is uniformly broken for that shape. Concrete examples:
  - **Freshservice** — `POST /tickets/{id}/notes` `private: true` server-default (this incident).
  - **Linear (GraphQL via REST wrapper, also REST endpoints)** — Documents have `confidential` boolean; defaults vary by mutation.
  - **Stripe** — `automatic_tax.enabled`, `expand_invoice_items`, several update endpoints have booleans where omission != false.

- **Frequency:** subclass:`boolean-body-field-with-non-false-server-default`. Likely fires once per affected endpoint per CLI, but invisibly — the bug never crashes, just silently produces the wrong outcome.

- **Fallback if the Printing Press doesn't fix it:** Future printed CLIs continue to silently default booleans to server-side defaults. Agents and users get unexpected results until a user complains (as happened here). The fallback is "hope someone catches it"; that's not reliable.

- **Worth a Printing Press fix?** Yes. The fix is one branch in `renderBodyMap()`. The cost-benefit math is heavily lopsided.

- **Inherent or fixable:** Fixable.

- **Durable fix:** In `renderBodyMap()`, for any param where `p.Type == "boolean"`, always emit `body[name] = bodyField` without the `!= zeroVal` guard. Other scalar types (int, string) retain the existing guard since their zero values usually do match server-side defaults; the bug shape is specific to booleans where the server's omission-default is `true`.

  Alternative (slightly more general but more verbose): emit `if cmd.Flags().Changed("flag-name") { body[name] = bodyField }` for all scalars. This is the canonical cobra idiom for distinguishing "user explicitly set zero" from "user didn't touch this flag", and avoids the boolean special case. Marginally more code per scalar but semantically correct for every type.

- **Test:** Positive — generate a CLI from a spec with a boolean body field, omit the flag, dry-run; assert the field appears in the body with value `false`. Negative — set the flag to `true`, assert the field appears with `true`. Negative for non-booleans: int field, omit the flag; assert the field is *omitted* (preserve existing behavior for ints/strings).

- **Evidence:** Real-world note posted private despite no `--private` flag; user had to post a second note to correct. They asked for a delete command and the CLI had none, so cleanup happened in the UI.

- **Related prior retros:** None — first CLI in this library.

---

### 2. `BaseURL` default ignores the spec's `servers[0].url` placeholder substitution from declared env vars (Bug)

- **What happened:** `freshservice-pp-cli` was generated with a hardcoded `BaseURL: "https://yourcompany.freshservice.com/api/v2"` in `internal/config/config.go`. The Freshservice spec **explicitly** declared `servers[0].url: "https://{domain}/api/v2"` with `variables.domain` plus an `x-auth-vars` entry naming `FRESHSERVICE_DOMAIN` as the env var holding that value. The generator emitted both the env-var loader AND the BaseURL default, but never wired the env var into the URL. Result: every live API call hit a DNS-failing placeholder host. We had to hand-patch `config.Load()` to substitute `FreshserviceDomain` into `BaseURL` before any live test would work.

- **Scorer correct?** N/A — surfaced during live testing, not a scorer finding. Notably the verify pass rate was 100% in mock mode; the bug is invisible without live testing.

- **Root cause:** Most likely in `client.go.tmpl` or wherever `BaseURL` is initialized — the spec parser exposes `servers[0].url` and the variables block, but the template hardcodes the literal default string without substitution. The information is present in the spec; the generator never consults it.

  Uncertainty: I haven't traced exactly which template (or which spec-parser step) drops the substitution. Candidates are `client.go.tmpl` (BaseURL init), `config.go.tmpl` (the Load function), or the spec-parser (not surfacing the variables as substitutable). An implementer should grep for where the literal `yourcompany.freshservice.com` (or the spec's `default:` value) ends up in the generated source and trace backwards.

- **Cross-API check:** Every multi-tenant SaaS API with `{placeholder}` in `servers[0].url`. Concrete examples:
  - **Freshservice** — `https://{domain}/api/v2` with `FRESHSERVICE_DOMAIN` (this incident).
  - **Atlassian Cloud (Jira/Confluence)** — `https://{your-domain}.atlassian.net/rest/api/3` with declared variable.
  - **Slack Workspace API** — `https://{workspace}.slack.com/api/...` for some endpoints.
  - **Shopify** — `https://{shop}.myshopify.com/admin/api/2024-01/`.
  - **Auth0** — `https://{tenant}.auth0.com/api/v2/`.

  Five concrete APIs with declared placeholders in their OpenAPI specs.

- **Frequency:** Every multi-tenant API. Likely 5-10 of the 23 APIs in the current catalog. Fires on every live request — 100% failure rate without manual patching.

- **Fallback if the Printing Press doesn't fix it:** Users either hit a confusing DNS-failure path or set `<API>_BASE_URL` manually as a workaround. The latter requires knowing the API's base URL by heart and writing it twice (once in `_BASE_URL`, once in `_DOMAIN` for auth). The CLI ships with two env vars that should be unified — that's a UX trap.

- **Worth a Printing Press fix?** Strongly yes. Multi-tenant APIs are common; this bug means every one ships broken-out-of-the-box.

- **Inherent or fixable:** Fixable. The spec's variables block is structured; substitution is a `strings.ReplaceAll(servers[0].url, "{"+name+"}", value)` per variable.

- **Durable fix:** At config.Load() time, after env vars are read, substitute every `{name}` in the spec's `servers[0].url` template with the value from the corresponding env var. Fall back to `default:` when the env var is unset, so doctor still has a real URL to probe. Also support the loose forms users actually paste (`https://`, trailing slash, suffix-included) via a normalizer like the one we added to the freshservice CLI.

  Template parameterization: the generator emits a per-CLI `normalize<Tenant>Domain()` helper only when the spec has placeholders; APIs with static base URLs continue to use the unchanged literal default.

- **Test:** Positive — generate a CLI from a spec with `servers[0].url: "https://{domain}/api/v2"` and `variables.domain.default: "example"`; assert that with the env var unset, doctor probes `https://example/api/v2`; with the env var set to `acme.example.com`, doctor probes `https://acme.example.com/api/v2`. Negative — generate a CLI from a spec with a static `servers[0].url`; assert no substitution code is emitted and the literal URL is used.

- **Evidence:** Live test against <tenant> tenant returned DNS failures until we patched `config.go` with a `normalizeFreshserviceDomain()` helper. The eventual fix required 25 lines of generator-emittable code, all of which the generator had the inputs for.

- **Related prior retros:** None.

---

### 3. HTTP Basic auth credential builder concatenates two arbitrary env vars instead of detecting the credential shape (Bug)

- **What happened:** The generated `config.AuthHeader()` built credentials as `c.FreshserviceApikey + ":" + c.FreshserviceDomain`, i.e. base64 of `"apikey:domain"`. Freshservice's HTTP Basic uses `apikey` as the username and an **empty** password — the wire shape is `base64("apikey:")`. The Freshservice API returned HTTP 401 on every request. We had to hand-patch the credential builder to drop the domain concatenation and emit `apikey + ":"`.

  The spec explicitly declared the right shape: `securitySchemes.basicAuth.description: "API key as username, empty string as password. Set FRESHSERVICE_APIKEY and FRESHSERVICE_DOMAIN."` plus `x-auth-vars` with ONE credential entry (`FRESHSERVICE_APIKEY`, `kind: per_call`). The generator ignored both signals.

- **Scorer correct?** N/A — surfaced in live testing.

- **Root cause:** `internal/generator/templates/config.go.tmpl:230`:
  ```
  credentials := c.{{resolveEnvVarField (index $basicAuthEnvVars 0).Name}} + ":" + c.{{resolveEnvVarField (index $basicAuthEnvVars 1).Name}}
  ```
  The template assumes Basic auth always has *exactly two* env vars to concatenate. The freshservice spec emitted two via `x-auth-vars` and the (implicit) `_DOMAIN` for tenant scoping; the template arbitrarily picked them as username/password. There's no profiler step distinguishing "single credential, empty password" Basic (Freshservice, Stripe, Mailgun) from "two credentials, both populated" Basic (Jira Cloud `email:apitoken`).

- **Cross-API check:** Every API using HTTP Basic auth where the empty-password convention applies. Concrete examples:
  - **Freshservice** — `apikey:` (this incident).
  - **Stripe** — `sk_live_xxx:` (REST API uses Basic with the secret key as username, empty password).
  - **Mailgun** — `api:key-xxx` (different shape: hardcoded username `api`, but still single-credential).
  - **Pingdom** — token as username, empty password.
  - **Jira Cloud** (counter-example, two-credential) — `email:apitoken` is the legitimate two-cred case.

  Five concrete APIs. Three need the single-cred fix; one (Jira) shows the counter-case that the fix must not break.

- **Frequency:** Every API using HTTP Basic with the single-cred-trailing-colon shape, plus every API where the spec author would expect the second env var (e.g., `_DOMAIN`) to populate the URL, not the password. The combined population is likely a majority of single-tenant Basic-auth APIs.

- **Fallback if the Printing Press doesn't fix it:** Users hit 401 on every request and assume their key is bad. Diagnosis is hard because the wire format isn't visible in normal CLI output; only a curl reproduction with explicit base64 reveals the issue. Without a fix, this is essentially "Basic auth shipped broken."

- **Worth a Printing Press fix?** Yes.

- **Inherent or fixable:** Fixable. The signal is in the spec: `securitySchemes.basicAuth.description` mentions "empty password" / "empty string as password"; `x-auth-vars` declares which env vars are `kind: per_call` credentials vs configuration. The profiler can detect these.

- **Durable fix:** In the spec parser / profiler, when classifying a Basic auth scheme:
  1. Count `x-auth-vars` entries with `kind: per_call, sensitive: true` (credential candidates).
  2. If exactly 1 → emit `credentials := c.<Cred> + ":"` (single-cred trailing colon).
  3. If exactly 2 → emit the existing two-arg concatenation.
  4. If the description matches a regex like `/empty (string|password)/i` and there's at least 1 credential candidate, force single-cred shape.

  Then the template branches on the profiled shape rather than blindly indexing `[0]` and `[1]`.

  Less invasive alternative: add a `x-basic-auth-form: "username-only" | "username-password"` spec extension that the spec author sets explicitly. The generator honors it; spec authors who don't set it get the existing behavior. Less powerful (relies on spec author knowing to set it) but simpler to land.

- **Test:** Positive — spec with one `kind: per_call` env var in `x-auth-vars` and a description mentioning "empty password" → assert generated `AuthHeader()` emits `apikey + ":"`. Negative — spec with two `kind: per_call` entries (Jira shape) → assert two-arg concatenation. Regression — re-run for current Jira spec; assert unchanged behavior.

- **Evidence:** Live `doctor` returned 401; after the hand-patch to `c.FreshserviceApikey + ":"`, every endpoint worked. The spec had all the information the generator needed.

- **Related prior retros:** None.

---

### 4. Sync pagination defaults to body-cursor extraction even when the spec declares integer `page` pagination (Bug / Template gap)

- **What happened:** Freshservice paginates exclusively by `?page=N` (with the `Link: <…>; rel="next"` header for client convenience). The generated `sync` command defaulted `cursorParam: "after"` and tried to extract a next-page cursor from the response body via `extractPaginationFromEnvelope()`. Freshservice's body has no cursor — the response is `{"tickets": [...]}` flat. So `extractPageItems` returned `hasMore=false` after page 1, and every sync stopped at 100 records. Listing 10,000 tickets required hand-patching the sync loop to synthesize a numeric page cursor.

  The spec **declared** the pagination shape: `parameters: page: {in: query, schema: {type: integer}}` was present on every list endpoint, and `info.description` mentioned the Link header. The profiler missed both signals and fell through to the template's `else` branch (`cursorParam: "after"`).

- **Scorer correct?** Verify and dogfood passed because they only test page-1 retrieval — the cap-at-100 bug doesn't surface in mock-mode runs. Scorer behavior here is *probe-too-shallow*, not *probe-wrong*; flagged as adjacent finding.

- **Root cause:** Two parts:

  (a) `internal/generator/templates/sync.go.tmpl:661` — `cursorParam: "{{if .Pagination.CursorParam}}{{.Pagination.CursorParam}}{{else}}after{{end}}"`. The fallback when the profiler returns no cursor name is `"after"`, an arbitrary choice that matches a few cursor-based APIs (e.g., Slack) but doesn't fit page-int APIs at all.

  (b) The spec profiler didn't populate `Pagination.CursorParam` from the spec's `page` integer param. The detection logic likely greps for known cursor field names (`cursor`, `next_token`, `after`, etc.) without recognizing `page: integer` as a distinct paginator shape that needs a different iteration strategy.

  Uncertainty: I haven't traced the profiler code that populates `Pagination.*`. Candidates: `internal/spec/` or `internal/openapi/`. An implementer should grep for `Pagination.CursorParam` assignments.

- **Cross-API check:** Every API using `?page=N` integer pagination + Link headers. Concrete examples:
  - **Freshservice** — page integer (this incident).
  - **GitHub** — `?page=N&per_page=N` for listing endpoints; Link header for navigation. Massively common usage.
  - **Atlassian Cloud (Jira/Confluence)** — `?startAt=N&maxResults=N` (offset-based, related shape).
  - **HubSpot** — mixed: some endpoints use `?limit=N&after=cursor`, others use `?offset=N`.

- **Frequency:** ~30-40% of REST APIs use page-int or offset pagination instead of cursor. Currently those CLIs silently truncate at the default page size.

- **Fallback if the Printing Press doesn't fix it:** Sync caps at the first page on these APIs. Users either don't notice (data quietly missing for novel-feature analytics like breach-risk) or hit it once their tenant grows past the page size and complain. The fallback is "hope users use --max-pages 0 and don't trust default behavior."

- **Worth a Printing Press fix?** Yes. Silent data truncation is a particularly bad failure mode because nothing flags it — the sync completes successfully with `total: 100`.

- **Inherent or fixable:** Fixable. Profiler detection is straightforward; template just needs a third branch for page-iteration.

- **Durable fix:** Profile the spec for pagination shape and emit one of three iteration strategies:
  - **Body cursor** (current default) — `?<cursor>=<value>` from a body field. Used when the spec's list response declares a `next_cursor` / `paging.next.after` / `links.next` body field.
  - **Page integer** — `?page=N` iteration. Used when list endpoints declare `page: {type: integer}` query param and don't declare a body cursor.
  - **Link header** — parse `Link: <…>; rel="next"`. Used when `info.description` or response headers mention RFC 5988, or as a fallback when body cursor extraction fails on the first page but a page-int param is present.

  Template emits the right loop shape based on profiled paginator. The current sync.go.tmpl loop structure is mostly reusable — just swap the "advance the cursor" block.

- **Test:** Positive — generate from a spec with `page: integer` param and no body cursor; assert the sync loop emits `params["page"] = strconv.Itoa(currentPage + 1)`. Negative — generate from a spec with `next_cursor` in response schema; assert body-cursor extraction is still the chosen strategy. Regression — re-run against current cursor-based CLIs (e.g., Stripe `starting_after`).

- **Evidence:** Sync stopped at 100 tickets despite tenant having 10,000+. Hand-patched the loop with `if pageSize.cursorParam == "page" && nextCursor == "" && len(items) >= pageSize.limit { nextCursor = strconv.Itoa(currentPage + 1) }`. After patch, 100→200→300→… → 10,000 (cap hit), clean terminate.

- **Related prior retros:** None.

---

### 5. `doctor` reports "reachable" without validating credentials or detecting wrong-host HTML responses (Template gap)

- **What happened:** `doctor` probed `GET /` (the base URL root) and reported `api: reachable` plus `credentials: present (not verified)`. For Freshservice's API tenant the root 404s — doctor counted that as reachable since the server responded. For the wrong host (`<org>-org.myfreshworks.com`, the Freshworks org dashboard) the root returned `200 text/html` (the dashboard SPA) — doctor counted that as reachable too. Users had no way to know their key actually worked until they ran a real command and got 401 / HTML-parse errors.

- **Scorer correct?** N/A — scorecard didn't flag this.

- **Root cause:** `internal/generator/templates/helpers.go.tmpl` (or root.go.tmpl, wherever the doctor command body lives). The template emits a `c.Get("/", nil)` probe and reports `credentials: present (not verified — set auth.verify_path in spec for an API acceptance check)`. The TODO is even in the generated source. The "set auth.verify_path in spec" hint suggests the generator already has an opt-in mechanism but it requires every spec author to remember to set it.

- **Cross-API check:** Every CLI's doctor should validate auth. The wrong-host detection specifically matters for multi-tenant APIs where the wrong subdomain returns 200+HTML:
  - **Freshservice** — `<org>-org.myfreshworks.com` returns the Freshworks dashboard SPA (this incident).
  - **Atlassian Cloud** — wrong site returns `docs.atlassian.com` redirect or marketing pages.
  - **Auth0** — wrong tenant returns `tenant-not-found.auth0.com` landing page (HTML).
  - **Slack** — wrong workspace returns `app.slack.com` redirect.

  Multi-tenant + wrong-host is one class. The broader class (every CLI should auth-validate) is universal.

- **Frequency:** Every CLI. Doctor today is a structural smoke test — green-lights configurations that will fail on the first real call.

- **Fallback if the Printing Press doesn't fix it:** Users discover bad auth on their first real command, often with a less-actionable error message than doctor would have given. For multi-tenant APIs with wrong-host traps, the diagnosis is much harder — commands "succeed" with HTML in the response body, agents try to parse JSON from HTML and fail confusingly.

- **Worth a Printing Press fix?** Yes. Doctor is the first thing users run; making it earn its keep is high-leverage.

- **Inherent or fixable:** Fixable.

- **Durable fix:** Doctor should:
  1. Pick a probe endpoint at generation time. Default: the first `GET` endpoint with `security: [<authScheme>]` required, with no required path or query params, that returns a small response. Spec authors override with `x-doctor-probe: /path` (which the template already mentions in its TODO).
  2. Issue the probe with the configured auth header. Branch on response:
     - 200 + JSON content-type → `OK (authenticated)`. Optionally extract a known field (`email`, `name`, `agent.email`) for `OK (authenticated as <person>)` if a `x-doctor-identity-field` extension declares one.
     - 200 + HTML/non-JSON content-type → `FAIL: probe returned HTML, not JSON — base URL is probably the wrong host. Check <ENV_VAR> for the canonical API tenant.`
     - 401 / 403 → `FAIL: credentials rejected. Check <ENV_VAR> against the API's credential page.`
     - 404 → `FAIL: probe endpoint not found at <base_url><probe-path> — base URL may be wrong.`
     - Network failure → `FAIL: <transport error>`.

  The wrong-host content-type check is the key novel piece; the rest just makes doctor authenticate instead of reporting "present (not verified)".

- **Test:** Positive — point at a working live API; assert `OK (authenticated as <identity>)`. Negative — bad key → `FAIL: credentials rejected`. Negative — wrong host that returns HTML → `FAIL: probe returned HTML`. Negative — no spec.yaml `x-doctor-probe`, no auth-required GET endpoint → graceful fallback to current behavior.

- **Evidence:** Original doctor said `reachable` + `present (not verified)` against `<tenant>-org.myfreshworks.com` (the wrong host); every actual command then failed with HTML responses. Hand-patched doctor probes `/agents/me`, checks Content-Type, and reports `OK (authenticated as <REDACTED:tenant-agent-email>)` on success. All five failure modes now produce actionable messages.

- **Related prior retros:** None.

---

## Prioritized Improvements

### P1 — High priority

| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---|---|---|---|---|---|---|
| F2 | BaseURL substitutes spec-declared `{placeholder}` variables from env vars at runtime | generator | every multi-tenant API (~5+ in catalog) | low — every live call fails | small | only emit substitution helper when spec declares variables block |
| F3 | HTTP Basic auth detects single-credential / two-credential shape from spec | generator | every Basic-auth API | low — 401 on every call | small | check existing Jira-shape spec for regression |
| F4 | Sync profiles spec for page-int / Link-header / body-cursor pagination | spec-parser + generator | ~30-40% of REST APIs | low — silent data truncation | medium | preserve cursor-based default for current cursor APIs |

### P2 — Medium priority

| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---|---|---|---|---|---|---|
| F1 | Boolean body fields always sent (or sent via `cmd.Flags().Changed`) | generator | every API with boolean body fields | medium — silent semantic inversion | small | only changes booleans, not ints/strings |
| F5 | Doctor performs authenticated probe with content-type check | generator | every CLI | medium — doctor is the first command users run | medium | requires spec annotation `x-doctor-probe` or auto-pick from auth-required GETs |

### Skip

| Finding | Title | Why it didn't make it |
|---|---|---|
| G | Lookup-by-name resolver for path-param GET endpoints | Step G: case-against stronger. The pattern is per-API specific (Freshservice's `?filter=name:'x'` vs GitHub's `/search?q=x` vs Stripe's `?email=x`), and a generic resolver would need vendor-specific query DSL knowledge or per-spec annotations. A spec author who can annotate could just declare a `<resource> by-name` command directly. The 30-line hand-written resolver we built for freshservice is normal per-CLI feature work, not generator-template work. |

### Dropped at triage

| Candidate | One-liner | Drop reason |
|---|---|---|
| Filter `--query` double-quoting | Freshservice 500s without literal `"..."` around the query value | API-quirk (Freshservice-family specific; not generalizable) |
| Tenant-meta caching for custom enum codes | Custom status names (e.g. 9 = "BI Melding") need lookup from synced ticket-form-fields | printed-CLI (the recipe belongs in SKILL guidance for customizable APIs, not in the generator) |
| Empty slice → null in JSON for hand-written commands | `var x []T` emits `null`; `make([]T, 0)` emits `[]` | printed-CLI (the SKILL's RunE skeleton already documents make-zero-cap form; not a generator emission) |
| Bare flag descriptions (e.g. "Private") | Spec param descriptions are sometimes one word | printed-CLI (SKILL Priority 3 polish step already covers enrichment) |

## Work Units
(see Phase 5.5)

## Anti-patterns

- **Verify+dogfood passed at 100% while the CLI was unshippable.** The 4 cardinal infrastructure bugs (BaseURL, Basic auth, pagination, double-quoting) all passed mock-mode verify cleanly. A CLI that scores 84/100 and `shipcheck` PASSES can still have every live request fail. The scoring suite needs at least one live-key-required check for `doctor` to count as green — verify can't tell if HTTP calls land correctly because it doesn't make HTTP calls. The user's instinct to spawn a pre-live-test code review before any live testing was the only thing that caught these in-session; making that review (or a structurally similar live-tier check) a standard Phase 5 gate would have caught all four.
- **Spec annotations were present but ignored.** Both finding #2 (BaseURL) and finding #3 (Basic auth) had their answers in the spec — `servers[0].url` variables for one, the `description: "API key as username, empty string as password"` plus `x-auth-vars` for the other. The generator silently fell through to a wrong default in both cases. Stronger profiler detection beats more spec annotations because authors don't reliably add what's already implicit.

## What the Printing Press Got Right

- **The novel-features scaffold worked.** Once the 4 cardinal bugs were patched and the local store was hydrated, all 10 transcendence commands produced behaviorally-correct output against real data on the first run. The store-query skeleton from the SKILL plus the `dryRunOK` / `flags.printJSON` helpers made hand-writing the 9 commands mechanical.
- **The shipcheck umbrella organization is right.** Six legs covering different signal types (structural, runtime, narrative, behavior) caught real issues at each level. The umbrella's per-leg verdict surface let us re-run only the failing leg during fix loops without re-running expensive ones.
- **`validate-narrative --full-examples` saved us.** Every quickstart/recipe example in research.json was actually executed under `PRINTING_PRESS_VERIFY=1` before publish — caught three SKILL/README examples with bad flags before they shipped (we saw 3 failures, fixed them, re-validated clean). That's exactly the catch this leg exists for.
- **Manuscripts archive structure.** Having `research/`, `proofs/`, and the JSON gate markers in one place per run meant the retro could reconstruct the full session shape from artifacts even without the conversation history.
