# Postman Explore CLI Brief

## API Identity
- **Domain:** Public API discovery directory at `postman.com/explore`. The "API Network" surface.
- **Users:** Developers searching for ready-made Postman Collections to learn or test third-party APIs (Stripe, Twilio, GitHub, etc.); API authors evaluating their own listing visibility; AI agents discovering public API surfaces with worked examples.
- **Data profile:** Five entity types — collections, workspaces, APIs, flows, teams — plus categories for filtering. ~40M-developer network. Unauthenticated browsing only; we are not modeling user-account features.
- **Surface:** All requests funnel through `POST https://www.postman.com/_api/ws/proxy` with a `{service, method, path, body?}` envelope. The catalog labels this `proxy-envelope` and Surf-with-Chrome-TLS clears Cloudflare without a clearance cookie. No auth required.

## Reachability Risk
- **Low.** `probe-reachability` returns `mode: browser_http` (Surf clears CF challenge). No clearance cookie capture in the Printed CLI. No GitHub issues found indicating systemic blocking of the proxy endpoint. The surface has been stable since the original sniff (2026-03-29).

## Top Workflows

1. **Search the public network for a known vendor's collection** — "Find Stripe's official collection so I can fork it." Power-user move. Today: open browser, click around. With the CLI: `postman-explore-pp-cli search "stripe" --type collection --json | jq`.
2. **Browse by category** — "Show me popular Communication APIs." Discovery without committing to a search term. Today: scroll category page. With the CLI: `postman-explore-pp-cli browse collections --category 12 --sort popular`.
3. **Trending discovery** — "What's getting attention this week on the API Network?" Today: visible only on the homepage feed. With the CLI: `postman-explore-pp-cli browse collections --sort week --limit 25 --json`.
4. **Compare publishers** — "Which teams have the most popular collections in DevTools?" Today: not directly answerable on the website. With the CLI + local store: `postman-explore-pp-cli teams top --category devtools --metric followers`.
5. **Track changes over time** — "Did Twilio publish anything new this month?" Today: impossible without scraping. With sync + local snapshots: `postman-explore-pp-cli watch team twilio --since 30d`.

## Table Stakes (must match)
- Search across all 5 entity types (collection, workspace, api, flow, team)
- Filter by category, sort by popular/recent/featured/new/week/alltime
- Pagination (limit/offset)
- Get full detail for a specific entity by slug or ID
- List categories
- Get team profile

These come straight from the catalog Spec's six endpoints: `listNetworkEntities`, `searchAll`, `getEntity`, `listCategories`, `getTeam`, `getStats` (or equivalent — to be confirmed by browser-sniff).

## Data Layer
- **Primary entities:** `collections`, `workspaces`, `apis`, `flows`, `teams`, `categories`. Single SQLite store. FTS5 over name + description + publisher.
- **Sync cursor:** `(entityType, sort=recent, offset)` paginating until we've covered the recent window. Full re-sync periodically; trending top-N every run.
- **High-gravity fields per entity:** `name`, `slug`, `summary`, `publisher.handle`, `publisher.id`, `categoryId`, `forkCount`, `viewCount`, `updatedAt`, `popularity`. The CLI's `--compact` output should return these only.
- **Why local store matters:** Watch commands, drift detection ("what changed since last sync"), top-N publisher analytics, offline FTS — none of these are achievable with a single live API call.

## Codebase Intelligence
- No SDK exists for postman.com/explore (the proxy is undocumented). All known Postman MCP servers (postmanlabs/postman-mcp-server, PostmanV3/postman-mcp-server, delano/postman-mcp-server) target the **authenticated** Postman API for managing personal/team workspaces — orthogonal surface, not feature-overlapping with discovery.
- The catalog Spec at `catalog/specs/postman-explore-spec.yaml` is the only known machine-readable description. Phase 1.7 will refresh it via browser-sniff per user request.

## User Vision
> "Ensure this is just for the postman.com/explore site. No authentication needed, only for looking up postman collections from the community."

This is an **explicit scope guard**: no `auth login`, no workspace-management commands, no private-network features, no Newman-style collection running. The CLI exists to help humans and agents **discover** community-contributed surfaces — nothing more.

## Product Thesis
- **Name:** `postman-explore-pp-cli`
- **Display name:** Postman Explore
- **Why it should exist:** The Postman API Network is the world's largest public API directory, but its discovery surface is web-only and unscriptable. Agents asking "is there a Postman Collection for service X?" cannot programmatically find out today. Postman's own CLI (`newman`, `postman-cli`) runs collections; the official MCP manages authenticated workspaces. Nothing today indexes the public network for offline search, agent-native query, or trend tracking.
- **Differentiator:** Local FTS over the entire network state, drift/watch commands no single API call could power, and `--json --select` ergonomics tuned for agent context budgets.

## Build Priorities

### Priority 0 (foundation)
- SQLite store covering all six entity types
- `sync` to populate from `listNetworkEntities` (paginated by entityType + sort) and `searchAll`
- `search` over the local FTS index
- `sql` for arbitrary local queries

### Priority 1 (absorbed table stakes)
- `browse collections|workspaces|apis|flows` — list with category/sort/pagination
- `search` — across all entity types or filtered by `--type`
- `get collection|workspace|api|flow <id>` — detail view
- `categories list`
- `teams get <handle>`
- `stats` — network-wide stats

### Priority 2 (transcendence)
Locally-derived commands that no live API call can answer; these come out of Phase 1.5.
