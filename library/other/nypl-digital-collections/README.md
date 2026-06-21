# Nypl Digital Collections CLI

New York Public Library Digital Collections API. Source docs: https://api.repo.nypl.org/. Note: NYPL states the Repo API is being deprecated and will no longer be available starting August 1st, 2026.

Printed by [@kierandotai](https://github.com/kierandotai) (kierandotai).

## Install

The recommended path installs both the `nypl-digital-collections-pp-cli` binary and the `pp-nypl-digital-collections` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install nypl-digital-collections
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install nypl-digital-collections --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install nypl-digital-collections --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install nypl-digital-collections --agent claude-code
npx -y @mvanhorn/printing-press-library install nypl-digital-collections --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/nypl-digital-collections/cmd/nypl-digital-collections-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/nypl-digital-collections-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-nypl-digital-collections --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-nypl-digital-collections --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-nypl-digital-collections skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-nypl-digital-collections. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/nypl-digital-collections-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `NYPL_DIGITAL_COLLECTIONS_NYPL_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/nypl-digital-collections/cmd/nypl-digital-collections-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "nypl-digital-collections": {
      "command": "nypl-digital-collections-pp-mcp",
      "env": {
        "NYPL_DIGITAL_COLLECTIONS_NYPL_TOKEN": "<your-key>"
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

Get your API key from your API provider's developer portal. The key typically looks like a long alphanumeric string.

```bash
export NYPL_DIGITAL_COLLECTIONS_NYPL_TOKEN="<paste-your-key>"
```

You can also persist this in your config file at `~/.config/nypl-digital-collections-pp-cli/config.toml`.

### 3. Verify Setup

```bash
nypl-digital-collections-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
nypl-digital-collections-pp-cli collections list
```

## Usage

Run `nypl-digital-collections-pp-cli --help` for the full command reference and flag list.

## Commands

### collections

Manage collections

- **`nypl-digital-collections-pp-cli collections get`** - Returns paginated child captures for a collection or subcollection/container UUID.
- **`nypl-digital-collections-pp-cli collections list`** - Returns paginated information about collections.

### items

Manage items

- **`nypl-digital-collections-pp-cli items get-counts`** - Returns collection item counts.
- **`nypl-digital-collections-pp-cli items get-details`** - Returns MODS for a capture UUID plus related capture information. UUID must belong to a valid capture.
- **`nypl-digital-collections-pp-cli items get-featured`** - Returns featured items.
- **`nypl-digital-collections-pp-cli items get-mets-alto`** - Returns METS ALTO for a given capture UUID.
- **`nypl-digital-collections-pp-cli items get-minified-alto`** - Returns minified ALTO for a given capture UUID.
- **`nypl-digital-collections-pp-cli items get-mods-captures`** - Returns MODS and capture information for a capture, item, container, or collection UUID.
- **`nypl-digital-collections-pp-cli items get-plain-text`** - Returns parsed plain text ALTO for a given capture UUID.
- **`nypl-digital-collections-pp-cli items get-rights`** - Returns rights profile information for a UUID.
- **`nypl-digital-collections-pp-cli items get-total`** - Returns the total number of digitized items.
- **`nypl-digital-collections-pp-cli items list-all-collection-captures`** - Returns all capture UUIDs, image IDs, item links, and titles for a capture, item, container, or collection UUID.
- **`nypl-digital-collections-pp-cli items list-captures`** - Returns capture UUIDs, image IDs, item links, and titles for an item, container, or collection UUID. Capture UUIDs are not valid input values.
- **`nypl-digital-collections-pp-cli items list-collection-captures`** - Returns capture UUIDs, image IDs, item links, and titles for a capture, item, container, or collection UUID.
- **`nypl-digital-collections-pp-cli items list-recent`** - Returns the most recently added captures.
- **`nypl-digital-collections-pp-cli items list-root`** - Returns all top-level UUIDs for collections and orphan items.
- **`nypl-digital-collections-pp-cli items lookup-identifier`** - Returns UUIDs for a given identifier type and identifier value such as local_image_id, local_bnumber, local_barcode, oclc, isbn, or lccn.
- **`nypl-digital-collections-pp-cli items search-digital`** - Returns results matching keywords anywhere in a MODS metadata record. Supports field-specific search, fuzzy/exact field type, optional filters, pagination, and public-domain filtering.

### mods

Manage mods

- **`nypl-digital-collections-pp-cli mods <uuid>`** - Returns MODS bibliographic data for a capture, item, container, or collection UUID.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
nypl-digital-collections-pp-cli collections list

# JSON for scripting and agents
nypl-digital-collections-pp-cli collections list --json

# Filter to specific fields
nypl-digital-collections-pp-cli collections list --json --select id,name,status

# Dry run — show the request without sending
nypl-digital-collections-pp-cli collections list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
nypl-digital-collections-pp-cli collections list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
nypl-digital-collections-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/nypl-digital-collections-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `NYPL_DIGITAL_COLLECTIONS_NYPL_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `nypl-digital-collections-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $NYPL_DIGITAL_COLLECTIONS_NYPL_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
