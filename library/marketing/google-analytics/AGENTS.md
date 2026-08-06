# google-analytics Printing Press CLI

This CLI is GA4-only. Do not add Search Console endpoints here; use `google-search-console-pp-cli` for GSC.

## Auth

Two credential shapes are supported and detected by `ga4.Credentials.Kind()`:

- `service_account` — `client_email` + `private_key`; mints an RS256 JWT for the requested scope.
- `authorized_user` — `client_id` + `client_secret` + `refresh_token` (ADC); exchanges the refresh token. **Do not send a scope on that request** — the original grant fixes it, and the CLI must not imply otherwise.

Resolution order:

1. `--credentials`
2. `GOOGLE_ANALYTICS_ADC`
3. `GOOGLE_APPLICATION_CREDENTIALS`
4. No implicit local fallback. Never hard-code a credential path — in particular, do not port the developer-local ADC path from any scratch script.

Scopes: reads use `ga4.AnalyticsReadonlyScope` via `flags.newClient()`; writes use `ga4.AnalyticsEditScope` via `flags.newWriteClient()`. Clients are cached per scope.

Property resolution for data commands is `--property`, then `GA4_PROPERTY_ID`. Do not hard-code brand property IDs in command implementations. Fleet checks can pass `health --properties <comma-list>` or set `GA4_PROPERTY_IDS`.

## Destructive-write gate

Every `delete` and both `archive` subcommands must call `requireExplicitYes(cmd, action)` before doing anything else.

That helper checks `cmd.Flags().Changed("yes")` and **must never be rewritten to read `rootFlags.yes`**: `--agent` sets that field to true, so reading it would let agent mode authorize destruction on its own. New destructive commands must reuse the helper rather than rolling their own check.

## Admin API versions

`Client.AdminBase` is v1beta and `Client.AdminAlphaBase` is v1alpha. `Client.AlphaBase` is the **Data** API alpha host used by `funnel` — do not reuse it for Admin calls.

`accessBindings` exists only in v1alpha; v1beta answers 404 with an HTML page, not JSON. That is why `admin` exposes `--api`.

## Required validation

- `go test ./...`
- `go build ./...`
- `google-analytics-pp-cli agent-context --agent`
- `google-analytics-pp-cli health --properties $GA4_PROPERTY_IDS --agent` when credentials/property grants are available
- Live the first authorized property smoke: `channels`, `compare`, `whats-changed`, `funnel`
- Gate smoke (no credentials needed, must exit non-zero without touching the network):
  `google-analytics-pp-cli key-events delete --name properties/1/keyEvents/x --agent`
- Live write smoke when an `analytics.edit` grant is available: `key-events list`, then
  `key-events create` against an event that already exists and confirm `409 ALREADY_EXISTS`
  rather than creating a new resource
