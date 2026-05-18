---
name: pp-emailoctopus
description: "Every EmailOctopus v2 endpoint, plus the cross-list joins, churn diffs, and rate-budgeted bulk operations the API... Trigger phrases: `manage my EmailOctopus list`, `sync subscribers to EmailOctopus`, `EmailOctopus campaign report`, `find cold subscribers`, `dedupe my email lists`, `use emailoctopus`, `run emailoctopus`."
author: "Trevin Chow"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - emailoctopus-pp-cli
---

# EmailOctopus — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `emailoctopus-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install emailoctopus --cli-only
   ```
2. Verify: `emailoctopus-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

EmailOctopus is the only major email-marketing platform whose free tier includes API access — but until now no maintained v2 CLI or SDK existed. This CLI ships all 25 endpoints with --json, --select, --csv, --dry-run, plus a local SQLite store that powers cross-list dedupe, per-contact engagement scoring, list churn diffs over time, and rate-budgeted bulk delete.

## When to Use This CLI

Use this CLI when an agent or operator needs the full EmailOctopus v2 surface — lists, contacts, tags, custom fields, campaign reports, and automation triggers — plus the local-join queries the hosted API can't answer (cross-list duplicates, per-contact engagement scoring, list churn since the last snapshot, tag set-algebra). Especially valuable for indie newsletter operators, small-SaaS growth engineers, and agencies running many client lists, since it works offline once synced and paces all mutations under the documented 10 req/sec rate limit.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local joins the API can't do
- **`contacts engagement`** — Score every contact by opens, clicks, and inactive-since across all campaigns. The API has no engagement-history endpoint; we synthesize it locally.

  _Pick this when an agent needs to find cold subscribers, build a reactivation cohort, or score engagement without paginating every campaign report by hand._

  ```bash
  emailoctopus-pp-cli contacts engagement --list <list_id> --inactive-since 90d --json
  ```
- **`contacts dedupe`** — Find contacts that appear on multiple lists in the account, optionally consolidating them onto one canonical list.

  _Pick this when an agent needs to clean up subscriber duplication, audit list sprawl, or plan a merge across multiple lists._

  ```bash
  emailoctopus-pp-cli contacts dedupe --json
  ```
- **`tags intersect`** — Find contacts matching boolean combinations of tags: --has trial-started --not activated returns the trial cohort that hasn't converted.

  _Pick this when an agent needs to segment subscribers by tag combinations for targeted outreach, churn reactivation, or audience reporting._

  ```bash
  emailoctopus-pp-cli tags intersect --list <list_id> --has trial-started --not activated --json
  ```

### Workflow accelerators
- **`campaigns digest`** — One-shot campaign report combining summary, top-N links, contact-level breakdown, and per-domain opens — rendered for terminal or Markdown paste.

  _Pick this when an agent needs to summarize a campaign's results for a stakeholder doc without screen-scraping the EmailOctopus dashboard._

  ```bash
  emailoctopus-pp-cli campaigns digest <campaign_id> --md
  ```
- **`contacts sync-csv`** — Push a CSV into EmailOctopus with mapped fields and tags. Dry-runs the diff against the local store first, then chunks into batch-upsert calls paced under the rate limit.

  _Pick this when an agent needs to atomically sync a CSV of contacts with tag/field mapping and pre-flight the change before applying it._

  ```bash
  emailoctopus-pp-cli contacts sync-csv ./subscribers.csv --list <list_id> --map email=Email,tag.plan=Plan --dry-run
  ```

### Local snapshots over time
- **`lists diff`** — Show contacts touched in this list since a relative time. Surfaces the change-set the API can't return — useful for incremental syncs, audit logs, or alerting on recent activity.

  _Pick this when an agent needs to see which contacts were touched in the last hour/day/week or run an incremental change-detection workflow._

  ```bash
  emailoctopus-pp-cli lists diff <list_id> --since yesterday --json
  ```

### Mutation safety
- **`contacts bulk-delete`** — Delete many contacts matching a local predicate, paced under the 10/sec API limit with a resumable progress file.

  _Pick this when an agent needs to clean up unsubscribed, bounced, or stale contacts in bulk without hitting 429s or losing progress mid-run._

  ```bash
  emailoctopus-pp-cli contacts bulk-delete --list <list_id> --where 'status=unsubscribed' --rate 8 --dry-run
  ```
- **`automations trigger-batch`** — Queue an automation for many contacts from stdin or CSV, paced under the rate limit with retry on 429.

  _Pick this when an agent needs to start an automation for a batch of contacts (trial-ending cohort, plan-upgrade celebration) without writing loop+backoff boilerplate._

  ```bash
  cat trial-ending.csv | emailoctopus-pp-cli automations trigger-batch <automation_id> --stdin
  ```

## Command Reference

**automations** — An automation is a sequence of automated steps triggered by an event, such as when a contact subscribes to a list or is tagged.
Automations allow you to automatically send emails, update fields, apply tags and more.


**campaigns** — A campaign is generally used to send a one-off, timely email to some or all of your subscribers. For example you may use a campaign to send the latest edition of your weekly newsletter, or to announce a new feature in your product.

- `emailoctopus-pp-cli campaigns get` — Get all campaigns
- `emailoctopus-pp-cli campaigns id-get` — Get campaign

**lists** — A list is a collection of contacts. Every one of your contacts will exist inside a list. The majority of our users only require one list, but multiple lists can be created and configured with different fields and tags in order to organise distinct groups of contacts.

- `emailoctopus-pp-cli lists get` — Get all lists
- `emailoctopus-pp-cli lists id-delete` — Delete a list
- `emailoctopus-pp-cli lists id-get` — Get list
- `emailoctopus-pp-cli lists id-put` — Update list
- `emailoctopus-pp-cli lists post` — Create list


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
emailoctopus-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Find cold subscribers across a list

```bash
emailoctopus-pp-cli contacts engagement --list <list_id> --inactive-since 90d --json --select email_address,last_engaged_at,opens,clicks
```

Joins synced contacts with all campaign contact-reports to surface anyone who hasn't opened or clicked in 90 days. The --select narrows the response to just the fields an agent needs, dropping the ~10KB of nested per-campaign detail.

### Generate a paste-ready campaign report

```bash
emailoctopus-pp-cli campaigns digest 071f24b2-51cd-11f1-a3ce-11fd783017da --md
```

One command renders summary metrics, top-clicked links, per-domain opens, and unsubscribe breakdown as Markdown — paste straight into Notion or a stakeholder doc.

### Find contacts on more than one list

```bash
emailoctopus-pp-cli contacts dedupe --json
```

Local SQL groups contacts by lowercased email across every synced list, returning the duplicates with their list memberships. Useful before consolidating lists or auditing subscriber sprawl.

### Segment by tag set-algebra

```bash
emailoctopus-pp-cli tags intersect --list <list_id> --has trial-started --not activated --json
```

Returns the contacts tagged trial-started but not activated — the cohort that's still in trial but hasn't converted. EmailOctopus has no native segmentation API.

### Bulk delete with a safety net

```bash
emailoctopus-pp-cli contacts bulk-delete --where status=unsubscribed --list c81c21ca-51cc-11f1-ad3e-ffeba75576ac --rate 8 --dry-run
```

Resolves the predicate against the local store and shows what would be deleted. Drop --dry-run to actually delete, paced at 8 req/sec under the 10/sec API limit, with a resumable progress file in case the run is interrupted.

### Diff a list to see what changed

```bash
emailoctopus-pp-cli lists diff c81c21ca-51cc-11f1-ad3e-ffeba75576ac --since 1d --json
```

Returns contacts in the list whose local synced_at timestamp is within the requested window. Useful for incremental change detection — the API has no native change feed.

## Auth Setup

Generate a v2 key at https://api.emailoctopus.com/developer/api-keys/create and export it as `EMAILOCTOPUS_API_KEY`. Keys created before October 2024 are v1-only and will return 401 against the v2 API — regenerate if you see auth errors.

Alternatively, persist the key without an env var: `emailoctopus-pp-cli auth set-token` saves it to the config file; `emailoctopus-pp-cli auth status` shows where the active key was loaded from; `emailoctopus-pp-cli auth logout` clears it.

Run `emailoctopus-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  emailoctopus-pp-cli campaigns get --agent --select id,name,status
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
emailoctopus-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
emailoctopus-pp-cli feedback --stdin < notes.txt
emailoctopus-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.emailoctopus-pp-cli/feedback.jsonl`. They are never POSTed unless `EMAILOCTOPUS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `EMAILOCTOPUS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
emailoctopus-pp-cli profile save briefing --json
emailoctopus-pp-cli --profile briefing campaigns get
emailoctopus-pp-cli profile list --json
emailoctopus-pp-cli profile show briefing
emailoctopus-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `emailoctopus-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add emailoctopus-pp-mcp -- emailoctopus-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which emailoctopus-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   emailoctopus-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `emailoctopus-pp-cli <command> --help`.
