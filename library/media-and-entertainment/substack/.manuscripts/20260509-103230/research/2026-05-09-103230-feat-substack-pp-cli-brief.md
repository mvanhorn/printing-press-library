# Substack CLI Brief

> Run 20260509-103230 — agent-operated growth loop for a Substack publication owner. WriteStack is indicative reference; we absorb and beat it.

## API Identity
- **Domain.** Substack — newsletter platform (long-form posts) + Notes (short-form Twitter-like surface) + Substack Chat. Three product surfaces; two distinct follower graphs (publication subscribers vs. profile followers).
- **Users.** Solo newsletter owner, ghostwriter/agency managing multiple clients, cross-promo network organizers.
- **Data profile.** Drafts, posts, notes, comments, replies, profiles, publications, sections, tags, recommendations, restacks, likes, subscribers, activity feed, inspiration corpus (cross-publication notes search), categories.
- **API shape.** No official public API for the community surface. Three bases:
  - `https://substack.com/api/v1` — global: login, current user, Notes, comment feed, search, profiles, reader feed, categories, recommendations.
  - `https://<sub>.substack.com/api/v1` — per-publication: drafts, posts, schedule, image upload, sections, tags, archive, recommendations(from).
  - `https://publisher-api.substack.com/v1` — **official Publisher API** (gated, opt-in, key-based): list_posts, get_post, post stats, subscriber counts/lookup.
- **Auth.** Cookie-based (`substack.sid` or older `connect.sid`) for community surface; `authorization: <key>` (raw, no Bearer) for the official Publisher API. No CSRF token; no native 2FA. Email magic-link is de-facto second factor on new sessions.

## Reachability Risk
**Medium.**

- **Captcha at password login** is the dominant breakage (6+ closed issues across `ma2za`, `Substack2Markdown`). Mitigation: paste-cookie-from-DevTools is the production default; email/password is best-effort secondary.
- **Cloudflare TLS fingerprinting on RSS feeds** blocks Python `httpx`; Go stdlib `net/http` is **not** in the typically-blocked set as of 2026 (low risk for our binary). `doctor` should still probe `https://substack.com/feed` with the runtime client to surface the failure mode early.
- **No rate-limit headers.** Throttle silently or 429 with no structure. Default 2 RPS for novel-feature commands; 25 RPS for read-only.
- **Notes API can shift without warning** — every endpoint is reverse-engineered. Centralize Notes write paths in a single internal package so a single fix lands across all callers.
- **Substack Chat / DMs** are unmapped (open PR on NHagar #19, never merged). Defer or browser-sniff in a follow-up run.

## Top Workflows
1. **Publish + schedule long-form posts.** Draft → prepublish-validate → publish or schedule. Image upload. Tags and sections.
2. **Post Notes 3–5×/day on a cadence.** Spaced ≥30 min, time-of-day rotated by audience-online window. Bulk schedule batch from a `notes/` queue. Auto-restack-self ~12h after publish; auto-like-self at fire time.
3. **Engage with other writers' Notes.** Read niche feed → comment substantively → restack-with-comment using endorsement/bridge/comment-first patterns → reply to incoming engagement on own Notes.
4. **Discover writers in niche.** Search publications, browse category leaderboards, resolve handles, build curated follow list. Find recommendation-swap partners.
5. **Cross-promote.** Read & write Substack Recommendations list (5 partners/week target). Draft personalized swap-outreach.
6. **Measure & learn.** Conversion analytics by format/time/hook (which Notes drove subs). Top-engagers report. Best-time-to-publish recommender. Streak tracking.

## Table Stakes (from WriteStack + community wrappers)
- Notes scheduling (single + bulk batch)
- AI Note generation (voice-matched, from-article, from-inspiration) — implementation strategy: shell out to user's own Anthropic/OpenAI key OR ship a `prompts/` template library and let the agent host call its own model
- Activity Center / notification triage with auto-like-on-reply, keyboard-shortcut-friendly output
- Notes Inspiration search (Substack-wide, sortable by reactions/comments/restacks/date)
- Posting heatmap (GitHub-style)
- Top-engagers leaderboard / Super Fans
- Streak tracking + reminder
- Best-time-to-publish recommender
- Auto-restack on publish; auto-like own note at fire time
- Note categories/tags + per-schedule category routing
- Note templates with goal taxonomy (paidSubscribers, freeSubscribers, restacks, likes, comments, clicks, balanced)
- Reply / restack / quote-restack drafts (and send-approved batch)
- Comment drafting on others' posts (inline-anchored)
- Idea/outline generator for long-form
- Multi-client ("ghostwriter") mode with per-client schedules and voice
- Cross-post to LinkedIn / X (Narrareach niche; WriteStack uses Buffer)
- Substack Recommendations management (list, set, sync)
- Heatmap visualization output (TUI or markdown export)
- MCP surface mirroring the entire CLI

## Data Layer
- **Primary entities** (`internal/store` schema):
  - `publications` (subdomain, custom_domain, id, name, description)
  - `posts` / `drafts` (id, slug, title, subtitle, body, draft_status, scheduled_at, published_at, type)
  - `notes` (id, body, body_json, parent_id, posted_at, author_user_id, attachments)
  - `comments` (id, post_id, parent_id, body, author_user_id, posted_at)
  - `profiles` (user_id, handle, name, bio, photo_url, publication_url, subscriber_count_estimate)
  - `subscriptions` / `followees` (subject_user_id, target_publication_id_or_user_id, kind)
  - `recommendations` (from_publication_id, to_publication_id, kind)
  - `categories` (id, name)
  - `engagements` (id, kind=[like|restack|restack-with-comment|comment|reply], note_or_post_id, target_handle, by_self, recorded_at)
  - `notes_queue` (id, body_json, scheduled_at, status, should_restack_self, should_like_self, category_id)
  - `inspiration_notes` (cached search results, cursor, fetched_at)
  - `analytics_snapshots` (date, subscribers_total, free_count, paid_count, source=community|publisher_api)
  - `post_stats` (post_id, opens, clicks, free_subs_acquired, paid_subs_acquired, fetched_at)
  - `reach_windows` (day_of_week, hour, conversion_rate, sample_size) — aggregation table
- **Sync cursor.** Per-resource `last_synced_at`. Notes use opaque cursor; posts use offset; categories use page; Publisher API uses `next` token. Abstract via a `--page-size` / `--cursor` pair.
- **FTS5.** `notes_fts(body)`, `posts_fts(title, subtitle, body)`, `profiles_fts(name, bio, handle)`.

## User Vision
> "Build an agent that handles publishing, engagement so I grow audience for my newsletter as well as followers. That needs to be achievable using the CLI. Replicate the functionality and features of writestack.io. WriteStack is indicative not exclusive. So think like that."

The CLI is the agent's wire to Substack. Every action a human grower would take must be CLI-reachable so an agent can drive the entire growth loop autonomously, including the parts where WriteStack stops (cross-post to LinkedIn/X, paid-rec swap-finder, DM nudges).

## Product Thesis
- **Name (binary).** `substack-pp-cli`
- **Product name (display).** Substack
- **Why this CLI should exist.** WriteStack is closed-source, web-only, plan-gated (Claude MCP behind Standard+/Premium tier), and stops at Notes scheduling + AI gen + analytics + Activity Center. There is no Go CLI today that performs publish, schedule, post-note, follow, like, comment, or recommendation-write operations. There is no single tool that combines (a) the writer surface, (b) the Notes surface, (c) the official Publisher API, and (d) the discovery surface. We do all four in one binary, agent-native by default, offline-first via SQLite, MCP-first, and *transcend* into cross-post + paid-rec-swap + reciprocity-tracking territory WriteStack doesn't cover.

## Build Priorities

### Priority 0 — Foundation (generator emits)
1. Cookie auth (paste-and-store + keyring); fallback email/password with captcha-aware error.
2. Three-base routing in the generated client.
3. Local SQLite store with schema above; FTS5 indexes; `sync` command per primary resource.
4. `doctor` command probing all three bases plus a `/feed` Cloudflare check.
5. `agent-context`, `--json`, `--select`, `--csv`, `--compact`, `--dry-run`, typed exit codes, MCP surface.

### Priority 1 — Absorb (match every existing tool)
- Drafts CRUD + prepublish + publish + schedule/unschedule + delete.
- Image upload (data-URI JSON).
- Sections + tags listing/create/attach.
- Notes create (`POST /comment/feed` with ProseMirror builder), read (own feed cursor-paginated), single-by-id, by-profile, by-publication.
- Comments: list-on-post, get-by-id, reply (parent_id).
- Profile: own, by-handle, by-id, followees.
- Search: publications, categories, all-in-category.
- Recommendations: from-publication (read).
- Publisher API (when key provided): list_posts, get_post, get_post_stats, get_subscriber_counts, get_subscriber.

### Priority 2 — Transcend (only possible with our approach)
The full transcendence list is decided in the Phase 1.5 absorb manifest after auto-suggest brainstorm. Seeds informed by research:
- Best-time-to-publish recommender (local store join: own Notes × engagement events × time bins).
- Top-engagers / Super Fans (local store join: engagement events grouped by reader handle).
- Conversion-by-format/time/hook (Notes stats × subscriber-list join).
- Niche-overlap-aware follow-list curator (category leaderboard × follower-graph heuristic).
- Reciprocity ledger (who I engaged with vs. who engaged with me).
- Posting cadence guard (rejects burst-schedule drops within 30 min window).
- Inspiration search with format-pattern extraction (curiosity-gap colon, 3-sentence formula, reframe).
- Streak status + per-day "today's queue empty?" check.
- Restack-with-comment pattern templates (endorsement/bridge/comment-first).
- Cross-promo swap-outreach drafter (LLM-assisted, drafts only).
- Cross-post body translator (Substack note → LinkedIn/X-shaped variants).
- Voice-corpus builder from own published posts.

### Priority 3 — Polish
- Skipped complex bodies for ProseMirror richness (start with text + bold + italic + link; expand if time).
- README cookbook + SKILL recipes covering daily growth-loop ritual.
- Recipe pairing `--agent` with `--select` for deep-nested Note responses.

---

**Next:** Phase 1.5 absorb manifest builds the full feature table (every WriteStack/MCP/community feature absorbed; every novel feature scored ≥5/10 admitted) and gates on user approval before generation.
