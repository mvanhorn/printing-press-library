---
name: pp-github
description: "Every gh issue, PR, repo, and code-search workflow, plus a local SQLite mirror, offline full-text search Trigger phrases: `find duplicate issues`, `who owes me reviews`, `what changed since the last release`, `who owns this file`, `search github offline`, `use github-pp-cli`, `run github-pp-cli`."
author: "Brandon Nye"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - github-pp-cli
    install:
      - kind: go
        bins: [github-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/developer-tools/github/cmd/github-pp-cli
---

# GitHub — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `github-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install github --cli-only
   ```
2. Verify: `github-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/github/cmd/github-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

github-pp-cli mirrors the GitHub collaboration core — issues, pull requests, commits, branches, contents, labels, releases, and search — into a local SQLite store you sync once and query offline forever. It matches the gh CLI and the official MCP server command-for-command on the scoped surfaces, then adds offline FTS search that survives GitHub's rate limits and local joins (issues to PRs to commits to authors) the online API cannot express in a single call.

## When to Use This CLI

Choose github-pp-cli when you need to query GitHub's issue, PR, commit, and code-search data repeatedly or offline, when you need cross-entity answers (a symbol across issues+PRs+commits, review backlog by reviewer, file ownership) that take multiple gh calls and a spreadsheet, or when an agent needs an issue's full working set in one rate-limit-friendly read.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI for GitHub Actions, workflow runs, or CI status — that surface is out of scope.
- Do not use this CLI for org, team, or enterprise administration.
- Do not use this CLI for git operations like clone, push, or rebase — use git.
- Do not use this CLI for notifications, gists, or marketplace.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Offline search and cross-entity joins
- **`issues dupes`** — Find open issues whose title and body match a term, ranked by relevance, so you spot duplicates before re-triaging.

  _Reach for this when triaging a new issue and you need to know whether it duplicates an existing one without burning the search rate limit._

  ```bash
  github-pp-cli issues dupes "panic on nil map" --repo cli/cli --limit 10
  ```
- **`mentions`** — One query across issue bodies, PR comments, and commit messages returning every place a symbol or error string appears, tagged by entity type.

  _Pick this over per-resource search when you need every reference to a symbol across issues, PRs, and commits at once._

  ```bash
  github-pp-cli mentions "ParseConfig" --repo cli/cli --since 30d --json
  ```
- **`issues context`** — Assemble one JSON envelope for an issue: the issue, its comments, and the recent commits that mention it, ready for an agent to act on.

  _Use this as the first call when an agent is assigned an issue and needs its full working set in one shot instead of N round-trips._

  ```bash
  github-pp-cli issues context 456 --repo cli/cli --agent --select issue.title,comments.body,commits.sha
  ```

### Release and review operations
- **`pulls review-load`** — Aggregate open PRs by requested reviewer to show who has the largest unactioned review queue.

  _Reach for this in a pre-release sweep to find the review bottleneck before it slips the release train._

  ```bash
  github-pp-cli pulls review-load --repo cli/cli --state open
  ```
- **`pulls stale`** — List open PRs with no new commit, review, or comment in N days, sorted by staleness with last-activity time.

  _Use this to decide which open PRs to nudge or close before they rot._

  ```bash
  github-pp-cli pulls stale --repo cli/cli --older-than 14d
  ```
- **`repos changelog`** — List every commit between two refs, attach its PR when synced, and group by author for release-note prep.

  _Reach for this when drafting release notes and you need what merged since the last tag, organized by contributor._

  ```bash
  github-pp-cli repos changelog --repo cli/cli --base v2.93.0 --head v2.94.0
  ```

### Repo intelligence
- **`repos who-touched`** — For a file or directory prefix, rank committers by number of commits touching that path, with first and last touch dates.

  _Use this when onboarding onto an unfamiliar area to find who knows a file before you ask around._

  ```bash
  github-pp-cli repos who-touched internal/parser --repo cli/cli --since 90d
  ```
- **`labels coverage`** — Report each label's open and closed issue and PR counts, and flag labels that are defined but unused and issues that are unlabeled.

  _Reach for this during triage hygiene to prune dead labels and catch issues that slipped through unlabeled._

  ```bash
  github-pp-cli labels coverage --repo cli/cli
  ```

## Command Reference

**assignees** — Manage assignees

- `github-pp-cli assignees issues-check-user-can-be-assigned` — Checks if a user has permission to be assigned to an issue in this repository.
- `github-pp-cli assignees issues-list` — Lists the [available assignees](https://docs.github.

**branches** — Manage branches

- `github-pp-cli branches get-branch` — Get a branch
- `github-pp-cli branches list` — List branches

**collaborators** — Manage collaborators

- `github-pp-cli collaborators add` — Add a user to a repository with a specified level of access.
- `github-pp-cli collaborators check` — For organization-owned repositories, the list of collaborators includes outside collaborators
- `github-pp-cli collaborators list` — For organization-owned repositories, the list of collaborators includes outside collaborators
- `github-pp-cli collaborators remove` — Removes a collaborator from a repository.

**comments** — Manage comments

- `github-pp-cli comments delete-commit` — Delete a commit comment
- `github-pp-cli comments get-commit` — Gets a specified commit comment. This endpoint supports the following custom media types.
- `github-pp-cli comments list-commit-for-repo` — Lists the commit comments for a specified repository. Comments are ordered by ascending ID.
- `github-pp-cli comments update-commit` — Updates the contents of a specified commit comment. This endpoint supports the following custom media types.

**commits** — Manage commits

- `github-pp-cli commits get` — Returns the contents of a single commit reference. You must have `read` access for the repository to use this endpoint.
- `github-pp-cli commits list` — **Signature verification object** The response will include a `verification` object that describes the result of

**compare** — Manage compare

- `github-pp-cli compare <owner> <repo> <basehead>` — Compares two commits against one another.

**contents** — Manage contents

- `github-pp-cli contents create-or-update-file` — Creates a new file or replaces an existing file in a repository. > [!
- `github-pp-cli contents delete-file` — Deletes a file in a repository.
- `github-pp-cli contents get` — Gets the contents of a file or directory in a repository. Specify the file path or directory with the `path` parameter.

**github-search** — Manage github search

- `github-pp-cli github-search code` — Searches for query terms inside of a file. This method returns up to 100 results [per page](https://docs.github.
- `github-pp-cli github-search commits` — Find commits via various criteria on the default branch (usually `main`).
- `github-pp-cli github-search issues-and-pull-requests` — Find issues by state and keyword. This method returns up to 100 results [per page](https://docs.github.
- `github-pp-cli github-search labels` — Find labels in a repository with names or descriptions that match search keywords.
- `github-pp-cli github-search repos` — Find repositories via various criteria. This method returns up to 100 results [per page](https://docs.github.
- `github-pp-cli github-search topics` — Find topics via various criteria. Results are sorted by best match.
- `github-pp-cli github-search users` — Find users via various criteria. This method returns up to 100 results [per page](https://docs.github.

**issues** — Interact with GitHub Issues.

- `github-pp-cli issues create` — Any user with pull access to a repository can create an issue. If [issues are disabled in the repository](https://docs.
- `github-pp-cli issues delete-comment` — You can use the REST API to delete comments on issues and pull requests.
- `github-pp-cli issues get` — The API returns a [`301 Moved Permanently` status](https://docs.github.
- `github-pp-cli issues get-comment` — You can use the REST API to get comments on issues and pull requests.
- `github-pp-cli issues get-event` — Gets a single event by the event id.
- `github-pp-cli issues list-comments-for-repo` — You can use the REST API to list comments on issues and pull requests for a repository.
- `github-pp-cli issues list-events-for-repo` — List issue events for a repository
- `github-pp-cli issues list-for-repo` — List issues in a repository. Only open issues will be listed. > [!
- `github-pp-cli issues pin-comment` — You can use the REST API to pin comments on issues. This endpoint supports the following custom media types.
- `github-pp-cli issues reactions-create-for-comment` — Create a reaction to an [issue comment](https://docs.github.com/rest/issues/comments#get-an-issue-comment).
- `github-pp-cli issues reactions-delete-for-comment` — > [!
- `github-pp-cli issues reactions-list-for-comment` — List the reactions to an [issue comment](https://docs.github.com/rest/issues/comments#get-an-issue-comment).
- `github-pp-cli issues unpin-comment` — You can use the REST API to unpin comments on issues.
- `github-pp-cli issues update` — Issue owners and users with push access or Triage role can edit an issue.
- `github-pp-cli issues update-comment` — You can use the REST API to update comments on issues and pull requests.

**labels** — Manage labels

- `github-pp-cli labels issues-create` — Creates a label for the specified repository with the given name and color. The name and color parameters are required.
- `github-pp-cli labels issues-delete` — Deletes a label using the given label name.
- `github-pp-cli labels issues-get` — Gets a label using the given name.
- `github-pp-cli labels issues-list-for-repo` — Lists all labels for a repository.
- `github-pp-cli labels issues-update` — Updates a label using the given label name.

**languages** — Manage languages

- `github-pp-cli languages <owner> <repo>` — Lists languages for the specified repository.

**milestones** — Manage milestones

- `github-pp-cli milestones issues-create` — Creates a milestone.
- `github-pp-cli milestones issues-delete` — Deletes a milestone using the given milestone number.
- `github-pp-cli milestones issues-get` — Gets a milestone using the given milestone number.
- `github-pp-cli milestones issues-list` — Lists milestones for a repository.
- `github-pp-cli milestones issues-update` — Update a milestone

**pulls** — Interact with GitHub Pull Requests.

- `github-pp-cli pulls create` — Draft pull requests are available in public repositories with GitHub Free and GitHub Free for organizations, GitHub Pro
- `github-pp-cli pulls delete-review-comment` — Delete a review comment for a pull request
- `github-pp-cli pulls get` — Draft pull requests are available in public repositories with GitHub Free and GitHub Free for organizations, GitHub Pro
- `github-pp-cli pulls get-review-comment` — Provides details for a specified review comment. This endpoint supports the following custom media types.
- `github-pp-cli pulls list` — Lists pull requests in a specified repository.
- `github-pp-cli pulls list-review-comments-for-repo` — Lists review comments for all pull requests in a repository. By default, review comments are in ascending order by ID.
- `github-pp-cli pulls reactions-create-for-request-review-comment` — Create a reaction to a [pull request review comment](https://docs.github.
- `github-pp-cli pulls reactions-delete-for-request-comment` — > [!
- `github-pp-cli pulls reactions-list-for-request-review-comment` — List the reactions to a [pull request review comment](https://docs.github.
- `github-pp-cli pulls update` — Draft pull requests are available in public repositories with GitHub Free and GitHub Free for organizations, GitHub Pro
- `github-pp-cli pulls update-review-comment` — Edits the content of a specified review comment. This endpoint supports the following custom media types.

**releases** — Manage releases

- `github-pp-cli releases create` — Users with push access to the repository can create a release. This endpoint triggers [notifications](https://docs.
- `github-pp-cli releases delete` — Users with push access to the repository can delete a release.
- `github-pp-cli releases delete-asset` — Delete a release asset
- `github-pp-cli releases generate-notes` — Generate a name and body describing a [release](https://docs.github.com/rest/releases/releases#get-a-release).
- `github-pp-cli releases get` — Gets a public release with the specified release ID. > [!
- `github-pp-cli releases get-asset` — To download the asset's binary content: - If within a browser
- `github-pp-cli releases get-by-tag` — Get a published release with the specified tag.
- `github-pp-cli releases get-latest` — View the latest published full release for the repository.
- `github-pp-cli releases list` — This returns a list of releases, which does not include regular Git tags that have not been associated with a release.
- `github-pp-cli releases update` — Users with push access to the repository can edit a release.
- `github-pp-cli releases update-asset` — Users with push access to the repository can edit a release asset.

**repos** — Interact with GitHub Repos.

- `github-pp-cli repos delete` — Deleting a repository requires admin access.
- `github-pp-cli repos get` — The `parent` and `source` objects are present when the repository is a fork.
- `github-pp-cli repos update` — **Note**: To edit a repository's topics, use the [Replace all repository topics](https://docs.github.

**stargazers** — Manage stargazers

- `github-pp-cli stargazers <owner> <repo>` — Lists the people that have starred the repository. This endpoint supports the following custom media types.

**subscribers** — Manage subscribers

- `github-pp-cli subscribers <owner> <repo>` — Lists the people watching the specified repository.

**tags** — Manage tags

- `github-pp-cli tags <owner> <repo>` — List repository tags

**topics** — Manage topics

- `github-pp-cli topics get-all` — Get all repository topics
- `github-pp-cli topics replace-all` — Replace all repository topics


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
github-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Populate the local mirror for a repository

```bash
github-pp-cli sync --repo cli/cli
```

GitHub list endpoints are path-scoped, so `sync` requires `--repo owner/repo`. After this, offline search and the novel aggregation commands can read from the local SQLite store.

### Triage duplicates before re-labeling

```bash
github-pp-cli issues dupes "connection reset" --repo cli/cli --limit 10
```

Populates the local mirror then FTS-matches synced issue titles/bodies so you label the canonical duplicate.

### Find a symbol across everything

```bash
github-pp-cli mentions "ParseConfig" --repo cli/cli --since 30d --json
```

One local query returns every issue, PR, comment, and commit referencing the symbol, tagged by type.

### Assemble an agent read-set for one issue

```bash
github-pp-cli issues context 456 --repo cli/cli --agent --select issue.title,comments.body,commits.sha
```

Returns the issue, its comments, and mentioning commits as one narrowed JSON envelope for a single offline read.

### Find who owns a file before changing it

```bash
github-pp-cli repos who-touched internal/parser --repo cli/cli --since 90d
```

Ranks committers by commits touching the path so you know who to ask.

## Auth Setup

Set a personal access token in GITHUB_TOKEN (or GH_TOKEN). A fine-grained or classic PAT with the scopes your workflow needs works; read-only scopes suffice for sync and search. github-pp-cli doctor reports your token state and remaining rate-limit budget.

Run `github-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  github-pp-cli branches list mock-value mock-value --agent --select id,name,status
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
github-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
github-pp-cli feedback --stdin < notes.txt
github-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/github-pp-cli/feedback.jsonl`. They are never POSTed unless `GITHUB_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `GITHUB_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
github-pp-cli profile save briefing --json
github-pp-cli --profile briefing branches list mock-value mock-value
github-pp-cli profile list --json
github-pp-cli profile show briefing
github-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `github-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/developer-tools/github/cmd/github-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add github-pp-mcp -- github-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which github-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   github-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `github-pp-cli <command> --help`.
