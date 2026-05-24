# Indeed CLI

**Search Indeed from your terminal — with a local job history no scraper keeps, so you can ask what's new since last time and grep everything offline.**

A read-only job-search CLI for Indeed. Search with all the filters (location, radius, date posted, job type, remote, salary floor), pull full job descriptions, and research companies. Every job you see is stored locally, so `new` shows only fresh postings for a saved search and `find` does offline full-text search across your whole history.

Learn more at [Indeed](https://www.indeed.com).

Printed by [@iceman12276](https://github.com/iceman12276) (Isaac Quintero).

## Install

The recommended path installs both the `indeed-pp-cli` binary and the `pp-indeed` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install indeed
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install indeed --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install indeed --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install indeed --agent claude-code
npx -y @mvanhorn/printing-press-library install indeed --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/indeed/cmd/indeed-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/indeed-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-indeed --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-indeed --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-indeed skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-indeed. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/indeed-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/indeed/cmd/indeed-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "indeed": {
      "command": "indeed-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# the core search, with filters
indeed-pp-cli search "software engineer" --location Remote


# full description for a result's key
indeed-pp-cli job get 3700db3b90d43bc1


# name a search to poll later
indeed-pp-cli saved save daily-remote "software engineer" --location Remote --posted 1


# only what's appeared since last run
indeed-pp-cli new daily-remote

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds

- **`new`** — Re-run a saved search and show only the postings that appeared since you last looked.

  _Reach for this to poll a query on a schedule without re-reading jobs you've already triaged._

  ```bash
  indeed-pp-cli new daily-remote --json
  ```
- **`find`** — Full-text search across every job you've ever synced, with no network call.

  _Use when you want to grep your whole job history, not just one fresh query._

  ```bash
  indeed-pp-cli find "rust kubernetes" --json --select key,title,company
  ```
- **`saved save`** — Persist a named query with all its filters so you can re-run it by name.

  _Use to define the recurring searches you care about once, then poll them._

  ```bash
  indeed-pp-cli saved save daily-remote "software engineer" --location Remote --posted 1 --sort date
  ```
- **`track`** — Add a job to a local shortlist you can list and annotate later.

  _Use to keep a working set of interesting jobs across sessions without logging in._

  ```bash
  indeed-pp-cli track 3700db3b90d43bc1
  ```

### Smarter filtering

- **`search`** — Drop results whose parsed salary falls below a threshold.

  _Use to cut low-paying noise that Indeed's own filters miss because salaries are free-text._

  ```bash
  indeed-pp-cli search "data engineer" --location "Austin, TX" --min-salary 120000 --json
  ```
- **`search`** — Run one keyword across several locations in a single command and dedup by job key.

  _Use when you're open to multiple metros and want one merged, deduped list._

  ```bash
  indeed-pp-cli search "product manager" --location "Austin,Dallas,Remote" --json
  ```

## Recipes


### Fresh remote roles today

```bash
indeed-pp-cli search "backend engineer" --location Remote --posted 1 --sort date --json
```

Newest remote backend roles posted in the last day, as JSON.

### Narrow a verbose result set

```bash
indeed-pp-cli search "data scientist" --location "New York, NY" --json --select key,title,company,salary,formattedLocation
```

Use --select to keep only the fields you need from each job and avoid burning context on the full payload.

### Salary floor

```bash
indeed-pp-cli search "ml engineer" --location Remote --min-salary 150000 --csv
```

Drop anything under $150k (by parsed salary) and export to CSV.

### Poll a saved search

```bash
indeed-pp-cli new daily-remote --json
```

Only the jobs that are new since the last time you ran this saved search.

## Usage

Run `indeed-pp-cli --help` for the full command reference and flag list.

## Commands

### related

Jobs related to a given job (Indeed "competitors jobs" feed).

- **`indeed-pp-cli related`** - List jobs similar to a given job key.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
indeed-pp-cli related

# JSON for scripting and agents
indeed-pp-cli related --json

# Filter to specific fields
indeed-pp-cli related --json --select id,name,status

# Dry run — show the request without sending
indeed-pp-cli related --dry-run

# Agent mode — JSON + compact + no prompts in one flag
indeed-pp-cli related --agent
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

## Health Check

```bash
indeed-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: ``

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **search returns 0 results but the query is valid** — Indeed may have escalated bot protection on the SERP; retry, and check `indeed-pp-cli doctor` for reachability.
- **salary is empty for many jobs** — Indeed only lists salary on some postings; salary is free-text and absent on most. Use --min-salary to keep only the ones that have it.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**JobSpy**](https://github.com/speedyapply/JobSpy) — Python (3460 stars)
- [**linkedin-jobs-scraper**](https://github.com/spinlud/linkedin-jobs-scraper) — TypeScript (178 stars)
- [**indeed-scraper**](https://github.com/rynobax/indeed-scraper) — JavaScript (56 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
