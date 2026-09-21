# SendFox CLI

**Use SendFox from the terminal or an agent to manage audiences and campaigns, sync account data locally, and generate evidence-backed growth, health, performance, and migration reports.**

The package includes an agent-ready CLI and MCP server for contacts, lists, campaigns, automations, forms, and account details. It adds safe local analytics that the API does not provide directly—growth history, audience-health cohorts, campaign benchmarks, and Kit migration analysis—while keeping every account write behind explicit approval.

## Reprint status and operating safeguards

This edition covers the documented SendFox OpenAPI v1.4.0 contract: 60 operations. Build it locally with `go build -o sendfox-pp-cli ./cmd/sendfox-pp-cli` and `go build -o sendfox-pp-mcp ./cmd/sendfox-pp-mcp`.

All account writes require explicit `--yes`; delivery, schedules, automation activation, form/domain changes and destructive operations also require `--approve-sensitive`. Preview with `--dry-run` first. Campaign creation without a schedule stays draft. Automation creation defaults inactive. Bulk actions default to server-side preview counts; use `--preview-count=false` only after reviewing targeting. The global `--dry-run` never contacts the server.

The API allows 60 requests/minute per authenticated user. The client defaults below one request/second, honors rate feedback and Retry-After for safe reads, and never automatically replays writes. Other processes share the same upstream quota; the local limiter is not an account-wide coordinator. `contacts bulk-wait` observes an existing asynchronous job with explicit poll and timeout budgets.

Use `capabilities` and resource help to discover the full contract. MCP exposes search, schema lookup and execute primitives with endpoint tools hidden, a campaign-performance intent, and local workflow tools. MCP mutations preview by default; execution needs `confirm=true`, and high-impact operations also need `approve_sensitive=true`. HTTP MCP defaults to loopback and requires a caller token in SENDFOX_MCP_HTTP_TOKEN. Non-loopback binds also require TLS, and every HTTP listener enforces bounded header, request-read and idle timeouts. Prefer stdio for local agent use.

All eight evidence reports run without credentials and without filesystem mutations. `workflow snapshot-save` is their explicit local-write companion. See [EVIDENCE.md](EVIDENCE.md) for snapshots, normalized Kit inputs, completeness rules, and examples. Prior account-snapshot, audience-map, hygiene-report, campaign-digest and launch-plan paths remain as compatibility commands. CSV audit/reconcile/import/onboard and local form generation remain available with current safeguards. Webhook handoff is replaced by an explicit API-gap report.

## Known Gaps

No public contract covers webhook management/event delivery, purchases/orders, reusable snippets/templates, public posts/feed controls, Kit-equivalent aggregate growth analytics, richer behavioral filters, or equivalent automation enrollment/sequence-progress transfer. Reports expose these gaps; rollback artifacts cannot unsend messages or restore unsupported sequence state. Read-only authenticated behavior was verified against the live API at publish time. Account mutations, real DNS, actual recipient selection and delivery remain unverified; automated tests otherwise use synthetic files and loopback mock servers.

## Install

The recommended path installs both the `sendfox-pp-cli` binary and the `pp-sendfox` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install sendfox
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install sendfox --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install sendfox --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install sendfox --agent claude-code
npx -y @mvanhorn/printing-press-library install sendfox --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/sendfox/cmd/sendfox-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/sendfox-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine ./sendfox-pp-cli`. On Unix, mark it executable: `chmod +x ./sendfox-pp-cli`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install sendfox --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-sendfox --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-sendfox --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install sendfox --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/sendfox-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SENDFOX_API_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/sendfox/cmd/sendfox-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "sendfox": {
      "command": "sendfox-pp-mcp",
      "env": {
        "SENDFOX_API_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

```bash
# Check the local CLI without account access.
sendfox-pp-cli doctor --dry-run

# Discover supported campaign commands.
sendfox-pp-cli which campaigns --agent

# Inspect a static campaign readiness report.
sendfox-pp-cli workflow campaign-preflight --input examples/snapshot.json --agent

```

## Unique Features

These workflows combine supplied local evidence into auditable reports and inactive plans.

### SendFox operations
- **`workflow audience-health`** — Find duplicate, invalid, suppressed and unassigned contacts, plus engagement recency cohorts that distinguish never-engaged from unknown.

  _Engagement timestamps form recency cohorts; missing or incomplete activity stays unknown rather than being labeled never-engaged._

  ```bash
  sendfox-pp-cli workflow audience-health --input examples/snapshot.json --agent
  ```
- **`workflow campaign-preflight`** — Check draft content, targeting, suppression evidence and sender readiness before any send.

  _Check draft content, targeting, suppression evidence and sender readiness before any send._

  ```bash
  sendfox-pp-cli workflow campaign-preflight --input examples/snapshot.json --agent
  ```
- **`workflow campaign-review`** — Combine campaign metrics, recipient-weighted benchmarks and trends, link performance and engagement cohorts into an auditable resend plan.

  _Rates are recomputed from counters, recipient-weighted, and compared with campaign medians and timestamped trends._

  ```bash
  sendfox-pp-cli workflow campaign-review --input examples/snapshot.json --agent
  ```
- **`workflow contact-dossier`** — Join contact details, memberships, tags and available engagement into one support packet.

  _Join contact details, memberships, tags and available engagement into one support packet._

  ```bash
  sendfox-pp-cli workflow contact-dossier --input examples/snapshot.json --id 1 --agent
  ```
- **`workflow automation-plan`** — Validate a versioned automation definition and produce inactive creation steps.

  _Validate a versioned automation definition and produce inactive creation steps._

  ```bash
  sendfox-pp-cli workflow automation-plan --input examples/snapshot.json --agent
  ```
- **`workflow export-bundle`** — Produce a read-only local inventory bundle with completeness, provenance and deterministic hashes.

  _This command never writes files; use `snapshot-save` when durable history is intended._

  ```bash
  sendfox-pp-cli workflow export-bundle --input examples/snapshot.json --agent
  ```
- **`workflow growth-report`** — Calculate timestamped audience growth from one snapshot and exact observed list joins, leaves, retention and net growth from two.

  _Metrics carry formulas, windows, numerators, denominators, evidence grades, completeness and warnings. Missing evidence stays null rather than becoming zero._

  ```bash
  sendfox-pp-cli workflow growth-report --input current.json --previous previous.json --window 30d --agent
  ```

### Migration evidence
- **`workflow migration-readiness`** — Compare local platform snapshots with suppression, mapping, delta and cutover gates.

  _Compare local platform snapshots with suppression, mapping, delta and cutover gates._

  ```bash
  sendfox-pp-cli workflow migration-readiness --input examples/snapshot.json --agent
  ```

## Recipes

### Audience audit

```bash
sendfox-pp-cli workflow audience-health --input examples/snapshot.json --agent
```

Report hygiene issues using local snapshot evidence.

### Export inventory

```bash
sendfox-pp-cli workflow export-bundle --input examples/snapshot.json --agent --select data.counts
```

Select only resource counts from the evidence bundle.

### Save snapshot history

```bash
sendfox-pp-cli workflow snapshot-save --input examples/snapshot.json --out ./history --agent
```

Use the explicit local-write command to atomically save a dated, checksummed snapshot in a `0700` directory with a `0600` JSON file. `--out` is required.

### Audience growth

```bash
sendfox-pp-cli workflow growth-report --input current.json --previous previous.json --window 30d --agent
```

Use a single snapshot for timestamped additions and unsubscribes. Add `--previous` for exact observed membership joins, leaves, retention and net growth between two complete snapshots.

### Migration evidence

```bash
sendfox-pp-cli workflow migration-readiness --input examples/snapshot.json --agent
```

Produce a fail-closed readiness report; missing Kit evidence remains unknown.

### Campaign preflight

```bash
sendfox-pp-cli workflow campaign-preflight --input examples/snapshot.json --agent
```

Check content, targeting and sender evidence without sending.

### Campaign review

```bash
sendfox-pp-cli workflow campaign-review --input examples/snapshot.json --agent
```

Read counters and links and produce an unscheduled resend plan.

### Contact dossier

```bash
sendfox-pp-cli workflow contact-dossier --input examples/snapshot.json --id 1 --agent
```

Join the selected synthetic contact with memberships and activity.

### Inactive automation

```bash
sendfox-pp-cli workflow automation-plan --input examples/snapshot.json --agent
```

Compile dependent creation requests with active=false.

### CSV audit

```bash
sendfox-pp-cli contacts audit-csv --file examples/contacts.csv --agent
```

Inspect malformed, duplicate and suppressed rows before any import.

### Draft request preview

```bash
sendfox-pp-cli campaigns create --title "September notes" --subject "September notes" --from-email "editor@example.com" --from-name "Example Editor" --html "<p>News</p>" --dry-run --agent
```

Validate and preview a draft request; no account request is made.

## Usage

Run `sendfox-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, credential-scoped `data-<hash>.db` files, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `SENDFOX_CONFIG_DIR`, `SENDFOX_DATA_DIR`, `SENDFOX_STATE_DIR`, or `SENDFOX_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `SENDFOX_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export SENDFOX_HOME=/srv/sendfox
sendfox-pp-cli doctor
```

Under `SENDFOX_HOME=/srv/sendfox`, the four dirs resolve to `/srv/sendfox/config`, `/srv/sendfox/data`, `/srv/sendfox/state`, and `/srv/sendfox/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "sendfox": {
      "command": "sendfox-pp-mcp",
      "env": {
        "SENDFOX_HOME": "/srv/sendfox"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `SENDFOX_DATA_DIR` overrides an explicit `--home` for that kind. Use `SENDFOX_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `SENDFOX_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `sendfox-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

Authenticated local stores are isolated by a one-way hash of the active credential. A legacy unscoped `data.db` is used only when no credential scope is available; it is never auto-adopted into an authenticated account because its ownership cannot be verified. Run `sendfox-pp-cli sync` to populate the scoped store after upgrading or changing credentials. An explicit `--db` remains operator-controlled.

## Commands

### automation_emails

Manage automation emails

- **`sendfox-pp-cli automation-emails delete`** - Remove an email from an automation
- **`sendfox-pp-cli automation-emails update`** - Update an automation email

### automations

Manage automations

- **`sendfox-pp-cli automations create`** - Create an automation
- **`sendfox-pp-cli automations create-email`** - Add an email to an automation
- **`sendfox-pp-cli automations delete`** - Soft-deletes the automation and cancels all scheduled deliverables
- **`sendfox-pp-cli automations get`** - Returns automation with triggers, items, and campaign stats
- **`sendfox-pp-cli automations list`** - List automations
- **`sendfox-pp-cli automations update`** - Update title, trigger, or active status. Activating reschedules stale deliverables.

### campaigns

Manage campaigns

- **`sendfox-pp-cli campaigns create`** - Creates a campaign as a draft. To send it, use the send endpoint or provide scheduled_at. Subject lines cannot start with "RE:" or "FWD:". At least one list is required if scheduled_at is provided.
- **`sendfox-pp-cli campaigns delete`** - Only draft campaigns (not yet sent) can be deleted. Uses soft delete.
- **`sendfox-pp-cli campaigns get`** - Get a specific campaign
- **`sendfox-pp-cli campaigns get-stats`** - Returns sent count and open/click/bounce/unsubscribe/spam counts and rates, all read from stored counters. Pass `include_link_stats=true` to also get `link_stats` — the campaign's links ranked by click count. That part is opt-in because it counts rows in `email_link_clicks` once per link rather than reading a counter, so leaving it off keeps this endpoint as cheap as it has always been.
- **`sendfox-pp-cli campaigns list`** - Returns a paginated list of campaigns (100 per page)
- **`sendfox-pp-cli campaigns list-engagement`** - Returns the contacts in one of a sent campaign's engagement groups. non_openers covers contacts the campaign actually sent to who did not open it — queued and cancelled deliverables are excluded, since they never had the chance.
- **`sendfox-pp-cli campaigns resend`** - Creates a new draft with the original's content, sender, and exclusions, aimed at the chosen slice of the original audience. Omit scheduled_at to leave it as a draft. The source campaign's lists are deliberately not carried over — the engagement segment is the audience.
- **`sendfox-pp-cli campaigns send`** - Schedules a draft campaign for immediate sending. The campaign must: - Not already be sent or scheduled - Have at least one list assigned - Not be a confirmation (double opt-in) email — those are delivered to each contact as they subscribe, never as a broadcast - The user must not be in a warmup/throttle period All existing abuse prevention applies automatically: content approval workflow, sending throttles, spam detection, and bounce rate monitoring.
- **`sendfox-pp-cli campaigns update`** - Only draft campaigns (not yet sent) can be updated. All fields are optional.

### contact_fields

Manage contact fields

- **`sendfox-pp-cli contact-fields create`** - Creates a custom field for contacts. The `name` is auto-generated from the `label` as a slug. `contact_field_type_id` is a fixed UUID identifying the field's data type. It is the same for every account, and the type *names* are not accepted as input: | Type | `contact_field_type_id` | | ------ | -------------------------------------- | | text | `0abe5601-7738-43c8-858f-81911ecf89ee` | | number | `e43c6471-b3ff-4541-80c5-964c62decd15` | | date | `fe7dd76c-5113-4159-bc7c-0dbf0ed8d404` | The value is immutable after creation and is not returned by any endpoint - responses carry the human-readable `type` (`text`, `number`, `date`) instead.
- **`sendfox-pp-cli contact-fields delete`** - Permanently deletes the contact field
- **`sendfox-pp-cli contact-fields get`** - Get a specific contact field
- **`sendfox-pp-cli contact-fields list`** - Returns custom contact fields defined by the user (20 per page)
- **`sendfox-pp-cli contact-fields update`** - Updates the label and auto-regenerates the name slug. Note: `contact_field_type_id` is immutable and cannot be changed after creation.

### contact_tags

Manage contact tags

- **`sendfox-pp-cli contact-tags create`** - Creates a tag. A brand color is auto-assigned when none is provided. Tag names are unique per account.
- **`sendfox-pp-cli contact-tags delete`** - Deletes the tag. Tagged contacts are kept — only the tag and its attachments (including campaign audience use) are removed.
- **`sendfox-pp-cli contact-tags get`** - Get a contact tag
- **`sendfox-pp-cli contact-tags list`** - Lists the account's tags, newest first, each with its contact count. Distinct from the legacy /tags endpoints, which operate on lists.
- **`sendfox-pp-cli contact-tags update`** - Update a contact tag

### contacts

Manage contacts

- **`sendfox-pp-cli contacts attach-tag`** - Idempotent. Returns the contact's tags after the change.
- **`sendfox-pp-cli contacts batch-import`** - Import up to 1,000 contacts in a single request. Creates new contacts or updates existing ones.
- **`sendfox-pp-cli contacts create`** - Create a new contact
- **`sendfox-pp-cli contacts create-bulk-action`** - Queues one action against every contact the filter matches, applied in chunks in the background. Returns immediately with an id to poll. **Run with `dry_run: true` first.** A dry run reports how many contacts match and changes nothing — it is the only way to catch a filter that matches more than intended, and a bulk write is not undoable. An empty filter (which would match the whole account) is rejected unless it is a dry run. Per-campaign conditions (`opened_campaign_id`, `not_opened_campaign_id`, `clicked_campaign_id`) are available here but not on `GET /contacts`, because they are resolved in the background rather than inside a request. Bulk delete and bulk unsubscribe are deliberately not offered.
- **`sendfox-pp-cli contacts delete`** - Soft-deletes a contact and cancels any scheduled deliverables
- **`sendfox-pp-cli contacts detach-tag`** - Remove a tag from a contact
- **`sendfox-pp-cli contacts get`** - Get a specific contact
- **`sendfox-pp-cli contacts get-activity`** - Returns paginated email deliverables and contact-level engagement summary
- **`sendfox-pp-cli contacts get-bulk-action`** - For a dry run, matched_count is the answer and nothing was modified. A run whose match set exceeds the per-request ceiling fails without applying anything.
- **`sendfox-pp-cli contacts list`** - Returns a paginated list of contacts (100 per page by default, up to 1000 via `per_page`). Supports engagement filtering through `filter[...]` query parameters, so you can answer questions like "who last opened over a year ago" without paging the whole account. All filter conditions are AND-ed. Pass `count_only=true` to get just the number of matches — the cheapest way to size an audience before acting on it.
- **`sendfox-pp-cli contacts list-tags-for`** - List a contact's tags
- **`sendfox-pp-cli contacts list-unsubscribed`** - List unsubscribed contacts
- **`sendfox-pp-cli contacts update`** - Update contact details including name, list memberships, and custom fields

### domains

Manage domains

- **`sendfox-pp-cli domains create`** - Adds a new sender domain and creates the corresponding SendGrid whitelabel domain. Requires an active subscription and SendGrid subuser.
- **`sendfox-pp-cli domains delete`** - Removes the domain from SendGrid and soft-deletes locally
- **`sendfox-pp-cli domains get`** - Returns domain details including DNS records needed for verification
- **`sendfox-pp-cli domains list`** - Returns a paginated list of the user's whitelabel/sender domains
- **`sendfox-pp-cli domains validate`** - Triggers DNS validation for the domain via SendGrid. Returns whether validation passed and any errors for specific DNS records.

### forms

Manage forms

- **`sendfox-pp-cli forms create`** - Creates a subscription form linked to one or more lists. Free users are limited to 1 form.
- **`sendfox-pp-cli forms delete`** - Soft-deletes the form
- **`sendfox-pp-cli forms get`** - Get a specific form
- **`sendfox-pp-cli forms list`** - List forms
- **`sendfox-pp-cli forms update`** - Update a form

### lists

Manage lists

- **`sendfox-pp-cli lists add-contact-to`** - Adds an existing contact to a list. If the contact is already in the list, no duplicate is created.
- **`sendfox-pp-cli lists contacts-in`** - Get contacts in a list
- **`sendfox-pp-cli lists create`** - Create a new contact list
- **`sendfox-pp-cli lists delete`** - Soft-deletes a list. Returns 409 if the list is used by forms, landing pages, or automations.
- **`sendfox-pp-cli lists get`** - Returns list details including average open and click rates
- **`sendfox-pp-cli lists list-lists`** - List contact lists
- **`sendfox-pp-cli lists remove-contact-from`** - Remove a contact from a list
- **`sendfox-pp-cli lists update`** - Update a contact list

### me

Manage me

- **`sendfox-pp-cli me`** - Get current user information

### unsubscribe

Manage unsubscribe

- **`sendfox-pp-cli unsubscribe`** - Unsubscribe a contact by email


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`sendfox-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`sendfox-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`sendfox-pp-cli learnings list`** - Inspect taught rows
- **`sendfox-pp-cli learnings forget <query>`** - Undo a teach
- **`sendfox-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`sendfox-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`sendfox-pp-cli teach-pattern`** - Install a query/resource template up front
- **`sendfox-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `SENDFOX_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `sendfox-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
sendfox-pp-cli automations list

# JSON for scripting and agents
sendfox-pp-cli automations list --json
# Filter to specific fields
sendfox-pp-cli automations list --json --select active,automation_items,automation_triggers

# Dry run — show the request without sending
sendfox-pp-cli automations list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
sendfox-pp-cli automations list --agent
```

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
sendfox-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `sendfox-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/sendfox-pp-cli/config.toml`; `--home`, `SENDFOX_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SENDFOX_API_TOKEN` | per_call | Yes | Set to your API credential. |
| `SENDFOX_BEARER_AUTH` | per_call | No | Compatibility alias for SENDFOX_API_TOKEN; use one token variable. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `sendfox-pp-cli doctor` reports `agentcookie: detected` and `auth status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `sendfox-pp-cli doctor` to check credentials
- Verify the environment variable is set: `sendfox-pp-cli auth status --json`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **HTTP 429** — Reduce --rate-limit and honor Retry-After; concurrent processes share the account budget.
- **HTTP 402 or 403 account_restricted** — Check the paid REST plan and account status; do not retry with altered credentials.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**Activepieces SendFox**](https://github.com/activepieces/activepieces) — TypeScript
- [**Pipedream SendFox**](https://github.com/PipedreamHQ/pipedream) — JavaScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

Compatibility: `auth set-token --stdin` accepts a personal access token only through stdin, never a positional argument. It saves locally without verifying against SendFox. `lists contacts <list_id>` aliases `lists contacts-in`; `webhooks` explains the public API gap. Evidence plans preserve request bodies even with `--agent`; `--select` remains available for deliberate projection.
