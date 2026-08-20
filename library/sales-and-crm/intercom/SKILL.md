---
name: pp-intercom
description: "Every Intercom resource as a typed CLI with offline sync and EU/AU regions. Trigger phrases: `search intercom conversations`, `tag intercom conversations in bulk`, `intercom contact 360`, `intercom SLA report`, `pull intercom articles`, `use intercom-pp-cli`, `run intercom-pp-cli`."
author: "Rob Zehner"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - intercom-pp-cli
    install:
      - kind: go
        bins: [intercom-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/sales-and-crm/intercom/cmd/intercom-pp-cli
---

# Intercom — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `intercom-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install intercom --cli-only
   ```
2. Verify: `intercom-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/intercom/cmd/intercom-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

This CLI exposes the full Intercom REST API as a single Go binary. Every endpoint is a subcommand. A `sync` command mirrors conversations, contacts, companies, tickets, and articles into a local SQLite database so offline `search` and four headline transcendence commands — `incident-tag`, `articles pull/push`, `contact 360`, and `conversations sla` — answer the questions the API alone can't. Region-aware (`--region us|eu|au`) and built for agents (JSON-first, typed exit codes, MCP-mirrored).

## When to Use This CLI

Use intercom-pp-cli when you need to run the same Intercom operation across many records, when the official MCP doesn't reach your workspace (EU/AU), when you need offline analytics the API doesn't compute (first-response SLA, contact 360 joins), or when you're building an agent that triages, tags, or summarizes conversations and needs every Intercom action as a callable tool.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Incident response and bulk ops
- **`conversations incident-tag`** — Tag every conversation mentioning a phrase in a time window with one safe-by-default command.

  _When an incident hits, tagging 50 conversations by hand is the bottleneck; this is the single command that closes the loop without spreadsheets._

  ```bash
  intercom-pp-cli conversations incident-tag --mentions "checkout fails" --since 24h --tag incident-2026-05-24 --apply
  ```

### Help center as code
- **`articles pull`** — Flatten every help-center article into a markdown tree you can edit in git, then push changes back.

  _Lets docs teams version-control help-center content alongside product code; multilingual articles land as sibling files._

  ```bash
  intercom-pp-cli articles pull --to ./articles/
  ```

### Local-only analytics
- **`contact 360`** — One nested payload joining a contact across companies, conversations, tickets, notes, and tags.

  _Replaces 4-6 separate API calls and a mental merge; this is the lookup an agent makes before every triage decision._

  ```bash
  intercom-pp-cli contact 360 mei@example.com --agent
  ```
- **`conversations sla`** — First-response and resolution-time metrics grouped by team or admin, computed over the local store.

  _Replaces the weekly export-to-spreadsheet ritual; the API alone can't compute this in one call._

  ```bash
  intercom-pp-cli conversations sla --group-by team --metric first-response,resolution --since 7d --agent
  ```

## Command Reference

**admins** — Everything about your Admins

- `intercom-pp-cli admins list` — You can fetch a list of admins for a given workspace.
- `intercom-pp-cli admins list-activity-logs` — You can get a log of activities by all admins in an app.
- `intercom-pp-cli admins retrieve` — You can retrieve the details of a single admin.

**ai** — Manage ai

- `intercom-pp-cli ai create-content-import-source` — You can create a new content import source by sending a POST request to this endpoint.
- `intercom-pp-cli ai create-external-page` — You can create a new external page by sending a POST request to this endpoint.
- `intercom-pp-cli ai delete-content-import-source` — You can delete a content import source by making a DELETE request this endpoint.
- `intercom-pp-cli ai delete-external-page` — Sending a DELETE request for an external page will remove it from the content library UI and from being used for AI
- `intercom-pp-cli ai get-content-import-source` — Retrieve a content import source
- `intercom-pp-cli ai get-external-page` — You can retrieve an external page.
- `intercom-pp-cli ai list-content-import-sources` — You can retrieve a list of all content import sources for a workspace.
- `intercom-pp-cli ai list-external-pages` — You can retrieve a list of all external pages for a workspace.
- `intercom-pp-cli ai update-content-import-source` — You can update an existing content import source.
- `intercom-pp-cli ai update-external-page` — You can update an existing external page (if it was created via the API).

**articles** — Everything about your Articles

- `intercom-pp-cli articles create` — You can create a new article by making a POST request to `https://api.intercom.io/articles`.
- `intercom-pp-cli articles delete` — You can delete a single article by making a DELETE request to `https://api.intercom.io/articles/<id>`.
- `intercom-pp-cli articles list` — You can fetch a list of all articles by making a GET request to `https://api.intercom.io/articles`.
- `intercom-pp-cli articles retrieve` — You can fetch the details of a single article by making a GET request to `https://api.intercom.io/articles/<id>`.
- `intercom-pp-cli articles search` — You can search for articles by making a GET request to `https://api.intercom.io/articles/search`.
- `intercom-pp-cli articles update` — You can update the details of a single article by making a PUT request to `https://api.intercom.io/articles/<id>`.

**companies** — Everything about your Companies

- `intercom-pp-cli companies create-or-update-company` — You can create or update a company.
- `intercom-pp-cli companies delete-company` — Delete a single company. This endpoint does not permanently remove the company.
- `intercom-pp-cli companies list-all` — You can list companies.
- `intercom-pp-cli companies retrieve-acompany-by-id` — You can fetch a single company.
- `intercom-pp-cli companies retrieve-company` — You can fetch a single company by passing in `company_id` or `name`. `https://api.intercom.io/companies?
- `intercom-pp-cli companies scroll-over-all` — The `list all companies` functionality does not work well for huge datasets
- `intercom-pp-cli companies update-company` — You can update a single company using the Intercom provisioned `id`.

**contacts** — Everything about your contacts

- `intercom-pp-cli contacts create` — You can create a new contact (ie. user or lead).
- `intercom-pp-cli contacts delete` — You can delete a single contact.
- `intercom-pp-cli contacts list` — You can fetch a list of all contacts (ie. users or leads) in your workspace.
- `intercom-pp-cli contacts merge` — You can merge a contact with a `role` of `lead` into a contact with a `role` of `user`.
- `intercom-pp-cli contacts search` — You can search for multiple contacts by the value of their attributes in order to fetch exactly who you want.
- `intercom-pp-cli contacts show` — You can fetch the details of a single contact.
- `intercom-pp-cli contacts show-by-external-id` — You can fetch the details of a single contact by external ID. Note that this endpoint only supports users and not leads.
- `intercom-pp-cli contacts update` — You can update an existing contact (ie. user or lead).

**conversations** — Everything about your Conversations

- `intercom-pp-cli conversations create` — You can create a conversation that has been initiated by a contact (ie. user or lead).
- `intercom-pp-cli conversations delete` — {% admonition type='warning' name='Irreversible operation' %} Deleting a conversation is permanent and cannot be
- `intercom-pp-cli conversations list` — You can fetch a list of all conversations.
- `intercom-pp-cli conversations redact` — You can redact a conversation part or the source message of a conversation (as seen in the source object).
- `intercom-pp-cli conversations retrieve` — You can fetch the details of a single conversation.
- `intercom-pp-cli conversations search` — You can search for multiple conversations by the value of their attributes in order to fetch exactly which ones you
- `intercom-pp-cli conversations update` — You can update an existing conversation.

**custom-object-instances** — Everything about your Custom Object instances.
{% admonition type="warning" name="Permission Requirements" %}
  From now on, to access this endpoint, you need additional permissions. Please head over to the [Developer Hub](https://app.intercom.com/a/apps/_/developer-hub) app package authentication settings to configure the required permissions.
{% /admonition %}

- `intercom-pp-cli custom-object-instances create` — Create or update a custom object instance
- `intercom-pp-cli custom-object-instances delete-by-external-id` — Delete a single Custom Object instance using the Intercom defined id.
- `intercom-pp-cli custom-object-instances delete-by-id` — Delete a single Custom Object instance by external_id.
- `intercom-pp-cli custom-object-instances get-by-external-id` — Fetch a Custom Object Instance by external_id.
- `intercom-pp-cli custom-object-instances get-by-id` — Fetch a Custom Object Instance by id.

**data-attributes** — Everything about your Data Attributes

- `intercom-pp-cli data-attributes create` — You can create a data attributes for a `contact` or a `company`.
- `intercom-pp-cli data-attributes lis` — You can fetch a list of all data attributes belonging to a workspace for contacts, companies or conversations.
- `intercom-pp-cli data-attributes update` — You can update a data attribute.

**download** — Manage download

- `intercom-pp-cli download <job_identifier>` — When a job has a status of complete, and thus a filled download_url

**events** — Manage events

- `intercom-pp-cli events create-data` — You will need an Access Token that has write permissions to send Events.
- `intercom-pp-cli events data-summaries` — Create event summaries for a user.
- `intercom-pp-cli events lis-data` — > 🚧 > > Please note that you can only 'list' events that are less than 90 days old.

**help-center** — Everything about your Help Center

- `intercom-pp-cli help-center create-collection` — You can create a new collection by making a POST request to `https://api.intercom.io/help_center/collections.`
- `intercom-pp-cli help-center delete-collection` — You can delete a single collection by making a DELETE request to `https://api.intercom.io/collections/<id>`.
- `intercom-pp-cli help-center list` — You can list all Help Centers by making a GET request to `https://api.intercom.io/help_center/help_centers`.
- `intercom-pp-cli help-center list-all-collections` — You can fetch a list of all collections by making a GET request to `https://api.intercom.io/help_center/collections`.
- `intercom-pp-cli help-center retrieve` — You can fetch the details of a single Help Center by making a GET request to `https://api.intercom.
- `intercom-pp-cli help-center retrieve-collection` — You can fetch the details of a single collection by making a GET request to `https://api.intercom.
- `intercom-pp-cli help-center update-collection` — You can update the details of a single collection by making a PUT request to `https://api.intercom.io/collections/<id>`.

**intercom-export** — Manage intercom export

- `intercom-pp-cli intercom-export cancel-data` — Cancel content data export
- `intercom-pp-cli intercom-export create-data` — To create your export job, you need to send a `POST` request to the export endpoint `https://api.intercom.
- `intercom-pp-cli intercom-export get-data` — You can view the status of your job by sending a `GET` request to the URL `https://api.intercom.

**internal-articles** — Everything about your Internal Articles

- `intercom-pp-cli internal-articles create` — You can create a new internal article by making a POST request to `https://api.intercom.io/internal_articles`.
- `intercom-pp-cli internal-articles delete` — You can delete a single internal article by making a DELETE request to `https://api.intercom.io/internal_articles/<id>`.
- `intercom-pp-cli internal-articles list` — You can fetch a list of all internal articles by making a GET request to `https://api.intercom.io/internal_articles`.
- `intercom-pp-cli internal-articles retrieve` — You can fetch the details of a single internal article by making a GET request to `https://api.intercom.
- `intercom-pp-cli internal-articles search` — You can search for internal articles by making a GET request to `https://api.intercom.io/internal_articles/search`.
- `intercom-pp-cli internal-articles update` — You can update the details of a single internal article by making a PUT request to `https://api.intercom.

**me** — Manage me

- `intercom-pp-cli me` — You can view the currently authorised admin along with the embedded app object (a 'workspace' in legacy terminology).

**messages** — Everything about your messages

- `intercom-pp-cli messages` — You can create a message that has been initiated by an admin.

**news** — Everything about your News

- `intercom-pp-cli news create-item` — You can create a news item
- `intercom-pp-cli news delete-item` — You can delete a single news item.
- `intercom-pp-cli news list-items` — You can fetch a list of all news items
- `intercom-pp-cli news list-live-newsfeed-items` — You can fetch a list of all news items that are live on a given newsfeed
- `intercom-pp-cli news list-newsfeeds` — You can fetch a list of all newsfeeds
- `intercom-pp-cli news retrieve-item` — You can fetch the details of a single news item.
- `intercom-pp-cli news retrieve-newsfeed` — You can fetch the details of a single newsfeed
- `intercom-pp-cli news update-item` — Update a news item

**notes** — Everything about your Notes

- `intercom-pp-cli notes <id>` — You can fetch the details of a single note.

**phone-call-redirects** — Manage phone call redirects

- `intercom-pp-cli phone-call-redirects` — You can use the API to deflect phone calls to the Intercom Messenger.

**segments** — Everything about your Segments

- `intercom-pp-cli segments list` — You can fetch a list of all segments.
- `intercom-pp-cli segments retrieve` — You can fetch the details of a single segment.

**subscription-types** — Everything about subscription types

- `intercom-pp-cli subscription-types` — You can list all subscription types. A list of subscription type objects will be returned.

**tags** — Everything about tags

- `intercom-pp-cli tags create` — You can use this endpoint to perform the following operations: **1.
- `intercom-pp-cli tags delete` — You can delete the details of tags that are on the workspace by passing in the id.
- `intercom-pp-cli tags find` — You can fetch the details of tags that are on the workspace by their id. This will return a tag object.
- `intercom-pp-cli tags list` — You can fetch a list of all tags for a given workspace.

**teams** — Everything about your Teams

- `intercom-pp-cli teams list` — This will return a list of team objects for the App.
- `intercom-pp-cli teams retrieve` — You can fetch the details of a single team, containing an array of admins that belong to this team.

**ticket-states** — Everything about your ticket states

- `intercom-pp-cli ticket-states` — You can get a list of all ticket states for a workspace.

**ticket-types** — Everything about your ticket types

- `intercom-pp-cli ticket-types create` — You can create a new ticket type. > 📘 Creating ticket types.
- `intercom-pp-cli ticket-types get` — You can fetch the details of a single ticket type.
- `intercom-pp-cli ticket-types list` — You can get a list of all ticket types for a workspace.
- `intercom-pp-cli ticket-types update` — You can update a ticket type. > 📘 Updating a ticket type.

**tickets** — Everything about your tickets

- `intercom-pp-cli tickets create` — You can create a new ticket.
- `intercom-pp-cli tickets delete` — {% admonition type='warning' name='Irreversible operation' %} Deleting a ticket is permanent and cannot be reversed.
- `intercom-pp-cli tickets get` — You can fetch the details of a single ticket.
- `intercom-pp-cli tickets search` — You can search for multiple tickets by the value of their attributes in order to fetch exactly which ones you want.
- `intercom-pp-cli tickets update` — You can update a ticket.

**visitors** — Everything about your Visitors

- `intercom-pp-cli visitors convert` — You can merge a Visitor to a Contact of role type `lead` or `user`. > 📘 What happens upon a visitor being converted?
- `intercom-pp-cli visitors retrieve-with-user-id` — You can fetch the details of a single visitor.
- `intercom-pp-cli visitors update` — Sending a PUT request to `/visitors` will result in an update of an existing Visitor. **Option 1.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
intercom-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Tag every conversation mentioning an outage

```bash
intercom-pp-cli conversations incident-tag --mentions "checkout fails" --since 24h --tag incident-2026-05-24 --apply
```

Search server-side, dry-run by default, --apply fans out tag mutations through the adaptive limiter.

### Find slow-response teams this week

```bash
intercom-pp-cli conversations sla --group-by team --metric first-response,resolution --since 7d --agent --select team,first_response_p50_minutes,resolution_p90_hours
```

Local SQL over conversations + parts; --select narrows the nested payload so agents don't burn context.

### Pull every help-center article into a git-managed tree

```bash
intercom-pp-cli articles pull --to ./articles/
```

Each article lands as `<id>-<slug>.<locale>.md` with YAML frontmatter; commit the directory to git, edit locally, then run `intercom-pp-cli articles push --from ./articles/` to sync diffs back.

### Resolve a contact before triaging

```bash
intercom-pp-cli contact 360 mei@example.com --agent --select contact.id,companies,open_conversations,recent_tickets
```

Joins five entities locally; replaces 4-6 separate API calls and removes the agent's need to template Intercom's nested search predicate.

### List conversations updated this week with --select to keep payload small

```bash
intercom-pp-cli conversations list --agent --select conversations.id,conversations.state,conversations.assignee.id,conversations.tags.id,conversations.title
```

Generated endpoint mirror; `--select` uses dotted paths to walk Intercom's nested envelope so agents don't load 30 KB per conversation.

## Auth Setup

Intercom uses Bearer access tokens (PAT-style). Export `INTERCOM_ACCESS_TOKEN` or run `intercom-pp-cli auth set-token "<token>"` to save it to your config. The CLI pins the `Intercom-Version: 2.13` header on every request. Tokens are workspace-scoped; pick the right region with `--region us|eu|au` or `INTERCOM_REGION`.

Run `intercom-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  intercom-pp-cli admins list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and use `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `INTERCOM_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `INTERCOM_CONFIG_DIR`, `INTERCOM_DATA_DIR`, `INTERCOM_STATE_DIR`, `INTERCOM_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `INTERCOM_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `intercom-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "intercom": {
        "command": "intercom-pp-mcp",
        "env": {
          "INTERCOM_HOME": "/srv/intercom"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `INTERCOM_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `INTERCOM_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
intercom-pp-cli recall "<user's question>" --agent
```

The response envelope:

```json
{
  "query": "...",
  "normalized": "<normalized form>",
  "query_entities": ["..."],
  "found": true | false,
  "match_score": 0.0,
  "results": [
    { "resource_id": "...", "resource_type": "...", "venue": "...",
      "confidence": 2, "entity_match": "exact|partial|unknown",
      "source": "taught|preseed|pattern", "warnings": ["..."] }
  ],
  "mismatches": [ /* only when --debug-mismatches */ ],
  "warnings": [ /* top-level */ ],
  "candidates": [
    { "id": 12, "class": "flag_alias | playbook_candidate",
      "summary": "...", "sightings": 3, "last_seen": "...",
      "rationale": "...",
      "next_action": ["<trial command>", "intercom-pp-cli learnings confirm 12"] }
  ],
  "playbook": {
    "query_family": "...",
    "playbook": {
      "steps": [ { "cmd": "<command with {slot} substitution>", "purpose": "..." } ],
      "entity_slots": ["$ENTITY"],
      "expected_tool_calls": 3
    },
    "slots_resolved": { "$ENTITY": { "token": "<live token>", "canonical": "<canonical>" } },
    "notes": "<workarounds + gotchas for this query family>"
  },
  "notes": "<duplicate surface for non-playbook callers>"
}
```

Empty-store short-circuit: if the store has no learnings, playbooks, or candidates yet (recall finds nothing and `learnings list` and `learnings candidates` are both empty), skip recall for the rest of this session instead of taxing every query; resume recall-first once something has been taught.

### Step 2: decision tree

Read `candidates`, `playbook`, `notes`, `results[0]`, and warnings in that order:

```
if Candidates present (warnings include "candidates_present"):
    -> candidates are try-then-confirm, never facts. Follow each candidate's
       two-step next_action verbatim: run the trial command first, then run
       `learnings confirm <id>` only after the trial verified the behavior.
       Reject a wrong candidate with `learnings reject <id>`.
    -> NEVER re-teach something recall surfaced as a candidate; confirm or
       reject that candidate instead of teaching a duplicate.
    -> candidates ride alongside playbooks and resource hits, not instead of
       them; continue with the branches below after acting on them.

if Playbook present:
    -> READ Playbook.notes verbatim FIRST (workarounds + gotchas the CLI surface doesn't expose)
    -> replay Playbook.steps in order, substituting Playbook.slots_resolved entries
       for the entity slot tokens. If a step's slot is unresolved, fall back to
       discovery for that step only.
    -> the Playbook's expected_tool_calls is a budget; if you find yourself running
       materially more, record the divergence via `intercom-pp-cli playbook amend`
       at end-of-session.

elif Notes present (no Playbook):
    -> read Notes verbatim before any discovery step; they carry known gotchas
       for this query family even when no structured choreography exists yet.

elif Found AND Results[0].EntityMatch == "exact" AND Results[0].Confidence >= 2:
    -> skip discovery; fetch live data for Results[*].ResourceID in parallel

elif Found AND Results[0].EntityMatch == "partial":
    -> candidate hint, NOT a hit; read the resource title to validate before trusting

elif (any row in Mismatches[] when --debug-mismatches was passed):
    -> treat as cold start; the stored learning is for a different entity
       (different canonical resolved from query_entities)

else:  // Found == false, no playbook, no notes
    -> cold start; run discovery normally; teach the answer afterward (Step 4).
       If the family has no playbook yet, that teach auto-synthesizes a
       playbook candidate from this session's journal - you do not need to
       record one by hand.
```

Playbook and Notes are orthogonal to the per-resource path. A recall response can carry both a Playbook AND a `Results[]` hit - use both: the Playbook tells you which choreography to run; the resource hits short-circuit specific steps. Default to skipping `mismatches`; pass `--debug-mismatches` only when investigating cold-start surprises.

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `intercom-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `intercom-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
intercom-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
intercom-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
intercom-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
intercom-pp-cli playbook amend \
  --query "<exact recall query string>" \
  --add-note "<your concrete correction>"
# (append shell `&` to background it)
```

What counts as worth amending: a behavior you OBSERVED this session that future-you would benefit from knowing. Examples worth amending:

- A workaround for a CLI surface that silently drops or misorders a flag.
- An undocumented endpoint shape (response wrapped in `{meta, results}`, payload nested two levels deeper than the docs claim).
- Observed schema drift (a field renamed, an index that shifted between seasons, a category label that the API now returns lower-cased).

What does NOT belong in notes:

- The year-specific or entity-specific answer to the user's question. That's the response, not a learning.
- Per-team / per-athlete / per-row data the playbook already retrieves at runtime.
- Statements that paraphrase what the existing notes already say.

The amend command appends to the family's existing notes with a timestamped marker (`[amend YYYY-MM-DDTHH:MMZ]: <text>`). Multiple amends accumulate; the audit trail is visible. If no playbook exists yet for the family, amend creates a notes-only one (so cold-start corrections still land).

#### PII discipline for amend notes

`playbook amend` notes are designed to potentially flow upstream as shared knowledge in future versions of the Printing Press. Keep them clean of user-identifying content so the upstream-contribution path stays open without retroactive scrubbing:

- **Do NOT embed** paths to user filesystems, personal API keys or tokens, user email addresses, user GitHub handles, or specific query histories tied to a single user.
- **Acceptable**: endpoint shapes, undocumented field names, API gotchas, observed schema drift, workarounds for CLI surfaces, generalizable pagination or retry tactics.

If a correction is only meaningful with user-specific context, it belongs in a personal note, not in the playbook amend.

### Measuring the loop

`intercom-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `INTERCOM_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
intercom-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
intercom-pp-cli feedback --stdin < notes.txt
intercom-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `INTERCOM_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `INTERCOM_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

```
intercom-pp-cli profile save briefing --json
intercom-pp-cli --profile briefing admins list
intercom-pp-cli profile list --json
intercom-pp-cli profile show briefing
intercom-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `intercom-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/intercom/cmd/intercom-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add intercom-pp-mcp -- intercom-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which intercom-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   intercom-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `intercom-pp-cli <command> --help`.
