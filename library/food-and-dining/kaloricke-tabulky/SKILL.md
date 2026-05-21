---
name: pp-kaloricke-tabulky
description: "Every Kalorické Tabulky feature plus offline FTS5 search, macro-gap windowing, and weight regression —... Trigger phrases: `log my breakfast on kalorické tabulky`, `what's my macro gap for the week`, `search Czech food nutrition`, `track my weight in kalorické tabulky`, `use kaloricke-tabulky`, `run kaloricke-tabulky`."
author: "Vladimir Orany"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - kaloricke-tabulky-pp-cli
---

# Kalorické Tabulky — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `kaloricke-tabulky-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install kaloricke-tabulky --cli-only
   ```
2. Verify: `kaloricke-tabulky-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Czech-language nutrition CLI for kaloricketabulky.cz. Search 244 000 foods with diacritics-tolerant FTS5, log meals in one command via local food cache, compute macro gaps across weeks, regress weight, mine allergens — every endpoint the web app uses, plus 10 transcendence features the web app refuses to expose.

## When to Use This CLI

Reach for this CLI when you need automated or agentic access to Kalorické Tabulky data — bulk diary exports, multi-day macro analytics, weight trend regression, allergen filtering, or quick food logging. It is Czech-language-aware (diacritics-folded search) and handles the kJ/kcal duality natively. Not the right tool for ad-hoc browser-only sessions where the web UI is faster.

## Unique Capabilities

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

## Command Reference

**achievements** — Achievements + tips engine

- `kaloricke-tabulky-pp-cli achievements` — List achievements

**activity** — Search the catalog of physical activities (kcal/min metadata)

- `kaloricke-tabulky-pp-cli activity` — Search activities by text (e.g. 'běh', 'plavání', 'joga')

**diary** — Daily food + activity diary; default date is today

- `kaloricke-tabulky-pp-cli diary days-filled` — Which days have any diary entries
- `kaloricke-tabulky-pp-cli diary get` — Get diary for a date (Czech DD.MM.YYYY format)

**favorite** — Per-user favorite foods and activities

- `kaloricke-tabulky-pp-cli favorite list-activity` — List favorite activities
- `kaloricke-tabulky-pp-cli favorite list-food` — List favorite foods

**find** — Combined search across foods, activities, and recipes

- `kaloricke-tabulky-pp-cli find` — One search across all entity types

**food** — Search the 244 000-food Czech database and look up nutrition by slug

- `kaloricke-tabulky-pp-cli food` — Search foods by Czech text (diacritics-tolerant on server)

**meal** — User-defined custom meals (saved meal templates)

- `kaloricke-tabulky-pp-cli meal` — List saved meals

**notifications** — In-app and site-wide messages

- `kaloricke-tabulky-pp-cli notifications inapp` — In-app messages for the current user
- `kaloricke-tabulky-pp-cli notifications site` — Site-wide messages

**recipe** — Search recipes (called 'meals' in the underlying API)

- `kaloricke-tabulky-pp-cli recipe` — Search recipes by text

**session** — Session lifecycle

- `kaloricke-tabulky-pp-cli session` — Extend the session (cookie refresh)

**stats** — Public counters published on the home page

- `kaloricke-tabulky-pp-cli stats diary-count` — Total diary entries across all users
- `kaloricke-tabulky-pp-cli stats foodstuff-count` — Total foods in the catalog
- `kaloricke-tabulky-pp-cli stats user-count` — Total registered users

**streak** — Logging streak gamification

- `kaloricke-tabulky-pp-cli streak` — Get logging streak

**summary** — Daily summary including energy, drink, macro targets, and weight history

- `kaloricke-tabulky-pp-cli summary <date>` — Get daily summary (energy, drink targets, monthly weight history)

**weight** — Record weight; history is in summary.monthWeight

- `kaloricke-tabulky-pp-cli weight` — Record weight for a date


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
kaloricke-tabulky-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Find Czech protein-dense foods I've actually eaten

```bash
kaloricke-tabulky-pp-cli diary frequency --days 60 --json | jq '.[] | select(.foodstuff.protein_g_per_100g > 20)'
```

Frequency analysis surfaces what you actually log; jq filters by typed nutrition.

### Energy-balance check for an ultrarunner

```bash
kaloricke-tabulky-pp-cli energy balance --days 14 --json --select date,energy_in_kj,energy_out_kj,net_kj
```

Joins diary energy and activity energy across a 14-day window with deeply-nested fields trimmed via --select for compact agent context.

### Daily macro gap for meal planning

```bash
kaloricke-tabulky-pp-cli macros gap --by-meal --json
```

Per-meal-slot view of today's macro targets vs actual, so an agent can recommend dinner that closes the protein gap.

### Substitute one food for a leaner alternative

```bash
kaloricke-tabulky-pp-cli food substitutes jablko --by energy --limit 5
```

Ranks foods by Euclidean distance on energy density; useful for swapping snacks within a calorie ceiling.

### Coach-grade weight report

```bash
kaloricke-tabulky-pp-cli weight regression --days 90 --target-kg 75 --json
```

Linear regression slope, R^2, and days-to-target — the chart math the web UI hides.

## Auth Setup

Authenticate with `auth login --email <you>` (password read from stdin, MD5-hashed before sending, then discarded). The cookie session is stored at `~/.config/kaloricke-tabulky/session`. To avoid a password on disk, use `auth login --chrome` to import the session cookies directly from a logged-in Chrome profile. Run `auth refresh` to extend the session.

Run `kaloricke-tabulky-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  kaloricke-tabulky-pp-cli achievements --agent --select id,name,status
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

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
kaloricke-tabulky-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
kaloricke-tabulky-pp-cli feedback --stdin < notes.txt
kaloricke-tabulky-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.kaloricke-tabulky-pp-cli/feedback.jsonl`. They are never POSTed unless `KALORICKE_TABULKY_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `KALORICKE_TABULKY_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
kaloricke-tabulky-pp-cli profile save briefing --json
kaloricke-tabulky-pp-cli --profile briefing achievements
kaloricke-tabulky-pp-cli profile list --json
kaloricke-tabulky-pp-cli profile show briefing
kaloricke-tabulky-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `kaloricke-tabulky-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add kaloricke-tabulky-pp-mcp -- kaloricke-tabulky-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which kaloricke-tabulky-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   kaloricke-tabulky-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `kaloricke-tabulky-pp-cli <command> --help`.
