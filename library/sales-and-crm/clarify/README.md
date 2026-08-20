# Clarify CLI

**Every Clarify API operation as a typed command, plus the morning briefing, meeting prep, and pipeline analytics the autonomous CRM knows about but cannot run.**

Clarify auto-builds your CRM from email, calendar, and meetings, but its only programmatic surfaces are a hosted MCP server and raw curl. This CLI covers all 75 API operations with the api-key auth scheme and JSON:API envelope handled natively, keeps a local SQLite mirror with transcript full-text search, and adds commands like prep, brief, followup, and dossier that no Clarify surface offers.

## Install

The recommended path installs both the `clarify-pp-cli` binary and the `pp-clarify` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install clarify
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install clarify --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install clarify --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install clarify --agent claude-code
npx -y @mvanhorn/printing-press-library install clarify --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/clarify/cmd/clarify-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/clarify-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install clarify --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-clarify --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-clarify --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install clarify --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/clarify-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `CLARIFY_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/clarify/cmd/clarify-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "clarify": {
      "command": "clarify-pp-mcp",
      "env": {
        "CLARIFY_CAMPAIGN_ID": "<campaignId>",
        "CLARIFY_ENTITY": "<entity>",
        "CLARIFY_ID": "<id>",
        "CLARIFY_OBJECT": "<object>",
        "CLARIFY_WORKSPACE": "<workspace>",
        "CLARIFY_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Clarify authenticates with an API key sent as `Authorization: api-key <key>` — not a Bearer token. Create a Personal key in Clarify under Settings, API Keys, then set `CLARIFY_API_KEY` to the raw key; the CLI adds the `api-key` scheme prefix for you. Every request is scoped to a workspace slug (visible in your Clarify login URL); set it once with `CLARIFY_WORKSPACE` or the config file.

## Quick Start

```bash
# verify the binary, config, and auth wiring before touching the API
clarify-pp-cli doctor --dry-run

# mirror one object type locally; repeat with object=person, company, meeting, task to mirror them all
clarify-pp-cli sync --resources resources --path-context object=deal

# start-of-day overview: today's meetings, their companies, and open deals
clarify-pp-cli brief

# deals going cold, grouped by stage
clarify-pp-cli stale --days 14 --json

# full-text search across everything synced, including meeting transcripts
clarify-pp-cli search "acme" --limit 10

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Rituals the CRM knows but cannot run
- **`prep`** — One command before a call: the meeting's attendees, their company, open deals, and transcript excerpts from past meetings with that company.

  _Reach for this when the task is preparing for one specific upcoming meeting rather than fetching raw records. Requires a synced local mirror (sync --resources resources --path-context object=<type> first)._

  ```bash
  clarify-pp-cli prep --next --agent
  ```
- **`brief`** — Start-of-day overview: today's meetings joined to their companies, open deals, and yesterday's record activity, on one screen.

  _Use this for a whole-day overview; use prep for a single meeting. Requires a synced local mirror (sync --resources resources --path-context object=<type> first)._

  ```bash
  clarify-pp-cli brief --json
  ```
- **`followup`** — The dropped-ball list: meetings with no subsequent activity, comment, or task on the linked deal or company.

  _Run it after a busy week to find meetings that never got a follow-up; --no-deal also surfaces companies with meetings but no open deal. Requires a synced local mirror (sync --resources resources --path-context object=<type> first)._

  ```bash
  clarify-pp-cli followup --since 7d --json
  ```

### Pipeline analytics the API does not have
- **`stale`** — Open deals with no activity in N days, grouped by pipeline stage.

  _The Monday pipeline-review question answered in one command instead of a CSV export. Requires a synced local mirror (sync --resources resources --path-context object=<type> first)._

  ```bash
  clarify-pp-cli stale --days 14 --json
  ```
- **`velocity`** — Per-stage dwell time and stage-to-stage conversion counts, accrued from a local stage-history table across repeated runs (the first run reports the current stage distribution).

  _Answers 'how long do deals sit in each stage' without exporting anything to a spreadsheet. Requires a synced local mirror; dwell and conversion analytics build up as you re-run sync and velocity over time._

  ```bash
  clarify-pp-cli velocity --json
  ```
- **`dupes`** — Finds likely duplicate people or companies by shared email, domain, or normalized name, and prints ready-to-run merge commands.

  _Weekly hygiene sweep for auto-built CRM data; each finding comes with the exact merge invocation to fix it. Requires a synced local mirror (sync --resources resources --path-context object=<type> first)._

  ```bash
  clarify-pp-cli dupes --type person --json
  ```

### Agent-native plumbing
- **`dossier`** — A complete background bundle on any record: fields, relationships, activities, comments, and related meetings with transcript references, in one compact payload.

  _The one-call answer to 'tell me everything about this person/company/deal' — use prep instead when the subject is a specific upcoming meeting. Requires a synced local mirror._

  ```bash
  clarify-pp-cli dossier 5f8b7d2e-9c4a-4e1b-8f3d-2a6c9e0b4d71 --agent --select record,related
  ```

## Recipes

### Prep for your next call

```bash
clarify-pp-cli prep --next --agent
```

Attendees, their company, open deals, and past-transcript excerpts in one compact payload.

### Find the week's dropped balls

```bash
clarify-pp-cli followup --since 7d --json
```

Meetings with no follow-up activity, comment, or task on the linked deal or company.

### Narrow a big deal query for an agent

```bash
clarify-pp-cli objects resources get my-workspace deal --agent --select data.attributes.name,data.attributes.amount,data.attributes.stage
```

JSON:API responses are deep; --select with dotted paths keeps only the fields the agent needs.

### Upsert a lead by email

```bash
clarify-pp-cli objects records create my-workspace person --match-on email_addresses --data-type person --data-attributes '{"name":{"first_name":"Jane","last_name":"Doe"},"email_addresses":{"items":["jane@example.com"]}}' --dry-run
```

match_on turns the insert into an upsert against the person unique field; drop --dry-run to send it.

### Weekly dupe sweep

```bash
clarify-pp-cli dupes --type company --json
```

Likely duplicates by shared domain or normalized name, each with a ready-to-run merge command.

## Usage

Run `clarify-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `CLARIFY_CONFIG_DIR`, `CLARIFY_DATA_DIR`, `CLARIFY_STATE_DIR`, or `CLARIFY_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `CLARIFY_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export CLARIFY_HOME=/srv/clarify
clarify-pp-cli doctor
```

Under `CLARIFY_HOME=/srv/clarify`, the four dirs resolve to `/srv/clarify/config`, `/srv/clarify/data`, `/srv/clarify/state`, and `/srv/clarify/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "clarify": {
      "command": "clarify-pp-mcp",
      "env": {
        "CLARIFY_HOME": "/srv/clarify"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `CLARIFY_DATA_DIR` overrides an explicit `--home` for that kind. Use `CLARIFY_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `CLARIFY_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `clarify-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### campaigns

Manage campaigns


### comments

Manage comments

- **`clarify-pp-cli comments create`** - Creates a comment on a record. The comment body is rich text, and `owner_id` plus `entity` identify the record it is attached to. Returns the created comment row.
- **`clarify-pp-cli comments delete`** - Permanently deletes a comment. A comment can be deleted by its author, by an admin, or by the owner of the agent that posted it. Returns the deleted comment. This cannot be undone.
- **`clarify-pp-cli comments get`** - Returns a single comment by its ID.
- **`clarify-pp-cli comments update`** - Replaces the body of an existing comment. Only the comment’s author may edit it. Returns the updated comment.

### layouts

Manage layouts

- **`clarify-pp-cli layouts get-by-id`** - Returns a single layout by its ID. A layout defines the UI structure — navigation, record page regions — rendered for the workspace.
- **`clarify-pp-cli layouts update`** - Replaces the layout’s `tree` and returns the updated layout.

### lists

Manage lists

- **`clarify-pp-cli lists <workspace>`** - Returns every list across all object types in the workspace as a paginated JSON:API collection. Filter by object type with `filter[entity]`. Dynamic lists are excluded unless you pass a matching `filter`.

### meetings

Manage meetings


### objects

Manage objects


### schemas

Manage schemas

- **`clarify-pp-cli schemas create-custom-object`** - Creates a new custom object type in the workspace and returns its generated JSON Schema. The name is normalized to a `c_`-prefixed identifier (for example "Sales Order" becomes `c_sales_order`).
- **`clarify-pp-cli schemas delete-custom-object`** - Deletes a custom object type and all of its records. The deletion runs asynchronously: the response returns a task you can poll to track progress. Only custom (`c_*`) objects can be deleted. This cannot be undone.
- **`clarify-pp-cli schemas get`** - Returns every object schema in the workspace as a cursor-paginated list of JSON:API resources. Each resource holds the full JSON Schema for one object type, including its fields, relationships, and presentation metadata.
- **`clarify-pp-cli schemas patch-enum-field-values`** - Adds or removes options on enum (single- and multi-select) fields for one object type. The request carries exactly one item in `data`; each field in `meta` names an `append` or `remove` operation. To modify several object types, send separate requests.
- **`clarify-pp-cli schemas update-entity`** - Replaces the full JSON Schema for an object type. The request body is the complete schema document, and its `$id` must match the object in the path. The change is applied asynchronously: the response returns a task you can poll to track progress.

### settings

Manage settings

- **`clarify-pp-cli settings delete-workspace`** - Removes the stored value of a workspace setting so it falls back to its default. Read-only settings are rejected, and some settings require admin permissions.
- **`clarify-pp-cli settings read-all-workspace`** - Returns every workspace setting keyed by name, with defaults applied for settings the workspace has not overridden.
- **`clarify-pp-cli settings read-workspace`** - Returns the value of a single workspace setting; the default value when the workspace has not overridden it.
- **`clarify-pp-cli settings write-workspace`** - Sets the value of a workspace setting by key. Read-only settings are rejected, and some settings require admin permissions.

### users

Manage users

- **`clarify-pp-cli users get`** - Returns the workspace’s users as a paginated JSON:API list. Each user includes their roles.
- **`clarify-pp-cli users get-workspaces`** - Returns a single workspace user as a JSON:API resource, including their roles and the time they were last active.

### workflows

Manage workflows

- **`clarify-pp-cli workflows create`** - Creates a workflow from a trigger and a set of blocks. Create it disabled and enable it once the automation graph is complete. Returns the created workflow.
- **`clarify-pp-cli workflows delete`** - Deletes a workflow. The deletion is applied asynchronously and the response body is empty. This cannot be undone.
- **`clarify-pp-cli workflows get`** - Returns the workspace’s workflows as an offset-paginated list of JSON:API resources. Use the `type` filter to return only sequences (campaigns) or general workflows.
- **`clarify-pp-cli workflows get-workspaces`** - Returns a single workflow as a JSON:API resource, including its trigger and the blocks that make up its automation graph.
- **`clarify-pp-cli workflows update`** - Applies a partial update to a workflow: only the fields present in `attributes` are changed. Use it to rename, enable or disable, or edit the trigger and blocks. Returns the updated workflow.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`clarify-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`clarify-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`clarify-pp-cli learnings list`** - Inspect taught rows
- **`clarify-pp-cli learnings forget <query>`** - Undo a teach
- **`clarify-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`clarify-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`clarify-pp-cli teach-pattern`** - Install a query/resource template up front
- **`clarify-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `CLARIFY_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `clarify-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
clarify-pp-cli comments get mock-value mock-value

# JSON for scripting and agents
clarify-pp-cli comments get mock-value mock-value --json
# Filter to specific fields by name
clarify-pp-cli comments get mock-value mock-value --json --select <field>[,<field>...]

# Dry run — show the request without sending
clarify-pp-cli comments get mock-value mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
clarify-pp-cli comments get mock-value mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Explicit confirmation** - `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Runtime Endpoint

This CLI resolves endpoint placeholders at runtime, so one installed binary can target different tenants or API versions without regeneration.

Endpoint environment variables:
- `CLARIFY_CAMPAIGN_ID` resolves `{campaignId}`
- `CLARIFY_ENTITY` resolves `{entity}`
- `CLARIFY_ID` resolves `{id}`
- `CLARIFY_OBJECT` resolves `{object}`
- `CLARIFY_WORKSPACE` resolves `{workspace}`

Base URL: `https://api.clarify.ai/v1`

## Health Check

```bash
clarify-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `clarify-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/clarify-pp-cli/config.toml`; `--home`, `CLARIFY_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `CLARIFY_CAMPAIGN_ID` | endpoint | Yes |  |
| `CLARIFY_ENTITY` | endpoint | Yes |  |
| `CLARIFY_ID` | endpoint | Yes |  |
| `CLARIFY_OBJECT` | endpoint | Yes |  |
| `CLARIFY_WORKSPACE` | endpoint | Yes |  |
| `CLARIFY_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `clarify-pp-cli doctor` reports `agentcookie: detected` and `auth status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `clarify-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $CLARIFY_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Invalid API key even though the key is correct** — Clarify keys are workspace-scoped: confirm CLARIFY_WORKSPACE matches the workspace the key was created in, and that the key is a Personal key from Settings, API Keys.
- **Multi-value fields like email_addresses fail to write** — Collection fields take {"items": [...]} — pass --email-addresses as a JSON object with an items array, not a bare list.
- **429 Too Many Requests during imports** — Use the bulk endpoints (records bulk) instead of per-record calls; the limit is 3000 requests per minute per endpoint and the client honors Retry-After.
- **brief, stale, or velocity return empty results** — These read the local mirror: run clarify-pp-cli sync --resources resources --path-context object=deal (and again for person, company, meeting, task) first.
