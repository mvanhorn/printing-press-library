# Table Reservation Goat CLI

One reservation CLI for OpenTable, Tock, and Resy — search all three networks at once, watch for cancellations, book + cancel end-to-end, and track changes from a local store agents can query.

Created by [@pejmanjohn](https://github.com/pejmanjohn) (Pejman Pour-Moezzi).
Contributors: [@ganes-j](https://github.com/ganes-j) (Jesse Ganes), [@teebs4140](https://github.com/teebs4140) (Dylan Thibault).

## Install

The recommended path installs both the `table-reservation-goat-pp-cli` binary and the `pp-table-reservation-goat` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install table-reservation-goat
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install table-reservation-goat --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install table-reservation-goat --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install table-reservation-goat --agent claude-code
npx -y @mvanhorn/printing-press-library install table-reservation-goat --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/cmd/table-reservation-goat-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/table-reservation-goat-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install table-reservation-goat --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-table-reservation-goat --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-table-reservation-goat --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install table-reservation-goat --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/table-reservation-goat-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/cmd/table-reservation-goat-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "table-reservation-goat": {
      "command": "table-reservation-goat-pp-mcp"
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
table-reservation-goat-pp-cli doctor
```

This checks your configuration.

### 3. Try Your First Command

```bash
```

## Usage

Run `table-reservation-goat-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `TABLE_RESERVATION_GOAT_CONFIG_DIR`, `TABLE_RESERVATION_GOAT_DATA_DIR`, `TABLE_RESERVATION_GOAT_STATE_DIR`, or `TABLE_RESERVATION_GOAT_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `TABLE_RESERVATION_GOAT_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export TABLE_RESERVATION_GOAT_HOME=/srv/table-reservation-goat
table-reservation-goat-pp-cli doctor
```

Under `TABLE_RESERVATION_GOAT_HOME=/srv/table-reservation-goat`, the four dirs resolve to `/srv/table-reservation-goat/config`, `/srv/table-reservation-goat/data`, `/srv/table-reservation-goat/state`, and `/srv/table-reservation-goat/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "table-reservation-goat": {
      "command": "table-reservation-goat-pp-mcp",
      "env": {
        "TABLE_RESERVATION_GOAT_HOME": "/srv/table-reservation-goat"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `TABLE_RESERVATION_GOAT_DATA_DIR` overrides an explicit `--home` for that kind. Use `TABLE_RESERVATION_GOAT_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `TABLE_RESERVATION_GOAT_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `table-reservation-goat-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### availability

Check open reservation slots across OpenTable, Tock, and Resy

- **`table-reservation-goat-pp-cli availability check`** - Check open slots for a restaurant on a specific date and party size
- **`table-reservation-goat-pp-cli availability multi-day`** - Multi-day availability for a single restaurant — Mon-Sun matrix

### experiences

List prepaid and tasting-menu experiences (Tock-style)


### me

Read your authenticated user profile from both networks


### reservations

List, book, modify, and cancel reservations (requires auth login)

OpenTable booking prefers attach mode when `TABLE_RESERVATION_GOAT_OT_CHROME_DEBUG_URL` is explicitly set. The attached Chrome profile must already be signed in to opentable.com. `TRG_ALLOW_BOOK=prepare` drives the exact date/time/party flow through an enabled final confirmation control without clicking it; only `TRG_ALLOW_BOOK=1` permits that final click.

```bash
export TABLE_RESERVATION_GOAT_OT_CHROME_DEBUG_URL=http://127.0.0.1:9223
TRG_ALLOW_BOOK=prepare table-reservation-goat-pp-cli book opentable:3688 --date 2026-07-20 --time 17:00 --party 2 --agent
```

Attach failures are machine-readable: `attach_unreachable`, `not_signed_in`, `selector_drift`, `form_validation`, `slot_taken`, and `incomplete_confirmation`. A committed booking whose restaurant ID cannot be recovered remains a `source: "book"` success and carries the `restaurant_id_unresolved` warning alongside its human-readable hint. Diagnostics expose only a sanitized page path and allowlisted control labels. When the debug endpoint is absent, OpenTable retains the existing HTTP booking path.

Tock booking uses the signed-in Chrome session at `TABLE_RESERVATION_GOAT_TOCK_CHROME_DEBUG_URL` (default `http://localhost:9222`) when reachable, with the existing stealth-headless session fallback otherwise. It preserves legacy time-slot buttons, then tries the exact-time `/search` row and the modern time-combobox/experience-card flow. The attached profile must already be signed in to exploretock.com.

Agent and `--no-input` runs never prompt for payment data. Set `TRG_TOCK_CVC` to a 3- or 4-digit CVC only when the venue requires per-booking verification; an omitted required CVC returns typed `cvc_required`. Layout failures return typed `selector_drift` with diagnostics limited to a query-free page path, booleans, time labels, and allowlisted control categories.


### restaurants

Search and inspect restaurants across OpenTable, Tock, and Resy

- **`table-reservation-goat-pp-cli restaurants get`** - Get a restaurant's full detail — hours, address, cuisine, price band, photos, accolades
- **`table-reservation-goat-pp-cli restaurants list`** - List restaurants across OpenTable, Tock, and Resy; filter by location, cuisine, price band, accolades, and party size

### wishlist

Read your saved/wishlisted restaurants from both networks



### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`table-reservation-goat-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`table-reservation-goat-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`table-reservation-goat-pp-cli learnings list`** - Inspect taught rows
- **`table-reservation-goat-pp-cli learnings forget <query>`** - Undo a teach
- **`table-reservation-goat-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`table-reservation-goat-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`table-reservation-goat-pp-cli teach-pattern`** - Install a query/resource template up front
- **`table-reservation-goat-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `TABLE_RESERVATION_GOAT_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `table-reservation-goat-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)

# JSON for scripting and agents

# Filter to specific fields

# Dry run — show the request without sending

# Agent mode — JSON + compact + no prompts in one flag
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
table-reservation-goat-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `table-reservation-goat-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/table-reservation-goat-pp-cli/config.toml`; `--home`, `TABLE_RESERVATION_GOAT_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
