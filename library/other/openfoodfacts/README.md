# Open Food Facts CLI

**Every Open Food Facts read endpoint, plus a local calorie/macro diary, product compare, and offline search no other Open Food Facts tool has — free, no account, pipeable to jq.**

A general-purpose Go CLI for the Open Food Facts database: barcode lookup, filtered search, Nutri-Score/NOVA/Eco-Score, additives and allergens, with a built-in mandatory User-Agent and an adaptive rate limiter so you don't get throttled or banned. On top of the API it adds what the consumer apps charge for: a local SQLite food diary (diary), healthier-alternative lookup (swap), product comparison (compare), and offline ranking/search that bypass the rate limit entirely.

Learn more at [Open Food Facts](https://slack.openfoodfacts.org/).

Created by [@quickpre](https://github.com/quickpre) (Pietro Cimmaruta).

## Install

The recommended path installs both the `openfoodfacts-pp-cli` binary and the `pp-openfoodfacts` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install openfoodfacts
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install openfoodfacts --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install openfoodfacts --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install openfoodfacts --agent claude-code
npx -y @mvanhorn/printing-press-library install openfoodfacts --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/openfoodfacts-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install openfoodfacts --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-openfoodfacts --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-openfoodfacts --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install openfoodfacts --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/openfoodfacts-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `OPENFOODFACTS_USER_AGENT_AUTH` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "openfoodfacts": {
      "command": "openfoodfacts-pp-mcp",
      "env": {
        "OPENFOODFACTS_USER_AGENT_AUTH": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

No API key, no account, no OAuth. Open Food Facts read endpoints are open data. The only requirement is a well-formed User-Agent, which the CLI sets automatically and lets you override with --user-agent or OPENFOODFACTS_USER_AGENT.

## Quick Start

```bash
# Scan-and-judge: kcal, Nutri-Score, NOVA, additives, allergens for one barcode.
openfoodfacts-pp-cli product 3017620422003 --compact

# Find products by category tag against the live API, agent-shaped JSON.
openfoodfacts-pp-cli find --categories-tags en:yogurts --agent

# Two products side-by-side on scores and per-100g nutriments.
openfoodfacts-pp-cli compare 3017620422003 7622210449283

# Log a food to the local diary.
openfoodfacts-pp-cli diary add 3017620422003 --servings 1.5

# Running kcal/macro totals vs your daily goal.
openfoodfacts-pp-cli diary today

```

## Unique Features

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

## Usage

Run `openfoodfacts-pp-cli --help` for the full command reference and flag list.

## Commands

### attribute-groups

Manage attribute groups

- **`openfoodfacts-pp-cli attribute-groups`** - Attributes are at the heart of personal search.
They score the products according to different criterias,
which could then be matched to a user's preferences.

This API helps you list attributes and display them in your application,
for the user to choose the importance of each criteria.

note: `/api/v2/attribute_groups_{lc}` is also a valid route, but consider it deprecated

### cgi

Manage cgi

- **`openfoodfacts-pp-cli cgi get-ingredients-pl`** - Open Food Facts uses optical character recognition (OCR) to retrieve nutritional data and other information from the product labels.
- **`openfoodfacts-pp-cli cgi get-nutrients-pl`** - Used to display the nutrition facts table of a product, or to display a form to input those nutrition facts.
- **`openfoodfacts-pp-cli cgi get-product-image-crop-pl`** - Although we recommend rotating photos manually and uploading a new version of the image,
the OFF API allows you to make api calls to automate this process.
You can rotate existing photos by setting the angle to 90º, 180º, or 270º clockwise.
- **`openfoodfacts-pp-cli cgi get-product-image-upload-pl`** - Photos are source and proof of data.
The first photo uploaded for a product is
auto-selected as the product’s “front” photo.'
- **`openfoodfacts-pp-cli cgi get-session-pl`** - Retrieve session cookie for writing operations.
- **`openfoodfacts-pp-cli cgi get-suggest-pl`** - For example , Dave is looking for packaging_shapes that contain the term "fe",
all packaging_shapes containing "fe" will be returned.
This is useful if you have a search in your application,
for a specific product field.
- **`openfoodfacts-pp-cli cgi post-product-image-crop-pl`** - Cropping is only relevant for editing existing products.
You cannot crop an image the first time you upload it to the system.
- **`openfoodfacts-pp-cli cgi post-product-image-unselect-pl`** - This endpoint allows the user to unselect a photo for a product.
The user must provide the product code and the image ID to unselect.
- **`openfoodfacts-pp-cli cgi post-product-jqm2-pl`** - This updates a product.

Note: If the barcode exists then you will be editing the existing product,
However if it doesn''t you will be creating a new product with that unique barcode,
and adding properties to the product.

### find

Manage find

- **`openfoodfacts-pp-cli find`** - Search request allows you to get products that match your search criteria.

It allows you create many custom APIs for your use case.

If a search query parameter has multiple values, separate them with commas (`,`).
When filtering with a language-specific parameter (`fr`, `de`, `en`, etc.),
specify the language code in the parameter name (for example: `categories_tags_en`).

**Important:** search API v2 does not support full text request (search_term),
you have to use [search API v1](https://wiki.openfoodfacts.org/API/Read/Search) for that.
Upcoming [search-a-licious project](https://github.com/openfoodfacts/search-a-licious) will fix that.

### Limiting results

You can limit the size of returned objects thanks to the `fields` object (see below).

Example: `fields=code,product_name,brands,attribute_groups`

Please use it as much as possible to avoid overloading the servers.

The search endpoint uses pagination: see `page` and `page_size` parameters.

**Beware:** `page_count` is the number of products returned in the current page, not the total number of pages.

### Conditions on tags

All `_tags`` parameters accepts either:

* a single value
* or a comma-separated list of values (doing a AND)
* or a pipe separated list of values (doing a OR)

You can exclude terms by using a "-" prefix.

For taxonomized entries, you might either use the tag id (recommended),
or a known synonym (without language prefix)

* `labels_tags=en:organic,en:fair-trade` find items that are fair-trade AND organic
* `labels_tags=en:organic|en:fair-trade` find items that are fair-trade OR organic
* `labels_tags=en:organic,en:-fair-trade` find items that are organic BUT NOT fair-trade


### Conditions on nutriments

To get a list of nutrients:

You can either query on nutrient per 100g (`_100g` suffix)
or per serving (`serving` suffix).

You can also add `_prepared_`
to get the nutrients in the prepared product instead of as sold.

You can add a comparison operator and value to the parameter name
to get products with nutrient above or below a value.
If you use a parameter value it exactly match it.

* `energy-kj_100g<200` products where energy in kj for 100g is less than 200kj
* `sugars_serving>10` products where sugar per serving is greater than 10g
* `saturated-fat_100g=1` products where saturated fat per 100g is exactly 10g
* `salt_prepared_serving<0.1` products where salt per serving for prepared product is less than 0.1g

### Combining filters and pagination (examples)

1. Breakfast cereals with Nutri-Score A or B, with explicit pagination:
   `/api/v2/search?categories_tags_en=breakfast-cereals&nutrition_grades_tags=a|b&page=2&page_size=20&fields=code,product_name,nutrition_grades`

2. Multiple constraints in one query (category + nutrients + sorting + compact fields):
   `/api/v2/search?categories_tags_en=orange-juices&sugars_100g%3C8&salt_100g%3C0.2&sort_by=last_modified_t&page=1&page_size=24&fields=code,product_name,nutriments`

3. Include/exclude tags together:
   `/api/v2/search?labels_tags=en:organic,en:-fair-trade&page=1&page_size=24&fields=code,product_name,labels_tags`

### Response structure at a glance

A successful response contains:

* `count`: total number of matching products
* `page`: current page number
* `page_size`: requested number of products per page
* `page_count`: number of products actually returned in this page
* `skip`: number of products skipped before this page
* `products`: array of products for the current page

Total page count can be computed with:
`Math.floor((count - 1) / page_size) + 1`

### More references

See also [wiki page](https://wiki.openfoodfacts.org/Open_Food_Facts_Search_API_Version_2)

### preferences

Manage preferences

- **`openfoodfacts-pp-cli preferences`** - This endpoint retrieves the weights corresponding to attribute preferences
for computing personal product recommendations. The weights are used to
personalize the product recommendations based on user preferences.

### product

Endpoints for managing product data and information.

- **`openfoodfacts-pp-cli product <code>`** - Fetches product details by its unique barcode. 
Can return all product details or specific fields like knowledge panels.

Use the `blame` parameter to include information about who last modified each field of the product.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
openfoodfacts-pp-cli attribute-groups

# JSON for scripting and agents
openfoodfacts-pp-cli attribute-groups --json

# Filter to specific fields
openfoodfacts-pp-cli attribute-groups --json --select id,name,status

# Dry run — show the request without sending
openfoodfacts-pp-cli attribute-groups --dry-run

# Agent mode — JSON + compact + no prompts in one flag
openfoodfacts-pp-cli attribute-groups --agent
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
openfoodfacts-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/open-food-facts-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `OPENFOODFACTS_USER_AGENT_AUTH` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `openfoodfacts-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `openfoodfacts-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $OPENFOODFACTS_USER_AGENT_AUTH`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **HTTP 429 or 503 from the API** — You hit the per-IP rate limit (15/min reads, 10/min search). The CLI backs off automatically; for bulk work run `sync` once then query with `--local` / `rank` / `sql` offline.
- **Requests rejected or empty as a suspected bot** — Set a contactable User-Agent: `export OPENFOODFACTS_USER_AGENT='myapp/1.0 (me@example.com)'`. Run `doctor` to confirm it's set.
- **Product has no Nutri-Score or missing nutriments** — Open Food Facts data is crowd-sourced; many products are partial. Nutri-Score needs a category. The CLI emits null fields rather than failing — check `product <code> --fields nutriments,categories`.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**openfoodfacts-nodejs**](https://github.com/openfoodfacts/openfoodfacts-nodejs) — TypeScript
- [**openfoodfacts-python**](https://github.com/openfoodfacts/openfoodfacts-python) — Python
- [**JagjeevanAK/OpenFoodFacts-MCP**](https://github.com/JagjeevanAK/OpenFoodFacts-MCP) — TypeScript
- [**nagarjun226/food-tracker-mcp**](https://github.com/nagarjun226/food-tracker-mcp) — Python
- [**openfoodfacts/labelr**](https://github.com/openfoodfacts/labelr) — Python
- [**angristan/openfoodfacts-api-c**](https://github.com/angristan/openfoodfacts-api-c) — C

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
