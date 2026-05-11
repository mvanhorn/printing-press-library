# Fathom CLI

**Sync your Fathom meetings once, then search, analyze, and act on them forever — offline, at scale, without burning API quota.**

fathom-pp-cli pulls every meeting, transcript, summary, and action item into a local SQLite store, then unlocks cross-meeting intelligence no MCP server or web UI can provide: commitment tracking across all your calls, topic trend analysis over weeks, pre-call account briefs, pipeline velocity detection, and team meeting-load audits.

## Install

The recommended path installs both the `fathom-pp-cli` binary and the `pp-fathom` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install fathom
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install fathom --cli-only
```


### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/fathom/cmd/fathom-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/fathom-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-fathom --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-fathom --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-fathom skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-fathom. The skill defines how its required CLI can be installed.
```

## Authentication

Fathom uses an API key passed via the X-Api-Key header. Generate one at fathom.video → User Settings → API Access. Set FATHOM_PP_CLI_API_KEY in your environment, then run doctor to verify.

## Quick Start

```bash
# Verify API key and connectivity
fathom-pp-cli doctor


# Pull all meetings, transcripts, summaries, and action items into local SQLite
fathom-pp-cli sync --full


# See everything you've promised in the last 30 days
fathom-pp-cli commitments --assignee me --since 30d


# Track topic frequency trends across recent customer calls
fathom-pp-cli topics --terms pricing,onboarding --weeks 8


# Get a pre-call brief for your next Acme meeting
fathom-pp-cli brief --domain acme.com

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`commitments`** — See every open action item you promised across all calls — grouped by meeting, assignee, and date — without opening a single recording.

  _Use when you need a weekly accountability audit of all meeting commitments before a 1:1 or team sync._

  ```bash
  fathom-pp-cli commitments --assignee me --since 30d --agent
  ```
- **`topics`** — Find out how often 'pricing,' 'onboarding,' or any keyword has surfaced in your meetings over the past N weeks — with week-over-week trend.

  _Use before a board meeting or quarterly review to synthesize what themes have been dominating customer calls._

  ```bash
  fathom-pp-cli topics --terms pricing,onboarding --weeks 12 --agent
  ```
- **`velocity`** — Track whether your meeting cadence with a key account is accelerating, stable, or stalling — month by month.

  _Use for pipeline health reviews: a stalling cadence with a key account is an early warning signal before the deal goes cold._

  ```bash
  fathom-pp-cli velocity --domain stripe.com --months 6 --agent
  ```
- **`workload`** — See which team members are spending the most hours in meetings per week and whether the load is worsening.

  _Use in 1:1 prep or team planning to identify who is in 'meeting hell' before assigning more collaborative work._

  ```bash
  fathom-pp-cli workload --team Engineering --weeks 4 --threshold 15 --agent
  ```

### Agent-native plumbing
- **`brief`** — Get a chronological history of every meeting with a specific person or company — past topics, open action items, last contact date — before you join a call.

  _Use immediately before a customer call to surface prior commitments and context without opening multiple browser tabs._

  ```bash
  fathom-pp-cli brief --domain acme.com --agent
  ```
- **`account`** — View a complete, domain-keyed history with any company: every meeting, topics discussed, action items, and cadence — in one structured output.

  _Use during account reviews, renewal prep, or CRM updates to get a full picture of all interactions with a company._

  ```bash
  fathom-pp-cli account --domain notion.so --agent
  ```

### Operational tooling
- **`stale`** — Find recordings that were captured but have no transcript, summary, or action items synced — useful for operators debugging pipeline gaps.

  _Use on Monday morning to audit which recordings from last week are missing data before your team needs them._

  ```bash
  fathom-pp-cli stale --since 7d --agent
  ```
- **`crm-gaps`** — Surface CRM-matched meetings where no action items were logged — calls that touched active deals but left no paper trail.

  _Use in RevOps audits to find sales calls where reps talked to prospects but forgot to log next steps in the CRM._

  ```bash
  fathom-pp-cli crm-gaps --since 30d --agent
  ```
- **`coverage`** — Track how reliably a recurring meeting (weekly planning, standup, 1:1) is being recorded over time.

  _Use to verify that mandatory-record meetings are actually being captured before auditing team performance._

  ```bash
  fathom-pp-cli coverage --pattern 'Weekly Planning' --weeks 10 --agent
  ```

## Usage

Run `fathom-pp-cli --help` for the full command reference and flag list.

## Commands

### meetings

Meeting recordings with transcripts, summaries, and action items

- **`fathom-pp-cli meetings list`** - List meetings with optional filters and included data

### recordings

Individual recording data: transcripts and summaries

- **`fathom-pp-cli recordings get-summary`** - Get AI-generated meeting summary in markdown format
- **`fathom-pp-cli recordings get-transcript`** - Get full transcript for a recording with speaker attribution and timestamps

### team-members

Members of your teams

- **`fathom-pp-cli team-members list`** - List team members, optionally filtered by team name

### teams

Teams your account has access to

- **`fathom-pp-cli teams list`** - List all teams accessible to your account

### webhooks

Webhooks for async meeting completion notifications

- **`fathom-pp-cli webhooks create`** - Create a webhook to receive meeting data on completion
- **`fathom-pp-cli webhooks delete`** - Delete a webhook by ID


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
fathom-pp-cli meetings

# JSON for scripting and agents
fathom-pp-cli meetings --json

# Filter to specific fields
fathom-pp-cli meetings --json --select id,name,status

# Dry run — show the request without sending
fathom-pp-cli meetings --dry-run

# Agent mode — JSON + compact + no prompts in one flag
fathom-pp-cli meetings --agent
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
npx skills add mvanhorn/printing-press-library/cli-skills/pp-fathom -g
```

Then invoke `/pp-fathom <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/fathom/cmd/fathom-pp-mcp@latest
```

Then register it:

```bash
claude mcp add fathom fathom-pp-mcp -e FATHOM_PP_CLI_API_KEY=<your-key>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/fathom-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `FATHOM_PP_CLI_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/fathom/cmd/fathom-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "fathom": {
      "command": "fathom-pp-mcp",
      "env": {
        "FATHOM_PP_CLI_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
fathom-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: ``

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `FATHOM_PP_CLI_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `fathom-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $FATHOM_PP_CLI_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **doctor returns 401 Unauthorized** — Check FATHOM_PP_CLI_API_KEY is set; generate a new key at fathom.video → User Settings → API Access
- **sync returns 429 Too Many Requests** — Fathom rate limit is 60 req/min; run fathom-pp-cli sync with --throttle to slow down bulk fetching
- **transcript or summary is empty in local store** — Run fathom-pp-cli stale --since 7d to find affected recordings, then fathom-pp-cli sync --recording-id <id> to re-sync individually
- **brief or account shows no results for a domain** — Ensure sync ran with --include-transcript --include-summary --include-action-items; re-run fathom-pp-cli sync --full if needed

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**Dot-Fun/fathom-mcp**](https://github.com/Dot-Fun/fathom-mcp) — Python
- [**lukas-bekr/fathom-mcp**](https://github.com/lukas-bekr/fathom-mcp) — TypeScript
- [**druellan/Fathom-Simple-MCP**](https://github.com/druellan/Fathom-Simple-MCP) — Python
- [**mackenly/fathom-api**](https://github.com/mackenly/fathom-api) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
