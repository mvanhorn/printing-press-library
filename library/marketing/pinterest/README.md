# Pinterest CLI

**Every Pinterest board, pin, and ad â€” plus offline analytics no other tool can answer in a single command.**

pinterest-pp-cli wraps the full Pinterest API v5 with a local SQLite data layer that makes compound queries possible. Ask which boards drive the most saves, which days generate peak engagement, or where your ad spend outperforms organic â€” without writing a single API call.

Learn more at [Pinterest](https://developers.pinterest.com/).

Created by [@shourovchowdhury91-debug](https://github.com/shourovchowdhury91-debug) (Shourov Chowdhury).

## Install

The recommended path installs both the `pinterest-pp-cli` binary and the `pp-pinterest` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install pinterest
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install pinterest --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install pinterest --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install pinterest --agent claude-code
npx -y @mvanhorn/printing-press-library install pinterest --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/pinterest/cmd/pinterest-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/pinterest-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-pinterest --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-pinterest --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-pinterest skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-pinterest. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/pinterest-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `PINTEREST_ACCESS_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/pinterest/cmd/pinterest-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "pinterest": {
      "command": "pinterest-pp-mcp",
      "env": {
        "PINTEREST_ACCESS_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Pinterest uses OAuth2. Run 'auth login' to open a browser window for consent. Your access token and refresh token are stored locally and refreshed automatically.

## Quick Start

```bash
# Verify setup
pinterest-pp-cli doctor --dry-run

# Authenticate via Pinterest OAuth
pinterest-pp-cli auth login

# Sync boards, pins, and analytics to local SQLite
pinterest-pp-cli sync

# List all your boards
pinterest-pp-cli boards list --json

# See your highest-performing boards
pinterest-pp-cli top-boards --limit 10 --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`top-boards`** — Rank all your boards by total saves across all their pins.

  _Use when you need to know which boards drive the most engagement before planning content._

  ```bash
  pinterest-pp-cli top-boards --limit 10 --agent
  ```
- **`trends`** — Track how your pin impressions and saves change week-over-week.

  _Use to identify content timing patterns and seasonal peaks for your account._

  ```bash
  pinterest-pp-cli trends --weeks 4 --json
  ```
- **`boards stale`** — List boards that haven't received a new pin in the last N days.

  _Use for content calendar planning to find gaps before a content audit._

  ```bash
  pinterest-pp-cli boards stale --days 30 --json
  ```
- **`timing`** — Surface which days of the week historically generate the most saves for your account.

  _Use before scheduling new pins to maximize reach based on your own historical data._

  ```bash
  pinterest-pp-cli timing --weeks 8 --json
  ```

### Agent-native plumbing
- **`compare`** — Compare paid campaign performance against organic pin performance side-by-side.

  _Use when deciding whether to boost organic pins or increase ad spend._

  ```bash
  pinterest-pp-cli compare --days 30 --json
  ```
- **`boards gap`** — Find trending Pinterest topics you haven't covered in your boards recently.

  _Use when planning content strategy to identify high-opportunity trending topics you're missing._

  ```bash
  pinterest-pp-cli boards gap --region US --days 14 --json
  ```

## Recipes


### Find your top boards

```bash
pinterest-pp-cli top-boards --limit 5 --agent --select name,total_saves,pin_count
```

After syncing, ranks boards by cumulative pin saves.

### Export a board for AI analysis

```bash
pinterest-pp-cli boards export designmilk/packaging --format md
```

Outputs LLM-ready Markdown with all pin titles, descriptions, and image URLs.

### Spot stale boards

```bash
pinterest-pp-cli boards stale --days 14 --json
```

Lists boards with no new pins in the past 2 weeks — useful for content planning.

### Compare ad vs organic

```bash
pinterest-pp-cli compare --days 30 --json --select metric,paid,organic
```

Side-by-side paid campaign vs organic pin metrics for the last 30 days.

## Usage

Run `pinterest-pp-cli --help` for the full command reference and flag list.

## Commands

### ad-accounts

View analytical information about advertising.


Note: If the current operation_user_account (defined by the access token)
has access to another user's Ad Accounts via
<a href='/docs/reference/business-access/'>Pinterest Business Access</a>,
you can modify your request to use the current operation_user_account's
permissions to those Ad Accounts by including the ad_account_id in the path
parameters for the request (e.g. .../?ad_account_id=12345&...).

- **`pinterest-pp-cli ad-accounts create`** - Create a new ad account. Different ad accounts can support different currencies, payment methods, etc.
An ad account is needed to create campaigns, ad groups, and ads; other accounts (your employees or partners) can be assigned business access and appropriate roles to access an ad account. <p/>
You can set up up to 50 ad accounts per user. (The user must have a business account to create an ad account.) <p/>
For more, see <a class="reference external" href="https://help.pinterest.com/en/business/article/create-an-advertiser-account">Create an advertiser account</a>.
- **`pinterest-pp-cli ad-accounts get`** - Get an ad account
- **`pinterest-pp-cli ad-accounts list`** - Get a list of the ad_accounts that the "operation user_account" has access to.
- This includes ad_accounts they own and ad_accounts that are owned by others who have granted them <a href="https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts">Business Access</a>.

### boards

View, create, update, or delete information about boards.

- **`pinterest-pp-cli boards create`** - Create a board owned by the "operation user_account".
Optional: Business Access: Specify an ad_account_id to use the owner of that ad_account as the "operation user_account".
- By default, the "operation user_account" is the token user_account.
- **`pinterest-pp-cli boards delete`** - Delete a board owned by the "operation user_account".
- Optional: Business Access: Specify an ad_account_id to use the owner of that ad_account as the "operation user_account".
- By default, the "operation user_account" is the token user_account.
- **`pinterest-pp-cli boards get`** - Get a board owned by the operation user_account - or a group board that has been shared with this account.
- Optional: Business Access: Specify an ad_account_id to use the owner of that ad_account as the "operation user_account".
- By default, the "operation user_account" is the token user_account.
- **`pinterest-pp-cli boards list`** - Get a list of the boards owned by the "operation user_account" + group boards where this account is a collaborator
Optional: Business Access: Specify an ad_account_id to use the owner of that ad_account as the "operation user_account".
Optional: Specify a privacy type (public, protected, or secret) to indicate which boards to return.
- If no privacy is specified, all boards that can be returned (based on the scopes of the token and ad_account role if applicable) will be returned.
- **`pinterest-pp-cli boards update`** - Update a board owned by the "operating user_account".
- Optional: Business Access: Specify an ad_account_id to use the owner of that ad_account as the "operation user_account".
- By default, the "operation user_account" is the token user_account.

### catalogs

Manage information about shopping product catalogs and items.

- **`pinterest-pp-cli catalogs feed-processing-results-list`** - Fetch a feed processing results owned by the "operation user_account". Please note that for now the bookmark parameter is not functional and only the first page will be available until it is implemented in some release in the near future.
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an <code>ad_account_id</code> (obtained via <a href='/docs/api/v5/#operation/ad_accounts/list'>List ad accounts</a>) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following <a href="https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts">Business Access</a> roles on the ad_account: Owner, Admin, Catalogs Manager.

<a href='/docs/shopping/catalog/'>Learn more</a>
- **`pinterest-pp-cli catalogs feeds-create`** - Create a new feed owned by the "operation user_account".
- By default, the "operation user_account" is the token user_account.

Please, be aware that "default_country"
and "default_locale" are not required in the spec for forward compatibility
but for now the API will not accept requests without those fields.

Optional: Business Access: Specify an <code>ad_account_id</code> (obtained via <a href='/docs/api/v5/#operation/ad_accounts/list'>List ad accounts</a>) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following <a href="https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts">Business Access</a> roles on the ad_account: Owner, Admin, Catalogs Manager.

For Retail partners, refer to <a href='https://help.pinterest.com/en/business/article/before-you-get-started-with-catalogs'>Before you get started with Catalogs</a>. For Hotel parterns, refer to <a href='/docs/shopping/catalog/'>Pinterest API for shopping</a>.
- **`pinterest-pp-cli catalogs feeds-delete`** - Delete a feed owned by the "operating user_account".
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an <code>ad_account_id</code> (obtained via <a href='/docs/api/v5/#operation/ad_accounts/list'>List ad accounts</a>) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following <a href="https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts">Business Access</a> roles on the ad_account: Owner, Admin, Catalogs Manager.

For Retail partners, refer to <a href='https://help.pinterest.com/en/business/article/before-you-get-started-with-catalogs'>Before you get started with Catalogs</a>. For Hotel parterns, refer to <a href='/docs/shopping/catalog/'>Pinterest API for shopping</a>.
- **`pinterest-pp-cli catalogs feeds-get`** - Get a single feed owned by the "operation user_account".
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an <code>ad_account_id</code> (obtained via <a href='/docs/api/v5/#operation/ad_accounts/list'>List ad accounts</a>) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following <a href="https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts">Business Access</a> roles on the ad_account: Owner, Admin, Catalogs Manager.

For Retail partners, refer to <a href='https://help.pinterest.com/en/business/article/before-you-get-started-with-catalogs'>Before you get started with Catalogs</a>. For Hotel parterns, refer to <a href='/docs/shopping/catalog/'>Pinterest API for shopping</a>.
- **`pinterest-pp-cli catalogs feeds-list`** - Fetch feeds owned by the "operation user_account".
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an <code>ad_account_id</code> (obtained via <a href='/docs/api/v5/#operation/ad_accounts/list'>List ad accounts</a>) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following <a href="https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts">Business Access</a> roles on the ad_account: Owner, Admin, Catalogs Manager.

For Retail partners, refer to <a href='https://help.pinterest.com/en/business/article/before-you-get-started-with-catalogs'>Before you get started with Catalogs</a>. For Hotel parterns, refer to <a href='/docs/shopping/catalog/'>Pinterest API for shopping</a>.
- **`pinterest-pp-cli catalogs feeds-update`** - Update a feed owned by the "operation user_account".
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an <code>ad_account_id</code> (obtained via <a href='/docs/api/v5/#operation/ad_accounts/list'>List ad accounts</a>) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following <a href="https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts">Business Access</a> roles on the ad_account: Owner, Admin, Catalogs Manager.

For Retail partners, refer to <a href='https://help.pinterest.com/en/business/article/before-you-get-started-with-catalogs'>Before you get started with Catalogs</a>. For Hotel parterns, refer to <a href='/docs/shopping/catalog/'>Pinterest API for shopping</a>.
- **`pinterest-pp-cli catalogs items-batch-get`** - Get a single catalogs items batch owned by the "operating user_account". <a href="/docs/shopping/catalog/#Update%20items%20in%20batch" target="_blank">See detailed documentation here.</a>
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an <code>ad_account_id</code> (obtained via <a href='/docs/api/v5/#operation/ad_accounts/list'>List ad accounts</a>) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following <a href="https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts">Business Access</a> roles on the ad_account: Owner, Admin, Catalogs Manager.
- **`pinterest-pp-cli catalogs items-batch-post`** - This endpoint supports multiple operations on a set of one or more catalog items owned by the "operation user_account". <a href="/docs/shopping/catalog/#Update%20items%20in%20batch" target="_blank">See detailed documentation here.</a>
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an <code>ad_account_id</code> (obtained via <a href='/docs/api/v5/#operation/ad_accounts/list'>List ad accounts</a>) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following <a href="https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts">Business Access</a> roles on the ad_account: Owner, Admin, Catalogs Manager.
- **`pinterest-pp-cli catalogs items-get`** - Get the items of the catalog owned by the "operation user_account". <a href="/docs/shopping/catalog/#Update%20items%20in%20batch" target="_blank">See detailed documentation here.</a>
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an <code>ad_account_id</code> (obtained via <a href='/docs/api/v5/#operation/ad_accounts/list'>List ad accounts</a>) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following <a href="https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts">Business Access</a> roles on the ad_account: Owner, Admin, Catalogs Manager.
- **`pinterest-pp-cli catalogs items-issues-list`** - List item validation issues for a given feed processing result owned by the "operation user_account". Up to 20 random samples of affected items are returned for each error and warning code. Please note that for now query parameters 'item_numbers' and 'item_validation_issue' cannot be used simultaneously until it is implemented in some release in the future.
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an <code>ad_account_id</code> (obtained via <a href='/docs/api/v5/#operation/ad_accounts/list'>List ad accounts</a>) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following <a href="https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts">Business Access</a> roles on the ad_account: Owner, Admin, Catalogs Manager.

<a href='/docs/shopping/catalog/'>Learn more</a>
- **`pinterest-pp-cli catalogs list`** - Fetch catalogs owned by the "operation user_account".
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an <code>ad_account_id</code> (obtained via <a href='/docs/api/v5/#operation/ad_accounts/list'>List ad accounts</a>) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following <a href="https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts">Business Access</a> roles on the ad_account: Owner, Admin, Catalogs Manager.

<a href='/docs/shopping/catalog/'>Learn more</a>
- **`pinterest-pp-cli catalogs product-group-pins-list`** - Get a list of product pins for a given Catalogs Product Group Id owned by the "operation user_account".
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an <code>ad_account_id</code> (obtained via <a href='/docs/api/v5/#operation/ad_accounts/list'>List ad accounts</a>) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following <a href="https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts">Business Access</a> roles on the ad_account: Owner, Admin, Catalogs Manager.

<a href='/docs/shopping/catalog/'>Learn more</a>
- **`pinterest-pp-cli catalogs product-groups-create`** - Create product group to use in Catalogs owned by the "operation user_account".
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an <code>ad_account_id</code> (obtained via <a href='/docs/api/v5/#operation/ad_accounts/list'>List ad accounts</a>) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following <a href="https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts">Business Access</a> roles on the ad_account: Owner, Admin, Catalogs Manager.

<a href='/docs/shopping/catalog/'>Learn more</a>
- **`pinterest-pp-cli catalogs product-groups-delete`** - Delete a product group owned by the "operation user_account" from being in use in Catalogs.
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an <code>ad_account_id</code> (obtained via <a href='/docs/api/v5/#operation/ad_accounts/list'>List ad accounts</a>) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following <a href="https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts">Business Access</a> roles on the ad_account: Owner, Admin, Catalogs Manager.

<a href='/docs/shopping/catalog/'>Learn more</a>
- **`pinterest-pp-cli catalogs product-groups-get`** - Get a singe product group for a given Catalogs Product Group Id owned by the "operation user_account".
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an <code>ad_account_id</code> (obtained via <a href='/docs/api/v5/#operation/ad_accounts/list'>List ad accounts</a>) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following <a href="https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts">Business Access</a> roles on the ad_account: Owner, Admin, Catalogs Manager.

<a href='/docs/shopping/catalog/'>Learn more</a>
- **`pinterest-pp-cli catalogs product-groups-list`** - Get a list of product groups for a given Catalogs Feed Id owned by the "operation user_account".
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an <code>ad_account_id</code> (obtained via <a href='/docs/api/v5/#operation/ad_accounts/list'>List ad accounts</a>) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following <a href="https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts">Business Access</a> roles on the ad_account: Owner, Admin, Catalogs Manager.

<a href='/docs/shopping/catalog/'>Learn more</a>
- **`pinterest-pp-cli catalogs product-groups-product-counts-get`** - Get a product counts for a given Catalogs Product Group owned by the "operation user_account".
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an <code>ad_account_id</code> (obtained via <a href='/docs/api/v5/#operation/ad_accounts/list'>List ad accounts</a>) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following <a href="https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts">Business Access</a> roles on the ad_account: Owner, Admin, Catalogs Manager.

<a href='/docs/shopping/catalog/'>Learn more</a>
- **`pinterest-pp-cli catalogs product-groups-update`** - Update product group owned by the "operation user_account" to use in Catalogs.
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an <code>ad_account_id</code> (obtained via <a href='/docs/api/v5/#operation/ad_accounts/list'>List ad accounts</a>) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following <a href="https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts">Business Access</a> roles on the ad_account: Owner, Admin, Catalogs Manager.

<a href='/docs/shopping/catalog/'>Learn more</a>
- **`pinterest-pp-cli catalogs products-by-product-group-filter-list`** - List products Pins owned by the "operation user_account" that meet the criteria specified in the Catalogs Product Group Filter given in the request.
- This endpoint has been implemented in POST to allow for complex filters. This specific POST endpoint is designed to be idempotent.
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an <code>ad_account_id</code> (obtained via <a href='/docs/api/v5/#operation/ad_accounts/list'>List ad accounts</a>) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following <a href="https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts">Business Access</a> roles on the ad_account: Owner, Admin, Catalogs Manager.

<a href='/docs/shopping/catalog/'>Learn more</a>

### integrations

View, create, or update commerce integrations.

- **`pinterest-pp-cli integrations commerce-del`** - Delete commerce integration metadata for the given external business ID.
Note: If you're interested in joining the beta, please reach out to your Pinterest account manager.
- **`pinterest-pp-cli integrations commerce-get`** - Get commerce integration metadata associated with the given external business ID.
Note: If you're interested in joining the beta, please reach out to your Pinterest account manager.
- **`pinterest-pp-cli integrations commerce-patch`** - Update commerce integration metadata for the given external business ID.
Note: If you're interested in joining the beta, please reach out to your Pinterest account manager.
- **`pinterest-pp-cli integrations commerce-post`** - Create commerce integration metadata to link an external business ID with a Pinterest merchant & ad account.
Note: If you're interested in joining the beta, please reach out to your Pinterest account manager.
- **`pinterest-pp-cli integrations get-by-id`** - Get integration metadata by ID.
Note: If you're interested in joining the beta, please reach out to your Pinterest account manager.
- **`pinterest-pp-cli integrations get-list`** - Get integration metadata list.
Note: If you're interested in joining the beta, please reach out to your Pinterest account manager.
- **`pinterest-pp-cli integrations logs-post`** - This endpoint receives batched logs from integration applications on partner platforms.
Note: If you're interested in joining the beta, please reach out to your Pinterest account manager.

### media

Register and manage media uploads.

- **`pinterest-pp-cli media create`** - Register your intent to upload media

The response includes all of the information needed to upload the media
to Pinterest.

To upload the media, make an HTTP POST request (using <tt>curl</tt>, for
example) to <tt>upload_url</tt> using the <tt>Content-Type</tt> header
value. Send the media file's contents as the request's <tt>file</tt>
parameter and also include all of the parameters from
<tt>upload_parameters</tt>.

<strong><a href='/docs/content/content-creation/#Creating%20video%20Pins'>Learn more</a></strong> about video Pin creation.
- **`pinterest-pp-cli media get`** - Get details for a registered media upload, including its current status.

<strong><a href='/docs/content/content-creation/#Creating%20video%20Pins'>Learn more</a></strong> about video Pin creation.
- **`pinterest-pp-cli media list`** - List media uploads filtered by given parameters.

<strong><a href='/docs/content/content-creation/#Creating%20video%20Pins'>Learn more</a></strong> about video Pin creation.

### oauth

Generate and refresh OAuth access tokens.

- **`pinterest-pp-cli oauth`** - Generate an OAuth access token by using an authorization code or a refresh token.

IMPORTANT: You need to start the OAuth flow via www.pinterest.com/oauth before calling this endpoint (or have an existing refresh token).

See <a href='/docs/getting-started/authentication/'>Authentication</a> for more.

<strong>Parameter <i>refresh_on</i> and its corresponding response type <i>everlasting_refresh</i> are now available to all apps! Later this year, continuous refresh will become the default behavior (ie you will no longer need to send this parameter). <a href='/docs/new/about-beta-access/'>Learn more</a>.</strong>

### pins

View, create, update, or delete information about Pins.

- **`pinterest-pp-cli pins create`** - Create a Pin on a board or board section owned by the "operation user_account".

Note: If the current "operation user_account" (defined by the access token) has access to another user's Ad Accounts via Pinterest Business Access, you can modify your request to make use of the current operation_user_account's permissions to those Ad Accounts by including the ad_account_id in the path parameters for the request (e.g. .../?ad_account_id=12345&...).

- This function is intended solely for publishing new content created by the user. If you are interested in saving content created by others to your Pinterest boards, sometimes called 'curated content', please use our <a href='/docs/add-ons/save-button'>Save button</a> instead. For more tips on creating fresh content for Pinterest, review our <a href='/docs/content/content-creation/'>Content App Solutions Guide</a>.

<strong><a href='/docs/content/content-creation/#Creating%20video%20Pins'>Learn more</a></strong> about video Pin creation.
- **`pinterest-pp-cli pins delete`** - Delete a Pins owned by the "operation user_account" - or on a group board that has been shared with this account.
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an <code>ad_account_id</code> (obtained via <a href='/docs/api/v5/#operation/ad_accounts/list'>List ad accounts</a>) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following <a href="https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts">Business Access</a> roles on the ad_account:

- For Pins on public or protected boards: Owner, Admin, Analyst, Campaign Manager.
- For Pins on secret boards: Owner, Admin.
- **`pinterest-pp-cli pins get`** - Get a Pin owned by the "operation user_account" - or on a group board that has been shared with this account.
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an <code>ad_account_id</code> (obtained via <a href='/docs/api/v5/#operation/ad_accounts/list'>List ad accounts</a>) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following <a href="https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts">Business Access</a> roles on the ad_account:

- For Pins on public or protected boards: Owner, Admin, Analyst, Campaign Manager.
- For Pins on secret boards: Owner, Admin.
- **`pinterest-pp-cli pins list`** - Get a list of the Pins owned by the "operation user_account".
- By default, the "operation user_account" is the token user_account.
- All Pins owned by the "operation user_account" are included, regardless of who owns the board they are on.
Optional: Business Access: Specify an ad_account_id to use the owner of that ad_account as the "operation user_account".
- **`pinterest-pp-cli pins update`** - Update a pin owned by the "operating user_account".
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an <code>ad_account_id</code> (obtained via <a href='/docs/api/v5/#operation/ad_accounts/list'>List ad accounts</a>) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following <a href="https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts">Business Access</a> roles on the ad_account:

- For Pins on public or protected boards: Owner, Admin, Analyst, Campaign Manager.
- For Pins on secret boards: Owner, Admin.

<strong>This endpoint is currently in beta and not available to all apps. <a href='/docs/new/about-beta-access/'>Learn more</a>.</strong>

### pinterest-search

Manage pinterest search

- **`pinterest-pp-cli pinterest-search partner-pins`** - <strong>This endpoint is currently in beta and not available to all apps. <a href='/docs/new/about-beta-access/'>Learn more</a>.</strong>

Get the top 10 Pins by a given search term.
- **`pinterest-pp-cli pinterest-search user-boards-get`** - Search for boards for the "operation user_account". This includes boards of all board types.
- By default, the "operation user_account" is the token user_account.

If using Business Access: Specify an ad_account_id to use the owner of that ad_account as the "operation user_account". See <a href='/docs/reference/business-access/'>Understanding Business Access</a> for more information.
- **`pinterest-pp-cli pinterest-search user-pins-list`** - Search for pins for the "operation user_account".
- By default, the "operation user_account" is the token user_account.

If using Business Access: Specify an ad_account_id to use the owner of that ad_account as the "operation user_account". See <a href='/docs/reference/business-access/'>Understanding Business Access</a> for more information.

### resources

View metadata about available metrics and targeting options in the Pinterest API.

- **`pinterest-pp-cli resources ad-account-countries-get`** - Get Ad Accounts countries
- **`pinterest-pp-cli resources delivery-metrics-get`** - Get the definitions for ads and organic metrics available across both synchronous and asynchronous report endpoints.
The `display_name` attribute will match how the metric is named in our native tools like Ads Manager.
See <a href='/docs/content/analytics/'>Organic Analytics</a> and <a href='/docs/ads/ad-analytics-reporting/'>Ads Analytics</a> for more information.
- **`pinterest-pp-cli resources interest-targeting-options-get`** - <p>Get details of a specific interest given interest ID.</p> <p>Click <a href="https://docs.google.com/spreadsheets/d/1HxL-0Z3p2fgxis9YBP2HWC3tvPrs1hAuHDRtH-NJTIM/edit#gid=118370875" target="_blank">here</a> for a spreadsheet listing interests and their IDs.</p>
- **`pinterest-pp-cli resources lead-form-questions-get`** - Get a list of all lead form question type names. Some questions might not be used.

<strong>This endpoint is currently in beta and not available to all apps. <a href='/docs/new/about-beta-access/'>Learn more</a>.</strong>
- **`pinterest-pp-cli resources metrics-ready-state-get`** - Learn whether conversion or non-conversion metrics are finalized and ready to query.
- **`pinterest-pp-cli resources targeting-options-get`** - <p>You can use targeting values in ads placement to define your intended audience. </p> <p>Targeting metrics are organized around targeting specifications.</p> <p>For more information on ads targeting, see <a class="reference external" href="https://help.pinterest.com/en/business/article/audience-targeting" target="_blank">Audience targeting</a>.</p>
<p><b>Sample return:</b></p> <pre class="literal-block"> [{&quot;36313&quot;: &quot;Australia: Moreton Bay - North&quot;, &quot;124735&quot;: &quot;Canada: North Battleford&quot;, &quot;36109&quot;: &quot;Australia: Murray&quot;, &quot;36108&quot;: &quot;Australia: Mid North Coast&quot;, &quot;36101&quot;: &quot;Australia: Capital Region&quot;, &quot;811&quot;: &quot;U.S.: Reno&quot;, &quot;36103&quot;: &quot;Australia: Central West&quot;, &quot;36102&quot;: &quot;Australia: Central Coast&quot;, &quot;36105&quot;: &quot;Australia: Far West and Orana&quot;, &quot;36104&quot;: &quot;Australia: Coffs Harbour - Grafton&quot;, &quot;36107&quot;: &quot;Australia: Illawarra&quot;, &quot;36106&quot;: &quot;Australia: Hunter Valley Exc Newcastle&quot;, &quot;554017&quot;: &quot;New Zealand: Wanganui&quot;, &quot;554016&quot;: &quot;New Zealand: Marlborough&quot;, &quot;554015&quot;: &quot;New Zealand: Gisborne&quot;, &quot;554014&quot;: &quot;New Zealand: Tararua&quot;, &quot;554013&quot;: &quot;New Zealand: Invercargill&quot;, &quot;GR&quot;: &quot;Greece&quot;, &quot;554011&quot;: &quot;New Zealand: Whangarei&quot;, &quot;554010&quot;: &quot;New Zealand: Far North&quot;, &quot;717&quot;: &quot;U.S.: Quincy-Hannibal-Keokuk&quot;, &quot;716&quot;: &quot;U.S.: Baton Rouge&quot;,...}] </pre>

### terms

View related and suggested terms for ads targeting.

- **`pinterest-pp-cli terms related-list`** - Get a list of terms logically related to each input term. <p/>
Example: the term 'workout' would list related terms like 'one song workout', 'yoga workout', 'workout motivation', etc.
- **`pinterest-pp-cli terms suggested-list`** - Get popular search terms that begin with your input term. <p/>
Example: 'sport' would return popular terms like 'sports bar' and 'sportswear', but not 'motor sports' since the phrase does not begin with the given term.

### trends

Manage trends

- **`pinterest-pp-cli trends`** - <p>Get the top trending search keywords among the Pinterest user audience.</p>
<p>Trending keywords can be used to inform ad targeting, budget strategy, and creative decisions about which products and Pins will resonate with your audience.</p>
<p>Geographic, demographic and interest-based filters are available to narrow down to the top trends among a specific audience. Multiple trend types are supported that can be used to identify newly-popular, evergreen or seasonal keywords.</p>
<p>For an interactive way to explore this data, please visit <a href="https://trends.pinterest.com">trends.pinterest.com</a>.

### user-account

View user accounts associated with a given access token.

- **`pinterest-pp-cli user-account analytics`** - Get analytics for the "operation user_account"
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an ad_account_id to use the owner of that ad_account as the "operation user_account".
- **`pinterest-pp-cli user-account analytics-top-pins`** - Gets analytics data about a user's top pins (limited to the top 50).
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an ad_account_id to use the owner of that ad_account as the "operation user_account".
- **`pinterest-pp-cli user-account analytics-top-video-pins`** - Gets analytics data about a user's top video pins (limited to the top 50).
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an ad_account_id to use the owner of that ad_account as the "operation user_account".
- **`pinterest-pp-cli user-account boards-user-follows-list`** - Get a list of the boards a user follows. The request returns a board summary object array.
- **`pinterest-pp-cli user-account follow-user-update`** - <strong>This endpoint is currently in beta and not available to all apps. <a href='/docs/new/about-beta-access/'>Learn more</a>.</strong>

Use this request, as a signed-in user, to follow another user.
- **`pinterest-pp-cli user-account followers-list`** - Get a list of your followers.
- **`pinterest-pp-cli user-account get`** - Get account information for the "operation user_account"
- By default, the "operation user_account" is the token user_account.

If using Business Access: Specify an ad_account_id to use the owner of that ad_account as the "operation user_account". See <a href='/docs/reference/business-access/'>Understanding Business Access</a> for more information.
- **`pinterest-pp-cli user-account linked-business-accounts-get`** - Get a list of your linked business accounts.
- **`pinterest-pp-cli user-account unverify-website-delete`** - Unverifu a website verified by the signed-in user.
- **`pinterest-pp-cli user-account user-following-get`** - Get a list of who a certain user follows.
- **`pinterest-pp-cli user-account user-websites-get`** - Get user websites, claimed or not
- **`pinterest-pp-cli user-account verify-website-update`** - Verify a website as a signed-in user.
- **`pinterest-pp-cli user-account website-verification-get`** - Get verification code for user to install on the website to claim it.

### users

Manage users



## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
pinterest-pp-cli ad-accounts list

# JSON for scripting and agents
pinterest-pp-cli ad-accounts list --json

# Filter to specific fields
pinterest-pp-cli ad-accounts list --json --select id,name,status

# Dry run — show the request without sending
pinterest-pp-cli ad-accounts list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
pinterest-pp-cli ad-accounts list --agent
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
pinterest-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/pinterest-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `PINTEREST_ACCESS_TOKEN` | per_call | No | Set to your API credential. |
| `PINTEREST_PINTEREST_OAUTH2` | per_call | No | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `pinterest-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `pinterest-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $PINTEREST_ACCESS_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Unauthorized** — Run 'pinterest-pp-cli auth login' to refresh your OAuth token
- **Rate limit exceeded (429)** — Pinterest trial access limits to 1000 req/day per category. Wait or apply for standard access.
- **Empty results from top-boards** — Run 'pinterest-pp-cli sync' first to populate the local store

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**collactivelabs/pinterest-mcp-server**](https://github.com/collactivelabs/pinterest-mcp-server) — JavaScript (8 stars)
- [**brtdwchtr/pinterest-export**](https://github.com/brtdwchtr/pinterest-export) — Python (1 stars)
- [**CDataSoftware/pinterest-mcp-server-by-cdata**](https://github.com/CDataSoftware/pinterest-mcp-server-by-cdata) — JavaScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
