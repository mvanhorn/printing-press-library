---
name: pp-tabiwa
description: "Compare regional tabiwa catalog products with payment units and original restriction evidence. Trigger phrases: `compare tabiwa passes`, `tabiwa points-only ticket`, `tabiwa catalog restrictions`, `use tabiwa`, `run tabiwa-pp-cli`."
author: "zjsng"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - tabiwa-pp-cli
    install:
      - kind: go
        bins: [tabiwa-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/travel/tabiwa/cmd/tabiwa-pp-cli
---

# tabiwa by WESTER catalog — Printing Press CLI

## Catalog scope and evidence

Public catalog reads work without login. The only sent cookie is `regionId`, the website's display preference: `10` せとうち, `20` 北陸, `30` 山陰, `40` 九州. Full detail pages enter Queue-it for ordinary HTTP and are excluded from runtime access. The CLI never imports queue/account cookies, books, pays, retrieves purchased tickets, or redeems a ticket.

Use `geography list --region 20` for exact provider prefecture/area IDs; these differ from national prefecture codes. Use `catalog search` for a bounded shortlist, `catalog inspect ID` for the selected catalog summary and `catalog compare ID ID --on YYYY-MM-DD` for price units and requested-date membership. `catalog` and `geography` are bounded aliases of their search/list commands. Live catalog commands support `--data-source auto` or `live`; `catalog saved` supports `auto` or `local`.

`price.unit` separates `JPY`, `WESTER_POINT`, and `unknown`. `points_only` is the source boolean or null; a yen display does not establish that points cannot be used. Amounts remain decimal strings. A quote's passenger/unit basis is unknown unless the overview explicitly identifies it, such as a per-vehicle taxi price. The CLI does not rank incomparable quotes, convert points, or calculate pass savings.

Requested-date membership is a published catalog filter. `not_listed` does not mean sold out, closed, ineligible or unavailable. Real availability is always unknown. An explicit overview sold-out notice has its own `published_status`; generic limited-stock wording never sets it. Area tags do not prove included routes: J0000900 tags 直島 in its broader area while its overview excludes unlisted 直島. Preserve both facts.

`restriction_evidence` contains original Japanese overview lines with restriction/redemption cues, not a complete policy or route description. Complete redemption terms and included routes remain unknown. Read `overview_truncated` and `restriction_evidence_truncated` before relying on the excerpts. Canonical product links lead to the source's final terms; opening them may require its ordinary queue.

Source reads have an 8 MiB response/1000-record bound. Search scans at most `--max-records` (default 500, maximum 1000) independently from `--limit` (default 10, maximum 50). Coverage reports source/scanned/returned counts and truncation. A zero result applies only to those filters and scanned records. Inspect requests one ID; compare requests two to five distinct IDs and uses at most two source reads. Every observation retains source URL and UTC retrieval time; requested dates are Japan calendar dates.

Generic root `search` uses local FTS over synced geography in auto/local modes; explicit live is unsupported. Use `catalog search` for bounded regional products. Generic MCP search/SQL use geography from the separate framework store. Save-capable catalog MCP tools are local writes because `save=true` persists selected private evidence; provider requests remain GET-only. Geography and saved-evidence tools remain reads.

Live commands save only with `--save`; `--no-cache --save` is refused. Selected normalized evidence is stored in `catalog/saved.db` under the resolved cache directory, at most 50 `(region,id)` observations, 24 KiB per record. Only a strictly newer observation instant replaces that product's prior evidence; delayed or equal-clock saves keep the existing observation. SQLite saves use normal transactions. `catalog saved` opens only an existing read-only database and performs no migration or source read; missing storage returns an empty saved list without creating directories. Saved clocks are original source clocks, not current availability. SQL's generic framework store is separate from this selected-evidence store.

Queue redirects, HTML challenges, throttles, contract changes, and network failures are explicit errors. Date-comparison failure returns an error instead of a partly fabricated comparison. No automatic source-access workaround or browser runtime is used. A large MCP result may require fewer products or narrower `--select` fields.

## Prerequisites: Install the CLI

This skill drives the `tabiwa-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install tabiwa --cli-only
   ```
2. Verify: `tabiwa-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/tabiwa/cmd/tabiwa-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Inspect published catalog summaries and requested-date membership. Full ticket terms, route coverage and stock remain unknown; finish on the canonical product page.

## When to Use This CLI

Use for tabiwa regional transport/attraction bundles and coupons, points-only prices, overview restriction evidence and saved catalog shortlists.

## Anti-triggers

Do not use this CLI for:
- Do not use for bookings, payments, purchased tickets, rewards balances or redemption actions.
- Do not infer real availability, full redemption policies, included routes or savings from catalog labels.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Catalog decisions
- **`catalog search`** — Discover regional products with a dated catalog filter and explicit scanned coverage.

  _Discover regional products with a dated catalog filter and explicit scanned coverage._

  ```bash
  tabiwa-pp-cli catalog search --region 20 --category transportation --on 2026-10-28 --limit 3 --agent
  ```
- **`catalog compare`** — Compare points-only and yen-quoted products without inventing currency conversion.

  _Compare points-only and yen-quoted products without inventing currency conversion._

  ```bash
  tabiwa-pp-cli catalog compare J0001900 J0000900 --region 10 --agent
  ```
- **`catalog inspect`** — Read original Japanese restriction and redemption hints while complete terms remain unknown.

  _Read original Japanese restriction and redemption hints while complete terms remain unknown._

  ```bash
  tabiwa-pp-cli catalog inspect J0000900 --region 10 --agent
  ```
- **`catalog compare`** — Show whether selected products appear in a requested-date catalog without claiming ticket stock.

  _Show whether selected products appear in a requested-date catalog without claiming ticket stock._

  ```bash
  tabiwa-pp-cli catalog compare J0001900 J0000900 --region 10 --on 2026-10-28 --agent
  ```
- **`catalog saved`** — Read saved selected products offline with their original observation times.

  _Read saved selected products offline with their original observation times._

  ```bash
  tabiwa-pp-cli catalog saved --region 10 --limit 3 --agent
  ```

## Command Reference

**catalog** — Inspect published ticket catalog summaries and their limits

- `tabiwa-pp-cli catalog` — Discover catalog summaries with regional and date filters; not live stock

**geography** — Resolve provider region-specific prefecture and area identifiers

- `tabiwa-pp-cli geography` — List source prefecture and area IDs for a selected regional catalog


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
tabiwa-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query. `--json` (and other machine formats) keep that exit-2 contract and write `{"matches":[]}` on stdout so agents can inspect the envelope without treating a miss as success.

## Recipes

### Payment units

```bash
tabiwa-pp-cli catalog compare J0001900 J0000900 --region 10 --agent
```

Compare points-only and yen-quoted products without inventing currency conversion.

### Source restrictions

```bash
tabiwa-pp-cli catalog inspect J0000900 --region 10 --agent
```

Read original Japanese restriction and redemption hints while complete terms remain unknown.

### Compact discovery

```bash
tabiwa-pp-cli catalog search --region 20 --category transportation --limit 3 --agent --select products.id,products.name,products.price,coverage
```

Keep identity, unit-qualified quotes and scan coverage.

## Auth Setup

Public catalog reads need no login. The only sent cookie is the documented regionId display preference. Queue-it full detail pages are excluded from runtime access.

Run `tabiwa-pp-cli doctor` to verify setup.

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
  tabiwa-pp-cli catalog --agent --select id,name,price
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

- Use `--home <dir>` for one invocation, or set `TABIWA_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `TABIWA_CONFIG_DIR`, `TABIWA_DATA_DIR`, `TABIWA_STATE_DIR`, `TABIWA_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `TABIWA_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains `config.json` and profiles. `data` contains the generic framework `data.db`. `state` contains local runtime and learning files. `cache` contains regenerable HTTP/cache files and selected observations in `catalog/saved.db`.
- Run `tabiwa-pp-cli doctor --fail-on warn` to surface path warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "tabiwa": {
        "command": "tabiwa-pp-mcp",
        "env": {
          "TABIWA_HOME": "/srv/tabiwa"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `TABIWA_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `TABIWA_HOME`, or `doctor` will not find credentials left under the former root.

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
tabiwa-pp-cli recall "$QUERY" --agent
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
      "next_action": ["<trial command>", "tabiwa-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `tabiwa-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `tabiwa-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `tabiwa-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately. Pass the query the same way as recall — argv/MCP, or file-then-`$QUERY`. Do not splice the question into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
tabiwa-pp-cli teach --query "$QUERY" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
QUERY=$(cat /path/to/question.txt)
tabiwa-pp-cli teach \
  --query "$QUERY" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
QUERY=$(cat /path/to/question.txt)
tabiwa-pp-cli teach-playbook \
  --query "$QUERY" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`. Pass the query and note as argv/MCP arguments, or write each with a non-shell file tool and read them back (`QUERY=$(cat ...)`, `NOTE=$(cat ...)`). Do not interpolate either string into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
NOTE=$(cat /path/to/note.txt)
tabiwa-pp-cli playbook amend \
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

`tabiwa-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `TABIWA_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
tabiwa-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
tabiwa-pp-cli feedback --stdin < notes.txt
tabiwa-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `TABIWA_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `TABIWA_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
tabiwa-pp-cli profile save briefing --json
tabiwa-pp-cli --profile briefing catalog
tabiwa-pp-cli profile list --json
tabiwa-pp-cli profile show briefing
tabiwa-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `tabiwa-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/travel/tabiwa/cmd/tabiwa-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add tabiwa-pp-mcp -- tabiwa-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which tabiwa-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   tabiwa-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `tabiwa-pp-cli <command> --help`.

For MCP comparisons, pass the comma-separated `ids` string, for example `{"ids":"J0001900,J0000900","region":"10","on":"2026-10-28"}`. The CLI equivalent is `catalog compare --ids J0001900,J0000900 --region 10 --on 2026-10-28`.
