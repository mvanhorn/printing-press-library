# US Data Research Brief

Issue #1189 asks for practical public economic and demographic recipes over BLS, Census, and BEA.

The first print keeps the surface intentionally small:

- BLS CPI and national unemployment work keylessly through the public v1 timeseries API.
- Census population lookups support city/state resolution for a small set of examples and require `US_DATA_CENSUS_API_KEY`.
- BEA industry/regional data requires `US_DATA_BEA_API_KEY`.
- Region comparison combines available source-backed facts and reports missing credentials clearly.

This avoids the common failure mode of a raw public-data endpoint dump. The CLI is designed for agents writing research briefs who need compact source-aware output.
