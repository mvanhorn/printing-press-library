# Garmin Connect CLI — Research Brief

This brief was written at publish time from the run's `research.json` and the research notes in `spec.yaml`. The original run directory did not survive to the publish.

## API Identity

- Garmin Connect, the service behind Garmin watches and the Connect app. Garmin publishes no API for personal accounts and no OpenAPI document.
- Server: `https://connectapi.garmin.com`. The spec is hand-authored OpenAPI 3.1 with 27 operations, all `GET`.
- Every path was read from maintained community clients (python-garminconnect, matin/garth, tcgoetz/GarminDB, Pythe1337N/garmin-connect) on 2026-09-07. Each operation carries an `x-source` citation to the client file and line.
- Every path was then executed against two live Garmin accounts on 2026-09-07 and answered HTTP 200. Response shapes carry an `x-shape` marker: `partially-verified` on 15 response schemas, `unverified` on 3, wherever neither a client source nor a live row settled the keys.

## Auth

- Data calls send `Authorization: Bearer <DI access token>`.
- A Garmin SSO service ticket is exchanged for the token pair at `diauth.garmin.com/di-oauth2-service/oauth/token`, and the same URL refreshes it. These two calls sit outside the spec's server, so the CLI's `auth login` and its refresh hook implement them by hand.
- Login follows bpauli/gccli: a browser sign-in caught on an ephemeral loopback port. The tool never sees the password and never handles MFA or CAPTCHA.
- `DI-Backend: connectapi.garmin.com` belongs to python-garminconnect's cookie fallback. Neither reference client sends it with a bearer token, so the spec gives it no default.

## Reachability Risk

- Medium. The API is private and can change without notice; the login flow depends on Garmin's SSO pages. (rating assigned at publish time; `research.json` records no reachability field.)
- Daily-stats range endpoints reject requests longer than 28 calendar days. Weekly endpoints take a count of weeks instead (52 per request in garth).
- Every probe row on 2026-09-07 answered 200. The archive fill stops when Garmin rate-limits a request, and stops when every recent request has failed.

## Alternatives

| Project | Language | Shape | Gap |
|---|---|---|---|
| tcgoetz/GarminDB | Python | batch importer into a local database | no query surface an agent can call, no JSON output |
| cyberjunky/python-garminconnect | Python | library | an agent writes a script per question |
| matin/garth | Python | library | same |
| bpauli/gccli | Go | CLI with loopback login | no local store, no cross-series analytics |

## Patterns Carried Over

- python-garminconnect: chunk each daily-stats range into 28-day windows and de-duplicate by calendar date.
- garth: page size 28 for daily series and 52 for weekly series, as constants.
- gccli: loopback browser login on an ephemeral `127.0.0.1` port.
- GarminDB: a durable local store is what makes multi-year questions answerable.

## Data Layer

One SQLite archive per home. Every series lands in one `resources` table keyed by `(resource_type, id)`, so two series join with a self-join. Each series keeps one bookmark, which lets an interrupted fill resume.

## Product Thesis

Garmin caps every daily-stats request at 28 days and issues no personal API token, so the ecosystem is a handful of Python libraries and no tool an agent can call. A CLI that walks the whole account into a local archive once can answer months-long sleep, training-load and heart-rate-zone questions from disk, with JSON on stdout.

## Build Priorities

1. `auth login`: loopback browser sign-in that refuses to store a token for any account other than the one named.
2. `history`: oldest-first archive fill inside the 28-day cap, one bookmark per series.
3. `insights sleep` and `insights training`: multi-series readouts; sleep sets the prior window beside the current one.
4. `sql`: one read-only `SELECT` over the archive.

Recommendation recorded in `research.json`: proceed-with-gaps, novelty 8.
