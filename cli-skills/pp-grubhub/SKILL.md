---
name: pp-grubhub
description: "Every Grubhub restaurant search, plus a sortable delivery-fee comparison board, a cross-restaurant deal radar Trigger phrases: `find restaurants near me on grubhub`, `compare grubhub delivery fees`, `who near me has a poke bowl`, `grubhub deals near`, `what should I order on grubhub`, `use grubhub`, `run grubhub`."
author: "Vincent Colombo"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - grubhub-pp-cli
    install:
      - kind: go
        bins: [grubhub-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/commerce/grubhub/cmd/grubhub-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/commerce/grubhub/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Grubhub — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `grubhub-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install grubhub --cli-only
   ```
2. Verify: `grubhub-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/grubhub/cmd/grubhub-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

grubhub-pp-cli browses Grubhub's marketplace from the command line: search restaurants by street address, browse full menus, and compare delivery fees, minimums, and ETAs across the whole neighborhood at once with `compare`. It caches restaurants and menus in a local SQLite store so `dish` can full-text-search menu items across nearby restaurants and `deals` can rank every active offer in one sweep. No API key required — it mints an anonymous Grubhub token for you.

## When to Use This CLI

Use grubhub-pp-cli when a user wants to find, compare, or order-research food delivery near an address: cheapest or fastest restaurant, a specific dish across the neighborhood, or the best current deal. It is ideal for agents that need deterministic JSON about restaurants, fees, menus, and offers without scraping the Grubhub web app. The local cache makes repeated comparisons over the same location fast and offline.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to place an order or manage a cart — it is read-only marketplace browsing.
- Do not use this CLI for order history or account/rewards data — logged-in features require a real Grubhub account login that this version does not support.
- Do not use this CLI for DoorDash, Uber Eats, or other delivery services — it only covers Grubhub/Seamless.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`compare`** — See delivery fee, minimum, ETA, rating, and distance for every nearby restaurant side by side, sorted however you want.

  _Reach for this when a user wants the cheapest or fastest option across the whole neighborhood, not one restaurant at a time._

  ```bash
  grubhub-pp-cli compare "350 5th Ave, New York, NY" --sort eta --agent
  ```
- **`dish`** — Find which nearby restaurants carry a specific dish, with price, by full-text searching cached menus across the neighborhood.

  _Reach for this when the user names a dish, not a restaurant: 'who near me has a poke bowl under $15'._

  ```bash
  grubhub-pp-cli dish "350 5th Ave, New York, NY" "poke bowl" --max-price 15 --agent
  ```

### Deal hunting
- **`deals`** — Rank every nearby restaurant currently running an offer, coupon, or promo code in one sweep.

  _Reach for this when the deal should pick the restaurant, not the other way around._

  ```bash
  grubhub-pp-cli deals "350 5th Ave, New York, NY" --agent
  ```
- **`pick`** — Get one recommended restaurant from a transparent score over fee, rating, active deals, and ETA.

  _Reach for this when the user wants a single 'just pick one' answer with a visible score breakdown._

  ```bash
  grubhub-pp-cli pick "350 5th Ave, New York, NY" --weight-deal 2 --agent
  ```

## Command Reference

**Primary (address-based, zero setup — auto-geocode + anonymous token)**

- `grubhub-pp-cli near <address>` — Search restaurants near a street address (filters: --cuisine, --pickup, --sort, --open-now, --limit)
- `grubhub-pp-cli compare <address>` — Sortable delivery fee/minimum/ETA/rating board (filters: --sort, --max-fee, --max-min, --eta-under)
- `grubhub-pp-cli dish <address> <query>` — Find which nearby restaurants carry a dish (--max-price, --diet, --max-scan-restaurants; --data-source local for cached)
- `grubhub-pp-cli deals <address>` — Rank nearby restaurants by active offers/coupons (--sort value|count)
- `grubhub-pp-cli pick <address>` — One recommended restaurant from a transparent score (--weight-fee/-eta/-rating/-deal)
- `grubhub-pp-cli menu <restaurant-id>` — Browse a restaurant's full menu (--category, --popular, --limit)
- `grubhub-pp-cli item <restaurant-id> <item-id>` — Show a menu item's modifiers and prices
- `grubhub-pp-cli geocode <address>` — Resolve a street address to coordinates

**Raw API surface (hidden; needs `auth login` or GRUBHUB_TOKEN; takes POINT(lng lat))**

- `grubhub-pp-cli restaurants search` — Raw restaurant search by location
- `grubhub-pp-cli restaurants get <id>` — Raw restaurant details + full menu
- `grubhub-pp-cli restaurants menu-item <id> <item-id>` — Raw menu item detail


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
grubhub-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Cheapest fast delivery near an address

```bash
grubhub-pp-cli compare "350 5th Ave, New York, NY" --sort fee --eta-under 30 --agent
```

Comparison board filtered to sub-30-minute ETAs, cheapest delivery first, as agent JSON.

### Find a dish across the neighborhood

```bash
grubhub-pp-cli dish "1 Infinite Loop, Cupertino, CA" "burrito" --max-price 12
```

Full-text search of cached menus for burritos under $12 at any nearby restaurant.

### Pull a full menu as compact JSON for an agent

```bash
grubhub-pp-cli menu 1414955 --agent --select menu_category_list.menu_item_list.name,menu_category_list.menu_item_list.price
```

Restaurant menus are deeply nested; --select narrows the payload to item names and prices so agents don't burn context on the full blob.

### Let the deal pick the restaurant

```bash
grubhub-pp-cli deals "350 5th Ave, New York, NY" --sort value
```

Ranks every nearby restaurant running an offer by value in one sweep.

## Auth Setup

Grubhub has no public API key. grubhub-pp-cli authenticates exactly like the website: it scrapes a fresh anonymous client id from grubhub.com and mints a short-lived bearer token automatically on first use, caching it locally. You don't set anything. For the raw `restaurants` endpoint commands you can optionally run `grubhub-pp-cli auth login` (still credential-free) or set GRUBHUB_TOKEN. Logged-in features like order history are not supported in this version.

Run `grubhub-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  grubhub-pp-cli restaurants get mock-value --agent --select id,name,status
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
grubhub-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
grubhub-pp-cli feedback --stdin < notes.txt
grubhub-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/grubhub-pp-cli/feedback.jsonl`. They are never POSTed unless `GRUBHUB_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `GRUBHUB_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
grubhub-pp-cli profile save briefing --json
grubhub-pp-cli --profile briefing restaurants get mock-value
grubhub-pp-cli profile list --json
grubhub-pp-cli profile show briefing
grubhub-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `grubhub-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/commerce/grubhub/cmd/grubhub-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add grubhub-pp-mcp -- grubhub-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which grubhub-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   grubhub-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `grubhub-pp-cli <command> --help`.
