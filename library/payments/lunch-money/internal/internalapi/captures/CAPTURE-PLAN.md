# Capture plan — remaining internal-API endpoints

What's still uncaptured in `internal/internalapi/captures/INVENTORY.md`. Roughly ordered
from cheapest+safest to hardest+riskiest. Each row tells a future session/subagent
exactly what to do.

## Ground rules (apply to every capture below)

- Drive Chrome MCP from `~/code/experiments/lunch-money-pp-cli/` with the existing
  cookie jar at `~/.config/lunch-money-pp-cli/internal-cookies.json` already valid
  (auto-refresh handles JWT expiry).
- Install the fetch+XHR interceptor BEFORE the triggering click; clear `window.__cap`
  right before; read `window.__cap` after. Canonical snippet is in
  `captures/transactions-mutations.md` step 3.
- For every mutation: reverse it in-session via the inverse UI action. Use test/dummy
  values (`ZZZZ_…_DELETE_ME` patterns) so accidental persistence is easy to find.
- Append findings to a per-resource file under `internal/internalapi/captures/`. Update
  `captures/INVENTORY.md` with the new method+path row.
- Sketch the Go method signature in `internal/internalapi/endpoints.go` first (panic
  body), implement after the wire format is confirmed, add a Cobra subcommand last.

---

## Tier 1 — Cheap, safe, high payoff

### 1. Plaid account refresh trigger
- **Page**: `/accounts` → click the refresh icon next to any Plaid-linked account
  (the cloud-with-arrows badge near "Active") — but NOT one currently in the middle
  of an import.
- **What we have**: bundle references `trigger_fetch` but `POST/PUT /v2/plaid_accounts/{id}/trigger_fetch`
  returns 404. The real path is unknown.
- **Capture target**: probable shapes are `POST /plaid_accounts/{id}/fetch`,
  `POST /plaid/items/{item_id}/fetch`, `POST /v2/plaid_accounts/{id}/sync`, or a different verb.
- **Reversal**: none needed — refresh is idempotent.
- **Risk**: low. Worst case the account briefly shows "syncing".
- **Wire target**: 1 method `TriggerPlaidFetch(plaidAccountID int64)`.

### 2. Recurring item EDIT
- **Page**: `/recurring` (currently empty in this account, so we'd need to first
  promote a suspected recurring transaction to a recurring item, or capture against
  the test rule's `recurring→{id}` linked items already in the system —
  e.g. recurring_id 12345 (Apple)).
- **Capture target**: `PUT /recurring_items/{id}` body shape. Probably mirrors public
  API but might have extra internal fields.
- **Reversal**: edit back to original values immediately.
- **Risk**: low. Recurring items are metadata, not transactions.

### 3. Suggested-rule acceptance flow
- **Page**: `/rules` → "Suggested Rules" tab at the left sidebar.
- **What we have**: `GET /rules/suggested?criteria_id=X` works (returns suggested
  expansions for an existing rule). The "accept this suggestion as a new rule"
  flow probably hits `POST /rules` with a suggestion-marked body, or a dedicated
  `/rules/suggested/{id}/accept`.
- **Capture target**: 1-2 endpoints.
- **Reversal**: immediately delete the newly-accepted rule (`POST /rules/bulk_delete`).
- **Risk**: low.

### 4. Budget set/clear per category
- **Page**: `/budget/2026/05` → click any category's budget cell → edit dollar amount → save.
- **What we have**: `GET /summary` returns budgeted amounts but the WRITE path
  (`/budgets`, `/budgets/{id}` etc.) all 404'd on GET. Probably needs different
  HTTP verb on existing path.
- **Capture target**: `PUT /budgets` or `PUT /categories/{id}/budget` or similar.
  Body likely includes `category_id`, `amount`, `start_date`, `currency`.
- **Reversal**: set the budget back to its prior value (note the old value before changing).
- **Risk**: low. Budgets are user-facing config, not money movement.

### 5. Autocategorize write paths (3 endpoints)
- **Page**: `/categories/auto`
  1. **Set Plaid→LM mapping** — change one detailed category's "Assign to Category"
     dropdown to a different LM category, then change it back.
  2. **Re-run categorization** — click "RE-RUN CATEGORIZATION" with currently 0
     uncategorized matches (so the run is a no-op).
  3. **Create missing categories** — click "CREATE MISSING CATEGORIES (0)" with
     count=0 so nothing actually changes.
- **Capture target**: probably `PUT /plaid/categories/{detailed_slug}`,
  `POST /plaid/categories/run`, `POST /plaid/categories/create_missing`.
- **Reversal**: only step (a) needs explicit revert — change the mapping back. (b)
  and (c) are no-ops when their counters are 0.
- **Risk**: low if executed with 0 affected transactions.

---

## Tier 2 — Settings sub-pages

### 6. Profile updates
- **Page**: `/profile` → edit budget name, primary currency, etc., save.
- **Capture target**: `PUT /me` body shape. Fields: `name`, `account_display_name`,
  `primary_currency`, `phone`, etc.
- **Reversal**: re-edit to original values.
- **Risk**: low (cosmetic), but be careful not to change primary_currency.

### 7. Notification / email preferences
- **Page**: `/settings` (under the notifications section if present)
- **Capture target**: `PUT /me/preferences` or `PUT /settings` or `PUT /me/notifications`.
- **Reversal**: flip back.
- **Risk**: low. Worst case: temporary opt-in/out of an email.

### 8. Date-format / week-start / locale prefs
- **Page**: `/settings` → "Set Date Format Preferences" button visible from the
  settings landing.
- **Capture target**: `PUT /me` with date format fields, or a dedicated
  `/preferences` endpoint.
- **Reversal**: revert.
- **Risk**: low.

---

## Tier 3 — Crypto + budget meta

### 9. Manual crypto CRUD
- **Page**: somewhere under `/accounts` → "Add account" → "Link my cryptocurrency
  wallet" path (creates a manual crypto asset).
- **Capture target**: `POST /v2/crypto/manual`, `PUT /v2/crypto/manual/{id}`,
  `DELETE /v2/crypto/manual/{id}`. Public API has these but the internal might
  use a different shape.
- **Reversal**: delete the test crypto.
- **Risk**: low.

### 10. Recurring-item dismiss / promote
- **Page**: `/recurring` → "Suggested Recurring" — accept/dismiss buttons.
- **Capture target**: probably `POST /recurring_items` (promote suggestion to
  real item) and `POST /recurring_items/{id}/dismiss` or similar.
- **Reversal**: delete the promoted item / un-dismiss.
- **Risk**: low.

---

## Tier 4 — Export / data download

### 11. CSV / JSON export of transactions
- **Page**: `/transactions` → look for "Download" button (the icon next to
  the gear and refresh icons in the toolbar — ref `_20`/`_21` in past page reads
  pointed to the download icon).
- **Capture target**: `GET /transactions/export` or `GET /exports` with date params,
  returning either a CSV blob or a signed URL.
- **Reversal**: none — read-only.
- **Risk**: zero, but the response body might be huge (full transaction history).

### 12. Export account-level data
- **Page**: `/billing` or `/settings` → GDPR/data-export button if present.
- **Capture target**: `POST /me/data_export` or `POST /gdpr/export`.
- **Reversal**: nothing to undo, but the actual export job may run async (poll endpoint
  to capture too).

---

## Tier 5 — File uploads (BLOCKED by Chrome MCP)

These three rows are tagged as blocked because Chrome MCP's `file_upload` tool
returned `{"code":-32000,"message":"Not allowed"}` for both `.txt` and `.pdf` files
when subagent 2 tried.

### 13. Transaction attachment upload
- **Page**: `/transactions` → click a transaction → "Add attachment" button.
- **Capture target**: `POST /v2/transactions/{id}/attachments` (newer typed client)
  OR `POST /transactions/file/{id}` (legacy path). Body is multipart/form-data.
- **Reversal**: delete the attachment after capture.
- **Workaround for the MCP block**: use the CLI's existing `internal request` with
  a manually-constructed multipart body (the internal client doesn't yet support
  multipart, so this would also need adding `DoMultipart` to `client.go`). OR
  use raw `curl` from the shell with the persisted cookie jar (export the jar as
  a `Cookie:` header and POST a multipart body).
- **Risk**: low if reversed.

### 14. PDF statement parsing
- **Page**: `/transactions` → "Import" dropdown → "Upload PDF statement".
- **Capture target**: `POST /transactions/file/pdf` (multipart). Returns
  `{data:{uuids:[...], processing:true}}`. Poll `POST /transactions/file/pdf/status`
  with `{uuids:[...]}` until processing:false.
- **Reversal**: do NOT import the parsed transactions (the post-parse modal has
  a "Cancel" button). If they import accidentally, delete them.
- **Workaround**: same as above — manual multipart via shell + cookie.
- **Risk**: medium if the parsed transactions accidentally commit. Use a real
  PDF statement for capture (the upload is parsed; bogus PDFs may 4xx).

### 15. CSV bulk import
- **Page**: `/transactions` → "Import" → upload CSV.
- **Capture target**: `PUT /transactions/bulk_insert/check` (dry-run) +
  `PUT /transactions/bulk_insert` (commit). Body is JSON, not multipart, after
  the file is parsed client-side. So this is actually CAPTURABLE without file
  upload — just feed the CSV through the column-mapping UI and stop at "Preview".
- **Reversal**: don't commit. If committed, `POST /transactions/bulk_delete`.
- **Risk**: low if stopped at preview.

### 16. Import-config presets
- **Page**: comes up during CSV import — "Save this mapping as a preset".
- **Capture target**: `GET /import_configs`, `PUT /import_configs` body shape.
- **Reversal**: delete the test preset.
- **Risk**: low.

---

## Tier 6 — Auth / 2FA

### 17. 2FA setup / verify
- **Page**: `/profile` or `/settings` → "Two-factor authentication" if available.
- **Capture target**: `POST /auth/2fa/setup`, `POST /auth/2fa/verify`,
  `GET /auth/2fa/backup_codes`.
- **Reversal**: disable 2FA after capture. **HARD STOP if disabling requires
  knowing the current TOTP secret** — that's a backup-codes flow which we don't
  have.
- **Risk**: **medium-high**. If 2FA gets enabled and we can't disable cleanly,
  account access depends on the saved TOTP secret. Only do this if you're willing
  to set up an authenticator alongside.

---

## Tier 7 — Misc / nice-to-have

### 18. Password change
- **Page**: `/profile` → "Change password"
- **Capture target**: `PUT /me/password` or `POST /auth/password/change`.
- **Reversal**: change it back via the same flow.
- **Risk**: medium — if the change succeeds but capture failed mid-way, the new
  password needs to be known.

### 19. Email change / verification
- **Capture target**: `PUT /me/email`, plus the verify-token endpoint.
- **Risk**: medium — emails get sent.

### 20. Trial / promo / referral redemption
- **Page**: `/refer`
- **Capture target**: probably `POST /referral/redeem` or similar.
- **Risk**: low (won't redeem on your own account).

---

## Suggested execution order

1. Tier 1 in one session (4-5 endpoints, 30 minutes, all reversible).
2. Tier 4 next (CSV/JSON export — pure read, huge value for offline backups).
3. Tier 2 if you want a full "personal info via CLI" surface.
4. Tier 5 #15 (CSV import via JSON payload — capturable without file upload).
5. Defer Tier 5 #13/#14, Tier 6, Tier 7 unless they become useful — they're
   either blocked or risky.

After each tier, append to `captures/INVENTORY.md` and run `go install` so the new
methods are immediately usable. Don't add a Cobra subcommand for every endpoint —
`internal request` is the escape hatch until something gets used 2+ times.

## Stop-the-world checks

- If the cookie jar fails the auth-status probe, stop and re-seed the cookie
  before continuing — don't try to debug refresh in the middle of a capture run.
- If Cloudflare turnstile fires (unlikely on api.lunchmoney.app but possible),
  stop and check rate.
- Never reverse `should_delete:true` rules (rule-engine actions) by deleting
  the transaction again — verify the captured body shape via bundle inspection
  first.
