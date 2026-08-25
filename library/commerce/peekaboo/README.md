# Peekaboo CLI

Peekaboo Guru deals, discounts, branches, and bank-card offers CLI

Created by [@qazmataz](https://github.com/qazmataz) (qazmataz).

## Install

The recommended path installs both the `peekaboo-pp-cli` binary and the `pp-peekaboo` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press install peekaboo
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install peekaboo --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press install peekaboo --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press install peekaboo --agent claude-code
npx -y @mvanhorn/printing-press install peekaboo --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/peekaboo/cmd/peekaboo-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/peekaboo-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press install peekaboo --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-peekaboo --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-peekaboo --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press install peekaboo --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/peekaboo-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. No token is required: the MCP server bootstraps Peekaboo's public guest token automatically. You may provide `PEEKABOO_TOKEN` to override it with your own credential.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/peekaboo/cmd/peekaboo-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "peekaboo": {
      "command": "peekaboo-pp-mcp",
      "env": {}
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Optional Credentials

No credential setup is required for the public guest flow. Authenticated commands bootstrap and cache Peekaboo's public guest token automatically. To use your own credential instead, store it:

```bash
peekaboo-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set it via environment variable:

```bash
export PEEKABOO_TOKEN="your-token-here"
```

### 3. Verify Setup

```bash
peekaboo-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
peekaboo-pp-cli amenities --country example-value
```

## Usage

Run `peekaboo-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `PEEKABOO_CONFIG_DIR`, `PEEKABOO_DATA_DIR`, `PEEKABOO_STATE_DIR`, or `PEEKABOO_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `PEEKABOO_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export PEEKABOO_HOME=/srv/peekaboo
peekaboo-pp-cli doctor
```

Under `PEEKABOO_HOME=/srv/peekaboo`, the four dirs resolve to `/srv/peekaboo/config`, `/srv/peekaboo/data`, `/srv/peekaboo/state`, and `/srv/peekaboo/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "peekaboo": {
      "command": "peekaboo-pp-mcp",
      "env": {
        "PEEKABOO_HOME": "/srv/peekaboo"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `PEEKABOO_DATA_DIR` overrides an explicit `--home` for that kind. Use `PEEKABOO_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `PEEKABOO_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `peekaboo-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### amenities

Amenities offered by a merchant (dine-in, delivery, ...)

- **`peekaboo-pp-cli amenities`** - List amenities for a merchant.

### branches

Physical branches of a merchant, each with coordinates for directions

- **`peekaboo-pp-cli branches <entityId>`** - List all branches of a merchant with latitude/longitude for map directions.

### brands

Brands and bank-card sources offering deals in a city

- **`peekaboo-pp-cli brands`** - List brands and card-deal sources in a city.

### cards

Bank/card sources offering deals at a merchant

- **`peekaboo-pp-cli cards <entityId>`** - List the banks/card sources that offer deals at a merchant.

### categories

Deal categories (Food, Lifestyle, Health, ...)

- **`peekaboo-pp-cli categories`** - List deal categories for a city.

### deals

Deals/offers for a merchant (title, discount %, validity)

- **`peekaboo-pp-cli deals`** - List deals/offers for a merchant, including bank-card offers.

### locations

Cities and locations Peekaboo covers (each with coordinates and merchant count)

- **`peekaboo-pp-cli locations`** - List cities/locations Peekaboo covers. Public: works without auth.

### places

Merchants/places offering deals in a city+category

- **`peekaboo-pp-cli places detail`** - Get full detail for a merchant (social, menu, gallery, rating, stats).
- **`peekaboo-pp-cli places list`** - List merchants offering deals in a city and category.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`peekaboo-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`peekaboo-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`peekaboo-pp-cli learnings list`** - Inspect taught rows
- **`peekaboo-pp-cli learnings forget <query>`** - Undo a teach
- **`peekaboo-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`peekaboo-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`peekaboo-pp-cli teach-pattern`** - Install a query/resource template up front
- **`peekaboo-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `PEEKABOO_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `peekaboo-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
peekaboo-pp-cli amenities --country example-value

# JSON for scripting and agents
peekaboo-pp-cli amenities --country example-value --json

# Filter to specific fields
peekaboo-pp-cli amenities --country example-value --json --select id,name,status

# Dry run — show the request without sending
peekaboo-pp-cli amenities --country example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
peekaboo-pp-cli amenities --country example-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - previous live reads can be reused from the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
peekaboo-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `peekaboo-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/peekaboo-pp-cli/config.toml`; `--home`, `PEEKABOO_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `PEEKABOO_TOKEN` | per_call | No | Optional override; the CLI bootstraps Peekaboo's public guest token when absent. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `peekaboo-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `peekaboo-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $PEEKABOO_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
