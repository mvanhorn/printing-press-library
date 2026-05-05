---
name: pp-hackernews
description: "Hacker News from your terminal — with a local SQLite store, snapshot history, and agent-native output no other HN tool has. Trigger phrases: `check hacker news`, `search hn`, `what is hn saying about`, `diff the hn front page`, `pulse on hn`, `look up hn user`, `hn who is hiring`, `hn top stories`, `use hackernews`, `run hackernews`."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["hackernews-pp-cli"]},"install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/hackernews/cmd/hackernews-pp-cli@latest","bins":["hackernews-pp-cli"],"label":"Install via go install"}]}}'
---

# Hacker News — Printing Press CLI

Combines the Firebase real-time API and the Algolia search API in one CLI. Sync once and run cross-month hiring digests, topic pulses, repost lookups, and rank-over-time queries against a local store — offline, scriptable, MCP-ready. Every command supports --json and --select; the API itself is read-only, so this CLI is too.

## When to Use This CLI

Reach for hackernews-pp-cli when you need to monitor or analyze HN signal programmatically: agent-driven daily diffs, topic pulses, hiring-thread aggregation, repost checks before submitting, structured thread digests for context windows. The local store makes follow-up queries cheap and offline-friendly.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local snapshots that compound
- **`since`** — See exactly what climbed, fell, appeared, or dropped off the front page since your last sync.

  _Reach for this when an agent wakes up daily and needs to know what shifted on HN since yesterday — without re-fetching 500 items every poll._

  ```bash
  hackernews-pp-cli since --json
  ```
- **`controversial`** — Stories ranked by the highest comment-to-point ratio over a recent window — the discussions everyone is arguing about.

  _Reach for this when you want stories with high engagement-to-approval — heated debate signal — instead of just popularity._

  ```bash
  hackernews-pp-cli controversial --window 7d --json
  ```
- **`velocity`** — Show a story's rank trajectory over time from local snapshots — climb, plateau, or fall.

  _Reach for this when an agent asks 'is this story still gaining traction or already cresting' — only meaningful answer comes from snapshots._

  ```bash
  hackernews-pp-cli velocity 47998158 --json
  ```
- **`sync`** — Pull top/new/best/show/ask/job lists and recently-changed items into local SQLite for offline use and snapshot history.

  _Run this once per day (or per hour for agents) — it is the foundation that turns since/velocity/controversial/users-stats from impossible into one SQL query._

  ```bash
  hackernews-pp-cli sync --resources updates --agent
  ```

### Algolia leverage
- **`pulse`** — See per-day mentions, average score, and comment volume for any topic over the last N days.

  _Reach for this when the question is 'is this topic heating up or cooling down' rather than 'what's the top story right now'._

  ```bash
  hackernews-pp-cli pulse rust --days 7 --agent
  ```
- **`repost`** — Has this URL been posted before? Lists every prior submission, with score, comments, and date.

  _Reach for this before submitting a Show HN — duplicate URLs flame out instantly; you want to know how a prior post did first._

  ```bash
  hackernews-pp-cli repost https://example.com/article
  ```
- **`search local`** — Offline full-text search over every story and comment you have ever synced — corpus grows with use.

  _Reach for this when investigating long-tail topics or replaying last quarter's research — Algolia might rank it down or drop it; your local corpus will not._

  ```bash
  hackernews-pp-cli search local "vector database" --limit 20 --json
  ```

### Hiring-thread mining
- **`hiring stats`** — Aggregate the last N monthly Who-is-Hiring threads: top languages, remote ratio, top companies, location distribution.

  _Reach for this when you need quarterly or seasonal hiring trends — language popularity, remote-share shifts, location density — not just this month's listings._

  ```bash
  hackernews-pp-cli hiring stats --months 3 --agent
  ```
- **`hiring companies`** — Companies that posted in M of the last N hiring threads, with first-seen, last-seen, and months-posted count.

  _Reach for this when sourcing or trend-tracking — which companies are persistent hirers vs one-off posters — without scraping HNHIRING.com._

  ```bash
  hackernews-pp-cli hiring companies --months 6 --min-posts 3 --agent
  ```

### Cross-entity local queries
- **`users stats`** — Median and p90 score across a user's submissions, plus traction buckets and hour-of-day score distribution.

  _Reach for this before posting your own work to learn your traction patterns, or when sizing up a poster's history before engaging._

  ```bash
  hackernews-pp-cli users stats pg --json
  ```

## Command Reference

**items** — Fetch any HN item (story, comment, job, poll) by ID

- `hackernews-pp-cli items <itemId>` — Get details for a specific story, comment, job, or poll

**maxitem** — Current maximum item ID

- `hackernews-pp-cli maxitem` — Returns the largest item ID currently assigned by Hacker News

**stories** — Browse top, new, and best Hacker News stories

- `hackernews-pp-cli stories ask` — Get the latest Ask HN posts
- `hackernews-pp-cli stories best` — Get the highest-voted stories on Hacker News
- `hackernews-pp-cli stories job` — Get the latest Hacker News job postings
- `hackernews-pp-cli stories new` — Get the newest stories on Hacker News
- `hackernews-pp-cli stories show` — Get the latest Show HN posts
- `hackernews-pp-cli stories top` — Get the current top stories on Hacker News

**updates** — Recently changed items and profiles

- `hackernews-pp-cli updates` — Items and user profiles that have changed recently

**users** — Look up Hacker News user profiles

- `hackernews-pp-cli users <userId>` — Get a user's profile including karma and submission history


**Hand-written commands**

- `hackernews-pp-cli sync` — Pull top/new/best/show/ask/job lists and recent items into the local SQLite store
- `hackernews-pp-cli search <query>` — Full-text search Hacker News stories and comments via Algolia (use 'search local' for offline FTS)
- `hackernews-pp-cli hiring` — Mine 'Ask HN: Who is hiring' threads (filter, stats, companies)
- `hackernews-pp-cli freelance` — Mine 'Ask HN: Freelancer? Seeking freelancer?' threads (filter)
- `hackernews-pp-cli since` — Show what changed on the front page since the last sync (added, removed, moved stories)
- `hackernews-pp-cli pulse <topic>` — Show what HN is saying about a topic — per-day mentions, score, and comment volume
- `hackernews-pp-cli controversial` — Find stories with the highest comment-to-point ratio (polarizing discussions)
- `hackernews-pp-cli repost <url>` — Has this URL been posted on HN? Lists prior submissions with scores and dates
- `hackernews-pp-cli velocity <id>` — Show a story's rank trajectory across local snapshots
- `hackernews-pp-cli doctor` — Run a self-diagnostic: API reachability, store writability, config sanity


## Freshness Contract

This printed CLI owns bounded freshness only for registered store-backed read command paths. In `--data-source auto` mode, those paths check `sync_state` and may run a bounded refresh before reading local data. `--data-source local` never refreshes. `--data-source live` reads the API and does not mutate the local store. Set `HACKERNEWS_NO_AUTO_REFRESH=1` to skip the freshness hook without changing source selection.

Covered paths:

- `hackernews-pp-cli stories`
- `hackernews-pp-cli stories ask`
- `hackernews-pp-cli stories best`
- `hackernews-pp-cli stories job`
- `hackernews-pp-cli stories new`
- `hackernews-pp-cli stories show`
- `hackernews-pp-cli stories top`
- `hackernews-pp-cli updates`

When JSON output uses the generated provenance envelope, freshness metadata appears at `meta.freshness`. Treat it as current-cache freshness for the covered command path, not a guarantee of complete historical backfill or API-specific enrichment.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
hackernews-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Daily front-page diff for an agent

```bash
hackernews-pp-cli sync && hackernews-pp-cli since --json --select added,removed,moved
```

Sync, then diff. The --select narrows the payload to just the change deltas — agents process kilobytes instead of megabytes.

### Topic monitoring with selected fields

```bash
hackernews-pp-cli pulse openai --days 7 --agent --select buckets.day,buckets.hits,buckets.avg_score
```

Per-day breakdown of mentions, average score, and comment volume — using --select to pull only the trend axes from the deeply-nested response.

### Pre-submit dupe check

```bash
hackernews-pp-cli repost https://example.com/article --json
```

Lists every prior submission of that URL with score, comments, and date — answers the 'has this been posted' question in one round-trip.

### Filter Who's Hiring for remote Go roles

```bash
hackernews-pp-cli hiring filter "(remote|REMOTE).*\\bGo\\b" --json
```

Regex against the latest monthly thread. The double-escaped \\b is a real regex word boundary; --json gives you structured rows instead of one big match dump.

### Quarterly hiring trend

```bash
hackernews-pp-cli hiring companies --months 6 --min-posts 3 --agent
```

Companies posting in 3+ of the last 6 monthly threads — first-seen, last-seen, total months. Cross-month queries you cannot do without a local store.

## Auth Setup

No authentication required.

Run `hackernews-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  hackernews-pp-cli items 47998158 --agent --select id,name,status
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
hackernews-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
hackernews-pp-cli feedback --stdin < notes.txt
hackernews-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.hackernews-pp-cli/feedback.jsonl`. They are never POSTed unless `HACKERNEWS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `HACKERNEWS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
hackernews-pp-cli profile save briefing --json
hackernews-pp-cli --profile briefing items 47998158
hackernews-pp-cli profile list --json
hackernews-pp-cli profile show briefing
hackernews-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `hackernews-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → CLI installation
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.23+)
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/hackernews/cmd/hackernews-pp-cli@latest
   ```
3. Verify: `hackernews-pp-cli --version`
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/hackernews/cmd/hackernews-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add hackernews-pp-mcp -- hackernews-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which hackernews-pp-cli`
   If not found, offer to install (see CLI Installation above).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   hackernews-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `hackernews-pp-cli <command> --help`.
