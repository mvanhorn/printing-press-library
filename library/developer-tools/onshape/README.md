# Onshape CLI

Onshape cloud CAD API CLI — manage documents, parts, assemblies, and exports from the terminal.

Printed by [@mhintz1980](https://github.com/mhintz1980) (Markimus).

## Install

The recommended path installs both the `onshape-pp-cli` binary and the `pp-onshape` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press install onshape
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install onshape --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press install onshape --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press install onshape --agent claude-code
npx -y @mvanhorn/printing-press install onshape --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/onshape-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-onshape --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-onshape --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-onshape skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-onshape. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/onshape-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `ONSHAPE_ACCESS_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "onshape": {
      "command": "onshape-pp-mcp",
      "env": {
        "ONSHAPE_ACCESS_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set up credentials

Onshape uses Access Key / Secret Key authentication (found in your Onshape account settings under **Settings > Developer > API Keys**):

```bash
export ONSHAPE_ACCESS_KEY=<your-access-key>
export ONSHAPE_SECRET_KEY=<your-secret-key>
```

Or save to config:

```bash
onshape-pp-cli auth set-token <your-access-key>
```

For self-hosted Onshape, set the base URL:

```bash
export ONSHAPE_BASE_URL=https://your-instance.onshape.com/api/v6
```

### 3. Verify setup

```bash
onshape-pp-cli doctor
```

This checks configuration, credentials, and API connectivity.

### 4. Search documents

```bash
onshape-pp-cli documents search --query "bracket" --limit 5
```

### 5. Browse a document's elements

```bash
onshape-pp-cli elements --did <document-id> --wvmid <workspace-id>
```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Agent-native CAD navigation
- **`documents search`** — Find recent CAD documents with compact structured fields that agents can carry into follow-up workspace, element, and export calls.

  _Use this first when an agent needs to locate the right assembly or part studio without burning context on full document metadata._

  ```bash
  onshape-pp-cli documents search --query Trailer --limit 5 --agent --select id,name,modifiedAt
  ```
- **`elements`** — Turn an Onshape document/workspace pair into a typed map of Part Studios, assemblies, BOM tabs, blobs, and application elements.

  _Use this after choosing a document to identify which element should feed part inspection, assembly inspection, export, or rendering._

  ```bash
  onshape-pp-cli elements --did 3cb6ad4256bb099a0e4813ab --wvm w --wvmid b3fe484986a689a317b7259b --agent --select id,name,elementType
  ```

### Assembly intelligence
- **`assemblies get`** — Fetch an assembly definition and select just the instance graph fields needed for CAD review, BOM reasoning, or downstream Blender planning.

  _Use this when an agent needs to understand assembly composition before exporting geometry or planning an animation/rendering scene._

  ```bash
  onshape-pp-cli assemblies get --did 3cb6ad4256bb099a0e4813ab --wvm w --wvmid b3fe484986a689a317b7259b --eid f22782a9f60e037e2f4d7c39 --agent --select rootAssembly.instances.id,rootAssembly.instances.name
  ```

### Export readiness
- **`parts list`** — List part IDs and names from a Part Studio so an agent can choose export targets without opening Onshape manually.

  _Use this before exporting a single component to Blender, CAM, simulation, or a rendering pipeline._

  ```bash
  onshape-pp-cli parts list --did 3cb6ad4256bb099a0e4813ab --wvm w --wvmid b3fe484986a689a317b7259b --eid 1753f0a84436bf2bc39d6da6 --agent --select name,partId
  ```

### Local state that compounds
- **`sync`** — Hydrate a local SQLite mirror so agents can search and analyze previously seen Onshape data without repeated live API calls.

  _Use this when an agent will work across many CAD documents or needs resilient offline search during a longer design/review session._

  ```bash
  onshape-pp-cli sync --resources documents --latest-only --agent
  ```

## Usage

Run `onshape-pp-cli --help` for the full command reference and flag list.

## Commands

### Documents

Search and manage Onshape documents

| Command | Description |
|---------|-------------|
| `documents search` | Search documents by name or query |
| `documents get <did>` | Get document metadata by ID |
| `documents create` | Create a new document |
| `documents delete <did>` | Delete a document |

### Elements & Assemblies

Inspect document structure, assemblies, and parts

| Command | Description |
|---------|-------------|
| `elements` | List elements (tabs) in a document workspace/version |
| `assemblies list` | List assemblies in a document |
| `assemblies get` | Get assembly definition and instances |
| `parts list` | List parts in a part studio element |
| `parts get` | Get part metadata by ID |

### Export

Export geometry and drawings in STEP, STL, IGES, and other formats

| Command | Description |
|---------|-------------|
| `exports element` | Export all parts in a part studio element |
| `exports part` | Export a single part by ID |

### Versions & Workspaces

Manage version history and branching within documents

| Command | Description |
|---------|-------------|
| `versions list` | List versions in a document |
| `versions create` | Create a new version |
| `workspaces list` | List workspaces in a document |
| `workspaces create` | Create a new workspace |
| `workspaces delete` | Delete a workspace |

### Data & Offline

Sync, search, and analyze Onshape data locally

| Command | Description |
|---------|-------------|
| `sync` | Sync API data to local SQLite for offline use |
| `search <query>` | Full-text search across synced data or live API |
| `analytics` | Run analytics queries on locally synced data |
| `export <resource>` | Export data to JSONL or JSON for backup/analysis |
| `import` | Import data from JSONL via API create/upsert |
| `tail` | Stream live changes by polling the API |
| `workflow archive` | Sync all resources for offline access |
| `workflow status` | Show local archive status |

### Utilities

| Command | Description |
|---------|-------------|
| `doctor` | Check CLI health, auth, and connectivity |
| `auth status` | Show authentication status |
| `auth setup` | Print steps for obtaining a credential |
| `auth set-token` | Save an API key to config |
| `auth logout` | Clear stored credentials |
| `api` | Browse all API endpoints by interface name |
| `profile` | Named sets of flags saved for reuse |
| `which` | Find the command that implements a capability |

## Output Formats

```bash
# Human-readable table (default in terminal)
onshape-pp-cli documents search --query "bracket"

# JSON for scripting and agents
onshape-pp-cli documents search --query "bracket" --json

# Filter to specific fields
onshape-pp-cli documents search --query "bracket" --json --select id,name,createdAt

# Compact output (key fields only)
onshape-pp-cli parts list --did <did> --wvmid <wid> --agent

# CSV for spreadsheets
onshape-pp-cli documents search --query "bracket" --csv

# Dry run — show the request without sending
onshape-pp-cli documents create --name "Test Part" --dry-run

# Save output to file
onshape-pp-cli documents search --query "bracket" --deliver file:results.json
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands accept structured JSON via `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Cookbook

### Search for a document and list its elements

```bash
# Find a document
onshape-pp-cli documents search --query "housing assembly" --limit 5 --json

# List elements (tabs) in the first result's default workspace
onshape-pp-cli elements --did <document-id> --wvmid <workspace-id>
```

### Export a part as STL

```bash
onshape-pp-cli exports part \
  --did <document-id> \
  --wvmid <workspace-id> \
  --eid <element-id> \
  --partid <part-id> \
  --format STL
```

### Export an entire part studio

```bash
onshape-pp-cli exports element \
  --did <document-id> \
  --wvmid <workspace-id> \
  --eid <element-id> \
  --format STEP
```

### Create a version checkpoint

```bash
onshape-pp-cli versions create \
  --did <document-id> \
  --workspace-id <workspace-id> \
  --name "v1.0-release" \
  --description "Approved for manufacturing"
```

### Create a new document

```bash
onshape-pp-cli documents create \
  --name "Bracket Design Rev 2" \
  --description "Updated mounting bracket for Q3 build"
```

### Browse assembly structure

```bash
onshape-pp-cli assemblies list \
  --did <document-id> \
  --wvmid <workspace-id> --json

onshape-pp-cli assemblies get \
  --did <document-id> \
  --wvmid <workspace-id> \
  --json --select id,name
```

### Sync everything for offline search

```bash
# Archive all resources locally
onshape-pp-cli workflow archive

# Full-text search across synced data
onshape-pp-cli search "m5 bolt pattern" --data-source local

# Check archive freshness
onshape-pp-cli workflow status
```

### List parts with thumbnails

```bash
onshape-pp-cli parts list \
  --did <document-id> \
  --wvmid <workspace-id> \
  --eid <element-id> \
  --with-thumbnails
```

### Incremental sync with analytics

```bash
# Sync only recent changes
onshape-pp-cli sync --since 24h

# Analyze synced data by type
onshape-pp-cli analytics --type documents --group-by owner --limit 10 --json
```

### Filter elements by type

```bash
onshape-pp-cli elements \
  --did <document-id> \
  --wvmid <workspace-id> \
  --element-type PARTSTUDIO
```

### Tail document changes in real time

```bash
onshape-pp-cli tail --interval 30s | jq 'select(.type == "document")'
```

### Delete a workspace

```bash
onshape-pp-cli workspaces delete \
  --did <document-id> \
  --wid <workspace-id> \
  --yes
```

### Export data for migration

```bash
onshape-pp-cli export documents --format jsonl --output onshape-backup.jsonl
```

### Pipe JSON body for a mutation

```bash
echo '{"name":"Custom Part","description":"Created via API"}' | \
  onshape-pp-cli documents create --stdin
```

## Health Check

```bash
onshape-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API. Sample output:

```
  OK   Config: ok
  OK   Auth: configured
  OK   Env Vars: OK 2/2 available
  OK   API: reachable (HTTP 200 at /)
  OK   Credentials: valid
  INFO Cache: unknown
  config_path: /home/user/.config/onshape-pp-cli/config.toml
  base_url: https://cad.onshape.com/api/v6
  version: 1.0.0
```

## Configuration

Config file: `~/.config/onshape-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `ONSHAPE_ACCESS_KEY` | per_call | Yes | Onshape API access key (from Settings > Developer > API Keys) |
| `ONSHAPE_SECRET_KEY` | per_call | Yes | Onshape API secret key |
| `ONSHAPE_BASE_URL` | per_call | No | Override base URL for self-hosted Onshape instances |
| `ONSHAPE_CONFIG` | per_call | No | Custom config file path |

## Troubleshooting

**Authentication errors (exit code 4)**
- Run `onshape-pp-cli doctor` to check credentials and connectivity
- Verify environment variables are set: `echo $ONSHAPE_ACCESS_KEY`
- Confirm keys are active in Onshape Settings > Developer > API Keys
- For self-hosted Onshape, check that `ONSHAPE_BASE_URL` points to your instance

**Not found errors (exit code 3)**
- Check that the document/workspace/part ID is correct
- Use `documents search` to find the right document ID
- Use `elements` to list valid element IDs within a document
- Remember `--wvm` defaults to `w` (workspace); use `v` for versions or `m` for microversions

**Rate limited (exit code 7)**
- Onshape has per-plan API rate limits
- Use `--rate-limit` to throttle requests: `onshape-pp-cli sync --rate-limit 2`
- Use `--no-cache` to bypass cache only when needed

**Usage errors (exit code 2)**
- Run the command with `--help` to see valid flags
- The CLI suggests similar flags on typos: unknown flag `--doc-id` may suggest `--did`

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
