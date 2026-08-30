# CookUnity CLI

**Mirror CookUnity's full weekly menu into a local database so you can plan meals offline — filter by macros, diet, and budget, and diff the menu week over week, none of which the web app can do.**

CookUnity's chef-driven menu is only visible logged-in and online, paginated across hundreds of lazy-loaded cards, with shallow filters that reset on reload. This CLI syncs the entire menu — every meal's macros, chef, price, diet tags, allergens, and ratings — into a local SQLite store, then lets you query it offline: build a macro-constrained meal plan with `plan`, rank best value with `value`, see what changed with `drift`, and search everything with FTS. Every command speaks `--json` for agents.

## Install

The recommended path installs both the `cookunity-pp-cli` binary and the `pp-cookunity` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install cookunity
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install cookunity --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install cookunity --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install cookunity --agent claude-code
npx -y @mvanhorn/printing-press-library install cookunity --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/cookunity/cmd/cookunity-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/cookunity-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install cookunity --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-cookunity --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-cookunity --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install cookunity --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/cookunity-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `COOKUNITY_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/cookunity/cmd/cookunity-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "cookunity": {
      "command": "cookunity-pp-mcp",
      "env": {
        "COOKUNITY_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

CookUnity has no public API. Auth is the Auth0 access token your logged-in browser session already holds. Copy it from the browser (DevTools → Network → any subscription.cookunity.com request → the `authorization` header value, which is the raw token with no `Bearer ` prefix) and set `COOKUNITY_TOKEN`. The token lasts about 24 hours; re-copy it when `doctor` reports it expired.

## Quick Start

```bash
# check config and token before syncing
cookunity-pp-cli doctor --dry-run

# mirror the full menu for a delivery date into the local store (omit --param to use the next delivery date)
cookunity-pp-cli sync --param date=2026-08-04

# browse the menu offline with macro filters
cookunity-pp-cli meals --min-protein 40 --max-calories 600 --json

# auto-build a week of meals hitting your targets
cookunity-pp-cli plan --protein-min 40 --calories-max 600 --count 8 --diet gluten-free

# rank the menu by protein per dollar
cookunity-pp-cli value --metric protein-per-dollar --limit 20

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local planning that compounds
- **`plan`** — Auto-build a set of meals that collectively hit your protein, calorie, budget, and diet targets — offline, in one command.

  _Reach for this when the user wants a week's worth of meals meeting nutrition or budget constraints instead of hand-picking dishes._

  ```bash
  cookunity-pp-cli plan --protein-min 40 --calories-max 600 --count 8 --diet gluten-free --agent
  ```
- **`drift`** — See exactly which meals were added, removed, or repriced between two weekly menus.

  _Use this to track what changed on the new menu without re-scanning 300+ dishes by hand._

  ```bash
  cookunity-pp-cli drift --from 2026-07-28 --to 2026-08-04 --json
  ```
- **`value`** — Rank meals by nutritional value per dollar (protein-per-dollar, calories-per-dollar) computed from the local mirror.

  _Pick this when the user cares about maximizing protein or calories per dollar across the menu._

  ```bash
  cookunity-pp-cli value --metric protein-per-dollar --limit 20 --agent
  ```

### Agent-native menu queries
- **`favorites`** — Show which of your favorited meals are actually on the current week's menu.

  _Use this first after a menu drop to see if the user's liked meals are back._

  ```bash
  cookunity-pp-cli favorites --json
  ```
- **`compare`** — Side-by-side macro, price, chef, and allergen comparison of two or more meals.

  _Reach for this to decide between specific candidate meals the user already has in mind._

  ```bash
  cookunity-pp-cli compare 3707 3210 --json
  ```
- **`chefs`** — Leaderboard of chefs by dish count, average rating, average price, and cuisines on the current menu.

  _Use this to explore the menu by chef or find highly-rated chefs._

  ```bash
  cookunity-pp-cli chefs --json
  ```

## Recipes

### Plan a high-protein gluten-free week

```bash
cookunity-pp-cli plan --protein-min 40 --calories-max 650 --count 8 --diet gluten-free --agent
```

Selects 8 meals from the synced menu that collectively fit the macro and diet constraints.

### Find the best protein-per-dollar meals

```bash
cookunity-pp-cli value --metric protein-per-dollar --limit 15 --agent --select name,chefName,protein,finalPrice
```

Ranks meals by protein per dollar and returns only the fields an agent needs, keeping the payload small.

### See what changed on the new menu

```bash
cookunity-pp-cli drift --from 2026-07-28 --to 2026-08-04 --json
```

Diffs two synced weekly snapshots to show added, removed, and repriced meals.

### Compare two meals head to head

```bash
cookunity-pp-cli compare 3707 3210 --json
```

Prints a side-by-side macro/price/chef/allergen comparison of the two meal ids.

## Usage

Run `cookunity-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `COOKUNITY_CONFIG_DIR`, `COOKUNITY_DATA_DIR`, `COOKUNITY_STATE_DIR`, or `COOKUNITY_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `COOKUNITY_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export COOKUNITY_HOME=/srv/cookunity
cookunity-pp-cli doctor
```

Under `COOKUNITY_HOME=/srv/cookunity`, the four dirs resolve to `/srv/cookunity/config`, `/srv/cookunity/data`, `/srv/cookunity/state`, and `/srv/cookunity/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "cookunity": {
      "command": "cookunity-pp-mcp",
      "env": {
        "COOKUNITY_HOME": "/srv/cookunity"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `COOKUNITY_DATA_DIR` overrides an explicit `--home` for that kind. Use `COOKUNITY_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `COOKUNITY_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `cookunity-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### meals

Query the locally-synced CookUnity meal catalog offline

- **`cookunity-pp-cli meals`** - List meals (hand-built to read the local store; this endpoint is the upstream source)

### menu

Fetch the CookUnity weekly menu (server-driven-UI) for a delivery date



### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`cookunity-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`cookunity-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`cookunity-pp-cli learnings list`** - Inspect taught rows
- **`cookunity-pp-cli learnings forget <query>`** - Undo a teach
- **`cookunity-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`cookunity-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`cookunity-pp-cli teach-pattern`** - Install a query/resource template up front
- **`cookunity-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `COOKUNITY_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `cookunity-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
cookunity-pp-cli meals

# JSON for scripting and agents
cookunity-pp-cli meals --json

# Filter to specific fields
cookunity-pp-cli meals --json --select id,name,status

# Dry run — show the request without sending
cookunity-pp-cli meals --dry-run

# Agent mode — JSON + compact + no prompts in one flag
cookunity-pp-cli meals --agent
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
cookunity-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `cookunity-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/cookunity-pp-cli/config.toml`; `--home`, `COOKUNITY_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `COOKUNITY_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `cookunity-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `cookunity-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $COOKUNITY_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **doctor reports the token is missing or expired (401)** — Re-copy the raw `authorization` header value from a logged-in subscription.cookunity.com request and set COOKUNITY_TOKEN; tokens last ~24h.
- **sync returns no meals** — Confirm the date passed via --param date=YYYY-MM-DD is a valid upcoming delivery date (menus publish ~2 weeks ahead) and that COOKUNITY_TOKEN is set.
- **meals is empty** — Run `cookunity-pp-cli sync` first; the meals command reads the local mirror.
