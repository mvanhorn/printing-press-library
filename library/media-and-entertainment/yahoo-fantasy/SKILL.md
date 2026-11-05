---
name: pp-yahoo-fantasy
description: "Full Yahoo Fantasy Sports read/write CLI and MCP server — set lineups, execute add/drops and FAAB waiver claims, and run local SQLite league analytics. Trigger phrases: `set my Yahoo fantasy football lineup`, `add and drop a player on Yahoo fantasy`, `check Yahoo fantasy waiver wire upgrades`, `show my Yahoo fantasy matchup and standings`, `use yahoo-fantasy`, `run yahoo-fantasy`."
author: "BenHof"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - yahoo-fantasy-pp-cli
    install:
      - kind: go
        bins: [yahoo-fantasy-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yahoo-fantasy/cmd/yahoo-fantasy-pp-cli
---

# Yahoo Fantasy Sports — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `yahoo-fantasy-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install yahoo-fantasy --cli-only
   ```
2. Verify: `yahoo-fantasy-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.10 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yahoo-fantasy/cmd/yahoo-fantasy-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Wraps the complete Yahoo Fantasy Sports v2 API for NFL, NBA, MLB, and NHL with automatic XML payload construction for lineup changes (`lineup`) and free-agent/waiver transactions (`add-drop`). Syncs leagues, rosters, matchups, draft results, and transactions into a local SQLite store for instant FTS5 search, all-play power rankings (`power-rankings`), waiver upgrade recommendations (`waiver-wire`), and FAAB bid audits (`faab-audit`).

## When to Use This CLI

Use this CLI and MCP server to inspect Yahoo Fantasy Football (and NBA/MLB/NHL) leagues, standings, weekly matchups, rosters, and free agents, as well as to execute starting lineup changes (`lineup`), free-agent pickups, drops, and FAAB waiver claims (`add-drop`). Prefer the compound commands (`lineup`, `add-drop`, `waiver-wire`, `matchup-radar`, `power-rankings`, `faab-audit`, `draft-roi`) over raw endpoint calls.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI for Yahoo Finance stock quotes or options chains (use yahoo-finance-pp-cli).
- Do not use this CLI for ESPN or Sleeper fantasy leagues.
- Do not execute live roster or transaction mutations without previewing via --dry-run when user intent is ambiguous.

## Unique Capabilities

### Roster & Transaction Operations
- **`lineup`** — Audit active starting slots for OUT, IR, or BYE players, preview bench swaps, and submit Yahoo XML roster updates.

  _Use this instead of raw PUT /roster calls when checking for inactive starters or moving players between starting, bench, and IR slots._

  ```bash
  yahoo-fantasy-pp-cli lineup --team 449.l.365083.t.1 --week 9 --set QB=449.p.31862,BN=449.p.30123 --dry-run --agent
  ```
- **`add-drop`** — Execute or dry-run free-agent adds, drops, atomic add/drops, and FAAB waiver claims with automatic XML payload construction.

  _Use this to safely preview and submit single adds, drops, or atomic add/drop waiver claims without hand-writing Yahoo XML._

  ```bash
  yahoo-fantasy-pp-cli add-drop --league 449.l.365083 --team 449.l.365083.t.1 --add 449.p.31862 --drop 449.p.30123 --faab 15 --dry-run --agent
  ```

### Weekly Matchup & Waiver Intelligence
- **`waiver-wire`** — Rank available free agents and waiver players by net points-per-game upgrade over your weakest bench player at each position.

  _Reach for this before Tuesday waiver runs to find highest-leverage roster upgrades without scanning raw 25-player pages._

  ```bash
  yahoo-fantasy-pp-cli waiver-wire --league 449.l.365083 --team 449.l.365083.t.1 --pos RB --limit 10 --agent
  ```
- **`matchup-radar`** — Compare both opponents' starting lineups slot-by-slot with projected vs. actual points, remaining starters, and injury flags.

  _Use this for a complete single-call head-to-head matchup breakdown instead of fetching scoreboard and two team rosters separately._

  ```bash
  yahoo-fantasy-pp-cli matchup-radar --league 449.l.365083 --team 449.l.365083.t.1 --week 9 --agent
  ```

### League Analytics & SQLite Intelligence
- **`power-rankings`** — Compute every team's true weekly all-play record, schedule luck delta, scoring consistency, and composite power rank.

  _Use this when evaluating true league contenders and unlucky teams beyond raw win-loss standings._

  ```bash
  yahoo-fantasy-pp-cli power-rankings --league 449.l.365083 --through-week 9 --agent
  ```
- **`faab-audit`** — Track every manager's remaining FAAB budget, positional spending breakdown, winning bids, and blocking bid thresholds.

  _Use this before setting a FAAB bid to see exactly how much budget rival managers have left and what they typically bid._

  ```bash
  yahoo-fantasy-pp-cli faab-audit --league 449.l.365083 --agent
  ```
- **`draft-roi`** — Evaluate snake draft rounds or auction dollar costs against season-to-date fantasy points and current roster retention.

  _Use this to grade draft performance and identify the highest-surplus steals and biggest busts in the league._

  ```bash
  yahoo-fantasy-pp-cli draft-roi --league 449.l.365083 --agent
  ```

## Command Reference

**games** — Inspect Yahoo Fantasy game seasons, codes, and stat categories

- `yahoo-fantasy-pp-cli games <game_key>` — Get Yahoo Fantasy game metadata by game code (nfl, nba, mlb, nhl) or numeric game ID

**leagues** — Query Yahoo Fantasy league settings, standings, weekly scoreboards, players, draft results, and transactions

- `yahoo-fantasy-pp-cli leagues get` — Get league overview metadata (name, current week, scoring type, season)
- `yahoo-fantasy-pp-cli leagues get-draft-results` — Get complete league draft results with round, overall pick, team key, player key, and auction cost
- `yahoo-fantasy-pp-cli leagues get-players` — List players in a league filtered by availability status (FA, W, T, A), position, and sort order
- `yahoo-fantasy-pp-cli leagues get-scoreboard` — Get weekly league scoreboard and head-to-head matchups with live and projected points
- `yahoo-fantasy-pp-cli leagues get-settings` — Get league rules, roster position slots, stat categories, point modifiers, and FAAB settings
- `yahoo-fantasy-pp-cli leagues get-standings` — Get league standings ordered by rank with win-loss-tie records, points for/against, and streak
- `yahoo-fantasy-pp-cli leagues get-teams` — List all teams in a league with manager profiles, waiver priority, and remaining FAAB budget
- `yahoo-fantasy-pp-cli leagues get-transactions` — Get league transaction activity log filtered by transaction types (add, drop, trade, commish)

**players** — Look up Yahoo Fantasy player profiles, weekly/season statistics, percent owned, and league ownership

- `yahoo-fantasy-pp-cli players get-ownership` — Get league-specific ownership status (free agent, waivers, or owning fantasy team) for players
- `yahoo-fantasy-pp-cli players get-percent-owned` — Get Yahoo-wide ownership percentage and week-over-week delta for one or more players
- `yahoo-fantasy-pp-cli players get-stats` — Get player statistics for one or more player keys in a league by week or season
- `yahoo-fantasy-pp-cli players search` — Search players by name within a league and return their profile and season stats

**teams** — Inspect fantasy teams, weekly rosters, head-to-head matchups, and update starting lineups

- `yahoo-fantasy-pp-cli teams get` — Get fantasy team details including manager, waiver priority, FAAB balance, and logos
- `yahoo-fantasy-pp-cli teams get-matchups` — Get head-to-head matchups and projected/actual scores for a team across one or more weeks
- `yahoo-fantasy-pp-cli teams get-roster` — Get a team's active and bench roster for a specific NFL week
- `yahoo-fantasy-pp-cli teams update-roster` — Update a team's starting lineup and bench positions for a given week or date

**transactions** — Execute and manage league transactions including free-agent adds, drops, atomic add/drops, FAAB waiver claims, and trades

- `yahoo-fantasy-pp-cli transactions cancel` — Cancel a pending waiver claim or proposed trade before it processes
- `yahoo-fantasy-pp-cli transactions create` — Submit an add, drop, atomic add/drop, waiver claim (with optional FAAB bid), or trade proposal
- `yahoo-fantasy-pp-cli transactions get` — Get details of a specific transaction, waiver claim, or pending trade by transaction key
- `yahoo-fantasy-pp-cli transactions update` — Edit a pending waiver claim (FAAB bid or priority) or accept/reject a pending trade proposal

**users** — List leagues and teams belonging to the authenticated Yahoo user

- `yahoo-fantasy-pp-cli users get-leagues` — List all Yahoo Fantasy leagues for the authenticated user filtered by sport code
- `yahoo-fantasy-pp-cli users get-teams` — List all fantasy teams owned by the authenticated Yahoo user across seasons


**add-drop**


- `yahoo-fantasy-pp-cli add-drop` — Execute or dry-run free-agent adds, drops, atomic add/drops, and FAAB waiver claims with automatic XML payload construction.


**draft-roi**


- `yahoo-fantasy-pp-cli draft-roi` — Evaluate snake draft rounds or auction dollar costs against season-to-date fantasy points and current roster retention.


**faab-audit**


- `yahoo-fantasy-pp-cli faab-audit` — Track every manager's remaining FAAB budget, positional spending breakdown, winning bids, and blocking bid thresholds.


**lineup**


- `yahoo-fantasy-pp-cli lineup` — Audit active starting slots for OUT, IR, or BYE players, preview bench swaps, and submit Yahoo XML roster updates.


**matchup-radar**


- `yahoo-fantasy-pp-cli matchup-radar` — Compare both opponents' starting lineups slot-by-slot with projected vs. actual points, remaining starters, and injury flags.


**power-rankings**


- `yahoo-fantasy-pp-cli power-rankings` — Compute every team's true weekly all-play record, schedule luck delta, scoring consistency, and composite power rank.


**waiver-wire**


- `yahoo-fantasy-pp-cli waiver-wire` — Rank available free agents and waiver players by net points-per-game upgrade over your weakest bench player at each position.




### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
yahoo-fantasy-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query. `--json` (and other machine formats) keep that exit-2 contract and write `{"matches":[]}` on stdout so agents can inspect the envelope without treating a miss as success.

## Recipes

### Preview and apply a Sunday morning lineup swap

```bash
yahoo-fantasy-pp-cli lineup --team 449.l.365083.t.1 --week 9 --set QB=449.p.31862,BN=449.p.30123 --dry-run --agent
```

Validates positional eligibility and previews the XML roster payload before updating your starting lineup.

### Submit an atomic add/drop waiver claim with FAAB

```bash
yahoo-fantasy-pp-cli add-drop --league 449.l.365083 --team 449.l.365083.t.1 --add 449.p.31862 --drop 449.p.30123 --faab 15 --dry-run --agent
```

Constructs Yahoo's <fantasy_content><transaction> XML for an atomic add/drop with a $15 FAAB bid.

### Find top RB waiver upgrades over your bench

```bash
yahoo-fantasy-pp-cli waiver-wire --league 449.l.365083 --team 449.l.365083.t.1 --pos RB --limit 5 --agent --select upgrades.add_player_name,upgrades.percent_owned,upgrades.net_upgrade_ppg
```

Compares available free agents and waiver RBs against your lowest-scoring bench RB and filters to key decision fields.

### Audit league FAAB budgets before Tuesday waivers

```bash
yahoo-fantasy-pp-cli faab-audit --league 449.l.365083 --agent
```

Shows every manager's remaining FAAB balance, total spent by position, and blocking bid thresholds.

## Auth Setup

Set `YAHOO_ACCESS_TOKEN` (or `YAHOO_FANTASY_TOKEN`) with a Yahoo OAuth 2.0 Bearer token, or configure `YAHOO_CLIENT_ID`, `YAHOO_CLIENT_SECRET`, and `YAHOO_REFRESH_TOKEN` so the CLI can automatically refresh expired access tokens against `https://api.login.yahoo.com/oauth2/get_token`.

Run `yahoo-fantasy-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color`.

Global format flags share one contract on promoted, novel, sync, and `--deliver` paths:

- `--json` — one JSON document on stdout (sync progress events go to stderr)
- `--compact` — keep identity/status/timestamp fields; does not change the document vs stream shape
- `--csv` / `--plain` — tabular rows (collection envelopes unwrap to the row array). `--csv` prefixes `'` on a cell or header key that starts with `=`, `+`, `-`, `@`, tab, CR, or LF (fullwidth `＝＋－＠` too) so a spreadsheet does not run it as a formula; a plain signed number such as `-5` or `+2.5e3` is unchanged.
- `--quiet` — one identity value per row, no envelope

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  yahoo-fantasy-pp-cli games mock-value --agent --select game_key,game_id,name
  ```
- **Previewable** — `--dry-run` shows the request without sending. A completed dry run exits 0 with `"success": true` and `"dry_run": true` and no HTTP `status`; validation failures still exit non-zero with `"success": false`.
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit confirmation** — `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and use `--ignore-missing` only when a missing delete target should count as success

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

- Use `--home <dir>` for one invocation, or set `YAHOO_FANTASY_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `YAHOO_FANTASY_CONFIG_DIR`, `YAHOO_FANTASY_DATA_DIR`, `YAHOO_FANTASY_STATE_DIR`, `YAHOO_FANTASY_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `YAHOO_FANTASY_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `yahoo-fantasy-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "yahoo-fantasy": {
        "command": "yahoo-fantasy-pp-mcp",
        "env": {
          "YAHOO_FANTASY_HOME": "/srv/yahoo-fantasy"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `YAHOO_FANTASY_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `YAHOO_FANTASY_HOME`, or `doctor` will not find credentials left under the former root.

Local data is unencrypted and relies on OS account permissions plus disk encryption. Reads in the default `--data-source auto` mode are written to the SQLite store; GET responses are cached for five minutes under the cache dir. `local-data purge` previews what it would erase (store files, response cache, learn journal) and deletes only with `--yes`; it is not exposed over MCP. `YAHOO_FANTASY_NO_LOCAL_DATA=1` disables the response cache and store writes, turns `auto` reads into live reads, and blocks `sync` and `workflow archive`; `--data-source local` still reads an existing store, and `YAHOO_FANTASY_DATA_SOURCE` sets the `--data-source` default.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, pass the question as an argv or MCP tool argument to `recall --agent`. Do not interpolate user-controlled text into a shell command line.

Quoted `recall "<question>"` breaks on an apostrophe, which is ordinary English. A quoted heredoc breaks when a body line equals the delimiter, and that delimiter is published in these docs. Write the question with a non-shell file-writing tool, then read it back as data:

```bash
# Write the question verbatim with your file-writing tool (no shell involved).
# Command substitution on a file only ever yields data — the shell never
# parses the file's bytes as syntax.
QUERY=$(cat /path/to/question.txt)
yahoo-fantasy-pp-cli recall "$QUERY" --agent
```

Prefer MCP: pass the question as the tool's query argument. `"$QUERY"` after a file read is argv-safe; putting the question itself in the command text is not.

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
      "next_action": ["<trial command>", "yahoo-fantasy-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `yahoo-fantasy-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `yahoo-fantasy-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `yahoo-fantasy-pp-cli sync --resources users` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately. Pass the query the same way as recall — argv/MCP, or file-then-`$QUERY`. Do not splice the question into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
yahoo-fantasy-pp-cli teach --query "$QUERY" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
QUERY=$(cat /path/to/question.txt)
yahoo-fantasy-pp-cli teach \
  --query "$QUERY" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
QUERY=$(cat /path/to/question.txt)
yahoo-fantasy-pp-cli teach-playbook \
  --query "$QUERY" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`. Pass the query and note as argv/MCP arguments, or write each with a non-shell file tool and read them back (`QUERY=$(cat ...)`, `NOTE=$(cat ...)`). Do not interpolate either string into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
NOTE=$(cat /path/to/note.txt)
yahoo-fantasy-pp-cli playbook amend \
  --query "$QUERY" \
  --add-note "$NOTE"
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

`yahoo-fantasy-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `YAHOO_FANTASY_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
yahoo-fantasy-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
yahoo-fantasy-pp-cli feedback --stdin < notes.txt
yahoo-fantasy-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `YAHOO_FANTASY_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `YAHOO_FANTASY_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` as a private 0600 file (exclusive randomly named temp in the destination directory, then rename; a planted symlink is not followed). Binary-response commands write decoded payload bytes (not the base64 JSON envelope) and print a small JSON receipt on stdout; `--json`/`--csv` do not refuse when this sink is set. |
| `webhook:<url>` | POST the output body to the URL (`application/json`). Redirects are not followed; a 3xx response fails the delivery and the body is not sent to the `Location`. |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures, including redirects, return non-zero and log the URL and HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

```
yahoo-fantasy-pp-cli profile save briefing --json
yahoo-fantasy-pp-cli --profile briefing games mock-value
yahoo-fantasy-pp-cli profile list --json
yahoo-fantasy-pp-cli profile show briefing
yahoo-fantasy-pp-cli profile delete briefing --yes
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
| 6 | Partial failure |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `yahoo-fantasy-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yahoo-fantasy/cmd/yahoo-fantasy-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add yahoo-fantasy-pp-mcp -- yahoo-fantasy-pp-mcp
   ```
3. Verify: `claude mcp list`

For a remote host, `yahoo-fantasy-pp-mcp --transport http` requires `YAHOO_FANTASY_MCP_HTTP_TOKEN` set to at least 32 characters (callers send `Authorization: Bearer <token>`; `openssl rand -hex 32` prints 64 hex characters and meets that minimum), binds `127.0.0.1:7777` by default, and needs `--tls-cert`/`--tls-key` for any non-loopback `--addr`. Requests with no `Origin` header are accepted. Loopback origins are accepted; any other `Origin` is refused unless listed in `YAHOO_FANTASY_MCP_HTTP_ALLOWED_ORIGINS` (comma-separated `scheme://host[:port]` values). `YAHOO_FANTASY_MCP_READ_ONLY=1` drops mirrored write commands (local-write commands that only touch the CLI's own learn store stay) and makes the raw execute tool refuse state-changing calls.

## Direct Use

1. Check if installed: `which yahoo-fantasy-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   yahoo-fantasy-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `yahoo-fantasy-pp-cli <command> --help`.
