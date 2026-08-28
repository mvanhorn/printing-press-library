---
name: pp-rightbrain
description: "The first Rightbrain CLI that reaches past tasks — agents, approvals, evals, triggers and audit, plus a local mirror that makes credit spend and latency regressions queryable. Trigger phrases: `is it safe to promote this revision`, `which agent runs are waiting for approval`, `what got slower or more expensive this week`, `why was this agent run slow`, `what changed in the project this week`, `check the rightbrain rollout split`, `use rightbrain`, `run rightbrain`."
author: "Farouk Umar"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - rightbrain-pp-cli
    install:
      - kind: go
        bins: [rightbrain-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/ai/rightbrain/cmd/rightbrain-pp-cli
---

# Rightbrain — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `rightbrain-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install rightbrain --cli-only
   ```
2. Verify: `rightbrain-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/rightbrain/cmd/rightbrain-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Rightbrain's own CLI covers login and task CRUD; the rest of the platform has no command line at all. This one exposes the whole user-reachable API surface, resolves your org and project from config so you never paste two UUIDs into a command, and mirrors runs into SQLite so the questions the API can only answer one page at a time — which revision is actually serving traffic, what got slower this week, which agent runs are parked awaiting approval — become single commands.

## When to Use This CLI

Reach for this CLI when you are operating Rightbrain in production rather than building against it for the first time. It is the right tool for promoting a task revision behind an eval gate, checking whether an A/B split is really splitting, sweeping for agent runs parked awaiting human approval, tracing why one agent run was slow, and answering week-over-week cost and latency questions across every task in a project. It is also the fastest way to reach any part of the API that Rightbrain's own CLI does not cover — agents, skills, collections, documents, triggers, evals, MCP servers, integrations, and the audit log.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to generate TypeScript types for a Next.js app — that is what `npx rightbrain generate` and `@rightbrain/sdk` are for.
- Do not use this CLI to author task prompts or design agents interactively; the dashboard is the authoring surface and this CLI is the operations surface.
- Do not use this CLI to hold a multi-turn conversation with an agent as a chat client; it streams and traces runs, it is not a chat UI.
- Do not use this CLI for Rightbrain staff-only administration (trial extension, model cost tables, signup webhooks) — those endpoints require an internal.admin scope no customer key carries and are deliberately not exposed.
- Do not use local-mirror commands to answer questions about state in the last few seconds; sync first, or use the live endpoint commands.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Ship changes without breaking production
- **`gate`** — Decide whether a candidate task revision is safe to promote, by comparing its eval pass rate against the last recorded result for a different revision of the same task.

  _Reach for this instead of running an eval and eyeballing the number when you need a machine-checkable promote/block decision with a non-zero exit code. Sync eval history first so a baseline exists._

  ```bash
  rightbrain-pp-cli gate 0195d1ff-1f05-437a-95ac-6de8969cb47b --min-pass-rate 0.9 --agent
  ```
- **`rollout`** — Show what traffic each task revision actually received versus the weight you configured, with failure rate, credits, and p50/p95 latency per revision.

  _Use this when an A/B revision is live and you need to know whether the split is real and whether the canary is slower or pricier._

  ```bash
  rightbrain-pp-cli rollout 0195d1ff-1f05-437a-95ac-6de8969cb47b --since 7d --agent
  ```
- **`eval-flake`** — Rank a task's eval test cases by how often they fail, separating genuine flake from cases that fail consistently.

  _Use this when the gate fails and you need to know whether the eval set itself is unreliable before touching the prompt._

  ```bash
  rightbrain-pp-cli eval-flake 0195d1ff-1f05-437a-95ac-6de8969cb47b --last 10 --agent
  ```

### Keep agents unblocked
- **`approvals`** — Triage every pending agent approval request by how long it has been parked and how soon it expires, with expired requests separated from actionable ones.

  _Run this first when an agent 'stopped responding' — a run parked at waiting_for_human emits nothing and pages nobody, and an approval whose window lapsed needs a re-run rather than a decision._

  ```bash
  rightbrain-pp-cli approvals --older-than 1h --agent
  ```
- **`agent-trace`** — Reconstruct one agent run as an indented timeline of paired tool calls and results with per-step elapsed time and a tool-duration histogram.

  _Reach for this when an agent run was slow or failed and you need to see which tool call hung, rather than replaying a raw event stream._

  ```bash
  rightbrain-pp-cli agent-trace 0195d207-32bb-d03d-cfdc-f4516e9222c8 --tools --agent
  ```

### Account for spend and latency
- **`drift`** — Compare this window against the previous one across every task and agent — mean credits, tokens, p95 latency, failure rate — and flag what moved.

  _Use this for the weekly regression sweep; it answers 'did anything get slower or pricier' without paging through run history per task._

  ```bash
  rightbrain-pp-cli drift --since 7d --group-by task --agent --select movers.name,movers.credits_delta_pct,movers.p95_delta_pct
  ```
- **`changelog`** — What changed in the project over a window, with resource UUIDs resolved to task, agent, skill, and collection names, plus an optional tamper-evidence verdict via --verify.

  _Reach for this for a Friday status or a compliance question — it is the only view that reads in names rather than UUIDs._

  ```bash
  rightbrain-pp-cli changelog --since 7d --verify --agent
  ```

## Command Reference

**agent_shares** — Manage agent shares

- `rightbrain-pp-cli agent-shares <org_id> <project_id>` — List TaskAgent shares in a project. TaskAgent shares are public share records scoped to the requested project.

**avatar** — Manage avatar

- `rightbrain-pp-cli avatar <org_id>` — Update organization avatar image

**domains** — Manage domains

- `rightbrain-pp-cli domains create-organization` — Org Domains Post
- `rightbrain-pp-cli domains delete-organization` — Org Domain Delete
- `rightbrain-pp-cli domains list-organization` — Org Domains List

**iam** — Manage iam

- `rightbrain-pp-cli iam organization-get-member` — Retrieve a specific member that has been granted direct access to the organization.
- `rightbrain-pp-cli iam organization-list-members` — Lists all members that have been granted direct access to the organization.
- `rightbrain-pp-cli iam organization-test-permissions` — Test the permissions that the caller (or another subject) holds on the organization.
- `rightbrain-pp-cli iam organization-update-member-roles` — Update the roles that a member holds on the organization.

**integration** — Manage integration

- `rightbrain-pp-cli integration` — Handle the OAuth callback for a platform integration.

**invite** — Manage invite

- `rightbrain-pp-cli invite create-organization` — Org Invites Create
- `rightbrain-pp-cli invite delete-organization` — Org Invites Delete
- `rightbrain-pp-cli invite list-organization` — Org Invites List

**join** — Manage join

- `rightbrain-pp-cli join <org_id>` — Org Join

**model** — Manage model

- `rightbrain-pp-cli model exclude-by-id-from-org` — Exclude one model across the organization. Requires `organization:edit` access.
- `rightbrain-pp-cli model exclude-by-provider-from-org` — Exclude all models supplied by a provider across the organization. Requires `organization:edit` access.
- `rightbrain-pp-cli model exclude-by-vendor-from-org` — Exclude all models served by a vendor across the organization. Requires `organization:edit` access.
- `rightbrain-pp-cli model list-exclusions-for-org` — List organization rules with their impact on active models. Requires `organization:edit` access.
- `rightbrain-pp-cli model remove-exclusion-from-org` — Remove an organization rule and widen model availability. Requires `organization:edit` access.

**org** — Manage org

- `rightbrain-pp-cli org create-organization` — Org Create
- `rightbrain-pp-cli org get-organization` — Org Get
- `rightbrain-pp-cli org list-organizations` — List organizations based on the user's relationship to them.
- `rightbrain-pp-cli org update-organization` — Org Update

**project** — Manage project

- `rightbrain-pp-cli project create` — Create Project
- `rightbrain-pp-cli project delete` — Soft delete a project. Marks the project as deleted without removing it from the database.
- `rightbrain-pp-cli project get` — Get Project
- `rightbrain-pp-cli project list` — List Project
- `rightbrain-pp-cli project update` — Update Project

**public** — Manage public

- `rightbrain-pp-cli public get-task-agent-share` — Return a public TaskAgent share page or sanitized public data.
- `rightbrain-pp-cli public get-task-share` — Access a task via share link. Returns task details based on share permissions.
- `rightbrain-pp-cli public get-task-share-file` — Download a file (image or document) from a task run example associated with a public task share

**runs** — Manage runs

- `rightbrain-pp-cli runs get-project-task-timing-report` — Aggregate visible Task timing across the project.
- `rightbrain-pp-cli runs get-project-task-usage-report` — Aggregate visible Task usage across the project.
- `rightbrain-pp-cli runs list-project-task` — List all task runs across a project. Returns a paginated list of task runs for all tasks in the project.

**shares** — Manage shares

- `rightbrain-pp-cli shares <org_id> <project_id>` — List all task shares across a project. Returns a paginated list of all task shares for tasks within the project.

**skills** — Manage skills

- `rightbrain-pp-cli skills get` — Retrieve lightweight metadata and active revision summary for a skill.
- `rightbrain-pp-cli skills list` — Browse the global skill catalog using lightweight metadata only.
- `rightbrain-pp-cli skills list-sources` — List all available skill sources in the global catalog.
- `rightbrain-pp-cli skills list-tags` — List global skill tags available for filtering the declarative skill catalog.

**task-mcp-server** — Manage task mcp server

- `rightbrain-pp-cli task-mcp-server` — Callback Task Mcp Server

**user** — Manage user

- `rightbrain-pp-cli user get-current` — Get user profile information.
- `rightbrain-pp-cli user update-current` — Update current user profile information
- `rightbrain-pp-cli user upload-avatar` — Upload and update user avatar image. Accepts image files up to 5MB in size. Supported formats: JPEG, PNG, GIF, WebP

**whoami** — Manage whoami

- `rightbrain-pp-cli whoami` — Get current authenticated user information including token details.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
rightbrain-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Under `--json` or `--agent` — which the recipes below use — a no-match is exit `0` with an empty `matches` array, so branch on `matches` being empty rather than on the exit code. Only the plain-text path returns exit `2` for no confident match. Either way, fall back to `--help` or a narrower query.

## Recipes

### Gate a revision before promoting it

```bash
rightbrain-pp-cli gate 0195d1ff-1f05-437a-95ac-6de8969cb47b --min-pass-rate 0.9
```

Runs the task's eval set against the candidate revision, compares the pass rate to the last recorded result for the revision currently taking traffic, and exits non-zero if it regressed — so it drops straight into CI.

### Check whether an A/B split is real

```bash
rightbrain-pp-cli rollout 0195d1ff-1f05-437a-95ac-6de8969cb47b --since 7d
```

Puts each revision's configured weight next to the traffic share it actually received, along with failure rate, mean credits, and p50/p95 latency, which is how you catch a canary that is starved or three times slower.

### Find every agent run stuck on a human

```bash
rightbrain-pp-cli approvals --older-than 1h --agent
```

Fans out across every agent in the project and returns one queue of runs parked at waiting_for_human sorted by parked age, with the gated tool named — the view the API and dashboard both lack.

### Narrow a large drift report for an agent

```bash
rightbrain-pp-cli drift --since 7d --group-by task --agent --select movers.name,movers.p95_delta_pct,movers.credits_delta_pct
```

Drift over a busy project returns a lot of per-task detail; --select trims the payload to just the mover name and the two deltas that decide whether to investigate, keeping agent context small.

### Trace a slow agent run

```bash
rightbrain-pp-cli agent-trace 0195d207-32bb-d03d-cfdc-f4516e9222c8 --tools
```

Rebuilds the run as a timeline of paired tool calls and results with elapsed time per step and a tool-duration histogram, so the nine-second step is obvious instead of buried in a flat event array.

### Write the Friday client update

```bash
rightbrain-pp-cli changelog --since 7d --verify
```

Lists what changed in the project with UUIDs resolved to task and agent names, and attaches the cryptographic integrity verdict so the same output answers the compliance question.

## Auth Setup

Rightbrain authenticates every request with `Authorization: Bearer <token>`. The simplest path is an API key created under Settings -> API Clients in the dashboard: set `RB_API_KEY`, plus `RB_ORG_ID` and `RB_PROJECT_ID` for the project it is scoped to. Those are the same three variables Rightbrain's own `rightbrain init` writes into `.env`, so an existing setup works unchanged. For service-to-service use, mint an OAuth 2.0 access token yourself at `https://oauth.rightbrain.ai/oauth2/token` and hand the result to this CLI as `RB_API_KEY` or via `auth set-token` — the CLI consumes an already-minted bearer token and does not perform the exchange for you. Org and project always come from the URL path rather than the token, which is why this CLI keeps them in config and injects them for you. Run `rightbrain-pp-cli doctor` to confirm credentials, reachability, and cache state in one shot.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  rightbrain-pp-cli project list "$RB_ORG_ID" --agent --select id,name,status
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

- Use `--home <dir>` for one invocation, or set `RIGHTBRAIN_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `RIGHTBRAIN_CONFIG_DIR`, `RIGHTBRAIN_DATA_DIR`, `RIGHTBRAIN_STATE_DIR`, `RIGHTBRAIN_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `RIGHTBRAIN_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `rightbrain-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "rightbrain": {
        "command": "rightbrain-pp-mcp",
        "env": {
          "RIGHTBRAIN_HOME": "/srv/rightbrain"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `RIGHTBRAIN_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `RIGHTBRAIN_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
rightbrain-pp-cli recall "<user's question>" --agent
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
      "next_action": ["<trial command>", "rightbrain-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `rightbrain-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `rightbrain-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `rightbrain-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
rightbrain-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
rightbrain-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
rightbrain-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
rightbrain-pp-cli playbook amend \
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

`rightbrain-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `RIGHTBRAIN_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
rightbrain-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
rightbrain-pp-cli feedback --stdin < notes.txt
rightbrain-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `RIGHTBRAIN_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `RIGHTBRAIN_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`, and commands that emit the standard `{meta, results}` envelope route their output to the named sink. Commands that print bare text (`version`, for instance) accept the flag but write nothing to the sink. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

```
rightbrain-pp-cli profile save briefing --json
rightbrain-pp-cli --profile briefing project list "$RB_ORG_ID"
rightbrain-pp-cli profile list --json
rightbrain-pp-cli profile show briefing
rightbrain-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found (also: `gate` did not clear the gate) |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 6 | Partial failure reported in the response body (downgrade with `--allow-partial-failure`) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `rightbrain-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/ai/rightbrain/cmd/rightbrain-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add rightbrain-pp-mcp -- rightbrain-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which rightbrain-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   rightbrain-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `rightbrain-pp-cli <command> --help`.
