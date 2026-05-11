---
name: pp-instagram
description: "Single-binary Instagram archiver with a local SQLite catalog, full-text search across captions and... Trigger phrases: `archive an instagram profile`, `download instagram posts`, `extract images from instagram`, `search my instagram archive`, `diff instagram highlights`, `use instagram-pp-cli`, `run instagram-pp-cli`."
author: "user"
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

This CLI is a pure-Go alternative to instaloader and gallery-dl for archiving public and authenticated Instagram pages. Every download lands in a local SQLite catalog, so search, watchlists, highlight-tray diff, and remote-vs-local mirror-diff are SQL queries — not shell loops over .txt sidecars. One static binary, no Python, no ffmpeg, no subprocesses.

## When to Use This CLI

Use this CLI when you need to archive public or authenticated Instagram pages and search across the result. It excels at recurring sync jobs over a watchlist of profiles and hashtags, alt-text-aware visual search across the local archive, highlight-tray diffs, and dry-run cost estimation before committing the rate budget. It is not the right tool for posting, following, or interacting — it only reads.

## Anti-Triggers

This CLI is read-only. Do NOT invoke it for any of these requests — it has no write commands and no plans to add them:

- "post to Instagram" / "publish a post" / "upload a photo to Instagram"
- "follow this account" / "unfollow"
- "like this post" / "comment on this post"
- "send a DM" / "reply to a DM"
- "delete my post" / "edit a caption"
- "create a story" / "publish a reel"
- Any request that mutates Instagram-side state.

For posting and account-management, use Instagram's first-party tools or the official Graph API for business/creator accounts.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`search`** — Full-text search across every caption, hashtag, mention, location, owner handle, and IG-generated alt-text in your local archive — no API call.

  _When the archive is the corpus, agents can answer cross-profile questions ("every post that mentions person X across these 40 accounts") without re-fetching from Instagram._

  ```bash
  instagram-pp-cli search "sunset golden hour" --owner natgeo --kind post --json --select results.shortcode,results.owner,results.taken_at,results.caption_snippet
  ```
- **`search`** — Captures Instagram's auto-generated accessibility_caption ("may contain: two people, sunglasses, outdoors") into the local catalog and exposes it via --alt search.

  _Lets an agent pick representative images by visual content from the local archive without OCR, vision models, or extra API calls._

  ```bash
  instagram-pp-cli search --alt "sunglasses outdoors" --json --select results.shortcode,results.alt_text,results.local_path
  ```
- **`sync`** — Register profiles and hashtags as a watchlist, then run one sync that walks the whole list under one shared rate budget instead of looping a shell over instaloader runs.

  _One predictable command handles the recurring weekly sync, so an agent can be tasked with "sync the watchlist" without managing per-target state._

  ```bash
  instagram-pp-cli watch add natgeo nasa rei && instagram-pp-cli sync watchlist --json --select summary.profile,summary.added,summary.skipped
  ```
- **`highlights diff`** — Compares the current highlight tray and items for a profile against the prior snapshot in the local store, reporting added, removed, and renamed items.

  _Brand and rights teams need the previous version of a highlight when something gets pulled — only a local snapshot can answer this._

  ```bash
  instagram-pp-cli highlights diff natgeo --json --select changes.tray,changes.kind,changes.shortcode
  ```
- **`whatsnew`** — Lists every shortcode added to the local catalog since the prior sync run, grouped by owner, with caption snippets.

  _Gives agents and researchers a delta after every sync without re-walking IG._

  ```bash
  instagram-pp-cli whatsnew --since last-run --json --select results.owner,results.shortcode,results.caption_snippet,results.taken_at
  ```
- **`diff`** — Cursor-walks shortcodes from Instagram, set-diffs against the local download_history, and reports both new posts and posts that were deleted upstream since the last sync.

  _Deleted-evidence preservation is the only way to answer "which posts disappeared since last week" — Instagram itself doesn't tell you._

  ```bash
  instagram-pp-cli diff natgeo --json --select added.shortcode,deleted.shortcode,deleted.taken_at
  ```
- **`search`** — Opt-in --rerank flag pipes the top FTS candidates' captions, alt-text, and first comments to whichever LLM env is set (ANTHROPIC_API_KEY, OPENAI_API_KEY, or OLLAMA_HOST) and returns the LLM's relevance ranking.

  _Lets an agent or designer search by theme even when captions and alt-text don't use the same words; no model is bundled and no LLM call happens unless --rerank is passed._

  ```bash
  instagram-pp-cli search "warm indoor light" --rerank --rerank-top 50 --json --select results.shortcode,results.score,results.alt_text,results.caption_snippet
  ```

### Reachability mitigation
- **`user`** — Walks the cursor without downloading, sums estimated bytes from each media_info, and reports request count plus wall-clock under the configured --rate.

  _Agents can preview cost before committing the rate budget; humans can decide whether to run now or split across days._

  ```bash
  instagram-pp-cli user natgeo --dry-run --rate 30 --json --select estimate.posts,estimate.bytes,estimate.requests,estimate.wall_clock_seconds
  ```
- **`doctor`** — Joins sessions, rate_limit_events, and download_history to report last-sync per watchlist entry, current 429 backoff state, session validity, and cursor-stall warnings.

  _Agents can decide whether to attempt a sync now or wait, and humans can spot a session that is one bad request from a checkpoint._

  ```bash
  instagram-pp-cli doctor --json --select sessions.username,sessions.status,backoff.until,stalled.handles
  ```

### Service-specific patterns
- **`get`** — Filter carousel children by media kind so a sync keeps only stills (skipping reels and video children) or only videos.

  _Lets agents scope a download to a media type without inspecting each carousel manually._

  ```bash
  instagram-pp-cli get https://www.instagram.com/p/Cabc123/ --json --select media_items.kind,media_items.local_path
  ```

## Command Reference

**accounts** — Look up Instagram profiles by username or user ID.

- `instagram-pp-cli accounts get` — Get a profile's web info by username (id, full_name, biography, follower_count, profile_pic_url, is_private).
- `instagram-pp-cli accounts user` — Get a user's full mobile-API profile by numeric user ID (richer fields than web info).

**feed** — Fetch authenticated user feeds: saved, liked, timeline, and per-user media.

- `instagram-pp-cli feed liked` — Fetch the authenticated user's liked posts.
- `instagram-pp-cli feed reels` — Fetch active stories (reels media) for a list of user IDs. Stories expire 24 hours after posting.
- `instagram-pp-cli feed saved` — Fetch the authenticated user's saved (bookmarked) posts.
- `instagram-pp-cli feed timeline` — Fetch the authenticated user's home feed.
- `instagram-pp-cli feed user` — Fetch a profile's media feed by numeric user ID. Pagination cursor is max_id.

**hashtag** — Look up hashtag pages and download tagged media.

- `instagram-pp-cli hashtag <tag_name>` — Get a hashtag's web info (post count, top posts, recent posts cursor).

**highlights** — Fetch profile highlight trays and per-highlight items.

- `instagram-pp-cli highlights reel` — Fetch a single highlight reel's items by highlight reel ID (prefix with 'highlight:', e.g., highlight:17855928765404872).
- `instagram-pp-cli highlights tray` — Fetch the highlight tray for a profile (cover images, titles, ordered list of highlight reels).

**location** — Fetch geotagged location pages and the posts at them.

- `instagram-pp-cli location <location_id>` — Fetch posts at a location ID (top, recent, ranked sections).

**media** — Fetch single posts, reels, and IGTV by media ID.

- `instagram-pp-cli media <media_id>` — Get full media JSON for a single post, reel, or IGTV by media ID, including carousel children, captions, alt-text, comments, and CDN URLs.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
instagram-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Search the entire archive for a person across every synced profile

```bash
instagram-pp-cli search "\bAlex Roe\b" --json --select results.owner,results.shortcode,results.taken_at,results.caption_snippet
```

FTS5 query joined back to the posts table; the dotted --select narrows the response so an agent can rank without parsing kilobytes of unused fields.

### Build a moodboard of stills from a watchlist

First sync the watchlist:

```bash
instagram-pp-cli sync watchlist
```

Then alt-text-search the local catalog:

```bash
instagram-pp-cli search --kind post --alt "sunglasses outdoors" --json --select results.shortcode,results.alt_text,results.local_path
```

Sync watchlist runs the whole watchlist, then alt-text search returns only carousel stills matching the visual brief.

### Diff a profile's highlights week-over-week

```bash
instagram-pp-cli highlights download natgeo && instagram-pp-cli highlights diff natgeo --json --select changes.tray,changes.kind,changes.shortcode
```

Sync writes a fresh snapshot; diff compares against the prior one and reports what changed.

### Estimate before downloading a profile

```bash
instagram-pp-cli user natgeo --dry-run --rate 30 --json --select estimate.posts,estimate.bytes,estimate.requests,estimate.wall_clock_seconds
```

Walks the cursor without saving; lets an agent decide whether to run now or split across days.

### Find every post deleted upstream since last sync

```bash
instagram-pp-cli diff natgeo --json --select deleted.shortcode,deleted.taken_at,deleted.caption_snippet
```

Set-diffs the live cursor against the local download_history; preserves evidence even when the upstream post is gone.

## Auth Setup

Instagram's reachable surface is the reverse-engineered web/private API, gated by session cookies. Authenticate by importing cookies from your logged-in Chrome/Firefox/Safari/Arc/Brave/Edge profile (`auth login --chrome`) or by pasting a sessionid value (`auth login --sessionid <value>`). Username+password login is not shipped — it is the most reliable way to trip a checkpoint or get an account banned. After login, the device fingerprint and cookies persist in the local store so the next run reuses them. Run `doctor` to confirm the session is still good before a long sync.

Note: `--chrome` requires a cookie-extractor tool installed locally (pycookiecheat, cookies, or cookie-scoop-cli). If none is available, use `--sessionid <value>` instead.

Run `instagram-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  instagram-pp-cli accounts get mock-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag

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
instagram-pp-cli --profile briefing accounts get mock-value
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
