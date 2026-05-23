# cmux-pp-cli Absorb Manifest

## Absorbed (match or beat the existing cmux CLI)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List workspaces | cmux list-workspaces --json | `workspaces list --json --select` + SQLite cache | --select/--csv/--quiet, persisted, agent-friendly columns |
| 2 | List windows | cmux list-windows --json | `windows list` | structured + cached |
| 3 | List panes in a workspace | cmux list-panes --workspace W --json | `panes list --workspace W` | accepts workspace title or ref |
| 4 | List surfaces in a workspace | cmux list-pane-surfaces --workspace W | `surfaces list --workspace W` | --select, normalized JSON |
| 5 | Read screen | cmux read-screen [--scrollback] | `surfaces read --workspace W --surface S` | base64 decoded, lines bounded |
| 6 | Surface health | cmux surface-health --workspace W | `surfaces health` | flags stranded panes (`in_window=false`) |
| 7 | Sidebar status entries | cmux sidebar-state | `status list --workspace W` | structured rows, ts-ordered |
| 8 | Sidebar log | cmux sidebar-state | `logs list --workspace W` | structured + bounded |
| 9 | List notifications | cmux list-notifications --json | `notifications list` | persisted, deduplicated by id |
| 10 | Workspace metadata (cwd, branch, pr) | cmux sidebar-state | `workspaces get --workspace W` | structured fields |
| 11 | List hooks | cmux set-hook --list | `hooks list` | flat table |
| 12 | List buffers | cmux list-buffers | `buffers list` | structured |
| 13 | Current workspace / window | cmux current-* | `workspaces current`, `windows current`, `whoami` | one-shot view |
| 14 | Ping / version / capabilities | cmux ping / version / capabilities | `doctor`, `version`, `capabilities list` | doctor checks all three + socket auth |
| 15 | Title-icon decoder | none (folklore in cookbook) | `status icons --title "<text>"` | exposes the cookbook's icon-priority rule as a callable |
| 16 | Display message | cmux display-message -p | `display message <text>` | wrapper for completeness |

## Transcendence (only possible with our approach)
| # | Feature | Command | Score | Why Only We Can Do This |
|---|---------|---------|-------|--------------------------|
| 1 | Cross-surface FTS search with snippet + switch | `cmux-pp-cli search "<query>" [--switch]` | 9/10 | FTS5 over `surfaces.title`, `notifications.body`, `pane_content_samples.text`, `workspaces.title` populated by `sync` + `panes sample`. `cmux find-window` returns workspace-only; ours returns workspace + surface + line snippet, and `--switch` calls `surface.focus` to jump straight to the matching surface. |
| 2 | Notify-driven event stream (replaces polling) | `cmux-pp-cli watch [--sink stdout\|file\|exec\|slack\|webhook] [--source notifications\|fsnotify] [--since <id>]` | 9/10 | Long-running poll of `cmux list-notifications --json` with id-cursor + debounce, or fsnotify on `~/Library/Application Support/cmux/session-com.cmuxterm.app.json`. Pluggable sinks (stdout JSONL / file / exec hook / Slack webhook / generic HTTP webhook). Lets ecosystem-manager wait on events instead of full pane scans. |
| 3 | Agent-status timeline | `cmux-pp-cli status timeline [--workspace W] [--since 1h]` | 8/10 | Query local `status_snapshots` (written per change during `sync`) for "what state was workspace X in over time". cmux carries only current state — the timeline lives entirely in our store. |
| 4 | Stuck-pane triage | `cmux-pp-cli status stuck [--over 30m] [--key claude_code]` | 8/10 | Local SQL: latest `status_snapshots` row per (workspace, key) where value='Needs input' and timestamp older than threshold. ecosystem-manager today cannot answer "what has been stuck >30 min" because nothing persists snapshots. |
| 5 | Normalized agent-state column | `cmux-pp-cli status awaiting [--all]` | 7/10 | Pure local transform joining `status_entries` (key=claude_code) + title-icon decode + `surfaces health` into one canonical `state ∈ {idle, working, awaiting, stranded}`. Replaces three signals with one column. |
| 6 | State-transition alerts | `cmux-pp-cli alert add --workspace W --on awaiting --sink slack:<url>`, `alert list` | 7/10 | Rules stored in local SQLite; `sync` diffs prior vs new `status_snapshots` and fires matching transitions to user-supplied sinks (stdout / file / exec / macOS osascript / Slack webhook / generic webhook). cmux's `notify` is sidebar-only. |
| 7 | Recently-flipped workspaces | `cmux-pp-cli status changes --since 1h` | 7/10 | Local SQL over `status_snapshots`: list (workspace, key, prev_value, new_value, transitioned_at) within the window. Directly answers ecosystem-manager's "did anything change since my last tick?" without re-capturing every pane. |
| 8 | Per-workspace summary card | `cmux-pp-cli workspaces card <ref>` | 6/10 | Local cross-entity join: workspace metadata + current `status_entries` + last 3 `notifications` + last `pane_content_samples` snippet per surface. cmux requires N RPC calls to assemble this. |
