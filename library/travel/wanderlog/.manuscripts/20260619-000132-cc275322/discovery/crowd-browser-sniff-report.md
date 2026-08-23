# Crowd-Sniff Report: Wanderlog

## npm Packages Analyzed

`cli-printing-press crowd-sniff --api wanderlog --base-url https://wanderlog.com` failed before producing structured provenance. Manual registry search found `wanderlog-mcp` and `@zaw_ye/wanderlog_mcp`; the maintained source-backed package is `wanderlog-mcp` v0.3.1.

## GitHub Repos Searched

Manual GitHub search found `shaikhspeare/wanderlog-mcp` and `danilden1/Wanderlog-to-KML` as relevant repositories. `shaikhspeare/wanderlog-mcp` was inspected with authenticated `gh api`; `danilden1/Wanderlog-to-KML` README was inspected with authenticated `gh api`.

## Endpoints Discovered

The crowd-sniff command discovered no endpoints and wrote no supplemental spec. Endpoint evidence comes from browser-sniff and manual source inspection instead.

## Base URL Resolution

Selected base URL: `https://wanderlog.com`. Evidence: browser-sniff capture, Wanderlog web app configuration, and `wanderlog-mcp` default `WANDERLOG_BASE_URL`.

## Auth Patterns Detected

`wanderlog-mcp` authenticates with a `connect.sid` browser session cookie supplied as `WANDERLOG_COOKIE`. Its HTTP transport sends the cookie in the `Cookie` header and uses `Origin`/`Referer` headers against `https://wanderlog.com`. Its mutation transport uses a ShareDB WebSocket at `/api/tripPlans/wsOverall/{tripKey}?clientSchemaVersion=2` with cookie auth.

## Parameter Name Evidence

Manual source inspection found parameters for guide search (`destination`, `geo_id`, `response_format`), guide fetch (`guide_key`, `day`, `response_format`), place search (`input`, `sessiontoken`, `location`, `radius`, `language`), place detail (`placeId`, `language`), and trip creation (`geoIds`, `startDate`, `endDate`, `privacy`, `title`).

## Coverage Summary

Crowd-sniff command coverage: 0 endpoints. Manual source and browser-sniff coverage: public geo autocomplete, place details/card data, places-list category pages, public guide/shared itinerary HTML extraction, comments/distinction/likes probes, authenticated trip home/list/get/create REST endpoints, and authenticated ShareDB JSON0 mutation workflows.

Command failure: `crowd-sniff: downloads API returned status 400`; `no endpoints discovered for "wanderlog"`.
