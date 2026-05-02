# Absorb Manifest

## Absorbed
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List and inspect organizations | Sentry API / sentry-mcp | `organizations` endpoint commands | JSON, select/csv, MCP mirror, local sync |
| 2 | List projects, teams, members, and environments | Sentry API | generated resource commands | consistent flags and agent output |
| 3 | List, retrieve, and inspect issues | Sentry API / Sentry CLI / sentry-mcp | generated issue commands | broad filtering and local search path |
| 4 | Fetch issue events, event IDs, tags, and hashes | Sentry API / sentry-mcp | generated issue/event commands | agent-readable debugging context |
| 5 | Inspect releases, commits, deploys, and files | Sentry API / official Sentry CLI | generated release commands | structured release audit output |
| 6 | Inspect monitors, check-ins, alert rules, workflows, and notifications | Sentry API | generated monitor/alert commands | one binary for operations audit |
| 7 | Inspect replays, replay selectors, and replay deletion jobs | Sentry API | generated replay commands | structured replay inventory |
| 8 | Query stats, sessions, event tables, and timeseries | Sentry API | generated analytics commands | scriptable JSON and CSV output |
| 9 | Agent-native Sentry access | sentry-mcp | generated MCP server plus Cobra-tree mirror | local stdio MCP without hosted middleware |
| 10 | Structured CLI output | Sentry CLI docs | generated `--json`, `--select`, `--csv`, `--compact` | predictable shell and agent use |

## Transcendence
| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|--------------|----------|
| 1 | Offline incident search | `search` | 8/10 | Syncs readable resources into SQLite and FTS, then searches locally. | Sentry issue triage and agent debugging workflows need fast context lookup. |
| 2 | SQL incident audits | `sql` | 8/10 | Lets operators query synchronized Sentry resources with read-only SQL. | SRE/project audits span projects, teams, issues, releases, and monitors. |
| 3 | Reusable agent context | `context` | 7/10 | Prints command and auth context for agents before selecting endpoint commands. | Sentry MCP and Sentry CLI both target agent workflows. |
| 4 | Full local export | `export` | 7/10 | Exports synchronized resources for incident reports and offline review. | Incident retros need portable evidence. |
| 5 | Broad MCP mirror | `sentry-pp-mcp` | 7/10 | Mirrors user-facing Cobra commands as MCP tools in addition to typed endpoint tools. | Sentry MCP validates agent demand but is hosted middleware-focused. |

## Risk Notes
- Mutating endpoints exist in the OpenAPI. Live testing must remain read-only unless disposable fixtures are explicitly approved.
- Some endpoints require organization/project/release/issue identifiers; verification may need fixture discovery from list endpoints.
