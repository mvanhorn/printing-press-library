# Attentive CLI

Use this CLI with Attentive API credentials and the public Attentive API surface.

Created by [@debgotwired](https://github.com/debgotwired) (Deb Mukherjee).

## Install

The recommended path installs both the `attentive-pp-cli` binary and the `pp-attentive` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install attentive
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install attentive --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install attentive --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install attentive --agent claude-code
npx -y @mvanhorn/printing-press-library install attentive --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/attentive/cmd/attentive-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/attentive-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install attentive --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-attentive --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-attentive --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install attentive --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/attentive-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `ATTENTIVE_BEARER_AUTH` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/attentive/cmd/attentive-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "attentive": {
      "command": "attentive-pp-mcp",
      "env": {
        "ATTENTIVE_BEARER_AUTH": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your access token from your API provider's developer portal, then store it:

```bash
attentive-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set it via environment variable:

```bash
export ATTENTIVE_BEARER_AUTH="your-token-here"
```

### 3. Verify Setup

```bash
attentive-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
attentive-pp-cli segments list
```

## Usage

Run `attentive-pp-cli --help` for the full command reference and flag list.

## Commands

### bulk

Manage bulk

- **`attentive-pp-cli bulk get-job-status`** - Checks the status of a bulk ingestion job identified by bulkJobId. This endpoint returns the current state of the job (e.g. `PENDING`, `IN_PROGRESS`, `COMPLETED`, `FAILED`) along with metadata such as timestamps and error messages if applicable.

If the job has completed successfully, the response includes a downloadable link to a `.jsonl` (JSON Lines) file containing a record of each request and its corresponding response. User's can audit results or process downstream outcomes of the bulk operation. You can fetch requests up to 2 weeks old.

Scopes Required: No Additional scopes required.

Default Rate Limit: 100 requests per second
- **`attentive-pp-cli bulk post-segment-members`** - Add members to a segment in bulk. This endpoint accepts 1 to 10,000 members per request. Members are identified by email, phone number, and/or client user ID. The request is validated, queued for asynchronous processing, and a unique batch job ID is returned for tracking the status.

**Request Limits:**
- Minimum: 1 member per request
- Maximum: 10,000 members per request
- At least one identifier (email, phone, or clientUserId) required per member

**Processing:**
- Jobs are processed asynchronously
- Use the batch job ID to check status via `GET /v2/bulk/job/{bulkJobId}`
- Results available as downloadable `.jsonl` file when completed

Scopes Required: [segments:all]

Default Rate Limit: 100 requests per second
- **`attentive-pp-cli bulk post-user-attributes`** - This endpoint allows clients to submit multiple user attribute updates in bulk, accepting up to 256 payloads per request. Each request is validated, and a unique batch ID is returned for tracking the status of the batch.

Scopes Required: [attributes:all, subscriptions:all]

Default Rate Limit: 100 requests per second

### me

Manage me

- **`attentive-pp-cli me`** - Make a call to this endpoint to test your unique token that you generate in the Attentive product.

### segments

Endpoints for submitting bulk segment member additions and removals. Use these endpoints to manage segment memberships in bulk and monitor the processing status asynchronously.

## Processing Times

The Bulk API processes jobs with the following targets:
- **Standard Processing**: The first 10,000 records per day per customer typically complete within 4 hours of request acceptance.
- **High-Volume Processing**: Additional records beyond 10,000 per day typically complete within 12 hours.

**Note**: For jobs with more than 1 million records, processing times may vary.

- **`attentive-pp-cli segments create`** - Creates a new empty segment with the specified name and optional description.

Scopes Required: [segments:Write]

Default Rate Limit: 100 requests per second
- **`attentive-pp-cli segments delete-by-external-id`** - Archives (soft deletes) a segment by external ID. The segment will no longer be visible in list operations but can be restored if needed.

Scopes Required: [segments:Write]

Default Rate Limit: 100 requests per second
- **`attentive-pp-cli segments get-by-external-id`** - Retrieves segment details by external ID.

Scopes Required: [segments:Read OR segments:Write]

Default Rate Limit: 100 requests per second
- **`attentive-pp-cli segments list`** - Lists segments with optional filtering by name, external ID, or update timestamp.

Scopes Required: [segments:Read OR segments:Write]

Default Rate Limit: 100 requests per second
- **`attentive-pp-cli segments patch-by-external-id`** - Partially updates an existing segment. Only provided fields will be updated.

Scopes Required: [segments:Write]

Default Rate Limit: 100 requests per second

### user

Manage user

- **`attentive-pp-cli user`** - Creates or updates a single user record, including associated attributes, subscriptions, and identifiers. If a user with the provided identifiers already exists, their information will be updated; otherwise, a new user will be created.

There is a limit of 100 of custom attributes that can be created. If intending to update an existing attribute, the name of the key must match the name of the existing attribute. If an existing attribute does not exist, a new attribute will be created with the given key as the name. Attributes with enumerated values must have a value that matches an existing enum value; new enum values will not be created.  Attempting to pass custom attributes as an array or a map such as `["New York City]` or `{"favorite city": "Boston"}` will result in a 400 error.

Default Rate Limit: 150 requests per second


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
attentive-pp-cli segments list

# JSON for scripting and agents
attentive-pp-cli segments list --json

# Filter to specific fields
attentive-pp-cli segments list --json --select id,name,status

# Dry run — show the request without sending
attentive-pp-cli segments list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
attentive-pp-cli segments list --agent
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
attentive-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/attentive-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `ATTENTIVE_BEARER_AUTH` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `attentive-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `attentive-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $ATTENTIVE_BEARER_AUTH`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
