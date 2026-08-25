# Woolworths CLI

**Every Woolworths catalogue, specials, store and trolley surface, plus a local price history that can tell a genuine half-price from an inflated was-price.**

Woolworths shows you today's price and a SAVE badge, and the badge is exactly what shoppers say they cannot trust. This CLI keeps its own price history in SQLite, so real-special returns a verdict instead of a number, cycle forecasts when the next half-price window opens, and swap ranks alternatives by unit price normalised across measure bases. It needs no API key, no login for the catalogue, and no headless browser. It is read-only apart from adding to a guest trolley.

## Install

The recommended path installs both the `woolworths-pp-cli` binary and the `pp-woolworths` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install woolworths
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install woolworths --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install woolworths --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install woolworths --agent claude-code
npx -y @mvanhorn/printing-press-library install woolworths --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/woolworths/cmd/woolworths-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/woolworths-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install woolworths --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-woolworths --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-woolworths --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install woolworths --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
woolworths-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/woolworths-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/woolworths/cmd/woolworths-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "woolworths": {
      "command": "woolworths-pp-mcp"
    }
  }
}
```

</details>

## Authentication

The catalogue, specials, stores and guest trolley need no account at all - only browser-shaped headers and a cookie jar the CLI warms automatically on first call. Personal surfaces (saved lists, past shops) sit behind Auth0 universal login with MFA, so there is no username/password flow to automate. On macOS and Linux, 'auth login --chrome' imports an existing logged-in Chrome session. On Windows that import is unavailable (the underlying cookie reader does not support it), so saved lists and past shops cannot be used there; the rest of the CLI is unaffected.

## Quick Start

```bash
# Confirms the cookie-warm transport reaches the API before anything else
woolworths-pp-cli doctor

# The catalogue works immediately with no key and no login
woolworths-pp-cli products search --term "tim tam" --page-size 5

# Unit-price ranking needs no history, so it is useful from the very first run
woolworths-pp-cli swap "olive oil" --limit 3 --max-scan-pages 1

# Records the first specials snapshot; run it again later to get a real diff
woolworths-pp-cli specials-diff --refresh

# Records observations and reports NO-HISTORY until enough accumulate
woolworths-pp-cli real-special "tim tam" --limit 2

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local history that compounds
- **`real-special`** — Tells you whether an advertised special is genuinely cheap or just a lower number next to an inflated was-price.

  _Reach for this instead of reading the SAVE badge: it returns one decided verdict per product rather than raw prices you have to judge yourself._

  ```bash
  woolworths-pp-cli real-special "tim tam" --limit 2 --agent
  ```
- **`cycle`** — Estimates when a product's next half-price window is likely to open, from its own recorded discount episodes.

  _Use this to decide buy-one-or-buy-six on a stockpile item, instead of guessing whether the deal returns next month._

  ```bash
  woolworths-pp-cli cycle 6073909 --agent
  ```
- **`specials-diff`** — Shows what entered and left each specials group since your last sync, with how long since each entrant was last discounted.

  _Reach for this on catalogue-rollover day to see only what actually changed rather than re-reading the whole specials list._

  ```bash
  woolworths-pp-cli specials-diff --limit 5 --agent
  ```

### Unit-price intelligence
- **`swap`** — Ranks in-stock alternatives by unit price normalised across different measure bases, so per-100g and per-1kg tiles compare correctly.

  _Pick this when the question is which product is genuinely cheaper per unit, not which has the lowest shelf price._

  ```bash
  woolworths-pp-cli swap "olive oil" --limit 3 --max-scan-pages 1 --agent
  ```
- **`multibuy`** — Works out what a multi-buy offer actually costs per unit at the quantity it demands, versus buying singly or buying a larger pack.

  _Use this before accepting a 2-for-$9 style offer, since a bigger single pack is often cheaper per unit._

  ```bash
  woolworths-pp-cli multibuy chocolate --limit 3 --max-scan-pages 1 --agent
  ```

### Agent-native plumbing
- **`basket`** — Prices a whole shopping list, showing which lines moved since the last time you ran it and what it did to the total.

  _Use this to turn a plain-text list into a costed, decided answer in one call instead of dozens of searches._

  ```bash
  woolworths-pp-cli basket ./groceries.txt --record=false --agent
  ```

## Recipes

### Check a special before you buy

```bash
woolworths-pp-cli real-special "tim tam" --agent
```

Returns a verdict per matching product rather than a price you have to interpret.

### Narrow a large search for an agent

```bash
woolworths-pp-cli products search --term milk --agent --select Products.Products.Name,Products.Products.Price,Products.Products.CupString
```

A bare search returns hundreds of KB; selecting three dotted paths keeps the response small enough to reason over.

### Find a cheaper equivalent by real unit price

```bash
woolworths-pp-cli swap "olive oil" --limit 10 --agent
```

Normalises across per-100g and per-1L tiles so the ranking is honest.

### See what changed on catalogue rollover

```bash
woolworths-pp-cli specials-diff --agent
```

Only the entrants and departures since your last sync, not the whole specials list.

### Cost this week's list against last week's

```bash
woolworths-pp-cli basket ./groceries.txt --record=false --agent
```

Per-line movement plus a basket total compared with the previous run.

## Usage

Run `woolworths-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `WOOLWORTHS_CONFIG_DIR`, `WOOLWORTHS_DATA_DIR`, `WOOLWORTHS_STATE_DIR`, or `WOOLWORTHS_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `WOOLWORTHS_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export WOOLWORTHS_HOME=/srv/woolworths
woolworths-pp-cli doctor
```

Under `WOOLWORTHS_HOME=/srv/woolworths`, the four dirs resolve to `/srv/woolworths/config`, `/srv/woolworths/data`, `/srv/woolworths/state`, and `/srv/woolworths/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "woolworths": {
      "command": "woolworths-pp-mcp",
      "env": {
        "WOOLWORTHS_HOME": "/srv/woolworths"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `WOOLWORTHS_DATA_DIR` overrides an explicit `--home` for that kind. Use `WOOLWORTHS_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `WOOLWORTHS_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `woolworths-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### categories

Browse the department tree and specials groups

- **`woolworths-pp-cli categories browse`** - Browse or page one category or specials group by node id
- **`woolworths-pp-cli categories list`** - Full department tree including the five specials groups and their product counts

### pastshops

Past shops / order history (requires an imported Chrome session)

- **`woolworths-pp-cli pastshops`** - Previous shops recorded against the account

### products

Search and inspect the Woolworths product catalogue

- **`woolworths-pp-cli products batch`** - Fetch several products in one call by comma-separated stockcodes
- **`woolworths-pp-cli products count`** - Cheap result counts for a term - products, specials, recipes - without fetching tiles
- **`woolworths-pp-cli products detail`** - Full product record including nutrition, variants and country of origin
- **`woolworths-pp-cli products schemaorg`** - Product as schema.org JSON-LD; an independent path from products detail
- **`woolworths-pp-cli products search`** - Search products by term, with optional specials-only filter and sort
- **`woolworths-pp-cli products suggestions`** - Ranked search suggestions and autocorrect for a partial term

### savedlists

Saved shopping lists (requires an imported Chrome session)

- **`woolworths-pp-cli savedlists get`** - One saved list including its products and free-text lines
- **`woolworths-pp-cli savedlists list`** - All saved shopping lists with product and free-text counts

### settings

Site configuration and feature flags

- **`woolworths-pp-cli settings bootstrap`** - App bootstrap config including current site version
- **`woolworths-pp-cli settings list`** - Site settings and feature flags

### stores

Find Woolworths stores, trading hours and facilities

- **`woolworths-pp-cli stores`** - Find stores near a postcode or coordinate, with trading hours and facilities

### trolley

Read and build the trolley; works anonymously as a guest cart

- **`woolworths-pp-cli trolley add`** - Add a product to the trolley by stockcode
- **`woolworths-pp-cli trolley get`** - Current trolley contents


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`woolworths-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`woolworths-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`woolworths-pp-cli learnings list`** - Inspect taught rows
- **`woolworths-pp-cli learnings forget <query>`** - Undo a teach
- **`woolworths-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`woolworths-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`woolworths-pp-cli teach-pattern`** - Install a query/resource template up front
- **`woolworths-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `WOOLWORTHS_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `woolworths-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
woolworths-pp-cli categories list

# JSON for scripting and agents
woolworths-pp-cli categories list --json
# Filter to specific fields by name
woolworths-pp-cli categories list --json --select <field>[,<field>...]

# Dry run — show the request without sending
woolworths-pp-cli categories list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
woolworths-pp-cli categories list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Explicit confirmation** - `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
woolworths-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `woolworths-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is ``; `--home`, `WOOLWORTHS_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `woolworths-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **The first command on a new machine takes about 13 seconds** — Expected. The client warms an Akamai cookie jar once per profile; later commands run in about a second. It is not a fault and needs no action.
- **HTTP 403 with an Access Denied HTML body** — The request went out without browser-shaped headers. Upgrade the CLI rather than setting a custom user agent.
- **real-special or cycle reports NO-HISTORY or an empty forecast** — Expected before history accumulates. Re-run 'real-special <term>' on the products you care about over several weeks; each run records an observation.
- **'sync' reports that a resource failed** — Only 'settings' syncs blind on this API. Every other list endpoint needs an argument, so price history is built by 'real-special' and 'specials-diff --refresh', not by 'sync'.
- **Saved lists or past shops return empty or 401** — The Auth0 session has expired. Log in to Woolworths in Chrome, then re-run 'auth login --chrome'.
- **A page size above 36 returns HTTP 400** — Woolworths hard-caps page size at 36. Use --limit and let the client paginate.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

TLS certificates are verified by default. For a trusted development or self-signed endpoint only, pass `--insecure` for one invocation, set `WOOLWORTHS_SKIP_TLS_VERIFY=true` for the current environment, or set `skip_tls_verify = true` in the config file for a persistent override.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**aus_grocery_price_database**](https://github.com/tjhowse/aus_grocery_price_database) — Go (40 stars)
- [**coles-woolworths-mcp-server**](https://github.com/hung-ngm/coles-woolworths-mcp-server) — Python (35 stars)
- [**Woolies**](https://github.com/tascord/Woolies) — JavaScript (33 stars)
- [**au-supermarket-apis**](https://github.com/drkno/au-supermarket-apis) — YAML (31 stars)
- [**coles_vs_woolies**](https://github.com/MattTimms/coles_vs_woolies) — Python (23 stars)
- [**Woolworths-mcp**](https://github.com/elijah-g/Woolworths-mcp) — TypeScript (14 stars)
- [**grocery-scraper**](https://github.com/Grocermatic/grocery-scraper) — TypeScript (6 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
