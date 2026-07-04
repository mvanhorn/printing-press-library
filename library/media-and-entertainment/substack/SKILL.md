---
name: pp-substack
description: "Run your Substack growth and authoring loop from the command line — publish rich drafts, manage a multi-publication portfolio, and measure what drives growth. Trigger phrases: `post a substack note`, `schedule a week of substack notes`, `find substack swap partners`, `which of my notes drove subs`, `what's my engagement reciprocity`, `voice-match a substack note`, `best time to post on substack`, `create a substack draft`, `sync my substack portfolio`, `top posts across my publications`, `search my substack posts`, `use substack`, `run substack`."
author: "user"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - substack-pp-cli
    install:
      - kind: go
        bins: [substack-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack/cmd/substack-pp-cli
---

# Substack — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `substack-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install substack --cli-only
   ```
2. Verify: `substack-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack/cmd/substack-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Reach for this CLI when an agent needs to operate a Substack publication end-to-end: posting Notes on a cadence, drafting and publishing long-form, engaging with niche writers, finding swap partners, and measuring which content actually drove subs. It is the right pick over WriteStack/StackSweller when you need agent-native plumbing (--json, --select, --dry-run, typed exit codes), offline-first analytics (every join runs locally over SQLite), or coverage of the writer surface those tools don't expose. Engage write actions (`engage like`, `engage restack`, `engage restack-with-comment`) print a curl-equivalent by default and only fire with `--send`; treat the print-curl output as a preflight, not a live action.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`growth attribution`** — Connect every Note you posted to the paid and free subscribers that actually arrived in the 24-hour window after, so you stop guessing which content drove growth.

  _Pick this over a generic stats call when an agent needs to decide which Note formats to repeat next week._

  ```bash
  substack-pp-cli growth attribution --days 30 --json --select rank,note_id,note_excerpt,subs_acquired,paid_subs_acquired
  ```
- **`engage reciprocity`** — See net-give/net-take per writer you engage with — who reciprocates your restacks/comments, who quietly free-rides on yours.

  _Use when an agent is deciding whether to keep investing in a swap partner; surfaces relationships before they go stale._

  ```bash
  substack-pp-cli engage reciprocity --days 30 --agent --select handle,outgoing,incoming,net,drift
  ```

### Algorithm-aware automation
- **`notes schedule --guard`** — Refuse to fire (or queue) a Note that lands less than 30 minutes after your last own-Note or violates your time-of-day rotation. Returns typed exit 2 with a JSON diagnosis.

  _Stops an agent from accidentally torching its own reach by dumping a queue all at once._

  ```bash
  substack-pp-cli notes schedule --at 2026-05-10T13:00:00Z --body "hook line\n\nbody" --guard --json
  ```
- **`growth best-time`** — Top day-of-week × hour cells ranked for whichever growth signal you pick (paid subs, likes, restacks, or comments) — not a single average.

  _An agent picking when to schedule tomorrow's Notes can ask for the goal it's optimizing instead of guessing._

  ```bash
  substack-pp-cli growth best-time --days 90 --for-goal subs --json --select day_of_week,hour,rate,sample_size
  ```

### Pattern intelligence
- **`discover patterns`** — Mechanically extracts which hook patterns (curiosity-gap colon, 3-sentence formula, em-dash reframe, question opener) actually rank in a niche, with restack/comment ratios.

  _An agent drafting Notes can ask which hook shape currently outperforms in this niche before generating._

  ```bash
  substack-pp-cli discover patterns --niche productivity --sort restacks --since 14d --agent --select pattern,sample_count,avg_restacks,avg_comments,top_example
  ```
- **`voice fingerprint`** — Measurable voice profile — sentence length, em-dash rate, colon-hook rate, hook-line ratios, vocabulary uniqueness — for any handle, with --diff to compare against another writer.

  _An agent drafting Notes for a ghostwriter client can verify the output stays inside the client's voice envelope._

  ```bash
  substack-pp-cli voice fingerprint --handle maya --diff devon --json --select metric,self,other,delta
  ```

### Network leverage
- **`recs find-partners`** — Score candidate publications for a Substack Recommendations swap by mutual-overlap density across followee + recommendation graphs.

  _An agent running a weekly cross-promo pass can rank candidates instead of pitching cold._

  ```bash
  substack-pp-cli recs find-partners --my-pub on --top 20 --json --select rank,handle,pub,overlap_score,shared_followees
  ```
- **`growth pod`** — Given a list of handles, render a member × member engagement matrix — last 30 days of restacks/comments/likes between every pair.

  _An agent organizing a mutual-aid pod can see who's net-positive vs free-riding without a spreadsheet._

  ```bash
  substack-pp-cli growth pod --members maya,devon,priya,jordan --days 30 --json
  ```

### Authoring with rich field control

- **`drafts create` / `drafts update`** — Full Substack draft API surface: 30+ flags covering title, subtitle, body (Markdown auto-converts to ProseMirror), section-id, type (newsletter/podcast/video/thread), audience, bylines, SEO metadata, social title, cover image, comment settings, podcast/video URLs, and visibility toggles. The only authoring path that gives agents field-level control without fighting a web editor.

  _Use when an agent is constructing a complete long-form post from structured data — research summary, translated copy, ghostwritten piece — and needs paywall, SEO, and section placement set in one command._

  ```bash
  export SUBSTACK_PUBLICATION=mypub
  substack-pp-cli drafts create --title "Why X matters" \
    --body-file ./post.md --audience only_paid \
    --seo-title "X explained" --seo-description "How X affects Y" \
    --cover-image https://substackcdn.com/.../cover.jpg --json
  ```

### Portfolio & analytics (local columnar store)

These commands read a **local SQLite store** populated by `portfolio sync`. The workflow is:

```
auth login --chrome  →  export SUBSTACK_PUBLICATION=<your-pub>  →  portfolio sync  →  portfolio / posts best / grep / subs churn / …
```

Custom-domain publications are supported: `auth login --chrome` captures the Creator-session cookie from the custom domain automatically.

- **`portfolio sync`** — The data-population command. Discovers every publication you own and writes posts, subscribers, and drafts into the local columnar store. Must be run before `portfolio`, `posts best`, `grep`, `schedule board`, `subs churn`, and `subs cross-sell` can return cross-publication data.

  ```bash
  export SUBSTACK_PUBLICATION=mypub
  substack-pp-cli portfolio sync --json
  ```

- **`portfolio`** — One-screen status of every publication you own: subscriber count, paid count, posts published, drafts pending, next scheduled. No tab-switching, no CSV exports.

  ```bash
  substack-pp-cli portfolio --json
  ```

- **`posts best`** — Rank posts by views, likes, comments, or restacks within a window. `--cross-pub` aggregates across all your publications.

  _Use when an agent is deciding which posts to twin into a new publication or surface in a weekly newsletter._

  ```bash
  substack-pp-cli posts best --by restacks --window 30d --cross-pub --json
  substack-pp-cli posts best --by views --limit 5 --publication mypub-en
  ```

- **`posts twin <slug> --to <pub>`** — Duplicate a published post into another publication you own as a draft. Preserves paywall markers, section mapping, and re-uploads images to the target CDN.

  ```bash
  substack-pp-cli posts twin my-en-slug --to mypub-de --dry-run --json
  substack-pp-cli posts twin my-en-slug --to mypub-de
  ```

- **`posts pair <en> <de>` / `posts pairs [--missing]`** — Record EN↔DE post pairings in a local table. `--missing` lists posts without a recorded twin — feed that list into `posts twin` to spin up the missing translations.

  ```bash
  substack-pp-cli posts pair my-en-slug my-de-slug
  substack-pp-cli posts pairs --missing --publication mypub-en --json
  ```

- **`grep <query>`** — FTS5 full-text search across synced posts, notes, and comments, ranked by bm25, returning snippets and source URLs. Optional `--scope`, `--publication`, and `--since` filters.

  ```bash
  substack-pp-cli grep "yield curve" --json
  substack-pp-cli grep "rate hike" --scope posts --publication mypub-en --since 2024-01-01
  ```

- **`schedule board`** — ASCII calendar of the next N days showing scheduled posts across every publication you own. Multi-publication editorial overview in one screen.

  ```bash
  substack-pp-cli schedule board --days 30 --json
  ```

- **`subs churn`** — Diff subscriber snapshots: who newly subscribed, who unsubscribed, who upgraded free→paid, who downgraded paid→free. Run `--snapshot` at least once first to create a baseline.

  ```bash
  substack-pp-cli subs churn --snapshot
  substack-pp-cli subs churn --since 7d --json --publication mypub-paid
  ```

- **`subs cross-sell`** — Emails that pay on at least one of your publications but are free or absent on the others. Requires 2+ owned publications in the local store. The cross-sell list Substack's UI does not ship.

  ```bash
  substack-pp-cli subs cross-sell --json --limit 100
  ```

## Command Reference

**categories** — Site-wide Substack category list — culture, technology, food, etc.

- `substack-pp-cli categories list` — List all Substack categories
- `substack-pp-cli categories list-publications` — List publications in a category

**comments** — Long-form post comments (distinct from Notes)

- `substack-pp-cli comments get` — Get a single comment by ID (same shape as a Note — Substack treats them uniformly)
- `substack-pp-cli comments list` — List comments on a post

**discover** — Discovery surfaces — search publications, embed metadata

- `substack-pp-cli discover` — Search Substack publications by query

**drafts** — Drafts CRUD + publish + schedule

- `substack-pp-cli drafts create` — Create a new draft
- `substack-pp-cli drafts delete` — Delete a draft
- `substack-pp-cli drafts get` — Get a draft by ID
- `substack-pp-cli drafts list` — List drafts
- `substack-pp-cli drafts prepublish` — Validate a draft for publication; returns blockers
- `substack-pp-cli drafts publish` — Publish a draft now
- `substack-pp-cli drafts schedule` — Schedule a draft for future publish (or unschedule with --post-date null)
- `substack-pp-cli drafts update` — Update an existing draft

**feed** — RSS feed for a publication

- `substack-pp-cli feed` — RSS XML feed (returns XML; use `--raw` to dump)

**images** — Image upload (data-URI JSON, not multipart)

- `substack-pp-cli images` — Upload an image; returns CDN URL. Body is data-URI JSON.

**inbox** — Authenticated reader feed (home feed) — Notes + posts surfaced for the current user

- `substack-pp-cli inbox home` — Authenticated home feed
- `substack-pp-cli inbox reader-posts` — Posts feed for current user

**notes** — Substack Notes — short-form posts (Substack treats Notes as comments internally)

- `substack-pp-cli notes create` — Post a new Note (POST /comment/feed). Body is ProseMirror JSON.
- `substack-pp-cli notes get` — Get a single Note by ID
- `substack-pp-cli notes list-by-profile` — List Notes by a profile (cursor pagination)
- `substack-pp-cli notes reply` — Reply to an existing Note (parent_id + ProseMirror body)

**grep** — Full-text search across synced posts, notes, and comments

- `substack-pp-cli grep <query>` — FTS5 search ranked by bm25, returning snippets and source URLs. Flags: `--scope posts|notes|comments|all`, `--publication`, `--since`, `--limit`

**portfolio** — Multi-publication status dashboard and data-population

- `substack-pp-cli portfolio` — One-screen status of every publication you own (subs, paid, posts, drafts, next scheduled). Run `portfolio sync` first.
- `substack-pp-cli portfolio sync` — Discover every publication you own and populate the local columnar store (publications/posts/subscribers/drafts). The prerequisite for all cross-publication analytics commands.

**posts** — Long-form posts and archives on a specific publication

- `substack-pp-cli posts archive` — Public archive of a publication's posts
- `substack-pp-cli posts best` — Rank cached posts by engagement metric (`--by views|likes|comments|restacks`, `--window`, `--cross-pub`, `--limit`, `--publication`)
- `substack-pp-cli posts get-by-slug` — Get a published post by URL slug
- `substack-pp-cli posts list-published` — List published posts on the publication (auth required)
- `substack-pp-cli posts pair <en-slug> <de-slug>` — Record an EN↔DE translation pairing in the local table
- `substack-pp-cli posts pairs` — List recorded post pairs; `--missing` shows posts without a twin; `--publication` filters to one pub
- `substack-pp-cli posts ranked-authors` — Ranked list of authors for a publication
- `substack-pp-cli posts twin <slug> --to <pub>` — Duplicate a published post into another publication you own as a draft (re-uploads images, preserves paywall markers)

**profiles** — Substack profiles — your own and other writers'

- `substack-pp-cli profiles from-linkedin` — Look up a Substack profile from a LinkedIn handle
- `substack-pp-cli profiles get-by-handle` — Get a public profile by handle (e.g. mvanhorn)
- `substack-pp-cli profiles get-by-id` — Get a public profile by numeric user ID
- `substack-pp-cli profiles handle-options` — Available handle suggestions for the current user
- `substack-pp-cli profiles posts` — All posts by an author across publications
- `substack-pp-cli profiles self` — Get the authenticated user's profile

**recommendations** — Substack Recommendations — outbound (publications I recommend)

- `substack-pp-cli recommendations <publication_id>` — List the publications a publication recommends

**sections** — Sections of a publication (newsletters can have multiple)

- `substack-pp-cli sections` — List sections + subscriptions

**settings** — Account settings + connectivity probe (used by doctor)

- `substack-pp-cli settings get` — Get account settings
- `substack-pp-cli settings ping` — Connectivity probe (non-destructive PUT used by doctor)

**schedule** — Cross-publication editorial scheduling

- `substack-pp-cli schedule board` — ASCII calendar of the next N days (`--days`) of scheduled posts across all owned publications

**subs** — Subscriber count, churn diff, and cross-sell analytics

- `substack-pp-cli subs authors` — List bylined authors of a publication
- `substack-pp-cli subs churn` — Diff subscriber snapshots (new/unsubscribed/upgraded/downgraded). Use `--snapshot` to create a baseline, then `--since` to diff. Flags: `--publication`, `--since`, `--snapshot`
- `substack-pp-cli subs count` — Get subscriber count (read off the launch-checklist payload)
- `substack-pp-cli subs cross-sell` — Emails paid on one publication but free/absent on others (requires 2+ owned pubs in local store). Flags: `--limit`

**tags** — Post tags

- `substack-pp-cli tags create` — Create a new tag
- `substack-pp-cli tags list` — List all tags for the publication


## Freshness Contract

This printed CLI owns bounded freshness only for registered store-backed read command paths. In `--data-source auto` mode, those paths check `sync_state` and may run a bounded refresh before reading local data. `--data-source local` never refreshes. `--data-source live` reads the API and does not mutate the local store. Set `SUBSTACK_NO_AUTO_REFRESH=1` to skip the freshness hook without changing source selection.

Covered paths:

- `substack-pp-cli categories`
- `substack-pp-cli categories get`
- `substack-pp-cli categories list`
- `substack-pp-cli categories search`
- `substack-pp-cli drafts`
- `substack-pp-cli drafts get`
- `substack-pp-cli drafts list`
- `substack-pp-cli drafts search`
- `substack-pp-cli inbox`
- `substack-pp-cli inbox get`
- `substack-pp-cli inbox list`
- `substack-pp-cli inbox search`
- `substack-pp-cli inbox-posts`
- `substack-pp-cli inbox-posts get`
- `substack-pp-cli inbox-posts list`
- `substack-pp-cli inbox-posts search`
- `substack-pp-cli posts`
- `substack-pp-cli posts get`
- `substack-pp-cli posts list`
- `substack-pp-cli posts search`
- `substack-pp-cli posts-published`
- `substack-pp-cli posts-published get`
- `substack-pp-cli posts-published list`
- `substack-pp-cli posts-published search`
- `substack-pp-cli posts-ranked`
- `substack-pp-cli posts-ranked get`
- `substack-pp-cli posts-ranked list`
- `substack-pp-cli posts-ranked search`
- `substack-pp-cli profiles`
- `substack-pp-cli profiles get`
- `substack-pp-cli profiles list`
- `substack-pp-cli profiles search`
- `substack-pp-cli sections`
- `substack-pp-cli sections get`
- `substack-pp-cli sections list`
- `substack-pp-cli sections search`
- `substack-pp-cli subs`
- `substack-pp-cli subs get`
- `substack-pp-cli subs list`
- `substack-pp-cli subs search`
- `substack-pp-cli tags`
- `substack-pp-cli tags get`
- `substack-pp-cli tags list`
- `substack-pp-cli tags search`

When JSON output uses the generated provenance envelope, freshness metadata appears at `meta.freshness`. Treat it as current-cache freshness for the covered command path, not a guarantee of complete historical backfill or API-specific enrichment.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
substack-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Daily growth-loop morning ritual

```bash
substack-pp-cli growth attribution --days 7 --agent --select rank,note_excerpt,subs_acquired
```

Surfaces yesterday's Note→sub winners. Pair with `substack-pp-cli engage reciprocity --days 7 --agent` to see whose engagement reciprocates yours, and `substack-pp-cli sync --since 24h` ahead of time to keep the local store fresh.

### Schedule a Note with the cadence guard

```bash
substack-pp-cli notes schedule --at 2030-05-13T09:00:00Z --body 'Tuesday hook line' --guard --json
```

Queues the Note locally; --guard refuses scheduling if it lands within 30 min of an existing own-Note (typed exit 2 + JSON diagnosis). Drop --guard or add --send to fire immediately.

### Find this week's swap partners

```bash
substack-pp-cli recs find-partners --my-pub on --top 5 --json --select rank,handle,pub,overlap_score
```

Ranks candidate publications by audience overlap; pipe to your draft-outreach tool of choice (substack-pp-cli does the ranking; outreach drafting is left to your agent's prompt).

### Capture a writer's voice fingerprint as JSON

```bash
substack-pp-cli voice fingerprint --handle alice --diff bob --json
```

Mechanical voice metrics for the named handle, with a delta against another writer when --diff is set. Save the JSON yourself; agent generation prompts can ingest it.

### Surface deeply nested Note metadata with --select

```bash
substack-pp-cli notes get c-12345 --agent --select id,body,attachments.url,attachments.image_url,attachments.published_bylines.name,attachments.published_bylines.handle,context.users.name
```

Notes responses are deeply nested (attachments, bylines, contextual users). Dotted --select narrows the payload so an agent doesn't burn context parsing 30KB of JSON it doesn't need.

### Bootstrap the portfolio analytics store

```bash
substack-pp-cli auth login --chrome
export SUBSTACK_PUBLICATION=mypub
substack-pp-cli portfolio sync --json
substack-pp-cli portfolio --json
```

Run this once after login. Every cross-publication analytics command (`posts best`, `grep`, `schedule board`, `subs churn`, `subs cross-sell`) reads the local store that `portfolio sync` populates.

### Publish a rich draft with SEO and cover image

```bash
export SUBSTACK_PUBLICATION=mypub
substack-pp-cli drafts create \
  --title "The case for X" \
  --subtitle "Three reasons it matters now" \
  --body-file ./post.md \
  --audience only_paid \
  --seo-title "Case for X" \
  --seo-description "Why X matters for Y" \
  --cover-image https://substackcdn.com/.../cover.jpg \
  --json
```

Converts the Markdown body to Substack's ProseMirror format automatically. The target publication is resolved from `$SUBSTACK_PUBLICATION`. Drop `--audience only_paid` for a public post.

### Twin your best EN post into a DE publication

```bash
# Sync first so the local store has current posts
substack-pp-cli portfolio sync --json

# Find top post by restacks
substack-pp-cli posts best --by restacks --limit 1 --publication mypub-en --json

# Preview the twin operation, then create the draft
substack-pp-cli posts twin my-en-slug --to mypub-de --dry-run --json
substack-pp-cli posts twin my-en-slug --to mypub-de --json
```

### Weekly subscriber churn digest

```bash
# Run this weekly (automate with a cron or agent schedule)
substack-pp-cli subs churn --since 7d --json --publication mypub-paid
```

First run ever needs a baseline: `substack-pp-cli subs churn --snapshot`. After that, `--since 7d` compares against the nearest snapshot within that window.

### Full-text search across all your publications

```bash
substack-pp-cli grep "interest rates" --scope posts --json
substack-pp-cli grep "reader question" --scope notes --since 2025-01-01 --limit 20 --json
```

## Auth Setup

Substack uses a session cookie (substack.sid). The only path today is `auth login --chrome` (also accepts `--browser` as an alias) — it reads the cookie from your logged-in Chrome via pycookiecheat / cookies / cookie-scoop-cli and stores it in the OS keyring. There is no password login and no manual cookie-paste subcommand. If your cookie expires, re-run `auth login --chrome`.

Run `substack-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  substack-pp-cli categories list --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
substack-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
substack-pp-cli feedback --stdin < notes.txt
substack-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/substack-pp-cli/feedback.jsonl`. They are never POSTed unless `SUBSTACK_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SUBSTACK_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
substack-pp-cli profile save briefing --json
substack-pp-cli --profile briefing categories list
substack-pp-cli profile list --json
substack-pp-cli profile show briefing
substack-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `substack-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack/cmd/substack-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add substack-pp-mcp -- substack-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which substack-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   substack-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `substack-pp-cli <command> --help`.
