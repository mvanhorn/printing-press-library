---
name: pp-forgejo
description: "The `gh` for Forgejo — works with any instance, stores your data locally, and speaks Forgejo natively. Trigger phrases: `check my forgejo issues`, `list pull requests on forgejo`, `create a release on codeberg`, `runner status across orgs`, `upload release assets to forgejo`, `use fj`, `use forgejo-pp-cli`."
author: "jrimmer"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - forgejo-pp-cli
    install:
      - kind: go
        bins: [forgejo-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/developer-tools/forgejo/cmd/forgejo-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/developer-tools/forgejo/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Forgejo — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `forgejo-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install forgejo --cli-only
   ```
2. Verify: `forgejo-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/forgejo/cmd/forgejo-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

fj brings GitHub CLI-level ergonomics to every Forgejo instance: multi-host auth with OAuth2 device flow, offline-searchable local SQLite, cross-repo dashboards, and Forgejo-specific features (runner management, ActivityPub, repo migration) that no other tool exposes. Works with Codeberg, self-hosted instances, and Forgejo Cloud with the same command surface.

## When to Use This CLI

Use fj when working with any Forgejo or Codeberg instance from the command line: triage issues across multiple repos, manage PRs and releases, monitor CI and runners, or script release pipelines. Especially suited for teams running self-hosted Forgejo who want `gh`-equivalent ergonomics without being on GitHub.

## Unique Capabilities

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

## Command Reference

**activitypub** — Manage activitypub

- `forgejo-pp-cli activitypub instance-actor` — Returns the instance's Actor
- `forgejo-pp-cli activitypub instance-actor-inbox` — Send to the inbox
- `forgejo-pp-cli activitypub instance-actor-outbox` — Display the outbox (always empty)
- `forgejo-pp-cli activitypub person` — Returns the Person actor for a user
- `forgejo-pp-cli activitypub person-activity` — Get a specific activity of the user
- `forgejo-pp-cli activitypub person-activity-note` — Get a specific activity object of the user
- `forgejo-pp-cli activitypub person-feed` — List the user's recorded activity
- `forgejo-pp-cli activitypub person-inbox` — Send to the inbox
- `forgejo-pp-cli activitypub repository` — Returns the Repository actor for a repo
- `forgejo-pp-cli activitypub repository-inbox` — Send to the inbox
- `forgejo-pp-cli activitypub repository-outbox` — Display the outbox

**admin** — Manage admin

- `forgejo-pp-cli admin add-rule-to-quota-group` — Adds a rule to a quota group
- `forgejo-pp-cli admin add-user-to-quota-group` — Add a user to a quota group
- `forgejo-pp-cli admin adopt-repository` — Adopt unadopted files as a repository
- `forgejo-pp-cli admin create-hook` — Create a hook
- `forgejo-pp-cli admin create-org` — Create an organization
- `forgejo-pp-cli admin create-public-key` — Add an SSH public key to user's account
- `forgejo-pp-cli admin create-quota-group` — Create a new quota group
- `forgejo-pp-cli admin create-quota-rule` — Create a new quota rule
- `forgejo-pp-cli admin create-repo` — Create a repository on behalf of a user
- `forgejo-pp-cli admin create-user` — Create a user account
- `forgejo-pp-cli admin cron-list` — List cron tasks
- `forgejo-pp-cli admin cron-run` — Run cron task
- `forgejo-pp-cli admin delete-hook` — Delete a hook
- `forgejo-pp-cli admin delete-quota-group` — Delete a quota group
- `forgejo-pp-cli admin delete-quota-rule` — Deletes a quota rule
- `forgejo-pp-cli admin delete-runner` — Delete a particular runner, no matter whether it is a global runner or scoped to an organization, user, or repository
- `forgejo-pp-cli admin delete-unadopted-repository` — Delete unadopted files
- `forgejo-pp-cli admin delete-user` — Delete user account
- `forgejo-pp-cli admin delete-user-emails` — Delete email addresses from a user's account
- `forgejo-pp-cli admin delete-user-public-key` — Remove a public key from user's account
- `forgejo-pp-cli admin edit-hook` — Update a hook
- `forgejo-pp-cli admin edit-quota-rule` — Change an existing quota rule
- `forgejo-pp-cli admin edit-user` — Edit an existing user
- `forgejo-pp-cli admin get-action-run-jobs` — Get action run jobs
- `forgejo-pp-cli admin get-all-emails` — List all users' email addresses
- `forgejo-pp-cli admin get-all-orgs` — List all organizations
- `forgejo-pp-cli admin get-hook` — Get a hook
- `forgejo-pp-cli admin get-quota-group` — Get information about the quota group
- `forgejo-pp-cli admin get-quota-rule` — Get information about a quota rule
- `forgejo-pp-cli admin get-registration-token` — This operation has been deprecated in Forgejo 15.
- `forgejo-pp-cli admin get-runner` — Get a particular runner, no matter whether it is a global runner or scoped to an organization, user, or repository
- `forgejo-pp-cli admin get-runner-registration-token` — This operation has been deprecated in Forgejo 15.
- `forgejo-pp-cli admin get-runners` — Get all runners, no matter whether they are global runners or scoped to an organization, user, or repository
- `forgejo-pp-cli admin get-user-quota` — Get the user's quota info
- `forgejo-pp-cli admin list-hooks` — List global (system) webhooks
- `forgejo-pp-cli admin list-quota-groups` — List the available quota groups
- `forgejo-pp-cli admin list-quota-rules` — List the available quota rules
- `forgejo-pp-cli admin list-user-emails` — List all email addresses for a user
- `forgejo-pp-cli admin list-users-in-quota-group` — List users in a quota group
- `forgejo-pp-cli admin register-runner` — Register a new global runner
- `forgejo-pp-cli admin remove-rule-from-quota-group` — Removes a rule from a quota group
- `forgejo-pp-cli admin remove-user-from-quota-group` — Remove a user from a quota group
- `forgejo-pp-cli admin rename-user` — Rename a user
- `forgejo-pp-cli admin search-emails` — Search users' email addresses
- `forgejo-pp-cli admin search-run-jobs` — This operation has been deprecated in Forgejo 15.
- `forgejo-pp-cli admin search-users` — Search users according filter conditions
- `forgejo-pp-cli admin set-user-quota-groups` — Set the user's quota groups to a given list.
- `forgejo-pp-cli admin unadopted-list` — List unadopted repositories

**forgejo-version** — Manage forgejo version

- `forgejo-pp-cli forgejo-version` — Returns the version of the running application

**gitignore** — Manage gitignore

- `forgejo-pp-cli gitignore get-template-info` — Returns information about a gitignore template
- `forgejo-pp-cli gitignore list-templates` — Returns a list of all gitignore templates

**label** — Manage label

- `forgejo-pp-cli label get-template-info` — Returns all labels in a template
- `forgejo-pp-cli label list-templates` — Returns a list of all label templates

**licenses** — Manage licenses

- `forgejo-pp-cli licenses get-template-info` — Returns information about a license template
- `forgejo-pp-cli licenses list-templates` — Returns a list of all license templates

**markdown** — Manage markdown

- `forgejo-pp-cli markdown render` — Render a markdown document as HTML
- `forgejo-pp-cli markdown render-raw` — Render raw markdown as HTML

**markup** — Manage markup

- `forgejo-pp-cli markup` — Render a markup document as HTML

**nodeinfo** — Manage nodeinfo

- `forgejo-pp-cli nodeinfo` — Returns the nodeinfo of the Forgejo application

**notifications** — Manage notifications

- `forgejo-pp-cli notifications notify-get-list` — List users's notification threads
- `forgejo-pp-cli notifications notify-get-thread` — Get notification thread by ID
- `forgejo-pp-cli notifications notify-new-available` — Check if unread notifications exist
- `forgejo-pp-cli notifications notify-read-list` — Mark notification threads as read, pinned or unread
- `forgejo-pp-cli notifications notify-read-thread` — Mark notification thread as read by ID

**org** — Manage org


**orgs** — Manage orgs

- `forgejo-pp-cli orgs create` — Create an organization
- `forgejo-pp-cli orgs delete` — Delete an organization
- `forgejo-pp-cli orgs edit` — Edit an organization
- `forgejo-pp-cli orgs get` — Get an organization
- `forgejo-pp-cli orgs get-all` — List all organizations

**packages** — Manage packages

- `forgejo-pp-cli packages delete` — Delete a package
- `forgejo-pp-cli packages get` — Gets a package
- `forgejo-pp-cli packages link` — Link a package to a repository
- `forgejo-pp-cli packages list` — Gets all packages of an owner
- `forgejo-pp-cli packages unlink` — Unlink a package from a repository

**repos** — Manage repos

- `forgejo-pp-cli repos delete` — Delete a repository
- `forgejo-pp-cli repos edit` — Edit a repository's properties. Only fields that are set will be changed.
- `forgejo-pp-cli repos get` — Get a repository
- `forgejo-pp-cli repos issue-search-issues` — Search for issues across the repositories that the user has access to
- `forgejo-pp-cli repos migrate` — Migrate a remote git repository
- `forgejo-pp-cli repos search` — Search for repositories

**repositories** — Manage repositories

- `forgejo-pp-cli repositories <id>` — Get a repository by id

**settings** — Manage settings

- `forgejo-pp-cli settings get-general-apisettings` — Get instance's global settings for api
- `forgejo-pp-cli settings get-general-attachment` — Get instance's global settings for Attachment
- `forgejo-pp-cli settings get-general-repository` — Get instance's global settings for repositories
- `forgejo-pp-cli settings get-general-uisettings` — Get instance's global settings for ui

**signing-key-gpg** — Manage signing key gpg

- `forgejo-pp-cli signing-key-gpg` — Get default signing-key.gpg

**signing-key-ssh** — Manage signing key ssh

- `forgejo-pp-cli signing-key-ssh` — Get default signing-key.ssh

**teams** — Manage teams

- `forgejo-pp-cli teams org-delete` — Delete a team
- `forgejo-pp-cli teams org-edit` — Edit a team
- `forgejo-pp-cli teams org-get` — Get a team

**topics** — Manage topics

- `forgejo-pp-cli topics` — Search for topics by keyword

**user** — Manage user

- `forgejo-pp-cli user add-email` — Add an email addresses to the current user's account
- `forgejo-pp-cli user block` — Blocks a user from the doer
- `forgejo-pp-cli user check-quota` — Check if the authenticated user is over quota for a given subject
- `forgejo-pp-cli user create-current-repo` — Create a repository
- `forgejo-pp-cli user create-hook` — Create a hook
- `forgejo-pp-cli user create-oauth2-application` — Creates a new OAuth2 application
- `forgejo-pp-cli user create-variable` — Create a user-level variable
- `forgejo-pp-cli user current-check-following` — Check whether a user is followed by the authenticated user
- `forgejo-pp-cli user current-check-starring` — Whether the authenticated is starring the repo
- `forgejo-pp-cli user current-delete-follow` — Unfollow a user
- `forgejo-pp-cli user current-delete-gpgkey` — Remove a GPG public key from current user's account
- `forgejo-pp-cli user current-delete-key` — Delete a public key
- `forgejo-pp-cli user current-delete-star` — Unstar the given repo
- `forgejo-pp-cli user current-get-gpgkey` — Get a GPG key
- `forgejo-pp-cli user current-get-key` — Get a public key
- `forgejo-pp-cli user current-list-followers` — List the authenticated user's followers
- `forgejo-pp-cli user current-list-following` — List the users that the authenticated user is following
- `forgejo-pp-cli user current-list-gpgkeys` — List the authenticated user's GPG keys
- `forgejo-pp-cli user current-list-keys` — List the authenticated user's public keys
- `forgejo-pp-cli user current-list-repos` — List the repos that the authenticated user owns
- `forgejo-pp-cli user current-list-starred` — The repos that the authenticated user has starred
- `forgejo-pp-cli user current-list-subscriptions` — List repositories watched by the authenticated user
- `forgejo-pp-cli user current-post-gpgkey` — Add a GPG public key to current user's account
- `forgejo-pp-cli user current-post-key` — Create a public key
- `forgejo-pp-cli user current-put-follow` — Follow a user
- `forgejo-pp-cli user current-put-star` — Star the given repo
- `forgejo-pp-cli user current-tracked-times` — List the current user's tracked times
- `forgejo-pp-cli user delete-avatar` — Delete avatar of the current user. It will be replaced by a default one
- `forgejo-pp-cli user delete-email` — Delete email addresses from the current user's account
- `forgejo-pp-cli user delete-hook` — Delete a hook
- `forgejo-pp-cli user delete-oauth2-application` — Delete an OAuth2 application
- `forgejo-pp-cli user delete-runner` — Delete a particular user-level runner
- `forgejo-pp-cli user delete-secret` — Delete a secret in a user scope
- `forgejo-pp-cli user delete-variable` — Delete a user-level variable which is created by current doer
- `forgejo-pp-cli user edit-hook` — Update a hook
- `forgejo-pp-cli user get-current` — Get the authenticated user
- `forgejo-pp-cli user get-hook` — Get a hook
- `forgejo-pp-cli user get-oauth2-application` — Get an OAuth2 application
- `forgejo-pp-cli user get-oauth2-applications` — List the authenticated user's oauth2 applications
- `forgejo-pp-cli user get-quota` — Get quota information for the authenticated user
- `forgejo-pp-cli user get-runner` — Get a particular runner that belongs to the user
- `forgejo-pp-cli user get-runner-registration-token` — This operation has been deprecated in Forgejo 15.
- `forgejo-pp-cli user get-runners` — Get the user's runners
- `forgejo-pp-cli user get-settings` — Get current user's account settings
- `forgejo-pp-cli user get-stop-watches` — Get list of all existing stopwatches
- `forgejo-pp-cli user get-variable` — Get a user-level variable which is created by current doer
- `forgejo-pp-cli user get-variables-list` — Get the user-level list of variables which is created by current doer
- `forgejo-pp-cli user get-verification-token` — Get a Token to verify
- `forgejo-pp-cli user list-blocked` — List the authenticated user's blocked users
- `forgejo-pp-cli user list-emails` — List all email addresses of the current user
- `forgejo-pp-cli user list-hooks` — List the authenticated user's webhooks
- `forgejo-pp-cli user list-quota-artifacts` — List the artifacts affecting the authenticated user's quota
- `forgejo-pp-cli user list-quota-attachments` — List the attachments affecting the authenticated user's quota
- `forgejo-pp-cli user list-quota-packages` — List the packages affecting the authenticated user's quota
- `forgejo-pp-cli user list-teams` — List all the teams a user belongs to
- `forgejo-pp-cli user org-list-current-orgs` — List the current user's organizations
- `forgejo-pp-cli user register-runner` — Register a new user-level runner
- `forgejo-pp-cli user search-run-jobs` — Search for user's action jobs according filter conditions
- `forgejo-pp-cli user unblock` — Unblocks a user from the doer
- `forgejo-pp-cli user update-avatar` — Update avatar of the current user
- `forgejo-pp-cli user update-oauth2-application` — Update an OAuth2 application, this includes regenerating the client secret
- `forgejo-pp-cli user update-secret` — Create or Update a secret value in a user scope
- `forgejo-pp-cli user update-settings` — Update settings in current user's account
- `forgejo-pp-cli user update-variable` — Update a user-level variable which is created by current doer
- `forgejo-pp-cli user verify-gpgkey` — Verify a GPG key

**users** — Manage users

- `forgejo-pp-cli users get` — Get a user
- `forgejo-pp-cli users search` — Search for users


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
forgejo-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

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

## Auth Setup

Set your Forgejo personal access token via the `FORGEJO_TOKEN` environment variable or `forgejo-pp-cli auth set-token <token>`. Use `auth setup` to print the steps for generating a token, `auth status` to verify the current credential, and `auth logout` to clear it. Multi-host profiles and OAuth2 device flow are not yet implemented.

Run `forgejo-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  forgejo-pp-cli forgejo-version --agent --select id,name,status
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
forgejo-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
forgejo-pp-cli feedback --stdin < notes.txt
forgejo-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/forgejo-pp-cli/feedback.jsonl`. They are never POSTed unless `FORGEJO_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `FORGEJO_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
forgejo-pp-cli profile save briefing --json
forgejo-pp-cli --profile briefing forgejo-version
forgejo-pp-cli profile list --json
forgejo-pp-cli profile show briefing
forgejo-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `forgejo-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/developer-tools/forgejo/cmd/forgejo-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add forgejo-pp-mcp -- forgejo-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which forgejo-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   forgejo-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `forgejo-pp-cli <command> --help`.
