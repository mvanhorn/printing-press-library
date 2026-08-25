---
name: pp-dreaming
description: "Every Dreaming Spanish and French tracking feature in one offline-first, agent-native CLI: roadmap math, transcript concordance, and bulk hour-logging. Trigger phrases: `what should I watch next on dreaming`, `log my external spanish hours`, `how many hours until I can speak spanish`, `find examples of the subjunctive in dreaming videos`, `search dreaming transcripts for a word`, `am I on pace for 600 hours`, `use dreaming`, `run dreaming`."
author: "Paul Bockewitz"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - dreaming-pp-cli
---

# Dreaming — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `dreaming-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install dreaming --cli-only
   ```
2. Verify: `dreaming-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Dreaming's web app shows your hours but won't plan, search, or bulk-import for you, and the community's features are scattered across a dozen browser extensions and scripts. This CLI unifies them into one agent-native tool backed by a local SQLite mirror of your catalog, daily series, and external-hours log - so `next` picks your next video offline, `external import` clears a CSV backlog in one shot, and `roadmap` lays out the whole L1-L7 fluency ladder with personalized ETAs.

## When to Use This CLI

Use the Dreaming CLI for any task about a learner's comprehensible-input progress on Dreaming Spanish or French: checking hours and pace toward a fluency level, picking the next right-level video, searching the catalog by topic/guide/dialect, logging or bulk-importing external listening time, and analyzing whether consumed difficulty is trending up. It is the right tool whenever the question involves the hours-based roadmap or the leveled video catalog.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Roadmap intelligence
- **`next`** — Get the next unwatched videos tuned to where you are on the fluency roadmap, sorted by fine-grained difficulty.

  _Reach for this instead of scrolling the catalog when an agent needs the single right-level, not-yet-watched video to recommend._

  ```bash
  dreaming next --limit 5 --agent
  ```
- **`diet`** — See whether the difficulty of what you actually watch is trending up over a window - the real signal that you're ready to level up.

  _Use this to answer 'is this learner actually progressing, or stuck rewatching easy content?'_

  ```bash
  dreaming diet --window 90d --agent
  ```
- **`roadmap`** — The whole L1-L7 ladder with comprehensible-input advisories (speaking, reading, native milestones) and personalized calendar ETAs in one view.

  _Reach for this when the question is 'where am I on the path to fluency and when do I hit the next milestone?'_

  ```bash
  dreaming roadmap --agent
  ```

### External-hours logging
- **`external import`** — Import a backlog of outside listening/watching hours from a CSV in one command instead of dozens of web-form clicks.

  _Use this whenever a learner has weeks of un-logged podcast/Netflix time to reconcile in bulk._

  ```bash
  dreaming external import backlog.csv --preview
  ```

### Local state that compounds
- **`transcript`** — Fetch a video's captions as clean, timestamp-free plain text (and cache the cue-level transcript).

  _Pick this when an agent needs to read or summarize what a single video actually says rather than just its title and tags._

  ```bash
  dreaming transcript 5f3a1b2c4d5e6f7a8b9c0d1e --agent
  ```
- **`concordance`** — Search every cached transcript for a word, phrase, regex, or verb-tense pattern and get each hit in context with the video, level, guide, and timestamp.

  _Reach for this to find real, level-appropriate examples of a word or grammar form in context - the core ritual of grammar-in-context study._

  ```bash
  dreaming concordance --tense subjunctive-imperfect --level intermediate --agent
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**day-watched-time** — Manage day watched time

- `dreaming-pp-cli day-watched-time` — List per-day on-platform watch time

**external-time** — Manage external time

- `dreaming-pp-cli external-time add` — Log an external input-time entry
- `dreaming-pp-cli external-time delete` — Delete an external input-time entry by id
- `dreaming-pp-cli external-time list` — List your external (outside-platform) input-time log entries

**inspect-external-video** — Manage inspect external video

- `dreaming-pp-cli inspect-external-video` — Resolve a YouTube URL for external-time logging

**playlist** — Manage playlist

- `dreaming-pp-cli playlist` — List your watched-video playlist

**sign-in** — Manage sign in

- `dreaming-pp-cli sign-in` — Exchange email/password for a bearer token

**user** — Manage user

- `dreaming-pp-cli user` — Get your account stats (hours, daily goal, subscription)

**video** — Manage video

- `dreaming-pp-cli video` — Get a single video's metadata, captions, and stream sources

**videos** — Manage videos

- `dreaming-pp-cli videos` — List the video catalog


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
dreaming-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Plan to a deadline

```bash
dreaming plan --target 600h --by 2026-12-31
```

Back-solves the daily minutes you need to reach 600 hours (L5, speaking recommended) by a date.

### Find the next video, agent-friendly

```bash
dreaming next --limit 5 --agent
```

Returns the next unwatched, level-appropriate videos as structured JSON for an agent to recommend.

### Narrow a large catalog payload

```bash
dreaming next --level beginner --dialect rioplatense --agent --select title,guide,difficulty
```

The picker returns verbose video records; --select trims each to just the fields an agent needs.

### Break down watched hours by teacher

```bash
dreaming stats by-guide --agent
```

Aggregates your watched videos per guide from the local store as structured JSON.

### Check readiness to level up

```bash
dreaming diet --window 90d --agent
```

Shows whether the difficulty of what you actually watched is climbing over the last 90 days.

### Read a video's transcript

```bash
dreaming transcript 5f3a1b2c4d5e6f7a8b9c0d1e
```

Fetches the inline captions and prints clean, timestamp-free text an agent can read or summarize.

### Resolve a downloadable stream

```bash
dreaming download 5f3a1b2c4d5e6f7a8b9c0d1e
```

Prints the signed HLS URL and a ready ffmpeg command; add --exec to download when ffmpeg is installed.

### Find a verb tense in real content

```bash
dreaming concordance --tense subjunctive-imperfect --level beginner
```

After 'transcript sync', returns level-appropriate sentences using the imperfect subjunctive, each with its video and timestamp.

### Concordance a specific word

```bash
dreaming concordance "entonces" --agent --select context,video_title,timestamp
```

Keyword-in-context hits for a word across the whole corpus as structured JSON for an agent.

## Auth Setup

Dreaming has no official API. Authenticate with your account bearer token: copy it from your browser's localStorage 'token' key on app.dreaming.com (run `dreaming auth setup` for the steps), then run `dreaming auth set-token <token>` or set DREAMING_TOKEN. You can also exchange email/password for a token with `sign-in`. Tokens expire periodically - re-run `auth set-token` (or re-export DREAMING_TOKEN) when you hit a 401.

Run `dreaming-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  dreaming-pp-cli day-watched-time --agent --select id,name,status
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
dreaming-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
dreaming-pp-cli feedback --stdin < notes.txt
dreaming-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/dreaming-pp-cli/feedback.jsonl`. They are never POSTed unless `DREAMING_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `DREAMING_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
dreaming-pp-cli profile save briefing --json
dreaming-pp-cli --profile briefing day-watched-time
dreaming-pp-cli profile list --json
dreaming-pp-cli profile show briefing
dreaming-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `dreaming-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add dreaming-pp-mcp -- dreaming-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which dreaming-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   dreaming-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `dreaming-pp-cli <command> --help`.
