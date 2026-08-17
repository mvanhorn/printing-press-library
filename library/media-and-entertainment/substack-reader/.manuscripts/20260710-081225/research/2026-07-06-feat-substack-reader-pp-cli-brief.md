# Substack Reader CLI — research brief (Phase 1)

## API Identity
- **Domain:** newsletter/publishing. Every author is their own publication at `<pub>.substack.com` or a custom domain. No official public API — a stable internal `/api/v1/` surface the web app uses.
- **Users:** readers who follow multiple newsletters and want durable, searchable, offline copies; agents that need a post's full text; researchers studying newsletters.
- **Data profile:** posts (metadata + `body_html`), publications, categories, comments, podcast/RSS. Free (`audience: everyone`) vs paid (`only_paid`/`founding`).

## Reachability Risk
- **None/Low.** Internal `/api/v1/` endpoints reachable anonymously, keyless, no Cloudflare challenge (verified). Only throttle is 429 (self-throttle). Historic risk: sitemap.xml removed (use `/api/v1/archive` instead); custom-domain redirect breaks naive cookie auth.
- Probe-safe endpoint used: `GET /api/v1/categories` (health check).

## Top Workflows
1. **Archive a publication** into a local corpus (fetch `/api/v1/archive` paginated). ✅ built.
2. **Read a post's full text** — free keyless; paid via own credential (`/api/v1/posts/{slug}`).
3. **Search** across archived posts offline (FTS over the local corpus).
4. **Digest** — what's new across my newsletters since last sync.
5. **Compare** two publications' cadence/topics/free-paid mix.

## Table Stakes (from competitors)
- List a publication's posts; fetch a single post's content; per-publication search; multi-format export; rate-limited archiving; paid-content access via the user's own session.

## Data Layer
- **Primary entity:** `posts` (keyed by numeric id; store the full archive/post JSON). Also publications, categories.
- **Sync cursor:** offset-based per publication.
- **FTS/search:** generic `resources_fts` over stored posts (works offline; verified).

## Codebase Intelligence
- Endpoint layer modeled on NHagar/substack_api. Auth for paid = cookie (`substack.sid`[+`connect.sid`], `.substack.com`-scoped) or private RSS `/feed/private/<token>`.

## Product Thesis
- **Name:** Substack Reader (binary `substack-pp-cli`; if published, consider `substack-reader` to avoid the existing account-mgmt `substack` CLI).
- **Why it should exist:** every existing tool fetches live per call. None builds a **local, full-text-searchable, SQL-queryable corpus that compounds** — the Medium Reader pattern, applied to Substack, with entitlement-bound paid access.

## Build Priorities
1. ✅ Keyless `archive` → SQLite → offline search (foundation, proven).
2. `read` (free keyless + Tier-1 paid), `digest`, `author-compare`; make framework `search`/`sql` real.
3. Tier-1 auth (cookie header + `.substack.com`-host redirect policy; and/or private RSS) — needs the attended crack.
4. Absorb-gate → shipcheck → dogfood → publish decision.
