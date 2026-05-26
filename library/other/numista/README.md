# Numista CLI

**Every Numista catalogue and collection feature, with offline FTS5 search and a monthly-quota tracker no Numista SDK has.**

This CLI wraps the Numista REST API in a Go single binary, persists every type, issuer, mint, catalogue, and collected item into a local SQLite store, and tracks the 2000-call monthly free-plan quota client-side so batches never run blind. Commands like `types series`, `collection value`, and `crawl issuer` only exist because the local cache lets the CLI compose dozens of calls into one quota-aware operation.

Printed by [@vinnyp](https://github.com/vinnyp) (Vinny Pasceri).

## Install

The recommended path installs both the `numista-pp-cli` binary and the `pp-numista` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install numista
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install numista --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install numista --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install numista --agent claude-code
npx -y @mvanhorn/printing-press-library install numista --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/numista/cmd/numista-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/numista-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-numista --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-numista --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-numista skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-numista. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/numista-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `NUMISTA_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "numista": {
      "command": "numista-pp-mcp",
      "env": {
        "NUMISTA_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Set `NUMISTA_API_KEY` in your environment (request one at https://en.numista.com/api/index.php — the free plan is 2000 calls/month). Catalogue and reference endpoints work with the API key alone. User-collection commands (`users collected-items add`, `collection value`, `users collections hydrate`) need an OAuth token — request an OAuth bearer with `numista-pp-cli oauth-token --grant-type client_credentials --scope view_collection` and save it via `numista-pp-cli auth set-token <token>` to grant the CLI access to your own account; the token is stored at ~/.numista-pp-cli/auth.json (mode 0600).

## Quick Start

```bash
# Confirm the API key is set, the network is reachable, and the local store is initialized.
numista-pp-cli doctor

# Local SQL view over the lookup_log table: every API call you have made this month, grouped by endpoint. Zero API cost. Pair with the root `--quota` flag (e.g. `numista-pp-cli --quota`) to see this month's used/remaining.
numista-pp-cli audit --by-endpoint --json

# Find a type by free-text search; one API call, result cached.
numista-pp-cli types search --q 'Australia 3 pence George VI' --json

# Fetch full type details; cached on subsequent runs.
numista-pp-cli types get 11013 --json

# Pull every year of issue + every grade's price for one type in one quota-aware fan-out.
numista-pp-cli types series 11013 --json

# Grant the CLI access to your own collection; needed once for user-scoped commands.
numista-pp-cli oauth-token --grant-type client_credentials --scope view_collection

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Quota economics
- **`--quota`** — Print the current month's used/remaining/reset Numista quota and exit, with zero API calls.

  _Reach for this before any batch or crawl to know whether the operation fits today's budget — zero quota cost._

  ```bash
  numista-pp-cli --quota --json
  ```
- **`audit`** — Query the local lookup_log directly to see every API call, its endpoint, duration, and cache-hit status — aggregated by day, endpoint, or type ID.

  _Use this to diagnose unexpected quota consumption, find duplicate lookups worth caching, or audit which endpoints power which workflows._

  ```bash
  numista-pp-cli audit --by-endpoint --json
  ```
- **`types batch`** — Parse a CSV / JSONL / text list of Numista type IDs (N# numbers) and look up each one against the API — with cache reuse, --dry-run cost forecast, and --resumable splitting across UTC months.

  _Use any time you have more than one type ID to look up. Dry-run is always cheap; resumable is essential for >2K-item lists._

  ```bash
  numista-pp-cli types batch --file ./type-ids.csv --dry-run --json
  ```

### Local state that compounds
- **`types series`** — For one type (e.g., N#11013 — Australia 3 pence George VI), pull every year-of-issue, every issue's prices across all grades, and persist the full price/mintage curve to the local store — then print the scarcity and price-evolution table.

  _Reach for this when a user wants the full picture of one coin type — every year it was struck, every grade's current value — in one command instead of dozens of individual calls._

  ```bash
  numista-pp-cli types series 11013 --json --select issues.year,issues.mintage,prices.grade,prices.price
  ```
- **`collection value`** — Sum the current estimated value of every item in a user's Numista collection, fetching missing prices on demand, and emit a per-item breakdown sorted by value.

  _Use to answer 'what is my collection worth right now?' without scrolling through Numista's web UI. Refuses to start when remaining quota is less than the number of items needing fresh prices._

  ```bash
  numista-pp-cli collection value 12345 --json
  ```
- **`refresh`** — Refresh cached types by re-fetching only fields that actually change (prices, mintage) while leaving cataloger-set identity fields untouched. --dry-run --older 30d lists what needs refresh without spending a call.

  _Use weekly or monthly to keep cached prices current without re-pulling the entire catalogue. --dry-run --older makes the cost predictable._

  ```bash
  numista-pp-cli refresh --all --older 30d --json
  ```
- **`crawl issuer`** — Crawl every type from one issuer (e.g., 'australia') matching a year range, persist to local store, and print a summary table. Forecasts call cost as %-of-monthly-quota and requires confirmation before crawling.

  _Reach for this when starting research on an issuer or period — pay the call cost once, then ask the local store any question for free._

  ```bash
  numista-pp-cli crawl issuer australia_section --years 1900-1950 --dry-run
  ```
- **`watchlist`** — Track price changes for a set of types over time. `watchlist add N#` registers a type; `watchlist check` refreshes prices, snapshots them to the prices table with a timestamp, and prints the diff since the last snapshot.

  _Run on a cron after adding a few types; the CLI surfaces material price moves without polluting your inbox._

  ```bash
  numista-pp-cli watchlist check --json
  ```

### Agent-native plumbing
- **`users collected-items add --from-file`** — Import a list of new collected items from CSV / JSONL with --dry-run cost forecast. Idempotent on (user_id, type_id, issue_id, grade) so re-running the same file is safe.

  _Use to migrate a collection from another tracker, or to bulk-add a haul from a coin show without dozens of UI clicks._

  ```bash
  numista-pp-cli users collected-items add 12345 --from-file imports.csv --dry-run
  ```
- **`users collections hydrate`** — Given a collection-folder-id, fan out get-item for every item, optionally fan out get-prices, and persist everything to the local store. Refuses to start when remaining quota is less than the item count.

  _Use once after `auth set-token` (with an OAuth bearer) to populate the local store with everything in your collection — then most subsequent reads are quota-free._

  ```bash
  numista-pp-cli users collections hydrate 12345 --with-prices --json
  ```

## Usage

Run `numista-pp-cli --help` for the full command reference and flag list.

## Commands

### catalogues

The API endpoints in this section allow to access the data of the Numista catalogue of coins, banknotes and exonumia.

- **`numista-pp-cli catalogues`** - Retrieve the list of all the reference catalogues used for cross-reference in the catalogue

### issuers

Manage issuers

- **`numista-pp-cli issuers`** - Retrieve the details about all the issuing countries and territories

### mints

Manage mints

- **`numista-pp-cli mints get`** - Retrieve the details about all the mints
- **`numista-pp-cli mints get-mintid`** - Retrieve the details about a specific mint.

### oauth-token

Manage oauth token

- **`numista-pp-cli oauth-token`** - In order to access the data of a Numista user, you will need to authenticate using the OAuth 2.0 protocol.
See the section "Authentication" above.

Two types of authentications are available: authorization code and client credentials.

For the "authoriation code" flow, call the endpoint `/oauth_token` with the following parameters:
- `grant_type` is "authorization_code".
- `code` is the code you received in the query string of the back redirection described above.
- `client_id` is the client ID which was assigned to your application and provided together with your API key.
- `client_secret` is your API key.
- `redirect_uri` is the redirection URI you specified for the step described above.

For the "client credentials" flow, call the endpoint `/oauth_token` with the following parameters:
- `grant_type` is "client_credentials".
- `scope` is a comma-separated list of permissions you are requesting (e.g. 'view_collection').

You may use the resulting access token for all subsequent API calls which need user authentication. 
The access token should be provided in the HTTP header of the subsequent API calls according the following model:

`Authorization: Bearer {access_token}`

The access token has a limited validity period. The lifetime of the access token is indicated in the response of the API.

### publications

Manage publications

- **`numista-pp-cli publications <id>`** - Retrieve the details about a specific item in the literature catalogue.

### types

Manage types

- **`numista-pp-cli types get`** - Retrieve the details about a specific type in the catalogue.
- **`numista-pp-cli types search`** - Search the catalogue for coin, banknote, and exonumia types. At least one of the following parameters should be provided: `q`, `issuer`, `catalogue`, `date`, or `year`.

### users

The API endpoints in this section allow to access data about the Numista users and their collection.

- **`numista-pp-cli users <user_id>`** - Get details about a user

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
numista-pp-cli catalogues

# JSON for scripting and agents
numista-pp-cli catalogues --json

# Filter to specific fields
numista-pp-cli catalogues --json --select id,name,status

# Dry run — show the request without sending
numista-pp-cli catalogues --dry-run

# Agent mode — JSON + compact + no prompts in one flag
numista-pp-cli catalogues --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
numista-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/numista-documentation-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `NUMISTA_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `numista-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $NUMISTA_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **HTTP 401 'The Numista API key is missing or incorrect'** — Set `export NUMISTA_API_KEY=your-key` (get one at https://en.numista.com/api/index.php) and re-run.
- **HTTP 429 'You sent too many simultaneous requests or you reached the limit of your monthly quota'** — Run `numista-pp-cli --quota` to see used vs remaining. Wait until the first of next month UTC for reset; or use cached data with `--cached` on read commands.
- **HTTP 403 on a user-collection command** — Run `numista-pp-cli oauth-token --grant-type client_credentials --scope view_collection` (or `edit_collection` for write) to grant the CLI an OAuth token for your account.
- **`types search` returns nothing for a term that works on numista.com** — The web search uses fuzzy matching the API does not. Try narrower keywords or filter by `--issuer <code>` (run `numista-pp-cli issuers list` to find codes).
- **Batch import / crawl seems to skip items mid-run** — Check `numista-pp-cli --quota` — you may have hit the 2000/month cap. Resume next month with `--resumable --checkpoint ./numista.ckpt`.

---

## Cookbook

Real workflows distilled into copy-paste-ready commands. Each one exercises a different novel feature.

### Identify a coin from a fuzzy description

```bash
numista-pp-cli types search --q 'Australia 3 pence George VI' --json --select types.id,types.title,types.min_year,types.max_year
```

Narrow with `--select` to keep the payload tight; the dotted-path projection turns ~50 lines of JSON per result into 4.

### See what a coin is worth in every grade

```bash
numista-pp-cli types series 11013 --json
```

One command pulls every year + every grade's price into the local store; subsequent reads are free.

### Estimate this month's batch cost before running it

```bash
numista-pp-cli types batch --file ./watchlist.csv --dry-run --json
```

`--dry-run` forecasts live calls vs cache hits vs %-of-quota; never spends a call.

### Refresh only stale prices in your cache

```bash
numista-pp-cli refresh --all --older 30d --json
```

Re-fetches only types whose prices are older than 30 days; never touches identity fields.

### Total a Numista user's collection value

```bash
numista-pp-cli collection value 12345 --json --select totals.estimated_value,totals.currency,items.id,items.grade,items.estimated_value
```

Pass your numeric Numista user ID as the positional arg. The output joins your collection against the prices table and refuses to start if remaining quota would be exceeded.

### Crawl every type for one issuer over a year range

```bash
numista-pp-cli crawl issuer australia_section --years 1900-1950 --dry-run --json
```

`--dry-run` first to see the call cost as a %-of-monthly-quota; drop `--dry-run` once the forecast looks reasonable. The fetched types persist to the local store, so subsequent SQL queries (`numista-pp-cli sql "..."`) cost nothing.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**namachieli/numista-api-sdk**](https://github.com/namachieli/numista-api-sdk) — Python
- [**@leopiccionia/numista-sdk**](https://github.com/leopiccionia/numista-sdk) — TypeScript
- [**numistalib**](https://numistalib.readthedocs.io/) — Python
- [**MihajloNesic/Numista**](https://github.com/MihajloNesic/Numista) — Java

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
