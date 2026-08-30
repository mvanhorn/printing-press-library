# MCP Market CLI Brief

## API Identity
- Domain: mcpmarket.com — directory/marketplace of 45,110+ MCP servers, MCP clients, and Agent Skills (Claude Code/Cowork, Codex, Cursor, etc.)
- Users: developers and agent builders discovering MCP servers/skills to wire into their AI clients; agents themselves searching for a tool to solve a task
- Data profile: catalog entries (server/client/skill) with name, description, author/owner, category tags, GitHub repo, popularity ("like"/star count), FAQ, related items. Highly stable read-heavy catalog; no user-generated mutation needed for the public side.

## Reachability Risk
- Low, with one caveat: the apex domain (`mcpmarket.com`) sits behind a Vercel Security Checkpoint (`x-vercel-mitigated: challenge`) that 403/429s plain `stdlib` HTTP but is cleared cleanly by Surf's Chrome-TLS-fingerprint transport (confirmed via `probe-reachability`, mode=`browser_http`, confidence 0.85, consistent across `/`, `/server/<slug>`, `/search`, `/leaderboards`, and paginated listing paths).
- `docs.mcpmarket.com` and `app.mcpmarket.com` are NOT behind the checkpoint — both respond directly to plain HTTP (200 and 401 respectively).
- No official public "catalog search API" is documented. `docs.mcpmarket.com/docs/mcp-servers/browsing-the-catalog` and `.../connecting-an-mcp-server` explicitly describe UI-only workflows ("Use the search box... to filter by name", "Enter your API key and click Connect"). The only documented API surface is Bearer-token account access (`Authorization: Bearer sk_user_...` → `GET https://app.mcpmarket.com/api/v1/me`), intended for org/team/toolkit management, not catalog discovery.
- Probe-safe endpoint used: `GET https://app.mcpmarket.com/api/v1/me` (returns clean `401 application/json` unauthenticated — confirms the API is live and reachable, not blocked).

## Browser-Sniff Findings (chrome-devtools MCP, pre-approved by user)
- Every catalog page (home, `/server/<slug>`, `/client/<slug>`, `/tools/skills/<slug>`, `/categories/<slug>`, `/search?q=`, `/leaderboards`, `/tools/skills/leaderboard`, `/daily`, `/daily/skills`) is server-rendered Next.js and embeds clean **schema.org JSON-LD** (`<script type="application/ld+json">`) directly in the initial HTML — `SoftwareApplication` for detail pages, `ItemList`/`SearchResultsPage` for listings/search/leaderboards, `FAQPage` for FAQs, `BreadcrumbList` everywhere. This is a first-class structured-extraction target: no fragile CSS scraping needed for the primary catalog fields (name, description, url, category, GitHub author/org, like-count, related items, feature list).
- Search is a plain server-rendered `GET /search?q=<query>` — no client-side XHR fetch of results. The `ItemList` JSON-LD on that page IS the search result set.
- Found one genuine public JSON API: `GET https://mcpmarket.com/api/similar-tools?slug=<slug>&category=<category>&type=server|skill` → `200 application/json`, returns `[{id, name, slug, description, owner:{name,avatar,url}}, ...]`. No auth required.
- Skill detail pages expose a `SKILL.md` tab whose full frontmatter + body text is present in the rendered DOM (`role="tabpanel"`) without a separate network fetch — it's embedded server-side. Extractable via HTML selector, not a raw markdown URL.
- A "Download skill" button exists on skill pages (produces `SKILL.md` or `.tar.gz`) — per docs this is a snapshot download; the CLI should surface the *instruction*/link, not silently auto-download without explicit user action (matches this session's own file-download consent rule).
- The authenticated management surface (connect a server to your org, create/manage toolkits, install a skill via auto-sync, team/billing/observability) is real (Bearer-token API confirmed live) but its endpoint shapes beyond `/me` are undocumented and were not captured live (no test account/session was available this run). Scoping this out of shipping-scope avoids inventing endpoints.

## Top Workflows
1. Search the catalog for a server/client/skill by keyword and see ranked results with popularity.
2. Look up full detail on one server/client/skill (description, features, category, GitHub author, install/config guidance, related items).
3. Browse by category or check what's trending today / all-time leaderboard.
4. Compare/discover related tools around one you already use (`similar-tools`).
5. Get the raw SKILL.md content for a skill to inspect or vendor into a project offline.

## Table Stakes (from Smithery CLI, the closest incumbent)
`@smithery/cli`: `install <server> --client <client>`, `uninstall`, `list clients`, `list servers --client`, `inspect <server>`, `run <server> --config`. MCP Market itself has no first-party CLI to date — this print is the first.

## Data Layer
- Primary entities: `server`, `client`, `skill`, `category` (each with slug primary key, JSON-LD-derived fields)
- Sync cursor: category/leaderboard/daily crawl, keyed by slug + last-seen popularity count
- FTS/search: local SQLite FTS mirrors name/description/category/author so `search` works offline once synced, in addition to live `/search?q=`

## Product Thesis
- Name: MCP Market CLI (`mcpmarket-pp-cli`)
- Why it should exist: MCP Market has zero official CLI/API wrapper today, unlike Smithery. Every discovery workflow (search, category browse, leaderboard, trending-today, related-tools) is currently a manual browser click-through behind a bot-protection edge. This CLI gives agents and developers scriptable, `--json`-native, offline-searchable access to the same 45k+ catalog, with the added ability to persist and diff what's trending over time (`sync` + local SQLite), something the website itself does not offer.

## Build Priorities
1. Local data layer for server/client/skill/category (JSON-LD extraction via Surf transport) + sync + offline FTS search
2. Absorb: search, list server/client/skill, category browse, leaderboard (all-time + daily), similar-tools, FAQ read, SKILL.md content view
3. Transcend: trending-delta tracking (what moved up/down since last sync), cross-category comparison, "what should I install for X" recommendation chaining off similar-tools
