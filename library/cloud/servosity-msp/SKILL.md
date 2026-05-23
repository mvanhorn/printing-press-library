---
name: pp-servosity-msp
description: "Printing Press CLI for Servosity Msp. Servosity REST API surface available to authenticated MSP partners."
author: "Damien Stevens"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - servosity-msp-cli
---

# Servosity Msp — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `servosity-msp-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install servosity-msp --cli-only
   ```
2. Verify: `servosity-msp-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Servosity REST API surface available to authenticated MSP partners. All operations are scoped to the authenticated reseller. Admin-only endpoints (cross-reseller listing, billing back-office, support tooling) are not included.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Fleet-wide intelligence
- **`attention`** — One screen across your whole book of clients. Merges open issues, stale backups, and in-flight DR events into a per-company ranked view, then persists the result so tomorrow's drift command can compare.

  _Reach for this in the morning to triage what needs follow-up across every client without clicking through a portal._

  ```bash
  servosity-msp-pp-cli attention --top 10 --json
  ```
- **`drift`** — Diff two snapshots the CLI collected — show which companies got worse, which recovered, and which are new since a past anchor. Default compares yesterday-to-now on the attention metric.

  _Use Monday morning to start with situation awareness instead of treating every week as a fresh slate._

  ```bash
  servosity-msp-pp-cli drift --metric attention --from yesterday --to now --json
  ```
- **`stale-backups`** — Slice the stale-backup-sets report by company, age window, and backup engine — entirely offline once cached. Use --refresh to repull from the API.

  _Run this Friday afternoon to compile the list of clients you need to email about a stalled backup._

  ```bash
  servosity-msp-pp-cli stale-backups --days 7 --engine restic --json
  ```
- **`backup-facts`** — Unified view across Servosity's three backup engines (classic, restic, DR) for one company or all. Engine, ID, hostname, last_successful_at, status, size — joined from three local store tables into one table.

  _Reach for this when triaging a client who has multiple engines protecting different devices and you need to know which engine is failing where._

  ```bash
  servosity-msp-pp-cli backup-facts --company 4421 --status fail --json
  ```

### Client-facing reporting
- **`qbr`** — Generate the backup section of a client's Quarterly Business Review as Markdown, HTML, or PDF. Job success rate, restore tests run this quarter, coverage map across all three engines, open issues, storage trend.

  _Use this 1-2 weeks before a client QBR. Saves 30-60 min of manual deck-building per client._

  ```bash
  servosity-msp-pp-cli qbr 4421 --quarter 2026-Q1 --format pdf --out acme-q1.pdf
  ```

### Daily ops efficiency
- **`triage`** — List open issues with filters, then batch-mutate them (ignore / archive / reactivate / comment) in one invocation with --dry-run support and typed exit codes.

  _Use when the issue queue is bursty or during a planned-outage window where many alerts cluster around one client._

  ```bash
  servosity-msp-pp-cli triage --company 4421 --ignore 18,22,29 --comment 'scheduled outage' --dry-run
  ```

### Disaster recovery
- **`restore-queue watch`** — Watch every active company's restore queue across the book during a DR event. Polls each company periodically and prints diffs since the last tick.

  _Use during an active disaster recovery event when multiple clients have restores in flight._

  ```bash
  servosity-msp-pp-cli restore-queue watch --interval 30s --json
  ```

### Business operations
- **`bill --reconcile`** — Pull the MSP's monthly Servosity bill and compare line-by-line against a CSV of what the MSP is invoicing their clients. Surfaces drift — clients under- or over-charged.

  _Run this every month-end before invoicing clients. Catches missed line items and pricing mismatches._

  ```bash
  servosity-msp-pp-cli bill --reconcile invoiced-2026-05.csv --month 2026-05 --json
  ```
- **`unprovisioned`** — List agents installed on client machines but not yet pulling backups, ranked by client. Surfaces lost revenue from incomplete onboardings.

  _Run weekly to catch agents installed during onboarding that never successfully phoned home._

  ```bash
  servosity-msp-pp-cli unprovisioned --age 24h --json
  ```
- **`storage-trend`** — Linear-regression forecast of when a specific client will hit a capacity threshold. Reads the historical storage_bytes time series from local snapshots; with --snapshot, persists a new measurement for future runs.

  _Run quarterly per high-storage client to identify upsell opportunities before they hit a hard limit._

  ```bash
  servosity-msp-pp-cli storage-trend 4421 --weeks 12 --threshold 1TB --json
  ```

## Command Reference

**agent-login** — Manage agent login

- `servosity-msp-cli agent-login create` — Create
- `servosity-msp-cli agent-login list` — List

**agent-sessions** — Manage agent sessions

- `servosity-msp-cli agent-sessions <agent_session_id>` — Read

**backup-job-report** — Manage backup job report

- `servosity-msp-cli backup-job-report <backup_destination_id> <backup_id> <backup_job_id> <backup_set_id>` — View detailed backup report for a backup job and destination.

**backup-job-report-summary** — Manage backup job report summary

- `servosity-msp-cli backup-job-report-summary <backup_destination_id> <backup_id> <backup_job_id> <backup_set_id>` — View summary backup report for a backup job and destination.

**backup-job-status** — Manage backup job status

- `servosity-msp-cli backup-job-status <backup_id>` — List backup job status for a backup account on a specific date.

**backup-jobs** — Manage backup jobs

- `servosity-msp-cli backup-jobs <backup_id>` — List backup jobs for a backup account.

**backup-plans** — Manage backup plans

- `servosity-msp-cli backup-plans list` — List backup plans.
- `servosity-msp-cli backup-plans read` — View a backup plan.

**backup-search** — Manage backup search

- `servosity-msp-cli backup-search` — List

**backup-sets** — Manage backup sets

- `servosity-msp-cli backup-sets create` — Create a backup-set for a backup account.
- `servosity-msp-cli backup-sets delete` — Delete a backup-set for a backup account.
- `servosity-msp-cli backup-sets list` — List backup-sets for a backup account.
- `servosity-msp-cli backup-sets read` — View a backup-set for a backup account.
- `servosity-msp-cli backup-sets update` — Accepts a json body with the following optional parameters.

**backups** — Manage backups

- `servosity-msp-cli backups create` — Create a backup account.
- `servosity-msp-cli backups delete` — Delete a backup account, also deleting all backup data.
- `servosity-msp-cli backups list` — List backup accounts.
- `servosity-msp-cli backups mfa-codes` — Mfa codes
- `servosity-msp-cli backups partial-update` — Partial update
- `servosity-msp-cli backups read` — View a backup account.
- `servosity-msp-cli backups update` — Update a backup account.

**companies** — Manage companies

- `servosity-msp-cli companies create` — Create a company.
- `servosity-msp-cli companies delete` — Delete a company, also deleting all backup accounts and backup data.
- `servosity-msp-cli companies fully-managed` — List fully-managed companies.
- `servosity-msp-cli companies fully-managed-ng` — List fully-managed companies.
- `servosity-msp-cli companies list` — List companies.
- `servosity-msp-cli companies partial-update` — Partial update
- `servosity-msp-cli companies read` — View a company.
- `servosity-msp-cli companies summary` — List companies with account summaries.
- `servosity-msp-cli companies summary-ng` — Summary ng
- `servosity-msp-cli companies update` — Update a company.

**company-notes** — Manage company notes

- `servosity-msp-cli company-notes create` — Create
- `servosity-msp-cli company-notes delete` — Delete
- `servosity-msp-cli company-notes list` — List
- `servosity-msp-cli company-notes partial-update` — Partial update
- `servosity-msp-cli company-notes read` — Read
- `servosity-msp-cli company-notes update` — Update

**components** — Manage components

- `servosity-msp-cli components` — List

**contracts** — Manage contracts

- `servosity-msp-cli contracts create` — Create
- `servosity-msp-cli contracts get-by-token` — Get by token
- `servosity-msp-cli contracts list` — List
- `servosity-msp-cli contracts partial-update` — Partial update
- `servosity-msp-cli contracts read` — Read
- `servosity-msp-cli contracts signatures` — Signatures
- `servosity-msp-cli contracts update` — Update

**credentials** — Manage credentials

- `servosity-msp-cli credentials create` — Create
- `servosity-msp-cli credentials delete` — Delete
- `servosity-msp-cli credentials list` — List
- `servosity-msp-cli credentials partial-update` — Partial update
- `servosity-msp-cli credentials read` — Read
- `servosity-msp-cli credentials update` — Update

**current-user** — Manage current user

- `servosity-msp-cli current-user api-token-delete` — Delete the current user's API token. A new one will be generated when requested.
- `servosity-msp-cli current-user api-token-list` — You will receive JSON response with `token`.
- `servosity-msp-cli current-user create` — Change the password of the current logged in user.
- `servosity-msp-cli current-user groups-list` — Groups list
- `servosity-msp-cli current-user helpjuice-sso-create` — Helpjuice sso create
- `servosity-msp-cli current-user hubspot-sso-create` — Hubspot sso create
- `servosity-msp-cli current-user list` — Get information about the current logged in user.
- `servosity-msp-cli current-user mfa-backup-codes-list` — Get unused backup codes. If no unused codes are left, remove all and generate new codes.
- `servosity-msp-cli current-user mfa-backup-codes-update` — Remove all backup codes and generate new codes.
- `servosity-msp-cli current-user notifications-delete` — Notifications delete
- `servosity-msp-cli current-user notifications-list` — Get current user notifications
- `servosity-msp-cli current-user profile-create` — Profile create
- `servosity-msp-cli current-user profile-list` — Profile list
- `servosity-msp-cli current-user start-mfa-create` — Start mfa create
- `servosity-msp-cli current-user start-mfa-list` — Start mfa list
- `servosity-msp-cli current-user start-mfa-verify-create` — Start mfa verify create
- `servosity-msp-cli current-user verified-mfa-delete` — Verified mfa delete
- `servosity-msp-cli current-user verified-mfa-list` — Verified mfa list
- `servosity-msp-cli current-user verified-mfa-send-code-create` — Verified mfa send code create

**download** — Manage download

- `servosity-msp-cli download` — Servosity one windows list

**dr-backups** — Manage dr backups

- `servosity-msp-cli dr-backups create` — Create a DR backup account.
- `servosity-msp-cli dr-backups delete` — Delete a DR backup account.
- `servosity-msp-cli dr-backups list` — List
- `servosity-msp-cli dr-backups partial-update` — Update a DR backup account.
- `servosity-msp-cli dr-backups read` — Read
- `servosity-msp-cli dr-backups update` — Update a DR backup account.

**issue-comments** — Manage issue comments

- `servosity-msp-cli issue-comments delete` — Delete
- `servosity-msp-cli issue-comments update` — Update

**issues** — Manage issues

- `servosity-msp-cli issues archived` — Archived
- `servosity-msp-cli issues ignored` — Ignored
- `servosity-msp-cli issues list` — List
- `servosity-msp-cli issues read` — Read

**report-subscriptions** — Manage report subscriptions

- `servosity-msp-cli report-subscriptions read` — Read
- `servosity-msp-cli report-subscriptions unsubscribe` — Unsubscribe
- `servosity-msp-cli report-subscriptions verify` — Verify

**reports** — Manage reports

- `servosity-msp-cli reports account-list` — Get a report of backup account types for each company and reseller in CSV format.
- `servosity-msp-cli reports classic-usage-list` — Get a usage report for all backup accounts in CSV format.
- `servosity-msp-cli reports clients-list` — Get a report of backup account client versions.
- `servosity-msp-cli reports dr-from-email-list` — Get a report of user profiles.
- `servosity-msp-cli reports maxio-price-points-list` — Get CSV with all Maxio price points.
- `servosity-msp-cli reports product-list` — Product list
- `servosity-msp-cli reports stale-backup-sets-list` — Get a report of all backup set last backup complete times.
- `servosity-msp-cli reports usage-list` — Usage list
- `servosity-msp-cli reports user-profiles-list` — Get a report of user profiles.

**resellers** — Manage resellers

- `servosity-msp-cli resellers partial-update` — Partial update
- `servosity-msp-cli resellers read` — View a reseller.
- `servosity-msp-cli resellers update` — Update a reseller.

**restic-backups** — Manage restic backups

- `servosity-msp-cli restic-backups create` — Create a restic backup account.
- `servosity-msp-cli restic-backups delete` — Delete a restic backup account.
- `servosity-msp-cli restic-backups list` — List
- `servosity-msp-cli restic-backups partial-update` — Update a restic backup account.
- `servosity-msp-cli restic-backups read` — Read
- `servosity-msp-cli restic-backups update` — Update a restic backup account.

**screenshot** — Manage screenshot

- `servosity-msp-cli screenshot <key>` — Read

**stats** — Manage stats

- `servosity-msp-cli stats list` — List
- `servosity-msp-cli stats live-list` — Live list
- `servosity-msp-cli stats user-list` — User list

**users** — Manage users

- `servosity-msp-cli users create` — Create
- `servosity-msp-cli users delete` — Remove a user from a reseller or company group.
- `servosity-msp-cli users list` — List
- `servosity-msp-cli users request-password-recovery-create` — Request password recovery for a user.
- `servosity-msp-cli users reset-password-create` — Pass only `token` to confirm the token is valid. Pass `token` and `password` to set the user's password.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
servosity-msp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup
Run `servosity-msp-cli auth setup` to print the URL and steps for getting a key (add `--launch` to open the URL). Then set:

```bash
export SERVOSITY_MSP_TOKEN="<your-key>"
```

Or persist it in `~/.config/servosity-partner-msp-pp-cli/config.toml`.

Run `servosity-msp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  servosity-msp-cli agent-login list --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
servosity-msp-cli feedback "the --since flag is inclusive but docs say exclusive"
servosity-msp-cli feedback --stdin < notes.txt
servosity-msp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.servosity-msp-cli/feedback.jsonl`. They are never POSTed unless `SERVOSITY_MSP_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SERVOSITY_MSP_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
servosity-msp-cli profile save briefing --json
servosity-msp-cli --profile briefing agent-login list
servosity-msp-cli profile list --json
servosity-msp-cli profile show briefing
servosity-msp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `servosity-msp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add servosity-msp-mcp -- servosity-msp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which servosity-msp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   servosity-msp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `servosity-msp-cli <command> --help`.
