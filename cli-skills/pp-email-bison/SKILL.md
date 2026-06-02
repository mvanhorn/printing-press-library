---
name: pp-email-bison
description: "Every Email Bison campaign, lead, and reply on the command line Trigger phrases: `list my email bison campaigns`, `which campaigns are below their daily cap`, `triage my email bison inbox`, `show interested replies since yesterday`, `which senders are disconnected`, `use email-bison`, `run email-bison`."
author: "riccardovandra"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - email-bison-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/sales-and-crm/email-bison/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# Email Bison — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `email-bison-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install email-bison --cli-only
   ```
2. Verify: `email-bison-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/email-bison/cmd/email-bison-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

A Go CLI for the Email Bison cold-email API with a local SQLite store, cursor-based sync, and offline FTS. Beyond mirroring every endpoint, it answers questions a single API call cannot: which campaigns are under their daily cap (campaigns headroom), which senders are degraded but still live (sender-emails health), which leads are stuck mid-sequence (leads stale), and which A/B variant is winning (campaigns variants).

## When to Use This CLI

Use this CLI when an agent or operator needs to run or audit Email Bison outreach programmatically: launching and pausing campaigns, ingesting and tagging leads, triaging the master inbox, and answering cross-entity questions (cap headroom, sender health, stale leads, variant win rates) that the web UI and raw API cannot answer in one shot. It shines for agencies running many workspaces who want scriptable, offline-queryable campaign state.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local joins that compound
- **`campaigns headroom`** — See which launched campaigns are sending below their daily cap, at cap, or idle, in one table.

  _Reach for this when an agent needs to find under-delivering campaigns across an account without opening every campaign in the UI._

  ```bash
  email-bison-pp-cli campaigns headroom --agent
  ```
- **`sender-emails health`** — One board joining sender state, attached campaigns, and recent bounces to spot dead or over-assigned inboxes.

  _Use before a deliverability audit to find senders that are degraded but still actively sending._

  ```bash
  email-bison-pp-cli sender-emails health --agent
  ```
- **`leads stale`** — Find leads stuck mid-sequence: in a live campaign, already emailed, no reply, and no scheduled next send.

  _Use weekly to surface leads silently rotting in a sequence so they can be re-engaged or cleaned out._

  ```bash
  email-bison-pp-cli leads stale --days 7 --agent
  ```

### Reply triage
- **`replies interested`** — Every reply marked interested across all campaigns since a timestamp, joined to its lead, campaign, and sender.

  _Reach for this for a daily 'what got interested since yesterday' triage across the whole account._

  ```bash
  email-bison-pp-cli replies interested --since 24h --agent
  ```
- **`replies triage`** — An oldest-first worklist of pending replies with lead and campaign context, ready to pipe into mark-interested or follow-up push.

  _Use as the entry point for a morning master-inbox triage loop an agent can work top to bottom._

  ```bash
  email-bison-pp-cli replies triage --agent
  ```

### Campaign optimization
- **`campaigns variants`** — Per A/B sequence-step variant, the reply rate and interested rate computed from local data.

  _Reach for this when deciding which subject/body variant to keep in a multi-step campaign._

  ```bash
  email-bison-pp-cli campaigns variants 6 --agent
  ```
- **`campaigns preflight`** — Before resuming, check locally that a campaign has a schedule, at least one step, a sender, leads, and that every merge tag exists as a custom variable.

  _Run this before launch to catch broken {VARIABLE} renders and missing senders that would silently kill a send._

  ```bash
  email-bison-pp-cli campaigns preflight 6 --agent
  ```

## Command Reference

**campaigns** — Manage campaigns

- `email-bison-pp-cli campaigns create` — Create a campaign. Returns the new campaign ID.
- `email-bison-pp-cli campaigns delete-sequence-step` — Delete a sequence step.
- `email-bison-pp-cli campaigns get` — Get a campaign and its settings.
- `email-bison-pp-cli campaigns list` — List campaigns in the workspace.
- `email-bison-pp-cli campaigns list-schedule-templates` — List saved schedule templates.
- `email-bison-pp-cli campaigns send-test-sequence-step` — Send a test email for a sequence step.

**custom-variables** — Manage custom variables

- `email-bison-pp-cli custom-variables create` — Create a custom variable.
- `email-bison-pp-cli custom-variables list` — List custom variables in the workspace.

**leads** — Manage leads

- `email-bison-pp-cli leads create` — Create a single lead.
- `email-bison-pp-cli leads list` — List leads (contacts) in the workspace.
- `email-bison-pp-cli leads update` — Update a lead and its custom variables.

**replies** — Manage replies

- `email-bison-pp-cli replies` — List replies in the master inbox.

**scheduled-emails** — Manage scheduled emails

- `email-bison-pp-cli scheduled-emails <lead_id_or_email>` — Get scheduled emails for a lead.

**sender-emails** — Manage sender emails

- `email-bison-pp-cli sender-emails list` — List connected sender email accounts.
- `email-bison-pp-cli sender-emails update` — Update a sender email account's settings.

**tags** — Manage tags

- `email-bison-pp-cli tags attach-to-campaigns` — Attach tags to campaigns.
- `email-bison-pp-cli tags attach-to-leads` — Attach tags to leads.
- `email-bison-pp-cli tags attach-to-sender-emails` — Attach tags to sender emails.
- `email-bison-pp-cli tags create` — Create a custom tag.
- `email-bison-pp-cli tags delete` — Delete a custom tag.
- `email-bison-pp-cli tags list` — List custom tags.
- `email-bison-pp-cli tags remove-from-campaigns` — Remove tags from campaigns.
- `email-bison-pp-cli tags remove-from-leads` — Remove tags from leads.
- `email-bison-pp-cli tags remove-from-sender-emails` — Remove tags from sender emails.

**users** — Manage users

- `email-bison-pp-cli users` — List users in the current workspace (also validates the token).

**webhook-events** — Manage webhook events

- `email-bison-pp-cli webhook-events` — Send a test webhook event.

**webhooks** — Manage webhooks

- `email-bison-pp-cli webhooks` — Create a webhook subscription.

**workspaces** — Manage workspaces

- `email-bison-pp-cli workspaces create-token` — Create an api-user token for a workspace (requires a super-admin token).
- `email-bison-pp-cli workspaces list` — List workspaces (requires a super-admin token).


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
email-bison-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Morning inbox triage

```bash
email-bison-pp-cli replies triage --agent
```

Pull an oldest-first queue of pending replies, then mark-interested or push the good ones into a follow-up campaign.

### Pre-launch safety check

```bash
email-bison-pp-cli campaigns preflight 6 --agent
```

Confirm a campaign has a schedule, sequence, senders, leads, and valid merge tags before calling resume.

### Deliverability sweep

```bash
email-bison-pp-cli sender-emails health --agent --select email,state,live_campaigns,recent_bounces
```

Surface degraded senders still attached to live campaigns, narrowing a verbose response to the fields that matter.

### Find rotting leads

```bash
email-bison-pp-cli leads stale --days 7 --agent
```

List leads emailed a week ago with no reply and no next send so they can be re-engaged or removed.

### Launch a campaign

```bash
email-bison-pp-cli campaigns resume campaign 6 --dry-run
```

Preview the launch call, then drop --dry-run to start sending once preflight is clean.

## Auth Setup

Email Bison is self-hosted: each workspace has its own dedicated instance domain. Set EMAIL_BISON_BASE_URL to your instance (default https://dedi.emailbison.com) and EMAIL_BISON_API_KEY to a workspace api-user token from Settings -> Developer API. Every token is scoped to one workspace.

Run `email-bison-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  email-bison-pp-cli campaigns list --agent --select id,name,status
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
email-bison-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
email-bison-pp-cli feedback --stdin < notes.txt
email-bison-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/email-bison-pp-cli/feedback.jsonl`. They are never POSTed unless `EMAIL_BISON_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `EMAIL_BISON_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
email-bison-pp-cli profile save briefing --json
email-bison-pp-cli --profile briefing campaigns list
email-bison-pp-cli profile list --json
email-bison-pp-cli profile show briefing
email-bison-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `email-bison-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add email-bison-pp-mcp -- email-bison-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which email-bison-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   email-bison-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `email-bison-pp-cli <command> --help`.
