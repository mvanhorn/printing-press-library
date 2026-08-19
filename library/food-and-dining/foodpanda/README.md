# foodpanda CLI

**Every foodpanda restaurant, menu and price in a local database — with cross-restaurant dish search, price history and fee comparison the app cannot do.**

foodpanda's API hands you one restaurant at a time and forgets everything the moment you close the tab. This CLI mirrors an entire area into SQLite, so you can ask questions foodpanda structurally cannot answer: which restaurant near me sells the cheapest biryani, what changed in this menu since last week, and how delivery fees really compare once service fee and minimum order are counted. Works across every market foodpanda runs, and needs no API key for catalog data.

Learn more at [foodpanda](https://disco.deliveryhero.io).

Created by [@qazmataz](https://github.com/qazmataz) (qazmataz).

## Install

The recommended path installs both the `foodpanda-pp-cli` binary and the `pp-foodpanda` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install foodpanda
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install foodpanda --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install foodpanda --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install foodpanda --agent claude-code
npx -y @mvanhorn/printing-press-library install foodpanda --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/foodpanda/cmd/foodpanda-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/foodpanda-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install foodpanda --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-foodpanda --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-foodpanda --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install foodpanda --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
foodpanda-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/foodpanda-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/foodpanda/cmd/foodpanda-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "foodpanda": {
      "command": "foodpanda-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Catalog browsing needs no credentials at all — vendor search, menus, prices, deals and reviews all work anonymously. Only your own account data (saved addresses, and the `home` command that depends on them) needs a session. Run `foodpanda-pp-cli auth login --chrome` to import the `token` cookie from a browser where you are already signed in; the CLI composes it into an Authorization header. There is no API key to request and nothing to paste.

## Quick Start

```bash
# confirm the browser header set is intact and the catalog host is reachable
foodpanda-pp-cli doctor --dry-run

# top-rated restaurants near central Lahore, straight from the live listing
foodpanda-pp-cli vendors list --latitude 31.5204 --longitude 74.3587 --sort rating_desc --limit 10

# mirror the area's vendors into SQLite; latitude, longitude and country are required because every foodpanda query is geo-scoped
foodpanda-pp-cli sync --resources vendors --global-param latitude=31.5204 --global-param longitude=74.3587 --global-param country=pk

# cross-restaurant dish --query search — the thing no foodpanda surface can do
foodpanda-pp-cli dish --query 'chicken biryani' --max-price 600

# compare true ordering cost, not just headline delivery fee
foodpanda-pp-cli fees --latitude 31.5204 --longitude 74.3587 --sort total

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`home`** — Rank every restaurant near your saved home address by what delivery actually costs you.

  _Reach for this when the question is 'what is cheapest to get delivered to me', not 'what is this one restaurant's fee'._

  ```bash
  foodpanda-pp-cli home --sort fee --limit 25 --agent
  ```
- **`dish`** — Find which nearby restaurant sells a specific dish cheapest, searching every synced menu at once.

  _Use this for item-level price hunting; use search when you want restaurants by name instead._

  ```bash
  foodpanda-pp-cli dish --query 'chicken biryani' --max-price 600 --agent
  ```
- **`menu-diff`** — Show what changed in a restaurant's menu and prices between two syncs.

  _Use this to catch price rises, removed items, or newly added deals over time._

  ```bash
  foodpanda-pp-cli menu-diff --vendor-code pk2v --since 7d --agent
  ```

### Competitive intelligence
- **`posture`** — Rank vendors by advertising and placement signals: CPC ad participation, promoted and premium status, and ranking score.

  _Use this for competitive analysis of who buys placement. It does not report merchant commission rates, which are not exposed in any consumer surface._

  ```bash
  foodpanda-pp-cli posture --latitude 31.5204 --longitude 74.3587 --ads-only --agent
  ```
- **`coverage`** — Determine which vendors actually deliver to an arbitrary point using each vendor's published delivery radius.

  _Use this before assuming a restaurant is orderable from an address you have not tried._

  ```bash
  foodpanda-pp-cli coverage --latitude 31.4820 --longitude 74.3430 --agent
  ```
- **`fees`** — Compare the full cost structure across an area: delivery fee, minimum order, service fee and VAT together.

  _Use this when headline delivery fee is misleading because service fee or minimum order dominates._

  ```bash
  foodpanda-pp-cli fees --latitude 24.8607 --longitude 67.0011 --sort total --agent
  ```

### Signal the site throws away
- **`digest`** — Split a restaurant's blended star rating into per-topic scores so food quality and delivery quality are separated.

  _Use this to tell 'the food is bad' apart from 'the delivery is bad' before trusting a rating._

  ```bash
  foodpanda-pp-cli digest --vendor-code pk2v --agent
  ```
- **`market-compare`** — Run the same query across every foodpanda market and compare vendor counts, ratings and fees side by side.

  _Use this for regional benchmarking; use vendors list when you only care about one city._

  ```bash
  foodpanda-pp-cli market-compare --query pizza --markets pk,sg,my --agent
  ```
- **`find`** — Search vendors live upstream and label how strongly each result actually matched the query.

  _Use this for live upstream search where match quality matters; use 'search' instead for offline full-text search over already-synced data._

  ```bash
  foodpanda-pp-cli find --query sushi --latitude 31.5204 --longitude 74.3587 --explain --agent
  ```

## Recipes

### Cheapest delivery near home

```bash
foodpanda-pp-cli home --sort fee --limit 20 --agent
```

Ranks every restaurant reaching your saved home address by real delivery cost, which the app never lets you sort by.

### Narrow a huge listing payload for an agent

```bash
foodpanda-pp-cli vendors list --latitude 31.5204 --longitude 74.3587 --limit 40 --agent --select code,name,rating,minimum_delivery_fee,minimum_order_amount
```

Vendor rows carry 80+ fields each; selecting five keeps the response small enough to reason over without burning context.

### Find the cheapest biryani in town

```bash
foodpanda-pp-cli dish --query biryani --max-price 700 --sort price --agent
```

Searches every synced menu at once and returns item-level matches with the restaurant that sells them.

### Track a menu for price rises

```bash
foodpanda-pp-cli menu-diff --vendor-code pk2v --since 14d --agent
```

Diffs two local snapshots to surface added, removed and repriced items over the last two weeks.

### See who is buying placement

```bash
foodpanda-pp-cli posture --latitude 24.8607 --longitude 67.0011 --ads-only --sort points --agent
```

Ranks vendors by CPC ad participation and premium placement signals for competitive analysis.

## Usage

Run `foodpanda-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `FOODPANDA_CONFIG_DIR`, `FOODPANDA_DATA_DIR`, `FOODPANDA_STATE_DIR`, or `FOODPANDA_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `FOODPANDA_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

### Credentials and freshness

| Variable | Effect |
|----------|--------|
| `FOODPANDA_TOKEN` | Session credential for the customer-scoped commands (`home`, `addresses`). Accepts the bare `token` JWT or a full cookie-jar string. This is the headless equivalent of `auth login --chrome`, which needs a desktop Chrome profile — use it in CI, containers and agent sandboxes. |
| `FOODPANDA_AUTO_REFRESH` | Set to `1` to re-sync stale local data before a command reads it. Off by default: these commands read a local mirror, and running one should not reach the network unless you asked. Equivalent to `--refresh-stale`. |
| `FOODPANDA_STALE_AFTER` | Staleness threshold as a Go duration (default `6h`). Matches what `doctor` reports as stale. |

Refresh only re-runs resources that recorded the scope they synced with, because foodpanda's listing endpoints are geo-scoped — refreshing without the original coordinates would mirror a different place than you synced. Resources synced before this was recorded are reported, not silently skipped.

For containers and agent sandboxes, prefer a single relocated root:

```bash
export FOODPANDA_HOME=/srv/foodpanda
foodpanda-pp-cli doctor
```

Under `FOODPANDA_HOME=/srv/foodpanda`, the four dirs resolve to `/srv/foodpanda/config`, `/srv/foodpanda/data`, `/srv/foodpanda/state`, and `/srv/foodpanda/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "foodpanda": {
      "command": "foodpanda-pp-mcp",
      "env": {
        "FOODPANDA_HOME": "/srv/foodpanda"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `FOODPANDA_DATA_DIR` overrides an explicit `--home` for that kind. Use `FOODPANDA_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `FOODPANDA_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `foodpanda-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### menu

Full vendor detail including nested menus, products and prices

- **`foodpanda-pp-cli menu <vendor_code>`** - Fetch one vendor with full menu, deals and delivery conditions

### reviews

Customer reviews with per-topic rating breakdown

- **`foodpanda-pp-cli reviews <vendor_code>`** - List reviews for a vendor, newest first

### vendors

Browse and search foodpanda vendors near a location

- **`foodpanda-pp-cli vendors list`** - List vendors near a coordinate, with filters and sorting
- **`foodpanda-pp-cli vendors search`** - Full-text search vendors and dishes near a coordinate


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`foodpanda-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`foodpanda-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`foodpanda-pp-cli learnings list`** - Inspect taught rows
- **`foodpanda-pp-cli learnings forget <query>`** - Undo a teach
- **`foodpanda-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`foodpanda-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`foodpanda-pp-cli teach-pattern`** - Install a query/resource template up front
- **`foodpanda-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `FOODPANDA_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `foodpanda-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
foodpanda-pp-cli menu mock-value

# JSON for scripting and agents
foodpanda-pp-cli menu mock-value --json

# Filter to specific fields
foodpanda-pp-cli menu mock-value --json --select id,name,status

# Dry run — show the request without sending
foodpanda-pp-cli menu mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
foodpanda-pp-cli menu mock-value --agent
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
foodpanda-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `foodpanda-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/foodpanda-pp-cli/config.toml`; `--home`, `FOODPANDA_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `foodpanda-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Every request returns HTTP 403 with an 'Access to this page has been denied' page** — PerimeterX is rejecting the request shape. Run `foodpanda-pp-cli doctor` to confirm the browser header set is being sent; a stripped User-Agent is the usual cause.
- **Search returns results that have nothing to do with the query** — foodpanda's search is fuzzy and never returns empty. Use `find --explain` to see match confidence and filter out weak rows.
- **A vendor code returns 404 on the menu command** — Vendor codes are market-scoped. Pass the same --country you used for the listing; a Pakistani code will 404 against the Singapore backend.
- **`home` reports no saved address** — Run `foodpanda-pp-cli auth login --chrome` from a browser profile signed in to foodpanda, then retry.
- **Offline commands return nothing after a fresh install** — Run `foodpanda-pp-cli sync --resources vendors --global-param latitude=31.5204 --global-param longitude=74.3587` first; dish --query and menu-diff also populate menu snapshots as they run.
- **Grocery or pandamart products are missing** — Darkstores are listed but their product catalog is a separate q-commerce surface this CLI does not cover. Restaurant menus are unaffected.
- **`search` fails with 'Invalid or non-existent country: null'** — The framework `search` defaults to --data-source auto and tries the live endpoint, which needs coordinates. Use `search <term> --data-source local` for offline FTS over synced data, or `find <term> --latitude <lat> --longitude <lng>` for live search.
- **`sync` reports 'Invalid or non-existent country: null'** — Pass the geo scope explicitly: `sync --resources vendors --global-param latitude=<lat> --global-param longitude=<lng> --global-param country=pk`.
- **Delivery fees all show the same value (e.g. 99 everywhere)** — That is foodpanda's flat listing floor, not a real price. Run `auth login --chrome` so `home` can resolve true per-vendor fees; rows then report fee_source: vendor-detail.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

TLS certificates are verified by default. For a trusted development or self-signed endpoint only, pass `--insecure` for one invocation, set `FOODPANDA_SKIP_TLS_VERIFY=true` for the current environment, or set `skip_tls_verify = true` in the config file for a persistent override.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**oloviz**](https://github.com/iCHAIT/oloviz) — Python
- [**apify-foodpanda-restaurants**](https://github.com/Nowi5/apify-foodpanda-restaurants) — JavaScript
- [**WebScrapingUsingPython_FoodPanda_And_HTMLTable**](https://github.com/vedingal/WebScrapingUsingPython_FoodPanda_And_HTMLTable) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
