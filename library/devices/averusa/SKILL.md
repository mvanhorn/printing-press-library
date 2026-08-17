---
name: pp-averusa
description: "Every AVer USA manual, spec sheet, and white paper in a local, type-filtered catalog — with spec comparison and link audits no AVer site offers. Trigger phrases: `get the manual for a CAM570`, `spec sheet for an AVer conference camera`, `compare AVer models side by side`, `AVer white papers`, `use averusa`, `run averusa`."
author: "drummerms"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - averusa-pp-cli
    install:
      - kind: go
        bins: [averusa-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/devices/averusa/cmd/averusa-pp-cli
---

# AVer USA — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `averusa-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install averusa --cli-only
   ```
2. Verify: `averusa-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/averusa/cmd/averusa-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

averusa.com hides its manuals, spec sheets, and white papers behind a Salesforce portal maze and per-model PDFs. averusa-pp-cli syncs the whole catalog into a local database, then answers the questions integrators actually ask: which model fits (compare), what are its specs (specs), what docs exist per model (coverage, docs pack), what changed since last sync (whats-new), and which PDF links are dead (doctor).

## When to Use This CLI

Use averusa-pp-cli whenever an agent or integrator needs an AVer USA user manual, spec sheet/datasheet, or white paper — browsing the type-filtered catalog, comparing models for a bid, assembling an offline job bag, or auditing which PDF links are still alive. Its local store answers cross-document questions ('which model has a white paper and which is discontinued?') that the Salesforce portal cannot.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to control AVer PTZ or conference cameras — that is device control over VISCA/IP, which belongs to Bitfocus Companion or vendor device tools.
- Do not use it to download or install firmware — firmware binaries live on AVer's external download manager, not the portal's fileField, and firmware installation is a device operation.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local catalog that compounds
- **`compare`** — Side-by-side spec fields for two or more AVer models from their datasheets, ready for bid comparisons and RFP tables. Spec fields come from datasheet PDFs extracted by `harvest --with-specs`.

  _Use this instead of opening N datasheet PDFs to answer 'which model fits?' for a bid._

  ```bash
  averusa-pp-cli compare CAM570 CAM550 --agent
  ```
- **`specs`** — One model's full spec fields as clean text or JSON for spec-compliance tables and agent pipelines. Fields come from datasheet PDFs extracted by `harvest --with-specs`.

  _Use this to fill an RFP spec-compliance row without opening the PDF._

  ```bash
  averusa-pp-cli specs CAM570 --json --agent
  ```
- **`whats-new`** — List documents and products added or updated since the last sync, filterable by age.

  _Use this to track new manuals or firmware docs across a fleet without re-checking the website._

  ```bash
  averusa-pp-cli whats-new --since 30d --json
  ```
- **`coverage`** — Per-model doc-type availability matrix for a category, flagging which models are missing manuals, spec sheets, or white papers.

  _Use this before a recommendation or commissioning checklist to catch missing compliance docs._

  ```bash
  averusa-pp-cli coverage conference-camera
  ```

### Reachability mitigation
- **`docs audit`** — HEAD-checks every document URL in the catalog and flags 404s and soft-404 shells, caching last-checked status locally.

  _Use this before pushing docs to a shared drive to catch dead or mislinked PDFs while they are still cheap to fix._

  ```bash
  averusa-pp-cli docs audit
  ```

### Offline job-site workflow
- **`docs pack`** — Batch-download every document for a model into one offline folder with stable <model>-<type> names, with a --dry-run preview. Downloads need entityIds, which `harvest` resolves at sync time.

  _Use this to pre-stage a job bag of manuals before driving to a site with no signal._

  ```bash
  averusa-pp-cli docs pack CAM570 --out ./job-570 --dry-run
  ```

### Service-specific intelligence
- **`products status`** — Flag which models AVer lists as discontinued, filterable by category.

  _Use this before specing a model into a school bid so a discontinued unit never ships in the quote._

  ```bash
  averusa-pp-cli products status --category conference-camera --json
  ```

## Command Reference

**harvest** — Build the local corpus (run this first)

- `averusa-pp-cli harvest` — walk the support-portal article sitemap (737 articles), fetch each article's SSR page, resolve its Salesforce entityId, and scrape the product catalog + discontinued lists into the local corpus
- `averusa-pp-cli harvest --only docs --limit 20` — narrow to a docs subset
- `averusa-pp-cli harvest --only products --with-specs` — also extract spec fields from datasheet PDFs (requires pdftotext)

**docs** — AVer USA support-portal knowledge articles and attached files

- `averusa-pp-cli docs search` — Type-filtered full-text search over the harvested catalog (`--type user-manual|spec-sheet|white-paper|...`)
- `averusa-pp-cli docs list` — List the harvested catalog, filterable by `--type`/`--model` (live sitemap fallback when unharvested)
- `averusa-pp-cli docs get` — Fetch a knowledge article as clean text (crawler-UA SSR)
- `averusa-pp-cli docs download` — Download an article's attached file (PDF)
- `averusa-pp-cli docs audit` — HEAD-check every document URL; flag 404s and soft-404 shells
- `averusa-pp-cli docs pack` — Batch-download every document for a model into one offline folder

**products** — AVer USA product catalog and datasheets

- `averusa-pp-cli products list` — List the harvested catalog, filterable by `--category`
- `averusa-pp-cli products get` — Fetch a product page as clean text (spec links, downloads)
- `averusa-pp-cli products docs` — List every document harvested for a model, plus its datasheet
- `averusa-pp-cli products status` — Flag models AVer lists as discontinued, filterable by category

**novel commands** — `compare` (side-by-side datasheet specs), `specs` (one model's spec fields), `whats-new` (docs/products updated since last harvest), `coverage` (per-model doc-type matrix)


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
averusa-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Field kit before you drive out

```bash
averusa-pp-cli docs pack CAM570 --out ./job-570
```

One command assembles the whole offline doc bag for a model with stable names — no portal clicking at the jobsite.

### Bid spec table

```bash
averusa-pp-cli specs CAM550 --json --agent
```

Structured spec fields straight into an RFP compliance table or agent pipeline.

### Which model fits?

```bash
averusa-pp-cli compare CAM570 CAM550 --agent
```

Side-by-side datasheet specs so a recommendation never relies on opening two PDFs.

### Audit the shared drive

```bash
averusa-pp-cli docs audit
```

Catches dead and mislinked PDFs before installers do — the CAM570 page currently mislinks cam520pro3-datasheet.pdf.

### White papers for an eval

```bash
averusa-pp-cli docs search "white paper" --type white-paper --json --select title,doc_type,model
```

Evaluation material filtered to white papers, narrowed with --select so agents don't parse the full payload.

## Auth Setup

No authentication required.

Run `averusa-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields, e.g. `--select title,doc_type,model`. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  averusa-pp-cli docs list --agent --select title,doc_type,model
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

- Use `--home <dir>` for one invocation, or set `AVERUSA_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `AVERUSA_CONFIG_DIR`, `AVERUSA_DATA_DIR`, `AVERUSA_STATE_DIR`, `AVERUSA_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `AVERUSA_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains the corpus database (`data.db`) and harvested state. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- This CLI has no authentication: every AVer surface it reads is public, so no credential storage exists or is needed.
- Run `averusa-pp-cli doctor --fail-on warn` to surface path warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "averusa": {
        "command": "averusa-pp-mcp",
        "env": {
          "AVERUSA_HOME": "/srv/averusa"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `AVERUSA_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `AVERUSA_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
averusa-pp-cli recall "<user's question>" --agent
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
      "next_action": ["<trial command>", "averusa-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `averusa-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `averusa-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `averusa-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
averusa-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
averusa-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
averusa-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
averusa-pp-cli playbook amend \
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

`averusa-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `AVERUSA_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
averusa-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
averusa-pp-cli feedback --stdin < notes.txt
averusa-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `AVERUSA_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `AVERUSA_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
averusa-pp-cli profile save briefing --json
averusa-pp-cli --profile briefing docs list
averusa-pp-cli profile list --json
averusa-pp-cli profile show briefing
averusa-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `averusa-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/devices/averusa/cmd/averusa-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add averusa-pp-mcp -- averusa-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which averusa-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   averusa-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `averusa-pp-cli <command> --help`.
