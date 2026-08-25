---
name: pp-synology
description: "Printing Press CLI for Synology. Query and operate Synology DSM NAS units - system health, storage, SMART, shares, users"
author: "smoochy"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - synology-pp-cli
---

# Synology — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `synology-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install synology --cli-only
   ```
2. Verify: `synology-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

DSM's WebAPI is namespaced RPC that answers HTTP 200 even when it fails, and its available namespaces differ per NAS depending on which packages are installed. This CLI handles the dialect for you: expired sessions renew themselves, DSM error codes become actionable messages, and 'session apis' tells you what your own NAS actually exposes. Everything is JSON-first, so '--agent' output pipes straight into jq or an agent's context.

## When Not to Use This CLI

Do not activate this CLI to create, modify or delete users, groups, shared folders, NFS permissions or any DSM setting, and do not use it for Container Manager or Download Station. Everything outside File Station is a read surface: inspection, export, sync and analysis. The one exception is the `files` group, which ships DSM's own File Station operations (`mkdir`, `rename`, and the copy, move and delete task commands).

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Knowing what this NAS actually offers
- **`session apis`** — List every API namespace this particular NAS exposes, with its version range and CGI path.

  _Run this first when a call fails with DSM error 102 - it tells you whether the namespace exists on this NAS at all or whether the package is simply not installed._

  ```bash
  synology-pp-cli session apis --agent --select data
  ```
- **`storage smart`** — Read a drive's raw SMART attribute table; 'storage smart-schedule' reads the scheduled SMART test configuration.

  _Reach for this when asked whether a disk is failing - it is the only machine-readable path to per-attribute drive health on a Synology NAS._

  ```bash
  synology-pp-cli storage smart --device sata1 --agent
  ```

## Recipes

### Drive health at a glance

```bash
synology-pp-cli storage disks --agent --select disks.id,disks.status,disks.temp,disks.model
```

The disks payload is deeply nested and verbose; --select narrows it to the four fields a health check needs.

### Is this namespace even available

```bash
synology-pp-cli session apis --agent
```

Needs no session and answers the question DSM error 102 raises: does this NAS have that API at all.

### Recent errors from the system log

```bash
synology-pp-cli log --level err --limit 50 --agent
```

Filters DSM's syslog server-side by level, so only the rows worth reading come back.

### Browse a share without mounting it

```bash
synology-pp-cli files list --folder-path /volume1/media --agent
```

File Station browsing over the API, so no SMB mount and no credentials on the filesystem.

### Mirror once, query many times

```bash
synology-pp-cli sync && synology-pp-cli search 'volume1' --agent
```

Sync writes every synced resource into local SQLite; search then answers offline without touching the NAS again.

## Command Reference

**files** — File Station browsing, search and file operations

- `synology-pp-cli files copy-start` — Start a background copy or move and return its task id
- `synology-pp-cli files copy-status` — Report the progress of a running copy or move
- `synology-pp-cli files copy-stop` — Stop a running copy or move
- `synology-pp-cli files delete-start` — Start a background delete and return its task id
- `synology-pp-cli files delete-status` — Report the progress of a running delete
- `synology-pp-cli files delete-stop` — Stop a running delete
- `synology-pp-cli files download` — Download a file or folder
- `synology-pp-cli files info` — Show File Station capabilities and limits
- `synology-pp-cli files list` — List the contents of one folder
- `synology-pp-cli files mkdir` — Create a folder
- `synology-pp-cli files rename` — Rename a file or folder
- `synology-pp-cli files search-results` — Read the results of a running or finished search
- `synology-pp-cli files search-start` — Start a background search and return its task id
- `synology-pp-cli files search-stop` — Stop a running search and release its task
- `synology-pp-cli files shares` — List the shared folders File Station can browse
- `synology-pp-cli files stat` — Show metadata for one file or folder

**folder** — Shared folders and their permissions

- `synology-pp-cli folder get` — Show one shared folder in detail
- `synology-pp-cli folder list` — List shared folders
- `synology-pp-cli folder permissions` — List the accounts and groups that hold permissions on one shared folder

**group** — Local groups

- `synology-pp-cli group list` — List local groups
- `synology-pp-cli group members` — List the accounts that belong to one group

**log** — DSM system log

- `synology-pp-cli log` — Read the DSM system log with filtering by level, date range and keyword

**nfs** — NFS service state

- `synology-pp-cli nfs` — Show whether the NFS service is enabled and which protocol versions it offers

**package** — Installed DSM packages

- `synology-pp-cli package` — List installed packages with their version and running state

**session** — DSM session handshake

- `synology-pp-cli session apis` — Enumerate every API namespace the NAS exposes, with its version range and CGI path
- `synology-pp-cli session login` — Log in to DSM and obtain a session id plus a SynoToken
- `synology-pp-cli session logout` — Invalidate the current session id

**storage** — Volumes, pools, disks and SMART data

- `synology-pp-cli storage disks` — List physical disks with model, serial number, temperature and health status
- `synology-pp-cli storage esata` — List attached eSATA storage devices
- `synology-pp-cli storage luns` — List iSCSI LUNs
- `synology-pp-cli storage overview` — Show the complete storage picture DSM Storage Manager renders, covering disks, pools and volumes in one call
- `synology-pp-cli storage pools` — List storage pools with their RAID level and member disks
- `synology-pp-cli storage smart` — Show the full SMART attribute table for one disk
- `synology-pp-cli storage smart-schedule` — List scheduled SMART tests
- `synology-pp-cli storage usb` — List attached USB storage devices
- `synology-pp-cli storage volumes` — List volumes with size, used space and status

**system** — DSM system identity, health and load

- `synology-pp-cli system dsm` — Show DSM product identity as the web UI reports it
- `synology-pp-cli system health` — Show the aggregate system health verdict DSM shows on its dashboard
- `synology-pp-cli system info` — Show model, serial number, firmware version, uptime and temperature
- `synology-pp-cli system network` — Show hostname, DNS, gateway and every network interface
- `synology-pp-cli system services` — List DSM services and whether each one is enabled
- `synology-pp-cli system ups` — Show the state of an attached uninterruptible power supply
- `synology-pp-cli system utilization` — Show current CPU, memory, disk and network utilization

**user** — Local user accounts

- `synology-pp-cli user get` — Show one local user account in detail
- `synology-pp-cli user list` — List local user accounts


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
synology-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

DSM authenticates with an account and password, not an API key. 'session login --account <user> --passwd <password>' exchanges them for a session id and a SynoToken, both of which are persisted locally; the password never is. Two-factor accounts pass '--otp-code' on the first login and reuse the returned device id afterwards. Set SYNOLOGY_ACCOUNT and SYNOLOGY_PASSWORD so an expired session renews itself mid-run. Point the CLI at your NAS with SYNOLOGY_BASE_URL, and add '--insecure' if it still uses DSM's own self-signed certificate on port 5001.

Run `synology-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  synology-pp-cli files list --folder-path /volume1/media --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only outside File Station** - system, storage, SMART, shares, users, groups and logs are read surfaces; only the `files` group mutates remote state, through DSM's own File Station task operations

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

- Use `--home <dir>` for one invocation, or set `SYNOLOGY_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `SYNOLOGY_CONFIG_DIR`, `SYNOLOGY_DATA_DIR`, `SYNOLOGY_STATE_DIR`, `SYNOLOGY_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `SYNOLOGY_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `synology-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "synology": {
        "command": "synology-pp-mcp",
        "env": {
          "SYNOLOGY_HOME": "/srv/synology"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `SYNOLOGY_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `SYNOLOGY_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
# Never interpolate the question into a command line, and never embed it in the
# command text at all - not in quotes and not in a heredoc, whose delimiter the
# question can itself contain. Write the question verbatim to a file with your
# file-writing tool (no shell involved), then read that file into a variable.
# Command substitution on a file only ever yields data: the shell never parses
# the file's bytes as syntax, so there is no boundary for the question to escape.
QUERY=$(cat /path/to/question.txt)
synology-pp-cli recall "$QUERY" --agent
```

Every `recall`, `teach`, `teach-playbook` and `playbook amend` example below reuses that `$QUERY` variable, and any other user-controlled string (a `--add-note` correction, a `learnings forget` query) is written to its own file and read in the same way.

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
      "next_action": ["<trial command>", "synology-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `synology-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `synology-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `synology-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
synology-pp-cli teach --query "$QUERY" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
synology-pp-cli teach \
  --query "$QUERY" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
synology-pp-cli teach-playbook \
  --query "$QUERY" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
# Write the correction verbatim to a file with your file-writing tool, then:
NOTE=$(cat /path/to/note.txt)
synology-pp-cli playbook amend \
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

`synology-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `SYNOLOGY_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
synology-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
synology-pp-cli feedback --stdin < notes.txt
synology-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `SYNOLOGY_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SYNOLOGY_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
synology-pp-cli profile save briefing --json
synology-pp-cli --profile briefing files list --folder-path /volume1/media
synology-pp-cli profile list --json
synology-pp-cli profile show briefing
synology-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `synology-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add synology-pp-mcp -- synology-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which synology-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   synology-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `synology-pp-cli <command> --help`.
