# Buildalert CLI

BuildAlert UK construction lead platform — planning applications across 400+ UK councils, with applicant addresses and agent details for paid subscribers.

Learn more at [Buildalert](https://www.buildalert.uk).

Printed by [@Bazil846](https://github.com/Bazil846) (Muhammad Khan).

## Install

The recommended path installs both the `buildalert-pp-cli` binary and the `pp-buildalert` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install buildalert
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install buildalert --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install buildalert --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install buildalert --agent claude-code
npx -y @mvanhorn/printing-press-library install buildalert --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/buildalert/cmd/buildalert-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/buildalert-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-buildalert --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-buildalert --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-buildalert skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-buildalert. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
buildalert-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/buildalert-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/buildalert/cmd/buildalert-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "buildalert": {
      "command": "buildalert-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Authenticate

This CLI uses your browser session for authentication. Log in to www.buildalert.uk in Chrome, then:

```bash
buildalert-pp-cli auth login --chrome
```

Requires a cookie extraction tool. Install one:

```bash
pip install pycookiecheat          # Python (recommended)
brew install barnardb/cookies/cookies  # Homebrew
```

When your session expires, run `auth login --chrome` again.

### 3. Verify Setup

```bash
buildalert-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
buildalert-pp-cli leads
```

## Unique Features

These capabilities aren't available in any other tool for this API.

### ZAZU integration
- **`zazu-diff`** — List BuildAlert leads that aren't yet in your ZAZU bd-mirror.sqlite, so you know exactly what BuildAlert's 400+ council coverage adds to your own scrapers.

  _Pick this when you need to know which BuildAlert leads to ingest into ZAZU without duplicating effort._

  ```bash
  buildalert-pp-cli zazu-diff --zazu-db ~/Downloads/Zazu/bd-mirror.sqlite --agent
  ```
- **`pending-letters`** — Surface BuildAlert leads where canSendLetter is true, no letter has been sent yet, and ZAZU's own send log has no record either — the morning worklist for outreach.

  _Use this as the single actionable list when deciding which homeowners to contact today._

  ```bash
  buildalert-pp-cli pending-letters --zazu-db ~/Downloads/Zazu/bd-mirror.sqlite --agent --select reference,address,estimationValueBand,distanceAway
  ```
- **`letter-conflict`** — Find BuildAlert leads where letterBeenSent is true AND ZAZU's send log already records a Telegram send for the same applicant — i.e., a double-mailed homeowner.

  _Run before any BuildAlert letter campaign to catch homeowners ZAZU already mailed._

  ```bash
  buildalert-pp-cli letter-conflict --zazu-db ~/Downloads/Zazu/bd-mirror.sqlite --json
  ```
- **`coverage`** — Group leads by council across both BuildAlert and ZAZU stores; flag councils where BuildAlert has volume but ZAZU has nothing (and vice versa).

  _Use this when planning which UK councils to add to ZAZU's scraper coverage next._

  ```bash
  buildalert-pp-cli coverage --zazu-db ~/Downloads/Zazu/bd-mirror.sqlite --json
  ```

### Spend visibility
- **`analytics --type transactions`** — Aggregate the £2-per-letter charges across the local transactions mirror, grouped by council, project type, or month — the answers your accountant asks for.

  _Use this for monthly or quarterly cost reviews; pipe to CSV for spreadsheet handoff._

  ```bash
  buildalert-pp-cli analytics --type transactions --group-by council --json
  ```
- **`roi-per-lead`** — Join transactions × tracking × applications locally to produce per-lead rows: cost, reply received, work won, total return — sorted by ROI.

  _Use this to identify which project types and councils convert at the best £/work-won ratio._

  ```bash
  buildalert-pp-cli roi-per-lead --json --select reference,cost,replied,workWon
  ```

### Offline querying
- **`nearby`** — Compute haversine distance from any UK postcode against the local mirror's lat/lng coordinates; return all leads inside a radius without re-hitting the API.

  _Use this to evaluate radius expansion experiments or to score leads against a satellite-office postcode._

  ```bash
  buildalert-pp-cli nearby --postcode HA1 --radius 10 --json --select reference,distance,fullDescription
  ```

## Usage

Run `buildalert-pp-cli --help` for the full command reference and flag list.

## Commands

### health

Backend liveness check.

- **`buildalert-pp-cli health`** - Lightweight liveness probe. Returns {"success": true}. Used by the web dashboard to keep the backend warm; useful as a doctor smoke test.

### leads

Planning-application leads matched to the user's filters across UK councils.

- **`buildalert-pp-cli leads`** - List planning-application leads matched to the user's saved filters. Embeds quickFilters (per-category counts) and pagination metadata in the response.

### letter_templates

User's saved letter templates for the £2-per-letter outreach flow.

- **`buildalert-pp-cli letter_templates`** - List the user's letter templates plus the baseLogoUrl for the templated letterhead.

### tracking

ROI tracking - letters sent, replies, conversion rate, work won, total return, plus chartData.

- **`buildalert-pp-cli tracking`** - ROI summary + per-letter tracking entries in a date window. Aggregates lettersSent, letterReplies, letterRepliesPercent, workWon, workWonPercent, totalReturn.

### transactions

BuildAlert letter-send transactions (£2/letter, £2.50/postcard purchases).

- **`buildalert-pp-cli transactions`** - List letter-send transactions in a date window. Requires both dateFrom and dateTo as unix-seconds.

### user

Authenticated user profile, dashboard summary, and filter preferences.

- **`buildalert-pp-cli user dashboard`** - Dashboard overview - newLeadsCount, letterSentCount, lastLetterSentDate, credits, totalPlanningApplications, and recent userLeads array.
- **`buildalert-pp-cli user details`** - Same payload as `user profile` (alias). Returns user profile + filter preferences.
- **`buildalert-pp-cli user profile`** - Authenticated user profile - email, role, profession, company, credits, postCode, radius, longitude, latitude, subscription state, and planningStatusToFilter.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
buildalert-pp-cli leads

# JSON for scripting and agents
buildalert-pp-cli leads --json

# Filter to specific fields
buildalert-pp-cli leads --json --select id,name,status

# Dry run — show the request without sending
buildalert-pp-cli leads --dry-run

# Agent mode — JSON + compact + no prompts in one flag
buildalert-pp-cli leads --agent
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

## Health Check

```bash
buildalert-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: ``

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `buildalert-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
