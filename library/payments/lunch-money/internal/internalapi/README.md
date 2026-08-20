# Internal API — undocumented endpoints

This package wraps Lunch Money's web-UI backend at `api.lunchmoney.app`. The
endpoints are reverse-engineered from live browser captures (see `captures/*.md`)
and are not part of the documented v2 API.

**Why bother:** the documented public API at `api.lunchmoney.dev/v2` does not
expose rules (auto-categorization), the autocategorization Plaid taxonomy mapping,
or several bulk-edit primitives. The web UI uses these. This package gives the CLI
parity with the web UI.

**Auth model:** session cookie (issued by `my.lunchmoney.app` login) +
short-lived JWT access cookie. The client transparently calls
`/auth/token/refresh` on 401 with `Access token expired` and retries once.

## Endpoint inventory

| Method | Path | Covered by | Notes |
|---|---|---|---|
| GET | `/rules?offset=N&limit=M` | `rules list` | Pagination follows |
| POST | `/rules` | `rules create` | conditions/actions body |
| PUT | `/rules/{criteria_id}` | `rules update` | path uses `criteria_id` NOT `rule_id` |
| POST | `/rules/apply` | `rules apply [--dry-run]` | bulk apply or preview |
| POST | `/rules/bulk_delete` | `rules delete` | `criteria_ids` array |
| GET | `/plaid/categories` | (planned `autocategorize list`) | Plaid taxonomy + LM mappings |
| POST | `/plaid/categories/populate` | (planned) | refresh Plaid taxonomy |
| POST | `/v2/plaid_accounts/fetch?id={id}` | `internal plaid-accounts fetch` | queues Plaid background fetch |
| PUT | `/v2/budgets` | `internal budget set` | set category-period budget |
| DELETE | `/v2/budgets?category_id=&start_date=` | `internal budget clear` | clear category-period budget |
| GET | `/assets` | (planned `internal assets list`) | full account list incl. Plaid access_tokens (sensitive) |
| GET | `/assets/{id}/status` | (planned) | dependency check before delete |
| POST | `/assets` | (planned `internal assets create`) | manual account create |
| PUT | `/assets/{id}` | (planned) | manual account update |
| PUT | `/assets/{id}/delete` | (planned) | soft-delete (with keep_items flag) |
| GET | `/assets/subtypes` | (planned) | valid type/subtype enum |
| GET | `/transactions?is_unreviewed=true&...` | (planned `review list`) | richer than public list |
| GET | `/snapshot/transactions_page` | (planned) | one-shot page snapshot |
| PUT | `/transactions/{id}` | (planned) | status toggle ({status: cleared|uncleared}) |
| PUT | `/transactions/bulk_update` | (planned `review mark`) | universal bulk-edit — any field |
| PUT | `/transactions/bulk_insert/check` | client helper | CSV import dry-run |
| GET | `/import_configs` | `internal import-configs list` | saved CSV mappings |
| POST | `/auth/token/refresh` | (automatic in client) | JWT refresh |
| GET | `/system/status` | `internal auth status` probe | 204 when healthy |

## Auth setup

The session cookie isn't readable from JS (HttpOnly). To seed:

1. Open `my.lunchmoney.app` in Chrome while logged in
2. Open DevTools → Network tab
3. Click any `api.lunchmoney.app/...` request
4. Right side → Headers → Request Headers → find `Cookie:`
5. Copy the full value
6. Run: `lunch-money-pp-cli internal auth set-cookie '<paste here>'`
   (or `pbpaste | lunch-money-pp-cli internal auth set-cookie --stdin`)
7. Verify: `lunch-money-pp-cli internal auth status` → `ok: session valid`

The cookie jar lives at `~/.config/lunch-money-pp-cli/internal-cookies.json` with 0600
permissions. It is local-only and never sent anywhere except `api.lunchmoney.app`.
JWT access cookies are refreshed automatically on 401.

For CI or one-off shells, set `LUNCHMONEY_INTERNAL_COOKIE` to the same raw
`Cookie:` header value. This takes precedence over the persisted jar and is not
written back to `internal-cookies.json`.

## Privacy

- `/assets` response includes Plaid `access_token` for each linked institution.
  The planned `internal assets list` command MUST drop this field by default and
  require an explicit `--show-secrets` flag.
- The `internal-cookies.json` file contains your session secret — treat it like
  an API token. Don't commit it. Don't email it.
- `LUNCHMONEY_INTERNAL_COOKIE` is also a session secret. Prefer passing it only
  in a short-lived shell or CI secret store.

## Limitations

These endpoints are undocumented. They can change shape or disappear without
warning. The captures in `captures/*.md` are timestamped — re-capture if behavior
seems off.

If `auth/token/refresh` returns 401, the long-lived session cookie has expired
(typical lifetime ≈ 30 days). Re-seed via `internal auth set-cookie`.
