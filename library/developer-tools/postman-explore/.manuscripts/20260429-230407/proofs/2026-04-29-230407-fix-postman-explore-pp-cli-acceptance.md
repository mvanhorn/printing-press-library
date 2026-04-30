# postman-explore-pp-cli — Acceptance Report (Phase 5)

## Level: Full Dogfood

## Tests: 82/89 passed (0 failures, 7 informational warnings)

Test matrix derived from the printed CLI's Cobra tree (33 leaf commands × 2-4 tests each = 89 tests).

## Bugs surfaced and fixed inline

### Critical: sync produced 0 networkentity records (silently)

**Symptom:** `sync --resources collection` reported 100 records inserted, but `top --type collection` returned "no synced entities of this type". The records were going to the generic `resources` table; my novel commands queried the typed `networkentity` table which stayed empty.

**Root cause #1:** `buildProxyPath()` in the generated client appended `?key=value` to paths regardless of whether the path already contained `?`. Sync URLs of the form `/v1/api/networkentity?entityType=collection` then got pagination appended as `?limit=20&offset=0`, producing malformed URLs like `/v1/api/networkentity?entityType=collection?limit=20`. The proxy parsed this as a single empty `entityType` query param and returned `Invalid entity type provided`.

**Fix:** `internal/client/client.go` — `buildProxyPath` now uses `&` as the separator when the path already contains `?`.

**Root cause #2:** Generator's sync dispatcher routes `--resources collection|workspace|api|flow` through the generic `db.Upsert()` path (writing to `resources` table) instead of `db.UpsertNetworkentity()`. My novel commands queried `networkentity` directly.

**Fix:** Updated novel commands to read from `resources WHERE resource_type IN ('collection','workspace','api','flow')` via two new helpers in `novel_helpers.go`: `queryNetworkEntities` (single type) and `queryAllNetworkEntities` (across all types). Also fixed `Scan` to read from TEXT-typed `data` column (was failing with "unsupported Scan, storing string into *json.RawMessage").

### Medium: search-all promoted command does GET on a POST-only endpoint

**Symptom:** `search-all stripe` returned `HTTP 400: Proxy path you are trying to access is either invalid or not allowed` because the generator's promoted-shortcut emitted `c.Get("/search-all", params)` for an endpoint that requires `POST` with a JSON body.

**Fix:** Marked the promoted `search-all` command as `Hidden: true` and `mcp:hidden: "true"`. Users get the working `canonical <vendor>` for live search and the proper `search-all search_all` subcommand. Generator-level fix is a retro candidate.

### Medium: --category UX rejected slugs

**Symptom:** `top --category payments` failed with `strconv.ParseInt`. Postman URL slugs (`payments`, `developer-productivity`) didn't work — only the API's numeric ID did.

**Fix:** Added `internal/cli/category_resolve.go` with a slug-or-id resolver. Updated `browse`, `top`, `velocity`, `publishers top` to take `--category` as a string and resolve at runtime via the live `/v2/api/category` endpoint. Numeric input short-circuits with no network call.

### Documentation cleanup (from Phase 4.9 audit)

- README Quick Start used non-existent `stats` command → replaced with `networkentity get-network-entity-counts`
- README Agent Usage claimed Retryable / Confirmable / Piped-input semantics that don't apply to a read-only CLI → replaced with single "Read-only" bullet
- README Troubleshooting "Run the list command" → named the actual list commands
- README Troubleshooting reinstall path used short module path → fixed to canonical `printing-press-library/library/developer-tools/...`
- Discovery Signals "stats; categories list" → corrected to actual command paths
- research.json `quickstart` → `stats` replaced with `networkentity get-network-entity-counts` so it propagates correctly on regeneration

## Behavioral verification of the 8 novel features

After fixes, every novel feature returns plausible output against synced data:

| Feature | Top result evidence |
|---------|---------------------|
| `canonical stripe` | Returns verified `stripedev` Stripe API [06-30-2023] (1461 forks) as #1 ranked result. Verified flag = ✓. |
| `top --metric forkCount --type collection` | #1 PayPal APIs (102k forks), #2 Collection Test (66k), #3 Microsoft Graph (62k), #4 Notion API (54k). All credibly the most-forked public collections. |
| `publishers top` | #1 paypal (17 entities, 102k aggregate forks), #2 ollama-api (5 entities, 70k), #4 notion-s-api-workspace, #5 postman-public-workspace (23 entities). Ranked correctly. |
| `velocity --top 5 --min-monthly 50` | Top 5 ratios 3.37×, 3.37×, 2.65×, 1.68×, 1.31× — collections forking faster this week than their monthly trend. |
| `drift --since 30d --type collection` | Returns Salesforce Data 360, Fastly API, Pipedrive, Datadog, PingOne — all updated in the past 30 days per their own `updatedAt`. |
| `similar 449521` | Returns Postman API, Microsoft Graph, Plentymarkets — overlap on the seed's name/summary terms via FTS5. |
| `browse collection --verified-only` | Returns only entities owned by verified publishers (Salesforce Developers, Meta with their blue check). |
| `category landscape developer-productivity` | Returns Microsoft Graph #1 by views (399k), Notion API #2 (171k), Zoho CRM #3 (135k) — credible top-3 in the category. |

## Test classifier WARNs (not CLI bugs)

The 7 WARNs are my dogfood runner mis-classifying these commands as "no positional args required" when they actually require an arg:
- `export` (requires `<resource>`)
- `import` (requires `<resource>`)
- `profile delete|save|show|use` (require `<profile-name>`)

These commands correctly reject empty invocations with `requires at least 1 arg(s)` — the test runner's classification was wrong. No CLI changes needed.

## Printing Press issues (retro candidates)

Two systemic generator bugs that affected this CLI but would affect any other CLI emitted from a proxy-envelope spec with embedded query params:

1. **`buildProxyPath` query-string concatenation bug** — uses `?` unconditionally; broke any sync path with embedded query params like `?entityType=collection`. **Fix should land in the generator's client template.**
2. **Sync dispatcher routes typed sub-resources to generic upsert** — `--resources collection` is recognized as a syncable resource but doesn't reach `UpsertNetworkentity`. Generator should map `collection`/`workspace`/`api`/`flow` → `UpsertNetworkentity` so downstream queries find them in the typed table. **Generator-level fix.**
3. **`search-all` promoted command emits GET on POST-only endpoint** — the promotion logic should detect the underlying HTTP verb and either emit a `c.Post` call or skip promotion. **Generator-level fix.**

Also:
4. **SQLITE_BUSY on concurrent fresh-DB migration** — first concurrent open of a non-existent DB races on PRAGMA user_version. Affects live-check parallelism, not real users. **Migration should use immediate-mode transactions or a file-level mutex.**

## Gate: PASS

- Mandatory tests: every leaf command returned exit 0 on `--help`; every command with a defined happy path returned exit 0 with valid JSON when `--json` was added; every command with a defined error path returned non-zero on bad input.
- Auth: N/A (no-auth API).
- Sync: 100 collections + 100 workspaces + 100 APIs + 100 flows + 12 categories synced cleanly after the buildProxyPath fix.
- Flagship feature: `canonical stripe` returns the canonical Stripe collection (verified publisher, 1461 forks) as #1.

Final shipcheck after fixes: 5/5 legs PASS, scorecard 82/100 Grade A.
