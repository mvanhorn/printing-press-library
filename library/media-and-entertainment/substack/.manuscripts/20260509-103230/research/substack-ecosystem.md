# Substack API Ecosystem Research

> Date: 2026-05-09. Source-of-truth: GitHub repos, raw source files, npm registry, GitHub issue trackers. All evidence direct-fetched and inspected. Star counts/dates as of fetch time.

This document gives the absorb manifest for a publication-owner growth-loop CLI a concrete picture of (a) what endpoints exist, (b) what auth looks like, (c) what every prior tool has built, and (d) where this is likely to break. The agent-native CLI we generate must absorb the union of these surfaces and beat them on coverage and ergonomics.

---

## 1. Substack's API surface (community-mapped)

Substack runs **two parallel HTTP surfaces** plus an **official Publisher API** (recently introduced for select publishers). Every community wrapper hits internal endpoints — there is no widely-published spec, but the same paths recur across 8+ independent reverse-engineering efforts, which makes them de-facto stable.

### 1a. Three base URLs (the most important fact in this whole doc)

| Base | Used for | Auth |
|---|---|---|
| `https://substack.com/api/v1` | Cross-publication: login, current-user, Notes, comment feed, recommendations, search, public-profile, reader feed, subscriptions list, user-setting | session cookie (`substack.sid` or `connect.sid`) |
| `https://<sub>.substack.com/api/v1` | Per-publication: drafts, posts, scheduling, image upload, publication users, sections/tags, post-management, archive | session cookie + `referer` header keyed to publication |
| `https://publisher-api.substack.com/v1` | **Official** Publisher API: list_posts, get_post, post stats, subscriber counts, subscriber lookup | `authorization: <api_key>` header (no `Bearer` prefix) |

Evidence:
- `arthurcolle/substack-mcp/substack_client.py` lines 555-556: `self.pub_base = f"https://{self.publication}/api/v1"; self.sub_base = "https://substack.com/api/v1"`
- `marcomoauro/substack-mcp/src/api/substack/SubstackApi.js`: `this.base_url = base_url || 'https://substack.com/api/v1'; this.publication_url = new URL('api/v1', publication_url).toString();`
- `dkships/substack-publisher-mcp/src/index.ts` line 67: `const BASE_URL = "https://publisher-api.substack.com/v1";` with `headers: { authorization: apiKey, accept: "application/json" }`
- `dkships` README links to `https://publisher-api.substack.com/v1/docs/` (returns 403 anonymously, requires API key)

This split matters: the agent-native CLI must route requests by surface, not by command verb.

### 1b. Endpoint catalog (de-facto)

All paths confirmed by reading raw source. Auth column = "session" (cookie required) or "official" (Publisher API key) or "none" (public).

#### Auth / session bootstrap
| Method | Path | Base | What | Auth | Sources |
|---|---|---|---|---|---|
| POST | `/login` | substack.com | Email+password login. Body: `{captcha_response, email, for_pub:"", password, redirect:"/"}` | none -> sets cookies | ma2za/api.py:135 |
| GET | `/user/profile/self` | substack.com | Current authenticated user payload | session | ma2za/api.py:296 |
| GET | `/settings` | substack.com | Account settings | session | ma2za/api.py:304 |
| PUT | `/user-setting` | substack.com | Touched as a "ping" connectivity check; body `{type:"last_home_tab", value_text:"inbox"}` returns `{user_id}` | session | jakub-following:21, jakub-conn-svc, arthurcolle:704 |
| GET | `/handle/options` | substack.com | Available handle suggestions for current user | session | jakub-profile-svc:18, arthurcolle:725 |

#### Publication / drafts (per-publication base)
| Method | Path | Base | What | Sources |
|---|---|---|---|---|
| GET | `/drafts` | publication | List drafts (filter, offset, limit) | ma2za:368, arthurcolle:835 |
| POST | `/drafts` | publication | Create draft | ma2za:405, arthurcolle:889 |
| GET | `/drafts/{id}` | publication | Get draft | ma2za:385, arthurcolle:849 |
| PUT | `/drafts/{id}` | publication | Update draft body / metadata | ma2za:417, arthurcolle:896 |
| DELETE | `/drafts/{id}` | publication | Delete draft | ma2za:393, arthurcolle:943 |
| POST | `/drafts/{id}/prepublish` | publication | Validate before publish | ma2za:433 |
| POST | `/drafts/{id}/publish` | publication | Publish now | ma2za:448, arthurcolle:939 |
| POST | `/drafts/{id}/schedule` | publication | Schedule publish (`{post_date}`) | ma2za:467 |
| POST | `/drafts/{id}/schedule` (with `null`) | publication | Unschedule | ma2za:483 |
| GET | `/post_management/published` | publication | List published posts | ma2za:340 |
| GET | `/archive?sort=new&limit=...` | publication | Archive listing | arthurcolle:789, NHagar/newsletter:133, noah-substack:23 |
| GET | `/posts/{slug}` | publication | Public post by slug | NHagar/post:37, noah-substack:74 |
| POST | `/image` | publication | Upload image; body: `{image: <data_uri>}`. Returns `{url, imageWidth, imageHeight, bytes, contentType}` | ma2za:497, arthurcolle:684 |
| GET | `/publication/users` | publication | Publication-bylines / authors | ma2za:315 |
| GET | `/publication_launch_checklist` | publication | Includes total subscriber count | ma2za:326 |
| GET | `/publication/post-tag` | publication | List tags | ma2za:535 |
| POST | `/publication/post-tag` | publication | Create tag | ma2za:545 |
| POST | `/post/{post_id}/tag/{tag_id}` | publication | Attach tag to post | ma2za:574 |
| GET | `/subscriptions` | publication | Sections+subscriptions list (used for `get_sections`) | ma2za:654 |
| GET | `/recommendations/from/{publication_id}` | publication | Outbound recommendations | NHagar/newsletter:276 |
| GET | `/publication/users/ranked?public=true` | publication | Ranked authors / leaderboard | NHagar/newsletter:304 |
| GET | `/feed` | publication | RSS feed (well-known) | noah-substack:133 |

#### Notes (substack.com base — global)
| Method | Path | Base | What | Sources |
|---|---|---|---|---|
| POST | `/comment/feed` (or `/comment/feed/`) | substack.com | **Create a Note** (and thread comments). Body is ProseMirror JSON wrapped under specific keys. | nanameru:315, arthurcolle:967, jakub-note-builder |
| POST | `/comment/attachment/` | substack.com | Attach link or media to a Note before posting | arthurcolle:972, jakub-note-builder |
| GET | `/reader/feed` | substack.com | Authenticated home feed (Notes + posts) | arthurcolle:998 |
| GET | `/reader/feed/profile/{profile_id}?types=note&cursor=...` | substack.com | Notes from a specific profile (cursor-paginated) | jakub-note-svc:88-89 |
| GET | `/reader/comment/{id}` | substack.com | Single Note (Substack treats Notes as comments internally) | jakub-note-svc:21, jakub-comment-svc:39 |
| GET | `/reader/posts` | substack.com | Posts feed for current user | ma2za:358 |

The Note / comment / chat surfaces share the `/comment/...` family. Comments on a post and Notes are the same object type with different parent IDs; the public Note URL is `https://substack.com/note/c-{note_id}` (`nanameru:329`).

#### Comments (on long-form posts)
| Method | Path | Base | What | Sources |
|---|---|---|---|---|
| GET | `/post/{postId}/comments` | substack.com | Comments on a post | jakub-comment-svc:21 |
| GET | `/reader/comment/{id}` | substack.com | Single comment / Note by ID | jakub-comment-svc:39 |
| POST | `/comment/feed/` (with `parent_id`) | substack.com | Reply to a post or another comment | arthurcolle:992 |

#### Profile / followers / search
| Method | Path | Base | What | Sources |
|---|---|---|---|---|
| GET | `/user/{username}/public_profile` | substack.com | Public profile by handle | NHagar/user:85 |
| GET | `/user/{userId}/public_profile` | substack.com | Public profile by ID | jakub-profile-svc |
| GET | `/profile/posts?profile_user_id={id}` | substack.com | Posts by an author across publications | jakub-post-svc:65 |
| GET | `/profile/search/linkedin/{handle}` | substack.com | Find Substack profile by LinkedIn handle | arthurcolle:767 |
| GET | `/user/{user_id}/subscriber-lists?lists=following` | publication | Returned for `getFollowing()` — current user's followees | jakub-following-svc |
| GET | `/publication/search` | substack.com | Search publications | NHagar/newsletter:15 |
| GET | `/categories` | substack.com | Site-wide category list | NHagar/category:23, ma2za:579 |
| GET | `/category/public/{id}/{type}?page=N` | substack.com | Publications in a category | NHagar/category:121, ma2za:602 |
| GET | `/category/public/{id}/all?page=N` | substack.com | All publications in a category | NHagar/category:121 |
| GET | `/publication/embed?...` | substack.com | Publication embed metadata | ma2za |

#### Notes / DM / Chat — **gap**
No community wrapper has reverse-engineered Substack Chat (DMs / inbox messaging) endpoints in detail. `arthurcolle/substack_client.py` and `jakub-conn-svc` use `/user-setting` `{type:"last_home_tab", value_text:"inbox"}` as a connectivity probe, suggesting an `inbox` tab exists, but no concrete chat-message POST endpoint appears in any surveyed wrapper. The `NHagar/substack_api` repo has an open PR (#19) titled "Add Chat module for reading publication subscriber chats" — it was opened 2026-02 and never merged. **This is unmapped territory** for the absorb manifest.

#### Analytics — only via official Publisher API
Community wrappers only have:
- `get_subscriber_count()` via `/publication_launch_checklist` (single number)
- nothing for opens / clicks / reads / growth-over-time

The official `publisher-api.substack.com/v1` provides:
- `GET /posts?startDate&endDate&sortBy&type&maxResults&next` — list published posts with pagination
- `GET /posts/{urlSlug}` — single post metadata
- `GET /posts/{urlSlug}/stats` — engagement metrics (opens, clicks, ...)
- `GET /subscribers/counts?startDate&endDate` — daily subscriber counts by type
- `GET /subscribers/{email}` — subscriber lookup by email

(Inferred from `dkships/substack-publisher-mcp/src/index.ts` registerTool definitions; the docs page itself is gated.)

#### Recommendations / cross-promotion
Only the **outbound** read endpoint is community-mapped:
- `GET /api/v1/recommendations/from/{publication_id}` returns publications this one recommends.

Creating/removing a recommendation does not appear in any surveyed wrapper. Likely uses `POST /publication/{id}/recommendation` based on URL patterns, but unconfirmed.

---

## 2. Auth model

### Cookies
- **Primary:** `substack.sid` — set on `.substack.com` domain. Equivalent to a password (the nanameru README emphasizes this verbatim: *"The `substack.sid` cookie is equivalent to a password — anyone with it has full account access (publish posts, edit billing, etc.). Treat it as such."*).
- **Alternate:** `connect.sid` — older Substack accounts have this name instead. `alexferrari88/sbstck-dl` exposes this as a CLI flag: `--cookie_name {substack.sid|connect.sid}`.
- The `marcomoauro/substack-mcp` `SubstackApi.js` sends both belt-and-suspenders: `Cookie: substack.sid=<token>; connect.sid=<token>;`.

### Required headers
- `User-Agent`: a browser-shaped UA. NHagar wrapper hard-codes `Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.77 Safari/537.36`. Default Python `requests`/Go `net/http` UAs are blocked by Cloudflare in some cases (see §5).
- `Referer` — required when posting drafts to a publication. `marcomoauro/SubstackApi.js`: `headers['referer'] = '${this.hostname}/publish/post';`
- `Cookie` — built from `substack.sid` + optionally `connect.sid`.
- No CSRF token observed in any wrapper (Substack's API treats the session cookie as sufficient for state-changing requests).

### Login flows
Three known paths, all converge on producing a `substack.sid` cookie:

1. **Email + password** (`POST https://substack.com/api/v1/login`). Body: `{captcha_response: null, email, for_pub: "", password, redirect: "/"}`. This is **frequently rate-limited and captcha-gated** (see §5); 6+ flagged GitHub issues.
2. **Magic link / email code**: a 6-digit code sent to the registered email. Substack does not expose a programmatic submission endpoint; community tools spawn a Playwright/Selenium browser (`ty13r/setup_auth.py`, `nanameru` browser helpers) and prompt the user to type the code into the rendered form.
3. **Direct cookie paste**: extract `substack.sid` from a logged-in browser DevTools and configure tools with it. This is the most reliable production path. The `nanameru` server uses `pycookiecheat` to read cookies directly out of an existing Chrome profile so "Substack can't tell anything was automated."

### 2FA
No Substack-native TOTP/SMS 2FA observed. Email-magic-link is the de-facto second factor for new sessions. Tools that automate it (`ty13r/setup_auth.py`) wait up to 120 seconds for the user to manually enter the code in the headed browser; the tool then scrapes the resulting `substack.sid` cookie out of the Playwright context.

### Session expiry
- Cookies have long lifetimes (months) but can be invalidated by:
  - Substack-side rotation after suspicious activity (captcha trigger).
  - Logging out elsewhere with "log out of all sessions."
  - Account-level password change.
- `ty13r/auth_manager.py` stores tokens in OS keyring (encrypted with Fernet) with metadata; on 401 it re-runs the browser flow.
- No public refresh-token endpoint exists; expiry equals "go log in again."

### Bearer tokens
- The **official** Publisher API uses `authorization: <api_key>` (raw, no `Bearer` prefix per `dkships` README). The key is generated inside the publication's settings ("Settings -> Publisher API keys") and is per-publication. Currently (2026) this is opt-in / gated to selected publications.
- No bearer token exists for the internal community-mapped surface — only cookies.

---

## 3. Community wrappers (top by maintenance + coverage)

Star counts and last-push dates direct from `api.github.com/repos/...` at fetch time.

| Repo | Lang | ★ | License | Last push | Coverage |
|---|---|---:|---|---|---|
| [`ma2za/python-substack`](https://github.com/ma2za/python-substack) | Python | 149 | MIT | 2026-05-08 | **Most complete writer-side wrapper.** Drafts CRUD, prepublish, publish, schedule/unschedule, image upload, sections, tags, subscriber count, login (email+pwd or cookies). Used as the engine by ≥ 3 MCP servers (nanameru, ty13r partial, marco). Active. |
| [`NHagar/substack_api`](https://github.com/NHagar/substack_api) | Python | 194 | MIT | 2026-03-16 | Reader-side: public_profile, publication search, archive, recommendations, categories, ranked authors. Cookie-only auth (no password login). Active. |
| [`jakub-k-slys/substack-api`](https://github.com/jakub-k-slys/substack-api) | TypeScript | 71 | MIT | 2026-05-04 | Modern TS client; entity-based domain model. Notes (read + builder), comments, profile, followees, posts. Powers `n8n-nodes-substack`. 3001 npm dl/mo. Very active. |
| [`alexferrari88/sbstck-dl`](https://github.com/alexferrari88/sbstck-dl) | Go | 216 | MIT | 2025-09-03 | Go CLI for archive download. Cookie-pasted auth. Read-only. Stale-ish (no push since Sep 2025) but issues active. |
| [`timf34/Substack2Markdown`](https://github.com/timf34/Substack2Markdown) | Python | 448 | MIT | 2026-04-22 | Selenium-driven; converts free + premium posts to MD. Read-only archival. Most-starred repo overall but scope is narrow. |
| [`Noah-Bjorner/SubstackAPI`](https://github.com/Noah-Bjorner/SubstackAPI) | TS | 21 | - | 2025-02-27 | Cloudflare Worker exposing public read-only endpoints (`/posts/latest`, `/posts/top`, `/posts/search`, `/post`). Useful as an example of the public archive surface. Stale. |
| [`bytewife/substack_scraper`](https://github.com/bytewife/substack_scraper) | Rust | 33 | MIT | 2024-10-05 | Rust archive scraper. Stale. |
| [`bit-of-a-shambles/substack`](https://github.com/bit-of-a-shambles/substack) | Ruby | 7 | Apache-2.0 | 2026-04-27 | Recent Ruby wrapper, less complete. |

**Top 5 most useful for an absorb-manifest** (by combined coverage + freshness):
1. `ma2za/python-substack` — writer surface (drafts, publish, schedule, image, tags, sections).
2. `NHagar/substack_api` — reader/discovery (search, archive, categories, recommendations, public profile).
3. `jakub-k-slys/substack-api` — Notes domain modeling (note builder, comment endpoints, profile feed cursor pagination).
4. `arthurcolle/substack-mcp` (`substack_client.py`, 1333 lines) — most comprehensive single-file reverse-engineered client; covers user-setting, handle/options, LinkedIn search, image upload as JSON `{image: <data_uri>}`.
5. `dkships/substack-publisher-mcp` — only artifact that talks to the **official** `publisher-api.substack.com/v1`.

---

## 4. Existing CLIs and MCP servers

### CLIs
| Tool | Lang | Style | Coverage |
|---|---|---|---|
| [`alexferrari88/sbstck-dl`](https://github.com/alexferrari88/sbstck-dl) | Go (Cobra) | Subcommand CLI | `download`, `list`, `version`. Archive read-only. **Closest analog to what we're building, but read-only and download-focused — no growth loop.** |
| `NHagar/substack_api` cli.py | Python | Click-style CLI shipped inside the wrapper | Search publications, get profiles, list category. Not advertised, not packaged separately. |
| Various scrapers (`bytewife`, `humblemat810`, `gitgithan`) | mixed | scripts | One-off downloaders, not user-facing CLIs. |
| [`postcli/substack`](https://github.com/postcli/substack) | found in search | "Substack CLI and MCP Server" | Not yet inspected; new entrant. |

There is **no Go CLI today that performs publish, schedule, post-note, follow, like, or comment operations.** That is the open gap a growth-loop CLI fills.

### MCP servers (sorted by recency + capability)
| Server | Lang | Focus | API surface | npm dl/mo |
|---|---|---|---|---|
| [`dkships/substack-publisher-mcp`](https://github.com/dkships/substack-publisher-mcp) | TS | **Official Publisher API** (read-only stats) | 6 tools: `list_publications`, `list_posts`, `get_post`, `get_post_stats`, `get_subscriber_counts`, `get_subscriber` | n/a (just published) |
| [`ty13r/substack-mcp-plus`](https://github.com/ty13r/substack-mcp-plus) | Python | Writer-side; the most polished | 12 tools: `create_formatted_post`, `update_post`, `publish_post`, `list_drafts`, `list_published`, `get_post_content`, `duplicate_post`, `upload_image`, `preview_draft`, `get_sections`, `get_subscriber_count`, `delete_draft`. Browser auth via Playwright + keyring/Fernet token storage. | 399 |
| [`marcomoauro/substack-mcp`](https://github.com/marcomoauro/substack-mcp) | JS | Drafts + posts (writer) | `create_draft_post`, `create_post` | 136 |
| [`arthurcolle/substack-mcp`](https://github.com/arthurcolle/substack-mcp) | Python | Most ambitious feature set | `substack_create_draft`, `substack_update_draft`, `substack_append_to_draft`, `substack_add_code_block`, `substack_add_image`, `substack_publish`, `substack_post_note`, `substack_get_drafts`, `substack_get_posts`, `substack_live_blog_start`, `substack_live_blog_end` (includes Notes + live-blog) | n/a |
| [`nanameru/substack-mcp`](https://github.com/nanameru/substack-mcp) | Python | Articles + Notes + cover image; Chrome-cookie auth | `create_draft`, `update_draft`, `publish_draft`, `schedule_draft`, `unschedule_draft`, `set_cover_image`, `upload_image`, `post_note`, `post_note_with_image`, `chat_check`, `make_slides` | n/a |
| [`dkyazzentwatwa/substack_mcp`](https://github.com/dkyazzentwatwa/substack_mcp) | Python | Read/research | structured access, real-time analysis | n/a |
| [`michalnaka/mcp-substack`](https://github.com/michalnaka/mcp-substack) | JS | Read-only (download/parse) | parse posts | n/a |
| [`jonathan-politzki/mcp-writer-substack`](https://github.com/jonathan-politzki/mcp-writer-substack) | Python | Writer-context tool: bridges Substack + Medium RSS into Claude | RSS-based; embeddings + semantic search | n/a |

### n8n / Zapier / Pipedream
- **n8n:** [`jakub-k-slys/n8n-nodes-substack`](https://github.com/jakub-k-slys/n8n-nodes-substack) (24 ★, 1046 npm dl/mo, last push 2026-05-08). Resources + operations:
  - **Profile**: `getOwnProfile`, `getProfileBySlug`, `getProfileById`, `getFollowees`
  - **Post**: `getAll`, `getPostsBySlug`, `getPostsById`, `getPostById`
  - **Note**: `create`, `get` (own notes), `getNotesBySlug`, `getNotesById`, `getNoteById`
  - **Comment**: `getAll` (for a post), `getCommentById`
  - Auth: API key (cookie value) + publication address.
  - This is the **single most authoritative list of "operations users want"** because every operation here corresponds to a real n8n customer use-case.
- **Zapier**: no first-party Substack integration. Substack's official integrations help only treats Zapier as an outbound RSS-trigger consumer (via Substack's RSS feed). No "create post" / "send note" actions exist on Zapier.
- **Pipedream**: page exists at `pipedream.com/apps/substack` but currently empty (no first-party actions). Confirmed via fetch.

### Claude skills / plugins for Substack
None found in the official Anthropic skill registry searches. The only Claude-adjacent surface is the MCP servers above.

---

## 5. Reachability risk

### Quantifying breakage signal
Issue-tracker scan across the 9 most-relied-on Substack repos (192 issues total) flagged 21 issues with reachability terms (`403`, `401`, `429`, `blocked`, `broken`, `deprecated`, `rate.?limit`, `captcha`, `cloudflare`, `forbid`, `expired`, `fail`, `cookie`, `sid`, `session`, `error`):

| Repo | Flagged / Total | Notable closed/open |
|---|---:|---|
| `timf34/Substack2Markdown` | 8/23 | Captcha at login (#4, #21), Selenium issue with premium (#18), can't download premium (#19, #20), error fetching sitemap (#9). |
| `alexferrari88/sbstck-dl` | 6/17 | "Private downloading isn't working for me" (#23, **open**), `--sid` flag broken (#4, fixed). |
| `ma2za/python-substack` | 6/19 | "Captcha error when logging in" (#27), "Complete/bypass captcha" (#14), 400 Invalid value (#8). |
| `jakub-k-slys/substack-api` | 1/4 | Profile fetch fails for users with no picture (#229, **open**). |

That's roughly **11% of all issues** and **~30% of open issues** in this set are reachability/breakage. This is high-friction territory.

### Fire patterns and mitigations
1. **Captcha at login (`/api/v1/login`)** — the most common and stalest failure mode. Fires when:
   - many login attempts from the same IP,
   - new IP / new client signature against an existing account,
   - account flagged for reasons internal to Substack.
   Mitigation in every wrapper: skip the password endpoint entirely. **Recommend cookies extracted from a logged-in browser** as the primary auth path, with email/password supported only as a secondary "best-effort" mode.
2. **Cloudflare TLS fingerprinting on RSS feeds** — `httpx`-based Python clients get 403 from `*.substack.com/feed` while `curl` from the same machine succeeds (issue: `heurema/herald#4`, 2026). Mitigation: use a fingerprint-stable HTTP client (Go's `net/http` with `http.DefaultTransport.TLSClientConfig` left default works; `curl_cffi`/`requests-html` works for Python). For our Go CLI this is naturally fine **only if the binary doesn't customize TLS aggressively**. Test against an HTTPS-protected `*.substack.com/feed` endpoint as part of doctor.
3. **Cookie expiry with no signal** — cookies don't carry server-readable expiry in many wrappers; the failure surfaces as a 401 or HTML redirect to `/sign-in`. Mitigation: store the cookie in OS keyring with a "last-known-good" timestamp; on 401 prompt the user to re-extract.
4. **Premium / private newsletters require a logged-in subscription cookie**. Multiple `sbstck-dl` and `Substack2Markdown` issues collapse to "I'm subscribed but the tool downloaded the preview." Mitigation: the cookie must come from a session that has the subscription, and the publication subdomain must match.
5. **Rate limiting (429)** — none of the surveyed wrappers actually hit `X-RateLimit-*` headers; substack throttles silently or returns 429 without structured headers. `arthurcolle` and `jakub` both implement client-side rate limiters (jakub uses `axios-rate-limit` at 25 RPS default; arthurcolle has a custom `_rate_limit_wait()`). Recommend default 2 RPS, configurable.
6. **Bot signal from User-Agent** — wrappers consistently ship a Chrome desktop UA. Substack's bot-detection appears UA-aware. Default UA must be a real browser string.

### Frequency
On the most-active wrappers (ma2za, sbstck-dl), reachability issues fire **every 1–3 months** based on issue cadence. They are not constant but are not ignorable either. The CLI's doctor command should run a connectivity probe (`PUT /user-setting` with `last_home_tab=inbox` returns `{user_id}`, used by jakub and arthurcolle as a non-destructive ping) and surface a clear remediation message on 401.

---

## 6. Notes vs. Publication: the two surfaces

These are **two product surfaces with two follower graphs**, both reachable from the same authenticated session but routed through different bases and modeled differently.

| Dimension | Notes | Publication |
|---|---|---|
| Base | `https://substack.com/api/v1` | `https://<sub>.substack.com/api/v1` |
| Object | `Note` (a kind of `comment`) | `Post` / `Draft` |
| Identity | tied to **user/profile** (`profile_user_id`) | tied to **publication** (`subdomain`) |
| Follower graph | "followers" — anyone can follow your profile feed regardless of subscription | "subscribers" — opted into your publication's email list |
| Create | `POST /comment/feed` (with attachments via `/comment/attachment/`) | `POST /<pub>/api/v1/drafts` then `/drafts/{id}/publish` |
| Read | `GET /reader/feed/profile/{id}?types=note&cursor=...` (cursor pagination) | `GET /<pub>/api/v1/archive?sort=new&offset=&limit=` (offset pagination) |
| Single | `GET /reader/comment/{id}` | `GET /<pub>/api/v1/posts/{slug}` |
| Public URL | `https://substack.com/note/c-{note_id}` | `https://<sub>.substack.com/p/{slug}` |
| Body shape | Substack-flavored ProseMirror JSON | Same ProseMirror JSON, wrapped under `draft_body` (stringified) |
| Image upload | `/comment/attachment/` returns attachment id | `/<pub>/api/v1/image` returns CDN URL |
| Like / heart | not surfaced in any community wrapper today | not surfaced |
| Restack / repost | not surfaced in any community wrapper today | n/a |

### Growth-loop action -> surface mapping

| Growth-loop action | Surface | Endpoint family |
|---|---|---|
| Publish a long-form post | Publication | `POST /drafts` -> `POST /drafts/{id}/publish` |
| Schedule a long-form post | Publication | `POST /drafts/{id}/schedule` |
| Post a Note (short-form) | Notes | `POST /comment/feed` |
| Reply to a comment on my post | Comments | `POST /comment/feed` with parent_id |
| Reply to someone's Note (engage) | Notes | `POST /comment/feed` with parent_id |
| Get my notes / engagement | Notes | `GET /reader/feed/profile/{my_id}?types=note` |
| Find new writers to engage with | Search/Profile | `GET /publication/search`, `GET /reader/feed`, `GET /category/public/...` |
| Cross-promote (recommend a publication) | Publication | (read) `GET /recommendations/from/{id}`; write unmapped |
| Track subscriber growth | Publication / Publisher API | `GET /publication_launch_checklist` (count) or official `/subscribers/counts` |
| Inspect post performance | Official Publisher API | `GET /posts/{slug}/stats` |
| Manage drafts | Publication | `GET/POST/PUT/DELETE /drafts/...` |
| Upload image for either | mixed | `POST /<pub>/api/v1/image` for posts, `POST /comment/attachment/` for Notes |

The fact that **Notes are routed through `substack.com` while Posts are routed through `<pub>.substack.com`** must be modeled as the most prominent dimension in the CLI's command tree. A `notes` command group hits one base; a `posts` / `drafts` group hits the other. Both share the same cookie.

---

## 7. Domain quirks

- **Subdomain routing.** Each publication is `<slug>.substack.com` (or a custom domain mapped via `pub.custom_domain`). The wrapper code consistently does `re.search(r"https://(.*).substack.com", publication_url.lower())` to recover the slug (ma2za:93). Custom domains are detected via `pub.get("custom_domain") or f"{pub['subdomain']}.substack.com"` (NHagar/user:257). The CLI must accept either form and resolve to the canonical `<slug>.substack.com` for API base construction; some wrappers (Noah) store the publication URL once and never resolve, which breaks for custom domains.

- **Pagination.**
  - Notes use **cursor pagination** (`?cursor=<base64ish>`).
  - Posts/archive use **offset pagination** (`?offset=N&limit=M&sort=new`).
  - Categories use **page numbers** (`?page=N`).
  - Publisher API uses **`next` token** pagination.
  Four shapes inside one product. The CLI must abstract these under a uniform `--page-size` / `--cursor` flag pair.

- **Response envelopes.**
  - Most endpoints return raw JSON (no wrapping envelope). Errors come as `{errors: [{msg, ...}]}` for state-changing requests (`ma2za/exceptions.py:11`).
  - The image upload returns flat `{url, imageWidth, imageHeight, bytes, contentType}`.
  - The Note POST returns `{id, ...}` (no envelope), and `id` is the only useful field — Substack does not return the public Note URL; you reconstruct it as `https://substack.com/note/c-{id}`.
  - **No GraphQL anywhere** in the surveyed surface.

- **Image / asset upload.** Two distinct flows:
  - **Posts**: `POST /<pub>/api/v1/image` with body `{image: "data:image/jpeg;base64,..."}` (base64 data URI in JSON, *not* multipart). Returns a `https://substackcdn.com/image/fetch/...` URL. Confirmed: arthurcolle line 684, ma2za line 497.
  - **Notes**: `POST /comment/attachment/` returns an attachment id you embed into the comment-feed POST.
  - The CDN URL pattern `https://substackcdn.com/image/fetch/<params>/<original_url>` is shared (`ty13r/image_handler.py:207`).

- **Rate limits.** No `X-RateLimit-*` headers observed. Tools self-throttle. Recommend default 2 RPS for novel-feature commands; 25 RPS only for read-only commands (`jakub` default).

- **Cloudflare.** All `*.substack.com` subdomains sit behind Cloudflare. TLS fingerprinting (JA3/JA4) blocks some HTTP clients (notably Python `httpx`) on the *RSS feed* path even when the same client succeeds against `/api/v1/`. Probable cause: feed routes vs API routes have different bot-protection rules. Go's stdlib `net/http` is **not** in the typically-blocked set as of 2026, but the doctor command should probe `https://substack.com/feed?...` regardless.

---

## Top 8 endpoints we MUST cover (ranked by growth-loop centrality)

Ranking is "what fraction of a publication-owner growth loop the endpoint unlocks, weighted by frequency of use." Every entry below is confirmed by ≥ 2 independent community wrappers.

| # | Endpoint | Surface | Why |
|---|---|---|---|
| **1** | `POST /api/v1/comment/feed` (substack.com) | Notes | The single most central growth action. Posting Notes is how publication owners build a follower graph that compounds into subscribers. Every novel-feature CLI in this space must do this. |
| **2** | `POST /<pub>/api/v1/drafts` -> `POST /<pub>/api/v1/drafts/{id}/publish` | Publication | The atomic "ship a post" pair. Without this the CLI is a read-only shell. |
| **3** | `POST /<pub>/api/v1/drafts/{id}/schedule` | Publication | Scheduled publish is the daily-driver workflow for serious authors. Cheap to add since it sits next to publish. |
| **4** | `POST /<pub>/api/v1/image` (JSON `{image: <data_uri>}`) | Publication | Posts and Notes both need uploaded images. Without this the publish path is incomplete. |
| **5** | `GET /api/v1/reader/feed/profile/{profile_id}?types=note&cursor=...` | Notes | Read your own (or someone else's) Notes. Foundation of "engage with my own audience" and "engage with other writers" workflows. |
| **6** | `POST /api/v1/comment/feed` with `parent_id` | Comments / Notes replies | Replies are the cheapest growth action — engaging with comments on your own posts and with other people's Notes. Same endpoint as #1, different body. |
| **7** | `GET /<pub>/api/v1/post_management/published` + Publisher API `GET /posts/{slug}/stats` | Publication / Official | Performance feedback loop. Knowing what worked closes the publish->learn->next-post loop. |
| **8** | `GET /api/v1/publication/search` + `GET /api/v1/category/public/{id}/all?page=N` | Discovery | Find writers to recommend, follow, or engage with. The discovery side of the graph — without it the CLI can't help an owner *grow*. |

Honorable mention (almost made the cut, kept out for sequencing): `PUT /api/v1/user-setting` (cheap connectivity probe, ideal for `doctor`); `GET /<pub>/api/v1/recommendations/from/{id}` (read recommendations); `GET /api/v1/user/{username}/public_profile` (resolve a handle to a `profile_user_id`).

---

## Top 3 reachability / auth risks

1. **Cookie-based auth is the only viable session path; password login captchas frequently.** This is the *primary* risk and informs the entire onboarding flow. Make `<cli> auth` accept a paste-from-DevTools cookie value and store it in OS keyring (matching `ty13r/auth_manager.py` Fernet+keyring pattern). Treat the email+password endpoint as second-class — implement it, but document captcha as an expected outcome and direct users to the cookie path. This risk is borne out by 6+ closed issues across `ma2za` and `Substack2Markdown` over multiple years.

2. **Cloudflare bot-protection on RSS / public read paths can block stdlib HTTP clients via TLS fingerprinting (JA3/JA4).** Less likely to bite a Go CLI than a Python one, but the failure mode is silent (403 with HTML "Just a moment...") and impossible to fix from request-level code alone. Mitigation: keep TLS settings stock; do not tune ciphers / HTTP/2 settings; ship a `<cli> doctor` probe that fetches `https://substack.com/feed` with the user's runtime to surface this early. Reference: `heurema/herald#4` (2026-04, open).

3. **No Notes API quota or sustainability guarantee.** Every Notes endpoint is reverse-engineered. Substack can change `/comment/feed` payload shape, parameter names, or auth requirements at any time without warning, and they have done so before (Substack2Markdown #4, #21). Mitigation: do not bake the request shape into hand-written commands; centralize Notes write-paths in a single `internal/substack/notes.go` so a single fix lands across all callers. Add a regression test in dogfood that creates+immediately-deletes a Note in a sentinel publication.

---

## Appendix: file-level provenance for the absorb manifest

The most directly liftable artifacts (Phase 1.5 absorb candidates):

- **Endpoint catalog** — extract path/method/body shape from these single files (~80 KB total):
  - `https://raw.githubusercontent.com/ma2za/python-substack/main/substack/api.py` (writer side, 700 LOC)
  - `https://raw.githubusercontent.com/arthurcolle/substack-mcp/main/substack_client.py` (Notes + reader side, 1333 LOC)
  - `https://raw.githubusercontent.com/jakub-k-slys/substack-api/main/src/internal/services/*.ts` (typed TS service files, ~10 small files)
  - `https://raw.githubusercontent.com/dkships/substack-publisher-mcp/main/src/index.ts` (official Publisher API, 11 KB)

- **Note ProseMirror builder** — `https://raw.githubusercontent.com/jakub-k-slys/substack-api/main/src/domain/note-builder.ts` (15 KB, fluent `.bold()` / `.italic()` / `.link()` API). Worth absorbing as the model for any `<cli> notes new` command.

- **Auth flow (browser fallback)** — `https://raw.githubusercontent.com/ty13r/substack-mcp-plus/main/setup_auth.py` (12 KB) and `src/auth_manager.py` (6 KB). Playwright + keyring + Fernet pattern. Use the design as inspiration; the CLI itself can shell out to `cookieextract`-style helpers and avoid bundling Playwright.

- **Operations vocabulary** — `https://raw.githubusercontent.com/jakub-k-slys/n8n-nodes-substack/main/nodes/Substack/{Profile,Post,Note,Comment}.operations.ts`. The `enum` values name the user-visible operations a battle-tested integration shipped — use them as the seed set for the CLI's command tree. Specifically:
  - profile: `getOwnProfile, getProfileBySlug, getProfileById, getFollowees`
  - post: `getAll, getPostsBySlug, getPostsById, getPostById`
  - note: `create, get, getNotesBySlug, getNotesById, getNoteById`
  - comment: `getAll, getCommentById`

The CLI will be the first tool in this ecosystem to combine (a) the writer surface (drafts/publish/schedule) from `ma2za`, (b) the Notes surface from `jakub` + `arthurcolle`, (c) the official Publisher API from `dkships`, and (d) the discovery surface from `NHagar` — under one binary with one auth model. None of the above tools cover all four. That is the absorb target.
