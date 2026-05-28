---
name: pp-hubspot
description: "Read-only contact analytics for HubSpot CRM — score drift, engagement decay, source ROI, and a daily digest Trigger phrases: `find dark hubspot contacts`, `hubspot score dropped`, `hubspot lifecycle stuck`, `hubspot source roi`, `hubspot duplicate contacts`, `use hubspot`, `run hubspot`."
author: "sdhilip200"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - hubspot-pp-cli
    install:
      - kind: go
        bins: [hubspot-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/cmd/hubspot-pp-cli
---

# HubSpot — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `hubspot-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install hubspot --cli-only
   ```
2. Verify: `hubspot-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/cmd/hubspot-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Every existing HubSpot CLI is CMS dev tooling or generic CRUD; every HubSpot MCP is conversational and write-heavy. This CLI mirrors the CRM Contacts API as typed commands, then layers nine cross-time analytics commands on top of a local SQLite store: engagement-decay, lifecycle-stuck, stale-but-valuable, source-roi, score-drift, owner-overload, silent-after-first-touch, duplicate-suspects, and daily-digest.

## When to Use This CLI

Reach for hubspot-pp-cli when the task is read-only contact analytics — re-engagement lists, lifecycle SLA breach detection, owner workload balancing, source ROI ranking, score-trend monitoring, or duplicate hygiene. The CLI works offline against a local SQLite store synced from the live API, so cross-time and composite-rank questions land in milliseconds.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI for create/update/delete on contacts; scopes are read-only by design and write workflows belong in HubSpot's UI or workflows.
- Do not use this CLI for Companies, Deals, Tickets, or Engagements — only Contacts. Those surfaces are out of scope for this CLI. Use a future HubSpot CLI variant once those catalog entries are available.
- Do not use this CLI for HubSpot CMS, themes, or developer tooling; HubSpot's own `hubspot-cli` is the right tool there.
- Do not use this CLI as a backup/export tool for compliance evidence — sync is incremental and the local store reflects whatever you last pulled.
- Do not call `crm post/patch/delete-*` write commands; they exist for spec completeness but the recommended Private App scopes (`crm.objects.contacts.read`, `crm.schemas.contacts.read`) block them with HTTP 403.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Cross-time analytics
- **`engagement-decay`** — Find previously-active contacts who have gone dark, ranked by prior engagement intensity.

  _Reach for this on Monday mornings to find warm leads that cooled — the classic re-engagement list a rep would otherwise build by hand._

  ```bash
  hubspot-pp-cli engagement-decay --window 30d --min-prior-opens 3 --json
  ```
- **`lifecycle-stuck`** — Surface contacts stuck in a lifecycle stage past team-median dwell time.

  _Reach for this when a sales manager asks 'who's breaching SLA?' — outliers relative to the team's own baseline, not a hardcoded threshold._

  ```bash
  hubspot-pp-cli lifecycle-stuck --stage marketingqualifiedlead --multiplier 2.0 --json
  ```
- **`score-drift`** — Find contacts whose HubSpot score dropped meaningfully over a window — leading indicator before lifecycle catches up.

  _Reach for this as an early disengagement signal that beats lifecycle stage by days or weeks._

  ```bash
  hubspot-pp-cli score-drift --window 14d --drop-pct 25 --json
  ```
- **`daily-digest`** — One-command morning briefing — new high-score contacts, biggest score drops, lifecycle promotions, freshly stale VIPs.

  _Reach for this on Monday morning as the single command a sales manager runs with coffee._

  ```bash
  hubspot-pp-cli daily-digest --since 1d --json
  ```

### Composite ranking
- **`stale-but-valuable`** — Find high-score, high-revenue contacts whose engagement has cooled — the VIPs going cold.

  _Reach for this when prioritizing save-the-whale outreach. Composite ranking is what makes it different from a plain filter._

  ```bash
  hubspot-pp-cli stale-but-valuable --score-min 70 --silent-days 21 --json
  ```
- **`source-roi`** — Rank contact acquisition sources by realized revenue and conversion, not raw volume.

  _Reach for this when marketing asks where to defund. Filters small-N sources so the ranking isn't dominated by noise._

  ```bash
  hubspot-pp-cli source-roi --since 2026-01-01 --min-contacts 20 --json
  ```

### Org-level pivots
- **`owner-overload`** — Flag sales reps carrying more active contacts than team median, weighted by lifecycle stage.

  _Reach for this when balancing book-of-business across AEs — the standard manual answer is 'export to Sheets and pivot.'_

  ```bash
  hubspot-pp-cli owner-overload --weight-by-stage --top 10 --json
  ```

### Pattern detection
- **`silent-after-first-touch`** — Find contacts who engaged exactly once and then went silent — distinguish tire-kickers from genuine interest.

  _Reach for this when triaging which one-touch contacts are worth a follow-up vs. archive._

  ```bash
  hubspot-pp-cli silent-after-first-touch --since 90d --json
  ```
- **`duplicate-suspects`** — Surface likely duplicate contacts via fuzzy match on email-local-part, normalized name, and company.

  _Reach for this during quarterly CRM hygiene. Outputs pair candidates with scores; merge stays manual (write scope intentionally out)._

  ```bash
  hubspot-pp-cli duplicate-suspects --threshold 0.85 --json
  ```

## Command Reference

**crm** — Manage crm

- `hubspot-pp-cli crm delete-v3-objects-contacts-contact-id-archive` — Delete a contact by ID. Deleted contacts can be restored within 90 days of deletion.
- `hubspot-pp-cli crm get-v3-objects-contacts-contact-id-get-by-id` — Retrieve a contact by its ID (`contactId`) or by a unique property (`idProperty`).
- `hubspot-pp-cli crm get-v3-objects-contacts-get-page` — Retrieve all contacts, using query parameters to specify the information that gets returned.
- `hubspot-pp-cli crm patch-v3-objects-contacts-contact-id-update` — Update an existing contact, identified by ID or email/unique property value.
- `hubspot-pp-cli crm post-v3-objects-contacts-batch-archive-archive` — Archive a batch of contacts by ID. Archived contacts can be restored within 90 days of deletion.
- `hubspot-pp-cli crm post-v3-objects-contacts-batch-create-create` — Create a batch of contacts.
- `hubspot-pp-cli crm post-v3-objects-contacts-batch-read-read` — Retrieve a batch of contacts by ID (`contactId`) or unique property value (`idProperty`).
- `hubspot-pp-cli crm post-v3-objects-contacts-batch-update-update` — Update a batch of contacts by ID (`contactId`) or unique property value (`idProperty`).
- `hubspot-pp-cli crm post-v3-objects-contacts-batch-upsert-upsert` — Upsert a batch of contacts.
- `hubspot-pp-cli crm post-v3-objects-contacts-create` — Create a single contact. Include a `properties` object to define [property values](https://developers.hubspot.
- `hubspot-pp-cli crm post-v3-objects-contacts-gdpr-delete-purge` — Permanently delete a contact and all associated content to follow GDPR.
- `hubspot-pp-cli crm post-v3-objects-contacts-merge-merge` — Merge two contact records. Learn more about [merging records](https://knowledge.hubspot.com/records/merge-records).
- `hubspot-pp-cli crm post-v3-objects-contacts-search-do-search` — Search for contacts by filtering on properties, searching through associations, and sorting results.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
hubspot-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Morning sales-manager digest

```bash
hubspot-pp-cli daily-digest --since 1d --json --select score_movers,stage_promotions,fresh_stale_vips
```

One command, three sections of cross-time deltas. Use `--select` so the response stays small enough to thread through an agent.

### Find dark VIPs

```bash
hubspot-pp-cli stale-but-valuable --score-min 70 --silent-days 21 --limit 25 --json
```

Composite rank by value × silence. Pair with `engagement-decay` to triangulate which save-the-whale candidates deserve outreach this week.

### Channel ROI without small-N noise

```bash
hubspot-pp-cli source-roi --since 2026-01-01 --min-contacts 20 --json
```

Joins source attribution to deal revenue, filters sources below the volume threshold so noisy 2-contact channels can't dominate.

### Owner load sanity check

```bash
hubspot-pp-cli owner-overload --weight-by-stage --top 10 --json
```

Weighted by lifecycle stage so SQLs count more than Leads. Shows reps above the team median — surface for the next 1:1.

### Dedupe candidates for hygiene sprints

```bash
hubspot-pp-cli duplicate-suspects --threshold 0.85 --json --select left_email,right_email,similarity,evidence
```

Pairwise fuzzy match goes beyond HubSpot's exact-email dedupe. `--select` keeps the envelope tight.

## Auth Setup

HubSpot read-only access uses a Private App access token. Create one at https://app.hubspot.com/private-apps, grant `crm.objects.contacts.read` and `crm.schemas.contacts.read`, then export it as `HUBSPOT_PRIVATE_APP_TOKEN`. The CLI sends it as `Authorization: Bearer <token>`. No write scopes — by design.

Run `hubspot-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  hubspot-pp-cli crm get-v3-objects-contacts-get-page --limit 5 --agent --select id,properties.email,properties.firstname
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
hubspot-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
hubspot-pp-cli feedback --stdin < notes.txt
hubspot-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/hubspot-pp-cli/feedback.jsonl`. They are never POSTed unless `HUBSPOT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `HUBSPOT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
hubspot-pp-cli profile save briefing --json
hubspot-pp-cli --profile briefing crm get-v3-objects-contacts-get-page --limit 5
hubspot-pp-cli profile list --json
hubspot-pp-cli profile show briefing
hubspot-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `hubspot-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/cmd/hubspot-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add hubspot-pp-mcp -- hubspot-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which hubspot-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   hubspot-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `hubspot-pp-cli <command> --help`.
