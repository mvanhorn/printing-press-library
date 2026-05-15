# Cartesia CLI

**Manage Cartesia voice agents, deployments, calls, metrics, and voices from the terminal — with a local SQLite mirror that makes agent grep, drift, and audit possible.**

Every Cartesia endpoint as a typed command, plus the offline joins the API can't do in one shot: deployment diff, transcript grep, metric trends, agent audit. Designed for AI agents and humans alike.

## Install

The recommended path installs both the `cartesia-pp-cli` binary and the `pp-cartesia` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install cartesia
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install cartesia --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/cartesia-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-cartesia --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-cartesia --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-cartesia skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-cartesia. The skill defines how its required CLI can be installed.
```

## Authentication

Cartesia ships two Bearer schemes on the same header: long-lived API keys for backend automation and short-lived JWT access tokens (obtained from POST /access-token with TTS or STT grants) for browser/edge clients. `auth set-token` stores the API key in OS-appropriate config; `auth access-token` mints a JWT.

## Quick Start

```bash
# Store your Cartesia API key so every subsequent call is authenticated.
cartesia-pp-cli auth set-token sk_car_your_key_here


# Verifies auth, base URL, and Cartesia-Version header before any real work.
cartesia-pp-cli doctor


# Pulls agents, voices, deployments, metrics, datasets into the local store so the compound commands work.
cartesia-pp-cli sync


# Once synced, this returns every transcript containing 'cancel' across all agents -- no extra API calls.
cartesia-pp-cli calls grep cancel --since 24h --json


# Joins calls, metric_results, and deployments locally for a single regression verdict.
cartesia-pp-cli agents audit agent_42 --since 24h --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`calls grep`** — Full-text + regex search across every synced call transcript, with windowed turns of context, agent filter, and time window.

  _When an agent says 'find every call where the customer asked about cancellation,' this is the single answer no API call gives._

  ```bash
  cartesia-pp-cli calls grep 'refund' --agent agent_42 --turns 2 --since 7d --json
  ```
- **`agents audit`** — Joins the last 24h of calls + each call's metric results + the deployment that handled it, flagging regressions vs the prior deployment.

  _Pick this when an agent wants a single 'is my voice agent regressing right now' verdict._

  ```bash
  cartesia-pp-cli agents audit agent_42 --since 24h --regression-threshold 0.1 --json
  ```
- **`agents diff`** — Side-by-side diff of prompt, voice, model, and config between any two deployments of the same agent.

  _Use this when an agent says 'why did this start failing after my last push?'_

  ```bash
  cartesia-pp-cli agents diff agent_42 --from dep_prev --to dep_curr --json
  ```
- **`metrics trend`** — Bucketed pass-rate or score-mean for any LLM-judge metric over time, sliced by deployment.

  _Reach for this when an agent wants to know 'is quality going up or down per release?'_

  ```bash
  cartesia-pp-cli metrics trend metric_pass_rate --bucket day --agent agent_42 --since 30d --json
  ```
- **`agents since`** — Every call plus every flagged metric result on a given agent since a timestamp, in one chronological feed.

  _Use this when an agent comes back from a break and asks 'what happened on my line while I was away?'_

  ```bash
  cartesia-pp-cli agents since agent_42 2h --json
  ```
- **`calls worst`** — Surfaces the lowest-scored calls across all agents in one window, with transcript snippets.

  _Use this when an agent says 'show me the worst things our voice agents said this week.'_

  ```bash
  cartesia-pp-cli calls worst --metric metric_csat --since 7d --limit 20 --json
  ```
- **`voices changelog`** — Voices created, updated, deleted since the last sync, with metadata diffs.

  _Use this when an agent needs to know which voices changed before a deployment._

  ```bash
  cartesia-pp-cli voices changelog --since 7d --json
  ```
- **`voices find`** — Free-text style match ("warm female, mid-pitch, Spanish") against locally cached voice catalog with structured filters.

  _Use this when an agent needs to pick a voice from a description, not an id._

  ```bash
  cartesia-pp-cli voices find 'warm female mid-pitch' --lang es --gender female --json
  ```
- **`sql`** — Read-only SELECT over the local SQLite store — agents, calls, transcripts, metrics, deployments.

  _Reach for this when no canned subcommand fits the question._

  ```bash
  cartesia-pp-cli sql "SELECT id, summary FROM calls ORDER BY start_time DESC LIMIT 20" --json
  ```

### Cost intelligence
- **`usage estimate`** — Estimates credit cost for a planned operation (fine-tune, localize) from local usage history.

  _Use this before committing to a billable operation an agent is about to kick off._

  ```bash
  cartesia-pp-cli usage estimate fine-tune --dataset ds_42 --json
  ```

## Usage

Run `cartesia-pp-cli --help` for the full command reference and flag list.

## Commands

### access-token

Manage access token

- **`cartesia-pp-cli access-token auth`** - Generates a new Access Token for the client. These tokens are short-lived and should be used to make requests to the API from authenticated clients.

### agents

Manage agents

- **`cartesia-pp-cli agents create-metric`** - Create a new metric.
- **`cartesia-pp-cli agents delete`** - Delete Agent
- **`cartesia-pp-cli agents download-call-audio`** - The downloaded audio file is in .wav format. This endpoint streams the audio file content (WAV format) to the client.
- **`cartesia-pp-cli agents export-metric-results`** - Export metric results to a CSV file. This endpoint streams at most 100k results as the CSV file directly to the client. Use the optional filters to narrow down the results to export.
- **`cartesia-pp-cli agents get`** - Returns the details of a specific agent. To create an agent, use the CLI or the Playground for the best experience and integration with Github.
- **`cartesia-pp-cli agents get-call`** - Get Call
- **`cartesia-pp-cli agents get-deployment`** - Get a deployment by its ID.
- **`cartesia-pp-cli agents get-metric`** - Get a metric by its ID.
- **`cartesia-pp-cli agents list`** - Lists all agents associated with your account.
- **`cartesia-pp-cli agents list-calls`** - Lists calls sorted by start time in descending order for a specific agent. `agent_id` is required and if you want to include `transcript` in the response, add `expand=transcript` to the request. This endpoint is paginated.
- **`cartesia-pp-cli agents list-metric-results`** - Paginated list of metric results. Filter results using the query parameters,
- **`cartesia-pp-cli agents list-metrics`** - List of all LLM-as-a-Judge metrics owned by your account.
- **`cartesia-pp-cli agents templates`** - List of public, Cartesia-provided agent templates to help you get started.
- **`cartesia-pp-cli agents update`** - Update Agent

### datasets

Manage datasets

- **`cartesia-pp-cli datasets create`** - Create a new dataset
- **`cartesia-pp-cli datasets delete`** - Delete a dataset
- **`cartesia-pp-cli datasets get`** - Retrieve a specific dataset by ID
- **`cartesia-pp-cli datasets list`** - Paginated list of datasets
- **`cartesia-pp-cli datasets update`** - Update an existing dataset

### fine-tunes

Manage fine tunes

- **`cartesia-pp-cli fine-tunes create`** - Create a new fine-tune
- **`cartesia-pp-cli fine-tunes delete`** - Delete a fine-tune
- **`cartesia-pp-cli fine-tunes get`** - Retrieve a specific fine-tune by ID
- **`cartesia-pp-cli fine-tunes list`** - Paginated list of all fine-tunes for the authenticated user

### infill

Manage infill

- **`cartesia-pp-cli infill bytes`** - Infill (Bytes).

Generate audio that smoothly connects two existing audio segments. This is useful for inserting new speech between existing speech segments while maintaining natural transitions.

**The cost is 1 credit per character of the infill text plus a fixed cost of 300 credits.**

At least one of `left_audio` or `right_audio` must be provided.

As with all generative models, there's some inherent variability, but here's some tips we recommend to get the best results from infill:
- Use longer infill transcripts
  - This gives the model more flexibility to adapt to the rest of the audio
- Target natural pauses in the audio when deciding where to clip
  - This means you don't need word-level timestamps to be as precise
- Clip right up to the start and end of the audio segment you want infilled, keeping as much silence in the left/right audio segments as possible
  - This helps the model generate more natural transitions

### pronunciation-dicts

Manage pronunciation dicts

- **`cartesia-pp-cli pronunciation-dicts create`** - Create a new pronunciation dictionary
- **`cartesia-pp-cli pronunciation-dicts delete`** - Delete a pronunciation dictionary
- **`cartesia-pp-cli pronunciation-dicts get`** - Retrieve a specific pronunciation dictionary by ID
- **`cartesia-pp-cli pronunciation-dicts list`** - List all pronunciation dictionaries for the authenticated user
- **`cartesia-pp-cli pronunciation-dicts update`** - Update a pronunciation dictionary

### stt

Manage stt

- **`cartesia-pp-cli stt transcribe`** - Transcribes audio files into text using Cartesia's Speech-to-Text API.

Upload an audio file and receive a complete transcription response. Supports arbitrarily long audio files with automatic intelligent chunking for longer audio.

**Supported audio formats:** flac, m4a, mp3, mp4, mpeg, mpga, oga, ogg, wav, webm

**Response format:** Returns JSON with transcribed text, duration, and language. Include `timestamp_granularities: ["word"]` to get word-level timestamps.
**Pricing:** Batch transcription is priced at **1 credit per 2 seconds** of audio processed.

<Note>
For migrating from the OpenAI SDK, see our [OpenAI Whisper to Cartesia Ink Migration Guide](/api-reference/stt/migrate-from-open-ai).
</Note>

### tts

Manage tts

- **`cartesia-pp-cli tts bytes`** - Text-to-Speech (Bytes).

The simplest way to stream generated audio.

See [Compare TTS Endpoints](https://docs.cartesia.ai/use-the-api/compare-tts-endpoints) for details.
- **`cartesia-pp-cli tts sse`** - Text-to-Speech (SSE).

Supports:
  - Streaming
  - Timestamps
  - context_id without transcript buffering


See [Compare TTS Endpoints](https://docs.cartesia.ai/use-the-api/compare-tts-endpoints) for details.

### voice-changer

Manage voice changer

- **`cartesia-pp-cli voice-changer bytes`** - Voice Changer (Bytes).

Takes an audio file of speech, and returns an audio file of speech spoken with the same intonation, but with a different voice.

This endpoint is priced at 15 characters per second of input audio.
- **`cartesia-pp-cli voice-changer sse`** - Voice Changer (SSE)

### voices

Manage voices

- **`cartesia-pp-cli voices clone`** - Clone a high similarity voice from an audio clip. Clones are more similar to the source clip, but may reproduce background noise. For these, use an audio clip about 5 seconds long.
- **`cartesia-pp-cli voices delete`** - Delete Voice
- **`cartesia-pp-cli voices get`** - Get Voice
- **`cartesia-pp-cli voices list`** - List Voices
- **`cartesia-pp-cli voices localize`** - Create a new voice from an existing voice localized to a new language and dialect.
- **`cartesia-pp-cli voices update`** - Update the name, description, and gender of a voice. To set the gender back to the default, set the gender to `null`. If gender is not specified, the gender will not be updated.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
cartesia-pp-cli agents list

# JSON for scripting and agents
cartesia-pp-cli agents list --json

# Filter to specific fields
cartesia-pp-cli agents list --json --select id,name,status

# Dry run — show the request without sending
cartesia-pp-cli agents list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
cartesia-pp-cli agents list --agent
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
npx skills add mvanhorn/printing-press-library/cli-skills/pp-cartesia -g
```

Then invoke `/pp-cartesia <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add cartesia cartesia-pp-mcp -e CARTESIA_ACCESS_TOKEN=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/cartesia-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `CARTESIA_ACCESS_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "cartesia": {
      "command": "cartesia-pp-mcp",
      "env": {
        "CARTESIA_ACCESS_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
cartesia-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/cartesia-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `CARTESIA_ACCESS_TOKEN` | per_call | No | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `cartesia-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $CARTESIA_ACCESS_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 from any command** — Run `cartesia-pp-cli auth status`; if missing, `auth set-token` or `export CARTESIA_API_KEY=sk_car_...`.
- **Cartesia-Version header rejected** — Set `CARTESIA_API_VERSION` env var to the date string from `cartesia-pp-cli status` (currently 2025-04-16).
- **calls grep returns nothing** — Calls aren't synced until you run `sync --resource calls --full`. The grep is offline-first.
- **TTS websocket disconnects mid-stream** — Reduce concurrency: WebSocket multiplexes contexts but caps per connection. Use --max-contexts to bound.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**cartesia-js**](https://github.com/cartesia-ai/cartesia-js) — TypeScript (131 stars)
- [**cartesia-python**](https://github.com/cartesia-ai/cartesia-python) — Python (121 stars)
- [**line**](https://github.com/cartesia-ai/line) — Python (100 stars)
- [**cartesia-mcp**](https://github.com/cartesia-ai/cartesia-mcp) — Python (13 stars)
- [**ai-say-cli**](https://www.npmjs.com/package/ai-say-cli) — JavaScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
