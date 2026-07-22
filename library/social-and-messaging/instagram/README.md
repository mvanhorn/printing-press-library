# Instagram CLI

**Every Instagram Business metric across all your brand accounts, with the cross-account comparison and follower-growth history Meta Business Suite never gives you.**

A local-first analytics CLI for the Instagram Graph API built for managers who run several owned Business/Creator accounts. It syncs accounts, media, insights, and competitor snapshots into a queryable SQLite store, then adds the views the official tools lack: rank your brands side by side (compare), follower-growth over time (growth), best-time-to-post, format breakdowns, and competitor deltas. Agent-native output, offline search, and typed exit codes throughout.

## Install

The recommended path installs both the `instagram-pp-cli` binary and the `pp-instagram` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install instagram
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install instagram --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install instagram --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install instagram --agent claude-code
npx -y @mvanhorn/printing-press-library install instagram --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/instagram/cmd/instagram-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/instagram-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-instagram --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-instagram --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-instagram skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-instagram. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/instagram-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `INSTAGRAM_ACCESS_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/instagram/cmd/instagram-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "instagram": {
      "command": "instagram-pp-mcp",
      "env": {
        "INSTAGRAM_ACCESS_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Auth is a Meta Graph API access token from a Business-type app on the Facebook-Login path. Set INSTAGRAM_ACCESS_TOKEN to a long-lived user or (recommended) non-expiring system-user token with scopes instagram_basic, instagram_manage_insights, pages_show_list, pages_read_engagement, business_management. The CLI resolves each brand's IG user id from your Pages via /me/accounts and never performs writes unless you explicitly run a publish command. Run 'doctor' to verify token scopes, expiry, and resolved account ids.

## Quick Start

```bash
# Confirm the binary works before wiring auth (no token needed).
instagram-pp-cli doctor --dry-run

# Auto-register the Instagram Business accounts linked to your Pages as brands.
instagram-pp-cli brands discover

# Collect each brand's profile, insights, and media into the local snapshot store.
instagram-pp-cli pull

# Rank your brands by engagement over the collected window.
instagram-pp-cli compare --since 30d --agent

# Find the highest-reach posts across all brands.
instagram-pp-cli top-posts --since 30d --metric reach --agent

```

## Unique Features

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

Media responses are deeply nested; --select trims to the high-gravity fields so an agent isn't parsing tens of KB.

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

## Usage

Run `instagram-pp-cli --help` for the full command reference and flag list.

## Commands

### account-insights

Account-level insights (reach, views, interactions, demographics)

- **`instagram-pp-cli account-insights demographics`** - Lifetime follower demographics broken down by age, gender, city, or country
- **`instagram-pp-cli account-insights list`** - Account insights over a window (reach, accounts_engaged, total_interactions, views)

### accounts

Brand account profiles (Instagram Business/Creator users linked to your Pages)

- **`instagram-pp-cli accounts get`** - Get a brand account profile (followers, follows, media counts, bio)
- **`instagram-pp-cli accounts pages`** - List the Facebook Pages you manage and their linked Instagram Business account ids

### business-discovery

Public metrics for any business/creator account (competitor research)

- **`instagram-pp-cli business-discovery <ig_user_id>`** - Fetch a competitor's public data via a business_discovery field expression

### comments

Comments and replies on media

- **`instagram-pp-cli comments list`** - List comments on a media object
- **`instagram-pp-cli comments replies`** - List replies to a comment

### hashtags

Hashtag search and top/recent media

- **`instagram-pp-cli hashtags recent-media`** - Recent public media for a hashtag id
- **`instagram-pp-cli hashtags search`** - Resolve a hashtag string to its id (limited to 30 unique tags per account / 7 days)
- **`instagram-pp-cli hashtags top-media`** - Top-performing public media for a hashtag id

### media

Posts, reels, and per-media insights

- **`instagram-pp-cli media create`** - Create a media container (step 1 of publishing). Analytics-first CLI; use --dry-run.
- **`instagram-pp-cli media get`** - Get a single media object by id
- **`instagram-pp-cli media insights`** - Per-media insights (reach, views, saved, shares, interactions; Reels watch-time)
- **`instagram-pp-cli media list`** - List a brand's media (posts, reels, carousels) newest-first
- **`instagram-pp-cli media publish`** - Publish a previously created media container (step 2).
- **`instagram-pp-cli media publish-limit`** - Check remaining content-publishing quota (rolling 24h)

### stories

Active stories (24h window)

- **`instagram-pp-cli stories <ig_user_id>`** - List a brand's currently-active stories (expire after 24h)

### tags

Media you have been tagged in

- **`instagram-pp-cli tags <ig_user_id>`** - List public media that tag your brand account


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
instagram-pp-cli account-insights list 17841400000000000

# JSON for scripting and agents
instagram-pp-cli account-insights list 17841400000000000 --json

# Filter to specific fields
instagram-pp-cli account-insights list 17841400000000000 --json --select data.name,data.period

# Dry run — show the request without sending
instagram-pp-cli account-insights list 17841400000000000 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
instagram-pp-cli account-insights list 17841400000000000 --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
instagram-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/instagram-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `INSTAGRAM_ACCESS_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `instagram-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `instagram-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $INSTAGRAM_ACCESS_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **OAuthException 'active access token must be used' (code 2500)** — Set INSTAGRAM_ACCESS_TOKEN; run 'instagram-pp-cli doctor' to verify the token resolves.
- **Metric returns empty where you expected impressions** — Use 'views' — Graph deprecated impressions/plays/video_views in 2025; the CLI requests 'views' by default.
- **growth or rivals returns an empty series on a fresh install** — These read accrued snapshots; run 'pull' on a schedule first so the local time-series can build up.
- **HTTP 4xx with x-business-use-case-usage near 100%** — You hit the Business Use Case rate limit; wait for estimated_time_to_regain_access (doctor prints it) or pull fewer brands.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**facebook-nodejs-business-sdk**](https://github.com/facebook/facebook-nodejs-business-sdk) — JavaScript
- [**instagram-analytics-mcp**](https://github.com/BilalTariq01/instagram-analytics-mcp) — Python
- [**ig-mcp**](https://github.com/jlbadano/ig-mcp) — TypeScript
- [**instagram-mcp**](https://github.com/AleemHaider/instagram-mcp) — Python
- [**instagram-api-go-client**](https://github.com/qcserestipy/instagram-api-go-client) — Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
