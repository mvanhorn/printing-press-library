# Kalorické Tabulky CLI

**Every Kalorické Tabulky feature plus offline FTS5 search, macro-gap windowing, and weight regression — agent-ready Czech nutrition tracking.**

Czech-language nutrition CLI for kaloricketabulky.cz. Search 244 000 foods with diacritics-tolerant FTS5, log meals in one command via local food cache, compute macro gaps across weeks, regress weight, mine allergens — every endpoint the web app uses, plus 10 transcendence features the web app refuses to expose.

Printed by [@musketyr](https://github.com/musketyr) (musketyr).

## Install

The recommended path installs both the `kaloricke-tabulky-pp-cli` binary and the `pp-kaloricke-tabulky` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press install kaloricke-tabulky
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install kaloricke-tabulky --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press install kaloricke-tabulky --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press install kaloricke-tabulky --agent claude-code
npx -y @mvanhorn/printing-press install kaloricke-tabulky --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/kaloricke-tabulky-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-kaloricke-tabulky --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-kaloricke-tabulky --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-kaloricke-tabulky skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-kaloricke-tabulky. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
kaloricke-tabulky-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/kaloricke-tabulky-current).
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
    "kaloricke-tabulky": {
      "command": "kaloricke-tabulky-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Authenticate with `auth login --email <you>` (password read from stdin, MD5-hashed before sending, then discarded). The cookie session is stored at `~/.config/kaloricke-tabulky/session`. To avoid a password on disk, use `auth login --chrome` to import the session cookies directly from a logged-in Chrome profile. Run `auth refresh` to extend the session.

## Quick Start

```bash
# Authenticate once; password is MD5-hashed locally before sending and never written to disk.
kaloricke-tabulky-pp-cli auth password-login --email <your-email>


# Czech-language food search; results come from the live autocomplete API or local FTS5 cache.
kaloricke-tabulky-pp-cli food --query 'tvaroh'


# Resolve the food by name and log it in one command.
kaloricke-tabulky-pp-cli diary log 'tvaroh nízkotučný' --grams 150 --meal lunch


# Daily summary trimmed to the fields you care about.
kaloricke-tabulky-pp-cli summary today --json --select todayEnergy,todayEnergyTarget,todayDrink


# Record today's weight.
kaloricke-tabulky-pp-cli weight add 78.4


# Where this week's macros fell short, by meal slot.
kaloricke-tabulky-pp-cli macros gap --days 7 --by-meal
```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Logging without the click-cost
- **`diary log`** — Search foods by Czech text and log one to the diary in a single command, with the right meal slot.

  _Choose this when an agent or user wants to log a known food without first looking up its id._

  ```bash
  kaloricke-tabulky-pp-cli diary log 'tvaroh' --grams 150 --meal lunch
  ```
- **`diary plan-meal`** — Given remaining macro budget for the day, greedy-selects from favorites + most-frequent foods that close the protein gap within an energy ceiling.

  _Use this when an agent needs to suggest 'what should this person eat next' grounded in foods they actually like._

  ```bash
  kaloricke-tabulky-pp-cli diary plan-meal --target-protein 40 --remaining-energy 2500 --meal dinner --json
  ```
- **`diary unlog`** — Removes the most recently added diary entry from the cached last-logged record.

  _Use this to recover from a misclicked log without hunting through the day's meal slots._

  ```bash
  kaloricke-tabulky-pp-cli diary unlog --last
  ```

### Cross-day diet analytics
- **`macros gap`** — Across the last N days, show how much of each macro target remains, optionally grouped by meal slot.

  _Reach for this when an agent needs to reason about meal planning that hits weekly targets, not single-day totals._

  ```bash
  kaloricke-tabulky-pp-cli macros gap --days 7 --by-meal --json
  ```
- **`energy balance`** — Joins per-day diary energy with activity energy from the local store; prints daily series and 7/14-day moving average.

  _Use this for athletes whose fuelling depends on day-to-day training load._

  ```bash
  kaloricke-tabulky-pp-cli energy balance --days 14 --json --select date,energy_in_kj,energy_out_kj,net_kj
  ```
- **`diary frequency`** — Counts how often each foodstuff appears in your diary over a window. Surfaces dietary monotony or staple frequencies.

  _Pick this when you need 'what does this user actually eat?' rather than 'what's documented for one day'._

  ```bash
  kaloricke-tabulky-pp-cli diary frequency --days 30 --min 5 --meal dinner --json
  ```

### Czech-food intelligence
- **`food substitutes`** — Given a foodstuff, find others in the local store with similar macro profile per 100 g; rank by Euclidean distance.

  _Use this to swap a food for one that hits a different macro target without changing the calorie ballpark._

  ```bash
  kaloricke-tabulky-pp-cli food substitutes jablko --by protein --limit 10 --json
  ```
- **`food allergens`** — Extracts Czech allergen tokens (lepek, laktóza, vejce, ořechy, sója, ryby) from the JSON-LD keywords on a food detail page.

  _Reach for this when filtering foods for a user with dietary restrictions, before the web UI would ever surface that data._

  ```bash
  kaloricke-tabulky-pp-cli food allergens jablko --json
  ```

### Coaching-grade outputs
- **`weight regression`** — OLS regression on weight history; reports slope (kg/week), R^2, and days-to-target at current trajectory.

  _Use this for coaching conversations grounded in 'you're losing 0.4 kg/week' rather than chart-eyeball._

  ```bash
  kaloricke-tabulky-pp-cli weight regression --days 90 --target-kg 75 --json
  ```
- **`diary export-json`** — Pulls N days of diary into one JSON document with typed nutrition totals per day. Agent-friendly bulk format.

  _Use this when an agent needs to load a coaching client's diary into a pandas DataFrame or pipe it through analytics._

  ```bash
  kaloricke-tabulky-pp-cli diary export-json --from 2026-04-01 --to 2026-05-21
  ```

## Usage

Run `kaloricke-tabulky-pp-cli --help` for the full command reference and flag list.

## Commands

### achievements

Achievements + tips engine

- **`kaloricke-tabulky-pp-cli achievements`** - List achievements

### activity

Search the catalog of physical activities (kcal/min metadata)

- **`kaloricke-tabulky-pp-cli activity`** - Search activities by text (e.g. 'běh', 'plavání', 'joga')

### diary

Daily food + activity diary; default date is today

- **`kaloricke-tabulky-pp-cli diary days-filled`** - Which days have any diary entries
- **`kaloricke-tabulky-pp-cli diary get`** - Get diary for a date (Czech DD.MM.YYYY format)

### favorite

Per-user favorite foods and activities

- **`kaloricke-tabulky-pp-cli favorite list-activity`** - List favorite activities
- **`kaloricke-tabulky-pp-cli favorite list-food`** - List favorite foods

### find

Combined search across foods, activities, and recipes

- **`kaloricke-tabulky-pp-cli find`** - One search across all entity types

### food

Search the 244 000-food Czech database and look up nutrition by slug

- **`kaloricke-tabulky-pp-cli food`** - Search foods by Czech text (diacritics-tolerant on server)

### meal

User-defined custom meals (saved meal templates)

- **`kaloricke-tabulky-pp-cli meal`** - List saved meals

### notifications

In-app and site-wide messages

- **`kaloricke-tabulky-pp-cli notifications inapp`** - In-app messages for the current user
- **`kaloricke-tabulky-pp-cli notifications site`** - Site-wide messages

### recipe

Search recipes (called 'meals' in the underlying API)

- **`kaloricke-tabulky-pp-cli recipe`** - Search recipes by text

### session

Session lifecycle

- **`kaloricke-tabulky-pp-cli session`** - Extend the session (cookie refresh)

### stats

Public counters published on the home page

- **`kaloricke-tabulky-pp-cli stats diary-count`** - Total diary entries across all users
- **`kaloricke-tabulky-pp-cli stats foodstuff-count`** - Total foods in the catalog
- **`kaloricke-tabulky-pp-cli stats user-count`** - Total registered users

### streak

Logging streak gamification

- **`kaloricke-tabulky-pp-cli streak`** - Get logging streak

### summary

Daily summary including energy, drink, macro targets, and weight history

- **`kaloricke-tabulky-pp-cli summary <date>`** - Get daily summary (energy, drink targets, monthly weight history)

### weight

Record weight; history is in summary.monthWeight

- **`kaloricke-tabulky-pp-cli weight`** - Record weight for a date


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
kaloricke-tabulky-pp-cli achievements

# JSON for scripting and agents
kaloricke-tabulky-pp-cli achievements --json

# Filter to specific fields
kaloricke-tabulky-pp-cli achievements --json --select id,name,status

# Dry run — show the request without sending
kaloricke-tabulky-pp-cli achievements --dry-run

# Agent mode — JSON + compact + no prompts in one flag
kaloricke-tabulky-pp-cli achievements --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
kaloricke-tabulky-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/kaloricke-tabulky/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `kaloricke-tabulky-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **auth login returns code 0 but later calls 401** — Check that you're not behind a corporate proxy that strips Set-Cookie. Try `auth login --chrome` instead.
- **Czech food search returns no results** — Search uses diacritics-folded match; if your input has unusual punctuation, try plain ASCII (e.g. 'tvaroh' instead of 'Tvaroh měkký').
- **diary log says food not found** — Run `food search <text>` first to verify a match; if FTS5 cache is stale, run `sync foodstuffs` to repopulate.
- **weight regression complains 'not enough data points'** — Regression needs at least 3 weight entries within the window. Lower `--days` or log more weights.

---

## Known Gaps

These are documented limitations of the current build. The headline features (login, food search, nutrition lookup, diary read, macro analytics, weight regression, BMR) are unaffected.

1. **`food allergens` returns no useful data today.** The JSON-LD `keywords` array on `kaloricketabulky.cz` food pages contains nutritional fact strings (Energetická hodnota, Bílkoviny, ...), not allergen tokens. Every food currently returns `count: 0`. Fixing requires a different extraction source (HTML ingredient list, or a separate endpoint). The command ships so the data source can be swapped in later without changing the surface; consider this feature stubbed until then.

2. **`food allergens` and `food get` slug resolution can land on foreign-language foods.** Example: `food allergens chleba` may return a Finnish oat product because that slug fuzzy-matches. Until a name-match filter or Czech-biased routing is added, prefer slugs you've copy-pasted from a `food search` result over slugs you've typed by hand.

3. **`weight regression` requires logged weights.** If you haven't logged at least 3 weights in the chosen window, the command returns "not enough entries" rather than a fake-data ghost result. Log a few weights with `weight add <kg>` first.

4. **Diary write endpoints other than `diary log` are scaffolded but not fully verified.** `diary food remove`, `diary note add`, `diary copy`, `food create`, `meal create`, `favorite add/remove`, `diary export pdf|xls` ship as commands with correct `--help` and dry-run modes; their POST body shapes were derived from the AngularJS web bundle but not exercised end-to-end against the live server in this build. Use them on test data first; report any 400/500 responses as bug reports.

5. **Auth login flow.** This CLI ships `auth password-login --email <e>` (custom MD5 password POST matching the AngularJS web app) and the generator-emitted `auth login --chrome` (cookie import). The `--chrome` flow may need extra setup; prefer `password-login` when in doubt.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**TomasHubelbauer/kaloricke-tabulky-api**](https://github.com/TomasHubelbauer/kaloricke-tabulky-api) — TypeScript
- [**aquilax/hranoprovod-cli**](https://github.com/aquilax/hranoprovod-cli) — Go
- [**zupzup/calories**](https://github.com/zupzup/calories) — Rust
- [**peterkeen/calorific**](https://github.com/peterkeen/calorific) — Ruby
- [**pfirsich/welo**](https://github.com/pfirsich/welo) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
