---
name: pp-digg
description: "Search Digg stories live across all seven topics (ai, tech, world-business, sports, science, gaming, entertainment), read the ranked story leaderboard with Digg Scores, and pull a story's sources, GitHub feeds and rank history from the terminal. Read-only. Trigger phrases: `what's trending on Digg`, `digg top stories`, `search Digg for`, `what is Digg saying about`, `digg sports stories`, `digg github stars`, `digg github recent`, `digg leaderboard`, `use digg`, `run digg-pp-cli`."
author: "Matt Van Horn"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - digg-pp-cli
---

# Digg — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `digg-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install digg --cli-only
   ```
2. Verify: `digg-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/cmd/digg-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use this CLI when an agent or power user needs structured access to Digg: live story search across all seven topics (ai, tech, world-business, sports, science, gaming, entertainment), the ranked story feed with Digg Scores, the sources behind a story, a local rank history, the account rankings, or the GitHub feeds. It is the right tool for finding what Digg is covering on a subject and how big each story is, for citing the posts behind a story, for tracking how a story's rank moved, or for feeding Digg signals into a larger automation. Do NOT use it for vote, comment, or post automation — those mutations are explicitly out of scope.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Live search and story sources
- **`search`** — Search Digg stories with Digg's own search, live. It needs no prior `sync` and reads no local copy. Without `--topic` every topic is searched; `--topic` takes one or more of ai, tech, world-business, sports, science, gaming, entertainment, all.

  _Each result carries `clusterId` (the story's UUID), `clusterUrlId` (the short id from its URL), `title`, `tldr`, `description`, `url`, `topic`, `diggScore`, `sourceCount`, `postCount` (same number as `sourceCount`), `uniqueAuthors`, `publishedAt`, `firstSourceAt` and `firstPostAge`. `meta` carries `source: "live"`, `stale: false`, `totalMatches`, `window` and `sort`._

  _Search rows carry no `rank`: Digg's search does not return one (`top` rows do). `postCount` always equals `sourceCount`, and `firstPostAge` is the age since `publishedAt` (hours under a day, whole days after), not the age of the first source._

  ```bash
  digg-pp-cli search "<topic>" --since 30d --agent --select title,diggScore,sourceCount,uniqueAuthors,url
  ```
  - **`--since`, `--until`** — a duration back from now (`12h`, `7d`, `2w`, `1m` for 30 days) or an ISO date (`2026-11-01`). They filter on `publishedAt`. Digg search reaches back 90 days.
  - **`--sort relevance|score|newest`** — `score` puts the highest Digg Score first.
  - **`--limit`** — may exceed one page of 50; search follows Digg's cursor.
  - A failed search exits non-zero. With `--json` or `--agent` the envelope still prints, with the reason in `meta.error`. Never read an empty `results` with `meta.error` set as "no stories".
- **`posts`** — Every source Digg attributes one story to, in any topic. Each row has the source's text (`body`), URL (`xUrl`), time (`posted_at`) and `kind`: `x` for an X post (with `author.username` and `author.display_name`) or `webpage` (with `title`). In JSON, `meta.sourcesTruncated` is `true` when the story has more sources than Digg served.

  ```bash
  digg-pp-cli posts <story> --limit 5 --agent --select author.username,xUrl,body
  ```
  `<story>` is the story's UUID (`clusterId`), its short id (`clusterUrlId`), or its URL. Digg itself accepts only the UUID, so a short id or URL resolves only after `search` has returned that story on this machine, from the record `search` keeps in the local store; one it has not recorded fails with a not-found error that says so. Pass the UUID from `clusterId` when `search` ran on another machine or home directory, or printed a warning that it could not record story ids. `--by time` sorts oldest first. `--by rank`, `--by type` and `--type` are accepted and have no effect, because Digg no longer reports author rank or post type.
- **`story`** — One story as Digg publishes it: title, summary, topic, URL and dates, the earlier and later stories in the same thread, and its first 20 sources. Takes the same `<story>` argument as `posts`.
- **`open`** — Print the story's digg.com URL under its own topic; `--launch` opens it in a browser.

### Ranked stories, live
- **`top`** — Digg's ranked story feed, read live with no sync. `--topic` returns one topic in rank order. Without it, `top` reads the first page (20 stories) of every topic and merges them by Digg Score.

  _Output is a `{meta, results}` envelope. Each row has `rank` (within its own topic), `diggScore`, `postCount`, `uniqueAuthors`, `publishedAt`, `topic`, `url`, `title`, `description`, `clusterId` and `clusterUrlId`._

  ```bash
  digg-pp-cli top --topic ai --view trending --range 7d --json --select results.title,results.diggScore,results.url
  ```
  `--view top|trending`, `--range 24h|7d|30d|all`, `--limit`.

### Local state that compounds
- **`sync`** — Take one snapshot of each topic's ranked list (`--topics`, default `ai`) and save it to a local SQLite database. `history`, `stats`, `author` and `authors top` read these snapshots. `watch` polls Digg itself and keeps its own baseline per topic. `search` and `top` do not need it.
- **`history`** — Full trajectory of one cluster's currentRank, peakRank, and delta over local snapshot history.

  _'Entered at #18, peaked at #4 over 6h, dropped to #22 by 24h' is impossible to learn from the live site._

  ```bash
  digg-pp-cli history iq7usf9e --json
  ```
- **`watch`** — Poll one Digg topic (`--topic`, default `ai`), compare each story's rank with the previous poll of that same topic (stored locally per topic, so a restarted watch continues from it and the first poll of a topic only sets the baseline), and alert when any cluster moves N+ ranks.

  _Read-only operational watcher; never writes anything back to Digg._

  ```bash
  digg-pp-cli watch --topic sports --alert 'rank.delta>=10'
  ```
- **`authors top`** — Top accounts in the local store, ranked by Digg's influence score, story count, or reach.

  ```bash
  digg-pp-cli authors top --by influence --limit 50 --json
  ```
- **`author`** — Every synced cluster a given X account contributed to.

  ```bash
  digg-pp-cli author Scobleizer --since 7d --json
  ```

### Account roster
- **`authors list`** — Digg's ranked roster of accounts on X, fetched from its rankings page and saved locally with rank, category, bio, vibeDistribution and GitHub URL.

  ```bash
  # Biggest movers since the last snapshot
  digg-pp-cli authors list --by rankChange --limit 20 --agent

  # Newly listed accounts
  digg-pp-cli authors list --only-new --agent
  ```
  Sort with `--by rank|rankChange|category|followers`; filter with `--category "<name>"`, `--only-new`, `--only-fallers`.
- **`authors get`** — Look up any X handle through `/api/search/users`, ranked or not. A handle with no rank comes back with `current_rank: null` and `tier_status: "off_1000"`, plus `nearest_in_1000`: the account at rank 1000 with its `peer_follow_count`, for comparison.

  ```bash
  digg-pp-cli authors get <handle> --agent
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**search** — Search Digg stories live, across one or more topics

- `digg-pp-cli search "<query>"` — Calls Digg's search. Flags: `--topic`, `--since`, `--until`, `--sort relevance|score|newest`, `--limit`.

**top** — Digg's ranked stories right now

- `digg-pp-cli top` — One topic in rank order, or every topic merged by Digg Score. Flags: `--topic`, `--view top|trending`, `--range 24h|7d|30d|all`, `--limit`.

**posts, story, open** — One story, in any topic. Each takes the story's UUID, short id or URL.

- `digg-pp-cli posts <story>` — The story's sources: text, author, URL and time. Flags: `--by time`, `--limit`.
- `digg-pp-cli story <story>` — The story's detail, thread links and first 20 sources.
- `digg-pp-cli open <story>` — Print the story's URL. Flag: `--launch`.

**sync, history, watch, stats** — Local rank history

- `digg-pp-cli sync` — Snapshot each topic's ranked list. Flags: `--topics`, `--db`.
- `digg-pp-cli history <clusterUrlId>` — One cluster's rank trajectory across snapshots.
- `digg-pp-cli watch` — Poll one topic and alert on rank moves. Flags: `--topic`, `--interval`, `--min-delta`, `--alert`, `--iterations`.
- `digg-pp-cli stats` — Cluster count, author count and snapshot depth of the local store.

**authors** — Inspect the accounts Digg tracks

- `digg-pp-cli authors get <handle>` — Look up any X handle; an unranked handle includes the rank-1000 `nearest_in_1000` anchor. Flag: `--limit` (fuzzy fallback).
- `digg-pp-cli authors list` — Full ranked roster from Digg's rankings page, saved with rich fields. Flags: `--by rank|rankChange|category|followers`, `--category`, `--only-new`, `--only-fallers`, `--limit`.
- `digg-pp-cli authors top` — Top contributors in the local store by influence, post count, or reach. Flags: `--by`, `--limit`.
- `digg-pp-cli authors overlap <userA> <userB>` — Synced clusters both accounts contributed to. Flag: `--limit`.

**rankings** — Digg's ranking of people and companies on X (/tech/x/rankings)

- `digg-pp-cli rankings list` — Full ranking of people and companies. Server-paginated; returns the initial-HTML slice. Flags: `--limit`, `--max-skip-ratio`.

**github** — GitHub feeds Digg surfaces alongside the X-account leaderboard

- `digg-pp-cli github stars` — Top repos ranked by starring activity from Digg-tracked accounts. Returns repo_full_name, language, stargazers_count, recent starrers, breakout/novel/ai_related scores, and the model's one-sentence classification. Flags: `--limit`, `--min-starrers N` (keep only repos with >= N distinct starrers; applied BEFORE --limit).
- `digg-pp-cli github new` — Recently first-seen repos with the Digg-tracked creator/starrer who first put them on Digg's radar (event_id, event_created_at, repo_full_name, creator). Flag: `--limit`.
- `digg-pp-cli github activity` — Top GitHub contributor leaderboard: per-author rank, contribution count, and distinct repos count over Digg's tracking window. Flag: `--limit`.
- `digg-pp-cli github recent` — Live activity feed: per-event entries with the GitHub URL and the user who acted. Flag: `--limit`.

**feed** — Raw Digg pages, as HTML

- `digg-pp-cli feed raw` — Fetch the raw HTML of Digg's AI topic page (/ai). Most users should run `top` instead.
- `digg-pp-cli feed story_raw` — Fetch the raw HTML of one story's page under /ai. Most users should run `story` or `posts` instead.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
digg-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Topic search for research workflows

```bash
digg-pp-cli search "<topic>" --since 30d --agent
```

Searches every Digg topic live and returns each story's Digg Score, source count, author count and dates. Add `--topic ai,tech` to narrow it and `--sort score` to put the biggest stories first. No sync is needed. Pair with `posts` for citations.

### The biggest stories right now

```bash
# Across every topic, merged by Digg Score
digg-pp-cli top --limit 20 --agent

# One topic, in rank order
digg-pp-cli top --topic sports --limit 10 --agent
```

### Sources behind a story (citations)

```bash
digg-pp-cli posts <story> --limit 5 --agent --select author.username,xUrl,body
```

Returns the text, author and URL of the posts and pages Digg built the story from. Pass the `clusterId` from a `search` or `top` row to go from topic to story to quotable sources in two commands.

### Roster browse: biggest movers and newly listed

```bash
# Biggest movers since the last snapshot
digg-pp-cli authors list --by rankChange --limit 20 --agent

# Newly listed accounts
digg-pp-cli authors list --only-new --agent
```

Sort with `--by rank|rankChange|category|followers`; filter with `--category`, `--only-new`, `--only-fallers`.

### Show every cluster a given X account contributed to this week

```bash
digg-pp-cli sync --topics ai,tech
digg-pp-cli author Scobleizer --since 7d --json --select label,clusterUrlId,activityAt
```

Queries the local store for synced clusters where the named author was a contributor; output is narrowed for agent consumption.

## Auth Setup

No authentication required.

Run `digg-pp-cli doctor` to verify setup. Its `search` line reports whether Digg's search endpoint answers (`reachable` or `unreachable: <reason>`).

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  digg-pp-cli feed raw --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Local history** — `sync` snapshots feed `history`, `stats` and `sql` from a local SQLite store; `search` and `top` always read Digg live
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
digg-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
digg-pp-cli feedback --stdin < notes.txt
digg-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.digg-pp-cli/feedback.jsonl`. They are never POSTed unless `DIGG_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `DIGG_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
digg-pp-cli profile save briefing --json
digg-pp-cli --profile briefing feed raw
digg-pp-cli profile list --json
digg-pp-cli profile show briefing
digg-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `digg-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add digg-pp-mcp -- digg-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which digg-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   digg-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `digg-pp-cli <command> --help`.
