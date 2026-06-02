---
name: pp-zapmail
description: "Every Zapmail dashboard action, plus an offline SQLite mirror of your whole mailbox and domain fleet, with deliverability rollups and renewal forecasting. Trigger phrases: `check my zapmail fleet health`, `which mailboxes are warmed but unassigned`, `export mailboxes to smartlead`, `what zapmail subscriptions renew next week`, `find failed zapmail mailboxes`, `use zapmail`, `run zapmail`."
author: "riccardovandra"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - zapmail-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/sales-and-crm/zapmail/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# Zapmail — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `zapmail-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install zapmail --cli-only
   ```
2. Verify: `zapmail-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Drive Zapmail from the command line with agent-native output (--json, --select, typed exit codes) and a --dry-run guard on everything that spends money. Sync your domains, mailboxes, subscriptions, and exports into local SQLite, then run fleet queries the dashboard cannot: fleet health rollups, warmed mailbox finders, failed-mailbox triage, renewal forecasts, and cost-per-active-mailbox. (v1 operates in your primary workspace.)

## When to Use This CLI

Use this CLI when you manage Zapmail cold-email infrastructure and need to query your fleet, script provisioning and exports, or embed Zapmail management in an agent or automation. It is strongest for cost and capacity reviews the dashboard can't answer. v1 operates in your primary workspace.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Fleet intelligence
- **`analytics --type fleet-health --group-by workspace`** — See every unhealthy or abused domain in your synced fleet in one table, instead of checking domains one at a time in the dashboard.

  _Reach for this first thing to triage deliverability across your whole fleet in one command._

  ```bash
  zapmail-pp-cli analytics --type fleet-health --group-by workspace --json
  ```
- **`mailboxes idle`** — List warmed mailboxes ready to send and report how many paid mailboxes are not yet provisioned, so you recover capacity you are paying for.

  _Use this to recover billable send capacity you forgot to put to work before a client notices low volume._

  ```bash
  zapmail-pp-cli mailboxes idle --json
  ```
- **`mailboxes failed`** — Surface every mailbox that failed or stalled during creation, grouped by domain and workspace, ready to retry.

  _Catch silently-failed provisioning before it shows up as missing send volume mid-campaign._

  ```bash
  zapmail-pp-cli mailboxes failed --json
  ```

### Cost and capacity
- **`analytics --type renewals --group-by week`** — Forecast which subscriptions and domains renew in the coming weeks and what each renewal will cost, so nothing lapses unexpectedly.

  _Use this on a Friday capacity review so a missed renewal never silently drops a sending domain._

  ```bash
  zapmail-pp-cli analytics --type renewals --group-by week --json
  ```
- **`analytics --type cost-efficiency --group-by workspace`** — Divide active subscription spend by the number of assigned mailboxes to expose where money is being wasted.

  _Reach for this to answer 'what are we actually paying per working mailbox' before a budget review._

  ```bash
  zapmail-pp-cli analytics --type cost-efficiency --group-by workspace --json
  ```
- **`analytics --type capacity --group-by workspace`** — Show purchased vs assigned vs available mailbox counts so you see unused capacity at a glance.

  _Use this to decide whether to assign more mailboxes or stop paying for capacity you aren't using._

  ```bash
  zapmail-pp-cli analytics --type capacity --group-by workspace --json
  ```

### Agent-native plumbing
- **`exports watch`** — Poll one export to completion and exit non-zero if it fails, so you can gate a script on a sequencer export finishing.

  _Use this in a script after starting an export so the next step only runs once the mailboxes actually land in the sequencer._

  ```bash
  zapmail-pp-cli exports watch --export-id 12345
  ```

## Command Reference

**dns** — DNS records on assigned domains

- `zapmail-pp-cli dns add` — Add one or more DNS records to an assigned domain
- `zapmail-pp-cli dns list` — Get all DNS records for an assigned domain

**domains** — Domains connected to or purchased through Zapmail

- `zapmail-pp-cli domains ai-finder` — Generate available domain name suggestions from keywords using AI
- `zapmail-pp-cli domains assignable` — List domains that have free capacity to assign new mailboxes
- `zapmail-pp-cli domains available-bulk` — Check registration availability for multiple domain names at once
- `zapmail-pp-cli domains health-score` — Retrieve the deliverability health score and abuse flag for one domain
- `zapmail-pp-cli domains list` — Retrieve all domains with status, DNS, forwarding, and assigned-mailbox counts

**exports** — Export mailboxes to third-party sequencers

- `zapmail-pp-cli exports accounts` — List connected third-party export accounts for an app
- `zapmail-pp-cli exports add-account` — Add third-party export account credentials
- `zapmail-pp-cli exports mailboxes` — Export mailboxes to one or more sequencer apps (Smartlead, Instantly, ReachInbox, etc.)
- `zapmail-pp-cli exports status` — Check the status of a running or completed export

**inbox** — Zapbox - read and send email from connected mailboxes

- `zapmail-pp-cli inbox accounts` — List inbox-connected accounts available in Zapbox
- `zapmail-pp-cli inbox emails` — Fetch recent emails for a connected account
- `zapmail-pp-cli inbox search` — Search emails across connected accounts
- `zapmail-pp-cli inbox send` — Send an email from a connected account (real outbound send)

**mailboxes** — Mailboxes provisioned on Zapmail domains

- `zapmail-pp-cli mailboxes get` — Get details of a single mailbox by its ID
- `zapmail-pp-cli mailboxes list` — Retrieve all mailboxes, grouped by domain, with counts and warmup status

**subscriptions** — Mailbox subscriptions and plans

- `zapmail-pp-cli subscriptions` — Get all subscriptions with plan, price, mailbox quantity, and billing period

**user** — Authenticated account details

- `zapmail-pp-cli user` — Retrieve the authenticated user: plan, mailbox counts, wallet balance

**wallet** — Account wallet balance and auto-recharge

- `zapmail-pp-cli wallet` — Get current wallet balance (USD) and auto-recharge settings

**workspaces** — Workspaces (isolated domain + mailbox containers)

- `zapmail-pp-cli workspaces create` — Create a new workspace, optionally with billing details
- `zapmail-pp-cli workspaces list` — Retrieve all workspaces for the authenticated account


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
zapmail-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Morning deliverability triage

```bash
zapmail-pp-cli sync && zapmail-pp-cli analytics --type fleet-health --group-by workspace --agent --select workspace,unhealthyCount,abusedCount
```

Sync the fleet, then narrow the rollup to just the at-risk counts for a fast scan.

### Recover wasted send capacity

```bash
zapmail-pp-cli mailboxes idle --json | jq '.[].email'
```

List warmed-but-unassigned mailboxes so you can put paid-for inboxes back to work.

### Clean export into a sequencer

```bash
zapmail-pp-cli exports mailboxes --apps SMARTLEAD --contains client.com --dry-run
```

Preview exactly which mailboxes would be exported before sending them, no real export performed.

### Forecast next week's renewals

```bash
zapmail-pp-cli analytics --type renewals --group-by week --csv
```

Get a week-bucketed renewal cost forecast as CSV for a spreadsheet or budget review.

### Send a one-off from a connected inbox

```bash
zapmail-pp-cli inbox send --account you@example.com --to prospect@example.com --subject 'Quick q' --body 'Hi there' --dry-run
```

Compose and preview an outbound send from a Zapbox-connected account before actually sending.

## Auth Setup

Zapmail uses a single API key sent in the x-auth-zapmail header (raw token, not Bearer). Get it at Dashboard > Settings > Integrations > API and set ZAPMAIL_API_KEY.

Run `zapmail-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  zapmail-pp-cli dns list --id 550e8400-e29b-41d4-a716-446655440000 --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

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
zapmail-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
zapmail-pp-cli feedback --stdin < notes.txt
zapmail-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/zapmail-pp-cli/feedback.jsonl`. They are never POSTed unless `ZAPMAIL_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ZAPMAIL_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
zapmail-pp-cli profile save briefing --json
zapmail-pp-cli --profile briefing dns list --id 550e8400-e29b-41d4-a716-446655440000
zapmail-pp-cli profile list --json
zapmail-pp-cli profile show briefing
zapmail-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `zapmail-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add zapmail-pp-mcp -- zapmail-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which zapmail-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   zapmail-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `zapmail-pp-cli <command> --help`.
