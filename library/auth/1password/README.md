# 1Password CLI

Agent-safe command layer over the official 1Password CLI and SDK service-account workflows.

Use 1Password from agents without turning every task into a secret reveal. The CLI resolves fuzzy requests to exact op:// references, audits metadata, checks policy, and only calls op read, op inject, or op run after an explicit plan.

## Install

The recommended path installs both the `1password-pp-cli` binary and the `pp-1password` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install 1password
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install 1password --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install 1password --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install 1password --agent claude-code
npx -y @mvanhorn/printing-press-library install 1password --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/auth/1password/cmd/1password-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/1password-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install 1password --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-1password --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-1password --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install 1password --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/1password-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/auth/1password/cmd/1password-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "1password": {
      "command": "1password-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Verify the official op CLI is installed and authenticated without reading any secret values.
1password-pp-cli op --json

# Summarize accessible vault and category metadata before planning a task.
1password-pp-cli access scope --json

# Check whether a task needs secret values, documents, cards, or write permissions.
1password-pp-cli secrets preflight --task "deploy using op run --env-file .env" --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Secrets
- **`secrets resolve`** — Resolve fuzzy agent requests to exact op:// vault/item/field references without printing values.

  _Prevents accidental broad reads and gives later commands an exact reference to use._

  ```bash
  1password-pp-cli secrets resolve --query "token" --json
  ```
- **`secrets read`** — Read only an exact op:// reference, with policy checks and an explicit --reveal gate before values are printed.

  _Turns value access into a narrow, auditable action instead of a fuzzy search._

  ```bash
  1password-pp-cli secrets read op://Engineering/GitHub/token --dry-run --json
  ```
- **`secrets explain`** — Explain why a particular item and field was selected for a task without revealing the field value.

  ```bash
  1password-pp-cli secrets explain --query "token" --json
  ```
- **`secrets preflight`** — Check whether a planned task or command appears to require secret values, documents, cards, or write permissions.

  ```bash
  1password-pp-cli secrets preflight --task "deploy using op run --env-file .env" --json
  ```

### Environment
- **`env plan`** — Parse .env, config, or shell files for missing variables and map them to safe op:// references.

  ```bash
  1password-pp-cli env plan API_TOKEN= --json
  ```
- **`env inject`** — Wrap op inject with a redacted plan first and require --write before producing an output file.

  ```bash
  1password-pp-cli env inject --in-file README.md --out-file injected.env --json
  ```

### Items
- **`items classify`** — Find secure notes that look like API credentials, logins, SSH keys, cards, or documents.

  ```bash
  1password-pp-cli items classify --json
  ```
- **`items duplicates`** — Detect duplicate titles, URLs, usernames, and likely copied credentials across vaults without printing secret values.

  ```bash
  1password-pp-cli items duplicates --json
  ```
- **`items ownership`** — Flag shared or service credentials missing owner, purpose, rotation, or environment tags.

  ```bash
  1password-pp-cli items ownership --json
  ```

### Cards
- **`cards audit`** — Find cards stored as notes or logins, missing owner/purpose tags, or CVV-like fields without printing card values.

  ```bash
  1password-pp-cli cards audit --json
  ```
- **`cards resolve`** — Return card item and field references without printing card numbers, expiry values, or CVVs.

  ```bash
  1password-pp-cli cards resolve --query "card" --json
  ```

### Documents
- **`documents inventory`** — List document metadata and exact references without downloading document contents.

  ```bash
  1password-pp-cli documents inventory --json
  ```
- **`documents audit`** — Flag sensitive filenames, oversized docs, private-key/cert-like documents, and documents in shared vaults.

  ```bash
  1password-pp-cli documents audit --json
  ```

### Sharing
- **`share preflight`** — Before sharing an item, show recipient, item category, included fields, expiry, and risk.

  ```bash
  1password-pp-cli share preflight --ref op://Engineering/GitHub/token --recipient recipient --expires-in 1d --json
  ```
- **`share audit`** — Report whether existing/shareable item link inspection is supported by op or the SDK and document unsupported status clearly.

  ```bash
  1password-pp-cli share audit --json
  ```

### Policy
- **`policy check`** — Enforce rules such as never reading credit-card values, exact refs for production, and required owner tags.

  ```bash
  1password-pp-cli policy check --ref op://Production/API/token --require-exact --json
  ```

### Access
- **`access scope`** — Summarize what the current service account or op session can access by vault, category, and count without values.

  ```bash
  1password-pp-cli access scope --json
  ```
- **`rate-limit status`** — Wrap op service-account ratelimit so agents can avoid burning quota.

  ```bash
  1password-pp-cli rate-limit status --json
  ```

### Agent
- **`agent grant-plan`** — Suggest the minimum service-account vault permissions needed for a task.

  ```bash
  1password-pp-cli agent grant-plan --task "read staging deploy token" --json
  ```

### Runtime
- **`run plan`** — Inspect an op run command or env files and show which secret references will resolve before executing.

  ```bash
  1password-pp-cli run plan --command "npm test" --json
  ```

### Audit
- **`audit stale`** — Flag items that appear old, untagged, duplicated, or probably unused from metadata.

  ```bash
  1password-pp-cli audit stale --days 180 --json
  ```
- **`audit misplaced`** — Find API keys, cards, documents, or SSH material saved in the wrong 1Password category.

  ```bash
  1password-pp-cli audit misplaced --json
  ```

## Usage

Run `1password-pp-cli --help` for the full command reference and flag list.

## Commands

### op

Inspect the local 1Password CLI authentication surface

- **`1password-pp-cli op`** - Show whether op is installed and authenticated without reading secret values


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
1password-pp-cli op

# JSON for scripting and agents
1password-pp-cli op --json

# Filter to specific fields
1password-pp-cli op --json --select id,name,status

# Dry run — show the request without sending
1password-pp-cli op --dry-run

# Agent mode — JSON + compact + no prompts in one flag
1password-pp-cli op --agent
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
1password-pp-cli doctor
```

Verifies configuration and access to the official `op` CLI.

## Auth Setup

Install the official 1Password CLI (`op`) and authenticate it before using this wrapper.

For automation, set a 1Password service-account token:

```bash
export OP_SERVICE_ACCOUNT_TOKEN="<service-account-token>"
```

For local desktop workflows, sign in with `op` using the normal 1Password CLI or desktop-app integration. This CLI warns when `OP_CONNECT_HOST` or `OP_CONNECT_TOKEN` is set because those Connect variables take precedence over service-account auth in `op`.

## Configuration

Config file: `~/.config/1password-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
