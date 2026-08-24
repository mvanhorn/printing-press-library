---
name: pp-qsys
description: "Q-SYS product specs, configuration procedures, and connection guides in one local index - with equipment-list compatibility checks neither QSC website can do. Trigger phrases: `q-sys specs`, `qsys product specs`, `how do I configure this q-sys`, `how do I connect q-sys`, `is this supported on q-sys designer`, `q-sys compatibility`, `q-sys deprecated`, `use qsys`, `run qsys`."
author: "drummerms"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - qsys-pp-cli
    install:
      - kind: go
        bins: [qsys-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/devices/qsys/cmd/qsys-pp-cli
---

# Q-SYS — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `qsys-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install qsys --cli-only
   ```
2. Verify: `qsys-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/qsys/cmd/qsys-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Q-SYS documentation is split across two sites: qsys.com carries the product spec sheets as PDFs, and help.qsys.com carries the configuration and networking guidance. Neither one can tell you whether a list of equipment runs on a given Designer version. This CLI harvests both into local SQLite, joins them into one record per product, and answers spec, configuration, wiring, and compatibility questions offline - including from a job site with no usable network.

## When to Use This CLI

Use this CLI for questions about Q-SYS equipment: what a product's specifications are, how to configure a specific device, how to connect or network it, and whether a set of equipment is supported on a given Q-SYS Designer version. It is built for AV integrators and designers working from an equipment list, and it works offline once harvested, which matters on job sites with locked-down networks.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to control a live Q-SYS Core - it never connects to hardware; use a QRC client library for that
- Do not use it to look up control pin names for Lua or QRC scripting - it indexes equipment documentation, not the schematic control surface
- Do not treat extracted spec text as authoritative for engineering sign-off - always confirm against the source PDF URL it returns
- Do not use it for pricing, availability, or lead times - it indexes documentation only

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### One record per product
- **`product get`** — See a Q-SYS product's overview, spec-sheet text, configuration pages, and connection guidance in one record.

  _Reach for this first for any question about a specific model; it answers spec, config, and wiring questions in one call instead of three._

  ```bash
  qsys-pp-cli product get CX-Q --agent
  ```

### Design-time safety checks
- **`compat check`** — Check a whole equipment list against a Q-SYS Designer version and get back what is supported and what is not.

  _Use this before quoting or commissioning to catch an unsupported part while it is still cheap to swap._

  ```bash
  qsys-pp-cli compat check CX-Q TSC-70-G3 NL-C4 --qds 9.4 --agent
  ```
- **`compat deprecated`** — Flag which models in a list are deprecated or discontinued before they reach a quote.

  _Use this to sanity-check a parts list; an end-of-life part caught at design time costs nothing, caught at order time costs a redesign._

  ```bash
  qsys-pp-cli compat deprecated CX-Q CXD-Q --agent
  ```
- **`bom verify`** — One report per model in an equipment list: version support, EOL status, and spec-sheet availability in a single pass.

  _Use this for the complete pre-quote check on a parts list instead of three separate lookups per part._

  ```bash
  qsys-pp-cli bom verify CX-Q TSC-70-G3 NL-C4 --qds 9.4 --agent
  ```

### Field answers
- **`connect`** — Get the networking, wiring, and I/O guidance that actually applies to a given model.

  _Use this for how-do-I-wire-this-in questions instead of reading the whole networking section._

  ```bash
  qsys-pp-cli connect TSC-70-G3 --agent
  ```
- **`integrations`** — Find which UC platforms (Teams, Zoom, Meet) a device is certified or integrated with.

  _Use this when a room design must match the client's chosen UC platform._

  ```bash
  qsys-pp-cli integrations TSC-70-G3 --agent
  ```

### Version-aware reads
- **`page get`** — Read a help page as of a specific Q-SYS Designer version from the versioned doc tree.

  _Use this when commissioning a system that runs an older Designer than today's docs describe._

  ```bash
  qsys-pp-cli page get control_router --version 9.4 --agent
  ```

### Trust the local copy
- **`coverage`** — Report how many products resolved a spec sheet and how many pages parsed, so extraction gaps are visible.

  _Run this after a harvest; a silent drop in coverage means the vendor changed their HTML and results are now incomplete._

  ```bash
  qsys-pp-cli coverage --agent
  ```

## Command Reference

**harvest** — Build the local corpus (run this first)

- `qsys-pp-cli harvest` — walk both vendor sitemaps (roughly 750 help pages and 270 product pages, rate limited) and build the local corpus every other command reads
- `qsys-pp-cli harvest --only pages|products|compat` — harvest one source instead of all three
- `qsys-pp-cli harvest --only products --limit 25 --with-pdfs` — narrow the walk, and also download and text-extract spec-sheet PDFs (slower; needs `pdftotext`)

**Do not confuse `harvest` with `sync`.** Top-level `sync` walks the generated endpoint resources and refreshes entity lookups; it does not build the corpus. The Q-SYS corpus is two scraped websites plus a PDF layer that must be joined locally, and only `harvest` builds it. Run `qsys-pp-cli coverage` afterwards to confirm the harvest landed.

**compat** — Hardware and software compatibility matrices

- `qsys-pp-cli compat by-product` — List the Q-SYS Designer versions and compatibility notes for a hardware product, per the compatibility matrix
- `qsys-pp-cli compat by-version` — List hardware support by Q-SYS Designer version
- `qsys-pp-cli compat deprecations` — List deprecated hardware and feature notices with the release in which each item was deprecated
- `qsys-pp-cli compat upgrade-path` — Show firmware and Q-SYS Designer upgrade path requirements, including the supported upgrade sequences

**networking** — Connection, wiring, and network setup guidance

- `qsys-pp-cli networking <topic>` — Fetch a Q-SYS networking or connection guidance page

**page** — Q-SYS Help documentation pages (configuration, networking, hardware)

- `qsys-pp-cli page get` — Fetch a Q-SYS Help documentation page as clean text
- `qsys-pp-cli page index` — Fetch the Q-SYS Help sitemap listing every documentation page

**product** — Q-SYS product pages and spec sheets on qsys.com

- `qsys-pp-cli product index` — Fetch the qsys.com sitemap listing every product page
- `qsys-pp-cli product page` — Fetch a qsys.com product page as clean text
- `qsys-pp-cli product resources` — List spec-sheet and manual PDF links for a product


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
qsys-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### First run: build the corpus

```bash
qsys-pp-cli harvest
qsys-pp-cli coverage
```

`harvest` walks both vendor sitemaps into the local corpus; every other command reads it. `coverage` then reports how many products resolved a spec sheet and how many pages parsed, so an incomplete harvest is visible instead of silent. Narrow a first pass with `--only products --limit 25`, and add `--with-pdfs` when spec-sheet text is needed.

### Check a whole BOM against a Designer version

```bash
qsys-pp-cli bom verify --qds 9.4 --agent < bom.txt
```

Reads an equipment list from a file on stdin and returns a per-model report: version support, EOL status, and spec-sheet availability.

### Narrow a verbose product record for an agent

```bash
qsys-pp-cli product get CX-Q --agent --select model,family,spec_pdf_url,discontinued
```

Product records carry full spec-sheet text; --select trims the payload to just the fields needed so an agent does not burn context on prose.

### Read docs as an older Designer version saw them

```bash
qsys-pp-cli page get control_router --version 9.4
```

A site commissioned on 9.4 reads the 9.4 tree instead of silently getting today's 10.x behavior.

### Get wiring guidance for a touchscreen

```bash
qsys-pp-cli connect TSC-70-G3
```

Resolves the model to its family and returns only the networking and wiring pages that apply to it.

### Verify the local copy is complete

```bash
qsys-pp-cli coverage --agent
```

Reports spec-sheet match rate and page parse rate so a silent extraction regression is visible.

## Auth Setup

No authentication required.

Run `qsys-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  qsys-pp-cli networking mock-value --agent --select id,name,status
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

- Use `--home <dir>` for one invocation, or set `QSYS_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `QSYS_CONFIG_DIR`, `QSYS_DATA_DIR`, `QSYS_STATE_DIR`, `QSYS_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `QSYS_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `qsys-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "qsys": {
        "command": "qsys-pp-mcp",
        "env": {
          "QSYS_HOME": "/srv/qsys"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `QSYS_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `QSYS_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
qsys-pp-cli recall "<user's question>" --agent
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
      "next_action": ["<trial command>", "qsys-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `qsys-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `qsys-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `qsys-pp-cli sync` to refresh entity lookups. Note that top-level `sync` only walks the generated endpoint resources and refreshes lookups — it does not build the corpus. If local reads are also coming back empty, run `qsys-pp-cli harvest` first; that is the corpus builder.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
qsys-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
qsys-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
qsys-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
qsys-pp-cli playbook amend \
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

`qsys-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `QSYS_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
qsys-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
qsys-pp-cli feedback --stdin < notes.txt
qsys-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `QSYS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `QSYS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
qsys-pp-cli profile save briefing --json
qsys-pp-cli --profile briefing networking mock-value
qsys-pp-cli profile list --json
qsys-pp-cli profile show briefing
qsys-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `qsys-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/devices/qsys/cmd/qsys-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add qsys-pp-mcp -- qsys-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which qsys-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   qsys-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `qsys-pp-cli <command> --help`.
