---
name: pp-zillow
description: "Printing Press CLI for Zillow. Sync official Zillow Research housing datasets and analyze markets without scraping Zillow.com."
author: "Hunter Veltri"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - zillow-pp-cli
    install:
      - kind: go
        bins: [zillow-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/zillow/cmd/zillow-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/other/zillow/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Zillow — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `zillow-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install zillow --cli-only
   ```
2. Verify: `zillow-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/zillow/cmd/zillow-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Sync public Zillow Research time series, compare regions, and build sourced affordability, supply, demand, negotiation, and relocation analysis. Core commands never scrape Zillow.com; optional Bridge requests remain separately authorized and uncached.

## High-value workflows

Use `--agent` for JSON, compact output, and non-interactive behavior.

```bash
# Resolve names before multi-market comparisons
zillow-pp-cli region resolve "Austin, TX" --agent

# One-region snapshot and client brief
zillow-pp-cli summary "Austin, TX" --agent
zillow-pp-cli client-brief "Austin, TX" --income 120000 --format markdown

# Explainable compound analysis
zillow-pp-cli negotiation "Austin, TX" --agent
zillow-pp-cli buy-vs-rent "Austin, TX" --rate 6.5 --years 7 --agent
zillow-pp-cli explain --agent -- negotiation

# Offline SQLite workflow
zillow-pp-cli sync market --agent
zillow-pp-cli sql "SELECT metric, COUNT(*) AS rows FROM observations GROUP BY metric" --agent
```

Core commands use 17 public Zillow Research CSV datasets and require no authentication. They never scrape Zillow.com or automate consumer pages.

Bridge is separate and optional. Use only with an approved `BRIDGE_ACCESS_TOKEN`:

```bash
zillow-pp-cli bridge status --agent
zillow-pp-cli bridge request --dataset <approved-dataset-id> --resource Property --top 10 --agent
```

Bridge responses are never cached. Never put its token in command arguments, URLs, notes, or output.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Client decisions
- **`affordability gap`** — Compare household income with Zillow's regional homeowner-income-needed estimate.

  _Use when an agent must turn a generic affordability estimate into a client-specific answer._

  ```bash
  zillow-pp-cli affordability gap "Austin, TX" --income 120000 --agent
  ```
- **`yield-proxy`** — Join ZORI and ZHVI on a common date for a transparent gross rent-to-value proxy.

  _Use for quick, explainable rent-versus-value comparisons without pretending the proxy is net yield._

  ```bash
  zillow-pp-cli yield-proxy "Austin, TX" --agent
  ```
- **`buy-vs-rent`** — Run a visible-assumption ownership-versus-rent cash-flow scenario.

  _Use when an agent needs an inspectable scenario rather than a black-box recommendation._

  ```bash
  zillow-pp-cli buy-vs-rent "Austin, TX" --rate 6.5 --years 7 --agent
  ```
- **`negotiation`** — Combine price cuts, sale-to-list ratio, days pending, and inventory into an explainable score.

  _Use for buyer preparation when every score component must be shown._

  ```bash
  zillow-pp-cli negotiation "Austin, TX" --agent
  ```

### Market structure
- **`supply-ratio`** — Join inventory and sales nowcast into an approximate months-of-inventory ratio.

  _Use when an agent needs a compact supply-balance signal with its approximation stated._

  ```bash
  zillow-pp-cli supply-ratio "Austin, TX" --agent
  ```
- **`turning-points`** — Find dated slope sign reversals across temperature, inventory, days pending, and ZHVI.

  _Use when an agent must identify when a market changed direction, not merely its current level._

  ```bash
  zillow-pp-cli turning-points "Austin, TX" --months 24 --agent
  ```
- **`tier-spread`** — Compare bottom-, middle-, and top-tier ZHVI growth.

  _Use when affordability tiers may be moving differently from the headline market._

  ```bash
  zillow-pp-cli tier-spread "Austin, TX" --agent
  ```
- **`demand-pressure`** — Combine rental demand and for-sale market momentum with component-level evidence.

  _Use when an agent must compare rental and ownership demand in one sourced view._

  ```bash
  zillow-pp-cli demand-pressure "Austin, TX" --agent
  ```
- **`new-build-gap`** — Compare new-construction pricing and sales activity with typical regional home value.

  _Use when an agent needs to quantify the new-build premium and activity context._

  ```bash
  zillow-pp-cli new-build-gap "Austin, TX" --agent
  ```

### Regional screening
- **`shortlist`** — Rank regions with explicit user weights and visible min-max-normalized components.

  _Use for relocation or market-screening work where ranking criteria must remain auditable._

  ```bash
  zillow-pp-cli shortlist --regions "394355,394530" --weight zhvi=-0.4 --weight zori=0.6 --agent
  ```
- **`breadth`** — Measure rising, falling, and unchanged regions by state or geography type.

  _Use when an agent needs to tell whether a trend is broad or isolated._

  ```bash
  zillow-pp-cli breadth --months 12 --group-by state --agent -- zhvi
  ```

### Evidence quality
- **`quality audit`** — Detect missing observations, duplicate regions, coverage gaps, and large monthly jumps.

  _Use before analysis when freshness, gaps, or anomalous changes could invalidate a conclusion._

  ```bash
  zillow-pp-cli quality audit --jump-threshold 20 --agent -- zhvi
  ```
- **`explain`** — Show formulas, datasets, freshness behavior, and caveats for compound commands.

  _Use before relying on a compound score or scenario in consequential work._

  ```bash
  zillow-pp-cli explain --agent -- negotiation
  ```

### Agent delivery
- **`client-brief`** — Compose deterministic Markdown or JSON briefs from sourced regional metrics.

  _Use when an agent must hand a human a concise brief without inventing narrative facts._

  ```bash
  zillow-pp-cli client-brief "Austin, TX" --income 120000 --format markdown
  ```

## Recipes

### Buyer negotiation brief

```bash
zillow-pp-cli negotiation "Austin, TX" --agent
```

Shows leverage score, every component, source dates, and caveats.

### Buy-versus-rent scenario

```bash
zillow-pp-cli buy-vs-rent "Austin, TX" --rate 6.5 --years 7 --agent
```

Compares cash flow using visible financing, growth, tax, insurance, maintenance, and transaction assumptions.

### Regional shortlist

```bash
zillow-pp-cli shortlist --regions "394355,394530" --weight zhvi=-0.4 --weight zori=0.6 --agent
```

Ranks candidate markets while exposing normalized components and user-supplied weights.

### Compact sourced snapshot

```bash
zillow-pp-cli summary "Austin, TX" --agent --select results.region,results.metrics,results.evidence
```

Keeps only the region, core metrics, and evidence fields for low-context agent use.

### Offline SQL

```bash
zillow-pp-cli sql "SELECT metric, COUNT(*) AS rows FROM observations GROUP BY metric" --agent
```

Queries normalized observations locally after sync with a read-only SQL gate.

## Command Reference

**research** — Official Zillow Research regional time-series downloads.

- `zillow-pp-cli research days-pending` — Download monthly metro mean days-to-pending observations.
- `zillow-pp-cli research homeowner-income` — Download monthly metro income-needed estimates for a typical home purchase with 20 percent down.
- `zillow-pp-cli research inventory` — Download monthly metro for-sale inventory observations.
- `zillow-pp-cli research market-temperature` — Download monthly metro market-temperature index observations.
- `zillow-pp-cli research new-con-price` — Download monthly metro new-construction median sale prices.
- `zillow-pp-cli research new-con-price-per-sqft` — Download monthly metro new-construction median sale price per square foot.
- `zillow-pp-cli research new-con-sales` — Download monthly metro new-construction sales counts.
- `zillow-pp-cli research price-cut-share` — Download monthly metro share of listings with a price cut.
- `zillow-pp-cli research sale-to-list` — Download monthly metro mean sale-to-list ratio observations.
- `zillow-pp-cli research sales` — Download monthly metro sales-count nowcast observations.
- `zillow-pp-cli research total-monthly-payment` — Download monthly metro total housing payment estimates for a typical home purchase with 20 percent down.
- `zillow-pp-cli research zhvf` — Download monthly metro Zillow Home Value Forecast growth observations.
- `zillow-pp-cli research zhvi` — Download monthly metro Zillow Home Value Index observations.
- `zillow-pp-cli research zhvi-bottom-tier` — Download monthly metro bottom-tier Zillow Home Value Index observations.
- `zillow-pp-cli research zhvi-top-tier` — Download monthly metro top-tier Zillow Home Value Index observations.
- `zillow-pp-cli research zordi` — Download monthly metro Zillow Observed Renter Demand Index observations.
- `zillow-pp-cli research zori` — Download monthly metro Zillow Observed Rent Index observations.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
zillow-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Public Zillow Research commands require no authentication. Bridge access is optional and requires an approved BRIDGE_ACCESS_TOKEN; the CLI sends it only as bearer authorization and never caches Bridge responses.

Run `zillow-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  zillow-pp-cli research days-pending --agent --select id,name,status
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

- Use `--home <dir>` for one invocation, or set `ZILLOW_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `ZILLOW_CONFIG_DIR`, `ZILLOW_DATA_DIR`, `ZILLOW_STATE_DIR`, `ZILLOW_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `ZILLOW_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `zillow-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "zillow": {
        "command": "zillow-pp-mcp",
        "env": {
          "ZILLOW_HOME": "/srv/zillow"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `ZILLOW_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `ZILLOW_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
zillow-pp-cli recall "<user's question>" --agent
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
      "next_action": ["<trial command>", "zillow-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `zillow-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `zillow-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `zillow-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
zillow-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
zillow-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
zillow-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
zillow-pp-cli playbook amend \
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

`zillow-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `ZILLOW_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
zillow-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
zillow-pp-cli feedback --stdin < notes.txt
zillow-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `ZILLOW_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ZILLOW_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
zillow-pp-cli profile save briefing --json
zillow-pp-cli --profile briefing research days-pending
zillow-pp-cli profile list --json
zillow-pp-cli profile show briefing
zillow-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `zillow-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/zillow/cmd/zillow-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add zillow-pp-mcp -- zillow-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which zillow-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   zillow-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `zillow-pp-cli <command> --help`.
