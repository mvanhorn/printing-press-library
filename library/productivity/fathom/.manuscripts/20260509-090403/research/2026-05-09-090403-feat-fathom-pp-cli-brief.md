# Fathom CLI Brief

## API Identity
- Domain: AI meeting intelligence — records Zoom/Meet/Teams calls, generates transcripts, summaries, action items
- Users: GTM teams, sales, engineers who rely on meeting context and need offline/agent-accessible meeting data
- Data profile: meetings (metadata + transcript + summary + action items + CRM matches), teams, team members, webhooks

## Reachability Risk
- Low. Official REST API at `https://api.fathom.ai/external/v1/` returns 200 for authenticated requests. API key auth works with `X-Api-Key` header. Rate limit: 60 req/min.

## Top Workflows
1. **Meeting sync + search** — Pull all meetings from Fathom into local SQLite, then search transcripts offline without burning API quota
2. **Action item audit** — Extract all action items across a date range, see what's assigned to whom, what's still open
3. **Person timeline** — See every meeting with a specific participant: topics, action items, talk ratio over time
4. **Morning digest** — Aggregate view of last 24-48h meetings: titles, summaries, action items in one output
5. **CRM sync intelligence** — Which deals appeared in meetings, total amounts, contact frequency

## Table Stakes (from MCP servers)
- List meetings with date range filters
- Get full transcript with speaker attribution
- Get AI summary (markdown)
- Get action items (with assignee, completion status, playback URL)
- List teams and team members
- Create/delete webhooks
- Filter by participant email, domain, team, recorder

## Data Layer
- Primary entities: meetings (with transcript, summary, action items, CRM matches, calendar invitees), teams, team members
- Sync cursor: cursor-based pagination on `/meetings`; store last sync cursor + created_after
- FTS/search: FTS5 on meeting transcripts and summaries for offline full-text search

## Codebase Intelligence
- Source: MCP servers (Dot-Fun/fathom-mcp, lukas-bekr/fathom-mcp)
- Auth: `X-Api-Key` header; env var `FATHOM_PP_CLI_API_KEY`
- Data model: meetings → recordings (transcript, summary); meetings → action_items; meetings → calendar_invitees; meetings → crm_matches
- Rate limiting: 60 req/min, X-RateLimit headers on 429, exponential backoff
- Architecture: cursor-based pagination, include flags to expand data inline (include_transcript, include_summary, include_action_items, include_crm_matches)

## Product Thesis
- Name: fathom-pp-cli
- Why it should exist: Every MCP server for Fathom loads transcripts token-by-token on demand, re-fetching the same data repeatedly. This CLI syncs all meeting data to local SQLite once, then enables sub-second offline search, cross-meeting aggregations, person timelines, deal intelligence, and meeting digest — capabilities that don't exist anywhere else.

## Build Priorities
1. Sync all meetings to SQLite (with transcript, summary, action items) — foundation for everything
2. Full-text search across synced transcripts (offline, no quota)
3. Action items list/audit across date ranges
4. Person timeline (all meetings with a specific person)
5. Morning digest
6. CRM deal intelligence
7. Speaker talk-ratio analysis
8. Meeting load audit (who's overloaded on the team)
9. Domain/company frequency analysis
10. Webhook management (create, delete, list)
