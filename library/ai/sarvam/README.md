# Sarvam AI CLI

**Every Sarvam AI model on your terminal — translate, speak, transcribe, chat, and extract documents in 22 Indian languages, with a local history that compounds**

Sarvam AI's official SDKs and MCP server are great for code, but nothing offers offline capability: no local history of translations, TTS generations, transcriptions, or chat threads. sarvam-pp-cli adds a local SQLite store, voice auditioning, conversation resume, batch job retry/report, pronunciation spot-checks, subtitle export, and a doc-ai extraction schema library — all with --json, --dry-run, and typed exit codes for agents and scripts.

## Install

The recommended path installs both the `sarvam-pp-cli` binary and the `pp-sarvam` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install sarvam
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install sarvam --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install sarvam --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install sarvam --agent claude-code
npx -y @mvanhorn/printing-press-library install sarvam --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/sarvam/cmd/sarvam-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/sarvam-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install sarvam --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-sarvam --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-sarvam --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install sarvam --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/sarvam-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SARVAM_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/ai/sarvam/cmd/sarvam-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "sarvam": {
      "command": "sarvam-pp-mcp",
      "env": {
        "SARVAM_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Authentication uses the Sarvam AI API subscription key (sk_ format). Set it with `export SARVAM_API_KEY=sk_...` or `sarvam-pp-cli auth set-token`. The key goes in the `api-subscription-key` header (or `Authorization: Bearer`). Note: an invalid key returns HTTP 403 with `invalid_api_key_error`, not 401 — treat 403 as the auth-failure signal. The separate Voice Agents platform (apps.sarvam.ai) uses a different X-API-Key system and is out of scope for this CLI.

## Quick Start

```bash
# Verify the CLI is installed and configured
sarvam-pp-cli doctor --dry-run

# First translation — the core workflow
sarvam-pp-cli translate --input "Hello, how are you?" --source-language-code en-IN --target-language-code hi-IN

# Generate speech audio
sarvam-pp-cli text-to-speech text-to-speech --text "नमस्ते, स्वागत है" --language-code hi-IN --speaker shubh

# Ask the multilingual chat model
sarvam-pp-cli chat --messages '[{"role":"user","content":"What is Sarvam AI?"}]' --model sarvam-105b

# Search past work offline
sarvam-pp-cli search "नमस्ते" --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Speech workflow
- **`voices preview`** — Generate one sample sentence across every TTS speaker and hear them all side by side

  _Use when choosing a TTS voice for a new language or prompt set without burning manual API calls_

  ```bash
  sarvam-pp-cli voices preview --lang hi-IN --sample "नमस्ते, स्वागत है" --speakers shubh,ritu,priya --output ./voices
  ```
- **`pron-check`** — Verify a term's TTS pronunciation via a speech round-trip (TTS then STT)

  _Use to confirm a pronunciation dictionary edit took effect before shipping IVR prompts_

  ```bash
  sarvam-pp-cli pron-check "SarvamPay" --lang hi-IN
  ```

### Local state that compounds
- **`chat resume`** — Continue a past chat thread from local history with full context

  _Use to continue an assistant session without losing context, offline from the original thread_

  ```bash
  sarvam-pp-cli chat resume 20260814_2d09e061 "what was our conclusion?"
  ```
- **`subs`** — Emit .srt/.vtt subtitles from timestamped transcriptions in local history

  _Use to turn a timestamped transcription into subtitles without a throwaway script_

  ```bash
  sarvam-pp-cli subs --from last --format srt --output subtitles.srt
  ```

### Job orchestration
- **`stt-job retry`** — Re-run only the failed files of a batch STT job with one command

  _Use when a batch job partially fails and you need to reprocess just the failures_

  ```bash
  sarvam-pp-cli stt-job retry 20260707_9f1c2b3a-4d5e-6f70-8a9b-c0d1e2f3a4b5 --failed-only --dir ./audio/
  ```
- **`stt-job report`** — Per-file digest of a batch STT job with typed exit codes for cron alerting

  _Use in cron to alert when a batch transcription job degrades_

  ```bash
  sarvam-pp-cli stt-job report 20260707_9f1c2b3a-4d5e-6f70-8a9b-c0d1e2f3a4b5 --json
  ```

### Document intelligence
- **`docai schema list`** — Save, list, and diff doc-ai extraction schemas locally

  _Use to version extraction schemas so schema changes never silently break extraction runs_

  ```bash
  sarvam-pp-cli docai schema list
  ```
- **`docai batch`** — Run a saved extraction schema over a folder of documents with job pacing

  _Use for weekly batch document extraction (KYC, invoices) without writing orchestration code_

  ```bash
  sarvam-pp-cli docai batch --schema invoice-v1 --dir ./docs/ --out ./results/
  ```

## Recipes

### Translate a support reply

```bash
sarvam-pp-cli translate --input "Your EMI of Rs. 3000 is pending" --source-language-code en-IN --target-language-code hi-IN --mode formal
```

Translate a single message with formal tone for customer communications

### Audition voices for a new prompt set

```bash
sarvam-pp-cli voices preview --lang hi-IN --sample "नमस्ते, स्वागत है" --speakers shubh,ritu,priya
```

Hear 3 voices on the same sample before committing to a speaker

### Subtitle a promo video

```bash
sarvam-pp-cli subs --from last --format srt --output subtitles.srt
```

Turn the last timestamped transcription into SRT subtitles

### Reprocess failed batch files

```bash
sarvam-pp-cli stt-job retry 20260707_9f1c2b3a-4d5e-6f70-8a9b-c0d1e2f3a4b5 --failed-only --dir ./audio/
```

Re-run only the files that failed in a batch transcription job

### Check a pronunciation dictionary

```bash
sarvam-pp-cli pron-check "SarvamPay" --lang hi-IN --dict p_5cb7faa6
```

Verify a custom pronunciation actually changed how the term sounds

### Extract fields from a folder of documents

```bash
sarvam-pp-cli docai batch --schema invoice-v1 --dir ./docs/ --out ./results/
```

Run a saved extraction schema over every document with pacing

### Continue a past chat session

```bash
sarvam-pp-cli chat resume 20260814_2d09e061 "what was our conclusion?"
```

Pick up an assistant conversation where it left off, with full context

## Usage

Run `sarvam-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `SARVAM_CONFIG_DIR`, `SARVAM_DATA_DIR`, `SARVAM_STATE_DIR`, or `SARVAM_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `SARVAM_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export SARVAM_HOME=/srv/sarvam
sarvam-pp-cli doctor
```

Under `SARVAM_HOME=/srv/sarvam`, the four dirs resolve to `/srv/sarvam/config`, `/srv/sarvam/data`, `/srv/sarvam/state`, and `/srv/sarvam/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "sarvam": {
      "command": "sarvam-pp-mcp",
      "env": {
        "SARVAM_HOME": "/srv/sarvam"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `SARVAM_DATA_DIR` overrides an explicit `--home` for that kind. Use `SARVAM_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `SARVAM_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `sarvam-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### chat

Manage chat

- **`sarvam-pp-cli chat`** - Creates a model response for the given chat conversation. Serves sarvam-105b and sarvam-105b-conversations models. Supports streaming via server-sent events, tool calling, and structured outputs.

### doc-ai

Manage doc ai

- **`sarvam-pp-cli doc-ai digitise`** - Creates and starts a digitisation job from files or pre-uploaded handles. Converts documents to machine-readable text with layout awareness. Runs asynchronously; poll status until terminal.
- **`sarvam-pp-cli doc-ai download-url`** - Returns a presigned URL to download the output of a completed doc-ai job.
- **`sarvam-pp-cli doc-ai extract`** - Creates and starts an extract job from files or pre-uploaded handles. Extract pulls structured fields out of documents according to a schema. Exactly one of file and upload_ids must be provided; exactly one of schema and config_id is required. Runs asynchronously.
- **`sarvam-pp-cli doc-ai results`** - Fetches the results of a completed doc-ai job, including extracted fields and annotations with confidence scores.
- **`sarvam-pp-cli doc-ai status`** - Polls the status of a doc-ai job until a terminal status (completed, partially_completed, failed, rejected).
- **`sarvam-pp-cli doc-ai upload`** - Creates a presigned URL to upload a document for doc-ai processing.

### models

Manage models

- **`sarvam-pp-cli models`** - Lists the model IDs this deployment currently serves. A model is only served where its backend is configured, so the list is environment-specific — fetch it rather than hardcoding IDs.

### speech-to-text

Manage speech to text

- **`sarvam-pp-cli speech-to-text speech-to-text`** - Transcribes speech to text in multiple Indian languages and English. Accepts an audio file via multipart form-data. Supports transcribe, translate, verbatim, translit, and codemix modes.
- **`sarvam-pp-cli speech-to-text stt-job-download`** - Returns presigned download URLs for the output files of a completed batch speech-to-text job.
- **`sarvam-pp-cli speech-to-text stt-job-initiate`** - Creates a new speech-to-text bulk job and returns a job UUID and storage details for processing multiple audio files.
- **`sarvam-pp-cli speech-to-text stt-job-start`** - Starts processing a speech-to-text bulk job after all audio files have been uploaded.
- **`sarvam-pp-cli speech-to-text stt-job-status`** - Returns the status of a batch speech-to-text job including per-file details and download information.
- **`sarvam-pp-cli speech-to-text stt-job-upload`** - Generates presigned upload URLs for audio files that will be processed in a speech-to-text bulk job.

### text-lid

Manage text lid

- **`sarvam-pp-cli text-lid`** - Identifies the language (e.g. hi-IN) and script (e.g. Devanagari) of the input text, supporting 10+ Indian languages and English.

### text-to-speech

Manage text to speech

- **`sarvam-pp-cli text-to-speech create-pronunciation-dictionary`** - Uploads a .json file to create a new pronunciation dictionary. Only supported by bulbul:v3. The returned dictionary_id can be passed as dict_id in text-to-speech requests.
- **`sarvam-pp-cli text-to-speech delete-pronunciation-dictionary`** - Deletes a pronunciation dictionary by ID.
- **`sarvam-pp-cli text-to-speech get-pronunciation-dictionary`** - Fetches a single pronunciation dictionary by ID.
- **`sarvam-pp-cli text-to-speech list-pronunciation-dictionaries`** - Lists all pronunciation dictionaries for the user. Dictionaries define custom word pronunciations used by bulbul:v3 TTS.
- **`sarvam-pp-cli text-to-speech stream`** - Converts the input text into a streamed spoken audio response using the specified output codec (e.g. MP3). Returns binary audio.
- **`sarvam-pp-cli text-to-speech text-to-speech`** - Converts text into spoken audio. The output is a base64-encoded audio string (WAV by default) that must be decoded before use. Supports bulbul:v2 and bulbul:v3 models with 30+ voices.
- **`sarvam-pp-cli text-to-speech update-pronunciation-dictionary`** - Updates an existing pronunciation dictionary with a new JSON file.

### translate

Manage translate

- **`sarvam-pp-cli translate`** - Converts text from one language to another while preserving meaning. Supports 22 Indic languages plus English.

### transliterate

Manage transliterate

- **`sarvam-pp-cli transliterate`** - Transliterates text from one script to another (e.g. Devanagari to Latin) while keeping the same language and pronunciation.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`sarvam-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`sarvam-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`sarvam-pp-cli learnings list`** - Inspect taught rows
- **`sarvam-pp-cli learnings forget <query>`** - Undo a teach
- **`sarvam-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`sarvam-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`sarvam-pp-cli teach-pattern`** - Install a query/resource template up front
- **`sarvam-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `SARVAM_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `sarvam-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
sarvam-pp-cli models

# JSON for scripting and agents
sarvam-pp-cli models --json
# Filter to specific fields
sarvam-pp-cli models --json --select created,id,object

# Dry run — show the request without sending
sarvam-pp-cli models --dry-run

# Agent mode — JSON + compact + no prompts in one flag
sarvam-pp-cli models --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
sarvam-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `sarvam-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/sarvam-ai-pp-cli/config.toml`; `--home`, `SARVAM_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SARVAM_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `sarvam-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `sarvam-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SARVAM_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **HTTP 403 invalid_api_key_error** — The key is invalid or expired — not a 401. Re-run `sarvam-pp-cli auth set-token` with a fresh key from the Sarvam dashboard.
- **TTS pitch/loudness rejected on bulbul:v3** — pitch and loudness are bulbul:v2-only. Use `--model bulbul:v2` or drop those flags.
- **Chat returns content: null with only reasoning** — Set `--max-tokens` explicitly — hidden reasoning can consume the token budget before content.
- **Translate rejects >1000 chars** — mayura:v1 caps at 1000 chars. Use `--model sarvam-translate:v1` for up to 2000 chars.
- **Rate limit 429** — Sarvam uses token-bucket limits (translate 60/min, TTS 60/min, chat 40/min on sarvam-105b). Add pacing or retry with backoff.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**sarvamai/skills**](https://github.com/sarvamai/skills) — Markdown (69 stars)
- [**sarvam-mcp**](https://github.com/sarvamai/sarvam-mcp) — Python (32 stars)
- [**sarvamai-cli**](https://github.com/indic-ai-contribs/sarvamai-cli) — JavaScript
- [**Sarvam-cli**](https://github.com/preethamak/Sarvam-cli) — Python
- [**sarvam-ai-sdk**](https://github.com/sarvamai/sarvam-ai-sdk) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
