---
name: pp-cartesia
description: "Manage Cartesia voice agents, deployments, calls, metrics, and voices from the terminal — with a local SQLite... Trigger phrases: `audit my cartesia agent`, `grep cartesia call transcripts`, `diff cartesia deployments`, `estimate cartesia credits`, `use cartesia`, `run cartesia`."
author: "user"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - cartesia-pp-cli
---

# Cartesia — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `cartesia-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install cartesia --cli-only
   ```
2. Verify: `cartesia-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Every Cartesia endpoint as a typed command, plus the offline joins the API can't do in one shot: deployment diff, transcript grep, metric trends, agent audit. Designed for AI agents and humans alike.

## When to Use This CLI

Reach for cartesia-pp-cli when iterating on a production Cartesia voice agent: tweaking prompts, watching deployments roll out, grepping call transcripts for failure modes, attaching LLM-judge metrics, or pulling regression reports. The local SQLite mirror is what makes the compound commands fast and offline-friendly.

## Unique Capabilities

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

## Command Reference

**access-token** — Manage access token

- `cartesia-pp-cli access-token` — Generates a new Access Token for the client. These tokens are short-lived and should be used to make requests to the...

**agents** — Manage agents

- `cartesia-pp-cli agents create-metric` — Create a new metric.
- `cartesia-pp-cli agents delete` — Delete Agent
- `cartesia-pp-cli agents download-call-audio` — The downloaded audio file is in .wav format. This endpoint streams the audio file content (WAV format) to the client.
- `cartesia-pp-cli agents export-metric-results` — Export metric results to a CSV file. This endpoint streams at most 100k results as the CSV file directly to the...
- `cartesia-pp-cli agents get` — Returns the details of a specific agent. To create an agent, use the CLI or the Playground for the best experience...
- `cartesia-pp-cli agents get-call` — Get Call
- `cartesia-pp-cli agents get-deployment` — Get a deployment by its ID.
- `cartesia-pp-cli agents get-metric` — Get a metric by its ID.
- `cartesia-pp-cli agents list` — Lists all agents associated with your account.
- `cartesia-pp-cli agents list-calls` — Lists calls sorted by start time in descending order for a specific agent. `agent_id` is required and if you want to...
- `cartesia-pp-cli agents list-metric-results` — Paginated list of metric results. Filter results using the query parameters,
- `cartesia-pp-cli agents list-metrics` — List of all LLM-as-a-Judge metrics owned by your account.
- `cartesia-pp-cli agents templates` — List of public, Cartesia-provided agent templates to help you get started.
- `cartesia-pp-cli agents update` — Update Agent

**datasets** — Manage datasets

- `cartesia-pp-cli datasets create` — Create a new dataset
- `cartesia-pp-cli datasets delete` — Delete a dataset
- `cartesia-pp-cli datasets get` — Retrieve a specific dataset by ID
- `cartesia-pp-cli datasets list` — Paginated list of datasets
- `cartesia-pp-cli datasets update` — Update an existing dataset

**fine-tunes** — Manage fine tunes

- `cartesia-pp-cli fine-tunes create` — Create a new fine-tune
- `cartesia-pp-cli fine-tunes delete` — Delete a fine-tune
- `cartesia-pp-cli fine-tunes get` — Retrieve a specific fine-tune by ID
- `cartesia-pp-cli fine-tunes list` — Paginated list of all fine-tunes for the authenticated user

**infill** — Manage infill

- `cartesia-pp-cli infill` — Infill (Bytes). Generate audio that smoothly connects two existing audio segments. This is useful for inserting new...

**pronunciation-dicts** — Manage pronunciation dicts

- `cartesia-pp-cli pronunciation-dicts create` — Create a new pronunciation dictionary
- `cartesia-pp-cli pronunciation-dicts delete` — Delete a pronunciation dictionary
- `cartesia-pp-cli pronunciation-dicts get` — Retrieve a specific pronunciation dictionary by ID
- `cartesia-pp-cli pronunciation-dicts list` — List all pronunciation dictionaries for the authenticated user
- `cartesia-pp-cli pronunciation-dicts update` — Update a pronunciation dictionary

**stt** — Manage stt

- `cartesia-pp-cli stt` — Transcribes audio files into text using Cartesia's Speech-to-Text API. Upload an audio file and receive a complete...

**tts** — Manage tts

- `cartesia-pp-cli tts bytes` — Text-to-Speech (Bytes). The simplest way to stream generated audio. See [Compare TTS...
- `cartesia-pp-cli tts sse` — Text-to-Speech (SSE). Supports: - Streaming - Timestamps - context_id without transcript buffering See [Compare TTS...

**voice-changer** — Manage voice changer

- `cartesia-pp-cli voice-changer bytes` — Voice Changer (Bytes). Takes an audio file of speech, and returns an audio file of speech spoken with the same...
- `cartesia-pp-cli voice-changer sse` — Voice Changer (SSE)

**voices** — Manage voices

- `cartesia-pp-cli voices clone` — Clone a high similarity voice from an audio clip. Clones are more similar to the source clip, but may reproduce...
- `cartesia-pp-cli voices delete` — Delete Voice
- `cartesia-pp-cli voices get` — Get Voice
- `cartesia-pp-cli voices list` — List Voices
- `cartesia-pp-cli voices localize` — Create a new voice from an existing voice localized to a new language and dialect.
- `cartesia-pp-cli voices update` — Update the name, description, and gender of a voice. To set the gender back to the default, set the gender to...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
cartesia-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Find calls where the agent misheard a date

```bash
cartesia-pp-cli calls grep yesterday --agent agent_42 --turns 1 --json --select id,start_time,agent_id
```

FTS-backed search over local transcripts with dotted --select so the response stays under one screen.

### Audit yesterday's deployment for regressions

```bash
cartesia-pp-cli agents audit agent_42 --since 24h --regression-threshold 0.1 --json
```

Joins calls + metric_results + deployments offline; flags any metric that dropped more than 10% from the prior deployment.

### Pick a Spanish voice from a description

```bash
cartesia-pp-cli voices find warm --lang es --gender female --agent
```

FTS5 + structured filters over the locally cached voice catalog; returns the top match in agent-ready JSON.

### Estimate credits before kicking off a fine-tune

```bash
cartesia-pp-cli usage estimate fine-tune --dataset ds_42 --agent
```

Compares the planned fine-tune to your local usage history so you don't pay for an experiment you'd skip.

### Find the worst-scored calls this week

```bash
cartesia-pp-cli calls worst --since 7d --limit 20 --json
```

Cross-agent ranking from local metric_results joined to call transcripts; surfaces the lowest scorers without touching the API.

## Auth Setup

Cartesia ships two Bearer schemes on the same header: long-lived API keys for backend automation and short-lived JWT access tokens (obtained from POST /access-token with TTS or STT grants) for browser/edge clients. `auth set-token` stores the API key in OS-appropriate config; `auth access-token` mints a JWT.

Run `cartesia-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  cartesia-pp-cli agents list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
cartesia-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
cartesia-pp-cli feedback --stdin < notes.txt
cartesia-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.cartesia-pp-cli/feedback.jsonl`. They are never POSTed unless `CARTESIA_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `CARTESIA_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
cartesia-pp-cli profile save briefing --json
cartesia-pp-cli --profile briefing agents list
cartesia-pp-cli profile list --json
cartesia-pp-cli profile show briefing
cartesia-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Async Jobs

For endpoints that submit long-running work, the generator detects the submit-then-poll pattern (a `job_id`/`task_id`/`operation_id` field in the response plus a sibling status endpoint) and wires up three extra flags on the submitting command:

| Flag | Purpose |
|------|---------|
| `--wait` | Block until the job reaches a terminal status instead of returning the job ID immediately |
| `--wait-timeout` | Maximum wait duration (default 10m, 0 means no timeout) |
| `--wait-interval` | Initial poll interval (default 2s; grows with exponential backoff up to 30s) |

Use async submission without `--wait` when you want to fire-and-forget; use `--wait` when you want one command to return the finished artifact.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `cartesia-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add cartesia-pp-mcp -- cartesia-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which cartesia-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   cartesia-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `cartesia-pp-cli <command> --help`.
