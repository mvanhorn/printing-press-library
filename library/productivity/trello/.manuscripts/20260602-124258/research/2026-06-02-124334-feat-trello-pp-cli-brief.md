# Trello CLI Brief

## API Identity
- Domain: Kanban project management (boards / lists / cards). Part of Atlassian.
- Users: developers, PMs, solo makers, teams tracking work on visual boards.
- Data profile: hierarchical (org -> board -> list -> card -> checklist/comment/attachment), member assignments, labels, due dates, activity feed.

## Reachability Risk
- None. Official documented REST API at developer.atlassian.com/cloud/trello/rest/. Stable, public, auth via key+token query params. 324 operations in the spec.

## Top Workflows
1. Create/triage cards: add a card to a list, set due date, assign members, label it.
2. Move cards across lists (the core kanban motion) and across boards.
3. Search across all cards/boards for text, member, label, due window.
4. Review a board's state: what's in each list, what's overdue, who's loaded.
5. Checklist management: add checklist + items, tick items, track completion.

## Table Stakes (from competing tools: mheap/trello-cli, EndlessHoper/trello-mcp 39 tools, delorenj/mcp-server-trello, py-trello)
- boards list/get/create/update, board labels, board members
- lists create/update/archive-all/move-all/move, get list cards
- cards get/create/update/delete/copy/search, comments add/get/update/delete
- card labels add/remove, members assign/remove, attachments add/get/delete
- labels create/update/delete
- checklists create/update/delete/get + items add/update/delete/copy
- members get-me/get/boards/cards, search members

## Data Layer
- Primary entities: boards, lists, cards, checklists, checkitems, labels, members, organizations, comments (card actions), attachments.
- Sync cursor: board dateLastActivity / card dateLastActivity; actions feed for incremental.
- FTS/search: cards (name, desc), boards (name), comments text. Offline search is the big differentiator: Trello's own search is online-only and rate-limited.

## Product Thesis
- Name: trello-pp-cli (Trello)
- Why it should exist: every existing Trello CLI/MCP is a thin online wrapper. None give you a local SQLite mirror you can grep, SQL-query, and run cross-board analytics on offline. Add agent-native output (--json/--select/--agent), typed exit codes, --dry-run on every mutation, and transcendence commands (workload balance, overdue sweep, stale-card detection, board velocity) that require the local join no single API call provides.

## Build Priorities
1. Data layer + sync for boards/lists/cards/checklists/labels/members/comments.
2. Absorb ALL competitor features (full CRUD across the hierarchy).
3. Transcendence: offline cross-board analytics commands.
