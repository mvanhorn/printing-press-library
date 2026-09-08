---
type: reference
title: Keyword Planner evidence contract
project: printing-press
created: 2026-09-07
updated: 2026-09-07
written_by: codex
---

# Keyword Planner evidence contract

This press collects Google Search keyword ideas and historical metrics into a local SQLite portfolio. Each collection is a new snapshot. Google's estimates describe searches and advertiser bids; they do not measure YouTube demand, RPM, or revenue.

## Receipt and interpretation

The response body is committed as exact bytes before JSON decoding. Its receipt retains the SHA-256 hash, HTTP status, request ID, fetch time, and ordered page, batch, and attempt identity. Error responses are evidence too. OAuth token responses and authentication headers never enter this database.

Normalized data links back to its receipt. A decoding or persistence failure keeps already committed evidence, marks the collection incomplete, and returns a nonzero exit. Output limits never change the collection scope. An explicit collection budget can stop pagination or batching only with a visible incomplete outcome.

Ideas belong to the combined seed request. Historical results retain submitted order, returned text, close variants, and unmatched inputs. A returned variant group is one observation. Separate batches do not establish global deduplication. Combined geographies stay one combined selection.

## Months, nulls, and estimates

The default range is the latest twelve closed UTC calendar months. On 07/09/2026 this resolves to September 2025 through August 2026. Explicit ranges override that default. Google can return a subset; missing months are not zero and are never padded or interpolated.

Requests ending in the current or a future month fail before a Google call. If a response nevertheless contains an open or future month, that entry remains only in the unchanged raw receipt. It cannot enter normalized monthly rows or analytical exports.

REST month names map to calendar January=1 through December=12. Legacy protobuf enum integers use January=2 through December=13; they are a separate representation. All numeric parsing preserves signed 64-bit precision. Absent and null values remain unavailable, while explicit zero remains zero.

An average of zero with competition `UNSPECIFIED` carries `ambiguous_zero_unspecified` and the analyst marker `PLANNER-BLIND`. It stays inspectable and is excluded from default aggregates. The press does not infer a suppression cause. A valid shorter series carries `PLANNER-SHORT-WINDOW` and remains usable.

Google's average monthly searches is its trailing twelve-month estimate, even when an older monthly range was requested. It is distinct from any local range average. Bid and CPC micros belong to the keyword snapshot, not individual historical months. Repeated bid columns in a monthly export are context. Missing optional CPC is unavailable and does not make a collection fail.

## Targeting and account context

Every request records explicit language and geo resource names, `GOOGLE_SEARCH`, API version, and resolved range. At most ten geographies are accepted. An all-geographies request requires explicit acknowledgement. No Search Partners mode is exposed.

`GOOGLE_ADS_CUSTOMER_ID` is the operating target; `GOOGLE_ADS_LOGIN_CUSTOMER_ID` supplies manager login context. These are separate bindings even when their values are equal. Verify eligibility for the configured target with doctor --live; manager status alone does not establish eligibility.

Currency comes from verified account metadata and carries provenance. No account currency is assumed.

The shipped compatibility mappings identify `languageConstants/1000` as English (`en`) and `geoTargetConstants/2840` as the United States (`US`). Google's [historical metrics examples](https://developers.google.com/google-ads/api/docs/keyword-planning/generate-historical-metrics) explicitly identify both constants; checked 07/09/2026. Other mappings remain unresolved unless supported by verified metadata. Unknown mappings are reported, never guessed.

## Local consumption

The delivered portfolio is `~/.local/share/keyword-planner/snapshots.db`. Portfolio commands operate offline. The separate generated learning store is not the evidence portfolio.

The standalone SQL recipe attaches the portfolio under the logical source alias `keyword_planner` using DuckDB's SQLite extension with `READ_ONLY`. The compatibility relation `keyword_monthly` preserves the warehouse fields and adds snapshot, receipt, endpoint, variant-group, targeting, currency, requested-range, status, and completeness context. Dates and timestamps are cast explicitly.

The normal single-country relation excludes incomplete collections, unresolved mappings, and combined-geography selections. Nulls remain inspectable; safe aggregates exclude unavailable and ambiguous values explicitly. Comparing or aggregating saved snapshots must preserve their identity rather than treating repeat fetches as additional demand.

The press does not alter the warehouse registry, loaders, or session database. See Google's [quota documentation](https://developers.google.com/google-ads/api/docs/best-practices/quotas) and DuckDB's [SQLite extension documentation](https://duckdb.org/docs/current/core_extensions/sqlite) for vendor contracts. Local pacing coordinates processes on this host; it does not represent account-wide remaining quota or coordinate other hosts.
