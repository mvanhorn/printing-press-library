# Github CLI

GitHub's v3 REST API.

Learn more at [Github](https://support.github.com/contact?tags=dotcom-rest-api).

## Install

### Go

```
go install github.com/mvanhorn/printing-press-library/library/developer-tools/github-pp-cli/cmd/github-pp-cli@latest
```

### Binary

Download from [Releases](https://github.com/mvanhorn/printing-press-library/releases).

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Verify Setup

```bash
github-pp-cli doctor
```

This checks your configuration.

### 3. Try Your First Command

```bash
github-pp-cli classrooms list
```

## Usage

Run `github-pp-cli --help` for the full command reference and flag list.

## Commands

### advisories

Manage advisories

- **`github-pp-cli advisories security-get-global-advisory`** - Get a global security advisory
- **`github-pp-cli advisories security-list-global`** - List global security advisories

### app

Information for integrations and installations.

- **`github-pp-cli app create-installation-access-token`** - Create an installation access token for an app
- **`github-pp-cli app delete-installation`** - Delete an installation for the authenticated app
- **`github-pp-cli app get-authenticated`** - Get the authenticated app
- **`github-pp-cli app get-installation`** - Get an installation for the authenticated app
- **`github-pp-cli app get-webhook-config-for`** - Get a webhook configuration for an app
- **`github-pp-cli app get-webhook-delivery`** - Get a delivery for an app webhook
- **`github-pp-cli app list-installation-requests-for-authenticated`** - List installation requests for the authenticated app
- **`github-pp-cli app list-installations`** - List installations for the authenticated app
- **`github-pp-cli app list-webhook-deliveries`** - List deliveries for an app webhook
- **`github-pp-cli app redeliver-webhook-delivery`** - Redeliver a delivery for an app webhook
- **`github-pp-cli app suspend-installation`** - Suspend an app installation
- **`github-pp-cli app unsuspend-installation`** - Unsuspend an app installation
- **`github-pp-cli app update-webhook-config-for`** - Update a webhook configuration for an app

### app-manifests

Manage app manifests


### applications

Manage applications


### apps

Information for integrations and installations.

- **`github-pp-cli apps get-by-slug`** - Get an app

### assignments

Manage assignments

- **`github-pp-cli assignments classroom-get-an`** - Get an assignment

### classrooms

Interact with GitHub Classroom.

- **`github-pp-cli classrooms get-a`** - Get a classroom
- **`github-pp-cli classrooms list`** - List classrooms

### codes-of-conduct

Insight into codes of conduct for your communities.

- **`github-pp-cli codes-of-conduct get-all`** - Get all codes of conduct
- **`github-pp-cli codes-of-conduct get-conduct-code`** - Get a code of conduct

### credentials

Revoke compromised or leaked GitHub credentials.

- **`github-pp-cli credentials revoke`** - Revoke a list of credentials

### emojis

List emojis available to use on GitHub.

- **`github-pp-cli emojis get`** - Get emojis

### enterprises

Manage enterprises


### events

Manage events

- **`github-pp-cli events activity-list-public`** - List public events

### feeds

Manage feeds

- **`github-pp-cli feeds activity-get`** - Get feeds

### gists

View, modify your gists.

- **`github-pp-cli gists create`** - Create a gist
- **`github-pp-cli gists delete`** - Delete a gist
- **`github-pp-cli gists get`** - Get a gist
- **`github-pp-cli gists get-revision`** - Get a gist revision
- **`github-pp-cli gists list`** - List gists for the authenticated user
- **`github-pp-cli gists list-public`** - List public gists
- **`github-pp-cli gists list-starred`** - List starred gists
- **`github-pp-cli gists update`** - Update a gist

### gitignore

View gitignore templates

- **`github-pp-cli gitignore get-all-templates`** - Get all gitignore templates
- **`github-pp-cli gitignore get-template`** - Get a gitignore template

### installation

Manage installation

- **`github-pp-cli installation apps-list-repos-accessible-to`** - List repositories accessible to the app installation
- **`github-pp-cli installation apps-revoke-access-token`** - Revoke an installation access token

### issues

Interact with GitHub Issues.

- **`github-pp-cli issues list`** - List issues assigned to the authenticated user

### licenses

View various OSS licenses.

- **`github-pp-cli licenses get`** - Get a license
- **`github-pp-cli licenses get-all-commonly-used`** - Get all commonly used licenses

### markdown

Render GitHub flavored Markdown

- **`github-pp-cli markdown render`** - Render a Markdown document
- **`github-pp-cli markdown render-raw`** - Render a Markdown document in raw mode

### marketplace-listing

Manage marketplace listing

- **`github-pp-cli marketplace-listing apps-get-subscription-plan-for-account`** - Get a subscription plan for an account
- **`github-pp-cli marketplace-listing apps-get-subscription-plan-for-account-stubbed`** - Get a subscription plan for an account (stubbed)
- **`github-pp-cli marketplace-listing apps-list-accounts-for-plan`** - List accounts for a plan
- **`github-pp-cli marketplace-listing apps-list-accounts-for-plan-stubbed`** - List accounts for a plan (stubbed)
- **`github-pp-cli marketplace-listing apps-list-plans`** - List plans
- **`github-pp-cli marketplace-listing apps-list-plans-stubbed`** - List plans (stubbed)

### meta

Endpoints that give information about the API.

- **`github-pp-cli meta get`** - Get GitHub meta information

### networks

Manage networks


### notifications

Manage notifications

- **`github-pp-cli notifications activity-delete-thread-subscription`** - Delete a thread subscription
- **`github-pp-cli notifications activity-get-thread`** - Get a thread
- **`github-pp-cli notifications activity-get-thread-subscription-for-authenticated-user`** - Get a thread subscription for the authenticated user
- **`github-pp-cli notifications activity-list-for-authenticated-user`** - List notifications for the authenticated user
- **`github-pp-cli notifications activity-mark-as-read`** - Mark notifications as read
- **`github-pp-cli notifications activity-mark-thread-as-done`** - Mark a thread as done
- **`github-pp-cli notifications activity-mark-thread-as-read`** - Mark a thread as read
- **`github-pp-cli notifications activity-set-thread-subscription`** - Set a thread subscription

### octocat

Manage octocat

- **`github-pp-cli octocat meta-get`** - Get Octocat

### organizations

Manage organizations

- **`github-pp-cli organizations orgs-list`** - List organizations

### orgs

Interact with organizations.

- **`github-pp-cli orgs delete`** - Delete an organization
- **`github-pp-cli orgs enable-or-disable-security-product-on-all-repos`** - Enable or disable a security feature for an organization
- **`github-pp-cli orgs get`** - Get an organization
- **`github-pp-cli orgs update`** - Update an organization

### rate-limit

Check your current rate limit status.

- **`github-pp-cli rate-limit get`** - Get rate limit status for the authenticated user

### repos

Interact with GitHub Repos.

- **`github-pp-cli repos delete`** - Delete a repository
- **`github-pp-cli repos get`** - Get a repository
- **`github-pp-cli repos update`** - Update a repository

### repositories

Manage repositories

- **`github-pp-cli repositories repos-list-public`** - List public repositories

### search

Search for specific items on GitHub.

- **`github-pp-cli search code`** - Search code
- **`github-pp-cli search commits`** - Search commits
- **`github-pp-cli search issues-and-pull-requests`** - Search issues and pull requests
- **`github-pp-cli search labels`** - Search labels
- **`github-pp-cli search repos`** - Search repositories
- **`github-pp-cli search topics`** - Search topics
- **`github-pp-cli search users`** - Search users

### teams

Interact with GitHub Teams.

- **`github-pp-cli teams delete-legacy`** - Delete a team (Legacy)
- **`github-pp-cli teams get-legacy`** - Get a team (Legacy)
- **`github-pp-cli teams update-legacy`** - Update a team (Legacy)

### user

Interact with and view information about users and also current user.

- **`github-pp-cli user activity-check-repo-is-starred-by-authenticated`** - Check if a repository is starred by the authenticated user
- **`github-pp-cli user activity-list-repos-starred-by-authenticated`** - List repositories starred by the authenticated user
- **`github-pp-cli user activity-list-watched-repos-for-authenticated`** - List repositories watched by the authenticated user
- **`github-pp-cli user activity-star-repo-for-authenticated`** - Star a repository for the authenticated user
- **`github-pp-cli user activity-unstar-repo-for-authenticated`** - Unstar a repository for the authenticated user
- **`github-pp-cli user add-email-for-authenticated`** - Add an email address for the authenticated user
- **`github-pp-cli user add-social-account-for-authenticated`** - Add social accounts for the authenticated user
- **`github-pp-cli user apps-add-repo-to-installation-for-authenticated`** - Add a repository to an app installation
- **`github-pp-cli user apps-list-installation-repos-for-authenticated`** - List repositories accessible to the user access token
- **`github-pp-cli user apps-list-installations-for-authenticated`** - List app installations accessible to the user access token
- **`github-pp-cli user apps-list-subscriptions-for-authenticated`** - List subscriptions for the authenticated user
- **`github-pp-cli user apps-list-subscriptions-for-authenticated-stubbed`** - List subscriptions for the authenticated user (stubbed)
- **`github-pp-cli user apps-remove-repo-from-installation-for-authenticated`** - Remove a repository from an app installation
- **`github-pp-cli user block`** - Block a user
- **`github-pp-cli user check-blocked`** - Check if a user is blocked by the authenticated user
- **`github-pp-cli user check-person-is-followed-by-authenticated`** - Check if a person is followed by the authenticated user
- **`github-pp-cli user codespaces-add-repository-for-secret-for-authenticated`** - Add a selected repository to a user secret
- **`github-pp-cli user codespaces-codespace-machines-for-authenticated`** - List machine types for a codespace
- **`github-pp-cli user codespaces-create-for-authenticated`** - Create a codespace for the authenticated user
- **`github-pp-cli user codespaces-create-or-update-secret-for-authenticated`** - Create or update a secret for the authenticated user
- **`github-pp-cli user codespaces-delete-for-authenticated`** - Delete a codespace for the authenticated user
- **`github-pp-cli user codespaces-delete-secret-for-authenticated`** - Delete a secret for the authenticated user
- **`github-pp-cli user codespaces-export-for-authenticated`** - Export a codespace for the authenticated user
- **`github-pp-cli user codespaces-get-export-details-for-authenticated`** - Get details about a codespace export
- **`github-pp-cli user codespaces-get-for-authenticated`** - Get a codespace for the authenticated user
- **`github-pp-cli user codespaces-get-public-key-for-authenticated`** - Get public key for the authenticated user
- **`github-pp-cli user codespaces-get-secret-for-authenticated`** - Get a secret for the authenticated user
- **`github-pp-cli user codespaces-list-for-authenticated`** - List codespaces for the authenticated user
- **`github-pp-cli user codespaces-list-repositories-for-secret-for-authenticated`** - List selected repositories for a user secret
- **`github-pp-cli user codespaces-list-secrets-for-authenticated`** - List secrets for the authenticated user
- **`github-pp-cli user codespaces-publish-for-authenticated`** - Create a repository from an unpublished codespace
- **`github-pp-cli user codespaces-remove-repository-for-secret-for-authenticated`** - Remove a selected repository from a user secret
- **`github-pp-cli user codespaces-set-repositories-for-secret-for-authenticated`** - Set selected repositories for a user secret
- **`github-pp-cli user codespaces-start-for-authenticated`** - Start a codespace for the authenticated user
- **`github-pp-cli user codespaces-stop-for-authenticated`** - Stop a codespace for the authenticated user
- **`github-pp-cli user codespaces-update-for-authenticated`** - Update a codespace for the authenticated user
- **`github-pp-cli user create-gpg-key-for-authenticated`** - Create a GPG key for the authenticated user
- **`github-pp-cli user create-public-ssh-key-for-authenticated`** - Create a public SSH key for the authenticated user
- **`github-pp-cli user create-ssh-signing-key-for-authenticated`** - Create a SSH signing key for the authenticated user
- **`github-pp-cli user delete-email-for-authenticated`** - Delete an email address for the authenticated user
- **`github-pp-cli user delete-gpg-key-for-authenticated`** - Delete a GPG key for the authenticated user
- **`github-pp-cli user delete-public-ssh-key-for-authenticated`** - Delete a public SSH key for the authenticated user
- **`github-pp-cli user delete-social-account-for-authenticated`** - Delete social accounts for the authenticated user
- **`github-pp-cli user delete-ssh-signing-key-for-authenticated`** - Delete an SSH signing key for the authenticated user
- **`github-pp-cli user follow`** - Follow a user
- **`github-pp-cli user get-authenticated`** - Get the authenticated user
- **`github-pp-cli user get-by-id`** - Get a user using their ID
- **`github-pp-cli user get-gpg-key-for-authenticated`** - Get a GPG key for the authenticated user
- **`github-pp-cli user get-public-ssh-key-for-authenticated`** - Get a public SSH key for the authenticated user
- **`github-pp-cli user get-ssh-signing-key-for-authenticated`** - Get an SSH signing key for the authenticated user
- **`github-pp-cli user interactions-get-restrictions-for-authenticated`** - Get interaction restrictions for your public repositories
- **`github-pp-cli user interactions-remove-restrictions-for-authenticated`** - Remove interaction restrictions from your public repositories
- **`github-pp-cli user interactions-set-restrictions-for-authenticated`** - Set interaction restrictions for your public repositories
- **`github-pp-cli user issues-list-for-authenticated`** - List user account issues assigned to the authenticated user
- **`github-pp-cli user list-blocked-by-authenticated`** - List users blocked by the authenticated user
- **`github-pp-cli user list-emails-for-authenticated`** - List email addresses for the authenticated user
- **`github-pp-cli user list-followed-by-authenticated`** - List the people the authenticated user follows
- **`github-pp-cli user list-followers-for-authenticated`** - List followers of the authenticated user
- **`github-pp-cli user list-gpg-keys-for-authenticated`** - List GPG keys for the authenticated user
- **`github-pp-cli user list-public-emails-for-authenticated`** - List public email addresses for the authenticated user
- **`github-pp-cli user list-public-ssh-keys-for-authenticated`** - List public SSH keys for the authenticated user
- **`github-pp-cli user list-social-accounts-for-authenticated`** - List social accounts for the authenticated user
- **`github-pp-cli user list-ssh-signing-keys-for-authenticated`** - List SSH signing keys for the authenticated user
- **`github-pp-cli user migrations-delete-archive-for-authenticated`** - Delete a user migration archive
- **`github-pp-cli user migrations-get-archive-for-authenticated`** - Download a user migration archive
- **`github-pp-cli user migrations-get-status-for-authenticated`** - Get a user migration status
- **`github-pp-cli user migrations-list-for-authenticated`** - List user migrations
- **`github-pp-cli user migrations-list-repos-for-authenticated`** - List repositories for a user migration
- **`github-pp-cli user migrations-start-for-authenticated`** - Start a user migration
- **`github-pp-cli user migrations-unlock-repo-for-authenticated`** - Unlock a user repository
- **`github-pp-cli user orgs-get-membership-for-authenticated`** - Get an organization membership for the authenticated user
- **`github-pp-cli user orgs-list-for-authenticated`** - List organizations for the authenticated user
- **`github-pp-cli user orgs-list-memberships-for-authenticated`** - List organization memberships for the authenticated user
- **`github-pp-cli user orgs-update-membership-for-authenticated`** - Update an organization membership for the authenticated user
- **`github-pp-cli user packages-delete-package-for-authenticated`** - Delete a package for the authenticated user
- **`github-pp-cli user packages-delete-package-version-for-authenticated`** - Delete a package version for the authenticated user
- **`github-pp-cli user packages-get-all-package-versions-for-package-owned-by-authenticated`** - List package versions for a package owned by the authenticated user
- **`github-pp-cli user packages-get-package-for-authenticated`** - Get a package for the authenticated user
- **`github-pp-cli user packages-get-package-version-for-authenticated`** - Get a package version for the authenticated user
- **`github-pp-cli user packages-list-docker-migration-conflicting-packages-for-authenticated`** - Get list of conflicting packages during Docker migration for authenticated-user
- **`github-pp-cli user packages-list-packages-for-authenticated`** - List packages for the authenticated user's namespace
- **`github-pp-cli user packages-restore-package-for-authenticated`** - Restore a package for the authenticated user
- **`github-pp-cli user packages-restore-package-version-for-authenticated`** - Restore a package version for the authenticated user
- **`github-pp-cli user repos-accept-invitation-for-authenticated`** - Accept a repository invitation
- **`github-pp-cli user repos-create-for-authenticated`** - Create a repository for the authenticated user
- **`github-pp-cli user repos-decline-invitation-for-authenticated`** - Decline a repository invitation
- **`github-pp-cli user repos-list-for-authenticated`** - List repositories for the authenticated user
- **`github-pp-cli user repos-list-invitations-for-authenticated`** - List repository invitations for the authenticated user
- **`github-pp-cli user set-primary-email-visibility-for-authenticated`** - Set primary email visibility for the authenticated user
- **`github-pp-cli user teams-list-for-authenticated`** - List teams for the authenticated user
- **`github-pp-cli user unblock`** - Unblock a user
- **`github-pp-cli user unfollow`** - Unfollow a user
- **`github-pp-cli user update-authenticated`** - Update the authenticated user

### users

Interact with and view information about users and also current user.

- **`github-pp-cli users get-by-username`** - Get a user
- **`github-pp-cli users list`** - List users

### versions

Manage versions

- **`github-pp-cli versions meta-get-all`** - Get all API versions

### zen

Manage zen

- **`github-pp-cli zen meta-get`** - Get the Zen of GitHub


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
github-pp-cli classrooms list

# JSON for scripting and agents
github-pp-cli classrooms list --json

# Filter to specific fields
github-pp-cli classrooms list --json --select id,name,status

# Dry run — show the request without sending
github-pp-cli classrooms list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
github-pp-cli classrooms list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Retryable** - creates return "already exists" on retry, deletes return "already deleted"
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Use as MCP Server

This CLI ships a companion MCP server for use with Claude Desktop, Cursor, and other MCP-compatible tools.

### Claude Code

```bash
claude mcp add github github-pp-mcp
```

### Claude Desktop

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "github": {
      "command": "github-pp-mcp"
    }
  }
}
```

## Health Check

```bash
github-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/github-pp-cli/config.toml`

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
