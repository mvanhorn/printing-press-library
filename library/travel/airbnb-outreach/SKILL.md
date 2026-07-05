---
name: pp-airbnb-outreach
description: "Printing Press CLI for Airbnb. Script Airbnb's real authenticated surface — search stays, contact hosts and property owners with templated..."
author: "JimPresting"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - airbnb-outreach-pp-cli
    install:
      - kind: go
        bins: [airbnb-outreach-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/travel/airbnb-outreach/cmd/airbnb-outreach-pp-cli
---

# Airbnb — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `airbnb-outreach-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install airbnb --cli-only
   ```
2. Verify: `airbnb-outreach-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/airbnb-outreach/cmd/airbnb-outreach-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Script Airbnb's real authenticated surface — search stays, contact hosts and property owners with templated messages and photos, and keep an offline searchable archive of every conversation and saved listing.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Outreach that scales
- **`outreach run`** — Search a location and message the top hosts with a templated message in one command.

  _Reach for this when a user wants to contact many property owners at once (relocation, long-stay, business inquiry)._

  ```bash
  airbnb-outreach-pp-cli outreach run "Berlin, Germany" --message "Hi {name} team, interested in a monthly stay — possible?" --limit 5
  ```
- **`outreach crm`** — Local record of every host you contacted, when, and about which listing.

  _Use after an outreach run to track who was contacted before following up._

  ```bash
  airbnb-outreach-pp-cli outreach crm --json
  ```

### Local state that compounds
- **`archive search`** — Full-text search across every message thread you've synced, offline.

  _Use to find what a host promised across months of conversations without scrolling the web inbox._

  ```bash
  airbnb-outreach-pp-cli archive search "early check-in"
  ```
- **`watch check`** — Track saved listings' prices over time and report drops.

  _Use to catch a price drop on a listing a user is considering._

  ```bash
  airbnb-outreach-pp-cli watch add 49070135 --checkin 2026-08-10 --checkout 2026-08-14 && airbnb-outreach-pp-cli watch check
  ```

### Reachability mitigation
- **`ops refresh`** — Re-harvest Airbnb's current GraphQL persisted-query hashes from its own JS so the CLI survives Airbnb deploys.

  _Run this if commands start failing after an Airbnb update, before assuming the CLI is broken._

  ```bash
  airbnb-outreach-pp-cli ops refresh
  ```

## Command Reference

**markets** — Public locale/market metadata (used for the health check)

- `airbnb-outreach-pp-cli markets` — Fetch Airbnb locale/market metadata (public, no auth)


**Hand-written commands**

- `airbnb-outreach-pp-cli search <location>` — Search Airbnb stays by location, dates, guests, price and filters
- `airbnb-outreach-pp-cli listing <listing-id>` — Show full detail for a listing (host, amenities, house rules, location)
- `airbnb-outreach-pp-cli quote <listing-id>` — Get a price breakdown quote for a listing and date range (read-only)
- `airbnb-outreach-pp-cli inbox` — List your Airbnb message threads (requires auth login)
- `airbnb-outreach-pp-cli thread <thread-id>` — Read a conversation with a host, newest messages first
- `airbnb-outreach-pp-cli contact <listing-id>` — Start a conversation with a host / property owner (dry-run by default)
- `airbnb-outreach-pp-cli message send|send-image|mark-read <thread-id>` — Send a message, send photo(s), or mark a thread read (writes need --confirm)
- `airbnb-outreach-pp-cli wishlist list|items` — List your wishlists and their saved listings
- `airbnb-outreach-pp-cli trips` — List your upcoming and past reservations
- `airbnb-outreach-pp-cli me` — Show the signed-in account (name, id, host status)
- `airbnb-outreach-pp-cli outreach run|crm` — Bulk-contact hosts from a search with a templated message + photo, and track replies (CRM)
- `airbnb-outreach-pp-cli archive <query>` — Offline full-text search across every synced conversation and saved listing
- `airbnb-outreach-pp-cli watch <listing-id>` — Track a listing's price over time and alert on drops
- `airbnb-outreach-pp-cli sync` — Sync inbox, wishlists, trips and watched listings into the local store
- `airbnb-outreach-pp-cli ops refresh|list` — Self-heal the GraphQL operation-hash registry by re-harvesting Airbnb's current hashes
- `airbnb-outreach-pp-cli auth login|status|logout` — Import your logged-in Chrome Airbnb session (login --chrome), check or clear it


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
airbnb-outreach-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

No authentication required.

Run `airbnb-outreach-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  airbnb-outreach-pp-cli markets --agent --select id,name,status
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
airbnb-outreach-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
airbnb-outreach-pp-cli feedback --stdin < notes.txt
airbnb-outreach-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.airbnb-outreach-pp-cli/feedback.jsonl`. They are never POSTed unless `AIRBNB_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `AIRBNB_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
airbnb-outreach-pp-cli profile save briefing --json
airbnb-outreach-pp-cli --profile briefing markets
airbnb-outreach-pp-cli profile list --json
airbnb-outreach-pp-cli profile show briefing
airbnb-outreach-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `airbnb-outreach-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/travel/airbnb-outreach/cmd/airbnb-outreach-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add airbnb-outreach-pp-mcp -- airbnb-outreach-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which airbnb-outreach-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   airbnb-outreach-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `airbnb-outreach-pp-cli <command> --help`.
