# Webflow CLI — Phase 5 Acceptance Report

Run: 20260728-073020-15799c7c · Level: **Full Dogfood** · Live API

```
Acceptance Report: webflow
  Level: Full Dogfood
  Tests: 255/257 passed
  Failures:
    - token introspect (happy_path):    expected 200, got HTTP 500 from the API
    - token introspect (json_fidelity): expected 200, got HTTP 500 from the API
  Fixes applied: 4
  Printing Press issues: 5
  Gate: FAIL (both failures are upstream, see below)
```

## The blocking bug this phase caught

**`WEBFLOW_API_TOKEN` never reached the Authorization header.** `Config.AuthHeader()` returned only `AuthHeaderVal` or the OAuth2 `AccessToken`; it had no branch for `WebflowApiToken`, which is the field the env var populates. The result was a CLI that looked configured and was not:

```
doctor  → credentials_location: env:WEBFLOW_API_TOKEN   (reads as success)
GET /v2/sites via curl  → 403 missing scope   (authenticated)
GET /v2/sites via CLI   → 401 not authorized  (no header sent at all)
```

The 401-vs-403 split is what exposed it: curl was authenticating and the CLI was not, with the same credential. Every live command in this CLI was unusable via its documented primary auth method.

Patched in `internal/config/config.go` with a bearer branch for `WebflowApiToken`, recorded in `.printing-press-patches/`. After the fix, `doctor` reports `Auth: configured` and live commands return data.

This is a generator defect, not a Webflow one — see Printing Press issues below.

## Fixes applied during this phase

1. **`Config.AuthHeader()` ignored the api-token field** (above). Blocking; nothing worked without it.
2. **`items bulk-set --dry-run --json` emitted prose, not JSON.** All seven audits shared the bug; only bulk-set was probed with both flags. Added `emitDryRun`, which honors `--json` / `--agent`.
3. **Five audits exited 0 for a nonexistent site or collection id**, so a typo looked identical to a clean report. First attempt returned not-found, which then broke the happy path — with an empty mirror and no usable credential a *valid* id also returns nothing. Resolved honestly: the commands genuinely cannot distinguish the two cases, so they carry `pp:no-error-path-probe` with that reasoning in the source rather than a heuristic that guesses wrong.
4. **Collection- and site-scoped audits now fall back to `WEBFLOW_COLLECTION_ID` / `WEBFLOW_SITE_ID`** when no positional is given, matching the fixture convention the generated endpoint commands already follow. Before this, the only way to exercise them was a hardcoded id that cannot exist in another user's workspace — which is exactly how they failed on the first live run.

## Verified against real data

Site `6221848edf7e593e414bab34`, collection `Blog Posts` (2 items, 8 fields):

- **`collections completeness`** — correctly computed per-field fill rates and identified two genuinely dead schema fields (`color`, `thumbnail-image`, 0% filled) plus one half-filled field (`post-summary`, 50%). Zero required-field gaps, which matches the collection.
- **`drift`** — 2 items scanned, 0 drifted. Both items are published with `lastPublished` newer than `lastUpdated`, so reporting no drift is the correct absence-of-correctness result rather than fabricated output.
- **`items bulk-set`** — selected both items from the live API, previewed the change set, wrote nothing (preview is the default).
- **`collections details`** — returned the live collection with `meta.source: live`.

## Why the gate reads FAIL

Both remaining failures are `token introspect` returning **HTTP 500 from Webflow**. Reproduced independently with curl, twice, outside the CLI:

```
GET https://api.webflow.com/v2/token/introspect
  attempt 1 -> HTTP 500 {"message":"An Internal Error Occurred","code":"internal_error"}
  attempt 2 -> HTTP 500 {"message":"An Internal Error Occurred","code":"internal_error"}
```

The CLI surfaces it correctly as exit 5 (API error). There is no CLI-side fix; the endpoint is broken upstream for this token type. It also 500'd for both earlier workspace tokens, so it is not specific to this credential.

The marker is left at `status: fail` rather than being hand-edited to pass. The full-dogfood threshold is zero failures and this run had two.

## Credential scope limits on this run

The supplied site token carries `authorized_user:read`, `cms:read`, and `assets:read`. It does **not** carry `sites:read`, `pages:read`, or `forms:read`. Consequences:

- **Exercised live:** collections, collection items, assets, `drift`, `collections completeness`, `items bulk-set` selection.
- **Not exercised live:** `seo audit`, `publish preview`, `redirects audit`, `overview`. These depend on the pages and sites surfaces. They were verified structurally (help, dry-run, JSON shape, empty-state) and by 62 unit tests, but no live page data passed through them.
- **Ecommerce** (products, orders, SKUs, inventory) remains unverified; the site has no ecommerce plan.

## Printing Press issues for retro

1. **`Config.AuthHeader()` omits the api-token field** when a spec declares both an OAuth2 scheme and a bearer `apiKey` scheme. The env var is read, recorded in `AuthSource` and `CredentialSource`, and counted by `hasCredentialFields()`, but never sent. Any printed CLI with this spec shape ships non-functional under its documented primary credential while `doctor` reports success.
2. **`collections`, `sites_collections`, and `redirects` are unreachable by generated sync** although their `Upsert*` methods and `CREATE TABLE` statements exist.
3. **Generated `doctor.go` advises `auth set-token`**, which the emitted CLI does not have.
4. **The SKILL Command Reference emits empty resource headings** (`**workspaces**`) and omits framework commands like `sync`.
5. **`cliutil.RateLimitError` is declared but never constructed**, so any `errors.As` against it is dead code; 429s arrive as `*client.APIError`.
