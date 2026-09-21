---
name: pp-sendfox
description: "Operate SendFox campaigns and audiences with typed commands, local evidence and guarded plans. Trigger phrases: `preflight a SendFox campaign`, `audit SendFox audience`, `plan an inactive SendFox automation`, `use sendfox`, `run sendfox`."
author: "cathrynlavery"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - sendfox-pp-cli
    install:
      - kind: go
        bins: [sendfox-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/marketing/sendfox/cmd/sendfox-pp-cli
---

# SendFox — Printing Press CLI

## Safety and evidence

This edition covers the documented 60-operation SendFox contract. Discover current commands with `which` and `capabilities`. Preview every write with `--dry-run`; execution requires `--yes`, and sends, schedules, activation, forms/domains or deletion additionally require `--approve-sensitive`. Do not interpret an evidence report's ready value or exit 0 as permission to act. The eight evidence reports read local JSON/YAML/SQLite only; reject live data-source requests and report missing/stale scopes explicitly. `workflow export-bundle` is strictly read-only; `workflow snapshot-save --out DIR` is the explicit local-file write. See EVIDENCE.md for the normalized snapshot schema and native-export limitations. Read-only authenticated behavior passed the publish-time live gate; account mutations, sends and delivery remain unverified.

For bulk jobs, `--preview-count` controls the API's count-only request while global `--dry-run` stays entirely local. No writes are automatically retried. The account budget is 60 requests/minute shared with every other client. Use `contacts bulk-wait` with a bounded request count to inspect progress.

MCP search/metadata/execute tools cover 60 operations with schemas; mutations preview unless confirm=true, with approve_sensitive=true additionally required for high-impact actions. Endpoint mirrors are hidden. Local workflows remain explicit tools.

## Prerequisites: Install the CLI

This skill drives the `sendfox-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install sendfox --cli-only
   ```
2. Verify: `sendfox-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/sendfox/cmd/sendfox-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Manage the current SendFox API from a predictable CLI and compact MCP surface. Combine campaign, audience and automation evidence locally; evaluate migration readiness without changing an account.

## When to Use This CLI

Use this CLI for SendFox audience operations, campaign preparation and reports, automation definitions and deterministic local export comparisons. Inspect plans and completeness before acting.

## Anti-triggers

Do not use this CLI for:
- Do not use for webhook delivery or purchases/orders: absent from the public contract.
- Do not use a readiness report as authorization to send, activate, change DNS/forms or migrate.

## Unique Capabilities

These workflows combine supplied local evidence into auditable reports and inactive plans.

### SendFox operations
- **`workflow audience-health`** — Find duplicate, invalid, suppressed and unassigned contacts, plus honest engagement cohorts.

  _Engagement timestamps form recency cohorts; missing or incomplete activity stays unknown rather than being labeled never-engaged._

  ```bash
  sendfox-pp-cli workflow audience-health --input examples/snapshot.json --agent
  ```
- **`workflow campaign-preflight`** — Check draft content, targeting, suppression evidence and sender readiness before any send.

  _Check draft content, targeting, suppression evidence and sender readiness before any send._

  ```bash
  sendfox-pp-cli workflow campaign-preflight --input examples/snapshot.json --agent
  ```
- **`workflow campaign-review`** — Combine campaign metrics, recipient-weighted benchmarks, trends, link performance and engagement cohorts into an auditable resend plan.

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
- **`workflow growth-report`** — Calculate timestamped audience growth from one snapshot and exact observed list joins, leaves and retention from two.

  _Every metric includes its formula, window, numerator, denominator, evidence grade, completeness and warnings. Missing evidence is null, never an invented zero._

  ```bash
  sendfox-pp-cli workflow growth-report --input current.json --previous previous.json --window 30d --agent
  ```

### Migration evidence
- **`workflow migration-readiness`** — Compare local platform snapshots with suppression, mapping, delta and cutover gates.

  _Compare local platform snapshots with suppression, mapping, delta and cutover gates._

  ```bash
  sendfox-pp-cli workflow migration-readiness --input examples/snapshot.json --agent
  ```

### Durable history helper

`workflow snapshot-save` is the explicit local-write companion to `export-bundle`. It requires `--out`, writes atomically with a `0700` directory and `0600` JSON file, and returns the artifact SHA-256.

```bash
sendfox-pp-cli workflow snapshot-save --input examples/snapshot.json --out ./history --agent
```

## Command Reference

**automation_emails** — Manage automation emails

- `sendfox-pp-cli automation-emails delete` — Remove an email from an automation
- `sendfox-pp-cli automation-emails update` — Update an automation email

**automations** — Manage automations

- `sendfox-pp-cli automations create` — Create an automation
- `sendfox-pp-cli automations create-email` — Add an email to an automation
- `sendfox-pp-cli automations delete` — Soft-deletes the automation and cancels all scheduled deliverables
- `sendfox-pp-cli automations get` — Returns automation with triggers, items, and campaign stats
- `sendfox-pp-cli automations list` — List automations
- `sendfox-pp-cli automations update` — Update title, trigger, or active status. Activating reschedules stale deliverables.

**campaigns** — Manage campaigns

- `sendfox-pp-cli campaigns create` — Creates a campaign as a draft. To send it, use the send endpoint or provide scheduled_at.
- `sendfox-pp-cli campaigns delete` — Only draft campaigns (not yet sent) can be deleted. Uses soft delete.
- `sendfox-pp-cli campaigns get` — Get a specific campaign
- `sendfox-pp-cli campaigns get-stats` — Returns sent count and open/click/bounce/unsubscribe/spam counts and rates, all read from stored counters.
- `sendfox-pp-cli campaigns list` — Returns a paginated list of campaigns (100 per page)
- `sendfox-pp-cli campaigns list-engagement` — Returns the contacts in one of a sent campaign's engagement groups.
- `sendfox-pp-cli campaigns resend` — Creates a new draft with the original's content, sender, and exclusions
- `sendfox-pp-cli campaigns send` — Schedules a draft campaign for immediate sending.
- `sendfox-pp-cli campaigns update` — Only draft campaigns (not yet sent) can be updated. All fields are optional.

**contact_fields** — Manage contact fields

- `sendfox-pp-cli contact-fields create` — Creates a custom field for contacts. The `name` is auto-generated from the `label` as a slug.
- `sendfox-pp-cli contact-fields delete` — Permanently deletes the contact field
- `sendfox-pp-cli contact-fields get` — Get a specific contact field
- `sendfox-pp-cli contact-fields list` — Returns custom contact fields defined by the user (20 per page)
- `sendfox-pp-cli contact-fields update` — Updates the label and auto-regenerates the name slug.

**contact_tags** — Manage contact tags

- `sendfox-pp-cli contact-tags create` — Creates a tag. A brand color is auto-assigned when none is provided. Tag names are unique per account.
- `sendfox-pp-cli contact-tags delete` — Deletes the tag.
- `sendfox-pp-cli contact-tags get` — Get a contact tag
- `sendfox-pp-cli contact-tags list` — Lists the account's tags, newest first, each with its contact count.
- `sendfox-pp-cli contact-tags update` — Update a contact tag

**contacts** — Manage contacts

- `sendfox-pp-cli contacts attach-tag` — Idempotent. Returns the contact's tags after the change.
- `sendfox-pp-cli contacts batch-import` — Import up to 1,000 contacts in a single request. Creates new contacts or updates existing ones.
- `sendfox-pp-cli contacts create` — Create a new contact
- `sendfox-pp-cli contacts create-bulk-action` — Queues one action against every contact the filter matches, applied in chunks in the background.
- `sendfox-pp-cli contacts delete` — Soft-deletes a contact and cancels any scheduled deliverables
- `sendfox-pp-cli contacts detach-tag` — Remove a tag from a contact
- `sendfox-pp-cli contacts get` — Get a specific contact
- `sendfox-pp-cli contacts get-activity` — Returns paginated email deliverables and contact-level engagement summary
- `sendfox-pp-cli contacts get-bulk-action` — For a dry run, matched_count is the answer and nothing was modified.
- `sendfox-pp-cli contacts list` — Returns a paginated list of contacts (100 per page by default, up to 1000 via `per_page`).
- `sendfox-pp-cli contacts list-tags-for` — List a contact's tags
- `sendfox-pp-cli contacts list-unsubscribed` — List unsubscribed contacts
- `sendfox-pp-cli contacts update` — Update contact details including name, list memberships, and custom fields

**domains** — Manage domains

- `sendfox-pp-cli domains create` — Adds a new sender domain and creates the corresponding SendGrid whitelabel domain.
- `sendfox-pp-cli domains delete` — Removes the domain from SendGrid and soft-deletes locally
- `sendfox-pp-cli domains get` — Returns domain details including DNS records needed for verification
- `sendfox-pp-cli domains list` — Returns a paginated list of the user's whitelabel/sender domains
- `sendfox-pp-cli domains validate` — Triggers DNS validation for the domain via SendGrid.

**forms** — Manage forms

- `sendfox-pp-cli forms create` — Creates a subscription form linked to one or more lists. Free users are limited to 1 form.
- `sendfox-pp-cli forms delete` — Soft-deletes the form
- `sendfox-pp-cli forms get` — Get a specific form
- `sendfox-pp-cli forms list` — List forms
- `sendfox-pp-cli forms update` — Update a form

**lists** — Manage lists

- `sendfox-pp-cli lists add-contact-to` — Adds an existing contact to a list. If the contact is already in the list, no duplicate is created.
- `sendfox-pp-cli lists contacts-in` — Get contacts in a list
- `sendfox-pp-cli lists create` — Create a new contact list
- `sendfox-pp-cli lists delete` — Soft-deletes a list. Returns 409 if the list is used by forms, landing pages, or automations.
- `sendfox-pp-cli lists get` — Returns list details including average open and click rates
- `sendfox-pp-cli lists list-lists` — List contact lists
- `sendfox-pp-cli lists remove-contact-from` — Remove a contact from a list
- `sendfox-pp-cli lists update` — Update a contact list

**me** — Manage me

- `sendfox-pp-cli me` — Get current user information

**unsubscribe** — Manage unsubscribe

- `sendfox-pp-cli unsubscribe` — Unsubscribe a contact by email


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
sendfox-pp-cli which "campaign statistics"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query. `--json` (and other machine formats) keep that exit-2 contract and write `{"matches":[]}` on stdout so agents can inspect the envelope without treating a miss as success.

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

## Auth Setup

Run `sendfox-pp-cli auth setup` for the URL and steps to obtain a token (add `--launch` to open the URL):

Or set `SENDFOX_API_TOKEN` as an environment variable.

Run `sendfox-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color`.

Global format flags share one contract on promoted, novel, sync, and `--deliver` paths:

- `--json` — one JSON document on stdout (sync progress events go to stderr)
- `--compact` — keep identity/status/timestamp fields; does not change the document vs stream shape
- `--csv` / `--plain` — tabular rows (collection envelopes unwrap to the row array)
- `--quiet` — one identity value per row, no envelope

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  sendfox-pp-cli automations list --agent --select active,automation_items,automation_triggers
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit confirmation** — `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
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

- Use `--home <dir>` for one invocation, or set `SENDFOX_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `SENDFOX_CONFIG_DIR`, `SENDFOX_DATA_DIR`, `SENDFOX_STATE_DIR`, `SENDFOX_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `SENDFOX_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, credential-scoped `data-<hash>.db` files, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Authenticated default stores are isolated by a one-way credential hash. Never treat an unscoped legacy `data.db` as belonging to the active account; run `sync` to populate the scoped store. An explicit `--db` is an operator-controlled override.
- Run `sendfox-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

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

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `SENDFOX_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `SENDFOX_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, pass the question as an argv or MCP tool argument to `recall --agent`. Do not interpolate user-controlled text into a shell command line.

Quoted `recall "<question>"` breaks on an apostrophe, which is ordinary English. A quoted heredoc breaks when a body line equals the delimiter, and that delimiter is published in these docs. Write the question with a non-shell file-writing tool, then read it back as data:

```bash
# Write the question verbatim with your file-writing tool (no shell involved).
# Command substitution on a file only ever yields data — the shell never
# parses the file's bytes as syntax.
QUERY=$(cat /path/to/question.txt)
sendfox-pp-cli recall "$QUERY" --agent
```

Prefer MCP: pass the question as the tool's query argument. `"$QUERY"` after a file read is argv-safe; putting the question itself in the command text is not.

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
      "next_action": ["<trial command>", "sendfox-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `sendfox-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `sendfox-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `sendfox-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately. Pass the query the same way as recall — argv/MCP, or file-then-`$QUERY`. Do not splice the question into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
sendfox-pp-cli teach --query "$QUERY" --resource-type campaigns --resource 42
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
QUERY=$(cat /path/to/question.txt)
sendfox-pp-cli teach \
  --query "$QUERY" \
  --resource 42 \
  --playbook-file ./playbooks/campaign-review.json \
  --playbook-notes-file ./playbooks/campaign-review-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
QUERY=$(cat /path/to/question.txt)
sendfox-pp-cli teach-playbook \
  --query "$QUERY" \
  --playbook-file ./playbooks/campaign-review.json \
  --notes-file ./playbooks/campaign-review-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`. Pass the query and note as argv/MCP arguments, or write each with a non-shell file tool and read them back (`QUERY=$(cat ...)`, `NOTE=$(cat ...)`). Do not interpolate either string into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
NOTE=$(cat /path/to/note.txt)
sendfox-pp-cli playbook amend \
  --query "$QUERY" \
  --add-note "$NOTE"
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

`sendfox-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `SENDFOX_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
sendfox-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
sendfox-pp-cli feedback --stdin < notes.txt
sendfox-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `SENDFOX_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SENDFOX_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename). Binary-response commands write decoded payload bytes (not the base64 JSON envelope) and print a small JSON receipt on stdout; `--json`/`--csv` do not refuse when this sink is set. |
| `webhook:<url>` | POST the output body to the URL (`application/json`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

```
sendfox-pp-cli profile save briefing --json
sendfox-pp-cli --profile briefing automations list
sendfox-pp-cli profile list --json
sendfox-pp-cli profile show briefing
sendfox-pp-cli profile delete briefing --yes
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
| 6 | Partial failure |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `sendfox-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/marketing/sendfox/cmd/sendfox-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add sendfox-pp-mcp -- sendfox-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which sendfox-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   sendfox-pp-cli capabilities --agent
   ```
4. If ambiguous, drill into subcommand help: `sendfox-pp-cli campaigns --help`.
