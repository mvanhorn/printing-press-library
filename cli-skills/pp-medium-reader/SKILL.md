---
name: pp-medium-reader
description: "Read any Medium author, publication, or tag as a local, full-text-searchable corpus — no API key, no account. Trigger phrases: `archive this Medium author`, `get the full text of this Medium article`, `read the UX tag feed on Medium`, `search Medium for <topic>`, `search my Medium corpus`, `use medium-reader-pp-cli`, `run medium-reader`."
author: "Maxime Delavergne"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - medium-reader-pp-cli
    install:
      - kind: go
        bins: [medium-reader-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/media-and-entertainment/medium-reader/cmd/medium-reader-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/media-and-entertainment/medium-reader/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Medium Reader — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `medium-reader-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install medium-reader --cli-only
   ```
2. Verify: `medium-reader-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/medium-reader/cmd/medium-reader-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Medium Reader reads Medium's own public surfaces (RSS, the article page, and the internal GraphQL endpoint) **directly — no API key, no proxy**. It mirrors authors, publications, and tags into a local SQLite store, so you can archive a writer's entire body of work, search across everything you have synced, and see what is new in a topic — offline, in one command, agent-native.

## When to Use This CLI

Use this CLI when an agent needs to read, archive, search, or analyze Medium content — a specific author's catalog, a publication's articles, a tag's RSS feed, or the full text of an article. It is the right tool when the task is research, monitoring, or building a local corpus of Medium writing. It works with zero credentials.

## Anti-triggers

Do not use this CLI for:
- Do not use to publish, update, clap, comment on, or follow on Medium; this is a read-only reader (Medium's official write API was closed to new integrations in 2025).
- Do not use to bulk-collect or redistribute copyrighted Medium content; this is for personal reading and research of content you have access to.
- Do not use for non-Medium blogs or RSS in general.

## Authentication

**None required.** Every command runs anonymously, with no key and no account ("Tier 0").

There is one **optional** layer ("Tier 1"): your own Medium **session cookie**, which unlocks the full body of member-locked articles on the `read` path (anonymously, those return only a short preview). It is your own browser session, never an API key, and is always optional:

```bash
export MEDIUM_SESSION="sid=<sid>; uid=<uid>"
# or: medium-reader-pp-cli auth login --cookie-file <path>   (flat JSON {"sid":"..","uid":".."})
# or (opt-in `-tags kooky` build only): medium-reader-pp-cli auth login --chrome --cookie-file <path>
```

Copy `sid`/`uid` from your browser's medium.com cookies (DevTools → Application → Cookies). Run `medium-reader-pp-cli auth login` to report the current tier (the token is masked). The `--chrome` auto-extract is a stub in the default binary (it points back to the env/file paths) and only does the real read in an opt-in `-tags kooky` build. Send only your own cookie, only to Medium.

## Core read commands (Tier 0, keyless)

- **`feed <@user|publication|tag>`** — Read the public RSS feed for an author (`@name`), a tag (`tag/<name>`), or a publication slug.

  ```bash
  medium-reader-pp-cli feed tag/ux --agent
  ```
- **`read <url|id>`** — Read a single article as Markdown. Member-locked posts return a preview anonymously; a Tier-1 cookie unlocks the full body.

  ```bash
  medium-reader-pp-cli read https://medium.com/p/818e7841df9c --agent
  ```
- **`search <query> --limit N`** — Search Medium for posts matching a query (via the internal GraphQL endpoint).

  ```bash
  medium-reader-pp-cli search "design systems" --limit 10 --agent
  ```

## Unique Capabilities

These capabilities aren't available in any other tool — and they run keyless, against your local mirror.

### Local corpus that compounds
- **`author-archive`** — Mirror a writer's entire body of work into local SQLite, full-text searchable offline. Accepts a 12-hex user id or a `@handle`/username (resolved keylessly from the public profile page).

  _Reach for this when you need a complete, queryable copy of one author's writing rather than the 10-item RSS window or one-article-at-a-time fetches._

  ```bash
  medium-reader-pp-cli author-archive @quincylarson --agent
  ```
- **`corpus`** — Full-text and regex search across everything you have synced locally (authors, publications, tags).

  _Use this to find a half-remembered passage across all the writing you have archived, without touching the network._

  ```bash
  medium-reader-pp-cli corpus "design systems" --agent --select title,author,url
  ```
- **`digest`** — A deduped, ranked 'what is new since last sync' across the authors, publications, and tags you have archived.

  _Use this as a personal what-did-I-miss feed across everything you have synced, computed offline._

  ```bash
  medium-reader-pp-cli digest --since 7d --agent
  ```

### Comparative analysis
- **`author-compare`** — Compare two writers on output cadence, topic mix, and engagement (claps and voters per article) from locally archived data.

  _Use this to weigh two writers or publications before committing to follow or archive one._

  ```bash
  medium-reader-pp-cli author-compare @quincylarson uxdesigncc --agent
  ```

## HTTP Transport

This CLI uses a Chrome-compatible HTTP transport (browser TLS impersonation) so Medium's public surfaces serve it like a browser — no API key and no resident browser process.

## Command Reference

**feed** — Read a Medium author, publication, or tag RSS feed (no key, no cookies).

- `medium-reader-pp-cli feed <@user|publication|tag>` — Auto-detects ref kind and returns recent posts from the public RSS feed.

**read** — Read a Medium article as Markdown (no key, no cookies).

- `medium-reader-pp-cli read <url|id>` — Renders the article body as Markdown; preview-only for member-locked posts unless a Tier-1 cookie is set.

**search** — Search Medium for posts matching a query (no key, no cookies).

- `medium-reader-pp-cli search <query> --limit N` — Returns matching posts (id, title, author, username, published-at).

**author-archive** — Mirror a writer's entire body of work into local SQLite.

- `medium-reader-pp-cli author-archive <userIdOrHandle> --max-articles N` — Resolves a handle keylessly, archives the catalog into the local store.

**corpus** — Full-text and regex search across everything synced locally.

**digest** — A deduped, ranked 'what is new since last sync' across what you have archived.

**author-compare** — Compare two writers on cadence, topic mix, and engagement.

**analytics** — Run analytics queries on locally synced data.

**doctor** — Check CLI health (Medium reachability, cookie tier, local cache).

**auth login** — Report or import the optional Tier-1 session cookie.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
medium-reader-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Archive an author and search their work offline

```bash
medium-reader-pp-cli author-archive uxdesigncc --agent && medium-reader-pp-cli corpus "accessibility" --data-source local --agent
```

Mirror a writer's catalog once, then search it with no further network calls.

### Scan a tag feed, then pull one full article

```bash
medium-reader-pp-cli feed tag/artificial-intelligence --agent --select id,title,author
medium-reader-pp-cli read <id> --agent --select title,word_count,is_preview_only
```

### Pull a member-locked article you can read as a subscriber

```bash
export MEDIUM_SESSION="sid=<sid>; uid=<uid>"
medium-reader-pp-cli read 818e7841df9c --agent
```

Anonymously this returns the preview; with your own member session it returns the full body.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise:

  ```bash
  medium-reader-pp-cli feed tag/ux --agent --select id,title,author
  ```
- **Offline-friendly** — `corpus`/`digest`/`author-compare` query the local SQLite store with no network calls
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — never use this CLI for create, update, delete, publish, comment, clap, follow, or other mutating requests; it has none

### Response envelope

Commands that read from the local store wrap output in a provenance envelope:

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
medium-reader-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
medium-reader-pp-cli feedback --stdin < notes.txt
medium-reader-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/medium-reader-pp-cli/feedback.jsonl`. They are never POSTed unless `MEDIUM_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `MEDIUM_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration.

```
medium-reader-pp-cli profile save briefing --json
medium-reader-pp-cli --profile briefing feed tag/ux
medium-reader-pp-cli profile list --json
medium-reader-pp-cli profile show briefing
medium-reader-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | Source error (Medium unreachable or changed; see `doctor`) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `medium-reader-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/medium-reader/cmd/medium-reader-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add medium-reader-pp-mcp -- medium-reader-pp-mcp
   ```
3. Verify: `claude mcp list`

The MCP server exposes 10 keyless tools (feed, read, search, author_archive, author_compare, corpus, digest, analytics, plus a local SQL tool and a context tool). To unlock member full bodies, pass your own cookie via the `MEDIUM_SESSION` env on the server.

## Direct Use

1. Check if installed: `which medium-reader-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Core read commands, Unique Capabilities, and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   medium-reader-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `medium-reader-pp-cli <command> --help`.
