# Scrape Creators CLI

**Every Scrape Creators endpoint, plus offline search, cross-platform compounding, and a local store no other Scrape Creators tool ships with.**

Wrap all 114 Scrape Creators endpoints across 23 platforms — TikTok, Instagram, YouTube, Facebook, LinkedIn, Twitter, Reddit, Threads, Bluesky, Pinterest, and more — and add a SQLite-backed store with full-text transcript search, cross-platform presence checks, ad-library unified search, and credit-burn projection. Read-only across the board, agent-native by default, every command a typed MCP tool.

Learn more at [Scrape Creators](https://scrapecreators.com).

## Install

### Go

```
go install github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/cmd/scrape-creators-pp-cli@latest
```

### Binary

Download from [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/scrape-creators-current).

## Authentication

Set SCRAPE_CREATORS_API_KEY (get one at https://scrapecreators.com). One header, no OAuth handshake.

## Quick Start

```bash
# Verify auth and see remaining credits.
scrape-creators-pp-cli account balance --json


# Cross-platform presence — see every platform a creator is on.
scrape-creators-pp-cli creator find mrbeast --json


# Pull a single profile (any of 23 platforms supported).
scrape-creators-pp-cli tiktok profile mrbeast --json


# Persist tiktok resources to local SQLite (use --resources for any platform).
scrape-creators-pp-cli sync --resources tiktok


# FTS5 across every synced transcript, offline.
scrape-creators-pp-cli transcripts search "giveaway" --json


# Unified Facebook + Google + LinkedIn ad library search.
scrape-creators-pp-cli ads search "Liquid Death" --json

```

## Unique Features

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

## Usage

Run `scrape-creators-pp-cli --help` for the full command reference and flag list.

## Commands

### account

Manage account

- **`scrape-creators-pp-cli account list`** - Returns the number of API credits remaining on your Scrape Creators account. The response contains a single creditCount field with your current balance.
- **`scrape-creators-pp-cli account list-getapiusage`** - Returns a paginated list of your API requests, including the endpoint called, status code, credits used, and timestamp. Useful for debugging and monitoring your API usage. Supports filtering by endpoint name and status code.
- **`scrape-creators-pp-cli account list-getdailyusagecount`** - Returns aggregated daily usage statistics for the last 30 days, including total credits consumed and number of requests per day.
- **`scrape-creators-pp-cli account list-getmostusedroutes`** - Returns your top 20 most called API endpoints ranked by call count, along with total credits consumed per endpoint. Defaults to the last 24 hours. Supports custom time ranges up to 1 year.

### amazon

Manage amazon

- **`scrape-creators-pp-cli amazon list`** - Scrapes a creator's Amazon Shop page by URL, returning their storefront profile and product collections. Returns avatar, name, description, socials, and lists with title and itemCount. Also includes trendingPicks with price and discount, curations with title and postCount, and a pageToken for pagination.

### bluesky

Get Bluesky posts and profile info

- **`scrape-creators-pp-cli bluesky list`** - Fetches a single Bluesky post by URL, returning the post's record text, author info, embed content, replyCount, repostCount, likeCount, and quoteCount. Also includes a replies array with threaded reply posts.
- **`scrape-creators-pp-cli bluesky list-profile`** - Retrieves a Bluesky user's public profile including handle, displayName, avatar, description, followersCount, followsCount, postsCount, createdAt, and verification status. The associated field shows counts for lists, feed generators, and starter packs.
- **`scrape-creators-pp-cli bluesky list-user`** - Fetches a paginated feed of posts from a Bluesky user, returning each post's uri, record text, author info, embed content, replyCount, repostCount, likeCount, quoteCount, and indexedAt. Supports pagination via cursor. Use user_id (the 'did') instead of handle for faster response times.

### detect-age-gender

Manage detect age gender

- **`scrape-creators-pp-cli detect-age-gender list`** - Uses AI to analyze a creator's profile photo and estimate their age and gender. Returns ageRange with low and high bounds, gender, and a confidence score for the gender prediction. The profile photo must contain a clear, visible face for accurate results.

### facebook

Get public Facebook profiles and posts

- **`scrape-creators-pp-cli facebook list`** - Retrieves a single public Facebook post or reel by URL. Returns post_id, like_count, comment_count, share_count, view_count, description, creation_time, and author details. For video posts, includes video sd_url, hd_url, thumbnail, and length_in_second. Optionally fetches comments and transcript via get_comments and get_transcript parameters.
- **`scrape-creators-pp-cli facebook list-adlibrary`** - Retrieves detailed information about a specific Facebook ad by its ID or URL. Returns adArchiveID, pageName, isActive, startDate, endDate, and a snapshot containing body, images, videos, display_format, link_url, and cta_text. For ads with multiple versions, the ad creative is found in the snapshot.cards array rather than snapshot.body.
- **`scrape-creators-pp-cli facebook list-adlibrary-2`** - Fetches all ads currently running for a specific company from the Meta Ad Library. Each ad includes ad_archive_id, page_name, is_active, publisher_platform, and a snapshot with body, images, videos, and display_format. Supports filtering by country, media_type, date range, and language with cursor-based pagination.
- **`scrape-creators-pp-cli facebook list-adlibrary-3`** - Searches the Meta Ad Library by keyword and returns matching ads. Each result includes ad_archive_id, page_name, is_active, publisher_platform, and a snapshot with body text, images, videos, and cta_text. Results cap around 1,500 via GET due to cursor size limits; switch to POST method with body params for larger result sets.
- **`scrape-creators-pp-cli facebook list-adlibrary-4`** - Searches for companies by name in the Meta Ad Library and returns their page IDs for use with other ad library endpoints. Each result includes page_id, name, category, likes, verification status, and Instagram details like ig_username and ig_followers.
- **`scrape-creators-pp-cli facebook list-group`** - Fetches posts from a public Facebook group, limited to 3 posts per page due to API limitations. Each post includes id, text, url, reactionCount, commentCount, publishTime, videoDetails, and topComments. Supports sorting by TOP_POSTS, RECENT_ACTIVITY, CHRONOLOGICAL, or CHRONOLOGICAL_LISTINGS, with cursor-based pagination.
- **`scrape-creators-pp-cli facebook list-post`** - Fetches comments from a Facebook post or reel with cursor-based pagination. Each comment includes id, text, created_at, reply_count, reaction_count, and author details with name and profile_picture. Passing a feedback_id instead of a url significantly speeds up the request.
- **`scrape-creators-pp-cli facebook list-post-2`** - Extracts the transcript text from a Facebook video post or reel. Returns the transcript as a single text string with line breaks. Only works on videos under 2 minutes in length.
- **`scrape-creators-pp-cli facebook list-profile`** - Retrieves public Facebook page details including category, address, email, phone, website, services, priceRange, rating, likeCount, and followerCount. Also returns adLibrary status with the page's ad activity and pageId. Optionally includes businessHours when get_business_hours is set to true.
- **`scrape-creators-pp-cli facebook list-profile-2`** - Fetches photos from a public Facebook page with pagination support. Each photo includes photo_id, accessibility_caption, viewer_image with uri, height, and width, plus a thumbnail and direct url. Pagination requires passing both next_page_id and cursor from the previous response.
- **`scrape-creators-pp-cli facebook list-profile-3`** - Returns publicly visible Facebook profile posts, limited to 3 posts per page due to API limitations. Each post includes id, text, url, reactionCount, commentCount, publishTime, videoDetails with sdUrl, hdUrl, and thumbnailUrl, plus topComments. Accepts either a url or pageId parameter, where pageId is faster.
- **`scrape-creators-pp-cli facebook list-profile-4`** - Fetches up to 10 reels per request from a public Facebook page. Each reel includes id, url, view_count, description, creation_time, video_url, thumbnail, play_time_in_ms, and music details. Pagination requires passing both next_page_id and cursor from the previous response.

### google

Scrape Google search results

- **`scrape-creators-pp-cli google list`** - Retrieves detailed information about a specific Google ad including advertiserId, creativeId, format, firstShown, lastShown, and overallImpressions. Returns creativeRegions, regionStats with per-region impression data, and variations with destinationUrl, headline, description, and imageUrl. Text extraction uses OCR, so accuracy may vary.
- **`scrape-creators-pp-cli google list-adlibrary`** - Searches the Google Ad Transparency Library for advertisers by name. Returns a list of matching advertisers with their name, advertiser_id, and region, plus a list of associated website domains. Use the returned advertiser_id to look up a company's ads.
- **`scrape-creators-pp-cli google list-company`** - Fetches public ads for a company from the Google Ad Transparency Library by domain or advertiser_id. Each ad includes advertiserId, creativeId, format, adUrl, advertiserName, domain, firstShown, and lastShown. Costs 25 credits per request when get_ad_details=true; without it, only advertiserId and creativeId are returned at 1 credit.
- **`scrape-creators-pp-cli google list-search`** - Performs a Google search and returns organic results with url, title, and description for each result. Supports an optional region parameter (2-letter country code) to get localized results from a specific country.

### instagram

Gets Instagram profiles, posts, and reels

- **`scrape-creators-pp-cli instagram list`** - Fetches a lightweight Instagram profile summary by user ID, returning username, full name, biography, profile picture URL, verification status, follower count, following count, media count, and account privacy and type. Ideal for quick lookups or enrichment when you already have the numeric user ID.
- **`scrape-creators-pp-cli instagram list-media`** - Generates an AI-powered speech-to-text transcription for an Instagram video post or reel. The video must be under 2 minutes long. Returns a transcripts array with each item's shortcode and transcribed text; carousel posts produce one transcript per video slide. Expect 10-30 second response times, and null when no speech is detected.
- **`scrape-creators-pp-cli instagram list-post`** - Fetches detailed metadata for a single Instagram post or reel by shortcode or URL. Returns caption text, like count, comment count, video URL, video play count, video duration, display images, owner info, tagged users, and carousel sidecar children when applicable. Play counts are Instagram-only views and exclude cross-posted Facebook views.
- **`scrape-creators-pp-cli instagram list-post-2`** - Retrieves comments on a public Instagram post or reel. Each comment includes the comment text, creation timestamp, and commenter details such as username, user ID, verification status, and profile picture URL. Supports cursor-based pagination to load additional comment pages.
- **`scrape-creators-pp-cli instagram list-profile`** - Retrieves comprehensive public Instagram profile information including biography, bio links, follower and following counts, verification status, and profile picture URLs. Also returns recent timeline posts with engagement metrics such as likes, comments, and video view counts, plus a list of related profiles. Useful for account overview, audience analysis, or discovering similar creators.
- **`scrape-creators-pp-cli instagram list-reels`** - Searches for Instagram reels matching a keyword or phrase via Google Search, bypassing Instagram's login-gated search. Returns a list of reels with shortcode, caption, thumbnail, video URL, play count, like count, comment count, video duration, owner details, location, and audio attribution info. Play counts are Instagram-only views and exclude cross-posted Facebook views. Supports page-based pagination for browsing additional results.
- **`scrape-creators-pp-cli instagram list-song`** - DEPRECATED — this endpoint is no longer functional. Instagram removed the public audio pages that this endpoint relied on, so it cannot return data. Do not use this endpoint; there is currently no replacement available.
- **`scrape-creators-pp-cli instagram list-user`** - Returns the raw HTML embed snippet for an Instagram user's profile widget. The response contains a single html string that can be inserted into a webpage to render an embeddable Instagram profile card. Requires the user's handle as input.
- **`scrape-creators-pp-cli instagram list-user-2`** - Lists all story highlight albums for an Instagram user. Each highlight includes its ID, title, cover thumbnail URL, and owner info with username and profile picture. Accepts either a user_id or handle; providing user_id yields faster responses.
- **`scrape-creators-pp-cli instagram list-user-3`** - Returns a paginated list of a user's public Instagram reels (short-form videos). Each reel includes its shortcode, play count, like count, comment count, video versions with download URLs, thumbnail image, and owner info. Note that reel captions are not returned by this endpoint. Play counts are Instagram-only views and exclude cross-posted Facebook views. Supports cursor-based pagination via max_id; providing a user_id instead of a handle yields faster responses.
- **`scrape-creators-pp-cli instagram list-user-4`** - Returns a paginated feed of a user's public Instagram posts, including photos, videos, and carousels. Each item includes media type, shortcode, caption text, like count, comment count, play count, video URLs, image URLs, and tagged users. Play counts reflect Instagram-only views and exclude cross-posted Facebook views. Supports cursor-based pagination via next_max_id for scrolling through the full timeline.
- **`scrape-creators-pp-cli instagram list-user-5`** - Fetches the full contents of a specific Instagram story highlight album by its ID. Returns the highlight's cover image, title, user info, and an items array containing each story with its media type, image or video URLs, dimensions, timestamp, and sticker/interactive element data. Useful for archiving or analyzing individual highlight reels.

### kick

Scrape Kick clips

- **`scrape-creators-pp-cli kick list`** - Fetches detailed data for a Kick clip by URL, including video, metadata, and channel info. Returns clip id, title, clip_url, thumbnail_url, video_url, view_count, likes_count, duration, privacy status, and is_mature flag. Also includes category details (name, slug), creator info (username), and channel info (username, profile_picture).

### komi

Scrape Komi pages

- **`scrape-creators-pp-cli komi list`** - Scrapes a Komi page by URL, extracting the creator's profile, social links, and featured content. Returns id, username, avatar, displayName, bio, and social accounts (instagram, tiktok, youtube, twitter, facebook, snapchat). Also includes links, an array of link and product objects each with id, url, title, type, thumbnail, and optional price and currency for products.

### linkbio

Scrape Linkbio (lnk.bio) pages

- **`scrape-creators-pp-cli linkbio list`** - Scrapes a Linkbio (lnk.bio) page by URL, extracting the creator's profile and all their links. Returns handle, id, social accounts (instagram, tiktok, youtube, twitter, whatsapp), email, website, and links — an array of link objects each with url and text.

### linkedin

Scrape LinkedIn

- **`scrape-creators-pp-cli linkedin list`** - Retrieves detailed information about a specific LinkedIn ad by URL. Returns id, description, headline, adType, advertiser, and targeting with language, location, and audience criteria. Also includes totalImpressions, impressionsByCountry, adDuration, startDate, and endDate.
- **`scrape-creators-pp-cli linkedin list-ads`** - Searches the LinkedIn Ad Library by company name, keyword, or companyId with optional country and date filters. Each ad includes id, description, headline, adType, advertiser, targeting details, image or video URLs, totalImpressions, and impressionsByCountry. Supports pagination via paginationToken.
- **`scrape-creators-pp-cli linkedin list-company`** - Fetches a LinkedIn company page with details including name, description, logo, cover image, slogan, location, headquarters, employee count (headcount/staff size), website, industry, company type, founded year, specialties, funding rounds with investors, featured employees, recent posts, and similar company pages.
- **`scrape-creators-pp-cli linkedin list-company-2`** - Retrieves paginated posts from a LinkedIn company page, including each post's URL, ID, publication date, and full text content. Supports page-based pagination up to a maximum of 7 pages due to a LinkedIn platform limitation.
- **`scrape-creators-pp-cli linkedin list-post`** - Fetches a single LinkedIn post or article, returning the title, headline, full description text, author info with follower count, publication date, like count (reactions), comment count, and individual comments. Also includes related articles from the same author in moreArticles.
- **`scrape-creators-pp-cli linkedin list-profile`** - Retrieves a person's public LinkedIn profile data, including their name, photo, location, follower count (followers), about/bio summary, recent posts, work experience, education, articles, activity feed, publications, projects, recommendations, and similar profiles. Only returns publicly available information visible in an incognito browser.

### linkme

Get Linkme profile info

- **`scrape-creators-pp-cli linkme list`** - Retrieves a Linkme profile by URL, including identity, social links, and contact details. Returns profile with id, firstName, username, bio, profileVisitCount, profileImage, verifiedAccount, and isAmbassador flag. Also includes infoLinks (email addresses) and webLinks, an array of categorized social platform links (Spotify, Instagram, YouTube, Twitter, Facebook, and more) each with linkValue and faceValue.

### linktree

Scrape Linktree pages

- **`scrape-creators-pp-cli linktree list`** - Scrapes a Linktree page by URL, extracting the creator's profile and all their links. Returns id, username, profilePictureUrl, description, verticals, timezone, and links — an array of link objects each with id, type, title, and url. Also includes detected social accounts (instagram, tiktok, spotify, youtube, soundcloud, apple_music) and email_address.

### pillar

Scrape Pillar pages

- **`scrape-creators-pp-cli pillar list`** - Scrapes a Pillar page by URL, extracting the creator's profile, social links, and products. Returns id, first_name, last_name, email, location, and social accounts (tiktok, spotify, twitter, youtube, facebook, linkedin, instagram, and more). Also includes links with click counts and products with title, price, description, and image.

### pinterest

Scrape Pinterest pins

- **`scrape-creators-pp-cli pinterest list`** - Fetches a paginated list of pins from a Pinterest board by URL, returning each pin's id, description, title, images, board info, pin_join annotations, and aggregated_pin_data. Supports pagination via cursor and a trim option for lighter responses.
- **`scrape-creators-pp-cli pinterest list-pin`** - Fetches detailed information about a single Pinterest pin by URL, returning title, description, link, dominantColor, originPinner, pinner, images at multiple resolutions (imageSpec_236x through imageSpec_orig), and pinJoin with visual annotations. Supports a trim option for lighter responses.
- **`scrape-creators-pp-cli pinterest list-search`** - Searches Pinterest for pins matching a query, returning results with id, url, title, description, images, link, domain, board info, and pinner details. Supports pagination via cursor and a trim option for lighter responses.
- **`scrape-creators-pp-cli pinterest list-user`** - Fetches a paginated list of boards for a Pinterest user, returning each board's name, url, description, pin_count, follower_count, owner info, cover_images, and created_at. Supports pagination via cursor and a trim option for lighter responses.

### reddit

Scrape Reddit posts and comments

- **`scrape-creators-pp-cli reddit list`** - Retrieves detailed information about a specific Reddit ad by its id. Returns an analysis_summary with headline and media insights, plus the full inspiration_creative object containing id, budget_category, industry, placements, objective, and creative details (headline, body, type, thumbnail_url, created_at, post_url). Also includes profile_info with the advertiser's name.
- **`scrape-creators-pp-cli reddit list-ads`** - Searches the Reddit Ad Library for ads matching a query, returning a maximum of 30 results. Each ad includes id, budget_category, industry, placements, objective, and a creative object with headline, body, thumbnail_url, type, created_at, and post_url. Also includes profile_info with the advertiser's name. Supports filtering by industries, budgets, formats, placements, and objectives.
- **`scrape-creators-pp-cli reddit list-post`** - Retrieves comments and post details from a Reddit post by URL. Returns the post with title, author, score, ups, upvote_ratio, num_comments, and created_utc, plus a comments array where each comment includes author, body, body_html, score, created_utc, parent_id, permalink, and nested replies. Supports cursor-based pagination for loading more comments and a trim parameter for lighter responses.
- **`scrape-creators-pp-cli reddit list-search`** - Searches across all of Reddit for posts matching a query. Each post includes title, author, selftext, subreddit, score, ups, upvote_ratio, num_comments, created_utc, url, permalink, and is_video. Supports sort (relevance, new, top, comment_count), timeframe filtering, pagination via the after token, and a trim parameter for lighter responses.
- **`scrape-creators-pp-cli reddit list-subreddit`** - Fetches posts from a subreddit with sorting and filtering options. Each post includes title, author, selftext, score, ups, upvote_ratio, num_comments, created_utc, url, permalink, subreddit_subscribers, and is_video. Supports sort (best, hot, new, top, rising), timeframe filtering, pagination via the after token, and a trim parameter for lighter responses.
- **`scrape-creators-pp-cli reddit list-subreddit-2`** - Retrieves metadata about a subreddit by name or URL. The subreddit name must be case-sensitive. Returns display_name, description, subscribers, weekly_active_users, weekly_contributions, rules, icon_img, header_img, advertiser_category, submit_text, and created_at.
- **`scrape-creators-pp-cli reddit list-subreddit-3`** - Searches within a specific subreddit for posts, comments, and media matching a query. Returns posts with title, votes, num_comments, url, and created_at; comments with author, body, votes, and parent post info; and media with title, media_type, image dimensions, and gallery_count. Supports sort, timeframe filtering, and cursor-based pagination.

### snapchat

Scrape Snapchat user profiles and thier stories

- **`scrape-creators-pp-cli snapchat list`** - Retrieves a Snapchat user's public profile by handle, including identity, stories, and spotlight content. Returns userProfile with username, title, snapcodeImageUrl, subscriberCount, bio, and profilePictureUrl. Also includes highlightStoryMetadata with individual story snaps (mediaUrl, mediaType, thumbnailUrl) and spotlightStoryMetadata with video details and engagement stats (viewCount, shareCount, commentCount).

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
- **`scrape-creators-pp-cli tiktok list-creators`** - Discovers trending and popular TikTok creators, filterable by follower count range, creator country, and audience country. Returns `creator_list`, an array of creator objects each with `nickname`, `unique_id`, `follower_count`, `likes_count`, `video_views`, `engagement_rate`, and avatar URLs. Sortable by engagement, follower count, or average views.
- **`scrape-creators-pp-cli tiktok list-hashtags`** - Discovers trending and popular TikTok hashtags, filterable by time period (7/30/120 days) and country. Returns a list of hashtag objects each with `hashtag_name`, `rank`, `trend` data, and related video examples. Useful for identifying viral topics and content trends on TikTok.
- **`scrape-creators-pp-cli tiktok list-product`** - Fetches full details for a specific TikTok Shop product by its URL, including stock levels and affiliate videos. Returns `product_info` with `product_base` (title, images, sold_count, price), `skus` (variants with exact `stock` counts), and `product_detail_review` (product_rating, review_count, sample reviews); also returns `shop_info` (shop_name, shop_rating, followers_count) and `related_videos` (affiliate TikToks promoting the product). Related videos are only available in the US region.
- **`scrape-creators-pp-cli tiktok list-profile`** - Fetches public profile data for a TikTok user by their handle — useful for looking up a creator's identity, bio, and account stats. Returns a `user` object (display name, avatar URLs, bio/signature, verification status, bio link) and a `stats` object (followerCount, followingCount, heartCount/total likes, videoCount). This only returns profile metadata, not the user's actual videos or followers list.
- **`scrape-creators-pp-cli tiktok list-profile-2`** - Fetches videos posted by a TikTok user, sortable by latest or most popular — use this to get a creator's video feed or TikToks. Returns `aweme_list`, an array of video objects each containing `aweme_id`, `desc` (caption), `statistics` (play_count, digg_count/likes, comment_count, share_count, collect_count/saves), and `video` (download URLs, duration, cover image). Paginate with `max_cursor` from the previous response.
- **`scrape-creators-pp-cli tiktok list-search`** - Searches for TikTok videos under a specific hashtag — useful for finding content by topic or trend. Returns `aweme_list`, an array of video objects each with `aweme_id`, `desc` (caption), `statistics` (play_count, digg_count/likes, comment_count, share_count), `video` info, and `author` details. Paginate with `cursor` from the previous response.
- **`scrape-creators-pp-cli tiktok list-search-2`** - Searches for TikTok videos by keyword or phrase — the general video search across all of TikTok. Returns `search_item_list`, an array of objects each containing `aweme_info` with `aweme_id`, `desc` (caption), `statistics` (play_count, digg_count/likes, comment_count, share_count), `video` info, and `author` details. Paginate with `cursor`.
- **`scrape-creators-pp-cli tiktok list-search-3`** - Searches TikTok's 'Top' results by query — returns both videos and photo carousels, unlike keyword search which only returns videos. Returns `items`, an array of objects each with `id`, `desc` (caption), `content_type` (video or photo carousel), `statistics` (play_count, digg_count/likes, comment_count, share_count), `video` info, and `images` for carousels. Paginate with `cursor`.
- **`scrape-creators-pp-cli tiktok list-search-4`** - Searches for TikTok users by keyword or name — useful for finding creators or accounts matching a query. Returns `users`, an array of objects each containing `user_info` (nickname, unique_id, signature/bio, follower_count, following_count, avatar) and associated `items`. Paginate with `cursor` from the previous response.
- **`scrape-creators-pp-cli tiktok list-shop`** - Lists all products from a specific TikTok Shop store by its URL. Returns an array of product objects each with `title`, `cover` images, `url`, `price` info, `sold_count`, `review_count`, and `rating`. Paginate with `cursor` from the previous response; filter by region.
- **`scrape-creators-pp-cli tiktok list-shop-2`** - Searches TikTok Shop for products matching a keyword query. Returns an array of product objects each with `title`, `cover` image, `url` (product page link), `price`, `sold_count`, `review_count`, `rating`, and `shop_name`. Paginate with `page`; filter by region.
- **`scrape-creators-pp-cli tiktok list-shop-3`** - Fetches customer reviews for a TikTok Shop product by URL or product_id. Returns `product_reviews`, an array of review objects each with `rating`, `display_text`, `review_timestamp_fmt`, `review_user` (name, avatar), and `sku_specification` (variant purchased); also returns `total_reviews` count and `rating_distribution`. Paginate with `page`.
- **`scrape-creators-pp-cli tiktok list-song`** - Fetches detailed metadata for a specific TikTok sound or song by its clipId. Returns `music_info` with `title`, `author`, `album`, `duration`, `user_count` (number of videos using this sound), `play_url`, cover art, and artist details. Use the `clipId` from a sound URL or from the popular songs endpoint.
- **`scrape-creators-pp-cli tiktok list-song-2`** - Fetches TikTok videos that use a specific sound or song, identified by its clipId. Returns `aweme_list`, an array of video objects each with `aweme_id`, `desc` (caption), `statistics` (play_count, digg_count/likes, comment_count, share_count), `video` info, and `author` details. Paginate with `cursor` from the previous response.
- **`scrape-creators-pp-cli tiktok list-user`** - Retrieves audience demographic data for a TikTok user, showing where their followers are located by country. Returns `audienceLocations`, an array of objects each containing `country`, `countryCode`, `count`, and `percentage`. Costs 26 credits per request.
- **`scrape-creators-pp-cli tiktok list-user-2`** - Retrieves the follower list of a TikTok account by handle or user_id — useful for seeing who follows a creator or getting subscriber data. Returns `followers`, an array of user objects each with `nickname`, `unique_id`, `uid`, `follower_count`, `following_count`, and avatar URLs; also returns `total` follower count. Paginate with `min_time` from the previous response.
- **`scrape-creators-pp-cli tiktok list-user-3`** - Retrieves the following list — accounts that a TikTok user follows — by their handle. Returns `followings`, an array of user objects each with `nickname`, `unique_id`, `uid`, `follower_count`, `following_count`, `signature`, and avatar URLs; also returns `total` count. Paginate with `min_time` from the previous response.
- **`scrape-creators-pp-cli tiktok list-user-4`** - Checks if a TikTok user is currently live streaming and retrieves their live room details. Returns `liveRoomUserInfo` (nickname, avatar, followerCount, roomId) and `liveRoom` (title, startTime, status, `liveRoomStats` with enterCount and userCount, plus `streamData` with playback URLs in multiple qualities).
- **`scrape-creators-pp-cli tiktok list-user-5`** - Fetches products featured in a TikTok user's public showcase — the products a creator promotes on their profile. Returns an array of product objects each with title, price, images, and shop details. Use POST request if pagination is cutting off too early. Just send the query params in the body.
- **`scrape-creators-pp-cli tiktok list-video`** - Fetches detailed data for a single TikTok video by URL, including its metadata, engagement stats, and optionally its transcript/captions. Returns `aweme_detail` with `desc` (caption), `statistics` (play_count, digg_count/likes, comment_count, share_count, collect_count), `video` (download URLs including no-watermark), `author` info, and `music` info; also returns `transcript` in WEBVTT format if `get_transcript=true`.
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
- **`scrape-creators-pp-cli twitch list-user`** - Fetches a list of videos (100 max) for a Twitch user, returning each video's id, slug, url, embedURL, title, viewCount, language, durationSeconds, game info, broadcaster details with follower count, thumbnailURL, and videoQualities at multiple resolutions with a signed videoURL for playback. Supports pagination via cursor and a trim option for lighter responses.

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
- **`scrape-creators-pp-cli youtube list-channel`** - Retrieves a paginated list of short-form videos (Shorts) from a YouTube channel, including each short's title, URL, view count (views), likes, comments, and description. Supports sorting by newest or popular, and use the continuationToken to page through all results. Returns data in the shorts array.
- **`scrape-creators-pp-cli youtube list-channelvideos`** - Fetches a paginated list of videos uploaded by a YouTube channel, including each video's title, URL, thumbnail, view count (views), publish date, duration, and description. Supports sorting by latest or popular, and use the continuationToken to page through all results. Optionally include extras like like count, comment count, and descriptions for each video.
- **`scrape-creators-pp-cli youtube list-communitypost`** - Retrieves the full details of a YouTube community post, including its text content, attached images, like count, publish date, and associated channel info. Also returns a linked video if the post includes one.
- **`scrape-creators-pp-cli youtube list-playlist`** - Retrieves all videos in a YouTube playlist, including the playlist title, owner info, total video count, and each video's title, URL, thumbnail, duration, and channel. Accepts the playlist ID found in the 'list' URL parameter.
- **`scrape-creators-pp-cli youtube list-search`** - Searches YouTube by keyword query and returns matching videos, channels, playlists, shorts, shelves, and live streams. Each video result includes title, URL, thumbnail, view count (views), publish date, duration, channel info, and badges. Supports filtering by upload date, sorting by relevance or popularity, and paginating with continuationToken.
- **`scrape-creators-pp-cli youtube list-search-2`** - Searches YouTube for content matching a specific hashtag and returns matching videos with title, URL, thumbnail, view count (views), publish date, duration, and channel info. Supports pagination via continuationToken and filtering to return all content types or only shorts.
- **`scrape-creators-pp-cli youtube list-shorts`** - Fetches approximately 48 currently trending YouTube Shorts (viral/popular short-form videos) per call, returning each short's title, URL, thumbnail, view count (views), like count (likes), comment count, publish date, channel info, keywords, and duration. Each subsequent call returns a fresh batch of different trending shorts.
- **`scrape-creators-pp-cli youtube list-video`** - Fetches full details for a YouTube video or short, including title, description, thumbnail, view count (views), like count (likes), comment count, publish date, duration, genre, keywords, chapters, collaborators, and available caption tracks (subtitles/captions). Also returns related recommended videos in watchNextVideos and channel info for the uploader.
- **`scrape-creators-pp-cli youtube list-video-2`** - Fetches comments and replies from a YouTube video, including each comment's text content, author details, like count, reply count, and publish date. Supports ordering by top or newest, and paginating with continuationToken. Limited to approximately 1,000 top comments or 7,000 newest comments.
- **`scrape-creators-pp-cli youtube list-video-3`** - Retrieves the captions, subtitles, or transcript of a YouTube video or short. Returns both a timestamped transcript array with start/end times and a plain-text version in transcript_only_text. Supports specifying a language code. Note: the video must be under 2 minutes for transcript extraction to work.
- **`scrape-creators-pp-cli youtube list-video-4`** - Fetches replies to a specific comment on a YouTube video, including each reply's text content, author details (name, channel ID, avatar, verified/creator status), like count, and publish date. Requires a continuationToken obtained from the 'repliesContinuationToken' field on comments returned by the Comments endpoint. Supports paginating through additional replies with the continuationToken returned in each response.


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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-scrape-creators -g
```

Then invoke `/pp-scrape-creators <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/cmd/scrape-creators-pp-mcp@latest
```

Then register it:

```bash
claude mcp add scrape-creators scrape-creators-pp-mcp -e SCRAPE_CREATORS_API_KEY_AUTH=<your-key>
```

</details>

<!-- pp-hermes-install-anchor -->
## Install via Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-scrape-creators --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-scrape-creators --force
```

## Install via OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-scrape-creators skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-scrape-creators. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/scrape-creators-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SCRAPE_CREATORS_API_KEY_AUTH` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle, install the MCP binary and configure it manually.

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
        "SCRAPE_CREATORS_API_KEY_AUTH": "<your-key>"
      }
    }
  }
}
```

</details>

## Cookbook

### Map a creator's full footprint

```bash
scrape-creators-pp-cli creator find mrbeast --json --select platform,handle,follower_count
```

Probes every platform's profile endpoint in parallel and returns a presence matrix with follower counts. Useful before writing a brief or outreach email.

### Find a creator's viral hits

```bash
scrape-creators-pp-cli sync --resources youtube && \
  scrape-creators-pp-cli content spikes mrbeast --threshold 2.0 --platform youtube --json
```

Sync videos to local SQLite, then return videos whose views are more than 2× the creator's average.

### Compare two creators side-by-side

```bash
scrape-creators-pp-cli creator compare mrbeast pewdiepie --platform youtube --json
```

Pulls each creator's profile from the chosen platform in parallel, computes engagement ratio, and returns one row per creator.

### Track a brand's ad campaigns across networks

```bash
scrape-creators-pp-cli ads monitor "Liquid Death" --json
```

Snapshots Facebook + Google + LinkedIn ads into SQLite. Rerun on a cron and the command diffs new vs disappeared ads since the last snapshot.

### Triangulate where a trend is rising

```bash
scrape-creators-pp-cli trends triangulate "AI agents" --json --select per_platform.platform,per_platform.count,per_platform.delta
```

Probes TikTok, YouTube, Reddit, and Threads for a topic in parallel. Returns per-platform velocity — the leading platform pops out when you run it on a cron.

### Watch a hashtag's growth

```bash
scrape-creators-pp-cli trends delta "booktok" --platform tiktok --days 7 --json
```

Snapshots the current hashtag count and reports the delta vs the most recent prior snapshot in the last 7 days. Schedule on a cron to track growth.

### Grep your transcript corpus

```bash
scrape-creators-pp-cli transcripts search "affiliate link" --json --select platform,id,snippet
```

FTS5-style search over every transcript synced into the local store — useful for brand-safety audits.

### Resolve a link-in-bio

```bash
scrape-creators-pp-cli bio resolve https://linktr.ee/mrbeast --json
```

Auto-detects linktree / komi / pillar / linkbio / linkme and returns a unified destination list. Five endpoints unified behind one command.

### Watch your credit budget

```bash
scrape-creators-pp-cli account budget --days 7 --json
```

Fuses the API's daily-usage history with your remaining credits to project days of runway at the current burn rate.

### Pipe results into jq

```bash
scrape-creators-pp-cli creator find mrbeast --json | \
  jq '.[] | select(.follower_count > 1000000) | {platform, follower_count}'
```

`--json` returns valid JSON regardless of where stdout is connected; status messages go to stderr so the pipe stays clean.

## Health Check

```bash
scrape-creators-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

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

### API-specific

- **401 unauthorized** — Set SCRAPE_CREATORS_API_KEY in your environment or run `scrape-creators-pp-cli auth set-token <key>`.
- **Empty result for a profile that should exist** — Some platforms require a username, others a handle prefixed with @, and a few need the numeric ID. Check the command's `--help` for the exact arg shape.
- **Credits running out faster than expected** — Run `scrape-creators-pp-cli account budget --json` — it shows daily spend and projects days remaining.
- **Transcript search returns nothing** — Transcripts must be synced first — run `scrape-creators-pp-cli sync <handle> --platform <p>` and try again.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**scrapecreators-cli**](https://github.com/ScrapeCreators/scrapecreators-cli) — JavaScript
- [**n8n-nodes-scrape-creators**](https://github.com/adrianhorning08/n8n-nodes-scrape-creators) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
