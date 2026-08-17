---
name: pp-printgoat
description: "Search and reliably download 3D-print files across Printables and Thingiverse (plus search Cults3D) — with real resumable downloads and cross-site duplicate detection nothing else has. Trigger phrases: `find me a 3D model for`, `search thingiverse`, `search printables`, `is this STL free anywhere else`, `download this 3d print file`, `3d print file search`, `use printgoat`, `run printgoat`."
author: "Nate Olson"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - printgoat-pp-cli
---

# PrintGoat — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `printgoat-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install printgoat --cli-only
   ```
2. Verify: `printgoat-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

PrintGoat unifies three separate 3D-model communities into one searchable, scriptable tool instead of three browser tabs. It's the only tool that actually resumes interrupted downloads via HTTP byte-range requests rather than just skipping files that already exist by name, and the only one that tells you when the same design is free on one site and paid on another before you download the wrong copy.

## When to Use This CLI

Use PrintGoat when searching for a 3D-printable design across multiple communities at once, when you need to know if a paid model has a free equivalent elsewhere, when managing a library of downloaded STL/3MF/G-code files, or when running a batch download job that needs to survive interruption.

## Anti-triggers

Do not use this CLI for:
- Do not use this for slicing STL/3MF files into G-code — it has no slicer integration.
- Do not use this to control a printer or print farm — no OctoPrint/Klipper integration exists.
- Do not use this to upload, publish, or cross-post your own designs to these sites — it is a discovery/download tool only.
- Do not expect Cults3D file downloads — that source is search/metadata only by the platform's own design.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Cross-site intelligence
- **`duplicates`** — See instantly when the same design is listed free on one site and paid on another, before you download the wrong one.

  _Reach for this before downloading anything from a paid Cults3D listing — it tells you if the same file exists free elsewhere._

  ```bash
  printgoat-pp-cli duplicates "gopro mount" --agent
  ```
- **`feed`** — Follow a designer across all the sites they post to and see new uploads from any of them in one feed.

  _Check this instead of manually revisiting three separate designer profile pages to see what's new._

  ```bash
  printgoat-pp-cli feed --agent
  ```
- **`formats gaps`** — Find alternatives when a model only ships in one file format and you need another.

  _Use when a search result is STL-only and you need 3MF or STEP — this looks for tag-similar alternatives that include the missing format._

  ```bash
  printgoat-pp-cli formats gaps printables:3161 --agent
  ```

### Provenance, resume & integrity
- **`history diff`** — Know exactly what changed on a model since you last looked — new files, price changes, license changes.

  _Use this instead of re-fetching a model's full detail when you only need to know what's different since last time._

  ```bash
  printgoat-pp-cli history diff printables:3161 --agent
  ```
- **`license audit`** — Flag license conflicts across your entire local library against how you actually intend to use the files.

  _Run this before a commercial print job to catch license conflicts the site's own badge won't warn you about._

  ```bash
  printgoat-pp-cli license audit --library --intent commercial --agent
  ```
- **`job download`** — Batch-download across all three sites as one crash-safe job that actually resumes from where it stopped.

  _Use this for any multi-file or multi-source batch pull; it survives crashes and rate-limit backoff without restarting completed files._

  ```bash
  printgoat-pp-cli job resume job-20260721-01 --agent
  ```
- **`snapshot verify`** — Prove exactly which file version you used for a past print job, even if the upstream model has since changed.

  _Use this for provenance/reproducibility on any job whose output needs to match a prior run exactly._

  ```bash
  printgoat-pp-cli snapshot verify batch-march-orders --agent
  ```
- **`library doctor`** — Find orphaned files, missing files, silent duplicates, and remote listings that have been delisted (and can never be re-fetched).

  _Run periodically to catch link rot before a delisted source becomes unrecoverable if the local copy is ever lost._

  ```bash
  printgoat-pp-cli library doctor --agent
  ```

### Print outcome memory
- **`similar`** — Re-search for alternatives after a bad print, automatically excluding the model and designer that failed you.

  _Use after logging a failed print (`log fail`) instead of re-running a generic search that will surface the same bad result again._

  ```bash
  printgoat-pp-cli similar printables:3161 --agent
  ```
- **`designer stats`** — See your own success/failure rate with a specific designer, pooled across every site they publish on.

  _Check this before downloading a new model from a designer you've printed before, when site-level popularity counts don't reflect your own experience._

  ```bash
  printgoat-pp-cli designer stats "PrintedSolid" --agent
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**categories** — Thingiverse categories

- `printgoat-pp-cli categories list` — List top-level categories
- `printgoat-pp-cli categories things` — List things in a category

**collections** — Thingiverse collections

- `printgoat-pp-cli collections get` — Get a collection by ID
- `printgoat-pp-cli collections things` — List things in a collection

**creations** — Cults3D creations (designs)

- `printgoat-pp-cli creations get` — Get a Cults3D creation's detail
- `printgoat-pp-cli creations search` — Search Cults3D creations by keyword

**models** — Printables 3D models (prints)

- `printgoat-pp-cli models <id>` — Get a Printables model's detail and file listing

**search_things** — Search Thingiverse things

- `printgoat-pp-cli search-things <term>` — Search for things by keyword

**tags** — Thingiverse tags

- `printgoat-pp-cli tags <tag>` — List things with a tag

**things** — Thingiverse things (3D models)

- `printgoat-pp-cli things categories` — List categories for a thing
- `printgoat-pp-cli things files` — List files for a thing
- `printgoat-pp-cli things get` — Get a thing by ID
- `printgoat-pp-cli things images` — List images for a thing
- `printgoat-pp-cli things tags` — List tags for a thing

**users** — Thingiverse users

- `printgoat-pp-cli users collections` — List a user's collections
- `printgoat-pp-cli users get` — Get a user's public profile
- `printgoat-pp-cli users things` — List a user's published things


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
printgoat-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Unified search with narrowed output

```bash
printgoat-pp-cli search "cable clip" --agent --select results.name,results.source,results.price,results.license
```

Search results are deeply nested per-source objects; --select trims the response to just the fields an agent needs for a quick comparison.

### Find the cheapest legitimate copy of a design

```bash
printgoat-pp-cli duplicates "phone stand" --agent
```

Surfaces the same design across sources with price and license side by side before you commit to a paid download.

### Resume an interrupted batch job

```bash
printgoat-pp-cli job resume job-20260721-01 --agent
```

Continues a multi-source download job from its last completed byte offset per file instead of restarting.

### Audit your library before a commercial print run

```bash
printgoat-pp-cli license audit --library --intent commercial --agent
```

Flags any locally downloaded file whose license doesn't permit the commercial use you've declared.

## Auth Setup

Each source has a different auth story: Printables needs nothing at all (its GraphQL backend works fully anonymously for search and download). Thingiverse requires a free self-registered OAuth2 app token (`auth set-token --source thingiverse`). Cults3D requires your account handle plus a self-service API key via HTTP Basic Auth (`auth set-token --source cults3d`) — and by Cults3D's own design, its API can search and browse but never download other users' files, so Cults3D results are search/metadata only.

Run `printgoat-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  printgoat-pp-cli categories list --agent --select id,name,status
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

- Use `--home <dir>` for one invocation, or set `PRINTGOAT_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `PRINTGOAT_CONFIG_DIR`, `PRINTGOAT_DATA_DIR`, `PRINTGOAT_STATE_DIR`, `PRINTGOAT_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `PRINTGOAT_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml`, profiles, and local download preferences (`download_dir`, `default_formats` via `config set`/`config get`). `data` contains `data.db` (the local SQLite store — downloads ledger, snapshots, print outcomes, followed designers, jobs). `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- `THINGIVERSE_TOKEN`, `CULTS3D_USERNAME`, and `CULTS3D_API_KEY` always come from the environment and are never read from or written to any file — there is no credential file or auth command in this CLI.
- Run `printgoat-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "printgoat": {
        "command": "printgoat-pp-mcp",
        "env": {
          "PRINTGOAT_HOME": "/srv/printgoat"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `PRINTGOAT_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `PRINTGOAT_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
printgoat-pp-cli recall "<user's question>" --agent
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
      "next_action": ["<trial command>", "printgoat-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `printgoat-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `printgoat-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `printgoat-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
printgoat-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
printgoat-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
printgoat-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
printgoat-pp-cli playbook amend \
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

`printgoat-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `PRINTGOAT_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
printgoat-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
printgoat-pp-cli feedback --stdin < notes.txt
printgoat-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `PRINTGOAT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `PRINTGOAT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
printgoat-pp-cli profile save briefing --json
printgoat-pp-cli --profile briefing categories list
printgoat-pp-cli profile list --json
printgoat-pp-cli profile show briefing
printgoat-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Auth error (missing/invalid credentials) |
| 5 | API error (upstream issue) |
| 6 | Partial failure (use `--allow-partial-failure` to downgrade to a warning) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `printgoat-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add printgoat-pp-mcp -- printgoat-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which printgoat-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   printgoat-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `printgoat-pp-cli <command> --help`.
