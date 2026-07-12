# Janeapp CLI

CLI for Jane (janeapp.com) patient online booking. Log in to any Jane clinic
with your patient username and password, browse practitioners, treatments, and
live availability, view your upcoming and past appointments, and book, reschedule,
or cancel — across every clinic you use, from one tool. Jane is multi-tenant: each
clinic is its own subdomain (e.g. embophysio.janeapp.com), so each profile stores
its own base URL and session.

Learn more at [Janeapp](https://jane.app).

Created by [@omarshahine](https://github.com/omarshahine) (Omar Shahine).

## Install

The recommended path installs both the `janeapp-pp-cli` binary and the `pp-janeapp` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install janeapp
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install janeapp --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install janeapp --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install janeapp --agent claude-code
npx -y @mvanhorn/printing-press-library install janeapp --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/health/janeapp/cmd/janeapp-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/janeapp-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install janeapp --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-janeapp --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-janeapp --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install janeapp --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
janeapp-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/janeapp-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/health/janeapp/cmd/janeapp-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "janeapp": {
      "command": "janeapp-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Authenticate

This CLI uses your browser session for authentication. Log in to .janeapp.com in Chrome, then:

```bash
janeapp-pp-cli auth login --chrome
```

Or import an existing browser capture:

```bash
janeapp-pp-cli auth login --cookies-file storage-state.json
```

`--cookies-file` accepts Playwright storage-state JSON or a raw `Cookie:` header text file. The Chrome path requires a cookie extraction tool. Install one:

```bash
pip install pycookiecheat          # Python (recommended)
brew install barnardb/cookies/cookies  # Homebrew
```

When your session expires, run `auth login --chrome` again.

### 3. Verify Setup

```bash
janeapp-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
janeapp-pp-cli appointments
```

## Usage

Run `janeapp-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `JANEAPP_CONFIG_DIR`, `JANEAPP_DATA_DIR`, `JANEAPP_STATE_DIR`, or `JANEAPP_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `JANEAPP_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export JANEAPP_HOME=/srv/janeapp
janeapp-pp-cli doctor
```

Under `JANEAPP_HOME=/srv/janeapp`, the four dirs resolve to `/srv/janeapp/config`, `/srv/janeapp/data`, `/srv/janeapp/state`, and `/srv/janeapp/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "janeapp": {
      "command": "janeapp-pp-mcp",
      "env": {
        "JANEAPP_HOME": "/srv/janeapp"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `JANEAPP_DATA_DIR` overrides an explicit `--home` for that kind. Use `JANEAPP_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `JANEAPP_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `janeapp-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### appointments

Your own appointments at the clinic (requires a logged-in session).

- **`janeapp-pp-cli appointments`** - List your upcoming and past appointments for the active profile.

### disciplines

Disciplines (categories of care) offered by the clinic, e.g. Physical Therapy.

- **`janeapp-pp-cli disciplines`** - List disciplines (service categories) with descriptions.

### locations

Clinic locations (address, phone, booking URL) for the active profile's Jane instance.

- **`janeapp-pp-cli locations`** - List clinic locations with address, contact info, and booking URL.

### openings

Live availability (openings) for a practitioner + treatment at a location.

- **`janeapp-pp-cli openings`** - List available appointment openings for a staff member + treatment at a location over a date window. Jane caps num_days at 1..7; the CLI's next-opening/watch commands page across multiple windows automatically.

### staff

Practitioners (staff members) and the treatments they offer.

- **`janeapp-pp-cli staff`** - List practitioners, their bookable treatment IDs, and online-booking availability.

### treatments

Bookable treatments/services with price, duration, and online-booking eligibility.

- **`janeapp-pp-cli treatments`** - List treatments (services) with price, duration, discipline, and whether they can be booked online.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`janeapp-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`janeapp-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`janeapp-pp-cli learnings list`** - Inspect taught rows
- **`janeapp-pp-cli learnings forget <query>`** - Undo a teach
- **`janeapp-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`janeapp-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`janeapp-pp-cli teach-pattern`** - Install a query/resource template up front
- **`janeapp-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `JANEAPP_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `janeapp-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
janeapp-pp-cli appointments

# JSON for scripting and agents
janeapp-pp-cli appointments --json

# Filter to specific fields
janeapp-pp-cli appointments --json --select id,name,status

# Dry run — show the request without sending
janeapp-pp-cli appointments --dry-run

# Agent mode — JSON + compact + no prompts in one flag
janeapp-pp-cli appointments --agent
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

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `JANEAPP_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `janeapp-pp-cli appointments`
- `janeapp-pp-cli appointments get`
- `janeapp-pp-cli appointments list`
- `janeapp-pp-cli appointments search`
- `janeapp-pp-cli disciplines`
- `janeapp-pp-cli disciplines get`
- `janeapp-pp-cli disciplines list`
- `janeapp-pp-cli disciplines search`
- `janeapp-pp-cli locations`
- `janeapp-pp-cli locations get`
- `janeapp-pp-cli locations list`
- `janeapp-pp-cli locations search`
- `janeapp-pp-cli openings`
- `janeapp-pp-cli openings get`
- `janeapp-pp-cli openings list`
- `janeapp-pp-cli openings search`
- `janeapp-pp-cli staff`
- `janeapp-pp-cli staff get`
- `janeapp-pp-cli staff list`
- `janeapp-pp-cli staff search`
- `janeapp-pp-cli treatments`
- `janeapp-pp-cli treatments get`
- `janeapp-pp-cli treatments list`
- `janeapp-pp-cli treatments search`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Health Check

```bash
janeapp-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `janeapp-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/janeapp-pp-cli/config.toml`; `--home`, `JANEAPP_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `janeapp-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
