---
name: pp-name-that-ui
description: "Turn vague UI language into exact, source-backed component and style guidance that coding agents can use directly. Trigger phrases: `what is this UI component called`, `identify this interface pattern`, `compare select and combobox`, `translate this to AppKit or SwiftUI`, `find the name of this visual style`, `use NameThatUI`, `run NameThatUI`."
author: "HenryBranchAdams"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - name-that-ui-pp-cli
    install:
      - kind: go
        bins: [name-that-ui-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/developer-tools/name-that-ui/cmd/name-that-ui-pp-cli
---

# NameThatUI — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `name-that-ui-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install name-that-ui --cli-only
   ```
2. Verify: `name-that-ui-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/name-that-ui/cmd/name-that-ui-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Identify components and visual styles from colloquial descriptions, retrieve canonical anatomy and platform symbols, and translate macOS terminology without authentication. Local sync powers compact context packs, terminology linting, project inventories, and change-impact checks while every answer preserves its NameThatUI source link.

## When to Use This CLI

Use this CLI when an agent or engineer needs to name a UI component or visual style, retrieve exact anatomy and implementation terminology, compare similar patterns, or translate between plain language and platform APIs. Sync first when the task needs deterministic offline search, project scanning, or catalog history.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to classify screenshots or perform pixel-level visual inspection.
- Do not use this CLI to install third-party component code or choose project dependencies.
- Do not treat its general references as the authority for a private design system when project-specific documentation exists.
- Do not use it as a substitute for runtime accessibility testing or visual QA.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Agent-ready guidance
- **`context-pack`** — Assemble one bounded, source-backed implementation packet for a component, optional style, and target framework.

  _Use this when an agent needs enough precise context to implement or repair a UI without hopping across reference pages._

  ```bash
  name-that-ui-pp-cli context-pack --component web/combobox --style glassmorphism --framework web --agent --select component.name,parts,apis,style_signals,cautions,source_urls
  ```
- **`crosswalk`** — See one UI concept across plain language, component parts, AppKit, SwiftUI, and ARIA or HTML terminology.

  _Use this when product language must become exact framework terminology without relying on a model's memory._

  ```bash
  name-that-ui-pp-cli crosswalk "menu bar extra" --agent
  ```

### Project-aware reference
- **`lint`** — Find colloquial or ambiguous UI terms in prose and return canonical, source-backed candidates without rewriting the file.

  _Use this before coding when tickets, prompts, or design specs may use names that send an agent toward the wrong component._

  ```bash
  name-that-ui-pp-cli lint ./README.md --agent
  ```
- **`inventory`** — Map UI API symbols in a source tree to canonical components and source references.

  _Use this to ground a UI repair or design-system review in the components that actually appear in the project._

  ```bash
  name-that-ui-pp-cli inventory . --agent --select files.path,files.matches.symbol,files.matches.component,files.matches.source_url
  ```

### Guidance over time
- **`impact`** — Show which project files may be affected by component or style guidance changes since a prior snapshot.

  _Use this after syncing updates to focus review on code whose source-backed guidance actually changed._

  ```bash
  name-that-ui-pp-cli impact . --since 2026-07-13 --agent
  ```

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 17 API entries from 221 total network entries
- Protocols: rest_json (75% confidence)
- Generation hints: browser_http_transport, requires_protected_client, weak_schema_confidence
- Caveats: empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.

## Command Reference

**catalog** — Read the public NameThatUI catalog

- `name-that-ui-pp-cli catalog get` — Fetch a canonical UI element page
- `name-that-ui-pp-cli catalog list` — List UI element pages from the public structured catalog

**reference** — Read cross-framework and update references

Reference commands fetch their public data directly; `reference` is not a `sync --resources` value.

- `name-that-ui-pp-cli reference feed` — Fetch recent NameThatUI catalog additions
- `name-that-ui-pp-cli reference sitemap` — Fetch the public catalog sitemap
- `name-that-ui-pp-cli reference translate` — Fetch the AppKit-to-SwiftUI translation table

**translate** — Look up published plain-language, AppKit, and SwiftUI mappings

- `name-that-ui-pp-cli translate <term> [--from appkit|swiftui|plain|any] [--to appkit|swiftui|plain|all] [--limit 10]`

**updates** — Merge public RSS and sitemap updates

- `name-that-ui-pp-cli updates [--since duration-or-YYYY-MM-DD] [--limit 25] [--kind feed|sitemap|all]`

**semantic-search** — Use NameThatUI's public semantic search and reranker

- `name-that-ui-pp-cli semantic-search` — Rank candidate UI elements or styles for a colloquial description

**styles** — Read the public visual-style atlas

- `name-that-ui-pp-cli styles get` — Fetch a visual-style guidance page
- `name-that-ui-pp-cli styles list` — List visual-style pages

**style** — Read only the synced visual-style mirror (`sync --resources styles`)

- `name-that-ui-pp-cli style identify <description>` — Rank styles using upstream name, signal, and section evidence
- `name-that-ui-pp-cli style list` — List synced styles by name
- `name-that-ui-pp-cli style get <slug-or-name>` — Get a full synced style record
- `name-that-ui-pp-cli style signals <slug-or-name>` — Get upstream signals
- `name-that-ui-pp-cli style compare <left> <right>` — Compare full records and source overlap
- `name-that-ui-pp-cli style code <slug-or-name>` — Get only upstream code or implementation sections
- `name-that-ui-pp-cli style cautions <slug-or-name>` — Get only upstream accessibility or caution sections


## Freshness Contract

This printed CLI owns bounded freshness only for registered store-backed read command paths. In `--data-source auto` mode, those paths check `sync_state` and may run a bounded refresh before reading local data. `--data-source local` never refreshes. `--data-source live` reads the API and does not mutate the local store. Set `NAME_THAT_UI_NO_AUTO_REFRESH=1` to skip the freshness hook without changing source selection.

Covered paths:

- `name-that-ui-pp-cli catalog`
- `name-that-ui-pp-cli catalog get`
- `name-that-ui-pp-cli catalog list`
- `name-that-ui-pp-cli context-pack`
- `name-that-ui-pp-cli crosswalk`
- `name-that-ui-pp-cli impact`
- `name-that-ui-pp-cli inventory`
- `name-that-ui-pp-cli lint`
- `name-that-ui-pp-cli styles`
- `name-that-ui-pp-cli styles get`
- `name-that-ui-pp-cli styles list`

When JSON output uses the generated provenance envelope, freshness metadata appears at `meta.freshness`. Treat it as current-cache freshness for the covered command path, not a guarantee of complete historical backfill or API-specific enrichment.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
name-that-ui-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Name an unfamiliar component part

```bash
name-that-ui-pp-cli identify "pale pill behind the menu bar icon" --agent --select results.name,results.part,results.score,results.source_url
```

Resolve colloquial language while keeping ambiguity and provenance visible.

### Choose between similar patterns

```bash
name-that-ui-pp-cli component compare select combobox --agent
```

Retrieve concise source-backed differences before committing to an interaction pattern.

### Translate a macOS concept

```bash
name-that-ui-pp-cli crosswalk "menu bar extra" --agent
```

Join product language to AppKit and SwiftUI terminology.

### Audit terminology in a design spec

```bash
name-that-ui-pp-cli lint ./README.md --agent
```

Find imprecise UI names without performing generative rewriting.

### Review guidance changes against a project

```bash
name-that-ui-pp-cli impact . --since 2026-07-13 --agent
```

Focus review on source files connected to changed catalog records.

## Auth Setup

NameThatUI is public and requires no account, token, cookies, or browser session. The CLI uses anonymous standard HTTP for live lookups and stores only public reference data locally.

Run `name-that-ui-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  name-that-ui-pp-cli catalog list --agent --select id,name,status
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

- Use `--home <dir>` for one invocation, or set `NAME_THAT_UI_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `NAME_THAT_UI_CONFIG_DIR`, `NAME_THAT_UI_DATA_DIR`, `NAME_THAT_UI_STATE_DIR`, `NAME_THAT_UI_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `NAME_THAT_UI_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains `config.toml` and saved profiles. `data` contains `data.db` and public reference mirrors. `state` contains `teach.log` and local learning state. `cache` contains regenerable cache files.
- `doctor` reports public connectivity plus local paths and cache state. `agent-context` exposes a schema v5 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "name-that-ui": {
        "command": "name-that-ui-pp-mcp",
        "env": {
          "NAME_THAT_UI_HOME": "/srv/name-that-ui"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `NAME_THAT_UI_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Move local artifacts before clearing `NAME_THAT_UI_HOME` so `doctor` can continue to locate the configured paths.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
name-that-ui-pp-cli recall "<user's question>" --agent
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
      "next_action": ["<trial command>", "name-that-ui-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `name-that-ui-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `name-that-ui-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `name-that-ui-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
name-that-ui-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
name-that-ui-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
name-that-ui-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
name-that-ui-pp-cli playbook amend \
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

`name-that-ui-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `NAME_THAT_UI_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
name-that-ui-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
name-that-ui-pp-cli feedback --stdin < notes.txt
name-that-ui-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `NAME_THAT_UI_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `NAME_THAT_UI_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
name-that-ui-pp-cli profile save briefing --json
name-that-ui-pp-cli --profile briefing catalog list
name-that-ui-pp-cli profile list --json
name-that-ui-pp-cli profile show briefing
name-that-ui-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `name-that-ui-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/developer-tools/name-that-ui/cmd/name-that-ui-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add name-that-ui-pp-mcp -- name-that-ui-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which name-that-ui-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   name-that-ui-pp-cli catalog list --agent
   ```
4. If ambiguous, drill into subcommand help: `name-that-ui-pp-cli catalog list --help`.
