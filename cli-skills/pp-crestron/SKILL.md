---
name: pp-crestron
description: "Every Crestron product, spec sheet, and firmware release — searchable offline, with fleet-wide currency checks the website cannot do. Trigger phrases: `what firmware is current for DM-NVX`, `find the spec sheet for a Crestron model`, `is this Crestron part discontinued`, `what changed in the latest Crestron firmware`, `compare two Crestron models`, `build a Crestron submittal package`, `use crestron`, `run crestron`."
author: "drummerms"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - crestron-pp-cli
    install:
      - kind: go
        bins: [crestron-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/devices/crestron/cmd/crestron-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/devices/crestron/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Crestron — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `crestron-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install crestron --cli-only
   ```
2. Verify: `crestron-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/crestron/cmd/crestron-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Crestron.com has no public API and no way to ask a question across more than one product at a time. This CLI mirrors the product catalog, the documentation library, and the firmware release history into local SQLite, then answers the questions integrators actually have: what firmware covers my installed models, what changed between two versions, which parts are discontinued and what replaced them. Sign in and it also unlocks release notes and firmware downloads.

## When to Use This CLI

Use this CLI for any question about Crestron hardware that lives on Crestron.com rather than on a device: what a product is, what its specifications are, which firmware version is current, what changed in a release, which documentation exists for a model, and whether a part has been discontinued. It is especially strong for questions spanning many models at once, which the website cannot answer at all.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to control, configure, or communicate with Crestron hardware on a network — it never talks to devices. Use Crestron Toolbox or the Crestron EDK PowerShell modules for that.
- Do not use this CLI to push or install firmware onto a control processor or touch panel; it only downloads the files.
- Do not use this CLI for Crestron Home smart-home control such as lights, shades, or scenes.
- Do not use this CLI to obtain dealer pricing — Crestron's public pricing endpoint returns no data for most models.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Fleet lifecycle intelligence
- **`fleet status`** — Check every model in your installed fleet against current firmware in one command.

  _Reach for this instead of checking models one at a time; it also catches releases that a per-model search would miss because the release is titled under a sibling model._

  ```bash
  crestron-pp-cli fleet status --file fleet.txt --agent
  ```
- **`lifecycle`** — Report whether a model is still sellable and trace its replacement chain.

  _Reach for this when triaging an as-built list to find which parts can still be ordered and what replaced the rest._

  ```bash
  crestron-pp-cli lifecycle UC-FCM-Z --agent
  ```

### Firmware knowledge base
- **`search`** — Search every firmware release note and change log at once for a term.

  _Use this to answer 'which version fixed X' without opening a dozen version pages._

  ```bash
  crestron-pp-cli search "HDCP" --type firmware_release --agent
  ```
- **`firmware diff`** — Show what changed between two firmware versions for a model.

  _Pick this when deciding whether an upgrade is worth scheduling on a live site._

  ```bash
  crestron-pp-cli firmware diff DM-NVX-384 7.3.5149.23092 7.4.0255.22319 --agent
  ```

### Design and submittal workflow
- **`submittal`** — Download every documentation asset for a list of models into per-model folders with a coverage report.

  _Use this to assemble a CSI submittal package in one step instead of hundreds of individual downloads._

  ```bash
  crestron-pp-cli submittal DM-NVX-384 --agent
  ```
- **`specs compare`** — Compare two models field by field across the full specification table.

  _Use this when choosing between sibling models in the same series, which often differ in only a few spec rows._

  ```bash
  crestron-pp-cli specs compare DM-NVX-360 DM-NVX-363 --agent
  ```

## Command Reference

**sync** — Build the local mirror (run this first)

- `crestron-pp-cli sync` — crawl Crestron.com into a local SQLite mirror: catalog categories, products, and firmware releases with their covered-model lists expanded
- `crestron-pp-cli sync --resources categories,products` — build just the catalog half
- `crestron-pp-cli sync --resources releases --notes --max-notes 50` — also pull release notes and change logs so `search` can full-text them (needs a signed-in session; without one the version and date are still recorded)
- `crestron-pp-cli sync --max-duration 1h --concurrency 3` — bound the whole crawl and the number of parallel category walks

Crestron publishes no API and no product sitemap, so the catalog is walked the way the website itself does. Nothing local — `search`, `specs`, `fleet`, `lifecycle`, `submittal` — has data until this runs. The root `--timeout` applies per request, not to the whole sync; use `--max-duration` (default 30m) for the overall budget.

**account** — Crestron.com sign-in state

- `crestron-pp-cli account` — Check whether the stored Crestron.com session is still signed in

**asset** — Download Crestron documentation and firmware files

- `crestron-pp-cli asset <guid> <filename>` — Download a public documentation asset such as a spec sheet, manual, certificate, CAD drawing, or Revit family

**catalog** — Browse the Crestron product catalog taxonomy

- `crestron-pp-cli catalog category` — Open a catalog category page and read its subcategories and product counts
- `crestron-pp-cli catalog products` — List the products in a catalog category (needs the category's document and node ids)
- `crestron-pp-cli catalog tree` — List every product category path in the catalog

**firmware** — Crestron firmware and software releases

- `crestron-pp-cli firmware release` — Read a firmware release page including its version, date, release notes, and change log (requires sign-in)
- `crestron-pp-cli firmware search` — Find firmware and software releases for a model or family

**product** — Look up Crestron products, specifications, and their documentation

- `crestron-pp-cli product accessories` — List optional accessories for a product
- `crestron-pp-cli product page` — Fetch a product detail page including its JSON-LD, specification table, and document id
- `crestron-pp-cli product replacements` — List replacement products for a discontinued item
- `crestron-pp-cli product resources` — List every documentation asset for a product by its document id
- `crestron-pp-cli product variants` — List the member models of a product series

**resource** — Search Crestron's documentation and firmware resource library

- `crestron-pp-cli resource` — Search spec sheets, manuals, firmware, certificates, and drawings


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
crestron-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### First run: build the local mirror

```bash
crestron-pp-cli sync
crestron-pp-cli product get DM-NVX-360
```

`sync` crawls Crestron.com into the local mirror; `search`, `specs`, `fleet`, `lifecycle`, and `submittal` all read it and return nothing until it has run. Add `--notes` with a signed-in session to make release notes and change logs full-text searchable. Bound a long crawl with `--max-duration` — the root `--timeout` applies per request, not to the whole sync.

### Audit a whole fleet for firmware currency

```bash
crestron-pp-cli fleet status --file fleet.txt --agent --select model,installed,latest,days_behind
```

Reads a plain list of model numbers and reports which are behind, resolving family-scoped releases so nothing is missed.

### Find which firmware version fixed something

```bash
crestron-pp-cli search "Dante" --type firmware_release --limit 10 --agent
```

Full-text searches every synced release note and change log at once.

### Narrow a verbose spec table to the fields you care about

```bash
crestron-pp-cli specs show DM-NVX-360 --agent --select sections.name,sections.rows.key,sections.rows.value
```

Specification tables run to dozens of rows across a dozen sections; selecting dotted paths keeps agent context small.

### Assemble a submittal package for a project

```bash
crestron-pp-cli submittal DM-NVX-384 TSW-1070 CP4N --out ./submittal --agent
```

Downloads every documentation asset for each model into its own folder and reports which asset classes were missing.

### Triage an as-built list for discontinued parts

```bash
crestron-pp-cli lifecycle UC-FCM-Z --agent --select model,status,replaced_by
```

Reports sellable status and the successor chain so a refresh estimate can be priced.

## Auth Setup

Most of this CLI works with no account at all: the product catalog, specifications, spec sheets, manuals, certificates, CAD and Revit files, and firmware version numbers and release dates are all public. A Crestron account unlocks two more things — firmware release notes and the firmware binaries themselves. Run `crestron-pp-cli auth login --chrome` and the CLI imports your existing Crestron.com session cookies straight from Chrome; it never asks for or stores your password.

Run `crestron-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  crestron-pp-cli asset mock-value mock-value --agent --select id,name,status
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

- Use `--home <dir>` for one invocation, or set `CRESTRON_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `CRESTRON_CONFIG_DIR`, `CRESTRON_DATA_DIR`, `CRESTRON_STATE_DIR`, `CRESTRON_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `CRESTRON_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `crestron-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "crestron": {
        "command": "crestron-pp-mcp",
        "env": {
          "CRESTRON_HOME": "/srv/crestron"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `CRESTRON_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `CRESTRON_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
crestron-pp-cli recall "<user's question>" --agent
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
      "next_action": ["<trial command>", "crestron-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `crestron-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `crestron-pp-cli learnings candidates` lists the full open set.

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
crestron-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
crestron-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
crestron-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
crestron-pp-cli playbook amend \
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

`crestron-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `CRESTRON_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
crestron-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
crestron-pp-cli feedback --stdin < notes.txt
crestron-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `CRESTRON_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `CRESTRON_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
crestron-pp-cli profile save briefing --json
crestron-pp-cli --profile briefing asset mock-value mock-value
crestron-pp-cli profile list --json
crestron-pp-cli profile show briefing
crestron-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `crestron-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/devices/crestron/cmd/crestron-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add crestron-pp-mcp -- crestron-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which crestron-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   crestron-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `crestron-pp-cli <command> --help`.
