# Naver Blog — Browser-Sniff Discovery Report

**Run:** 20260519-162524
**Backend:** browser-use 0.12.7 (Chrome headless) + direct curl probes
**Canary URLs (real existing data from Chilly cron):**
- `https://m.blog.naver.com/selly9401/224234460263` — 칠리여성청결제 post (5 likes, 0 comments)
- `https://m.blog.naver.com/koko_yun/224230858514`
- `https://m.blog.naver.com/perfect62/224238091312`
- Plus search canary: `https://m.search.naver.com/search.naver?where=m_view&query=칠리+여성청결제`

**Auth posture:** All endpoints below verified anonymous (no `NAVER_CLIENT_ID`, empty cookie jar, no Referer/UA required for the reaction API).

## 🏆 Major Finding: Reaction API is fully public

The single highest-value Phase 1.7 outcome — finding the like/sympathy XHR — succeeded. **Playwright is no longer needed for engagement counts.**

```
GET https://apis.naver.com/blogserver/like/v1/search/contents
  ?pool=blogid
  &q=BLOG[<blog_id>_<log_no>,<blog_id>_<log_no>,...]
  &isDuplication=false
```

Headers: NONE required (no Referer, no User-Agent, no auth, no Origin).
Response: clean JSON.
Batching: yes — multiple `(blog_id, log_no)` pairs in one call, comma-separated inside `BLOG[...]`.

Sample response (trimmed):
```json
{
  "contents": [
    {
      "serviceId": "BLOG",
      "contentsId": "selly9401_224234460263",
      "reactions": [
        {
          "reactionType": "like",
          "count": 5,
          "reactionTypeCode": { "messageCode": "reaction.type.like", "description": "좋아요" }
        }
      ]
    }
  ]
}
```

The user's `fetch_sheet.py` previously used Playwright to read `span.u_likeit_text._count.num` inside a PC iframe. The new CLI uses this REST endpoint instead — 1 HTTP call per N posts vs N Chromium launches per N posts. **Same exact integer Naver renders to the page.**

## Endpoint catalog (all verified replayable)

| # | Purpose | URL pattern | Method | Format | Notes |
|---|---|---|---|---|---|
| 1 | **Reaction/like count** | `https://apis.naver.com/blogserver/like/v1/search/contents?pool=blogid&q=BLOG[<id>_<n>,...]&isDuplication=false` | GET | JSON | No headers required. Batchable. |
| 2 | **Per-blog feed (mobile API)** | `https://m.blog.naver.com/api/blogs/<blog_id>/post-list?categoryNo=0&itemCount=30&page=<N>` | GET | JSON | Returns `sympathyCnt`, `commentCnt`, `shareCnt`, `readCount`, `briefContents`, `addDate` (13-digit ms epoch), thumbnails, category — all per post. **No headers required**; UA + Referer are defensive only. `totalCount` is `0` in the response (Naver-side counter quirk) — actual item count is `len(result.items)` and pagination ends when `items` is empty. |
| 3 | **Per-blog feed (PC fallback)** | `https://blog.naver.com/PostTitleListAsync.naver?blogId=<id>&currentPage=<N>&countPerPage=30&categoryNo=<N>` | GET | JSON-ish | Documented in `hames`; needs invalid-escape sanitization (`body.replace(/\\(?!["\\/bfnrtu])/g, '')`). Use only when endpoint #2 returns empty. |
| 4 | **Mobile post detail (PRIMARY for body+tags)** | `https://m.blog.naver.com/<blog_id>/<log_no>` | GET | HTML | `og:title`, `og:description`, `og:image`, `gsTagName="tag1,tag2,..."` (load-bearing), `.se-main-container` body inline (no iframe). Headers: UA + `Accept-Language: ko-KR,ko;q=0.9,en;q=0.8` (defensive). |
| 5 | **PC PostView (FALLBACK for comment count + publishDate)** | `https://blog.naver.com/PostView.naver?blogId=<id>&logNo=<n>&redirect=Dlog&widgetTypeCall=true&directAccess=false` | GET | HTML | Has `<em id="commentCount">N</em>` inline (empty = 0). Has `<span class="se_publishDate">2026. 3. 30. 15:35</span>`. Larger payload (~250KB). Use when feed doesn't cover the log_no. |
| 6 | **Integrated search SERP (mobile)** | `https://m.search.naver.com/search.naver?where=m_view&query=<urlencoded>` | GET | HTML | 22 `m.blog.naver.com/<id>/<n>` post URLs per page. Hashtag works via `%23<tag>` prefix. Title/snippet extractable from `class="api_txt_lines"` markup. Pagination via `start=N` increment is unreliable; defer to dogfood. |
| 7 | **Integrated search SERP (PC)** | `https://search.naver.com/search.naver?where=view&query=<urlencoded>` | GET | HTML | Same as #6 but returns `blog.naver.com/<id>/<n>` (PC URLs). |

## Selectors and regex (post detail extraction)

**Mobile post (`m.blog.naver.com/<id>/<n>`) — PRIMARY:**
- `og:title` → `<meta property="og:title" content="(.+?)"`
- `og:description` (snippet) → `<meta property="og:description" content="(.+?)"`
- `og:image` (thumbnail) → `<meta property="og:image" content="(.+?)"`
- Tags → `gsTagName\s*=\s*['"]([^'"]+)['"]` then split on `,`
- Body text → strip everything outside `<div class="se-main-container">...</div>`, then HTML→text
- Body HTML (compressed) → keep `<div class="se-main-container">...</div>` block

**PostView (`blog.naver.com/PostView.naver?...`) — FALLBACK:**
- Comment count → `<em\s+id="commentCount"[^>]*>([0-9]+)?\s*</em>` (empty match = 0)
- Floating bottom → `<em\s+id="floating_bottom_commentCount"[^>]*>([0-9]+)?\s*</em>`
- Publish date → `<span class="se_publishDate[^"]*"[^>]*>([^<]+?)</span>` → parse `YYYY. M. D. HH:mm` → KST → UTC

**Engagement (`apis.naver.com/blogserver/like/v1/search/contents`):**
- `contents[i].reactions[j]` where `reactionType=="like"` → `count`

## Feed JSON fields worth surfacing (all per-post, free)

| Field | Type | Notes |
|---|---|---|
| `logNo` | int | Numeric ID for `(blog_id, log_no)` canonical key |
| `domainIdOrBlogId` | string | `blog_id` |
| `titleWithInspectMessage` | string | Post title (raw, sometimes URL-encoded — `decodeURIComponent(t.replace(/\+/g,' '))` then HTML entity decode) |
| `briefContents` | string | Excerpt/snippet (already HTML-stripped, 200-ish chars) |
| `sympathyCnt` | int | **Likes count — same value as reaction API** |
| `commentCnt` | int | Comment count |
| `shareCnt` | int | Share count |
| `readCount` | int | View/read count |
| `addDate` | int | 13-digit epoch ms (UTC; render as KST for display) |
| `categoryNo` | int | Category ID |
| `categoryName` | string | Category name (Korean) |
| `thumbnailList` | array | Image thumbnails |
| `videoPlayTime` | int | Seconds; 0 if no video |
| `scrapType` | string | `0` no-scrap, `1` open, etc. |

## URL canonicalization (matches `lib/naver_url.py`)

Three accepted input shapes; canonical key is `(blog_id, log_no)`:
1. `https://blog.naver.com/<blog_id>/<log_no>` — PC URL (wraps body in iframe; **avoid for body extraction**)
2. `https://m.blog.naver.com/<blog_id>/<log_no>` — mobile URL (preferred)
3. `https://blog.naver.com/PostView.naver?blogId=<blog_id>&logNo=<log_no>` — direct iframe target

Canonical output (Phase 2 emits): `https://m.blog.naver.com/<blog_id>/<log_no>`

## Reachability mode

Per Phase 1.9 decision matrix: `mode: standard_http` — every endpoint above answers a plain `curl` with no challenge, no clearance cookie, no Surf fingerprint required. Anti-bot risk is **low**.

Defensive headers (apply on all requests for resilience):
- `User-Agent: Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1` (mobile UA for mobile endpoints; Chrome desktop UA for PostView / SERP)
- `Accept-Language: ko-KR,ko;q=0.9,en;q=0.8`
- `Referer: https://m.blog.naver.com/<blog_id>` for the per-blog feed (defensive; not strictly required but matches hames pattern)

Rate-limit hygiene: 1–3s between requests, 3 retries with 1.5s/3s/4.5s backoff (matches working OSS scrapers; ship as default `cliutil.AdaptiveLimiter` config).

## Open items (deferred to dogfood)

1. **SERP pagination beyond 22 results.** `start=N` increment didn't advance. Naver's mobile SERP may use an XHR for "more" results that we'd need to sniff on click. For the canary's queries (Chilly product mentions), 22 results is plenty. Defer until a real query asks for more.
2. **CommentList endpoint.** snudm's dated `CommentList.nhn` redirects to PostView modal; no clean JSON found. Counts via PostView `<em id="commentCount">` are adequate; full comment list is out of scope for v1.

## What this means for the spec

- **No Playwright dependency.** The acceptance criterion "100% same-instant engagement match" is achievable with the reaction API.
- **Two HTTP calls per post (worst case):** PostView for comment count + content, reaction API for likes. Both are cheap.
- **One HTTP call per N posts in a feed:** the feed JSON has everything needed; reaction API is a free bonus for cross-validation.
- **No HMAC tokens, no CSRF, no cookies, no login.** All endpoints work from any IP (no Korean IP requirement observed).
- **Defensive headers only.** UA + Accept-Language are belt-and-suspenders; tests confirm even these are not strictly required for the reaction API.
