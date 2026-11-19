---
name: pp-readwise
description: "Every Readwise and Reader feature, plus a local mirror of your whole library that you can search, triage and dedupe offline. Trigger phrases: `what have I read about`, `find my highlights on`, `what should I read next`, `clean up my Reader inbox`, `save this article to Reader`, `use readwise`, `run readwise`."
author: "Vinny Pasceri"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - readwise-pp-cli
    install:
      - kind: go
        bins: [readwise-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/productivity/readwise/cmd/readwise-pp-cli
---

# Readwise — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `readwise-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install readwise --cli-only
   ```
2. Verify: `readwise-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.11 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/readwise/cmd/readwise-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Sync your highlights, books and Reader documents once into a local SQLite store, then query them without the 20-requests-a-minute list limit. Ask what you have already read on a topic with `ask`, plan a reading slot with `backlog` and `read-next`, and clean up with `dedupe` and `triage stale`. Writes (save, tag, archive, bulk update) still go straight to the API, with dry runs first.

## When to Use This CLI

Use this CLI when an agent needs the user's own Readwise highlights, books or Reader documents: finding what they already read on a topic, planning or triaging their reading queue, finding duplicates or stale items, or saving and tagging items. Prefer the local commands for anything that scans the library, because the live list endpoints allow only 20 requests a minute.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to read or summarize the full text of the web article itself; it works with what the user saved and highlighted.
- Do not use it to manage Readwise Books shelves, ratings or reading sessions; that API is not publicly documented and is not supported.
- Do not use it to configure Readwise's own webhooks or integrations; those are set up in the Readwise web app.
- Do not use it for semantic or vector search; search here is local full-text.

## Unique Capabilities

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

## Command Reference

**book-tags** — Tags attached to books/sources

- `readwise-pp-cli book-tags create` — Add a tag to a book
- `readwise-pp-cli book-tags delete` — Remove a tag from a book
- `readwise-pp-cli book-tags get` — Get a tag on a book
- `readwise-pp-cli book-tags list` — List tags on a book
- `readwise-pp-cli book-tags update` — Rename a tag on a book

**books** — Books and sources (Kindle, articles, tweets, podcasts) that own highlights

- `readwise-pp-cli books get` — Get a single book/source by ID
- `readwise-pp-cli books list` — List books/sources (rate-limited to 20 req/min; prefer 'sync' for bulk pulls).
- `readwise-pp-cli books update` — Update a book/source

**documents** — Reader documents — saved articles, PDFs, emails, RSS, tweets, videos (Reader v3)

- `readwise-pp-cli documents bulk-update` — Update up to 50 documents in one request; 20 req/min per token. Returns {results: [...
- `readwise-pp-cli documents delete` — Delete a Reader document; 20 req/min per token. Returns 204; also deletes all its highlights and notes.
- `readwise-pp-cli documents list` — List Reader documents with filters (rate-limited to 20 req/min; prefer 'sync')
- `readwise-pp-cli documents save` — Save a document to Reader by URL (or HTML); 50 req/min per token. url is required unless parent_id is set.
- `readwise-pp-cli documents update` — Update fields on a Reader document; 50 req/min per token.

**highlight-tags** — Tags attached to individual highlights

- `readwise-pp-cli highlight-tags create` — Add a tag to a highlight
- `readwise-pp-cli highlight-tags delete` — Remove a tag from a highlight
- `readwise-pp-cli highlight-tags get` — Get a tag on a highlight
- `readwise-pp-cli highlight-tags list` — List tags on a highlight
- `readwise-pp-cli highlight-tags update` — Rename a tag on a highlight

**highlights** — Highlights from books, articles, tweets, and podcasts (Readwise classic v2)

- `readwise-pp-cli highlights create` — Create or upsert one or more highlights (creates source books/articles as needed).
- `readwise-pp-cli highlights delete` — Delete a highlight
- `readwise-pp-cli highlights get` — Get a single highlight by ID
- `readwise-pp-cli highlights list` — List highlights (rate-limited to 20 req/min; prefer 'sync' for bulk pulls).
- `readwise-pp-cli highlights update` — Update fields on an existing highlight

**reader-tags** — Tags used across Reader documents

- `readwise-pp-cli reader-tags` — List all Reader document tags (20 req/min per token)

**review** — Daily spaced-repetition review highlights

- `readwise-pp-cli review complete` — Mark today's daily review complete; returns review_id and review_completed (true)
- `readwise-pp-cli review get` — Get today's daily review highlights; 404 means no review today, not a failure


**ask**


- `readwise-pp-cli ask` — Pull every highlight and saved-document passage you have on a topic into one source-attributed, note-ready result.


**backlog**


- `readwise-pp-cli backlog` — See your whole reading debt by location, estimated minutes and age, and pick what fits the time you have.


**connections**


- `readwise-pp-cli connections` — Find items you read months apart that share tags, authors or distinctive terms, from one seed item or across the library. `--seed-doc` selects a seed, `--top` caps pairs, and `--all` with `--library-wide` considers every document.


**dedupe**


- `readwise-pp-cli dedupe` — Find the same article saved twice, or saved both as a Reader document and as a classic Readwise source.


**digest**


- `readwise-pp-cli digest` — Roll up everything you highlighted in a time window across your Readwise books, articles, tweets and podcasts, grouped by source.


**read-next**


- `readwise-pp-cli read-next` — Get a short ranked shortlist of unread Reader documents that fit the minutes you have, favouring your preferred location and skipping ones you already started.


**triage**


- `readwise-pp-cli triage stale` — Surface forgotten, never-opened saved documents (as Reader IDs you can pass to documents bulk-update) and books with zero highlights.


**watch**

- `readwise-pp-cli watch add` — Save a watch; seeds the seen baseline unless --include-existing treats existing matches as new.
- `readwise-pp-cli watch diff` — Show new-since-last items for a saved watch (no state change, no notify)
- `readwise-pp-cli watch list` — List saved watches with their id, name, kind, and stored filters
- `readwise-pp-cli watch remove` — Hard-delete a saved watch and its seen-history (cascades watch_seen rows)
- `readwise-pp-cli watch run` — Check watches; --name selects one, --notify sends webhooks, --strict fails on any watch failure, and --include-text respects recipient consent.



- `readwise-pp-cli watch` — Save a query and see only the new matching documents and highlights since the last run.


**webhook**

- `readwise-pp-cli webhook add` — Register an HTTPS recipient; --events selects docs,highlights or *; --secret sets an HMAC secret, --secret-stdin reads its first stdin line. --include-text requires recipient consent.
- `readwise-pp-cli webhook list` — List configured webhooks (secrets are never shown)
- `readwise-pp-cli webhook ping` — Send a test payload to a webhook to confirm delivery
- `readwise-pp-cli webhook remove` — Hard-delete a webhook by id



- `readwise-pp-cli webhook` — Register https endpoints that receive new 'watch run --notify' matches, optionally HMAC-signed with a shared secret and SSRF-checked before delivery.

Every payload sends kind, watch, count, generated_at and item id, title, author, url. These fields disclose reading history. Highlight text / document summary is sent only with --include-text and recipient consent.




**Self-learning loop**

- `readwise-pp-cli recall <query>` — Look up cached resources for a query before running discovery
- `readwise-pp-cli teach` — Record a query -> resource mapping (silent on success, safe to background with `&`)
- `readwise-pp-cli learnings list` — Inspect taught rows
- `readwise-pp-cli learnings forget <query>` — Undo a teach; requires --resource, --action, or --all
- `readwise-pp-cli learnings candidates` — List auto-captured candidates awaiting confirm/reject
- `readwise-pp-cli learnings stats` — Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- `readwise-pp-cli teach-pattern` — Install a query/resource template up front
- `readwise-pp-cli teach-lookup` — Add an entity mapping (e.g. country code, team alias) for pattern substitution

**agent-context**

- `readwise-pp-cli agent-context` — Describe commands, flags, authentication, and available profiles for agents.

**analytics**

- `readwise-pp-cli analytics` — Analyze locally synced data with count, group-by, and summary operations.

**api**

- `readwise-pp-cli api` — Browse API resource interfaces using their raw command names.

**auth**

- `readwise-pp-cli auth logout` — Clear stored credentials
- `readwise-pp-cli auth set-token` — Save an API token from stdin to the credentials file; never pass the token in argv.
- `readwise-pp-cli auth setup` — Print steps for obtaining a credential (use --launch to open the URL)
- `readwise-pp-cli auth status` — Show authentication status

**completion**

- `readwise-pp-cli completion bash` — Generate the autocompletion script for the bash shell.
- `readwise-pp-cli completion fish` — Generate the autocompletion script for the fish shell.
- `readwise-pp-cli completion powershell` — Generate the autocompletion script for powershell.
- `readwise-pp-cli completion zsh` — Generate the autocompletion script for the zsh shell.

**doctor**

- `readwise-pp-cli doctor` — Check configuration, credentials, connectivity, and per-resource local-store row counts.

**export**

- `readwise-pp-cli export` — Export paginated API records as --format jsonl or json; --output selects a file.

**feedback**

- `readwise-pp-cli feedback` — Record feedback locally in feedback.jsonl; POST only when READWISE_FEEDBACK_ENDPOINT is set and --send or READWISE_FEEDBACK_AUTO_SEND=true is enabled.
- `readwise-pp-cli feedback list` — List recent feedback entries

**import**

- `readwise-pp-cli import` — POST each JSONL record from --input (or - for stdin); failures are logged and processing continues.

**learnings**

- `readwise-pp-cli learnings confirm` — Materialize a verified candidate after a successful trial.
- `readwise-pp-cli learnings purge` — Delete expired and confirmed candidates; --tombstones also deletes rejected rows and allows their derivation again.
- `readwise-pp-cli learnings reject` — Reject a candidate and retain its derivation tombstone.

**local-data**

- `readwise-pp-cli local-data purge` — Lists what would be deleted and changes nothing unless --yes is passed.

**mirror**

- `readwise-pp-cli mirror` — Back up both APIs with a verifiable manifest; supports --incremental, --verify, --export-dir, --force, and --db-path.

**playbook**

- `readwise-pp-cli playbook amend` — Append a timestamped correction to query-family notes.
- `readwise-pp-cli playbook list` — List stored playbooks (query_family, content presence, last observed)

**profile**

- `readwise-pp-cli profile delete` — Remove a profile
- `readwise-pp-cli profile list` — List saved profiles
- `readwise-pp-cli profile save` — Save explicitly set flags under a profile name; saving again replaces it.
- `readwise-pp-cli profile show` — Show a profile's values as JSON
- `readwise-pp-cli profile use` — Print the flag values a profile will apply (does not execute anything)

**search**

- `readwise-pp-cli search` — Search synced books, highlights, and documents with local FTS5; --data-source live is refused.

**sync**

- `readwise-pp-cli sync` — Sync API data into SQLite with resumable incremental sync or --full resync.

A filtered sync (`--resource-param` that narrows rows) never advances the shared checkpoint. Capped syncs resume where they stopped, including the separate Classic books-list pass. The books-list pass re-reads its saved page to absorb small result-set shifts. It restarts at page 1 when the page size changes or the saved page disappears. `--full` discards any capped continuation; `--full --latest-only` preserves it while refreshing the head.

**teach-playbook**

- `readwise-pp-cli teach-playbook` — Store a query-family playbook and/or notes.

**version**

- `readwise-pp-cli version` — Print version

**which**

- `readwise-pp-cli which` — Find a command by capability; exit 2 means no confident match.

**workflow**

- `readwise-pp-cli workflow archive` — Sync all syncable resources into the local SQLite archive.
- `readwise-pp-cli workflow status` — Show local archive status and sync state for all resources

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
readwise-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query. `--json` (and other machine formats) keep that exit-2 contract and write `{"matches":[]}` on stdout so agents can inspect the envelope without treating a miss as success.


**help**

- `readwise-pp-cli help` — Show help for any command.

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

## Auth Setup

Readwise uses a personal access token from https://readwise.io/access_token, sent as `Authorization: Token <token>`. One token covers both the classic v2 API and the Reader v3 API. Set it in READWISE_API_TOKEN.

Run `readwise-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color`.

Global format flags share one contract on promoted, novel, sync, and `--deliver` paths:

- `--json` — one JSON document on stdout (sync progress events go to stderr)
- `--compact` — keep identity/status/timestamp fields; does not change the document vs stream shape
- `--csv` / `--plain` — tabular rows (collection envelopes unwrap to the row array). `--csv` prefixes `'` on a cell or header key that starts with `=`, `+`, `-`, `@`, tab, CR, or LF (fullwidth `＝＋－＠` too) so a spreadsheet does not run it as a formula; a plain signed number such as `-5` or `+2.5e3` is unchanged.
- `--quiet` — one identity value per row, no envelope

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  readwise-pp-cli highlights list --page-size 5 --agent --select id,text
  ```
- **Previewable** — `--dry-run` shows the request without sending. A completed dry run exits 0 with `"success": true` and `"dry_run": true` and no HTTP `status`; validation failures still exit non-zero with `"success": false`.
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit confirmation** — `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and use `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Single-page live lists return `{meta, results}`. `meta.api_count` preserves the API count (its meaning depends on the endpoint). `meta.has_more` is false only when the API explicitly says there is no next page; it can be absent when continuation is unknown. `meta.next_cursor` is the exact value to pass back using `meta.next_cursor_flag`: `--page-cursor` for Reader v3 lists, or `--page` for Readwise v2 lists. Keep the same filters when resuming. These paging fields are absent under `--all` and local reads.

```bash
readwise-pp-cli documents list --json
readwise-pp-cli documents list --page-cursor "<next_cursor>" --json
```

When a single page has more results, stderr emits a JSON `event: "truncated"`, with `hint`, `next_cursor_flag`, and `next_cursor` when continuation is available. With `--human-friendly`, it prints `warning: results truncated; ...` instead. It suggests `--all` only when pagination can advance; an opaque response without a usable cursor may emit only the truncation event. If `--all` lacks any declared pagination signal, stderr warns that only page 1 is returned.

`--all` follows pages up to the 100-page safety cap (`paginatedGetMaxPages`). For books/highlights, the default `--page-size 100` allows at most 10,000 items; `--all --page-size 1000` raises that upper bound to 100,000 items.

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

`meta.next_cursor` / `meta.next_cursor_flag` are CLI-level values for the API page or cursor flag; they are distinct from the MCP envelope’s `next_cursor` / `next_offset` and must not be swapped.

MCP list responses carry `results`, `count`, `returned_count`, `truncated`, `next_offset`, and `max_bytes` (plus a note when truncated). The byte budget can truncate before the item limit; resume mirrored tools with their MCP continuation fields. `PP_MCP_MAX_BYTES` sets the shared byte budget, default 32000.

## Environment variables

| Name | Purpose |
| --- | --- |
| `READWISE_BASE_URL` | Override the API base URL. |
| `READWISE_CONFIG` | Config file path when --config is not set. |
| `READWISE_MAX_RESPONSE_BYTES` | Buffered HTTP response byte limit, including decompressed bodies; default 134217728 (128 MiB). Invalid or non-positive values use the default. |
| `READWISE_USER_AGENT` | Override the User-Agent for API requests and webhook output delivery. |
| `READWISE_CLI_PATH` | Companion CLI path for MCP when no sibling executable is found; checked before PATH. |
| `PRINTING_PRESS_CLIENT_PROFILE` | Select a tenant-gated client profile; explicit --client-profile wins. Separate from saved --profile flags. |
| `PP_MCP_MAX_BYTES` | MCP response byte budget; default 32000. Invalid or non-positive values use the default. |
| `READWISE_FEEDBACK_ENDPOINT` | Optional destination for feedback POSTs; unset keeps feedback local. |
| `READWISE_FEEDBACK_AUTO_SEND` | Set to true to send feedback automatically when an endpoint is configured. |
| `READWISE_CA_CERT` | PEM CA bundle added to system trust roots. |
| `READWISE_TLS_SERVER_NAME` | Certificate verification name when the dialed host differs; TLS flags override these env settings. |
| `PP_MCP_TRANSPORT` | MCP transport default (stdio or http); --transport wins, otherwise stdio is used. |

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `READWISE_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `READWISE_CONFIG_DIR`, `READWISE_DATA_DIR`, `READWISE_STATE_DIR`, `READWISE_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `READWISE_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `readwise-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

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

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `READWISE_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `READWISE_HOME`, or `doctor` will not find credentials left under the former root.

Local data is unencrypted and relies on OS account permissions plus disk encryption. Reads in the default `--data-source auto` mode are written to the SQLite store; GET responses are cached for five minutes under the cache dir. `local-data purge` previews what it would erase (store files, response cache, learn journal) and deletes only with `--yes`; it is not exposed over MCP. `READWISE_NO_LOCAL_DATA=1` disables the response cache and store writes, turns `auto` reads into live reads, and blocks `sync` and `workflow archive`; `--data-source local` still reads an existing store, and `READWISE_DATA_SOURCE` sets the `--data-source` default.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, pass the question as an argv or MCP tool argument to `recall --agent`. Do not interpolate user-controlled text into a shell command line.

Quoted `recall "<question>"` breaks on an apostrophe, which is ordinary English. A quoted heredoc breaks when a body line equals the delimiter, and that delimiter is published in these docs. Write the question with a non-shell file-writing tool, then read it back as data:

```bash
# Write the question verbatim with your file-writing tool (no shell involved).
# Command substitution on a file only ever yields data — the shell never
# parses the file's bytes as syntax.
QUERY=$(cat /path/to/question.txt)
readwise-pp-cli recall "$QUERY" --agent
```

Prefer MCP: pass the question as the tool's query argument. `"$QUERY"` after a file read is argv-safe; putting the question itself in the command text is not.

The response envelope:

```json
{
  "query": "...",
  "normalized": "<normalized form>",
  "query_entities": ["..."],
  "found": true | false,
  "match_score": 0.0,
  "results": [
    { "resource_id": "...", "resource_type": "...", "venue": "...",
      "confidence": 2, "entity_match": "exact|partial|unknown",
      "source": "taught|preseed|pattern", "warnings": ["..."] }
  ],
  "mismatches": [ /* only when --debug-mismatches */ ],
  "warnings": [ /* top-level */ ],
  "candidates": [
    { "id": 12, "class": "flag_alias | playbook_candidate",
      "summary": "...", "sightings": 3, "last_seen": "...",
      "rationale": "...",
      "next_action": ["<trial command>", "readwise-pp-cli learnings confirm 12"] }
  ],
  "playbook": {
    "query_family": "...",
    "playbook": {
      "steps": [ { "cmd": "<command with {slot} substitution>", "purpose": "..." } ],
      "entity_slots": ["$ENTITY"],
      "expected_tool_calls": 3
    },
    "slots_resolved": { "$ENTITY": { "token": "<live token>", "canonical": "<canonical>" } },
    "notes": "<workarounds + gotchas for this query family>"
  },
  "notes": "<duplicate surface for non-playbook callers>"
}
```

Empty-store short-circuit: if the store has no learnings, playbooks, or candidates yet (recall finds nothing and `learnings list` and `learnings candidates` are both empty), skip recall for the rest of this session instead of taxing every query; resume recall-first once something has been taught.

### Step 2: decision tree

Read `candidates`, `playbook`, `notes`, `results[0]`, and warnings in that order:

```
if Candidates present (warnings include "candidates_present"):
    -> candidates are try-then-confirm, never facts. Follow each candidate's
       two-step next_action verbatim: run the trial command first, then run
       `learnings confirm <id>` only after the trial verified the behavior.
       Reject a wrong candidate with `learnings reject <id>`.
    -> NEVER re-teach something recall surfaced as a candidate; confirm or
       reject that candidate instead of teaching a duplicate.
    -> candidates ride alongside playbooks and resource hits, not instead of
       them; continue with the branches below after acting on them.

if Playbook present:
    -> READ Playbook.notes verbatim FIRST (workarounds + gotchas the CLI surface doesn't expose)
    -> replay Playbook.steps in order, substituting Playbook.slots_resolved entries
       for the entity slot tokens. If a step's slot is unresolved, fall back to
       discovery for that step only.
    -> the Playbook's expected_tool_calls is a budget; if you find yourself running
       materially more, record the divergence via `readwise-pp-cli playbook amend`
       at end-of-session.

elif Notes present (no Playbook):
    -> read Notes verbatim before any discovery step; they carry known gotchas
       for this query family even when no structured choreography exists yet.

elif Found AND Results[0].EntityMatch == "exact" AND Results[0].Confidence >= 2:
    -> skip discovery; fetch live data for Results[*].ResourceID in parallel

elif Found AND Results[0].EntityMatch == "partial":
    -> candidate hint, NOT a hit; read the resource title to validate before trusting

elif (any row in Mismatches[] when --debug-mismatches was passed):
    -> treat as cold start; the stored learning is for a different entity
       (different canonical resolved from query_entities)

else:  // Found == false, no playbook, no notes
    -> cold start; run discovery normally; teach the answer afterward (Step 4).
       If the family has no playbook yet, that teach auto-synthesizes a
       playbook candidate from this session's journal - you do not need to
       record one by hand.
```

Playbook and Notes are orthogonal to the per-resource path. A recall response can carry both a Playbook AND a `Results[]` hit - use both: the Playbook tells you which choreography to run; the resource hits short-circuit specific steps. Default to skipping `mismatches`; pass `--debug-mismatches` only when investigating cold-start surprises.

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `readwise-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `readwise-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately. Pass the query the same way as recall — argv/MCP, or file-then-`$QUERY`. Do not splice the question into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
readwise-pp-cli teach --query "$QUERY" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
QUERY=$(cat /path/to/question.txt)
readwise-pp-cli teach \
  --query "$QUERY" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
QUERY=$(cat /path/to/question.txt)
readwise-pp-cli teach-playbook \
  --query "$QUERY" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`. Pass the query and note as argv/MCP arguments, or write each with a non-shell file tool and read them back (`QUERY=$(cat ...)`, `NOTE=$(cat ...)`). Do not interpolate either string into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
NOTE=$(cat /path/to/note.txt)
readwise-pp-cli playbook amend \
  --query "$QUERY" \
  --add-note "$NOTE"
# (append shell `&` to background it)
```

What counts as worth amending: a behavior you OBSERVED this session that future-you would benefit from knowing. Examples worth amending:

- A workaround for a CLI surface that silently drops or misorders a flag.
- An undocumented endpoint shape (response wrapped in `{meta, results}`, payload nested two levels deeper than the docs claim).
- Observed schema drift (a field renamed, an index that shifted between seasons, a category label that the API now returns lower-cased).

What does NOT belong in notes:

- The year-specific or entity-specific answer to the user's question. That's the response, not a learning.
- Per-team / per-athlete / per-row data the playbook already retrieves at runtime.
- Statements that paraphrase what the existing notes already say.

The amend command appends to the family's existing notes with a timestamped marker (`[amend YYYY-MM-DDTHH:MMZ]: <text>`). Multiple amends accumulate; the audit trail is visible. If no playbook exists yet for the family, amend creates a notes-only one (so cold-start corrections still land).

#### PII discipline for amend notes

`playbook amend` notes are designed to potentially flow upstream as shared knowledge in future versions of the Printing Press. Keep them clean of user-identifying content so the upstream-contribution path stays open without retroactive scrubbing:

- **Do NOT embed** paths to user filesystems, personal API keys or tokens, user email addresses, user GitHub handles, or specific query histories tied to a single user.
- **Acceptable**: endpoint shapes, undocumented field names, API gotchas, observed schema drift, workarounds for CLI surfaces, generalizable pagination or retry tactics.

If a correction is only meaningful with user-specific context, it belongs in a personal note, not in the playbook amend.

### Measuring the loop

`readwise-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `READWISE_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
readwise-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
readwise-pp-cli feedback --stdin < notes.txt
readwise-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `READWISE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `READWISE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Four sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `spacefast:<name>` | Publish output as a live web page on Spacefast |
| `file:<path>` | Atomically write output to `<path>` as a private 0600 file (exclusive randomly named temp in the destination directory, then rename; a planted symlink is not followed). Binary-response commands write decoded payload bytes (not the base64 JSON envelope) and print a small JSON receipt on stdout; `--json`/`--csv` do not refuse when this sink is set. |
| `webhook:<url>` | POST the output body to the URL (`application/json`). Redirects are not followed; a 3xx response fails the delivery and the body is not sent to the `Location`. |

`watch run --deliver` includes matched item titles/urls in the selected sink and prints a stderr notice when matches are present.

Unknown schemes are refused with a structured error naming the supported set. Webhook failures, including redirects, return non-zero and log the URL and HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

```
readwise-pp-cli profile save briefing --json
readwise-pp-cli --profile briefing highlights list --page-size 5
readwise-pp-cli profile list --json
readwise-pp-cli profile show briefing
readwise-pp-cli profile delete briefing --yes
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
| 6 | Partial failure |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `readwise-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/readwise/cmd/readwise-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add readwise-pp-mcp -- readwise-pp-mcp
   ```
3. Verify: `claude mcp list`

For a remote host, `readwise-pp-mcp --transport http` requires `READWISE_MCP_HTTP_TOKEN` set to at least 32 characters (callers send `Authorization: Bearer <token>`; `openssl rand -hex 32` prints 64 hex characters and meets that minimum), binds `127.0.0.1:7777` by default, and needs `--tls-cert`/`--tls-key` for any non-loopback `--addr`. Requests with no `Origin` header are accepted. Loopback origins are accepted; any other `Origin` is refused unless listed in `READWISE_MCP_HTTP_ALLOWED_ORIGINS` (comma-separated `scheme://host[:port]` values). `READWISE_MCP_READ_ONLY=1` drops mirrored write commands (local-write commands that only touch the CLI's own learn store stay) and makes the raw execute tool refuse state-changing calls.

## Direct Use

1. Check if installed: `which readwise-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   readwise-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `readwise-pp-cli <command> --help`.

### Output limits

`backlog`, `dedupe`, and `triage stale` default to `--limit 50`. Output reports `limited`, `limit_note`, and uncapped `total_*` counts. Use `--limit 0` to get everything.

### Agent output shape

`--agent` wraps results as `{meta, results}`. Read the payload under `results`; `meta` describes provenance. The compatibility exception is `read-next`, which always prints a JSON array on stdout, including with `--agent`; its hints and excluded-length count go to stderr.

`watch run` prints new item IDs, titles, and URLs before marking them seen; JSON includes `items` for each watch, including when notifying webhooks. A later run shows only newly matched items.

`ask` returns up to 50 matches per source. JSON adds `truncated` and `returned_counts` (returned passages per source); Markdown prints a notice when more matches exist. Reader child rows are excluded before the document limit, so truncation reflects top-level matches.

