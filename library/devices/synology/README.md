# Synology CLI

**Every read surface of a Synology NAS - system, storage, SMART, shares, users, logs and File Station - as one agent-friendly CLI with a local mirror.**

DSM's WebAPI is namespaced RPC that answers HTTP 200 even when it fails, and its available namespaces differ per NAS depending on which packages are installed. This CLI handles the dialect for you: expired sessions renew themselves, DSM error codes become actionable messages, and 'session apis' tells you what your own NAS actually exposes. Everything is JSON-first, so '--agent' output pipes straight into jq or an agent's context.

## Install

The recommended path installs both the `synology-pp-cli` binary and the `pp-synology` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install synology
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install synology --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install synology --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install synology --agent claude-code
npx -y @mvanhorn/printing-press-library install synology --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/synology-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install synology --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-synology --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-synology --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install synology --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/synology-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "synology": {
      "command": "synology-pp-mcp"
    }
  }
}
```

</details>

## Authentication

DSM authenticates with an account and password, not an API key. 'session login --account <user> --passwd <password>' exchanges them for a session id and a SynoToken, both of which are persisted locally; the password never is. Two-factor accounts pass '--otp-code' on the first login and reuse the returned device id afterwards. Set SYNOLOGY_ACCOUNT and SYNOLOGY_PASSWORD so an expired session renews itself mid-run. Point the CLI at your NAS with SYNOLOGY_BASE_URL, and add '--insecure' if it still uses DSM's own self-signed certificate on port 5001.

Rather than exporting those into a shell profile, where every child process inherits the password, put them in `~/.claude/printing-press/synology-pp-cli/.env` (override the path with `SYNOLOGY_ENV_FILE`). Every printing-press CLI reads its own file under `~/.claude/printing-press/<cli-name>/`, so the credentials stay collected in one place without two CLIs ever sharing a file:

```
SYNOLOGY_BASE_URL=https://nas.example.lan:5001
SYNOLOGY_ACCOUNT=nas-user
SYNOLOGY_PASSWORD=your-password
SYNOLOGY_INSECURE_TLS=1
```

The CLI reads the file directly and never re-exports it. Precedence, weakest first: the credentials file, then this `.env`, then real environment variables. The password is never copied into the config file and is never written back to disk. Keep the file owner-only readable; the CLI warns when it is not, but leaves the permissions alone because the file is shared with the other printing-press CLIs.

## Quick Start

```bash
# Confirm the CLI is wired up before any credentials are involved.
synology-pp-cli doctor --dry-run

# This call needs no session, so it proves the NAS is reachable and shows what it offers.
synology-pp-cli session apis --agent

# Exchange credentials for a session id; everything below needs one.
synology-pp-cli session login --account admin --passwd '<password>'

# The single most useful read: overall system state in one call.
synology-pp-cli system health --agent

# Narrow a verbose payload down to the three fields that matter for a health check.
synology-pp-cli storage disks --agent --select id,status,temp

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Knowing what this NAS actually offers
- **`session apis`** — List every API namespace this particular NAS exposes, with its version range and CGI path.

  _Run this first when a call fails with DSM error 102 - it tells you whether the namespace exists on this NAS at all or whether the package is simply not installed._

  ```bash
  synology-pp-cli session apis --agent --select data
  ```
- **`storage smart`** — Read a drive's raw SMART attribute table; 'storage smart-schedule' reads the scheduled SMART test configuration.

  _Reach for this when asked whether a disk is failing - it is the only machine-readable path to per-attribute drive health on a Synology NAS._

  ```bash
  synology-pp-cli storage smart --device sata1 --agent
  ```

## Recipes

### Drive health at a glance

```bash
synology-pp-cli storage disks --agent --select disks.id,disks.status,disks.temp,disks.model
```

The disks payload is deeply nested and verbose; --select narrows it to the four fields a health check needs.

### Is this namespace even available

```bash
synology-pp-cli session apis --agent
```

Needs no session and answers the question DSM error 102 raises: does this NAS have that API at all.

### Recent errors from the system log

```bash
synology-pp-cli log --level err --limit 50 --agent
```

Filters DSM's syslog server-side by level, so only the rows worth reading come back.

### Browse a share without mounting it

```bash
synology-pp-cli files list --folder-path /volume1/media --agent
```

File Station browsing over the API, so no SMB mount and no credentials on the filesystem.

### Mirror once, query many times

```bash
synology-pp-cli sync && synology-pp-cli search 'volume1' --agent
```

Sync writes every synced resource into local SQLite; search then answers offline without touching the NAS again.

## Usage

Run `synology-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `SYNOLOGY_CONFIG_DIR`, `SYNOLOGY_DATA_DIR`, `SYNOLOGY_STATE_DIR`, or `SYNOLOGY_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `SYNOLOGY_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export SYNOLOGY_HOME=/srv/synology
synology-pp-cli doctor
```

Under `SYNOLOGY_HOME=/srv/synology`, the four dirs resolve to `/srv/synology/config`, `/srv/synology/data`, `/srv/synology/state`, and `/srv/synology/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "synology": {
      "command": "synology-pp-mcp",
      "env": {
        "SYNOLOGY_HOME": "/srv/synology"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `SYNOLOGY_DATA_DIR` overrides an explicit `--home` for that kind. Use `SYNOLOGY_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `SYNOLOGY_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `synology-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### files

File Station browsing, search and file operations

- **`synology-pp-cli files copy-start`** - Start a background copy or move and return its task id
- **`synology-pp-cli files copy-status`** - Report the progress of a running copy or move
- **`synology-pp-cli files copy-stop`** - Stop a running copy or move
- **`synology-pp-cli files delete-start`** - Start a background delete and return its task id
- **`synology-pp-cli files delete-status`** - Report the progress of a running delete
- **`synology-pp-cli files delete-stop`** - Stop a running delete
- **`synology-pp-cli files download`** - Download a file or folder
- **`synology-pp-cli files info`** - Show File Station capabilities and limits
- **`synology-pp-cli files list`** - List the contents of one folder
- **`synology-pp-cli files mkdir`** - Create a folder
- **`synology-pp-cli files rename`** - Rename a file or folder
- **`synology-pp-cli files search-results`** - Read the results of a running or finished search
- **`synology-pp-cli files search-start`** - Start a background search and return its task id
- **`synology-pp-cli files search-stop`** - Stop a running search and release its task
- **`synology-pp-cli files shares`** - List the shared folders File Station can browse
- **`synology-pp-cli files stat`** - Show metadata for one file or folder

### folder

Shared folders and their permissions

- **`synology-pp-cli folder get`** - Show one shared folder in detail
- **`synology-pp-cli folder list`** - List shared folders
- **`synology-pp-cli folder permissions`** - List the accounts and groups that hold permissions on one shared folder

### group

Local groups

- **`synology-pp-cli group list`** - List local groups
- **`synology-pp-cli group members`** - List the accounts that belong to one group

### log

DSM system log

- **`synology-pp-cli log`** - Read the DSM system log with filtering by level, date range and keyword

### nfs

NFS service state

- **`synology-pp-cli nfs`** - Show whether the NFS service is enabled and which protocol versions it offers

### package

Installed DSM packages

- **`synology-pp-cli package`** - List installed packages with their version and running state

### session

DSM session handshake

- **`synology-pp-cli session apis`** - Enumerate every API namespace the NAS exposes, with its version range and CGI path
- **`synology-pp-cli session login`** - Log in to DSM and obtain a session id plus a SynoToken
- **`synology-pp-cli session logout`** - Invalidate the current session id

### storage

Volumes, pools, disks and SMART data

- **`synology-pp-cli storage disks`** - List physical disks with model, serial number, temperature and health status
- **`synology-pp-cli storage esata`** - List attached eSATA storage devices
- **`synology-pp-cli storage luns`** - List iSCSI LUNs
- **`synology-pp-cli storage overview`** - Show the complete storage picture DSM Storage Manager renders, covering disks, pools and volumes in one call
- **`synology-pp-cli storage pools`** - List storage pools with their RAID level and member disks
- **`synology-pp-cli storage smart`** - Show the full SMART attribute table for one disk
- **`synology-pp-cli storage smart-schedule`** - List scheduled SMART tests
- **`synology-pp-cli storage usb`** - List attached USB storage devices
- **`synology-pp-cli storage volumes`** - List volumes with size, used space and status

### system

DSM system identity, health and load

- **`synology-pp-cli system dsm`** - Show DSM product identity as the web UI reports it
- **`synology-pp-cli system health`** - Show the aggregate system health verdict DSM shows on its dashboard
- **`synology-pp-cli system info`** - Show model, serial number, firmware version, uptime and temperature
- **`synology-pp-cli system network`** - Show hostname, DNS, gateway and every network interface
- **`synology-pp-cli system services`** - List DSM services and whether each one is enabled
- **`synology-pp-cli system ups`** - Show the state of an attached uninterruptible power supply
- **`synology-pp-cli system utilization`** - Show current CPU, memory, disk and network utilization

### user

Local user accounts

- **`synology-pp-cli user get`** - Show one local user account in detail
- **`synology-pp-cli user list`** - List local user accounts


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`synology-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`synology-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`synology-pp-cli learnings list`** - Inspect taught rows
- **`synology-pp-cli learnings forget <query>`** - Undo a teach
- **`synology-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`synology-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`synology-pp-cli teach-pattern`** - Install a query/resource template up front
- **`synology-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `SYNOLOGY_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `synology-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
synology-pp-cli files list --folder-path /volume1/media

# JSON for scripting and agents
synology-pp-cli files list --folder-path /volume1/media --json

# Filter to specific fields
synology-pp-cli files list --folder-path /volume1/media --json --select id,name,status

# Dry run — show the request without sending
synology-pp-cli files list --folder-path /volume1/media --dry-run

# Agent mode — JSON + compact + no prompts in one flag
synology-pp-cli files list --folder-path /volume1/media --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only outside File Station** - system, storage, SMART, shares, users, groups and logs are read surfaces only; the `files` group is the one exception and ships DSM's own File Station task operations (`mkdir`, `rename`, `copy-start`/`copy-status`/`copy-stop`, `delete-start`/`delete-status`/`delete-stop`)
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
synology-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `synology-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/synology-pp-cli/config.toml` on every platform (the home directory is `%USERPROFILE%` on Windows); `--home`, `SYNOLOGY_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Known Gaps

- **No account-to-groups lookup.** `SYNO.Core.User.Group` implements only `join` on DSM 7.1.1 and answers `list` with error 103, so there is no single call that maps an account to its groups. Combine `group list` with `group members` instead.
- **Docker and Download Station are out of scope.** Both are package APIs rather than DSM core, and neither is covered by this CLI.
- **`files download` returns binary payloads only.** The response is wrapped in a base64 envelope (`_pp_binary`, `content_type`, `data`). A text file comes back as its raw body, which the JSON guard rejects with an authentication error; download text files over the File Station web UI or a share mount instead.
- **`folder permissions` needs a principal set.** DSM answers error 403 when `user_group_type` is absent, so the flag defaults to `local_user`; pass `--user-group-type local_group` for groups.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `synology-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

### API-specific
- **DSM error 102: the requested API does not exist on this NAS** — Run 'synology-pp-cli session apis' - the package that provides that namespace is probably not installed.
- **DSM error 119 or 106 on every call** — The session id expired; run 'session login' again, or set SYNOLOGY_ACCOUNT and SYNOLOGY_PASSWORD so it renews itself.
- **DSM error 105 on system or storage commands** — Most SYNO.Core.* and SYNO.Storage.* calls need an administrator account; log in as one.
- **x509: certificate signed by unknown authority on port 5001** — Add '--insecure', or set SYNOLOGY_INSECURE_TLS=1, if the NAS still uses DSM's self-signed certificate.
- **DSM error 403 on login** — The account has two-factor enabled; pass '--otp-code <six digits>' on the first login and reuse the returned device id afterwards.
