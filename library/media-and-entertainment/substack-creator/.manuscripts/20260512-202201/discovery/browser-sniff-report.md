# Substack Browser-Sniff Discovery Report

## Capture Method
- **Mode:** Hybrid (community-source verification + reachability probe)
- **Rationale:** Substack's reverse-engineered API is extensively documented in 4+ active community wrappers (jakub-k-slys/substack-api v4.0.0, postcli/substack, ty13r/substack-mcp-plus, NHagar/substack_api). Endpoint set is well-known; full browser-driven sniff was not needed for endpoint discovery. Crowd-sniff (Phase 1.8) expands this further. User has authenticated session available for Phase 5 live testing.
- **Probe-reachability:** `mode: standard_http` for both `substack.com` and `substack.com/api/v1/publication/search?query=tech`. No WAF or browser-clearance required. Runtime ships standard HTTP.

## Auth Model
- **Type:** Cookie session auth (`cookie` in spec)
- **Required cookies:** `connect.sid` (web session), `substack.sid` (post-login). Both come from a logged-in browser on `*.substack.com`.
- **Login flow:** Web magic-link OTP or password. Programmatic access reads cookies from Chrome's SQLite store (`Cookies` DB).
- **Header pattern:** `Cookie: connect.sid=<value>; substack.sid=<value>`
- **Per-publication scoping:** Many write endpoints target `<subdomain>.substack.com/api/v1/...` (the user's publication subdomain), not just `substack.com`.

## Replayability
**All endpoints below are replayable through standard HTTP** with the session cookie attached. No live page-context execution required. Surf transport not needed.

## Endpoints (consolidated from probe + community sources)
| Path | Method | Auth | Purpose |
|------|--------|------|---------|
| `/api/v1/profile` | GET | cookie | Own profile |
| `/api/v1/profile/{handle}` | GET | optional | Another user's profile |
| `/api/v1/feed` | GET | cookie | Personal feed |
| `/api/v1/feed?tab={for-you,following,categories}` | GET | cookie | Feed by tab |
| `/api/v1/notes` | GET | cookie | Notes feed (cursor pagination) |
| `/api/v1/post/{id}/comments` | GET | optional | Comments on post |
| `/api/v1/comment/feed` | POST | cookie | Add comment |
| `/api/v1/reaction` | POST | cookie | React to post/note/comment |
| `/api/v1/notes/{id}/restack` | POST | cookie | Restack a note |
| `/api/v1/posts/{slug}` | GET | optional/cookie | Single post (paywalled needs cookie) |
| `/api/v1/posts/` | GET | optional | Posts list |
| `/api/v1/publication/search?query=&limit=&page=` | GET | none | Search publications globally |
| `/api/v1/publication/{id}/subscribers` | GET | cookie | Subscribers (own pub only) |
| `/api/v1/publication/{id}/post_count` | GET | cookie | Post count |
| `/api/v1/subscriber/add` | POST | cookie | Add subscriber |
| `/api/v1/free_subscribers/export` | GET | cookie | CSV export of free subs |
| `/api/v1/paid_subscribers/export` | GET | cookie | CSV export of paid subs |
| `/api/v1/drafts` | POST | cookie | Create draft |
| `/api/v1/drafts/{id}` | PUT | cookie | Update draft |
| `/api/v1/drafts/{id}/publish` | POST | cookie | Publish draft |
| `/api/v1/drafts/{id}/preview` | GET | cookie | Author-only preview link |
| `/api/v1/drafts/{id}` | DELETE | cookie | Delete draft |
| `/api/v1/posts` | GET | cookie | Own posts (drafts + published) |
| `/api/v1/post_management/drafts` | GET | cookie | Drafts list |
| `/api/v1/post_management/published` | GET | cookie | Published list |
| `/api/v1/post_management/scheduled` | GET | cookie | Scheduled list |
| `/api/v1/sections` | GET | cookie | Publication categories |
| `/api/v1/image` | POST | cookie | CDN image upload |
| `/api/v1/categories` | GET | none | Global categories |
| `/api/v1/category/{id}/newsletters` | GET | none | Newsletters in category |
| `/api/v1/podcast/{post_id}/audio.m4a` | GET | optional | Podcast audio |
| `/api/v1/publication/{id}/recommendations` | GET | none | Pub recommendations |
| `/api/v1/me/subscriptions` | GET | cookie | What I subscribe to |
| `/api/v1/follows` | GET | cookie | Profiles I follow |
| `/api/v1/dashboard/stats` | GET | cookie | Publication analytics (engagement) |

## Authenticated-only Endpoints
- All `/api/v1/drafts/*`, `/api/v1/post_management/*`, `/api/v1/publication/{id}/subscribers`, `/api/v1/subscriber/add`, `/api/v1/free_subscribers/export`, `/api/v1/paid_subscribers/export`, `/api/v1/dashboard/stats`, `/api/v1/me/*`
- `/api/v1/reaction`, `/api/v1/comment/feed`, `/api/v1/notes/*` (write operations)

## Rate Limiting
- Empirically: ~2 req/s safe (sbstck-dl default). No documented hard limit. Recommend adaptive limiter with exponential backoff on 429.

## Known Volatility
- **Post scheduling endpoint changed in 2025** (ty13r/substack-mcp-plus v1.0.3 had to remove). Treat scheduling as best-effort; surface upstream errors.

## Notes for Generator
- `auth.type: cookie` with two cookies (`connect.sid`, `substack.sid`), domain `substack.com`
- `http_transport: standard` (not browser-clearance)
- Multiple base URLs: global `substack.com/api/v1` AND per-publication `<subdomain>.substack.com/api/v1`. The spec should let users configure `--publication <subdomain>` per command.
