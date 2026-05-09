# MyFitnessPal CLI

**Pull every meal you ever logged out of MyFitnessPal — per-food CSV, agent-shaped trends, and a local SQLite store no other MFP tool ships.**

MyFitnessPal closed their API, gates per-food export behind premium, and ships per-meal rows even there. This CLI imports your browser session, syncs your diary to local SQLite, and answers questions the official UI never can: which 5 foods drove 80% of your protein this quarter, what's your weekly weight slope vs your deficit, what changed since last sync. Built with agent-native output (`--json`, `--select`, `context`) so Claude can reason over your last 14 days in one tool call.

Learn more at [MyFitnessPal](https://www.myfitnesspal.com).

## Install

The recommended path installs both the `myfitnesspal-pp-cli` binary and the `pp-myfitnesspal` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install myfitnesspal
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install myfitnesspal --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/myfitnesspal/cmd/myfitnesspal-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/myfitnesspal-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-myfitnesspal --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-myfitnesspal --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-myfitnesspal skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-myfitnesspal. The skill defines how its required CLI can be installed.
```

## Authentication

MyFitnessPal closed their public API. This CLI uses your logged-in browser session — log in to myfitnesspal.com in Chrome, then run `myfitnesspal-pp-cli auth login --chrome`. Cookies are read from the .myfitnesspal.com domain. Sessions usually last 7-30 days; when they expire, log in again in Chrome and re-run `auth login --chrome`.

## Quick Start

```bash
# Imports your MFP cookies from Chrome (one-time setup; works after you've logged in to myfitnesspal.com)
myfitnesspal-pp-cli auth login --chrome


# Verifies the session is valid and api.myfitnesspal.com is reachable
myfitnesspal-pp-cli doctor


# Pulls four months of diary, exercises, water, measurements, and goals into the local SQLite store
myfitnesspal-pp-cli sync --from 2026-01-01 --to 2026-05-08


# Per-food CSV export — the headline thing premium MFP doesn't deliver
myfitnesspal-pp-cli export csv --from 2026-01-01 --to 2026-05-08 --out diary.csv


# One-shot agent context: 14 days of diary totals, weight trend, current goals, recent foods, macro deltas
myfitnesspal-pp-cli context --days 14 --json


# Joins weight measurements with calorie deficit to compute the implied calories-per-pound ratio
myfitnesspal-pp-cli analytics weight-trend --weeks 8 --smooth 7d

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`export csv`** — Export your food diary to CSV with one row per logged food, not per meal. Premium MFP only ships per-meal CSVs.

  _Reach for this when an agent needs the user's full eating history at food granularity for analysis, training, or long-form coaching memory._

  ```bash
  myfitnesspal-pp-cli export csv --from 2026-01-01 --to 2026-05-08 --out diary.csv
  ```
- **`analytics top-foods`** — Pareto query: which N foods drove X% of your protein/carbs/fat/fiber/sugar/calories over a window?

  _Use when an agent is helping the user understand what's actually driving a macro target, not what they think is._

  ```bash
  myfitnesspal-pp-cli analytics top-foods --nutrient protein --days 60 --cumulative-percent 80 --json
  ```
- **`find`** — Full-text search across every diary entry and food in the local store. Returns date, meal, servings, calories per match.

  _Use when an agent needs to recall every time the user logged a specific food without scrolling through months of diary._

  ```bash
  myfitnesspal-pp-cli find --food "Chipotle Bowl" --from 2026-01-01 --json
  ```
- **`analytics streak`** — Longest run of consecutive days where calorie totals fall within ±tolerance of your goal.

  _Use when the user asks how their adherence is trending — answer arrives without subjective interpretation._

  ```bash
  myfitnesspal-pp-cli analytics streak --days 60 --tolerance 0.05 --json
  ```

### Agent-native plumbing
- **`context`** — Single-call snapshot of the last N days: diary totals, weight trend, current goals, recent foods, macro deltas — sized for an agent context window.

  _First call any agent should make before reasoning about a user's nutrition — gives the full picture in one shot._

  ```bash
  myfitnesspal-pp-cli context --days 14 --json
  ```

## Usage

Run `myfitnesspal-pp-cli --help` for the full command reference and flag list.

## Commands

### api_user

Authenticated user record on the v2 API (preferences, paid subs, profiles).

- **`myfitnesspal-pp-cli api_user get`** - Get the v2 user record (units, goals preferences, paid subs, profiles).

### diary

Daily food diary (per-meal entries with full nutrient panel).

- **`myfitnesspal-pp-cli diary get_day`** - Get one day's food diary as scraped HTML (legacy surface python-myfitnesspal uses).
- **`myfitnesspal-pp-cli diary load_recent`** - Load the recent-foods quick-pick list for a meal.

### exercise

Cardio and strength exercises logged on a given day.

- **`myfitnesspal-pp-cli exercise get_day`** - Get one day's exercise log (cardio + strength) as scraped HTML.

### food

Search the public food database, view food details, log custom foods.

- **`myfitnesspal-pp-cli food details`** - Get full nutrient panel for a single food by MFP food id.
- **`myfitnesspal-pp-cli food search`** - Search the food database.
- **`myfitnesspal-pp-cli food suggested_servings`** - Get common serving-size suggestions for a food (powers the "1 cup / 100g / medium" picker).

### goals

Daily calorie / macro / water / weight goals.

- **`myfitnesspal-pp-cli goals get`** - Get your current daily goals (calorie target, macro split, water target) as scraped HTML.

### measurement

Weight, body fat, and other body measurements (time series).

- **`myfitnesspal-pp-cli measurement get_range`** - Get a date range of values for one measurement type as scraped HTML.
- **`myfitnesspal-pp-cli measurement types`** - List the measurement types defined for your account (Weight, BodyFat, Neck, Waist, Hips, plus custom).

### note

Free-text notes attached to a day's food or exercise diary.

- **`myfitnesspal-pp-cli note get`** - Get the food note for a single day.

### reports

Aggregated time-series reports (any nutrient or weight as a date->value series).

- **`myfitnesspal-pp-cli reports get`** - Get a time-series report (e.g. nutrition/Net%20Calories/30 returns the last 30 days of net calories).

### user

Authenticated user account info, units, and preferences.

- **`myfitnesspal-pp-cli user auth_token`** - Bootstrap a v2 bearer token from your session cookies.
- **`myfitnesspal-pp-cli user top_foods_server`** - Get your top-logged foods over a date range, computed server-side (powers the "your most-eaten" insights).

### water

Daily water intake tracking.

- **`myfitnesspal-pp-cli water get`** - Get water intake for a single day.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
myfitnesspal-pp-cli api_user --user-id 550e8400-e29b-41d4-a716-446655440000

# JSON for scripting and agents
myfitnesspal-pp-cli api_user --user-id 550e8400-e29b-41d4-a716-446655440000 --json

# Filter to specific fields
myfitnesspal-pp-cli api_user --user-id 550e8400-e29b-41d4-a716-446655440000 --json --select id,name,status

# Dry run — show the request without sending
myfitnesspal-pp-cli api_user --user-id 550e8400-e29b-41d4-a716-446655440000 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
myfitnesspal-pp-cli api_user --user-id 550e8400-e29b-41d4-a716-446655440000 --agent
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-myfitnesspal -g
```

Then invoke `/pp-myfitnesspal <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/myfitnesspal/cmd/myfitnesspal-pp-mcp@latest
```

Then register it:

```bash
# Some tools work without auth. For full access, set up auth first:
myfitnesspal-pp-cli auth login --chrome

claude mcp add myfitnesspal myfitnesspal-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
myfitnesspal-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/myfitnesspal-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/myfitnesspal/cmd/myfitnesspal-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "myfitnesspal": {
      "command": "myfitnesspal-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
myfitnesspal-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

- **Config file:** `~/.config/myfitnesspal-pp-cli/config.toml` (mode `0o600`, owner-only)
- **Local SQLite store:** `~/.local/share/myfitnesspal-pp-cli/data.db` (directory `0o700`, file `0o600`)
- **MFP transport state:** `~/.config/myfitnesspal-pp-cli/mfp-state.json` (cached numeric `mfp-user-id`)

## Known Gaps

This CLI ships with a focused subset of the planned manifest working end-to-end. The
remaining features are deferred to a follow-up `/printing-press-polish` run.

**Working today:**
- All JSON-only absorbed endpoints (`food details`, `food suggested-servings`, `food search` request*, `measurement types`, `water get`, `note get`, `reports get`, `api-user get`, `user auth-token`, `user top-foods-server`)
- `diary get-day` with the HTML parser ported from python-myfitnesspal v2.0.4
- `pull-diary` (date-range sync into local SQLite)
- `export csv` (per-food CSV — the headline feature premium MFP doesn't ship)
- `find` (FTS5 search across diary entries)
- `context` (one-call agent context dump for the last N days)
- `analytics top-foods` (Pareto query: which foods drove most of one nutrient)
- `analytics streak` (longest run inside ±tolerance of calorie goal)

**Deferred to v0.2 (`/printing-press-polish`):**
- HTML parsers for `/exercise/diary`, `/measurements/edit`, `/account/my-goals`, `POST /food/search`, `POST /food/load_recent` — currently the corresponding commands hit the live endpoints but emit raw HTML rather than parsed structures.
- Transcendence: `analytics weight-trend`, `analytics macro-trend`, `analytics weekly-diff`, `analytics gap-candidates`, `sync diff`, `reports backfill`, `meal expand`. Each is one focused command's worth of work over the local SQLite store.

The v0.2 features are not stubs in this build — they simply don't exist yet. Run
`/printing-press-polish myfitnesspal` to fill them in against the same spec.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `myfitnesspal-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **doctor reports session expired** — Log in to myfitnesspal.com in Chrome (the same profile you used for `auth login --chrome`), then re-run `myfitnesspal-pp-cli auth login --chrome`. Sessions typically last 7-30 days.
- **auth login --chrome can't find cookies** — Install one of: `pip install pycookiecheat`, `cargo install cookie-scoop-cli`, or `brew install cookies`. Then re-run `auth login --chrome --profile Default`. If you have multiple Chrome profiles, list them with `auth login --chrome --list-profiles`.
- **/v2/foods/{id} returns 403** — Some food ids are user-scoped (only readable by accounts that have logged that food). Try `food search --query <name>` first to get an accessible food id.
- **POST /food/search returns the login page** — Session cookie expired. Re-run `auth login --chrome` after logging in to myfitnesspal.com again.
- **sync fails with 429 rate-limit** — MFP throttles bursty access. The CLI defaults to 1 req/sec; if you raised --concurrency, drop it back to 1 and retry. Wait 60 seconds before re-running.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**coddingtonbear/python-myfitnesspal**](https://github.com/coddingtonbear/python-myfitnesspal) — Python (861 stars)
- [**marcosav/myfitnesspal-api**](https://github.com/marcosav/myfitnesspal-api) — Java (23 stars)
- [**savaki/myfitnesspal**](https://github.com/savaki/myfitnesspal) — Go (19 stars)
- [**hbmartin/myfitnesspal-to-google-sheets**](https://github.com/hbmartin/myfitnesspal-to-google-sheets) — Python (14 stars)
- [**AdamWalt/myfitnesspal-mcp-python**](https://github.com/AdamWalt/myfitnesspal-mcp-python) — Python (11 stars)
- [**seeM/myfitnesspal-to-sqlite**](https://github.com/seeM/myfitnesspal-to-sqlite) — Python (7 stars)
- [**jnelle/MyFitnesspal-API-Golang**](https://github.com/jnelle/MyFitnesspal-API-Golang) — Go (6 stars)
- [**seonixx/myfitnesspal**](https://github.com/seonixx/myfitnesspal) — Go (1 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
