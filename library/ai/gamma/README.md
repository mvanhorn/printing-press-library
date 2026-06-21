# Gamma CLI

**Generate polished presentations, documents, and webpages from a text prompt — with one-command async polling and auto-download.**

Gamma generates AI-powered presentations, documents, and webpages from a text prompt. The CLI adds async-wait-and-download as a single command, sharing preset shorthands, and image-source aliases — all using --watch --download --sharing --image-source.

## Install

The recommended path installs both the `gamma-pp-cli` binary and the `pp-gamma` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install gamma
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install gamma --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install gamma --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install gamma --agent claude-code
npx -y @mvanhorn/printing-press-library install gamma --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/gamma-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install gamma --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-gamma --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-gamma --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install gamma --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/gamma-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `GAMMA_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "gamma": {
      "command": "gamma-pp-mcp",
      "env": {
        "GAMMA_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Set GAMMA_API_KEY from gamma.app/settings. Run 'gamma-pp-cli auth set-token <key>' to persist it.

## Quick Start

```bash
# Verify the CLI is installed and auth key is configured
gamma-pp-cli doctor --dry-run

# Generate a presentation and wait for completion
gamma-pp-cli generations create --input-text "Q3 2025 product roadmap: new features, timeline, owners" --format presentation --watch --agent

# Generate a document and download the export
gamma-pp-cli generations create --input-text "AI market overview for investors" --format document --watch --download /tmp/gamma-exports/ --agent

# List available themes to use in generations
gamma-pp-cli themes --agent

# Generate with sharing preset and AI images
gamma-pp-cli generations create --input-text "Startup pitch deck" --format presentation --sharing view-link --image-source aiGenerated --watch --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Generation workflow
- **`generations create --watch`** — Poll until generation completes, then print gammaUrl and credits. Bridges the async gap without manual polling.

  _Use whenever you start a generation and need the final gammaUrl in one command, without a separate polling loop._

  ```bash
  gamma-pp-cli generations create --input-text "Q3 product roadmap" --format presentation --watch --agent
  ```
- **`generations create --download`** — After --watch completes, download the exportUrl to a local directory with the gammaId as filename.

  _Use when you need the exported file locally for further processing without a separate download step._

  ```bash
  gamma-pp-cli generations create --input-text "AI market overview" --format document --watch --download /tmp/gamma-exports/ --agent
  ```
- **`generations create --sharing`** — Map a preset name (private, view-link, edit-link, workspace-view) to the workspaceAccess + externalAccess flag pair.

  _Use instead of specifying two separate access flags when the sharing intent is a standard preset._

  ```bash
  gamma-pp-cli generations create --input-text "Q4 investor update" --format presentation --sharing view-link --watch --agent
  ```
- **`generations create --image-source`** — Short alias for --image-options-source that sets the AI model for image generation without the nested flag path.

  _Use to specify image generation method without remembering the nested flag path._

  ```bash
  gamma-pp-cli generations create --input-text "Product launch slides" --format presentation --image-source aiGenerated --watch --agent
  ```

## Usage

Run `gamma-pp-cli --help` for the full command reference and flag list.

## Commands

### folders

Manage folders

- **`gamma-pp-cli folders`** - List folders the authenticated user can access. Use returned id values in folderIds in generation requests. Cursor-paginated with hasMore and nextCursor fields.

### gammas

Manage gammas

- **`gamma-pp-cli gammas <gammaId>`** - Delete a Gamma permanently. Requires workspace admin role. Auto-archives first if needed. Gamma becomes immediately inaccessible. gammaId must be the API file ID (g_... prefix), not the URL slug.

### generations

Create and poll AI generations

- **`gamma-pp-cli generations create`** - Start an asynchronous generation from text. Returns generationId immediately. Poll GET /v1.0/generations/{id} every 5 seconds until status is completed or failed. Generation typically takes 1-3 minutes. Credits charged on completion.
- **`gamma-pp-cli generations create-from-template`** - Adapt an existing Gamma by providing the template file ID and instructions. Template structure preserved by default. gammaId must be the API file ID (g_... prefix), not the URL slug.
- **`gamma-pp-cli generations get-status`** - Poll this endpoint until status is completed or failed. On completion returns gammaId (g_... prefix), gammaUrl, exportUrl (if exportAs was set, expires approx 1 week), and credits. PNG exports are a .zip with one PNG per card.

### themes

Manage themes

- **`gamma-pp-cli themes`** - List themes available in the authenticated workspace. Use returned id as themeId in generation requests. Cursor-paginated with hasMore and nextCursor fields.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
gamma-pp-cli folders

# JSON for scripting and agents
gamma-pp-cli folders --json

# Filter to specific fields
gamma-pp-cli folders --json --select id,name,status

# Dry run — show the request without sending
gamma-pp-cli folders --dry-run

# Agent mode — JSON + compact + no prompts in one flag
gamma-pp-cli folders --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
gamma-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/gamma-public-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `GAMMA_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `gamma-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `gamma-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $GAMMA_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

### API-specific
- **Error: GAMMA_API_KEY not set** — export GAMMA_API_KEY=<your-key> or run: gamma-pp-cli auth set-token <key>
- **--watch times out** — Increase --wait-timeout (default 10m). Long documents can take several minutes.
- **--download fails with no file** — Use --watch with --download — download requires a completed generation with an exportUrl.
