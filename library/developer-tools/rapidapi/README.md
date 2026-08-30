# Rapidapi CLI

RapidAPI Hub marketplace CLI - search APIs, browse categories & collections, inspect providers, and manage your account (subscriptions, favorites, notifications, workspace) via the hub's own GraphQL gateway

Learn more at [Rapidapi](https://rapidapi.com).

Created by [@SomSamantray](https://github.com/SomSamantray) (Som Samantray).

## Install

The recommended path installs both the `rapidapi-pp-cli` binary and the `pp-rapidapi` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install rapidapi
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install rapidapi --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install rapidapi --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install rapidapi --agent claude-code
npx -y @mvanhorn/printing-press-library install rapidapi --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/rapidapi/cmd/rapidapi-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/rapidapi-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install rapidapi --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-rapidapi --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-rapidapi --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install rapidapi --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/rapidapi-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `RAPIDAPI_CSRF_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/rapidapi/cmd/rapidapi-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "rapidapi": {
      "command": "rapidapi-pp-mcp",
      "env": {
        "RAPIDAPI_CSRF_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

The RapidAPI hub gateway authenticates with a **session cookie** (`rapidapi-context-id`) plus a **session-bound CSRF token** auto-fetched at request time — even public marketplace queries need the CSRF bootstrap.

Set the cookie (your RapidAPI user ID — find it in Chrome DevTools → Application → Cookies → `https://rapidapi.com` → `rapidapi-context-id`):

```bash
export RAPIDAPI_COOKIE="<rapidapi-context-id value>"
# or persist it
rapidapi-pp-cli auth login --cookie "<value>"
```

**Cloudflare-gated networks:** set the `cf_clearance` cookie (DevTools → Application → Cookies) too:

```bash
export RAPIDAPI_CLEARANCE="<cf_clearance value>"
# or
rapidapi-pp-cli auth login --clearance "<value>"
```

Account commands (`account whoami`, `account subscriptions`, `account notifications`) additionally require the browser session's HttpOnly cookies and return empty results without them.

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your API key from your API provider's developer portal. The key typically looks like a long alphanumeric string.

```bash
export RAPIDAPI_CSRF_TOKEN="<paste-your-key>"
```

### 3. Verify Setup

```bash
rapidapi-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
rapidapi-pp-cli categories --limit 10
```

## Usage

Run `rapidapi-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `RAPIDAPI_CONFIG_DIR`, `RAPIDAPI_DATA_DIR`, `RAPIDAPI_STATE_DIR`, or `RAPIDAPI_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `RAPIDAPI_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export RAPIDAPI_HOME=/srv/rapidapi
rapidapi-pp-cli doctor
```

Under `RAPIDAPI_HOME=/srv/rapidapi`, the four dirs resolve to `/srv/rapidapi/config`, `/srv/rapidapi/data`, `/srv/rapidapi/state`, and `/srv/rapidapi/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "rapidapi": {
      "command": "rapidapi-pp-mcp",
      "env": {
        "RAPIDAPI_HOME": "/srv/rapidapi"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `RAPIDAPI_DATA_DIR` overrides an explicit `--home` for that kind. Use `RAPIDAPI_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `RAPIDAPI_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `rapidapi-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Unique Features

These capabilities aren't available in any other tool for this API.

### Offline marketplace sync & search
- **`sync`** — Pull categories, collections, and top APIs into a local SQLite store with sync-state tracking; `search` then queries the cached data offline.

  _Choose this when you need repeated marketplace research without re-hitting the hub every time._

### Hub analytics with local aggregation
- **`analytics`** / **`stats`** / **`trends`** — Hub-wide metrics (APIs, users, traffic) plus per-day request/error aggregates computed locally (SUM, AVG, error rates, day-over-day deltas).

  _Choose this when you need usage trends or a hub pulse without a dashboard._

### Chrome TLS transport
- **`system csrf`** — The CLI's uTLS Chrome-fingerprint HTTP/2 transport passes the Cloudflare bot gate, so live GraphQL queries work from the terminal like a browser.

  _Choose this when direct HTTP to the hub gateway would otherwise 403._

## Commands

### account

Your RapidAPI account (requires login)

- **`rapidapi-pp-cli account notifications`** - List your notifications
- **`rapidapi-pp-cli account saved`** - List APIs you saved as favorites
- **`rapidapi-pp-cli account subscriptions`** - List your API subscriptions with plan details and status
- **`rapidapi-pp-cli account whoami`** - Show the active logged-in user, their entities, and tenant
- **`rapidapi-pp-cli account workspace`** - Show your workspace: owned APIs, subscribed APIs, metrics

### apis

Inspect individual marketplace APIs

- **`rapidapi-pp-cli apis`** - Show a single API's full detail: endpoints, versions, billing plans, rating, owner

### categories

Browse API categories

- **`rapidapi-pp-cli categories`** - List top-level marketplace categories with weights and descriptions

### collections

Browse curated API collections

- **`rapidapi-pp-cli collections list`** - List curated collections (Recommended, Popular, Free, AI-based)
- **`rapidapi-pp-cli collections show`** - Show a collection's detail and its APIs

### marketplace

Search the RapidAPI marketplace

- **`rapidapi-pp-cli marketplace`** - Search APIs across the marketplace with filters, facets, scores, and pagination

### metrics

Hub-wide marketplace statistics

- **`rapidapi-pp-cli metrics`** - Show public marketplace metrics: total APIs, users, consumers, API traffic

### users

Inspect marketplace users

- **`rapidapi-pp-cli users`** - Show a user profile and their published APIs


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`rapidapi-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`rapidapi-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`rapidapi-pp-cli learnings list`** - Inspect taught rows
- **`rapidapi-pp-cli learnings forget <query>`** - Undo a teach
- **`rapidapi-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`rapidapi-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`rapidapi-pp-cli teach-pattern`** - Install a query/resource template up front
- **`rapidapi-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `RAPIDAPI_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `rapidapi-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Cookbook

Real-world recipes (flags verified against `--help`):

```bash
# Find free weather APIs ranked by popularity
rapidapi-pp-cli marketplace search weather --category Weather --limit 10 --json --select id,name,score

# Quick search without filters, then cache results for offline use
rapidapi-pp-cli search --term linkedin --limit 5

# Explore a curated collection
rapidapi-pp-cli collections show recommended-apis --json

# Get the full spec of a specific API (endpoints, plans, rating)
rapidapi-pp-cli apis meteostat/meteostat --with-endpoints

# Hub-wide pulse check
rapidapi-pp-cli metrics --from 2026-01-01 --to 2026-12-31 --json

# Your subscriptions and notifications (needs session cookie)
rapidapi-pp-cli account subscriptions --status ACTIVE
rapidapi-pp-cli account notifications --limit 5

# Traffic analytics with local aggregation
rapidapi-pp-cli account analytics --from 2026-08-01 --to 2026-08-28
rapidapi-pp-cli trends --days 14

# Export your cached API research to CSV for a spreadsheet
rapidapi-pp-cli export --resource api --format csv --out apis.csv

# CI-friendly health gate
rapidapi-pp-cli doctor --fail-on error
```

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
rapidapi-pp-cli categories --limit 10

# JSON for scripting and agents
rapidapi-pp-cli categories --limit 10 --json
# Filter to specific fields
rapidapi-pp-cli categories --limit 10 --json --select id,name,weight

# Dry run — show the request without sending
rapidapi-pp-cli categories --limit 10 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
rapidapi-pp-cli categories --limit 10 --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Explicit confirmation** - `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
rapidapi-pp-cli doctor
```

Checks config, auth, cache freshness, and gateway reachability. Use `--fail-on error` to exit non-zero on errors (CI-friendly), or `--fail-on stale` to also fail when the local cache is stale.

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `rapidapi-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/rapidapi-pp-cli/config.toml`; `--home`, `RAPIDAPI_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `RAPIDAPI_CSRF_TOKEN` | per_call | Yes | Set to your API credential. |
| `RAPIDAPI_COOKIE` | per_call | Yes | Set to your API credential. |
| `RAPIDAPI_CLEARANCE` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `rapidapi-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `rapidapi-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $RAPIDAPI_CSRF_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
