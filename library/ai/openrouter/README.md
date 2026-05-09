# OpenRouter CLI

**Agent-first OpenRouter introspection — terse output for cron and AI agents (--agent and --llm modes), local SQLite catalog, free MCP server, per-caller cost attribution.**

Other OpenRouter CLIs are chat REPLs. This one is built for the cron job and the AI agent calling out to `Bash`. Absorbed introspection commands (`credits`, `models`, `key`, `generation`, `providers`) honor the global `--agent` flag (sets --json --compact --no-input). Eight novel commands (`usage cost-by`, `models query`, `key eta`, `providers degraded`, `generation explain`, `usage anomaly`, `endpoints failover`, `budget`) ship a `--llm` mode that returns under 200 tokens of key:value output. A local SQLite catalog lets you query the 400+ model list with `models query "tools=true cost.completion<1"` instead of pasting 425KB of JSON into context.

Learn more at [OpenRouter](https://openrouter.ai/docs).

## Install

The recommended path installs both the `openrouter-pp-cli` binary and the `pp-openrouter` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install openrouter
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install openrouter --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/openrouter-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-openrouter --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-openrouter --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-openrouter skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-openrouter. The skill defines how its required CLI can be installed.
```

## Authentication

Set `OPENROUTER_API_KEY` for per-call operations (creds, models, usage, generation lookup). For sub-key management (`keys list/create/delete`), set `OPENROUTER_MANAGEMENT_KEY` separately — OpenRouter's API splits these intentionally and this CLI honors the split. Both variables are read fresh per command; nothing is persisted to disk by default.

## Quick Start

```bash
# Verify auth, network, and local store. Run this first.
openrouter doctor


# Terse balance output for cron and agent consumers (--agent = --json --compact --no-input).
openrouter credits --agent


# Refresh the local SQLite catalog from /models, /providers, /key, /activity.
openrouter sync


# Shortlist tool-capable cheap deep-context models from local catalog (no 425KB JSON dump).
openrouter models query "tools=true cost.completion<1 ctx>=64k" --llm


# Project when the weekly cap will trip from /key + 7d burn rate.
openrouter key eta --llm

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`usage cost-by`** — Group your OpenRouter spend by which cron/agent fired the call, not just by model. Joins local generations with caller tags from your tool-call logger.

  _Use this when an agent needs to answer 'which automated job is burning my OpenRouter budget?' before deciding what to throttle._

  ```bash
  openrouter usage cost-by --group cron --since 7d --llm
  ```
- **`models query`** — Query the model catalog with structured filters (tools=true, cost.completion<1, ctx>=64k) — compiled to SQL over a local SQLite cache. Works offline.

  _Use this when an agent needs to shortlist models for an experiment without hallucinating pricing or pasting 400 model rows into context._

  ```bash
  openrouter models query "tools=true cost.completion<1 ctx>=64k modality=text" --llm
  ```
- **`generation explain`** — For a generation id, returns the cost, latency, prompt/completion token counts, AND a delta vs the cheapest provider for the same model+token-count.

  _Use this when an agent needs to decide whether a generation was expensive because of the model choice, the prompt size, or the provider markup._

  ```bash
  openrouter generation explain gen-abc123 --llm
  ```

### Agent-native plumbing
- **`providers degraded`** — Returns the set of currently-degraded provider/model pairs by polling /providers and per-model /endpoints. Pipe into your router to preempt 429s.

  _Use this in a router or fallback chain when an agent needs to skip degraded provider/model pairs before dispatch instead of after a failed call._

  ```bash
  openrouter providers degraded --json | jq -r '.[].model_id'
  ```
- **`usage anomaly`** — Flags days where per-model cost exceeds 2σ of the trailing 7-day mean. Deterministic z-score, no LLM in the loop. Designed for cron.

  _Use this in a daily cron when an agent needs to detect cost regressions before a credit-low alarm fires._

  ```bash
  openrouter usage anomaly --since 24h --baseline 7d --llm
  ```
- **`key eta`** — Projects when your weekly OpenRouter cap will trip, based on /key.limit_reset, current usage, and your trailing 7-day burn rate.

  _Use this in a daily cron when an agent needs to know whether scheduled work will fit in the remaining weekly cap._

  ```bash
  openrouter key eta --llm
  ```
- **`budget`** — Set a weekly USD cap per cron job (budget set scan-pipeline 2usd). Pre-flight check returns exit 0 (under cap) or 8 (over) from tagged generations.

  _Use this when an agent needs structural budget enforcement per sub-agent or per cron, not aspirational env-var quotas._

  ```bash
  openrouter budget check scan-pipeline && ./scan-pipeline.mjs
  ```
- **`endpoints failover`** — For a model id, lists all providers serving it ranked by current status, pricing, and observed p50 latency from local cache. Pipe-feeds routers.

  _Use this when an agent needs to choose a provider for a given model based on current availability, not the static config order._

  ```bash
  openrouter endpoints failover anthropic/claude-opus-4-7 --json
  ```

## Usage

Run `openrouter-pp-cli --help` for the full command reference and flag list.

## Commands

### activity

Manage activity

- **`openrouter-pp-cli activity get-user`** - Returns user activity data grouped by endpoint for the last 30 (completed) UTC days. [Management key](/docs/guides/overview/auth/management-api-keys) required.

### credits

Credit management endpoints

- **`openrouter-pp-cli credits get`** - Get total credits purchased and used for the authenticated user. [Management key](/docs/guides/overview/auth/management-api-keys) required.

### endpoints

Endpoint information

- **`openrouter-pp-cli endpoints list-zdr`** - Preview the impact of ZDR on the available endpoints

### generation

Generation history endpoints

- **`openrouter-pp-cli generation get`** - Get request & usage metadata for a generation
- **`openrouter-pp-cli generation list-content`** - Get stored prompt and completion content for a generation

### key

Manage key

- **`openrouter-pp-cli key get-current`** - Get information on the API key associated with the current authentication session

### keys

Manage keys

- **`openrouter-pp-cli keys create`** - Create a new API key for the authenticated user. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-pp-cli keys delete`** - Delete an existing API key. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-pp-cli keys get`** - Get a single API key by hash. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-pp-cli keys list`** - List all API keys for the authenticated user. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-pp-cli keys update`** - Update an existing API key. [Management key](/docs/guides/overview/auth/management-api-keys) required.

### models

Model information endpoints

- **`openrouter-pp-cli models get`** - List all models and their properties
- **`openrouter-pp-cli models list-count`** - Get total count of available models
- **`openrouter-pp-cli models list-user`** - List models filtered by user provider preferences, [privacy settings](https://openrouter.ai/docs/guides/privacy/provider-logging), and [guardrails](https://openrouter.ai/docs/guides/features/guardrails). If requesting through `eu.openrouter.ai/api/v1/...` the results will be filtered to models that satisfy [EU in-region routing](https://openrouter.ai/docs/guides/privacy/provider-logging#enterprise-eu-in-region-routing).

### openrouter-auth

Manage openrouter auth

- **`openrouter-pp-cli openrouter-auth create-keys-code`** - Create an authorization code for the PKCE flow to generate a user-controlled API key
- **`openrouter-pp-cli openrouter-auth exchange-code-for-apikey`** - Exchange an authorization code from the PKCE flow for a user-controlled API key

### providers

Provider information endpoints

- **`openrouter-pp-cli providers list`** - List all providers


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
openrouter-pp-cli credits

# JSON for scripting and agents
openrouter-pp-cli credits --json

# Filter to specific fields
openrouter-pp-cli credits --json --select id,name,status

# Dry run — show the request without sending
openrouter-pp-cli credits --dry-run

# Agent mode — JSON + compact + no prompts in one flag
openrouter-pp-cli credits --agent
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-openrouter -g
```

Then invoke `/pp-openrouter <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add openrouter openrouter-pp-mcp -e OPENROUTER_API_KEY=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/openrouter-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `OPENROUTER_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "openrouter": {
      "command": "openrouter-pp-mcp",
      "env": {
        "OPENROUTER_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
openrouter-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/openrouter-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `OPENROUTER_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `openrouter-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $OPENROUTER_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **exit code 4 on credits/key** — OPENROUTER_API_KEY is missing or malformed. Run `openrouter auth status` to see resolved sources.
- **exit code 5 on keys list** — Sub-key management requires OPENROUTER_MANAGEMENT_KEY (separate from OPENROUTER_API_KEY). Set it: `export OPENROUTER_MANAGEMENT_KEY=...`.
- **models query returns empty** — Run `openrouter sync --resources models` first — local catalog must be populated.
- **usage cost-by returns 'no data'** — Caller tagging requires the tier-2.0 effect-tool-call logger to be writing to ~/.openclaw/tool-call-log/. Verify with `ls ~/.openclaw/tool-call-log/`.
- **providers degraded returns empty arrays** — Set-diff requires a previous snapshot at ~/.local/share/openrouter-pp-cli/providers-prev.json. Run twice (once to seed, once to compare).

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**grahamking/ort**](https://github.com/grahamking/ort) — Rust (36 stars)
- [**mrgoonie/openrouter-cli**](https://github.com/mrgoonie/openrouter-cli) — TypeScript (18 stars)
- [**simonw/llm-openrouter**](https://github.com/simonw/llm-openrouter) — Python
- [**oop7/OrChat**](https://github.com/oop7/OrChat) — Python
- [**physics91/openrouter-mcp**](https://github.com/physics91/openrouter-mcp) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
