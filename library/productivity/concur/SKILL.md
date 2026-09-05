---
name: pp-concur
description: "Every expense-report and travel workflow Concur's web app offers, plus duplicate detection and real flight/hotel search no Concur tool has -- filed through the same session your browser already uses. Trigger phrases: `file my Concur expense report`, `submit my expense report`, `what's in my Concur trip`, `use concur`, `run concur`."
author: "Allen Lew"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - concur-pp-cli
    install:
      - kind: go
        bins: [concur-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/accounting/concur/cmd/concur-pp-cli
---

# SAP Concur — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `concur-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install concur --cli-only
   ```
2. Verify: `concur-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/accounting/concur/cmd/concur-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

SAP Concur's official API requires enterprise partner credentials most individual users can never get. This CLI defaults to your logged-in browser session instead, so filing expense reports and checking travel works the same day you install it. Local SQLite sync turns your report history into something you can search, join, and validate offline.

## When to Use This CLI

Use this CLI for filing and checking your own SAP Concur expense reports and travel -- creating reports, pulling in available card charges, validating and submitting, and checking trip/itinerary status. It is the right choice whenever the task is something an individual employee would otherwise do by logging into concursolutions.com.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to register a new SAP Concur partner application or manage OAuth2 App Center listings -- that is an admin/partner workflow, not an end-user one.
- Do not use this CLI to actually book new travel (flights/hotels/cars). `flights search` and `hotels search` return real, live fares and rates from your actual corporate-negotiated policy for research and comparison, but completing a booking is a complex multi-step web/agent experience this CLI does not implement -- finish the booking in Concur's web app.
- Do not use this CLI for company-wide financial reporting or accounting-system integration -- use the documented v3/v4 partner REST API directly for that.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Conditional browser fallback for report creation
- **`reports create`** — Automatically and transparently retries report creation via automated browser when the Concur v4 API rejects pure HTTP requests with a `policyId is required` error. This fallback is completely conditional and only triggers for tenants requiring explicit policy assignment. It never guesses a Concur region: the UI host is derived only from a base URL that's actually `concursolutions.com`, or from an explicit `CONCUR_UI_BASE_URL` override — anything else is a clear error, not a silent default. If the browser click already succeeded before a later step fails, the error says so explicitly (with the report ID when known) instead of looking like a safely-retryable failure; do not blindly retry in that case.

### Local state that compounds
- **`expenses scan-duplicates`** — Find potential double-entered charges across all of your synced expenses.

  _Run this before submitting a batch of reports if you suspect a corporate-card charge and a manually-entered cash expense might be the same transaction._

  ```bash
  concur-pp-cli expenses scan-duplicates --agent
  ```

### Live travel shopping (search only, never books)
- **`flights search`** — Real flight availability and fares from a live shopping session against your actual corporate-negotiated rates and travel policy -- not a public fare aggregator. One-way by default; `--return` includes both legs in the search but only renders the outbound leg (see `--help` for the known gap).

  _Use this to compare real options before requesting travel, with policy-compliance flags already applied per fare._

  ```bash
  concur-pp-cli flights search --from LAX --to "New York" --depart 2026-10-12 --yes --agent
  ```
- **`hotels search`** — Real hotel availability and rates via a live, policy-scoped search -- the same inventory and pricing Concur's own Hotel Search page shows. Drives a real browser (see HTTP Transport and Auth Setup below) rather than a direct API call, because the hotel shopping-session mutation is blocked from scripted replay by the tenant's bot-mitigation.

  _A one-time dedicated-browser setup (see Auth Setup) avoids a separate login every time this command's session expires._

  ```bash
  concur-pp-cli hotels search --to "New York" --check-in 2026-10-12 --check-out 2026-10-18 --yes --agent
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

**Exception: `hotels search`.** Confirmed live that Concur's hotel shopping-session mutation is blocked from scripted HTTP replay by the tenant's bot-mitigation (byte-for-byte replay of a request that had just succeeded natively in the browser still failed). So this one command drives a real browser via `agent-browser` instead (`npm install -g agent-browser && agent-browser install`) -- `flights search` and every other command remain pure HTTP; only the hotel-shopping mutation needs a real browser.

## Command Reference

**account** — Current user profile, policies, and delegate context

- `concur-pp-cli account travel <user_id>` — Get the current user's travel profile and loyalty programs
- `concur-pp-cli account whoami` — Get the current user's profile, addresses, and travel IDs

**attendees** — Attendee catalog and per-expense attendee associations

- `concur-pp-cli attendees add` — Add attendees to an expense (merge-preserves existing associations; Concur's underlying association call is a replace
- `concur-pp-cli attendees list` — Get attendees currently associated with an expense

**delegates** — Delegate (act-on-behalf-of) relationships

- `concur-pp-cli delegates` — List users the current session user delegates for, with permission flags

**expense_types** — Expense type catalog and per-type dynamic form fields

- `concur-pp-cli expense-types list` — List usable expense types for the current user's policy

**expenses** — Expense line items within a report

- `concur-pp-cli expenses create` — Create an expense inside a report (core v3-equivalent fields: type, date, amount, currency, payment type)
- `concur-pp-cli expenses get` — Get a single expense with its filled/empty field manifest
- `concur-pp-cli expenses update` — Fill or change writable fields on an expense (core + custom/list fields)

**flights** — Search flight locations, travel policy preferences, and real flight availability (creates a live shopping session -- searches only, never books)

- `concur-pp-cli flights locations <query>` — Resolve an airport, city, or metro name to Concur's travel location IDs; metro queries (e.g. "New York") resolve to one search endpoint covering all constituent airports
- `concur-pp-cli flights preferences` — Show your travel policy's flight search defaults
- `concur-pp-cli flights search --from <origin> --to <dest> --depart <date> [--return <date>]` — Search real flight availability and fares

**hotels** — Search real hotel availability and rates (drives a real browser search -- searches only, never books)

- `concur-pp-cli hotels search --to <destination> --check-in <date> --check-out <date>` — Search real hotel availability and rates; requires `agent-browser` installed (see HTTP Transport and Auth Setup below)

**lists** — Valid values for list-type expense form fields

- `concur-pp-cli lists --list-id <id>` — Get valid values for a list-type form field by list ID

**locations** — Location catalog for filling expense/attendee location fields

- `concur-pp-cli locations <query>` — Search the location catalog by city or venue name

**payment_types** — Payment type catalog (Cash, Company Card, etc.)

- `concur-pp-cli payment-types` — List payment types available to the current user

**receipts** — Receipt image/PDF attachment

- `concur-pp-cli receipts <expense_id> --file <path>` — Attach a receipt image or PDF to an expense

**reports** — Expense report headers and lifecycle

- `concur-pp-cli reports create` — Create a new expense report header (transparently falls back to browser-driven creation on tenants requiring policy selection)
- `concur-pp-cli reports get` — Get a report's header, expenses, and web deep link
- `concur-pp-cli reports list` — List the current user's expense reports
- `concur-pp-cli reports submit` — Submit a report for approval
- `concur-pp-cli reports update` — Update a report's name or business purpose

**requests** — Travel requests / pre-trip authorization (UNVERIFIED paths -- see spec header notes)

- `concur-pp-cli requests get` — Get a travel request's detail and workflow status
- `concur-pp-cli requests list` — List the current user's travel requests

**travel_allowance** — Per-diem / travel allowance calculations (UNVERIFIED path -- see spec header notes)

- `concur-pp-cli travel-allowance <trip_id>` — Get travel allowance (per-diem) calculation results for a trip

**trips** — Booked trips and itineraries (UNVERIFIED paths -- see spec header notes)

- `concur-pp-cli trips get` — Get a trip's itinerary detail
- `concur-pp-cli trips list` — List the current user's upcoming and past trips


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
concur-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Check for duplicate charges

```bash
concur-pp-cli expenses scan-duplicates --agent
```

### Compare real flight and hotel options before requesting travel

```bash
concur-pp-cli flights search --from LAX --to "New York" --depart 2026-10-12 --yes --agent
concur-pp-cli hotels search --to "New York" --check-in 2026-10-12 --check-out 2026-10-18 --yes --agent
```

Both create a live shopping session against your real tenant -- searches only, never books. `flights search` is a direct API call; `hotels search` drives a real browser (see HTTP Transport and Auth Setup) and is markedly slower.

Scan the local SQLite cache for likely double-entered transactions across all your reports.

## Auth Setup

Concur's documented OAuth2 partner API is gated behind a Partner Enablement Manager relationship -- there is no self-serve signup, and this CLI does not implement that OAuth2 flow at all. Instead, this CLI authenticates via cookie/browser-session auth: run 'auth login --chrome', log into your company's Concur portal like you normally would (including SSO/MFA), and the CLI captures the resulting session. If a command fails with 401/403 and your company IT has partner OAuth2 credentials, that workflow requires calling the documented v3/v4 REST API directly (developer.concur.com) outside this CLI -- it is not something 'auth login' or any other command here can switch to.

Run `concur-pp-cli doctor` to verify setup.

### `hotels search` has a second, separate login by default

`hotels search` drives its own `agent-browser`-controlled Chrome instance (see HTTP Transport above), which does not share cookies with `auth login --chrome`'s source browser or credential store. Confirmed live that bridging them by copying cookies does not work -- Concur's bot-mitigation appears to bind the session to the browser/device that created it, not just the cookie value, so a copied JWT gets cleared by the server on the next navigation even when every cookie (including the Akamai bot-sensor ones) is copied alongside it. The first time (or whenever that session expires), `hotels search` opens its own Chrome window and asks you to log in there directly -- that login persists across later invocations until it expires again, so this is an occasional cost, not a per-search one.

**Optional one-time setup to avoid that second login entirely**: run a dedicated Chrome profile with remote debugging enabled and log into Concur there once. Use a real named profile (Chrome menu -> "Add Person", or chrome://settings -> Add profile) rather than a throwaway `--user-data-dir`, so `auth login --chrome --profile "<name>"` can read its cookies too:

```bash
"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \
  --remote-debugging-port=9222 --profile-directory="<profile dir name>"
```

`hotels search` auto-detects that session (tries CDP ports 9222, 9333, 9229 in order, or set `CONCUR_CDP_PORT` for a custom port) and *attaches* to it -- rather than copying its credentials -- before falling back to its own isolated login. Attaching, not copying, is what makes this work: it is the same live browser connection, so there is no separate device fingerprint for Concur's bot-mitigation to reject. Keep that Chrome window running whenever you plan to use `hotels search`; if you see "the dedicated Concur browser ... is no longer logged in", log in there again.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  concur-pp-cli payment-types --agent --select paymentTypeId,paymentTypeName,description
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

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `CONCUR_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `CONCUR_CONFIG_DIR`, `CONCUR_DATA_DIR`, `CONCUR_STATE_DIR`, `CONCUR_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `CONCUR_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `concur-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "concur": {
        "command": "concur-pp-mcp",
        "env": {
          "CONCUR_HOME": "/srv/concur"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `CONCUR_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `CONCUR_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
concur-pp-cli recall "<user's question>" --agent
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
      "next_action": ["<trial command>", "concur-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `concur-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `concur-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `concur-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
concur-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
concur-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
concur-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
concur-pp-cli playbook amend \
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

`concur-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `CONCUR_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
concur-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
concur-pp-cli feedback --stdin < notes.txt
concur-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `CONCUR_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `CONCUR_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
concur-pp-cli profile save briefing --json
concur-pp-cli --profile briefing payment-types
concur-pp-cli profile list --json
concur-pp-cli profile show briefing
concur-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Async Jobs

For endpoints that submit long-running work, the generator detects the submit-then-poll pattern (a `job_id`/`task_id`/`operation_id` field in the response plus a sibling status endpoint) and wires up three extra flags on the submitting command:

| Flag | Purpose |
|------|---------|
| `--wait` | Block until the job reaches a terminal status instead of returning the job ID immediately |
| `--wait-timeout` | Maximum wait duration (default 10m, 0 means no timeout) |
| `--wait-interval` | Initial poll interval (default 2s; grows with exponential backoff up to 30s) |

Use async submission without `--wait` when you want to fire-and-forget; use `--wait` when you want one command to return the finished artifact.

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

1. **Empty, `help`, or `--help`** → show `concur-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/accounting/concur/cmd/concur-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add concur-pp-mcp -- concur-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which concur-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   concur-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `concur-pp-cli <command> --help`.
