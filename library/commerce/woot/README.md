# Woot CLI

**Search Woot's live All Deals catalog with filters, prices, offline full-text search, and read-only GraphQL access.**

Find offers that are buried deep in Woot's paginated All Deals catalog, then save a locally searchable snapshot with explicit completeness metadata. The CLI exposes the browser-observed searchOffers query without allowing GraphQL mutations.

Learn more at [Woot All Deals](https://www.woot.com/alldeals).

Created by [@TheFabulousMoolah](https://github.com/TheFabulousMoolah) (Matthew Vassallo).

## Install

The recommended path installs both the `woot-pp-cli` binary and the `pp-woot` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install woot
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install woot --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install woot --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install woot --agent claude-code
npx -y @mvanhorn/printing-press-library install woot --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/woot-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install woot --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-woot --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-woot --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install woot --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/woot-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `D24QG5ZSX8XDC4_CLOUDFRONT_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "woot": {
      "command": "woot-pp-mcp",
      "env": {
        "D24QG5ZSX8XDC4_CLOUDFRONT_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Capture the x-api-key value from a successful graphql request made by your own Woot All Deals browser session, then provide it through D24QG5ZSX8XDC4_CLOUDFRONT_API_KEY or auth set-token.

## Quick Start

```bash
# Scan up to 10000 live result slots and return matching offers plus explicit scan-completeness metadata as compact JSON.
woot-pp-cli deals rayon --limit 10000 --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Live deal discovery
- **`deals`** — Scan Woot's paged All Deals results and filter live offers by keyword.

  _Use this when Woot's visible All Deals pages contain an offer that a single GraphQL page or direct offer lookup would miss; scan metadata reports short windows, duplicate IDs, and rows without identities._

  ```bash
  woot-pp-cli deals rayon --limit 10000 --agent
  ```

## Recipes

### Search a filtered slice of All Deals

```bash
woot-pp-cli deals laptop --category computers --price-range 50-100 --limit 500 --agent
```

Scan current computer deals priced from $50 to $100 and return title matches as compact JSON.

## Usage

Run `woot-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `WOOT_CONFIG_DIR`, `WOOT_DATA_DIR`, `WOOT_STATE_DIR`, or `WOOT_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `WOOT_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export WOOT_HOME=/srv/woot
woot-pp-cli doctor
```

Under `WOOT_HOME=/srv/woot`, the four dirs resolve to `/srv/woot/config`, `/srv/woot/data`, `/srv/woot/state`, and `/srv/woot/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "woot": {
      "command": "woot-pp-mcp",
      "env": {
        "WOOT_HOME": "/srv/woot"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `WOOT_DATA_DIR` overrides an explicit `--home` for that kind. Use `WOOT_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `WOOT_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `woot-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### deals

List current Woot All Deals offers and optionally filter them locally by keyword.

- **`woot-pp-cli deals [keyword]`** - Fetch current offers via Woot's All Deals GraphQL `searchOffers` call, scanning paged results up to `--limit`, then filter titles, slugs, and item attributes locally.
- **`woot-pp-cli deals [keyword] --from-url <woot-alldeals-url>`** - Reuse Woot's visible All Deals filters such as category, price range, and page number from a copied URL.

Machine output distinguishes raw rows (`scanned`) from rows remaining after duplicate removal (`unique_scanned`) and reports `duplicate_rows`, `missing_id_rows`, `expected_scan`, and `incomplete`. `incomplete` describes the requested live scan window, not whether `--limit` covered the entire catalog. Human table output prints a warning when that window is incomplete.

### graphql

Run read-only Woot GraphQL queries.

- **`woot-pp-cli graphql`** - Fetch one current All Deals offer with a default `searchOffers` query.
- **`woot-pp-cli graphql --query '<query>'`** - Run a custom read-only Woot GraphQL query. Mutation and subscription documents are rejected.

### sync and search

Build and query a local full-text index of current Woot offers.

- **`woot-pp-cli sync --full`** - Start at the head of All Deals, store normalized prices locally, and remove expired rows only after two consecutive full scans return the same complete set of deal IDs. If Woot changes between scans, existing rows are preserved and the local snapshot is marked incomplete so the next sync retries from the head.
- **`woot-pp-cli search '<query>' --type deals --data-source local`** - Search the synced offer catalog without making another Woot request.

Woot's offset feed can report more deals than one pass returns and can repeat IDs while the BestSelling order changes. The live `deals` command removes repeated IDs and marks the requested window incomplete; `sync` exits with an incomplete-snapshot warning instead of claiming success. Existing local matches and prices remain usable, but a missing local match is not authoritative; rerun sync later to attempt verification again.

`--no-prune` preserves local rows that are absent from a verified live snapshot. If any such rows remain, the CLI deliberately keeps the local store marked incomplete; run `sync --full` without `--no-prune` when you need an exact current ID set.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
woot-pp-cli graphql

# JSON for scripting and agents
woot-pp-cli graphql --json

# Custom read-only GraphQL query
woot-pp-cli graphql --query '{ searchOffers(Filter:{}, Sort:BestSelling, Limit:1, Skip:0){ TotalHits } }' --json

# Filter to specific fields
woot-pp-cli graphql --json --select data.searchOffers.TotalHits

# Dry run — show the request without sending
woot-pp-cli graphql --dry-run

# Agent mode — JSON + compact + no prompts in one flag
woot-pp-cli graphql --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
woot-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `woot-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/woot-pp-cli/config.toml`; `--home`, `WOOT_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `D24QG5ZSX8XDC4_CLOUDFRONT_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `woot-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `woot-pp-cli doctor` to check credentials
- Verify the environment variable is present without printing it: `test -n "$D24QG5ZSX8XDC4_CLOUDFRONT_API_KEY" && echo set || echo not-set`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **The API returns HTTP 401 or says a valid authorization header was not provided.** — Capture a fresh x-api-key from a successful Woot All Deals graphql request and set D24QG5ZSX8XDC4_CLOUDFRONT_API_KEY.
- **A local search returns no results or stale results.** — Run woot-pp-cli sync --full before searching the local deals index.
- **A full sync reports an incomplete snapshot.** — Woot returned duplicate, missing, or changing offset pages. Existing local matches remain useful, but rerun later before treating a missing match as authoritative.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://d24qg5zsx8xdc4.cloudfront.net/graphql
- Capture coverage: 7 API entries from 7 total network entries
- Reachability: standard_http (65% confidence)
- Protocols: graphql (92% confidence)
- Auth signals: api_key — headers: x-api-key
- Candidate command ideas: list_graphql — Derived from observed GET /graphql traffic.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
