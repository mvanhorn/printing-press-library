# Hugging Face CLI

**A stack-aware model-intel CLI for local-inference rigs and autonomous agent fleets — not just an API mirror.**

Closes the loop between Hugging Face discovery and the operational decisions only you can make: would this replace what I run today, will it load on my backend, has it already been benched. Every command is multi-runtime safe (TTY-agnostic, --json + stable schemas, exit-code semantics, flock-guarded state) so the same binary serves Rick at the terminal and JARVIS in a container.

Learn more at [Hugging Face](https://huggingface.co).

## Install

The recommended path installs both the `huggingface-pp-cli` binary and the `pp-huggingface` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install huggingface
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install huggingface --cli-only
```


### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/ml-ops/huggingface/cmd/huggingface-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/huggingface-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-huggingface --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-huggingface --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-huggingface skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-huggingface. The skill defines how its required CLI can be installed.
```

## Authentication

Anonymous read works for every public endpoint. Set HF_TOKEN (or use the cached token at ~/.cache/huggingface/token) for higher rate limits.

## Quick Start

```bash
# Probe the runtime first — agents branch all subsequent calls on this output.
huggingface-pp-cli doctor --json


# Find quant variants matching your hardware budget.
huggingface-pp-cli find-quants Qwen/Qwen3.6-35B-A3B --max-size 25g --uploaders unsloth,bartowski,mradermacher


# Diff against the model the named agent runs today.
huggingface-pp-cli vs-current Qwen/Qwen3.6-35B-A3B --agent main --json


# Verify backend support before a 30GB download.
huggingface-pp-cli backend-check Qwen/Qwen3.6-35B-A3B --backend llama.cpp,mlx --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Stack-aware verdicts

- **`find-feature`** — Search HF for models by architecture feature (mtp, mla, moe, gqa, sliding-window, rope-yarn) — fields HF doesn't index, recovered from config.json + card text.

  _Reach for this when an agent needs models with specific architectural capabilities (MoE, MTP-readiness, long-context attention) that HF's search UI cannot filter on._

  ```bash
  huggingface-pp-cli find-feature mtp --size 20b-40b --backend llama.cpp --json
  ```
- **`vs-current`** — Diff a candidate model against the model the named agent currently runs, by reading data/openclaw.json directly.

  _The single command that closes the loop between HF discovery and the operational decision to switch models._

  ```bash
  huggingface-pp-cli vs-current Qwen/Qwen3.6-35B-A3B --agent main --json
  ```
- **`backend-check`** — Verdict whether a model's architecture is supported by a target backend, citing PR numbers, commit SHAs, and wiki notes with source_checked dates.

  _Use before downloading any new model; catches incompatibility upfront._

  ```bash
  huggingface-pp-cli backend-check Qwen/Qwen3.6-VL --backend llama.cpp,mlx --json
  ```
- **`bench-history`** — Join an HF model id with local model-eval-harness results to surface tasks run, scores, vs-baseline delta, and last-run date.

  _Stops the eval pipeline from re-running known losers on every refresh._

  ```bash
  huggingface-pp-cli bench-history Qwen/Qwen3.6-35B-A3B --json
  ```
- **`eval-candidates`** — Emit a list of HF candidates in the model-eval-harness input format directly.

  _Use when scouting a new base model — pipes directly into the harness without copy-paste._

  ```bash
  huggingface-pp-cli eval-candidates --base Qwen/Qwen3.6-35B-A3B --target-size 25g --emit harness-input
  ```

### Compounding intel loops

- **`watch`** — State-file subscription with cron-callable poll; matches emit through the MC API alert pipeline as structured events.

  _Reach for this when monitoring a specific uploader, base model, or feature for new releases as part of intel automation._

  ```bash
  huggingface-pp-cli watch unsloth --kind uploader --notify jarvis
  ```

### Discovery

- **`find-quants`** — Surface GGUF quant variants of a base model with uploader rep, effective size, and quant-type classification — sorted by trusted-uploader allowlist + size filter.

  _Use when picking a quant for a target hardware budget; encodes Rick's trusted-uploader heuristic (unsloth, bartowski, mradermacher)._

  ```bash
  huggingface-pp-cli find-quants Qwen/Qwen3.6-35B-A3B --prefer iq4_nl,q4_k_m --max-size 25g --json
  ```

### Agent-native plumbing

- **`doctor`** — Single-call structured runtime-shape covering TTY, JSON, state writability, live config presence, harness presence, backend-matrix age, rate-limit remaining, HF reachability, proxy presence.

  _Required first call for any agent script; tells the agent what runtime constraints apply._

  ```bash
  huggingface-pp-cli doctor --json
  ```

## Usage

Run `huggingface-pp-cli --help` for the full command reference and flag list.

## Commands

### models

Model repositories on the Hugging Face Hub. Includes card metadata, file tree, raw config.json.

- **`huggingface-pp-cli models get`** - Get full model card metadata for one model.
- **`huggingface-pp-cli models list`** - List or search models. Supports HF search/filter/sort/author params.
- **`huggingface-pp-cli models raw`** - Fetch raw file contents (config.json, README.md, tokenizer_config.json, etc.).
- **`huggingface-pp-cli models tree`** - List files in a model repo at a given revision.

### users

Hugging Face users / organizations.

- **`huggingface-pp-cli users get`** - User/org overview: aggregate downloads, model count, recent uploads.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
huggingface-pp-cli models list

# JSON for scripting and agents
huggingface-pp-cli models list --json

# Filter to specific fields
huggingface-pp-cli models list --json --select id,name,status

# Dry run — show the request without sending
huggingface-pp-cli models list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
huggingface-pp-cli models list --agent
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

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `HF_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `huggingface-pp-cli models`
- `huggingface-pp-cli models get`
- `huggingface-pp-cli models list`
- `huggingface-pp-cli models raw`
- `huggingface-pp-cli models tree`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-huggingface -g
```

Then invoke `/pp-huggingface <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


```bash
go install github.com/mvanhorn/printing-press-library/library/ml-ops/huggingface/cmd/huggingface-pp-mcp@latest
```

Then register it:

```bash
claude mcp add huggingface huggingface-pp-mcp -e HF_TOKEN=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/huggingface-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `HF_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/ml-ops/huggingface/cmd/huggingface-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "huggingface": {
      "command": "huggingface-pp-mcp",
      "env": {
        "HF_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
huggingface-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/huggingface-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `HF_TOKEN` | per_call | Yes | Set to your API credential. |
| `HUGGING_FACE_HUB_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `huggingface-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $HF_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **Exit code 5 (rate limit)** — Set HF_TOKEN to a Hugging Face read token from https://huggingface.co/settings/tokens. Anonymous limits are ~5K req/day; authed limits are much higher.
- **Exit code 6 from vs-current / bench-history** — Pass --config-path / --harness or run from a directory containing data/openclaw.json or workspace/scripts/model-eval-harness. The CLI exits cleanly rather than crash when context is missing.
- **doctor reports backend_matrix_age_days > 90** — Bundled backend-support.json is stale. Override via --backend-support <path> or ~/.local/state/hf-cli/backend-support.override.json.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
