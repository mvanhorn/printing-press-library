---
name: pp-screencloud
description: "A Playgrounds release and fleet-operations CLI that understands both Studio and the app content service. Trigger phrases: `audit my ScreenCloud Playgrounds`, `show which screens this Playground affects`, `check ScreenCloud permissions`, `check ScreenCloud preview drift`, `reconcile a failed Playgrounds create`, `synchronize ScreenCloud Studio topology`, `inspect ScreenCloud Playgrounds files and data`."
author: "BenHof"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - screencloud-pp-cli
    install:
      - kind: go
        bins: [screencloud-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/marketing/screencloud/cmd/screencloud-pp-cli
---

# ScreenCloud — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `screencloud-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install screencloud --cli-only
   ```
2. Verify: `screencloud-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/screencloud/cmd/screencloud-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Inspect the maintained ScreenCloud Studio v2.103.0 reference snapshot, synchronize sanitized placement metadata locally, and coordinate Playgrounds files, data, previews, scoped tokens, and capability diagnostics from one guarded workflow. Release impact, readiness, config drift, contract checks, and recovery plans compose multiple ScreenCloud surfaces into repeatable operator workflows.

## When to Use This CLI

Use this CLI for the maintained ScreenCloud Studio v2.103.0 schema reference, Playgrounds development and release analysis, placement topology, least-privilege diagnostics for mapped commands, scoped-token checks, and safe agent automation that needs stable structured output. It is especially valuable when a task crosses Studio app-instance metadata and Playgrounds files, data, preview, viewer, channel, playlist, share, or screen state.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to design visual content interactively; use the ScreenCloud Playgrounds editor.
- Do not use it to automate login-gated websites; use ScreenCloud Dashboards or the supported browser workflow.
- Do not treat the deprecated Signage REST API as the current Studio integration surface.
- Do not perform any live mutation, including arbitrary GraphQL mutations, app-instance creation, scoped-JWT minting, or Playgrounds writes, without fresh approval, an exact target review, and `--yes`; `--agent` never grants mutation approval.
- Do not treat auth capabilities output as authorization or proof that a write will succeed.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Release confidence
- **`playgrounds impact`** — Map the synchronized spaces, channels, playlists, and screens related to a reviewed Playgrounds working copy before publishing.

  _Use this before a publish when an agent must explain the known synchronized placement graph and its completeness, not merely show a file fingerprint._

  ```bash
  screencloud-pp-cli playgrounds impact 6f14d9d8-7e6d-42a1-9bb4-0a3d75a8a123 --dir ./fixtures/playgrounds --home /tmp/screencloud-pp-sample-home --agent
  ```
- **`playgrounds contract-check`** — Verify the current management and viewer content contracts after minting ephemeral scoped JWTs, without changing Playgrounds content.

  _Use this before automation relies on the bundle-derived Playgrounds contract or after an unexpected response shape._

  ```bash
  screencloud-pp-cli playgrounds contract-check --app-uuid 6f14d9d8-7e6d-42a1-9bb4-0a3d75a8a123 --space-id 11223344-5566-4788-99aa-bbccddeeff00 --agent --yes
  ```
- **`playgrounds preview-drift`** — Find unpublished previews, production-ahead conflicts, and preview work that has waited too long.

  _Use this when an agent needs a fleet-level preview queue rather than inspecting preview and production workspaces one at a time._

  ```bash
  screencloud-pp-cli playgrounds preview-drift --older-than 7d --home /tmp/screencloud-pp-sample-home --agent
  ```

### Fleet intelligence
- **`playgrounds readiness`** — Find missing, inactive, outdated, dangling, and inconsistent Playgrounds deployments across the organization.

  _Use this instead of separate list calls when an agent needs an actionable organization-wide health verdict._

  ```bash
  screencloud-pp-cli playgrounds readiness --home /tmp/screencloud-pp-sample-home --agent --select summary,findings,complete,hint
  ```
- **`playgrounds config-drift`** — Detect structurally divergent Playgrounds configurations without storing or revealing private values.

  _Use this when an agent must compare a fleet safely without pulling sensitive configuration values into context._

  ```bash
  screencloud-pp-cli playgrounds config-drift --app-uuid 6f14d9d8-7e6d-42a1-9bb4-0a3d75a8a123 --home /tmp/screencloud-pp-sample-home --agent
  ```

### Safe recovery
- **`playgrounds create-reconcile`** — Turn a partial create receipt into a resume or cleanup plan; a no-op conclusion requires live verification.

  _Use this after an interrupted two-service create workflow instead of guessing which mutation is safe to repeat. The example uses a shipped fixture; use your own redacted receipt path for real recovery._

  ```bash
  screencloud-pp-cli playgrounds create-reconcile --receipt ./fixtures/receipts/summer-campaign.json --verify-live --yes --dry-run --agent
  ```

### Safe automation
- **`auth capabilities`** — Explain whether the current identity appears able to run a supported mapped command without exposing token material or raw effective grants.

  _Use this before automation or a guarded mutation needs a least-privilege preflight; it is diagnostic evidence, not authorization or a guarantee of mutation success._

  ```bash
  screencloud-pp-cli auth capabilities --for 'playgrounds files put' --agent --select summary,capabilities,partial_visibility,authorization_proof,visibility
  ```

## Provenance and limitations

The Studio surface uses ScreenCloud's official v2.103.0 GraphQL reference. Playgrounds file, data, preview, and viewer behavior was corroborated from authenticated browser traffic and production bundles because it lacks an equivalent stable public reference. Runtime introspection and live content mutation tests were excluded; incomplete local evidence must remain incomplete.

## Abbreviated Generated Endpoint Reference

This table covers the absorbed GraphQL endpoint only. Use `screencloud-pp-cli --help`, `agent-context --pretty`, and subcommand `--help` for the complete command tree.

**graphql** — Execute maintained or user-supplied Studio GraphQL documents

- `screencloud-pp-cli graphql` — Execute a Studio GraphQL document with optional variables


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
screencloud-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

### Required mirror preparation

Before running fleet readiness, impact, or configuration analysis, build a fresh complete metadata mirror:

```bash
screencloud-pp-cli sync --resources apps,spaces,app-installs,app-instances,app-versions,channels,playlists,screens,associations,share-associations --max-pages 10 --agent
```

Readiness additionally requires fresh sanitized Playgrounds files/data timestamps. Populate them one app at a time with `playgrounds preview-drift --refresh --app-uuid <id> --space-id <id> --yes`; missing or older-than-24-hour evidence remains incomplete.

## Recipes

### Audit Playgrounds fleet health

```bash
screencloud-pp-cli playgrounds readiness --agent --select summary,findings,complete,hint
```

After bounded sync and per-app timestamp refresh, joins instances, installs, spaces, versions, and sanitized content evidence; absent, stale, or truncated evidence returns complete=false.

### Map a working-copy change

```bash
screencloud-pp-cli playgrounds impact 6f14d9d8-7e6d-42a1-9bb4-0a3d75a8a123 --dir ./campaign-playground --agent
```

Uses a working-copy fingerprint and the synchronized sanitized relationship graph to map related spaces, channels, playlists, and screens; completeness reflects required mirror state.

### Check the live Playgrounds contract

```bash
screencloud-pp-cli playgrounds contract-check --app-uuid 6f14d9d8-7e6d-42a1-9bb4-0a3d75a8a123 --space-id 11223344-5566-4788-99aa-bbccddeeff00 --agent --yes
```

After fresh approval, mints ephemeral management and viewer JWTs and performs content-read-only assertions; it never persists the tokens or changes Playgrounds content.

### Find aging previews

```bash
screencloud-pp-cli playgrounds preview-drift --older-than 7d --agent
```

Surfaces drift only for apps whose sanitized timestamps were previously populated with preview-drift --refresh --app-uuid <id> --space-id <id> --yes; missing evidence returns complete=false.

### Check least-privilege readiness

```bash
screencloud-pp-cli auth capabilities --for 'playgrounds files put' --agent --select summary,capabilities,partial_visibility,authorization_proof,visibility
```

Explains available, missing, or unknown capabilities for a supported mapped command; partial permission visibility and unmapped commands fail closed as unknown.

## Auth Setup

Set SCREENCLOUD_API_KEY to the organization API key shown in ScreenCloud Studio's Developer page, set SCREENCLOUD_ORGANIZATION_ID (or pass org current --expected-org-id) to enable fail-closed organization matching, and set the region-specific GraphQL endpoint when it differs from the default. The CLI conditionally verifies currentOrgId against that expected organization and can compare currentToken/currentUser structure with the published permission catalog without printing raw grants. Management and viewer JWTs require separate --yes approval, are redacted from normal output, and are never persisted.

Run `ScreenCloud-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to read commands or mutation dry-runs. It expands to `--json --compact --no-input --no-color`; live mutations still require a separate, freshly approved `--yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  screencloud-pp-cli graphql request --query 'query { currentOrgId }' --agent --select data
  ```
- **Previewable** — `--dry-run` summarizes the planned request without sending
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `SCREENCLOUD_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `SCREENCLOUD_CONFIG_DIR`, `SCREENCLOUD_DATA_DIR`, `SCREENCLOUD_STATE_DIR`, `SCREENCLOUD_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `SCREENCLOUD_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `screencloud-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "screencloud": {
        "command": "screencloud-pp-mcp",
        "env": {
          "SCREENCLOUD_HOME": "/srv/screencloud"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `SCREENCLOUD_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `SCREENCLOUD_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
screencloud-pp-cli recall "<user's question>" --agent
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
      "next_action": ["<trial command>", "screencloud-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `screencloud-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `screencloud-pp-cli learnings candidates` lists the full open set.

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
screencloud-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
screencloud-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
screencloud-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
screencloud-pp-cli playbook amend \
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

`screencloud-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `SCREENCLOUD_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
screencloud-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
screencloud-pp-cli feedback --stdin < notes.txt
screencloud-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `SCREENCLOUD_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SCREENCLOUD_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
screencloud-pp-cli profile save briefing --json
screencloud-pp-cli --profile briefing graphql request --query 'query { currentOrgId }'
screencloud-pp-cli profile list --json
screencloud-pp-cli profile show briefing
screencloud-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General runtime error |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `screencloud-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (use `--agent` for reads or dry-runs; require fresh approval and separate `--yes` for live mutations)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/marketing/screencloud/cmd/screencloud-pp-cli@latest
   go install github.com/mvanhorn/printing-press-library/library/marketing/screencloud/cmd/screencloud-pp-mcp@latest
   ```
   Both binaries are required because MCP tools shell out through the guarded companion CLI. Alternatively set `SCREENCLOUD_CLI_PATH` to its absolute path.
2. Register with Claude Code:
   ```bash
   claude mcp add screencloud-pp-mcp -- screencloud-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which screencloud-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   screencloud-pp-cli <command> [subcommand] [args] --agent
   ```
4. If the command may mint a token or change remote state, stop after `--dry-run --agent` until the user freshly approves the exact target; only then add `--yes`.
5. If ambiguous, drill into subcommand help: `screencloud-pp-cli <command> --help`.
