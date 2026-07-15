# Copper CLI

**The Copper CRM command line no one else built: full CRUD plus a local database, weighted pipeline forecasting, stale-deal detection, and the bulk operations Copper's own API refuses to provide.**

Copper has no CLI, no Go client, and no agent-native tool. This turns a click-heavy web CRM into a scriptable, offline-queryable surface. It mirrors people, companies, leads, opportunities, projects, tasks, and activities into local SQLite, then adds the weighted forecast (forecast), cold-deal sweep (stale), and rate-limit-aware bulk editor (bulk) that the API and web UI leave out.

## Install

The recommended path installs both the `copper-pp-cli` binary and the `pp-copper` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install copper
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install copper --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install copper --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install copper --agent claude-code
npx -y @mvanhorn/printing-press-library install copper --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/copper/cmd/copper-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/copper-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install copper --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-copper --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-copper --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install copper --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/copper-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `COPPER_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/copper/cmd/copper-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "copper": {
      "command": "copper-pp-mcp",
      "env": {
        "COPPER_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Copper uses a multi-header API key. Set COPPER_API_KEY (System Settings -> API Keys -> Create a Key) and COPPER_USER_EMAIL (the email of the key owner). Every request sends X-PW-AccessToken, X-PW-UserEmail, and X-PW-Application: developer_api.

## Quick Start

```bash
# Verify both credentials are wired before any live call
copper-pp-cli doctor --dry-run

# Mirror the pipeline graph into local SQLite
copper-pp-cli sync --resources opportunities,pipelines,pipeline_stages,users

# Weighted expected revenue, grouped by stage
copper-pp-cli forecast --by stage --agent

# Find open deals gone cold for 3+ weeks
copper-pp-cli stale --days 21 --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Pipeline intelligence
- **`forecast`** — Weighted expected-revenue roll-up: sums monetary_value x win_probability over open opportunities, grouped by stage, assignee, or close-month.

  _Reach for this instead of exporting CSV and pivoting by hand for any expected-revenue or commit/quota question._

  ```bash
  copper-pp-cli forecast --pipeline 12345 --by stage --agent
  ```
- **`stale`** — Surfaces open opportunities with no interaction in N days, sorted by staleness x value, across all reps.

  _Use to find deals going cold before they die; pipe the output into bulk reassign._

  ```bash
  copper-pp-cli stale --days 21 --by assignee --agent
  ```
- **`who`** — Joins an opportunity to its company, people, and recent activities into one related-records view.

  _Use for one-call deal prep before nudging or logging a touch._

  ```bash
  copper-pp-cli who opportunity:88 --agent
  ```

### Operations Copper's API refuses to provide
- **`bulk`** — Applies field updates (stage, owner, custom fields) across many opportunities with bounded concurrency and heuristic 429 backoff.

  _Use for mass updates to existing records; the safe way to avoid the rate-limit wall a naive loop hits._

  ```bash
  copper-pp-cli bulk move --query stale.json --set pipeline_stage_id=9 --concurrency 4 --dry-run
  ```
- **`upsert`** — Match-then-create-or-update for people and leads; normalizes the people.emails[] vs leads.email shape difference.

  _Use to sync external rows without creating duplicates; not a blind create._

  ```bash
  copper-pp-cli upsert person --match email --file contacts.json --dry-run
  ```
- **`dedupe`** — Local SQLite self-join surfacing people or leads that share an email, name, or company.

  _Run before or after a sync to catch duplicate contacts the API will not flag._

  ```bash
  copper-pp-cli dedupe people --on email --agent
  ```

### CRM hygiene
- **`log`** — Creates an activity with the type resolved by name (bumps interaction_count); log fix deletes and recreates to edit an immutable activity.

  _Use to log or correct a single touch; for the same touch across many records use bulk._

  ```bash
  copper-pp-cli log call --on opportunity:88 --note "Left voicemail"
  ```

## Recipes


### Monday weighted forecast by rep

```bash
copper-pp-cli forecast --by assignee --agent --select assignee_name,weighted_value,open_value
```

Commit view per rep without a spreadsheet pivot.

### Sweep then bulk-reassign cold deals

```bash
copper-pp-cli stale --days 30 --by assignee --agent
```

List cold deals grouped by owner. Save the JSON, then feed the ids into the bulk reassign command (with --set assignee_id=<id>) to mass-reassign — preview with --dry-run before applying.

### Idempotent contact import

```bash
copper-pp-cli upsert person --match email --file leads.json --dry-run
```

Create-or-update external rows without duplicating contacts.

### One-call deal prep

```bash
copper-pp-cli who opportunity:88 --agent --select company.name,people.name,activities.details
```

Pull the deal's company, contacts, and recent touches in one narrowed view.

## Usage

Run `copper-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `COPPER_CONFIG_DIR`, `COPPER_DATA_DIR`, `COPPER_STATE_DIR`, or `COPPER_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `COPPER_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export COPPER_HOME=/srv/copper
copper-pp-cli doctor
```

Under `COPPER_HOME=/srv/copper`, the four dirs resolve to `/srv/copper/config`, `/srv/copper/data`, `/srv/copper/state`, and `/srv/copper/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "copper": {
      "command": "copper-pp-mcp",
      "env": {
        "COPPER_HOME": "/srv/copper"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `COPPER_DATA_DIR` overrides an explicit `--home` for that kind. Use `COPPER_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `COPPER_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `copper-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### account

Account details

- **`copper-pp-cli account`** - Fetch account details

### activities

Manage activities (notes and logged interactions)

- **`copper-pp-cli activities create`** - Create a new activity
- **`copper-pp-cli activities delete`** - Delete an activity
- **`copper-pp-cli activities get`** - Fetch an activity by id
- **`copper-pp-cli activities search`** - List/search activities

### activity-types

Activity types

- **`copper-pp-cli activity-types`** - List all activity types

### companies

Manage companies

- **`copper-pp-cli companies activities`** - List a company's activities
- **`copper-pp-cli companies create`** - Create a new company
- **`copper-pp-cli companies delete`** - Delete a company
- **`copper-pp-cli companies get`** - Fetch a company by id
- **`copper-pp-cli companies search`** - List/search companies
- **`copper-pp-cli companies update`** - Update a company

### contact-types

Contact types

- **`copper-pp-cli contact-types`** - List all contact types

### custom-activity-types

Custom activity types

- **`copper-pp-cli custom-activity-types create`** - Create a new custom activity type
- **`copper-pp-cli custom-activity-types get`** - Fetch a custom activity type by id
- **`copper-pp-cli custom-activity-types list`** - List all custom activity types
- **`copper-pp-cli custom-activity-types update`** - Update a custom activity type

### custom-field-definitions

Custom field definitions

- **`copper-pp-cli custom-field-definitions create`** - Create a new custom field definition
- **`copper-pp-cli custom-field-definitions delete`** - Delete a custom field definition
- **`copper-pp-cli custom-field-definitions get`** - Fetch a custom field definition by id
- **`copper-pp-cli custom-field-definitions list`** - List all custom field definitions
- **`copper-pp-cli custom-field-definitions update`** - Update a custom field definition

### customer-sources

Customer (lead) sources

- **`copper-pp-cli customer-sources`** - List all customer sources

### lead-statuses

Lead statuses

- **`copper-pp-cli lead-statuses`** - List all lead statuses

### leads

Manage leads

- **`copper-pp-cli leads activities`** - List a lead's activities
- **`copper-pp-cli leads convert`** - Convert a lead into a person/company/opportunity
- **`copper-pp-cli leads create`** - Create a new lead
- **`copper-pp-cli leads delete`** - Delete a lead
- **`copper-pp-cli leads get`** - Fetch a lead by id
- **`copper-pp-cli leads search`** - List/search leads
- **`copper-pp-cli leads update`** - Update a lead
- **`copper-pp-cli leads upsert`** - Create or update a lead, matched by email or a custom field

### loss-reasons

Opportunity loss reasons

- **`copper-pp-cli loss-reasons`** - List all loss reasons

### opportunities

Manage opportunities (deals)

- **`copper-pp-cli opportunities create`** - Create a new opportunity
- **`copper-pp-cli opportunities delete`** - Delete an opportunity
- **`copper-pp-cli opportunities get`** - Fetch an opportunity by id
- **`copper-pp-cli opportunities search`** - List/search opportunities
- **`copper-pp-cli opportunities update`** - Update an opportunity

### people

Manage people (contacts)

- **`copper-pp-cli people activities`** - List a person's activities
- **`copper-pp-cli people create`** - Create a new person
- **`copper-pp-cli people delete`** - Delete a person
- **`copper-pp-cli people fetch-by-email`** - Fetch a person by email address
- **`copper-pp-cli people get`** - Fetch a person by id
- **`copper-pp-cli people search`** - List/search people
- **`copper-pp-cli people update`** - Update a person

### pipeline-stages

Pipeline stages

- **`copper-pp-cli pipeline-stages`** - List all pipeline stages

### pipelines

Sales pipelines

- **`copper-pp-cli pipelines list`** - List all pipelines
- **`copper-pp-cli pipelines stages`** - List stages within a specific pipeline

### projects

Manage projects

- **`copper-pp-cli projects create`** - Create a new project
- **`copper-pp-cli projects delete`** - Delete a project
- **`copper-pp-cli projects get`** - Fetch a project by id
- **`copper-pp-cli projects search`** - List/search projects
- **`copper-pp-cli projects update`** - Update a project

### tags

Tags used across records

- **`copper-pp-cli tags`** - List all tags

### tasks

Manage tasks

- **`copper-pp-cli tasks create`** - Create a new task
- **`copper-pp-cli tasks delete`** - Delete a task
- **`copper-pp-cli tasks get`** - Fetch a task by id
- **`copper-pp-cli tasks search`** - List/search tasks
- **`copper-pp-cli tasks update`** - Update a task

### users

Users in the account

- **`copper-pp-cli users get`** - Fetch a user by id
- **`copper-pp-cli users search`** - List/search users

### webhooks

Webhook subscriptions

- **`copper-pp-cli webhooks create`** - Create a new webhook subscription
- **`copper-pp-cli webhooks delete`** - Delete (unsubscribe) a webhook subscription
- **`copper-pp-cli webhooks get`** - Fetch a webhook subscription by id
- **`copper-pp-cli webhooks list`** - List all webhook subscriptions


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
copper-pp-cli account

# JSON for scripting and agents
copper-pp-cli account --json

# Filter to specific fields
copper-pp-cli account --json --select id,name,status

# Dry run — show the request without sending
copper-pp-cli account --dry-run

# Agent mode — JSON + compact + no prompts in one flag
copper-pp-cli account --agent
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
copper-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `copper-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is ``; `--home`, `COPPER_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `COPPER_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `copper-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `copper-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $COPPER_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 authentication error** — Set both COPPER_API_KEY and COPPER_USER_EMAIL; the email must be the key owner's account email
- **429 Too Many Requests on a bulk run** — Lower --concurrency (Copper caps near 180 req/min and sends no rate-limit headers)
- **forecast or stale returns nothing** — Run sync --resources opportunities first; these commands read the local mirror

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**salespreso/prospyr**](https://github.com/salespreso/prospyr) — Python (7 stars)
- [**Gamesight/copper-typescript**](https://github.com/Gamesight/copper-typescript) — TypeScript (6 stars)
- [**ClaimerApp/copper-sdk**](https://github.com/ClaimerApp/copper-sdk) — Python (5 stars)
- [**dazanza/copper-mcp**](https://github.com/dazanza/copper-mcp) — JavaScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
