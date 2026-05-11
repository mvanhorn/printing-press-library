# Instagram PP-CLI — Absorb Manifest
# Run: 20260510-220738

## Absorbed Features (Table Stakes)

Every feature found in any existing tool. Kill checks applied inline.

| # | Command | Description | Source Tool(s) | Kill Check |
|---|---------|-------------|----------------|------------|
| 1 | `profile get <username>` | Fetch full user profile: follower count, bio, post count, verified status | instagrapi, instagram-private-api | KEEP — core identity lookup |
| 2 | `profile me` | Fetch authenticated user's own profile | instagrapi, mcpware/instagram-mcp | KEEP — diagnostic + identity |
| 3 | `profile search <query>` | Search users by username or display name | instagram-private-api, instagrapi | KEEP — discovery |
| 4 | `media list <username>` | List a user's posts, paginated, structured JSON | instagrapi, instagram-private-api, mcpware/instagram-mcp | KEEP — baseline |
| 5 | `media get <media-id>` | Get single post details (caption, likes, comments, dimensions) | instagrapi, mcpware/instagram-mcp | KEEP — baseline |
| 6 | `media download <media-id>` | Download photo or video to disk | ig-dl, instagram-private-api | KEEP — high demand |
| 7 | `media download-bulk <username>` | Download all posts from a user to a local directory | ig-dl, gallery-dl wrapper | KEEP — archival use case |
| 8 | `media like <media-id>` | Like a post | instagrapi | KEEP — write action |
| 9 | `media unlike <media-id>` | Unlike a post | instagrapi | KEEP — write action |
| 10 | `media save <media-id>` | Bookmark/save a post to collection | instagrapi | KEEP |
| 11 | `media saved` | List authenticated user's saved posts | instagrapi | KEEP — read own data |
| 12 | `media liked` | List posts the authenticated user has liked | instagram-private-api | KEEP — own activity |
| 13 | `feed timeline` | Get home timeline feed | instagrapi, supreme-gg-gg/instagram-cli | KEEP — core surface |
| 14 | `feed explore` | Get Explore feed recommendations | instagrapi | KEEP — discovery |
| 15 | `stories list <username>` | List active stories for a user | instagrapi, go-instagram-cli, mcpware/instagram-mcp | KEEP — expiring content |
| 16 | `stories download <username>` | Download all active stories before expiry | ig-dl, go-instagram-cli | KEEP — time-sensitive |
| 17 | `stories view <story-id>` | Mark a story as viewed (POST to seen endpoint) | instagrapi | KEEP — interaction |
| 18 | `reels list <username>` | List a user's reels | instagrapi, mcpware/instagram-mcp | KEEP — growing surface |
| 19 | `followers list <username>` | List all followers, paginated | instagrapi, instagram-private-api | KEEP — core relationship |
| 20 | `following list <username>` | List all accounts a user follows | instagrapi, instagram-private-api | KEEP — core relationship |
| 21 | `friendships follow <username>` | Follow a user | instagrapi | KEEP — write action |
| 22 | `friendships unfollow <username>` | Unfollow a user | instagrapi | KEEP — write action |
| 23 | `friendships check <username>` | Check follow relationship: mutual / one-way / none | instagrapi | KEEP — quick lookup |
| 24 | `comments list <media-id>` | List comments on a post, paginated | instagrapi, mcpware/instagram-mcp | KEEP — content surface |
| 25 | `comments add <media-id> <text>` | Post a comment on a media item | instagrapi | KEEP — write action |
| 26 | `hashtag posts <hashtag>` | Get recent posts for a hashtag | instagrapi, instagram-private-api | KEEP — discovery |
| 27 | `location posts <location-id>` | Get posts tagged at a location | instagrapi | KEEP — geo discovery |
| 28 | `sync users` | Sync profile data for followed accounts to SQLite | (novel baseline) | KEEP — offline layer |
| 29 | `sync media <username>` | Sync a user's media posts to SQLite store | (novel baseline) | KEEP — offline layer |
| 30 | `sync followers <username>` | Sync followers + following lists to SQLite (with snapshot timestamp) | (novel baseline) | KEEP — enables diff features |

---

## Transcendence Table

Features that no existing tool offers. All powered by local SQLite cross-entity queries or 3-table JOINs — not possible via any single Instagram API call.

| # | Command | Score | Persona Served | Buildability Proof | Source |
|---|---------|-------|----------------|-------------------|--------|
| T1 | `followers diff <username>` | 10/10 | Creator tracking audience churn; Manager monitoring brand account health | SELECT from two `followers_snapshots` rows taken at different times; SET DIFFERENCE identifies unfollowers. Zero API calls after sync. | novel — SQLite snapshot diff |
| T2 | `following non-mutual` | 10/10 | Anyone managing who they follow; curiosity-driven personal accounts | JOIN `following` LEFT OUTER JOIN `followers` WHERE `followers.pk IS NULL`; lists every account you follow that doesn't follow back. | novel — 2-table JOIN |
| T3 | `posts engagement <username> [--since 30d]` | 10/10 | Creator benchmarking own content; Manager comparing posts for a client | SELECT media_id, caption, (likes+comments)/followers*100 AS rate FROM `media` JOIN `users` ORDER BY rate DESC. No aggregation endpoint exists. | novel — computed rate via JOIN |
| T4 | `dm search <query>` | 9/10 | Anyone who DMs heavily and needs to find an old conversation or promise | FTS5 on `dm_messages.body`; returns thread, sender, date, snippet. Instagram web/app has zero message search capability. | novel — FTS5 on DMs |
| T5 | `posts type-compare <username>` | 8/10 | Creator A/B testing content format; Manager advising on format strategy | SELECT media_type, AVG(likes), AVG(comments) FROM `media` WHERE owner=? GROUP BY media_type. Pure SQLite aggregation. | novel — GROUP BY media_type |
| T6 | `hashtag performance [<username>] [--since 30d]` | 8/10 | Creator optimizing hashtag strategy; Manager reporting hashtag ROI | JOIN `media` × `media_hashtags`; aggregate engagement per hashtag. No Instagram surface exposes this correlation. | novel — cross-table hashtag corr |
| T7 | `comments top-commenters [<username>] [--since 30d]` | 8/10 | Creator identifying superfans for community engagement | SELECT commenter_pk, COUNT(*) FROM `comments` WHERE media owner=? GROUP BY commenter_pk ORDER BY COUNT DESC. Multi-post aggregation impossible via API. | novel — cross-post aggregation |
| T8 | `comments strangers <username> [--since 30d]` | 8/10 | Creator measuring reach beyond current audience; Marketer proving campaign reach | SELECT comments.* FROM `comments` LEFT JOIN `followers` ON comments.commenter_pk=followers.pk WHERE followers.pk IS NULL. 3-table JOIN. | novel — 3-table reach JOIN |

---

## Kill-Check Summary

No absorbed features killed. All 30 map to real confirmed endpoints in `instagram-browser-sniff-spec.yaml` or to the SQLite data layer (sync/search commands).

Absorb kill checks applied per `references/absorb-scoring.md`:
- **LLM dependency:** None require LLM calls.
- **External service:** All call Instagram API or read from local SQLite.
- **Auth gap:** All 30 require session cookie auth, which is the assumed baseline.
- **Scope creep:** No features require scraping third-party sites.
- **Reimplementation:** Sync commands write real API responses to store; no hand-rolled payloads.

Transcendence kill checks:
- **Wrapper vs leverage:** All 8 produce output no single Instagram API call can return (multi-snapshot diff, cross-table JOIN, FTS5 search).
- **Transcendence proof:** All 8 cite specific SQLite mechanism (snapshot diff, LEFT JOIN, GROUP BY, FTS5).
- **Weekly use:** T1–T4 confirmed weekly-or-more by persona; T5–T8 at minimum monthly for active creators.
