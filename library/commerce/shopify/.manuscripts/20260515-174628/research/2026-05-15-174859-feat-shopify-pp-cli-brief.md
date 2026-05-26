# Shopify CLI Brief (Reprint)

## API Identity
- Domain: Ecommerce. Shopify Admin GraphQL API at /admin/api/{api_version}/graphql.json on a shop domain (e.g. mystore.myshopify.com).
- Users: Merchants, ops teams, agencies, and increasingly agentic clients (Claude Desktop, OpenAI Codex CLI, custom MCP consumers) that need to read and act on store data.
- Data profile: Orders, products, customers, inventory, fulfillment-orders, plus async bulk-operations for large exports.

## Reachability Risk
- None. Shopify Admin GraphQL is officially supported; auth via X-Shopify-Access-Token from a custom app. No public rate-limit issues at small dogfood scale.

## User Vision
- User reprint reason: "codex".
- Interpret as: bias the brainstorm toward MCP / agent-consumer surfaces (OpenAI Codex CLI being a primary MCP-consuming agent). Agent-native depth on the Admin GraphQL surface is the top axis when scoring novel features. Polish on the human CLI is secondary.

## Top Workflows
1. Sync recent orders + products to local SQLite for offline query and agent context.
2. Search across local resources (orders by email, products by title/sku) without burning API calls.
3. Run bulk-operations exports and poll/inspect status — the only sane way to ship large extracts from Shopify GraphQL.
4. Compose ad-hoc analytics SQL over the synced data (revenue by day, low-inventory variants, repeat buyers).
5. From an agent: answer "what's the status of order #1234", "which products are out of stock", "summarize last 7 days" — without making the agent learn GraphQL.

## Table Stakes
- Read access to orders, products, customers, inventory, fulfillment-orders.
- Cursor-based pagination (`first`/`after`) on every list endpoint.
- Bulk-operations primitives.
- Agent-native output: --json, --select dotted paths, --compact, structured exit codes.
- MCP server (stdio + http) so Codex / Claude Desktop / hosted agents can consume the same surface.

## Data Layer
- Primary entities: orders, products, customers, inventory_items, fulfillment_orders.
- Sync cursor: per-entity updatedAt watermark.
- FTS/search: per-table FTS5 over text fields (order name+email, product title+handle, customer name+email).

## Product Thesis
- Name: shopify-pp-cli.
- Why it should exist: every existing Shopify tool is either a heavy framework CLI (Shopify CLI) or per-MCP point tool. This one ships agent-native output, a local SQLite store for offline / repeated queries, bulk-operations primitives, AND an MCP surface with both stdio and HTTP transport — so a Codex / Claude / hosted-agent client can drive it without the agent ever touching GraphQL syntax.

## Build Priorities
1. Regenerate on machine v4.6.1 (was v3.2.1) to pick up runtime cobratree MCP walker, scoring fixes, narrative emission improvements.
2. Add `mcp.transport: [stdio, http]` so MCP consumers beyond stdio (Codex CLI is stdio; hosted agents prefer http) can connect.
3. Keep the prior 5 absorbed resources and bulk-operations command — they are still the right surface.
4. Sample novel features post-gen via the novel-features subagent under the codex/MCP bias.
