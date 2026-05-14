# Novel Features Brainstorm — ghl-pp-cli

## Customer model

**Persona 1: Alex (gym owner / operator, i2 Fitness)**

- **Today (without this CLI):** Alex juggles three browser tabs into the GHL sub-account (Contacts, Conversations, Opportunities) plus the Riley n8n workflows tab plus the trainer-dashboard Supabase mirror. To answer "did we follow up on this week's leads?" he opens GHL contacts, filters by tag, clicks each contact to verify Riley actually sent the SMS, then cross-references the conversation thread. To check "is Riley still texting people who said stop?" he has no answer at all — he searches contact-by-contact for the `ai off` tag and prays it was applied correctly. He re-runs the same three GHL searches every weekday morning.
- **Weekly ritual:** Monday morning lead review: pull leads tagged in the last 7 days, verify each has either a Riley conversation in progress or a kill-switch tag, escalate anything that fell through. End-of-week pipeline review: walk opportunities by stage, flag stale ones, update value/stage on the few that moved.
- **Frustration:** Cannot see kill-switch tags (`ai off`, `human handover`) on a contact without clicking deep into the contact card, and cannot answer "how many contacts have `ai off` set" with a single query. The two tags that gate every Riley message are invisible at the speed he works.

**Persona 2: Riley (the AI agent, running inside i2-ai-nurture)**

- **Today (without this CLI):** Riley runs as a Claude-based SMS/voice nurture agent inside an n8n workflow. Today she calls the raw GHL REST API through n8n HTTP nodes — verbose JSON in, verbose JSON out, no terse defaults, every list response blows her context window. To check "should I text this contact?" she has to fetch the full contact payload and grep tags manually. To find a recent conversation she paginates raw API calls. She has no SQL surface; everything is one-shot REST.
- **Weekly ritual:** Per-trigger, hundreds of times a day: receive a webhook, read the contact, decide whether to send SMS (kill-switch check), send SMS, log outcome, sometimes trigger a workflow.
- **Frustration:** Token cost. Every contact-list and conversation-list call returns multi-KB JSON when she needs id + name + phone + kill-switch flag. Every kill-switch check is a substring search through a tags array because no tool surfaces it as a typed boolean.

**Persona 3: RJ (coach, trainer-dashboard user)**

- **Today (without this CLI):** RJ lives in the trainer-dashboard, which mirrors GHL contacts via Supabase. When the mirror is stale or missing a field he opens the GHL UI. To check "which of my clients haven't had a touchpoint in 14 days" he eyeballs the dashboard, which doesn't expose conversation timestamps. To add a quick note about a session he opens GHL contact card → notes tab → type → save.
- **Weekly ritual:** Friday client review — walk his roster, flag anyone he hasn't talked to in two weeks, drop a quick text or note.
- **Frustration:** "When did I last hear from this client?" requires opening the GHL conversation thread one client at a time. Conversation recency is the answer he needs and the dashboard can't give him.

## Survivors (11, all ≥ 7/10)

| # | Feature | Command | Score | How It Works | Persona | Evidence |
|---|---------|---------|-------|--------------|---------|----------|
| 1 | Kill-switch roster | `killswitch list [--tag ai-off\|human-handover]` | 10/10 | Local SQLite join of `contacts` × `tag_memberships` × latest `messages`; renders id, name, phone, which kill-switch tag, last-message ts | Alex, Riley | Brief User Vision; brief Table Stakes gap; Alex Persona frustration |
| 2 | Kill-switch check (typed exit) | `killswitch check <contact-id-or-phone>` | 10/10 | Single contact lookup (store-first, API fallback); typed exit codes 0/2/3 (clear/ai-off/handover) | Riley | Riley persona token cost; brief Riley-safety contract |
| 3 | Activity window | `activity --since 24h\|7d` | 10/10 | Union of `messages`, `contacts.created_at`, `opportunities.stage_changed_at`, `appointments.created_at`, ordered by ts desc | Alex | Brief Table Stakes gap; Top Workflow #8 |
| 4 | Tag analytics | `tags stats` | 10/10 | `SELECT tag, COUNT(contact_id), MAX(updated_at), kill_switch_flag GROUP BY tag` | Alex | Brief Table Stakes gap; Top Workflow #7 |
| 5 | Client recency | `contacts recency [--tag client] [--over 14d]` | 9/10 | `contacts LEFT JOIN messages` aggregating MAX(inbound_ts), MAX(outbound_ts), sort by oldest | RJ | RJ Persona Friday ritual |
| 6 | Stale opportunities | `opportunities stale --days 14` | 8/10 | `WHERE stage_changed_at < now - :days`, grouped by pipeline+stage | Alex | Alex end-of-week ritual |
| 7 | Workflow membership | `workflows members <wfid>` | 8/10 | Vendor endpoint + local cache; terse default | Alex | Brief Table Stakes gap (workflows not in official MCP) |
| 8 | SMS send preflight | `sms preflight <contact> --body "..."` | 9/10 | Local checks (E.164 phone, no `ai off`, business hours) + fresh contact fetch; typed exit codes; no send | Riley | Riley per-send ritual; brief kill-switch contract |
| 9 | Pipeline funnel | `opportunities funnel <pipeline-id>` | 7/10 | `SELECT stage, COUNT(*), SUM(monetary_value) GROUP BY stage` | Alex | Top Workflow #4 |
| 10 | Inbox triage | `inbox triage [--since 4h]` | 8/10 | Unread inbound with no outbound in window AND contact lacks `ai off`; one-line output | Alex, Riley | Alex daily ritual; brief one-line-default vision |
| 11 | Daily KPI ticker | `kpi today` | 10/10 | One-line: new contacts, SMS sent, appointments booked, opps moved, kill-switch trips, JSON-friendly | Alex | Brief User Vision: business-dashboard consumer |

## Killed candidates (audit trail)

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| Conversation digest | Overlaps with inbox triage; triage is more actionable (filters to unread + idle), digest's kill-switch flag folded into the triage output | Inbox triage (#10) |
| Phone reverse lookup | Thin transcendence over absorb row #5 (`search contact by phone/email`); kill-switch surfacing better served by #1/#2 | Kill-switch roster (#1) |
| Custom-field bulk set | Monthly use at best (soft kill) + scope creep (write-loop application shape); per-contact CF update already absorbed | Absorb row #29 |
| Calendar utilization | Subsumed by activity window once descoped; no-show + free-slot-ratio failed verifiability | Activity window (#3) |
| Workflow trigger with guard | Duplicate of absorb row #26 (workflow trigger w/ kill-switch guard) | Absorb row #26 |
