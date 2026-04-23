# Action Map Proposal: PR #113 command naming

Draft for review before any cobra wiring lands. Target destination on merge: `library/developer-tools/scrape-creators/.manuscripts/action-map-proposal.md` on the `feat/scrape-creators` branch.

Planning doc: `docs/plans/2026-04-23-002-feat-pr-113-library-quality-plan.md`, unit U2.

## Purpose

PR #113 ships 115 endpoints registered under OpenAPI operation IDs (`list`, `list-post-2`, `list-user-5`, `list-adlibrary-3`). This proposal renames every endpoint to a `platform action` pair matching the shape `@scrapecreators/cli` v1 exposes via `src/command-registry.js`. Adrian reviews naming here before any of the 82 leaf files plus 24 `promoted_<platform>.go` parents are restructured.

Once the map is agreed:

1. Compile to `action_map.yaml` -> `internal/cli/action_map.go`.
2. Split every `promoted_<platform>.go` so the per-platform parent is pure navigation (no `RunE`). The former parent shortcut moves to a named action.
3. Register operation-ID leaf names and former top-level parent shortcuts as hidden cobra aliases for backward compatibility.

## Naming rules

Derived from v1's `parseToolPath` (`github.com/ScrapeCreators/scrapecreators-cli/src/command-registry.js`):

- Strip the version prefix (`/v1`, `/v2`, `/v3`). Version dedup picks the latest version per `(platform, action)` pair.
- First segment after the version is the platform.
- Remaining segments, joined by `-` and lowercased, form the action.
- Single-segment path with a hyphen (e.g., `/v1/detect-age-gender`) splits on the first `-`: platform = part before, action = part after.
- Single-segment path with no hyphen (e.g., `/v1/linktree`) uses `get` as the action.
- `adLibrary` lowercases to `adlibrary`.

Lint: every action matches `^[a-z][a-z0-9-]*$`; no `list-` prefix; no trailing `-N` version suffix.

## v1 coverage note

Every endpoint in this table appears in PR #113's `spec.json`. v1's tool list comes from the same API, so the vast majority of names correspond directly. Entries marked `PP-only` in the "v1 exposes" column are endpoints v1 either renames at the CLI surface or does not expose. Adrian: please flag any entry where the proposed name does not match v1's current command-registry output; those are the ones to correct first.

## Per-platform proposal

### tiktok

| OpenAPI path | Current Use: | Proposed v2 | v1 exposes | Rationale |
|--------------|--------------|-------------|------------|-----------|
| /v1/tiktok/profile | tiktok list-profile | tiktok profile | yes | direct |
| /v1/tiktok/user/audience | tiktok list-user | tiktok user-audience | yes | direct |
| /v3/tiktok/profile/videos | tiktok list-profile-2 | tiktok profile-videos | yes | v3 beats v1 per dedup |
| /v2/tiktok/video | tiktok list-video-4 | tiktok video | yes | v2 beats v1 per dedup |
| /v1/tiktok/video/transcript | tiktok list-video-3 | tiktok video-transcript | yes | direct |
| /v1/tiktok/user/live | tiktok list-user-4 | tiktok user-live | yes | direct |
| /v1/tiktok/video/comments | tiktok list-video-2 | tiktok video-comments | yes | direct |
| /v1/tiktok/video/comment/replies | tiktok list-video | tiktok video-comment-replies | yes | direct |
| /v1/tiktok/user/following | tiktok list-user-3 | tiktok user-following | yes | direct |
| /v1/tiktok/user/followers | tiktok list-user-2 | tiktok user-followers | yes | direct |
| /v1/tiktok/search/users | tiktok list-search-4 | tiktok search-users | yes | direct |
| /v1/tiktok/search/hashtag | tiktok list-search | tiktok search-hashtag | yes | direct |
| /v1/tiktok/search/keyword | tiktok list-search-2 | tiktok search-keyword | yes | direct |
| /v1/tiktok/search/top | tiktok list-search-3 | tiktok search-top | yes | direct |
| /v1/tiktok/songs/popular | tiktok list-songs | tiktok songs-popular | yes | direct |
| /v1/tiktok/creators/popular | tiktok list | tiktok creators-popular | yes | replaces parent-shortcut |
| /v1/tiktok/videos/popular | tiktok list-videos | tiktok videos-popular | yes | direct |
| /v1/tiktok/hashtags/popular | tiktok list-hashtags | tiktok hashtags-popular | yes | direct |
| /v1/tiktok/song | tiktok list-song | tiktok song | yes | direct |
| /v1/tiktok/song/videos | tiktok list-song-2 | tiktok song-videos | yes | direct |
| /v1/tiktok/get-trending-feed | tiktok list-gettrendingfeed | tiktok trending-feed | yes | drop `get-` prefix per v1 convention |
| /v1/tiktok/shop/search | tiktok list-shop-3 | tiktok shop-search | yes | direct |
| /v1/tiktok/shop/products | tiktok list-shop-2 | tiktok shop-products | yes | direct |
| /v1/tiktok/product | tiktok list-product | tiktok product | yes | direct |
| /v1/tiktok/shop/product/reviews | tiktok list-shop | tiktok shop-product-reviews | yes | direct |
| /v1/tiktok/user/showcase | tiktok list-user-5 | tiktok user-showcase | yes | direct |

Parent shortcut today (`promoted_tiktok.go` `RunE`): points at creators/popular. After the split, that behavior lives under `tiktok creators-popular`. Invoking `scrape-creators-pp-cli tiktok` with no action prints help.

### instagram

| OpenAPI path | Current Use: | Proposed v2 | v1 exposes | Rationale |
|--------------|--------------|-------------|------------|-----------|
| /v1/instagram/profile | instagram list-profile | instagram profile | yes | direct |
| /v1/instagram/basic-profile | instagram list | instagram basic-profile | yes | replaces parent-shortcut |
| /v2/instagram/user/posts | instagram list-user-5 | instagram user-posts | yes | v2 wins |
| /v1/instagram/post | instagram list-post | instagram post | yes | direct |
| /v2/instagram/media/transcript | instagram list-media | instagram media-transcript | yes | direct |
| /v2/instagram/reels/search | instagram list-reels | instagram reels-search | yes | direct |
| /v2/instagram/post/comments | instagram list-post-2 | instagram post-comments | yes | direct |
| /v1/instagram/user/reels | instagram list-user-4 | instagram user-reels | yes | direct |
| /v1/instagram/user/highlights | instagram list-user-3 | instagram user-highlights | yes | direct |
| /v1/instagram/user/highlight/detail | instagram list-user-2 | instagram user-highlight-detail | yes | direct |
| /v1/instagram/song/reels | instagram list-song | instagram song-reels | yes | direct (deprecated endpoint; keep) |
| /v1/instagram/user/embed | instagram list-user | instagram user-embed | yes | direct |

### youtube

| OpenAPI path | Current Use: | Proposed v2 | v1 exposes | Rationale |
|--------------|--------------|-------------|------------|-----------|
| /v1/youtube/channel | youtube list | youtube channel | yes | replaces parent-shortcut |
| /v1/youtube/channel-videos | youtube list-channelvideos | youtube channel-videos | yes | direct |
| /v1/youtube/channel/shorts | youtube list-channel | youtube channel-shorts | yes | direct |
| /v1/youtube/video | youtube list-video | youtube video | yes | direct |
| /v1/youtube/video/transcript | youtube list-video-4 | youtube video-transcript | yes | direct |
| /v1/youtube/search | youtube list-search | youtube search | yes | direct |
| /v1/youtube/search/hashtag | youtube list-search-2 | youtube search-hashtag | yes | direct |
| /v1/youtube/video/comments | youtube list-video-3 | youtube video-comments | yes | direct |
| /v1/youtube/video/comment/replies | youtube list-video-2 | youtube video-comment-replies | yes | direct |
| /v1/youtube/shorts/trending | youtube list-shorts | youtube shorts-trending | yes | direct |
| /v1/youtube/playlist | youtube list-playlist | youtube playlist | yes | direct |
| /v1/youtube/community-post | youtube list-communitypost | youtube community-post | yes | direct |

### linkedin

| OpenAPI path | Current Use: | Proposed v2 | v1 exposes | Rationale |
|--------------|--------------|-------------|------------|-----------|
| /v1/linkedin/profile | linkedin list-profile | linkedin profile | yes | direct |
| /v1/linkedin/company | linkedin list-company | linkedin company | yes | direct |
| /v1/linkedin/company/posts | linkedin list-company-2 | linkedin company-posts | yes | direct |
| /v1/linkedin/post | linkedin list-post | linkedin post | yes | direct |
| /v1/linkedin/ads/search | linkedin list-ads | linkedin ads-search | yes | direct |
| /v1/linkedin/ad | linkedin list | linkedin ad | yes | replaces parent-shortcut |

### facebook

| OpenAPI path | Current Use: | Proposed v2 | v1 exposes | Rationale |
|--------------|--------------|-------------|------------|-----------|
| /v1/facebook/profile | facebook list-profile | facebook profile | yes | direct |
| /v1/facebook/profile/reels | facebook list-profile-4 | facebook profile-reels | yes | direct |
| /v1/facebook/profile/photos | facebook list-profile-2 | facebook profile-photos | yes | direct |
| /v1/facebook/profile/posts | facebook list-profile-3 | facebook profile-posts | yes | direct |
| /v1/facebook/group/posts | facebook list-group | facebook group-posts | yes | direct |
| /v1/facebook/post | facebook list-post | facebook post | yes | direct |
| /v1/facebook/post/transcript | facebook list-post-3 | facebook post-transcript | yes | direct |
| /v1/facebook/post/comments | facebook list-post-2 | facebook post-comments | yes | direct |
| /v1/facebook/adLibrary/ad | facebook list | facebook adlibrary-ad | yes | replaces parent-shortcut; lowercase adlibrary per v1 |
| /v1/facebook/adLibrary/search/ads | facebook list-adlibrary | facebook adlibrary-search-ads | yes | direct |
| /v1/facebook/adLibrary/company/ads | facebook list-adlibrary-2 | facebook adlibrary-company-ads | yes | direct |
| /v1/facebook/adLibrary/search/companies | facebook list-adlibrary-3 | facebook adlibrary-search-companies | yes | direct |

### google

| OpenAPI path | Current Use: | Proposed v2 | v1 exposes | Rationale |
|--------------|--------------|-------------|------------|-----------|
| /v1/google/search | google list-search | google search | yes | direct |
| /v1/google/company/ads | google list-company | google company-ads | yes | direct |
| /v1/google/ad | google list | google ad | yes | replaces parent-shortcut |
| /v1/google/adLibrary/advertisers/search | google list-adlibrary | google adlibrary-advertisers-search | yes | direct |

### twitter

| OpenAPI path | Current Use: | Proposed v2 | v1 exposes | Rationale |
|--------------|--------------|-------------|------------|-----------|
| /v1/twitter/profile | twitter list-profile | twitter profile | yes | direct |
| /v1/twitter/user-tweets | twitter list-usertweets | twitter user-tweets | yes | direct |
| /v1/twitter/tweet | twitter list-tweet | twitter tweet | yes | direct |
| /v1/twitter/tweet/transcript | twitter list-tweet-2 | twitter tweet-transcript | yes | direct |
| /v1/twitter/community | twitter list | twitter community | yes | replaces parent-shortcut |
| /v1/twitter/community/tweets | twitter list-community | twitter community-tweets | yes | direct |

### reddit

| OpenAPI path | Current Use: | Proposed v2 | v1 exposes | Rationale |
|--------------|--------------|-------------|------------|-----------|
| /v1/reddit/subreddit/details | reddit list-subreddit-2 | reddit subreddit-details | yes | direct |
| /v1/reddit/subreddit | reddit list-subreddit | reddit subreddit | yes | direct |
| /v1/reddit/subreddit/search | reddit list-subreddit-3 | reddit subreddit-search | yes | direct |
| /v1/reddit/post/comments | reddit list-post | reddit post-comments | yes | direct |
| /v1/reddit/search | reddit list-search | reddit search | yes | direct |
| /v1/reddit/ads/search | reddit list-ads | reddit ads-search | yes | direct |
| /v1/reddit/ad | reddit list | reddit ad | yes | replaces parent-shortcut |

### truthsocial

| OpenAPI path | Current Use: | Proposed v2 | v1 exposes | Rationale |
|--------------|--------------|-------------|------------|-----------|
| /v1/truthsocial/profile | truthsocial list-profile | truthsocial profile | yes | direct |
| /v1/truthsocial/user/posts | truthsocial list-user | truthsocial user-posts | yes | direct |
| /v1/truthsocial/post | truthsocial list | truthsocial post | yes | replaces parent-shortcut |

### threads

| OpenAPI path | Current Use: | Proposed v2 | v1 exposes | Rationale |
|--------------|--------------|-------------|------------|-----------|
| /v1/threads/profile | threads list-profile | threads profile | yes | direct |
| /v1/threads/user/posts | threads list-user | threads user-posts | yes | direct |
| /v1/threads/post | threads list | threads post | yes | replaces parent-shortcut |
| /v1/threads/search | threads list-search | threads search | yes | direct |
| /v1/threads/search/users | threads list-search-2 | threads search-users | yes | direct |

### bluesky

| OpenAPI path | Current Use: | Proposed v2 | v1 exposes | Rationale |
|--------------|--------------|-------------|------------|-----------|
| /v1/bluesky/profile | bluesky list-profile | bluesky profile | yes | direct |
| /v1/bluesky/user/posts | bluesky list-user | bluesky user-posts | yes | direct |
| /v1/bluesky/post | bluesky list | bluesky post | yes | replaces parent-shortcut |

### pinterest

| OpenAPI path | Current Use: | Proposed v2 | v1 exposes | Rationale |
|--------------|--------------|-------------|------------|-----------|
| /v1/pinterest/search | pinterest list-search | pinterest search | yes | direct |
| /v1/pinterest/pin | pinterest list-pin | pinterest pin | yes | direct |
| /v1/pinterest/user/boards | pinterest list-user | pinterest user-boards | yes | direct |
| /v1/pinterest/board | pinterest list | pinterest board | yes | replaces parent-shortcut |

### twitch

| OpenAPI path | Current Use: | Proposed v2 | v1 exposes | Rationale |
|--------------|--------------|-------------|------------|-----------|
| /v1/twitch/profile | twitch list-profile | twitch profile | yes | direct |
| /v1/twitch/clip | twitch list | twitch clip | yes | replaces parent-shortcut |

### kick

| OpenAPI path | Current Use: | Proposed v2 | v1 exposes | Rationale |
|--------------|--------------|-------------|------------|-----------|
| /v1/kick/clip | kick list | kick clip | yes | replaces parent-shortcut |

### snapchat

| OpenAPI path | Current Use: | Proposed v2 | v1 exposes | Rationale |
|--------------|--------------|-------------|------------|-----------|
| /v1/snapchat/profile | snapchat list | snapchat profile | yes | replaces parent-shortcut |

### Single-endpoint platforms (map to `<platform> get` per v1 convention)

| OpenAPI path | Current Use: | Proposed v2 | v1 exposes | Rationale |
|--------------|--------------|-------------|------------|-----------|
| /v1/linktree | linktree list | linktree get | yes | single-endpoint -> get |
| /v1/komi | komi list | komi get | yes | single-endpoint -> get |
| /v1/pillar | pillar list | pillar get | yes | single-endpoint -> get |
| /v1/linkbio | linkbio list | linkbio get | yes | single-endpoint -> get |
| /v1/amazon/shop | amazon list | amazon shop | yes | two-segment platform+action |
| /v1/linkme | linkme list | linkme get | yes | single-endpoint -> get |

### detect (special case: single-segment path with hyphen)

| OpenAPI path | Current Use: | Proposed v2 | v1 exposes | Rationale |
|--------------|--------------|-------------|------------|-----------|
| /v1/detect-age-gender | detect-age-gender list | detect age-gender | yes | split on first hyphen per v1 |

### account (PP-only surfacing)

v1 exposes only `scrapecreators balance` (at the top level). PR #113 surfaces four `/v1/account/*` endpoints. Proposal: surface all four under an `account` platform. The `balance` alias stays as a hidden top-level command pointing at `account credit-balance`.

| OpenAPI path | Current Use: | Proposed v2 | v1 exposes | Rationale |
|--------------|--------------|-------------|------------|-----------|
| /v1/account/credit-balance | account list | account credit-balance | via top-level `balance` | PP-only platform; v1 `balance` becomes hidden alias |
| /v1/account/get-api-usage | account list-getapiusage | account api-usage | PP-only | drop `get-` prefix |
| /v1/account/get-daily-usage-count | account list-getdailyusagecount | account daily-usage | PP-only | drop `get-` and `-count` noise |
| /v1/account/get-most-used-routes | account list-getmostusedroutes | account most-used-routes | PP-only | drop `get-` prefix |

Also: preserve top-level `balance` as a hidden alias (same shape as v1) that routes to `account credit-balance`.

## Non-endpoint CLI commands (keep as-is)

These are CLI-layer features, not API endpoints. No action-map entry needed. Current names are fine.

- `auth` (credential management)
- `doctor` (health check)
- `api` (endpoint discovery; current `api discovery` may become `api list` for consistency with cobra convention)
- `export`
- `import`
- `sync`
- `search` (local FTS with live fallback)
- `analytics` (local DB queries)
- `tail` (polling stream)
- `workflow` (compound multi-op)
- `completion` (cobra-standard)
- `version`
- `help`
- `agent` (plus `agent add <target>` subcommand, per plan U4)

## Hidden alias policy

Every operation-ID leaf name (`list`, `list-post-2`, `list-user-5`, etc.) registers as a `cobra.Command` with `Hidden: true` and `Aliases:` pointing at the new name. Existing top-level parent shortcuts (e.g., `scrape-creators-pp-cli facebook --url <ad>` invoking the ad-library-search shortcut) also register as hidden aliases that route to the new action. Removal is not scheduled; these aliases are the compatibility guarantee for users who already pinned old names.

## Renames needing Adrian's judgment call

These are the names where v1's strict algorithm produces something that reads awkwardly, and the proposal takes a light-touch liberty. Flag any that should revert to strict v1 output.

1. `tiktok trending-feed` (strict: `tiktok get-trending-feed`). Dropped `get-` prefix.
2. `account api-usage` (strict would be `account get-api-usage`). Dropped `get-`.
3. `account daily-usage` (strict: `account get-daily-usage-count`). Dropped `get-` and `-count`.
4. `account most-used-routes` (strict: `account get-most-used-routes`). Dropped `get-`.
5. Whether to collapse `adlibrary-ad`/`adlibrary-search-ads`/`adlibrary-company-ads`/`adlibrary-search-companies` into a cleaner ad-library sub-namespace (e.g., `facebook ads show|search|company|companies`) or keep the verbose dash-separated form. Proposal keeps verbose for strict v1 parity; an `ads` sub-namespace reads better but diverges from v1's output.

## Former parent shortcut moves

`promoted_<platform>.go` files today carry their own `RunE`. After the split, each points at a specific endpoint that becomes a named action. Summary:

| Platform | Former parent RunE targets | New action |
|----------|----------------------------|------------|
| tiktok | /v1/tiktok/creators/popular | tiktok creators-popular |
| instagram | /v1/instagram/basic-profile | instagram basic-profile |
| youtube | /v1/youtube/channel | youtube channel |
| linkedin | /v1/linkedin/ad | linkedin ad |
| facebook | /v1/facebook/adLibrary/ad | facebook adlibrary-ad |
| google | /v1/google/ad | google ad |
| twitter | /v1/twitter/community | twitter community |
| reddit | /v1/reddit/ad | reddit ad |
| truthsocial | /v1/truthsocial/post | truthsocial post |
| threads | /v1/threads/post | threads post |
| bluesky | /v1/bluesky/post | bluesky post |
| pinterest | /v1/pinterest/board | pinterest board |
| twitch | /v1/twitch/clip | twitch clip |
| kick | /v1/kick/clip | kick clip |
| snapchat | /v1/snapchat/profile | snapchat profile |
| linktree, komi, pillar, linkbio, linkme | single endpoint | `<platform> get` |
| amazon | /v1/amazon/shop | amazon shop |
| account | /v1/account/credit-balance | account credit-balance |
| detect-age-gender | /v1/detect-age-gender | detect age-gender |

After the split, `scrape-creators-pp-cli tiktok` with no arguments prints help (not an action). The old shortcut behavior stays available through a hidden alias pointing at the new action.

## Total counts

- Endpoints in `spec.json`: 115
- New `platform action` entries: 115 (1:1 mapping; no endpoints dropped)
- Hidden operation-ID aliases: one per leaf, plus one per former top-level parent shortcut
- Platforms with per-platform cobra parent: 24 (every entry in the former parent shortcut table above, plus single-endpoint platforms)

## What to review

Adrian, the review budget is naming only. Specifically:

1. Any name that should read differently than the strict v1 algorithm produces (flagged above in the "Renames needing judgment call" section).
2. Any name that does not match v1's current `command-registry.js` output for endpoints v1 exposes. I believe this table matches, but v1's live output is authoritative.
3. Whether the PP-only `account` platform is right, or whether you prefer a different top-level (e.g., keep `balance` as the primary and nest the three usage endpoints under it: `balance usage`, `balance daily-usage`, `balance top-routes`).
4. Whether the `adlibrary` entries should collapse into an `ads` sub-namespace.

Implementer: once this is agreed, U2 compiles `action_map.yaml` -> `action_map.go`, splits the 24 `promoted_<platform>.go` files, rewrites the 82 leaf registrations, and lands the hidden aliases. U2 waits for PR #113 to merge before any cobra wiring.
