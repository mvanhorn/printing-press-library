---
name: pp-archive-is
description: "Use this skill whenever the user wants to archive a URL, bypass a paywall, look up an existing archive, view a cached version of a webpage, pull article text from archive.today or the Wayback Machine, or batch-archive a list of URLs. archive.today + Wayback Machine CLI with lookup-before-submit, automatic fallback when one backend is down, and agent-friendly output. No API key required. Triggers on phrasings like 'archive this article', 'bypass the paywall on this link', 'grab the cached text', 'save this url to archive.today', 'check if this was already archived', 'bulk archive these 20 URLs'."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["archive-is-pp-cli"]},"install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/archive-is/cmd/archive-is-pp-cli@latest","bins":["archive-is-pp-cli"],"label":"Install via go install"}]}}'
---

# archive.today — Printing Press CLI

Bypass paywalls and look up web archives via archive.today — reverse-engineered from the Memento timegate, `/submit/` endpoint, and CDX index into a proper CLI. Wayback Machine fallback is wired in automatically: if archive.today is CAPTCHA-gating or down, the CLI transparently queries `web.archive.org/cdx/search/cdx` and returns the Wayback snapshot text.

## When to Use This CLI

Reach for this whenever a user wants to archive a URL, read a paywalled article, check whether something was previously archived, or batch-capture a list of URLs for research. Specifically good when:

- A user sends a paywalled link and asks "can you read this" → `read` fetches text via archive
- They want to preserve a URL that might change → `save` forces a fresh capture
- They want historical versions → `history` lists all known snapshots
- They have 20+ URLs to archive → `bulk` runs rate-limited batch archival

Don't reach for this if the URL is trivially scrapeable without archive services (no paywall, robots-allowed, direct HTTP works), or if the user wants the original source rather than a cached version.

## Unique Capabilities

The whole CLI is unique — archive.today has no official API. But within this CLI, certain commands are the differentiators.

### The hero commands

- **`read <url>`** — Find or create an archive for a URL. Looks up existing snapshots first (Memento timegate → CDX fallback); submits a fresh capture only if nothing exists. The "always do the right thing" command.

  _This is how 90% of agent calls should start. It's idempotent — calling it twice on the same URL doesn't double-submit._

- **`get <url> [--format text|html]` / `tldr <url>`** — Fetch article text, optionally LLM-summarized. Automatic Wayback fallback when archive.today serves a CAPTCHA (which happens daily to cloud IPs).

  _`tldr` pipes the fetched text through a summarization step — useful for agent chains where you want a short take without shipping 20KB of HTML back._

### Durability operations

- **`save <url>`** — Force a fresh capture via `/submit/?url=<x>&anyway=1`. Use when `read` returns an existing snapshot that's too old or missing a paywall update.

- **`history <url>`** — List all known snapshots via Memento timemap parsing. Shows every capture date across both archive.today and Wayback.

- **`bulk [file]`** — Rate-limited batch archiving from a file or stdin. Reads URLs one per line, submits each with backoff, returns a report of successes / failures / pre-existing.

  _`grep -oE 'https?://[^ )]+' notes.md | archive-is-pp-cli bulk -` archives every URL in a markdown file._

- **`request <url>`** — Fire-and-forget submit with optional wait+poll. Useful for long captures where you want to come back later.

### Observability

- **`snapshots newest <url>`** — Just the newest snapshot URL for a target, useful in scripts.

- **`captures`** — List your local capture index (post-sync).

- **`feeds`** — archive.today's global recent-archives feed.

- **`--backend archive-is,wayback`** — Every read/get accepts a backend preference. Defaults to archive-is with Wayback fallback; flip the order for Wayback-primary.

## Command Reference

Archive + retrieve:

- `archive-is-pp-cli read <url>` — Find or create (hero command)
- `archive-is-pp-cli get <url>` — Fetch article text (with Wayback fallback)
- `archive-is-pp-cli tldr <url>` — Fetch + summarize
- `archive-is-pp-cli save <url>` — Force fresh capture
- `archive-is-pp-cli request <url>` — Fire-and-forget submit
- `archive-is-pp-cli check <url>` — Does an archive exist?

Listing + history:

- `archive-is-pp-cli history <url>` — All known snapshots
- `archive-is-pp-cli newest <url>` — Newest snapshot URL
- `archive-is-pp-cli captures` — Local capture index
- `archive-is-pp-cli feeds` — Global recent feed

Batch:

- `archive-is-pp-cli bulk [file]` — Batch from file or stdin

Local store:

- `archive-is-pp-cli sync` / `archive` / `export` / `import` — Local SQLite ops

Auth + health:

- `archive-is-pp-cli auth` — Config (no API key needed; auth is a no-op)
- `archive-is-pp-cli doctor` — Verify backend reachability

## Recipes

### Read a paywalled article

```bash
archive-is-pp-cli read "https://www.wsj.com/articles/..." --agent
# or: return just the text
archive-is-pp-cli get "https://www.wsj.com/articles/..." --format text --agent
```

`read` returns the archive URL (finding existing or creating new). `get --format text` returns the article body, falling back to Wayback if archive.today CAPTCHAs.

### Preserve a URL before it changes

```bash
archive-is-pp-cli save "https://example.com/important-page" --agent
archive-is-pp-cli history "https://example.com/important-page" --agent  # verify
```

Force capture, then check history to confirm the new snapshot registered.

### Bulk archive a research batch

```bash
grep -oE 'https?://[^ )]+' research-notes.md | archive-is-pp-cli bulk - --agent
# or from a file:
archive-is-pp-cli bulk urls.txt --agent
```

Reads URLs one per line, submits each with exponential backoff, returns per-URL status (archived, pre-existing, failed) as JSON.

### Wayback-preferred for a reliable-read

```bash
archive-is-pp-cli read "https://ft.com/content/xyz" --backend wayback,archive-is --agent
```

Use when the Wayback Machine snapshot is known to be cleaner or archive.today is rate-limiting.

## Auth Setup

**No API key required.** Archive.today and Wayback Machine are both public. The `auth` subcommand exists for consistency but is a no-op — `doctor` reports "Auth: not required" which is the expected state.

Optional env:
- `ARCHIVE_IS_BASE_URL` — override archive.today host (for mirrors)
- `WAYBACK_BASE_URL` — override Wayback Machine host

## Agent Mode

Add `--agent` to any command. Expands to `--json --compact --no-input --no-color --yes --no-prompt`. Every action command also prints structured `next_actions` hints on stderr when called non-interactively — the calling agent sees "tried X, got Y, consider Z" automatically.

Notable flags:
- `--submit-timeout <duration>` — max wait for a fresh submit (default `10m`; `0` = unbounded)
- `--backend archive-is,wayback` — backend preference and fallback order
- `--format text|html` — `get`/`tldr` output format

### Filtering output

`--select` accepts dotted paths to descend into nested responses; arrays traverse element-wise:

```bash
archive-is-pp-cli <command> --agent --select id,name
archive-is-pp-cli <command> --agent --select items.id,items.owner.name
```

Use this to narrow huge payloads to the fields you actually need — critical for deeply nested API responses.


### Response envelope

Data-layer commands wrap output in `{"meta": {...}, "results": <data>}`. Parse `.results` for data and `.meta.source` to know whether it's `live` or local. The `N results (live)` summary is printed to stderr only when stdout is a TTY; piped/agent consumers see pure JSON on stdout.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error |
| 3 | Not found (no snapshot exists) |
| 5 | API error (archive.today or Wayback down) |
| 7 | Rate limited (too many submits) |

## Installation

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/archive-is/cmd/archive-is-pp-cli@latest

# If `@latest` installs a stale build (Go module proxy cache lag), install from main:
GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
  go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/archive-is/cmd/archive-is-pp-cli@main
archive-is-pp-cli doctor
```

### MCP Server

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/archive-is/cmd/archive-is-pp-mcp@latest

# If `@latest` installs a stale build (Go module proxy cache lag), install from main:
GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
  go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/archive-is/cmd/archive-is-pp-mcp@main
claude mcp add archive-is-pp-mcp -- archive-is-pp-mcp
```

## Argument Parsing

Given `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → run `archive-is-pp-cli --help`
2. **`install`** → CLI; **`install mcp`** → MCP
3. **Anything that looks like a URL, or "archive <url>" / "bypass paywall on <url>"** → `read <url> --agent` is the default — it's idempotent and covers the 90% case.
4. **"bulk archive" / "archive these"** → `bulk` from stdin if URLs are pasted, else ask for the file path.

<!-- pr-218-features -->
## Agent Workflow Features

This CLI exposes three shared agent-workflow capabilities patched in from cli-printing-press PR #218.

### Named profiles

Persist a set of flags under a name and reuse them across invocations.

```bash
# Save the current non-default flags as a named profile
archive-is-pp-cli profile save <name>

# Use a profile — overlays its values onto any flag you don't set explicitly
archive-is-pp-cli --profile <name> <command>

# List / inspect / remove
archive-is-pp-cli profile list
archive-is-pp-cli profile show <name>
archive-is-pp-cli profile delete <name> --yes
```

Flag precedence: explicit flag > env var > profile > default.

### --deliver

Route command output to a sink other than stdout. Useful when an agent needs to hand a result to a file, a webhook, or another process without plumbing.

```bash
archive-is-pp-cli <command> --deliver file:/path/to/out.json
archive-is-pp-cli <command> --deliver webhook:https://hooks.example/in
```

File sinks write atomically (tmp + rename). Webhook sinks POST `application/json` (or `application/x-ndjson` when `--compact` is set). Unknown schemes produce a structured refusal listing the supported set.

### feedback

Record in-band feedback about this CLI from the agent side of the loop. Local-only by default; safe to call without configuration.

```bash
archive-is-pp-cli feedback "what surprised you or tripped you up"
archive-is-pp-cli feedback list         # show local entries
archive-is-pp-cli feedback clear --yes  # wipe
```

Entries append to `~/.archive-is-pp-cli/feedback.jsonl` as JSON lines. When `ARCHIVE_IS_FEEDBACK_ENDPOINT` is set and either `--send` is passed or `ARCHIVE_IS_FEEDBACK_AUTO_SEND=true`, the entry is also POSTed upstream (non-blocking — local write always succeeds).

