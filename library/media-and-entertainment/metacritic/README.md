# Metacritic CLI

Undocumented internal JSON API powering metacritic.com (a Fandom property).
Reverse-engineered from the site's public web client. Every request carries a
shared public `apiKey` that the site embeds in its own JavaScript bundle, so no
user credentials are required for read access.

One backend (`backend.metacritic.com`) serves every media type. The `mcoTypeId`
query parameter selects the medium on the browse endpoint: 1 = TV shows,
2 = movies, 13 = games. The `{mediaType}` path segment (`games`, `movies`,
`shows`) selects the medium on the title-detail, filters, and review endpoints.

The non-obvious insight: Metacritic is not just a review aggregator, it is a
cross-media taste graph. Every gap between a title's Metascore (critics) and
user score is a signal about critic-audience divergence.

Scope: this spec covers games, movies, and TV shows plus search, filters, title
detail, and critic/user reviews. Music (albums) is served through a separate
query shape and is intentionally left as a follow-up.

Learn more at [Metacritic](https://www.metacritic.com).

Created by [@coopdogGGs](https://github.com/coopdogGGs) (Ryan Cooper).

## Install

The recommended path installs both the `metacritic-pp-cli` binary and the `pp-metacritic` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install metacritic
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install metacritic --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install metacritic --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install metacritic --agent claude-code
npx -y @mvanhorn/printing-press-library install metacritic --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/metacritic/cmd/metacritic-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/metacritic-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install metacritic --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-metacritic --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-metacritic --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install metacritic --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/metacritic-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `METACRITIC_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/metacritic/cmd/metacritic-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "metacritic": {
      "command": "metacritic-pp-mcp",
      "env": {
        "METACRITIC_API_KEY": "<your-key>"
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
export METACRITIC_API_KEY="<paste-your-key>"
```

You can also persist this in your config file at `~/.config/metacritic-pp-cli/config.toml`.

### 3. Verify Setup

```bash
metacritic-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
metacritic-pp-cli composer mock-value --media-type games --api-key your-token-here
```

## Usage

Run `metacritic-pp-cli --help` for the full command reference and flag list.

## Commands

### composer

Manage composer

- **`metacritic-pp-cli composer <slug>`** - Returns the full composed detail payload for a title, including Metascore,
user score, summary, release data, and platform/genre metadata.

### finder

Manage finder

- **`metacritic-pp-cli finder browse-titles`** - Paginated, sortable list of titles for one medium. `mcoTypeId` selects the
medium (1 = TV shows, 2 = movies, 13 = games). Powers the /browse/ pages.
- **`metacritic-pp-cli finder list-filters`** - Returns the filter facets (genres, platforms, streaming networks, product
types) valid for the given medium.
- **`metacritic-pp-cli finder search-titles`** - Full-text search spanning games, movies, TV, and people.

### reviews

Critic and user reviews for a title

- **`metacritic-pp-cli reviews list-critic`** - List critic reviews for a title
- **`metacritic-pp-cli reviews list-user`** - List user reviews for a title


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
metacritic-pp-cli composer mock-value --media-type games --api-key your-token-here

# JSON for scripting and agents
metacritic-pp-cli composer mock-value --media-type games --api-key your-token-here --json

# Filter to specific fields
metacritic-pp-cli composer mock-value --media-type games --api-key your-token-here --json --select id,name,status

# Dry run — show the request without sending
metacritic-pp-cli composer mock-value --media-type games --api-key your-token-here --dry-run

# Agent mode — JSON + compact + no prompts in one flag
metacritic-pp-cli composer mock-value --media-type games --api-key your-token-here --agent
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
metacritic-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/metacritic-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `METACRITIC_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `metacritic-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `metacritic-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $METACRITIC_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
