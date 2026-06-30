# Zoho Desk CLI

**The only scriptable, agent-native CLI for Zoho Desk: every ticket operation plus a local SQLite store, offline search, and SLA/triage analytics no help-desk tool ships.**

Manage tickets, contacts, accounts, and agents from the command line with automatic OAuth token refresh, multi-data-center handling, and auto-pagination with 429 backoff. Then go beyond the API: sla-radar forecasts breaches before they happen, agent-load finds the real bottleneck, triage builds one ranked queue, and morning composes your whole shift brief.

## Install

The recommended path installs both the `zoho-desk-pp-cli` binary and the `pp-zoho-desk` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install zoho-desk
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install zoho-desk --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install zoho-desk --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install zoho-desk --agent claude-code
npx -y @mvanhorn/printing-press-library install zoho-desk --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/zoho-desk/cmd/zoho-desk-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/zoho-desk-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install zoho-desk --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-zoho-desk --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-zoho-desk --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install zoho-desk --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local OAuth2 refresh-token credentials — configure them first if you haven't:

```bash
export ZOHO_DESK_CLIENT_ID="your-token-here"
export ZOHO_DESK_CLIENT_SECRET="your-token-here"
export ZOHO_DESK_REFRESH_TOKEN="your-token-here"
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/zoho-desk-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `ZOHO_DESK_CLIENT_ID` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/zoho-desk/cmd/zoho-desk-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "zoho-desk": {
      "command": "zoho-desk-pp-mcp",
      "env": {
        "ZOHO_DESK_CLIENT_ID": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Zoho Desk uses OAuth2 with a permanent refresh token and 1-hour access tokens. Register a self-client at api-console.zoho.com, grant Desk scopes, and exchange the code for a refresh token. Set ZOHO_DESK_CLIENT_ID, ZOHO_DESK_CLIENT_SECRET, and ZOHO_DESK_REFRESH_TOKEN; the CLI refreshes access tokens automatically. Every call also needs your orgId (run 'organizations list' to find it) and the right data center.

## Quick Start

```bash
# Check config and reachability before anything else
zoho-desk-pp-cli doctor --dry-run

# Find your orgId (required on every other call)
zoho-desk-pp-cli organizations list --json

# Pull tickets/contacts/agents into the local store
zoho-desk-pp-cli sync --json

# See the ranked queue of what needs attention first
zoho-desk-pp-cli triage --limit 20 --agent

# Catch tickets about to breach SLA
zoho-desk-pp-cli sla-radar --within 2h --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local analytics no UI exposes
- **`sla-radar`** — See which open tickets will breach SLA in the next N hours, ranked by time-to-due, before they breach.

  _Reach for this to act before a breach instead of reporting one after._

  ```bash
  zoho-desk-pp-cli sla-radar --within 2h --agent
  ```
- **`agent-load`** — See which agents are actually overloaded right now, weighting open tickets by priority and age versus the team median.

  _Use before reassigning work to find the real bottleneck, not just raw counts._

  ```bash
  zoho-desk-pp-cli agent-load --weighted --agent
  ```
- **`triage`** — One ranked queue merging unassigned, overdue, and high-priority tickets with a combined score.

  _Start the day here: the single list of what needs attention first._

  ```bash
  zoho-desk-pp-cli triage --limit 20 --agent
  ```
- **`morning`** — A single brief composing breach forecast, agent load, and overnight changes.

  _The one command to run at the start of a shift._

  ```bash
  zoho-desk-pp-cli morning --agent
  ```
- **`rebalance`** — Proposes ticket moves from overloaded agents to idle ones, then optionally applies them in bulk.

  _Use to even out queues without hand-picking each reassignment._

  ```bash
  zoho-desk-pp-cli rebalance --plan --agent
  ```
- **`breach-history`** — Who breached SLA and the trend, grouped by agent or department.

  _Use for retrospective SLA accountability._

  ```bash
  zoho-desk-pp-cli breach-history --by agent --agent
  ```

### Local state that compounds
- **`since`** — Every ticket modified within a time window, with its current status, priority, and assignee, sorted most-recent first.

  _Use to catch up on what moved while you were away._

  ```bash
  zoho-desk-pp-cli since 12h --agent
  ```
- **`contact-360`** — Everything about one customer in one view: their contact record, account, and every ticket with status and priority.

  _Use before a call to load full customer context in one command._

  ```bash
  zoho-desk-pp-cli contact-360 PII_EMAIL_EXAMPLE --agent
  ```

## Recipes

### Morning shift brief

```bash
zoho-desk-pp-cli morning --agent
```

One command: breach forecast, agent load, and overnight changes.

### Rebalance an overloaded queue

```bash
zoho-desk-pp-cli rebalance --plan --agent
```

See proposed moves from overloaded agents to idle ones before applying.

### Customer context before a call

```bash
zoho-desk-pp-cli contact-360 PII_EMAIL_EXAMPLE --agent
```

All tickets and account for one customer.

### Narrow a large ticket list

```bash
zoho-desk-pp-cli tickets list --status Open --limit 50 --agent --select data.id,data.subject,data.status,data.assigneeId
```

Ticket payloads are large; --select with dotted paths returns only the fields you need.

### Find forgotten tickets

```bash
zoho-desk-pp-cli stale --days 5 --agent
```

Tickets with no update in 5 days.

## Usage

Run `zoho-desk-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `ZOHO_DESK_CONFIG_DIR`, `ZOHO_DESK_DATA_DIR`, `ZOHO_DESK_STATE_DIR`, or `ZOHO_DESK_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `ZOHO_DESK_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export ZOHO_DESK_HOME=/srv/zoho-desk
zoho-desk-pp-cli doctor
```

Under `ZOHO_DESK_HOME=/srv/zoho-desk`, the four dirs resolve to `/srv/zoho-desk/config`, `/srv/zoho-desk/data`, `/srv/zoho-desk/state`, and `/srv/zoho-desk/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "zoho-desk": {
      "command": "zoho-desk-pp-mcp",
      "env": {
        "ZOHO_DESK_HOME": "/srv/zoho-desk"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `ZOHO_DESK_DATA_DIR` overrides an explicit `--home` for that kind. Use `ZOHO_DESK_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `ZOHO_DESK_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `zoho-desk-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### accounts

Manage accounts (companies)

- **`zoho-desk-pp-cli accounts contacts`** - List contacts for an account
- **`zoho-desk-pp-cli accounts create`** - Create an account
- **`zoho-desk-pp-cli accounts get`** - Get an account by ID
- **`zoho-desk-pp-cli accounts list`** - List accounts
- **`zoho-desk-pp-cli accounts tickets`** - List tickets for an account
- **`zoho-desk-pp-cli accounts update`** - Update an account

### agents

Help-desk agents

- **`zoho-desk-pp-cli agents count`** - Get total agent count
- **`zoho-desk-pp-cli agents get`** - Get an agent by ID
- **`zoho-desk-pp-cli agents list`** - List agents

### articles

Knowledge base articles

- **`zoho-desk-pp-cli articles get`** - Get a KB article by ID
- **`zoho-desk-pp-cli articles list`** - List KB articles
- **`zoho-desk-pp-cli articles search`** - Search KB articles

### comments

Internal/public ticket comments

- **`zoho-desk-pp-cli comments create`** - Add a comment to a ticket
- **`zoho-desk-pp-cli comments delete`** - Delete a comment
- **`zoho-desk-pp-cli comments list`** - List comments on a ticket

### contacts

Manage contacts (customers)

- **`zoho-desk-pp-cli contacts create`** - Create a contact
- **`zoho-desk-pp-cli contacts get`** - Get a contact by ID
- **`zoho-desk-pp-cli contacts list`** - List contacts
- **`zoho-desk-pp-cli contacts tickets`** - List tickets for a contact
- **`zoho-desk-pp-cli contacts update`** - Update a contact

### departments

Departments

- **`zoho-desk-pp-cli departments get`** - Get a department by ID
- **`zoho-desk-pp-cli departments list`** - List departments

### organizations

Portals/organizations you can access (orgId source)

- **`zoho-desk-pp-cli organizations accessible`** - List all organizations accessible to the user
- **`zoho-desk-pp-cli organizations get`** - Get one organization
- **`zoho-desk-pp-cli organizations list`** - List organizations the token can access

### products

Products

- **`zoho-desk-pp-cli products get`** - Get a product by ID
- **`zoho-desk-pp-cli products list`** - List products

### slas

Service-level agreements

- **`zoho-desk-pp-cli slas`** - List SLAs

### tags

Tags

- **`zoho-desk-pp-cli tags`** - List tags

### tasks

Tasks

- **`zoho-desk-pp-cli tasks create`** - Create a task
- **`zoho-desk-pp-cli tasks get`** - Get a task by ID
- **`zoho-desk-pp-cli tasks list`** - List tasks

### teams

Agent teams

- **`zoho-desk-pp-cli teams agents`** - List agents in a team
- **`zoho-desk-pp-cli teams get`** - Get a team by ID
- **`zoho-desk-pp-cli teams list`** - List teams

### threads

Ticket conversation threads

- **`zoho-desk-pp-cli threads get`** - Get a thread's full content
- **`zoho-desk-pp-cli threads latest`** - Get the latest thread on a ticket
- **`zoho-desk-pp-cli threads list`** - List threads on a ticket

### ticketfields

Ticket field/layout metadata

- **`zoho-desk-pp-cli ticketfields`** - List ticket fields

### tickets

Manage support tickets

- **`zoho-desk-pp-cli tickets close`** - Close one or more tickets
- **`zoho-desk-pp-cli tickets create`** - Create a ticket
- **`zoho-desk-pp-cli tickets draft-reply`** - Create a draft reply on a ticket
- **`zoho-desk-pp-cli tickets get`** - Get a ticket by ID
- **`zoho-desk-pp-cli tickets history`** - Get a ticket's history/audit trail
- **`zoho-desk-pp-cli tickets list`** - List tickets
- **`zoho-desk-pp-cli tickets merge`** - Merge other tickets into this one
- **`zoho-desk-pp-cli tickets metrics`** - Get response/resolution time metrics for a ticket
- **`zoho-desk-pp-cli tickets send-reply`** - Send a reply on a ticket
- **`zoho-desk-pp-cli tickets update`** - Update a ticket

### tickets_search

Cross-module search

- **`zoho-desk-pp-cli tickets-search`** - Search tickets by criteria

### timeentries

Ticket time entries

- **`zoho-desk-pp-cli timeentries create`** - Add a time entry to a ticket
- **`zoho-desk-pp-cli timeentries list`** - List time entries on a ticket


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
zoho-desk-pp-cli accounts list

# JSON for scripting and agents
zoho-desk-pp-cli accounts list --json

# Filter to specific fields
zoho-desk-pp-cli accounts list --json --select id,name,status

# Dry run — show the request without sending
zoho-desk-pp-cli accounts list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
zoho-desk-pp-cli accounts list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
zoho-desk-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `zoho-desk-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/zoho-desk-pp-cli/config.toml`; `--home`, `ZOHO_DESK_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `ZOHO_DESK_CLIENT_ID` | auth_flow_input | Yes |  |
| `ZOHO_DESK_CLIENT_SECRET` | auth_flow_input | No | Set during initial auth setup. |
| `ZOHO_DESK_REFRESH_TOKEN` | auth_flow_input | Yes | Set during initial auth setup. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `zoho-desk-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `zoho-desk-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $ZOHO_DESK_CLIENT_ID`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **INVALID_OAUTH or 401 on every call** — orgId is missing or belongs to a different portal than your token. Run 'organizations list' and set the right orgId.
- **invalid_client when refreshing the token** — Your accounts data center does not match where the client was registered. Set the correct --dc / ZOHO_DC.
- **429 TOO_MANY_REQUESTS** — You hit the concurrency or daily credit cap. The CLI honors Retry-After and backs off; lower --concurrency or wait.
- **List returns only 100 records** — Lists cap at 100 per page. Use 'sync' (auto-paginates) or raise --from to page through.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**thomas-kl1/php-sdk-zoho-desk**](https://github.com/thomas-kl1/php-sdk-zoho-desk) — PHP (6 stars)
- [**Trifoia/zoho-desk-sdk**](https://github.com/Trifoia/zoho-desk-sdk) — JavaScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
