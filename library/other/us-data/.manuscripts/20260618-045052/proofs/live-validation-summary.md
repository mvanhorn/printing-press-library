# us-data Live Validation Summary

## Environment

- BLS keyless public API: available.
- `US_DATA_CENSUS_API_KEY`: not configured locally.
- `US_DATA_BEA_API_KEY`: not configured locally.

## Validated Workflows

- `us-data-pp-cli cpi --agent` uses BLS public v1 timeseries and returns live CPI data.
- `us-data-pp-cli unemployment --agent` uses BLS public v1 timeseries and returns live national unemployment data.
- `us-data-pp-cli population --place "Austin, TX" --agent` returns structured Census key setup guidance when no key is configured.
- `us-data-pp-cli industry --naics 541511 --agent` returns structured BEA UserID setup guidance when no key is configured.
- `us-data-pp-cli compare-regions "Seattle, WA" "Austin, TX" --agent` returns a comparison shell and names missing Census/BEA env vars.

## Auth Notes

Census no-key test returned an HTTP 302 with `X-DataWebAPI-KeyError: 1` and a `missing_key.html` location, matching the current Census user guide. BEA documentation requires a registered UserID. Those keys are not included in repository artifacts.
