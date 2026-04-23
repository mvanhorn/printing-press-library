# Scrape Creators CLI

The easiest way to scrape public social media data at scale. Extract profiles, posts, videos, comments, and more from TikTok, Instagram, YouTube, Twitter, LinkedIn, Facebook, Reddit, and 27+ platforms.

Learn more at [Scrape Creators](https://scrapecreators.com).

## Install

### Go

```
go install github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/cmd/scrape-creators-pp-cli@latest
```

### Binary

Download from [Releases](https://github.com/mvanhorn/printing-press-library/releases).

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your API key from your API provider's developer portal. The key typically looks like a long alphanumeric string.

```bash
export SCRAPE_CREATORS_API_KEY_AUTH="<paste-your-key>"
```

You can also persist this in your config file at `~/.config/scrape-creators-pp-cli/config.toml`.

### 3. Verify Setup

```bash
scrape-creators-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
scrape-creators-pp-cli account list
```

## Unique Features

These capabilities aren't available in any other tool for this API.

- **`videos spikes`** — Find videos that performed significantly above a creator's average — the ones that actually went viral.
- **`transcripts search`** — Search across all a creator's video transcripts for any keyword or phrase — like grep for TikTok.
- **`profile compare`** — Compare two or more creators side-by-side on follower count, engagement rate, posting cadence, and content volume.
- **`videos cadence`** — See when a creator posts — by day of week and hour — so you can benchmark their publishing strategy.
- **`profile track`** — Record daily follower snapshots for any creator and chart their growth trajectory over time.
- **`account budget`** — Track how quickly you're spending API credits and project how many days until you hit your limit.
- **`search trends`** — Track whether a hashtag is growing or shrinking by comparing video counts across snapshot intervals.
- **`videos analyze`** — Rank all of a creator's synced videos by engagement rate (not raw likes) to surface their true best performers.

## Usage

<!-- HELP_OUTPUT -->

## Commands

### account

Manage account

- **`scrape-creators-pp-cli account list`** - Get credit balance
- **`scrape-creators-pp-cli account list-getapiusage`** - Get request history
- **`scrape-creators-pp-cli account list-getdailyusagecount`** - Get daily usage
- **`scrape-creators-pp-cli account list-getmostusedroutes`** - Get most used routes

### amazon

Manage amazon

- **`scrape-creators-pp-cli amazon list`** - Amazon Shop page

### bluesky

Get Bluesky posts and profile info

- **`scrape-creators-pp-cli bluesky list`** - Fetches a single Bluesky post by URL, returning the post's record text, author info, embed content, replyCount, repostCount, likeCount, and quoteCount. Also includes a replies array with threaded reply posts.
- **`scrape-creators-pp-cli bluesky list-profile`** - Retrieves a Bluesky user's public profile including handle, displayName, avatar, description, followersCount, followsCount, postsCount, createdAt, and verification status. The associated field shows counts for lists, feed generators, and starter packs.
- **`scrape-creators-pp-cli bluesky list-user`** - Fetches a paginated feed of posts from a Bluesky user, returning each post's uri, record text, author info, embed content, replyCount, repostCount, likeCount, quoteCount, and indexedAt. Supports pagination via cursor. Use user_id (the 'did') instead of handle for faster response times.

### detect-age-gender

Manage detect age gender

- **`scrape-creators-pp-cli detect-age-gender list`** - Get Age and Gender

### facebook

Get public Facebook profiles and posts

- **`scrape-creators-pp-cli facebook list`** - Ad Details
- **`scrape-creators-pp-cli facebook list-adlibrary`** - Company Ads
- **`scrape-creators-pp-cli facebook list-adlibrary-2`** - Searches the Meta Ad Library by keyword and returns matching ads. Each result includes ad_archive_id, page_name, is_active, publisher_platform, and a snapshot with body text, images, videos, and cta_text. Results cap around 1,500 via GET due to cursor size limits; switch to POST method with body params for larger result sets.
- **`scrape-creators-pp-cli facebook list-adlibrary-3`** - Search for Companies
- **`scrape-creators-pp-cli facebook list-group`** - Facebook Group Posts
- **`scrape-creators-pp-cli facebook list-post`** - Retrieves a single public Facebook post or reel by URL. Returns post_id, like_count, comment_count, share_count, view_count, description, creation_time, and author details. For video posts, includes video sd_url, hd_url, thumbnail, and length_in_second. Optionally fetches comments and transcript via get_comments and get_transcript parameters.
- **`scrape-creators-pp-cli facebook list-post-2`** - Fetches comments from a Facebook post or reel with cursor-based pagination. Each comment includes id, text, created_at, reply_count, reaction_count, and author details with name and profile_picture. Passing a feedback_id instead of a url significantly speeds up the request.
- **`scrape-creators-pp-cli facebook list-post-3`** - Extracts the transcript text from a Facebook video post or reel. Returns the transcript as a single text string with line breaks. Only works on videos under 2 minutes in length.
- **`scrape-creators-pp-cli facebook list-profile`** - Retrieves public Facebook page details including category, address, email, phone, website, services, priceRange, rating, likeCount, and followerCount. Also returns adLibrary status with the page's ad activity and pageId. Optionally includes businessHours when get_business_hours is set to true.
- **`scrape-creators-pp-cli facebook list-profile-2`** - Profile Photos
- **`scrape-creators-pp-cli facebook list-profile-3`** - Profile Posts
- **`scrape-creators-pp-cli facebook list-profile-4`** - Profile Reels

### google

Scrape Google search results

- **`scrape-creators-pp-cli google list`** - Ad Details
- **`scrape-creators-pp-cli google list-adlibrary`** - Advertiser Search
- **`scrape-creators-pp-cli google list-company`** - Company Ads
- **`scrape-creators-pp-cli google list-search`** - Performs a Google search and returns organic results with url, title, and description for each result. Supports an optional region parameter (2-letter country code) to get localized results from a specific country.

### instagram

Gets Instagram profiles, posts, and reels

- **`scrape-creators-pp-cli instagram list`** - Basic Profile
- **`scrape-creators-pp-cli instagram list-media`** - Generates an AI-powered speech-to-text transcription for an Instagram video post or reel. The video must be under 2 minutes long. Returns a transcripts array with each item's shortcode and transcribed text; carousel posts produce one transcript per video slide. Expect 10-30 second response times, and null when no speech is detected.
- **`scrape-creators-pp-cli instagram list-post`** - Post/Reel Info
- **`scrape-creators-pp-cli instagram list-post-2`** - Retrieves comments on a public Instagram post or reel. Each comment includes the comment text, creation timestamp, and commenter details such as username, user ID, verification status, and profile picture URL. Supports cursor-based pagination to load additional comment pages.
- **`scrape-creators-pp-cli instagram list-profile`** - Retrieves comprehensive public Instagram profile information including biography, bio links, follower and following counts, verification status, and profile picture URLs. Also returns recent timeline posts with engagement metrics such as likes, comments, and video view counts, plus a list of related profiles. Useful for account overview, audience analysis, or discovering similar creators.
- **`scrape-creators-pp-cli instagram list-reels`** - Search Reels
- **`scrape-creators-pp-cli instagram list-song`** - Reels using Song (Deprecated)
- **`scrape-creators-pp-cli instagram list-user`** - Embed HTML
- **`scrape-creators-pp-cli instagram list-user-2`** - Highlights Details
- **`scrape-creators-pp-cli instagram list-user-3`** - Story Highlights
- **`scrape-creators-pp-cli instagram list-user-4`** - Returns a paginated list of a user's public Instagram reels (short-form videos). Each reel includes its shortcode, play count, like count, comment count, video versions with download URLs, thumbnail image, and owner info. Note that reel captions are not returned by this endpoint. Play counts are Instagram-only views and exclude cross-posted Facebook views. Supports cursor-based pagination via max_id; providing a user_id instead of a handle yields faster responses.
- **`scrape-creators-pp-cli instagram list-user-5`** - Returns a paginated feed of a user's public Instagram posts, including photos, videos, and carousels. Each item includes media type, shortcode, caption text, like count, comment count, play count, video URLs, image URLs, and tagged users. Play counts reflect Instagram-only views and exclude cross-posted Facebook views. Supports cursor-based pagination via next_max_id for scrolling through the full timeline.

### kick

Scrape Kick clips

- **`scrape-creators-pp-cli kick list`** - Fetches detailed data for a Kick clip by URL, including video, metadata, and channel info. Returns clip id, title, clip_url, thumbnail_url, video_url, view_count, likes_count, duration, privacy status, and is_mature flag. Also includes category details (name, slug), creator info (username), and channel info (username, profile_picture).

### komi

Scrape Komi pages

- **`scrape-creators-pp-cli komi list`** - Komi page

### linkbio

Scrape Linkbio (lnk.bio) pages

- **`scrape-creators-pp-cli linkbio list`** - Linkbio page

### linkedin

Scrape LinkedIn

- **`scrape-creators-pp-cli linkedin list`** - Ad Details
- **`scrape-creators-pp-cli linkedin list-ads`** - Search Ads
- **`scrape-creators-pp-cli linkedin list-company`** - Company Page
- **`scrape-creators-pp-cli linkedin list-company-2`** - Company Posts
- **`scrape-creators-pp-cli linkedin list-post`** - Fetches a single LinkedIn post or article, returning the title, headline, full description text, author info with follower count, publication date, like count (reactions), comment count, and individual comments. Also includes related articles from the same author in moreArticles.
- **`scrape-creators-pp-cli linkedin list-profile`** - Person's Profile

### linkme

Get Linkme profile info

- **`scrape-creators-pp-cli linkme list`** - Retrieves a Linkme profile by URL, including identity, social links, and contact details. Returns profile with id, firstName, username, bio, profileVisitCount, profileImage, verifiedAccount, and isAmbassador flag. Also includes infoLinks (email addresses) and webLinks, an array of categorized social platform links (Spotify, Instagram, YouTube, Twitter, Facebook, and more) each with linkValue and faceValue.

### linktree

Scrape Linktree pages

- **`scrape-creators-pp-cli linktree list`** - Linktree page

### pillar

Scrape Pillar pages

- **`scrape-creators-pp-cli pillar list`** - Pillar page

### pinterest

Scrape Pinterest pins

- **`scrape-creators-pp-cli pinterest list`** - Fetches a paginated list of pins from a Pinterest board by URL, returning each pin's id, description, title, images, board info, pin_join annotations, and aggregated_pin_data. Supports pagination via cursor and a trim option for lighter responses.
- **`scrape-creators-pp-cli pinterest list-pin`** - Fetches detailed information about a single Pinterest pin by URL, returning title, description, link, dominantColor, originPinner, pinner, images at multiple resolutions (imageSpec_236x through imageSpec_orig), and pinJoin with visual annotations. Supports a trim option for lighter responses.
- **`scrape-creators-pp-cli pinterest list-search`** - Searches Pinterest for pins matching a query, returning results with id, url, title, description, images, link, domain, board info, and pinner details. Supports pagination via cursor and a trim option for lighter responses.
- **`scrape-creators-pp-cli pinterest list-user`** - User Boards

### reddit

Scrape Reddit posts and comments

- **`scrape-creators-pp-cli reddit list`** - Get Ad
- **`scrape-creators-pp-cli reddit list-ads`** - Search Ads
- **`scrape-creators-pp-cli reddit list-post`** - Post Comments
- **`scrape-creators-pp-cli reddit list-search`** - Searches across all of Reddit for posts matching a query. Each post includes title, author, selftext, subreddit, score, ups, upvote_ratio, num_comments, created_utc, url, permalink, and is_video. Supports sort (relevance, new, top, comment_count), timeframe filtering, pagination via the after token, and a trim parameter for lighter responses.
- **`scrape-creators-pp-cli reddit list-subreddit`** - Subreddit Posts
- **`scrape-creators-pp-cli reddit list-subreddit-2`** - Subreddit Details
- **`scrape-creators-pp-cli reddit list-subreddit-3`** - Subreddit Search

### snapchat

Scrape Snapchat user profiles and thier stories

- **`scrape-creators-pp-cli snapchat list`** - User Profile

### threads

Get Threads posts

- **`scrape-creators-pp-cli threads list`** - Fetches a single Threads post by URL, returning the post's caption, like_count, view_counts, reshare_count, direct_reply_count, image_versions2, and taken_at. Also includes comments and related_posts arrays. Supports a trim option for lighter responses.
- **`scrape-creators-pp-cli threads list-profile`** - Retrieves a Threads user's public profile including username, full_name, biography, profile_pic_url, follower_count, is_verified, bio_links, and hd_profile_pic_versions. Also indicates whether the account is a threads-only user via is_threads_only_user.
- **`scrape-creators-pp-cli threads list-search`** - Search by Keyword
- **`scrape-creators-pp-cli threads list-search-2`** - Search Users
- **`scrape-creators-pp-cli threads list-user`** - Fetches the most recent posts from a Threads user, returning id, caption text, code, like_count, reshare_count, direct_reply_count, repost_count, image_versions2, video_versions, and taken_at. Only the last 20-30 posts are publicly visible. Supports a trim option for lighter responses.

### tiktok

Scrape TikTok profiles, videos, and more

- **`scrape-creators-pp-cli tiktok list`** - Get popular creators
- **`scrape-creators-pp-cli tiktok list-gettrendingfeed`** - Trending Feed
- **`scrape-creators-pp-cli tiktok list-hashtags`** - Get popular hashtags
- **`scrape-creators-pp-cli tiktok list-product`** - Product Details
- **`scrape-creators-pp-cli tiktok list-profile`** - Fetches public profile data for a TikTok user by their handle — useful for looking up a creator's identity, bio, and account stats. Returns a `user` object (display name, avatar URLs, bio/signature, verification status, bio link) and a `stats` object (followerCount, followingCount, heartCount/total likes, videoCount). This only returns profile metadata, not the user's actual videos or followers list.
- **`scrape-creators-pp-cli tiktok list-profile-2`** - Profile Videos
- **`scrape-creators-pp-cli tiktok list-search`** - Search by Hashtag
- **`scrape-creators-pp-cli tiktok list-search-2`** - Search by Keyword
- **`scrape-creators-pp-cli tiktok list-search-3`** - Top Search
- **`scrape-creators-pp-cli tiktok list-search-4`** - Search Users
- **`scrape-creators-pp-cli tiktok list-shop`** - Product Reviews
- **`scrape-creators-pp-cli tiktok list-shop-2`** - Shop Products
- **`scrape-creators-pp-cli tiktok list-shop-3`** - Shop Search
- **`scrape-creators-pp-cli tiktok list-song`** - Get Song Details
- **`scrape-creators-pp-cli tiktok list-song-2`** - TikToks using Song
- **`scrape-creators-pp-cli tiktok list-songs`** - Get popular songs
- **`scrape-creators-pp-cli tiktok list-user`** - User's Audience Demographics
- **`scrape-creators-pp-cli tiktok list-user-2`** - Retrieves the follower list of a TikTok account by handle or user_id — useful for seeing who follows a creator or getting subscriber data. Returns `followers`, an array of user objects each with `nickname`, `unique_id`, `uid`, `follower_count`, `following_count`, and avatar URLs; also returns `total` follower count. Paginate with `min_time` from the previous response.
- **`scrape-creators-pp-cli tiktok list-user-3`** - Retrieves the following list — accounts that a TikTok user follows — by their handle. Returns `followings`, an array of user objects each with `nickname`, `unique_id`, `uid`, `follower_count`, `following_count`, `signature`, and avatar URLs; also returns `total` count. Paginate with `min_time` from the previous response.
- **`scrape-creators-pp-cli tiktok list-user-4`** - TikTok Live
- **`scrape-creators-pp-cli tiktok list-user-5`** - User Showcase
- **`scrape-creators-pp-cli tiktok list-video`** - Comment Replies
- **`scrape-creators-pp-cli tiktok list-video-2`** - Fetches comments on a TikTok video by URL — useful for reading audience reactions, replies, and engagement. Returns `comments`, an array where each comment includes `text`, `digg_count` (likes), `reply_comment_total`, `create_time`, and a `user` object with the commenter's nickname and unique_id; also returns `total` comment count. Paginate with `cursor` from the previous response.
- **`scrape-creators-pp-cli tiktok list-video-3`** - Extracts the transcript, captions, or subtitles from a TikTok video by URL. Returns `id`, `url`, and `transcript` as a WEBVTT-formatted string with timestamped text segments. Video must be under 2 minutes; costs an additional 10 credits when `use_ai_as_fallback=true`.
- **`scrape-creators-pp-cli tiktok list-video-4`** - Video Info
- **`scrape-creators-pp-cli tiktok list-videos`** - Get popular videos

### truthsocial

Manage truthsocial

- **`scrape-creators-pp-cli truthsocial list`** - Fetches a single Truth Social post by URL, returning text, id, created_at, url, content, account details, media_attachments, card link previews, replies_count, reblogs_count, and favourites_count. Only posts from prominent public figures (e.g., Trump, Vance) are accessible without authentication.
- **`scrape-creators-pp-cli truthsocial list-profile`** - Retrieves a Truth Social user's public profile including display_name, username, avatar, header, followers_count, following_count, statuses_count, verified status, website, and created_at. Only prominent public figures (e.g., Trump, Vance) are accessible without authentication; most other accounts will not work.
- **`scrape-creators-pp-cli truthsocial list-user`** - User Posts

### twitch

Scrape Twitch clips

- **`scrape-creators-pp-cli twitch list`** - Fetches detailed data for a Twitch clip by URL, including metadata and direct video URLs. Returns clip id, slug, url, embedURL, title, viewCount, language, durationSeconds, game info, broadcaster details with follower count, thumbnailURL, and videoQualities at multiple resolutions with a signed videoURL for playback. Also includes additional clips from the same broadcaster.
- **`scrape-creators-pp-cli twitch list-profile`** - Retrieves a Twitch user's public profile by handle, including identity, social links, and content. Returns id, handle, displayName, description, followers count, and linked social accounts (instagram, x, tiktok). Also includes allVideos with game info, duration, and view counts, featuredClips with clip metadata and thumbnails, and similarStreamers.

### twitter

Get Twitter profiles, tweets, followers and more

- **`scrape-creators-pp-cli twitter list`** - Retrieves details about a Twitter/X Community by URL. Returns the community name, description, rest_id, join_policy, created_at, member_count, rules, and creator_results with the creator's profile. Also includes members_facepile_results with avatar images of recent members.
- **`scrape-creators-pp-cli twitter list-community`** - Community Tweets
- **`scrape-creators-pp-cli twitter list-profile`** - Retrieves a Twitter user's profile by handle, including account metadata and statistics. Returns name, screen_name, description, followers_count, friends_count, statuses_count, favourites_count, location, profile_image_url_https, and is_blue_verified. Also includes verification_info, tipjar_settings, highlights_info, and creator_subscriptions_count.
- **`scrape-creators-pp-cli twitter list-tweet`** - Tweet Details
- **`scrape-creators-pp-cli twitter list-tweet-2`** - Extracts the transcript from a Twitter video tweet using AI-powered transcription. The video must be under 2 minutes long. Returns a success flag and the full transcript text. This endpoint is slower than others due to the AI processing step.
- **`scrape-creators-pp-cli twitter list-usertweets`** - User Tweets

### youtube

Scrape YouTube channels, videos, and more

- **`scrape-creators-pp-cli youtube list`** - Channel Details
- **`scrape-creators-pp-cli youtube list-channel`** - Channel Shorts
- **`scrape-creators-pp-cli youtube list-channelvideos`** - Channel Videos
- **`scrape-creators-pp-cli youtube list-communitypost`** - Community Post Details
- **`scrape-creators-pp-cli youtube list-playlist`** - Retrieves all videos in a YouTube playlist, including the playlist title, owner info, total video count, and each video's title, URL, thumbnail, duration, and channel. Accepts the playlist ID found in the 'list' URL parameter.
- **`scrape-creators-pp-cli youtube list-search`** - Searches YouTube by keyword query and returns matching videos, channels, playlists, shorts, shelves, and live streams. Each video result includes title, URL, thumbnail, view count (views), publish date, duration, channel info, and badges. Supports filtering by upload date, sorting by relevance or popularity, and paginating with continuationToken.
- **`scrape-creators-pp-cli youtube list-search-2`** - Search by Hashtag
- **`scrape-creators-pp-cli youtube list-shorts`** - Trending Shorts
- **`scrape-creators-pp-cli youtube list-video`** - Video/Short Details
- **`scrape-creators-pp-cli youtube list-video-2`** - Comment Replies
- **`scrape-creators-pp-cli youtube list-video-3`** - Fetches comments and replies from a YouTube video, including each comment's text content, author details, like count, reply count, and publish date. Supports ordering by top or newest, and paginating with continuationToken. Limited to approximately 1,000 top comments or 7,000 newest comments.
- **`scrape-creators-pp-cli youtube list-video-4`** - Retrieves the captions, subtitles, or transcript of a YouTube video or short. Returns both a timestamped transcript array with start/end times and a plain-text version in transcript_only_text. Supports specifying a language code. Note: the video must be under 2 minutes for transcript extraction to work.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
scrape-creators-pp-cli account list

# JSON for scripting and agents
scrape-creators-pp-cli account list --json

# Filter to specific fields
scrape-creators-pp-cli account list --json --select id,name,status

# Dry run — show the request without sending
scrape-creators-pp-cli account list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
scrape-creators-pp-cli account list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Retryable** - creates return "already exists" on retry, deletes return "already deleted"
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - `echo '{"key":"value"}' | scrape-creators-pp-cli <resource> create --stdin`
- **Cacheable** - GET responses cached for 5 minutes, bypass with `--no-cache`
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set
- **Progress events** - paginated commands emit NDJSON events to stderr in default mode

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use as MCP Server

This CLI ships a companion MCP server for use with Claude Desktop, Cursor, and other MCP-compatible tools.

### Claude Code

```bash
claude mcp add scrape-creators scrape-creators-pp-mcp -e SCRAPE_CREATORS_API_KEY_AUTH=<your-key>
```

### Claude Desktop

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "scrape-creators": {
      "command": "scrape-creators-pp-mcp",
      "env": {
        "SCRAPE_CREATORS_API_KEY_AUTH": "<your-key>"
      }
    }
  }
}
```

## Cookbook

Common workflows and recipes:

```bash
# List resources as JSON for scripting
scrape-creators-pp-cli account list --json

# Filter to specific fields
scrape-creators-pp-cli account list --json --select id,name,status

# Dry run to preview the request
scrape-creators-pp-cli account list --dry-run

# Sync data locally for offline search
scrape-creators-pp-cli sync

# Search synced data
scrape-creators-pp-cli search "query"

# Export for backup
scrape-creators-pp-cli export --format jsonl > backup.jsonl
```

## Health Check

```bash
scrape-creators-pp-cli doctor
```

<!-- DOCTOR_OUTPUT -->

## Configuration

Config file: `~/.config/scrape-creators-pp-cli/config.toml`

Environment variables:
- `SCRAPE_CREATORS_API_KEY_AUTH`

## Troubleshooting

**Authentication errors (exit code 4)**
- Run `scrape-creators-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SCRAPE_CREATORS_API_KEY_AUTH`

**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

**Rate limit errors (exit code 7)**
- The CLI auto-retries with exponential backoff
- If persistent, wait a few minutes and try again

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**n8n-nodes-scrape-creators**](https://github.com/adrianhorning08/n8n-nodes-scrape-creators) — JavaScript
- [**scrape-creators-examples**](https://github.com/adrianhorning08/scrape-creators-examples) — JavaScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
