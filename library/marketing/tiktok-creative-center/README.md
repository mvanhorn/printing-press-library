# TikTok Creative Center CLI

**TikTok trending hashtags, viral content, and the Top Ads competitor library — synced to a local store that tells you what to create and what account to build.**

Replays TikTok Creative Center's real endpoints — trending hashtags with popularity curves and audience demographics, representative videos, and the Top Contents (Top Ads) library — and syncs them to SQLite. Then it layers on cross-entity niche briefs, growth-velocity diffing, competitor sweeps, an underserved/viral opportunity score, and a `decide` command that synthesizes all of it into a concrete content + account recommendation. The end goal isn't trends — it's the decision of what to make.

Learn more at [TikTok Creative Center](https://ads.tiktok.com).

Created by [@JTZ18](https://github.com/JTZ18) (Jon).

## Install

The recommended path installs both the `tiktok-creative-center-pp-cli` binary and the `pp-tiktok-creative-center` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install tiktok-creative-center
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install tiktok-creative-center --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install tiktok-creative-center --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install tiktok-creative-center --agent claude-code
npx -y @mvanhorn/printing-press-library install tiktok-creative-center --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/tiktok-creative-center/cmd/tiktok-creative-center-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/tiktok-creative-center-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install tiktok-creative-center --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-tiktok-creative-center --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-tiktok-creative-center --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install tiktok-creative-center --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/tiktok-creative-center-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/tiktok-creative-center/cmd/tiktok-creative-center-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "tiktok-creative-center": {
      "command": "tiktok-creative-center-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Creative Center uses a login session cookie plus an X-CSRFToken header. Run `auth login --chrome` once to capture the session from your logged-in Chrome profile; the CLI replays those cookies and the CSRF token on every request. No official API key exists.

## Quick Start

```bash
# Health check with no network or auth needed
tiktok-creative-center-pp-cli doctor --dry-run


# Capture your Creative Center session from Chrome
tiktok-creative-center-pp-cli auth login --chrome


# See what's trending right now
tiktok-creative-center-pp-cli hashtags list --region US --days 7 --agent


# Mirror trends into the local store for offline + velocity
tiktok-creative-center-pp-cli sync --resources hashtags --param countryCode=US --param timeRange=7


# One-command niche brief
tiktok-creative-center-pp-cli niche "your niche" --region US --agent


# The whole point: what to create + what account to build
tiktok-creative-center-pp-cli decide "your niche" --region US --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Cross-entity research

- **`niche`** — Get a one-command brief for a niche: trending hashtags, their top creators, and representative videos ranked together.

  _When you need to understand a niche fast instead of clicking through five Creative Center pages._

  ```bash
  tiktok-creative-center-pp-cli niche "marvel rivals" --region US --days 7 --agent
  ```
- **`competitor`** — Summarize a competitor's top-performing content and which trending hashtags they ride.

  _When sizing up a competitor's creative strategy and trend positioning in one pass._

  ```bash
  tiktok-creative-center-pp-cli competitor "myketowersmtyk" --region US --agent
  ```
- **`similar`** — Surface similar or co-rising hashtags to one you specify, using stored industries and shared creators.

  _Expand a winning tag into its cluster of related tags._

  ```bash
  tiktok-creative-center-pp-cli similar "marvelrivalss9" --region US --agent
  ```

### Trend intelligence

- **`velocity`** — Measure which tracked hashtags are accelerating by diffing popularity curves across syncs.

  _Spot rising trends before they peak, not after._

  ```bash
  tiktok-creative-center-pp-cli velocity --region US --top 10 --agent
  ```
- **`since`** — Show hashtags and top content that are newly trending since your last sync.

  _Catch what changed overnight without re-reading the whole feed._

  ```bash
  tiktok-creative-center-pp-cli since 24h --region US --agent
  ```
- **`watch`** — Track hashtags and report which crossed a popularity threshold since the last snapshot.

  _Hands-off monitoring of the tags you care about._

  ```bash
  tiktok-creative-center-pp-cli watch add "gaming" --threshold 80
  ```

### Decision intelligence

- **`viral`** — Rank hashtags and content by opportunity = high popularity + low publish count. Find what's rising but not yet saturated — the signal for what to create before everyone piles in.

  _Tells you WHERE the white space is — the single most useful signal for deciding what to create._

  ```bash
  tiktok-creative-center-pp-cli viral --region US --days 7 --top 20 --agent
  ```
- **`content`** — One ranked feed of trending/viral content pulled from the Top Ads library, representative videos in hashtag detail, and top creators' work — across all three in one list.

  _See what's actually going viral in the niche — not just the tags, the content itself._

  ```bash
  tiktok-creative-center-pp-cli content "marvel rivals" --region US --days 7 --agent
  ```
- **`decide`** — Given a niche, output a concrete recommendation: which hashtags to ride, what content formats are working, where the saturation gaps are, how competitors are positioned, and what account angle is still open.

  _This is the actual goal: decide what content and account to create. Every other command feeds this one._

  ```bash
  tiktok-creative-center-pp-cli decide "marvel rivals" --region US --days 7 --agent
  ```

## Recipes


### The decision

```bash
tiktok-creative-center-pp-cli decide "marvel rivals" --region US --days 7 --agent
```

The whole point: a concrete recommendation of what content to make and what account angle is open, synthesized from trends + viral scores + competitors.

### Find white space

```bash
tiktok-creative-center-pp-cli viral --region US --days 7 --top 20 --agent
```

Ranks hashtags by opportunity (high popularity, low publish count) so you create before it saturates.

### See what's going viral

```bash
tiktok-creative-center-pp-cli content "marvel rivals" --region US --days 7 --agent
```

One ranked feed of representative + top-ad + creator videos across the niche.

### Size up a competitor

```bash
tiktok-creative-center-pp-cli competitor "myketowersmtyk" --region US --agent
```

Summarize a competitor's top-performing content and the trending hashtags they ride.

### Narrow verbose output

```bash
tiktok-creative-center-pp-cli hashtags list --region US --agent --select items.hashtagName,items.publishCnt,items.rankIndex
```

Use --select with dotted paths to trim large nested responses for agent context.

### Trend velocity

```bash
tiktok-creative-center-pp-cli velocity --region US --top 10 --agent
```

After two syncs, diff popularity curves to find accelerating hashtags.

## Usage

Run `tiktok-creative-center-pp-cli --help` for the full command reference and flag list.

## Commands

### hashtags

Trending hashtags with popularity curves, publish counts, and top creators

- **`tiktok-creative-center-pp-cli hashtags detail`** - Get a hashtag's popularity curve, audience age/geo profile, and representative videos
- **`tiktok-creative-center-pp-cli hashtags list`** - List trending hashtags for a country and time range

### top-ads

Top Contents (Top Ads) library for competitor research

- **`tiktok-creative-center-pp-cli top-ads list`** - Search the Top Ads / top-performing content library by region, metric, and period
- **`tiktok-creative-center-pp-cli top-ads overview`** - Get Top Ads filter metadata (industries, objectives, regions)

### trends

Trends portal metadata and reference data

- **`tiktok-creative-center-pp-cli trends`** - Trends portal configuration and reference data


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
tiktok-creative-center-pp-cli hashtags list

# JSON for scripting and agents
tiktok-creative-center-pp-cli hashtags list --json

# Filter to specific fields
tiktok-creative-center-pp-cli hashtags list --json --select id,name,status

# Dry run — show the request without sending
tiktok-creative-center-pp-cli hashtags list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
tiktok-creative-center-pp-cli hashtags list --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
tiktok-creative-center-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/tiktok-creative-center-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **StatusCode != 0 or empty items** — Session expired — re-run `auth login --chrome` and retry
- **403 / login redirect** — Log into ads.tiktok.com in Chrome first, then `auth login --chrome`
- **Top Ads returns 'not supported top contents setting'** — Use default period/sort flags; omit unsupported label combinations

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://accounts.google.com/v3/signin/_/AccountsSignInUi/browserinfo
- Capture coverage: 783 API entries from 2047 total network entries
- Reachability: standard_http (65% confidence)
- Protocols: google_batchexecute (95% confidence), rpc_envelope (90% confidence), rest_json (75% confidence)
- Auth signals: api_key — query: key, msToken, tcc_key, token
- Generation hints: has_rpc_envelope, weak_schema_confidence
- Candidate command ideas: create_GetHashtagDetail — Derived from observed POST /CreativeOne/KnowledgeAPI/GetHashtagDetail traffic.; create_GetHashtagList — Derived from observed POST /CreativeOne/KnowledgeAPI/GetHashtagList traffic.; create_abtest_config — Derived from observed POST /service/{service_id}/abtest_config/ traffic.; create_act — Derived from observed POST /api/v2/pixel/act traffic.; create_batch — Derived from observed POST /monitor_browser/collect/batch/ traffic.; create_batchexecute — Derived from observed POST /v3/signin/_/AccountsSignInUi/data/batchexecute traffic.; create_browserinfo — Derived from observed POST /v3/signin/_/AccountsSignInUi/browserinfo traffic.; create_check — Derived from observed POST /api/v3/self_serve/feature_gating/check/ traffic.

Warnings from discovery:
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**davidteather/TikTok-Api**](https://github.com/davidteather/TikTok-Api) — Python (1200 stars)
- [**Apify TikTok Trending Hashtags**](https://apify.com/codebyte/tiktok-trending-hashtags-analytics) — JavaScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
