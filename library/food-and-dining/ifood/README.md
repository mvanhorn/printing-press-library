# iFood CLI

**Agent-ready iFood grocery quoting with browser-owned authentication and confirmation-gated cart planning.**

This CLI combines typed iFood grocery discovery with a deterministic Browser-backed workflow for comparing a complete shopping list across eligible markets. Authentication and anti-fraud state stay inside the signed-in browser, while the CLI validates observations, totals, and the exact cart plan locally.

Learn more at [iFood](https://www.ifood.com.br).

Created by [@matheuscoelhomalta](https://github.com/matheuscoelhomalta) (Matheus Coêlho).

## Install

The recommended path installs both the `ifood-pp-cli` binary and the `pp-ifood` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install ifood
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install ifood --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install ifood --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install ifood --agent claude-code
npx -y @mvanhorn/printing-press-library install ifood --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/ifood/cmd/ifood-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ifood-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install ifood --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-ifood --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-ifood --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install ifood --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ifood-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `IFOOD_BEARER_AUTH` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/ifood/cmd/ifood-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "ifood": {
      "command": "ifood-pp-mcp",
      "env": {
        "IFOOD_BEARER_AUTH": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Prefer the Browser-backed commands, which never export cookies or authorization headers. Direct private-API commands accept IFOOD_BEARER_AUTH only in controlled environments and may still be rejected by iFood anti-automation controls.

## Quick Start

```bash
# Generate the safe quotation workflow and default six-item requirements.
ifood-pp-cli browser plan --json --no-learn

# Inspect the credential-free observation schema.
ifood-pp-cli browser schema --json --no-learn

# Inspect the preview-first direct cart builder without changing a cart.
ifood-pp-cli cart build --help

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Browser-backed agent workflow
- **`browser plan`** — Emit the complete browser-owned quotation and cart workflow without exporting credentials.

  _Agents get deterministic requirements while credentials remain inside the browser._

  ```bash
  ifood-pp-cli browser plan --json --no-learn
  ```
- **`browser validate-quote`** — Validate complete product observations from at least three markets meeting a configurable rating floor.

  _The CLI fails closed on missing, unavailable, unnamed, or unpriced products._

  ```bash
  ifood-pp-cli browser validate-quote --input ./ifood-quote.json --json --no-learn
  ```

### Guarded cart boundary
- **`browser cart-plan`** — Produce the exact selected merchant, products, quantities, and expected total before any cart interaction.

  _Agents can request approval for a concrete plan and stop on substitutions or price changes._

  ```bash
  ifood-pp-cli browser cart-plan --input ./ifood-quote.json --json --no-learn
  ```
- **`cart build`** — Resolve product terms into an exact read-only cart-item preview; use `cart add --execute --yes` only after reviewing it.

  _Dry-run remains the default and checkout is never attempted._

  ```bash
  ifood-pp-cli cart build --help
  ```

### Direct API composition
- **`quote`** — Compare requested grocery terms across eligible markets and return complete price estimates.

  _Agents can inspect the direct-API composition when a controlled session supports it._

  ```bash
  ifood-pp-cli quote --help
  ```

## Recipes

### Validate a complete quotation

```bash
ifood-pp-cli browser validate-quote --input ./ifood-quote.json --json --no-learn
```

Rejects incomplete or below-threshold markets and selects the lowest complete known total.

### Prepare a confirmation-ready cart plan

```bash
ifood-pp-cli browser cart-plan --input ./ifood-quote.json --json --no-learn
```

Produces an exact local plan and never writes to the remote cart.

### Inspect direct cart composition safely

```bash
ifood-pp-cli cart build --help
```

Shows preview-first cart composition options without running a mutation.

## Usage

Run `ifood-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `IFOOD_CONFIG_DIR`, `IFOOD_DATA_DIR`, `IFOOD_STATE_DIR`, or `IFOOD_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `IFOOD_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export IFOOD_HOME=/srv/ifood
ifood-pp-cli doctor
```

Under `IFOOD_HOME=/srv/ifood`, the four dirs resolve to `/srv/ifood/config`, `/srv/ifood/data`, `/srv/ifood/state`, and `/srv/ifood/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "ifood": {
      "command": "ifood-pp-mcp",
      "env": {
        "IFOOD_HOME": "/srv/ifood"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `IFOOD_DATA_DIR` overrides an explicit `--home` for that kind. Use `IFOOD_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `IFOOD_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `ifood-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### bm

Manage bm

- **`ifood-pp-cli bm`** - List grocery-market home sections for a delivery location

### ifood-web-read-search

Manage ifood web read search

- **`ifood-pp-cli ifood-web-read-search <merchant_id>`** - Search product items inside one grocery merchant

### merchants

Manage merchants

- **`ifood-pp-cli merchants <merchant_id>`** - Get a grocery merchant catalog


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`ifood-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`ifood-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`ifood-pp-cli learnings list`** - Inspect taught rows
- **`ifood-pp-cli learnings forget <query>`** - Undo a teach
- **`ifood-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`ifood-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`ifood-pp-cli teach-pattern`** - Install a query/resource template up front
- **`ifood-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `IFOOD_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `ifood-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
ifood-pp-cli bm --latitude -9.65 --longitude -35.71 --supported-actions merchant --supported-cards MERCHANT_LIST --supported-headers OPERATION_HEADER

# JSON for scripting and agents
ifood-pp-cli bm --latitude -9.65 --longitude -35.71 --supported-actions merchant --supported-cards MERCHANT_LIST --supported-headers OPERATION_HEADER --json
# Filter to specific fields by name
ifood-pp-cli bm --latitude -9.65 --longitude -35.71 --supported-actions merchant --supported-cards MERCHANT_LIST --supported-headers OPERATION_HEADER --json --select <field>[,<field>...]

# Dry run — show the request without sending
ifood-pp-cli bm --latitude -9.65 --longitude -35.71 --supported-actions merchant --supported-cards MERCHANT_LIST --supported-headers OPERATION_HEADER --dry-run

# Agent mode — JSON + compact + no prompts in one flag
ifood-pp-cli bm --latitude -9.65 --longitude -35.71 --supported-actions merchant --supported-cards MERCHANT_LIST --supported-headers OPERATION_HEADER --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Remote writes are gated** - Browser planning and validation never write remotely; direct cart commands preview by default and require both `--execute` and `--yes` before a cart mutation
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
ifood-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `ifood-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/ifood-pp-cli/config.toml`; `--home`, `IFOOD_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `IFOOD_BEARER_AUTH` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `ifood-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `ifood-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $IFOOD_BEARER_AUTH`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Direct API commands return an HTML 404 or authentication error** — Use the Browser-backed workflow with the existing signed-in iFood session; do not export cookies or anti-fraud values.
- **A cart plan reports ready=false** — Collect complete observations for at least three eligible markets and rerun browser validate-quote before planning the cart.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

TLS certificates are verified by default. For a trusted development or self-signed endpoint only, pass `--insecure` for one invocation, set `IFOOD_SKIP_TLS_VERIFY=true` for the current environment, or set `skip_tls_verify = true` in the config file for a persistent override.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
