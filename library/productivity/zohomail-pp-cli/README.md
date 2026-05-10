# zohomail-pp-cli

Printing Press-style Go CLI for Zoho Mail.

## Install

```bash
go install github.com/DankeyDevDave/zohomail-pp-cli@latest
```

For local development:

```bash
go install .
```

## Auth

Use either a short-lived access token:

```bash
export ZOHO_MAIL_ACCESS_TOKEN='...'
```

Or a refresh-token setup:

```bash
export ZOHO_CLIENT_ID='...'
export ZOHO_CLIENT_SECRET='...'
export ZOHO_REFRESH_TOKEN='...'
```

Check what the CLI can see without printing secrets:

```bash
zohomail-pp-cli doctor
```

Save auth locally after Self Client token exchange:

```bash
zohomail-pp-cli token --self-client --code '<generated-code>' --save
```

Then discover account and standard folder defaults:

```bash
zohomail-pp-cli configure
```

If you already know the IDs, save them without another API call:

```bash
zohomail-pp-cli configure \
  --account-id 123456789 \
  --inbox-folder-id 111 \
  --sent-folder-id 222 \
  --spam-folder-id 333 \
  --trash-folder-id 444 \
  --archive-folder-id 555
```

If token exchange returns `invalid_client`, verify that `ZOHO_CLIENT_ID` and `ZOHO_CLIENT_SECRET` belong to the same active Zoho API Console client and that `ZOHO_ACCOUNTS_BASE_URL` matches the account data center.

For non-US Zoho data centers, set both bases, for example:

```bash
export ZOHO_MAIL_BASE_URL='https://mail.zoho.eu'
export ZOHO_ACCOUNTS_BASE_URL='https://accounts.zoho.eu'
```

## OAuth helper

Automated browser login:

```bash
zohomail-pp-cli client-setup
zohomail-pp-cli login --client-id '1000....' --client-secret '...'
```

`client-setup` opens the Zoho API Console and prints the redirect URI/scopes to configure. `login` opens Zoho in the browser, listens on `http://localhost:53682/callback`, exchanges the returned code, saves the client credentials + refresh token, and discovers account/folder defaults. The redirect URI must exist on the Zoho API Console client.

If the client credentials are already saved, later logins need no flags:

```bash
zohomail-pp-cli login
```

If you already have a refresh token, skip the browser flow and save it once:

```bash
zohomail-pp-cli auth-save --client-id '1000....' --client-secret '...' --refresh-token '1000....'
```

If those values live in Bitwarden/rbw, store them as custom fields named
`ZOHO_CLIENT_ID`, `ZOHO_CLIENT_SECRET`, and `ZOHO_REFRESH_TOKEN`, then run:

```bash
rbw unlock
zohomail-pp-cli auth-rbw --item 'Zoho Mail OAuth'
```

To populate that Bitwarden item from the latest local `ZOHO_*` shell-history
exports without printing secrets:

```bash
scripts/write_zoho_auth_to_bitwarden.py
```

For remote shells:

```bash
zohomail-pp-cli login --no-open
```

```bash
zohomail-pp-cli auth-url --redirect-uri 'http://localhost'
zohomail-pp-cli token --code '<returned-code>' --redirect-uri 'http://localhost'
```

For Zoho API Console Self Client generated codes, use:

```bash
zohomail-pp-cli token --self-client --code '<generated-code>'
```

Required scopes for the core commands:

```text
ZohoMail.accounts.READ
ZohoMail.folders.READ
ZohoMail.messages.READ
ZohoMail.messages.CREATE
```

## Commands

```bash
zohomail-pp-cli accounts
zohomail-pp-cli folders
zohomail-pp-cli inbox --limit 20
zohomail-pp-cli sent --limit 20
zohomail-pp-cli list --folder inbox --limit 20
zohomail-pp-cli search --search-key 'from:person@example.com' --limit 10
zohomail-pp-cli read --folder inbox --message-id 1709876190693100009
zohomail-pp-cli read --account-id 123456789 --folder-id 987654321 --message-id 1709876190693100009 --mode details
zohomail-pp-cli send --from me@example.com --to you@example.com --subject 'Hello' --content 'Body'
```

Use `--output json` for raw API-shaped output.

## API coverage

Implemented against Zoho Mail REST APIs:

- `GET /api/accounts`
- `GET /api/accounts/{accountId}/folders`
- `GET /api/accounts/{accountId}/messages/view`
- `GET /api/accounts/{accountId}/messages/search`
- `GET /api/accounts/{accountId}/folders/{folderId}/messages/{messageId}/content`
- `GET /api/accounts/{accountId}/folders/{folderId}/messages/{messageId}/details`
- `POST /api/accounts/{accountId}/messages`

References:

- https://www.zoho.com/mail/help/api/account-api.html
- https://www.zoho.com/mail/help/api/get-all-folder-details.html
- https://www.zoho.com/mail/help/api/get-emails-list.html
- https://www.zoho.com/mail/help/api/get-search-emails.html
- https://www.zoho.com/mail/help/api/email-api.html
- https://www.zoho.com/mail/help/api/post-send-an-email.html
