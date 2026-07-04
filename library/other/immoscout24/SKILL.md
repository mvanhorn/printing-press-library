---
name: pp-immoscout24
description: "Search ImmoScout24 through the replayable mobile JSON API, not brittle blocked HTML. Trigger phrases: `search ImmoScout24`, `find apartments on ImmobilienScout24`, `check this ImmoScout expose`, `use immoscout24`, `run immoscout24`."
author: "Thilo"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - immoscout24-pp-cli
    install:
      - kind: go
        bins: [immoscout24-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/immoscout24/cmd/immoscout24-pp-cli
---

# ImmoScout24 — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `immoscout24-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install immoscout24 --cli-only
   ```
2. Verify: `immoscout24-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/immoscout24/cmd/immoscout24-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Use the mobile JSON surface behind ImmoScout24 to count searches, list fresh listing cards, retrieve map markers, and inspect expose details. The CLI keeps responses agent-friendly with JSON, selection, bounded paging, and local sync conventions.

## When to Use This CLI

Use this CLI when a task needs ImmoScout24 listing search, expose detail inspection, or repeatable apartment-hunt monitoring from agent workflows. Prefer it over website scraping because the mobile JSON API is replayable and structured.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to bypass paywalls, CAPTCHA, login gates, or contact restrictions.
- Do not use this CLI for official business listing insertion/export workflows that require ImmoScout24 OAuth credentials.
- Do not use this CLI for high-volume scraping or proxy rotation.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Mobile JSON search
- **`immoscout24-mobile-search total`** — Count matching ImmoScout24 mobile listings before paging through results.

  _Use this first to understand query size and avoid unnecessarily paging broad searches._

  ```bash
  immoscout24-pp-cli immoscout24-mobile-search total --geocodes /de/berlin/berlin --realestatetype apartmentrent --pricetype calculatedtotalrent --json --select results.totalResults
  ```
- **`immoscout24-mobile-search list`** — Fetch listing cards with IDs, titles, address snippets, prices, room counts, thumbnails, and shareable expose links.

  _Use this when an agent needs real listing IDs and summaries to inspect or compare._

  ```bash
  immoscout24-pp-cli immoscout24-mobile-search list --geocodes /de/berlin/berlin --realestatetype apartmentrent --pricetype calculatedtotalrent --pagesize 1 --json --select results.resultListItems.item.id,results.resultListItems.item.title,results.resultListItems.item.web_url
  ```
- **`immoscout24-mobile-search map`** — Retrieve map markers and paging metadata for broad regional searches.

  _Use this to estimate geographic spread and map hit counts before fetching details._

  ```bash
  immoscout24-pp-cli immoscout24-mobile-search map --geocodes /de/berlin/berlin --real-estate-type apartmentrent --pagesize 2 --json --select results.paging.numberOfHits,results.markers.type
  ```

### Expose details
- **`expose`** — Inspect detailed mobile expose sections for a real listing ID.

  _Use this after listing search when an agent needs detail sections for one property._

  ```bash
  immoscout24-pp-cli expose 168983602 --json --compact
  ```
- **`expose inspect`** — Extract grouped expose text sections plus deduplicated property image/media URLs and separately detected floorplans for careful property review.

  _Use this when an agent must inspect wording, amenities, captions, pictures, floorplans, and other media before recommending a property._

  ```bash
  immoscout24-pp-cli expose inspect 168983602 --json --select results.id,results.web_url,results.text_count,results.texts.title,results.texts.items.value,results.image_count,results.floorplan_count,results.floorplan_document_count,results.images.render_url,results.floorplans.render_url,results.floorplan_documents.url
  ```

### Link translation
- **`translate-url`** — Convert between ImmoScout24 web links, mobile API URLs, expose IDs, and CLI commands.

  _Use this to turn a shared web link into replayable mobile API calls, or turn CLI search parameters back into links you can send._

  ```bash
  immoscout24-pp-cli translate-url --search-type radius --geocoordinates '50.078;8.445;8' --realestatetype apartmentrent --pricetype calculatedtotalrent --price=-2500 --numberofrooms=3- --livingspace=100- --equipment=parking --fulltext modern --json --select web_url,mobile_urls.list,cli_commands.list
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**expose** — Retrieve expose detail records.

- `immoscout24-pp-cli expose <id>` — Returns detailed sections, media, contact metadata, map information, and tracking fields for an expose ID.
- `immoscout24-pp-cli expose inspect <id>` — Extracts grouped expose text sections, image/media URLs, separately detected floorplans, and non-image links for detailed review.

**immoscout24-mobile-search** — Manage immoscout24 mobile search

- `immoscout24-pp-cli immoscout24-mobile-search list` — Retrieves listing cards for a mobile search query. Use a small pagesize and respect rate limits.
- `immoscout24-pp-cli immoscout24-mobile-search map` — Returns map markers and paging metadata for broad searches.
- `immoscout24-pp-cli immoscout24-mobile-search total` — Returns the total number of listings matching a mobile search query.

**translate-url** — Translate ImmoScout24 links and CLI search parameters.

- `immoscout24-pp-cli translate-url [url-or-id]` — Converts between ImmoScout24 web links, mobile API URLs, expose IDs, and CLI commands.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
immoscout24-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Check install health

```bash
immoscout24-pp-cli doctor --dry-run --agent
```

Run a no-network health check in agent-friendly mode.

### Search around Kriftel, Hattersheim, and Hofheim

```bash
immoscout24-pp-cli immoscout24-mobile-search list --search-type radius --geocoordinates '50.078;8.445;8' --realestatetype apartmentrent --pricetype calculatedtotalrent --price=-2500 --numberofrooms=3- --livingspace=100- --equipment=parking --fulltext modern --pagesize 20 --json --select results.resultListItems.item.id,results.resultListItems.item.title,results.resultListItems.item.attributes,results.resultListItems.item.web_url
```

Use this for a rental apartment search near Kriftel/Hattersheim/Hofheim with at least 3 rooms, at least 100 sqm, warm rent below 2500 EUR, and a fixed parking-space filter. Then inspect candidate exposes for kitchen layout, floorplans, and modern condition with: immoscout24-pp-cli expose inspect <id> --json --select results.web_url,results.text_count,results.image_count,results.floorplan_count,results.floorplan_document_count,results.texts.title,results.texts.items.value,results.images.render_url,results.floorplans.render_url,results.floorplan_documents.url

## Auth Setup

No authentication required.

Run `immoscout24-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  immoscout24-pp-cli expose inspect 168983602 --agent --max-texts 20 --select results.id,results.web_url,results.texts.title,results.texts.items.value
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

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `IMMOSCOUT24_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `IMMOSCOUT24_CONFIG_DIR`, `IMMOSCOUT24_DATA_DIR`, `IMMOSCOUT24_STATE_DIR`, `IMMOSCOUT24_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `IMMOSCOUT24_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `immoscout24-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "immoscout24": {
        "command": "immoscout24-pp-mcp",
        "env": {
          "IMMOSCOUT24_HOME": "/srv/immoscout24"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `IMMOSCOUT24_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `IMMOSCOUT24_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
immoscout24-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
immoscout24-pp-cli feedback --stdin < notes.txt
immoscout24-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `IMMOSCOUT24_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `IMMOSCOUT24_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
immoscout24-pp-cli profile save briefing --json
immoscout24-pp-cli --profile briefing expose 168983602
immoscout24-pp-cli profile list --json
immoscout24-pp-cli profile show briefing
immoscout24-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `immoscout24-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/immoscout24/cmd/immoscout24-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add immoscout24-pp-mcp -- immoscout24-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which immoscout24-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   immoscout24-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `immoscout24-pp-cli <command> --help`.
