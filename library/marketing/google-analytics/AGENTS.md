# google-analytics Printing Press CLI

This CLI is GA4-only. Do not add Search Console endpoints here; use `google-search-console-pp-cli` for GSC.

## Auth

Use a Google service account JSON key with `https://www.googleapis.com/auth/analytics.readonly`. Named selection is `--account`, `GOOGLE_ANALYTICS_ACCOUNT`, then saved default. `auth add <name> --from-credentials <file>` saves a 0600 copy; each invocation mints a fresh token. `auth accounts`, `auth set-default`, and `auth remove` manage saved keys. Named accounts ignore ambient `GOOGLE_APPLICATION_CREDENTIALS`.

The legacy `default` account still resolves `--credentials` before `GOOGLE_APPLICATION_CREDENTIALS`, with no implicit local fallback. Explicit credential paths bypass saved defaults and conflict with account selectors. Keep diagnostics free of private keys and tokens.

Property resolution for data commands is `--property`, then `GA4_PROPERTY_ID`. Do not hard-code brand property IDs in command implementations. Fleet checks can pass `health --properties <comma-list>` or set `GA4_PROPERTY_IDS`.

## Required validation

- `go test ./...`
- `go build ./...`
- `google-analytics-pp-cli agent-context --agent`
- `google-analytics-pp-cli health --properties $GA4_PROPERTY_IDS --agent` when credentials/property grants are available
- Live the first authorized property smoke: `channels`, `compare`, `whats-changed`, `funnel`
