# Substack CLI

**Run your Substack growth loop from the command line — publish, schedule, engage, and measure with cross-table joins no single Substack API call can.**

Substack has no public API and the closed-source tools that work around it (WriteStack, StackSweller) stop at Notes scheduling and a heatmap. This CLI absorbs every endpoint the community has reverse-engineered across 8 wrappers, then transcends with local-SQLite analytics: per-Note subscriber attribution (`growth attribution`), engagement reciprocity tracking (`engage reciprocity`), and a goal-aware best-time recommender (`growth best-time`). Every command is MCP-callable so an agent can drive the full publish → engage → measure → swap loop.

## Install

The recommended path installs both the `substack-pp-cli` binary and the `pp-substack` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install substack
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install substack --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/substack-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-substack --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-substack --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-substack skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-substack. The skill defines how its required CLI can be installed.
```

## Authentication

Substack uses a session cookie (substack.sid). The only path today is `auth login --chrome` (also accepts `--browser` as an alias) — it reads the cookie from your logged-in Chrome via pycookiecheat / cookies / cookie-scoop-cli and stores it in the OS keyring. There is no password login and no manual cookie-paste subcommand. If your cookie expires, re-run `auth login --chrome`.

## Quick Start

```bash
# Reads your logged-in Substack session out of Chrome and stores it in the OS keyring.
substack-pp-cli auth login --chrome


# Probes all three Substack bases plus the RSS path to surface auth or Cloudflare issues early.
substack-pp-cli doctor


# Pulls posts, drafts, your Notes, comments, profiles, and subscriber-count snapshots into the local store.
substack-pp-cli sync --since 30d


# Dry-run prints the request without firing; drop --dry-run to publish.
substack-pp-cli notes new --body "Stop refreshing the feed. Spend 15 minutes in your inbox replying to commenters and you'll outgrow 90% of writers who don't." --dry-run


# Surfaces which of your last 30 days of Notes brought subs.
substack-pp-cli growth attribution --days 30 --agent --select rank,note_excerpt,subs_acquired


# Ranks candidate publications for a recommendation swap by audience overlap.
substack-pp-cli recs find-partners --my-pub on --top 10 --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`growth attribution`** — Connect every Note you posted to the paid and free subscribers that actually arrived in the 24-hour window after, so you stop guessing which content drove growth.

  _Pick this over a generic stats call when an agent needs to decide which Note formats to repeat next week._

  ```bash
  substack-pp-cli growth attribution --days 30 --json --select rank,note_id,note_excerpt,subs_acquired,paid_subs_acquired
  ```
- **`engage reciprocity`** — See net-give/net-take per writer you engage with — who reciprocates your restacks/comments, who quietly free-rides on yours.

  _Use when an agent is deciding whether to keep investing in a swap partner; surfaces relationships before they go stale._

  ```bash
  substack-pp-cli engage reciprocity --days 30 --agent --select handle,outgoing,incoming,net,drift
  ```

### Algorithm-aware automation
- **`notes schedule --guard`** — Refuse to fire (or queue) a Note that lands less than 30 minutes after your last own-Note or violates your time-of-day rotation. Returns typed exit 2 with a JSON diagnosis.

  _Stops an agent from accidentally torching its own reach by dumping a queue all at once._

  ```bash
  substack-pp-cli notes schedule --at 2026-05-10T13:00:00Z --body "hook line\n\nbody" --guard --json
  ```
- **`growth best-time`** — Top day-of-week × hour cells ranked for whichever growth signal you pick (paid subs, likes, restacks, or comments) — not a single average.

  _An agent picking when to schedule tomorrow's Notes can ask for the goal it's optimizing instead of guessing._

  ```bash
  substack-pp-cli growth best-time --days 90 --for-goal subs --json --select day_of_week,hour,rate,sample_size
  ```

### Pattern intelligence
- **`discover patterns`** — Mechanically extracts which hook patterns (curiosity-gap colon, 3-sentence formula, em-dash reframe, question opener) actually rank in a niche, with restack/comment ratios.

  _An agent drafting Notes can ask which hook shape currently outperforms in this niche before generating._

  ```bash
  substack-pp-cli discover patterns --niche productivity --sort restacks --since 14d --agent --select pattern,sample_count,avg_restacks,avg_comments,top_example
  ```
- **`voice fingerprint`** — Measurable voice profile — sentence length, em-dash rate, colon-hook rate, hook-line ratios, vocabulary uniqueness — for any handle, with --diff to compare against another writer.

  _An agent drafting Notes for a ghostwriter client can verify the output stays inside the client's voice envelope._

  ```bash
  substack-pp-cli voice fingerprint --handle maya --diff devon --json --select metric,self,other,delta
  ```

### Network leverage
- **`recs find-partners`** — Score candidate publications for a Substack Recommendations swap by mutual-overlap density across followee + recommendation graphs.

  _An agent running a weekly cross-promo pass can rank candidates instead of pitching cold._

  ```bash
  substack-pp-cli recs find-partners --my-pub on --top 20 --json --select rank,handle,pub,overlap_score,shared_followees
  ```
- **`growth pod`** — Given a list of handles, render a member × member engagement matrix — last 30 days of restacks/comments/likes between every pair.

  _An agent organizing a mutual-aid pod can see who's net-positive vs free-riding without a spreadsheet._

  ```bash
  substack-pp-cli growth pod --members maya,devon,priya,jordan --days 30 --json
  ```

## Usage

Run `substack-pp-cli --help` for the full command reference and flag list.

## Commands

### categories

Site-wide Substack category list — culture, technology, food, etc.

- **`substack-pp-cli categories list`** - List all Substack categories
- **`substack-pp-cli categories list-publications`** - List publications in a category

### comments

Long-form post comments (distinct from Notes)

- **`substack-pp-cli comments get`** - Get a single comment by ID (same shape as a Note — Substack treats them uniformly)
- **`substack-pp-cli comments list`** - List comments on a post

### discover

Discovery surfaces — search publications, embed metadata

- **`substack-pp-cli discover search-publications`** - Search Substack publications by query

### drafts

Drafts CRUD + publish + schedule

- **`substack-pp-cli drafts create`** - Create a new draft
- **`substack-pp-cli drafts delete`** - Delete a draft
- **`substack-pp-cli drafts get`** - Get a draft by ID
- **`substack-pp-cli drafts list`** - List drafts
- **`substack-pp-cli drafts prepublish`** - Validate a draft for publication; returns blockers
- **`substack-pp-cli drafts publish`** - Publish a draft now
- **`substack-pp-cli drafts schedule`** - Schedule a draft for future publish (or unschedule with --post-date null)
- **`substack-pp-cli drafts update`** - Update an existing draft

### feed

RSS feed for a publication

- **`substack-pp-cli feed rss`** - RSS XML feed (returns XML; use `--raw` to dump)

### images

Image upload (data-URI JSON, not multipart)

- **`substack-pp-cli images upload`** - Upload an image; returns CDN URL. Body is data-URI JSON.

### inbox

Authenticated reader feed (home feed) — Notes + posts surfaced for the current user

- **`substack-pp-cli inbox home`** - Authenticated home feed
- **`substack-pp-cli inbox reader-posts`** - Posts feed for current user

### notes

Substack Notes — short-form posts (Substack treats Notes as comments internally)

- **`substack-pp-cli notes new`** - Post a new Note from Markdown (auto-converts to ProseMirror; the agent-friendly entry point)
- **`substack-pp-cli notes create`** - Post a new Note (POST /comment/feed). Body is raw ProseMirror JSON via `--body-json`.
- **`substack-pp-cli notes schedule`** - Schedule a Note locally with a cadence guard (refuses bursts within 30 min; typed exit 2)
- **`substack-pp-cli notes get`** - Get a single Note by ID
- **`substack-pp-cli notes list-by-profile`** - List Notes by a profile (cursor pagination)
- **`substack-pp-cli notes reply`** - Reply to an existing Note (parent_id + ProseMirror body)

### posts

Long-form posts and archives on a specific publication

- **`substack-pp-cli posts archive`** - Public archive of a publication's posts
- **`substack-pp-cli posts get-by-slug`** - Get a published post by URL slug
- **`substack-pp-cli posts list-published`** - List published posts on the publication (auth required)
- **`substack-pp-cli posts ranked-authors`** - Ranked list of authors for a publication

### profiles

Substack profiles — your own and other writers'

- **`substack-pp-cli profiles from-linkedin`** - Look up a Substack profile from a LinkedIn handle
- **`substack-pp-cli profiles get-by-handle`** - Get a public profile by handle (e.g. mvanhorn)
- **`substack-pp-cli profiles get-by-id`** - Get a public profile by numeric user ID
- **`substack-pp-cli profiles handle-options`** - Available handle suggestions for the current user
- **`substack-pp-cli profiles posts`** - All posts by an author across publications
- **`substack-pp-cli profiles self`** - Get the authenticated user's profile

### recommendations

Substack Recommendations — outbound (publications I recommend)

- **`substack-pp-cli recommendations from`** - List the publications a publication recommends

### sections

Sections of a publication (newsletters can have multiple)

- **`substack-pp-cli sections list`** - List sections + subscriptions

### settings

Account settings + connectivity probe (used by doctor)

- **`substack-pp-cli settings get`** - Get account settings
- **`substack-pp-cli settings ping`** - Connectivity probe (non-destructive PUT used by doctor)

### subs

Subscriber count + publication metadata

- **`substack-pp-cli subs authors`** - List bylined authors of a publication
- **`substack-pp-cli subs count`** - Get subscriber count (read off the launch-checklist payload)

### tags

Post tags

- **`substack-pp-cli tags create`** - Create a new tag
- **`substack-pp-cli tags list`** - List all tags for the publication


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
substack-pp-cli categories list

# JSON for scripting and agents
substack-pp-cli categories list --json

# Filter to specific fields
substack-pp-cli categories list --json --select id,name,status

# Dry run — show the request without sending
substack-pp-cli categories list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
substack-pp-cli categories list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `SUBSTACK_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `substack-pp-cli categories`
- `substack-pp-cli categories list`
- `substack-pp-cli categories list-publications`
- `substack-pp-cli drafts`
- `substack-pp-cli drafts create`
- `substack-pp-cli drafts delete`
- `substack-pp-cli drafts get`
- `substack-pp-cli drafts list`
- `substack-pp-cli drafts prepublish`
- `substack-pp-cli drafts publish`
- `substack-pp-cli drafts schedule`
- `substack-pp-cli drafts update`
- `substack-pp-cli inbox`
- `substack-pp-cli inbox home`
- `substack-pp-cli inbox reader-posts`
- `substack-pp-cli inbox-posts`
- `substack-pp-cli posts`
- `substack-pp-cli posts archive`
- `substack-pp-cli posts get-by-slug`
- `substack-pp-cli posts list-published`
- `substack-pp-cli posts ranked-authors`
- `substack-pp-cli posts-published`
- `substack-pp-cli posts-ranked`
- `substack-pp-cli profiles`
- `substack-pp-cli profiles from-linkedin`
- `substack-pp-cli profiles get-by-handle`
- `substack-pp-cli profiles get-by-id`
- `substack-pp-cli profiles handle-options`
- `substack-pp-cli profiles posts`
- `substack-pp-cli profiles self`
- `substack-pp-cli sections`
- `substack-pp-cli subs`
- `substack-pp-cli subs authors`
- `substack-pp-cli subs count`
- `substack-pp-cli tags`
- `substack-pp-cli tags create`
- `substack-pp-cli tags list`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Runtime Endpoint

This CLI resolves endpoint placeholders at runtime, so one installed binary can target different tenants or API versions without regeneration.

Endpoint environment variables:
- `SUBSTACK_PUBLICATION` resolves `{publication}`

Base URL: `https://substack.com/api/v1`

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-substack -g
```

Then invoke `/pp-substack <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
# Some tools work without auth. For full access, set up auth first:
substack-pp-cli auth login --chrome

claude mcp add substack substack-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
substack-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/substack-current).
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
    "substack": {
      "command": "substack-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
substack-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/substack-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `substack-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized on any write command** — Cookie expired. Run `substack-pp-cli auth login --chrome` to re-import (also aliased as `--browser`).
- **RSS / `posts feed` returns 403 with 'Just a moment...' HTML** — Cloudflare TLS fingerprinting. Run `substack-pp-cli doctor` to confirm; if it reports the RSS leg blocked, retry from a different IP or use `posts archive` (uses the JSON API which Cloudflare doesn't gate as aggressively).
- **Notes posted at the same minute fail or get hidden by the algorithm** — Re-run with `--guard` (default in `notes schedule`); the cadence guard will reject sub-30-min spacing with exit 2 and a JSON diagnosis explaining the violation.
- **`engage like` / `engage restack` printed a curl-equivalent instead of firing** — That's the default — these endpoints aren't in any community wrapper yet, so the CLI prints the request shape so you can preflight it. Add `--send` to actually fire.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**alexferrari88/sbstck-dl**](https://github.com/alexferrari88/sbstck-dl) — Go (216 stars)
- [**NHagar/substack_api**](https://github.com/NHagar/substack_api) — Python (194 stars)
- [**ma2za/python-substack**](https://github.com/ma2za/python-substack) — Python (149 stars)
- [**jakub-k-slys/substack-api**](https://github.com/jakub-k-slys/substack-api) — TypeScript (71 stars)
- [**jakub-k-slys/n8n-nodes-substack**](https://github.com/jakub-k-slys/n8n-nodes-substack) — TypeScript (24 stars)
- [**ty13r/substack-mcp-plus**](https://github.com/ty13r/substack-mcp-plus) — Python
- [**arthurcolle/substack-mcp**](https://github.com/arthurcolle/substack-mcp) — Python
- [**nanameru/substack-mcp**](https://github.com/nanameru/substack-mcp) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
