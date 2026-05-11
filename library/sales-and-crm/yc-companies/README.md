# Y Combinator CLI

**Every YC-backed company in a local database, with watch lists, deltas, and cross-batch stats no scraper has.**

Sync the full Y Combinator company directory into a local SQLite store, then filter across batch, industry, tag, status, region, and team size in one query. Watch a portfolio and see what changed between syncs. Compute cross-batch aggregates the live site cannot show.

## Install

The recommended path installs both the `yc-companies-pp-cli` binary and the `pp-yc-companies` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install yc-companies
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install yc-companies --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/yc-companies-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-yc-companies --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-yc-companies --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-yc-companies skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-yc-companies. The skill defines how its required CLI can be installed.
```

## Quick Start

```bash
# Pull the latest YC directory (5,889 companies, ~5 MB) into local SQLite.
yc-companies-pp-cli sync


# Multi-axis filters work offline; pipe to jq.
yc-companies-pp-cli companies list --batch w25 --industry fintech --json


# Track a list of slugs locally.
yc-companies-pp-cli watch add stripe airbnb doordash


# Surface team_size / status / hiring deltas between snapshots.
yc-companies-pp-cli watch diff --since 2026-04-01 --agent


# Peer discovery via tag overlap.
yc-companies-pp-cli companies similar stripe --limit 10 --json --select slug,name,score


# Cross-batch industry pivot — count, avg team size, % hiring.
yc-companies-pp-cli stats by-batch --industry ai --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`watch add`** — Track a personal set of YC companies and see what changes between syncs.

  _Reach for this when an agent needs to compare a list of YC companies across syncs without re-uploading the slug list each time._

  ```bash
  yc-companies-pp-cli watch add stripe airbnb doordash
  ```
- **`watch diff`** — Show team_size, status, and hiring changes on your watched companies since a prior snapshot.

  _Reach for this when an agent is asked 'what changed on these YC companies recently' and needs structured delta rows, not a fresh full list._

  ```bash
  yc-companies-pp-cli watch diff --since 2026-04-01 --agent
  ```
- **`companies new`** — Companies that appeared in the directory after a date or since the last sync.

  _Reach for this for 'what's new in YC' questions — the only correct answer requires snapshot history._

  ```bash
  yc-companies-pp-cli companies new --since 2026-04-01 --json --select slug,name,batch,one_liner
  ```
- **`companies changes`** — Diff status, team_size, or isHiring across the whole index between two snapshots, optionally scoped to specific slugs or target values.

  _Reach for this for any 'who flipped X' question — newly hiring, newly acquired, jumped team size._

  ```bash
  yc-companies-pp-cli companies changes --field isHiring --to true --since 2026-04-01 --json
  ```

### Cross-row local computation
- **`companies similar`** — Given a YC slug, rank peers by tag overlap, industry match, and batch proximity.

  _Reach for this when an agent needs competitors or peers for a specific YC company without an LLM-judged similarity call._

  ```bash
  yc-companies-pp-cli companies similar stripe --limit 10 --json --select slug,name,score,shared_tags
  ```
- **`stats by-batch`** — GROUP BY over the local companies table — count, average team size, % hiring, % top, % acquired per cell.

  _Reach for this for any 'how has X changed across batches' question — counts, growth, hiring share, team-size trends._

  ```bash
  yc-companies-pp-cli stats by-batch --industry fintech --json
  ```
- **`batches show`** — One-shot batch view: company count, top industries, top tags, % hiring, % top company, % acquired, median team size.

  _Reach for this when an agent or analyst needs a quick read on the shape of a single batch._

  ```bash
  yc-companies-pp-cli batches show w25 --json
  ```

## Usage

Run `yc-companies-pp-cli --help` for the full command reference and flag list.

## Commands

### batches

Browse YC batches.

- **`yc-companies-pp-cli batches get`** - Fetch every company in a YC batch.

### companies

List, fetch, filter, and search Y Combinator companies.

- **`yc-companies-pp-cli companies black_founded`** - Fetch Black-founded YC companies.
- **`yc-companies-pp-cli companies get_in_batch`** - Fetch a single company by batch slug and company slug.
- **`yc-companies-pp-cli companies hiring`** - Fetch companies currently hiring.
- **`yc-companies-pp-cli companies hispanic_latino_founded`** - Fetch Hispanic/Latino-founded YC companies.
- **`yc-companies-pp-cli companies list`** - Fetch the full directory (5,889+ companies).
- **`yc-companies-pp-cli companies nonprofit`** - Fetch nonprofit YC companies.
- **`yc-companies-pp-cli companies top`** - Fetch top-ranked YC companies.
- **`yc-companies-pp-cli companies women_founded`** - Fetch women-founded YC companies.

### industries

Browse YC industry verticals.

- **`yc-companies-pp-cli industries get`** - Fetch every company in an industry.

### meta

Directory metadata.

- **`yc-companies-pp-cli meta get`** - Counts and last-updated timestamp for the directory.

### tags

Browse YC tags.

- **`yc-companies-pp-cli tags get`** - Fetch every company with a given tag.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
yc-companies-pp-cli batches mock-value

# JSON for scripting and agents
yc-companies-pp-cli batches mock-value --json

# Filter to specific fields
yc-companies-pp-cli batches mock-value --json --select id,name,status

# Dry run — show the request without sending
yc-companies-pp-cli batches mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
yc-companies-pp-cli batches mock-value --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-yc-companies -g
```

Then invoke `/pp-yc-companies <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add yc-companies yc-companies-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/yc-companies-current).
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
    "yc-companies": {
      "command": "yc-companies-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
yc-companies-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/yc-companies-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **`No companies found` after fresh install** — Run `yc-companies-pp-cli sync` first — every read command reads from the local SQLite store.
- **`watch diff` returns empty** — Watch diff needs at least two snapshots. Run `yc-companies-pp-cli sync` a second time after some upstream data changes; or use `--since <older-date>`.
- **Slug not found in `companies get <slug>`** — Use `yc-companies-pp-cli companies search '<name>'` first — slugs are kebab-case derivatives, not human names.
- **Old data** — Run `yc-companies-pp-cli doctor` to compare local meta.last_updated against upstream; `yc-companies-pp-cli sync` to refresh.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**yc-oss/api**](https://github.com/yc-oss/api) — JavaScript
- [**Nneji123/ycombinator-scraper**](https://github.com/Nneji123/ycombinator-scraper) — Python
- [**corralm/yc-scraper**](https://github.com/corralm/yc-scraper) — Python
- [**akshaybhalotia/yc_company_scraper**](https://github.com/akshaybhalotia/yc_company_scraper) — Python
- [**dirkjbreeuwer/yc-scraper**](https://github.com/dirkjbreeuwer/yc-scraper) — Python
- [**goofygary/yc-startup-scraper**](https://github.com/goofygary/yc-startup-scraper) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
