---
name: pp-harris-teeter
description: "Printing Press CLI for Harris Teeter. Harris Teeter grocery shopping API discovered from the logged-in web app"
author: "Jonathan Moss"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - harris-teeter-pp-cli
    install:
      - kind: go
        bins: [harris-teeter-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/commerce/harris-teeter/cmd/harris-teeter-pp-cli
---

# Harris Teeter — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `harris-teeter-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install harris-teeter --cli-only
   ```
2. Verify: `harris-teeter-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.25+):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/harris-teeter/cmd/harris-teeter-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Harris Teeter grocery shopping API discovered from the logged-in web app

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Browser-backed reachability
- **`auth login --chrome`** — Read Harris Teeter session cookies from the already logged-in Chrome session, falling back through agent-browser/browser-use when standalone cookie tools are missing.

  _Use this first when the CLI reports auth errors or a browser session has changed._

  ```bash
  harris-teeter-pp-cli auth login --chrome
  ```

### Store-context grocery reads
- **`products search`** — Search Harris Teeter products using the browser-observed Atlas search endpoint with location, fulfillment method, and LAF/modality headers.

  _Use this to inspect current store-specific grocery results from a terminal or agent workflow._

  ```bash
  harris-teeter-pp-cli products search --query milk --location-id 09700096 --page-size 5 --json --no-input
  ```
- **`products get`** — Fetch full product, offer, nutrition, inventory, and variant projections by UPC/GTIN through the logged-in Atlas product endpoint.

  _Use this when an agent needs exact item metadata without scraping a rendered product page._

  ```bash
  harris-teeter-pp-cli products get --upc 0007203673813 --json --no-input
  ```

### Account-aware savings
- **`coupons`** — List Harris Teeter digital coupons from the authenticated web endpoint, including optional UPC filtering.

  _Use this to check available savings before building a grocery list._

  ```bash
  harris-teeter-pp-cli coupons --page-size 5 --json --no-input
  ```
- **`cart`** — Inspect the authenticated Harris Teeter cart and shopping-list surfaces without exposing mutating checkout or order actions.

  _Use this when an agent needs to understand the current cart state without changing it._

  ```bash
  harris-teeter-pp-cli cart --json --no-input
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 166 API entries from 889 total network entries
- Protocols: rest_json (75% confidence)
- Auth signals: bearer_token — headers: Authorization; api_key — query: filter.keyword, key
- Generation hints: browser_http_transport, requires_protected_client, weak_schema_confidence
- Candidate command ideas: create_EdClXAWAB — Derived from observed POST /qB1py7FxiRxje/xBaRro/nGmSVUVg/N3irhr9NJ5N5cQ/dW1ccAkbBg/G3J/EdClXAWAB traffic.; create_dont_forget_usual_products — Derived from observed POST /atlas/v1/recommendations/v1/dont-forget-usual-products traffic.; create_echoData — Derived from observed POST /clickstream/v1/echoData traffic.; create_events — Derived from observed POST /clickstream/v1/events traffic.; create_preferences — Derived from observed POST /atlas/v1/modality/preferences traffic.; create_prioritized_carousels — Derived from observed POST /atlas/v1/search/v1/prioritized-carousels traffic.; create_qESoJeGYu — Derived from observed POST /qB1py7FxiRxje/xBaRro/nGmSVUVg/p8irhr9N/Ph5ncAkbBg/V3l/qESoJeGYu traffic.; create_realtimeconversion — Derived from observed POST /track/realtimeconversion traffic.
- Caveats: empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.

## Command Reference

**account** — Inspect logged-in customer preferences and membership state.

- `harris-teeter-pp-cli account enrollments` — List membership enrollments and benefits for the current account.
- `harris-teeter-pp-cli account preferences` — List customer preferences for the logged-in account.

**cart** — Inspect the logged-in account cart.

- `harris-teeter-pp-cli cart` — List carts for the current logged-in Harris Teeter account.

**coupons** — List available digital coupons and coupon-product links.

- `harris-teeter-pp-cli coupons` — List available digital coupons, optionally filtered by UPC.

**lists** — Inspect Harris Teeter shopping lists.

- `harris-teeter-pp-cli lists get` — Get a shopping list by ID.
- `harris-teeter-pp-cli lists list` — List shopping lists for the current logged-in account.

**products** — Search Harris Teeter products, look up item details, and inspect search facets.

- `harris-teeter-pp-cli products get` — Get full product, offer, nutrition, inventory, and variant details by UPC/GTIN.
- `harris-teeter-pp-cli products related-tags` — Get related search tags for a query and location.
- `harris-teeter-pp-cli products search` — Search products for a store location and fulfillment method.
- `harris-teeter-pp-cli products suggestions` — Get search suggestions for a query and location.
- `harris-teeter-pp-cli products visual-navigations` — Get visual navigation categories shown on search pages.

**recommendations** — Inspect personalized grocery recommendations from the web app.

- `harris-teeter-pp-cli recommendations better-for-you` — Get better-for-you product recommendations.
- `harris-teeter-pp-cli recommendations purchase-history-homepage` — Get homepage purchase-history shortcuts for the logged-in account.
- `harris-teeter-pp-cli recommendations start-my-cart` — Get the Start My Cart product recommendations shown on the homepage.

**stores** — Find Harris Teeter stores and store metadata.

- `harris-teeter-pp-cli stores` — Find stores by ZIP code, city, state, or address text.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
harris-teeter-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

This CLI uses a browser session. Log in to .harristeeter.com in Chrome, then:

```bash
harris-teeter-pp-cli auth login --chrome
```

The CLI uses `pycookiecheat`, `cookies`, or `cookie-scoop-cli` when available, and falls back to a live Chrome session through `agent-browser`/`browser-use` when those tools are missing.

Product and coupon endpoints also need Harris Teeter location/availability/fulfillment headers. The CLI defaults to the captured Beau Rivage Marketplace context (`location-id=09700096`, `facility-id=12956`). Override with `HARRIS_TEETER_LOCATION_ID`, `HARRIS_TEETER_FACILITY_ID`, `HARRIS_TEETER_MODALITY_TYPE`, or a full `HARRIS_TEETER_LAF_OBJECT` JSON value when using a different store.

Run `harris-teeter-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  harris-teeter-pp-cli cart --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
harris-teeter-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
harris-teeter-pp-cli feedback --stdin < notes.txt
harris-teeter-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.harris-teeter-pp-cli/feedback.jsonl`. They are never POSTed unless `HARRIS_TEETER_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `HARRIS_TEETER_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
harris-teeter-pp-cli profile save briefing --json
harris-teeter-pp-cli --profile briefing cart
harris-teeter-pp-cli profile list --json
harris-teeter-pp-cli profile show briefing
harris-teeter-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `harris-teeter-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/commerce/harris-teeter/cmd/harris-teeter-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add harris-teeter-pp-mcp -- harris-teeter-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which harris-teeter-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   harris-teeter-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `harris-teeter-pp-cli <command> --help`.
