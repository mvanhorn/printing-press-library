# Haven Hot Chicken CLI

**Browse Haven menus, compare locations, and plan a meal from saved prices.**

Fetch public Haven location menus and item options. Save menus locally to compare prices, estimate base subtotals, inspect changes and find shared availability.

## Install

```bash
npx -y @mvanhorn/printing-press-library install haven-hot-chicken --cli-only
```

The catalog installer becomes available after this submission is merged and the library catalog is regenerated.

## Build from source

From this source directory, build the CLI with Go 1.26.6 or newer:

```powershell
go build -o haven-hot-chicken-pp-cli.exe ./cmd/haven-hot-chicken-pp-cli
./haven-hot-chicken-pp-cli.exe --help
```

Examples below use the binary name; use its full path when its folder is not on PATH. No API key or login is required for this public menu scope.


## Authentication

Public menu and location commands require no login or API key. The brand-routing header is a public identifier. Orders, payments, rewards and account history are outside this CLI.

## Quick Start

```bash
# Check the CLI setup.
haven-hot-chicken-pp-cli doctor --dry-run

# Save complete North Haven and New Haven menu observations.
haven-hot-chicken-pp-cli haven refresh --locations 14208,14205

# Read the saved menu with its observation time.
haven-hot-chicken-pp-cli haven menu --location 14208

# Compare the same named item across saved menus.
haven-hot-chicken-pp-cli haven compare --item "The Sandwich" --locations 14208,14205

```

## Recipes

### Compact menu

```bash
haven-hot-chicken-pp-cli haven menu --location 14208 --agent --select items.name,items.price_cents
```

Return only saved item names and base prices.

### Shared choices

```bash
haven-hot-chicken-pp-cli haven common --locations 14208,14205 --agent
```

Find items available in both saved menus.

### Saved changes

```bash
haven-hot-chicken-pp-cli haven changes --location 14208 --agent
```

Compare the two newest complete observations when available.

## Unique Features

Saved-menu tools.

### Saved menu comparisons
- **`haven compare`** — Compare the same named menu item across saved location menus.

  _Compare the same named menu item across saved location menus._

  ```bash
  haven-hot-chicken-pp-cli haven compare --item "The Sandwich" --locations 14208,14205 --agent
  ```
- **`haven changes`** — Show additions, removals, price and availability changes between two complete saved menus.

  _Show additions, removals, price and availability changes between two complete saved menus._

  ```bash
  haven-hot-chicken-pp-cli haven changes --location 14208 --agent
  ```
- **`haven common`** — Find items marked available in every selected saved location menu.

  _Find items marked available in every selected saved location menu._

  ```bash
  haven-hot-chicken-pp-cli haven common --locations 14208,14205 --agent
  ```

### Meal and pickup planning
- **`haven subtotal`** — Estimate a meal base-price subtotal from explicit item IDs and quantities.

  _Estimate a meal base-price subtotal from explicit item IDs and quantities._

  ```bash
  haven-hot-chicken-pp-cli haven subtotal --location 14208 --item 9656289=2 --agent
  ```
- **`haven nearby`** — Rank saved shops by straight-line distance from supplied coordinates.

  _Rank saved shops by straight-line distance from supplied coordinates._

  ```bash
  haven-hot-chicken-pp-cli haven nearby --lat 41.3 --lon -72.9 --agent
  ```

## Usage

Run `haven-hot-chicken-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `HAVEN_HOT_CHICKEN_CONFIG_DIR`, `HAVEN_HOT_CHICKEN_DATA_DIR`, `HAVEN_HOT_CHICKEN_STATE_DIR`, or `HAVEN_HOT_CHICKEN_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `HAVEN_HOT_CHICKEN_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export HAVEN_HOT_CHICKEN_HOME=/srv/haven-hot-chicken
haven-hot-chicken-pp-cli doctor
```

Under `HAVEN_HOT_CHICKEN_HOME=/srv/haven-hot-chicken`, the four dirs resolve to `/srv/haven-hot-chicken/config`, `/srv/haven-hot-chicken/data`, `/srv/haven-hot-chicken/state`, and `/srv/haven-hot-chicken/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "haven-hot-chicken": {
      "command": "haven-hot-chicken-pp-mcp",
      "env": {
        "HAVEN_HOT_CHICKEN_HOME": "/srv/haven-hot-chicken"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `HAVEN_HOT_CHICKEN_DATA_DIR` overrides an explicit `--home` for that kind. Use `HAVEN_HOT_CHICKEN_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `HAVEN_HOT_CHICKEN_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `haven-hot-chicken-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### items

Manage items


### locations

Manage locations

- **`haven-hot-chicken-pp-cli locations get`** - Get location
- **`haven-hot-chicken-pp-cli locations list`** - List locations

### menu-categories

Manage menu categories

- **`haven-hot-chicken-pp-cli menu-categories`** - List menu categories

### menus

Manage menus

- **`haven-hot-chicken-pp-cli menus`** - List menus


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`haven-hot-chicken-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`haven-hot-chicken-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`haven-hot-chicken-pp-cli learnings list`** - Inspect taught rows
- **`haven-hot-chicken-pp-cli learnings forget <query>`** - Undo a teach
- **`haven-hot-chicken-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`haven-hot-chicken-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`haven-hot-chicken-pp-cli teach-pattern`** - Install a query/resource template up front
- **`haven-hot-chicken-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `HAVEN_HOT_CHICKEN_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `haven-hot-chicken-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
haven-hot-chicken-pp-cli locations list

# JSON for scripting and agents
haven-hot-chicken-pp-cli locations list --json
# Filter to specific fields
haven-hot-chicken-pp-cli locations list --json --select alert_banner,business_hours,city

# Dry run — show the request without sending
haven-hot-chicken-pp-cli locations list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
haven-hot-chicken-pp-cli locations list --agent
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
haven-hot-chicken-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `haven-hot-chicken-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/haven-hot-chicken-pp-cli/config.toml`; `--home`, `HAVEN_HOT_CHICKEN_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
