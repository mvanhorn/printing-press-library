---
name: pp-offerup
description: "Search local OfferUp listings from your terminal, keep them in a local database Trigger phrases: `find deals on OfferUp`, `what's the going rate for X on OfferUp`, `search OfferUp near me`, `any new OfferUp listings for X`, `check OfferUp prices`, `use offerup`, `run offerup`."
author: "Trevin Chow"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - offerup-pp-cli
    install:
      - kind: go
        bins: [offerup-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/commerce/offerup/cmd/offerup-pp-cli
---

# OfferUp — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `offerup-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install offerup --cli-only
   ```
2. Verify: `offerup-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/offerup/cmd/offerup-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use this CLI when an agent or user needs local-marketplace data from OfferUp: searching listings near a ZIP, pulling an item's full detail, vetting a seller, or — its real edge — answering price questions (what's the going rate, what's underpriced, what dropped) over a local store of listings. All of that works with no login. When logged in (`auth login --chrome`), it also manages your own account: your listings (view active/archived, mark sold, archive), saved lists, and messages. It does not create listings (OfferUp web posts Jobs only; listing creation is mobile-app-only) or send messages.

## When Not to Use This CLI

The public commands are read-only. The authenticated commands add two account mutations — `my-listings mark-sold` and `my-listings archive` — both gated behind login and an explicit `--confirm`. Do not use this CLI to create/post a listing (mobile-app-only), send messages, make or accept offers, purchase, or otherwise change remote state it does not expose.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local price intelligence
- **`price-check`** — See the real going rate for an item in your area — median, 25th/75th percentile, min, max, and the firm-vs-negotiable split — computed across every listing you've pulled.

  _Reach for this before buying or pricing a resale — it answers "what is this actually worth here" with a number instead of a scroll._

  ```bash
  offerup-pp-cli price-check "herman miller aeron" --zip 85001 --agent
  ```
- **`deals`** — Surface listings priced a chosen percentage below the local median for that item — the underpriced finds, ranked by how far under they are.

  _The deal-sniping command: it tells an agent which listings are underpriced right now, not just what exists._

  ```bash
  offerup-pp-cli deals "dewalt drill" --zip 98101 --below 25 --agent
  ```
- **`price-drops`** — Detect listings whose price fell between syncs — the same item, now cheaper — sorted by the size of the drop.

  _Catches sellers who just cut a price on something you're tracking — the moment to make an offer._

  ```bash
  offerup-pp-cli price-drops "macbook pro" --since 7d --agent
  ```

### Track saved searches
- **`new-since`** — Show only the listings that appeared since a cutoff for a saved search, so you never re-scan items you already saw.

  _Use this for a recurring watch — it answers "what dropped since I last looked" in one call._

  ```bash
  offerup-pp-cli new-since "road bike" --since 24h --agent
  ```
- **`digest`** — A single composite report for a saved search combining what's new, what dropped in price, and what's underpriced.

  _The morning-ritual command and the natural single MCP tool call for an agent watching a market._

  ```bash
  offerup-pp-cli digest "snowboard" --since 24h --agent
  ```

### Seller intelligence
- **`seller-scan`** — Pull a seller's full synced inventory alongside their reputation badges (business/dealer/TruYou), join date, and the median asking price across their listings.

  _Vet a dealer before buying to flip, or watch a high-volume seller's pricing in one view._

  ```bash
  offerup-pp-cli seller-scan 161842229 --agent
  ```

## Authenticated Commands (login required)

These act on the user's own OfferUp account and need a captured session. Auth is the OfferUp web session cookie, captured once via the `press-auth` companion through a controlled Chrome login window (no API key; the session is encrypted at rest). All public commands above need none of this.

```bash
offerup-pp-cli auth login --chrome     # one-time: opens Chrome, captures the session
offerup-pp-cli auth status --json      # {"loggedIn": true, "pressAuthInstalled": true}
offerup-pp-cli account --agent         # your own profile + reputation (auth tokens never surfaced)
offerup-pp-cli my-listings --agent     # your active listings
offerup-pp-cli my-listings archived --agent
offerup-pp-cli saved --agent           # your saved/favorited lists
offerup-pp-cli messages --agent        # your message threads
offerup-pp-cli messages read <discussion-id> --agent
```

Mutations preview by default and apply only with `--confirm`:

```bash
offerup-pp-cli my-listings mark-sold <listing-id> --confirm
offerup-pp-cli my-listings archive <listing-id> --confirm
```

Without login, these commands exit with an auth error pointing at `auth login --chrome` (exit code 4). Creating a new listing is not supported (mobile-app-only).

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**listings** — Search and view OfferUp listings (public, no login)

- `offerup-pp-cli listings get` — Get the full detail for one listing
- `offerup-pp-cli listings search` — Search live OfferUp listings by keyword and location


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
offerup-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Narrow a verbose search for an agent

```bash
offerup-pp-cli listings search "macbook pro" --zip 98101 --agent --select listingId,title,price,locationName,conditionText
```

Listing payloads are wide; --agent with --select returns only the fields an agent needs, saving context.

### Find this week's underpriced finds

```bash
offerup-pp-cli deals "aeron chair" --zip 85001 --below 30 --agent
```

Lists Aeron chairs priced at least 30% below the local median around Phoenix.

### Daily watch on a saved search

```bash
offerup-pp-cli digest "road bike" --since 24h --agent
```

One call returns what's new, what dropped in price, and what's a deal for road bikes since yesterday.

### Vet a seller before buying to flip

```bash
offerup-pp-cli seller-scan 161842229 --agent
```

Shows the seller's badges, join date, full inventory, and median asking price in one view.

## Auth Setup

Public commands need no login: search, item detail (listings get), seller lookup, category-scoped browse, and every price-intelligence command (price-check, deals, new-since, price-drops, digest, seller-scan). Set location with --zip (or --lat/--lon), not a credential. Account commands act on your own OfferUp account and require login: account (your profile), my-listings (plus archived, mark-sold, archive), saved, and messages (plus messages read). Auth is your OfferUp web session cookie. Capture it once with `auth login --chrome`, which extracts the cookie straight from your logged-in browser — no extra install needed. The optional `press-auth` companion adds a smoother one-click controlled-Chrome capture and encrypts the session at rest, but it is not required. You can also paste the cookie via the `OFFERUP_COOKIE` environment variable or `auth set-token`. Check with `auth status`, clear with `auth logout`. The mutating commands my-listings mark-sold and my-listings archive preview by default and apply only with --confirm. Creating a new listing is not supported — OfferUp's web app posts Jobs only, so listing creation is mobile-app-only.

Run `offerup-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  offerup-pp-cli listings get mock-value --agent --select id,name,status
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
offerup-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
offerup-pp-cli feedback --stdin < notes.txt
offerup-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/offerup-pp-cli/feedback.jsonl`. They are never POSTed unless `OFFERUP_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `OFFERUP_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
offerup-pp-cli profile save briefing --json
offerup-pp-cli --profile briefing listings get mock-value
offerup-pp-cli profile list --json
offerup-pp-cli profile show briefing
offerup-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `offerup-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/commerce/offerup/cmd/offerup-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add offerup-pp-mcp -- offerup-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which offerup-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   offerup-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `offerup-pp-cli <command> --help`.
