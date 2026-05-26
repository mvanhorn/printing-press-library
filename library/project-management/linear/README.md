# Linear CLI

**Offline-capable, agent-native Linear CLI with SQLite-backed sync, FTS5 search, cross-cycle comparison, project burndown projection, and a pp_created fixture-lifecycle contract that lets agents mutate real workspaces safely.**

Pulls your workspace into a local SQLite store with FTS5 search and runs compound queries that no live API call can answer in one round-trip — today view, bottleneck detection, project burndown, cycle comparison. Ships a thin linear_search + linear_execute MCP orchestration pair (with named multi-step intents for triage, standup, sprint plan, weekly update, and grooming) so agents reach the full surface in ~1K tokens instead of enumerating 60+ endpoint mirrors.

Printed by [@mvanhorn](https://github.com/mvanhorn) (Matt Van Horn).

## Install

The recommended path installs both the `linear-pp-cli` binary and the `pp-linear` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install linear
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install linear --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install linear --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install linear --agent claude-code
npx -y @mvanhorn/printing-press-library install linear --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/project-management/linear/cmd/linear-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/linear-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-linear --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-linear --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-linear skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-linear. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/linear-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `LINEAR_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "linear": {
      "command": "linear-pp-mcp",
      "env": {
        "LINEAR_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Linear personal API keys go in the `Authorization` header verbatim — no `Bearer` prefix. Run `linear-pp-cli auth set-token lin_api_yourkeyhere` to save your key (no Bearer prefix needed for Linear personal API keys), or export `LINEAR_API_KEY=lin_api_...`. Personal API keys are workspace-scoped; the doctor command validates auth, API connectivity, and store health in one shot.

## Quick Start

```bash
# Save your Linear personal API key (or export LINEAR_API_KEY)
linear-pp-cli auth set-token <your-key>

# Burn your workspace into the local SQLite store for offline + transcendent queries
linear-pp-cli sync --full

# Your ranked work queue for today across every team
linear-pp-cli today --json

# Pre-sprint-planning overload + blocked-count signal for one team
linear-pp-cli bottleneck --team ENG

# Project landing date from regressed velocity, not the static target someone typed in
linear-pp-cli projects burndown PROJ_ID --weeks 8

# Archive only the test issues this CLI created in this session
linear-pp-cli pp-cleanup

```

## Unique Features

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

## Usage

Run `linear-pp-cli --help` for the full command reference and flag list.

## Commands

### attachments

Manage attachments

- **`linear-pp-cli attachments <id>`** - Get a single attachment

### audit-entry-types

Manage audit-entry-types

- **`linear-pp-cli audit-entry-types`** - Get a single auditentrytype

### auth-resolver-responses

Manage auth-resolver-responses

- **`linear-pp-cli auth-resolver-responses`** - Get a single authresolverresponse

### authentication-session-responses

Manage authentication-session-responses

- **`linear-pp-cli authentication-session-responses`** - Get a single authenticationsessionresponse

### email-intake-addresses

Manage email-intake-addresses

- **`linear-pp-cli email-intake-addresses <id>`** - Get a single emailintakeaddress

### favorites

Manage favorites

- **`linear-pp-cli favorites <id>`** - Get a single favorite

### initiative-relations

Manage initiative-relations

- **`linear-pp-cli initiative-relations <id>`** - Get a single initiativerelation

### initiative-to-projects

Manage initiative-to-projects

- **`linear-pp-cli initiative-to-projects <id>`** - Get a single initiativetoproject

### initiatives

Manage initiatives

- **`linear-pp-cli initiatives <id>`** - Get a single initiative

### integrations

Manage integrations

- **`linear-pp-cli integrations create`** - Create a integration
- **`linear-pp-cli integrations delete`** - Delete a integration

### issue-priority-values

Manage issue-priority-values

- **`linear-pp-cli issue-priority-values`** - Get a single issuepriorityvalue

### organizations

Manage organizations

- **`linear-pp-cli organizations`** - Get a single organization

### project-labels

Manage project-labels

- **`linear-pp-cli project-labels <id>`** - Get a single projectlabel

### project-milestones

Manage project-milestones

- **`linear-pp-cli project-milestones <id>`** - Get a single projectmilestone

### project-relations

Manage project-relations

- **`linear-pp-cli project-relations <id>`** - Get a single projectrelation

### project-statuses

Manage project-statuses

- **`linear-pp-cli project-statuses <id>`** - Get a single projectstatus

### projects

Manage projects

- **`linear-pp-cli projects <id>`** - Get a single project

### release-notes

Manage release-notes

- **`linear-pp-cli release-notes <id>`** - Get a single releasenote

### release-pipelines

Manage release-pipelines

- **`linear-pp-cli release-pipelines`** - Get a single releasepipeline

### release-stages

Manage release-stages

- **`linear-pp-cli release-stages <id>`** - Get a single releasestage

### releases

Manage releases

- **`linear-pp-cli releases <id>`** - Get a single release

### roadmap-to-projects

Manage roadmap-to-projects

- **`linear-pp-cli roadmap-to-projects <id>`** - Get a single roadmaptoproject

### roadmaps

Manage roadmaps

- **`linear-pp-cli roadmaps <id>`** - Get a single roadmap

### teams

Manage teams

- **`linear-pp-cli teams`** - Get a single team

### templates

Manage templates

- **`linear-pp-cli templates`** - Get a single template

### user-settingses

Manage user-settingses

- **`linear-pp-cli user-settingses`** - Get a single usersettings

### users

Manage users

- **`linear-pp-cli users`** - Get a single user

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
linear-pp-cli attachments mock-value

# JSON for scripting and agents
linear-pp-cli attachments mock-value --json

# Filter to specific fields
linear-pp-cli attachments mock-value --json --select id,name,status

# Dry run — show the request without sending
linear-pp-cli attachments mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
linear-pp-cli attachments mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
linear-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/linear-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `LINEAR_API_KEY` | per_call | Yes | Set to your API credential. |

## Freshness and Data Sources

Read commands fall into three categories with different data-source semantics. The persistent flags `--data-source auto|live|local` and `--max-age <duration>` control where reads come from and when to warn about stale local data.

| Category | Commands | Default | Override |
| --- | --- | --- | --- |
| **Live-first with local fallback** | `attachments`, `projects get`, `teams`, `initiatives get`, `issues`, `issues list` (the v4 refactor) | `--data-source auto`: live API → write-through → fall back to local on network error | `--data-source live` (no fallback), `--data-source local` (no API) |
| **Snapshot-computational** | `today`, `bottleneck`, `blocking`, `similar`, `velocity`, `slipped`, `cycles compare`, `projects burndown`, `initiatives health`, `milestones at-risk` | Local store only — no live equivalent exists. **Must `sync` first.** | None (flag ignored) |
| **Mutations** | `issues create`, `pp-cleanup` | Always live; on success, the HTTP cache is invalidated AND the entity is written back to the local store | n/a |

**`--max-age` (default 30 minutes):**

When a store-backed read returns data older than `--max-age`, a stderr hint suggests running `sync`. Set `--max-age 6h` for archival workflows or `--max-age 0` to disable the warning entirely. JSON output stays clean — the hint is stderr-only.

**Cold-start hint:** Running `today`, `issues list`, `bottleneck`, etc. before any sync prints `(no issues in local store — run 'linear-pp-cli sync' to populate)` to stderr.

**Budget-conscious agent pattern (Linear meters ~1500 complexity points/hour on personal keys):**

```bash
# Hydrate once at session start
linear-pp-cli sync

# Read freely from local — zero API budget
linear-pp-cli today --data-source local
linear-pp-cli bottleneck --team ENG --data-source local
linear-pp-cli issues list --assignee me --data-source local

# Mutate — write-back keeps the store fresh, no re-sync needed
linear-pp-cli issues create --title "..." --team ENG --pp-session $SESSION

# Verify from local
linear-pp-cli issues list --data-source local --pp-session $SESSION

# Refresh every ~30 minutes for long sessions
linear-pp-cli sync
```

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `linear-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $LINEAR_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **Authentication failed / 401 from Linear** — Run `linear-pp-cli doctor` — it checks key validity, header shape (no Bearer prefix), and rate-limit headroom in one shot.
- **Rate limit error / complexity budget exceeded** — Lower concurrency on sync and prefer offline reads — Linear meters by complexity points (~1500/hr for personal API keys), mutations cost more than queries.
- **`sync --full` is slow or paginates indefinitely** — Run `linear-pp-cli sync` after the first full sync — it cursors on updatedAt and only fetches changed rows.
- **FTS5 search returns no rows for a term you know exists** — Run `linear-pp-cli sync` to refresh the FTS index, or `linear-pp-cli doctor` to confirm the FTS triggers fired on the latest sync.
- **Agent accidentally mutated an issue it did not create** — Set `LINEAR_PP_CLI_TRUST_MODE=strict` in the environment — strict mode refuses any mutation on an issue ID not in the local pp_created ledger.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**Finesssee/linear-cli**](https://github.com/Finesssee/linear-cli) — Rust
- [**schpet/linear-cli**](https://github.com/schpet/linear-cli) — Ruby
- [**czottmann/linearis**](https://github.com/czottmann/linearis) — TypeScript
- [**dorkitude/linctl**](https://github.com/dorkitude/linctl) — Go
- [**evangodon/linear-cli**](https://github.com/evangodon/linear-cli) — Go
- [**linear-mcp**](https://github.com/tacticlaunch/mcp-linear) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
