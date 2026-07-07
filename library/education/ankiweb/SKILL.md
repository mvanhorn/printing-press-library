---
name: pp-ankiweb
description: "The only terminal-native way to search, rank Trigger phrases: `ankiweb shared decks`, `search anki decks`, `best anki deck for`, `download anki deck`, `my synced anki decks`, `use ankiweb`, `run ankiweb`."
author: "Paul Bockewitz"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - ankiweb-pp-cli
---

# AnkiWeb — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `ankiweb-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install ankiweb --cli-only
   ```
2. Verify: `ankiweb-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/education/ankiweb/cmd/ankiweb-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Reach for AnkiWeb CLI when you want to discover, evaluate, or track community shared decks from the terminal or an agent, list your cloud-synced decks, search the cards already in your collection, or add new notes. It is the right tool for ranking decks by quality, filtering by audio coverage, comparing candidates, watching for new decks, searching your own cards, and adding cards — things the AnkiWeb website does slowly or an agent cannot otherwise reach. For editing or deleting existing cards, reviewing/studying, or local desktop automation, use an AnkiConnect-based tool instead.

## When Not to Use This CLI

Do not activate this CLI for requests that edit or delete existing cards, review/study cards, create decks, or perform local desktop automation — use an AnkiConnect-based tool for those. The only write this CLI performs is adding new notes via `notes add` (guarded by `--dry-run` and confirmation); everything else is read-only inspection, search, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Catalog intelligence the website can't do
- **`shared rank`** — Rank shared decks by approval rate (upvotes vs downvotes) with a minimum-vote floor, instead of the raw vote counts the website shows.

  _Pick the highest-quality deck for a topic in one command instead of eyeballing vote counts deck by deck._

  ```bash
  ankiweb-pp-cli shared rank spanish --min-votes 20 --agent
  ```
- **`shared search`** — Filter shared decks by whether they include audio or images, critical for language learners.

  _Surface only media-rich decks when audio matters (language study) without opening each deck._

  ```bash
  ankiweb-pp-cli shared search japanese --has-audio --agent
  ```
- **`compare`** — Compare multiple shared decks in one table: approval rate, note count, audio/image coverage, and freshness.

  _Decide between near-duplicate decks at a glance instead of flipping between tabs._

  ```bash
  ankiweb-pp-cli compare 241428882 815543631 --agent
  ```
- **`shared fresh`** — Rank or filter shared decks by last-modified date to surface actively maintained decks.

  _Avoid stale abandoned decks by finding the ones updated recently._

  ```bash
  ankiweb-pp-cli shared fresh anatomy --since 2024-01-01 --agent
  ```

### Local state that compounds
- **`watch`** — Show shared decks that are new or changed for a search term since your last sync.

  _Re-run a weekly topic search and see only what's new instead of re-scanning the whole list._

  ```bash
  ankiweb-pp-cli watch spanish --since-last-sync --agent
  ```
- **`drift`** — Track download-count changes on the decks you've published, between syncs.

  _See whether your published decks are gaining traction over time without manual note-taking._

  ```bash
  ankiweb-pp-cli drift --agent
  ```
- **`brief`** — One digest for a topic: top decks by approval rate, audio coverage, the freshest deck, and how many are new since last sync.

  _Get a complete read on a topic's deck landscape in one call instead of running four commands._

  ```bash
  ankiweb-pp-cli brief spanish --agent
  ```

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 5 API entries from 6 total network entries
- Protocols: protobuf (95% confidence), sveltekit_spa (90% confidence)
- Auth signals: cookie — cookies: has_auth
- Generation hints: protobuf_responses_require_handwritten_decoders, cookie_auth_validated, download_requires_signed_token
- Candidate command ideas: search — GET /svc/shared/list-decks?search= returns repeated deck protobuf {id,title,upvotes,downvotes,modified,notes,audio,images}; info — GET /svc/shared/item-info?sharedId= returns full deck detail + reviews protobuf; download — GET /svc/shared/download-deck/{id}?t= requires client-minted signed token (op=sdd); token generation unresolved; list — POST /svc/decks/deck-list-info returns user's synced decks; cookie-gated
- Caveats: : All API responses are protobuf, not JSON; generated JSON decoding will not work without hand-written wire-format readers.; : Shared-deck download needs a signed ?t= token minted by AnkiWeb client JS; not reproducible from captured traffic alone.

## Command Reference

**decks** — Your cloud-synced decks and study stats (requires AnkiWeb login)

- `ankiweb-pp-cli decks` — List your synced decks with card counts and study stats (protobuf response; 200 with session cookie, 403 without)

**shared** — Browse, search, and download public shared decks (no login required)

- `ankiweb-pp-cli shared download` — Download a shared deck .apkg. NOTE: requires a signed ?
- `ankiweb-pp-cli shared info` — Full detail + reviews for one shared deck (protobuf response)
- `ankiweb-pp-cli shared search` — Search the shared-deck catalog by keyword (protobuf response)


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
ankiweb-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Find the best audio-rich Spanish deck

```bash
ankiweb-pp-cli shared rank spanish --has-audio --min-votes 20 --agent --select decks.title,decks.approval,decks.notes
```

Ranks audio-bearing Spanish decks by approval rate and narrows the JSON to just title, approval, and note count.

### Compare three anatomy decks

```bash
ankiweb-pp-cli compare 241428882 815543631 1713698257 --agent
```

One table of approval rate, notes, audio/image coverage, and freshness across the three deck ids.

### What anatomy decks are new this week

```bash
ankiweb-pp-cli watch anatomy --since-last-sync --agent
```

Diffs the current catalog against your last sync and lists only new or changed decks.

### Topic briefing

```bash
ankiweb-pp-cli brief japanese --agent
```

Top decks by approval, audio coverage percentage, the freshest deck, and new-since-sync count in one digest.

## Auth Setup

AnkiWeb uses a session cookie, not an API key. Run `auth login --chrome` to import your logged-in ankiweb.net session, or set ANKIWEB_COOKIES. Public shared-deck search and info need no login.

Run `ankiweb-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  ankiweb-pp-cli decks --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
ankiweb-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
ankiweb-pp-cli feedback --stdin < notes.txt
ankiweb-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/ankiweb-pp-cli/feedback.jsonl`. They are never POSTed unless `ANKIWEB_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ANKIWEB_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
ankiweb-pp-cli profile save briefing --json
ankiweb-pp-cli --profile briefing decks
ankiweb-pp-cli profile list --json
ankiweb-pp-cli profile show briefing
ankiweb-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `ankiweb-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add ankiweb-pp-mcp -- ankiweb-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which ankiweb-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   ankiweb-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `ankiweb-pp-cli <command> --help`.
