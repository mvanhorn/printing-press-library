---
name: pp-instagram
description: "Every Instagram Business metric across all your brand accounts Trigger phrases: `compare my instagram brands`, `instagram follower growth`, `best time to post on instagram`, `top instagram posts this month`, `instagram competitor analytics`, `use instagram-pp-cli`, `run instagram analytics`."
author: "Mohammed Al Khamis"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - instagram-pp-cli
    install:
      - kind: go
        bins: [instagram-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/social-and-messaging/instagram/cmd/instagram-pp-cli
---

# Instagram — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `instagram-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install instagram --cli-only
   ```
2. Verify: `instagram-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/instagram/cmd/instagram-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

A local-first analytics CLI for the Instagram Graph API built for managers who run several owned Business/Creator accounts. It syncs accounts, media, insights, and competitor snapshots into a queryable SQLite store, then adds the views the official tools lack: rank your brands side by side (compare), follower-growth over time (growth), best-time-to-post, format breakdowns, and competitor deltas. Agent-native output, offline search, and typed exit codes throughout.

## When to Use This CLI

Use this CLI when an agent or operator needs analytics across multiple owned Instagram Business/Creator accounts: comparing brands, tracking follower growth and engagement trends over time, ranking posts, analyzing content formats, or benchmarking competitors. It is the right tool when the question spans accounts or needs history the Graph API's short windows cannot answer.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to scrape or analyze arbitrary public profiles you do not own — only accounts linked to your Pages, plus public business_discovery snapshots, are supported.
- Do not use it as a primary content scheduler; publishing exists with --dry-run but the CLI is analytics-first.
- Do not use it for Facebook Page or Meta Ads analytics — it covers the Instagram organic surface only.
- Do not use it for real-time/streaming metrics — Graph insights are daily/aggregate, not live.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Cross-account portfolio analytics
- **`compare`** — Rank all your brand accounts side by side by reach, interactions, and engagement rate over a window.

  _Reach for this when an agent needs 'which of my brands is winning' in one ranked table instead of N separate account-insights calls._

  ```bash
  instagram-pp-cli compare --since 30d --agent
  ```
- **`growth`** — Track follower-count growth over time per brand, week over week.

  _Reach for this when the question is trend (are we growing?), not a point-in-time count the profile field already gives._

  ```bash
  instagram-pp-cli growth --since 8w --agent
  ```
- **`rivals`** — Track rival public accounts' follower and engagement growth across syncs, benchmarked against your brands.

  _Reach for this for competitive trend questions a single business_discovery call cannot answer._

  ```bash
  instagram-pp-cli rivals --since 30d --agent
  ```

### Derived-from-local-history
- **`best-time`** — Surface the weekday/hour slots where your posts historically earn the most engagement.

  _Reach for this to recommend a posting schedule grounded in that account's own data, not generic folklore._

  ```bash
  instagram-pp-cli best-time --account acme --agent
  ```
- **`top-posts`** — Rank individual posts across your brands by reach, interactions, saves, or shares over a window.

  _Reach for this to find the highest-performing content fast instead of paging and sorting media by hand._

  ```bash
  instagram-pp-cli top-posts --since 30d --metric reach --agent
  ```
- **`formats`** — Compare Reels vs Feed vs Story vs Carousel by reach, engagement, and Reels watch-time.

  _Reach for this to answer 'which content format works for this brand' in one aggregate instead of per-post insight calls._

  ```bash
  instagram-pp-cli formats --account bistro --agent
  ```
- **`hashtag-perf`** — Rank the hashtags you track by the reach and engagement of their top media.

  _Reach for this to compare hashtag ROI; use the absorbed hashtag search to discover new tags instead._

  ```bash
  instagram-pp-cli hashtag-perf --agent
  ```

## Command Reference

**account-insights** — Account-level insights (reach, views, interactions, demographics)

- `instagram-pp-cli account-insights demographics` — Lifetime follower demographics broken down by age, gender, city, or country
- `instagram-pp-cli account-insights list` — Account insights over a window (reach, accounts_engaged, total_interactions, views)

**accounts** — Brand account profiles (Instagram Business/Creator users linked to your Pages)

- `instagram-pp-cli accounts get` — Get a brand account profile (followers, follows, media counts, bio)
- `instagram-pp-cli accounts pages` — List the Facebook Pages you manage and their linked Instagram Business account ids

**business-discovery** — Public metrics for any business/creator account (competitor research)

- `instagram-pp-cli business-discovery <ig_user_id>` — Fetch a competitor's public data via a business_discovery field expression

**comments** — Comments and replies on media

- `instagram-pp-cli comments list` — List comments on a media object
- `instagram-pp-cli comments replies` — List replies to a comment

**hashtags** — Hashtag search and top/recent media

- `instagram-pp-cli hashtags recent-media` — Recent public media for a hashtag id
- `instagram-pp-cli hashtags search` — Resolve a hashtag string to its id (limited to 30 unique tags per account / 7 days)
- `instagram-pp-cli hashtags top-media` — Top-performing public media for a hashtag id

**media** — Posts, reels, and per-media insights

- `instagram-pp-cli media create` — Create a media container (step 1 of publishing). Analytics-first CLI; use --dry-run.
- `instagram-pp-cli media get` — Get a single media object by id
- `instagram-pp-cli media insights` — Per-media insights (reach, views, saved, shares, interactions; Reels watch-time)
- `instagram-pp-cli media list` — List a brand's media (posts, reels, carousels) newest-first
- `instagram-pp-cli media publish` — Publish a previously created media container (step 2).
- `instagram-pp-cli media publish-limit` — Check remaining content-publishing quota (rolling 24h)

**stories** — Active stories (24h window)

- `instagram-pp-cli stories <ig_user_id>` — List a brand's currently-active stories (expire after 24h)

**tags** — Media you have been tagged in

- `instagram-pp-cli tags <ig_user_id>` — List public media that tag your brand account


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
instagram-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Rank brands by engagement this month

```bash
instagram-pp-cli compare --since 30d --agent
```

One ranked table across every owned account — the view Meta Business Suite never shows.

### Narrow nested media insights for an agent

```bash
instagram-pp-cli media list 17841400000000000 --agent --select data.caption,data.media_product_type,data.like_count,data.comments_count
```

Pass a brand's IG user id (from 'brands list'); media responses are deeply nested, so --select trims to the high-gravity fields and keeps an agent from parsing tens of KB.

### Find when this brand should post

```bash
instagram-pp-cli best-time --account bistro --agent
```

Buckets the brand's own post history by weekday and hour to recommend slots, not generic advice.

### Track a competitor's growth

```bash
instagram-pp-cli rivals --since 30d --agent
```

Diffs accumulated business_discovery snapshots into rival follower/engagement deltas.

### Compare content formats

```bash
instagram-pp-cli formats --account cafe --agent
```

Aggregates Reels vs Feed vs Story vs Carousel including Reels watch-time.

## Auth Setup

Auth is a Meta Graph API access token from a Business-type app on the Facebook-Login path. Set INSTAGRAM_ACCESS_TOKEN to a long-lived user or (recommended) non-expiring system-user token with scopes instagram_basic, instagram_manage_insights, pages_show_list, pages_read_engagement, business_management. The CLI resolves each brand's IG user id from your Pages via /me/accounts and never performs writes unless you explicitly run a publish command. Run 'doctor' to verify token scopes, expiry, and resolved account ids.

Run `instagram-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  instagram-pp-cli account-insights list 17841400000000000 --agent --select data.name,data.period
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

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
instagram-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
instagram-pp-cli feedback --stdin < notes.txt
instagram-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/instagram-pp-cli/feedback.jsonl`. They are never POSTed unless `INSTAGRAM_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `INSTAGRAM_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
instagram-pp-cli profile save briefing --json
instagram-pp-cli --profile briefing account-insights list 17841400000000000
instagram-pp-cli profile list --json
instagram-pp-cli profile show briefing
instagram-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `instagram-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/instagram/cmd/instagram-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add instagram-pp-mcp -- instagram-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which instagram-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   instagram-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `instagram-pp-cli <command> --help`.
