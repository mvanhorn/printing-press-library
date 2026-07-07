# Azure DevOps CLI

**The fastest Azure DevOps CLI — offline-first, agent-native, and the only one that answers 'why is this sprint burning' without opening a browser.**

azure-devops-pp-cli syncs your work items, builds, and PRs to a local SQLite mirror and lets you query them with SQL or structured --agent output. It matches everything `az devops` and the Microsoft MCP server can do, then adds fifteen cross-entity analytics commands that require no live API calls — sprint velocity, PR review queues, scope creep detection, cycle time analysis, and more.

## Install

The recommended path installs both the `azure-devops-pp-cli` binary and the `pp-azure-devops` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install azure-devops
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install azure-devops --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install azure-devops --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install azure-devops --agent claude-code
npx -y @mvanhorn/printing-press-library install azure-devops --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/azure-devops/cmd/azure-devops-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/azure-devops-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install azure-devops --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-azure-devops --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-azure-devops --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install azure-devops --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/azure-devops-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `CORE_USERNAME` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/azure-devops/cmd/azure-devops-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "azure-devops": {
      "command": "azure-devops-pp-mcp",
      "env": {
        "CORE_USERNAME": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Set AZURE_DEVOPS_TOKEN to a Personal Access Token. Generate one at dev.azure.com/{org}/_usersSettings/tokens with scopes: Work Items (read/write), Code (read), Build (read), Release (read/execute). Run `azure-devops-pp-cli auth setup --launch` to open the token page. Save with `azure-devops-pp-cli auth set-token <your-pat>`.

## Quick Start

```bash
# Verify config and connectivity before syncing
azure-devops-pp-cli doctor --dry-run

# Preview what standup will fetch once auth is configured
azure-devops-pp-cli standup --dry-run

# Morning digest: your PRs, work items, and failed builds
azure-devops-pp-cli standup --agent

# Which PRs need your review right now (build green + no votes yet)
azure-devops-pp-cli pr review-queue --agent

# Team velocity trend over the last 6 sprints
azure-devops-pp-cli velocity --sprints 6 --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`standup`** — See your PRs awaiting review, in-progress work items, and newly failed builds in one command — ready before the standup meeting.

  _Use this when an agent needs a quick briefing on a developer's current work state before creating tasks or PRs on their behalf._

  ```bash
  azure-devops-pp-cli standup --agent
  ```
- **`velocity`** — See sprint-over-sprint velocity with trend line for the current team — points completed vs committed, across the last N sprints.

  _Use this when an agent needs to assess team capacity or forecast delivery dates based on historical throughput._

  ```bash
  azure-devops-pp-cli velocity --sprints 6 --agent
  ```
- **`work sprint-creep`** — See which work items were added to the current sprint after it started, with total story points of scope added mid-sprint.

  _Use this when an agent needs to identify whether a sprint is at risk due to scope expansion after commit._

  ```bash
  azure-devops-pp-cli work sprint-creep --team ENG --json
  ```
- **`work rollover`** — Find work items that have been moved to a new sprint more than once — the chronic blockers that keep slipping through sprint planning.

  _Use this when an agent needs to identify persistent delivery risks before sprint planning._

  ```bash
  azure-devops-pp-cli work rollover --min-rollovers 2 --agent
  ```
- **`pipeline flaky`** — Identify build stages with the highest intermittent failure rate over the last N runs — the stages that fail and then pass without code changes.

  _Use this when an agent needs to identify which pipeline stages are unreliable test signals before acting on build failures._

  ```bash
  azure-devops-pp-cli pipeline flaky --definition-id 42 --last 30 --agent
  ```
- **`wit cycle-time`** — Measure average time from Active to Done per work item type over a date range — the team's true delivery throughput.

  _Use this when an agent needs to estimate realistic delivery timelines based on historical cycle time._

  ```bash
  azure-devops-pp-cli wit cycle-time --type Bug --weeks 12 --agent
  ```
- **`git branch-health`** — See a health dashboard for every repo in the project: default branch build status, last commit age, and open PR count — in one table.

  _Use this when an agent needs to assess overall code health across a multi-repo project before planning changes._

  ```bash
  azure-devops-pp-cli git branch-health --agent --select repo,buildStatus,lastCommitDays,openPRs
  ```
- **`work area-load`** — See open work item counts and total story points grouped by area path — to spot overloaded teams or abandoned backlogs.

  _Use this when an agent needs to identify which teams have the most outstanding work before assigning new items._

  ```bash
  azure-devops-pp-cli work area-load --state Active --agent
  ```
- **`pr aging`** — Find pull requests with no reviewer activity for N or more days, grouped by author — to surface forgotten PRs before they go stale.

  _Use this when an agent needs to identify PRs that are at risk of losing context due to review delays._

  ```bash
  azure-devops-pp-cli pr aging --days 3 --agent
  ```
- **`branches stale`** — List branches that are safe to delete: already merged into default, no open PRs, and no commits in the last N days.

  _Use this when an agent is helping clean up a repository's branch proliferation._

  ```bash
  azure-devops-pp-cli branches stale --repo myapp --days 30 --agent
  ```
- **`builds cost`** — See agent minutes consumed per pipeline per week over the last N weeks — to find pipelines that are growing in CI cost.

  _Use this when an agent needs to identify which pipelines are contributing to growing CI/CD costs._

  ```bash
  azure-devops-pp-cli builds cost --weeks 8 --agent
  ```

### Agent-native plumbing
- **`pr review-queue`** — See only the PRs where you are a required reviewer, the build is passing, and no other required reviewer has voted yet — ordered by readiness.

  _Use this when an agent needs to identify which PRs a developer should review first to unblock teammates._

  ```bash
  azure-devops-pp-cli pr review-queue --agent --select title,buildStatus,waitingReviewers
  ```
- **`release gate-queue`** — See all release gates and YAML pipeline stages currently waiting for your approval across every pipeline, with how long each has been waiting.

  _Use this when an agent needs to identify which deployments are blocked on human approval and how urgent each is._

  ```bash
  azure-devops-pp-cli release gate-queue --agent --select pipelineName,stageName,waitMinutes
  ```
- **`work diff`** — Show exactly which fields changed between two revisions of a work item — a git-style diff for specification and acceptance criteria.

  _Use this when an agent needs to understand what changed in a work item's requirements between two points in time._

  ```bash
  azure-devops-pp-cli work diff --id 4812 --from 3 --to 7 --agent
  ```
- **`git commit-builds`** — Given a commit SHA, see every build and pipeline run that included that commit and whether it passed or failed.

  _Use this when an agent needs to determine whether a specific code change has been validated by CI._

  ```bash
  azure-devops-pp-cli git commit-builds --sha a1b2c3d4 --repo myapp --agent
  ```

## Recipes


### Morning standup in one command

```bash
azure-devops-pp-cli standup --agent --select myPRs,myWorkItems,failedBuilds
```

Returns your daily briefing as structured JSON: PRs waiting for your review, work items in progress, and builds that failed since yesterday.

### Find PRs ready for your review

```bash
azure-devops-pp-cli pr review-queue --agent --select title,repo,buildStatus,waitingReviewers
```

Shows only PRs where you are required, the build is green, and no other required reviewer has voted — ordered by most ready first.

### Sprint velocity trend for capacity planning

```bash
azure-devops-pp-cli velocity --sprints 8 --team ENG --agent --select sprint,committed,completed,velocityPoints
```

Returns 8-sprint velocity history as structured JSON for an agent to compute averages and forecast next sprint capacity.

### Scope creep check at sprint mid-point

```bash
azure-devops-pp-cli work sprint-creep --team ENG --agent --select addedItems,addedPoints,percentCreep
```

Shows what was added to the current sprint after it started, with story points and percentage of total sprint scope that crept in.

### Cross-repo health before a release

```bash
azure-devops-pp-cli git branch-health --agent --select repo,buildStatus,lastCommitDays,openPRs
```

One-line summary of all repos: default branch build status, days since last commit, and open PR count — to confirm everything is green before a release.

## Usage

Run `azure-devops-pp-cli --help` for the full command reference and flag list.

## Commands

### apis

Manage apis

- **`azure-devops-pp-cli apis avatar-remove-project-avatar`** - Removes the avatar for the project.
- **`azure-devops-pp-cli apis avatar-set-project-avatar`** - Sets the avatar for the project.
- **`azure-devops-pp-cli apis categorized-teams-get`** - Gets list of user readable teams in a project and teams user is member of (excluded from readable list).
- **`azure-devops-pp-cli apis processes-get`** - Get a process by ID.
- **`azure-devops-pp-cli apis processes-list`** - Get a list of processes.
- **`azure-devops-pp-cli apis projects-create`** - Queues a project to be created. Use the [GetOperation](../../operations/operations/get) to periodically check for create project status.
- **`azure-devops-pp-cli apis projects-delete`** - Queues a project to be deleted. Use the [GetOperation](../../operations/operations/get) to periodically check for delete project status.
- **`azure-devops-pp-cli apis projects-get`** - Get project with the specified id or name, optionally including capabilities.
- **`azure-devops-pp-cli apis projects-get-project-properties`** - Get a collection of team project properties.
- **`azure-devops-pp-cli apis projects-list`** - Get all projects in the organization that the authenticated user has access to.
- **`azure-devops-pp-cli apis projects-set-project-properties`** - Create, update, and delete team project properties.
- **`azure-devops-pp-cli apis projects-update`** - Update an existing project's name, abbreviation, description, or restore a project.
- **`azure-devops-pp-cli apis teams-create`** - Create a team in a team project.

Possible failure scenarios
Invalid project name/ID (project doesn't exist) 404
Invalid team name or description 400
Team already exists 400
Insufficient privileges 400
- **`azure-devops-pp-cli apis teams-delete`** - Delete a team.
- **`azure-devops-pp-cli apis teams-get`** - Get a specific team.
- **`azure-devops-pp-cli apis teams-get-all-teams`** - Get a list of all teams.
- **`azure-devops-pp-cli apis teams-get-team-members-with-extended-properties`** - Get a list of members for a specific team.
- **`azure-devops-pp-cli apis teams-get-teams`** - Get a list of teams.
- **`azure-devops-pp-cli apis teams-update`** - Update a team's name and/or description.

### build-apis

Manage build apis

- **`azure-devops-pp-cli build-apis artifacts-create`** - Associates an artifact with a build.
- **`azure-devops-pp-cli build-apis artifacts-list`** - Gets all artifacts for a build.
- **`azure-devops-pp-cli build-apis attachments-get`** - Gets a specific attachment.
- **`azure-devops-pp-cli build-apis attachments-list`** - Gets the list of attachments of a specific type that are associated with a build.
- **`azure-devops-pp-cli build-apis authorizedresources-authorize-project-resources`** - Authorizedresources authorize project resources
- **`azure-devops-pp-cli build-apis authorizedresources-list`** - Authorizedresources list
- **`azure-devops-pp-cli build-apis badge-get-build-badge-data`** - Gets a badge that indicates the status of the most recent build for the specified branch.
- **`azure-devops-pp-cli build-apis builds-delete`** - Deletes a build.
- **`azure-devops-pp-cli build-apis builds-get`** - Gets a build
- **`azure-devops-pp-cli build-apis builds-get-build-changes`** - Gets the changes associated with a build
- **`azure-devops-pp-cli build-apis builds-get-build-log`** - Gets an individual log file for a build.
- **`azure-devops-pp-cli build-apis builds-get-build-logs`** - Gets the logs for a build.
- **`azure-devops-pp-cli build-apis builds-get-build-work-items-refs`** - Gets the work items associated with a build. Only work items in the same project are returned.
- **`azure-devops-pp-cli build-apis builds-get-build-work-items-refs-from-commits`** - Gets the work items associated with a build, filtered to specific commits.
- **`azure-devops-pp-cli build-apis builds-get-changes-between-builds`** - Gets the changes made to the repository between two given builds.
- **`azure-devops-pp-cli build-apis builds-get-retention-leases-for-build`** - Gets all retention leases that apply to a specific build.
- **`azure-devops-pp-cli build-apis builds-get-work-items-between-builds`** - Gets all the work items between two builds.
- **`azure-devops-pp-cli build-apis builds-list`** - Gets a list of builds.
- **`azure-devops-pp-cli build-apis builds-queue`** - Queues a build
- **`azure-devops-pp-cli build-apis builds-update-build`** - Updates a build.
- **`azure-devops-pp-cli build-apis builds-update-builds`** - Updates multiple builds.
- **`azure-devops-pp-cli build-apis definitions-create`** - Creates a new definition.
- **`azure-devops-pp-cli build-apis definitions-delete`** - Deletes a definition and all associated builds.
- **`azure-devops-pp-cli build-apis definitions-get`** - Gets a definition, optionally at a specific revision.
- **`azure-devops-pp-cli build-apis definitions-get-definition-revisions`** - Gets all revisions of a definition.
- **`azure-devops-pp-cli build-apis definitions-list`** - Gets a list of definitions.
- **`azure-devops-pp-cli build-apis definitions-restore-definition`** - Restores a deleted definition
- **`azure-devops-pp-cli build-apis definitions-update`** - Updates an existing build definition.  In order for this operation to succeed, the value of the "Revision" property of the request body must match the existing build definition's. It is recommended that you obtain the existing build definition by using GET, modify the build definition as necessary, and then submit the modified definition with PUT.
- **`azure-devops-pp-cli build-apis folders-create`** - Creates a new folder.
- **`azure-devops-pp-cli build-apis folders-delete`** - Deletes a definition folder. Definitions and their corresponding builds will also be deleted.
- **`azure-devops-pp-cli build-apis folders-list`** - Gets a list of build definition folders.
- **`azure-devops-pp-cli build-apis folders-update`** - Updates an existing folder at given  existing path
- **`azure-devops-pp-cli build-apis general-settings-get`** - Gets pipeline general settings.
- **`azure-devops-pp-cli build-apis general-settings-update`** - Updates pipeline general settings.
- **`azure-devops-pp-cli build-apis latest-get`** - Gets the latest build for a definition, optionally scoped to a specific branch.
- **`azure-devops-pp-cli build-apis leases-add`** - Adds new leases for pipeline runs.
- **`azure-devops-pp-cli build-apis leases-delete`** - Removes specific retention leases.
- **`azure-devops-pp-cli build-apis leases-get`** - Returns the details of the retention lease given a lease id.
- **`azure-devops-pp-cli build-apis leases-get-retention-leases-by-minimal-retention-leases`** - Returns any leases matching the specified MinimalRetentionLeases
- **`azure-devops-pp-cli build-apis leases-update`** - Updates the duration or pipeline protection status of a retention lease.
- **`azure-devops-pp-cli build-apis metrics-get-definition-metrics`** - Gets build metrics for a definition.
- **`azure-devops-pp-cli build-apis metrics-get-project-metrics`** - Gets build metrics for a project.
- **`azure-devops-pp-cli build-apis options-list`** - Gets all build definition options supported by the system.
- **`azure-devops-pp-cli build-apis properties-get-build-properties`** - Gets properties for a build.
- **`azure-devops-pp-cli build-apis properties-get-definition-properties`** - Gets properties for a definition.
- **`azure-devops-pp-cli build-apis properties-update-build-properties`** - Updates properties for a build.
- **`azure-devops-pp-cli build-apis properties-update-definition-properties`** - Updates properties for a definition.
- **`azure-devops-pp-cli build-apis report-get`** - Gets a build report.
- **`azure-devops-pp-cli build-apis resources-authorize-definition-resources`** - Resources authorize definition resources
- **`azure-devops-pp-cli build-apis resources-list`** - Resources list
- **`azure-devops-pp-cli build-apis retention-get`** - Gets the project's retention settings.
- **`azure-devops-pp-cli build-apis retention-update`** - Updates the project's retention settings.
- **`azure-devops-pp-cli build-apis settings-get`** - Gets the build settings.
- **`azure-devops-pp-cli build-apis settings-update`** - Updates the build settings.
- **`azure-devops-pp-cli build-apis source-providers-get-file-contents`** - Gets the contents of a file in the given source code repository.
- **`azure-devops-pp-cli build-apis source-providers-get-path-contents`** - Gets the contents of a directory in the given source code repository.
- **`azure-devops-pp-cli build-apis source-providers-get-pull-request`** - Gets a pull request object from source provider.
- **`azure-devops-pp-cli build-apis source-providers-list`** - Get a list of source providers and their capabilities.
- **`azure-devops-pp-cli build-apis source-providers-list-branches`** - Gets a list of branches for the given source code repository.
- **`azure-devops-pp-cli build-apis source-providers-list-repositories`** - Gets a list of source code repositories.
- **`azure-devops-pp-cli build-apis source-providers-list-webhooks`** - Gets a list of webhooks installed in the given source code repository.
- **`azure-devops-pp-cli build-apis source-providers-restore-webhooks`** - Recreates the webhooks for the specified triggers in the given source code repository.
- **`azure-devops-pp-cli build-apis stages-update`** - Update a build stage
- **`azure-devops-pp-cli build-apis status-get`** - <p>Gets the build status for a definition, optionally scoped to a specific branch, stage, job, and configuration.</p> <p>If there are more than one, then it is required to pass in a stageName value when specifying a jobName, and the same rule then applies for both if passing a configuration parameter.</p>
- **`azure-devops-pp-cli build-apis tags-add-build-tag`** - Adds a tag to a build.
- **`azure-devops-pp-cli build-apis tags-add-build-tags`** - Adds tags to a build.
- **`azure-devops-pp-cli build-apis tags-add-definition-tag`** - Adds a tag to a definition
- **`azure-devops-pp-cli build-apis tags-add-definition-tags`** - Adds multiple tags to a definition.
- **`azure-devops-pp-cli build-apis tags-delete-build-tag`** - Removes a tag from a build. NOTE: This API will not work for tags with special characters. To remove tags with special characters, use the PATCH method instead (in 6.0+)
- **`azure-devops-pp-cli build-apis tags-delete-definition-tag`** - Removes a tag from a definition. NOTE: This API will not work for tags with special characters. To remove tags with special characters, use the PATCH method instead (in 6.0+)
- **`azure-devops-pp-cli build-apis tags-delete-tag`** - Removes a tag from builds, definitions, and from the tag store
- **`azure-devops-pp-cli build-apis tags-get-build-tags`** - Gets the tags for a build.
- **`azure-devops-pp-cli build-apis tags-get-definition-tags`** - Gets the tags for a definition.
- **`azure-devops-pp-cli build-apis tags-get-tags`** - Gets a list of all build tags in the project.
- **`azure-devops-pp-cli build-apis tags-update-build-tags`** - Adds/Removes tags from a build.
- **`azure-devops-pp-cli build-apis tags-update-definition-tags`** - Adds/Removes tags from a definition.
- **`azure-devops-pp-cli build-apis templates-delete`** - Deletes a build definition template.
- **`azure-devops-pp-cli build-apis templates-get`** - Gets a specific build definition template.
- **`azure-devops-pp-cli build-apis templates-list`** - Gets all definition templates.
- **`azure-devops-pp-cli build-apis templates-save-template`** - Updates an existing build definition template.
- **`azure-devops-pp-cli build-apis timeline-get`** - Gets details for a build
- **`azure-devops-pp-cli build-apis yaml-get`** - Converts a definition to YAML, optionally at a specific revision.

### git-apis

Manage git apis

- **`azure-devops-pp-cli git-apis annotated-tags-create`** - Create an annotated tag.

Repositories have both a name and an identifier. Identifiers are globally unique, but several projects
may contain a repository of the same name. You don't need to include the project if you specify a
repository by ID. However, if you specify a repository by name, you must also specify the project (by name or ID).
- **`azure-devops-pp-cli git-apis annotated-tags-get`** - Get an annotated tag.

Repositories have both a name and an identifier. Identifiers are globally unique, but several projects
may contain a repository of the same name. You don't need to include the project if you specify a
repository by ID. However, if you specify a repository by name, you must also specify the project (by name or ID).
- **`azure-devops-pp-cli git-apis blobs-get-blob`** - Get a single blob.

Repositories have both a name and an identifier. Identifiers are globally unique,
but several projects may contain a repository of the same name. You don't need to include
the project if you specify a repository by ID. However, if you specify a repository by name,
you must also specify the project (by name or ID).
- **`azure-devops-pp-cli git-apis blobs-get-blobs-zip`** - Gets one or more blobs in a zip file download.
- **`azure-devops-pp-cli git-apis cherry-picks-create`** - Cherry pick a specific commit or commits that are associated to a pull request into a new branch.
- **`azure-devops-pp-cli git-apis cherry-picks-get-cherry-pick`** - Retrieve information about a cherry pick operation by cherry pick Id.
- **`azure-devops-pp-cli git-apis cherry-picks-get-cherry-pick-for-ref-name`** - Retrieve information about a cherry pick operation for a specific branch. This operation is expensive due to the underlying object structure, so this API only looks at the 1000 most recent cherry pick operations.
- **`azure-devops-pp-cli git-apis commits-get`** - Retrieve a particular commit.
- **`azure-devops-pp-cli git-apis commits-get-changes`** - Retrieve changes for a particular commit.
- **`azure-devops-pp-cli git-apis commits-get-commits-batch`** - Retrieve git commits for a project matching the search criteria
- **`azure-devops-pp-cli git-apis commits-get-push-commits`** - Retrieve a list of commits associated with a particular push.
- **`azure-devops-pp-cli git-apis diffs-get`** - Find the closest common commit (the merge base) between base and target commits, and get the diff between either the base and target commits or common and target commits.
- **`azure-devops-pp-cli git-apis forks-create-fork-sync-request`** - Request that another repository's refs be fetched into this one. It syncs two existing forks. To create a fork, please see the <a href="https://docs.microsoft.com/en-us/rest/api/vsts/git/repositories/create?view=azure-devops-rest-5.1"> repositories endpoint</a>
- **`azure-devops-pp-cli git-apis forks-get-fork-sync-request`** - Get a specific fork sync operation's details.
- **`azure-devops-pp-cli git-apis forks-get-fork-sync-requests`** - Retrieve all requested fork sync operations on this repository.
- **`azure-devops-pp-cli git-apis forks-list`** - Retrieve all forks of a repository in the collection.
- **`azure-devops-pp-cli git-apis import-requests-create`** - Create an import request.
- **`azure-devops-pp-cli git-apis import-requests-get`** - Retrieve a particular import request.
- **`azure-devops-pp-cli git-apis import-requests-query`** - Retrieve import requests for a repository.
- **`azure-devops-pp-cli git-apis import-requests-update`** - Retry or abandon a failed import request.

There can only be one active import request associated with a repository. Marking a failed import request abandoned makes it inactive.
- **`azure-devops-pp-cli git-apis items-get-items-batch`** - Retrieves a batch of items in a repo / project for a given list of paths or a long path
- **`azure-devops-pp-cli git-apis items-list`** - Get Item Metadata and/or Content for a collection of items. The download parameter is to indicate whether the content should be available as a download or just sent as a stream in the response. Doesn't apply to zipped content which is always returned as a download.
- **`azure-devops-pp-cli git-apis merge-bases-list`** - Find the merge bases of two commits, optionally across forks. If otherRepositoryId is not specified, the merge bases will only be calculated within the context of the local repositoryNameOrId.
- **`azure-devops-pp-cli git-apis merges-create`** - Request a git merge operation. Currently we support merging only 2 commits.
- **`azure-devops-pp-cli git-apis merges-get`** - Get a specific merge operation's details.
- **`azure-devops-pp-cli git-apis policy-configurations-get`** - Retrieve a list of policy configurations by a given set of scope/filtering criteria.

Below is a short description of how all of the query parameters interact with each other:
- repositoryId set, refName set: returns all policy configurations that *apply* to a particular branch in a repository
- repositoryId set, refName unset: returns all policy configurations that *apply* to a particular repository
- repositoryId unset, refName unset: returns all policy configurations that are *defined* at the project level
- repositoryId unset, refName set: returns all project-level branch policies, plus the project level configurations
For all of the examples above, when policyType is set, it'll restrict results to the given policy type
- **`azure-devops-pp-cli git-apis pull-request-attachments-create`** - Attach a new file to a pull request.
- **`azure-devops-pp-cli git-apis pull-request-attachments-delete`** - Delete a pull request attachment.
- **`azure-devops-pp-cli git-apis pull-request-attachments-get`** - Get the file content of a pull request attachment.
- **`azure-devops-pp-cli git-apis pull-request-attachments-list`** - Get a list of files attached to a given pull request.
- **`azure-devops-pp-cli git-apis pull-request-comment-likes-create`** - Add a like on a comment.
- **`azure-devops-pp-cli git-apis pull-request-comment-likes-delete`** - Delete a like on a comment.
- **`azure-devops-pp-cli git-apis pull-request-comment-likes-list`** - Get likes for a comment.
- **`azure-devops-pp-cli git-apis pull-request-commits-get-pull-request-commits`** - Get the commits for the specified pull request.
- **`azure-devops-pp-cli git-apis pull-request-commits-get-pull-request-iteration-commits`** - Get the commits for the specified iteration of a pull request.
- **`azure-devops-pp-cli git-apis pull-request-iteration-changes-get`** - Retrieve the changes made in a pull request between two iterations.
- **`azure-devops-pp-cli git-apis pull-request-iteration-statuses-create`** - Create a pull request status on the iteration. This operation will have the same result as Create status on pull request with specified iteration ID in the request body.

The only required field for the status is `Context.Name` that uniquely identifies the status.
Note that `iterationId` in the request body is optional since `iterationId` can be specified in the URL.
A conflict between `iterationId` in the URL and `iterationId` in the request body will result in status code 400.
- **`azure-devops-pp-cli git-apis pull-request-iteration-statuses-delete`** - Delete pull request iteration status.

You can remove multiple statuses in one call by using Update operation.
- **`azure-devops-pp-cli git-apis pull-request-iteration-statuses-get`** - Get the specific pull request iteration status by ID. The status ID is unique within the pull request across all iterations.
- **`azure-devops-pp-cli git-apis pull-request-iteration-statuses-list`** - Get all the statuses associated with a pull request iteration.
- **`azure-devops-pp-cli git-apis pull-request-iteration-statuses-update`** - Update pull request iteration statuses collection. The only supported operation type is `remove`.

This operation allows to delete multiple statuses in one call.
The path of the `remove` operation should refer to the ID of the pull request status.
For example `path="/1"` refers to the pull request status with ID 1.
- **`azure-devops-pp-cli git-apis pull-request-iterations-get`** - Get the specified iteration for a pull request.
- **`azure-devops-pp-cli git-apis pull-request-iterations-list`** - Get the list of iterations for the specified pull request.
- **`azure-devops-pp-cli git-apis pull-request-labels-create`** - Create a tag (if that does not exists yet) and add that as a label (tag) for a specified pull request. The only required field is the name of the new label (tag).
- **`azure-devops-pp-cli git-apis pull-request-labels-delete`** - Removes a label (tag) from the set of those assigned to the pull request. The tag itself will not be deleted.
- **`azure-devops-pp-cli git-apis pull-request-labels-get`** - Retrieves a single label (tag) that has been assigned to a pull request.
- **`azure-devops-pp-cli git-apis pull-request-labels-list`** - Get all the labels (tags) assigned to a pull request.
- **`azure-devops-pp-cli git-apis pull-request-properties-list`** - Get external properties of the pull request.
- **`azure-devops-pp-cli git-apis pull-request-properties-update`** - Create or update pull request external properties. The patch operation can be `add`, `replace` or `remove`. For `add` operation, the path can be empty. If the path is empty, the value must be a list of key value pairs. For `replace` operation, the path cannot be empty. If the path does not exist, the property will be added to the collection. For `remove` operation, the path cannot be empty. If the path does not exist, no action will be performed.
- **`azure-devops-pp-cli git-apis pull-request-query-get`** - This API is used to find what pull requests are related to a given commit.  It can be used to either find the pull request that created a particular merge commit or it can be used to find all pull requests that have ever merged a particular commit.  The input is a list of queries which each contain a list of commits. For each commit that you search against, you will get back a dictionary of commit -> pull requests.
- **`azure-devops-pp-cli git-apis pull-request-reviewers-create-pull-request-reviewer`** - Add a reviewer to a pull request or cast a vote.
- **`azure-devops-pp-cli git-apis pull-request-reviewers-create-pull-request-reviewers`** - Add reviewers to a pull request.
- **`azure-devops-pp-cli git-apis pull-request-reviewers-create-unmaterialized-pull-request-reviewer`** - Add an unmaterialized identity to the reviewers of a pull request.
- **`azure-devops-pp-cli git-apis pull-request-reviewers-delete`** - Remove a reviewer from a pull request.
- **`azure-devops-pp-cli git-apis pull-request-reviewers-get`** - Retrieve information about a particular reviewer on a pull request
- **`azure-devops-pp-cli git-apis pull-request-reviewers-list`** - Retrieve the reviewers for a pull request
- **`azure-devops-pp-cli git-apis pull-request-reviewers-update-pull-request-reviewer`** - Edit a reviewer entry. These fields are patchable: isFlagged, hasDeclined
- **`azure-devops-pp-cli git-apis pull-request-reviewers-update-pull-request-reviewers`** - Reset the votes of multiple reviewers on a pull request.  NOTE: This endpoint only supports updating votes, but does not support updating required reviewers (use policy) or display names.
- **`azure-devops-pp-cli git-apis pull-request-share-share-pull-request`** - Sends an e-mail notification about a specific pull request to a set of recipients
- **`azure-devops-pp-cli git-apis pull-request-statuses-create`** - Create a pull request status.

The only required field for the status is `Context.Name` that uniquely identifies the status.
Note that you can specify iterationId in the request body to post the status on the iteration.
- **`azure-devops-pp-cli git-apis pull-request-statuses-delete`** - Delete pull request status.

You can remove multiple statuses in one call by using Update operation.
- **`azure-devops-pp-cli git-apis pull-request-statuses-get`** - Get the specific pull request status by ID. The status ID is unique within the pull request across all iterations.
- **`azure-devops-pp-cli git-apis pull-request-statuses-list`** - Get all the statuses associated with a pull request.
- **`azure-devops-pp-cli git-apis pull-request-statuses-update`** - Update pull request statuses collection. The only supported operation type is `remove`.

This operation allows to delete multiple statuses in one call.
The path of the `remove` operation should refer to the ID of the pull request status.
For example `path="/1"` refers to the pull request status with ID 1.
- **`azure-devops-pp-cli git-apis pull-request-thread-comments-create`** - Create a comment on a specific thread in a pull request (up to 500 comments can be created per thread).
- **`azure-devops-pp-cli git-apis pull-request-thread-comments-delete`** - Delete a comment associated with a specific thread in a pull request.
- **`azure-devops-pp-cli git-apis pull-request-thread-comments-get`** - Retrieve a comment associated with a specific thread in a pull request.
- **`azure-devops-pp-cli git-apis pull-request-thread-comments-list`** - Retrieve all comments associated with a specific thread in a pull request.
- **`azure-devops-pp-cli git-apis pull-request-thread-comments-update`** - Update a comment associated with a specific thread in a pull request.
- **`azure-devops-pp-cli git-apis pull-request-threads-create`** - Create a thread in a pull request.
- **`azure-devops-pp-cli git-apis pull-request-threads-get`** - Retrieve a thread in a pull request.
- **`azure-devops-pp-cli git-apis pull-request-threads-list`** - Retrieve all threads in a pull request.
- **`azure-devops-pp-cli git-apis pull-request-threads-update`** - Update a thread in a pull request.
- **`azure-devops-pp-cli git-apis pull-request-work-items-list`** - Retrieve a list of work items associated with a pull request.
- **`azure-devops-pp-cli git-apis pull-requests-create`** - Create a pull request.
- **`azure-devops-pp-cli git-apis pull-requests-get-pull-request`** - Retrieve a pull request.
- **`azure-devops-pp-cli git-apis pull-requests-get-pull-request-by-id`** - Retrieve a pull request.
- **`azure-devops-pp-cli git-apis pull-requests-get-pull-requests`** - Retrieve all pull requests matching a specified criteria.

Please note that description field will be truncated up to 400 symbols in the result.
- **`azure-devops-pp-cli git-apis pull-requests-get-pull-requests-by-project`** - Retrieve all pull requests matching a specified criteria.

Please note that description field will be truncated up to 400 symbols in the result.
- **`azure-devops-pp-cli git-apis pull-requests-update`** - Update a pull request

These are the properties that can be updated with the API:
 - Status
 - Title
 - Description (up to 4000 characters)
 - CompletionOptions
 - MergeOptions
 - AutoCompleteSetBy.Id
 - TargetRefName (when the PR retargeting feature is enabled)
 Attempting to update other properties outside of this list will either cause the server to throw an `InvalidArgumentValueException`,
 or to silently ignore the update.
- **`azure-devops-pp-cli git-apis pushes-create`** - Push changes to the repository.
- **`azure-devops-pp-cli git-apis pushes-get`** - Retrieves a particular push.
- **`azure-devops-pp-cli git-apis pushes-list`** - Retrieves pushes associated with the specified repository.
- **`azure-devops-pp-cli git-apis refs-favorites-create`** - Creates a ref favorite
- **`azure-devops-pp-cli git-apis refs-favorites-delete`** - Deletes the refs favorite specified
- **`azure-devops-pp-cli git-apis refs-favorites-get`** - Gets the refs favorite for a favorite Id.
- **`azure-devops-pp-cli git-apis refs-favorites-list`** - Gets the refs favorites for a repo and an identity.
- **`azure-devops-pp-cli git-apis refs-list`** - Queries the provided repository for its refs and returns them.
- **`azure-devops-pp-cli git-apis refs-update-ref`** - Lock or Unlock a branch.
- **`azure-devops-pp-cli git-apis refs-update-refs`** - Creating, updating, or deleting refs(branches).

Updating a ref means making it point at a different commit than it used to. You must specify both the old and new commit to avoid race conditions.
- **`azure-devops-pp-cli git-apis repositories-create`** - Create a git repository in a team project.
- **`azure-devops-pp-cli git-apis repositories-delete`** - Delete a git repository
- **`azure-devops-pp-cli git-apis repositories-delete-repository-from-recycle-bin`** - Destroy (hard delete) a soft-deleted Git repository.
- **`azure-devops-pp-cli git-apis repositories-get-deleted-repositories`** - Retrieve deleted git repositories.
- **`azure-devops-pp-cli git-apis repositories-get-recycle-bin-repositories`** - Retrieve soft-deleted git repositories from the recycle bin.
- **`azure-devops-pp-cli git-apis repositories-get-repository`** - Retrieve a git repository.
- **`azure-devops-pp-cli git-apis repositories-list`** - Retrieve git repositories.
- **`azure-devops-pp-cli git-apis repositories-restore-repository-from-recycle-bin`** - Recover a soft-deleted Git repository. Recently deleted repositories go into a soft-delete state for a period of time before they are hard deleted and become unrecoverable.
- **`azure-devops-pp-cli git-apis repositories-update`** - Updates the Git repository with either a new repo name or a new default branch.
- **`azure-devops-pp-cli git-apis reverts-create`** - Starts the operation to create a new branch which reverts changes introduced by either a specific commit or commits that are associated to a pull request.
- **`azure-devops-pp-cli git-apis reverts-get-revert`** - Retrieve information about a revert operation by revert Id.
- **`azure-devops-pp-cli git-apis reverts-get-revert-for-ref-name`** - Retrieve information about a revert operation for a specific branch.
- **`azure-devops-pp-cli git-apis stats-list`** - Retrieve statistics about all branches within a repository.
- **`azure-devops-pp-cli git-apis statuses-create`** - Create Git commit status.
- **`azure-devops-pp-cli git-apis statuses-list`** - Get statuses associated with the Git commit.
- **`azure-devops-pp-cli git-apis suggestions-list`** - Retrieve a pull request suggestion for a particular repository or team project.
- **`azure-devops-pp-cli git-apis trees-get`** - The Tree endpoint returns the collection of objects underneath the specified tree. Trees are folders in a Git repository.

Repositories have both a name and an identifier. Identifiers are globally unique, but several projects may contain a repository of the same name. You don't need to include the project if you specify a repository by ID. However, if you specify a repository by name, you must also specify the project (by name or ID.

### pipelines-apis

Manage pipelines apis

- **`azure-devops-pp-cli pipelines-apis artifacts-get`** - Get a specific artifact from a pipeline run
- **`azure-devops-pp-cli pipelines-apis logs-get`** - Get a specific log from a pipeline run
- **`azure-devops-pp-cli pipelines-apis logs-list`** - Get a list of logs from a pipeline run.
- **`azure-devops-pp-cli pipelines-apis pipelines-create`** - Create a pipeline.
- **`azure-devops-pp-cli pipelines-apis pipelines-get`** - Gets a pipeline, optionally at the specified version
- **`azure-devops-pp-cli pipelines-apis pipelines-list`** - Get a list of pipelines.
- **`azure-devops-pp-cli pipelines-apis preview-preview`** - Queues a dry run of the pipeline and returns an object containing the final yaml.
- **`azure-devops-pp-cli pipelines-apis runs-get`** - Gets a run for a particular pipeline.
- **`azure-devops-pp-cli pipelines-apis runs-list`** - Gets top 10000 runs for a particular pipeline.
- **`azure-devops-pp-cli pipelines-apis runs-run-pipeline`** - Runs a pipeline.

### release-apis

Manage release apis

- **`azure-devops-pp-cli release-apis approvals-list`** - Get a list of approvals
- **`azure-devops-pp-cli release-apis approvals-update`** - Update status of an approval
- **`azure-devops-pp-cli release-apis attachments-get-release-task-attachment-content`** - Get a release task attachment.
- **`azure-devops-pp-cli release-apis attachments-get-release-task-attachments`** - Get the release task attachments.
- **`azure-devops-pp-cli release-apis attachments-get-task-attachment-content`** - GetTaskAttachmentContent API is deprecated. Use GetReleaseTaskAttachmentContent API instead.
- **`azure-devops-pp-cli release-apis attachments-get-task-attachments`** - GetTaskAttachments API is deprecated. Use GetReleaseTaskAttachments API instead.
- **`azure-devops-pp-cli release-apis definitions-create`** - Create a release definition
- **`azure-devops-pp-cli release-apis definitions-delete`** - Delete a release definition.
- **`azure-devops-pp-cli release-apis definitions-get`** - Get a release definition.
- **`azure-devops-pp-cli release-apis definitions-get-definition-revision`** - Get release definition for a given definitionId and revision
- **`azure-devops-pp-cli release-apis definitions-get-release-definition-history`** - Get revision history for a release definition
- **`azure-devops-pp-cli release-apis definitions-list`** - Get a list of release definitions.
- **`azure-devops-pp-cli release-apis definitions-update`** - Update a release definition.
- **`azure-devops-pp-cli release-apis deployments-list`** - Deployments list
- **`azure-devops-pp-cli release-apis folders-create`** - This method is no longer supported. Use CreateFolder with folder parameter API.
- **`azure-devops-pp-cli release-apis folders-delete`** - Deletes a definition folder for given folder name and path and all it's existing definitions.
- **`azure-devops-pp-cli release-apis folders-list`** - Gets folders.
- **`azure-devops-pp-cli release-apis folders-update`** - Updates an existing folder at given existing path.
- **`azure-devops-pp-cli release-apis gates-update`** - Updates the gate for a deployment.
- **`azure-devops-pp-cli release-apis manual-interventions-get`** - Get manual intervention for a given release and manual intervention id.
- **`azure-devops-pp-cli release-apis manual-interventions-list`** - List all manual interventions for a given release.
- **`azure-devops-pp-cli release-apis manual-interventions-update`** - Update manual intervention.
- **`azure-devops-pp-cli release-apis releases-create`** - Create a release.
- **`azure-devops-pp-cli release-apis releases-get-logs`** - Get logs for a release Id.
- **`azure-devops-pp-cli release-apis releases-get-release-environment`** - Get a release environment.
- **`azure-devops-pp-cli release-apis releases-get-release-revision`** - Get release for a given revision number.
- **`azure-devops-pp-cli release-apis releases-get-task-log`** - Gets the task log of a release as a plain text file.
- **`azure-devops-pp-cli release-apis releases-list`** - Get a list of releases
- **`azure-devops-pp-cli release-apis releases-update-release`** - Update a complete release object.
- **`azure-devops-pp-cli release-apis releases-update-release-environment`** - Update the status of a release environment
- **`azure-devops-pp-cli release-apis releases-update-release-resource`** - Update few properties of a release.

### search-apis

Manage search apis

- **`azure-devops-pp-cli search-apis <organization>`** - Provides a set of results for the search text.

### wiki-apis

Manage wiki apis

- **`azure-devops-pp-cli wiki-apis attachments-create`** - Creates an attachment in the wiki.
- **`azure-devops-pp-cli wiki-apis page-moves-create`** - Creates a page move operation that updates the path and order of the page as provided in the parameters.
- **`azure-devops-pp-cli wiki-apis page-stats-get`** - Returns page detail corresponding to Page ID.
- **`azure-devops-pp-cli wiki-apis pages-batch-get`** - Returns pageable list of Wiki Pages
- **`azure-devops-pp-cli wiki-apis pages-create-or-update`** - Creates or edits a wiki page.
- **`azure-devops-pp-cli wiki-apis pages-delete-page`** - Deletes a wiki page.
- **`azure-devops-pp-cli wiki-apis pages-delete-page-by-id`** - Deletes a wiki page.
- **`azure-devops-pp-cli wiki-apis pages-get-page`** - Gets metadata or content of the wiki page for the provided path. Content negotiation is done based on the `Accept` header sent in the request.
- **`azure-devops-pp-cli wiki-apis pages-get-page-by-id`** - Gets metadata or content of the wiki page for the provided page id. Content negotiation is done based on the `Accept` header sent in the request.
- **`azure-devops-pp-cli wiki-apis pages-update`** - Edits a wiki page.
- **`azure-devops-pp-cli wiki-apis wikis-create`** - Creates the wiki resource.
- **`azure-devops-pp-cli wiki-apis wikis-delete`** - Deletes the wiki corresponding to the wiki ID or wiki name provided.
- **`azure-devops-pp-cli wiki-apis wikis-get`** - Gets the wiki corresponding to the wiki ID or wiki name provided.
- **`azure-devops-pp-cli wiki-apis wikis-list`** - Gets all wikis in a project or collection.
- **`azure-devops-pp-cli wiki-apis wikis-update`** - Updates the wiki corresponding to the wiki ID or wiki name provided using the update parameters.

### work-apis

Manage work apis

- **`azure-devops-pp-cli work-apis boardcolumns-list`** - Get available board columns in a project
- **`azure-devops-pp-cli work-apis boardrows-list`** - Get available board rows in a project
- **`azure-devops-pp-cli work-apis deliverytimeline-get`** - Get Delivery View Data
- **`azure-devops-pp-cli work-apis iterationcapacities-get`** - Get an iteration's capacity for all teams in iteration
- **`azure-devops-pp-cli work-apis plans-create`** - Add a new plan for the team
- **`azure-devops-pp-cli work-apis plans-delete`** - Delete the specified plan
- **`azure-devops-pp-cli work-apis plans-get`** - Get the information for the specified plan
- **`azure-devops-pp-cli work-apis plans-list`** - Get the information for all the plans configured for the given team
- **`azure-devops-pp-cli work-apis plans-update`** - Update the information for the specified plan
- **`azure-devops-pp-cli work-apis processconfiguration-get`** - Get process configuration

### workitemtracking-apis

Manage workitemtracking apis

- **`azure-devops-pp-cli workitemtracking-apis account-my-work-recent-activity-list`** - Gets recent work item activities
- **`azure-devops-pp-cli workitemtracking-apis artifact-link-types-list`** - Get the list of work item tracking outbound artifact link types.
- **`azure-devops-pp-cli workitemtracking-apis work-item-icons-get`** - Get a work item icon given the friendly name and icon color.
- **`azure-devops-pp-cli workitemtracking-apis work-item-icons-list`** - Get a list of all work item icons.
- **`azure-devops-pp-cli workitemtracking-apis work-item-relation-types-get`** - Gets the work item relation type definition.
- **`azure-devops-pp-cli workitemtracking-apis work-item-relation-types-list`** - Gets the work item relation types.
- **`azure-devops-pp-cli workitemtracking-apis work-item-transitions-list`** - Returns the next state on the given work item IDs.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
azure-devops-pp-cli apis avatar-remove-project-avatar mock-value mock-value --api-version example-value

# JSON for scripting and agents
azure-devops-pp-cli apis avatar-remove-project-avatar mock-value mock-value --api-version example-value --json

# Filter to specific fields
azure-devops-pp-cli apis avatar-remove-project-avatar mock-value mock-value --api-version example-value --json --select id,name,status

# Dry run — show the request without sending
azure-devops-pp-cli apis avatar-remove-project-avatar mock-value mock-value --api-version example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
azure-devops-pp-cli apis avatar-remove-project-avatar mock-value mock-value --api-version example-value --agent
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
azure-devops-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/azure-devops-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `CORE_USERNAME` | per_call | Yes |  |
| `CORE_PASSWORD` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `azure-devops-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `azure-devops-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $CORE_USERNAME`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Unauthorized on every request** — Check AZURE_DEVOPS_TOKEN is set and contains a valid PAT. Regenerate at dev.azure.com/{org}/_usersSettings/tokens.
- **doctor reports 'project not found'** — Set AZURE_DEVOPS_PROJECT to the exact project name (case-sensitive). Run `azure-devops-pp-cli projects list` to see available projects.
- **sync returns 429 Too Many Requests** — Azure DevOps rate-limits at 200 TSTUs per 5 minutes. Add --max-pages 5 to limit sync volume, or wait 5 minutes and retry.
- **standup returns empty results after sync** — Run `azure-devops-pp-cli standup --dry-run` to verify the command resolves, then run without --dry-run once AZURE_DEVOPS_TOKEN, AZURE_DEVOPS_ORG, and AZURE_DEVOPS_PROJECT are set.
- **auth set-token shows 'credential saved' but doctor still shows 'not configured'** — Restart the terminal to clear any old AZURE_DEVOPS_TOKEN env var that overrides the saved credential. Run `azure-devops-pp-cli auth status` to see which source is active.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**azure-devops-mcp**](https://github.com/microsoft/azure-devops-mcp) — TypeScript (1800 stars)
- [**mcp-server-azure-devops**](https://github.com/Tiberriver256/mcp-server-azure-devops) — TypeScript (374 stars)
- [**azure-devops-go-api**](https://github.com/microsoft/azure-devops-go-api) — Go (224 stars)
- [**mcp-for-azure-devops-boards**](https://github.com/danielealbano/mcp-for-azure-devops-boards) — Rust (6 stars)
- [**azure-devops-cli-extension**](https://github.com/Azure/azure-devops-cli-extension) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
