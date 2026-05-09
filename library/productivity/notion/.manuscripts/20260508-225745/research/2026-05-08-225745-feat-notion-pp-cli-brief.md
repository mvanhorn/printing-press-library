# Notion CLI Brief

## API Identity
- Domain: Workspace/knowledge management — pages, databases (data sources), blocks, comments, users
- Users: Teams using Notion as their primary knowledge base, project management, and linked-database workspace
- Data profile: Pages + database records + block trees + user/comment metadata; heavy relation graph between databases

## Reachability Risk
- **LOW** — No systematic 403 issues in top SDK repos. Rate limits not documented but not reported as a problem. Official OpenAPI spec at developers.notion.com/openapi.json (32 endpoints, current API version 2025-09-03). Token auth is bearer, no OAuth complexity for internal integrations.

## Top Workflows
1. **Cross-database compound queries** — "All open tasks for client X linked to project Y, owned by Z" — impossible via Notion UI, requires local join
2. **Stale page/database detection** — pages/databases untouched >30 days across the workspace
3. **Workspace export and archival** — backup Notion workspace to markdown, including linked databases
4. **Relation graph traversal** — which databases link to which, how deep the dependency is, organizational ontology
5. **Batch task creation** — create N items across one or more databases in one command

## Table Stakes (competing tools cover these)
- Page CRUD (view, create, update, delete)
- Database query with filters (human-readable filters, not raw JSON)
- Block operations (append, update, delete)
- Markdown I/O (export page as markdown, append markdown)
- Comment read/write
- User listing
- Full-text workspace search
- Raw API escape hatch
- JSON output mode
- File/image upload

## Data Layer
- Primary entities: pages, database items (data source items), blocks (content), databases, users, comments
- Sync cursor: `last_edited_time` on pages + data source items (incremental sync)
- FTS/search: FTS5 on page titles, block text, database property values
- Relations: store normalized FK links between pages/databases for local graph queries
- Key tables: `pages`, `data_source_items`, `blocks`, `databases`, `users`, `comments`, `relations`

## Codebase Intelligence
- Source: notion-sdk-js (official, makenotion/notion-sdk-js, 5,598 stars)
- Auth: Bearer token in Authorization header, env var `NOTION_TOKEN`
- Data model: Pages have properties (typed: text, select, date, number, checkbox, relation, people, url, email, phone, formula, rollup, created_by, last_edited_by). Databases/Data Sources are containers with typed schema. Blocks form a tree (children fetch requires separate call).
- Rate limiting: Not documented, not reported as a problem at scale
- Architecture: REST with JSON. Block content requires recursive tree traversal (no deep fetch in one call). Data Sources are the 2025 replacement for Databases.

## User Vision
- **VBT workspace**: multiple active clients (Ryder, TrackFrame, ClearPackaging, WorldEmblem, OP, CarnCorp, ShadeStore, bMedia)
- **Primary need**: cross-database compound queries that the Notion UI cannot answer
- **Secondary needs**: sync Notion state to other tools (Jira/Tempo/Slack), stale page detection, extract structured data for reporting, expose relation graph as queryable structure ("organizational ontology signal")
- **Same SQLite-mirror pattern as slack-pp-cli**

## Product Thesis
- Name: notion-pp-cli
- Why it should exist: Notion has the best data model but the worst query interface for power users. Every team that uses Notion heavily hits the same wall: the UI can't answer compound questions that span multiple linked databases. A local SQLite mirror with SQL access and compound CLI commands is the missing infrastructure layer between Notion and the humans (and agents) who need to reason across their entire workspace.

## Build Priorities
1. **SQLite sync** — incremental, relation-aware, FTS5 on titles+content — this is the foundation everything else builds on
2. **Cross-database query commands** — cross, stale, since, ontology — the transcendence commands that only work with a local store
3. **Full absorbed feature parity** — match notion-cli (4ier) and both official MCP servers
4. **Agent-native output** — --json, --select, --compact, typed exit codes on every command
5. **Batch operations** — create/update N items across databases with retry
