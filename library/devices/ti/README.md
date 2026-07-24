# Texas Instruments CLI

**TI part compliance without the portal: anonymous RoHS/REACH per orderable, plus cookie-injected FMD Class-D XML and per-part exemptions.**

Reads the compliance facts straight off TI's server-rendered product pages with plain HTTP — no login, no browser. When a myTI cookie snapshot is deposited (via the existing Browserbase human-in-the-loop login), the same CLI unlocks the Environmental Ratings table, per-part RoHS exemptions, and IPC-1752A Class-D FMD XML. It never logs in itself and fails closed when cookies are stale.

## Install

The recommended path installs both the `ti-pp-cli` binary and the `pp-ti` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install ti
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install ti --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install ti --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install ti --agent claude-code
npx -y @mvanhorn/printing-press-library install ti --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/ti/cmd/ti-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ti-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install ti --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-ti --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-ti --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install ti --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ti-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/devices/ti/cmd/ti-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "ti": {
      "command": "ti-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Anonymous commands (part compliance, part coc) need no credentials. Login-gated commands (part ratings, part fmd) consume a cookie snapshot captured by a human login in a Browserbase session — pass it via --cookies <file>. The CLI never performs a login and never sees your password; when cookies are stale it exits with a distinct 'login required' error so the orchestrator can trigger a fresh HIL login.

## Quick Start

```bash
# Health check — works without any credentials
ti-pp-cli doctor --dry-run

# Anonymous RoHS/REACH for one orderable part number
ti-pp-cli part compliance TUSB320RWBR --json

# Download TI's signed blanket compliance statement PDFs
ti-pp-cli part coc --out-dir ./evidence

# Verify a deposited myTI cookie snapshot before gated lookups
ti-pp-cli auth check --cookies ./ti-cookies.json

# Fetch the IPC-1752A Class-D FMD XML (requires working cookies)
ti-pp-cli part fmd TUSB320RWBR --cookies ./ti-cookies.json --out-dir ./evidence

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Extraction-lane plumbing
- **`part compliance`** — Get a part's RoHS and REACH verdicts plus the applicable signed TI compliance statement PDFs in one JSON envelope.

  _Reach for this when a supplier-extraction run needs verdicts AND evidence documents in a single deterministic call._

  ```bash
  ti-pp-cli part compliance TUSB320RWBR --json --coc-dir ./evidence
  ```
- **`part evidence`** — One command that always returns anonymous verdicts and blanket PDFs, and adds the rich ratings table and Class-D FMD XML when cookies work.

  _Use when you want the most complete evidence available right now without pre-deciding whether login works._

  ```bash
  ti-pp-cli part evidence TUSB320RWBR --cookies ./ti-cookies.json --out-dir ./evidence --json
  ```

### Login-gated access
- **`auth check`** — Find out whether the deposited myTI cookie snapshot still works before running a batch of gated extractions.

  _Run it before any part ratings or part fmd batch; a stale result means trigger a fresh HIL login instead of burning lookups._

  ```bash
  ti-pp-cli auth check --cookies ./ti-cookies.json --json
  ```

## Recipes

### Lane extraction bundle

```bash
ti-pp-cli part compliance TUSB320RWBR --json --coc-dir ./evidence
```

Verdicts plus applicable signed statement PDFs in one JSON envelope, shaped for the extraction lane.

### Pick out just the verdicts

```bash
ti-pp-cli part compliance TUSB320RWBR --agent --select rohs_compliance,reach_compliance
```

Narrow the envelope to the two fields the lane maps, saving agent context.

### Full evidence ladder

```bash
ti-pp-cli part evidence TUSB320RWBR --cookies ./ti-cookies.json --out-dir ./evidence --json
```

Everything available right now — anonymous rungs always, gated rungs when cookies work, per-rung status either way.

### Pre-flight a batch

```bash
ti-pp-cli auth check --cookies ./ti-cookies.json --json
```

Confirm the cookie snapshot is alive before a batch of gated lookups.

## Usage

Run `ti-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `TI_CONFIG_DIR`, `TI_DATA_DIR`, `TI_STATE_DIR`, or `TI_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `TI_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export TI_HOME=/srv/ti
ti-pp-cli doctor
```

Under `TI_HOME=/srv/ti`, the four dirs resolve to `/srv/ti/config`, `/srv/ti/data`, `/srv/ti/state`, and `/srv/ti/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "ti": {
      "command": "ti-pp-mcp",
      "env": {
        "TI_HOME": "/srv/ti"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `TI_DATA_DIR` overrides an explicit `--home` for that kind. Use `TI_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `TI_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `ti-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### literature



- **`ti-pp-cli literature <doc>`** - Signed blanket compliance statement PDFs (szzq088 RoHS, szzq119 RoHS exemptions, szzq087 REACH, szzq077 low-halogen, szzq195 IEC 62474).

### part



- **`ti-pp-cli part <generic>`** - Server-rendered product page carrying a JSON-LD block with per-orderable rohsCompliant, reachStatus, MSL rating, lead finish, and packaging.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`ti-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`ti-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`ti-pp-cli learnings list`** - Inspect taught rows
- **`ti-pp-cli learnings forget <query>`** - Undo a teach
- **`ti-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`ti-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`ti-pp-cli teach-pattern`** - Install a query/resource template up front
- **`ti-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `TI_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `ti-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
ti-pp-cli literature mock-value

# JSON for scripting and agents
ti-pp-cli literature mock-value --json

# Filter to specific fields
ti-pp-cli literature mock-value --json --select id,name,status

# Dry run — show the request without sending
ti-pp-cli literature mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
ti-pp-cli literature mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
ti-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `ti-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is ``; `--home`, `TI_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **part ratings / part fmd exits with 'login required or cookies stale'** — Trigger a fresh human login via the Browserbase HIL flow and re-deposit the cookie snapshot; myTI invalidates sessions server-side every few hours.
- **part compliance says the orderable was not found on the product page** — Check the OPN spelling; the CLI resolves the generic (TUSB320RWBR → TUSB320) and matches your OPN against the page's orderables list.
