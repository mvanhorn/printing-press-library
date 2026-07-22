---
name: pp-finra
description: "Every FINRA Query API dataset, plus local trend detection and batch registration checks no other FINRA tool offers. Trigger phrases: `check Reg SHO short volume`, `look up CRD registration status`, `FINRA complaint filings`, `TRACE bond data`, `FINRA threshold list`, `check broker registration`, `use finra`, `run finra`."
author: "Michael Schreiber"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - finra-pp-cli
    install:
      - kind: go
        bins: [finra-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/finra/cmd/finra-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/other/finra/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# FINRA — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `finra-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install finra --cli-only
   ```
2. Verify: `finra-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/finra/cmd/finra-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

FINRA CLI treats the entire ~35-dataset Query API catalog as a live-discoverable surface instead of a hand-curated list, then layers offline SQLite history on top so Reg SHO escalation streaks, TRACE liquidity trends, and 4530 complaint surges become one-command answers instead of manual CSV diffing.

## When to Use This CLI

Use this CLI for FINRA Query API data pulls (equity, fixed income, registration/licensing), compliance monitoring rituals (Reg SHO threshold tracking, 4530 complaint surges), and batch CRD validation before filing submissions.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to submit U4/U5/BR registration filings — Submission API payload schemas are not publicly documented and are not implemented.
- Do not use this CLI for FileX bulk file transfers — that is SFTP/HTTPS/S3 infrastructure outside this CLI's scope.
- Do not use this CLI for real-time push notifications — FINRA's Notification API is poll-based, not a webhook system.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`regsho threshold-watch`** — See which symbols just crossed the Reg SHO 5-day threshold escalation point before it triggers a mandatory close-out.

  _Pick this over a raw threshold-list pull when you need to know how many consecutive days a symbol has been flagged, not just whether it's flagged today._

  ```bash
  finra-pp-cli regsho threshold-watch --symbol GME --json
  ```
- **`complaints new`** — See 4530 customer complaint filings for a firm within a recent time window, without re-reading the full history.

  _Use this for a quick recent-activity check; use complaints list for the full history or a custom date range._

  ```bash
  finra-pp-cli complaints new --firm 19847 --since 7d --json
  ```
- **`fixedincome health`** — One snapshot report joining TRACE, Corporate/Agency Debt Market Breadth, and Corporate/Agency Debt Market Sentiment for a time window.

  _Use this for a single fixed-income market-condition snapshot instead of four separate dataset pulls stitched together by hand._

  ```bash
  finra-pp-cli fixedincome health --since 7d --json
  ```
- **`trace liquidity`** — Month-over-month trend in TRACE Monthly Volume trade count and volume for a product category (or market-wide if no category given).

  _Use this for a month-over-month volume/trade-count trend for a product category; use 'trace search' for raw monthly aggregate records instead._

  ```bash
  finra-pp-cli trace liquidity --sub-product CORP --since 180d --json
  ```
- **`registration timeline`** — Full chronological registration-status history for one person, joining Composite Individual, Firm Registration Status History, and Individual Delta records.

  _Use this to see how a rep's registration status changed over time; use 'registration individual --crd' for just the current snapshot._

  ```bash
  finra-pp-cli registration timeline --crd 1234567 --json
  ```

### Agent-native plumbing
- **`registration validate-batch`** — Validate many CRDs from a file in one call instead of checking them one at a time.

  _Use this before submitting a batch of registration filings to confirm every CRD is currently valid._

  ```bash
  finra-pp-cli registration validate-batch --file crds.csv --json
  ```

## Command Reference

**async** — Poll async job status and results for bulk data extracts

- `finra-pp-cli async <group> <name> <requestId>` — Poll the status and results of an async data request

**catalog** — Discover available FINRA datasets and their capabilities

- `finra-pp-cli catalog` — List all available datasets, optionally filtered by group or name

**data** — Query FINRA dataset records (equity, fixed income, registration, firm, and content datasets)

- `finra-pp-cli data get` — Get a single dataset record by its primary ID (only for datasets where supportsGetById is true — check via 'catalog')
- `finra-pp-cli data list` — List/filter dataset records via query parameters
- `finra-pp-cli data query` — Filter dataset records via a JSON request body (richer filtering than 'data list')

**metadata** — Inspect field-level schema and partition fields for a dataset

- `finra-pp-cli metadata <group> <name>` — Get field list, types, and partition fields for a dataset

**partitions** — List available partition values (typically dates) for incremental sync

- `finra-pp-cli partitions <group> <name>` — List available partition values for a dataset, used for incremental sync


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
finra-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Track a symbol's Reg SHO escalation

```bash
finra-pp-cli regsho threshold-watch --symbol GME --json --select symbol,consecutive_days,escalated
```

Narrow the streak-tracker output to just the fields that matter for a compliance check.

### Fixed-income market snapshot

```bash
finra-pp-cli fixedincome health --since 7d --json
```

One command replaces four separate dataset pulls (TRACE, breadth, sentiment) stitched together by hand.

### Recent complaint filings for a firm

```bash
finra-pp-cli complaints new --firm 19847 --since 7d --json
```

Filters 4530 filings to a recent window instead of scanning the full history.

### Bulk-validate CRDs before a filing batch

```bash
finra-pp-cli registration validate-batch --file crds.csv --json
```

Turns N one-at-a-time CRD lookups into a single batch call.

### TRACE monthly volume trend for a product category

```bash
finra-pp-cli trace liquidity --sub-product CORP --since 180d --json --select sub_product,liquidity_trend,avg_trades_per_month
```

Pairs --json with --select to keep agent output small for a deeply nested trend response.

## Auth Setup

FINRA uses OAuth2 client_credentials: your API Client ID and Client Secret (from the FINRA API Console) are exchanged for a short-lived Bearer token. Set FINRA_CLIENT_ID and FINRA_CLIENT_SECRET, then run 'finra-pp-cli auth login' to fetch and cache a token — it refreshes automatically before expiry.

Run `finra-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  finra-pp-cli async OTCMARKET REGSHODAILY req-12345 --agent --select id,name,status
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

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `FINRA_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `FINRA_CONFIG_DIR`, `FINRA_DATA_DIR`, `FINRA_STATE_DIR`, `FINRA_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `FINRA_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `finra-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "finra": {
        "command": "finra-pp-mcp",
        "env": {
          "FINRA_HOME": "/srv/finra"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `FINRA_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `FINRA_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
finra-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
finra-pp-cli feedback --stdin < notes.txt
finra-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `FINRA_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `FINRA_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
finra-pp-cli profile save briefing --json
finra-pp-cli --profile briefing async OTCMARKET REGSHODAILY req-12345
finra-pp-cli profile list --json
finra-pp-cli profile show briefing
finra-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `finra-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/finra/cmd/finra-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add finra-pp-mcp -- finra-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which finra-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   finra-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `finra-pp-cli <command> --help`.
