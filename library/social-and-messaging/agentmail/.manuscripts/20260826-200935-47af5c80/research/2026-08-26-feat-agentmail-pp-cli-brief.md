# AgentMail CLI Brief

## API Identity
- Domain: AgentMail REST API at `https://api.agentmail.to/v0/`; official OpenAPI 3.1 and AsyncAPI 2.6 documents are published from the API-reference site.
- Users: AI agents that need durable email identities; support and operations agents handling inbound requests; outreach agents sending controlled mail; supervisors managing multiple inboxes and tenants.
- Data profile: 82 paths and 138 HTTP operations across agent onboarding, inboxes, messages, threads, drafts, webhooks, allow/block lists, metrics, domains, API keys, pods, organizations, and identity.
- Auth: `Authorization: Bearer <token>`; canonical environment variable `AGENTMAIL_API_KEY`.

## Reachability Risk
- Low. Authenticated `GET /v0/auth/me` returned HTTP 200 with the supplied credential. Credential value is not recorded in this brief or any artifact.
- Rate limits are per API key and return HTTP 429 with `Retry-After`; plan limits vary by subscription.

## Top Workflows
1. Provision an inbox, then send a first message with explicit recipients, HTML/text content, labels, headers, and attachments.
2. Receive and triage inbound mail using inbox- or organization-wide message/thread listing, substring filters, full-text relevance search, highlights, and extracted reply content.
3. Maintain human-in-the-loop conversations by creating, reviewing, updating, scheduling, forwarding, replying to, and safely sending drafts.
4. Operate a fleet of agent inboxes with domain configuration, webhooks, allow/block lists, usage/event metrics, API-key management, pods, and organization identity.
5. Run reliable automation with pagination, `Retry-After` backoff, client IDs for resource creation, and `Idempotency-Key` for irreversible sends.

## Table Stakes
- Official CLI supports resource commands, JSON/JSONL/YAML/pretty/raw formats, transformations, file arguments, custom base URLs, debug logging, and `AGENTMAIL_API_KEY`.
- Official MCP surface exposes inbox, message, thread, draft, attachment, verification, and organization-selection tools; hosted Streamable HTTP endpoint is `https://mcp.agentmail.to/mcp`, with npm/Python stdio bridges.
- SDKs exist for Python, Node/TypeScript, and Go; Python and Node docs cover async usage, retries/timeouts, pagination, raw responses, and WebSockets.
- CRUD and search coverage for inboxes, threads, messages, drafts, webhooks, domains, lists, API keys, and pods; organization-wide thread/message workflows are essential for supervisors.
- Safe sending primitives: draft review, scheduled `send_at`, `client_id` creation idempotency, `Idempotency-Key` send idempotency, dry-run support in this CLI, and clear 400/403/404/409 handling.

## Data Layer
- Primary entities: inboxes, messages, threads, drafts, attachments, labels, webhooks, webhook headers/events, domains, allow/block-list entries, metrics events/usage, API keys, pods, organizations, and authenticated identity.
- Persist normalized resource envelopes plus raw JSON so generated typed commands and novel local queries can coexist. Preserve message/thread IDs, mailbox scope, sender/recipient arrays, subject, text/html/extracted content, labels, timestamps, status, and pagination cursors.
- Sync cursor: per-resource `page_token` plus last successful sync timestamp; preserve API pagination and `ascending`/time filters.
- FTS/search: local FTS over message sender/recipients/subject/body/extracted content and thread subject/participants, while retaining API relevance-ranked search for live queries.
- Cache freshness: leave generator-owned pre-read cache disabled; AgentMail is quota-metered and sends are irreversible. Explicit `sync` is safer than surprising every read with an upstream refresh.

## Codebase Intelligence
- Source: official `agentmail-to/agentmail-cli`, `agentmail-to/agentmail-mcp`, `agentmail-to/agentmail-python`, and `agentmail-to/agentmail-node` repositories.
- Auth: bearer token in `Authorization`; `AGENTMAIL_API_KEY` is the dominant SDK/CLI convention.
- Data model: inboxes own messages, threads, drafts, and webhooks; organization-wide endpoints aggregate across inboxes; pods add tenant-scoped copies of those resources.
- Rate limiting: HTTP 429 with `Retry-After`; exponential backoff is documented. Creation uses `client_id`; sends/replies/forwards/draft sends use `Idempotency-Key` and keys expire after 24 hours.
- Architecture: hosted MCP is the canonical implementation; npm/Python bridges proxy the hosted server and discover its current tool catalog rather than embedding a second API implementation.

## User Vision
- Build a complete AgentMail CLI, exercise both read and write paths using the supplied credential, review the result thoroughly, then promote and publish it. Write operations must use disposable/test data and remain explicit and auditable.

## Product Thesis
- Name: AgentMail CLI
- Why it should exist: It should match AgentMail's broad API and MCP surface while giving agents one predictable Go binary with typed commands, local SQLite history, FTS, safe dry-runs, idempotency-aware mutations, bounded output, and composable JSON. The differentiator is durable local operational memory: agents can answer follow-up, triage, and fleet questions that a single remote API call cannot.

## Build Priorities
1. Generate the complete official OpenAPI surface with canonical bearer auth, explicit MCP code orchestration for the large endpoint set, and learn vocabulary for inbox/thread/message/draft/domain concepts.
2. Preserve all official CLI/MCP/SDK workflows: inbox provisioning, message and thread search, attachments/raw mail, replies/forwards, draft scheduling, webhooks, domains, lists, metrics, API keys, pods, and org identity.
3. Add local-first agent workflows: inbox triage, follow-up detection, conversation digest, fleet health, and send-safety review, each with JSON/agent output and honest empty-state behavior.
4. Verify every command path, dry-run, JSON shape, error path, live read, and approved disposable write lifecycle before promotion.

## Sources
- https://docs.agentmail.to/llms.txt
- https://docs.agentmail.to/openapi.yaml
- https://docs.agentmail.to/quickstart.md
- https://docs.agentmail.to/messages.md
- https://docs.agentmail.to/threads.md
- https://docs.agentmail.to/drafts.md
- https://docs.agentmail.to/knowledge-base/rate-limits.md
- https://docs.agentmail.to/idempotency.md
- https://raw.githubusercontent.com/agentmail-to/agentmail-cli/main/README.md
- https://raw.githubusercontent.com/agentmail-to/agentmail-mcp/main/README.md
- https://raw.githubusercontent.com/agentmail-to/agentmail-mcp/main/mcp-manifest.json
