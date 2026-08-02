# Dndbeyond CLI

Read-only access to D&D Beyond's public SRD and rules pages, plus local
normalization of user-supplied character-sheet snapshots and exported PDFs.

Learn more at [Dndbeyond](https://www.dndbeyond.com).

## Install

The recommended path installs both the `dndbeyond-pp-cli` binary and the `pp-dndbeyond` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install dndbeyond
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install dndbeyond --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install dndbeyond --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install dndbeyond --agent claude-code
npx -y @mvanhorn/printing-press-library install dndbeyond --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/dndbeyond/cmd/dndbeyond-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/dndbeyond-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install dndbeyond --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-dndbeyond --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-dndbeyond --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install dndbeyond --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/dndbeyond-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/dndbeyond/cmd/dndbeyond-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "dndbeyond": {
      "command": "dndbeyond-pp-mcp"
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
dndbeyond-pp-cli doctor
```

This checks your configuration.

### 3. Try Your First Command

```bash
dndbeyond-pp-cli pages search
```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local character sharing
- **`characters inspect`** — Normalize a user-supplied D&D Beyond or Beyond20 character snapshot or exported PDF.

  _Agents can read a user's authorized character artifact while keeping account access and mutations out of the CLI._

  ```bash
  dndbeyond-pp-cli characters inspect --file character.pdf --agent
  ```

## Usage

Run `dndbeyond-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `DNDBEYOND_CONFIG_DIR`, `DNDBEYOND_DATA_DIR`, `DNDBEYOND_STATE_DIR`, or `DNDBEYOND_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `DNDBEYOND_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export DNDBEYOND_HOME=/srv/dndbeyond
dndbeyond-pp-cli doctor
```

Under `DNDBEYOND_HOME=/srv/dndbeyond`, the four dirs resolve to `/srv/dndbeyond/config`, `/srv/dndbeyond/data`, `/srv/dndbeyond/state`, and `/srv/dndbeyond/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "dndbeyond": {
      "command": "dndbeyond-pp-mcp",
      "env": {
        "DNDBEYOND_HOME": "/srv/dndbeyond"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `DNDBEYOND_DATA_DIR` overrides an explicit `--home` for that kind. Use `DNDBEYOND_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `DNDBEYOND_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `dndbeyond-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### pages

Public D&D Beyond rules and reference pages.

- **`dndbeyond-pp-cli pages basic-rules`** - Read the public D&D Beyond Basic Rules landing page.
- **`dndbeyond-pp-cli pages rules-glossary`** - Read the public 2024 rules glossary page.
- **`dndbeyond-pp-cli pages search`** - Search D&D Beyond's public site search page.
- **`dndbeyond-pp-cli pages section`** - Read a public Basic Rules section by its documented URL slug.
- **`dndbeyond-pp-cli pages srd`** - Read the public System Reference Document landing page.

### characters

The character command is deliberately local-only. It accepts a JSON snapshot
that the user already has (including the documented Beyond20 character object)
or an exported, fillable D&D Beyond PDF. It returns a bounded, shareable sheet
summary and omits account identity, narrative notes, and secrets. It does not
log in, fetch private D&D Beyond data, roll dice, or mutate a character.

- **`dndbeyond-pp-cli characters inspect --file <path> --agent`** - Normalize a
  local character JSON snapshot or PDF for an agent or MCP caller. `.pdf` files
  are detected automatically; use `--format json` or `--format pdf` to be explicit.
- **`cat character.json | dndbeyond-pp-cli characters inspect --stdin --agent`**
  - Read a JSON snapshot from stdin without putting it in command-line history.
- **`Get-Content character.pdf -AsByteStream | dndbeyond-pp-cli characters inspect --stdin --format pdf --agent`**
  - Read an exported PDF from stdin without writing a parsed copy to disk.

The CLI does not claim a D&D Beyond character-data API. Public/session-backed
web discovery is intentionally a separate, user-consented follow-up because
the site may change its sheet DOM and its terms restrict unauthorized
extraction or automation. The PDF reader supports both ordinary AcroForm
fields and D&D Beyond's page-level widget export; image-only PDFs that require
OCR are not currently supported.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`dndbeyond-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`dndbeyond-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`dndbeyond-pp-cli learnings list`** - Inspect taught rows
- **`dndbeyond-pp-cli learnings forget <query>`** - Undo a teach
- **`dndbeyond-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`dndbeyond-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`dndbeyond-pp-cli teach-pattern`** - Install a query/resource template up front
- **`dndbeyond-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `DNDBEYOND_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `dndbeyond-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
dndbeyond-pp-cli pages search

# JSON for scripting and agents
dndbeyond-pp-cli pages search --json

# Filter to specific fields
dndbeyond-pp-cli pages search --json --select id,name,status

# Dry run — show the request without sending
dndbeyond-pp-cli pages search --dry-run

# Agent mode — JSON + compact + no prompts in one flag
dndbeyond-pp-cli pages search --agent
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
dndbeyond-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `dndbeyond-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/dndbeyond/config.toml`; `--home`, `DNDBEYOND_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

## HTTP Transport

This CLI uses standard HTTP transport with HTTP/2 disabled for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
