---
name: pp-substack-creator
description: "Every Substack feature, plus a local SQLite database, full-text search across years of writing, and the only... Trigger phrases: `publish to my Substack`, `draft a Substack post`, `subscriber churn on Substack`, `list my Substack drafts`, `Substack portfolio`, `search my Substack archive`, `use substack-creator-pp-cli`, `run substack-creator-pp-cli`."
author: "JimPresting"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - substack-creator-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/media-and-entertainment/substack-creator/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

<!-- // PATCH: skill-install-canonical — restored the canonical SKILL.md install section so verify-skill's canonical-sections drift check passes after manifest got a category. See .printing-press-patches.json. -->

# Substack — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `substack-creator-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install substack-creator --cli-only
   ```
2. Verify: `substack-creator-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack-creator/cmd/substack-creator-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Reach for substack-creator-pp-cli when an agent or workflow needs to operate across more than one Substack publication, build offline analytics from synced data, or run any command that the web UI does not expose (subscriber churn diffs, cross-publication ranking, cross-sell joins, FTS over years of writing). Single-publication read-only tasks can usually run against postcli or jakub-k-slys/substack-api; everything that requires local state, cross-pub joins, or the portfolio view is what this CLI is for.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Multi-publication workflow
- **`portfolio`** — One-screen status of every publication you own: subscriber count, paid count, last-published-at, drafts pending, next scheduled. No tab-switching, no CSV exports.

  _When an agent or human owns multiple Substacks (English + German, free + premium tiers), this is the only command that answers 'what is the state of all of them right now'._

  ```bash
  substack-creator-pp-cli portfolio --json
  ```
- **`posts twin`** — Duplicate a published post into another publication you own as a draft. Preserves paywall markers, sections, and re-uploads images to the target publication's CDN.

  _Bilingual or multi-tier creators copy-paste between publications today. This collapses the ritual into one command and leaves the target draft ready for translation or pricing-tier adjustment._

  ```bash
  substack-creator-pp-cli posts twin my-post-slug --to mypub-de --dry-run
  ```
- **`posts pairs`** — Record EN<->DE post pairings in a local table. 'posts pair <en> <de>' adds; 'posts pairs --missing' lists posts in one language with no recorded twin in the other.

  _Bilingual newsletter owners forget which posts already have a translation. This command answers it offline and feeds the 'posts twin' flow._

  ```bash
  substack-creator-pp-cli posts pairs --missing --publication mypub-en --json
  ```
- **`schedule board`** — ASCII calendar of the next 30 days showing scheduled posts across every publication you own. Multi-pub editorial overview in one screen.

  _Editorial planning across publications needs one calendar. This is the only command that renders it._

  ```bash
  substack-creator-pp-cli schedule board --json
  ```

### Local state that compounds
- **`subscribers churn`** — Diff two SQLite snapshots of your subscriber list: who newly subscribed, who unsubscribed, who upgraded free->paid, who downgraded paid->free, since a chosen window.

  _Agents auditing retention want named churn rows, not aggregate counts. Sunday-evening review or weekly automation reads this list and pipes it forward._

  ```bash
  substack-creator-pp-cli subscribers churn --publication mypub-paid --since 7d --json
  ```
- **`subscribers cross-sell`** — SQL join across your publications' subscriber lists: emails paid on one publication but free or absent on the others, sorted by paid-publication coverage. The cross-sell list Substack does not ship.

  _The most obvious upsell candidates are paying readers on one of your newsletters who don't know your other ones exist. This command surfaces them for a once-a-month email blast._

  ```bash
  substack-creator-pp-cli subscribers cross-sell --json
  ```
- **`posts best`** — Rank posts by views, likes, comments, or restacks within a window. Optionally aggregate across every publication you own to find your overall top performer.

  _For repurposing decisions, you need the best post across the portfolio, not within one pub. This is the input for Monday-morning content planning._

  ```bash
  substack-creator-pp-cli posts best --by restacks --window 30d --cross-pub --json
  ```
- **`grep`** — FTS5 over post bodies + Notes + comments, ranked by bm25, returning snippets and source URLs. Optional scope (posts/notes/comments/all), publication, and since filter.

  _Agents and writers re-citing their own writing need full-archive search across years. Substack cannot do this; this CLI ships it as a one-liner._

  ```bash
  substack-creator-pp-cli grep "yield curve" --scope all --since 2024-01-01 --json
  ```

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 0 API entries from 0 total network entries

## Command Reference

**categories** — Substack content categories.

- `substack-creator-pp-cli categories list` — List all global categories.
- `substack-creator-pp-cli categories newsletters` — List newsletters in a category.

**comments** — Comments on posts.

- `substack-creator-pp-cli comments add` — Add a comment to a post.
- `substack-creator-pp-cli comments list` — List comments on a post.
- `substack-creator-pp-cli comments react` — React to a comment.

**dashboard** — Publication analytics and engagement stats.

- `substack-creator-pp-cli dashboard <publication_id>` — Aggregate dashboard stats for a publication you own.

**drafts** — Manage post drafts.

- `substack-creator-pp-cli drafts create` — Create a new draft.
- `substack-creator-pp-cli drafts delete` — Delete a draft.
- `substack-creator-pp-cli drafts get` — Get a draft by ID.
- `substack-creator-pp-cli drafts list` — List your drafts.
- `substack-creator-pp-cli drafts preview` — Get an author-only preview link for a draft.
- `substack-creator-pp-cli drafts publish` — Publish a draft.
- `substack-creator-pp-cli drafts update` — Update an existing draft.

**feed** — Your reader feed.

- `substack-creator-pp-cli feed` — Get your feed.

**images** — Upload images to Substack's CDN.

- `substack-creator-pp-cli images` — Upload an image.

**me** — Your own subscriptions, follows, and personal recommendations.

- `substack-creator-pp-cli me follows` — Profiles you follow.
- `substack-creator-pp-cli me recommendations` — Personal recommendations.
- `substack-creator-pp-cli me subscriptions` — What you subscribe to.

**notes** — Substack Notes (microblog).

- `substack-creator-pp-cli notes list` — List your recent notes.
- `substack-creator-pp-cli notes publish` — Publish a new note.
- `substack-creator-pp-cli notes react` — React to a note.
- `substack-creator-pp-cli notes reply` — Reply to a note.
- `substack-creator-pp-cli notes restack` — Restack a note.

**posts** — Read and interact with your published posts.

- `substack-creator-pp-cli posts get` — Get a single post by slug.
- `substack-creator-pp-cli posts list` — List your own posts (drafts + published).
- `substack-creator-pp-cli posts react` — React to a post (heart it).
- `substack-creator-pp-cli posts restack` — Restack a post to your Notes.
- `substack-creator-pp-cli posts stats` — Engagement stats (likes/comments/restacks) for a post.

**profiles** — Read your own or another user's Substack profile.

- `substack-creator-pp-cli profiles get` — Get another user's profile by handle.
- `substack-creator-pp-cli profiles me` — Get your own profile and publications.

**publications** — Search and inspect Substack publications globally.

- `substack-creator-pp-cli publications recommendations` — List publications recommended by a given publication.
- `substack-creator-pp-cli publications search` — Search publications by query.

**sections** — Publication sections / categories.

- `substack-creator-pp-cli sections <publication_id>` — List sections for a publication.

**subscribers** — Manage your publication's subscribers.

- `substack-creator-pp-cli subscribers add` — Add a subscriber by email.
- `substack-creator-pp-cli subscribers count` — Get total subscriber counts (free + paid).
- `substack-creator-pp-cli subscribers export_free` — Export free subscribers as CSV.
- `substack-creator-pp-cli subscribers export_paid` — Export paid subscribers as CSV.
- `substack-creator-pp-cli subscribers list` — List subscribers for a publication you own.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
substack-creator-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Sunday churn review

```bash
substack-creator-pp-cli sync --full && substack-creator-pp-cli subscribers churn --since 7d --json --select email,event,delta_at,publication
```

Refresh state, then list every named subscriber event of the last week in a flat structure ready to pipe into a spreadsheet

### Cross-sell email list

```bash
substack-creator-pp-cli subscribers cross-sell --json --select email,paid_on,free_on,ltv | jq -r '.[] | [.email, .paid_on, (.free_on|join(";"))]|@csv'
```

Find paying readers on one of your publications who are free or absent on the others, formatted as CSV for a one-off campaign

### Mirror an English post to German pub as draft

```bash
substack-creator-pp-cli posts twin my-en-slug --to mypub-de --dry-run && substack-creator-pp-cli posts twin my-en-slug --to mypub-de
```

Preview the twin operation (re-uploaded images, paywall markers, section mapping), then run it for real

### Find every mention of a topic across years

```bash
substack-creator-pp-cli grep "yield curve" --scope all --since 2024-01-01 --json --select title,publication,publish_date,snippet,url
```

FTS5 search across posts, notes, and comments returning matched snippets with dotted-path field selection so the agent context stays small

### Weekly portfolio brief for agents

```bash
substack-creator-pp-cli portfolio --json --select publications,subscriber_total,paid_total,drafts_pending,scheduled_next
```

Compact summary for an agent to read once at the top of a session; uses dotted-path --select to drop unnecessary fields

## Auth Setup

Substack has no API tokens. The CLI reads your `connect.sid` and `substack.sid` cookies from a logged-in Chrome session: run `auth login --chrome` once, and the cookies are saved to `~/.config/substack-creator-pp-cli/config.toml`. When the session expires (every few weeks), log into Substack in your browser and rerun the command. No password, no OTP, no scraped credentials.

Run `substack-creator-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  substack-creator-pp-cli categories list --agent --select id,name,status
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
substack-creator-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
substack-creator-pp-cli feedback --stdin < notes.txt
substack-creator-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.substack-creator-pp-cli/feedback.jsonl`. They are never POSTed unless `SUBSTACK_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SUBSTACK_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
substack-creator-pp-cli profile save briefing --json
substack-creator-pp-cli --profile briefing categories list
substack-creator-pp-cli profile list --json
substack-creator-pp-cli profile show briefing
substack-creator-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `substack-creator-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add substack-creator-pp-mcp -- substack-creator-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which substack-creator-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   substack-creator-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `substack-creator-pp-cli <command> --help`.
