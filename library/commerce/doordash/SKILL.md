---
name: pp-doordash
description: "Printing Press CLI for Doordash. DoorDash GraphQL operation spec curated from sniffed traffic and ashah360/doordash-mcp query files. This..."
author: "bricenice17"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - doordash-pp-cli
---

# Doordash — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `doordash-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install doordash --cli-only
   ```
2. Verify: `doordash-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/doordash/cmd/doordash-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Read-only DoorDash workflows
- **`search`** — Search DoorDash stores through the consumer GraphQL autocomplete feed without mutating cart or order state.

  _Useful for meal/vendor research while preserving account safety._

  ```bash
  doordash-pp-cli search "thai" --agent
  ```
- **`menu`** — Fetch DoorDash store menus in a normalized shape without changing the cart.

  _Lets agents compare menu choices before any cart mutation is considered._

  ```bash
  doordash-pp-cli menu --store-id STORE_ID --agent
  ```
- **`item-options`** — Inspect item option groups and nested add-ons before adding anything to a cart.

  _Agents can reason about required modifiers before proposing a cart change._

  ```bash
  doordash-pp-cli item-options --store-id STORE_ID --item-id ITEM_ID --agent
  ```
- **`recent-orders`** — Read recent DoorDash order summaries without placing a new order.

  _Supports repeat-order and preference analysis without checkout risk._

  ```bash
  doordash-pp-cli recent-orders --limit 3 --agent
  ```

### Guarded mutation boundary
- **`cart`** — Expose cart inspection and mutations as a separately named, guarded command family rather than mixing them into search/menu reads.

  _Clear command boundaries reduce accidental purchase-flow side effects._

  ```bash
  doordash-pp-cli cart --help
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport over HTTP/3 for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**graphql** — DoorDash web GraphQL operations. All operations POST JSON to /graphql/<operation>?operation=<operation>; mutations must be wrapped by guarded domain commands before live use.

- `doordash-pp-cli graphql create-add-cart-item` — Mutation: add an item to a DoorDash cart. Must remain hard-gated in domain commands.
- `doordash-pp-cli graphql create-autocomplete-facet-feed` — Search DoorDash autocomplete/store facets by free-text query.
- `doordash-pp-cli graphql create-checkout` — Fetch checkout data for a cart without placing an order.
- `doordash-pp-cli graphql create-consumer-order-cart` — Fetch the current consumer order cart.
- `doordash-pp-cli graphql create-convenience-search-query` — Search retail/convenience catalog items within a DoorDash store.
- `doordash-pp-cli graphql create-create-order-from-cart` — Mutation: submit a DoorDash order from a cart. Must require explicit live-order gate.
- `doordash-pp-cli graphql create-delete-cart` — Mutation: delete a DoorDash cart. Must remain hard-gated in domain commands.
- `doordash-pp-cli graphql create-detailed-cart-items` — Fetch detailed cart item data for checkout/cart review.
- `doordash-pp-cli graphql create-get-has-new-notifications` — Fetch notification count/status.
- `doordash-pp-cli graphql create-get-open-carts-count` — Fetch count of open carts for the session.
- `doordash-pp-cli graphql create-item-page` — Fetch item details and customization options for a store item.
- `doordash-pp-cli graphql create-list-carts` — List active/recent DoorDash carts visible to the authenticated session.
- `doordash-pp-cli graphql create-poll-order-payment-status` — Poll payment status for a submitted order.
- `doordash-pp-cli graphql create-promo-sticky-footer` — Fetch promo sticky-footer data for a cart/store context.
- `doordash-pp-cli graphql create-remove-cart-item-v2` — Mutation: remove an item from a DoorDash cart. Must remain hard-gated in domain commands.
- `doordash-pp-cli graphql create-storepage-feed` — Fetch a store page/menu feed for a DoorDash store ID.
- `doordash-pp-cli graphql create-total-fee-tally` — Fetch fees/tax/total estimate for a cart.
- `doordash-pp-cli graphql create-update-cart-item-v2` — Mutation: update cart item quantity/options. Must remain hard-gated in domain commands.
- `doordash-pp-cli graphql create-validate-consumer-address-with-address-link-id` — Validate a DoorDash address link ID.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
doordash-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

This CLI uses a DoorDash browser-session Cookie header plus optional CSRF token. Export the Cookie header through an approved private flow, then import without printing secrets:

```bash
doordash-pp-cli auth login --cookie-file - --csrf-token <token> --json < /secure/path/cookie-header.txt
```

Avoid passing real cookies as shell arguments; stdin or a 0600 file is safer. Run `doordash-pp-cli doctor --json` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  doordash-pp-cli graphql create-add-cart-item --operation-name example-resource --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
doordash-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
doordash-pp-cli feedback --stdin < notes.txt
doordash-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.doordash-pp-cli/feedback.jsonl`. They are never POSTed unless `DOORDASH_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `DOORDASH_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
doordash-pp-cli profile save briefing --json
doordash-pp-cli --profile briefing graphql create-add-cart-item --operation-name example-resource
doordash-pp-cli profile list --json
doordash-pp-cli profile show briefing
doordash-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `doordash-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add doordash-pp-mcp -- doordash-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which doordash-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   doordash-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `doordash-pp-cli <command> --help`.
