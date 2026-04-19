# contact-goat Absorb Manifest

## Context

Combo CLI: LinkedIn (primary, MCP subprocess) + Happenstance (secondary, cookie-auth sniff) + Deepline (tertiary, docs-based REST).

v1 shipping scope is a curated subset - full absorb happens over versions. All shipping-scope items below build fully; items explicitly marked `(stub)` ship as placeholders with honest "not yet wired" messaging.

## Absorbed - LinkedIn (via stickerdaniel/linkedin-mcp-server Python subprocess)

| # | Feature | Best Source | Our Implementation | Added Value | Status |
|---|---------|-------------|-------------------|-------------|--------|
| 1 | Search people by keywords + location | MCP search_people | linkedin search-people via Go MCP client | --json, --limit, SQLite cache, offline FTS5 re-search | shipping |
| 2 | Search jobs by keywords + location | MCP search_jobs | linkedin search-jobs | Daily diff against cache, job-closure detection | shipping |
| 3 | Get person profile with section selection | MCP get_person_profile | linkedin get-person | --sections flag, local cache, age TTL | shipping |
| 4 | Get company profile | MCP get_company_profile | linkedin get-company | --sections flag (posts, jobs), cache | shipping |
| 5 | Get messaging inbox | MCP get_inbox | linkedin inbox | Recent conversations, SQLite cache | shipping |
| 6 | Search people with filters (industry, school, past_company, network_depth) | tomquirk/linkedin-api Voyager endpoints | linkedin search-people --filters | Filter coverage the MCP lacks (tradeoff: higher ban risk, requires user opt-in via --voyager) | shipping |
| 7 | Sidebar profile extraction | MCP get_sidebar_profiles | linkedin sidebar <person-url> | Useful for warm-intro discovery | (stub - v2) |
| 8 | Connection request send/accept | MCP connect_with_person | linkedin connect | Blocked by MCP issue #365 (false-positive success); stub with link to upstream issue | (stub - blocked upstream) |
| 9 | Get conversation by user/thread | MCP get_conversation | linkedin conversation | Blocked by MCP issue #307 (multi-thread bug); stub | (stub - blocked upstream) |
| 10 | Keyword search messages | MCP search_conversations | linkedin search-messages | shipping |
| 11 | Send DM | MCP send_message | linkedin send-message | --dry-run mandatory, --confirm prompt | shipping |
| 12 | Get company posts | MCP get_company_posts | linkedin company-posts | shipping |
| 13 | Get job details | MCP get_job_details | linkedin job <id> | shipping |

## Absorbed - Happenstance (web app, cookie-auth via sniff)

| # | Feature | Best Source | Our Implementation | Added Value | Status |
|---|---------|-------------|-------------------|-------------|--------|
| 14 | Get current user | sniff /api/user | happenstance whoami | identity + plan + linkedin URL | shipping |
| 15 | Get usage limits | sniff /api/user/limits | happenstance limits | searches remaining + renewal date, warn when <3 | shipping |
| 16 | List recent searches | sniff /api/dynamo/recent | happenstance searches | SQLite cache, filter by age | shipping |
| 17 | Get search results | sniff /api/dynamo?requestId | happenstance search <id> | --json, parse logs (SEARCH_SCOPE, FILTER), extract result people | shipping |
| 18 | Get search suggested posts | sniff /api/search/{id}/suggested-posts | happenstance search-posts <id> | shipping |
| 19 | List recent research dossiers | sniff /api/research/recent | happenstance research-list | shipping |
| 20 | Research history (paginated) | sniff /api/research/history | happenstance research-history --page N | shipping |
| 21 | List friends (my network) | sniff /api/friends/list | happenstance friends | SQLite cache as my-network graph | shipping |
| 22 | Get feed | sniff /api/feed | happenstance feed --unseen --limit N | shipping |
| 23 | List notifications | sniff /api/notifications | happenstance notifications | shipping |
| 24 | Resolve user by UUID | sniff /api/clerk/referrer | (internal helper for dossier) | used in graph operations | shipping (internal) |
| 25 | Get uploads/integrations status | sniff /api/uploads/status/user | happenstance integrations-status | Which data sources are connected | shipping |

## Absorbed - Deepline (docs-based REST, Bearer dlp_ key)

| # | Feature | Best Source | Our Implementation | Added Value | Status |
|---|---------|-------------|-------------------|-------------|--------|
| 26 | Person search -> email waterfall | Deepline docs | deepline find-email <name> --company X | Pre-flight cost, --max-credits, --dry-run | shipping |
| 27 | Apollo people search | Deepline integrations | deepline search-people --title --location | Pre-flight cost, --limit default 10 | shipping |
| 28 | Email find by domain | Deepline docs | deepline email-find <domain> | shipping |
| 29 | Phone find | Deepline docs | deepline phone-find | shipping |
| 30 | Company search | Deepline docs | deepline search-companies | shipping |
| 31 | Company enrich | Deepline docs | deepline enrich-company <domain> | shipping |
| 32 | Person enrich | Deepline docs | deepline enrich-person <linkedin-url> | shipping |
| 33 | Generic tool execute (escape hatch) | Deepline /api/v2/integrations/{toolId}/execute | deepline execute <toolId> --payload @file.json | passthrough for any tool not in curated list | shipping |
| 34 | Credit balance | inferred (no endpoint in docs) | deepline credits | (stub - needs endpoint discovery via sniff) | (stub - needs endpoint) |

## Transcendence - the combo value prop

| # | Feature | Command | Why Only We Can Do This |
|---|---------|---------|------------------------|
| T1 | Warm-intro path | warm-intro <target> | Requires LinkedIn 1st-degree UNION Happenstance friends, joined by person identity (linkedin URL). No single tool has both graphs. |
| T2 | Company network coverage | coverage <company> | Requires LinkedIn company-employee cache + Happenstance friend affiliations, ranked by my tenure distance. |
| T3 | Cross-source prospect search | prospect "<query>" --budget N | Fan-out across LinkedIn + Happenstance + Deepline (budget-gated), dedup by linkedin_url, rank by network-strength + relevance. Shows Deepline cost pre-flight. |
| T4 | Unified dossier | dossier <person> | Compose LinkedIn profile + Happenstance research + (optional) Deepline enrich. Local cache keyed by linkedin URL; --sections to scope. |
| T5 | Prospect budget | budget | Aggregate Deepline spend this month, recent high-cost calls, --set-limit enforcement persisted in SQLite. |
| T6 | Network intersection | intersect | People in BOTH my LinkedIn 1st-degree AND Happenstance friends - highest-signal warm intros. |
| T7 | Since-last-sync diff | since <duration> | Time-windowed diff across LI connections + Happenstance feed + new research. Requires local snapshots. |
| T8 | Network graph export | graph export --format gexf | Export full cross-source network as GEXF/DOT/JSON for Gephi/Graphviz. Only possible because SQLite has unified entity table. |

Minimum required: 5 transcendence features. Shipping 8 in v1 (T1-T5 as flagship; T6-T8 as shipping-scope but smaller).

## Stubs explicitly approved for v1

The following ship as stubs in v1 with honest "not yet wired" messaging (reasons noted):
- linkedin sidebar (T/S has it working but low priority for v1)
- linkedin connect (blocked upstream - MCP #365)
- linkedin conversation (blocked upstream - MCP #307)
- deepline credits (needs endpoint discovery - either sniff Deepline dashboard or reach out for docs)

## Data Layer (Priority 0)

SQLite tables:
- people (linkedin_id PK, linkedin_url UNIQUE, full_name, title, company, location, last_seen_at, source: set[li, hp, dl])
- companies (slug PK, name, domain, description, size, last_seen_at)
- connection_edges (viewer_user_id, person_id, source, strength INT, first_seen)
- searches_li, searches_hp (local cache of search query + result person_ids + timestamp)
- research_dossiers (research_id PK, person_linkedin_url, content JSON, created_at)
- deepline_log (call_id PK, tool_id, payload_hash, cost_credits, status, timestamp) for budget tracking
- friends_hp (person_id PK, hp_uuid, name, image_url, connection_count)
- feed_items_hp (item_id PK, item_type, created_at, data JSON)
- jobs_li (job_posting_urn PK, title, company, location, posted_at, closed_at)

FTS5 virtual tables: people_fts (name + title + company), companies_fts (name + description), research_fts (content).

## Source attribution

- stickerdaniel/linkedin-mcp-server (1,610 stars, Python, MIT) - primary LinkedIn
- joeyism/linkedin_scraper (3,969 stars, Python) - reference for richer Person dataclass (skills, recommendations, volunteer)
- tomquirk/linkedin-api (Python, unofficial) - reference for Voyager search filters (optional --voyager path)
- happenstance.ai + developer.happenstance.ai - primary Happenstance + fallback for write endpoints
- code.deepline.com/docs - primary Deepline

## Counts

- Absorbed: 34 commands (LinkedIn 13 + Happenstance 12 + Deepline 9)
- Transcendence: 8 features
- Stubs: 4 explicit (connect, conversation, sidebar deferred, credits)
- Plus: auth login/logout/status (cookie-based for LI+HP, API-key for DL), doctor, sync, search (FTS across all three), config
- Total surface: ~48 subcommands

## Priority inversion check

Primary (LinkedIn) commands: 13 absorbed + transcendence features that lead with LI (T1, T2, T4 all join LI)
Secondary (Happenstance) commands: 12 absorbed
Tertiary (Deepline) commands: 9 absorbed

Primary has most commands. NO INVERSION. LinkedIn leads the README.

Economics check: LinkedIn + Happenstance are free via user's existing sessions. Deepline is paid - the --deepline flag gates those commands; no Deepline commands run without explicit opt-in. Every Deepline call surfaces estimated cost.

## Risks and known limitations

1. **Clerk JWT expiry** - __session cookie is ~60s TTL. CLI needs to either refresh via clerk.happenstance.ai or prompt user to re-extract from Chrome. V1 implementation: extract fresh cookies from Chrome on each invocation; if Chrome closed, error with instructions.
2. **LinkedIn MCP requires Python + uvx** - the CLI ships as a Go binary but spawns `uvx linkedin-scraper-mcp` as a subprocess. Adds a runtime dependency. Doctor must check for Python/uvx and surface install instructions.
3. **LinkedIn UI churn** - MCP tools occasionally break when LinkedIn ships HTML changes. CLI should pin to a known-working version and surface when tools fail with detection of upstream issues.
4. **Deepline costs** - every command costs money. CLI defaults --limit 10 and --dry-run flags prominent.
5. **Privacy** - we store LinkedIn profiles and Happenstance friends in local SQLite. `contact-goat wipe` command to nuke local data. No cloud sync.
