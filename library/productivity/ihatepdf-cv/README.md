# ihatepdf.cv CLI

**Private PDF operations for agents, scripts, and offline pipelines.**

ihatepdf.cv CLI brings the website's no-upload promise to the terminal. Merge, inspect, fingerprint, encrypt, search, and catalog local documents with JSON output.

Learn more at [ihatepdf.cv](https://www.ihatepdf.cv).

Created by [@SomSamantray](https://github.com/SomSamantray) (Som Samantray).

## Install

The recommended path installs both the `ihatepdf-cv-pp-cli` binary and the `pp-ihatepdf-cv` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install ihatepdf-cv
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install ihatepdf-cv --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install ihatepdf-cv --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install ihatepdf-cv --agent claude-code
npx -y @mvanhorn/printing-press-library install ihatepdf-cv --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/ihatepdf-cv/cmd/ihatepdf-cv-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ihatepdf-cv-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install ihatepdf-cv --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-ihatepdf-cv --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-ihatepdf-cv --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install ihatepdf-cv --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ihatepdf-cv-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/ihatepdf-cv/cmd/ihatepdf-cv-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "ihatepdf-cv": {
      "command": "ihatepdf-cv-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Confirm the local-only runtime without touching a file.
ihatepdf-cv-pp-cli doctor --dry-run

# Inspect page count, metadata, and integrity before processing.
ihatepdf-cv-pp-cli inspect report.pdf --json

# Check for sensitive content and metadata risks.
ihatepdf-cv-pp-cli privacy-scan report.pdf --agent

# Record stable hashes for downstream verification.
ihatepdf-cv-pp-cli fingerprint report.pdf --json

```

## Privacy and capability boundaries

Browser-only ihatepdf.cv features such as redaction UI, workflow presets, OCR model downloads, interactive signatures, camera capture, P2P sharing, and whiteboard collaboration are intentionally not emulated by this CLI.

## Unique Features

These capabilities aren't available in any other tool for this API.

### Private local document pipelines
- **`privacy-scan`** — Find content and metadata risks before a PDF leaves your machine.

  _Choose this when an agent must check a document for accidental sensitive content before delivery._

  ```bash
  ihatepdf-cv-pp-cli privacy-scan report.pdf --agent
  ```
- **`catalog index`** — Index PDF paths, hashes, page counts, and extracted text in a local SQLite catalog.

  _Choose this when an agent needs a searchable, repeatable inventory of local PDFs._

  ```bash
  ihatepdf-cv-pp-cli catalog index ./reports --recursive --agent
  ```

### Trust and audit
- **`fingerprint`** — Record SHA-256, SHA-1, and MD5 hashes for inputs and outputs.

  _Choose this when downstream systems need to verify that a PDF was not changed._

  ```bash
  ihatepdf-cv-pp-cli fingerprint report.pdf --json
  ```

### Agent-native plumbing
- **`inspect`** — Expose PDF structure, metadata, and hashes as compact JSON.

  _Choose this before any transformation when an agent needs to understand the artifact._

  ```bash
  ihatepdf-cv-pp-cli inspect report.pdf --json
  ```
- **`search`** — Search extracted text across local PDFs and return matching snippets.

  _Choose this when an agent needs to locate a phrase across a local document set._

  ```bash
  ihatepdf-cv-pp-cli search invoice reports/*.pdf --agent
  ```

## Recipes

### Audit a PDF before sharing

```bash
ihatepdf-cv-pp-cli privacy-scan report.pdf --agent --select risks,metadata
```

Return only the risk and metadata fields an agent needs.

### Create an integrity record

```bash
ihatepdf-cv-pp-cli fingerprint report.pdf --json
```

Capture hashes for a reproducible handoff.

### Merge a packet

```bash
ihatepdf-cv-pp-cli merge cover.pdf appendix.pdf --output packet.pdf --json
```

Combine files in explicit order without uploading them.

## Usage

Run `ihatepdf-cv-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `IHATEPDF_CV_CONFIG_DIR`, `IHATEPDF_CV_DATA_DIR`, `IHATEPDF_CV_STATE_DIR`, or `IHATEPDF_CV_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `IHATEPDF_CV_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export IHATEPDF_CV_HOME=/srv/ihatepdf-cv
ihatepdf-cv-pp-cli doctor
```

Under `IHATEPDF_CV_HOME=/srv/ihatepdf-cv`, the four dirs resolve to `/srv/ihatepdf-cv/config`, `/srv/ihatepdf-cv/data`, `/srv/ihatepdf-cv/state`, and `/srv/ihatepdf-cv/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "ihatepdf-cv": {
      "command": "ihatepdf-cv-pp-mcp",
      "env": {
        "IHATEPDF_CV_HOME": "/srv/ihatepdf-cv"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `IHATEPDF_CV_DATA_DIR` overrides an explicit `--home` for that kind. Use `IHATEPDF_CV_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `IHATEPDF_CV_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `ihatepdf-cv-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### merge-pdf

Manage merge pdf

- **`ihatepdf-cv-pp-cli merge-pdf`** - Check the ihatepdf.cv website


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`ihatepdf-cv-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`ihatepdf-cv-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`ihatepdf-cv-pp-cli learnings list`** - Inspect taught rows
- **`ihatepdf-cv-pp-cli learnings forget <query>`** - Undo a teach
- **`ihatepdf-cv-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`ihatepdf-cv-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`ihatepdf-cv-pp-cli teach-pattern`** - Install a query/resource template up front
- **`ihatepdf-cv-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `IHATEPDF_CV_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `ihatepdf-cv-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
ihatepdf-cv-pp-cli merge-pdf

# JSON for scripting and agents
ihatepdf-cv-pp-cli merge-pdf --json
# Filter to specific fields by name
ihatepdf-cv-pp-cli merge-pdf --json --select <field>[,<field>...]

# Dry run — show the request without sending
ihatepdf-cv-pp-cli merge-pdf --dry-run

# Agent mode — JSON + compact + no prompts in one flag
ihatepdf-cv-pp-cli merge-pdf --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - catalog/search commands use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
ihatepdf-cv-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `ihatepdf-cv-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/ihatepdf-cv-local-pp-cli/config.toml`; `--home`, `IHATEPDF_CV_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **The input is not a readable PDF** — Run `ihatepdf-cv-pp-cli inspect file.pdf --json` and verify the path and file type.
- **A transformation refuses to overwrite a file** — Choose a different `--output` path; the CLI never overwrites inputs by default.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

TLS certificates are verified by default. For a trusted development or self-signed endpoint only, pass `--insecure` for one invocation, set `IHATEPDF_CV_SKIP_TLS_VERIFY=true` for the current environment, or set `skip_tls_verify = true` in the config file for a persistent override.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
