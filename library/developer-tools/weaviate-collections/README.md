# Weaviate Collections CLI

**Full collection-config control for Weaviate Cloud, plus schema history, drift diffing, and lint that no other Weaviate tool has.**

Manage every aspect of Weaviate collections — vectorizers, replication, sharding, multi-tenancy, and property indexes — from the command line. Unlike the official weaviate-cli or community tools, this CLI keeps a local history of your collection configs so you can diff, lint, and roll back with confidence.

Learn more at [Weaviate Collections](https://github.com/weaviate).

Created by [@SomSamantray](https://github.com/SomSamantray) (SomSamantray).

## Install

The recommended path installs both the `weaviate-collections-pp-cli` binary and the `pp-weaviate-collections` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install weaviate-collections
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install weaviate-collections --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install weaviate-collections --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install weaviate-collections --agent claude-code
npx -y @mvanhorn/printing-press-library install weaviate-collections --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/weaviate-collections/cmd/weaviate-collections-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/weaviate-collections-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install weaviate-collections --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-weaviate-collections --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-weaviate-collections --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install weaviate-collections --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/weaviate-collections-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `WEAVIATE_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/weaviate-collections/cmd/weaviate-collections-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "weaviate-collections": {
      "command": "weaviate-collections-pp-mcp",
      "env": {
        "WEAVIATE_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

```bash
# Verify config and connectivity without making a live call
weaviate-collections-pp-cli doctor --dry-run

# See every collection in your cluster
weaviate-collections-pp-cli schema dump

# Inspect a collection's full config
weaviate-collections-pp-cli schema objects-get Article --json

# Save a local snapshot before making changes
weaviate-collections-pp-cli schema snapshot --label baseline

# Check for risky config before shipping
weaviate-collections-pp-cli collections lint Article

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`schema snapshot`** — Save a point-in-time copy of every collection's config to the local store, then browse history over time.

  _Use before any risky schema change so you have a rollback reference._

  ```bash
  weaviate-collections-pp-cli schema snapshot --label pre-migration
  ```
- **`schema diff`** — Diff a collection's live config against a saved snapshot or another collection.

  _Catch unintended config drift between environments or over time._

  ```bash
  weaviate-collections-pp-cli schema diff Article --against pre-migration
  ```

### Reachability mitigation
- **`collections lint`** — Flag risky collection configs: no vectorizer set, replication factor of 1, unindexed high-cardinality properties.

  _Catch production-readiness gaps before they bite in an incident._

  ```bash
  weaviate-collections-pp-cli collections lint Article
  ```

### Agent-native plumbing
- **`tenants audit`** — See tenant counts and activity status across every collection in one view.

  _Spot inactive or overloaded tenants across a multi-tenant deployment at a glance._

  ```bash
  weaviate-collections-pp-cli tenants audit --json
  ```
- **`schema export`** — Export every collection's config as one portable JSON bundle for backup or promotion to another environment.

  _Promote a schema from staging to production, or restore after an incident, without full data backup._

  ```bash
  weaviate-collections-pp-cli schema export --output schema-bundle.json
  ```

## Recipes

### Snapshot before a risky change

```bash
weaviate-collections-pp-cli schema snapshot --label pre-change && weaviate-collections-pp-cli schema objects-update Article --replication-config-factor 3
```

Save a rollback reference, then apply the change.

### Diff a select subset of a large collection config

```bash
weaviate-collections-pp-cli schema objects-get Article --agent --select class,vectorizer,replicationConfig
```

Pull only the fields that matter instead of the full nested config blob.

## Usage

Run `weaviate-collections-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `WEAVIATE_COLLECTIONS_CONFIG_DIR`, `WEAVIATE_COLLECTIONS_DATA_DIR`, `WEAVIATE_COLLECTIONS_STATE_DIR`, or `WEAVIATE_COLLECTIONS_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `WEAVIATE_COLLECTIONS_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export WEAVIATE_COLLECTIONS_HOME=/srv/weaviate-collections
weaviate-collections-pp-cli doctor
```

Under `WEAVIATE_COLLECTIONS_HOME=/srv/weaviate-collections`, the four dirs resolve to `/srv/weaviate-collections/config`, `/srv/weaviate-collections/data`, `/srv/weaviate-collections/state`, and `/srv/weaviate-collections/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "weaviate-collections": {
      "command": "weaviate-collections-pp-mcp",
      "env": {
        "WEAVIATE_COLLECTIONS_HOME": "/srv/weaviate-collections"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `WEAVIATE_COLLECTIONS_DATA_DIR` overrides an explicit `--home` for that kind. Use `WEAVIATE_COLLECTIONS_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `WEAVIATE_COLLECTIONS_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `weaviate-collections-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### indexes

Manage indexes

- **`weaviate-collections-pp-cli indexes <className>`** - Returns per-property index state including active reindex progress. This powers the UI to show live migration status.

### properties

Manage properties

- **`weaviate-collections-pp-cli properties <className>`** - Adds a new property definition to an existing collection (`className`) definition.

### schema

Manage schema

- **`weaviate-collections-pp-cli schema dump`** - Retrieves the definitions of all collections (classes) currently in the database schema.
- **`weaviate-collections-pp-cli schema objects-create`** - Defines and creates a new collection (class).<br/><br/>If [`AutoSchema`](https://docs.weaviate.io/weaviate/config-refs/collections#auto-schema) is enabled (not recommended for production), Weaviate might attempt to infer schema from data during import. Manual definition via this endpoint provides explicit control.
- **`weaviate-collections-pp-cli schema objects-delete`** - Removes a collection definition from the schema. WARNING: This action permanently deletes all data objects stored within the collection.
- **`weaviate-collections-pp-cli schema objects-get`** - Retrieve the definition of a specific collection (`className`), including its properties, configuration, and vectorizer settings.
- **`weaviate-collections-pp-cli schema objects-update`** - Updates the configuration settings of an existing collection (`className`) based on the provided definition. Note: This operation modifies mutable settings specified in the request body. It does not add properties (use `POST /schema/{className}/properties` for that) or change the collection name.

### shards

Manage shards

- **`weaviate-collections-pp-cli shards get`** - Retrieves the status of all shards associated with the specified collection (`className`). For multi-tenant collections, use the `tenant` query parameter to retrieve status for a specific tenant's shards.
- **`weaviate-collections-pp-cli shards update`** - Updates the status of a specific shard within a collection (e.g., sets it to `READY` or `READONLY`). This is typically used after resolving an underlying issue (like disk space) that caused a shard to become non-operational. There is also a convenience function in each client to set the status of all shards of a collection.

### tenants

Manage tenants

- **`weaviate-collections-pp-cli tenants create`** - Creates one or more new tenants for a specified collection (`className`). Multi-tenancy must be enabled for the collection via its definition.
- **`weaviate-collections-pp-cli tenants delete`** - Deletes one or more specified tenants from a collection (`className`). WARNING: This action permanently deletes all data associated with the specified tenants.
- **`weaviate-collections-pp-cli tenants exists`** - Checks for the existence of a specific tenant within the given collection (`className`).
- **`weaviate-collections-pp-cli tenants get`** - Retrieves a list of all tenants currently associated with the specified collection.
- **`weaviate-collections-pp-cli tenants get-one`** - Retrieves details about a specific tenant within the given collection (`className`), such as its current activity status.
- **`weaviate-collections-pp-cli tenants update`** - Updates the activity status (e.g., `ACTIVE`, `INACTIVE`, etc.) of one or more specified tenants within a collection (`className`).

### vectors

Manage vectors



### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`weaviate-collections-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`weaviate-collections-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`weaviate-collections-pp-cli learnings list`** - Inspect taught rows
- **`weaviate-collections-pp-cli learnings forget <query>`** - Undo a teach
- **`weaviate-collections-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`weaviate-collections-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`weaviate-collections-pp-cli teach-pattern`** - Install a query/resource template up front
- **`weaviate-collections-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `WEAVIATE_COLLECTIONS_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `weaviate-collections-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
weaviate-collections-pp-cli indexes mock-value

# JSON for scripting and agents
weaviate-collections-pp-cli indexes mock-value --json

# Filter to specific fields
weaviate-collections-pp-cli indexes mock-value --json --select id,name,status

# Dry run — show the request without sending
weaviate-collections-pp-cli indexes mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
weaviate-collections-pp-cli indexes mock-value --agent
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
weaviate-collections-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `weaviate-collections-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/weaviate-collections-pp-cli/config.toml`; `--home`, `WEAVIATE_COLLECTIONS_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `WEAVIATE_API_KEY` | per_call | Yes | Set to your API credential. |
| `WEAVIATE_COLLECTIONS_BASE_URL` | per_call | Yes | Your Weaviate Cloud cluster URL, e.g. `https://your-cluster-id.weaviate.cloud/v1` (find it in the Weaviate Cloud console). Every cluster has a unique hostname, so there is no usable default — `doctor` reports "not configured" until this is set. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `weaviate-collections-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `weaviate-collections-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $WEAVIATE_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Unauthorized on every command** — Set WEAVIATE_API_KEY to your Weaviate Cloud API key (Bearer token).
- **Empty collections list on a real cluster** — Run 'weaviate-collections-pp-cli sync' to refresh the local store from the live API.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**weaviate-cli**](https://github.com/weaviate/weaviate-cli) — Python
- [**weave-cli**](https://github.com/maximilien/weave-cli) — Go
- [**mcp-weaviate**](https://github.com/sajal2692/mcp-weaviate) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
