# Framer CLI Brief

## API Identity
- Domain: Website builder / design tool (no-code platform)
- Users: Designers, developers, agencies porting/building sites in Framer
- Data profile: CMS collections (blog posts, portfolio items, products), canvas nodes (pages, components, frames), code files (React components, overrides), assets (images, SVGs, files), styles (colors, text), localization data, redirects

## Reachability Risk
- [Low] Server API is in open beta (Feb 2026), free during beta. WebSocket-based with JS SDK only (`framer-api` npm). No REST endpoints documented. Go CLI will need Node.js bridge pattern (Go → Node.js subprocess using framer-api package).

## Top Workflows
1. **Bulk CMS import** — Import blog posts, portfolio items, products from CSV/JSON/Markdown into Framer CMS collections. The #1 pain point for site porting.
2. **Publish & deploy** — Preview changes, then deploy to production programmatically. CI/CD integration.
3. **Content sync** — Keep CMS in sync with external sources (Notion, Airtable, headless CMS). Triggered by webhooks or schedules.
4. **Asset management** — Bulk upload images/files, manage assets across projects.
5. **Project scaffolding** — Set up redirects, custom code, SEO config, localization from a config file.

## Table Stakes
- Connect to project via API key
- List/create/update/delete CMS collections and items
- Manage collection fields (add, remove, reorder)
- Upload images and files
- Create and manage code files (React components, overrides)
- Publish preview deployments
- Deploy to production
- Get project info and publish status
- Manage redirects
- View change history (getChangedPaths, getChangeContributors)

## Data Layer
- Primary entities: Projects (connection configs), Collections (CMS), Items (CMS records), CodeFiles, Assets, Styles, Nodes, Redirects
- Sync cursor: Version-based (Framer tracks versions internally)
- FTS/search: Over CMS items (by field content), code files (by name/content)

## Codebase Intelligence
- Source: framer/server-api-examples (GitHub), tmcpro/framer-mcp (GitHub)
- Auth: API key per project, set in site settings. Passed as second arg to `connect()` or via `FRAMER_API_KEY` env var.
- Data model: Projects contain Collections (CMS), each Collection has Fields and Items. Canvas has Nodes (frames, text, images). Code files are standalone React components.
- Rate limiting: None documented (beta). Non-transactional — partial failures possible.
- Architecture: WebSocket-based stateful channel. Cold start on first command (~1-2s), then fast. JS SDK only (`framer-api` npm). V8 sandbox execution on server side.

## Existing Tools
1. **tmcpro/framer-mcp** — MCP server (Node.js/Express + WebSocket + Framer Plugin). Supports nodes, selection, components, code files, project info. Most complete existing tool.
2. **Sheshiyer/framer-plugin-mcp** — MCP server for Framer plugins with web3 capabilities.
3. **superprat/framer-design-mcp-server** — MCP server for designing web pages via Server API.
4. **WestonThayer/framer-cli** — Legacy CLI for Framer Studio (outdated, Framer X era).
5. **steveruizok/framer-tools** — Legacy Framer X CLI tools (outdated).
6. **kyo504/framer-x-cli** — Legacy Framer X CLI (outdated).
7. **remorses/unframer** — Extract Framer components for use in React.

## User Vision
- User is actively porting a site into Framer and finding everything manual. Needs automation for the tedious parts of site migration.

## Product Thesis
- Name: framer-pp-cli
- Why it should exist: Framer's Server API is powerful but only accessible via JavaScript SDK. No CLI exists for the modern Server API. Designers and developers porting sites into Framer do everything manually — importing content, uploading assets, setting up redirects, configuring SEO. A CLI automates all of this, making Framer scriptable from any CI/CD pipeline, terminal, or AI agent.

## Build Priorities
1. CMS bulk operations (import/export collections and items from CSV/JSON/Markdown)
2. Publish and deploy pipeline (preview → deploy with rollback)
3. Asset management (bulk upload, list, organize)
4. Project configuration automation (redirects, custom code, localization)
5. Transcendence features (diff, migration wizard, template scaffolding)

## Technical Architecture Notes
- **Node.js bridge pattern**: Go CLI will use embedded Node.js scripts that import `framer-api`. Go handles CLI UX (flags, --json, --select, SQLite store, FTS). Node.js handles WebSocket communication with Framer.
- **Requirement**: Node.js 18+ and `framer-api` npm package must be installed. CLI `doctor` command verifies prerequisites.
- **Auth**: `FRAMER_API_KEY` env var + project URL stored in local config. Multi-project support via named profiles.
