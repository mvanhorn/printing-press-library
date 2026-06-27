---
name: pp-flipp
description: "Find local Flipp flyer deals, coupons, and grocery savings by ZIP or postal code. Trigger phrases: `find grocery deals near me`, `check Flipp flyers`, `coupons by zip code`, `weekly grocery ad savings`, `compare my shopping list`, `use Flipp`, `run flipp`."
author: "mlabrenz"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - flipp-pp-cli
    install:
      - kind: go
        bins: [flipp-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/food-and-dining/flipp/cmd/flipp-pp-cli
---

# Flipp — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `flipp-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install flipp --cli-only
   ```
2. Verify: `flipp-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/flipp/cmd/flipp-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Search local weekly flyers, coupons, merchants, and item clippings from Flipp's unauthenticated web endpoints. The CLI adds agent-native JSON, local SQLite, basket comparison, expiring-deal views, and unit-price helpers for grocery planning.

## When to Use This CLI

Use this CLI when a user asks for local grocery flyer deals, coupons, merchant coverage, price comparison, or ZIP-code savings from Flipp. It is strongest for read-only shopping research and agent workflows that need structured output.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to place grocery orders or reserve pickup slots.
- Do not use this CLI for loyalty-account-only coupons that require a retailer login.
- Do not treat Flipp's reverse-engineered endpoints as a guaranteed official API.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local savings workflows
- **`basket price`** — Compare a grocery list across nearby merchants and see the cheapest practical basket.

  _Use this when an agent needs to choose where to shop for a list rather than find one item._

  ```bash
  flipp-pp-cli basket price --items milk,eggs,bread --zip 85001 --agent
  ```
- **`deals scan`** — Scan curated staple categories and rank local deals by discount, urgency, and merchant.

  _Use this when the user asks for the best grocery savings nearby without naming exact products._

  ```bash
  flipp-pp-cli deals scan --category groceries --zip 85001 --min-discount 25 --agent
  ```

### Local snapshots
- **`expiring`** — Find local flyer and coupon savings that expire within a chosen window.

  _Use this when timing matters and the agent needs to prioritize deals before they disappear._

  ```bash
  flipp-pp-cli expiring --days 3 --zip 85001 --agent
  ```
- **`coverage`** — Show which nearby merchants have active food flyers, searchable items, and coupon coverage.

  _Use this before shopping-plan work to understand which stores have enough data to compare._

  ```bash
  flipp-pp-cli coverage --zip 85001 --agent
  ```
- **`watchlist add`** — Persist target prices for recurring staples and compare them against future synced snapshots.

  _Use this when the user wants recurring savings alerts rather than a one-time search._

  ```bash
  flipp-pp-cli watchlist add milk --target-price 3.50 --zip 85001 --agent
  ```

### Deal quality
- **`unit-price`** — Normalize item prices by package size when Flipp listings include parseable quantities.

  _Use this when raw prices are misleading because package sizes differ._

  ```bash
  flipp-pp-cli unit-price milk --zip 85001 --agent
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**flyers** — Browse active local flyers and flyer items

- `flipp-pp-cli flyers data` — Fetch the combined local flyer and coupon data payload
- `flipp-pp-cli flyers items` — List item clippings from a specific flyer
- `flipp-pp-cli flyers list` — List active flyers for a ZIP or postal code

**items** — Search Flipp flyer and ecommerce deal items

- `flipp-pp-cli items <q>` — Search local flyer and ecommerce deals by keyword

**location** — Resolve a default Flipp location

- `flipp-pp-cli location` — Detect the current public IP location as a Flipp postal code

**merchants** — List merchants Flipp tracks

- `flipp-pp-cli merchants` — List Flipp merchants available for a ZIP or postal code


## Freshness Contract

This printed CLI owns bounded freshness only for registered store-backed read command paths. In `--data-source auto` mode, those paths check `sync_state` and may run a bounded refresh before reading local data. `--data-source local` never refreshes. `--data-source live` reads the API and does not mutate the local store. Set `FLIPP_NO_AUTO_REFRESH=1` to skip the freshness hook without changing source selection.

Covered paths:

- `flipp-pp-cli flyers`
- `flipp-pp-cli flyers get`
- `flipp-pp-cli flyers list`
- `flipp-pp-cli flyers search`
- `flipp-pp-cli merchants`
- `flipp-pp-cli merchants get`
- `flipp-pp-cli merchants list`
- `flipp-pp-cli merchants search`

When JSON output uses the generated provenance envelope, freshness metadata appears at `meta.freshness`. Treat it as current-cache freshness for the covered command path, not a guarantee of complete historical backfill or API-specific enrichment.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
flipp-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Compare a short grocery basket

```bash
flipp-pp-cli basket price --items milk,eggs,bread --zip 85001 --agent --select merchant,total,items.name,items.price
```

Groups matching deals by merchant so an agent can pick the cheapest local trip.

### Find expiring food savings

```bash
flipp-pp-cli expiring --days 3 --zip 85001 --agent
```

Prioritizes local deals whose flyer validity window is closing.

### Inspect one flyer clipping list

```bash
flipp-pp-cli flyers items 8005907 --sid 1234567890123456 --json --select name,price,cutout_image_url
```

Returns flyer item text plus image URLs for ambiguous listings.

### Scan grocery category deals

```bash
flipp-pp-cli deals scan --category groceries --zip 85001 --min-discount 25 --agent
```

Fans out staple searches and ranks the best local discounts.

## Auth Setup

No authentication required.

Run `flipp-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  flipp-pp-cli flyers list --agent --select id,name,status
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

- Use `--home <dir>` for one invocation, or set `FLIPP_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `FLIPP_CONFIG_DIR`, `FLIPP_DATA_DIR`, `FLIPP_STATE_DIR`, `FLIPP_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `FLIPP_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `flipp-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "flipp": {
        "command": "flipp-pp-mcp",
        "env": {
          "FLIPP_HOME": "/srv/flipp"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `FLIPP_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `FLIPP_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
flipp-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
flipp-pp-cli feedback --stdin < notes.txt
flipp-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `FLIPP_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `FLIPP_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
flipp-pp-cli profile save briefing --json
flipp-pp-cli --profile briefing flyers list
flipp-pp-cli profile list --json
flipp-pp-cli profile show briefing
flipp-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `flipp-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/food-and-dining/flipp/cmd/flipp-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add flipp-pp-mcp -- flipp-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which flipp-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   flipp-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `flipp-pp-cli <command> --help`.
