# Google Trends Browser-Sniff Discovery Report

## Goal
Primary browser-sniff goal: "Search for a keyword, view interest-over-time / interest-by-region / related queries, then check Trending Now." Anonymous flow — no login, no auth context required.

## Method
- Backend: `agent-browser` v0.27.0 (browser-use install failed — no `uv`/`pip` on PATH; agent-browser was already installed and compatible).
- Flow driven: loaded homepage → dismissed cookie banner → typed "coffee" into the search box → clicked Explore → captured network → clicked "Trending Now" → captured network → stopped HAR recording (226 requests total).
- HAR saved to `browser-sniff-capture.har` (source for `cli-printing-press browser-sniff`).

## Reachability finding (critical)
- A direct full-page navigation to `/trends/explore?q=coffee&date=today%2012-m&geo=US` returned **HTTP 429** on first contact (Google's literal rate-limit error page), before any pacing was even applied.
- A plain `curl` GET of the homepage, with no browser fingerprint, also returned **429** immediately.
- `cli-printing-press probe-reachability` against the same URL confirmed both `stdlib` and `surf-chrome` (Chrome-TLS-fingerprint) transports get 429'd anonymously — classified `mode: browser_clearance_http`, `needs_browser_capture: true`, `needs_clearance_cookie: true`.
- By contrast, driving the SPA interaction through a real headless-Chrome session (agent-browser/CDP — real TLS handshake + JS execution + referrer chain) succeeded cleanly end-to-end.
- **Conclusion:** the printed CLI needs a clearance cookie (`NID`, domain `.google.com`, HttpOnly) harvested from a real browser session, not just a Chrome-fingerprinted HTTP client. This is an anonymous NID bootstrap, not a user login — the CLI should self-bootstrap by driving a lightweight headless-browser page load to mint the cookie, or via `auth login --chrome` importing the NID cookie from the user's existing Chrome profile.

## Endpoints Discovered (core data contract — matches pytrends' documented shape, confirmed live)

| Endpoint | Method | Auth | Notes |
|---|---|---|---|
| `/trends/api/explore/pickers/geo` | GET | cookie (NID) | Geo picker/lookup table |
| `/trends/api/explore/pickers/category` | GET | cookie (NID) | Category id→name lookup table |
| `/trends/api/explore` | POST | cookie (NID) | Takes `req` JSON (`comparisonItem[]`, `category`, `property`); returns per-widget `token` + `request` for each of: TIMESERIES, GEO_MAP, RELATED_QUERIES/TOPICS |
| `/trends/api/widgetdata/multiline` | GET | cookie (NID) | Interest-over-time; needs `req` + `token` from the explore response |
| `/trends/api/widgetdata/comparedgeo` | GET | cookie (NID) | Interest-by-region; needs `req` + `token` |
| `/trends/api/widgetdata/relatedsearches` | GET | cookie (NID) | Related queries AND related topics (top + rising) via `restriction`; needs `req` + `token` |

All responses are prefixed with Google's XSSI defense string `)]}',` — must be stripped before JSON-decoding (4 chars for `/api/explore`, 5 chars for widget/picker responses — verify exact offset per endpoint at build time).

## Endpoints Discovered (Trending Now — separately versioned, higher-maintenance surface)

| Endpoint | Method | Notes |
|---|---|---|
| `/trending` (page load) | GET | Bootstraps `f.sid` (session id) and `bl` (build label, e.g. `boq_trends-boq-servers-frontend_20260712.06_p0`) embedded in inline JS — both required for the RPC call below and both change over time |
| `/_/TrendsUi/data/batchexecute?rpcids=DqDTgb&f.sid=...&bl=...` | POST | `f.req=[[["DqDTgb","[\"<hl>\",<geo-flag>,<unknown>]",null,"generic"]]]` — returns the trending-terms list |
| `/_/TrendsUi/data/batchexecute?rpcids=g4kJzf&f.sid=...&bl=...` | POST | `f.req` carries `[[geo, term, ...], ...]` for every trending term from the DqDTgb call — returns related-entity enrichment per term |

This is Google's opaque internal `batchexecute` RPC protocol (shared across many Google products), not a REST endpoint. The `f.req` payload is a nested, URL-encoded JSON array keyed positionally, not by field name — decoding it requires reverse-engineering the array shape per RPC id, and `bl`/`f.sid` are minted per page load and will drift over time (matches GitHub issue #638's report of Google changing this exact surface in Feb 2025). **Recommend hand-coding this as an explicit two-step scrape-then-RPC flow, or shipping it as a documented v1 gap** — a decision for Phase 1.5's absorb gate, not embedded silently in the generated typed-endpoint surface.

## Noise filtered out of the raw sniff
The mechanical `browser-sniff` spec-generation step also captured unrelated page-chrome traffic that is NOT part of the Trends data API and was excluded from the hand-curated spec:
- `/$rpc/google.internal.onegoogle.asyncdata.v1.AsyncDataService/GetAsyncData` — the OneGoogle account/apps-bar widget (also the source of the tool's incorrect `X-Goog-Api-Key` auth inference — that header belongs to this unrelated internal service, not Trends data).
- `/recaptcha/enterprise/reload`, `/recaptcha/api2/ubd` — reCAPTCHA Enterprise telemetry (background bot-scoring, not a data endpoint).
- `/v1/survey/startup_config`, `/v1/survey/trigger/trigger_anonymous` — an unrelated in-page user-research survey widget.
- `/_/TrendsUi/browserinfo` — client telemetry/beacon, not data.
- `/g/collect`, `/__utm.gif` — Google Analytics/UTM beacons.

## Auth correction
The mechanical sniff inferred `auth.type: api_key` / header `X-Goog-Api-Key` from the unrelated OneGoogle RPC call above — this is wrong for the actual Trends data endpoints, which authenticate via the anonymous `NID` cookie only. Corrected in the hand-curated spec to `auth.type: cookie`.
