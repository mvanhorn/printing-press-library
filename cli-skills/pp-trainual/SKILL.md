---
name: pp-trainual
description: "Every Trainual feature plus compliance audit, assignment gap detection, and training analytics no other tool provides. Trigger phrases: `who is behind on training`, `check trainual compliance`, `trainual onboarding status`, `which curriculums have no tests`, `training completion by role`, `use trainual`, `run trainual`."
author: "Devin"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - trainual-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/productivity/trainual/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# Trainual — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `trainual-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install trainual --cli-only
   ```
2. Verify: `trainual-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/trainual/cmd/trainual-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Sync your entire Trainual account to a local SQLite database, then run compliance audits, detect assignment gaps, and produce coverage matrices that would take hours of manual spreadsheet work. Works offline, pipes to jq, and outputs agent-native JSON.

## When to Use This CLI

Use this CLI when you need training compliance data, onboarding tracking, or content quality audits for a Trainual account. It is the right choice for any task that requires correlating users, subjects, roles, and completion data — questions the Trainual web UI cannot answer without manual spreadsheet work.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Training intelligence
- **`compliance-audit`** — Surface every employee below a completion threshold, grouped by role — the Monday compliance check in one command.

  _Use when an agent needs to identify training compliance gaps across an organization without clicking through dozens of Trainual UI screens._

  ```bash
  trainual-pp-cli compliance-audit --threshold 80 --role "Front Desk" --agent
  ```
- **`assignment-gaps`** — Detect users missing subject assignments that peers in their role have — catch forgotten onboarding assignments before managers complain.

  _Use when an agent needs to verify assignment completeness across roles — catches partial gaps, not just fully unassigned users._

  ```bash
  trainual-pp-cli assignment-gaps --by-role --json
  ```
- **`content-audit`** — List all curriculums with course counts, test counts, and enrollment — flag empty, untested, and orphaned content in one table.

  _Use when an agent needs to identify structural training content problems (empty curriculums, missing tests) across the entire content library._

  ```bash
  trainual-pp-cli content-audit --show-empty --show-untested --agent
  ```
- **`onboarding-status`** — Show new hires from the last N days with their assignment completeness and completion percentage, grouped by role.

  _Use when an agent needs to check whether recent hires are on track with their training assignments._

  ```bash
  trainual-pp-cli onboarding-status --days 30 --json --select user,role,completion_percentage
  ```

### Local state that compounds
- **`coverage-matrix`** — Produce a role-by-subject matrix showing completion percentage per cell — the exact report ops managers build by hand in Excel.

  _Use when an agent needs a complete training coverage snapshot across all roles and subjects in one structured output._

  ```bash
  trainual-pp-cli coverage-matrix --json
  ```
- **`role-completion`** — Rank roles by average completion percentage — instantly see which venues or teams are ahead or behind.

  _Use when an agent needs to compare training progress across organizational units (venues, departments, teams)._

  ```bash
  trainual-pp-cli role-completion --sort avg_completion --agent
  ```
- **`completion-trend`** — Show how a user's completion percentage has changed over successive syncs — week-over-week training progress.

  _Use when an agent needs to track whether a flagged employee's training is actually improving over time._

  ```bash
  trainual-pp-cli completion-trend 1618115 --weeks 8 --json
  ```

### Agent-native plumbing
- **`bulk-assign`** — Assign subjects to all users in a role with one command — the role-based fan-out that individual assign-subjects can't do.

  _Use when an agent needs to ensure all members of a role have consistent subject assignments without iterating manually._

  ```bash
  trainual-pp-cli bulk-assign --role "Kitchen" --subjects 101,102,103 --dry-run
  ```

## Command Reference

**roles** — Roles (groups) for organizing users and assignments

- `trainual-pp-cli roles` — List all roles with optional assigned user data

**subjects** — Training subjects (curriculums) containing courses and tests

- `trainual-pp-cli subjects get` — Get a specific subject by ID
- `trainual-pp-cli subjects list` — List all subjects with optional assigned user data

**tests** — Tests (surveys/quizzes) within a subject

- `trainual-pp-cli tests get` — Get a specific test
- `trainual-pp-cli tests list` — List all tests for a subject

**topics** — Topics (courses) within a subject

- `trainual-pp-cli topics get` — Get a specific topic
- `trainual-pp-cli topics list` — List all topics for a subject

**users** — Manage employees and their training assignments

- `trainual-pp-cli users archive` — Archive (deactivate) a user
- `trainual-pp-cli users assign-roles` — Assign roles to a user
- `trainual-pp-cli users assign-subjects` — Assign subjects (curriculums) to a user
- `trainual-pp-cli users create` — Invite a new user (triggers invitation email, consumes seat)
- `trainual-pp-cli users get` — Get a specific user by ID
- `trainual-pp-cli users list` — List all users with optional completion and assignment data
- `trainual-pp-cli users unarchive` — Restore an archived user
- `trainual-pp-cli users unassign-roles` — Remove role assignments from a user
- `trainual-pp-cli users unassign-subjects` — Remove subject assignments from a user
- `trainual-pp-cli users update` — Update user details


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
trainual-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Monday compliance check

```bash
trainual-pp-cli compliance-audit --threshold 80 --json --select user.name,user.email,subject.name,completion_percentage
```

Show everyone below 80% completion with only the fields a manager needs to act on.

### New hire onboarding report

```bash
trainual-pp-cli onboarding-status --days 14 --agent
```

Check all users who joined in the last 2 weeks and their training progress.

### Content quality sweep

```bash
trainual-pp-cli content-audit --show-empty --show-untested --json
```

Find curriculums with no courses or no tests — the structural gaps in your training library.

### Role comparison for stakeholders

```bash
trainual-pp-cli role-completion --sort avg_completion --agent --select role.name,avg_completion,user_count
```

Rank venues/teams by training completion for leadership reporting.

### Bulk onboarding setup

```bash
trainual-pp-cli bulk-assign --role "New Hire" --subjects 1340738,1295262 --dry-run
```

Preview what would happen before assigning mandatory subjects to all new hires in a role.

## Auth Setup

Trainual uses Bearer token authentication. Generate your API key in Trainual Settings > Integrations > API, then set TRAINUAL_API_KEY in your environment.

Run `trainual-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  trainual-pp-cli roles --agent --select id,name,status
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
trainual-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
trainual-pp-cli feedback --stdin < notes.txt
trainual-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.trainual-pp-cli/feedback.jsonl`. They are never POSTed unless `TRAINUAL_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `TRAINUAL_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
trainual-pp-cli profile save briefing --json
trainual-pp-cli --profile briefing roles
trainual-pp-cli profile list --json
trainual-pp-cli profile show briefing
trainual-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `trainual-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add trainual-pp-mcp -- trainual-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which trainual-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   trainual-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `trainual-pp-cli <command> --help`.
