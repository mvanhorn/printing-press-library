# us-data-pp-cli

`us-data-pp-cli` is a read-only Printing Press CLI for official US public economic and demographic data. It gives agents and analysts practical recipes over BLS, Census, and BEA data without making them memorize series IDs, Census geography predicates, or BEA dataset names.

## Who Uses This

- Data analysts preparing public-data research briefs.
- Data engineers who need scriptable official data checks.
- AI agents comparing cities, labor indicators, or economic context.
- Consultants, founders, and market researchers who need quick source-backed facts.

## Sources

- **BLS**: Bureau of Labor Statistics Public Data API. CPI and national unemployment use the keyless v1 timeseries endpoint by default.
- **Census**: Census Data API. Current Census data queries require an API key, exposed as `US_DATA_CENSUS_API_KEY`.
- **BEA**: Bureau of Economic Analysis API. BEA requests require a registered UserID, exposed as `US_DATA_BEA_API_KEY`.

The CLI reports source-provided data and simple comparisons. It does not forecast, make investment recommendations, provide legal advice, or infer missing statistics.

## Install

After this CLI is published in the Printing Press library:

```bash
npx -y @mvanhorn/printing-press-library install us-data
```

From source:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/us-data/cmd/us-data-pp-cli@latest
```

## Quick Start

Fetch a live CPI snapshot from BLS:

```bash
us-data-pp-cli cpi --agent
```

Fetch national unemployment from BLS:

```bash
us-data-pp-cli unemployment --agent
```

Check Census population setup or fetch population when a key is configured:

```bash
us-data-pp-cli population --place "Austin, TX" --agent
```

Build a source-aware comparison shell for two regions:

```bash
us-data-pp-cli compare-regions "Seattle, WA" "Austin, TX" --agent
```

Show source coverage and auth requirements:

```bash
us-data-pp-cli sources --agent
```

## Auth

BLS CPI and national unemployment work without credentials through the public v1 timeseries API.

Census and BEA require keys:

```bash
export US_DATA_CENSUS_API_KEY="..."
export US_DATA_BEA_API_KEY="..."
```

Optional:

```bash
export US_DATA_BLS_API_KEY="..."
```

The first print keeps BLS calls keyless for live dogfood. A later extension can use `US_DATA_BLS_API_KEY` for v2 metadata and higher limits.

## Commands

### `cpi`

Returns the latest BLS CPI observation, prior monthly observation, and percent change when calculable.

```bash
us-data-pp-cli cpi --series CUUR0000SA0 --years 3 --agent
```

### `unemployment`

Returns national unemployment by default.

```bash
us-data-pp-cli unemployment --agent
```

State unemployment requires LA/LAS series mappings. In this first print, pass an explicit BLS `--series` when using a local/state series.

### `population`

Fetches Census ACS profile population for supported city/state labels when `US_DATA_CENSUS_API_KEY` is configured. Without the key, it returns structured setup guidance instead of failing silently.

```bash
us-data-pp-cli population --place "Austin, TX" --agent
```

Supported examples in this first print: `Austin, TX`, `Seattle, WA`, `San Francisco, CA`, `New York, NY`.

### `wages`

Explains source-backed occupational wage expansion status. It deliberately avoids returning an unrelated national earnings proxy for an occupation.

```bash
us-data-pp-cli wages --occupation "software developer" --agent
```

### `industry`

Uses BEA setup guidance unless `US_DATA_BEA_API_KEY` is configured. With a key, it queries a starter BEA Regional `SAINC5N` state-level table and includes the raw BEA response for extension work. The `--naics`, `--industry`, and `--state` flags are returned as request context only in this first print; they are not applied to the live BEA query until table and line mappings are expanded.

```bash
us-data-pp-cli industry --naics 541511 --agent
```

### `compare-regions`

Builds an agent-readable comparison shell. With Census and BEA keys configured, it can include source-backed population and regional economic facts; without keys, it names the missing env vars clearly.

```bash
us-data-pp-cli compare-regions "Seattle, WA" "Austin, TX" --agent
```

## Notes

- BLS v1 has lower limits than registered v2 access.
- Census datasets are tied to vintages/reference years.
- BEA dataset/table/line metadata should be used when extending industry mappings.
- A missing result is not proof that a fact does not exist; it can mean the selected dataset, vintage, geography, or auth configuration does not cover it.
