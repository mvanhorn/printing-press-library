# Orgo CLI Absorb Manifest

## Source tools cataloged

| Tool | Type | URL | Stars | Features absorbed |
|------|------|-----|-------|-------------------|
| `@orgo-ai/mcp` | TS MCP server | https://github.com/nickvasilescu/orgo-mcp | n/a | 24 |
| `orgo` Python SDK | PyPI | https://pypi.org/project/orgo (docs.orgo.ai) | n/a | ~12 (Computer methods) |
| `orgo` npm SDK | npm | https://www.npmjs.com/package/orgo | n/a | ~12 (Computer class) |
| Orgo OpenAPI 3.1 v2.0.0 | spec | https://docs.orgo.ai/api-reference/openapi.json | n/a | 29 paths (full surface) |
| `n8n-nodes-orgo` | n8n integration | https://www.npmjs.com/package/n8n-nodes-orgo | n/a | wraps create/control |
| `@pipedream/orgo` | Pipedream | https://www.npmjs.com/package/@pipedream/orgo | n/a | wraps create/control |

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 1 | List workspaces | `@orgo-ai/mcp` orgo_list_workspaces | `orgo workspaces list` | Auto-JSON when piped, `--select`, FTS5 offline |
| 2 | Get workspace | `@orgo-ai/mcp` orgo_get_workspace | `orgo workspaces get <id>` | Local cache, `--data-source local\|live\|auto` |
| 3 | Workspace by name | `@orgo-ai/mcp` orgo_workspace_by_name | `orgo workspaces get-by-name <name>` | Local index → 50ms lookup |
| 4 | Create workspace | `@orgo-ai/mcp` orgo_create_workspace | `orgo workspaces create --name X` | `--dry-run`, idempotent |
| 5 | Delete workspace | spec DELETE /workspaces/{id} | `orgo workspaces delete <id>` | `--yes` confirmation guard, batch via stdin |
| 6 | List computers | `@orgo-ai/mcp` orgo_list_computers | `orgo computers list [--workspace W]` | FTS5 over name/status, `--compact` token-conscious |
| 7 | Get computer | `@orgo-ai/mcp` orgo_get_computer | `orgo computers get <id>` | Accepts UUID OR fly_instance_id |
| 8 | Create computer | `@orgo-ai/mcp` orgo_create_computer | `orgo computers create --workspace W --name N` | `--snapshot` template, `--cpu/--ram/--disk-gb/--region` |
| 9 | Delete computer | `@orgo-ai/mcp` orgo_delete_computer | `orgo computers delete <id>` | `--yes`, accepts list via `--stdin` |
| 10 | Restart computer | `@orgo-ai/mcp` orgo_restart_computer | `orgo computers restart <id>` | Marked `mcp:destructive` |
| 11 | Clone computer | `@orgo-ai/mcp` orgo_clone_computer | `orgo computers clone <id>` | Tier-aware, 201 Created handling |
| 12 | Ensure running | `@orgo-ai/mcp` orgo_ensure_running | `orgo computers ensure-running <id>` | Idempotent, returns `{resumed: bool}` |
| 13 | Resize computer | `@orgo-ai/mcp` orgo_resize_computer | `orgo computers resize <id> --cpu N --ram N` | Handles 207 partial-success envelope |
| 14 | Move computer (workspace) | spec PATCH /computers/{id}/move | `orgo computers move <id> --to-workspace W` | Owner-only enforcement surfaces 403 cleanly |
| 15 | Start computer | spec POST /computers/{id}/start | `orgo computers start <id>` | — |
| 16 | Stop computer | spec POST /computers/{id}/stop | `orgo computers stop <id>` | — |
| 17 | Auto-stop config (read) | spec GET /computers/{id}/auto-stop | `orgo computers auto-stop <id>` | — |
| 18 | Auto-stop config (write) | spec PATCH /computers/{id}/auto-stop | `orgo computers auto-stop <id> --minutes N` | `0` disables; doctor flags absent values |
| 19 | VNC password | spec GET /computers/{id}/vnc-password | `orgo computers vnc-password <id>` | `--mcp:hidden` (sensitive) by default |
| 20 | Screenshot | `@orgo-ai/mcp` orgo_screenshot, py SDK .screenshot() | `orgo computers screenshot <id> [--out f.png]` | Caches PNG locally for `replay`/`audit` |
| 21 | Click | `@orgo-ai/mcp` orgo_click, py SDK .left_click() | `orgo computers click <id> --x N --y N --button left` | Logs to local actions store |
| 22 | Double click | py SDK .double_click() | `orgo computers click <id> --double` | — |
| 23 | Drag | `@orgo-ai/mcp` orgo_drag, py SDK .drag() | `orgo computers drag <id> --from X,Y --to X,Y` | Validates coords against last screenshot resolution |
| 24 | Type text | `@orgo-ai/mcp` orgo_type, py SDK .type() | `orgo computers type <id> --text "..."` | `--stdin` for long text |
| 25 | Press key | `@orgo-ai/mcp` orgo_key, py SDK .key() | `orgo computers key <id> --key Enter` | — |
| 26 | Scroll | `@orgo-ai/mcp` orgo_scroll, py SDK .scroll() | `orgo computers scroll <id> --direction down --amount 3` | — |
| 27 | Wait | spec POST /computers/{id}/wait | `orgo computers wait <id> --duration 5` | — |
| 28 | Bash | `@orgo-ai/mcp` orgo_bash, py SDK .bash() | `orgo computers bash <id> --command "ls"` | `--stdin` for multiline; logs to actions store |
| 29 | Exec Python | `@orgo-ai/mcp` orgo_exec, py SDK .exec() | `orgo computers exec <id> --code "..."` | `--stdin`, `--timeout`; logs to actions store |
| 30 | Stream start (RTMP) | spec POST /computers/{id}/stream/start | `orgo computers stream start <id> --connection-name N` | — |
| 31 | Stream status | spec GET /computers/{id}/stream/status | `orgo computers stream status <id>` | — |
| 32 | Stream stop | spec POST /computers/{id}/stream/stop | `orgo computers stream stop <id>` | — |
| 33 | List files | `@orgo-ai/mcp` orgo_list_files | `orgo files list --workspace W [--computer C]` | — |
| 34 | Export file (off VM) | `@orgo-ai/mcp` orgo_export_file | `orgo files export <id> --path /tmp/foo` | — |
| 35 | Upload file (to VM) | `@orgo-ai/mcp` orgo_upload_file | `orgo files upload --workspace W --file ./local.txt` | `--computer` optional |
| 36 | Download file URL | `@orgo-ai/mcp` orgo_download_file | `orgo files download <id>` | Returns presigned URL; `--write-to <path>` to fetch and save |
| 37 | Delete file | spec DELETE /files/delete | `orgo files delete <id>` | `--yes` confirmation guard |
| 38 | Local SQLite mirror | discrawl pattern | `orgo sync` mirrors workspaces + computers + files; FTS5 over names + status | Offline search, 50ms latency vs API round-trip |
| 39 | Standard agent flags | press defaults | every command: `--json`, `--select`, `--compact`, `--dry-run`, `--quiet`, `--csv`, `--stdin` | Auto-JSON when piped (no flag) |
| 40 | Typed exit codes | press defaults | 0=ok, 2=usage, 3=not found, 4=auth, 5=API, 7=rate-limited | Branch on `$?` instead of grepping stderr |

## Transcendence (only possible with our approach)

These come from the novel-features brainstorm subagent (3-pass: customer model → 2× candidates → adversarial cut). Full audit trail in `2026-05-08-215235-novel-features-brainstorm.md`. Persona-served column shown in audit; survivors only here.

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 1 | Fleet doctor | `orgo doctor [--workspace W]` | 8/10 | Joins `GET /workspaces` and `GET /computers` across the user's whole account, flags `suspended` (over-quota), `error`, `creating`/`stopping` stuck > N min, and probes `ORGO_API_KEY` validity. | Brief Top Workflow #3 ("Fleet ops"); brief flags `suspended` semantics + stale `ORGO_API_KEY` in `.zshrc`. |
| 2 | Idle computers | `orgo idle [--threshold-hours 24]` | 7/10 | From the local actions log, finds the most recent action timestamp per computer, joins with `status=running`, orders by hours-idle desc. | Brief Top Workflow #4 ("Cost/quota stewardship: which computers are idle"); brief data layer lists "actions" as fourth primary entity. |
| 3 | Oversized computers | `orgo oversized [--min-cores 4] [--idle-days 7]` | 6/10 | From local SQLite, flags computers with CPU >= 4 cores OR RAM >= 16 GB whose last CLI-recorded action is older than `--idle-days` AND `auto_stop_minutes` is null/large. Configuration-vs-activity heuristic. | Brief Build Priority #6; brief Top Workflow #4 names "which computers are oversized." |
| 4 | Agent action replay | `orgo replay <computer-id> [--since 1h] [--out replay.html]` | 9/10 | Reads the local actions log and locally cached screenshots, emits a single self-contained static HTML file with timeline, inline screenshots, bash transcripts, exec snippets. | Brief Build Priority #6 explicitly names "agent action replay"; brief NOI cited verbatim. |
| 5 | Audit trail | `orgo audit [--workspace W] [--since 7d]` | 9/10 | FTS5 index over the local actions log scoped by workspace + time window; chronological table of every CLI-driven screenshot/click/bash/exec. Pipeable to `claude` for narration. | Brief NOI; brief data layer lists "actions" as fourth primary entity. |
| 6 | Action grep | `orgo grep "<query>" [--type bash\|exec\|click] [--computer C]` | 8/10 | FTS5 over the local actions log's bash text + exec code + click coordinates. Pure local SQLite, no API call. | Brief data layer: "search over historical bash commands and exec snippets the CLI itself ran." |
| 7 | Bulk prune | `orgo prune [--status suspended,error] [--older-than 7d] [--dry-run]` | 7/10 | Lists computers across all workspaces matching status + age; dry-run by default; on confirm, loops `DELETE /computers/{id}`. | Brief Top Workflow #3: "Fleet ops — list every workspace's computers, see which are running/idle/suspended, clean up." |
| 8 | Cost breakdown | `orgo cost [--workspace W] [--since 30d] [--forecast]` | 7/10 | From local action timestamps + observed status transitions, reconstructs per-computer running-hours, multiplies by per-tier $/hr, sums by workspace. `--forecast` projects month-end. | Brief Top Workflow #4: "Cost/quota stewardship... when does the user hit the quota"; Build Priority #6 names "quota forecasting." |
