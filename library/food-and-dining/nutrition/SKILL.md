---
name: pp-nutrition
description: "One agent-native CLI over USDA FoodData Central and NutritionValue.org - with cross-source enrichment, protein-density comparison, nutrient ranking, and a local SQLite cache no other nutrition tool ships. Trigger phrases: `nutrition facts for`, `how much protein in`, `compare calories of`, `net carbs in`, `foods highest in`, `log what I ate`, `use nutrition`, `run nutrition-pp-cli`."
author: "Matt Van Horn"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - nutrition-pp-cli
    install:
      - kind: go
        bins: [nutrition-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/food-and-dining/nutrition/cmd/nutrition-pp-cli
---

# Nutrition (USDA FoodData Central + NutritionValue.org) — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `nutrition-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install nutrition --cli-only
   ```
2. Verify: `nutrition-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/nutrition/cmd/nutrition-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Look up any food's nutrition from USDA's ~600K-food database, then enrich it with NutritionValue.org's derived analytics (net carbs, omega-6/omega-3 ratio) that the USDA API never exposes. Compare foods on a per-100kcal protein-density basis, rank foods by any nutrient, filter by compound thresholds, and log a daily diary against targets - all offline-cached, cited, and agent-native.

## When to Use This CLI

Use this CLI when an agent or user needs trustworthy, citable food nutrition data: looking up macros and micros for a food, comparing foods by protein density or on a common basis, ranking foods by a nutrient, finding foods that meet compound dietary thresholds, enriching a USDA record with NutritionValue.org's derived analytics, or logging a daily food diary against targets. It is offline-cached and agent-native, so repeated lookups and aggregations are cheap and the numbers always trace to a real USDA fdcId.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI for restaurant chain menu items - USDA and NutritionValue.org do not carry curated restaurant menus (use Nutritionix or CalorieKing).
- Do not use this CLI for glycemic index - neither source provides it.
- Do not use this CLI to place grocery orders or track weight/biometrics - it is a nutrition-facts and diary tool, not a fitness app.
- Do not use this CLI as a medical or clinical nutrition authority - it reports database values, not personalized dietary advice.

## Unique Capabilities

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

## Command Reference

**food** — Manage food

- `nutrition-pp-cli food <fdcId>` — Retrieves a single food item by an FDC ID. Optional format and nutrients can be specified.

**foods** — Manage foods

- `nutrition-pp-cli foods get` — Retrieves a list of food items by a list of up to 20 FDC IDs. Optional format and nutrients can be specified.
- `nutrition-pp-cli foods get-list` — Retrieves a paged list of foods. Use the pageNumber parameter to page through the entire result set.
- `nutrition-pp-cli foods get-search` — Search for foods using keywords.
- `nutrition-pp-cli foods post` — Retrieves a list of food items by a list of up to 20 FDC IDs. Optional format and nutrients can be specified.
- `nutrition-pp-cli foods post-list` — Retrieves a paged list of foods. Use the pageNumber parameter to page through the entire result set.
- `nutrition-pp-cli foods post-search` — Search for foods using keywords.

**json-spec** — Manage json spec

- `nutrition-pp-cli json-spec` — The OpenAPI 3.0 specification for the FDC API rendered as JSON (JavaScript Object Notation)

**yaml-spec** — Manage yaml spec

- `nutrition-pp-cli yaml-spec` — The OpenAPI 3.0 specification for the FDC API rendered as YAML (YAML Ain't Markup Language)


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
nutrition-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

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

## Auth Setup

USDA FoodData Central needs a free api.data.gov key. Set FDC_API_KEY (or USDA_API_KEY). Without a key the CLI falls back to USDA's public DEMO_KEY (rate-limited to ~30 requests/hour). NutritionValue.org commands need no key.

Run `nutrition-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  nutrition-pp-cli food mock-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `NUTRITION_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `NUTRITION_CONFIG_DIR`, `NUTRITION_DATA_DIR`, `NUTRITION_STATE_DIR`, `NUTRITION_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `NUTRITION_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `nutrition-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

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

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `NUTRITION_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `NUTRITION_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
nutrition-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
nutrition-pp-cli feedback --stdin < notes.txt
nutrition-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `NUTRITION_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `NUTRITION_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
nutrition-pp-cli profile save briefing --json
nutrition-pp-cli --profile briefing food mock-value
nutrition-pp-cli profile list --json
nutrition-pp-cli profile show briefing
nutrition-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `nutrition-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/food-and-dining/nutrition/cmd/nutrition-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add nutrition-pp-mcp -- nutrition-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which nutrition-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   nutrition-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `nutrition-pp-cli <command> --help`.
