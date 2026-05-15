---
name: pp-operon
description: "Every Operon API endpoint plus 10 compound queries the API does not expose: similar-advertiser lookup, click-chain verification, spec drift detection, a local SQLite mirror with sync, demand freshness and health, placement replay and watch, auction explain, trust-score history, and campaign group-by-wallet. Trigger phrases: `test operon placement`, `list operon demand`, `operon doctor`, `operon spec verify`, `operon click follow`, `operon demand similar`, `operon sync`, `operon demand stale`, `operon demand health`, `operon placement replay`, `operon placement watch`, `operon auction explain`, `operon campaign trust-history`, `operon campaign group-by-wallet`, `use operon-pp-cli`, `run operon-pp-cli`."
author: "yaooooooooooooooo"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - operon-pp-cli
---

# Operon — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `operon-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install operon --cli-only
   ```
2. Verify: `operon-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

This CLI (binary name: `operon-pp-cli`) is the agent-native CLI and MCP server for Operon, the ad network for AI agents. It wraps the full placement and demand surface with agent-native UX (typed exit codes, JSON/select/compact output modes, dry-run on every mutation), adds three live-composition queries the API does not expose (demand similar, click follow, spec verify), and ships a local SQLite store + `sync` command that unlocks seven more transcendence queries: demand stale, demand health, placement replay, placement watch, auction explain, campaign trust-history, and campaign group-by-wallet.

## When to Use This CLI

Reach for operon-pp-cli when you need to debug an Operon integration outside of @operon/sdk's Node-only surface, when you need cross-advertiser composition queries the API does not expose (similar-advertiser lookup, click chain walking, spec drift detection, freshness, replay, auction explain, trust history, wallet rollups), or when you want to expose Operon as MCP tools to a non-Node agent runtime via the bundled operon-pp-mcp binary.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Live composition
- **`demand similar`** — Find advertisers with overlapping category, assets, and serviceType to a given advertiser.

  _Pre-bid competitive analysis: 'who else is competing for this category?'_

  ```bash
  operon-pp-cli demand similar adv_changenow --json
  ```
- **`click follow`** — Walk the /c/{impressionId} redirect chain, validate the URL scheme, and confirm the final landing matches the advertiser's clickUrl.

  _End-to-end attribution debugging in one command instead of three curl invocations._

  ```bash
  operon-pp-cli click follow imp_a1b2c3d4e5f60718
  ```
- **`spec verify`** — Re-fetch the published OpenAPI spec, compare schemas against what the live API actually returns, flag drift.

  _Catches contract drift the same way the press scorecard would - before downstream integrators do._

  ```bash
  operon-pp-cli spec verify --json
  ```

### Local store
- **`sync`** — Fetch /demand and upsert into a local SQLite store so freshness, replay, watch, and trust-history queries work offline.

  _Prerequisite for every offline-readable transcendence command; turns a stateless API into one with a history._

  ```bash
  operon-pp-cli sync --json
  ```
- **`demand stale`** — List demand entries the local mirror hasn't seen refreshed within the last N hours.

  _Spot churned advertisers before placing campaigns against names that are about to disappear from the pool._

  ```bash
  operon-pp-cli demand stale --hours 48
  ```
- **`demand health`** — Composite freshness + coverage + trust report across the locally synced demand index.

  _Single-call health dashboard for the index; tells you immediately whether a category has zero coverage._

  ```bash
  operon-pp-cli demand health --json
  ```
- **`placement replay`** — Re-issue a previously logged placement request to the live API and diff the new winner, scoutScore, decision, and ranking size against the original.

  _Catches ranking and scoutScore drift between two timepoints — useful when an advertiser keeps losing to the same competitor._

  ```bash
  operon-pp-cli placement replay imp_a1b2c3d4e5f60718
  ```
- **`placement watch`** — Poll the local placement log and stream new auction outcomes as a compact line or JSONL stream.

  _Live tail for in-flight auction outcomes so a publisher can see decisions land in real time._

  ```bash
  operon-pp-cli placement watch --duration 30s --json
  ```
- **`auction explain`** — Decode the auction.ranking[] from a logged placement into a sorted human table (rank, service, score, bid, eligible, reason).

  _Turns an opaque JSON blob into a readable explanation of *why* a given advertiser won (or didn't)._

  ```bash
  operon-pp-cli auction explain imp_a1b2c3d4e5f60718
  ```
- **`campaign trust-history`** — Render a sparkline + table of locally observed trust scores for a campaign or advertiser id.

  _Visual trust audit trail — watch a campaign's score drift up or down over weeks._

  ```bash
  operon-pp-cli campaign trust-history adv_changenow
  ```
- **`campaign group-by-wallet`** — Group locally mirrored campaigns by x402 payer wallet with per-wallet count, total balance, and category list.

  _Spot one advertiser running multiple campaigns from the same wallet (common with agencies)._

  ```bash
  operon-pp-cli campaign group-by-wallet --json
  ```

## Command Reference

**c** — Manage c

- `operon-pp-cli c <impressionId>` — Public 302 redirect endpoint. Logs the click against the impression (with idempotent dedup) and forwards to the...

**demand** — Manage demand

- `operon-pp-cli demand` — Returns the public projection of active production-lane advertisers. The response strips operational fields...

**developers** — One-time developer registration for the sandbox-to-production graduation path.

- `operon-pp-cli developers` — One-time developer registration. Lifts the caller's UUID from the 100-call/hour sandbox quota to the 1000-call/hour...

**operon-ad-network-health** — Manage operon ad network health

- `operon-pp-cli operon-ad-network-health` — Returns 200 OK with body `ok` when the server is up. Never returns 4xx; liveness probes that fail return 5xx or...

**placement** — Manage placement

- `operon-pp-cli placement` — Returns a ranked placement (sponsored or blocked) for the current impression. Two auth lanes: - **Production lane**:...

**x402** — Manage x402

- `operon-pp-cli x402 cancel-campaign` — Marks the campaign cancelled and returns unspent USDC to the funding wallet. Bearer token issued at creation. Rate...
- `operon-pp-cli x402 create-campaign` — x402-gated. First call (no `X-PAYMENT` header) returns HTTP 402 with a JSON body containing the payment challenge...
- `operon-pp-cli x402 read-campaign` — Returns current balance, stats, status, and trust score. Bearer token issued at creation. Rate limit: 60/min per IP.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
operon-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Sanity-check a placement request before integrating @operon/sdk

```bash
operon-pp-cli placement --impression-context-request-context-query "swap USDC for SOL" --impression-context-request-context-category DeFi --impression-context-request-context-intent swap --impression-context-request-context-asset USDC --dry-run --json
```

Submit a synthetic impression context via --dry-run. Returns the request payload without sending so you can verify the contract shape before integrating from @operon/sdk.

### Find DeFi advertisers competing for swap intent

```bash
operon-pp-cli demand similar adv_changenow --json --select id,service,serviceType,category
```

Live set-intersection over the demand index. Useful before submitting a new advertiser to see who else is in the same lane.

### Verify the published spec matches live API behavior

```bash
operon-pp-cli spec verify --json
```

Re-fetches operon.so/openapi.json, probes documented GET endpoints with a short timeout, and reports any drift between documented and actual response shapes.

### Walk a click attribution chain end-to-end

```bash
operon-pp-cli click follow imp_a1b2c3d4e5f60718 --json
```

Reads the /c/{impressionId} redirect Location header, validates the URL scheme (https/http only), detects the operon.so fallback, and HEADs the landing URL to confirm it responds.

## Auth Setup

Two auth lanes. For read-only smoke testing and the sandbox lane, set X-Operon-Client to any well-formed UUID via OPERON_CLIENT_UUID or the auto-generated config file. For production placements, set OPERON_API_KEY to a Bearer key issued by Operon ops. The doctor command reports which lane is active.

Run `operon-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  operon-pp-cli demand --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
operon-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
operon-pp-cli feedback --stdin < notes.txt
operon-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.operon-pp-cli/feedback.jsonl`. They are never POSTed unless `OPERON_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `OPERON_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
operon-pp-cli profile save briefing --json
operon-pp-cli --profile briefing demand
operon-pp-cli profile list --json
operon-pp-cli profile show briefing
operon-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `operon-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add operon-pp-mcp -- operon-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which operon-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   operon-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `operon-pp-cli <command> --help`.
