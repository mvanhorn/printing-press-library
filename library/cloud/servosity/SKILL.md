---
name: pp-servosity
description: "Every Servosity endpoint as a typed command, plus a local fleet mirror, snapshot history, and cross-engine rollups... Trigger phrases: `what needs my attention on servosity`, `fleet stale backups`, `show me company in servosity`, `triage servosity issues`, `drift since yesterday on servosity`, `use servosity`, `run servosity`."
author: "Damien Stevens"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - servosity-pp-cli
---

# Servosity — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `servosity-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install servosity --cli-only
   ```
2. Verify: `servosity-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Every Servosity REST endpoint becomes a typed Cobra command (Resellers, Companies, three backup engines, Issues, Reports, Admin) with `--json`, `--select`, `--csv`, `--dry-run`, and typed exit codes. A local SQLite mirror makes the fleet queryable offline. The attention command answers 'what needs my attention?' across all admin rollups in one call. The drift command tells you what got worse since yesterday. The backup-facts view unifies classic, restic, and DR engines into one view nothing else exposes.

## When to Use This CLI

Reach for servosity-pp-cli whenever you need to see the Servosity fleet from the terminal: the morning attention sweep, the Friday stale-backup hunt, ad-hoc 'is ACME OK?' checks, batch issue triage, or DRaaS-in-flight oversight. The local SQLite store and snapshot history make it the right tool for any 'what changed since X?' question — those are unanswerable through the web UI.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Loop closure for fleet operators
- **`attention`** — One screen merges admin attention + dirty repos + DRaaS-in-progress + open issues, ranked per-company, and persists each call so tomorrow can drift against today.

  _Use this when an agent or human asks 'what needs my attention right now?' across the whole fleet — it answers in one call instead of four._

  ```bash
  servosity-pp-cli attention --json --select companies.name,companies.score
  ```
- **`triage`** — List open issues with filters, then batch ignore / archive / reactivate / comment in one invocation with --dry-run and typed exit codes.

  _Use this for batch issue actions when the queue is bursty — pipe-friendly, scriptable, idempotent._

  ```bash
  servosity-pp-cli triage --audience support --company 4421 --ignore 18,22,29 --dry-run
  ```
- **`restore-queue list`** — List per-company restore queues across companies the local store knows about; --watch repolls and prints diffs.

  _Use this during an active DR event to keep one terminal pinned on every queue's progress._

  ```bash
  servosity-pp-cli restore-queue list --watch --interval 30s
  ```

### Offline fleet querying
- **`stale-backups`** — Slice the synced /reports/stale-backup-sets/ snapshot by reseller, company, age window, and backup engine — entirely offline once synced. Use --refresh to repull the CSV.

  _Use this on Friday's 'who needs a follow-up?' sweep without burning an API call per slice._

  ```bash
  servosity-pp-cli stale-backups --days 7 --engine restic --json
  ```
- **`drift`** — Diff two snapshots the CLI itself collected (attention, stale, dirty-repos) — show what got worse and what recovered between two timestamps.

  _Use this every morning to surface fleet trend instead of treating every Monday like a fresh slate._

  ```bash
  servosity-pp-cli drift --metric attention --from yesterday --to now --json
  ```
- **`backup-facts`** — Query a unified view (engine + id + company_id + last_successful_at + last_status + size_bytes) over all three backup engines synced into the local store.

  _Use this when you don't care which engine — you just need 'who hasn't backed up successfully since X?' across the whole fleet._

  ```bash
  servosity-pp-cli backup-facts --last-success-before 2026-05-04 --json
  ```
- **`find`** — SQLite FTS5 across companies (name, billing notes), issues (title and comments), and backups (descriptive name, last error) — one query hits the whole fleet.

  _Use this when you remember a phrase but not which entity owned it — one call replaces hunting through three list pages._

  ```bash
  servosity-pp-cli find "image manager" --in issues,backups --json --select hits.resource,hits.snippet
  ```

### Per-company quick view
- **`company show`** — Single command pulls a company's metadata + addresses + contracts + all backups across three engines + open issues + agent sessions into one human or --json view.

  _Use this when a customer asks 'is my backup OK?' — one call, every relevant fact, ready to paste into a ticket._

  ```bash
  servosity-pp-cli company show 4421 --json
  ```

### Tier-One support workflows
- **`clear`** — Resolve one or more names as companies (then resellers) and batch-ignore their active issues until a human-readable time. Defaults to --dry-run for production safety.

  _Use this when a partner is doing planned maintenance and you want their alert noise paused until morning — one command instead of dozens of UI clicks._

  ```bash
  servosity-pp-cli clear "ACME Corp, BDH Technology" --until "6am tomorrow" --dry-run
  ```
- **`stale-issues`** — Pull your FMDB companies, fetch active issues, classify known-safe-to-archive patterns from a shipped rule table, auto-archive the safe ones, ignore non-dashboard noise, and print unknowns for review. Defaults to --dry-run.

  _Use this every weekday before standup to clear the obvious stale noise off your dashboard so triage focuses on what's actually new._

  ```bash
  servosity-pp-cli stale-issues --mine --cutoff "11pm yesterday" --auto-archive-known --dry-run
  ```

## Command Reference

**admin** — Manage admin

- `servosity-pp-cli admin attention-list` — Attention list
- `servosity-pp-cli admin dirty-repos-list` — Dirty repos list
- `servosity-pp-cli admin draas-in-progress-list` — Draas in progress list
- `servosity-pp-cli admin maxio-price-points-create` — Maxio price points create
- `servosity-pp-cli admin maxio-price-points-list` — Maxio price points list
- `servosity-pp-cli admin maxio-price-points-update` — Maxio price points update
- `servosity-pp-cli admin notification-broadcast-create` — Send a notification to all users.
- `servosity-pp-cli admin notification-create` — Send a notification to a user.
- `servosity-pp-cli admin servosity-one-push-message-create` — Servosity one push message create
- `servosity-pp-cli admin servosity-one-push-update-create` — Servosity one push update create
- `servosity-pp-cli admin servosity-one-worker-token-list` — Servosity one worker token list
- `servosity-pp-cli admin support-staff-list` — Support staff list
- `servosity-pp-cli admin users-list` — Users list
- `servosity-pp-cli admin worker-agents-list` — Worker agents list
- `servosity-pp-cli admin worker-run-list` — Worker run list
- `servosity-pp-cli admin worker-run-update` — Worker run update

**agent-login** — Manage agent login

- `servosity-pp-cli agent-login create` — Create
- `servosity-pp-cli agent-login list` — List

**agent-sessions** — Manage agent sessions

- `servosity-pp-cli agent-sessions <agent_session_id>` — Read

**backup-job-report** — Manage backup job report

- `servosity-pp-cli backup-job-report <backup_destination_id> <backup_id> <backup_job_id> <backup_set_id>` — View detailed backup report for a backup job and destination.

**backup-job-report-summary** — Manage backup job report summary

- `servosity-pp-cli backup-job-report-summary <backup_destination_id> <backup_id> <backup_job_id> <backup_set_id>` — View summary backup report for a backup job and destination.

**backup-job-status** — Manage backup job status

- `servosity-pp-cli backup-job-status <backup_id>` — List backup job status for a backup account on a specific date.

**backup-jobs** — Manage backup jobs

- `servosity-pp-cli backup-jobs <backup_id>` — List backup jobs for a backup account.

**backup-plans** — Manage backup plans

- `servosity-pp-cli backup-plans list` — List backup plans.
- `servosity-pp-cli backup-plans read` — View a backup plan.

**backup-search** — Manage backup search

- `servosity-pp-cli backup-search` — List

**backup-sets** — Manage backup sets

- `servosity-pp-cli backup-sets create` — Create a backup-set for a backup account.
- `servosity-pp-cli backup-sets delete` — Delete a backup-set for a backup account.
- `servosity-pp-cli backup-sets list` — List backup-sets for a backup account.
- `servosity-pp-cli backup-sets read` — View a backup-set for a backup account.
- `servosity-pp-cli backup-sets update` — Accepts a json body with the following optional parameters. `ReadOnly`: Boolean `Name`: String Backup set name...

**backups** — Manage backups

- `servosity-pp-cli backups create` — Create a backup account.
- `servosity-pp-cli backups delete` — Delete a backup account, also deleting all backup data.
- `servosity-pp-cli backups list` — List backup accounts.
- `servosity-pp-cli backups mfa-codes` — Mfa codes
- `servosity-pp-cli backups partial-update` — Partial update
- `servosity-pp-cli backups read` — View a backup account.
- `servosity-pp-cli backups update` — Update a backup account.

**companies** — Manage companies

- `servosity-pp-cli companies create` — Create a company.
- `servosity-pp-cli companies delete` — Delete a company, also deleting all backup accounts and backup data.
- `servosity-pp-cli companies fully-managed` — List fully-managed companies.
- `servosity-pp-cli companies fully-managed-ng` — List fully-managed companies.
- `servosity-pp-cli companies list` — List companies.
- `servosity-pp-cli companies partial-update` — Partial update
- `servosity-pp-cli companies read` — View a company.
- `servosity-pp-cli companies summary` — List companies with account summaries.
- `servosity-pp-cli companies summary-ng` — Summary ng
- `servosity-pp-cli companies update` — Update a company.

**company-notes** — Manage company notes

- `servosity-pp-cli company-notes create` — Create
- `servosity-pp-cli company-notes delete` — Delete
- `servosity-pp-cli company-notes list` — List
- `servosity-pp-cli company-notes partial-update` — Partial update
- `servosity-pp-cli company-notes read` — Read
- `servosity-pp-cli company-notes update` — Update

**components** — Manage components

- `servosity-pp-cli components` — List

**contracts** — Manage contracts

- `servosity-pp-cli contracts create` — Create
- `servosity-pp-cli contracts get-by-token` — Get by token
- `servosity-pp-cli contracts list` — List
- `servosity-pp-cli contracts partial-update` — Partial update
- `servosity-pp-cli contracts read` — Read
- `servosity-pp-cli contracts signatures` — Signatures
- `servosity-pp-cli contracts update` — Update

**credentials** — Manage credentials

- `servosity-pp-cli credentials create` — Create
- `servosity-pp-cli credentials delete` — Delete
- `servosity-pp-cli credentials list` — List
- `servosity-pp-cli credentials partial-update` — Partial update
- `servosity-pp-cli credentials read` — Read
- `servosity-pp-cli credentials update` — Update

**current-user** — Manage current user

- `servosity-pp-cli current-user api-token-delete` — Delete the current user's API token. A new one will be generated when requested.
- `servosity-pp-cli current-user api-token-list` — You will receive JSON response with `token`. To make API calls with the token, add an `Authorization` header to your...
- `servosity-pp-cli current-user create` — Change the password of the current logged in user.
- `servosity-pp-cli current-user groups-list` — Groups list
- `servosity-pp-cli current-user helpjuice-sso-create` — Helpjuice sso create
- `servosity-pp-cli current-user hubspot-sso-create` — Hubspot sso create
- `servosity-pp-cli current-user list` — Get information about the current logged in user.
- `servosity-pp-cli current-user mfa-backup-codes-list` — Get unused backup codes. If no unused codes are left, remove all and generate new codes.
- `servosity-pp-cli current-user mfa-backup-codes-update` — Remove all backup codes and generate new codes.
- `servosity-pp-cli current-user notifications-delete` — Notifications delete
- `servosity-pp-cli current-user notifications-list` — Get current user notifications
- `servosity-pp-cli current-user profile-create` — Profile create
- `servosity-pp-cli current-user profile-list` — Profile list
- `servosity-pp-cli current-user start-mfa-create` — Start mfa create
- `servosity-pp-cli current-user start-mfa-list` — Start mfa list
- `servosity-pp-cli current-user start-mfa-verify-create` — Start mfa verify create
- `servosity-pp-cli current-user verified-mfa-delete` — Verified mfa delete
- `servosity-pp-cli current-user verified-mfa-list` — Verified mfa list
- `servosity-pp-cli current-user verified-mfa-send-code-create` — Verified mfa send code create

**download** — Manage download

- `servosity-pp-cli download` — Servosity one windows list

**dr-backups** — Manage dr backups

- `servosity-pp-cli dr-backups create` — Create a DR backup account.
- `servosity-pp-cli dr-backups delete` — Delete a DR backup account.
- `servosity-pp-cli dr-backups list` — List
- `servosity-pp-cli dr-backups partial-update` — Update a DR backup account.
- `servosity-pp-cli dr-backups read` — Read
- `servosity-pp-cli dr-backups update` — Update a DR backup account.

**issue-comments** — Manage issue comments

- `servosity-pp-cli issue-comments delete` — Delete
- `servosity-pp-cli issue-comments update` — Update

**issues** — Manage issues

- `servosity-pp-cli issues archived` — Archived
- `servosity-pp-cli issues ignored` — Ignored
- `servosity-pp-cli issues list` — List
- `servosity-pp-cli issues read` — Read

**postmark-webhook** — Manage postmark webhook

- `servosity-pp-cli postmark-webhook` — Create

**report-subscriptions** — Manage report subscriptions

- `servosity-pp-cli report-subscriptions read` — Read
- `servosity-pp-cli report-subscriptions unsubscribe` — Unsubscribe
- `servosity-pp-cli report-subscriptions verify` — Verify

**reports** — Manage reports

- `servosity-pp-cli reports account-list` — Get a report of backup account types for each company and reseller in CSV format.
- `servosity-pp-cli reports classic-usage-list` — Get a usage report for all backup accounts in CSV format.
- `servosity-pp-cli reports clients-list` — Get a report of backup account client versions.
- `servosity-pp-cli reports dr-from-email-list` — Get a report of user profiles.
- `servosity-pp-cli reports maxio-price-points-list` — Get CSV with all Maxio price points.
- `servosity-pp-cli reports product-list` — Product list
- `servosity-pp-cli reports stale-backup-sets-list` — Get a report of all backup set last backup complete times.
- `servosity-pp-cli reports usage-list` — Usage list
- `servosity-pp-cli reports user-profiles-list` — Get a report of user profiles.

**reseller-notes** — Manage reseller notes

- `servosity-pp-cli reseller-notes create` — Create
- `servosity-pp-cli reseller-notes delete` — Delete
- `servosity-pp-cli reseller-notes list` — List
- `servosity-pp-cli reseller-notes partial-update` — Partial update
- `servosity-pp-cli reseller-notes read` — Read
- `servosity-pp-cli reseller-notes update` — Update

**resellers** — Manage resellers

- `servosity-pp-cli resellers create` — Create a reseller.
- `servosity-pp-cli resellers delete` — Delete a reseller, also deleting all companies, backup accounts, and backup data.
- `servosity-pp-cli resellers list` — List resellers.
- `servosity-pp-cli resellers partial-update` — Partial update
- `servosity-pp-cli resellers read` — View a reseller.
- `servosity-pp-cli resellers summary` — List resellers with account summaries.
- `servosity-pp-cli resellers update` — Update a reseller.

**restic-backups** — Manage restic backups

- `servosity-pp-cli restic-backups create` — Create a restic backup account.
- `servosity-pp-cli restic-backups delete` — Delete a restic backup account.
- `servosity-pp-cli restic-backups list` — List
- `servosity-pp-cli restic-backups partial-update` — Update a restic backup account.
- `servosity-pp-cli restic-backups read` — Read
- `servosity-pp-cli restic-backups update` — Update a restic backup account.

**restore-queue-web-login** — Manage restore queue web login

- `servosity-pp-cli restore-queue-web-login list` — List
- `servosity-pp-cli restore-queue-web-login update` — Update

**screenshot** — Manage screenshot

- `servosity-pp-cli screenshot <key>` — Read

**sms-fm-mfa-callback** — Manage sms fm mfa callback

- `servosity-pp-cli sms-fm-mfa-callback` — Create

**sms-mfa-callback** — Manage sms mfa callback

- `servosity-pp-cli sms-mfa-callback` — Create

**stats** — Manage stats

- `servosity-pp-cli stats list` — List
- `servosity-pp-cli stats live-list` — Live list
- `servosity-pp-cli stats user-list` — User list

**users** — Manage users

- `servosity-pp-cli users create` — Create
- `servosity-pp-cli users delete` — Remove a user from a reseller or company group.
- `servosity-pp-cli users list` — List
- `servosity-pp-cli users request-password-recovery-create` — Request password recovery for a user.
- `servosity-pp-cli users reset-password-create` — Pass only `token` to confirm the token is valid. Pass `token` and `password` to set the user's password.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
servosity-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Morning fleet sweep

```bash
servosity-pp-cli attention --json --select companies.name,companies.score,companies.reasons
```

Daily Loop Architect ritual: rank everything that needs eyes-on across all admin endpoints in one call.

### Stale-backup follow-up list (Friday)

```bash
servosity-pp-cli stale-backups --refresh --days 7 --timeout 180s --json --select results.reseller,results.company,results.backup_set,results.last_complete_at,results.days_stale
```

Friday CSV-replacement: pull /reports/stale-backup-sets/ (slow on big fleets — bump timeout), snapshot it locally, filter for >7-day stale sets. Subsequent slices read from the local store with no API call.

### What changed since yesterday morning?

```bash
servosity-pp-cli drift --metric attention --from yesterday --to now --json
```

Diff today's attention snapshot against yesterday's — see who became attention-worthy and who recovered.

### Find anything mentioning a phrase

```bash
servosity-pp-cli find "image manager" --in issues,backups --json --select hits.resource,hits.id,hits.snippet
```

FTS5 hit across the synced store — find a phrase across issues and backups in one call instead of three list pages.

### Batch ignore noise issues

```bash
servosity-pp-cli triage --audience support --reseller 12 --ignore 18,22,29 --dry-run
```

Print the actions, then re-run without --dry-run to execute them in one call instead of nine UI clicks.

### Clear a partner until 6am tomorrow

```bash
servosity-pp-cli clear "BDH Technology" --until "6am tomorrow" --dry-run
```

Resolve the name as a reseller, list every active issue for every company under it, and print the ignore plan; drop --dry-run + add --confirm to execute.

### Morning stale-issue cleanup (yours)

```bash
servosity-pp-cli stale-issues --mine --cutoff "11pm yesterday" --auto-archive-known --dry-run
```

Daily Tier-One ritual: archive every known-safe stale issue from your FMDB, ignore non-dashboard noise, list the unknown ones for manual review.

## Auth Setup

Auth is a Servosity API token. Set SERVOSITY_API_TOKEN in your environment (or store it in macOS Keychain as a generic password named SERVOSITY_API_TOKEN and export it from your shell rc) and the CLI sends Authorization: Token <key> on every call — Servosity uses Django REST framework's Token scheme, not Bearer. **The Servosity API is single-tenant production: every mutating command (clear, triage, stale-issues, agent-restart subcommands) defaults to --dry-run; you must drop --dry-run AND pass --confirm to actually call the live API.**

Run `servosity-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  servosity-pp-cli agent-login list --agent --select id,name,status
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
servosity-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
servosity-pp-cli feedback --stdin < notes.txt
servosity-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.servosity-pp-cli/feedback.jsonl`. They are never POSTed unless `SERVOSITY_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SERVOSITY_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
servosity-pp-cli profile save briefing --json
servosity-pp-cli --profile briefing agent-login list
servosity-pp-cli profile list --json
servosity-pp-cli profile show briefing
servosity-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `servosity-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add servosity-pp-mcp -- servosity-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which servosity-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   servosity-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `servosity-pp-cli <command> --help`.
