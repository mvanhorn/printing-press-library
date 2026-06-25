# Forgejo CLI

**The `gh` for Forgejo — works with any instance, stores your data locally, and speaks Forgejo natively.**

fj brings GitHub CLI-level ergonomics to every Forgejo instance: multi-host auth with OAuth2 device flow, offline-searchable local SQLite, cross-repo dashboards, and Forgejo-specific features (runner management, ActivityPub, repo migration) that no other tool exposes. Works with Codeberg, self-hosted instances, and Forgejo Cloud with the same command surface.

Printed by [@jrimmer](https://github.com/jrimmer) (jrimmer).

## Install

The recommended path installs both the `forgejo-pp-cli` binary and the `pp-forgejo` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install forgejo
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install forgejo --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install forgejo --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install forgejo --agent claude-code
npx -y @mvanhorn/printing-press-library install forgejo --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/forgejo/cmd/forgejo-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/forgejo-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-forgejo --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-forgejo --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-forgejo skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-forgejo. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/forgejo-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `FORGEJO_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/forgejo/cmd/forgejo-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "forgejo": {
      "command": "forgejo-pp-mcp",
      "env": {
        "FORGEJO_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Set your Forgejo personal access token via the `FORGEJO_TOKEN` environment variable or `forgejo-pp-cli auth set-token <token>`. Use `auth setup` to print the steps for generating a token, `auth status` to verify the current credential, and `auth logout` to clear it. Multi-host profiles and OAuth2 device flow are not yet implemented.

## Quick Start

```bash
# Verify fj is configured and reachable
fj doctor --dry-run

# Check CI status across all repos (dry-run shows what would be fetched)
fj ci status --all-repos --dry-run

# See your open issues across all repos
fj issue dashboard --all-repos --assignee @me

# See PRs waiting for your review
fj pr queue --all-repos

# Check unread notifications
fj notification inbox --unread

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`issue dashboard`** — See all your open issues across every repo (and every Forgejo host) in one sorted table — no browser, no tab-switching.

  _Use this when you need a triage-ready list of issues spanning multiple repos or instances without opening a browser._

  ```bash
  fj issue dashboard --all-hosts --assignee @me --label bug --agent
  ```
- **`notification inbox`** — One inbox for all your Forgejo instances — Codeberg, company Forgejo, university instance — sorted by recency with a host column.

  _Use when a contributor works across multiple Forgejo instances and needs to triage notifications without switching browser profiles._

  ```bash
  fj notification inbox --all-hosts --unread --since 24h --agent
  ```
- **`issue sweep`** — Find issues with no activity past a threshold, label them, post a comment, and optionally close — with --dry-run and confirmation before acting.

  _Use during weekly triage automation to keep the issue tracker clean without per-issue web UI clicks._

  ```bash
  fj issue sweep --stale-after 90d --label stale --comment 'No activity in 90 days. Closing unless updated.' --dry-run --agent
  ```
- **`pr queue`** — Your pending reviews across all repos in one view — PRs where you are a requested reviewer, with CI check status and approval counts.

  _Use to surface your pending review obligations across multiple repos without opening each repo's PR list individually._

  ```bash
  fj pr queue --all-repos --review-requested @me --json --agent
  ```

### Forgejo-native admin
- **`runner sweep`** — See the status of every Actions runner across all your orgs in one pass — flag offline runners, watch live with --watch.

  _Use when administering a self-hosted Forgejo instance with multiple org-scoped runners that need monitoring._

  ```bash
  fj runner sweep --all-orgs --watch --json --agent
  ```
- **`ci status`** — Pass/fail status of the latest CI run per repo (or per branch) across all watched repos, with --watch for live tailing.

  _Use before cutting a release to confirm all repos are green, or to monitor CI state across a multi-repo service._

  ```bash
  fj ci status --all-repos --branch main --watch --json --agent
  ```

### Release pipeline automation
- **`release changelog`** — Auto-generate a grouped changelog between two tags from closed issues and merged PRs — label-grouped, markdown or JSON output.

  _Use to generate release notes during a publish workflow without manually scanning the PR list._

  ```bash
  fj release changelog v1.2.0 v1.3.0 --owner myorg --repo myproject --format md --section bug,feature,break --agent
  ```
- **`release upload`** — Upload release assets with a real progress bar, automatic retry on failure, and post-upload size verification — not silent curl.

  _Use during release pipelines where large binary uploads need to be reliable and observable._

  ```bash
  fj release upload v1.3.0 ./dist/*.tar.gz --owner myorg --repo myproject --retry 3 --progress
  ```

## Recipes


### Triage morning inbox

```bash
fj issue dashboard --all-hosts --assignee @me --label bug --agent --select title,repo,updated_at,url
```

Pulls all bug-labeled issues assigned to you across every configured host into a compact agent-readable list.

### Cut a release with changelog

```bash
fj release changelog v1.2.0 v1.3.0 --format md | fj release create --tag v1.3.0 --notes-from-stdin
```

Generates a changelog from closed issues and merged PRs between two tags, pipes it into a new release.

### Upload release assets reliably

```bash
fj release upload v1.3.0 ./dist/*.tar.gz --retry 3 --progress
```

Uploads binaries with progress bar and automatic retry on network failure.

### Sweep stale issues dry-run

```bash
fj issue sweep --stale-after 90d --label stale --dry-run --agent --select number,title,last_activity
```

Shows which issues would be labeled stale without making any changes — review before running without --dry-run.

### Monitor runners across orgs

```bash
fj runner sweep --all-orgs --watch --json --agent --select name,status,last_online,org
```

Streams runner status across all org scopes every 10 seconds; pipe to a Slack webhook for alerts.

## Usage

Run `forgejo-pp-cli --help` for the full command reference and flag list.

## Commands

### activitypub

Manage activitypub

- **`forgejo-pp-cli activitypub instance-actor`** - Returns the instance's Actor
- **`forgejo-pp-cli activitypub instance-actor-inbox`** - Send to the inbox
- **`forgejo-pp-cli activitypub instance-actor-outbox`** - Display the outbox (always empty)
- **`forgejo-pp-cli activitypub person`** - Returns the Person actor for a user
- **`forgejo-pp-cli activitypub person-activity`** - Get a specific activity of the user
- **`forgejo-pp-cli activitypub person-activity-note`** - Get a specific activity object of the user
- **`forgejo-pp-cli activitypub person-feed`** - List the user's recorded activity
- **`forgejo-pp-cli activitypub person-inbox`** - Send to the inbox
- **`forgejo-pp-cli activitypub repository`** - Returns the Repository actor for a repo
- **`forgejo-pp-cli activitypub repository-inbox`** - Send to the inbox
- **`forgejo-pp-cli activitypub repository-outbox`** - Display the outbox

### admin

Manage admin

- **`forgejo-pp-cli admin add-rule-to-quota-group`** - Adds a rule to a quota group
- **`forgejo-pp-cli admin add-user-to-quota-group`** - Add a user to a quota group
- **`forgejo-pp-cli admin adopt-repository`** - Adopt unadopted files as a repository
- **`forgejo-pp-cli admin create-hook`** - Create a hook
- **`forgejo-pp-cli admin create-org`** - Create an organization
- **`forgejo-pp-cli admin create-public-key`** - Add an SSH public key to user's account
- **`forgejo-pp-cli admin create-quota-group`** - Create a new quota group
- **`forgejo-pp-cli admin create-quota-rule`** - Create a new quota rule
- **`forgejo-pp-cli admin create-repo`** - Create a repository on behalf of a user
- **`forgejo-pp-cli admin create-user`** - Create a user account
- **`forgejo-pp-cli admin cron-list`** - List cron tasks
- **`forgejo-pp-cli admin cron-run`** - Run cron task
- **`forgejo-pp-cli admin delete-hook`** - Delete a hook
- **`forgejo-pp-cli admin delete-quota-group`** - Delete a quota group
- **`forgejo-pp-cli admin delete-quota-rule`** - Deletes a quota rule
- **`forgejo-pp-cli admin delete-runner`** - Delete a particular runner, no matter whether it is a global runner or scoped to an organization, user, or repository
- **`forgejo-pp-cli admin delete-unadopted-repository`** - Delete unadopted files
- **`forgejo-pp-cli admin delete-user`** - Delete user account
- **`forgejo-pp-cli admin delete-user-emails`** - Delete email addresses from a user's account
- **`forgejo-pp-cli admin delete-user-public-key`** - Remove a public key from user's account
- **`forgejo-pp-cli admin edit-hook`** - Update a hook
- **`forgejo-pp-cli admin edit-quota-rule`** - Change an existing quota rule
- **`forgejo-pp-cli admin edit-user`** - Edit an existing user
- **`forgejo-pp-cli admin get-action-run-jobs`** - Get action run jobs
- **`forgejo-pp-cli admin get-all-emails`** - List all users' email addresses
- **`forgejo-pp-cli admin get-all-orgs`** - List all organizations
- **`forgejo-pp-cli admin get-hook`** - Get a hook
- **`forgejo-pp-cli admin get-quota-group`** - Get information about the quota group
- **`forgejo-pp-cli admin get-quota-rule`** - Get information about a quota rule
- **`forgejo-pp-cli admin get-registration-token`** - This operation has been deprecated in Forgejo 15. Use the web UI or [`/admin/actions/runners`](#/admin/registerAdminRunner) instead.
- **`forgejo-pp-cli admin get-runner`** - Get a particular runner, no matter whether it is a global runner or scoped to an organization, user, or repository
- **`forgejo-pp-cli admin get-runner-registration-token`** - This operation has been deprecated in Forgejo 15. Use the web UI or [`/admin/actions/runners`](#/admin/registerAdminRunner) instead.
- **`forgejo-pp-cli admin get-runners`** - Get all runners, no matter whether they are global runners or scoped to an organization, user, or repository
- **`forgejo-pp-cli admin get-user-quota`** - Get the user's quota info
- **`forgejo-pp-cli admin list-hooks`** - List global (system) webhooks
- **`forgejo-pp-cli admin list-quota-groups`** - List the available quota groups
- **`forgejo-pp-cli admin list-quota-rules`** - List the available quota rules
- **`forgejo-pp-cli admin list-user-emails`** - List all email addresses for a user
- **`forgejo-pp-cli admin list-users-in-quota-group`** - List users in a quota group
- **`forgejo-pp-cli admin register-runner`** - Register a new global runner
- **`forgejo-pp-cli admin remove-rule-from-quota-group`** - Removes a rule from a quota group
- **`forgejo-pp-cli admin remove-user-from-quota-group`** - Remove a user from a quota group
- **`forgejo-pp-cli admin rename-user`** - Rename a user
- **`forgejo-pp-cli admin search-emails`** - Search users' email addresses
- **`forgejo-pp-cli admin search-run-jobs`** - This operation has been deprecated in Forgejo 15. Use [`/admin/actions/runners/jobs`](#/admin/adminGetActionRunJobs) instead.
- **`forgejo-pp-cli admin search-users`** - Search users according filter conditions
- **`forgejo-pp-cli admin set-user-quota-groups`** - Set the user's quota groups to a given list.
- **`forgejo-pp-cli admin unadopted-list`** - List unadopted repositories

### forgejo-version

Manage forgejo version

- **`forgejo-pp-cli forgejo-version`** - Returns the version of the running application

### gitignore

Manage gitignore

- **`forgejo-pp-cli gitignore get-template-info`** - Returns information about a gitignore template
- **`forgejo-pp-cli gitignore list-templates`** - Returns a list of all gitignore templates

### label

Manage label

- **`forgejo-pp-cli label get-template-info`** - Returns all labels in a template
- **`forgejo-pp-cli label list-templates`** - Returns a list of all label templates

### licenses

Manage licenses

- **`forgejo-pp-cli licenses get-template-info`** - Returns information about a license template
- **`forgejo-pp-cli licenses list-templates`** - Returns a list of all license templates

### markdown

Manage markdown

- **`forgejo-pp-cli markdown render`** - Render a markdown document as HTML
- **`forgejo-pp-cli markdown render-raw`** - Render raw markdown as HTML

### markup

Manage markup

- **`forgejo-pp-cli markup`** - Render a markup document as HTML

### nodeinfo

Manage nodeinfo

- **`forgejo-pp-cli nodeinfo`** - Returns the nodeinfo of the Forgejo application

### notifications

Manage notifications

- **`forgejo-pp-cli notifications notify-get-list`** - List users's notification threads
- **`forgejo-pp-cli notifications notify-get-thread`** - Get notification thread by ID
- **`forgejo-pp-cli notifications notify-new-available`** - Check if unread notifications exist
- **`forgejo-pp-cli notifications notify-read-list`** - Mark notification threads as read, pinned or unread
- **`forgejo-pp-cli notifications notify-read-thread`** - Mark notification thread as read by ID

### org

Manage org


### orgs

Manage orgs

- **`forgejo-pp-cli orgs create`** - Create an organization
- **`forgejo-pp-cli orgs delete`** - Delete an organization
- **`forgejo-pp-cli orgs edit`** - Edit an organization
- **`forgejo-pp-cli orgs get`** - Get an organization
- **`forgejo-pp-cli orgs get-all`** - List all organizations

### packages

Manage packages

- **`forgejo-pp-cli packages delete`** - Delete a package
- **`forgejo-pp-cli packages get`** - Gets a package
- **`forgejo-pp-cli packages link`** - Link a package to a repository
- **`forgejo-pp-cli packages list`** - Gets all packages of an owner
- **`forgejo-pp-cli packages unlink`** - Unlink a package from a repository

### repos

Manage repos

- **`forgejo-pp-cli repos delete`** - Delete a repository
- **`forgejo-pp-cli repos edit`** - Edit a repository's properties. Only fields that are set will be changed.
- **`forgejo-pp-cli repos get`** - Get a repository
- **`forgejo-pp-cli repos issue-search-issues`** - Search for issues across the repositories that the user has access to
- **`forgejo-pp-cli repos migrate`** - Migrate a remote git repository
- **`forgejo-pp-cli repos search`** - Search for repositories

### repositories

Manage repositories

- **`forgejo-pp-cli repositories <id>`** - Get a repository by id

### settings

Manage settings

- **`forgejo-pp-cli settings get-general-apisettings`** - Get instance's global settings for api
- **`forgejo-pp-cli settings get-general-attachment`** - Get instance's global settings for Attachment
- **`forgejo-pp-cli settings get-general-repository`** - Get instance's global settings for repositories
- **`forgejo-pp-cli settings get-general-uisettings`** - Get instance's global settings for ui

### signing-key-gpg

Manage signing key gpg

- **`forgejo-pp-cli signing-key-gpg`** - Get default signing-key.gpg

### signing-key-ssh

Manage signing key ssh

- **`forgejo-pp-cli signing-key-ssh`** - Get default signing-key.ssh

### teams

Manage teams

- **`forgejo-pp-cli teams org-delete`** - Delete a team
- **`forgejo-pp-cli teams org-edit`** - Edit a team
- **`forgejo-pp-cli teams org-get`** - Get a team

### topics

Manage topics

- **`forgejo-pp-cli topics`** - Search for topics by keyword

### user

Manage user

- **`forgejo-pp-cli user add-email`** - Add an email addresses to the current user's account
- **`forgejo-pp-cli user block`** - Blocks a user from the doer
- **`forgejo-pp-cli user check-quota`** - Check if the authenticated user is over quota for a given subject
- **`forgejo-pp-cli user create-current-repo`** - Create a repository
- **`forgejo-pp-cli user create-hook`** - Create a hook
- **`forgejo-pp-cli user create-oauth2-application`** - Creates a new OAuth2 application
- **`forgejo-pp-cli user create-variable`** - Create a user-level variable
- **`forgejo-pp-cli user current-check-following`** - Check whether a user is followed by the authenticated user
- **`forgejo-pp-cli user current-check-starring`** - Whether the authenticated is starring the repo
- **`forgejo-pp-cli user current-delete-follow`** - Unfollow a user
- **`forgejo-pp-cli user current-delete-gpgkey`** - Remove a GPG public key from current user's account
- **`forgejo-pp-cli user current-delete-key`** - Delete a public key
- **`forgejo-pp-cli user current-delete-star`** - Unstar the given repo
- **`forgejo-pp-cli user current-get-gpgkey`** - Get a GPG key
- **`forgejo-pp-cli user current-get-key`** - Get a public key
- **`forgejo-pp-cli user current-list-followers`** - List the authenticated user's followers
- **`forgejo-pp-cli user current-list-following`** - List the users that the authenticated user is following
- **`forgejo-pp-cli user current-list-gpgkeys`** - List the authenticated user's GPG keys
- **`forgejo-pp-cli user current-list-keys`** - List the authenticated user's public keys
- **`forgejo-pp-cli user current-list-repos`** - List the repos that the authenticated user owns
- **`forgejo-pp-cli user current-list-starred`** - The repos that the authenticated user has starred
- **`forgejo-pp-cli user current-list-subscriptions`** - List repositories watched by the authenticated user
- **`forgejo-pp-cli user current-post-gpgkey`** - Add a GPG public key to current user's account
- **`forgejo-pp-cli user current-post-key`** - Create a public key
- **`forgejo-pp-cli user current-put-follow`** - Follow a user
- **`forgejo-pp-cli user current-put-star`** - Star the given repo
- **`forgejo-pp-cli user current-tracked-times`** - List the current user's tracked times
- **`forgejo-pp-cli user delete-avatar`** - Delete avatar of the current user. It will be replaced by a default one
- **`forgejo-pp-cli user delete-email`** - Delete email addresses from the current user's account
- **`forgejo-pp-cli user delete-hook`** - Delete a hook
- **`forgejo-pp-cli user delete-oauth2-application`** - Delete an OAuth2 application
- **`forgejo-pp-cli user delete-runner`** - Delete a particular user-level runner
- **`forgejo-pp-cli user delete-secret`** - Delete a secret in a user scope
- **`forgejo-pp-cli user delete-variable`** - Delete a user-level variable which is created by current doer
- **`forgejo-pp-cli user edit-hook`** - Update a hook
- **`forgejo-pp-cli user get-current`** - Get the authenticated user
- **`forgejo-pp-cli user get-hook`** - Get a hook
- **`forgejo-pp-cli user get-oauth2-application`** - Get an OAuth2 application
- **`forgejo-pp-cli user get-oauth2-applications`** - List the authenticated user's oauth2 applications
- **`forgejo-pp-cli user get-quota`** - Get quota information for the authenticated user
- **`forgejo-pp-cli user get-runner`** - Get a particular runner that belongs to the user
- **`forgejo-pp-cli user get-runner-registration-token`** - This operation has been deprecated in Forgejo 15. Use the web UI or [`/user/actions/runners`](#/user/registerUserRunner) instead.
- **`forgejo-pp-cli user get-runners`** - Get the user's runners
- **`forgejo-pp-cli user get-settings`** - Get current user's account settings
- **`forgejo-pp-cli user get-stop-watches`** - Get list of all existing stopwatches
- **`forgejo-pp-cli user get-variable`** - Get a user-level variable which is created by current doer
- **`forgejo-pp-cli user get-variables-list`** - Get the user-level list of variables which is created by current doer
- **`forgejo-pp-cli user get-verification-token`** - Get a Token to verify
- **`forgejo-pp-cli user list-blocked`** - List the authenticated user's blocked users
- **`forgejo-pp-cli user list-emails`** - List all email addresses of the current user
- **`forgejo-pp-cli user list-hooks`** - List the authenticated user's webhooks
- **`forgejo-pp-cli user list-quota-artifacts`** - List the artifacts affecting the authenticated user's quota
- **`forgejo-pp-cli user list-quota-attachments`** - List the attachments affecting the authenticated user's quota
- **`forgejo-pp-cli user list-quota-packages`** - List the packages affecting the authenticated user's quota
- **`forgejo-pp-cli user list-teams`** - List all the teams a user belongs to
- **`forgejo-pp-cli user org-list-current-orgs`** - List the current user's organizations
- **`forgejo-pp-cli user register-runner`** - Register a new user-level runner
- **`forgejo-pp-cli user search-run-jobs`** - Search for user's action jobs according filter conditions
- **`forgejo-pp-cli user unblock`** - Unblocks a user from the doer
- **`forgejo-pp-cli user update-avatar`** - Update avatar of the current user
- **`forgejo-pp-cli user update-oauth2-application`** - Update an OAuth2 application, this includes regenerating the client secret
- **`forgejo-pp-cli user update-secret`** - Create or Update a secret value in a user scope
- **`forgejo-pp-cli user update-settings`** - Update settings in current user's account
- **`forgejo-pp-cli user update-variable`** - Update a user-level variable which is created by current doer
- **`forgejo-pp-cli user verify-gpgkey`** - Verify a GPG key

### users

Manage users

- **`forgejo-pp-cli users get`** - Get a user
- **`forgejo-pp-cli users search`** - Search for users


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
forgejo-pp-cli forgejo-version

# JSON for scripting and agents
forgejo-pp-cli forgejo-version --json

# Filter to specific fields
forgejo-pp-cli forgejo-version --json --select id,name,status

# Dry run — show the request without sending
forgejo-pp-cli forgejo-version --dry-run

# Agent mode — JSON + compact + no prompts in one flag
forgejo-pp-cli forgejo-version --agent
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
forgejo-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/forgejo-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `FORGEJO_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `forgejo-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $FORGEJO_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **auth: 401 Unauthorized** — Run `fj auth status` to see which token is active; re-run `fj auth login --host <url>` to refresh.
- **pr queue returns nothing** — Run `fj sync --resources pulls` first to populate the local cache.
- **release upload fails on large files** — Use `fj release upload <tag> <file> --retry 3 --progress` for retry and progress tracking.
- **notification inbox empty across hosts** — Run `fj sync --resources notifications --all-hosts` to pull from all configured instances.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**tea**](https://gitea.com/gitea/tea) — Go (243 stars)
- [**forge**](https://github.com/git-pkgs/forge) — Go (147 stars)
- [**git-forge**](https://github.com/Leleat/git-forge) — Rust (12 stars)
- [**gitea-mcp**](https://gitea.com/gitea/gitea-mcp) — Go
- [**codeberg-skill**](https://github.com/rna0/codeberg-skill) — TypeScript
- [**gitea-js**](https://www.npmjs.com/package/gitea-js) — JavaScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
