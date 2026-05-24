---
name: pp-smartlead
description: "Every SmartLead API feature, plus a local mirror that answers campaign-health, silent-lead, and cross-campaign... Trigger phrases: `check my smartlead campaigns`, `which leads went silent`, `is this domain already pitched`, `rank my sender accounts`, `use smartlead`, `run smartlead`."
author: "bossriceshark"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - smartlead-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/sales-and-crm/smartlead/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# SmartLead — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `smartlead-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install smartlead --cli-only
   ```
2. Verify: `smartlead-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/smartlead/cmd/smartlead-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use this CLI when an agent or operator needs to audit cold-email outreach in SmartLead: checking which campaigns are healthy, building follow-up lists of silent leads, deduplicating leads across campaigns before launch, or ranking sender accounts by deliverability. It is the right tool when the question spans multiple campaigns or needs week-over-week history, because those answers come from the local mirror rather than a single API call.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`health`** — One-shot scorecard for every campaign — bounce rate, reply rate, silent-lead count, sender count, and a stale flag — without clicking through the dashboard.

  _Reach for this first when an agent needs to know which campaigns are healthy before drilling into any one of them._

  ```bash
  smartlead-pp-cli health --json
  ```
- **`silent`** — Finds leads that were emailed but have not replied within N days — the exact set to follow up with or retire.

  _Use this to build a follow-up list instead of paging the whole lead set and diffing timestamps by hand._

  ```bash
  smartlead-pp-cli silent --campaign 12345 --days 7 --json
  ```
- **`dupes`** — Scans the whole lead mirror for emails or domains that appear in two or more campaigns; --domain prints the full pitch ledger for one site.

  _Run this before adding leads to a campaign to avoid double-contacting a prospect already in flight._

  ```bash
  smartlead-pp-cli dupes --domain example.com --json
  ```
- **`drift`** — Computes week-over-week reply, open, and bounce deltas for a campaign by querying analytics one seven-day window at a time.

  _Use this to catch a campaign decaying over time rather than judging it from a single point-in-time stat._

  ```bash
  smartlead-pp-cli drift --campaign 12345 --weeks 4 --json
  ```

### Deliverability intelligence
- **`sender-health`** — Ranks every email sender account by a composite of inbox-warmup landing rate, SMTP/IMAP connection health, and sending utilization.

  _Reach for this to find which sender accounts are dragging deliverability before they tank a campaign._

  ```bash
  smartlead-pp-cli sender-health --json
  ```
- **`warmup-gate`** — Checks each sender account against warmup thresholds (--min-days, --min-inbox-rate); with --strict it exits non-zero when any account fails — a scriptable launch gate.

  _Call this in a launch script to block attaching a sender account that is not warmed up yet._

  ```bash
  smartlead-pp-cli warmup-gate --account 6789 --json
  ```

## Command Reference

**campaigns** — Manage campaigns

- `smartlead-pp-cli campaigns create` — Create a new campaign
- `smartlead-pp-cli campaigns delete` — Delete a campaign
- `smartlead-pp-cli campaigns get` — Get a campaign by ID
- `smartlead-pp-cli campaigns list` — Retrieves every email campaign for the authenticated SmartLead account.

**client** — Manage client

- `smartlead-pp-cli client create` — Create a whitelabel client
- `smartlead-pp-cli client list` — List all whitelabel clients

**email-accounts** — Manage email accounts

- `smartlead-pp-cli email-accounts create` — Add a new email sender account
- `smartlead-pp-cli email-accounts list` — List all email sender accounts
- `smartlead-pp-cli email-accounts reconnect-failed` — Trigger a reconnect of all disconnected email accounts
- `smartlead-pp-cli email-accounts update` — Update an email sender account

**leads** — Manage leads

- `smartlead-pp-cli leads add-domain-block-list` — Add domains to the account-wide block list
- `smartlead-pp-cli leads get-by-email` — Look up a lead by email address
- `smartlead-pp-cli leads list-categories` — List all lead categories for the account
- `smartlead-pp-cli leads update` — Update a lead's fields


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
smartlead-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Morning campaign triage

```bash
smartlead-pp-cli sync && smartlead-pp-cli health --json
```

Refresh the mirror, then get the all-campaigns health scorecard in one structured payload.

### Pre-launch dedupe check

```bash
smartlead-pp-cli dupes --domain prospect-site.com --json
```

See every campaign, category, and last-contact date for a domain before pitching it again.

### Build a follow-up list

```bash
smartlead-pp-cli silent --campaign 12345 --days 10 --json
```

Pull leads emailed but silent for 10+ days to feed into a follow-up sequence.

### Trim a verbose campaign payload

```bash
smartlead-pp-cli campaigns list --agent --select id,name,status
```

SmartLead campaign objects are large; --agent with --select narrows the response to just the fields an agent needs.

### Gate a launch on sender warmup

```bash
smartlead-pp-cli warmup-gate --account 6789 --json
```

Returns a typed exit code so a launch script can block on a sender that is not warmed up.

## Auth Setup

SmartLead authenticates with an API key passed as the api_key query parameter. Set SMARTLEAD_API_KEY in your environment (find the key under Settings -> API in the SmartLead app). No OAuth, no login flow.

Run `smartlead-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  smartlead-pp-cli campaigns list --agent --select id,name,status
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
smartlead-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
smartlead-pp-cli feedback --stdin < notes.txt
smartlead-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.smartlead-pp-cli/feedback.jsonl`. They are never POSTed unless `SMARTLEAD_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SMARTLEAD_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
smartlead-pp-cli profile save briefing --json
smartlead-pp-cli --profile briefing campaigns list
smartlead-pp-cli profile list --json
smartlead-pp-cli profile show briefing
smartlead-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `smartlead-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add smartlead-pp-mcp -- smartlead-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which smartlead-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   smartlead-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `smartlead-pp-cli <command> --help`.
