# Ori CLI Absorb Manifest

## Source Tool Catalog

| Tool | Type | Notes |
|------|------|-------|
| `mcp-a2a-bridge` | MCP server (internal) | Per-agent suffix pattern: `chat_<agent>`, `cancel_<agent>`, `list_tasks_<agent>`, `resume_<agent>`, `approvals_list_<agent>`, `approvals_respond_<agent>`. The only "competing" tool; we are absorbing and beating it. |
| `openclaw-a2a-server` raw HTTP | JSON-RPC | The underlying surface. No CLI wrapper exists. |
| `launchctl` (system) | Built-in | Used today via memorized incantations; we wrap the operationally relevant subset. |

No external competing CLIs exist for this surface — it's an internal home-lab tool.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|---------------------|-------------|
| 1 | Send a message to ori | bridge `chat_ori` | `ori chat ori "..."` | Shell access, `--json`, `--context`, scriptable, no Claude Code in the loop |
| 2 | Send a message to adam | bridge `chat_adam` | `ori chat adam "..."` | Same as above, one binary for both agents |
| 3 | Resume task by id | bridge `resume_ori`/`resume_adam` | `ori tasks resume <id> --agent <name>` | Unified across agents, `--json` output |
| 4 | Cancel a task | bridge `cancel_*` | `ori tasks cancel <id> --agent <name>` | Cross-agent, idempotent on terminal tasks |
| 5 | List in-flight tasks | bridge `list_tasks_*` | `ori tasks list --agent <name>` | `--state`, `--since`, `--context`, `--limit`, `--json`, `--csv`, `--select` |
| 6 | Get one task snapshot | (not in bridge — only via list) | `ori tasks get <id> --agent <name>` | New capability, direct fetch by id |
| 7 | List pending approvals | bridge `approvals_list_*` | `ori approvals list [--agent <name>]` | **Cross-agent default** (single pane), `--agent` filters |
| 8 | Respond to approval | bridge `approvals_respond_*` | `ori approvals respond <id> --decision approve\|deny [--reason] --agent <name>` | Unified verb across agents |
| 9 | Discover agent list | direct `/.well-known/agents.json` | `ori agents list` | Caches in SQLite, `--json` output |
| 10 | Inspect agent card | direct `/.well-known/<n>/agent-card.json` | `ori agents get <name>` | Pretty-printed capabilities, `--json` |
| 11 | Health check | direct `curl /healthz` | `ori health` | Composable with `ori doctor` |
| 12 | Streaming response output | bridge MCP progress notifications | SSE-aware client in `chat` command | Renders chunks live in TTY, accumulates in JSON mode |
| 13 | `input_required` handling | bridge MCP elicitation | `ori chat ... --on-input prompt\|fail\|stdin` | Three modes: interactive prompt (default in TTY), exit 7 (default non-TTY), stdin pipe for scripts |

**13 absorbed features.** Every bridge MCP tool is matched. Plus 4 features the bridge doesn't expose (direct task fetch, cross-agent approvals, agent caching, configurable input-required behavior).

## Transcendence (only possible with our approach)

These are the hand-built commands that justify the CLI's existence beyond the absorbed set. Each scores ≥7/10 on user-value-per-build-hour for this user's actual operational pain.

| # | Feature | Command | Score | Why Only We Can Do This |
|---|---------|---------|-------|------------------------|
| 1 | One-verb stack diagnostic | `ori doctor` | 9/10 | Bundles 7 separate diagnostic commands the user runs today: launchd state for the a2a-server, `/healthz` reachability, both agents responding to a probe message, `~/.openclaw/agents.toml` validity, the plugin cache env var commented-out check in NAS compose, codex OAuth refresh freshness, gateway WebSocket tunnel reachability. The MCP bridge has zero visibility into launchd / compose / OAuth state. |
| 2 | Launchd bridge kickstart | `ori kickstart [--wait]` | 8/10 | Wraps `launchctl kickstart -k gui/$(id -u)/dev.error2.openclaw-a2a-server` plus a `/healthz` poll with timeout. Today this is a 2-step ritual the user has to remember after every `compose down/up`. |
| 3 | Sync to local SQLite | `ori sync [--full]` | 9/10 | Paginates `ListTasks` across all configured agents, upserts into the local store, refreshes approvals. Foundation for every other transcendence feature. The bridge has no persistence. |
| 4 | FTS5 search across task history | `ori tasks search <query>` | 9/10 | "What did ori work on related to kanban hygiene this week" — impossible via the bridge because tasks aren't persisted anywhere queryable. Filters: `--agent`, `--since`, `--state`. Output shows task_id, agent, state, first matching line. |
| 5 | Contexts (conversation grouping) | `ori contexts list [--agent <name>] [--since <duration>]` | 8/10 | Derives conversation contexts from cached tasks (group by `context_id`, show first/last activity, task count, peek of first user message). Lets the user resume a forgotten thread by context rather than remembering task ids. |
| 6 | Live state watcher | `ori watch [--agent <name>] [--interval 5s]` | 7/10 | Polls `ListTasks` every N seconds and prints state transitions live: "ori task abc123: running → input_required at 23:01:32". Useful for "is ori still working?" without paying for a Claude Code turn. |
| 7 | Logs tail | `ori logs tail [--stream stdout\|stderr] [--lines N]` | 7/10 | Wraps `tail -F ~/Library/Logs/openclaw-a2a-server/{stdout,stderr}.log` with `--stream` switching. Today the user has to remember the path. |
| 8 | Cross-agent approval queue | `ori approvals pending [--watch]` | 8/10 | Default `approvals list` aggregates across all agents (table-stakes for absorbed feature 7), but `approvals pending --watch` polls every N seconds for new requests and prints them as they arrive. Replaces eyeballing two MCP bridge tools alternately. |

**8 transcendence features, all ≥7/10.** Five of them (doctor, kickstart, sync, tasks search, contexts list) are direct responses to operational pain documented in the user's project memory.

## Build-Priority Summary

- **P0 (foundation, generator-emitted):** SQLite store schema for tasks, agents, approvals, contexts; sync engine; FTS5 indexes.
- **P1 (13 absorbed features):** All bridge MCP tools matched + 4 new direct-fetch capabilities. Generator emits most as REST-shaped Cobra commands from the spec.
- **P2 (8 transcendence features):** Hand-built. All ≥7/10. None are stubs.

## Stub Disclosure
- None. Every feature in this manifest is shipping-scope. No stubs.

## Source Priority
- Single source: `openclaw-a2a-server` on `localhost:8788`. No combo CLI logic needed.

## Economics
- Free. Loopback only. No paid tier, no rate limits, no API key.
