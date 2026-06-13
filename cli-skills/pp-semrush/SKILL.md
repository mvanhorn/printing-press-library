---
name: pp-semrush
description: "Semrush CLI with Domain Overview, organic keyword research, and backlink intelligence — JSON-first, agent-native, local SQLite cache for offline use"
author: "australian-digital"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - semrush-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/marketing/semrush/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Semrush — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `semrush-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install semrush --cli-only
   ```
2. Verify: `semrush-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Semrush CLI — Domain Overview, organic keyword research, and backlink intelligence via the Semrush public API

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Compounded keyword intelligence
- **`research`** — Run a full keyword research pass — seeds expand via the Keyword Magic Tool, filter by Personal Keyword Difficulty against your client domain, score by intent + volume + opportunity, dedupe across seeds, and emit a ready-for-Sheets ranked list.

  _When an agent needs to assemble a keyword shortlist for content briefing, this returns a scored, deduped list in one call instead of 5+ chained KMT requests._

  ```bash
  semrush-pp-cli research --seeds "ethical investing,green super" --domain client.com --database au --agent
  ```
- **`keyword-magic`** — Run the Keyword Magic Tool against a seed and get Personal Keyword Difficulty (PKD) scored against your target domain. PKD is not exposed by the public Analytics API.

  _Agents researching content gaps for a specific client need PKD, not the generic Keyword Difficulty the public API returns._

  ```bash
  semrush-pp-cli keyword-magic "seo audit" --domain client.com --database us --mode broad
  ```

### Agent-native delivery
- **`sheets`** — Push Semrush data (keyword research, rankings, gap analysis) directly into your existing Google Sheets client template — no copy/paste, no CSV roundtrip.

  _When an agent runs research and the deliverable is a populated client tracker, this skips the manual hand-off entirely._

  ```bash
  cat research-output.json | semrush-pp-cli sheets push 1AbC...xyz --tab "Keyword Research"
  ```

### Position Tracking automation
- **`pt add-keywords`** — Add one or more keywords to a Semrush Position Tracking campaign, optionally tagging them so they show up grouped in the PT UI. The public Semrush API does not expose Position Tracking write endpoints.

  _When an agent generates a content brief, it can drop the target keywords directly into PT — no manual paste-into-UI step required._

  ```bash
  semrush-pp-cli pt add-keywords 24453414_2945960 --keywords "tax deductible donations,eofy charity" --tags "#articles,eofy-campaign"
  ```
- **`pt rankings`** — Returns the current organic rankings snapshot for every tracked keyword in a Position Tracking campaign — position, position diff, volume, SERP features, traffic estimate. Different from `pt report` which returns rank history.

  _Agents reporting on weekly client performance can fetch the full Overview tab data in one call instead of paginating the rate-limited public Analytics endpoints._

  ```bash
  semrush-pp-cli pt rankings 24453414_2945960 --domain client.com --agent --select data.keywords.keyword,data.keywords.position,data.keywords.volume
  ```
- **`pt annotate`** — Add an annotation (note) to a Position Tracking campaign — appears on the PT chart at the specified date. Lets you correlate site changes (publishes, redirects, algorithm updates) with ranking movements.

  _Agents that schedule publishes can drop the annotation directly into PT, so 8 weeks later the ranking change is clearly tied to the content move._

  ```bash
  semrush-pp-cli pt annotate 24453414_2945960 --title "New EOFY cluster published" --note "3 articles + 1 refreshed page" --date 2026-06-13
  ```

## Command Reference

**backlinks** — Backlink data for a target domain or URL

- `semrush-pp-cli backlinks <target>` — Backlink count, referring domains, Authority Score

**domain** — Domain Overview — organic traffic, top keywords, top pages, competitors

- `semrush-pp-cli domain organic_competitors` — Domains ranking for the same keywords as the target — organic competitors
- `semrush-pp-cli domain organic_keywords` — Keywords a domain ranks for in organic Google search results
- `semrush-pp-cli domain organic_pages` — Top organic landing pages and their ranking keywords for a domain
- `semrush-pp-cli domain overview` — Domain overview (organic + paid traffic, keyword count, traffic cost) for a target domain

**keyword** — Keyword research — overview, related, questions, difficulty

- `semrush-pp-cli keyword difficulty` — Keyword difficulty (KD%) — chance of ranking on first SERP page
- `semrush-pp-cli keyword overview` — Volume, CPC, KD, competition for a single keyword
- `semrush-pp-cli keyword questions` — Question-style keywords containing the seed phrase (great for content SEO)
- `semrush-pp-cli keyword related` — Related keywords (semantic variants) for a seed phrase


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
semrush-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup
Run `semrush-pp-cli auth setup` to print the URL and steps for getting a key (add `--launch` to open the URL). Then set:

```bash
export SEMRUSH_API_KEY="<your-key>"
```

Or persist it in `~/.config/semrush-pp-cli/config.toml`.

Run `semrush-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  semrush-pp-cli backlinks mock-value --type example-value --agent --select id,name,status
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
semrush-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
semrush-pp-cli feedback --stdin < notes.txt
semrush-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.semrush-pp-cli/feedback.jsonl`. They are never POSTed unless `SEMRUSH_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SEMRUSH_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
semrush-pp-cli profile save briefing --json
semrush-pp-cli --profile briefing backlinks mock-value --type example-value
semrush-pp-cli profile list --json
semrush-pp-cli profile show briefing
semrush-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `semrush-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add semrush-pp-mcp -- semrush-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which semrush-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   semrush-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `semrush-pp-cli <command> --help`.
