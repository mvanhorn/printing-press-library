---
type: research-brief
title: Google Keyword Planner evidence collector
api: keyword-planner
api_version: v25
created: 2026-09-08
public_projection: true
---

# Implementation brief

Build a read-only Google Ads Keyword Planner CLI with durable local evidence. The two headline methods are GenerateKeywordIdeas and GenerateKeywordHistoricalMetrics; a small read-only account lookup supplies currency provenance. The API contract comes from Google's v25 Discovery/RPC documentation. Authentication uses the AdWords OAuth scope, an in-memory access token refreshed from a grant, a developer token and optional manager login context.

## Required behavior

- Ideas accepts 1–20 keyword seeds and follows every page token. Historical batches lists of at most 10,000 terms while preserving submitted order and close-variant lineage.
- Targeting is explicit: language resource, at most ten geo resources or explicit all-geographies, GOOGLE_SEARCH, and a requested year-month range.
- Default history requests twelve closed UTC months. Longer ranges remain explicit; returned coverage is recorded without fabricating unavailable months. Google's average-monthly metric is its trailing-year average, independent of the requested range.
- Decode REST month names and historical protobuf month values correctly. Preserve signed int64 values, micros and null-versus-zero distinctions.
- Reject current/future request ranges before network activity. Preserve anomalous returned months in raw evidence while excluding them from normalized analytics.
- Every collection receives a fresh immutable snapshot. Save exact Ads response bytes before decode/retry; preserve incomplete attempts and failure receipts. Never persist OAuth token responses or credential headers.
- Preserve submitted, returned and close-variant relationships. Combined geographies are one combined selection. Never multiply demand into separate variants or countries.
- Flag ambiguous zero/UNSPECIFIED data as PLANNER-BLIND; exclude it from safe summaries. Valid shorter series remain usable as PLANNER-SHORT-WINDOW.
- Offline list/show/search/export and analysis require neither credentials nor network. Keep generated learning/profile state separate from the evidence database.
- Provide a read-only DuckDB attachment and evidence, compatibility and safe views with explicit DATE/BIGINT/TIMESTAMP types.
- Bound retries to transient failures and coordinate pacing across processes on one host. Permanent authentication, account and daily-quota failures stop clearly.
- CLI and MCP must use the same curated input, authentication and evidence policy. Do not expose generic raw endpoint bypasses or account mutation.

## Evidence views

Trace links one number to its raw receipt. Coverage explains missing or excluded data. Variants preserves lineage. Diff compares compatible immutable snapshots. Integrity reconciles stored hashes, receipts and rows. Safe statistics excludes incomplete, anomalous and ambiguous data.

## Acceptance and limits

Live checks exercise both Planner methods. Fixtures cover empty results, optional CPC, null/zero, all month forms, int64 precision, future anomalies, incomplete paging, failed persistence, invalid grants, permissions, quota and retired API versions. Offline and MCP checks deny network and omit credentials.

Google supplies estimates for Google search and advertiser bids. These are not measured YouTube demand, revenue forecasts or organic SEO difficulty. Missing or suppressed data has no invented explanation. The published MCP verification surface is stdio; optional generated HTTP support remains outside that verification scope.

## Primary references

- [Generate keyword ideas](https://developers.google.com/google-ads/api/docs/keyword-planning/generate-keyword-ideas)
- [Historical metrics](https://developers.google.com/google-ads/api/docs/keyword-planning/generate-historical-metrics)
- [REST authentication](https://developers.google.com/google-ads/api/rest/auth)
- [Quotas](https://developers.google.com/google-ads/api/docs/best-practices/quotas)
- [API lifecycle](https://developers.google.com/google-ads/api/docs/sunset-dates)
