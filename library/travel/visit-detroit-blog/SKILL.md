---
name: pp-visit-detroit-blog
description: "Detroit's official \"Inside the D\" blog as a CLI: offline full-text search, cross-axis filtering by category and neighborhood, related reads, and reading-list exports. Trigger phrases: `what should I do in Detroit`, `Detroit dining articles in Corktown`, `best things to do in Greektown`, `read the Detroit donuts article`, `recent Inside the D blog posts`, `use visit-detroit-blog`, `run visit-detroit-blog`."
author: "user"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - visit-detroit-blog-pp-cli
---

# Visit Detroit Blog — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `visit-detroit-blog-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install visit-detroit-blog --cli-only
   ```
2. Verify: `visit-detroit-blog-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/visit-detroit-blog/cmd/visit-detroit-blog-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Inside the D CLI mirrors Visit Detroit's editorial blog into a local SQLite store, so you can full-text search every article body offline and then slice it by category, neighborhood, and date in a single query (blogs list) — something the site's single-facet instant-search can't do. blogs related surfaces articles that share the most topics and neighborhoods; blogs coverage maps where the blog is dense or thin; blogs reading-list exports a neutral, sponsored-free reading list for a team.

## When to Use This CLI

Reach for this CLI when an agent or user needs Detroit travel and editorial recommendations grounded in Visit Detroit's official blog: finding articles by neighborhood and topic together, reading full article bodies offline, discovering related reads, or assembling a shareable reading list. It is the right tool for 'what should I do, eat, or see in <Detroit neighborhood>' questions and for separating editorial from sponsored content. It is not a hotel/restaurant directory — it is the editorial blog.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Queries the website can't express
- **`blogs list`** — Filter Detroit articles by category AND neighborhood AND date window in one query — the slice the website's single-facet search can't express.

  _Reach for this when a request combines a topic and a place (and optionally recency) — e.g. 'recent dining articles in Corktown' — instead of issuing several single-facet searches and intersecting them by hand._

  ```bash
  inside-the-d-pp-cli blogs list --category Dining --region Corktown --since 2026-01-01 --agent
  ```
- **`blogs related`** — Find the articles that share the most categories and neighborhoods with a given post, ranked.

  _Use this to chain recommendations — after surfacing one good article, hand the user (or yourself) the next best reads without another keyword search._

  ```bash
  inside-the-d-pp-cli blogs related donuts --limit 5 --agent
  ```
- **`blogs coverage`** — Cross-tabulate article counts across every category and every neighborhood to see where coverage is dense or thin.

  _Use this to answer 'which neighborhoods have the most Outdoors coverage' or to spot gaps before recommending an under-covered area._

  ```bash
  inside-the-d-pp-cli blogs coverage --category Outdoors --agent
  ```

### Take it with you
- **`blogs reading-list`** — Materialize an ordered, deduped reading list (markdown/json/csv) from any filter to a file — with an option to drop sponsored posts for a neutral handout.

  _Use this when a person needs to hand a curated, source-stable list to a team or attendee — not a one-off query that disappears when the tab closes._

  ```bash
  inside-the-d-pp-cli blogs reading-list --region "Downtown Detroit" --category Culture --no-sponsored --output detroit-culture.md
  ```

## Command Reference

Run `visit-detroit-blog-pp-cli sync` once to populate the local store, then:

**blogs** — browse, read, and analyze articles

- `visit-detroit-blog-pp-cli blogs list` — filter articles across category, neighborhood, and date
- `visit-detroit-blog-pp-cli blogs get <slug>` — read a full article by slug, URI, or id
- `visit-detroit-blog-pp-cli blogs related <slug>` — articles sharing the most categories and neighborhoods
- `visit-detroit-blog-pp-cli blogs coverage` — category × neighborhood cross-tab
- `visit-detroit-blog-pp-cli blogs reading-list` — export an ordered md/json/csv reading list

**Top-level**

- `visit-detroit-blog-pp-cli search <query>` — offline ranked full-text search
- `visit-detroit-blog-pp-cli categories` — list blog categories with article counts
- `visit-detroit-blog-pp-cli regions` — list neighborhoods/regions with article counts
- `visit-detroit-blog-pp-cli recent` — newest articles by post date
- `visit-detroit-blog-pp-cli sync` — pull all articles into the local store


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
visit-detroit-blog-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Recent dining articles in a neighborhood

```bash
visit-detroit-blog-pp-cli blogs list --category Dining --region "Eastern Market" --since 2025-01-01
```

Cross-axis filter: topic plus place plus recency in one query the site's single-facet search can't run.

### Neutral attendee reading list

```bash
visit-detroit-blog-pp-cli blogs reading-list --region "Downtown Detroit" --category Culture --no-sponsored --output detroit-culture.md
```

Builds a stable, sponsored-free markdown reading list to hand to a team — every web search is otherwise ephemeral.

### Where is the blog dense vs thin?

```bash
visit-detroit-blog-pp-cli blogs coverage --category Outdoors
```

Cross-tabulates Outdoors coverage by neighborhood — a two-dimensional view Algolia's facet API can't return.

### Agent: narrow a large result to high-gravity fields

```bash
visit-detroit-blog-pp-cli search "patio season" --agent --select title,uri,snippet --limit 5
```

Pairs --agent with --select so the agent gets only title, URL, and summary instead of full bodies for every hit — bounded context.

### Chain from one good article to the next

```bash
visit-detroit-blog-pp-cli blogs related ikea --limit 5 --agent
```

Returns the most topically and geographically similar posts to an article, as JSON, for follow-up recommendations.

## Auth Setup

No authentication required.

Run `visit-detroit-blog-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on long article bodies:

  ```bash
  visit-detroit-blog-pp-cli blogs list --agent --select title,url,categories
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

### Output shape

Commands emit a **bare JSON array** on stdout under `--json`/`--agent` or when piped — a single JSON object for single-article commands like `blogs get`. There is no wrapper envelope, so parse the array directly:

```bash
visit-detroit-blog-pp-cli blogs list --category Dining --agent | jq '.[].title'
```

On an interactive terminal with no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`), output is a human-readable table instead. All article data is read from the local SQLite store, which `sync` populates from Algolia.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
visit-detroit-blog-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
visit-detroit-blog-pp-cli feedback --stdin < notes.txt
visit-detroit-blog-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.visit-detroit-blog-pp-cli/feedback.jsonl`. They are never POSTed unless `VISIT_DETROIT_BLOG_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `VISIT_DETROIT_BLOG_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
visit-detroit-blog-pp-cli profile save briefing --json
visit-detroit-blog-pp-cli --profile briefing blogs
visit-detroit-blog-pp-cli profile list --json
visit-detroit-blog-pp-cli profile show briefing
visit-detroit-blog-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `visit-detroit-blog-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add visit-detroit-blog-pp-mcp -- visit-detroit-blog-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which visit-detroit-blog-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   visit-detroit-blog-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `visit-detroit-blog-pp-cli <command> --help`.
