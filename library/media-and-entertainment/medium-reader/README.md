# Medium Reader

**Read any Medium author, publication, or tag as a local, full-text-searchable corpus.**

Medium Reader talks **directly to Medium's own public surfaces** — the RSS feeds, the article page, and Medium's internal `/_/graphql` endpoint — so it needs **no API key and no proxy**. Every command runs anonymously. It mirrors authors, publications, and tags into a local SQLite store, so you can archive a writer's entire body of work, search across everything you have synced, and see what is new in a topic — offline, in one command, agent-native.

Created by [@maxswinguy](https://github.com/maxswinguy) (Maxime Delavergne).

## Install

The recommended path installs both the `medium-reader-pp-cli` binary and the `pp-medium-reader` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install medium-reader
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install medium-reader --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install medium-reader --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install medium-reader --agent claude-code
npx -y @mvanhorn/printing-press-library install medium-reader --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/medium-reader/cmd/medium-reader-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/medium-reader-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install medium-reader --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-medium-reader --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-medium-reader --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install medium-reader --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/medium-reader-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. No credentials are required. The bundle optionally accepts a `MEDIUM_SESSION` cookie (your own Medium session) to unlock member full bodies — leave it blank to run anonymously.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/medium-reader/cmd/medium-reader-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "medium-reader": {
      "command": "medium-reader-pp-mcp"
    }
  }
}
```

To unlock member full bodies, add your own session cookie under `env`:

```json
{
  "mcpServers": {
    "medium-reader": {
      "command": "medium-reader-pp-mcp",
      "env": {
        "MEDIUM_SESSION": "sid=<sid>; uid=<uid>"
      }
    }
  }
}
```

</details>

## Authentication

**No key. No account. No setup.** Medium Reader reads Medium's public surfaces anonymously (this is "Tier 0"), so every command works the moment it is installed.

There is one **optional** layer, "Tier 1": your own Medium **session cookie**. Medium serves only a short preview body for member-locked articles to anonymous readers. If you are a logged-in Medium member, you can hand the CLI your own browser session so the `read` path returns the full article body you can already read in your browser. This is **your own cookie, never an API key** — and it is always optional.

Import it any of these ways (first hit wins):

```bash
# 1. Environment variable
export MEDIUM_SESSION="sid=<sid>; uid=<uid>"

# 2. A flat-JSON file
echo '{"sid":"<sid>","uid":"<uid>"}' > ~/.medium-cookies.json
medium-reader-pp-cli auth login --cookie-file ~/.medium-cookies.json
# (or set MEDIUM_COOKIE_FILE=~/.medium-cookies.json)
```

Copy `sid`/`uid` from your browser's medium.com cookies (DevTools → Application → Cookies → `https://medium.com`). Run `medium-reader-pp-cli auth login` with no arguments to see which path (if any) is currently providing a session; the token is always **masked** in output so scripted runs never leak it. See [Ethics & Terms of Service](#ethics--terms-of-service) before using a cookie.

**Extract from Chrome (optional, opt-in build).** For convenience, `auth login --chrome` can read your session straight from your local Chrome profile and save it for reuse:

```bash
medium-reader-pp-cli auth login --chrome --cookie-file ~/.medium-cookies.json
```

This requires a special build — `make build-chrome` (or `go build -tags kooky`) — because it adds a browser-cookie dependency and, on macOS, triggers a Keychain authorization prompt. The **default published binary keeps `--chrome` as a stub** that points you back to the env/file methods above (which work everywhere, including Windows and unattended/agent runs). On the opt-in build, `--chrome` saves the extracted cookie with `0600` permissions and prints only a masked confirmation — never the raw token.

## Quick Start

```bash
# health check — verifies Medium reachability, cookie tier, and local cache (no key needed)
medium-reader-pp-cli doctor

# read an author, publication, or tag's RSS feed (anonymous)
medium-reader-pp-cli feed @quincylarson --agent
medium-reader-pp-cli feed tag/ux --agent

# read a single article as Markdown
medium-reader-pp-cli read https://medium.com/p/818e7841df9c --agent

# search Medium for posts matching a query
medium-reader-pp-cli search "design systems" --limit 10 --agent

# mirror an author's full catalog into the local store, then query it offline
medium-reader-pp-cli author-archive @quincylarson --agent
medium-reader-pp-cli corpus "accessibility" --agent
```

## Unique Features

These capabilities aren't available in any other tool — and they all run keyless, against your local mirror.

### Local corpus that compounds
- **`author-archive`** — Mirror a writer's entire body of work into local SQLite, full-text searchable offline. Accepts either a 12-hex user id or a `@handle`/username (resolved keylessly from the author's public profile page).

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

## Recipes

### Archive an author and search their work offline

```bash
medium-reader-pp-cli author-archive uxdesigncc --agent && medium-reader-pp-cli corpus "accessibility" --data-source local --agent
```

Mirror a writer's catalog once, then search it with no further network calls.

### Read a tag feed, then pull the full text of one post

```bash
medium-reader-pp-cli feed tag/artificial-intelligence --agent --select id,title,author
medium-reader-pp-cli read <id> --agent --select title,word_count,is_preview_only
```

Scan a topic feed, then fetch the body of the article you care about.

### Pull a member-locked article you can read as a subscriber

```bash
export MEDIUM_SESSION="sid=<sid>; uid=<uid>"
medium-reader-pp-cli read 818e7841df9c --agent
```

Anonymously, `read` returns the short preview (`is_preview_only: true`). With your own member session, it returns the full body you can already read in your browser.

## Usage

Run `medium-reader-pp-cli --help` for the full command reference and flag list.

## Commands

### feed

Read a Medium author, publication, or tag RSS feed (no key, no cookies).

- **`medium-reader-pp-cli feed <@user|publication|tag>`** — Auto-detects the reference kind (`@name` = user, `tag/<name>` or a bare word = tag, otherwise a publication slug), fetches the public RSS 2.0 feed, and returns a normalized list of recent posts.

### read

Read a Medium article as Markdown (no key, no cookies).

- **`medium-reader-pp-cli read <url|id>`** — Fetches the article page, parses the embedded post JSON, and renders the body as Markdown. Member-locked articles return a preview body anonymously (`is_preview_only: true`); supply a Tier-1 cookie to unlock the full body.

### search

Search Medium for posts matching a query (no key, no cookies).

- **`medium-reader-pp-cli search <query> --limit N`** — Queries Medium's internal GraphQL search and returns matching posts (id, title, author, username, published-at).

### author-archive

Mirror a writer's entire body of work into local SQLite, full-text searchable offline.

- **`medium-reader-pp-cli author-archive <userIdOrHandle> --max-articles N`** — Resolves a `@handle` to a user id keylessly, fetches the author's post ids via GraphQL, reads each body via the page source, and upserts them into the local store for `corpus`/`digest`/`author-compare`.

### corpus

Full-text and regex search across everything you have synced locally (authors, publications, tags).

### digest

A deduped, ranked 'what is new since last sync' across the authors, publications, and tags you have archived.

### author-compare

Compare two writers on output cadence, topic mix, and engagement (claps and voters per article) from locally archived data.

### analytics

Run analytics queries on locally synced data.

### Utility commands

- **`doctor`** — Check CLI health: Medium reachability (via the Chrome-impersonation transport), cookie tier, and local cache report.
- **`auth login`** — Report or import the optional Tier-1 session cookie (see [Authentication](#authentication)).
- **`which "<capability>"`** — Resolve a natural-language capability query to the best matching command.
- **`agent-context`** — Emit structured JSON describing this CLI for agents.
- **`profile`** — Save and reuse named sets of flags.
- **`feedback`** — Record feedback about this CLI (local by default).
- **`version`** — Print version.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
medium-reader-pp-cli feed tag/ux

# JSON for scripting and agents
medium-reader-pp-cli feed tag/ux --json

# Filter to specific fields
medium-reader-pp-cli feed tag/ux --json --select id,title,author

# Agent mode — JSON + compact + no prompts in one flag
medium-reader-pp-cli feed tag/ux --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,title` returns only fields you need
- **Read-only** - this CLI does not create, update, delete, publish, send, or mutate any Medium resource
- **Offline-friendly** - `corpus`/`digest`/`author-compare` query the local SQLite store with no network calls
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` source error, `7` rate limited, `10` config error.

## Health Check

```bash
medium-reader-pp-cli doctor
```

Verifies Medium reachability through the Chrome-impersonation transport, reports the current cookie tier (Tier 0 anonymous or Tier 1), and summarizes the local cache. No credentials required.

## Configuration

Medium Reader needs no configuration to run. The only settings are the optional Tier-1 cookie inputs:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `MEDIUM_SESSION` | env | No | Your own Medium session cookie (`sid=..; uid=..`, or a bare sid). Unlocks member full bodies. |
| `MEDIUM_COOKIE_FILE` | env | No | Path to a flat-JSON cookie file `{"sid":"..","uid":".."}`. The `--cookie-file` flag overrides it. |

The local store lives at `~/.local/share/medium-reader-pp-cli/data.db` (feedback at `feedback.jsonl` alongside it).

## Ethics & Terms of Service

Medium Reader is built for **personal reading, research, and archiving of content you have legitimate access to**. Please use it accordingly:

- **Respect Medium's Terms of Service.** This tool reads the same public pages and feeds your browser does, through a standard browser-like client. It is rate-limit-aware (`--rate-limit`) — don't hammer Medium.
- **The Tier-1 cookie is your own session, only.** It is sent only to Medium (never to any third party), is kept out of all output (masked), and unlocks only the bodies you can already read as a logged-in member. Store the cookie file with restrictive permissions. Never use someone else's session.
- **Don't bulk-collect or redistribute copyrighted content.** Archiving a writer's catalog for your own offline reading and search is fine; republishing or redistributing it is not.
- **This is a reader, not a writer.** It never posts, comments, claps, follows, or mutates anything on Medium. Medium's official write API was closed to new integrations in 2025.

## If Medium changes

Medium Reader depends on Medium's own public surfaces (RSS, the article page, and the internal GraphQL endpoint), which can change without notice. The design degrades gracefully: `feed` and `read` keep working even if GraphQL changes, and `search`/`author-archive` surface a clear "surface unavailable; Medium may have changed its internal API" error rather than failing silently.

**If a surface breaks, the fix lives here.** This CLI is a printed, self-contained Go program in the [`mvanhorn/printing-press-library`](https://github.com/mvanhorn/printing-press-library) monorepo — there is no external API service to wait on. Open a PR against the library (or file an issue) with what changed, and the source adapter (`internal/source/rss`, `internal/source/page`, or `internal/source/graphql`) can be updated directly.

## Troubleshooting

**Source errors (exit code 5)**
- Run `medium-reader-pp-cli doctor` to confirm Medium is reachable and see your cookie tier.
- `search`/`author-archive` reporting "surface unavailable" means Medium changed its internal GraphQL API — `feed`/`read` should still work. See [If Medium changes](#if-medium-changes).

**`read` returns only a short preview (`is_preview_only: true`)**
- That article is member-locked and you are running anonymously. Import your own session cookie (see [Authentication](#authentication)) to unlock the full body.

**Rate limited (exit code 7)**
- Slow down with `--rate-limit <rps>`, or archive once and query offline with `--data-source local`.

## HTTP Transport

This CLI uses a Chrome-compatible HTTP transport (TLS/JA3 browser impersonation) so Medium's public surfaces serve it the same way they serve a browser, with no API key and no resident browser process.

---

## Sources & Inspiration

Related open-source projects in the same space:

- [**medium-api**](https://github.com/weeping-angel/medium-api) — Python (250 stars)
- [**medium-api-js**](https://github.com/weeping-angel/medium-api-js) — JavaScript (40 stars)
- [**medium-mcp-server**](https://github.com/Dishant27/medium-mcp-server) — TypeScript (30 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
