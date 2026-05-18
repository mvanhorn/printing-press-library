# Substack CLI

**Every Substack feature, plus a local SQLite database, full-text search across years of writing, and the only cross-publication portfolio view that exists.**

Manage every Substack you own from one terminal. Sync posts, drafts, notes, comments, and subscribers into a local SQLite store; FTS-search them offline; diff subscriber lists for honest churn reports; spot cross-sell candidates by joining paid and free lists across publications; and twin published posts into a sibling publication as drafts. Built for owners of multiple Substacks — bilingual writers, paid+free tier creators, anyone with more than one newsletter.

## Install

The recommended path installs both the `substack-creator-pp-cli` binary and the `pp-substack-creator` agent skill in one shot:

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
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-substack-creator --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-substack-creator --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-substack-creator skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-substack-creator. The skill defines how its required CLI can be installed.
```

## Authentication

Substack has no API tokens. The CLI reads your `connect.sid` and `substack.sid` cookies from a logged-in Chrome session: run `auth login --chrome` once, and the cookies are saved to `~/.config/substack-creator-pp-cli/config.toml`. When the session expires (every few weeks), log into Substack in your browser and rerun the command. No password, no OTP, no scraped credentials.

## Quick Start

```bash
# Import the connect.sid + substack.sid cookies from your logged-in Chrome session
substack-creator-pp-cli auth login --chrome


# Verify the session works end-to-end against /api/v1/profile
substack-creator-pp-cli doctor


# First-time full sync: posts, drafts, notes, comments, subscribers for every publication you own
substack-creator-pp-cli sync --full


# Cross-publication snapshot — the command the web UI cannot produce
substack-creator-pp-cli portfolio --json


# Named delta of who joined, left, upgraded, downgraded since last sync
substack-creator-pp-cli subscribers churn --since 7d --json


# FTS5 over years of posts + notes + comments
substack-creator-pp-cli grep "yield curve" --scope all --since 2024-01-01

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Multi-publication workflow
- **`portfolio`** — One-screen status of every publication you own: subscriber count, paid count, last-published-at, drafts pending, next scheduled. No tab-switching, no CSV exports.

  _When an agent or human owns multiple Substacks (English + German, free + premium tiers), this is the only command that answers 'what is the state of all of them right now'._

  ```bash
  substack-creator-pp-cli portfolio --json
  ```
- **`posts twin`** — Duplicate a published post into another publication you own as a draft. Preserves paywall markers, sections, and re-uploads images to the target publication's CDN.

  _Bilingual or multi-tier creators copy-paste between publications today. This collapses the ritual into one command and leaves the target draft ready for translation or pricing-tier adjustment._

  ```bash
  substack-creator-pp-cli posts twin my-post-slug --to mypub-de --dry-run
  ```
- **`posts pairs`** — Record EN<->DE post pairings in a local table. 'posts pair <en> <de>' adds; 'posts pairs --missing' lists posts in one language with no recorded twin in the other.

  _Bilingual newsletter owners forget which posts already have a translation. This command answers it offline and feeds the 'posts twin' flow._

  ```bash
  substack-creator-pp-cli posts pairs --missing --publication mypub-en --json
  ```
- **`schedule board`** — ASCII calendar of the next 30 days showing scheduled posts across every publication you own. Multi-pub editorial overview in one screen.

  _Editorial planning across publications needs one calendar. This is the only command that renders it._

  ```bash
  substack-creator-pp-cli schedule board --json
  ```

### Local state that compounds
- **`subscribers churn`** — Diff two SQLite snapshots of your subscriber list: who newly subscribed, who unsubscribed, who upgraded free->paid, who downgraded paid->free, since a chosen window.

  _Agents auditing retention want named churn rows, not aggregate counts. Sunday-evening review or weekly automation reads this list and pipes it forward._

  ```bash
  substack-creator-pp-cli subscribers churn --publication mypub-paid --since 7d --json
  ```
- **`subscribers cross-sell`** — SQL join across your publications' subscriber lists: emails paid on one publication but free or absent on the others, sorted by paid-publication coverage. The cross-sell list Substack does not ship.

  _The most obvious upsell candidates are paying readers on one of your newsletters who don't know your other ones exist. This command surfaces them for a once-a-month email blast._

  ```bash
  substack-creator-pp-cli subscribers cross-sell --json
  ```
- **`posts best`** — Rank posts by views, likes, comments, or restacks within a window. Optionally aggregate across every publication you own to find your overall top performer.

  _For repurposing decisions, you need the best post across the portfolio, not within one pub. This is the input for Monday-morning content planning._

  ```bash
  substack-creator-pp-cli posts best --by restacks --window 30d --cross-pub --json
  ```
- **`grep`** — FTS5 over post bodies + Notes + comments, ranked by bm25, returning snippets and source URLs. Optional scope (posts/notes/comments/all), publication, and since filter.

  _Agents and writers re-citing their own writing need full-archive search across years. Substack cannot do this; this CLI ships it as a one-liner._

  ```bash
  substack-creator-pp-cli grep "yield curve" --scope all --since 2024-01-01 --json
  ```

## Usage

Run `substack-creator-pp-cli --help` for the full command reference and flag list.

## Commands

### categories

Substack content categories.

- **`substack-creator-pp-cli categories list`** - List all global categories.
- **`substack-creator-pp-cli categories newsletters`** - List newsletters in a category.

### comments

Comments on posts.

- **`substack-creator-pp-cli comments add`** - Add a comment to a post.
- **`substack-creator-pp-cli comments list`** - List comments on a post.
- **`substack-creator-pp-cli comments react`** - React to a comment.

### dashboard

Publication analytics and engagement stats.

- **`substack-creator-pp-cli dashboard stats`** - Aggregate dashboard stats for a publication you own.

### drafts

Manage post drafts.

- **`substack-creator-pp-cli drafts create`** - Create a new draft.
- **`substack-creator-pp-cli drafts delete`** - Delete a draft.
- **`substack-creator-pp-cli drafts get`** - Get a draft by ID.
- **`substack-creator-pp-cli drafts list`** - List your drafts.
- **`substack-creator-pp-cli drafts preview`** - Get an author-only preview link for a draft.
- **`substack-creator-pp-cli drafts publish`** - Publish a draft.
- **`substack-creator-pp-cli drafts update`** - Update an existing draft.

### feed

Your reader feed.

- **`substack-creator-pp-cli feed list`** - Get your feed.

### images

Upload images to Substack's CDN.

- **`substack-creator-pp-cli images upload`** - Upload an image.

### me

Your own subscriptions, follows, and personal recommendations.

- **`substack-creator-pp-cli me follows`** - Profiles you follow.
- **`substack-creator-pp-cli me recommendations`** - Personal recommendations.
- **`substack-creator-pp-cli me subscriptions`** - What you subscribe to.

### notes

Substack Notes (microblog).

- **`substack-creator-pp-cli notes list`** - List your recent notes.
- **`substack-creator-pp-cli notes publish`** - Publish a new note.
- **`substack-creator-pp-cli notes react`** - React to a note.
- **`substack-creator-pp-cli notes reply`** - Reply to a note.
- **`substack-creator-pp-cli notes restack`** - Restack a note.

### posts

Read and interact with your published posts.

- **`substack-creator-pp-cli posts get`** - Get a single post by slug.
- **`substack-creator-pp-cli posts list`** - List your own posts (drafts + published).
- **`substack-creator-pp-cli posts react`** - React to a post (heart it).
- **`substack-creator-pp-cli posts restack`** - Restack a post to your Notes.
- **`substack-creator-pp-cli posts stats`** - Engagement stats (likes/comments/restacks) for a post.

### profiles

Read your own or another user's Substack profile.

- **`substack-creator-pp-cli profiles get`** - Get another user's profile by handle.
- **`substack-creator-pp-cli profiles me`** - Get your own profile and publications.

### publications

Search and inspect Substack publications globally.

- **`substack-creator-pp-cli publications recommendations`** - List publications recommended by a given publication.
- **`substack-creator-pp-cli publications search`** - Search publications by query.

### sections

Publication sections / categories.

- **`substack-creator-pp-cli sections list`** - List sections for a publication.

### subscribers

Manage your publication's subscribers.

- **`substack-creator-pp-cli subscribers add`** - Add a subscriber by email.
- **`substack-creator-pp-cli subscribers count`** - Get total subscriber counts (free + paid).
- **`substack-creator-pp-cli subscribers export_free`** - Export free subscribers as CSV.
- **`substack-creator-pp-cli subscribers export_paid`** - Export paid subscribers as CSV.
- **`substack-creator-pp-cli subscribers list`** - List subscribers for a publication you own.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
substack-creator-pp-cli categories list

# JSON for scripting and agents
substack-creator-pp-cli categories list --json

# Filter to specific fields
substack-creator-pp-cli categories list --json --select id,name,status

# Dry run — show the request without sending
substack-creator-pp-cli categories list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
substack-creator-pp-cli categories list --agent
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-substack-creator -g
```

Then invoke `/pp-substack-creator <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
# Some tools work without auth. For full access, set up auth first:
substack-creator-pp-cli auth login --chrome

claude mcp add substack substack-creator-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
substack-creator-pp-cli auth login --chrome
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
      "command": "substack-creator-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
substack-creator-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/substack-creator-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `substack-creator-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **`auth status` reports session expired** — Log into Substack in your Chrome browser and rerun `substack-creator-pp-cli auth login --chrome` to refresh cookies
- **`drafts publish` returns 404 or 422** — Substack's scheduled-publish endpoint changed in 2025; use `drafts publish <id>` for immediate publish and verify the draft state with `drafts get <id> --json` first
- **Rate-limited with HTTP 429 from list endpoints** — Lower the request rate: set `[network] rate_per_sec = 1` in `~/.config/substack-creator-pp-cli/config.toml` — Substack's empirical limit is around 2/s
- **`posts twin --to <pub>` fails on image upload** — Image uploads must go to the target publication's subdomain. Run `auth status` to confirm the cookie is valid for that subdomain
- **`portfolio` shows wrong subscriber counts** — Run `sync --full` to refresh — the SQLite cache is updated only on sync

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Capture coverage: 0 API entries from 0 total network entries
- Reachability: standard_http (0% confidence)

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**jakub-k-slys/substack-api**](https://github.com/jakub-k-slys/substack-api) — TypeScript (72 stars)
- [**postcli/substack**](https://github.com/postcli/substack) — TypeScript
- [**ty13r/substack-mcp-plus**](https://github.com/ty13r/substack-mcp-plus) — Python
- [**NHagar/substack_api**](https://github.com/NHagar/substack_api) — Python
- [**alexferrari88/sbstck-dl**](https://github.com/alexferrari88/sbstck-dl) — Go
- [**alvarolorentedev/substack-cli**](https://github.com/alvarolorentedev/substack-cli) — JavaScript
- [**marcomoauro/substack-mcp**](https://github.com/marcomoauro/substack-mcp) — JavaScript
- [**anshulkhare7/substack-cli**](https://github.com/anshulkhare7/substack-cli) — JavaScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
