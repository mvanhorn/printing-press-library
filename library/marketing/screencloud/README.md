# ScreenCloud CLI

**A Playgrounds release and fleet-operations CLI that understands both Studio and the app content service.**

Inspect the maintained ScreenCloud Studio v2.103.0 reference snapshot, synchronize sanitized placement metadata locally, and coordinate Playgrounds files, data, previews, scoped tokens, and capability diagnostics from one guarded workflow. Release impact, readiness, config drift, contract checks, and recovery plans compose multiple ScreenCloud surfaces into repeatable operator workflows.

## Install

The recommended path installs both the `screencloud-pp-cli` binary and the `pp-screencloud` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install screencloud
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install screencloud --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install screencloud --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install screencloud --agent claude-code
npx -y @mvanhorn/printing-press-library install screencloud --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/screencloud/cmd/screencloud-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/screencloud-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install screencloud --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-screencloud --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-screencloud --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install screencloud --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/screencloud-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SCREENCLOUD_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/screencloud/cmd/screencloud-pp-cli@latest
go install github.com/mvanhorn/printing-press-library/library/marketing/screencloud/cmd/screencloud-pp-mcp@latest
```

Both binaries are required because the MCP server mirrors the guarded CLI command tree. Alternatively, set `SCREENCLOUD_CLI_PATH` to the installed companion CLI.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "screencloud": {
      "command": "screencloud-pp-mcp",
      "env": {
        "SCREENCLOUD_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Set SCREENCLOUD_API_KEY to the organization API key shown in ScreenCloud Studio's Developer page, set SCREENCLOUD_ORGANIZATION_ID (or pass org current --expected-org-id) to enable fail-closed organization matching, and set the region-specific GraphQL endpoint when it differs from the default. The CLI conditionally verifies currentOrgId against that expected organization and can compare currentToken/currentUser structure with the published permission catalog without printing raw grants. Management and viewer JWTs require separate --yes approval, are redacted from normal output, and are never persisted.

## Quick Start

```bash
# Validate configuration and local prerequisites without using credentials or changing ScreenCloud.
screencloud-pp-cli doctor --dry-run

# Verify that the API key belongs to the intended organization.
screencloud-pp-cli org current --expected-org-id "$SCREENCLOUD_ORGANIZATION_ID" --agent

# Inspect least-privilege capability states without exposing raw grants or changing ScreenCloud.
screencloud-pp-cli auth capabilities --for sync --agent --select summary,capabilities,partial_visibility,authorization_proof,visibility

# Build the sanitized placement topology used by fleet and impact analysis.
screencloud-pp-cli sync --resources apps,spaces,app-installs,app-instances,app-versions,channels,playlists,screens,associations,share-associations --max-pages 10

# With fresh approval, populate sanitized files/data timestamps for each Playgrounds app; otherwise readiness remains incomplete.
screencloud-pp-cli playgrounds preview-drift --refresh --app-uuid 6f14d9d8-7e6d-42a1-9bb4-0a3d75a8a123 --space-id 11223344-5566-4788-99aa-bbccddeeff00 --yes --agent

# Turn the synchronized topology into actionable Playgrounds readiness findings.
screencloud-pp-cli playgrounds readiness --agent --select summary,findings,complete,hint

```

## Unique Features

These purpose-built compositions go beyond raw endpoint mirrors by combining ScreenCloud Studio, Playgrounds, local evidence, and explicit safety gates.

### Release confidence
- **`playgrounds impact`** — Map the synchronized spaces, channels, playlists, and screens related to a reviewed Playgrounds working copy before publishing.

  _Use this before a publish when an agent must explain the known synchronized placement graph and its completeness, not merely show a file fingerprint._

  ```bash
  screencloud-pp-cli playgrounds impact 6f14d9d8-7e6d-42a1-9bb4-0a3d75a8a123 --dir ./fixtures/playgrounds --home /tmp/screencloud-pp-sample-home --agent
  ```
- **`playgrounds contract-check`** — Verify the current management and viewer content contracts after minting ephemeral scoped JWTs, without changing Playgrounds content.

  _Use this before automation relies on the bundle-derived Playgrounds contract or after an unexpected response shape._

  ```bash
  screencloud-pp-cli playgrounds contract-check --app-uuid 6f14d9d8-7e6d-42a1-9bb4-0a3d75a8a123 --space-id 11223344-5566-4788-99aa-bbccddeeff00 --agent --yes
  ```
- **`playgrounds preview-drift`** — Find unpublished previews, production-ahead conflicts, and preview work that has waited too long.

  _Use this when an agent needs a fleet-level preview queue rather than inspecting preview and production workspaces one at a time._

  ```bash
  screencloud-pp-cli playgrounds preview-drift --older-than 7d --home /tmp/screencloud-pp-sample-home --agent
  ```

### Fleet intelligence
- **`playgrounds readiness`** — Find missing, inactive, outdated, dangling, and inconsistent Playgrounds deployments across the organization.

  _Use this instead of separate list calls when an agent needs an actionable organization-wide health verdict._

  ```bash
  screencloud-pp-cli playgrounds readiness --home /tmp/screencloud-pp-sample-home --agent --select summary,findings,complete,hint
  ```
- **`playgrounds config-drift`** — Detect structurally divergent Playgrounds configurations without storing or revealing private values.

  _Use this when an agent must compare a fleet safely without pulling sensitive configuration values into context._

  ```bash
  screencloud-pp-cli playgrounds config-drift --app-uuid 6f14d9d8-7e6d-42a1-9bb4-0a3d75a8a123 --home /tmp/screencloud-pp-sample-home --agent
  ```

### Safe recovery
- **`playgrounds create-reconcile`** — Turn a partial create receipt into a resume or cleanup plan; a no-op conclusion requires live verification.

  _Use this after an interrupted two-service create workflow instead of guessing which mutation is safe to repeat. The example uses a shipped fixture; use your own redacted receipt path for real recovery._

  ```bash
  screencloud-pp-cli playgrounds create-reconcile --receipt ./fixtures/receipts/summer-campaign.json --verify-live --yes --dry-run --agent
  ```

### Safe automation
- **`auth capabilities`** — Explain whether the current identity appears able to run a supported mapped command without exposing token material or raw effective grants.

  _Use this before automation or a guarded mutation needs a least-privilege preflight; it is diagnostic evidence, not authorization or a guarantee of mutation success._

  ```bash
  screencloud-pp-cli auth capabilities --for 'playgrounds files put' --agent --select summary,capabilities,partial_visibility,authorization_proof,visibility
  ```

## Recipes

### Audit Playgrounds fleet health

```bash
screencloud-pp-cli playgrounds readiness --agent --select summary,findings,complete,hint
```

After bounded sync and per-app timestamp refresh, joins instances, installs, spaces, versions, and sanitized content evidence; absent, stale, or truncated evidence returns complete=false.

### Map a working-copy change

```bash
screencloud-pp-cli playgrounds impact 6f14d9d8-7e6d-42a1-9bb4-0a3d75a8a123 --dir ./campaign-playground --agent
```

Uses a working-copy fingerprint and the synchronized sanitized relationship graph to map related spaces, channels, playlists, and screens; completeness reflects required mirror state.

### Check the live Playgrounds contract

```bash
screencloud-pp-cli playgrounds contract-check --app-uuid 6f14d9d8-7e6d-42a1-9bb4-0a3d75a8a123 --space-id 11223344-5566-4788-99aa-bbccddeeff00 --agent --yes
```

After fresh approval, mints ephemeral management and viewer JWTs and performs content-read-only assertions; it never persists the tokens or changes Playgrounds content.

### Find aging previews

```bash
screencloud-pp-cli playgrounds preview-drift --older-than 7d --agent
```

Surfaces drift only for apps whose sanitized timestamps were previously populated with preview-drift --refresh --app-uuid <id> --space-id <id> --yes; missing evidence returns complete=false.

### Check least-privilege readiness

```bash
screencloud-pp-cli auth capabilities --for 'playgrounds files put' --agent --select summary,capabilities,partial_visibility,authorization_proof,visibility
```

Explains available, missing, or unknown capabilities for a supported mapped command; partial permission visibility and unmapped commands fail closed as unknown.

## Usage

Run `screencloud-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `SCREENCLOUD_CONFIG_DIR`, `SCREENCLOUD_DATA_DIR`, `SCREENCLOUD_STATE_DIR`, or `SCREENCLOUD_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `SCREENCLOUD_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export SCREENCLOUD_HOME=/srv/screencloud
screencloud-pp-cli doctor
```

Under `SCREENCLOUD_HOME=/srv/screencloud`, the four dirs resolve to `/srv/screencloud/config`, `/srv/screencloud/data`, `/srv/screencloud/state`, and `/srv/screencloud/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "screencloud": {
      "command": "screencloud-pp-mcp",
      "env": {
        "SCREENCLOUD_HOME": "/srv/screencloud"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `SCREENCLOUD_DATA_DIR` overrides an explicit `--home` for that kind. Use `SCREENCLOUD_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `SCREENCLOUD_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `screencloud-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Abbreviated Generated Endpoint Reference

The generated endpoint table below covers the absorbed GraphQL endpoint. Run `screencloud-pp-cli --help`, `screencloud-pp-cli agent-context --pretty`, or subcommand `--help` for the complete command tree, including Studio, Playgrounds, sync, search, runtime, token, and diagnostic commands.

### graphql

Execute maintained or user-supplied Studio GraphQL documents

- **`screencloud-pp-cli graphql`** - Execute a Studio GraphQL document with optional variables


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`screencloud-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`screencloud-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`screencloud-pp-cli learnings list`** - Inspect taught rows
- **`screencloud-pp-cli learnings forget <query>`** - Undo a teach
- **`screencloud-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`screencloud-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`screencloud-pp-cli teach-pattern`** - Install a query/resource template up front
- **`screencloud-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `SCREENCLOUD_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `screencloud-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable output (default in terminal, JSON when piped)
screencloud-pp-cli graphql request --query 'query { currentOrgId }'

# JSON for scripting and agents
screencloud-pp-cli graphql request --query 'query { currentOrgId }' --json

# Filter to specific fields
screencloud-pp-cli graphql request --query 'query { currentOrgId }' --json --select data

# Dry run — show the request without sending
screencloud-pp-cli graphql request --query 'query { currentOrgId }' --dry-run

# Agent mode — JSON + compact + no prompts; mutations still require a separate --yes
screencloud-pp-cli graphql request --query 'query { currentOrgId }' --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` summarizes the planned request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - every live mutation requires a separate `--yes`; `--agent` does not imply approval
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `1` general runtime error, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
screencloud-pp-cli doctor
```

Checks configuration, credential presence and storage, and endpoint reachability. It does not prove that a credential is valid; use `org current` for an authenticated harmless probe.

## Configuration

Run `screencloud-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/screencloud-pp-cli/config.toml`; `--home`, `SCREENCLOUD_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SCREENCLOUD_API_KEY` | per_call | Conditional | Organization API key; stored credentials are also supported. |
| `SCREENCLOUD_BASE_URL` | per_call | No | Override the regional Studio GraphQL base URL. |
| `SCREENCLOUD_PLAYGROUNDS_URL` | per_call | No | Override the Playgrounds content-service URL. |
| `SCREENCLOUD_ORGANIZATION_ID` | per_call | No | Expected organization UUID enforced by `org current`; `--expected-org-id` takes precedence. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `screencloud-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `screencloud-pp-cli doctor` to check credentials
- Run `screencloud-pp-cli auth inspect --json`; it reports presence and source without printing the credential.
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **The endpoint returns HTTP 200 but the command fails.** — Inspect the reported GraphQL errors; ScreenCloud can return operation failures in the errors array with HTTP 200.
- **Authentication works but the organization check fails.** — Confirm SCREENCLOUD_ORGANIZATION_ID and the region-specific GraphQL endpoint match the organization shown in Studio's Developer page.
- **A Playgrounds command reports contract drift.** — After fresh approval to mint scoped JWTs, run screencloud-pp-cli playgrounds contract-check with --app-uuid, --space-id, and --yes; its content assertions are read-only.
- **A local fleet command reports unsynced or stale data.** — Run a bounded sync including channels, playlists, screens, associations, and share-associations, review query cost and freshness, then retry.
- **A capability check reports unknown or permission-incomplete.** — Treat the result as fail-closed, verify the API key role in Studio, and do not infer mutation authorization from a partial permission view.

## Provenance and limitations

The Studio surface is grounded in ScreenCloud's official v2.103.0 GraphQL reference. The Playgrounds file, data, preview, and viewer contract was corroborated from authenticated browser traffic and production bundles because ScreenCloud does not publish the same stable reference for that service. Runtime GraphQL introspection was unavailable, live content mutation testing was intentionally excluded, and absent or stale local mirror evidence is reported as incomplete rather than healthy.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
