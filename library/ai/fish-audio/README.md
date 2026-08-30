# Fish Audio CLI

**Fish Audio text-to-speech, voice cloning, and transcription from the terminal, with a local render log that tracks every byte and dollar the API never records.**

Render text to audio files, clone a voice from a sample, design a voice from a prompt, and transcribe audio, all with agent-friendly JSON output. Every render lands in a local SQLite log so you can skip duplicate renders, report spend by voice or model, and verify a clone's fidelity before it goes live.

## Install

The recommended path installs both the `fish-audio-pp-cli` binary and the `pp-fish-audio` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install fish-audio
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install fish-audio --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install fish-audio --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install fish-audio --agent claude-code
npx -y @mvanhorn/printing-press-library install fish-audio --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/fish-audio/cmd/fish-audio-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/fish-audio-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install fish-audio --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-fish-audio --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-fish-audio --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install fish-audio --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/fish-audio-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `FISH_AUDIO_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/ai/fish-audio/cmd/fish-audio-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "fish-audio": {
      "command": "fish-audio-pp-mcp",
      "env": {
        "FISH_AUDIO_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Set FISH_AUDIO_API_KEY (from https://fish.audio/app/api-keys) or store it with `fish-audio-pp-cli auth set-token`. The `model` header selects the TTS model (s1, s2-pro, s2.1-pro, s2.1-pro-free); the CLI validates it because the API silently falls back to s2.1-pro on typos.

## Quick Start

```bash
# confirm the key and API reachability
fish-audio-pp-cli doctor --dry-run

# find a public voice by description
fish-audio-pp-cli voice discover --query "warm female narrator" --limit 5

# render one line to a file and log its cost
fish-audio-pp-cli tts render --text "Hi, this is Pearl. How can I help?" --voice 802e3bc2b27e49c2995d23ef70e6ac89 --out greeting.mp3

# clone a voice from a reference clip
fish-audio-pp-cli voice clone --title "Pearl greeting" --audio sample.wav

# see what you have spent so far
fish-audio-pp-cli render spend --group-by model

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`render log`** — See every past TTS render with its text, model, voice, byte count, and cost.

  _Use it to recover what was already rendered before spending credit again._

  ```bash
  fish-audio-pp-cli render log --limit 20 --agent
  ```
- **`render spend`** — Total Fish Audio spend grouped by voice, model, or day for a side-by-side with your ElevenLabs invoice.

  _Reach for it when the question is 'what did this voice cost us this month'._

  ```bash
  fish-audio-pp-cli render spend --group-by model --since 30d --agent
  ```
- **`tts render`** — Hash the request and reuse a prior identical render instead of paying for it again.

  _Use it in iteration loops where most lines have not changed._

  ```bash
  fish-audio-pp-cli tts render --text "Hi, this is Pearl." --voice 802e3bc2b27e49c2995d23ef70e6ac89 --out greeting.mp3 --skip-if-rendered --agent
  ```
- **`render diff`** — Show the cost, model, and byte deltas between two past renders.

  _Use it when picking between two takes of the same line._

  ```bash
  fish-audio-pp-cli render diff 1 2 --agent
  ```

### Verify before you ship
- **`voice verify`** — Render a reference phrase with a cloned voice, transcribe it back, and report word-error-rate.

  _Run it before swapping a production voice to a new clone._

  ```bash
  fish-audio-pp-cli voice verify 7f92f8afb8ec43bf81429cc1c9199cb1 --agent
  ```
- **`tts batch`** — Estimate a batch's cost against your live API credit and refuse to start if it would overdraw.

  _Use it for unattended batch jobs so a long script cannot fail halfway on credit._

  ```bash
  fish-audio-pp-cli tts batch --input lines.txt --voice 802e3bc2b27e49c2995d23ef70e6ac89 --out-dir ./out --budget-guard --agent
  ```

## Recipes

### Render a greeting and check its cost

```bash
fish-audio-pp-cli tts render --text "Hi, this is Pearl." --voice 802e3bc2b27e49c2995d23ef70e6ac89 --out greeting.mp3 --agent --select file,bytes,cost_usd,model
```

Writes the file and returns only the fields an agent needs to log.

### Batch a script with a budget guard

```bash
fish-audio-pp-cli tts batch --input lines.txt --voice 802e3bc2b27e49c2995d23ef70e6ac89 --out-dir ./out --budget-guard --agent
```

Refuses to start if the estimated cost exceeds live API credit.

### Verify a clone before going live

```bash
fish-audio-pp-cli voice verify 7f92f8afb8ec43bf81429cc1c9199cb1 --agent
```

Reports word-error-rate from a TTS-then-ASR round trip.

### Find a public voice

```bash
fish-audio-pp-cli voice discover --query "calm British male" --limit 5 --agent --select id,title,languages
```

Searches the cached public catalog with FTS.

## Usage

Run `fish-audio-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `FISH_AUDIO_CONFIG_DIR`, `FISH_AUDIO_DATA_DIR`, `FISH_AUDIO_STATE_DIR`, or `FISH_AUDIO_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `FISH_AUDIO_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export FISH_AUDIO_HOME=/srv/fish-audio
fish-audio-pp-cli doctor
```

Under `FISH_AUDIO_HOME=/srv/fish-audio`, the four dirs resolve to `/srv/fish-audio/config`, `/srv/fish-audio/data`, `/srv/fish-audio/state`, and `/srv/fish-audio/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "fish-audio": {
      "command": "fish-audio-pp-mcp",
      "env": {
        "FISH_AUDIO_HOME": "/srv/fish-audio"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `FISH_AUDIO_DATA_DIR` overrides an explicit `--home` for that kind. Use `FISH_AUDIO_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `FISH_AUDIO_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `fish-audio-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### asr

Manage asr

- **`fish-audio-pp-cli asr`** - Speech to Text

### model

Manage model

- **`fish-audio-pp-cli model create`** - Create Model for Users via API
- **`fish-audio-pp-cli model delete`** - Delete Model
- **`fish-audio-pp-cli model get`** - Get Model
- **`fish-audio-pp-cli model list`** - List Models
- **`fish-audio-pp-cli model update`** - Update Model

### tts

Manage tts

- **`fish-audio-pp-cli tts create`** - Text to Speech
- **`fish-audio-pp-cli tts create-stream`** - Text to Speech Stream with Timestamps

### voice-design

Manage voice design

- **`fish-audio-pp-cli voice-design`** - Voice Design

### wallet

Manage wallet



### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`fish-audio-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`fish-audio-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`fish-audio-pp-cli learnings list`** - Inspect taught rows
- **`fish-audio-pp-cli learnings forget <query>`** - Undo a teach
- **`fish-audio-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`fish-audio-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`fish-audio-pp-cli teach-pattern`** - Install a query/resource template up front
- **`fish-audio-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `FISH_AUDIO_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `fish-audio-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
fish-audio-pp-cli model list

# JSON for scripting and agents
fish-audio-pp-cli model list --json
# Filter to specific fields
fish-audio-pp-cli model list --json --select author,cover_image,created_at

# Dry run — show the request without sending
fish-audio-pp-cli model list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
fish-audio-pp-cli model list --agent
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

## Health Check

```bash
fish-audio-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `fish-audio-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/fish-audio-pp-cli/config.toml`; `--home`, `FISH_AUDIO_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `FISH_AUDIO_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `fish-audio-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `fish-audio-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $FISH_AUDIO_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **audio sounds like a different model than requested** — The API falls back to s2.1-pro on unknown model strings; run `fish-audio-pp-cli tts resolve --voice 7f92f8afb8ec43bf81429cc1c9199cb1 --model s2.1-pro` to validate before rendering.
- **402 or 'insufficient credit' on render** — Run `fish-audio-pp-cli wallet balance`; dev API credit and subscription package are separate ledgers.
- **multi-speaker tags rejected** — `<|speaker:N|>` tags need an S2-family model; pass `--model s2.1-pro`.
- **429 on batch jobs** — Concurrency is capped by spend tier (5/15/50 slots); lower `--concurrency` on `tts batch`.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**elevenlabs-pp-cli**](https://github.com/mvanhorn/printing-press-library) — Go (1965 stars)
- [**fish-audio-python**](https://github.com/fishaudio/fish-audio-python) — Python (211 stars)
- [**fish-audio-typescript**](https://github.com/fishaudio/fish-audio-typescript) — TypeScript (14 stars)
- [**mcp-fish-audio-server**](https://github.com/da-okazaki/mcp-fish-audio-server) — TypeScript (13 stars)
- [**fish-audio-go**](https://github.com/fishaudio/fish-audio-go) — Go (7 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
