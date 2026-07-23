---
name: pp-art-goat
description: "OmniMuseum aggregator for a daily contemplative art practice — eight open-access museum and astronomy APIs as one local corpus. Trigger phrases: `sit with a piece of art`, `today's piece of art`, `what's APOD`, `browse art collections`, `museum aggregator`, `open access art practice`, `8-source contemplative tool`, `art journal stats`, `walk a theme across museums`, `journal compare sits`, `revisit a past sit`, `bridge from last sit mood`, `artist arc across sources`, `use art-goat`, `run art-goat`."
author: "justinwfu"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - art-goat-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/media-and-entertainment/art-goat/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# art-goat — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `art-goat-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install art-goat --cli-only
   ```
2. Verify: `art-goat-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/cmd/art-goat-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use art-goat when you want a daily contemplative practice grounded in real museum and astronomy collections. It's the right choice when one piece of extended attention is more useful than skimming a feed of fifty. It's not the right choice for shopping for art, building a catalog, or doing research that needs the museum's full canonical metadata in raw form — for that, use the AIC API directly.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Contemplative practice
- **`sit`** — Sit with one piece of art for a fixed period. Opens the image in your browser, shows the curator's description, runs a quiet timer, and captures your reflection in the journal.

  _Use this when you want to spend deliberate time with one piece instead of skimming a feed. The reflection is persisted locally for later search._

  ```bash
  art-goat-pp-cli sit --duration 10 --dry-run
  ```
- **`today`** — Today's curated piece, chosen using anti-repeat against your sits and journal-aware diversity against your recent medium and culture choices. Includes a one-line 'why this today'.

  _Use today as the default daily entry point. It avoids showing you what you already sat with and rotates against your recent medium and region choices._

  ```bash
  art-goat-pp-cli today
  ```

### Practice journal
- **`journal stats`** — Practice statistics emphasising source breadth, medium variety, region coverage, and mood drift, with streak available at the bottom labeled 'if you want to know'.

  _Use this to see breadth instead of streak length. Practice quality is captured as how widely you've ranged, not how many days in a row you've sat._

  ```bash
  art-goat-pp-cli journal stats
  ```
- **`journal search`** — FTS5 over your reflection history. Surfaces every past sit whose text matches the query.

  _Use this to find prior reflections by token. Years of practice become queryable._

  ```bash
  art-goat-pp-cli journal search "solitude"
  ```

## Command Reference

**planetary** — Manage planetary

- `art-goat-pp-cli planetary` — Fetch the Astronomy Picture of the Day for a given date or date range. Anonymous via DEMO_KEY.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
art-goat-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Start a daily practice

```bash
art-goat-pp-cli sources sync && art-goat-pp-cli today
```

Populate the store, then enter today's curated practice in a single line.

### Sit with a specific piece

```bash
art-goat-pp-cli sit aic:24645 --duration 10
```

Pull AIC's Hokusai 'Under the Wave off Kanagawa' for a ten-minute sit.

### Look back on reflections

```bash
art-goat-pp-cli journal search "water" --json --select started_at,work_id,reflection
```

Find every past sit where you wrote about water. JSON output piped through select keeps the response narrow.

### Step laterally across sources

```bash
art-goat-pp-cli similar aic:24645 --json --select source,title,creator
```

Surface other works in the corpus that share medium, period, region, or creator with a seed. Useful for letting one piece point you to neighboring ones across museums.

### Find all works by an artist

```bash
art-goat-pp-cli artist "van gogh" --json --select id,title,date
```

Federated substring match on the canonical creator field, chronological.

### Read a creator's career arc across museums

```bash
art-goat-pp-cli artist "hokusai" --arc
```

Groups works into stylistic periods using `date_start` + the `period` field where available and renders a 5-line career narrative. Plays to the federated corpus — pulling a creator across museums gives a richer arc than any one museum's holdings.

### Walk a theme through the corpus

```bash
art-goat-pp-cli path --theme "impermanence"
art-goat-pp-cli path --theme "stillness" --steps 7 --json
```

FTS5 search across title/medium/period/region/creator/description, returned as a diversity-ordered walk. `--steps` controls walk length (default 5). Print only — no journal writes.

### Bridge from your last sit's mood

```bash
art-goat-pp-cli today --mode bridge-from-last
```

Reads the last sit's mood (1–5 scale) from the journal and picks toward the opposite half — heavy → calmer, calm → energizing. Falls back to default rotation if no prior sit. Useful for a contemplative practice that rotates energy across days, not just visual variety.

### Compare two sits

```bash
art-goat-pp-cli journal compare 12 47
```

Side-by-side: each sit's work, prompt, reflection, mood, and tags. Turns the journal from a flat list into a longitudinal log.

### Revisit a past sit

```bash
art-goat-pp-cli journal revisit --age 1y
art-goat-pp-cli journal revisit --age 6mo --json
```

Surfaces the sit closest to `today - <age>` within a ±7 day window. Supports `Nd`, `Nw`, `Nmo`, `Ny`. "What was I noticing this day a year ago" — a tiny addition with outsized practice value.

### Quick contemplative pulse without a timer

```bash
art-goat-pp-cli presence --source met
```

Random piece + prompt, no timer, no journal write. Drop-in for a 30-second pause without committing to a full sit.

### Export practice to Markdown

```bash
ART_GOAT_JOURNAL_PATH=~/Obsidian/art-goat art-goat-pp-cli journal export
```

Mirror the journal to your Obsidian vault. SQLite stays canonical for queries; the Markdown files are tool-agnostic.

## Auth Setup

Anonymous out of the box for `aic`, `met`, `cleveland`, and `npmtw`. NASA APOD and Smithsonian Open Access work via DEMO_KEY (~30 req/hr ceiling); upgrade with `NASA_API_KEY` / `SMITHSONIAN_API_KEY` / `ART_GOAT_API_KEY`. Rijksmuseum and Te Papa require free signup keys: `RIJKSMUSEUM_API_KEY` (or `ART_GOAT_RIJKS_KEY`) and `TEPAPA_API_KEY` (or `ART_GOAT_TEPAPA_KEY`). The CLI surfaces the exact env var in its error when a key is missing.

Run `art-goat-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  art-goat-pp-cli planetary --agent --select id,name,status
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
art-goat-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
art-goat-pp-cli feedback --stdin < notes.txt
art-goat-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.art-goat-pp-cli/feedback.jsonl`. They are never POSTed unless `ART_GOAT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ART_GOAT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
art-goat-pp-cli profile save briefing --json
art-goat-pp-cli --profile briefing planetary
art-goat-pp-cli profile list --json
art-goat-pp-cli profile show briefing
art-goat-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `art-goat-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add art-goat-pp-mcp -- art-goat-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which art-goat-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   art-goat-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `art-goat-pp-cli <command> --help`.
