# REWE CLI

**Order REWE groceries for pickup from the terminal or an agent - offers, basket, earliest slot and recipes, with a session that renews itself.**

rewe-pp-cli replays the logged-in REWE web shop: search your market's catalogue, build the basket by text, pick the earliest pickup slot and place the order with pay-at-pickup behind an explicit --confirm. offers, deals mine and recipe basket turn the shop into a weekly planner.

Learn more at [REWE](https://www.rewe.de).

Created by [@jimpresting](https://github.com/jimpresting) (JimPresting).

## Install

The recommended path installs both the `rewe-pp-cli` binary and the `pp-rewe` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install rewe
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install rewe --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install rewe --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install rewe --agent claude-code
npx -y @mvanhorn/printing-press-library install rewe --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/rewe-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install rewe --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-rewe --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-rewe --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install rewe --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses the REWE session file — import it first if you haven't (log in to rewe.de once in a browser, then):

```bash
rewe-pp-cli auth import --file cookies.json
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/rewe-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS, `%APPDATA%\Claude\claude_desktop_config.json` on Windows):

```json
{
  "mcpServers": {
    "rewe": {
      "command": "rewe-pp-mcp"
    }
  }
}
```

</details>

### Remote MCP over HTTP

`rewe-pp-mcp --transport http` (or `PP_MCP_TRANSPORT=http`) serves streamable HTTP at `http://127.0.0.1:7777/mcp`. It refuses to start until `REWE_MCP_HTTP_TOKEN` is set to at least 32 characters, and every request must send `Authorization: Bearer <token>`. The command below prints 64 hex characters and meets that minimum. Plain HTTP is allowed only on a loopback `--addr`; any other bind, including `:7777` (every interface), also needs `--tls-cert` and `--tls-key`. Requests with no `Origin` header are accepted, as are loopback origins (`localhost`, `127.0.0.1`, or `[::1]`, any port). Any other `Origin` is refused unless it is listed in `REWE_MCP_HTTP_ALLOWED_ORIGINS`, a comma-separated list of `scheme://host[:port]` values with no path. That list is not a CORS policy; the server does not emit CORS headers.

```bash
export REWE_MCP_HTTP_TOKEN="$(openssl rand -hex 32)"
rewe-pp-mcp --transport http
```

Set `REWE_MCP_READ_ONLY=1` (exactly `1`) to drop mirrored commands that are not marked read-only and make the raw execute tool refuse state-changing calls. Commands marked local-write, which write only this CLI's own learn store, stay available.

## Authentication

Log in once to rewe.de in a browser (REWE requires a captcha and two-factor login), then run rewe-pp-cli auth import --cdp http://127.0.0.1:9222 or --file cookies.json. The CLI keeps the shop session alive with REWE's 30-day single-sign-on cookie (auth renew, also done automatically) and says when a fresh browser login is needed.

## Quick Start

```bash
# find a pickup market near you
rewe-pp-cli markets pickup 50667

# check the imported session
rewe-pp-cli auth session --json

# prices of your market
rewe-pp-cli products search --query hafermilch --per-page 5

# add by text
rewe-pp-cli basket add hafermilch --qty 2

# earliest pickup
rewe-pp-cli slots earliest

# preview before ordering
rewe-pp-cli order place --slot earliest --phone +4915112345678 --dry-run

```

## Unique Features

### Shopping intelligence
- **`offers`** — Current offers of your market (and next week's once REWE publishes them) as rows with title, price and category.

  _Use it to answer what is on offer or to propose deals._

  ```bash
  rewe-pp-cli offers --search kaffee --json
  ```
- **`deals mine`** — Offers that match products you bought before or saved as favourites.

  _Basis for proactive offer suggestions._

  ```bash
  rewe-pp-cli deals mine --json
  ```
- **`recipe basket`** — Map a REWE recipe's ingredients to shop products for N servings and add the recommended ones.

  _Recipe planning: pick recipes, get one basket._

  ```bash
  rewe-pp-cli recipe basket gemuese-lasagne --servings 4 --json
  ```
- **`price history`** — Price changes of a product seen in searches and offers over time.

  _Tell whether an offer is really cheap._

  ```bash
  rewe-pp-cli price history 7381513 --json
  ```

### Order flow
- **`slots earliest`** — The earliest free pickup slot with its fee, optionally reserved right away.

  _Propose the earliest pickup before ordering._

  ```bash
  rewe-pp-cli slots earliest --json
  ```
- **`order place`** — Place the pickup order: refresh basket, reserve slot, pay in market, submit - only with --confirm.

  _The only command that spends money; agents must show the preview and get a yes first._

  ```bash
  rewe-pp-cli order place --slot earliest --phone +4915112345678 --dry-run
  ```
- **`basket add`** — Add products by search text and quantity without knowing listing ids.

  _Natural-language orders map straight onto this._

  ```bash
  rewe-pp-cli basket add hafermilch --qty 2 --json
  ```
- **`list add`** — Turn a shopping list (one item per line, optional quantity) into basket items.

  _Feed a transcribed voice note directly._

  ```bash
  rewe-pp-cli list add --file einkauf.txt --dry-run --json
  ```

## Recipes

### Offers on coffee this week

```bash
rewe-pp-cli offers --search kaffee --agent --select title,price,category
```

Compact rows for an agent reply.

### Recipe to basket

```bash
rewe-pp-cli recipe basket gemuese-lasagne --servings 4 --json
```

Recommended products per ingredient.

### Earliest slot

```bash
rewe-pp-cli slots earliest --json
```

Slot id, start, end and fee.

## Usage

Run `rewe-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `REWE_CONFIG_DIR`, `REWE_DATA_DIR`, `REWE_STATE_DIR`, or `REWE_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `REWE_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export REWE_HOME=/srv/rewe
rewe-pp-cli doctor
```

Under `REWE_HOME=/srv/rewe`, the four dirs resolve to `/srv/rewe/config`, `/srv/rewe/data`, `/srv/rewe/state`, and `/srv/rewe/cache`.

On Windows, set the same variable in PowerShell:

```powershell
$env:REWE_HOME = "C:\srv\rewe"
rewe-pp-cli doctor
```

Under `$env:REWE_HOME = "C:\srv\rewe"`, the four dirs resolve to `C:\srv\rewe\config`, `C:\srv\rewe\data`, `C:\srv\rewe\state`, and `C:\srv\rewe\cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "rewe": {
      "command": "rewe-pp-mcp",
      "env": {
        "REWE_HOME": "/srv/rewe"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `REWE_DATA_DIR` overrides an explicit `--home` for that kind. Use `REWE_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `REWE_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `rewe-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

### Local data

Reads are saved locally by default: in `--data-source auto` mode every successful read is also written to the SQLite store (`data.db`, or `data-<hash>.db` per credential) in the data directory. GET responses are cached for five minutes under `http/` in the cache directory. Credentials live in `credentials.toml` in the data directory. The learn journal lives under `learn/` in the state directory. `rewe-pp-cli doctor` prints the resolved paths, every store file, and the response-cache size.

None of it is encrypted. New files are created owner-only; beyond that the data relies on OS account permissions and full-disk encryption (FileVault, BitLocker, LUKS).

- **Erase it:** `rewe-pp-cli local-data purge` lists the store files, response cache, and learn journal it would delete; add `--yes` to delete them, or `--only store|cache|journal` to narrow it. Config and credentials are kept.
- **Run without a local copy:** set `REWE_NO_LOCAL_DATA=1`. Reads go to the API (`--data-source live`; `--data-source local` still reads an existing store), the response cache is off (`--no-cache`), nothing new is written to the store, and `sync` and `workflow archive` refuse to run. Set it in an MCP host's `env` block and every command the server runs inherits it.
- **Choose the read source without flags:** `REWE_DATA_SOURCE=auto|live|local` sets the `--data-source` default; an explicit flag wins.

## Commands

### basket

The shop basket of the logged-in customer (versioned; every change returns the whole basket).

- **`rewe-pp-cli basket add-many`** - Add several listings at once (used after recipe mapping).
- **`rewe-pp-cli basket get`** - Show a basket with line items, totals, fees and the reserved pickup slot.
- **`rewe-pp-cli basket remove`** - Remove a listing from the basket.
- **`rewe-pp-cli basket set`** - Set the quantity of a listing in the basket (adds it when missing). Quantity 0 removes it.

### favorites

"Deine Produkte": favourite lists.

- **`rewe-pp-cli favorites add`** - Add a product to a favourite list.
- **`rewe-pp-cli favorites list`** - List favourite lists and their items.
- **`rewe-pp-cli favorites remove`** - Remove an item from a favourite list.

### markets

Find REWE markets that offer pickup or delivery and select the market the shop works with.

- **`rewe-pp-cli markets current`** - Show the selected market, service type, postal code and whether the session is logged in.
- **`rewe-pp-cli markets pickup`** - List markets with pickup service (Abholservice) for a German postal code, nearest first.
- **`rewe-pp-cli markets search`** - Search REWE markets by postal code, city or street, with opening status.
- **`rewe-pp-cli markets select`** - Select the market and service (PICKUP or DELIVERY) that prices, basket and slots refer to.
- **`rewe-pp-cli markets services`** - Show whether pickup and delivery are available for a postal code.

### payment

Payment options for the current checkout.

- **`rewe-pp-cli payment`** - List enabled and disabled payment methods for an order total (MARKET_PAYMENT = pay at pickup).

### products

Search the catalogue of the selected market with current prices.

- **`rewe-pp-cli products predictions`** - Suggested follow-up search terms for products already chosen (cross-sell).
- **`rewe-pp-cli products search`** - Search products of the selected market; returns listing ids (needed for the basket) and prices in cents.

### purchased

"Bereits gekauft": products bought before in the online shop.

- **`rewe-pp-cli purchased`** - List previously bought products with categories and current offers among them.

### recipes

REWE recipes (Rezeptwelt).

- **`rewe-pp-cli recipes mapping`** - Result of an ingredient-to-product mapping (poll until processed is true).
- **`rewe-pp-cli recipes search`** - Search recipes; returns recipe ids, total count and facets.
- **`rewe-pp-cli recipes tiles`** - Recipe cards (title, rating, time, difficulty, url) for recipe ids.

### slots

Pickup timeslots of the selected market.

- **`rewe-pp-cli slots list`** - List all pickup slots with start, end, fee and whether they are selected.
- **`rewe-pp-cli slots overview`** - Short overview of the next pickup slots (header widget).
- **`rewe-pp-cli slots reserve`** - Reserve a pickup slot for the basket (expires after about 1 hour unless the order is placed).


### auth


- **`rewe-pp-cli auth renew`** - Keep the REWE shop session alive without a password by replaying the 30-day single-sign-on cookie.


### basket


- **`rewe-pp-cli basket add`** - Add products by search text and quantity without knowing listing ids.


### deals


- **`rewe-pp-cli deals mine`** - Offers that match products you bought before or saved as favourites.


### list


- **`rewe-pp-cli list add`** - Turn a shopping list (one item per line, optional quantity) into basket items.


### offers


- **`rewe-pp-cli offers`** - Current offers of your market (and next week's once REWE publishes them) as rows with title, price and category.


### order


- **`rewe-pp-cli order place`** - Place the pickup order: refresh basket, reserve slot, pay in market, submit - only with --confirm.


### price


- **`rewe-pp-cli price history`** - Price changes of a product seen in searches and offers over time.


### recipe


- **`rewe-pp-cli recipe basket`** - Map a REWE recipe's ingredients to shop products for N servings and add the recommended ones.


### slots


- **`rewe-pp-cli slots earliest`** - The earliest free pickup slot with its fee, optionally reserved right away.




### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`rewe-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`rewe-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`rewe-pp-cli learnings list`** - Inspect taught rows
- **`rewe-pp-cli learnings forget <query>`** - Undo a teach
- **`rewe-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`rewe-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`rewe-pp-cli teach-pattern`** - Install a query/resource template up front
- **`rewe-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `REWE_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `rewe-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
rewe-pp-cli basket get mock-value

# JSON for scripting and agents
rewe-pp-cli basket get mock-value --json
# Filter to specific fields by name
rewe-pp-cli basket get mock-value --json --select <field>[,<field>...]

# Dry run — show the request without sending
rewe-pp-cli basket get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
rewe-pp-cli basket get mock-value --agent
```

`--csv` prefixes a leading `'` on cells and header keys that start with `=`, `+`, `-`, `@`, a tab, or a line break (including fullwidth `＝＋－＠`) so a spreadsheet does not run them as formulas. Plain signed numbers such as `-5` and `+2.5e3` are left unchanged.

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Explicit confirmation** - `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `6` partial failure, `7` rate limited, `10` config error.

## Health Check

```bash
rewe-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `rewe-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/rewe-pp-cli/config.toml` on macOS/Linux and `%USERPROFILE%\.config\rewe-pp-cli\config.toml` on Windows; `--home`, `REWE_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

`--ca-cert`, `REWE_CA_CERT`, or `ca_cert` adds a PEM CA bundle to the system trust roots. `--tls-server-name`, `REWE_TLS_SERVER_NAME`, or `tls_server_name` is the name checked against the certificate when the dialed address does not match it. Precedence is flag, then environment, then the config file. A config file other users can change does not supply these settings. Certificate verification failures are not retried.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `rewe-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **exit code 4 / re-login needed** — Log in to rewe.de in a browser again, then run rewe-pp-cli auth import
- **403 with Cloudflare challenge** — Wait a few minutes and retry; keep request volume low; re-import cookies to refresh cf_clearance
- **prices look national, not your market** — rewe-pp-cli markets select --zip 50667 --market-id 3200008

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

TLS certificates are verified by default. For a trusted development or self-signed endpoint only, pass `--insecure` for one invocation, set `REWE_SKIP_TLS_VERIFY=true` for the current environment, or set `skip_tls_verify = true` in the config file for a persistent override. `--insecure` overrides `--ca-cert` and `--tls-server-name`.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://www.rewe.de/api/marketselection/configuration
- Capture coverage: 205 API entries from 241 total network entries
- Reachability: browser_clearance_http (82% confidence)
- Protocols: ssr_embedded_data (85% confidence), rest_json (75% confidence), html_scrape (55% confidence)
- Protection signals: cloudflare (90% confidence), login_redirect (80% confidence)
- Generation hints: browser_clearance_required, requires_browser_auth, requires_protected_client
- Candidate command ideas: create_authenticate — Derived from observed POST /realms/sso/login-actions/authenticate traffic.; create_changes — Derived from observed POST /shop/api/baskets/{basket_id}/changes traffic.; create_frontend_includes — Derived from observed POST /api/frontend-includes traffic.; create_lineitems — Derived from observed POST /shop/api/favorites/{favorite_id}/lineitems traffic.; create_listings — Derived from observed POST /shop/api/baskets/listings traffic.; create_mapping — Derived from observed POST /api/v2/recipes/mapping traffic.; create_multi_product_bonus_badges — Derived from observed POST /shop/api/multi-product-bonus-badges traffic.; create_submit — Derived from observed POST /shop/checkout/payment-options/submit traffic.

Warnings from discovery:
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- error_status_cluster: Endpoint cluster only observed error HTTP statuses.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**korb**](https://github.com/yannick-cw/korb) — Haskell
- [**rewerse-engineering**](https://github.com/ByteSizedMarius/rewerse-engineering) — Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
