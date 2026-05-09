---
name: pp-openrouter
description: "Agent-first OpenRouter introspection — terse output for cron and AI agents (--agent and --llm modes), local SQLite... Trigger phrases: `openrouter credits`, `check openrouter budget`, `openrouter cost by cron`, `shortlist openrouter models`, `openrouter providers degraded`, `use openrouter`, `run openrouter`."
author: "Rick van de Laar"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - openrouter-pp-cli
---

# OpenRouter — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `openrouter-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install openrouter --cli-only
   ```
2. Verify: `openrouter-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/openrouter/cmd/openrouter-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Other OpenRouter CLIs are chat REPLs. This one is built for the cron job and the AI agent calling out to `Bash`. Absorbed introspection commands (`credits`, `models`, `key`, `generation`, `providers`) honor the global `--agent` flag (sets --json --compact --no-input). Eight novel commands (`usage cost-by`, `models query`, `key eta`, `providers degraded`, `generation explain`, `usage anomaly`, `endpoints failover`, `budget`) ship a `--llm` mode that returns under 200 tokens of key:value output. A local SQLite catalog lets you query the 400+ model list with `models query "tools=true cost.completion<1"` instead of pasting 425KB of JSON into context.

## When to Use This CLI

Use this CLI when an AI agent needs OpenRouter introspection data and you're optimizing for token efficiency. Absorbed commands accept `--agent` for compact JSON; the 8 novel commands ship `--llm` mode for terse key:value output. Use it when you need cost attribution beyond model+provider (which-cron-fired-the-call). Use it for pre-flight gates in bash compositions and for local-catalog querying that would otherwise blow context. Skip it for chat — use `grahamking/ort` for chat ergonomics.

## Unique Capabilities

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

## Command Reference

**activity** — Manage activity

- `openrouter-pp-cli activity` — Returns user activity data grouped by endpoint for the last 30 (completed) UTC days. [Management...

**credits** — Credit management endpoints

- `openrouter-pp-cli credits` — Get total credits purchased and used for the authenticated user. [Management...

**endpoints** — Endpoint information

- `openrouter-pp-cli endpoints` — Preview the impact of ZDR on the available endpoints

**generation** — Generation history endpoints

- `openrouter-pp-cli generation get` — Get request & usage metadata for a generation
- `openrouter-pp-cli generation list-content` — Get stored prompt and completion content for a generation

**key** — Manage key

- `openrouter-pp-cli key` — Get information on the API key associated with the current authentication session

**keys** — Manage keys

- `openrouter-pp-cli keys create` — Create a new API key for the authenticated user. [Management key](/docs/guides/overview/auth/management-api-keys)...
- `openrouter-pp-cli keys delete` — Delete an existing API key. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- `openrouter-pp-cli keys get` — Get a single API key by hash. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- `openrouter-pp-cli keys list` — List all API keys for the authenticated user. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- `openrouter-pp-cli keys update` — Update an existing API key. [Management key](/docs/guides/overview/auth/management-api-keys) required.

**models** — Model information endpoints

- `openrouter-pp-cli models get` — List all models and their properties
- `openrouter-pp-cli models list-count` — Get total count of available models
- `openrouter-pp-cli models list-user` — List models filtered by user provider preferences, [privacy settings](https://openrouter.ai/docs/guides/privacy/provi...

**openrouter-auth** — Manage openrouter auth

- `openrouter-pp-cli openrouter-auth create-keys-code` — Create an authorization code for the PKCE flow to generate a user-controlled API key
- `openrouter-pp-cli openrouter-auth exchange-code-for-apikey` — Exchange an authorization code from the PKCE flow for a user-controlled API key

**providers** — Provider information endpoints

- `openrouter-pp-cli providers` — List all providers


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
openrouter-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Pre-flight gate before expensive cron

```bash
openrouter budget check scan-pipeline --json
```

Returns exit 0 if under cap or 8 if over. Compose with && in bash to gate expensive jobs without half-execution.

### Cost attribution for the week

```bash
openrouter usage cost-by --group cron --since 7d --llm
```

Returns one row per cron name with total cost — answers 'which automated job is burning my OpenRouter budget'.

### Shortlist tool-capable cheap deep-context models (with --select narrowing)

```bash
openrouter models query "tools=true cost.completion<1 ctx>=64k" --agent --select id,context_length,pricing.completion
```

Local SQLite + dotted --select narrows ~400 models to a handful of fields. Agent context cost: ~150 tokens vs ~425KB raw.

### Detect cost regression before credit-low alarm fires

```bash
openrouter usage anomaly --since 24h --baseline 7d --llm
```

Flags per-model days exceeding 2σ over trailing 7-day mean. Daily cron catches runaway loops 1-3 days before /credits hits zero.

### Pre-empt 429s by skipping degraded providers

```bash
openrouter providers degraded --json
```

Returns {added, degraded, removed} arrays from set-diff vs prior snapshot. Pipe-feeds your router/suspension logic. Replaces reactive 429-and-learn with leading-indicator polling.

## Auth Setup

Set `OPENROUTER_API_KEY` for per-call operations (creds, models, usage, generation lookup). For sub-key management (`keys list/create/delete`), set `OPENROUTER_MANAGEMENT_KEY` separately — OpenRouter's API splits these intentionally and this CLI honors the split. Both variables are read fresh per command; nothing is persisted to disk by default.

Run `openrouter-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  openrouter-pp-cli credits --agent --select id,name,status
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
openrouter-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
openrouter-pp-cli feedback --stdin < notes.txt
openrouter-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.openrouter-pp-cli/feedback.jsonl`. They are never POSTed unless `OPENROUTER_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `OPENROUTER_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
openrouter-pp-cli profile save briefing --json
openrouter-pp-cli --profile briefing credits
openrouter-pp-cli profile list --json
openrouter-pp-cli profile show briefing
openrouter-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

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

1. **Empty, `help`, or `--help`** → show `openrouter-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add openrouter-pp-mcp -- openrouter-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which openrouter-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   openrouter-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `openrouter-pp-cli <command> --help`.
