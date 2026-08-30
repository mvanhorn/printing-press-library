# AgentMail Absorb Manifest

## Surface inventory

The official OpenAPI 3.1 document contains 82 paths and 138 operations. The official CLI, SDKs, and hosted MCP expose the same core surface with additional agent-oriented output and onboarding conventions. The rows below group identical CRUD/search families while retaining every user-facing feature family that must be covered by generated commands.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---:|---|---|---|---|
| 1 | Agent sign-up with human email, username, returned key/inbox/org | Official quickstart/OpenAPI | `(generated endpoint) agent sign-up POST /v0/agent/sign-up` | JSON-safe onboarding and no secret persistence |
| 2 | Agent OTP verification | Official quickstart/OpenAPI | `(generated endpoint) agent verify POST /v0/agent/verify` | Explicit verification errors and dry-run |
| 3 | Inboxes list with pagination/order | Official CLI, MCP `list_inboxes` | `(generated endpoint) inboxes list GET /v0/inboxes` | Offline mirror, FTS/search, bounded output |
| 4 | Inbox get/create/update/delete | Official CLI, MCP inbox tools | `(generated endpoint) inboxes get/create/update/delete` | `client_id` safe retries and dry-run mutations |
| 5 | Inbox-scoped threads list/search/get | Official CLI, MCP thread tools | `(generated endpoint) inboxes threads list/search/get` | Exact filters plus API relevance search |
| 6 | Organization-wide threads list/search/get | Official Threads docs, MCP thread tools | `(generated endpoint) threads list/search/get` | Cross-inbox supervision and local history |
| 7 | Thread attachments/update/delete | OpenAPI, MCP thread tools | `(generated endpoint) threads attachments/update/delete` | Typed attachment metadata and safe output |
| 8 | Inbox-scoped messages list with filters | Official Messages docs | `(generated endpoint) inboxes messages list GET /v0/inboxes/{inbox_id}/messages` | Local FTS and extracted reply content |
| 9 | Message full-text search with highlights | Official Messages docs, MCP `search_messages` | `(generated endpoint) inboxes messages search` | API relevance plus offline search |
| 10 | Message get/update/delete | Official Messages docs, MCP `update_message` | `(generated endpoint) inboxes messages get/update/delete` | Label-aware JSON and typed errors |
| 11 | Batch get and batch update messages | OpenAPI | `(generated endpoint) inboxes messages batch-get/batch-update` | Deterministic batch output and dry-run |
| 12 | Message send | Official quickstart/CLI, MCP `send_message` | `(generated endpoint) inboxes messages send` | Idempotency-Key, dry-run, recipient validation |
| 13 | Reply, reply-all, forward | Official Messages docs, MCP reply/forward tools | `(generated endpoint) inboxes messages reply/reply-all/forward` | Conversation-safe mutations with idempotency |
| 14 | Raw message and attachments | OpenAPI, MCP `get_attachment` | `(generated endpoint) inboxes messages get-raw/get-attachment` | Stream/download-friendly output |
| 15 | Draft list/get/create/update/delete/send | Official Drafts docs, MCP draft tools | `(generated endpoint) inboxes drafts list/get/create/update/delete/send` | Human review before irreversible send |
| 16 | Draft reply/forward composition | Official Drafts docs | `(generated endpoint) inboxes drafts create --in-reply-to/--forward-of` | Explicit threading and review state |
| 17 | Scheduled draft sending and cancellation | Official Drafts docs | `(behavior in agentmail inboxes drafts create/update) scheduled `send_at` and cancellation | ISO time handling and clear state |
| 18 | Webhook CRUD and headers | OpenAPI, MCP webhook tools | `(generated endpoint) webhooks list/get/create/update/delete/get-headers/update-headers` | Configurable event delivery and dry-run |
| 19 | Webhook event types | Official webhook docs/MCP manifest | `(generated endpoint) webhooks events` | Typed event payloads for agent routing |
| 20 | Allow/block lists | OpenAPI, official docs | `(generated endpoint) lists list/get/create/delete` | Direction/type/entry filtering and local mirror |
| 21 | Metrics events and usage | OpenAPI, official docs | `(generated endpoint) metrics query-events/query-usage` | Persisted usage history and bounded queries |
| 22 | Inbox events | OpenAPI | `(generated endpoint) inboxes events list` | Local event timeline |
| 23 | Domains CRUD, zone file, verification | OpenAPI, official docs | `(generated endpoint) domains list/get/create/update/delete/zone-file/verify` | DNS setup visibility and typed status |
| 24 | Organization API keys | OpenAPI, MCP API-key tools | `(generated endpoint) api-keys list/create/delete` | Explicit secret redaction and dry-run |
| 25 | AgentID public-key credentials | OpenAPI, official docs | `(generated endpoint) api-keys public-keys list/create/update/revoke` | Public-key lifecycle coverage |
| 26 | Revoke all AgentID sign-in keys | OpenAPI | `(generated endpoint) api-keys public-keys agentid-sign-in revoke-all` | Explicit destructive-action guard |
| 27 | Pods CRUD | OpenAPI, official multi-tenancy docs | `(generated endpoint) pods list/get/create/delete` | Tenant-aware local storage |
| 28 | Pod-scoped inboxes/threads/drafts/webhooks/domains/lists/metrics/API keys | OpenAPI | `(generated endpoint) pods <resource> ...` | Preserve tenant scope instead of flattening data |
| 29 | Organization identity | OpenAPI | `(generated endpoint) organizations get` | Stable identity context |
| 30 | Who am I | OpenAPI, MCP auth tool | `(generated endpoint) auth me` | Read-only health/auth probe |
| 31 | Pagination and time/order filters | OpenAPI, SDK docs | `(behavior in agentmail sync/search) page tokens, ascending, before/after, bounded limits` | Resumable local sync and agent-sized output |
| 32 | API-key auth and canonical environment setup | Official CLI/SDK docs | `(behavior in agentmail doctor/auth) AGENTMAIL_API_KEY bearer auth` | Consistent auth diagnostics and no credential leakage |
| 33 | JSON/JSONL/YAML/pretty/raw output and transforms | Official CLI README | `(behavior in agentmail root output flags) --json/--agent/--select/--compact/--csv/--quiet` | Composable output and field selection |
| 34 | File arguments for text/binary attachment payloads | Official CLI README | `(behavior in agentmail command input) @file:// and @data:// handling where generated body supports it` | Scriptable attachment input |
| 35 | Rate-limit and retry guidance | Official rate-limit docs/SDKs | `(behavior in agentmail client) 429/Retry-After surfaced with bounded retries` | No silent empty results on throttle |
| 36 | Resource idempotency | Official idempotency docs | `(behavior in agentmail mutations) client_id on creates` | Safe retry semantics |
| 37 | Send idempotency | Official idempotency docs | `(behavior in agentmail send/reply/forward) Idempotency-Key on irreversible sends` | Prevent duplicate mail |
| 38 | Hosted MCP and stdio bridge parity | Official MCP README/manifest | `(behavior in agentmail mcp) generated typed endpoint tools plus code orchestration` | Thin remote-capable MCP surface with local CLI parity |
| 39 | Python/Node/Go SDK feature parity | Official SDK repositories/docs | `(behavior in agentmail generated client) typed HTTP commands matching SDK methods` | One installable binary and offline store |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---:|---|---|---:|---|---|---|---|
| 1 | Triage queue | `agentmail triage queue --db <path> [--inbox <id>] [--since 7d]` | 10/10 | hand-code | Joins synced messages, threads, inboxes, labels, drafts, and sync freshness to rank inbound conversations with no later outbound response. | Brief inbound-triage workflow; official thread/message/draft docs; local data-layer requirement. | Use this command for action-oriented unresolved inbound work. Do NOT use this command for a context-only conversation rollup; use `agentmail thread rollup` instead. |
| 2 | Send check | `agentmail send check <draft-id> [--db <path>] [--live]` | 10/10 | hand-code | Combines draft/attachment records with local send fingerprints and idempotency evidence to report deterministic pre-send risks. | User request for explicit read/write testing; official idempotency and draft-review docs. | Use this command for pre-send risk review of a draft. Do NOT use this command to diagnose the result of a message already sent; use `agentmail delivery reconcile` instead. |
| 3 | Thread rollup | `agentmail thread rollup --db <path> <thread-id>...` | 9/10 | hand-code | Drains local thread/message/draft records and computes participants, counts, latest direction, age, labels, extracted reply content, and next-state indicators. | Official thread/message/draft model and extracted reply-content guidance. | Use this command for context and handoff on known conversations. Do NOT use this command to produce an action-ranked unresolved inbound queue; use `agentmail triage queue` instead. |
| 4 | Fleet health | `agentmail fleet health --db <path> [--org <id>]` | 9/10 | hand-code | Joins inbox, domain, webhook, list, metrics, API-key, pod, and organization records into readiness findings. | Official multi-tenancy, domains, webhooks, metrics, and API-key surfaces; fleet-supervisor persona. | none |
| 5 | Schedule audit | `agentmail schedule audit --db <path> [--due-within 24h]` | 9/10 | hand-code | Scans local scheduled drafts and joins inbox/thread/send state to find overdue, orphaned, duplicated, or unreviewed sends. | Official draft scheduling and human-in-the-loop docs; idempotency guidance. | none |
| 6 | Delivery reconcile | `agentmail delivery reconcile --db <path> [--since 7d] [--inbox <id>]` | 8/10 | hand-code | Correlates outbound messages, status/timestamps, thread placement, sync cursors, and later inbound activity to expose post-send anomalies. | Official message/thread model; brief reliable-automation workflow. | Use this command for post-send outcome reconciliation. Do NOT use this command to approve a draft before an irreversible send; use `agentmail send check` instead. |

## Cut candidates

- `agentmail followups`: folded into `triage queue`; same latest-inbound/latest-outbound predicate.
- `agentmail find local`: local FTS is required infrastructure but a generic search mirror is absorbed framework behavior, not a distinct differentiator.
- `agentmail inbox onboard`: multi-write orchestration and rollback complexity exceed the weekly-value evidence; use generated commands and recipes.
- `agentmail webhook readiness`: folded into `fleet health`.
- `agentmail org cost-pulse`: the API exposes usage, not pricing/cost attribution; avoiding invented billing semantics.
- `agentmail send ledger`: its evidence is consumed by `send check` and `delivery reconcile` without a separate user-facing decision.
- `agentmail attachment bundle`: a thin wrapper over absorbed attachment endpoints.

## Implementation commitments

- Every generated endpoint remains reachable through typed CLI commands and MCP code orchestration.
- Every hand-coded transcendence command is non-interactive, read-only unless explicitly documented, dry-run safe, JSON/agent capable, bounded, and backed by real local data.
- No LLM summarization, external enrichment API, resident browser, or fake API response is required.
- Write-side dogfood requires disposable inbox/draft data and explicit cleanup; live verification must never send to an unintended recipient.
