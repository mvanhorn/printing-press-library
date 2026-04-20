---
name: pp-hubspot
description: "Use this skill whenever the user asks about HubSpot CRM contacts, companies, deals, tickets, tasks, calls, emails, meetings, engagements, pipelines, deal velocity / stale deals / coverage, or wants to search across their CRM data. Also for creating / updating / deleting any HubSpot record or managing associations between objects. HubSpot CLI covering 15 HubSpot APIs with offline SQLite search and pipeline analytics. Requires a HubSpot access token. Triggers on phrasings like 'find contacts at Acme', 'show deals closing this month', 'which deals are stale', 'pipeline velocity this quarter', 'log a call for contact X', 'create a task for tomorrow'."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["hubspot-pp-cli"],"env":["HUBSPOT_ACCESS_TOKEN"]},"primaryEnv":"HUBSPOT_ACCESS_TOKEN","install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/cmd/hubspot-pp-cli@latest","bins":["hubspot-pp-cli"],"label":"Install via go install"}]}}'
---

# HubSpot — Printing Press CLI

Manage HubSpot CRM contacts, companies, deals, tickets, engagements, pipelines, and associations with offline search and pipeline analytics. Derived from 15 official HubSpot OpenAPI specs, covering the core CRM + engagements + marketing objects.

## When to Use This CLI

Reach for this when a user wants CRM lookups or mutations from outside the HubSpot web UI — especially when batched (find-then-update N contacts), analytical (pipeline velocity, stale deal detection), or chained with other tools via pipes. Agent-native output makes it ideal for "write this call note and tag the contact" style workflows.

Don't reach for this if the user has a dedicated workflow in HubSpot's own Workflows / Sequences tools that's cleaner, or if they need marketing-specific automation beyond the CRM object layer.

## Unique Capabilities

### Pipeline analytics

- **`deals velocity`** — Average time-in-stage per pipeline stage. Identifies stages where deals stall.

  _The "where is my funnel leaking" command. Dashboard visualizations hide the stage-level dwell time._

- **`deals stale [--days N]`** — Deals that haven't been updated in N days. Sales ops hygiene.

- **`deals coverage`** — Revenue coverage ratio — pipeline size vs quota. Forecasting tool.

### Search across CRM

- **`search "<query>"`** — Full-text search across synced contacts, companies, deals, tickets, notes, calls, emails. Single query spans all objects.

  _Much faster than HubSpot's per-object search UI; works offline after sync._

### Associations — the hard-to-use part

- **`associations <fromObjectType> <objectId> <toObjectType>`** — List associations from one record to another type (e.g., contacts on a deal, deals on a company).

  _Wraps HubSpot's quirky associations API (which requires specifying both ends of the type mapping) into a single call._

### Bulk operations

Every top-level object (`contacts`, `companies`, `deals`, `tickets`, etc.) supports create/update/delete with batch variants that the HubSpot API exposes. The CLI surfaces them consistently.

## Command Reference

CRM objects (each supports list, get, create, update, delete, search):

- `hubspot-pp-cli contacts`
- `hubspot-pp-cli companies`
- `hubspot-pp-cli deals` — plus `velocity`, `stale`, `coverage`
- `hubspot-pp-cli tickets`

Engagements:

- `hubspot-pp-cli tasks`
- `hubspot-pp-cli notes`
- `hubspot-pp-cli calls`
- `hubspot-pp-cli emails`
- `hubspot-pp-cli meetings`

Pipeline and schema:

- `hubspot-pp-cli pipelines` — List + get pipelines and stages
- `hubspot-pp-cli properties` — Property schema per object type
- `hubspot-pp-cli lists` — Static and active lists

Associations:

- `hubspot-pp-cli associations <fromType> <id> <toType>` — List related records

Marketing / misc:

- `hubspot-pp-cli owners` — CRM users / record owners
- `hubspot-pp-cli analytics` — Aggregate metrics

Local store + utility:

- `hubspot-pp-cli sync` / `export` / `import` / `archive` — Offline data ops
- `hubspot-pp-cli search <query>` — Cross-object full-text
- `hubspot-pp-cli auth set-token <HUBSPOT_ACCESS_TOKEN>`
- `hubspot-pp-cli doctor` — Verify

## Recipes

### Morning pipeline review

```bash
hubspot-pp-cli deals stale --days 14 --agent           # neglected deals
hubspot-pp-cli deals velocity --weeks 8 --agent        # stage-level timing over 8 weeks
hubspot-pp-cli deals coverage --pipeline <PIPELINE_ID> --limit 50 --agent
```

`deals stale` surfaces deals untouched for N days. `deals velocity` computes per-stage dwell times over the last N weeks. `deals coverage` lists deals in the pipeline; compare the total value against your quota externally.

### Find a contact and log a call

```bash
hubspot-pp-cli contacts search --query "alice@acme.com" --agent
hubspot-pp-cli calls create \
  --hs-call-title "Pricing discussion" \
  --hs-call-body "Discussed pricing, sending proposal by Thursday" \
  --hs-call-duration 1200000 --hs-call-direction OUTBOUND \
  --hs-timestamp "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --agent
```

Note: duration is milliseconds. The CLI's `associations <fromType> <id> <toType>` command LISTS associations — it doesn't create them. To associate the new call with the contact in one request, pipe a full JSON body via `calls create --stdin` that includes an `associations` array; see HubSpot's API docs for the body shape.

### What does Acme look like in our CRM?

```bash
COMPANY=$(hubspot-pp-cli companies search --query "Acme" --agent | jq -r '.results[0].id')
hubspot-pp-cli companies get "$COMPANY" --agent
hubspot-pp-cli associations companies "$COMPANY" contacts --agent   # people at Acme
hubspot-pp-cli associations companies "$COMPANY" deals --agent      # deals with Acme
```

One company lookup → associated contacts → associated deals. A full 360° CRM view for a single org.

### Cross-object search

```bash
hubspot-pp-cli search "hormone therapy" --agent
```

Returns matching contacts, companies, deals, tickets, and engagements (notes, calls, emails) in one response. Useful for domain-spanning queries.

## Auth Setup

HubSpot uses private app access tokens. Create one at [app.hubspot.com/l/settings/account-settings/integrations](https://app.hubspot.com/l/settings/account-settings/integrations) → Private Apps.

```bash
export HUBSPOT_ACCESS_TOKEN="pat-..."
hubspot-pp-cli auth set-token "$HUBSPOT_ACCESS_TOKEN"
hubspot-pp-cli doctor
```

Grant the private app the scopes you need (typically: `crm.objects.contacts.*`, `crm.objects.companies.*`, `crm.objects.deals.*`, `crm.objects.tickets.*`, plus engagement scopes). Missing scopes surface as exit code 4 with a specific scope name.

## Agent Mode

Add `--agent` to any command. Expands to `--json --compact --no-input --no-color --yes`. Useful flags: `--select`, `--dry-run`, `--rate-limit N` (HubSpot is aggressive on rate limits; throttle for bulk ops), `--query <expr>` for search commands.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error |
| 3 | Not found (contact, company, deal, etc.) |
| 4 | Auth required / missing scope |
| 5 | API error |
| 7 | Rate limited |
| 10 | Config error |

## Installation

### CLI

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/cmd/hubspot-pp-cli@latest

# If `@latest` installs a stale build (Go module proxy cache lag), install from main:
GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
  go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/cmd/hubspot-pp-cli@main
hubspot-pp-cli auth set-token YOUR_HUBSPOT_ACCESS_TOKEN
hubspot-pp-cli doctor
```

### MCP Server

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/cmd/hubspot-pp-mcp@latest

# If `@latest` installs a stale build (Go module proxy cache lag), install from main:
GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
  go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/cmd/hubspot-pp-mcp@main
claude mcp add -e HUBSPOT_ACCESS_TOKEN=<token> hubspot-pp-mcp -- hubspot-pp-mcp
```

## Argument Parsing

Given `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → run `hubspot-pp-cli --help`
2. **`install`** → CLI; **`install mcp`** → MCP
3. **"find contact / company / deal"** → `<object> search --query <expr> --agent`
4. **"pipeline health"** → `deals velocity | stale | coverage`
5. **"log call / task / note"** → `<engagement> create --hs-call-title/--hs-call-body/--hs-call-duration ... --agent`, then `associations <engagementType> <id> contacts <contactId>` to link
6. **Anything else** → check install + auth, route by CRM object verb, run with `--agent`.

<!-- pr-218-features -->
## Agent Workflow Features

This CLI exposes three shared agent-workflow capabilities patched in from cli-printing-press PR #218.

### Named profiles

Persist a set of flags under a name and reuse them across invocations.

```bash
# Save the current non-default flags as a named profile
hubspot-pp-cli profile save <name>

# Use a profile — overlays its values onto any flag you don't set explicitly
hubspot-pp-cli --profile <name> <command>

# List / inspect / remove
hubspot-pp-cli profile list
hubspot-pp-cli profile show <name>
hubspot-pp-cli profile delete <name> --yes
```

Flag precedence: explicit flag > env var > profile > default.

### --deliver

Route command output to a sink other than stdout. Useful when an agent needs to hand a result to a file, a webhook, or another process without plumbing.

```bash
hubspot-pp-cli <command> --deliver file:/path/to/out.json
hubspot-pp-cli <command> --deliver webhook:https://hooks.example/in
```

File sinks write atomically (tmp + rename). Webhook sinks POST `application/json` (or `application/x-ndjson` when `--compact` is set). Unknown schemes produce a structured refusal listing the supported set.

### feedback

Record in-band feedback about this CLI from the agent side of the loop. Local-only by default; safe to call without configuration.

```bash
hubspot-pp-cli feedback "what surprised you or tripped you up"
hubspot-pp-cli feedback list         # show local entries
hubspot-pp-cli feedback clear --yes  # wipe
```

Entries append to `~/.hubspot-pp-cli/feedback.jsonl` as JSON lines. When `HUBSPOT_FEEDBACK_ENDPOINT` is set and either `--send` is passed or `HUBSPOT_FEEDBACK_AUTO_SEND=true`, the entry is also POSTed upstream (non-blocking — local write always succeeds).

