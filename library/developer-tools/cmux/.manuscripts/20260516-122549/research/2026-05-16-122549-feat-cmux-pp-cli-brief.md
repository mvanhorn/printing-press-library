# cmux CLI Brief

## API Identity
- Domain: local macOS terminal/browser multiplexer ("cmux.app"). 139 RPC methods over a Unix socket at `/tmp/cmux.sock`, exposed through a CLI binary at `/Applications/cmux.app/Contents/Resources/bin/cmux`.
- Users: developers running multiple Claude Code / AI-agent sessions across many workspaces; ecosystem-manager and cto-agent skills that poll panes for state changes; people who want notify-style loop closure between agent panes.
- Data profile: ephemeral live state (windows, workspaces, panes, surfaces, sidebar statuses, notifications). All state already exists in two surfaces — the socket RPCs and the on-disk session JSON at `~/Library/Application Support/cmux/session-com.cmuxterm.app.json`. Status entries carry `key` (e.g. `claude_code`), `value` ("Running" / "Needs input"), `icon`, `color`, `timestamp` — the timestamp lets us build a real timeline once we persist snapshots.

## Reachability Risk
- None for the cmux binary itself (local IPC).
- Indirect risk: cmux.app must be running. Probing via `cmux ping` is instant and authoritative; doctor must surface "cmux not running" as a first-class diagnostic.
- Socket auth: `--password` flag > `CMUX_SOCKET_PASSWORD` env > keychain entry. No network auth.

## Top Workflows
1. **Cross-pane content search.** "Which workspace/surface mentions 'WAF cookie'?" Today: `cmux find-window --content "X"` returns matching *workspaces* but not which *surface* matched and no snippet — too coarse to switch a tab. The user explicitly called this out: "Search across panes (titles, workspaces, text in terminal for AI or just search to switch tabs would be cool)."
2. **Notify-driven loop closure.** ecosystem-manager polls every tick via `capture-pane` (expensive in context). `cmux list-notifications` + the planned fsnotify daemon would push events instead. The user called this out: "We could use notify to close the loop instead of polling for agent 'manager' use cases."
3. **Agent-status timeline.** `statusEntries` already carry per-workspace agent state with timestamps; nothing persists them, so there's no history of "how long has this pane been Needs-input?" or "when did claude_code go Running on workspace 6?" — exactly what the manager needs to decide what to triage next.
4. **Workspace inventory with computed agent state.** Decode `statusEntries` + title icons (braille spinner = working, ✻/✶/✳ = awaiting input) into a single normalized state column. Combine with `surface-health` so the manager can list `awaiting-input` panes in one call.
5. **Better alerting from cmux into the user's environment.** `cmux notify` only writes a sidebar notification; the printed CLI should be able to fan out to macOS user notifications, Slack DM (`D19JPP7RQ`), or any webhook, and watch for state transitions (e.g. "tell me when claude_code on Tuck transitions from Running → Needs input").

## Table Stakes (vs the cmux CLI itself)
- Mirror the read-only side of every cmux noun cleanly: workspaces, windows, panes, pane-surfaces, status, notifications, log entries, hooks.
- `--json`, `--select`, `--csv`, `--compact`, `--quiet` on every read. (cmux already emits JSON when `--json` is passed; we layer the Printing Press' selector/CSV layer.)
- `doctor`: cmux binary present, cmux running, socket auth resolves, env vars (`CMUX_WORKSPACE_ID` / `CMUX_SURFACE_ID`) sane.
- `sync` + `search` + `sql` against a local SQLite store, like every other printed CLI, but the store also stores time-series snapshots (every sync writes a `status_snapshots` row per workspace).

## Data Layer
- Primary entities: `workspaces`, `windows`, `panes`, `surfaces`, `status_entries`, `notifications`, `pane_content_samples`, `status_snapshots` (time series).
- Sync cursor: `notification.id` for new notifications since last sync; for status, append a new `status_snapshots` row per (workspace, key) when the value or icon changes.
- FTS/search: FTS5 over `surfaces.title`, `notifications.body`, `pane_content_samples.text`, `workspaces.title`. This is the cross-pane search.

## Codebase Intelligence
- DeepWiki: not applicable (cmux is a private macOS app; no public GitHub source).
- Auth: socket-only. `--password` / `CMUX_SOCKET_PASSWORD` / keychain. No HTTP, no API key.
- Architecture: live state in app memory; persisted state in session JSON; agents talk to the app via socket. Status transitions are observable in two places: the socket emits notifications via `notification.create*`, and the session JSON file is rewritten on every state change.

## User Vision (from prompt)
- "Enhanced AI optimized CLI for cmux.dev (it has a CLI)."
- "We could use notify to close the loop instead of polling for agent 'manager' use cases."
- "See what ecosystem-manager and cto-agent do with tmux to learn some use cases and optimize it."
- "Could be opportunity to notify the user with better alerting."
- "Search across panes (titles, workspaces, text in terminal for AI or just search to switch tabs would be cool)."

## Product Thesis
- Name: `cmux-pp-cli` — the AI-agent-optimized companion to cmux.app.
- Why it should exist: cmux's CLI is RPC-shaped (one method = one verb); agents and managers need a *state-shaped* surface (lists, filters, time-series, FTS, notifications-as-events). The companion CLI gives ecosystem-manager and friends a notify-driven event loop, cross-pane search with surface granularity + snippets, and a persistent status timeline — without forking cmux.

## Build Priorities
1. Read-only mirror of every cmux noun (`workspaces list/get`, `windows list`, `panes list`, `surfaces list/get`, `status list`, `notifications list`, `log list`, `hooks list`) with the Printing Press' `--json`/`--select`/`--csv` layer.
2. Local SQLite store + `sync` that captures workspaces, surfaces, statuses, notifications, and writes a status-snapshot row on every change.
3. **Cross-pane search** (`search "X"`) — FTS over titles + notification bodies + sampled pane content; returns workspace + surface + snippet, with a `--switch` flag that calls `surface.focus` to switch to the match (write-side, opt-in).
4. **Notify-driven watch loop** (`watch` / `tail`) — long-running `notifications list` poller with a debounce, emitting JSON lines per event; pluggable sinks (stdout, file, exec hook, Slack webhook).
5. **Agent-status timeline** (`status timeline`, `status awaiting`) — query the snapshot table for "how long has each workspace been Needs-input?", "which workspaces flipped state in the last hour?", "what's stuck > 30 min?".
6. **Better alerting** (`alert`) — declare watch rules (`alert add --workspace Tuck --on awaiting --sink slack`), fire when sync detects a matching state transition.
7. **Title-icon decoding** as a reusable column (`status icons`, exposed everywhere the title is shown) so callers don't have to re-parse braille glyphs.
8. **Pane content sampling** (`panes sample --workspace X --surface Y --lines 60`) — captures via `read-screen` + scrollback and persists into the FTS table.
9. Doctor: cmux installed, cmux running, socket auth resolves, sample RPC succeeds.
10. MCP surface that exposes the read-only commands so the ecosystem-manager subagent can call them without shelling out.
