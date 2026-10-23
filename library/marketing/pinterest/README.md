# Pinterest CLI

**Every Pinterest v5 endpoint plus period-over-period analytics, Pin hygiene audits, and offline Pin search.**

Covers boards, Pins, analytics, trends, ads, catalogs, and audiences from one binary with --dry-run on writes, no interactive prompts, and explicit --yes confirmation. A bounded local sync powers offline search, the 'pins audit' hygiene report, and 'trends gaps'; 'perf compare' and 'pins funnel' turn raw analytics into answers.

Learn more at [Pinterest](https://developers.pinterest.com/).

Created by [@ElizabethcStone](https://github.com/ElizabethcStone) (Elizabeth Stone).

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

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.10 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/pinterest/cmd/pinterest-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/pinterest-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install pinterest --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-pinterest --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-pinterest --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install pinterest --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

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

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS, `%APPDATA%\Claude\claude_desktop_config.json` on Windows):

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

Pinterest uses OAuth 2.0 authorization code.

**Before you start: you need your own Pinterest developer app.** A normal Pinterest login is not enough. Pinterest only gives API access to apps registered on its developer site, and this CLI does not ship one, so it logs in as *your* app. Creating one is free and takes a few minutes; you only do it once.

1. **Create your developer app.** Sign in at https://developers.pinterest.com/apps/ with the Pinterest account you want to manage, then create (connect) a new app. Give it a name, a short description and your website, and accept the developer terms. Pinterest may take a while to approve it. A new app starts with **Trial access**, which is enough for the CLI to work on your own account. If you already have an app, open it instead.
2. **Register the redirect URI** `http://localhost:8875/callback` under the app's redirect links. It must match exactly; if you change it, pass the same port with `--port`.
3. **Copy the App ID** and **reveal the App secret key** on the app page. Copy the revealed value, not the masked dots.
4. **Log in:**

   ```bash
   pinterest-pp-cli auth login --client-id <app-id>
   ```

   The CLI asks for the client secret with hidden input, so it never shows on screen or lands in shell history. Paste it and press Enter. (Non-interactive alternative: set `PINTEREST_CLIENT_ID` and `PINTEREST_CLIENT_SECRET`.)
5. **Approve in the browser.** Pinterest shows "Authorize app"; click **Give access**. The CLI saves the token and keeps it fresh with continuous refresh.
6. **Check it:** `pinterest-pp-cli auth status` or `pinterest-pp-cli doctor`.

**Scopes.** By default, login requests every scope Pinterest documents: ads, billing, biz_access, boards, catalogs, pins and user_accounts, read/write plus the secret variants. It leaves out `msot:write`, which Pinterest's OAuth docs do not list. To choose a different set, pass `--scopes` (comma-separated, replaces the default) or set `PINTEREST_SCOPES` to keep the choice. `--scopes basic` means boards, pins and user_accounts read+write, a smaller set for accounts or apps that cannot grant ads, billing, biz_access or catalogs. Pinterest's docs do not say whether personal (non-business) accounts can grant those four; if login is rejected, try `basic`. Add `msot:write` only if Pinterest has enabled MSOT for your app, e.g. `--scopes basic,msot:write`.

**Already have an access token?** Export it as `PINTEREST_ACCESS_TOKEN` (`PINTEREST_OAUTH2` is also accepted) for one-off runs, or store it with `echo "$TOKEN" | pinterest-pp-cli auth set-token`.

**Login troubleshooting**

| What you see | Cause | Fix |
|---|---|---|
| "invalid scopes requested" on Pinterest's page | Your app or account can't grant one of the requested scopes | Retry with `--scopes basic`, or list only the scopes you need |
| Redirect error on Pinterest's page | The redirect URI isn't registered exactly | Add `http://localhost:8875/callback` to the app, or match `--port` to the one you registered |
| `exchanging code for token: HTTP 401 ... Authentication failed` | Wrong client secret (often the masked dots were copied, or the secret was reset) | Reveal the secret on the app page, or reset it, and log in again |

## Quick Start

> **First time?** Pinterest's API needs your own (free) Pinterest developer app before you can log in. Follow [Authentication](#authentication) once, then come back here.

```bash
# Confirm the binary and command tree are installed (no network); run plain 'doctor' to check auth and the API
pinterest-pp-cli doctor --dry-run


# Confirm which Pinterest account the token belongs to
pinterest-pp-cli user-account get --agent --select username,account_type


# Pull your boards, sections, and Pins into the local store (bounded by default)
pinterest-pp-cli sync


# Find Pins missing alt text, links, titles, or UTM tags
pinterest-pp-cli pins audit --agent


# See this month versus last month
pinterest-pp-cli perf compare --days 28 --agent

```

## Unique Features

### Analytics that answer the question

- **`perf compare`** — See whether impressions, saves, and outbound clicks grew versus the previous equal-length period, with absolute and percent deltas.

  _Reach for this for a weekly or monthly 'did we grow?' answer instead of calling account analytics twice and doing date math._

  ```bash
  pinterest-pp-cli perf compare --days 28 --agent
  ```
- **`pins funnel`** — Rank your top Pins by impressions (or --sort-by metric), compute save and outbound-click rates, and flag Pins that get saved but don't send traffic.

  _Use to decide which Pins to refresh or repurpose to drive site traffic._

  ```bash
  pinterest-pp-cli pins funnel --days 30 --limit 25 --agent
  ```
- **`trends gaps`** — List trending keywords for a region with how many of your synced Pins cover each, zero-coverage first. Coverage counts need a synced local store.

  _Use when planning the next content batch to find trends you have nothing on._

  ```bash
  pinterest-pp-cli trends gaps --region US --agent
  ```

### Local state that compounds

- **`pins audit`** — Find published Pins missing alt text, a destination link, a title, or UTM tracking, grouped by board. Reads the local store; run sync first.

  _Use before an SEO cleanup pass to get the exact Pin IDs that need fixing._

  ```bash
  pinterest-pp-cli pins audit --group-by board --agent
  ```
- **`pins preflight`** — Validate a proposed Pin (field limits, board and section by name, UTM link) and get a ready-to-send create body without writing anything. Board names resolve from the local store; run sync first.

  _Use before pins create so an agent never publishes an incomplete or misrouted Pin._

  ```bash
  pinterest-pp-cli pins preflight --board "Recipes" --link https://example.com/blog/meal-prep?utm_source=pinterest --image-url https://example.com/img/meal-prep.jpg --title "Weeknight dinner ideas" --alt-text "Bowl of pasta on a table" --agent
  ```

### Agent-native plumbing

- **`ads report`** — Create, wait for, and download an ads performance report in one command. Requires the ads:read scope and --ad-account-id (or PINTEREST_AD_ACCOUNT_ID).

  _Use for campaign or ad performance instead of manually polling the async report endpoint._

  ```bash
  pinterest-pp-cli ads report --level campaign --days 7 --agent
  ```

## Recipes


### Monthly growth check

```bash
pinterest-pp-cli perf compare --days 28 --agent
```

Current vs prior 28 days for impressions, saves, outbound clicks, and engagement.

### Narrow a large Pin listing

```bash
pinterest-pp-cli pins list --agent --select items.id,items.title,items.link,items.board_id
```

Pin payloads are large; select only the fields an agent needs.

### SEO cleanup list

```bash
pinterest-pp-cli pins audit --group-by board --agent
```

Pins missing alt text, links, titles, or UTM tags, grouped by board.

### Content gaps from trends

```bash
pinterest-pp-cli trends gaps --region US --agent
```

Trending keywords your synced Pins don't cover yet.

## Usage

Run `pinterest-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `PINTEREST_CONFIG_DIR`, `PINTEREST_DATA_DIR`, `PINTEREST_STATE_DIR`, or `PINTEREST_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `PINTEREST_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export PINTEREST_HOME=/srv/pinterest
pinterest-pp-cli doctor
```

Under `PINTEREST_HOME=/srv/pinterest`, the four dirs resolve to `/srv/pinterest/config`, `/srv/pinterest/data`, `/srv/pinterest/state`, and `/srv/pinterest/cache`.

On Windows, set the same variable in PowerShell:

```powershell
$env:PINTEREST_HOME = "C:\srv\pinterest"
pinterest-pp-cli doctor
```

Under `$env:PINTEREST_HOME = "C:\srv\pinterest"`, the four dirs resolve to `C:\srv\pinterest\config`, `C:\srv\pinterest\data`, `C:\srv\pinterest\state`, and `C:\srv\pinterest\cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "pinterest": {
      "command": "pinterest-pp-mcp",
      "env": {
        "PINTEREST_HOME": "/srv/pinterest"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `PINTEREST_DATA_DIR` overrides an explicit `--home` for that kind. Use `PINTEREST_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `PINTEREST_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `pinterest-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### ad-accounts

View analytical information about advertising.

Note: If the current operation_user_account (defined by the access token)
has access to another user's Ad Accounts via
<a href='/docs/getting-started/using-business-access/'>Pinterest Business Access</a>,
you can modify your request to use the current operation_user_account's
permissions to those Ad Accounts by including the ad_account_id in the path
parameters for the request (e.g. .../?ad_account_id=12345&...).

- **`pinterest-pp-cli ad-accounts create`** - Create a new ad account. Different ad accounts can support different currencies, payment methods, etc.
An ad account is needed to create campaigns, ad groups, and ads; other accounts (your employees or partners) can be assigned business access and appropriate roles to access an ad account.

You can set up up to 50 ad accounts per user. (The user must have a business account to create an ad account.)
For more, see [Create an advertiser account](https://help.pinterest.com/en/business/article/create-an-advertiser-account).
- **`pinterest-pp-cli ad-accounts get`** - Get an ad account
- **`pinterest-pp-cli ad-accounts list`** - Get a list of the ad_accounts that the "operation user_account" has access to.
        - This includes ad_accounts they own and ad_accounts that are owned by others who have granted them [Business Access](https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts).

### advanced-auction

View, create, or update advanced auction item bid options.

- **`pinterest-pp-cli advanced-auction items-get-post`** - Get the bid options for a batch of retail catalog items.

The catalog must be owned by the "operation user_account". [See detailed documentation here.](/docs/api-features/shopping-overview/#Update%20items%20in%20batch) By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an `ad_account_id` (obtained via [List ad accounts](/docs/api/v5/#operation/ad_accounts/list)) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following [Business Access](https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts) roles on the ad_account: `Owner`, `Admin`.

This endpoint is not available to all users.
- **`pinterest-pp-cli advanced-auction items-submit-post`** - This endpoint supports multiple operations on a set of one or more bid options (bid price and bid adjustments for targeting categories) for retail catalog items. These advanced auction settings are applied in campaigns using objective_type `CATALOG_SALES` and ad groups using bid_strategy_type `MAX_BID`.

The catalog must be owned by the "operation user_account". [See detailed documentation here.](/docs/api-features/modify-items-in-batch/) By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an `ad_account_id` (obtained via [List ad accounts](/docs/api/v5/#operation/ad_accounts/list)) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following [Business Access](https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts) roles on the ad_account: `Owner`, `Admin`.

This endpoint is not available to all users.

### boards

View, create, update, or delete information about boards.

- **`pinterest-pp-cli boards create`** - Create a board owned by the "operation user_account".
Optional: Business Access: Specify an ad_account_id to use the owner of that ad_account as the "operation user_account".
* By default, the "operation user_account" is the token user_account.
- **`pinterest-pp-cli boards delete`** - Delete a board owned by the "operation user_account".
* Optional: Business Access: Specify an ad_account_id to use the owner of that ad_account as the "operation user_account".
* By default, the "operation user_account" is the token user_account.
- **`pinterest-pp-cli boards get`** - Get a board owned by the operation user_account - or a group board that has been shared with this account.
* Optional: Business Access: Specify an ad_account_id to use the owner of that ad_account as the "operation user_account".
* By default, the "operation user_account" is the token user_account.
- **`pinterest-pp-cli boards list`** - Get a list of the boards owned by the "operation user_account" + group boards where this account is a collaborator
Optional: Business Access: Specify an ad_account_id to use the owner of that ad_account as the "operation user_account".
Optional: Specify a privacy type (public, protected, or secret) to indicate which boards to return.
* If no privacy is specified, all boards that can be returned (based on the scopes of the token and ad_account role if applicable) will be returned.
- **`pinterest-pp-cli boards update`** - Update a board owned by the "operating user_account".
* Optional: Business Access: Specify an ad_account_id to use the owner of that ad_account as the "operation user_account".
* By default, the "operation user_account" is the token user_account.

### business-access

Manage business access

- **`pinterest-pp-cli business-access brand-accounts-create`** - Create a Brand Account that will be a child business of a business hierarchy. Request must contain name, username, and country.
- **`pinterest-pp-cli business-access brand-accounts-update`** - Update an existing Brand Account

### businesses

Manage businesses

- **`pinterest-pp-cli businesses get-business-employers`** - Get all of the viewing user's business employers.
- **`pinterest-pp-cli businesses respond-business-access-invites`** - Accept or decline invites or requests.

### catalogs

Manage information about shopping product catalogs and items.

- **`pinterest-pp-cli catalogs available-filter-values`** - Get the available filter attributes and values associated with a given feed or catalog owned by the "operation user_account".
- By default, the "operation user_account" is the token user_account.
- `country`, `language`, and `feed_id` are only used in retail catalogs.
- Note: It is not guaranteed that all available filter values will be returned. Instead this endpoint will return values from a sample of up to 1000 items.

Optional: Business Access: Specify an `ad_account_id` (obtained via [List ad accounts](/docs/api/v5/#operation/ad_accounts/list)) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following [Business Access](https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts) roles on the ad_account: Owner, Admin, Catalogs Manager.

[Learn more](/docs/api-features/shopping-overview/)
- **`pinterest-pp-cli catalogs create`** - Create a new catalog owned by the "operation user_account".
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an ad_account_id (obtained via [List ad accounts](/docs/api/v5/#operation/ad_accounts/list)) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following [Business Access](https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts) roles on the ad_account: Owner, Admin, Catalogs Manager.

[Learn more](/docs/api-features/shopping-overview/)

Note: Access to the Product and Creative Assets catalog type is restricted to a specific group of users. If you require access, please reach out to your partner manager.
- **`pinterest-pp-cli catalogs feed-processing-results-list`** - Fetch a feed processing results owned by the "operation user_account". Please note that for now the bookmark parameter is not functional and only the first page will be available until it is implemented in some release in the near future.
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an `ad_account_id` (obtained via [List ad accounts](/docs/api/v5/#operation/ad_accounts/list)) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following [Business Access](https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts) roles on the ad_account: Owner, Admin, Catalogs Manager.

[Learn more](/docs/api-features/shopping-overview/)
- **`pinterest-pp-cli catalogs feeds-create`** - Create a new feed owned by the "operation user_account".
- By default, the "operation user_account" is the token user_account.

Please, be aware that "default_country"
and "default_locale" are not required in the spec for forward compatibility
but for now the API will not accept requests without those fields.

Optional: Business Access: Specify an `ad_account_id` (obtained via [List ad accounts](/docs/api/v5/#operation/ad_accounts/list)) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following [Business Access](https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts) roles on the ad_account: Owner, Admin, Catalogs Manager.

For Retail partners, refer to [Before you get started with Catalogs](https://help.pinterest.com/en/business/article/before-you-get-started-with-catalogs). For Hotel partners, refer to [Pinterest API for shopping](/docs/api-features/shopping-overview/).

Note: Access to the Creative Assets catalog type is restricted to a specific group of users.
If you require access, please reach out to your partner manager.
- **`pinterest-pp-cli catalogs feeds-delete`** - Delete a feed owned by the "operation user_account".
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an `ad_account_id` (obtained via [List ad accounts](/docs/api/v5/#operation/ad_accounts/list)) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following [Business Access](https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts) roles on the ad_account: Owner, Admin, Catalogs Manager.

For Retail partners, refer to [Before you get started with Catalogs](https://help.pinterest.com/en/business/article/before-you-get-started-with-catalogs). For Hotel partners, refer to [Pinterest API for shopping](/docs/api-features/shopping-overview/).
- **`pinterest-pp-cli catalogs feeds-get`** - Get a single feed owned by the "operation user_account".
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an `ad_account_id` (obtained via [List ad accounts](/docs/api/v5/#operation/ad_accounts/list)) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following [Business Access](https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts) roles on the ad_account: Owner, Admin, Catalogs Manager.

For Retail partners, refer to [Before you get started with Catalogs](https://help.pinterest.com/en/business/article/before-you-get-started-with-catalogs). For Hotel partners, refer to [Pinterest API for shopping](/docs/api-features/shopping-overview/).
- **`pinterest-pp-cli catalogs feeds-ingest`** - Ingest items for a given feed owned by the "operation user_account".

Optional: Business Access: Specify an `ad_account_id` (obtained via [List ad accounts](/docs/api/v5/#operation/ad_accounts/list)) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following [Business Access](https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts) roles on the ad_account: Owner, Admin, Catalogs Manager.

[Learn more](/docs/api-features/shopping-overview/)

Note: This endpoint is restricted to a specific group of users. If you require access, please reach out to your partner manager.
- **`pinterest-pp-cli catalogs feeds-list`** - Fetch feeds owned by the "operation user_account".
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an `ad_account_id` (obtained via [List ad accounts](/docs/api/v5/#operation/ad_accounts/list)) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following [Business Access](https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts) roles on the ad_account: Owner, Admin, Catalogs Manager.

For Retail partners, refer to [Before you get started with Catalogs](https://help.pinterest.com/en/business/article/before-you-get-started-with-catalogs). For Hotel partners, refer to [Pinterest API for shopping](/docs/api-features/shopping-overview/).
- **`pinterest-pp-cli catalogs feeds-update`** - Update a feed owned by the "operation user_account".
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an `ad_account_id` (obtained via [List ad accounts](/docs/api/v5/#operation/ad_accounts/list)) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following [Business Access](https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts) roles on the ad_account: Owner, Admin, Catalogs Manager.

For Retail partners, refer to [Before you get started with Catalogs](https://help.pinterest.com/en/business/article/before-you-get-started-with-catalogs). For Hotel partners, refer to [Pinterest API for shopping](/docs/api-features/shopping-overview/).

Note: Access to the Creative Assets catalog type is restricted to a specific group of users.
If you require access, please reach out to your partner manager.
- **`pinterest-pp-cli catalogs items-batch-get`** - Get a single catalogs items batch owned by the "operating user_account". [See detailed documentation here.](/docs/api-features/shopping-overview/#Update%20items%20in%20batch)
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an `ad_account_id` (obtained via [List ad accounts](/docs/api/v5/#operation/ad_accounts/list)) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following [Business Access](https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts) roles on the ad_account: Owner, Admin, Catalogs Manager.
- **`pinterest-pp-cli catalogs items-batch-post`** - This endpoint supports multiple operations on a set of one or more catalog items owned by the "operation user_account". [See detailed documentation here.](/docs/work-with-catalogs/modify-items-in-batch/)
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an `ad_account_id` (obtained via [List ad accounts](/docs/api/v5/#operation/ad_accounts/list)) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following [Business Access](https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts) roles on the ad_account: Owner, Admin, Catalogs Manager.

Note:
- Access to the Creative Assets catalog type is restricted to a specific group of users.
If you require access, please reach out to your partner manager.
- The item UPSERT operation is restricted to users without a feed data source. If you plan to migrate item ingestion from feeds to the API, please reach out to your partner manager or via the Help Center to get assistance.
- **`pinterest-pp-cli catalogs items-issues-list`** - List item validation issues for a given feed processing result owned by the "operation user_account". Up to 20 random samples of affected items are returned for each error and warning code. Please note that for now query parameters 'item_numbers' and 'item_validation_issue' cannot be used simultaneously until it is implemented in some release in the future.
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an `ad_account_id` (obtained via [List ad accounts](/docs/api/v5/#operation/ad_accounts/list)) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following [Business Access](https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts) roles on the ad_account: Owner, Admin, Catalogs Manager.

Note: To get a list of all affected items instead of sampled issues, please refer to [Build catalogs report](/docs/api/v5/#operation/reports/create) and [Get catalogs report](/docs/api/v5/#operation/reports/get) endpoints. Moreover, they support multiple types of catalogs.

[Learn more](/docs/api-features/shopping-overview/)
- **`pinterest-pp-cli catalogs items-post`** - Get the items of the catalog owned by the "operation user_account". [See detailed documentation here.](/docs/api-features/shopping-overview/#Update%20items%20in%20batch)
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an `ad_account_id` (obtained via [List ad accounts](/docs/api/v5/#operation/ad_accounts/list)) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following [Business Access](https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts) roles on the ad_account: Owner, Admin, Catalogs Manager.

Note: Access to the Creative Assets catalog type is restricted to a specific group of users.
If you require access, please reach out to your partner manager.
- **`pinterest-pp-cli catalogs list`** - Fetch catalogs owned by the "operation user_account".
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an ad_account_id (obtained via [List ad accounts](/docs/api/v5/#operation/ad_accounts/list)) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following [Business Access](https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts) roles on the ad_account: Owner, Admin, Catalogs Manager.

[Learn more](/docs/api-features/shopping-overview/)
- **`pinterest-pp-cli catalogs product-group-pins-list`** - Get a list of product pins for a given Catalogs Product Group Id owned by the "operation user_account".
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an `ad_account_id` (obtained via [List ad accounts](/docs/api/v5/#operation/ad_accounts/list)) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following [Business Access](https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts) roles on the ad_account: Owner, Admin, Catalogs Manager.

[Learn more](/docs/api-features/shopping-overview/)
- **`pinterest-pp-cli catalogs product-groups-create`** - Create product group to use in Catalogs owned by the "operation user_account".
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an `ad_account_id` (obtained via [List ad accounts](/docs/api/v5/#operation/ad_accounts/list)) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following [Business Access](https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts) roles on the ad_account: Owner, Admin, Catalogs Manager.
"Catalog-based product groups" can include items from all data sources (feeds and API) and are available to both non-retail catalogs with any data sources and retail catalogs with API-created items. If your catalog only contains retail items created via feeds, you should use the "retail feed-based" option.
[Learn more](/docs/api-features/shopping-overview/)

Note: Access to the Creative Assets catalog type is restricted to a specific group of users.
If you require access, please reach out to your partner manager.
- **`pinterest-pp-cli catalogs product-groups-create-many`** - Create product group to use in Catalogs owned by the "operation user_account".
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an `ad_account_id` (obtained via [List ad accounts](/docs/api/v5/#operation/ad_accounts/list)) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following [Business Access](https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts) roles on the ad_account: Owner, Admin, Catalogs Manager.

[Learn more](/docs/api-features/shopping-overview/)

Note: Access to the Creative Assets catalog type is restricted to a specific group of users.
If you require access, please reach out to your partner manager.
- **`pinterest-pp-cli catalogs product-groups-delete`** - Delete a product group owned by the "operation user_account" from being in use in Catalogs.
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an `ad_account_id` (obtained via [List ad accounts](/docs/api/v5/#operation/ad_accounts/list)) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following [Business Access](https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts) roles on the ad_account: Owner, Admin, Catalogs Manager.

[Learn more](/docs/api-features/shopping-overview/)
- **`pinterest-pp-cli catalogs product-groups-delete-many`** - Delete product groups owned by the "operation user_account".
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an `ad_account_id` (obtained via [List ad accounts](/docs/api/v5/#operation/ad_accounts/list)) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following [Business Access](https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts) roles on the ad_account: Owner, Admin, Catalogs Manager.

[Learn more](/docs/api-features/shopping-overview/)
- **`pinterest-pp-cli catalogs product-groups-get`** - Get a single product group for a given Catalogs Product Group Id owned by the "operation user_account".
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an `ad_account_id` (obtained via [List ad accounts](/docs/api/v5/#operation/ad_accounts/list)) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following [Business Access](https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts) roles on the ad_account: Owner, Admin, Catalogs Manager.

[Learn more](/docs/api-features/shopping-overview/)
- **`pinterest-pp-cli catalogs product-groups-list`** - Get a list of product groups for a given Catalogs Feed Id owned by the "operation user_account".
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an `ad_account_id` (obtained via [List ad accounts](/docs/api/v5/#operation/ad_accounts/list)) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following [Business Access](https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts) roles on the ad_account: Owner, Admin, Catalogs Manager.

[Learn more](/docs/api-features/shopping-overview/)
- **`pinterest-pp-cli catalogs product-groups-product-counts-get`** - Get a product counts for a given Catalogs Product Group owned by the "operation user_account".
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an `ad_account_id` (obtained via [List ad accounts](/docs/api/v5/#operation/ad_accounts/list)) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following [Business Access](https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts) roles on the ad_account: Owner, Admin, Catalogs Manager.

[Learn more](/docs/api-features/shopping-overview/)
- **`pinterest-pp-cli catalogs product-groups-update`** - Update product group owned by the "operation user_account" to use in Catalogs.
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an `ad_account_id` (obtained via [List ad accounts](/docs/api/v5/#operation/ad_accounts/list)) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following [Business Access](https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts) roles on the ad_account: Owner, Admin, Catalogs Manager.
"Catalog-based product groups" can include items from all data sources (feeds and API) and are available to both non-retail catalogs with any data sources and retail catalogs with API-created items. If your catalog only contains retail items created via feeds, you should use the "retail feed-based" option.
[Learn more](/docs/api-features/shopping-overview/)

Note: Access to the Creative Assets catalog type is restricted to a specific group of users.
If you require access, please reach out to your partner manager.
- **`pinterest-pp-cli catalogs products-by-product-group-filter-list`** - List products Pins owned by the "operation user_account" that meet the criteria specified in the Catalogs Product Group Filter given in the request.
- This endpoint has been implemented in POST to allow for complex filters. This specific POST endpoint is designed to be idempotent.
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an `ad_account_id` (obtained via [List ad accounts](/docs/api/v5/#operation/ad_accounts/list)) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following [Business Access](https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts) roles on the ad_account: Owner, Admin, Catalogs Manager.

Note: This endpoint only supports RETAIL catalog at the moment.

[Learn more](/docs/api-features/shopping-overview/)
- **`pinterest-pp-cli catalogs reports-create`** - Async request to create a report of the catalog owned by the "operation user_account". This endpoint generates a report upon receiving the first approved request of the day. Any following requests with identical parameters will yield the same report even if data has changed.
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an `ad_account_id` (obtained via [List ad accounts](/docs/api/v5/#operation/ad_accounts/list)) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following [Business Access](https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts) roles on the ad_account: Owner, Admin, Catalogs Manager.

Note: The All Items report is limited to 25 million items per catalog.
- **`pinterest-pp-cli catalogs reports-get`** - This returns a URL to a report given a token returned from [Build catalogs report](/docs/api/v5/#operation/reports/create). You can use the URL to download the report.
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an `ad_account_id` (obtained via [List ad accounts](/docs/api/v5/#operation/ad_accounts/list)) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following [Business Access](https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts) roles on the ad_account: Owner, Admin, Catalogs Manager.
- **`pinterest-pp-cli catalogs reports-stats`** - List aggregated numbers of issues for a catalog owned by the "operation user_account".
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an `ad_account_id` (obtained via [List ad accounts](/docs/api/v5/#operation/ad_accounts/list)) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following [Business Access](https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts) roles on the ad_account: Owner, Admin, Catalogs Manager.

### integrations

View, create, or update commerce integrations.

- **`pinterest-pp-cli integrations commerce-del`** - Delete commerce integration metadata for the given external business ID. Note: If you're interested in joining the beta, please reach out to your Pinterest account manager.
- **`pinterest-pp-cli integrations commerce-get`** - Get commerce integration metadata associated with the given external business ID. Note: If you're interested in joining the beta, please reach out to your Pinterest account manager.
- **`pinterest-pp-cli integrations commerce-patch`** - Update commerce integration metadata for the given external business ID. Note: If you're interested in joining the beta, please reach out to your Pinterest account manager.
- **`pinterest-pp-cli integrations commerce-post`** - Create commerce integration metadata to link an external business ID with a Pinterest merchant & ad account. Note: If you're interested in joining the beta, please reach out to your Pinterest account manager.
- **`pinterest-pp-cli integrations get-by-id`** - Get integration metadata by ID.
Note: If you're interested in joining the beta, please reach out to your Pinterest account manager.
- **`pinterest-pp-cli integrations get-list`** - Get integration metadata list.
Note: If you're interested in joining the beta, please reach out to your Pinterest account manager.
- **`pinterest-pp-cli integrations logs-post`** - This endpoint receives batched logs from integration applications on partner platforms.
Note: If you're interested in joining the beta, please reach out to your Pinterest account manager.

### media

Register and manage media uploads.

- **`pinterest-pp-cli media create`** - Register your intent to upload media.

The response includes all of the information needed to upload the media to Pinterest.

To upload the media, make an HTTP POST request (using `curl`, for example) to `upload_url` using the `Content-Type` header value. Send the media file's contents as the request's `file` parameter and also include all of the parameters from `upload_parameters`.

**[Learn more](/docs/api-features/creating-boards-and-pins/#creating-video-pins)** about video Pin creation.
- **`pinterest-pp-cli media get`** - Get details for a registered media upload, including its current status.

**[Learn more](/docs/api-features/creating-boards-and-pins/#creating-video-pins)** about video Pin creation.
- **`pinterest-pp-cli media list`** - List media uploads filtered by given parameters.

**[Learn more](/docs/api-features/creating-boards-and-pins/#creating-video-pins)** about video Pin creation.

### notifications

Send notifications.

- **`pinterest-pp-cli notifications`** - Used by third-party partners to send notifications to Pinterest. These notifications could be specific for your use-case or generic notification that are accepted by Pinterests' systems. This API is gated and you need to request access to this feature.

### oauth

Generate and refresh OAuth access tokens.

- **`pinterest-pp-cli oauth conversion-token`** - Generate a new and long-lived OAuth access token dedicated for sending conversions using a valid access token.
- **`pinterest-pp-cli oauth token`** - Generate a new OAuth access token using an authorization code; or refresh an existing one using a continuous refresh token.

Follow the complete steps for [requesting and refreshing tokens](/docs/getting-started/set-up-authentication-and-authorization/).

**Note:** If your app was created **before September 25, 2025**, make sure to set the `continuous_refresh` parameter to `true` to use the continuous refresh token (60-day expiration, refreshable indefinitely). Pinterest no longer supports the legacy refresh token (365-day expiration, hard limit).

Disregard this note if your app was activated on or after September 25, 2025. You are automatically using the continuous refresh token.

Use [Token Debugger](/docs/developer-tools/token-debugger/) to validate and inspect your access token.
- **`pinterest-pp-cli oauth token-revoke`** - Revokes an access or refresh token. Only tokens issued for system users are currently supported. Revoked tokens become immediately invalid and unusable.

### pins

View, create, update, or delete information about Pins.

- **`pinterest-pp-cli pins create`** - Create a Pin on a board or board section owned by the "operation user_account".

 Note: If the current "operation user_account" (defined by the access token) has access to another user's Ad Accounts via Pinterest Business Access, you can modify your request to make use of the current operation_user_account's permissions to those Ad Accounts by including the ad_account_id in the path parameters for the request (e.g. .../?ad_account_id=12345&...).

- This function is intended solely for publishing new content created by the user. If you are interested in saving content created by others to your Pinterest boards, sometimes called 'curated content', please use our [Save button](/docs/web-features/add-ons-overview/) instead. For more tips on creating fresh content for Pinterest, review our [Content App Solutions Guide](/docs/api-features/content-overview/).

**[Learn more](/docs/api-features/creating-boards-and-pins/#creating-video-pins)** about video Pin creation.

**[Learn more](/docs/api-features/creating-boards-and-pins/#creating-image-pins)** about image Pin creation.
- **`pinterest-pp-cli pins delete`** - Delete a Pins owned by the "operation user_account" - or on a group board that has been shared with this account.
  - By default, the "operation user_account" is the token user_account.

  Optional: Business Access: Specify an `ad_account_id` (obtained via [List ad accounts](/docs/api/v5/#operation/ad_accounts/list)) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following [Business Access](https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts) roles on the ad_account:

  - For Pins on public or protected boards: Owner, Admin, Analyst, Campaign Manager.
  - For Pins on secret boards: Owner, Admin.
- **`pinterest-pp-cli pins get`** - Get a Pin owned by the "operation user_account" - or on a group board that has been shared with this account.
  - By default, the "operation user_account" is the token user_account.

  Optional: Business Access: Specify an `ad_account_id` (obtained via [List ad accounts](/docs/api/v5/#operation/ad_accounts/list)) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following [Business Access](https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts) roles on the ad_account:

  - For Pins on public or protected boards: Owner, Admin, Analyst, Campaign Manager.
  - For Pins on secret boards: Owner, Admin.
- **`pinterest-pp-cli pins list`** - Get a list of the Pins owned by the "operation user_account".
    - By default, the "operation user_account" is the token user_account.
    - All Pins owned by the "operation user_account" are included, regardless of who owns the board they are on.

    Optional: Business Access: Specify an `ad_account_id` to use the owner of that ad_account as the "operation user_account".

    Disclaimer: There are known performance issues when filtering by field `creative_type` and including protected pins.
    If your request is timing out in this scenario, we encourage you to use [GET List Pins on Board](/docs/api/v5/#operation/boards/list_pins).
- **`pinterest-pp-cli pins multi-analytics`** - **This endpoint is currently in beta and not available to all apps. [Learn more](/docs/getting-started/using-beta-and-restricted-features/).**

Get analytics for multiple pins owned by the "operation user_account" - or on a group board that has been shared with this account.
- The maximum number of pins supported in a single request is 100.
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an `ad_account_id` (obtained via [List ad accounts](/docs/api/v5/#operation/ad_accounts/list)) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following [Business Access](https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts) roles on the ad_account:

- For Pins on public or protected boards: Admin, Analyst.
- For Pins on secret boards: Admin.

If Pin was created before `2023-03-20` lifetime metrics will only be available for Video and Idea Pin formats. Lifetime metrics are available for all Pin formats since then.
- **`pinterest-pp-cli pins update`** - Update a pin owned by the "operating user_account".
- By default, the "operation user_account" is the token user_account.

Optional: Business Access: Specify an `ad_account_id` (obtained via [List ad accounts](/docs/api/v5/#operation/ad_accounts/list)) to use the owner of that ad_account as the "operation user_account". In order to do this, the token user_account must have one of the following [Business Access](https://help.pinterest.com/en/business/article/share-and-manage-access-to-your-ad-accounts) roles on the ad_account:

- For Pins on public or protected boards: Owner, Admin, Analyst, Campaign Manager.
- For Pins on secret boards: Owner, Admin.

**This endpoint is currently in beta and not available to all apps. [Learn more](/docs/getting-started/using-beta-and-restricted-features/).**

### pinterest-search

Manage pinterest search

- **`pinterest-pp-cli pinterest-search partner-pins`** - **This endpoint is currently in beta and not available to all apps. [Learn more](/docs/getting-started/using-beta-and-restricted-features/).**

Get the top 10 Pins by a given search term.
- **`pinterest-pp-cli pinterest-search user-boards-get`** - Search for boards for the "operation user_account". This includes boards of all board types.
- By default, the "operation user_account" is the token user_account.

If using Business Access: Specify an ad_account_id to use the owner of that ad_account as the "operation user_account". See [Understanding Business Access](/docs/getting-started/using-business-access/) for more information.
- **`pinterest-pp-cli pinterest-search user-pins-list`** - Search for pins for the "operation user_account".
- By default, the "operation user_account" is the token user_account.

If using Business Access: Specify an ad_account_id to use the owner of that ad_account as the "operation user_account". See [Understanding Business Access](/docs/getting-started/using-business-access/) for more information.

### resources

View metadata about available metrics and targeting options in the Pinterest API.

- **`pinterest-pp-cli resources ad-account-countries-get`** - Get Ad Accounts countries
- **`pinterest-pp-cli resources delivery-metrics-get`** - Get the definitions for ads and organic metrics available across both synchronous and asynchronous report endpoints.
The `display_name` attribute will match how the metric is named in our native tools like Ads Manager.
See [Organic Analytics](/docs/api-features/analytics-overview/) and [Ads Analytics](/docs/api-features/ads-reporting/) for more information.
- **`pinterest-pp-cli resources interest-targeting-options-get`** - Get details of a specific interest given interest ID.

Click [here](https://docs.google.com/spreadsheets/d/1HxL-0Z3p2fgxis9YBP2HWC3tvPrs1hAuHDRtH-NJTIM/edit#gid=118370875) for a spreadsheet listing interests and their IDs.
- **`pinterest-pp-cli resources lead-form-questions-get`** - Get a list of all lead form question type names. Some questions might not be used.

**This endpoint is currently in beta and not available to all apps. [Learn more](/docs/getting-started/using-beta-and-restricted-features/).**
- **`pinterest-pp-cli resources metrics-ready-state-get`** - Learn whether conversion or non-conversion metrics are finalized and ready to query.
- **`pinterest-pp-cli resources targeting-options-get`** - You can use targeting values in ads placement to define your intended audience.

    Targeting metrics are organized around targeting specifications.

    For more information on ads targeting, see [Audience targeting](https://help.pinterest.com/en/business/article/audience-targeting).

    **Sample return:**

    ```
    [{"36313": "Australia: Moreton Bay - North", "124735": "Canada: North Battleford", "36109": "Australia: Murray", "36108": "Australia: Mid North Coast", "36101": "Australia: Capital Region", "811": "U.S.: Reno", "36103": "Australia: Central West", "36102": "Australia: Central Coast", "36105": "Australia: Far West and Orana", "36104": "Australia: Coffs Harbour - Grafton", "36107": "Australia: Illawarra", "36106": "Australia: Hunter Valley Exc Newcastle", "554017": "New Zealand: Wanganui", "554016": "New Zealand: Marlborough", "554015": "New Zealand: Gisborne", "554014": "New Zealand: Tararua", "554013": "New Zealand: Invercargill", "GR": "Greece", "554011": "New Zealand: Whangarei", "554010": "New Zealand: Far North", "717": "U.S.: Quincy-Hannibal-Keokuk", "716": "U.S.: Baton Rouge",...}]
    ```

### terms

View related and suggested terms for ads targeting.

- **`pinterest-pp-cli terms related-list`** - Get a list of terms logically related to each input term.

Example: the term 'workout' would list related terms like 'one song workout', 'yoga workout', 'workout motivation', etc.
- **`pinterest-pp-cli terms suggested-list`** - Get popular search terms that begin with your input term.

Example: 'sport' would return popular terms like 'sports bar' and 'sportswear', but not 'motor sports' since the phrase does not begin with the given term.

### trends

View trending terms, topics, and product categories.

- **`pinterest-pp-cli trends editorial-articles-list`** - Get a list of published editorial articles. Translations of the editorials will be provided if available; otherwise, the default language will be English.
- **`pinterest-pp-cli trends featured-topics-list`** - Enables advertisers to pull top five trending topics by interest and market, at full parity with the Pinterest Trends UI.
- **`pinterest-pp-cli trends product-categories-details-list`** - Enables advertisers to retrieve demographic information, related pins, and trend lines for specified product categories
- **`pinterest-pp-cli trends product-categories-trending-list`** - Get a list of growing Shopping Product Categories in ranked order allowing filtering by engagement type, vertical, age, and gender.
- **`pinterest-pp-cli trends trending-keywords-list`** - Get the top trending search keywords among the Pinterest user audience.

Trending keywords can be used to inform ad targeting, budget strategy, and creative decisions about which products and Pins will resonate with your audience.

Geographic, demographic and interest-based filters are available to narrow down to the top trends among a specific audience. Multiple trend types are supported that can be used to identify newly-popular, evergreen or seasonal keywords.

For an interactive way to explore this data, please visit [trends.pinterest.com](https://trends.pinterest.com).

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
- **`pinterest-pp-cli user-account follow-user-update`** - **This endpoint is currently in beta and not available to all apps. [Learn more](/docs/getting-started/using-beta-and-restricted-features/).**

Use this request, as a signed-in user, to follow another user.
- **`pinterest-pp-cli user-account followers-list`** - Get a list of your followers.
- **`pinterest-pp-cli user-account get`** - Get account information for the "operation user_account"
- By default, the "operation user_account" is the token user_account.

[Understanding Business Access]: https://developers.pinterest.com/docs/getting-started/using-business-access/ "Understanding Business Access"
If using Business Access: Specify an ad_account_id to use the owner of that ad_account as the "operation user_account". See [Understanding Business Access] for more information.
- **`pinterest-pp-cli user-account linked-business-accounts-get`** - Get a list of your linked business accounts.
- **`pinterest-pp-cli user-account unverify-website-delete`** - Unverify a website verified by the signed-in user.
- **`pinterest-pp-cli user-account user-following-get`** - Get a list of who a certain user follows.
- **`pinterest-pp-cli user-account user-websites-get`** - Get user websites, claimed or not
- **`pinterest-pp-cli user-account verify-website-update`** - Verify a website as a signed-in user.
- **`pinterest-pp-cli user-account website-verification-get`** - Get verification code for user to install on the website to claim it.

### users

Manage users

- **`pinterest-pp-cli users interests user-account-followed`** - List the interests a user follows.


### ads (workflow commands)

Async ads report in one command.

- **`pinterest-pp-cli ads report`** - Create, wait for, and download an ads performance report in one command. Requires the ads:read scope and --ad-account-id (or PINTEREST_AD_ACCOUNT_ID).


### perf (workflow commands)

Account performance comparisons.

- **`pinterest-pp-cli perf compare`** - See whether impressions, saves, and outbound clicks grew versus the previous equal-length period, with absolute and percent deltas.


### pins (workflow commands)


- **`pinterest-pp-cli pins audit`** - Find published Pins missing alt text, a destination link, a title, or UTM tracking, grouped by board. Reads the local store; run sync first.

- **`pinterest-pp-cli pins preflight`** - Validate a proposed Pin (field limits, board and section by name, UTM link) and get a ready-to-send create body without writing anything. Board names resolve from the local store; run sync first.

- **`pinterest-pp-cli pins funnel`** - Rank your top Pins by impressions (or --sort-by metric), compute save and outbound-click rates, and flag Pins that get saved but don't send traffic.


### trends (workflow commands)


- **`pinterest-pp-cli trends gaps`** - List trending keywords for a region with how many of your synced Pins cover each, zero-coverage first. Coverage counts need a synced local store.




### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`pinterest-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`pinterest-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`pinterest-pp-cli learnings list`** - Inspect taught rows
- **`pinterest-pp-cli learnings forget <query>`** - Undo a teach
- **`pinterest-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`pinterest-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`pinterest-pp-cli teach-pattern`** - Install a query/resource template up front
- **`pinterest-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `PINTEREST_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `pinterest-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
pinterest-pp-cli ad-accounts list

# JSON for scripting and agents
pinterest-pp-cli ad-accounts list --json
# Filter to specific fields
pinterest-pp-cli ad-accounts list --json --select bookmark,items

# Dry run — show the request without sending
pinterest-pp-cli ad-accounts list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
pinterest-pp-cli ad-accounts list --agent
```

`--csv` prefixes a leading `'` on cells and header keys that start with `=`, `+`, `-`, `@`, a tab, or a line break (including fullwidth `＝＋－＠`) so a spreadsheet does not run them as formulas. Plain signed numbers such as `-5` and `+2.5e3` are left unchanged.

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Explicit confirmation** - `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `6` partial failure, `7` rate limited, `10` config error.

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `PINTEREST_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths (bare `sync` refreshes boards, pins, sections, and board pins; other resources need `sync --resources ...`):
- `pinterest-pp-cli ad-accounts`
- `pinterest-pp-cli ad-accounts get`
- `pinterest-pp-cli ad-accounts list`
- `pinterest-pp-cli boards`
- `pinterest-pp-cli boards get`
- `pinterest-pp-cli boards list`
- `pinterest-pp-cli businesses`
- `pinterest-pp-cli catalogs`
- `pinterest-pp-cli catalogs list`
- `pinterest-pp-cli integrations`
- `pinterest-pp-cli media`
- `pinterest-pp-cli media get`
- `pinterest-pp-cli media list`
- `pinterest-pp-cli pins`
- `pinterest-pp-cli pins get`
- `pinterest-pp-cli pins list`
- `pinterest-pp-cli resources`
- `pinterest-pp-cli terms`
- `pinterest-pp-cli trends`
- `pinterest-pp-cli user-account`
- `pinterest-pp-cli user-account get`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Health Check

```bash
pinterest-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `pinterest-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/pinterest-pp-cli/config.toml` on macOS/Linux and `%USERPROFILE%\.config\pinterest-pp-cli\config.toml` on Windows; `--home`, `PINTEREST_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

`--ca-cert`, `PINTEREST_CA_CERT`, or `ca_cert` adds a PEM CA bundle to the system trust roots. `--tls-server-name`, `PINTEREST_TLS_SERVER_NAME`, or `tls_server_name` is the name checked against the certificate when the dialed address does not match it. Precedence is flag, then environment, then the config file. A config file other users can change does not supply these settings. Certificate verification failures are not retried.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `PINTEREST_ACCESS_TOKEN` | per_call | No | Set to your API credential. |
| `PINTEREST_OAUTH2` | per_call | No | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `pinterest-pp-cli doctor` reports `agentcookie: detected` and `pinterest-pp-cli auth status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `pinterest-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $PINTEREST_ACCESS_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized after a few weeks** — Run pinterest-pp-cli auth status; if the refresh token expired, re-run pinterest-pp-cli auth login
- **redirect_uri mismatch during login** — Pass the exact registered callback, e.g. pinterest-pp-cli auth login --port 8875
- **invalid scopes requested during login** — Default login requests the standard scopes (not msot:write). If your app lacks some, pick a smaller set: pinterest-pp-cli auth login --scopes boards:read,pins:read,user_accounts:read. Add msot:write only for MSOT-enabled apps.
- **429 Too Many Requests on trial access** — Trial apps are capped at 1000 requests/day; use the local store (search, pins audit) and sync with --max-pages

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**pinterest-mcp**](https://github.com/clugtu/pinterest-mcp) — Python (7 stars)
- [**sapyx-mcp**](https://github.com/sapyx/sapyx-mcp) — JavaScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
