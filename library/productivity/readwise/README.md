# Readwise CLI

**Every Readwise and Reader feature, plus a local mirror of your whole library that you can search, triage and dedupe offline.**

Sync your highlights, books and Reader documents once into a local SQLite store, then query them without the 20-requests-a-minute list limit. Ask what you have already read on a topic with `ask`, plan a reading slot with `backlog` and `read-next`, and clean up with `dedupe` and `triage stale`. Writes (save, tag, archive, bulk update) still go straight to the API, with dry runs first.

## Install

The recommended path installs both the `readwise-pp-cli` binary and the `pp-readwise` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install readwise
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install readwise --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install readwise --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install readwise --agent claude-code
npx -y @mvanhorn/printing-press-library install readwise --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.11 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/readwise/cmd/readwise-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/readwise-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install readwise --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-readwise --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-readwise --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install readwise --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/readwise-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `READWISE_API_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/readwise/cmd/readwise-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS, `%APPDATA%\Claude\claude_desktop_config.json` on Windows):

```json
{
  "mcpServers": {
    "readwise": {
      "command": "readwise-pp-mcp",
      "env": {
        "READWISE_API_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

### Remote MCP over HTTP

`readwise-pp-mcp --transport http` (or `PP_MCP_TRANSPORT=http`) serves streamable HTTP at `http://127.0.0.1:7777/mcp`. It refuses to start until `READWISE_MCP_HTTP_TOKEN` is set to at least 32 characters, and every request must send `Authorization: Bearer <token>`. The command below prints 64 hex characters and meets that minimum. Plain HTTP is allowed only on a loopback `--addr`; any other bind, including `:7777` (every interface), also needs `--tls-cert` and `--tls-key`. Requests with no `Origin` header are accepted, as are loopback origins (`localhost`, `127.0.0.1`, or `[::1]`, any port). Any other `Origin` is refused unless it is listed in `READWISE_MCP_HTTP_ALLOWED_ORIGINS`, a comma-separated list of `scheme://host[:port]` values with no path. That list is not a CORS policy; the server does not emit CORS headers.

```bash
export READWISE_MCP_HTTP_TOKEN="$(openssl rand -hex 32)"
readwise-pp-mcp --transport http
```

Set `READWISE_MCP_READ_ONLY=1` (exactly `1`) to drop mirrored commands that are not marked read-only and make the raw execute tool refuse state-changing calls. Commands marked local-write, which write only this CLI's own learn store, stay available.

## Authentication

Readwise uses a personal access token from https://readwise.io/access_token, sent as `Authorization: Token <token>`. One token covers both the classic v2 API and the Reader v3 API. Set it in READWISE_API_TOKEN.

## Quick Start

```bash
# Checks config and the token without calling the network.
readwise-pp-cli doctor --dry-run

# Builds the local mirror once; later syncs are incremental.
readwise-pp-cli sync --resources highlights,books,documents

# Offline full-text search over what you synced.
readwise-pp-cli search "spaced repetition" --type highlights --limit 10

# Plans a 30-minute reading slot from your unread queue.
readwise-pp-cli backlog --budget 30m --agent

# Gathers everything you have on a topic, with sources.
readwise-pp-cli ask "habits" --include highlights,docs --group-by source --format md

```

## Unique Features

### Your library, answered offline
- **`ask`** — Pull every highlight and saved-document passage you have on a topic into one source-attributed, note-ready result.

  _Reach for this before writing or deciding on a topic, when an agent needs everything the user already read about it in one bounded response._

  ```bash
  readwise-pp-cli ask "attention is all you need" --include highlights,docs --group-by source --format md
  ```
- **`digest`** — Roll up everything you highlighted in a time window across your Readwise books, articles, tweets and podcasts, grouped by source.

  _Use this for a weekly review or to file a period's highlights into a notes system._

  ```bash
  readwise-pp-cli digest --since 2026-11-01 --group-by source --category books,articles --format md
  ```
- **`connections`** — Find items you read months apart that share tags, authors or distinctive terms, from one seed item or across the library. `--seed-doc` selects a seed, `--top` caps pairs, and `--all` with `--library-wide` considers every document.

  _Use this to resurface related past reading for a seed item without doing a topic search._

  ```bash
  readwise-pp-cli connections --library-wide --by tags,author --min-overlap 2 --agent
  ```

### Inbox triage
- **`backlog`** — See your whole reading debt by location, estimated minutes and age, and pick what fits the time you have.

  _Use this when the user has a fixed reading slot and wants a deterministic plan instead of scrolling the Reader inbox._

  ```bash
  readwise-pp-cli backlog --location new,later --budget 30m --sort priority --agent
  ```
- **`read-next`** — Get a short ranked shortlist of unread Reader documents that fit the minutes you have, favouring your preferred location and skipping ones you already started.

  _Use this when the user has a few minutes and asks what to read right now._

  ```bash
  readwise-pp-cli read-next --have 15m --prefer shortlist --avoid started --top 5 --agent
  ```

### Library hygiene
- **`dedupe`** — Find the same article saved twice, or saved both as a Reader document and as a classic Readwise source.

  _Use this before bulk exports or cleanups; the command is read-only and never deletes, so pass the IDs you choose to documents bulk-update --dry-run first._

  ```bash
  readwise-pp-cli dedupe --by url,title --scope docs,books --agent
  ```
- **`triage stale`** — Surface forgotten, never-opened saved documents (as Reader IDs you can pass to documents bulk-update) and books with zero highlights.

  _Use this to find cleanup candidates; the command is read-only, and only the document IDs (not the zero-highlight books) can be archived with documents bulk-update._

  ```bash
  readwise-pp-cli triage stale --older-than 180d --never-opened --zero-highlights --agent
  ```

### Automation
- **`watch`** — Save a query and see only the new matching documents and highlights since the last run.

  _Use this in scheduled agent jobs that must process only new items for standing topics._

  ```bash
  readwise-pp-cli watch run --agent
  ```
- **`webhook`** — Register https endpoints that receive new 'watch run --notify' matches, optionally HMAC-signed with a shared secret and SSRF-checked before delivery.

  _Use this to push watch alerts into another automation; add a watch first, then run 'watch run --notify'._

  ```bash
  readwise-pp-cli webhook list --agent
  ```

## Recipes

### Topic dossier for a draft

```bash
readwise-pp-cli ask "stoicism" --include highlights,docs --group-by source --format md
```

Collects every highlight and passage on a topic into note-ready blocks with their sources.

### Narrow a large document list for an agent

```bash
readwise-pp-cli documents list --location later --agent --select id,title,word_count
```

Returns only the fields an agent needs from a large Reader list.

### Weekly highlight review

```bash
readwise-pp-cli digest --since 2026-11-11 --group-by source --format md
```

Rolls up the week's highlights across all sources for filing into notes.

### Safe cleanup pass

```bash
readwise-pp-cli triage stale --older-than 180d --never-opened --agent
```

Lists forgotten documents as IDs you can archive with documents bulk-update.

### Alert on new reading about a topic

```bash
readwise-pp-cli watch add --name stoicism --kind highlights --query stoicism
```

Saves a highlights query; after each sync, 'watch run --notify' shows (or pushes to a registered webhook) only the new matches.

## Usage

Run `readwise-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `READWISE_CONFIG_DIR`, `READWISE_DATA_DIR`, `READWISE_STATE_DIR`, or `READWISE_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `READWISE_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export READWISE_HOME=/srv/readwise
readwise-pp-cli doctor
```

Under `READWISE_HOME=/srv/readwise`, the four dirs resolve to `/srv/readwise/config`, `/srv/readwise/data`, `/srv/readwise/state`, and `/srv/readwise/cache`.

On Windows, set the same variable in PowerShell:

```powershell
$env:READWISE_HOME = "C:\srv\readwise"
readwise-pp-cli doctor
```

Under `$env:READWISE_HOME = "C:\srv\readwise"`, the four dirs resolve to `C:\srv\readwise\config`, `C:\srv\readwise\data`, `C:\srv\readwise\state`, and `C:\srv\readwise\cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "readwise": {
      "command": "readwise-pp-mcp",
      "env": {
        "READWISE_HOME": "/srv/readwise"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `READWISE_DATA_DIR` overrides an explicit `--home` for that kind. Use `READWISE_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `READWISE_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `readwise-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

### Local data

Reads are saved locally by default: in `--data-source auto` mode every successful read is also written to the SQLite store (`data.db`, or `data-<hash>.db` per credential) in the data directory. GET responses are cached for five minutes under `http/` in the cache directory. Credentials live in `credentials.toml` in the data directory. The learn journal lives under `learn/` in the state directory. `readwise-pp-cli doctor` prints the resolved paths, every store file, and the response-cache size.

None of it is encrypted. New files are created owner-only; beyond that the data relies on OS account permissions and full-disk encryption (FileVault, BitLocker, LUKS).

Pre-migration backups are full local database copies at `<db>.v<N>.bak`, created before a one-way schema upgrade. They are owner-only (0600) and preserve the old schema for recovery. Once the upgrade is verified, delete that path manually or use `local-data purge --yes` for managed stores. Custom `--db` paths and their backups require manual removal.

- **Erase it:** `readwise-pp-cli local-data purge` lists the store files, response cache, and learn journal it would delete; add `--yes` to delete them, or `--only store|cache|journal` to narrow it. Config and credentials are kept.
- **Run without a local copy:** set `READWISE_NO_LOCAL_DATA=1`. Reads go to the API (`--data-source live`; `--data-source local` still reads an existing store), the response cache is off (`--no-cache`), nothing new is written to the store, and `sync` and `workflow archive` refuse to run. Set it in an MCP host's `env` block and every command the server runs inherits it.
- **Choose the read source without flags:** `READWISE_DATA_SOURCE=auto|live|local` sets the `--data-source` default; an explicit flag wins.

## Commands

### book-tags

Tags attached to books/sources

- **`readwise-pp-cli book-tags create`** - Add a tag to a book
- **`readwise-pp-cli book-tags delete`** - Remove a tag from a book
- **`readwise-pp-cli book-tags get`** - Get a tag on a book
- **`readwise-pp-cli book-tags list`** - List tags on a book
- **`readwise-pp-cli book-tags update`** - Rename a tag on a book

### books

Books and sources (Kindle, articles, tweets, podcasts) that own highlights

- **`readwise-pp-cli books get`** - Get a single book/source by ID
- **`readwise-pp-cli books list`** - List books/sources (rate-limited to 20 req/min; prefer 'sync' for bulk pulls). Page-based response: count, next, previous, results; follow next until null
- **`readwise-pp-cli books update`** - Update a book/source

### documents

Reader documents — saved articles, PDFs, emails, RSS, tweets, videos (Reader v3)

- **`readwise-pp-cli documents bulk-update`** - Update up to 50 documents in one request; 20 req/min per token. Returns {results: [...]}; 200 means all succeeded, 207 means partial success/per-item failure (exit 6; --allow-partial-failure downgrades to exit 0 with a warning), 400 means bad payload
- **`readwise-pp-cli documents delete`** - Delete a Reader document; 20 req/min per token. Returns 204; also deletes all its highlights and notes. Use a dry-run and confirmation gate before deleting
- **`readwise-pp-cli documents list`** - List Reader documents with filters (rate-limited to 20 req/min; prefer 'sync')
- **`readwise-pp-cli documents save`** - Save a document to Reader by URL (or HTML); 50 req/min per token. url is required unless parent_id is set. To create a highlight, supply parent_id and content; only category, content, notes, parent_id, saved_using, tags are accepted. A passage not found in the parent returns 400 and creates nothing
- **`readwise-pp-cli documents update`** - Update fields on a Reader document; 50 req/min per token. For highlights only notes and tags are accepted; other keys return 400. Notes cannot be updated directly; update them on their highlight

### highlight-tags

Tags attached to individual highlights

- **`readwise-pp-cli highlight-tags create`** - Add a tag to a highlight
- **`readwise-pp-cli highlight-tags delete`** - Remove a tag from a highlight
- **`readwise-pp-cli highlight-tags get`** - Get a tag on a highlight
- **`readwise-pp-cli highlight-tags list`** - List tags on a highlight
- **`readwise-pp-cli highlight-tags update`** - Rename a tag on a highlight

### highlights

Highlights from books, articles, tweets, and podcasts (Readwise classic v2)

- **`readwise-pp-cli highlights create`** - Create or upsert one or more highlights (creates source books/articles as needed). Dedupe key: title+author+text+source_url. Re-sending the same highlight_url with new text updates the highlight
- **`readwise-pp-cli highlights delete`** - Delete a highlight
- **`readwise-pp-cli highlights get`** - Get a single highlight by ID
- **`readwise-pp-cli highlights list`** - List highlights (rate-limited to 20 req/min; prefer 'sync' for bulk pulls). Page-based response: count, next, previous, results; follow next until null
- **`readwise-pp-cli highlights update`** - Update fields on an existing highlight

### reader-tags

Tags used across Reader documents

- **`readwise-pp-cli reader-tags`** - List all Reader document tags (20 req/min per token)

### review

Daily spaced-repetition review highlights

- **`readwise-pp-cli review complete`** - Mark today's daily review complete; returns review_id and review_completed (true)
- **`readwise-pp-cli review get`** - Get today's daily review highlights; 404 means no review today, not a failure


### ask


- **`readwise-pp-cli ask`** - Pull every highlight and saved-document passage you have on a topic into one source-attributed, note-ready result.


### backlog


- **`readwise-pp-cli backlog`** - See your whole reading debt by location, estimated minutes and age, and pick what fits the time you have.


### connections


- **`readwise-pp-cli connections`** - Find items you read months apart that share tags, authors or distinctive terms, from one seed item or across the library. `--seed-doc` selects a seed, `--top` caps pairs, and `--all` with `--library-wide` considers every document.


### dedupe


- **`readwise-pp-cli dedupe`** - Find the same article saved twice, or saved both as a Reader document and as a classic Readwise source.


### digest


- **`readwise-pp-cli digest`** - Roll up everything you highlighted in a time window across your Readwise books, articles, tweets and podcasts, grouped by source.


### read-next


- **`readwise-pp-cli read-next`** - Get a short ranked shortlist of unread Reader documents that fit the minutes you have, favouring your preferred location and skipping ones you already started.


### triage


- **`readwise-pp-cli triage stale`** - Surface forgotten, never-opened saved documents (as Reader IDs you can pass to documents bulk-update) and books with zero highlights.


### watch


- **`readwise-pp-cli watch add`** - Save a watch; seeds the seen baseline unless --include-existing treats existing matches as new.
- **`readwise-pp-cli watch diff`** - Show new-since-last items for a saved watch (no state change, no notify)
- **`readwise-pp-cli watch list`** - List saved watches with their id, name, kind, and stored filters
- **`readwise-pp-cli watch remove`** - Hard-delete a saved watch and its seen-history (cascades watch_seen rows)
- **`readwise-pp-cli watch run`** - Check watches; --name selects one, --notify sends webhooks, --strict fails on any watch failure, and --include-text respects recipient consent.


- **`readwise-pp-cli watch`** - Save a query and see only the new matching documents and highlights since the last run.


### webhook


- **`readwise-pp-cli webhook add`** - Register an HTTPS recipient; --events selects docs,highlights or *; --secret sets an HMAC secret, --secret-stdin reads its first stdin line. --include-text requires recipient consent.
- **`readwise-pp-cli webhook list`** - List configured webhooks (secrets are never shown)
- **`readwise-pp-cli webhook ping`** - Send a test payload to a webhook to confirm delivery
- **`readwise-pp-cli webhook remove`** - Hard-delete a webhook by id


- **`readwise-pp-cli webhook`** - Register https endpoints that receive new 'watch run --notify' matches, optionally HMAC-signed with a shared secret and SSRF-checked before delivery.

Every payload sends kind, watch, count, generated_at and item id, title, author, url. These fields disclose reading history. Highlight text / document summary is sent only with --include-text and recipient consent.




### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`readwise-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`readwise-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`readwise-pp-cli learnings list`** - Inspect taught rows
- **`readwise-pp-cli learnings forget <query>`** - Undo a teach; requires --resource, --action, or --all
- **`readwise-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`readwise-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`readwise-pp-cli teach-pattern`** - Install a query/resource template up front
- **`readwise-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `READWISE_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `readwise-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

### agent-context

- **`readwise-pp-cli agent-context`** - Describe commands, flags, authentication, and available profiles for agents.

### analytics

- **`readwise-pp-cli analytics`** - Analyze locally synced data with count, group-by, and summary operations.

### api

- **`readwise-pp-cli api`** - Browse API resource interfaces using their raw command names.

### auth

- **`readwise-pp-cli auth logout`** - Clear stored credentials
- **`readwise-pp-cli auth set-token`** - Save an API token from stdin to the credentials file; never pass the token in argv.
- **`readwise-pp-cli auth setup`** - Print steps for obtaining a credential (use --launch to open the URL)
- **`readwise-pp-cli auth status`** - Show authentication status

### completion

- **`readwise-pp-cli completion bash`** - Generate the autocompletion script for the bash shell.
- **`readwise-pp-cli completion fish`** - Generate the autocompletion script for the fish shell.
- **`readwise-pp-cli completion powershell`** - Generate the autocompletion script for powershell.
- **`readwise-pp-cli completion zsh`** - Generate the autocompletion script for the zsh shell.

### doctor

- **`readwise-pp-cli doctor`** - Check configuration, credentials, connectivity, and per-resource local-store row counts.

### export

- **`readwise-pp-cli export`** - Export paginated API records as --format jsonl or json; --output selects a file.

### feedback

- **`readwise-pp-cli feedback`** - Record feedback locally in feedback.jsonl; POST only when READWISE_FEEDBACK_ENDPOINT is set and --send or READWISE_FEEDBACK_AUTO_SEND=true is enabled.
- **`readwise-pp-cli feedback list`** - List recent feedback entries

### import

- **`readwise-pp-cli import`** - POST each JSONL record from --input (or - for stdin); failures are logged and processing continues.

### learnings

- **`readwise-pp-cli learnings confirm`** - Materialize a verified candidate after a successful trial.
- **`readwise-pp-cli learnings purge`** - Delete expired and confirmed candidates; --tombstones also deletes rejected rows and allows their derivation again.
- **`readwise-pp-cli learnings reject`** - Reject a candidate and retain its derivation tombstone.

### local-data

- **`readwise-pp-cli local-data purge`** - Lists what would be deleted and changes nothing unless --yes is passed.

### mirror

- **`readwise-pp-cli mirror`** - Back up both APIs with a verifiable manifest; supports --incremental, --verify, --export-dir, --force, and --db-path.

### playbook

- **`readwise-pp-cli playbook amend`** - Append a timestamped correction to query-family notes.
- **`readwise-pp-cli playbook list`** - List stored playbooks (query_family, content presence, last observed)

### profile

- **`readwise-pp-cli profile delete`** - Remove a profile
- **`readwise-pp-cli profile list`** - List saved profiles
- **`readwise-pp-cli profile save`** - Save explicitly set flags under a profile name; saving again replaces it.
- **`readwise-pp-cli profile show`** - Show a profile's values as JSON
- **`readwise-pp-cli profile use`** - Print the flag values a profile will apply (does not execute anything)

### search

- **`readwise-pp-cli search`** - Search synced books, highlights, and documents with local FTS5; --data-source live is refused.

### sync

- **`readwise-pp-cli sync`** - Sync API data into SQLite with resumable incremental sync or --full resync.

### teach-playbook

- **`readwise-pp-cli teach-playbook`** - Store a query-family playbook and/or notes.

### version

- **`readwise-pp-cli version`** - Print version

### which

- **`readwise-pp-cli which`** - Find a command by capability; exit 2 means no confident match.

### workflow

- **`readwise-pp-cli workflow archive`** - Sync all syncable resources into the local SQLite archive.
- **`readwise-pp-cli workflow status`** - Show local archive status and sync state for all resources


### help

- **`readwise-pp-cli help`** - Show help for any command.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
readwise-pp-cli highlights list --page-size 5

# JSON for scripting and agents
readwise-pp-cli highlights list --page-size 5 --json
# Filter to specific fields
readwise-pp-cli highlights list --page-size 5 --json --select id,text

# Dry run — show the request without sending
readwise-pp-cli highlights list --page-size 5 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
readwise-pp-cli highlights list --page-size 5 --agent
```

`--csv` prefixes a leading `'` on cells and header keys that start with `=`, `+`, `-`, `@`, a tab, or a line break (including fullwidth `＝＋－＠`) so a spreadsheet does not run them as formulas. Plain signed numbers such as `-5` and `+2.5e3` are left unchanged.

### Response envelope

Read commands wrap data in `{meta, results}`; parse `.results` for payload and `.meta.source` for live/local provenance (with `synced_at` and `reason` when applicable). `--agent` uses this envelope too. The `read-next` exception always prints a JSON array, including under `--agent`; hints go to stderr.

Single-page live lists return `{meta, results}`. `meta.api_count` preserves the API count (its meaning depends on the endpoint). `meta.has_more` is false only when the API explicitly says there is no next page; it can be absent when continuation is unknown. `meta.next_cursor` is the exact value to pass back using `meta.next_cursor_flag`: `--page-cursor` for Reader v3 lists, or `--page` for Readwise v2 lists. Keep the same filters when resuming. These paging fields are absent under `--all` and local reads.

```bash
readwise-pp-cli documents list --json
readwise-pp-cli documents list --page-cursor "<next_cursor>" --json
```

When a single page has more results, stderr emits a JSON `event: "truncated"`, with `hint`, `next_cursor_flag`, and `next_cursor` when continuation is available. With `--human-friendly`, it prints `warning: results truncated; ...` instead. It suggests `--all` only when pagination can advance; an opaque response without a usable cursor may emit only the truncation event. If `--all` lacks any declared pagination signal, stderr warns that only page 1 is returned.

`--all` follows pages up to the 100-page safety cap (`paginatedGetMaxPages`). For books/highlights, the default `--page-size 100` allows at most 10,000 items; `--all --page-size 1000` raises that upper bound to 100,000 items.

MCP responses have a separate byte-budget envelope with `truncated`, `next_offset`, and `max_bytes`. Mirrored tools can resume with MCP `next_cursor`; CLI `meta.next_cursor` / `meta.next_cursor_flag` refer to API pagination and must not be substituted for MCP continuation.

### Output delivery and saved profiles

`--deliver` supports four sinks: `stdout`, `file:<path>` (atomic private file), `webhook:<url>` (POST; redirects refused), and `spacefast:<name>` (publish a live web page). Inspect the target before sending output externally.

Use `profile save` to capture explicit flags and `--profile <name>` to apply them; explicit flags override saved values. `profile use <name>` prints the saved values without executing a command. `agent-context` lists available profiles.

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

## Health Check

```bash
readwise-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API, and reports per-resource local-store row counts.

## Configuration

Run `readwise-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/readwise-pp-cli/config.toml` on macOS/Linux and `%USERPROFILE%\.config\readwise-pp-cli\config.toml` on Windows; `--home`, `READWISE_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

`--ca-cert`, `READWISE_CA_CERT`, or `ca_cert` adds a PEM CA bundle to the system trust roots. `--tls-server-name`, `READWISE_TLS_SERVER_NAME`, or `tls_server_name` is the name checked against the certificate when the dialed address does not match it. Precedence is flag, then environment, then the config file. A config file other users can change does not supply these settings. Certificate verification failures are not retried.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `READWISE_API_TOKEN` | per_call | Yes | Set to your API credential. |
| `READWISE_BASE_URL` | optional | No | Override the API base URL. |
| `READWISE_CONFIG` | optional | No | Config file path when --config is not set. |
| `READWISE_MAX_RESPONSE_BYTES` | optional | No | Buffered HTTP response byte limit, including decompressed bodies; default 134217728 (128 MiB). Invalid or non-positive values use the default. |
| `READWISE_USER_AGENT` | optional | No | Override the User-Agent for API requests and webhook output delivery. |
| `READWISE_CLI_PATH` | optional | No | Companion CLI path for MCP when no sibling executable is found; checked before PATH. |
| `PRINTING_PRESS_CLIENT_PROFILE` | optional | No | Select a tenant-gated client profile; explicit --client-profile wins. Separate from saved --profile flags. |
| `PP_MCP_MAX_BYTES` | optional | No | MCP response byte budget; default 32000. Invalid or non-positive values use the default. |
| `READWISE_FEEDBACK_ENDPOINT` | optional | No | Optional destination for feedback POSTs; unset keeps feedback local. |
| `READWISE_FEEDBACK_AUTO_SEND` | optional | No | Set to true to send feedback automatically when an endpoint is configured. |
| `READWISE_CA_CERT` | optional | No | PEM CA bundle added to system trust roots. |
| `READWISE_TLS_SERVER_NAME` | optional | No | Certificate verification name when the dialed host differs; TLS flags override these env settings. |
| `PP_MCP_TRANSPORT` | optional | No | MCP transport default (stdio or http); --transport wins, otherwise stdio is used. |


### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `readwise-pp-cli doctor` reports `agentcookie: detected` and `auth status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `readwise-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $READWISE_API_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **HTTP 429 Too Many Requests during sync or list commands** — Wait for the Retry-After delay the CLI reports, then rerun `readwise-pp-cli sync`; use local commands instead of repeated live lists.
- **Commands say the local store is empty or stale** — Run `readwise-pp-cli sync --resources highlights,books,documents` (add `--full` to rebuild).
- **HTTP 401 on every command** — Set a fresh token from https://readwise.io/access_token in READWISE_API_TOKEN, then run `readwise-pp-cli doctor`.
- **Bulk update reports some items failed** — The API returned a partial success; rerun `documents bulk-update` with only the failed IDs it listed.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**readwise-mcp**](https://github.com/IAmAlexander/readwise-mcp) — TypeScript (24 stars)
- [**readwise-cli (official)**](https://github.com/readwiseio/readwise-cli) — TypeScript
- [**readwise-cli**](https://github.com/Nardjo/readwise-cli) — TypeScript
- [**readwise-mcp-enhanced**](https://github.com/arnaldo-delisio/readwise-mcp-enhanced) — TypeScript
- [**readwise-skills (official)**](https://github.com/readwiseio/readwise-skills) — Markdown
- [**readwise-reader-cli**](https://github.com/Scarvy/readwise-reader-cli) — Python
- [**pyreadwise**](https://github.com/rwxd/pyreadwise) — Python
- [**kiseki-labs-readwise-mcp**](https://github.com/kiseki-technologies/kiseki-labs-readwise-mcp) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

### Output limits

`backlog`, `dedupe`, and `triage stale` default to `--limit 50`. Output reports `limited`, `limit_note`, and uncapped `total_*` counts. Use `--limit 0` to get everything.

