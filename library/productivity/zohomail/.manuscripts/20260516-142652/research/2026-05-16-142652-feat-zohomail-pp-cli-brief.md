# Zoho Mail CLI Brief

## API Identity
- Domain: Zoho Mail account, folder, message, search, and send workflows.
- Users: operators and agents that need terminal access to Zoho Mail without opening the mailbox UI.
- Data profile: accounts, folders, message metadata, message content, search results, and outbound email payloads.

## Reachability Risk
- Medium. Zoho Mail APIs require OAuth2 and data-center-specific base URLs. The CLI supports both direct access tokens and refresh-token exchange through `accounts.zoho.com` or an overridden regional accounts host.

## Top Workflows
1. Discover accounts and folders, then save defaults in local config.
2. List inbox, sent, archive, spam, trash, or named-folder messages.
3. Search messages with Zoho `searchKey` syntax.
4. Read a message by folder and message ID.
5. Send a new email from the configured account.

## Auth Surface
- `login` runs a browser OAuth redirect flow on `localhost:53682/callback`.
- `auth-url` prints an authorization URL for manual OAuth setup.
- `token` exchanges an authorization code and can save refresh-token config.
- `auth-client-credentials` requests a short-lived token for headless scripting where supported.
- `auth-device` supports device authorization for non-browser clients where the Zoho tenant allows it.
- `auth-rbw` loads client credentials and refresh token from Bitwarden via `rbw`.

## Product Thesis
`zohomail-pp-cli` gives agents a small, auditable, stdlib-only command surface for Zoho Mail operations. It keeps secrets out of output, supports regional Zoho hosts, and exposes mail workflows as repeatable CLI commands.
