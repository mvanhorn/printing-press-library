# Linear CLI

**Offline-capable, agent-native Linear CLI with SQLite-backed sync, FTS5 search, cross-cycle comparison, project burndown projection, and a pp_created fixture-lifecycle contract that lets agents mutate real workspaces safely.**

Pulls your workspace into a local SQLite store with FTS5 search and runs compound queries that no live API call can answer in one round-trip — today view, bottleneck detection, project burndown, cycle comparison. Ships a thin linear_search + linear_execute MCP orchestration pair (with named multi-step intents for triage, standup, sprint plan, weekly update, and grooming) so agents reach the full surface in ~1K tokens instead of enumerating 60+ endpoint mirrors.

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

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/project-management/linear/cmd/linear-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/linear-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install linear --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-linear --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-linear --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install linear --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

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

Linear personal API keys go in the `Authorization` header verbatim — no `Bearer` prefix. Run `linear-pp-cli auth set-token <your-personal-API-key>` to save your key (no Bearer prefix needed for Linear personal API keys), or export `LINEAR_API_KEY=lin_api_...`. Personal API keys are workspace-scoped; the doctor command validates auth, API connectivity, and store health in one shot.

## Quick Start

```bash
# Save your Linear personal API key (or export LINEAR_API_KEY)
linear-pp-cli auth set-token lin_api_yourkeyhere

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
- **`issues search`** — Search synced Linear issues by text before creating a new ticket; refreshes stale search data or fails visibly instead of serving an empty index.

  _Reach for this before filing anything, when a false "no duplicates" answer would cost more than the refresh._

  ```bash
  linear-pp-cli issues search "login redirect" --json
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
- **`issues edit --parent`** — Create, set, change, or clear Linear parent and sub-issue links without raw GraphQL.

  _Reach for this when restructuring a tree of work and you want identifiers, not UUIDs, in the command._

  ```bash
  linear-pp-cli issues edit ESP-1156 --parent ESP-1155 --agent
  ```
- **`rate-limit`** — Read the workspace API budget (limit, remaining, reset) before spending it on a batch or a full sync.

  _Reach for this before a batch write or a full sync, when running out of budget mid-run is the expensive failure._

  ```bash
  linear-pp-cli rate-limit --json
  ```

### Portfolio inventory
- **`projects list`** — List Linear projects live, optionally scoped by team, before attaching or auditing portfolio work.

  _Reach for this when you need the current project inventory before attaching an issue or auditing portfolio work._

  ```bash
  linear-pp-cli projects list --json
  ```
- **`projects search`** — Search Linear projects live by name with an optional team filter.

  _Reach for this when you know a project by name and need its row without scrolling the whole inventory._

  ```bash
  linear-pp-cli projects search "billing" --json
  ```
- **`projects resolve`** — Resolve one Linear project name to a UUID, preferring exact matches and supporting team scoping.

  _Reach for this before any command that demands a project UUID, so you never paste one by hand._

  ```bash
  linear-pp-cli projects resolve "Billing" --json
  ```
- **`initiatives list`** — List Linear initiatives live with their current status and URL.

  _Reach for this when you need the initiative inventory before a portfolio review._

  ```bash
  linear-pp-cli initiatives list --json
  ```
- **`initiatives search`** — Search Linear initiatives live by name.

  _Reach for this when you know an initiative by name and want only its row._

  ```bash
  linear-pp-cli initiatives search "platform" --json
  ```
- **`initiatives resolve`** — Resolve one Linear initiative name to a UUID, preferring exact matches.

  _Reach for this before linking a project to an initiative, so the UUID comes from the workspace and not from a copy-paste._

  ```bash
  linear-pp-cli initiatives resolve "Platform" --json
  ```

### Conventional reads
- **`issues`** — Get one or several Linear issues by identifier, in caller order, resolving through the local store before the API.

  _Reach for this whenever you have an identifier from a human or a commit message and want the issue without a UUID lookup._

  ```bash
  linear-pp-cli issues ESP-1155 --agent
  ```
- **`documents`** — View a Linear document by UUID, bare slugId, URL slug, or full document URL.

  _Reach for this when the only handle you have is a URL someone pasted into chat._

  ```bash
  linear-pp-cli documents my-runbook-f7f48ab36080 --agent
  ```
- **`comments add`** — Add a comment to a Linear issue, taking the body inline, from a file, or from stdin so markdown and shell metacharacters survive intact.

  _Reach for this when the comment carries a code block or a command line that must arrive literally._

  ```bash
  linear-pp-cli comments add --issue ESP-1155 --body-file /tmp/comment.md --agent
  ```

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

## Usage

Run `linear-pp-cli --help` for the full command reference and flag list.

## Commands

The full tree, straight out of `linear-pp-cli agent-context --pretty`. Run `linear-pp-cli <command> --help` for flags.

### attachments

Attachment cards on issues: get, create, link a URL, update, delete, and dedupe by URL

- **`linear-pp-cli attachments <id>`** - Get a single attachment
- **`linear-pp-cli attachments create`** - Create an attachment card on an issue
- **`linear-pp-cli attachments delete <attachment-id>`** - Delete an attachment
- **`linear-pp-cli attachments for-url`** - List the attachments recorded against a URL
- **`linear-pp-cli attachments link-url`** - Link a URL to an issue and let Linear unfurl it
- **`linear-pp-cli attachments update <attachment-id>`** - Update an attachment's title, subtitle, metadata, or icon

### auth

Manage authentication for Linear

- **`linear-pp-cli auth logout`** - Clear stored credentials
- **`linear-pp-cli auth set-token <token>`** - Save an API token to the config file
- **`linear-pp-cli auth setup`** - Print steps for obtaining a credential (use --launch to open the URL)
- **`linear-pp-cli auth status`** - Show authentication status

### comments

List, add, edit, delete, and resolve Linear comments

- **`linear-pp-cli comments add [issue]`** - Add a Linear comment
- **`linear-pp-cli comments delete <comment-id>`** - Delete a Linear comment
- **`linear-pp-cli comments edit <comment-id>`** - Edit a Linear comment
- **`linear-pp-cli comments list [issue]`** - List comments on an issue
- **`linear-pp-cli comments resolve <comment-id>`** - Resolve a comment thread
- **`linear-pp-cli comments unresolve <comment-id>`** - Reopen a previously resolved comment thread

### custom-views

Linear custom views (saved filters): list, get, create, update, delete

- **`linear-pp-cli custom-views create`** - Create a custom view
- **`linear-pp-cli custom-views delete <custom-view-id>`** - Delete a custom view
- **`linear-pp-cli custom-views get <custom-view-id>`** - Get one custom view including its filter payload
- **`linear-pp-cli custom-views list`** - List custom views
- **`linear-pp-cli custom-views update <custom-view-id>`** - Update a custom view

### cycles

Linear cycles: cycle-over-cycle comparison

- **`linear-pp-cli cycles archive <cycle-id>`** - Archive a cycle, unlinking every issue still assigned to it
- **`linear-pp-cli cycles compare <cycle-a> <cycle-b>`** - Compare two cycles side-by-side: completion %, scope added/cut, carryover
- **`linear-pp-cli cycles create`** - Create a cycle on a team with an explicit start and end
- **`linear-pp-cli cycles list`** - List cycles with their dates and progress, optionally filtered by team
- **`linear-pp-cli cycles shift-all`** - Shift a cycle and every cycle after it by a number of days
- **`linear-pp-cli cycles start-upcoming <cycle-id>`** - Start the next upcoming cycle as of midnight today
- **`linear-pp-cli cycles update <cycle-id>`** - Update a cycle's name, description, window, or completion time

### documents

View, list, create, edit, delete, and restore Linear documents

- **`linear-pp-cli documents [document-ref]`** - View, list, create, edit, delete, and restore Linear documents
- **`linear-pp-cli documents create`** - Create a Linear document
- **`linear-pp-cli documents delete <document-ref>`** - Delete a Linear document
- **`linear-pp-cli documents edit <document-id-or-slug>`** - Edit a Linear document
- **`linear-pp-cli documents get <document-ref>`** - View a Linear document
- **`linear-pp-cli documents list`** - List Linear documents
- **`linear-pp-cli documents unarchive <document-id>`** - Restore a deleted Linear document

### favorites

Sidebar favorites: get, create, reorder or refolder, delete

- **`linear-pp-cli favorites <id>`** - Get a single favorite
- **`linear-pp-cli favorites create`** - Favorite one Linear entity
- **`linear-pp-cli favorites delete <favorite-id>`** - Unfavorite an entity
- **`linear-pp-cli favorites update <favorite-id>`** - Reorder or refolder a favorite

### feedback

Record feedback about this CLI (local by default; upstream opt-in)

- **`linear-pp-cli feedback [text]`** - Record feedback about this CLI (local by default; upstream opt-in)
- **`linear-pp-cli feedback list`** - List recent feedback entries

### groups

Inspect the workflow-state groups that --state resolves against

- **`linear-pp-cli groups check <group-or-type>`** - Resolve one state token and show exactly what it matches
- **`linear-pp-cli groups list`** - List every effective state group and where it is declared

### initiatives

Linear initiatives: get, list, search, resolve, and portfolio health rollup

- **`linear-pp-cli initiatives archive <initiative>`** - Archive a Linear initiative
- **`linear-pp-cli initiatives create`** - Create a Linear initiative
- **`linear-pp-cli initiatives delete <initiative>`** - Delete a Linear initiative
- **`linear-pp-cli initiatives get <id>`** - Get a single initiative
- **`linear-pp-cli initiatives health`** - Rolled-up portfolio view per initiative: project progress, milestone risk, slippage
- **`linear-pp-cli initiatives link-project <initiative>`** - Link a project to a Linear initiative
- **`linear-pp-cli initiatives list`** - List Linear initiatives
- **`linear-pp-cli initiatives resolve <name>`** - Resolve one Linear initiative name to its UUID
- **`linear-pp-cli initiatives search <query>`** - Search Linear initiatives by name
- **`linear-pp-cli initiatives unarchive <initiative>`** - Restore an archived Linear initiative
- **`linear-pp-cli initiatives unlink-project <initiative>`** - Unlink a project from a Linear initiative
- **`linear-pp-cli initiatives update <initiative>`** - Update a Linear initiative

### issues

Get, list, or create Linear issues

- **`linear-pp-cli issues [ID]`** - Get, list, or create Linear issues
- **`linear-pp-cli issues archive <issue>`** - Archive a Linear issue
- **`linear-pp-cli issues batch-create --file <path>`** - Create up to 50 issues in one transaction and record them in the pp_created ledger
- **`linear-pp-cli issues batch-update --ids <a,b,c>`** - Apply one change to up to 50 issues in a single transaction
- **`linear-pp-cli issues close-duplicate <issue> --of <canonical-issue>`** - Close an issue as a duplicate: create the duplicate relation, then set the duplicate state
- **`linear-pp-cli issues create`** - Create a new Linear issue and record it in the pp_created ledger
- **`linear-pp-cli issues delete <issue>`** - Trash a Linear issue
- **`linear-pp-cli issues edit <issue-id>`** - Edit a Linear issue
- **`linear-pp-cli issues get ID`** - Get Linear issues by identifier
- **`linear-pp-cli issues list`** - List issues from the local sqlite store with filters
- **`linear-pp-cli issues search <query>`** - Search synced issues by text (alias for similar)
- **`linear-pp-cli issues subscribe <issue>`** - Subscribe a user to a Linear issue
- **`linear-pp-cli issues unarchive <issue>`** - Restore an archived Linear issue
- **`linear-pp-cli issues unsubscribe <issue>`** - Unsubscribe a user from a Linear issue

### labels

Manage Linear issue labels: list, create, update, delete, retire, restore

- **`linear-pp-cli labels create`** - Create an issue label, workspace-wide or scoped to one team
- **`linear-pp-cli labels delete <label-id>`** - Permanently delete an issue label
- **`linear-pp-cli labels list`** - List issue labels, optionally filtered to labels safe for a team
- **`linear-pp-cli labels restore <label-id>`** - Restore a retired issue label so it can be applied again
- **`linear-pp-cli labels retire <label-id>`** - Retire an issue label so it can no longer be applied to new issues
- **`linear-pp-cli labels update <label-id>`** - Update an issue label's name, color, description, or parent

### milestones

List project milestones at risk of missing their target date

- **`linear-pp-cli milestones at-risk`** - Rank portfolio milestones by projected slippage past target date
- **`linear-pp-cli milestones create`** - Create a project milestone
- **`linear-pp-cli milestones delete <milestone-id>`** - Delete a project milestone
- **`linear-pp-cli milestones list`** - List a project's milestones
- **`linear-pp-cli milestones move <milestone-id>`** - Move a project milestone and its issues to another project
- **`linear-pp-cli milestones update <milestone-id>`** - Update a project milestone

### notifications

Read and triage the authenticated user's Linear inbox

- **`linear-pp-cli notifications archive <notification-id>`** - Archive one notification
- **`linear-pp-cli notifications archive-all`** - Archive every notification about one entity
- **`linear-pp-cli notifications get <notification-id>`** - Get one notification by id
- **`linear-pp-cli notifications list`** - List the authenticated user's notifications
- **`linear-pp-cli notifications read <notification-id>`** - Mark one notification as read
- **`linear-pp-cli notifications read-all`** - Mark every notification about one entity as read
- **`linear-pp-cli notifications snooze <notification-id> --until <when>`** - Snooze one notification until a given time
- **`linear-pp-cli notifications snooze-all`** - Snooze every notification about one entity
- **`linear-pp-cli notifications unarchive <notification-id>`** - Restore one archived notification
- **`linear-pp-cli notifications unread <notification-id>`** - Mark one notification as unread
- **`linear-pp-cli notifications unread-all`** - Mark every notification about one entity as unread
- **`linear-pp-cli notifications unread-count`** - Report the authenticated user's unread notification count
- **`linear-pp-cli notifications unsnooze <notification-id>`** - Wake one snoozed notification
- **`linear-pp-cli notifications unsnooze-all`** - Wake every snoozed notification about one entity

### pp-test

List Linear issues this CLI has created (test-fixture ledger)

- **`linear-pp-cli pp-test list`** - List active fixtures (issues this CLI created and has not yet archived)
- **`linear-pp-cli pp-test sessions`** - List all distinct session tags with active fixtures

### profile

Named sets of flags saved for reuse

- **`linear-pp-cli profile delete <name>`** - Remove a profile
- **`linear-pp-cli profile list`** - List saved profiles
- **`linear-pp-cli profile save <name> [--<flag> <value> ...]`** - Save the current invocation's non-default flags as a named profile
- **`linear-pp-cli profile show <name>`** - Show a profile's values as JSON
- **`linear-pp-cli profile use <name>`** - Print the flag values a profile will apply (does not execute anything)

### project-updates

List, create, edit, and archive Linear project updates

- **`linear-pp-cli project-updates archive <project-update-id>`** - Archive a Linear project update
- **`linear-pp-cli project-updates create`** - Create a new Linear project update
- **`linear-pp-cli project-updates list`** - List project updates for a Linear project
- **`linear-pp-cli project-updates unarchive <project-update-id>`** - Restore an archived Linear project update
- **`linear-pp-cli project-updates update <project-update-id>`** - Edit a posted Linear project update

### projects

Linear projects: get, list, search, resolve, and burndown projection

- **`linear-pp-cli projects add-label <project-id>`** - Attach a project label to a Linear project
- **`linear-pp-cli projects burndown <project>`** - Project a project's landing date from estimate vs measured velocity
- **`linear-pp-cli projects create`** - Create a Linear project
- **`linear-pp-cli projects delete <project-id>`** - Delete (trash) a Linear project
- **`linear-pp-cli projects get <id>`** - Get a single project
- **`linear-pp-cli projects list`** - List Linear projects
- **`linear-pp-cli projects remove-label <project-id>`** - Detach a project label from a Linear project
- **`linear-pp-cli projects resolve <name>`** - Resolve one Linear project name to its UUID
- **`linear-pp-cli projects search <query>`** - Search Linear projects by name
- **`linear-pp-cli projects update <project-id>`** - Update a Linear project

### reactions

List, add, and remove emoji reactions

- **`linear-pp-cli reactions add`** - Add an emoji reaction to a comment, issue, project update, or initiative update
- **`linear-pp-cli reactions list`** - List the reactions on a comment, issue, project update, or initiative update
- **`linear-pp-cli reactions remove <reaction-id>`** - Remove a reaction

### relations

Read and manage issue relations (blocks, duplicate, related, similar)

- **`linear-pp-cli relations create <issue> --type <type> --to <issue>`** - Create an issue relation (blocks, duplicate, related, similar)
- **`linear-pp-cli relations delete <relation-id>`** - Delete an issue relation by its relation UUID
- **`linear-pp-cli relations list <issue>`** - List every relation touching one issue, in both directions

### templates

Workspace templates: get, list, create, update, delete

- **`linear-pp-cli templates create`** - Create a template
- **`linear-pp-cli templates delete <template-id>`** - Delete a template
- **`linear-pp-cli templates get <template-id>`** - Get one template including its templateData payload
- **`linear-pp-cli templates list`** - List workspace templates, optionally scoped by team or type
- **`linear-pp-cli templates update <template-id>`** - Update a template

### workflow-states

Manage Linear workflow states: list, create, update, archive

- **`linear-pp-cli workflow-states archive <state-id>`** - Archive a workflow state
- **`linear-pp-cli workflow-states create`** - Create a workflow state on a team's board
- **`linear-pp-cli workflow-states list`** - List workflow states, optionally filtered by team
- **`linear-pp-cli workflow-states update <state-id>`** - Update a workflow state's name, color, description, or board position

### Standalone commands

- **`linear-pp-cli analytics`** - Run analytics queries on locally synced data
- **`linear-pp-cli api [interface]`** - Browse all API endpoints by interface name
- **`linear-pp-cli audit-entry-types`** - Get a single auditentrytype
- **`linear-pp-cli authentication-session-responses`** - Get a single authenticationsessionresponse
- **`linear-pp-cli blocking`** - Show issues you are blocking — sorted by downstream impact
- **`linear-pp-cli bottleneck`** - Find overloaded team members and blocked issues
- **`linear-pp-cli doctor`** - Check CLI health
- **`linear-pp-cli email-intake-addresses <id>`** - Get a single emailintakeaddress
- **`linear-pp-cli initiative-relations <id>`** - Get a single initiativerelation
- **`linear-pp-cli initiative-to-projects <id>`** - Get a single initiativetoproject
- **`linear-pp-cli issue-priority-values`** - Get a single issuepriorityvalue
- **`linear-pp-cli load`** - Show workload distribution per assignee
- **`linear-pp-cli me`** - Show current authenticated user
- **`linear-pp-cli organizations`** - Get a single organization
- **`linear-pp-cli orphans`** - Find items missing key fields like assignee or project
- **`linear-pp-cli pp-cleanup`** - Archive Linear issues this CLI created (scoped to the pp_created ledger)
- **`linear-pp-cli project-labels <id>`** - Get a single projectlabel
- **`linear-pp-cli project-milestones <id>`** - Get a single projectmilestone
- **`linear-pp-cli project-relations <id>`** - Get a single projectrelation
- **`linear-pp-cli project-statuses <id>`** - Get a single projectstatus
- **`linear-pp-cli rate-limit`** - Show the current Linear API rate limit budget
- **`linear-pp-cli reconcile`** - Decide whether proposed work is a duplicate, a relative, or genuinely new, and optionally act on it
- **`linear-pp-cli release-notes <id>`** - Get a single releasenote
- **`linear-pp-cli release-pipelines`** - Get a single releasepipeline
- **`linear-pp-cli release-stages <id>`** - Get a single releasestage
- **`linear-pp-cli releases <id>`** - Get a single release
- **`linear-pp-cli roadmap-to-projects <id>`** - Get a single roadmaptoproject
- **`linear-pp-cli roadmaps <id>`** - Get a single roadmap
- **`linear-pp-cli similar [query]`** - Find potentially duplicate issues using fuzzy text search
- **`linear-pp-cli slipped`** - Show issues that slipped from the previous cycle into the current cycle
- **`linear-pp-cli sql <query>`** - Run read-only SQL against the local store
- **`linear-pp-cli stale`** - Find items with no updates in N days
- **`linear-pp-cli sync`** - Sync Linear data to local SQLite store
- **`linear-pp-cli teams`** - Get a single team
- **`linear-pp-cli today`** - Show your issues for today across all teams
- **`linear-pp-cli unblocked`** - Show blocked issues whose blockers are now all closed
- **`linear-pp-cli user-settingses`** - Get a single usersettings
- **`linear-pp-cli users`** - Get a single user
- **`linear-pp-cli velocity`** - Show sprint velocity trends over recent cycles
- **`linear-pp-cli which [query]`** - Find the command that implements a capability

`completion bash`, `completion zsh`, `completion fish`, `completion powershell`, `help [command]` and `version` are the usual cobra built-ins.

`roadmaps` and `roadmap-to-projects` are marked deprecated. Linear replaced Roadmap and RoadmapToProject with Initiative and InitiativeToProject, so reach for the `initiatives` family instead. The old commands still work.

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

## Output contract

Every JSON response belongs to exactly one of three shapes. Which one a command uses is a property of the command, not of the run, so a caller can hard-code the access path.

**Reads answer `{results, meta}`.** `results` is the payload, an array for list and search commands and an object for single-subject rollups. It is never `null`: an empty read answers `{"results": [], "meta": {...}}`. `meta` always carries `source` (`live`, `local`, or `mixed` on `issues search --live`) and `resource_type`, and adds `synced_at` on local reads, `reason` for why local was chosen, `count`, and the scope that produced the result (`query`, `team_scope`). `--select` descends into `results` and leaves `meta` intact, so field filtering never costs you provenance.

```bash
linear-pp-cli projects list --team SYMPH --agent --select id,name,team.key
# {"results":[{"id":"...","name":"...","team":{"key":"SYMPH"}}],"meta":{"source":"live","resource_type":"projects","count":1,"team_scope":"SYMPH","reason":"user_requested"}}
```

**Mutations answer an event object.** A write reports what it did: `success`, the affected entity, and, under `--dry-run`, a `{"event":"would_*"}` preview naming the GraphQL mutation and the exact input it would have sent. Writes never wrap themselves in `results`, because a write has one subject and no provenance question to answer.

**Resolvers answer a bare object.** `projects resolve` and `initiatives resolve` print the single matched entity at the top level, so `.id` is one hop away. An ambiguous or missing name is a typed usage error carrying the candidate list, not an empty success. This is the one read family that is deliberately not enveloped, because its whole contract is exactly one object or a failure.

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

Agent recipes:

```bash
# Full current issue body; compact output strips descriptions unless selected
linear-pp-cli issues ENG-123 --agent --data-source live --select identifier,title,description,state.name,url

# Several current issue bodies in caller order
linear-pp-cli issues ENG-123,ENG-124 --agent --data-source live --select identifier,title,description,state.name,url

# Safe multiline writes; body files preserve shell snippets literally
linear-pp-cli comments add --issue ENG-123 --body-file /tmp/comment.md --agent
```

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
| **Freshness-coordinated local search** | `issues search` | Local FTS over synced issues. If issue data is stale or empty, the command refreshes teams, workflow states, labels, and issues behind a cross-process lock before searching; if refresh fails, it returns a typed error. Agent/JSON output is a provenance envelope with freshness metadata; `refreshed` means local issue data changed during the invocation, and `refreshed_by` identifies whether this process, a peer, or an external sync did it. | `--data-source local` explicitly allows stale/offline local results; `--max-age 0` disables the freshness gate and marks `freshness_gate_disabled`. Empty local stores are marked with `unsynced`. |
| **Snapshot-computational** | `today`, `bottleneck`, `blocking`, `similar`, `velocity`, `slipped`, `cycles compare`, `projects burndown`, `initiatives health`, `milestones at-risk` | Local store only — no live equivalent exists. **Must `sync` first.** | None (flag ignored) |
| **Label discovery** | `labels list --team ENG` | `--data-source auto`: reads live by default; `--data-source local` reads the synced `issue_labels` table | `--data-source live`, `--data-source local` |
| **Live collaboration reads** | `comments list`, `documents`, `documents list` | Always live; comments and working-session docs are collaboration surfaces where stale local state is misleading | n/a |
| **Live-only, no local snapshot** | `unblocked`, every `notifications` verb, `rate-limit` | Always live. `unblocked` refuses `--data-source local` with a usage error rather than pretending | n/a |
| **Mutations** | The issue lifecycle (`create`, `edit`, `archive`, `unarchive`, `delete`, `subscribe`, `unsubscribe`, `close-duplicate`, `batch-create`, `batch-update`), plus the write leaves on `relations`, `comments`, `documents`, `labels`, `workflow-states`, `cycles`, `projects`, `milestones`, `initiatives`, `project-updates`, `attachments`, `templates`, `custom-views`, `favorites`, `reactions`, `notifications`, `reconcile --execute` and `pp-cleanup` | Always live; on success, the HTTP cache is invalidated AND the changed entity is written back to the local store | n/a |

Promoted Linear GraphQL read commands such as `teams` and `projects get` use POST `/graphql` internally. They should not be reimplemented with shell-level GET calls; Linear rejects GET `/graphql` with CSRF/preflight errors.

**What `sync` populates:** issues, projects, teams, cycles, users, labels, and workflow states, then the workflow shell resources: documents, templates, custom views, favorites, project milestones, project statuses, initiatives, and issue relations. Those eight tables existed from the first migration and stayed empty until now. Each one is crawled from a workspace-wide connection and reconciled like the rest: rows whose id was not seen upstream are deleted, and a crawl cut short by `--max-pages` prunes nothing. The five with a generic read path (`templates`, `favorites`, `initiatives`, `project-milestones`, `project-statuses`) are mirrored into the read cache in the same pass, so `--data-source local` serves them offline instead of reporting an empty store. `documents` and `custom-views` keep their live-only commands, so their tables are a snapshot for analytics rather than a read path.

**Sync modes:**

| Mode | Fetches | Prunes |
| --- | --- | --- |
| `sync` | paged crawl from the stored cursor, capped by `--max-pages` (default 10) | yes, once a resource reaches its last page |
| `sync --incremental` | only rows with `updatedAt` after that resource's last recorded sync, minus a five minute overlap. Resources whose query takes no filter are fetched in full | never |
| `sync --full` | everything, cursors cleared first | yes |
| `sync --no-prune` | as the mode it is combined with | no |

The prune is what makes `--data-source local` trustworthy: after a resource is fetched to its last page, local rows whose id was not seen upstream are deleted. A crawl truncated by `--max-pages` warns and skips the prune, an empty live set is refused rather than read as "everything was deleted", and `issues_fts` is verified against `issues` after an issues prune. An incremental fetch is never an enumeration of the workspace, so it cannot tell a deletion from a row that did not change, which is why it never prunes and says so in the run summary. `--full` and `--incremental` are mutually exclusive, because `--full` clears the cursors `--incremental` reads.

Archived rows are excluded from sync on purpose, so the prune keeps removing them locally. To see them, read live:

```bash
linear-pp-cli issues list --include-archived --state all --agent
linear-pp-cli relations list ENG-123 --include-archived --agent
```

Pair `--include-archived` with `--state all` on issues, since an archived issue keeps whatever state it was archived in. Rows gain `archived_at`, and both commands re-apply the predicate to store-served rows so an archived row cached by an earlier `--include-archived` read cannot leak into a later default read.

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
linear-pp-cli issues create --title "..." --team ENG --pp-session "$SESSION"

# Verify from local
linear-pp-cli issues list --data-source local --pp-session "$SESSION"

# Refresh every ~30 minutes for long sessions. --incremental is the cheap one
linear-pp-cli sync --incremental
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
- **`sync --full` is slow or paginates indefinitely** — Run `linear-pp-cli sync --incremental` after the first full sync — it cursors on updatedAt and only fetches changed rows.
- **FTS5 search returns no rows for a term you know exists** — Run `linear-pp-cli sync --incremental` to refresh the FTS index, or `linear-pp-cli doctor` to confirm the FTS triggers fired on the latest sync.
- **Agent accidentally mutated an issue it did not create** — Set `LINEAR_PP_CLI_TRUST_MODE=strict` in the environment — strict mode refuses any mutation on an issue ID not in the local pp_created ledger.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**Finesssee/linear-cli**](https://github.com/Finesssee/linear-cli) — Rust
- [**schpet/linear-cli**](https://github.com/schpet/linear-cli) — Ruby
- [**czottmann/linearis**](https://github.com/czottmann/linearis) — TypeScript
- [**dorkitude/linctl**](https://github.com/dorkitude/linctl) — Go
- [**evangodon/linear-cli**](https://github.com/evangodon/linear-cli) — Go
- [**linear-mcp**](https://github.com/tacticlaunch/mcp-linear) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
