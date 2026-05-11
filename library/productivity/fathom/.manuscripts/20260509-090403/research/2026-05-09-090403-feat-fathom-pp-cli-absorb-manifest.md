# Fathom CLI Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 1 | List meetings | Fathom MCP (all) | `meetings list` with date/team/domain/recorder filters | Offline from SQLite; --json, --select, --agent |
| 2 | Get meeting | Fathom MCP (Dot-Fun) | `meetings get <id>` with full details | Returns from local store first, live fallback |
| 3 | Get transcript | Fathom MCP (all) | `recordings transcript <id>` | Local from store; --format raw/speaker/csv |
| 4 | Get summary | Fathom MCP (all) | `recordings summary <id>` | Local from store; --format markdown/json |
| 5 | Get action items | Fathom MCP (lukas-bekr) | `meetings action-items <id>` | Local from store; --assignee filter |
| 6 | Search meetings | Fathom MCP (druellan) | `search "<term>"` (built-in) | FTS5 offline; works on transcripts + summaries |
| 7 | List teams | Fathom MCP (Dot-Fun) | `teams list` | From local store |
| 8 | List team members | Fathom MCP (Dot-Fun) | `team-members list` | --team filter |
| 9 | Create webhook | Fathom MCP (lukas-bekr) | `webhooks create` | Full params; --dry-run; shows secret |
| 10 | Delete webhook | Fathom MCP (Dot-Fun) | `webhooks delete <id>` | --yes to confirm |
| 11 | Export meetings | Fathom MCP (export-all) | `sync --full` + `meetings export` | Markdown export with auto-filename |
| 12 | Filter by domain | Fathom MCP (lukas-bekr) | `meetings list --domain` | AND/OR domain filter |
| 13 | Filter by recorder | Fathom MCP | `meetings list --recorded-by` | Multiple recorders |
| 14 | Filter by team | Fathom MCP | `meetings list --team` | Multiple teams |

## Transcendence (only possible with our approach)

| # | Feature | Command | Why Only We Can Do This |
|---|---------|---------|------------------------|
| 1 | Cross-meeting commitment tracker | `commitments` | Cross-meeting action item join across all meetings in SQLite; no API endpoint exists for "all my open promises" |
| 2 | Topic frequency + trend | `topics` | FTS5 term counting over all transcripts by ISO week; rate limit makes live scan of 100+ transcripts infeasible |
| 3 | Pre-call account brief | `brief` | Participant-keyed retrospective joining meetings + summaries + action_items; no single API endpoint for this |
| 4 | Engagement velocity tracker | `velocity` | Month-by-month cadence per external domain computed from all participant records; requires local join |
| 5 | Team meeting workload | `workload` | Per-member weekly meeting hour aggregation joined across all meetings; live scan exhausts rate limit for 20-person team |
| 6 | Account relationship history | `account` | Domain-keyed view across meetings, topics, action items; Fathom is recording-centric, not account-centric |
| 7 | Stale transcript detector | `stale` | Store integrity introspection: differentiate never-synced vs. empty vs. missing; live API cannot answer this |
| 8 | CRM gap audit | `crm-gaps` | Three-table join: CRM-matched meetings with no action items; sales hygiene impossible with per-meeting live calls |
| 9 | Recording coverage report | `coverage` | Recurring meeting coverage by title pattern over time; requires historical local data |
