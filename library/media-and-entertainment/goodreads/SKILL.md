---
name: pp-goodreads
description: "Printing Press CLI + MCP for Goodreads, read and write — rate books (GraphQL RateBook/UnrateBook), write/publicize reviews, add to and create shelves, home feed, friends, recommendations, genre/topic search, book pages, and Goodreads Giveaways. Reverse-engineered from the logged-in web app (no public API), live-verified."
author: "zaydiscold"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - goodreads-pp-cli
---

# Goodreads — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `goodreads-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer into a user bin directory:
   ```bash
   npx -y @mvanhorn/printing-press-library install goodreads --cli-only --bin-dir ~/.local/bin
   ```
2. Verify: `goodreads-pp-cli --version`
3. Ensure `~/.local/bin` is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/goodreads/cmd/goodreads-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints and sends the configured Goodreads session as the `_session_id2` cookie. It does not require a resident browser process for normal API calls.

## MCP Surface

The MCP binary defaults to stdio and can serve streamable HTTP for hosted agents:

```bash
goodreads-pp-mcp
goodreads-pp-mcp --transport http --addr :7777
```

Default MCP registration exposes `goodreads_search` and `goodreads_execute` for the full route map, plus local `search`, `sql`, and `context`. Raw endpoint mirror tools are opt-in with `GOODREADS_MCP_ENDPOINT_MIRRORS=1`. Mutating routes remain blocked unless `GOODREADS_PP_ALLOW_WRITES=1` is set after explicit approval.

## Command Reference

**amazon-purchases** — Manage amazon purchases

- `goodreads-pp-cli amazon-purchases` — Amazon purchases import/inspection page.

**book** — Manage book

- `goodreads-pp-cli book <book_slug>` — Book detail page. Current page is Next.js-backed.

**comment** — Manage comment

- `goodreads-pp-cli comment <user_slug>` — User comments/recent posts page.

**feed** — Read the home updates feed (friends' activity)

- `goodreads-pp-cli feed list` — Read the home updates feed (friends' activity). Use `--page <n>` to walk older activity. Read-only POST to `/home/load_more_updates`.

**friend** — Explore Goodreads friends and friend requests

- `goodreads-pp-cli friend list` — Friends index page.
- `goodreads-pp-cli friend list-requests` — Friend requests page.

**genres** — Browse Goodreads genre and shelf discovery pages

- `goodreads-pp-cli genres get` — Genre landing page.
- `goodreads-pp-cli genres list` — Genre index page.
- `goodreads-pp-cli genres list-list` — Alphabetical genre shelves index.
- `goodreads-pp-cli genres list-search` — Genre finder route.

**giveaway** — Browse and enter Goodreads Giveaways

- `goodreads-pp-cli giveaway list` — Browse the Goodreads Giveaways listing (`--format`, `--genre`).
- `goodreads-pp-cli giveaway show <giveaway_id>` — Show one giveaway detail page.
- `goodreads-pp-cli giveaway enter <giveaway_id>` — Enter a giveaway. NOT YET LIVE: the entry POST shape was not captured; live execution is refused, `--dry-run` previews the best-known request.

**goodreads-web-undocumented-search** — Manage goodreads web undocumented search

- `goodreads-pp-cli goodreads-web-undocumented-search` — Canonical book search route from OpenSearch descriptor.

**list** — Manage list

- `goodreads-pp-cli list <list_id>` — Public Listopia list page.

**message** — Inspect and plan Goodreads message-folder actions

- `goodreads-pp-cli message create` — Batch message folder/read action.
- `goodreads-pp-cli message get` — User message folder page.
- `goodreads-pp-cli message get-show` — Message detail page.
- `goodreads-pp-cli message list` — User message inbox.
- `goodreads-pp-cli message list-markallasread` — Mark all visible inbox messages read.

**notes** — Read and plan Kindle notes/highlights actions

- `goodreads-pp-cli notes get` — User Kindle Notes & Highlights index.
- `goodreads-pp-cli notes get-bookslug` — Notes/highlights detail page for one book and user.

**notifications** — Inspect Goodreads notifications and tracking calls

- `goodreads-pp-cli notifications create` — Notification tracking call emitted by the UI.
- `goodreads-pp-cli notifications list` — User notifications page.

**opensearch-xml** — Manage opensearch xml

- `goodreads-pp-cli opensearch-xml` — OpenSearch descriptor for Goodreads book search.

**quotes** — Read Goodreads quotes and quote widgets

- `goodreads-pp-cli quotes get` — User quotes widget script.
- `goodreads-pp-cli quotes list` — Current user's quotes list.

**rating** — Set or clear your star rating for a book (GraphQL)

Ratings are written through the modern Goodreads AWS AppSync GraphQL API (`RateBook` / `UnrateBook`), not a legacy form. They need a bound AppSync JWT — a different credential from the session cookie. Set it via `GOODREADS_GRAPHQL_TOKEN` (see Auth Setup). Writes are gated: `--dry-run` to preview, or `GOODREADS_PP_ALLOW_WRITES=1` after approval to execute.

- `goodreads-pp-cli rating set <book_id> --stars <1-5>` — Rate a book (RateBook GraphQL mutation).
- `goodreads-pp-cli rating clear <book_id>` — Clear your rating (UnrateBook GraphQL mutation).

**recommendations** — Inspect Goodreads recommendation pages

- `goodreads-pp-cli recommendations list` — Personalized recommendations page.
- `goodreads-pp-cli recommendations list-tome` — Friends' recommendations page.

**review** — Read and write reviews; plan bookshelf/review table actions

- `goodreads-pp-cli review create <book_id>` — Write/update a review for a book (`--review`, `--spoiler`, `--publicize`, `--add-to-blog`, `--shelf`, `--notes`). Posts the legacy `/review/update/:book_id` form (form-urlencoded). The Rails CSRF token comes from the `GOODREADS_AUTHENTICITY_TOKEN` env var or stdin (the `--authenticity-token` flag is deprecated — it leaks into shell history / `ps aux`). Star ratings are GraphQL-only — use `rating set`.
- `goodreads-pp-cli review create-update <book_id> --stdin` — Inline review/date/notes field update for one book. Pipe a JSON object of Rails review form fields (e.g. `{"review[review]":"..."}`); sent form-urlencoded to `/review/update/:book_id`. CSRF token via `GOODREADS_AUTHENTICITY_TOKEN`.
- `goodreads-pp-cli review create-updatelist` — Batch update selected reviews/books on a user's shelf table.
- `goodreads-pp-cli review get` — Bookshelf list for a user, optionally filtered by shelf.
- `goodreads-pp-cli review get-listrss` — This public XML route is the cleanest current read API for shelf exports.
- `goodreads-pp-cli review get-stats` — Reading stats page.
- `goodreads-pp-cli review list` — Review drafts page.
- `goodreads-pp-cli review list-duplicates` — Duplicate books tool.
- `goodreads-pp-cli review list-import` — Import/export page.

**shelf** — Inspect and plan Goodreads shelf operations

- `goodreads-pp-cli shelf create` — Add or remove one book from a shelf through the shelf chooser helper.
- `goodreads-pp-cli shelf create-movebatch` — Reorder books/shelves in batch.
- `goodreads-pp-cli shelf create-movebatch-2` — Persist shelf-table position changes for a user.
- `goodreads-pp-cli shelf create-movetoposition` — Move one shelf/book row to a specific position.
- `goodreads-pp-cli shelf create-removebook` — Remove one book from a shelf from the button helper.
- `goodreads-pp-cli shelf create-update` — Update shelf display/settings.
- `goodreads-pp-cli shelf get` — Public shelf landing page.
- `goodreads-pp-cli shelf list` — Top public shelves index.

**tooltips** — Manage tooltips

- `goodreads-pp-cli tooltips` — Batch tooltip metadata for book ids shown in shelf/list pages.

**topic** — Manage topic

- `goodreads-pp-cli topic` — Discussions page, optionally filtered to groups.

**user** — Inspect Goodreads profile and people pages

- `goodreads-pp-cli user get` — Profile delay-loaded sections.
- `goodreads-pp-cli user get-show` — User profile page.
- `goodreads-pp-cli user get-yearinbooks` — Year in Books page.
- `goodreads-pp-cli user list` — People discovery page for most popular reviewers.
- `goodreads-pp-cli user list-topreaders` — People discovery page for top readers.
- `goodreads-pp-cli user list-topreviewers` — People discovery page for top reviewers.

**user-following** — Inspect Goodreads most-followed people discovery

- `goodreads-pp-cli user-following` — People discovery page for most-followed users.

**user-shelves** — Plan custom Goodreads shelf creation

- `goodreads-pp-cli user-shelves` — Create a custom user shelf.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
goodreads-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup
Goodreads no longer offers new public developer API keys. Run `goodreads-pp-cli auth setup` to print the URL and steps for configuring a Goodreads browser session cookie (add `--launch` to open Goodreads). Then set only the Goodreads `_session_id2` cookie value:

```bash
export GOODREADS_GOODREADS_COOKIE_SESSION="<your-goodreads-session-cookie>"
```

Or persist it in `~/.config/goodreads-web-undocumented-pp-cli/config.toml`.

Do not paste broad Amazon, AWS, or unrelated browser cookies.

Run `goodreads-pp-cli doctor` to verify setup.

### GraphQL token (rating writes only)

The `rating set` / `rating clear` commands write through the modern Goodreads AWS AppSync GraphQL API, which authenticates with a bound JWT — **a separate credential from the session cookie**. To obtain it:

1. Open `goodreads.com` logged in, open browser DevTools → Network.
2. Click a rating star on any book page; find the request to `*.appsync-api.*.amazonaws.com/graphql`.
3. Copy the value of its `Authorization` request header.
4. `export GOODREADS_GRAPHQL_TOKEN='<that value>'`

The session cookie alone cannot authenticate GraphQL. A mint-from-cookie endpoint was not observed in the live capture, so the env var is the supported path. All other commands (reads, the legacy review form, shelf writes) use only the session cookie.

### Rails CSRF token (review / shelf form writes)

The legacy form writes (`review create`, `review create-update`, `shelf create`) need a Rails CSRF `authenticity_token` from the relevant edit page (e.g. the hidden `authenticity_token` input on `/review/edit/:book_id`). Supply it via the `GOODREADS_AUTHENTICITY_TOKEN` env var (preferred) or pipe it on stdin — both keep it out of shell history and the process table (`ps aux`). The `--authenticity-token` flag still works but is deprecated for that reason.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  goodreads-pp-cli amazon-purchases --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

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
goodreads-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
goodreads-pp-cli feedback --stdin < notes.txt
goodreads-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.goodreads-pp-cli/feedback.jsonl`. They are never POSTed unless `GOODREADS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `GOODREADS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
goodreads-pp-cli profile save briefing --json
goodreads-pp-cli --profile briefing amazon-purchases
goodreads-pp-cli profile list --json
goodreads-pp-cli profile show briefing
goodreads-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `goodreads-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add goodreads-pp-mcp -- goodreads-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which goodreads-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   goodreads-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `goodreads-pp-cli <command> --help`.
