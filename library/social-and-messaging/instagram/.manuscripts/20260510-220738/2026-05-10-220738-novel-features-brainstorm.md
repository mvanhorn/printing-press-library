# Novel Features Brainstorm — Instagram PP-CLI
# Run: 20260510-220738
# Subagent: general-purpose (3-pass: customer model → candidates → adversarial cut)

---

## Customer model

### Persona 1: Maya — Content Creator (10K–100K followers)

**Today (without this CLI):** Opens Instagram on her phone, scrolls analytics in the app, screenshots engagement stats, manually copies numbers into a Google Sheet to track trends. When she wants to find an old post she mentioned, she scrolls. Uses instagram-private-api on Python 3.8 but it breaks every two months with challenge flows. Cannot answer: "Which of my posts from last quarter got the most engagement?"

**Weekly ritual:** Posts 5x/week (mix of photos, carousels, reels). Every Sunday she reviews the week: which post performed best, are her hashtags working, who are her most engaged followers. Checks follower count daily.

**Frustration:** Instagram analytics only show 7/30/90 day rollups. She cannot get per-post engagement rate broken down by content type without manual spreadsheets.

---

### Persona 2: Jordan — Social Media Manager (manages 3–8 brand accounts)

**Today (without this CLI):** Uses Later + Sprout Social ($300/mo combined). Exports CSVs monthly. Writes Python scripts against instagrapi that break quarterly. Cannot answer: "Who unfollowed the brand this week?" or "Which hashtags correlate with above-average engagement?"

**Weekly ritual:** Monday: follower count check across all accounts. Wednesday: engagement report. Friday: content calendar confirmation. Gets asked "why are we losing followers?" and has no fast answer.

**Frustration:** Every tool either requires a Meta business account (rules out personal-auth accounts) or is fragile Python. No CLI exists for scripting.

---

### Persona 3: Sam — Power User / Privacy-Conscious Personal Account

**Today (without this CLI):** Uses the web app. Manually checks who follows them vs who they follow. Has no way to search their own DMs. Wonders who has been engaging with their posts but isn't a follower.

**Weekly ritual:** Checks follower count, occasionally cleans up who they're following. Tries to find old DM conversations by scrolling.

**Frustration:** Instagram web has zero DM search. Can't see who they follow that doesn't follow back without a third-party web app (sketchy, requires login credentials).

---

### Persona 4: Alex — Brand/Marketing Analyst

**Today (without this CLI):** Exports from Meta Business Suite. Cannot query across posts programmatically. Wants to know: which commenters on our posts are not followers (brand reach beyond audience)?

**Weekly ritual:** Generates weekly engagement reports for clients. Tracks follower growth delta.

**Frustration:** No tool produces "strangers who comment" — reach beyond current audience — without expensive third-party services.

---

## Candidates (pre-cut)

### Source (a) — Persona-driven

**A1 — `followers diff <username>`** (Persona 1, 2: follower loss mystery)
Command: `instagram followers diff <username> [--since 7d]`
Resolves: "Why are we losing followers?" by showing exactly who unfollowed since last snapshot.
Source: (a) persona-driven — Jordan/Maya frustration
Rubric: KEEP — powered by snapshot diff in SQLite; not a wrapper; weekly use confirmed.

**A2 — `following non-mutual`** (Persona 3: who don't follow back)
Command: `instagram following non-mutual`
Resolves: The "who follows me back?" question without handing credentials to sketchy web tools.
Source: (a) persona-driven — Sam frustration
Rubric: KEEP — 2-table JOIN; not callable via any single API endpoint.

**A3 — `posts engagement <username> [--since 30d]`** (Persona 1: which posts performed best)
Command: `instagram posts engagement <username>`
Resolves: "Which posts got the best engagement?" via per-post rate calculation.
Source: (a) persona-driven — Maya's Sunday ritual
Rubric: KEEP — computed (likes+comments)/followers; JOIN across media + users tables.

**A4 — `dm search <query>`** (Persona 3: find old DM conversation)
Command: `instagram dm search "invoice" `
Resolves: The total absence of Instagram DM search on any surface.
Source: (a) persona-driven — Sam frustration
Rubric: KEEP — FTS5 on dm_messages; verified zero competition on this.

### Source (b) — Service-specific content patterns

**B1 — `posts type-compare <username>`** (Instagram-specific: reels vs carousels vs photos)
Command: `instagram posts type-compare <username>`
Resolves: The A/B question — do carousels outperform reels for my audience?
Source: (b) service-specific — Instagram's media_type enum (IMAGE, VIDEO, CAROUSEL_ALBUM)
Rubric: KEEP — GROUP BY media_type; no single API call returns this aggregation.

**B2 — `stories batch-download --following`** (Instagram-specific: stories expire in 24h)
Command: `instagram stories batch-download --following`
Downloads all active stories from everyone you follow in one command before they expire.
Source: (b) service-specific — stories expiry is Instagram's defining content tension
Rubric: REFRAME — not transcendent (it's N API calls in a loop). Downgrade to absorbed feature (table stakes). KILL from novel list.

**B3 — `highlights list <username>` + `highlights download <highlight-id>`**
Story highlights are Instagram-specific permanent content; no CLI tool covers them well.
Source: (b) service-specific
Rubric: KILL — endpoint not confirmed in sniff spec; no highlight endpoint captured. Scope creep risk.

### Source (c) — Cross-entity local queries

**C1 — `hashtag performance [<username>] [--since 30d]`**
Command: `instagram hashtag performance`
JOIN media × media_hashtags → aggregate likes+comments per hashtag. Shows which tags correlate with above-average engagement.
Source: (c) cross-entity JOIN
Rubric: KEEP — 2-table JOIN; no Instagram API surface exposes per-hashtag engagement for own posts.

**C2 — `comments top-commenters [<username>] [--since 30d]`**
Command: `instagram comments top-commenters`
Identifies superfans: COUNT(comments) per commenter across all posts in date range.
Source: (c) cross-entity aggregation
Rubric: KEEP — impossible via API (each post's comments are paginated separately; no aggregate endpoint).

**C3 — `comments strangers <username> [--since 30d]`**
Command: `instagram comments strangers <username>`
3-table JOIN: comments × followers → commenters who are not followers. Proves reach beyond audience.
Source: (c) 3-table JOIN
Rubric: KEEP — exactly Alex's use case; no equivalent exists anywhere.

**C4 — `feed overlap <username1> <username2>`**
Posts liked by both users — intersection query across two liked-media sets.
Source: (c)
Rubric: KILL — requires syncing two different accounts' liked posts; auth scope doesn't include other accounts' likes. Auth gap.

**C5 — `mentions timeline [<username>]`**
Aggregate all posts where the user is tagged, sorted by engagement.
Source: (c)
Rubric: KILL — tagging endpoint not in sniff spec; would require additional undiscovered endpoint. Scope risk.

---

## Survivors and kills

### Survivors

| Feature | Command | Score | Persona | Buildability Proof | Sibling killed |
|---------|---------|-------|---------|-------------------|----------------|
| Follower unfollow detector | `followers diff <username>` | 10/10 | Maya, Jordan | Two `followers_snapshots` rows; SET DIFFERENCE; zero API calls after sync | — none; unique shape |
| Non-mutual following | `following non-mutual` | 10/10 | Sam | LEFT JOIN `following` × `followers` WHERE followers.pk IS NULL | `feed overlap` (auth gap kill) |
| Per-post engagement rank | `posts engagement <username>` | 10/10 | Maya, Jordan | (likes+comments)/followers*100 JOIN across `media`+`users`; pure SQLite | `mentions timeline` (endpoint gap kill) |
| DM keyword search | `dm search <query>` | 9/10 | Sam | FTS5 on `dm_messages.body`; no Instagram surface has search at all | `highlights` (endpoint not sniffed) |
| Content format comparison | `posts type-compare <username>` | 8/10 | Maya, Jordan | GROUP BY media_type → AVG(likes), AVG(comments); pure SQLite aggregation | `stories batch-download` (not transcendent) |
| Hashtag–engagement correlation | `hashtag performance` | 8/10 | Maya, Jordan | JOIN `media` × `media_hashtags` → engagement per tag | `feed overlap` (auth gap) |
| Superfan detector | `comments top-commenters` | 8/10 | Maya | COUNT(comments.commenter_pk) across all owned posts, cross-post | `highlights` (endpoint gap) |
| Reach-beyond-audience | `comments strangers <username>` | 8/10 | Alex | 3-table JOIN: `comments` LEFT JOIN `followers` ON commenter_pk WHERE followers.pk IS NULL | `mentions timeline` (spec gap) |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| `stories batch-download --following` | Not transcendent — N API calls in a loop; no SQLite power | `stories download <username>` (absorbed) |
| `highlights list/download` | Endpoint not captured in sniff spec; scope creep if added speculatively | `stories download` (absorbed) |
| `feed overlap <u1> <u2>` | Auth gap — can't read another account's liked posts | `following non-mutual` |
| `mentions timeline` | Tagging endpoint absent from sniff spec; can't verify without additional discovery | `comments strangers` |

---

## Reprint verdicts

N/A — first print; no prior research.json exists.
