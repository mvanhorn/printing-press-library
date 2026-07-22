# GoHighLevel CLI

**The terminal for GoHighLevel. Bulk ops, dedup, and pipeline reports in seconds — local cache, agent-native JSON, multi-location support.**

Every GHL feature, plus a local SQLite mirror, name->ID resolution, SQL over your contacts and opportunities, and proper handling of the lowercase-pit footgun. The first GoHighLevel CLI — built for operators who run real-estate brokerages and digital agencies and want to stop hand-rolling retry logic and stage-history scripts.

Created by [@jenwilliams-eng](https://github.com/jenwilliams-eng) (Jen Williams).

## Install

The recommended path installs both the `gohighlevel-pp-cli` binary and the `pp-gohighlevel` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install gohighlevel
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install gohighlevel --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install gohighlevel --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install gohighlevel --agent claude-code
npx -y @mvanhorn/printing-press-library install gohighlevel --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/gohighlevel/cmd/gohighlevel-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/gohighlevel-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install gohighlevel --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-gohighlevel --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-gohighlevel --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install gohighlevel --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/gohighlevel-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `GHL_PIT_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "gohighlevel": {
      "command": "gohighlevel-pp-mcp",
      "env": {
        "GHL_PIT_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

GoHighLevel uses Private Integration Tokens (PIT). Set GHL_PIT_TOKEN in your environment (lowercase pit- prefix — capital Pit- returns 401 Invalid JWT). Optionally set GHL_LOCATION_ID for the active sub-account, or use named profiles via `gohighlevel-pp-cli config use <name>` to switch between locations like KWCP and THINK.

## Quick Start

```bash
# verify your PIT token works and the API is reachable
gohighlevel-pp-cli doctor

# pull contacts, opportunities, pipelines, custom fields, and tags into the local SQLite cache
gohighlevel-pp-cli sync

# stage-by-stage opportunity counts, ready to paste into a Looker sheet
gohighlevel-pp-cli opp funnel --pipeline "Non-KW" --tsv

# opportunities that have not moved in 30 days, derived from local stage-transition history
gohighlevel-pp-cli opp stale --pipeline "Non-KW" --days 30 --json

# any cross-entity query becomes one SQL statement
gohighlevel-pp-cli sql "SELECT firstName, lastName, email FROM contacts WHERE tags LIKE '%recruit%' LIMIT 20" --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`opp stale`** — Find opportunities sitting in a stage longer than N days, using synthesized stage-entry timestamps from sync history.

  _When the user asks 'which recruits have been sitting in this stage too long', this is the only path that resolves it without re-running their custom dashboard script._

  ```bash
  gohighlevel-pp-cli opp stale --pipeline "Non-KW" --stage "Recruit Lead" --days 30 --json
  ```
- **`opp funnel`** — Stage-by-stage count and total monetary value for a pipeline, output as TSV ready to paste into a dashboard sheet.

  _Replaces the user's Monday L10 prep script with a one-liner; the agent can re-run it on demand with different pipeline filters._

  ```bash
  gohighlevel-pp-cli opp funnel --pipeline "Non-KW" --tsv
  ```
- **`sql`** — Run read-only SQL against the local SQLite mirror of contacts, opportunities, pipelines, stages, tags, custom_fields, conversations, messages, and appointments.

  _The cross-entity differentiator. Anything an agent wants to answer about the GHL data becomes one SQL statement instead of three paginated API calls assembled in Python._

  ```bash
  gohighlevel-pp-cli sql "SELECT name, COUNT(*) FROM opportunities GROUP BY pipelineStageId ORDER BY 2 DESC" --json
  ```
- **`contact dedup`** — Group contacts by lowercased email and E.164 phone, score by filled-field count and recency, emit a merge plan.

  _Replaces a 200-line Python script the operator runs weekly. Agents can preview the merge before any data changes._

  ```bash
  gohighlevel-pp-cli contact dedup --by email,phone --dry-run --json
  ```
- **`contact decay`** — Find opportunities in a stage whose contacts have had no inbound or outbound messages in N days.

  _Catches recruits going cold before they're lost. The operator's existing alert script becomes a one-liner the agent can re-run with different thresholds._

  ```bash
  gohighlevel-pp-cli contact decay --stage "Engaged" --idle-days 30 --json
  ```
- **`recruit hot`** — Rank recruits by a composite of production signals, engagement, and recruit-tag count.

  _Monday L10 prep, before Kymber asks. Agents can re-tune the threshold without re-running the entire Python pipeline._

  ```bash
  gohighlevel-pp-cli recruit hot --threshold 25 --tsv
  ```
- **`convo thread`** — Reconstruct a chronological SMS+email+call thread for a single contact from the local messages table.

  _Before any recruit outreach, the operator wants the whole conversation in one place. Agents can read the full history without four MCP calls._

  ```bash
  gohighlevel-pp-cli convo thread --contact jane@example.com --json
  ```

### Service-specific guardrails
- **`field id`** — Translate human-readable custom field names into opaque GHL IDs, with did-you-mean suggestions on typos.

  _Pair `field id` with `sql` to query contacts by custom-field value without hardcoding 20-character GHL IDs; eliminates an entire class of broken-after-rename bugs._

  ```bash
  gohighlevel-pp-cli field id "Agent Affiliation"
  ```
- **`config use`** — Named profiles for each GHL sub-account (KWCP, THINK), with --location flag honored across every command.

  _Prevents the wrong-tenant footgun. Agents can be told 'use the kwcp profile' once at session start and never think about it again._

  ```bash
  gohighlevel-pp-cli config use kwcp && gohighlevel-pp-cli --location think opp funnel --pipeline "Non-KW"
  ```
- **`doctor`** — Validate the PIT token (auto-lowercases the prefix), ping the active location, and report local cache state.

  _First command to run when anything is acting funny. Agents can call it before complex flows to short-circuit a token or cache problem._

  ```bash
  gohighlevel-pp-cli doctor --json
  ```

### Agent-native plumbing
- **`contact bulk-tag`** — Apply or remove a tag across many contacts read from stdin, with chunked 100-at-a-time delivery and connection-drop retry.

  _The highest-leverage daily op for any agency — after every training, after every campaign, after every import. Without this the operator hand-rolls retry logic every time._

  ```bash
  cat emails.csv | gohighlevel-pp-cli contact bulk-tag --tag "Attended_2026-05-19" --dry-run
  ```

## Usage

Run `gohighlevel-pp-cli --help` for the full command reference and flag list.

## Commands

### calendars

Manage calendars

- **`gohighlevel-pp-cli calendars create`** - Create
- **`gohighlevel-pp-cli calendars create-appointment`** - Create appointment
- **`gohighlevel-pp-cli calendars delete-appointment`** - Delete appointment
- **`gohighlevel-pp-cli calendars get`** - Get
- **`gohighlevel-pp-cli calendars get-appointment`** - Get appointment
- **`gohighlevel-pp-cli calendars list`** - List
- **`gohighlevel-pp-cli calendars list-events`** - List calendar events for a location
- **`gohighlevel-pp-cli calendars update-appointment`** - Update appointment

### contacts

Manage contacts

- **`gohighlevel-pp-cli contacts bulk-update-tags`** - Bulk add/remove tags across many contacts
- **`gohighlevel-pp-cli contacts create`** - Create a contact
- **`gohighlevel-pp-cli contacts delete`** - Delete a contact
- **`gohighlevel-pp-cli contacts find-duplicate`** - Find duplicate contact by email or name
- **`gohighlevel-pp-cli contacts get`** - Get a contact by id
- **`gohighlevel-pp-cli contacts search`** - GHL's /contacts/search has a hard 100-page cap. Use the `startAfter`
cursor returned in the response for pagination past 10k results.
Phone search returns 500 — use email or name instead.
- **`gohighlevel-pp-cli contacts update`** - Update a contact
- **`gohighlevel-pp-cli contacts upsert`** - Upsert contact by email or phone

### conversations

Manage conversations

- **`gohighlevel-pp-cli conversations get`** - Get
- **`gohighlevel-pp-cli conversations search`** - Search conversations (Version 2021-04-15)
- **`gohighlevel-pp-cli conversations send-message`** - Send an SMS, email, or other message

### locations

Manage locations

- **`gohighlevel-pp-cli locations get`** - Get
- **`gohighlevel-pp-cli locations search`** - Search locations (agency-level)

### opportunities

Manage opportunities

- **`gohighlevel-pp-cli opportunities create-opportunity`** - Create opportunity
- **`gohighlevel-pp-cli opportunities delete-opportunity`** - Delete opportunity
- **`gohighlevel-pp-cli opportunities get-opportunity`** - Get opportunity
- **`gohighlevel-pp-cli opportunities list-pipelines`** - List pipelines for a location
- **`gohighlevel-pp-cli opportunities search`** - Search opportunities across a pipeline
- **`gohighlevel-pp-cli opportunities update-opportunity`** - Update opportunity
- **`gohighlevel-pp-cli opportunities upsert-opportunity`** - Upsert opportunity

### surveys

Manage surveys

- **`gohighlevel-pp-cli surveys`** - List

### users

Manage users

- **`gohighlevel-pp-cli users get`** - Get
- **`gohighlevel-pp-cli users search`** - Search users in a location

### workflows

Manage workflows

- **`gohighlevel-pp-cli workflows`** - List workflows for a location

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
gohighlevel-pp-cli calendars list --location-id 550e8400-e29b-41d4-a716-446655440000

# JSON for scripting and agents
gohighlevel-pp-cli calendars list --location-id 550e8400-e29b-41d4-a716-446655440000 --json

# Filter to specific fields
gohighlevel-pp-cli calendars list --location-id 550e8400-e29b-41d4-a716-446655440000 --json --select id,name,status

# Dry run — show the request without sending
gohighlevel-pp-cli calendars list --location-id 550e8400-e29b-41d4-a716-446655440000 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
gohighlevel-pp-cli calendars list --location-id 550e8400-e29b-41d4-a716-446655440000 --agent
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

## Health Check

```bash
gohighlevel-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/gohighlevel-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `GHL_PIT_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `gohighlevel-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $GHL_PIT_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Invalid JWT** — Your PIT prefix is uppercase. Re-export the token with the lowercase prefix: `export GHL_PIT_TOKEN=pit-<uuid>` (or run `gohighlevel-pp-cli doctor` — it auto-lowercases and warns).
- **500 error on contact search by phone** — GHL's phone search returns 500. Search by email or name instead: `gohighlevel-pp-cli contact search --email jane@example.com`.
- **Cannot create dropdown / multi-select custom field via API** — GHL does not expose this through the API. Create the field in the UI, then sync: `gohighlevel-pp-cli sync` (the new field will be in the local custom_fields table).
- **`workflow enroll` silently does not fire the workflow** — The contact is already enrolled in that workflow; the 'Form Submitted' trigger does not re-fire for re-enrollment. Remove first with `gohighlevel-pp-cli contact unenroll <contactId> <workflowId>`, then re-enroll.
- **Connection drops mid-sync** — Re-run `gohighlevel-pp-cli sync` — incremental sync resumes from the last cursor. The client uses exponential backoff with 5 retries on connection drops.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**mastanley13/GoHighLevel-MCP**](https://github.com/mastanley13/GoHighLevel-MCP) — TypeScript
- [**BusyBee3333/Go-High-Level-MCP-2026-Complete**](https://github.com/BusyBee3333/Go-High-Level-MCP-2026-Complete) — TypeScript
- [**robbyDAninja/7fa-ghl-mcp**](https://github.com/robbyDAninja/7fa-ghl-mcp) — TypeScript
- [**tenfoldmarc/ghl-mcp**](https://github.com/tenfoldmarc/ghl-mcp) — TypeScript
- [**@gohighlevel/api-client**](https://www.npmjs.com/package/@gohighlevel/api-client) — TypeScript
- [**highlevel-python**](https://pypi.org/project/highlevel-python/) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
