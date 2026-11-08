# Milanote CLI Brief

## API Identity
- Domain: app.milanote.com — visual boards for creative work (notes, headings, images, links, to-dos, columns, nested boards on a free-form canvas).
- Users: designers, writers, filmmakers, students; the operator uses it for short-film development (story / characters / shot list / look / storyboards / locked script).
- Data profile: tree of boards; each board = elements with `elementType` (CARD, BOARD, COLUMN, IMAGE, LINK, FILE, TASK_LIST, COMMENT_THREAD, …), `location {parentId, section CANVAS|INBOX|TRASH, position {x,y}|{score}}`, rich-text `content.textContent` (ProseMirror doc). Low write volume, moderate read volume.
- No public API, no webhooks, no official MCP (Milanote roadmap request; confirmed by community projects, checked 2026-11).

## Reachability Risk
- Medium. Undocumented private API. Reads are JSON REST under `/api/*` with the HttpOnly session cookie; writes go over the web app's socket.io (Engine.IO 4) channel, which also answers plain HTTP long-polling (`/socket.io/?userId=…&EIO=4&transport=polling`). The site sets `aws-waf-token` + AWSALB cookies: AWS WAF may fingerprint TLS (the cotlet project impersonates Chrome TLS via curl_cffi). Must be verified from Go's stdlib client in Phase 1.9.
- Free plan stops at 100 cards — writes can be refused by quota.

## Top Workflows
1. Read a whole board tree (board + sub-boards) as structured data / Markdown — e.g. pull the a short film script and shot list into files.
2. Push structured documents onto a board: a Markdown file → headings + note cards laid out on a grid (what was done by hand for the film: scene headings + note per scene).
3. Add / edit / move individual elements: note, heading, image (upload), link, to-do, column, sub-board, at exact canvas x/y.
4. Search across all boards for text; list recent boards; walk the board tree.
5. Housekeeping: trash/restore, rename, duplicate board as a template, export via Milanote's own exporter.

## Table Stakes
- From cotletdev/milanote-api-mcp-server (Python, 30 tools, Elastic-2.0): list/recent/search/get/create/rename/duplicate/export boards; add/edit note; add link; to-do lists + tick task; columns; move card; colour; labels; trash/list-trash/restore; comments add/edit/delete; sharing get/invite/remove/publish/unpublish; usage. Does NOT do: headings, images upload, canvas x/y placement (everything lands in Unsorted), layout, offline search.
- From serhanceetin/Milanote-Unofficial-API (Python, 2023, stale): element model incl. sketch, document, color swatch, annotation, location.
- setobox/milanote-api (TS worker): public shared boards → JSON only.

## Data Layer
- Primary entities: boards, elements (cards), comments, labels.
- Sync cursor: none server-side; sync = walk board tree from `rootBoardId` via `/api/boards?ids=` and upsert by `_id` + `meta.modifiedTime`.
- FTS/search: local FTS5 over element plain text + board titles, so search works offline and greps across every board at once.

## User Vision
- the operator: "create a CLI for Milanote" to read/create/edit boards, notes, headings, images and layout, driven from agents (this chat session) instead of slow phone/browser UI automation. Logged in via Google in his Chrome on the mini.
- the operator (2026-11-08 09:59): "a keen focus on organization, proper/equal spacing and utilizing the tools available for checkboxes, headings, lists, etc." → layout engine with an even grid/gutters (auto-arrange, align, distribute), and rich-text output that uses Milanote's native heading nodes, bullet/ordered lists and task (checkbox) items instead of plain paragraphs.

## Product Thesis
- Name: milanote-pp-cli
- Why it should exist: Milanote is canvas-first and has no API; every existing tool dumps cards into the Unsorted panel and ignores layout. This CLI treats the canvas as addressable: place headings/notes/images at exact positions, lay out a Markdown document as a column grid, export a board tree to Markdown, and search all boards offline from a local mirror — agent-native JSON output throughout.

## Build Priorities
1. Auth from the logged-in Chrome session (cookie import) + `doctor`/`whoami`; Go HTTP read path to `/api/users/me`, `/api/boards?ids=`, `/api/boards/search`, `/api/users/me/history`.
2. Socket.io polling write channel in Go (ELEMENT_CREATE / ELEMENT_UPDATE / ELEMENT_MOVE_MULTI) with acks; element-id generator (base62 secs + clientId + tick).
3. Element commands: note, heading, image (upload), link, todo, column, board; edit text; move to x/y; trash/restore.
4. `board tree`, `board export --md`, `sync` into SQLite + `search` FTS.
5. `layout` from Markdown: `##` → heading, body → note, columns on a grid.
