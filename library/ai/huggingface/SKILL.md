---
name: pp-huggingface
description: "A stack-aware model-intel CLI for local-inference rigs and autonomous agent fleets — not just an API mirror. Trigger phrases: `find quants of`, `should I switch to`, `will this run on llama.cpp`, `what's trending on HF`, `has this been benchmarked`, `watch this uploader`, `use huggingface`, `huggingface CLI`."
author: "Rick van de Laar"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - huggingface-pp-cli
    install:
      - kind: go
        bins: [huggingface-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/ml-ops/huggingface/cmd/huggingface-pp-cli
---

# Hugging Face — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `huggingface-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install huggingface --cli-only
   ```
2. Verify: `huggingface-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/ml-ops/huggingface/cmd/huggingface-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Closes the loop between Hugging Face discovery and the operational decisions only you can make: would this replace what I run today, will it load on my backend, has it already been benched. Every command is multi-runtime safe (TTY-agnostic, --json + stable schemas, exit-code semantics, flock-guarded state) so the same binary serves Rick at the terminal and JARVIS in a container.

## When to Use This CLI

Reach for this CLI whenever the answer to 'should I switch from what I run today?' would otherwise require a WebFetch dance through model cards, config.json files, GitHub PRs, and local config. The stack-aware commands (vs-current, backend-check, bench-history, find-feature) only make sense when the caller already has Rick's local-inference stack and eval pipeline available — at the cost of a tighter scope, every command answers an operational question that generic HF tools cannot.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

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

## Command Reference

**models** — Model repositories on the Hugging Face Hub. Includes card metadata, file tree, raw config.json.

- `huggingface-pp-cli models get` — Get full model card metadata for one model.
- `huggingface-pp-cli models list` — List or search models. Supports HF search/filter/sort/author params.
- `huggingface-pp-cli models raw` — Fetch raw file contents (config.json, README.md, tokenizer_config.json, etc.).
- `huggingface-pp-cli models tree` — List files in a model repo at a given revision.

**users** — Hugging Face users / organizations.

- `huggingface-pp-cli users <user>` — User/org overview: aggregate downloads, model count, recent uploads.


**Hand-written commands**

- `huggingface-pp-cli find-quants <base-model>` — Find quantized variants of a base model, sorted by uploader rep + size. Stack-aware allowlist + size-class filter...
- `huggingface-pp-cli trending` — What's trending on Hugging Face. Size-class + library + window filters that HF doesn't expose.
- `huggingface-pp-cli model-card <id>` — Stack-relevant model card: MoE active params loud, effective GGUF size, training data summary.
- `huggingface-pp-cli derivatives <base-id>` — Discover models fine-tuned from a given base model.
- `huggingface-pp-cli uploader-rep <user>` — Uploader reputation: aggregate downloads, recency, model count, trusted-uploader flag.
- `huggingface-pp-cli compare-quants <id1> <id2> [...]` — Side-by-side comparison of multiple quant variants: size, quant type, uploader, MoE active-param ratio.
- `huggingface-pp-cli eval-candidates` — Emit model-eval-harness-ready candidate list. Wires HF discovery into the eval loop.
- `huggingface-pp-cli find-feature <feature>` — Search models by architecture feature (mtp, mla, moe, gqa, sliding-window, rope-yarn, etc). Heuristic config.json +...
- `huggingface-pp-cli vs-current <id>` — Diff candidate against the model the named agent currently runs. Reads agents.defaults.models from openclaw.json.
- `huggingface-pp-cli backend-check <id>` — Backend-readiness oracle. Bundled support matrix with citations + source_checked dates. Catches the 'downloaded 30GB...
- `huggingface-pp-cli bench-history <id>` — Join HF id with local model-eval-harness results. Stops re-evaluating known losers.
- `huggingface-pp-cli watch <target>` — Add a target (uploader / base-model / feature) to the watch list for periodic polling.
- `huggingface-pp-cli watch-poll` — Cron-callable: check watch targets for new matches since last poll, emit events.
- `huggingface-pp-cli local` — List models already on disk + map to HF ids. Walks ~/.cache/huggingface/hub/ + custom dirs.
- `huggingface-pp-cli doctor` — Single-call structured runtime probe: TTY, JSON support, state writability, live config, harness, backend-matrix...
- `huggingface-pp-cli schema [<command>]` — Per-command JSON output schema with version field. Defends agents against output drift.


## Freshness Contract

This printed CLI owns bounded freshness only for registered store-backed read command paths. In `--data-source auto` mode, those paths check `sync_state` and may run a bounded refresh before reading local data. `--data-source local` never refreshes. `--data-source live` reads the API and does not mutate the local store. Set `HF_NO_AUTO_REFRESH=1` to skip the freshness hook without changing source selection.

Covered paths:

- `huggingface-pp-cli models`
- `huggingface-pp-cli models get`
- `huggingface-pp-cli models list`
- `huggingface-pp-cli models raw`
- `huggingface-pp-cli models tree`

When JSON output uses the generated provenance envelope, freshness metadata appears at `meta.freshness`. Treat it as current-cache freshness for the covered command path, not a guarantee of complete historical backfill or API-specific enrichment.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
huggingface-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Should I switch?

```bash
huggingface-pp-cli vs-current Qwen/Qwen3.6-35B-A3B --agent main --json --select arch_delta,size_delta,license_delta,verdict
```

Diffs candidate against the running primary model and surfaces only the delta fields.

### Find an MoE with MTP that runs on llama.cpp

```bash
huggingface-pp-cli find-feature mtp --size 20b-40b --backend llama.cpp --moe --json
```

Architecture-feature search HF doesn't expose; returns only entries with backend-readiness verdict YES.

### Pick a quant for 25GB hardware budget

```bash
huggingface-pp-cli find-quants Qwen/Qwen3.6-35B-A3B --prefer iq4_nl,q4_k_m --max-size 25g --uploaders unsloth,bartowski,mradermacher --json --select id,quant,size_gb,uploader,uploader_rep
```

Sorted by uploader rep + size; --select trims output to the agent-relevant fields.

### Has this been benched?

```bash
huggingface-pp-cli bench-history Qwen/Qwen3.6-35B-A3B --json
```

Joins HF id with local harness results — exits 6 cleanly if no harness data.

## Auth Setup

Anonymous read works for every public endpoint. Set HF_TOKEN (or use the cached token at ~/.cache/huggingface/token) for higher rate limits.

Run `huggingface-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  huggingface-pp-cli models list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

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
huggingface-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
huggingface-pp-cli feedback --stdin < notes.txt
huggingface-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.huggingface-pp-cli/feedback.jsonl`. They are never POSTed unless `HUGGINGFACE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `HUGGINGFACE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
huggingface-pp-cli profile save briefing --json
huggingface-pp-cli --profile briefing models list
huggingface-pp-cli profile list --json
huggingface-pp-cli profile show briefing
huggingface-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `huggingface-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/ml-ops/huggingface/cmd/huggingface-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add huggingface-pp-mcp -- huggingface-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which huggingface-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   huggingface-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `huggingface-pp-cli <command> --help`.
