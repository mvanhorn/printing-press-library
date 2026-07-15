# Ollama Cloud CLI

**Routes every prompt to the right hosted Ollama model. Wraps chat, embeddings, catalog, and the OpenAI-compatible mirror — and ships an advise verb that picks a model by prompt features, session context, budget, and latency.**

Ollama Cloud's catalog spans 20+ open-weights models (Qwen3, GPT-OSS, DeepSeek-V3, Kimi-K2, GLM, Llama, Gemma) and growing. No built-in picker exists. This CLI is the only one that combines live catalog metadata with prompt-feature extraction and emits a why/alternatives envelope you can pipe into other agents.

Created by [@rvdlaar](https://github.com/rvdlaar) (Rick van de Laar).

## Install

The recommended path installs both the `ollama-cloud-pp-cli` binary and the `pp-ollama-cloud` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install ollama-cloud
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install ollama-cloud --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install ollama-cloud --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install ollama-cloud --agent claude-code
npx -y @mvanhorn/printing-press-library install ollama-cloud --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/ollama-cloud/cmd/ollama-cloud-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ollama-cloud-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install ollama-cloud --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-ollama-cloud --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-ollama-cloud --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install ollama-cloud --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ollama-cloud-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `OLLAMA_CLOUD_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "ollama-cloud": {
      "command": "ollama-cloud-pp-mcp",
      "env": {
        "OLLAMA_CLOUD_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Bearer auth via OLLAMA_CLOUD_API_KEY (intentionally distinct from any local-Ollama env var). Free tier is rate-limited weekly; the budget command surfaces exhaustion before workflows fail.

## Quick Start

```bash
# Confirms auth, catalog reachable
ollama-cloud-pp-cli doctor

# Live catalog of hosted models
ollama-cloud-pp-cli tags --json

# Inspect drift between live catalog and curated metadata overlay
ollama-cloud-pp-cli advise --validate-catalog --json

# Pick a model for a specific prompt; pass any file path (or - for stdin)
ollama-cloud-pp-cli advise --prompt-file ./prompt.txt --task-hint coding --json

# Probe free-tier quota before launching long sessions
ollama-cloud-pp-cli budget --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Routing intelligence
- **`advise`** — Picks the right Ollama Cloud model for a prompt by combining live catalog, heuristic prompt-feature extraction, curated cost/latency metadata, and an optional cheap meta-LLM tiebreak.

  _When an agent needs to pick a hosted Ollama model and the default routing is wrong, reach for advise instead of hardcoding the model name._

  ```bash
  ollama-cloud-pp-cli advise --prompt-file ./prompt.txt --task-hint coding --budget-remaining-usd 0.50 --json
  ```
- **`compare`** — Runs the same prompt against N hosted models in parallel and emits side-by-side response, tokens, and latency.

  _Use when calibrating advisor recommendations or picking between two close models._

  ```bash
  ollama-cloud-pp-cli compare --prompt-file ./p.txt --models qwen3-coder:480b,gpt-oss:120b,deepseek-v3.1:671b --json
  ```
- **`advise`** — With --explain, advise emits the full scoring trace: feature extraction, per-model scores, filter passes, tiebreak rationale.

  _Reach for this when an advise recommendation surprises you and you want to understand why._

  ```bash
  ollama-cloud-pp-cli advise --prompt-file ./p.txt --explain --format md
  ```

### Engagement canary
- **`advise-replay`** — Replays advisor recommendations and reports divergence between recommended models and actually-chosen models. Foundation for the divergence canary; the prompt corpus is not retained so judge-LLM scoring is not in scope until a corpus sidecar ships.

  _Run weekly to detect advisor drift; surfaces divergence between recommended and actual-chosen models._

  ```bash
  ollama-cloud-pp-cli advise-replay --since 7d --diverge-only --json --select rows,divergence_count,divergence_pct
  ```

### Operations
- **`budget`** — Probes the free-tier weekly cap with a 1-token chat. Parses Ollama Cloud's 429 prose and emits a structured verdict (ok | exhausted | unknown) with the upgrade URL so agents can pre-flight quota before launching long sessions.

  _Run before launching a long agent session to confirm quota is available._

  ```bash
  ollama-cloud-pp-cli budget --json
  ```
- **`cost-trace`** — Aggregates advisor-log cost estimates over a time window; compares per-model and per-task-hint spend.

  _Use to decide whether to upgrade to a paid Ollama Cloud tier._

  ```bash
  ollama-cloud-pp-cli cost-trace --since 7d --group-by task-hint --json
  ```

## Usage

Run `ollama-cloud-pp-cli --help` for the full command reference and flag list.

## Commands

### chat

Manage chat

- **`ollama-cloud-pp-cli chat chat`** - Native Ollama chat endpoint. Supports streaming.
- **`ollama-cloud-pp-cli chat completions`** - OpenAI-compatible chat completions endpoint.

### embeddings

Manage embeddings

- **`ollama-cloud-pp-cli embeddings embed`** - Native Ollama embeddings endpoint.
- **`ollama-cloud-pp-cli embeddings openai-embed`** - Generate embeddings (OpenAI-compatible)

### models

Manage models

- **`ollama-cloud-pp-cli models models`** - Catalog in OpenAI list-models format.

### ps

Manage ps

- **`ollama-cloud-pp-cli ps ps`** - Shows currently-loaded models. On Ollama Cloud this typically reflects models with active sessions.

### show

Manage show

- **`ollama-cloud-pp-cli show show`** - Returns model metadata, template, modelfile, capabilities.

### tags

Manage tags

- **`ollama-cloud-pp-cli tags tags`** - Returns the live catalog of hosted Ollama Cloud models.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
ollama-cloud-pp-cli chat chat --model example-value

# JSON for scripting and agents
ollama-cloud-pp-cli chat chat --model example-value --json

# Filter to specific fields
ollama-cloud-pp-cli chat chat --model example-value --json --select id,name,status

# Dry run — show the request without sending
ollama-cloud-pp-cli chat chat --model example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
ollama-cloud-pp-cli chat chat --model example-value --agent
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
ollama-cloud-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/ollama-cloud-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `OLLAMA_CLOUD_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `ollama-cloud-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $OLLAMA_CLOUD_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **HTTP 429 with 'you have reached your weekly usage limit'** — Free tier exhausted. Run `ollama-cloud-pp-cli budget --json` to confirm; pick a non-rate-limited model with `advise --exclude <exhausted-models>`; or upgrade at https://ollama.com/upgrade
- **advise returns unexpected model** — Run `advise --explain --format md` to see the scoring trace. Adjust --task-hint, --exclude, or the curated models.json overlay.
- **401 unauthorized** — Confirm OLLAMA_CLOUD_API_KEY is set; the CLI does NOT read OLLAMA_API_KEY (intentional, to avoid local-daemon collisions)
- **advise picks a model not in /api/tags** — Catalog snapshot is stale. Run `tags --no-cache` to repopulate the local SQLite snapshot.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**openai-python**](https://github.com/openai/openai-python) — Python (22000 stars)
- [**litellm**](https://github.com/BerriAI/litellm) — Python (14000 stars)
- [**ollama-python**](https://github.com/ollama/ollama-python) — Python (5200 stars)
- [**ollama-js**](https://github.com/ollama/ollama-js) — TypeScript (4500 stars)
- [**aichat**](https://github.com/sigoden/aichat) — Rust (3800 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
