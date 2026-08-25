# Claude Agent SDK Python Docs CLI

**A source-grounded CLI for exploring and verifying the Claude Agent SDK Python docs.**

Fetch the raw Claude Code docs index, search the Python Agent SDK reference, and extract exact symbols, examples, and citations. Novel verification and context commands help agents use the SDK without guessing.

## Install

The recommended path installs both the `claude-agent-sdk-python-docs-pp-cli` binary and the `pp-claude-agent-sdk-python-docs` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install claude-agent-sdk-python-docs
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install claude-agent-sdk-python-docs --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install claude-agent-sdk-python-docs --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install claude-agent-sdk-python-docs --agent claude-code
npx -y @mvanhorn/printing-press-library install claude-agent-sdk-python-docs --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available, install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/claude-agent-sdk-python-docs/cmd/claude-agent-sdk-python-docs-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/claude-agent-sdk-python-docs-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install claude-agent-sdk-python-docs --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-claude-agent-sdk-python-docs --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-claude-agent-sdk-python-docs --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install claude-agent-sdk-python-docs --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/claude-agent-sdk-python-docs-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/claude-agent-sdk-python-docs/cmd/claude-agent-sdk-python-docs-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "claude-agent-sdk-python-docs": {
      "command": "claude-agent-sdk-python-docs-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Verify the CLI and local configuration without network or docs writes.
claude-agent-sdk-python-docs-pp-cli doctor --dry-run

# Fetch the docs index and Python Agent SDK page through the generated docs endpoints.
claude-agent-sdk-python-docs-pp-cli sync --resources pages --full

# Find the exact docs page and section for an SDK option or type.
claude-agent-sdk-python-docs-pp-cli search "ClaudeAgentOptions" --type pages --agent --select items.title,items.url

# Read a symbol with its anchor, examples, and citations.
claude-agent-sdk-python-docs-pp-cli symbol ClaudeSDKClient --agent

# Build a compact task bundle for an agent implementing SDK custom tools.
claude-agent-sdk-python-docs-pp-cli context "custom tools" --agent --select sections.title,examples.code,citations.url

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Docs-grounded verification
- **`verify`** — Check Python code imports and qualified names against documented Claude Agent SDK symbols.

  _Use this before trusting generated SDK code or reviewing a PR that imports claude_agent_sdk._

  ```bash
  claude-agent-sdk-python-docs-pp-cli verify ./src --agent
  ```
- **`diff`** — Compare current docs entity hashes against an optional baseline file.

  _Use this to detect whether SDK docs facts have changed against a saved baseline._

  ```bash
  claude-agent-sdk-python-docs-pp-cli diff --since latest --agent
  ```
- **`audit-links`** — Validate internal anchors, guide links, and section references in the fetched docs corpus.

  _Use this before relying on context bundles or citations in generated code reviews._

  ```bash
  claude-agent-sdk-python-docs-pp-cli audit-links --agent
  ```

### Agent-native context
- **`context`** — Build a compact source-cited docs bundle for one SDK implementation task.

  _Use this when an agent needs exact docs context without loading full Markdown pages._

  ```bash
  claude-agent-sdk-python-docs-pp-cli context "custom tools" --agent --select sections.title,examples.code,citations.url
  ```
- **`recipe`** — Compose a deterministic implementation scaffold from documented snippets and exact signatures.

  _Use this when you need a copyable starting point constrained to documented SDK patterns._

  ```bash
  claude-agent-sdk-python-docs-pp-cli recipe "streaming ClaudeSDKClient" --agent
  ```

### SDK surface intelligence
- **`map`** — Map functions, classes, types, options, tools, hooks, and message blocks by entity type.

  _Use this to discover the available SDK surface before choosing an implementation path._

  ```bash
  claude-agent-sdk-python-docs-pp-cli map --kind classes,types,options --agent
  ```
- **`coverage examples`** — Report which documented symbols have extracted examples and which do not.

  _Use this to find example-backed SDK APIs and documentation coverage gaps._

  ```bash
  claude-agent-sdk-python-docs-pp-cli coverage examples --agent
  ```

## Recipes


### Find the right option fields for a client setup

```bash
claude-agent-sdk-python-docs-pp-cli search "ClaudeAgentOptions" --type pages --agent --select items.title,items.url,items.snippet
```

Returns only the high-signal fields an agent needs instead of the full reference page.

### Build a custom-tools implementation bundle

```bash
claude-agent-sdk-python-docs-pp-cli context "custom tools" --agent --select sections.title,examples.code,citations.url
```

Pairs `--agent` with `--select` so downstream agents get just examples, section titles, and citations.

### Verify a Python project against current docs

```bash
claude-agent-sdk-python-docs-pp-cli verify ./src --agent
```

Flags undocumented Claude Agent SDK Python identifiers with source citations.

### Inspect documented SDK surface area

```bash
claude-agent-sdk-python-docs-pp-cli map --kind classes,types,options --agent
```

Shows the SDK inventory by entity type before writing code.

## Usage

Run `claude-agent-sdk-python-docs-pp-cli --help` for the full command reference and flag list.

## Commands

### pages

Fetch Claude Agent SDK documentation pages

- **`claude-agent-sdk-python-docs-pp-cli pages custom-tools`** - Fetch the Agent SDK custom tools guide
- **`claude-agent-sdk-python-docs-pp-cli pages index`** - Fetch the Claude Code documentation index
- **`claude-agent-sdk-python-docs-pp-cli pages mcp`** - Fetch the Agent SDK MCP guide
- **`claude-agent-sdk-python-docs-pp-cli pages overview`** - Fetch the Agent SDK overview
- **`claude-agent-sdk-python-docs-pp-cli pages permissions`** - Fetch the Agent SDK permissions guide
- **`claude-agent-sdk-python-docs-pp-cli pages python`** - Fetch the Python Agent SDK reference
- **`claude-agent-sdk-python-docs-pp-cli pages quickstart`** - Fetch the Agent SDK quickstart
- **`claude-agent-sdk-python-docs-pp-cli pages sessions`** - Fetch the Agent SDK sessions guide
- **`claude-agent-sdk-python-docs-pp-cli pages structured-outputs`** - Fetch the Agent SDK structured output guide


## Output Formats

```bash
# Raw Markdown response (binary response; use read/search/context for structured output)
claude-agent-sdk-python-docs-pp-cli pages custom-tools

# Dry run — show the request without sending
claude-agent-sdk-python-docs-pp-cli pages custom-tools --dry-run

# Structured agent output
claude-agent-sdk-python-docs-pp-cli context "custom tools" --agent --select sections.title,examples.code,citations.url
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Agent-friendly** - docs intelligence commands return structured JSON with `--agent` and `--select`
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Freshness

The generated `pages` endpoints and hand-authored docs intelligence commands read the public Claude Code docs over HTTPS. The docs intelligence commands intentionally reject `--data-source local`; use `--data-source auto` or `--data-source live` for those commands.

For structured retrieval, prefer `read`, `search`, `symbol`, `examples`, `guide`, or `context` over raw `pages` binary endpoints.

## Health Check

```bash
claude-agent-sdk-python-docs-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/claude-agent-sdk-python-docs-pp-cli/config.json`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the page key or symbol spelling.
- Run `claude-agent-sdk-python-docs-pp-cli map --agent` to see documented symbols.

### API-specific
- **Search returns no results.** — Try a more specific SDK symbol, such as `ClaudeSDKClient` or `ClaudeAgentOptions`.
- **A symbol seems missing.** — Run `claude-agent-sdk-python-docs-pp-cli map --kind classes,types,options --agent` to inspect the parsed docs surface.
- **Generated code uses an undocumented SDK name.** — Run `claude-agent-sdk-python-docs-pp-cli verify ./path --agent` and follow the cited replacement or docs link.
