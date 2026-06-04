---
name: pp-ollama-cloud
description: "Routes every prompt to the right hosted Ollama model. Wraps chat, embeddings, catalog, and the OpenAI-compatible... Trigger phrases: `pick an ollama model`, `advise on a model`, `which ollama cloud model should I use`, `route this prompt`, `use ollama cloud`, `run ollama cloud`."
author: "user"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - ollama-cloud-pp-cli
---

# Ollama Cloud — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `ollama-cloud-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer into a user bin directory:
   ```bash
   npx -y @mvanhorn/printing-press-library install ollama-cloud --cli-only --bin-dir ~/.local/bin
   ```
2. Verify: `ollama-cloud-pp-cli --version`
3. Ensure `~/.local/bin` is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/ollama-cloud/cmd/ollama-cloud-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Pick this CLI when you need agent-driven access to Ollama's hosted catalog and want routing intelligence baked in. Especially useful when the alternative is hardcoding a model name and hoping it stays right. Pairs naturally with multi-backend setups where ollama-cloud is one of several providers.

## Unique Capabilities

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

## Command Reference

**chat** — Manage chat

- `ollama-cloud-pp-cli chat chat` — Native Ollama chat endpoint. Supports streaming.
- `ollama-cloud-pp-cli chat completions` — OpenAI-compatible chat completions endpoint.

**embeddings** — Manage embeddings

- `ollama-cloud-pp-cli embeddings embed` — Native Ollama embeddings endpoint.
- `ollama-cloud-pp-cli embeddings openai-embed` — Generate embeddings (OpenAI-compatible)

**models** — Manage models

- `ollama-cloud-pp-cli models` — Catalog in OpenAI list-models format.

**ps** — Manage ps

- `ollama-cloud-pp-cli ps` — Shows currently-loaded models. On Ollama Cloud this typically reflects models with active sessions.

**show** — Manage show

- `ollama-cloud-pp-cli show` — Returns model metadata, template, modelfile, capabilities.

**tags** — Manage tags

- `ollama-cloud-pp-cli tags` — Returns the live catalog of hosted Ollama Cloud models.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
ollama-cloud-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Advise with explain trace as markdown

```bash
ollama-cloud-pp-cli advise --prompt-file ./prompt.txt --task-hint coding --explain --format md
```

--explain populates the feature trace and filtered-candidate audit; --format md renders for humans.

### Detect catalog drift

```bash
ollama-cloud-pp-cli advise --validate-catalog --json --select uncurated_live,curated_not_in_live
```

Diffs live /api/tags against curated models.json overlay; flags both uncurated live models and stale curated patterns.

### Replay last week recommendations

```bash
ollama-cloud-pp-cli advise-replay --since 7d --dry-run --json --select divergence_count,divergence_pct,emitted
```

Engagement canary; dotted-path select keeps response under context budget.

### Side-by-side comparison

```bash
ollama-cloud-pp-cli compare --prompt-file ./prompt.txt --models qwen3-coder:480b,gpt-oss:120b --json
```

Runs the prompt against both models in parallel and reports per-model latency, tokens, and rate-limit state.

### Cost rollup by task hint

```bash
ollama-cloud-pp-cli cost-trace --since 7d --group-by task-hint --json --select groups,row_count
```

Aggregates the advisor JSONL log by task-hint so you can decide whether to upgrade the Ollama Cloud tier.

## Auth Setup

Bearer auth via OLLAMA_CLOUD_API_KEY (intentionally distinct from any local-Ollama env var). Free tier is rate-limited weekly; the budget command surfaces exhaustion before workflows fail.

Run `ollama-cloud-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  ollama-cloud-pp-cli chat chat --model example-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

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
ollama-cloud-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
ollama-cloud-pp-cli feedback --stdin < notes.txt
ollama-cloud-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.ollama-cloud-pp-cli/feedback.jsonl`. They are never POSTed unless `OLLAMA_CLOUD_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `OLLAMA_CLOUD_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
ollama-cloud-pp-cli profile save briefing --json
ollama-cloud-pp-cli --profile briefing chat chat --model example-value
ollama-cloud-pp-cli profile list --json
ollama-cloud-pp-cli profile show briefing
ollama-cloud-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `ollama-cloud-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add ollama-cloud-pp-mcp -- ollama-cloud-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which ollama-cloud-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   ollama-cloud-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `ollama-cloud-pp-cli <command> --help`.
