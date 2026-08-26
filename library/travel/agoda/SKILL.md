---
name: pp-agoda
description: "Search Agoda hotels with the true all-in price, and re-rank by what you will actually pay. Trigger phrases: `find hotels in Tokyo on agoda`, `what will this agoda hotel actually cost`, `cheapest dates to stay in Bangkok`, `which agoda hotel is cheapest all in`, `check agoda prices for these dates`, `use agoda`, `run agoda`."
author: "Victor Wibisono"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - agoda-pp-cli
    install:
      - kind: go
        bins: [agoda-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/travel/agoda/cmd/agoda-pp-cli
---

# Agoda — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `agoda-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install agoda --cli-only
   ```
2. Verify: `agoda-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/agoda/cmd/agoda-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Agoda returns both the advertised price and the true all-in price in the same response, but only ever shows you the advertised one. This CLI surfaces both, breaks out the hidden tax-and-fee delta, and re-sorts by real cost - which routinely changes which hotel is cheapest. It talks to Agoda over plain HTTP with no browser, no rendering service, and no API key, and it keeps a local price history so it can answer questions a stateless scraper cannot.

## When to Use This CLI

Use this CLI when an agent needs real Agoda hotel prices, particularly when the question involves cost. It is the right tool for judging what a stay actually costs, comparing finalists, sweeping flexible dates for a price floor, and tracking price drops over time. Agoda's inventory is strongest in Asia-Pacific, so it is often the better source than a Booking.com or Google Hotels tool for destinations in that region.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to complete a booking or take payment - it surfaces prices and deep links, and the reservation itself happens on Agoda.
- Do not use it for flights, activities, airport transfers, or car rental; it is scoped to hotels only.
- Do not use it to compare Agoda against other travel sites - it reads Agoda only, so a cross-OTA question needs a different tool.
- Do not use it to cancel, modify, or refund an existing reservation.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Honest pricing
- **`hotels search`** — Shows what you will actually pay, not the teaser rate, with the hidden tax-and-fee delta broken out per property.

  _Reach for this instead of any scraped Agoda price. Quote the inclusive figure to a user; the advertised rate is not what they will be charged._

  ```bash
  agoda-pp-cli hotels search Tokyo --checkin 2026-10-15 --nights 2 --adults 2 --currency USD --agent
  ```
- **`hotels rank`** — Re-sorts a destination's results by true all-in price instead of Agoda's teaser-price ranking.

  _Use this whenever the decision is about price. Ordinary search ordering inherits Agoda's advertised-price ranking and will mislead._

  ```bash
  agoda-pp-cli hotels rank Tokyo --checkin 2026-10-15 --nights 2 --limit 10 --agent
  ```
- **`hotels fees`** — Flags properties whose tax-and-fee ratio is an outlier against the destination median.

  _Use before recommending a property. A hotel with a below-median advertised price and an above-median fee ratio is the classic bait pattern._

  ```bash
  agoda-pp-cli hotels fees Tokyo --checkin 2026-10-15 --nights 2 --agent
  ```

### Local state that compounds
- **`prices cheapest`** — Returns the cheapest check-in dates across a flexible window for a destination.

  _Use for flexible-date travelers. Returns the price floor across a window rather than a single-date quote._

  ```bash
  agoda-pp-cli prices cheapest Tokyo --window 2026-10-01..2026-11-30 --nights 3 --agent
  ```
- **`vip delta`** — Runs the same search signed-in and anonymous, then diffs per property to show what your VIP tier is actually worth.

  _Use when a user asks whether signing in or chasing a VIP tier is worth it. Reports the measured discount on a real search instead of marketing copy._

  ```bash
  agoda-pp-cli vip delta Tokyo --checkin 2026-10-15 --nights 2 --agent
  ```
- **`watch run`** — Surfaces only watched properties whose latest true all-in price dropped meaningfully below their trailing median.

  _Schedule it. Returns empty most days and returns something worth acting on when a watched property actually drops._

  ```bash
  agoda-pp-cli watch run --min-pct 7 --agent
  ```
- **`search`** — Full-text search over every property this CLI has already seen, with no network call.

  _Use after a few live searches to answer property questions without spending a request or waiting on the network._

  ```bash
  agoda-pp-cli search "shinjuku" --agent
  ```

### Agent-native plumbing
- **`compare`** — Puts finalist properties side by side on true all-in price, hidden fee share, review score, star rating, and free-cancellation deadline.

  _Use once the choice is narrowed to finalists, instead of re-reading two detail pages and eyeballing the difference._

  ```bash
  agoda-pp-cli compare 936623 788273 --destination Tokyo --checkin 2026-10-15 --nights 2 --agent
  ```

## Command Reference

**destinations** — Resolve a destination name to the numeric city id every Agoda search requires

- `agoda-pp-cli destinations` — Resolve a free-text destination (city, area, landmark) to an Agoda city id

**reviews** — Guest reviews for a property, paginated and sortable

- `agoda-pp-cli reviews` — List guest reviews for a property by Agoda hotel id


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
agoda-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### What will this actually cost

```bash
agoda-pp-cli hotels search Tokyo --checkin 2026-10-15 --nights 2 --adults 2 --currency USD --agent --select results.name,results.price_all_in,results.price_advertised,results.hidden_pct
```

Returns just the four fields that matter for a cost decision, keeping the deeply nested Agoda payload out of the agent's context.

### The cheapest hotel is not the one listed cheapest

```bash
agoda-pp-cli hotels rank Tokyo --checkin 2026-10-15 --nights 2 --limit 10 --agent
```

Re-sorts by all-in cost; properties with above-average fee loads drop down the list and genuinely cheaper stays surface.

### Find the price floor for a flexible trip

```bash
agoda-pp-cli prices cheapest Tokyo --window 2026-10-01..2026-11-30 --nights 3 --agent
```

Sweeps a two-month window in one pass and returns the cheapest check-in dates rather than a single-date quote.

### Spot the resort-fee trap

```bash
agoda-pp-cli hotels fees Tokyo --checkin 2026-10-15 --nights 2 --agent
```

Ranks properties by how much of their true cost is tax and fees, flagging outliers against the destination median.

### Is signing in worth anything here

```bash
agoda-pp-cli vip delta Tokyo --checkin 2026-10-15 --nights 2 --agent
```

Issues the same search authenticated and anonymous and reports the measured per-property discount.

## Auth Setup

Public hotel search, destination lookup, property detail, and reviews need no credentials at all - they replay over ordinary HTTP. Only member-priced and account surfaces (saved properties, AgodaVIP tier, vip delta) need a logged-in session: copy the Cookie header from a signed-in agoda.com browser tab and export it as AGODA_COOKIE (AGODA_SESSION_COOKIE is also accepted), and subsequent authenticated calls replay with it.

Run `agoda-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  agoda-pp-cli reviews --agent --select hotelReviewId,rating,reviewComments
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `AGODA_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `AGODA_CONFIG_DIR`, `AGODA_DATA_DIR`, `AGODA_STATE_DIR`, `AGODA_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `AGODA_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `agoda-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "agoda": {
        "command": "agoda-pp-mcp",
        "env": {
          "AGODA_HOME": "/srv/agoda"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `AGODA_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `AGODA_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
agoda-pp-cli recall "<user's question>" --agent
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
      "next_action": ["<trial command>", "agoda-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `agoda-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `agoda-pp-cli learnings candidates` lists the full open set.

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
agoda-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
agoda-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
agoda-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
agoda-pp-cli playbook amend \
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

`agoda-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `AGODA_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
agoda-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
agoda-pp-cli feedback --stdin < notes.txt
agoda-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `AGODA_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `AGODA_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
agoda-pp-cli profile save briefing --json
agoda-pp-cli --profile briefing reviews
agoda-pp-cli profile list --json
agoda-pp-cli profile show briefing
agoda-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `agoda-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/travel/agoda/cmd/agoda-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add agoda-pp-mcp -- agoda-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which agoda-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   agoda-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `agoda-pp-cli <command> --help`.
