# Priority Software CLI

**The entire Priority REST API — every form, subform, batch, and attachment — plus offline sync, schema intelligence, and throttle-safe bulk operations no other Priority tool has.**

Priority exposes every ERP screen as an OData entity, but ships no CLI and no official SDK. This CLI covers the full surface — generic CRUD on any form, OData query options, $batch, attachments, text subforms, metadata — and then goes beyond it: a local SQLite mirror with full-text search, per-tenant schema intelligence ('forms search', 'forms licensed'), and batch journaling with resume ('batch resume'). A built-in adaptive limiter keeps every command under Priority's 100-requests-per-minute fair-use cap.

## Install

The recommended path installs both the `priority-pp-cli` binary and the `pp-priority` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install priority
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install priority --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install priority --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install priority --agent claude-code
npx -y @mvanhorn/printing-press-library install priority --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/priority/cmd/priority-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/priority-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install priority --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-priority --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-priority --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install priority --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/priority-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `PRIORITY_API_USERNAME` and `PRIORITY_API_PASSWORD` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/priority/cmd/priority-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "priority": {
      "command": "priority-pp-mcp",
      "env": {
        "PRIORITY_API_USERNAME": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Priority instances are per-tenant: set PRIORITY_BASE_URL to your service root (https://<server>/odata/Priority/<tabula.ini>/<company>). Authenticate with Basic credentials via PRIORITY_API_USERNAME and PRIORITY_API_PASSWORD — the username is the 'API User Name' from the Personnel File form, not your login name. For server-to-server automation, use a Personal Access Token (REST Interface Access Tokens form): set PRIORITY_API_USERNAME to the token and PRIORITY_API_PASSWORD to the literal string PAT. The default base URL points at Priority's official public sandbox (user apidemo, password 123, read-only).

## Quick Start

```bash
# Verify configuration safely; drop --dry-run to test live connectivity
priority-pp-cli doctor --dry-run

# Query any form with OData options — this is the generic surface that covers the whole API
priority-pp-cli entity list ORDERS --filter "ORDSTATUSDES eq 'Confirmed'" --top 5

# Build the local mirror that powers offline search and analytics
priority-pp-cli sync --resources orders,customers,invoices

# Full-text search the mirror without burning the 100/min rate cap
priority-pp-cli search "chocolate" --type customers

# One-command customer 360: invoiced totals, age buckets, open orders, recent invoices
priority-pp-cli customer summary 1011 --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Tenant schema intelligence
- **`forms search`** — Find any form, field, or subform on your tenant by name fragment, instantly and offline.

  _Reach for this before any entity call on an unfamiliar tenant — field names differ per install and guessing burns the 100/min rate cap._

  ```bash
  priority-pp-cli forms search WARHS --json
  ```
- **`forms diff`** — Snapshot your tenant's schema and see exactly which forms and fields changed after an upgrade or customization.

  _Run after tenant upgrades to find silent field changes before integrations start failing with 400s._

  ```bash
  priority-pp-cli forms diff --baseline pre-upgrade
  ```
- **`forms licensed`** — Discover which forms are actually API-enabled on this tenant instead of finding out via 400 errors.

  _"API Cannot Be Run for This Form" is the most common real-world Priority API failure; probe once, know forever._

  ```bash
  priority-pp-cli forms licensed --forms ORDERS,CUSTOMERS,AINVOICES
  ```

### Throttle-safe bulk operations
- **`batch resume`** — Re-run only the failed operations from a partially-failed $batch — Priority batches have no rollback.

  _After a half-failed bulk load, this is the difference between surgical recovery and manually diffing response JSON._

  ```bash
  priority-pp-cli batch resume 42
  ```
- **`reconcile`** — Verify your local mirror matches the live tenant using windowed probes — the API has no $count.

  _Answers "did last night's sync actually get everything" without a full re-pull._

  ```bash
  priority-pp-cli reconcile --resource orders
  ```

### Offline ERP analytics
- **`shortage`** — See which parts will run out given the open order book — open demand vs on-hand stock, optionally netting inbound POs.

  _Replaces the weekly Excel VLOOKUP between order lines and stock; sync orders with --resource-param "orders:$expand=ORDERITEMS_SUBFORM" first, and expect on_hand null when the tenant exposes no balance fields._

  ```bash
  priority-pp-cli shortage --include-inbound --agent
  ```
- **`customer summary`** — One command for a customer's whole picture: invoiced totals, age buckets by invoice date, open orders, recent invoices.

  _The pre-collections-call briefing, at terminal speed, without burning the rate cap._

  ```bash
  priority-pp-cli customer summary 1011 --agent
  ```

## Recipes

### Order drill-down with narrowed agent output

```bash
priority-pp-cli orders get SO17000003 --expand "ORDERITEMS_SUBFORM" --agent --select "CUSTNAME,ORDNAME,ORDERITEMS_SUBFORM.PARTNAME,ORDERITEMS_SUBFORM.TQUANT,ORDERITEMS_SUBFORM.PRICE"
```

Orders with expanded line items return tens of KB; --select with dotted paths keeps only what the agent needs.

### Find tenant-custom fields before writing

```bash
priority-pp-cli forms search WARHS --json
```

Per-install custom fields (ORGT_* prefixes) are invisible in the docs; grep the cached schema before building a payload.

### Bulk load with surgical recovery

```bash
priority-pp-cli batch load --file ops.jsonl --dry-run
```

Preview a journaled $batch load; drop --dry-run to execute, and 'batch resume <id>' re-runs only failed operations.

### Monday collections review

```bash
priority-pp-cli aging --agent
```

Book-wide AR aging buckets from the local mirror; follow with 'debtors' for the ranked call list.

### Raw OData escape hatch

```bash
priority-pp-cli query "ORDERS?$filter=CUSTNAME eq '1011'&$expand=ORDERITEMS_SUBFORM"
```

Anything the API can express that no typed command covers — with auth and throttling still handled for you.

## Usage

Run `priority-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `PRIORITY_CONFIG_DIR`, `PRIORITY_DATA_DIR`, `PRIORITY_STATE_DIR`, or `PRIORITY_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `PRIORITY_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export PRIORITY_HOME=/srv/priority
priority-pp-cli doctor
```

Under `PRIORITY_HOME=/srv/priority`, the four dirs resolve to `/srv/priority/config`, `/srv/priority/data`, `/srv/priority/state`, and `/srv/priority/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "priority": {
      "command": "priority-pp-mcp",
      "env": {
        "PRIORITY_HOME": "/srv/priority"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `PRIORITY_DATA_DIR` overrides an explicit `--home` for that kind. Use `PRIORITY_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `PRIORITY_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `priority-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### batch

OData $batch — up to 100 operations per request, no rollback

- **`priority-pp-cli batch`** - Execute a JSON $batch request (max 100 ops; supports id/dependsOn/atomicityGroup and $<id>/ references)

### customers

Customers (CUSTOMERS form)

- **`priority-pp-cli customers get`** - Get one customer by customer number
- **`priority-pp-cli customers list`** - List customers

### entity

Generic OData surface — query and modify ANY Priority form by name (case-sensitive UPPERCASE)

- **`priority-pp-cli entity delete`** - Delete a record (or subform line via FORM('key')/SUB_SUBFORM(n) keyspec on parent path)
- **`priority-pp-cli entity get`** - Get one record by key. Single key: "'SO17000003'" (quotes included). Composite: "IVNUM='T9696',IVTYPE='A',DEBIT='D'"
- **`priority-pp-cli entity list`** - List records of any form with full OData query options
- **`priority-pp-cli entity subform`** - Read a subform (related lines) of a record

### invoices

AR invoices (AINVOICES form; composite key IVNUM+IVTYPE+DEBIT — use 'entity get' for single records)

- **`priority-pp-cli invoices`** - List AR invoices

### meta

Service metadata and version probes

- **`priority-pp-cli meta clear-metadata`** - Clear server-side metadata cache for one entity (empty body clears all)
- **`priority-pp-cli meta entity-metadata`** - Per-entity metadata (v25.0+; top-level entities only; prime with a $top=1 GET first)
- **`priority-pp-cli meta metadata`** - Full tenant EDMX schema (XML, can exceed 10MB)
- **`priority-pp-cli meta odata-version`** - OData gateway version
- **`priority-pp-cli meta version`** - Priority version of the tenant, e.g. 22.0-...

### orders

Sales orders (ORDERS form)

- **`priority-pp-cli orders get`** - Get one sales order by order number
- **`priority-pp-cli orders list`** - List sales orders

### parts

Part catalog (LOGPART form)

- **`priority-pp-cli parts get`** - Get one part by part number
- **`priority-pp-cli parts list`** - List parts

### porders

Purchase orders (PORDERS form)

- **`priority-pp-cli porders get`** - Get one purchase order by number
- **`priority-pp-cli porders list`** - List purchase orders

### suppliers

Suppliers/vendors (SUPPLIERS form)

- **`priority-pp-cli suppliers get`** - Get one supplier by supplier number
- **`priority-pp-cli suppliers list`** - List suppliers

### warehouses

Warehouses (WAREHOUSES form)

- **`priority-pp-cli warehouses`** - List warehouses


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`priority-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`priority-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`priority-pp-cli learnings list`** - Inspect taught rows
- **`priority-pp-cli learnings forget <query>`** - Undo a teach
- **`priority-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`priority-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`priority-pp-cli teach-pattern`** - Install a query/resource template up front
- **`priority-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `PRIORITY_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `priority-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
priority-pp-cli customers list

# JSON for scripting and agents
priority-pp-cli customers list --json

# Filter to specific fields
priority-pp-cli customers list --json --select id,name,status

# Dry run — show the request without sending
priority-pp-cli customers list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
priority-pp-cli customers list --agent
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
priority-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `priority-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/priority-pp-cli/config.toml`; `--home`, `PRIORITY_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `PRIORITY_API_USERNAME` | per_call | Yes | Set to your API credential. |
| `PRIORITY_API_PASSWORD` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `priority-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `priority-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $PRIORITY_API_USERNAME`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **HTTP 429 Too Many Requests** — You hit Priority's fair-use cap (100 calls/min/user). The built-in limiter backs off automatically; for bulk work use 'batch load' (journaled, 100 ops per request) or lower 'sync --max-pages'.
- **400 with 'API Cannot Be Run for This Form'** — The form is not API-licensed on your tenant. Run 'priority-pp-cli forms licensed --forms <FORM>' to map which forms are enabled, then ask your Priority admin to license the rest.
- **404 Not Found on an entity that exists in the UI** — Form and field names are case-sensitive UPPERCASE (ORDERS, not orders). Run 'priority-pp-cli forms search <name>' to find the exact form name on your tenant.
- **$since returns an error on some entities** — $since only works on entities with BPM (business process management) applied. Use a $filter on a date field instead for non-BPM forms.
- **Cannot address a record with a composite key** — Pass the full key spec: priority-pp-cli entity get AINVOICES "IVNUM='T9696',IVTYPE='A',DEBIT='D'".

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**Priority-Mcp**](https://github.com/assafch/Priority-Mcp) — TypeScript (2 stars)
- [**n8n-nodes-priority**](https://github.com/HirezRa/n8n-nodes-priority) — TypeScript (1 stars)
- [**priority-tsdk**](https://github.com/victor-rioba/priority-sdk) — TypeScript (1 stars)
- [**optima-pms-pp-cli**](https://github.com/th1-ai/optima-pms-pp-cli) — Go
- [**priority-api (PHP)**](https://github.com/MordiSacks/priority-api) — PHP
- [**MedatechUK.APY**](https://github.com/MedatechUK/Medatech.APY) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
