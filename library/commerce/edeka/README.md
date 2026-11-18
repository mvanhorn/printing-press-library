# EDEKA CLI

**Shop your local EDEKA market from the terminal or an agent: offers, basket by text, earliest pickup slot, checkout with a confirmation gate.**

Works with every market on edeka.shop by its subdomain. Markets, offers and product search need no login; basket, slots and checkout use your imported browser session, which renews itself through the shop's one-year remember-me cookie.

Learn more at [EDEKA](https://edeka.shop).

Created by [@jimpresting](https://github.com/jimpresting) (JimPresting).

## Install

The recommended path installs both the `edeka-pp-cli` binary and the `pp-edeka` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install edeka
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install edeka --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install edeka --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install edeka --agent claude-code
npx -y @mvanhorn/printing-press-library install edeka --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.11 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/edeka/cmd/edeka-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/edeka-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install edeka --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-edeka --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-edeka --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install edeka --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle uses the EDEKA session file — import it first from a logged-in browser (auth import is not available through MCP; point EDEKA_SESSION_FILE at the same file for the MCP server):

```bash
edeka-pp-cli auth import --cdp http://127.0.0.1:9222
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/edeka-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/edeka/cmd/edeka-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS, `%APPDATA%\Claude\claude_desktop_config.json` on Windows):

```json
{
  "mcpServers": {
    "edeka": {
      "command": "edeka-pp-mcp"
    }
  }
}
```

</details>

### Remote MCP over HTTP

`edeka-pp-mcp --transport http` (or `PP_MCP_TRANSPORT=http`) serves streamable HTTP at `http://127.0.0.1:7777/mcp`. It refuses to start until `EDEKA_MCP_HTTP_TOKEN` is set to at least 32 characters, and every request must send `Authorization: Bearer <token>`. The command below prints 64 hex characters and meets that minimum. Plain HTTP is allowed only on a loopback `--addr`; any other bind, including `:7777` (every interface), also needs `--tls-cert` and `--tls-key`. Requests with no `Origin` header are accepted, as are loopback origins (`localhost`, `127.0.0.1`, or `[::1]`, any port). Any other `Origin` is refused unless it is listed in `EDEKA_MCP_HTTP_ALLOWED_ORIGINS`, a comma-separated list of `scheme://host[:port]` values with no path. That list is not a CORS policy; the server does not emit CORS headers.

```bash
export EDEKA_MCP_HTTP_TOKEN="$(openssl rand -hex 32)"
edeka-pp-mcp --transport http
```

Set `EDEKA_MCP_READ_ONLY=1` (exactly `1`) to drop mirrored commands that are not marked read-only and make the raw execute tool refuse state-changing calls. Commands marked local-write, which write only this CLI's own learn store, stay available.

## Authentication

EDEKA has no public API. Log in to your market's shop (https://<market>.edeka.shop) once in a browser, then run 'edeka-pp-cli auth import --cdp http://127.0.0.1:<port>' or 'auth import --file cookies.json'. The CLI keeps the cookies in a session file (EDEKA_SESSION_FILE overrides the path) and renews the shop session through the remember-me cookie oxid_1. When that cookie is gone, commands exit with code 4: log in again and re-import.

## Quick Start

```bash
# check the install without network
edeka-pp-cli doctor --dry-run

# find EDEKA markets near a postal code
edeka-pp-cli markets search 89073

# work with that market
edeka-pp-cli markets select doerflinger-ulm

# this week's offers
edeka-pp-cli offers --limit 10

# basket by text (needs an imported session)
edeka-pp-cli basket add hafermilch --qty 2 --dry-run

# earliest pickup slot
edeka-pp-cli slots earliest

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Order flow
- **`basket add`** — Add a product by id or by plain words; the CLI picks the best match in your market and adds it.

  _Use it when a customer names an item in their own words._

  ```bash
  edeka-pp-cli basket add hafermilch --qty 2 --dry-run
  ```
- **`slots earliest`** — The first free pickup slot of your market in one call.

  _Use it to propose a pickup time before ordering._

  ```bash
  edeka-pp-cli slots earliest --market doerflinger-ulm --json
  ```
- **`order place`** — Preview the order (slot, payment, items, total) and place it only with --confirm.

  _Use it after the customer said yes to the preview._

  ```bash
  edeka-pp-cli order place --payment mxcash --dry-run
  ```
- **`list add`** — Add many free-text lines with quantities at once and see what matched.

  _Use it for a whole shopping list from one message._

  ```bash
  edeka-pp-cli list add milch butter '2x kaffee' --dry-run
  ```

### Shopping intelligence
- **`deals check`** — Which of these items are on offer in your market this week.

  _Use it to tell the customer about offers on their usual items._

  ```bash
  edeka-pp-cli deals check milch kaffee butter --market doerflinger-ulm --json
  ```
- **`price history`** — Prices an article had over time in your markets, from your own searches and offer lookups.

  _Use it to judge whether an offer is really cheap._

  ```bash
  edeka-pp-cli price history 5915407005MXST --json
  ```
- **`compare markets`** — The same search on several EDEKA markets side by side.

  _Use it when the customer can choose between nearby markets._

  ```bash
  edeka-pp-cli compare markets milch --market doerflinger-ulm --json
  ```

## Recipes

### Offers in one category

```bash
edeka-pp-cli offers --category Kuehlprodukte --agent --select title,price,unit_price
```

Compact offer rows for an agent reply.

### Are my staples on offer

```bash
edeka-pp-cli deals check milch kaffee butter --market doerflinger-ulm --json
```

Offer-badged hits per term.

### Basket by voice note

```bash
edeka-pp-cli list add milch '2x butter' bananen --dry-run --json
```

Shows what each line would add before changing the basket.

### Preview the order

```bash
edeka-pp-cli order place --slot earliest --payment mxcash --json
```

Reserves the earliest pickup slot and prints slot, payment, items and total from the shop review page without ordering; needs items in the basket. Add --confirm --expect-total <total_cents> only after the customer said yes.

## Usage

Run `edeka-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `EDEKA_CONFIG_DIR`, `EDEKA_DATA_DIR`, `EDEKA_STATE_DIR`, or `EDEKA_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `EDEKA_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export EDEKA_HOME=/srv/edeka
edeka-pp-cli doctor
```

Under `EDEKA_HOME=/srv/edeka`, the four dirs resolve to `/srv/edeka/config`, `/srv/edeka/data`, `/srv/edeka/state`, and `/srv/edeka/cache`.

On Windows, set the same variable in PowerShell:

```powershell
$env:EDEKA_HOME = "C:\srv\edeka"
edeka-pp-cli doctor
```

Under `$env:EDEKA_HOME = "C:\srv\edeka"`, the four dirs resolve to `C:\srv\edeka\config`, `C:\srv\edeka\data`, `C:\srv\edeka\state`, and `C:\srv\edeka\cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "edeka": {
      "command": "edeka-pp-mcp",
      "env": {
        "EDEKA_HOME": "/srv/edeka"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `EDEKA_DATA_DIR` overrides an explicit `--home` for that kind. Use `EDEKA_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `EDEKA_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `edeka-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

### Local data

Reads are saved locally by default: in `--data-source auto` mode every successful read is also written to the SQLite store (`data.db`, or `data-<hash>.db` per credential) in the data directory. GET responses are cached for five minutes under `http/` in the cache directory. Credentials live in `credentials.toml` in the data directory. The learn journal lives under `learn/` in the state directory. `edeka-pp-cli doctor` prints the resolved paths, every store file, and the response-cache size.

None of it is encrypted. New files are created owner-only; beyond that the data relies on OS account permissions and full-disk encryption (FileVault, BitLocker, LUKS).

- **Erase it:** `edeka-pp-cli local-data purge` lists the store files, response cache, and learn journal it would delete; add `--yes` to delete them, or `--only store|cache|journal` to narrow it. Config and credentials are kept.
- **Run without a local copy:** set `EDEKA_NO_LOCAL_DATA=1`. Reads go to the API (`--data-source live`; `--data-source local` still reads an existing store), the response cache is off (`--no-cache`), nothing new is written to the store, and `sync` and `workflow archive` refuse to run. Set it in an MCP host's `env` block and every command the server runs inherits it.
- **Choose the read source without flags:** `EDEKA_DATA_SOURCE=auto|live|local` sets the `--data-source` default; an explicit flag wins.

## Commands

Every command takes `--json`, `--agent` and `--select`. Shop commands take `--market <subdomain>` (default: `EDEKA_MARKET`, then `markets select`). Commands marked *login* need an imported session (`auth import`); the rest work for any market without login.

### markets

- **`edeka-pp-cli markets search <postal-code | town>`** - Markets that sell online near a postal code or in a town: slug, address, pickup/delivery flags, delivery postal codes.
- **`edeka-pp-cli markets select <subdomain>`** - Choose the market for later commands (also stored in the session file).
- **`edeka-pp-cli markets current`** - The selected market and whether the session is logged in there.
- **`edeka-pp-cli markets zip <postal-code>`** - Raw market finder rows (generated endpoint).
- **`edeka-pp-cli markets cities <town>`** - The portal's raw town list (generated endpoint; unfiltered).

### offers, products, categories

- **`edeka-pp-cli offers [--category <path>] [--search <words>] [--limit N] [--page N]`** - This week's offers of the market (32 per page).
- **`edeka-pp-cli products search <words> [--limit N]`** - Product search with price, unit price, offer and weight flags; `in_basket` with a session.
- **`edeka-pp-cli products show <article-id | url>`** - One product: price, unit price, availability, description.
- **`edeka-pp-cli categories [--offers] [--level N]`** - The category tree; `--offers` lists the paths `offers --category` takes.
- **`edeka-pp-cli suggest <term>`** - The shop's search suggestions (generated endpoint).
- **`edeka-pp-cli deals check <term...>`** - Which of these items are on offer this week.
- **`edeka-pp-cli compare markets <words> [--market a --market b | --zip <postal-code>]`** - The same search on several markets side by side.
- **`edeka-pp-cli price history <article-id | words>`** - Prices recorded locally by searches and offer lookups.

### basket (*login*)

- **`edeka-pp-cli basket show`** - Items, quantities, prices, total.
- **`edeka-pp-cli basket add <article-id | words> [--qty N] [--grams G]`** - Add by id or by best match for the words; `--grams` for weight articles (meat/cheese counter).
- **`edeka-pp-cli list add <line...> [--file path]`** - Many lines such as `2x butter`, `milch`, `kaffee x2` at once, with a match report.
- **`edeka-pp-cli basket set <article-id> <quantity>`** - Absolute quantity (0 removes).
- **`edeka-pp-cli basket remove <article-id>`** - Remove a line.
- **`edeka-pp-cli basket clear`** - Empty the basket.

### slots

- **`edeka-pp-cli slots list [--days N] [--date YYYY-MM-DD] [--type pickup|delivery]`** - Free slots day by day.
- **`edeka-pp-cli slots earliest [--type pickup|delivery]`** - The first free slot.
- **`edeka-pp-cli slots reserve <earliest | "YYYY-MM-DD HH:MM">`** (*login*) - Choose the slot for the order; the shop holds it for a few minutes.
- **`edeka-pp-cli slots selected`** / **`slots reservation`** (*login*) - The slot chosen in this session and the shop's reservation state (generated endpoints).
- **`edeka-pp-cli slots release`** (*login*) - Ask the shop to drop the chosen slot. Some markets refuse (exit 5); the reservation then lapses on its own.

### checkout and orders (*login*)

- **`edeka-pp-cli payment options`** - Payment methods of the checkout (needs items and a slot).
- **`edeka-pp-cli order place [--slot earliest] [--payment mxcash|mxlocalcashcard] [--confirm --expect-total <cents>]`** - Without `--confirm` a preview only (slot, payment, items, total from the shop's final review page). With `--confirm` plus `--expect-total` (the preview's total) it places the order and refuses if the total changed. Pay at handover only.
- **`edeka-pp-cli order list`** - The order history.

### auth

- **`edeka-pp-cli auth import --cdp <loopback-url> | --file <cookies.json>`** - Import the cookies of a logged-in browser (not available through MCP).
- **`edeka-pp-cli auth session`** - Logged in or not, remember-me expiry; exit code 4 when a new login is needed.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`edeka-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`edeka-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`edeka-pp-cli learnings list`** - Inspect taught rows
- **`edeka-pp-cli learnings forget <query>`** - Undo a teach
- **`edeka-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`edeka-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`edeka-pp-cli teach-pattern`** - Install a query/resource template up front
- **`edeka-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `EDEKA_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `edeka-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Sessions, markets and order safety

- **No password in the CLI.** The customer logs in once in a browser; `auth import` copies the cookies into a session file (owner-only permissions). The shop's remember-me cookie `oxid_1` (one year) gives every later call a fresh shop session without a password. When it is gone, commands exit with code 4 and the customer logs in again.
- **One session file per customer.** `EDEKA_SESSION_FILE` points a call at that customer's file, so one server can serve many customers. The file also stores the customer's market.
- **Any market.** Each EDEKA retailer runs its own shop on `<market>.edeka.shop`; `--market` or `EDEKA_MARKET` switches between them. Cookies are per market, so a login is per market too.
- **Orders only with `--confirm`.** `order place` previews by default and walks the checkout only up to the shop's final review page. Show the preview to the customer and add `--confirm --expect-total <total_cents>` (plus `--slot '<confirm_slot>'` when the preview chose a slot) only after a clear yes; the order is refused when the basket total changed in between, and `--slot earliest` is refused with `--confirm` because it could pick a different time. Payment is limited to cash or card at handover.
- **Slots are held briefly.** `slots reserve` (and `order place --slot`) chooses a slot in the shop session; the shop holds it for a few minutes, so reserve right before ordering.
- **Not supported:** order cancellation (the shop shows no cancel control for orders on the tested market), recipes (the shops have none), online card or direct-debit payment, delivery slots before a delivery area was chosen once in the browser.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
edeka-pp-cli markets cities Ulm

# JSON for scripting and agents
edeka-pp-cli markets cities Ulm --json
# Filter to specific fields
edeka-pp-cli markets cities Ulm --json --select results.name,results.zipcode

# Dry run — show the request without sending
edeka-pp-cli markets cities Ulm --dry-run

# Agent mode — JSON + compact + no prompts in one flag
edeka-pp-cli markets cities Ulm --agent
```

`--csv` prefixes a leading `'` on cells and header keys that start with `=`, `+`, `-`, `@`, a tab, or a line break (including fullwidth `＝＋－＠`) so a spreadsheet does not run them as formulas. Plain signed numbers such as `-5` and `+2.5e3` are left unchanged.

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit confirmation** - `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - `price history` reads the local SQLite price store without network
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
edeka-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `edeka-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/edeka-pp-cli/config.toml` on macOS/Linux and `%USERPROFILE%\.config\edeka-pp-cli\config.toml` on Windows; `--home`, `EDEKA_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

`--ca-cert`, `EDEKA_CA_CERT`, or `ca_cert` adds a PEM CA bundle to the system trust roots. `--tls-server-name`, `EDEKA_TLS_SERVER_NAME`, or `tls_server_name` is the name checked against the certificate when the dialed address does not match it. Precedence is flag, then environment, then the config file. A config file other users can change does not supply these settings. Certificate verification failures are not retried.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `edeka-pp-cli auth session` to see whether the session is logged in; re-import with `edeka-pp-cli auth import` after a browser login
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run `edeka-pp-cli products search <words>` or `edeka-pp-cli basket show` to see valid article ids

### API-specific
- **exit code 4 / re-login needed** — Log in to <market>.edeka.shop in a browser, then run edeka-pp-cli auth import --cdp <url> (or --file cookies.json)
- **no market selected** — Run 'edeka-pp-cli markets select <subdomain>' once, or set EDEKA_MARKET=<subdomain>
- **basket call returns an HTML page instead of JSON** — The shop's form token expired; run the command again (the CLI fetches a fresh token once automatically)
