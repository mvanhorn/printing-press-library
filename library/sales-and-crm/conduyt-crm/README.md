# Conduyt CRM CLI

**Full-coverage Conduyt CLI: every endpoint, plus Kloudi-grade send-safety forensics no other surface has.**

Full coverage of the Conduyt CRM API with a local SQLite store, offline search, and agent-native output. On top of the mirror: send-check, imports blame, imports watch, reports scorecard, reports compare, automations hours-audit and dialer coverage — the commands that answer 'is this list safe to text?', 'what did that import actually do?', 'who is on pace?' and 'what will the dialer hand out next?' in one call.

Learn more at [Conduyt CRM](https://conduyt.app).

Created by [@ptaramona](https://github.com/ptaramona) (Conduyt).
Contributors: [@cathrynlavery](https://github.com/cathrynlavery) (Cathryn Lavery).

## Install

The recommended path installs both the `conduyt-crm-pp-cli` binary and the `pp-conduyt-crm` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install conduyt-crm
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install conduyt-crm --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install conduyt-crm --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install conduyt-crm --agent claude-code
npx -y @mvanhorn/printing-press-library install conduyt-crm --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/conduyt-crm-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install conduyt-crm --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-conduyt-crm --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-conduyt-crm --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install conduyt-crm --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/conduyt-crm-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `CONDUYT_CRM_BEARER_AUTH` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS, `%APPDATA%\Claude\claude_desktop_config.json` on Windows):

```json
{
  "mcpServers": {
    "conduyt-crm": {
      "command": "conduyt-crm-pp-mcp",
      "env": {
        "CONDUYT_CRM_BEARER_AUTH": "<your-key>"
      }
    }
  }
}
```

</details>

### Remote MCP over HTTP

`conduyt-crm-pp-mcp --transport http` (or `PP_MCP_TRANSPORT=http`) serves streamable HTTP at `http://127.0.0.1:7777/mcp`. It refuses to start until `CONDUYT_CRM_MCP_HTTP_TOKEN` is set to at least 32 characters, and every request must send `Authorization: Bearer <token>`. The command below prints 64 hex characters and meets that minimum. Plain HTTP is allowed only on a loopback `--addr`; any other bind, including `:7777` (every interface), also needs `--tls-cert` and `--tls-key`. Requests with no `Origin` header are accepted, as are loopback origins (`localhost`, `127.0.0.1`, or `[::1]`, any port). Any other `Origin` is refused unless it is listed in `CONDUYT_CRM_MCP_HTTP_ALLOWED_ORIGINS`, a comma-separated list of `scheme://host[:port]` values with no path. That list is not a CORS policy; the server does not emit CORS headers.

```bash
export CONDUYT_CRM_MCP_HTTP_TOKEN="$(openssl rand -hex 32)"
conduyt-crm-pp-mcp --transport http
```

Set `CONDUYT_CRM_MCP_READ_ONLY=1` (exactly `1`) to drop mirrored commands that are not marked read-only and make the raw execute tool refuse state-changing calls. Commands marked local-write, which write only this CLI's own learn store, stay available.

## Quick Start

```bash
# Verify your cdy_ API key and connectivity first
conduyt-crm-pp-cli doctor

# Pull contacts, deals, and imports into the local store
conduyt-crm-pp-cli sync --full

# The anti-Kloudi gate: is this segment safe to message?
conduyt-crm-pp-cli send-check --tag new-leads

# What the SMS provider did with your drips, and why
conduyt-crm-pp-cli reports get-sms-delivery --json

```bash
echo "$TOKEN" | conduyt-crm-pp-cli auth set-token
```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Kloudi-grade send safety
- **`send-check`** — Go/no-go verdict on a list, tag, or contact before any SMS or drip: phone present, Twilio-verified line type, Twilio validity, and DNC status in one table (see imports blame for an import's side effects).

  _Run it before acting on any 'text this list' instruction — it is the anti-Kloudi gate._

  ```bash
  conduyt-crm-pp-cli send-check --tag kloudi-july --agent
  ```
- **`imports blame`** — For one import job: every error row grouped by reason, plus the job-scoped side effects (queued, retryable, exhausted, held).

  _Answers 'which rows of the Tuesday import failed, and are its side effects still queued or held?' in one call after any bulk push._

  ```bash
  conduyt-crm-pp-cli imports blame 3f2a9c1e-0000-0000-0000-000000000000 --json
  ```
- **`imports watch`** — Polls an import job to completion, then renders the row-outcome recap (created/updated/skipped/errors) and, with --verify, checks its job-scoped side effects.

  _Drop it into n8n or cron and alert on bad imports instead of discovering them hours later._

  ```bash
  conduyt-crm-pp-cli imports watch 3f2a9c1e-0000-0000-0000-000000000000 --verify
  ```
- **`contacts verify-line-type`** — Counts unverified contacts and prints the tenant-paid Twilio cost without spending (--estimate); 'run --until-done' with one scope (--smart-list or --contact-ids) drives the batched verification to done. Requires the account's Phone Line-Type Verification setting (403 'not enabled' otherwise).

  _Know the dollar exposure before spending it; then run 'contacts verify-line-type run --until-done --smart-list <uuid>' (or --contact-ids <uuid,...>) to verify._

  ```bash
  conduyt-crm-pp-cli contacts verify-line-type --smart-list 7c1d2e3f-0000-0000-0000-000000000000 --estimate
  ```
- **`drips audit`** — Flags duplicate sends in a window: any contact who received the same message body more than once, from the local store.

  _Proves a drip cohort fired exactly once per contact — or names the exceptions._

  ```bash
  conduyt-crm-pp-cli drips audit 9b8c7d6e-0000-0000-0000-000000000000 --json
  ```

### Reports as commands
- **`reports scorecard`** — Target versus actual per user and metric with attainment percent and pace to the end of the period.

  _Pick it when asked who is on pace, off pace, or how far from target a rep is._

  ```bash
  conduyt-crm-pp-cli reports scorecard --period month --agent
  ```
- **`reports compare`** — Any core report called for a window and the equal prior window, with prior, current, delta and delta percent on every numeric value. `--assigned-to` applies to speed-to-lead, appointments and call-performance; `--pipeline` to funnel.

  _Use it to explain why a number moved week over week without pulling both reports by hand._

  ```bash
  conduyt-crm-pp-cli reports compare speed-to-lead --range 7d --json
  ```
- **`automations hours-audit`** — Every Send SMS, Send Email and Assign step across synced automations, listed with or without an operating-hours window. With `--published-only`, `summary.unpublished` counts automations with no published audited actions.

  _Use it to confirm quiet-hours coverage after a publish or when compliance asks which steps can fire after hours._

  ```bash
  conduyt-crm-pp-cli automations hours-audit --published-only --json
  ```

### Dialer operations
- **`dialer coverage`** — Priorities in dial order with queue depth per priority and the count of available agents; empty priorities are listed under empty_priorities with a warning (exit 0).

  _Run it before the floor starts to see what Smart Dialing will hand out next and where a priority has run dry._

  ```bash
  conduyt-crm-pp-cli dialer coverage --limit 25 --agent
  ```

### Local state that compounds
- **`analytics`** — GROUP BY over synced entities — segment counts without paging the API (--type/--group-by/--limit).

  _The fast path for 'how many contacts are X' questions agents ask constantly._

  ```bash
  conduyt-crm-pp-cli analytics --type contacts --group-by lineType --agent
  ```
- **`tail`** — Interval polling over a resource that re-emits current results as JSON — watch messages or activities as a blast goes out.

  _Confirm automation side effects and outbound sends as they happen._

  ```bash
  conduyt-crm-pp-cli tail messages --interval 5s --agent
  ```

## Recipes

### Kloudi-safe bulk import

```bash
conduyt-crm-pp-cli imports watch "$JOB" --verify
```

Watch an import job to completion, then render the outcome recap and verify the job-scoped side effects, with typed exit codes (preflight the file first via imports post-preflight --stdin).

### Blame a bad blast

```bash
conduyt-crm-pp-cli imports blame "$JOB" --agent --select row_outcomes,errors_by_reason
```

Group an import's error rows by reason and read its job-scoped side effects (queued, retryable, exhausted, held).

### Segment safety audit

```bash
conduyt-crm-pp-cli send-check --tag q3-cohort --agent --select summary
```

Bounded agent-shaped go/no-go summary; drill in with --select verdicts.contact_id,verdicts.verdict.

### Why did speed to lead move?

```bash
conduyt-crm-pp-cli reports compare speed-to-lead --range 7d --agent --select rows.metric,rows.prior,rows.current,rows.delta_pct
```

The equal prior window is computed for you and every numeric value carries its delta.

### Who is on pace this month?

```bash
conduyt-crm-pp-cli reports scorecard --period month --agent --select rows.user_id,rows.metric,rows.target,rows.actual,rows.attainment_pct
```

Targets joined to agent performance, narrowed to the columns an agent needs.

### What will the dialer hand out next?

```bash
conduyt-crm-pp-cli dialer coverage --agent
```

Priorities in dial order with queue depth and available agents.

## Usage

Run `conduyt-crm-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `CONDUYT_CRM_CONFIG_DIR`, `CONDUYT_CRM_DATA_DIR`, `CONDUYT_CRM_STATE_DIR`, or `CONDUYT_CRM_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `CONDUYT_CRM_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export CONDUYT_CRM_HOME=/srv/conduyt-crm
conduyt-crm-pp-cli doctor
```

Under `CONDUYT_CRM_HOME=/srv/conduyt-crm`, the four dirs resolve to `/srv/conduyt-crm/config`, `/srv/conduyt-crm/data`, `/srv/conduyt-crm/state`, and `/srv/conduyt-crm/cache`.

On Windows, set the same variable in PowerShell:

```powershell
$env:CONDUYT_CRM_HOME = "C:\srv\conduyt-crm"
conduyt-crm-pp-cli doctor
```

Under `$env:CONDUYT_CRM_HOME = "C:\srv\conduyt-crm"`, the four dirs resolve to `C:\srv\conduyt-crm\config`, `C:\srv\conduyt-crm\data`, `C:\srv\conduyt-crm\state`, and `C:\srv\conduyt-crm\cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "conduyt-crm": {
      "command": "conduyt-crm-pp-mcp",
      "env": {
        "CONDUYT_CRM_HOME": "/srv/conduyt-crm"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `CONDUYT_CRM_DATA_DIR` overrides an explicit `--home` for that kind. Use `CONDUYT_CRM_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `CONDUYT_CRM_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `conduyt-crm-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

### Local data

Reads are saved locally by default: in `--data-source auto` mode every successful read is also written to the SQLite store (`data.db`, or `data-<hash>.db` per credential) in the data directory. GET responses are cached for five minutes under `http/` in the cache directory. Credentials live in `credentials.toml` in the data directory. The learn journal lives under `learn/` in the state directory. `conduyt-crm-pp-cli doctor` prints the resolved paths, every store file, and the response-cache size.

None of it is encrypted. New files are created owner-only; beyond that the data relies on OS account permissions and full-disk encryption (FileVault, BitLocker, LUKS).

- **Erase it:** `conduyt-crm-pp-cli local-data purge` lists the store files, response cache, and learn journal it would delete; add `--yes` to delete them, or `--only store|cache|journal` to narrow it. Config and credentials are kept.
- **Run without a local copy:** set `CONDUYT_CRM_NO_LOCAL_DATA=1`. Reads go to the API (`--data-source live`; `--data-source local` still reads an existing store), the response cache is off (`--no-cache`), nothing new is written to the store, and `sync` and `workflow archive` refuse to run. Set it in an MCP host's `env` block and every command the server runs inherits it.
- **Choose the read source without flags:** `CONDUYT_CRM_DATA_SOURCE=auto|live|local` sets the `--data-source` default; an explicit flag wins.

## Commands

### account

Manage account

- **`conduyt-crm-pp-cli account get-readiness`** - List account readiness
- **`conduyt-crm-pp-cli account post-company-name`** - The one screen after a one-click Google or Microsoft sign-up ("What's your company called?"). Sets the account name and clears the pending flag the sign-up set (GET /auth/me answers account.needsCompanyName), in one conditional write; an account that owes no name is a 409 company_name_not_pending (rename a workspace through PATCH /settings companyName). Owner or admin, interactive browser session only (an API key is a 403). The name rules are the register route's, with its field and code: a link is name_contains_link, more than 120 characters is account_name_too_long, fewer than 2 after the trim is account_name_too_short (400, field accountName).

### account-exports

Manage account exports

- **`conduyt-crm-pp-cli account-exports get`** - List account-exports
- **`conduyt-crm-pp-cli account-exports post`** - Create / invoke account-exports

### accounts

Manage accounts

- **`conduyt-crm-pp-cli accounts get`** - List accounts
- **`conduyt-crm-pp-cli accounts post`** - Create / invoke accounts

### activities

Activity feed and logging

- **`conduyt-crm-pp-cli activities create-activity`** - Log an activity
- **`conduyt-crm-pp-cli activities get-assignment-reasons`** - List activities assignment-reasons
- **`conduyt-crm-pp-cli activities get-id`** - Get activities id
- **`conduyt-crm-pp-cli activities list`** - List activities

### activity

Manage activity

- **`conduyt-crm-pp-cli activity`** - Streams the activities the activity list would answer for the same filters (the same visibility, deal fence and soft-deleted-parent rules as GET /activities) as CSV, newest first by createdAt then id. Columns: id, type, description, user, contact, deal, timestamp. An export from this endpoint holds at most 100,000 rows AND 50 MB of CSV: more matching rows are refused 413 export_too_large before any row is written (narrow the date range or filters, or export every matching row in the background: POST /account-exports with entities ["activity"] and the same filters as activityFilters). One request streams within a 55 s deadline that also covers the initial count. The LAST line of every response is a trailer: `#conduyt-export,complete,<rows written by the whole export>` when every row was written; `#conduyt-export,continue,<cursor>` when the deadline came: request again with ?cursor=<cursor> and the SAME filters and append that part, which has no header row (the cursor is signed and bound to the account, the filters and the ceiling; any other cursor is a 400 cursor_invalid); `#conduyt-export,too_large,<rows|bytes>` when the ceiling was reached while streaming: discard everything. A response without a trailer was cut: discard it.

### admin

Super-admin account management and impersonation

- **`conduyt-crm-pp-cli admin clean-test-accounts`** - Delete test accounts
- **`conduyt-crm-pp-cli admin clean-test-data`** - Clean test data from the system
- **`conduyt-crm-pp-cli admin impersonate`** - Impersonate a user (super-admin)
- **`conduyt-crm-pp-cli admin list-accounts`** - (#612) A page of accounts, newest first (page / per_page, default 50, max 200; limit is an alias of per_page), with the total that match q and status. (#620) meta.summary carries the total / comp / trialing / paid counts over the whole filtered set on every answer. `status=ending_soon` is trial users whose trial ends within 7 days or has ended. A malformed or conflicting query parameter is a 400. Each row carries the columns the admin console needs. `status=trialing` is the trial users listing: it leaves comped and test accounts out. (#518) `isTest` is read-only: it is set only by registration's test-account rule, which needs both an e2e+ address on conduyt.com, conduyt.app or brightmarketingsolutions.com and the harness secret (TEST_ACCOUNT_SIGNUP_SECRET, sent as the conduyt_test_signup cookie or the x-conduyt-test-signup header), and no route accepts it.
- **`conduyt-crm-pp-cli admin stop-impersonate`** - Stop impersonating
- **`conduyt-crm-pp-cli admin toggle-comp`** - Toggle comp (free) status for an account

### ai

AI-powered features (chat, email compose, contact enrichment)

- **`conduyt-crm-pp-cli ai chat`** - AI chat assistant
- **`conduyt-crm-pp-cli ai compose-email`** - AI-assisted email composition
- **`conduyt-crm-pp-cli ai deal-insights`** - AI-generated deal insights and recommendations
- **`conduyt-crm-pp-cli ai enrich-contact`** - AI-powered contact data enrichment
- **`conduyt-crm-pp-cli ai get-actions-runs-id`** - Get ai actions runs id
- **`conduyt-crm-pp-cli ai get-agenda`** - List ai agenda
- **`conduyt-crm-pp-cli ai get-daily-brief`** - List ai daily-brief
- **`conduyt-crm-pp-cli ai get-feed`** - List ai feed
- **`conduyt-crm-pp-cli ai get-insights`** - List ai insights
- **`conduyt-crm-pp-cli ai get-next-actions`** - List ai next-actions
- **`conduyt-crm-pp-cli ai get-usage`** - List ai usage
- **`conduyt-crm-pp-cli ai improve-email`** - AI-assisted email improvement
- **`conduyt-crm-pp-cli ai post-actions`** - Create / invoke ai actions
- **`conduyt-crm-pp-cli ai post-actions-execute`** - Create / invoke ai actions execute
- **`conduyt-crm-pp-cli ai post-conversation-intelligence`** - Create / invoke ai conversation-intelligence
- **`conduyt-crm-pp-cli ai post-insights`** - Create / invoke ai insights
- **`conduyt-crm-pp-cli ai post-tasks`** - Create / invoke ai tasks
- **`conduyt-crm-pp-cli ai summarize-contact`** - AI-generated contact summary

### api-keys

API key management

- **`conduyt-crm-pp-cli api-keys check`** - Returns tier, scopes, rate limit, IP allowlist, and usage for the API key used on the request.
- **`conduyt-crm-pp-cli api-keys create`** - Returns the full key exactly once. Store it securely.
- **`conduyt-crm-pp-cli api-keys get-id`** - Get api-keys id
- **`conduyt-crm-pp-cli api-keys list`** - Returns API key metadata. Never returns the full key.
- **`conduyt-crm-pp-cli api-keys revoke`** - Revoke an API key
- **`conduyt-crm-pp-cli api-keys update`** - Update key name, tier, scopes, IP allowlist, rate limit, and optional expiry. API-key auth cannot manage API keys.

### api_search

Manage api search

- **`conduyt-crm-pp-cli api-search`** - Global search across contacts, companies, and deals

### appointments

Appointment scheduling

- **`conduyt-crm-pp-cli appointments create`** - Create an appointment
- **`conduyt-crm-pp-cli appointments delete`** - Delete an appointment
- **`conduyt-crm-pp-cli appointments get`** - Get an appointment by ID
- **`conduyt-crm-pp-cli appointments list`** - List all appointments
- **`conduyt-crm-pp-cli appointments update`** - Update an appointment

### automation-executions

Automation execution logs and step details

- **`conduyt-crm-pp-cli automation-executions get`** - Get execution details by ID
- **`conduyt-crm-pp-cli automation-executions list`** - List automation execution logs

### automation-folders

Manage automation folders

- **`conduyt-crm-pp-cli automation-folders delete-id`** - Delete automation-folders id
- **`conduyt-crm-pp-cli automation-folders get`** - List automation-folders
- **`conduyt-crm-pp-cli automation-folders patch-id`** - Update automation-folders id
- **`conduyt-crm-pp-cli automation-folders post`** - Create / invoke automation-folders

### automations

Workflow automations (native + n8n), publishing, analytics

- **`conduyt-crm-pp-cli automations create`** - Create an automation
- **`conduyt-crm-pp-cli automations create-from-template`** - Create automation from a template
- **`conduyt-crm-pp-cli automations delete`** - Delete an automation
- **`conduyt-crm-pp-cli automations get`** - Get an automation by ID
- **`conduyt-crm-pp-cli automations get-distribution-steps`** - List automations distribution-steps
- **`conduyt-crm-pp-cli automations get-failures`** - List automations failures
- **`conduyt-crm-pp-cli automations get-failures-drilldown`** - List automations failures drilldown
- **`conduyt-crm-pp-cli automations get-resolve`** - List automations resolve
- **`conduyt-crm-pp-cli automations get-schema`** - List automations schema
- **`conduyt-crm-pp-cli automations get-step-logs`** - List automations step-logs
- **`conduyt-crm-pp-cli automations list`** - List automations
- **`conduyt-crm-pp-cli automations list-actions`** - List available automation actions
- **`conduyt-crm-pp-cli automations list-condition-fields`** - List available condition fields for triggers
- **`conduyt-crm-pp-cli automations list-events`** - List available trigger events
- **`conduyt-crm-pp-cli automations list-templates`** - List automation templates
- **`conduyt-crm-pp-cli automations post-copilot`** - Create / invoke automations copilot
- **`conduyt-crm-pp-cli automations post-import`** - Create / invoke automations import
- **`conduyt-crm-pp-cli automations post-resolve`** - Create / invoke automations resolve
- **`conduyt-crm-pp-cli automations post-validate`** - Create / invoke automations validate
- **`conduyt-crm-pp-cli automations test-webhook`** - Send a test payload to an automation's webhook URL
- **`conduyt-crm-pp-cli automations update`** - Update an automation

### availability

Manage availability

- **`conduyt-crm-pp-cli availability get`** - Get current user's availability rules
- **`conduyt-crm-pp-cli availability set`** - Set availability rules

### batch-operations

Manage batch operations

- **`conduyt-crm-pp-cli batch-operations get`** - List batch-operations
- **`conduyt-crm-pp-cli batch-operations get-id`** - Get batch-operations id
- **`conduyt-crm-pp-cli batch-operations post`** - Create / invoke batch-operations

### billing

Stripe billing, checkout, and subscription status

- **`conduyt-crm-pp-cli billing create-checkout-session`** - Create a Stripe checkout session
- **`conduyt-crm-pp-cli billing create-portal`** - Create a Stripe billing portal session
- **`conduyt-crm-pp-cli billing delete-addons`** - Delete billing addons
- **`conduyt-crm-pp-cli billing get-addons`** - List billing addons
- **`conduyt-crm-pp-cli billing get-gate`** - List billing gate
- **`conduyt-crm-pp-cli billing get-status`** - Get subscription status
- **`conduyt-crm-pp-cli billing patch-addons`** - Update billing addons
- **`conduyt-crm-pp-cli billing post-addons`** - Create / invoke billing addons
- **`conduyt-crm-pp-cli billing webhook-stripe`** - Stripe billing webhook

### booking-pages

Public booking pages (Calendly-style)

- **`conduyt-crm-pp-cli booking-pages create`** - Create a booking page
- **`conduyt-crm-pp-cli booking-pages delete`** - Delete a booking page
- **`conduyt-crm-pp-cli booking-pages get`** - Get a booking page by ID
- **`conduyt-crm-pp-cli booking-pages list`** - List booking pages
- **`conduyt-crm-pp-cli booking-pages update`** - Update a booking page

### booking-routing-forms

Manage booking routing forms

- **`conduyt-crm-pp-cli booking-routing-forms delete-id`** - Delete booking-routing-forms id
- **`conduyt-crm-pp-cli booking-routing-forms get`** - List booking-routing-forms
- **`conduyt-crm-pp-cli booking-routing-forms get-id`** - Get booking-routing-forms id
- **`conduyt-crm-pp-cli booking-routing-forms patch-id`** - Update booking-routing-forms id
- **`conduyt-crm-pp-cli booking-routing-forms post`** - Create / invoke booking-routing-forms

### bulk

Manage bulk

- **`conduyt-crm-pp-cli bulk delete-contacts`** - Bulk delete contacts
- **`conduyt-crm-pp-cli bulk edit-contacts`** - Bulk edit contact fields
- **`conduyt-crm-pp-cli bulk edit-deals`** - Bulk edit deal fields
- **`conduyt-crm-pp-cli bulk get-status`** - Get bulk operation status
- **`conduyt-crm-pp-cli bulk post-contacts-delete-matching`** - Create / invoke bulk contacts delete-matching
- **`conduyt-crm-pp-cli bulk post-contacts-dnc`** - Create / invoke bulk contacts dnc
- **`conduyt-crm-pp-cli bulk post-contacts-untag`** - Create / invoke bulk contacts untag
- **`conduyt-crm-pp-cli bulk tag-contacts`** - Bulk add/remove tags on contacts
- **`conduyt-crm-pp-cli bulk update-contacts`** - Bulk update contacts with field values
- **`conduyt-crm-pp-cli bulk update-deals`** - Updates up to 100 deals in one request. Each item in `updates` carries the deal `id` plus the fields to change for THAT deal, and each is applied on its own: one item failing never blocks the others. A valid batch of 1 to 100 items answers 200 with a per-item outcome for every item; authentication, billing, request-shape, rate-limit and server failures use their documented top-level statuses instead. Rate limited to 5 requests per minute per account and IP. Length limits count UTF-16 code units as JavaScript does, so a character outside the Basic Multilingual Plane such as an emoji counts 2; JSON Schema counts it 1, so a string near the limit can pass this schema and still be refused per item.

### bulk-operations

Bulk update, delete, and tag contacts and deals

- **`conduyt-crm-pp-cli bulk-operations`** - List bulk-operations unified

### calendar

Internal calendar and appointment management

- **`conduyt-crm-pp-cli calendar connect-google`** - Initiate Google Calendar OAuth
- **`conduyt-crm-pp-cli calendar connect-microsoft`** - Initiate Microsoft Calendar OAuth
- **`conduyt-crm-pp-cli calendar create-event`** - Create an event on a connected calendar
- **`conduyt-crm-pp-cli calendar delete-connection`** - Disconnect a calendar
- **`conduyt-crm-pp-cli calendar delete-event`** - Delete a synced calendar event
- **`conduyt-crm-pp-cli calendar get-connection`** - Get a calendar connection by ID
- **`conduyt-crm-pp-cli calendar get-event`** - Get a synced calendar event
- **`conduyt-crm-pp-cli calendar get-health`** - List calendar health
- **`conduyt-crm-pp-cli calendar list-connections`** - List calendar connections
- **`conduyt-crm-pp-cli calendar list-events`** - List synced calendar events
- **`conduyt-crm-pp-cli calendar sync`** - Trigger manual calendar sync
- **`conduyt-crm-pp-cli calendar update-event`** - Update a synced calendar event
- **`conduyt-crm-pp-cli calendar webhook-google`** - Google Calendar push notification webhook
- **`conduyt-crm-pp-cli calendar webhook-microsoft`** - Microsoft Calendar webhook

### calendars

Internal calendar and appointment management

- **`conduyt-crm-pp-cli calendars create`** - Create a calendar
- **`conduyt-crm-pp-cli calendars get`** - Get a calendar by ID
- **`conduyt-crm-pp-cli calendars list`** - List internal calendars
- **`conduyt-crm-pp-cli calendars update`** - Update a calendar

### call-flows

Manage call flows

- **`conduyt-crm-pp-cli call-flows delete-functions-id`** - Delete call-flows functions id
- **`conduyt-crm-pp-cli call-flows delete-id`** - Delete call-flows id
- **`conduyt-crm-pp-cli call-flows get`** - List call-flows
- **`conduyt-crm-pp-cli call-flows get-functions`** - List call-flows functions
- **`conduyt-crm-pp-cli call-flows get-functions-id`** - Get call-flows functions id
- **`conduyt-crm-pp-cli call-flows get-id`** - Get call-flows id
- **`conduyt-crm-pp-cli call-flows get-numbers`** - List call-flows numbers
- **`conduyt-crm-pp-cli call-flows get-roster`** - List call-flows roster
- **`conduyt-crm-pp-cli call-flows patch-functions-id`** - Update call-flows functions id
- **`conduyt-crm-pp-cli call-flows patch-id`** - Update call-flows id
- **`conduyt-crm-pp-cli call-flows post`** - Create / invoke call-flows
- **`conduyt-crm-pp-cli call-flows post-audio`** - Create / invoke call-flows audio
- **`conduyt-crm-pp-cli call-flows post-functions`** - Create / invoke call-flows functions
- **`conduyt-crm-pp-cli call-flows post-functions-id-test`** - Create / invoke call-flows functions id test
- **`conduyt-crm-pp-cli call-flows post-tombstones-release`** - Create / invoke call-flows tombstones release

### calls

Call log management

- **`conduyt-crm-pp-cli calls count-unknown-callers`** - Board #35. `total`: inbound calls to that line since `since` that arrived with no contact (the ones still contact-less plus the ones the unknown-caller prompt linked since); `unmatched`: those still contact-less (list them with GET /dialer/history?unmatched=1&line=...). `since` is the caller's start of day and must be within the last 31 days. Requires `dialer.view`; counts only, under the Call history visibility rule (a scoped member counts their own calls, contact-less calls and calls linked to a contact they can see).
- **`conduyt-crm-pp-cli calls create-record`** - Create a call record
- **`conduyt-crm-pp-cli calls get`** - Get a call by ID
- **`conduyt-crm-pp-cli calls link call-to-contact`** - Board #35, the unknown-caller prompt's 'Search a contact'. `calls link call-to-contact <id> --contact-id <uuid> [--attach-number] [--server-dry-run]` links a contact-less call to a contact (compare-and-set; already linked to this contact answers 200 `alreadyLinked`; linked to another contact is 409 `call_already_linked`). `--attach-number` also writes the call's number into the contact's next free phone slot. `--server-dry-run` sends the API's body `dryRun: true` (the server previews and writes nothing; the request is sent); the global `--dry-run` prints the request and sends nothing. The `calls` group is hidden from the top-level help, but this command works as written.
- **`conduyt-crm-pp-cli calls list`** - List call records
- **`conduyt-crm-pp-cli calls post-voice-drop`** - Create / invoke calls voice-drop
- **`conduyt-crm-pp-cli calls update`** - Update a call record (e.g., add notes)

### chat

Internal team chat channels and messages

- **`conduyt-crm-pp-cli chat add-member`** - Add a member to a channel
- **`conduyt-crm-pp-cli chat add-reaction`** - Add a reaction to a message
- **`conduyt-crm-pp-cli chat create-channel`** - Create a chat channel
- **`conduyt-crm-pp-cli chat delete-agents-id`** - Delete chat agents id
- **`conduyt-crm-pp-cli chat delete-agents-id-webhook`** - Delete chat agents id webhook
- **`conduyt-crm-pp-cli chat delete-agents-me-mcp`** - The server is stateless (no session id), so DELETE answers 405 with `Allow: POST` and writes nothing.
- **`conduyt-crm-pp-cli chat delete-message`** - Delete a message (its attachments are queued for deletion). The sender, or a channel admin or owner for a person's key (#685, as in Slack, members delete their own messages with communications.create and membership alone). AI teammates' keys are allowed (#625) on their OWN messages only (never a person's, whatever the channel role), with the chat scope and the trail people have (no edit window for anyone), behind the live fence inside the write's transaction; the agent's change is audited in that transaction. People need communications.create and channel membership; an agent key is judged on the same right. A CRM bot post (a win, help or panic card the CRM authored) is deleted by a channel admin or owner only; a member's own delete of one is refused with 403. The write is scoped to this channel, so a message a contact merge moved to another channel is answered 409 and nothing is deleted or audited.
- **`conduyt-crm-pp-cli chat edit-message`** - Edit the caller's own message (body; record mentions are re-resolved). AI teammates' keys are allowed (#625) on their OWN messages only (never a person's, whatever the channel role), with the chat scope and the trail people have (no edit window for anyone), behind the live fence inside the write's transaction; the agent's change is audited in that transaction. People need communications.create (the right to have posted the message) and channel membership; a person edits only their OWN message, an admin or owner cannot edit another person's (#685); an agent key is judged on the same right. Only a text message can be edited (a card, system line or CRM bot post is refused with 403). The write is scoped to this channel, so a message a contact merge moved to another channel is answered 409 and nothing is edited.
- **`conduyt-crm-pp-cli chat follow-thread`** - (#684 lane F, Paul 2026-11-06: thread notifications follow Slack) Only while the workspace's thread view (teamChatThreads) is on; 409 code team_chat_threads_off otherwise. messageId is the thread ROOT (a reply id, an unknown id, another channel's message or, for a key whose scopes do not grant deals, a deal card: 404). Only a thread's followers are notified of a reply that stays in the thread: the root's author, everyone who replied, everyone mentioned in it, and whoever followed it here. A later reply or mention follows an unfollowed person again. Idempotent.
- **`conduyt-crm-pp-cli chat get-agents`** - The workspace's live AI teammates (a removed teammate is never listed), oldest first. A signed-in member reads the Settings rows (the whole live roster). An owner's or admin's API key (the chat scope; the read tier is enough) reads one row per teammate: { id, handle, name, provider, state, connected, connection, knowledgeFolders, createdAt } and nothing else (no token, no pair code, no webhook address); a member's key and an AI teammate's own key are refused (403). Use a row's id or @handle with the Grok Bot connect operations (/chat/agents/{id}/connect and /chat/agents/{id}/webhook). Creating, pairing and removing a teammate stay in Settings > AI (a signed-in session): POST /chat/agents refuses every API key. An API key is answered at most 200 live teammates; a workspace past that is refused to the key (413), never cut (the Settings session is not bounded).
- **`conduyt-crm-pp-cli chat get-agents-id-webhook`** - Get chat agents id webhook
- **`conduyt-crm-pp-cli chat get-agents-me-inbox`** - List chat agents me inbox
- **`conduyt-crm-pp-cli chat get-agents-me-mcp`** - The server is stateless and offers no SSE stream, so GET answers 405 with `Allow: POST`.
- **`conduyt-crm-pp-cli chat get-channels-id-linked-record`** - Get chat channels id linked-record
- **`conduyt-crm-pp-cli chat get-channels-id-notifications`** - Get chat channels id notifications
- **`conduyt-crm-pp-cli chat get-channels-id-pins`** - Get chat channels id pins
- **`conduyt-crm-pp-cli chat get-channels-id-upload`** - Get chat channels id upload
- **`conduyt-crm-pp-cli chat get-floor`** - List chat floor
- **`conduyt-crm-pp-cli chat get-message`** - Get a chat message by ID
- **`conduyt-crm-pp-cli chat get-messages-message-id-location`** - Get chat messages messageId location
- **`conduyt-crm-pp-cli chat get-presence`** - List chat presence
- **`conduyt-crm-pp-cli chat get-search`** - List chat search
- **`conduyt-crm-pp-cli chat get-thread`** - (#684) Only while the workspace's thread view (account setting teamChatThreads) is on; 409 code team_chat_threads_off otherwise. messageId is the thread ROOT (a top-level message; a reply id is a 404). The replies served are the thread's replies that are not deleted, inside the 90-day history window and, for a key whose scopes do not grant deals, not deal cards; total and earlierCount count the same set. Oldest first. Without before: the LATEST limit replies; before=<seq> reads the page below that reply ("Load N earlier replies": send earlierCursor). The root is served whatever its age while it has a reply inside the window (404 once it has none); a deleted root is served as stored (deletedAt set, body "Message deleted"). aiReactions: an AI teammate's turn that ended with a reaction instead of a reply, computed from its claim (never a stored message), while the reaction stands. replying: the AI teammates whose turn on this thread is running. Both are empty while AI teammates are off. Replies are posted with POST /chat/channels/{id}/messages and parentId.
- **`conduyt-crm-pp-cli chat get-thread-files`** - The attachments on a thread's root message and on every reply that carries that root, oldest first, capped at 200 per read (hasMore says there are more). Only messages inside the channel's 90-day history window count, a deleted message contributes nothing, and the replies of a deleted root keep their files. messageId must be a thread root; a reply id answers 404, and so does a root older than the window with no reply inside it. Same access as the channel read (a non-member of the channel is refused). Answers 409 team_chat_threads_off unless the account setting teamChatThreads is on.
- **`conduyt-crm-pp-cli chat get-typing-status`** - Get who is currently typing
- **`conduyt-crm-pp-cli chat get-unread`** - List chat unread
- **`conduyt-crm-pp-cli chat list-channels`** - List chat channels
- **`conduyt-crm-pp-cli chat list-messages`** - List messages in a channel
- **`conduyt-crm-pp-cli chat patch-agents-id`** - Update chat agents id
- **`conduyt-crm-pp-cli chat patch-channels-id-agents`** - Update chat channels id agents
- **`conduyt-crm-pp-cli chat patch-channels-id-notifications`** - Update chat channels id notifications
- **`conduyt-crm-pp-cli chat post-agents`** - Create / invoke chat agents
- **`conduyt-crm-pp-cli chat post-agents-id-connect`** - Create / invoke chat agents id connect
- **`conduyt-crm-pp-cli chat post-agents-id-pair-code`** - Create / invoke chat agents id pair-code
- **`conduyt-crm-pp-cli chat post-agents-id-webhook-test`** - Create / invoke chat agents id webhook test
- **`conduyt-crm-pp-cli chat post-agents-me-heartbeat`** - Create / invoke chat agents me heartbeat
- **`conduyt-crm-pp-cli chat post-agents-me-inbox-ack`** - Create / invoke chat agents me inbox ack
- **`conduyt-crm-pp-cli chat post-agents-me-inbox-claim`** - Create / invoke chat agents me inbox claim
- **`conduyt-crm-pp-cli chat post-agents-me-mcp`** - (#1140) A stateless MCP server over Streamable HTTP for an AI teammate that runs in the cloud (for example a Grok Bot routine woken by a webhook): it answers JSON only, with no session id and no SSE stream. One JSON-RPC 2.0 message per request; a batch (an array) is refused with -32600.
Methods: `initialize` (echoes protocolVersion 2025-06-18 or 2025-03-26, otherwise answers 2025-06-18; capabilities `tools` only), notifications such as `notifications/initialized` (202, no body), `ping`, `tools/list`, `tools/call`. Any other method answers -32601; an unknown tool -32602.
Tools (each calls the existing route in-process with the caller's own key, so every rule of that route applies unchanged, the rate limits included):
- `reply` {channelId, parentId, body, idempotencyKey, claimToken?}: POST /chat/channels/{id}/messages. Rail mode: idempotencyKey `turn:<messageId>` with no claimToken; the same key returns the first reply (idempotentReplay) instead of posting twice. Claim mode: `reply:<messageId>` plus the claim token completes that claim.
- `react` {channelId, messageId, emoji, claimToken?, sourceMessageId?}: POST /chat/channels/{id}/messages/{messageId}/reactions; with a token it ends a claimed turn.
- `stay_silent` {messageId, claimToken}: POST /chat/agents/me/inbox/claim with action silent (claim mode only).
- `send` {channelId, body}: POST /chat/channels/{id}/messages, a new message (never an answer to a turn).
- `linked_record` {channelId}: GET /chat/channels/{id}/linked-record (read-only).
- `read_messages` {channelId, limit 1 to 50, default 20}: GET /chat/channels/{id}/messages?limit=.
A route's success is the tool result's text content (its `data`); a route's refusal is a result with `isError: true` whose text is the route's error body plus its `status` (for example 409 agent_human_required, 409 reply_stale_claim, 403 Not a member of this channel). Arguments of the wrong type or a missing required argument are `isError: true` with code invalid_arguments, and no route runs.
Who: a paired AI teammate key only, sent as `Authorization: Bearer cdy_...` (the chat scope); the same key in X-API-Key, a session, or anything else is 403 agent_key_required. Never put the key in the URL.
Mutation budget: this path is NOT charged per request by the key's 500 an hour mutation budget; each WRITE tool call (reply, react, send, stay_silent) is charged once to that same budget instead. Protocol methods and the read tools (read_messages, linked_record) are never charged. A write past the budget is a tool result with isError true: the budget's message, status 429, code mutation_budget_exceeded and retryAfter (seconds).
- **`conduyt-crm-pp-cli chat post-agents-pair`** - Create / invoke chat agents pair
- **`conduyt-crm-pp-cli chat post-ask`** - Create / invoke chat ask
- **`conduyt-crm-pp-cli chat post-celebrate`** - Create / invoke chat celebrate
- **`conduyt-crm-pp-cli chat post-channels-id-messages-message-id-claim`** - Create / invoke chat channels id messages messageId claim
- **`conduyt-crm-pp-cli chat post-channels-id-messages-message-id-handoff`** - Create / invoke chat channels id messages messageId handoff
- **`conduyt-crm-pp-cli chat post-channels-id-messages-message-id-pin`** - Toggle (or set, with pinned) a deal card's pin; any member of the channel. An AI teammate's pin and unpin are audited (chat.message_pinned / chat.message_unpinned). AI teammates' keys are allowed (#625): the same chat scope and rules as a person, behind the live fence inside the write's transaction. An agent key answers 401 agent_revoked (the agent or its key was revoked, or the agent or its owner is inactive), 402 plan_required, 404 ai_teammates_off, 403 when it is not a member of the channel, and 409 agents_muted_in_channel when "Agents respond here" is off in this channel; each refusal writes nothing.
- **`conduyt-crm-pp-cli chat post-channels-id-upload-finalize`** - Step 3 of a direct-to-storage chat attachment: the staged bytes are judged by content and copied into the channel's attachment folder; send the answer in a message's metadata.attachments (POST /chat/channels/{id}/messages). AI teammates' keys are allowed (#625): the same chat scope and rules as a person, behind the live fence inside the write's transaction. An agent key answers 401 agent_revoked (the agent or its key was revoked, or the agent or its owner is inactive), 402 plan_required, 404 ai_teammates_off, 403 when it is not a member of the channel, and 409 agents_muted_in_channel when "Agents respond here" is off in this channel; each refusal writes nothing.
- **`conduyt-crm-pp-cli chat post-channels-id-upload-intent`** - Step 1 of a direct-to-storage chat attachment: answers a presigned PUT for the declared name, type and size (the chat attachment limits), then PUT the bytes and POST .../upload/finalize. The caller is recorded as the uploader. AI teammates' keys are allowed (#625): the same chat scope and rules as a person, behind the live fence inside the write's transaction. An agent key answers 401 agent_revoked (the agent or its key was revoked, or the agent or its owner is inactive), 402 plan_required, 404 ai_teammates_off, 403 when it is not a member of the channel, and 409 agents_muted_in_channel when "Agents respond here" is off in this channel; each refusal writes nothing.
- **`conduyt-crm-pp-cli chat post-handoff`** - Create / invoke chat handoff
- **`conduyt-crm-pp-cli chat put-agents-id-webhook`** - Update chat agents id webhook
- **`conduyt-crm-pp-cli chat put-agents-me-knowledge`** - (#1209) An AI teammate's listener (by its listenerId, the id its heartbeat and claims carry) reports its KNOWLEDGE FOLDER (a folder on the listener's computer the teammate reads on every turn, whoever asked, read-only; by default the directory the listener was started in). Conduyt stores only, per listener, the folder count, never the path, a file name or contents; every surface reads the aggregate over the listeners whose heartbeat beats now (configured = any of them has a folder; folders = the sum), so a listener started with --no-knowledge never hides another one's folder, and a stopped listener drops out with its last beat. acknowledged: true records that the owner acknowledged the consent on this start ("Anyone who can mention your teammate can ask it about the files in this folder. Set it up for project or document folders, never one holding secrets. Files that look like credentials are skipped, but that is a safety net, not a guarantee."). The teammate's own paired key only. A report that changes this listener's entry is stored; the audit row ai_knowledge_folder.changed is written when the aggregate changes or on an acknowledgement; a report that changes nothing writes nothing. The teammate's entry is removed when it is revoked.
- **`conduyt-crm-pp-cli chat remove-member`** - Remove a member from a channel
- **`conduyt-crm-pp-cli chat remove-reaction`** - Remove a reaction from a message
- **`conduyt-crm-pp-cli chat send-message`** - Send a message in a channel
- **`conduyt-crm-pp-cli chat send-typing-indicator`** - Send a typing indicator
- **`conduyt-crm-pp-cli chat unfollow-thread`** - (#684 lane F) The caller stops being notified of the thread's replies, until a later reply of theirs or a mention of them follows them again (Slack does). Same guards as PUT. Idempotent.
- **`conduyt-crm-pp-cli chat upload-file`** - Upload a file to a channel

### companies

Company (organization) management

- **`conduyt-crm-pp-cli companies create-company`** - Create a company
- **`conduyt-crm-pp-cli companies delete-company`** - Soft-delete a company
- **`conduyt-crm-pp-cli companies get-company`** - Get a company by ID
- **`conduyt-crm-pp-cli companies get-duplicates`** - List companies duplicates
- **`conduyt-crm-pp-cli companies get-export`** - List companies export
- **`conduyt-crm-pp-cli companies get-reporting`** - List companies reporting
- **`conduyt-crm-pp-cli companies list`** - List companies
- **`conduyt-crm-pp-cli companies post-merge`** - Create / invoke companies merge
- **`conduyt-crm-pp-cli companies update-company`** - Update a company

### conduyt-auth

Manage conduyt auth

- **`conduyt-crm-pp-cli conduyt-auth accept-invite`** - Accept a team invitation
- **`conduyt-crm-pp-cli conduyt-auth change-password`** - Change password (authenticated)
- **`conduyt-crm-pp-cli conduyt-auth confirm-sso-signup`** - The consent (a browser flow, before any session). JSON only (415), from Conduyt's own pages only (403), at most 1024 bytes (413); the body is closed and termsAccepted must be true (400, and nothing is consumed). The token must match this browser's conduyt_sso_pending cookie; the pending sign-up is then consumed once (410 when used, expired or unknown) and the account is created through the same creation as POST /auth/register (the register route's 3 per IP per hour, the protected-domain refusal). The answer is where the page goes next - the company-name screen with the session cookie set, or a /login?error= path for a refusal.
- **`conduyt-crm-pp-cli conduyt-auth delete-sessions`** - Delete auth sessions
- **`conduyt-crm-pp-cli conduyt-auth forgot-password`** - Request a password reset email
- **`conduyt-crm-pp-cli conduyt-auth get-login-history`** - List auth login-history
- **`conduyt-crm-pp-cli conduyt-auth get-me`** - Identical to GET /users/me, byte for byte (that route re-exports this handler) — including `data.account.name`.
- **`conduyt-crm-pp-cli conduyt-auth get-mfa-setup`** - List auth mfa setup
- **`conduyt-crm-pp-cli conduyt-auth get-saml-account-slug-metadata`** - Get auth saml accountSlug metadata
- **`conduyt-crm-pp-cli conduyt-auth get-sandbox-key`** - List auth sandbox-key
- **`conduyt-crm-pp-cli conduyt-auth get-security-center`** - List auth security-center
- **`conduyt-crm-pp-cli conduyt-auth get-sessions`** - List auth sessions
- **`conduyt-crm-pp-cli conduyt-auth login`** - Authenticates user credentials and returns a session cookie. Rate limited to 5 requests per 15 minutes per IP.
- **`conduyt-crm-pp-cli conduyt-auth logout`** - Log out (destroy session)
- **`conduyt-crm-pp-cli conduyt-auth patch-security-center`** - Update auth security-center
- **`conduyt-crm-pp-cli conduyt-auth post-mfa-challenge`** - Create / invoke auth mfa challenge
- **`conduyt-crm-pp-cli conduyt-auth post-mfa-disable`** - Create / invoke auth mfa disable
- **`conduyt-crm-pp-cli conduyt-auth post-mfa-setup`** - Create / invoke auth mfa setup
- **`conduyt-crm-pp-cli conduyt-auth post-mfa-verify`** - Create / invoke auth mfa verify
- **`conduyt-crm-pp-cli conduyt-auth post-sandbox-key`** - Create / invoke auth sandbox-key
- **`conduyt-crm-pp-cli conduyt-auth post-signup-events`** - Used by the sign-up page before any session exists: one of signup_page_viewed, signup_form_started, signup_sso_clicked {provider}, signup_submit_attempted, signup_error {field, code} (never a message), under the page's anonymous funnel id, with the page's attribution. Rate limited per IP. The server adds is_test true to the event when the request presents the test-account harness secret or comes from a headless browser or known crawler (matched on the User-Agent, which is never stored); a client can never send it.
- **`conduyt-crm-pp-cli conduyt-auth read-sso-signup-consent`** - The consent screen's read (a browser flow, before any session). After a one-click Google or Microsoft sign-up returns with a NEW verified identity, the callback keeps a one-use pending sign-up for 15 minutes and sends the person to /register/confirm?t=TOKEN; the token must be the one the httpOnly conduyt_sso_pending cookie (path /api/v1/auth/sso) carries. Answers who is continuing and with which provider, and records the signup_sso_consent_shown funnel event. Nothing is consumed or created.
- **`conduyt-crm-pp-cli conduyt-auth register`** - Creates a new user and account. Rate limited to 3 requests per hour per IP. Requires `--terms-accepted` (consent to the Terms of Service and Privacy Policy)
- **`conduyt-crm-pp-cli conduyt-auth reset-password`** - Reset password with token
- **`conduyt-crm-pp-cli conduyt-auth start-google-signup`** - The register page's one-click Google sign-up start (a browser flow, before any session; GET on this path is the sign-in start, a redirect). The page sends it on the click, without a terms box; the Terms are accepted on the consent screen after the provider returns. The funnel id and the attribution are sealed into the short-lived httpOnly OAuth state cookie, bound to the CSRF state with a MAC (10 minutes, at most 4 KB); the answer is the Google URL the page navigates to. On the return the callback sends a verified new email to the consent screen (/register/confirm, a one-use pending sign-up of 15 minutes, bound to the browser); accepting the Terms there creates the account, through the same creation as POST /auth/register, and sends the person to the company-name screen (POST /account/company-name). JSON only (415 otherwise), from Conduyt's own pages only (403 otherwise), at most 8192 bytes (413), and a sign-up start budget of 20 per IP per hour, charged only after the body passes (429); the accounts themselves stay capped by the register route's 3 per IP per hour when the consent creates them. Fail-closed when the limiter is unavailable (503). A request with neither Origin nor Sec-Fetch-Site is refused (403). Each start's state is one-use.
- **`conduyt-crm-pp-cli conduyt-auth start-microsoft-signup`** - The register page's one-click Microsoft sign-up start (a browser flow, before any session; GET on this path is the sign-in start, a redirect). The page sends it on the click, without a terms box; the Terms are accepted on the consent screen after the provider returns. The funnel id and the attribution are sealed into the short-lived httpOnly OAuth state cookie, bound to the CSRF state with a MAC (10 minutes, at most 4 KB); the answer is the Microsoft URL the page navigates to. On the return the callback sends a verified new email to the consent screen (/register/confirm, a one-use pending sign-up of 15 minutes, bound to the browser); accepting the Terms there creates the account, through the same creation as POST /auth/register, and sends the person to the company-name screen (POST /account/company-name). JSON only (415 otherwise), from Conduyt's own pages only (403 otherwise), at most 8192 bytes (413), and a sign-up start budget of 20 per IP per hour, charged only after the body passes (429); the accounts themselves stay capped by the register route's 3 per IP per hour when the consent creates them. Fail-closed when the limiter is unavailable (503). A request with neither Origin nor Sec-Fetch-Site is refused (403). Each start's state is one-use.
- **`conduyt-crm-pp-cli conduyt-auth switch-account`** - Switch to a different account

### conduyt-help

Manage conduyt help

- **`conduyt-crm-pp-cli conduyt-help get-guides`** - List help guides
- **`conduyt-crm-pp-cli conduyt-help get-guides-slug`** - Get help guides slug
- **`conduyt-crm-pp-cli conduyt-help get-guides-slug-feedback`** - Get help guides slug feedback
- **`conduyt-crm-pp-cli conduyt-help post-guides-slug-feedback`** - Create / invoke help guides slug feedback
- **`conduyt-crm-pp-cli conduyt-help post-guides-slug-open`** - Create / invoke help guides slug open
- **`conduyt-crm-pp-cli conduyt-help post-suggestions`** - Create / invoke help suggestions

### confirm

Manage confirm

- **`conduyt-crm-pp-cli confirm post`** - Create / invoke confirm
- **`conduyt-crm-pp-cli confirm put`** - Update confirm

### contact

Contact management, tagging, scoring, import/export, merge, duplicates

- **`conduyt-crm-pp-cli contact`** - Creates or updates a contact by email or phone match. Designed for inbound webhook integrations.

### contacts

Contact management, tagging, scoring, import/export, merge, duplicates

- **`conduyt-crm-pp-cli contacts create`** - Creates a new contact. Rate limited to 30 requests per minute. With `callId` (the unknown-caller prompt's "Create new lead"), the new contact is linked to that contact-less INBOUND call in the SAME transaction: the phone is the call's own caller number (a submitted `phone` must be omitted or strictly the same number, else 422 `phone_not_call_number`), stored under the account's country; `firstName` is required (400 `first_name_required`) and, for this lead only, the account's form-required contact fields and required custom fields are not required (values that are sent are still validated). The caller needs `dialer.create` (an API key also needs the `calls` scope; a `dialer` key covers it). A call that does not exist is 404 `call_not_found`; a call already linked is 409 `call_already_linked` (or 404 `call_not_found` when its contact is hidden from the caller); an outbound call is 422 `call_not_inbound`; a call with no usable caller number is 422 `number_unusable`. The lead's `source` (always "Inbound Call") and owner are derived from the call, so a request with `callId` must NOT carry `source` or `assignedTo` (any value, null included): it is refused with 422 `call_derived_fields`. Nothing is created on any refusal. callId derives the owner: the call taker, else the signed-in session user; an API-key request, an inactive call taker or an inactive fallback user derive no owner. A contact that derives no owner is then subject to the workspace's normal lead routing and Smart View assignment like any other create, so it may still end up assigned.
- **`conduyt-crm-pp-cli contacts delete`** - Soft-delete a contact
- **`conduyt-crm-pp-cli contacts export`** - Export contacts as CSV
- **`conduyt-crm-pp-cli contacts duplicate-proposals list-contact <id> [--cursor <c>] [--limit <n>] [--held-cursor <c>] [--held-limit <n>]`** - Identity rules v2: the possible duplicates of a contact (`proposals` open or not_sure, `heldIdentifiers`), keyset paged with `total`, `nextCursor`, the held twins and membership revisions (re-read page one when a later page answers another revision); read-only. The old name `get-contacts-id` is an alias.
- **`conduyt-crm-pp-cli contacts duplicate-proposals decide-contact <id> <proposalId> --decision not_sure|different|merge`** - Record a decision: `not_sure` keeps both visible, `different` is final, `merge` only records a pair ALREADY merged (never merges; 409 `not_merged`); 409 `already_decided`, 403 `other_record_not_editable`; answers `previousMembershipRevision`, `membershipRevision`, `total`. The old name `post-contacts-id-proposal-id` is an alias.
- **`conduyt-crm-pp-cli imports duplicate-proposals get-imports-id <importId> [--page <n>] [--per-page <n>]`** - The duplicate proposals one import raised on the records it created (every status, oldest first, `{ rows, meta }`); a record you cannot view is "A contact you cannot view"; read-only.
- **`conduyt-crm-pp-cli contacts find-duplicate`** - Find duplicate contacts
- **`conduyt-crm-pp-cli contacts get`** - Get a contact by ID
- **`conduyt-crm-pp-cli contacts get-count`** - The total of a contacts list, asked separately from its rows (GET /api/v1/contacts answers
meta.total null when it did not count). Takes the same filter parameters as GET /api/v1/contacts;
paging, sort and include are not read.

A list whose total is cached (the unfiltered list, or a Smart View alone) answers the cached exact
count while it is under a minute old (`age_ms`), otherwise recounts it now. When another request is
already recounting it and nothing is cached yet the answer is `total: null, pending: true`: ask again
in a few seconds. Any other filter is counted exactly now; a count that cannot finish within 30 s
answers `total: null, timed_out: true`. A search counts up to 10,000 (`total_capped`).

A smartListId that is not a uuid is a 400 (invalid_smart_list_id); one that names no Smart List of
the account is a 404 (smart_list_not_found), as on GET /api/v1/contacts. Any other failure of the
count is an error (500), never timed_out.
- **`conduyt-crm-pp-cli contacts get-dnc-status`** - List contacts dnc-status
- **`conduyt-crm-pp-cli contacts get-geo`** - List contacts geo
- **`conduyt-crm-pp-cli contacts get-import-template`** - Download CSV import template
- **`conduyt-crm-pp-cli contacts get-sources`** - List contacts sources
- **`conduyt-crm-pp-cli contacts import`** - Import contacts from CSV
- **`conduyt-crm-pp-cli contacts list`** - Returns a paginated list of contacts. Supports search, filtering by tag,
source, company, assigned user, date ranges, lead score, and smart views.

Smart views: `needs_follow_up`, `hot_leads`, `new_this_week`,
`recently_won`, `untagged`, `missing_email`, `missing_phone`.

The rows never wait for an exact count of a large account: `meta.total` may be null
(`meta.total_deferred: true`) or a cached count (`meta.total_age_ms`). `meta.has_more`
is always exact; GET /api/v1/contacts/count returns the total of the same filters.
A smartListId that is not a uuid is a 400 (invalid_smart_list_id); one that names no Smart List of
the account is a 404 (smart_list_not_found).
- **`conduyt-crm-pp-cli contacts merge`** - Merge two contacts
- **`conduyt-crm-pp-cli contacts post-ai-import`** - Create / invoke contacts ai-import
- **`conduyt-crm-pp-cli contacts post-dry-run`** - Create / invoke contacts dry-run
- **`conduyt-crm-pp-cli contacts post-verify-line-type`** - Create / invoke contacts verify-line-type
- **`conduyt-crm-pp-cli contacts score-batch`** - Batch twin of GET /contacts/{id}/score, used by the contacts list so a page makes one request instead of one per row. A GET, so read-tier and sandbox keys reach it and it spends no mutation budget. Same permission (contacts view) and same visibility as the single-contact route. Ids that do not exist, belong to another account, are deleted or are not visible to the caller are absent from scores (never an error). More than 100 ids is a 400. Each entry carries contactId, score, tier, breakdown, signals and computedAt.
- **`conduyt-crm-pp-cli contacts update`** - Deprecated on contacts (2026-10-13): `customFields.oes_number` and `customFields.dpp_oes_raw`. The OES number identifies an opportunity, so it lives on the deal card (deal custom fields `oes_number` / `dpp_oes_raw`, one live card per number: PATCH /deals/{id}). The contact keys are still accepted (and kept unique per workspace) while existing contact values are migrated onto their cards; new integrations should write the card.

### conversations

Threaded conversation view per contact

- **`conduyt-crm-pp-cli conversations get`** - Get conversation thread for a contact
- **`conduyt-crm-pp-cli conversations get-export`** - List conversations export
- **`conduyt-crm-pp-cli conversations get-metrics`** - List conversations metrics
- **`conduyt-crm-pp-cli conversations list`** - List conversation threads
- **`conduyt-crm-pp-cli conversations patch-contact-id`** - Update conversations contactId

### custom-fields

Custom field definitions for contacts and deals

- **`conduyt-crm-pp-cli custom-fields create`** - Create a custom field definition
- **`conduyt-crm-pp-cli custom-fields delete`** - Delete a custom field definition
- **`conduyt-crm-pp-cli custom-fields get-id`** - Get custom-fields id
- **`conduyt-crm-pp-cli custom-fields list`** - List custom field definitions
- **`conduyt-crm-pp-cli custom-fields update`** - Update a custom field definition

### dashboard

Dashboard summary metrics

- **`conduyt-crm-pp-cli dashboard`** - The counts and totals are cached per server instance for 60 seconds per account, role and visibility scope; for 60 seconds after that a cached copy is still answered while one recompute runs in the background. Under that, every instance shares the last computation of the same account and scope (stored server-side, no person or deal data). A shared copy under 3 minutes old is answered as is, an older one is answered while one recompute runs after the response. Every list with a person or a deal (recent activity, needs follow-up, stale deals, overdue tasks, workload names) is read live on every request. generatedAt says when the counts were read and aggregatesAgeMs how old they were when this response was sent. fresh=1 recomputes now. When the numbers cannot be read in time (another request computing a scope's numbers for the first time, the shared cache unable to coordinate, or this request's own computation out of time) the answer is a retryable 503 dashboard_computing with Retry-After. MCP and CLI parity for this 503 is tracked under board #602.

### dashboards

Dashboard summary metrics

- **`conduyt-crm-pp-cli dashboards delete-id`** - Delete dashboards id
- **`conduyt-crm-pp-cli dashboards get`** - List dashboards
- **`conduyt-crm-pp-cli dashboards get-id`** - Get dashboards id
- **`conduyt-crm-pp-cli dashboards patch-id`** - Update dashboards id
- **`conduyt-crm-pp-cli dashboards post`** - Create / invoke dashboards

### data-model

Manage data model

- **`conduyt-crm-pp-cli data-model`** - List data-model quality

### deals

Deal/opportunity management within pipelines

- **`conduyt-crm-pp-cli deals create`** - Creates a new deal in a pipeline stage. Rate limited to 30 requests per minute.
- **`conduyt-crm-pp-cli deals delete`** - Soft-delete a deal
- **`conduyt-crm-pp-cli deals delete-views-id`** - Delete deals views id
- **`conduyt-crm-pp-cli deals get`** - Get a deal by ID
- **`conduyt-crm-pp-cli deals get-export`** - List deals export
- **`conduyt-crm-pp-cli deals get-forecast`** - List deals forecast
- **`conduyt-crm-pp-cli deals get-inspection`** - The six Pipeline health checks (High-value idle, Stale in stage, Unassigned, Needs next action, No close date, Missing contact) over the open deals of one pipeline (or every visible active pipeline), ordered by money at risk. With `check`, also that check's deals, highest value first. The Deal Pipeline page reads the same numbers through GET /deals?health_summary=true, which also takes every board filter.
- **`conduyt-crm-pp-cli deals get-lead-id`** - Resolves a Lead ID (the first 8 to 32 hex characters of a deal id or of a contact id, dashes optional, any case; a whole id matches exactly) to the deals it names: deals whose id starts with it, and the deals of the contact whose id does. Uses the same read visibility as GET /deals/{id} (deals.view, owned or shared leads, the pipeline allowlist), not the board's assignee-only scope; a Lead ID the caller cannot see answers like one nobody has. Lists no text matches. Each deal carries matchedBy (deal or contact), its contact (null when hidden from the caller) and onBoard (whether the caller's pipeline board, whose scope is narrower, lists it). 400 when the term is not a Lead ID or more than 100 visible records share it; 403 without deals.view or for an API key whose scopes do not grant deals.
- **`conduyt-crm-pp-cli deals get-views`** - List deals views
- **`conduyt-crm-pp-cli deals get-views-id`** - Get deals views id
- **`conduyt-crm-pp-cli deals list`** - Returns deals with Kanban-optimized sort order (stage, sortOrder, then requested sort).
- **`conduyt-crm-pp-cli deals list --board-snapshot --pipeline <id> --board-columns <stageId>[:<limit>],...`** - Every named column of one pipeline from ONE database snapshot (a moved deal is in exactly one column): `{ columns: [{ stageId, limit, deals }], meta: { board_snapshot: true } }`, rows only, read live; `--page`, `--stage` and `--all` are refused before any request. 503 `board_snapshot_busy` (snapshot only) or `gate_busy` (any deals list call): nothing was changed, retry after Retry-After.
- **`conduyt-crm-pp-cli deals patch-views-id`** - Update deals views id
- **`conduyt-crm-pp-cli deals post-probabilities`** - Create / invoke deals probabilities
- **`conduyt-crm-pp-cli deals post-views`** - Create / invoke deals views
- **`conduyt-crm-pp-cli deals update`** - Update a deal

### dialer

Click-to-call dialer via Twilio

- **`conduyt-crm-pp-cli dialer delete-call-id-participants-participant-sid`** - Delete dialer call id participants participantSid
- **`conduyt-crm-pp-cli dialer delete-call-id-supervise`** - Delete dialer call id supervise
- **`conduyt-crm-pp-cli dialer get-agent-statuses`** - List dialer agent-statuses
- **`conduyt-crm-pp-cli dialer get-agents-status`** - List dialer agents-status
- **`conduyt-crm-pp-cli dialer get-call-id`** - Get dialer call id
- **`conduyt-crm-pp-cli dialer get-call-id-media`** - Get dialer call id media
- **`conduyt-crm-pp-cli dialer get-call-id-participants`** - Get dialer call id participants
- **`conduyt-crm-pp-cli dialer get-call-id-post-call`** - Get dialer call id post-call
- **`conduyt-crm-pp-cli dialer get-calls`** - List dialer calls
- **`conduyt-crm-pp-cli dialer get-calls-hourly`** - List dialer calls hourly
- **`conduyt-crm-pp-cli dialer get-capabilities`** - List dialer capabilities
- **`conduyt-crm-pp-cli dialer get-dispositions`** - List dialer dispositions
- **`conduyt-crm-pp-cli dialer get-health`** - List dialer health
- **`conduyt-crm-pp-cli dialer get-history`** - The Call History list, with search (`q`) and filters (`recorded_only`, `disposition`, `user_id`, `date_from`, `date_to`, `direction`, `status`, and the unknown-caller filters `unmatched` and `line`). Search results are AND-ed with the tenant and contact-visibility scope.
- **`conduyt-crm-pp-cli dialer get-inbound-lookup`** - List dialer inbound-lookup
- **`conduyt-crm-pp-cli dialer get-intelligence`** - List dialer intelligence
- **`conduyt-crm-pp-cli dialer get-leaderboard`** - List dialer leaderboard
- **`conduyt-crm-pp-cli dialer get-live-monitor`** - List dialer live-monitor
- **`conduyt-crm-pp-cli dialer get-local-presence-resolve`** - List dialer local-presence resolve
- **`conduyt-crm-pp-cli dialer get-queue`** - List dialer queue
- **`conduyt-crm-pp-cli dialer get-queue-open`** - List dialer queue open
- **`conduyt-crm-pp-cli dialer get-recordings`** - List dialer recordings
- **`conduyt-crm-pp-cli dialer get-settings`** - List dialer settings
- **`conduyt-crm-pp-cli dialer get-setup`** - List dialer setup
- **`conduyt-crm-pp-cli dialer get-stats`** - List dialer stats
- **`conduyt-crm-pp-cli dialer get-status-log`** - List dialer status-log
- **`conduyt-crm-pp-cli dialer get-status-log-team`** - List dialer status-log team
- **`conduyt-crm-pp-cli dialer get-status-log-timeline`** - List dialer status-log timeline
- **`conduyt-crm-pp-cli dialer get-token`** - Get a Twilio browser token for click-to-call
- **`conduyt-crm-pp-cli dialer initiate-call`** - Initiate an outbound call
- **`conduyt-crm-pp-cli dialer patch-call-id`** - Update dialer call id
- **`conduyt-crm-pp-cli dialer patch-call-id-participants-participant-sid`** - Update dialer call id participants participantSid
- **`conduyt-crm-pp-cli dialer patch-settings`** - Update dialer settings
- **`conduyt-crm-pp-cli dialer post-call-eligibility`** - Create / invoke dialer call eligibility
- **`conduyt-crm-pp-cli dialer post-call-id-ai-disposition`** - Create / invoke dialer call id ai-disposition
- **`conduyt-crm-pp-cli dialer post-call-id-facetime-status`** - Create / invoke dialer call id facetime-status
- **`conduyt-crm-pp-cli dialer post-call-id-hangup`** - Create / invoke dialer call id hangup
- **`conduyt-crm-pp-cli dialer post-call-id-hold`** - Create / invoke dialer call id hold
- **`conduyt-crm-pp-cli dialer post-call-id-mute`** - Create / invoke dialer call id mute
- **`conduyt-crm-pp-cli dialer post-call-id-participants`** - Create / invoke dialer call id participants
- **`conduyt-crm-pp-cli dialer post-call-id-record`** - Create / invoke dialer call id record
- **`conduyt-crm-pp-cli dialer post-call-id-supervise`** - Join an active call as a supervisor (admins and owners). listen: muted, nobody hears the supervisor; whisper: only the REP hears the supervisor (the coach target is always the rep's leg of this call, never the first other participant); barge: everyone hears the supervisor. A received call that is not on its conference line yet is moved there first (the rep and the caller hear about a second of silence). agentCallSid (optional, any mode): the rep leg the caller last saw (GET /dialer/live-monitor agentLegSid); when the call's rep leg is another, the request is refused before anything moves.
- **`conduyt-crm-pp-cli dialer post-call-id-transfer`** - Create / invoke dialer call id transfer
- **`conduyt-crm-pp-cli dialer post-call-id-vm-drop`** - Create / invoke dialer call id vm-drop
- **`conduyt-crm-pp-cli dialer post-heartbeat`** - Create / invoke dialer heartbeat
- **`conduyt-crm-pp-cli dialer post-local-presence-sync`** - Create / invoke dialer local-presence sync
- **`conduyt-crm-pp-cli dialer post-mirror-token`** - Create / invoke dialer mirror-token
- **`conduyt-crm-pp-cli dialer post-queue-next`** - Create / invoke dialer queue next
- **`conduyt-crm-pp-cli dialer post-queue-release`** - Create / invoke dialer queue release
- **`conduyt-crm-pp-cli dialer post-queue-skip`** - Create / invoke dialer queue skip
- **`conduyt-crm-pp-cli dialer post-queue-worked`** - Create / invoke dialer queue worked
- **`conduyt-crm-pp-cli dialer post-setup-wire-inbound`** - Create / invoke dialer setup wire-inbound
- **`conduyt-crm-pp-cli dialer post-status-log`** - Create / invoke dialer status-log
- **`conduyt-crm-pp-cli dialer post-status-log-rollup`** - Create / invoke dialer status-log rollup

### dnc

Manage dnc

- **`conduyt-crm-pp-cli dnc delete-id`** - Delete dnc id
- **`conduyt-crm-pp-cli dnc get`** - List dnc
- **`conduyt-crm-pp-cli dnc post`** - Create / invoke dnc
- **`conduyt-crm-pp-cli dnc post-import`** - Create / invoke dnc import

### document-templates

Proposal and contract templates with merge fields

- **`conduyt-crm-pp-cli document-templates create`** - Create a document template
- **`conduyt-crm-pp-cli document-templates delete`** - Delete a document template
- **`conduyt-crm-pp-cli document-templates get`** - Get a document template by ID
- **`conduyt-crm-pp-cli document-templates list`** - List document templates
- **`conduyt-crm-pp-cli document-templates update`** - Update a document template

### drip-campaigns

SMS drip campaign engine

- **`conduyt-crm-pp-cli drip-campaigns create`** - Create a drip campaign
- **`conduyt-crm-pp-cli drip-campaigns delete`** - Delete a drip campaign
- **`conduyt-crm-pp-cli drip-campaigns get-id`** - Get drip-campaigns id
- **`conduyt-crm-pp-cli drip-campaigns list`** - List SMS drip campaigns
- **`conduyt-crm-pp-cli drip-campaigns seed`** - Seed default drip campaigns
- **`conduyt-crm-pp-cli drip-campaigns update`** - Update a drip campaign

### drip-enrollments

Manage drip enrollments

- **`conduyt-crm-pp-cli drip-enrollments list`** - List drip enrollments
- **`conduyt-crm-pp-cli drip-enrollments post-batch-pause`** - Create / invoke drip-enrollments batch pause
- **`conduyt-crm-pp-cli drip-enrollments post-batch-resume`** - Create / invoke drip-enrollments batch resume
- **`conduyt-crm-pp-cli drip-enrollments post-batch-stop`** - Create / invoke drip-enrollments batch stop

### drip-tracks

Manage drip tracks

- **`conduyt-crm-pp-cli drip-tracks delete-id`** - Delete drip-tracks id
- **`conduyt-crm-pp-cli drip-tracks get`** - List drip-tracks
- **`conduyt-crm-pp-cli drip-tracks get-id`** - Get drip-tracks id
- **`conduyt-crm-pp-cli drip-tracks patch-id`** - Update drip-tracks id
- **`conduyt-crm-pp-cli drip-tracks post`** - Create / invoke drip-tracks
- **`conduyt-crm-pp-cli drip-tracks post-import-steps`** - Create / invoke drip-tracks import-steps

### email

Send individual and bulk emails

- **`conduyt-crm-pp-cli email get-opt-out-list`** - List email opt-out-list
- **`conduyt-crm-pp-cli email post-opt-out-list`** - Create / invoke email opt-out-list
- **`conduyt-crm-pp-cli email post-opt-out-list-check`** - Answers whether one email address is on this workspace's email opt-out list, and if so its source and since when. It is a POST only so the address travels in the JSON body and never in a URL (request logs keep URLs); it reads and stores nothing. Send a bare address (person@example.com, at most 320 characters). The server is more tolerant than this contract promises, and it also trims, lowercases and strips a display name before matching by hash, so the case of the address does not matter, but only the bare form is documented. Owners, admins and the platform super admin only. An API key reaches it with the emails scope, and a read-tier key may call it (this exact path is the only list route a read-tier key may POST).
- **`conduyt-crm-pp-cli email post-opt-out-list-resubscribe`** - Create / invoke email opt-out-list resubscribe
- **`conduyt-crm-pp-cli email send`** - Sends one email through the account's connected Resend domain and logs it as a Message row linked to the contact. `to` must equal the linked contact's registered email address (case-insensitive) — this stops a caller from citing a contact's id but sending to a different address, which would bypass that contact's opt-out/CAN-SPAM status. Provide `html`, `text`, or both; at least one body is required. There is no scheduling field — this endpoint sends immediately or fails; it never queues a future send. This is a contact-bound send — `cc`/`bcc` are NOT supported, so a non-empty value is rejected with 400 (omitted, null, whitespace-only and [] are accepted as none), for the same reason as the `to` guard above — only `to` is checked against the contact's opt-out/registered-address status, so any other recipient would bypass it. Send to each additional contact as its own request.
- **`conduyt-crm-pp-cli email send-bulk`** - Send bulk emails

### email-domains

Custom email domain verification (Resend)

- **`conduyt-crm-pp-cli email-domains add`** - Add a custom email domain
- **`conduyt-crm-pp-cli email-domains delete-reply-capture`** - Delete email-domains reply-capture
- **`conduyt-crm-pp-cli email-domains get`** - Get email domain configuration
- **`conduyt-crm-pp-cli email-domains get-reply-capture`** - List email-domains reply-capture
- **`conduyt-crm-pp-cli email-domains get-status`** - List email-domains status
- **`conduyt-crm-pp-cli email-domains post-reply-capture`** - Create / invoke email-domains reply-capture
- **`conduyt-crm-pp-cli email-domains post-reply-capture-verify`** - Create / invoke email-domains reply-capture verify
- **`conduyt-crm-pp-cli email-domains remove`** - Remove email domain
- **`conduyt-crm-pp-cli email-domains update`** - Update email domain settings
- **`conduyt-crm-pp-cli email-domains verify`** - Verify DNS configuration for email domain

### email-signature

Manage email signature

- **`conduyt-crm-pp-cli email-signature`** - The 16 built-in designs and the 20 copied-style designs (the public generator's template file), each with its own accent, and the social networks a signature can link (the keys overrides.social takes). The same for every workspace. A browser session or an API key with the emails scope.

### emails

Send individual and bulk emails

- **`conduyt-crm-pp-cli emails create-sequence`** - One model for Email and SMS sequences. Steps send saved templates (Email Templates or SMS Templates of the
sequence's type) a number of days apart inside the send window; nothing is written inline. `type` is fixed at
creation. Runs stop on a reply, a booked meeting, an opt-out (STOP for SMS), a closed deal (when
`autoUnenrollOnDealClosed`), or a removal; see `GET /emails/sequences/{id}/enrollments` for how each run ended.
- **`conduyt-crm-pp-cli emails create-template`** - Email templates built in the editor keep their builder state in hidden markers: CONDUYT_BLOCKS and CONDUYT_META
(subject B, preheader, custom head HTML, footer HTML). Metadata precedence: the CONDUYT_META marker in bodyHtml
wins over one in body; the editor writes it to bodyHtml only, and body's marker is read only when bodyHtml has
none. A request that would leave DIFFERENT markers in body and bodyHtml is refused with 400
template_meta_conflict ("body and bodyHtml both carry a CONDUYT_META marker and they differ"). Stored rows are
never rewritten.

Pasted HTML (email only): send bodyMode "html" with html (your own designed email, up to 100 KB). The
stored bodyHtml is built from html: the builder's blocks (from bodyHtml in the request, else the stored row)
are kept, and body defaults to the HTML's visible text when none is sent. Sends use the pasted HTML as the
body with only what every email gets: merge fields rendered, the preheader / head / footer HTML from the
metadata, open tracking, and an unsubscribe link unless the HTML already carries {{unsubscribe_url}} (or
{{unsubscribeUrl}}). bodyMode "blocks" takes the builder's bodyHtml; switching back keeps the pasted HTML
(returned as html). Refusals (400, nothing written): html_required (bodyMode html without html, or empty),
html_too_large (over 100 KB: Gmail clips emails over 102 KB), html_script (a <script> anywhere in the HTML), html_without_html_mode (html
without bodyMode html), invalid_body_mode (not "blocks" / "html", or on an SMS template), body_html_required
(bodyMode blocks without bodyHtml), body_mode_conflict (bodyMode blocks with html-mode metadata in bodyHtml),
html_conditional_comment (an Outlook conditional comment that ends before its <![endif]-->, is never closed, or
has an unreadable condition), html_office_markup (VML / Office elements outside an Outlook-only block),
html_duplicate_attribute (an element inside an Outlook-only block with the same attribute twice).
A template returned by POST, PATCH or GET /emails/templates/{id} carries bodyMode ("blocks" / "html"; null
for SMS) and html (the pasted HTML in html mode, the kept copy in blocks mode, else null); a LIST row carries
bodyMode only (fetch the template by id for html).
- **`conduyt-crm-pp-cli emails delete-template`** - Delete an email template
- **`conduyt-crm-pp-cli emails enroll-in-sequence`** - Enroll contacts in a sequence
- **`conduyt-crm-pp-cli emails get-health`** - List emails health
- **`conduyt-crm-pp-cli emails get-sequence`** - Get an email sequence by ID
- **`conduyt-crm-pp-cli emails get-sequence-stats`** - Get sequence performance stats
- **`conduyt-crm-pp-cli emails get-sequences-senders`** - List emails sequences senders
- **`conduyt-crm-pp-cli emails get-template`** - Get an email template by ID
- **`conduyt-crm-pp-cli emails get-templates-id-usage`** - Get emails templates id usage
- **`conduyt-crm-pp-cli emails list`** - List email messages
- **`conduyt-crm-pp-cli emails list-sequence-enrollments`** - List enrollments for a sequence
- **`conduyt-crm-pp-cli emails list-sequences`** - List email sequences
- **`conduyt-crm-pp-cli emails list-templates`** - List email templates
- **`conduyt-crm-pp-cli emails post-sequences-id-pick-from-list`** - Create / invoke emails sequences id pick-from-list
- **`conduyt-crm-pp-cli emails post-sequences-id-preflight`** - Create / invoke emails sequences id preflight
- **`conduyt-crm-pp-cli emails test-send-template`** - Send a test email from a template
- **`conduyt-crm-pp-cli emails unenroll-from-sequence`** - Unenroll contacts from a sequence
- **`conduyt-crm-pp-cli emails update-sequence`** - Any field but `type` (400 when sent). `archived: true` turns the sequence Off and hides it from lists
(`GET /emails/sequences?archived=true` shows archived ones); `archived: false` restores it. Changing
`senderUserId` requires an active member.
- **`conduyt-crm-pp-cli emails update-template`** - Email templates built in the editor keep their builder state in hidden markers: CONDUYT_BLOCKS and CONDUYT_META
(subject B, preheader, custom head HTML, footer HTML). Metadata precedence: the CONDUYT_META marker in bodyHtml
wins over one in body; the editor writes it to bodyHtml only, and body's marker is read only when bodyHtml has
none. A request that would leave DIFFERENT markers in body and bodyHtml is refused with 400
template_meta_conflict ("body and bodyHtml both carry a CONDUYT_META marker and they differ"). Stored rows are
never rewritten.

Pasted HTML (email only): send bodyMode "html" with html (your own designed email, up to 100 KB). The
stored bodyHtml is built from html: the builder's blocks (from bodyHtml in the request, else the stored row)
are kept, and body defaults to the HTML's visible text when none is sent. Sends use the pasted HTML as the
body with only what every email gets: merge fields rendered, the preheader / head / footer HTML from the
metadata, open tracking, and an unsubscribe link unless the HTML already carries {{unsubscribe_url}} (or
{{unsubscribeUrl}}). bodyMode "blocks" takes the builder's bodyHtml; switching back keeps the pasted HTML
(returned as html). Refusals (400, nothing written): html_required (bodyMode html without html, or empty),
html_too_large (over 100 KB: Gmail clips emails over 102 KB), html_script (a <script> anywhere in the HTML), html_without_html_mode (html
without bodyMode html), invalid_body_mode (not "blocks" / "html", or on an SMS template), body_html_required
(bodyMode blocks without bodyHtml), body_mode_conflict (bodyMode blocks with html-mode metadata in bodyHtml),
html_conditional_comment (an Outlook conditional comment that ends before its <![endif]-->, is never closed, or
has an unreadable condition), html_office_markup (VML / Office elements outside an Outlook-only block),
html_duplicate_attribute (an element inside an Outlook-only block with the same attribute twice).
A template returned by POST, PATCH or GET /emails/templates/{id} carries bodyMode ("blocks" / "html"; null
for SMS) and html (the pasted HTML in html mode, the kept copy in blocks mode, else null); a LIST row carries
bodyMode only (fetch the template by id for html).

### errors

Manage errors

- **`conduyt-crm-pp-cli errors`** - Create / invoke errors log

### feature-interest

Manage feature interest

- **`conduyt-crm-pp-cli feature-interest`** - Create / invoke feature-interest

### files

File uploads and attachments

- **`conduyt-crm-pp-cli files create-record`** - Create a file attachment record
- **`conduyt-crm-pp-cli files delete`** - Delete a file attachment
- **`conduyt-crm-pp-cli files list`** - List file attachments
- **`conduyt-crm-pp-cli files post-upload-finalize`** - Create / invoke files upload finalize
- **`conduyt-crm-pp-cli files post-upload-intent`** - Create / invoke files upload intent
- **`conduyt-crm-pp-cli files upload`** - Upload a file

### forms

Lead capture forms and submissions

- **`conduyt-crm-pp-cli forms create`** - Create a form
- **`conduyt-crm-pp-cli forms delete`** - Archive a form
- **`conduyt-crm-pp-cli forms get`** - Get a form by ID
- **`conduyt-crm-pp-cli forms list`** - List forms
- **`conduyt-crm-pp-cli forms update`** - Update a form

### generated-documents

Manage generated documents

- **`conduyt-crm-pp-cli generated-documents delete-id`** - Delete generated-documents id
- **`conduyt-crm-pp-cli generated-documents get`** - List generated-documents
- **`conduyt-crm-pp-cli generated-documents get-id`** - Get generated-documents id
- **`conduyt-crm-pp-cli generated-documents patch-id`** - Update generated-documents id
- **`conduyt-crm-pp-cli generated-documents post`** - Create / invoke generated-documents

### groups

Manage groups

- **`conduyt-crm-pp-cli groups delete-id`** - Delete groups id
- **`conduyt-crm-pp-cli groups get`** - List groups
- **`conduyt-crm-pp-cli groups patch-id`** - Update groups id
- **`conduyt-crm-pp-cli groups post`** - Create / invoke groups

### health

Manage health

- **`conduyt-crm-pp-cli health`** - List health

### imports

CSV import jobs with mapping and deduplication

- **`conduyt-crm-pp-cli imports create`** - Create an import job
- **`conduyt-crm-pp-cli imports get`** - Get import job status
- **`conduyt-crm-pp-cli imports get-users`** - List imports users
- **`conduyt-crm-pp-cli imports list`** - List import jobs
- **`conduyt-crm-pp-cli imports post-preflight`** - Create / invoke imports preflight
- **`conduyt-crm-pp-cli imports post-upload-url`** - Create / invoke imports upload-url
- **`conduyt-crm-pp-cli imports upload-file`** - Upload a CSV file for import

### integrations

Third-party integrations (Zapier, etc.)

- **`conduyt-crm-pp-cli integrations connect`** - Connect an integration
- **`conduyt-crm-pp-cli integrations create-form-intake`** - Creates a per-form intake address. The response carries the full `url` (the credential; the list returns it too while the intake is active); paste it into the form plugin's webhook settings. Requires settings.edit, contacts.create and contacts.edit (the address creates and updates contacts) and active billing; API keys need the `form-intakes` AND `contacts` scopes on a write or admin tier key. At most 100 active addresses.
- **`conduyt-crm-pp-cli integrations create-zapier-subscription`** - Create a Zapier webhook subscription
- **`conduyt-crm-pp-cli integrations delete-zapier-subscription`** - Delete a Zapier subscription
- **`conduyt-crm-pp-cli integrations disconnect`** - Disconnect an integration
- **`conduyt-crm-pp-cli integrations get-calendly-event-types`** - List integrations calendly event-types
- **`conduyt-crm-pp-cli integrations get-calendly-status`** - List integrations calendly status
- **`conduyt-crm-pp-cli integrations get-form-intake-status`** - Whether the account has any ACTIVE WordPress forms intake address, and how many. Answers a boolean and a count only, never an intake, URL or token (the intake list returns the full URLs and needs the intake-address authority; this needs only settings.view). The Connections page's WordPress forms row reads it.
- **`conduyt-crm-pp-cli integrations get-health`** - List integrations health
- **`conduyt-crm-pp-cli integrations get-mailchimp-audiences`** - List integrations mailchimp audiences
- **`conduyt-crm-pp-cli integrations get-mailchimp-settings`** - List integrations mailchimp settings
- **`conduyt-crm-pp-cli integrations get-slack-channels`** - List integrations slack channels
- **`conduyt-crm-pp-cli integrations get-zapier-sample-data`** - Get sample data for a Zapier event
- **`conduyt-crm-pp-cli integrations list`** - List active integrations
- **`conduyt-crm-pp-cli integrations list-form-intakes`** - Every ACTIVE intake address of the account (newest first; at most 100 exist), then the revoked history (the newest 200 revocations). Each active row carries its full `url` (the credential, to paste into the form plugin); a revoked row's `url` is null. The URLs are credentials that create and edit contacts, so listing needs what creating needs: settings.edit, contacts.create and contacts.edit; API keys need the `form-intakes` AND `contacts` scopes on a write or admin tier key.
- **`conduyt-crm-pp-cli integrations list-zapier-subscriptions`** - List Zapier webhook subscriptions
- **`conduyt-crm-pp-cli integrations patch-calendly-event-types`** - Update integrations calendly event-types
- **`conduyt-crm-pp-cli integrations post-calendly-disconnect`** - Create / invoke integrations calendly disconnect
- **`conduyt-crm-pp-cli integrations post-dpp-ingest`** - Create / invoke integrations dpp ingest
- **`conduyt-crm-pp-cli integrations post-mailchimp-confirm-reconciliation`** - Create / invoke integrations mailchimp confirm-reconciliation
- **`conduyt-crm-pp-cli integrations post-mailchimp-push`** - Create / invoke integrations mailchimp push
- **`conduyt-crm-pp-cli integrations post-mailchimp-reconcile-statuses`** - Create / invoke integrations mailchimp reconcile-statuses
- **`conduyt-crm-pp-cli integrations post-mailchimp-unsubscribes-retry`** - Create / invoke integrations mailchimp unsubscribes retry
- **`conduyt-crm-pp-cli integrations post-posthog-intent`** - Create / invoke integrations posthog intent
- **`conduyt-crm-pp-cli integrations post-slack-settings`** - Create / invoke integrations slack settings
- **`conduyt-crm-pp-cli integrations post-slack-test`** - Create / invoke integrations slack test
- **`conduyt-crm-pp-cli integrations put-mailchimp-settings`** - Update integrations mailchimp settings
- **`conduyt-crm-pp-cli integrations revoke-form-intake`** - Revokes the address: it answers 404 from now on. The row is kept (status "revoked", revokedAt) for history. Revoking a revoked address returns it unchanged. Requires settings.edit; API keys need the `form-intakes` scope.

### invoices

Invoice creation, sending, payments, PDF generation

- **`conduyt-crm-pp-cli invoices create`** - Create an invoice
- **`conduyt-crm-pp-cli invoices delete`** - Delete an invoice
- **`conduyt-crm-pp-cli invoices get`** - Get an invoice by ID
- **`conduyt-crm-pp-cli invoices get-next-number`** - Get the next auto-incremented invoice number
- **`conduyt-crm-pp-cli invoices list`** - List invoices
- **`conduyt-crm-pp-cli invoices update`** - Update an invoice

### knowledge

Manage knowledge

- **`conduyt-crm-pp-cli knowledge delete-sources-id`** - Delete knowledge sources id
- **`conduyt-crm-pp-cli knowledge get-sources`** - List knowledge sources
- **`conduyt-crm-pp-cli knowledge post-sources`** - Create / invoke knowledge sources
- **`conduyt-crm-pp-cli knowledge post-sources-id-reindex`** - Create / invoke knowledge sources id reindex

### lead-pool

Manage lead pool

- **`conduyt-crm-pp-cli lead-pool`** - List lead-pool

### lead-routing

Lead assignment routing rules and assignment history

- **`conduyt-crm-pp-cli lead-routing create-rule`** - Create a lead routing rule
- **`conduyt-crm-pp-cli lead-routing delete-rule`** - Delete a lead routing rule
- **`conduyt-crm-pp-cli lead-routing get-rule`** - Get a lead routing rule
- **`conduyt-crm-pp-cli lead-routing list-lead-assignment-log`** - List lead assignment history
- **`conduyt-crm-pp-cli lead-routing list-rules`** - List lead routing rules
- **`conduyt-crm-pp-cli lead-routing reorder-rules`** - Reorder lead routing rules
- **`conduyt-crm-pp-cli lead-routing test`** - Simulate lead routing
- **`conduyt-crm-pp-cli lead-routing update-rule`** - Update a lead routing rule

### mailbox

Manage mailbox

- **`conduyt-crm-pp-cli mailbox delete-connections-id`** - Delete mailbox connections id
- **`conduyt-crm-pp-cli mailbox get-connections`** - List mailbox connections
- **`conduyt-crm-pp-cli mailbox post-connect-google`** - Create / invoke mailbox connect google
- **`conduyt-crm-pp-cli mailbox post-connect-microsoft`** - Create / invoke mailbox connect microsoft

### me

Manage me

- **`conduyt-crm-pp-cli me get-my-email-signature`** - One signature per workspace member. Returns the saved signature (or, when none is saved yet, the draft a first visit
starts from: saved false, html null), every field's resolved value and source (fields: profile = the user, workspace
= the membership's position and mobile, company = the Company details, signature = department and added networks),
and the context the editor previews with (the caller's profile, position, the account's branding and Company
details, the caller's own active booking pages). signature.html and signature.plain are RENDERED AT READ TIME from
those live sources (exactly what Compose appends on the next send); a profile, membership, branding, Company
details or booking page change shows here with no re-save. When the current sources make the saved signature too
large to render, html and plain are null and renderError says why. A browser session, or an API key acting as its
user (needs the emails scope).
- **`conduyt-crm-pp-cli me put-my-email-signature`** - Saving replaces the caller's one signature in this workspace. Every part may be omitted and takes its default:
designId classic, overrides {} (every field follows its source), the default options, useInCompose true. Only the
choices are stored: the signature is rendered with the public generator's engine from the live sources at every
use (this response, GET, and each send). Compose appends it (POST /messages with signature "new" or "reply"),
rendered at send time, when useInCompose and the matching options.compose flag are on. email is never an override (it is the
sign-in address). Links must be web addresses (overrides.web, banner and meeting links) or profile links (social).
options.addons.book.pageId must be one of the caller's own active booking pages. A browser session, or an API key
acting as its user (emails scope, write tier).

### messages

SMS and email message history

- **`conduyt-crm-pp-cli messages check`** - Grades a draft with the same check the contact composers run as the rep types (the conduyt.com checker engine) and answers the score out of 100, the grade, the verdict, every flagged word with why and safer alternatives, the subject's flagged words (email), the checks, and for a text the engine's count. Nothing is stored and nothing is sent; it only suggests. Merge tokens such as {{contact.firstName}} are never flagged. Every hit offset (start, end) is a Unicode code-point offset into the submitted text, not a UTF-16 index. Any signed-in member may call it; an API key reaches it with the contacts, emails, sms or messages scope (a read-tier key too) when its member may view contacts or email templates. 20 requests per minute per account. Engine 2.1.0 also answers breakdown (the points taken off, grouped), notices and unicodeCulprits (texts only) and the text count's units and estimate; setting clean to true adds cleaned, the cleaner's proposal (never applied). Further fields an engine release adds are additive.
- **`conduyt-crm-pp-cli messages check-links`** - Checks every link in a draft live from the server, so a dead, redirected, insecure or shortened link is caught before Send. Links are read from the body (and the subject of an email, first) after merge tokens are masked the way POST /messages/check masks them, then from `targets` (whole values, never read as prose); duplicates are collapsed; the answer lists them in the order they appear. Only http and https links are fetched: mailto:, tel: and sms: links are not_fetched; a link with a merge token is merge_field (its address is only known at send time); past 20 web links the rest are not_checked. Safe fetch: a raw IP host, or a host that resolves to any loopback, private, carrier-grade NAT, link-local, unique-local, multicast, reserved or cloud metadata address, is never fetched (blocked_private), and every redirect hop is checked the same way; 5 redirects at most; HEAD first, GET only on 405 or 501 with the body read to 64 KiB at most; 3 s per hop and 8 s per message; a fixed User-Agent, no cookies or credentials. Results are remembered 10 minutes per account and address in the server's memory; nothing is stored and nothing is sent. The answer is notices only: it never changes the Message check score. A `draft` (instead of, or with, `body`) is rendered ON THE SERVER into the message its send delivers, by the send's own functions and with the context the send loads, and the links of that message are checked under the same rules: an email draft (kind compose) as POST /messages delivers it (the inserted template's render with its preheader, custom head CSS and footer, the A/B subjects, the contact's merge fields including custom and date fields, the account and the sender, the sender's saved signature rendered now), or a template editor's unsaved template (templateDraft) as the editor's own Send test (POST /emails/templates/test-send) renders it, with its sample contact and the sender's rep profile; a text-only email is read as the HTML the provider builds from its text; an SMS draft (kind sms) as POST /messages/sms/send delivers it, with the File links block of files already uploaded. The HTML is read as a mail client renders it: what its stylesheets (any selector, in the document or in Outlook-only blocks for the Outlook view) or style attributes hide is not a link. The contact's own unsubscribe link and an uploaded file's link are Conduyt's own (not_fetched). A draft needs communications.create (the send's permission), and each resource it names is loaded through its own endpoint's authorization, for a session and an API key alike: a contactId through contacts.view and the contact's visibility (404 otherwise), a templateId through email_templates.view (403 Missing permission: email_templates.view, as GET /emails/templates/{id}), attached files through files.view and GET /files/{id}/download's record authorization (the parent module's view permission and the parent record's visibility; 404 otherwise), the signature through GET /me/email-signature's rule (an active member; 404 not_a_member); an API key also needs exactly what the send the draft mirrors needs, by the same tier and scope rules as calling it (an email: POST /messages, plus GET /me/email-signature with includeSignature and GET /emails/templates/{id} with templateId; a templateDraft: POST /emails/templates/test-send; an SMS: POST /messages/sms/send, plus GET /files/{id} for each attached file), else 403; a missing or inactive template is a 404 and a corrupt template or a signature too large a 422, as the send. Same access as POST /messages/check (any signed-in member; an API key with the contacts, emails, sms or messages scope, a read-tier key too, when its member may view contacts or email templates). 30 requests per minute per account, a draft included. Budgets: the whole check ends 8 s after the request started: one deadline covers every stage of a draft (its reads, the signature, rendering, parsing, selector matching and the fetches), and a draft not rendered and read by then answers one row (url This message, not_checked, Could not finish the check in time) with truncated true; two drafts are read at once on a server (another answers one not_checked row, Checker busy: try again); a draft's read stops at 200 links (anchors and addresses in the text together), 20,000 HTML nodes, 2,000 stylesheet selectors, 50,000 matched elements of all selectors together and 1 MiB of visible text, and an answer has 200 rows at most: past a cap, the first 199 links, one row (url This message, not_checked, Too large to check fully) and truncated true. A stylesheet selector with :has(), with :nth-child(... of S) or with more than 8 compound selectors is not matched: the read is then truncated the same way (nothing that stylesheet could hide is fetched). The HTML is read in an isolated worker that is stopped at the read deadline (4 s after the start), which then answers the Could not finish the check in time row. A draft's answer carries resourceVersion.
- **`conduyt-crm-pp-cli messages create`** - Create a message record
- **`conduyt-crm-pp-cli messages get-sms`** - Get an SMS message by ID
- **`conduyt-crm-pp-cli messages list`** - List messages
- **`conduyt-crm-pp-cli messages send-sms`** - Send an SMS message

### notes

Notes attached to contacts or deals

- **`conduyt-crm-pp-cli notes create`** - Body is capped at 50 KB. Returns 413 if exceeded.
- **`conduyt-crm-pp-cli notes delete`** - Delete a note
- **`conduyt-crm-pp-cli notes get`** - Get a note by ID
- **`conduyt-crm-pp-cli notes list`** - List notes
- **`conduyt-crm-pp-cli notes update`** - Update a note

### notifications

In-app notifications

- **`conduyt-crm-pp-cli notifications apply-profile`** - Apply notification profile
- **`conduyt-crm-pp-cli notifications create`** - Create a notification
- **`conduyt-crm-pp-cli notifications get-digest-settings`** - Get notification digest settings
- **`conduyt-crm-pp-cli notifications list`** - List notifications
- **`conduyt-crm-pp-cli notifications list-profiles`** - List notification profiles
- **`conduyt-crm-pp-cli notifications mark-all-read`** - Mark all notifications as read
- **`conduyt-crm-pp-cli notifications mark-read`** - Mark a notification as read
- **`conduyt-crm-pp-cli notifications replace-profiles`** - Replace notification profiles
- **`conduyt-crm-pp-cli notifications update-digest-settings`** - Update notification digest settings

### phone-numbers

Manage phone numbers

- **`conduyt-crm-pp-cli phone-numbers`** - List phone-numbers

### pipelines

Sales pipeline and stage management

- **`conduyt-crm-pp-cli pipelines create`** - Requires owner or admin role. Subject to plan limits.
- **`conduyt-crm-pp-cli pipelines delete`** - Delete a pipeline
- **`conduyt-crm-pp-cli pipelines get`** - Get a pipeline by ID
- **`conduyt-crm-pp-cli pipelines list`** - List pipelines with stages
- **`conduyt-crm-pp-cli pipelines reorder`** - Writes every active pipeline's position in ONE transaction. order must list every active pipeline id exactly once; expectedRevision is the order the caller last read (the ids by position, comma-joined, as GET /pipelines returns them). Owner / admin.
- **`conduyt-crm-pp-cli pipelines update`** - Update a pipeline

### playbook-enrollments

Manage playbook enrollments

- **`conduyt-crm-pp-cli playbook-enrollments get`** - List playbook-enrollments
- **`conduyt-crm-pp-cli playbook-enrollments get-id`** - Get playbook-enrollments id
- **`conduyt-crm-pp-cli playbook-enrollments post`** - Create / invoke playbook-enrollments
- **`conduyt-crm-pp-cli playbook-enrollments post-id`** - Create / invoke playbook-enrollments id

### playbooks

Manage playbooks

- **`conduyt-crm-pp-cli playbooks delete-id`** - Delete playbooks id
- **`conduyt-crm-pp-cli playbooks get`** - List playbooks
- **`conduyt-crm-pp-cli playbooks get-id`** - Get playbooks id
- **`conduyt-crm-pp-cli playbooks patch-id`** - Update playbooks id
- **`conduyt-crm-pp-cli playbooks post`** - Create / invoke playbooks

### pool-filter-sets

Manage pool filter sets

- **`conduyt-crm-pp-cli pool-filter-sets delete-id`** - Delete pool-filter-sets id
- **`conduyt-crm-pp-cli pool-filter-sets get`** - List pool-filter-sets
- **`conduyt-crm-pp-cli pool-filter-sets patch-id`** - Update pool-filter-sets id
- **`conduyt-crm-pp-cli pool-filter-sets post`** - Create / invoke pool-filter-sets

### products

Product catalog for invoices

- **`conduyt-crm-pp-cli products create`** - Create a product
- **`conduyt-crm-pp-cli products delete`** - Delete a product
- **`conduyt-crm-pp-cli products get`** - Get a product by ID
- **`conduyt-crm-pp-cli products list`** - List products
- **`conduyt-crm-pp-cli products update`** - Update a product

### public

Unauthenticated public endpoints (booking, form submit)

- **`conduyt-crm-pp-cli public bind-booking-documents`** - Public, no authentication — binds pointers minted for THIS booking. Each pointer is proven (the minted row, the object's presence and size, the validated bytes copied to a final key), attached to the lead's files and recorded on the booking; idempotent by pointer. A pointer that cannot be bound is listed in skipped with a reason, never a 500. 409 documents_in_progress while another bind runs, 409 booking_changed when the booking's identities moved (retry), 410 booking_gone when the booking can no longer take documents, 503 while the booking is being erased or merged.
- **`conduyt-crm-pp-cli public book-appointment`** - Book an appointment via public page
- **`conduyt-crm-pp-cli public get-booking-page`** - Get a public booking page by slug
- **`conduyt-crm-pp-cli public get-booking-routing-slug`** - Get public booking routing slug
- **`conduyt-crm-pp-cli public get-booking-slots`** - Get available time slots for a booking page
- **`conduyt-crm-pp-cli public get-booking-slug-embed`** - Get public booking slug embed
- **`conduyt-crm-pp-cli public get-booking-slug-embed-badge`** - Get public booking slug embed badge
- **`conduyt-crm-pp-cli public get-booking-slug-embed-popup`** - Get public booking slug embed popup
- **`conduyt-crm-pp-cli public get-booking-slug-frame-policy`** - Get public booking slug frame-policy
- **`conduyt-crm-pp-cli public get-booking-slug-ical`** - Get public booking slug ical
- **`conduyt-crm-pp-cli public get-branding-account-id-icon`** - Get public branding accountId icon
- **`conduyt-crm-pp-cli public get-screen-share-code`** - Get public screen-share code
- **`conduyt-crm-pp-cli public mint-booking-document-upload`** - Public, no authentication — for a booking THIS page made. Answers a short-lived, size-bound presigned PUT into private storage; the browser PUTs the bytes there, then binds the pointer with the documents operation. 404 when the page does not take documents or the booking is not this page's live booking; 409 at the per-booking cap; 503 while the booking is being erased or merged, or when uploads are unavailable.
- **`conduyt-crm-pp-cli public post-booking-routing-slug`** - Create / invoke public booking routing slug
- **`conduyt-crm-pp-cli public post-booking-slug-waitlist`** - Create / invoke public booking slug waitlist
- **`conduyt-crm-pp-cli public post-contact`** - Create / invoke public contact
- **`conduyt-crm-pp-cli public post-screen-share-code`** - Create / invoke public screen-share code
- **`conduyt-crm-pp-cli public post-screen-share-code-state`** - Create / invoke public screen-share code state
- **`conduyt-crm-pp-cli public post-subscribe`** - Create / invoke public subscribe

### push

Manage push

- **`conduyt-crm-pp-cli push get-public-key`** - Get VAPID public key for web push
- **`conduyt-crm-pp-cli push subscribe`** - Subscribe to web push notifications
- **`conduyt-crm-pp-cli push unsubscribe`** - Unsubscribe from web push

### quick-connects

Manage quick connects

- **`conduyt-crm-pp-cli quick-connects delete-id`** - Delete quick-connects id
- **`conduyt-crm-pp-cli quick-connects get`** - List quick-connects
- **`conduyt-crm-pp-cli quick-connects patch-id`** - Update quick-connects id
- **`conduyt-crm-pp-cli quick-connects post`** - Create / invoke quick-connects

### quick-notes

Manage quick notes

- **`conduyt-crm-pp-cli quick-notes delete-id`** - Delete quick-notes id
- **`conduyt-crm-pp-cli quick-notes get`** - List quick-notes
- **`conduyt-crm-pp-cli quick-notes patch-id`** - Update quick-notes id
- **`conduyt-crm-pp-cli quick-notes post`** - Create / invoke quick-notes

### realtime

Manage realtime

- **`conduyt-crm-pp-cli realtime`** - List realtime token

### remote-assist

Manage remote assist

- **`conduyt-crm-pp-cli remote-assist post-end`** - Create / invoke remote-assist end
- **`conduyt-crm-pp-cli remote-assist post-start`** - Create / invoke remote-assist start

### rep-shifts

Manage rep shifts

- **`conduyt-crm-pp-cli rep-shifts delete-id`** - Delete rep-shifts id
- **`conduyt-crm-pp-cli rep-shifts get`** - List rep-shifts
- **`conduyt-crm-pp-cli rep-shifts patch-id`** - Update rep-shifts id
- **`conduyt-crm-pp-cli rep-shifts post`** - Create / invoke rep-shifts

### reporting

Manage reporting

- **`conduyt-crm-pp-cli reporting get-email`** - List reporting email
- **`conduyt-crm-pp-cli reporting get-pipeline`** - List reporting pipeline
- **`conduyt-crm-pp-cli reporting get-sms`** - List reporting sms

### reports

Pipeline, revenue, activity, team, and custom reports

- **`conduyt-crm-pp-cli reports create-custom`** - Create a custom report
- **`conduyt-crm-pp-cli reports delete-custom`** - Delete a custom report
- **`conduyt-crm-pp-cli reports delete-subscriptions-sid`** - Delete reports subscriptions sid
- **`conduyt-crm-pp-cli reports get-activity`** - Activity report
- **`conduyt-crm-pp-cli reports get-agent-performance`** - List reports agent-performance
- **`conduyt-crm-pp-cli reports get-appointments`** - List reports appointments
- **`conduyt-crm-pp-cli reports get-attribution`** - List reports attribution
- **`conduyt-crm-pp-cli reports get-call-performance`** - List reports call-performance
- **`conduyt-crm-pp-cli reports get-calls`** - List reports calls
- **`conduyt-crm-pp-cli reports get-custom`** - Get a custom report by ID
- **`conduyt-crm-pp-cli reports get-custom-fields`** - List reports custom fields
- **`conduyt-crm-pp-cli reports get-custom-id-subscriptions`** - Get reports custom id subscriptions
- **`conduyt-crm-pp-cli reports get-deliveries-did-files-index`** - Get reports deliveries did files index
- **`conduyt-crm-pp-cli reports get-dialer-agent-hourly`** - List reports dialer agent-hourly
- **`conduyt-crm-pp-cli reports get-dialer-awards`** - List reports dialer awards
- **`conduyt-crm-pp-cli reports get-dialer-dispositions`** - List reports dialer dispositions
- **`conduyt-crm-pp-cli reports get-dialer-list-performance`** - List reports dialer list-performance
- **`conduyt-crm-pp-cli reports get-email-providers`** - List reports email-providers
- **`conduyt-crm-pp-cli reports get-email-templates`** - List reports email-templates
- **`conduyt-crm-pp-cli reports get-funnel`** - List reports funnel
- **`conduyt-crm-pp-cli reports get-pipeline`** - Pipeline performance report
- **`conduyt-crm-pp-cli reports get-revenue`** - Revenue report
- **`conduyt-crm-pp-cli reports get-sms-delivery`** - List reports sms-delivery
- **`conduyt-crm-pp-cli reports get-sms-drip-conversion`** - List reports sms-drip-conversion
- **`conduyt-crm-pp-cli reports get-sms-templates`** - List reports sms-templates
- **`conduyt-crm-pp-cli reports get-speed-to-lead`** - List reports speed-to-lead
- **`conduyt-crm-pp-cli reports get-subscriptions-sid`** - Get reports subscriptions sid
- **`conduyt-crm-pp-cli reports get-subscriptions-sid-deliveries`** - Get reports subscriptions sid deliveries
- **`conduyt-crm-pp-cli reports get-targets`** - List reports targets
- **`conduyt-crm-pp-cli reports get-team`** - Team performance report
- **`conduyt-crm-pp-cli reports list-custom`** - List saved custom reports
- **`conduyt-crm-pp-cli reports patch-subscriptions-sid`** - Update reports subscriptions sid
- **`conduyt-crm-pp-cli reports post-custom-id-export`** - Create / invoke reports custom id export
- **`conduyt-crm-pp-cli reports post-custom-id-subscriptions`** - Create / invoke reports custom id subscriptions
- **`conduyt-crm-pp-cli reports post-custom-preview`** - Create / invoke reports custom preview
- **`conduyt-crm-pp-cli reports post-subscriptions-sid-run`** - Create / invoke reports subscriptions sid run
- **`conduyt-crm-pp-cli reports put-targets`** - Update reports targets
- **`conduyt-crm-pp-cli reports run-custom`** - Execute a custom report and return results
- **`conduyt-crm-pp-cli reports update-custom`** - Update a custom report

### ring-groups

Manage ring groups

- **`conduyt-crm-pp-cli ring-groups delete-id`** - Delete ring-groups id
- **`conduyt-crm-pp-cli ring-groups get`** - List ring-groups
- **`conduyt-crm-pp-cli ring-groups get-id`** - Get ring-groups id
- **`conduyt-crm-pp-cli ring-groups patch-id`** - Update ring-groups id
- **`conduyt-crm-pp-cli ring-groups post`** - Create / invoke ring-groups
- **`conduyt-crm-pp-cli ring-groups post-release-number`** - Create / invoke ring-groups release-number

### saved-filters

Manage saved filters

- **`conduyt-crm-pp-cli saved-filters delete-id`** - Saved filters are personal Smart Views now: delete the migrated view with DELETE /api/v1/smart-views/:id. Every call answers 410 saved_filters_retired.
- **`conduyt-crm-pp-cli saved-filters get`** - List saved-filters
- **`conduyt-crm-pp-cli saved-filters patch-id`** - Saved filters are personal Smart Views now: edit the migrated view with PATCH /api/v1/smart-views/:id (its id is listed by GET /api/v1/smart-views). Every call answers 410 saved_filters_retired.
- **`conduyt-crm-pp-cli saved-filters post`** - Saved filters are personal Smart Views now: create one with POST /api/v1/smart-views (personal: true keeps it private for an admin key). Every call answers 410 saved_filters_retired.

### schema

Manage schema

- **`conduyt-crm-pp-cli schema get-api-catalog`** - List schema api-catalog
- **`conduyt-crm-pp-cli schema get-public`** - List schema public

### scim

Manage scim

- **`conduyt-crm-pp-cli scim delete-v2-users-id`** - Delete scim v2 Users id
- **`conduyt-crm-pp-cli scim get-v2-users`** - List scim v2 Users
- **`conduyt-crm-pp-cli scim get-v2-users-id`** - Get scim v2 Users id
- **`conduyt-crm-pp-cli scim patch-v2-users-id`** - Update scim v2 Users id
- **`conduyt-crm-pp-cli scim post-v2-users`** - Create / invoke scim v2 Users
- **`conduyt-crm-pp-cli scim put-v2-users-id`** - Update scim v2 Users id

### scoring-rules

Lead scoring rule management

- **`conduyt-crm-pp-cli scoring-rules create`** - Create a scoring rule
- **`conduyt-crm-pp-cli scoring-rules delete`** - Delete a scoring rule
- **`conduyt-crm-pp-cli scoring-rules list`** - List lead scoring rules
- **`conduyt-crm-pp-cli scoring-rules recalculate-scores`** - Recalculate all contact scores
- **`conduyt-crm-pp-cli scoring-rules simulate`** - Simulate scoring rules for a contact
- **`conduyt-crm-pp-cli scoring-rules update`** - Update a scoring rule

### screen-share

Manage screen share

- **`conduyt-crm-pp-cli screen-share delete-session-code`** - Delete screen-share session code
- **`conduyt-crm-pp-cli screen-share get-seats`** - List screen-share seats
- **`conduyt-crm-pp-cli screen-share get-session-code`** - Get screen-share session code
- **`conduyt-crm-pp-cli screen-share post-seats`** - Create / invoke screen-share seats
- **`conduyt-crm-pp-cli screen-share post-session`** - Create / invoke screen-share session

### settings

Account settings, branding, SMS/Twilio configuration

- **`conduyt-crm-pp-cli settings delete-ai-byo`** - Delete settings ai byo
- **`conduyt-crm-pp-cli settings delete-fromk`** - Delete settings fromk
- **`conduyt-crm-pp-cli settings delete-fromk-webhook`** - Delete settings fromk webhook
- **`conduyt-crm-pp-cli settings delete-project-blue`** - Delete settings project-blue
- **`conduyt-crm-pp-cli settings delete-project-blue-voice`** - Delete settings project-blue voice
- **`conduyt-crm-pp-cli settings delete-warmy`** - Delete settings warmy
- **`conduyt-crm-pp-cli settings delete-whatsapp`** - Delete settings whatsapp
- **`conduyt-crm-pp-cli settings delete-zoom`** - Delete settings zoom
- **`conduyt-crm-pp-cli settings get`** - Includes messageCheck.minimumGrade (admin only; default { enabled: false, channel: { sms: null, email: null }, adminOverride: true }). Not enforced yet.
- **`conduyt-crm-pp-cli settings get-ai`** - List settings ai
- **`conduyt-crm-pp-cli settings get-ai-local-files`** - List settings ai-local-files
- **`conduyt-crm-pp-cli settings get-ai-teammates`** - List settings ai-teammates
- **`conduyt-crm-pp-cli settings get-branding`** - The white-label branding (name, logo, favicon, colours, custom domain) and the account's Company details, the eight fields every member's email signature reads ("From company details"), with the settingsRevision they were read at. Needs settings.view (an API key needs the settings scope).
- **`conduyt-crm-pp-cli settings get-delivery-guard`** - List settings delivery-guard
- **`conduyt-crm-pp-cli settings get-dormancy`** - List settings dormancy
- **`conduyt-crm-pp-cli settings get-fromk`** - List settings fromk
- **`conduyt-crm-pp-cli settings get-fromk-lines`** - List settings fromk lines
- **`conduyt-crm-pp-cli settings get-lead-visibility`** - List settings lead-visibility
- **`conduyt-crm-pp-cli settings get-operations`** - List settings operations
- **`conduyt-crm-pp-cli settings get-project-blue`** - List settings project-blue
- **`conduyt-crm-pp-cli settings get-project-blue-lines`** - List settings project-blue lines
- **`conduyt-crm-pp-cli settings get-project-blue-voice`** - List settings project-blue voice
- **`conduyt-crm-pp-cli settings get-scheduled-export`** - List settings scheduled-export
- **`conduyt-crm-pp-cli settings get-sms`** - Get SMS provider settings
- **`conduyt-crm-pp-cli settings get-twilio`** - Get Twilio configuration
- **`conduyt-crm-pp-cli settings get-warmy`** - List settings warmy
- **`conduyt-crm-pp-cli settings get-warmy-engine`** - List settings warmy-engine
- **`conduyt-crm-pp-cli settings get-whatsapp`** - List settings whatsapp
- **`conduyt-crm-pp-cli settings get-whatsapp-senders`** - List settings whatsapp senders
- **`conduyt-crm-pp-cli settings get-zoom`** - List settings zoom
- **`conduyt-crm-pp-cli settings patch-ai`** - Update settings ai
- **`conduyt-crm-pp-cli settings patch-ai-local-files`** - Update settings ai-local-files
- **`conduyt-crm-pp-cli settings patch-ai-teammates`** - Update settings ai-teammates
- **`conduyt-crm-pp-cli settings patch-delivery-guard`** - Update settings delivery-guard
- **`conduyt-crm-pp-cli settings patch-dormancy`** - Update settings dormancy
- **`conduyt-crm-pp-cli settings patch-lead-visibility`** - Update settings lead-visibility
- **`conduyt-crm-pp-cli settings patch-scheduled-export`** - Update settings scheduled-export
- **`conduyt-crm-pp-cli settings patch-warmy-engine`** - Update settings warmy-engine
- **`conduyt-crm-pp-cli settings post-operations`** - Create / invoke settings operations
- **`conduyt-crm-pp-cli settings post-zoom-test`** - Create / invoke settings zoom test
- **`conduyt-crm-pp-cli settings put-ai-byo`** - Update settings ai byo
- **`conduyt-crm-pp-cli settings put-fromk`** - Update settings fromk
- **`conduyt-crm-pp-cli settings put-fromk-webhook`** - Update settings fromk webhook
- **`conduyt-crm-pp-cli settings put-project-blue`** - Update settings project-blue
- **`conduyt-crm-pp-cli settings put-project-blue-lines-line-id`** - Update settings project-blue lines lineId
- **`conduyt-crm-pp-cli settings put-project-blue-voice`** - Update settings project-blue voice
- **`conduyt-crm-pp-cli settings put-warmy`** - Update settings warmy
- **`conduyt-crm-pp-cli settings put-whatsapp`** - Update settings whatsapp
- **`conduyt-crm-pp-cli settings put-zoom`** - Update settings zoom
- **`conduyt-crm-pp-cli settings test-integration`** - Test an integration connection
- **`conduyt-crm-pp-cli settings test-sms`** - Send a test SMS
- **`conduyt-crm-pp-cli settings test-twilio`** - Test Twilio configuration
- **`conduyt-crm-pp-cli settings update`** - Update account settings
- **`conduyt-crm-pp-cli settings update-branding`** - Two shapes. (1) The white-label fields (JSON or multipart with a logo / favicon upload); needs the premium plan.
(2) The Company details on their own: { companyDetails, expectedSettingsRevision }. Any of the eight fields, each a
string ("" or null clears it); website must be a web address, the four social fields profile links; unknown fields
are refused 422. expectedSettingsRevision is required (400 expected_settings_revision_required) and must match the
revision GET returned (409 stale_settings_revision when another save landed first). companyDetails cannot be sent
together with the white-label fields (400 company_details_alone). Admins only (settings.edit; an API key needs the
settings scope and a write tier). Nothing is re-rendered: every email signature is rendered from the live Company
details at each use, so the next signature any rep reads or sends carries the new values.
- **`conduyt-crm-pp-cli settings update-sms`** - Update SMS provider settings
- **`conduyt-crm-pp-cli settings update-twilio`** - Update Twilio configuration

### short-links

Manage short links

- **`conduyt-crm-pp-cli short-links`** - Create / invoke short-links

### smart-lists

Static contact lists

- **`conduyt-crm-pp-cli smart-lists create`** - Create a smart list
- **`conduyt-crm-pp-cli smart-lists delete-id`** - Delete smart-lists id
- **`conduyt-crm-pp-cli smart-lists get-id`** - Get smart-lists id
- **`conduyt-crm-pp-cli smart-lists list`** - List smart lists (static contact lists)
- **`conduyt-crm-pp-cli smart-lists update`** - Update a smart list

### smart-views

Manage smart views

- **`conduyt-crm-pp-cli smart-views delete-id`** - Delete smart-views id
- **`conduyt-crm-pp-cli smart-views get-dial-order`** - List smart-views dial-order
- **`conduyt-crm-pp-cli smart-views get-id`** - Get smart-views id
- **`conduyt-crm-pp-cli smart-views get-version`** - List smart-views version
- **`conduyt-crm-pp-cli smart-views list`** - List available smart view definitions
- **`conduyt-crm-pp-cli smart-views patch-dial-order`** - Update smart-views dial-order
- **`conduyt-crm-pp-cli smart-views patch-id`** - Update smart-views id
- **`conduyt-crm-pp-cli smart-views patch-reorder`** - Update smart-views reorder
- **`conduyt-crm-pp-cli smart-views post`** - Create / invoke smart-views

### sms

Manage sms

- **`conduyt-crm-pp-cli sms get-send-logs`** - List sms send-logs
- **`conduyt-crm-pp-cli sms get-transports`** - List sms transports
- **`conduyt-crm-pp-cli sms get-whatsapp-templates`** - List sms whatsapp templates
- **`conduyt-crm-pp-cli sms post-send-logs`** - Create / invoke sms send-logs

### sms-providers

Manage sms providers

- **`conduyt-crm-pp-cli sms-providers delete-id`** - Delete sms-providers id
- **`conduyt-crm-pp-cli sms-providers get`** - List sms-providers
- **`conduyt-crm-pp-cli sms-providers get-health`** - List sms-providers health
- **`conduyt-crm-pp-cli sms-providers get-id`** - Get sms-providers id
- **`conduyt-crm-pp-cli sms-providers get-options`** - Provider picker for the SMS provider dropdown: the id and name of every provider the send service can
actually use (active webhook providers whose webhook URL passes the outbound safety check), ordered by
name. Reachable with a key scoped `sms` or `messages`; the caller needs communications.view or
communications.create.

Pagination on this operation is cursor-based and differs from the global `page` / `per_page` rules:
- `limit` (1-50, default 50) is how many providers to return. A `limit` above 50 is capped to 50, not
  rejected; 0, a negative or non-integer value, or a repeated `limit` is a 400.
- `cursor` resumes after the last row the previous page scanned. Pass `meta.nextCursor` back unchanged.
  An empty, malformed or repeated cursor is a 400.
- One request checks at most 250 providers, so a page can hold fewer than `limit` items while more
  remain. Keep paging while `meta.nextCursor` is not null; `meta.truncated` is true exactly when it is set.

A 503 with `Retry-After` means the webhook URLs could not be verified right now (a resolver outage);
the response is never a partial list.
- **`conduyt-crm-pp-cli sms-providers get-template-options`** - The redacted provider list the SMS Templates page loads: each ACTIVE provider's id, name and colour, ordered by name,
for the caller's own account. Nothing else is returned: no webhook URL or headers, field mapping, counts or
credentials. The caller needs email_templates.view OR settings.view (the management route GET /sms-providers
needs settings.view only). An API key reaches it with an `emails`, `sms`, `messages` or `settings` scope (the member behind the key still needs one of the two permissions). Unlike GET /sms-providers/options (the send
picker: usable, URL-safe webhook providers) this lists every active provider, builtin included.
- **`conduyt-crm-pp-cli sms-providers patch-id`** - Update sms-providers id
- **`conduyt-crm-pp-cli sms-providers post`** - Create / invoke sms-providers

### sso

Manage sso

- **`conduyt-crm-pp-cli sso delete-scim-tokens-id`** - Delete sso scim-tokens id
- **`conduyt-crm-pp-cli sso get-connection`** - Answers 200 with `data.connection` set to null when the account has no enabled SSO connection (not a 404). Still 404 when the SSO feature is off for the deployment, and session-only for owners and admins.
- **`conduyt-crm-pp-cli sso get-scim-tokens`** - List sso scim-tokens
- **`conduyt-crm-pp-cli sso post-scim-tokens`** - Create / invoke sso scim-tokens
- **`conduyt-crm-pp-cli sso put-connection`** - Update sso connection

### status-metadata

Manage status metadata

- **`conduyt-crm-pp-cli status-metadata delete-id`** - Delete status-metadata id
- **`conduyt-crm-pp-cli status-metadata get`** - List status-metadata
- **`conduyt-crm-pp-cli status-metadata get-buckets`** - List status-metadata buckets
- **`conduyt-crm-pp-cli status-metadata get-by-stage-stage-id`** - Get status-metadata by-stage stageId
- **`conduyt-crm-pp-cli status-metadata get-decisions`** - List status-metadata decisions
- **`conduyt-crm-pp-cli status-metadata get-id`** - Get status-metadata id
- **`conduyt-crm-pp-cli status-metadata patch-id`** - Update status-metadata id
- **`conduyt-crm-pp-cli status-metadata post`** - Create / invoke status-metadata
- **`conduyt-crm-pp-cli status-metadata post-dry-run`** - Create / invoke status-metadata dry-run
- **`conduyt-crm-pp-cli status-metadata post-import`** - Create / invoke status-metadata import

### support

Manage support

- **`conduyt-crm-pp-cli support post-requests`** - Create / invoke support requests
- **`conduyt-crm-pp-cli support post-upload-url`** - Create / invoke support upload-url

### tags

Tag management and merging

- **`conduyt-crm-pp-cli tags create`** - Create a tag
- **`conduyt-crm-pp-cli tags delete`** - Delete a tag
- **`conduyt-crm-pp-cli tags get-id`** - Get tags id
- **`conduyt-crm-pp-cli tags list`** - List tags
- **`conduyt-crm-pp-cli tags merge`** - Merge two tags
- **`conduyt-crm-pp-cli tags update`** - Update a tag

### tasks

Task management with assignment and due dates

- **`conduyt-crm-pp-cli tasks create`** - Creates a task, optionally on a contact and/or a deal. As a Pipeline health quick fix ("Set next action"), send `checkOrigin` + `expectedDealUpdatedAt` with `dealId`: the task is created only while the deal still needs it (see TaskCreate.checkOrigin); the database allows at most one non-done quick-fix task per deal (a second is refused 409 `deal_has_next_action`). Ordinary tasks are unconstrained: they may coexist with a quick-fix task, and may race one.
- **`conduyt-crm-pp-cli tasks delete`** - Delete a task
- **`conduyt-crm-pp-cli tasks get`** - Get a task by ID
- **`conduyt-crm-pp-cli tasks get-assignment-rules`** - List tasks assignment-rules
- **`conduyt-crm-pp-cli tasks get-open-deal-ids`** - List tasks open-deal-ids
- **`conduyt-crm-pp-cli tasks get-queues`** - List tasks queues
- **`conduyt-crm-pp-cli tasks get-recurring`** - List tasks recurring
- **`conduyt-crm-pp-cli tasks get-sla-rules`** - List tasks sla-rules
- **`conduyt-crm-pp-cli tasks get-workload`** - List tasks workload
- **`conduyt-crm-pp-cli tasks list`** - List tasks
- **`conduyt-crm-pp-cli tasks put-assignment-rules`** - Update tasks assignment-rules
- **`conduyt-crm-pp-cli tasks put-queues`** - Update tasks queues
- **`conduyt-crm-pp-cli tasks put-recurring`** - Update tasks recurring
- **`conduyt-crm-pp-cli tasks put-sla-rules`** - Update tasks sla-rules
- **`conduyt-crm-pp-cli tasks update`** - Update a task

### team

Manage team

- **`conduyt-crm-pp-cli team`** - List team members

### up-for-grabs

Manage up for grabs

- **`conduyt-crm-pp-cli up-for-grabs`** - List up-for-grabs

### users

Team member management and invitations

- **`conduyt-crm-pp-cli users get`** - Get a team member by ID
- **`conduyt-crm-pp-cli users get-me`** - Returns the same response as GET /auth/me — this route re-exports that handler, so the two are identical, byte for byte, for the same caller and request. The caller's own user record, the current account, role, permissions and (for an API-key caller) the key's own metadata. The account's display name is at `data.account.name`.
- **`conduyt-crm-pp-cli users get-me-tour`** - List users me tour
- **`conduyt-crm-pp-cli users invite`** - Invite a team member
- **`conduyt-crm-pp-cli users list`** - List team members
- **`conduyt-crm-pp-cli users patch-me`** - Update users me
- **`conduyt-crm-pp-cli users post`** - Create / invoke users
- **`conduyt-crm-pp-cli users post-me-tour`** - Create / invoke users me tour
- **`conduyt-crm-pp-cli users remove`** - Remove a team member
- **`conduyt-crm-pp-cli users update`** - Update a team member

### warmy-engine

Manage warmy engine

- **`conduyt-crm-pp-cli warmy-engine get-health`** - List warmy-engine health
- **`conduyt-crm-pp-cli warmy-engine post-sync`** - Create / invoke warmy-engine sync
- **`conduyt-crm-pp-cli warmy-engine post-templates`** - Create / invoke warmy-engine templates

### webhook-logs

Manage webhook logs

- **`conduyt-crm-pp-cli webhook-logs`** - Request and response bodies are redacted unless the caller has full contact visibility.

### webhooks

Outbound webhook management and logs

- **`conduyt-crm-pp-cli webhooks create`** - URL is validated for SSRF protection. HMAC signing secret is auto-generated.
- **`conduyt-crm-pp-cli webhooks create-legacy`** - Create an outbound webhook endpoint (legacy alias)
- **`conduyt-crm-pp-cli webhooks delete`** - Archive a webhook
- **`conduyt-crm-pp-cli webhooks get`** - Get a webhook by ID
- **`conduyt-crm-pp-cli webhooks inbound-contact`** - Inbound webhook for contact data
- **`conduyt-crm-pp-cli webhooks inbound-deal`** - Inbound webhook for deal data
- **`conduyt-crm-pp-cli webhooks inbound-form-submission`** - Public: no API key; the token in the URL is the credential (create one with POST /integrations/wordpress-forms/intakes). For Gravity Forms (Webhooks Add-On), WPForms (Webhooks addon) and Elementor Pro (Webhook action). Accepts JSON, application/x-www-form-urlencoded (Elementor's default and "Advanced Data" bracket forms) and multipart without files, up to 64 KB. Field names match case-insensitively with spaces and punctuation ignored: first name (firstName, first_name, first, "First Name"), last name, a single name / full_name / "Your name" (split on the first space), email ("E-mail", "Email Address"), phone (tel, telephone, "Phone Number"), company, message (comments, notes), utm_source / utm_medium / utm_campaign / utm_term / utm_content, "Page URL" and "Page Title" (attribution), tags (an array or a comma-separated string), source, form_name (names the timeline entry). Elementor's form_id, Date, Time, User Agent, Remote IP and Powered by are dropped. Any other field is saved to the contact custom field whose key or label matches it; every field is on the contact's timeline entry "Submitted <form name>". Creates or updates the contact by email then phone (a resend never creates a second contact or a second timeline entry) and applies the intake's default tag, source and owner (explicit source / tags in the body win).
- **`conduyt-crm-pp-cli webhooks list`** - List outbound webhooks
- **`conduyt-crm-pp-cli webhooks list-deliveries`** - List deliveries for a webhook
- **`conduyt-crm-pp-cli webhooks list-endpoints`** - List configured webhook endpoints (legacy alias)
- **`conduyt-crm-pp-cli webhooks list-replay-deliveries`** - List webhook deliveries for replay review
- **`conduyt-crm-pp-cli webhooks messages`** - Inbound webhook for message events
- **`conduyt-crm-pp-cli webhooks replay`** - Re-enqueues up to 100 matching deliveries (oldest first). At least one of deliveryIds, eventType or status is required. With no status, only failed deliveries are selected. Replaying succeeded deliveries requires forceSucceededReplay with confirmSucceededReplay REPLAY_SUCCEEDED. Settlement-unconfirmed deliveries (dispatched: the request left and no outcome was recorded; unconfirmed: the stale sweep's terminal form of the same ambiguity) may already have been accepted by the receiver: they are selected only with forceDispatchedReplay and confirmDispatchedReplay REPLAY_DISPATCHED, which states the receiver was checked and did not get them. Manual test deliveries are never replayed. A consent delivery (contact.sms_number_opt_in / opt_out) is replayed only while its consent event is still the latest for the number; one superseded by a newer consent event is refused (consentOrderSupersededIds), so a replay can never reverse the subscriber's current consent state.
- **`conduyt-crm-pp-cli webhooks sms-inbound`** - Twilio inbound SMS webhook
- **`conduyt-crm-pp-cli webhooks sms-status`** - Twilio SMS status callback
- **`conduyt-crm-pp-cli webhooks stripe-invoice`** - Stripe invoice webhook
- **`conduyt-crm-pp-cli webhooks test`** - Sends a manual test delivery (is_test true, header X-Conduyt-Test) whose payload carries the top-level key _test: true. That key is reserved for this route: an event payload carrying a top-level _test key is refused (a terminal delivery whose last error starts with reserved_payload_key), never stripped or sent.
- **`conduyt-crm-pp-cli webhooks update`** - Update a webhook
- **`conduyt-crm-pp-cli webhooks voice-inbound`** - Twilio inbound voice webhook
- **`conduyt-crm-pp-cli webhooks voice-recording`** - Twilio recording callback
- **`conduyt-crm-pp-cli webhooks voice-status`** - Twilio voice status callback
- **`conduyt-crm-pp-cli webhooks voice-voicemail`** - Twilio voicemail callback

### workflows

Simple trigger-action workflows

- **`conduyt-crm-pp-cli workflows create`** - Create a workflow
- **`conduyt-crm-pp-cli workflows delete`** - Delete a workflow
- **`conduyt-crm-pp-cli workflows get`** - Get a workflow by ID
- **`conduyt-crm-pp-cli workflows list`** - List workflows
- **`conduyt-crm-pp-cli workflows update`** - Update a workflow


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`conduyt-crm-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`conduyt-crm-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`conduyt-crm-pp-cli learnings list`** - Inspect taught rows
- **`conduyt-crm-pp-cli learnings forget <query>`** - Undo a teach
- **`conduyt-crm-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`conduyt-crm-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`conduyt-crm-pp-cli teach-pattern`** - Install a query/resource template up front
- **`conduyt-crm-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `CONDUYT_CRM_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `conduyt-crm-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
conduyt-crm-pp-cli accounts get

# JSON for scripting and agents
conduyt-crm-pp-cli accounts get --json

# Filter to specific fields
conduyt-crm-pp-cli accounts get --json --select id,name,status

# Dry run — show the request without sending
conduyt-crm-pp-cli accounts get --dry-run

# Agent mode — JSON + compact + no prompts in one flag
conduyt-crm-pp-cli accounts get --agent
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
conduyt-crm-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `conduyt-crm-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/conduyt-pp-cli/config.toml` on macOS/Linux and `%USERPROFILE%\.config\conduyt-crm-pp-cli\config.toml` on Windows; `--home`, `CONDUYT_CRM_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

`--ca-cert`, `CONDUYT_CRM_CA_CERT`, or `ca_cert` adds a PEM CA bundle to the system trust roots. `--tls-server-name`, `CONDUYT_CRM_TLS_SERVER_NAME`, or `tls_server_name` is the name checked against the certificate when the dialed address does not match it. Precedence is flag, then environment, then the config file. A config file other users can change does not supply these settings. Certificate verification failures are not retried.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `CONDUYT_CRM_BEARER_AUTH` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `conduyt-crm-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `conduyt-crm-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $CONDUYT_CRM_BEARER_AUTH`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **403 'scope not permitted' on a command** — Your cdy_ key lacks that domain scope — an admin grants it in Settings > API Keys (dnc and dialer require explicit scopes).
- **verify-line-type returns 403 'not enabled'** — Enable Phone Line-Type Verification in Settings > Twilio first (tenant-paid feature, default off).
- **429 rate limit on bulk reads** — Run 'sync' once and query the local store (search/sql/analytics) instead of paging the API.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**conduyt-mcp**](https://github.com/ptaramona/conduyt-mcp) — TypeScript
- [**conduyt-crm-cli (prior print)**](https://conduyt-crm-pp-cli) — Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
