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
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/project-management/linear/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# Linear — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `linear-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install linear --cli-only
   ```
2. Verify: `linear-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Pulls your workspace into a local SQLite store with FTS5 search and runs compound queries that no live API call can answer in one round-trip — today view, bottleneck detection, project burndown, cycle comparison. Ships a thin linear_search + linear_execute MCP orchestration pair (with named multi-step intents for triage, standup, sprint plan, weekly update, and grooming) so agents reach the full surface in ~1K tokens instead of enumerating 60+ endpoint mirrors.

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
- **`similar`** — Find issues that look like duplicates of a query string using offline FTS5 fuzzy matching.

  _Reach for this during triage when you suspect an incoming bug duplicates an existing issue._

  ```bash
  linear-pp-cli similar "login redirect bug" --limit 5 --json
  ```

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

**integrations** — Manage integrations

- `linear-pp-cli integrations create` — Create a integration
- `linear-pp-cli integrations delete` — Delete a integration

**issue-priority-values** — Manage issue-priority-values

- `linear-pp-cli issue-priority-values` — Get a single issuepriorityvalue

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

**projects** — Manage projects

- `linear-pp-cli projects <id>` — Get a single project

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


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
linear-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

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

**Category 2: Snapshot-computational (local-only by necessity)**

- `today`, `bottleneck`, `blocking`, `similar`, `velocity`, `slipped`, `cycles compare`, `projects burndown`, `initiatives health`, `milestones at-risk`
- These compute joins/aggregations/FTS5 matches over your synced corpus — there is no single live Linear API call that returns these shapes. The `--data-source` flag is ignored; they always read from the local store.
- **You must `sync` before using these.** Cold-start hint: an empty result prints `(no <resource> in local store — run 'linear-pp-cli sync' to populate)` to stderr.
- Stale-data hint: if the local store hasn't been synced within `--max-age` (default 30 minutes), reads print `(<resource> data is Xm old, exceeds --max-age=30m — run 'linear-pp-cli sync' to refresh)` to stderr. `--json` output stays clean (the hint is stderr-only).

**Category 3: Mutations**

- `issues create`, `pp-cleanup`
- Always hit the API. On success, the HTTP response cache is invalidated AND the new/changed entity is written back to the local store, so a subsequent `issues list --data-source local` sees the mutation without requiring another sync.

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

- **Pipeable** — JSON on stdout, errors on stderr
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
