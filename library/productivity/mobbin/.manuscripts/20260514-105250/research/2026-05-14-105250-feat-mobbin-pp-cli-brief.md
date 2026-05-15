# Mobbin CLI Brief

## API Identity
- **Domain:** Curated design-inspiration library — real shipped apps' screens, flows, UI elements, paywalls, onboarding patterns. Scale (May 2026): 621,500+ screens, 142,200+ flows, ~1,150 apps across iOS / Android / Web.
- **Users:** Product designers, PMs, UX researchers running competitor teardowns, pattern decks, and feature-design crits.
- **Data profile:** Curated, slow-changing reference content. Heavily benefits from local SQLite (offline FTS across screens/flows/apps/patterns/elements; saved decks, version-diff snapshots).

## Reachability Risk
- **Low for current internal routes; medium-term tightening expected.** No Cloudflare/captcha 403s reported in any wrapper repo. `pdcolandrea/mobbin-mcp` (the most-used wrapper) is currently functional — open issue #35 (Apr 28 2026) reports 401 after first call due to auto-refresh fragility, but the API itself responds. Mobbin launched their official MCP (`api.mobbin.com/mcp`, OAuth, paid-only) on **May 11 2026** — they likely prefer agents go through that channel and may eventually tighten internal-route access.
- Daily token-refresh required for cookie sessions (Supabase Auth standard). Captured cookie is good for ~24h, then needs re-export or refresh-token round-trip.

## Top Workflows
1. **Pattern teardown** — pull 10 paywall examples across fintech for a design crit.
2. **Onboarding audit** — "show me every B2B SaaS first-run flow from the last 6 months."
3. **Element library cross-app** — empty states / error states / settings across an industry vertical.
4. **Longitudinal app evolution** — how did Airbnb's filter UI change across versions (paid only).
5. **Collection-driven research** — assemble + share curated screen decks for stakeholders.

## Table Stakes (from competitor analysis)
- Search (apps, screens, flows, elements) with filters (platform / industry / pattern / element).
- Get app detail (logo, slug, latest version, flow list).
- Get app screens (paginated, with platform + version filters).
- Get app flows (with screen list, action steps).
- Get screen detail (image URLs, applied patterns, applied elements, parent flow).
- Get available filters (platforms, industries, patterns, elements).
- Auth: browser session import + token refresh.
- Image download (thumb + full-res, where plan allows).

## Data Layer
- **Primary entities:**
  - `apps` (id, slug, name, platform, industry, company_hq, company_stage, logo_url, latest_version_id, updated_at)
  - `app_versions` (id, app_id, version, captured_at)
  - `screens` (id, app_version_id, image_url, image_url_full, captured_at, flow_id?, position_in_flow?)
  - `flows` (id, app_version_id, name, screen_ids[], action_count)
  - `elements` (id, name, category)
  - `patterns` (id, name, category)
  - `screen_patterns`, `screen_elements` (junction)
  - `collections` (id, name, owner, screen_ids[])
  - `industries`, `platforms` (reference)
- **Sync cursor:** `updated_at` per entity; full re-list when filters change.
- **FTS/search:** FTS5 across screens (name + caption + pattern names), flows (name + app + actions), apps (name + slug + industry).

## Codebase Intelligence
- **pdcolandrea/mobbin-mcp** — TypeScript MCP wrapping internal Supabase routes
  - Endpoints accessed: `POST /api/content/search-apps`, `POST /api/content/search-screens`, `POST /api/content/search-flows`, `GET /api/content/app/<slug>/screens`, `GET /api/content/app/<slug>/flows`, `GET /api/content/screen/<id>`, `GET /api/content/filters`.
  - Auth: Supabase JWT cookies `sb-ujasntkfphywizsdaapi-auth-token.0` / `.1` (split JWT). Stores refreshed token at `~/.mobbin-mcp/auth.json`.
  - Refresh: POST `/auth/v1/token?grant_type=refresh_token` to Supabase project.
- **ismailsaleekh/mobbin-agent** — Playwright-driven, structured-JSON extraction, batch 1920px PNG downloads via `gather.mjs`.
- **solejay/mobbin-cli** — closest existing CLI; commands `app screens download`, `shots download`, `auth status`; multi-profile auth (`--profile`).
- **underthestars-zhy/MobbinAPI** (Swift, stale 2023) — covers app history, collections CRUD, flow tree builder.

## User Vision
User confirmed: **logged-in Chrome session available**. CLI must support `auth login --chrome` to import Supabase cookies from Chrome's cookie store (defense-in-depth: also accept `--cookie-file` for HAR/exported tokens). The CLI should not require a separate Mobbin API key — there isn't one.

## Auth Model
- **Type:** `composed` — Supabase JWT carried in two cookies (`sb-ujasntkfphywizsdaapi-auth-token.0/.1`) **plus** the regular session cookie.
- **Auth flow:** Chrome cookie import → store JWT pair in `~/.mobbin-pp-cli/auth.json` → refresh via `https://ujasntkfphywizsdaapi.supabase.co/auth/v1/token?grant_type=refresh_token` when access token expires (~1h).
- **Free tier:** Limited content (latest 4 apps); most endpoints respond with paginated previews.
- **Pro tier:** Full library, full image resolutions, version history.

## Product Thesis
- **Name:** `mobbin-pp-cli`
- **Why it should exist:** Every existing tool is either (a) Mobbin's own MCP (paid-only, agent-only — no terminal interface), (b) reverse-engineered MCP servers (no local store, no batch downloads, no offline search), or (c) a closed-source Chrome extension. Nobody ships a CLI that (1) lets you offline-search every screen you've explored, (2) downloads decks for design crits, (3) tracks version drift across your favorite apps, and (4) works alongside Mobbin's official MCP. This is the gap.

## Build Priorities
1. **Spec-source:** browser-sniff to discover `/api/content/*` shapes (the user's logged-in Chrome session enables full-content sniffing). Author internal YAML spec with `auth.type: cookie` and Chrome cookie import.
2. **Core absorbed surface:** match every tool in pdcolandrea/mobbin-mcp + solejay/mobbin-cli (search apps/screens/flows, get app screens/flows/filters, screen detail, batch downloads, auth profiles).
3. **Local store:** SQLite mirror of synced apps/screens/flows/patterns/elements, FTS5 for cross-entity search.
4. **Novel features (transcendence):**
   - Cross-app pattern decks (`mobbin deck "fintech paywalls" --limit 20 --export-zip`)
   - Version drift watch (`mobbin watch <app> --since 30d` — show what flows changed)
   - Offline pattern bench (`mobbin bench "onboarding" --industry b2b-saas`)
   - "Inspired by" reverse-search (drop a competitor's screenshot, find closest Mobbin matches)
   - Local screen library SQL (`mobbin sql "SELECT app, pattern FROM screens WHERE element='paywall' GROUP BY app"`)

## Source Priority
Single source (Mobbin only) — no inversion risk.

## Reachability strategy
- **Discovery:** browser-sniff with logged-in Chrome (user confirmed); fall back to `pdcolandrea/mobbin-mcp` source for endpoint shapes.
- **Runtime:** stdlib HTTP + Supabase cookie import; no headless browser required for command execution.
