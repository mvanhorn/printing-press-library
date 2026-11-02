# Government Data CLI Brief
## API Identity
Catalog metadata discovery across data.ca.gov and data.gov. Actual files remain with publishing agencies. Read-only scope: search, inspect metadata and resources, access downloads.
## Users
Evidence-backed target personas (workflow inferences, not interviews): a California water-policy analyst repeatedly finding county and state water data for weekly reporting; a civic-data journalist revisiting source datasets and checking whether links or metadata changed; a research data engineer maintaining repeatable dataset discovery and download pipelines.
Evidence: https://data.gov/user-guide/ describes catalog search/filter/detail workflows; https://resources.data.gov/resources/data-gov-open-data-howto/ explains agency-hosted data and harvested metadata; California developer docs expose search, detail, organizations and datastore queries. ckanapi supports paginated metadata export. Community dataset-discovery complaints describe repeated multi-tab searches and broken download links.
## Top Workflows
1. Weekly topic sweep: query both catalogs, filter publisher and format, shortlist resources with provenance.
2. Weekly source refresh: revisit known datasets, compare metadata and resource URLs with the previous saved state.
3. Repeatable research intake: inspect license/update dates and resource formats before downloading selected files.
4. Reporter verification: identify identical agency download URLs listed in both catalogs without claiming distinct datasets are identical.
## Reachability Risk
California documented CKAN search succeeds anonymously. Old catalog.data.gov/api/3/action/package_search returns 404; older MCP clients hardcode it. Current federal catalog OpenAPI at https://catalog.data.gov/openapi.json and official v4 docs at https://resources.data.gov/catalog-api/ . Official gateway https://api.gsa.gov/technology/datagov/v4/search succeeds with public DEMO_KEY. User approved DEMO_KEY for initial tests. Personal keys for automated production querying; 30/hour, 50/day demo budget. California docs: 60 anonymous requests/minute.
## Table Stakes
ckanapi: arbitrary actions, search, pagination, metadata export, pipelines, timeouts; administrative writes excluded by user read-only scope.
melaodoidao/datagov-mcp-server: package_search, package_show, group_list, tag_list, resource URL access. Source confirms legacy CKAN URL; replace with current federal equivalents (keywords/organizations) rather than ship broken commands.
## Data Layer
Local datasets keyed by catalog plus id; resource URLs, publisher, title, description, formats, license, modified timestamps, raw metadata, fetch time. Bounded topic search sync; never imply a query cache is the full government catalog. SQLite FTS for saved records. Federal opaque after cursor; California rows/start offsets.
## Pain Points
Different API envelopes/cursors, duplicate catalog listings of one agency file, metadata vs actual-file distinction, stale/broken links, reproducibility of source selection. Avoid interpreting metadata recency as underlying measurement recency.
## User Vision
Equal prominence for both catalogs. Search, metadata inspection and download access. Documented APIs. User approved temporary California capture; actual Chrome DevTools export contains search and detail navigation, 35 requests, 33 bodies. Sniff yielded 7 routes, including unstable Next.js build-specific routes: treat these as evidence only; use stable CKAN APIs for runtime.
## Source Priority
Peers: data-ca-gov and data-gov. Same top-level workflow supports either or both. Federal credential applies only to federal gateway; California stays unauthenticated. No inversion.
## Product Thesis
Name: Government Data. One repeatable intake workflow across California and national catalog metadata with explicit provenance and source-specific errors.
## Build Priorities
Unified bounded search; inspect dataset and resources; safe explicit download destination; publisher/keyword discovery; local saved search data with reproducible output.
## Sources
https://data.ca.gov/docs/developer-documentation
https://resources.data.gov/catalog-api/
https://catalog.data.gov/openapi.json
https://github.com/ckan/ckanapi/blob/master/README.md
https://github.com/melaodoidao/datagov-mcp-server/blob/main/src/index.ts
Wrapper issues inspected: pkg_resources deprecation and search gzip issue #167; no evidence these block current public reads. No useful DeepWiki analysis extracted.

## SEC Scope Revision
# SEC EDGAR requested scope addition (research draft)
User explicitly requested direct SEC EDGAR search, filings and company financial facts. The original California and Data.gov catalogs retain equal prominence; SEC is a separate direct source under edgar commands.

## Official contract evidence
- https://www.sec.gov/search-filings/edgar-application-programming-interfaces : public read APIs require no authentication or API key.
- Submissions: GET https://data.sec.gov/submissions/CIK##########.json . Ten-digit zero-padded CIK. Recent filings column arrays plus filings.files for older history. Do not claim recent array is complete history.
- Company facts: GET https://data.sec.gov/api/xbrl/companyfacts/CIK##########.json . Standard taxonomy entity-wide facts; preserve taxonomy, concept, units, period, form, filed date, accession and amendments. No invented canonical/latest financial statement aggregation.
- Company concept: GET https://data.sec.gov/api/xbrl/companyconcept/CIK##########/{taxonomy}/{concept}.json . Fact arrays separated by units.
- Filing documents and indexes: https://www.sec.gov/search-filings/edgar-search-assistance/accessing-edgar-data . Archive documents free to download. Exact accession and document names must come from upstream evidence.
- Fair access: maximum 10 requests/second across machines, descriptive User-Agent with contact email. Default conservative pacing and cache; contact requested from user, do not invent one or reuse unrelated account information.

## Proposed baseline additions
1. Company lookup by ticker or CIK with explicit resolution.
2. Filing search filtered by company, form and date, traversing older submission files within bounded limits.
3. Filing metadata, document listing and explicit selected document download.
4. Company XBRL facts and concept selection with units/period/filing provenance.
5. Full-text filing search: research SEC search surface separately before promise; public data.sec.gov APIs alone do not supply full-text search. No substitution with a paid third-party API.

## Pending
User-Agent contact before live SEC requests; official source reachability; current full-text search contract and browser discovery gate; revised absorb manifest and approval. Prior 14-capability plan has not been approved and generation has not started.

## Live read-only proof
Company submissions, companyconcept Assets, companyfacts and EFTS full-text search all returned HTTP 200 with expected JSON structures. Real user-authorized contact supplied in request header only. Config env: SEC_USER_AGENT.

Chrome DevTools EFTS capture succeeded: six GETs with HTTP 200 and usable bodies, including q/startdt/enddt full-text search. SEC under edgar command family; original catalogs remain peers. SEC_USER_AGENT configurable, no personal contact embedded.
