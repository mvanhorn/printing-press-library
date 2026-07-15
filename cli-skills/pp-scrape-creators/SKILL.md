---
name: pp-scrape-creators
description: "Every Scrape Creators endpoint across 28 platforms, plus a local store with offline transcript search, cross-platform joins, and ad-library diffing no other Scrape Creators tool ships. Trigger phrases: `find which platforms a creator is on`, `compare these creators' engagement`, `monitor a brand's ads`, `search creator transcripts for a keyword`, `track a hashtag across platforms`, `use scrape creators`, `run scrape-creators`."
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
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/developer-tools/scrape-creators/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Scrape Creators — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `scrape-creators-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install scrape-creators --cli-only
   ```
2. Verify: `scrape-creators-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/cmd/scrape-creators-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Scrape Creators exposes 164 read-only endpoints across TikTok, Instagram, YouTube, Facebook, LinkedIn, GitHub, Spotify, and 21 more platforms behind one API key. This CLI mirrors all of them as typed commands and MCP tools, then adds the layer the official CLI lacks: a local SQLite store with FTS5 so paid transcripts, profiles, and ad creatives become a queryable, diffable corpus. Cross-platform commands like 'creator find', 'trends triangulate', and 'ads monitor' answer questions no single endpoint can.

## When to Use This CLI

Use this CLI when an agent or analyst needs on-demand public social-media data across many platforms and wants to accumulate it locally for repeat querying. It is the right choice for competitive ad monitoring, influencer discovery and vetting, transcript-corpus building for RAG, and cross-platform trend and follower tracking — anything where a local, diffable copy of paid data beats stateless per-call fetches.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to post, comment, message, or modify anything on any platform — every endpoint is read-only public-data extraction.
- Do not use it as a free data source — every request costs a Scrape Creators credit; for one-off ad-hoc reads the hosted MCP or a single curl may be cheaper than a full sync.
- Do not use it to access private or login-gated content; it only returns public data the API exposes.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Cross-platform compounding

- **`creator find`** — Given one handle, see which of 12 creator platforms the creator is on with follower counts side-by-side.

  _Reach for this before writing creator outreach — one call replaces 11+ manual per-platform lookups._

  ```bash
  scrape-creators-pp-cli creator find mrbeast --agent
  ```
- **`creator compare`** — Compare two or more creators side-by-side on follower count, engagement rate, and content volume.

  _Use it to separate real reach from vanity follower counts when vetting a shortlist._

  ```bash
  scrape-creators-pp-cli creator compare mkbhd mrwhosetheboss --agent --select handle,engagement_rate,follower_count
  ```
- **`trends triangulate`** — Snapshot a hashtag or topic across platforms in one call to see which platform it is biggest on.

  _Use it to catch a trend's leading platform before it crests, for content-timing calls._

  ```bash
  scrape-creators-pp-cli trends triangulate "stanley cup" --agent
  ```

### Local engagement analytics

- **`content spikes`** — Surface the videos that performed far above a creator's own baseline — the ones that actually went viral.

  _Pick this over a raw post list when you want a creator's outlier hits, not their average output._

  ```bash
  scrape-creators-pp-cli content spikes mrbeast --agent
  ```

### Local store that compounds

- **`transcripts search`** — FTS5 full-text search across every transcript you've synced — YouTube, TikTok, Instagram, Facebook, LinkedIn, and Rumble.

  _Reach for this for brand-safety and topic sweeps over a corpus you already paid to fetch — no credits re-spent._

  ```bash
  scrape-creators-pp-cli transcripts search "affiliate link" --agent --select creator,platform,snippet
  ```
- **`creator track`** — Append a follower snapshot per run on a chosen platform, then read the growth trajectory over time.

  _Run it on a schedule to chart partner-creator growth; meaningful once multiple snapshots accumulate._

  ```bash
  scrape-creators-pp-cli creator track mrbeast --agent
  ```
- **`ads monitor`** — Snapshot a brand's live ads across Facebook, TikTok, Google, and LinkedIn ad libraries; on rerun, diff new ads vs. ones that disappeared.

  _Use it for recurring competitive ad tracking — the first run is a unified search, every rerun is a what-changed diff._

  ```bash
  scrape-creators-pp-cli ads monitor nike --agent
  ```

### Agent-native plumbing

- **`account budget`** — See how fast you're spending API credits and how many days remain at the current pace, computed from the API's credit balance and daily usage history.

  _Credits are pay-as-you-go and depletion returns HTTP 402 mid-workflow — check runway before a big sync._

  ```bash
  scrape-creators-pp-cli account budget --agent
  ```

## Command Reference

**account** — Manage account

- `scrape-creators-pp-cli account list` — Returns the number of API credits remaining on your Scrape Creators account.
- `scrape-creators-pp-cli account list-getapiusage` — Returns a paginated list of your API requests, including the endpoint called, status code, credits used, and timestamp.
- `scrape-creators-pp-cli account list-getdailyusagecount` — Returns aggregated daily usage statistics for the last 30 days
- `scrape-creators-pp-cli account list-getmostusedroutes` — Returns your top 20 most called API endpoints ranked by call count, along with total credits consumed per endpoint.

**amazon** — Manage amazon

- `scrape-creators-pp-cli amazon` — Scrapes a creator's Amazon Shop page by URL, returning their storefront profile and product collections.

**bluesky** — Get Bluesky posts and profile info

- `scrape-creators-pp-cli bluesky list` — Fetches a single Bluesky post by URL, returning the post's record text, author info, embed content, replyCount
- `scrape-creators-pp-cli bluesky list-profile` — Retrieves a Bluesky user's public profile including handle, displayName, avatar, description, followersCount
- `scrape-creators-pp-cli bluesky list-user` — Fetches a paginated feed of posts from a Bluesky user, returning each post's uri, record text, author info

**detect-age-gender** — Manage detect age gender

- `scrape-creators-pp-cli detect-age-gender` — Uses AI to analyze a creator's profile photo and estimate their age and gender.

**facebook** — Get public Facebook profiles and posts

- `scrape-creators-pp-cli facebook list` — Get the events of a city. Check out this [link](https://www.facebook.
- `scrape-creators-pp-cli facebook list-adlibrary` — Retrieves detailed information about a specific Facebook ad by its ID or URL.
- `scrape-creators-pp-cli facebook list-adlibrary-2` — Retrieves a transcript for a single Facebook Ad Library video ad by ID or URL.
- `scrape-creators-pp-cli facebook list-adlibrary-3` — Fetches all ads currently running for a specific company from the Meta Ad Library.
- `scrape-creators-pp-cli facebook list-adlibrary-4` — Searches the Meta Ad Library by keyword and returns matching ads.
- `scrape-creators-pp-cli facebook list-adlibrary-5` — Searches for companies by name in the Meta Ad Library and returns their page IDs for use with other ad library
- `scrape-creators-pp-cli facebook list-event` — Get a specific event by its URL or id
- `scrape-creators-pp-cli facebook list-events` — Search for events by name.
- `scrape-creators-pp-cli facebook list-group` — Fetches posts from a public Facebook group, limited to 3 posts per page due to API limitations.
- `scrape-creators-pp-cli facebook list-marketplace` — Fetches details for a Facebook Marketplace item by item id or Marketplace item URL, including title, description, price
- `scrape-creators-pp-cli facebook list-marketplace-2` — Searches Facebook Marketplace listings by keyword and lat/lng. Supports pagination with the returned cursor.
- `scrape-creators-pp-cli facebook list-marketplace-3` — Searches Facebook Marketplace locations/cities and returns coordinates you can use with the Marketplace Search endpoint.
- `scrape-creators-pp-cli facebook list-post` — Retrieves a single public Facebook post or reel by URL.
- `scrape-creators-pp-cli facebook list-post-2` — Fetches comments from a Facebook post or reel with cursor-based pagination.
- `scrape-creators-pp-cli facebook list-post-3` — Extracts the transcript text from a Facebook video post or reel.
- `scrape-creators-pp-cli facebook list-post-4` — Get the replies to a comment.
- `scrape-creators-pp-cli facebook list-profile` — Retrieves public Facebook page details including category, address, email, phone, website, services, priceRange, rating
- `scrape-creators-pp-cli facebook list-profile-2` — Get the events of a public Facebook page
- `scrape-creators-pp-cli facebook list-profile-3` — Fetches photos from a public Facebook page with pagination support.
- `scrape-creators-pp-cli facebook list-profile-4` — Returns publicly visible Facebook profile posts, limited to 3 posts per page due to API limitations.
- `scrape-creators-pp-cli facebook list-profile-5` — Fetches up to 10 reels per request from a public Facebook page.

**github** — Scrape GitHub profiles, repositories, and public activity

- `scrape-creators-pp-cli github list` — Retrieves public metadata for one GitHub repository, including owner, description, language, stars, forks, topics
- `scrape-creators-pp-cli github list-trending` — Scrapes GitHub's public Trending developers page.
- `scrape-creators-pp-cli github list-trending-2` — Scrapes GitHub's public Trending repositories page.
- `scrape-creators-pp-cli github list-user` — Retrieves public GitHub user details including name, bio, avatar, company, location, blog, follower counts
- `scrape-creators-pp-cli github list-user-2` — Retrieves GitHub profile contribution activity for a user from the public profile activity timeline.
- `scrape-creators-pp-cli github list-user-3` — Retrieves the public GitHub contribution graph for a user and year
- `scrape-creators-pp-cli github list-user-4` — Retrieves public GitHub followers for a user. Each follower includes login, avatar, user URL, type, and GitHub IDs.
- `scrape-creators-pp-cli github list-user-5` — Retrieves public accounts followed by a GitHub user.
- `scrape-creators-pp-cli github list-user-6` — Searches public GitHub pull requests authored by a user using GitHub's public search index.
- `scrape-creators-pp-cli github list-user-7` — Retrieves a user's public repositories with repo metadata like description, language, stars, forks, topics, license

**google** — Scrape Google search results

- `scrape-creators-pp-cli google list` — Retrieves detailed information about a specific Google ad including advertiserId, creativeId, format, firstShown
- `scrape-creators-pp-cli google list-adlibrary` — Searches the Google Ad Transparency Library for advertisers by name.
- `scrape-creators-pp-cli google list-company` — Fetches public ads for a company from the Google Ad Transparency Library by domain or advertiser_id.
- `scrape-creators-pp-cli google list-search` — Performs a Google search and returns organic results with url, title, and description for each result.

**instagram** — Gets Instagram profiles, posts, and reels

- `scrape-creators-pp-cli instagram list` — Fetches a lightweight Instagram profile summary by user ID, returning username, full name, biography
- `scrape-creators-pp-cli instagram list-audio` — Fetches the reels Instagram exposes for an audio page like instagram.com/reels/audio/{audio_id}/.
- `scrape-creators-pp-cli instagram list-media` — Generates an AI-powered speech-to-text transcription for an Instagram video post or reel.
- `scrape-creators-pp-cli instagram list-post` — Fetches detailed metadata for a single Instagram post or reel by shortcode or URL.
- `scrape-creators-pp-cli instagram list-post-2` — Retrieves comments on a public Instagram post or reel.
- `scrape-creators-pp-cli instagram list-profile` — Retrieves comprehensive public Instagram profile information including biography, bio links
- `scrape-creators-pp-cli instagram list-reels` — Fetches trending reels from Instagram's public instagram.com/reels page.
- `scrape-creators-pp-cli instagram list-reels-2` — Searches for Instagram reels matching a keyword or phrase via Google Search, bypassing Instagram's login-gated search.
- `scrape-creators-pp-cli instagram list-search` — Finds public Instagram posts for a hashtag using Google Search, then returns post details such as caption
- `scrape-creators-pp-cli instagram list-search-2` — Searches Google for public Instagram results matching a keyword or phrase, then returns matching public profiles.
- `scrape-creators-pp-cli instagram list-user` — Returns the raw HTML embed snippet for an Instagram user's profile widget.
- `scrape-creators-pp-cli instagram list-user-2` — Lists all story highlight albums for an Instagram user.
- `scrape-creators-pp-cli instagram list-user-3` — Returns a paginated list of a user's public Instagram reels (short-form videos).
- `scrape-creators-pp-cli instagram list-user-4` — Returns a paginated feed of a user's public Instagram posts, including reels, photos, videos, and carousels.
- `scrape-creators-pp-cli instagram list-user-5` — Fetches the full contents of a specific Instagram story highlight album by its ID.

**kick** — Scrape Kick clips

- `scrape-creators-pp-cli kick` — Fetches detailed data for a Kick clip by URL, including video, metadata, and channel info.

**komi** — Scrape Komi pages

- `scrape-creators-pp-cli komi` — Scrapes a Komi page by URL, extracting the creator's profile, social links, and featured content.

**kwai** — Scrape Kwai profiles, posts, and user feeds

- `scrape-creators-pp-cli kwai list` — Fetches public Kwai post details including caption, media URLs, cover images, counts, author info, and music metadata.
- `scrape-creators-pp-cli kwai list-profile` — Fetches public Kwai profile data including username, bio, avatar, verification status, gender, and public counts.
- `scrape-creators-pp-cli kwai list-user` — Fetches a paginated list of public Kwai posts for a user, including captions, media URLs, covers, counts, author info

**linkbio** — Scrape Linkbio (lnk.bio) pages

- `scrape-creators-pp-cli linkbio` — Scrapes a Linkbio (lnk.bio) page by URL, extracting the creator's profile and all their links.

**linkedin** — Scrape LinkedIn

- `scrape-creators-pp-cli linkedin list` — Retrieves detailed information about a specific LinkedIn ad by URL.
- `scrape-creators-pp-cli linkedin list-ads` — Searches the LinkedIn Ad Library by company name, keyword, or companyId with optional country and date filters.
- `scrape-creators-pp-cli linkedin list-company` — Fetches a LinkedIn company page with details including name, description, logo, cover image, slogan, location
- `scrape-creators-pp-cli linkedin list-company-2` — Retrieves paginated posts from a LinkedIn company page, including each post's URL, ID, publication date
- `scrape-creators-pp-cli linkedin list-post` — Fetches a single LinkedIn post or article, returning the title, headline, full description text
- `scrape-creators-pp-cli linkedin list-post-2` — Fetches the transcript from a LinkedIn post video when LinkedIn exposes one publicly.
- `scrape-creators-pp-cli linkedin list-profile` — Retrieves a person's public LinkedIn profile data, including their name, photo, location, follower count (followers)
- `scrape-creators-pp-cli linkedin list-search` — Finds public LinkedIn posts, feed updates, and Pulse articles by keyword using Google Search

**linkme** — Get Linkme profile info

- `scrape-creators-pp-cli linkme` — Retrieves a Linkme profile by URL, including identity, social links, and contact details.

**linktree** — Scrape Linktree pages

- `scrape-creators-pp-cli linktree` — Scrapes a Linktree page by URL, extracting the creator's profile and all their links.

**pillar** — Scrape Pillar pages

- `scrape-creators-pp-cli pillar` — Scrapes a Pillar page by URL, extracting the creator's profile, social links, and products.

**pinterest** — Scrape Pinterest pins

- `scrape-creators-pp-cli pinterest list` — Fetches a paginated list of pins from a Pinterest board by URL, returning each pin's id, description, title, images
- `scrape-creators-pp-cli pinterest list-pin` — Fetches detailed information about a single Pinterest pin by URL, returning title, description, link, dominantColor
- `scrape-creators-pp-cli pinterest list-search` — Searches Pinterest for pins matching a query, returning results with id, url, title, description, images, link, domain
- `scrape-creators-pp-cli pinterest list-user` — Fetches a paginated list of boards for a Pinterest user, returning each board's name, url, description, pin_count

**reddit** — Scrape Reddit posts and comments

- `scrape-creators-pp-cli reddit list` — Searches across all of Reddit for posts matching a query.
- `scrape-creators-pp-cli reddit list-post` — Retrieves comments and post details from a Reddit post by URL.
- `scrape-creators-pp-cli reddit list-post-2` — Gets the transcript from a Reddit video post or direct v.redd.it URL when Reddit exposes a VTT caption file.
- `scrape-creators-pp-cli reddit list-subreddit` — Fetches posts from a subreddit with sorting and filtering options.
- `scrape-creators-pp-cli reddit list-subreddit-2` — Retrieves metadata about a subreddit by name or URL. The subreddit name must be case-sensitive.
- `scrape-creators-pp-cli reddit list-subreddit-3` — Searches within a specific subreddit for posts, comments, and media matching a query.

**rumble** — Scrape Rumble search, videos, transcripts, and channel videos

- `scrape-creators-pp-cli rumble list` — Searches Rumble videos by keyword.
- `scrape-creators-pp-cli rumble list-channel` — Gets videos from a Rumble channel by handle or URL.
- `scrape-creators-pp-cli rumble list-video` — Gets Rumble video details by URL.
- `scrape-creators-pp-cli rumble list-video-2` — Gets all top level comments for a Rumble video by URL.
- `scrape-creators-pp-cli rumble list-video-3` — Gets a Rumble video's transcript when captions are available.

**snapchat** — Scrape Snapchat user profiles and thier stories

- `scrape-creators-pp-cli snapchat` — Retrieves a Snapchat user's public profile by handle, including identity, stories, and spotlight content.

**soundcloud** — Scrape SoundCloud playlists and tracks

- `scrape-creators-pp-cli soundcloud list` — Fetches detailed information about a SoundCloud artist by its handle or URL.
- `scrape-creators-pp-cli soundcloud list-artist` — Fetches tracks/songs for a SoundCloud artist by handle or URL.
- `scrape-creators-pp-cli soundcloud list-track` — Fetches detailed information about a SoundCloud track/song by URL.

**spotify** — Scrape Spotify artists, songs, and albums

- `scrape-creators-pp-cli spotify list` — Retrieves detailed information about a Spotify album by its id or URL, including album metadata, artists, release date
- `scrape-creators-pp-cli spotify list-artist` — Retrieves detailed information about a Spotify artist by their handle, including name, followers count, genres
- `scrape-creators-pp-cli spotify list-podcast` — Retrieves detailed information about a Spotify podcast by its id or URL.
- `scrape-creators-pp-cli spotify list-podcast-2` — Returns episodes for a Spotify podcast. Pass the cursor returned by a response to get the next page.
- `scrape-creators-pp-cli spotify list-search` — Search Spotify for tracks, artists, albums, episodes, podcasts, and audiobooks.
- `scrape-creators-pp-cli spotify list-track` — Retrieves detailed information about a Spotify track by its id or URL, including track metadata, artists, album info

**threads** — Get Threads posts

- `scrape-creators-pp-cli threads list` — Fetches a single Threads post by URL, returning the post's caption, like_count, view_counts, reshare_count
- `scrape-creators-pp-cli threads list-profile` — Retrieves a Threads user's public profile including username, full_name, biography, profile_pic_url, follower_count
- `scrape-creators-pp-cli threads list-search` — Searches Threads for posts matching a keyword, returning up to 10 results with caption text, like_count, reshare_count
- `scrape-creators-pp-cli threads list-search-2` — Searches for Threads users by username, returning matching profiles with username, full_name, profile_pic_url
- `scrape-creators-pp-cli threads list-user` — Fetches the most recent posts from a Threads user, returning id, caption text, code, like_count, reshare_count

**tiktok** — Scrape TikTok profiles, videos, and more

- `scrape-creators-pp-cli tiktok list` — Fetches TikTok's trending/For You feed for a given region — useful for discovering viral content and what's currently
- `scrape-creators-pp-cli tiktok list-adlibrary` — Fetches one TikTok Creative Center Top Ad by material/ad ID or URL.
- `scrape-creators-pp-cli tiktok list-adlibrary-2` — Searches TikTok Creative Center Top Ads, the ad library page at ads.tiktok.
- `scrape-creators-pp-cli tiktok list-creators` — Discovers trending and popular TikTok creators, filterable by follower count range, creator country
- `scrape-creators-pp-cli tiktok list-hashtags` — Discovers trending and popular TikTok hashtags, filterable by time period (7/30/120 days) and country.
- `scrape-creators-pp-cli tiktok list-live` — Gets curated room-level info for a TikTok live using TokAPI's live info endpoint.
- `scrape-creators-pp-cli tiktok list-product` — Fetches full details for a specific US TikTok Shop product by its URL, including stock levels and affiliate videos.
- `scrape-creators-pp-cli tiktok list-profile` — Fetches public profile data for a TikTok user by their handle or user_id — useful for looking up a creator's identity
- `scrape-creators-pp-cli tiktok list-profile-2` — Returns the TikTok region code for a public profile, like `US` for United States or `MX` for Mexico.
- `scrape-creators-pp-cli tiktok list-profile-3` — Fetches videos posted by a TikTok user
- `scrape-creators-pp-cli tiktok list-search` — Searches for TikTok videos under a specific hashtag — useful for finding content by topic or trend.
- `scrape-creators-pp-cli tiktok list-search-2` — Searches for TikTok videos by keyword or phrase — the general video search across all of TikTok.
- `scrape-creators-pp-cli tiktok list-search-3` — Gets the autocomplete suggestions TikTok shows while someone is typing in search.
- `scrape-creators-pp-cli tiktok list-search-4` — Searches TikTok's 'Top' results by query — returns both videos and photo carousels
- `scrape-creators-pp-cli tiktok list-search-5` — Searches for TikTok users by keyword or name — useful for finding creators or accounts matching a query.
- `scrape-creators-pp-cli tiktok list-shop` — Lists all products from a specific TikTok Shop store by its URL.
- `scrape-creators-pp-cli tiktok list-shop-2` — Searches TikTok Shop for products matching a keyword query.
- `scrape-creators-pp-cli tiktok list-shop-3` — Fetches customer reviews for a TikTok Shop product by URL or product_id.
- `scrape-creators-pp-cli tiktok list-song` — Fetches detailed metadata for a specific TikTok sound or song by its clipId.
- `scrape-creators-pp-cli tiktok list-song-2` — Fetches TikTok videos that use a specific sound or song, identified by its clipId.
- `scrape-creators-pp-cli tiktok list-user` — Retrieves audience demographic data for a TikTok user, showing where their followers are located by country.
- `scrape-creators-pp-cli tiktok list-user-2` — Retrieves the follower list of a TikTok account by handle or user_id — useful for seeing who follows a creator or
- `scrape-creators-pp-cli tiktok list-user-3` — Retrieves the following list — accounts that a TikTok user follows — by their handle.
- `scrape-creators-pp-cli tiktok list-user-4` — Checks if a TikTok user is currently live streaming and retrieves their live room details.
- `scrape-creators-pp-cli tiktok list-user-5` — Fetches products featured in a TikTok user's public showcase — the products a creator promotes on their profile.
- `scrape-creators-pp-cli tiktok list-video` — Fetches detailed data for a single TikTok video by URL, including its metadata, engagement stats
- `scrape-creators-pp-cli tiktok list-video-2` — Fetches comments on a TikTok video by URL — useful for reading audience reactions, replies, and engagement.
- `scrape-creators-pp-cli tiktok list-video-3` — Extracts the transcript, captions, or subtitles from a TikTok video by URL.
- `scrape-creators-pp-cli tiktok list-video-4` — Fetches replies to a specific TikTok comment by its ID.

**truthsocial** — Manage truthsocial

- `scrape-creators-pp-cli truthsocial list` — Fetches a single Truth Social post by URL, returning text, id, created_at, url, content, account details
- `scrape-creators-pp-cli truthsocial list-profile` — Retrieves a Truth Social user's public profile including display_name, username, avatar, header, followers_count
- `scrape-creators-pp-cli truthsocial list-user` — Fetches a paginated list of posts from a Truth Social user, returning text, id, created_at, url, content, account info

**twitch** — Scrape Twitch clips

- `scrape-creators-pp-cli twitch list` — Fetches detailed data for a Twitch clip by URL, including metadata and direct video URLs.
- `scrape-creators-pp-cli twitch list-profile` — Retrieves a Twitch user's public profile by handle, including identity, social links, and content.
- `scrape-creators-pp-cli twitch list-user` — Fetches a user's schedule by handle, returning a list of scheduled events with start time, end time, title, description
- `scrape-creators-pp-cli twitch list-user-2` — Fetches a list of videos (100 max) for a Twitch user, returning each video's id, slug, url, embedURL, title, viewCount

**twitter** — Get Twitter profiles, tweets, followers and more

- `scrape-creators-pp-cli twitter list` — Retrieves details about a Twitter/X Community by URL.
- `scrape-creators-pp-cli twitter list-community` — Fetches tweets posted within a Twitter/X Community by URL.
- `scrape-creators-pp-cli twitter list-profile` — Retrieves a Twitter user's profile by handle, including account metadata and statistics.
- `scrape-creators-pp-cli twitter list-tweet` — Retrieves detailed information about a specific tweet by URL, including the author's profile and engagement metrics.
- `scrape-creators-pp-cli twitter list-tweet-2` — Extracts the transcript from a Twitter video tweet using AI-powered transcription.
- `scrape-creators-pp-cli twitter list-usertweets` — Fetches tweets from a Twitter user's profile by handle.

**youtube** — Scrape YouTube channels, videos, and more

- `scrape-creators-pp-cli youtube list` — Retrieves comprehensive YouTube channel profile data including name, avatar images, subscriber count (subscribers)
- `scrape-creators-pp-cli youtube list-channel` — Fetches community posts from a YouTube channel's Posts tab, including post ID, URL, content, images, attached video
- `scrape-creators-pp-cli youtube list-channel-2` — Fetches live streams and past streams from a YouTube channel's Live tab, including title, URL, thumbnail, view count
- `scrape-creators-pp-cli youtube list-channel-3` — Fetches playlists from a YouTube channel's Playlists tab, including playlist ID, title, thumbnail, video count
- `scrape-creators-pp-cli youtube list-channel-4` — Retrieves a paginated list of short-form videos (Shorts) from a YouTube channel, including each short's title, URL
- `scrape-creators-pp-cli youtube list-channelvideos` — Fetches a paginated list of videos uploaded by a YouTube channel, including each video's title, URL, thumbnail
- `scrape-creators-pp-cli youtube list-communitypost` — Retrieves the full details of a YouTube community post, including its text content, attached images, like count
- `scrape-creators-pp-cli youtube list-playlist` — Retrieves all videos in a YouTube playlist, including the playlist title, owner info, total video count
- `scrape-creators-pp-cli youtube list-search` — Searches YouTube by keyword query and returns matching videos, channels, playlists, shorts, shelves, and live streams.
- `scrape-creators-pp-cli youtube list-search-2` — Searches YouTube for content matching a specific hashtag and returns matching videos with title, URL, thumbnail
- `scrape-creators-pp-cli youtube list-shorts` — Fetches approximately 48 currently trending YouTube Shorts (viral/popular short-form videos) per call
- `scrape-creators-pp-cli youtube list-video` — Fetches full details for a YouTube video or short, including title, description, thumbnail, view count (views)
- `scrape-creators-pp-cli youtube list-video-2` — Fetches comments and replies from a YouTube video, including each comment's text content, author details, like count
- `scrape-creators-pp-cli youtube list-video-3` — Experimental endpoint.
- `scrape-creators-pp-cli youtube list-video-4` — Retrieves the captions, subtitles, or transcript of a YouTube video or short.
- `scrape-creators-pp-cli youtube list-video-5` — Fetches replies to a specific comment on a YouTube video, including each reply's text content, author details (name


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
scrape-creators-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Vet an influencer shortlist

```bash
scrape-creators-pp-cli creator compare mkbhd mrwhosetheboss unboxtherapy --agent --select handle,platform,follower_count,engagement_rate
```

Compare candidates on engagement rate, not just follower count, to separate real reach from vanity metrics.

### Monitor a competitors ads weekly

```bash
scrape-creators-pp-cli ads monitor nike --agent
```

First run snapshots Nikes current creatives across Facebook, TikTok, Google, and LinkedIn; rerun weekly to diff new vs. pulled ads.

### Search a cached transcript corpus

```bash
scrape-creators-pp-cli transcripts search "sponsored by" --agent --select creator,platform,snippet
```

Transcripts are cached to the local store whenever you run the per-platform transcript commands; this searches them offline with no credits re-spent.

### Catch a rising trends leading platform

```bash
scrape-creators-pp-cli trends triangulate "labubu" --agent
```

See per-platform result velocity for a topic and which platform it is cresting on first.

### Check credit runway before a big pull

```bash
scrape-creators-pp-cli account budget --agent
```

Project days-remaining at your current burn rate so a batch of calls does not hit HTTP 402 halfway through.

## Auth Setup

Authentication is a single Scrape Creators API key sent in the x-api-key header. Set SCRAPECREATORS_API_KEY in your environment, or run 'auth login' to store it. Credits are pay-as-you-go and never expire; a depleted balance returns HTTP 402, so check 'account budget' before a large sync.

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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `SCRAPE_CREATORS_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `SCRAPE_CREATORS_CONFIG_DIR`, `SCRAPE_CREATORS_DATA_DIR`, `SCRAPE_CREATORS_STATE_DIR`, `SCRAPE_CREATORS_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `SCRAPE_CREATORS_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `scrape-creators-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "scrape-creators": {
        "command": "scrape-creators-pp-mcp",
        "env": {
          "SCRAPE_CREATORS_HOME": "/srv/scrape-creators"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `SCRAPE_CREATORS_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `SCRAPE_CREATORS_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
scrape-creators-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
scrape-creators-pp-cli feedback --stdin < notes.txt
scrape-creators-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `SCRAPE_CREATORS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SCRAPE_CREATORS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
