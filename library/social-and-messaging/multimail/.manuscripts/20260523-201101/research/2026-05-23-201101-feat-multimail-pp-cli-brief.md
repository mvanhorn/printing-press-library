# MultiMail CLI Brief

## API Identity
- Domain: Email-as-a-Service for AI agents
- Users: AI agents (Claude, GPT, Codex), developers building agent-email integrations, DevOps teams managing agent mailboxes, operators governing agent autonomy
- Data profile: Emails (inbound/outbound, markdown/HTML), mailboxes with oversight modes, contacts, API keys, audit logs, webhooks, billing, sending allowlists, agent registrations. Trust ladder progression (read_only → gated_all → gated_send → monitored → autonomous). Operator session management.

## Reachability Risk
- None. First-party API with documented OpenAPI spec (69 paths). Live at https://api.multimail.dev. Auth via Bearer token (X-API-Key header). Live spec served at /v1/openapi.json.

## Top Workflows
1. **Inbox triage** — Check inbox across mailboxes, read emails, reply with context. The #1 agent workflow.
2. **Send with oversight** — Compose and send email, may be gated by oversight mode. Allowlisted recipients bypass the gated_send queue automatically.
3. **Trust ladder progression** — Request upgrade from current oversight mode, apply approval code, check status.
4. **Allowlist management** — Add/remove/list sending allowlist patterns (exact emails or *@domain.com wildcards) to bypass gated_send for trusted recipients. Requires operator approval via email OTP.
5. **Agent self-registration** — Register for API credentials via the auth.md protocol (WorkOS spec): POST /agent/auth → receive OTP via operator email → POST /agent/auth/claim/complete with code → get API key.
6. **Mailbox management** — Create, configure, and manage mailboxes with different oversight modes per mailbox.
7. **Compliance audit** — Review audit log, check API key usage, monitor oversight decisions.

## Table Stakes
- List/read/send/reply emails
- Manage mailboxes (CRUD + configure oversight mode)
- Contact management (add/search/delete)
- Spam/suppression management
- API key management (create/list/update/delete)
- Webhook management (create/list/get/delete)
- Account status and billing
- Oversight approval queue (list pending, decide)
- Domain management (create/list/get/delete/verify)
- Operator sessions (start/verify/end)
- Export data
- Usage statistics
- Sending allowlist (list/add/remove with operator approval)
- Agent auth registration (register/claim/complete)
- OAuth discovery (well-known endpoints)

## Data Layer
- Primary entities: emails, mailboxes, contacts, api_keys, audit_events, webhooks, allowlist_entries
- Sync cursor: email.id (ULID, monotonically increasing), audit_event.id
- FTS/search: email subject + body + sender + recipient

## Product Thesis
- Name: `mm` (MultiMail CLI)
- Why it should exist: Agents in shell-first environments (Codex, CI/CD, agentic shells) need MultiMail access without MCP. A CLI with local SQLite cache enables compound queries impossible via the API alone — inbox health scores, stale thread detection, oversight velocity, trust ladder analytics, allowlist coverage analysis. The CLI is a second distribution channel that serves agents who operate in pipelines, not chat contexts. The auth.md registration flow gives agents a self-service credential path without pre-provisioned API keys.

## Build Priorities
1. Full API parity with all 69 OpenAPI endpoints plus the 44 MCP tools' grouped operations
2. New allowlist commands (list/add/remove with operator approval two-step flow)
3. New auth.md agent registration commands (register/view-claim/complete-claim)
4. New OAuth well-known discovery commands
5. Local SQLite data layer with FTS5 email search
6. Agent-native output (auto-JSON, --compact, typed exit codes)
7. Compound commands (inbox health, stale threads, trust status, oversight summary, quota forecast)
8. Incremental sync with cursor tracking
