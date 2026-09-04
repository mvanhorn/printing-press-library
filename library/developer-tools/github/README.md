# GitHub CLI

**Every gh issue, PR, repo, and code-search workflow, plus a local SQLite mirror, offline full-text search, and cross-entity joins no GitHub tool offers.**

github-pp-cli mirrors the GitHub collaboration core — issues, pull requests, commits, branches, contents, labels, releases, and search — into a local SQLite store you sync once and query offline forever. It matches the gh CLI and the official MCP server command-for-command on the scoped surfaces, then adds offline FTS search that survives GitHub's rate limits and local joins (issues to PRs to commits to authors) the online API cannot express in a single call.

Learn more at [GitHub](https://support.github.com/contact?tags=dotcom-rest-api).

Created by [@brandon-charles-novice-developer](https://github.com/brandon-charles-novice-developer) (Brandon Nye).

## Install

The recommended path installs both the `github-pp-cli` binary and the `pp-github` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install github
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install github --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install github --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install github --agent claude-code
npx -y @mvanhorn/printing-press-library install github --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/github/cmd/github-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/github-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install github --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-github --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-github --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install github --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/github-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `GITHUB_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/github/cmd/github-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "github": {
      "command": "github-pp-mcp",
      "env": {
        "GITHUB_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Set a personal access token in GITHUB_TOKEN (or GH_TOKEN). A fine-grained or classic PAT with the scopes your workflow needs works; read-only scopes suffice for sync and search. github-pp-cli doctor reports your token state and remaining rate-limit budget.

## Quick Start

```bash
# Confirm the binary, token, and rate-limit budget before doing real work.
github-pp-cli doctor --dry-run

# Populate the local mirror from a repo and find duplicate issues offline.
github-pp-cli issues dupes "rate limit" --repo cli/cli

# Find a symbol across issues, PRs, and commits in one query.
github-pp-cli mentions "ParseConfig" --repo cli/cli

# See who owes the most reviews across all open PRs.
github-pp-cli pulls review-load --repo cli/cli

```

## Unique Features

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

## Recipes


### Triage duplicates before re-labeling

```bash
github-pp-cli issues dupes "connection reset" --limit 10
```

Offline FTS over the synced issue corpus surfaces likely duplicates so you label the canonical one.

### Find a symbol across everything

```bash
github-pp-cli mentions "ParseConfig" --since 30d --json
```

One local query returns every issue, PR comment, and commit message referencing the symbol, tagged by type.

### Assemble an agent read-set for one issue

```bash
github-pp-cli issues context 456 --agent --select issue.title,comments.body,commits.sha
```

Returns the issue, its comments, and mentioning commits as one narrowed JSON envelope so an agent reads its working set in a single offline call.

### Find who owns a file before changing it

```bash
github-pp-cli repos who-touched internal/parser --since 90d
```

Ranks committers by commits touching the path so you know who to ask.

## Usage

Run `github-pp-cli --help` for the full command reference and flag list.

## Commands

### assignees

Manage assignees

- **`github-pp-cli assignees issues-check-user-can-be-assigned`** - Checks if a user has permission to be assigned to an issue in this repository.

If the `assignee` can be assigned to issues in the repository, a `204` header with no content is returned.

Otherwise a `404` status code is returned.
- **`github-pp-cli assignees issues-list`** - Lists the [available assignees](https://docs.github.com/articles/assigning-issues-and-pull-requests-to-other-github-users/) for issues in a repository.

### branches

Manage branches

- **`github-pp-cli branches get-branch`** - Get a branch
- **`github-pp-cli branches list`** - List branches

### collaborators

Manage collaborators

- **`github-pp-cli collaborators add`** - Add a user to a repository with a specified level of access. If the repository is owned by an organization, this API does not add the user to the organization - a user that has repository access without being an organization member is called an "outside collaborator" (if they are not an Enterprise Managed User) or a "repository collaborator" if they are an Enterprise Managed User. These users are exempt from some organization policies - see "[Adding outside collaborators to repositories](https://docs.github.com/organizations/managing-user-access-to-your-organizations-repositories/managing-outside-collaborators/adding-outside-collaborators-to-repositories-in-your-organization)" to learn more about these collaborator types.

This endpoint triggers [notifications](https://docs.github.com/github/managing-subscriptions-and-notifications-on-github/about-notifications).

Adding an outside collaborator may be restricted by enterprise and organization administrators. For more information, see "[Enforcing repository management policies in your enterprise](https://docs.github.com/admin/policies/enforcing-policies-for-your-enterprise/enforcing-repository-management-policies-in-your-enterprise#enforcing-a-policy-for-inviting-outside-collaborators-to-repositories)" and "[Setting permissions for adding outside collaborators](https://docs.github.com/organizations/managing-organization-settings/setting-permissions-for-adding-outside-collaborators)" for organization settings.

For more information on permission levels, see "[Repository permission levels for an organization](https://docs.github.com/github/setting-up-and-managing-organizations-and-teams/repository-permission-levels-for-an-organization#permission-levels-for-repositories-owned-by-an-organization)". There are restrictions on which permissions can be granted to organization members when an organization base role is in place. In this case, the role being given must be equal to or higher than the org base permission. Otherwise, the request will fail with:

```
Cannot assign {member} permission of {role name}
```

Note that, if you choose not to pass any parameters, you'll need to set `Content-Length` to zero when calling out to this endpoint. For more information, see "[HTTP method](https://docs.github.com/rest/guides/getting-started-with-the-rest-api#http-method)."

The invitee will receive a notification that they have been invited to the repository, which they must accept or decline. They may do this via the notifications page, the email they receive, or by using the [API](https://docs.github.com/rest/collaborators/invitations).

For Enterprise Managed Users, this endpoint does not send invitations - these users are automatically added to organizations and repositories. Enterprise Managed Users can only be added to organizations and repositories within their enterprise.

**Updating an existing collaborator's permission level**

The endpoint can also be used to change the permissions of an existing collaborator without first removing and re-adding the collaborator. To change the permissions, use the same endpoint and pass a different `permission` parameter. The response will be a `204`, with no other indication that the permission level changed.

**Rate limits**

You are limited to sending 50 invitations to a repository per 24 hour period. Note there is no limit if you are inviting organization members to an organization repository.
- **`github-pp-cli collaborators check`** - For organization-owned repositories, the list of collaborators includes outside collaborators, organization members that are direct collaborators, organization members with access through team memberships, organization members with access through default organization permissions, and organization owners.

Team members will include the members of child teams.

The authenticated user must have push access to the repository to use this endpoint.

OAuth app tokens and personal access tokens (classic) need the `read:org` and `repo` scopes to use this endpoint.
- **`github-pp-cli collaborators list`** - For organization-owned repositories, the list of collaborators includes outside collaborators, organization members that are direct collaborators, organization members with access through team memberships, organization members with access through default organization permissions, and organization owners.
The `permissions` hash returned in the response contains the base role permissions of the collaborator. The `role_name` is the highest role assigned to the collaborator after considering all sources of grants, including: repo, teams, organization, and enterprise.
There is presently not a way to differentiate between an organization level grant and a repository level grant from this endpoint response.

Team members will include the members of child teams.

The authenticated user must have write, maintain, or admin privileges on the repository to use this endpoint. For organization-owned repositories, the authenticated user needs to be a member of the organization.
OAuth app tokens and personal access tokens (classic) need the `read:org` and `repo` scopes to use this endpoint.
- **`github-pp-cli collaborators remove`** - Removes a collaborator from a repository.

To use this endpoint, the authenticated user must either be an administrator of the repository or target themselves for removal.

This endpoint also:
- Cancels any outstanding invitations sent by the collaborator
- Unassigns the user from any issues
- Removes access to organization projects if the user is not an organization member and is not a collaborator on any other organization repositories.
- Unstars the repository
- Updates access permissions to packages

Removing a user as a collaborator has the following effects on forks:
 - If the user had access to a fork through their membership to this repository, the user will also be removed from the fork.
 - If the user had their own fork of the repository, the fork will be deleted.
 - If the user still has read access to the repository, open pull requests by this user from a fork will be denied.

> [!NOTE]
> A user can still have access to the repository through organization permissions like base repository permissions.

Although the API responds immediately, the additional permission updates might take some extra time to complete in the background.

For more information on fork permissions, see "[About permissions and visibility of forks](https://docs.github.com/pull-requests/collaborating-with-pull-requests/working-with-forks/about-permissions-and-visibility-of-forks)".

### comments

Manage comments

- **`github-pp-cli comments delete-commit`** - Delete a commit comment
- **`github-pp-cli comments get-commit`** - Gets a specified commit comment.

This endpoint supports the following custom media types. For more information, see "[Media types](https://docs.github.com/rest/using-the-rest-api/getting-started-with-the-rest-api#media-types)."

- **`application/vnd.github-commitcomment.raw+json`**: Returns the raw markdown body. Response will include `body`. This is the default if you do not pass any specific media type.
- **`application/vnd.github-commitcomment.text+json`**: Returns a text only representation of the markdown body. Response will include `body_text`.
- **`application/vnd.github-commitcomment.html+json`**: Returns HTML rendered from the body's markdown. Response will include `body_html`.
- **`application/vnd.github-commitcomment.full+json`**: Returns raw, text, and HTML representations. Response will include `body`, `body_text`, and `body_html`.
- **`github-pp-cli comments list-commit-for-repo`** - Lists the commit comments for a specified repository. Comments are ordered by ascending ID.

This endpoint supports the following custom media types. For more information, see "[Media types](https://docs.github.com/rest/using-the-rest-api/getting-started-with-the-rest-api#media-types)."

- **`application/vnd.github-commitcomment.raw+json`**: Returns the raw markdown body. Response will include `body`. This is the default if you do not pass any specific media type.
- **`application/vnd.github-commitcomment.text+json`**: Returns a text only representation of the markdown body. Response will include `body_text`.
- **`application/vnd.github-commitcomment.html+json`**: Returns HTML rendered from the body's markdown. Response will include `body_html`.
- **`application/vnd.github-commitcomment.full+json`**: Returns raw, text, and HTML representations. Response will include `body`, `body_text`, and `body_html`.
- **`github-pp-cli comments update-commit`** - Updates the contents of a specified commit comment.

This endpoint supports the following custom media types. For more information, see "[Media types](https://docs.github.com/rest/using-the-rest-api/getting-started-with-the-rest-api#media-types)."

- **`application/vnd.github-commitcomment.raw+json`**: Returns the raw markdown body. Response will include `body`. This is the default if you do not pass any specific media type.
- **`application/vnd.github-commitcomment.text+json`**: Returns a text only representation of the markdown body. Response will include `body_text`.
- **`application/vnd.github-commitcomment.html+json`**: Returns HTML rendered from the body's markdown. Response will include `body_html`.
- **`application/vnd.github-commitcomment.full+json`**: Returns raw, text, and HTML representations. Response will include `body`, `body_text`, and `body_html`.

### commits

Manage commits

- **`github-pp-cli commits get`** - Returns the contents of a single commit reference. You must have `read` access for the repository to use this endpoint.

> [!NOTE]
> If there are more than 300 files in the commit diff and the default JSON media type is requested, the response will include pagination link headers for the remaining files, up to a limit of 3000 files. Each page contains the static commit information, and the only changes are to the file listing.

This endpoint supports the following custom media types. For more information, see "[Media types](https://docs.github.com/rest/using-the-rest-api/getting-started-with-the-rest-api#media-types)." Pagination query parameters are not supported for these media types.

- **`application/vnd.github.diff`**: Returns the diff of the commit. Larger diffs may time out and return a 5xx status code.
- **`application/vnd.github.patch`**: Returns the patch of the commit. Diffs with binary data will have no `patch` property. Larger diffs may time out and return a 5xx status code.
- **`application/vnd.github.sha`**: Returns the commit's SHA-1 hash. You can use this endpoint to check if a remote reference's SHA-1 hash is the same as your local reference's SHA-1 hash by providing the local SHA-1 reference as the ETag.

**Signature verification object**

The response will include a `verification` object that describes the result of verifying the commit's signature. The following fields are included in the `verification` object:

| Name | Type | Description |
| ---- | ---- | ----------- |
| `verified` | `boolean` | Indicates whether GitHub considers the signature in this commit to be verified. |
| `reason` | `string` | The reason for verified value. Possible values and their meanings are enumerated in table below. |
| `signature` | `string` | The signature that was extracted from the commit. |
| `payload` | `string` | The value that was signed. |
| `verified_at` | `string` | The date the signature was verified by GitHub. |

These are the possible values for `reason` in the `verification` object:

| Value | Description |
| ----- | ----------- |
| `expired_key` | The key that made the signature is expired. |
| `not_signing_key` | The "signing" flag is not among the usage flags in the GPG key that made the signature. |
| `gpgverify_error` | There was an error communicating with the signature verification service. |
| `gpgverify_unavailable` | The signature verification service is currently unavailable. |
| `unsigned` | The object does not include a signature. |
| `unknown_signature_type` | A non-PGP signature was found in the commit. |
| `no_user` | No user was associated with the `committer` email address in the commit. |
| `unverified_email` | The `committer` email address in the commit was associated with a user, but the email address is not verified on their account. |
| `bad_email` | The `committer` email address in the commit is not included in the identities of the PGP key that made the signature. |
| `unknown_key` | The key that made the signature has not been registered with any user's account. |
| `malformed_signature` | There was an error parsing the signature. |
| `invalid` | The signature could not be cryptographically verified using the key whose key-id was found in the signature. |
| `valid` | None of the above errors applied, so the signature is considered to be verified. |
- **`github-pp-cli commits list`** - **Signature verification object**

The response will include a `verification` object that describes the result of verifying the commit's signature. The following fields are included in the `verification` object:

| Name | Type | Description |
| ---- | ---- | ----------- |
| `verified` | `boolean` | Indicates whether GitHub considers the signature in this commit to be verified. |
| `reason` | `string` | The reason for verified value. Possible values and their meanings are enumerated in table below. |
| `signature` | `string` | The signature that was extracted from the commit. |
| `payload` | `string` | The value that was signed. |
| `verified_at` | `string` | The date the signature was verified by GitHub. |

These are the possible values for `reason` in the `verification` object:

| Value | Description |
| ----- | ----------- |
| `expired_key` | The key that made the signature is expired. |
| `not_signing_key` | The "signing" flag is not among the usage flags in the GPG key that made the signature. |
| `gpgverify_error` | There was an error communicating with the signature verification service. |
| `gpgverify_unavailable` | The signature verification service is currently unavailable. |
| `unsigned` | The object does not include a signature. |
| `unknown_signature_type` | A non-PGP signature was found in the commit. |
| `no_user` | No user was associated with the `committer` email address in the commit. |
| `unverified_email` | The `committer` email address in the commit was associated with a user, but the email address is not verified on their account. |
| `bad_email` | The `committer` email address in the commit is not included in the identities of the PGP key that made the signature. |
| `unknown_key` | The key that made the signature has not been registered with any user's account. |
| `malformed_signature` | There was an error parsing the signature. |
| `invalid` | The signature could not be cryptographically verified using the key whose key-id was found in the signature. |
| `valid` | None of the above errors applied, so the signature is considered to be verified. |

### compare

Manage compare

- **`github-pp-cli compare <owner> <repo> <basehead>`** - Compares two commits against one another. You can compare refs (branches or tags) and commit SHAs in the same repository, or you can compare refs and commit SHAs that exist in different repositories within the same repository network, including fork branches. For more information about how to view a repository's network, see "[Understanding connections between repositories](https://docs.github.com/repositories/viewing-activity-and-data-for-your-repository/understanding-connections-between-repositories)."

This endpoint is equivalent to running the `git log BASE..HEAD` command, but it returns commits in a different order. The `git log BASE..HEAD` command returns commits in reverse chronological order, whereas the API returns commits in chronological order.

This endpoint supports the following custom media types. For more information, see "[Media types](https://docs.github.com/rest/using-the-rest-api/getting-started-with-the-rest-api#media-types)."

- **`application/vnd.github.diff`**: Returns the diff of the commit.
- **`application/vnd.github.patch`**: Returns the patch of the commit. Diffs with binary data will have no `patch` property.

The API response includes details about the files that were changed between the two commits. This includes the status of the change (if a file was added, removed, modified, or renamed), and details of the change itself. For example, files with a `renamed` status have a `previous_filename` field showing the previous filename of the file, and files with a `modified` status have a `patch` field showing the changes made to the file.

When calling this endpoint without any paging parameter (`per_page` or `page`), the returned list is limited to 250 commits, and the last commit in the list is the most recent of the entire comparison.

**Working with large comparisons**

To process a response with a large number of commits, use a query parameter (`per_page` or `page`) to paginate the results. When using pagination:

- The list of changed files is only shown on the first page of results, and it includes up to 300 changed files for the entire comparison.
- The results are returned in chronological order, but the last commit in the returned list may not be the most recent one in the entire set if there are more pages of results.

For more information on working with pagination, see "[Using pagination in the REST API](https://docs.github.com/rest/guides/using-pagination-in-the-rest-api)."

**Signature verification object**

The response will include a `verification` object that describes the result of verifying the commit's signature. The `verification` object includes the following fields:

| Name | Type | Description |
| ---- | ---- | ----------- |
| `verified` | `boolean` | Indicates whether GitHub considers the signature in this commit to be verified. |
| `reason` | `string` | The reason for verified value. Possible values and their meanings are enumerated in table below. |
| `signature` | `string` | The signature that was extracted from the commit. |
| `payload` | `string` | The value that was signed. |
| `verified_at` | `string` | The date the signature was verified by GitHub. |

These are the possible values for `reason` in the `verification` object:

| Value | Description |
| ----- | ----------- |
| `expired_key` | The key that made the signature is expired. |
| `not_signing_key` | The "signing" flag is not among the usage flags in the GPG key that made the signature. |
| `gpgverify_error` | There was an error communicating with the signature verification service. |
| `gpgverify_unavailable` | The signature verification service is currently unavailable. |
| `unsigned` | The object does not include a signature. |
| `unknown_signature_type` | A non-PGP signature was found in the commit. |
| `no_user` | No user was associated with the `committer` email address in the commit. |
| `unverified_email` | The `committer` email address in the commit was associated with a user, but the email address is not verified on their account. |
| `bad_email` | The `committer` email address in the commit is not included in the identities of the PGP key that made the signature. |
| `unknown_key` | The key that made the signature has not been registered with any user's account. |
| `malformed_signature` | There was an error parsing the signature. |
| `invalid` | The signature could not be cryptographically verified using the key whose key-id was found in the signature. |
| `valid` | None of the above errors applied, so the signature is considered to be verified. |

### contents

Manage contents

- **`github-pp-cli contents create-or-update-file`** - Creates a new file or replaces an existing file in a repository.

> [!NOTE]
> If you use this endpoint and the "[Delete a file](https://docs.github.com/rest/repos/contents/#delete-a-file)" endpoint in parallel, the concurrent requests will conflict and you will receive errors. You must use these endpoints serially instead.

OAuth app tokens and personal access tokens (classic) need the `repo` scope to use this endpoint. The `workflow` scope is also required in order to modify files in the `.github/workflows` directory.
- **`github-pp-cli contents delete-file`** - Deletes a file in a repository.

You can provide an additional `committer` parameter, which is an object containing information about the committer. Or, you can provide an `author` parameter, which is an object containing information about the author.

The `author` section is optional and is filled in with the `committer` information if omitted. If the `committer` information is omitted, the authenticated user's information is used.

You must provide values for both `name` and `email`, whether you choose to use `author` or `committer`. Otherwise, you'll receive a `422` status code.

> [!NOTE]
> If you use this endpoint and the "[Create or update file contents](https://docs.github.com/rest/repos/contents/#create-or-update-file-contents)" endpoint in parallel, the concurrent requests will conflict and you will receive errors. You must use these endpoints serially instead.
- **`github-pp-cli contents get`** - Gets the contents of a file or directory in a repository. Specify the file path or directory with the `path` parameter. If you omit the `path` parameter, you will receive the contents of the repository's root directory.

This endpoint supports the following custom media types. For more information, see "[Media types](https://docs.github.com/rest/using-the-rest-api/getting-started-with-the-rest-api#media-types)."

- **`application/vnd.github.raw+json`**: Returns the raw file contents for files and symlinks.
- **`application/vnd.github.html+json`**: Returns the file contents in HTML. Markup languages are rendered to HTML using GitHub's open-source [Markup library](https://github.com/github/markup).
- **`application/vnd.github.object+json`**: Returns the contents in a consistent object format regardless of the content type. For example, instead of an array of objects for a directory, the response will be an object with an `entries` attribute containing the array of objects.

If the content is a directory, the response will be an array of objects, one object for each item in the directory. When listing the contents of a directory, submodules have their "type" specified as "file". Logically, the value _should_ be "submodule". This behavior exists [for backwards compatibility purposes](https://git.io/v1YCW). In the next major version of the API, the type will be returned as "submodule".

If the content is a symlink and the symlink's target is a normal file in the repository, then the API responds with the content of the file. Otherwise, the API responds with an object describing the symlink itself.

If the content is a submodule, the `submodule_git_url` field identifies the location of the submodule repository, and the `sha` identifies a specific commit within the submodule repository. Git uses the given URL when cloning the submodule repository, and checks out the submodule at that specific commit. If the submodule repository is not hosted on github.com, the Git URLs (`git_url` and `_links["git"]`) and the github.com URLs (`html_url` and `_links["html"]`) will have null values.

**Notes**:

- To get a repository's contents recursively, you can [recursively get the tree](https://docs.github.com/rest/git/trees#get-a-tree).
- This API has an upper limit of 1,000 files for a directory. If you need to retrieve
more files, use the [Git Trees API](https://docs.github.com/rest/git/trees#get-a-tree).
- Download URLs expire and are meant to be used just once. To ensure the download URL does not expire, please use the contents API to obtain a fresh download URL for each download.
- If the requested file's size is:
  - 1 MB or smaller: All features of this endpoint are supported.
  - Between 1-100 MB: Only the `raw` or `object` custom media types are supported. Both will work as normal, except that when using the `object` media type, the `content` field will be an empty
string and the `encoding` field will be `"none"`. To get the contents of these larger files, use the `raw` media type.
  - Greater than 100 MB: This endpoint is not supported.

### github-search

Manage github search

- **`github-pp-cli github-search code`** - Searches for query terms inside of a file. This method returns up to 100 results [per page](https://docs.github.com/rest/guides/using-pagination-in-the-rest-api).

When searching for code, you can get text match metadata for the file **content** and file **path** fields when you pass the `text-match` media type. For more details about how to receive highlighted search results, see [Text match metadata](https://docs.github.com/rest/search/search#text-match-metadata).

For example, if you want to find the definition of the `addClass` function inside [jQuery](https://github.com/jquery/jquery) repository, your query would look something like this:

`q=addClass+in:file+language:js+repo:jquery/jquery`

This query searches for the keyword `addClass` within a file's contents. The query limits the search to files where the language is JavaScript in the `jquery/jquery` repository.

Considerations for code search:

Due to the complexity of searching code, there are a few restrictions on how searches are performed:

*   Only the _default branch_ is considered. In most cases, this will be the `master` branch.
*   Only files smaller than 384 KB are searchable.
*   You must always include at least one search term when searching source code. For example, searching for [`language:go`](https://github.com/search?utf8=%E2%9C%93&q=language%3Ago&type=Code) is not valid, while [`amazing
language:go`](https://github.com/search?utf8=%E2%9C%93&q=amazing+language%3Ago&type=Code) is.

> [!NOTE]
> `repository.description`, `repository.owner.type`, and `repository.owner.node_id` are closing down on this endpoint and will return `null` in a future API version. Use the [Get a repository](https://docs.github.com/rest/repos/repos#get-a-repository) endpoint (`GET /repos/{owner}/{repo}`) to retrieve full repository metadata.

This endpoint requires you to authenticate and limits you to 10 requests per minute.
- **`github-pp-cli github-search commits`** - Find commits via various criteria on the default branch (usually `main`). This method returns up to 100 results [per page](https://docs.github.com/rest/guides/using-pagination-in-the-rest-api).

When searching for commits, you can get text match metadata for the **message** field when you provide the `text-match` media type. For more details about how to receive highlighted search results, see [Text match
metadata](https://docs.github.com/rest/search/search#text-match-metadata).

For example, if you want to find commits related to CSS in the [octocat/Spoon-Knife](https://github.com/octocat/Spoon-Knife) repository. Your query would look something like this:

`q=repo:octocat/Spoon-Knife+css`
- **`github-pp-cli github-search issues-and-pull-requests`** - Find issues by state and keyword. This method returns up to 100 results [per page](https://docs.github.com/rest/guides/using-pagination-in-the-rest-api).

When searching for issues, you can get text match metadata for the issue **title**, issue **body**, and issue **comment body** fields when you pass the `text-match` media type. For more details about how to receive highlighted
search results, see [Text match metadata](https://docs.github.com/rest/search/search#text-match-metadata).

For example, if you want to find the oldest unresolved Python bugs on Windows. Your query might look something like this.

`q=windows+label:bug+language:python+state:open&sort=created&order=asc`

This query searches for the keyword `windows`, within any open issue that is labeled as `bug`. The search runs across repositories whose primary language is Python. The results are sorted by creation date in ascending order, which means the oldest issues appear first in the search results.

> [!NOTE]
> For requests made by GitHub Apps with a user access token, you can't retrieve a combination of issues and pull requests in a single query. Requests that don't include the `is:issue` or `is:pull-request` qualifier will receive an HTTP `422 Unprocessable Entity` response. To get results for both issues and pull requests, you must send separate queries for issues and pull requests. For more information about the `is` qualifier, see "[Searching only issues or pull requests](https://docs.github.com/github/searching-for-information-on-github/searching-issues-and-pull-requests#search-only-issues-or-pull-requests)."
- **`github-pp-cli github-search labels`** - Find labels in a repository with names or descriptions that match search keywords. Returns up to 100 results [per page](https://docs.github.com/rest/guides/using-pagination-in-the-rest-api).

When searching for labels, you can get text match metadata for the label **name** and **description** fields when you pass the `text-match` media type. For more details about how to receive highlighted search results, see [Text match metadata](https://docs.github.com/rest/search/search#text-match-metadata).

For example, if you want to find labels in the `linguist` repository that match `bug`, `defect`, or `enhancement`. Your query might look like this:

`q=bug+defect+enhancement&repository_id=64778136`

The labels that best match the query appear first in the search results.
- **`github-pp-cli github-search repos`** - Find repositories via various criteria. This method returns up to 100 results [per page](https://docs.github.com/rest/guides/using-pagination-in-the-rest-api).

When searching for repositories, you can get text match metadata for the **name** and **description** fields when you pass the `text-match` media type. For more details about how to receive highlighted search results, see [Text match metadata](https://docs.github.com/rest/search/search#text-match-metadata).

For example, if you want to search for popular Tetris repositories written in assembly code, your query might look like this:

`q=tetris+language:assembly&sort=stars&order=desc`

This query searches for repositories with the word `tetris` in the name, the description, or the README. The results are limited to repositories where the primary language is assembly. The results are sorted by stars in descending order, so that the most popular repositories appear first in the search results.
- **`github-pp-cli github-search topics`** - Find topics via various criteria. Results are sorted by best match. This method returns up to 100 results [per page](https://docs.github.com/rest/guides/using-pagination-in-the-rest-api). See "[Searching topics](https://docs.github.com/articles/searching-topics/)" for a detailed list of qualifiers.

When searching for topics, you can get text match metadata for the topic's **short\_description**, **description**, **name**, or **display\_name** field when you pass the `text-match` media type. For more details about how to receive highlighted search results, see [Text match metadata](https://docs.github.com/rest/search/search#text-match-metadata).

For example, if you want to search for topics related to Ruby that are featured on https://github.com/topics. Your query might look like this:

`q=ruby+is:featured`

This query searches for topics with the keyword `ruby` and limits the results to find only topics that are featured. The topics that are the best match for the query appear first in the search results.
- **`github-pp-cli github-search users`** - Find users via various criteria. This method returns up to 100 results [per page](https://docs.github.com/rest/guides/using-pagination-in-the-rest-api).

When searching for users, you can get text match metadata for the issue **login**, public **email**, and **name** fields when you pass the `text-match` media type. For more details about highlighting search results, see [Text match metadata](https://docs.github.com/rest/search/search#text-match-metadata). For more details about how to receive highlighted search results, see [Text match metadata](https://docs.github.com/rest/search/search#text-match-metadata).

For example, if you're looking for a list of popular users, you might try this query:

`q=tom+repos:%3E42+followers:%3E1000`

This query searches for users with the name `tom`. The results are restricted to users with more than 42 repositories and over 1,000 followers.

This endpoint does not accept authentication and will only include publicly visible users. As an alternative, you can use the GraphQL API. The GraphQL API requires authentication and will return private users, including Enterprise Managed Users (EMUs), that you are authorized to view. For more information, see "[GraphQL Queries](https://docs.github.com/graphql/reference/queries#search)."

### issues

Interact with GitHub Issues.

- **`github-pp-cli issues create`** - Any user with pull access to a repository can create an issue. If [issues are disabled in the repository](https://docs.github.com/articles/disabling-issues/), the API returns a `410 Gone` status.

This endpoint triggers [notifications](https://docs.github.com/github/managing-subscriptions-and-notifications-on-github/about-notifications). Creating content too quickly using this endpoint may result in secondary rate limiting. For more information, see "[Rate limits for the API](https://docs.github.com/rest/using-the-rest-api/rate-limits-for-the-rest-api#about-secondary-rate-limits)"
and "[Best practices for using the REST API](https://docs.github.com/rest/guides/best-practices-for-using-the-rest-api)."

This endpoint supports the following custom media types. For more information, see "[Media types](https://docs.github.com/rest/using-the-rest-api/getting-started-with-the-rest-api#media-types)."

- **`application/vnd.github.raw+json`**: Returns the raw markdown body. Response will include `body`. This is the default if you do not pass any specific media type.
- **`application/vnd.github.text+json`**: Returns a text only representation of the markdown body. Response will include `body_text`.
- **`application/vnd.github.html+json`**: Returns HTML rendered from the body's markdown. Response will include `body_html`.
- **`application/vnd.github.full+json`**: Returns raw, text, and HTML representations. Response will include `body`, `body_text`, and `body_html`.
- **`github-pp-cli issues delete-comment`** - You can use the REST API to delete comments on issues and pull requests. Every pull request is an issue, but not every issue is a pull request.
- **`github-pp-cli issues get`** - The API returns a [`301 Moved Permanently` status](https://docs.github.com/rest/guides/best-practices-for-using-the-rest-api#follow-redirects) if the issue was
[transferred](https://docs.github.com/articles/transferring-an-issue-to-another-repository/) to another repository. If
the issue was transferred to or deleted from a repository where the authenticated user lacks read access, the API
returns a `404 Not Found` status. If the issue was deleted from a repository where the authenticated user has read
access, the API returns a `410 Gone` status. To receive webhook events for transferred and deleted issues, subscribe
to the [`issues`](https://docs.github.com/webhooks/event-payloads/#issues) webhook.

> [!NOTE]
> GitHub's REST API considers every pull request an issue, but not every issue is a pull request. For this reason, "Issues" endpoints may return both issues and pull requests in the response. You can identify pull requests by the `pull_request` key. Be aware that the `id` of a pull request returned from "Issues" endpoints will be an _issue id_. To find out the pull request id, use the "[List pull requests](https://docs.github.com/rest/pulls/pulls#list-pull-requests)" endpoint.

This endpoint supports the following custom media types. For more information, see "[Media types](https://docs.github.com/rest/using-the-rest-api/getting-started-with-the-rest-api#media-types)."

- **`application/vnd.github.raw+json`**: Returns the raw markdown body. Response will include `body`. This is the default if you do not pass any specific media type.
- **`application/vnd.github.text+json`**: Returns a text only representation of the markdown body. Response will include `body_text`.
- **`application/vnd.github.html+json`**: Returns HTML rendered from the body's markdown. Response will include `body_html`.
- **`application/vnd.github.full+json`**: Returns raw, text, and HTML representations. Response will include `body`, `body_text`, and `body_html`.
- **`github-pp-cli issues get-comment`** - You can use the REST API to get comments on issues and pull requests. Every pull request is an issue, but not every issue is a pull request.

This endpoint supports the following custom media types. For more information, see "[Media types](https://docs.github.com/rest/using-the-rest-api/getting-started-with-the-rest-api#media-types)."

- **`application/vnd.github.raw+json`**: Returns the raw markdown body. Response will include `body`. This is the default if you do not pass any specific media type.
- **`application/vnd.github.text+json`**: Returns a text only representation of the markdown body. Response will include `body_text`.
- **`application/vnd.github.html+json`**: Returns HTML rendered from the body's markdown. Response will include `body_html`.
- **`application/vnd.github.full+json`**: Returns raw, text, and HTML representations. Response will include `body`, `body_text`, and `body_html`.
- **`github-pp-cli issues get-event`** - Gets a single event by the event id.
- **`github-pp-cli issues list-comments-for-repo`** - You can use the REST API to list comments on issues and pull requests for a repository. Every pull request is an issue, but not every issue is a pull request.

By default, issue comments are ordered by ascending ID.

This endpoint supports the following custom media types. For more information, see "[Media types](https://docs.github.com/rest/using-the-rest-api/getting-started-with-the-rest-api#media-types)."

- **`application/vnd.github.raw+json`**: Returns the raw markdown body. Response will include `body`. This is the default if you do not pass any specific media type.
- **`application/vnd.github.text+json`**: Returns a text only representation of the markdown body. Response will include `body_text`.
- **`application/vnd.github.html+json`**: Returns HTML rendered from the body's markdown. Response will include `body_html`.
- **`application/vnd.github.full+json`**: Returns raw, text, and HTML representations. Response will include `body`, `body_text`, and `body_html`.
- **`github-pp-cli issues list-events-for-repo`** - List issue events for a repository
- **`github-pp-cli issues list-for-repo`** - List issues in a repository. Only open issues will be listed.

> [!NOTE]
> GitHub's REST API considers every pull request an issue, but not every issue is a pull request. For this reason, "Issues" endpoints may return both issues and pull requests in the response. You can identify pull requests by the `pull_request` key. Be aware that the `id` of a pull request returned from "Issues" endpoints will be an _issue id_. To find out the pull request id, use the "[List pull requests](https://docs.github.com/rest/pulls/pulls#list-pull-requests)" endpoint.

This endpoint supports the following custom media types. For more information, see "[Media types](https://docs.github.com/rest/using-the-rest-api/getting-started-with-the-rest-api#media-types)."

- **`application/vnd.github.raw+json`**: Returns the raw markdown body. Response will include `body`. This is the default if you do not pass any specific media type.
- **`application/vnd.github.text+json`**: Returns a text only representation of the markdown body. Response will include `body_text`.
- **`application/vnd.github.html+json`**: Returns HTML rendered from the body's markdown. Response will include `body_html`.
- **`application/vnd.github.full+json`**: Returns raw, text, and HTML representations. Response will include `body`, `body_text`, and `body_html`.
- **`github-pp-cli issues pin-comment`** - You can use the REST API to pin comments on issues.

This endpoint supports the following custom media types. For more information, see "[Media types](https://docs.github.com/rest/using-the-rest-api/getting-started-with-the-rest-api#media-types)."

- **`application/vnd.github.raw+json`**: Returns the raw markdown body. Response will include `body`. This is the default if you do not pass any specific media type.
- **`application/vnd.github.text+json`**: Returns a text only representation of the markdown body. Response will include `body_text`.
- **`application/vnd.github.html+json`**: Returns HTML rendered from the body's markdown. Response will include `body_html`.
- **`application/vnd.github.full+json`**: Returns raw, text, and HTML representations. Response will include `body`, `body_text`, and `body_html`.
- **`github-pp-cli issues reactions-create-for-comment`** - Create a reaction to an [issue comment](https://docs.github.com/rest/issues/comments#get-an-issue-comment). A response with an HTTP `200` status means that you already added the reaction type to this issue comment.
- **`github-pp-cli issues reactions-delete-for-comment`** - > [!NOTE]
> You can also specify a repository by `repository_id` using the route `DELETE delete /repositories/:repository_id/issues/comments/:comment_id/reactions/:reaction_id`.

Delete a reaction to an [issue comment](https://docs.github.com/rest/issues/comments#get-an-issue-comment).
- **`github-pp-cli issues reactions-list-for-comment`** - List the reactions to an [issue comment](https://docs.github.com/rest/issues/comments#get-an-issue-comment).
- **`github-pp-cli issues unpin-comment`** - You can use the REST API to unpin comments on issues.
- **`github-pp-cli issues update`** - Issue owners and users with push access or Triage role can edit an issue.

This endpoint supports the following custom media types. For more information, see "[Media types](https://docs.github.com/rest/using-the-rest-api/getting-started-with-the-rest-api#media-types)."

- **`application/vnd.github.raw+json`**: Returns the raw markdown body. Response will include `body`. This is the default if you do not pass any specific media type.
- **`application/vnd.github.text+json`**: Returns a text only representation of the markdown body. Response will include `body_text`.
- **`application/vnd.github.html+json`**: Returns HTML rendered from the body's markdown. Response will include `body_html`.
- **`application/vnd.github.full+json`**: Returns raw, text, and HTML representations. Response will include `body`, `body_text`, and `body_html`.
- **`github-pp-cli issues update-comment`** - You can use the REST API to update comments on issues and pull requests. Every pull request is an issue, but not every issue is a pull request.

This endpoint supports the following custom media types. For more information, see "[Media types](https://docs.github.com/rest/using-the-rest-api/getting-started-with-the-rest-api#media-types)."

- **`application/vnd.github.raw+json`**: Returns the raw markdown body. Response will include `body`. This is the default if you do not pass any specific media type.
- **`application/vnd.github.text+json`**: Returns a text only representation of the markdown body. Response will include `body_text`.
- **`application/vnd.github.html+json`**: Returns HTML rendered from the body's markdown. Response will include `body_html`.
- **`application/vnd.github.full+json`**: Returns raw, text, and HTML representations. Response will include `body`, `body_text`, and `body_html`.

### labels

Manage labels

- **`github-pp-cli labels issues-create`** - Creates a label for the specified repository with the given name and color. The name and color parameters are required. The color must be a valid [hexadecimal color code](http://www.color-hex.com/).
- **`github-pp-cli labels issues-delete`** - Deletes a label using the given label name.
- **`github-pp-cli labels issues-get`** - Gets a label using the given name.
- **`github-pp-cli labels issues-list-for-repo`** - Lists all labels for a repository.
- **`github-pp-cli labels issues-update`** - Updates a label using the given label name.

### languages

Manage languages

- **`github-pp-cli languages <owner> <repo>`** - Lists languages for the specified repository. The value shown for each language is the number of bytes of code written in that language.

### milestones

Manage milestones

- **`github-pp-cli milestones issues-create`** - Creates a milestone.
- **`github-pp-cli milestones issues-delete`** - Deletes a milestone using the given milestone number.
- **`github-pp-cli milestones issues-get`** - Gets a milestone using the given milestone number.
- **`github-pp-cli milestones issues-list`** - Lists milestones for a repository.
- **`github-pp-cli milestones issues-update`** - Update a milestone

### pulls

Interact with GitHub Pull Requests.

- **`github-pp-cli pulls create`** - Draft pull requests are available in public repositories with GitHub Free and GitHub Free for organizations, GitHub Pro, and legacy per-repository billing plans, and in public and private repositories with GitHub Team and GitHub Enterprise Cloud. For more information, see [GitHub's products](https://docs.github.com/github/getting-started-with-github/githubs-products) in the GitHub Help documentation.

To open or update a pull request in a public repository, you must have write access to the head or the source branch. For organization-owned repositories, you must be a member of the organization that owns the repository to open or update a pull request.

This endpoint triggers [notifications](https://docs.github.com/github/managing-subscriptions-and-notifications-on-github/about-notifications). Creating content too quickly using this endpoint may result in secondary rate limiting. For more information, see "[Rate limits for the API](https://docs.github.com/rest/using-the-rest-api/rate-limits-for-the-rest-api#about-secondary-rate-limits)" and "[Best practices for using the REST API](https://docs.github.com/rest/guides/best-practices-for-using-the-rest-api)."

This endpoint supports the following custom media types. For more information, see "[Media types](https://docs.github.com/rest/using-the-rest-api/getting-started-with-the-rest-api#media-types)."

- **`application/vnd.github.raw+json`**: Returns the raw markdown body. Response will include `body`. This is the default if you do not pass any specific media type.
- **`application/vnd.github.text+json`**: Returns a text only representation of the markdown body. Response will include `body_text`.
- **`application/vnd.github.html+json`**: Returns HTML rendered from the body's markdown. Response will include `body_html`.
- **`application/vnd.github.full+json`**: Returns raw, text, and HTML representations. Response will include `body`, `body_text`, and `body_html`.
- **`github-pp-cli pulls delete-review-comment`** - Delete a review comment for a pull request
- **`github-pp-cli pulls get`** - Draft pull requests are available in public repositories with GitHub Free and GitHub Free for organizations, GitHub Pro, and legacy per-repository billing plans, and in public and private repositories with GitHub Team and GitHub Enterprise Cloud. For more information, see [GitHub's products](https://docs.github.com/github/getting-started-with-github/githubs-products) in the GitHub Help documentation.

Lists details of a pull request by providing its number.

When you get, [create](https://docs.github.com/rest/pulls/pulls/#create-a-pull-request), or [edit](https://docs.github.com/rest/pulls/pulls#update-a-pull-request) a pull request, GitHub creates a merge commit to test whether the pull request can be automatically merged into the base branch. This test commit is not added to the base branch or the head branch. You can review the status of the test commit using the `mergeable` key. For more information, see "[Checking mergeability of pull requests](https://docs.github.com/rest/guides/getting-started-with-the-git-database-api#checking-mergeability-of-pull-requests)".

The value of the `mergeable` attribute can be `true`, `false`, or `null`. If the value is `null`, then GitHub has started a background job to compute the mergeability. After giving the job time to complete, resubmit the request. When the job finishes, you will see a non-`null` value for the `mergeable` attribute in the response. If `mergeable` is `true`, then `merge_commit_sha` will be the SHA of the _test_ merge commit.

The value of the `merge_commit_sha` attribute changes depending on the state of the pull request. Before merging a pull request, the `merge_commit_sha` attribute holds the SHA of the _test_ merge commit. After merging a pull request, the `merge_commit_sha` attribute changes depending on how you merged the pull request:

*   If merged as a [merge commit](https://docs.github.com/articles/about-merge-methods-on-github/), `merge_commit_sha` represents the SHA of the merge commit.
*   If merged via a [squash](https://docs.github.com/articles/about-merge-methods-on-github/#squashing-your-merge-commits), `merge_commit_sha` represents the SHA of the squashed commit on the base branch.
*   If [rebased](https://docs.github.com/articles/about-merge-methods-on-github/#rebasing-and-merging-your-commits), `merge_commit_sha` represents the commit that the base branch was updated to.

Pass the appropriate [media type](https://docs.github.com/rest/using-the-rest-api/getting-started-with-the-rest-api#media-types) to fetch diff and patch formats.

This endpoint supports the following custom media types. For more information, see "[Media types](https://docs.github.com/rest/using-the-rest-api/getting-started-with-the-rest-api#media-types)."

- **`application/vnd.github.raw+json`**: Returns the raw markdown body. Response will include `body`. This is the default if you do not pass any specific media type.
- **`application/vnd.github.text+json`**: Returns a text only representation of the markdown body. Response will include `body_text`.
- **`application/vnd.github.html+json`**: Returns HTML rendered from the body's markdown. Response will include `body_html`.
- **`application/vnd.github.full+json`**: Returns raw, text, and HTML representations. Response will include `body`, `body_text`, and `body_html`.
- **`application/vnd.github.diff`**: For more information, see "[git-diff](https://git-scm.com/docs/git-diff)" in the Git documentation. If a diff is corrupt, contact us through the [GitHub Support portal](https://support.github.com/). Include the repository name and pull request ID in your message.
- **`github-pp-cli pulls get-review-comment`** - Provides details for a specified review comment.

This endpoint supports the following custom media types. For more information, see "[Media types](https://docs.github.com/rest/using-the-rest-api/getting-started-with-the-rest-api#media-types)."

- **`application/vnd.github-commitcomment.raw+json`**: Returns the raw markdown body. Response will include `body`. This is the default if you do not pass any specific media type.
- **`application/vnd.github-commitcomment.text+json`**: Returns a text only representation of the markdown body. Response will include `body_text`.
- **`application/vnd.github-commitcomment.html+json`**: Returns HTML rendered from the body's markdown. Response will include `body_html`.
- **`application/vnd.github-commitcomment.full+json`**: Returns raw, text, and HTML representations. Response will include `body`, `body_text`, and `body_html`.
- **`github-pp-cli pulls list`** - Lists pull requests in a specified repository.

Draft pull requests are available in public repositories with GitHub
Free and GitHub Free for organizations, GitHub Pro, and legacy per-repository billing
plans, and in public and private repositories with GitHub Team and GitHub Enterprise
Cloud. For more information, see [GitHub's products](https://docs.github.com/github/getting-started-with-github/githubs-products)
in the GitHub Help documentation.

This endpoint supports the following custom media types. For more information, see "[Media types](https://docs.github.com/rest/using-the-rest-api/getting-started-with-the-rest-api#media-types)."

- **`application/vnd.github.raw+json`**: Returns the raw markdown body. Response will include `body`. This is the default if you do not pass any specific media type.
- **`application/vnd.github.text+json`**: Returns a text only representation of the markdown body. Response will include `body_text`.
- **`application/vnd.github.html+json`**: Returns HTML rendered from the body's markdown. Response will include `body_html`.
- **`application/vnd.github.full+json`**: Returns raw, text, and HTML representations. Response will include `body`, `body_text`, and `body_html`.
- **`github-pp-cli pulls list-review-comments-for-repo`** - Lists review comments for all pull requests in a repository. By default,
review comments are in ascending order by ID.

This endpoint supports the following custom media types. For more information, see "[Media types](https://docs.github.com/rest/using-the-rest-api/getting-started-with-the-rest-api#media-types)."

- **`application/vnd.github-commitcomment.raw+json`**: Returns the raw markdown body. Response will include `body`. This is the default if you do not pass any specific media type.
- **`application/vnd.github-commitcomment.text+json`**: Returns a text only representation of the markdown body. Response will include `body_text`.
- **`application/vnd.github-commitcomment.html+json`**: Returns HTML rendered from the body's markdown. Response will include `body_html`.
- **`application/vnd.github-commitcomment.full+json`**: Returns raw, text, and HTML representations. Response will include `body`, `body_text`, and `body_html`.
- **`github-pp-cli pulls reactions-create-for-request-review-comment`** - Create a reaction to a [pull request review comment](https://docs.github.com/rest/pulls/comments#get-a-review-comment-for-a-pull-request). A response with an HTTP `200` status means that you already added the reaction type to this pull request review comment.
- **`github-pp-cli pulls reactions-delete-for-request-comment`** - > [!NOTE]
> You can also specify a repository by `repository_id` using the route `DELETE /repositories/:repository_id/pulls/comments/:comment_id/reactions/:reaction_id.`

Delete a reaction to a [pull request review comment](https://docs.github.com/rest/pulls/comments#get-a-review-comment-for-a-pull-request).
- **`github-pp-cli pulls reactions-list-for-request-review-comment`** - List the reactions to a [pull request review comment](https://docs.github.com/rest/pulls/comments#get-a-review-comment-for-a-pull-request).
- **`github-pp-cli pulls update`** - Draft pull requests are available in public repositories with GitHub Free and GitHub Free for organizations, GitHub Pro, and legacy per-repository billing plans, and in public and private repositories with GitHub Team and GitHub Enterprise Cloud. For more information, see [GitHub's products](https://docs.github.com/github/getting-started-with-github/githubs-products) in the GitHub Help documentation.

To open or update a pull request in a public repository, you must have write access to the head or the source branch. For organization-owned repositories, you must be a member of the organization that owns the repository to open or update a pull request.

This endpoint supports the following custom media types. For more information, see "[Media types](https://docs.github.com/rest/using-the-rest-api/getting-started-with-the-rest-api#media-types)."

- **`application/vnd.github.raw+json`**: Returns the raw markdown body. Response will include `body`. This is the default if you do not pass any specific media type.
- **`application/vnd.github.text+json`**: Returns a text only representation of the markdown body. Response will include `body_text`.
- **`application/vnd.github.html+json`**: Returns HTML rendered from the body's markdown. Response will include `body_html`.
- **`application/vnd.github.full+json`**: Returns raw, text, and HTML representations. Response will include `body`, `body_text`, and `body_html`.
- **`github-pp-cli pulls update-review-comment`** - Edits the content of a specified review comment.

This endpoint supports the following custom media types. For more information, see "[Media types](https://docs.github.com/rest/using-the-rest-api/getting-started-with-the-rest-api#media-types)."

- **`application/vnd.github-commitcomment.raw+json`**: Returns the raw markdown body. Response will include `body`. This is the default if you do not pass any specific media type.
- **`application/vnd.github-commitcomment.text+json`**: Returns a text only representation of the markdown body. Response will include `body_text`.
- **`application/vnd.github-commitcomment.html+json`**: Returns HTML rendered from the body's markdown. Response will include `body_html`.
- **`application/vnd.github-commitcomment.full+json`**: Returns raw, text, and HTML representations. Response will include `body`, `body_text`, and `body_html`.

### releases

Manage releases

- **`github-pp-cli releases create`** - Users with push access to the repository can create a release.

This endpoint triggers [notifications](https://docs.github.com/github/managing-subscriptions-and-notifications-on-github/about-notifications). Creating content too quickly using this endpoint may result in secondary rate limiting. For more information, see "[Rate limits for the API](https://docs.github.com/rest/using-the-rest-api/rate-limits-for-the-rest-api#about-secondary-rate-limits)" and "[Best practices for using the REST API](https://docs.github.com/rest/guides/best-practices-for-using-the-rest-api)."
- **`github-pp-cli releases delete`** - Users with push access to the repository can delete a release.
- **`github-pp-cli releases delete-asset`** - Delete a release asset
- **`github-pp-cli releases generate-notes`** - Generate a name and body describing a [release](https://docs.github.com/rest/releases/releases#get-a-release). The body content will be markdown formatted and contain information like the changes since last release and users who contributed. The generated release notes are not saved anywhere. They are intended to be generated and used when creating a new release.
- **`github-pp-cli releases get`** - Gets a public release with the specified release ID.

> [!NOTE]
> This returns an `upload_url` key corresponding to the endpoint for uploading release assets. This key is a hypermedia resource. For more information, see "[Getting started with the REST API](https://docs.github.com/rest/using-the-rest-api/getting-started-with-the-rest-api#hypermedia)."
- **`github-pp-cli releases get-asset`** - To download the asset's binary content:

- If within a browser, fetch the location specified in the `browser_download_url` key provided in the response.
- Alternatively, set the `Accept` header of the request to
  [`application/octet-stream`](https://docs.github.com/rest/using-the-rest-api/getting-started-with-the-rest-api#media-types).
  The API will either redirect the client to the location, or stream it directly if possible.
  API clients should handle both a `200` or `302` response.
- **`github-pp-cli releases get-by-tag`** - Get a published release with the specified tag.
- **`github-pp-cli releases get-latest`** - View the latest published full release for the repository.

The latest release is the most recent non-prerelease, non-draft release, sorted by the `created_at` attribute. The `created_at` attribute is the date of the commit used for the release, and not the date when the release was drafted or published.
- **`github-pp-cli releases list`** - This returns a list of releases, which does not include regular Git tags that have not been associated with a release. To get a list of Git tags, use the [Repository Tags API](https://docs.github.com/rest/repos/repos#list-repository-tags).

Information about published releases are available to everyone. Only users with push access will receive listings for draft releases.
- **`github-pp-cli releases update`** - Users with push access to the repository can edit a release.
- **`github-pp-cli releases update-asset`** - Users with push access to the repository can edit a release asset.

### repos

Interact with GitHub Repos.

- **`github-pp-cli repos delete`** - Deleting a repository requires admin access.

If an organization owner has configured the organization to prevent members from deleting organization-owned
repositories, you will get a `403 Forbidden` response.

OAuth app tokens and personal access tokens (classic) need the `delete_repo` scope to use this endpoint.
- **`github-pp-cli repos get`** - The `parent` and `source` objects are present when the repository is a fork. `parent` is the repository this repository was forked from, `source` is the ultimate source for the network.

> [!NOTE]
> - In order to see the `security_and_analysis` block for a repository you must have admin permissions for the repository or be an owner or security manager for the organization that owns the repository. For more information, see "[Managing security managers in your organization](https://docs.github.com/organizations/managing-peoples-access-to-your-organization-with-roles/managing-security-managers-in-your-organization)."
> - To view merge-related settings, you must have the `contents:read` and `contents:write` permissions.
- **`github-pp-cli repos update`** - **Note**: To edit a repository's topics, use the [Replace all repository topics](https://docs.github.com/rest/repos/repos#replace-all-repository-topics) endpoint.

### stargazers

Manage stargazers

- **`github-pp-cli stargazers <owner> <repo>`** - Lists the people that have starred the repository.

This endpoint supports the following custom media types. For more information, see "[Media types](https://docs.github.com/rest/using-the-rest-api/getting-started-with-the-rest-api#media-types)."

- **`application/vnd.github.star+json`**: Includes a timestamp of when the star was created.

### subscribers

Manage subscribers

- **`github-pp-cli subscribers <owner> <repo>`** - Lists the people watching the specified repository.

### tags

Manage tags

- **`github-pp-cli tags <owner> <repo>`** - List repository tags

### topics

Manage topics

- **`github-pp-cli topics get-all`** - Get all repository topics
- **`github-pp-cli topics replace-all`** - Replace all repository topics


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
github-pp-cli branches list mock-value mock-value

# JSON for scripting and agents
github-pp-cli branches list mock-value mock-value --json

# Filter to specific fields
github-pp-cli branches list mock-value mock-value --json --select id,name,status

# Dry run — show the request without sending
github-pp-cli branches list mock-value mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
github-pp-cli branches list mock-value mock-value --agent
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
github-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/github-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `GITHUB_TOKEN` | per_call | No | Set to your API credential. |
| `GH_TOKEN` | per_call | No | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `github-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `github-pp-cli doctor` to check credentials
- Verify the environment variable is present without printing it: `test -n "${GITHUB_TOKEN:-}" && echo "GITHUB_TOKEN is set"`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **403 or 429 with a rate-limit message** — Authenticated REST is 5,000/hr and search is 30/min; populate once with --repo and read offline, and check budget with github-pp-cli doctor.
- **A novel command returns empty results** — Those commands read the local mirror — pass --repo owner/repo once to populate it (the framework sync cannot fill GitHub repo-scoped resources).
- **401 Unauthorized** — Set GITHUB_TOKEN (or run gh auth login) and re-check with github-pp-cli doctor.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**gh**](https://github.com/cli/cli) — Go (39000 stars)
- [**hub**](https://github.com/mislav/hub) — Go (22800 stars)
- [**github-mcp-server**](https://github.com/github/github-mcp-server) — Go (19000 stars)
- [**go-github**](https://github.com/google/go-github) — Go (10000 stars)
- [**octokit.js**](https://github.com/octokit/octokit.js) — JavaScript (7800 stars)
- [**PyGithub**](https://github.com/PyGithub/PyGithub) — Python (7100 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
