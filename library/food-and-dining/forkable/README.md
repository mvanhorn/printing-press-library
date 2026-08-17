# Forkable CLI

**A CLI and MCP server for your Forkable office-lunch program, with a local database and history, spend, and preference queries the web app cannot answer, plus commands to set, confirm, and skip meal orders (dry-run by default; --confirm to apply).**

Forkable exposes no public API. This CLI reverse-engineers the my-account app's GraphQL surface into a Go binary with clean read commands and agent-native output. On top of the raw reads it adds longitudinal views the product never shows — served-meal history, preference-vs-served drift, spend trends, allowance utilization, venue rotation, and a week-ahead digest — all fetched live from Forkable. It also exposes the my-account app's own meal-management mutations as meal set, meal set-all, meal confirm, meal skip, and reorder; these are dry-run by default and only place, confirm, or skip real orders when you pass --confirm.

Learn more at [Forkable](https://forkable.com).

## Install

The recommended path installs both the `forkable-pp-cli` binary and the `pp-forkable` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install forkable
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install forkable --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install forkable --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install forkable --agent claude-code
npx -y @mvanhorn/printing-press-library install forkable --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/forkable/cmd/forkable-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/forkable-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install forkable --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-forkable --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-forkable --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install forkable --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
forkable-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/forkable-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/forkable/cmd/forkable-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "forkable": {
      "command": "forkable-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Forkable authenticates with a browser session cookie plus a per-request CSRF token fetched from /api/v2/csrf_token. Log in to forkable.com in Chrome, then run 'forkable-pp-cli auth login --chrome' to import your session. There is no API key.

## Quick Start

```bash
# Verify the binary, config, and auth wiring before hitting the API.
forkable-pp-cli doctor --dry-run

# Import your logged-in Forkable session from Chrome (no API key exists).
forkable-pp-cli auth login --chrome

# Confirm auth works by fetching your profile and preferences.
forkable-pp-cli account --json

# See what you've actually eaten over the last quarter (fetched live).
forkable-pp-cli served-history --since 90d --agent

# Export per-month lunch spend for finance.
forkable-pp-cli spend-trend --since 6mo --by month --csv

```

## Known Gaps

- **`tail` and `import` do not work against Forkable.** These are generic
  REST helpers that build resource paths like `/deliveries` or `/account` and
  call them directly. Forkable exposes only a GraphQL endpoint
  (`/api/v2/graphql`), so those paths 404. Use the dedicated read commands
  (`deliveries`, `account`, `served-history`, etc.) instead; there is no
  streaming or bulk-import surface on this API.

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local history that compounds
- **`served-history`** — See every meal actually served to you over time, with date, venue, price, and dietary level.

  _Reach for this when an agent needs a longitudinal view of what a person has eaten, not just the current delivery._

  ```bash
  forkable-pp-cli served-history --since 90d --agent
  ```
- **`preference-drift`** — Flag served meals that violate your stated dislikes or dietary restrictions, or miss your likes.

  _Use this to audit whether auto-selection is actually honoring dietary preferences over time._

  ```bash
  forkable-pp-cli preference-drift --since 60d --agent
  ```
- **`venue-rotation`** — Rank venues by how often they've served you and how recently.

  _Use this to spot venue fatigue or under-used favorites across the whole synced window._

  ```bash
  forkable-pp-cli venue-rotation --since 120d --agent
  ```

### Making the opaque legible
- **`why-picked`** — Explain why a delivery's meal was auto-selected by ranking candidate items and their scores.

  _Pick this to explain a single day's auto-selected meal; use preference-drift for aggregate conformance._

  ```bash
  forkable-pp-cli why-picked --delivery 1219480 --agent
  ```

### Finance and allowances
- **`spend-trend`** — Bucket lunch spend into per-week or per-month totals with CSV export.

  _Reach for this when finance needs a time series of lunch cost, not a single delivery receipt._

  ```bash
  forkable-pp-cli spend-trend --since 6mo --by month --csv
  ```
- **`allowance-burn`** — Show granted-vs-consumed allowance utilization per club, including multi-club comparison.

  _Use this to see which teams are over- or under-using their lunch budget._

  ```bash
  forkable-pp-cli allowance-burn --by club --csv
  ```

### Agent-native plumbing
- **`upcoming-digest`** — One agent-shaped line per upcoming day: date, venue, auto-selected item, price, allowance headroom.

  _Pick this for a quick 'what's coming this week' summary an agent can read in one shot._

  ```bash
  forkable-pp-cli upcoming-digest --agent
  ```

## Recipes

### What have I eaten this quarter

```bash
forkable-pp-cli served-history --since 90d --agent --select date,venue,name,price
```

Longitudinal list of served meals, narrowed to the high-signal fields to keep agent context small.

### Audit dietary conformance

```bash
forkable-pp-cli preference-drift --since 60d --json
```

Flags any served meal that conflicts with your stated dislikes or restrictions.

### Monthly lunch spend for finance

```bash
forkable-pp-cli spend-trend --since 6mo --by month --csv
```

Per-month spend totals exported as CSV for a budget close.

### Which teams are burning their allowance

```bash
forkable-pp-cli allowance-burn --by club --csv
```

Granted-vs-consumed allowance utilization per club, side by side.

### This week's lunch at a glance

```bash
forkable-pp-cli upcoming-digest --agent
```

One compact line per upcoming day for a quick agent-readable briefing.

## Usage

Run `forkable-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `FORKABLE_CONFIG_DIR`, `FORKABLE_DATA_DIR`, `FORKABLE_STATE_DIR`, or `FORKABLE_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `FORKABLE_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export FORKABLE_HOME=/srv/forkable
forkable-pp-cli doctor
```

Under `FORKABLE_HOME=/srv/forkable`, the four dirs resolve to `/srv/forkable/config`, `/srv/forkable/data`, `/srv/forkable/state`, and `/srv/forkable/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "forkable": {
      "command": "forkable-pp-mcp",
      "env": {
        "FORKABLE_HOME": "/srv/forkable"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `FORKABLE_DATA_DIR` overrides an explicit `--home` for that kind. Use `FORKABLE_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `FORKABLE_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `forkable-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### account



- **`forkable-pp-cli account`** - Show the authenticated Forkable user: profile, roles, dietary preferences (likes/dislikes/restrictions), companies, managed clubs, and delegations.

### buffet_addresses



- **`forkable-pp-cli buffet-addresses`** - List your buffet delivery addresses (street, city, postal code, coordinates, club).

### clubs



- **`forkable-pp-cli clubs`** - List meal clubs (teams/offices) you belong to or manage, with delivery address, delivery days, allowances, billing/payment settings, and memberships. Override --query roles to filter by role.

### csrf



- **`forkable-pp-cli csrf`** - Fetch a CSRF token. Read-only. Used as the handshake before authenticated GraphQL queries and as the reachability/health probe.

### deliveries



- **`forkable-pp-cli deliveries in-progress-ids`** - List IDs of deliveries currently in progress.
- **`forkable-pp-cli deliveries list`** - List your meal deliveries from a given date forward, including per-delivery orders, chosen menu items, receipts, and delivery windows. Override --query to change the 'from' date.

### meal_scores



- **`forkable-pp-cli meal-scores`** - Show meal auto-selection scores (menuId, itemId, score) for a delivery and user across candidate menus. Requires deliveryId, userId, and menuIds inlined in the query.

### menus



- **`forkable-pp-cli menus`** - Get menu(s) with venue, sections, items, prices, dietary levels, ratings, and modifiers. Requires menu ids and a clubId inlined in the query; edit --query with values from your deliveries (availableMenuIds) and clubs (mealClubsAs).

### notifications



- **`forkable-pp-cli notifications`** - List account notifications shown in the my-account app (title, description, links, publish window).

### venue_usage



- **`forkable-pp-cli venue-usage`** - Get per-venue usage keyed by venue id over a date range. Requires venue ids and from/to dates inlined in the query.

### Meal management (writes)

These commands place real orders and spend against your account. They are **dry-run by default**: without `--confirm` they print the GraphQL mutation and variables they would send and stop. Pass `--confirm` to actually apply the change.

- **`forkable-pp-cli meal set <deliveryId> --item <id> --menu <id> [--modifier <modifierId>:<optionId>] [--replace-piece <uuid>] [--note <text>] [--confirm]`** - Override the auto-picked meal for one delivery day (`replacePiece`). `--replace-piece` takes the currently-selected piece's UUID (from `deliveries` or `served-history`); omit it to add a new piece.
- **`forkable-pp-cli meal set-all --deliveries <id,id> --item <id> --menu <id> [--modifier <modifierId>:<optionId>] [--confirm]`** - Apply one meal item across several delivery days (`replaceAllPieces`).
- **`forkable-pp-cli meal confirm <deliveryId> [--unconfirm] [--confirm]`** - Confirm (or `--unconfirm`) a delivery day (`confirmDelivery`).
- **`forkable-pp-cli meal skip <deliveryId> [--confirm]`** - Skip / cancel one or more delivery days (`removeDelivery`).
- **`forkable-pp-cli reorder <fromDate> --onto <deliveryId> [--replace-piece <uuid>] [--confirm]`** - Repeat the meal you had on a past date onto an upcoming delivery day.

**Choosing item options with `--modifier`.** Items that carry a required option — for example a "Choose Protein" group with `min: 1` — are rejected by Forkable unless you send a selection. Pass `--modifier <modifierId>:<optionId>[,<optionId>...]` (repeatable) to build that selection; the CLI assembles the `selectionsHash` object the `replacePiece` mutation needs. Find the modifier and option ids under each item's `modifiers` in the `menus` command output. Example: `forkable-pp-cli meal set 12345 --item 678 --menu 90 --modifier 16:10 --replace-piece <uuid> --confirm` selects option `10` for modifier `16`.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`forkable-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`forkable-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`forkable-pp-cli learnings list`** - Inspect taught rows
- **`forkable-pp-cli learnings forget <query>`** - Undo a teach
- **`forkable-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`forkable-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`forkable-pp-cli teach-pattern`** - Install a query/resource template up front
- **`forkable-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `FORKABLE_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `forkable-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
forkable-pp-cli buffet-addresses --query example-value

# JSON for scripting and agents
forkable-pp-cli buffet-addresses --query example-value --json

# Filter to specific fields
forkable-pp-cli buffet-addresses --query example-value --json --select id,name,status

# Dry run — show the request without sending
forkable-pp-cli buffet-addresses --query example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
forkable-pp-cli buffet-addresses --query example-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Safe writes** - read commands query Forkable; the write commands (`meal set`, `meal set-all`, `meal confirm`, `meal skip`, `reorder`) are dry-run by default and only mutate your account when you pass `--confirm`
- **Live fetch** - commands query Forkable directly over GraphQL; there is no local sync/cache step
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
forkable-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `forkable-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is ``; `--home`, `FORKABLE_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `forkable-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Commands return HTTP 401** — Your session expired; re-run 'forkable-pp-cli auth login --chrome' after logging in to forkable.com in Chrome.
- **A GraphQL query with placeholder ids returns empty** — menus/meal-scores/venue-usage need real ids: get them from 'deliveries list' (availableMenuIds) and 'clubs list', then pass a custom --query.
- **History or trend commands show nothing** — These fetch live from Forkable; make sure 'account --json' works first, then widen the window with --since 180d.
