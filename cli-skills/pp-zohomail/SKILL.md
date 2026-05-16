---
name: pp-zohomail
description: "Use when working with Zoho Mail from an agent through the `zohomail-pp-cli` command. Trigger for Zoho Mail account setup, OAuth/self-client token exchange, saving local defaults, listing inbox/sent/spam/trash/archive folders, searching or reading messages, and sending mail only when explicitly requested."
author: "Jacques Wainwright"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - zohomail-pp-cli
    install:
      - kind: go
        bins: [zohomail-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/productivity/zohomail/cmd/zohomail-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/productivity/zohomail/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# Zoho Mail — Printing Press CLI

Use `zohomail-pp-cli` for Zoho Mail account, folder, message, search, read, and send operations.
Prefer configured defaults over copying account/folder IDs into every command.

## Prerequisites: Install the CLI

This skill drives the `zohomail-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install zohomail --cli-only
   ```
2. Verify: `zohomail-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/zohomail/cmd/zohomail-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## Auth

Prefer refresh-token auth so agents do not need repeated short-lived token pastes:

```bash
export ZOHO_CLIENT_ID='...'
export ZOHO_CLIENT_SECRET='...'
export ZOHO_REFRESH_TOKEN='...'
```

For one-off use, an access token also works:

```bash
export ZOHO_MAIL_ACCESS_TOKEN='...'
```

Check visible config without printing secrets:

```bash
zohomail-pp-cli doctor
```

Preferred new-device login:

```bash
zohomail-pp-cli client-setup
zohomail-pp-cli login --client-id '1000....' --client-secret '...'
```

`client-setup` opens the Zoho API Console and prints the required redirect URI/scopes. `login` opens browser OAuth, listens on `http://localhost:53682/callback`, saves client credentials + refresh auth, and discovers account/folder defaults. The Zoho API Console client must have that exact redirect URI. After credentials are saved, future `zohomail-pp-cli login` runs need no `--client-id` or `--client-secret`. In remote shells, use `zohomail-pp-cli login --no-open` and open the printed URL manually.

If a valid refresh token already exists, skip browser login and store it once:

```bash
zohomail-pp-cli auth-save --client-id '1000....' --client-secret '...' --refresh-token '1000....'
```

If the values are in Bitwarden/rbw, prefer custom fields named `ZOHO_CLIENT_ID`, `ZOHO_CLIENT_SECRET`, and `ZOHO_REFRESH_TOKEN`:

```bash
rbw unlock
zohomail-pp-cli auth-rbw --item 'Zoho Mail OAuth'
```

If `doctor` shows account and folder defaults but no auth, do not re-run account/folder discovery. Save auth only:

```bash
zohomail-pp-cli configure --save-token
```

Persist auth and default IDs when the user wants normal commands without repeated flags:

```bash
zohomail-pp-cli token --self-client --code '<generated-code>' --save
zohomail-pp-cli configure
```

If account/folder IDs are already known, configure them offline:

```bash
zohomail-pp-cli configure \
  --account-id "$ZOHO_ACCOUNT_ID" \
  --inbox-folder-id "$ZOHO_INBOX_FOLDER_ID" \
  --sent-folder-id "$ZOHO_SENT_FOLDER_ID" \
  --spam-folder-id "$ZOHO_SPAM_FOLDER_ID" \
  --trash-folder-id "$ZOHO_TRASH_FOLDER_ID" \
  --archive-folder-id "$ZOHO_ARCHIVE_FOLDER_ID"
```

If token exchange returns `invalid_client`, verify that `ZOHO_CLIENT_ID` and `ZOHO_CLIENT_SECRET` belong to the same active Zoho API Console client and that `ZOHO_ACCOUNTS_BASE_URL` matches the account data center.

For EU, IN, AU, JP, CA, CN, or other Zoho data centers, set both:

```bash
export ZOHO_MAIL_BASE_URL='https://mail.zoho.eu'
export ZOHO_ACCOUNTS_BASE_URL='https://accounts.zoho.eu'
```

## OAuth Bootstrap

```bash
zohomail-pp-cli client-setup
zohomail-pp-cli login
zohomail-pp-cli auth-save --client-id '1000....' --client-secret '...' --refresh-token '1000....'
zohomail-pp-cli auth-rbw --item 'Zoho Mail OAuth'
zohomail-pp-cli auth-url --redirect-uri 'http://localhost'
zohomail-pp-cli token --code '<code-from-browser>' --redirect-uri 'http://localhost'
```

For fully headless scripts that can tolerate a 1-hour token TTL (client credentials flow, works with Self Client):

```bash
zohomail-pp-cli auth-client-credentials --client-id '1000....' --client-secret '...'
# Returns an access_token only; no refresh token. Repeat the call each hour.
```

For device authorization flow (requires a **Non-browser Application** client type in Zoho API Console — Self Client does not support this grant):

```bash
zohomail-pp-cli auth-device --client-id '1000....' --client-secret '...'
# Prints verification_url and user_code; open the URL in a browser, enter the code, approve.
# Polls automatically; on approval saves the refresh token and discovers account/folder defaults.
# Use --no-open to suppress browser launch (e.g. remote shells).
```

For Zoho API Console Self Client generated codes, use:

```bash
zohomail-pp-cli token --self-client --code '<generated-code>'
```

Core scopes:

```text
ZohoMail.accounts.READ
ZohoMail.folders.READ
ZohoMail.messages.READ
ZohoMail.messages.CREATE
```

## Common Commands

Authenticate headlessly via client credentials (Self Client, access token only, 1h TTL):

```bash
zohomail-pp-cli auth-client-credentials --client-id '1000....' --client-secret '...'
```

Authenticate via device flow (Non-browser Application client only — not Self Client):

```bash
zohomail-pp-cli auth-device --client-id '1000....' --client-secret '...'
```

Check setup:

```bash
zohomail-pp-cli doctor
```

List accounts:

```bash
zohomail-pp-cli accounts
```

List folders:

```bash
zohomail-pp-cli folders
```

List messages:

```bash
zohomail-pp-cli inbox --limit 20
zohomail-pp-cli sent --limit 20
zohomail-pp-cli archive --limit 20
zohomail-pp-cli list --folder inbox --limit 20
```

Search messages:

```bash
zohomail-pp-cli search --search-key 'from:person@example.com' --limit 20
```

Read message content:

```bash
zohomail-pp-cli read --folder inbox --message-id "$ZOHO_MESSAGE_ID"
```

Send mail:

```bash
zohomail-pp-cli send --from me@example.com --to you@example.com --subject 'Subject' --content 'Body'
```

Use `--output json` when another tool will parse output.

## Safety

- Do not echo tokens in chat or logs.
- Treat pasted Zoho access tokens, refresh tokens, client secrets, and authorization codes as compromised; advise rotation after setup.
- Prefer read-only commands until the user explicitly asks to send mail.
- For `send`, confirm recipients, subject, and content when operating on real accounts unless the user already provided exact values.
- Do not persist secrets in repo files.
