---
name: pp-jinko
description: "Printing Press CLI for Jinko."
author: "Shuai"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - jinko-pp-cli
    install:
      - kind: go
        bins: [jinko-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/travel/jinko/cmd/jinko-pp-cli
---

# Jinko — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `jinko-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install jinko --cli-only
   ```
2. Verify: `jinko-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/jinko/cmd/jinko-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.



## Command Reference

**devplatform** — Manage devplatform

- `jinko-pp-cli devplatform create` — Unified flight shop endpoint.
- `jinko-pp-cli devplatform create-admin` — Add credits to a user's rate limit balance (admin only)
- `jinko-pp-cli devplatform create-keys` — Validate an API key without incrementing rate limit counter (public, no auth required)
- `jinko-pp-cli devplatform create-keys-2` — Verify an API key and increment rate limit counter (public, no auth required)
- `jinko-pp-cli devplatform create-orgs` — Create a new devplatform organization (admin only)
- `jinko-pp-cli devplatform create-orgs-2` — Create a partner API key that can be used to create shadow users (admin only)
- `jinko-pp-cli devplatform create-orgs-3` — Create a shadow user that can later be claimed by a real identity (partner key or admin auth)
- `jinko-pp-cli devplatform create-users` — Create a new developer platform user (admin only)
- `jinko-pp-cli devplatform create-users-2` — Check whether a devplatform user exists and their current status (public, no auth)
- `jinko-pp-cli devplatform create-users-3` — Link a real WorkOS identity to an existing shadow user account. Email is taken from JWT, not request body.
- `jinko-pp-cli devplatform create-users-4` — Create a new API key for the authenticated user
- `jinko-pp-cli devplatform create-users-5` — Self-register a new devplatform account using the authenticated JWT identity
- `jinko-pp-cli devplatform delete` — Revoke an API key by ID for the authenticated user
- `jinko-pp-cli devplatform delete-orgs` — Revoke an API key for a specific user in an organization (org_admin or admin)
- `jinko-pp-cli devplatform get` — Get organization details by ID (org_admin or admin)
- `jinko-pp-cli devplatform get-orgs` — List all users belonging to an organization (org_admin or admin)
- `jinko-pp-cli devplatform get-users` — Resolve the active API key for a user by WorkOS ID (admin only)
- `jinko-pp-cli devplatform list` — List all devplatform organizations (admin only)
- `jinko-pp-cli devplatform list-users` — List all users or filter by status (pending, active, suspended). Admin only.
- `jinko-pp-cli devplatform list-users-2` — Get the devplatform user profile for the authenticated JWT user
- `jinko-pp-cli devplatform list-users-3` — List all API keys for the authenticated user
- `jinko-pp-cli devplatform list-users-4` — Get API usage and rate limit statistics for the authenticated user
- `jinko-pp-cli devplatform update` — Approve a pending devplatform user by WorkOS ID (admin only)
- `jinko-pp-cli devplatform update-admin` — Update the role of a devplatform user (admin only)
- `jinko-pp-cli devplatform update-orgs` — Assign a user as admin of an organization by WorkOS ID (admin only)

**flights** — Manage flights

- `jinko-pp-cli flights create` — Check a flight offer from search results to get confirmed availability and detailed fare options
- `jinko-pp-cli flights create-destinationsearch` — Search for destination cities with cheapest flight options based on filter criteria
- `jinko-pp-cli flights create-refundcheck` — Check whether a flight booking is eligible for a refund and get the refund amount
- `jinko-pp-cli flights create-search` — Search flights (synchronous, filter-based)

**hotels** — Manage hotels

- `jinko-pp-cli hotels create` — Search hotels with a natural-language query (e.g. 'beach
- `jinko-pp-cli hotels create-shop` — Search hotels by destination and return available rooms with live pricing

**payment** — Manage payment

- `jinko-pp-cli payment create` — Create a payment authorization for a quote using Stripe
- `jinko-pp-cli payment create-authorization` — Capture an authorized payment (full or partial amount)
- `jinko-pp-cli payment create-cart` — Create a payment authorization for a shopping cart quote using Stripe
- `jinko-pp-cli payment create-stripe` — Get the Stripe client secret (PaymentIntent flow) or checkout URL (Checkout Session flow)
- `jinko-pp-cli payment get` — Retrieve payment authorization information by ID
- `jinko-pp-cli payment get-authorization` — Check the current status of a payment from Stripe and update local record
- `jinko-pp-cli payment get-quote` — Retrieve payment authorization information by quote ID

**shop** — Manage shop

- `jinko-pp-cli shop create` — Unified trip management endpoint.
- `jinko-pp-cli shop create-fulfillment` — Verify payment with Stripe API and transition fulfillment from awaiting_payment to processing
- `jinko-pp-cli shop create-fulfillment-2` — Retrieve current fulfillment orchestration status and results
- `jinko-pp-cli shop create-fulfillment-3` — Schedule fulfillment orchestration for all successfully quoted products
- `jinko-pp-cli shop create-quote` — Retrieve current quote orchestration status and results
- `jinko-pp-cli shop create-quote-2` — Schedule quote orchestration for all products in a cart
- `jinko-pp-cli shop create-sync` — Selects or updates ancillaries (seats, bags, meals, assistance) for a trip item.
- `jinko-pp-cli shop create-sync-2` — Starts the checkout flow for a trip (cart)
- `jinko-pp-cli shop get` — Retrieve a trip by ID

**shopping-cart** — Manage shopping cart

- `jinko-pp-cli shopping-cart create` — Create a new shopping cart bound to a session
- `jinko-pp-cli shopping-cart delete` — Remove a product from the shopping cart (soft delete)
- `jinko-pp-cli shopping-cart get` — Retrieve a shopping cart with its products
- `jinko-pp-cli shopping-cart get-shoppingcart` — Retrieve a specific product's details from the cart
- `jinko-pp-cli shopping-cart update` — Add a product to an existing shopping cart
- `jinko-pp-cli shopping-cart update-shoppingcart` — Update contact info and/or travelers on a shopping cart

**travelers** — Manage travelers

- `jinko-pp-cli travelers create` — Create a traveler profile
- `jinko-pp-cli travelers delete` — Delete a traveler profile
- `jinko-pp-cli travelers get` — Retrieve a traveler profile by ID
- `jinko-pp-cli travelers list` — List traveler profiles for the authenticated user
- `jinko-pp-cli travelers update` — Update a traveler profile


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
jinko-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

No authentication required.

Run `jinko-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  jinko-pp-cli devplatform list --agent --select id,name,status
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
jinko-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
jinko-pp-cli feedback --stdin < notes.txt
jinko-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/jinko-pp-cli/feedback.jsonl`. They are never POSTed unless `JINKO_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `JINKO_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
jinko-pp-cli profile save briefing --json
jinko-pp-cli --profile briefing devplatform list
jinko-pp-cli profile list --json
jinko-pp-cli profile show briefing
jinko-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `jinko-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/travel/jinko/cmd/jinko-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add jinko-pp-mcp -- jinko-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which jinko-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   jinko-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `jinko-pp-cli <command> --help`.
