# Sumble CLI

**Every Sumble v6 feature, plus the credit-awareness the API itself won't give you: cost estimates before every billed call, a running balance, a budget guard, and a local SQLite cache so you never pay twice.**

Sumble is usage-based, and the bare API gives you no way to see a balance, preview a call's cost, or avoid re-billing data you already pulled. This CLI fixes all three: cost-estimate previews spend before you pay, balance and spend track every credit from a local ledger, budget refuses calls over a ceiling, and sync caches organizations, people, postings, and technologies so stack-diff and stale run offline for zero credits.

Created by [@cpard](https://github.com/cpard) (Kostas Pardalis).

## Install

The recommended path installs both the `sumble-pp-cli` binary and the `pp-sumble` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install sumble
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install sumble --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install sumble --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install sumble --agent claude-code
npx -y @mvanhorn/printing-press-library install sumble --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/sumble/cmd/sumble-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/sumble-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install sumble --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-sumble --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-sumble --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install sumble --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/sumble-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SUMBLE_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "sumble": {
      "command": "sumble-pp-mcp",
      "env": {
        "SUMBLE_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

```bash
# Confirm SUMBLE_API_KEY is set and the API is reachable before spending anything.
sumble-pp-cli doctor

# Cheap (1 credit) lookup of the canonical technology slug to filter on.
sumble-pp-cli technologies --query kafka

# See the credit cost (~125) before you pay it; run 'budget set <n>' first to hard-cap it.
sumble-pp-cli cost-estimate organizations.find --rows 25

# Billed call (5 credits/row); results are cached locally so reads are free afterward.
sumble-pp-cli organizations find --filters-technologies '["kafka"]' --limit 25

# Check remaining credits after the call.
sumble-pp-cli balance

# List cached entities worth re-billing; runs offline for zero credits.
sumble-pp-cli stale --older-than 24h

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Credit economy
- **`cost-estimate`** — See exactly how many credits a billed Sumble call will cost before you run it.

  _Reach for this before any find/enrich/brief call so you never spend credits blind; the estimate is exact, not heuristic._

  ```bash
  sumble-pp-cli cost-estimate organizations.find --rows 25
  ```
- **`balance`** — Show remaining Sumble credits and recent burn without spending any.

  _Use this to check headroom before a batch of billed calls — it is the only programmatic way to read the balance._

  ```bash
  sumble-pp-cli balance --json
  ```
- **`budget`** — Set a credit ceiling so any call whose estimate exceeds it is refused before it dials.

  _Set this once at the start of an autonomous session; billed commands then self-abort with a nonzero exit instead of overspending._

  ```bash
  sumble-pp-cli budget set 500
  ```
- **`spend`** — Break down credits spent over time by endpoint and by day.

  _Use this to find what is eating credits (usually people enrich and intelligence briefs) and adjust the workflow._

  ```bash
  sumble-pp-cli spend --since 2026-05-01 --by endpoint
  ```

### Local-cache leverage
- **`stale`** — List cached organizations, people, and jobs older than Sumble's freshness window.

  _Run before a refresh pass to re-bill only stale rows instead of the whole cache._

  ```bash
  sumble-pp-cli stale --older-than 24h --json
  ```
- **`stack-diff`** — Compare two cached organizations' technology stacks — shared and unique technologies.

  _Use this for competitive teardown or to find a prospect's gaps versus a reference account, without spending credits._

  ```bash
  sumble-pp-cli stack-diff stripe.com adyen.com
  ```

### Cheap-path workflows
- **`reconcile`** — Resolve a CSV of company names/URLs to Sumble IDs via the cheap match endpoint, then report which still need a billed enrich.

  _Use this to attach Sumble IDs to a CRM export at minimal cost before deciding which accounts justify a full enrich._

  ```bash
  sumble-pp-cli reconcile accounts.csv --json
  ```

## Usage

Run `sumble-pp-cli --help` for the full command reference and flag list.

## Commands

### contact-lists

Manage saved contact (people) lists

- **`sumble-pp-cli contact-lists add`** - Add people to a saved contact list by id (free)
- **`sumble-pp-cli contact-lists create`** - Create a new contact list (free)
- **`sumble-pp-cli contact-lists get`** - Get the people in a saved contact list (1 credit per person returned)
- **`sumble-pp-cli contact-lists list`** - List your saved contact lists (1 credit per list returned)

### organization-lists

Manage saved organization lists

- **`sumble-pp-cli organization-lists add`** - Add organizations to a saved list by id or slug (free)
- **`sumble-pp-cli organization-lists create`** - Create a new organization list (free)
- **`sumble-pp-cli organization-lists get`** - Get the organizations in a saved list (1 credit per organization returned)
- **`sumble-pp-cli organization-lists list`** - List your saved organization lists (1 credit per list returned)

### organizations

Find, enrich, and match organizations by technographic and firmographic criteria

- **`sumble-pp-cli organizations enrich`** - Enrich one organization's technology stack with job/people/team signals (5 credits per technology found)
- **`sumble-pp-cli organizations find`** - Find organizations matching technology/category/firmographic filters (5 credits per row returned)
- **`sumble-pp-cli organizations intelligence-brief`** - AI-generated intelligence brief for an organization (50 credits when complete; 202 while pending is free)
- **`sumble-pp-cli organizations match`** - Resolve up to 1000 company names/URLs/locations to Sumble organizations (1 credit per matched org; unmatched free)

### people

Find, traverse, and enrich people at organizations

- **`sumble-pp-cli people enrich`** - Reveal a person's email (10 credits) and/or phone (80 credits); cached or unavailable reveals are free
- **`sumble-pp-cli people find`** - Find people at an organization by job function/level/country (1 credit per person)
- **`sumble-pp-cli people find-related-people`** - Find people above/below a person in the org chart (1 credit per person)

### postings

Find job postings and the people behind them — Sumble's hiring-signal layer

- **`sumble-pp-cli postings find`** - Find job postings by technology/category/country (2 credits per job, 3 with descriptions)
- **`sumble-pp-cli postings find-related-people`** - Find people associated with a job posting (1 credit per person)
- **`sumble-pp-cli postings get`** - Get a single job posting with its full description (1 credit)

### technologies

Search Sumble's technology taxonomy

- **`sumble-pp-cli technologies`** - Search technologies by name; returns canonical slugs (1 credit only if at least one match, else free)

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
sumble-pp-cli contact-lists list

# JSON for scripting and agents
sumble-pp-cli contact-lists list --json

# Filter to specific fields
sumble-pp-cli contact-lists list --json --select id,name,status

# Dry run — show the request without sending
sumble-pp-cli contact-lists list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
sumble-pp-cli contact-lists list --agent
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
sumble-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/sumble-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SUMBLE_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `sumble-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SUMBLE_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **Calls fail with HTTP 422 'Field required'** — find/enrich endpoints require a filters object; pass at least one of --filters-technologies, --filters-technology-categories, or --filters-query (array fields take a JSON array, e.g. --filters-technologies '["kafka"]').
- **HTTP 429 rate limited** — Sumble caps at ~10 requests/second aggregate; the client backs off automatically, but lower --limit or batch fewer commands if it persists.
- **balance shows a stale or zero value** — balance reads the last billed response; run any cheap billed call (e.g. technologies find) or sync to refresh the ledger.
- **A call was refused with a budget error** — raise or clear the ceiling with 'budget set <n>' / 'budget clear'.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
