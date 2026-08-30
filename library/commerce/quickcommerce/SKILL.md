---
name: pp-quickcommerce
description: "Compare live Indian product prices, stock, and delivery ETAs from the terminal, then keep the history locally. Trigger phrases: `compare milk prices across BlinkIt and Zepto`, `find the fastest delivery near Bangalore`, `check QuickCommerce API credits`, `show price history for item 501346`, `use QuickCommerce API`, `run QuickCommerce CLI`."
author: "Som Samantray"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - quickcommerce-pp-cli
    install:
      - kind: go
        bins: [quickcommerce-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/commerce/quickcommerce/cmd/quickcommerce-pp-cli
---

# QuickCommerce API — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `quickcommerce-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install quickcommerce --cli-only
   ```
2. Verify: `quickcommerce-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/quickcommerce/cmd/quickcommerce-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

QuickCommerce CLI covers the provider's REST and hosted-MCP surface while adding a local mirror for repeatable analysis. Use history, fastest delivery, credit planning, stale-data checks, and unit-price views to turn location-sensitive responses into decisions.

## When to Use This CLI

Use this CLI for location-aware product search, price and stock comparisons, delivery ETA decisions, and repeatable local analysis across Indian commerce platforms. It is especially useful when an agent needs bounded JSON, credit-aware fan-out, and historical context instead of one ephemeral API response.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to place orders, manage carts, or perform checkout; the API exposes read-only product and delivery data.
- Do not use the local mirror as proof of current availability without checking its age; refresh paid observations deliberately.
- Do not use this CLI for platforms or locations outside the documented QuickCommerce API coverage.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`history prices`** — Query local observations to see price, stock, rating, and availability movement over time.

  _Choose this when an agent needs to explain what changed instead of fetching only the current value._

  ```bash
  quickcommerce-pp-cli history prices --item 501346 --since 30d --agent
  ```
- **`history diff`** — Show the field-level changes between the latest saved observations.

  _Choose this after ingestion when an agent needs a precise price, stock, or ETA delta._

  ```bash
  quickcommerce-pp-cli history diff --item 501346 --latest 2 --agent
  ```
- **`mirror ingest`** — Persist real QuickCommerce command responses and metadata into the local SQLite mirror.

  _Choose this when an agent wants future history, offline search, or a reproducible observation record._

  ```bash
  quickcommerce-pp-cli mirror ingest --stdin --agent
  ```

### Decision-ready comparisons
- **`delivery fastest`** — Rank currently available delivery options while preserving closed or unparseable platforms.

  _Choose this when an agent must recommend a viable fastest platform rather than list raw ETA rows._

  ```bash
  quickcommerce-pp-cli delivery fastest --location 12.9021,77.6639 --agent
  ```
- **`mirror coverage`** — Report observed, missing, and stale platform coverage for a location.

  _Choose this when an agent needs to know whether a location has usable evidence across platforms._

  ```bash
  quickcommerce-pp-cli mirror coverage --location 12.9021,77.6639 --agent
  ```
- **`prices value`** — Compare price per unit from explicit pack quantities without guessing missing units.

  _Choose this when the cheapest sticker price may not be the cheapest comparable quantity._

  ```bash
  quickcommerce-pp-cli prices value --query milk --location 12.9021,77.6639 --agent
  ```

### Agent-native safety
- **`requests plan`** — Calculate fan-out credit cost and affordability before making paid platform requests.

  _Choose this before an agent launches a multi-platform search or ETA fan-out._

  ```bash
  quickcommerce-pp-cli requests plan --platforms blinkit,zepto --operation search --agent
  ```
- **`mirror stale`** — Find saved product and ETA observations that are older than a chosen trust window.

  _Choose this before making decisions from local data whose location-sensitive freshness matters._

  ```bash
  quickcommerce-pp-cli mirror stale --max-age 24h --agent
  ```

## Command Reference

**account** — Inspect API credit balance and packs.

- `quickcommerce-pp-cli account` — Check credit balance, usage, and expiry.

**comparison** — Compare products across multiple platforms.

- `quickcommerce-pp-cli comparison` — Search and compare products across multiple platforms.

**delivery** — Check delivery timing and store availability.

- `quickcommerce-pp-cli delivery compare` — Compare delivery ETAs across quick-commerce platforms.
- `quickcommerce-pp-cli delivery eta` — Get delivery ETA and store availability for one platform.

**items** — Fetch current details for a platform item.

- `quickcommerce-pp-cli items` — Get live price, stock, and availability for an item ID.

**platforms** — Discover platform support and ETA scope.

- `quickcommerce-pp-cli platforms` — List platforms supported by each endpoint.

**products** — Search products by keyword, location, and platform.

- `quickcommerce-pp-cli products` — Search products on one platform by keyword and location.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
quickcommerce-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Compare milk prices

```bash
quickcommerce-pp-cli comparison --query milk --platforms BlinkIt,Zepto,Swiggy --latitude 12.9021 --longitude 77.6639 --agent --select results
```

Ask for only the cross-platform result map instead of the full response envelope.

### Plan a paid fan-out

```bash
quickcommerce-pp-cli requests plan --platforms blinkit,zepto,swiggy --operation search --json
```

Check the expected credit cost before making a multi-platform search.

### Rank delivery speed

```bash
quickcommerce-pp-cli delivery fastest --location 12.9021,77.6639 --agent
```

Rank viable open platforms while retaining unavailable rows for explanation.

### Find stale observations

```bash
quickcommerce-pp-cli mirror stale --max-age 24h --json
```

Detect local data that needs a deliberate refresh.

### Inspect an item trend

```bash
quickcommerce-pp-cli history prices --item 501346 --since 30d --agent
```

Review saved price, inventory, and availability movement for one item. 

## Auth Setup

Set QUICKCOMMERCE_API_KEY to the API key from the QuickCommerce dashboard. The CLI sends it as X-API-Key and keeps paid calls explicit; platform discovery is available without a key.

Run `quickcommerce-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  quickcommerce-pp-cli comparison --query milk --platforms BlinkIt,Zepto --latitude 12.9021 --longitude 77.6639 --agent --select query,platforms,lat
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

- Use `--home <dir>` for one invocation, or set `QUICKCOMMERCE_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `QUICKCOMMERCE_CONFIG_DIR`, `QUICKCOMMERCE_DATA_DIR`, `QUICKCOMMERCE_STATE_DIR`, `QUICKCOMMERCE_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `QUICKCOMMERCE_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `quickcommerce-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "quickcommerce": {
        "command": "quickcommerce-pp-mcp",
        "env": {
          "QUICKCOMMERCE_HOME": "/srv/quickcommerce"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `QUICKCOMMERCE_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `QUICKCOMMERCE_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
# Agent input must already be structuralized and identifier-free. Do not
# read from stdin: agent shells are noninteractive and a user question can
# contain names, emails, phone numbers, or account identifiers.
#
# Capture the value with your Write tool, not a shell heredoc: a heredoc's
# terminator is matched by exact line content, so if the value itself
# contains a line equal to the delimiter, the heredoc closes early and
# whatever follows that line runs as literal shell commands. Writing the
# value to a file with a non-shell tool sidesteps shell parsing of it
# entirely -- there's no delimiter to spoof, and reading it back with
# `read` from file redirection has no terminator-matching step to game.
# 1. Write tool -> a scratch file, containing exactly:
#    compare product prices across platforms
STRUCTURAL_QUERY_FILE=$(mktemp)
# (Write tool call targets $STRUCTURAL_QUERY_FILE here)
IFS= read -r STRUCTURAL_QUERY < "$STRUCTURAL_QUERY_FILE"
rm -f "$STRUCTURAL_QUERY_FILE"
quickcommerce-pp-cli recall "$STRUCTURAL_QUERY" --agent
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
      "next_action": ["<trial command>", "quickcommerce-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `quickcommerce-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `quickcommerce-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `quickcommerce-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
# Pass only an identifier-free structural query held by the agent. Never copy
# a raw user question into the learning store. Capture it with your Write
# tool, not a shell heredoc: a heredoc's terminator is matched by exact line
# content, so a value containing a line equal to the delimiter closes it
# early and runs whatever follows as literal shell commands. Writing the
# value to a file with a non-shell tool sidesteps shell parsing entirely.
# 1. Write tool -> a scratch file, containing exactly:
#    compare product prices across platforms
STRUCTURAL_QUERY_FILE=$(mktemp)
# (Write tool call targets $STRUCTURAL_QUERY_FILE here)
IFS= read -r STRUCTURAL_QUERY < "$STRUCTURAL_QUERY_FILE"
rm -f "$STRUCTURAL_QUERY_FILE"
quickcommerce-pp-cli teach --query "$STRUCTURAL_QUERY" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Use a sanitized structural query supplied by the agent; this example is
# intentionally noninteractive and contains no user-identifying context.
# Capture it with your Write tool, not a shell heredoc -- see the recall
# recipe above for why: a heredoc's terminator is exact-line-matched and a
# value colliding with the delimiter runs whatever follows as real commands.
# 1. Write tool -> a scratch file, containing exactly:
#    compare product prices across platforms
STRUCTURAL_QUERY_FILE=$(mktemp)
# (Write tool call targets $STRUCTURAL_QUERY_FILE here)
IFS= read -r STRUCTURAL_QUERY < "$STRUCTURAL_QUERY_FILE"
rm -f "$STRUCTURAL_QUERY_FILE"

# Common case: record both the resource learning AND the playbook in one call.
quickcommerce-pp-cli teach \
  --query "$STRUCTURAL_QUERY" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
quickcommerce-pp-cli teach-playbook \
  --query "$STRUCTURAL_QUERY" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
# Both values must be structuralized before assignment: remove names, emails,
# phone numbers, account IDs, personal paths, and user-specific query history.
# Capture both with your Write tool, not shell heredocs -- see the recall
# recipe above for why: a heredoc's terminator is exact-line-matched and a
# value colliding with the delimiter runs whatever follows as real commands.
# 1. Write tool -> a scratch file, containing exactly:
#    compare product prices across platforms
STRUCTURAL_QUERY_FILE=$(mktemp)
# (Write tool call targets $STRUCTURAL_QUERY_FILE here)
IFS= read -r STRUCTURAL_QUERY < "$STRUCTURAL_QUERY_FILE"
rm -f "$STRUCTURAL_QUERY_FILE"
# 2. Write tool -> a second scratch file, containing exactly:
#    API response wraps items under a nested data.results field
STRUCTURAL_NOTE_FILE=$(mktemp)
# (Write tool call targets $STRUCTURAL_NOTE_FILE here)
IFS= read -r STRUCTURAL_NOTE < "$STRUCTURAL_NOTE_FILE"
rm -f "$STRUCTURAL_NOTE_FILE"
quickcommerce-pp-cli playbook amend \
  --query "$STRUCTURAL_QUERY" \
  --add-note "$STRUCTURAL_NOTE"
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

`quickcommerce-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `QUICKCOMMERCE_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
quickcommerce-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
quickcommerce-pp-cli feedback --stdin < notes.txt
quickcommerce-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `QUICKCOMMERCE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `QUICKCOMMERCE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

```
quickcommerce-pp-cli profile save briefing --json
quickcommerce-pp-cli --profile briefing comparison --query milk --platforms BlinkIt,Zepto --latitude 12.9021 --longitude 77.6639
quickcommerce-pp-cli profile list --json
quickcommerce-pp-cli profile show briefing
quickcommerce-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `quickcommerce-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/commerce/quickcommerce/cmd/quickcommerce-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add quickcommerce-pp-mcp -- quickcommerce-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which quickcommerce-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   quickcommerce-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `quickcommerce-pp-cli <command> --help`.
