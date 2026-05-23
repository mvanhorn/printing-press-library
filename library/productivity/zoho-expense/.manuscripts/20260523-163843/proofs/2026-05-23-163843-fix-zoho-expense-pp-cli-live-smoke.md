# Zoho Expense CLI Live Smoke (Quick Check)

Run ID: 20260523-163843
Tenant: the authenticated test workspace (India region, www.zohoapis.in)
Auth: OAuth 2.0 self-client (authorization-code → refresh + access tokens)
Granted scopes: `ZohoExpense.orgsettings.READ`, `ZohoExpense.expense.READ|CREATE`, `ZohoExpense.expensereport.READ|CREATE|UPDATE`

## Quick Check Matrix (7/7 tests pass)

| # | Test | Result | Notes |
|---|---|---|---|
| 1 | `doctor` | ✓ PASS | Auth/Env/API/Credentials all OK. auth_source=oauth2; base_url=https://www.zohoapis.in/expense/v1. |
| 2 | `organizations list --json` | ✓ PASS | Returns the test workspace (India). |
| 3 | `reporting-tags list --json` | ✓ PASS | code=0, 0 tags configured on this test org. |
| 4 | `expenses list --per-page 5 --json` | ✓ PASS | Empty response (0 expenses on this org this month) — envelope valid, `applied_filter: ExpenseDate.ThisMonth`. |
| 5 | `sync --resources organizations,reporting_tags,expenses` | ✓ PASS | sync_summary: 3 resources, 1 record total (the organization), 0 errors. |
| 6 | `search "test" --data-source=local` | ✓ PASS | Valid meta envelope returned, empty results (0 synced expenses). |
| 7 | `merchant list --json` | ✓ PASS | Returns `[]` (no merchants yet — expected, since 0 synced expenses). |

## Findings (non-blocking)

1. **Zoho-envelope unwrapping (P1, polish target).** Every Zoho list endpoint returns `{"code": 0, "message": "success", "<resource>": [...]}`. The CLI's generated commands pass this through as `.results.<resource>` — agents and `--select` consumers must add an extra hop (`.results.organizations[]` instead of `[]`). Two clean fixes for v2:
   - Add a spec-level `extract: organizations` directive per endpoint that the generator uses to unwrap.
   - Or hand-edit `internal/client/zoho_envelope.go` with a response post-processor that strips the envelope if `code == 0`.
2. **Scope-narrow self-client (informational).** This run's self-client did not request `ZohoExpense.expensecategories.READ` or `ZohoExpense.projects.READ`, so those endpoints return `code: 57` (not authorized). Doesn't affect the matrix. For a full-coverage CLI, document that `ZohoExpense.fullaccess.ALL` is recommended for personal use, with narrower scopes for production hardening.
3. **`token_expiry = ""` TOML parse error (P2, retro).** Writing an empty string into the TOML `token_expiry` field caused a parse failure (`cannot decode TOML string into time.Time`). Workaround: omit the field entirely. Fix: generator should accept either empty string or omitted field and zero-value the Time. Filed for retro.
4. **`auth login` flag-only flow (P2).** The generator's `auth login` does browser-OAuth (localhost callback), but Zoho's self-client is the dominant pattern — exchanging a pasted 10-min code. Should add a `--code=<value>` flag path that skips the browser dance. Adjacent generator improvement.

## What was NOT tested (deferred to Full Dogfood)

- Receipt upload + autoscan polling (would create a test expense)
- `invoice ingest` batch flow
- `expense-tag` mutation (no UPDATE scope was granted on expenses)
- `close` (creating an expense report; would mutate state)
- `gst-split` (needs a real expense to operate on)
- `auth refresh` end-to-end (have a fresh access token from the exchange)

These all build and respond to `--help` and `--dry-run` correctly (verified during Phase 4 shipcheck). Mutation correctness against the live tenant is deferred until the user is ready to run a Full Dogfood with `--auto-tag`-style writes.

## Gate

**PASS** — all 6 required Quick Check tests passed (matrix threshold: 5/6). Auth and sync are healthy; doctor green; envelope shape is consistent. No blocking issues.

## Tokens issued (redacted)

```
access_token: REDACTED-ACCESS-PREFIX (70 chars, 1h validity)
refresh_token: REDACTED-REFRESH-PREFIX (70 chars, long-lived)
api_domain: https://www.zohoapis.in
organization_id: REDACTED-ORG-ID
```

The refresh_token is stored at `/tmp/zoho-quickcheck-config.toml` (session-local, mode 600). Not promoted to `~/.config/zoho-expense-pp-cli/config.toml` — the user can decide whether to persist via `zoho-expense-pp-cli auth login` (browser flow) or by copying the config later.
