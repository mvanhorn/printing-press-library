---
name: pp-scrape-creators
description: "Every Scrape Creators endpoint, plus offline search, cross-platform compounding, and a local store no other Scrape Creators tool ships with. Trigger phrases: `scrape creators`, `tiktok profile`, `instagram profile`, `youtube channel`, `facebook ad library`, `creator on every platform`, `social media transcript search`, `use scrape-creators`, `run scrape-creators`."
version: "3.2.1"
author: "Adrian Horning"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - scrape-creators-pp-cli
    install:
      - kind: go
        bins: [scrape-creators-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/cmd/scrape-creators-pp-cli
---

# Scrape Creators — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `scrape-creators-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install scrape-creators --cli-only
   ```
2. Verify: `scrape-creators-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/cmd/scrape-creators-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use this CLI when an agent needs public social-media data across multiple platforms in one invocation, when you need offline search over previously fetched content, or when you want cross-platform compound queries (presence, ads, trends) that no single API call answers. It's read-only — pair it with a posting tool if you need write capability.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Cross-platform compounding
- **`creator find`** — Given a single handle, see which of 11+ platforms the creator is on, with follower counts side-by-side.

  _When an agent needs a creator's full footprint before writing outreach or analysis, this returns it in one call instead of 11._

  ```bash
  scrape-creators-pp-cli creator find mrbeast --json
  ```
- **`trends triangulate`** — Snapshot a hashtag or keyword on TikTok, YouTube, Reddit, and Threads in one call to see which platform it's rising fastest on.

  _Marketers asking 'is this trend cresting on TikTok' can see the answer plus the leading-indicator platform in one command._

  ```bash
  scrape-creators-pp-cli trends triangulate "AI agents" --json
  ```
- **`ads search`** — Search Facebook, Google, and LinkedIn ad libraries in one query — see every ad a brand is currently running.

  _Competitive intel without juggling three vendors and three APIs._

  ```bash
  scrape-creators-pp-cli ads search "Liquid Death" --json
  ```
- **`ads monitor`** — Snapshot a brand's ads on Facebook, Google, and LinkedIn into SQLite; on rerun, diff new ads vs ones that disappeared.

  _Cron-friendly competitive monitoring without glue code._

  ```bash
  scrape-creators-pp-cli ads monitor "Liquid Death" --json
  ```
- **`bio resolve`** — Paste any linktree.ee, komi.io, pillar.io, linkbio, or linkme URL and get the unified destination list.

  _Lead-research tasks that need a creator's full link footprint stop caring which bio service they used._

  ```bash
  scrape-creators-pp-cli bio resolve https://linktr.ee/mrbeast --json
  ```

### Local state that compounds
- **`transcripts search`** — FTS5-indexed search across all transcripts you've synced — TikTok, YouTube, Instagram, Facebook, and Twitter videos.

  _Agents doing brand-mention or keyword-monitoring across video content can grep their entire synced corpus offline._

  ```bash
  scrape-creators-pp-cli transcripts search "affiliate link" --json --select creator,platform,snippet
  ```
- **`content spikes`** — Find videos that performed significantly above a creator's average — the ones that actually went viral.

  _When asked 'which videos took off,' the agent can answer with statistical confidence instead of guessing._

  ```bash
  scrape-creators-pp-cli content spikes mrbeast --threshold 2.0 --platform youtube --json
  ```
- **`creator compare`** — Compare two or more creators side-by-side on follower count, engagement rate, posting cadence, and content volume.

  _Influencer-shortlist work that used to take a spreadsheet now takes one command._

  ```bash
  scrape-creators-pp-cli creator compare mrbeast pewdiepie --platform youtube --json
  ```
- **`creator track`** — Snapshot a creator's follower count daily across every platform; chart their growth trajectory over time.

  _Trend lines for momentum decisions (sponsorship, partnership timing)._

  ```bash
  scrape-creators-pp-cli creator track mrbeast --json
  ```
- **`content cadence`** — See when a creator posts — by day of week and hour — across every platform you've synced for them.

  _Benchmark a competitor's publishing strategy or recommend the right slot for a creator partner._

  ```bash
  scrape-creators-pp-cli content cadence mrbeast --platform tiktok --json
  ```
- **`content analyze`** — Rank a creator's synced content by engagement rate (not raw likes) so you see their true best performers.

  _Surfaces over-performers that raw view counts hide._

  ```bash
  scrape-creators-pp-cli content analyze mrbeast --platform youtube --json
  ```
- **`trends delta`** — Track whether a hashtag is growing or shrinking by comparing video counts across snapshot intervals.

  _Distinguish a stable hashtag from a rising or dying one in seconds._

  ```bash
  scrape-creators-pp-cli trends delta "booktok" --days 7 --platform tiktok --json
  ```

### Operator ergonomics
- **`account budget`** — See how fast you're spending API credits and how many days remain at current pace, fused with the API's own usage history.

  _Catches runaway sync jobs before they exhaust the plan._

  ```bash
  scrape-creators-pp-cli account budget --json
  ```

## Command Reference

**account** — Manage account

- `scrape-creators-pp-cli account list` — Returns the number of API credits remaining on your Scrape Creators account. The response contains a single...
- `scrape-creators-pp-cli account list-getapiusage` — Returns a paginated list of your API requests, including the endpoint called, status code, credits used, and...
- `scrape-creators-pp-cli account list-getdailyusagecount` — Returns aggregated daily usage statistics for the last 30 days, including total credits consumed and number of...
- `scrape-creators-pp-cli account list-getmostusedroutes` — Returns your top 20 most called API endpoints ranked by call count, along with total credits consumed per endpoint....

**amazon** — Manage amazon

- `scrape-creators-pp-cli amazon` — Scrapes a creator's Amazon Shop page by URL, returning their storefront profile and product collections. Returns...

**bluesky** — Get Bluesky posts and profile info

- `scrape-creators-pp-cli bluesky list` — Fetches a single Bluesky post by URL, returning the post's record text, author info, embed content, replyCount,...
- `scrape-creators-pp-cli bluesky list-profile` — Retrieves a Bluesky user's public profile including handle, displayName, avatar, description, followersCount,...
- `scrape-creators-pp-cli bluesky list-user` — Fetches a paginated feed of posts from a Bluesky user, returning each post's uri, record text, author info, embed...

**detect-age-gender** — Manage detect age gender

- `scrape-creators-pp-cli detect-age-gender` — Uses AI to analyze a creator's profile photo and estimate their age and gender. Returns ageRange with low and high...

**facebook** — Get public Facebook profiles and posts

- `scrape-creators-pp-cli facebook list` — Retrieves a single public Facebook post or reel by URL. Returns post_id, like_count, comment_count, share_count,...
- `scrape-creators-pp-cli facebook list-adlibrary` — Retrieves detailed information about a specific Facebook ad by its ID or URL. Returns adArchiveID, pageName,...
- `scrape-creators-pp-cli facebook list-adlibrary-2` — Fetches all ads currently running for a specific company from the Meta Ad Library. Each ad includes ad_archive_id,...
- `scrape-creators-pp-cli facebook list-adlibrary-3` — Searches the Meta Ad Library by keyword and returns matching ads. Each result includes ad_archive_id, page_name,...
- `scrape-creators-pp-cli facebook list-adlibrary-4` — Searches for companies by name in the Meta Ad Library and returns their page IDs for use with other ad library...
- `scrape-creators-pp-cli facebook list-group` — Fetches posts from a public Facebook group, limited to 3 posts per page due to API limitations. Each post includes...
- `scrape-creators-pp-cli facebook list-post` — Fetches comments from a Facebook post or reel with cursor-based pagination. Each comment includes id, text,...
- `scrape-creators-pp-cli facebook list-post-2` — Extracts the transcript text from a Facebook video post or reel. Returns the transcript as a single text string with...
- `scrape-creators-pp-cli facebook list-profile` — Retrieves public Facebook page details including category, address, email, phone, website, services, priceRange,...
- `scrape-creators-pp-cli facebook list-profile-2` — Fetches photos from a public Facebook page with pagination support. Each photo includes photo_id,...
- `scrape-creators-pp-cli facebook list-profile-3` — Returns publicly visible Facebook profile posts, limited to 3 posts per page due to API limitations. Each post...
- `scrape-creators-pp-cli facebook list-profile-4` — Fetches up to 10 reels per request from a public Facebook page. Each reel includes id, url, view_count, description,...

**google** — Scrape Google search results

- `scrape-creators-pp-cli google list` — Retrieves detailed information about a specific Google ad including advertiserId, creativeId, format, firstShown,...
- `scrape-creators-pp-cli google list-adlibrary` — Searches the Google Ad Transparency Library for advertisers by name. Returns a list of matching advertisers with...
- `scrape-creators-pp-cli google list-company` — Fetches public ads for a company from the Google Ad Transparency Library by domain or advertiser_id. Each ad...
- `scrape-creators-pp-cli google list-search` — Performs a Google search and returns organic results with url, title, and description for each result. Supports an...

**instagram** — Gets Instagram profiles, posts, and reels

- `scrape-creators-pp-cli instagram list` — Fetches a lightweight Instagram profile summary by user ID, returning username, full name, biography, profile...
- `scrape-creators-pp-cli instagram list-media` — Generates an AI-powered speech-to-text transcription for an Instagram video post or reel. The video must be under 2...
- `scrape-creators-pp-cli instagram list-post` — Fetches detailed metadata for a single Instagram post or reel by shortcode or URL. Returns caption text, like count,...
- `scrape-creators-pp-cli instagram list-post-2` — Retrieves comments on a public Instagram post or reel. Each comment includes the comment text, creation timestamp,...
- `scrape-creators-pp-cli instagram list-profile` — Retrieves comprehensive public Instagram profile information including biography, bio links, follower and following...
- `scrape-creators-pp-cli instagram list-reels` — Searches for Instagram reels matching a keyword or phrase via Google Search, bypassing Instagram's login-gated...
- `scrape-creators-pp-cli instagram list-song` — DEPRECATED — this endpoint is no longer functional. Instagram removed the public audio pages that this endpoint...
- `scrape-creators-pp-cli instagram list-user` — Returns the raw HTML embed snippet for an Instagram user's profile widget. The response contains a single html...
- `scrape-creators-pp-cli instagram list-user-2` — Lists all story highlight albums for an Instagram user. Each highlight includes its ID, title, cover thumbnail URL,...
- `scrape-creators-pp-cli instagram list-user-3` — Returns a paginated list of a user's public Instagram reels (short-form videos). Each reel includes its shortcode,...
- `scrape-creators-pp-cli instagram list-user-4` — Returns a paginated feed of a user's public Instagram posts, including photos, videos, and carousels. Each item...
- `scrape-creators-pp-cli instagram list-user-5` — Fetches the full contents of a specific Instagram story highlight album by its ID. Returns the highlight's cover...

**kick** — Scrape Kick clips

- `scrape-creators-pp-cli kick` — Fetches detailed data for a Kick clip by URL, including video, metadata, and channel info. Returns clip id, title,...

**komi** — Scrape Komi pages

- `scrape-creators-pp-cli komi` — Scrapes a Komi page by URL, extracting the creator's profile, social links, and featured content. Returns id,...

**linkbio** — Scrape Linkbio (lnk.bio) pages

- `scrape-creators-pp-cli linkbio` — Scrapes a Linkbio (lnk.bio) page by URL, extracting the creator's profile and all their links. Returns handle, id,...

**linkedin** — Scrape LinkedIn

- `scrape-creators-pp-cli linkedin list` — Retrieves detailed information about a specific LinkedIn ad by URL. Returns id, description, headline, adType,...
- `scrape-creators-pp-cli linkedin list-ads` — Searches the LinkedIn Ad Library by company name, keyword, or companyId with optional country and date filters. Each...
- `scrape-creators-pp-cli linkedin list-company` — Fetches a LinkedIn company page with details including name, description, logo, cover image, slogan, location,...
- `scrape-creators-pp-cli linkedin list-company-2` — Retrieves paginated posts from a LinkedIn company page, including each post's URL, ID, publication date, and full...
- `scrape-creators-pp-cli linkedin list-post` — Fetches a single LinkedIn post or article, returning the title, headline, full description text, author info with...
- `scrape-creators-pp-cli linkedin list-profile` — Retrieves a person's public LinkedIn profile data, including their name, photo, location, follower count...

**linkme** — Get Linkme profile info

- `scrape-creators-pp-cli linkme` — Retrieves a Linkme profile by URL, including identity, social links, and contact details. Returns profile with id,...

**linktree** — Scrape Linktree pages

- `scrape-creators-pp-cli linktree` — Scrapes a Linktree page by URL, extracting the creator's profile and all their links. Returns id, username,...

**pillar** — Scrape Pillar pages

- `scrape-creators-pp-cli pillar` — Scrapes a Pillar page by URL, extracting the creator's profile, social links, and products. Returns id, first_name,...

**pinterest** — Scrape Pinterest pins

- `scrape-creators-pp-cli pinterest list` — Fetches a paginated list of pins from a Pinterest board by URL, returning each pin's id, description, title, images,...
- `scrape-creators-pp-cli pinterest list-pin` — Fetches detailed information about a single Pinterest pin by URL, returning title, description, link, dominantColor,...
- `scrape-creators-pp-cli pinterest list-search` — Searches Pinterest for pins matching a query, returning results with id, url, title, description, images, link,...
- `scrape-creators-pp-cli pinterest list-user` — Fetches a paginated list of boards for a Pinterest user, returning each board's name, url, description, pin_count,...

**reddit** — Scrape Reddit posts and comments

- `scrape-creators-pp-cli reddit list` — Retrieves detailed information about a specific Reddit ad by its id. Returns an analysis_summary with headline and...
- `scrape-creators-pp-cli reddit list-ads` — Searches the Reddit Ad Library for ads matching a query, returning a maximum of 30 results. Each ad includes id,...
- `scrape-creators-pp-cli reddit list-post` — Retrieves comments and post details from a Reddit post by URL. Returns the post with title, author, score, ups,...
- `scrape-creators-pp-cli reddit list-search` — Searches across all of Reddit for posts matching a query. Each post includes title, author, selftext, subreddit,...
- `scrape-creators-pp-cli reddit list-subreddit` — Fetches posts from a subreddit with sorting and filtering options. Each post includes title, author, selftext,...
- `scrape-creators-pp-cli reddit list-subreddit-2` — Retrieves metadata about a subreddit by name or URL. The subreddit name must be case-sensitive. Returns...
- `scrape-creators-pp-cli reddit list-subreddit-3` — Searches within a specific subreddit for posts, comments, and media matching a query. Returns posts with title,...

**snapchat** — Scrape Snapchat user profiles and thier stories

- `scrape-creators-pp-cli snapchat` — Retrieves a Snapchat user's public profile by handle, including identity, stories, and spotlight content. Returns...

**threads** — Get Threads posts

- `scrape-creators-pp-cli threads list` — Fetches a single Threads post by URL, returning the post's caption, like_count, view_counts, reshare_count,...
- `scrape-creators-pp-cli threads list-profile` — Retrieves a Threads user's public profile including username, full_name, biography, profile_pic_url, follower_count,...
- `scrape-creators-pp-cli threads list-search` — Searches Threads for posts matching a keyword, returning up to 10 results with caption text, like_count,...
- `scrape-creators-pp-cli threads list-search-2` — Searches for Threads users by username, returning matching profiles with username, full_name, profile_pic_url,...
- `scrape-creators-pp-cli threads list-user` — Fetches the most recent posts from a Threads user, returning id, caption text, code, like_count, reshare_count,...

**tiktok** — Scrape TikTok profiles, videos, and more

- `scrape-creators-pp-cli tiktok list` — Fetches TikTok's trending/For You feed for a given region — useful for discovering viral content and what's...
- `scrape-creators-pp-cli tiktok list-creators` — Discovers trending and popular TikTok creators, filterable by follower count range, creator country, and audience...
- `scrape-creators-pp-cli tiktok list-hashtags` — Discovers trending and popular TikTok hashtags, filterable by time period (7/30/120 days) and country. Returns a...
- `scrape-creators-pp-cli tiktok list-product` — Fetches full details for a specific TikTok Shop product by its URL, including stock levels and affiliate videos....
- `scrape-creators-pp-cli tiktok list-profile` — Fetches public profile data for a TikTok user by their handle — useful for looking up a creator's identity, bio,...
- `scrape-creators-pp-cli tiktok list-profile-2` — Fetches videos posted by a TikTok user, sortable by latest or most popular — use this to get a creator's video...
- `scrape-creators-pp-cli tiktok list-search` — Searches for TikTok videos under a specific hashtag — useful for finding content by topic or trend. Returns...
- `scrape-creators-pp-cli tiktok list-search-2` — Searches for TikTok videos by keyword or phrase — the general video search across all of TikTok. Returns...
- `scrape-creators-pp-cli tiktok list-search-3` — Searches TikTok's 'Top' results by query — returns both videos and photo carousels, unlike keyword search which...
- `scrape-creators-pp-cli tiktok list-search-4` — Searches for TikTok users by keyword or name — useful for finding creators or accounts matching a query. Returns...
- `scrape-creators-pp-cli tiktok list-shop` — Lists all products from a specific TikTok Shop store by its URL. Returns an array of product objects each with...
- `scrape-creators-pp-cli tiktok list-shop-2` — Searches TikTok Shop for products matching a keyword query. Returns an array of product objects each with `title`,...
- `scrape-creators-pp-cli tiktok list-shop-3` — Fetches customer reviews for a TikTok Shop product by URL or product_id. Returns `product_reviews`, an array of...
- `scrape-creators-pp-cli tiktok list-song` — Fetches detailed metadata for a specific TikTok sound or song by its clipId. Returns `music_info` with `title`,...
- `scrape-creators-pp-cli tiktok list-song-2` — Fetches TikTok videos that use a specific sound or song, identified by its clipId. Returns `aweme_list`, an array of...
- `scrape-creators-pp-cli tiktok list-user` — Retrieves audience demographic data for a TikTok user, showing where their followers are located by country. Returns...
- `scrape-creators-pp-cli tiktok list-user-2` — Retrieves the follower list of a TikTok account by handle or user_id — useful for seeing who follows a creator or...
- `scrape-creators-pp-cli tiktok list-user-3` — Retrieves the following list — accounts that a TikTok user follows — by their handle. Returns `followings`, an...
- `scrape-creators-pp-cli tiktok list-user-4` — Checks if a TikTok user is currently live streaming and retrieves their live room details. Returns...
- `scrape-creators-pp-cli tiktok list-user-5` — Fetches products featured in a TikTok user's public showcase — the products a creator promotes on their profile....
- `scrape-creators-pp-cli tiktok list-video` — Fetches detailed data for a single TikTok video by URL, including its metadata, engagement stats, and optionally its...
- `scrape-creators-pp-cli tiktok list-video-2` — Fetches comments on a TikTok video by URL — useful for reading audience reactions, replies, and engagement....
- `scrape-creators-pp-cli tiktok list-video-3` — Extracts the transcript, captions, or subtitles from a TikTok video by URL. Returns `id`, `url`, and `transcript` as...
- `scrape-creators-pp-cli tiktok list-video-4` — Fetches replies to a specific TikTok comment by its ID. Returns `comments`, an array of comment objects each with...

**truthsocial** — Manage truthsocial

- `scrape-creators-pp-cli truthsocial list` — Fetches a single Truth Social post by URL, returning text, id, created_at, url, content, account details,...
- `scrape-creators-pp-cli truthsocial list-profile` — Retrieves a Truth Social user's public profile including display_name, username, avatar, header, followers_count,...
- `scrape-creators-pp-cli truthsocial list-user` — Fetches a paginated list of posts from a Truth Social user, returning text, id, created_at, url, content, account...

**twitch** — Scrape Twitch clips

- `scrape-creators-pp-cli twitch list` — Fetches detailed data for a Twitch clip by URL, including metadata and direct video URLs. Returns clip id, slug,...
- `scrape-creators-pp-cli twitch list-profile` — Retrieves a Twitch user's public profile by handle, including identity, social links, and content. Returns id,...
- `scrape-creators-pp-cli twitch list-user` — Fetches a list of videos (100 max) for a Twitch user, returning each video's id, slug, url, embedURL, title,...

**twitter** — Get Twitter profiles, tweets, followers and more

- `scrape-creators-pp-cli twitter list` — Retrieves details about a Twitter/X Community by URL. Returns the community name, description, rest_id, join_policy,...
- `scrape-creators-pp-cli twitter list-community` — Fetches tweets posted within a Twitter/X Community by URL. Returns an array of tweets, each with id, full_text,...
- `scrape-creators-pp-cli twitter list-profile` — Retrieves a Twitter user's profile by handle, including account metadata and statistics. Returns name, screen_name,...
- `scrape-creators-pp-cli twitter list-tweet` — Retrieves detailed information about a specific tweet by URL, including the author's profile and engagement metrics....
- `scrape-creators-pp-cli twitter list-tweet-2` — Extracts the transcript from a Twitter video tweet using AI-powered transcription. The video must be under 2 minutes...
- `scrape-creators-pp-cli twitter list-usertweets` — Fetches tweets from a Twitter user's profile by handle. Note: Twitter publicly returns only ~100 of the user's most...

**youtube** — Scrape YouTube channels, videos, and more

- `scrape-creators-pp-cli youtube list` — Retrieves comprehensive YouTube channel profile data including name, avatar images, subscriber count (subscribers),...
- `scrape-creators-pp-cli youtube list-channel` — Retrieves a paginated list of short-form videos (Shorts) from a YouTube channel, including each short's title, URL,...
- `scrape-creators-pp-cli youtube list-channelvideos` — Fetches a paginated list of videos uploaded by a YouTube channel, including each video's title, URL, thumbnail, view...
- `scrape-creators-pp-cli youtube list-communitypost` — Retrieves the full details of a YouTube community post, including its text content, attached images, like count,...
- `scrape-creators-pp-cli youtube list-playlist` — Retrieves all videos in a YouTube playlist, including the playlist title, owner info, total video count, and each...
- `scrape-creators-pp-cli youtube list-search` — Searches YouTube by keyword query and returns matching videos, channels, playlists, shorts, shelves, and live...
- `scrape-creators-pp-cli youtube list-search-2` — Searches YouTube for content matching a specific hashtag and returns matching videos with title, URL, thumbnail,...
- `scrape-creators-pp-cli youtube list-shorts` — Fetches approximately 48 currently trending YouTube Shorts (viral/popular short-form videos) per call, returning...
- `scrape-creators-pp-cli youtube list-video` — Fetches full details for a YouTube video or short, including title, description, thumbnail, view count (views), like...
- `scrape-creators-pp-cli youtube list-video-2` — Fetches comments and replies from a YouTube video, including each comment's text content, author details, like...
- `scrape-creators-pp-cli youtube list-video-3` — Retrieves the captions, subtitles, or transcript of a YouTube video or short. Returns both a timestamped transcript...
- `scrape-creators-pp-cli youtube list-video-4` — Fetches replies to a specific comment on a YouTube video, including each reply's text content, author details (name,...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
scrape-creators-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Map a creator's full footprint

```bash
scrape-creators-pp-cli creator find mrbeast --json --select platform,handle,follower_count
```

Probes every platform's profile endpoint and returns a presence matrix with follower counts. Useful before writing a brief or outreach email.

### Find a creator's viral hits

```bash
scrape-creators-pp-cli sync --resources youtube && scrape-creators-pp-cli content spikes mrbeast --threshold 2.0 --platform youtube --json
```

Sync videos to local SQLite, then return videos whose engagement is more than 2× the creator's average.

### Track a brand's ad campaigns across networks

```bash
scrape-creators-pp-cli ads monitor "Liquid Death" --json
```

Snapshots Facebook + Google + LinkedIn ads into SQLite; rerun on a cron and diff new vs disappeared.

### Triangulate where a trend is rising

```bash
scrape-creators-pp-cli trends triangulate "AI agents" --json --select platform,result_count,delta_pct
```

Probes TikTok, YouTube, Reddit, and Threads for a topic and returns per-platform velocity. Reads the leading platform when run repeatedly.

### Grep your transcript corpus

```bash
scrape-creators-pp-cli transcripts search "affiliate link" --json --select creator,platform,video_url,snippet
```

FTS5 over every transcript you've synced, returning the creator/platform/URL/match — useful for brand-safety audits.

## Auth Setup

Set SCRAPE_CREATORS_API_KEY (get one at https://scrapecreators.com). One header, no OAuth handshake.

Run `scrape-creators-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  scrape-creators-pp-cli account list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

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
scrape-creators-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
scrape-creators-pp-cli feedback --stdin < notes.txt
scrape-creators-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.scrape-creators-pp-cli/feedback.jsonl`. They are never POSTed unless `SCRAPE_CREATORS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SCRAPE_CREATORS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
scrape-creators-pp-cli profile save briefing --json
scrape-creators-pp-cli --profile briefing account list
scrape-creators-pp-cli profile list --json
scrape-creators-pp-cli profile show briefing
scrape-creators-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `scrape-creators-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)
## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/cmd/scrape-creators-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add scrape-creators-pp-mcp -- scrape-creators-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which scrape-creators-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   scrape-creators-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `scrape-creators-pp-cli <command> --help`.
