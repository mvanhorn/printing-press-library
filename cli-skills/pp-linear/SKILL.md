---
name: pp-linear
description: "Linear project-management CLI for the terminal. Manage issues, projects, cycles, teams, initiatives, roadmaps, and customer records via the Linear GraphQL API with offline-capable SQLite sync. Use when the user asks about their Linear issues, wants today's queue, sprint velocity, team workload, bottlenecks, duplicate / stale / orphaned issues, release pipelines, or wants to create, update, or search Linear items from the terminal. Offline search and analytics work without an API round-trip after a one-time sync."
author: "Matt Van Horn"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - linear-pp-cli
    install:
      - kind: go
        bins: [linear-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/project-management/linear/cmd/linear-pp-cli
---

# Linear - Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `linear-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer (use the `cli-only` flag to skip the MCP component):
   <!-- npx -y @mvanhorn/printing-press install linear cli-only -->
   ```
   npx -y @mvanhorn/printing-press install linear cli-only
   ```
2. Verify: `linear-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/project-management/linear/cmd/linear-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Reach for this when the user wants:

- a fast "what's on my plate today" view across teams (`today`, `me`)
- find or look up a specific issue by identifier (`issues ESP-1155`)
- list issues assigned to them or a teammate, filtered by team / state (`issues list --assignee me --state started`)
- sprint velocity / team workload / bottleneck analysis (`velocity`, `workload`, `load`, `bottleneck`)
- find stale issues, duplicates, or orphaned items (`stale`, `similar`, `orphans`)
- search across issues, projects, and cycles offline (`sync` once, then `similar` hits SQLite)
- list or inspect projects, cycles, milestones, roadmaps, initiatives, releases
- create / update issues, projects, or cycles (via the typed subcommands and `workflow`)
- export Linear data to JSONL for backup or migration
- stream live changes without polling the web UI (`tail`)
- run read-only SQL against the synced store (`sql` for power users)

Trigger phrases: "what's assigned to me", "look up issue ABC-123", "find my Linear tickets", "what's on my plate", "show me my Linear queue".

Skip it when the user wants to configure team settings, integrations, or OAuth apps; those admin surfaces live in the Linear web admin.

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** -> show `linear-pp-cli --help`
2. **Starts with `install`** -> ends with `mcp` -> MCP installation; otherwise -> CLI installation
3. **Anything else** -> Direct Use (map to the best command and run it)
## MCP Server Installation

The CLI ships an MCP server at `linear-pp-mcp`:

```bash
go install github.com/mvanhorn/printing-press-library/library/project-management/linear/cmd/linear-pp-mcp@latest
claude mcp add -e LINEAR_API_KEY=lin_api_... linear-pp-mcp -- linear-pp-mcp
```

Ask the user for the actual key value before running.

## Direct Use

1. Check installed: `which linear-pp-cli`. If missing, offer CLI installation.
2. Run `linear-pp-cli sync` once (or when data is stale) to populate the local SQLite store. Analytics and search commands then run offline.
3. Discover commands: `linear-pp-cli --help`; drill into `linear-pp-cli <cmd> --help`.
4. Execute with `--agent` for structured output:
   ```bash
   linear-pp-cli <command> [args] --agent
   ```
5. `--data-source auto` (default) hits the local store first with live fallback; use `--data-source live` to force a live call (e.g. for time-sensitive queries on unsynced fields).

## Notable Commands

| Command | What it does |
|---------|--------------|
| `today` | Your issues across all teams, triaged to today's queue |
| `me` | Current authenticated user plus a snapshot of your open work |
| `issues <ID>` | Get a single issue by identifier (e.g. `issues ESP-1155`) |
| `issues list` | List issues from the local store with filters (`--assignee`, `--state`, `--team`, `--project`, `--limit`) |
| `projects` | Get/list projects with milestones and health status |
| `cycles` | Get/list sprint cycles for any team |
| `velocity` | Sprint velocity trends across recent cycles |
| `workload` / `load` | Issue + estimate distribution per team member |
| `bottleneck` | Overloaded assignees and blocked issues |
| `stale` | Issues not updated in N days |
| `similar <text>` | Fuzzy-find potential duplicate issues |
| `orphans` | Items missing assignee, project, or estimate |
| `sync` | Populate local SQLite from the GraphQL API |
| `tail` | Stream live changes by polling at an interval |
| `export` / `import` | JSONL round-trip for backup and migration |
| `sql` | Read-only SQL against the local store (power users) |

Run any command with `--help` for full flag documentation.

## Finding Issues

Three patterns cover the common cases:

```bash
# Look up a specific issue by identifier
linear-pp-cli issues ESP-1155

# List all issues assigned to the authenticated user, excluding completed/canceled
linear-pp-cli issues list --assignee me

# Narrow to a team and state (also accepts --project, --limit, --json)
linear-pp-cli issues list --team ESP --state started --json
```

`issues list` reads from the local store, so run `linear-pp-cli sync` first. `issues <ID>` tries the local store, then falls back to a live GraphQL query, and works without sync.

`--state` matches on state.type so it works across teams with customized state names: `started`, `backlog`, `unstarted`, `completed`, `canceled`, `triage`, or `all`. The default `active` excludes completed and canceled.

`--assignee` accepts `me`, a user UUID, a display name, or an email. `--team` and `--project` accept either a key (e.g. `ESP`) or a UUID.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields, with dotted-path support (see below)
- **Previewable** — `--dry-run` shows the request without sending
- **Cacheable** — GET responses cached for 5 minutes, bypass with `--no-cache`
- **Non-interactive** — never prompts, every input is a flag


### Filtering output

`--select` accepts dotted paths to descend into nested responses; arrays traverse element-wise:

```bash
linear-pp-cli <command> --agent --select id,name
linear-pp-cli <command> --agent --select items.id,items.owner.name
```

Use this to narrow huge payloads to the fields you actually need — critical for deeply nested API responses.


### Response envelope

Data-layer commands wrap output in `{"meta": {...}, "results": <data>}`. Parse `.results` for data and `.meta.source` to know whether it's `live` or local. The `N results (live)` summary is printed to stderr only when stdout is a TTY; piped/agent consumers see pure JSON on stdout.


## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found (issue, project, team) |
| 4 | Authentication required (LINEAR_API_KEY missing or invalid) |
| 5 | API error (Linear upstream, including GraphQL errors) |
| 7 | Rate limited (Linear enforces per-key complexity budgets) |

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`today`** — See all your issues for today across every team, ranked by priority and cycle deadline.

  _When an agent is asked 'what should I work on right now?', this returns the ranked list in one call instead of N team-scoped list queries._

  ```bash
  linear-pp-cli today --json --select identifier,title,priority,cycle.endsAt
  ```
- **`bottleneck`** — See which team members are overloaded and which issues are blocked before sprint planning.

  _Pre-sprint-planning question 'who is overloaded right now' becomes a single agent call instead of scrolling N tabs._

  ```bash
  linear-pp-cli bottleneck --team ENG --json
  ```
- **`projects burndown`** — Project a project's landing date by linear-regressing remaining estimate against the team's measured velocity.

  _Replaces static project target-dates with a velocity-driven projection an agent can compare against the milestone date._

  ```bash
  linear-pp-cli projects burndown PROJ-42 --json
  ```
- **`cycles compare`** — Side-by-side metrics between any two cycles: completion %, scope added, scope cut, carryover, average cycle time.

  _Friday-update ritual: 'how does this cycle compare to last cycle?' becomes one call._

  ```bash
  linear-pp-cli cycles compare 42 43 --json
  ```
- **`stale`** — Find issues that haven't been touched in N days, grouped by team and project.

  _Backlog-grooming workflow: surface zombie issues without paying API complexity for a full scan._

  ```bash
  linear-pp-cli stale --days 30 --team ENG --json
  ```
- **`slipped`** — Show what carried over from last cycle into this cycle, grouped by team and reason heuristic.

  _Maya's Friday update needs 'what slipped' as a structured list, not a manual count._

  ```bash
  linear-pp-cli slipped --team ENG --json
  ```
- **`blocking`** — Show issues you are blocking — sorted by impact (downstream count and priority).

  _Daily ritual: 'what's blocked because of me' becomes one call instead of clicking through every assigned issue._

  ```bash
  linear-pp-cli blocking --json
  ```
- **`similar`** — Find issues that look like duplicates of a query string using offline FTS5 fuzzy matching.

  _Triage and grooming: catch dupes before filing or while sweeping the inbox._

  ```bash
  linear-pp-cli similar "login button broken" --json
  ```
- **`velocity`** — Track sprint completion rates over the last N cycles to spot productivity trends.

  _Multi-cycle trend lines feed the burndown projection and the weekly stakeholder update._

  ```bash
  linear-pp-cli velocity --weeks 8 --team ENG --json
  ```
- **`initiatives health`** — Rolled-up portfolio view per initiative: child project progress, milestone target-vs-projected dates, slippage flags.

  _Portfolio review: 'which milestone in my portfolio is most at risk this month' becomes one ranked answer._

  ```bash
  linear-pp-cli initiatives health --json
  ```

### Agent-native plumbing
- **`pp-test list`** — List Linear issues this CLI created in the current or named session, then archive them with pp-cleanup.

  _Lets agents create test issues during a session and clean up only their own fixtures, never touching pre-existing tickets._

  ```bash
  linear-pp-cli pp-test list --session current && linear-pp-cli pp-cleanup --session current --dry-run
  ```
- **`issues create`** — Refuse mutations on Linear issues not in the local pp_created ledger when --trust-mode strict is set; works on the create command and any future mutation surfaces.

  _When an agent runs against a real workspace, this is the safety net that prevents silent damage to non-test data._

  ```bash
  linear-pp-cli issues create --title "test" --team ENG --trust-mode strict --session demo
  ```
