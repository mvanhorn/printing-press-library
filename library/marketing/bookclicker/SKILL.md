---
name: pp-bookclicker
description: "Every Bookclicker workflow, plus a local mirror of the newsletter marketplace that makes launch planning a single query. Trigger phrases: `plan my book launch on bookclicker`, `find newsletter swaps for my romance release`, `which newsletters should I book this month`, `confirm my sent promos`, `which partners cancel my swaps`, `use bookclicker`, `run bookclicker`."
author: "wmiles81"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - bookclicker-pp-cli
    install:
      - kind: go
        bins: [bookclicker-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/marketing/bookclicker/cmd/bookclicker-pp-cli
---

# Bookclicker — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `bookclicker-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install bookclicker --cli-only
   ```
2. Verify: `bookclicker-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/bookclicker/cmd/bookclicker-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Bookclicker is a marketplace where authors swap, sell and buy newsletter promo slots. Its web UI shows 25 newsletters at a time across many pages and makes you open one calendar per list. This CLI syncs the marketplace into local SQLite, so 'plan' can rank every candidate newsletter for a launch window by reach, price, or opens-per-dollar in one call, and keeps history so 'partner-roi', 'drift' and 'swap-balance' can answer questions the product structurally cannot.

## When to Use This CLI

Use this CLI for any Bookclicker task an author or their assistant does by hand: finding newsletters to promote a book, checking availability and pricing, sending or answering swap and paid offers, confirming sent promos, and reviewing which partners were worth booking. It is strongest when the question spans many newsletters at once, or spans time, because it answers those from a local mirror instead of paging the marketplace.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to change payment methods or delete a Bookclicker account; those surfaces are deliberately excluded.
- Do not use it to send newsletters — the actual sending happens in MailerLite or whichever provider the list is integrated with.
- Do not use it to read or manage third-party newsletter API keys; integration keys are redacted by design.
- Do not use it for a one-off lookup of a single known list when the web UI is already open.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Launch planning
- **`plan`** — Find every newsletter that can run your book in a date window, ranked by fit.

  _Reach for this instead of paging the marketplace: it answers 'who can promote this book, when, for how much' in one call._

  ```bash
  bookclicker-pp-cli plan --book 12345 --from 2026-09-01 --to 2026-09-30 --max-price 25 --agent
  ```
- **`search`** — Full-text search every synced newsletter by name, pen name, or genre in milliseconds.

  _Use for any 'which lists cover X' question rather than walking paginated API results._

  ```bash
  bookclicker-pp-cli search "romantic suspense" --agent
  ```
- **`plan`** — Rank candidate lists by estimated opens per dollar instead of raw subscriber count.

  _Biggest list is rarely the best value; this is the ranking the UI cannot express._

  ```bash
  bookclicker-pp-cli plan --from 2026-09-01 --to 2026-09-30 --rank value --limit 20 --agent
  ```

### Partner intelligence
- **`swap-balance`** — See which swap partners agreed to a swap and then cancelled or declined.

  _Use before rebooking a partner: it surfaces repeat cancellers that the product never flags._

  ```bash
  bookclicker-pp-cli swap-balance --flaky --agent
  ```
- **`partner-roi`** — Rank past promo partners by delivered reach against what they cost.

  _Use when deciding who to rebook; needs at least one prior sync of history._

  ```bash
  bookclicker-pp-cli partner-roi --since 180d --agent
  ```
- **`drift`** — Show newsletters whose open or click rate has decayed since earlier syncs.

  _Catches decaying lists before you spend a launch slot on them._

  ```bash
  bookclicker-pp-cli drift --min-drop 0.05 --agent
  ```

### Operations
- **`confirm-due`** — List every promo awaiting your confirmation, oldest first.

  _This is the product's recurring manual chore; run it before confirming promos._

  ```bash
  bookclicker-pp-cli confirm-due --agent
  ```
- **`launch health`** — Show which dates in a book's launch window still have no promo booked.

  _Answers 'is this launch actually covered' without reading a calendar by eye._

  ```bash
  bookclicker-pp-cli launch health --book 12345 --agent
  ```
- **`capacity`** — Show remaining Solo, Feature and Mention slots per newsletter per date.

  _Use before sending an offer to confirm the slot type is actually available._

  ```bash
  bookclicker-pp-cli capacity --list 12345 --from 2026-09-01 --to 2026-09-14 --agent
  ```
- **`stale`** — List pending offers that have gone unanswered the longest.

  _Surfaces offers worth cancelling and rebooking elsewhere._

  ```bash
  bookclicker-pp-cli stale --days 7 --agent
  ```

## Command Reference

**account** — Your Bookclicker account snapshot

- `bookclicker-pp-cli account` — Get the account snapshot: user settings, owned lists, books and pen names

**booking_calendars** — Booking-side availability view

- `bookclicker-pp-cli booking-calendars` — Get booking availability for a newsletter and book

**books** — Books you promote, grouped under pen names

- `bookclicker-pp-cli books create` — Add a book under a pen name
- `bookclicker-pp-cli books delete` — Delete a book
- `bookclicker-pp-cli books get` — Get one book by id
- `bookclicker-pp-cli books list` — List every book on your account
- `bookclicker-pp-cli books update` — Update a book

**calendars** — Dated availability for a newsletter

- `bookclicker-pp-cli calendars` — Get dated availability for a newsletter over N months

**confirm_promos** — Confirming that booked promotions actually went out

- `bookclicker-pp-cli confirm-promos create` — Confirm a promotion was sent, naming the campaign that carried it
- `bookclicker-pp-cli confirm-promos options` — List the newsletter campaigns that could satisfy a promotion

**conversations** — Messages with counterparties

- `bookclicker-pp-cli conversations` — Start a conversation with another user

**external_reservations** — Promotions booked outside Bookclicker

- `bookclicker-pp-cli external-reservations create` — Record a promotion booked off-platform
- `bookclicker-pp-cli external-reservations update` — Update an off-platform promotion record

**integrations** — Newsletter provider integrations

- `bookclicker-pp-cli integrations <id>` — Get one newsletter provider integration and its health status. The provider API key is redacted.

**inventories** — Per-date promotion slots on a newsletter

- `bookclicker-pp-cli inventories get` — Get the promotion slots offered on one date
- `bookclicker-pp-cli inventories set` — Set the promotion slots offered on one date

**lists** — The newsletter marketplace

- `bookclicker-pp-cli lists campaigns` — Campaign history for a newsletter
- `bookclicker-pp-cli lists search` — Search marketplace newsletters available to promote a book

**my_lists** — Newsletters you own and sell or swap spots on

- `bookclicker-pp-cli my-lists` — List your own newsletters with pricing and swap settings

**pen_names** — Author identities that own books and newsletters

- `bookclicker-pp-cli pen-names create` — Create a pen name
- `bookclicker-pp-cli pen-names delete` — Delete a pen name
- `bookclicker-pp-cli pen-names for-buyer` — List pen names eligible to book promotions as a buyer
- `bookclicker-pp-cli pen-names list` — List your pen names with their books, requests and groups
- `bookclicker-pp-cli pen-names update` — Update a pen name

**reservations** — Swap and paid promotion bookings

- `bookclicker-pp-cli reservations accept` — Accept an incoming promotion offer
- `bookclicker-pp-cli reservations buyer-cancel` — Cancel a promotion you booked
- `bookclicker-pp-cli reservations buyer-cancel-all` — Cancel every promotion you booked (bulk, destructive)
- `bookclicker-pp-cli reservations buyer-refund` — Process a buyer-side refund
- `bookclicker-pp-cli reservations decline` — Decline an incoming promotion offer
- `bookclicker-pp-cli reservations dismiss` — Dismiss a reservation notice from the feed
- `bookclicker-pp-cli reservations refund-request` — Request a refund for a paid promotion
- `bookclicker-pp-cli reservations request-confirmation` — Ask the seller to confirm a promotion was sent
- `bookclicker-pp-cli reservations seller-cancel` — Cancel a promotion booked on your newsletter
- `bookclicker-pp-cli reservations seller-cancel-all` — Cancel every promotion booked on your newsletters (bulk, destructive)
- `bookclicker-pp-cli reservations seller-refund` — Issue a refund as the newsletter owner


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
bookclicker-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Fill a launch window on a budget

```bash
bookclicker-pp-cli plan --book 12345 --from 2026-09-01 --to 2026-09-30 --max-price 25 --rank value --agent --select lists.name,lists.solo_price,lists.active_member_count,lists.open_rate
```

Ranks candidate newsletters by opens-per-dollar and narrows the payload to the four fields that drive the decision.

### Find partners who cancel swaps

```bash
bookclicker-pp-cli swap-balance --flaky --agent
```

Lists counterparties who agreed to a swap and then cancelled or declined it.

### Clear the confirmation backlog

```bash
bookclicker-pp-cli confirm-due --agent
```

Shows every promo awaiting confirmation so none silently ages out.

### Check a list before booking it

```bash
bookclicker-pp-cli capacity --list 12345 --from 2026-09-01 --to 2026-09-14
```

Shows remaining Solo, Feature and Mention slots per date against the platform caps.

### Spot decaying newsletters

```bash
bookclicker-pp-cli drift --min-drop 0.05
```

Flags lists whose open or click rate fell since an earlier sync, before you rebook them.

## Auth Setup

Bookclicker has no API keys or OAuth. Authentication is a Rails session cookie plus a CSRF token, exactly like a browser. Run 'auth login' to sign in and store the session in your local config, or import an existing browser session. Mutating commands automatically attach the X-CSRF-Token header. The stored session is a credential: it lives in your local config file and is never written to logs or output.

Run `bookclicker-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  bookclicker-pp-cli account --agent --select id,name,status
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

- Use `--home <dir>` for one invocation, or set `BOOKCLICKER_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `BOOKCLICKER_CONFIG_DIR`, `BOOKCLICKER_DATA_DIR`, `BOOKCLICKER_STATE_DIR`, `BOOKCLICKER_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `BOOKCLICKER_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `bookclicker-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "bookclicker": {
        "command": "bookclicker-pp-mcp",
        "env": {
          "BOOKCLICKER_HOME": "/srv/bookclicker"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `BOOKCLICKER_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `BOOKCLICKER_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
bookclicker-pp-cli recall "<user's question>" --agent
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
      "next_action": ["<trial command>", "bookclicker-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `bookclicker-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `bookclicker-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `bookclicker-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
bookclicker-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
bookclicker-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
bookclicker-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
bookclicker-pp-cli playbook amend \
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

`bookclicker-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `BOOKCLICKER_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
bookclicker-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
bookclicker-pp-cli feedback --stdin < notes.txt
bookclicker-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `BOOKCLICKER_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `BOOKCLICKER_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
bookclicker-pp-cli profile save briefing --json
bookclicker-pp-cli --profile briefing account
bookclicker-pp-cli profile list --json
bookclicker-pp-cli profile show briefing
bookclicker-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `bookclicker-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/marketing/bookclicker/cmd/bookclicker-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add bookclicker-pp-mcp -- bookclicker-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which bookclicker-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   bookclicker-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `bookclicker-pp-cli <command> --help`.
