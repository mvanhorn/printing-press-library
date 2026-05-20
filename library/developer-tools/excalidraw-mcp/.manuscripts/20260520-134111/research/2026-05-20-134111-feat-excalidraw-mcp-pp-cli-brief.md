# Excalidraw MCP CLI Brief

## API Identity
- Domain: Diagram creation and canvas automation — whiteboard, flowcharts, architecture diagrams, wireframes, ERDs
- Users: Software engineers, technical writers, AI agents building content pipelines, devops teams documenting infrastructure, educators
- Data profile: Canvas elements (shapes, text, arrows, images), diagram snapshots, exported images (PNG/SVG), Excalidraw+ cloud scenes/collections

## Reachability Risk
- **Low (canvas server)** — local Express.js server on 127.0.0.1:3000; no internet required, no bot protection, direct HTTP
- **Low (cloud API)** — api.excalidraw.com/api/v1 with Bearer token auth; standard REST, no known blocking

## Top Workflows
1. **CLI-driven diagram creation** — create architecture/flowchart elements from scripts, templates, or AI-generated specs without opening a browser; export to PNG/SVG for docs
2. **Mermaid → Excalidraw pipeline** — convert Mermaid code to live Excalidraw diagrams, export as image; useful for CI/CD doc generation
3. **Snapshot & versioning** — save named canvas checkpoints before destructive edits, restore to known-good states, diff element counts
4. **Batch diagram assembly** — create multiple related diagrams from a YAML/JSON manifest in one command; import into Excalidraw+ scenes
5. **Cloud scene management** — organize diagrams into collections, manage workspace members, sync local exports to cloud

## Table Stakes (must match competing tools)
- `mcp-tools` (Go, ~1.6k stars): call any MCP tool from CLI, scripting-friendly JSON output, interactive shell mode, all three transports
- `excalidraw-cli` (Node.js, ahmadawais): JSON in → `.excalidraw` file, export to excalidraw.com
- `@swiftlysingh/excalidraw-cli` (Node.js): DSL-based flowchart creation, ELK.js auto-layout, PNG/SVG export
- `@tommywalkie/excalidraw-cli` (Node.js): `.excalidraw` JSON → PNG via node-canvas + Rough.js
- `excalidraw-brute-export-cli` (Node.js): Playwright/Firefox headless export to SVG/PNG

## Data Layer
- Primary entities: elements (shape, text, arrow, etc.), snapshots, exported files, cloud scenes, collections
- Sync cursor: canvas element count + timestamp from `/api/sync/status`
- FTS/search: element IDs, types, labels, bounding-box queries
- Local SQLite stores: element history, snapshot registry, export log, cloud scene index

## Codebase Intelligence
- Source: yctimlin/mcp_excalidraw source analysis (~1.9k stars)
- Auth: None for canvas server (localhost); Bearer token for Excalidraw+ cloud
- Data model: ServerElement with id, type, x/y, width/height, strokeColor, backgroundColor, fillStyle, strokeWidth, roughness, opacity, text, boundElements, groupIds
- Architecture: Canvas server (Express + WebSocket) ↔ MCP server (stdio) ↔ AI agent. Async pattern: export/viewport POST triggers WebSocket to browser frontend, frontend POSTs result back to `/result` endpoint. Go CLI targets canvas server REST directly (no MCP protocol overhead needed).
- Rate limiting: None for local canvas server. Excalidraw+ cloud API: standard 429 with Retry-After header.

## Product Thesis
- Name: excalidraw-mcp-pp-cli
- Why it should exist: The only Go CLI for Excalidraw's canvas server REST API. Every other CLI is Node.js, requires npm/npx, and can't pipe cleanly into Go/shell automation pipelines. This CLI gives agent-native teams `--json` output, typed exit codes, offline element search, and a snapshot-based version history — all without keeping a browser open. It also bridges local canvas work with Excalidraw Plus cloud in one binary.

## Build Priorities
1. Element CRUD + batch-create (the core canvas control surface)
2. Mermaid conversion command (`from-mermaid`) with export pipeline
3. Snapshot save/restore with SQLite history registry
4. Export to PNG/SVG (async two-step handled transparently)
5. Excalidraw Plus cloud: scenes list/get/create/update, collections CRUD
6. `doctor` command: health check for both canvas server and cloud API
7. Transcendence: diagram-diff, template library, batch-from-yaml, element-stats
