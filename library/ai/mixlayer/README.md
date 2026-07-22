# Mixlayer CLI

Privacy-first Mixlayer CLI for OpenAI-compatible chat, local PII redaction and vaulting, reasoning-aware model ladders, live model catalog refresh, and ledger-backed cost proofs.

Created by [@cathrynlavery](https://github.com/cathrynlavery) (Cathryn Lavery).

## What This CLI Adds

Mixlayer exposes one OpenAI-compatible chat endpoint, but its real CLI value is the model ladder, `reasoning_content`, seed determinism, and the free 4B tier. This CLI keeps the generated `chat` and `models` wrappers and adds a local intelligence layer:

- `shield` masks sensitive data before frontier-model calls, writes privacy receipts, and keeps the reversible token vault local.
- `ladder`, `escalate`, and `council` compare default model rungs, reason traces, latency, and cost. Pass explicit Mixlayer model IDs when you want a different set.
- `ask`, `replay`, `grep`, `models query`, and `sql` turn prompts, answers, reasoning, and model metadata into a local SQLite ledger.
- `savings` and `compare` calculate cost deltas from local ledger evidence rather than vendor claims.

## Install

The recommended path installs both the `mixlayer-pp-cli` binary and the `pp-mixlayer` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install mixlayer
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install mixlayer --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install mixlayer --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install mixlayer --agent claude-code
npx -y @mvanhorn/printing-press-library install mixlayer --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/mixlayer/cmd/mixlayer-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/mixlayer-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install mixlayer --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-mixlayer --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-mixlayer --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install mixlayer --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/mixlayer-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `MIXLAYER_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/ai/mixlayer/cmd/mixlayer-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "mixlayer": {
      "command": "mixlayer-pp-mcp",
      "env": {
        "MIXLAYER_API_KEY": "<your-key>"
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

Get your access token from your API provider's developer portal, then store it:

```bash
mixlayer-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set it via environment variable:

```bash
export MIXLAYER_API_KEY="your-token-here"
```

### 3. Verify Setup

```bash
mixlayer-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
mixlayer-pp-cli models list
```

### Privacy Firewall

```bash
mixlayer-pp-cli shield scan customer-export.csv --max-risk 0 --json
mixlayer-pp-cli shield redact customer-export.csv --diff -o masked.csv
mixlayer-pp-cli shield ingest big-export.csv -o masked-corpus.csv --manifest tranches.json
mixlayer-pp-cli shield ask "What segments are at risk?" --data customer-export.csv --json
mixlayer-pp-cli vault list --json
mixlayer-pp-cli shield audit --json
```

`shield ask` redacts locally, re-runs detectors on the outbound masked payload, sends only the masked corpus, then rehydrates the answer from the local vault. The audit receipt records the outbound payload hash, model, guard-model label, masked entity count, and residual leak count; the tripwire itself is enforced by local detectors.

### Model Ladder And Reasoning Ledger

```bash
mixlayer-pp-cli ask "Draft a migration plan" --model qwen/qwen3.5-27b --show-thinking --json
mixlayer-pp-cli replay run_abc123 --json
mixlayer-pp-cli grep "migration" --json
mixlayer-pp-cli ladder "Which option is cheapest?" --reasoning --json
mixlayer-pp-cli escalate "Classify these support tickets" --confidence 0.85 --json
mixlayer-pp-cli council "Pick the safest launch plan" --json
mixlayer-pp-cli models query "ctx>=128k tools reasoning" --json
mixlayer-pp-cli models query "kimi" --refresh --json
mixlayer-pp-cli sql "select model, count(*) from runs group by model" --json
```

Use `mixlayer-pp-cli models list --json` to ask Mixlayer for the live model catalog available to your API key. `models query` seeds a local cache with the documented Qwen rungs plus console-visible IDs such as `qwen/qwen3.6-27b`, `qwen/qwen3.6-35b-a3b`, and `moonshotai/kimi-k2.7-code`; add `--refresh` to merge the live `/models` response into that cache. `--model`, `--rungs`, `--members`, and `--judge` accept any Mixlayer model ID the API authorizes.

### Cost Proof

```bash
mixlayer-pp-cli savings --vs gpt-frontier --json
mixlayer-pp-cli compare "Summarize this incident" --cheap qwen/qwen3.5-9b --json
```

## Unique Features

These capabilities aren't available in any other tool for this API.
- **`shield ingest`** — Split large CSV/text inputs into deterministic tranches, redact each tranche through one shared local vault, and reassemble a masked corpus with a manifest.
- **`shield ask`** — Redact local data, run an outbound PII tripwire, send only the masked payload to a frontier model, rehydrate the answer locally, and save a privacy receipt.
- **`shield scan`** — Detect high-risk PII locally with CI-friendly exit codes and no upstream model call.
- **`shield redact`** — Write a masked copy of a file and populate the local token-to-value vault with stable pseudonyms.
- **`shield restructure`** — Drop columns, bucket numeric values, and coarsen dates to reduce re-identification risk before prompting.
- **`vault`** — List, rehydrate, rotate, or purge the local reversible pseudonym vault without sending values upstream.
- **`shield audit`** — Inspect outbound payload hashes, byte counts, model IDs, masked entity counts, and residual leak counts for shielded sends.
- **`ladder`** — Run one prompt across selected Mixlayer model rungs, optionally comparing reasoning_content, latency, token usage, and cost.
- **`escalate`** — Start on a cheap rung and climb only when the prior answer appears insufficiently confident.
- **`council`** — Fan out to member model rungs and ask a judge model to synthesize answers and flag disagreements.
- **`ask`** — Ask Mixlayer, optionally request reasoning_content, and save prompt, answer, model, seed, token usage, cost, and raw response locally.
- **`replay`** — Re-run a saved prompt with its original model and seed to check determinism or drift.
- **`grep`** — Search saved prompts, answers, and reasoning traces through the local FTS ledger.
- **`models query`** — Query a local model cache with DSL terms like ctx>=128k tools reasoning, seed it with documented and console-visible model IDs, and optionally refresh it from live /models.
- **`sql`** — Run SELECT-only SQLite queries over the local ledger, vault, audit, ladder, and model cache tables.
- **`savings`** — Roll up saved ledger cost against static GPT or Claude frontier baselines.
- **`compare`** — Run a prompt through a cheap rung and frontier rung, report the cost delta, and attach a local-history quality note.

## Usage

Run `mixlayer-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `MIXLAYER_CONFIG_DIR`, `MIXLAYER_DATA_DIR`, `MIXLAYER_STATE_DIR`, or `MIXLAYER_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `MIXLAYER_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export MIXLAYER_HOME=/srv/mixlayer
mixlayer-pp-cli doctor
```

Under `MIXLAYER_HOME=/srv/mixlayer`, the four dirs resolve to `/srv/mixlayer/config`, `/srv/mixlayer/data`, `/srv/mixlayer/state`, and `/srv/mixlayer/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "mixlayer": {
      "command": "mixlayer-pp-mcp",
      "env": {
        "MIXLAYER_HOME": "/srv/mixlayer"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `MIXLAYER_DATA_DIR` overrides an explicit `--home` for that kind. Use `MIXLAYER_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `MIXLAYER_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `mixlayer-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### chat

OpenAI-compatible chat completions

- **`mixlayer-pp-cli chat`** - Create a chat completion for the given model and messages

### models

Browse the model catalog

- **`mixlayer-pp-cli models get`** - Retrieve a model by ID
- **`mixlayer-pp-cli models list`** - List all available models


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
mixlayer-pp-cli models list

# JSON for scripting and agents
mixlayer-pp-cli models list --json

# Filter to specific fields
mixlayer-pp-cli models list --json --select id,name,status

# Dry run — show the request without sending
mixlayer-pp-cli models list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
mixlayer-pp-cli models list --agent
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
mixlayer-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `mixlayer-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/mixlayer-pp-cli/config.toml`; `--home`, `MIXLAYER_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `MIXLAYER_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `mixlayer-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `mixlayer-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $MIXLAYER_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
