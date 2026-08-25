# Cosmos CLI

**Search, organize, export, and audit your Cosmos inspiration library from the terminal.**

cosmos-pp-cli mirrors the replayable Cosmos GraphQL surface, then adds confirmed writes, JSON and HTML exports, local membership snapshots, cross-collection analysis, and attribution-aware workflows. Browser automation was used only during development discovery; normal commands use ordinary HTTP and remain scriptable.

Learn more at [Cosmos](https://api.cosmos.so).

Created by [@elliottrjacobs](https://github.com/elliottrjacobs) (Elliott Jacobs).

## Install

The recommended path installs both the `cosmos-pp-cli` binary and the `pp-cosmos` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install cosmos
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install cosmos --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install cosmos --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install cosmos --agent claude-code
npx -y @mvanhorn/printing-press-library install cosmos --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/cosmos/cmd/cosmos-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/cosmos-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install cosmos --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-cosmos --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-cosmos --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install cosmos --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/cosmos-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Enter the name of an existing tenant-gated Printing Press client profile when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/cosmos/cmd/cosmos-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "cosmos": {
      "command": "cosmos-pp-mcp",
      "env": {
        "PRINTING_PRESS_CLIENT_PROFILE": "cosmos-default"
      }
    }
  }
}
```

</details>

## Authentication

Public discovery commands work without a token. Personal collections and writes use a Cosmos bearer token supplied through `COSMOS_TOKEN`. Run `cosmos-pp-cli auth identify --json`, then create a tenant-gated client profile whose credential reference is `env:COSMOS_TOKEN`; credentials are never included in exported manifests or JSON output.

## Quick Start

```bash
# Search public inspiration
cosmos-pp-cli discover elements 'brutalist typography' --limit 12 --json

# Inspect your collections after authenticating
cosmos-pp-cli collection list --json

# Build a local history
cosmos-pp-cli sync --resources collections,elements --full

# Review the week's curation
cosmos-pp-cli review --since 7d --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`review`** — Turn recent saves, unfiled elements, duplicate connections, AI flags, and missing attribution into a deterministic maintenance queue.

  _Use it before a weekly curation session to see exactly what needs attention._

  ```bash
  cosmos-pp-cli review --since 7d --json
  ```

### Cross-collection intelligence
- **`collection overlap`** — Compare two collections and report shared elements, duplicate media, and references unique to each side.

  _Use it to keep project moodboards distinct and avoid recycling the same references._

  ```bash
  cosmos-pp-cli collection overlap 101 202 --json
  ```
- **`collection coverage`** — Compare live Cosmos search results with live collection membership and return promising references that are not already saved.

  _Use it when a board feels repetitive and you need genuinely new references._

  ```bash
  cosmos-pp-cli collection coverage --collection 101 --query 'brutalist typography' --limit 20 --json
  ```

### Trustworthy creative archives
- **`provenance audit`** — Report missing source URLs or authors and show source concentration from live collection elements.

  _Use it before publishing or handing off a moodboard so references remain attributable._

  ```bash
  cosmos-pp-cli provenance audit --collection 101 --json
  ```
- **`snapshot diff`** — Compare historical collection membership and show added, removed, and moved elements.

  _Use it to audit a team's curation changes or reproduce an earlier moodboard state._

  ```bash
  cosmos-pp-cli snapshot diff --from 7d --to now --json
  ```

### Discovery that compounds
- **`element trail`** — Walk visual similarity results to a bounded depth and emit a deduplicated, source-aware graph.

  _Use it to explore a visual direction without losing the path back to the seed reference._

  ```bash
  cosmos-pp-cli element trail --id 2113061259 --depth 2 --limit 12 --json
  ```

## Recipes

### Export an attributed project collection

```bash
cosmos-pp-cli export collection 101 --output ./collection-101.json --json
```

Exports normalized element, media, and source-attribution metadata as JSON.

### Find gaps in a moodboard

```bash
cosmos-pp-cli collection coverage --collection 101 --query 'warm modernist interiors' --limit 30 --json
```

Returns live search candidates not already connected to the collection.

### Audit attribution before handoff

```bash
cosmos-pp-cli provenance audit --collection 101 --json
```

Shows missing sources and concentration by source domain without crawling third-party sites.

## Usage

Run `cosmos-pp-cli --help` for the full command reference and flag list. The primary human-facing surface is:

- `auth identify`, `identity`, `whoami`, `client add|validate|set-default`
- `discover elements|collections|all|featured`
- `collection list|show|elements|search|create|create-sub|connect|disconnect|overlap|coverage`
- `element show|similar|connections|save-url|trail`
- `activity list`, `feed`, `import status`
- `export collection|gallery`, `sync`, `review`, `provenance audit`, `snapshot diff`

Generated operation-shaped commands remain hidden expert CLI internals. They are not registered as MCP tools; MCP exposes only the curated command surface.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `data.db` and private, client-profile-scoped `cosmos-snapshots/*.json` files |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `COSMOS_CONFIG_DIR`, `COSMOS_DATA_DIR`, `COSMOS_STATE_DIR`, or `COSMOS_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `COSMOS_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export COSMOS_HOME=/srv/cosmos
cosmos-pp-cli doctor
```

Under `COSMOS_HOME=/srv/cosmos`, the four dirs resolve to `/srv/cosmos/config`, `/srv/cosmos/data`, `/srv/cosmos/state`, and `/srv/cosmos/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "cosmos": {
      "command": "cosmos-pp-mcp",
      "env": {
        "COSMOS_HOME": "/srv/cosmos"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `COSMOS_DATA_DIR` overrides an explicit `--home` for that kind. Use `COSMOS_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `COSMOS_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Cosmos tokens remain in `COSMOS_TOKEN`; this CLI does not persist them to its config or data directories. Run `cosmos-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

The public CLI exposes a curated, stable command vocabulary. Captured GraphQL operation families are internal implementation details and cannot be executed directly.

- `discover elements|collections|all|featured` - search and browse visual references
- `collection list|show|elements|search|create|create-sub|connect|disconnect|overlap|coverage` - organize and analyze collections
- `element show|similar|connections|save-url|trail` - inspect, save, and explore elements
- `activity list` and `feed` - inspect recent activity and recommendations
- `review` and `provenance audit` - audit organization and attribution
- `sync` and `snapshot diff` - maintain profile-scoped local history
- `export collection|gallery` - export to an explicit file path
- `import status` - inspect provider-managed import progress
- `identity`, `whoami`, and `doctor` - verify auth and tenant binding

Run `cosmos-pp-cli <command> --help` for the current flags and examples.

## Output Formats

```bash
# Human-readable output (default in terminal, JSON when piped)
cosmos-pp-cli collection list

# JSON for scripting and agents
cosmos-pp-cli collection list --json

# Filter to specific fields
cosmos-pp-cli collection list --json --select id,name

# Dry run — show the request without sending
cosmos-pp-cli collection create "Campaign references" --dry-run

# Agent mode — JSON + compact + no prompts in one flag
cosmos-pp-cli discover elements "editorial typography" --agent
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
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
cosmos-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `cosmos-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/cosmos-pp-cli/config.toml`; `--home`, `COSMOS_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `COSMOS_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `cosmos-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `cosmos-pp-cli doctor` to check credentials
- Verify the environment variable is present without printing it: `test -n "$COSMOS_TOKEN"`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **A personal command returns 401** — Set `COSMOS_TOKEN`, run `cosmos-pp-cli auth identify --json`, and validate the tenant-gated client profile. Replace an expired token in the environment rather than storing credentials in CLI configuration.
- **A GraphQL command reports an unknown field** — The Cosmos web API is undocumented and may have drifted; rerun discovery or update the stored operation document.
- **Snapshot diff reports no data** — Run `cosmos-pp-cli sync --resources collections,elements --full` at least twice before comparing history.

## Discovery Signals

This CLI was generated from a sanitized, authenticated, read-only browser capture of `https://www.cosmos.so/`.

- 37 distinct GraphQL operations were retained after removing CORS, notification-hub, analytics, and unrelated traffic.
- The API endpoint is `https://api.cosmos.so/graphql`; standard HTTP works and no browser runtime is required.
- Personal operations resolve `COSMOS_TOKEN` in memory through the selected client profile and send it as `Authorization: Bearer …`.
- Mutation documents used by login, create, save, connect, and disconnect were cross-checked against the community Cosmos MCP implementation because the official web app capture was intentionally read-only.
- The durable capture is scalar-redacted and contains no authorization headers or account identifiers.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**jpoindexter/cosmos-mcp**](https://github.com/jpoindexter/cosmos-mcp) — typescript
- [**rclaycock/cosmos-scraper-mk-3**](https://github.com/rclaycock/cosmos-scraper-mk-3) — javascript
- [**rawpage/suggaplay**](https://github.com/rawpage/suggaplay) — javascript
- [**rslosh/promptbox**](https://github.com/rslosh/promptbox) — typescript
- [**likeahuman-ai/roxit-masterclass cosmos-inspo**](https://github.com/likeahuman-ai/roxit-masterclass) — shell

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
