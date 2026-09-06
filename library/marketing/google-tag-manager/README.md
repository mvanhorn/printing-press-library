# Google Tag Manager CLI

**Mirror any GTM container into a local database, then audit, diff, and query it from the terminal — everything the GTM console can't do, read-only and safe against production.**

The Google Tag Manager console can't diff anything, can't bulk-query, can't cross-reference the tag/trigger/variable graph, and can't export to version control. This CLI pulls a container into local SQLite and turns all of that into one command: audit hygiene, diff workspace-vs-live or container-vs-container, inventory GA4 ids and Consent Mode coverage across your whole fleet, and full-text search every parameter. It is read-only by construction — the mutating endpoints are stripped, so it is safe to point at a production container.

Learn more at [Google Tag Manager](https://google.com).

Created by [@stellato](https://github.com/stellato) (Greg Stellato).

## Install

The recommended path installs both the `google-tag-manager-pp-cli` binary and the `pp-google-tag-manager` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install google-tag-manager
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install google-tag-manager --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install google-tag-manager --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install google-tag-manager --agent claude-code
npx -y @mvanhorn/printing-press-library install google-tag-manager --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/google-tag-manager/cmd/google-tag-manager-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/google-tag-manager-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install google-tag-manager --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-google-tag-manager --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-google-tag-manager --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install google-tag-manager --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/google-tag-manager-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `GTM_ACCESS_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/google-tag-manager/cmd/google-tag-manager-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "google-tag-manager": {
      "command": "google-tag-manager-pp-mcp",
      "env": {
        "GTM_ACCESS_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Google Tag Manager has no API key — it uses OAuth 2.0. Provide a read-scoped access token as GTM_ACCESS_TOKEN. The quickest path with gcloud: run `gcloud auth application-default login --scopes=https://www.googleapis.com/auth/tagmanager.readonly`, then `export GTM_ACCESS_TOKEN=$(gcloud auth application-default print-access-token)`. Alternatively use a service account whose email is granted Viewer access in GTM admin and mint a token for it. The CLI sends the token as `Authorization: Bearer <token>` and only ever needs the read-only scope.

## Quick Start

```bash
# Check the token and API reachability before anything else
google-tag-manager-pp-cli doctor

# Find your account id
google-tag-manager-pp-cli tagmanager accounts-list

# Mirror the live container into the local store
google-tag-manager-pp-cli pull --account 6012345 --container 9876543 --live

# Run the hygiene battery against the mirror
google-tag-manager-pp-cli audit --json

# Find where a measurement id is referenced
google-tag-manager-pp-cli search "G-ABCDE12345"

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local mirror that compounds
- **`pull`** — Walk an entire GTM container — every tag, trigger, variable, folder, template, client, zone, and gtag config — into a local SQLite store in one command.

  _Reach for this first — every other command reads the mirror, and re-pulls become time-stamped snapshots for history/drift._

  ```bash
  google-tag-manager-pp-cli pull --account 6012345 --container 9876543 --live
  ```
- **`export`** — Emit a deterministic flattened representation of the container for clean version-control diffs, or container-export-compatible JSON.

  _Use to check GTM config into git and review changes in pull requests._

  ```bash
  google-tag-manager-pp-cli export --flat
  ```

### Hygiene, compliance & governance
- **`audit`** — Run a battery of hygiene checks against the mirror: dead tags, orphan triggers, unused variables, paused tags, Custom HTML tags, and tags firing on All Pages.

  _Use to gate a release in CI or sanity-check a container before publishing; typed exit code fails the build on high-severity findings._

  ```bash
  google-tag-manager-pp-cli audit --json --fail-on high
  ```
- **`consent-report`** — Per-tag consentSettings coverage and a list of tags that fire without consent gating, classified by vendor (ads vs analytics).

  _Use to answer 'are we Consent Mode v2 compliant for the EEA' without clicking through every tag._

  ```bash
  google-tag-manager-pp-cli consent-report --json
  ```

### Diff & fleet visibility
- **`diff`** — Field-level diff between two snapshots: workspace vs live, version vs version, or container vs container, with stable name+type keys so renames are not add+remove.

  _Use before publishing to see exactly what changed, or to compare two properties' containers._

  ```bash
  google-tag-manager-pp-cli diff live workspace:7
  ```
- **`fleet`** — One table across all pulled containers: GA4/measurement-id inventory, Consent Mode coverage, Custom HTML count, and entity counts per container.

  _Use when you manage containers across multiple properties and need a portfolio view the console can never give._

  ```bash
  google-tag-manager-pp-cli fleet --metric ga4 --json
  ```
- **`search`** — Substring search across every entity's name, type, parameters, and notes — find hardcoded GA4 ids, stray pixels, or leftover URLs across all pulled containers.

  _Use to track down where a measurement id, domain, or vendor pixel is referenced before changing or removing it._

  ```bash
  google-tag-manager-pp-cli search "G-ABCDE12345"
  ```

### Dependency graph
- **`uses`** — Show everything that references a variable or trigger — the safe-to-delete / what-breaks-if-I-touch-this check.

  _Use before deleting or renaming a variable or trigger to see its blast radius._

  ```bash
  google-tag-manager-pp-cli uses "DLV - userId"
  ```
- **`fires`** — Bidirectional graph walk: which tags fire on a trigger, or which triggers and variables a tag depends on.

  _Use for incident debugging — 'why did this fire' or 'what fires here'._

  ```bash
  google-tag-manager-pp-cli fires --trigger 42 --json
  ```

## Recipes


### Pre-publish review

```bash
google-tag-manager-pp-cli diff live workspace:7 --json
```

See exactly what a workspace changes relative to the published live container before publishing.

### Consent Mode v2 audit

```bash
google-tag-manager-pp-cli consent-report --json --select tags.name,tags.consentStatus
```

List every tag with its consent gating status, narrowed to just name and status for a compact agent-friendly payload.

### Fleet GA4 inventory

```bash
google-tag-manager-pp-cli fleet --metric ga4 --csv
```

Export which GA4 measurement ids appear in which containers across your whole estate as CSV.

### Blast-radius before deleting a variable

```bash
google-tag-manager-pp-cli uses "DLV - userId" --json
```

List every tag and trigger that references a variable so you know what breaks before touching it.

### Find stray pixels

```bash
google-tag-manager-pp-cli search "facebook" --json --select results.container,results.name,results.type
```

Full-text search every entity across all pulled containers for leftover vendor pixels, narrowing the nested response to the fields that matter.

## Usage

Run `google-tag-manager-pp-cli --help` for the full command reference and flag list.

## Commands

### tagmanager

Manage tagmanager

- **`google-tag-manager-pp-cli tagmanager accounts-containers-destinations-list`** - Lists all Destinations linked to a GTM Container.
- **`google-tag-manager-pp-cli tagmanager accounts-containers-environments-list`** - Lists all GTM Environments of a GTM Container.
- **`google-tag-manager-pp-cli tagmanager accounts-containers-list`** - Lists all Containers that belongs to a GTM Account.
- **`google-tag-manager-pp-cli tagmanager accounts-containers-lookup`** - Looks up a Container by destination ID.
- **`google-tag-manager-pp-cli tagmanager accounts-containers-snippet`** - Gets the tagging snippet for a Container.
- **`google-tag-manager-pp-cli tagmanager accounts-containers-version-headers-latest`** - Gets the latest container version header
- **`google-tag-manager-pp-cli tagmanager accounts-containers-version-headers-list`** - Lists all Container Versions of a GTM Container.
- **`google-tag-manager-pp-cli tagmanager accounts-containers-versions-live`** - Gets the live (i.e. published) container version
- **`google-tag-manager-pp-cli tagmanager accounts-containers-workspaces-built-in-variables-list`** - Lists all the enabled Built-In Variables of a GTM Container.
- **`google-tag-manager-pp-cli tagmanager accounts-containers-workspaces-clients-list`** - Lists all GTM Clients of a GTM container workspace.
- **`google-tag-manager-pp-cli tagmanager accounts-containers-workspaces-folders-list`** - Lists all GTM Folders of a Container.
- **`google-tag-manager-pp-cli tagmanager accounts-containers-workspaces-get-status`** - Finds conflicting and modified entities in the workspace.
- **`google-tag-manager-pp-cli tagmanager accounts-containers-workspaces-gtag-config-list`** - Lists all Google tag configs in a Container.
- **`google-tag-manager-pp-cli tagmanager accounts-containers-workspaces-list`** - Lists all Workspaces that belong to a GTM Container.
- **`google-tag-manager-pp-cli tagmanager accounts-containers-workspaces-tags-list`** - Lists all GTM Tags of a Container.
- **`google-tag-manager-pp-cli tagmanager accounts-containers-workspaces-templates-list`** - Lists all GTM Templates of a GTM container workspace.
- **`google-tag-manager-pp-cli tagmanager accounts-containers-workspaces-triggers-list`** - Lists all GTM Triggers of a Container.
- **`google-tag-manager-pp-cli tagmanager accounts-containers-workspaces-variables-list`** - Lists all GTM Variables of a Container.
- **`google-tag-manager-pp-cli tagmanager accounts-containers-workspaces-zones-list`** - Lists all GTM Zones of a GTM container workspace.
- **`google-tag-manager-pp-cli tagmanager accounts-list`** - Lists all GTM Accounts that a user has access to.
- **`google-tag-manager-pp-cli tagmanager accounts-user-permissions-get`** - Gets a user's Account & Container access.
- **`google-tag-manager-pp-cli tagmanager accounts-user-permissions-list`** - List all users that have access to the account along with Account and Container user access granted to each of them.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
google-tag-manager-pp-cli tagmanager accounts-containers-destinations-list mock-value

# JSON for scripting and agents
google-tag-manager-pp-cli tagmanager accounts-containers-destinations-list mock-value --json

# Filter to specific fields
google-tag-manager-pp-cli tagmanager accounts-containers-destinations-list mock-value --json --select id,name,status

# Dry run — show the request without sending
google-tag-manager-pp-cli tagmanager accounts-containers-destinations-list mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
google-tag-manager-pp-cli tagmanager accounts-containers-destinations-list mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
google-tag-manager-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/tag-manager-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `GTM_ACCESS_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `google-tag-manager-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `google-tag-manager-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $GTM_ACCESS_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 missing authentication credential** — GTM_ACCESS_TOKEN is unset or expired; mint a fresh one with gcloud auth application-default print-access-token
- **403 insufficient permission / scope** — Re-run gcloud auth application-default login with --scopes=https://www.googleapis.com/auth/tagmanager.readonly, or grant the service account Viewer access in GTM admin
- **audit/diff returns nothing** — Run pull first — the query commands read the local mirror, not the live API
- **403 on a container you can see in the UI** — The token's identity differs from your GTM login; confirm the token's account has access to that container

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**gtm-tools**](https://github.com/lessmess-dev/gtm-tools) — JavaScript
- [**google-api-go-client (tagmanager/v2)**](https://github.com/googleapis/google-api-go-client) — Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
