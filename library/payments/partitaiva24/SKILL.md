---
name: pp-partitaiva24
description: "Every Partitaiva24 endpoint as a typed CLI command, plus a local SQLite mirror and the full draft→PDF→SDI workflow agents need. Trigger phrases: `show partitaiva24 invoices`, `where am I against the forfettario limit`, `quarterly tax projection partitaiva24`, `AR aging for my partita iva`, `validate EU VAT IDs in my customer list`, `use partitaiva24`, `run partitaiva24`, `crea fattura partitaiva24`, `crea fattura draft`, `scarica pdf fattura`, `rata unica a data`, `trasmetti fattura SDI`, `bozza fattura`, `anteprima pdf fattura`."
author: "giuseppebisemi"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - partitaiva24-pp-cli
---

# Partitaiva24 — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `partitaiva24-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install partitaiva24 --cli-only
   ```
2. Verify: `partitaiva24-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Partitaiva24 ships a complete REST namespace at /api/v1/ but no SDK and no docs. This CLI mirrors the full surface — invoices, income invoices, customers, F24, corrispettivi, esterometro, docs, attachments, subscriptions — into a local store, then layers offline analytics most freelancers reach for monthly: forfettario turnover meter, quarterly IVA/IRPEF/INPS projection, customer concentration, VIES batch validation, F24 iCal export, and a portable backup the platform doesn't offer.

## When to Use This CLI

Pick this CLI when you (or your agent) need answers Partitaiva24's web UI fragments across multiple pages — quarterly tax projection, AR aging, customer concentration, SDI sweep — or when you need a portable backup of your invoicing data. Don't reach for it for one-off web actions like sending a single PDF; that's two clicks in the browser. The win is bulk operations and offline analytics over a synced local mirror.

## Unique Capabilities

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

## Command Reference

**account** — User profile and fiscal info

- `partitaiva24-pp-cli account show` — Get user profile
- `partitaiva24-pp-cli account update` — Update profile

**attachments** — Generic file attachments

- `partitaiva24-pp-cli attachments add` — Upload an attachment
- `partitaiva24-pp-cli attachments delete` — Delete an attachment
- `partitaiva24-pp-cli attachments list` — List attachments

**corrispettivi** — Corrispettivi telematici (cash receipts)

- `partitaiva24-pp-cli corrispettivi attach` — Attach file to corrispettivo
- `partitaiva24-pp-cli corrispettivi create` — Create a corrispettivo
- `partitaiva24-pp-cli corrispettivi delete` — Delete a corrispettivo
- `partitaiva24-pp-cli corrispettivi detach` — Detach file from corrispettivo
- `partitaiva24-pp-cli corrispettivi draft` — Get draft corrispettivo
- `partitaiva24-pp-cli corrispettivi get` — Get a corrispettivo
- `partitaiva24-pp-cli corrispettivi list` — List corrispettivi
- `partitaiva24-pp-cli corrispettivi update` — Update a corrispettivo

**customers** — Customer registry

- `partitaiva24-pp-cli customers create` — Create a customer
- `partitaiva24-pp-cli customers delete` — Delete a customer
- `partitaiva24-pp-cli customers get` — Get a customer
- `partitaiva24-pp-cli customers list` — List customers
- `partitaiva24-pp-cli customers update` — Update a customer

**data_request** — GDPR data export request

- `partitaiva24-pp-cli data_request status` — Status of pending data-export request
- `partitaiva24-pp-cli data_request submit` — Submit a data-export request

**docs** — Documents from your commercialista

- `partitaiva24-pp-cli docs list` — List documents
- `partitaiva24-pp-cli docs mark_read` — Mark document as read
- `partitaiva24-pp-cli docs upload` — Upload a document

**esterometro** — Foreign transactions (esterometro)

- `partitaiva24-pp-cli esterometro create` — Create a foreign-mgmt entry
- `partitaiva24-pp-cli esterometro delete` — Delete a foreign-mgmt entry
- `partitaiva24-pp-cli esterometro get` — Get a foreign-mgmt entry
- `partitaiva24-pp-cli esterometro list` — List foreign-mgmt entries
- `partitaiva24-pp-cli esterometro mark_paid` — Mark foreign entry as paid
- `partitaiva24-pp-cli esterometro update` — Update a foreign-mgmt entry

**events** — Calendar events (deadlines, due dates)

- `partitaiva24-pp-cli events` — List calendar events

**f24** — F24 tax payment forms

- `partitaiva24-pp-cli f24 archive` — Archive an F24
- `partitaiva24-pp-cli f24 list` — List F24 forms
- `partitaiva24-pp-cli f24 mark_paid` — Mark F24 as paid
- `partitaiva24-pp-cli f24 mark_read` — Mark F24 as read
- `partitaiva24-pp-cli f24 ravvedimento` — Request ravvedimento operoso (late-payment correction)

**fiscal_year** — Fiscal-year metadata (turnover limit, regime, default VAT)

- `partitaiva24-pp-cli fiscal_year <year>` — Get fiscal-year info

**income** — Income invoices (received from suppliers)

- `partitaiva24-pp-cli income attachment` — Download an income invoice attachment by index
- `partitaiva24-pp-cli income get` — Get a received invoice
- `partitaiva24-pp-cli income import` — Import received invoices
- `partitaiva24-pp-cli income list` — List received invoices
- `partitaiva24-pp-cli income mark_paid` — Mark a received invoice as paid
- `partitaiva24-pp-cli income mark_read` — Mark a received invoice as read

**invoices** — Active invoices (issued to customers)

- `partitaiva24-pp-cli invoices add_attachment` — Attach a file to the invoice
- `partitaiva24-pp-cli invoices create` — Create an invoice (raw — prefer `create-safe` for unattended use)
- **`partitaiva24-pp-cli invoices create-safe`** — Create with phantom protection + idempotent pre-check (recommended for agents)
- `partitaiva24-pp-cli invoices defaults` — Invoice defaults — VAT rates, doc types, pension funds, witholdings
- `partitaiva24-pp-cli invoices delete` — Delete an invoice (drafts only)
- **`partitaiva24-pp-cli invoices download-pdf`** — Render proforma PDF locally via the SPA's bundled jsPDF (`--no-footer`, `--launch`)
- `partitaiva24-pp-cli invoices export` — Export invoices to XML/ZIP
- `partitaiva24-pp-cli invoices file` — Get the SDI XML envelope URL (charged invoices only)
- `partitaiva24-pp-cli invoices get` — Get an invoice by ID
- `partitaiva24-pp-cli invoices import` — Import invoices from XML/SDI
- **`partitaiva24-pp-cli invoices link`** — Print the canonical web UI URL for an invoice
- `partitaiva24-pp-cli invoices list` — List issued invoices
- `partitaiva24-pp-cli invoices list_attachments` — List invoice attachments
- `partitaiva24-pp-cli invoices mark_paid` — Mark an invoice as paid
- `partitaiva24-pp-cli invoices remove_attachment` — Remove an attachment from the invoice
- `partitaiva24-pp-cli invoices sdi_accept` — Accept (or reject) SDI-refused invoices
- `partitaiva24-pp-cli invoices sdi_notification` — Get SDI transmission notification for the invoice
- `partitaiva24-pp-cli invoices send_pdf` — Email the invoice PDF to its recipient
- `partitaiva24-pp-cli invoices sign` — Digitally sign the invoice
- `partitaiva24-pp-cli invoices skeleton` — Default starter invoice document shape (use as create-safe body template)
- `partitaiva24-pp-cli invoices stats` — Annual invoice statistics — turnover, paid, unpaid, VAT
- `partitaiva24-pp-cli invoices transmit` — Transmit invoice to SDI ⚠️ **IRREVERSIBLE** (only undo: nota di credito)
- `partitaiva24-pp-cli invoices update` — Update an invoice (drafts: any field; charged: limited)

**notifications** — User notifications and badge counts

- `partitaiva24-pp-cli notifications broadcast` — List broadcast notifications
- `partitaiva24-pp-cli notifications list` — List notifications + badge counts

**password** — Account password

- `partitaiva24-pp-cli password` — Change account password

**questionario** — Annual tax questionnaire

- `partitaiva24-pp-cli questionario attach` — Attach a file to the questionnaire
- `partitaiva24-pp-cli questionario config` — Questionnaire window (start/end dates)
- `partitaiva24-pp-cli questionario show` — Get questionnaire for a year
- `partitaiva24-pp-cli questionario submit` — Submit / update the questionnaire for a year

**settings** — User settings

- `partitaiva24-pp-cli settings list` — List settings
- `partitaiva24-pp-cli settings set` — Set or update a named setting
- `partitaiva24-pp-cli settings unset` — Delete a named setting

**subscriptions** — Account subscription / billing

- `partitaiva24-pp-cli subscriptions cancel` — Cancel a subscription
- `partitaiva24-pp-cli subscriptions gateways` — Available payment gateways
- `partitaiva24-pp-cli subscriptions get` — Get a subscription
- `partitaiva24-pp-cli subscriptions list` — List subscriptions
- `partitaiva24-pp-cli subscriptions update` — Update a subscription

**tickets** — Support tickets

- `partitaiva24-pp-cli tickets categories` — List ticket categories
- `partitaiva24-pp-cli tickets create` — Open a support ticket
- `partitaiva24-pp-cli tickets get` — Get a ticket
- `partitaiva24-pp-cli tickets list` — List support tickets
- `partitaiva24-pp-cli tickets reply` — Reply to a ticket

**tools** — Utility tools (VIES, PA registry, XML parser, signing)

- `partitaiva24-pp-cli tools check_vies` — Validate an EU VAT ID via VIES
- `partitaiva24-pp-cli tools get` — API version info
- `partitaiva24-pp-cli tools ping` — Health check ping
- `partitaiva24-pp-cli tools search_pa` — Look up Italian Pubblica Amministrazione by IPA code
- `partitaiva24-pp-cli tools sign` — Digitally sign a PDF document
- `partitaiva24-pp-cli tools system_info` — Server system info (PHP, MySQL, OS)
- `partitaiva24-pp-cli tools xml2invoice` — Parse an SDI XML file into invoice JSON


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
partitaiva24-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Forfettario quarter check-in

```bash
partitaiva24-pp-cli turnover --year 2026 --json
```

Sync, then read the two numbers every forfettario freelancer wants on the 1st of every month.

### Find stuck SDI transmissions

```bash
partitaiva24-pp-cli sdi watch --older-than 7d --json --select id,number,date,to.companyname
```

Lists every invoice transmitted >7 days ago without an ack — narrow output to four columns so an agent can act on it cleanly.

### Validate every EU customer's VAT in one shot

```bash
partitaiva24-pp-cli vies bulk --json
```

Runs /tools/check-vies for each synced EU customer and emits a structured invalid/valid list.

### Backup before switching commercialista

```bash
partitaiva24-pp-cli backup -o ~/backups/p24-$(date +%Y%m%d).zip
```

Full snapshot — SQLite + CSV + actual invoice PDFs — the portable copy the platform doesn't give you.

### AR aging for a meeting

```bash
partitaiva24-pp-cli aging --json --select bucket,customer,total
```

Three columns ready to paste into a meeting note.

### Crea fattura draft + scarica PDF (workflow agentico completo)

```bash
# 1. Crea la draft con phantom protection (idempotent, auto-cleanup)
RESP=$(cat fattura.json | partitaiva24-pp-cli invoices create-safe \
  --stdin --auto-cleanup --idempotent --json)
ID=$(echo "$RESP" | jq -r .id)
echo "Draft creata: $ID"

# 2. Scarica il PDF proforma (default = senza watermark agent-friendly)
partitaiva24-pp-cli invoices download-pdf "$ID" -o "/tmp/draft-$ID.pdf" --no-footer

# 3. Restituisci link + path PDF all'utente
echo "Web: $(partitaiva24-pp-cli invoices link "$ID")"
echo "PDF: /tmp/draft-$ID.pdf"
```

Quattro chiamate, zero prompt interattivi, output 100% strutturato. Sicuro per esecuzione unattended: `--idempotent` evita duplicati su retry, `--auto-cleanup` recupera da partial-failure WordPress.

### Refresh automatico (cookie + nonce in un comando)

```bash
partitaiva24-pp-cli auth refresh
```

Legge il cookie direttamente da Chrome e recupera il nonce dalla homepage di Partitaiva24 — zero DevTools, zero copia-incolla. Richiede `pycookiecheat` (`pip install pycookiecheat`) installato una volta sola.

Usa questo ogni volta che vedi `rest_cookie_invalid_nonce`, o programmalo come cron giornaliero se usi il CLI in automazione.

### Anteprima PDF rapida (apri direttamente)

```bash
partitaiva24-pp-cli invoices download-pdf <id> -o "/tmp/$(date +%s).pdf" --launch
```

Crea il PDF temp, lo apre in Anteprima (macOS) / xdg-open (Linux). Per quando vuoi vedere subito una draft modificata.

## Auth Setup

Partitaiva24's API is gated by the same WordPress session your browser uses: the `p24_logged_in_<hash>` cookie plus an `X-WP-Nonce` header on every authenticated call.

### First-time setup

```bash
# 1. Login to https://partitaiva24.cloud/ in Chrome.
# 2. F12 → Network tab → reload → click any /api/v1/* XHR.
# 3. Request Headers → copy:
#    - Cookie:     full value starting with "p24_logged_in_..."
#    - X-WP-Nonce: 10-char hex token
# 4. Save both:
partitaiva24-pp-cli auth set \
  --cookie 'p24_logged_in_<hash>=email|exp|hash|hmac' \
  --nonce 'abc1234567'

# 5. Verify:
partitaiva24-pp-cli doctor
# Expected: "Credentials: valid" against /user/profile
```

### Nonce refresh (every ~24h)

WordPress nonces rotate roughly daily. When you see `rest_cookie_invalid_nonce`:

```bash
# Open DevTools, copy the new X-WP-Nonce, then:
partitaiva24-pp-cli auth set --nonce 'xyz9876543'
```

The cookie itself stays valid until you sign out of Chrome.

### Env-var alternative (CI / scripts)

```bash
export P24_COOKIE='p24_logged_in_<hash>=...'
export P24_NONCE='abc1234567'
partitaiva24-pp-cli doctor
```

Env vars override config file; useful for ephemeral runs and CI.

### Why two-part credential

WordPress REST plugins use **double-submit nonce** for CSRF protection: the cookie proves authentication, the X-WP-Nonce header proves intent. The CLI handles both transparently — you never see them again after `auth set`. The API returns `rest_cookie_invalid_nonce` (400-shape error, exit code 5) when the nonce is stale; that's the only signal you need to refresh it.

### Mutators are real-world fiscally binding

`invoices transmit <id>` sends the invoice to **Sistema di Interscambio** (SDI) and is **irreversible** — the only way to "cancel" a transmitted invoice is a `nota di credito` (TD04). Best practice for agents:

1. Always create as `status: "draft"` first
2. Use `invoices download-pdf` to generate proforma + present to user for review
3. Use `invoices update` to fix anything
4. Only run `invoices transmit` after explicit human confirmation

`invoices delete` works only on drafts; charged/transmitted invoices cannot be deleted.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  partitaiva24-pp-cli attachments list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
partitaiva24-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
partitaiva24-pp-cli feedback --stdin < notes.txt
partitaiva24-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.partitaiva24-pp-cli/feedback.jsonl`. They are never POSTed unless `PARTITAIVA24_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `PARTITAIVA24_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
partitaiva24-pp-cli profile save briefing --json
partitaiva24-pp-cli --profile briefing attachments list
partitaiva24-pp-cli profile list --json
partitaiva24-pp-cli profile show briefing
partitaiva24-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `partitaiva24-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add partitaiva24-pp-mcp -- partitaiva24-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which partitaiva24-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   partitaiva24-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `partitaiva24-pp-cli <command> --help`.
