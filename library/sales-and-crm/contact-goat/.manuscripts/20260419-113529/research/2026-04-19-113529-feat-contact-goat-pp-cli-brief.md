# contact-goat CLI Brief

## API Identity
- Domain: business contact search and enrichment (prospecting, warm-intro mapping, recruiting, cold discovery)
- Users: founders, salespeople, recruiters, fundraisers, investors
- Data profile: people + companies + connection edges + search results; network graph is the moat

## Reachability Risk
- Low for LinkedIn (0 ban reports in last 30 days on stickerdaniel MCP, Patchright actively maintained)
- Low for Happenstance (sniff target is their own web app the user is logged into)
- Low for Deepline (clean REST, well-documented, predictable 401 shape)

## Top Workflows
1. Prospecting: search_people by title+location+industry, filter to 2nd-degree, enrich with email via Deepline, stash in local store
2. Warm-intro mapping: given a target person/company, walk LinkedIn 1st-degree + Happenstance network graph to find mutuals who can intro
3. Recruiting: search_jobs/search_people filtered by target company, cross-reference with Happenstance for "people I already know there"
4. Company research: get_company_profile + get_company_posts + search_people@company, snapshot to local for diff
5. Cold discovery escape hatch: when LinkedIn/network returns empty, deepline search net-new contacts with waterfall email enrichment

## Table Stakes
- Every feature stickerdaniel MCP has: person/company profiles, people/job search, messaging, sidebar profiles
- Happenstance: network search, person research dossier
- Deepline: person search, email waterfall, company search, credit balance/cost pre-flight
- --json everywhere, --dry-run on any write or paid call, typed exit codes, offline search via SQLite FTS

## Data Layer
- Primary entities: person (publicIdentifier + urn), company (slug + urn), job, conversation, message, connection edge, search_result_set, happenstance_network_edge, deepline_enrichment_log
- Sync cursor: per-source timestamp; LinkedIn queues sequential per MCP, Happenstance polls async (30-60s searches), Deepline is synchronous
- FTS/search: FTS5 on person.name/title/company, company.name/description, job.title/company, message.body

## Source Priority
- Primary: LinkedIn - wrapper-library (stickerdaniel/linkedin-mcp-server, Python MCP subprocess via Patchright) - auth: browser-session, free
- Secondary: Happenstance - web app sniff (cookie session auth, free tier via logged-in Chrome) - reference: developer.happenstance.ai/openapi.json for endpoint shape
- Tertiary: Deepline - docs-based REST (code.deepline.com/api/v2), Bearer dlp_ key - auth: paid (credit-based, cost pre-flight mandatory)
- Economics: LinkedIn+Happenstance free with user's existing sessions. Deepline is paid; --deepline flag gates those commands; every Deepline call surfaces estimated cost before execution.
- Inversion risk: Happenstance has the cleanest OpenAPI of the three, but LinkedIn is the headline. Do NOT let spec completeness invert ordering; LinkedIn commands lead the README.

## User Vision
- "Super LinkedIn": LinkedIn search with my auth like the MCP, Happenstance to go deeper with my social network outside LinkedIn limitations, Deepline if I want to pay money for contacts
- Key insight: browser session does the heavy lifting for the free path; paid API is an escape hatch, not the main event

## Product Thesis
- Name: contact-goat
- Why it should exist: No existing tool combines LinkedIn's authoritative current-employment graph with Happenstance's cross-source network graph with Deepline's paid cold-discovery. Current alternative is tab-juggling across three web apps plus manual CSV export. A CLI that unifies into one SQLite store enables compound queries (warm-intro paths, network-relative prospecting, budgeted enrichment) nobody has.

## Build Priorities
1. Priority 0: SQLite schema + sync commands for person/company/search-result (foundation for everything)
2. Priority 1 (LinkedIn): all 13 stickerdaniel MCP tools via Go MCP-client subprocess
3. Priority 1 (Happenstance): search-network, research-person via cookie-auth sniffed endpoints
4. Priority 1 (Deepline): top 10-15 tools via docs-derived wrappers + generic `execute` passthrough
5. Priority 2 (transcend): warm-intro graph (LI 1st-degree UNION Happenstance network), find-mutuals, prospect budget (Deepline cost aggregator), network-relative search (Happenstance-ranked then LinkedIn-enriched), company-coverage (who I already know at X)
