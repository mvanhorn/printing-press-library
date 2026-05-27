# Wavespeed CLI

Docs-derived OpenAPI spec for WaveSpeed AI's REST API. WaveSpeed is a
unified AI generation platform for image, video, audio, 3D, and LLM models.

The dynamic model-run endpoint is not represented as a generated OpenAPI
path because WaveSpeed model IDs are slash-delimited API paths such as
`wavespeed-ai/hunyuan-video/t2v`; ordinary OpenAPI path-parameter clients
correctly percent-encode slashes. The printed CLI adds a hand-authored
`run` command that submits to the literal model path.

Learn more at [Wavespeed](https://wavespeed.ai).

Printed by [@cathrynlavery](https://github.com/cathrynlavery) (Cathryn Lavery).

## Install

The recommended path installs both the `wavespeed-pp-cli` binary and the `pp-wavespeed` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install wavespeed
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install wavespeed --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install wavespeed --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install wavespeed --agent claude-code
npx -y @mvanhorn/printing-press-library install wavespeed --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/wavespeed/cmd/wavespeed-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/wavespeed-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-wavespeed --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-wavespeed --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-wavespeed skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-wavespeed. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/wavespeed-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `WAVESPEED_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/ai/wavespeed/cmd/wavespeed-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "wavespeed": {
      "command": "wavespeed-pp-mcp",
      "env": {
        "WAVESPEED_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your API key from your API provider's developer portal. The key typically looks like a long alphanumeric string.

```bash
export WAVESPEED_API_KEY="<paste-your-key>"
```

You can also persist this in your config file at `~/.config/wavespeed-pp-cli/config.toml`.

### 3. Verify Setup

```bash
wavespeed-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
wavespeed-pp-cli billings
```

## Usage

Run `wavespeed-pp-cli --help` for the full command reference and flag list.

## Commands

### account_balance

Manage account balance

- **`wavespeed-pp-cli account-balance`** - Retrieve the authenticated account balance.

### billings

Billing and usage records

- **`wavespeed-pp-cli billings`** - Search billing records for the authenticated account.

### media_uploads

Manage media uploads

- **`wavespeed-pp-cli media-uploads`** - Upload a binary file to WaveSpeed media storage.

### model_pricing

Manage model pricing

- **`wavespeed-pp-cli model-pricing`** - Estimate the unit price for a model run using the same inputs that will be submitted to the model endpoint.

### models

Model catalog and model metadata

- **`wavespeed-pp-cli models`** - List available WaveSpeed models and their API schemas.

### run

Submit generation tasks to slash-delimited WaveSpeed model paths.

- **`wavespeed-pp-cli run <model-id> --input '<json>'`** - Submit a model run with JSON inputs.
- **`wavespeed-pp-cli run <model-id> --input-file request.json --price --wait --download`** - Price, submit, poll, and download output URLs.

### prediction_deletions

Manage prediction deletions

- **`wavespeed-pp-cli prediction-deletions`** - Delete one or more predictions from history.

### prediction_results

Manage prediction results

- **`wavespeed-pp-cli prediction-results <task_id>`** - Retrieve the latest status and result payload for a prediction task.

### predictions

Prediction submission history and result retrieval

- **`wavespeed-pp-cli predictions`** - Query recent prediction history. The API history window is limited; sync accumulates across runs.

### usage_stats

Manage usage stats

- **`wavespeed-pp-cli usage-stats`** - Retrieve usage statistics for the authenticated account.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
wavespeed-pp-cli billings

# JSON for scripting and agents
wavespeed-pp-cli billings --json

# Filter to specific fields
wavespeed-pp-cli billings --json --select id,name,status

# Dry run — show the request without sending
wavespeed-pp-cli billings --dry-run

# Submit a model run without path-encoding slash-delimited model IDs
wavespeed-pp-cli run wavespeed-ai/flux-dev --input '{"prompt":"a studio product photo"}' --wait

# Agent mode — JSON + compact + no prompts in one flag
wavespeed-pp-cli billings --agent
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
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
wavespeed-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/wavespeed-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `WAVESPEED_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `wavespeed-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `wavespeed-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $WAVESPEED_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
