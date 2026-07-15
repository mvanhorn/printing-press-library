# Nutrition (USDA FoodData Central + NutritionValue.org) CLI

**One agent-native CLI over USDA FoodData Central and NutritionValue.org - with cross-source enrichment, protein-density comparison, nutrient ranking, and a local SQLite cache no other nutrition tool ships.**

Look up any food's nutrition from USDA's ~600K-food database, then enrich it with NutritionValue.org's derived analytics (net carbs, omega-6/omega-3 ratio) that the USDA API never exposes. Compare foods on a per-100kcal protein-density basis, rank foods by any nutrient, filter by compound thresholds, and log a daily diary against targets - all offline-cached, cited, and agent-native.

Learn more at [Nutrition (USDA FoodData Central + NutritionValue.org)](https://nal.altarama.com/reft100.aspx?key=FoodData).

Created by [@mvanhorn](https://github.com/mvanhorn) (Matt Van Horn).

## Install

The recommended path installs both the `nutrition-pp-cli` binary and the `pp-nutrition` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install nutrition
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install nutrition --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install nutrition --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install nutrition --agent claude-code
npx -y @mvanhorn/printing-press-library install nutrition --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/nutrition/cmd/nutrition-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/nutrition-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install nutrition --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-nutrition --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-nutrition --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install nutrition --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/nutrition-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `FDC_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/nutrition/cmd/nutrition-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "nutrition": {
      "command": "nutrition-pp-mcp",
      "env": {
        "FDC_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

USDA FoodData Central needs a free api.data.gov key. Set FDC_API_KEY (or USDA_API_KEY). Without a key the CLI falls back to USDA's public DEMO_KEY (rate-limited to ~30 requests/hour). NutritionValue.org commands need no key.

## Quick Start

```bash
# Check API key config and reachability before anything else.
nutrition-pp-cli doctor --dry-run

# Find a food and grab a match's fdcId.
nutrition-pp-cli foods get-search --query "cheddar cheese" --page-size 5

# Full USDA nutrition record for cheddar (fdcId 173414).
nutrition-pp-cli food 173414

# Add NutritionValue.org derived analytics (net carbs, omega ratio).
nutrition-pp-cli enrich 173414 --agent

# Rank foods by protein density (grams protein per 100 kcal).
nutrition-pp-cli compare 2646170 173414 747447 --basis 100kcal --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Cross-source intelligence
- **`enrich`** — Merge NutritionValue.org's derived analytics (net carbs and omega-6/omega-3 ratio) onto a USDA food record by its shared FDC id.

  _Reach for this when a user wants net carbs or the omega-6/omega-3 ratio - numbers no USDA API call returns._

  ```bash
  nutrition-pp-cli enrich 173414 --agent
  ```
- **`rank`** — Top or bottom foods by any single nutrient, filterable by category and dataset, ingested from NutritionValue.org's precomputed ranking pages.

  _Reach for this to answer 'which foods are highest/lowest in nutrient X' without paging the whole database yourself._

  ```bash
  nutrition-pp-cli rank potassium --order lowest --agent
  ```

### Local math no API does
- **`compare`** — Compare 2-5 foods side by side on a common basis: per 100 g, per serving, or per 100 kcal (protein density).

  _Reach for this to rank foods by protein density or compare macros fairly across different serving sizes._

  ```bash
  nutrition-pp-cli compare 2646170 173414 747447 --basis 100kcal --agent
  ```
- **`find`** — Find foods that satisfy multiple nutrient thresholds at once (e.g. protein >= 20 g AND kcal <= 165 per 100 g).

  _Reach for this for diet-constrained discovery like high-protein low-calorie foods in one query._

  ```bash
  nutrition-pp-cli find --min protein=20 --max-kcal 165 --agent
  ```
- **`meal`** — Total nutrition across several foods at given quantities in one stateless call (id:quantity pairs).

  _Reach for this to get the nutrition of a recipe or plate without recording it in the diary._

  ```bash
  nutrition-pp-cli meal 173414:50g 1105314:120g --agent
  ```

### Local state that compounds
- **`log`** — A persistent daily food diary with macro/micro targets, backed by local SQLite, with today/summary/progress reports.

  _Reach for this to record what a user ate and report progress against daily targets - a Claude-loggable tracker._

  ```bash
  nutrition-pp-cli log progress --agent
  ```
- **`cite`** — Emit an APA or MLA citation for a USDA food record so agent output is verifiable and traceable to a real fdcId.

  _Reach for this when a user needs a citable source for a nutrition figure in a report or agent answer._

  ```bash
  nutrition-pp-cli cite 173414 --style apa
  ```

## Recipes

### Protein density leaderboard

```bash
nutrition-pp-cli compare 2646170 173414 747447 --basis 100kcal --agent --select foods.description,foods.protein_per_100kcal
```

Compare foods by grams of protein per 100 kcal and pull only the name and density fields.

### Low-carb shopping list

```bash
nutrition-pp-cli rank carbs --order lowest --limit 15 --agent
```

Get the 15 lowest-carbohydrate foods from NutritionValue.org's ranking pages.

### Enrich a food with derived analytics

```bash
nutrition-pp-cli enrich 173414 --agent --select net_carbs_g_per_100g,omega_6_3_ratio
```

Merge NutritionValue.org's net carbs and omega-6/omega-3 ratio onto a USDA record.

### Total a meal

```bash
nutrition-pp-cli meal 173414:50g 1105314:120g --agent
```

Sum nutrition across cheddar and a banana at given gram quantities in one call.

### Daily progress against targets

```bash
nutrition-pp-cli log progress --agent
```

Report today's logged intake versus configured macro targets.

## Usage

Run `nutrition-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `NUTRITION_CONFIG_DIR`, `NUTRITION_DATA_DIR`, `NUTRITION_STATE_DIR`, or `NUTRITION_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `NUTRITION_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export NUTRITION_HOME=/srv/nutrition
nutrition-pp-cli doctor
```

Under `NUTRITION_HOME=/srv/nutrition`, the four dirs resolve to `/srv/nutrition/config`, `/srv/nutrition/data`, `/srv/nutrition/state`, and `/srv/nutrition/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "nutrition": {
      "command": "nutrition-pp-mcp",
      "env": {
        "NUTRITION_HOME": "/srv/nutrition"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `NUTRITION_DATA_DIR` overrides an explicit `--home` for that kind. Use `NUTRITION_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `NUTRITION_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `nutrition-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### food

Manage food

- **`nutrition-pp-cli food <fdcId>`** - Retrieves a single food item by an FDC ID. Optional format and nutrients can be specified.

### foods

Manage foods

- **`nutrition-pp-cli foods get`** - Retrieves a list of food items by a list of up to 20 FDC IDs. Optional format and nutrients can be specified. Invalid FDC ID's or ones that are not found are omitted and an empty set is returned if there are no matches.
- **`nutrition-pp-cli foods get-list`** - Retrieves a paged list of foods. Use the pageNumber parameter to page through the entire result set.
- **`nutrition-pp-cli foods get-search`** - Search for foods using keywords. Results can be filtered by dataType and there are options for result page sizes or sorting.
- **`nutrition-pp-cli foods post`** - Retrieves a list of food items by a list of up to 20 FDC IDs. Optional format and nutrients can be specified. Invalid FDC ID's or ones that are not found are omitted and an empty set is returned if there are no matches.
- **`nutrition-pp-cli foods post-list`** - Retrieves a paged list of foods. Use the pageNumber parameter to page through the entire result set.
- **`nutrition-pp-cli foods post-search`** - Search for foods using keywords. Results can be filtered by dataType and there are options for result page sizes or sorting.

### json-spec

Manage json spec

- **`nutrition-pp-cli json-spec`** - The OpenAPI 3.0 specification for the FDC API rendered as JSON (JavaScript Object Notation)

### yaml-spec

Manage yaml spec

- **`nutrition-pp-cli yaml-spec`** - The OpenAPI 3.0 specification for the FDC API rendered as YAML (YAML Ain't Markup Language)


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
nutrition-pp-cli food mock-value

# JSON for scripting and agents
nutrition-pp-cli food mock-value --json

# Filter to specific fields
nutrition-pp-cli food mock-value --json --select id,name,status

# Dry run — show the request without sending
nutrition-pp-cli food mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
nutrition-pp-cli food mock-value --agent
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
nutrition-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `nutrition-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/food-data-central-pp-cli/config.toml`; `--home`, `NUTRITION_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `FDC_API_KEY` | per_call | Yes | Set to your API credential. |
| `USDA_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `nutrition-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `nutrition-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $FDC_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **HTTP 200 but the body is HTML, or 429 Too Many Requests** — You are rate-limited. Set a real key: export FDC_API_KEY=<your-key> (DEMO_KEY allows only ~30/hour).
- **food get returns sparse nutrients for a branded product** — Branded foods carry label nutrients only; use --data-type Foundation or SR Legacy for a full nutrient profile.
- **enrich or rank returns nothing** — NutritionValue.org data pages need a browser-like request; the CLI handles this, but retry once if the site is briefly slow, and keep requests polite.
- **search returns branded junk instead of the generic food** — Filter with --data-type Foundation,SR%20Legacy to prefer whole-food records.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**littlebunch/fdc-api**](https://github.com/littlebunch/fdc-api) — Go (18 stars)
- [**razvannicolae/Food-Facts-MCP**](https://github.com/razvannicolae/Food-Facts-MCP) — Python (13 stars)
- [**aquilax/hranoprovod-cli**](https://github.com/aquilax/hranoprovod-cli) — Go (11 stars)
- [**SciRustaceans/caltui**](https://github.com/SciRustaceans/caltui) — Go (2 stars)
- [**cyanheads/usda-mcp-server**](https://github.com/cyanheads/usda-mcp-server) — TypeScript
- [**daveremy/nutrition-mcp**](https://github.com/daveremy/nutrition-mcp) — TypeScript
- [**FergusFettes/food-cli**](https://github.com/FergusFettes/food-cli) — Python
- [**DeepSpaceCartel/fooddata-central-client-go**](https://github.com/DeepSpaceCartel/fooddata-central-client-go) — Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
