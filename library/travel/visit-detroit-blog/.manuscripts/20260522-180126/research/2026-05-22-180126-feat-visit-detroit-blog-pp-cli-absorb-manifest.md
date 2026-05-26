# Inside the D CLI — Absorb Manifest

**API:** Inside the D (Visit Detroit editorial blog), slug `visit-detroit-blog`, binary `visit-detroit-blog-pp-cli`.
**Backend:** Algolia search REST, index `prod-visit-detroit-listings`, `sectionName:Blogs` (748 articles), public search-only key, auth=none.

## Ecosystem scan (Step 1.5a)

| Search | Result |
|--------|--------|
| Visit Detroit / Inside the D CLI, MCP, or wrapper on GitHub | **None** — no tool wraps this blog. |
| Algolia content/blog reader tools | Only `algolia/cli` + `algolia/mcp` (generic index *admin* devtools — manage your own indices; not consumer content tools and useless without index internals). |
| Claude plugin / skill for Visit Detroit | None. |

**Conclusion:** zero direct competitors. The "incumbent" is the website's own browse-only UI + Algolia instant-search box (single-facet). Everything below is matched against that and beaten with offline SQLite, agent-native output, and cross-axis queries the UI can't express.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Full-text search across articles | visitdetroit.com Algolia instant-search box | Live Algolia query + offline SQLite FTS5 (`search`) | Offline, ranked, regex/SQL-composable, `--json`/`--select` |
| 2 | Browse by category | site category nav (blogCategories) | `blogs list --category` + `categories` (counts) | Cross-filter with region+date, agent-native |
| 3 | Browse by neighborhood/region | site region tags (partnerRegions) | `blogs list --region` + `regions` (counts) | Combine with category, neighborhood-name aliases |
| 4 | Open & read an article | site article web page | `blogs get <slug>` reads full body offline | No browser, full content from store, `--json` |
| 5 | Recent posts | site "latest" ordering | `recent --limit N` (postDate sort) | `--since`/`--until` date windows, offline |
| 6 | Article metadata (image/date/categories) | site article header | structured fields on every record | `--select` dotted paths, CSV, bounded output |
| 7 | Sync / index the corpus | (Algolia admin tooling only) | `sync` pulls all 748 into SQLite | Offline corpus = foundation for everything below |

Every absorbed row ships with `--json`, `--select`, `--dry-run` where it mutates (none here — read-only), typed exit codes, and SQLite persistence.

## Transcendence (only possible with our approach)

From the novel-features subagent (first print; 16 candidates → 5 survivors, all ≥5/10). Persona-served column is the audit trail.

| # | Feature | Command | Score | Persona | Why Only We Can Do This |
|---|---------|---------|-------|---------|-------------------------|
| 1 | Cross-axis filter (category × region × time) | `blogs list --category <c> --region <r> --since <d> --until <d>` | 9/10 | Renee, Atlas | One SQLite WHERE over category, region, and date at once — the combined query the single-facet Algolia instant-search UI cannot express. |
| 2 | Related reads | `blogs related <slug> --limit N` | 9/10 | Renee, Marcus, Atlas | Local join ranking posts by shared categories ∩ regions; the site has no related-posts surface crossing both axes. |
| 3 | Coverage map (category × region cross-tab) | `blogs coverage [--category <c>]` | 7/10 | Marcus, Priya | GROUP BY cross-tabulation Algolia's one-dimensional facet API cannot return. |
| 4 | Reading list / export | `blogs reading-list [filters] --output <file>` | 7/10 | Priya | Materializes an ordered, deduped md/json/csv list to a file from the local store; every web search is otherwise ephemeral browser tabs. |
| 5 | Editorial-only filter (shared flag) | `--no-sponsored` / `--sponsored-only` on list/search/reading-list | 6/10 | Priya, Renee | Local boolean predicate on the stored `sponsored` column — separate editorial from sponsored for neutral handouts. |

Notes:
- #1 is the cross-axis *capability* layered onto the absorbed `blogs list` (the single-facet pieces are table stakes; the combination is the leverage). #5 is a shared flag, not a standalone command.
- Genuinely-new standalone commands: `blogs related`, `blogs coverage`, `blogs reading-list`.

## Stubs

None. Every feature above is fully buildable against the live Algolia index + local SQLite store. No paid tier, no headless browser, no LLM dependency.

## Final command surface (target)

- **Foundation (P0):** `sync`, `search`, `sql` (generator), SQLite `blog_posts` + FTS5.
- **Absorbed (P1):** `blogs list` (filters + sponsored flag), `blogs get`, `categories`, `regions`, `recent`.
- **Transcendence (P2):** `blogs related`, `blogs coverage`, `blogs reading-list`.
- Plus generator framework: `doctor`, `context`, `version`, MCP server (cobratree mirror).
