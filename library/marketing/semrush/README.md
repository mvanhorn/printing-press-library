# Semrush CLI

Semrush data on tap: Domain Overview, organic keyword research, competitor analysis, and backlink intelligence — JSON-first, agent-native, with a local SQLite cache for offline use.

## Install

The recommended path installs both the `semrush-pp-cli` binary and the `pp-semrush` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install semrush
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install semrush --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/semrush-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-semrush --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-semrush --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-semrush skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-semrush. The skill defines how its required CLI can be installed.
```

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your Semrush API key from the [API units page](https://www.semrush.com/accounts/subscription-info/api-units/). The CLI authenticates by passing the key as the `key` query parameter on every request.

```bash
export SEMRUSH_API_KEY="<paste-your-key>"
```

You can also persist this in your config file at `~/.config/semrush-pp-cli/config.toml`.

### 3. Verify Setup

```bash
semrush-pp-cli doctor
```

This checks your configuration, credentials, and API connectivity.

### 4. Try Your First Commands

```bash
# Domain overview — organic + paid traffic, keyword count, traffic cost
semrush-pp-cli domain overview anthropic.com

# Top organic keywords a domain ranks for
semrush-pp-cli domain organic-keywords anthropic.com --database us

# Keyword research — volume, CPC, KD, competition
semrush-pp-cli keyword overview "claude api"

# Backlink count and Authority Score for a domain
semrush-pp-cli backlinks anthropic.com
```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Compounded keyword intelligence
- **`research`** — Run a full keyword research pass — seeds expand via the Keyword Magic Tool, filter by Personal Keyword Difficulty against your client domain, score by intent + volume + opportunity, dedupe across seeds, and emit a ready-for-Sheets ranked list.

  _When an agent needs to assemble a keyword shortlist for content briefing, this returns a scored, deduped list in one call instead of 5+ chained KMT requests._

  ```bash
  semrush-pp-cli research --seeds "ethical investing,green super" --domain client.com --database au --agent
  ```
- **`keyword-magic`** — Run the Keyword Magic Tool against a seed and get Personal Keyword Difficulty (PKD) scored against your target domain. PKD is not exposed by the public Analytics API.

  _Agents researching content gaps for a specific client need PKD, not the generic Keyword Difficulty the public API returns._

  ```bash
  semrush-pp-cli keyword-magic "seo audit" --domain client.com --database us --mode broad
  ```

### Agent-native delivery
- **`sheets`** — Push Semrush data (keyword research, rankings, gap analysis) directly into your existing Google Sheets client template — no copy/paste, no CSV roundtrip.

  _When an agent runs research and the deliverable is a populated client tracker, this skips the manual hand-off entirely._

  ```bash
  cat research-output.json | semrush-pp-cli sheets push 1AbC...xyz --tab "Keyword Research"
  ```

### Position Tracking automation
- **`pt add-keywords`** — Add one or more keywords to a Semrush Position Tracking campaign, optionally tagging them so they show up grouped in the PT UI. The public Semrush API does not expose Position Tracking write endpoints.

  _When an agent generates a content brief, it can drop the target keywords directly into PT — no manual paste-into-UI step required._

  ```bash
  semrush-pp-cli pt add-keywords 24453414_2945960 --keywords "tax deductible donations,eofy charity" --tags "#articles,eofy-campaign"
  ```
- **`pt rankings`** — Returns the current organic rankings snapshot for every tracked keyword in a Position Tracking campaign — position, position diff, volume, SERP features, traffic estimate. Different from `pt report` which returns rank history.

  _Agents reporting on weekly client performance can fetch the full Overview tab data in one call instead of paginating the rate-limited public Analytics endpoints._

  ```bash
  semrush-pp-cli pt rankings 24453414_2945960 --domain client.com --agent --select data.keywords.keyword,data.keywords.position,data.keywords.volume
  ```
- **`pt annotate`** — Add an annotation (note) to a Position Tracking campaign — appears on the PT chart at the specified date. Lets you correlate site changes (publishes, redirects, algorithm updates) with ranking movements.

  _Agents that schedule publishes can drop the annotation directly into PT, so 8 weeks later the ranking change is clearly tied to the content move._

  ```bash
  semrush-pp-cli pt annotate 24453414_2945960 --title "New EOFY cluster published" --note "3 articles + 1 refreshed page" --date 2026-06-13
  ```

## Usage

Run `semrush-pp-cli --help` for the full command reference and flag list.

## Commands

### Domain analysis

- **`semrush-pp-cli domain overview <domain>`** — Organic + paid traffic, keyword count, traffic cost
- **`semrush-pp-cli domain organic-keywords <domain>`** — Keywords a domain ranks for in organic Google search results
- **`semrush-pp-cli domain organic-competitors <domain>`** — Domains ranking for the same keywords (organic competitors)
- **`semrush-pp-cli domain organic-pages <domain>`** — Top organic landing pages and their ranking keywords

### Keyword research

- **`semrush-pp-cli keyword overview <phrase>`** — Volume, CPC, KD, competition for a single keyword
- **`semrush-pp-cli keyword related <phrase>`** — Related keywords (semantic variants) for a seed phrase
- **`semrush-pp-cli keyword questions <phrase>`** — Question-style keywords (great for content SEO)
- **`semrush-pp-cli keyword difficulty <phrase>`** — Keyword difficulty (KD%) — chance of ranking on SERP page 1

### Backlinks

- **`semrush-pp-cli backlinks <target>`** — Backlink count, referring domains, Authority Score

### Local cache & search

- **`semrush-pp-cli sync`** — Sync API data to local SQLite for offline search and analysis
- **`semrush-pp-cli workflow archive`** — Archive all syncable resources to the local store
- **`semrush-pp-cli workflow status`** — Show local archive status and sync state

### Utilities

- **`semrush-pp-cli doctor`** — Check CLI health (config, credentials, connectivity)
- **`semrush-pp-cli auth`** — Manage authentication for Semrush
- **`semrush-pp-cli profile`** — Named sets of flags saved for reuse
- **`semrush-pp-cli api`** — Browse all API endpoints by interface name
- **`semrush-pp-cli which`** — Find the command that implements a capability

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
semrush-pp-cli domain overview anthropic.com

# JSON for scripting and agents
semrush-pp-cli domain overview anthropic.com --json

# Filter to specific fields
semrush-pp-cli keyword overview "claude api" --json --select Ph,Nq,Cp

# CSV for spreadsheet import
semrush-pp-cli domain organic-keywords anthropic.com --csv

# Dry run — show the request without sending (no API units consumed)
semrush-pp-cli backlinks anthropic.com --dry-run

# Agent mode — JSON + compact + no prompts + no color in one flag
semrush-pp-cli domain overview anthropic.com --agent
```

## Cookbook

```bash
# Compare a domain against its top organic competitors
semrush-pp-cli domain organic-competitors anthropic.com --display-limit 10 --json

# Get the top 100 organic keywords a domain ranks for in the UK database
semrush-pp-cli domain organic-keywords anthropic.com --database uk --display-limit 100

# Pull keyword volume + difficulty for a list of seed phrases
for kw in "claude api" "claude sonnet" "claude opus"; do
  semrush-pp-cli keyword overview "$kw" --json --compact
done

# Find question-style keywords for content briefs
semrush-pp-cli keyword questions "anthropic" --display-limit 50 --json

# Sync data locally, then run repeated searches offline
semrush-pp-cli sync
semrush-pp-cli workflow status

# Preview the API request without sending (no Semrush API units consumed)
semrush-pp-cli backlinks anthropic.com --dry-run --json

# Save a reusable flag profile (e.g., always query the US database with JSON output)
semrush-pp-cli profile save us-json --database us --json
semrush-pp-cli domain overview anthropic.com --profile us-json

# Route output to a file or webhook
semrush-pp-cli domain organic-keywords anthropic.com --json --deliver file:keywords.json
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-semrush -g
```

Then invoke `/pp-semrush <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add semrush semrush-pp-mcp -e SEMRUSH_API_KEY=<your-key>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/semrush-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SEMRUSH_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "semrush": {
      "command": "semrush-pp-mcp",
      "env": {
        "SEMRUSH_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
semrush-pp-cli doctor
```

Sample output:

```
  OK Config: ok
  OK Auth: ok
  OK Env Vars: SEMRUSH_API_KEY present
  OK API: reachable (HTTP 200 at /)
  config_path: ~/.config/semrush-pp-cli/config.toml
  base_url: https://api.semrush.com
  version: 1.0.0
  INFO Cache: ok
    db_path: ~/.local/share/semrush-pp-cli/data.db
    schema_version: 2
    stale_after: 6h0m0s
```

Verifies configuration, credentials, and connectivity to the Semrush API.

## Configuration

Config file: `~/.config/semrush-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SEMRUSH_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting

**Authentication errors (exit code 4)**
- Run `semrush-pp-cli doctor` to check credentials and connectivity
- Verify the environment variable is set: `echo $SEMRUSH_API_KEY`
- Confirm the key is active on the [API units page](https://www.semrush.com/accounts/subscription-info/api-units/)

**Rate limited (exit code 7)**
- Semrush enforces per-second and per-day API unit limits — back off and retry
- Use `--rate-limit <N>` to throttle requests per second
- Run `--dry-run` first to preview a request without consuming API units

**Not found / empty results (exit code 3)**
- Verify the domain or phrase is correct
- Try a different country database with `--database <code>` (us, uk, au, de, fr, etc.)
- Some reports return empty for very small domains or low-volume keywords

**Stale cache**
- The local SQLite store is considered stale after 6 hours by default
- Force a fresh API call with `--data-source live` or rerun `semrush-pp-cli sync`

**Config errors (exit code 10)**
- Default config path: `~/.config/semrush-pp-cli/config.toml`
- Pass `--config <path>` to use a different file
- Run `semrush-pp-cli doctor` for the resolved config path

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
