---
name: pp-instagram
description: "Printing Press CLI for Instagram. Instagram Web API (reverse-engineered from authenticated browser session)"
author: "Bryan.Aaron"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - instagram-pp-cli
---

# Instagram — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `instagram-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install instagram --cli-only
   ```
2. Verify: `instagram-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Instagram Web API (reverse-engineered from authenticated browser session)

## HTTP Transport

This CLI uses standard HTTP transport with HTTP/2 disabled for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 0 API entries from 0 total network entries
- Protocols: rest_json (95% confidence), graphql (70% confidence)
- Generation hints: requires_browser_auth, requires_browser_http, chrome_ua_required

## Command Reference

**explore** — Explore/discover content

- `instagram-pp-cli explore` — Get explore grid content (trending posts)

**feed** — Home feed and user posts

- `instagram-pp-cli feed liked` — Get media liked by the authenticated user
- `instagram-pp-cli feed saved` — Get the authenticated user's saved posts
- `instagram-pp-cli feed timeline` — Get the home feed (posts from accounts you follow)
- `instagram-pp-cli feed user_posts` — Get posts from a specific user

**friendships** — Follow relationships and pending requests

- `instagram-pp-cli friendships followers` — Get followers of a user
- `instagram-pp-cli friendships following` — Get accounts a user is following
- `instagram-pp-cli friendships pending` — Get pending follow requests (for private accounts)

**graphql** — Instagram GraphQL API (for complex queries)

- `instagram-pp-cli graphql` — Execute a GraphQL query against the Instagram API

**hashtags** — Hashtag search and info

- `instagram-pp-cli hashtags` — Get info about a hashtag including post count

**locations** — Location search

- `instagram-pp-cli locations` — Search for locations and places

**media** — Posts, reels, and media

- `instagram-pp-cli media comments` — Get comments on a post
- `instagram-pp-cli media info` — Get detailed info for a specific post/reel
- `instagram-pp-cli media likers` — Get users who liked a post

**notifications** — Activity notifications

- `instagram-pp-cli notifications inbox` — Get the notifications inbox (likes, comments, follows, mentions)
- `instagram-pp-cli notifications mark_seen` — Mark notifications as seen

**reels** — Reels/Clips

- `instagram-pp-cli reels` — Get reels posted by a specific user

**stories** — Stories and highlights

- `instagram-pp-cli stories highlights_tray` — Get highlights for a user
- `instagram-pp-cli stories reels_media` — Get story reels for specific users
- `instagram-pp-cli stories tray` — Get the stories tray (all unviewed stories from followed accounts)

**users** — User profiles and account info

- `instagram-pp-cli users current_user` — Get the authenticated user's own account info
- `instagram-pp-cli users profile` — Get full user profile by username
- `instagram-pp-cli users search` — Search users, hashtags, and places


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
instagram-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

This CLI uses a browser session. Log in to .instagram.com in Chrome, then:

```bash
instagram-pp-cli auth login --chrome
```

Requires a cookie extraction tool (`pycookiecheat` via pip, or `cookies` via Homebrew).

Run `instagram-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  instagram-pp-cli graphql --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
instagram-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
instagram-pp-cli feedback --stdin < notes.txt
instagram-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.instagram-pp-cli/feedback.jsonl`. They are never POSTed unless `INSTAGRAM_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `INSTAGRAM_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
instagram-pp-cli profile save briefing --json
instagram-pp-cli --profile briefing graphql
instagram-pp-cli profile list --json
instagram-pp-cli profile show briefing
instagram-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `instagram-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add instagram-pp-mcp -- instagram-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which instagram-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   instagram-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `instagram-pp-cli <command> --help`.
