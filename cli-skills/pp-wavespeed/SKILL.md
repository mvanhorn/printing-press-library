---
name: pp-wavespeed
description: "Printing Press CLI for Wavespeed. Docs-derived OpenAPI spec for WaveSpeed AI's REST API."
author: "Cathryn Lavery"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - wavespeed-pp-cli
    install:
      - kind: go
        bins: [wavespeed-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/ai/wavespeed/cmd/wavespeed-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/ai/wavespeed/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Wavespeed — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `wavespeed-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install wavespeed --cli-only
   ```
2. Verify: `wavespeed-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/wavespeed/cmd/wavespeed-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Docs-derived OpenAPI spec for WaveSpeed AI's REST API. WaveSpeed is a
unified AI generation platform for image, video, audio, 3D, and LLM models.

The dynamic model-run endpoint is not represented as a generated OpenAPI
path because WaveSpeed model IDs are slash-delimited API paths such as
`wavespeed-ai/hunyuan-video/t2v`; ordinary OpenAPI path-parameter clients
correctly percent-encode slashes. The printed CLI adds a hand-authored
`run` command that submits to the literal model path.

## Command Reference

**account_balance** — Manage account balance

- `wavespeed-pp-cli account-balance` — Retrieve the authenticated account balance.

**billings** — Billing and usage records

- `wavespeed-pp-cli billings` — Search billing records for the authenticated account.

**media_uploads** — Manage media uploads

- `wavespeed-pp-cli media-uploads` — Upload a binary file to WaveSpeed media storage.

**model_pricing** — Manage model pricing

- `wavespeed-pp-cli model-pricing` — Estimate the unit price for a model run using the same inputs that will be submitted to the model endpoint.

**models** — Model catalog and model metadata

- `wavespeed-pp-cli models` — List available WaveSpeed models and their API schemas.

**run** — Submit generation tasks to slash-delimited WaveSpeed model paths.

- `wavespeed-pp-cli run <model-id> --input '<json>'` — Submit a model run with JSON inputs.
- `wavespeed-pp-cli run <model-id> --input-file request.json --price --wait --download` — Price, submit, poll, and download output URLs.

**prediction_deletions** — Manage prediction deletions

- `wavespeed-pp-cli prediction-deletions` — Delete one or more predictions from history.

**prediction_results** — Manage prediction results

- `wavespeed-pp-cli prediction-results <task_id>` — Retrieve the latest status and result payload for a prediction task.

**predictions** — Prediction submission history and result retrieval

- `wavespeed-pp-cli predictions` — Query recent prediction history. The API history window is limited; sync accumulates across runs.

**usage_stats** — Manage usage stats

- `wavespeed-pp-cli usage-stats` — Retrieve usage statistics for the authenticated account.

## Novel commands / Workflows (D2C content production)

WaveSpeed ships a content-production layer for D2C brands posting to social
(Instagram, TikTok, Facebook, X). It is **produce-only**: it emits post-ready
files plus a per-platform `manifest.json` a downstream social-posting tool
consumes. Every novel command emits a uniform agent envelope
(`command`, `results`, `suggested_next`, `recommended_action`,
`cost_spent`, `library_record_errors`, `partial_failure`) and supports
`--dry-run` to preview planned files, costs, and merged params without spending.

Novel commands record each generation to a local library DB by default; pass
`--no-record` to opt out. Plain `run` does the inverse — pass `--record` to
record it. A library write failure is logged and surfaced in
`library_record_errors`; it never fails a successful generation.

**Plan**

- `wavespeed-pp-cli plan brief-to-shotlist --prompt "<brief>" --platforms instagram,tiktok` — turn a brief into a structured shotlist. Hybrid planner: `--planner deterministic|llm|auto` (default `auto`), with an LLM fallback via `--planner-model <id>`. `--aspects` and `--from-file` are also accepted.
- `wavespeed-pp-cli plan model-pick <intent>` — recommend a model for an intent from the live catalog.
- `wavespeed-pp-cli plan cost-estimate <shotlist.json>` — price a shotlist against `/model/pricing` and your balance.

**Produce**

- `wavespeed-pp-cli pack --concept "<concept>" --platforms instagram,tiktok` — multi-platform pack at stable `packs/<slug>/<platform>/` paths with a per-platform manifest. Flags: `--aspects`, `--concurrency`, `--max-cost`, `--on-failure abort|continue`, `--history`, `--clean`, `--strict-video`, `--model`, `--brand`, `--seed`, `--out-dir`.
- `wavespeed-pp-cli batch --from shots.csv --max-cost 5.00` — submit many prompts from CSV/JSON. `--fail-tolerant` (default fail-fast), `--concurrency`, `--model`, `--brand`.
- `wavespeed-pp-cli variants --base shotlist.json --vary seed --count 4` — controlled sweep (`--vary seed|style|model`, `--values`).
- `wavespeed-pp-cli compose --steps "text->image,image->video" --prompt "..." --models m1,m2` — explicit step pipeline; a failed step rolls back later steps.

**Refine**

- `wavespeed-pp-cli aspects <image> --platforms instagram,tiktok` — re-frame one image to target ratios (`--aspects`, `--outpaint`, `--prompt`, `--model`).
- `wavespeed-pp-cli restyle <image> --brand helm` — apply a brand/style (`--style`, `--model`).

**Library**

- `wavespeed-pp-cli library list --brand helm --platform instagram --since 30d` — list recorded generations (`--model`, `--tag`, `--limit`).
- `wavespeed-pp-cli library search "<query>"` — FTS5 prompt search (`--limit`).
- `wavespeed-pp-cli library show <id>` — one generation with its tags.
- `wavespeed-pp-cli library tag <id> --add hero --remove draft` — tag management.
- `wavespeed-pp-cli library export <dir>` — export matching generations as JSON.
- `wavespeed-pp-cli library cost-report --since 30d --group-by brand` — cost rollup (`--group-by brand|model|platform|tag`).

**QA**

- `wavespeed-pp-cli qa preflight <shotlist.json>` — pass/warn/fail checks (balance vs cost, model availability, prompt safety, platform request-shape, brand coverage).

**Brand**

- `wavespeed-pp-cli brand init <name> --from-file brand.json` — create a profile (or use field flags: `--style-anchors`, `--negative`, `--palette`, `--voice`, `--models`, `--platforms`). Non-interactive by default.
- `wavespeed-pp-cli brand show <name>`, `brand list`, `brand apply <name>` (sets the active brand in `wavespeed.json`), `brand edit <name>`.

A full agent chain: `brand apply → plan brief-to-shotlist → plan cost-estimate → qa preflight → pack`, piping JSON between steps.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
wavespeed-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Run `wavespeed-pp-cli auth setup` to print the URL and steps for getting a key (add `--launch` to open the URL). Then set:

```bash
export WAVESPEED_API_KEY="<your-key>"
```

Or persist it in `~/.config/wavespeed-pp-cli/config.toml`.

Run `wavespeed-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  wavespeed-pp-cli billings --agent --select id,name,status
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
wavespeed-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
wavespeed-pp-cli feedback --stdin < notes.txt
wavespeed-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/wavespeed-pp-cli/feedback.jsonl`. They are never POSTed unless `WAVESPEED_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `WAVESPEED_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what _surprised_ you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink            | Effect                                                                                          |
| --------------- | ----------------------------------------------------------------------------------------------- |
| `stdout`        | Default; write to stdout only                                                                   |
| `file:<path>`   | Atomically write output to `<path>` (tmp + rename)                                              |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
wavespeed-pp-cli profile save briefing --json
wavespeed-pp-cli --profile briefing billings
wavespeed-pp-cli profile list --json
wavespeed-pp-cli profile show briefing
wavespeed-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning                       |
| ---- | ----------------------------- |
| 0    | Success                       |
| 2    | Usage error (wrong arguments) |
| 3    | Resource not found            |
| 4    | Authentication required       |
| 5    | API error (upstream issue)    |
| 7    | Rate limited (wait and retry) |
| 10   | Config error                  |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `wavespeed-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/ai/wavespeed/cmd/wavespeed-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add wavespeed-pp-mcp -- wavespeed-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which wavespeed-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   wavespeed-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `wavespeed-pp-cli <command> --help`.
