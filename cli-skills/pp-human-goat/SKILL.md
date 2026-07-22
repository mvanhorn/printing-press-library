---
name: pp-human-goat
description: "Hire real humans from the terminal — autonomous TaskRabbit checkout with a verified undo, plus Magic remote errands, in one agent-native binary. Trigger phrases: `hire a mover on saturday`, `book someone to assemble my furniture`, `call this number and ask`, `who did i hire before`, `cancel that booking`, `use human-goat`, `run goat`."
author: "Matt Van Horn"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - human-goat-pp-cli
    install:
      - kind: go
        bins: [human-goat-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/productivity/human-goat/cmd/human-goat-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/productivity/human-goat/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Human-Goat — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `human-goat-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install human-goat --cli-only
   ```
2. Verify: `human-goat-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/human-goat/cmd/human-goat-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

goat unifies two human networks behind a common task model: TaskRabbit for in-person local labor and Magic for remote errands. Its headline is hands-off checkout on TaskRabbit against the card on file — searched, ranked by honest all-in price and review quality, and booked with no prompt — made safe by a spend cap and a cancel command that verifies the cancellation actually landed.

## When to Use This CLI

Use goat when an agent should get a real-world task done end to end — hire a mover, assemble furniture, make a phone call, book something online — rather than surfacing options for a human to finish. It is strongest for autonomous TaskRabbit hiring with a safety net and for dispatching remote errands to Magic.

## Anti-triggers

Do not use this CLI for:
- Do not use goat to edit the payment method on file — the card is managed in the TaskRabbit app.
- Do not use goat for programmatic password login to either service — auth is cookie/api-key only.
- Do not use goat to move money outside a task booking — no transfers, payouts, or standalone tipping.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Hands-off hiring
- **`hire`** — Say the job and the date; goat searches, ranks by review quality and honest all-in price, and checks out against the card on file with no prompt.

  _Reach for this when the user wants a task done, not a Tasker shortlist to review._

  ```bash
  human-goat-pp-cli hire "help moving" --on 2026-07-11 --min-rating 4.8 --max-total 200 --agent --lat 47.6062 --lng -122.3321
  ```
- **`cancel`** — Cancels a booking and confirms it landed by re-reading status, reporting whether it was inside the free window and any fee.

  _This is the undo that makes autonomous checkout tolerable; always available after a hire._

  ```bash
  human-goat-pp-cli cancel task_abc123 --agent
  ```
- **`hire`** — Refuses to check out when the computed all-in total exceeds a configurable ceiling, printing the total and the cap.

  _Use --max-total (or the config default) to cap autonomous spend before any booking is placed._

  ```bash
  human-goat-pp-cli hire movers --on saturday --min-rating 4.9 --max-total 150 --lat 47.6062 --lng -122.3321
  ```

### Honest pricing
- **`best`** — Folds TaskRabbit's hidden service (~15%) and trust-and-support (5-15%) fees into the displayed hourly rate, honoring the CA/MA service-fee-only rule.

  _Every price surface (search, compare, best, hire, spend) shows the real all-in rate, not the teaser._

  ```bash
  human-goat-pp-cli best "help moving" --on saturday --min-rating 4.9 --agent
  ```
- **`watch`** — Polls recommendations for a category and date (optionally a favorite or a rate ceiling) and alerts when a match opens.

  _Use when nothing good is available now and you want the first qualifying slot._

  ```bash
  human-goat-pp-cli watch movers --on saturday --max-rate 60
  ```

### One surface, two human networks
- **`dispatch`** — Routes a plain-language task to Magic (remote-doable) or TaskRabbit (in-person) by task shape, with a --via override.

  _Say what you want done and let goat pick the human network; force it with --via taskrabbit|magic._

  ```bash
  human-goat-pp-cli dispatch "call the dentist and reschedule my cleaning"
  ```
- **`spend`** — SQL over local booking, invoice, and Magic-task history by category, tasker, source, or month, using true effective all-in $/hr for TaskRabbit.

  _Answers where the money went across both human networks with fees folded in._

  ```bash
  human-goat-pp-cli spend --by source --agent
  ```
- **`status`** — One list of every in-flight task across TaskRabbit bookings and Magic requests, joined on the common task model and sorted by state.

  _Use for a cross-source view of everything in flight; use 'tasks list' for TR-only and 'track <id>' for one Magic request._

  ```bash
  human-goat-pp-cli status --open --agent
  ```

## Command Reference

**account** — The logged-in TaskRabbit client account profile

- `human-goat-pp-cli account` — Get the authenticated account profile

**categories** — TaskRabbit task categories and templates for the account metro

- `human-goat-pp-cli categories` — List task templates for the account metro (title, category_name, category_id, default_template_id, top_category)

**invoices** — Invoice and payment-history flags

- `human-goat-pp-cli invoices` — Whether the account has submitted invoices (payment-history presence flag)

**system** — TaskRabbit account bootstrap, metro, and dashboard reachability

- `human-goat-pp-cli system bootstrap` — Account bootstrap — metro (id, name, country), payment_method_types, stream_api_key.
- `human-goat-pp-cli system dashboard-counts` — Dashboard tab counts (open tasks, messages, etc.)

**taskers** — Favorite, past, and suggested TaskRabbit Taskers

- `human-goat-pp-cli taskers favorites` — List your favorited Taskers (poster=client, rabbit=tasker)
- `human-goat-pp-cli taskers past` — List Taskers you have hired before
- `human-goat-pp-cli taskers suggestions` — Tasker suggestions for the account metro


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
human-goat-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Autonomous hire with a spend cap

```bash
human-goat-pp-cli hire "help moving" --on 2026-07-11 --min-rating 4.8 --max-total 200 --agent --lat 47.6062 --lng -122.3321
```

Searches, ranks by all-in price x reviews, checks out under the cap, and prints the booking id, Tasker, time, and total with no prompt.

### Undo a booking and verify

```bash
human-goat-pp-cli cancel task_abc123 --agent
```

Cancels, re-reads status, and reports cancelled plus whether it was inside the free window and any fee.

### Dispatch a remote errand

```bash
human-goat-pp-cli call 5209076052 "when does the jewelry store open"
```

Sends a phone-call task to Magic and returns a request id to track; the answer comes back in the conversation.

### Narrow a verbose payload for an agent

```bash
human-goat-pp-cli taskers favorites --agent --select items.name,items.all_in_rate,items.rating
```

Returns only the fields an agent needs from a deeply nested Tasker list instead of the full payload.

### Where did the money go

```bash
human-goat-pp-cli spend --by source --agent
```

Splits TaskRabbit vs Magic totals from the local store, with TaskRabbit rows using true all-in effective rate.

## Auth Setup

TaskRabbit auth is cookie replay from a logged-in Chrome session: run `human-goat-pp-cli auth login --chrome` to lift the session + XSRF-TOKEN cookies. Never a programmatic password login (the login form is reCAPTCHA-gated). Magic uses an x-api-key read from $MAGIC_API_KEY or ~/.magic/api_key.

Run `human-goat-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  human-goat-pp-cli account --agent --select id,name,status
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

- Use `--home <dir>` for one invocation, or set `HUMAN_GOAT_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `HUMAN_GOAT_CONFIG_DIR`, `HUMAN_GOAT_DATA_DIR`, `HUMAN_GOAT_STATE_DIR`, `HUMAN_GOAT_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `HUMAN_GOAT_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `human-goat-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "human-goat": {
        "command": "human-goat-pp-mcp",
        "env": {
          "HUMAN_GOAT_HOME": "/srv/human-goat"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `HUMAN_GOAT_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `HUMAN_GOAT_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
human-goat-pp-cli recall "<user's question>" --agent
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
      "next_action": ["<trial command>", "human-goat-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `human-goat-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `human-goat-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `human-goat-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
human-goat-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
human-goat-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
human-goat-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
human-goat-pp-cli playbook amend \
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

`human-goat-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `HUMAN_GOAT_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
human-goat-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
human-goat-pp-cli feedback --stdin < notes.txt
human-goat-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `HUMAN_GOAT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `HUMAN_GOAT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
human-goat-pp-cli profile save briefing --json
human-goat-pp-cli --profile briefing account
human-goat-pp-cli profile list --json
human-goat-pp-cli profile show briefing
human-goat-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `human-goat-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/human-goat/cmd/human-goat-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add human-goat-pp-mcp -- human-goat-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which human-goat-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   human-goat-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `human-goat-pp-cli <command> --help`.
