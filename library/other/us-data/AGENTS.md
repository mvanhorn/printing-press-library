# us-data-pp-cli Agent Notes

This CLI is a generated-then-curated Printing Press entry for official US public data. Keep the command surface read-only and recipe-oriented.

## Source Boundaries

- BLS keyless workflows use the public v1 timeseries endpoint.
- Census data queries require `US_DATA_CENSUS_API_KEY`.
- BEA data queries require `US_DATA_BEA_API_KEY`.

Do not add forecasts, investment advice, legal advice, or statistical modeling beyond source-provided values and simple documented comparisons.

## Patch Recording

Record hand edits under `.printing-press-patches/`. Do not edit generated catalog artifacts such as `registry.json` or `cli-skills/pp-us-data/SKILL.md`.
