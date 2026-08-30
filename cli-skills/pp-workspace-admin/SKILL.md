---
name: pp-workspace-admin
description: "GAT-style Google Workspace auditing as an agent-native CLI: sync Directory, Drive, Gmail, Reports, and Alert Center into a local store, then run offline cross-API audits, offboarding, and OAuth-app risk checks that GAM and the official gws CLI cannot persist. Trigger phrases: `offboard a user`, `audit external Drive sharing`, `which OAuth apps have Full Drive access`, `show me this user's security posture`, `reconstruct this account's activity`, `use workspace-admin`, `run workspace-admin`."
author: "RyanGravetteIDLA"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - workspace-admin-pp-cli
    install:
      - kind: go
        bins: [workspace-admin-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/productivity/workspace-admin/cmd/workspace-admin-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/productivity/workspace-admin/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Google Workspace Admin — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `workspace-admin-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install workspace-admin --cli-only
   ```
2. Verify: `workspace-admin-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/workspace-admin/cmd/workspace-admin-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

A curated Google Workspace admin and audit tool built on the Admin SDK (Directory, Reports, Alert Center), Drive, and Gmail. Unlike stateless tools, it syncs Workspace metadata into a local SQLite store so audits like audit external-shares, audit app-risk, and audit user360 run offline and join data across APIs. workflow offboard executes a departing user's full lifecycle in one command, and every command is agent-native with --json, --select, --dry-run, and typed exit codes.

## When to Use This CLI

Use this CLI for GAT-style Google Workspace auditing and lifecycle automation: offboarding departing users, auditing external Drive sharing, scoring third-party OAuth app risk, reviewing login anomalies and dormant accounts, reconstructing a compromised account's activity, and producing offline cross-API reports over Directory, Drive, Gmail, Reports, and Alert Center. It is the right choice when you want joinable, persisted audit data that stateless tools like GAM and the official gws CLI do not keep.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI for real-time browser DLP, keyword interception, screenshots, or on-screen blocking — those require GAT Shield's Chrome endpoint extension and have no Google API equivalent.
- Do not use this CLI as a mail-flow gateway to block phishing before delivery; it performs post-hoc metadata audit and remediation, not pre-inbox filtering.
- Do not use this CLI to read another user's file or email contents without a service account configured for domain-wide delegation and impersonation.
- Do not use this CLI for Google Cloud / IAM resource management — use gcloud; this tool manages Google Workspace, not GCP.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Lifecycle workflows that compound
- **`workflow offboard`** — Run a departing user's entire lifecycle in one command — suspend, sign out, revoke OAuth tokens, transfer Drive ownership, set Gmail delegation, remove from groups, and move to a suspended OU.

  _Reach for this instead of running eight separate endpoint commands when a user leaves; it guarantees no step is skipped and records what completed._

  ```bash
  workspace-admin-pp-cli workflow offboard departing@example.com --manager manager@example.com --dry-run
  ```

### Cross-API audits only the local store enables
- **`audit user360`** — One per-user posture report joining the user's Drive footprint and quota, security (2SV, suspended), connected OAuth apps, devices, Gmail settings, group memberships, and recent activity.

  _Pick this to answer 'what is going on with this account' in one call instead of stitching five APIs together._

  ```bash
  workspace-admin-pp-cli audit user360 user@example.com --agent --select security,apps,devices
  ```
- **`audit external-shares`** — Find every Drive file shared 'anyone with link' or with an external domain, joined to its owner and the owner's org unit.

  _Use this for the domain-wide external Drive exposure sweep; for one user's Drive footprint use audit user360._

  ```bash
  workspace-admin-pp-cli audit external-shares --domain gmail.com --agent --select files.name,files.owner,files.permissions.emailAddress
  ```
- **`audit reconstruct`** — Merge one user's login, admin, Drive, and token activity into a single ordered timeline for post-compromise review.

  _Pick this after a suspected compromise to see everything an account did in order; for domain-wide anomalies use audit logins._

  ```bash
  workspace-admin-pp-cli audit reconstruct user@example.com --since 7d --agent
  ```
- **`audit email-exposure`** — Sweep every user's Gmail forwarding, sendAs, delegates, and filters for external forwarding, external send-as, delegates, and forward/delete rules — the standard business-email-compromise indicators.

  _Use this for the domain-wide BEC sweep of forwarding rules, delegates, and suspicious filters; for one user's full email settings use audit user360._

  ```bash
  workspace-admin-pp-cli audit email-exposure --agent --select findings.user,findings.type,findings.detail
  ```

### Security posture
- **`audit app-risk`** — Roll up every third-party OAuth app authorized in the domain by a curated scope-to-risk tier (Low/Moderate/High), flag apps with Full Drive access, and count users-per-app with the member list.

  _Reach for this during third-party app review to rank risk and find which users authorized a dangerous app._

  ```bash
  workspace-admin-pp-cli audit app-risk --min-risk high --agent
  ```
- **`audit logins`** — Surface failed-login bursts, new-country logins, and dormant accounts (no login since N days) from synced login activity joined to users.

  _Use this for the weekly login-anomaly and dormant-account review; for a single user's forensic timeline use audit reconstruct._

  ```bash
  workspace-admin-pp-cli audit logins --failures --geo --since 7d --agent
  ```

### Local state that compounds
- **`groups expand`** — Recursively flatten nested groups into their effective direct-user membership, cycle-safe.

  _Use this to answer 'who is effectively in this group' before an access decision; to list one user's group memberships use audit user360._

  ```bash
  workspace-admin-pp-cli groups expand all-staff@example.com --agent --select members.email,members.viaGroup
  ```

## Command Reference

**admin** — Manage admin

- `workspace-admin-pp-cli admin channels-stop` — Stops watching resources through this channel.
- `workspace-admin-pp-cli admin customer-devices-chromeos-commands-get` — Gets command data a specific command issued to the device.
- `workspace-admin-pp-cli admin customer-devices-chromeos-issue-command` — Issues a command for the device to execute.
- `workspace-admin-pp-cli admin customers-chrome-print-servers-batch-create-print-servers` — Creates multiple print servers.
- `workspace-admin-pp-cli admin customers-chrome-print-servers-batch-delete-print-servers` — Deletes multiple print servers.
- `workspace-admin-pp-cli admin customers-chrome-print-servers-create` — Creates a print server.
- `workspace-admin-pp-cli admin customers-chrome-print-servers-delete` — Deletes a print server.
- `workspace-admin-pp-cli admin customers-chrome-print-servers-get` — Returns a print server's configuration.
- `workspace-admin-pp-cli admin customers-chrome-print-servers-list` — Lists print server configurations.
- `workspace-admin-pp-cli admin customers-chrome-print-servers-patch` — Updates a print server's configuration.
- `workspace-admin-pp-cli admin customers-chrome-printers-batch-create-printers` — Creates printers under given Organization Unit.
- `workspace-admin-pp-cli admin customers-chrome-printers-batch-delete-printers` — Deletes printers in batch.
- `workspace-admin-pp-cli admin customers-chrome-printers-create` — Creates a printer under given Organization Unit.
- `workspace-admin-pp-cli admin customers-chrome-printers-list` — List printers configs.
- `workspace-admin-pp-cli admin customers-chrome-printers-list-printer-models` — Lists the supported printer models.
- `workspace-admin-pp-cli admin directory-asps-delete` — Deletes an ASP issued by a user.
- `workspace-admin-pp-cli admin directory-asps-get` — Gets information about an ASP issued by a user.
- `workspace-admin-pp-cli admin directory-asps-list` — Lists the ASPs issued by a user.
- `workspace-admin-pp-cli admin directory-chromeosdevices-action` — Takes an action that affects a Chrome OS Device. This includes deprovisioning, disabling, and re-enabling devices.
- `workspace-admin-pp-cli admin directory-chromeosdevices-get` — Retrieves a Chrome OS device's properties.
- `workspace-admin-pp-cli admin directory-chromeosdevices-list` — Retrieves a paginated list of Chrome OS devices within an account.
- `workspace-admin-pp-cli admin directory-chromeosdevices-move-devices-to-ou` — Moves or inserts multiple Chrome OS devices to an organizational unit. You can move up to 50 devices at once.
- `workspace-admin-pp-cli admin directory-chromeosdevices-patch` — Updates a device's updatable properties, such as `annotatedUser`, `annotatedLocation`, `notes`, `orgUnitPath`
- `workspace-admin-pp-cli admin directory-chromeosdevices-update` — Updates a device's updatable properties, such as `annotatedUser`, `annotatedLocation`, `notes`, `orgUnitPath`
- `workspace-admin-pp-cli admin directory-customers-get` — Retrieves a customer.
- `workspace-admin-pp-cli admin directory-customers-patch` — Patches a customer.
- `workspace-admin-pp-cli admin directory-customers-update` — Updates a customer.
- `workspace-admin-pp-cli admin directory-domain-aliases-delete` — Deletes a domain Alias of the customer.
- `workspace-admin-pp-cli admin directory-domain-aliases-get` — Retrieves a domain alias of the customer.
- `workspace-admin-pp-cli admin directory-domain-aliases-insert` — Inserts a domain alias of the customer.
- `workspace-admin-pp-cli admin directory-domain-aliases-list` — Lists the domain aliases of the customer.
- `workspace-admin-pp-cli admin directory-domains-delete` — Deletes a domain of the customer.
- `workspace-admin-pp-cli admin directory-domains-get` — Retrieves a domain of the customer.
- `workspace-admin-pp-cli admin directory-domains-insert` — Inserts a domain of the customer.
- `workspace-admin-pp-cli admin directory-domains-list` — Lists the domains of the customer.
- `workspace-admin-pp-cli admin directory-groups-aliases-delete` — Removes an alias.
- `workspace-admin-pp-cli admin directory-groups-aliases-insert` — Adds an alias for the group.
- `workspace-admin-pp-cli admin directory-groups-aliases-list` — Lists all aliases for a group.
- `workspace-admin-pp-cli admin directory-groups-delete` — Deletes a group.
- `workspace-admin-pp-cli admin directory-groups-get` — Retrieves a group's properties.
- `workspace-admin-pp-cli admin directory-groups-insert` — Creates a group.
- `workspace-admin-pp-cli admin directory-groups-list` — Retrieves all groups of a domain or of a user given a userKey (paginated).
- `workspace-admin-pp-cli admin directory-groups-patch` — Updates a group's properties. This method supports [patch semantics](/admin-sdk/directory/v1/guides/performance#patch).
- `workspace-admin-pp-cli admin directory-groups-update` — Updates a group's properties.
- `workspace-admin-pp-cli admin directory-members-delete` — Removes a member from a group.
- `workspace-admin-pp-cli admin directory-members-get` — Retrieves a group member's properties.
- `workspace-admin-pp-cli admin directory-members-has-member` — Checks whether the given user is a member of the group.
- `workspace-admin-pp-cli admin directory-members-insert` — Adds a user to the specified group.
- `workspace-admin-pp-cli admin directory-members-list` — Retrieves a paginated list of all members in a group. This method times out after 60 minutes.
- `workspace-admin-pp-cli admin directory-members-patch` — Updates the membership properties of a user in the specified group.
- `workspace-admin-pp-cli admin directory-members-update` — Updates the membership of a user in the specified group.
- `workspace-admin-pp-cli admin directory-mobiledevices-action` — Takes an action that affects a mobile device. For example, remotely wiping a device.
- `workspace-admin-pp-cli admin directory-mobiledevices-delete` — Removes a mobile device.
- `workspace-admin-pp-cli admin directory-mobiledevices-get` — Retrieves a mobile device's properties.
- `workspace-admin-pp-cli admin directory-mobiledevices-list` — Retrieves a paginated list of all user-owned mobile devices for an account.
- `workspace-admin-pp-cli admin directory-orgunits-delete` — Removes an organizational unit.
- `workspace-admin-pp-cli admin directory-orgunits-get` — Retrieves an organizational unit.
- `workspace-admin-pp-cli admin directory-orgunits-insert` — Adds an organizational unit.
- `workspace-admin-pp-cli admin directory-orgunits-list` — Retrieves a list of all organizational units for an account.
- `workspace-admin-pp-cli admin directory-orgunits-patch` — Updates an organizational unit. This method supports [patch semantics](/admin-sdk/directory/v1/guides/performance#patch)
- `workspace-admin-pp-cli admin directory-orgunits-update` — Updates an organizational unit.
- `workspace-admin-pp-cli admin directory-privileges-list` — Retrieves a paginated list of all privileges for a customer.
- `workspace-admin-pp-cli admin directory-resources-buildings-delete` — Deletes a building.
- `workspace-admin-pp-cli admin directory-resources-buildings-get` — Retrieves a building.
- `workspace-admin-pp-cli admin directory-resources-buildings-insert` — Inserts a building.
- `workspace-admin-pp-cli admin directory-resources-buildings-list` — Retrieves a list of buildings for an account.
- `workspace-admin-pp-cli admin directory-resources-buildings-patch` — Patches a building.
- `workspace-admin-pp-cli admin directory-resources-buildings-update` — Updates a building.
- `workspace-admin-pp-cli admin directory-resources-calendars-delete` — Deletes a calendar resource.
- `workspace-admin-pp-cli admin directory-resources-calendars-get` — Retrieves a calendar resource.
- `workspace-admin-pp-cli admin directory-resources-calendars-insert` — Inserts a calendar resource.
- `workspace-admin-pp-cli admin directory-resources-calendars-list` — Retrieves a list of calendar resources for an account.
- `workspace-admin-pp-cli admin directory-resources-calendars-patch` — Patches a calendar resource.
- `workspace-admin-pp-cli admin directory-resources-calendars-update` — Updates a calendar resource.
- `workspace-admin-pp-cli admin directory-resources-features-delete` — Deletes a feature.
- `workspace-admin-pp-cli admin directory-resources-features-get` — Retrieves a feature.
- `workspace-admin-pp-cli admin directory-resources-features-insert` — Inserts a feature.
- `workspace-admin-pp-cli admin directory-resources-features-list` — Retrieves a list of features for an account.
- `workspace-admin-pp-cli admin directory-resources-features-patch` — Patches a feature.
- `workspace-admin-pp-cli admin directory-resources-features-rename` — Renames a feature.
- `workspace-admin-pp-cli admin directory-resources-features-update` — Updates a feature.
- `workspace-admin-pp-cli admin directory-role-assignments-delete` — Deletes a role assignment.
- `workspace-admin-pp-cli admin directory-role-assignments-get` — Retrieves a role assignment.
- `workspace-admin-pp-cli admin directory-role-assignments-insert` — Creates a role assignment.
- `workspace-admin-pp-cli admin directory-role-assignments-list` — Retrieves a paginated list of all roleAssignments.
- `workspace-admin-pp-cli admin directory-roles-delete` — Deletes a role.
- `workspace-admin-pp-cli admin directory-roles-get` — Retrieves a role.
- `workspace-admin-pp-cli admin directory-roles-insert` — Creates a role.
- `workspace-admin-pp-cli admin directory-roles-list` — Retrieves a paginated list of all the roles in a domain.
- `workspace-admin-pp-cli admin directory-roles-patch` — Patches a role.
- `workspace-admin-pp-cli admin directory-roles-update` — Updates a role.
- `workspace-admin-pp-cli admin directory-schemas-delete` — Deletes a schema.
- `workspace-admin-pp-cli admin directory-schemas-get` — Retrieves a schema.
- `workspace-admin-pp-cli admin directory-schemas-insert` — Creates a schema.
- `workspace-admin-pp-cli admin directory-schemas-list` — Retrieves all schemas for a customer.
- `workspace-admin-pp-cli admin directory-schemas-patch` — Patches a schema.
- `workspace-admin-pp-cli admin directory-schemas-update` — Updates a schema.
- `workspace-admin-pp-cli admin directory-tokens-delete` — Deletes all access tokens issued by a user for an application.
- `workspace-admin-pp-cli admin directory-tokens-get` — Gets information about an access token issued by a user.
- `workspace-admin-pp-cli admin directory-tokens-list` — Returns the set of tokens specified user has issued to 3rd party applications.
- `workspace-admin-pp-cli admin directory-two-step-verification-turn-off` — Turns off 2-Step Verification for user.
- `workspace-admin-pp-cli admin directory-users-aliases-delete` — Removes an alias.
- `workspace-admin-pp-cli admin directory-users-aliases-insert` — Adds an alias.
- `workspace-admin-pp-cli admin directory-users-aliases-list` — Lists all aliases for a user.
- `workspace-admin-pp-cli admin directory-users-aliases-watch` — Watches for changes in users list.
- `workspace-admin-pp-cli admin directory-users-delete` — Deletes a user.
- `workspace-admin-pp-cli admin directory-users-get` — Retrieves a user.
- `workspace-admin-pp-cli admin directory-users-insert` — Creates a user.
- `workspace-admin-pp-cli admin directory-users-list` — Retrieves a paginated list of either deleted users or all users in a domain.
- `workspace-admin-pp-cli admin directory-users-make` — Makes a user a super administrator.
- `workspace-admin-pp-cli admin directory-users-patch` — Updates a user using patch semantics.
- `workspace-admin-pp-cli admin directory-users-photos-delete` — Removes the user's photo.
- `workspace-admin-pp-cli admin directory-users-photos-get` — Retrieves the user's photo.
- `workspace-admin-pp-cli admin directory-users-photos-patch` — Adds a photo for the user. This method supports [patch semantics](/admin-sdk/directory/v1/guides/performance#patch).
- `workspace-admin-pp-cli admin directory-users-photos-update` — Adds a photo for the user.
- `workspace-admin-pp-cli admin directory-users-sign-out` — Signs a user out of all web and device sessions and reset their sign-in cookies.
- `workspace-admin-pp-cli admin directory-users-undelete` — Undeletes a deleted user.
- `workspace-admin-pp-cli admin directory-users-update` — Updates a user.
- `workspace-admin-pp-cli admin directory-users-watch` — Watches for changes in users list.
- `workspace-admin-pp-cli admin directory-verification-codes-generate` — Generates new backup verification codes for the user.
- `workspace-admin-pp-cli admin directory-verification-codes-invalidate` — Invalidates the current backup verification codes for the user.
- `workspace-admin-pp-cli admin directory-verification-codes-list` — Returns the current set of valid backup verification codes for the specified user.

**admin-sdk-admin** — Manage admin sdk admin

- `workspace-admin-pp-cli admin-sdk-admin channels-stop` — Stop watching resources through this channel.
- `workspace-admin-pp-cli admin-sdk-admin reports-activities-list` — Retrieves a list of activities for a specific customer's account and application such as the Admin console application
- `workspace-admin-pp-cli admin-sdk-admin reports-activities-watch` — Start receiving notifications for account activities. For more information, see Receiving Push Notifications.
- `workspace-admin-pp-cli admin-sdk-admin reports-customer-usage-reports-get` — Retrieves a report which is a collection of properties and statistics for a specific customer's account.
- `workspace-admin-pp-cli admin-sdk-admin reports-entity-usage-reports-get` — Retrieves a report which is a collection of properties and statistics for entities used by users within the account.
- `workspace-admin-pp-cli admin-sdk-admin reports-user-usage-report-get` — Retrieves a report which is a collection of properties and statistics for a set of users with the account.

**alerts** — Manage alerts

- `workspace-admin-pp-cli alerts delete` — Marks the specified alert for deletion.
- `workspace-admin-pp-cli alerts get` — Gets the specified alert. Attempting to get a nonexistent alert returns `NOT_FOUND` error.
- `workspace-admin-pp-cli alerts list` — Lists the alerts.
- `workspace-admin-pp-cli alerts undelete` — Restores, or 'undeletes', an alert that was marked for deletion within the past 30 days.

**alerts-batch-delete** — Manage alerts batch delete

- `workspace-admin-pp-cli alerts-batch-delete` — Performs batch delete operation on alerts.

**alerts-batch-undelete** — Manage alerts batch undelete

- `workspace-admin-pp-cli alerts-batch-undelete` — Performs batch undelete operation on alerts.

**changes** — Manage changes

- `workspace-admin-pp-cli changes get-start-page-token` — Gets the starting pageToken for listing future changes.
- `workspace-admin-pp-cli changes list` — Lists the changes for a user or shared drive.
- `workspace-admin-pp-cli changes watch` — Subscribes to changes for a user. To use this method, you must include the pageToken query parameter.

**channels** — Manage channels

- `workspace-admin-pp-cli channels` — Stop watching resources through this channel

**drafts** — Manage drafts

- `workspace-admin-pp-cli drafts create` — Creates a new draft with the `DRAFT` label.
- `workspace-admin-pp-cli drafts delete` — Immediately and permanently deletes the specified draft. Does not simply trash it.
- `workspace-admin-pp-cli drafts get` — Gets the specified draft.
- `workspace-admin-pp-cli drafts list` — Lists the drafts in the user's mailbox.
- `workspace-admin-pp-cli drafts send` — Sends the specified, existing draft to the recipients in the `To`, `Cc`, and `Bcc` headers.
- `workspace-admin-pp-cli drafts update` — Replaces a draft's content.

**drive-about** — Manage drive about

- `workspace-admin-pp-cli drive-about` — Gets information about the user, the user's Drive, and system capabilities.

**drives** — Manage drives

- `workspace-admin-pp-cli drives create` — Creates a shared drive.
- `workspace-admin-pp-cli drives delete` — Permanently deletes a shared drive for which the user is an organizer.
- `workspace-admin-pp-cli drives get` — Gets a shared drive's metadata by ID.
- `workspace-admin-pp-cli drives list` — Lists the user's shared drives.
- `workspace-admin-pp-cli drives update` — Updates the metadata for a shared drive.

**files** — Manage files

- `workspace-admin-pp-cli files create` — Creates a file.
- `workspace-admin-pp-cli files delete` — Permanently deletes a file owned by the user without moving it to the trash.
- `workspace-admin-pp-cli files empty-trash` — Permanently deletes all of the user's trashed files.
- `workspace-admin-pp-cli files generate-ids` — Generates a set of file IDs which can be provided in create or copy requests.
- `workspace-admin-pp-cli files get` — Gets a file's metadata or content by ID.
- `workspace-admin-pp-cli files list` — Lists or searches files.
- `workspace-admin-pp-cli files update` — Updates a file's metadata and/or content.

**gmail-settings** — Manage gmail settings

- `workspace-admin-pp-cli gmail-settings create` — Adds a delegate with its verification status set directly to `accepted`, without sending any verification email.
- `workspace-admin-pp-cli gmail-settings create-gmail` — Creates a filter. Note: you can only create a maximum of 1,000 filters.
- `workspace-admin-pp-cli gmail-settings create-gmail-2` — Creates a forwarding address.
- `workspace-admin-pp-cli gmail-settings create-gmail-3` — Creates a custom 'from' send-as alias.
- `workspace-admin-pp-cli gmail-settings create-gmail-4` — Creates and configures a client-side encryption identity that's authorized to send mail from the user account.
- `workspace-admin-pp-cli gmail-settings create-gmail-5` — Creates and uploads a client-side encryption S/MIME public key certificate chain and private key metadata for the
- `workspace-admin-pp-cli gmail-settings delete` — Removes the specified delegate (which can be of any verification status)
- `workspace-admin-pp-cli gmail-settings delete-gmail` — Immediately and permanently deletes the specified filter.
- `workspace-admin-pp-cli gmail-settings delete-gmail-2` — Deletes the specified forwarding address and revokes any verification that may have been required.
- `workspace-admin-pp-cli gmail-settings delete-gmail-3` — Deletes the specified send-as alias. Revokes any verification that may have been required for using it.
- `workspace-admin-pp-cli gmail-settings delete-gmail-4` — Deletes a client-side encryption identity.
- `workspace-admin-pp-cli gmail-settings delete-gmail-5` — Deletes the specified S/MIME config for the specified send-as alias.
- `workspace-admin-pp-cli gmail-settings disable` — Turns off a client-side encryption key pair.
- `workspace-admin-pp-cli gmail-settings enable` — Turns on a client-side encryption key pair that was turned off.
- `workspace-admin-pp-cli gmail-settings get` — Gets the specified delegate.
- `workspace-admin-pp-cli gmail-settings get-auto-forwarding` — Gets the auto-forwarding setting for the specified account.
- `workspace-admin-pp-cli gmail-settings get-gmail` — Gets a filter.
- `workspace-admin-pp-cli gmail-settings get-gmail-2` — Gets the specified forwarding address.
- `workspace-admin-pp-cli gmail-settings get-gmail-3` — Gets the specified send-as alias.
- `workspace-admin-pp-cli gmail-settings get-gmail-4` — Retrieves a client-side encryption identity configuration.
- `workspace-admin-pp-cli gmail-settings get-gmail-5` — Retrieves an existing client-side encryption key pair.
- `workspace-admin-pp-cli gmail-settings get-gmail-6` — Gets the specified S/MIME config for the specified send-as alias.
- `workspace-admin-pp-cli gmail-settings get-imap` — Gets IMAP settings.
- `workspace-admin-pp-cli gmail-settings get-language` — Gets language settings.
- `workspace-admin-pp-cli gmail-settings get-pop` — Gets POP settings.
- `workspace-admin-pp-cli gmail-settings get-vacation` — Gets vacation responder settings.
- `workspace-admin-pp-cli gmail-settings insert` — Insert (upload) the given S/MIME config for the specified send-as alias.
- `workspace-admin-pp-cli gmail-settings list` — Lists the delegates for the specified account.
- `workspace-admin-pp-cli gmail-settings list-gmail` — Lists the message filters of a Gmail user.
- `workspace-admin-pp-cli gmail-settings list-gmail-2` — Lists the forwarding addresses for the specified account.
- `workspace-admin-pp-cli gmail-settings list-gmail-3` — Lists the send-as aliases for the specified account.
- `workspace-admin-pp-cli gmail-settings list-gmail-4` — Lists the client-side encrypted identities for an authenticated user.
- `workspace-admin-pp-cli gmail-settings list-gmail-5` — Lists client-side encryption key pairs for an authenticated user.
- `workspace-admin-pp-cli gmail-settings list-gmail-6` — Lists S/MIME configs for the specified send-as alias.
- `workspace-admin-pp-cli gmail-settings obliterate` — Deletes a client-side encryption key pair permanently and immediately.
- `workspace-admin-pp-cli gmail-settings patch` — Patch the specified send-as alias.
- `workspace-admin-pp-cli gmail-settings patch-gmail` — Associates a different key pair with an existing client-side encryption identity.
- `workspace-admin-pp-cli gmail-settings set-default` — Sets the default S/MIME config for the specified send-as alias.
- `workspace-admin-pp-cli gmail-settings update` — Updates a send-as alias. If a signature is provided, Gmail will sanitize the HTML before saving it with the alias.
- `workspace-admin-pp-cli gmail-settings update-auto-forwarding` — Updates the auto-forwarding setting for the specified account.
- `workspace-admin-pp-cli gmail-settings update-imap` — Updates IMAP settings.
- `workspace-admin-pp-cli gmail-settings update-language` — Updates language settings.
- `workspace-admin-pp-cli gmail-settings update-pop` — Updates POP settings.
- `workspace-admin-pp-cli gmail-settings update-vacation` — Updates vacation responder settings.
- `workspace-admin-pp-cli gmail-settings verify` — Sends a verification email to the specified send-as alias address. The verification status must be `pending`.

**history** — Manage history

- `workspace-admin-pp-cli history <userId>` — Lists the history of all changes to the given mailbox.

**labels** — Manage labels

- `workspace-admin-pp-cli labels create` — Creates a new label.
- `workspace-admin-pp-cli labels delete` — Immediately and permanently deletes the specified label and removes it from any messages and threads that it is applied
- `workspace-admin-pp-cli labels get` — Gets the specified label.
- `workspace-admin-pp-cli labels list` — Lists all labels in the user's mailbox.
- `workspace-admin-pp-cli labels patch` — Patch the specified label.
- `workspace-admin-pp-cli labels update` — Updates the specified label.

**messages** — Manage messages

- `workspace-admin-pp-cli messages batch-delete` — Deletes many messages by message ID.
- `workspace-admin-pp-cli messages batch-modify` — Modifies the labels on the specified messages.
- `workspace-admin-pp-cli messages delete` — Immediately and permanently deletes the specified message. This operation cannot be undone. Prefer `messages.
- `workspace-admin-pp-cli messages get` — Gets the specified message.
- `workspace-admin-pp-cli messages import` — Imports a message into only this user's mailbox
- `workspace-admin-pp-cli messages insert` — Directly inserts a message into only this user's mailbox similar to `IMAP APPEND`
- `workspace-admin-pp-cli messages list` — Lists the messages in the user's mailbox.
- `workspace-admin-pp-cli messages send` — Sends the specified message to the recipients in the `To`, `Cc`, and `Bcc` headers.

**settings** — Manage settings

- `workspace-admin-pp-cli settings alertcenter-get` — Returns customer-level settings.
- `workspace-admin-pp-cli settings alertcenter-update` — Updates the customer-level settings.

**stop** — Manage stop

- `workspace-admin-pp-cli stop <userId>` — Stop receiving push notifications for the given user mailbox.

**teamdrives** — Manage teamdrives

- `workspace-admin-pp-cli teamdrives create` — Deprecated use drives.create instead.
- `workspace-admin-pp-cli teamdrives delete` — Deprecated use drives.delete instead.
- `workspace-admin-pp-cli teamdrives get` — Deprecated use drives.get instead.
- `workspace-admin-pp-cli teamdrives list` — Deprecated use drives.list instead.
- `workspace-admin-pp-cli teamdrives update` — Deprecated use drives.update instead

**threads** — Manage threads

- `workspace-admin-pp-cli threads delete` — Immediately and permanently deletes the specified thread. Any messages that belong to the thread are also deleted.
- `workspace-admin-pp-cli threads get` — Gets the specified thread.
- `workspace-admin-pp-cli threads list` — Lists the threads in the user's mailbox.

**users_profile** — Manage users profile

- `workspace-admin-pp-cli users-profile <userId>` — Gets the current user's Gmail profile.

**watch** — Manage watch

- `workspace-admin-pp-cli watch <userId>` — Set up or update a push notification watch on the given user mailbox.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
workspace-admin-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Preview a full offboard

```bash
workspace-admin-pp-cli workflow offboard departing@example.com --manager manager@example.com --dry-run
```

Shows every step (suspend, token revoke, Drive transfer, delegation, device wipe, group removal, OU move) without changing anything; drop --dry-run to execute.

### External exposure for one domain, narrowed output

```bash
workspace-admin-pp-cli audit external-shares --domain gmail.com --agent --select files.name,files.owner,files.permissions.emailAddress
```

Uses dotted --select paths to return only the fields that matter from a deeply nested Drive permissions response, keeping agent context small.

### High-risk OAuth apps

```bash
workspace-admin-pp-cli audit app-risk --min-risk high --agent
```

Lists third-party apps tiered High risk (including Full Drive access) with their authorizing users, for third-party app review.

### Reconstruct a compromised account

```bash
workspace-admin-pp-cli audit reconstruct user@example.com --since 7d --agent
```

Produces one ordered timeline of the user's login, admin, Drive, and token activity over the last 7 days.

### Search the local audit store

```bash
workspace-admin-pp-cli search "external" --type drive_files --json
```

Full-text search over synced Drive metadata after a sync, returning structured JSON.

## Auth Setup

Google Workspace admin APIs use OAuth2. This CLI accepts any Google access token as a bearer token via GOOGLE_WORKSPACE_TOKEN (for example from `gcloud auth print-access-token`). The first-class admin path is a service account with domain-wide delegation: `workspace-admin-pp-cli auth service-account --key sa.json --impersonate admin@example.com` mints and caches an access token by impersonating a super admin (required for Directory, Reports, and Alert Center). For Drive and Gmail data belonging to a specific user, impersonate that user with `--impersonate user@example.com`. Authorize the service account's scopes once in Admin Console > Security > API Controls > Domain-wide Delegation; run `doctor` to verify each scope.

Run `workspace-admin-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  workspace-admin-pp-cli alerts list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and use `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `WORKSPACE_ADMIN_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `WORKSPACE_ADMIN_CONFIG_DIR`, `WORKSPACE_ADMIN_DATA_DIR`, `WORKSPACE_ADMIN_STATE_DIR`, `WORKSPACE_ADMIN_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `WORKSPACE_ADMIN_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `workspace-admin-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "workspace-admin": {
        "command": "workspace-admin-pp-mcp",
        "env": {
          "WORKSPACE_ADMIN_HOME": "/srv/workspace-admin"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `WORKSPACE_ADMIN_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `WORKSPACE_ADMIN_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
workspace-admin-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
workspace-admin-pp-cli feedback --stdin < notes.txt
workspace-admin-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `WORKSPACE_ADMIN_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `WORKSPACE_ADMIN_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
workspace-admin-pp-cli profile save briefing --json
workspace-admin-pp-cli --profile briefing alerts list
workspace-admin-pp-cli profile list --json
workspace-admin-pp-cli profile show briefing
workspace-admin-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `workspace-admin-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/workspace-admin/cmd/workspace-admin-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add workspace-admin-pp-mcp -- workspace-admin-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which workspace-admin-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   workspace-admin-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `workspace-admin-pp-cli <command> --help`.
