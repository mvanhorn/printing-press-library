# Drug Enforcement CLI

**Search FDA drug recall records by drug, firm, recency, or recall number — keyless, with a safety contract that never calls a drug "safe".**

A keyless CLI over the openFDA drug enforcement endpoint. The check, firm, recent, and reference commands wrap FDA recall search with pre-built queries; every result cites the recall number, prints the FDA class legend, and carries an enforcement-not-medical-advice disclaimer. A drug with no recall is reported as 'no recall records found', never as safe.

## Install

The recommended path installs both the `drug-enforcement-pp-cli` binary and the `pp-drug-enforcement` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install drug-enforcement
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install drug-enforcement --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install drug-enforcement --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install drug-enforcement --agent claude-code
npx -y @mvanhorn/printing-press-library install drug-enforcement --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/health/drug-enforcement/cmd/drug-enforcement-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/drug-enforcement-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install drug-enforcement --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-drug-enforcement --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-drug-enforcement --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install drug-enforcement --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/drug-enforcement-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/health/drug-enforcement/cmd/drug-enforcement-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "drug-enforcement": {
      "command": "drug-enforcement-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# confirm the CLI and openFDA endpoint are reachable
drug-enforcement-pp-cli doctor --dry-run

# recalls mentioning a drug
drug-enforcement-pp-cli check "ibuprofen"

# narrow to the most serious (Class I) recalls
drug-enforcement-pp-cli check "metformin" --class 1

# recalls initiated in the last 30 days
drug-enforcement-pp-cli recent --days 30

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Recall lookups
- **`check`** — Find active FDA recalls that mention a drug, optionally filtered to the most serious class.

  _Reach for this when an agent needs the recall status of a specific medication by name._

  ```bash
  drug-enforcement-pp-cli check "ibuprofen" --class 1
  ```
- **`firm`** — List every recall attributed to a recalling firm or manufacturer.

  _Use this to audit a manufacturer's recall history._

  ```bash
  drug-enforcement-pp-cli firm "Teva"
  ```
- **`recent`** — List recalls initiated in the last N days, most recent first.

  _Use this for a periodic sweep of newly initiated drug recalls._

  ```bash
  drug-enforcement-pp-cli recent --days 30
  ```
- **`reference`** — Show full detail for a single recall number, with the FDA class legend.

  _Use this to expand one recall's full facts after a check/firm/recent lookup surfaces its number._

  ```bash
  drug-enforcement-pp-cli reference D-0183-2023
  ```

## Usage

Run `drug-enforcement-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `DRUG_ENFORCEMENT_CONFIG_DIR`, `DRUG_ENFORCEMENT_DATA_DIR`, `DRUG_ENFORCEMENT_STATE_DIR`, or `DRUG_ENFORCEMENT_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `DRUG_ENFORCEMENT_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export DRUG_ENFORCEMENT_HOME=/srv/drug-enforcement
drug-enforcement-pp-cli doctor
```

Under `DRUG_ENFORCEMENT_HOME=/srv/drug-enforcement`, the four dirs resolve to `/srv/drug-enforcement/config`, `/srv/drug-enforcement/data`, `/srv/drug-enforcement/state`, and `/srv/drug-enforcement/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "drug-enforcement": {
      "command": "drug-enforcement-pp-mcp",
      "env": {
        "DRUG_ENFORCEMENT_HOME": "/srv/drug-enforcement"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `DRUG_ENFORCEMENT_DATA_DIR` overrides an explicit `--home` for that kind. Use `DRUG_ENFORCEMENT_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `DRUG_ENFORCEMENT_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `drug-enforcement-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### enforcement

FDA drug recall enforcement records (openFDA /drug/enforcement.json)

- **`drug-enforcement-pp-cli enforcement`** - Search drug recall enforcement records with an openFDA Lucene search expression


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
drug-enforcement-pp-cli enforcement

# JSON for scripting and agents
drug-enforcement-pp-cli enforcement --json

# Filter to specific fields
drug-enforcement-pp-cli enforcement --json --select id,name,status

# Dry run — show the request without sending
drug-enforcement-pp-cli enforcement --dry-run

# Agent mode — JSON + compact + no prompts in one flag
drug-enforcement-pp-cli enforcement --agent
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
drug-enforcement-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `drug-enforcement-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/drug-enforcement/config.toml`; `--home`, `DRUG_ENFORCEMENT_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

### API-specific
- **check returns 'no recall records found'** — This means no FDA enforcement record matched — it is not a statement that the drug is safe. Consult a pharmacist or doctor.
- **HTTP 429 from openFDA** — openFDA is rate limited to 240 requests/minute per IP; wait and retry.
