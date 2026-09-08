# Pakistan Bureau of Statistics CLI

**The only machine-readable Pakistani price panel — every urban centre and essential item the Bureau prices weekly, accrued locally because the Bureau itself does not keep the series.**

Pakistan Bureau of Statistics publishes the richest sub-national price data in the country one machine-hostile weekly file at a time, indexes those files in a hand-edited JavaScript array, and deletes its own history: its live index reaches further back than the Internet Archive does, and one monthly annex is already served under two different months so one of them is lost. This CLI turns those scattered releases into one queryable panel and records every gap as a gap. Use spread and drift for questions the source cannot answer at all, coverage and verify to audit what you have, and revisions to catch what the Bureau quietly changed.

Learn more at [Pakistan Bureau of Statistics](https://www.pbs.gov.pk).

Created by [@qazmataz](https://github.com/qazmataz) (qazmataz).

## Install

The recommended path installs both the `pbs-pp-cli` binary and the `pp-pbs` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install pbs
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install pbs --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install pbs --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install pbs --agent claude-code
npx -y @mvanhorn/printing-press-library install pbs --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/pbs/cmd/pbs-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/pbs-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install pbs --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-pbs --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-pbs --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install pbs --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/pbs-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/pbs/cmd/pbs-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "pbs": {
      "command": "pbs-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# confirm the site is reachable before fetching anything
pbs-pp-cli doctor

# list the newest releases from the scraped index, sorted by real date
pbs-pp-cli releases --limit 5

# fetch and parse every release into the local store; resumable and checkpointed
pbs-pp-cli sync --full

# see exactly which weeks are missing before trusting any series
pbs-pp-cli coverage --gaps

# the question the source cannot answer: one item, two cities, over time
pbs-pp-cli drift "Wheat Flour Bag" --city lahore --city quetta

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Panel joins the source cannot answer
- **`spread`** — See the cheapest and dearest city for any essential item in any week, and how that gap has moved over time.

  _Reach for this when the question is about geography rather than time — which city is an outlier on this item right now._

  ```bash
  pbs-pp-cli spread "Onions" --as-of 2026-09-03 --agent
  ```
- **`drift`** — Follow one item's price across named cities and weeks, with missing weeks shown as gaps rather than filled.

  _Reach for this when the question is about time for a known item and known cities._

  ```bash
  pbs-pp-cli drift "Wheat Flour Bag" --city lahore --city quetta --agent
  ```
- **`basket`** — Build your own price index over any subset of items using the weight vector PBS republishes every week.

  _Reach for this to build a food-only or fuel-only index the Bureau never computes._

  ```bash
  pbs-pp-cli basket --item "Wheat Flour Bag" --item "Sugar Refined" --weight lowest --agent
  ```
- **`movers`** — Rank every item in a release by percent change recomputed from price levels, nationally or per city, with outliers flagged.

  _Reach for this when the question is which items moved this week, rather than one known item._

  ```bash
  pbs-pp-cli movers --as-of 2026-09-03 --top 10 --flag-outliers --agent
  ```

### Only the accrued store knows
- **`weights`** — Read the item weight vector for any release, diff two releases, and check the totals sum to 100.

  _Reach for this before trusting a long index series — a silent re-weighting breaks comparability._

  ```bash
  pbs-pp-cli weights --check-total --agent
  ```
- **`revisions`** — Detect releases the Bureau has rewritten, re-pointed, or deleted since you captured them.

  _Reach for this to prove your local copy is the surviving record, and to catch a silent restatement._

  ```bash
  pbs-pp-cli revisions --all --agent
  ```

### Honesty instrumentation
- **`verify`** — Audit parsed cell values against the source's own arithmetic invariants, including the weight totals and min-avg-max ordering.

  _Reach for this before quoting any figure — it distinguishes a real price move from a parse bug._

  ```bash
  pbs-pp-cli verify --all --agent
  ```
- **`coverage`** — Show which releases exist upstream, which the local store holds, and every hole named as a hole.

  _Reach for this whenever a result looks clean — a silently short series is the classic false positive._

  ```bash
  pbs-pp-cli coverage --gaps --agent
  ```

## Recipes

### Which city is gouging on onions this week

```bash
pbs-pp-cli spread "Onions" --as-of 2026-09-03 --agent --select summary.min_city,summary.max_city,summary.range,summary.contributing_cities
```

Narrows a wide cross-city payload to just the extremes and the count of cities that actually reported, so an agent does not spend context on all the per-city rows.

### Food inflation for the poorest quintile

```bash
pbs-pp-cli basket --item "Wheat Flour Bag" --item "Sugar Refined" --item "Pulse Gram" --weight lowest --json
```

Rebuilds an index over a chosen food subset using the Bureau's own lowest-quintile weights, which it republishes every week but never applies to a custom basket.

### Audit a series before quoting it

```bash
pbs-pp-cli coverage --gaps --state-census --json
```

Prints missing releases and the per-release census of zero, blank and N.A. cells, so a clean-looking result can be checked for a silently short sample.

### Catch what the Bureau changed under you

```bash
pbs-pp-cli revisions --all --json
```

Compares stored file hashes and index rows against the live site to surface rewritten bytes, re-pointed filenames, and releases that now return 404.

### Track how the basket itself was re-weighted

```bash
pbs-pp-cli weights --diff 2023-07-13..2026-09-03 --json
```

The weight vector ships in every release with no change log upstream, so this diff exists only because the local store kept both editions.

## Usage

Run `pbs-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `PBS_CONFIG_DIR`, `PBS_DATA_DIR`, `PBS_STATE_DIR`, or `PBS_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `PBS_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export PBS_HOME=/srv/pbs
pbs-pp-cli doctor
```

Under `PBS_HOME=/srv/pbs`, the four dirs resolve to `/srv/pbs/config`, `/srv/pbs/data`, `/srv/pbs/state`, and `/srv/pbs/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "pbs": {
      "command": "pbs-pp-mcp",
      "env": {
        "PBS_HOME": "/srv/pbs"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `PBS_DATA_DIR` overrides an explicit `--home` for that kind. Use `PBS_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `PBS_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `pbs-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### file

Static annexure/report files. All 202 live in one flat directory; the filename is the only variable.

- **`pbs-pp-cli file <filename>`** - Fetch one annexure, report, or CPI annex file by exact filename. Formats: .xlsx for the 44 most recent weekly releases (2025-10-23 onward) and .pdf for the other 104 -- so the text-layer PDF path covers 70% of the panel and is not optional. Never construct this filename; take it from `releases`.

### index

The PBS price-statistics release index. Carries TWO JavaScript literal arrays in the served HTML: `data` (148 weekly SPI releases, 2023-07-13..2026-09-03) and `cpidata1` (50 CPI months, July 2022..August 2026).

- **`pbs-pp-cli index`** - Fetch the release-index page. THE ONLY ENUMERATOR -- release URLs are UNCONSTRUCTIBLE: 35 distinct annexure filename shapes across 148 rows (top shape covers only 53), including one `Annex.pdf` with no date at all, plus load-bearing typos (SumarySPI, SummaryReport, USCCP, Annexture). A guessed URL returns a clean HTTP 404, never a soft 200. This endpoint's html_extract returns the raw file-link list; the authoritative date-paired index comes from the hand-written JS-literal parser in `releases`.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`pbs-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`pbs-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`pbs-pp-cli learnings list`** - Inspect taught rows
- **`pbs-pp-cli learnings forget <query>`** - Undo a teach
- **`pbs-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`pbs-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`pbs-pp-cli teach-pattern`** - Install a query/resource template up front
- **`pbs-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `PBS_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `pbs-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
pbs-pp-cli file mock-value

# JSON for scripting and agents
pbs-pp-cli file mock-value --json
# Filter to specific fields by name
pbs-pp-cli file mock-value --json --select <field>[,<field>...]

# Dry run — show the request without sending
pbs-pp-cli file mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
pbs-pp-cli file mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
pbs-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `pbs-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/pbs-pp-cli/config.toml`; `--home`, `PBS_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **A series looks shorter than expected, or a trend has a suspicious flat stretch.** — Run 'pbs-pp-cli coverage --gaps'. Seventeen weeks are absent from the upstream index itself, including a seven-week hole in autumn 2024.
- **A city average looks far too low for an item.** — Run 'pbs-pp-cli coverage --state-census'. A numeric zero, a blank cell and the string N.A. are three different kinds of no-price and must not be averaged as zero.
- **A percent-change figure reads as exactly zero.** — That is the source's own column, which is genuinely zero. Use 'pbs-pp-cli movers', which recomputes change from price levels.
- **A rebuilt national average does not match the published one.** — Expected. The Bureau weights cities and does not publish the city weight vector, so 'pbs-pp-cli verify --check national-average' reports the divergence rather than failing.
- **A release fetch returns HTTP 404.** — Release URLs are not constructible and the Bureau does rewrite them. Re-run 'pbs-pp-cli releases' to refresh the index, then 'pbs-pp-cli revisions' to see what moved.
