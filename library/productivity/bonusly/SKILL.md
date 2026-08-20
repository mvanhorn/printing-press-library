---
name: pp-bonusly
description: "The recognition analytics Bonusly reserves for admins -- rebuilt from data any employee can already read. Trigger phrases: `give someone bonusly recognition`, `check my bonusly points balance`, `who have I not recognized on my team`, `search my bonusly recognition history`, `bonusly budget pacing for my team`, `use bonusly`, `run bonusly`."
author: "Allen Lew"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - bonusly-pp-cli
    install:
      - kind: go
        bins: [bonusly-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/productivity/bonusly/cmd/bonusly-pp-cli
---

# Bonusly — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `bonusly-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install bonusly --cli-only
   ```
2. Verify: `bonusly-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/bonusly/cmd/bonusly-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Bonusly's own Analytics API and admin reports compute participation, spend, and recognition-equity insights, but they require an admin scope most employees don't have. This CLI mirrors exactly the data a regular employee CAN read -- the company feed, org chart, your own balance and redemptions -- into a local SQLite database, then computes that same category of insight offline: budget pacing, burn-rate forecasting, neglected-teammate detection, and company-values trends. No admin access required, and it works when you're offline.

## When to Use This CLI

Use this CLI when a Bonusly user without admin access wants participation, equity, or spend insight that Bonusly's own UI reserves for admin reports, or wants to search/analyze their own recognition history offline.

## Anti-triggers

Do not use this CLI for:
- Do not use this for company-wide admin reporting (participation report, spend totals, redemption approvals) -- those need an admin-scoped token this CLI does not support.
- Do not use this to detect deleted recognitions retroactively -- the non-admin feed has no tombstone/deletion signal, only the admin-gated Analytics API does.
- Do not use this for real-time notifications or a live dashboard -- it's a CLI with a local mirror, not a background service.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`recognition audit`** — See whether your team's recognition spend is on pace with its monthly budget, broken down by department -- the report Bonusly normally reserves for admins.

  _Reach for this when a team lead wants pacing/budget visibility without admin access, instead of manually tallying the web feed._

  ```bash
  bonusly-pp-cli recognition audit --dept engineering --agent
  ```
- **`recognition search-mine`** — Search your own given and received recognition offline, without scrolling the public company feed. Requires one live lookup to confirm your identity (no local fallback); not yet verified against a real API response in this build.

  _Use this for self-review prep or recalling why you were recognized -- the company-wide feed search returns everyone's posts, not just yours._

  ```bash
  bonusly-pp-cli recognition search-mine "migration project" --agent
  ```
- **`balance history`** — Track your giving-allowance burn rate over time and see forfeiture coming before the monthly reset, not after.

  _Check this a few days before month-end to see if points are about to be forfeited._

  ```bash
  bonusly-pp-cli balance history --agent
  ```
- **`recognition gap`** — Find direct reports you haven't recognized recently, without an admin's Participation Report. Requires a live lookup to resolve your manager identity and direct reports (no local fallback); not yet verified against a real API response in this build.

  _Use before 1:1s or sprint retros to catch teammates who've gone unrecognized._

  ```bash
  bonusly-pp-cli recognition gap --manager me --days 30 --agent
  ```
- **`recognition values`** — See which company-value hashtags are actually trending in a department, instead of manually tallying the feed.

  _Use this for a culture pulse-check across a team or the whole company._

  ```bash
  bonusly-pp-cli recognition values --dept engineering --agent
  ```
- **`redemptions forecast`** — Project your reward-redemption spend from your own history -- a simple trend line, not a black box.

  _Use this to sanity-check whether your redeemable balance will cover a reward you're eyeing._

  ```bash
  bonusly-pp-cli redemptions forecast --agent
  ```

## Command Reference

<!-- pp:hand-edit bonusly-remove-broken-commands — awards, groups, incentives,
     and meetings command groups removed below: no working endpoint could be
     found for any of them despite ~20 live path probes each. See
     .printing-press-patches/bonusly-remove-broken-commands.json. -->

**balance** — Your points balance and lifetime stats

- `bonusly-pp-cli balance` — Your current giving/redeemable balance, monthly budget, and lifetime stats

**company** — Company metadata

- `bonusly-pp-cli company` — Company metadata: name, locale, plan, feature flags, subscription state

**departments** — Departments configured for your company, with headcounts

- `bonusly-pp-cli departments list` — List departments with per-department user counts
- `bonusly-pp-cli departments users` — List users belonging to a department (exact match)

**locations** — Locations configured for your company, with headcounts

- `bonusly-pp-cli locations list` — List locations with per-location user counts
- `bonusly-pp-cli locations users` — List users belonging to a location (exact match)

**org** — Org-chart traversal: top-level users, direct reports, manager chains, reporting trees

- `bonusly-pp-cli org chain` — Walk the manager chain upward from a user, closest-first
- `bonusly-pp-cli org reports` — List users who report directly to a given manager
- `bonusly-pp-cli org top` — List users with no manager (org-chart entry points)
- `bonusly-pp-cli org tree` — Walk the reporting tree downward from a user

**give** — Give recognition to one or more colleagues with structured flags

- `bonusly-pp-cli give` — Give recognition using `--to`, `--amount`, `--message`, `--hashtag`. Synthesizes the `+N @mention message #hashtag` reason string Bonusly's API expects; prefer this over the raw `recognition create` passthrough below.

**recognition** — Give, browse, and manage recognition (bonuses)

- `bonusly-pp-cli recognition create` — Give recognition via the raw reason-string DSL directly (low-level passthrough; prefer `give` above for structured input).
- `bonusly-pp-cli recognition delete` — Delete (undo) a recognition you gave, within 24 hours of creation
- `bonusly-pp-cli recognition feed` — List/browse the company recognition feed with filters
- `bonusly-pp-cli recognition get` — Get a single recognition by id
- `bonusly-pp-cli recognition given` — List recognition given by a user
- `bonusly-pp-cli recognition group-count` — Resolve a group (department/location/team) and count how many recipients a post would reach
- `bonusly-pp-cli recognition last-given` — Get when you last gave recognition to each of a batch of users (max 20 ids)
- `bonusly-pp-cli recognition list-types` — List the recognition-type values accepted by feed filters (celebrations, awards, incentives, peer, external_recognition)
- `bonusly-pp-cli recognition received` — List recognition received by a user
- `bonusly-pp-cli recognition update` — Edit a recognition you gave, within 24 hours of creation.

**redemptions** — Your own reward redemptions

- `bonusly-pp-cli redemptions get` — Get a single reward redemption by id (your own, or any if you have rewards-admin)
- `bonusly-pp-cli redemptions list-mine` — List your own reward redemptions, newest first

**users** — Your own profile and other users in your company

- `bonusly-pp-cli users get` — Resolve a single user by id, email, or display name
- `bonusly-pp-cli users get-bulk` — Bulk-fetch users by a list of user IDs
- `bonusly-pp-cli users me` — Get the authenticated user's own profile
- `bonusly-pp-cli users search` — Search users by name or email, with optional department/location filters


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
bonusly-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query against this CLI's curated *unique-capability* index (the transcendence commands above), not the full command tree — it will not resolve general operations like giving recognition, auth, or sync. Exit code `0` means at least one match; exit code `2` means no confident match. For anything outside the unique capabilities, use `--help` or the command reference above directly.

## Recipes

### Narrow a noisy feed response to just the fields you need

```bash
bonusly-pp-cli recognition feed --hashtag teamwork --agent --select recognitions.giver.display_name,recognitions.receivers.display_name,recognitions.amount
```

The feed returns giver/receiver/hashtag/reason/amount per row; --select keeps only the fields an agent actually needs instead of the full nested payload.

### Check budget pacing before month-end

```bash
bonusly-pp-cli recognition audit --dept engineering --agent
```

Joins the synced feed against department headcount to show spend-vs-budget offline.

### Catch a neglected direct report before a 1:1

```bash
bonusly-pp-cli recognition gap --manager me --days 30 --agent
```

Flags direct reports you haven't recognized in the last N days by joining the org tree against your own giving history.

### Forecast when your monthly allowance will forfeit

```bash
bonusly-pp-cli balance history --agent
```

Diffs locally-snapshotted balance history to project burn rate against the monthly reset.

## Auth Setup

Bonusly authenticates with a Personal Access Token (PAT), not OAuth. Any employee can mint one from their own Settings -> Services page (not the admin-only Company -> Integrations -> API & Tokens page) -- pick only the scopes this CLI needs (user:read, recognition:read, recognition:write, rewards:read; admin scopes won't even be offered to a non-admin account). Tokens expire after up to 365 days with email reminders 30 and 7 days out. If self-serve token generation isn't available on your account, `bonusly-pp-cli auth login --chrome` authenticates from a captured browser session instead.

Run `bonusly-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  bonusly-pp-cli org top --agent --select id,name,email
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
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

- Use `--home <dir>` for one invocation, or set `BONUSLY_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `BONUSLY_CONFIG_DIR`, `BONUSLY_DATA_DIR`, `BONUSLY_STATE_DIR`, `BONUSLY_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `BONUSLY_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `bonusly-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "bonusly": {
        "command": "bonusly-pp-mcp",
        "env": {
          "BONUSLY_HOME": "/srv/bonusly"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `BONUSLY_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `BONUSLY_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
bonusly-pp-cli recall "<user's question>" --agent
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
      "next_action": ["<trial command>", "bonusly-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `bonusly-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `bonusly-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `bonusly-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
bonusly-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
bonusly-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
bonusly-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
bonusly-pp-cli playbook amend \
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

`bonusly-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `BONUSLY_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
bonusly-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
bonusly-pp-cli feedback --stdin < notes.txt
bonusly-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `BONUSLY_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `BONUSLY_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
bonusly-pp-cli profile save briefing --json
bonusly-pp-cli --profile briefing org top
bonusly-pp-cli profile list --json
bonusly-pp-cli profile show briefing
bonusly-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `bonusly-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/bonusly/cmd/bonusly-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add bonusly-pp-mcp -- bonusly-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which bonusly-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   bonusly-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `bonusly-pp-cli <command> --help`.
