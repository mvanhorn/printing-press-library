---
name: pp-scrape-creators
description: "Scrape Creators CLI — scrape public social-media data across TikTok, Instagram, YouTube, Twitter/X, LinkedIn, Facebook, Reddit, Threads, Bluesky, Pinterest, Snapchat, Twitch, Kick, Truth Social, and 15+ link-in-bio platforms. Use when the user wants to fetch a profile, posts, comments, videos, ads, or transcripts from a social platform; compare creators; find spike/viral videos; track follower growth; analyze posting cadence; snapshot hashtag trends; inspect API credit budget; or sync a platform's data locally for offline search and analytics."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["scrape-creators-pp-cli"],"env":["SCRAPE_CREATORS_API_KEY_AUTH"]},"primaryEnv":"SCRAPE_CREATORS_API_KEY_AUTH","install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/cmd/scrape-creators-pp-cli@latest","bins":["scrape-creators-pp-cli"],"label":"Install via go install"}]}}'
---

# Scrape Creators - Printing Press CLI

Scrape public social-media data from the terminal — profiles, posts, comments, videos, ads, and transcripts across 30+ platforms. Read-only wrapper around the [Scrape Creators](https://scrapecreators.com) API. Ships a local SQLite sync and analytics commands (`spikes`, `compare`, `cadence`, `track`, `analyze`, `transcripts`, `search trends`, `account budget`) that compound over time.

## When to Use This CLI

Reach for this when the user wants to:

- Look up a public profile or post on any supported platform (`<platform> profile`, `<platform> post`, `<platform> user-posts`)
- Fetch comments, transcripts, or ad details (`<platform> post-comments`, `<platform> video-transcript`, `facebook adlibrary-search-companies`)
- Find a creator's best-performing videos by engagement rate (`tiktok spikes`, `tiktok analyze`)
- Compare creators side-by-side or track one over time (`tiktok compare`, `tiktok track`)
- Understand *when* a creator posts (`tiktok cadence`)
- Search across all of a creator's video transcripts (`tiktok transcripts`)
- Snapshot a hashtag's momentum (`search trends`)
- Audit API credit spend and forecast days remaining (`account budget`)
- Pull a platform's data into local SQLite for offline work (`sync`, `workflow archive`)
- Full-text-search synced data (`search`)

Skip this CLI when the user wants to *post* content (it's read-only) or authenticate as a specific social-media user (it only sees public data).

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `scrape-creators-pp-cli --help`
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → CLI installation
3. **Anything else** → Direct Use (map to the best command and run it)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.22+).
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/cmd/scrape-creators-pp-cli@latest
   ```

   If `@latest` installs a stale build (Go module proxy cache lag):
   ```bash
   GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
     go install github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/cmd/scrape-creators-pp-cli@main
   ```
3. Verify: `scrape-creators-pp-cli --version`.
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.
5. Auth setup:
   ```bash
   export SCRAPE_CREATORS_API_KEY_AUTH="<your-key>"
   ```
   Get a key at <https://app.scrapecreators.com>.
6. Verify: `scrape-creators-pp-cli doctor` reports config, auth, and connectivity status.

## MCP Server Installation

The CLI ships an MCP server (`scrape-creators-pp-mcp`) and a one-liner installer that wires it into any supported agent's config:

```bash
# Preferred — lets the CLI write the correct config format for each agent
scrape-creators-pp-cli agent add claude-code
scrape-creators-pp-cli agent add claude-desktop
scrape-creators-pp-cli agent add cursor
scrape-creators-pp-cli agent add codex

# Use --hosted to point at https://api.scrapecreators.com/mcp instead of the local binary
scrape-creators-pp-cli agent add claude-code --hosted

# Use --force to overwrite an existing scrape-creators entry (diff is printed by default)
scrape-creators-pp-cli agent add cursor --force
```

All writes land at mode `0600`. Alternatively, install the MCP binary manually:

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/cmd/scrape-creators-pp-mcp@latest
claude mcp add scrape-creators scrape-creators-pp-mcp -e SCRAPE_CREATORS_API_KEY_AUTH=<your-key>
```

## Direct Use

1. Check installed: `which scrape-creators-pp-cli`. If missing, offer CLI installation.
2. Ensure `SCRAPE_CREATORS_API_KEY_AUTH` is exported (or saved to the config file) before any call that hits the API.
3. Discover commands: `scrape-creators-pp-cli --help`; drill in with `scrape-creators-pp-cli <platform> --help`.
4. Execute with `--agent` for structured output:
   ```bash
   scrape-creators-pp-cli <platform> <action> [args] --agent
   ```
5. For repeat analytics, run `scrape-creators-pp-cli sync --resources <type>` first so later commands read from local SQLite.

Handle / hashtag normalization: the CLI strips leading `@` from handles and `#` from hashtags automatically. Both `@charlidamelio` and `charlidamelio` work.

## Notable Commands

### Platform scrapers

| Platform | Top-level command | Example leaf actions |
|----------|-------------------|----------------------|
| TikTok | `tiktok` | `profile`, `video`, `video-comments`, `video-transcript`, `search-keyword`, `trending-feed` |
| Instagram | `instagram` | `profile`, `post`, `post-comments`, `user-posts`, `media-transcript` |
| YouTube | `youtube` | `channel`, `playlist`, `video`, `video-transcript`, `search` |
| Twitter / X | `twitter` | `profile`, `user-tweets`, `tweet`, `community` |
| LinkedIn | `linkedin` | `profile`, `company`, `company-posts`, `post`, `ad` |
| Facebook | `facebook` | `profile`, `post`, `post-comments`, `group-posts`, `adlibrary-search-companies` |
| Reddit | `reddit` | `subreddit-details`, `subreddit-search`, `search`, `post-comments` |
| Threads | `threads` | `profile`, `post`, `search` |
| Pinterest | `pinterest` | `pin`, `search`, `user-boards` |
| Bluesky | `bluesky` | `post`, `user-posts` |
| Truth Social | `truthsocial` | `profile`, `user-posts` |
| Twitch | `twitch` | `profile` (clip via bare `twitch --url`) |
| Kick | `kick` | clip via bare `kick --url` |
| Snapchat | `snapchat` | `snapchat --handle <name>` |
| Google | `google` | `search`, `ad`, `company-ads` |
| Link-in-bio | `linkbio`, `linktree`, `linkme`, `komi`, `pillar`, `amazon` | `<platform> --url <page-url>` |
| Age/gender detection | `detect-age-gender` | `--url <image-url>` |

### Analytics commands (TikTok, built on top of the scraped data)

| Command | What it does |
|---------|--------------|
| `tiktok spikes` | Videos that outperformed the creator's own engagement-rate average (threshold multiplier via `--threshold`) |
| `tiktok analyze` | Rank a creator's videos by engagement rate (likes+comments+shares / views) |
| `tiktok compare` | Compare multiple creators — repeat `--handle` once per creator |
| `tiktok cadence` | Posting frequency by day of week and hour of day |
| `tiktok track` | Daily follower snapshot; pass `--history` to chart growth |
| `tiktok transcripts` | Fetch all transcripts for a creator; `--search <term>` to grep across them |

### Cross-platform and data-layer commands

| Command | What it does |
|---------|--------------|
| `search <query>` | FTS5 full-text search over synced data (falls back to API when available) |
| `search trends` | Snapshot a hashtag's result count + top videos (`--hashtag <tag>`) |
| `account budget` | Credit balance + projected days remaining at current burn rate |
| `account api-usage` / `daily-usage` / `most-used-routes` | Usage history views |
| `sync` | Pull API data into local SQLite (`--resources <list>`, `--since <dur>`, `--full`) |
| `tail` | Stream live changes by polling the API (NDJSON to stdout) |
| `analytics` | Count / group-by / top-N over synced data |
| `export` | Export synced data to JSONL or JSON (`--format`, `--output`) |
| `import` | Push a JSONL file into the API via upsert |
| `workflow archive` | One-shot sync of every supported resource; `--full` forces re-archive |
| `workflow status` | Local archive sync state |
| `api` | Browse every raw API endpoint by interface name (power-user escape hatch) |
| `agent add` | Wire the MCP server into `claude-code`, `claude-desktop`, `cursor`, or `codex` |
| `doctor` | Config / auth / connectivity health check |

Run any command with `--help` for full flag documentation.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr; paginated commands emit NDJSON progress to stderr
- **Filterable** — `--select` keeps a subset of fields, dotted paths descend into nested responses
- **Previewable** — `--dry-run` shows the request without sending
- **Cacheable** — GET responses cached for 5 minutes, bypass with `--no-cache`
- **Rate-limitable** — `--rate-limit <rps>` caps requests per second (helpful when walking a large follower list)
- **Data-source switch** — `--data-source live|local|auto` chooses API, local SQLite, or auto fallback
- **Non-interactive** — never prompts; `--no-input` disables every prompt and `--agent` implies it

### Filtering output

`--select` accepts dotted paths; arrays traverse element-wise:

```bash
scrape-creators-pp-cli tiktok profile --handle charlidamelio --agent --select user.nickname,stats.followerCount
scrape-creators-pp-cli tiktok videos-popular --agent --select items.id,items.stats.playCount
```

Use this to narrow huge payloads to the fields you actually need.

### Response envelope

Data-layer commands wrap output in `{"meta": {...}, "results": <data>}`. Parse `.results` for data and `.meta.source` to know whether it's `live` or local. The `N results (live)` summary is printed to stderr only when stdout is a TTY; piped/agent consumers see pure JSON on stdout.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found (profile, post, video, …) |
| 4 | Authentication required (`SCRAPE_CREATORS_API_KEY_AUTH` missing or invalid) |
| 5 | API error (Scrape Creators upstream) |
| 7 | Rate limited |
| 10 | Config error |
