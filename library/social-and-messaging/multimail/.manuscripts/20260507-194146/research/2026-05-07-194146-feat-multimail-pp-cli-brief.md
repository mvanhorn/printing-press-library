# MultiMail CLI Brief

## API Identity
- Domain: Email-as-a-Service for AI agents
- Users: AI agents (Claude, GPT, Codex), developers building agent-email integrations, DevOps teams managing agent mailboxes
- Data profile: Emails (inbound/outbound), mailboxes, contacts, API keys, audit logs, webhooks, billing. Email bodies as markdown (inbound) / HTML (outbound). Trust ladder progression data. Oversight approval queue.

## Reachability Risk
- None. First-party API with documented OpenAPI spec (59KB, 61 paths). Live at https://api.multimail.dev. Auth via X-API-Key header.

## Top Workflows
1. **Inbox triage** — Check inbox, read emails, reply with context. The #1 agent workflow.
2. **Send with oversight** — Compose and send email, may be gated by oversight mode (drafts → gated → monitored → autonomous).
3. **Trust ladder progression** — Request upgrade from current oversight mode, apply approval code, check status.
4. **Mailbox management** — Create, configure, and manage mailboxes with different oversight modes per mailbox.
5. **Compliance audit** — Review audit log, check API key usage, monitor oversight decisions.

## Table Stakes
- List/read/send/reply emails
- Manage mailboxes (CRUD + configure)
- Contact management (add/search/delete)
- Spam/suppression management
- API key management
- Webhook management
- Account status and billing
- Oversight approval queue

## Data Layer
- Primary entities: emails, mailboxes, contacts, api_keys, audit_events, webhooks
- Sync cursor: email.id (ULID, monotonically increasing), audit_event.id
- FTS/search: email subject + body + sender + recipient

## Product Thesis
- Name: `mm` (MultiMail CLI)
- Why it should exist: Agents in shell-first environments (Codex, CI/CD, agentic shells) need MultiMail access without MCP. A CLI with local SQLite cache enables compound queries impossible via the API alone — inbox health scores, stale thread detection, oversight velocity, trust ladder analytics. The CLI is a second distribution channel that serves agents who operate in pipelines, not chat contexts.

## Build Priorities
1. Full API parity with 47 MCP tools (every tool = a CLI command)
2. Local SQLite data layer with FTS5 email search
3. Agent-native output (auto-JSON, --compact, typed exit codes)
4. Compound commands (inbox health, stale threads, trust status, oversight summary, quota forecast)
5. Incremental sync with cursor tracking
