---
name: pp-linear
description: "Linear project-management CLI for the terminal. Manage issues, projects, cycles, teams, initiatives, roadmaps, and customer records via the Linear GraphQL API with offline-capable SQLite sync. Use when the user asks about their Linear issues, wants today's queue, sprint velocity, team workload, bottlenecks, duplicate / stale / orphaned issues, release pipelines, or wants to create, update, or search Linear items from the terminal. Offline search and analytics work without an API round-trip after a one-time sync."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - linear-pp-cli
      env:
        - LINEAR_API_KEY
    primaryEnv: LINEAR_API_KEY
    install:
      - kind: go
        bins: [linear-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/project-management/linear/cmd/linear-pp-cli
---

# Linear - Printing Press CLI

Manage Linear issues, projects, cycles, teams, and releases from the terminal. The CLI pairs live GraphQL calls with a local SQLite sync so searches, analytics, and cross-entity queries run offline in milliseconds instead of round-tripping to Linear's API.

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

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.23+).
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/project-management/linear/cmd/linear-pp-cli@latest
   ```
3. Verify: `linear-pp-cli --version`.
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.
5. Auth setup:
   ```bash
   export LINEAR_API_KEY="lin_api_..."
   ```
   Create a personal API key at https://linear.app/settings/api.
6. Verify: `linear-pp-cli doctor` reports key status and org identity.

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
