# Twilio CLI Brief

## API Identity
- Domain: Twilio Core REST API v2010 (`api.twilio.com/2010-04-01/...`)
- Spec source: https://raw.githubusercontent.com/twilio/twilio-oai/main/spec/json/twilio_api_v2010.json (catalog-verified 2026-03-23)
- Users: developers shipping SMS/MMS/Voice features; ops teams reconciling usage and call/message logs; agencies running per-customer subaccounts
- Data profile: high-volume time-series (Messages, Calls, UsageRecords) plus low-volume reference (PhoneNumbers, Subaccounts, Conferences); Twilio's pagination is page-token based and the master tools page slowly when 10k+ records

## Reachability Risk
- **Low** — `api.twilio.com` is reachable to programmatic clients in 2026. No Cloudflare/WAF/geo blocks reported in twilio-cli, twilio-go, or twilio-python issue trackers. Standard rate limit is ~100 req/s per account, raisable. Live testing skipped this run because user-provided creds 401'd; no infrastructure block evidence.

## Top Workflows
1. **Send SMS / WhatsApp** — `messages:create`. The single most-invoked endpoint.
2. **Pull message/call logs by date range** — `messages:list` / `calls:list` with `DateSent`/`StartTime` filters; powers reconciliation, compliance audit, debugging delivery failures.
3. **Search & buy phone numbers** — `available-phone-numbers/{Country}/Local|Mobile|TollFree:list` → `incoming-phone-numbers:create`. Many filters (area code, capability, locality).
4. **Download call recordings** — `recordings:list` with `CallSid` or date range, fetch media URL.
5. **Usage / cost reporting** — `usage/records/{daily|monthly|yearly|today}` per category. Painful in the official CLI: must call per category and assemble the totals client-side.
6. **Subaccount management** — agencies create per-customer subaccounts, configure their auth tokens, list usage per subaccount.
7. **Configure phone-number webhooks** — point Voice/SMS URL at the right endpoint after deploys; `incoming-phone-numbers:update`.

## Table Stakes (must match competing tools)
- Every CRUD operation in v2010 OpenAPI exposed as a typed command (Messages, Calls, Recordings, Conferences, IncomingPhoneNumbers, AvailablePhoneNumbers, Subaccounts, UsageRecords, OutgoingCallerIds, Applications, Queues, Tokens, Tokens, Notifications, Transcriptions, AddressV2010, ValidationRequest, ConnectApps, AuthorizedConnectApps, Keys, NewKeys, NewSigningKey, SigningKey).
- HTTP Basic auth with **either** Account SID + Auth Token **or** API Key SID + Secret.
- `--json`, `--csv`, `--select` for every read.
- `--dry-run` for every mutation.
- Pagination support (Twilio uses `Page`/`PageSize`/`PageToken` patterns).
- Subaccount support — pass `--account-sid` / `TWILIO_ACCOUNT_SID` to scope calls under a subaccount.

## Data Layer
- Primary entities (highest gravity, in rank order):
  1. **Messages** — high volume, date-range queries, full-text on `Body`, status filters
  2. **Calls** — paired with Recordings, status/duration analytics
  3. **UsageRecords** — billing reconciliation, offline aggregation across categories
  4. **Recordings** — paired with Calls, media URL
  5. **IncomingPhoneNumbers** — webhook-config lookups
- Sync cursor: `DateUpdated` (Twilio supports `DateUpdated>=` filters on most resources, plus `DateSent`/`StartTime` for time-series specifically)
- FTS/search: `Body` (Messages), `FriendlyName` (most resources), `From`/`To` (Messages, Calls), `PhoneNumber` (PhoneNumbers, Recordings via Call lookup)

## Codebase Intelligence
- Source: GitHub (twilio/twilio-cli, twilio-labs/mcp, twilio/twilio-go, twilio/twilio-python).
- Auth: HTTP Basic. Username = AccountSid (AC...) **or** ApiKeySid (SK...). Password = AuthToken **or** ApiKeySecret. Header: `Authorization: Basic <base64>`.
- Data model: every v2010 resource is account-scoped: `/2010-04-01/Accounts/{AccountSid}/Messages.json`. Subaccount calls swap the AccountSid in the path; the credential's owning account must have parent rights.
- Rate limiting: ~100 req/s per account by default; 429 with `Retry-After` header. Concurrency limits per resource (e.g., 1 outgoing call per number/sec by default).
- Architecture: pure REST, no GraphQL, page-token pagination, .json suffix on every resource. Twilio's "edge" routing (`api.<edge>.twilio.com`) is host-override only — no credential change needed.

## Auth nuances worth surfacing in `doctor`
- Standard API Keys can't manage Accounts/Subaccounts or other API Keys → detect + warn.
- API Keys are bound to the account that minted them → calling subaccount paths with parent-account-only key returns 401 → detect + warn.
- Auth Token has primary/secondary slots — signature validation breaks if app uses secondary while primary was rotated.
- OAuth 2.0 exists for Twilio's Organizations / Public APIs but **not for v2010** Messages/Calls/Recordings — do not promise it.

## User Vision
- Skipped: user selected "Let's go" without volunteering vision text.

## Product Thesis
- Name: `twilio-pp-cli`
- Headline: **Every Twilio Core feature, plus offline message/call/usage history and SQL-grade analytics no other Twilio tool has.**
- Why this should exist:
  1. The official `twilio-cli` is a thin Node wrapper with 1–3s cold-start and **no local state** — every reconcile or audit query hits the API.
  2. Steampipe's Twilio plugin gives SQL but requires a Postgres FDW + Steampipe daemon — heavy to set up, not agent-native.
  3. Twilio MCPs auto-mirror the spec but burn agent context on every tool call (no local store, no FTS, no aggregation).
  4. A single static binary that syncs Messages/Calls/UsageRecords/Recordings into SQLite + offers FTS + raw `--sql` over the local store + per-category usage aggregation is genuinely missing from the ecosystem.

## Build Priorities
1. Generated v2010 surface (every resource as typed Cobra commands with `--json`/`--csv`/`--select`/`--dry-run`).
2. Local SQLite store + sync for Messages, Calls, UsageRecords, Recordings, IncomingPhoneNumbers.
3. FTS over Messages.Body + From/To, Calls From/To, IncomingPhoneNumbers FriendlyName.
4. `doctor` with auth-mode detection, scoped-key detection, parent/subaccount mismatch detection.
5. Transcendence commands listed in §1.5 manifest (cost-by-category, message-flow analytics, idle-number finder, recording-pair view, etc.).
