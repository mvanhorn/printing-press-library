# Oura CLI — Research Brief

Run ID: 20260618-214839
API: Oura Ring (Oura Cloud API V2)
Spec: official OpenAPI 3.1, vendored locally as `oura-spec.json`
Category: health

## Source

Oura publishes an official OpenAPI 3.1 document at
`https://cloud.ouraring.com/v2/static/json/openapi-1.34.json` (72 paths). The
spec covers the read-only personal-data surface: daily sleep, readiness,
activity, heart rate, workouts, sessions, SpO2, stress, resilience, VO2 max,
cardiovascular age, tags, ring configuration, and personal info.

## Spec correction (vendored)

The upstream spec ships a malformed server URL: `servers[0].url` is
`https://api.None.com` — a Python `None` leaked into Oura's generated FastAPI
document. Left as-is, a generated CLI bakes that as its default base URL and
DNS-fails on every call. The vendored `oura-spec.json` replaces it with the
documented host `https://api.ouraring.com`. The OAuth authorize/token URLs in
the spec's security scheme were already correct and were not modified.

## Auth

OAuth2 authorization_code (3-legged browser flow). The spec advertises both
`BearerAuth` and `OAuth2`; `auth_preference: OAuth2` is pinned because Oura
deprecated static personal access tokens in December 2025, so the simpler
bearer scheme the parser would otherwise pick is a dead path. Authorize URL
`https://cloud.ouraring.com/oauth/authorize`, token URL
`https://api.ouraring.com/oauth/token`, refresh tokens supported and
auto-refreshed by the generated client. Webhook subscription routes use a
separate `x-client-id` + `x-client-secret` header pair and are not the
default-generated surface.

## Competitive landscape

Existing Oura tooling is dominated by stateless client libraries, not
terminal-native CLIs with a local data layer:

- turing-complet/python-ouraring — Python library, 140 stars
- hedgertronic/oura-ring — Python library, 84 stars
- Pinta365/oura_api — TypeScript library, 40 stars
- arzzen/oura — bash CLI, 9 stars (thin curl wrapper)
- ruhrpotter/oura-cli — Go CLI, 4 stars (thin V2 reader)

None offer a local SQLite mirror, full-text search, local analytics, or an MCP
server. That gap is the basis for this CLI's transcend layer.

## Recommendation

Proceed. Official spec, clean read-only surface, an auth model consistent with
existing library OAuth entries (spotify, zoho-expense, toodledo, figma,
pipedrive), and a clear novelty story over the existing stateless wrappers.
