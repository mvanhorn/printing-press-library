---
name: pp-producthunt
description: "Read Product Hunt from your terminal — works token-free for the daily skim, unlocks a launch-day cockpit and a marketer research desk in one onboarding step. Trigger phrases: `what launched on product hunt today`, `find ai launches on product hunt this week`, `how is my product hunt launch tracking`, `compare these product hunt launches`, `summarize the comments on this product hunt post`, `what does a good product hunt launch look like at hour 6`, `use producthunt`, `run producthunt`."
version: "3.2.1"
author: "Trevin Chow"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - producthunt-pp-cli
    install:
      - kind: go
        bins: [producthunt-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/marketing/producthunt/cmd/producthunt-pp-cli
---

# Product Hunt — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `producthunt-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install producthunt --cli-only
   ```
2. Verify: `producthunt-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/producthunt/cmd/producthunt-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Pick this CLI when an agent or script needs Product Hunt data — daily launch leaderboards, topic-vertical scouting, individual launch detail with comments, or aggregated trajectories that PH's own UI never exposes. The local SQLite store and GraphQL fall-through mean repeated queries stay cheap; the no-auth feed tier means the CLI is useful before the user has even configured a key. Indie founders launching this week reach for the launch-day cockpit (`launch-day`, `benchmark`, `trajectory`, `questions`, `compare`); marketers doing competitive research reach for the marketer research desk (`category snapshot`, `posts grep`, `lookalike`, `launches calendar`).

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Founder launch-day cockpit
- **`posts launch-day`** — Renders your launch's votes-over-time trajectory side-by-side with today's top 5 launches — the answer to 'am I catching up to the leader.' Sync-driven, refreshes from the local store.

  _Reach for this on launch day when a maker asks 'how am I tracking vs the leaders' — the side-by-side trajectories replace ten tabs._

  ```bash
  producthunt posts launch-day my-launch-slug --json
  ```
- **`posts benchmark`** — Reports percentile curves at hour-N for top-10 and top-50 launches in a topic, computed from accumulated local history. Tells a founder if their hour-6 votes are 'good' for their category.

  _Use before launching to set realistic targets, or during launch to know whether a slow start is normal for the category or a real problem._

  ```bash
  producthunt posts benchmark --topic artificial-intelligence --hour 6 --json
  ```
- **`posts trajectory`** — Plots a single launch's votes-over-time from local snapshots. Foundational for launch-day-tracker; also useful standalone for retro analysis after the fact.

  _Reach for this when reviewing a past launch or competitor — the curve shows momentum that a single end-of-day vote count hides._

  ```bash
  producthunt posts trajectory my-launch-slug --json
  ```
- **`posts questions`** — Surfaces only comments that look like real questions (regex `?` plus heuristic verbs like 'how does', 'what's the', 'can it'), ranked by vote count. Cuts hundreds of launch-day comments down to the ones that need a maker's reply.

  _Use during or after launch day to identify which comments deserve a real reply versus which are cheerleading or spam._

  ```bash
  producthunt posts questions my-launch-slug --json
  ```
- **`posts compare`** — Column-aligned comparison of two or more launches: votes, comments, topics, tagline, url, launch-time delta. Replaces juggling browser tabs.

  _Pick this when a founder is benchmarking their launch against precedents or a marketer is triangulating between similar competitive launches._

  ```bash
  producthunt posts compare cursor-ide windsurf-ide claude-code --json
  ```

### Marketer research desk
- **`category snapshot`** — Slide-deck-ready brief for a topic over a window: leaderboard + momentum delta vs prior window + most active poster handles + top emerging tagline tags.

  _Reach for this on weekly category-research cadence — the single-output brief replaces opening 30 launch pages by hand._

  ```bash
  producthunt category snapshot --topic artificial-intelligence --window weekly --agent --select leaderboard,momentum_delta
  ```
- **`posts grep`** — Searches taglines and descriptions of launches in a window for a term — your brand, a competitor's brand, a category keyword. Returns matching launches with the matched snippet.

  _Use this as a recurring brand-mention monitor or to find competitive launches that name your category in their pitch._

  ```bash
  producthunt posts grep --term "\\bclaude\\b" --since 7d --topic developer-tools --json
  ```
- **`posts lookalike`** — Given a launch slug, finds the most similar prior launches by topic overlap plus tagline FTS rank. Builds a competitive set automatically.

  _Reach for this to build a competitive set quickly or to find precedent launches when planning your own positioning._

  ```bash
  producthunt posts lookalike notion --json --select edges.node.name,edges.node.tagline
  ```
- **`launches calendar`** — Shows what launched what day in a week (and prior weeks for context), with hour-of-day distribution. Helps a founder pick a strong launch slot.

  _Use before scheduling a launch to find a less-crowded day or hour in your topic._

  ```bash
  producthunt launches calendar --topic artificial-intelligence --week 18 --json
  ```

### Cross-persona monitoring
- **`topics watch`** — Detects new posts crossing a vote threshold in a topic since the last sync. Synthesizes an offline subscription against an API that has none.

  _Schedule this in cron to alert on notable new launches in a vertical without hammering the GraphQL endpoint._

  ```bash
  producthunt topics watch artificial-intelligence --min-votes 200 --json
  ```

### Agent-native plumbing
- **`posts since`** — Local-first time-window query: `posts since 2h`, `posts since 24h`. Falls through to live GraphQL if the window extends past the last sync.

  _Reach for this from agentic flows that ask 'what's new on Product Hunt' — the local-first behavior keeps token costs low and the fall-through guarantees freshness._

  ```bash
  producthunt posts since 6h --json --select edges.node.name,edges.node.votesCount
  ```
- **`context`** — Returns a single JSON blob covering top posts in a window, top comments, topic followers, and your viewer status. One call answers 'what's the state of this topic right now' for an agent.

  _Use as the first call in an agentic Product Hunt workflow — one snapshot replaces 'list posts then list comments then check viewer'._

  ```bash
  producthunt context --topic artificial-intelligence --since 24h --json
  ```

## Command Reference

**feed** — Public Atom feed of featured Product Hunt launches (no auth required)

- `producthunt-pp-cli feed` — Fetch the public Atom feed of recent featured launches; needs no token


**Hand-written commands**

- `producthunt-pp-cli posts` — Get, list, and analyze Product Hunt launches via GraphQL
- `producthunt-pp-cli posts get <id-or-slug>` — Fetch full detail for a single launch (votes, comments, topics, makers, media)
- `producthunt-pp-cli posts list` — List launches filtered by topic, order, featured-flag, or posted-after window
- `producthunt-pp-cli posts comments <id-or-slug>` — List comments on a launch (commenter identities are redacted by Product Hunt)
- `producthunt-pp-cli posts trajectory <slug>` — Plot a launch's votes-over-time from local snapshots
- `producthunt-pp-cli posts launch-day <my-slug>` — Render YOUR launch's trajectory side-by-side with today's top 5 launches
- `producthunt-pp-cli posts benchmark` — Show percentile vote curves at hour-N for top-10 / top-50 launches in a topic
- `producthunt-pp-cli posts compare <slug1> <slug2> [<slug3>...]` — Column-aligned comparison of N launches: votes, comments, topics, tagline, url
- `producthunt-pp-cli posts questions <slug>` — Filter a launch's comments to only those that look like genuine questions
- `producthunt-pp-cli posts grep` — Search synced launches' taglines and descriptions for a keyword (brand-mention tracker)
- `producthunt-pp-cli posts lookalike <slug>` — Find prior launches in the same topic with similar tagline tokens (competitive set)
- `producthunt-pp-cli posts since <duration>` — Local-first time-window query (e.g., `posts since 6h`); falls through to live API
- `producthunt-pp-cli comments` — Get individual comments by id
- `producthunt-pp-cli comments get <id>` — Fetch a single comment by ID
- `producthunt-pp-cli collections` — Get and list Product Hunt curated collections
- `producthunt-pp-cli collections get <id-or-slug>` — Fetch a single collection with its post list
- `producthunt-pp-cli collections list` — List collections, optionally filtered by featured-flag or by a contained post/user
- `producthunt-pp-cli topics` — Get, list, and search Product Hunt topics (categories)
- `producthunt-pp-cli topics get <id-or-slug>` — Fetch a single topic (followers count, posts count)
- `producthunt-pp-cli topics list` — List topics, optionally ordered by newest or popularity
- `producthunt-pp-cli topics search <query>` — Search topics by name or description
- `producthunt-pp-cli topics watch <slug>` — Detect new posts crossing a vote threshold in a topic since the last sync
- `producthunt-pp-cli users` — Get user profiles and a user's launch/voted history (non-self users redacted by Product Hunt)
- `producthunt-pp-cli users get <id-or-username>` — Fetch a user profile (only `whoami`/yourself returns full data; non-self users return [REDACTED])
- `producthunt-pp-cli users posts <username>` — List the posts a user has made (post data is unredacted; the user identity itself may be redacted)
- `producthunt-pp-cli users voted-posts <username>` — List the posts a user has voted for (same redaction caveat as users posts)
- `producthunt-pp-cli whoami` — Show the authenticated user (full data; reports remaining complexity-budget and auth mode)
- `producthunt-pp-cli category` — Marketer research desk: category-level snapshots and trends
- `producthunt-pp-cli category snapshot` — Slide-deck-ready brief for a topic over a window: leaderboard, momentum delta, active handles, emerging tags
- `producthunt-pp-cli launches` — Calendar and timing analysis for launch slot picking
- `producthunt-pp-cli launches calendar` — Show what launched what day in a week, with hour-of-day distribution
- `producthunt-pp-cli context` — Single-call agent snapshot: top posts + top comments + topic state + viewer in one JSON blob
- `producthunt-pp-cli today` — Today's top launches (alias for `posts list --order=RANKING --posted-after=midnight`)
- `producthunt-pp-cli recent` — Most recent launches (alias for `posts list --order=NEWEST`)
- `producthunt-pp-cli search <query>` — Full-text search of locally synced posts
- `producthunt-pp-cli sync` — Sync the local store: posts, topics, collections, comments (cursor-based, resumable)
- `producthunt-pp-cli sql <query>` — Read-only SQL against the local SQLite store
- `producthunt-pp-cli auth` — Manage Product Hunt authentication (developer token by default, OAuth client_credentials alternate)
- `producthunt-pp-cli auth onboard` — Interactive wizard: walks you through creating a free OAuth app, prefills `https://localhost/callback` for the...
- `producthunt-pp-cli auth set-token <token>` — Set the developer token explicitly without going through the onboarding wizard
- `producthunt-pp-cli auth status` — Report which auth mode is active and what data it can access
- `producthunt-pp-cli doctor` — Auth-stage-aware diagnostic: tells you whether to onboard, regenerate a token, or wait for a budget reset


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
producthunt-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Launch-day cockpit for your own launch

```bash
producthunt posts launch-day my-launch-slug --json
```

Renders your launch's vote trajectory side-by-side with today's top 5 — the answer to 'am I catching up to the leader' without refreshing the website

### Triage a launch's comments to genuine questions

```bash
producthunt posts questions my-launch-slug --json
```

Filters synced comments to only those that look like real questions (regex `\\?` plus question-verbs) ranked by vote count — replaces scrolling 200 comments to find the 5 that deserve a reply

### Narrow a deeply nested response with --agent --select

```bash
producthunt posts list --topic artificial-intelligence --posted-after 2026-04-01 --agent --select edges.node.name,edges.node.votesCount,edges.node.user.username
```

Posts list responses are deeply nested (edges -> node -> user); pairing `--agent` with dotted `--select` paths keeps the output to just the columns the agent needs and slashes token cost

### Weekly category brief for marketing

```bash
producthunt category snapshot --topic developer-tools --window weekly --json
```

Single-output brief: leaderboard for the week, momentum delta vs prior week, most active poster handles, top emerging tagline tags — slide-deck-ready

### Brand-mention monitor

```bash
producthunt posts grep --term "\\bclaude\\b|\\banthropic\\b" --since 7d --json
```

Finds any launch in the window with `claude` or `anthropic` in tagline or description — schedule it nightly to catch competitive activity

## Auth Setup

Product Hunt's GraphQL API supports two auth modes; the CLI handles both. Recommended for personal use: visit https://www.producthunt.com/v2/oauth/applications, create an application (the redirect URL field is required by the form but unused for personal-token flow — set it to `https://localhost/callback`), then scroll to the bottom of the app page and click `Create Token` to generate a developer token that never expires. Set `PRODUCT_HUNT_TOKEN=<your-token>` or run `producthunt auth onboard` for an interactive walkthrough. For CI/automation, the alternate mode is OAuth `client_credentials`: set `PRODUCT_HUNT_CLIENT_ID` and `PRODUCT_HUNT_CLIENT_SECRET` from the same app page; the CLI exchanges them for an access token internally and refreshes on 401 (note: under OAuth client_credentials, the `whoami` command returns null because the public scope has no user context). The public Atom feed (`producthunt feed`) needs no token at all.

Run `producthunt-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  producthunt-pp-cli feed --agent --select id,name,status
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
producthunt-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
producthunt-pp-cli feedback --stdin < notes.txt
producthunt-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.producthunt-pp-cli/feedback.jsonl`. They are never POSTed unless `PRODUCTHUNT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `PRODUCTHUNT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
producthunt-pp-cli profile save briefing --json
producthunt-pp-cli --profile briefing feed
producthunt-pp-cli profile list --json
producthunt-pp-cli profile show briefing
producthunt-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `producthunt-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)
## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/marketing/producthunt/cmd/producthunt-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add producthunt-pp-mcp -- producthunt-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which producthunt-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   producthunt-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `producthunt-pp-cli <command> --help`.
