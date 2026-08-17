---
name: pp-forkable
description: "A CLI and MCP server for your Forkable office-lunch program, with a local database and history, spend, and preference queries the web app cannot answer, plus commands to set, confirm, and skip meal orders (dry-run by default; --confirm to apply). Trigger phrases: `what have I eaten on forkable`, `forkable lunch spend this month`, `which forkable venues do we use most`, `did my forkable meals match my dietary preferences`, `forkable week ahead`, `set my forkable meal`, `skip forkable tomorrow`, `use forkable`, `run forkable`."
author: "Allen Lew"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - forkable-pp-cli
    install:
      - kind: go
        bins: [forkable-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/food-and-dining/forkable/cmd/forkable-pp-cli
---

# Forkable — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `forkable-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install forkable --cli-only
   ```
2. Verify: `forkable-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/forkable/cmd/forkable-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Forkable exposes no public API. This CLI reverse-engineers the my-account app's GraphQL surface into a Go binary with clean read commands and agent-native output. On top of the raw reads it adds longitudinal views the product never shows — served-meal history, preference-vs-served drift, spend trends, allowance utilization, venue rotation, and a week-ahead digest — all fetched live from Forkable. It also exposes the my-account app's own meal-management mutations as meal set, meal set-all, meal confirm, meal skip, and reorder; these are dry-run by default and only place, confirm, or skip real orders when you pass --confirm.

## When to Use This CLI

Use this CLI when an agent or script needs to query a Forkable office-lunch program from the terminal: what meals were served, how much was spent, which venues rotate, and whether served meals match stated dietary preferences. It is the only programmatic surface for Forkable and the only way to get longitudinal history the web app does not aggregate. It can also set, confirm, and skip upcoming meal deliveries on the user's behalf — those commands are dry-run by default and require `--confirm` to apply. All commands fetch live from Forkable (no local sync step).

## Anti-triggers

Do not use this CLI for:
- Do not place, change, or cancel meal orders without explicit user intent — the `meal set`, `meal set-all`, `meal confirm`, `meal skip`, and `reorder` commands mutate the real account and require `--confirm`; run them dry-run first and confirm the printed mutation before adding `--confirm`.
- Do not use this CLI to add or remove club members or manage billing — use the Forkable web app.
- Do not use this CLI for real-time delivery tracking notifications — those come via Forkable's email/Slack/SMS.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local history that compounds
- **`served-history`** — See every meal actually served to you over time, with date, venue, price, and dietary level.

  _Reach for this when an agent needs a longitudinal view of what a person has eaten, not just the current delivery._

  ```bash
  forkable-pp-cli served-history --since 90d --agent
  ```
- **`preference-drift`** — Flag served meals that violate your stated dislikes or dietary restrictions, or miss your likes.

  _Use this to audit whether auto-selection is actually honoring dietary preferences over time._

  ```bash
  forkable-pp-cli preference-drift --since 60d --agent
  ```
- **`venue-rotation`** — Rank venues by how often they've served you and how recently.

  _Use this to spot venue fatigue or under-used favorites across the whole synced window._

  ```bash
  forkable-pp-cli venue-rotation --since 120d --agent
  ```

### Making the opaque legible
- **`why-picked`** — Explain why a delivery's meal was auto-selected by ranking candidate items and their scores.

  _Pick this to explain a single day's auto-selected meal; use preference-drift for aggregate conformance._

  ```bash
  forkable-pp-cli why-picked --delivery 1219480 --agent
  ```

### Finance and allowances
- **`spend-trend`** — Bucket lunch spend into per-week or per-month totals with CSV export.

  _Reach for this when finance needs a time series of lunch cost, not a single delivery receipt._

  ```bash
  forkable-pp-cli spend-trend --since 6mo --by month --csv
  ```
- **`allowance-burn`** — Show granted-vs-consumed allowance utilization per club, including multi-club comparison.

  _Use this to see which teams are over- or under-using their lunch budget._

  ```bash
  forkable-pp-cli allowance-burn --by club --csv
  ```

### Agent-native plumbing
- **`upcoming-digest`** — One agent-shaped line per upcoming day: date, venue, auto-selected item, price, allowance headroom.

  _Pick this for a quick 'what's coming this week' summary an agent can read in one shot._

  ```bash
  forkable-pp-cli upcoming-digest --agent
  ```

## Command Reference

**account** — 

- `forkable-pp-cli account` — Show the authenticated Forkable user: profile, roles, dietary preferences (likes/dislikes/restrictions), companies

**buffet_addresses** — 

- `forkable-pp-cli buffet-addresses` — List your buffet delivery addresses (street, city, postal code, coordinates, club).

**clubs** — 

- `forkable-pp-cli clubs` — List meal clubs (teams/offices) you belong to or manage, with delivery address, delivery days, allowances

**csrf** — 

- `forkable-pp-cli csrf` — Fetch a CSRF token. Read-only.

**deliveries** — 

- `forkable-pp-cli deliveries in-progress-ids` — List IDs of deliveries currently in progress.
- `forkable-pp-cli deliveries list` — List your meal deliveries from a given date forward, including per-delivery orders, chosen menu items, receipts

**meal_scores** — 

- `forkable-pp-cli meal-scores` — Show meal auto-selection scores (menuId, itemId, score) for a delivery and user across candidate menus.

**menus** — 

- `forkable-pp-cli menus` — Get menu(s) with venue, sections, items, prices, dietary levels, ratings, and modifiers.

**notifications** — 

- `forkable-pp-cli notifications` — List account notifications shown in the my-account app (title, description, links, publish window).

**venue_usage** — 

- `forkable-pp-cli venue-usage` — Get per-venue usage keyed by venue id over a date range. Requires venue ids and from/to dates inlined in the query.

**meal management (writes)** — place real orders and spend; **dry-run by default**, pass `--confirm` to apply.

- `forkable-pp-cli meal set <deliveryId> --item <id> --menu <id> [--modifier <modifierId>:<optionId>] [--replace-piece <uuid>] [--note <text>] [--confirm]` — Override the auto-picked meal for one delivery day (`replacePiece`). `--replace-piece` takes the current piece's UUID (from `deliveries` or `served-history`).
- `forkable-pp-cli meal set-all --deliveries <id,id> --item <id> --menu <id> [--modifier <modifierId>:<optionId>] [--confirm]` — Apply one meal item across several delivery days (`replaceAllPieces`).
- `forkable-pp-cli meal confirm <deliveryId> [--unconfirm] [--confirm]` — Confirm (or `--unconfirm`) a delivery day (`confirmDelivery`).
- `forkable-pp-cli meal skip <deliveryId> [--confirm]` — Skip / cancel one or more delivery days (`removeDelivery`); `--deliveries <id,id>` skips several.
- `forkable-pp-cli reorder <fromDate> --onto <deliveryId> [--replace-piece <uuid>] [--confirm]` — Repeat the meal you had on a past date onto an upcoming delivery day.

**Required item options — `--modifier`.** An item with a required option group (e.g. "Choose Protein", `min: 1`) is rejected by Forkable unless you send a selection. Pass `--modifier <modifierId>:<optionId>[,<optionId>...]` (repeatable) — the CLI builds the `selectionsHash` (`{"16":[10]}`) the `replacePiece` mutation needs. Find modifier and option ids under each item's `modifiers` in the `menus` output. Example: `forkable-pp-cli meal set 12345 --item 678 --menu 90 --modifier 16:10 --replace-piece <uuid> --confirm`.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
forkable-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### What have I eaten this quarter

```bash
forkable-pp-cli served-history --since 90d --agent --select date,venue,name,price
```

Longitudinal list of served meals, narrowed to the high-signal fields to keep agent context small.

### Audit dietary conformance

```bash
forkable-pp-cli preference-drift --since 60d --json
```

Flags any served meal that conflicts with your stated dislikes or restrictions.

### Monthly lunch spend for finance

```bash
forkable-pp-cli spend-trend --since 6mo --by month --csv
```

Per-month spend totals exported as CSV for a budget close.

### Which teams are burning their allowance

```bash
forkable-pp-cli allowance-burn --by club --csv
```

Granted-vs-consumed allowance utilization per club, side by side.

### This week's lunch at a glance

```bash
forkable-pp-cli upcoming-digest --agent
```

One compact line per upcoming day for a quick agent-readable briefing.

## Auth Setup

Forkable authenticates with a browser session cookie plus a per-request CSRF token fetched from /api/v2/csrf_token. Log in to forkable.com in Chrome, then run 'forkable-pp-cli auth login --chrome' to import your session. There is no API key.

Run `forkable-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  forkable-pp-cli buffet-addresses --query example-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Safe writes** — read commands query Forkable; the write commands (`meal set`, `meal set-all`, `meal confirm`, `meal skip`, `reorder`) are dry-run by default and only mutate the account when invoked with `--confirm`
- **Live fetch** — commands query Forkable directly over GraphQL; there is no local sync/cache step
- **Non-interactive** — never prompts, every input is a flag

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

- Use `--home <dir>` for one invocation, or set `FORKABLE_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `FORKABLE_CONFIG_DIR`, `FORKABLE_DATA_DIR`, `FORKABLE_STATE_DIR`, `FORKABLE_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `FORKABLE_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `forkable-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "forkable": {
        "command": "forkable-pp-mcp",
        "env": {
          "FORKABLE_HOME": "/srv/forkable"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `FORKABLE_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `FORKABLE_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
forkable-pp-cli recall "<user's question>" --agent
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
      "next_action": ["<trial command>", "forkable-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `forkable-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `forkable-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet. This CLI fetches live (no sync command); populate lookups by running the relevant read command (e.g. `deliveries list`, `clubs list`).
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
forkable-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
forkable-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
forkable-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
forkable-pp-cli playbook amend \
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

`forkable-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `FORKABLE_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
forkable-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
forkable-pp-cli feedback --stdin < notes.txt
forkable-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `FORKABLE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `FORKABLE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
forkable-pp-cli profile save briefing --json
forkable-pp-cli --profile briefing buffet-addresses --query example-value
forkable-pp-cli profile list --json
forkable-pp-cli profile show briefing
forkable-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `forkable-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/food-and-dining/forkable/cmd/forkable-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add forkable-pp-mcp -- forkable-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which forkable-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   forkable-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `forkable-pp-cli <command> --help`.
