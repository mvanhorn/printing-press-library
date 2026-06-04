# Amazon Ads Schema Provenance

Run ID: `20260527-233111`

The printed CLI is based on the community-maintained Amazon Ads OpenAPI schema set from `whitebox-co/amazon-ads-api` under `docs/schemas`, pinned to commit `757caf2865692e7d1eb05a97436c42265c317542`.

The shipped `spec.json` is the internal Printing Press merged spec with an Amazon Ads Login with Amazon OAuth refresh-token overlay:

- Token URL: `https://api.amazon.com/auth/o2/token`
- Authorization URL: `https://www.amazon.com/ap/oa`
- Verification path: `/v2/profiles`
- Base URL: `https://advertising-api.amazon.com`
- Required auth inputs: `AMAZON_ADS_CLIENT_ID`, `AMAZON_ADS_CLIENT_SECRET`, `AMAZON_ADS_REFRESH_TOKEN`
- Request-scope header: `Amazon-Advertising-API-Scope`, sourced from the selected profile ID

The manifest records `spec_path: spec.json` and the SHA-256 checksum of the checked-in merged spec.

Hourly-report note: the pinned DSP reporting schema exposes `SUMMARY` and `DAILY` `timeUnit` values. The merged spec also includes Amazon Ads Insights timing data with `hourOfDay`. The novel `dayparting-analysis` and `budget-pacing` commands therefore operate on exported reports that already include `hour`, `hourOfDay`, or timestamped rows instead of assuming every Sponsored Ads report can be generated hourly.
