# Google Trends CLI

**Every Google Trends workflow pytrends offered, plus the local history and rate-limit resilience that archived, frozen wrapper never had.**

pytrends — the de-facto standard Python wrapper — was archived in April 2025, and Google Trends throttles anonymous access aggressively and unpredictably. This CLI absorbs the full explore-to-widget contract with built-in retry/backoff, caches every query locally so history compounds instead of evaporating, and answers questions the live site simply cannot: what was trending three weeks ago, what changed since your last check, and which related terms are actually worth a content brief.

Learn more at [Google Trends](https://trends.google.com).

Created by [@kmorebetter](https://github.com/kmorebetter) (Kerry Morrison).

## Install

The recommended path installs both the `google-trends-pp-cli` binary and the `pp-google-trends` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install google-trends
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install google-trends --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install google-trends --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install google-trends --agent claude-code
npx -y @mvanhorn/printing-press-library install google-trends --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/google-trends/cmd/google-trends-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/google-trends-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install google-trends --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-google-trends --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-google-trends --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install google-trends --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
google-trends-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/google-trends-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/google-trends/cmd/google-trends-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "google-trends": {
      "command": "google-trends-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Google Trends has no login and no API key. Plain, unfingerprinted HTTP clients (a bare `curl`, Go's default `net/http`) get blocked (HTTP 429) on first contact with the explore/widget API — confirmed live during generation. This CLI's built-in Chrome-TLS-fingerprinted transport gets past that block on its own; no setup step is required. `auth login --chrome` is optional — it imports a clearance cookie from your logged-in Chrome session, which can help if you hit sustained rate-limiting under heavy use, but it is not a prerequisite for normal use.

## Quick Start

```bash
# Confirms the CLI and local database are set up correctly before you make any live calls.
google-trends-pp-cli doctor --dry-run

# The most common call: interest-over-time for a single keyword. Works out of the box, no auth step needed.
google-trends-pp-cli trends interest coffee --geo US --timeframe "today 12-m" --json

# Syncs related/rising search terms locally — this is what powers the local-only commands below.
google-trends-pp-cli trends related "meal prep" --geo US

# A transcendence command: ranks content ideas from what's already synced, no extra API call.
google-trends-pp-cli trends opportunities "meal prep" --agent

```

> **Known gap:** `trends trending` (Google's "Trending Now" daily list) is currently broken — Google's `batchexecute` RPC protocol for this specific surface is undocumented and returns a server-side error frame for this CLI's requests. This is isolated to the Trending Now feature; every other command above (`interest`, `region`, `related`, `geo-gap`, `history search`, `opportunities`) is confirmed working against the live API. See Troubleshooting.

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`trends trending at`** — See what was trending on a specific past date and geo — a question Google Trends' own live UI cannot answer.

  _Reach for this when reconstructing what was trending on a specific past day instead of scraping the live site, which cannot show historical trending snapshots at all._

  ```bash
  google-trends-pp-cli trends trending at --date 2026-06-15 --geo US --agent
  ```
- **`trends history search`** — Full-text search across every related-term and trending-topic result you've ever synced, offline.

  _Use this to check whether a term you tracked weeks ago ever surfaced as related or trending, without re-querying the rate-limited live API._

  ```bash
  google-trends-pp-cli trends history search "electric vehicle" --table related --agent
  ```
- **`trends changes`** — See exactly what changed for a keyword's related terms and interest score since your last sync.

  _Reach for this instead of re-eyeballing a chart when you need to know precisely what's new since last week._

  ```bash
  google-trends-pp-cli trends changes coffee --since 7d --agent
  ```
- **`trends opportunities`** — Ranks a keyword's related/rising terms by rising-momentum times parent-topic growth, surfacing the best content ideas first.

  _Use this when you need a shortlist of content ideas ranked by momentum, not the raw related-queries dump._

  ```bash
  google-trends-pp-cli trends opportunities "meal prep" --agent
  ```

### Computed insight
- **`trends seasonality`** — Computes monthly averages, peak month, and a seasonality strength score for a keyword from cached history.

  _Reach for this to detect a keyword's seasonal pattern from cached history instead of eyeballing a chart by hand._

  ```bash
  google-trends-pp-cli trends seasonality "pumpkin spice" --geo US --agent
  ```
- **`trends geo-gap`** — Ranks the regions where two keywords' interest diverges most, instead of raw per-region values you'd have to compare by hand.

  _Use this for a ranked list of the biggest regional gaps between two keywords, not the raw per-region compare._

  ```bash
  google-trends-pp-cli trends geo-gap nike adidas --resolution REGION --agent
  ```

### Reachability mitigation
- **`trends stale`** — Lists tracked keywords whose local data hasn't been refreshed recently, so you know what to re-sync before you hit a rate limit.

  _Check this before a bulk re-sync so you refresh only what's actually stale, given this API's confirmed rate-limit fragility._

  ```bash
  google-trends-pp-cli trends stale --older-than 14d --agent
  ```

## Recipes

### Compare a brand against two competitors

```bash
google-trends-pp-cli trends interest nike --compare adidas,puma --geo US --timeframe "today 3-m" --json
```

Returns all three keywords on the same 0-100 scale in one call, the shared-scale comparison pytrends required manual payload construction for.

### Find this week's best content ideas from a topic

```bash
google-trends-pp-cli trends opportunities "home workout" --agent
```

Ranks rising related terms by momentum instead of dumping the raw related-queries list.

### See what changed since last week without re-scraping

```bash
google-trends-pp-cli trends changes coffee --since 7d --agent
```

Diffs the last two local snapshots — a question the live site cannot answer since it has no history.

### Pull a large regional breakdown and narrow to the fields that matter

```bash
google-trends-pp-cli trends region coffee --resolution CITY --geo US --agent --select region_name,value
```

City-level breakdowns return dozens of rows with extra metadata; --select trims the response to just the two fields an agent typically needs.

### Check what was trending on a specific past date

```bash
google-trends-pp-cli trends trending at --date 2026-06-01 --geo US --json
```

Answers a question the live Trending Now page cannot — it only ever shows the current window.

## Usage

Run `google-trends-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `GOOGLE_TRENDS_CONFIG_DIR`, `GOOGLE_TRENDS_DATA_DIR`, `GOOGLE_TRENDS_STATE_DIR`, or `GOOGLE_TRENDS_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `GOOGLE_TRENDS_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export GOOGLE_TRENDS_HOME=/srv/google-trends
google-trends-pp-cli doctor
```

Under `GOOGLE_TRENDS_HOME=/srv/google-trends`, the four dirs resolve to `/srv/google-trends/config`, `/srv/google-trends/data`, `/srv/google-trends/state`, and `/srv/google-trends/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "google-trends": {
      "command": "google-trends-pp-mcp",
      "env": {
        "GOOGLE_TRENDS_HOME": "/srv/google-trends"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `GOOGLE_TRENDS_DATA_DIR` overrides an explicit `--home` for that kind. Use `GOOGLE_TRENDS_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `GOOGLE_TRENDS_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `google-trends-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### trends

Core Google Trends explore/widget flow. explore and pickers_* are directly callable; widget_* endpoints require a token minted by a prior explore call (chained by hand-written wrapper commands in Priority 0/1 — see the absorb manifest). All responses are prefixed with Google's XSSI defense string )]}', which must be stripped before JSON decoding.


- **`google-trends-pp-cli trends explore`** - Requests per-widget tokens for a comparison of up to 5 keywords. Response's ['widgets'] array yields {id, token, request} for TIMESERIES, GEO_MAP, and RELATED_QUERIES/TOPICS widgets.
- **`google-trends-pp-cli trends pickers-category`** - Full category id-to-name lookup tree. Openly accessible, no clearance cookie required.
- **`google-trends-pp-cli trends pickers-geo`** - Full geo code-to-name lookup tree (country/region/DMA/city). Openly accessible, no clearance cookie required.
- **`google-trends-pp-cli trends widget-comparedgeo`** - Interest-by-region data for the GEO_MAP widget. req and token must come from a prior explore call's matching widget entry.
- **`google-trends-pp-cli trends widget-multiline`** - Interest-over-time data for the TIMESERIES widget. req and token must come from a prior explore call's matching widget entry.
- **`google-trends-pp-cli trends widget-relatedsearches`** - Related queries and related topics (both top and rising) for the RELATED_QUERIES/RELATED_TOPICS widgets. req and token must come from a prior explore call's matching widget entry.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
google-trends-pp-cli trends explore --hl en-US --tz 42 --req '{}'

# JSON for scripting and agents
google-trends-pp-cli trends explore --hl en-US --tz 42 --req '{}' --json

# Filter to specific fields
google-trends-pp-cli trends explore --hl en-US --tz 42 --req '{}' --json --select id,name,status

# Dry run — show the request without sending
google-trends-pp-cli trends explore --hl en-US --tz 42 --req '{}' --dry-run

# Agent mode — JSON + compact + no prompts in one flag
google-trends-pp-cli trends explore --hl en-US --tz 42 --req '{}' --agent
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
google-trends-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `google-trends-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/google-trends-pp-cli/config.toml`; `--home`, `GOOGLE_TRENDS_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `google-trends-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **HTTP 429 on every call** — Google Trends throttles aggressively and inconsistently. Wait a minute and retry — the CLI's built-in adaptive backoff handles routine cases, but sustained blocking may benefit from running `auth login --chrome` for a clearance cookie.
- **`trends trending` fails with "server returned an error frame"** — Known gap, not a transient failure. The Trending Now page's `batchexecute` RPC is Google's undocumented internal protocol; this CLI sends the same request shape and session tokens a real browser sends, but the server currently returns an error frame instead of trending-terms data. `auth login --chrome` and retrying will not fix this. Every other `trends` command is unaffected and confirmed working live.
- **Related/rising query counts look different from the live website** — This is expected — Google's own server-side sampling means identical queries can return different numbers, especially for low-volume terms and small geographies. Not a CLI bug.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://trends.google.com/trends/explore
- Capture coverage: 29 API entries from 226 total network entries
- Reachability: browser_http (78% confidence)
- Protocols: google_batchexecute (95% confidence), rpc_envelope (90% confidence), rest_json (75% confidence)
- Auth signals: api_key — headers: X-Goog-Api-Key; api_key — query: key, token
- Protection signals: protected_web (75% confidence)
- Generation hints: browser_http_transport, has_rpc_envelope, requires_protected_client, weak_schema_confidence
- Candidate command ideas: create_GetAsyncData — Derived from observed POST /$rpc/google.internal.onegoogle.asyncdata.v1.AsyncDataService/GetAsyncData traffic.; create_batchexecute — Derived from observed POST /_/TrendsUi/data/batchexecute traffic.; create_browserinfo — Derived from observed POST /_/TrendsUi/browserinfo traffic.; create_explore — Derived from observed POST /trends/api/explore traffic.; create_reload — Derived from observed POST /recaptcha/enterprise/reload traffic.; create_startup_config — Derived from observed POST /v1/survey/startup_config traffic.; create_trigger_anonymous — Derived from observed POST /v1/survey/trigger/trigger_anonymous traffic.; create_ubd — Derived from observed POST /recaptcha/api2/ubd traffic.

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

- [**pytrends**](https://github.com/GeneralMills/pytrends) — Python (3700 stars)
- [**google-trends-api**](https://github.com/pat310/google-trends-api) — JavaScript (966 stars)
- [**gogtrends**](https://github.com/groovili/gogtrends) — Go (89 stars)
- [**google-trends-cli**](https://github.com/Nao-30/google-trends-cli) — Python (6 stars)
- [**trendspyg**](https://github.com/flack0x/trendspyg) — Python
- [**pytrends-modern**](https://github.com/yiromo/pytrends-modern) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
