# Crates Io CLI

Read-only endpoints for crates.io ecosystem intelligence.

Created by [@sdhilip200](https://github.com/sdhilip200) (Dhilip Subramanian).

## Install

The recommended path installs both the `crates-io-pp-cli` binary and the `pp-crates-io` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press-library install crates-io
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install crates-io --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install crates-io --skill-only
```

To constrain the skill install to one or more specific agents, pass `--agent <name>` using the installer-supported agent name.

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/crates-io/cmd/crates-io-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/crates-io-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install crates-io --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-crates-io --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-crates-io --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install crates-io --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use as an MCP bundle

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle for one-click MCP extension installs where supported.

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/crates-io-current).
2. Double-click the `.mcpb` file and follow your MCP host's install flow.

Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle, install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/crates-io/cmd/crates-io-pp-mcp@latest
```

Add to your MCP host config:

```json
{
  "mcpServers": {
    "crates-io": {
      "command": "crates-io-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Verify Setup

```bash
crates-io-pp-cli doctor
```

This checks your configuration.

### 3. Try Your First Command

```bash
crates-io-pp-cli crates get mock-value
```

## Usage

Run `crates-io-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `CRATES_IO_CONFIG_DIR`, `CRATES_IO_DATA_DIR`, `CRATES_IO_STATE_DIR`, or `CRATES_IO_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `CRATES_IO_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export CRATES_IO_HOME=/srv/crates-io
crates-io-pp-cli doctor
```

Under `CRATES_IO_HOME=/srv/crates-io`, the four dirs resolve to `/srv/crates-io/config`, `/srv/crates-io/data`, `/srv/crates-io/state`, and `/srv/crates-io/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "crates-io": {
      "command": "crates-io-pp-mcp",
      "env": {
        "CRATES_IO_HOME": "/srv/crates-io"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `CRATES_IO_DATA_DIR` overrides an explicit `--home` for that kind. Use `CRATES_IO_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `CRATES_IO_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `crates-io-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### crates

Manage crates

- **`crates-io-pp-cli crates get`** - Get crate metadata
- **`crates-io-pp-cli crates search`** - Search crates


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
crates-io-pp-cli crates get mock-value

# JSON for scripting and agents
crates-io-pp-cli crates get mock-value --json

# Filter to specific fields
crates-io-pp-cli crates get mock-value --json --select id,name,status

# Dry run — show the request without sending
crates-io-pp-cli crates get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
crates-io-pp-cli crates get mock-value --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
crates-io-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `crates-io-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/crates-io-registry-pp-cli/config.toml`; `--home`, `CRATES_IO_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
