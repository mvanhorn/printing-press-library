# Excalidraw MCP CLI Absorb Manifest

## Sources Inventoried
1. **yctimlin/mcp_excalidraw** (~1.9k stars) — 26 MCP tools, canvas server REST API
2. **excalidraw/excalidraw-mcp** (~4.5k stars) — official MCP Apps streaming server (no discrete tools to absorb)
3. **mcp-tools** (f/mcptools, ~1.6k stars, Go) — general MCP CLI: `tools`, `call`, `shell`, `proxy`, `mock`, `alias`, `guard`
4. **excalidraw-cli** (ahmadawais, Node.js) — JSON in → `.excalidraw`, checkpoint management
5. **@swiftlysingh/excalidraw-cli** (Node.js) — DSL-based flowchart creation, ELK.js layout, PNG/SVG export
6. **@tommywalkie/excalidraw-cli** (Node.js) — `.excalidraw` → PNG via node-canvas
7. **excalidraw-brute-export-cli** (Node.js) — Playwright/Firefox headless PNG/SVG export
8. **Excalidraw Plus Cloud API** (plus.excalidraw.com, 41 endpoints) — scenes, collections, workspace, users, invites, logs

---

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 1 | List all canvas elements | yctimlin MCP (`query_elements`) | `elements list` → JSON array | `--json`, `--select`, `--type` filter, offline after sync |
| 2 | Create element | yctimlin MCP (`create_element`) | `elements create --type rect --x 10 --y 10 --width 200 --height 100` | `--dry-run`, template presets |
| 3 | Update element | yctimlin MCP (`update_element`) | `elements update <id> --stroke-color "#ff0000"` | `--dry-run`, field-level patch |
| 4 | Delete element | yctimlin MCP (`delete_element`) | `elements delete <id>` | `--dry-run`, confirm prompt |
| 5 | Search elements by type/bbox | yctimlin MCP (`query_elements`) | `elements search --type arrow --x-min 0 --x-max 500` | `--json`, composable |
| 6 | Batch create elements | yctimlin MCP (`batch_create_elements`) | `elements batch --file spec.json` | Read from stdin or file; `--dry-run` |
| 7 | Duplicate elements | yctimlin MCP (`duplicate_elements`) | `elements duplicate <id>` | offset positioning flags |
| 8 | Clear canvas | yctimlin MCP (`clear_canvas`) | `elements clear` | `--dry-run`, requires `--confirm` |
| 9 | Align elements | yctimlin MCP (`align_elements`) | `elements align --ids "id1,id2" --direction left` | `--dry-run` |
| 10 | Distribute elements | yctimlin MCP (`distribute_elements`) | `elements distribute --ids "id1,id2" --axis horizontal` | `--dry-run` |
| 11 | Group elements | yctimlin MCP (`group_elements`) | `elements group --ids "id1,id2"` | named groups |
| 12 | Ungroup elements | yctimlin MCP (`ungroup_elements`) | `elements ungroup --group-id <gid>` | — |
| 13 | Lock elements | yctimlin MCP (`lock_elements`) | `elements lock --ids "id1,id2"` | — |
| 14 | Unlock elements | yctimlin MCP (`unlock_elements`) | `elements unlock --ids "id1,id2"` | — |
| 15 | Describe canvas scene | yctimlin MCP (`describe_scene`) | `scene describe` | `--json` for agent parsing |
| 16 | Canvas screenshot | yctimlin MCP (`get_canvas_screenshot`) | `scene screenshot` | `--output <file.png>` |
| 17 | Save snapshot | yctimlin MCP (`snapshot_scene`) | `snapshots save <name>` | SQLite history registry |
| 18 | Restore snapshot | yctimlin MCP (`restore_snapshot`) | `snapshots restore <name>` | `--dry-run` shows diff |
| 19 | List snapshots | yctimlin REST `/api/snapshots` | `snapshots list` | `--json`, age display |
| 20 | Export to PNG/SVG | yctimlin MCP (`export_to_image`) | `export image --format png --output diagram.png` | wraps async 2-step transparently |
| 21 | Export to .excalidraw file | yctimlin MCP (`export_scene`) | `export scene --output diagram.excalidraw` | `--compact` strips empty fields |
| 22 | Import .excalidraw file | yctimlin MCP (`import_scene`) | `import scene --file diagram.excalidraw` | `--merge` to add without clearing |
| 23 | Export to shareable URL | yctimlin MCP (`export_to_excalidraw_url`) | `export url` | prints URL + QR code flag |
| 24 | Mermaid → Excalidraw | yctimlin MCP (`create_from_mermaid`) | `from-mermaid --input diagram.mmd --export png` | one-step pipeline with export |
| 25 | Set viewport | yctimlin MCP (`set_viewport`) | `viewport set --zoom 1.5 --scroll-to-content` | — |
| 26 | Read diagram guide | yctimlin MCP (`read_diagram_guide`) | `guide` | `--json` for agent parsing |
| 27 | Health check | yctimlin REST `/health` | `doctor` | checks canvas server + cloud API |
| 28 | Sync status | yctimlin REST `/api/sync/status` | `doctor --verbose` | element count, ws clients, memory |
| 29 | Create `.excalidraw` from JSON | excalidraw-cli (ahmadawais) | `elements batch --file spec.json` | extends with field validation |
| 30 | PNG export via headless | excalidraw-brute-export-cli | `export image --format png` | wraps canvas server, no Playwright |
| 31 | MCP tool discovery | mcp-tools (`tools`) | `mcp tools --server-url http://...` | lists all tools on any MCP server |
| 32 | MCP tool call | mcp-tools (`call`) | `mcp call <tool> --params '{}'` | structured JSON output |
| 33 | List cloud scenes | Excalidraw+ cloud | `cloud scenes list` | `--json`, pagination |
| 34 | Get cloud scene | Excalidraw+ cloud | `cloud scenes get <id>` | `--content` flag for full data |
| 35 | Create cloud scene | Excalidraw+ cloud | `cloud scenes create --name "diagram"` | — |
| 36 | Update cloud scene content | Excalidraw+ cloud | `cloud scenes push <id> --file diagram.excalidraw` | upload local export to cloud |
| 37 | Delete cloud scene | Excalidraw+ cloud | `cloud scenes delete <id>` | `--dry-run`, `--confirm` |
| 38 | List collections | Excalidraw+ cloud | `cloud collections list` | — |
| 39 | Create collection | Excalidraw+ cloud | `cloud collections create --name "Q2 Arch"` | — |
| 40 | Add scene to collection | Excalidraw+ cloud | `cloud collections add-scene <coll-id> <scene-id>` | — |
| 41 | List workspace users | Excalidraw+ cloud | `cloud workspace users` | — |
| 42 | Send workspace invite | Excalidraw+ cloud | `cloud workspace invite --email <email>` | — |
| 43 | List audit logs | Excalidraw+ cloud | `cloud workspace logs` | `--json`, `--limit` |
| 44 | Sync canvas to SQLite | (novel) | `sync` | element history, search |
| 45 | FTS search over elements | (novel) | `search "<term>"` | matches text labels, types, IDs |
| 46 | SQL query over elements | (novel) | `sql "SELECT * FROM elements WHERE type='arrow'"` | full SQLite access |

---

## Transcendence (only possible with our approach)

| # | Feature | Command | Why Only We Can Do This | Score |
|---|---------|---------|------------------------|-------|
| 1 | Diagram diff between snapshots | `diff <snap1> <snap2>` | Requires local SQLite snapshots — no tool tracks element-level changes between saves | 10/10 |
| 2 | Batch diagram generation from YAML manifest | `batch --manifest diagrams.yaml` | Requires orchestrating: create → populate → export → snapshot cycle per diagram; no tool supports a declarative multi-diagram pipeline | 10/10 |
| 3 | Element change history / audit log | `elements history <id>` | Requires SQLite-persisted element mutations; no other tool tracks per-element deltas across sessions | 9/10 |
| 4 | Diagram template library | `templates apply <name>` | Requires a local SQLite template registry with parameterized element sets; none of the existing CLIs have templates | 9/10 |
| 5 | Auto-layout from adjacency spec | `layout --from spec.yaml --engine elk` | Requires computing positions from an adjacency/relationship YAML without the user placing elements manually; ELK.js-inspired but Go-native | 9/10 |
| 6 | Canvas element stats & analytics | `stats` | Requires aggregation across element types, bounding-box analysis, color distribution — only possible with local store | 8/10 |
| 7 | Sync local canvas to cloud on save | `watch --sync-cloud` | Requires file-watch loop + canvas server polling + Excalidraw+ push; no tool bridges local and cloud automatically | 8/10 |
| 8 | Diagram stale-check for docs repos | `stale --since 90d` | Requires correlating `.excalidraw` files in a git repo with their last-modification date; flags diagrams that need updating | 8/10 |
| 9 | Import Mermaid from file + export PNG in one step | `convert --input flow.mmd --output diagram.png` | Combines three MCP tool calls (from_mermaid → snapshot → export_image) transparently; no CLI does this in one invocation | 7/10 |
| 10 | Agent-friendly context dump | `context --agent` | Dumps: element count, types histogram, color palette in use, bounding-box summary — optimized to fit agent context windows | 7/10 |

---

## Stubs (explicit, requires user acknowledgement)

| Feature | Status | Reason |
|---------|--------|--------|
| `scene screenshot` | (stub for export path) | Requires browser canvas open for async screenshot; degrades gracefully if unavailable |
| `export image --format png` | (async, may degrade) | Async two-step (POST → WebSocket → result POST); if browser not open, returns error with helpful guidance |
| `layout --engine elk` | (stub in v1) | ELK.js layout requires Go port or subprocess; ships as stub that shows spec-generated positions |
| `watch --sync-cloud` | (scaffold) | File-watch + cloud push loop; v1 ships skeleton with manual `sync-cloud` command |

---

## Novel Feature Summary
- 10 absorbed competitor sources → 46 absorbed features
- 10 transcendence features (all scoring ≥ 7/10)
- This is the only Go CLI for Excalidraw; the nearest competitor (`mcp-tools`) is a generic MCP client with no Excalidraw-specific knowledge
