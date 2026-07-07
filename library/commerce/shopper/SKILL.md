---
name: pp-shopper
description: "The first CLI for Shopper — every catalog, cart Trigger phrases: `search shopper for arroz`, `what's in my shopper basket`, `when is my shopper charge`, `did my shopper prices go up`, `what should I restock on shopper`, `use shopper`, `run shopper`."
author: "educrvz"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - shopper-pp-cli
    install:
      - kind: go
        bins: [shopper-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/commerce/shopper/cmd/shopper-pp-cli
---

# Shopper — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `shopper-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install shopper --cli-only
   ```
2. Verify: `shopper-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/shopper/cmd/shopper-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Shopper's recurring basket, fixed charge-7-days-before clock, and drifting prices produce a time series the official app discards every cycle. This CLI keeps it in a local SQLite store, unlocking charge-calendar, basket diff, price-watch, restock prediction, catalog-drift detection, and cashback optimization — none of which any Shopper interface offers.

## When to Use This CLI

Use this CLI to inspect and reason about a Shopper grocery subscription: search the catalog, read the recurring basket and delivery schedule, and answer cross-cycle questions about price changes, charge dates, restock timing, and cashback. It shines for questions the web UI can't answer because it only shows the current cycle.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to place or pay for an order — checkout/payment is intentionally out of scope.
- Do not use it for non-Shopper grocery services (iFood, Daki, Rappi, Carrefour).
- Do not use it to change account passwords or payment cards.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Time & Money Clock
- **`charge-calendar`** — Every upcoming cycle's charge date, edit-lock deadline, and delivery date in one timeline so you never miss an edit window or get surprised by a charge.

  _Reach for this before any basket edit to answer 'can I still change the order / when does money leave the account'._

  ```bash
  shopper charge-calendar --weeks 8 --locking-soon --agent
  ```

### Basket Intelligence
- **`basket diff`** — Compares your current recurring basket against a previous cycle's snapshot to show exactly what was added, dropped, or re-quantified before the template locks.

  _Reach for this to audit what changed before confirming a cycle, or to explain why this charge differs from the last._

  ```bash
  shopper basket diff --from last-cycle --to current --agent --select added,removed,price_changed
  ```
- **`restock predict`** — Predicts when you'll run out of each staple from your historical buying cadence and suggests what to add to the upcoming basket.

  _Reach for this to proactively fill the next basket so the household doesn't run out of cafe/arroz/fralda mid-cycle._

  ```bash
  shopper restock predict --horizon 14d --suggest-adds --agent
  ```

### Price & Catalog Drift
- **`price-watch`** — Tracks the price history of the SKUs you actually buy and alerts when one rises or drops meaningfully versus your own purchase baseline.

  _Reach for this before confirming a cycle to catch staples that quietly got pricier, or find real drops worth stocking up on._

  ```bash
  shopper price-watch --threshold 8% --since 60d --only-basket --agent
  ```
- **`catalog drift`** — Flags products you buy that were discontinued, silently swapped, or kept their price while shrinking the pack, surfacing the real R$/kg or R$/L change.

  _Reach for this when the user asks 'why is my bill the same but I have less', or to auto-find replacements for staples that vanished._

  ```bash
  shopper catalog drift --metric per-unit --kind shrinkflation,discontinued --since 90d --agent
  ```

### Cashback Optimization
- **`cashback optimize`** — Computes the cheapest set of items to add (or whether to wait) to cross the next cashback tier, favoring things you'll need anyway.

  _Reach for this near the edit deadline to decide whether topping up the basket earns net-positive cashback without buying junk._

  ```bash
  shopper cashback optimize --tier 2399 --reward 100 --prefer-restock --agent
  ```

## Command Reference

**address** — Operations on address

- `shopper-pp-cli address` — GET /address/

**cart** — Operations on summary

- `shopper-pp-cli cart add` — Add a product to the cart or increase its quantity
- `shopper-pp-cli cart summary` — GET /cart/summary
- `shopper-pp-cli cart remove` — Remove a product from the cart or decrease its quantity

**catalog** — Operations on departments

- `shopper-pp-cli catalog count` — POST /catalog/search/count
- `shopper-pp-cli catalog filters` — POST /catalog/search/filters
- `shopper-pp-cli catalog search` — POST /catalog/search
- `shopper-pp-cli catalog banner-view` — GET /catalog/banners/{banner_id}/view
- `shopper-pp-cli catalog banners` — GET /catalog/banners
- `shopper-pp-cli catalog departments` — GET /catalog/departments
- `shopper-pp-cli catalog news` — GET /catalog/products/news
- `shopper-pp-cli catalog suggest` — GET /catalog/search/suggest

**delivery** — Operations on summary

- `shopper-pp-cli delivery calendar` — GET /delivery/v2/calendar
- `shopper-pp-cli delivery summary` — GET /delivery/summary

**features** — Operations on toggle

- `shopper-pp-cli features select` — POST /features/stores/select
- `shopper-pp-cli features start` — POST /features/timer/start
- `shopper-pp-cli features view` — POST /features/toggle/view
- `shopper-pp-cli features stores` — GET /features/stores
- `shopper-pp-cli features tick` — GET /features/timer/tick
- `shopper-pp-cli features toggle` — GET /features/toggle

**session** — Session and social-login validation

- `shopper-pp-cli session` — GET /auth/validation/social


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
shopper-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Find a staple and check your price baseline

```bash
shopper catalog search 'cafe pilao' --agent --select results.name,results.price && shopper price-watch --only-basket --threshold 5%
```

Search the catalog, then see whether your tracked staples drifted in price.

### Pre-deadline cycle review

```bash
shopper charge-calendar --locking-soon --agent && shopper basket diff --from last-cycle --to current --agent
```

Check which cycle locks soon and audit what changed before it does.

### Hit the cashback tier efficiently

```bash
shopper cashback optimize --tier 2399 --reward 100 --prefer-restock --agent
```

Decide the cheapest useful top-up to earn cashback.

## Auth Setup

Shopper uses a Bearer JWT. Sign in at shopper.com.br in your browser, copy the token from the Authorization header of any siteapi.shopper.com.br request (DevTools > Network), and set SHOPPER_TOKEN. The CLI also sends app-os-x-version and your store context automatically.

Run `shopper-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  shopper-pp-cli address --agent --select id,name,status
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
shopper-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
shopper-pp-cli feedback --stdin < notes.txt
shopper-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/shopper-pp-cli/feedback.jsonl`. They are never POSTed unless `SHOPPER_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SHOPPER_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
shopper-pp-cli profile save briefing --json
shopper-pp-cli --profile briefing address
shopper-pp-cli profile list --json
shopper-pp-cli profile show briefing
shopper-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `shopper-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/commerce/shopper/cmd/shopper-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add shopper-pp-mcp -- shopper-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which shopper-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   shopper-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `shopper-pp-cli <command> --help`.
