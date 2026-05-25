# Memberstack CLI

**The first CLI for Memberstack — every Admin API endpoint plus stale-member detection, plan coverage, JWT decode, bulk operations, and a local SQLite mirror no SDK or MCP has.**

Memberstack ships an SDK and an MCP server, but no command-line tool. memberstack-pp-cli covers every Admin REST endpoint with agent-native --json output, then adds a local SQLite mirror that powers stale-member detection, plan-coverage pivots, custom-fields flatten, drift audit, and one-shot snapshot export — things no dashboard, SDK, or MCP can do. Writable via the CLI: member emails, custom-field values, metaData, plan-connections, and custom data records. Dashboard-only: defining new custom field names, creating plans, and creating new data tables (the REST API does not expose these).

Learn more at [Memberstack](https://www.memberstack.com).

Printed by [@Quarterpak](https://github.com/Quarterpak) (Anton Sahibzada).

## Install

The recommended path installs both the `memberstack-pp-cli` binary and the `pp-memberstack` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install memberstack
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install memberstack --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install memberstack --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install memberstack --agent claude-code
npx -y @mvanhorn/printing-press-library install memberstack --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/auth/memberstack/cmd/memberstack-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/memberstack-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-memberstack --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-memberstack --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-memberstack skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-memberstack. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/memberstack-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `MEMBERSTACK_SECRET_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/auth/memberstack/cmd/memberstack-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "memberstack": {
      "command": "memberstack-pp-mcp",
      "env": {
        "MEMBERSTACK_SECRET_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Authenticate by exporting MEMBERSTACK_SECRET_KEY (sk_sb_* for sandbox / test mode, sk_* or sk_live_* for live mode). Keys come from Memberstack > Dev Tools. The CLI never reads the browser-side public key (pk_*) — that is for the DOM SDK only. Rate limit is 25 requests/sec; the sync command paces itself automatically.

## Quick Start

```bash
# Confirm auth, network, and store are wired up before doing anything else.
memberstack-pp-cli doctor --dry-run

# Mirror every member into the local SQLite store.
memberstack-pp-cli sync --full

# Set custom-field values on a member — preview with --dry-run, remove the flag to commit.
memberstack-pp-cli members update mem_example --custom-fields '{"plan_tier":"gold","city":"NYC"}' --dry-run

# Find dormant members in one command — no equivalent in dashboard or MCP.
memberstack-pp-cli stale --days 30 --json --select id,auth.email,lastLogin

# Pivot active plan-connections across every member; zero-plan members are flagged.
memberstack-pp-cli plan-coverage

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`stale`** — Find members whose lastLogin is older than N days.

  _When an agent needs to find dormant accounts for cleanup or re-engagement campaigns, this is the single command to reach for. The dashboard and MCP cannot filter by lastLogin server-side._

  ```bash
  memberstack-pp-cli stale --days 30 --json --select id,auth.email,lastLogin
  ```
- **`plan-coverage`** — Pivot active plan-connections across all members; flag members with zero active plans.

  _Agents auditing 'which members have access to which plans' should call this once instead of N member-get calls._

  ```bash
  memberstack-pp-cli plan-coverage --agent
  ```
- **`fields flatten`** — Pivot every member's customFields map into a flat table (CSV or JSON).

  _Agents producing BI or marketing exports should not have to hand-write the pivot; one command produces the spreadsheet shape._

  ```bash
  memberstack-pp-cli fields flatten --csv > members.csv
  ```
- **`audit`** — Diff the local snapshot against live to surface what changed since the last sync.

  _Agents validating a deploy or investigating support tickets should call this to see what mutated._

  ```bash
  memberstack-pp-cli audit --sample 100 --json
  ```

### Agent-native plumbing
- **`token decode`** — Decode a Memberstack JWT locally without calling the API.

  _When an agent needs to inspect a JWT during frontend debugging, this is faster than verify and works without the secret key._

  ```bash
  memberstack-pp-cli token decode eyJhbGciOi...
  ```
- **`bulk delete`** — Wipe members matching a SQL WHERE clause, with --dry-run preview.

  _Agents asked to clean up sandbox test accounts should call this once. Always preview first; explicit --apply required to commit._

  ```bash
  memberstack-pp-cli bulk delete --where "auth_email LIKE '%@test.local'" --dry-run
  ```
- **`watch`** — Poll the cursor and print new members as they arrive (live tail).

  _When an agent needs to react to new signups (alerting, enrichment, Slack notify), pipe this through jq instead of building a webhook receiver._

  ```bash
  memberstack-pp-cli watch --since 5m --json
  ```
- **`records find`** — Build the Prisma findMany payload from --where, --order-by, --limit flags.

  _Agents querying custom data tables get the same expressiveness without constructing JSON envelopes._

  ```bash
  memberstack-pp-cli records find products --where 'inStock=true' --order-by price:asc --limit 10
  ```

### Reachability mitigation
- **`plans list`** — Build a plans index from planConnections observed across sync, with (planId, member-count, lastSeen).

  _When an agent needs to know which plan IDs are in use without opening the dashboard, this is the shortcut._

  ```bash
  memberstack-pp-cli plans list --json
  ```

## Recipes


### Find and clean up dormant test accounts

```bash
memberstack-pp-cli stale --days 60 --json --select id,auth.email | jq -r '.[].id' | xargs -I{} memberstack-pp-cli members delete {} --dry-run
```

Dry-run preview; remove --dry-run when you are ready to commit.

### Export every member with custom fields flattened to CSV

```bash
memberstack-pp-cli fields flatten --csv > members-$(date +%F).csv
```

One row per member; one column per custom field plus auth.email, planConnections, lastLogin.

### Watch new signups

```bash
memberstack-pp-cli watch --since 1h --agent --select id,auth.email,createdAt
```

Cursor-polls the API; one JSON object per new member, pipeable to jq or any other notifier.

### Find members with no active plan

```bash
memberstack-pp-cli plan-coverage --agent --select id,auth.email,activePlanCount | jq 'map(select(.activePlanCount == 0))'
```

Identifies free-tier members who should be re-engaged or pruned.

### Query a custom data table without building a Prisma envelope

```bash
memberstack-pp-cli records find products --where 'inStock=true,price>=20' --order-by price:asc --limit 10 --agent --select records.id,records.data.name,records.data.price
```

Shorthand flags compile to the Prisma findMany payload the API expects.

## Usage

Run `memberstack-pp-cli --help` for the full command reference and flag list.

## Commands

### data-tables

List and inspect custom data table schemas

- **`memberstack-pp-cli data-tables get`** - Get a single data table schema
- **`memberstack-pp-cli data-tables list`** - List custom data tables

### members

Member CRUD, JWT verification, and plan-connection helpers

- **`memberstack-pp-cli members create`** - Create a member
- **`memberstack-pp-cli members delete`** - Delete a member
- **`memberstack-pp-cli members get`** - Look up by Memberstack ID (mem_*) or URL-encoded email.
- **`memberstack-pp-cli members list`** - Paginated list of members. Cursor is the numeric `endCursor` from the previous response.
- **`memberstack-pp-cli members update`** - Update a member
- **`memberstack-pp-cli members verify-token`** - Validates a Memberstack-issued JWT server-side and returns the decoded claims.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
memberstack-pp-cli data-tables list

# JSON for scripting and agents
memberstack-pp-cli data-tables list --json

# Filter to specific fields
memberstack-pp-cli data-tables list --json --select id,name,status

# Dry run — show the request without sending
memberstack-pp-cli data-tables list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
memberstack-pp-cli data-tables list --agent
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
memberstack-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/memberstack-admin-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `MEMBERSTACK_SECRET_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `memberstack-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $MEMBERSTACK_SECRET_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Unauthorized on every command** — Check the key prefix — sk_sb_* for sandbox, sk_* or sk_live_* for live. Public keys (pk_*) are browser-only and will not work.
- **429 Too Many Requests during sync** — Memberstack caps at 25 req/sec. Pass --rate-limit 20 to back off, or wait ≈60 seconds and retry.
- **Plan ID not found** — Plans live in the dashboard, not the REST API. Run plans list to see the plan IDs the CLI has observed in planConnections.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**@memberstack/admin**](https://www.npmjs.com/package/@memberstack/admin) — JavaScript
- [**memberstack-skills**](https://github.com/Memberstack/memberstack-skills) — TypeScript
- [**Memberstack MCP Server (Beta)**](https://docs.memberstack.com/hc/en-us/articles/41661556042267-Memberstack-MCP-Server-Beta) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
