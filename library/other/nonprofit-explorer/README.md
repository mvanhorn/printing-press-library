# Nonprofit Explorer CLI

Look up US nonprofits and their IRS Form 990 financials via ProPublica's Nonprofit Explorer API. Free, no authentication required.

Created by [@fannan](https://github.com/fannan) (Sean Fannan).

## Install

The recommended path installs both the `nonprofit-explorer-pp-cli` binary and the `pp-nonprofit-explorer` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install nonprofit-explorer
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install nonprofit-explorer --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install nonprofit-explorer --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install nonprofit-explorer --agent claude-code
npx -y @mvanhorn/printing-press-library install nonprofit-explorer --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/nonprofit-explorer-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install nonprofit-explorer --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-nonprofit-explorer --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-nonprofit-explorer --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install nonprofit-explorer --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/nonprofit-explorer-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "nonprofit-explorer": {
      "command": "nonprofit-explorer-pp-mcp"
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
nonprofit-explorer-pp-cli doctor
```

This checks your configuration.

### 3. Try Your First Command

```bash
nonprofit-explorer-pp-cli search "food bank" --state CA
nonprofit-explorer-pp-cli org "american red cross"
```

## Usage

Run `nonprofit-explorer-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `NONPROFIT_EXPLORER_CONFIG_DIR`, `NONPROFIT_EXPLORER_DATA_DIR`, `NONPROFIT_EXPLORER_STATE_DIR`, or `NONPROFIT_EXPLORER_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `NONPROFIT_EXPLORER_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export NONPROFIT_EXPLORER_HOME=/srv/nonprofit-explorer
nonprofit-explorer-pp-cli doctor
```

Under `NONPROFIT_EXPLORER_HOME=/srv/nonprofit-explorer`, the four dirs resolve to `/srv/nonprofit-explorer/config`, `/srv/nonprofit-explorer/data`, `/srv/nonprofit-explorer/state`, and `/srv/nonprofit-explorer/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "nonprofit-explorer": {
      "command": "nonprofit-explorer-pp-mcp",
      "env": {
        "NONPROFIT_EXPLORER_HOME": "/srv/nonprofit-explorer"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `NONPROFIT_EXPLORER_DATA_DIR` overrides an explicit `--home` for that kind. Use `NONPROFIT_EXPLORER_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `NONPROFIT_EXPLORER_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `nonprofit-explorer-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

All EIN-taking commands accept either an EIN (with or without the dash —
`53-0196605` or `530196605`) or a nonprofit **name**, which auto-resolves to the
top search match with a `Resolved "<name>" → EIN <ein> (...)` note.

### search

Ranked live search of US nonprofits by name or keyword, with state, NTEE
major-group, and 501(c) sub-code filters. Results carry the full NTEE-CC
cause-area name (e.g. `K31Z` → Food Banks & Pantries).

```bash
nonprofit-explorer-pp-cli search "food bank" --state CA --limit 10
nonprofit-explorer-pp-cli search "community foundation" --c-code 3 --state NV
```

### org

Organization profile plus its latest Form 990: revenue, expenses, assets,
liabilities, net, NTEE-CC classification, and the filed-PDF link.

```bash
nonprofit-explorer-pp-cli org "american red cross"
nonprofit-explorer-pp-cli org 87-4084202
```

### financials

Year-by-year revenue / expense / net / asset trajectory with year-over-year
revenue change, plus the latest year's revenue composition (contributions,
program revenue, investment income, other) and personnel-cost share of expenses.

```bash
nonprofit-explorer-pp-cli financials "american red cross"
```

### filings

Every filing with parsed financial data, newest first, plus a count of
PDF-only filings.

```bash
nonprofit-explorer-pp-cli filings 530196605
```

### compare

Side-by-side latest-990 comparison of two or more organizations — mix names
and EINs freely.

```bash
nonprofit-explorer-pp-cli compare "american red cross" "united way worldwide"
```

### people

Officer compensation total and its share of expenses, other salaries & wages,
payroll taxes, and professional fundraising fees by year. Aggregates only:
individual officer names, titles, and per-person compensation live in the
filed 990 PDF (Part VII), linked per year.

```bash
nonprofit-explorer-pp-cli people "american red cross"
```

### Raw API mirrors

Thin 1:1 wrappers over the two ProPublica endpoints, returning the complete
unmodified JSON — use these when you need every raw extract field.

- **`nonprofit-explorer-pp-cli organizations <ein>`** - Raw `/organizations/<ein>.json`: full organization record plus `filings_with_data` (parsed financials) and `filings_without_data` (PDF-only).
- **`nonprofit-explorer-pp-cli search-json`** - Raw `/search.json`: full-text search with `--state-id`, `--ntee-id`, and `--c-code-id` filters. Paginated (100 results per page).

```bash
nonprofit-explorer-pp-cli organizations 530196605
nonprofit-explorer-pp-cli search-json --q "food bank" --state-id CA --data-source live
```


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`nonprofit-explorer-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`nonprofit-explorer-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`nonprofit-explorer-pp-cli learnings list`** - Inspect taught rows
- **`nonprofit-explorer-pp-cli learnings forget <query>`** - Undo a teach
- **`nonprofit-explorer-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`nonprofit-explorer-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`nonprofit-explorer-pp-cli teach-pattern`** - Install a query/resource template up front
- **`nonprofit-explorer-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `NONPROFIT_EXPLORER_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `nonprofit-explorer-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
nonprofit-explorer-pp-cli search "animal rescue" --state NV

# JSON for scripting and agents
nonprofit-explorer-pp-cli org "american red cross" --json

# Filter to specific fields
nonprofit-explorer-pp-cli org 530196605 --json --select ntee_name,latest_filing

# Dry run — show the request without sending
nonprofit-explorer-pp-cli organizations 530196605 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
nonprofit-explorer-pp-cli org "american red cross" --agent
```

## Cookbook

Real-world recipes; every command accepts an EIN (with or without dash) or a
nonprofit name that auto-resolves to the top match.

### Vet a charity before donating

```bash
nonprofit-explorer-pp-cli org "doctors without borders"
nonprofit-explorer-pp-cli financials "doctors without borders"
```

Profile plus latest Form 990, then the year-by-year revenue/expense/net
trajectory with YoY change and revenue composition.

### Find grantmaking foundations in your state

```bash
nonprofit-explorer-pp-cli search "family foundation" --state NV --c-code 3 --limit 15
```

### Compare peer organizations side by side

```bash
nonprofit-explorer-pp-cli compare "american red cross" "salvation army" 530196605
```

### Check officer compensation trends

```bash
nonprofit-explorer-pp-cli people "american red cross"
```

Officer compensation totals and share of expenses by year; per-year 990 PDF
links carry the individual officer detail (Part VII).

### Feed an agent pipeline

```bash
nonprofit-explorer-pp-cli org 530196605 --agent | jq '.results.latest_filing.totrevenue'
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
nonprofit-explorer-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `nonprofit-explorer-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/propublica-nonprofit-explorer-pp-cli/config.toml`; `--home`, `NONPROFIT_EXPLORER_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
