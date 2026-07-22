---
name: pp-bambu
description: "Agent-ready Bambu print monitoring with exact plate previews and start/finish payloads. Trigger phrases: `what is my Bambu printer doing`, `post when this print starts and finishes`, `send this Bambu print to Discord`, `show my Bambu plate preview`, `when will the Bambu print finish`, `monitor my Bambu print`."
author: "Todd Dailey"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - bambu-pp-cli
    install:
      - kind: go
        bins: [bambu-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/devices/bambu/cmd/bambu-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/devices/bambu/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Bambu Lab — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `bambu-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install bambu --cli-only
   ```
2. Verify: `bambu-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/bambu/cmd/bambu-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

For people wiring an agent or code to monitor a Bambu print and post updates to Discord, a webhook, or another automation. Use normal local access for live MQTT status, exact-current-job 3MF weight and preview, provider-neutral start/finish events, and local history; Developer Mode, LAN Only Mode, Bambu Cloud, printer control, and farm management are intentionally unsupported.

## When to Use This CLI

Use this CLI when an agent or automation needs structured local truth about a Bambu print, including progress, ETA, AMS/HMS state, exact plate metadata, a model preview, and start/finish payloads. For printer control, LAN Only Mode, Developer Mode, camera walls, or farm management, use Bambuddy or another control-oriented project.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI for printer control, print dispatch, movement, heating, firmware, camera control, or AMS writes.
- Do not use it with Developer Mode, LAN Only Mode, Bambu Cloud, remote relays, or farm-management workflows.
- Do not use it for slicing, mesh repair, MakerWorld browsing, or spool inventory management.
- Do not expose the local access code in flags, logs, artifacts, chat messages, or MCP calls.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### History that improves live decisions
- **`job eta`** — Correct the printer's finish estimate using the error pattern from prior runs of the same job.

  _Use this when an automation or person needs a finish time grounded in this printer's observed history._

  ```bash
  bambu-pp-cli job eta --agent
  ```
- **`history failure-correlations`** — Rank the printer, filament, plate, firmware, speed, and temperature contexts most associated with failed jobs.

  _Use this when repeated failures need evidence-backed triage rather than inspection of one job._

  ```bash
  bambu-pp-cli history failure-correlations --since 30d --agent
  ```
- **`job timeline`** — Reconstruct one print's stages, pauses, layers, temperature recovery, and errors as an ordered timeline.

  _Use this to explain where one print slowed, paused, recovered, or failed._

  ```bash
  bambu-pp-cli job timeline --latest --agent
  ```
- **`job repeats`** — Compare duration, pauses, material, errors, and outcomes across repeated runs of the same plate.

  _Use this to judge whether a recurring print is stable and repeatable._

  ```bash
  bambu-pp-cli job repeats "Colored Accents" --agent
  ```

### Material-aware operation
- **`ams runway`** — Estimate whether the active AMS tray can cover the current plate's remaining weight, with explicit unknown output when tray estimates or multi-material mapping are ambiguous.

  _Use this before leaving a long print unattended or deciding which spool needs attention._

  ```bash
  bambu-pp-cli ams runway --agent
  ```

### Integration resilience
- **`printer field-diff`** — Compare first and latest persisted redacted MQTT schemas for added, removed, and type-changed fields across a selected window.

  _Use this to inspect structural report changes between locally persisted observations._

  ```bash
  bambu-pp-cli printer field-diff --since 7d --agent
  ```

## Command Reference

**observations** — Locally persisted, redacted printer observations collected from LAN status and event commands.

- `bambu-pp-cli observations` — List locally persisted printer observations.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
bambu-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Hand-written Extensions

These commands are declared by the spec author and require separate hand-written wiring; the generator does not emit Cobra registration for them. They are listed here for discoverability and are intentionally outside `## Command Reference` so the verify-skill unknown-command check does not treat them as generator-owned paths.

- `bambu-pp-cli discover` — Discover Bambu printers on the private LAN with SSDP and serial filtering.
- `bambu-pp-cli profile` — Manage secret-safe multi-printer profiles and environment precedence.
- `bambu-pp-cli printer status` — Fetch a fresh normalized MQTT pushall status snapshot.
- `bambu-pp-cli printer raw` — Return the complete redacted MQTT report including unknown firmware fields.
- `bambu-pp-cli events watch` — Emit provider-neutral start and terminal print lifecycle payloads as NDJSON.
- `bambu-pp-cli events monitor` — Monitor one print and automatically retain its event log and preview assets.
- `bambu-pp-cli printer watch` — Stream normalized printer snapshots and restart-safe lifecycle transitions.
- `bambu-pp-cli printer temperatures` — Show current and target bed chamber nozzle and AMS temperatures.
- `bambu-pp-cli printer fans` — Show available part auxiliary chamber heatbreak and airduct fan state.
- `bambu-pp-cli printer capabilities` — Show model-aware lights speed network door storage firmware nozzle and tool capabilities.
- `bambu-pp-cli ams status` — Show AMS units trays materials colors RFID humidity temperature and remaining estimates.
- `bambu-pp-cli ams active` — Resolve the active AMS tray or external spool with Bambu tray semantics.
- `bambu-pp-cli ams services` — Show read-only AMS drying and filament backup service state.
- `bambu-pp-cli printer health` — Show normalized HMS messages and print errors with history.
- `bambu-pp-cli job current` — Show current job IDs plate progress ETA layers stages speed file and timing.
- `bambu-pp-cli job objects` — Show printable and skipped object identities for the current job.
- `bambu-pp-cli job metadata` — Read exact-current-job bounded 3MF plate object and material metadata over FTPS.
- `bambu-pp-cli job thumbnail` — Write the validated current plate preview image to an explicit output path.
- `bambu-pp-cli printer services` — Show queue upload upgrade camera and timelapse operational flags.
- `bambu-pp-cli files list` — List printer files over certificate-verified implicit FTPS.
- `bambu-pp-cli files download` — Download a bounded printer file to an explicit safe local path.
- `bambu-pp-cli history` — Query locally persisted jobs outcomes durations transitions and observations.
- `bambu-pp-cli maintenance` — Track usage-derived maintenance due state forecasts and completion records.
- `bambu-pp-cli analytics` — Run printer and job analytics over the local observation store.
- `bambu-pp-cli job eta` — Forecast calibrated completion time and a historical error band.
- `bambu-pp-cli ams runway` — Estimate loaded filament surplus or shortfall for the current plate.
- `bambu-pp-cli job repeats` — Compare persisted executions of the same printable job.
- `bambu-pp-cli printer field-diff` — Detect added removed type-changed and stale firmware report fields.
- `bambu-pp-cli history failure-correlations` — Correlate failed jobs with printer filament plate firmware speed and temperature context.
- `bambu-pp-cli job timeline` — Reconstruct stages pauses layers thermal recovery and errors for one print.

## Recipes

### Automation-ready print events

```bash
bambu-pp-cli events monitor --agent
```

Run this before starting the print. The command waits for one print, streams its start payload with available embedded 3MF project/profile titles, object names, weight, and preview, exits after the terminal payload, and stores the same NDJSON plus any preview in a private timestamped data directory. The matching terminal payload reuses that enriched identity without repeating the attachment. `job.name` prefers the embedded 3MF project title, then a sole printable object's extension-free name; `job.source_name` preserves the printer label, while `job.project_name`, `job.profile_name`, and `job.objects` remain explicit. The local Bambu Studio window filename is not transmitted in the printer-side 3MF and cannot be recovered through LAN MQTT/FTPS. Use `--output-dir` only when another program requires a specific location. Display-started jobs may expose only printer-resident G-code and therefore omit 3MF metadata.

`events watch` is the lower-level primitive for long-running consumers that manage their own lifecycle and asset storage.

For a daemon or bot that should keep watching across many jobs, always give the otherwise-unbounded watcher an explicit operational lifetime:

```bash
bambu-pp-cli events watch --agent --asset-dir <dir> --timeout 24h
```

### Compact current-print summary

```bash
bambu-pp-cli printer status --agent --select state,job.name,job.percent,job.estimated_finish_at,job.current_layer,job.total_layers,ams_active
```

Keep only the fields a notification or agent needs.

### Check filament runway

```bash
bambu-pp-cli ams runway --agent
```

Estimate whether loaded material can finish the current plate.

### Explain the latest print

```bash
bambu-pp-cli job timeline --latest --agent
```

Reconstruct stages, pauses, layer progress, temperature recovery, and errors.

### Find recurring failure context

```bash
bambu-pp-cli history failure-correlations --since 30d --agent
```

Group failed jobs by printer, filament, plate, firmware, speed, and temperature context.

## Auth Setup

Set BAMBU_SERIAL and BAMBU_ACCESS_CODE from the printer network settings. BAMBU_HOST is optional because discovery follows DHCP changes; set it only when multicast SSDP cannot cross the local network. The CLI uses normal local access only, validates the printer CA, and matches the peer certificate CN to the configured serial.

Run `bambu-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  bambu-pp-cli observations --agent --select id,name,status
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

- Use `--home <dir>` for one invocation, or set `BAMBU_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `BAMBU_CONFIG_DIR`, `BAMBU_DATA_DIR`, `BAMBU_STATE_DIR`, `BAMBU_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `BAMBU_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `bambu-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "bambu": {
        "command": "bambu-pp-mcp",
        "env": {
          "BAMBU_HOME": "/srv/bambu"
        }
      }
    }
  }
  ```

Path precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `BAMBU_HOME` or per-kind vars for durable installation relocation, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `BAMBU_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
bambu-pp-cli recall "<user's question>" --agent
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
      "next_action": ["<trial command>", "bambu-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `bambu-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `bambu-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `bambu-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
bambu-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
bambu-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
bambu-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
bambu-pp-cli playbook amend \
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

`bambu-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `BAMBU_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
bambu-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
bambu-pp-cli feedback --stdin < notes.txt
bambu-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `BAMBU_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `BAMBU_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
bambu-pp-cli profile save briefing --json
bambu-pp-cli --profile briefing observations
bambu-pp-cli profile list --json
bambu-pp-cli profile show briefing
bambu-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Printer Profiles

The common one-printer setup uses `BAMBU_SERIAL` and `BAMBU_ACCESS_CODE`. For multiple printers, store only names and environment-variable references, then use the global `--printer` selector:

```bash
export OFFICE_BAMBU_SERIAL="<office-printer-serial>"
export OFFICE_BAMBU_ACCESS_CODE="<office-local-access-code>"
bambu-pp-cli profile printer-add --name office --serial-env OFFICE_BAMBU_SERIAL --access-code-env OFFICE_BAMBU_ACCESS_CODE
bambu-pp-cli profile printer-list --agent
bambu-pp-cli events monitor --printer office --agent
bambu-pp-cli profile printer-delete --name office
```

Printer profiles do not store serials or access codes. `--printer` applies to live commands, printer-scoped history, and `observations`; free-form `search` and `sql` require an explicit persisted `printer_key` predicate.

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

1. **Empty, `help`, or `--help`** → show `bambu-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/devices/bambu/cmd/bambu-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add bambu-pp-mcp -- bambu-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which bambu-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   bambu-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `bambu-pp-cli <command> --help`.
