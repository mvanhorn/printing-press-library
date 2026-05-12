# Kickstarter CLI Absorb Manifest

## Tools surveyed
| Source | Type | URL | Coverage |
|--------|------|-----|----------|
| markolson/kickscraper | Ruby gem | github.com/markolson/kickscraper | Most mature unofficial wrapper. Auth via email/password (broken on modern KS), public search via `/discover/advanced?format=json` |
| rabidlogic/PyKickstarter | Python lib | github.com/rabidlogic/PyKickstarter | Project + Reward + Category models |
| mikeminutillo/Kickstarter.Api | C# unofficial | github.com/mikeminutillo/Kickstarter.Api | Unofficial REST client |
| gippy/kickstarter-search | Apify Actor | github.com/gippy/kickstarter-search | Search JSON via Chrome TLS impersonation |
| qbie/apify scraper | Commercial actor | apify.com/qbie/kickstarter-scraper | Paid, broad coverage |
| njday/Kickstarter-Scraper | Python script | github.com/njday/Kickstarter-Scraper | URL-list to CSV |
| sparkfun/KickScraper | Project mgmt | github.com/sparkfun/KickScraper | Creator-side backer messaging (out of scope for us) |
| webrobots.io datasets | Monthly dataset | webrobots.io/kickstarter-datasets/ | Bulk historical (we won't replicate; we'll link in README) |

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Search projects by keyword | kickscraper search | `kickstarter search "<query>"` via `/discover/advanced?term=&format=json` over Surf | Local FTS after sync, JSONL output, offline replay |
| 2 | Filter by category | kickscraper categories | `--category technology` resolves slug → `category_id` | Slug resolution table baked in (15 main + 36 sub), no API call to look up |
| 3 | Filter by subcategory | kickscraper categories | `--subcategory ai` (where supported) | Same |
| 4 | Sort by newest/popular/most-funded/end-date | KS Discover | `--sort newest\|magic\|popularity\|most_funded\|most_backed\|end_date` | All sort modes exposed as one flag |
| 5 | Filter by status (live/successful/failed/cancelled) | KS Discover | `--status live\|successful\|failed\|cancelled` | Same |
| 6 | Filter by location | KS Advanced | `--location <woeid>` | Optional |
| 7 | Paginate results | KS Discover | `--page N --limit M` with auto-sync | Sync mode walks pages until exhausted |
| 8 | Fetch project detail by slug | kickscraper Project.find | `kickstarter project get <creator>/<slug>` | Optional `--json --select` field pruning |
| 9 | List rewards on a project | kickscraper Reward | `kickstarter project rewards <slug>` | JSON-native |
| 10 | List project updates | kickscraper Update | `kickstarter project updates <slug>` | Synced into local store |
| 11 | List categories + subcategories | kickscraper Category | `kickstarter categories list` | Offline from baked-in table |
| 12 | oEmbed lookup | KS oEmbed service | `kickstarter project embed <url>` | Lightweight project metadata, no Surf needed |
| 13 | Get creator profile | kickscraper User | `kickstarter creator get <username>` | Synced into `creators` table |
| 14 | Most-funded all time | KS Discover sort | `kickstarter funded --top --all-time` | Cross-page sync |
| 15 | Trending now | KS Discover sort | `kickstarter trending` | Composite (magic + funding velocity) |
| 16 | Project search by tag | KS Advanced tags | `--tag <slug>` | Native to advanced search |
| 17 | Export results as CSV | kickscraper export | `--csv` flag on any list command | Generator emits via flags.printJSON path |
| 18 | Filter by funding goal range | KS Advanced filters | `--goal-min N --goal-max N` | Surfaces as cli flags |
| 19 | Filter by pledged-amount range | KS Advanced filters | `--pledged-min N --pledged-max N` | Same |
| 20 | Magazine article listing | (nobody ships this) | `kickstarter magazine list` via HTML scrape of `/magazine` | **First tool to cover Magazine programmatically** |
| 21 | Magazine article detail | (nobody ships this) | `kickstarter magazine get <slug>` | Full body text extracted via Surf + structured parser |

Every row above is shipping scope. No stubs.

## Transcendence (only possible with our approach)

| # | Feature | Command | Why Only We Can Do This |
|---|---------|---------|------------------------|
| 1 | Unified "latest news" roll-up | `kickstarter latest-news [--vertical <slug>] [--limit N] [--json]` | Fans out across Discover (new launches), Trending, AND Magazine in one invocation — returns a single ranked JSONL stream Scout can consume. No competing tool does multi-surface aggregation. |
| 2 | Tech-radar | `kickstarter tech-radar [--subcategory ai\|hardware\|software\|robots] [--days 7]` | Time-windowed query against synced data; "Technology launches in the last 7 days ranked by funding-velocity" requires local store. KS API can't compute funding velocity over a time window. |
| 3 | Funding-rank | `kickstarter funding-rank --window 24h` | % funded per hour-since-launch; only possible with periodic sync snapshots in SQLite. |
| 4 | Vertical mapping | `kickstarter vertical <vertical-slug> [--score-threshold 0.6]` | Per-project keyword scoring against Scout's verticals (ai-agents, frontier-ai, smb-saas, geopolitics, aus-tech, india-tech). Stored in `vertical_match` table. Compound query no API exposes. |
| 5 | Creator portfolio | `kickstarter creator portfolio <username>` | Every project a creator ever launched + cross-project pattern detection (serial launchers, abandoned projects). Requires sync. |
| 6 | Category-velocity | `kickstarter category velocity [--window 7d]` | Which categories are launching fastest this week. Aggregate over launched_at column. |
| 7 | Magazine FTS search | `kickstarter magazine search "<query>"` | Offline full-text search over scraped Magazine bodies. Nobody else mirrors Magazine, let alone indexes it. |
| 8 | Diff-since-last-sync | `kickstarter sync --since-last [--diff]` | Show only projects that appeared/changed since last sync — agent-native delta surface for periodic Scout runs. |
| 9 | JSONL stream for Scout | `kickstarter latest-news --jsonl --vertical ai-agents` | Pure JSONL output (one object per line) sized for direct ingestion into Scout's signal pipeline. No human-table mode polluting the stream. |

## Stubs
None. Every row is shipping scope.
