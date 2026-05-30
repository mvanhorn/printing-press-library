---
name: pp-make
description: "Every Make (formerly Integromat) Management API operation from the terminal, plus blocking agent runs, git-backed blueprint sync, dev-to-prod promote with ID remap, and a cross-team DLQ inbox no other tool has. Trigger phrases: `trigger a make scenario and wait`, `run make scenario synchronously`, `back up make blueprints to git`, `promote make scenario from dev to prod`, `list make scenarios across all teams`, `make.com dlq inbox`, `audit make connections`, `use make`, `use make-pp-cli`."
author: "Wade Carpenter"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - make-pp-cli
    install:
      - kind: go
        bins: [make-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/productivity/make/cmd/make-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/productivity/make/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# Make.com — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `make-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install make --cli-only
   ```
2. Verify: `make-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/make/cmd/make-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

The official make-cli is a 1:1 REST mirror; the Make Cloud MCP is paid-tier and Anthropic-hosted only. make-pp-cli is a single Go binary with an offline SQLite mirror, agent-native --json/--select/--csv on every read, a `scenarios run --wait` that actually returns the bundle, blueprint sync for git version control, and a dev-to-prod promote that solves the ID-remap problem the community has been complaining about for years.

## When to Use This CLI

Reach for make-pp-cli whenever an agent needs a synchronous Make scenario result, whenever scenarios need to move between teams without manually re-pinning connections, or whenever Make's per-scenario UI surfaces (DLQs, connections, hooks) need cross-account analysis. It is the only Make tool that runs as a Go binary, mirrors the API offline into SQLite, and ships agent-native flags on every read.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Agent-native plumbing
- **`scenarios run --wait`** — Trigger a scenario and block until it finishes, returning the execution result as one JSON envelope. Composes /scenarios/{id}/run + executions polling + bundle fetch.

  _When an agent needs a Make scenario's output to decide its next step, this is the only command that returns the bundle inline._

  ```bash
  make-pp-cli scenarios run 3041366 --wait --timeout 5m --json
  ```

### Version control for scenarios
- **`blueprint sync`** — Export every scenario's blueprint into a local repo as canonical JSON, one file per scenario, with a separate sidecar for metadata noise. Commit-ready.

  _Lets an agent or human treat scenarios as code: review diffs, restore prior versions, run CI on changes._

  ```bash
  make-pp-cli blueprint sync --repo ./make-blueprints --all-teams
  ```
- **`blueprint promote`** — Promote a scenario from one team to another, rewriting connectionId/hookId/dataStoreId/folderId via auto-suggested name match or a user-edited YAML map. Dry-run shows the rewrites.

  _Pick this when a Make scenario needs to ship from dev to prod — the only command that solves the ID-remap problem._

  ```bash
  make-pp-cli blueprint promote --from-team 588013 --to-team 999999 --scenario 3454192 --auto-suggest --dry-run
  ```
- **`blueprint diff`** — Diff two blueprint snapshots from the local snapshot table; restore re-PUTs a snapshot via the API. Structural diff ignores the metadata.expect/restore noise.

  _After a blueprint promote went wrong, this is the rollback path — git log + git restore for Make scenarios._

  ```bash
  make-pp-cli blueprint diff 3454192 --from ./make-blueprints/team-588013/3454192.blueprint.json --to current --json
  ```

### Local state that compounds
- **`dlq inbox`** — List incomplete executions across all scenarios in one or many teams, group by error-reason fingerprint, bulk retry or resolve with a regex match.

  _When triaging Make failures, this is the only command that shows everything that broke and groups by why._

  ```bash
  make-pp-cli dlq inbox --all-teams --age 24h --group-by reason --json
  ```
- **`connections audit`** — Find connections that are unused, expiring, or repeatedly errored. Joins local connections with walked blueprint references and execution errors.

  _Run before a prod cutover, or weekly to catch stale OAuth tokens before they 401 your scenarios._

  ```bash
  make-pp-cli connections audit --all-teams --unused --expiring 168h --json
  ```
- **`scenarios list-all`** — Union scenarios across every team the token can see, with last-edit, last-run, error rate, folder path, active flag. Filter by --stale Nh.

  _Monday-morning ritual command: what is active and what is stale across everything you own._

  ```bash
  make-pp-cli scenarios list-all --active --stale 720h --json --select rows.id,rows.name,rows.teamId,rows.lastEdit
  ```
- **`hooks map`** — For each webhook, show which scenarios consume it by walking every blueprint's gateway:CustomWebHook references. Flag orphan hooks (no consumer) and shared hooks (multiple consumers).

  _When a webhook misroutes or you are cleaning up old hooks, this tells you what is actually connected._

  ```bash
  make-pp-cli hooks map --all-teams --orphans --json
  ```

## Command Reference

**connections** — Connections — auth grants to external services

- `make-pp-cli connections delete` — Delete a connection
- `make-pp-cli connections get` — Get a connection
- `make-pp-cli connections list` — List connections for a team
- `make-pp-cli connections test` — Verify a connection is live

**data_stores** — Data stores — Make's key-value persistence

- `make-pp-cli data-stores create` — Create a data store
- `make-pp-cli data-stores delete` — Delete data stores
- `make-pp-cli data-stores get` — Get a data store
- `make-pp-cli data-stores list` — List data stores for a team
- `make-pp-cli data-stores update` — Update data store

**data_structures** — Data structures — typed schemas referenced by data stores and webhooks

- `make-pp-cli data-structures create` — Create a data structure
- `make-pp-cli data-structures delete` — Delete a data structure
- `make-pp-cli data-structures get` — Get a data structure
- `make-pp-cli data-structures list` — List data structures for a team

**devices** — Mobile devices registered for triggering Make scenarios

- `make-pp-cli devices delete` — Delete a device
- `make-pp-cli devices get` — Get a device
- `make-pp-cli devices list` — List devices for a team

**dlqs** — Incomplete executions (Dead Letter Queue) — scenarios that failed

- `make-pp-cli dlqs get` — Get one DLQ entry
- `make-pp-cli dlqs list` — List incomplete executions for a scenario
- `make-pp-cli dlqs resolve` — Resolve (acknowledge) a failed execution
- `make-pp-cli dlqs retry` — Retry a failed execution

**executions** — Scenario executions — historical runs

- `make-pp-cli executions <scenarioId> <executionId>` — Get one execution's detail

**folders** — Scenario folders — group scenarios within a team

- `make-pp-cli folders create` — Create a folder
- `make-pp-cli folders delete` — Delete a folder
- `make-pp-cli folders list` — List scenario folders for a team
- `make-pp-cli folders update` — Rename a folder

**functions** — Custom functions usable inside scenario mappings

- `make-pp-cli functions delete` — Delete a custom function
- `make-pp-cli functions get` — Get a custom function
- `make-pp-cli functions list` — List custom functions for a team

**hooks** — Webhooks — entry points that trigger scenarios

- `make-pp-cli hooks delete` — Delete a webhook
- `make-pp-cli hooks disable` — Disable a webhook
- `make-pp-cli hooks enable` — Enable a webhook
- `make-pp-cli hooks get` — Get a webhook
- `make-pp-cli hooks learn-start` — Start webhook learning mode
- `make-pp-cli hooks learn-stop` — Stop webhook learning mode
- `make-pp-cli hooks list` — List webhooks for a team
- `make-pp-cli hooks ping` — Send a ping to a webhook

**keys** — Keys — credentials (API tokens, basic auth, etc.) attached to connections

- `make-pp-cli keys delete` — Delete a key
- `make-pp-cli keys get` — Get a key
- `make-pp-cli keys list` — List keys for a team

**orgs** — Organizations — top-level account container

- `make-pp-cli orgs get` — Get an organization
- `make-pp-cli orgs list` — List organizations visible to this token

**scenarios** — Scenarios — Make's workflow automations

- `make-pp-cli scenarios activate` — Activate a scenario
- `make-pp-cli scenarios blueprint` — Export a scenario's blueprint as JSON
- `make-pp-cli scenarios clone` — Clone a scenario
- `make-pp-cli scenarios create` — Create a scenario from a blueprint
- `make-pp-cli scenarios deactivate` — Deactivate a scenario
- `make-pp-cli scenarios delete` — Delete a scenario
- `make-pp-cli scenarios get` — Get a scenario
- `make-pp-cli scenarios list` — List scenarios for a team
- `make-pp-cli scenarios logs` — Get execution log entries for a scenario
- `make-pp-cli scenarios run` — Trigger one execution of a scenario (asynchronous unless --wait is used)
- `make-pp-cli scenarios update` — Update scenario metadata

**sdk_apps** — Custom SDK apps — user-built integrations published to Make

- `make-pp-cli sdk-apps get` — Get an SDK app
- `make-pp-cli sdk-apps list` — List SDK apps for a team

**teams** — Teams within an organization

- `make-pp-cli teams create` — Create a team
- `make-pp-cli teams delete` — Delete a team
- `make-pp-cli teams get` — Get a team
- `make-pp-cli teams list` — List teams in an organization

**templates** — Public templates — sharable scenario blueprints

- `make-pp-cli templates get` — Get a template
- `make-pp-cli templates list` — List templates for a team

**users** — Users — token owner and team members

- `make-pp-cli users` — Get the authenticated user


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
make-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Hand-written Extensions

These commands are declared by the spec author and require separate hand-written wiring; the generator does not emit Cobra registration for them. They are listed here for discoverability and are intentionally outside `## Command Reference` so the verify-skill unknown-command check does not treat them as generator-owned paths.

- `make-pp-cli blueprint sync` — Mirror every scenario's blueprint into a local git repo (auto-canonicalized)
- `make-pp-cli blueprint promote` — Promote a scenario from one team to another with auto-remap of connection/hook/data-store IDs
- `make-pp-cli blueprint diff` — Diff two blueprint snapshots from the local snapshot table
- `make-pp-cli blueprint restore` — Restore a blueprint snapshot to the live API
- `make-pp-cli scenarios run-wait` — Trigger a scenario and block until execution finishes; emit bundles as JSON
- `make-pp-cli scenarios list-all-teams` — Union scenarios across every team the token can see; supports --stale Nd
- `make-pp-cli dlq inbox` — Cross-scenario DLQ inbox grouped by error fingerprint with bulk retry/resolve
- `make-pp-cli connections audit` — Find unused, expiring, or repeatedly errored connections
- `make-pp-cli hooks map` — Webhook to scenario routing map; flags orphan and shared hooks

## Recipes

### Agent-blocking scenario run with output bundle

```bash
make-pp-cli scenarios run 3041366 --wait --timeout 5m --json --select status,bundles
```

Triggers the scenario, blocks until terminal, returns the execution status and parsed output bundles inline so the calling agent can act on the result.

### Weekly DLQ triage by error reason

```bash
make-pp-cli dlq inbox --all-teams --age 24h --group-by reason --json
```

Joins incomplete executions across every team and groups by extracted error fingerprint so the on-call sees patterns, not individual failures.

### Promote a scenario dev to prod with auto-remap dry-run

```bash
make-pp-cli blueprint promote --from-team 588013 --to-team 999999 --scenario 3454192 --auto-suggest --dry-run
```

Reads the source blueprint, matches connections/hooks/data-stores by name in the target, prints the rewrite map so you can review before the real promote.

### Find stale OAuth connections before prod cutover

```bash
make-pp-cli connections audit --all-teams --expiring 168h --unused --json --select rows.id,rows.name,rows.expire,rows.issues
```

Surfaces every connection expiring within 7 days (168h) or that isn't referenced by any synced scenario blueprint, so a stale token does not 401 the production run.

### Sync every blueprint to a local repo nightly

```bash
make-pp-cli blueprint sync --repo ./make-blueprints --all-teams
```

Replaces the community's DIY 'daily-export-to-github' Make scenarios. Run from cron; pipe into `git add -A && git commit` to commit.

## Auth Setup

Auth is a Make API token (`Authorization: Token <token>`) plus a zone (us1, us2, eu1, eu2). Token is created at make.com profile -> API -> Add token. Set `MAKE_API_TOKEN` and `MAKE_ZONE` in your environment; `doctor` verifies both.

Run `make-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  make-pp-cli connections list --team-id 42 --agent --select id,name,status
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
make-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
make-pp-cli feedback --stdin < notes.txt
make-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/make-pp-cli/feedback.jsonl`. They are never POSTed unless `MAKE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `MAKE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
make-pp-cli profile save briefing --json
make-pp-cli --profile briefing connections list --team-id 42
make-pp-cli profile list --json
make-pp-cli profile show briefing
make-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `make-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/make/cmd/make-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add make-pp-mcp -- make-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which make-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   make-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `make-pp-cli <command> --help`.
