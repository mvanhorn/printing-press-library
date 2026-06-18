# Affine CLI

AFFiNE self-hosted GraphQL API wrapper generated from qtz-affine schema.gql and client .gql documents.

Learn more at [Affine](https://affine.quartzo.ai).

Created by [@gabrieldasf](https://github.com/gabrieldasf) (gabrieldasf).

## Install

The recommended path installs both the `affine-pp-cli` binary and the `pp-affine` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install affine
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install affine --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install affine --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install affine --agent claude-code
npx -y @mvanhorn/printing-press-library install affine --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/affine/cmd/affine-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/affine-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install affine --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-affine --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-affine --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install affine --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/affine-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `AFFINE_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/affine/cmd/affine-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "affine": {
      "command": "affine-pp-mcp",
      "env": {
        "AFFINE_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your access token from your API provider's developer portal, then store it:

```bash
affine-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set it via environment variable:

```bash
export AFFINE_TOKEN="your-token-here"
```

### 3. Verify Setup

```bash
affine-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
affine-pp-cli admins admin-all-shared-links --operation-name example-resource
```


## Unique Features

These capabilities aren't available in any other tool for this API.

### Canvas operations
- **`canvas plan`** — Builds a deterministic AFFiNE canvas card layout from a compact JSON tree spec.

  _Use this before applying or reviewing a new canvas structure._

  ```bash
  affine-pp-cli canvas plan --json
  ```
- **`canvas model`** — Normalizes AFFiNE canvas JSON into explicit nodes and connectors.

  _Use this when comparing or auditing canvas structure._

  ```bash
  affine-pp-cli canvas model --json
  ```

### Canvas safety
- **`canvas validate`** — Checks planned canvas nodes and connectors for invalid geometry and orphaned links.

  _Use this before applying generated canvas plans._

  ```bash
  affine-pp-cli canvas validate --json
  ```
- **`canvas apply`** — Converts a canvas plan into explicit dry-run operations without enabling live Y.js writes.

  _Use this to inspect the exact operations a plan would perform._

  ```bash
  affine-pp-cli canvas apply --dry-run --json
  ```

### Canvas repair
- **`canvas card set-image`** — Patches an existing AFFiNE canvas card image/logo while preserving card identity and text.

  _Use this when a visual card asset needs to change without rebuilding the board._

  ```bash
  affine-pp-cli canvas card set-image --workspace <workspace-id> --doc <doc-id> --card <card-id> --source-id <blob-id> --alt LightRAG --dry-run --json
  ```
- **`canvas doc integrity`** — Checks AFFiNE canvas document snapshots for structural issues such as missing children maps.

  _Use this before repairing a canvas that renders incomplete content._

  ```bash
  affine-pp-cli canvas doc integrity --snapshot-file doc.bin --json
  ```
- **`canvas doc repair`** — Repairs a narrow set of AFFiNE canvas integrity issues with dry-run and backup controls.

  _Use this only after integrity diagnostics identify a supported repair._

  ```bash
  affine-pp-cli canvas doc repair --workspace <workspace-id> --doc <doc-id> --dry-run --json
  ```

## Usage

Run `affine-pp-cli --help` for the full command reference and flag list.

## Commands

### admins

AFFiNE admins GraphQL operations.

- **`affine-pp-cli admins admin-all-shared-links`** - Run AFFiNE query adminAllSharedLinks.
- **`affine-pp-cli admins admin-dashboard`** - Run AFFiNE query adminDashboard.
- **`affine-pp-cli admins admin-server-config`** - Run AFFiNE query adminServerConfig.
- **`affine-pp-cli admins admin-update-workspace`** - Run AFFiNE mutation adminUpdateWorkspace.
- **`affine-pp-cli admins admin-workspace`** - Run AFFiNE query adminWorkspace.
- **`affine-pp-cli admins admin-workspaces`** - Run AFFiNE query adminWorkspaces.
- **`affine-pp-cli admins admin-workspaces-count`** - Run AFFiNE query adminWorkspacesCount.

### blobs

AFFiNE blobs GraphQL operations.

- **`affine-pp-cli blobs abort-blob-upload`** - Run AFFiNE mutation abortBlobUpload.
- **`affine-pp-cli blobs complete-blob-upload`** - Run AFFiNE mutation completeBlobUpload.
- **`affine-pp-cli blobs create-blob-upload`** - Run AFFiNE mutation createBlobUpload.
- **`affine-pp-cli blobs delete-blob`** - Run AFFiNE mutation deleteBlob.
- **`affine-pp-cli blobs get-blob-upload-part-url`** - Run AFFiNE query getBlobUploadPartUrl.
- **`affine-pp-cli blobs list-blobs`** - Run AFFiNE query listBlobs.
- **`affine-pp-cli blobs release-deleted-blobs`** - Run AFFiNE mutation releaseDeletedBlobs.
- **`affine-pp-cli blobs set-blob`** - Run AFFiNE mutation setBlob.

### comments

AFFiNE comments GraphQL operations.

- **`affine-pp-cli comments create-comment`** - Run AFFiNE mutation createComment.
- **`affine-pp-cli comments create-reply`** - Run AFFiNE mutation createReply.
- **`affine-pp-cli comments delete-comment`** - Run AFFiNE mutation deleteComment.
- **`affine-pp-cli comments delete-reply`** - Run AFFiNE mutation deleteReply.
- **`affine-pp-cli comments list-comment-changes`** - Run AFFiNE query listCommentChanges.
- **`affine-pp-cli comments list-comments`** - Run AFFiNE query listComments.
- **`affine-pp-cli comments resolve-comment`** - Run AFFiNE mutation resolveComment.
- **`affine-pp-cli comments update-comment`** - Run AFFiNE mutation updateComment.
- **`affine-pp-cli comments update-reply`** - Run AFFiNE mutation updateReply.
- **`affine-pp-cli comments upload-comment-attachment`** - Run AFFiNE mutation uploadCommentAttachment.

### copilots

AFFiNE copilots GraphQL operations.

- **`affine-pp-cli copilots add-context-blob`** - Run AFFiNE mutation addContextBlob.
- **`affine-pp-cli copilots add-context-category`** - Run AFFiNE mutation addContextCategory.
- **`affine-pp-cli copilots add-context-doc`** - Run AFFiNE mutation addContextDoc.
- **`affine-pp-cli copilots add-context-file`** - Run AFFiNE mutation addContextFile.
- **`affine-pp-cli copilots add-workspace-embedding-files`** - Run AFFiNE mutation addWorkspaceEmbeddingFiles.
- **`affine-pp-cli copilots add-workspace-embedding-ignored-docs`** - Run AFFiNE mutation addWorkspaceEmbeddingIgnoredDocs.
- **`affine-pp-cli copilots cleanup-copilot-session`** - Run AFFiNE mutation cleanupCopilotSession.
- **`affine-pp-cli copilots copilot-quota`** - Run AFFiNE query copilotQuota.
- **`affine-pp-cli copilots create-copilot-context`** - Run AFFiNE mutation createCopilotContext.
- **`affine-pp-cli copilots create-copilot-message`** - Run AFFiNE mutation createCopilotMessage.
- **`affine-pp-cli copilots create-copilot-session`** - Run AFFiNE mutation createCopilotSession.
- **`affine-pp-cli copilots create-copilot-session-with-history`** - Run AFFiNE mutation createCopilotSessionWithHistory.
- **`affine-pp-cli copilots fork-copilot-session`** - Run AFFiNE mutation forkCopilotSession.
- **`affine-pp-cli copilots get-all-workspace-embedding-ignored-docs`** - Run AFFiNE query getAllWorkspaceEmbeddingIgnoredDocs.
- **`affine-pp-cli copilots get-copilot-doc-sessions`** - Run AFFiNE query getCopilotDocSessions.
- **`affine-pp-cli copilots get-copilot-histories`** - Run AFFiNE query getCopilotHistories.
- **`affine-pp-cli copilots get-copilot-history-ids`** - Run AFFiNE query getCopilotHistoryIds.
- **`affine-pp-cli copilots get-copilot-latest-doc-session`** - Run AFFiNE query getCopilotLatestDocSession.
- **`affine-pp-cli copilots get-copilot-pinned-sessions`** - Run AFFiNE query getCopilotPinnedSessions.
- **`affine-pp-cli copilots get-copilot-recent-sessions`** - Run AFFiNE query getCopilotRecentSessions.
- **`affine-pp-cli copilots get-copilot-session`** - Run AFFiNE query getCopilotSession.
- **`affine-pp-cli copilots get-copilot-sessions`** - Run AFFiNE query getCopilotSessions.
- **`affine-pp-cli copilots get-copilot-workspace-sessions`** - Run AFFiNE query getCopilotWorkspaceSessions.
- **`affine-pp-cli copilots get-prompt-models`** - Run AFFiNE query getPromptModels.
- **`affine-pp-cli copilots get-transcript-task`** - Run AFFiNE query getTranscriptTask.
- **`affine-pp-cli copilots get-workspace-embedding-files`** - Run AFFiNE query getWorkspaceEmbeddingFiles.
- **`affine-pp-cli copilots get-workspace-embedding-ignored-docs`** - Run AFFiNE query getWorkspaceEmbeddingIgnoredDocs.
- **`affine-pp-cli copilots list-context`** - Run AFFiNE query listContext.
- **`affine-pp-cli copilots list-context-object`** - Run AFFiNE query listContextObject.
- **`affine-pp-cli copilots match-context`** - Run AFFiNE query matchContext.
- **`affine-pp-cli copilots match-files`** - Run AFFiNE query matchFiles.
- **`affine-pp-cli copilots match-workspace-docs`** - Run AFFiNE query matchWorkspaceDocs.
- **`affine-pp-cli copilots queue-workspace-embedding`** - Run AFFiNE mutation queueWorkspaceEmbedding.
- **`affine-pp-cli copilots remove-context-blob`** - Run AFFiNE mutation removeContextBlob.
- **`affine-pp-cli copilots remove-context-category`** - Run AFFiNE mutation removeContextCategory.
- **`affine-pp-cli copilots remove-context-doc`** - Run AFFiNE mutation removeContextDoc.
- **`affine-pp-cli copilots remove-context-file`** - Run AFFiNE mutation removeContextFile.
- **`affine-pp-cli copilots remove-workspace-embedding-files`** - Run AFFiNE mutation removeWorkspaceEmbeddingFiles.
- **`affine-pp-cli copilots remove-workspace-embedding-ignored-docs`** - Run AFFiNE mutation removeWorkspaceEmbeddingIgnoredDocs.
- **`affine-pp-cli copilots retry-transcript-task`** - Run AFFiNE mutation retryTranscriptTask.
- **`affine-pp-cli copilots settle-transcript-task`** - Run AFFiNE mutation settleTranscriptTask.
- **`affine-pp-cli copilots submit-transcript-task`** - Run AFFiNE mutation submitTranscriptTask.
- **`affine-pp-cli copilots update-copilot-session`** - Run AFFiNE mutation updateCopilotSession.

### docs

AFFiNE docs GraphQL operations.

- **`affine-pp-cli docs activate-license`** - Run AFFiNE mutation activateLicense.
- **`affine-pp-cli docs app-config`** - Run AFFiNE query appConfig.
- **`affine-pp-cli docs calendar-accounts`** - Run AFFiNE query calendarAccounts.
- **`affine-pp-cli docs calendar-providers`** - Run AFFiNE query calendarProviders.
- **`affine-pp-cli docs cancel-subscription`** - Run AFFiNE mutation cancelSubscription.
- **`affine-pp-cli docs change-email`** - Run AFFiNE mutation changeEmail.
- **`affine-pp-cli docs change-password`** - Run AFFiNE mutation changePassword.
- **`affine-pp-cli docs create-change-password-url`** - Run AFFiNE mutation createChangePasswordUrl.
- **`affine-pp-cli docs create-checkout-session`** - Run AFFiNE mutation createCheckoutSession.
- **`affine-pp-cli docs create-customer-portal`** - Run AFFiNE mutation createCustomerPortal.
- **`affine-pp-cli docs create-selfhost-customer-portal`** - Run AFFiNE mutation createSelfhostCustomerPortal.
- **`affine-pp-cli docs create-user`** - Run AFFiNE mutation createUser.
- **`affine-pp-cli docs create-workspace`** - Run AFFiNE mutation createWorkspace.
- **`affine-pp-cli docs deactivate-license`** - Run AFFiNE mutation deactivateLicense.
- **`affine-pp-cli docs delete-account`** - Run AFFiNE mutation deleteAccount.
- **`affine-pp-cli docs delete-user`** - Run AFFiNE mutation deleteUser.
- **`affine-pp-cli docs delete-workspace`** - Run AFFiNE mutation deleteWorkspace.
- **`affine-pp-cli docs disable-user`** - Run AFFiNE mutation disableUser.
- **`affine-pp-cli docs enable-user`** - Run AFFiNE mutation enableUser.
- **`affine-pp-cli docs generate-license-key`** - Run AFFiNE mutation generateLicenseKey.
- **`affine-pp-cli docs generate-user-access-token`** - Run AFFiNE mutation generateUserAccessToken.
- **`affine-pp-cli docs get-current-user`** - Run AFFiNE query getCurrentUser.
- **`affine-pp-cli docs get-current-user-features`** - Run AFFiNE query getCurrentUserFeatures.
- **`affine-pp-cli docs get-doc-role-permissions`** - Run AFFiNE query getDocRolePermissions.
- **`affine-pp-cli docs get-invite-info`** - Run AFFiNE query getInviteInfo.
- **`affine-pp-cli docs get-invoices-count`** - Run AFFiNE query getInvoicesCount.
- **`affine-pp-cli docs get-public-user-by-id`** - Run AFFiNE query getPublicUserById.
- **`affine-pp-cli docs get-user-features`** - Run AFFiNE query getUserFeatures.
- **`affine-pp-cli docs get-user-settings`** - Run AFFiNE query getUserSettings.
- **`affine-pp-cli docs get-workspaces`** - Run AFFiNE query getWorkspaces.
- **`affine-pp-cli docs grant-doc-user-roles`** - Run AFFiNE mutation grantDocUserRoles.
- **`affine-pp-cli docs import-users`** - Run AFFiNE mutation ImportUsers.
- **`affine-pp-cli docs install-license`** - Run AFFiNE mutation installLicense.
- **`affine-pp-cli docs invoices`** - Run AFFiNE query invoices.
- **`affine-pp-cli docs leave-workspace`** - Run AFFiNE mutation leaveWorkspace.
- **`affine-pp-cli docs link-cal-dav-account`** - Run AFFiNE mutation linkCalDavAccount.
- **`affine-pp-cli docs link-calendar-account`** - Run AFFiNE mutation linkCalendarAccount.
- **`affine-pp-cli docs list-notifications`** - Run AFFiNE query listNotifications.
- **`affine-pp-cli docs list-users`** - Run AFFiNE query listUsers.
- **`affine-pp-cli docs mention-user`** - Run AFFiNE mutation mentionUser.
- **`affine-pp-cli docs oauth-providers`** - Run AFFiNE query oauthProviders.
- **`affine-pp-cli docs preview-license`** - Run AFFiNE mutation previewLicense.
- **`affine-pp-cli docs prices`** - Run AFFiNE query prices.
- **`affine-pp-cli docs publish-page`** - Run AFFiNE mutation publishPage.
- **`affine-pp-cli docs quota`** - Run AFFiNE query quota.
- **`affine-pp-cli docs read-all-notifications`** - Run AFFiNE mutation readAllNotifications.
- **`affine-pp-cli docs read-notification`** - Run AFFiNE mutation readNotification.
- **`affine-pp-cli docs recover-doc`** - Run AFFiNE mutation recoverDoc.
- **`affine-pp-cli docs remove-avatar`** - Run AFFiNE mutation removeAvatar.
- **`affine-pp-cli docs resume-subscription`** - Run AFFiNE mutation resumeSubscription.
- **`affine-pp-cli docs revoke-doc-user-roles`** - Run AFFiNE mutation revokeDocUserRoles.
- **`affine-pp-cli docs revoke-member-permission`** - Run AFFiNE mutation revokeMemberPermission.
- **`affine-pp-cli docs revoke-public-page`** - Run AFFiNE mutation revokePublicPage.
- **`affine-pp-cli docs revoke-user-access-token`** - Run AFFiNE mutation revokeUserAccessToken.
- **`affine-pp-cli docs send-change-email`** - Run AFFiNE mutation sendChangeEmail.
- **`affine-pp-cli docs send-change-password-email`** - Run AFFiNE mutation sendChangePasswordEmail.
- **`affine-pp-cli docs send-set-password-email`** - Run AFFiNE mutation sendSetPasswordEmail.
- **`affine-pp-cli docs send-test-email`** - Run AFFiNE mutation sendTestEmail.
- **`affine-pp-cli docs send-verify-change-email`** - Run AFFiNE mutation sendVerifyChangeEmail.
- **`affine-pp-cli docs send-verify-email`** - Run AFFiNE mutation sendVerifyEmail.
- **`affine-pp-cli docs server-config`** - Run AFFiNE query serverConfig.
- **`affine-pp-cli docs set-workspace-public-by-id`** - Run AFFiNE mutation setWorkspacePublicById.
- **`affine-pp-cli docs unlink-calendar-account`** - Run AFFiNE mutation unlinkCalendarAccount.
- **`affine-pp-cli docs update-account`** - Run AFFiNE mutation updateAccount.
- **`affine-pp-cli docs update-account-features`** - Run AFFiNE mutation updateAccountFeatures.
- **`affine-pp-cli docs update-app-config`** - Run AFFiNE mutation updateAppConfig.
- **`affine-pp-cli docs update-calendar-account`** - Run AFFiNE mutation updateCalendarAccount.
- **`affine-pp-cli docs update-doc-default-role`** - Run AFFiNE mutation updateDocDefaultRole.
- **`affine-pp-cli docs update-doc-user-role`** - Run AFFiNE mutation updateDocUserRole.
- **`affine-pp-cli docs update-subscription`** - Run AFFiNE mutation updateSubscription.
- **`affine-pp-cli docs update-user-profile`** - Run AFFiNE mutation updateUserProfile.
- **`affine-pp-cli docs update-user-settings`** - Run AFFiNE mutation updateUserSettings.
- **`affine-pp-cli docs update-workspace-calendars`** - Run AFFiNE mutation updateWorkspaceCalendars.
- **`affine-pp-cli docs upload-avatar`** - Run AFFiNE mutation uploadAvatar.
- **`affine-pp-cli docs validate-config`** - Run AFFiNE query validateConfig.
- **`affine-pp-cli docs verify-email`** - Run AFFiNE mutation verifyEmail.

### graphql

Raw AFFiNE GraphQL operations.

- **`affine-pp-cli graphql`** - Execute any AFFiNE GraphQL document.

### subscriptions

AFFiNE subscriptions GraphQL operations.

- **`affine-pp-cli subscriptions refresh-subscription`** - Run AFFiNE mutation refreshSubscription.
- **`affine-pp-cli subscriptions request-apply-subscription`** - Run AFFiNE mutation requestApplySubscription.
- **`affine-pp-cli subscriptions subscription`** - Run AFFiNE query subscription.

### users

AFFiNE users GraphQL operations.

- **`affine-pp-cli users get-user`** - Run AFFiNE query getUser.
- **`affine-pp-cli users get-user-by-email`** - Run AFFiNE query getUserByEmail.

### workspaces

AFFiNE workspaces GraphQL operations.

- **`affine-pp-cli workspaces accept-invite-by-invite-id`** - Run AFFiNE mutation acceptInviteByInviteId.
- **`affine-pp-cli workspaces approve-workspace-team-member`** - Run AFFiNE mutation approveWorkspaceTeamMember.
- **`affine-pp-cli workspaces calendar-events`** - Run AFFiNE query calendarEvents.
- **`affine-pp-cli workspaces clear-workspace-byok-configs`** - Run AFFiNE mutation clearWorkspaceByokConfigs.
- **`affine-pp-cli workspaces create-invite-link`** - Run AFFiNE mutation createInviteLink.
- **`affine-pp-cli workspaces create-workspace-byok-local-lease`** - Run AFFiNE mutation createWorkspaceByokLocalLease.
- **`affine-pp-cli workspaces delete-workspace-byok-config`** - Run AFFiNE mutation deleteWorkspaceByokConfig.
- **`affine-pp-cli workspaces get-doc-created-by-updated-by-list`** - Run AFFiNE query getDocCreatedByUpdatedByList.
- **`affine-pp-cli workspaces get-doc-last-accessed-members`** - Run AFFiNE query getDocLastAccessedMembers.
- **`affine-pp-cli workspaces get-doc-page-analytics`** - Run AFFiNE query getDocPageAnalytics.
- **`affine-pp-cli workspaces get-doc-summary`** - Run AFFiNE query getDocSummary.
- **`affine-pp-cli workspaces get-license`** - Run AFFiNE query getLicense.
- **`affine-pp-cli workspaces get-member-count-by-workspace-id`** - Run AFFiNE query getMemberCountByWorkspaceId.
- **`affine-pp-cli workspaces get-recently-updated-docs`** - Run AFFiNE query getRecentlyUpdatedDocs.
- **`affine-pp-cli workspaces get-workspace`** - Run AFFiNE query getWorkspace.
- **`affine-pp-cli workspaces get-workspace-page-by-id`** - Run AFFiNE query getWorkspacePageById.
- **`affine-pp-cli workspaces get-workspace-page-meta-by-id`** - Run AFFiNE query getWorkspacePageMetaById.
- **`affine-pp-cli workspaces get-workspace-public-by-id`** - Run AFFiNE query getWorkspacePublicById.
- **`affine-pp-cli workspaces get-workspace-public-pages`** - Run AFFiNE query getWorkspacePublicPages.
- **`affine-pp-cli workspaces get-workspace-role-permissions`** - Run AFFiNE query getWorkspaceRolePermissions.
- **`affine-pp-cli workspaces get-workspace-subscription`** - Run AFFiNE query getWorkspaceSubscription.
- **`affine-pp-cli workspaces grant-workspace-team-member`** - Run AFFiNE mutation grantWorkspaceTeamMember.
- **`affine-pp-cli workspaces indexer-aggregate`** - Run AFFiNE query indexerAggregate.
- **`affine-pp-cli workspaces indexer-search`** - Run AFFiNE query indexerSearch.
- **`affine-pp-cli workspaces indexer-search-docs`** - Run AFFiNE query indexerSearchDocs.
- **`affine-pp-cli workspaces invite-by-emails`** - Run AFFiNE mutation inviteByEmails.
- **`affine-pp-cli workspaces list-history`** - Run AFFiNE query listHistory.
- **`affine-pp-cli workspaces reorder-workspace-byok-configs`** - Run AFFiNE mutation reorderWorkspaceByokConfigs.
- **`affine-pp-cli workspaces revoke-invite-link`** - Run AFFiNE mutation revokeInviteLink.
- **`affine-pp-cli workspaces set-enable-ai`** - Run AFFiNE mutation setEnableAi.
- **`affine-pp-cli workspaces set-enable-doc-embedding`** - Run AFFiNE mutation setEnableDocEmbedding.
- **`affine-pp-cli workspaces set-enable-sharing`** - Run AFFiNE mutation setEnableSharing.
- **`affine-pp-cli workspaces set-enable-url-preview`** - Run AFFiNE mutation setEnableUrlPreview.
- **`affine-pp-cli workspaces test-workspace-byok-config`** - Run AFFiNE mutation testWorkspaceByokConfig.
- **`affine-pp-cli workspaces upsert-workspace-byok-config`** - Run AFFiNE mutation upsertWorkspaceByokConfig.
- **`affine-pp-cli workspaces workspace-blob-quota`** - Run AFFiNE query workspaceBlobQuota.
- **`affine-pp-cli workspaces workspace-byok-settings`** - Run AFFiNE query workspaceByokSettings.
- **`affine-pp-cli workspaces workspace-calendars`** - Run AFFiNE query workspaceCalendars.
- **`affine-pp-cli workspaces workspace-invoices`** - Run AFFiNE query workspaceInvoices.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
affine-pp-cli admins admin-all-shared-links --operation-name example-resource

# JSON for scripting and agents
affine-pp-cli admins admin-all-shared-links --operation-name example-resource --json

# Filter to specific fields
affine-pp-cli admins admin-all-shared-links --operation-name example-resource --json --select id,name,status

# Dry run — show the request without sending
affine-pp-cli admins admin-all-shared-links --operation-name example-resource --dry-run

# Agent mode — JSON + compact + no prompts in one flag
affine-pp-cli admins admin-all-shared-links --operation-name example-resource --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
affine-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/affine-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `AFFINE_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `affine-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `affine-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $AFFINE_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
