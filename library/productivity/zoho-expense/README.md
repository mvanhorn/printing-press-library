# Zoho Expense CLI

**Upload receipts, auto-tag expenses from learned merchant memory, and close out the month in one command — the agent-friendly Zoho Expense CLI nobody else has built.**

Zoho ships autoscan; this CLI ships everything that comes after — receipt dedup, learned merchant→category mapping, India GST splitting, and monthly report bundling. Built for AI agents that ingest invoices from an inbox on a cadence and need an idempotent, structured-output upload path. India-region by default; works against any Zoho datacenter by overriding the base URL.

Learn more at [Zoho Expense](https://www.zoho.com/in/expense/).

Created by [@amitav13](https://github.com/amitav13) (Amitav Khandelwal).

## Install

The recommended path installs both the `zoho-expense-pp-cli` binary and the `pp-zoho-expense` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install zoho-expense
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install zoho-expense --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install zoho-expense --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install zoho-expense --agent claude-code
npx -y @mvanhorn/printing-press-library install zoho-expense --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/zoho-expense/cmd/zoho-expense-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/zoho-expense-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install zoho-expense --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-zoho-expense --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-zoho-expense --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install zoho-expense --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local OAuth tokens — authenticate first if you haven't:

```bash
zoho-expense-pp-cli auth login
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/zoho-expense-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `ZOHO_EXPENSE_ORGANIZATION_ID` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/zoho-expense/cmd/zoho-expense-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "zoho-expense": {
      "command": "zoho-expense-pp-mcp",
      "env": {
        "ZOHO_EXPENSE_ORGANIZATION_ID": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Zoho Expense uses OAuth 2.0 with a self-client authorization-code flow. Create a self-client at https://api-console.zoho.in/, generate a 10-minute authorization code, then run `zoho-expense-pp-cli auth login --client-id <id> --client-secret <secret>` (browser callback flow). The CLI exchanges the code for a long-lived refresh token (stored at `~/.config/zoho-expense-pp-cli/config.toml`) and refreshes access tokens transparently. Tokens use the literal `Zoho-oauthtoken` Authorization prefix (not `Bearer`).

## Quick Start

```bash
# Exchange the 10-min code for a refresh token
zoho-expense-pp-cli auth login --client-id $ZOHO_EXPENSE_CLIENT_ID --client-secret $ZOHO_EXPENSE_CLIENT_SECRET

# Set the active organization (first one in the list)
zoho-expense-pp-cli org use $(zoho-expense-pp-cli organizations list --json | jq -r '.[0].organization_id')

# Sync categories, reporting tags, projects, customers, expenses into the local store
zoho-expense-pp-cli sync

# Upload one receipt with hash-dedup and auto-tag
zoho-expense-pp-cli receipt upload ~/Downloads/uber-receipt.pdf --auto-tag

# Batch ingest a folder for AI consumption
zoho-expense-pp-cli invoice ingest ~/Downloads/october-invoices --auto-tag --agent

# Bundle the month into a report and submit
zoho-expense-pp-cli close --month 2026-10 --auto-submit

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Hermes-first ingestion
- **`invoice ingest`** — Batch upload a folder of invoices to Zoho Expense, SHA256-dedup against the local store, poll autoscan in parallel, and auto-tag from learned merchant memory — the headline workflow for AI agents that ingest invoices from email.

  _Agents that ingest invoices on a cadence need one command that handles dedup, autoscan, and tagging — not three._

  ```bash
  zoho-expense-pp-cli invoice ingest ~/Downloads/october-invoices --auto-tag --agent
  ```
- **`receipt upload`** — SHA256-hash incoming receipts at upload, refuse duplicates (use --force to override).

  _Idempotent receipt upload — safe to re-run on a folder agents have already processed._

  ```bash
  zoho-expense-pp-cli receipt upload ~/Downloads/uber-receipt.pdf --auto-tag
  ```
- **`expense-untagged`** — List expenses missing category_id or project_id; `--auto-fix` applies the merchant→tag memory map and PUTs back to Zoho.

  _One command turns 30 untagged scanned receipts into 30 tagged ones, using the user's own historical mapping._

  ```bash
  zoho-expense-pp-cli expense-untagged --auto-fix --agent
  ```

### Local intelligence
- **`merchant list`** — Build a local merchant→category/project/tag mapping from sync history; pre-fill tags on first sight of a new merchant.

  _Solves Zoho's 'first-time merchant is always uncategorized' limitation with zero API calls._

  ```bash
  zoho-expense-pp-cli merchant list --agent --select merchant_name,category_name,seen_count
  ```
- **`merchant map`** — Train the local merchant→category mapping for future auto-tag (one merchant at a time, or bulk from a CSV).

  _Lets the user (or agent) explicitly seed the auto-tag map without waiting for sync history to learn it._

  ```bash
  zoho-expense-pp-cli merchant map 'AWS' --category Software --project Engineering
  ```

### Monthly automation
- **`close`** — Bundle a month's unreported expenses into an expense report in one shot — flags still-processing autoscans, flags untagged items, creates the report, attaches, optionally submits.

  _The natural end of every monthly agent workflow — turn raw expenses into a submitted report without leaving the terminal._

  ```bash
  zoho-expense-pp-cli close --month 2026-10 --auto-submit --agent
  ```

### India tax workflow
- **`gst-split`** — India-specific: parse an expense's tax_id and line items, compute CGST/SGST (intra-state) or IGST (inter-state) shares, emit the breakdown or update the expense.

  _Turns a monthly export into something a CA can directly file, without the user re-doing the math by hand._

  ```bash
  zoho-expense-pp-cli gst-split exp_1234567890 --emit-csv
  ```

## Usage

Run `zoho-expense-pp-cli --help` for the full command reference and flag list.

## Commands

### currencies

Currencies and exchange rates

- **`zoho-expense-pp-cli currencies create`** - Add a currency to the org
- **`zoho-expense-pp-cli currencies delete`** - Delete a currency
- **`zoho-expense-pp-cli currencies get`** - Get a currency
- **`zoho-expense-pp-cli currencies list`** - List currencies configured in the org
- **`zoho-expense-pp-cli currencies update`** - Update a currency

### customers

Customers (contacts) expenses can be billed to

- **`zoho-expense-pp-cli customers create`** - Create a customer
- **`zoho-expense-pp-cli customers delete`** - Delete a customer
- **`zoho-expense-pp-cli customers get`** - Get a customer
- **`zoho-expense-pp-cli customers list`** - List customers
- **`zoho-expense-pp-cli customers update`** - Update a customer

### expense_categories

Expense categories used to classify expenses

- **`zoho-expense-pp-cli expense_categories create`** - Create an expense category
- **`zoho-expense-pp-cli expense_categories delete`** - Delete an expense category
- **`zoho-expense-pp-cli expense_categories disable`** - Disable a category
- **`zoho-expense-pp-cli expense_categories enable`** - Enable a category
- **`zoho-expense-pp-cli expense_categories get`** - Get an expense category
- **`zoho-expense-pp-cli expense_categories list`** - List expense categories
- **`zoho-expense-pp-cli expense_categories update`** - Update an expense category

### expense_reports

Expense reports — bundles of expenses submitted for approval and reimbursement

- **`zoho-expense-pp-cli expense_reports approval-history`** - View the approval history of a report
- **`zoho-expense-pp-cli expense_reports approve`** - Approve an expense report
- **`zoho-expense-pp-cli expense_reports create`** - Create an expense report
- **`zoho-expense-pp-cli expense_reports get`** - Get an expense report (includes attached expenses)
- **`zoho-expense-pp-cli expense_reports list`** - List expense reports
- **`zoho-expense-pp-cli expense_reports reimburse`** - Mark an expense report as reimbursed
- **`zoho-expense-pp-cli expense_reports reject`** - Reject an expense report
- **`zoho-expense-pp-cli expense_reports update`** - Update an expense report — used to attach more expenses

### expenses

Expenses — the primary entity for an expense management CLI

- **`zoho-expense-pp-cli expenses create`** - Create an expense (JSON body — for receipt upload use 'receipt upload')
- **`zoho-expense-pp-cli expenses get`** - Get a single expense
- **`zoho-expense-pp-cli expenses list`** - List expenses with rich filters (date range, status, user, category, project)
- **`zoho-expense-pp-cli expenses merge`** - Merge multiple expenses (used to dedupe a scanned-receipt expense with a manual one)
- **`zoho-expense-pp-cli expenses update`** - Update an expense — used to add category/project/tags after autoscan

### organizations

Zoho Expense organizations you have access to

- **`zoho-expense-pp-cli organizations get`** - Get organization details
- **`zoho-expense-pp-cli organizations list`** - List organizations accessible to the authenticated user

### projects

Projects expenses can be associated with

- **`zoho-expense-pp-cli projects activate`** - Mark a project active
- **`zoho-expense-pp-cli projects create`** - Create a project
- **`zoho-expense-pp-cli projects deactivate`** - Mark a project inactive
- **`zoho-expense-pp-cli projects delete`** - Delete a project
- **`zoho-expense-pp-cli projects get`** - Get a project
- **`zoho-expense-pp-cli projects list`** - List projects
- **`zoho-expense-pp-cli projects update`** - Update a project

### receipts

Upload receipts for autoscan

- **`zoho-expense-pp-cli receipts`** - Upload a receipt file (multipart). Server queues for autoscan; poll `expenses get` for status.

### reporting_tags

Reporting tags (custom tagging schema for expenses, e.g. cost center, billable, GST treatment)

- **`zoho-expense-pp-cli reporting_tags activate`** - Activate a reporting tag
- **`zoho-expense-pp-cli reporting_tags create`** - Create a reporting tag
- **`zoho-expense-pp-cli reporting_tags deactivate`** - Deactivate a reporting tag
- **`zoho-expense-pp-cli reporting_tags delete`** - Delete a reporting tag
- **`zoho-expense-pp-cli reporting_tags get`** - Get a reporting tag
- **`zoho-expense-pp-cli reporting_tags list`** - List reporting tags
- **`zoho-expense-pp-cli reporting_tags list-options`** - List all options for a reporting tag
- **`zoho-expense-pp-cli reporting_tags update`** - Update a reporting tag

### taxes

Taxes (GST in India) applied to expenses

- **`zoho-expense-pp-cli taxes create`** - Create a tax
- **`zoho-expense-pp-cli taxes delete`** - Delete a tax
- **`zoho-expense-pp-cli taxes get`** - Get a tax
- **`zoho-expense-pp-cli taxes list`** - List taxes
- **`zoho-expense-pp-cli taxes update`** - Update a tax

### trips

Business trips that group related expenses

- **`zoho-expense-pp-cli trips approve`** - Approve a trip
- **`zoho-expense-pp-cli trips cancel`** - Cancel a trip
- **`zoho-expense-pp-cli trips close`** - Close a trip (no further expenses can be added)
- **`zoho-expense-pp-cli trips create`** - Create a trip
- **`zoho-expense-pp-cli trips delete`** - Delete a trip
- **`zoho-expense-pp-cli trips get`** - Get a trip
- **`zoho-expense-pp-cli trips list`** - List trips
- **`zoho-expense-pp-cli trips reject`** - Reject a trip
- **`zoho-expense-pp-cli trips update`** - Update a trip

### users

Manage users in the organization

- **`zoho-expense-pp-cli users activate`** - Mark a user as active
- **`zoho-expense-pp-cli users deactivate`** - Mark a user as inactive
- **`zoho-expense-pp-cli users delete`** - Delete a user
- **`zoho-expense-pp-cli users get`** - Get a user
- **`zoho-expense-pp-cli users invite`** - Invite a user into the org
- **`zoho-expense-pp-cli users list`** - List users
- **`zoho-expense-pp-cli users me`** - Get the authenticated user's profile
- **`zoho-expense-pp-cli users update`** - Update a user

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
zoho-expense-pp-cli currencies list

# JSON for scripting and agents
zoho-expense-pp-cli currencies list --json

# Filter to specific fields
zoho-expense-pp-cli currencies list --json --select id,name,status

# Dry run — show the request without sending
zoho-expense-pp-cli currencies list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
zoho-expense-pp-cli currencies list --agent
```

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

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `ZOHO_EXPENSE_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `zoho-expense-pp-cli currencies`
- `zoho-expense-pp-cli currencies create`
- `zoho-expense-pp-cli currencies delete`
- `zoho-expense-pp-cli currencies get`
- `zoho-expense-pp-cli currencies list`
- `zoho-expense-pp-cli currencies update`
- `zoho-expense-pp-cli customers`
- `zoho-expense-pp-cli customers create`
- `zoho-expense-pp-cli customers delete`
- `zoho-expense-pp-cli customers get`
- `zoho-expense-pp-cli customers list`
- `zoho-expense-pp-cli customers update`
- `zoho-expense-pp-cli expense_categories`
- `zoho-expense-pp-cli expense_categories create`
- `zoho-expense-pp-cli expense_categories delete`
- `zoho-expense-pp-cli expense_categories disable`
- `zoho-expense-pp-cli expense_categories enable`
- `zoho-expense-pp-cli expense_categories get`
- `zoho-expense-pp-cli expense_categories list`
- `zoho-expense-pp-cli expense_categories update`
- `zoho-expense-pp-cli expense_reports`
- `zoho-expense-pp-cli expense_reports approval_history`
- `zoho-expense-pp-cli expense_reports approve`
- `zoho-expense-pp-cli expense_reports create`
- `zoho-expense-pp-cli expense_reports get`
- `zoho-expense-pp-cli expense_reports list`
- `zoho-expense-pp-cli expense_reports reimburse`
- `zoho-expense-pp-cli expense_reports reject`
- `zoho-expense-pp-cli expense_reports update`
- `zoho-expense-pp-cli expenses`
- `zoho-expense-pp-cli expenses create`
- `zoho-expense-pp-cli expenses get`
- `zoho-expense-pp-cli expenses list`
- `zoho-expense-pp-cli expenses merge`
- `zoho-expense-pp-cli expenses update`
- `zoho-expense-pp-cli organizations`
- `zoho-expense-pp-cli organizations get`
- `zoho-expense-pp-cli organizations list`
- `zoho-expense-pp-cli projects`
- `zoho-expense-pp-cli projects activate`
- `zoho-expense-pp-cli projects create`
- `zoho-expense-pp-cli projects deactivate`
- `zoho-expense-pp-cli projects delete`
- `zoho-expense-pp-cli projects get`
- `zoho-expense-pp-cli projects list`
- `zoho-expense-pp-cli projects update`
- `zoho-expense-pp-cli reporting_tags`
- `zoho-expense-pp-cli reporting_tags activate`
- `zoho-expense-pp-cli reporting_tags create`
- `zoho-expense-pp-cli reporting_tags deactivate`
- `zoho-expense-pp-cli reporting_tags delete`
- `zoho-expense-pp-cli reporting_tags get`
- `zoho-expense-pp-cli reporting_tags list`
- `zoho-expense-pp-cli reporting_tags list_options`
- `zoho-expense-pp-cli reporting_tags update`
- `zoho-expense-pp-cli taxes`
- `zoho-expense-pp-cli taxes create`
- `zoho-expense-pp-cli taxes delete`
- `zoho-expense-pp-cli taxes get`
- `zoho-expense-pp-cli taxes list`
- `zoho-expense-pp-cli taxes update`
- `zoho-expense-pp-cli trips`
- `zoho-expense-pp-cli trips approve`
- `zoho-expense-pp-cli trips cancel`
- `zoho-expense-pp-cli trips close`
- `zoho-expense-pp-cli trips create`
- `zoho-expense-pp-cli trips delete`
- `zoho-expense-pp-cli trips get`
- `zoho-expense-pp-cli trips list`
- `zoho-expense-pp-cli trips reject`
- `zoho-expense-pp-cli trips update`
- `zoho-expense-pp-cli users`
- `zoho-expense-pp-cli users activate`
- `zoho-expense-pp-cli users deactivate`
- `zoho-expense-pp-cli users delete`
- `zoho-expense-pp-cli users get`
- `zoho-expense-pp-cli users invite`
- `zoho-expense-pp-cli users list`
- `zoho-expense-pp-cli users me`
- `zoho-expense-pp-cli users update`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Health Check

```bash
zoho-expense-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/zoho-expense-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `ZOHO_EXPENSE_CLIENT_ID` | auth_flow_input | Yes | OAuth client ID from api-console.zoho.in self-client |
| `ZOHO_EXPENSE_CLIENT_SECRET` | auth_flow_input | Yes | Set during initial auth setup. |
| `ZOHO_EXPENSE_REFRESH_TOKEN` | harvested | Yes | Populated automatically by auth login. |
| `ZOHO_EXPENSE_ORGANIZATION_ID` | per_call | Yes | Zoho Expense organization ID (sent as X-com-zoho-expense-organizationid header on every request) |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `zoho-expense-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $ZOHO_EXPENSE_ORGANIZATION_ID`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **HTTP 401 'authentication value invalid'** — Run `zoho-expense-pp-cli auth refresh` — the access token expired (1h lifetime). If that fails, the refresh token may have rotated out (max 20 per user); re-run `auth login` with a fresh authorization code.
- **HTTP 404 / endpoint not found from a .com token** — Region mismatch. Your token was issued by accounts.zoho.com but you're targeting www.zohoapis.in. Override `base_url` in the config file or set ZOHO_EXPENSE_REGION accordingly.
- **Receipt upload returns 'autoscan_status: Failed'** — File format or size issue — max 5MB; JPG/PNG/PDF only. Multi-page PDFs scan but slower. Try re-saving the PDF as image-only and retry.
- **First expense for a new merchant has no category** — Expected — Zoho only auto-categorizes after the merchant has been seen + categorized once. Use `merchant map '<name>' --category ...` to seed the local mapping, then `expense-untagged --auto-fix`.
- **HTTP 429 rate-limited** — Zoho caps at 100 req/min per org. The CLI's adaptive limiter handles this automatically; if you see this surface, you're running parallel batches — drop concurrency.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**schmorrison/Zoho**](https://github.com/schmorrison/Zoho) — Go (39 stars)
- [**ramp-cli**](https://github.com/ramp-public/ramp-cli) — Python (28 stars)
- [**tdesposito/pyZohoAPI**](https://github.com/tdesposito/pyZohoAPI) — Python (8 stars)
- [**airbyte source-zoho-expense**](https://github.com/airbytehq/airbyte/pull/47406) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
