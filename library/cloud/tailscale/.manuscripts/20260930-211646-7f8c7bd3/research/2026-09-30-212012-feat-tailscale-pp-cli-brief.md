<!-- slop-gate: off -->
# Tailscale CLI Brief

## API Identity
- Domain: Tailscale control-plane admin API v2, base `https://api.tailscale.com/api/v2`. Official OpenAPI 3.1.0 spec served at `https://api.tailscale.com/api/v2?outputOpenapiSchema=true` (247 KB, 93 operations across Devices, DeviceInvites, UserInvites, Logging, DNS, Keys, PolicyFile, DevicePosture, Users, Contacts, Webhooks, TailnetSettings, Services, OAuthApps, Organizations). The spec header warns the spec itself is unstable, endpoints are stable.
- Users: people who run a tailnet (solo operators with a handful of Macs up to IT admins), and agents acting for them. The local `tailscale` command only controls the node it runs on; admin work (route approval, exit nodes, policy file, sharing, key expiry, auth keys, users) lives in the admin console or this API.
- Data profile: small (tens to low hundreds of devices, users, keys). No pagination: list endpoints return everything. Policy file is a single HuJSON document with an ETag.

## Reachability Risk
- None. Live read-only probe on 2026-09-30 with a user API access token: `GET /tailnet/-/devices` returned 200 with 8 devices. No pagination, no published rate limits; back off on 429.
- Auth: `bearerAuth` (HTTP bearer). API access tokens `tskey-api-*` (1-90 day expiry, user-scoped, created at https://login.tailscale.com/admin/settings/keys) or trust credentials: OAuth client `tskey-client-*` exchanged at `POST /api/v2/oauth/token` (form `client_id`, `client_secret`, `grant_type=client_credentials`) for a short-lived bearer token with scopes. Basic auth (token as username) also accepted.
- Tailnet path param: `-` means the token's default tailnet.
- OAuth-derived tokens cannot create/resend/accept device invites (spec note); user invites need a user-owned key.

## Top Workflows
1. Approve / unapprove subnet routes and exit nodes on one device without clobbering other enabled routes. `POST /device/{id}/routes` replaces the entire enabled set, so a naive call drops routes. Exit node = the pair `0.0.0.0/0` + `::/0`.
2. Edit the policy file safely: GET with `Accept: application/hujson` so comments survive, back it up locally, compute a diff, `POST /tailnet/-/acl/validate`, then `POST /tailnet/-/acl` with `If-Match: <ETag>` (412 on mismatch). Typical adds: `nodeAttrs` entries (Taildrive `drive:share` / `drive:access`, funnel) and `grants` (Taildrive app cap, contractor access).
3. Key-expiry report: every device's `expires`, days left, `keyExpiryDisabled`, flag devices inside N days. Read-only.
4. Machine sharing: list share invites per device (`GET /device/{id}/device-invites`), see accepted state and acceptor, revoke (`DELETE /device-invites/{id}`).
5. Inventory and audit: devices (filter by tag/os/lastSeen/updateAvailable), keys (expiring auth keys and API tokens), users, configuration audit log.

## Table Stakes
- Full endpoint mirror of all 93 operations (tscli covers most; YawLabs MCP has 97 tools).
- Both auth paths: API access token and OAuth client credentials, env vars `TAILSCALE_API_KEY`, `TAILSCALE_OAUTH_CLIENT_ID`, `TAILSCALE_OAUTH_CLIENT_SECRET`, `TAILSCALE_TAILNET` (same names YawLabs MCP and the Terraform provider use; tscli uses `TAILSCALE_API_KEY` + `TAILSCALE_TAILNET`).
- `--json` output, policy get as HuJSON or JSON, validate/preview, device filters (`fields=all`, server-side `<field>=<value>` filters).
- Retry on 429/502/503/504 for idempotent methods only; never retry POST.

## Data Layer
- Primary entities: devices (with routes when `fields=all`), users, keys, device invites, DNS config, tailnet settings, policy-file snapshots (local backups with ETag + timestamp).
- Sync cursor: none needed; full refresh is cheap (no pagination, small cardinality).
- FTS/search: devices by hostname/name/tags/user/os; users by login/display name.

## Codebase Intelligence
- `github.com/tailscale/hujson` (BSD-3, Tailscale) parses HuJSON into an AST that preserves comments and supports RFC 6902 JSON Patch (`Value.Patch`) and `Format()`. This gives comment-preserving inserts into `nodeAttrs` / `grants` without hand string surgery.
- The local `tailscale status --json` `ID` field (per peer and Self) equals the API's device `nodeId`, so local status output can resolve API device IDs without a round trip.

## User Vision
- From Cathryn's brief (2026-09-30): live-testing tailscale-superpowers needed a browser session and a hand-written Python script for admin jobs. The CLI must make these repeatable and safe for agents.
- Lessons to build in: (1) HuJSON GET + local backup + validate + If-Match ETag + show added/removed lines before any policy write; (2) routes POST replaces the whole set, read `enabledRoutes` first and send the remainder; (3) `tailscale status --json` ID == API nodeId; (4) read-only key-expiry report with N-day flag.
- Novel commands requested: `devices expiry`, `routes approve` / `routes unapprove` preserving other routes, `policy add-entry` for nodeAttrs and grants (validate, ETag, backup, diff), `policy restore <backup>`, machine share list and revoke (API supports it via device-invites).
- Safety: every mutation supports `--dry-run`; in agent mode mutations default to dry-run unless explicitly confirmed. Never print tokens. Keep real device names, IPs, tailnet names, usernames out of committed files and fixtures.

- Follow-up from Cathryn (2026-09-30, mid-run): "create a CLI, novel commands for agent friendly networking". Weight novel features toward what an AI agent needs to reason about and safely change tailnet networking: which device can reach which (policy preview), which routes/exit nodes are advertised but unapproved, who has access to a shared machine, what changed recently, and resolving local `tailscale status --json` IDs to API devices. Agent mode must default mutations to dry-run, with structured plans an agent can show a human before applying.

## Product Thesis
- Name: tailscale-pp-cli
- Why it should exist: tscli and the YawLabs MCP mirror endpoints; neither makes the two dangerous whole-object writes (routes, policy file) safe by construction. This CLI does read-modify-write with preservation, backup, validation, ETag concurrency, diffs and dry-run defaults for agents, plus an expiry report and share audit the admin console only shows per device.

## Build Priorities
1. Generate the full 93-operation mirror from the official spec with bearer + OAuth client-credentials auth and `-` default tailnet.
2. `routes approve|unapprove|list` with read-modify-write and `--exit-node` shorthand; dry-run default in agent mode.
3. `policy get|backup|diff|add-entry|restore` on HuJSON with `tailscale/hujson`, validate, If-Match ETag, local backups.
4. `devices expiry` report (days left, `--within N`, exit code when anything is inside the window).
5. `shares list|revoke` over device-invites across all devices; local sync + search for devices/users/keys.
