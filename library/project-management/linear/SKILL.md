---
name: pp-linear
description: "Offline-capable, agent-native Linear CLI with SQLite-backed sync, FTS5 search, cross-cycle comparison, project... Trigger phrases: `what's on my Linear plate today`, `Linear sprint plan for the team`, `Linear cycle comparison`, `Linear burndown for the project`, `which Linear milestone is at risk`, `stale Linear issues`, `clean up the Linear test tickets I created`, `use linear-pp-cli`, `run linear-pp-cli`."
author: "Matt Van Horn"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - linear-pp-cli
---

# Linear — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `linear-pp-cli` binary. **Do not invoke a command named `linear` when this skill is active.** If `linear-pp-cli` is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install linear --cli-only
   ```
2. Verify: `linear-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/project-management/linear/cmd/linear-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## Agent Contract

- Add `--agent` to commands unless a human-readable table is explicitly needed. It implies JSON, compact output, non-interactive mode, no color, and confirmation-safe scripting.
- Use `--data-source live` for closeout/state/description checks where current truth matters. Use `issues search` for duplicate checks; it refreshes stale issue search data or fails visibly. Use `--data-source local` or `similar` only when stale/offline local duplicate search is intentional.
- A missing `description` in compact output does not mean an empty issue body. Request it explicitly: `linear-pp-cli issues ENG-123 --agent --data-source live --select identifier,title,description,state.name,url`.
- Before passing label UUIDs to `issues create` or `issues edit`, run `linear-pp-cli labels list --team ENG --agent --select id,name,global,team.key`. Use only global labels or labels owned by the target issue team; the CLI preflights label ownership and refuses cross-team labels before mutating.
- Never pass multiline Markdown, shell snippets, GraphQL, logs, backticks, `$()` expansions, or media-rich content as inline shell arguments. Write the body to a file or stdin and use the `*-file` / `*-stdin` flags below.

## When to Use This CLI

Reach for this CLI when you need joined queries that span issues, cycles, projects, and milestones — questions Linear's UI answers across multiple tabs and the API answers across multiple round-trips. It's the right pick for agents driving Linear over MCP (the orchestration pair plus named intents covers the full surface in ~1K tokens), for engineering managers preparing Friday updates (cycle comparison, slipped, burndown, blocking queue), and for any agent that must mutate a real workspace under the pp_created fixture-lifecycle contract.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`today`** — See all of your assigned issues across every team for today, ranked by priority and cycle deadline.

  _Reach for this when an agent or human needs a single ranked work queue across every team, without naming the underlying joins._

  ```bash
  linear-pp-cli today --json --agent
  ```
- **`bottleneck`** — See which team members are overloaded and which issues are blocked before sprint planning.

  _Reach for this in sprint planning when you need to see who is overloaded and where work is stuck in one view._

  ```bash
  linear-pp-cli bottleneck --team ENG --json
  ```
- **`stale`** — Find issues that haven't been touched in N days, grouped by team and project.

  _Reach for this during backlog grooming when you need to surface forgotten issues without exhausting the API rate limit._

  ```bash
  linear-pp-cli stale --days 30 --team ENG --json
  ```
- **`issues search` / `similar`** — Find issues that look like duplicates of a query string using local FTS5 search.

  _Reach for this during triage when you suspect an incoming bug duplicates an existing issue._

  ```bash
  linear-pp-cli issues search "login redirect bug" --limit 5 --agent
  linear-pp-cli issues search "pipeline follow-up" --team SYMPH --limit 10 --agent
  linear-pp-cli similar "login redirect bug" --limit 5 --json
  linear-pp-cli similar "pipeline follow-up" --team SYMPH --limit 10 --agent
  ```

  Prefer `issues search` when checking for existing tickets before creating or updating follow-up work; it is the supported issue-search spelling agents tend to reach for. It coordinates freshness for duplicate checks: fresh local data is searched immediately, stale or empty issue data refreshes behind a cross-process lock before search, and refresh failures return a typed error instead of silently serving stale results. Add `--team <key-name-or-uuid>` when a common project name or label appears across teams and the duplicate check must stay inside the target team's queue. Multi-word `issues search` queries may be quoted or passed as separate words; both forms are joined into one FTS query. Under `--agent` / `--json`, `issues search` returns a provenance envelope with freshness metadata; `similar` keeps the legacy raw result array. Use `--data-source local` only when stale/offline local results are intentional.

### Cross-entity rollups
- **`projects burndown`** — Project a project's landing date by linear-regressing remaining estimate against the team's measured velocity.

  _Reach for this when stakeholders ask when a project will land and the project page only shows a static target date someone typed in months ago._

  ```bash
  linear-pp-cli projects burndown PROJ_ID --weeks 8 --json
  ```
- **`cycles compare`** — Side-by-side metrics between any two cycles: completion %, scope added, scope cut, carryover, average cycle time.

  _Reach for this for cycle retros and Friday updates when you need a numeric diff rather than two browser tabs._

  ```bash
  linear-pp-cli cycles compare 42 43 --json
  ```
- **`slipped`** — Show what carried over from last cycle into this cycle, grouped by team and reason heuristic.

  _Reach for this in Friday stakeholder updates when you need a structured slipped-from-last-cycle list, not just a saved view._

  ```bash
  linear-pp-cli slipped --team ENG --json
  ```
- **`velocity`** — Track sprint completion rates over the last N cycles to spot productivity trends.

  _Reach for this in Monday sprint planning to ground rebalance decisions in actual completion data, not the team's last cycle alone._

  ```bash
  linear-pp-cli velocity --weeks 8 --json
  ```
- **`initiatives health`** — Rolled-up portfolio view per initiative: child project progress, milestone target-vs-projected dates, slippage flags.

  _Reach for this in portfolio reviews when stakeholders want the initiative-level rollup, not seven open project tabs._

  ```bash
  linear-pp-cli initiatives health --json
  ```
- **`milestones at-risk`** — List portfolio milestones whose projected landing date has slipped past their target, ranked by slip magnitude.

  _Reach for this in weekly portfolio review when the question is which milestone is most at risk, not which initiative is healthy._

  ```bash
  linear-pp-cli milestones at-risk --json
  ```

### Personal queues
- **`blocking`** — Show issues you are blocking — sorted by downstream impact (downstream count × downstream priority).

  _Reach for this every morning when you need to know which of your in-flight issues are stalling teammates downstream._

  ```bash
  linear-pp-cli blocking --json
  ```

### Agent-native plumbing
- **`pp-test list`** — List Linear issues this CLI created in the current or named session, then archive them with pp-cleanup.

  _Reach for this when an agent needs to clean up only the tickets it created in a session — the workspace's existing data must not be touched._

  ```bash
  linear-pp-cli pp-test list --json
  ```
- **`issues create --trust-mode strict`** — Refuse mutations on Linear issues not in the local pp_created ledger when --trust-mode strict is set; works on create and any future mutation surface.

  _Reach for this when running an agent against a real workspace with real data — strict mode makes accidental mutation impossible._

  ```bash
  linear-pp-cli issues create --title "Test ticket" --team ENG --trust-mode strict
  ```
- **Parent and sub-issue linking** — Create child issues and set, change, or clear parent links without leaving the CLI for raw GraphQL.

  _Reach for this when an agent is creating issue trees, epics, or follow-up hierarchies and needs parentage wired safely._

  ```bash
  linear-pp-cli issues create --title "Child task" --team ENG --parent ENG-123 --description-file /tmp/body.md --agent
  linear-pp-cli issues edit ENG-124 --parent ENG-123 --agent
  linear-pp-cli issues edit ENG-124 --no-parent --agent
  ```
- **Team-safe issue labels** — Discover labels that are valid for the target Linear team, including global labels, before creating or editing issues.

  _Reach for this before passing label UUIDs to `issues create` or `issues edit`; Linear rejects labels owned by another team, and the CLI preflights label ownership before mutating._

  ```bash
  linear-pp-cli labels list --team ENG --agent --select id,name,global,team.key
  linear-pp-cli issues create --title "Title" --team ENG --label <global-or-eng-label-id> --agent
  ```
- **Project and initiative name resolution** — Resolve portfolio objects by human name before writing issue relationships.

  _Reach for this when a user gives an issue identifier plus a project or initiative name. `--project` is UUID-only; use `--project-name` when the input is a human project name._

  ```bash
  linear-pp-cli projects list --agent --select id,name,team.key,state,url
  linear-pp-cli projects search "Autonomous Backlog Manager & Dispatch Governance" --team SYMPH --agent --select id,name,team.key,initiative.name,url
  linear-pp-cli initiatives list --agent --select id,name,status,url
  linear-pp-cli initiatives search "Dispatch Governance" --agent --select id,name,status,url
  linear-pp-cli issues edit SYMPH-795 --project-name "Autonomous Backlog Manager & Dispatch Governance" --dry-run --agent
  linear-pp-cli issues edit SYMPH-795 --project-name "Autonomous Backlog Manager & Dispatch Governance" --agent
  ```

  `--project-name` always performs a live Linear read to resolve the UUID, even when the surrounding issue write is a dry-run. Use `projects search` first when the name is partial; writes require a normalized exact project-name match.
- **Shell-safe Linear writes with media** — Create and update issue descriptions, comments, and Linear docs without putting Markdown bodies on the shell command line.

  _Reach for this whenever a body contains newlines, quotes, backticks, `$()` expansions, shell commands, images, logs, or agent-generated Markdown._

  ```bash
  linear-pp-cli issues create --title "Title" --team ENG --description-file /tmp/body.md --media /tmp/screenshot.png --agent
  linear-pp-cli issues edit ENG-123 --description-file /tmp/body.md --agent
  linear-pp-cli issues edit ENG-123 --media /tmp/screenshot.png --agent
  linear-pp-cli comments add --issue ENG-123 --body-file /tmp/comment.md --media /tmp/screenshot.png --agent
  linear-pp-cli comments edit <comment-id> --body-file /tmp/comment.md --agent
  linear-pp-cli documents create --title "Runbook" --issue ENG-123 --content-file /tmp/runbook.md --agent
  linear-pp-cli documents create --title "Team runbook" --team ENG --content-file /tmp/runbook.md --agent
  linear-pp-cli documents edit <document-id-or-slug> --content-file /tmp/updated.md --agent
  ```

  `documents create` requires exactly one parent (`--issue`, `--project`, `--team`, `--initiative`, `--cycle`, `--release`, or `--folder`); `--team` accepts a key such as `ENG` or a UUID. `issues edit --media`, `comments edit --media`, and `documents edit --media` with no body/content flag fetch the existing Markdown live and append uploaded media links. Images become Markdown image embeds; non-images become Markdown links. Add `--media-public` only when the uploaded asset must be reachable outside the Linear workspace.
- **Current issue reads and comments** — Read full issue bodies and discussion without falling back to stale local state. `comments list` takes the issue positionally (preferred) or via `--issue`.

  ```bash
  linear-pp-cli issues ENG-123 --agent --data-source live --select identifier,title,description,state.id,state.name,url
  linear-pp-cli comments list ENG-123 --agent
  linear-pp-cli comments list --issue ENG-123 --agent --limit 100
  ```
- **State transitions** — Move an issue between workflow states without raw SQL, GraphQL, or `api` spelunking. The exact recipe:

  1. List the states for the issue's team: `workflow-states list --team <key>` (alias: `states list`).
  2. Pick the target by `name` or `type` and copy its `id` UUID.
  3. Pass that UUID to `issues edit --state`.

  ```bash
  linear-pp-cli workflow-states list --team ENG --agent --select id,name,type
  linear-pp-cli issues edit ENG-123 --state <state-uuid> --agent
  ```

  Or skip the lookup entirely with one-command transitions resolved against the issue's own team:

  ```bash
  linear-pp-cli issues edit ENG-123 --state-name "In Progress" --agent
  linear-pp-cli issues edit ENG-123 --state-type started --agent   # usage error if the team has several 'started' states
  ```

  `issues create` takes the same trio, resolved against `--team`, so an issue can open directly in the right state. `--state` requires a UUID (a non-UUID value is a usage error pointing at `--state-name`):

  ```bash
  linear-pp-cli issues create --title "..." --team ENG --state-name "In Progress" --agent
  ```

  Do not use `linear-pp-cli api` or `linear-pp-cli sql` for workflow states — `api` only exposes generated REST-shaped interfaces (currently `integrations`), not Linear GraphQL objects.
- **Linear document reads** — `documents <ref>` accepts every identifier form Linear surfaces: the document UUID, the bare `slugId` (`f7f48ab36080`), the full URL slug (`my-runbook-f7f48ab36080`), or the entire document URL. Copy whichever you have; no slug trimming or parsing shims needed.

  ```bash
  linear-pp-cli documents my-runbook-f7f48ab36080 --agent --select title,updatedAt,content
  linear-pp-cli documents "https://linear.app/<org>/document/my-runbook-f7f48ab36080" --agent
  ```

## Command Reference

**attachments** — Manage attachments

- `linear-pp-cli attachments <id>` — Get a single attachment

**audit-entry-types** — Manage audit-entry-types

- `linear-pp-cli audit-entry-types` — Get a single auditentrytype

**auth-resolver-responses** — Manage auth-resolver-responses

- `linear-pp-cli auth-resolver-responses` — Get a single authresolverresponse

**authentication-session-responses** — Manage authentication-session-responses

- `linear-pp-cli authentication-session-responses` — Get a single authenticationsessionresponse

**email-intake-addresses** — Manage email-intake-addresses

- `linear-pp-cli email-intake-addresses <id>` — Get a single emailintakeaddress

**favorites** — Manage favorites

- `linear-pp-cli favorites <id>` — Get a single favorite

**initiative-relations** — Manage initiative-relations

- `linear-pp-cli initiative-relations <id>` — Get a single initiativerelation

**initiative-to-projects** — Manage initiative-to-projects

- `linear-pp-cli initiative-to-projects <id>` — Get a single initiativetoproject

**initiatives** — Manage initiatives

- `linear-pp-cli initiatives <id>` — Get a single initiative
- `linear-pp-cli initiatives list` — List Linear initiatives
- `linear-pp-cli initiatives search <query>` — Search Linear initiatives by name
- `linear-pp-cli initiatives resolve <name>` — Resolve one Linear initiative name to its UUID

**integrations** — Manage integrations

- `linear-pp-cli integrations create` — Create a integration
- `linear-pp-cli integrations delete` — Delete a integration

**issue-priority-values** — Manage issue-priority-values

- `linear-pp-cli issue-priority-values` — Get a single issuepriorityvalue

**labels** — List Linear issue labels with team ownership

- `linear-pp-cli labels list --team ENG` — List global labels plus labels owned by the target team

**organizations** — Manage organizations

- `linear-pp-cli organizations` — Get a single organization

**project-labels** — Manage project-labels

- `linear-pp-cli project-labels <id>` — Get a single projectlabel

**project-milestones** — Manage project-milestones

- `linear-pp-cli project-milestones <id>` — Get a single projectmilestone

**project-relations** — Manage project-relations

- `linear-pp-cli project-relations <id>` — Get a single projectrelation

**project-statuses** — Manage project-statuses

- `linear-pp-cli project-statuses <id>` — Get a single projectstatus

**project-updates** — Create and list Linear project updates (status posts on a project)

- `linear-pp-cli project-updates list --project <uuid> --agent` — List project updates for a project
- `linear-pp-cli project-updates list --project-name "My Project" --limit 10 --agent` — List updates by project name
- `linear-pp-cli project-updates create --project <uuid> --body-file /tmp/update.md --health onTrack --agent` — Post a project update with markdown body
- `linear-pp-cli project-updates create --project-name "My Project" --body "Sprint on track." --health onTrack --agent` — Post update by project name
- `linear-pp-cli project-updates create --project <uuid> --body-stdin --health atRisk --agent < /tmp/update.md` — Post update from stdin

**projects** — Manage projects

- `linear-pp-cli projects <id>` — Get a single project
- `linear-pp-cli projects list` — List Linear projects
- `linear-pp-cli projects search <query>` — Search Linear projects by name
- `linear-pp-cli projects resolve <name>` — Resolve one Linear project name to its UUID

**release-notes** — Manage release-notes

- `linear-pp-cli release-notes <id>` — Get a single releasenote

**release-pipelines** — Manage release-pipelines

- `linear-pp-cli release-pipelines` — Get a single releasepipeline

**release-stages** — Manage release-stages

- `linear-pp-cli release-stages <id>` — Get a single releasestage

**releases** — Manage releases

- `linear-pp-cli releases <id>` — Get a single release

**roadmap-to-projects** — Manage roadmap-to-projects

- `linear-pp-cli roadmap-to-projects <id>` — Get a single roadmaptoproject

**roadmaps** — Manage roadmaps

- `linear-pp-cli roadmaps <id>` — Get a single roadmap

**teams** — Manage teams

- `linear-pp-cli teams` — Get a single team

**templates** — Manage templates

- `linear-pp-cli templates` — Get a single template

**user-settingses** — Manage user-settingses

- `linear-pp-cli user-settingses` — Get a single usersettings

**users** — Manage users

- `linear-pp-cli users` — Get a single user

**workflow-states** — List Linear workflow states (alias: `states`)

- `linear-pp-cli workflow-states list --team ENG --agent --select id,name,type` — List a team's states with the UUIDs `issues edit --state` needs


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
linear-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

For duplicate checks, `linear-pp-cli which "search issues by text" --agent` should point to `issues search`; use that instead of inventing `issues search --help` fallbacks or raw SQL. If the local issue cache is stale, `issues search` refreshes it or fails with a typed freshness error; agents should not jump to raw GraphQL just because the cache was stale.
For parent/sub-issue linking, `linear-pp-cli which "set issue parent" --agent` should point to `issues edit --parent`.

## Recipes


### Friday stakeholder update

```bash
linear-pp-cli cycles compare current previous --json --select completionPct,scopeAdded,scopeCut,carryover,meanCycleTimeHours
```

Two-row diff of the current and previous cycle, narrowed to the five fields that go into a stakeholder doc — pipe to your LLM of choice to write the prose.

### Daily what-now for an agent

```bash
linear-pp-cli today --json --agent --select id,identifier,title,state.name,cycle.endsAt,priority
```

Ranked work queue with only the fields an agent needs to decide what to pick up; --agent enables agent-mode envelope, --select narrows the payload from kilobytes to ~200 bytes per row.

### Sprint planning rebalance

```bash
linear-pp-cli bottleneck --team ENG --json | jq '.[] | select(.loadIndex > 1.2)'
```

Pulls per-assignee load and pipes to jq for the overloaded slice — the bottleneck command exposes the join; jq does the filter so the command stays composable.

### Backlog grooming sweep

```bash
linear-pp-cli stale --days 60 --team ENG --json --select identifier,title,assignee.name,updatedAt
```

Stale-issue scan with a curated --select projection that's small enough to keep in context across many invocations.

### Agent fixture cleanup

```bash
linear-pp-cli pp-test list --session current --json && linear-pp-cli pp-cleanup --session current
```

List then archive only the issues this CLI created in the current session — never touches pre-existing workspace data.

## Auth Setup

Linear personal API keys go in the `Authorization` header verbatim — no `Bearer` prefix. Run `linear-pp-cli auth set-token lin_api_yourkeyhere` to save your key (no Bearer prefix needed for Linear personal API keys), or export `LINEAR_API_KEY=lin_api_...`. Personal API keys are workspace-scoped; the doctor command validates auth, API connectivity, and store health in one shot.

Run `linear-pp-cli doctor` to verify setup.

## Freshness and Data Sources (read this before driving in an agent loop)

Commands fall into three categories with different data-source semantics. Use `--data-source auto|live|local` to control where reads come from; use `--max-age <duration>` to set the threshold for the "your local data is stale" hint.

**Category 1: Live-first with local fallback** (the spec-emitted commands and the v4-refactored `issues list/get`)

- `attachments <id>`, `projects get <id>`, `teams <id>`, `initiatives get <id>`, `issues <id>`, `issues list`, etc.
- Default (`--data-source auto`): hits Linear's API, writes the response through to the local store, falls back to the store only on **network error** (DNS/timeout/connection refused). 4xx and 5xx errors propagate — they don't silently use stale data.
- `--data-source live`: always hit the API; no fallback. Use this when an agent must have current data and would rather fail loudly than serve stale.
- `--data-source local`: never hit the API. Use this in tight agent loops to conserve Linear's complexity budget (~1500 points/hour on personal keys).
- Promoted Linear GraphQL reads such as `teams` and `projects get` use POST `/graphql` internally. Do not recreate them as shell-level GET `/graphql` calls; Linear rejects that shape with CSRF/preflight errors.

**Category 2: Snapshot-computational (local-only by necessity)**

- `today`, `bottleneck`, `blocking`, `similar`, `velocity`, `slipped`, `cycles compare`, `projects burndown`, `initiatives health`, `milestones at-risk`
- These compute joins/aggregations/FTS5 matches over your synced corpus — there is no single live Linear API call that returns these shapes. The `--data-source` flag is ignored; they always read from the local store.
- **You must `sync` before using these.** Cold-start hint: an empty result prints `(no <resource> in local store — run 'linear-pp-cli sync' to populate)` to stderr.
- Stale-data hint: if the local store hasn't been synced within `--max-age` (default 30 minutes), reads print `(<resource> data is Xm old, exceeds --max-age=30m — run 'linear-pp-cli sync' to refresh)` to stderr. `--json` output stays clean (the hint is stderr-only).

**Freshness-coordinated local search**

- `issues search`
- Uses the local FTS issue index, but coordinates freshness for duplicate checks. If issues data is fresh enough, it searches immediately. If issues data is stale or empty, it takes a cross-process lock, refreshes teams, workflow states, labels, and issues, then searches.
- If refresh fails, it emits a typed JSON error under `--agent` instead of returning stale duplicate-search results. Agent/JSON output is a provenance envelope with freshness metadata; `refreshed` means local issue data changed during the invocation, and `refreshed_by` identifies whether this process, a peer, or an external sync did it. Use `--data-source local` only for explicit offline/stale mode; the JSON metadata marks that stale-local policy. Use `--max-age 0` only when disabling the freshness gate is intentional; the metadata marks `freshness_gate_disabled`. Empty local stores are marked with `unsynced`.

**Category 3: Mutations**

- `issues create`, `issues edit`, `comments add`, `comments edit`, `documents create`, `documents edit`, `pp-cleanup`
- Always hit the API. On success, the HTTP response cache is invalidated AND the new/changed entity is written back to the local store, so a subsequent `issues list --data-source local` sees the mutation without requiring another sync.

**Live-only collaboration reads**

- `comments list`, `documents <id-or-slug>`, `documents list`
- These read the current Linear API because comments and working-session docs are collaboration surfaces where stale local state is more misleading than helpful.

**Label discovery**

- `labels list --team ENG`
- Default (`--data-source auto`) reads live and returns global labels plus labels owned by the named team. `--data-source local` reads the synced `issue_labels` table after `linear-pp-cli sync`.
- Use the returned IDs for `issues create --label` or `issues edit --label`; cross-team label IDs are rejected before the issue mutation is sent.

**The budget-conscious agent loop:**

```bash
# 1. Hydrate once (one batched query for everything)
linear-pp-cli sync

# 2. Read freely — store-backed, zero budget
linear-pp-cli today
linear-pp-cli bottleneck --team ENG --data-source local

# 3. Mutate — write-back keeps the store fresh
linear-pp-cli issues create --title "..." --team ENG --pp-session $SESSION

# 4. Verify the mutation from local (no extra API call)
linear-pp-cli issues list --data-source local --pp-session $SESSION

# 5. Re-sync every ~30 minutes if the session is long
linear-pp-cli sync
```

**Cleanup contract:**

Every `issues create` records the new ticket in a local `pp_created` table tagged with the session (default: timestamp, override with `--pp-session <tag>` or `PP_SESSION` env var). `pp-cleanup --session <tag>` archives only those tickets via the real Linear archive mutation. `--trust-mode strict` refuses mutations on issues not in `pp_created` — pair with the session tag for a hard floor against agent-driven workspace pollution.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout. Failures are JSON too: in `--agent`/`--json` mode every typed error (usage, not-found, auth, API, rate-limit, config) is a one-line envelope on stdout, so piping stdout straight into a JSON parser is always safe:

  ```json
  {"error":"document \"missing-doc\" not found","code":3,"type":"not_found"}
  ```

  The process still exits with the typed code from the Exit Codes table; `type` is the machine-readable name for it (`usage`, `not_found`, `auth`, `api`, `partial_failure`, `rate_limit`, `config`). No `2>&1 | python json` defensive wrappers needed on read paths.
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  linear-pp-cli attachments mock-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
linear-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
linear-pp-cli feedback --stdin < notes.txt
linear-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.linear-pp-cli/feedback.jsonl`. They are never POSTed unless `LINEAR_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `LINEAR_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
linear-pp-cli profile save briefing --json
linear-pp-cli --profile briefing attachments mock-value
linear-pp-cli profile list --json
linear-pp-cli profile show briefing
linear-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `linear-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add linear-pp-mcp -- linear-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which linear-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   linear-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `linear-pp-cli <command> --help`.
