# traffic-intel-pp-cli

Private Printing Press MVP CLI for combined Google Search Console (GSC), Google Analytics 4 (GA4), and Ahrefs-style traffic intelligence.

The MVP is **local-first**: it never calls external APIs by itself. `sync` loads a generic ecommerce/SEO fixture by default or imports local JSON. When explicitly requested with `--source`, `--live`, or `--real`, it shells out to child CLIs (`google-search-console-pp-cli`, `google-analytics-pp-cli`, `ahrefs-pp-cli`) in agent/JSON mode and stores normalized local results.

## Printing Press metadata

- `.printing-press.json` declares the private CLI scaffold, agent context command, doctor command, env names, and planned child adapter commands.
- `Makefile` provides `fmt`, `test`, `build`, `doctor`, and `smoke` targets.

## Build

```bash
make build
# or
go build ./cmd/traffic-intel-pp-cli
```

## Agent mode

`--agent` sets `--json --compact --no-input --yes --no-color`.

```bash
traffic-intel-pp-cli --agent agent-context
traffic-intel-pp-cli --agent doctor
traffic-intel-pp-cli --agent sources doctor
```

`agent-context` emits schema `traffic-intel.agent-context/v1` with commands, env var presence booleans, and the planned child-CLI source plan. Doctors report child binary paths and env presence only; they do not print env values or secrets.

## Typical workflow

```bash
traffic-intel-pp-cli profile save --name site --site https://example.com --ga-property 123 --ahrefs-project example
traffic-intel-pp-cli --profile site sync
traffic-intel-pp-cli --profile site sync --source all
traffic-intel-pp-cli --profile site money-pages --limit 5
traffic-intel-pp-cli --profile site query-revenue jackets
traffic-intel-pp-cli --profile site explain-drop
traffic-intel-pp-cli --profile site refresh-queue
traffic-intel-pp-cli --profile site opportunity-gap
traffic-intel-pp-cli --profile site refresh-brief /collections/winter-jackets
traffic-intel-pp-cli --profile site quick-wins
traffic-intel-pp-cli --profile site revenue-at-risk
traffic-intel-pp-cli --profile site cannibalization
traffic-intel-pp-cli --profile site topic-clusters
traffic-intel-pp-cli --profile site source-coverage
traffic-intel-pp-cli --profile site internal-link-plan
traffic-intel-pp-cli --profile site experiment-plan /collections/winter-jackets
traffic-intel-pp-cli --profile site forecast-impact
traffic-intel-pp-cli --profile site stale-winners
traffic-intel-pp-cli --profile site digest weekly
```

Profiles live at `~/.traffic-intel-pp-cli/profiles.json` unless `--home` or `TRAFFIC_INTEL_HOME` overrides the directory.

## Sources

Page metrics preserve top-level convenience fields and nested source fields:

- `sources.gsc`: clicks, impressions, CTR, average position, previous clicks, query sample.
- `sources.ga4`: sessions, conversions, revenue, previous sessions/revenue.
- `sources.ahrefs`: backlinks, referring domains, top keyword.

Optional env vars used for profile defaults or future child CLI sync:

- `TRAFFIC_INTEL_HOME`
- `GSC_SITE_URL`
- `GA4_PROPERTY_ID`
- `AHREFS_PROJECT` / `AHREFS_TARGET`

Child CLI sync is opt-in. `sync --source all` runs all three child commands and requires all three source configs; `--source gsc`, `--source ga4`, and `--source ahrefs` run one configured source. `--live` and `--real` are aliases for `--source all`. Fixture mode remains the default when none of those flags are present.

## Novel analysis commands

- `opportunity-gap` ranks high-impression pages in positions 4-20 where CTR trails the expected curve and GA4/Ahrefs value makes the upside worth chasing.
- `quick-wins` surfaces near-page-one pages with weak CTR and conversion or revenue value.
- `revenue-at-risk` ranks pages where lost clicks, sessions, or revenue overlap with meaningful commercial value.
- `refresh-brief <url-or-topic>` generates an agent-ready page brief with likely issue, metrics, recommended actions, and follow-up commands.
- `cannibalization` groups pages competing for the same query/topic and recommends a canonical URL.
- `topic-clusters` summarizes clicks, revenue, backlinks, and decay by inferred topic cluster.
- `source-coverage` audits which pages have GSC, GA4, and Ahrefs evidence and what source sync is missing.
- `internal-link-plan` recommends source and target pages for internal links based on topic, revenue, and link equity.
- `experiment-plan <url-or-topic>` turns one page into title, meta, content, and measurement tests.
- `forecast-impact` estimates click, conversion, and revenue upside from closing CTR gaps.
- `stale-winners` finds valuable pages to refresh before visible decline.

## Import format

`sync --import path.json` accepts either:

- a full `DataSet` JSON object with `profile`, `source`, and `pages`; or
- a JSON array of page metric objects.
