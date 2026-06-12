# google-analytics Printing Press CLI

This CLI is GA4-only. Do not add Search Console endpoints here; use `google-search-console-pp-cli` for GSC.

## Auth

Use a Google service account JSON key with `https://www.googleapis.com/auth/analytics.readonly`. Resolution order:

1. `--credentials`
2. `GOOGLE_APPLICATION_CREDENTIALS`
3. `/Users/knox/.agents/google-service-account.json` when present

Property resolution for data commands is `--property`, then `GA4_PROPERTY_ID`. Do not hard-code brand property IDs in command implementations. Fleet checks can pass `health --properties <comma-list>` or set `GA4_PROPERTY_IDS`.

## Required validation

- `go test ./...`
- `go build ./...`
- `google-analytics-pp-cli agent-context --agent`
- `google-analytics-pp-cli health --properties 280199692,540652239 --agent` when credentials/property grants are available
- Live BestSelf smoke: `channels`, `compare`, `whats-changed`, `funnel`
