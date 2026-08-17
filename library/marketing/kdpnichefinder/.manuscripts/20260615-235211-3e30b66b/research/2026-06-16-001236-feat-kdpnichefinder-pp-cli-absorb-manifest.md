# KDP Niche Finder CLI — Absorb Manifest

## Absorbed (match or beat the web UI + competitor table-stakes)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Browse niche bucket | KDP Niche Finder web | (generated endpoint) niches browse {type} | --json/--select/--limit, offline mirror |
| 2 | Search within bucket | web ?search | (behavior in niches browse) --search | composable, scriptable |
| 3 | Paginate bucket | Laravel paginator | (behavior in niches browse) --page | bounded fetch |
| 4 | List categories | /api/categories | (generated endpoint) categories list | typed output |
| 5 | List folders | /api/folders | (generated endpoint) folders list | --json |
| 6 | Create folder | POST /api/folders | (generated endpoint) folders create --name | scriptable, --dry-run |
| 7 | Save/unsave book | POST /api/books/{id}/toggle-save | (generated endpoint) books save {id} --folder | idempotent intent, --dry-run |
| 8 | View saved books | /app/saved-books | (generated endpoint) saved list | offline mirror |
| 9 | Current user | /api/user | (generated endpoint) user get | doctor integration |
| 10 | Demand metric (est. sales/revenue) | Publisher Rocket BSR→sales | (behavior in niches browse / store) estimated_monthly_sales/revenue | provided directly per book |

## Transcendence (only possible with local SQLite + agent-native output)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Opportunity rank | rank | hand-code | Cross-bucket composite ranking; web shows one bucket at a time | Use to rank niches across all four buckets by opportunity (incl. --select value for revenue-per-price). Do NOT use for per-folder totals or single-book competitor sets. |
| 2 | Revenue drift | drift | hand-code | Local timestamped snapshots; upstream has NO history API | Use to see which synced niches are rising/fading vs an earlier snapshot. Snapshots auto-recorded on sync; needs >=2 syncs on different dates. |
| 3 | Cross-bucket dedup | dupes | hand-code | ASIN-derived join across buckets, invisible in UI | none |
| 4 | Publisher saturation | saturation | hand-code | publisher-concentration metric the API has no field for | Use for bucket-level revenue concentration (whale vs fragmented). For one book's competitors, use competitors. |
| 5 | Competitor inspect | competitors | hand-code | reverse-ASIN/price-band join around a focus book | Use to inspect one book's competitors (same publisher / price band). For whole-bucket concentration, use saturation. |
| 6 | KDP keyword CSV export | export | spec-emits | service-specific KDP CSV shape over synced data | none |
| 7 | Title keyword frequency | keywords | hand-code | mechanical title-token aggregation, no LLM | none |

Hand-code count: 6 (rank, drift, dupes, saturation, competitors, keywords). spec-emits/framework: 1 (export).
Stubs: none.
