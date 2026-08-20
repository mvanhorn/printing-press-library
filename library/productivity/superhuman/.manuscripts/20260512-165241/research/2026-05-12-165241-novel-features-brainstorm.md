# Superhuman Novel-Features Brainstorm

## Customer model

### Persona 1: Matt the Founder-Operator (the user)
**Today (without this CLI).** Matt lives in Claude Code and the terminal. He keeps Superhuman open in a Chrome tab because there's no Mac terminal client for it. To draft from a CLI today he either pastes into the web app or tries the official MCP, which silently logs him out roughly every day. When he's mid-flow in Claude Code and wants "respond to that VC thread," he has to context-switch to the browser, find the thread, click in, type or paste, and click Send.

**Weekly ritual.** Every morning he triages the inbox (Important pane, then Other). Once a week he batch-archives newsletters, snoozes anything that won't ship until Monday, and drafts founder updates. He bcc's his CRM, uses snippets for "intro" / "decline politely" / "VC update," and tracks who hasn't responded to his last ask.

**Frustration.** "MCP always logs me out which sucks a lot." He owns the Business plan, but the OAuth round-trip on every other Claude Code session breaks his flow. He wants a single `superhuman inbox` that just works from any shell, any Claude Code thread, and that he can pipe to `jq` and `claude`.

### Persona 2: Priya the Sales Lead
**Today.** She lives in Superhuman's web app: split inbox, snippets, Smart Send. She uses Read Statuses to see whether her last touch was opened. Her CRM sits in another tab. She copies subject + last reply timestamps into a spreadsheet to figure out who's gone cold.

**Weekly ritual.** Every Monday she reviews "people I emailed last week who never replied," sets reminders, and re-pings with a snippet. She tracks reply latency on her side (how fast she responds to inbound) because her VP measures it.

**Frustration.** Read Statuses are per-thread in the UI; there's no view that says "the 12 prospects from last week who opened but never replied." Reminders live on individual threads, not in a queryable list. Reply-latency analytics don't exist anywhere in Superhuman.

### Persona 3: Riley the Inbox-Zero Agent Operator
**Today.** Riley is an agent-native power user. They run Claude Code with custom skills that read email, summarize threads, draft replies, and propose triage actions. They cobble this together today with Gmail API + a homegrown wrapper, but they actually pay for Superhuman and want its Snooze, Split Inbox, and Smart Send semantics in their agent loop.

**Weekly ritual.** Nightly: agent runs over the day's inbox, drafts replies that wait in Drafts for human review the next morning, snoozes anything not actionable today, and applies labels. Weekly: agent produces a digest of what got archived, what's snoozed, and what's still unanswered after N days.

**Frustration.** The official MCP exposes 4 tools and is gated to the Business plan; the unofficial CLI exists but in TypeScript with no Claude Code-native MCP surface and no local store, so every agent step pays a fresh round-trip to the Backend Portal RPC.

## Candidates (pre-cut)

(See Survivors and kills table below for the cut judgment per candidate.)

| # | Name | Command | One-liner | Persona | Source | Verdict |
|---|------|---------|-----------|---------|--------|---------|
| C1 | Chrome-CDP durable login | `auth login --chrome` | Attach to logged-in Chrome on port 9222, lift Firebase JWT, persist + auto-refresh. | Matt, Riley | (e), (b) | KEEP |
| C2 | Unified cross-account inbox | `unified inbox` | One stream across accounts. | Matt | (a), (b) | KEEP |
| C3 | Awaiting-reply tracker | `awaiting --older-than 3d` | Threads where you sent last and recipient hasn't replied in N days. | Priya, Matt | (c) | KEEP |
| C4 | Read-status leaderboard | `opens --since 7d` | Who opened your emails, how many times, last open. | Priya | (b), (c) | KEEP |
| C5 | Reply-latency analytics | `latency --to me` | Median/p90 reply time inbound vs outbound. | Priya, Matt | (c) | KEEP |
| C6 | Pattern-driven triage | `triage --rule rules.yaml` | YAML rules to archive/snooze/label/reply-with-snippet. | Riley, Matt | (a), (b) | KEEP |
| C7 | Snooze-coming-back radar | `returning --within 7d` | Threads scheduled to un-snooze this week. | Priya, Matt | (b) | KEEP |
| C8 | Daily digest | `digest --today` | Single markdown report of inbox state. | Matt | (c) | KEEP |
| C9 | Snippets with variables | `snippet use intro --var name=Alice` | Templated snippets resolved locally. | Matt, Priya | brief P2 | KEEP |
| C10 | Smart-Send hold-and-release | `send --undo 30s` / `unsend` | Hold-and-release sender with abort. | Matt, Riley | (b), (a) | KEEP |
| C11 | Split-Inbox pane router | `inbox --pane important` | Honor Important/Other/VIP/Pinned panes. | Matt, Priya | (b) | KILL (folds into absorbed inbox filters) |
| C12 | Reminders inbox | `remind list` | First-class queryable reminders list. | Priya | (b) | KEEP |
| C13 | Contacts-top map | `contacts top --since 30d` | Top contacts by message count + reply rate. | Priya, Matt | (c) | KILL (reachable by piping C3+C4+C5) |
| C14 | Watch a thread | `watch <thread> --notify` | Poll-and-notify on a thread. | Priya | (a), (e) | KILL (belongs as --watch flag) |
| C15 | Thread brief (mechanical) | `thread brief <id>` | Mechanical thread summary. | Riley, Matt | (e) | KEEP |
| C16 | AI inbox-zero coach | `coach --week` | LLM-dependent advice. | Matt | speculation | KILL (LLM-dependency check) |
| C17 | Schedule send | `schedule send --at "Tue 8am"` | Wrapper around Send Later. | Matt | (b) | KILL (absorbed) |
| C18 | Auto-snooze if no reply | `auto-snooze --if-no-reply 2d` | Auto-snooze rule. | Priya, Matt | (a) | KILL (folded into C6) |

## Survivors and kills

### Survivors

| # | Feature | Command | Score | How It Works | Persona | Evidence |
|---|---------|---------|-------|-------------|---------|----------|
| 1 | Chrome-CDP durable login | `auth login --chrome` | 10/10 | Connects to user's running Chrome on CDP port 9222, extracts Firebase JWT from `mail.superhuman.com`, persists to `~/.config/superhuman-pp-cli/tokens.json`, refreshes via Firebase token endpoint on every command. | Matt, Riley | Verbatim user pain in brief User Vision ("MCP always logs me out"); edwinhu/superhuman-cli already proves the CDP path |
| 2 | Unified cross-account inbox | `unified inbox`, `unified search "term"` | 9/10 | Per-account SQLite stores are unioned in a single Go query; account derivable from token bundle; `--account` filter. | Matt | Brief Product Thesis #3: "Unify inboxes across accounts — Superhuman's loudest missing feature in user reviews"; multi-account is manifest #16 |
| 3 | Awaiting-reply tracker | `awaiting --older-than 3d --vip` | 9/10 | Local SQL: `messages WHERE from_me=1 GROUP BY thread_id HAVING max(ts)=last_message AND last_inbound_ts < last_outbound_ts AND age > N`; joins contacts for VIP. | Priya, Matt | Brief Read-Statuses identity; Priya weekly ritual; no Superhuman view exposes this |
| 4 | Read-status leaderboard | `opens --since 7d --by contact` | 8/10 | Aggregates `read_status_events` (data layer in brief) by message -> contact -> day; outputs `--csv` for sheets. | Priya | Brief data layer explicitly seeds `read_status_events`; Superhuman shows per-thread only |
| 5 | Reply-latency analytics | `latency --from me --since 30d` | 8/10 | SQL aggregation over `messages.received_at` deltas grouped by direction, contact, label; percentiles in-process. | Priya, Matt | Brief P2 lists `latency`; sales VP-measured metric absent from Superhuman |
| 6 | Pattern-driven bulk triage | `triage --rule rules.yaml --dry-run` | 9/10 | YAML DSL parses to predicates over local store; matched threads fan out to archive/snooze/label/reply endpoints; absorbs C18 auto-snooze as a rule kind. | Riley, Matt | Brief P2 lists `triage`; Inbox Zero is Superhuman content pattern |
| 7 | Snooze-coming-back radar | `returning --within 7d` | 7/10 | Local view over snooze metadata stored at snooze time; groups by return-day; includes original reason. | Matt | Brief P2 lists `snooze-coming-back`; Snooze is a Superhuman content pattern |
| 8 | Daily inbox-zero digest | `digest --today --markdown`, `digest --week` | 8/10 | Rolls up local store: arrived, archived, snoozed, drafted, awaiting, returning, opens; pipe-to-`claude` for prose. | Matt | Brief P2 lists `digest`; Inbox Zero identity; mechanical |
| 9 | Snippets with variables | `snippet use intro --var name=Alice` | 6/10 | Loads snippet body via absorbed `snippets.list`, replaces `{{var}}` placeholders locally with --var pairs + `{{today}}`/`{{from}}` autovars. | Matt, Priya | Brief P2 explicitly lists `snippet use --vars` |
| 10 | Smart-Send hold-and-release | `send --undo 30s <draft>`, `unsend <queue-id>` | 7/10 | Persists send intent to local queue table with fire-at timestamp; background goroutine fires `messages/send` on expiry; `unsend` deletes before fire. Honors PRINTING_PRESS_VERIFY short-circuit. | Matt, Riley | Smart Send is THE Superhuman content pattern; agents need safe abort window |
| 11 | Reminders inbox | `remind list`, `remind add <thread> in 3d "follow up"`, `remind clear <id>` | 6/10 | Wraps `/reminders/*` endpoints, indexes locally, exposes flat queryable list across all threads. | Priya | Reminders endpoint in API summary; Superhuman UI exposes only per-thread |
| 12 | Thread brief (mechanical) | `thread brief <id>` | 6/10 | Aggregates participants, message count, subject-rename history, attachments, labels, read-status totals from local store; `--json` for Claude Code piping. | Riley, Matt | Brief User Vision = Claude Code surface; rubric mechanical-reframe rule |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|--------------------------|
| C11 Split-Inbox pane router | Thin renaming of one filter — already covered by absorbed `inbox` filters | #6 triage rules can target panes as predicates |
| C13 Contacts-top map | Wrapper-ish; same data reachable by piping #3 + #4 + #5 outputs through `jq` | #4 opens leaderboard, #5 latency |
| C14 Watch a thread | Polling wrapper; belongs as generic `--watch` flag, not a novel command | #6 triage rules can fire on a watched predicate |
| C16 AI inbox-zero coach | LLM-dependency kill check; no mechanical version distinct from digest | #8 digest piped to `claude` |
| C17 Schedule send | Thin wrapper around Send Later; belongs in table-stakes send | Absorbed `send` with `--at` flag |
| C18 Auto-snooze if no reply | Folded into #6 triage rules as a rule kind | #6 triage rules |
