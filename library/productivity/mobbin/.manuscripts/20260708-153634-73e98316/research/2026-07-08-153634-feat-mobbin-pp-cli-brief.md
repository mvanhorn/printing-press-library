# Mobbin CLI Brief (reprint)

## API Identity
- **Domain:** Curated design-inspiration library — real shipped app UI: screens, user flows, UI elements, paywalls, onboarding patterns, across Web / iOS / Android. Scale (mid-2026): 621,500+ screens, 140,000+ flows, ~1,150 apps.
- **Users:** Product designers, design-systems leads, PMs, and founders running competitor teardowns, pattern decks, and feature-design crits. Increasingly also AI agents composing design research (Mobbin shipped its own MCP for exactly this).
- **Data profile:** Curated, slow-changing reference content keyed by stable UUIDs. Ideal for a local SQLite mirror — offline FTS across apps/screens/flows/patterns/elements/collections, saved decks, and longitudinal drift snapshots that the live API (current-state only) cannot answer.

## Users
- **Product designer prepping a Wednesday design crit** — needs 15-20 real paywall or empty-state examples across a vertical, downloaded full-res, ready to drop into Figma. Today this is a 20-tab screenshot-and-rename loop.
- **Design-systems lead cataloguing a pattern** — wants a cross-app leaderboard: "who ships this checkout pattern, how recently, on which platform?" — a shape no Mobbin endpoint returns.
- **Founder auditing competitor onboarding** — "show me every B2B-SaaS first-run flow from the last quarter" and "what did Stripe ship since last month?" — time-windowed and longitudinal, both absent from the live API.
- **Web product designer checking platform parity** — verifying the iOS reference still matches the desktop pattern for a fixed app set; requires two platform-scoped calls joined on app slug.

## Top Workflows
1. **Pattern teardown** — pull N paywall/checkout examples across a vertical for a crit (`screens search` + full-res grab).
2. **Onboarding / flow audit** — every B2B-SaaS first-run flow in the last 6 months, grouped by app.
3. **Element library cross-app** — empty states / error states / settings across an industry vertical.
4. **Longitudinal app evolution** — how a competitor's flows/screens changed across snapshots (local-only).
5. **Collection-driven research** — assemble and share curated screen/flow/app decks for stakeholders (now with write CRUD).

## Table Stakes (competitor parity)
- Search apps / screens / flows with filters (platform, app category, pattern, element, OCR keyword, animation, flow action).
- List apps by platform; popular apps + previews; paginated discover page.
- Full filter taxonomy (categories, patterns, elements, flow actions with definitions + counts).
- Trending apps / sites / filter-tags / OCR keywords; searchable sites list; cross-entity autocomplete.
- Collections: list, contents (paginated), and write CRUD (create/delete, add/remove screen/flow/app).
- Workspaces list (required to create a collection).
- Auth: browser/Chrome cookie import + Supabase token refresh; multi-profile.
- Full-res image download from the Bytescale CDN.

## Data Layer
- **Primary entities:** `apps` (id, appName, platform, appCategories, thumbnailUrl, created/updatedAt), `screens` (id, appId, appName, platform, screenName, imageUrl, patterns[], elements[]), `flows` (id, appId, appName, flowName, platform, steps[]), `patterns` / `elements` (id, slug, name, definition, count), `collections` (id, name, description, workspaceId, createdAt), plus reference dictionaries (categories, flow-actions) and `workspaces`.
- **Sync cursor:** `updatedAt` per entity; full re-list when filters change. Snapshots are timestamped so `drift`/`audit` can diff across syncs.
- **FTS/search:** FTS5 across screens (name + pattern/element names + OCR keywords), flows (name + app + actions), apps (name + categories). `sql`/`search` return empty until `sync` populates the store.

## Reachability Risk
- **Auth shape:** cookie / Supabase session. Two split-JWT cookies (`sb-ujasntkfphywizsdaapi-auth-token.0` / `.1`) against Supabase project `ujasntkfphywizsdaapi`. No public API key exists. `press-auth` / `auth login --chrome` imports the logged-in browser cookies; access token is short-lived (~1h), refreshed via `/auth/v1/token?grant_type=refresh_token`; captured session good ~24h before re-export.
- **Public (no auth):** `apps list/popular/discover`, `filters list`, all `trending`, `sites list`. These work for smoke-tests and free-tier use.
- **Needs Mobbin Pro session:** `apps/screens/flows search`, `autocomplete` (full quality), all `collections`, `workspaces`. Free tier returns limited/preview content (latest few apps).
- **Bot-protection risk:** Low today — no Cloudflare/captcha 403s reported across wrapper repos; internal routes respond. Medium-term tightening plausible now that Mobbin ships a first-party paid MCP (`api.mobbin.com/mcp`, OAuth) and has publicly deprecated the leading reverse-engineered wrapper; internal-route access may be restricted over time. Writes hit Supabase PostgREST directly (Bearer + apikey).

## User Vision
The operator is reprinting to replace a stale unmerged library PR (#574) that used Printing Press v4.6.0, below the current minimum. Goal: clean regeneration under the current machine (v4.27.1) preserving the 6 prior novel features (deck, bench, audit, drift, grab, cross). Original author/creator is Darin Kishore.

## Product Thesis
- **Name:** `mobbin-pp-cli` (binary `mobbin-pp-cli`; slug `mobbin`)
- **Why it should exist:** Every alternative is either (a) Mobbin's own MCP — paid-only, agent-only, one tool, no terminal surface; (b) reverse-engineered MCP servers with no local store, no batch downloads, no offline search — and the leading one (pdcolandrea) is now archived/deprecated; or (c) closed-source Chrome extensions. Nobody ships a CLI that offline-searches every synced screen, exports design-crit decks in one command, tracks version drift across apps, does write-side collection CRUD, and coexists with Mobbin's official MCP. That is the gap.

## Build Priorities
1. **Spec-source fixed** — internal YAML spec is the source of truth (no re-sniff). `auth.type: cookie`, split Supabase JWT, Chrome import.
2. **Absorbed surface** — match/beat every competing tool: apps/screens/flows search, filters/trending/sites/autocomplete, collections read + write CRUD, workspaces, per-app browsing — all with `--json`/`--select`/`--agent`.
3. **Local store** — SQLite mirror of apps/screens/flows/patterns/elements/collections; FTS5 cross-entity search; timestamped snapshots for drift/audit.
4. **Novel features (preserve the prior 6):** `deck` (crit-deck zip export), `bench` (offline pattern leaderboard), `audit` (time-windowed flow audit), `drift` (version drift across snapshots), `grab` (batch full-res Bytescale download with filename templating + manifest), `cross` (cross-platform parity join). Plus first-class `--save-images` on every screen-emitting command.

## Reachability Gate
- Decision: PASS
- Evidence: GET https://mobbin.com/api/searchable-apps/web -> HTTP 200, real JSON apps array (public no_auth endpoint).
