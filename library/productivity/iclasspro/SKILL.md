---
name: pp-iclasspro
description: "Read iClassPro public catalogs, local history, and an authenticated read-only Office Portal surface. Trigger phrases: `what classes does this gym have`, `check openings at`, `read iClassPro families`, `check the staff dashboard`, `read attendance`, `read transactions`, `use iclasspro`, `run iclasspro`."
author: "bobe"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - iclasspro-pp-cli
    install:
      - kind: go
        bins: [iclasspro-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/productivity/iclasspro/cmd/iclasspro-pp-cli
---

# iClassPro — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `iclasspro-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install iclasspro --cli-only
   ```
2. Verify: `iclasspro-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/iclasspro/cmd/iclasspro-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

iClassPro powers thousands of gymnastics, swim, dance, and cheer businesses and ships no developer API at all. This CLI treats the portal's own endpoints as one: it works against any account by its portal slug, keeps every sync in local SQLite, and answers the questions the upstream API structurally cannot — what changed since last week (drift), how fast a class is filling (fill-rate), and what registration opens next (opens-soon). It also exposes the filters the portal UI uses but nobody documented, including free-text search and openings-only.

## When to Use This CLI

Use this CLI whenever a task involves an iClassPro-powered business: gymnastics gyms, swim schools, dance and cheer studios. It is the right tool for reading a gym's class and camp catalog, searching or filtering it, watching for openings and registration windows, exporting schedules to a website or calendar, and comparing several gyms at once. It is especially suited to agencies and multi-location operators, because every command takes a portal slug and the local mirror can hold many accounts side by side. Prefer it over ad-hoc HTTP calls, because it knows which filters the API actually honors and which it silently ignores.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to enroll a student, add to a cart, apply a promo code, or pay — it is read-only by design and has no such commands.
- Do not use it to mutate staff-side records, take attendance, charge or refund a family, generate/export reports, read staff payroll, or manage time clocks. Its Office Portal surface is an explicit read-only allow-list.
- Do not use it to cancel or reschedule a booking on a family's behalf; direct the person to the customer portal.
- Do not point it at a class-management product that is not iClassPro — Mariana Tek, Vagaro, Jackrabbit, and Sutra all have unrelated APIs.
- Do not treat its output as a system of record for enrollment counts; openings is a portal-facing number, not the gym's roster.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local history the API forgets
- **`watch`** — Get told the moment a spot frees up in a class or camp that is currently full.

  _Reach for this instead of re-polling a list endpoint yourself: it stores every observation, so it reports the transition rather than the current value._

  ```bash
  iclasspro-pp-cli watch scottsdalegymnastics --class 8357 --agent
  ```
- **`drift`** — See what changed between syncs: classes and camps added, removed, retimed, or newly marked deleted.

  _Use this to answer 'what is different since last week' without re-reading the whole catalog into context._

  ```bash
  iclasspro-pp-cli drift scottsdalegymnastics --since 7d --agent
  ```
- **`opens-soon`** — Find registration that has not opened yet or is about to close, across every synced class and camp.

  _This is the only way to see a camp before its registration opens, which is when popular sessions are still winnable._

  ```bash
  iclasspro-pp-cli opens-soon scottsdalegymnastics --days 14 --agent
  ```
- **`fill-rate`** — Show how fast classes are filling over time, by class or by program.

  _Answers whether a class is trending toward full or toward cancellation, which no upstream call can express._

  ```bash
  iclasspro-pp-cli fill-rate scottsdalegymnastics --programs 589 --agent
  ```

### Catalog hygiene and publishing
- **`calendar`** — Export synced camps and classes to an RFC 5545 calendar file, one event per session.

  _Turns a portal catalog into something a calendar app, a website, or a newsletter can consume directly._

  ```bash
  iclasspro-pp-cli calendar scottsdalegymnastics --format ics --out fall-camps.ics
  ```
- **`lint`** — Flag catalog quality problems: missing descriptions or images, expired registration windows, deleted-but-listed programs.

  _Use before publishing a schedule to a website; it catches the records that will render blank or dead._

  ```bash
  iclasspro-pp-cli lint scottsdalegymnastics --agent
  ```

### Multi-tenant reach
- **Expired customer-session fallback** — Retry the anonymous Open API when a stored customer JWT returns HTTP 401.

  _A login that was once needed cannot later take down a catalog the account publishes publicly. Sign-in-gated accounts still surface their login requirement._

  ```bash
  iclasspro-pp-cli locations examplegym --agent
  ```
- **`tenant`** — Report which surfaces an account actually exposes: open, sign-in-gated, or plan-gated.

  _Run this first against any new account; it prevents mistaking a sign-in gate for a gym with no classes._

  ```bash
  iclasspro-pp-cli tenant examplegym --agent
  ```
- **`compare`** — Compare the same kind of program across several gyms at once.

  _Built for franchise and agency operators who track many gyms and need one table instead of many tabs._

  ```bash
  iclasspro-pp-cli compare --accounts scottsdalegymnastics,oasisgymnastics,tigar --agent
  ```

### Read-only Office Portal access
- **`admin`** — Read an explicit allow-list of authenticated staff resources without mutation, export, or response caching.

  _Provides staff-side visibility without giving an agent a generic request escape hatch or any write capability. Attendance reads discover the internal timeslot from the class and date when it is unambiguous._

  ```bash
  iclasspro-pp-cli admin families examplegym --q smith --limit 25 --agent
  iclasspro-pp-cli admin attendance examplegym 12345 2026-08-12 --agent
  ```

## Command Reference

**admin** — Authenticated, read-only Office Portal data

- `iclasspro-pp-cli admin capabilities` — Describe the allow-listed staff reads without requiring a session
- `iclasspro-pp-cli admin dashboard <account>` — Saved dashboard and available widget catalog
- `iclasspro-pp-cli admin families <account>` — Search families
- `iclasspro-pp-cli admin students <account>` — Search students
- `iclasspro-pp-cli admin class-search <account>` — Search staff-side classes
- `iclasspro-pp-cli admin enrollments <account>` — Search enrollments
- `iclasspro-pp-cli admin attendance <account> <class-id> <date> [timeslot-id]` — Read one roster and attendance state; omit `timeslot-id` to discover the unique event for that date
- `iclasspro-pp-cli admin transactions <account>` — Search gateway transaction history
- `iclasspro-pp-cli admin reports <account>` — List report definitions; never generate or export

**auth** — Read-only customer and staff sessions

- `iclasspro-pp-cli auth staff-login <account>` — Exchange environment credentials for a private Office Portal cookie session
- `iclasspro-pp-cli auth staff-status` — List staff sessions without exposing cookies
- `iclasspro-pp-cli auth staff-logout <account>` — Remove a stored staff session

**bookings** — The portal booking menu — the authoritative source of camp typeIds

- `iclasspro-pp-cli bookings <account> <locationId>` — Booking menu tiles for a location; camp tiles carry the typeId that 'camps list' requires

**camps** — Camps and events — open gyms, clinics, kids night out, school-break camps

- `iclasspro-pp-cli camps get` — Full camp detail including HTML description, per-session blocks, room, instructors, and deletion/expiry flags
- `iclasspro-pp-cli camps list` — List camps for one camp type. typeId comes from 'bookings', NOT from 'programs camps'

**classes** — Ongoing classes — the primary catalog

- `iclasspro-pp-cli classes get` — Full class detail including the HTML description the list endpoint omits
- `iclasspro-pp-cli classes list` — List classes with server-side filtering. Only the flags below are honored upstream; any other filter is applied locally

**instructors** — Instructors teaching classes at a location

- `iclasspro-pp-cli instructors <account> <locationId>` — Instructors who teach classes; ids are valid values for 'classes list --instructors'

**levels** — Skill levels used to band classes

- `iclasspro-pp-cli levels <account> <locationId>` — Active skill levels with display colors; ids are valid values for 'classes list --levels'

**locations** — Physical locations for an account

- `iclasspro-pp-cli locations <account>` — List every location on an iClassPro account, with contact details and portal branding

**news** — Portal news articles

- `iclasspro-pp-cli news <account> <articleId>` — Fetch a single portal news article by id

**parties** — Birthday party booking availability

- `iclasspro-pp-cli parties <account> <locationId>` — Dates a party can be booked at a location

**products** — ProShop retail catalog

- `iclasspro-pp-cli products <account> <locationId>` — Retail products with pricing, sale state, variations, and inventory

**programs** — Program categories for classes, camps, and appointments

- `iclasspro-pp-cli programs appointments` — Appointment program categories; returns a plan-gate message on accounts without the appointments subscription
- `iclasspro-pp-cli programs camps` — Camp program categories. These ids are programIds, NOT the typeIds 'camps list' needs — use 'bookings' for those
- `iclasspro-pp-cli programs classes` — Class program categories; ids are valid values for 'classes list --programs'

**sessions** — Enrollment sessions (date-bounded terms)

- `iclasspro-pp-cli sessions <account>` — Sessions for an account; ids are valid values for 'classes list --sessions'


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
iclasspro-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Narrow a large class list down to just what an agent needs

```bash
iclasspro-pp-cli classes list oasisgymnastics --openings 1 --limit 50 --agent --select id,name,openings,allowWaitlist,schedule.dayName,schedule.startTime
```

Some gyms return well over a hundred classes with nested schedule arrays; selecting dotted paths keeps the payload small enough to reason over without paging through raw JSON.

### Resolve camp type ids the correct way, then list that type

```bash
iclasspro-pp-cli bookings scottsdalegymnastics 1 --agent --select title,target,targetParams.typeId
```

The booking menu is the only authoritative source of camp type ids; feeding a programId from 'programs camps' into --type-id silently returns nothing.

### Publish a gym's camp schedule to a calendar file

```bash
iclasspro-pp-cli calendar scottsdalegymnastics --format ics --out fall-camps.ics
```

Builds one event per camp session from the block and schedule arrays, with the portal registration link attached to each event.

### Check several gyms for openings in one pass

```bash
iclasspro-pp-cli compare --accounts scottsdalegymnastics,oasisgymnastics,tigar --agent
```

Joins the synced copies of each account locally, which is the only way to line up programs whose names and ids differ per gym.

### Audit a catalog before pushing it to a client website

```bash
iclasspro-pp-cli lint scottsdalegymnastics --agent
```

Flags the records that will render blank or dead on a site: missing descriptions, missing images, expired registration windows, and programs already marked deleted upstream.

### Establish a staff session without putting credentials in shell history

```bash
ICLASSPRO_STAFF_USERNAME=staff-user ICLASSPRO_STAFF_PASSWORD='...' iclasspro-pp-cli auth staff-login examplegym
```

The password is used only for login and is never persisted. Verify with 'auth staff-status'; status output never exposes the cookie.

### Record an authoritative catalog snapshot

```bash
iclasspro-pp-cli sync examplegym --resources classes,camps --agent
```

Only a complete classes-and-camps walk replaces the snapshot used by `drift`. A classes-only or camps-only run still updates openings history and the search cache, but it cannot make the omitted resource type look deleted.

### Read staff-side operational data

```bash
iclasspro-pp-cli admin families examplegym --q smith --limit 25 --agent
```

The admin group covers dashboard, families, students, class search, enrollments, attendance, transactions, and report definitions through an explicit read-only endpoint allow-list.

### Keep public reads working after a customer session expires

```bash
iclasspro-pp-cli locations examplegym --agent
```

The CLI tries a stored customer session first. If iClassPro rejects that JWT with HTTP 401, it automatically retries the same read through the anonymous Open API. Public catalogs continue normally; gated catalogs return their sign-in message so you can run `auth login` again.

### Read attendance without hunting for an internal timeslot ID

```bash
iclasspro-pp-cli admin attendance examplegym 12345 2026-08-12 --agent
```

The CLI resolves the date's unique `tsId` through the Office Portal schedule endpoint before reading the roster. If a class has multiple events that day, the error lists their non-sensitive timeslot IDs so you can pass the intended one as the optional fourth argument.

## Auth Setup

Most accounts need no credentials for public catalog reads. Customer-gated catalogs use 'auth login'. If a stored customer token expires, the CLI retries the same read anonymously so an optional stale login cannot break a public catalog; a genuinely gated catalog still reports that a fresh login is required. Staff-side reads use a separate Office Portal session created by 'auth staff-login' from ICLASSPRO_STAFF_USERNAME and ICLASSPRO_STAFF_PASSWORD. Credentials are never accepted as flags, passwords are never persisted, and only server-issued session material is stored in the private 0600 session file. The admin surface is an explicit read-only allow-list and does not cache Office Portal responses.

Run `iclasspro-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  iclasspro-pp-cli bookings mock-value mock-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

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

- Use `--home <dir>` for one invocation, or set `ICLASSPRO_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `ICLASSPRO_CONFIG_DIR`, `ICLASSPRO_DATA_DIR`, `ICLASSPRO_STATE_DIR`, `ICLASSPRO_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `ICLASSPRO_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `iclasspro-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "iclasspro": {
        "command": "iclasspro-pp-mcp",
        "env": {
          "ICLASSPRO_HOME": "/srv/iclasspro"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `ICLASSPRO_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `ICLASSPRO_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
iclasspro-pp-cli recall "<user's question>" --agent
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
      "next_action": ["<trial command>", "iclasspro-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `iclasspro-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `iclasspro-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
iclasspro-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
iclasspro-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
iclasspro-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
iclasspro-pp-cli playbook amend \
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

`iclasspro-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `ICLASSPRO_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
iclasspro-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
iclasspro-pp-cli feedback --stdin < notes.txt
iclasspro-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `ICLASSPRO_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ICLASSPRO_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
iclasspro-pp-cli profile save briefing --json
iclasspro-pp-cli --profile briefing bookings mock-value mock-value
iclasspro-pp-cli profile list --json
iclasspro-pp-cli profile show briefing
iclasspro-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `iclasspro-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/iclasspro/cmd/iclasspro-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add iclasspro-pp-mcp -- iclasspro-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which iclasspro-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   iclasspro-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `iclasspro-pp-cli <command> --help`.
