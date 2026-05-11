# Unify CLI

**The read layer Unify's API doesn't ship. Local search, SQL, and coverage reports over a CRM that has no list-records endpoint.**

Unify's Data API gives you upsert, find-unique, and write — but no way to list, search, or query records. This CLI ships the missing read layer: local SQLite mirror with FTS5 search, read-only SQL across Unify and Salesforce-mirrored objects, schema snapshot/diff, coverage and score-divergence audits, batch CSV vetting, and dry-run-by-default upserts.

Learn more at [Unify](https://www.unifygtm.com/support).

## Install

The recommended path installs both the `unify-pp-cli` binary and the `pp-unify` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install unify
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install unify --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/unify-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-unify --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-unify --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-unify skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-unify. The skill defines how its required CLI can be installed.
```

## Authentication

Set UNIFY_API_KEY (generated at Settings → Developers in the Unify dashboard) to a Data API key. The CLI uses the same X-Api-Key header as the official Python and TypeScript SDKs.

## Quick Start

```bash
# Confirm auth + API reachability
unify-pp-cli doctor


# Inventory every object in your workspace (Unify standard + Salesforce-mirrored)
unify-pp-cli objects list --agent


# Tell sync which records to track — records have no list endpoint, so this is the cursor
unify-pp-cli watch add company --match domain=gladly.com


# Mirror schema + watched records into the local SQLite store
unify-pp-cli sync


# FTS5 across every synced record type — the read layer the API doesn't ship
unify-pp-cli search 'gladly' --agent


# Joinable SQL over the local mirror — answers the questions the API can't
unify-pp-cli sql "SELECT id, json_extract(attrs,'\$.name') as name FROM record_company WHERE json_extract(attrs,'\$.industry')='Retail' LIMIT 10" --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local read layer over a write-only API
- **`search`** — Full-text search across every synced object's records in one command. Returns typed hits across company, person, opportunity, and every mirrored Salesforce object.

  _When you need to answer 'is this domain in our workspace anywhere?' across Unify and Salesforce-mirrored objects, this is the one command. No N find-unique calls._

  ```bash
  unify-pp-cli search 'gladly' --agent
  ```
- **`sql`** — Run read-only SQL queries with joins across per-object record tables. Cross Unify standard objects with Salesforce-mirrored ones in a single query.

  _Answers operational questions the API can't: industry/segment slices, employee thresholds, joined opportunity/owner views. The query mode an agent reaches for instead of writing scratch Python._

  ```bash
  unify-pp-cli sql "SELECT json_extract(attrs,'$.name') as name, json_extract(attrs,'$.employee_count') as employees FROM record_company WHERE json_extract(attrs,'$.industry')='Retail' AND CAST(json_extract(attrs,'$.employee_count') AS INTEGER) >= 200" --agent
  ```
- **`trace`** — Walk reference attributes (opportunities, people, owner) from a starting record without N+1 API calls.

  _One-shot 'show me everything connected to this company' for spot-checks and pre-call prep._

  ```bash
  unify-pp-cli trace company 73fcb798-9ccd-4138-8f6a-9a801123783c --agent
  ```
- **`watch`** — Persist match-keys for records you care about; sync refreshes them via parallel find-unique on every run.

  _Without a watchlist, sync has nothing to enumerate. With it, your daily refresh hits the records you actually use._

  ```bash
  unify-pp-cli watch add company --match domain=gladly.com
  ```

### Operational audits
- **`coverage`** — Set-difference report between two record tables on a shared key with optional bucketing by attribute and matched-but-stale rows.

  _Tells you which Salesforce accounts are missing from Unify (and vice versa), bucketed by industry/owner — the central artifact of an account-coverage review._

  ```bash
  unify-pp-cli coverage --left salesforce_account --right company --key domain --by industry --agent
  ```
- **`audit-scores`** — Flag records where two numeric attributes diverge beyond a threshold. Powers scoring sanity checks across Unify-internal and Salesforce-mirrored score fields.

  _Catches the 'false-scoring auto-deduct-50pts' class of bugs before they ship into a campaign._

  ```bash
  unify-pp-cli audit-scores --object company --field unify_score --field salesforce_lead_score --threshold 50 --agent
  ```
- **`schema diff`** — Snapshot objects, attributes, and attribute options; diff two snapshots to find adds, removes, and type changes.

  _When the SF-aligned admin adds new fields, you can prove what changed between any two points in time and act on the delta._

  ```bash
  unify-pp-cli schema diff --since 1d --agent
  ```

### AE / outbound workflows
- **`vet`** — Read a CSV column of match values, run find-unique in parallel for each, and enrich each row with exists/has_opportunity/owner/last_activity_at.

  _Pre-launch vet for an outbound sequence drops from N alt-tabs to one command. Bulk pre-flight without bothering an engineer._

  ```bash
  unify-pp-cli vet --csv /tmp/prospects.csv --object company --match-col domain --agent
  ```
- **`import-csv`** — Predict what an upsert from CSV will do (create / update / no-op counts per row) by combining the local mirror with find-unique fallbacks.

  _Stops you from running a 2k-row upsert blind. Reports per-row outcome with a writable plan you can pipe back into upsert._

  ```bash
  unify-pp-cli import-csv --object company --file /tmp/accounts.csv --match-on domain --plan
  ```

## Usage

Run `unify-pp-cli --help` for the full command reference and flag list.

## Commands

### data

Manage data

- **`unify-pp-cli data create-object`** - Create object
- **`unify-pp-cli data create-object-attribute`** - Create object attribute
- **`unify-pp-cli data create-object-attribute-option`** - Create object attribute option
- **`unify-pp-cli data create-object-record`** - Create object record
- **`unify-pp-cli data delete-object`** - Delete object
- **`unify-pp-cli data delete-object-attribute`** - Delete object attribute
- **`unify-pp-cli data delete-object-attribute-option`** - Delete object attribute option
- **`unify-pp-cli data delete-object-record`** - Delete object record
- **`unify-pp-cli data find-unique-object-record`** - Find unique object record
- **`unify-pp-cli data get-object`** - Get object
- **`unify-pp-cli data get-object-attribute`** - Get object attribute
- **`unify-pp-cli data get-object-attribute-option`** - Get object attribute option
- **`unify-pp-cli data get-object-record`** - Get object record
- **`unify-pp-cli data list-object-attribute-options`** - List object attribute options
- **`unify-pp-cli data list-object-attributes`** - List object attributes
- **`unify-pp-cli data list-objects`** - List objects
- **`unify-pp-cli data update-object`** - Update object
- **`unify-pp-cli data update-object-attribute`** - Update object attribute
- **`unify-pp-cli data update-object-attribute-option`** - Update object attribute option
- **`unify-pp-cli data update-object-record`** - Update object record
- **`unify-pp-cli data upsert-object-record`** - Upsert object record


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
unify-pp-cli data create-object --api-name example-resource

# JSON for scripting and agents
unify-pp-cli data create-object --api-name example-resource --json

# Filter to specific fields
unify-pp-cli data create-object --api-name example-resource --json --select id,name,status

# Dry run — show the request without sending
unify-pp-cli data create-object --api-name example-resource --dry-run

# Agent mode — JSON + compact + no prompts in one flag
unify-pp-cli data create-object --api-name example-resource --agent
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
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-unify -g
```

Then invoke `/pp-unify <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add unify unify-pp-mcp -e UNIFY_API_KEY=<your-key>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/unify-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `UNIFY_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "unify": {
      "command": "unify-pp-mcp",
      "env": {
        "UNIFY_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
unify-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/unify-data-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `UNIFY_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `unify-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $UNIFY_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 status='unauthorized'** — Re-issue your key at Settings → Developers in Unify and re-export UNIFY_API_KEY; the CLI sends X-Api-Key on every call.
- **search/sql returns 0 rows but the record exists in Unify** — The record isn't in the local mirror yet. Run unify-pp-cli watch add <object> --match <key>=<value>, then unify-pp-cli sync.
- **sql says `no such column: <attr>`** — Per-object record tables hold attribute values inside a JSON `attrs` column, not as typed columns. Project attributes with `json_extract(attrs, '$.<attribute>')`; only `id`, `created_at`, `updated_at`, and `attrs` are direct columns on `record_<object>` tables.
- **find-unique returns 404 unexpectedly** — Confirm the attribute you're matching on is a unique attribute (find-unique requires it). Run unify-pp-cli attrs list <object> to inspect.
- **upsert says it would create when you expected an update** — Pass --plan first: it shows per-row creates vs updates vs no-ops without writing. Adjust --match-on or merge mode and re-run.
- **schema diff shows nothing** — You need at least two snapshots. Run unify-pp-cli schema snapshot today and again later, then diff between them.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**intent-js-client**](https://github.com/unifygtm/intent-js-client) — TypeScript (11 stars)
- [**sdk-python**](https://github.com/unifygtm/sdk-python) — Python
- [**sdk-typescript**](https://github.com/unifygtm/sdk-typescript) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
