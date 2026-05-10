# AGENTS.md

## Commands

```bash
# Build
go build -o zohomail-pp-cli .

# Install locally
go install .

# Run tests
go test ./...

# Run a single test
go test -run TestAccountsUsesZohoAuthHeader ./...
```

## Architecture

Single-package Go CLI (`package main`, `main.go`). No external dependencies — stdlib only.

### Core types

- `config` — all runtime state: auth credentials, account/folder IDs, base URLs, output format, HTTP client. Populated in priority order: config file → env vars → CLI flags.
- `client` — thin wrapper around `config` that carries out HTTP calls to the Zoho Mail REST API.
- `apiEnvelope` — Zoho API response shape `{"status":…,"data":…}`.

### Entrypoint flow

`main()` → `run(args, stdout, stderr)` — the `run` function is the testable entrypoint. Tests call `run(...)` directly against an `httptest.Server`.

Command dispatch is a `switch` on `args[0]` inside `run`. Each case parses its own flags with a `flag.FlagSet`.

### Config loading

`configFromEnv()` loads in this order:
1. File at `$PP_ZOHOMAIL_CONFIG` or `~/.config/zohomail-pp-cli/config.json`
2. Env vars (override file): `ZOHO_MAIL_ACCESS_TOKEN`, `ZOHO_CLIENT_ID`, `ZOHO_CLIENT_SECRET`, `ZOHO_REFRESH_TOKEN`, `ZOHO_ACCOUNT_ID`, `ZOHO_INBOX_FOLDER_ID`, etc.
3. Global flags (`--config`, `--output`, `--account-id`) applied after parsing.

### Auth

`client.bearerToken()` implements token resolution:
- Returns `config.AccessToken` directly if set.
- Otherwise calls `client.tokenRequest` to exchange the refresh token for a short-lived access token.

### Output

`writeFormatted(w, body, output)` — `pretty` mode extracts `.data`, pretty-prints arrays as tab-separated rows via `printRows`; `json` mode writes raw response body.

### Commands implemented

| Command | What it does |
|---|---|
| `doctor` | Print config and auth state without revealing secrets |
| `configure` | Discover and save account/folder defaults (or set offline) |
| `accounts` | List Zoho Mail accounts |
| `folders` | List folders for the configured account |
| `inbox` / `sent` / `archive` / `spam` / `trash` | List messages in that folder |
| `list --folder <name>` | List messages in a named folder |
| `search --search-key <query>` | Search messages |
| `read --folder <name> --message-id <id>` | Read message content or details |
| `send` | Send an email |
| `login` | Browser OAuth flow; listens on `localhost:53682/callback` |
| `client-setup` | Print redirect URI/scopes for Zoho API Console setup |
| `auth-save` | Save client credentials + refresh token without browser |
| `auth-rbw` | Load credentials from Bitwarden via `rbw` |
| `auth-url` | Print OAuth authorization URL |
| `token` | Exchange authorization code for tokens |
| `auth-client-credentials` | Client credentials flow (access token only, 1h TTL) |
| `auth-device` | Device authorization flow (Non-browser Application client only) |

### Zoho API base URLs

Default: `https://mail.zoho.com` / `https://accounts.zoho.com`. Override for non-US data centers via `ZOHO_MAIL_BASE_URL` / `ZOHO_ACCOUNTS_BASE_URL`.
