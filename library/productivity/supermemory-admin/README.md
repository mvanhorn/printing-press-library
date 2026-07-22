# Supermemory Admin CLI

**A project-scoped admin CLI for Supermemory memory, recall, documents, profiles, and container tags.**

Supermemory's API is powerful, but agents need a repeatable operator surface: auth setup, project scoping, dry runs, compact JSON output, local sync/search, and MCP parity.

## Install

The recommended path installs both the `supermemory-admin-pp-cli` binary and the `pp-supermemory-admin` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install supermemory-admin
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install supermemory-admin --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install supermemory-admin --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install supermemory-admin --agent claude-code
npx -y @mvanhorn/printing-press-library install supermemory-admin --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/supermemory-admin/cmd/supermemory-admin-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/supermemory-admin-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install supermemory-admin --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-supermemory-admin --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-supermemory-admin --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install supermemory-admin --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/supermemory-admin-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SUPERMEMORY_ADMIN_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/supermemory-admin/cmd/supermemory-admin-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "supermemory-admin": {
      "command": "supermemory-admin-pp-mcp",
      "env": {
        "SUPERMEMORY_ADMIN_TOKEN": "<your-key>",
        "SUPERMEMORY_ADMIN_PROJECT": "<optional-project-id>"
      }
    }
  }
}
```

</details>

## Authentication

Create a Supermemory API key and set SUPERMEMORY_ADMIN_TOKEN. Optionally set SUPERMEMORY_ADMIN_PROJECT to scope every request with x-sm-project.

## Quick Start

```bash
# Verify config, base URL reachability, and auth hints.
supermemory-admin-pp-cli doctor --agent

# Preview the recall request body without sending credentials or making a live call.
supermemory-admin-pp-cli supermemory-recall --q "deployment context" --agent --dry-run

# Preview a project-scoped profile request using the x-sm-project header.
supermemory-admin-pp-cli profiles --container-tag project_123 --agent --dry-run

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Operator Safety
- **`SUPERMEMORY_ADMIN_PROJECT=<project-id> supermemory-admin-pp-cli supermemory-recall`** — Scope every CLI and MCP request to one Supermemory project with the x-sm-project header, without hand-editing raw headers.

  _Keeps agent memories partitioned by project/codebase while preserving the same CLI surface._

  ```bash
  SUPERMEMORY_ADMIN_PROJECT=project_123 supermemory-admin-pp-cli supermemory-recall --q "deployment context" --agent
  ```

### Recall
- **`supermemory-recall`** — Run low-latency memory recall from a compact, agent-friendly CLI surface with selectable JSON output.

  _Lets agents retrieve focused memory context without opening a dashboard or carrying broad history in prompt context._

  ```bash
  supermemory-admin-pp-cli supermemory-recall --q "deployment context" --agent --select results.id,results.memory,results.similarity
  ```

### Local Intelligence
- **`sync + search`** — Sync compatible Supermemory resources into local SQLite for offline search and inspection.

  _Gives operators a local audit/search loop for memory-adjacent resources._

  ```bash
  supermemory-admin-pp-cli sync --agent && supermemory-admin-pp-cli search "project context" --agent
  ```

## Usage

Run `supermemory-admin-pp-cli --help` for the full command reference and flag list.

## Commands

### connection_resources

Manage connection resources

- **`supermemory-admin-pp-cli connection-resources <connectionId>`** - Fetch resources for a connection (supported providers: GitHub for now)

### connections

External service integrations

- **`supermemory-admin-pp-cli connections delete-v3-by-id`** - Delete a specific connection by ID
- **`supermemory-admin-pp-cli connections delete-v3-by-provider`** - Delete connection for a specific provider and container tags
- **`supermemory-admin-pp-cli connections get-v3-by-id`** - Get connection details with id
- **`supermemory-admin-pp-cli connections post-v3-by-provider`** - Initialize connection and get authorization URL
- **`supermemory-admin-pp-cli connections post-v3-list`** - List all connections

### container-tags

Manage container tags

- **`supermemory-admin-pp-cli container-tags delete-v3-by`** - Delete a container tag and all its documents and memories. Only organization owners and admins can perform this action.
- **`supermemory-admin-pp-cli container-tags get-v3-by`** - Get settings for a container tag
- **`supermemory-admin-pp-cli container-tags patch-v3-by`** - Update settings for a container tag
- **`supermemory-admin-pp-cli container-tags post-v3-merge`** - Merge multiple container tags into a target tag. All documents from the source tags will be updated to reference the target tag, and the source tags will be deleted after successful merge.

### conversations

Manage conversations

- **`supermemory-admin-pp-cli conversations`** - Ingest or update a conversation

### documents

List, get, and search documents

- **`supermemory-admin-pp-cli documents delete-v3-bulk`** - Bulk delete documents by IDs or container tags
- **`supermemory-admin-pp-cli documents delete-v3-by-id`** - Delete a document by ID or customId
- **`supermemory-admin-pp-cli documents get-v3-by-id`** - Get a document by ID
- **`supermemory-admin-pp-cli documents get-v3-processing`** - Get documents that are currently being processed
- **`supermemory-admin-pp-cli documents patch-v3-by-id`** - Update a document with any content type (text, url, file, etc.) and metadata
- **`supermemory-admin-pp-cli documents post-v3`** - Add a document with any content type (text, url, file, etc.) and metadata
- **`supermemory-admin-pp-cli documents post-v3-batch`** - Add multiple documents in a single request. Each document can have any content type (text, url, file, etc.) and metadata
- **`supermemory-admin-pp-cli documents post-v3-file`** - Upload a file to be processed
- **`supermemory-admin-pp-cli documents post-v3-list`** - Retrieves a paginated list of documents with their metadata and workflow status
- **`supermemory-admin-pp-cli documents post-v3-search`** - Search memories with advanced filtering

### memories

Manage memories

- **`supermemory-admin-pp-cli memories delete-v4`** - Forget (soft delete) a memory entry. The memory is marked as forgotten but not permanently deleted.
- **`supermemory-admin-pp-cli memories patch-v4`** - Update a memory by creating a new version. The original memory is preserved with isLatest=false.
- **`supermemory-admin-pp-cli memories post-v4`** - Create memories directly, bypassing the document ingestion workflow. Generates embeddings and makes them immediately searchable.
- **`supermemory-admin-pp-cli memories post-v4-list`** - List all latest memory entries from specified container tags with their update history and source documents

### profiles

Entity profiles for users, participants, or any entity — includes profile search

- **`supermemory-admin-pp-cli profiles`** - Get user profile with optional search results

### settings

Organization settings

- **`supermemory-admin-pp-cli settings get-v3`** - Get settings for an organization
- **`supermemory-admin-pp-cli settings patch-v3`** - Update settings for an organization
- **`supermemory-admin-pp-cli settings post-v3-reset`** - Reset organization content: removes documents, memories, spaces (except default project), connections, and org settings. Preserves the org, members, and billing.

### supermemory_recall

Manage supermemory recall

- **`supermemory-admin-pp-cli supermemory-recall`** - Search memory entries - Low latency for conversational


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
supermemory-admin-pp-cli connection-resources mock-value

# JSON for scripting and agents
supermemory-admin-pp-cli connection-resources mock-value --json

# Filter to specific fields
supermemory-admin-pp-cli connection-resources mock-value --json --select id,name,status

# Dry run — show the request without sending
supermemory-admin-pp-cli connection-resources mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
supermemory-admin-pp-cli connection-resources mock-value --agent
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
supermemory-admin-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/supermemory-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SUPERMEMORY_ADMIN_TOKEN` | per_call | Yes | Set to your API credential. |
| `SUPERMEMORY_ADMIN_PROJECT` | per_call | No | Optional project id. When set, requests include `x-sm-project` to scope operations. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `supermemory-admin-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `supermemory-admin-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SUPERMEMORY_ADMIN_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
