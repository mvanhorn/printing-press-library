---
name: pp-hackernews
description: "Use this skill whenever the user asks about Hacker News, trending tech stories, what's on HN, who's hiring, Show HN / Ask HN, Y Combinator news, or wants to research discussion activity on a topic (Rust, AI, databases, etc.). Hacker News CLI powered by Firebase (real-time) + Algolia (search). No auth required. Triggers on natural phrasings like 'what's trending on HN', 'any good Show HN this week', 'who's hiring remote Rust engineers', 'what did HN say about the new Apple announcement', 'was this link already posted'."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["hackernews-pp-cli"]},"install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/hackernews/cmd/hackernews-pp-cli@latest","bins":["hackernews-pp-cli"],"label":"Install via go install"}]}}'
---

# Hacker News — Printing Press CLI

Hacker News from your terminal. Browse, search, and analyze stories with pipe-friendly output. Powered by both the Firebase API (real-time story IDs and threads) and the Algolia Search API (full-text search with filters). No auth required — HN is fully public.

## When to Use This CLI

Reach for this when a user wants to scan HN without opening the browser, research discussion patterns on a topic, dig into hiring trends, analyze a specific thread, or pipe HN data into downstream tools. Designed for agent consumption: every command emits clean JSON with `--json` and `comments --flat` is meant for piping into an LLM for summarization.

Don't reach for this when the user just wants to read a single story — the HN site itself is faster for that. Reach for this when they want structure, filtering, aggregation, or offline access.

## Unique Capabilities

These commands aren't available in any other HN tool.

### Time-windowed discovery

- **`since <duration> [--sort velocity] [--min-points N]`** — What hit HN while you were away. One command, no setup.

  _Solves the "I was offline for 12 hours, what did I miss" problem without scrolling through 20 pages of the site._

- **`show [--hot] [--since 7d]`** — Show HN sorted by velocity or best-of-week.

- **`ask [--topic <t>] [--unanswered]`** — Ask HN filtered by topic or find questions with no top-level responses.

### Discussion analytics

- **`tldr <story_id>`** — Mechanical thread digest: top takes, most active commenters, controversy score. Not an LLM summary — structured extraction you can pipe into an LLM.

  _Pairs perfectly with `| claude "summarize in 5 bullets"` for agentic thread reading._

- **`controversial [--since 24h] [--min-comments 50]`** — Stories with high comment-to-point ratios. These are the fights, the divisive topics, the stories where people disagree.

- **`pulse <topic>`** — Activity timeline + stats for a topic. "What's HN saying about Rust this week?" → points plot, story count, top voices.

### Hiring intelligence

- **`hiring [regex] [--remote] [--tech] [--salary]`** — Smart "Who is Hiring?" filtering. Accepts a regex as a positional argument for keyword matching (e.g., `hiring "rust"`), then apply boolean filters for remote / tech stack / salary info. Much richer than naive grep.

- **`hiring-stats [--months N]`** — Aggregate hiring data across months: most requested languages, remote percentage, salary ranges, trends over time.

### Utility

- **`repost <url>`** — Check if a URL was posted before (and how it did). Runs the Algolia URL lookup.

- **`my <username>`** — Submission tracking for a user: which posts got traction, best posting times, karma delta.

- **`since` with piped output** — `hackernews-pp-cli stories --json | jq '.results[] | select(.score > 100)'` becomes standard tooling for custom filtering.

## Command Reference

Story discovery:

- `hackernews-pp-cli stories` — Current top stories
- `hackernews-pp-cli stories top|new|best|ask|show|jobs` — Type-specific feeds
- `hackernews-pp-cli get <storyId>` — Single story with full metadata
- `hackernews-pp-cli comments <id>` — Comment tree for a story

Search:

- `hackernews-pp-cli search <query>` — Full-text search via Algolia
- `hackernews-pp-cli query <query>` — Same, with advanced filters
- `hackernews-pp-cli by-date <query>` — Time-sorted search

Users:

- `hackernews-pp-cli users <userId>` — User profile and karma
- `hackernews-pp-cli my <username>` — Track a user's submissions with traction analysis

Jobs:

- `hackernews-pp-cli jobs` — Current job listings
- `hackernews-pp-cli hiring [regex]` — Filtered "Who is Hiring?" parsing
- `hackernews-pp-cli hiring-stats` — Aggregate trends across months

Local data:

- `hackernews-pp-cli sync` — Sync stories to local SQLite for offline search
- `hackernews-pp-cli archive` / `export <resource>` / `import <resource>` — Local store ops

Unique commands (see Unique Capabilities): `since`, `show`, `ask`, `tldr`, `controversial`, `pulse`, `repost`.

## Recipes

### "What did I miss on HN in the last day?"

```bash
hackernews-pp-cli since 24h --min-points 100 --sort velocity --agent
```

Returns stories posted in the last 24 hours with at least 100 points, sorted by how fast they're climbing. One call replaces 20 minutes of scrolling.

### Research a topic's week of activity

```bash
hackernews-pp-cli pulse "AI agents" --agent
hackernews-pp-cli by-date "AI agents" --tags story --agent | jq '.hits[0:5]'
```

`pulse` gives the temporal shape (when did activity spike, who commented most); `by-date` pulls the top 5 matching stories for deeper reading.

### Hiring market intel for remote Rust

```bash
hackernews-pp-cli hiring "rust" --remote --tech --agent
hackernews-pp-cli hiring-stats --months 6 --agent
```

First call lists current-month posts that mention Rust and remote. Second call shows multi-month trends — is Rust growing or cooling on HN, what's the remote share, typical salary bands.

### Summarize a massive discussion thread

```bash
hackernews-pp-cli tldr 38795924 --agent           # structured top takes
hackernews-pp-cli comments 38795924 --flat --agent | claude "summarize in 5 bullets"
```

`tldr` extracts structure (top-voted takes, active commenters, controversy score). The flat comments dump pipes cleanly into an LLM for prose summary.

### Check if your blog post was already shared

```bash
hackernews-pp-cli repost "https://example.com/my-post" --agent
```

Returns any matching submissions — points, date, commenter count. Tells you if it's worth resubmitting (dead thread, low points) or not (already viral once).

## Auth Setup

**None required.** Hacker News APIs (Firebase + Algolia) are fully public. The `auth` subcommand exists for consistency with other Printing Press CLIs but is a no-op here — `doctor` will report "Auth: not required" as a warning, which is expected.

Optional config:
- `HACKERNEWS_CONFIG` — override config file path
- `HACKERNEWS_BASE_URL` — override the Firebase base URL (rarely needed)

## Agent Mode

Add `--agent` to any command. Expands to `--json --compact --no-input --no-color --yes` — structured, pipe-friendly, no prompts. Use `--select <fields>` to cherry-pick fields for compact agent context, `--dry-run` to preview the API request, `--no-cache` to bypass the 5-minute GET cache.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error |
| 3 | Not found (story/user/item) |
| 5 | API error (upstream Firebase or Algolia issue) |
| 7 | Rate limited |

## Installation

### CLI

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/hackernews/cmd/hackernews-pp-cli@latest

# If `@latest` installs a stale build (Go module proxy cache lag), install from main:
GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
  go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/hackernews/cmd/hackernews-pp-cli@main
hackernews-pp-cli doctor  # verifies API reachability
```

### MCP Server

10 public MCP tools exposed (no auth gating — everything works via MCP).

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/hackernews/cmd/hackernews-pp-mcp@latest

# If `@latest` installs a stale build (Go module proxy cache lag), install from main:
GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
  go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/hackernews/cmd/hackernews-pp-mcp@main
claude mcp add hackernews-pp-mcp -- hackernews-pp-mcp
```

## Argument Parsing

Given `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → run `hackernews-pp-cli --help`
2. **`install`** → CLI install; **`install mcp`** → MCP install
3. **Anything else** → check `which hackernews-pp-cli` (offer install if missing), match user intent to a command (lean on Unique Capabilities for time-windowed or analytical queries; Command Reference for direct lookups), run with `--agent` for structured output.

<!-- pr-218-features -->
## Agent Workflow Features

This CLI exposes three shared agent-workflow capabilities patched in from cli-printing-press PR #218.

### Named profiles

Persist a set of flags under a name and reuse them across invocations.

```bash
# Save the current non-default flags as a named profile
hackernews-pp-cli profile save <name>

# Use a profile — overlays its values onto any flag you don't set explicitly
hackernews-pp-cli --profile <name> <command>

# List / inspect / remove
hackernews-pp-cli profile list
hackernews-pp-cli profile show <name>
hackernews-pp-cli profile delete <name> --yes
```

Flag precedence: explicit flag > env var > profile > default.

### --deliver

Route command output to a sink other than stdout. Useful when an agent needs to hand a result to a file, a webhook, or another process without plumbing.

```bash
hackernews-pp-cli <command> --deliver file:/path/to/out.json
hackernews-pp-cli <command> --deliver webhook:https://hooks.example/in
```

File sinks write atomically (tmp + rename). Webhook sinks POST `application/json` (or `application/x-ndjson` when `--compact` is set). Unknown schemes produce a structured refusal listing the supported set.

### feedback

Record in-band feedback about this CLI from the agent side of the loop. Local-only by default; safe to call without configuration.

```bash
hackernews-pp-cli feedback "what surprised you or tripped you up"
hackernews-pp-cli feedback list         # show local entries
hackernews-pp-cli feedback clear --yes  # wipe
```

Entries append to `~/.hackernews-pp-cli/feedback.jsonl` as JSON lines. When `HACKERNEWS_FEEDBACK_ENDPOINT` is set and either `--send` is passed or `HACKERNEWS_FEEDBACK_AUTO_SEND=true`, the entry is also POSTed upstream (non-blocking — local write always succeeds).

