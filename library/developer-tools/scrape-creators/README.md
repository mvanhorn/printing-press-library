# Scrape Creators CLI

**Every Scrape Creators endpoint across 28 platforms, plus a local store with offline transcript search, cross-platform joins, and ad-library diffing no other Scrape Creators tool ships.**

Scrape Creators exposes 164 read-only endpoints across TikTok, Instagram, YouTube, Facebook, LinkedIn, GitHub, Spotify, and 21 more platforms behind one API key. This CLI mirrors all of them as typed commands and MCP tools, then adds the layer the official CLI lacks: a local SQLite store with FTS5 so paid transcripts, profiles, and ad creatives become a queryable, diffable corpus. Cross-platform commands like 'creator find', 'trends triangulate', and 'ads monitor' answer questions no single endpoint can.

Learn more at [Scrape Creators](https://scrapecreators.com).

Created by [@adrianhorning08](https://github.com/adrianhorning08) (Adrian Horning).
Contributors: [@tmchow](https://github.com/tmchow) (Trevin Chow).

## Install

The recommended path installs both the `scrape-creators-pp-cli` binary and the `pp-scrape-creators` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install scrape-creators
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install scrape-creators --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install scrape-creators --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install scrape-creators --agent claude-code
npx -y @mvanhorn/printing-press-library install scrape-creators --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/cmd/scrape-creators-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/scrape-creators-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install scrape-creators --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-scrape-creators --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-scrape-creators --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install scrape-creators --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/scrape-creators-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SCRAPECREATORS_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/cmd/scrape-creators-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "scrape-creators": {
      "command": "scrape-creators-pp-mcp",
      "env": {
        "SCRAPECREATORS_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Authentication is a single Scrape Creators API key sent in the x-api-key header. Set SCRAPECREATORS_API_KEY in your environment, or run 'auth login' to store it. Credits are pay-as-you-go and never expire; a depleted balance returns HTTP 402, so check 'account budget' before a large sync.

## Quick Start

```bash
# health check — verifies the API key and reachability without spending a credit
scrape-creators-pp-cli doctor


# check your credit balance before fetching
scrape-creators-pp-cli account list --json


# find which of the major platforms a creator is on in one call
scrape-creators-pp-cli creator find mrbeast --agent


# see which platform a topic is biggest on
scrape-creators-pp-cli trends triangulate "stanley cup" --agent


# full-text search transcripts you have cached locally (run the per-platform transcript commands first to populate)
scrape-creators-pp-cli transcripts search "giveaway" --agent --select creator,platform,snippet

```

## Unique Features

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

## Usage

Run `scrape-creators-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `SCRAPE_CREATORS_CONFIG_DIR`, `SCRAPE_CREATORS_DATA_DIR`, `SCRAPE_CREATORS_STATE_DIR`, or `SCRAPE_CREATORS_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `SCRAPE_CREATORS_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export SCRAPE_CREATORS_HOME=/srv/scrape-creators
scrape-creators-pp-cli doctor
```

Under `SCRAPE_CREATORS_HOME=/srv/scrape-creators`, the four dirs resolve to `/srv/scrape-creators/config`, `/srv/scrape-creators/data`, `/srv/scrape-creators/state`, and `/srv/scrape-creators/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

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

Precedence matters in fleets: an ambient per-kind variable such as `SCRAPE_CREATORS_DATA_DIR` overrides an explicit `--home` for that kind. Use `SCRAPE_CREATORS_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `SCRAPE_CREATORS_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `scrape-creators-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### account

Manage account

- **`scrape-creators-pp-cli account list`** - Returns the number of API credits remaining on your Scrape Creators account. The response contains a single creditCount field with your current balance.
- **`scrape-creators-pp-cli account list-getapiusage`** - Returns a paginated list of your API requests, including the endpoint called, status code, credits used, and timestamp. Useful for debugging and monitoring your API usage. Supports filtering by endpoint name and status code.
- **`scrape-creators-pp-cli account list-getdailyusagecount`** - Returns aggregated daily usage statistics for the last 30 days, including total credits consumed and number of requests per day.
- **`scrape-creators-pp-cli account list-getmostusedroutes`** - Returns your top 20 most called API endpoints ranked by call count, along with total credits consumed per endpoint. Defaults to the last 24 hours. Supports custom time ranges up to 1 year.

### amazon

Manage amazon

- **`scrape-creators-pp-cli amazon`** - Scrapes a creator's Amazon Shop page by URL, returning their storefront profile and product collections. Returns avatar, name, description, socials, and lists with title and itemCount. Also includes trendingPicks with price and discount, curations with title and postCount, and a pageToken for pagination.

### bluesky

Get Bluesky posts and profile info

- **`scrape-creators-pp-cli bluesky list`** - Fetches a single Bluesky post by URL, returning the post's record text, author info, embed content, replyCount, repostCount, likeCount, and quoteCount. Also includes a replies array with threaded reply posts.
- **`scrape-creators-pp-cli bluesky list-profile`** - Retrieves a Bluesky user's public profile including handle, displayName, avatar, description, followersCount, followsCount, postsCount, createdAt, and verification status. The associated field shows counts for lists, feed generators, and starter packs.
- **`scrape-creators-pp-cli bluesky list-user`** - Fetches a paginated feed of posts from a Bluesky user, returning each post's uri, record text, author info, embed content, replyCount, repostCount, likeCount, quoteCount, and indexedAt. Supports pagination via cursor. Use user_id (the 'did') instead of handle for faster response times.

### detect-age-gender

Manage detect age gender

- **`scrape-creators-pp-cli detect-age-gender`** - Uses AI to analyze a creator's profile photo and estimate their age and gender. Returns ageRange with low and high bounds, gender, and a confidence score for the gender prediction. The profile photo must contain a clear, visible face for accurate results.

### facebook

Get public Facebook profiles and posts

- **`scrape-creators-pp-cli facebook list`** - Get the events of a city. Check out this [link](https://www.facebook.com/events/explore/saint-petersburg-florida/111326725552547) for an example of where we are getting the data from.
- **`scrape-creators-pp-cli facebook list-adlibrary`** - Retrieves detailed information about a specific Facebook ad by its ID or URL. Returns adArchiveID, pageName, isActive, startDate, endDate, and a snapshot containing body, images, videos, display_format, link_url, and cta_text. For ads with multiple versions, the ad creative is found in the snapshot.cards array rather than snapshot.body.
- **`scrape-creators-pp-cli facebook list-adlibrary-2`** - Retrieves a transcript for a single Facebook Ad Library video ad by ID or URL. If Facebook exposes captions, those are used. Otherwise we try to transcribe the public video URL. Credits are only deducted when transcript is returned. If the ad has no video or no transcript is available, transcript will be null and no credit is charged.
- **`scrape-creators-pp-cli facebook list-adlibrary-3`** - Fetches all ads currently running for a specific company from the Meta Ad Library. Each ad includes ad_archive_id, page_name, is_active, publisher_platform, and a snapshot with body, images, videos, and display_format. Supports filtering by country, media_type, date range, and language with cursor-based pagination.
- **`scrape-creators-pp-cli facebook list-adlibrary-4`** - Searches the Meta Ad Library by keyword and returns matching ads. Each result includes ad_archive_id, page_name, is_active, publisher_platform, and a snapshot with body text, images, videos, and cta_text. Results cap around 1,500 via GET due to cursor size limits; switch to POST method with body params for larger result sets.
- **`scrape-creators-pp-cli facebook list-adlibrary-5`** - Searches for companies by name in the Meta Ad Library and returns their page IDs for use with other ad library endpoints. Each result includes page_id, name, category, likes, verification status, and Instagram details like ig_username and ig_followers.
- **`scrape-creators-pp-cli facebook list-event`** - Get a specific event by its URL or id
- **`scrape-creators-pp-cli facebook list-events`** - Search for events by name. You can take a look at the page from Facebook we are getting the data from [here](https://www.facebook.com/events/search/?q=dogs)
- **`scrape-creators-pp-cli facebook list-group`** - Fetches posts from a public Facebook group, limited to 3 posts per page due to API limitations. Each post includes id, text, url, reactionCount, commentCount, publishTime, videoDetails, and topComments. Supports sorting by TOP_POSTS, RECENT_ACTIVITY, CHRONOLOGICAL, or CHRONOLOGICAL_LISTINGS, with cursor-based pagination.
- **`scrape-creators-pp-cli facebook list-marketplace`** - Fetches details for a Facebook Marketplace item by item id or Marketplace item URL, including title, description, price, location, condition, photos, seller, and availability flags. Rental listings can include listing_date_text and availability_text from Facebook's Marketplace GraphQL response, for example 'Listed over a week ago' and 'Available now'. creation_time can still be null when Facebook does not expose an exact timestamp.
- **`scrape-creators-pp-cli facebook list-marketplace-2`** - Searches Facebook Marketplace listings by keyword and lat/lng. Supports pagination with the returned cursor. Pass the cursor value back as-is. When sort_by is creation_time_descend, Facebook can still return slightly different ordering between identical requests. For alerting/new-item workflows, scrape multiple pages and dedupe by listing id instead of relying on page 1 item order being identical every run.
- **`scrape-creators-pp-cli facebook list-marketplace-3`** - Searches Facebook Marketplace locations/cities and returns coordinates you can use with the Marketplace Search endpoint.
- **`scrape-creators-pp-cli facebook list-post`** - Retrieves a single public Facebook post or reel by URL. Returns post_id, like_count, comment_count, share_count, view_count, description, creation_time, and author details. For some reels, Facebook does not expose the same view count on the individual post page that it shows on the profile Reels grid. This value can be null or lower than the public Reels badge. If you need the public Reel badge count, call /v1/facebook/profile/reels with the author URL and match the reel by post_id. For video posts, includes video sd_url, hd_url, thumbnail, and length_in_second. Optionally fetches comments and transcript via get_comments and get_transcript parameters.
- **`scrape-creators-pp-cli facebook list-post-2`** - Fetches comments from a Facebook post or reel with cursor-based pagination. Each comment includes id, text, created_at, reply_count, reaction_count, and author details with name and profile_picture. Passing a feedback_id instead of a url significantly speeds up the request.
- **`scrape-creators-pp-cli facebook list-post-3`** - Extracts the transcript text from a Facebook video post or reel. Returns the transcript as a single text string with line breaks. Only works on videos under 2 minutes in length.
- **`scrape-creators-pp-cli facebook list-post-4`** - Get the replies to a comment.
- **`scrape-creators-pp-cli facebook list-profile`** - Retrieves public Facebook page details including category, address, email, phone, website, services, priceRange, rating, likeCount, and followerCount. Also returns adLibrary status with the page's ad activity and pageId. Optionally includes businessHours when get_business_hours is set to true. If Facebook shows an 18+ or private content gate, the response is still 200 and includes isPrivate: true. If the page is not found, the response is 404 with accountDoesNotExist: true and isPrivate: false.
- **`scrape-creators-pp-cli facebook list-profile-2`** - Get the events of a public Facebook page
- **`scrape-creators-pp-cli facebook list-profile-3`** - Fetches photos from a public Facebook page with pagination support. Each photo includes photo_id, accessibility_caption, viewer_image with uri, height, and width, plus a thumbnail and direct url. Pagination requires passing both next_page_id and cursor from the previous response.
- **`scrape-creators-pp-cli facebook list-profile-4`** - Returns publicly visible Facebook profile posts, limited to 3 posts per page due to API limitations. Each post includes id, text, url, reactionCount, commentCount, publishTime, videoDetails with sdUrl, hdUrl, and thumbnailUrl, plus topComments. Accepts either a url or pageId parameter, where pageId is faster.
- **`scrape-creators-pp-cli facebook list-profile-5`** - Fetches up to 10 reels per request from a public Facebook page. Each reel includes id, url, view_count, description, creation_time, video_url, thumbnail, play_time_in_ms, and music details. Pagination requires passing both next_page_id and cursor from the previous response.

### github

Scrape GitHub profiles, repositories, and public activity

- **`scrape-creators-pp-cli github list`** - Retrieves public metadata for one GitHub repository, including owner, description, language, stars, forks, topics, license, visibility, default branch, open issues, and timestamps. Pass a full GitHub repository url.
- **`scrape-creators-pp-cli github list-trending`** - Scrapes GitHub's public Trending developers page. Returns ranked developers with username, name, public profile URL, avatar, and the popular repository GitHub shows for that developer when available. Use language for paths like javascript or python and since for daily/weekly/monthly.
- **`scrape-creators-pp-cli github list-trending-2`** - Scrapes GitHub's public Trending repositories page. Returns ranked repositories with public URLs, descriptions, language, star/fork counts, stars for the selected range, and built-by users when GitHub shows them. Use language for paths like JavaScript or Python, since for daily/weekly/monthly, and spoken_language_code for GitHub's spoken language filter.
- **`scrape-creators-pp-cli github list-user`** - Retrieves public GitHub user details including name, bio, avatar, company, location, blog, follower counts, public repo counts, and account timestamps. Pass username, handle, or a full GitHub user url.
- **`scrape-creators-pp-cli github list-user-2`** - Retrieves GitHub profile contribution activity for a user from the public profile activity timeline. Defaults to the current year when year is not provided. Results come back one month at a time in the activity array. Pass cursor from the previous response to page backward through the year.
- **`scrape-creators-pp-cli github list-user-3`** - Retrieves the public GitHub contribution graph for a user and year, including total contributions and daily contribution counts/intensity. Pass github handle, or a full GitHub profile url. Defaults to the current year when year is not provided.
- **`scrape-creators-pp-cli github list-user-4`** - Retrieves public GitHub followers for a user. Each follower includes login, avatar, user URL, type, and GitHub IDs. Pass username, handle, or a full GitHub user url. Supports cursor pagination.
- **`scrape-creators-pp-cli github list-user-5`** - Retrieves public accounts followed by a GitHub user. Each account includes login, avatar, profile URL, type, and GitHub IDs. Pass username, handle, or a full GitHub profile url. Supports cursor pagination.
- **`scrape-creators-pp-cli github list-user-6`** - Searches public GitHub pull requests authored by a user using GitHub's public search index. Pass username, handle, or url. Optional since and until filters use YYYY-MM-DD created dates. Results include the PR title, repo, state, created_at, and url, sorted by newest created first.
- **`scrape-creators-pp-cli github list-user-7`** - Retrieves a user's public repositories with repo metadata like description, language, stars, forks, topics, license, visibility, default branch, and timestamps. Pass username, handle, or url. Supports pagination with cursor, plus GitHub's type, sort, and direction parameters.

### google

Scrape Google search results

- **`scrape-creators-pp-cli google list`** - Retrieves detailed information about a specific Google ad including advertiserId, creativeId, format, firstShown, lastShown, and overallImpressions. Returns creativeRegions, regionStats with per-region impression data, and variations with destinationUrl, headline, description, and imageUrl. Text extraction uses OCR, so accuracy may vary.
- **`scrape-creators-pp-cli google list-adlibrary`** - Searches the Google Ad Transparency Library for advertisers by name. Returns a list of matching advertisers with their name, advertiser_id, and region, plus a list of associated website domains. Use the returned advertiser_id to look up a company's ads. Defaults to US when region is not passed, so pass a 2-letter country code like AU or CA when searching for advertisers in another region.
- **`scrape-creators-pp-cli google list-company`** - Fetches public ads for a company from the Google Ad Transparency Library by domain or advertiser_id. Each ad includes advertiserId, creativeId, format, adUrl, advertiserName, domain, firstShown, and lastShown. Costs 25 credits per request when get_ad_details=true; without it, only advertiserId and creativeId are returned at 1 credit.
- **`scrape-creators-pp-cli google list-search`** - Performs a Google search and returns organic results with url, title, and description for each result. Supports an optional region parameter (2-letter country code) to get localized results from a specific country.

### instagram

Gets Instagram profiles, posts, and reels

- **`scrape-creators-pp-cli instagram list`** - Fetches a lightweight Instagram profile summary by user ID, returning username, full name, biography, profile picture URL, verification status, follower count, following count, media count, and account privacy and type. Ideal for quick lookups or enrichment when you already have the numeric user ID.
- **`scrape-creators-pp-cli instagram list-audio`** - Fetches the reels Instagram exposes for an audio page like instagram.com/reels/audio/{audio_id}/. Pass the audio_id from that URL. Use cursor from the previous response to request the next page when Instagram returns one.
- **`scrape-creators-pp-cli instagram list-media`** - Generates an AI-powered speech-to-text transcription for an Instagram video post or reel. The video must be under 2 minutes long. Returns a transcripts array with each item's shortcode and transcribed text; carousel posts produce one transcript per video slide. Expect 10-30 second response times, and null when no speech is detected.
- **`scrape-creators-pp-cli instagram list-post`** - Fetches detailed metadata for a single Instagram post or reel by shortcode or URL. Returns caption text, like count, comment count, video URL, video play count, video duration, display images, owner info, tagged users, and carousel sidecar children when applicable. Play counts are Instagram-only views and exclude cross-posted Facebook views.
- **`scrape-creators-pp-cli instagram list-post-2`** - Retrieves comments on a public Instagram post or reel. Each comment includes the comment text, creation timestamp, and commenter details such as username, user ID, verification status, and profile picture URL. Supports cursor-based pagination to load additional comment pages.
- **`scrape-creators-pp-cli instagram list-profile`** - Retrieves comprehensive public Instagram profile information including biography, bio links, follower and following counts, verification status, and profile picture URLs. Also returns recent timeline posts with engagement metrics such as likes, comments, and video view counts, plus a list of related profiles. Useful for account overview, audience analysis, or discovering similar creators.
- **`scrape-creators-pp-cli instagram list-reels`** - Fetches trending reels from Instagram's public instagram.com/reels page. Instagram only gives a small batch at a time and the results can overlap, so call this endpoint over and over when you want more. Each call should return new-ish results, but expect some duplicates because that is how Instagram's reels page behaves too. Returns `reels`, an array of reel objects with shortcode, URL, caption, media URLs, engagement counts when Instagram exposes them, and user info.
- **`scrape-creators-pp-cli instagram list-reels-2`** - Searches for Instagram reels matching a keyword or phrase via Google Search, bypassing Instagram's login-gated search. Returns a list of reels with shortcode, caption, thumbnail, video URL, play count, like count, comment count, video duration, owner details, location, and audio attribution info. Play counts are Instagram-only views and exclude cross-posted Facebook views. Supports page-based pagination for browsing additional results.
- **`scrape-creators-pp-cli instagram list-search`** - Finds public Instagram posts for a hashtag using Google Search, then returns post details such as caption, play count when available, like count, comment count, owner, and post time. Results depend on what Google has indexed, so this is best-effort and not a complete Instagram-native hashtag search. Pass media_type=reels if you only want reels. Use date_posted for recent posts and pass the returned cursor to fetch the next page.
- **`scrape-creators-pp-cli instagram list-search-2`** - Searches Google for public Instagram results matching a keyword or phrase, then returns matching public profiles. Profile-page matches are marked matched_from=profile. Reel/post caption matches are enriched into the creator profile and marked matched_from=caption. This is best-effort and depends on what Google has indexed; it is not a complete native Instagram profile search.
- **`scrape-creators-pp-cli instagram list-user`** - Returns the raw HTML embed snippet for an Instagram user's profile widget. The response contains a single html string that can be inserted into a webpage to render an embeddable Instagram profile card. Requires the user's handle as input.
- **`scrape-creators-pp-cli instagram list-user-2`** - Lists all story highlight albums for an Instagram user. Each highlight includes its ID, title, cover thumbnail URL, and owner info with username and profile picture. Accepts either a user_id or handle; providing user_id yields faster responses.
- **`scrape-creators-pp-cli instagram list-user-3`** - Returns a paginated list of a user's public Instagram reels (short-form videos). Each reel includes its shortcode, play count, like count, comment count, video versions with download URLs, thumbnail image, and owner info. Note that reel captions are not returned by this endpoint. Play counts are Instagram-only views and exclude cross-posted Facebook views. Supports cursor-based pagination via max_id; providing a user_id instead of a handle yields faster responses.
- **`scrape-creators-pp-cli instagram list-user-4`** - Returns a paginated feed of a user's public Instagram posts, including reels, photos, videos, and carousels. Each item includes media type, shortcode, caption text, like count, comment count, play count, video URLs, image URLs, and tagged users. Play counts reflect Instagram-only views and exclude cross-posted Facebook views. Supports cursor-based pagination via next_max_id for scrolling through the full timeline.
- **`scrape-creators-pp-cli instagram list-user-5`** - Fetches the full contents of a specific Instagram story highlight album by its ID. Returns the highlight's cover image, title, user info, and an items array containing each story with its media type, image or video URLs, dimensions, timestamp, and sticker/interactive element data. Useful for archiving or analyzing individual highlight reels.

### kick

Scrape Kick clips

- **`scrape-creators-pp-cli kick`** - Fetches detailed data for a Kick clip by URL, including video, metadata, and channel info. Returns clip id, title, clip_url, thumbnail_url, video_url, view_count, likes_count, duration, privacy status, and is_mature flag. Also includes category details (name, slug), creator info (username), and channel info (username, profile_picture).

### komi

Scrape Komi pages

- **`scrape-creators-pp-cli komi`** - Scrapes a Komi page by URL, extracting the creator's profile, social links, and featured content. Returns id, username, avatar, displayName, bio, and social accounts (instagram, tiktok, youtube, twitter, facebook, snapchat). Also includes links, an array of link and product objects each with id, url, title, type, thumbnail, and optional price and currency for products.

### kwai

Scrape Kwai profiles, posts, and user feeds

- **`scrape-creators-pp-cli kwai list`** - Fetches public Kwai post details including caption, media URLs, cover images, counts, author info, and music metadata. Uses Kwai's public web API endpoint, not HTML scraping.
- **`scrape-creators-pp-cli kwai list-profile`** - Fetches public Kwai profile data including username, bio, avatar, verification status, gender, and public counts. Uses Kwai's public web API endpoint, not HTML scraping.
- **`scrape-creators-pp-cli kwai list-user`** - Fetches a paginated list of public Kwai posts for a user, including captions, media URLs, covers, counts, author info, and the next cursor when more results are available. Uses Kwai's public web API endpoint, not HTML scraping.

### linkbio

Scrape Linkbio (lnk.bio) pages

- **`scrape-creators-pp-cli linkbio`** - Scrapes a Linkbio (lnk.bio) page by URL, extracting the creator's profile and all their links. Returns handle, id, social accounts (instagram, tiktok, youtube, twitter, whatsapp), email, website, and links — an array of link objects each with url and text.

### linkedin

Scrape LinkedIn

- **`scrape-creators-pp-cli linkedin list`** - Retrieves detailed information about a specific LinkedIn ad by URL. Returns id, description, headline, adType, advertiser, and targeting with language, location, and audience criteria. Also includes totalImpressions, impressionsByCountry, adDuration, startDate, and endDate.
- **`scrape-creators-pp-cli linkedin list-ads`** - Searches the LinkedIn Ad Library by company name, keyword, or companyId with optional country and date filters. Each ad includes id, description, headline, adType, advertiser, targeting details, image or video URLs, totalImpressions, and impressionsByCountry. Supports pagination via paginationToken.
- **`scrape-creators-pp-cli linkedin list-company`** - Fetches a LinkedIn company page with details including name, description, logo, cover image, slogan, location, headquarters, employee count (headcount/staff size), website, industry, company type, founded year, specialties, funding rounds with investors, featured employees, recent posts, and similar company pages.
- **`scrape-creators-pp-cli linkedin list-company-2`** - Retrieves paginated posts from a LinkedIn company page, including each post's URL, ID, publication date, and full text content. Supports page-based pagination up to a maximum of 7 pages due to a LinkedIn platform limitation.
- **`scrape-creators-pp-cli linkedin list-post`** - Fetches a single LinkedIn post or article, returning the title, headline, full description text, author info with follower count, publication date, like count (reactions), comment count, and individual comments. Also includes related articles from the same author in moreArticles.
- **`scrape-creators-pp-cli linkedin list-post-2`** - Fetches the transcript from a LinkedIn post video when LinkedIn exposes one publicly. Returns null with transcriptNotAvailable when the post has no transcript, and only deducts credits when a transcript is returned.
- **`scrape-creators-pp-cli linkedin list-profile`** - Retrieves a person's public LinkedIn profile data, including their name, photo, location, follower count (followers), about/bio summary, recent posts, work experience, education, articles, activity feed, publications, projects, recommendations, and similar profiles. Only returns publicly available information visible in an incognito browser.
- **`scrape-creators-pp-cli linkedin list-search`** - Finds public LinkedIn posts, feed updates, and Pulse articles by keyword using Google Search, then returns post details such as description, author, media, images, like count, comment count, and published date when LinkedIn exposes them publicly. Results depend on what Google has indexed, so this is best-effort and not a complete LinkedIn-native search. Use date_posted for recent posts and pass the returned cursor to fetch the next page.

### linkme

Get Linkme profile info

- **`scrape-creators-pp-cli linkme`** - Retrieves a Linkme profile by URL, including identity, social links, and contact details. Returns profile with id, firstName, username, bio, profileVisitCount, profileImage, verifiedAccount, and isAmbassador flag. Also includes infoLinks (email addresses) and webLinks, an array of categorized social platform links (Spotify, Instagram, YouTube, Twitter, Facebook, and more) each with linkValue and faceValue.

### linktree

Scrape Linktree pages

- **`scrape-creators-pp-cli linktree`** - Scrapes a Linktree page by URL, extracting the creator's profile and all their links. Returns id, username, profilePictureUrl, description, verticals, timezone, and links — an array of link objects each with id, type, title, and url. Also includes detected social accounts (instagram, tiktok, spotify, youtube, soundcloud, apple_music) and email_address.

### pillar

Scrape Pillar pages

- **`scrape-creators-pp-cli pillar`** - Scrapes a Pillar page by URL, extracting the creator's profile, social links, and products. Returns id, first_name, last_name, email, location, and social accounts (tiktok, spotify, twitter, youtube, facebook, linkedin, instagram, and more). Also includes links with click counts and products with title, price, description, and image.

### pinterest

Scrape Pinterest pins

- **`scrape-creators-pp-cli pinterest list`** - Fetches a paginated list of pins from a Pinterest board by URL, returning each pin's id, description, title, images, board info, pin_join annotations, and aggregated_pin_data. Supports pagination via cursor and a trim option for lighter responses.
- **`scrape-creators-pp-cli pinterest list-pin`** - Fetches detailed information about a single Pinterest pin by URL, returning title, description, link, dominantColor, originPinner, pinner, images at multiple resolutions (imageSpec_236x through imageSpec_orig), and pinJoin with visual annotations. Supports a trim option for lighter responses.
- **`scrape-creators-pp-cli pinterest list-search`** - Searches Pinterest for pins matching a query, returning results with id, url, title, description, images, link, domain, board info, and pinner details. Supports pagination via cursor and a trim option for lighter responses.
- **`scrape-creators-pp-cli pinterest list-user`** - Fetches a paginated list of boards for a Pinterest user, returning each board's name, url, description, pin_count, follower_count, owner info, cover_images, and created_at. Supports pagination via cursor and a trim option for lighter responses.

### reddit

Scrape Reddit posts and comments

- **`scrape-creators-pp-cli reddit list`** - Searches across all of Reddit for posts matching a query. Each post includes title, author, selftext, subreddit, score, ups, upvote_ratio, num_comments, created_utc, url, permalink, and is_video. Supports sort (relevance, new, top, comment_count), timeframe filtering, pagination via the after token, and a trim parameter for lighter responses.
- **`scrape-creators-pp-cli reddit list-post`** - Retrieves comments and post details from a Reddit post by URL. Returns the post with title, author, score, ups, upvote_ratio, num_comments, and created_utc, plus a comments array where each comment includes author, body, body_html, score, created_utc, parent_id, permalink, and nested replies. Supports cursor-based pagination for loading more comments and a trim parameter for lighter responses.
- **`scrape-creators-pp-cli reddit list-post-2`** - Gets the transcript from a Reddit video post or direct v.redd.it URL when Reddit exposes a VTT caption file. Returns the raw WebVTT in raw_vtt plus a parsed plain-text transcript. If Reddit does not expose captions for the video, transcript is null and transcriptNotAvailable is true.
- **`scrape-creators-pp-cli reddit list-subreddit`** - Fetches posts from a subreddit with sorting and filtering options. Each post includes title, author, selftext, score, ups, upvote_ratio, num_comments, created_utc, url, permalink, subreddit_subscribers, and is_video. Supports sort (best, hot, new, top, rising), timeframe filtering, pagination via the after token, and a trim parameter for lighter responses.
- **`scrape-creators-pp-cli reddit list-subreddit-2`** - Retrieves metadata about a subreddit by name or URL. The subreddit name must be case-sensitive. Returns display_name, description, subscribers, weekly_active_users, weekly_contributions, rules, icon_img, header_img, advertiser_category, submit_text, and created_at.
- **`scrape-creators-pp-cli reddit list-subreddit-3`** - Searches within a specific subreddit for posts, comments, and media matching a query. Returns posts with title, votes, num_comments, url, and created_at; comments with author, body, votes, and parent post info; and media with title, media_type, image dimensions, and gallery_count. Supports sort, timeframe filtering, and cursor-based pagination.

### rumble

Scrape Rumble search, videos, transcripts, and channel videos

- **`scrape-creators-pp-cli rumble list`** - Searches Rumble videos by keyword. Returns matching videos and shorts with title, URL, thumbnail, channel, published date, viewCountText, viewCountInt, and a numeric cursor for the next page.
- **`scrape-creators-pp-cli rumble list-channel`** - Gets videos from a Rumble channel by handle or URL. Returns channel metadata, videos, shorts, and a numeric cursor for the next page.
- **`scrape-creators-pp-cli rumble list-video`** - Gets Rumble video details by URL. Returns title, description, thumbnail, channel, publish date, view count, likes, dislikes, captions, and media metadata when available.
- **`scrape-creators-pp-cli rumble list-video-2`** - Gets all top level comments for a Rumble video by URL. Returns comment text, author, createdAt, createdAtText, likeCount, dislikeCount, and replyCount when comment bodies are public.
- **`scrape-creators-pp-cli rumble list-video-3`** - Gets a Rumble video's transcript when captions are available. If Rumble does not expose captions for the video, transcript will be null and you will not be charged.

### snapchat

Scrape Snapchat user profiles and thier stories

- **`scrape-creators-pp-cli snapchat`** - Retrieves a Snapchat user's public profile by handle, including identity, stories, and spotlight content. Returns userProfile with username, title, snapcodeImageUrl, subscriberCount, bio, and profilePictureUrl. Also includes highlightStoryMetadata with individual story snaps (mediaUrl, mediaType, thumbnailUrl) and spotlightStoryMetadata with video details and engagement stats (viewCount, shareCount, commentCount).

### soundcloud

Scrape SoundCloud playlists and tracks

- **`scrape-creators-pp-cli soundcloud list`** - Fetches detailed information about a SoundCloud artist by its handle or URL. Returns artist metadata including id, name, followers, etc
- **`scrape-creators-pp-cli soundcloud list-artist`** - Fetches tracks/songs for a SoundCloud artist by handle or URL. Returns the artist profile, track list, and pagination info from SoundCloud.
- **`scrape-creators-pp-cli soundcloud list-track`** - Fetches detailed information about a SoundCloud track/song by URL. Returns track title, plays, likes, reposts, comments, artwork, artist, and other SoundCloud metadata.

### spotify

Scrape Spotify artists, songs, and albums

- **`scrape-creators-pp-cli spotify list`** - Retrieves detailed information about a Spotify album by its id or URL, including album metadata, artists, release date, cover art, copyright info, tracks, and sharing details.
- **`scrape-creators-pp-cli spotify list-artist`** - Retrieves detailed information about a Spotify artist by their handle, including name, followers count, genres, and related artists. Accepts a handle as input and returns artist metadata such as id, name, followers, genres, and related artists.
- **`scrape-creators-pp-cli spotify list-podcast`** - Retrieves detailed information about a Spotify podcast by its id or URL. Spotify calls podcasts shows internally, so Spotify podcast URLs use /show/.
- **`scrape-creators-pp-cli spotify list-podcast-2`** - Returns episodes for a Spotify podcast. Pass the cursor returned by a response to get the next page.
- **`scrape-creators-pp-cli spotify list-search`** - Search Spotify for tracks, artists, albums, episodes, podcasts, and audiobooks. Use type=podcasts when you need podcast ids for the podcast endpoints.
- **`scrape-creators-pp-cli spotify list-track`** - Retrieves detailed information about a Spotify track by its id or URL, including track metadata, artists, album info, duration, playability, and sharing details.

### threads

Get Threads posts

- **`scrape-creators-pp-cli threads list`** - Fetches a single Threads post by URL, returning the post's caption, like_count, view_counts, reshare_count, direct_reply_count, image_versions2, and taken_at. Also includes comments and related_posts arrays. Supports a trim option for lighter responses.
- **`scrape-creators-pp-cli threads list-profile`** - Retrieves a Threads user's public profile including username, full_name, biography, profile_pic_url, follower_count, is_verified, bio_links, and hd_profile_pic_versions. Also indicates whether the account is a threads-only user via is_threads_only_user.
- **`scrape-creators-pp-cli threads list-search`** - Searches Threads for posts matching a keyword, returning up to 10 results with caption text, like_count, reshare_count, direct_reply_count, user info, and image_versions2. Supports optional start_date and end_date filters plus a trim option. Only 10 results are returned per request due to public API limitations.
- **`scrape-creators-pp-cli threads list-search-2`** - Searches for Threads users by username, returning matching profiles with username, full_name, profile_pic_url, is_verified, and pk. Useful for finding user accounts before fetching their profile or posts.
- **`scrape-creators-pp-cli threads list-user`** - Fetches the most recent posts from a Threads user, returning id, caption text, code, like_count, reshare_count, direct_reply_count, repost_count, image_versions2, video_versions, and taken_at. Only the last 20-30 posts are publicly visible. Supports a trim option for lighter responses.

### tiktok

Scrape TikTok profiles, videos, and more

- **`scrape-creators-pp-cli tiktok list`** - Fetches TikTok's trending/For You feed for a given region — useful for discovering viral content and what's currently popular. Returns `aweme_list`, an array of video objects each with `aweme_id`, `desc` (caption), `statistics` (play_count, digg_count/likes, comment_count, share_count, collect_count), `video` (playback and download URLs, cover), `author` info, and `image_post_info` for photo carousels.
- **`scrape-creators-pp-cli tiktok list-adlibrary`** - Fetches one TikTok Creative Center Top Ad by material/ad ID or URL. Uses the same Creative Center data behind pages like ads.tiktok.com/business/creativecenter/topads/{id}/pc/en, including title, metrics, video info, landing page, country codes, objective, industry, source, the Creative Center URL, summary/analysis, interactive time analysis graph data, and recommended-for-you ads when TikTok provides them.
- **`scrape-creators-pp-cli tiktok list-adlibrary-2`** - Searches TikTok Creative Center Top Ads, the ad library page at ads.tiktok.com/business/creativecenter/inspiration/topads. Supports US and other 2-letter regions, period filters, sorting, keyword search, and cursor pagination. Returns TikTok's public top ad material objects, including ad title, metrics, video info, landing page, and pagination.
- **`scrape-creators-pp-cli tiktok list-creators`** - Discovers trending and popular TikTok creators, filterable by follower count range, creator country, and audience country. Returns `creator_list`, an array of creator objects each with `nickname`, `unique_id`, `follower_count`, `likes_count`, `video_views`, `engagement_rate`, and avatar URLs. Sortable by engagement, follower count, or average views.
- **`scrape-creators-pp-cli tiktok list-hashtags`** - Discovers trending and popular TikTok hashtags, filterable by time period (7/30/120 days) and country. Returns a list of hashtag objects each with `hashtag_name`, `rank`, `trend` data, and related video examples. Useful for identifying viral topics and content trends on TikTok.
- **`scrape-creators-pp-cli tiktok list-live`** - Gets curated room-level info for a TikTok live using TokAPI's live info endpoint. Use `/v1/tiktok/user/live` first to find the `room_id`. If you only have a TikTok handle and need the user's numeric id, use `/v1/tiktok/profile` first to get `user.id`. This endpoint is separate from `/v1/tiktok/user/live` because it uses a different upstream call and returns a smaller response with the most relevant fields: `room_id`, `like_count`, `viewer_count`, `status`, `title`, `cover_url`, and `owner`.
- **`scrape-creators-pp-cli tiktok list-product`** - Fetches full details for a specific US TikTok Shop product by its URL, including stock levels and affiliate videos. Returns `product_info` with `product_base` (title, images, sold_count, price), `skus` (variants with exact `stock` counts), and `product_detail_review` (product_rating, review_count, sample reviews); also returns `shop_info` (shop_name, shop_rating, followers_count) and `related_videos` (affiliate TikToks promoting the product). This endpoint currently supports the US region only.
- **`scrape-creators-pp-cli tiktok list-profile`** - Fetches public profile data for a TikTok user by their handle or user_id — useful for looking up a creator's identity, bio, and account stats. Returns a `user` object (display name, avatar URLs, bio/signature, verification status, bio link) and a `stats` object (followerCount, followingCount, heartCount/total likes, videoCount). This only returns profile metadata, not the user's actual videos or followers list.
- **`scrape-creators-pp-cli tiktok list-profile-2`** - Returns the TikTok region code for a public profile, like `US` for United States or `MX` for Mexico.
- **`scrape-creators-pp-cli tiktok list-profile-3`** - Fetches videos posted by a TikTok user, sortable by latest or most popular — use this to get a creator's video feed or TikToks. Returns `aweme_list`, an array of video objects each containing `aweme_id`, `desc` (caption), `statistics` (play_count, digg_count/likes, comment_count, share_count, collect_count/saves), and `video` (download URLs, duration, cover image). Paginate with `max_cursor` from the previous response.
- **`scrape-creators-pp-cli tiktok list-search`** - Searches for TikTok videos under a specific hashtag — useful for finding content by topic or trend. Returns `aweme_list`, an array of video objects each with `aweme_id`, `desc` (caption), `statistics` (play_count, digg_count/likes, comment_count, share_count), `video` info, and `author` details. Paginate with `cursor` from the previous response. TikTok may return duplicate results.
- **`scrape-creators-pp-cli tiktok list-search-2`** - Searches for TikTok videos by keyword or phrase — the general video search across all of TikTok. Returns `search_item_list`, an array of objects each containing `aweme_info` with `aweme_id`, `desc` (caption), `statistics` (play_count, digg_count/likes, comment_count, share_count), `video` info, and `author` details. Paginate with `cursor`. TikTok may return duplicate results.
- **`scrape-creators-pp-cli tiktok list-search-3`** - Gets the autocomplete suggestions TikTok shows while someone is typing in search. Returns `suggestions`, a clean array of suggested search terms and the most useful metadata for each suggestion.
- **`scrape-creators-pp-cli tiktok list-search-4`** - Searches TikTok's 'Top' results by query — returns both videos and photo carousels, unlike keyword search which only returns videos. Returns `items`, an array of objects each with `id`, `desc` (caption), `content_type` (video or photo carousel), `statistics` (play_count, digg_count/likes, comment_count, share_count), `video` info, and `images` for carousels. Paginate with `cursor`. TikTok may return duplicate results.
- **`scrape-creators-pp-cli tiktok list-search-5`** - Searches for TikTok users by keyword or name — useful for finding creators or accounts matching a query. Returns `users`, an array of objects each containing `user_info` (nickname, unique_id, signature/bio, follower_count, following_count, avatar) and associated `items`. Paginate with `cursor` from the previous response.
- **`scrape-creators-pp-cli tiktok list-shop`** - Lists all products from a specific TikTok Shop store by its URL. Returns an array of product objects each with `title`, `cover` images, `url`, `price` info, `sold_count`, `review_count`, and `rating`. Paginate with `cursor` from the previous response; filter by region; use `sort_by=top` for best-selling products or `sort_by=new_releases` for newest products. Non-US shop catalog coverage depends on TikTok exposing that shop in the selected region, so some shops can return `not_found` outside the US even when they appear in shop search.
- **`scrape-creators-pp-cli tiktok list-shop-2`** - Searches TikTok Shop for products matching a keyword query. Returns an array of product objects each with `title`, `cover` image, `url` (product page link), `price`, `sold_count`, `review_count`, `rating`, and `shop_name`. Paginate with `page`; filter by region.
- **`scrape-creators-pp-cli tiktok list-shop-3`** - Fetches customer reviews for a TikTok Shop product by URL or product_id. Returns `product_reviews`, an array of review objects each with `rating`, `display_text`, `review_timestamp_fmt`, `review_user` (name, avatar), and `sku_specification` (variant purchased); also returns `total_reviews` count and `rating_distribution`. Paginate with `page`.
- **`scrape-creators-pp-cli tiktok list-song`** - Fetches detailed metadata for a specific TikTok sound or song by its clipId. Returns `music_info` with `title`, `author`, `album`, `duration`, `user_count` (number of videos using this sound), `play_url`, cover art, and artist details. Use the `clipId` from a sound URL or from the popular songs endpoint.
- **`scrape-creators-pp-cli tiktok list-song-2`** - Fetches TikTok videos that use a specific sound or song, identified by its clipId. Returns `aweme_list`, an array of video objects each with `aweme_id`, `desc` (caption), `statistics` (play_count, digg_count/likes, comment_count, share_count), `video` info, and `author` details. Paginate with `cursor` from the previous response.
- **`scrape-creators-pp-cli tiktok list-user`** - Retrieves audience demographic data for a TikTok user, showing where their followers are located by country. Returns `audienceLocations`, an array of objects each containing `country`, `countryCode`, `count`, and `percentage`. Costs 26 credits per request.
- **`scrape-creators-pp-cli tiktok list-user-2`** - Retrieves the follower list of a TikTok account by handle or user_id — useful for seeing who follows a creator or getting subscriber data. Returns `followers`, an array of user objects each with `nickname`, `unique_id`, `uid`, `follower_count`, `following_count`, and avatar URLs; also returns `total` follower count. Paginate with `min_time` from the previous response.
- **`scrape-creators-pp-cli tiktok list-user-3`** - Retrieves the following list — accounts that a TikTok user follows — by their handle. Returns `followings`, an array of user objects each with `nickname`, `unique_id`, `uid`, `follower_count`, `following_count`, `signature`, and avatar URLs; also returns `total` count. Paginate with `min_time` from the previous response.
- **`scrape-creators-pp-cli tiktok list-user-4`** - Checks if a TikTok user is currently live streaming and retrieves their live room details. Returns `liveRoomUserInfo` (nickname, avatar, followerCount, roomId) and `liveRoom` (title, startTime, status, `liveRoomStats` with enterCount and userCount, plus `streamData` with playback URLs in multiple qualities).
- **`scrape-creators-pp-cli tiktok list-user-5`** - Fetches products featured in a TikTok user's public showcase — the products a creator promotes on their profile. Returns an array of product objects each with title, price, images, and shop details. Use POST request if pagination is cutting off too early. Just send the query params in the body.
- **`scrape-creators-pp-cli tiktok list-video`** - Fetches detailed data for a single TikTok video by URL, including its metadata, engagement stats, and optionally its transcript/captions. Returns `aweme_detail` with `desc` (caption), `statistics` (play_count, digg_count/likes, comment_count, share_count, collect_count), `video` URLs, `author` info, and `music` info; also returns `transcript` in WEBVTT format if `get_transcript=true`. For no-watermark video URLs, use `aweme_detail.video.download_no_watermark_addr.url_list[0]` when it exists. If it is missing and `aweme_detail.video.has_watermark` is false, use `aweme_detail.video.play_addr.url_list[0]` instead. If `has_watermark` is true and `download_no_watermark_addr` is missing, TikTok did not return a no-watermark URL for that video.
- **`scrape-creators-pp-cli tiktok list-video-2`** - Fetches comments on a TikTok video by URL — useful for reading audience reactions, replies, and engagement. Returns `comments`, an array where each comment includes `text`, `digg_count` (likes), `reply_comment_total`, `create_time`, and a `user` object with the commenter's nickname and unique_id; also returns `total` comment count. Paginate with `cursor` from the previous response.
- **`scrape-creators-pp-cli tiktok list-video-3`** - Extracts the transcript, captions, or subtitles from a TikTok video by URL. Returns `id`, `url`, and `transcript` as a WEBVTT-formatted string with timestamped text segments. Video must be under 2 minutes; costs an additional 10 credits when `use_ai_as_fallback=true`.
- **`scrape-creators-pp-cli tiktok list-video-4`** - Fetches replies to a specific TikTok comment by its ID. Returns `comments`, an array of comment objects each with `text`, `user` info, and `create_time`. Paginate with `cursor` from the previous response.

### truthsocial

Manage truthsocial

- **`scrape-creators-pp-cli truthsocial list`** - Fetches a single Truth Social post by URL, returning text, id, created_at, url, content, account details, media_attachments, card link previews, replies_count, reblogs_count, and favourites_count. Only posts from prominent public figures (e.g., Trump, Vance) are accessible without authentication.
- **`scrape-creators-pp-cli truthsocial list-profile`** - Retrieves a Truth Social user's public profile including display_name, username, avatar, header, followers_count, following_count, statuses_count, verified status, website, and created_at. Only prominent public figures (e.g., Trump, Vance) are accessible without authentication; most other accounts will not work.
- **`scrape-creators-pp-cli truthsocial list-user`** - Fetches a paginated list of posts from a Truth Social user, returning text, id, created_at, url, content, account info, media_attachments, card link previews, replies_count, reblogs_count, and favourites_count. Supports pagination via next_max_id and a trim option for lighter responses. Only prominent public figures (e.g., Trump, Vance) are accessible without authentication.

### twitch

Scrape Twitch clips

- **`scrape-creators-pp-cli twitch list`** - Fetches detailed data for a Twitch clip by URL, including metadata and direct video URLs. Returns clip id, slug, url, embedURL, title, viewCount, language, durationSeconds, game info, broadcaster details with follower count, thumbnailURL, and videoQualities at multiple resolutions with a signed videoURL for playback. Also includes additional clips from the same broadcaster.
- **`scrape-creators-pp-cli twitch list-profile`** - Retrieves a Twitch user's public profile by handle, including identity, social links, and content. Returns id, handle, displayName, description, followers count, and linked social accounts (instagram, x, tiktok). Also includes allVideos with game info, duration, and view counts, featuredClips with clip metadata and thumbnails, and similarStreamers.
- **`scrape-creators-pp-cli twitch list-user`** - Fetches a user's schedule by handle, returning a list of scheduled events with start time, end time, title, description, and thumbnail URL. Supports pagination via cursor and a trim option for lighter responses.
- **`scrape-creators-pp-cli twitch list-user-2`** - Fetches a list of videos (100 max) for a Twitch user, returning each video's id, slug, url, embedURL, title, viewCount, language, durationSeconds, game info, broadcaster details with follower count, thumbnailURL, and videoQualities at multiple resolutions with a signed videoURL for playback. Supports pagination via cursor and a trim option for lighter responses.

### twitter

Get Twitter profiles, tweets, followers and more

- **`scrape-creators-pp-cli twitter list`** - Retrieves details about a Twitter/X Community by URL. Returns the community name, description, rest_id, join_policy, created_at, member_count, rules, and creator_results with the creator's profile. Also includes members_facepile_results with avatar images of recent members.
- **`scrape-creators-pp-cli twitter list-community`** - Fetches tweets posted within a Twitter/X Community by URL. Returns an array of tweets, each with id, full_text, view_count, favorite_count, retweet_count, reply_count, bookmark_count, quote_count, created_at, and source. Each tweet includes a user object with the author's name, screen_name, avatar, followers_count, and is_blue_verified status.
- **`scrape-creators-pp-cli twitter list-profile`** - Retrieves a Twitter user's profile by handle, including account metadata and statistics. Returns name, screen_name, description, followers_count, friends_count, statuses_count, favourites_count, location, profile_image_url_https, and is_blue_verified. Also includes verification_info, tipjar_settings, highlights_info, and creator_subscriptions_count.
- **`scrape-creators-pp-cli twitter list-tweet`** - Retrieves detailed information about a specific tweet by URL, including the author's profile and engagement metrics. Returns rest_id, full_text, views count, favorite_count, retweet_count, reply_count, bookmark_count, quote_count, created_at, source, and media entities. Supports a trim parameter for a lighter response.
- **`scrape-creators-pp-cli twitter list-tweet-2`** - Extracts the transcript from a Twitter video tweet using AI-powered transcription. The video must be under 2 minutes long. Returns a success flag and the full transcript text. This endpoint is slower than others due to the AI processing step.
- **`scrape-creators-pp-cli twitter list-usertweets`** - Fetches tweets from a Twitter user's profile by handle. Note: Twitter publicly returns only ~100 of the user's most popular tweets, not chronological or latest. Each tweet includes rest_id, full_text, views count, favorite_count, retweet_count, reply_count, bookmark_count, quote_count, created_at, media entities, and url. Supports a trim parameter for a lighter response.

### youtube

Scrape YouTube channels, videos, and more

- **`scrape-creators-pp-cli youtube list`** - Retrieves comprehensive YouTube channel profile data including name, avatar images, subscriber count (subscribers), total video and view counts, join date, tags, and linked social accounts like Twitter and Instagram. Accepts a channelId, handle, or full channel URL as input. Returns channel metadata such as country, email, and external store links when available.
- **`scrape-creators-pp-cli youtube list-channel`** - Fetches community posts from a YouTube channel's Posts tab, including post ID, URL, content, images, attached video, like count, publish time, channel info, and a continuationToken when YouTube has more results. Pass a handle or channelId for the first page, then pass continuationToken to page through more posts.
- **`scrape-creators-pp-cli youtube list-channel-2`** - Fetches live streams and past streams from a YouTube channel's Live tab, including title, URL, thumbnail, view count, publish time, duration, and a continuationToken when YouTube has more results. Pass a handle or channelId for the first page, then pass continuationToken to page through more lives.
- **`scrape-creators-pp-cli youtube list-channel-3`** - Fetches playlists from a YouTube channel's Playlists tab, including playlist ID, title, thumbnail, video count, channel info, playlist URL, and a continuationToken when YouTube has more results. Pass a handle or channelId for the first page, then pass continuationToken to page through more playlists.
- **`scrape-creators-pp-cli youtube list-channel-4`** - Retrieves a paginated list of short-form videos (Shorts) from a YouTube channel, including each short's title, URL, view count (views), likes, comments, and description. Supports sorting by newest or popular, and use the continuationToken to page through all results. Returns data in the shorts array.
- **`scrape-creators-pp-cli youtube list-channelvideos`** - Fetches a paginated list of videos uploaded by a YouTube channel, including each video's title, URL, thumbnail, view count (views), publish date, duration, and description. Supports sorting by latest or popular, and use the continuationToken to page through all results. Optionally include extras like like count, comment count, and descriptions for each video.
- **`scrape-creators-pp-cli youtube list-communitypost`** - Retrieves the full details of a YouTube community post, including its text content, attached images, like count, publish date, and associated channel info. Also returns a linked video if the post includes one.
- **`scrape-creators-pp-cli youtube list-playlist`** - Retrieves all videos in a YouTube playlist, including the playlist title, owner info, total video count, and each video's title, URL, thumbnail, duration, and channel. Accepts the playlist ID found in the 'list' URL parameter.
- **`scrape-creators-pp-cli youtube list-search`** - Searches YouTube by keyword query and returns matching videos, channels, playlists, shorts, shelves, and live streams. Each video result includes title, URL, thumbnail, view count (views), publish date, duration, channel info, and badges. Supports filtering by upload date, sorting by relevance or popularity, and paginating with continuationToken. Set is_paid_promotions=true to search YouTube videos with the paid product placement / sponsorship disclosure.
- **`scrape-creators-pp-cli youtube list-search-2`** - Searches YouTube for content matching a specific hashtag and returns matching videos with title, URL, thumbnail, view count (views), publish date, duration, and channel info. Supports pagination via continuationToken and filtering to return all content types or only shorts.
- **`scrape-creators-pp-cli youtube list-shorts`** - Fetches approximately 48 currently trending YouTube Shorts (viral/popular short-form videos) per call, returning each short's title, URL, thumbnail, view count (views), like count (likes), comment count, publish date, channel info, keywords, and duration. Each subsequent call returns a fresh batch of different trending shorts.
- **`scrape-creators-pp-cli youtube list-video`** - Fetches full details for a YouTube video or short, including title, description, thumbnail, view count (views), like count (likes), comment count, publish date, duration, genre, keywords, chapters, collaborators, and available caption tracks (subtitles/captions). Also returns related recommended videos in watchNextVideos and channel info for the uploader.
- **`scrape-creators-pp-cli youtube list-video-2`** - Fetches comments and replies from a YouTube video, including each comment's text content, author details, like count, reply count, and publish date. Supports ordering by top or newest, and paginating with continuationToken. Limited to approximately 1,000 top comments or 7,000 newest comments.
- **`scrape-creators-pp-cli youtube list-video-3`** - Experimental endpoint. Checks a YouTube video for the paid-promotion disclosure and infers likely sponsors/promoted brands from the public description, description links, promo-code text, and transcript. YouTube tells us that a video contains paid promotion, but it does not always tell us the sponsor directly, so this endpoint returns suspected sponsors with confidence and evidence. This is inferred, not an official YouTube sponsor field. Feedback welcome: support@scrapecreators.com
- **`scrape-creators-pp-cli youtube list-video-4`** - Retrieves the captions, subtitles, or transcript of a YouTube video or short. Returns both a timestamped transcript array with start/end times and a plain-text version in transcript_only_text. Supports specifying a language code. Note: the video must be under 2 minutes for transcript extraction to work.
- **`scrape-creators-pp-cli youtube list-video-5`** - Fetches replies to a specific comment on a YouTube video, including each reply's text content, author details (name, channel ID, avatar, verified/creator status), like count, and publish date. Requires a continuationToken obtained from the 'repliesContinuationToken' field on comments returned by the Comments endpoint. Supports paginating through additional replies with the continuationToken returned in each response.


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
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
scrape-creators-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `scrape-creators-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/scrape-creators-pp-cli/config.toml`; `--home`, `SCRAPE_CREATORS_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SCRAPECREATORS_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `scrape-creators-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `scrape-creators-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SCRAPECREATORS_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **HTTP 402 / payment required mid-command** — Your credit balance is depleted. Top up at scrapecreators.com, then check runway with 'account budget'.
- **Commands time out on transcript or large-feed endpoints** — The API runs on AWS Lambda with a hard 29s ceiling; narrow the request (single handle, --limit) or retry — transcripts are capped at source videos under 2 minutes.
- **A single platform's endpoints return errors while others work** — Scrape Creators occasionally has per-platform incidents from upstream changes; check scrapecreators.com/status and retry that platform later.
- **transcripts search returns nothing** — The local store has no transcripts yet. Run the per-platform transcript commands (e.g. youtube list-transcript, tiktok list-transcript) — they cache results locally — then search.

---

## Known Gaps

- **A few endpoints have no runnable `--help` example.** Endpoints whose only required input is a response-derived pagination token (`continuationToken`, `feedback_id`, `expansion_token` — e.g. `youtube list-video-5`, `facebook list-post-4`) cannot ship a stable example, since the token comes from a prior call and expires. Fetch the token from the parent command's output, then pass it. Every other endpoint's `--help` example is runnable as shown.
- **The API is credit-metered.** Every request costs one Scrape Creators credit and a depleted balance returns HTTP 402. Run `account budget` to project runway. The local store is populated by your reads (and cached), not by a pre-read auto-refresh, so credits are only spent on commands you actually run.
- **A few platform endpoints depend on upstream availability.** TikTok Creative Center surfaces (`tiktok list-creators`, `tiktok list-hashtags`, the TikTok ad library) periodically return errors when TikTok's own Creative Center is down — the CLI surfaces the upstream message verbatim.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**n8n-nodes-scrape-creators**](https://github.com/adrianhorning08/n8n-nodes-scrape-creators) — TypeScript (19 stars)
- [**scrapecreators-cli**](https://github.com/ScrapeCreators/scrapecreators-cli) — JavaScript (4 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
