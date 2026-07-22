---
name: pp-techtwitter
description: "Every Tech Twitter read endpoint, plus an offline-searchable local mirror and cited evidence bundles no other tool has. Trigger phrases: `what changed in tech twitter`, `what's trending in tech`, `give me a tech digest`, `what narrative is emerging in AI`, `search tech twitter for`, `use techtwitter`, `run techtwitter`."
author: "danielkhunter"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - techtwitter-pp-cli
    install:
      - kind: go
        bins: [techtwitter-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/social-and-messaging/techtwitter/cmd/techtwitter-pp-cli
---

# Tech Twitter — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `techtwitter-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install techtwitter --cli-only
   ```
2. Verify: `techtwitter-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/techtwitter/cmd/techtwitter-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Tech Twitter curates, dedupes, summarizes, and quality-scores the high-signal slice of tech X. This CLI turns that into a local SQLite mirror you can full-text search offline, and adds commands that only a compounding store can answer: what changed since your last sync, topic momentum over time, emerging narratives, time travel to any past day, and offline cited evidence bundles for agents.

## When to Use This CLI

Use this CLI when an agent or user needs cited, structured evidence from Tech Twitter's curated tech-discourse corpus, when you want offline full-text search over a deduped quality-scored stream instead of the raw X firehose, or when the question is about change over time (what's new, what's gaining momentum, what narrative is emerging) that a single stateless API call cannot answer.

## Anti-triggers

Do not use this CLI for:
- Posting, liking, bookmarking, or otherwise writing to X/Twitter — this CLI is read-only over Tech Twitter's curated corpus, not an X client.
- Fetching arbitrary tweets straight from X — only tweets Tech Twitter has curated are available.
- Account, billing, sponsor, or submission flows on techtwitter.com — those are auth-gated and out of scope.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`since`** — See only the tweets curated or newly hot since your last sync, instead of re-reading the whole stream.

  _Reach for this to answer "what did I miss" without re-fetching or re-scanning the full corpus._

  ```bash
  techtwitter-pp-cli since 24h --agent
  ```
- **`momentum`** — Show which topics are rising, falling, or newly appearing across the heatmap snapshots stored on each sync.

  _Use this when the question is about change over time, which no single API call can answer._

  ```bash
  techtwitter-pp-cli momentum --window 7d --json
  ```
- **`narrative`** — Surface keywords that newly emerged or accelerated versus prior snapshots, grounded in supporting stored tweets.

  _Pick this over a one-shot narrative pull when you want what is *newly* emerging, not just what is currently large._

  ```bash
  techtwitter-pp-cli narrative --json
  ```
- **`author-rank`** — Rank authors by accumulated stored engagement over a window, each with their best curated tweet.

  _Use this for a leaderboard that compounds as you sync, not the API's one-shot day winner._

  ```bash
  techtwitter-pp-cli author-rank --window 7d --limit 10 --json
  ```
- **`time-travel`** — Show the curated tweets for a specific date (YYYY-MM-DD, today, yesterday, or latest), live or fully offline from the local store.

  _Use this to pull the snapshot of what tech Twitter was saying on a given day, with no live call once synced._

  ```bash
  techtwitter-pp-cli time-travel 2026-06-07 --limit 10 --json
  ```

### Agent-native evidence, offline
- **`digest`** — Assemble a read-list from the local store for a window: top tweets, recent articles, and top authors.

  _Reach for this to hand an agent a cited, single-shot "what's happening in tech" rollup with zero extra calls._

  ```bash
  techtwitter-pp-cli digest --window 24h --agent
  ```
- **`evidence`** — Build an evidence bundle mirroring the agent-context kinds from local SQLite, with canonical-URL citations and no network.

  _Use this when an agent needs cited evidence but you want it grounded in the local mirror with no upstream call._

  ```bash
  techtwitter-pp-cli evidence read-list --agent --select evidence.title,evidence.canonicalUrl
  ```

## Command Reference

**agent** — Machine-agent evidence bundles

- `techtwitter-pp-cli agent` — Cited evidence bundle for an agent question

**articles** — Long-form articles backed by tweet provenance

- `techtwitter-pp-cli articles` — List long-form articles

**command** — Command-center signals (hot takes, main character, heatmap, stats, by-date)

- `techtwitter-pp-cli command heatmap` — Topic momentum heatmap (keyword, count, engagement)
- `techtwitter-pp-cli command hot-takes` — High-reply, debate-heavy curated tweets
- `techtwitter-pp-cli command main-character` — The day's most-engaged authors (main character)
- `techtwitter-pp-cli command stats` — Corpus index health stats
- `techtwitter-pp-cli command tweets-by-date` — Curated tweets for a date (today, yesterday, latest, or YYYY-MM-DD)

**newsletters** — In-app newsletters (Threads)

- `techtwitter-pp-cli newsletters` — List published in-app newsletters

**profiles** — Curated author profiles

- `techtwitter-pp-cli profiles` — Search author profiles by name, handle, or bio

**tweets** — Search and browse the curated tweet stream

- `techtwitter-pp-cli tweets author` — Curated tweets for a specific author handle
- `techtwitter-pp-cli tweets get` — Get a single curated tweet by UUID (with prev/next neighbors)
- `techtwitter-pp-cli tweets latest` — Get the single most recent curated tweet (with nextTweetId)
- `techtwitter-pp-cli tweets monthly` — Curated tweets for a given month
- `techtwitter-pp-cli tweets search` — Search curated tweets by text (relevance-ranked)
- `techtwitter-pp-cli tweets topic` — Curated tweets for a topic slug
- `techtwitter-pp-cli tweets trending` — Ranked trending curated tweets from the last 7 days


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
techtwitter-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Cited what-changed evidence for an agent

```bash
techtwitter-pp-cli agent --kind what-changed --agent --select evidence.title,evidence.canonicalUrl,evidence.qualityScore
```

Pulls a cited evidence bundle and narrows the deeply nested response to just title, citation URL, and quality score so an agent isn't flooded with media and metric fields.

### Daily tech standup, offline

```bash
techtwitter-pp-cli digest --window 24h --agent
```

Composes a read-list of top tweets, recent articles, and top authors for the last day from the local mirror, agent-formatted with citations.

### What is newly emerging

```bash
techtwitter-pp-cli narrative --json
```

Diffs stored heatmap snapshots to surface keywords that just emerged or accelerated, each grounded in supporting stored tweets.

### Search the curated corpus offline

```bash
techtwitter-pp-cli search "agentic coding" --json --limit 10
```

Full-text searches the synced local store with no network call, returning quality-scored curated tweets as JSON.

### Track a topic's momentum

```bash
techtwitter-pp-cli momentum --window 7d --json
```

Shows which topics are rising, falling, or newly appearing across the snapshots stored over the last week of syncs.

## Auth Setup

No authentication required.

Run `techtwitter-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  techtwitter-pp-cli articles --agent --select id,name,status
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
techtwitter-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
techtwitter-pp-cli feedback --stdin < notes.txt
techtwitter-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/techtwitter-pp-cli/feedback.jsonl`. They are never POSTed unless `TECHTWITTER_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `TECHTWITTER_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
techtwitter-pp-cli profile save briefing --json
techtwitter-pp-cli --profile briefing articles
techtwitter-pp-cli profile list --json
techtwitter-pp-cli profile show briefing
techtwitter-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `techtwitter-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/techtwitter/cmd/techtwitter-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add techtwitter-pp-mcp -- techtwitter-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which techtwitter-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   techtwitter-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `techtwitter-pp-cli <command> --help`.
