---
name: pp-azure-devops
description: "The fastest Azure DevOps CLI — offline-first, agent-native Trigger phrases: `show my Azure DevOps work items`, `check my ADO pull requests`, `what's the sprint velocity`, `which PRs need my review`, `check Azure DevOps pipeline status`, `use azure-devops`, `run azure-devops`."
author: "davbebawwy"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - azure-devops-pp-cli
    install:
      - kind: go
        bins: [azure-devops-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/developer-tools/azure-devops/cmd/azure-devops-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/developer-tools/azure-devops/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Azure DevOps — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `azure-devops-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install azure-devops --cli-only
   ```
2. Verify: `azure-devops-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/azure-devops/cmd/azure-devops-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

azure-devops-pp-cli syncs your work items, builds, and PRs to a local SQLite mirror and lets you query them with SQL or structured --agent output. It matches everything `az devops` and the Microsoft MCP server can do, then adds fifteen cross-entity analytics commands that require no live API calls — sprint velocity, PR review queues, scope creep detection, cycle time analysis, and more.

## When to Use This CLI

Use azure-devops-pp-cli when an agent needs to read or write Azure DevOps data: work items, pull requests, pipelines, builds, or sprint state. It is the fastest path to structured JSON output for ADO data without a browser or the Python-based az devops extension. Use it especially for cross-entity analytics (velocity, cycle time, scope creep) that require the local SQLite mirror. Prefer `az devops` only when you need features this CLI does not yet cover (migrations, live entitlement management).

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI for Azure resource management (VMs, storage, AKS) — use the `az` CLI instead.
- Do not use this CLI for GitHub repositories — use the `gh` CLI.
- Do not use this CLI for reading raw file content from Azure Blob Storage.
- Do not use this CLI for Azure Boards queries that require real-time board state not yet synced.

## Unique Capabilities

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

## Command Reference

**apis** — Manage apis

- `azure-devops-pp-cli apis avatar-remove-project-avatar` — Removes the avatar for the project.
- `azure-devops-pp-cli apis avatar-set-project-avatar` — Sets the avatar for the project.
- `azure-devops-pp-cli apis categorized-teams-get` — Gets list of user readable teams in a project and teams user is member of (excluded from readable list).
- `azure-devops-pp-cli apis processes-get` — Get a process by ID.
- `azure-devops-pp-cli apis processes-list` — Get a list of processes.
- `azure-devops-pp-cli apis projects-create` — Queues a project to be created. Use the [GetOperation](../..
- `azure-devops-pp-cli apis projects-delete` — Queues a project to be deleted. Use the [GetOperation](../..
- `azure-devops-pp-cli apis projects-get` — Get project with the specified id or name, optionally including capabilities.
- `azure-devops-pp-cli apis projects-get-project-properties` — Get a collection of team project properties.
- `azure-devops-pp-cli apis projects-list` — Get all projects in the organization that the authenticated user has access to.
- `azure-devops-pp-cli apis projects-set-project-properties` — Create, update, and delete team project properties.
- `azure-devops-pp-cli apis projects-update` — Update an existing project's name, abbreviation, description, or restore a project.
- `azure-devops-pp-cli apis teams-create` — Create a team in a team project.
- `azure-devops-pp-cli apis teams-delete` — Delete a team.
- `azure-devops-pp-cli apis teams-get` — Get a specific team.
- `azure-devops-pp-cli apis teams-get-all-teams` — Get a list of all teams.
- `azure-devops-pp-cli apis teams-get-team-members-with-extended-properties` — Get a list of members for a specific team.
- `azure-devops-pp-cli apis teams-get-teams` — Get a list of teams.
- `azure-devops-pp-cli apis teams-update` — Update a team's name and/or description.

**build-apis** — Manage build apis

- `azure-devops-pp-cli build-apis artifacts-create` — Associates an artifact with a build.
- `azure-devops-pp-cli build-apis artifacts-list` — Gets all artifacts for a build.
- `azure-devops-pp-cli build-apis attachments-get` — Gets a specific attachment.
- `azure-devops-pp-cli build-apis attachments-list` — Gets the list of attachments of a specific type that are associated with a build.
- `azure-devops-pp-cli build-apis authorizedresources-authorize-project-resources` — Authorizedresources authorize project resources
- `azure-devops-pp-cli build-apis authorizedresources-list` — Authorizedresources list
- `azure-devops-pp-cli build-apis badge-get-build-badge-data` — Gets a badge that indicates the status of the most recent build for the specified branch.
- `azure-devops-pp-cli build-apis builds-delete` — Deletes a build.
- `azure-devops-pp-cli build-apis builds-get` — Gets a build
- `azure-devops-pp-cli build-apis builds-get-build-changes` — Gets the changes associated with a build
- `azure-devops-pp-cli build-apis builds-get-build-log` — Gets an individual log file for a build.
- `azure-devops-pp-cli build-apis builds-get-build-logs` — Gets the logs for a build.
- `azure-devops-pp-cli build-apis builds-get-build-work-items-refs` — Gets the work items associated with a build. Only work items in the same project are returned.
- `azure-devops-pp-cli build-apis builds-get-build-work-items-refs-from-commits` — Gets the work items associated with a build, filtered to specific commits.
- `azure-devops-pp-cli build-apis builds-get-changes-between-builds` — Gets the changes made to the repository between two given builds.
- `azure-devops-pp-cli build-apis builds-get-retention-leases-for-build` — Gets all retention leases that apply to a specific build.
- `azure-devops-pp-cli build-apis builds-get-work-items-between-builds` — Gets all the work items between two builds.
- `azure-devops-pp-cli build-apis builds-list` — Gets a list of builds.
- `azure-devops-pp-cli build-apis builds-queue` — Queues a build
- `azure-devops-pp-cli build-apis builds-update-build` — Updates a build.
- `azure-devops-pp-cli build-apis builds-update-builds` — Updates multiple builds.
- `azure-devops-pp-cli build-apis definitions-create` — Creates a new definition.
- `azure-devops-pp-cli build-apis definitions-delete` — Deletes a definition and all associated builds.
- `azure-devops-pp-cli build-apis definitions-get` — Gets a definition, optionally at a specific revision.
- `azure-devops-pp-cli build-apis definitions-get-definition-revisions` — Gets all revisions of a definition.
- `azure-devops-pp-cli build-apis definitions-list` — Gets a list of definitions.
- `azure-devops-pp-cli build-apis definitions-restore-definition` — Restores a deleted definition
- `azure-devops-pp-cli build-apis definitions-update` — Updates an existing build definition.
- `azure-devops-pp-cli build-apis folders-create` — Creates a new folder.
- `azure-devops-pp-cli build-apis folders-delete` — Deletes a definition folder. Definitions and their corresponding builds will also be deleted.
- `azure-devops-pp-cli build-apis folders-list` — Gets a list of build definition folders.
- `azure-devops-pp-cli build-apis folders-update` — Updates an existing folder at given existing path
- `azure-devops-pp-cli build-apis general-settings-get` — Gets pipeline general settings.
- `azure-devops-pp-cli build-apis general-settings-update` — Updates pipeline general settings.
- `azure-devops-pp-cli build-apis latest-get` — Gets the latest build for a definition, optionally scoped to a specific branch.
- `azure-devops-pp-cli build-apis leases-add` — Adds new leases for pipeline runs.
- `azure-devops-pp-cli build-apis leases-delete` — Removes specific retention leases.
- `azure-devops-pp-cli build-apis leases-get` — Returns the details of the retention lease given a lease id.
- `azure-devops-pp-cli build-apis leases-get-retention-leases-by-minimal-retention-leases` — Returns any leases matching the specified MinimalRetentionLeases
- `azure-devops-pp-cli build-apis leases-update` — Updates the duration or pipeline protection status of a retention lease.
- `azure-devops-pp-cli build-apis metrics-get-definition-metrics` — Gets build metrics for a definition.
- `azure-devops-pp-cli build-apis metrics-get-project-metrics` — Gets build metrics for a project.
- `azure-devops-pp-cli build-apis options-list` — Gets all build definition options supported by the system.
- `azure-devops-pp-cli build-apis properties-get-build-properties` — Gets properties for a build.
- `azure-devops-pp-cli build-apis properties-get-definition-properties` — Gets properties for a definition.
- `azure-devops-pp-cli build-apis properties-update-build-properties` — Updates properties for a build.
- `azure-devops-pp-cli build-apis properties-update-definition-properties` — Updates properties for a definition.
- `azure-devops-pp-cli build-apis report-get` — Gets a build report.
- `azure-devops-pp-cli build-apis resources-authorize-definition-resources` — Resources authorize definition resources
- `azure-devops-pp-cli build-apis resources-list` — Resources list
- `azure-devops-pp-cli build-apis retention-get` — Gets the project's retention settings.
- `azure-devops-pp-cli build-apis retention-update` — Updates the project's retention settings.
- `azure-devops-pp-cli build-apis settings-get` — Gets the build settings.
- `azure-devops-pp-cli build-apis settings-update` — Updates the build settings.
- `azure-devops-pp-cli build-apis source-providers-get-file-contents` — Gets the contents of a file in the given source code repository.
- `azure-devops-pp-cli build-apis source-providers-get-path-contents` — Gets the contents of a directory in the given source code repository.
- `azure-devops-pp-cli build-apis source-providers-get-pull-request` — Gets a pull request object from source provider.
- `azure-devops-pp-cli build-apis source-providers-list` — Get a list of source providers and their capabilities.
- `azure-devops-pp-cli build-apis source-providers-list-branches` — Gets a list of branches for the given source code repository.
- `azure-devops-pp-cli build-apis source-providers-list-repositories` — Gets a list of source code repositories.
- `azure-devops-pp-cli build-apis source-providers-list-webhooks` — Gets a list of webhooks installed in the given source code repository.
- `azure-devops-pp-cli build-apis source-providers-restore-webhooks` — Recreates the webhooks for the specified triggers in the given source code repository.
- `azure-devops-pp-cli build-apis stages-update` — Update a build stage
- `azure-devops-pp-cli build-apis status-get` — Gets the build status for a definition, optionally scoped to a specific branch, stage, job, and configuration.
- `azure-devops-pp-cli build-apis tags-add-build-tag` — Adds a tag to a build.
- `azure-devops-pp-cli build-apis tags-add-build-tags` — Adds tags to a build.
- `azure-devops-pp-cli build-apis tags-add-definition-tag` — Adds a tag to a definition
- `azure-devops-pp-cli build-apis tags-add-definition-tags` — Adds multiple tags to a definition.
- `azure-devops-pp-cli build-apis tags-delete-build-tag` — Removes a tag from a build. NOTE: This API will not work for tags with special characters.
- `azure-devops-pp-cli build-apis tags-delete-definition-tag` — Removes a tag from a definition. NOTE: This API will not work for tags with special characters.
- `azure-devops-pp-cli build-apis tags-delete-tag` — Removes a tag from builds, definitions, and from the tag store
- `azure-devops-pp-cli build-apis tags-get-build-tags` — Gets the tags for a build.
- `azure-devops-pp-cli build-apis tags-get-definition-tags` — Gets the tags for a definition.
- `azure-devops-pp-cli build-apis tags-get-tags` — Gets a list of all build tags in the project.
- `azure-devops-pp-cli build-apis tags-update-build-tags` — Adds/Removes tags from a build.
- `azure-devops-pp-cli build-apis tags-update-definition-tags` — Adds/Removes tags from a definition.
- `azure-devops-pp-cli build-apis templates-delete` — Deletes a build definition template.
- `azure-devops-pp-cli build-apis templates-get` — Gets a specific build definition template.
- `azure-devops-pp-cli build-apis templates-list` — Gets all definition templates.
- `azure-devops-pp-cli build-apis templates-save-template` — Updates an existing build definition template.
- `azure-devops-pp-cli build-apis timeline-get` — Gets details for a build
- `azure-devops-pp-cli build-apis yaml-get` — Converts a definition to YAML, optionally at a specific revision.

**git-apis** — Manage git apis

- `azure-devops-pp-cli git-apis annotated-tags-create` — Create an annotated tag. Repositories have both a name and an identifier.
- `azure-devops-pp-cli git-apis annotated-tags-get` — Get an annotated tag. Repositories have both a name and an identifier.
- `azure-devops-pp-cli git-apis blobs-get-blob` — Get a single blob. Repositories have both a name and an identifier.
- `azure-devops-pp-cli git-apis blobs-get-blobs-zip` — Gets one or more blobs in a zip file download.
- `azure-devops-pp-cli git-apis cherry-picks-create` — Cherry pick a specific commit or commits that are associated to a pull request into a new branch.
- `azure-devops-pp-cli git-apis cherry-picks-get-cherry-pick` — Retrieve information about a cherry pick operation by cherry pick Id.
- `azure-devops-pp-cli git-apis cherry-picks-get-cherry-pick-for-ref-name` — Retrieve information about a cherry pick operation for a specific branch.
- `azure-devops-pp-cli git-apis commits-get` — Retrieve a particular commit.
- `azure-devops-pp-cli git-apis commits-get-changes` — Retrieve changes for a particular commit.
- `azure-devops-pp-cli git-apis commits-get-commits-batch` — Retrieve git commits for a project matching the search criteria
- `azure-devops-pp-cli git-apis commits-get-push-commits` — Retrieve a list of commits associated with a particular push.
- `azure-devops-pp-cli git-apis diffs-get` — Find the closest common commit (the merge base) between base and target commits
- `azure-devops-pp-cli git-apis forks-create-fork-sync-request` — Request that another repository's refs be fetched into this one. It syncs two existing forks.
- `azure-devops-pp-cli git-apis forks-get-fork-sync-request` — Get a specific fork sync operation's details.
- `azure-devops-pp-cli git-apis forks-get-fork-sync-requests` — Retrieve all requested fork sync operations on this repository.
- `azure-devops-pp-cli git-apis forks-list` — Retrieve all forks of a repository in the collection.
- `azure-devops-pp-cli git-apis import-requests-create` — Create an import request.
- `azure-devops-pp-cli git-apis import-requests-get` — Retrieve a particular import request.
- `azure-devops-pp-cli git-apis import-requests-query` — Retrieve import requests for a repository.
- `azure-devops-pp-cli git-apis import-requests-update` — Retry or abandon a failed import request. There can only be one active import request associated with a repository.
- `azure-devops-pp-cli git-apis items-get-items-batch` — Retrieves a batch of items in a repo / project for a given list of paths or a long path
- `azure-devops-pp-cli git-apis items-list` — Get Item Metadata and/or Content for a collection of items.
- `azure-devops-pp-cli git-apis merge-bases-list` — Find the merge bases of two commits, optionally across forks.
- `azure-devops-pp-cli git-apis merges-create` — Request a git merge operation. Currently we support merging only 2 commits.
- `azure-devops-pp-cli git-apis merges-get` — Get a specific merge operation's details.
- `azure-devops-pp-cli git-apis policy-configurations-get` — Retrieve a list of policy configurations by a given set of scope/filtering criteria.
- `azure-devops-pp-cli git-apis pull-request-attachments-create` — Attach a new file to a pull request.
- `azure-devops-pp-cli git-apis pull-request-attachments-delete` — Delete a pull request attachment.
- `azure-devops-pp-cli git-apis pull-request-attachments-get` — Get the file content of a pull request attachment.
- `azure-devops-pp-cli git-apis pull-request-attachments-list` — Get a list of files attached to a given pull request.
- `azure-devops-pp-cli git-apis pull-request-comment-likes-create` — Add a like on a comment.
- `azure-devops-pp-cli git-apis pull-request-comment-likes-delete` — Delete a like on a comment.
- `azure-devops-pp-cli git-apis pull-request-comment-likes-list` — Get likes for a comment.
- `azure-devops-pp-cli git-apis pull-request-commits-get-pull-request-commits` — Get the commits for the specified pull request.
- `azure-devops-pp-cli git-apis pull-request-commits-get-pull-request-iteration-commits` — Get the commits for the specified iteration of a pull request.
- `azure-devops-pp-cli git-apis pull-request-iteration-changes-get` — Retrieve the changes made in a pull request between two iterations.
- `azure-devops-pp-cli git-apis pull-request-iteration-statuses-create` — Create a pull request status on the iteration.
- `azure-devops-pp-cli git-apis pull-request-iteration-statuses-delete` — Delete pull request iteration status. You can remove multiple statuses in one call by using Update operation.
- `azure-devops-pp-cli git-apis pull-request-iteration-statuses-get` — Get the specific pull request iteration status by ID.
- `azure-devops-pp-cli git-apis pull-request-iteration-statuses-list` — Get all the statuses associated with a pull request iteration.
- `azure-devops-pp-cli git-apis pull-request-iteration-statuses-update` — Update pull request iteration statuses collection. The only supported operation type is `remove`.
- `azure-devops-pp-cli git-apis pull-request-iterations-get` — Get the specified iteration for a pull request.
- `azure-devops-pp-cli git-apis pull-request-iterations-list` — Get the list of iterations for the specified pull request.
- `azure-devops-pp-cli git-apis pull-request-labels-create` — Create a tag (if that does not exists yet) and add that as a label (tag) for a specified pull request.
- `azure-devops-pp-cli git-apis pull-request-labels-delete` — Removes a label (tag) from the set of those assigned to the pull request. The tag itself will not be deleted.
- `azure-devops-pp-cli git-apis pull-request-labels-get` — Retrieves a single label (tag) that has been assigned to a pull request.
- `azure-devops-pp-cli git-apis pull-request-labels-list` — Get all the labels (tags) assigned to a pull request.
- `azure-devops-pp-cli git-apis pull-request-properties-list` — Get external properties of the pull request.
- `azure-devops-pp-cli git-apis pull-request-properties-update` — Create or update pull request external properties. The patch operation can be `add`, `replace` or `remove`.
- `azure-devops-pp-cli git-apis pull-request-query-get` — This API is used to find what pull requests are related to a given commit.
- `azure-devops-pp-cli git-apis pull-request-reviewers-create-pull-request-reviewer` — Add a reviewer to a pull request or cast a vote.
- `azure-devops-pp-cli git-apis pull-request-reviewers-create-pull-request-reviewers` — Add reviewers to a pull request.
- `azure-devops-pp-cli git-apis pull-request-reviewers-create-unmaterialized-pull-request-reviewer` — Add an unmaterialized identity to the reviewers of a pull request.
- `azure-devops-pp-cli git-apis pull-request-reviewers-delete` — Remove a reviewer from a pull request.
- `azure-devops-pp-cli git-apis pull-request-reviewers-get` — Retrieve information about a particular reviewer on a pull request
- `azure-devops-pp-cli git-apis pull-request-reviewers-list` — Retrieve the reviewers for a pull request
- `azure-devops-pp-cli git-apis pull-request-reviewers-update-pull-request-reviewer` — Edit a reviewer entry. These fields are patchable: isFlagged, hasDeclined
- `azure-devops-pp-cli git-apis pull-request-reviewers-update-pull-request-reviewers` — Reset the votes of multiple reviewers on a pull request.
- `azure-devops-pp-cli git-apis pull-request-share-share-pull-request` — Sends an e-mail notification about a specific pull request to a set of recipients
- `azure-devops-pp-cli git-apis pull-request-statuses-create` — Create a pull request status. The only required field for the status is `Context.
- `azure-devops-pp-cli git-apis pull-request-statuses-delete` — Delete pull request status. You can remove multiple statuses in one call by using Update operation.
- `azure-devops-pp-cli git-apis pull-request-statuses-get` — Get the specific pull request status by ID. The status ID is unique within the pull request across all iterations.
- `azure-devops-pp-cli git-apis pull-request-statuses-list` — Get all the statuses associated with a pull request.
- `azure-devops-pp-cli git-apis pull-request-statuses-update` — Update pull request statuses collection. The only supported operation type is `remove`.
- `azure-devops-pp-cli git-apis pull-request-thread-comments-create` — Create a comment on a specific thread in a pull request (up to 500 comments can be created per thread).
- `azure-devops-pp-cli git-apis pull-request-thread-comments-delete` — Delete a comment associated with a specific thread in a pull request.
- `azure-devops-pp-cli git-apis pull-request-thread-comments-get` — Retrieve a comment associated with a specific thread in a pull request.
- `azure-devops-pp-cli git-apis pull-request-thread-comments-list` — Retrieve all comments associated with a specific thread in a pull request.
- `azure-devops-pp-cli git-apis pull-request-thread-comments-update` — Update a comment associated with a specific thread in a pull request.
- `azure-devops-pp-cli git-apis pull-request-threads-create` — Create a thread in a pull request.
- `azure-devops-pp-cli git-apis pull-request-threads-get` — Retrieve a thread in a pull request.
- `azure-devops-pp-cli git-apis pull-request-threads-list` — Retrieve all threads in a pull request.
- `azure-devops-pp-cli git-apis pull-request-threads-update` — Update a thread in a pull request.
- `azure-devops-pp-cli git-apis pull-request-work-items-list` — Retrieve a list of work items associated with a pull request.
- `azure-devops-pp-cli git-apis pull-requests-create` — Create a pull request.
- `azure-devops-pp-cli git-apis pull-requests-get-pull-request` — Retrieve a pull request.
- `azure-devops-pp-cli git-apis pull-requests-get-pull-request-by-id` — Retrieve a pull request.
- `azure-devops-pp-cli git-apis pull-requests-get-pull-requests` — Retrieve all pull requests matching a specified criteria.
- `azure-devops-pp-cli git-apis pull-requests-get-pull-requests-by-project` — Retrieve all pull requests matching a specified criteria.
- `azure-devops-pp-cli git-apis pull-requests-update` — Update a pull request These are the properties that can be updated with the API
- `azure-devops-pp-cli git-apis pushes-create` — Push changes to the repository.
- `azure-devops-pp-cli git-apis pushes-get` — Retrieves a particular push.
- `azure-devops-pp-cli git-apis pushes-list` — Retrieves pushes associated with the specified repository.
- `azure-devops-pp-cli git-apis refs-favorites-create` — Creates a ref favorite
- `azure-devops-pp-cli git-apis refs-favorites-delete` — Deletes the refs favorite specified
- `azure-devops-pp-cli git-apis refs-favorites-get` — Gets the refs favorite for a favorite Id.
- `azure-devops-pp-cli git-apis refs-favorites-list` — Gets the refs favorites for a repo and an identity.
- `azure-devops-pp-cli git-apis refs-list` — Queries the provided repository for its refs and returns them.
- `azure-devops-pp-cli git-apis refs-update-ref` — Lock or Unlock a branch.
- `azure-devops-pp-cli git-apis refs-update-refs` — Creating, updating, or deleting refs(branches).
- `azure-devops-pp-cli git-apis repositories-create` — Create a git repository in a team project.
- `azure-devops-pp-cli git-apis repositories-delete` — Delete a git repository
- `azure-devops-pp-cli git-apis repositories-delete-repository-from-recycle-bin` — Destroy (hard delete) a soft-deleted Git repository.
- `azure-devops-pp-cli git-apis repositories-get-deleted-repositories` — Retrieve deleted git repositories.
- `azure-devops-pp-cli git-apis repositories-get-recycle-bin-repositories` — Retrieve soft-deleted git repositories from the recycle bin.
- `azure-devops-pp-cli git-apis repositories-get-repository` — Retrieve a git repository.
- `azure-devops-pp-cli git-apis repositories-list` — Retrieve git repositories.
- `azure-devops-pp-cli git-apis repositories-restore-repository-from-recycle-bin` — Recover a soft-deleted Git repository.
- `azure-devops-pp-cli git-apis repositories-update` — Updates the Git repository with either a new repo name or a new default branch.
- `azure-devops-pp-cli git-apis reverts-create` — Starts the operation to create a new branch which reverts changes introduced by either a specific commit or commits
- `azure-devops-pp-cli git-apis reverts-get-revert` — Retrieve information about a revert operation by revert Id.
- `azure-devops-pp-cli git-apis reverts-get-revert-for-ref-name` — Retrieve information about a revert operation for a specific branch.
- `azure-devops-pp-cli git-apis stats-list` — Retrieve statistics about all branches within a repository.
- `azure-devops-pp-cli git-apis statuses-create` — Create Git commit status.
- `azure-devops-pp-cli git-apis statuses-list` — Get statuses associated with the Git commit.
- `azure-devops-pp-cli git-apis suggestions-list` — Retrieve a pull request suggestion for a particular repository or team project.
- `azure-devops-pp-cli git-apis trees-get` — The Tree endpoint returns the collection of objects underneath the specified tree.

**pipelines-apis** — Manage pipelines apis

- `azure-devops-pp-cli pipelines-apis artifacts-get` — Get a specific artifact from a pipeline run
- `azure-devops-pp-cli pipelines-apis logs-get` — Get a specific log from a pipeline run
- `azure-devops-pp-cli pipelines-apis logs-list` — Get a list of logs from a pipeline run.
- `azure-devops-pp-cli pipelines-apis pipelines-create` — Create a pipeline.
- `azure-devops-pp-cli pipelines-apis pipelines-get` — Gets a pipeline, optionally at the specified version
- `azure-devops-pp-cli pipelines-apis pipelines-list` — Get a list of pipelines.
- `azure-devops-pp-cli pipelines-apis preview-preview` — Queues a dry run of the pipeline and returns an object containing the final yaml.
- `azure-devops-pp-cli pipelines-apis runs-get` — Gets a run for a particular pipeline.
- `azure-devops-pp-cli pipelines-apis runs-list` — Gets top 10000 runs for a particular pipeline.
- `azure-devops-pp-cli pipelines-apis runs-run-pipeline` — Runs a pipeline.

**release-apis** — Manage release apis

- `azure-devops-pp-cli release-apis approvals-list` — Get a list of approvals
- `azure-devops-pp-cli release-apis approvals-update` — Update status of an approval
- `azure-devops-pp-cli release-apis attachments-get-release-task-attachment-content` — Get a release task attachment.
- `azure-devops-pp-cli release-apis attachments-get-release-task-attachments` — Get the release task attachments.
- `azure-devops-pp-cli release-apis attachments-get-task-attachment-content` — GetTaskAttachmentContent API is deprecated. Use GetReleaseTaskAttachmentContent API instead.
- `azure-devops-pp-cli release-apis attachments-get-task-attachments` — GetTaskAttachments API is deprecated. Use GetReleaseTaskAttachments API instead.
- `azure-devops-pp-cli release-apis definitions-create` — Create a release definition
- `azure-devops-pp-cli release-apis definitions-delete` — Delete a release definition.
- `azure-devops-pp-cli release-apis definitions-get` — Get a release definition.
- `azure-devops-pp-cli release-apis definitions-get-definition-revision` — Get release definition for a given definitionId and revision
- `azure-devops-pp-cli release-apis definitions-get-release-definition-history` — Get revision history for a release definition
- `azure-devops-pp-cli release-apis definitions-list` — Get a list of release definitions.
- `azure-devops-pp-cli release-apis definitions-update` — Update a release definition.
- `azure-devops-pp-cli release-apis deployments-list` — Deployments list
- `azure-devops-pp-cli release-apis folders-create` — This method is no longer supported. Use CreateFolder with folder parameter API.
- `azure-devops-pp-cli release-apis folders-delete` — Deletes a definition folder for given folder name and path and all it's existing definitions.
- `azure-devops-pp-cli release-apis folders-list` — Gets folders.
- `azure-devops-pp-cli release-apis folders-update` — Updates an existing folder at given existing path.
- `azure-devops-pp-cli release-apis gates-update` — Updates the gate for a deployment.
- `azure-devops-pp-cli release-apis manual-interventions-get` — Get manual intervention for a given release and manual intervention id.
- `azure-devops-pp-cli release-apis manual-interventions-list` — List all manual interventions for a given release.
- `azure-devops-pp-cli release-apis manual-interventions-update` — Update manual intervention.
- `azure-devops-pp-cli release-apis releases-create` — Create a release.
- `azure-devops-pp-cli release-apis releases-get-logs` — Get logs for a release Id.
- `azure-devops-pp-cli release-apis releases-get-release-environment` — Get a release environment.
- `azure-devops-pp-cli release-apis releases-get-release-revision` — Get release for a given revision number.
- `azure-devops-pp-cli release-apis releases-get-task-log` — Gets the task log of a release as a plain text file.
- `azure-devops-pp-cli release-apis releases-list` — Get a list of releases
- `azure-devops-pp-cli release-apis releases-update-release` — Update a complete release object.
- `azure-devops-pp-cli release-apis releases-update-release-environment` — Update the status of a release environment
- `azure-devops-pp-cli release-apis releases-update-release-resource` — Update few properties of a release.

**search-apis** — Manage search apis

- `azure-devops-pp-cli search-apis <organization>` — Provides a set of results for the search text.

**wiki-apis** — Manage wiki apis

- `azure-devops-pp-cli wiki-apis attachments-create` — Creates an attachment in the wiki.
- `azure-devops-pp-cli wiki-apis page-moves-create` — Creates a page move operation that updates the path and order of the page as provided in the parameters.
- `azure-devops-pp-cli wiki-apis page-stats-get` — Returns page detail corresponding to Page ID.
- `azure-devops-pp-cli wiki-apis pages-batch-get` — Returns pageable list of Wiki Pages
- `azure-devops-pp-cli wiki-apis pages-create-or-update` — Creates or edits a wiki page.
- `azure-devops-pp-cli wiki-apis pages-delete-page` — Deletes a wiki page.
- `azure-devops-pp-cli wiki-apis pages-delete-page-by-id` — Deletes a wiki page.
- `azure-devops-pp-cli wiki-apis pages-get-page` — Gets metadata or content of the wiki page for the provided path.
- `azure-devops-pp-cli wiki-apis pages-get-page-by-id` — Gets metadata or content of the wiki page for the provided page id.
- `azure-devops-pp-cli wiki-apis pages-update` — Edits a wiki page.
- `azure-devops-pp-cli wiki-apis wikis-create` — Creates the wiki resource.
- `azure-devops-pp-cli wiki-apis wikis-delete` — Deletes the wiki corresponding to the wiki ID or wiki name provided.
- `azure-devops-pp-cli wiki-apis wikis-get` — Gets the wiki corresponding to the wiki ID or wiki name provided.
- `azure-devops-pp-cli wiki-apis wikis-list` — Gets all wikis in a project or collection.
- `azure-devops-pp-cli wiki-apis wikis-update` — Updates the wiki corresponding to the wiki ID or wiki name provided using the update parameters.

**work-apis** — Manage work apis

- `azure-devops-pp-cli work-apis boardcolumns-list` — Get available board columns in a project
- `azure-devops-pp-cli work-apis boardrows-list` — Get available board rows in a project
- `azure-devops-pp-cli work-apis deliverytimeline-get` — Get Delivery View Data
- `azure-devops-pp-cli work-apis iterationcapacities-get` — Get an iteration's capacity for all teams in iteration
- `azure-devops-pp-cli work-apis plans-create` — Add a new plan for the team
- `azure-devops-pp-cli work-apis plans-delete` — Delete the specified plan
- `azure-devops-pp-cli work-apis plans-get` — Get the information for the specified plan
- `azure-devops-pp-cli work-apis plans-list` — Get the information for all the plans configured for the given team
- `azure-devops-pp-cli work-apis plans-update` — Update the information for the specified plan
- `azure-devops-pp-cli work-apis processconfiguration-get` — Get process configuration

**workitemtracking-apis** — Manage workitemtracking apis

- `azure-devops-pp-cli workitemtracking-apis account-my-work-recent-activity-list` — Gets recent work item activities
- `azure-devops-pp-cli workitemtracking-apis artifact-link-types-list` — Get the list of work item tracking outbound artifact link types.
- `azure-devops-pp-cli workitemtracking-apis work-item-icons-get` — Get a work item icon given the friendly name and icon color.
- `azure-devops-pp-cli workitemtracking-apis work-item-icons-list` — Get a list of all work item icons.
- `azure-devops-pp-cli workitemtracking-apis work-item-relation-types-get` — Gets the work item relation type definition.
- `azure-devops-pp-cli workitemtracking-apis work-item-relation-types-list` — Gets the work item relation types.
- `azure-devops-pp-cli workitemtracking-apis work-item-transitions-list` — Returns the next state on the given work item IDs.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
azure-devops-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

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

## Auth Setup

Set AZURE_DEVOPS_TOKEN to a Personal Access Token. Generate one at dev.azure.com/{org}/_usersSettings/tokens with scopes: Work Items (read/write), Code (read), Build (read), Release (read/execute). Run `azure-devops-pp-cli auth setup --launch` to open the token page. Save with `azure-devops-pp-cli auth set-token <your-pat>`.

Run `azure-devops-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  azure-devops-pp-cli apis avatar-remove-project-avatar mock-value mock-value --api-version example-value --agent --select id,name,status
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
azure-devops-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
azure-devops-pp-cli feedback --stdin < notes.txt
azure-devops-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/azure-devops-pp-cli/feedback.jsonl`. They are never POSTed unless `AZURE_DEVOPS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `AZURE_DEVOPS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
azure-devops-pp-cli profile save briefing --json
azure-devops-pp-cli --profile briefing apis avatar-remove-project-avatar mock-value mock-value --api-version example-value
azure-devops-pp-cli profile list --json
azure-devops-pp-cli profile show briefing
azure-devops-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Async Jobs

For endpoints that submit long-running work, the generator detects the submit-then-poll pattern (a `job_id`/`task_id`/`operation_id` field in the response plus a sibling status endpoint) and wires up three extra flags on the submitting command:

| Flag | Purpose |
|------|---------|
| `--wait` | Block until the job reaches a terminal status instead of returning the job ID immediately |
| `--wait-timeout` | Maximum wait duration (default 10m, 0 means no timeout) |
| `--wait-interval` | Initial poll interval (default 2s; grows with exponential backoff up to 30s) |

Use async submission without `--wait` when you want to fire-and-forget; use `--wait` when you want one command to return the finished artifact.

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

1. **Empty, `help`, or `--help`** → show `azure-devops-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/developer-tools/azure-devops/cmd/azure-devops-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add azure-devops-pp-mcp -- azure-devops-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which azure-devops-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   azure-devops-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `azure-devops-pp-cli <command> --help`.
