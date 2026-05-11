## Customer model

### Persona A — Mara, the visual-archive researcher

**Today (without this CLI):** Mara is a writer/researcher building a book about a niche subculture. She has tabs open on ~40 Instagram public profiles of practitioners, plus a half-dozen relevant hashtags. She runs `instaloader profilename` in a tmux pane, gets 401s after 12 posts ([instaloader#2307](https://github.com/instaloader/instaloader/issues/2307)), re-imports cookies from Firefox via `-b firefox`, retries, and watches `iterator-<shortcode>.json.xz` sidecars pile up in `./profilename/`. To find the one post about "the 1973 retreat," she `grep -ri "1973 retreat" ./*/`. Captions live in `.txt` files; geotags live in other `.txt` files; comments live in yet other JSONs. She cannot answer "show me every post that mentions person X across all 40 profiles."

**Weekly ritual:** Every Sunday she runs a sync over her watch-list (the 40 profiles + 6 hashtags), reviews new posts, tags interesting ones for the book, and exports a working subset for her editor.

**Frustration:** Cross-profile search. The archive is on disk but unsearchable as a corpus. Every grep is a Sunday-evening fight.

### Persona B — Dev, the brand-asset operator

**Today (without this CLI):** Dev runs social for a CPG brand. He needs every piece of UGC where the brand is `@mention`-tagged or `#hashtag`-tagged or appears in a Story, so legal can pull rights and creative can re-grade. He has a Zapier flow that sometimes triggers, plus an intern who screenshots Stories before they expire at the 24-hour cliff. Highlights drift in and out of relevance; the cover changes when the team reshuffles a highlight tray. He pays for 4K Stogram on one laptop because nothing else handles Stories+Highlights reliably, but it has no CLI and can't push to S3.

**Weekly ritual:** Monday morning he pulls last week's tagged-in posts, all new Stories from a list of ~25 creators on the brand's roster, and the weekly highlight diff. He delivers a Drive folder + a spreadsheet of "media id → caption → @-mentions → location → creator handle" to the agency.

**Frustration:** Stories expire while he's asleep, and Highlights silently lose items (creators delete a clip out of a tray) — he wants the previous version. Today he has no way to diff a highlight tray week-over-week.

### Persona C — Sasha, the visual-trend designer

**Today (without this CLI):** Sasha designs moodboards for an agency. She maintains a long list of inspiration accounts (~80) and a handful of hashtags. She wants every carousel slide that's a *still image* (skipping reels and video thumbnails) tagged by dominant color or aesthetic motif. She currently runs `gallery-dl --extractor instagram` per profile, opens Bridge to sort by color, and re-tags files in Lightroom. Alt-text accessibility captions (`accessibility_caption` — the IG-generated "Photo by … may contain: two people, sunglasses, outdoors") would be gold to her, but no tool surfaces them.

**Weekly ritual:** Friday afternoon she builds a moodboard for the next pitch — pulls a stack of recent carousels from her shortlist, filters down to single-image posts that match a brief ("warm, indoor, soft light"), and assembles a PDF.

**Frustration:** No tool exposes IG's own alt-text captions, and her archive has no notion of "carousel slide 2 is a still — slide 3 is a video; give me only the stills."

### Persona D — Kai, the rights-and-takedown analyst

**Today (without this CLI):** Kai works for a photographer collective. They monitor ~200 accounts known to repost members' work without credit. Today they keep a spreadsheet of shortcodes seen, manually visit each repost, screenshot it, and write a takedown email. They have no way to know when a known-bad account adds a new repost; they catch most violations weeks late, after a fan tips them off.

**Weekly ritual:** Wednesday they sweep the watch-list, look for new posts whose caption or comments contain a member's name, and prepare DMCA packets (image + URL + caption + timestamp + the IG media_id).

**Frustration:** No incremental "what's new across these 200 accounts since last Wednesday, filtered to posts whose captions reference these 12 names." Today this takes a full afternoon of clicking.

## Candidates (pre-cut)

### (a) Persona-driven

1. **Local-corpus FTS search** — `igpress search "<query>" [--owner u] [--since d] [--kind post|reel|story]` — full-text search over captions, hashtags, mentions, location names, alt-text across the whole local catalog. Persona: Mara, Kai. Source: (a), (f). Kill/keep: passes (uses local SQLite + FTS5; no LLM; buildable from `posts_fts`).

2. **Cross-profile watchlist sync** — `igpress watch add <handle>…` + `igpress sync` — register N profiles/hashtags, then one command does an incremental fast-update pass over the whole list with one shared rate-limit budget. Persona: Mara, Kai, Dev. Source: (a). Kill/keep: passes (composes existing `user`/`hashtag` with one rate budget; adds `watchlist` table).

3. **Highlight-tray diff** — `igpress highlights diff <handle> [--since <run-id>]` — compare current highlight_tray + highlight_items to the prior snapshot in SQLite, report added/removed items per tray and renamed/recovered titles. Persona: Dev. Source: (a), (c). Kill/keep: passes (pure local join; snapshots are already in DB).

4. **Tagged-mention sweep** — `igpress mentions <name>… [--since d]` — across all synced posts/comments, return every shortcode whose caption, mention list, or comments contain any of N names; outputs CSV/JSON with media_id, url, owner, taken_at, caption snippet. Persona: Kai, Dev. Source: (a), (c). Kill/keep: passes (FTS5 over `posts_fts` + `comments`).

### (b) Service-specific content patterns

5. **Carousel slide filter by media kind** — `igpress get/user --slide-kind image|video` (alongside existing `--slide 1-3`) — keep only image children of a carousel, or only video children, across a profile. Persona: Sasha. Source: (b). Kill/keep: passes (filter on `media_items.kind`; single-flag addition).

6. **Alt-text (accessibility_caption) capture + search** — capture IG's `accessibility_caption` into `media_items.alt_text`, add to `posts_fts`, expose `igpress search --alt "two people sunglasses outdoors"`. Persona: Sasha, Mara. Source: (b), (f). Kill/keep: passes (field is in `media_info` response per brief; mechanical, no LLM).

7. **Stories-expiry watcher (one-shot)** — `igpress stories watch [--once]` — for everyone in the watchlist, fetch any stories that haven't expired yet and capture them; idempotent so cron-able. Persona: Dev. Source: (a), (b). Kill/keep: passes if descoped to one-shot run (not a daemon). The cron itself is the user's responsibility.

8. **Reels original-audio index** — every reel's `clips_metadata.original_sound_info.audio_asset_id` and `music_metadata.music_asset_info.id` captured into an `audio_tracks` table; `igpress audio <track-id>` lists every local reel using that audio. Persona: Sasha (trend tracking), Mara. Source: (b), (c). Kill/keep: passes (mechanical extraction; the brief's media_info JSON includes these fields).

9. **Branded-content / paid-partnership flag capture** — capture `is_paid_partnership` + `sponsor_tags` and expose `igpress list --paid` / `--sponsor <handle>`. Persona: Kai (advertiser accountability), Dev. Source: (b), (c). Kill/keep: passes (already in `media_info`).

10. **Location-radius local query** — given a lat/lng + radius, return all archived posts taken at locations within that radius using the `locations` table. Persona: Mara (place-based research). Source: (b), (c). Kill/keep: passes (haversine in Go; pure local).

### (c) Cross-entity local queries

11. **Profile-graph "who reposts whom"** — `igpress repost-graph <handle>` — for every archived post owned by `<handle>`, find archived posts on OTHER profiles whose `caption`, `media_items.sha256`, or `mentions` matches; output adjacency list. Persona: Kai (DMCA), Mara. Source: (c). Kill/keep: passes (sha256 join on `media_items` + FTS join on captions).

12. **"What's new since last sync"** — `igpress whatsnew [--since <run-id>]` — across the watchlist, list every shortcode whose `media_items.downloaded_at > last_run_at`, grouped by owner, with caption snippets. Persona: Mara, Dev, Kai. Source: (c). Kill/keep: passes (one SQL query on `download_history` + `posts`).

13. **Dry-run with size + rate-budget estimate** — `igpress user <h> --dry-run` — enumerate shortcodes that *would* be fetched, sum estimated bytes from `media_info` `original_width/height`, and estimate request count + wall-clock under the configured `--rate`. Persona: Mara, Dev. Source: (c), (f). Kill/keep: passes (the brief explicitly calls this out as a differentiator).

14. **Sync-health report** — `igpress doctor [--handle h]` — show last-successful-sync per watchlist entry, current 429 backoff state, session validity, posts-per-day rolling avg from `rate_limit_events`, and any profiles missing more than N consecutive expected posts (cursor stall). Persona: Dev (operations), Kai. Source: (c), (f). Kill/keep: passes (joins `sessions` + `rate_limit_events` + `download_history`).

### (f) DeepWiki / Codebase Intelligence

15. **AdaptiveLimiter built on `rate_limit_events`** — port instaloader's `RateController` sliding-window backoff but, unlike instaloader, drive it from a *persistent* `rate_limit_events` table so backoff state survives process exits and is shared across `get`/`user`/`hashtag`. Persona: all four. Source: (f). Kill/keep: keep but reframe — this is **infrastructure** not a user-facing feature. Cut from the candidate list (it's a quality bar across all commands, not a "novel feature"). MOVED to infra, removed from survivor pool.

16. **Device-fingerprint persistence** — borrow instagrapi's per-account device_id + persistent UA pinning, stored in `sessions.device_id`, regenerated only on explicit `igpress login --rotate-device`. Persona: all four (ban avoidance). Source: (f). Kill/keep: same verdict — infra, not a user-visible feature. Cut from candidate list.

17. **Mirror-diff against a fresh upstream pull** — `igpress diff <handle>` — re-enumerate shortcodes from IG, compare to local `download_history`, report deletions (post present locally, missing remotely) and additions. Persona: Kai (deleted-evidence preservation), Mara. Source: (c), (f). Kill/keep: passes (small reuse of the cursor walker).

18. **Comments-thread FTS** — separate FTS5 over `comments(text, username)` joined back to `posts`; `igpress comments search "<query>" [--owner h]`. Persona: Kai, Mara. Source: (c), (f). Kill/keep: passes — but very near-duplicate of #4 (mentions). Sibling-merge candidate.

## Survivors and kills

### Survivors

Score key: Domain Fit (0-3) + User Pain (0-3) + Build Feasibility (0-2) + Research Backing (0-2) = raw / 10.

| # | Feature | Command | Score | How It Works | Persona | Evidence |
|---|---------|---------|-------|--------------|---------|----------|
| 1 | Local-corpus FTS search | `igpress search "<q>" [--owner h] [--since d] [--kind ...] [--alt]` | 10/10 (3+3+2+2) | FTS5 virtual table `posts_fts(caption, hashtags, mentions, location_name, owner_username, alt_text)` joined to `posts`/`media_items` — pure SQL, no API call at query time. | Mara, Kai | Brief Data Layer §FTS5 explicitly lists this table; instaloader has no equivalent (sidecar `.txt` files only). |
| 2 | Alt-text capture + search | (data: `media_items.alt_text`; query: `igpress search --alt "..."`) | 9/10 (3+2+2+2) | `media_info.accessibility_caption` is captured into `media_items.alt_text` on every download and indexed in `posts_fts`. | Sasha, Mara | Brief Data Layer flags `accessibility_caption` as auto-generated and useful for offline visual search; no extractor surveyed (instaloader, gallery-dl, ig-dl) exposes it. |
| 3 | Cross-profile watchlist + sync | `igpress watch add/rm/list`, `igpress sync` | 9/10 (3+3+1+2) | New `watchlist(entry_kind, entry_value, added_at, last_synced_at)` table; `sync` iterates entries under one shared rate budget, reusing `user`/`hashtag` walkers. | Mara, Kai, Dev | instaloader requires shell-looping over arg lists; no surveyed tool ships first-class watchlists. Brief Workflows §1+§3+§7. |
| 4 | Highlight-tray diff | `igpress highlights diff <h>` | 8/10 (2+3+1+2) | Snapshot `highlights_tray` + `highlight_items` per sync; diff reports added/removed/renamed items vs prior snapshot. | Dev | Brief Top Workflows §5 + Data Layer `highlights`; gallery-dl/instaloader treat each pull as fresh — no diff. |
| 5 | What's-new since last sync | `igpress whatsnew [--since <run-id>]` | 8/10 (3+3+1+1) | One SQL over `download_history` + `posts` (`downloaded_at > :last_run_at`), grouped by owner. | Mara, Dev, Kai | Brief Workflow §6+§7 (incremental, resumable); instaloader's `--fast-update` reports nothing, just stops. |
| 6 | Dry-run with size + rate-budget estimate | `igpress get/user --dry-run` | 8/10 (3+2+2+1) | Cursor-walk shortcodes; sum estimated bytes from `media_info.original_dimensions`; divide request count by configured `--rate` for wall-clock estimate. | Mara, Dev | Brief Product Thesis §5 names `--dry-run` as a differentiator vs instaloader; #2511 / #2307 show users hitting walls they wish they'd predicted. |
| 7 | Mirror-diff (remote vs local) | `igpress diff <h>` | 7/10 (2+2+2+1) | Cursor-walk shortcodes from IG, set-diff against `download_history` rows; report `added`/`deleted` lists. | Kai, Mara | Brief Product Thesis §2 (re-runs are O(diff)); deleted-evidence capture is core for DMCA persona. |
| 8 | Carousel slide-kind filter | `igpress get/user --slide-kind image\|video` | 6/10 (2+2+2+0) | At child-write time, skip rows whose `media_items.kind != requested`. | Sasha | Brief Workflow §2 (carousel children); existing `--slide 1-3` filters by index not kind — gap in instaloader/gallery-dl. |
| 9 | Sync-health doctor | `igpress doctor [--handle h]` | 6/10 (2+2+1+1) | Joins `sessions` + `rate_limit_events` + `download_history`; reports last-sync per watchlist entry, 429 backoff state, session validity, cursor-stall warnings. | Dev | Brief Reachability Risk §all + Codebase Intel §RateController; instagrapi explicitly doesn't backoff for you. |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| Stories-expiry watcher (one-shot) | Daemon / cron is scope creep; the *capture* is already covered by absorbed #5 (`igpress stories`). Pure one-shot version is redundant with watchlist sync. | #3 Watchlist sync (run it from cron) |
| Tagged-mention sweep (`igpress mentions <name>…`) | Sibling of #1 FTS search with `--name` flag attached — not novel by itself. | #1 Local FTS search |
| Comments-thread FTS as a separate command | Folds into #1 — comments FTS table is just another source feeding `posts_fts` joins; not a standalone command. | #1 Local FTS search |
| Reels original-audio index | Niche; persona evidence weak (only Sasha tangentially). Domain fit 2, user pain 1 — sub-threshold. | None (cut clean) |
| Branded-content / paid-partnership flag | Useful field to capture but not a user-facing command — becomes a column on `posts`, not a feature. Reframed as data-capture, not as a novel feature. | #1 (search by `--paid` filter once column exists) |
| Location-radius query | Cool but speculative; no persona named a specific weekly use. Score 4/10 (2+1+1+0). | #1 (search by `location_name`) |
| Profile-graph repost detector | sha256 reposts are rare on IG (the platform re-encodes); caption/mention join is already #1 with `--owner` group-by. Build cost high, payoff speculative. | #1 + #7 (search + mirror-diff) |
| AdaptiveLimiter built on rate_limit_events | Infrastructure, not a user-facing feature. Lives in the HTTP layer beneath every command. | (cross-cutting; not a candidate) |
| Device-fingerprint persistence | Infrastructure, not a user-facing feature. Lives in `igpress login`. | (cross-cutting; not a candidate) |
