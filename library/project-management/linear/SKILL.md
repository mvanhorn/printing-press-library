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

This skill drives the `linear-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install linear --cli-only
   ```
2. Verify: `linear-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/project-management/linear/cmd/linear-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Pulls your workspace into a local SQLite store with FTS5 search and runs compound queries that no live API call can answer in one round-trip — today view, bottleneck detection, project burndown, cycle comparison. Ships a thin linear_search + linear_execute MCP orchestration pair (with named multi-step intents for triage, standup, sprint plan, weekly update, and grooming) so agents reach the full surface in ~1K tokens instead of enumerating 60+ endpoint mirrors.

## Agent Contract

- Add `--agent` to commands unless a human-readable table is explicitly needed. It implies JSON, compact output, non-interactive mode, no color, and confirmation-safe scripting.
- Use `--data-source live` for closeout/state/description checks where current truth matters. Use `issues search` for duplicate checks; it refreshes stale issue search data or fails visibly. Use `--data-source local` or `similar` only when stale/offline local duplicate search is intentional.
- **Before filing new work, reconcile it.** `reconcile --title "..." --team ENG --agent` is read-only and answers one question with evidence: is this a duplicate, a relative, or genuinely new. Only pass `--execute` once you accept the decision. See the reconcile-before-filing recipe below.
- A missing `description` in compact output does not mean an empty issue body. Request it explicitly: `linear-pp-cli issues ENG-123 --agent --data-source live --select identifier,title,description,state.name,url`.
- Fetch several known issues in one call with comma-separated identifiers: `linear-pp-cli issues ENG-123,ENG-124 --agent`. The result array preserves caller order and removes duplicate identifiers; a missing member fails the whole read instead of returning a partial set.
- Prefer the canonical read and comment forms shown here. Common agent phrasing is accepted without changing behavior: `issues get|view|show ENG-123`, `documents get|view <ref>`, and `comments create` are compatibility aliases for `issues ENG-123`, `documents <ref>`, and `comments add`. The aliases accept the same global flags, comma reads, body files, targets, and media flags as their canonical commands; there is deliberately no `documents show` alias.
- Before passing label UUIDs to `issues create` or `issues edit`, run `linear-pp-cli labels list --team ENG --agent --select id,name,global,team.key`. Use only global labels or labels owned by the target issue team; the CLI preflights label ownership and refuses cross-team labels before mutating.
- Any flag that names a set of workflow states (`--state`, `--completed-group`, `--candidate-group`) resolves through the state group registry, not through a hardcoded list. Run `groups list` once per workspace so you know what `active` and `completed` actually mean there. The full vocabulary is in "State groups" below.
- Never pass multiline Markdown, shell snippets, GraphQL, logs, backticks, `$()` expansions, or media-rich content as inline shell arguments. Write the body to a file or stdin and use the `*-file` / `*-stdin` flags below.
- Writes are previewable. Every mutation honours `--dry-run` and answers a `{"event":"would_*"}` object naming the mutation and the exact input it would send. Deletes confirm unless `--yes` (implied by `--agent`) and accept `--ignore-missing`; creates accept `--idempotent`.

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

## Command Reference

Every command the binary ships is listed here. Regenerate this inventory at any time with `linear-pp-cli agent-context --pretty`, which is the machine-readable form of exactly this tree.

### Command families

**attachments**: Attachment cards on issues: get, create, link a URL, update, delete, and dedupe by URL

- `linear-pp-cli attachments <id>`: Get a single attachment
- `linear-pp-cli attachments create`: Create an attachment card on an issue
- `linear-pp-cli attachments delete <attachment-id>`: Delete an attachment
- `linear-pp-cli attachments for-url`: List the attachments recorded against a URL
- `linear-pp-cli attachments link-url`: Link a URL to an issue and let Linear unfurl it
- `linear-pp-cli attachments update <attachment-id>`: Update an attachment's title, subtitle, metadata, or icon

**auth**: Manage authentication for Linear

- `linear-pp-cli auth logout`: Clear stored credentials
- `linear-pp-cli auth set-token <token>`: Save an API token to the config file
- `linear-pp-cli auth setup`: Print steps for obtaining a credential (use --launch to open the URL)
- `linear-pp-cli auth status`: Show authentication status

**comments**: List, add, edit, delete, and resolve Linear comments

- `linear-pp-cli comments add [issue]`: Add a Linear comment
- `linear-pp-cli comments delete <comment-id>`: Delete a Linear comment
- `linear-pp-cli comments edit <comment-id>`: Edit a Linear comment
- `linear-pp-cli comments list [issue]`: List comments on an issue
- `linear-pp-cli comments resolve <comment-id>`: Resolve a comment thread
- `linear-pp-cli comments unresolve <comment-id>`: Reopen a previously resolved comment thread

**custom-views**: Linear custom views (saved filters): list, get, create, update, delete

- `linear-pp-cli custom-views create`: Create a custom view
- `linear-pp-cli custom-views delete <custom-view-id>`: Delete a custom view
- `linear-pp-cli custom-views get <custom-view-id>`: Get one custom view including its filter payload
- `linear-pp-cli custom-views list`: List custom views
- `linear-pp-cli custom-views update <custom-view-id>`: Update a custom view

**cycles**: Linear cycles: cycle-over-cycle comparison

- `linear-pp-cli cycles archive <cycle-id>`: Archive a cycle, unlinking every issue still assigned to it
- `linear-pp-cli cycles compare <cycle-a> <cycle-b>`: Compare two cycles side-by-side: completion %, scope added/cut, carryover
- `linear-pp-cli cycles create`: Create a cycle on a team with an explicit start and end
- `linear-pp-cli cycles list`: List cycles with their dates and progress, optionally filtered by team
- `linear-pp-cli cycles shift-all`: Shift a cycle and every cycle after it by a number of days
- `linear-pp-cli cycles start-upcoming <cycle-id>`: Start the next upcoming cycle as of midnight today
- `linear-pp-cli cycles update <cycle-id>`: Update a cycle's name, description, window, or completion time

**documents**: View, list, create, edit, delete, and restore Linear documents

- `linear-pp-cli documents [document-ref]`: View, list, create, edit, delete, and restore Linear documents
- `linear-pp-cli documents create`: Create a Linear document
- `linear-pp-cli documents delete <document-ref>`: Delete a Linear document
- `linear-pp-cli documents edit <document-id-or-slug>`: Edit a Linear document
- `linear-pp-cli documents get <document-ref>`: View a Linear document
- `linear-pp-cli documents list`: List Linear documents
- `linear-pp-cli documents unarchive <document-id>`: Restore a deleted Linear document

**favorites**: Sidebar favorites: get, create, reorder or refolder, delete

- `linear-pp-cli favorites <id>`: Get a single favorite
- `linear-pp-cli favorites create`: Favorite one Linear entity
- `linear-pp-cli favorites delete <favorite-id>`: Unfavorite an entity
- `linear-pp-cli favorites update <favorite-id>`: Reorder or refolder a favorite

**feedback**: Record feedback about this CLI (local by default; upstream opt-in)

- `linear-pp-cli feedback [text]`: Record feedback about this CLI (local by default; upstream opt-in)
- `linear-pp-cli feedback list`: List recent feedback entries

**groups**: Inspect the workflow-state groups that --state resolves against

- `linear-pp-cli groups check <group-or-type>`: Resolve one state token and show exactly what it matches
- `linear-pp-cli groups list`: List every effective state group and where it is declared

**initiatives**: Linear initiatives: get, list, search, resolve, and portfolio health rollup

- `linear-pp-cli initiatives archive <initiative>`: Archive a Linear initiative
- `linear-pp-cli initiatives create`: Create a Linear initiative
- `linear-pp-cli initiatives delete <initiative>`: Delete a Linear initiative
- `linear-pp-cli initiatives get <id>`: Get a single initiative
- `linear-pp-cli initiatives health`: Rolled-up portfolio view per initiative: project progress, milestone risk, slippage
- `linear-pp-cli initiatives link-project <initiative>`: Link a project to a Linear initiative
- `linear-pp-cli initiatives list`: List Linear initiatives
- `linear-pp-cli initiatives resolve <name>`: Resolve one Linear initiative name to its UUID
- `linear-pp-cli initiatives search <query>`: Search Linear initiatives by name
- `linear-pp-cli initiatives unarchive <initiative>`: Restore an archived Linear initiative
- `linear-pp-cli initiatives unlink-project <initiative>`: Unlink a project from a Linear initiative
- `linear-pp-cli initiatives update <initiative>`: Update a Linear initiative

**issues**: Get, list, or create Linear issues

- `linear-pp-cli issues [ID]`: Get, list, or create Linear issues
- `linear-pp-cli issues archive <issue>`: Archive a Linear issue
- `linear-pp-cli issues batch-create --file <path>`: Create up to 50 issues in one transaction and record them in the pp_created ledger
- `linear-pp-cli issues batch-update --ids <a,b,c>`: Apply one change to up to 50 issues in a single transaction
- `linear-pp-cli issues close-duplicate <issue> --of <canonical-issue>`: Close an issue as a duplicate: create the duplicate relation, then set the duplicate state
- `linear-pp-cli issues create`: Create a new Linear issue and record it in the pp_created ledger
- `linear-pp-cli issues delete <issue>`: Trash a Linear issue
- `linear-pp-cli issues edit <issue-id>`: Edit a Linear issue
- `linear-pp-cli issues get ID`: Get Linear issues by identifier
- `linear-pp-cli issues list`: List issues from the local sqlite store with filters
- `linear-pp-cli issues search <query>`: Search synced issues by text (alias for similar)
- `linear-pp-cli issues subscribe <issue>`: Subscribe a user to a Linear issue
- `linear-pp-cli issues unarchive <issue>`: Restore an archived Linear issue
- `linear-pp-cli issues unsubscribe <issue>`: Unsubscribe a user from a Linear issue

**labels**: Manage Linear issue labels: list, create, update, delete, retire, restore

- `linear-pp-cli labels create`: Create an issue label, workspace-wide or scoped to one team
- `linear-pp-cli labels delete <label-id>`: Permanently delete an issue label
- `linear-pp-cli labels list`: List issue labels, optionally filtered to labels safe for a team
- `linear-pp-cli labels restore <label-id>`: Restore a retired issue label so it can be applied again
- `linear-pp-cli labels retire <label-id>`: Retire an issue label so it can no longer be applied to new issues
- `linear-pp-cli labels update <label-id>`: Update an issue label's name, color, description, or parent

**milestones**: List project milestones at risk of missing their target date

- `linear-pp-cli milestones at-risk`: Rank portfolio milestones by projected slippage past target date
- `linear-pp-cli milestones create`: Create a project milestone
- `linear-pp-cli milestones delete <milestone-id>`: Delete a project milestone
- `linear-pp-cli milestones list`: List a project's milestones
- `linear-pp-cli milestones move <milestone-id>`: Move a project milestone and its issues to another project
- `linear-pp-cli milestones update <milestone-id>`: Update a project milestone

**notifications**: Read and triage the authenticated user's Linear inbox

- `linear-pp-cli notifications archive <notification-id>`: Archive one notification
- `linear-pp-cli notifications archive-all`: Archive every notification about one entity
- `linear-pp-cli notifications get <notification-id>`: Get one notification by id
- `linear-pp-cli notifications list`: List the authenticated user's notifications
- `linear-pp-cli notifications read <notification-id>`: Mark one notification as read
- `linear-pp-cli notifications read-all`: Mark every notification about one entity as read
- `linear-pp-cli notifications snooze <notification-id> --until <when>`: Snooze one notification until a given time
- `linear-pp-cli notifications snooze-all`: Snooze every notification about one entity
- `linear-pp-cli notifications unarchive <notification-id>`: Restore one archived notification
- `linear-pp-cli notifications unread <notification-id>`: Mark one notification as unread
- `linear-pp-cli notifications unread-all`: Mark every notification about one entity as unread
- `linear-pp-cli notifications unread-count`: Report the authenticated user's unread notification count
- `linear-pp-cli notifications unsnooze <notification-id>`: Wake one snoozed notification
- `linear-pp-cli notifications unsnooze-all`: Wake every snoozed notification about one entity

**pp-test**: List Linear issues this CLI has created (test-fixture ledger)

- `linear-pp-cli pp-test list`: List active fixtures (issues this CLI created and has not yet archived)
- `linear-pp-cli pp-test sessions`: List all distinct session tags with active fixtures

**profile**: Named sets of flags saved for reuse

- `linear-pp-cli profile delete <name>`: Remove a profile
- `linear-pp-cli profile list`: List saved profiles
- `linear-pp-cli profile save <name> [--<flag> <value> ...]`: Save the current invocation's non-default flags as a named profile
- `linear-pp-cli profile show <name>`: Show a profile's values as JSON
- `linear-pp-cli profile use <name>`: Print the flag values a profile will apply (does not execute anything)

**project-updates**: List, create, edit, and archive Linear project updates

- `linear-pp-cli project-updates archive <project-update-id>`: Archive a Linear project update
- `linear-pp-cli project-updates create`: Create a new Linear project update
- `linear-pp-cli project-updates list`: List project updates for a Linear project
- `linear-pp-cli project-updates unarchive <project-update-id>`: Restore an archived Linear project update
- `linear-pp-cli project-updates update <project-update-id>`: Edit a posted Linear project update

**projects**: Linear projects: get, list, search, resolve, and burndown projection

- `linear-pp-cli projects add-label <project-id>`: Attach a project label to a Linear project
- `linear-pp-cli projects burndown <project>`: Project a project's landing date from estimate vs measured velocity
- `linear-pp-cli projects create`: Create a Linear project
- `linear-pp-cli projects delete <project-id>`: Delete (trash) a Linear project
- `linear-pp-cli projects get <id>`: Get a single project
- `linear-pp-cli projects list`: List Linear projects
- `linear-pp-cli projects remove-label <project-id>`: Detach a project label from a Linear project
- `linear-pp-cli projects resolve <name>`: Resolve one Linear project name to its UUID
- `linear-pp-cli projects search <query>`: Search Linear projects by name
- `linear-pp-cli projects update <project-id>`: Update a Linear project

**reactions**: List, add, and remove emoji reactions

- `linear-pp-cli reactions add`: Add an emoji reaction to a comment, issue, project update, or initiative update
- `linear-pp-cli reactions list`: List the reactions on a comment, issue, project update, or initiative update
- `linear-pp-cli reactions remove <reaction-id>`: Remove a reaction

**relations**: Read and manage issue relations (blocks, duplicate, related, similar)

- `linear-pp-cli relations create <issue> --type <type> --to <issue>`: Create an issue relation (blocks, duplicate, related, similar)
- `linear-pp-cli relations delete <relation-id>`: Delete an issue relation by its relation UUID
- `linear-pp-cli relations list <issue>`: List every relation touching one issue, in both directions

**templates**: Workspace templates: get, list, create, update, delete

- `linear-pp-cli templates create`: Create a template
- `linear-pp-cli templates delete <template-id>`: Delete a template
- `linear-pp-cli templates get <template-id>`: Get one template including its templateData payload
- `linear-pp-cli templates list`: List workspace templates, optionally scoped by team or type
- `linear-pp-cli templates update <template-id>`: Update a template

**workflow-states**: Manage Linear workflow states: list, create, update, archive

- `linear-pp-cli workflow-states archive <state-id>`: Archive a workflow state
- `linear-pp-cli workflow-states create`: Create a workflow state on a team's board
- `linear-pp-cli workflow-states list`: List workflow states, optionally filtered by team
- `linear-pp-cli workflow-states update <state-id>`: Update a workflow state's name, color, description, or board position

### Standalone commands

- `linear-pp-cli analytics`: Run analytics queries on locally synced data
- `linear-pp-cli api [interface]`: Browse all API endpoints by interface name
- `linear-pp-cli audit-entry-types`: Get a single auditentrytype
- `linear-pp-cli authentication-session-responses`: Get a single authenticationsessionresponse
- `linear-pp-cli blocking`: Show issues you are blocking — sorted by downstream impact
- `linear-pp-cli bottleneck`: Find overloaded team members and blocked issues
- `linear-pp-cli doctor`: Check CLI health
- `linear-pp-cli email-intake-addresses <id>`: Get a single emailintakeaddress
- `linear-pp-cli initiative-relations <id>`: Get a single initiativerelation
- `linear-pp-cli initiative-to-projects <id>`: Get a single initiativetoproject
- `linear-pp-cli issue-priority-values`: Get a single issuepriorityvalue
- `linear-pp-cli load`: Show workload distribution per assignee
- `linear-pp-cli me`: Show current authenticated user
- `linear-pp-cli organizations`: Get a single organization
- `linear-pp-cli orphans`: Find items missing key fields like assignee or project
- `linear-pp-cli pp-cleanup`: Archive Linear issues this CLI created (scoped to the pp_created ledger)
- `linear-pp-cli project-labels <id>`: Get a single projectlabel
- `linear-pp-cli project-milestones <id>`: Get a single projectmilestone
- `linear-pp-cli project-relations <id>`: Get a single projectrelation
- `linear-pp-cli project-statuses <id>`: Get a single projectstatus
- `linear-pp-cli rate-limit`: Show the current Linear API rate limit budget
- `linear-pp-cli reconcile`: Decide whether proposed work is a duplicate, a relative, or genuinely new, and optionally act on it
- `linear-pp-cli release-notes <id>`: Get a single releasenote
- `linear-pp-cli release-pipelines`: Get a single releasepipeline
- `linear-pp-cli release-stages <id>`: Get a single releasestage
- `linear-pp-cli releases <id>`: Get a single release
- `linear-pp-cli roadmap-to-projects <id>`: Get a single roadmaptoproject
- `linear-pp-cli roadmaps <id>`: Get a single roadmap
- `linear-pp-cli similar [query]`: Find potentially duplicate issues using fuzzy text search
- `linear-pp-cli slipped`: Show issues that slipped from the previous cycle into the current cycle
- `linear-pp-cli sql <query>`: Run read-only SQL against the local store
- `linear-pp-cli stale`: Find items with no updates in N days
- `linear-pp-cli sync`: Sync Linear data to local SQLite store
- `linear-pp-cli teams`: Get a single team
- `linear-pp-cli today`: Show your issues for today across all teams
- `linear-pp-cli unblocked`: Show blocked issues whose blockers are now all closed
- `linear-pp-cli user-settingses`: Get a single usersettings
- `linear-pp-cli users`: Get a single user
- `linear-pp-cli velocity`: Show sprint velocity trends over recent cycles
- `linear-pp-cli which [query]`: Find the command that implements a capability

`completion <shell>`, `help [command]` and `version` are the usual cobra built-ins. `completion bash`, `completion zsh`, `completion fish` and `completion powershell` each print a shell autocompletion script.

`roadmaps` and `roadmap-to-projects` are marked deprecated: Linear replaced Roadmap and RoadmapToProject with Initiative and InitiativeToProject. They still work. Reach for the `initiatives` family instead.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
linear-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

For duplicate checks, `linear-pp-cli which "search issues by text" --agent` should point to `issues search`; use that instead of inventing `issues search --help` fallbacks or raw SQL. If the local issue cache is stale, `issues search` refreshes it or fails with a typed freshness error; agents should not jump to raw GraphQL just because the cache was stale.
For parent/sub-issue linking, `linear-pp-cli which "set issue parent" --agent` should point to `issues edit --parent`.

## State groups

A group names a set of workflow states. Every flag that selects states resolves through the same registry, so there is exactly one place to say what "open" means in your workspace.

**The vocabulary.** `--state` (on `today`, `bottleneck`, `blocking`, `slipped`, `issues list`, `unblocked`) accepts:

| Token | Meaning |
|-------|---------|
| a group name | `active`, `completed`, `triage`, `backlog`, `unstarted`, `started`, `canceled`, `duplicate`, plus anything you declare. Run `groups list`. |
| `all` | Reserved. Emits no state predicate at all, so a new Linear state type can never be silently excluded. |
| a raw type | One of the seven `WorkflowState.type` values: `triage`, `backlog`, `unstarted`, `started`, `completed`, `canceled`, `duplicate`. |
| `type:<type>` | The raw API type, bypassing a group of the same name that shadows it. |
| `name:<state name>` | One literal state name in your workspace, bypassing every group. |

Tokens are trimmed and lowercased before they reach either the live query or the local re-check, so `--state Active`, `--state " active"` and `--state active` are the same run.

`--completed-group` (on `projects burndown`, `cycles compare`, `milestones at-risk`) and `--candidate-group` (on `reconcile`) take the same tokens and default to `completed` and `active`.

**`duplicate` is not open.** The built-in `active` group is the positive set `["triage","backlog","unstarted","started"]`. It used to be the negative set "not completed, not canceled", which matched `duplicate`, so issues closed as duplicates were reported as open work. If you want the old behaviour back, declare it (below).

**Declaring your own.** Declarations live in `groups.toml` next to your config file, never in `config.toml`, because `auth` writes rewrite `config.toml` wholesale and would delete them. `groups list` prints the resolved path in `meta.groups_path`, and `$LINEAR_GROUPS` overrides it.

```toml
schema_version = 1

# Workspace-wide: available to every team.
[state_groups.wip]
description = "Actually being worked, plus our review column"
types = ["started"]
names = ["In Review"]

# Restore the pre-fix meaning of `active`, if that is what you want.
[state_groups.active]
types = ["triage", "backlog", "unstarted", "started", "duplicate"]

# Team-scoped: replaces, never merges with, the workspace group of the same name.
[team_state_groups.ENG.wip]
types = ["started"]
names = ["In Review", "QA"]
```

Membership is the union of the two keys: a state belongs to the group when its `type` is listed in `types` OR its `name` matches one of `names`, case-insensitively. There is no negation and no nesting, on purpose. Precedence is team, then workspace, then builtin, and `groups list` names what shadows what.

Check a token before you trust it:

```bash
linear-pp-cli groups check wip --team ENG --agent
```

That prints the predicate, the `WorkflowStateFilter` it will send, the concrete states it hits in your workspace, and any declared `names` that match no state, which is how a typo in `groups.toml` surfaces before it silently narrows a report.

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

Linear personal API keys go in the `Authorization` header verbatim — no `Bearer` prefix. Run `linear-pp-cli auth set-token <your-personal-API-key>` to save your key (no Bearer prefix needed for Linear personal API keys), or export `LINEAR_API_KEY=lin_api_...`. Personal API keys are workspace-scoped; the doctor command validates auth, API connectivity, and store health in one shot.

Run `linear-pp-cli doctor` to verify setup.

## Freshness and Data Sources (read this before driving in an agent loop)

Commands fall into three categories with different data-source semantics. Use `--data-source auto|live|local` to control where reads come from; use `--max-age <duration>` to set the threshold for the "your local data is stale" hint.

**Category 1: Live-first with local fallback** (the spec-emitted commands and the v4-refactored `issues list/get`)

- `attachments <id>`, `projects get|list|search`, `teams <id>`, `initiatives get|list|search`, `issues <id>`, `issues list`, `cycles list`, `labels list`, `templates list|get`, `custom-views list|get`, `milestones list`, `project-updates list`, `favorites <id>`, `project-statuses <id>`, and the other single-object reads.
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

- The issue lifecycle (`issues create`, `edit`, `archive`, `unarchive`, `delete`, `subscribe`, `unsubscribe`, `close-duplicate`, `batch-create`, `batch-update`), `relations create|delete`, `comments add|edit|delete|resolve|unresolve`, `documents create|edit|delete|unarchive`, `labels create|update|delete|retire|restore`, `workflow-states create|update|archive`, `cycles create|update|archive|shift-all|start-upcoming`, `projects create|update|delete|add-label|remove-label`, `milestones create|update|delete|move`, `initiatives create|update|archive|unarchive|delete|link-project|unlink-project`, `project-updates create|update|archive|unarchive`, `attachments create|link-url|update|delete`, `templates create|update|delete`, `custom-views create|update|delete`, `favorites create|update|delete`, `reactions add|remove`, every `notifications` verb, `reconcile --execute`, and `pp-cleanup`.
- Always hit the API. On success, the HTTP response cache is invalidated AND the new/changed entity is written back to the local store, so a subsequent `issues list --data-source local` sees the mutation without requiring another sync.
- Every one of them honours `--dry-run`, answering a `{"event":"would_*"}` object with the mutation name and the exact input. Deletes confirm unless `--yes` and honour `--ignore-missing`; creates honour `--idempotent`. Linear answers every GraphQL call with HTTP 200, so the no-op switches match on the error prose rather than on a status code.

**Live-only reads**

- `comments list`, `documents <id-or-slug>`, `documents list`
- These read the current Linear API because comments and working-session docs are collaboration surfaces where stale local state is more misleading than helpful.
- `unblocked`, `notifications` (every verb) and `rate-limit` are live only for a different reason: there is no local snapshot to answer from. `unblocked` refuses `--data-source local` with a usage error rather than pretending. `relations list` is the exception in this neighbourhood: it reads live and writes through to the `issue_relations` table, so `relations list ENG-123 --data-source local` works offline after one live read.

**Label discovery**

- `labels list --team ENG`
- Default (`--data-source auto`) reads live and returns global labels plus labels owned by the named team. `--data-source local` reads the synced `issue_labels` table after `linear-pp-cli sync`.
- Use the returned IDs for `issues create --label` or `issues edit --label`; cross-team label IDs are rejected before the issue mutation is sent.

**The budget-conscious agent loop:**

```bash
# 1. Hydrate once (paged crawl, then a prune that reconciles upstream deletions)
linear-pp-cli sync

# 2. Read freely — store-backed, zero budget
linear-pp-cli today
linear-pp-cli bottleneck --team ENG --data-source local

# 3. Mutate — write-back keeps the store fresh
linear-pp-cli issues create --title "..." --team ENG --pp-session "$SESSION"

# 4. Verify the mutation from local (no extra API call)
linear-pp-cli issues list --data-source local --pp-session "$SESSION"

# 5. Re-sync every ~30 minutes if the session is long. --incremental is the cheap one
linear-pp-cli sync --incremental
```

**Cleanup contract:**

Every `issues create` and every `issues batch-create` record the new tickets in a local `pp_created` table tagged with the session (default: timestamp, override with `--pp-session <tag>`, `--session <tag>` or the `PP_SESSION` env var). `pp-test list` shows the active fixtures, `pp-test sessions` lists every session tag that still has some, and `pp-cleanup --session <tag>` archives only those tickets via the real Linear archive mutation. `--trust-mode strict` refuses mutations on issues absent from `pp_created` across the whole write surface (see the trust-mode entry above). Pair it with a session tag for a hard floor against agent-driven workspace pollution.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout. Failures are JSON too: in `--agent`/`--json` mode every typed error (usage, not-found, auth, API, rate-limit, config) is a one-line envelope on stdout, so piping stdout straight into a JSON parser is always safe:

  ```json
  {"error":"document \"missing-doc\" not found","code":3,"type":"not_found"}
  ```

  The process still exits with the typed code from the Exit Codes table; `type` is the machine-readable name for it (`usage`, `not_found`, `auth`, `api`, `partial_failure`, `rate_limit`, `config`). No `2>&1 | python json` defensive wrappers needed on read paths.
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  linear-pp-cli projects list --team ENG --agent --select id,name,team.key
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

### Output contract

Every JSON response belongs to exactly one of three shapes. Which one a command uses is a property of the command, not of the run, so a caller can hard-code the access path.

**Reads answer `{results, meta}`.** `results` is the payload, an array for list and search commands and an object for single-subject rollups such as `reconcile`. It is never `null`: an empty read answers `{"results": [], "meta": {...}}`. `meta` always carries `source` (`live`, `local`, or `mixed` on `issues search --live`) and `resource_type`, and adds `synced_at` on local reads, `reason` for why local was chosen, `count`, and the scope that produced the result (`query`, `team_scope`). `--select` descends into `results` and leaves `meta` intact, so field filtering never costs you provenance.

```bash
linear-pp-cli projects list --team ENG --agent --select id,name,team.key
# {"results":[{"id":"…","name":"…","team":{"key":"ENG"}}],"meta":{"source":"live","resource_type":"projects","count":1,"team_scope":"ENG","reason":"user_requested"}}
```

`projects list`, `projects search`, `initiatives list` and `initiatives search` used to emit a bare array and now emit this envelope. If you have a script that indexed the output directly, `jq '.[]'` becomes `jq '.results[]'`.

**Mutations answer an event object.** A write reports what it did: `success`, the affected entity, and, under `--dry-run`, a `{"event":"would_*"}` preview naming the GraphQL mutation and the exact input it would have sent. Writes never wrap themselves in `results`, because a write has one subject and no provenance question to answer.

**Resolvers answer a bare object.** `projects resolve` and `initiatives resolve` print the single matched entity at the top level, so `.id` is one hop away. An ambiguous or missing name is a typed usage error carrying the candidate list, not an empty success. This is the one read family that is deliberately not enveloped, because its whole contract is exactly one object or a failure.

A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set, so piped and agent consumers get pure JSON on stdout.

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
| 6 | Partial failure (a multi-step write completed some steps and not others) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

Exit 6 is the one worth handling explicitly. It means a plan ran partway: a duplicate close whose relation landed but whose state write failed, or a `reconcile --execute` two-step whose second mutation failed after the issue was created. The response names the step that failed and how to finish it. `--allow-partial-failure` downgrades it to a stderr warning and exit 0 when a partial result is acceptable.

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
