---
name: pp-ts
description: "Every TreasurySpring read endpoint Trigger phrases: `treasuryspring holdings`, `what's maturing`, `obligor concentration`, `available yields`, `cash flow forecast`, `use ts`, `run treasuryspring cli`."
author: "Dickie"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - ts-pp-cli
    install:
      - kind: go
        bins: [ts-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/payments/ts/cmd/ts-pp-cli
---

# TreasurySpring — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `ts-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install ts --cli-only
   ```
2. Verify: `ts-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/ts/cmd/ts-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

ts mirrors your entities, indications, holdings, subscriptions and obligor exposures into local SQLite, then joins across them offline. Run `ts ladder` for a settlement-adjusted cash-flow forecast, `ts concentration --by obligor` for group-wide credit risk, or `ts book` for the consolidated portfolio across every entity. Agent-native: --json, --select, typed exit codes.

## When to Use This CLI

Use ts to inspect a TreasurySpring treasury book from the command line or an agent: list available products and yields, monitor live holdings, project cash flows by maturity, measure obligor concentration, and track lifecycle events. Best when you need portfolio-level answers that span entities or resources, or scriptable/offline access the portal and per-call API do not give.

## Anti-triggers

Do not use this CLI for:
- Do not use ts to place real subscriptions or change maturity actions unless the write surface was explicitly enabled and you intend to commit capital.
- Do not use ts for non-TreasurySpring treasury data; it only mirrors the TreasurySpring Public API.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local mirror that compounds
- **`ladder`** — Week-by-week projection of cash landing back, with maturities shifted to real settlement dates across all entities.

  _Reach for this to answer 'when does liquidity arrive' instead of reading raw per-holding maturity dates._

  ```bash
  ts ladder --by week --currency USD --json
  ```
- **`concentration`** — Total credit exposure to each obligor as a share of the consolidated book, flagging self-set limit breaches.

  _Reach for this for group-wide counterparty risk; per-entity obligor-exposure calls cannot see the consolidated position._

  ```bash
  ts concentration --by obligor --limit 10% --json
  ```
- **`book`** — One portfolio across every legal entity: total invested, weighted-average yield/maturity, currency split.

  _Reach for this for the group-treasurer view no single login or call returns._

  ```bash
  ts book --group-by currency,maturity-bucket --json
  ```

## Command Reference

**cell** — Get information about Cells

- `ts-pp-cli cell <code>` — Retrieves data for a single Cell.

**entity** — Manage entity

- `ts-pp-cli entity get` — Retrieves data for a single Entity if the user has permission to view it.
- `ts-pp-cli entity get-entities` — Retrieves a list of all entities that the user has permission to view.

**event** — Stream of normalised events for integration and reconciliation

- `ts-pp-cli event checkpoint` — Delete a named event checkpoint.
- `ts-pp-cli event checkpoint-checkpoint` — Return a single event checkpoint by name.
- `ts-pp-cli event checkpoint-checkpoint-2` — Advance the checkpoint to a new cursor position. Supply the `new_cursor` to move to.
- `ts-pp-cli event checkpoint-checkpoint-3` — Create a new named event checkpoint, or return the existing one if it already exists.
- `ts-pp-cli event checkpoints` — Return all event checkpoints for the authenticated user, ordered by name.
- `ts-pp-cli event get` — Return a page of events from the stream. Supports cursor-based pagination with an optional timestamp lower bound.

**health** — Manage health

- `ts-pp-cli health` — Perform a health check by returning a JSON response with a status code of 200 (OK).

**holding** — Get information about holdings. For how subscriptions become holdings and how holdings move through their lifecycle, see the FTF Lifecycle section.

- `ts-pp-cli holding get` — Retrieves a list of all holdings that the user has permission to view.
- `ts-pp-cli holding get-entitycode` — Retrieves data for a single holding if the user has permission to view it.

**holidays** — Manage holidays

- `ts-pp-cli holidays` — Retrieves a list of all holidays for a given year.

**indication** — Get information about Indications

- `ts-pp-cli indication <code>` — Retrieves a list of all Indications that the user has permission to view.

**oauth** — OAuth 2.0 endpoint to exchange your Client Credentials for a token. This token can then be used to access the API.

- `ts-pp-cli oauth` — Obtain an access token using the client credentials grant type.

**obligor-exposure** — Get information about Obligors

- `ts-pp-cli obligor-exposure <code>` — Get data for obligor exposure by code

**subscribe** — Manage subscribe

- `ts-pp-cli subscribe` — Subscribe to an FTF

**subscription** — FTF Subscriptions

- `ts-pp-cli subscription` — Retrieves a list of all subscriptions that the user has permission to view.

**task** — Get information about Pending Tasks

- `ts-pp-cli task get` — Retrieves a list of all pending tasks that the user has.
- `ts-pp-cli task get-uid` — Retrieves a pending task by uid.
- `ts-pp-cli task post` — Used to approve or deny a pending task.

**webhook** — Integrate with webhooks to receive notifications

- `ts-pp-cli webhook delete` — Deregister an existing webhook for a user
- `ts-pp-cli webhook post` — Register a url to a user for webhook notifications


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
ts-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Group-wide credit concentration

```bash
ts concentration --by obligor --limit 10% --json --select obligor,exposure,share
```

Consolidated obligor exposure with limit-breach flags; --select narrows the deeply nested rollup.

### Cash-flow forecast by week

```bash
ts ladder --by week --currency USD --json
```

Settlement-adjusted maturity ladder across all entities, bucketed by week.

### Consolidated group book

```bash
ts book --group-by currency --json
```

Total invested and weighted-average yield/maturity across every legal entity.

## Auth Setup

TreasurySpring uses OAuth2 client-credentials. Set TS_CLIENT_ID and TS_CLIENT_SECRET, then `ts auth login` exchanges them at /oauth/token for a bearer token cached locally. Alternatively set TS_BEARER_TOKEN with a pre-minted token. Use the sandbox with --base-url or TS_ENV=sandbox.

Run `ts-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  ts-pp-cli cell mock-value --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
ts-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
ts-pp-cli feedback --stdin < notes.txt
ts-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/ts-pp-cli/feedback.jsonl`. They are never POSTed unless `TS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `TS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
ts-pp-cli profile save briefing --json
ts-pp-cli --profile briefing cell mock-value
ts-pp-cli profile list --json
ts-pp-cli profile show briefing
ts-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `ts-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/payments/ts/cmd/ts-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add ts-pp-mcp -- ts-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which ts-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   ts-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `ts-pp-cli <command> --help`.
