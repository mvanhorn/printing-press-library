# Zenodo CLI

This is for the zenodo API http://developers.zenodo.org/#rest-api

Created by [@Scan-law4](https://github.com/Scan-law4).

## Install

The recommended path installs both the `zenodo-pp-cli` binary and the `pp-zenodo` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install zenodo
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install zenodo --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install zenodo --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install zenodo --agent claude-code
npx -y @mvanhorn/printing-press-library install zenodo --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/zenodo/cmd/zenodo-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/zenodo-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install zenodo --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-zenodo --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-zenodo --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install zenodo --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/zenodo-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `ZENODO_ACCESS_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/zenodo/cmd/zenodo-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "zenodo": {
      "command": "zenodo-pp-mcp",
      "env": {
        "ZENODO_ACCESS_TOKEN": "<your-key>"
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
export ZENODO_ACCESS_TOKEN="<paste-your-key>"
```

To persist credentials, use `zenodo-pp-cli auth set-token <token>`. Stored secrets live in `credentials.toml` under the data directory, not in `config.toml`.

### 3. Verify Setup

```bash
zenodo-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
zenodo-pp-cli communities
```

## Usage

Run `zenodo-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `ZENODO_CONFIG_DIR`, `ZENODO_DATA_DIR`, `ZENODO_STATE_DIR`, or `ZENODO_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `ZENODO_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export ZENODO_HOME=/srv/zenodo
zenodo-pp-cli doctor
```

Under `ZENODO_HOME=/srv/zenodo`, the four dirs resolve to `/srv/zenodo/config`, `/srv/zenodo/data`, `/srv/zenodo/state`, and `/srv/zenodo/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "zenodo": {
      "command": "zenodo-pp-mcp",
      "env": {
        "ZENODO_HOME": "/srv/zenodo"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `ZENODO_DATA_DIR` overrides an explicit `--home` for that kind. Use `ZENODO_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `ZENODO_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `zenodo-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### communities

Manage communities

- **`zenodo-pp-cli communities`** - List of communities

### deposit

Manage deposit

- **`zenodo-pp-cli deposit create`** - Create a new deposit
- **`zenodo-pp-cli deposit create-file`** - Create a new file
- **`zenodo-pp-cli deposit delete`** - Delete a single deposit
- **`zenodo-pp-cli deposit delete-file`** - Delete an existing deposition file resource. Note, only deposition files for unpublished depositions may be deleted.
- **`zenodo-pp-cli deposit discard`** - Discard changes in the current editing session.
- **`zenodo-pp-cli deposit edit`** - Unlock already submitted deposition for editing.
- **`zenodo-pp-cli deposit get`** - Get a single deposit
- **`zenodo-pp-cli deposit get-file`** - Retrieve a single deposition file.
- **`zenodo-pp-cli deposit list`** - List of deposits
- **`zenodo-pp-cli deposit list-files-for`** - List of files for deposit
- **`zenodo-pp-cli deposit new-version`** - Create a new version of a deposition.
- **`zenodo-pp-cli deposit publish`** - Publish a deposition. Note, once a deposition is published, you can no longer delete it.
- **`zenodo-pp-cli deposit put`** - Update metadata for a single deposit
- **`zenodo-pp-cli deposit sort-files`** - Sort the files for a deposit
- **`zenodo-pp-cli deposit update-file`** - Update a deposition file resource. Currently the only use is renaming an already uploaded file. If you want to replace the actual file, please delete the file and upload a new file.

### funders

Manage funders

- **`zenodo-pp-cli funders`** - List of funders

### grants

Manage grants

- **`zenodo-pp-cli grants`** - List of grants

### licenses

Manage licenses

- **`zenodo-pp-cli licenses`** - List of licenses

### records

Manage records

- **`zenodo-pp-cli records`** - List of records


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`zenodo-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`zenodo-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`zenodo-pp-cli learnings list`** - Inspect taught rows
- **`zenodo-pp-cli learnings forget <query>`** - Undo a teach
- **`zenodo-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`zenodo-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`zenodo-pp-cli teach-pattern`** - Install a query/resource template up front
- **`zenodo-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `ZENODO_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `zenodo-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
zenodo-pp-cli communities

# JSON for scripting and agents
zenodo-pp-cli communities --json

# Filter to specific fields
zenodo-pp-cli communities --json --select id,name,status

# Dry run — show the request without sending
zenodo-pp-cli communities --dry-run

# Agent mode — JSON + compact + no prompts in one flag
zenodo-pp-cli communities --agent
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
zenodo-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `zenodo-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/zenodo-pp-cli/config.toml`; `--home`, `ZENODO_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `ZENODO_ACCESS_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `zenodo-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `zenodo-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $ZENODO_ACCESS_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
