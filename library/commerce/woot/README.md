# Woot CLI

Discovered API spec for d24qg5zsx8xdc4-cloudfront

Learn more at [Woot](https://d24qg5zsx8xdc4.cloudfront.net).

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

## Quick Start

### Search Woot

Set the Woot frontend GraphQL key first. You can capture a fresh value from a Woot browser session or provide one through the environment:

```bash
export D24QG5ZSX8XDC4_CLOUDFRONT_API_KEY="<paste-your-key>"
```

Then search current Woot offers:

```bash
woot-pp-cli deals rayon --limit 10000
woot-pp-cli deals rayon --from-url 'https://www.woot.com/alldeals?selectedCategories=sport&selectedPriceRanges=[0,24.99]-[25,49.99]&page=13'
woot-pp-cli deals --category sport --price-range under-25 --price-range 25-50 --page 13 rayon
woot-pp-cli deals --json --compact laptop
```

`deals` uses the same frontend `searchOffers` GraphQL call as Woot's All Deals page, then filters titles, slugs, and item attributes locally. Increase `--limit` to scan more paged All Deals results, or paste a filtered `/alldeals` URL with `--from-url` to reuse Woot's category, price, and page filters.

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

This CLI uses Woot's browser-facing GraphQL key. Capture a current value from your own Woot browser session and provide it with this environment variable:

```bash
export D24QG5ZSX8XDC4_CLOUDFRONT_API_KEY="<paste-your-key>"
```

To persist credentials, use `woot-pp-cli auth set-token <token>`. Stored secrets live in `credentials.toml` under the data directory, not in `config.toml`.

### 3. Verify Setup

```bash
woot-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
woot-pp-cli deals rayon --limit 10000
```

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

### graphql

Run read-only Woot GraphQL queries.

- **`woot-pp-cli graphql`** - Fetch one current All Deals offer with a default `searchOffers` query.
- **`woot-pp-cli graphql --query '<query>'`** - Run a custom read-only Woot GraphQL query. Mutation and subscription documents are rejected.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
woot-pp-cli graphql

# JSON for scripting and agents
woot-pp-cli graphql --json

# Custom read-only GraphQL query
woot-pp-cli graphql --query '{ searchOffers(Filter:{}, Sort:BestSelling, Limit:1, Skip:0){ TotalHits } }' --json

# Filter to specific fields
woot-pp-cli graphql --json --select id,name,status

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
- Verify the environment variable is set: `echo $D24QG5ZSX8XDC4_CLOUDFRONT_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

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
