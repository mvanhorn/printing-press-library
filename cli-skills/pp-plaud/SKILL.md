---
name: pp-plaud
description: "Every conversation, queryable — your transcripts as a searchable history, not a feed. Trigger phrases: `what did I promise this week`, `what did Sandra say about renewal`, `what topics are coming up in my meetings`, `what commitments have I forgotten`, `who haven't I talked to lately`, `search my conversations`, `my plaud recordings`, `use plaud-pp-cli`, `run plaud-pp-cli`."
author: "jnalv414"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - plaud-pp-cli
    install:
      - kind: go
        bins: [plaud-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/productivity/plaud/cmd/plaud-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/productivity/plaud/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# Plaud — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `plaud-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install plaud --cli-only
   ```
2. Verify: `plaud-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/plaud/cmd/plaud-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Plaud captures every meeting with speaker diarization. plaud-pp-cli mirrors that history to a local SQLite store with FTS5, then asks the questions the app cannot: what did I promise across all my recordings, who said what about a topic over time, which commitments did I drop, who has gone quiet. A raw `sql` escape hatch lives at the bottom for anything we didn't anticipate.

## When to Use This CLI

Reach for plaud-pp-cli when the user wants to query their conversation history as data: "what did I promise across all meetings", "who said X about Y over time", "have I been consistent". Don't reach for it for one-recording-at-a-time consumption — Plaud's app or the official CLI are fine for that. The transcendence commands (commitments, topic, about, forgotten, themes, cross-meeting, silence, mentioned-me) only work after `sync` has populated the local SQLite mirror.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Memory of what you said

- **`commitments`** — Surface every promise you made — "I'll send", "let me follow up", "by EOW" — across every recording, grouped by who you said it to, with the date and recording link.

  _When the user asks "what did I promise people this week?", reach for this before opening any individual recording. The answer is a list, not a transcript._

  ```bash
  plaud-pp-cli commitments --since 30d --open --by-person --agent
  ```
- **`about`** — Every utterance by one named speaker, across every recording, with ±1 segments of surrounding context. Optional topic filter for laser-focus.

  _Pre-call prep: pull everything a person said about a topic in one read. Beats scrubbing recordings at 2x speed._

  ```bash
  plaud-pp-cli about 'Sandra' --topic 'renewal' --since 90d --agent
  ```
- **`forgotten`** — Commitments you made that have no follow-up signal in any later recording. The set difference between things-you-said-you-would-do and things-you-actually-talked-about-again.

  _Surface the broken promises before the other person does. There is no other tool that knows what you said you'd do AND whether you followed through._

  ```bash
  plaud-pp-cli forgotten --since 90d --by-person --agent
  ```
- **`mentioned-me`** — Every time someone OTHER than you said your name in a recording. Pulls from the cached /user/me name and FTS5-filtered out your own segments.

  _Third-party mentions of the user — who's talking about them and what they're saying. No other Plaud tool surfaces this._

  ```bash
  plaud-pp-cli mentioned-me --since 90d --agent
  ```

### Patterns across conversations

- **`topic`** — Trace how often a topic comes up across your recordings over time. Bucketed by week or day. Shows emerging vs decaying signal with the speakers anchoring each bucket.

  _When the user is wondering whether a concern is growing or fading, this command answers it in one query instead of opening 20 summaries._

  ```bash
  plaud-pp-cli topic 'pricing' --since 90d --bucket week --agent
  ```
- **`themes`** — Top n-gram shingles in recent recordings vs the prior equivalent window. Frequency-based — no LLM. Output shows what's new this period, what's vanished, what's steady.

  _Monthly reviews without reading anything. The deltas tell you where your attention is shifting._

  ```bash
  plaud-pp-cli themes --last 30d --against 30d-prior --agent
  ```
- **`cross-meeting`** — Every utterance by one person about one topic, ordered chronologically across meetings, with prior + next segment as context. Read it to see whether their (or your) position has drifted.

  _When you need to know whether a stakeholder has been saying the same thing over time. Drift detection without rereading every meeting._

  ```bash
  plaud-pp-cli cross-meeting 'Marcus' 'launch date' --agent
  ```
- **`silence`** — People who used to appear in your recordings but haven't in N days. Last recording link + last topic mentioned. Surfaces relationships going cold.

  _For managers and operators: which reports, customers, or stakeholders have gone quiet? The recording you need is the absence of one._

  ```bash
  plaud-pp-cli silence --days 21 --agent
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**ai** — AI subsystem

- `plaud-pp-cli ai status` — Check Plaud AI subsystem reachability and current operational state. Useful as a lightweight smoke test before...
- `plaud-pp-cli ai transsumm` — Combined transcript + AI summary (newer recordings)

**filetags** — Folders/tags for recordings

- `plaud-pp-cli filetags` — List the user's file tags (folder structure)

**recordings** — Voice recordings

- `plaud-pp-cli recordings audio-url` — Get pre-signed audio download URL (24-hour TTL)
- `plaud-pp-cli recordings delete` — Permanently delete recordings (batch, irreversible)
- `plaud-pp-cli recordings export` — Export transcript or summary as DOCX/PDF/TXT/MD
- `plaud-pp-cli recordings get` — Get full recording detail including content_list
- `plaud-pp-cli recordings list` — List recordings (after sync, served from local store)
- `plaud-pp-cli recordings list-by-ids` — Fetch specific recordings by ID list (returns trans_result + ai_content)
- `plaud-pp-cli recordings share` — Create a shareable public link for the recording
- `plaud-pp-cli recordings trash` — Move recordings to trash (batch)
- `plaud-pp-cli recordings untrash` — Restore recordings from trash (batch)
- `plaud-pp-cli recordings upload-info` — Pre-flight telemetry call (used internally before exports)

**users** — Plaud account

- `plaud-pp-cli users` — Get the authenticated user (cached after sync)


**Hand-written commands**

- `plaud-pp-cli auth` — Authenticate with Plaud (email+password → JWT)
- `plaud-pp-cli search` — Full-text search across transcripts (FTS5 over local store)
- `plaud-pp-cli sync` — Sync recordings, transcripts, and summaries to local SQLite
- `plaud-pp-cli sql` — Run a read-only SQL query against the local store
- `plaud-pp-cli doctor` — Verify JWT, region, /user/me reachability, store schema
- `plaud-pp-cli commitments` — Surface every promise across all transcripts (regex over speaker-diarized content)
- `plaud-pp-cli topic` — Trace how often a topic comes up over time (bucketed mention counts)
- `plaud-pp-cli about` — Every utterance by one named speaker, optionally filtered by topic
- `plaud-pp-cli forgotten` — Commitments you made with no follow-up signal in any later recording
- `plaud-pp-cli themes` — N-gram frequency deltas between two time windows (emerging vs decaying)
- `plaud-pp-cli cross-meeting` — Every utterance by a person about a topic, ordered chronologically (drift detection)
- `plaud-pp-cli silence` — Speakers who used to appear but haven't in N days (last recording + last topic)
- `plaud-pp-cli mentioned-me` — Third-party mentions of the authenticated user across transcripts
- `plaud-pp-cli speakers` — List speakers across the corpus with appearance counts and last-heard timestamps
- `plaud-pp-cli export` — Bulk export to Obsidian, plain Markdown, or JSON


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
plaud-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Sunday-night planning

```bash
plaud-pp-cli sync && plaud-pp-cli commitments --since 7d --open --agent --select speaker,commitment,recording_id,start_time
```

Sync first, then pull every open commitment from the last week. `--select` narrows the response to what the agent needs — plaud transcripts can be huge.

### Pre-call prep for a renewal

```bash
plaud-pp-cli about 'Sandra' --topic 'pricing' --since 90d --agent
```

Every utterance by Sandra mentioning pricing across the last quarter of meetings. Faster than scrubbing recordings.

### Monthly thematic review

```bash
plaud-pp-cli themes --last 30d --against 30d-prior --agent
```

What's growing and what's fading in your recent conversations. Frequency-based, no LLM call — runs entirely on local data.

### Who has gone quiet

```bash
plaud-pp-cli silence --days 21 --agent
```

Speakers who appeared in past recordings but not in the last 21 days. For managers and operators tracking relationships.

### Cross-meeting drift check

```bash
plaud-pp-cli cross-meeting 'Marcus' 'launch date' --agent
```

Every utterance from Marcus mentioning launch date, ordered chronologically. Read the rows top to bottom — that's the drift.

### Raw SQL when typed commands fall short

```bash
plaud-pp-cli sql 'SELECT speaker, COUNT(*) AS n FROM transcripts GROUP BY speaker ORDER BY n DESC LIMIT 10' --json
```

SELECT-only escape hatch. FTS5 MATCH is supported. The store schema is documented in the README.

## Auth Setup

Auth uses Plaud's own user-app endpoint (POST /auth/access-token, email + password) — same as the community libraries. Tokens last ~300 days; the CLI silently re-logs in 30 days before expiry. For a no-password path, `auth login --chrome` lifts the JWT directly from your logged-in browser. Region (us / eu / ap) auto-detects on the first call.

Run `plaud-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  plaud-pp-cli filetags --agent --select id,name,status
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
plaud-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
plaud-pp-cli feedback --stdin < notes.txt
plaud-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.plaud-pp-cli/feedback.jsonl`. They are never POSTed unless `PLAUD_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `PLAUD_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
plaud-pp-cli profile save briefing --json
plaud-pp-cli --profile briefing filetags
plaud-pp-cli profile list --json
plaud-pp-cli profile show briefing
plaud-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `plaud-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/plaud/cmd/plaud-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add plaud-pp-mcp -- plaud-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which plaud-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   plaud-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `plaud-pp-cli <command> --help`.
