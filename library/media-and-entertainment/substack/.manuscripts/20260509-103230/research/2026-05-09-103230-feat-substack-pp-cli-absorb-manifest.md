# Absorb Manifest — Substack CLI (pre-novel)

> Step 1.5b output. Every feature from every existing tool that touches Substack, mapped to our CLI implementation. Sources: WriteStack (writestack.io), 8 community wrappers, 8 MCP servers, n8n-nodes-substack, sbstck-dl, adjacent tools (StackSweller, StackBuddy, Narrareach, SparkLoop, Substack-native).

## Absorbed (match or beat everything that exists)

### A. Drafts / Long-form publishing (publication base)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | List drafts | ma2za, ty13r, marcomoauro | `posts list --type=draft` (mirrors `GET /<pub>/api/v1/drafts`) | `--json --select`, offline cache |
| 2 | Get draft by id | ma2za, ty13r | `posts get <id>` | offline-cached |
| 3 | Create draft | ma2za, ty13r, marcomoauro, arthurcolle | `posts new --title --body --subtitle` (`POST /drafts`) | `--stdin` body via Markdown→ProseMirror, `--dry-run` |
| 4 | Update draft | ma2za, arthurcolle, ty13r | `posts update <id>` (`PUT /drafts/{id}`) | idempotent, partial fields, `--dry-run` |
| 5 | Append to draft | arthurcolle | `posts append <id> --body` | streaming/agent-friendly |
| 6 | Delete draft | ma2za, arthurcolle, ty13r | `posts delete <id>` | typed exit on partial state |
| 7 | Prepublish validate | ma2za | `posts prepublish <id>` | shows blockers as JSON |
| 8 | Publish now | ma2za, arthurcolle, ty13r, marcomoauro | `posts publish <id>` | `--dry-run` default; `--send` to actually publish |
| 9 | Schedule publish | ma2za, nanameru | `posts schedule <id> --at <iso>` | `--dry-run` default |
| 10 | Unschedule | ma2za | `posts unschedule <id>` | |
| 11 | Duplicate post | ty13r | `posts duplicate <id>` | local clone |
| 12 | Get post content | ty13r | `posts content <id>` | rendered Markdown export |
| 13 | Preview draft | ty13r | `posts preview <id>` | opens in browser via `--launch` |
| 14 | List published | ma2za, ty13r | `posts list --type=published` | offline cache |
| 15 | Get post by slug (public) | NHagar, Noah | `posts get-public <pub> <slug>` | reachable without auth |
| 16 | Archive listing | NHagar, ma2za, sbstck-dl | `posts archive --pub <slug> [--cursor]` | offset pagination abstracted |
| 17 | Set cover image | nanameru | `posts cover <id> --image <path>` | |
| 18 | Get sections | ma2za, ty13r | `sections list` | |
| 19 | List tags | ma2za | `tags list` | |
| 20 | Create tag | ma2za | `tags new --name` | |
| 21 | Attach tag to post | ma2za | `posts tag <id> --tag <tag-id>` | batch via stdin |
| 22 | Image upload | ma2za, ty13r, arthurcolle | `images upload <path>` (data-URI JSON `POST /image`) | returns CDN URL; auto-base64 |
| 23 | Comment attachment | arthurcolle, jakub | `notes attach <path>` (`POST /comment/attachment/`) | for Notes payloads |
| 24 | RSS feed | sbstck-dl, NHagar | `posts feed --pub <slug>` | offline cache |
| 25 | Live blog start/end | arthurcolle | `posts live-blog <id> --start/--end` | |
| 26 | Sub count (community) | ma2za, ty13r | `subs count` (`GET /publication_launch_checklist`) | cheap probe |

### B. Notes / Short-form (substack.com base)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 27 | Post a Note | jakub, arthurcolle, nanameru, ty13r | `notes new --body` (`POST /comment/feed`) | ProseMirror builder via `--stdin`/Markdown |
| 28 | Post Note with image | nanameru, jakub | `notes new --body --image <path>` | one-shot |
| 29 | Get own Notes | jakub, n8n | `notes list --self` (cursor pagination) | offline cache + FTS |
| 30 | Get Note by id | jakub | `notes get <id>` | reconstructs public URL `/note/c-{id}` |
| 31 | Get Notes by handle | jakub, n8n | `notes list --handle <h>` | |
| 32 | Get Notes by id | n8n | `notes list --user-id <id>` | |
| 33 | Reader feed (home) | arthurcolle | `feed home --types=note,post` | algorithmic feed |
| 34 | Profile feed | jakub | `feed profile --user-id <id>` | for following specific writers |
| 35 | Reply to a Note | arthurcolle, jakub | `notes reply <id> --body` | parent_id; `--dry-run` default |
| 36 | Schedule Notes (single) | WriteStack, StackSweller, StackBuddy | `notes schedule --at <iso>` | local queue + cron-like fire |
| 37 | Bulk schedule notes | WriteStack | `notes schedule --batch <file>` | YAML/JSON queue, ≥30 min spacing default |

### C. Engagement (currently unmapped in any wrapper — needs reverse-engineering)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 38 | Like a Note | (gap — no wrapper) | `engage like <note-url>` | reverse-engineer endpoint or browser-sniff in Phase 5 retry |
| 39 | Restack a Note | (gap — no wrapper) | `engage restack <note-url>` | likewise |
| 40 | Restack-with-comment | (gap — no wrapper, but WriteStack drafts the body) | `engage restack-with-comment <note-url> --body --pattern=endorsement\|bridge\|comment-first` | drafts via templates; `--send` to fire |
| 41 | Auto-restack own note | WriteStack `autoRetweet` schema | `notes auto-restack-self --enable [--delay 12h]` | local scheduler |
| 42 | Auto-like own note | WriteStack `shouldLike` schema | `notes auto-like-self --enable` | fires at scheduled-note publish-time |

### D. Comments on long-form posts
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 43 | List comments on a post | jakub, n8n | `comments list --post-id <id>` | offline cache |
| 44 | Get comment by id | jakub, n8n | `comments get <id>` | |
| 45 | Reply to a comment | arthurcolle | `comments reply <id> --body` | parent_id; `--dry-run` |
| 46 | Comment on a post | (gap) | `comments new --post-id <id> --body` | |
| 47 | Comment with inline-anchor | WriteStack `startOffset/endOffset/text` | `comments new --post-id <id> --anchor "<text>" --body` | mirror WriteStack inline-comment shape |

### E. Profile / followers / discovery
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 48 | Get own profile | ma2za, jakub, n8n | `profile self` (`GET /user/profile/self`) | |
| 49 | Get profile by handle | NHagar, jakub, n8n | `profile get --handle <h>` | |
| 50 | Get profile by id | jakub, n8n | `profile get --user-id <id>` | |
| 51 | List followees (own) | jakub, n8n | `following list` (`GET /user/{id}/subscriber-lists?lists=following`) | |
| 52 | LinkedIn-handle profile lookup | arthurcolle | `profile from-linkedin <handle>` | |
| 53 | Account settings | ma2za | `settings get` | |
| 54 | Available handles | jakub, arthurcolle | `profile suggest-handles` | |

### F. Search / discovery (substack-wide)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 55 | Search publications | NHagar | `discover search --type=publication --q <q>` | |
| 56 | List categories | NHagar, ma2za | `categories list` | |
| 57 | List publications in category | NHagar, ma2za | `discover category <id> [--page N]` | |
| 58 | Posts by author across pubs | jakub | `discover posts-by --user-id <id>` | |
| 59 | Notes inspiration (Substack-wide search) | WriteStack | `discover notes --niche <slug> [--sort=restacks\|comments\|likes\|date] [--since 7d]` | format-pattern extraction + FTS |
| 60 | Publication embed metadata | ma2za | `discover embed <pub>` | |

### G. Recommendations / cross-promo
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 61 | Read recommendations from a pub | NHagar, n8n | `recs from <pub>` (`GET /recommendations/from/{id}`) | |
| 62 | Set Substack Recommendations list | (gap — WriteStack does it via session UI) | `recs sync --partners <h>...` | |
| 63 | Ranked publication authors | NHagar | `discover ranked --pub <slug>` | |

### H. Activity / notifications / inbox
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 64 | Activity feed (own) | WriteStack `Activity Center` | `inbox list [--type=comment\|restack\|new-sub]` | streamlined feed |
| 65 | Auto-like on reply | WriteStack | `inbox reply --auto-like --auto-dismiss` | |
| 66 | Reply-batch drafts | WriteStack `Activity Center` | `inbox draft-replies --batch` | drafts to JSON; `inbox send-approved <id>` |
| 67 | Connectivity probe (`PUT /user-setting`) | ma2za, jakub, arthurcolle | `doctor` uses internally | non-destructive ping |
| 68 | Substack Chat / DMs | (open-territory; PR #19 stale) | (deferred) | flagged for browser-sniff in retry |

### I. Subscribers / analytics (community)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 69 | Subscriber count | ma2za | `subs count` | |
| 70 | Email-unsubscribers ledger | WriteStack `EmailUnsubscribers` | `subs unsubscribers list` | |

### J. AI / generation features
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 71 | AI Note generation (voice-matched) | WriteStack `AI Note Generator` | `notes generate --voice <profile-name> [--from-article <id>] [--count N]` | shells to user's `ANTHROPIC_API_KEY` or `OPENAI_API_KEY`; ships prompt templates; offline corpus indexing |
| 72 | Idea/outline generator | WriteStack `Idea` table | `posts brainstorm --topic` | |
| 73 | Article teaser generator | WriteStack public tool | `posts teaser <id>` | |
| 74 | Profile scorer | WriteStack public tool | `profile score [--handle <h>]` | |

### K. Heatmap / streak / public tools
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 75 | Posting heatmap | WriteStack `/heatmap` | `analytics heatmap [--days 90]` | GitHub-style ASCII or SVG export |
| 76 | Streak status | WriteStack `setStreak/fetchingStreak` | `analytics streak` | local computed |
| 77 | Streak reminder | WriteStack `shouldSendStreakReminder` | `analytics streak --remind-at HH:MM` | OS notification via `--launch`; PRINTING_PRESS_VERIFY-safe |
| 78 | Top engagers | WriteStack `topEngagers` | `analytics top-engagers [--limit N] [--days 30]` | local store join |
| 79 | Super Fans | WriteStack `/fans` | `analytics super-fans` | |

### L. Multi-client / ghostwriter mode
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 80 | Switch client context | WriteStack `selectedClientId` | `client switch <name>` | scopes all subsequent commands |
| 81 | Add client | WriteStack `Ghostwriter*` tables | `client add --name --pub --cookie` | |
| 82 | Remove client | WriteStack `DELETE /api/ghost-writer/clients` | `client remove <name>` | |
| 83 | Per-client schedule isolation | WriteStack `clientSchedules` | scopes `notes schedule` and `notes queue` per client | |
| 84 | Per-client voice corpus | WriteStack `clientArticles` | scopes voice-matched generation | |

### M. Substack-native cross-promo (read-only today)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 85 | Substack Recommendations (read) | Substack-native; NHagar | included in F |  |
| 86 | Boosts (paid recs) | Substack-native paid | `recs boosts list` (read-only; `--launch` opens dashboard) | |

### N. Cross-post bridges (Narrareach niche; WriteStack has Buffer integration)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 87 | Buffer channel post | WriteStack `bufferChannelIds*` | `crosspost buffer <note-id> --channel <id>` | |
| 88 | LinkedIn cross-post | Narrareach | (deferred — needs LinkedIn auth) | flagged |
| 89 | X (Twitter) cross-post | Narrareach, generic | (deferred — needs X auth) | flagged |
| 90 | RSS-only outbound | sbstck-dl | `posts feed --pub` already covers | |

### O. Templates / categories / queues
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 91 | Note categories | WriteStack `NoteCategory*` | `notes categories list/add/assign` | |
| 92 | Per-schedule category routing | WriteStack `categoryIdsToPostWith` | `notes schedule --category <id>` | |
| 93 | Note templates with goal taxonomy | WriteStack `NoteTemplateGoal` | `notes templates list [--goal=paid\|free\|restacks\|likes\|comments\|clicks\|balanced]` | |
| 94 | Note template difficulty | WriteStack `NoteTemplateDifficulty` | `notes templates filter --difficulty=beginner\|intermediate\|advanced` | |

### P. Auth / session
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 95 | Cookie paste auth | sbstck-dl, ty13r | `auth set-token --cookie` | OS keyring storage |
| 96 | Chrome cookie import | nanameru `pycookiecheat` | `auth login --chrome` | reads logged-in Chrome session |
| 97 | Email + password login | ma2za | `auth login --email --password` | best-effort; captcha-aware error |
| 98 | Token rotation on 401 | ty13r `auth_manager` | `auth refresh` | re-prompts cookie paste |

---

**Total absorbed: ~98 features** spanning the writer surface, Notes surface, engagement (with 5 endpoints currently unmapped — flagged for fix-while-building), comments, profile/discovery, recommendations, activity, subscribers/analytics, AI, public tools, multi-client, cross-post, and auth.

This is the universe. Now novel features come from joining the local store, exploiting Substack's algorithm-aware patterns, exposing growth-loop rituals as one-shot commands, and surfacing reciprocity ledgers no individual API call can provide.

---

## Transcendence (only possible with our approach)

Survivors from Phase 1.5c.5 brainstorm (8 features, all scoring ≥6/10). Full audit trail (personas, pre-cut candidates, kill reasons) in `2026-05-09-103230-novel-features-brainstorm.md`.

| # | Feature | Command | Score | Why Only We Can Do This | Persona |
|---|---------|---------|-------|------------------------|---------|
| T1 | Note→Sub attribution ledger | `analytics attribution [--days 30]` | 9/10 | Local SQLite join: `notes` × `analytics_snapshots` deltas × `engagements` over a 24h post-window per Note. No single API call tells you which Note drove which sub; WriteStack approximates via heatmap correlation but has no per-Note attribution. | Maya |
| T2 | Reciprocity ledger | `engage reciprocity [--handle <h>] [--days 30]` | 9/10 | Joins outgoing `engagements` (where `by_self=true`) against incoming engagements per target handle; emits net-give/net-take with drift alerts. No wrapper exposes both directions; no API call returns reciprocity per peer. | Priya, Maya |
| T3 | Cadence guard | `notes schedule --guard` (pre-flight gate) | 8/10 | Reads queue + posted_at history; blocks/warns on <30 min spacing or time-of-day rotation violation. Substack-specific noting ritual; typed exit code 2 + JSON diagnosis. | Maya, Devon |
| T4 | Best-time recommender | `analytics best-time [--days 90] [--for-goal=subs\|likes\|restacks]` | 8/10 | Aggregates own Notes × engagement events into `reach_windows` (day_of_week × hour); ranks top cells per chosen goal. Goal-aware optimization beats WriteStack's heatmap. | Maya, Devon |
| T5 | Hook-pattern miner | `discover patterns --niche <slug> [--sort restacks]` | 7/10 | Pulls cached `inspiration_notes`; mechanically extracts colon-hook, 3-sentence formula, em-dash-reframe ratios per niche. Pattern aggregation is novel — WriteStack does generation, not measurement. | Maya, Devon |
| T6 | Swap-partner finder | `recs find-partners --my-pub <slug> [--top 20]` | 7/10 | Joins `recommendations` × my `subscriptions`/followees × `profiles`; scores candidate pubs by mutual-overlap density. No tool today computes this. | Priya, Maya |
| T7 | Pod restack scoreboard | `analytics pod --members <h>...` | 7/10 | Given handles, joins last-30d engagements into member×member matrix; markdown table + JSON. Replaces Priya's manual spreadsheet. | Priya |
| T8 | Voice fingerprint | `voice fingerprint [--handle <h>] [--diff <other>]` | 6/10 | Mechanical extraction over cached Notes/posts: sentence length, em-dash rate, colon-hook rate, hook-line ratios, vocabulary uniqueness. WriteStack does voice-matched generation but exposes no measurable fingerprint. | Devon, Maya |

---

## Stub disclosure

Two endpoint families are flagged for **fix-while-building** (reverse-engineer or browser-sniff during Phase 3) — they are SHIPPING SCOPE, not stubs:
- **Engagement writes** (`engage like`, `engage restack`) — community wrappers don't expose them. We will inspect the Substack web app's network panel during Phase 3 to capture the live endpoints. Falls back to print-the-curl on failure.
- **Recommendations write** (`recs sync`) — same pattern.

One feature shipping as **honest stub** (explicit "feature available; backend deferred"):
- **AI Note generation** (`notes generate --voice ...`) — ships the prompt-template library + voice corpus extractor, calls user's `ANTHROPIC_API_KEY` / `OPENAI_API_KEY` if set, prints the prompt for offline use otherwise. No bundled LLM; no fake responses.

Two features explicitly **deferred to a follow-up run**:
- **Substack Chat / DMs** — endpoints unmapped (open PR #19 stale on NHagar/substack_api). Browser-sniff in retry.
- **LinkedIn/X cross-post bridges** — require external auth not provisioned this run. Re-add when LinkedIn/X auth is configured.
