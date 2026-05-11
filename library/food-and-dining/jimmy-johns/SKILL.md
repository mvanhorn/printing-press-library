---
name: pp-jimmy-johns
description: "First terminal CLI for Jimmy John's ordering — local Unwich conversion, agent-native JSON, every endpoint typed. Trigger phrases: `jimmy john's`, `freaky fast`, `unwich`, `jj order`, `use jimmy-johns`, `run jimmy-johns`."
author: "Omar Shahine"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - jimmy-johns-pp-cli
---

# Jimmy John's — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `jimmy-johns-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install jimmy-johns --cli-only
   ```
2. Verify: `jimmy-johns-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Browse stores and menus, build carts, view rewards, and one-shot reorders from the terminal. The Unwich converter computes lettuce-wrap modifier deltas locally so agents can build no-bread orders without an extra API round trip.

## When to Use This CLI

Use this CLI when scripting Jimmy John's order workflows from the terminal, when building a cart for an agent that needs typed access to JJ's menu and rewards data, or when caching menu state for offline composition. The Unwich converter is particularly useful for low-carb diet flows.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local cart composition
- **`menu unwich-convert`** — Convert a sandwich's modifier set to an Unwich (lettuce wrap) variant — pure-local computation, no live API call.

  _Reach for this when an agent is building a JJ cart for a user with a no-bread preference — it gives you the exact modifier delta with no API round-trip._

  ```bash
  jimmy-johns-pp-cli menu product-modifiers 33328641 --json | jimmy-johns-pp-cli menu unwich-convert --product-id 33328641 --json
  ```
- **`order plan`** — Suggest a sized cart for a group order — sandwiches + sides + cookies + drinks scaled to N people with dietary filters.

  _Reach for this when an agent gets a 'lunch for the team' request — it returns a ready-to-submit cart structure with rationale per line._

  ```bash
  jimmy-johns-pp-cli order plan --people 8 --dietary vegetarian --json
  ```
- **`menu half-and-half`** — Compose a two-product share order with the agent-facing note that JJ doesn't natively support half-and-half slicing.

  _Reach for this when a user says 'half Vito, half Pepe' — the command outputs the actual cart and the in-store ask the user has to make._

  ```bash
  jimmy-johns-pp-cli menu half-and-half --left 33328641 --right 33328700 --json
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**account** — User account, profile, addresses, and saved payments

- `jimmy-johns-pp-cli account current` — Get the authenticated user's profile (name, email, preferences).
- `jimmy-johns-pp-cli account delivery_addresses` — List the authenticated user's saved delivery addresses.
- `jimmy-johns-pp-cli account login` — Authenticate with email + password. Sets JJ session cookies.
- `jimmy-johns-pp-cli account saved_payments` — List the authenticated user's saved payment methods.
- `jimmy-johns-pp-cli account web_token` — Refresh the web session token (called internally by the SPA).

**menu** — Menu products, filters, and modifier options

- `jimmy-johns-pp-cli menu product_filters` — List available menu filter dimensions (categories, dietary tags, allergens).
- `jimmy-johns-pp-cli menu product_modifiers` — List modifier groups (bread, toppings, add-ons) for a specific product.
- `jimmy-johns-pp-cli menu products` — List menu products for the current store (subs, sides, drinks, cookies, catering).

**order** — Cart and order management

- `jimmy-johns-pp-cli order add_items` — Add one or more items to the current cart in a single call.
- `jimmy-johns-pp-cli order current` — Get the current in-progress order/cart.
- `jimmy-johns-pp-cli order upsell` — Get upsell suggestions for the current cart (sides, drinks, cookies).

**rewards** — Freaky Fast Rewards points balance and catalog

- `jimmy-johns-pp-cli rewards catalog` — List available reward redemptions for the current points balance.
- `jimmy-johns-pp-cli rewards summary` — Get the authenticated user's rewards points balance and recent activity.

**stores** — Jimmy John's store locations and operating info

- `jimmy-johns-pp-cli stores get_disclaimers` — Get store-specific disclaimers (delivery zone caveats, hours warnings).
- `jimmy-johns-pp-cli stores list` — List stores. Accepts an address search or filter; returns stores with hours, distance, pickup/delivery flags.

**system** — System utilities (Google Maps signing for store finder)

- `jimmy-johns-pp-cli system` — Sign a Google Maps URL for client-side use (used internally by store finder)


## Freshness Contract

This printed CLI owns bounded freshness only for registered store-backed read command paths. In `--data-source auto` mode, those paths check `sync_state` and may run a bounded refresh before reading local data. `--data-source local` never refreshes. `--data-source live` reads the API and does not mutate the local store. Set `JIMMY_JOHNS_NO_AUTO_REFRESH=1` to skip the freshness hook without changing source selection.

Covered paths:

- `jimmy-johns-pp-cli menu`
- `jimmy-johns-pp-cli menu product_filters`
- `jimmy-johns-pp-cli menu product_modifiers`
- `jimmy-johns-pp-cli menu products`
- `jimmy-johns-pp-cli stores`
- `jimmy-johns-pp-cli stores get_disclaimers`
- `jimmy-johns-pp-cli stores list`

When JSON output uses the generated provenance envelope, freshness metadata appears at `meta.freshness`. Treat it as current-cache freshness for the covered command path, not a guarantee of complete historical backfill or API-specific enrichment.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
jimmy-johns-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Show every menu item that has Unwich support

```bash
jimmy-johns-pp-cli menu products --json | jq '.[] | select(.category=="sandwich")'
```

Filter the local menu cache to sandwiches before piping into the unwich converter.

## Auth Setup

Jimmy John's runs PerimeterX bot protection. Authenticate by capturing cookies from a fresh, hand-driven Chrome session via 'browser-use cookies export', then 'jimmy-johns-pp-cli auth import-cookies --from-file <path>'. Sessions that get fingerprinted by automation stay flagged for ~1 hour.

Run `jimmy-johns-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  jimmy-johns-pp-cli stores list --agent --select id,name,status
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
jimmy-johns-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
jimmy-johns-pp-cli feedback --stdin < notes.txt
jimmy-johns-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.jimmy-johns-pp-cli/feedback.jsonl`. They are never POSTed unless `JIMMY_JOHNS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `JIMMY_JOHNS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
jimmy-johns-pp-cli profile save briefing --json
jimmy-johns-pp-cli --profile briefing stores list
jimmy-johns-pp-cli profile list --json
jimmy-johns-pp-cli profile show briefing
jimmy-johns-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `jimmy-johns-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add jimmy-johns-pp-mcp -- jimmy-johns-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which jimmy-johns-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   jimmy-johns-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `jimmy-johns-pp-cli <command> --help`.
