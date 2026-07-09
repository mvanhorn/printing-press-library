---
name: pp-juneoven
description: "Control a June oven from the terminal — self-pair, preheat, adjust, watch live telemetry and camera frames, and cancel, over June's cloud with agent-native JSON. No HomeKit, no June app. Trigger phrases: `preheat my june oven to 350`, `what's my june oven doing`, `cancel the oven`, `set a timer on the june oven`, `watch the june oven`, `is the oven preheated`, `use juneoven`, `run juneoven`."
author: "Matt Van Horn"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - juneoven-pp-cli
    install:
      - kind: go
        bins: [juneoven-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/devices/juneoven/cmd/juneoven-pp-cli
---

# June Oven — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `juneoven-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install juneoven --cli-only
   ```
2. Verify: `juneoven-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/juneoven/cmd/juneoven-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Direct pair/preheat/watch/cancel control with no HomeKit and no June app, plus local history (record, log, curve, preheat-stats), workflow primitives (ready, eta), and named repeatable cooks (repeat). All agent-native JSON.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Cook workflow primitives
- **`ready`** — Wait for the oven to reach its target temperature, then exit 0 (non-zero on timeout).

  _Lets an agent synchronously gate the next cook step on preheat completion instead of busy-waiting._

  ```bash
  juneoven-pp-cli ready --timeout 20 --agent
  ```
- **`eta`** — Non-blocking estimate of time until the oven reaches target, from the live climb rate.

  _An agent can sequence the next action off a returned ETA without blocking._

  ```bash
  juneoven-pp-cli eta --agent
  ```
- **`repeat`** — Save a named mode+temp(+timer) preset in local SQLite and replay it with one word.

  _Collapses a repeated weekly cook into a single memorable command._

  ```bash
  juneoven-pp-cli repeat <name>
  ```

### Local history the cloud never keeps
- **`record`** — Capture the current cook (session + telemetry/camera activity) into local SQLite.

  _Turns ephemeral live cooks into a queryable local dataset the cloud never stores._

  ```bash
  juneoven-pp-cli record --label sourdough --agent
  ```
- **`log`** — List past cooks (mode, target, duration, outcome) from local history.

  _Answers historical questions about the oven that are impossible through June's API._

  ```bash
  juneoven-pp-cli log --since 7d --agent
  ```
- **`curve`** — Export one recorded cook's temperature/progress samples as JSON or CSV.

  _Gives a plottable series for one cook, for analysis or piping._

  ```bash
  juneoven-pp-cli curve 3 --format csv
  ```
- **`preheat-stats`** — Median/fastest/slowest time-to-target per cook over recorded cooks.

  _Shows how your specific oven performs over time and whether it is degrading._

  ```bash
  juneoven-pp-cli preheat-stats --cook bake --agent
  ```

## Recipes

### Preheat and record

```bash
juneoven-pp-cli preheat --temp 450 && juneoven-pp-cli record --label pizza
```

Start a cook and capture its session to local history.

### Weekly cook by name

```bash
juneoven-pp-cli repeat sunday-roast
```

Replay a saved mode+temp+timer preset in one command.

### Export a cook's curve

```bash
juneoven-pp-cli curve 3 --format csv --agent --select ts,cavity_f
```

Pull one session's temperature series as CSV for plotting.

## Command Reference

Control a June oven directly over June's cloud. No HomeKit, no June phone app. Temperatures are °F by default; add `--celsius` where a temperature is taken.

- `juneoven-pp-cli pair` — Pair this CLI with your oven by typing an 8-digit code on its screen. Run once; credentials are stored at 0600 and control the oven.
- `juneoven-pp-cli status` — Connection state, idle/active, and current target temperature.
- `juneoven-pp-cli preheat --temp 350` — Start a cook (`--mode bake` or `roast`). Reports the oven's ack (`success`/`not-allowed`).
- `juneoven-pp-cli temp --temp 375` — Change the target of the active cook (June may reject a live retarget with `not-allowed`).
- `juneoven-pp-cli timer --minutes 10` — Set a cook timer.
- `juneoven-pp-cli cancel` — Stop the active cook. `not-allowed` when already idle (reported as a normal result, not an error).
- `juneoven-pp-cli watch --seconds 120` — Stream live telemetry and camera-frame events as JSON lines until the cook ends or the timeout elapses.
- `juneoven-pp-cli cam --timeout 15` — Print the next interior camera frame's signed URL (frames are typically pushed only during an active cook).

Every command supports `--json` / `--agent` for structured output. Command acks come back as `{"action","acked","status"}`; `status` is the oven's own verdict.

### Unique Capabilities

June's cloud keeps only live state and no history. These commands add a durable local record (SQLite) and workflow primitives no other June tool offers:

- `juneoven-pp-cli record --label sourdough` — Capture the current cook (session + activity) into local history. The only durable record; the cloud stores none.
- `juneoven-pp-cli log --since 7d` — List past cooks (mode, target, duration, outcome). Impossible via June's API.
- `juneoven-pp-cli repeat <name>` — Save (`--save <name> --mode --temp --timer`) and replay named cooks in one word. `--list` shows saved presets.
- `juneoven-pp-cli ready --timeout 20` — Block until the oven reaches target, typed exit (0 ready, 4 timeout). Gate an agent step on preheat completion.
- `juneoven-pp-cli eta` — Non-blocking predicted time-to-ready from the live climb rate.
- `juneoven-pp-cli curve <session-id> --format csv` — Export one recorded cook's temperature curve.
- `juneoven-pp-cli preheat-stats --cook bake` — Median/fastest/slowest time-to-target per cook over recorded cooks.

**Firmware note (important):** `ready`, `eta`, `curve`, and `preheat-stats` need an oven that streams live cavity-temperature telemetry. Some firmware (e.g. the `penguin` model) streams only interior camera frames and no temperature; on those ovens these four commands return an honest empty/notice result rather than data, while `record`, `log`, and `repeat` work fully. `status`, `preheat`, `temp`, `timer`, `cancel`, `watch`, and `cam` work on all paired ovens.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
juneoven-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Auth is direct pairing: run `juneoven-pp-cli pair` once and type an 8-digit code on the oven. A signing key and access token are stored at ~/.config/juneoven-pp-cli/identity.json (0600).

Run `juneoven-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  juneoven-pp-cli status --agent --select state,target_f
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
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

- Use `--home <dir>` for one invocation, or set `JUNEOVEN_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `JUNEOVEN_CONFIG_DIR`, `JUNEOVEN_DATA_DIR`, `JUNEOVEN_STATE_DIR`, `JUNEOVEN_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `JUNEOVEN_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `juneoven-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "juneoven": {
        "command": "juneoven-pp-mcp",
        "env": {
          "JUNEOVEN_HOME": "/srv/juneoven"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `JUNEOVEN_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `JUNEOVEN_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
juneoven-pp-cli recall "<user's question>" --agent
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
      "next_action": ["<trial command>", "juneoven-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `juneoven-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `juneoven-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `juneoven-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
juneoven-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
juneoven-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
juneoven-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
juneoven-pp-cli playbook amend \
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

`juneoven-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `JUNEOVEN_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
juneoven-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
juneoven-pp-cli feedback --stdin < notes.txt
juneoven-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `JUNEOVEN_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `JUNEOVEN_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
juneoven-pp-cli profile save briefing --json
juneoven-pp-cli --profile briefing devices associated mock-value
juneoven-pp-cli profile list --json
juneoven-pp-cli profile show briefing
juneoven-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `juneoven-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/devices/juneoven/cmd/juneoven-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add juneoven-pp-mcp -- juneoven-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which juneoven-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   juneoven-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `juneoven-pp-cli <command> --help`.
