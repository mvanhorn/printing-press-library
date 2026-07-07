---
name: pp-uk-train-goat
description: "The only Go-native, agent-native, MCP-exposed UK rail CLI; live boards, journey planning Trigger phrases: `next train from <station>`, `departures from <station>`, `trains london to <destination>`, `is the X.YY to <station> on time`, `which platform is the <service> on`, `use uk-train-goat`, `run uk-train-goat-pp-cli`."
author: "Sachin Ahuja"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - uk-train-goat-pp-cli
---

# UK Train Goat — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `uk-train-goat-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install uk-train-goat --cli-only
   ```
2. Verify: `uk-train-goat-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

uk-train-goat wraps the free National Rail OpenLDBWS API with a Cobra command tree, an MCP server, and an offline station database. Live departures, arrivals, journey planning A->B, and service status all run from one terminal command and ship with a programmatic eval grader that pins tool descriptions for LLM agents.

## When to Use This CLI

Use uk-train-goat for any UK National Rail question an agent or terminal user has: live departures, live arrivals, journey planning between two stations on today or a future date, and service-status / delay-reason lookups. The local SQLite store gives every command offline CRS resolution and supports saved-commute features (`go`, `saved status`, `recent`) that a thin API wrapper cannot. Do not use uk-train-goat for booking, fare comparisons against Trainline, EU/Eurostar trains, or real-time push notifications; those are explicit non-goals for v0.1.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds

- **`go`** — Run a saved commute in one keystroke; resolves origin, destination, and time-window from the local store.

  _Pick this when an agent needs the user's named recurring route resolved from local state, not a fresh A->B planning call._

  ```bash
  uk-train-goat-pp-cli go morning --json --select services.std,services.platform,services.etd
  ```
- **`saved status`** — Fans out across every saved route and prints a single ranked status table for all your commutes.

  _Pick this on disruption mornings when a user has multiple saved commutes and wants one answer, not N separate board calls._

  ```bash
  uk-train-goat-pp-cli saved status --json --select routes.name,routes.next.std,routes.next.etd
  ```
- **`recent`** — Re-runs your recent A->B searches with fresh live data, side-by-side, so iterative trip planning is one keystroke per refresh.

  _Pick this when the user is iterating dates or origins for one trip; every previous query stays one keystroke away._

  ```bash
  uk-train-goat-pp-cli recent --json --select queries.from,queries.to,queries.next.std
  ```

### Agent-native plumbing

- **`stations`** — FTS5 search over the local UK CRS code table; resolves Paddington -> PAD with no network call.

  _Pick this whenever the user names a station in prose; resolve to CRS once, then call any other uk-train-goat command._

  ```bash
  uk-train-goat-pp-cli stations --search kings --json --select results.crs,results.name
  ```
- **`board`** — Accept a comma-separated list of CRS codes and merge departures across all of them into one ranked time-ordered list.

  _Pick this when an agent needs the next London-bound (or other regional cluster) departure across multiple terminals at once._

  ```bash
  uk-train-goat-pp-cli board PAD,KGX,EUS --in 30m --json --select services.origin,services.std,services.destination
  ```
- **`eval`** — Programmatic eval grader that scores an LLM agent against a fixture suite of natural-language UK-rail prompts; 80% pass-rate threshold blocks ship.

  _Run this in CI on changes to internal/cli or internal/mcp; failing evals mean an agent will pick the wrong tool or wrong args._

  ```bash
  EVAL_AGENT_MODEL=claude-sonnet-4-6 uk-train-goat-pp-cli eval --json
  ```

### Service-specific content

- **`why`** — Combines GetServiceDetails delay/cancel reasons with adjacent operator alerts into one screen explaining what is going wrong with one service.

  _Pick this when the user wants to understand a delay, not just see it._

  ```bash
  uk-train-goat-pp-cli why L8rW0bMonHt3K4IengVPQw== --json
  ```
- **`journey`** — Ranks A->B options by combined scheduled-time, current delay, and platform-known signal so on-time-but-later beats earlier-but-late.

  _Pick this for trip planning during disruption; choosing the earliest scheduled departure is often wrong when delays exist._

  ```bash
  uk-train-goat-pp-cli journey RDG PAD --rank --json --select journeys.std,journeys.delay,journeys.platform
  ```

## Command Reference

**status** — Internal placeholder resource. Real commands are hand-authored against the OpenLDBWS wrapper.

- `uk-train-goat-pp-cli status` — Placeholder; deleted post-generate. Real CLI commands live in internal/cli/board.go etc.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
uk-train-goat-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Daily morning commute check

```bash
uk-train-goat-pp-cli go morning --json
```

Resolves the saved commute named 'morning' and returns the next departures filtered to your destination.

### Compact multi-station agent query

```bash
uk-train-goat-pp-cli board PAD,KGX,EUS --in 30m --agent --select services.origin,services.std,services.destination,services.etd
```

Multi-origin fan-out + dotted-path field selection; tight payload for an LLM context window. --in / --within accept human-readable durations (30m, 1h).

### Why is my train late

```bash
uk-train-goat-pp-cli why $SERVICE_ID --json
```

Surfaces one service's scheduled vs expected times, platform, operator, and calling points, with a plain-prose status line (on time, running late, or cancelled). The delay-reason text and NRCC operator alerts (including strike notices) live on the live board, not the service-detail payload. See `board`/`arrivals`, where `messages[]` carries the alert banners and `delay_reason` carries the cause.

### Iterative trip planning

```bash
uk-train-goat-pp-cli recent --json
```

Re-runs your recent journey queries with fresh live data; one keystroke per refresh.

### CI eval gate

```bash
uk-train-goat-pp-cli eval --json
```

Runs the agent eval suite (set EVAL_AGENT_MODEL=claude-sonnet-4-6 in your environment first; exits non-zero if pass rate drops below 80%).

## Auth Setup
Run `uk-train-goat-pp-cli auth setup` to print the URL and steps for getting a key (add `--launch` to open the URL). Then set:

```bash
export LDBWS_API_TOKEN="<your-key>"
```

Or persist it in `~/.config/uk-train-goat-pp-cli/config.toml`.

Run `uk-train-goat-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  uk-train-goat-pp-cli status --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
uk-train-goat-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
uk-train-goat-pp-cli feedback --stdin < notes.txt
uk-train-goat-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/uk-train-goat-pp-cli/feedback.jsonl`. They are never POSTed unless `UK_TRAIN_GOAT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `UK_TRAIN_GOAT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
uk-train-goat-pp-cli profile save briefing --json
uk-train-goat-pp-cli --profile briefing status
uk-train-goat-pp-cli profile list --json
uk-train-goat-pp-cli profile show briefing
uk-train-goat-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `uk-train-goat-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add uk-train-goat-pp-mcp -- uk-train-goat-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which uk-train-goat-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   uk-train-goat-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `uk-train-goat-pp-cli <command> --help`.
