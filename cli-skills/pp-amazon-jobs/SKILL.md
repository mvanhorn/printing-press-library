---
name: pp-amazon-jobs
description: "Every Amazon.jobs feature, plus a local job store, new-since diffing, and aggregation Amazon's own site can't do — no login required. Trigger phrases: `find amazon jobs`, `search amazon.jobs for`, `new amazon reqs since yesterday`, `which teams at amazon are hiring for`, `amazon jobs in seattle`, `use amazon-jobs`, `run amazon-jobs`."
author: "qazmataz"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - amazon-jobs-pp-cli
    install:
      - kind: go
        bins: [amazon-jobs-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/productivity/amazon-jobs/cmd/amazon-jobs-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/productivity/amazon-jobs/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Amazon Jobs — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `amazon-jobs-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install amazon-jobs --cli-only
   ```
2. Verify: `amazon-jobs-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/amazon-jobs/cmd/amazon-jobs-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

amazon-jobs turns Amazon's careers site from a page you refresh into a queryable, watchable dataset. It uses the same unauthenticated JSON the site does, so there is no API key and no scraping fragility. Beyond search and filters, it keeps a local SQLite mirror that powers `new` (reqs unseen since your last check), `stats` (counts by city/team/category the empty server facets can't give), and `skills` (which teams demand a given qualification).

## When to Use This CLI

Use amazon-jobs when a task involves finding, filtering, tracking, or analyzing open roles at Amazon or AWS: keyword and location search, watching for newly-posted reqs over time, or aggregating the open-req landscape by city, team, or demanded skill. It is ideal for agents doing recurring labor-market research because output is deterministic JSON and the local store answers questions a single API call cannot.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI for job boards other than Amazon.jobs — it only queries Amazon's careers API.
- Do not use it to submit an application; it is read-only and has no apply/authenticated workflow.
- Do not use it for general AWS service management — it is unrelated to the AWS SDK or AWS APIs.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds

- **`new`** — See only the Amazon reqs that appeared since you last synced a saved search — no more re-scanning the whole list every morning.

  _Reach for this when a user tracks a role over time and wants the delta, not the full list._

  ```bash
  amazon-jobs-pp-cli new sde-seattle --agent
  ```
- **`save`** — Persist a named query plus its filters and diff cursor so a search and its new-since state survive between runs.

  _Use this to set up a repeatable watch that `new` and `sync` then operate on by name._

  ```bash
  amazon-jobs-pp-cli save sde-seattle "software engineer" --city Seattle --country USA
  ```

### Aggregation the API can't do

- **`stats`** — Count synced reqs grouped by city, state, team, or category — the aggregation Amazon's own empty facets[] never returns.

  _Use this to answer 'which cities/teams have the most open reqs' without paging every result._

  ```bash
  amazon-jobs-pp-cli stats --by city --agent
  ```
- **`skills`** — Rank teams and cities by how many synced reqs demand a given skill keyword in their basic/preferred qualifications.

  _Reach for this for labor-market questions like 'who is hiring for X skill' rather than retrieving individual reqs._

  ```bash
  amazon-jobs-pp-cli skills Rust --agent
  ```

### Honest client-side filtering

- **`find`** — Filter live results by intern, manager, university, and schedule type — fields the .json endpoint silently ignores as server params.

  _Use these flags when a user wants senior-IC, non-intern, or schedule-specific roles that Amazon can't filter server-side._

  ```bash
  amazon-jobs-pp-cli find "software engineer" --manager=false --intern=false --agent
  ```

### True recency: use `posted_date`, never `updated_time`

- **`find --posted-within`** — Filter on the real posting date. Accepts `24h`, `3d`, `7d`, `2w`.

  **This is a correctness trap, not a convenience flag.** `updated_time` is a relative string ("about 21 hours") that tracks the last edit or re-index — not the posting. Measured on 1000 live reqs: 514 were posted more than 14 days ago, and **all 514** reported `updated_time` under 48 hours; the worst case was posted August 2025 and read "about 21 hours". If a user asks for "jobs posted recently" or wants to "apply as fast as possible", filter with `--posted-within` and report `posted_date`. Never present `updated_time` as recency.

  Rows where the two disagree badly (posted >14 days ago, updated <48h) are marked `(edited)` in human output and carry `"updated_diverged": true` in JSON — surface that caveat to the user rather than dropping it.

  `posted_date` is day-granular; the API has no sub-day posting timestamp. `--posted-within 7d` therefore means "posted on or after (today − 7 days)", inclusive by whole date. Do not promise the user hour-level precision on posting time — it does not exist upstream.

  ```bash
  amazon-jobs-pp-cli find "program manager" --country GBR --posted-within 7d --agent
  ```

- **`find --description-contains` / `--description-not-contains`** — Case-insensitive regex over `description` + `basic_qualifications` + `preferred_qualifications`, HTML stripped before matching.

  _Reach for this whenever the user's constraint is buried in prose rather than exposed as a facet: visa/sponsorship eligibility, relocation support (check both directions — "Relocation assistance is NOT provided" and "Relocation benefits are offered" both exist), or language requirements (Mandarin, Japanese N1) that silently disqualify._

  Invalid regex falls back to a literal match, so `C++` works as typed. These matches are rare (sponsorship language appears in roughly 16 of 1000 reqs, relocation language in about 2), so raise `--max-scan-pages` when nothing comes back — the "no matching jobs" note tells you how many were scanned.

  ```bash
  amazon-jobs-pp-cli find "" --country SGP --description-not-contains "without sponsorship" --max-scan-pages 10 --agent
  ```

## Command Reference

**postings** — Search Amazon job listings

- `amazon-jobs-pp-cli postings` — Search Amazon job listings by keyword, location, and sort order


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
amazon-jobs-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Track new SDE roles in Seattle each morning

```bash
amazon-jobs-pp-cli save sde-seattle "software engineer" --city Seattle --country USA && amazon-jobs-pp-cli new sde-seattle --agent
```

Persist a named search once, then run `new` to see only reqs that appeared since your last sync.

### Agent-native search narrowed to key fields

```bash
amazon-jobs-pp-cli find "solutions architect" --country USA --agent --select title,location,posted_date,job_path
```

Amazon reqs carry huge description/qualification text; --select trims the payload to the fields an agent needs.

### Where is AWS hiring most right now?

```bash
amazon-jobs-pp-cli sync aws --max-pages 10 && amazon-jobs-pp-cli stats --by city
```

Mirror AWS reqs locally, then aggregate by city — counts the empty server facets never return.

### Which teams demand a specific skill

```bash
amazon-jobs-pp-cli sync engineer --max-pages 10 && amazon-jobs-pp-cli skills Rust --agent
```

Scan synced qualification text for a keyword and rank teams/cities by demand.

### Senior IC roles only (no manager, no intern)

```bash
amazon-jobs-pp-cli find "software engineer" --country USA --manager=false --intern=false
```

Client-side NULL-safe filters for fields Amazon can't filter server-side.

### Only reqs actually posted this week

```bash
amazon-jobs-pp-cli find "program manager" --country GBR --posted-within 7d --max-scan-pages 10 --agent
```

Filters on the true `posted_date`. Any row carrying `"updated_diverged": true` was posted long ago and merely re-indexed — do not describe it to the user as newly posted.

### Screen out roles that won't sponsor a visa

```bash
amazon-jobs-pp-cli find "" --country SGP --description-not-contains "without sponsorship" --posted-within 2w --max-scan-pages 10 --agent
```

Sponsorship, relocation, and language constraints exist only in description prose. Combine the text filter with a recency window to build a shortlist worth applying to.

### Sweep several countries for fresh reqs

```bash
for c in GBR IRL LUX JPN SGP SAU ARE; do
  amazon-jobs-pp-cli find "" --country "$c" --posted-within 7d --max-scan-pages 10 --agent
done
```

The API takes one country per request, so multi-market searches fan out client-side. Pair with `save`/`new` when the sweep should run repeatedly and only report reqs unseen since last time.

## Auth Setup

No authentication required.

Run `amazon-jobs-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  amazon-jobs-pp-cli postings --agent --select id,name,status
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

- Use `--home <dir>` for one invocation, or set `AMAZON_JOBS_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `AMAZON_JOBS_CONFIG_DIR`, `AMAZON_JOBS_DATA_DIR`, `AMAZON_JOBS_STATE_DIR`, `AMAZON_JOBS_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `AMAZON_JOBS_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `amazon-jobs-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "amazon-jobs": {
        "command": "amazon-jobs-pp-mcp",
        "env": {
          "AMAZON_JOBS_HOME": "/srv/amazon-jobs"
        }
      }
    }
  }
  ```

Override the API host with `AMAZON_JOBS_BASE_URL` (or the `base_url` key in `config.toml`) when the default `https://www.amazon.jobs` is not reachable from this machine. If `doctor` reports `cannot resolve host`, the fault is the local resolver, not the API or your arguments: some routers and split-DNS/VPN setups refuse the `amazon.jobs` zone. Do not retry the command or rewrite the query — unresolvable hosts fail fast by design and will keep failing. Confirm with `host amazon.jobs` versus `host amazon.jobs 1.1.1.1`, then switch resolver, drop the VPN, or set `AMAZON_JOBS_BASE_URL`. Use `--dry-run` on `find` to print the exact URL and client-side filters the command would use without sending the request.

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `AMAZON_JOBS_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `AMAZON_JOBS_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
amazon-jobs-pp-cli recall "<user's question>" --agent
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
      "next_action": ["<trial command>", "amazon-jobs-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `amazon-jobs-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `amazon-jobs-pp-cli learnings candidates` lists the full open set.

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
amazon-jobs-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
amazon-jobs-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
amazon-jobs-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
amazon-jobs-pp-cli playbook amend \
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

`amazon-jobs-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `AMAZON_JOBS_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
amazon-jobs-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
amazon-jobs-pp-cli feedback --stdin < notes.txt
amazon-jobs-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `AMAZON_JOBS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `AMAZON_JOBS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
amazon-jobs-pp-cli profile save briefing --json
amazon-jobs-pp-cli --profile briefing postings
amazon-jobs-pp-cli profile list --json
amazon-jobs-pp-cli profile show briefing
amazon-jobs-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `amazon-jobs-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/amazon-jobs/cmd/amazon-jobs-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add amazon-jobs-pp-mcp -- amazon-jobs-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which amazon-jobs-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   amazon-jobs-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `amazon-jobs-pp-cli <command> --help`.
