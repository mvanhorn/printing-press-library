# Pop CLI

POP API for European electronic invoicing workflows.

This Printing Press blueprint intentionally exposes only the documented
8-operation surface used by the POP MCP README at
`getpopapi/pop-mcp@07a4a19ad5e657fa94d16fe617b8288e5046c489`:

  - create-xml
  - create-ubl
  - create-pdf
  - sdi-via-pop/document-notifications
  - peppol/document-get
  - sdi-via-pop/document-get
  - sdi-via-pop/document-verify
  - sdi-via-pop/document-preserve

The `data` payloads are intentionally modeled as free-form JSON objects so
the CLI can pass through the full POP invoice structure without freezing a
brittle nested schema in this catalog spec.

Upstream POP MCP changes should be reviewed periodically and intentionally
synced into this print when the documented public surface evolves.

Learn more at [Pop](https://popapi.io/en/).

Created by [@mircobabini](https://github.com/mircobabini) (Mirco Babini).

## Install

The recommended path installs both the `pop-pp-cli` binary and the `pp-pop` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install pop
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install pop --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install pop --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install pop --agent claude-code
npx -y @mvanhorn/printing-press-library install pop --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/pop/cmd/pop-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/pop-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install pop --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-pop --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-pop --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install pop --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/pop-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `POP_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/payments/pop/cmd/pop-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "pop": {
      "command": "pop-pp-mcp",
      "env": {
        "POP_API_KEY": "<your-key>"
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
export POP_API_KEY="<paste-your-key>"
```

You can also persist this in your config file at `~/.config/pop-pp-cli/config.toml`.

### 3. Verify Setup

```bash
pop-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
pop-pp-cli create-pdf
```

## Usage

Run `pop-pp-cli --help` for the full command reference and flag list.

## Commands

### create-pdf

Create branded PDF invoices and optionally email them.

- **`pop-pp-cli create-pdf`** - Generate a branded PDF invoice. Depending on account setup and payload,
POP may return a direct file, a URL, or a JSON response.

### create-ubl

Create PEPPOL UBL invoices and optionally submit them to the network.

- **`pop-pp-cli create-ubl`** - Generate a PEPPOL BIS / UBL invoice and optionally submit it through
POP's PEPPOL integration.

### create-xml

Create Italian FatturaPA XML invoices and optionally submit them to SdI.

- **`pop-pp-cli create-xml`** - Generate an Italian FatturaPA XML document and optionally submit it to
SdI. The `data` object must follow POP's invoice structure.

### peppol

Manage PEPPOL document workflows

- **`pop-pp-cli peppol get-document`** - Retrieve a PEPPOL document from POP by UUID and optional zone.

### sdi

Manage SdI document workflows

- **`pop-pp-cli sdi get-invoice-status`** - Read POP's recorded SdI notifications for a submitted invoice UUID.
- **`pop-pp-cli sdi get-sdi-document`** - Retrieve a stored SdI document by UUID.
- **`pop-pp-cli sdi preserve-sdi-document`** - Archive an accepted SdI document in POP's long-term storage.
- **`pop-pp-cli sdi verify-sdi-document`** - Validate a Base64-encoded SdI XML document before submission.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
pop-pp-cli create-pdf

# JSON for scripting and agents
pop-pp-cli create-pdf --json

# Filter to specific fields
pop-pp-cli create-pdf --json --select id,name,status

# Dry run — show the request without sending
pop-pp-cli create-pdf --dry-run

# Agent mode — JSON + compact + no prompts in one flag
pop-pp-cli create-pdf --agent
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
pop-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/pop-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `POP_API_KEY` | per_call | No | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `pop-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $POP_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
