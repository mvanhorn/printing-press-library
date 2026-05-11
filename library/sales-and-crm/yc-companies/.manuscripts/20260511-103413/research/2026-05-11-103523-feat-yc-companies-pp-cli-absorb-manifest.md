# YC Companies CLI — Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 1 | List all companies | yc-oss/api `companies/all.json`; Nneji123/ycombinator-scraper | `sync` into SQLite; `companies list` reads locally | Sub-second over 5,889 rows; offline; agent-native JSON |
| 2 | Filter by batch | yc-oss/api `batches/<slug>.json`; dirkjbreeuwer/yc-scraper | `companies list --batch w24` | Composes with --industry/--tag/--status in one query |
| 3 | Filter by industry | yc-oss/api `industries/<slug>.json` | `companies list --industry fintech` | Composes |
| 4 | Filter by tag | yc-oss/api `tags/<slug>.json` | `companies list --tag ai` (CSV multi-tag) | Multi-tag intersect via local query |
| 5 | Filter by status | none (offline-only) | `companies list --status acquired` | Hard via static endpoints |
| 6 | Top-companies view | yc-oss/api `companies/top.json` | `companies list --top` | Composes |
| 7 | Hiring filter | yc-oss/api `companies/hiring.json`; Nneji123 scrape-jobs | `companies list --hiring` | Composes; refreshable via sync |
| 8 | Non-profit filter | yc-oss/api `companies/nonprofit.json` | `companies list --nonprofit` | Composes |
| 9 | Demographic highlight filters | yc-oss/api `companies/{women,black,hispanic}-founded.json` | `companies list --highlight women` | Composes |
| 10 | Get single company by slug | yc-oss/api per-company JSON; live YC page | `companies get <slug>` | One-shot; cached locally |
| 11 | Search by keyword | none of the scrapers had real FTS | `companies search "<query>"` (FTS5 over name+one_liner+long_description+tags+industry) | Sub-second offline FTS |
| 12 | JSON output | every scraper (export) | `--json` everywhere with `--select`/`--compact` | Composable |
| 13 | CSV output | Nneji123, corralm | `--csv` everywhere; `companies export --csv` | Cleaner |
| 14 | Refresh / sync | every scraper has re-fetch | `sync` pulls meta + all.json; snapshots history | Powers history features |
| 15 | List batches | none enumerate as separate cmd | `batches list` | Quick taxonomy enumeration |
| 16 | List industries | none | `industries list` | Quick taxonomy enumeration |
| 17 | List tags | none | `tags list` | Quick taxonomy enumeration |
| 18 | Region filter | data has regions[]; no scraper filters | `companies list --region "United States"` | Cross-axis filter |
| 19 | Team-size filter | data has team_size; no scraper filters | `companies list --min-team-size N --max-team-size N` | Cross-axis filter |
| 20 | Launched-date filter | data has launched_at; no scraper filters | `companies list --launched-after 2024-01-01` | Time slice |
| 21 | Open in browser | some scrapers print URLs | `companies open <slug>` (print by default; `--launch` opens) | One-keystroke; verify-safe |
| 22 | Doctor / health check | none | `doctor` checks meta.json reachability + store integrity | Standard pp-cli surface |

## Transcendence (only possible with our approach)

| # | Feature | Command | Why Only We Can Do This |
|---|---------|---------|--------------------------|
| 1 | Watch list management | `watch add <slug>...` / `watch remove <slug>...` / `watch list` | Local SQLite `watch` table — no online source exposes per-user portfolio state. Primitive that powers `watch diff`. |
| 2 | Watch diff over snapshots | `watch diff [--since <date>]` | Joins two `companies_history` snapshots, filtered to watched slugs; reports team_size / status / isHiring deltas. No API exposes historical state. |
| 3 | New companies since date / last sync | `companies new --since <date>` (or `--since-last-sync`) | Anti-join across snapshots — companies present now and absent at `<date>`. Static endpoints never expose deltas. |
| 4 | Cross-index change feed | `companies changes --field <status\|team_size\|isHiring> [--to <val>] --since <date> [--slugs a,b,c]` | Diff field-by-field between two snapshots; subsumes "hiring-flipped-true" recruiter signals and status-flip detection. |
| 5 | Peer discovery by tags | `companies similar <slug> [--limit N]` | Local Jaccard on tag set + industry match + batch-proximity bonus. No scraper does similarity; no static endpoint exposes the pairwise computation. |
| 6 | Cross-batch / cross-industry aggregates | `stats by-batch [--industry <slug>] [--tag <slug>]` / `stats by-industry [--batch <slug>]` | GROUP BY over the local companies table — count, avg team_size, % hiring, % top, % acquired per cell. Static endpoints are single-axis. |
| 7 | Batch summary card | `batches show <slug>` | Local single-batch projection: company count, top 5 industries, top 10 tags, % hiring, % top, % acquired, median team_size. One-shot card vs a raw list. |

## Stubs / known gaps

None — all 22 absorbed features and all 7 transcendence features are shipping-scope. (Founder-search was killed in the brainstorm because it depends on the optional `question_answers` field; revisit on a second print if the field is present in synced data.)
