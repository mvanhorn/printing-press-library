# Naver Blog CLI

**Every Naver Blog feature the OSS ecosystem has learned, in one Go binary — with a local SQLite store, an offline Korean FTS, and likes counts that match the live page exactly without Playwright.**

naver-blog-pp-cli is the read-only CLI for searching, fetching, and aggregating public Naver Blog posts. Phase 1.7 sniffed Naver's public reaction API, so the CLI gets the same byte-for-byte likes count the page renders — no headless browser, no quota, no Naver Developer credentials. A SQLite store and an offline Korean FTS turn one-shot queries into a queryable historical corpus. The `bundle` command runs a multi-query SERP sweep with engagement enrichment in a single binary invocation — wrap it in any scheduler (Hermes, launchd, GitHub Actions) to drive recurring sheet updates.

Learn more at [Naver Blog](https://m.blog.naver.com).

Printed by [@AKL-John](https://github.com/AKL-John).

## Install

The recommended path installs both the `naver-blog-pp-cli` binary and the `pp-naver-blog` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press install naver-blog
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install naver-blog --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press install naver-blog --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press install naver-blog --agent claude-code
npx -y @mvanhorn/printing-press install naver-blog --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/naver-blog-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-naver-blog --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-naver-blog --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-naver-blog skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-naver-blog. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/naver-blog-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "naver-blog": {
      "command": "naver-blog-pp-mcp"
    }
  }
}
```

</details>

## Authentication

No authentication required. The CLI exclusively uses public read-only endpoints: m.blog.naver.com (feed + post HTML), blog.naver.com (PostView.naver fallback), apis.naver.com (reaction API), and m.search.naver.com (search SERP). The Naver Developer API (X-Naver-Client-Id + X-Naver-Client-Secret) is NOT used in v1; if you want to use it as an alternate search backend, set NAVER_CLIENT_ID and NAVER_CLIENT_SECRET (v2 work).

## Quick Start

```bash
# Confirm the CLI can reach all 4 Naver hosts and the local store opens cleanly.
naver-blog-pp-cli doctor


# Fetch one post: title, body, tags, exact likes (via reaction API), comments — JSON.
naver-blog-pp-cli post https://blog.naver.com/selly9401/224234460263 --agent


# List 30 most-recent posts from a blog with engagement metrics inline (one HTTP call).
naver-blog-pp-cli feed selly9401 --limit 30 --agent


# Find Naver Blog posts tagged with BOTH hashtags (intersect).
naver-blog-pp-cli hashtag '칠리,여성청결제' --require-all --agent


# What changed in this post's engagement since last week — using local SQLite history.
naver-blog-pp-cli posts-diff https://blog.naver.com/selly9401/224234460263 --since 168h --agent


# The headline integration: run a query bundle, get a sheet-ready TSV.
naver-blog-pp-cli bundle ~/.config/chilly-queries.yaml --format tsv --enrich-engagement

```

## Unique Features

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

## Usage

Run `naver-blog-pp-cli --help` for the full command reference and flag list.

## Commands

### blogs

Per-blog feeds. The feed JSON endpoint returns 30 recent posts per page, each with sympathyCnt (likes), commentCnt, shareCnt, readCount, briefContents, and addDate — all in one HTTP call.

- **`naver-blog-pp-cli blogs <blog_id>`** - List recent posts from a Naver blog with engagement metrics already populated. Pagination via --page; items array is empty when no more pages.
- **`naver-blog-pp-cli blogs-info <blog_id>`** - Get blog profile metadata including subscribers, daily and total visitors, Power Blog status, directory category, profile image, and cover image.

### find

Online discovery against Naver integrated search (m.search.naver.com). Use 'find posts' for keyword queries, 'find hashtag' for tag queries. Returns ~22 results per page with title and snippet. (Note: the press's built-in 'search' command does offline FTS over the local SQLite cache; this 'find' resource is for live Naver searches.)

- **`naver-blog-pp-cli find hashtag`** - Search Naver Blog posts by hashtag(s). Use 'tag1,tag2' for multiple; --require-all returns only posts containing all tags.
- **`naver-blog-pp-cli find posts`** - Find Naver Blog posts by keyword (live Naver search). Returns mobile blog URLs, title, and snippet. Pair with --agent --select to narrow the output for AI consumers.

### posts

Public post lookups (single post or batch). Returns title, body, tags, likes (공감), comments. Engagement matches the live page exactly via the public reaction API — no Playwright.

- **`naver-blog-pp-cli posts <blog_id> <log_no>`** - Get a single Naver Blog post by blog ID and log number. Extracts title, body, tags from the mobile post page, then queries the reaction API for the like count and PostView.naver for comment count. Output matches what you see in a browser.

### reactions

Public reaction (likes/공감) API. Single REST call returns counts for one or many posts. Same integer Naver renders to the page.

- **`naver-blog-pp-cli reactions`** - Get like/공감 counts for one or more posts. The 'q' parameter is composed as BLOG[blog_id_log_no,blog_id_log_no,...] — the post command builds this automatically; use directly only for batch queries.

### Novel commands (not in spec)

These ship as first-class commands but aren't generated from any spec endpoint — they combine local SQLite state, sniffed APIs, or pipelines you'd otherwise stitch together yourself. Each is also exposed as an MCP tool.

- **`naver-blog-pp-cli posts-diff <url>`** — Engagement delta for a post since its last cached snapshot. Returns likes_delta, comments_delta, and (when no baseline exists) annotates the response with `first snapshot`.
- **`naver-blog-pp-cli posts-comments <url>`** — Fetch the actual comment text for a post via the public cbox endpoint. Flat by default; `--tree` reconstructs nested replies.
- **`naver-blog-pp-cli blogs-health --ids-file <path>`** — Per-blog activity rollup: posts_in_window, median_likes, median_comments, sponsored_ratio (with `--include-sponsored`), days_since_last_post.
- **`naver-blog-pp-cli blogs-info <blog_id>`** — Blog profile and reach: subscriber_count, day_visitor_count, total_visitor_count, power_blog, directory_name, profile/cover images.
- **`naver-blog-pp-cli categories <blog_id>`** — Numbered Naver categories the blog uses, with observed post counts; `--category N` filters feed and search results.
- **`naver-blog-pp-cli bundle <queries.yaml>`** (alias: `cron`) — Run a YAML/JSONL bundle of keyword + hashtag queries, dedupe by canonical URL, optionally enrich with engagement (likes, comments, publish date), emit JSONL/TSV/CSV.
- **`naver-blog-pp-cli query <text>`** — Korean-aware FTS5 over the local SQLite cache. Title, body, tags, nickname; ranked by FTS relevance and likes.
- **`naver-blog-pp-cli neighbors <#tag>`** — Top N hashtags that co-occur with the target tag across the cached corpus.
- **`naver-blog-pp-cli batch --from-file <path>`** — Backfill engagement for a CSV / JSON / newline-delimited file of post URLs in any of the three Naver URL shapes; order-preserving, per-row error JSON.
- **`naver-blog-pp-cli posts --flag-sponsored`** — Mechanically detect KFTC sponsored-disclosure phrases (협찬, 체험단, 광고 포함, 유료광고 포함, 본 포스팅은 ... 받아 작성) and return `sponsored` + matched markers.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
naver-blog-pp-cli posts selly9401 224234460263

# JSON for scripting and agents
naver-blog-pp-cli posts selly9401 224234460263 --json

# Filter to specific fields
naver-blog-pp-cli posts selly9401 224234460263 --json --select title,likes,comment_count

# Plain TSV rows without a header for shell pipelines
naver-blog-pp-cli posts selly9401 224234460263 --plain

# Dry run — show the request without sending
naver-blog-pp-cli posts selly9401 224234460263 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
naver-blog-pp-cli posts selly9401 224234460263 --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Confirmable** - `--yes` for explicit confirmation in scripts and destructive local actions
- **Piped input** - commands that advertise `--stdin` read request or feedback bodies from stdin
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Doctor

```bash
naver-blog-pp-cli doctor
```

`naver-blog-pp-cli doctor` is the health-check command. It replaces the older Health Check label and verifies configuration, API connectivity, and local cache status.

## Cookbook

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

Compares the live engagement to the most recent snapshot in local SQLite older than 24h. Returns the delta only. First run for a post has no baseline and the response is annotated `first snapshot (no baseline at or before cutoff)`.

### Narrow a verbose post payload for an agent

```bash
naver-blog-pp-cli posts selly9401 224234460263 --agent --select url,blog_id,log_no,title,likes,comments,sponsored,published_at
```

Per-post output includes `body_text` (often kilobytes). `--select` narrows to just the fields the agent needs, keeping prompt context lean. Pair with `--flag-sponsored` when scanning a batch.

### Find tag neighborhoods for campaign strategy

```bash
naver-blog-pp-cli neighbors '#칠리여성청결제' --top 20 --agent
```

Returns the top 20 hashtags that co-occur with the target tag across the cached corpus, ranked by joint-occurrence count. The seed tag itself is excluded from the response.

### Audit influencer comment quality

```bash
naver-blog-pp-cli posts-comments https://blog.naver.com/perfect62/224286416663 --all --agent --select user_name,contents,reg_time_utc,sympathy_count
```

Pulls every comment on a post, narrowed to author / body / time / likes so an agent can scan for boilerplate, bot patterns, or genuine engagement. Hidden / secret comments come back with `visible: false` and an empty `contents` field.

### Qualify a blog's reach and Naver tier

```bash
naver-blog-pp-cli blogs-info selly9401 --agent --select blog_id,subscriber_count,day_visitor_count,total_visitor_count,power_blog,directory_name
```

Returns profile-level reach and tier signals for one blog, including subscriber count, daily and total visitors, Power Blog status, Naver directory category, and profile/cover images.

### Korean-aware offline search over the local corpus

```bash
naver-blog-pp-cli query '여성청결제 후기' --limit 20 --agent --select url,title,likes
```

FTS5 over every post ever cached — title, body, tags, nickname — ranked by FTS relevance and likes. No HTTP calls; rerun campaign queries weekly without paying Naver.

### Batch backfill URLs from a Slack paste

```bash
naver-blog-pp-cli batch --from-file slack-bundle.csv --agent
```

Accepts CSV (with a `url` column or `--url-column`), JSON arrays of strings, or newline-delimited URLs in any mix of `m.blog.naver.com`, `blog.naver.com`, and `PostView.naver` shapes. Canonicalizes each, runs concurrent fetches with adaptive rate limiting, preserves input order, emits per-URL error JSON for failures.

## Configuration

Config file: `~/.config/naver-blog-pp-cli/config.toml` (override with `NAVER_BLOG_CONFIG`).

Static request headers can be configured under `headers`; per-command header overrides take precedence.

### Environment variables

| Variable | Default | Purpose |
| --- | --- | --- |
| `NAVER_BLOG_CONFIG` | `~/.config/naver-blog-pp-cli/config.toml` | Override the config file path. |
| `NAVER_BLOG_BASE_URL` | `https://m.blog.naver.com` | Override the base URL used for post fetches (useful for tests / self-hosted mirrors). |
| `NAVER_BLOG_CLI_PATH` | (auto-discovered) | Absolute path to the `naver-blog-pp-cli` binary; consumed by the MCP server when shelling out to novel commands. |
| `NAVER_BLOG_MCP_TRANSPORT` | `stdio` | MCP transport (`stdio` or `http`) when launching `naver-blog-pp-mcp` without `--transport`. |
| `NAVER_BLOG_MCP_ADDR` | `127.0.0.1:7720` | HTTP listen address for `--transport=http`. |
| `NAVER_BLOG_FEEDBACK_ENDPOINT` | (unset) | Optional upstream URL for `naver-blog-pp-cli feedback` submissions. |
| `NAVER_BLOG_FEEDBACK_AUTO_SEND` | `0` | Auto-send feedback when set to `1`/`true` (still requires `NAVER_BLOG_FEEDBACK_ENDPOINT`). |
| `AGENT_ID` | (unset) | Optional tag attached to feedback submissions. |
| `NO_COLOR`, `TERM` | (env) | Respected for color suppression; `TERM=dumb` disables colors. |

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **doctor says 'apis.naver.com unreachable'** — Run `naver-blog-pp-cli doctor --json` to see the failing URL. From Korea this should always work; from non-KR IPs Naver occasionally serves degraded responses — set NAVER_BLOG_USER_AGENT to a Chrome desktop UA and retry.
- **Likes count is 0 when the page shows a higher number** — Your local SQLite has a stale snapshot. Run `posts get <url>` (without --cache) to re-query the reaction API directly. The CLI never uses static-HTML for likes; if 0 persists, file a bug — the post was likely deleted or set to private.
- **search posts returns empty for a query that has results in a browser** — Naver rotates the integrated-search DOM selectors periodically. Use `--debug-html` to dump the SERP HTML, then file an issue with the dump attached. Until a fix ships, run the query through `blogs feed` against any blogs known to post about the topic.
- **post-batch --from-file fails with 'invalid URL'** — Naver Blog URLs come in 3 shapes (blog.naver.com/<id>/<n>, m.blog.naver.com/<id>/<n>, blog.naver.com/PostView.naver?blogId=X&logNo=Y). All three are accepted. If the input is a CSV, the URL column must be named `url` (or use --url-column to specify).
- **Sponsored detector misses an obviously sponsored post** — The detector only matches KFTC-required phrases (협찬, 체험단, 광고 포함, 유료광고 포함, '본 포스팅은 ... 받아 작성됨'). Posts that use creative-but-non-compliant wording (e.g., '협업', 'PR') will not be caught — by design, since those are not legal disclosures.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**snudm/naver-blog-crawler**](https://github.com/snudm/naver-blog-crawler) — Python (30 stars)
- [**krsite-dl/krsite-dl**](https://github.com/krsite-dl/krsite-dl) — Python (7 stars)
- [**baek-labs/hames (naver_blog_scraper.js)**](https://github.com/baek-labs/hames) — JavaScript
- [**tiger-beom/naverblog_scraping**](https://github.com/tiger-beom/naverblog_scraping) — Python
- [**isnow890/naver-search-mcp**](https://github.com/isnow890/naver-search-mcp) — TypeScript
- [**hyunsikhwang/naverblog**](https://github.com/hyunsikhwang/naverblog) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
