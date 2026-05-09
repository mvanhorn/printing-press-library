---
name: pp-product-hunt
description: "Every Product Hunt feature, plus launch-day momentum tracking, offline search, and a local database no other Product... Trigger phrases: `what launched on Product Hunt today`, `track my Product Hunt launch`, `competitor research on Product Hunt`, `Product Hunt topic monitoring`, `which topics are trending on Product Hunt`, `use product-hunt`, `run product-hunt-pp-cli`."
author: "actionsslave"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - product-hunt-pp-cli
---

# Product Hunt — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `product-hunt-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install product-hunt --cli-only
   ```
2. Verify: `product-hunt-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/product-hunt/cmd/product-hunt-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

This CLI syncs launches, topics, and makers to a local SQLite store so you can query them offline, track vote momentum in real time, and pipe structured JSON into your own scripts. Whether you're researching a launch, monitoring a topic, or building a daily briefing, it gives you PH data without the browser.

## When to Use This CLI

Use product-hunt-pp-cli when you need structured Product Hunt data in scripts, LLM pipelines, or CI workflows. Best for founders tracking launch-day momentum, developers monitoring topic velocity, and agents running daily briefings. Prefer this over manual PH browsing for any repeatable data workflow.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`posts momentum`** — See live vote and comment deltas for any post since your last check — no browser required on launch day.

  _Use this to track launch-day rank changes or monitor a competitor's post in real time without polling a browser._

  ```bash
  product-hunt-pp-cli posts momentum developer-tools-daily --json
  ```
- **`watchlist`** — Pin any posts (yours, competitors), then batch-refresh all of them at once to see current stats and deltas.

  _Use to track your own launch and a handful of competitors without opening the browser._

  ```bash
  product-hunt-pp-cli watchlist refresh --json
  ```
- **`posts cross-topic`** — Find posts that appear in multiple specified topics simultaneously — 'AI tools that are also productivity tools'.

  _Use to find products in your exact market niche where multiple category labels intersect._

  ```bash
  product-hunt-pp-cli posts cross-topic --topics artificial-intelligence,productivity --json
  ```

### Launch intelligence
- **`topics audit`** — Get every recent launch in a topic ranked by votes, with maker overlap detection across top posts.

  _Use before launching to understand competitive density and identify which makers dominate your category._

  ```bash
  product-hunt-pp-cli topics audit developer-tools --limit 20 --agent
  ```
- **`topics velocity`** — Week-over-week post count and average vote delta for a topic — is your category heating up or cooling down?

  _Use to decide whether a category is worth entering or to track how competitive dynamics are shifting over time._

  ```bash
  product-hunt-pp-cli topics velocity developer-tools --weeks 4 --agent
  ```
- **`posts vote-rate`** — Rank posts by votes-per-day-since-launch instead of raw vote totals — surfaces underrated recent launches.

  _Use to find products with strong launch momentum that raw vote leaderboards don't surface._

  ```bash
  product-hunt-pp-cli posts vote-rate --topic developer-tools --days 30 --json
  ```
- **`topics trending`** — Which topics have the fastest-growing post count and vote volume this week versus last week?

  _Use to identify emerging categories before they peak, or to find where to launch for maximum visibility._

  ```bash
  product-hunt-pp-cli topics trending --days 7 --agent
  ```
- **`makers portfolio`** — Aggregate stats across a maker's full product history: total votes, avg per launch, best launch, days since last launch.

  _Use to research prolific makers before collaborating, or track your own cumulative launch impact._

  ```bash
  product-hunt-pp-cli makers portfolio rrhoover --json
  ```

### Agent-native plumbing
- **`digest`** — Structured summary of yesterday's or today's top launches per topic — works offline after sync.

  _Use in morning briefing scripts or LLM pipelines to get a structured feed of top launches without hitting rate limits._

  ```bash
  product-hunt-pp-cli digest developer-tools --yesterday --json
  ```
- **`topics inbox`** — See only posts that are new since you last checked for each subscribed topic — cursor-based, never re-shows seen launches.

  _Use for competitive monitoring workflows where you want exactly the delta since last run, not a full topic dump._

  ```bash
  product-hunt-pp-cli topics inbox --json --select posts.name,posts.votesCount
  ```

## Command Reference

**collections** — Browse Product Hunt curated collections

- `product-hunt-pp-cli collections get` — Get a collection by slug or numeric ID
- `product-hunt-pp-cli collections list` — List collections

**comments** — Look up individual Product Hunt comments

- `product-hunt-pp-cli comments <id>` — Get a comment by ID

**posts** — Browse and track Product Hunt launches

- `product-hunt-pp-cli posts comments` — List comments for a post
- `product-hunt-pp-cli posts get` — Get a specific post by slug or numeric ID
- `product-hunt-pp-cli posts list` — List posts with optional topic, sort, and date filters

**topics** — Browse and monitor Product Hunt topic categories

- `product-hunt-pp-cli topics get` — Get a topic by slug or numeric ID
- `product-hunt-pp-cli topics list` — List topics with optional search query

**users** — Look up Product Hunt maker and hunter profiles

- `product-hunt-pp-cli users follow` — Follow a user
- `product-hunt-pp-cli users get` — Get a user profile by username or numeric ID
- `product-hunt-pp-cli users me` — Get the authenticated user's profile
- `product-hunt-pp-cli users unfollow` — Unfollow a user


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
product-hunt-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Morning launch briefing

```bash
product-hunt-pp-cli digest developer-tools --yesterday --json | jq '.posts[] | {name, votes: .votesCount, tagline}'
```

Get yesterday's developer-tools launches as a structured JSON feed for your morning script or LLM pipeline

### Track your own launch

```bash
product-hunt-pp-cli watchlist add my-product-slug && product-hunt-pp-cli posts momentum my-product-slug
```

Pin your launch and see live vote+comment deltas since your last check

### Pre-launch competitive audit

```bash
product-hunt-pp-cli topics audit developer-tools --limit 20 --agent --select posts.name,posts.votesCount,posts.makers.username
```

Get every recent launch in developer-tools ranked by votes with maker overlap — use --select to narrow the response for agents

### Topic trend analysis

```bash
product-hunt-pp-cli topics velocity developer-tools --weeks 4 --agent
```

See week-over-week post count and avg vote delta to understand whether your target category is heating up

### New-only topic feed

```bash
product-hunt-pp-cli topics subscribe developer-tools && product-hunt-pp-cli topics inbox --json
```

Subscribe to a topic and see only posts you haven't seen yet — advances the cursor so next run shows only newer launches

## Auth Setup

Get a free developer token from your Product Hunt API Dashboard (producthunt.com → Settings → API Dashboard → Create Application → Developer Token). Set PRODUCT_HUNT_TOKEN in your environment. The token never expires.

Run `product-hunt-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  product-hunt-pp-cli collections list --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
product-hunt-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
product-hunt-pp-cli feedback --stdin < notes.txt
product-hunt-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.product-hunt-pp-cli/feedback.jsonl`. They are never POSTed unless `PRODUCT_HUNT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `PRODUCT_HUNT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
product-hunt-pp-cli profile save briefing --json
product-hunt-pp-cli --profile briefing collections list
product-hunt-pp-cli profile list --json
product-hunt-pp-cli profile show briefing
product-hunt-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `product-hunt-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add product-hunt-pp-mcp -- product-hunt-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which product-hunt-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   product-hunt-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `product-hunt-pp-cli <command> --help`.
