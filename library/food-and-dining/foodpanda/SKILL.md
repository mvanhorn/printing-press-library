---
name: pp-foodpanda
description: "Every foodpanda restaurant, menu and price in a local database — with cross-restaurant dish search, price history and fee comparison the app cannot do. Trigger phrases: `cheapest delivery near me`, `find biryani on foodpanda`, `compare restaurant delivery fees`, `what changed on this foodpanda menu`, `which restaurants deliver to this address`, `use foodpanda`, `run foodpanda`."
author: "qazmataz"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - foodpanda-pp-cli
    install:
      - kind: go
        bins: [foodpanda-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/food-and-dining/foodpanda/cmd/foodpanda-pp-cli
---

# foodpanda — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `foodpanda-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install foodpanda --cli-only
   ```
2. Verify: `foodpanda-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

foodpanda's API hands you one restaurant at a time and forgets everything the moment you close the tab. This CLI mirrors an entire area into SQLite, so you can ask questions foodpanda structurally cannot answer: which restaurant near me sells the cheapest biryani, what changed in this menu since last week, and how delivery fees really compare once service fee and minimum order are counted. Works across every market foodpanda runs, and needs no API key for catalog data.

## When to Use This CLI

Reach for this CLI whenever a question spans more than one restaurant, or spans time. It is the right tool for comparing prices, fees or ratings across many vendors at once, for finding which nearby restaurant sells a specific dish, for tracking how a menu changes between syncs, and for pulling structured menu data for analysis. It covers every foodpanda market, not just one country.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to place an order, build a cart, or pay — it is read-only by design and has no checkout.
- Do not use it to fetch merchant commission rates; those are contract terms that appear in no consumer-facing foodpanda response.
- Do not use it for pandamart or grocery product catalogs; darkstores are listed but their item catalog is a separate surface this CLI does not cover.
- Do not use it to read your past orders; foodpanda exposes no web order-history route.
- Do not use it for a competitor delivery platform such as Uber Eats, Deliveroo or Careem — it only speaks to foodpanda.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`home`** — Rank every restaurant near your saved home address by what delivery actually costs you.

  _Reach for this when the question is 'what is cheapest to get delivered to me', not 'what is this one restaurant's fee'._

  ```bash
  foodpanda-pp-cli home --sort fee --limit 25 --agent
  ```
- **`dish`** — Find which nearby restaurant sells a specific dish cheapest, searching every synced menu at once.

  _Use this for item-level price hunting; use search when you want restaurants by name instead._

  ```bash
  foodpanda-pp-cli dish --query 'chicken biryani' --max-price 600 --agent
  ```
- **`menu-diff`** — Show what changed in a restaurant's menu and prices between two syncs.

  _Use this to catch price rises, removed items, or newly added deals over time._

  ```bash
  foodpanda-pp-cli menu-diff --vendor-code pk2v --since 7d --agent
  ```

### Competitive intelligence
- **`posture`** — Rank vendors by advertising and placement signals: CPC ad participation, promoted and premium status, and ranking score.

  _Use this for competitive analysis of who buys placement. It does not report merchant commission rates, which are not exposed in any consumer surface._

  ```bash
  foodpanda-pp-cli posture --latitude 31.5204 --longitude 74.3587 --ads-only --agent
  ```
- **`coverage`** — Determine which vendors actually deliver to an arbitrary point using each vendor's published delivery radius.

  _Use this before assuming a restaurant is orderable from an address you have not tried._

  ```bash
  foodpanda-pp-cli coverage --latitude 31.4820 --longitude 74.3430 --agent
  ```
- **`fees`** — Compare the full cost structure across an area: delivery fee, minimum order, service fee and VAT together.

  _Use this when headline delivery fee is misleading because service fee or minimum order dominates._

  ```bash
  foodpanda-pp-cli fees --latitude 24.8607 --longitude 67.0011 --sort total --agent
  ```

### Signal the site throws away
- **`digest`** — Split a restaurant's blended star rating into per-topic scores so food quality and delivery quality are separated.

  _Use this to tell 'the food is bad' apart from 'the delivery is bad' before trusting a rating._

  ```bash
  foodpanda-pp-cli digest --vendor-code pk2v --agent
  ```
- **`market-compare`** — Run the same query across every foodpanda market and compare vendor counts, ratings and fees side by side.

  _Use this for regional benchmarking; use vendors list when you only care about one city._

  ```bash
  foodpanda-pp-cli market-compare --query pizza --markets pk,sg,my --agent
  ```
- **`find`** — Search vendors live upstream and label how strongly each result actually matched the query.

  _Use this for live upstream search where match quality matters; use 'search' instead for offline full-text search over already-synced data._

  ```bash
  foodpanda-pp-cli find --query sushi --latitude 31.5204 --longitude 74.3587 --explain --agent
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**menu** — Full vendor detail including nested menus, products and prices

- `foodpanda-pp-cli menu <vendor_code>` — Fetch one vendor with full menu, deals and delivery conditions

**reviews** — Customer reviews with per-topic rating breakdown

- `foodpanda-pp-cli reviews <vendor_code>` — List reviews for a vendor, newest first

**vendors** — Browse and search foodpanda vendors near a location

- `foodpanda-pp-cli vendors list` — List vendors near a coordinate, with filters and sorting
- `foodpanda-pp-cli vendors search` — Full-text search vendors and dishes near a coordinate


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
foodpanda-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Cheapest delivery near home

```bash
foodpanda-pp-cli home --sort fee --limit 20 --agent
```

Ranks every restaurant reaching your saved home address by real delivery cost, which the app never lets you sort by.

### Narrow a huge listing payload for an agent

```bash
foodpanda-pp-cli vendors list --latitude 31.5204 --longitude 74.3587 --limit 40 --agent --select code,name,rating,minimum_delivery_fee,minimum_order_amount
```

Vendor rows carry 80+ fields each; selecting five keeps the response small enough to reason over without burning context.

### Find the cheapest biryani in town

```bash
foodpanda-pp-cli dish --query biryani --max-price 700 --sort price --agent
```

Searches every synced menu at once and returns item-level matches with the restaurant that sells them.

### Track a menu for price rises

```bash
foodpanda-pp-cli menu-diff --vendor-code pk2v --since 14d --agent
```

Diffs two local snapshots to surface added, removed and repriced items over the last two weeks.

### See who is buying placement

```bash
foodpanda-pp-cli posture --latitude 24.8607 --longitude 67.0011 --ads-only --sort points --agent
```

Ranks vendors by CPC ad participation and premium placement signals for competitive analysis.

## Auth Setup

Catalog browsing needs no credentials at all — vendor search, menus, prices, deals and reviews all work anonymously. Only your own account data (saved addresses, and the `home` command that depends on them) needs a session. Run `foodpanda-pp-cli auth login --chrome` to import the `token` cookie from a browser where you are already signed in; the CLI composes it into an Authorization header. There is no API key to request and nothing to paste.

Run `foodpanda-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  foodpanda-pp-cli menu mock-value --agent --select id,name,status
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

- Use `--home <dir>` for one invocation, or set `FOODPANDA_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `FOODPANDA_CONFIG_DIR`, `FOODPANDA_DATA_DIR`, `FOODPANDA_STATE_DIR`, `FOODPANDA_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `FOODPANDA_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `foodpanda-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "foodpanda": {
        "command": "foodpanda-pp-mcp",
        "env": {
          "FOODPANDA_HOME": "/srv/foodpanda"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `FOODPANDA_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `FOODPANDA_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
foodpanda-pp-cli recall "<user's question>" --agent
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
      "next_action": ["<trial command>", "foodpanda-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `foodpanda-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `foodpanda-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `foodpanda-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
foodpanda-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
foodpanda-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
foodpanda-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
foodpanda-pp-cli playbook amend \
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

`foodpanda-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `FOODPANDA_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
foodpanda-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
foodpanda-pp-cli feedback --stdin < notes.txt
foodpanda-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `FOODPANDA_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `FOODPANDA_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
foodpanda-pp-cli profile save briefing --json
foodpanda-pp-cli --profile briefing menu mock-value
foodpanda-pp-cli profile list --json
foodpanda-pp-cli profile show briefing
foodpanda-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `foodpanda-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/food-and-dining/foodpanda/cmd/foodpanda-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add foodpanda-pp-mcp -- foodpanda-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which foodpanda-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   foodpanda-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `foodpanda-pp-cli <command> --help`.
