# NEPRA browser-sniff discovery report

**Decision:** approved by the operator at the Phase 1.7 gate.
**Backend:** `agent-browser` 0.27.0 (`network har start` + `network requests`).
**Why not browser-use:** browser-use 0.1.6 is installed but its CDP WebSocket handshake
timed out — it requires a manual "Allow remote debugging" click at
`chrome://inspect/#remote-debugging`. Fell back per Cardinal Rule 1's valid fallback set.

## Primary goal
Browse the State of Industry generation data and DISCO performance reports as a user
would, to reveal any query/filter/search surface or JSON API behind the static HTML.

## Flow walked
1. `https://nepra.org.pk/` (homepage)
2. `Performance Reports.php`
3. `Detail of Generation/Main.htm`
4. `.../List of Companies Genenration wise 2023-24_files/sheet001.htm`
   — the browser followed the frameset and resolved to
   `List of Companies Genenration wise 2023-24.htm`, confirming the two-hop shell/payload shape
5. `Detail of Generation/Quarterly Data (XWD & KE).htm`

## RESULT: no hidden API contract exists. Direct HTTP is the complete surface.

HAR: 124 requests captured.

| Signal | Observed |
|---|---|
| Non-telemetry XHR/fetch requests | **0** |
| JSON responses | **0** |
| `/api/` paths | **0** |
| GraphQL | **0** |
| WebSocket | **0** |
| Auth headers (authorization/cookie/x-api-key/x-csrf) | **0** |
| Forms / inputs on generation pages | **0 / 0** |
| Frames on generation pages | 2 (frameset) |

Resource-type tally across the walk: 49 Image, 24 Script, 20 Stylesheet, 10 Document,
6 XHR, 5 Other, 5 Media, 5 Font.

**All 6 XHR requests were `POST /cdn-cgi/rum?` → 204** — Cloudflare Real User Monitoring
telemetry beacons, not a data API.

Hosts contacted: nepra.org.pk (97), static.cloudflareinsights.com (9, telemetry),
www.google.com (4), fonts.gstatic.com (3), fonts.googleapis.com (2), cse.google.com (1),
clients1.google.com (1).

## Two findings the capture added beyond direct HTTP

1. **NEPRA's on-site search is a Google Custom Search Engine widget** (`cse.google.com`,
   `clients1.google.com`). There is no native NEPRA search endpoint to call. A CLI must
   therefore build its own local index; it cannot proxy a site-search API, and should not
   route through Google CSE (separate key and ToS surface).
2. **The frameset redirect is browser-side.** Requesting the `_files/sheet001.htm` payload
   directly in a browser resolves to the parent `.htm` shell. Direct HTTP with curl fetches
   the payload as-is, so the CLI must request the `_files/sheet001.htm` path explicitly and
   must NOT rely on browser-style frame resolution.

## Replayability verdict: PASS

Cardinal Rule 5 is satisfied by structured HTML extraction targets. The shippable surface is
plain HTTP + HTML table extraction against static, predictably-named Excel-exported documents.
The printed CLI ships direct `net/http` with `response_format: html`. **No browser in the
runtime, no clearance cookie, no session.**

## Spec source
`sniffed` is not the right label: the browser produced no endpoints the direct-HTTP recon had
not already mapped. The spec will be **hand-authored internal YAML** with `response_format: html`
endpoints, the same pattern PBS and CDC use, informed by this capture's negative result.

---

## INDEPENDENT CONFIRMATION — operator's own Chrome DevTools, 2026-09-08 ~21:17 PKT

The operator opened DevTools on the same instrumented Chrome instance (banner: "Chrome is
being controlled by automated test software") and applied the **Fetch/XHR** filter directly.

**Result: `3 / 280 requests`.** All three are `POST https://nepra.org.pk/cdn-cgi/rum?`
returning `204 No Content`, `Content-Type: text/plain`, `Cf-Ray a37f3f48ce06ab61-SIN`,
CORS-scoped (`Access-Control-Allow-Origin: https://nepra.org.pk`,
`Access-Control-Allow-Methods: POST,OPTIONS`). Request side is
`Content-Type: application/json`, `Content-Length: 872`, `Sec-Fetch-Dest: empty`,
`Sec-Fetch-Mode: cors` — a Cloudflare Real User Monitoring beacon.

This is a second, independent measurement using a different tool over a LARGER request
population than the agent-browser capture (280 vs 124) and it agrees exactly:
**the site has no data API, no JSON endpoint, no GraphQL, no WebSocket.**
The only Fetch/XHR traffic in existence on this site is Cloudflare telemetry.

Confidence on "direct HTTP + HTML extraction is the complete surface": **high, two tools.**

## THIRD FINDING — the charset is declared ONLY in-document, not in the HTTP header

Measured across five surfaces:

| Surface | HTTP `Content-Type` | in-document `<meta>` |
|---|---|---|
| `nepra.org.pk` (homepage) | `text/html; charset=UTF-8` | none |
| `Detail of Generation/Main.htm` | `text/html` (**no charset**) | `charset=windows-1252` |
| `...2023-24_files/sheet001.htm` | `text/html` (**no charset**) | `charset=windows-1252` |
| `...2017-18_files/sheet001.htm` | `text/html` (**no charset**) | `charset=windows-1252` |
| `Quarterly Data (XWD & KE).htm` | `text/html` (**no charset**) | `charset=windows-1252` |

TWO distinct traps here, and the second is the dangerous one:

1. The CMS homepage is UTF-8 while every Excel-exported data file is windows-1252. There is
   no single site-wide encoding; the decode path must be per-response.
2. **For the data files the HTTP header carries NO charset parameter at all.** A client that
   trusts `Content-Type` alone gets no encoding signal and will typically default to UTF-8,
   which silently mangles the windows-1252 bytes — high-bytes become replacement characters
   and, as measured earlier in this run, a naive UTF-8 read can report ZERO `<tr>`/`<td>`
   tags in a 493 KB file that actually contains 139 rows and 4,965 cells.

Parser requirement: read the in-document `<meta charset>` and decode accordingly; when the
HTTP header omits a charset on these `_files/sheet001.htm` payloads, decode as windows-1252
rather than defaulting to UTF-8. Do not assume the HTTP header is authoritative.
