# eRank CLI Absorb Manifest

## Scope
- Shipping target: eRank Keyword Tool and Top Listings workflow captured from the authenticated member page.
- Spec source: browser-sniffed HAR from `members.erank.com/keyword-tool/top-listings`.
- Out of scope for this run: full eRank platform clone (Rank Checker, Traffic Stats, Listing Audits, Competitor Sales, Sales Map, Delivery Status). These require separate captures and are not approved shipping scope here.

## Absorbed

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Keyword statistics for marketplace/country | eRank Keyword Tool | erank-pp-cli keyword-tool list-stats --keyword "dad mug" --marketplace etsy --country USA | JSON/CSV export, reusable scripts, local snapshot source |
| 2 | Top listings for a keyword | eRank Top Listings tab | erank-pp-cli keyword-tool list-top-listings --keyword "dad mug" --marketplace etsy --country USA | Agent-readable top-listing rows with local history |
| 3 | Related searches | eRank Keyword Tool | erank-pp-cli keyword-tool list-related-searches --keyword "dad mug" --marketplace etsy --country USA | Query expansion without browser UI |
| 4 | Near-match keyword ideas | eRank Keyword Tool | erank-pp-cli keyword-tool list-near-matches --keyword "dad mug" --marketplace etsy --country USA | Long-tail discovery in structured output |
| 5 | Etsy tag data | eRank Keyword Tool | erank-pp-cli keyword-tool list-etsy-tags --keyword "dad mug" --marketplace etsy --country USA | Tag mining directly usable in listing drafts |
| 6 | Competition analysis | eRank Keyword Tool XHR | erank-pp-cli keyword-tool create-competition --keyword "dad mug" --marketplace etsy --country USA --dry-run | POST body surfaced with dry-run safety |
| 7 | Google/search trend data | eRank Keyword Tool XHR | erank-pp-cli keyword-tool create-google-data --keyword "dad mug" --country USA --dry-run | Scriptable trend enrichment |
| 8 | Keyword difficulty | eRank Keyword Tool XHR | erank-pp-cli keyword-tool create-keyword-difficulty --keyword "dad mug" --marketplace etsy --country USA --dry-run | Difficulty signal available outside UI |
| 9 | Save keyword lookup history | eRank Keyword Tool XHR | erank-pp-cli keyword-tool create-save-history --keyword "dad mug" --marketplace etsy --country USA --dry-run | Mutation guarded by dry-run |
| 10 | Keyword list names | eRank Keyword Lists | erank-pp-cli keywordlist list-names | List inventory for agents |
| 11 | Keyword list terms | eRank Keyword Lists | erank-pp-cli keywordlist list-terms | Batch analysis input source |
| 12 | Daily quota | eRank quota endpoint | erank-pp-cli quota list-daily | Agents can avoid burning limited lookups blindly |

## Transcendence

| # | Feature | Command | Buildability | Score | Why Only We Can Do This |
|---|---------|---------|--------------|-------|--------------------------|
| 1 | Niche Opportunity Score | opportunity | hand-code | 9/10 | Joins stats, competition, difficulty, related searches, near matches, and top listings into one seller-ready decision score. |
| 2 | Top Listing Gap Analyzer | listing gaps | hand-code | 9/10 | Compares a seller draft title/tags against tags and phrases proven in current top listings. |
| 3 | Tag Consensus Map | tags consensus | hand-code | 8/10 | Builds a consensus graph from top-listing tags, eRank Etsy tags, related searches, and near matches. |
| 4 | Keyword Drift Watcher | watch drift | hand-code | 8/10 | Stores repeated keyword snapshots locally, then detects competition, difficulty, and top-listing changes over time. |
| 5 | Keyword Portfolio Optimizer | lists optimize | hand-code | 8/10 | Uses saved keyword lists plus live stats to flag cannibalized, saturated, weak, and missing keywords across a seller research portfolio. |
| 6 | Saturation Warning | saturation | hand-code | 7/10 | Combines competition, difficulty, tag reuse, and top-listing density into a practical crowded-market warning. |
| 7 | POD Angle Finder | angles | hand-code | 7/10 | Turns related searches, near matches, tags, and top listings into evidence-backed product angles without inventing unsupported demand. |

## Stub List
- None approved. If a captured endpoint cannot generate or replay, return to this gate before downgrading it.

## Risks
- Browser-sniffed spec inferred `api_key` auth incorrectly from query parameters; pre-generation auth enrichment must replace this with cookie/session auth.
- `traffic-analysis.json` marks `browser_http_transport` and `requires_protected_client`.
- Some POST schemas have weak confidence because the capture had one keyword workflow.
- eRank quotas are real; live dogfood must use a small matrix and respect `quota list-daily`.
