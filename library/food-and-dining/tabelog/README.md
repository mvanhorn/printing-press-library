# Tabelog CLI

Find restaurants and bars for a Japan trip, then keep a factual shortlist with personal notes. This native Go executable reads public English Tabelog pages, returns compact structured results, and keeps saved comparisons available offline.

Created by [@zjsng](https://github.com/zjsng) (zjsng).

## Build and run

From this directory, with Go 1.26.6 or newer:

```bash
go build -o tabelog-pp-cli ./cmd/tabelog-pp-cli
./tabelog-pp-cli --help
./tabelog-pp-cli find --area tokyo --cuisine bar --meal dinner --budget-max 5000 --limit 5 --agent
```

Examples below assume the binary is on PATH. Public discovery needs no account or API key. An optional `tabelog-pp-mcp` executable supports stdio hosts and runs only when explicitly started.

## Discover and inspect

```bash
tabelog-pp-cli areas Shinjuku --agent
tabelog-pp-cli cuisines bar --agent
tabelog-pp-cli find --area tokyo --cuisine bar --meal dinner --budget-max 5000 --limit 5 --agent
tabelog-pp-cli show https://tabelog.com/en/tokyo/A1301/A130101/13005012/ --agent
```

Location choices distinguish stations and districts. Verified prefecture slugs such as `tokyo` work directly; choose a returned typed source URL/selector for ambiguous names. `show` accepts canonical restaurant URLs or previously fetched IDs.

Find defaults to five candidates in Tabelog's highest-rated order. Meal budgets are genuine source filters. Supported yen thresholds appear in help; unsupported values are rejected rather than rounded. These are average-price brackets, not a guaranteed bill.

`--limit` and `--max-pages` bound discovery. Metadata reports effective criteria, returned/scanned counts and remaining coverage. A source next-page URL is a page link, not a cursor immediately after the last displayed result.

## Keep trip context

```bash
tabelog-pp-cli lists add tokyo-bars 13005012 --note "Ginza bar option" --agent
tabelog-pp-cli lists compare tokyo-bars --agent
tabelog-pp-cli lists audit tokyo-bars --require hours,payment,reservation,dinner_budget --max-age 24h --agent
tabelog-pp-cli lists refresh tokyo-bars 13005012 --agent
```

## Unique Features

| Command | Result |
|---|---|
| `lists add` | Fetched candidates and personal notes in named lists |
| `lists compare` | Offline comparison of saved facts and notes |
| `lists refresh` | Current facts, changes and visible per-ID failures |
| `lists alternatives` | Saved backups matching explicit constraints |
| `lists audit` | Unfetched, source-unknown and old evidence identified separately |

`lists show`, `lists note`, and `lists remove` maintain notebooks. Notes survive source refreshes; failed requests preserve valid snapshots. See [saved-list semantics and examples](docs/saved-lists.md).

## Output and freshness

Domain commands return `items` and `meta`. `--agent` produces compact JSON while retaining ratings, budgets and provenance.

Find summaries place shared retrieval time, source surface and meal-budget source in `meta`; per-row values override those defaults. `meta.budget_source` supplies omitted budget sources. Explicit projection accesses full records:

```bash
tabelog-pp-cli find --area tokyo --cuisine bar --limit 5 --agent --select items.id,items.name,items.rating,items.url
```

`--data-source auto` reuses fresh cache data or fetches; `live` forces source requests; `local` makes zero network requests and reports a cache miss when needed. Saved comparison and audit are local; refresh requires live requests. Dry runs preview operations without network or persistent writes.

With `--agent` or `--json`, domain errors write compact `error` and numeric `code` fields to stderr. Partial refreshes can also return usable results on stdout with a nonzero exit. Global flag-parser errors may remain plain text.

Unknown values remain unknown. Station distance is relative to its named station. Listed and review-based budgets remain separate. Hours and reservation policies are source facts, not proof of current opening or seat availability. [Tabelog explains its weighted ratings here](https://tabelog.com/en/help/score).

## Resource use and agents

Search uses listing facts without fetching every restaurant detail. Requests, pages, bodies, cache growth and refresh concurrency are bounded. Storage opens when needed; normal commands use neither a browser nor a background process.

[SKILL.md](SKILL.md) provides the concise agent workflow. Command help is the current flag reference. This CLI reads remote data and changes only its own local saved state; reservations and accounts remain website tasks.

[E2E documentation](e2e/README.md) covers replay, live verification and resource measurements. [AGENTS.md](AGENTS.md) records contributor invariants.

## Sources and inspiration

- [Tabelog English](https://tabelog.com/en/): observed public source contract.
- [Gurume](https://github.com/narumiruna/gurume): discovery, suggestions, structured details and agent workflows.
- [tabelog_scraping](https://github.com/xyx-is/tabelog_scraping): source identity and retrieval-time provenance.
- [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press): generated foundation.

Apache-2.0; see [LICENSE](LICENSE).
