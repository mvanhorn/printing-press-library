# Freshservice CLI

**Every Freshservice operation in one Go binary — with offline search, SLA intelligence, and agent-native JSON that no other Freshservice tool has.**

freshservice-pp-cli is the first general-purpose terminal CLI for Freshservice — the jira-cli equivalent for ITSM. It covers tickets, changes, assets, users, and knowledge base with full CRUD, local SQLite sync, FTS across all entity types, and novel analytics commands that would require three dashboards and a spreadsheet to replicate manually.

Created by [@mark-van-de-ven](https://github.com/mark-van-de-ven) (Mark van de Ven).

## Install

The recommended path installs both the `freshservice-pp-cli` binary and the `pp-freshservice` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install freshservice
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install freshservice --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install freshservice --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install freshservice --agent claude-code
npx -y @mvanhorn/printing-press-library install freshservice --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/freshservice/cmd/freshservice-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/freshservice-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-freshservice --force
```

Inside a Hermes chat session:

```text
/skills install mvanhorn/printing-press-library/cli-skills/pp-freshservice --force
```

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install freshservice --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/freshservice-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `FRESHSERVICE_APIKEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "freshservice": {
      "command": "freshservice-pp-mcp",
      "env": {
        "FRESHSERVICE_APIKEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Authenticate with a Personal Access Token (PAT) from your Freshservice account:

```bash
export FRESHSERVICE_APIKEY=<your-pat>
export FRESHSERVICE_DOMAIN=<tenant>.freshservice.com
freshservice-pp-cli doctor
```

`doctor` performs an authenticated probe against `/agents/me` and reports the agent identity on success (e.g. `OK (authenticated as you_at_example.com)`), so you know the key works before running any real command.

### Picking the right domain

`FRESHSERVICE_DOMAIN` is the **Freshservice API tenant** (e.g. `acme.freshservice.com`), NOT the Freshworks unified org dashboard (e.g. `acme-org.myfreshworks.com`). Freshworks deployments often have both — the dashboard and the API live on different subdomains.

Symptoms of using the wrong one:
- `doctor` prints `Domain FAIL: ... looks like the Freshworks unified-org dashboard ...`
- `doctor` prints `Credentials FAIL: /agents/me returned HTML, not JSON.`
- All commands appear to "succeed" but `results` is a giant HTML string

Fix: drop any `-org` suffix from the subdomain and use `.freshservice.com` instead of `.myfreshworks.com`. Re-run `doctor`.

## Quick Start

```bash
# verify your PAT and domain are configured correctly
freshservice-pp-cli doctor

# pull all tickets, assets, changes, and users into the local store
freshservice-pp-cli sync --full

# see your assigned tickets and pending approvals in one view
freshservice-pp-cli my-queue user_at_example.com --agent

# which tickets will breach SLA in the next 4 hours
freshservice-pp-cli breach-risk --hours 4 --agent

# find related tickets and KB articles before creating a duplicate
freshservice-pp-cli search "VPN issue" --in tickets,kb --agent

# see which agents are overloaded before assigning a ticket
freshservice-pp-cli workload --group "IT Support" --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### SLA Management
- **`breach-risk`** — Shows every open ticket projected to breach SLA within the next N hours, sorted by minutes remaining — act before the clock runs out, not after.

  _Use this when an SRE or IT admin needs to know which tickets will breach SLA before the next check-in — prevents reactive firefighting._

  ```bash
  freshservice-pp-cli breach-risk --hours 4 --group Infrastructure --agent
  ```
- **`dept-sla`** — Aggregates SLA compliance percentage, breach count, and mean time to resolve by requester department for a rolling period — exec-ready ranking without exporting to Excel.

  _Use this when an AI agent is generating an executive SLA compliance report or identifying departments that need service level attention._

  ```bash
  freshservice-pp-cli dept-sla --period 30d --sort breach-rate --agent
  ```

### Daily Workflow
- **`my-queue`** — Combines all tickets assigned to you with SLA countdown plus any change records awaiting your approval — the first command an agent runs each morning.

  _Use this to get an AI agent's complete pending workload in one structured call before deciding which task to action next._

  ```bash
  freshservice-pp-cli my-queue user_at_example.com --agent
  ```
- **`search`** — Runs a single ranked full-text search across tickets, assets, change records, and KB articles simultaneously — find everything related to an incident keyword in one shot.

  _Use this when an AI agent needs to gather context about a symptom across all ITSM entities before proposing a resolution._

  ```bash
  freshservice-pp-cli search "database crash" --in tickets,assets,changes --agent
  ```

### Team Operations
- **`workload`** — Table of agents with open ticket count, average ticket age, P1/P2 count, and normalized load score — see who is drowning and who has capacity in five seconds.

  _Use this when an AI agent needs to decide which human agent to assign a new ticket to based on current capacity._

  ```bash
  freshservice-pp-cli workload --group "Network Support" --agent
  ```
- **`oncall-gap`** — Identifies time windows where high-severity tickets arrived but no agent in the group acknowledged within SLA — surfaces staffing gaps in on-call rotations.

  _Use this to identify on-call schedule gaps before the next incident strikes the same window._

  ```bash
  freshservice-pp-cli oncall-gap --group Infrastructure --period 4w --severity P1,P2 --agent
  ```

### Change Management
- **`change-collisions`** — Flags change records whose planned maintenance windows overlap, optionally filtered by CI — prevents two teams from scheduling conflicting downtime on the same system.

  _Use this before approving a change to verify no other group has a conflicting maintenance window on the same infrastructure._

  ```bash
  freshservice-pp-cli change-collisions --window 48h --ci prod-db-01 --agent
  ```

### Problem Management
- **`recurrence`** — Uses FTS similarity on ticket subjects and descriptions to surface repeated symptom patterns grouped by asset, requester, or keyword — shows which problems keep coming back.

  _Use this to identify root-cause candidates when an AI agent is investigating a chronic incident pattern._

  ```bash
  freshservice-pp-cli recurrence --asset FS-1042 --days 90 --agent
  ```

### Knowledge Management
- **`kb-gaps`** — Matches recent ticket subjects against the KB article corpus using FTS and ranks topic clusters with no matching article by ticket volume — tells you exactly what to document first.

  _Use this when an AI agent is drafting a knowledge base improvement plan and needs to prioritize which gaps to fill._

  ```bash
  freshservice-pp-cli kb-gaps --group "Desktop Support" --days 30 --min-tickets 3 --agent
  ```

### Asset Management
- **`orphan-assets`** — Finds assets with no associated open ticket, no active contract, and no assigned user activity in the last N days — surfaces hardware you are paying maintenance on that nobody uses.

  _Use this during IT asset audits to identify candidates for decommission or reallocation without manual cross-referencing._

  ```bash
  freshservice-pp-cli orphan-assets --type laptop --days 60 --agent
  ```

## Usage

Run `freshservice-pp-cli --help` for the full command reference and flag list.

## Commands

### agent-fields

Manage agent fields

- **`freshservice-pp-cli agent-fields list`** - List agent form fields

### agents

Manage agents

- **`freshservice-pp-cli agents create`** - Create an agent
- **`freshservice-pp-cli agents delete`** - Delete agent
- **`freshservice-pp-cli agents get`** - Get agent by ID
- **`freshservice-pp-cli agents list`** - List agents
- **`freshservice-pp-cli agents update`** - Update agent

### assets

Manage assets

- **`freshservice-pp-cli assets create`** - Create an asset
- **`freshservice-pp-cli assets delete`** - Delete an asset
- **`freshservice-pp-cli assets get`** - Get asset by display ID
- **`freshservice-pp-cli assets list`** - List or search assets
- **`freshservice-pp-cli assets update`** - Update an asset

### canned-responses

Manage canned responses

- **`freshservice-pp-cli canned-responses get`** - Get canned response
- **`freshservice-pp-cli canned-responses list`** - List canned responses

### change-form-fields

Manage change form fields

- **`freshservice-pp-cli change-form-fields list`** - List change form fields

### changes

Manage changes

- **`freshservice-pp-cli changes create`** - Create a change
- **`freshservice-pp-cli changes delete`** - Delete a change
- **`freshservice-pp-cli changes filter`** - Filter changes by query
- **`freshservice-pp-cli changes get`** - Get change by ID
- **`freshservice-pp-cli changes list`** - List changes
- **`freshservice-pp-cli changes update`** - Update a change

### contracts

Manage contracts

- **`freshservice-pp-cli contracts list`** - List contracts

### departments

Manage departments

- **`freshservice-pp-cli departments list`** - List departments

### groups

Manage groups

- **`freshservice-pp-cli groups create`** - Create agent group
- **`freshservice-pp-cli groups get`** - Get agent group
- **`freshservice-pp-cli groups list`** - List agent groups
- **`freshservice-pp-cli groups update`** - Update agent group

### locations

Manage locations

- **`freshservice-pp-cli locations list`** - List locations

### products

Manage products

- **`freshservice-pp-cli products create`** - Create product
- **`freshservice-pp-cli products get`** - Get product
- **`freshservice-pp-cli products list`** - List products
- **`freshservice-pp-cli products update`** - Update product

### requester-fields

Manage requester fields

- **`freshservice-pp-cli requester-fields list`** - List requester form fields

### requesters

Manage requesters

- **`freshservice-pp-cli requesters create`** - Create a requester
- **`freshservice-pp-cli requesters deactivate`** - Deactivate requester
- **`freshservice-pp-cli requesters get`** - Get requester by ID
- **`freshservice-pp-cli requesters list`** - List requesters
- **`freshservice-pp-cli requesters update`** - Update requester

### service-catalog

Manage service catalog

- **`freshservice-pp-cli service-catalog list-items`** - List service catalog items
- **`freshservice-pp-cli service-catalog place-service-request`** - Place a service catalog request

### solutions

Manage solutions

- **`freshservice-pp-cli solutions get-category`** - Get knowledge base category
- **`freshservice-pp-cli solutions list-categories`** - List knowledge base categories

### ticket-form-fields

Manage ticket form fields

- **`freshservice-pp-cli ticket-form-fields list`** - List ticket form fields

### tickets

Manage tickets

- **`freshservice-pp-cli tickets create`** - Create a ticket
- **`freshservice-pp-cli tickets delete`** - Delete a ticket
- **`freshservice-pp-cli tickets filter`** - Filter tickets by query
- **`freshservice-pp-cli tickets get`** - Get ticket by ID
- **`freshservice-pp-cli tickets list`** - List tickets
- **`freshservice-pp-cli tickets update`** - Update a ticket

### vendors

Manage vendors

- **`freshservice-pp-cli vendors list`** - List vendors

### workspaces

Manage workspaces

- **`freshservice-pp-cli workspaces list`** - List workspaces

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
freshservice-pp-cli agent-fields

# JSON for scripting and agents
freshservice-pp-cli agent-fields --json

# Filter to specific fields
freshservice-pp-cli agent-fields --json --select id,name,status

# Dry run — show the request without sending
freshservice-pp-cli agent-fields --dry-run

# Agent mode — JSON + compact + no prompts in one flag
freshservice-pp-cli agent-fields --agent
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
freshservice-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/freshservice-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `FRESHSERVICE_APIKEY` | per_call | Yes | Set to your API credential. |
| `FRESHSERVICE_DOMAIN` | per_call | Yes | Your Freshservice subdomain, e.g. acme.freshservice.com |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `freshservice-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $FRESHSERVICE_APIKEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized on every request** — verify FRESHSERVICE_APIKEY is your PAT (not username/password) and FRESHSERVICE_DOMAIN is yourcompany.freshservice.com without https://
- **500 error on filter queries** — wrap filter query in quotes: --query '"status:2 AND priority:3"' — Freshservice requires double-quoted filter strings
- **429 Too Many Requests** — your plan rate limit is being hit; reduce --per-page or add --delay between bulk operations
- **sync returns no results** — check FRESHSERVICE_DOMAIN matches your actual subdomain; run doctor --verbose for connection details

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**ankitpokhrel/jira-cli**](https://github.com/ankitpokhrel/jira-cli) — Go (5600 stars)
- [**flycastpartnersinc/FreshservicePS**](https://github.com/flycastpartnersinc/FreshservicePS) — PowerShell (36 stars)
- [**effytech/freshservice_mcp**](https://github.com/effytech/freshservice_mcp) — Python (31 stars)
- [**matthewoestreich/psFreshservice**](https://github.com/matthewoestreich/psFreshservice) — PowerShell (14 stars)
- [**theapsgroup/steampipe-plugin-freshservice**](https://github.com/theapsgroup/steampipe-plugin-freshservice) — Go (7 stars)
- [**theapsgroup/go-freshservice**](https://github.com/theapsgroup/go-freshservice) — Go (4 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
