# Substack CLI Brief

## API Identity
- **Domain:** Newsletter / Publishing platform (writers, creators, paid subscriptions, Notes social layer)
- **Users:** Newsletter creators (publish + analytics), readers (consume + restack), multi-publication owners (this user has 3 publications: MacroView, MakroSicht, JBP Capital Premium Invest)
- **Data profile:** Posts (drafts + published), Notes (microblog/social), Comments, Subscribers (free/paid), Publications, Sections, Reactions, Restacks, Profiles, Recommendations
- **Official API status:** Substack has only a tiny official "Developer API" for LinkedIn-handle lookups. The full creator/reader API is **internal, undocumented, reverse-engineered** and authenticated via the `connect.sid` / `substack.sid` cookie from a logged-in browser session. Substack support page exists but offers almost nothing actionable.

## Reachability Risk
- **Low–Medium.** Multiple production tools (jakub-k-slys/substack-api 4.0.0, postcli/substack, ty13r/substack-mcp-plus, sbstck-dl) hit Substack's internal API actively in 2026 and are still being released. Substack does not aggressively block community tooling at reasonable rates.
- **Known fragility:** Post scheduling endpoint changed (404 in ty13r v1.0.3). Internal endpoints can break without notice. Mitigation: keep auth/transport thin, surface upstream failures clearly.
- **Auth complication:** The session cookie expires; users must occasionally re-extract from Chrome.

## Top Workflows (this user specifically)
1. **Manage 3 publications from one terminal** — list/draft/publish/schedule posts across MacroView, MakroSicht, JBP Capital Premium Invest without context-switching the web UI.
2. **Subscriber analytics + churn watch** — see who subscribes/unsubscribes, free→paid conversions, top engagers per publication, weekly delta.
3. **Cross-publication content reuse** — find which post performed best, duplicate/repurpose German↔English (MacroView ↔ MakroSicht), cross-link.
4. **Notes engagement loop** — publish Notes, track restacks/replies, auto-react/restack handlers.
5. **Offline archive of own content** — markdown export of all posts (with images), full-text search across years of writing.

## Table Stakes (every competing tool has these — we match all)
- **Posts:** list/get/create draft/update/publish/delete/duplicate/list-published/list-drafts
- **Notes:** list/publish/reply/react/restack
- **Comments:** list/add/react
- **Profile:** own profile / get other / follow-list
- **Feed:** for-you / following / categories
- **Subscribers:** list/count (per publication)
- **Sections:** list (publication categories)
- **Images:** upload to Substack CDN
- **Auth:** Chrome cookie grab, manual cookie paste, email OTP
- **Download/archive:** posts → markdown/HTML, images, file attachments, date filter
- **Output:** `--json` for piping

## Data Layer (SQLite, offline)
- **Primary entities:** publications, posts (with body_html, body_markdown, word_count, paywalled, scheduled_at), drafts, notes, comments, subscribers, sections, reactions, restacks, profiles, follows, recommendations
- **Sync cursor:** per-publication post_id cursor + per-feed Notes cursor; lastSynced timestamps per resource
- **FTS5/search:** full-text index over post titles + bodies + Notes + comments; supports phrase, AND/OR, and per-publication scoping
- **Cross-publication joins:** the killer move — most existing tools are single-pub. SQLite makes "best post by engagement across all my pubs" a one-liner.

## Codebase Intelligence (from research, no DeepWiki call needed yet — well-documented community)
- **Source ground-truth:** read `postcli/substack` (TS/Node, 16 MCP tools) and `jakub-k-slys/substack-api` (TS, entity-based, v4.0.0) for endpoint + auth patterns
- **Auth (universal):** `Cookie: connect.sid=<value>; substack.sid=<value>` against `<subdomain>.substack.com/api/v1/...` or `substack.com/api/v1/...` for cross-pub
- **Critical endpoints (from reverse-engineering articles):**
  - `GET  /api/v1/profile` — own profile
  - `GET  /api/v1/notes?cursor=…` — notes feed (paginated)
  - `GET  /api/v1/posts/` — posts list
  - `GET  /api/v1/posts/<slug>` — single post
  - `GET  /api/v1/comments/<post_id>` — comments
  - `POST /api/v1/comment/feed` — add comment
  - `POST /api/v1/reaction` — react (post/note/comment)
  - `POST /api/v1/comment/<id>/restack` / `POST /api/v1/notes/<id>/restack`
  - `GET  /api/v1/publication/search?query=…`
  - `POST /api/v1/subscriber/add`
  - `GET  /api/v1/subscriber/list` (auth-gated, per-publication)
  - `POST /api/v1/drafts` / `PUT /api/v1/drafts/<id>` / `POST /api/v1/drafts/<id>/publish`
  - `POST /api/v1/image` (CDN upload)
  - `GET  /api/v1/sections`
- **Rate limiting:** community tools default to ~2 req/s; no documented hard limit, but we ship adaptive limiter to be safe.

## User Vision (from briefing)
- User owns 3 publications and wants a unified terminal workflow. Browser session is available — authenticated sniff is the right path. No upfront feature list; the absorb manifest plus a 3-publication multi-tenant model is the headline differentiator.

## Product Thesis
- **Name:** `substack-pp-cli`
- **Tagline:** *Every Substack feature, plus a local SQLite database, full-text search, and cross-publication insights no other Substack tool has.*
- **Why it should exist:** Every existing tool is single-publication or read-only or MCP-only. None aggregate across publications, none ship an offline SQLite layer with FTS, none give creators with multiple newsletters a single workspace. This user (3 publications, German + English) is the canonical target.

## Competitor Landscape (sources for the absorb manifest)
| Tool | Lang | Mode | Key Strength | Stars/Status |
|------|------|------|-------------|--------------|
| postcli/substack | Node | CLI + MCP + TUI | 16 MCP tools, automations, chrome cookie auth | Active |
| ty13r/substack-mcp-plus | Python | MCP | 12 tools, rich text, drafts/publish | Active |
| jakub-k-slys/substack-api | TS | SDK | v4.0.0, entity-based, fluent API | 72★ |
| NHagar/substack_api | Python | SDK + CLI | Newsletter/Post/User, categories, search | Active |
| alexferrari88/sbstck-dl | **Go** | CLI | Download/archive, MD/HTML, images/files, date filter, rate limit | Active |
| alvarolorentedev/substack-cli | Node | CLI | Subscriber CSV/JSON export | Smaller |
| anshulkhare7/substack-cli | — | CLI | Drafts + publish + subscribers | Smaller |
| marcomoauro/substack-mcp | Node | MCP | Posts/drafts | Smaller |
| michalnaka/mcp-substack | — | MCP | Download/parse posts for Claude Desktop | Smaller |
| dkyazzentwatwa/substack_mcp | — | MCP | Real-time content analysis | Smaller |

## Build Priorities (will be expanded into absorb manifest in Phase 1.5)
1. Data layer + sync for all primary entities, multi-publication aware
2. Match every command/tool from postcli/substack + ty13r/substack-mcp-plus + sbstck-dl
3. Transcendence features: cross-publication analytics, churn watch, drift detection, content reuse, multi-pub Notes scheduling, FTS over years of writing

## Open Questions for Phase 1.7 / Phase 1.6
- Confirm `AUTH_SESSION_AVAILABLE=true` (user confirmed: browser-logged-in to substack.com with 3 publications)
- Phase 1.7 will sniff the authenticated session against `substack.com` to capture publication list, subscriber endpoints, draft endpoints, scheduling endpoints (volatile per ty13r 1.0.3 note)
