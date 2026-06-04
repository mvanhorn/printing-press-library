---
name: pp-facebook-marketplace
description: "A write-gated Marketplace seller CLI for search, listing creation, photo upload, local watches, and replies. Trigger phrases: `search Facebook Marketplace`, `watch Marketplace listings`, `draft a Marketplace listing`, `reply to a Marketplace buyer`."
author: "user"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - facebook-marketplace-pp-cli
    install:
      - kind: go
        bins: [facebook-marketplace-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/commerce/facebook-marketplace/cmd/facebook-marketplace-pp-cli
---

# Facebook Marketplace — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `facebook-marketplace-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer into a user bin directory:
   ```bash
   npx -y @mvanhorn/printing-press-library install facebook-marketplace --cli-only --bin-dir ~/.local/bin
   ```
2. Verify: `facebook-marketplace-pp-cli --version`
3. Ensure `~/.local/bin` is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/facebook-marketplace/cmd/facebook-marketplace-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use this CLI for local, human-paced Facebook Marketplace workflows: searching, watching, triaging matches, drafting listings, and replying to seller inbox threads after explicit approval. Avoid using it for bulk automation or cross-platform scraping.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Seller workflow
- **`draft`** — Draft a Marketplace title, description, and price suggestion from photos and notes.

  _Use this when preparing a seller listing before opening a write-gated post flow._

  ```bash
  facebook-marketplace-pp-cli draft --photos chair-front.jpg,chair-tag.jpg --notes "walnut dining chair, small scratch on back" --json
  ```
- **`reply`** — Prepare and send a seller inbox reply only when `--write` and doctor gating both pass.

  _Use this only for human-approved sell-side messaging._

  ```bash
  facebook-marketplace-pp-cli reply --thread 1525836598898750 --message "Yes, it is still available." --write --json
  ```

### Buy-side workflow
- **`watch add`** — Persist a Marketplace search watch with deterministic keyword, price, and distance filters.

  _Use this when the agent needs to monitor Marketplace without deciding relevance on every raw result._

  ```bash
  facebook-marketplace-pp-cli watch add --name "eames" --query "eames lounge" --max-price 1500 --radius 60 --must-have-keywords "chair,lounge" --json
  ```
- **`matches`** — Show new watch matches after deterministic filtering.

  _Use this when an agent needs the shortlist worth showing a human buyer._

  ```bash
  facebook-marketplace-pp-cli matches --new --json
  ```

### Local mirror
- **`stale`** — Find local seller listings older than seven days with no engagement.

  _Use this when deciding which seller listings need price changes or renewal._

  ```bash
  facebook-marketplace-pp-cli stale --days 7 --json
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport over HTTP/3 for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**composer** — Sell-side composer helper operations.

- `facebook-marketplace-pp-cli composer price_prediction` — Fetch Marketplace composer price prediction.
- `facebook-marketplace-pp-cli composer root` — Fetch Marketplace listing composer metadata.
- `facebook-marketplace-pp-cli composer shipping_options` — Fetch calculated shipping options for a draft listing.

**inbox** — Marketplace inbox and messaging operations.

- `facebook-marketplace-pp-cli inbox list` — Fetch Marketplace inbox overview.
- `facebook-marketplace-pp-cli inbox message_seller` — Send a Marketplace seller message.
- `facebook-marketplace-pp-cli inbox seller_threads` — Fetch Marketplace seller inbox threads.
- `facebook-marketplace-pp-cli inbox seller_threads_page` — Fetch a page of Marketplace seller inbox threads.

**listing** — Listing detail and sell-side listing operations.

- `facebook-marketplace-pp-cli listing change_availability` — Change a Marketplace listing availability state.
- `facebook-marketplace-pp-cli listing create` — Create a Marketplace listing from a prepared composer payload; pass `--photo` to upload local photos first.
- `facebook-marketplace-pp-cli listing delete` — Delete a Marketplace for-sale item.
- `facebook-marketplace-pp-cli listing get` — Fetch a Marketplace listing detail page payload.
- `facebook-marketplace-pp-cli listing media` — Fetch Marketplace listing media payload.
- `facebook-marketplace-pp-cli listing upload-photo` — Upload a local photo and return the Marketplace composer `photo_id`.

**marketplace** — Marketplace browse and location operations.

- `facebook-marketplace-pp-cli marketplace browse_feed` — Fetch Marketplace browse feed results.
- `facebook-marketplace-pp-cli marketplace set_browse_radius` — Set Marketplace browse radius.
- `facebook-marketplace-pp-cli marketplace set_buy_location` — Set Marketplace buying location.

**marketplace_search** — Marketplace search operations.

- `facebook-marketplace-pp-cli marketplace_search` — Search Marketplace listings.


**Hand-written commands**

- `facebook-marketplace-pp-cli draft` — AI listing drafter from photos and notes.
- `facebook-marketplace-pp-cli watch` — Buy-side deterministic watcher commands.
- `facebook-marketplace-pp-cli matches` — Show watcher matches after deterministic filtering.
- `facebook-marketplace-pp-cli stale` — Show local listings older than seven days with no engagement.
- `facebook-marketplace-pp-cli reply` — Write-gated sell-side message reply helper.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
facebook-marketplace-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Create a buy-side watch

```bash
facebook-marketplace-pp-cli watch add --name "eames" --query "eames lounge" --max-price 1500 --radius 60 --json
```

Stores deterministic filter criteria locally so future runs can compare new listings.

### Draft a seller listing

```bash
facebook-marketplace-pp-cli draft --photos chair-front.jpg,chair-tag.jpg --notes "walnut dining chair, small scratch" --json
```

Produces listing copy before any write-gated Marketplace mutation.

To create with local photos, run `facebook-marketplace-pp-cli doctor` first, then pass `--write` and one or more `--photo` flags to `listing create`. The CLI uploads each local file, appends the returned `photo_id` values to `variables.input.data.common.photo_ids`, and then submits the create mutation.

## Auth Setup

Run `facebook-marketplace-pp-cli auth login --chrome` while logged in to Facebook in Chrome. The captured browser session is the credential; do not store session material in the Dropbox project workspace.

Run `facebook-marketplace-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  facebook-marketplace-pp-cli inbox list --fb-api-req-friendly-name example-resource --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
facebook-marketplace-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
facebook-marketplace-pp-cli feedback --stdin < notes.txt
facebook-marketplace-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.facebook-marketplace-pp-cli/feedback.jsonl`. They are never POSTed unless `FACEBOOK_MARKET_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `FACEBOOK_MARKET_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
facebook-marketplace-pp-cli profile save briefing --json
facebook-marketplace-pp-cli --profile briefing inbox list --fb-api-req-friendly-name example-resource
facebook-marketplace-pp-cli profile list --json
facebook-marketplace-pp-cli profile show briefing
facebook-marketplace-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `facebook-marketplace-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add facebook-marketplace-pp-mcp -- facebook-marketplace-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which facebook-marketplace-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   facebook-marketplace-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `facebook-marketplace-pp-cli <command> --help`.
