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

Child CLI sync is opt-in. `sync --source all` runs all three child commands; `--source gsc`, `--source ga4`, and `--source ahrefs` run one source. `--live` and `--real` are aliases for `--source all`. Fixture mode remains the default when none of those flags are present.

## Import format

`sync --import path.json` accepts either:

- a full `DataSet` JSON object with `profile`, `source`, and `pages`; or
- a JSON array of page metric objects.
