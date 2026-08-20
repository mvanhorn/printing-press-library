# Superhuman CLI Brief

## API Identity
- Domain: premium email client (Mail), now also Docs + AI. Grammarly acquired Superhuman in July 2025 for ~$825M; Grammarly rebranded to "Superhuman" in October 2025.
- Users: power emailers, founders, executives, sales, agents (via the new Mail MCP).
- Data profile: email threads, messages, drafts, contacts, snippets, calendar events, labels, attachments, read statuses, AI-generated drafts.

## Reachability Risk
- **Medium-low for personal use.** No CloudFlare challenge on `mcp.mail.superhuman.com` (clean 401). `mail.superhuman.com` is a normal logged-in web app.
- **ToS risk if redistributing.** Superhuman has no public API and no documented developer agreement for the internal RPC. Personal-use CLI against the user's own logged-in session is fine; publishing to the public library means flagging this in the Phase 6 publish prompt.
- **MCP path is plan-gated.** The official MCP requires a Business or Enterprise plan. Wrapping the web-app RPC bypasses this gate.

## Source Surfaces (priority order)
1. **Web-app Backend Portal RPC** at `app.superhuman.com` / `mail.superhuman.com`. Primary source. Reverse-engineered by edwinhu/superhuman-cli. Discoverable via browser-sniff with the user's logged-in Chrome.
2. **Ask AI endpoint**: `POST /v3/ai.askAIProxy`. Secondary; gives semantic-search and AI-draft generation.
3. **Underlying Gmail API + Microsoft Graph** (via the OAuth tokens Superhuman caches in the page). Tertiary; used for attachment downloads, calendar.
4. **Official MCP** at `https://mcp.mail.superhuman.com/mcp`. Adapter only — 4 tools (`create_or_update_draft`, `create_or_update_event`, `list_threads`, `send_email`) gated to Business plan. Useful as a fallback transport for users who don't want CDP-based JWT extraction.

## Top Workflows
1. **Read inbox in CLI** — `superhuman inbox`, `superhuman read <thread>`. Listed first because it's the user's stated #1 goal.
2. **Draft + send/reply from CLI / Claude Code** — `superhuman draft create --to alice --subject ... --body-stdin`, `superhuman reply <thread> --body-file ...`, `superhuman send <draft>`. Stated #2 goal.
3. **Search across all email** — local FTS5 + remote Ask AI fallback. `superhuman search "invoice from acme"`.
4. **Triage at scale** — bulk archive/snooze/label based on filters. `superhuman archive --label promotions --older-than 7d`.
5. **Agent-native pipelines** — `superhuman inbox --json | jq ...`, `superhuman thread <id> --select subject,from,snippet --json`. The whole CLI is `--json`/`--select`/`--csv` capable.

## Table Stakes (must absorb)
- Every command in edwinhu/superhuman-cli: inbox, search, read, send, reply, reply-all, forward, draft CRUD, archive, delete, mark, star, snooze, contact search, snippet list/use, calendar CRUD, label CRUD, attachment list/download, AI (semantic search).
- Every tool in the official MCP: list_threads, create_or_update_draft, send_email, create_or_update_event.
- Standard email-CLI verbs from himalaya / aerc / mu: multi-account, JSON output, OAuth, structured search query language.

## Data Layer
- Primary entities: `threads`, `messages`, `drafts`, `contacts`, `snippets`, `labels`, `calendar_events`, `attachments`, `read_statuses`.
- Sync cursor: incremental via thread/message `updated_at` cursors. Initial sync caps at last 30 days by default with `--full` opt-in.
- FTS5: `messages_fts(subject, from, to, body)` for local fast search. Falls through to Ask AI for semantic queries.
- Read-status time-series: `read_status_events(message_id, event_type, ts)` — enables novel analytics.

## Codebase Intelligence
- **edwinhu/superhuman-cli** (TypeScript, 3 stars). Documents the four real backends Superhuman uses: Backend Portal RPC, AI proxy, Gmail+Graph passthrough, CDP-based JWT extraction. Auth caches to `~/.config/superhuman-cli/tokens.json` with auto-refresh. **Our Go CLI mirrors this pattern but in a single static binary with durable refresh.**
- **superhuman/mcp-mail** (JavaScript, official). Sparse README; tool list documented only on the help center. Tools: create_or_update_draft, create_or_update_event, list_threads, send_email + Superhuman-specific capabilities (Split Inbox triage, Read Statuses, Smart Send, Ask AI).
- Auth: web-app uses a short-lived JWT in localStorage / cookies; MCP uses OAuth PKCE.

## User Vision
> "I want to be able to pull in my emails / respond / draft emails from CLI / Claude Code type place."

> "I'm logged in on my Chrome AND I have the MCP. The MCP always logs me out which sucks a lot."

Vision = read/draft/respond from terminal and from Claude Code. Pain = MCP OAuth churn (every ~1h–24h). Solution = native Go CLI with durable refresh + local store + agent-native I/O.

## Product Thesis
- **Name:** `superhuman-pp-cli` (binary), shipped as `superhuman` in the library.
- **Why it should exist:**
  1. Bring the full Superhuman feature set to the terminal and to Claude Code with `--json` / `--select` / typed exit codes.
  2. **Solve the "MCP logs me out" problem** by managing the OAuth refresh lifecycle natively in Go.
  3. Unify inboxes across accounts — Superhuman's loudest missing feature in user reviews.
  4. Local FTS5 store enables offline search and agent-driven bulk triage that no Superhuman surface exposes.
  5. Free tier viable: bypass the Business-plan MCP gate by talking to the user's own logged-in web-app session.

## Build Priorities
1. **P0 data layer** — SQLite store for threads, messages, drafts, contacts, snippets, labels, calendar_events, attachments, read_statuses. FTS5 on messages. Sync command with cursor.
2. **P1 absorbed features** — full coverage of edwinhu/superhuman-cli command surface + the 4 MCP tools.
3. **P2 transcendence**:
   - `auth login --chrome` — extract JWT from logged-in Chrome via CDP, with durable refresh (fixes MCP logout pain).
   - `unified` — read across all linked Superhuman accounts in one stream.
   - `triage` — pattern-driven bulk archive/snooze/label/reply, agent-driven.
   - `latency` — reply-latency analytics (who you respond to quickly).
   - `digest` — daily/weekly inbox-zero report with what came in, what was triaged.
   - `snooze-coming-back` — see what's scheduled to return this week.
   - `snippet use --vars` — variable-templated snippets.

## Auth Strategy
- Primary: `auth login --chrome` attaches to the user's running Chrome (CDP on port 9222) and extracts the JWT from `mail.superhuman.com`. Persists to `~/.config/superhuman-pp-cli/tokens.json` and auto-refreshes.
- Fallback: `auth login --mcp` does the OAuth PKCE flow against `mcp.mail.superhuman.com/mcp`. Only useful for Business/Enterprise users who prefer it.
- Doctor: `superhuman doctor` checks token freshness and refreshes on demand.

## Reachability Probe Plan
Before generation, run `printing-press probe-reachability https://app.superhuman.com/api/v1/ping` (or whatever browser-sniff surfaces) to classify the transport mode. Expect `standard_http` or `browser_http` for a logged-in session. If `browser_clearance_http`, route through Surf + cookie import.

## Recommended Next Step
Approve the browser-sniff gate and capture the logged-in `mail.superhuman.com` session to enumerate the Backend Portal RPC. Use that capture to write an internal YAML spec. Generate from that spec. Hand-build the CDP auth flow and the durable refresh logic.
