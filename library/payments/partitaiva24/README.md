# Partitaiva24 CLI

**Every Partitaiva24 endpoint as a typed CLI command, plus a local SQLite mirror that answers the questions the web UI doesn't — turnover countdown, quarterly tax projection, AR aging, SDI status sweep.**

Partitaiva24 ships a complete REST namespace at /api/v1/ but no SDK and no docs. This CLI mirrors the full surface — invoices, income invoices, customers, F24, corrispettivi, esterometro, docs, attachments, subscriptions — into a local store, then layers offline analytics most freelancers reach for monthly: forfettario turnover meter, quarterly IVA/IRPEF/INPS projection, customer concentration, VIES batch validation, F24 iCal export, and a portable backup the platform doesn't offer.

## Install

The recommended path installs both the `partitaiva24-pp-cli` binary and the `pp-partitaiva24` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install partitaiva24
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install partitaiva24 --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/partitaiva24-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-partitaiva24 --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-partitaiva24 --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-partitaiva24 skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-partitaiva24. The skill defines how its required CLI can be installed.
```

## Authentication

Partitaiva24's API is gated by the same WordPress session your browser uses: the `p24_logged_in_<hash>` cookie plus an `X-WP-Nonce` header.

**Recommended — automatic refresh (no DevTools):**

```bash
pip install pycookiecheat   # one-time setup
partitaiva24-pp-cli auth refresh
```

`auth refresh` reads the session cookie directly from your logged-in Chrome profile and fetches the nonce from the Partitaiva24 page automatically. Run it whenever you see `rest_cookie_invalid_nonce` — nonces rotate roughly every 24 hours.

**Manual fallback** (no Python required): open DevTools → Network → any `/api/v1/` request → copy `Cookie` and `X-WP-Nonce` headers, then run:

```bash
partitaiva24-pp-cli auth set --cookie '<full-cookie>' --nonce '<nonce>'
```

## Quick Start

```bash
# Capture the WordPress session and nonce from your browser DevTools.
partitaiva24-pp-cli auth set --cookie '<paste full Cookie header>' --nonce '<paste X-WP-Nonce>'


# Confirms the session is live and shows API version + system info.
partitaiva24-pp-cli doctor


# Pulls invoices, income invoices, customers, F24, corrispettivi, docs, attachments, events into the local store.
partitaiva24-pp-cli sync


# Where you stand against the forfettario cap right now.
partitaiva24-pp-cli turnover --year 2026


# Unpaid invoices bucketed by days overdue.
partitaiva24-pp-cli aging


# Find transmissions that didn't come back from SDI.
partitaiva24-pp-cli sdi watch --older-than 7d

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Tax & turnover insights
- **`turnover`** — See exactly how close you are to the forfettario annual turnover limit, with a days-to-limit projection at your current run-rate.

  _Forfettari blow past the cap with no warning. This is the single most-asked question and the platform doesn't answer it._

  ```bash
  partitaiva24-pp-cli turnover --year 2026 --json
  ```
- **`tax-due`** — Project IVA, IRPEF, and INPS owed at quarter-end from already-synced invoices, your fiscal regime, and tax rate.

  _Cash-flow planning needs a number, not the four web pages you'd have to visit otherwise._

  ```bash
  partitaiva24-pp-cli tax-due --quarter 2026-Q2 --json
  ```
- **`reconcile`** — Net out issued invoices against received invoices for any period — quarterly margin in one shot.

  _Margin per quarter without exporting two CSVs and pivoting them by hand._

  ```bash
  partitaiva24-pp-cli reconcile --period 2026-Q1 --json
  ```

### Receivables & cash flow
- **`aging`** — Group unpaid invoices into 0-30, 31-60, 61-90, and 90+ day buckets, broken down by customer.

  _The first thing any commercialista wants on the 1st of the month._

  ```bash
  partitaiva24-pp-cli aging --json --select bucket,customer,total
  ```
- **`clients top`** — Rank customers by year-to-date revenue and flag dangerous concentration (one client > 80% of turnover invites AdE attention for forfettari).

  _Concentration risk is invisible until it isn't; the AdE check is silent._

  ```bash
  partitaiva24-pp-cli clients top --year 2026 --limit 10
  ```

### Compliance audits
- **`vies bulk`** — Run /tools/check-vies for every EU customer in your registry and flag invalid or expired VAT IDs.

  _An invalid EU VAT on an invoice is an SDI rejection plus a reverse-charge mistake._

  ```bash
  partitaiva24-pp-cli vies bulk --json
  ```
- **`f24 ical`** — Emit upcoming F24 deadlines as an iCal file you can drop into any calendar app.

  _F24 is the most-missed Italian tax deadline. Surface it where you actually look._

  ```bash
  partitaiva24-pp-cli f24 ical -o ~/calendars/f24-due.ics
  ```
- **`sdi watch`** — List every invoice you've transmitted to SDI that hasn't been acknowledged after a threshold (default 7 days).

  _Stuck SDI transmissions become AdE penalties if you don't catch them._

  ```bash
  partitaiva24-pp-cli sdi watch --older-than 7d --json
  ```
- **`esterometro export`** — Generate the AdE-ready CSV for the quarterly esterometro filing from synced foreign-mgmt and EU/extra-EU invoices.

  _Quarterly compliance task; eliminating the manual CSV build is the entire point._

  ```bash
  partitaiva24-pp-cli esterometro export 2026 -o esterometro-2026.csv
  ```
- **`stamp-due`** — Sum stamp duty owed by quarter from invoices that carry a stamp, ready to cross-check against the AdE pre-fill.

  _The AdE prefill is wrong often enough to want a check column._

  ```bash
  partitaiva24-pp-cli stamp-due --year 2026 --json
  ```
- **`numbering audit`** — Verify your invoices are sequentially numbered per fiscal year as Italian law requires; flag gaps, duplicates, and out-of-order dates.

  _A missed numbering gap is a real AdE finding during an inspection._

  ```bash
  partitaiva24-pp-cli numbering audit --year 2026
  ```

### Local store
- **`backup`** — Sync every resource into local SQLite, then dump a portable archive (JSON + CSV + the actual invoice PDFs) you can carry to any other platform.

  _Real-world: every freelancer who's tried to leave one of these platforms knows this is the missing button._

  ```bash
  partitaiva24-pp-cli backup -o ~/backups/p24-$(date +%Y%m%d).zip
  ```

### Workflow fatturazione
- **`invoices create-safe`** — Crea una fattura draft con due difese: pre-check del numero (rifiuta duplicati o ritorna l'esistente con --idempotent) e post-error verify (rileva phantom record lasciati da partial-failure 4xx/5xx, opzionalmente li auto-cancella).

  _Indispensabile per workflow agentici: senza phantom protection una retry naive crea duplicati su un'API fiscalmente vincolante._

  ```bash
  partitaiva24-pp-cli invoices create-safe --file fattura.json --auto-cleanup --json --agent
  ```
- **`invoices link`** — Restituisce l'URL canonico della SPA per qualsiasi fattura. Output URL nudo per default, JSON con --json/--agent.

  _Agenti che mostrano una draft a un umano per approvazione passano dall'URL: uno stato (creato) e poi un link per il visual review._

  ```bash
  partitaiva24-pp-cli invoices link 06d5e64f-4be9-11f1-89c5-0667992834eb --json
  ```
- **`invoices download-pdf`** — Scarica il PDF identico al bottone 'Scarica PDF' della SPA usando il renderer jsPDF embedded nella CLI. --no-footer per togliere il watermark partitaiva24, --launch per aprirlo subito.

  _Chiude il cerchio del workflow agentico: create draft, download proforma, review, transmit. Tutto da CLI senza intervento umano sul browser._

  ```bash
  partitaiva24-pp-cli invoices download-pdf 06d5e64f-4be9-11f1-89c5-0667992834eb -o ~/Downloads/fattura.pdf --no-footer
  ```

### Auth
- **`auth set`** — Salva cookie p24_logged_in_<hash> e header X-WP-Nonce in config TOML. Il nonce ruota ogni ~24h: refresh con auth set --nonce <new>.

  _L'unico comando che NON esponiamo come MCP tool (mcp:hidden) per principio: gli agenti non devono mai leggere/scrivere credenziali dell'utente._

  ```bash
  partitaiva24-pp-cli auth set --cookie 'p24_logged_in_<hash>=...' --nonce 'abc1234567'
  ```

## Usage

Run `partitaiva24-pp-cli --help` for the full command reference and flag list.

## Commands

### account

User profile and fiscal info

- **`partitaiva24-pp-cli account show`** - Get user profile
- **`partitaiva24-pp-cli account update`** - Update profile

### attachments

Generic file attachments

- **`partitaiva24-pp-cli attachments add`** - Upload an attachment
- **`partitaiva24-pp-cli attachments delete`** - Delete an attachment
- **`partitaiva24-pp-cli attachments list`** - List attachments

### corrispettivi

Corrispettivi telematici (cash receipts)

- **`partitaiva24-pp-cli corrispettivi attach`** - Attach file to corrispettivo
- **`partitaiva24-pp-cli corrispettivi create`** - Create a corrispettivo
- **`partitaiva24-pp-cli corrispettivi delete`** - Delete a corrispettivo
- **`partitaiva24-pp-cli corrispettivi detach`** - Detach file from corrispettivo
- **`partitaiva24-pp-cli corrispettivi draft`** - Get draft corrispettivo
- **`partitaiva24-pp-cli corrispettivi get`** - Get a corrispettivo
- **`partitaiva24-pp-cli corrispettivi list`** - List corrispettivi
- **`partitaiva24-pp-cli corrispettivi update`** - Update a corrispettivo

### customers

Customer registry

- **`partitaiva24-pp-cli customers create`** - Create a customer
- **`partitaiva24-pp-cli customers delete`** - Delete a customer
- **`partitaiva24-pp-cli customers get`** - Get a customer
- **`partitaiva24-pp-cli customers list`** - List customers
- **`partitaiva24-pp-cli customers update`** - Update a customer

### data_request

GDPR data export request

- **`partitaiva24-pp-cli data_request status`** - Status of pending data-export request
- **`partitaiva24-pp-cli data_request submit`** - Submit a data-export request

### docs

Documents from your commercialista

- **`partitaiva24-pp-cli docs list`** - List documents
- **`partitaiva24-pp-cli docs mark_read`** - Mark document as read
- **`partitaiva24-pp-cli docs upload`** - Upload a document

### esterometro

Foreign transactions (esterometro)

- **`partitaiva24-pp-cli esterometro create`** - Create a foreign-mgmt entry
- **`partitaiva24-pp-cli esterometro delete`** - Delete a foreign-mgmt entry
- **`partitaiva24-pp-cli esterometro get`** - Get a foreign-mgmt entry
- **`partitaiva24-pp-cli esterometro list`** - List foreign-mgmt entries
- **`partitaiva24-pp-cli esterometro mark_paid`** - Mark foreign entry as paid
- **`partitaiva24-pp-cli esterometro update`** - Update a foreign-mgmt entry

### events

Calendar events (deadlines, due dates)

- **`partitaiva24-pp-cli events list`** - List calendar events

### f24

F24 tax payment forms

- **`partitaiva24-pp-cli f24 archive`** - Archive an F24
- **`partitaiva24-pp-cli f24 list`** - List F24 forms
- **`partitaiva24-pp-cli f24 mark_paid`** - Mark F24 as paid
- **`partitaiva24-pp-cli f24 mark_read`** - Mark F24 as read
- **`partitaiva24-pp-cli f24 ravvedimento`** - Request ravvedimento operoso (late-payment correction)

### fiscal_year

Fiscal-year metadata (turnover limit, regime, default VAT)

- **`partitaiva24-pp-cli fiscal_year get`** - Get fiscal-year info

### income

Income invoices (received from suppliers)

- **`partitaiva24-pp-cli income attachment`** - Download an income invoice attachment by index
- **`partitaiva24-pp-cli income get`** - Get a received invoice
- **`partitaiva24-pp-cli income import`** - Import received invoices
- **`partitaiva24-pp-cli income list`** - List received invoices
- **`partitaiva24-pp-cli income mark_paid`** - Mark a received invoice as paid
- **`partitaiva24-pp-cli income mark_read`** - Mark a received invoice as read

### invoices

Active invoices (issued to customers)

- **`partitaiva24-pp-cli invoices add_attachment`** - Attach a file to the invoice
- **`partitaiva24-pp-cli invoices create`** - Create an invoice
- **`partitaiva24-pp-cli invoices defaults`** - Invoice defaults — VAT rates, doc types, pension funds, witholdings
- **`partitaiva24-pp-cli invoices delete`** - Delete an invoice
- **`partitaiva24-pp-cli invoices export`** - Export invoices to XML/ZIP
- **`partitaiva24-pp-cli invoices file`** - Download invoice PDF
- **`partitaiva24-pp-cli invoices get`** - Get an invoice by ID
- **`partitaiva24-pp-cli invoices import`** - Import invoices from XML/SDI
- **`partitaiva24-pp-cli invoices list`** - List issued invoices
- **`partitaiva24-pp-cli invoices list_attachments`** - List invoice attachments
- **`partitaiva24-pp-cli invoices mark_paid`** - Mark an invoice as paid
- **`partitaiva24-pp-cli invoices remove_attachment`** - Remove an attachment from the invoice
- **`partitaiva24-pp-cli invoices sdi_accept`** - Accept (or reject) SDI-refused invoices
- **`partitaiva24-pp-cli invoices sdi_notification`** - Get SDI transmission notification for the invoice
- **`partitaiva24-pp-cli invoices send_pdf`** - Email the invoice PDF to its recipient
- **`partitaiva24-pp-cli invoices sign`** - Digitally sign the invoice
- **`partitaiva24-pp-cli invoices skeleton`** - Default starter invoice document shape
- **`partitaiva24-pp-cli invoices stats`** - Annual invoice statistics — turnover, paid, unpaid, VAT
- **`partitaiva24-pp-cli invoices transmit`** - Transmit invoice to SDI
- **`partitaiva24-pp-cli invoices update`** - Update an invoice

### notifications

User notifications and badge counts

- **`partitaiva24-pp-cli notifications broadcast`** - List broadcast notifications
- **`partitaiva24-pp-cli notifications list`** - List notifications + badge counts

### password

Account password

- **`partitaiva24-pp-cli password change`** - Change account password

### questionario

Annual tax questionnaire

- **`partitaiva24-pp-cli questionario attach`** - Attach a file to the questionnaire
- **`partitaiva24-pp-cli questionario config`** - Questionnaire window (start/end dates)
- **`partitaiva24-pp-cli questionario show`** - Get questionnaire for a year
- **`partitaiva24-pp-cli questionario submit`** - Submit / update the questionnaire for a year

### settings

User settings

- **`partitaiva24-pp-cli settings list`** - List settings
- **`partitaiva24-pp-cli settings set`** - Set or update a named setting
- **`partitaiva24-pp-cli settings unset`** - Delete a named setting

### subscriptions

Account subscription / billing

- **`partitaiva24-pp-cli subscriptions cancel`** - Cancel a subscription
- **`partitaiva24-pp-cli subscriptions gateways`** - Available payment gateways
- **`partitaiva24-pp-cli subscriptions get`** - Get a subscription
- **`partitaiva24-pp-cli subscriptions list`** - List subscriptions
- **`partitaiva24-pp-cli subscriptions update`** - Update a subscription

### tickets

Support tickets

- **`partitaiva24-pp-cli tickets categories`** - List ticket categories
- **`partitaiva24-pp-cli tickets create`** - Open a support ticket
- **`partitaiva24-pp-cli tickets get`** - Get a ticket
- **`partitaiva24-pp-cli tickets list`** - List support tickets
- **`partitaiva24-pp-cli tickets reply`** - Reply to a ticket

### tools

Utility tools (VIES, PA registry, XML parser, signing)

- **`partitaiva24-pp-cli tools check_vies`** - Validate an EU VAT ID via VIES
- **`partitaiva24-pp-cli tools get`** - API version info
- **`partitaiva24-pp-cli tools ping`** - Health check ping
- **`partitaiva24-pp-cli tools search_pa`** - Look up Italian Pubblica Amministrazione by IPA code
- **`partitaiva24-pp-cli tools sign`** - Digitally sign a PDF document
- **`partitaiva24-pp-cli tools system_info`** - Server system info (PHP, MySQL, OS)
- **`partitaiva24-pp-cli tools xml2invoice`** - Parse an SDI XML file into invoice JSON


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
partitaiva24-pp-cli attachments list

# JSON for scripting and agents
partitaiva24-pp-cli attachments list --json

# Filter to specific fields
partitaiva24-pp-cli attachments list --json --select id,name,status

# Dry run — show the request without sending
partitaiva24-pp-cli attachments list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
partitaiva24-pp-cli attachments list --agent
```

## Cookbook

Recipes that combine local-store reads, live API calls, and the analytics
commands that the web UI doesn't expose.

### Find unpaid invoices over 60 days old, by customer

```bash
partitaiva24-pp-cli sync
partitaiva24-pp-cli aging --as-of 2026-05-11 --json | \
  jq '.buckets[] | select(.bucket == "61-90" or .bucket == "90+")'
```

### Project Q2 taxes from synced invoices

```bash
partitaiva24-pp-cli tax-due --quarter 2026-Q2 --json --select \
  irpef_due,iva_due,inps_estimate,quarter_compenso
```

### Watch SDI for stuck transmissions older than 7 days

```bash
partitaiva24-pp-cli sdi watch --older-than 7d --json | \
  jq '.[] | {number, date, sdi_status}'
```

### Find clients concentrating revenue (forfettario single-client risk)

```bash
partitaiva24-pp-cli clients top --year 2026 --limit 5 --json
```

The `pct_of_total` field flags customers crossing the 80% threshold that
invites AdE scrutiny for forfettari.

### Audit invoice numbering for gaps and duplicates

```bash
partitaiva24-pp-cli numbering audit --year 2026 --json
```

Runs per-sezionale checks for gaps, duplicates, and out-of-order dates as
required by Italian invoicing law.

### Bulk-validate EU VAT IDs through VIES

```bash
partitaiva24-pp-cli vies bulk --limit 50 --json --select checked,invalid
```

### Export F24 deadlines to your calendar

```bash
partitaiva24-pp-cli f24 ical -o ~/Calendar/p24-f24-2026.ics
# Then drop the file into Apple Calendar, Google Calendar, etc.
```

### Generate quarterly esterometro CSV

```bash
partitaiva24-pp-cli esterometro export 2026 -o esterometro-2026.csv
```

### Reconcile issued vs received for a quarter

```bash
partitaiva24-pp-cli reconcile --period 2026-Q1 --json --select \
  active_total,passive_total,margin
```

### Create an invoice safely (idempotent + phantom cleanup)

```bash
partitaiva24-pp-cli invoices create-safe --file fattura.json \
  --idempotent --auto-cleanup --json
# or pipe JSON in directly
cat fattura.json | partitaiva24-pp-cli invoices create-safe --stdin \
  --idempotent --auto-cleanup --json
```

Wraps the underlying create with a pre-check (refuses duplicate numbers) and
post-error verify (cleans up phantom records left by partial 4xx/5xx failures).

### Open an invoice in the web UI from the terminal

```bash
open "$(partitaiva24-pp-cli invoices link <invoice-id>)"
```

### Search synced invoices full-text

```bash
partitaiva24-pp-cli sync
partitaiva24-pp-cli search "consulenza" --type invoices --data-source local
```

### Refresh auth without DevTools

```bash
pip install pycookiecheat   # one-time setup
partitaiva24-pp-cli auth refresh
```

Reads the WordPress session cookie from your logged-in Chrome profile and
fetches a fresh nonce. Run when calls return `rest_cookie_invalid_nonce`.

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-partitaiva24 -g
```

Then invoke `/pp-partitaiva24 <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
# Some tools work without auth. For full access, set up auth first:
partitaiva24-pp-cli auth login --chrome

claude mcp add partitaiva24 partitaiva24-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
partitaiva24-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/partitaiva24-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "partitaiva24": {
      "command": "partitaiva24-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
partitaiva24-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/partitaiva24-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `P24_COOKIE` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `partitaiva24-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $P24_COOKIE`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **rest_cookie_invalid_nonce** — Run `partitaiva24-pp-cli auth refresh` (requires `pycookiecheat`). Manual fallback: copy X-WP-Nonce from DevTools and run `auth set --nonce <value>`.
- **rest_not_logged_in** — Cookie expired. Run `partitaiva24-pp-cli auth refresh`, or sign in to partitaiva24.cloud, copy the full Cookie header, then `auth set --cookie '<value>'`.
- **Empty list on `invoices list` after sync succeeded** — The user has no invoices for the requested fiscal year. Try `--year` with another year or `invoices stats` to confirm.
- **`turnover` reports 0% but you have invoices** — `sync` first — turnover reads invoices-stats live but joins against the synced fiscal_year row.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
