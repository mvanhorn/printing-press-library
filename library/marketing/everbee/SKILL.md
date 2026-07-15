---
name: pp-everbee
description: "Etsy niche research whose scores come with their evidence attached — seeded search, honest confidence, and a local store that turns repeat research into trends. Trigger phrases: `research the dad shirt niche on Etsy`, `is this Etsy niche low competition`, `find low-competition Etsy sub-niches under dad`, `what tags do winning Etsy listings use for this niche`, `size up the competitors in this Etsy niche`, `use everbee`, `run everbee`."
author: "horknfbr"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - everbee-pp-cli
    install:
      - kind: go
        bins: [everbee-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/marketing/everbee/cmd/everbee-pp-cli
---

# EverBee — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `everbee-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install everbee --cli-only
   ```
2. Verify: `everbee-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/everbee/cmd/everbee-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

EverBee's data is the best Etsy research signal available, but its API will happily answer a question you did not ask: the default suggestion feeds return unranked filler regardless of your seed. This CLI queries the endpoints EverBee's own search boxes call, then stamps every returned row with a relevance score, an evidence count, and provenance. Confidence tracks evidence coverage, so a niche with no keyword support cannot come back looking confident. Use 'research niche' for a defensible verdict on one seed, 'research subniches' to rank a whole family of them, and 'selftest' to prove the data path is semantically sound before you trust a batch run.

## When to Use This CLI

Reach for this CLI when an Etsy seller or research agent needs to decide whether a niche is worth entering, and needs to be able to defend that decision. It is the right tool for seeded keyword and product research, for ranking sub-niches under a parent theme, for sizing up the competitors already in a niche, and for tracking how a niche moves week over week. It is especially suited to agent workflows because every result carries its evidence count, provenance, and an honest confidence, and because 'selftest' lets an agent verify the data path is semantically valid before trusting a batch of results.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to list or edit a seller's own Etsy listings — it is read-only Etsy market research and has no write path to Etsy.
- Do not use it for EverBee Store (the commerce/storefront product); that is a separate official API and this CLI does not cover it.
- Do not treat its revenue, sales, or search-volume figures as Etsy ground truth — they are EverBee estimates and should be reported as research signals.
- Do not use the unranked browse feeds ('products' and 'keyword-research list') to answer a question about a specific niche; they ignore your seed. Use 'research niche' or 'products search'.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Evidence-aware research
- **`research niche`** — Score an Etsy niche from a seed keyword and get the evidence behind the score, not just the number.

  _Reach for this instead of a raw keyword call when you need to defend a low-competition claim: every verdict carries its evidence count, provenance, and an honest confidence._

  ```bash
  everbee-pp-cli research niche "dad shirt" --agent
  ```
- **`research subniches`** — Expand a parent niche into child niches and rank them on comparable, normalized scores.

  _Use this when the task is 'find me the least-crowded corner of X' rather than 'tell me about X'._

  ```bash
  everbee-pp-cli research subniches --parent dad --product apparel --exclude-svg-png --agent
  ```
- **`research competitors`** — Get the market shape of a niche: result count, median price, review and sales density, listing-age quartiles.

  _Answers 'who would I be competing against, and how entrenched are they' before any design work starts._

  ```bash
  everbee-pp-cli research competitors "dad shirt" --agent
  ```
- **`research tags`** — See which tags and title tokens the winning listings in a niche agree on, and whether demand is seasonal or evergreen when EverBee supplies trend data.

  _Use before writing a listing: it gives you the consensus vocabulary of the niche. The seasonality verdict is reported as 'unknown' when EverBee returns no trend data, which is common — it never guesses._

  ```bash
  everbee-pp-cli research tags "dad shirt" --agent
  ```

### Local state that compounds
- **`research drift`** — Compare a niche against a saved baseline to see what actually moved since last time.

  _Turns repeated research into a trend instead of a series of disconnected screenshots._

  ```bash
  everbee-pp-cli research drift "dad shirt" --agent
  ```
- **`research listing`** — Resolve an Etsy listing URL or ID to what we actually know about it, and say so plainly when we know nothing.

  _Distinguishes 'this listing does not exist' from 'we have no data on it yet' — the two failures an agent must never conflate._

  ```bash
  everbee-pp-cli research listing 4515173344 --agent
  ```

### Agent-native plumbing
- **`selftest`** — Check that the research path is not just reachable but actually returning relevant data.

  _Run this first in any automated session: it is the difference between 'the API answered' and 'the answer means something'._

  ```bash
  everbee-pp-cli selftest --agent
  ```

## Command Reference

**account** — EverBee account plan and research quota

- `everbee-pp-cli account` — Show the EverBee account's current plan, research quota, and usage.

**keyword_research** — Etsy keyword research — volume, competition, score, CPC, and trend

- `everbee-pp-cli keyword-research list` — Browse EverBee's default keyword feed (what the UI shows before you search).
- `everbee-pp-cli keyword-research search` — Seeded keyword search.

**products** — Etsy product/listing research — sales, revenue, tags, price, listing type, and age

- `everbee-pp-cli products` — Browse EverBee's default product feed (what the UI shows before you search).

**shops** — Etsy competitor shop research — revenue, sales, listing counts, conversion, and reviews

- `everbee-pp-cli shops resolve` — Resolve an Etsy shop handle to its EverBee identity (shop_id, exact shop name, rating, review count, year created).
- `everbee-pp-cli shops search` — Search EverBee's Etsy shop database.


## Freshness Contract

This printed CLI owns bounded freshness only for registered store-backed read command paths. In `--data-source auto` mode, those paths check `sync_state` and may run a bounded refresh before reading local data. `--data-source local` never refreshes. `--data-source live` reads the API and does not mutate the local store. Set `EVERBEE_NO_AUTO_REFRESH=1` to skip the freshness hook without changing source selection.

Covered paths:

- `everbee-pp-cli keyword-research`
- `everbee-pp-cli keyword-research list`
- `everbee-pp-cli keyword-research search`
- `everbee-pp-cli products`
- `everbee-pp-cli products search`
- `everbee-pp-cli products search`
- `everbee-pp-cli shops`
- `everbee-pp-cli shops search`
- `everbee-pp-cli shops search`

When JSON output uses the generated provenance envelope, freshness metadata appears at `meta.freshness`. Treat it as current-cache freshness for the covered command path, not a guarantee of complete historical backfill or API-specific enrichment.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
everbee-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Defend a low-competition claim

```bash
everbee-pp-cli research niche "dad shirt" --agent
```

Returns demand, competition, saturation, price band, evidence count, and an opportunity score, plus the provenance of each metric so the verdict can be audited rather than trusted.

### Find the least-crowded corner of a theme

```bash
everbee-pp-cli research subniches --parent dad --product apparel --exclude-svg-png --limit 20 --agent
```

Expands the parent into child niches from EverBee's own suggestion engine, drops nothing but flags product type, and normalizes scores so the children are actually comparable.

### Narrow a verbose product payload for an agent

```bash
everbee-pp-cli products search --search-term "dad shirt" --agent --select results.title,results.price,results.listing_type,results.cached_est_mo_revenue
```

Product rows carry 68 fields each; --select trims the payload to the four that drive a decision, keeping agent context small.

### Size up the competition before designing

```bash
everbee-pp-cli research competitors "dad shirt" --agent
```

Reports result count, median price, review and sales density, and listing-age quartiles, with the raw rows printed alongside so the statistics can be checked.

### Track a niche week over week

```bash
everbee-pp-cli research drift "dad shirt" --save-baseline --agent
```

Saves a snapshot to the local store; re-running later diffs against it and reports what actually moved, with both fetch timestamps in provenance.

## Auth Setup

EverBee authenticates with a session token minted by Google SSO — there is no API-key page. Run 'everbee-pp-cli auth setup' for the steps to obtain a token, then store it with 'everbee-pp-cli auth set-token <token>', or set EVERBEE_ACCESS_TOKEN directly. Tokens expire; a 401 means the token needs replacing, not that the CLI is broken. Your EverBee plan gates research volume: the free Hobby plan allows only 10 keyword searches per month, and this CLI reports that cap as a typed error rather than as an empty result.

Run `everbee-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  everbee-pp-cli keyword-research list --agent --select results.title,results.price
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

- Use `--home <dir>` for one invocation, or set `EVERBEE_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `EVERBEE_CONFIG_DIR`, `EVERBEE_DATA_DIR`, `EVERBEE_STATE_DIR`, `EVERBEE_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `EVERBEE_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `everbee-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "everbee": {
        "command": "everbee-pp-mcp",
        "env": {
          "EVERBEE_HOME": "/srv/everbee"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `EVERBEE_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `EVERBEE_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
everbee-pp-cli recall "<user's question>" --agent
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
      "next_action": ["<trial command>", "everbee-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `everbee-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `everbee-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `everbee-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
everbee-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
everbee-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
everbee-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
everbee-pp-cli playbook amend \
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

`everbee-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `EVERBEE_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
everbee-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
everbee-pp-cli feedback --stdin < notes.txt
everbee-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `EVERBEE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `EVERBEE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
everbee-pp-cli profile save briefing --json
everbee-pp-cli --profile briefing keyword-research list
everbee-pp-cli profile list --json
everbee-pp-cli profile show briefing
everbee-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `everbee-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/marketing/everbee/cmd/everbee-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add everbee-pp-mcp -- everbee-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which everbee-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   everbee-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `everbee-pp-cli <command> --help`.
