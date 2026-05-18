---
name: pp-skool
description: "Every Skool community feature, plus a local SQLite mirror, FTS, and cross-community ops no other Skool tool ships."
author: "quoxientzero"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - skool-pp-cli
---

# Skool — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `skool-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install skool --cli-only
   ```
2. Verify: `skool-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Community + course platform (skool.com). No public API — reverse-engineered from www.skool.com Next.js data routes (reads) and api2.skool.com REST (writes), authenticated via the auth_token JWT cookie.

## When to Use This CLI

Use this CLI when you operate or moderate a Skool community and need analytics, exports, or automation that the native UI cannot do — classroom-to-markdown export, cross-community SQL, or scheduled digests for an agent.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`posts top`** — Rank recent posts by upvotes, comments, or engagement and return them with full content.

  _Pick this for a daily/weekly cron that surfaces the 3-5 most-engaging posts from any community — perfect for catching up without scrolling._

  ```bash
  skool-pp-cli posts top --community earlyaidopters --since 7d --top 5 --by engagement --json
  ```
- **`leaderboard`** — Top members by points for the community, with level and bio fields included.

  _Pick this when an agent needs the current community leaderboard in one call without scraping the page._

  ```bash
  skool-pp-cli leaderboard --community bewarethedefault --top 25 --json
  ```
- **`digest since`** — Aggregate everything new across posts, comments, members, and lessons since a timestamp.

  _Pick this when an agent needs a single brief of community activity for a daily/weekly cron._

  ```bash
  skool-pp-cli digest since 24h --json
  ```
- **`sql`** — Run read-only SQL across every community in your local store.

  _Pick this when an agent needs to compose a query across multiple Skool communities you own or operate._

  ```bash
  skool-pp-cli sql 'SELECT community, COUNT(*) FROM posts GROUP BY community'
  ```

### Agent-native plumbing
- **`calendar export`** — Export upcoming community events to an .ics file for Google Cal / Outlook.

  _Pick this when a member wants community events on their personal calendar without manual entry._

  ```bash
  skool-pp-cli calendar export --ics > community.ics
  ```
- **`classroom export`** — Export an entire course to a markdown bundle (modules, lessons, attachments, video URLs).

  _Pick this when an agent needs to ingest a course for offline reference, search, or LLM retrieval._

  ```bash
  skool-pp-cli classroom export <course-slug> --out ./course/
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**calendar** — Community calendar events

- `skool-pp-cli calendar <buildId>` — List upcoming and recent calendar events

**classroom** — Classroom (courses, modules, lessons) for a community

- `skool-pp-cli classroom get-course` — Get a single course with its modules and lessons
- `skool-pp-cli classroom list` — List all courses in a community

**community** — Community feed, settings, and metadata

- `skool-pp-cli community about` — About page (rules, owner, member count)
- `skool-pp-cli community info` — Get the community feed (posts, leaderboard summary, upcoming events, settings)
- `skool-pp-cli community leaderboard-tab` — Leaderboard tab (community page rendered with t=leaderboard)
- `skool-pp-cli community members-tab` — Members tab data (community page rendered with t=members)

**me** — Current authenticated user dashboard

- `skool-pp-cli me <buildId>` — Get current user, joined communities, and dashboard state

**members** — Community members and moderation

- `skool-pp-cli members approve` — Approve a pending member request
- `skool-pp-cli members ban` — Ban a member from the community
- `skool-pp-cli members pending` — List pending member join requests
- `skool-pp-cli members reject` — Reject a pending member request

**notifications** — User notifications

- `skool-pp-cli notifications list` — List notifications for the authenticated user
- `skool-pp-cli notifications mark-read` — Mark notifications as read (empty ids = mark all)

**posts** — Posts (forum threads) inside a community

- `skool-pp-cli posts comment` — Add a comment to a post
- `skool-pp-cli posts create` — Create a new post (body = TipTap JSON; use --md to convert markdown)
- `skool-pp-cli posts delete` — Delete a post
- `skool-pp-cli posts get` — Get a post detail page including comment tree
- `skool-pp-cli posts like` — Like (upvote) a post
- `skool-pp-cli posts unlike` — Unlike a post
- `skool-pp-cli posts update` — Update an existing post


## Freshness Contract

This printed CLI owns bounded freshness only for registered store-backed read command paths. In `--data-source auto` mode, those paths check `sync_state` and may run a bounded refresh before reading local data. `--data-source local` never refreshes. `--data-source live` reads the API and does not mutate the local store. Set `SKOOL_NO_AUTO_REFRESH=1` to skip the freshness hook without changing source selection.

Covered paths:

- `skool-pp-cli notifications`
- `skool-pp-cli notifications list`
- `skool-pp-cli notifications mark-read`

When JSON output uses the generated provenance envelope, freshness metadata appears at `meta.freshness`. Treat it as current-cache freshness for the covered command path, not a guarantee of complete historical backfill or API-specific enrichment.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
skool-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Daily digest cron

```bash
skool-pp-cli sync bewarethedefault && skool-pp-cli digest since 24h --json --select new_posts,new_members,top_comments
```

One sync + one query. Pipes cleanly to a scheduled agent that drafts a Slack/email digest.

### Export a course to markdown

```bash
skool-pp-cli classroom export ai-foundations --out ./ai-foundations/
```

Recursive walk: modules → lessons → attachments + Mux URLs. One folder per course, ready for LLM ingestion.

### Cross-community engagement SQL

```bash
skool-pp-cli sql 'SELECT community, COUNT(*) AS posts_30d FROM posts WHERE created_at > date("now","-30 days") GROUP BY community ORDER BY posts_30d DESC'
```

Read-only SQL over the local store. Works across every community you have synced.

### Top 10 leaderboard

```bash
skool-pp-cli leaderboard --community bewarethedefault --top 10 --json
```

Current 30-day leaderboard. Members with rank, points, level, bio fields.

## Multi-community use

The CLI supports any Skool community you're a member of — the `auth_token` cookie covers all communities you're logged into globally. Switch communities via the `--community <slug>` flag on any command, or change the default in `~/.config/skool-pp-cli/config.toml` under `[template_vars]`.

```bash
# Default community from config
skool-pp-cli digest since 24h

# Override per call
skool-pp-cli digest since 24h --community early-ai-adopters --json
skool-pp-cli leaderboard --community some-other-community --top 10

# Sync multiple communities into one local store (each row gets a community tag)
skool-pp-cli sync --community bewarethedefault
skool-pp-cli sync --community early-ai-adopters

# Cross-community SQL once both are synced
skool-pp-cli sql "SELECT community, resource_type, COUNT(*) FROM resources GROUP BY community, resource_type"

# Bash one-liner: daily digest across N communities
for c in bewarethedefault early-ai-adopters another-community; do
  skool-pp-cli digest since 24h --community $c --json --select community,new_post_count,new_posts >> ~/skool-daily-digest.jsonl
done
```

The community-tagged store is the foundation for the v0.3 cross-community analytics commands (at-risk members, churn cohort, engagement profile across communities).

### Top posts by engagement (signal in the noise)

```bash
skool-pp-cli posts top --community earlyaidopters --since 7d --top 5 --by engagement --json
skool-pp-cli posts top --community bewarethedefault --since 24h --top 3 --by upvotes
```

The headline daily/weekly command. Walks paginated community feed, dedups, filters to window, ranks by upvotes / comments / engagement / newest, returns full post body so the agent has everything it needs in one call.

## Auth Setup

Skool has no public API. Authenticate with the auth_token JWT cookie from your logged-in browser session: `skool-pp-cli auth set-token` (writes ~/.config/skool-pp-cli/config.toml). Same cookie covers reads and writes; CloudFront requires a realistic User-Agent which the CLI sets automatically.

Run `skool-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  skool-pp-cli calendar mock-value --community example-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

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
skool-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
skool-pp-cli feedback --stdin < notes.txt
skool-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.skool-pp-cli/feedback.jsonl`. They are never POSTed unless `SKOOL_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SKOOL_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
skool-pp-cli profile save briefing --json
skool-pp-cli --profile briefing calendar mock-value --community example-value
skool-pp-cli profile list --json
skool-pp-cli profile show briefing
skool-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `skool-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/skool/cmd/skool-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add skool-pp-mcp -- skool-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which skool-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   skool-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `skool-pp-cli <command> --help`.
