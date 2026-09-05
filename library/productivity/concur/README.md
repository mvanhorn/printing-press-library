# SAP Concur CLI

**Every expense-report and travel workflow Concur's web app offers, plus duplicate detection and real flight/hotel search no Concur tool has -- filed through the same session your browser already uses.**

SAP Concur's official API requires enterprise partner credentials most individual users can never get. This CLI defaults to your logged-in browser session instead, so filing expense reports and checking travel works the same day you install it. Local SQLite sync turns your report history into something you can search, join, and validate offline.

## Install

The recommended path installs both the `concur-pp-cli` binary and the `pp-concur` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install concur
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install concur --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install concur --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install concur --agent claude-code
npx -y @mvanhorn/printing-press-library install concur --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/accounting/concur/cmd/concur-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/concur-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install concur --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-concur --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-concur --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install concur --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
concur-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/concur-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/accounting/concur/cmd/concur-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "concur": {
      "command": "concur-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Concur's documented OAuth2 partner API is gated behind a Partner Enablement Manager relationship -- there is no self-serve signup, and this CLI does not implement that OAuth2 flow at all. Instead, this CLI authenticates via cookie/browser-session auth: run 'auth login --chrome', log into your company's Concur portal like you normally would (including SSO/MFA), and the CLI captures the resulting session. If a command fails with 401/403 and your company IT has partner OAuth2 credentials, that workflow requires calling the documented v3/v4 REST API directly (developer.concur.com) outside this CLI -- it is not something 'auth login' or any other command here can switch to.

### `hotels search` has a second, separate login by default

`hotels search` drives its own `agent-browser`-controlled Chrome instance (see HTTP Transport below), which does not share cookies with `auth login --chrome`'s source browser or credential store. Confirmed live that bridging them by copying cookies does not work -- Concur's bot-mitigation appears to bind the session to the browser/device that created it, not just the cookie value, so a copied JWT gets cleared by the server on the next navigation even when every cookie (including the Akamai bot-sensor ones) is copied alongside it. The first time (or whenever that session expires), `hotels search` opens its own Chrome window and asks you to log in there directly -- that login persists across later invocations until it expires again, so this is an occasional cost, not a per-search one.

**Optional one-time setup to avoid that second login entirely**: run a dedicated Chrome profile with remote debugging enabled and log into Concur there once. Use a real named profile (Chrome menu -> "Add Person", or chrome://settings -> Add profile) rather than a throwaway `--user-data-dir`, so `auth login --chrome --profile "<name>"` can read its cookies too:

```bash
"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \
  --remote-debugging-port=9222 --profile-directory="<profile dir name>"
```

`hotels search` auto-detects that session (tries CDP ports 9222, 9333, 9229 in order, or set `CONCUR_CDP_PORT` for a custom port) and *attaches* to it -- rather than copying its credentials -- before falling back to its own isolated login. Attaching, not copying, is what makes this work: it is the same live browser connection, so there is no separate device fingerprint for Concur's bot-mitigation to reject. Keep that Chrome window running whenever you plan to use `hotels search`; if you see "the dedicated Concur browser ... is no longer logged in", log in there again.

### Browser-automation fallback for report creation

On Concur tenants where custom policies are required, creating a report via pure HTTP can fail with `policyId is required` because the CLI has no way to query or supply policy IDs. To handle this, `reports create` has a transparent browser-automation fallback. When the HTTP API returns that specific validation error, the command will automatically spin up `agent-browser` (using the same Remote Debugging CDP attach or isolated login as described for `hotels search` above), drive the report creation modal, and seamlessly fetch the resulting report details to return to you in the standard CLI format. This fallback is completely automatic and conditional; tenants that do not require policy ID selection will continue to use the fast, dependency-free HTTP API path.

The fallback derives which Concur web UI host to open from your configured API base URL, but only when that URL is actually a `concursolutions.com` host -- it will never silently guess a region. If you're on a supported proxy or custom endpoint where that derivation can't work, set `CONCUR_UI_BASE_URL` to the correct region's UI host explicitly (e.g. `https://us2.concursolutions.com`); without a derivable host or this override, the command fails with a clear error rather than risking the wrong tenant/region. If the click succeeds in Concur but a later step fails, the error will say so explicitly (with the report ID, when known) instead of looking like an ordinary, safely-retryable failure -- re-running `reports create` blindly at that point risks creating a duplicate report, so check `reports get <id>` or `reports list` first.

## Quick Start

```bash
# Verify the binary and config are healthy before touching auth.
concur-pp-cli doctor --dry-run

# Capture your Concur session the same way you'd log in normally -- no API key needed.
concur-pp-cli auth login --chrome

# Get your Concur user ID once -- most commands need it via --user-id on every invocation (there's no built-in default-flag mechanism for it yet; export it as a shell variable to avoid retyping, e.g. USER_ID=$(concur-pp-cli account whoami --agent --select id --quiet)).
concur-pp-cli account whoami --agent

# See your existing expense reports.
concur-pp-cli reports list --user-id 550e8400-e29b-41d4-a716-446655440000 --agent

# Create a report the same way the web UI's 'Create Expense Report' button does.
concur-pp-cli reports create --name "October Travel" --purpose "Client site visit" --user-id 550e8400-e29b-41d4-a716-446655440000

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Conditional browser fallback for report creation
- **`reports create`** — Automatically and transparently retries report creation via automated browser when the Concur v4 API rejects pure HTTP requests with a `policyId is required` error. This fallback is completely conditional and only triggers for tenants requiring explicit policy assignment.

### Local state that compounds
- **`expenses scan-duplicates`** — Find potential double-entered charges across all of your synced expenses.

  _Run this before submitting a batch of reports if you suspect a corporate-card charge and a manually-entered cash expense might be the same transaction._

  ```bash
  concur-pp-cli expenses scan-duplicates --agent
  ```

### Live travel shopping (search only, never books)
- **`flights search`** — Real flight availability and fares from a live shopping session against your actual corporate-negotiated rates and travel policy -- not a public fare aggregator. One-way by default; `--return` includes both legs in the search but only renders the outbound leg (see `--help` for the known gap).

  _Use this to compare real options before requesting travel, with policy-compliance flags already applied per fare._

  ```bash
  concur-pp-cli flights search --from LAX --to "New York" --depart 2026-10-12 --yes --agent
  ```
- **`hotels search`** — Real hotel availability and rates via a live, policy-scoped search -- the same inventory and pricing Concur's own Hotel Search page shows. Drives a real browser (see HTTP Transport and Authentication below) rather than a direct API call, because the hotel shopping-session mutation is blocked from scripted replay by the tenant's bot-mitigation.

  _A one-time dedicated-browser setup (see Authentication) avoids a separate login every time this command's session expires._

  ```bash
  concur-pp-cli hotels search --to "New York" --check-in 2026-10-12 --check-out 2026-10-18 --yes --agent
  ```

## Recipes

### Check for duplicate charges

```bash
concur-pp-cli expenses scan-duplicates --agent
```

Scan the local SQLite cache for likely double-entered transactions across all your reports.

### Compare real flight and hotel options before requesting travel

```bash
concur-pp-cli flights search --from LAX --to "New York" --depart 2026-10-12 --yes --agent
concur-pp-cli hotels search --to "New York" --check-in 2026-10-12 --check-out 2026-10-18 --yes --agent
```

Both create a live shopping session against your real tenant -- searches only, never books. `flights search` is a direct API call; `hotels search` drives a real browser (see HTTP Transport and Authentication) and is markedly slower.

## Usage

Run `concur-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `CONCUR_CONFIG_DIR`, `CONCUR_DATA_DIR`, `CONCUR_STATE_DIR`, or `CONCUR_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `CONCUR_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export CONCUR_HOME=/srv/concur
concur-pp-cli doctor
```

Under `CONCUR_HOME=/srv/concur`, the four dirs resolve to `/srv/concur/config`, `/srv/concur/data`, `/srv/concur/state`, and `/srv/concur/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "concur": {
      "command": "concur-pp-mcp",
      "env": {
        "CONCUR_HOME": "/srv/concur"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `CONCUR_DATA_DIR` overrides an explicit `--home` for that kind. Use `CONCUR_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `CONCUR_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `concur-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### account

Current user profile, policies, and delegate context

- **`concur-pp-cli account travel <user_id>`** - Get the current user's travel profile and loyalty programs
- **`concur-pp-cli account whoami`** - Get the current user's profile, addresses, and travel IDs

### attendees

Attendee catalog and per-expense attendee associations

- **`concur-pp-cli attendees add`** - Add attendees to an expense (merge-preserves existing associations; Concur's underlying association call is a replace, so this reads first then re-POSTs the union)
- **`concur-pp-cli attendees list`** - Get attendees currently associated with an expense

### delegates

Delegate (act-on-behalf-of) relationships

- **`concur-pp-cli delegates`** - List users the current session user delegates for, with permission flags

### expense_types

Expense type catalog and per-type dynamic form fields

- **`concur-pp-cli expense-types list`** - List usable expense types for the current user's policy

### expenses

Expense line items within a report

- **`concur-pp-cli expenses create`** - Create an expense inside a report (core v3-equivalent fields: type, date, amount, currency, payment type)
- **`concur-pp-cli expenses get`** - Get a single expense with its filled/empty field manifest
- **`concur-pp-cli expenses update`** - Fill or change writable fields on an expense (core + custom/list fields)

### flights

Search flight locations, travel policy preferences, and real flight availability (creates a live shopping session -- searches only, never books)

- **`concur-pp-cli flights locations <query>`** - Resolve an airport, city, or metro name to Concur's travel location IDs; metro queries (e.g. "New York") resolve to one search endpoint covering all constituent airports
- **`concur-pp-cli flights preferences`** - Show your travel policy's flight search defaults
- **`concur-pp-cli flights search --from <origin> --to <dest> --depart <date> [--return <date>]`** - Search real flight availability and fares

### hotels

Search real hotel availability and rates (drives a real browser search -- searches only, never books)

- **`concur-pp-cli hotels search --to <destination> --check-in <date> --check-out <date>`** - Search real hotel availability and rates; requires `agent-browser` installed (see HTTP Transport and Authentication below)

### lists

Valid values for list-type expense form fields

- **`concur-pp-cli lists --list-id <id>`** - Get valid values for a list-type form field by list ID

### locations

Location catalog for filling expense/attendee location fields

- **`concur-pp-cli locations <query>`** - Search the location catalog by city or venue name

### payment_types

Payment type catalog (Cash, Company Card, etc.)

- **`concur-pp-cli payment-types`** - List payment types available to the current user

### receipts

Receipt image/PDF attachment

- **`concur-pp-cli receipts <expense_id> --file <path>`** - Attach a receipt image or PDF to an expense

### reports

Expense report headers and lifecycle

- **`concur-pp-cli reports create`** - Create a new expense report header
- **`concur-pp-cli reports get`** - Get a report's header, expenses, and web deep link
- **`concur-pp-cli reports list`** - List the current user's expense reports
- **`concur-pp-cli reports submit`** - Submit a report for approval
- **`concur-pp-cli reports update`** - Update a report's name or business purpose

### requests

Travel requests / pre-trip authorization (UNVERIFIED paths -- see spec header notes)

- **`concur-pp-cli requests get`** - Get a travel request's detail and workflow status
- **`concur-pp-cli requests list`** - List the current user's travel requests

### travel_allowance

Per-diem / travel allowance calculations (UNVERIFIED path -- see spec header notes)

- **`concur-pp-cli travel-allowance <trip_id>`** - Get travel allowance (per-diem) calculation results for a trip

### trips

Booked trips and itineraries (UNVERIFIED paths -- see spec header notes)

- **`concur-pp-cli trips get`** - Get a trip's itinerary detail
- **`concur-pp-cli trips list`** - List the current user's upcoming and past trips


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`concur-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`concur-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`concur-pp-cli learnings list`** - Inspect taught rows
- **`concur-pp-cli learnings forget <query>`** - Undo a teach
- **`concur-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`concur-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`concur-pp-cli teach-pattern`** - Install a query/resource template up front
- **`concur-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `CONCUR_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `concur-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
concur-pp-cli payment-types

# JSON for scripting and agents
concur-pp-cli payment-types --json
# Filter to specific fields
concur-pp-cli payment-types --json --select paymentTypeId,paymentTypeName,description

# Dry run — show the request without sending
concur-pp-cli payment-types --dry-run

# Agent mode — JSON + compact + no prompts in one flag
concur-pp-cli payment-types --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
concur-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `concur-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/concur-pp-cli/config.toml`; `--home`, `CONCUR_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `concur-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **auth login --chrome captures no cookies or the session expires quickly** — Install press-auth (go install github.com/mvanhorn/cli-printing-press/v4/cmd/press-auth@latest) for a more reliable capture: press-auth login concursolutions.com --login-url https://www.concursolutions.com/ --jwt-carrier-cookie JWT
- **reports submit fails with 'Missing required field: Business Purpose'** — Auto-fill known-rule expense types via 'concur-pp-cli expenses apply-rules <report-id> --user-id <id> --config expense_types.json'; run it with --dry-run first to preview the changes without writing anything.
- **every command wants --user-id and I don't want to retype my own GUID constantly** — This CLI has no built-in default-flag mechanism for --user-id yet ('profile save' only captures global output flags like --json, not per-command flags). Export it as a shell variable instead. First capture your ID: `USER_ID=$(concur-pp-cli account whoami --agent --select id --quiet)`. Then pass it on other commands: `--user-id "$USER_ID"`.
- **commands fail with 401/403 against reports or expenses endpoints** — Your company's Concur tenant may route those calls through the OAuth2 partner API instead of the cookie-authenticated path this CLI uses by default. This CLI does not implement the OAuth2 partner flow; if your company IT has partner credentials, use the documented v3/v4 REST API directly (developer.concur.com) for that workflow instead.
- **`hotels search` keeps opening its own Chrome window and asking me to log in, separately from `auth login --chrome`** — Expected: it drives a different, isolated browser instance and cannot share credentials with `auth login --chrome`'s source browser (copying cookies between them was tried and confirmed not to work -- see Authentication above). The login persists across later invocations until that session expires, so this is occasional, not per-search. To avoid it entirely, set up a dedicated debug-enabled Chrome profile once (see Authentication above); `hotels search` auto-detects and attaches to it instead of opening its own.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

**Exception: `hotels search`.** Confirmed live that Concur's hotel shopping-session mutation is blocked from scripted HTTP replay by the tenant's bot-mitigation (byte-for-byte replay of a request that had just succeeded natively in the browser still failed). So this one command drives a real browser via `agent-browser` instead (`npm install -g agent-browser && agent-browser install`) -- `flights search` and every other command remain pure HTTP; only the hotel-shopping mutation needs a real browser.

TLS certificates are verified by default. For a trusted development or self-signed endpoint only, pass `--insecure` for one invocation, set `CONCUR_SKIP_TLS_VERIFY=true` for the current environment, or set `skip_tls_verify = true` in the config file for a persistent override.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**sap-concur-mcp-server-by-cdata**](https://github.com/CDataSoftware/sap-concur-mcp-server-by-cdata) — Java (2 stars)
- [**sap-concur-connector**](https://github.com/Tevasoft/sap-concur-connector) — Python (1 stars)
- [**sap-concur-browser-automation**](https://github.com/Nilesunknowing346/sap-concur-browser-automation) — JavaScript (1 stars)
- [**concur-mcp**](https://github.com/bharath2020/concur-mcp) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
