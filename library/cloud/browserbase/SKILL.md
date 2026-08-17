---
name: pp-browserbase
description: "Every Browserbase cloud feature, plus session lifecycle control, local history, and usage analytics no other tool has. Trigger phrases: `create a browser session`, `fetch this page`, `search the web for X`, `check my browserbase usage`, `list my browserbase sessions`, `use browserbase`, `run browserbase`."
author: "Som Samantray"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - browserbase-pp-cli
    install:
      - kind: go
        bins: [browserbase-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/cloud/browserbase/cmd/browserbase-pp-cli
---

# Browserbase — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `browserbase-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install browserbase --cli-only
   ```
2. Verify: `browserbase-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/browserbase/cmd/browserbase-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Manage the full Browserbase surface — sessions, projects, downloads, contexts, agents, functions — from one agent-native CLI. Track orphaned sessions, batch-fetch with rate-limit pacing, diff agent runs, and watch usage trends from a local SQLite store that compounds across syncs.

## When to Use This CLI

Use browserbase-pp-cli when you need to manage cloud headless browsers programmatically: create and clean up sessions, fetch pages or run web searches without writing browser code, track project usage, and manage agents or hosted functions. It is the right choice for automation scripts, CI pipelines, and AI agents that need structured, reproducible browser infrastructure operations.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI for interactive browser automation with DOM clicks and form fills — that needs a live CDP session; use `sessions run` to get a connectUrl and attach Playwright/Puppeteer.
- Do not use this CLI as a general web scraper for sites with heavy anti-bot protection — use the fetch endpoint's proxy/stealth options deliberately.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Session lifecycle control
- **`sessions orphans`** — Find running sessions that were never released (keepAlive orphans) and the runtime they're burning, then optionally stop them in batch. Requires `sync` first — an unsynced store returns an empty scan.

  _Reach for this when a crashed automation may have left billable sessions running — it turns a silent cost leak into a one-command cleanup._

  ```bash
  browserbase-pp-cli sessions orphans --older-than 15m --stop --json
  ```
- **`sessions run`** — Create a session, print its connect URL, and guarantee it is released on completion, timeout, or interrupt.

  _Use when you need a browser session for a script or agent without leaking keepAlive sessions on failure._

  ```bash
  browserbase-pp-cli sessions run --project 1fbe3566-db19-4010-9410-0ba94f0497ea --timeout 15m
  ```

### Agent-native fetch pipeline
- **`fetch batch`** — Fetch a list of URLs with rate-limit pacing and a resumable checkpoint, so large scrape jobs survive interruptions.

  _Reach for this when scraping a list of pages: it paces requests, skips already-fetched URLs, and reports per-URL status._

  ```bash
  browserbase-pp-cli fetch batch --file urls.txt --format markdown --resume --json
  ```
- **`agents runs diff`** — Compare two agent runs structurally — message sequences and final structured results — to see what changed between prompt iterations.

  _Reach for this when iterating on an agent's prompt or schema and you need to see exactly what changed between two runs._

  ```bash
  browserbase-pp-cli agents runs diff 52f6b13d-eb27-436d-86ff-356b2fd01697 2d310606-42fa-483c-9a7b-7102a85ddb09 --json
  ```

### Local state that compounds
- **`projects digest`** — See everything that ran in a project this week — sessions, agent runs, and downloads grouped by day — from the local store. Requires `sync` first; an unsynced store returns an empty digest.

  _Use when reviewing what a project actually did over a period, without clicking through the dashboard._

  ```bash
  browserbase-pp-cli projects digest --project 1fbe3566-db19-4010-9410-0ba94f0497ea --since 7d --json
  ```
- **`usage trend`** — Track per-project browserMinutes and proxyBytes over sync history and spot quota creep before the bill arrives. Requires `sync` first; an unsynced store returns an empty trend.

  _Use when you own the Browserbase bill and want to see usage direction, not just a snapshot._

  ```bash
  browserbase-pp-cli usage trend --project 1fbe3566-db19-4010-9410-0ba94f0497ea --since 30d --json
  ```
- **`web history`** — Review past fetch and search calls with cached results, and re-emit a cached response without re-hitting the API. Requires `sync` first; an unsynced store returns an empty history.

  _Reach for this when you need provenance for what was fetched, or want to re-run a prior result offline._

  ```bash
  browserbase-pp-cli web history --since 7d --type fetch --json
  ```

## Command Reference

**agents** — Manage agents

- `browserbase-pp-cli agents create` — Create a reusable agent. An agent defines a `systemPrompt` and `resultSchema` that guide its behavior for every run.
- `browserbase-pp-cli agents delete` — Delete an agent. Runs that already referenced this agent are unaffected.
- `browserbase-pp-cli agents get` — Retrieve an agent by ID.
- `browserbase-pp-cli agents list` — List agents across your account. Supports filtering by creation time.
- `browserbase-pp-cli agents runs-create` — Run a browser agent to complete the `task` by using web search and browser tooling.
- `browserbase-pp-cli agents runs-get` — Retrieve the current status and details of a run, including its result and associated session information.
- `browserbase-pp-cli agents runs-list` — List runs across your account. Supports filtering by status, by the agent they reference, and by creation time.
- `browserbase-pp-cli agents runs-messages` — Returns a paginated list of messages produced by a run, in chronological order, with the oldest messages first.
- `browserbase-pp-cli agents runs-stop` — Request that an in-progress run stop. The run winds down and transitions to `STOPPED`.
- `browserbase-pp-cli agents update` — Update an existing agent. Only the fields provided in the body are modified; omitted fields are left unchanged.

**certificates** — Manage certificates

- `browserbase-pp-cli certificates delete` — Delete a Certificate
- `browserbase-pp-cli certificates get` — Get a Certificate
- `browserbase-pp-cli certificates list` — List Certificates
- `browserbase-pp-cli certificates upload` — Upload a Certificate

**contexts** — Manage contexts

- `browserbase-pp-cli contexts create` — Create a Context
- `browserbase-pp-cli contexts delete` — Delete a Context
- `browserbase-pp-cli contexts get` — Get a Context

**downloads** — Manage downloads

- `browserbase-pp-cli downloads delete` — Delete a download file from storage and mark as deleted.
- `browserbase-pp-cli downloads get` — Get download metadata (Accept: application/json) or file content (Accept: application/octet-stream).
- `browserbase-pp-cli downloads list` — List all downloads for a session with optional filtering and pagination.

**extensions** — Manage extensions

- `browserbase-pp-cli extensions delete` — Delete an Extension
- `browserbase-pp-cli extensions get` — Get an Extension
- `browserbase-pp-cli extensions upload` — Upload an Extension

**fetch** — Manage fetch

- `browserbase-pp-cli fetch` — Fetch a page and return its content, headers, and metadata.

**functions** — Manage functions

- `browserbase-pp-cli functions builds-get` — Get a Function Build
- `browserbase-pp-cli functions builds-get-logs` — Get Function Build Logs
- `browserbase-pp-cli functions builds-list` — List Function Builds
- `browserbase-pp-cli functions get` — Get a Function
- `browserbase-pp-cli functions invocations-get` — Get an Invocation
- `browserbase-pp-cli functions invocations-get-logs` — Get Invocation Logs
- `browserbase-pp-cli functions list` — List Functions
- `browserbase-pp-cli functions versions-get` — Get a Function Version
- `browserbase-pp-cli functions versions-list-invocations` — List Invocations for a Function Version

**projects** — Manage projects

- `browserbase-pp-cli projects get` — Get a Project
- `browserbase-pp-cli projects list` — List Projects

**sessions** — Manage sessions

- `browserbase-pp-cli sessions create` — Create a Session
- `browserbase-pp-cli sessions get` — Get a Session
- `browserbase-pp-cli sessions list` — List Sessions
- `browserbase-pp-cli sessions update` — Update a Session

**websearch** — Manage websearch

- `browserbase-pp-cli websearch` — Perform a web search and return structured results.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
browserbase-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Clean up orphaned sessions

```bash
browserbase-pp-cli sessions orphans --older-than 15m --stop --json
```

Find and release sessions that were never explicitly stopped, preventing wasted browser minutes.

### Scrape a list of URLs safely

```bash
browserbase-pp-cli fetch batch --file urls.txt --format markdown --resume --json
```

Fetch many pages with rate-limit pacing and resumable progress.

### Weekly project review

```bash
browserbase-pp-cli projects digest --project 1fbe3566-db19-4010-9410-0ba94f0497ea --since 7d --json
```

See every session, agent run, and download in the project this week.

### Check usage trend

```bash
browserbase-pp-cli usage trend --project 1fbe3566-db19-4010-9410-0ba94f0497ea --since 30d --json
```

Spot quota creep before the bill arrives.

### Narrow a fetch response for agents

```bash
browserbase-pp-cli fetch --url https://news.ycombinator.com --format markdown --agent --select statusCode,markdown
```

Fetch a page and select only the fields an agent needs, saving context.

## Auth Setup

Set BROWSERBASE_API_KEY (your key looks like `bb_live_...`). Optionally set BROWSERBASE_PROJECT_ID to scope commands to a project.

Run `browserbase-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  browserbase-pp-cli agents list --agent --select agentId,createdAt,name
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

- Use `--home <dir>` for one invocation, or set `BROWSERBASE_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `BROWSERBASE_CONFIG_DIR`, `BROWSERBASE_DATA_DIR`, `BROWSERBASE_STATE_DIR`, `BROWSERBASE_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `BROWSERBASE_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `browserbase-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "browserbase": {
        "command": "browserbase-pp-mcp",
        "env": {
          "BROWSERBASE_HOME": "/srv/browserbase"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `BROWSERBASE_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `BROWSERBASE_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
browserbase-pp-cli recall "<user's question>" --agent
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
      "next_action": ["<trial command>", "browserbase-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `browserbase-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `browserbase-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `browserbase-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
browserbase-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
browserbase-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
browserbase-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
browserbase-pp-cli playbook amend \
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

`browserbase-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `BROWSERBASE_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
browserbase-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
browserbase-pp-cli feedback --stdin < notes.txt
browserbase-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `BROWSERBASE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `BROWSERBASE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
browserbase-pp-cli profile save briefing --json
browserbase-pp-cli --profile briefing agents list
browserbase-pp-cli profile list --json
browserbase-pp-cli profile show briefing
browserbase-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Async Jobs

For endpoints that submit long-running work, the generator detects the submit-then-poll pattern (a `job_id`/`task_id`/`operation_id` field in the response plus a sibling status endpoint) and wires up three extra flags on the submitting command:

| Flag | Purpose |
|------|---------|
| `--wait` | Block until the job reaches a terminal status instead of returning the job ID immediately |
| `--wait-timeout` | Maximum wait duration (default 10m, 0 means no timeout) |
| `--wait-interval` | Initial poll interval (default 2s; grows with exponential backoff up to 30s) |

Use async submission without `--wait` when you want to fire-and-forget; use `--wait` when you want one command to return the finished artifact.

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

1. **Empty, `help`, or `--help`** → show `browserbase-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/cloud/browserbase/cmd/browserbase-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add browserbase-pp-mcp -- browserbase-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which browserbase-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   browserbase-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `browserbase-pp-cli <command> --help`.
