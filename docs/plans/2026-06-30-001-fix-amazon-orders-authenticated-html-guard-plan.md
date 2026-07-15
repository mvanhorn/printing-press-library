---
title: "fix(amazon-orders): reject login HTML and repair authenticated order reads"
type: fix
created: 2026-06-30
depth: standard
target_repo: printing-press-library
target_path: library/commerce/amazon-orders
artifact_contract: ce-unified-plan/v1
artifact_readiness: implementation-ready
product_contract_source: ce-plan-bootstrap
execution: code
---

# fix(amazon-orders): reject login HTML and repair authenticated order reads

## Goal Capsule

| Field | Value |
|---|---|
| Objective | Make `amazon-orders-pp-cli` fail honestly when Amazon returns login or challenge HTML, and stop the CLI from parsing or caching that HTML as real orders, invoices, or transactions. |
| Authority hierarchy | User runtime evidence from 2026-06-30; `library/commerce/amazon-orders/AGENTS.md`; repo patch convention in `AGENTS.md`; existing Amazon Orders CLI behavior and tests. |
| Execution profile | Code fix in the published library CLI, with characterization tests before behavior changes where practical. |
| Stop conditions | A live Amazon login page must never be returned as an unrelated order, a successful empty order list, an invoice page, or cached synced data. |
| Tail ownership | LFG should continue through implementation, review, verification, commit/PR, and CI per the active pipeline. |

---

## Product Contract

### Summary

`amazon-orders-pp-cli` currently treats Amazon sign-in HTML as successful data because Amazon returns the login page with HTTP 200. In the user runtime, `doctor` reported auth as valid and cache as fresh, but `orders get 702-5010515-8774615` returned an unrelated order-like payload, `orders list` produced no useful results, `find Aqara` found nothing, and `sync` stored sign-in pages for `orders` and `transactions`.

This fix makes authenticated HTML validation explicit. Commands that require logged-in Amazon pages must detect sign-in or bot-challenge pages before parsing and before writing to the local SQLite store. Local reads must also address endpoint identity correctly so order-detail cache lookups use the requested `orderID`, not the static path segment `order-details`.

### Problem Frame

Amazon buyer pages are authenticated, server-rendered HTML, not a documented buyer API. The current HTTP client correctly sees a network-level success when Amazon replies with status 200, but status 200 is not proof that the requested authenticated surface was delivered. The parser layer then makes the failure worse: order-detail parsing accepts any page with an order-shaped token, order-list parsing accepts a login page as an empty order list, invoice extraction reports the login page title, and sync writes raw login HTML into the local store.

The bug is high-impact for finance triage because the CLI may appear authenticated and current while withholding the actual order and payment evidence needed to classify a purchase.

### Requirements

**Authenticated content validation**
- R1. Authenticated Amazon HTML commands must reject login, sign-in, and bot/challenge pages before parsing user data.
- R2. `orders get <orderID>` must never return a parsed detail whose `orderId` differs from the requested `orderID`.
- R3. `orders list`, `find`, novel order-list commands, `transactions`, `orders invoice`, and sync must not treat sign-in HTML as successful empty data or successful page metadata.
- R4. `doctor` must validate the same authenticated order-history surface used by real commands, not only a saved browser-session proof file or generic `/` reachability.

**Cache and local-read correctness**
- R5. `sync`, live read-through caching, and local fallback must not persist or serve sign-in HTML as trusted SQLite data, including rows written by older binaries.
- R6. Local reads for order detail and invoice must key by the relevant request parameter, especially `orderID`, instead of deriving identity from the static path segment.

**Runtime configuration and operator safety**
- R7. The fix must preserve support for configured Amazon domains and base URLs, including Amazon Mexico setups that use `amazon.com.mx`, rather than hardcoding all validation and hints to `amazon.com`.
- R8. Error output must be actionable and safe: no cookies or auth material in messages; remediation should point to `amazon-orders-pp-cli auth login --chrome` and `amazon-orders-pp-cli doctor`.
- R9. Existing successful parses for legitimate order-list, order-detail, invoice, transaction, and search flows must keep working.

### Acceptance Examples

- AE1. Given Amazon returns `<title>Amazon Iniciar sesion</title>` or `<title>Amazon Sign-In</title>` for `/your-orders/order-details?orderID=702-5010515-8774615`, when `orders get 702-5010515-8774615 --json --data-source live --no-cache` runs, then the command exits non-zero with an auth/session hint and does not emit another order ID. The detector should handle accented and unaccented Spanish sign-in titles.
- AE2. Given Amazon returns a sign-in page for `/your-orders/orders`, when `orders list --data-source live` runs, then the command exits non-zero instead of returning `[]` or `results: null` as a successful order list.
- AE3. Given Amazon returns a sign-in page during `sync --resources orders,transactions`, when the sync completes, then those resources are reported as failed/auth-invalid and no sign-in page rows are written to the `resources` table.
- AE4. Given a local SQLite store contains a real parsed order detail keyed by `702-5010515-8774615`, when `orders get 702-5010515-8774615 --data-source local` runs, then the local lookup asks for that order ID, not `order-details`.
- AE5. Given a configured base URL of `https://www.amazon.com.mx`, when `doctor` validates credentials, then diagnostics and proof checks use the configured host/domain semantics and do not reject solely because the default spec domain is `.amazon.com`.
- AE6. Given an older local SQLite store already contains an `orders` or `transactions` row whose body is Amazon sign-in HTML, when any local or auto-fallback command reads it, then the command rejects or skips that row instead of treating the cache as fresh usable data.

### Scope Boundaries

In scope:
- Add a shared authenticated-page guard for Amazon login, sign-in, and challenge HTML.
- Apply the guard before parsing command responses and before cache/store writes.
- Tighten `doctor` and browser-session proof validation around real authenticated content.
- Fix endpoint identity for local order detail/invoice reads and list-vs-detail local reads where this CLI currently misroutes them.
- Add regression tests and a `.printing-press-patches/` entry.

Outside this PR:
- Building a new browser automation backfill flow.
- Rewriting Amazon parsers for every locale or A/B layout.
- Adding full CFDI/fiscal package processing. This CLI only provides the Amazon evidence surface.
- Changing the Printing Press generator globally unless implementation proves the defect is shared and cannot be solved as a CLI-local published-library patch.

### Open Questions

No blocking product questions remain.

Deferred:
- DQ1. Whether the upstream Printing Press generator should learn a generic "HTML auth-wall guard" for all authenticated scraping CLIs. This PR should record the Amazon-specific patch first; upstreaming can follow once the local fix is proven.

### Sources

- User runtime evidence from 2026-06-30: `doctor` reported configured auth/fresh cache while live/local order reads, `find Aqara`, and `sync` returned or stored sign-in pages.
- `AGENTS.md` for published-library patch metadata requirements.
- `library/commerce/amazon-orders/AGENTS.md` for CLI-specific runtime discovery and patching rules.
- `docs/patterns/authenticated-session-scraping.md` for the repo's authenticated scraping model.
- `library/commerce/amazon-orders/internal/client/client.go`
- `library/commerce/amazon-orders/internal/cli/data_source.go`
- `library/commerce/amazon-orders/internal/cli/sync.go`
- `library/commerce/amazon-orders/internal/cli/doctor.go`
- `library/commerce/amazon-orders/internal/cli/auth.go`
- `library/commerce/amazon-orders/internal/cli/orders_get.go`
- `library/commerce/amazon-orders/internal/cli/orders_list.go`
- `library/commerce/amazon-orders/internal/cli/orders_invoice.go`
- `library/commerce/amazon-orders/internal/cli/promoted_transactions.go`
- `library/commerce/amazon-orders/internal/parser/order_detail.go`
- `library/commerce/amazon-orders/internal/parser/orders_list.go`
- `library/commerce/amazon-orders/internal/parser/transactions.go`
- `library/commerce/amazon-orders/internal/store/store.go`
- `library/commerce/amazon-orders/spec.yaml`

---

## Planning Contract

### Key Technical Decisions

- KTD1. Put auth-wall detection in shared CLI/parser-adjacent code, not as one-off checks inside every command. The failure mode crosses live reads, parser entry points, doctor, sync, and cache write-through; a single tested classifier keeps behavior consistent.
- KTD2. Treat HTTP 200 plus sign-in/challenge HTML as an auth error. For authenticated scraping, content identity is part of auth validation. A status-code-only client success is insufficient for Amazon buyer pages.
- KTD3. Guard before persistence. It is not enough for parsers to reject login pages after the fact because `sync` and write-through caching can store raw HTML before a parser ever sees it.
- KTD4. Keep parser-level safeguards too. Command-level guards catch the common path, but parser tests should also prove login HTML and mismatched order IDs cannot accidentally become structured user data.
- KTD5. Repair local identity with explicit endpoint IDs. `resolveLocal` currently derives get-by-ID identity from the last path segment. Amazon detail and invoice endpoints carry the useful ID in `orderID`, so local lookup must use that parameter or a command-provided local ID.
- KTD6. Preserve configured host/domain behavior. The implementation should derive validation URLs and cookie-domain expectations from config/spec/runtime where available, so `.amazon.com.mx` sessions are not invalidated by `.amazon.com` constants.
- KTD7. Make auth failures durable and agent-readable. JSON mode should expose a stable error surface or existing exit-code path while human mode gives concise remediation, with no cookie leakage.
- KTD8. Use a library-side patch record. This is a published CLI repair under `library/commerce/amazon-orders`; add one `.printing-press-patches/<id>.json` entry capturing the durable behavior a reprint must preserve.

### High-Level Technical Design

The fix has two guard layers:

1. **Authenticated HTML classifier:** a small Amazon-specific helper inspects a bounded prefix/body text for sign-in and challenge markers, including English and Spanish titles, common sign-in form/action names, and existing interstitial markers. It returns a typed classification with a safe reason string.
2. **Read/persistence enforcement:** live, local, and cache-backed HTML reads call the classifier before handing bytes to parsers or SQLite. Login/challenge classifications return an auth-style error that `classifyAPIError`, sync, and doctor can render.

Local reads get a second correction: get-by-ID resolution accepts an explicit local ID derived from `orderID` where the endpoint uses query parameters rather than path IDs. List endpoints that naturally return pages of orders or transactions should use local list behavior instead of attempting to fetch a fake ID from the path.

### Implementation Constraints

- Keep edits scoped to `library/commerce/amazon-orders`.
- Do not introduce new external services or browser dependencies.
- Do not print cookies, raw auth headers, or full login HTML in errors.
- Prefer fixtures or small inline HTML samples over live Amazon calls in tests.
- Keep generated-code comments minimal and mark durable generated-tree customizations through `.printing-press-patches/`.

### Sequencing

1. Characterize login-page and mismatched-order failures with tests.
2. Add the shared classifier and typed auth error surface.
3. Apply the classifier to live reads, parsers, sync, and doctor.
4. Repair local endpoint identity/list behavior.
5. Add patch metadata and run verification/dogfood.

---

## Implementation Units

### U1. Characterize authenticated HTML failure cases

- **Goal:** Lock in the observed bad behavior with tests before changing production paths.
- **Requirements:** R1, R2, R3, R5
- **Files:**
  - `library/commerce/amazon-orders/internal/parser/parser_test.go`
  - `library/commerce/amazon-orders/internal/cli/data_source_test.go` (create if no better local test file exists)
  - `library/commerce/amazon-orders/internal/cli/sync_test.go` (create or extend if test seams are practical)
  - `library/commerce/amazon-orders/internal/cli/novel_helpers_test.go` (create if guarding `fetchOrderListPages` needs direct coverage)
- **Approach:** Add minimal fixtures for Amazon sign-in HTML in English and Spanish, plus an order-detail HTML snippet whose body contains a different order ID than the requested one. Prefer focused unit tests over live network.
- **Test Scenarios:**
  - Sign-in HTML is detected as unauthenticated.
  - Spanish sign-in titles, including accented and unaccented renderings, are detected as unauthenticated.
  - `ParseOrderDetail` plus the command/request validation path cannot accept a mismatched order ID.
  - `fetchOrderListPages` cannot turn sign-in HTML into an empty successful search/list result.
  - Local/cache-backed sign-in rows written by an older binary are rejected or skipped.
  - Empty legitimate pages and malformed HTML produce distinct errors from auth-wall pages.
- **Verification:** `go test ./internal/parser ./internal/cli -run 'Auth|Login|Sign|OrderID|Sync'` passes from `library/commerce/amazon-orders`.

### U2. Add shared Amazon authenticated-page guard

- **Goal:** Provide one tested classifier/error path for sign-in, login, and challenge HTML.
- **Requirements:** R1, R3, R8, R9
- **Files:**
  - `library/commerce/amazon-orders/internal/cli/authenticated_html.go` (create)
  - `library/commerce/amazon-orders/internal/cli/authenticated_html_test.go` (create)
  - `library/commerce/amazon-orders/internal/cli/helpers.go`
  - `library/commerce/amazon-orders/internal/parser/parser.go` (only if parser-level reuse belongs there)
- **Approach:** Implement a bounded-body classifier with safe markers such as Amazon sign-in titles, sign-in form/action identifiers, account login path fragments, and existing bot-interstitial markers. Return a typed error that maps to auth exit behavior and structured hints.
- **Test Scenarios:**
  - English `Amazon Sign-In` page is rejected.
  - Spanish `Amazon Iniciar sesion` page is rejected.
  - A normal order-list fixture with `order-card` content is not rejected.
  - A normal order-detail fixture with `ORDER #` and item links is not rejected.
  - Returned errors do not include raw cookie strings or full HTML.
- **Verification:** `go test ./internal/cli -run AuthenticatedHTML` passes.

### U3. Enforce the guard on live reads, local/cache reads, write-through, and sync

- **Goal:** Stop login HTML before it reaches parsers, command output, or SQLite, regardless of whether the bytes came from live Amazon, local fallback, or older cached rows.
- **Requirements:** R1, R3, R5, R8, R9
- **Files:**
  - `library/commerce/amazon-orders/internal/cli/data_source.go`
  - `library/commerce/amazon-orders/internal/cli/sync.go`
  - `library/commerce/amazon-orders/internal/cli/novel_helpers.go`
  - `library/commerce/amazon-orders/internal/cli/find.go`
  - `library/commerce/amazon-orders/internal/cli/where_is_my_stuff.go` (if direct guarding is needed beyond `fetchOrderListPages`)
  - `library/commerce/amazon-orders/internal/cli/arriving_soon.go` (if direct guarding is needed beyond `fetchOrderListPages`)
  - `library/commerce/amazon-orders/internal/cli/late.go` (if direct guarding is needed beyond `fetchOrderListPages`)
  - `library/commerce/amazon-orders/internal/cli/orders_get.go`
  - `library/commerce/amazon-orders/internal/cli/orders_list.go`
  - `library/commerce/amazon-orders/internal/cli/orders_invoice.go`
  - `library/commerce/amazon-orders/internal/cli/promoted_transactions.go`
  - `library/commerce/amazon-orders/internal/cli/promoted_gift-cards.go` (if the same authenticated HTML path applies)
  - `library/commerce/amazon-orders/internal/cli/promoted_shipments.go` (if the same authenticated HTML path applies)
- **Approach:** Wrap HTML response handling with `ensureAuthenticatedAmazonHTML` before parsing, before local fallback output, and before `writeThroughCache`. In `syncResource`, classify each fetched page before `extractPageItems` or `upsertSingleObject`. Guard `fetchOrderListPages` because `find`, `where-is-my-stuff`, `arriving-soon`, and `late` use that helper directly instead of `resolveRead`. Decide whether sync treats auth-wall pages as hard errors rather than warnings; because this means the session is invalid, hard error is the safer default.
- **Test Scenarios:**
  - `resolveRead` live mode returns an auth error for sign-in HTML.
  - `resolveRead` local mode rejects or skips a stored sign-in HTML row.
  - Auto mode does not write a sign-in page through to the store.
  - `syncResource` with a fake client returning sign-in HTML returns an error and stores zero rows.
  - `find` over a sign-in first page returns an auth error, not an empty result set.
  - Real-looking order list/detail fixtures still parse and, when eligible, write through to store.
- **Verification:** `go test ./internal/cli ./internal/store` passes.

### U4. Tighten parser and command validation for order identity

- **Goal:** Prevent unrelated order IDs from being emitted for a requested order.
- **Requirements:** R2, R8, R9
- **Files:**
  - `library/commerce/amazon-orders/internal/cli/orders_get.go`
  - `library/commerce/amazon-orders/internal/parser/order_detail.go`
  - `library/commerce/amazon-orders/internal/parser/parser_test.go`
- **Approach:** After parsing an order detail, compare the parsed `OrderID` with the requested argument. If the parsed ID is empty or mismatched, return a specific error that says the order detail page did not match the requested order and suggests re-running auth/doctor when the page looked like login. Keep parser extraction tolerant, but make the command authoritative about request/response identity.
- **Test Scenarios:**
  - Requested `702-5010515-8774615`, parsed `144-5062705-8396341` fails.
  - Requested and parsed IDs match succeeds.
  - Empty parsed ID from non-login malformed detail fails with a parse/content error, not success.
- **Verification:** `go test ./internal/parser ./internal/cli -run OrderDetail` passes.

### U5. Repair local read identity and list semantics

- **Goal:** Make local reads use meaningful Amazon resource identities.
- **Requirements:** R3, R6, R9
- **Files:**
  - `library/commerce/amazon-orders/internal/cli/data_source.go`
  - `library/commerce/amazon-orders/internal/cli/orders_get.go`
  - `library/commerce/amazon-orders/internal/cli/orders_list.go`
  - `library/commerce/amazon-orders/internal/cli/orders_invoice.go`
  - `library/commerce/amazon-orders/internal/cli/promoted_transactions.go`
  - `library/commerce/amazon-orders/internal/store/store.go` (only if store ID fallback must be extended)
  - `library/commerce/amazon-orders/internal/cli/data_source_test.go`
- **Approach:** Add a narrow way for generated/hand-authored commands to pass an explicit local resource ID, or teach `resolveLocal` to prefer known query parameters like `orderID` for get-by-ID HTML endpoints. Set list endpoints (`orders list`, `transactions`) to local list semantics rather than fake get-by-ID lookups. Keep the change local to Amazon Orders unless implementation reveals a safe generator-level helper already exists.
- **Test Scenarios:**
  - Local `orders get 702-5010515-8774615` asks the store for ID `702-5010515-8774615`.
  - Local invoice for the same order asks the store for the same order ID or documented invoice key.
  - Local `orders list` returns all synced order rows rather than looking up ID `orders`.
  - Local `transactions` returns synced transaction rows rather than looking up ID `transactions`.
- **Verification:** `go test ./internal/cli -run 'Resolve|Local|DataSource'` passes.

### U6. Make doctor validate live authenticated content and configured domains

- **Goal:** Make `doctor` report invalid credentials when the real order-history probe receives a login page, and preserve `.com.mx` style setups.
- **Requirements:** R4, R7, R8
- **Files:**
  - `library/commerce/amazon-orders/internal/cli/doctor.go`
  - `library/commerce/amazon-orders/internal/cli/auth.go`
  - `library/commerce/amazon-orders/internal/config/config.go` (only if domain/config derivation needs support)
  - `library/commerce/amazon-orders/internal/cli/doctor_test.go` (create or extend)
  - `library/commerce/amazon-orders/internal/cli/auth_count_cookies_test.go` (extend if cookie-domain behavior changes)
- **Approach:** Change credential validation from proof-file-only to proof plus live authenticated-content probe. The probe should use `cfg.BaseURL` and the validation path, inspect response content with the shared guard, and mark credentials invalid if sign-in/challenge HTML appears. Align browser-session proof `CookieDomain` checks with configured domain behavior instead of hardcoded `.amazon.com` where possible.
- **Test Scenarios:**
  - Doctor reports credentials invalid when a stub server returns sign-in HTML at `/gp/your-account/order-history`.
  - Doctor reports credentials valid when the stub server returns a minimal authenticated order-history page.
  - Doctor diagnostics include the configured base URL.
  - A `.amazon.com.mx` or `https://www.amazon.com.mx` config path is not rejected solely due to the default `.amazon.com` constant.
- **Verification:** `go test ./internal/cli -run Doctor` passes.

### U7. Record the published-library patch and run CLI dogfood

- **Goal:** Make the behavior durable across future reprints and prove it against the user's failure mode.
- **Requirements:** R1, R2, R3, R4, R5, R6, R7, R8, R9
- **Files:**
  - `library/commerce/amazon-orders/.printing-press-patches/authenticated-html-guard.json` (create)
  - `library/commerce/amazon-orders/README.md` (only if remediation copy needs a durable user-facing update)
- **Approach:** Add one patch metadata file with schema version 2, `base_run_id` and `base_printing_press_version` copied from `.printing-press.json`, and a summary focused on the durable behavior. Build and run the CLI locally. If the user's current Amazon session is still invalid, the live command should fail clearly rather than returning another order or caching login HTML.
- **Test Scenarios:**
  - Patch metadata validates as JSON and names the changed code files.
  - Built CLI returns a non-zero auth/content error for the known failing order when Amazon still serves login HTML.
  - Built CLI does not create or preserve sign-in rows in the local store during the dogfood sync attempt.
- **Verification:** `go test ./...`, `go build ./cmd/amazon-orders-pp-cli`, and targeted live smoke commands from `library/commerce/amazon-orders`.

---

## Verification Contract

| Gate | Command | Covers | Done Signal |
|---|---|---|---|
| Unit tests | `go test ./internal/parser ./internal/cli ./internal/store` from `library/commerce/amazon-orders` | U1-U6 | All targeted parser, CLI, sync, and store tests pass. |
| Full package tests | `go test ./...` from `library/commerce/amazon-orders` | U1-U7 | Entire Amazon Orders module passes. |
| Build | `go build ./cmd/amazon-orders-pp-cli` from `library/commerce/amazon-orders` | U7 | CLI binary builds without generated-tree compile regressions. |
| Doctor smoke | `go run ./cmd/amazon-orders-pp-cli doctor --agent --fail-on error` from `library/commerce/amazon-orders` | U6-U7 | Either reports valid authenticated content or fails with a clear auth/session reason; it must not claim credentials are valid solely from stale proof when live content is sign-in HTML. |
| Known order smoke | `go run ./cmd/amazon-orders-pp-cli orders get 702-5010515-8774615 --json --no-input --no-color --yes --data-source live --no-cache` from `library/commerce/amazon-orders` | U2-U4, U7 | Must not return an unrelated order ID. If auth is invalid, exits non-zero with remediation. If auth is valid, returns the requested order ID. |
| Search smoke | `go run ./cmd/amazon-orders-pp-cli find Aqara --json --no-input --no-color --yes --max-pages 1` from `library/commerce/amazon-orders` | U3, U7 | Must not return an empty success when the first order-history page is sign-in HTML. |
| Sync smoke | `go run ./cmd/amazon-orders-pp-cli sync --resources orders,transactions --since 90d --json --no-input --no-color --yes --strict` from `library/commerce/amazon-orders` | U3, U7 | Must not report success while storing sign-in pages. |
| Store inspection | Query the local SQLite `resources` table after sync smoke for `Amazon Sign-In`, `Amazon Iniciar sesion`, and the known order ID | U3, U5, U7 | No sign-in page rows remain as successful synced data; real rows use meaningful IDs when available. |

---

## Definition of Done

- Every requirement R1-R9 is satisfied or explicitly deferred with a non-blocking reason.
- The known user failure mode no longer produces an unrelated order, a false empty result, or a fresh cache containing sign-in HTML.
- `doctor` no longer gives a false-positive credential verdict when the live authenticated probe returns login/challenge content.
- Local order detail lookup uses the requested order identity.
- Tests cover English and Spanish Amazon sign-in pages, mismatched order IDs, sync/store guard behavior, and doctor live-content validation.
- `.printing-press-patches/authenticated-html-guard.json` exists and follows the repo's schema version 2 convention.
- `go test ./...` and `go build ./cmd/amazon-orders-pp-cli` pass from `library/commerce/amazon-orders`.
- Any experimental code, temporary fixtures not used by tests, and local debug output are removed before commit.
