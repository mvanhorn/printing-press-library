---
name: pp-drudgereport
description: "Drudge Report in your terminal, with the editorial signal (splash, red, slot, tenure) the live page broadcasts but... Trigger phrases: `what's on drudge`, `drudge splash`, `drudge breaking`, `what changed on drudge`, `drudge tenure`, `drudge sources this week`, `use drudgereport`, `run drudgereport`."
author: "Matt Van Horn"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - drudgereport-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/media-and-entertainment/drudgereport/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Drudge Report — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `drudgereport-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install drudgereport --cli-only
   ```
2. Verify: `drudgereport-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Read Drudge's lead, breaking-red items, and ranked headlines in one bounded JSON call. Keep a local SQLite snapshot history so you can answer 'what's changed since I checked,' 'how long has that been the splash,' and 'what was Drudge leading with on Tuesday' — none of which the live site supports.

## When to Use This CLI

Reach for this CLI when the task is 'what is Drudge featuring' or 'how has Drudge framed X over time.' It is read-only, the local SQLite store rewards repeated sync runs with deeper analytics, and the agent surface returns bounded JSON. Not the right tool for live breaking news outside Drudge's curation.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Editorial signal-aware reading
- **`splash`** — Just the current center-slot splash headline with image, outbound URL, red flag, and how long it has been on splash.

  _When an agent is asked 'what's on Drudge right now?' this is the one-shot bounded answer; no HTML soup to re-parse._

  ```bash
  drudgereport splash --json
  ```
- **`breaking`** — Every headline currently set in red by Drudge's editor, ordered by slot importance.

  _Tells an agent the difference between 'on Drudge' and 'Drudge thinks this is breaking,' a high-signal filter at zero token cost._

  ```bash
  drudgereport breaking --json
  ```
- **`headlines`** — All current headlines ranked by composite editorial weight (slot + red + image), not by chronology.

  _Returns Drudge in priority order with bounded payloads agents can pipe._

  ```bash
  drudgereport headlines --limit 10 --json --select title,slot,is_red,url
  ```

### Local state that compounds
- **`tail`** — Slot transitions and color changes between consecutive snapshots: promoted to splash, demoted from splash, went red, disappeared, appeared.

  _Lets an agent answer 'what changed on Drudge?' without re-fetching the page or diffing screenshots._

  ```bash
  drudgereport tail --since 6h --json
  ```
- **`tenure`** — How long the current splash has been the splash, plus the all-time longest-tenured splash leaderboard.

  _Distinguishes 'breaking right now' from 'Drudge wants this to stick,' a question agents and journalists both need._

  ```bash
  drudgereport tenure --history --json
  ```
- **`sources`** — Outbound-domain frequency leaderboard over a window with rising/falling delta vs the prior window, optionally crosstabbed by slot.

  _Surfaces editorial bent shifts week over week; high-leverage for media analysts and agents writing about narrative pickup._

  ```bash
  drudgereport sources --window 168h --by-slot --json
  ```
- **`on-date`** — Reconstruct Drudge at any past timestamp the CLI has observed: splash, red items, ranked headlines as of that moment.

  _Answers 'what was Drudge leading with when X happened?' — a question the live site cannot._

  ```bash
  drudgereport on-date 2026-04-15T08:30 --json
  ```
- **`bent`** — Ratio of red items by outbound domain over a window: which outlets Drudge tends to break vs which he tends to merely column.

  _Quantifies a previously gut-feel media-criticism observation; agents writing media-analysis copy can cite numbers._

  ```bash
  drudgereport bent --window 168h --json
  ```
- **`story`** — Every slot_event for one story_id ordered by timestamp: when it appeared, where it moved, when it went red, when it dropped, total tenure.

  _Lets an agent reconstruct one story's editorial life on Drudge, useful for retrospective newsletter analysis._

  ```bash
  drudgereport story abc123 --json
  ```

### Agent-native plumbing
- **`digest`** — One-pager: splash count, longest-tenured splash, top 5 outbound domains, biggest red-surge stories over the week.

  _Replaces the 'what did Drudge feature this week' graf that journalists currently assemble by hand._

  ```bash
  drudgereport digest --week --json
  ```

## Command Reference

**feed** — Unofficial community RSS feed mirror of Drudge Report (feedpress.me). Used as a cross-check source for pubDate and pre-grouped Related stories.

- `drudgereport-pp-cli feed` — Fetch the unofficial RSS feed mirror. Items embed position labels (Main headline / First column / Second column) in...

**page** — Drudge Report's curated home page. Most users should run `sync` then `splash`/`headlines`/`breaking`; the raw fetch is exposed for debugging.

- `drudgereport-pp-cli page` — Fetch the raw drudgereport.com HTML page. The CLI's parser turns this into ranked headlines with slot, is_red,...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
drudgereport-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Just the splash, for a daily briefing

```bash
drudgereport splash --json --select title,url,image_url,is_red,splash_tenure_seconds
```

Bounded payload with every signal an agent needs to summarize Drudge's lead in one sentence.

### What's red right now

```bash
drudgereport breaking --json --select title,url,slot
```

Every red headline with its slot; lets the agent distinguish splash-red from column-red without re-parsing the page.

### Top 10 ranked headlines with select-narrowing

```bash
drudgereport headlines --limit 10 --json --select title,slot,is_red,url,outbound_domain
```

Returns the priority-ordered list of stories in agent-friendly shape with only the fields needed for downstream framing.

### What changed in the last 6 hours

```bash
drudgereport tail --since 6h --json
```

Slot_event rows for promotions, demotions, color changes, and disappearances; cheaper than diffing two fetches.

### Which outlets Drudge leaned on this week

```bash
drudgereport sources --window 7d --by-slot --json
```

Outbound-domain leaderboard split by slot (splash vs red vs column) over the trailing 7 days, with rising/falling delta vs the prior 7 days.

## Auth Setup

No authentication required.

Run `drudgereport-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  drudgereport-pp-cli feed --agent --select id,name,status
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
drudgereport-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
drudgereport-pp-cli feedback --stdin < notes.txt
drudgereport-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.drudgereport-pp-cli/feedback.jsonl`. They are never POSTed unless `DRUDGEREPORT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `DRUDGEREPORT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
drudgereport-pp-cli profile save briefing --json
drudgereport-pp-cli --profile briefing feed
drudgereport-pp-cli profile list --json
drudgereport-pp-cli profile show briefing
drudgereport-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `drudgereport-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add drudgereport-pp-mcp -- drudgereport-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which drudgereport-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   drudgereport-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `drudgereport-pp-cli <command> --help`.
