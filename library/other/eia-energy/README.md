# Eia Energy CLI

Query metadata-aware US Energy Information Administration series.

Make EIA's large self-describing hierarchy useful from the terminal without hidden unit conversions, frequency mismatches, silent truncation, forecasts, or causal claims.

## Install

The recommended path installs both the `eia-energy-pp-cli` binary and the `pp-eia-energy` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install eia-energy
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install eia-energy --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install eia-energy --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install eia-energy --agent claude-code
npx -y @mvanhorn/printing-press-library install eia-energy --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/eia-energy/cmd/eia-energy-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/eia-energy-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install eia-energy --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-eia-energy --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-eia-energy --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install eia-energy --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/eia-energy-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `EIA_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/eia-energy/cmd/eia-energy-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "eia-energy": {
      "command": "eia-energy-pp-mcp",
      "env": {
        "EIA_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

```bash
# Inspect recent California grid measures and trailing baselines.
eia-energy-pp-cli grid pulse --respondent CISO --hours 24 --agent

# Create or compare a bounded source snapshot.
eia-energy-pp-cli revisions --route electricity/rto/region-data --facet respondent=CISO --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.
- **`grid pulse`** — Fetch recent balancing-authority measures and show per-type latest values, units, freshness, and transparent trailing means.
- **`state compare`** — Compare explicitly selected state series only when route, data column, frequency, periods, and units align.
- **`spread`** — Align two facet series by period and refuse subtraction when reported units differ.
- **`anomaly`** — Compute deterministic mean and standard-deviation deviations from exact bounded EIA source rows without forecasting.
- **`watch run`** — Evaluate a bounded JSON rule set once for thresholds, freshness, missing data, and changes while retaining source rows.
- **`revisions`** — Compare a bounded route/facet snapshot with the prior local observation and report changed period-value-unit keys.

## Recipes

### Compare two aligned grid series

```bash
eia-energy-pp-cli spread --route electricity/rto/region-data --left respondent=CISO --right respondent=ERCO --type D --frequency hourly --hours 24 --agent
```

Subtract only periods with identical units and frequency.

## Usage

Run `eia-energy-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `EIA_ENERGY_CONFIG_DIR`, `EIA_ENERGY_DATA_DIR`, `EIA_ENERGY_STATE_DIR`, or `EIA_ENERGY_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `EIA_ENERGY_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export EIA_ENERGY_HOME=/srv/eia-energy
eia-energy-pp-cli doctor
```

Under `EIA_ENERGY_HOME=/srv/eia-energy`, the four dirs resolve to `/srv/eia-energy/config`, `/srv/eia-energy/data`, `/srv/eia-energy/state`, and `/srv/eia-energy/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "eia-energy": {
      "command": "eia-energy-pp-mcp",
      "env": {
        "EIA_ENERGY_HOME": "/srv/eia-energy"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `EIA_ENERGY_DATA_DIR` overrides an explicit `--home` for that kind. Use `EIA_ENERGY_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `EIA_ENERGY_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `eia-energy-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### electricity

Manage electricity

- **`eia-energy-pp-cli electricity`** - Query balancing-authority demand, generation, forecast, and interchange data


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`eia-energy-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`eia-energy-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`eia-energy-pp-cli learnings list`** - Inspect taught rows
- **`eia-energy-pp-cli learnings forget <query>`** - Undo a teach
- **`eia-energy-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`eia-energy-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`eia-energy-pp-cli teach-pattern`** - Install a query/resource template up front
- **`eia-energy-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `EIA_ENERGY_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `eia-energy-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
eia-energy-pp-cli electricity

# JSON for scripting and agents
eia-energy-pp-cli electricity --json

# Filter to specific fields
eia-energy-pp-cli electricity --json --select id,name,status

# Dry run — show the request without sending
eia-energy-pp-cli electricity --dry-run

# Agent mode — JSON + compact + no prompts in one flag
eia-energy-pp-cli electricity --agent
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
eia-energy-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `eia-energy-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/eia-pp-cli/config.toml`; `--home`, `EIA_ENERGY_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `EIA_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `eia-energy-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `eia-energy-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $EIA_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **EIA rejects or throttles a request** — 
