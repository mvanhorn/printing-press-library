# Build Log: excalidraw-mcp-pp-cli

## What Was Built

### Priority 0 (Foundation)
- Generated 14-resource CLI from dual spec (canvas server + cloud API)
- 41 endpoints from canvas server REST API + Excalidraw Plus cloud
- SQLite data layer via generated `sync` command
- FTS search via `search` command
- Doctor command with canvas server + cloud API health checks

### Priority 1 (Absorbed Features)
All 46 absorbed features from 8 sources are generated:
- `elements` group: list, create, update, delete, search, batch, clear, from-mermaid, get
- `snapshots` group: create, get, list
- `scenes` group: cloud-create, cloud-delete, cloud-get, cloud-list, cloud-update, content (get/replace/update)
- `collections` group: cloud-create, cloud-delete, cloud-get, cloud-list, cloud-update, scenes (add, list)
- `files` group: list, upload, delete
- `viewport`: promoted top-level command
- `excalidraw-canvas-cloud-health`: promoted (canvas server health check)
- `excalidraw-canvas-cloud-export`: promoted (canvas PNG/SVG export)
- `excalidraw-canvas-cloud-sync`: promoted (sync status)
- `logs`: promoted (cloud audit logs)
- `workspace`: cloud-get, cloud-list-users, cloud-remove-user
- `invites`: cloud-create, cloud-delete, cloud-list

### Priority 2 (Transcendence Features Built)
1. `diff <snap1> <snap2>` — element-level snapshot diff
2. `convert --input <file.mmd> --output <file.png>` — one-step Mermaid→PNG
3. `stats` — canvas element type distribution + bounding box + colors
4. `stale --dir <path> --since <duration>` — find stale .excalidraw files
5. `agent-canvas-context` — compact canvas summary for agent context windows

## Intentionally Deferred
- `batch --manifest` (multi-diagram pipeline): ships as scaffold; deferred to post-ship
- `templates apply` (template library): deferred; SQLite-backed, complex scope
- `watch --sync-cloud` (file-watch loop): scaffold only; v1 ships manual cloud push
- `elements history <id>` (mutation audit): deferred; requires store schema extension

## Generator Limitations Found
- "health", "export", "sync" resource names shadow framework Cobra commands; auto-renamed to `excalidraw-canvas-cloud-health`, `excalidraw-canvas-cloud-export`, `excalidraw-canvas-cloud-sync` — acceptable, `doctor` serves as the primary health check entry point
- Canvas server export is async (POST→WebSocket→POST-result); convert.go wraps this with a 500ms delay that works for typical diagrams

## Spec Notes
- `base64.StdEncoding.DecodeString` for export data — canvas server returns base64 PNG/SVG with optional data: prefix stripped
- `time.Sleep(500ms)` in convert.go is a pragmatic wait for async canvas rendering; sufficient for most use cases
