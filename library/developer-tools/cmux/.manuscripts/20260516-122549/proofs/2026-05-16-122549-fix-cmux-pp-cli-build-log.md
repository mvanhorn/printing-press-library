# cmux-pp-cli Build Log

## What was built

### Priority 0: foundation
- Generator emitted full scaffolding (root, doctor, sync, search, sql, store, MCP walker, cliutil, helpers) — 8/8 quality gates passed on first generation.
- `internal/cmuxclient/` package wraps the cmux binary as a subprocess client. Implements: `Run`, `RunJSON`, `Ping`, `Version`, `ReadSession`, `ListWorkspaces`, `ResolveWorkspaceRef`, `ListNotifications`, `ListSurfaces`, `SurfaceHealth`, `ListStatusEntries`, `ListLogEntries`, `ListWindows`, `ListPanes`, `Capabilities`, `Hooks`, `Buffers`, `ReadScreen`, `FocusSurface`, `Dispatch`. Every entrypoint is annotated `// pp:client-call` per AGENTS.md anti-reimplementation rule. Plus `icons.go` with the canonical agent-state decoder (`IconState`, `CanonicalState`).
- `internal/client/client.go` surgical edit: when `BaseURL` contains `cmux.localhost` (the spec placeholder), `do()` dispatches to `cmuxclient.Dispatch` instead of issuing HTTP. Single early-return; the rest of the generated HTTP machinery stays dead-code-correct for non-cmux fallback.
- `internal/snapshotstore/` is a side-database (separate from the generator-emitted store) carrying `status_snapshots`, `alert_rules`, `alert_fires`, `notification_seen`, `pane_content_samples` (FTS5). Tested via `store_test.go`.

### Priority 1: absorbed (16 features from the absorb manifest)
All routed through the cmux IPC shim:
- workspaces list/get/current, windows list/current, panes list, surfaces list/health/read, status list, logs list, notifications list, hooks list, buffers list, capabilities list, doctor (now cmux-aware via `/` dispatch).

### Priority 2: transcendence (8 novel features per manifest)
- `search "<q>" [--switch]` — `internal/cli/cmux_search.go`. Multi-source search over workspace titles, surface titles, notification bodies, and FTS5 pane content samples. Returns workspace + surface + snippet. `--switch` calls `surface.focus` via `tab-action --action select` (verify-aware via `PRINTING_PRESS_VERIFY`).
- `watch` — `internal/cli/watch.go`. Long-running notification poll with id-cursor + debounce + pluggable sinks (stdout/file/exec/slack/webhook/macos). Supports `--source fsnotify` (session-JSON mtime poll) and `--one-shot`. Verify-aware short-circuit.
- `status timeline` / `status stuck` / `status awaiting` / `status changes` / `status icons` — `internal/cli/status_novel.go`. All five attach to the existing `newStatusPromotedCmd` parent via `addNovelStatusSubcommands`.
- `alert add/list/remove/fires/test` — `internal/cli/alert.go` + `internal/cli/sinks.go`. Rule storage in `alert_rules`, fire log in `alert_fires`, sink emission via `Sink.Emit` (parsed via `ParseSink`). `evaluateAlerts` runs on every `snapshot` against recent transitions; idempotent guard via `lastByRule` so the same transition fires once per rule.
- `workspaces card <ref>` — `internal/cli/workspaces_card.go`. Cross-entity join (workspace metadata + status + surfaces + last sampled pane snippets + recent notifications).
- `panes sample` — `internal/cli/panes_sample.go`. Captures screen text via cmux `read-screen` and writes into `pane_content_samples` FTS table.
- `snapshot` — `internal/cli/snapshot.go`. Records current observation per (workspace, key), invokes `evaluateAlerts` afterwards.

### Test coverage
- `internal/cmuxclient/icons_test.go` — IconState and CanonicalState tables.
- `internal/snapshotstore/store_test.go` — transitions, latest-per-key, alert rules CRUD, pane FTS.
- `internal/cli/sinks_test.go` — `ParseSink` table + `summarizeEvent` transition shape.

## What was intentionally deferred
- Per-notification surface-id → surface-ref mapping in `search` results. cmux notifications carry a `surface_id` (uuid) but not the `surface:N` ref; mapping would require an extra workspace probe. Left empty in notification hits; not a search-correctness issue.
- Decimating very long pane samples beyond the snippet truncation. FTS5 handles documents up to MB-scale comfortably.
- Long-running daemon mode for `watch` (the current shape runs in foreground and exits on ctx cancellation; mature deployment would be a launchd plist — out of scope for this print).

## Skipped body fields
None — every endpoint is a read-only GET; no bodies in spec.

## Generator limitations found
- Generated sync's ID extractor warns on `panes` (no `id` field; ref is the identity) and `capabilities` (each item is a string method name). Both flow into store as 0 rows; the novel commands don't rely on those generated tables (they use cmuxclient + snapshotstore directly). Could be a retro candidate: allow spec authors to declare `id_field: ref` (or similar) to override ID extraction.
- `newStatusPromotedCmd` is itself runnable (a shortcut to `status list`) AND a parent of `awaiting/timeline/stuck/changes/icons`. Cobra allows this; works in practice. Worth verifying the agent UX is clean across both surfaces.

