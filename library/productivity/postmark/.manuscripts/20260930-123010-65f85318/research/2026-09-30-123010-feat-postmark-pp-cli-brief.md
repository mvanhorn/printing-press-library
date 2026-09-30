<!-- slop-gate: off -->
# Postmark CLI Brief

Supporting research (same dir): `postmark-docs-inventory.md` (87 documented ops, drift, list shapes, mutation table), `postmark-ecosystem.md` (29 tools, feature catalogs, pains), `live-auth-probes.md` (read-only probes against the user's account), `specs/` (official Swagger 2.0 server + account specs).

## API Identity
- Domain: transactional and broadcast email delivery (ActiveCampaign-owned). Sending (single, batch up to 500, template, bulk), outbound/inbound message activity, bounces, suppressions per message stream, templates and layouts, stats time series, webhooks, inbound rules, server config. Account layer: servers, sending domains (DKIM/SPF/Return-Path), sender signatures, cross-server template push, data removals.
- Users:
  - SaaS developers wiring password resets, receipts, OTP codes, and notifications into apps; they debug "the user says they never got the email".
  - Support and ops staff who look up a specific recipient's delivery history, reactivate a bounced address, or clear a suppression so a customer can log in.
  - Founders and small teams running several products on one Postmark account. The user's own account has one account token and 6 servers (Main App, Staging, Billing, Marketing Site, Docs, Support), each a separate product.
  - Teams keeping templates in git and promoting them staging to production (official CLI's core use; eSolia backup Action; postmark-template-trans).
  - AI agents sending on a user's behalf (official MCP, pmx, agent-postmark), where double-sends and unsafe writes are the known failure (postmark-mcp #54).
- Data profile: high-volume, time-ordered event data (outbound messages, opens, clicks, bounces, inbound) with a hard 45-day default retention (7-365 configurable) and a 10,000 count+offset deep-paging cap; low-volume config data (servers, streams, templates up to 100/server, domains, signatures, webhooks, inbound rules); permanent daily stats time series (EST); account-level config spanning servers.

## Reachability Risk
- Low. Official, documented REST API at `https://api.postmarkapp.com`. Live read-only probes on 2026-09-30 returned 200 for account (`GET /servers`) and server (`GET /server`, `GET /messages/outbound`) tokens.
- 429 is documented without a numeric limit; neither official client retries (postmark-mcp source, postmark.js #129). The CLI needs backoff on 429.
- Auth trap (live-verified): sending both `X-Postmark-Server-Token` and `X-Postmark-Account-Token` on `GET /server` returns 401 ErrorCode 10, while `GET /servers` and `GET /messages/outbound` tolerate both. The CLI must choose the header per endpoint family, not send both globally.
- Bulk email (`/email/bulk`) requires account approval (else 422 ErrorCode 14). Data removals are "available by request only".

## Top Workflows
1. **Recipient delivery triage ("did X get it?")**: search outbound by recipient over a time window, open the event timeline, check bounce history and stream suppressions, then reactivate or unsuppress. Evidence: MCP `diagnoseDelivery`, pmx `otp`, agent-postmark `investigate delivery`, Postmark support article 1267.
2. **Templates as code across servers**: pull to disk, diff, validate/render with a test model, push to another server, back up in CI. Evidence: official CLI pull/push/preview, CLI issues #92 (300-template cap), #87 (filtered pull), #75 (no delete), #102 ("going to move off of postmark templates"), Postmark's 2018 cross-environment blog.
3. **Bounce and suppression hygiene**: list inactive addresses by type and domain, bulk reactivate eligible hard bounces, delete or add suppressions (max 50 per call), resend what was blocked. Evidence: dszp retry script, arnaud-coral removal script, MCP activate/delete tools.
4. **Scripted and agent sending**: raw, template, batch (500), bulk, with stream selection and dry-run. Evidence: CLI `email raw|template`, CLI #56 (no stream choice), #88 (batch from CSV), MCP #42 (bulk), MCP #54 (agent double-send).
5. **Deliverability and domain health**: bounce/spam rates against thresholds, DKIM/SPF/Return-Path status per domain, webhook health, per-server and per-stream stats. Evidence: MCP `getDeliveryStats`, pmx `overview`, agent-postmark `domain-health`/`stream-health`/`webhooks health`.
6. **Multi-server oversight on one account**: which of my servers is bouncing, which has unverified domains, where did sends drop. Evidence: the user's own 6-server account; agent-postmark and pmx are the only tools with named server profiles; official CLI requires swapping env vars per run.

## Table Stakes
- Every documented operation as a typed command (87 ops: server spec 43, account spec 23, plus 21 doc-only: message streams 6, webhooks 7, suppressions 3, bulk 3, data removals 2), including `/stats/outbound/opens/readtimes` which the SDK uses.
- Send commands: raw, template, batch, batch-with-templates, bulk, with MessageStream, Metadata, Tag, Attachments, Headers, TrackOpens/TrackLinks; the `POSTMARK_API_TEST` sandbox token for validation-only sends.
- Template pull/push/preview parity with the official CLI, minus its 300 cap, plus single-template pull, delete, and cross-server push via `/templates/push`.
- Recipient diagnosis composite (MCP `diagnoseDelivery` parity).
- Server profiles: select a server by name (agent-postmark, pmx parity).
- Agent safety: `--yes`/confirm on writes, `--dry-run`, typed exit codes including rate-limit (pmx exit 5), structured errors with fixability hints (agent-postmark `fixable_by`), token redaction in output.
- Retry with backoff on 429 and 5xx for reads; idempotency guard for sends.

## Data Layer
- Primary entities: outbound_messages (+ details/events), inbound_messages, bounces, opens, clicks, templates (with content hash and version history), servers, message_streams, suppressions, webhooks, inbound_rules, domains, sender_signatures, stats_daily snapshots.
- Sync cursor: messages, bounces, opens, clicks sync by `fromdate`/`todate` windows (day windows, split further when a window exceeds 10,000) instead of pure offset, per the Airbyte fix (PR 87518). Config entities do full refresh. Stats sync by date range and are permanent upstream.
- Retention angle: local store keeps messages, events, and bounces past Postmark's 45-day expiry, which no existing tool does.
- FTS/search: subject, recipient, from, tag, metadata values, template name/alias/subject/body, bounce description.
- Multi-server: every synced row carries `server_id`, so cross-server queries work from one store.

## Codebase Intelligence
- Source: postmark.js `ServerClient.ts` / `AccountClient.ts`, postmark-mcp `index.js`, postmark-cli `src/commands` (read from source in `postmark-ecosystem.md`).
- Auth: `X-Postmark-Server-Token` (server scope) and `X-Postmark-Account-Token` (account scope). De facto env vars `POSTMARK_SERVER_TOKEN` and `POSTMARK_ACCOUNT_TOKEN`. Account token returns every server's `ApiTokens[]` via `GET /servers`, so one account credential can resolve per-server tokens by server name (live-verified).
- Data model: account > servers > message streams (<=10 per server, 1 inbound) > messages/bounces/suppressions; templates and webhooks are per server; domains and sender signatures are per account.
- Rate limiting: 429 only, no published number, no rate-limit headers.
- Architecture: suppressions live per stream; SpamComplaint suppressions cannot be deleted via API; per-message opens store only the first open; bounce dumps kept 30 days; batch sends return HTTP 200 with per-message ErrorCode.

## User Vision
- "Let's create a plan for the best agent CLI and what novel commands we can create." Plan first; the absorb manifest is the plan.
- The user runs one Postmark account with 6 product servers and wants both the account API and a server API (Main App) exercised in testing. Tokens are stored in a password manager (`a password-manager entry holding the account and server tokens`).

## Product Thesis
- Name: postmark-pp-cli (display name "Postmark").
- Why it should exist: the official CLI is six commands and under-maintained (issue "Is this project dead?"); the official MCP is server-token only, one server per process, Markdown output, no retry, no pagination past 100 templates. Nothing keeps data past 45 days, nothing sees across servers, and no tool protects agents from double-sending. This CLI covers all 87 operations, runs one account across all servers by name, archives activity locally past retention, and makes sending safe for agents.

## Build Priorities
1. Spec preparation: merge server + account specs, add the 21 doc-only ops and drift fixes from the inventory, replace per-operation token header params with two apiKey security schemes, and make the client pick the header per endpoint family (live 401 when both are sent to `GET /server`).
2. Server profiles: account token plus `--server <name>` resolving the server token through `GET /servers`; env vars `POSTMARK_SERVER_TOKEN` / `POSTMARK_ACCOUNT_TOKEN` as the base.
3. Local store and windowed sync for messages, bounces, events, templates, config, and stats across all servers.
4. Recipient triage, template workflows, bounce/suppression hygiene, and safe sending as the transcendence layer.
5. Agent contract: dry-run and sandbox-token sends, write confirmation, typed exit codes, 429 backoff, token redaction.
