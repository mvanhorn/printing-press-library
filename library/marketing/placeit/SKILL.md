---
name: pp-placeit
description: "Every one of Placeit's 164k mockup, logo, video, and design templates — searchable, sortable by real popularity Trigger phrases: `search placeit for t-shirt mockups`, `find printify-ready mockups`, `best selling placeit logos`, `build a twitch kit`, `what's new on placeit`, `use placeit`, `run placeit`."
author: "Vincent Colombo"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - placeit-pp-cli
    install:
      - kind: go
        bins: [placeit-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/marketing/placeit/cmd/placeit-pp-cli
---

# Placeit — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `placeit-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install placeit --cli-only
   ```
2. Verify: `placeit-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/placeit/cmd/placeit-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Placeit has no public API, no bulk search, and no offline catalog. This CLI mirrors the entire Algolia-indexed catalog into a local SQLite database, then layers on commands no Placeit tool has: rank results by actual purchase count (top), filter Printify-ready mockups for a POD pipeline (pod), assemble matched Twitch kits (kit), and cross-tabulate the catalog's tag facets (gaps). Search and browse need no login; account and saved-templates use your Placeit session.

## When to Use This CLI

Use this CLI to search, filter, and analyze Placeit's template catalog programmatically: finding the best-selling or Printify-ready mockups for a product line, assembling matched streamer kits, or auditing catalog coverage across tag facets. It is ideal for print-on-demand sellers, streamers, and social-media managers who batch-produce assets and want scriptable, offline-cacheable catalog access.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to actually render or download a finished design — Placeit's render/download is a browser-only flow; use 'open' to launch the template in your browser instead.
- Do not use this CLI to edit a template's text, colors, or uploaded artwork — there is no headless editor.
- Do not use this CLI to manage billing or change a Placeit subscription.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Popularity intelligence
- **`top`** — Rank matching templates by real purchase count, not opaque relevance.

  _Reach for this when a seller needs the mockups that actually convert, not just the first search hits._

  ```bash
  placeit-pp-cli top "t-shirt" --category mockups --agent
  ```
- **`rank`** — Show a template's purchase percentile within its category and tag cohort.

  _Reach for this to judge whether a specific template is a strong pick before committing to it._

  ```bash
  placeit-pp-cli rank 41935 --agent
  ```

### Print-on-demand
- **`pod`** — Return only Printify-compatible mockups for a query with deep links for a listing pipeline.

  _Reach for this when batching POD listings so you never open a mockup that isn't print-ready._

  ```bash
  placeit-pp-cli pod "hoodie" --agent --select stage_link,editor_link
  ```

### Matched sets
- **`kit`** — Assemble a coherent streamer kit (overlay, panels, emotes, frame) and flag missing slots.

  _Reach for this when a streamer rebrand needs a visually consistent set, not mismatched single assets._

  ```bash
  placeit-pp-cli kit "neon gaming" --agent
  ```

### Taxonomy navigation
- **`industry-map`** — Map the 152-entry industry taxonomy with template counts from your synced mirror.

  _Reach for this to size a niche and find the industries with the most ready-made templates._

  ```bash
  placeit-pp-cli industry-map "coffee shop"
  ```

### Monitoring
- **`watch`** — Persist named searches and report templates newly matching since the last sync.

  _Reach for this to keep a client niche fresh without re-running searches by hand every week._

  ```bash
  placeit-pp-cli watch run --agent
  ```

### Catalog analytics
- **`gaps`** — Pivot the catalog across two tag facets to surface under-served template combinations.

  _Reach for this to spot mockup coverage gaps (e.g. a device with thin on-model diversity)._

  ```bash
  placeit-pp-cli gaps --facet device_tags --by ethnicity_tags
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**account** — Your Placeit account and subscription status (requires a logged-in session)

- `placeit-pp-cli account` — Show the signed-in user's account type and subscription status

**bookmarks** — Templates you've saved/bookmarked on Placeit (requires a logged-in session)

- `placeit-pp-cli bookmarks` — List the signed-in user's bookmarked templates

**campaigns** — Active Placeit marketing campaigns and promotions (no login required)

- `placeit-pp-cli campaigns` — List active Placeit campaigns and promotions


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
placeit-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Find the best-selling Printify t-shirt mockups

```bash
placeit-pp-cli pod "t-shirt" --agent --select name,stage_link,purchases
```

Filters to Printify-ready mockups and narrows the JSON to just the fields a listing pipeline needs.

### Rank cached logos by popularity in a niche

```bash
placeit-pp-cli top "coffee shop logo" --category logos --limit 20 --agent
```

Orders local-mirror matches by real purchase count for agent consumption.

### Audit on-model mockup diversity

```bash
placeit-pp-cli gaps --facet device_tags --by ethnicity_tags
```

Cross-tabulates two tag facets to reveal under-served mockup combinations.

### Narrow a deeply nested template record for an agent

```bash
placeit-pp-cli template 41935 --agent --select name,category_name,stage_link,large_thumb,device_tags
```

Template records carry 40+ fields and 9 tag arrays; --select keeps only what the agent needs.

### Watch a client niche for new templates

```bash
placeit-pp-cli watch add "halloween instagram" && placeit-pp-cli watch run --agent
```

Saves a named search, then reports templates newly published since the last sync.

## Auth Setup

Search, browse, sync, and template lookup use Placeit's public Algolia catalog and need no credentials. The account and bookmarks commands read your Placeit (Envato) session, which you import once from Chrome via the auth login step shown in Quick Start. Because Placeit sits behind Cloudflare, the CLI ships a browser-compatible HTTP transport for those calls.

Run `placeit-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  placeit-pp-cli bookmarks --user-id 550e8400-e29b-41d4-a716-446655440000 --agent --select id,name,status
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
placeit-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
placeit-pp-cli feedback --stdin < notes.txt
placeit-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/placeit-pp-cli/feedback.jsonl`. They are never POSTed unless `PLACEIT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `PLACEIT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
placeit-pp-cli profile save briefing --json
placeit-pp-cli --profile briefing bookmarks --user-id 550e8400-e29b-41d4-a716-446655440000
placeit-pp-cli profile list --json
placeit-pp-cli profile show briefing
placeit-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `placeit-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/marketing/placeit/cmd/placeit-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add placeit-pp-mcp -- placeit-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which placeit-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   placeit-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `placeit-pp-cli <command> --help`.
