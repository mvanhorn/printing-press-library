---
name: pp-seek
description: "Every SEEK search and job-detail feature, plus a local job history, salary distributions, and hiring trends no other SEEK tool has. Trigger phrases: `search seek for <role> jobs in <city>`, `what's the salary range for <role> on seek`, `any new jobs matching my saved searches`, `who is hiring <role> in <city>`, `seek hiring trends for <classification>`, `use seek`, `run seek`."
author: "Paul Siola"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - seek-pp-cli
    install:
      - kind: go
        bins: [seek-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/productivity/seek/cmd/seek-pp-cli
---

# SEEK — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `seek-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install seek --cli-only
   ```
2. Verify: `seek-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/seek/cmd/seek-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

seek-pp-cli searches Australian and New Zealand job listings, pulls full job details through SEEK's own JSON endpoints (never the bot-gated HTML), and keeps every job it has seen in a local SQLite store. On top of that it computes salary percentiles (salary), hiring-volume trends over time (trends), per-company hiring scans (company), and runs all your saved searches for new postings in one command (me new-jobs). No API key; no hosted scraper.

## When to Use This CLI

Use seek-pp-cli when an agent needs to search or monitor Australian/NZ job listings, pull structured job details, compute salary statistics, or track hiring volume over time. It caches every job it fetches to a local SQLite store, so salary and trends answer aggregate questions the SEEK site and every scraper cannot.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to submit a job application or edit a SEEK profile/resume — it is read-only against your account.
- Do not use it for job boards other than SEEK (Indeed, LinkedIn, JobStreet non-AU/NZ) — it only speaks to au.seek.com.
- Do not use it for the SEEK partner/hirer Job Posting API (developer.seek.com) — that is a different, credentialed product.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`salary`** — Get salary percentiles, a histogram, and the pay-disclosure rate for a role and location by sampling matching listings (which it also caches locally).

  _Reach for this instead of fetching hundreds of listings and computing pay statistics yourself._

  ```bash
  seek-pp-cli salary "registered nurse" --where "Melbourne VIC" --agent
  ```
- **`trends`** — See how job-listing volume for a classification, region, or work arrangement changes over time, bucketed from the listing dates of every job the CLI has cached, with period-over-period deltas.

  _Use this when the question is 'is hiring up or down', not 'what is open right now'._

  ```bash
  seek-pp-cli trends --by classification --since 90d --agent
  ```

### Cross-entity joins
- **`company`** — Profile one advertiser: their company details and review ratings plus every current opening and a local history of how many roles they've posted.

  _Reach for this for 'who is hiring for X' market scans and interview prep instead of scrolling a company's SEEK page._

  ```bash
  seek-pp-cli company "Atlassian" --active --agent
  ```
- **`me new-jobs`** — Run every one of your SEEK saved searches and return only the postings that aren't already in your local store, tagged by which search matched.

  _This is the daily-driver command for an active job hunt; use it instead of re-scrolling the SEEK site._

  ```bash
  seek-pp-cli me new-jobs --since 7d --agent
  ```

### Agent-native plumbing
- **`listings facets`** — Break a live query down by classification, work type, pay band, or region by tallying the sampled result pages — no local store needed.

  _Use this for a fast shape-of-the-market answer before committing to a full search or a sync._

  ```bash
  seek-pp-cli listings facets --keywords "data analyst" --where "Brisbane QLD" --group-by classification --agent
  ```
- **`classifications`** — Browse or resolve SEEK's numeric classification and subclassification IDs so filtered searches can target them precisely.

  _Call this first when you need a --classification value for listings search._

  ```bash
  seek-pp-cli classifications software
  ```

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 12 API entries from 60 total network entries
- Protocols: rest_json (95% confidence), graphql (95% confidence), ssr_embedded_data (80% confidence)
- Auth signals: cookie — cookies: SEEK_SESSION; none
- Generation hints: primary transport: standard_http against https://au.seek.com, search surface is REST JSON at /api/jobsearch/v5/search; job-detail and account surfaces are GraphQL at POST /graphql, graphql endpoint rejects introspection and enforces a real schema; jobDetails(id: ID!) and jobSearchV7(params: JobSearchV7QueryInput!) verified working with hand-written queries, AU and NZ share the au.seek.com host; switch via siteKey=AU-Main|NZ-Main and locale=en-AU|en-NZ, authenticated surface uses same-origin session cookies on .seek.com (no Authorization header, no CSRF token on read queries); emit auth login --chrome / press-auth cookie companion, requires_browser_auth for the me/* commands only; all jobs/* commands are unauthenticated
- Candidate command ideas: search — GET /api/jobsearch/v5/search — keyword+location+filter job search, the primary workflow; get — POST /graphql jobDetails(id) — full job description, salary, apply link; count — POST /graphql jobSearchV7 JobCountV7 — result count for a filter combination without fetching rows; recommended — POST /graphql JobDetailsRecommendedJobs(jobDetailsId) — similar jobs; saved-searches — viewer -> ApacSavedSearch{id,name,query,createdDate,newToYouCountLabel,subscribeToNewJobs} (cookie auth); saved-jobs — viewer.savedJobs(first) (cookie auth); applied-jobs — viewer.searchAppliedJobs / applied job history (cookie auth)
- Caveats: hand_written_queries: GraphQL operation bodies were reconstructed by the agent and verified against the live endpoint (HTTP 200); they are leaner than the SPA's persisted queries. jobDetailsPersonalised/GetMatchedQualities field selections were observed but not fully transcribed.; cookie_replay_unverified: Cookie auth confirmed working in-browser (viewer queries returned data). Replay outside the browser was not validated this run because session cookie values were deliberately not extracted; Phase 5 live smoke or first auth login --chrome will confirm.

## Command Reference

**listings** — Search and inspect SEEK job listings

- `seek-pp-cli listings` — Search job listings by keyword, location, and filters

**me** — Your SEEK account: saved searches, saved jobs, applied jobs (needs `auth login --chrome`)

- `seek-pp-cli me job-status` — Check which of the given job IDs you've saved or already applied to
- `seek-pp-cli me saved-jobs` — List jobs you've saved on SEEK
- `seek-pp-cli me saved-searches` — List your saved searches / job alerts


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
seek-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query. `--json` (and other machine formats) keep that exit-2 contract and write `{"matches":[]}` on stdout so agents can inspect the envelope without treating a miss as success.

## Recipes

### Daily new-jobs digest

```bash
seek-pp-cli me new-jobs --since 24h --agent
```

Runs every saved search and returns only postings not already in the local store.

### Salary benchmark for a role

```bash
seek-pp-cli salary "data engineer" --where "All Australia" --agent
```

Percentiles and disclosure rate over the listings the command samples and caches for that role and location.

### Competitor hiring snapshot

```bash
seek-pp-cli company "Canva" --agent --select openings.title,openings.location,openings.listingDate
```

All current openings for one advertiser with just the fields an agent needs from a large nested response.

### Is ICT hiring cooling?

```bash
seek-pp-cli trends --by classification --since 180d --agent
```

Period-over-period listing counts bucketed from the listing dates of jobs cached locally.

### Shape of a market before a full search

```bash
seek-pp-cli listings facets --keywords "registered nurse" --where "Perth WA" --group-by salary --agent
```

Per-facet counts tallied from the sampled result pages, zero extra rows fetched.

## Auth Setup

Job search, job details, salary data, and company profiles need no authentication. The me commands (saved searches, saved jobs, applied status) read your SEEK session: run `seek-pp-cli auth login --chrome` once while logged in to au.seek.com in Chrome and the CLI imports the session cookie. Nothing is written to your SEEK account.

Run `seek-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color`.

Global format flags share one contract on promoted, novel, sync, and `--deliver` paths:

- `--json` — one JSON document on stdout (sync progress events go to stderr)
- `--compact` — keep identity/status/timestamp fields; does not change the document vs stream shape
- `--csv` / `--plain` — tabular rows (collection envelopes unwrap to the row array)
- `--quiet` — one identity value per row, no envelope

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  seek-pp-cli listings --agent --select id,title,teaser
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

- Use `--home <dir>` for one invocation, or set `SEEK_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `SEEK_CONFIG_DIR`, `SEEK_DATA_DIR`, `SEEK_STATE_DIR`, `SEEK_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `SEEK_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `seek-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "seek": {
        "command": "seek-pp-mcp",
        "env": {
          "SEEK_HOME": "/srv/seek"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `SEEK_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `SEEK_HOME`, or `doctor` will not find credentials left under the former root.

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
seek-pp-cli recall "$QUERY" --agent
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
      "next_action": ["<trial command>", "seek-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `seek-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `seek-pp-cli learnings candidates` lists the full open set.

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

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately. Pass the query the same way as recall — argv/MCP, or file-then-`$QUERY`. Do not splice the question into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
seek-pp-cli teach --query "$QUERY" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
QUERY=$(cat /path/to/question.txt)
seek-pp-cli teach \
  --query "$QUERY" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
QUERY=$(cat /path/to/question.txt)
seek-pp-cli teach-playbook \
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
seek-pp-cli playbook amend \
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

`seek-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `SEEK_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
seek-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
seek-pp-cli feedback --stdin < notes.txt
seek-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `SEEK_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SEEK_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename). Binary-response commands write decoded payload bytes (not the base64 JSON envelope) and print a small JSON receipt on stdout; `--json`/`--csv` do not refuse when this sink is set. |
| `webhook:<url>` | POST the output body to the URL (`application/json`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

```
seek-pp-cli profile save briefing --json
seek-pp-cli --profile briefing listings
seek-pp-cli profile list --json
seek-pp-cli profile show briefing
seek-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `seek-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/seek/cmd/seek-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add seek-pp-mcp -- seek-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which seek-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   seek-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `seek-pp-cli <command> --help`.
