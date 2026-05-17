---
name: pp-higgsfield
description: "Every Higgsfield workflow with a local memory: batch model fanouts, Soul library search, and credit guards no other... Trigger phrases: `generate a video with higgsfield`, `fanout this prompt across models`, `find my higgsfield soul id`, `search my higgsfield prompts`, `how much have I spent on higgsfield this week`, `use higgsfield`, `run higgsfield`."
author: "Higgsfield AI"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - higgsfield-pp-cli
---

# Higgsfield — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `higgsfield-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install higgsfield --cli-only
   ```
2. Verify: `higgsfield-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/higgsfield/cmd/higgsfield-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

This CLI mirrors the official Higgsfield CLI surface (auth, generate, soul-id, marketing-studio, product-photoshoot, marketplace-cards, upload, account, workspace) and adds the local SQLite store that powers fanout, soul-ids search, credit guards, and prompt history search. Single binary, agent-native JSON-first, MCP-exposed via the runtime Cobra-tree mirror.

## When to Use This CLI

Use higgsfield-pp-cli whenever you need to drive Higgsfield from an agent or want local memory across runs. Pick the official `higgsfield` CLI when you only need a single human one-shot generation. Pick higgsfield-pp-cli when you want batch fanouts, Soul ID search, credit guards, persistent generation history, or MCP exposure. The CLI mirrors every command the official one offers, so switching is loss-free.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Batch orchestration

- **`fanout`** — Submit one prompt to N models in parallel and track them as a single fanout group.

  _Reach for this whenever you need to compare the same prompt across models; one command replaces N submits and N polls._

  ```bash
  higgsfield-pp-cli fanout --prompt "cinematic dawn over Manhattan, slow camera push" --models veo3_1,seedance_2_0,kling3_0 --max-cost 200 --json
  ```
- **`fanout wait`** — Poll every job in a fanout, then render a side-by-side table of model, cost, duration, and result URL.

  _Decide which model to ship by reading one table, not N tabs._

  ```bash
  higgsfield-pp-cli fanout wait fan_20260516_001 --json --select model,cost,result_url
  ```

### Local state that compounds

- **`soul-ids search`** — Full-text search across Soul ID names plus the prompts each Soul ID has been used in, ranked by last-used.

  _Stop scrolling Soul IDs to find the one you trained two months ago; ask by name or by the prompt that used it._

  ```bash
  higgsfield-pp-cli soul-ids search "riggs" --json
  ```
- **`fanout --max-cost`** — Pre-flight cost estimate across every fanout model; refuses to submit if total exceeds the cap and prints the per-model breakdown.

  _Run before you spend; the operator (or agent) sees the credit hit before the run lands._

  ```bash
  higgsfield-pp-cli fanout --prompt "..." --models veo3_1,veo3_1_lite --max-cost 80
  ```
- **`search`** — FTS5 search across every prompt you've ever submitted, joined with model, Soul ID name, and transaction memo.

  _Recall the exact prompt that produced last week's hero shot without re-typing or guessing._

  ```bash
  higgsfield-pp-cli search "cinematic riggs dawn" --json --select prompt,model,result_url,created_at
  ```
- **`soul-ids usage`** — Every generation that used a given Soul ID, ordered by date, with thumbnail URLs and total cost.

  _Audit a Soul ID's history before retiring it, or pull every cut-in for an episode summary._

  ```bash
  higgsfield-pp-cli soul-ids usage soul_riggs_42 --since 30d --json
  ```
- **`account spend`** — Local SQL aggregation over synced credit transactions, grouped by model, day, or Soul ID.

  _Weekly credit review without manual pagination; forecasts next month's spend by trend._

  ```bash
  higgsfield-pp-cli account spend --since 7d --group-by model --json
  ```

### Agent-native plumbing

- **`generate export`** — Export the local generations table joined with models and Soul IDs to CSV or JSONL for spreadsheet review or downstream pipelines.

  _Hand the week's output to any tool that reads CSV/JSONL; no manual scraping required._

  ```bash
  higgsfield-pp-cli export models --format jsonl --output models.jsonl
  ```

## Command Reference

**account** — Account status, credit balance, and credit transactions

- `higgsfield-pp-cli account status` — Show signed-in account email, plan, and available credits
- `higgsfield-pp-cli account transactions` — List recent credit transactions (debits for generations, credits for top-ups/refunds)

**generations** — AI generation jobs — image, video, text-to-image, image-to-video, Soul-driven, Marketing Studio

- `higgsfield-pp-cli generations cancel` — Cancel a pending generation
- `higgsfield-pp-cli generations get` — Get one generation by ID (web backend)
- `higgsfield-pp-cli generations list` — List recent generation jobs from the web backend (cloud.higgsfield.ai). Returns the most-recent 100 by default.

**models** — Higgsfield model catalog — 35 image, video, and text models across both backends

- `higgsfield-pp-cli models get` — Get one model's full parameter schema
- `higgsfield-pp-cli models list` — List all available models with type, display name, and backend assignment

**soul_ids** — Custom Soul references (trained characters and styles) for use in soul-aware models

- `higgsfield-pp-cli soul_ids create` — Train a new Soul ID from at least 5 reference images. Returns the new Soul ID's training job — poll with `soul-id...
- `higgsfield-pp-cli soul_ids get` — Get one Soul ID and its training status
- `higgsfield-pp-cli soul_ids list` — List all your Soul IDs (custom-trained character references)

**uploads** — Upload media (images, videos, audio) for use in subsequent generations as upload_id references

- `higgsfield-pp-cli uploads` — Initialize a media upload — returns a media_id for the second step

**workspaces** — Billing workspaces — switch between personal and shared workspace contexts

- `higgsfield-pp-cli workspaces list` — List workspaces available to your account
- `higgsfield-pp-cli workspaces status` — Show the currently selected workspace


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
higgsfield-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Compare a prompt across two video models with a budget cap

```bash
higgsfield-pp-cli fanout --prompt "cinematic dawn over Manhattan, slow camera push" --models veo3_1,seedance_2_0 --max-cost 80 --json
```

Submits the prompt to Veo 3.1 and Seedance 2.0 in parallel, refuses if estimated cost exceeds 80 credits, and returns a single fanout_id linking both jobs.

### Find every generation that used a Soul ID this month

```bash
higgsfield-pp-cli soul-ids usage soul_riggs_42 --since 30d --json --select prompt,model,result_url,cost
```

Local SQL join over generations × soul_ids; reads from the synced store with no API call.

### Search past prompts and narrow the output for an agent

```bash
higgsfield-pp-cli search "cinematic riggs dawn" --json --select prompt,model,result_url,created_at
```

FTS5 over your full prompt history; --select keeps the response compact for downstream tools.

### Weekly credit review by model

```bash
higgsfield-pp-cli account spend --since 7d --group-by model --json
```

Aggregates synced transactions by model; pipe to jq for trend analysis or `--format csv` for spreadsheet review.

### Export the week's generations for review

```bash
higgsfield-pp-cli export models --format jsonl --output models.jsonl
```

Streams generations joined with models and Soul IDs as JSONL; one line per record, agent-friendly.

## Auth Setup

Two auth paths. Web backend (default, recommended for most 2026 models): set HIGGSFIELD_JWT from your logged-in Higgsfield CLI's credentials file and HIGGSFIELD_ENABLE_WEB_BACKEND=1. Platform API (stable, ~8 models): set HIGGSFIELD_API_KEY and HIGGSFIELD_SECRET (or HF_CREDENTIALS=KEY:SECRET). The CLI picks the right backend per model; commands that need the unselected backend will print an actionable error.

Run `higgsfield-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  higgsfield-pp-cli generations list --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
higgsfield-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
higgsfield-pp-cli feedback --stdin < notes.txt
higgsfield-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.higgsfield-pp-cli/feedback.jsonl`. They are never POSTed unless `HIGGSFIELD_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `HIGGSFIELD_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
higgsfield-pp-cli profile save briefing --json
higgsfield-pp-cli --profile briefing generations list
higgsfield-pp-cli profile list --json
higgsfield-pp-cli profile show briefing
higgsfield-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `higgsfield-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add higgsfield-pp-mcp -- higgsfield-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which higgsfield-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   higgsfield-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `higgsfield-pp-cli <command> --help`.
