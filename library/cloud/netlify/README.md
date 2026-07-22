# Netlify CLI

**Every Netlify API endpoint, plus a local SQLite mirror of your whole account so you can search, diff, and audit across all sites at once — offline and agent-native.**

The official netlify-cli is stateless and per-site. netlify-pp-cli syncs your sites, deploys, DNS, env vars, and form submissions into a local database, then adds cross-site commands no other tool has: env-drift compares variables across sites, submissions search runs offline full-text search over every form, dns-audit flags dangling records, and overview shows the health of every site in one view.

Learn more at [Netlify](https://www.netlify.com/docs/api/).

Created by [@Selrach84](https://github.com/Selrach84) (Charles Denzel Segovia).

## Install

The recommended path installs both the `netlify-pp-cli` binary and the `pp-netlify` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install netlify
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install netlify --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install netlify --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install netlify --agent claude-code
npx -y @mvanhorn/printing-press-library install netlify --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/netlify/cmd/netlify-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/netlify-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install netlify --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-netlify --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-netlify --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install netlify --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/netlify-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `NETLIFY_AUTH_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/netlify/cmd/netlify-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "netlify": {
      "command": "netlify-pp-mcp",
      "env": {
        "NETLIFY_FORM_ID": "<form_id>",
        "NETLIFY_AUTH_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

```bash
# check config and API reachability before anything else
netlify-pp-cli doctor --dry-run

# confirm auth works and list your sites
netlify-pp-cli sites list --json

# mirror your account into the local database
netlify-pp-cli sync --json

# cross-site health once the mirror is populated
netlify-pp-cli overview --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Cross-site state that compounds
- **`overview`** — One command shows build status, last deploy time, and form counts for every site in your account.

  _Reach for this to survey an entire account at once instead of paging through the dashboard per site._

  ```bash
  netlify-pp-cli overview --json --select name,last_deploy,state
  ```
- **`env-drift`** — Finds env vars that exist on some sites but are missing or differ on others, across contexts.

  _Use before a release to catch a secret set on staging but missing on production._

  ```bash
  netlify-pp-cli env-drift --json
  ```
- **`dns-audit`** — Aggregates DNS records across all zones and flags records pointing at deleted sites and soon-expiring SNI certs.

  _Use to find dangling DNS after tearing down sites, in one pass over the whole account._

  ```bash
  netlify-pp-cli dns-audit --json
  ```

### Offline search
- **`submissions search`** — Full-text search across every form submission from every site, offline.

  _Reach for this to find a specific contact/lead across all forms without clicking through the UI._

  ```bash
  netlify-pp-cli submissions search "refund" --json --limit 20
  ```
- **`since`** — Shows deploys and builds across all sites within a time window (e.g. last 2h).

  _Reach for this to answer 'what shipped recently' across an account after an incident._

  ```bash
  netlify-pp-cli since 2h --json
  ```
- **`deploy-diff`** — Compares two deploys of a site (state, commit, context, error) side by side from the local mirror.

  _Use to see exactly what changed between a working deploy and a broken one when hunting a regression._

  ```bash
  netlify-pp-cli deploy-diff <deploy-a> <deploy-b> --json
  ```

## Recipes

### Audit env vars before a release

```bash
netlify-pp-cli env-drift --json --select key,sites_missing
```

Lists variables set on some sites but missing on others so nothing ships half-configured.

### Find a lead across all forms

```bash
netlify-pp-cli submissions search "acme corp" --json --select site_name,form_name,email,created_at
```

Offline full-text search over every submission with only the fields an agent needs.

### Post-incident deploy review

```bash
netlify-pp-cli since 4h --json --select site_name,state,commit_ref
```

Shows every deploy across the account in the last four hours.

## Usage

Run `netlify-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `NETLIFY_CONFIG_DIR`, `NETLIFY_DATA_DIR`, `NETLIFY_STATE_DIR`, or `NETLIFY_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `NETLIFY_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export NETLIFY_HOME=/srv/netlify
netlify-pp-cli doctor
```

Under `NETLIFY_HOME=/srv/netlify`, the four dirs resolve to `/srv/netlify/config`, `/srv/netlify/data`, `/srv/netlify/state`, and `/srv/netlify/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "netlify": {
      "command": "netlify-pp-mcp",
      "env": {
        "NETLIFY_HOME": "/srv/netlify"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `NETLIFY_DATA_DIR` overrides an explicit `--home` for that kind. Use `NETLIFY_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `NETLIFY_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `netlify-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### accounts

Manage accounts

- **`netlify-pp-cli accounts cancel`** - Cancel
- **`netlify-pp-cli accounts create`** - Create
- **`netlify-pp-cli accounts get`** - Get
- **`netlify-pp-cli accounts list-for-user`** - List for user
- **`netlify-pp-cli accounts list-types-for-user`** - List types for user
- **`netlify-pp-cli accounts update`** - Update

### agent-runners

Manage agent runners

- **`netlify-pp-cli agent-runners create`** - Create
- **`netlify-pp-cli agent-runners create-upload-url`** - Create upload url
- **`netlify-pp-cli agent-runners delete`** - Delete
- **`netlify-pp-cli agent-runners get`** - Get
- **`netlify-pp-cli agent-runners list`** - List
- **`netlify-pp-cli agent-runners update`** - Update

### ai-gateway

Manage ai gateway

- **`netlify-pp-cli ai-gateway`** - Get providers

### billing

Manage billing

- **`netlify-pp-cli billing`** - List payment methods for user

### builds

Manage builds

- **`netlify-pp-cli builds <build_id>`** - Get site

### deploy-keys

Manage deploy keys

- **`netlify-pp-cli deploy-keys create`** - Create
- **`netlify-pp-cli deploy-keys delete`** - Delete
- **`netlify-pp-cli deploy-keys get`** - Get
- **`netlify-pp-cli deploy-keys list`** - List

### deploys

Manage deploys

- **`netlify-pp-cli deploys delete`** - Delete
- **`netlify-pp-cli deploys get`** - Get

### dns-zones

Manage dns zones

- **`netlify-pp-cli dns-zones create`** - Create
- **`netlify-pp-cli dns-zones delete`** - Delete
- **`netlify-pp-cli dns-zones get`** - Get
- **`netlify-pp-cli dns-zones get-dnszones`** - Get dnszones

### forms

Manage forms


### hooks

Manage hooks

- **`netlify-pp-cli hooks create-by-site-id`** - Create by site id
- **`netlify-pp-cli hooks delete`** - Delete
- **`netlify-pp-cli hooks get`** - Get
- **`netlify-pp-cli hooks list-by-site-id`** - List by site id
- **`netlify-pp-cli hooks list-types`** - List types
- **`netlify-pp-cli hooks update`** - Update

### oauth

Manage oauth

- **`netlify-pp-cli oauth create-ticket`** - Create ticket
- **`netlify-pp-cli oauth exchange-ticket`** - Exchange ticket
- **`netlify-pp-cli oauth show-ticket`** - Show ticket

### purge

Manage purge

- **`netlify-pp-cli purge`** - Purges cached content from Netlify's CDN. Supports purging by Cache-Tag.

### services

Manage services

- **`netlify-pp-cli services get`** - Get
- **`netlify-pp-cli services show`** - Show

### sites

Manage sites

- **`netlify-pp-cli sites create`** - **Note:** Environment variable keys and values have moved from `build_settings.env` and `repo.env` to a new endpoint. Please use [createEnvVars](#tag/environmentVariables/operation/createEnvVars) to create environment variables for a site.
- **`netlify-pp-cli sites delete`** - Delete
- **`netlify-pp-cli sites get`** - **Note:** Environment variable keys and values have moved from `build_settings.env` and `repo.env` to a new endpoint. Please use [getEnvVars](#tag/environmentVariables/operation/getEnvVars) to retrieve site environment variables.
- **`netlify-pp-cli sites list`** - **Note:** Environment variable keys and values have moved from `build_settings.env` and `repo.env` to a new endpoint. Please use [getEnvVars](#tag/environmentVariables/operation/getEnvVars) to retrieve site environment variables.
- **`netlify-pp-cli sites update`** - **Note:** Environment variable keys and values have moved from `build_settings.env` and `repo.env` to a new endpoint. Please use [updateEnvVar](#tag/environmentVariables/operation/updateEnvVar) to update a site's environment variables.

### submissions

Manage submissions

- **`netlify-pp-cli submissions delete`** - Delete
- **`netlify-pp-cli submissions list-form`** - List form

### user

Manage user

- **`netlify-pp-cli user`** - Get current


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
netlify-pp-cli accounts get mock-value

# JSON for scripting and agents
netlify-pp-cli accounts get mock-value --json

# Filter to specific fields
netlify-pp-cli accounts get mock-value --json --select id,name,status

# Dry run — show the request without sending
netlify-pp-cli accounts get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
netlify-pp-cli accounts get mock-value --agent
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

## Runtime Endpoint

This CLI resolves endpoint placeholders at runtime, so one installed binary can target different tenants or API versions without regeneration.

Endpoint environment variables:
- `NETLIFY_FORM_ID` resolves `{form_id}`

Base URL: `https://api.netlify.com/api/v1`

## Health Check

```bash
netlify-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `netlify-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/netlifys-documentation-pp-cli/config.toml`; `--home`, `NETLIFY_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `NETLIFY_FORM_ID` | endpoint | Yes |  |
| `NETLIFY_AUTH_TOKEN` | per_call | No | Set to your API credential. |
| `NETLIFY_NETLIFY_AUTH` | per_call | No | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `netlify-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `netlify-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $NETLIFY_AUTH_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Unauthorized on every call** — export NETLIFY_AUTH_TOKEN=<personal access token from app.netlify.com/user/applications>
- **overview or search returns empty** — run netlify-pp-cli sync first to populate the local mirror
