---
name: pp-openfoodfacts
description: "Every Open Food Facts read endpoint, plus a local calorie/macro diary, product compare Trigger phrases: `look up this barcode`, `nutrition facts for`, `check the Nutri-Score of`, `log this food to my diary`, `find a healthier alternative to`, `compare these two products`, `use openfoodfacts`, `run openfoodfacts`."
author: "Pietro Cimmaruta"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - openfoodfacts-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/other/openfoodfacts/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Open Food Facts — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `openfoodfacts-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install openfoodfacts --cli-only
   ```
2. Verify: `openfoodfacts-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

A general-purpose Go CLI for the Open Food Facts database: barcode lookup, filtered search, Nutri-Score/NOVA/Eco-Score, additives and allergens, with a built-in mandatory User-Agent and an adaptive rate limiter so you don't get throttled or banned. On top of the API it adds what the consumer apps charge for: a local SQLite food diary (diary), healthier-alternative lookup (swap), product comparison (compare), and offline ranking/search that bypass the rate limit entirely.

## When to Use This CLI

Use this CLI when an agent or script needs nutrition facts for a packaged food without an API key, account, or paid tier — barcode lookups, name search with nutrient filters, Nutri-Score/NOVA/additive/allergen data, or a local calorie/macro diary. It is the right choice over a raw HTTP call because it ships rate-limit safety, a mandatory User-Agent, offline search, and aggregation primitives (compare, recipe, diary) the bare API lacks.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`diary add`** — Log foods by barcode and track running kcal/protein/fat/carbs against a daily goal — free, offline, scriptable.

  _Reach for this when an agent needs to track or total a user's intake without an account, subscription, or network round-trip per query._

  ```bash
  openfoodfacts-pp-cli diary add 3017620422003 --servings 1.5 && openfoodfacts-pp-cli diary today --agent
  ```
- **`diary since`** — Per-day macro totals and averages across a date range from the local diary.

  _Use when summarizing a week of intake or feeding macro trends into a planner._

  ```bash
  openfoodfacts-pp-cli diary since 2026-06-08 --agent
  ```
- **`allergens set`** — Store your allergens once, then check any product (and exit non-zero on) one whose allergens or traces match.

  _Use as a guardrail in any flow that must refuse foods matching a fixed allergen list._

  ```bash
  openfoodfacts-pp-cli allergens set milk,gluten && openfoodfacts-pp-cli allergens check 3017620422003
  ```
- **`budget`** — Search constrained to the macros left in today's goal after what you've logged.

  _Reach for this to suggest foods that still fit a user's remaining daily budget._

  ```bash
  openfoodfacts-pp-cli budget "snack" --agent
  ```

### Decision tools
- **`compare`** — Put two or more products side-by-side on per-100g nutriments, Nutri-Score, NOVA, Eco-Score, and sugar.

  _Reach for this on store-brand-vs-name-brand or any 'which is healthier' decision._

  ```bash
  openfoodfacts-pp-cli compare 3017620422003 7622210449283 --agent
  ```
- **`swap`** — Given a product, find better-scoring items in the same category, ranked by Nutri-Score/NOVA.

  _Use when a user wants a concrete substitute, not just a verdict on what they're holding._

  ```bash
  openfoodfacts-pp-cli swap 3017620422003 --max-nova 3 --agent
  ```
- **`recipe`** — Sum per-serving nutriments across multiple products into one recipe block plus a per-serving block.

  _Reach for this when an LLM meal-planner needs totaled nutrition for a composed dish._

  ```bash
  openfoodfacts-pp-cli recipe 3017620422003 7622210449283 --servings 4 --agent
  ```

### Reachability mitigation
- **`rank`** — Rank synced products in a category by any nutriment, entirely from the local store.

  _Use for repeated comparisons across a category without burning the 10-req/min search budget._

  ```bash
  openfoodfacts-pp-cli rank --category breakfast-cereals --sort sugar --agent
  ```

## Command Reference

**attribute-groups** — Manage attribute groups

- `openfoodfacts-pp-cli attribute-groups` — Attributes are at the heart of personal search.

**cgi** — Manage cgi

- `openfoodfacts-pp-cli cgi get-ingredients-pl` — Open Food Facts uses optical character recognition (OCR)
- `openfoodfacts-pp-cli cgi get-nutrients-pl` — Used to display the nutrition facts table of a product, or to display a form to input those nutrition facts.
- `openfoodfacts-pp-cli cgi get-product-image-crop-pl` — Although we recommend rotating photos manually and uploading a new version of the image
- `openfoodfacts-pp-cli cgi get-product-image-upload-pl` — Photos are source and proof of data.
- `openfoodfacts-pp-cli cgi get-session-pl` — Retrieve session cookie for writing operations.
- `openfoodfacts-pp-cli cgi get-suggest-pl` — For example , Dave is looking for packaging_shapes that contain the term 'fe'
- `openfoodfacts-pp-cli cgi post-product-image-crop-pl` — Cropping is only relevant for editing existing products.
- `openfoodfacts-pp-cli cgi post-product-image-unselect-pl` — This endpoint allows the user to unselect a photo for a product.
- `openfoodfacts-pp-cli cgi post-product-jqm2-pl` — This updates a product.

**find** — Manage find

- `openfoodfacts-pp-cli find` — Search request allows you to get products that match your search criteria.

**preferences** — Manage preferences

- `openfoodfacts-pp-cli preferences` — This endpoint retrieves the weights corresponding to attribute preferences for computing personal product

**product** — Endpoints for managing product data and information.

- `openfoodfacts-pp-cli product <code>` — Fetches product details by its unique barcode. Can return all product details or specific fields like knowledge panels.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
openfoodfacts-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Aisle verdict with allergen gate

```bash
openfoodfacts-pp-cli allergens set milk,gluten && openfoodfacts-pp-cli allergens check 3017620422003
```

Store your allergens once, then `allergens check` flags a HIT and exits non-zero when the product matches — a scriptable buy/skip gate.

### Daily macro loop

```bash
openfoodfacts-pp-cli diary goal --kcal 2000 --protein 150 && openfoodfacts-pp-cli diary add 3017620422003 --servings 2 && openfoodfacts-pp-cli diary today --agent
```

Set a goal, log foods by barcode, and read running totals vs the goal — MyFitnessPal's core loop, free and offline.

### Healthier swap in a category

```bash
openfoodfacts-pp-cli swap 3017620422003 --max-nova 3 --agent
```

Find better-scoring products in the same category as a given barcode, ranked by Nutri-Score.

### Narrow a deep payload with --select

```bash
openfoodfacts-pp-cli find --categories-tags en:chocolate-biscuits --agent --select products.code,products.product_name,products.nutriscore_grade
```

Search responses carry tens of KB per page; dotted --select paths return only the fields an agent needs, keeping context small.

### Offline ranking without burning rate limit

```bash
openfoodfacts-pp-cli sync --resources find && openfoodfacts-pp-cli rank --sort sugar --agent
```

Sync a category to SQLite once, then rank by any nutriment offline — no repeated API calls, no throttling.

## Auth Setup

No API key, no account, no OAuth. Open Food Facts read endpoints are open data. The only requirement is a well-formed User-Agent, which the CLI sets automatically and lets you override with --user-agent or OPENFOODFACTS_USER_AGENT.

Run `openfoodfacts-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  openfoodfacts-pp-cli attribute-groups --agent --select id,name,status
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
openfoodfacts-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
openfoodfacts-pp-cli feedback --stdin < notes.txt
openfoodfacts-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/openfoodfacts-pp-cli/feedback.jsonl`. They are never POSTed unless `OPENFOODFACTS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `OPENFOODFACTS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
openfoodfacts-pp-cli profile save briefing --json
openfoodfacts-pp-cli --profile briefing attribute-groups
openfoodfacts-pp-cli profile list --json
openfoodfacts-pp-cli profile show briefing
openfoodfacts-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `openfoodfacts-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add openfoodfacts-pp-mcp -- openfoodfacts-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which openfoodfacts-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   openfoodfacts-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `openfoodfacts-pp-cli <command> --help`.
