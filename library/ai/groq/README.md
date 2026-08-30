# Groq Cloud CLI

**Every Groq endpoint in your terminal, plus a local ledger that tracks token cost and rate-limit budget.**

groq-pp-cli wraps the full GroqCloud API — chat, responses, audio, vision, embeddings, reranking, batches, files, and fine-tuning — into one agent-native CLI. Beyond the endpoints, it syncs a model catalog to SQLite, keeps a local completion ledger that tracks cost and usage, and turns Groq's x-ratelimit headers into a spend-and-budget view no other Groq tool offers.

Learn more at [Groq Cloud](https://console.groq.com).

Created by [@SomSamantray](https://github.com/SomSamantray) (Som Samantray).

## Install

The recommended path installs both the `groq-pp-cli` binary and the `pp-groq` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install groq
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install groq --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install groq --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install groq --agent claude-code
npx -y @mvanhorn/printing-press-library install groq --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/groq/cmd/groq-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/groq-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install groq --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-groq --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-groq --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install groq --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/groq-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `GROQ_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/ai/groq/cmd/groq-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "groq": {
      "command": "groq-pp-mcp",
      "env": {
        "GROQ_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

```bash
# Health check that works without credentials — confirms config and network wiring
groq-pp-cli doctor --dry-run

# See the live model catalog; also validates your GROQ_API_KEY
groq-pp-cli models list --json

# First completion, with usage and latency stats
groq-pp-cli chat completions --model openai/gpt-oss-20b --messages '[{"role":"user","content":"Explain fast inference in one sentence"}]'

# Build the local SQLite mirror for offline search and the ledger commands
groq-pp-cli sync --resources models,files,batches --full

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local ledger that compounds
- **`rate-limits`** — See remaining per-model request/token budget from every API call, with reset windows.

  _Reach for this before a bulk run or when a 429 interrupts a pipeline; it tells you the exact remaining budget and reset time instead of guessing._

  ```bash
  groq-pp-cli rate-limits --model openai/gpt-oss-20b --json
  ```
- **`costs`** — Aggregate token and dollar spend from your local completion history, grouped by model or day.

  _Use this to answer 'how much did my eval runs cost this week' without exporting anything from the console._

  ```bash
  groq-pp-cli costs --since 48h --group-by model --agent
  ```

### Empirical model selection
- **`compare`** — Run one prompt across several models and rank them by latency, tokens/sec, usage, and cost.

  _Pick the right model for a task by measuring real speed and cost instead of reading spec sheets._

  ```bash
  groq-pp-cli compare "Explain transformers in one line" --models openai/gpt-oss-20b,openai/gpt-oss-120b --agent
  ```

### Batch workflow guardrails
- **`batch validate`** — Validate every line of a .jsonl batch request file against the endpoint schema and estimate tokens/cost before uploading.

  _Catch malformed batch lines and get a cost estimate before submitting a 100 MB file._

  ```bash
  groq-pp-cli batch validate eval-batch.jsonl --json
  ```
- **`batch diagnose`** — Tabulate a completed batch's per-line status codes and errors, highlighting retry-worthy failures.

  _Know exactly which batch lines failed and why, in seconds, from the shell._

  ```bash
  groq-pp-cli batch diagnose batch_abc123 --json
  ```

### Paced bulk audio
- **`audio batch`** — Transcribe, translate, or synthesize speech over many audio files with rate-limit-aware pacing and a results manifest.

  _Run a whole folder of episodes without dying mid-batch on a rate limit or re-processing completed files._

  ```bash
  groq-pp-cli audio batch episodes/ --action transcribe --pace --model whisper-large-v3
  ```

## Recipes

### Rank models on one prompt

```bash
groq-pp-cli compare "Explain transformers" --models openai/gpt-oss-20b,openai/gpt-oss-120b,qwen/qwen3.6-27b --agent --select models.0.model,models.0.latency_ms
```

The --agent + --select pair narrows the ranked comparison to the fields you care about instead of dumping full outputs.

### Check your budget before a big run

```bash
groq-pp-cli rate-limits --json
```

See remaining per-model requests and tokens before launching a bulk workload.

### Pre-flight a batch file

```bash
groq-pp-cli batch validate eval-batch.jsonl --json
```

Validate every request line and estimate cost before uploading a .jsonl batch.

### Transcribe a folder with pacing

```bash
groq-pp-cli audio batch episodes/ --action transcribe --pace --model whisper-large-v3
```

Bulk transcription that paces itself against your rate-limit budget and writes a success/failure manifest.

### What did my evals cost

```bash
groq-pp-cli costs --since 48h --group-by model
```

Aggregate token and dollar spend from local history, grouped by model.

## Usage

Run `groq-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `GROQ_CONFIG_DIR`, `GROQ_DATA_DIR`, `GROQ_STATE_DIR`, or `GROQ_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `GROQ_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export GROQ_HOME=/srv/groq
groq-pp-cli doctor
```

Under `GROQ_HOME=/srv/groq`, the four dirs resolve to `/srv/groq/config`, `/srv/groq/data`, `/srv/groq/state`, and `/srv/groq/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "groq": {
      "command": "groq-pp-mcp",
      "env": {
        "GROQ_HOME": "/srv/groq"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `GROQ_DATA_DIR` overrides an explicit `--home` for that kind. Use `GROQ_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `GROQ_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `groq-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### audio

Manage audio

- **`groq-pp-cli audio speech`** - Generates audio from the input text.
- **`groq-pp-cli audio transcribe`** - Transcribes audio into the input language.
- **`groq-pp-cli audio translate`** - Translates audio into English.

### batches

Manage batches

- **`groq-pp-cli batches cancel`** - Cancels a batch.
- **`groq-pp-cli batches create`** - Creates and executes a batch from an uploaded file of requests.
- **`groq-pp-cli batches get`** - Retrieves a batch.
- **`groq-pp-cli batches list`** - Returns a list of the user's batches with their current status and request counts.

### chat

Manage chat

- **`groq-pp-cli chat completions`** - Creates a model response for the given chat conversation.

### embeddings

Manage embeddings

- **`groq-pp-cli embeddings`** - Creates an embedding vector representing the input text.

### files

Manage files

- **`groq-pp-cli files delete`** - Delete a file.
- **`groq-pp-cli files download`** - Returns the contents of the specified file.
- **`groq-pp-cli files list`** - Returns a list of files that belong to the user's organization, with id, filename, purpose, bytes, and timestamps.
- **`groq-pp-cli files retrieve`** - Returns detailed information about a specific file by its ID.
- **`groq-pp-cli files upload`** - Upload a file that can be used across various endpoints.

The Batch API only supports `.jsonl` files up to 100 MB in size. The input also has a specific required

### fine_tunings

Manage fine tunings

- **`groq-pp-cli fine-tunings create`** - Creates a new fine tuning for the already uploaded files This endpoint is in closed beta.
- **`groq-pp-cli fine-tunings delete`** - Deletes an existing fine tuning by id This endpoint is in closed beta.
- **`groq-pp-cli fine-tunings get`** - Retrieves an existing fine tuning by id This endpoint is in closed beta.
- **`groq-pp-cli fine-tunings list`** - Lists all previously created fine tunings. This endpoint is in closed beta.

### models

Manage models

- **`groq-pp-cli models delete`** - Delete a model
- **`groq-pp-cli models list`** - Lists all models currently available on the account, including context window, pricing, and capabilities.
- **`groq-pp-cli models retrieve`** - Returns detailed information about a specific model by ID.

### reranking

Manage reranking

- **`groq-pp-cli reranking`** - Given a query and a list of documents, returns the documents ranked by their relevance to the query.
The documents are scored and sorted in descending order of relevance.

### responses

Manage responses

- **`groq-pp-cli responses`** - Creates a model response for the given input.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`groq-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`groq-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`groq-pp-cli learnings list`** - Inspect taught rows
- **`groq-pp-cli learnings forget <query>`** - Undo a teach
- **`groq-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`groq-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`groq-pp-cli teach-pattern`** - Install a query/resource template up front
- **`groq-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `GROQ_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `groq-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
groq-pp-cli batches list

# JSON for scripting and agents
groq-pp-cli batches list --json
# Filter to specific fields
groq-pp-cli batches list --json --select cancelled_at,cancelling_at,completed_at

# Dry run — show the request without sending
groq-pp-cli batches list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
groq-pp-cli batches list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Explicit confirmation** - `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `GROQ_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `groq-pp-cli batches`
- `groq-pp-cli batches get`
- `groq-pp-cli batches list`
- `groq-pp-cli batches search`
- `groq-pp-cli files`
- `groq-pp-cli files get`
- `groq-pp-cli files list`
- `groq-pp-cli files search`
- `groq-pp-cli fine_tunings`
- `groq-pp-cli fine_tunings get`
- `groq-pp-cli fine_tunings list`
- `groq-pp-cli fine_tunings search`
- `groq-pp-cli models`
- `groq-pp-cli models get`
- `groq-pp-cli models list`
- `groq-pp-cli models search`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Health Check

```bash
groq-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `groq-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/groq-pp-cli/config.toml`; `--home`, `GROQ_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `GROQ_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `groq-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `groq-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $GROQ_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **429 Too Many Requests** — Run `groq-pp-cli rate-limits` to see remaining per-model budget, then wait for `retry-after` or lower volume; bulk audio supports `--pace`
- **401 Invalid API key** — Export GROQ_API_KEY from console.groq.com/keys or run `groq-pp-cli auth set-token <key>`
- **Model not found** — Run `groq-pp-cli models list` for current IDs — preview models are deprecated without notice
- **Unsupported field error** — Groq is OpenAI-compatible but not identical: logprobs, logit_bias, top_logprobs, and messages[].name are unsupported — drop them from the request

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**groq-code-cli**](https://github.com/build-with-groq/groq-code-cli) — TypeScript (740 stars)
- [**groq-python**](https://github.com/groq/groq-python) — Python (611 stars)
- [**groq-mcp-server**](https://github.com/groq/groq-mcp-server) — Python (44 stars)
- [**groq-cli-chat**](https://github.com/OleksiyM/groq-cli-chat) — Go (21 stars)
- [**groqcli**](https://github.com/ciraben/groqcli) — Python (1 stars)
- [**groq-cli-minimal**](https://github.com/orliesaurus/groq-cli-minimal) — Shell (1 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
