# Namecheap CLI

Curated OpenAPI description for Namecheap's XML API. The real API uses a single
endpoint (`/xml.response`) with a `Command` query parameter plus Namecheap's
query-string authentication parameters (`ApiUser`, `ApiKey`, `UserName`, `ClientIp`).
Generation uses command-shaped pseudo paths that are normalized back to `/xml.response`
by the Namecheap printed CLI patch layer.

Created by [@cathryn-lavery](https://github.com/cathryn-lavery) (Cathryn Lavery).

## Install

The recommended path installs both the `namecheap-pp-cli` binary and the `pp-namecheap` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install namecheap
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install namecheap --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install namecheap --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install namecheap --agent claude-code
npx -y @mvanhorn/printing-press-library install namecheap --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/namecheap/cmd/namecheap-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/namecheap-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-namecheap --force
```

Inside a Hermes chat session:

```text
/skills install mvanhorn/printing-press-library/cli-skills/pp-namecheap --force
```

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install namecheap --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/namecheap-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `NAMECHEAP_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "namecheap": {
      "command": "namecheap-pp-mcp",
      "env": {
        "NAMECHEAP_API_KEY": "<your-key>"
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
export NAMECHEAP_API_KEY="<paste-your-key>"
```

You can also persist this in your config file at `~/.config/namecheap-pp-cli/config.toml`.

### 3. Verify Setup

```bash
namecheap-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
namecheap-pp-cli dns get-email-forwarding
```

## Usage

Run `namecheap-pp-cli --help` for the full command reference and flag list.

## Commands

### dns

Manage dns

- **`namecheap-pp-cli dns get-email-forwarding`** - Runs `namecheap.domains.dns.getEmailForwarding`.
- **`namecheap-pp-cli dns get-hosts`** - Runs `namecheap.domains.dns.getHosts`.
- **`namecheap-pp-cli dns get-list`** - Get DNS nameserver type and nameservers.
- **`namecheap-pp-cli dns set-custom`** - Runs `namecheap.domains.dns.setCustom`.
- **`namecheap-pp-cli dns set-default`** - Switch a domain to Namecheap default DNS.
- **`namecheap-pp-cli dns set-hosts`** - Runs `namecheap.domains.dns.setHosts`; HostName1/RecordType1/Address1/TTL1 style parameters can be passed via --param-json in the patched CLI.

### domains

Manage domains

- **`namecheap-pp-cli domains check`** - Check domain availability for one or more domains.
- **`namecheap-pp-cli domains create`** - Runs `namecheap.domains.create`. This is a mutating paid operation; use dry-run unless intentionally registering.
- **`namecheap-pp-cli domains get-contacts`** - Runs `namecheap.domains.getContacts`.
- **`namecheap-pp-cli domains get-info`** - Runs `namecheap.domains.getInfo` for a domain.
- **`namecheap-pp-cli domains get-list`** - Runs `namecheap.domains.getList` with paging and optional filters.
- **`namecheap-pp-cli domains get-registrar-lock`** - Runs `namecheap.domains.getRegistrarLock`.
- **`namecheap-pp-cli domains get-tld-list`** - Runs `namecheap.domains.getTldList`.
- **`namecheap-pp-cli domains renew`** - Runs `namecheap.domains.renew`. Mutating paid operation.
- **`namecheap-pp-cli domains set-registrar-lock`** - Runs `namecheap.domains.setRegistrarLock`.

### ssl

Manage ssl

- **`namecheap-pp-cli ssl get-info`** - Get SSL certificate information.
- **`namecheap-pp-cli ssl get-list`** - Runs `namecheap.ssl.getList`.
- **`namecheap-pp-cli ssl parse-csr`** - Parse a certificate signing request.

### users

Manage users

- **`namecheap-pp-cli users address-get-info`** - Runs `namecheap.users.address.getInfo`.
- **`namecheap-pp-cli users address-get-list`** - Runs `namecheap.users.address.getList`.
- **`namecheap-pp-cli users get-balances`** - Runs `namecheap.users.getBalances`.
- **`namecheap-pp-cli users get-pricing`** - Runs `namecheap.users.getPricing`.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
namecheap-pp-cli dns get-email-forwarding

# JSON for scripting and agents
namecheap-pp-cli dns get-email-forwarding --json

# Filter to specific fields
namecheap-pp-cli dns get-email-forwarding --json --select id,name,status

# Dry run — show the request without sending
namecheap-pp-cli dns get-email-forwarding --dry-run

# Agent mode — JSON + compact + no prompts in one flag
namecheap-pp-cli dns get-email-forwarding --agent
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
namecheap-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/namecheap-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `NAMECHEAP_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `namecheap-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $NAMECHEAP_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
