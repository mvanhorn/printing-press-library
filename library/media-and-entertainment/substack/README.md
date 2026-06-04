# Substack CLI

**Run your Substack growth and authoring loop from the command line — publish rich drafts, manage a multi-publication portfolio, and measure what drives growth.**

Substack has no public API and the closed-source tools that work around it (WriteStack, StackSweller) stop at Notes scheduling and a heatmap. This CLI covers the read endpoints the community has reverse-engineered across the 8 wrappers we studied, plus rich authoring (30+ flags on `drafts create`/`update`, Markdown→ProseMirror conversion), a multi-publication portfolio layer (`portfolio sync` → `portfolio`, `posts best`, `grep`, `schedule board`, `subs churn`, `subs cross-sell`), and local-SQLite analytics. Every command is MCP-callable so an agent can drive the full publish → engage → measure → swap loop.

Created by [@chirantan](https://github.com/chirantan) (Chirantan Rajhans).

## Install

The recommended path installs both the `substack-pp-cli` binary and the `pp-substack` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install substack
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install substack --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install substack --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install substack --agent claude-code
npx -y @mvanhorn/printing-press-library install substack --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack/cmd/substack-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/substack-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-substack --force
```

Inside a Hermes chat session:

```text
/skills install mvanhorn/printing-press-library/cli-skills/pp-substack --force
```

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install substack --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

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


```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack/cmd/substack-pp-mcp@latest
```

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

## Authentication

Substack uses a session cookie (substack.sid). The only path today is `auth login --chrome` (also accepts `--browser` as an alias) — it reads the cookie from your logged-in Chrome via pycookiecheat / cookies / cookie-scoop-cli and stores it in the OS keyring. There is no password login and no manual cookie-paste subcommand. If your cookie expires, re-run `auth login --chrome`.

## Quick Start

```bash
# Health check — confirms your stored Substack session is valid. Capture it first with `substack-pp-cli auth login --chrome`.
substack-pp-cli doctor --dry-run

# Probes all three Substack bases plus the RSS path to surface auth or Cloudflare issues early.
substack-pp-cli doctor

# Pulls posts, drafts, your Notes, comments, profiles, and subscriber-count snapshots into the local store.
substack-pp-cli sync --since 30d

# Bootstrap the portfolio analytics store (run once after login).
export SUBSTACK_PUBLICATION=mypub
substack-pp-cli portfolio sync --json

# One-screen status of every publication you own.
substack-pp-cli portfolio --json

# Dry-run prints the request without firing; drop --dry-run to publish.
substack-pp-cli notes new --body "Stop refreshing the feed. Spend 15 minutes in your inbox replying to commenters and you'll outgrow 90% of writers who don't." --dry-run

# Create a rich draft with Markdown body, paid audience, SEO metadata, and cover image.
# (SUBSTACK_PUBLICATION must already be set)
substack-pp-cli drafts create \
  --title "Why X matters" --body-file ./post.md \
  --audience only_paid --seo-title "X explained" \
  --cover-image https://substackcdn.com/.../cover.jpg --json

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

### Authoring with rich field control

- **`drafts create` / `drafts update`** — Full Substack draft API surface: 30+ flags covering title, subtitle, body (Markdown auto-converts to ProseMirror), section-id, post type (newsletter/podcast/video/thread), audience, bylines, SEO metadata, social title, cover image, comment settings, podcast/video URLs, and visibility toggles. The only authoring path that gives agents field-level control without fighting a web editor.

  _Use when an agent is constructing a complete long-form post from structured data — research summary, translated copy, ghostwritten piece — and needs paywall, SEO, and section placement set in one command._

  ```bash
  # Markdown body, paid-only, with SEO + cover image
  # (SUBSTACK_PUBLICATION must already be set to the target publication subdomain)
  substack-pp-cli drafts create \
    --title "Why X matters" --subtitle "A short analysis" \
    --body-file ./post.md --audience only_paid \
    --seo-title "X explained" --seo-description "How X affects Y" \
    --cover-image https://substackcdn.com/.../cover.jpg --json

  # Update just the title and subtitle of an existing draft
  substack-pp-cli drafts update 12345 \
    --title "New title" --subtitle "New subtitle"
  ```

### Portfolio & Analytics (local columnar store)

These commands read a **local SQLite store** populated by `portfolio sync`. The workflow is:

```
auth login --chrome  →  export SUBSTACK_PUBLICATION=<your-pub>  →  portfolio sync  →  portfolio / posts best / grep / subs churn / …
```

Custom-domain publications are supported: `auth login --chrome` captures the Creator-session cookie from the custom domain automatically.

- **`portfolio sync`** — The data-population command. Discovers every publication you own and writes posts, subscribers, and drafts into the local columnar store. Must be run before the analytics commands below return cross-publication data.

  ```bash
  export SUBSTACK_PUBLICATION=mypub
  substack-pp-cli portfolio sync --json
  ```

- **`portfolio`** — One-screen status of every publication you own: subscriber count, paid count, posts published, drafts pending, next scheduled. No tab-switching, no CSV exports.

  ```bash
  substack-pp-cli portfolio --json
  ```

- **`posts best`** — Rank posts by views, likes, comments, or restacks within a window. `--cross-pub` aggregates across all your publications.

  ```bash
  substack-pp-cli posts best --by restacks --window 30d --cross-pub --json
  substack-pp-cli posts best --by views --limit 5 --publication mypub-en
  ```

- **`posts twin <slug> --to <pub>`** — Duplicate a published post into another publication you own as a draft. Preserves paywall markers, section mapping, and re-uploads images to the target CDN.

  ```bash
  substack-pp-cli posts twin my-en-slug --to mypub-de --dry-run --json
  substack-pp-cli posts twin my-en-slug --to mypub-de
  ```

- **`posts pair <en> <de>` / `posts pairs [--missing]`** — Record EN↔DE post pairings. `--missing` lists posts without a recorded twin — feed that output into `posts twin` to spin up the missing translations.

  ```bash
  substack-pp-cli posts pair my-en-slug my-de-slug
  substack-pp-cli posts pairs --missing --publication mypub-en --json
  ```

- **`grep <query>`** — FTS5 full-text search across synced posts, notes, and comments, ranked by bm25, returning snippets and source URLs.

  ```bash
  substack-pp-cli grep "yield curve" --json
  substack-pp-cli grep "rate hike" --scope posts --publication mypub-en --since 2024-01-01
  ```

- **`schedule board`** — ASCII calendar of the next N days showing scheduled posts across every publication you own. Multi-publication editorial overview in one screen.

  ```bash
  substack-pp-cli schedule board --days 30 --json
  ```

- **`subs churn`** — Diff subscriber snapshots: who newly subscribed, who unsubscribed, who upgraded free→paid, who downgraded paid→free. Run `--snapshot` at least once first.

  ```bash
  substack-pp-cli subs churn --snapshot
  substack-pp-cli subs churn --since 7d --json --publication mypub-paid
  ```

- **`subs cross-sell`** — Emails paid on one of your publications but free or absent on the others. Requires 2+ owned publications in the local store. The cross-sell list Substack's UI does not ship.

  ```bash
  substack-pp-cli subs cross-sell --json --limit 100
  ```

## Recipes

### Daily growth-loop morning ritual

```bash
substack-pp-cli growth attribution --days 7 --agent --select rank,note_excerpt,subs_acquired
```

Surfaces yesterday's Note→sub winners. Pair with `substack-pp-cli engage reciprocity --days 7 --agent` to see whose engagement reciprocates yours, and `substack-pp-cli sync --since 24h` ahead of time to keep the local store fresh.

### Schedule a Note with the cadence guard

```bash
substack-pp-cli notes schedule --at 2030-05-13T09:00:00Z --body 'Tuesday hook line' --guard --json
```

Queues the Note locally; --guard refuses scheduling if it lands within 30 min of an existing own-Note (typed exit 2 + JSON diagnosis). Drop --guard or add --send to fire immediately.

### Find this week's swap partners

```bash
substack-pp-cli recs find-partners --my-pub on --top 5 --json --select rank,handle,pub,overlap_score
```

Ranks candidate publications by audience overlap; pipe to your draft-outreach tool of choice (substack-pp-cli does the ranking; outreach drafting is left to your agent's prompt).

### Capture a writer's voice fingerprint as JSON

```bash
substack-pp-cli voice fingerprint --handle alice --diff bob --json
```

Mechanical voice metrics for the named handle, with a delta against another writer when --diff is set. Save the JSON yourself; agent generation prompts can ingest it.

### Surface deeply nested Note metadata with --select

```bash
substack-pp-cli notes get c-12345 --agent --select id,body,attachments.url,attachments.image_url,attachments.published_bylines.name,attachments.published_bylines.handle,context.users.name
```

Notes responses are deeply nested (attachments, bylines, contextual users). Dotted --select narrows the payload so an agent doesn't burn context parsing 30KB of JSON it doesn't need.

### Bootstrap the portfolio analytics store

```bash
substack-pp-cli auth login --chrome
export SUBSTACK_PUBLICATION=mypub
substack-pp-cli portfolio sync --json
substack-pp-cli portfolio --json
```

Run once after login. Every cross-publication analytics command reads the local store that `portfolio sync` populates.

### Publish a rich draft with SEO and cover image

```bash
export SUBSTACK_PUBLICATION=mypub
substack-pp-cli drafts create \
  --title "The case for X" \
  --subtitle "Three reasons it matters now" \
  --body-file ./post.md \
  --audience only_paid \
  --seo-title "Case for X" \
  --seo-description "Why X matters for Y" \
  --cover-image https://substackcdn.com/.../cover.jpg \
  --json
```

### Twin your best EN post into a DE publication

```bash
# Sync first so the local store has current posts
substack-pp-cli portfolio sync --json

# Find top post by restacks
substack-pp-cli posts best --by restacks --limit 1 --publication mypub-en --json

# Preview, then create the draft
substack-pp-cli posts twin my-en-slug --to mypub-de --dry-run --json
substack-pp-cli posts twin my-en-slug --to mypub-de --json
```

### Weekly subscriber churn digest

```bash
# Run once to set a baseline, then weekly to see movement
substack-pp-cli subs churn --snapshot
substack-pp-cli subs churn --since 7d --json --publication mypub-paid
```

### Full-text search across all your publications

```bash
substack-pp-cli grep "interest rates" --scope posts --json
substack-pp-cli grep "reader question" --scope notes --since 2025-01-01 --limit 20 --json
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

- **`substack-pp-cli discover`** - Search Substack publications by query

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

- **`substack-pp-cli feed`** - RSS XML feed (returns XML; use `--raw` to dump)

### images

Image upload (data-URI JSON, not multipart)

- **`substack-pp-cli images`** - Upload an image; returns CDN URL. Body is data-URI JSON.

### inbox

Authenticated reader feed (home feed) — Notes + posts surfaced for the current user

- **`substack-pp-cli inbox home`** - Authenticated home feed
- **`substack-pp-cli inbox reader-posts`** - Posts feed for current user

### notes

Substack Notes — short-form posts (Substack treats Notes as comments internally)

- **`substack-pp-cli notes create`** - Post a new Note (POST /comment/feed). Body is ProseMirror JSON.
- **`substack-pp-cli notes get`** - Get a single Note by ID
- **`substack-pp-cli notes list-by-profile`** - List Notes by a profile (cursor pagination)
- **`substack-pp-cli notes reply`** - Reply to an existing Note (parent_id + ProseMirror body)

### grep

Full-text search across synced posts, notes, and comments

- **`substack-pp-cli grep <query>`** - FTS5 search ranked by bm25, returning snippets and source URLs. Flags: `--scope posts|notes|comments|all`, `--publication`, `--since`, `--limit`

### portfolio

Multi-publication status dashboard and data-population

- **`substack-pp-cli portfolio`** - One-screen status of every publication you own (subs, paid, posts, drafts, next scheduled). Run `portfolio sync` first.
- **`substack-pp-cli portfolio sync`** - Discover every publication you own and populate the local columnar store (publications/posts/subscribers/drafts). The prerequisite for all cross-publication analytics commands.

### posts

Long-form posts and archives on a specific publication

- **`substack-pp-cli posts archive`** - Public archive of a publication's posts
- **`substack-pp-cli posts best`** - Rank cached posts by engagement metric (`--by views|likes|comments|restacks`, `--window`, `--cross-pub`, `--limit`, `--publication`)
- **`substack-pp-cli posts get-by-slug`** - Get a published post by URL slug
- **`substack-pp-cli posts list-published`** - List published posts on the publication (auth required)
- **`substack-pp-cli posts pair <en-slug> <de-slug>`** - Record an EN↔DE translation pairing in the local table
- **`substack-pp-cli posts pairs`** - List recorded post pairs; `--missing` shows posts without a twin; `--publication` filters to one pub
- **`substack-pp-cli posts ranked-authors`** - Ranked list of authors for a publication
- **`substack-pp-cli posts twin <slug> --to <pub>`** - Duplicate a published post into another publication you own as a draft (re-uploads images, preserves paywall markers)

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

- **`substack-pp-cli recommendations <publication_id>`** - List the publications a publication recommends

### sections

Sections of a publication (newsletters can have multiple)

- **`substack-pp-cli sections`** - List sections + subscriptions

### settings

Account settings + connectivity probe (used by doctor)

- **`substack-pp-cli settings get`** - Get account settings
- **`substack-pp-cli settings ping`** - Connectivity probe (non-destructive PUT used by doctor)

### schedule

Cross-publication editorial scheduling

- **`substack-pp-cli schedule board`** - ASCII calendar of the next N days (`--days`) of scheduled posts across all owned publications

### subs

Subscriber count, churn diff, and cross-sell analytics

- **`substack-pp-cli subs authors`** - List bylined authors of a publication
- **`substack-pp-cli subs churn`** - Diff subscriber snapshots (new/unsubscribed/upgraded/downgraded). Use `--snapshot` to create a baseline, then `--since` to diff. Flags: `--publication`, `--since`, `--snapshot`
- **`substack-pp-cli subs count`** - Get subscriber count (read off the launch-checklist payload)
- **`substack-pp-cli subs cross-sell`** - Emails paid on one publication but free/absent on others (requires 2+ owned pubs). Flags: `--limit`

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
- `substack-pp-cli categories get`
- `substack-pp-cli categories list`
- `substack-pp-cli categories search`
- `substack-pp-cli drafts`
- `substack-pp-cli drafts get`
- `substack-pp-cli drafts list`
- `substack-pp-cli drafts search`
- `substack-pp-cli inbox`
- `substack-pp-cli inbox get`
- `substack-pp-cli inbox list`
- `substack-pp-cli inbox search`
- `substack-pp-cli inbox-posts`
- `substack-pp-cli inbox-posts get`
- `substack-pp-cli inbox-posts list`
- `substack-pp-cli inbox-posts search`
- `substack-pp-cli posts`
- `substack-pp-cli posts get`
- `substack-pp-cli posts list`
- `substack-pp-cli posts search`
- `substack-pp-cli posts-published`
- `substack-pp-cli posts-published get`
- `substack-pp-cli posts-published list`
- `substack-pp-cli posts-published search`
- `substack-pp-cli posts-ranked`
- `substack-pp-cli posts-ranked get`
- `substack-pp-cli posts-ranked list`
- `substack-pp-cli posts-ranked search`
- `substack-pp-cli profiles`
- `substack-pp-cli profiles get`
- `substack-pp-cli profiles list`
- `substack-pp-cli profiles search`
- `substack-pp-cli sections`
- `substack-pp-cli sections get`
- `substack-pp-cli sections list`
- `substack-pp-cli sections search`
- `substack-pp-cli subs`
- `substack-pp-cli subs get`
- `substack-pp-cli subs list`
- `substack-pp-cli subs search`
- `substack-pp-cli tags`
- `substack-pp-cli tags get`
- `substack-pp-cli tags list`
- `substack-pp-cli tags search`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Runtime Endpoint

This CLI resolves endpoint placeholders at runtime, so one installed binary can target different tenants or API versions without regeneration.

Endpoint environment variables:
- `SUBSTACK_PUBLICATION` resolves `{publication}`

Base URL: `https://substack.com/api/v1`

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
- **`engage like` / `engage restack` printed a curl-equivalent instead of firing** — That's the default — these endpoints aren't in any community wrapper, so the CLI prints the request shape so you can preflight it. Add `--send` to actually fire.

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
