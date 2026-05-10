---
name: pp-ordertogo
description: "Browse, cart, and place orders at any OrderToGo.com restaurant from the terminal — pure-Go agent-native client... Trigger phrases: `order my usual`, `place my regular order`, `order from <restaurant>`, `what's my usual order`, `how much have I spent at`, `is <restaurant> open right now`, `use ordertogo`, `run ordertogo`."
author: "user"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - ordertogo-pp-cli
---

# OrderToGo — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `ordertogo-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install ordertogo --cli-only
   ```
2. Verify: `ordertogo-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/ordertogo/cmd/ordertogo-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

OrderToGo.com is a multi-tenant pickup ordering platform that powers small chains across multiple US metros. There is no CLI, MCP, or SDK presence anywhere for it. This CLI gives an agent every read endpoint, a safe `order plan` that composes a cart locally and validates against a `--max` cap, and an `order place` that drives a headless Chrome through Braintree DropIn to actually submit. Everything is offline-cacheable, agent-callable, and verify-env safe by construction.

## When to Use This CLI

Reach for this CLI when an agent needs to handle 'order my usual', 'is the restaurant open', 'how much have I spent here this month', or 'place this exact cart with a budget cap'. It's read-fast (everything cached locally after one sync), write-safe (every place command is gated behind --confirm + --max + verify-env short-circuit), and works for any OrderToGo customer regardless of metro.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Safe agent-driven ordering
- **`order plan`** — Recompose your previous order locally, validate against tax, and refuse to fire if total exceeds your --max cap.

  _Reach for this when an agent needs to know 'will this fit my budget' before committing — answer comes from one structured call with no browser involvement._

  ```bash
  ordertogo-pp-cli order plan --reuse-last --max 30 --json
  ```
- **`order place`** — Drive a headless Chrome via chromedp to complete the Braintree DropIn flow with your saved card, with the budget cap enforced before the browser opens.

  _Use when an agent has a budget-validated plan and the user has confirmed; this is the one command that actually moves money._

  ```bash
  ordertogo-pp-cli order place --reuse-last --confirm --max 30
  ```

### Local state that compounds
- **`usual`** — Cluster your historical orders by item-set similarity and surface the recurring set that defines 'your usual' at any restaurant you order from.

  _Reach for this when a user says 'order my usual' and the agent needs to decide whether one obvious pattern exists or whether to ask which usual._

  ```bash
  ordertogo-pp-cli usual --restaurant <slug> --json
  ```
- **`spending`** — Total spent, average order, days since last order, top items, and weekly cadence — all from local order history with one SQL query.

  _Use when an agent needs to answer 'how much have I spent here' or 'how often do I order' without re-fetching history._

  ```bash
  ordertogo-pp-cli spending --since 90d --json
  ```
- **`order plan`** — Pass --tip auto and the CLI applies your average tip percentage from history at this restaurant.

  _Reach for this when an agent doesn't want to make the user pick a tip percentage — go with what's habitual._

  ```bash
  ordertogo-pp-cli order plan --reuse-last --tip auto --json
  ```

### Reachability mitigation
- **`order plan`** — Refuse to fire if the restaurant is not currently open at the requested pickup time, using cached hours.

  _Catches a class of agent failures before the payment flow opens._

  ```bash
  ordertogo-pp-cli order plan --reuse-last --pickup-at '7:00 PM'
  ```

### Agent-native plumbing
- **`agent-context`** — Single-call structured dump: account, default restaurant, your usual, last-order summary, budget hint, days-since-last.

  _Reach for this when an agent enters a session and needs a complete picture before suggesting any action._

  ```bash
  ordertogo-pp-cli agent-context --json
  ```
- **`order place`** — All side-effect commands short-circuit when PRINTING_PRESS_VERIFY=1 is set, printing 'would place: <plan>' instead of submitting.

  _Use when an agent or test harness wants to exercise the place path without real-world consequences._

  ```bash
  PRINTING_PRESS_VERIFY=1 ordertogo-pp-cli order place --reuse-last --confirm --max 30
  ```

## Command Reference

**coupons** — Promotional coupons available to your account

- `ordertogo-pp-cli coupons list` — List active coupons for your account (endpoint shape inferred from web 'My Coupons' panel)
- `ordertogo-pp-cli coupons mark_used` — Mark a promotion code as used after applying it to an order

**giftcards** — Giftcard balances and history per restaurant

- `ordertogo-pp-cli giftcards` — List your giftcards across all restaurants (endpoint shape inferred from web 'My Giftcards' panel)

**notifications** — Notification badge for orders, rewards, and platform messages

- `ordertogo-pp-cli notifications` — Count of unread notifications for your account

**orders** — Order history, detail, validation, and tracking - the core ordering data path

- `ordertogo-pp-cli orders cancel` — Cancel your own order within the void window (typically before preparing-state)
- `ordertogo-pp-cli orders list` — List your order history across all restaurants (returns latest N orders, server-paginated)
- `ordertogo-pp-cli orders show` — Get order detail by orderid - items, options, totals, payment method, points earned, status timeline
- `ordertogo-pp-cli orders track` — HTML order tracking page (received → preparing → ready → picked up). Parsed for status by `order track`.
- `ordertogo-pp-cli orders validate` — Pre-validate a cart - returns an order token plus tax computation, used by `order plan` before any payment surface opens

**payment** — Braintree client token for payment-method nonce generation (used internally by chromedp headless flow)

- `ordertogo-pp-cli payment braintree_token` — Returns a Braintree client token used by the DropIn UI to mint a single-use payment nonce. Hand-driven by chromedp...
- `ordertogo-pp-cli payment checkout` — Submit order with payment nonce and customer details. Body must include `nonce` from Braintree client. The CLI uses...

**restaurants** — Restaurants on the OrderToGo platform - filter by location, view detail, list multi-location chains

- `ordertogo-pp-cli restaurants list` — List restaurants in a location code (e.g. sto for Seattle area)
- `ordertogo-pp-cli restaurants menu` — Full menu for a restaurant with categories, items, and modifier options
- `ordertogo-pp-cli restaurants show` — Show restaurant detail - hours, address, phone, location code, multi-location chain mapping

**rewards** — Reward points balances per restaurant

- `ordertogo-pp-cli rewards` — List your reward points across all restaurants (endpoint shape inferred from web 'My Rewards' panel)


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
ordertogo-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Order your usual under a cap

```bash
ordertogo-pp-cli orders plan --reuse-last --max 30 --json --select totals.subtotal,totals.tax,totals.tip,totals.total,items.name
```

Compose your usual cart, validate with tax, and emit only the bottom-line numbers + item names. The /m/api/orders response includes every cart field; --select keeps the agent context tight.

### Place after agent confirmation (verify-env safe)

```bash
PRINTING_PRESS_VERIFY=1 ordertogo-pp-cli orders place --reuse-last --confirm --max 30
```

Run only after the agent has shown the plan to the user and the user has approved. Under PRINTING_PRESS_VERIFY=1 the CLI prints 'would place' without launching Chrome - useful for dry-run agent flows.

### Single-call agent dump

```bash
ordertogo-pp-cli agent-context --json
```

Returns account, default restaurant, last-order summary, your usual, budget hint, days-since-last - everything an agent needs to decide whether to suggest 'order my usual'.

### Find a restaurant in your area

```bash
ordertogo-pp-cli restaurants list --location-code sto --json --select id,name,slug,is_open
```

List all restaurants in the Seattle (sto) location. Replace sto with your metro code (det, sea, etc) - reads from synced order history when available.

### Show recent order history

```bash
ordertogo-pp-cli orders list --json
```

Hit /m/api/getmicmeshorders directly for your latest orders. Combine with --select to keep payloads tight.

## Auth Setup

OrderToGo authenticates via Firebase phone-OTP on the web. The CLI does not implement phone OTP — instead, `auth login --chrome` imports your existing OrderToGo session cookies from your local Chrome profile (default user-data-dir, default profile). After import, every command travels with the cookie; refresh by re-running `auth login --chrome` if your session expires.

Run `ordertogo-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  ordertogo-pp-cli coupons list --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
ordertogo-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
ordertogo-pp-cli feedback --stdin < notes.txt
ordertogo-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.ordertogo-pp-cli/feedback.jsonl`. They are never POSTed unless `ORDERTOGO_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ORDERTOGO_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
ordertogo-pp-cli profile save briefing --json
ordertogo-pp-cli --profile briefing coupons list
ordertogo-pp-cli profile list --json
ordertogo-pp-cli profile show briefing
ordertogo-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `ordertogo-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add ordertogo-pp-mcp -- ordertogo-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which ordertogo-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   ordertogo-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `ordertogo-pp-cli <command> --help`.
