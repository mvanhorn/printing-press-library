---
name: pp-woolworths
description: "Every Woolworths catalogue, specials, store and trolley surface, plus a local price history that can tell a genuine half-price from an inflated was-price. Trigger phrases: `is this woolworths special actually cheap`, `when will this go half price again`, `cheapest per 100g at woolworths`, `what's on special at woolworths this week`, `price my woolworths shopping list`, `use woolworths`, `run woolworths`."
author: "Richard Gill"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - woolworths-pp-cli
    install:
      - kind: go
        bins: [woolworths-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/commerce/woolworths/cmd/woolworths-pp-cli
---

# Woolworths — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `woolworths-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install woolworths --cli-only
   ```
2. Verify: `woolworths-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/woolworths/cmd/woolworths-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Woolworths shows you today's price and a SAVE badge, and the badge is exactly what shoppers say they cannot trust. This CLI keeps its own price history in SQLite, so real-special returns a verdict instead of a number, cycle forecasts when the next half-price window opens, and swap ranks alternatives by unit price normalised across measure bases. It needs no API key, no login for the catalogue, and no headless browser. It is read-only apart from adding to a guest trolley.

## When to Use This CLI

Use this CLI for Australian grocery pricing questions that need a decided answer rather than raw product JSON: whether a special is genuine, when a discount is likely to return, which alternative is cheaper per unit, and what a whole shopping list costs versus last time. It is also the right tool for building or costing a trolley programmatically, since the guest trolley works without any account.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI for Everyday Rewards points, boosters, or e-receipts - that stack is MFA-gated with short-lived tokens and is not automatable.
- Do not use this CLI to compare against Coles, ALDI or IGA - it covers Woolworths only.
- Do not use this CLI to look up a product by barcode - the barcode endpoint lives on a mobile API whose key is revoked.
- Do not use this CLI to place or pay for an order - it can build a trolley but does not check out.
- Do not use this CLI as a bulk catalogue scraper - it rate-limits deliberately, and walking every category is what gets shoppers IP-banned.
- Do not rely on 'sync' to build price history - on this API only 'settings' syncs blind; the history-recording commands are 'real-special' and 'specials-diff --refresh'.
- Do not use 'auth login --chrome' on Windows - browser-session import is unavailable there, so saved lists and past shops are unreachable on that platform.
- Do not use 'basket --dry-run' to price a list; it only echoes the action. Use '--record=false' for a real run that writes nothing.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local history that compounds
- **`real-special`** — Tells you whether an advertised special is genuinely cheap or just a lower number next to an inflated was-price.

  _Reach for this instead of reading the SAVE badge: it returns one decided verdict per product rather than raw prices you have to judge yourself._

  ```bash
  woolworths-pp-cli real-special "tim tam" --limit 2 --agent
  ```
- **`cycle`** — Estimates when a product's next half-price window is likely to open, from its own recorded discount episodes.

  _Use this to decide buy-one-or-buy-six on a stockpile item, instead of guessing whether the deal returns next month._

  ```bash
  woolworths-pp-cli cycle 6073909 --agent
  ```
- **`specials-diff`** — Shows what entered and left each specials group since your last sync, with how long since each entrant was last discounted.

  _Reach for this on catalogue-rollover day to see only what actually changed rather than re-reading the whole specials list._

  ```bash
  woolworths-pp-cli specials-diff --limit 5 --agent
  ```

### Unit-price intelligence
- **`swap`** — Ranks in-stock alternatives by unit price normalised across different measure bases, so per-100g and per-1kg tiles compare correctly.

  _Pick this when the question is which product is genuinely cheaper per unit, not which has the lowest shelf price._

  ```bash
  woolworths-pp-cli swap "olive oil" --limit 3 --max-scan-pages 1 --agent
  ```
- **`multibuy`** — Works out what a multi-buy offer actually costs per unit at the quantity it demands, versus buying singly or buying a larger pack.

  _Use this before accepting a 2-for-$9 style offer, since a bigger single pack is often cheaper per unit._

  ```bash
  woolworths-pp-cli multibuy chocolate --limit 3 --max-scan-pages 1 --agent
  ```

### Agent-native plumbing
- **`basket`** — Prices a whole shopping list, showing which lines moved since the last time you ran it and what it did to the total.

  _Use this to turn a plain-text list into a costed, decided answer in one call instead of dozens of searches._

  ```bash
  woolworths-pp-cli basket ./groceries.txt --record=false --agent
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**categories** — Browse the department tree and specials groups

- `woolworths-pp-cli categories browse` — Browse or page one category or specials group by node id
- `woolworths-pp-cli categories list` — Full department tree including the five specials groups and their product counts

**pastshops** — Past shops / order history (requires an imported Chrome session)

- `woolworths-pp-cli pastshops` — Previous shops recorded against the account

**products** — Search and inspect the Woolworths product catalogue

- `woolworths-pp-cli products batch` — Fetch several products in one call by comma-separated stockcodes
- `woolworths-pp-cli products count` — Cheap result counts for a term - products, specials, recipes - without fetching tiles
- `woolworths-pp-cli products detail` — Full product record including nutrition, variants and country of origin
- `woolworths-pp-cli products schemaorg` — Product as schema.org JSON-LD; an independent path from products detail
- `woolworths-pp-cli products search` — Search products by term, with optional specials-only filter and sort
- `woolworths-pp-cli products suggestions` — Ranked search suggestions and autocorrect for a partial term

**savedlists** — Saved shopping lists (requires an imported Chrome session)

- `woolworths-pp-cli savedlists get` — One saved list including its products and free-text lines
- `woolworths-pp-cli savedlists list` — All saved shopping lists with product and free-text counts

**settings** — Site configuration and feature flags

- `woolworths-pp-cli settings bootstrap` — App bootstrap config including current site version
- `woolworths-pp-cli settings list` — Site settings and feature flags

**stores** — Find Woolworths stores, trading hours and facilities

- `woolworths-pp-cli stores` — Find stores near a postcode or coordinate, with trading hours and facilities

**trolley** — Read and build the trolley; works anonymously as a guest cart

- `woolworths-pp-cli trolley add` — Add a product to the trolley by stockcode
- `woolworths-pp-cli trolley get` — Current trolley contents


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
woolworths-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Check a special before you buy

```bash
woolworths-pp-cli real-special "tim tam" --agent
```

Returns a verdict per matching product rather than a price you have to interpret.

### Narrow a large search for an agent

```bash
woolworths-pp-cli products search --term milk --agent --select Products.Products.Name,Products.Products.Price,Products.Products.CupString
```

A bare search returns hundreds of KB; selecting three dotted paths keeps the response small enough to reason over.

### Find a cheaper equivalent by real unit price

```bash
woolworths-pp-cli swap "olive oil" --limit 10 --agent
```

Normalises across per-100g and per-1L tiles so the ranking is honest.

### See what changed on catalogue rollover

```bash
woolworths-pp-cli specials-diff --agent
```

Only the entrants and departures since your last sync, not the whole specials list.

### Cost this week's list against last week's

```bash
woolworths-pp-cli basket ./groceries.txt --record=false --agent
```

Per-line movement plus a basket total compared with the previous run.

## Auth Setup

The catalogue, specials, stores and guest trolley need no account at all - only browser-shaped headers and a cookie jar the CLI warms automatically on first call. Personal surfaces (saved lists, past shops) sit behind Auth0 universal login with MFA, so there is no username/password flow to automate. On macOS and Linux, 'auth login --chrome' imports an existing logged-in Chrome session. On Windows that import is unavailable (the underlying cookie reader does not support it), so saved lists and past shops cannot be used there; the rest of the CLI is unaffected.

Run `woolworths-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  woolworths-pp-cli categories list --agent
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit confirmation** — `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

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

- Use `--home <dir>` for one invocation, or set `WOOLWORTHS_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `WOOLWORTHS_CONFIG_DIR`, `WOOLWORTHS_DATA_DIR`, `WOOLWORTHS_STATE_DIR`, `WOOLWORTHS_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `WOOLWORTHS_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `woolworths-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "woolworths": {
        "command": "woolworths-pp-mcp",
        "env": {
          "WOOLWORTHS_HOME": "/srv/woolworths"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `WOOLWORTHS_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `WOOLWORTHS_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
woolworths-pp-cli recall "<user's question>" --agent
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
      "next_action": ["<trial command>", "woolworths-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `woolworths-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `woolworths-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `woolworths-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
woolworths-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
woolworths-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
woolworths-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
woolworths-pp-cli playbook amend \
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

`woolworths-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `WOOLWORTHS_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
woolworths-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
woolworths-pp-cli feedback --stdin < notes.txt
woolworths-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `WOOLWORTHS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `WOOLWORTHS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
woolworths-pp-cli profile save briefing --json
woolworths-pp-cli --profile briefing categories list
woolworths-pp-cli profile list --json
woolworths-pp-cli profile show briefing
woolworths-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `woolworths-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/commerce/woolworths/cmd/woolworths-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add woolworths-pp-mcp -- woolworths-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which woolworths-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   woolworths-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `woolworths-pp-cli <command> --help`.
