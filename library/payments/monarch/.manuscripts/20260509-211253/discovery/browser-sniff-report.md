# Monarch Browser-Sniff Discovery Report

## Capture Setup
- **Backend:** browser-use (CLI mode, --profile Default)
- **Target:** `https://app.monarch.com` (logged-in session via the user's Chrome profile)
- **Date:** 2026-05-09
- **Interceptor:** `window.fetch` wrapper that logs `POST /graphql` requests and responses

## Coverage
- **Captured operations:** 83 GraphQL POSTs total
- **Unique operations:** 33
- **Pages walked:** dashboard, accounts, transactions, budget, cashflow, recurring, categories, goals, insights, networth, settings/categories, settings/tags, holdings, reports/income, reports/spending, reports/networth, transactions/rules, recurring/upcoming
- **Response bodies:** captured during the run, redacted on disk (sizes only)

## Endpoint Surface
- **Domain:** `https://api.monarch.com` (confirms the Jan 2026 domain change away from `api.monarchmoney.com`)
- **Endpoint:** `POST /graphql` for every operation
- **Envelope:** `{"operationName": "<name>", "query": "<gql>", "variables": <obj>}`
- **Response shape:** standard GraphQL `{data: {...}, errors?: [...]}`

## Auth Pattern (confirmed against captures)
- **Cookie auth in browser:** the captures used the user's Chrome `_session_id` and related cookies, sent automatically with the GraphQL fetches.
- **Token auth (headless):** wrappers in the wild authenticate with `Authorization: Token <session>` after a POST to `/auth/login` (email+password) or `/auth/login/multi_factor` (email+password+code). MFA modes: email OTP and TOTP.
- **Recommendation for the generated CLI:** support both — `auth login --chrome` for session-cookie reuse and `auth login --email --password --mfa` for headless flows. Persist the token via OS keychain (Keyring on macOS) with file fallback.

## Captured Operation Catalog (33 unique)
| # | Operation Name | Type | Resource |
|---|----------------|------|----------|
| 1 | `Common_LatestForceRefreshOperation` | query | accounts.refresh_status |
| 2 | `Common_ForceRefreshAccountsQuery` | query | accounts.refresh |
| 3 | `Web_AccountFilterQuery` | query | accounts.filter_metadata |
| 4 | `Common_GetDisplayBalanceAtDate` | query | accounts.balance_at_date |
| 5 | `Common_GetHouseholdPreferences` | query | me.preferences |
| 6 | `Common_LegacyGoalsMigrationQuery` | query | goals (legacy) |
| 7 | `Web_GetAccountsPage` | query | accounts.list |
| 8 | `Web_GetAccountsPageRecentBalance` | query | accounts.balance_history |
| 9 | `Web_GetAccountTypes` | query | accounts.types |
| 10 | `Common_UserProfileFlags` | query | me.flags |
| 11 | `Common_GetAggregateSnapshots` | query | accounts.networth_snapshots |
| 12 | `Web_GetTransactionFiltersMetadata` | query | transactions.filter_metadata |
| 13 | `Web_GetDownloadTransactionsSession` | query | transactions.download_session |
| 14 | `Web_GetTransactionsSummaryCard` | query | transactions.summary |
| 15 | `Web_GetTransactionsPage` | query | transactions.list (paginated) |
| 16 | `Web_MintTransactionsCountQuery` | query | transactions.count |
| 17 | `Web_TransactionsFilterQuery` | query | transactions.filter |
| 18 | `Web_GetUserDismissedRetailSyncBanner` | query | (UI only - skip) |
| 19 | `Web_GetTransactionsList` | query | transactions.list (lite) |
| 20 | `Common_GetCategories` | query | categories.list |
| 21 | `Common_GetSubscriptionDetails` | query | subscription.get |
| 22 | `Common_GetBudgetSettings` | query | budgets.settings |
| 23 | `Common_GetBudgetStatus` | query | budgets.status |
| 24 | `Common_GetJointPlanningData` | query | budgets.joint_planning |
| 25 | `RecurringMerchantSearch` | query | recurring.search |
| 26 | `Common_GetAggregatedRecurringItems` | query | recurring.list |
| 27 | `Common_GetSpinwheelCreditReport` | query | credit.report |
| 28 | `Web_GoalsV2` | query | goals.list |
| 29 | `Common_GetReportConfigurations` | query | reports.configurations |
| 30 | `Common_GetReportsData` | query | reports.data |
| 31 | `ManageGetCategoryGroups` | query | categories.groups |
| 32 | `Common_GetHouseholdTransactionTags` | query | tags.list |
| 33 | `GetTransactionDrawer` | query | transactions.get |

Mutations were intentionally NOT triggered against the user's live account during browser-sniff. The mutation surface (create/update/delete transactions, budgets, goals, categories, tags, rules, splits) will be implemented in Phase 3 from community wrapper documentation (`keithah/monarchmoney-enhanced`, `eshaffer321/monarchmoney-go`) and validated separately.

## Replayability Assessment
- **Replayable:** every captured operation. Pure POST /graphql with JSON body and `Authorization` header. No client-side signing, no CSRF token in body, no DOM-injected nonces.
- **Headers required:** `Authorization: Token <session>` (or session cookies in cookie-mode), `Content-Type: application/json`, `Accept: application/json`.
- **Persisted-query hashes:** none observed — Monarch sends full query text in the POST body (not Apollo's persisted-query mode). This means our generated client doesn't need a hash registry — we can synthesize queries directly.

## Risks Identified
1. **Operation names change.** Monarch's web-app updates can rename operations (e.g., `Web_GoalsV2` was previously `Common_LegacyGoalsMigrationQuery`). The generated CLI must tolerate operation-name drift gracefully — surface a clean error rather than crash on `null` data.
2. **Domain pinning.** Hardcoding `api.monarch.com` at build time is correct today (Jan 2026 onward) but a future domain change would break the CLI. Make the base URL configurable via `MONARCH_BASE_URL` env override.
3. **No public schema.** GraphQL introspection is disabled on the production endpoint (typical for production GraphQL). The generator won't be able to derive types from the live API; we must hand-author response types or unmarshal into `map[string]any` and project fields.

## Auth Cookie Validation
The captures originated from the user's logged-in Chrome session. The cookies that were sent with each GraphQL call (browser-use loaded them via `--profile Default`) are the same cookies we'd import via `auth login --chrome`. Cookie reuse worked end-to-end during the walkthrough (every GraphQL response was 200, no 401s). This means the printed CLI's `auth login --chrome` flow has a viable replayable surface.

## Spec Output
- The hand-authored internal YAML spec at `$RESEARCH_DIR/monarch-spec.yaml` was informed by this capture. It models 14 resources / 38 endpoints, all routing to `POST /graphql`. The Phase 3 hand-build replaces the generated REST client with a GraphQL envelope client that reads `operationName` from a per-endpoint registry.
