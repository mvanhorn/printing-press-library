# Zoho Campaigns CLI

**Every Zoho Campaigns operation, plus report history, offline search, and headless self-refreshing OAuth no other tool has.**

The only open-source CLI for Zoho Campaigns. It mirrors campaigns and per-recipient engagement data into local SQLite and tracks mailing-list counts through digest-written snapshots, answering the questions Zoho's current-state UI can't: 'digest' rolls up a whole window for dashboards, 'delta' shows a campaign's trajectory, and 'growth' tracks list health. Self-client OAuth refreshes automatically — built for scheduled, headless, agent-driven runs. (Mailing lists themselves aren't cached offline — a known Zoho listkey quirk; list data lives in the count snapshots.)

## Install

The recommended path installs both the `zoho-campaigns-pp-cli` binary and the `pp-zoho-campaigns` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install zoho-campaigns
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install zoho-campaigns --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install zoho-campaigns --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install zoho-campaigns --agent claude-code
npx -y @mvanhorn/printing-press-library install zoho-campaigns --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/zoho-campaigns/cmd/zoho-campaigns-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/zoho-campaigns-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install zoho-campaigns --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-zoho-campaigns --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-zoho-campaigns --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install zoho-campaigns --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local OAuth2 refresh-token credentials — configure them first if you haven't:

```bash
export ZOHO_CAMPAIGNS_CLIENT_ID="your-token-here"
export ZOHO_CAMPAIGNS_CLIENT_SECRET="your-token-here"
export ZOHO_CAMPAIGNS_REFRESH_TOKEN="your-token-here"
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/zoho-campaigns-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `ZOHO_CAMPAIGNS_CLIENT_ID` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/zoho-campaigns/cmd/zoho-campaigns-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "zoho-campaigns": {
      "command": "zoho-campaigns-pp-mcp",
      "env": {
        "ZOHO_CAMPAIGNS_CLIENT_ID": "<your-key>"
      }
    }
  }
}
```

</details>


## Authentication

Auth is a Zoho self-client (api-console.zoho.com): set ZOHO_CAMPAIGNS_CLIENT_ID, ZOHO_CAMPAIGNS_CLIENT_SECRET, and ZOHO_CAMPAIGNS_REFRESH_TOKEN (or store them via config). Zoho refresh tokens do not expire, so access tokens mint themselves before every call — no browser, no re-consent, no recertification. Scopes needed: ZohoCampaigns.campaign.ALL and ZohoCampaigns.contact.ALL (READ variants suffice for reporting-only use).

## Quick Start

```bash
# Verify config and credential wiring; run plain 'doctor' for a live reachability check
zoho-campaigns-pp-cli doctor --dry-run

# Mirror campaigns for offline search; the report and list-count snapshots that power delta and growth are written by 'digest'
zoho-campaigns-pp-cli sync --resources campaigns

# The org-wide rollup: sends, rates, list totals — dashboard-ready JSON
zoho-campaigns-pp-cli digest --since 30d --agent

# Recent sent campaigns with their campaignkey values for drill-down
zoho-campaigns-pp-cli campaigns list --status sent --range 10 --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Report history that compounds
- **`delta`** — See how one campaign's opens, clicks, bounces, and unsubscribes changed between snapshots — the trajectory Zoho never shows.

  _Reach for this when asked whether a campaign's performance is still moving or has plateaued; needs at least two digest-written snapshots in the window — no instant trajectory on a fresh install._

  ```bash
  zoho-campaigns-pp-cli delta 3z44ba67f3e0a1bfdac6 --since 7d --agent
  ```
- **`digest`** — One-shot rollup of everything sent in a window: aggregate open/click/bounce rates, list totals, and biggest movers.

  _The single command to call when refreshing a marketing dashboard or writing a daily brief; --since 24h gives the overnight change feed._

  ```bash
  zoho-campaigns-pp-cli digest --since 30d --agent
  ```
- **`growth`** — Per-list subscriber, unsubscribe, and bounce trend lines over time — is the list growing or bleeding.

  _Use for questions about list health over time rather than a point-in-time count; trends appear only after 'digest' has run at least twice over time._

  ```bash
  zoho-campaigns-pp-cli growth --since 90d --agent
  ```

### Cross-campaign contact intelligence
- **`engagement`** — Rank contacts by engagement across ALL campaigns — most engaged, or dead weight that never opens.

  _Answers who the hottest contacts are before outreach, or who to prune, without opening 44 campaign reports._

  ```bash
  zoho-campaigns-pp-cli engagement --top 20 --agent
  ```
- **`bounce-audit`** — Bounced contacts joined to current list membership — deliverability cleanup candidates, pipeable into do-not-mail.

  _Run after event imports or before big sends to protect sender reputation._

  ```bash
  zoho-campaigns-pp-cli bounce-audit --since 90d --csv
  ```
- **`journey`** — One contact's chronological history across every campaign — what they received, opened, and clicked.

  _Sales-prep lookup before a call: what marketing a contact has actually seen — reads actions cached by 'engagement' or 'bounce-audit', so run one of those first._

  ```bash
  zoho-campaigns-pp-cli journey ola.nordmann@example.com --agent
  ```

## Recipes

### Dashboard rollup

```bash
zoho-campaigns-pp-cli digest --since 30d --agent
```

Everything sent in 30 days with aggregate rates and list totals, as compact JSON for a dashboard refresh.

### Campaign trajectory

```bash
zoho-campaigns-pp-cli delta 3z44ba67f3e0a1bfdac6 --since 7d
```

How the campaign's opens, clicks, and unsubs moved across snapshots — works once 'digest' has run at least twice in the window.

### Compact campaign list

```bash
zoho-campaigns-pp-cli campaigns list --status sent --agent --select campaign_name,campaign_status,sent_date_string
```

Narrow the verbose campaign payload to just the fields a report needs.

### Find the dead weight

```bash
zoho-campaigns-pp-cli engagement --never-opened --agent
```

Contacts who never opened anything across all synced campaigns — prune candidates.

### Post-event hygiene

```bash
zoho-campaigns-pp-cli bounce-audit --since 90d --csv
```

Bounced contacts still on lists after an event import, ready to pipe into do-not-mail.

## Usage

Run `zoho-campaigns-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `ZOHO_CAMPAIGNS_CONFIG_DIR`, `ZOHO_CAMPAIGNS_DATA_DIR`, `ZOHO_CAMPAIGNS_STATE_DIR`, or `ZOHO_CAMPAIGNS_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `ZOHO_CAMPAIGNS_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export ZOHO_CAMPAIGNS_HOME=/srv/zoho-campaigns
zoho-campaigns-pp-cli doctor
```

Under `ZOHO_CAMPAIGNS_HOME=/srv/zoho-campaigns`, the four dirs resolve to `/srv/zoho-campaigns/config`, `/srv/zoho-campaigns/data`, `/srv/zoho-campaigns/state`, and `/srv/zoho-campaigns/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "zoho-campaigns": {
      "command": "zoho-campaigns-pp-mcp",
      "env": {
        "ZOHO_CAMPAIGNS_HOME": "/srv/zoho-campaigns"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `ZOHO_CAMPAIGNS_DATA_DIR` overrides an explicit `--home` for that kind. Use `ZOHO_CAMPAIGNS_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `ZOHO_CAMPAIGNS_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `zoho-campaigns-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### campaigns

Email campaign operations — list, reports, recipients, and lifecycle

- **`zoho-campaigns-pp-cli campaigns clone`** - Clone an existing campaign
- **`zoho-campaigns-pp-cli campaigns create`** - Create a campaign (draft) — name, sender, subject, content URL, target lists
- **`zoho-campaigns-pp-cli campaigns delete`** - Delete a campaign
- **`zoho-campaigns-pp-cli campaigns details`** - Full campaign configuration — subject, sender, status, lists
- **`zoho-campaigns-pp-cli campaigns last-report`** - Report for the most recently sent campaign — quick health pulse
- **`zoho-campaigns-pp-cli campaigns list`** - List recent campaigns with status filter — the source of campaignkey values
- **`zoho-campaigns-pp-cli campaigns recipients`** - Per-recipient engagement data for a campaign, by action type
- **`zoho-campaigns-pp-cli campaigns report`** - Campaign performance report — opens, clicks, bounces, unsubscribes, geo, reach
- **`zoho-campaigns-pp-cli campaigns schedule`** - Schedule a campaign for a future send time
- **`zoho-campaigns-pp-cli campaigns send`** - Send a draft campaign now
- **`zoho-campaigns-pp-cli campaigns sent`** - Recently sent campaigns with delivery stats

### contacts

Mailing lists, subscribers, segments, and contact fields

- **`zoho-campaigns-pp-cli contacts bulk-add`** - Add up to 10 contacts to an existing list by email
- **`zoho-campaigns-pp-cli contacts create-list`** - Create a new mailing list and add initial contacts
- **`zoho-campaigns-pp-cli contacts delete-list`** - Delete a mailing list
- **`zoho-campaigns-pp-cli contacts do-not-mail`** - Move a contact to the org-wide Do-Not-Mail registry
- **`zoho-campaigns-pp-cli contacts fields`** - Contact field schema — display names and API field names for contactinfo payloads
- **`zoho-campaigns-pp-cli contacts list-details`** - Advanced details for one list, including related campaigns
- **`zoho-campaigns-pp-cli contacts lists`** - All mailing lists with listkey, contact/unsub/bounce counts, and owner
- **`zoho-campaigns-pp-cli contacts segment-contacts`** - Contacts in a list segment
- **`zoho-campaigns-pp-cli contacts segment-details`** - Details of a list segment
- **`zoho-campaigns-pp-cli contacts subscribe`** - Subscribe one contact (with custom fields) to a list. Note: Zoho always sends a confirmation email
- **`zoho-campaigns-pp-cli contacts subscriber-count`** - Subscriber count for a list by status
- **`zoho-campaigns-pp-cli contacts subscribers`** - Subscribers of a list, filtered by status, paginated
- **`zoho-campaigns-pp-cli contacts unsubscribe`** - Unsubscribe a contact from a list
- **`zoho-campaigns-pp-cli contacts update-list`** - Rename a list or change its signup form visibility

### tags

Contact tag management

- **`zoho-campaigns-pp-cli tags add`** - Create a tag
- **`zoho-campaigns-pp-cli tags associate`** - Attach a tag to a contact
- **`zoho-campaigns-pp-cli tags deassociate`** - Remove a tag from a contact
- **`zoho-campaigns-pp-cli tags delete`** - Delete a tag
- **`zoho-campaigns-pp-cli tags list`** - All tags in the org


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`zoho-campaigns-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`zoho-campaigns-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`zoho-campaigns-pp-cli learnings list`** - Inspect taught rows
- **`zoho-campaigns-pp-cli learnings forget <query>`** - Undo a teach
- **`zoho-campaigns-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`zoho-campaigns-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`zoho-campaigns-pp-cli teach-pattern`** - Install a query/resource template up front
- **`zoho-campaigns-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `ZOHO_CAMPAIGNS_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `zoho-campaigns-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
zoho-campaigns-pp-cli campaigns list

# JSON for scripting and agents
zoho-campaigns-pp-cli campaigns list --json

# Filter to specific fields
zoho-campaigns-pp-cli campaigns list --json --select id,name,status

# Dry run — show the request without sending
zoho-campaigns-pp-cli campaigns list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
zoho-campaigns-pp-cli campaigns list --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `6` partial failure in response body (see `--allow-partial-failure`), `7` rate limited, `10` config error.

## Health Check

```bash
zoho-campaigns-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `zoho-campaigns-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/zoho-campaigns-pp-cli/config.toml`; `--home`, `ZOHO_CAMPAIGNS_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `ZOHO_CAMPAIGNS_CLIENT_ID` | auth_flow_input | Yes | OAuth client ID. |
| `ZOHO_CAMPAIGNS_CLIENT_SECRET` | auth_flow_input | No | Set during initial auth setup. |
| `ZOHO_CAMPAIGNS_REFRESH_TOKEN` | auth_flow_input | Yes | Set during initial auth setup. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `zoho-campaigns-pp-cli doctor` reports `agentcookie: detected` and `auth status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `zoho-campaigns-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $ZOHO_CAMPAIGNS_CLIENT_ID`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items


### API-specific
- **Response says status error with code 1007 or 'Unauthorized request'** — Refresh token or client secret is wrong, or the account lives on another data center — verify ZOHO_CAMPAIGNS_CLIENT_ID/SECRET/REFRESH_TOKEN and that the org authenticates at accounts.zoho.com
- **Error code 6101 'No campaign available in this view'** — Not an auth failure — the status filter matched nothing; widen --status to all or raise --range
- **Calls suddenly fail for ~30 minutes** — Zoho locks the API for 30 minutes after 500 calls in 5 minutes — rely on sync plus local commands (digest, engagement) instead of live loops
- **subscribe returns 'Please give the correct data'** — contactinfo field names must exactly match the schema from 'contacts fields' (e.g. "Contact Email", "First Name")

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**keepsuit/laravel-zoho-campaigns**](https://github.com/keepsuit/laravel-zoho-campaigns) — PHP (2 stars)
- [**TheWorkflowAcademy/Zoho-Campaigns-API-Subscribe-Contact**](https://github.com/TheWorkflowAcademy/Zoho-Campaigns-API-Subscribe-Contact) — Deluge

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
