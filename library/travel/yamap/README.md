# YAMAP CLI

**Discover Japan hikes with compact source facts and dated contributor evidence.**

Created by [@zjsng](https://github.com/zjsng) (zjsng).

Search named mountains, planned model courses, recorded trips and map areas. Keep source metrics and contributor observations separate from closure and safety decisions.

## Install

After this PR is merged and the catalog updates:

```sh
npx -y @mvanhorn/printing-press-library install yamap --cli-only
```

## Build

Requires Go 1.26.6 or newer. From this source checkout:

```sh
go build -o yamap-pp-cli ./cmd/yamap-pp-cli
./yamap-pp-cli --version
```

## Authentication

Anonymous public first-party JSON reads; no account or paid key. Membership-only GPX and multi-landmark operations excluded.

## Quick Start

```bash
# Inspect bounded public access configuration.
yamap-pp-cli doctor --dry-run

# Disambiguate same-name mountains by prefecture.
yamap-pp-cli mountains search 高尾山 --limit 3 --agent

# Find planned reference routes.
yamap-pp-cli routes search 高尾山 --limit 3 --agent

# Inspect dated contributor evidence.
yamap-pp-cli reports recent 高尾山 --limit 3 --agent

```

## Agent Usage

Focused `mountains`, `routes`, `reports`, `maps`, and `inventory` commands emit compact `{meta,results}` JSON by default, also with `--agent`/`--json`. Summaries are lazy: no detail fanout, photo downloads, tracks or geometry. `--select` projects result fields while preserving provenance and coverage metadata. Null is unavailable; zero is a valid source value.

```sh
./yamap-pp-cli mountains search 高尾山 --select id,name,prefectures,url --agent
./yamap-pp-cli reports get 51497803 --select id,title,metrics.distance_m,metrics.elapsed_seconds --agent
./yamap-pp-cli reports search 高尾山 --page 2 --limit 3 --agent
```

Default one page, five results; `--limit` 1–20. Recent scans default one page of twenty candidates, at most five pages with `--max-scan-pages`; dates use trip start, not publication/update time. Recent results are sorted only within examined candidates. Source search windows can cap at 10,000 reports. Coverage stays partial; zero results are not evidence of no hikes, safe trails or open routes.

## Evidence definitions

- `planned_model_course`: reference route metrics and standard course time, not a recorded trip or personalized plan.
- Activity summaries lack a planned flag on some source responses: `is_planned:null`, `route_kind:activity_log_summary`. Detail identifies recorded versus planned when the source supplies the flag.
- List report metrics are `legacy_activity_summary`; detail prefers regularized `activity_whole_section`. The source totals can differ. `elapsed_seconds` includes rest; `active_seconds` is source non-rest time. YAMAP automatically identifies stops of at least three minutes as rest. Exact `moving_seconds` is null.
- Contributor observation text has the trip date and source link. It is not authoritative advice. Seasonal or old evidence remains dated; publication recency does not make an old trip recent.
- Publisher caution notices retain linked source attribution and publication dates, bounded to five notices/600 characters each. Linked authorities are not fetched or independently verified. Course closed/dashed flags are publisher flags; false never establishes open or safe.
- Source map bounds are map-area coverage. No trail/track overlap, offline tiles, navigability, current closures or safety is established. Deprecated status and source version remain visible.

All outputs identify unknown official closure coverage. Verify authoritative closures and conditions separately; this CLI does not certify safety or access.

## Cache and resource limits

Exact endpoint/query/language responses cache for 15 minutes under the resolved cache directory in `hiking-v1`. `--home ./local-state` isolates all paths; `YAMAP_HOME` is also supported. Cache entries use private atomic files, bounded to 64 entries/16 MiB. The cache is request inventory, not a comprehensive offline database.

- `--refresh`: refresh the exact request; no full inventory crawl.
- `--no-cache`: bypass cache reads and writes.
- `--data-source local`: exact cached request only, with explicit stale metadata/warning. No network fallback.
- `--data-source live`: force a public network read.
- `--max-age`: accepted cache age, positive and at most 24 hours.
- `--diagnostics`: request count, response size and elapsed time on stderr.

Focused commands run serial requests, at most two attempts on transient HTTP 5xx/429, at most two requests/second. They use a 30-second total command budget (configurable positive `--timeout`, max 2m), 15-second per-request timeout and 4 MiB response cap. Longer Retry-After is surfaced immediately. No stale fallback after upstream failure. Focused output is JSON; CSV/plain/quiet modes are rejected with exit 2 rather than silently ignored. Compatibility source mirrors and local Printing Press utilities remain separate from focused workflows.

## Unique Features

Focused helpers preserve the evidence boundaries described above.

### Hiking evidence
- **`reports recent`** — Find recorded activity evidence by trip date in a bounded scan


  ```bash
  yamap-pp-cli reports recent 高尾山 --limit 3 --agent
  ```
- **`reports observations`** — Read bounded contributor observations with trip dates and source links


  ```bash
  yamap-pp-cli reports observations 51497803 --agent
  ```
- **`routes compare`** — Compare planned metrics with recorded metrics without assuming track equivalence


  ```bash
  yamap-pp-cli routes compare 1771 51497803 --agent
  ```
- **`maps coverage`** — Inspect map area bounds and limits of map coverage


  ```bash
  yamap-pp-cli maps coverage 77 --agent
  ```
- **`inventory status`** — Inspect cached request inventory and freshness without a crawl


  ```bash
  yamap-pp-cli inventory status --agent
  ```

## Health Check

```sh
./yamap-pp-cli doctor --agent
./yamap-pp-cli inventory status --agent
```

Health verifies API connectivity; it does not establish trail conditions. Automatic learning is disabled for focused hiking reads so they create only their cache files. No global configuration was changed by this build.

## Troubleshooting

Exit codes: 0 success (including no matches), 2 invalid input, 3 unavailable source ID/exact cache miss, 4 membership/access restriction, 5 network/challenge/schema/upstream failure, 7 throttle, 10 local path/config failure. Errors/diagnostics go to stderr; failed focused reads emit no result JSON. A failed page makes the whole operation fail, without returning phantom empty pages or fabricated partial rows.

HTTP 202/HTML means a challenge, not an empty result. Retry later after an explicit refresh; the CLI does not keep a browser resident or bypass account restrictions. A 404 can mean removed/private content. Japanese names and course `name` wire filtering were live verified; unsupported filters are not silently sent.

## Cookbook

```sh
./yamap-pp-cli routes search 高尾山 --limit 3 --agent
./yamap-pp-cli reports recent 高尾山 --days 14 --max-scan-pages 2 --agent
./yamap-pp-cli reports observations 51497803 --agent
./yamap-pp-cli routes compare 1771 51497803 --agent
./yamap-pp-cli maps coverage 77 --agent
```

Comparison does not prove that two routes are the same. GPX downloads, membership operations, multi-landmark premium filtering, personal accounts, purchases, bookings and authoritative closure verification are outside shipped scope. Free public views were exhausted before selecting this scope. Provider Premium currently advertises 5,700 JPY/year or 780 JPY/month; no paid subscription is used here.

Source definitions: [YAMAP time metrics](https://help.yamap.com/hc/ja/articles/900005534146), [Premium](https://yamap.com/premium), [multi-landmark restrictions](https://help.yamap.com/hc/ja/articles/30703212737561). Community research: [yamap-export](https://github.com/akiyama709/yamap-export) informed anonymous access and language/User-Agent behavior; its archival scope is excluded.

Canonical project, staging/library paths, live measurements, independent review and final acceptance are recorded in [evidence/FINAL.md](evidence/FINAL.md).

## Recipes

### Compact mountain candidates

```bash
yamap-pp-cli mountains search 高尾山 --agent --select id,name,prefectures,url
```

Keep source IDs and Japanese names.

### Recent contributor reports

```bash
yamap-pp-cli reports recent 高尾山 --limit 3 --agent
```

Activity-date filter within bounded candidate coverage.

### Inspect map coverage

```bash
yamap-pp-cli maps coverage 77 --agent
```

Area bounds do not establish route safety or offline availability.
