---
name: pp-naver-blog
description: "Every Naver Blog feature the OSS ecosystem has learned, in one Go binary — with a local SQLite store, an offline... Trigger phrases: `search naver blog for`, `get naver blog post engagement`, `네이버 블로그 검색`, `칠리 블로그 인플루언서 운영 결과`, `monthly chilly naver blog data`, `use naver-blog`, `run naver-blog`."
author: "user"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - naver-blog-pp-cli
---

# Naver Blog — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `naver-blog-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install naver-blog --cli-only
   ```
2. Verify: `naver-blog-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/cmd/naver-blog-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

The naver-blog-pp-cli binary provides read-only search, fetch, and aggregation for public Naver Blog posts. Phase 1.7 sniffed Naver's public reaction API, so the CLI gets the same byte-for-byte likes count the page renders — no headless browser, no quota, no Naver Developer credentials. A SQLite store and an offline Korean FTS turn one-shot queries into a queryable historical corpus. The `bundle` command runs a multi-query SERP sweep with engagement enrichment in a single binary invocation — wrap it in any scheduler (Hermes, launchd, GitHub Actions) to drive recurring sheet updates.

## When to Use This CLI

Reach for naver-blog-pp-cli when an agent is asked to search Korean Naver Blog posts by keyword or hashtag, extract a single post's title/body/tags/engagement, or aggregate engagement across a list of post URLs. It is the right choice when accuracy matters (the reaction API gives exact same-instant counts), when the workflow runs weekly or more often (the SQLite cache compounds), or when the Naver Developer API's 25k-request daily quota or credential requirement is a friction. It is NOT the right choice for write workflows (publishing, editing, deleting posts) or for authenticated-only data (admin stats, 이웃 neighbor lists, drafts) — both are explicitly out of scope.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`posts-diff`** — See exactly which posts gained likes or comments since you last checked, without re-scraping everything.

  _When an agent is asked 'which Chilly posts moved this week', this is the one-call answer — no full re-crawl._

  ```bash
  naver-blog-pp-cli posts-diff https://blog.naver.com/selly9401/224234460263 --since 24h --agent
  ```
- **`blogs-health`** — Given a file of blog IDs, returns posts-this-week, median likes, median comments, sponsored ratio, and days-since-last-post — one command, every influencer.

  _Replaces a Friday-afternoon Notion-doc-writing ritual with a deterministic JSON._

  ```bash
  naver-blog-pp-cli blogs-health --ids-file ~/.config/influencers.txt --window 7d --agent
  ```
- **`query`** — Korean-aware FTS5 over every post ever cached — title, body, tags, nickname — ranked by FTS relevance and likes.

  _Rerun the same campaign query weekly without paying for new Naver fetches._

  ```bash
  naver-blog-pp-cli query '여성청결제 후기' --limit 20 --agent --select url,title,likes
  ```

### Korean content-pattern intelligence
- **`posts --flag-sponsored`** — Mechanically detects KFTC-required sponsored-post disclosure phrases in the body — 협찬, 체험단, 광고 포함, 유료광고 포함 — and returns a sponsored boolean plus the matched markers.

  _Distinguishes genuine reviews from paid placements for influencer-audit workflows._

  ```bash
  naver-blog-pp-cli posts selly9401 224234460263 --flag-sponsored --agent --select sponsored,sponsored_markers,title
  ```
- **`neighbors`** — For a target hashtag, returns the top N hashtags that travel with it across the cached corpus — the campaign 'neighborhood'.

  _Strategy signal for campaign hashtag expansion._

  ```bash
  naver-blog-pp-cli neighbors '#칠리여성청결제' --top 20 --agent
  ```
- **`categories`** — Lists a blog's numbered Naver categories with post counts; --category flag filters feed/local-search results by category number.

  _Topic-segment an influencer (e.g., only their beauty posts) without crawling everything._

  ```bash
  naver-blog-pp-cli categories selly9401 --agent
  ```
- **`posts-comments`** — Fetch the actual content of comments on a Naver Blog post — author, body, timestamps, like counts — via the public cbox endpoint. Optional flat or nested-tree output.

  _When auditing influencer engagement quality, the content of comments matters as much as the count — bot comments, identical-template spam, or genuine engagement._

  ```bash
  naver-blog-pp-cli posts-comments https://blog.naver.com/perfect62/224286416663 --agent
  ```
- **`blogs-info`** — Get rich blog profile data: subscriber count, daily/total visitor counts, Naver Power Blog status, directory category, profile/cover images — everything Naver shows on the blog homepage.

  _Influencer-marketing audits need to qualify creators by reach (subscribers, daily visitors) and Naver-assigned tier badges. This is the only command that gives both._

  ```bash
  naver-blog-pp-cli blogs-info selly9401 --agent
  ```

### Headline integration
- **`bundle`** — Runs a list of keyword/hashtag queries from a YAML/JSONL file, enriches every result with engagement, dedupes by canonical URL, and streams a sheet-ready TSV — one binary replaces the Hermes web_search + web_extract + Playwright pipeline.

  _The Monday-morning Chilly Hermes cron becomes one CLI invocation with no Chromium dependency._

  ```bash
  naver-blog-pp-cli bundle ~/.config/chilly-queries.yaml --format tsv
  ```

### Agent-native plumbing
- **`batch`** — Accepts CSV / JSON column / newline-delimited URLs in any mix of m.blog.naver.com / blog.naver.com / PostView.naver shapes; canonicalizes each; runs concurrent fetches with adaptive rate limiting; preserves input order; emits per-URL error JSON for failures.

  _Slack-bundle workflow: paste a column of URLs, get a column of engagement._

  ```bash
  naver-blog-pp-cli batch --from-file slack-bundle.csv --agent
  ```

## Command Reference

**blogs** — Per-blog feeds. The feed JSON endpoint returns 30 recent posts per page, each with sympathyCnt (likes), commentCnt, shareCnt, readCount, briefContents, and addDate — all in one HTTP call.

- `naver-blog-pp-cli blogs <blog_id>` — List recent posts from a Naver blog with engagement metrics already populated. Pagination via --page; items array is...
- `naver-blog-pp-cli blogs-info <blog_id>` — Get blog profile metadata including subscribers, daily and total visitors, Power Blog status, directory category, profile image, and cover image.

**find** — Online discovery against Naver integrated search (m.search.naver.com). Use 'find posts' for keyword queries, 'find hashtag' for tag queries. Returns ~22 results per page with title and snippet. (Note: the press's built-in 'search' command does offline FTS over the local SQLite cache; this 'find' resource is for live Naver searches.)

- `naver-blog-pp-cli find hashtag` — Search Naver Blog posts by hashtag(s). Use 'tag1,tag2' for multiple; --require-all returns only posts containing all...
- `naver-blog-pp-cli find posts` — Find Naver Blog posts by keyword (live Naver search). Returns mobile blog URLs, title, and snippet. Pair with...

**posts** — Public post lookups (single post or batch). Returns title, body, tags, likes (공감), comments. Engagement matches the live page exactly via the public reaction API — no Playwright.

- `naver-blog-pp-cli posts <blog_id> <log_no>` — Get a single Naver Blog post by blog ID and log number. Extracts title, body, tags from the mobile post page, then...

**reactions** — Public reaction (likes/공감) API. Single REST call returns counts for one or many posts. Same integer Naver renders to the page.

- `naver-blog-pp-cli reactions` — Get like/공감 counts for one or more posts. The 'q' parameter is composed as...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
naver-blog-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Drive a scheduled sheet update from a query bundle

```bash
naver-blog-pp-cli bundle ~/.config/chilly-queries.yaml --format tsv --enrich-engagement
```

Runs the queries listed in the YAML file, enriches with engagement (likes, comments, publish date), dedupes by canonical URL, and streams TSV on stdout for Google Sheets ingestion. Pipe to `pbcopy`, save to a file, or feed an `append_*.py` style helper. Scheduling lives in the runner (Hermes / launchd / GitHub Actions / cron) wrapping this command. Legacy alias `cron` is retained for backward compat with existing wrappers.

### Friday influencer health roll-up

```bash
naver-blog-pp-cli blogs-health --ids-file ~/.config/influencers.txt --window 7d --agent
```

For each blog ID in the file, returns posts-this-week, median engagement, sponsored ratio (with `--include-sponsored`), days-since-last-post.

### Find what changed in a watched post

```bash
naver-blog-pp-cli posts-diff https://blog.naver.com/selly9401/224234460263 --since 24h --agent
```

Compares the live engagement to the most recent snapshot in local SQLite older than 24h. Returns the delta only.

### Narrow a verbose post payload for an agent (--agent + --select)

```bash
naver-blog-pp-cli post https://blog.naver.com/selly9401/224234460263 --agent --select url,blog_id,log_no,title,likes,comments,sponsored,published_at
```

Per-post output includes body_text (often kilobytes). --select narrows to just the fields the agent needs, keeping prompt context lean. Pair with --flag-sponsored when scanning a batch.

### Find tag neighborhoods for campaign strategy

```bash
naver-blog-pp-cli neighbors '#칠리여성청결제' --top 20 --agent
```

Returns the top 20 hashtags that co-occur with the target tag across the cached corpus, ranked by joint-occurrence count.

### Audit influencer comment quality

```bash
naver-blog-pp-cli posts-comments https://blog.naver.com/perfect62/224286416663 --all --agent --select user_name,contents,reg_time_utc,sympathy_count
```

Pulls every comment on a post, narrowed to author/body/time/likes for an agent to scan for boilerplate or bot patterns.

### Qualify a blog's reach and Naver tier

```bash
naver-blog-pp-cli blogs-info selly9401 --agent --select blog_id,subscriber_count,day_visitor_count,total_visitor_count,power_blog,directory_name
```

Returns profile-level reach and tier signals for one blog, including subscriber count, daily/total visitors, Power Blog status, Naver directory category, and profile/cover images.

## Auth Setup

No authentication required.

Run `naver-blog-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  naver-blog-pp-cli posts mock-value mock-value --agent --select id,name,status
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
naver-blog-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
naver-blog-pp-cli feedback --stdin < notes.txt
naver-blog-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.naver-blog-pp-cli/feedback.jsonl`. They are never POSTed unless `NAVER_BLOG_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `NAVER_BLOG_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
naver-blog-pp-cli profile save briefing --json
naver-blog-pp-cli --profile briefing posts mock-value mock-value
naver-blog-pp-cli profile list --json
naver-blog-pp-cli profile show briefing
naver-blog-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `naver-blog-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add naver-blog-pp-mcp -- naver-blog-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which naver-blog-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   naver-blog-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `naver-blog-pp-cli <command> --help`.
