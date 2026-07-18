# EIA Energy CLI Research Brief

## API Identity
EIA API v2 is a self-discovering hierarchy of energy datasets covering electricity, petroleum, natural gas, coal, nuclear, international data, state profiles, and more. Routes expose metadata, facets, frequencies, and data columns. A free API key is mandatory in the `api_key` query parameter; JSON data is capped at 5,000 rows with `offset`/`length` pagination.

## Users
- An energy-market analyst checking regional power/fuel movements every morning.
- A policy or climate researcher comparing states and fuels over consistent periods.
- A journalist reproducing an energy-price or generation chart from public data.
- An operations/procurement analyst monitoring price, inventory, generation, or demand thresholds.

## Top Workflows
1. **Latest grid pulse:** fetch latest balancing-authority demand/generation/interchange observations and compare with a local trailing baseline.
2. **State/fuel comparison:** resolve route metadata/facets and compare normalized series across states, sectors, or fuels.
3. **Spread monitor:** calculate documented cross-series spreads with aligned frequency/period and report stale/missing observations.
4. **Anomaly watch:** retain selected series and alert on deviations while showing the baseline and source rows.

## Reachability Risk
API keys can be temporarily suspended when clients exceed request tolerances. Dataset shapes vary by route and all returned data values are strings. Frequency/facet alignment is essential; the CLI must not silently compare incompatible units or periodicities.

## Table Stakes
Route discovery; route metadata; facet values; generic data query with `data[]`, facets, frequency, start/end, sort, offset/length; common electricity/petroleum/natural-gas shortcuts; pagination; local sync/search.

## Data Layer
SQLite stores route schemas, facet dictionaries, series observations keyed by route/facets/period/frequency/unit, query provenance, and watch definitions. Derived comparisons must retain units and alignment decisions.

## Codebase Intelligence
The EIA web explorer, notebooks, language SDKs, and generic HTTP clients cover raw retrieval. A dedicated CLI can make the self-describing hierarchy navigable, cache schemas, align series safely, and provide repeatable monitoring.

## User Vision
Build `grid`, `state`, `fuel`, `spread`, `anomaly`, and `watch`, with generic route/facet discovery underneath.

## Product Thesis
Make EIA's enormous hierarchy explorable and analytically safe from the terminal: no hidden unit conversions, frequency mismatches, or truncated results.

## Build Priorities
1. Route/facet discovery and safe generic query.
2. Grid/state/fuel shortcuts.
3. Spread/anomaly/watch with unit/frequency guards.
4. Local history and agent output.

## Sources
- https://www.eia.gov/opendata/documentation.php (raw capture HTTP 200)
- https://api.eia.gov/v2/

