# Google Health API CLI — Research Brief

Run ID: 20260619-180224
API: Google Health API (v4)
Spec: derived from Google's official v4 discovery document, rebuilt as Swagger 2.0 (`spec.json`)
Category: devices

## Source

Google publishes a machine-readable **discovery document** (not OpenAPI) for the
Google Health API at
`https://health.googleapis.com/$discovery/rest?version=v4` (265 KB, 25 methods,
141 schemas). Base URL `https://health.googleapis.com`. The surface is two
resource trees: `users` (data points across ~38 data types, profile, settings,
identity, IRN profile, paired devices) and `projects` (webhook subscribers and
subscriptions).

The Google Health API launched in 2026 as the **successor to the deprecated
Fitbit Web API**, unifying health and fitness data from Fitbit, Pixel Watch, and
third-party devices on Google's OAuth 2.0 infrastructure.

## Spec rebuild (provenance)

The printing-press generator consumes OpenAPI, not Google's discovery format.
The off-the-shelf `api-spec-converter` produced valid Swagger 2.0 but collapsed
every operation onto Google's reserved-expansion API templates
(`/v4/{+parent}/dataPoints`), from which the generator could not derive resource
names — yielding a 1-endpoint CLI. The spec was instead **rebuilt from the
discovery document's `flatPath` values** (e.g.
`v4/users/{usersId}/dataTypes/{dataTypesId}/dataPoints`) by a small converter
(`build-ghealth-spec.ps1`), reusing the 141 already-converted schema
definitions. Result: 9 resources / 25 endpoints with proper REST structure. The
OAuth security scheme was set to Google's `authorization_code` flow (authorize
`https://accounts.google.com/o/oauth2/auth`, token
`https://oauth2.googleapis.com/token`). This provenance is disclosed honestly:
the spec is derived from Google's official discovery doc, with paths rebuilt
from `flatPath`.

## Auth

OAuth2 authorization_code (3-legged browser flow) via Google OAuth 2.0. The
generated client persists and auto-refreshes tokens. The runtime bearer access
token is read from `GOOGLEHEALTH_OAUTH2C`. Webhook subscriber/subscription
routes under `projects` require a separate Google Cloud project identity and are
outside the default user-data read surface.

## Competitive landscape

The new Google Health API has almost no third-party tooling, and the full
local-sync + analytics + MCP pattern exists only for *other* vendors:

- davidmosiah/google-health-mcp — TypeScript MCP server, ~10 stars, the only
  third-party Google Health tool; live-read bridge with no durable datastore and
  no cross-metric analytics.
- Google official client libraries (health/v4, 7 languages) — bare auto-generated
  request/response bindings; no CLI, storage, analytics, or MCP.
- orcasgit/python-fitbit — 633 stars, abandoned since 2016 (predecessor API).
- veerendra2/fitbit-cli — ~10 stars; README explicitly declines to migrate to the
  Google Health API.
- bes-dev/garmy — Garmin, ~61 stars, sync+analytics+MCP but single-vendor and stale.
- drakulavich/oura-cli — Oura, ~3 stars, local SQLite + trends, MCP on roadmap.
- tcgoetz/GarminDB — Garmin, 3.2k stars, SQLite + rollups, no MCP.

No verified tool pairs the **new Google Health API** with a durable local SQLite
store, cross-data-type analytics, and an MCP server.

## Recommendation

Proceed. A durable, forward-looking API (the Fitbit successor), a clean
read-mostly surface, an auth model consistent with existing library OAuth entries,
and a clear novelty story: first to pair the new Google Health API with local
sync, cross-metric analytics, and MCP.
