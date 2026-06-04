# Goodreads CLI

A complete read+write CLI + MCP for Goodreads: rate books via the modern AppSync GraphQL API
(RateBook/UnrateBook), write reviews with spoiler and feed-publicize via the legacy form, add to
and create shelves, browse the home updates feed, friends, recommendations, genre/topic search,
book pages, and Goodreads Giveaways. Goodreads has no current public developer API, so these
routes are reverse-engineered from the logged-in web app and verified live.

Evidence was captured on 2026-05-22 with browser-harness-js. Raw artifacts live under
goodreads/proofs/ and are mode 0600 because they contain account-visible metadata.

This spec intentionally does not include cookies, CSRF tokens, request headers, response bodies,
or raw Kindle highlight text.

Learn more at [Goodreads](https://www.goodreads.com).

Created by [@zaydiscold](https://github.com/zaydiscold) (zaydiscold).

## Install

The recommended path installs both the `goodreads-pp-cli` binary and the `pp-goodreads` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install goodreads
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install goodreads --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install goodreads --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install goodreads --agent claude-code
npx -y @mvanhorn/printing-press-library install goodreads --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/goodreads/cmd/goodreads-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/goodreads-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-goodreads --force
```

Inside a Hermes chat session:

```text
/skills install mvanhorn/printing-press-library/cli-skills/pp-goodreads --force
```

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install goodreads --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/goodreads-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `GOODREADS_GOODREADS_COOKIE_SESSION` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "goodreads": {
      "command": "goodreads-pp-mcp",
      "env": {
        "GOODREADS_GOODREADS_COOKIE_SESSION": "<your-goodreads-session-cookie>"
      }
    }
  }
}
```

</details>

## MCP Surface

The MCP server defaults to a compact code-orchestration shape:

- `goodreads_search` finds profile, bookshelf, notes/highlights, messages, quotes, friends, people, genre, and account-action routes.
- `goodreads_execute` invokes one discovered route by `endpoint_id`.
- Mutating Goodreads routes remain blocked unless `GOODREADS_PP_ALLOW_WRITES=1` is set after explicit approval.

The binary serves stdio by default:

```bash
goodreads-pp-mcp
```

For hosted agents, use streamable HTTP:

```bash
goodreads-pp-mcp --transport http --addr :7777
```

Set `GOODREADS_MCP_ENDPOINT_MIRRORS=1` only when debugging the older raw endpoint mirror surface.

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Goodreads no longer offers new public developer API keys. For account-specific pages, use the value of your authenticated Goodreads `_session_id2` browser cookie. Do not paste broad Amazon, AWS, or unrelated browser cookies.

```bash
export GOODREADS_GOODREADS_COOKIE_SESSION="<your-goodreads-session-cookie>"
```

You can also persist this in your config file at `~/.config/goodreads-web-undocumented-pp-cli/config.toml`.

### 3. Verify Setup

```bash
goodreads-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
goodreads-pp-cli amazon-purchases
```

## Usage

Run `goodreads-pp-cli --help` for the full command reference and flag list.

## Commands

### amazon-purchases

Manage amazon purchases

- **`goodreads-pp-cli amazon-purchases`** - Amazon purchases import/inspection page.

### book

Manage book

- **`goodreads-pp-cli book <book_slug>`** - Book detail page. Current page is Next.js-backed.

### comment

Manage comment

- **`goodreads-pp-cli comment <user_slug>`** - User comments/recent posts page.

### feed

Read the home updates feed (friends' activity)

- **`goodreads-pp-cli feed list`** - Read the home updates feed (friends' activity). Use `--page <n>` for older activity. Read-only POST to `/home/load_more_updates`.

### friend

Explore Goodreads friends and friend requests

- **`goodreads-pp-cli friend list`** - Friends index page.
- **`goodreads-pp-cli friend list-requests`** - Friend requests page.

### genres

Browse Goodreads genre and shelf discovery pages

- **`goodreads-pp-cli genres get`** - Genre landing page.
- **`goodreads-pp-cli genres list`** - Genre index page.
- **`goodreads-pp-cli genres list-list`** - Alphabetical genre shelves index.
- **`goodreads-pp-cli genres list-search`** - Genre finder route.

### giveaway

Browse and enter Goodreads Giveaways

- **`goodreads-pp-cli giveaway list`** - Browse the Goodreads Giveaways listing (`--format`, `--genre`).
- **`goodreads-pp-cli giveaway show <giveaway_id>`** - Show one giveaway detail page.
- **`goodreads-pp-cli giveaway enter <giveaway_id>`** - Enter a giveaway. NOT YET LIVE: the entry POST was not captured; live execution is refused, `--dry-run` previews the request.

### goodreads-web-undocumented-search

Manage goodreads web undocumented search

- **`goodreads-pp-cli goodreads-web-undocumented-search`** - Canonical book search route from OpenSearch descriptor.

### list

Manage list

- **`goodreads-pp-cli list <list_id>`** - Public Listopia list page.

### message

Inspect and plan Goodreads message-folder actions

- **`goodreads-pp-cli message create`** - Batch message folder/read action.
- **`goodreads-pp-cli message get`** - User message folder page.
- **`goodreads-pp-cli message get-show`** - Message detail page.
- **`goodreads-pp-cli message list`** - User message inbox.
- **`goodreads-pp-cli message list-markallasread`** - Mark all visible inbox messages read.

### notes

Read and plan Kindle notes/highlights actions

- **`goodreads-pp-cli notes get`** - User Kindle Notes & Highlights index.
- **`goodreads-pp-cli notes get-bookslug`** - Notes/highlights detail page for one book and user.

### notifications

Inspect Goodreads notifications and tracking calls

- **`goodreads-pp-cli notifications create`** - Notification tracking call emitted by the UI.
- **`goodreads-pp-cli notifications list`** - User notifications page.

### opensearch-xml

Manage opensearch xml

- **`goodreads-pp-cli opensearch-xml`** - OpenSearch descriptor for Goodreads book search.

### quotes

Read Goodreads quotes and quote widgets

- **`goodreads-pp-cli quotes get`** - User quotes widget script.
- **`goodreads-pp-cli quotes list`** - Current user's quotes list.

### rating

Set or clear your star rating for a book (GraphQL)

Ratings are written through the modern Goodreads AWS AppSync GraphQL API (`RateBook` / `UnrateBook`), not a legacy form. They require a bound AppSync JWT set via `GOODREADS_GRAPHQL_TOKEN` — a different credential from the session cookie (see Configuration). Writes are gated by `--dry-run` / `GOODREADS_PP_ALLOW_WRITES=1`.

- **`goodreads-pp-cli rating set <book_id> --stars <1-5>`** - Rate a book (RateBook GraphQL mutation).
- **`goodreads-pp-cli rating clear <book_id>`** - Clear your rating (UnrateBook GraphQL mutation).

### recommendations

Inspect Goodreads recommendation pages

- **`goodreads-pp-cli recommendations list`** - Personalized recommendations page.
- **`goodreads-pp-cli recommendations list-tome`** - Friends' recommendations page.

### review

Read and write reviews; plan bookshelf/review table actions

- **`goodreads-pp-cli review create <book_id>`** - Write/update a review (`--review`, `--spoiler`, `--publicize`, `--add-to-blog`, `--shelf`, `--notes`). Posts the legacy `/review/update/:book_id` form (form-urlencoded). The Rails CSRF token comes from the `GOODREADS_AUTHENTICITY_TOKEN` env var or stdin (`--authenticity-token` is deprecated — it leaks into shell history / `ps aux`). Star ratings are GraphQL-only — use `rating set`.
- **`goodreads-pp-cli review create-update <book_id> --stdin`** - Inline review/date/notes field update for one book. Pipe a JSON object of Rails review form fields; sent form-urlencoded to `/review/update/:book_id`. CSRF token via `GOODREADS_AUTHENTICITY_TOKEN`.
- **`goodreads-pp-cli review create-updatelist`** - Batch update selected reviews/books on a user's shelf table.
- **`goodreads-pp-cli review get`** - Bookshelf list for a user, optionally filtered by shelf.
- **`goodreads-pp-cli review get-listrss`** - This public XML route is the cleanest current read API for shelf exports. Authenticated shelf pages
can expose a private keyed RSS URL; do not log or commit that key.
- **`goodreads-pp-cli review get-stats`** - Reading stats page.
- **`goodreads-pp-cli review list`** - Review drafts page.
- **`goodreads-pp-cli review list-duplicates`** - Duplicate books tool.
- **`goodreads-pp-cli review list-import`** - Import/export page.

### shelf

Inspect and plan Goodreads shelf operations

- **`goodreads-pp-cli shelf create`** - Add or remove one book from a shelf through the shelf chooser helper.
- **`goodreads-pp-cli shelf create-movebatch`** - Reorder books/shelves in batch.
- **`goodreads-pp-cli shelf create-movebatch-2`** - Persist shelf-table position changes for a user.
- **`goodreads-pp-cli shelf create-movetoposition`** - Move one shelf/book row to a specific position.
- **`goodreads-pp-cli shelf create-removebook`** - Remove one book from a shelf from the button helper.
- **`goodreads-pp-cli shelf create-update`** - Update shelf display/settings.
- **`goodreads-pp-cli shelf get`** - Public shelf landing page.
- **`goodreads-pp-cli shelf list`** - Top public shelves index.

### tooltips

Manage tooltips

- **`goodreads-pp-cli tooltips`** - Batch tooltip metadata for book ids shown in shelf/list pages.

### topic

Manage topic

- **`goodreads-pp-cli topic`** - Discussions page, optionally filtered to groups.

### user

Inspect Goodreads profile and people pages

- **`goodreads-pp-cli user get`** - Profile delay-loaded sections.
- **`goodreads-pp-cli user get-show`** - User profile page.
- **`goodreads-pp-cli user get-yearinbooks`** - Year in Books page.
- **`goodreads-pp-cli user list`** - People discovery page for most popular reviewers.
- **`goodreads-pp-cli user list-topreaders`** - People discovery page for top readers.
- **`goodreads-pp-cli user list-topreviewers`** - People discovery page for top reviewers.

### user-following

Inspect Goodreads most-followed people discovery

- **`goodreads-pp-cli user-following`** - People discovery page for most-followed users.

### user-shelves

Plan custom Goodreads shelf creation

- **`goodreads-pp-cli user-shelves`** - Create a custom user shelf.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
goodreads-pp-cli amazon-purchases

# JSON for scripting and agents
goodreads-pp-cli amazon-purchases --json

# Filter to specific fields
goodreads-pp-cli amazon-purchases --json --select id,name,status

# Dry run — show the request without sending
goodreads-pp-cli amazon-purchases --dry-run

# Agent mode — JSON + compact + no prompts in one flag
goodreads-pp-cli amazon-purchases --agent
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
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
goodreads-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/goodreads-web-undocumented-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `GOODREADS_GOODREADS_COOKIE_SESSION` | browser cookie | Yes | Goodreads `_session_id2` cookie value from an authenticated browser session. |
| `GOODREADS_GRAPHQL_TOKEN` | AppSync JWT | For `rating` writes only | The `Authorization` header value from a `*.appsync-api.*.amazonaws.com/graphql` request in browser DevTools. Required only for `rating set` / `rating clear`; a different credential from the session cookie. |
| `GOODREADS_AUTHENTICITY_TOKEN` | Rails CSRF token | For `review` / `shelf` form writes | The `authenticity_token` hidden-input value from the relevant edit page (e.g. `/review/edit/:book_id`). Preferred over the deprecated `--authenticity-token` flag, which leaks into shell history and `ps aux`; stdin is also accepted. |
| `GOODREADS_PP_ALLOW_WRITES` | flag (`1`/`true`) | For executing any write | Must be set after explicit approval to let account mutations (review writes, shelf changes, ratings) execute against the live site. Without it, writes are refused; preview with `--dry-run`. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `goodreads-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $GOODREADS_GOODREADS_COOKIE_SESSION`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints and sends the configured Goodreads session as the `_session_id2` cookie. It does not require a resident browser process for normal API calls.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
