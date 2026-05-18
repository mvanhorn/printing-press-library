# Substack Novel-Features Brainstorm (Audit Trail)

## Customer model

**Persona 1 — Jim, the trilingual macro newsletterist**

*Today:* Jim owns three Substacks — MacroView (EN), MakroSicht (DE), and JBP Capital Premium Invest. Every Tuesday and Friday morning he writes a market take in one language, then has to swivel-chair into the other publication's dashboard, paste, reformat, fix paywall markers, re-upload images, re-tag sections. The Substack web UI has no concept of "my publications" as a portfolio.

*Weekly ritual:* Monday — drafts in MacroView; Tuesday — port to MakroSicht; Wednesday — premium pick for JBP Capital; Friday — Notes restack chain across all three handles to drive cross-traffic; Sunday — checks subscriber deltas in three separate browser tabs.

*Frustration:* No single pane of glass for three publications. Subscriber churn is invisible until weekly email. Cross-language duplication is manual copy-paste hell. Best-performing posts are unknowable without three CSV exports and a spreadsheet.

**Persona 2 — The churn-watcher subscriber-list owner**

*Today:* Jim again, but in a different mode. Sunday evening he wants to know who unsubscribed from JBP Capital this week, who upgraded from free to paid, and whether the same email is paid on JBP but free on MacroView (a cross-sell candidate).

*Weekly ritual:* Opens Substack dashboard, exports CSV per publication, opens in Excel, vlookups, gives up by post-#2.

*Frustration:* Substack ships no diff view. No cross-publication subscriber join. No "paid here / free there" report. The data is all there in the dashboard endpoints, just locked in a UI.

**Persona 3 — The archivist who writes in Notes daily**

*Today:* Jim posts a Note every morning at 7am — a single chart commentary across his MacroView handle. He restacks his own posts into Notes, replies to commenters, hands out reactions. Several years of Notes are now historical macro commentary worth searching.

*Weekly ritual:* Every Friday tries to find "that thing I wrote about the yield curve in March" — has to scroll Notes manually. Substack search is shallow and only covers posts, not Notes.

*Frustration:* No full-text search across years of Notes + comments + posts. No offline archive. No way to grep "yield curve" across all three publications and the Notes feed.

## Candidates (pre-cut)

1. **portfolio** — `portfolio` — One-screen status of all 3 publications: subscriber count, paid count, last-published-at, drafts pending, scheduled next. Persona 1. Source: (a)+(e).
2. **post-twin** — `posts twin <slug> --to <publication>` — Duplicate a published post into a sibling publication as a draft, preserving paywall markers, sections, images. Persona 1. Source: (a)+(b)+(e).
3. **churn** — `subscribers churn [--publication] [--since 7d]` — Diff two SQLite snapshots: new/unsub/upgrades/downgrades. Persona 2. Source: (a)+(c)+(b).
4. **cross-sell** — `subscribers cross-sell` — Emails paid on one pub but free/absent on others. Persona 2. Source: (c)+(e).
5. **best** — `posts best [--by …] [--window 30d] [--cross-pub]` — Rank posts by engagement cross-pub. Persona 1+2. Source: (b)+(c)+(f).
6. **bilingual-pair** — `posts pair <en-slug> <de-slug>` / `posts pairs [--missing]` — EN↔DE pairing tracker. Persona 1. Source: (b)+(c)+(e).
7. **drift** — `subscribers drift --window 30d` — ASCII sparkline of net subscribers. Persona 2. Source: (b)+(c).
8. **archive-all** — Bulk archive all 3 pubs. Persona 3. Source: (a)+(b)+(e).
9. **grep** — FTS5 across posts+notes+comments cache. Persona 3. Source: (c)+(b).
10. **note-streak** — Days-in-a-row metric. Persona 3. Source: (c).
11. **reactions-given** — Weekly audit. Persona 3. Source: (b)+(c).
12. **top-engagers** — Rank subs by engagement. Persona 2. Source: (c)+(b).
13. **schedule-board** — Cross-pub calendar. Persona 1. Source: (b)+(f).
14. **paywall-audit** — Persona 1. Source: (b)+(c).
15. **section-balance** — Persona 1+2. Source: (c).
16. **note-thread** — Persona 3. Source: (c)+(b).

## Survivors and kills

### Survivors

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|--------------|----------|
| 1 | Portfolio dashboard | `portfolio` | 9/10 | Aggregates subscriber count, posts, drafts, scheduled per publication from SQLite cache into one ASCII table | Brief Top Workflow #1, no competitor, persona 1 weekly need |
| 2 | Cross-publication twin | `posts twin <slug> --to <pub>` | 8/10 | Reads source post, re-uploads images via `/api/v1/image`, POSTs `/api/v1/drafts` against target subdomain with paywall markers + section mapping | Brief Top Workflow #3, no tool does cross-pub; ty13r duplicate is single-pub |
| 3 | Subscriber churn diff | `subscribers churn [--publication] [--since 7d]` | 9/10 | Diffs SQLite snapshots: new/unsubscribed/upgraded/downgraded vs prior window | Brief Top Workflow #2, Substack ships no diff view, persona 2 Sunday |
| 4 | Cross-publication subscriber join | `subscribers cross-sell` | 8/10 | SQL join across `subscribers.publication_id`: paid on A, free/absent on B/C | Persona 2, brief "killer move", no competitor |
| 5 | Best posts cross-pub | `posts best [--by …] [--window 30d] [--cross-pub]` | 7/10 | Ranks cached post stats; optional grouping across all pubs | Brief Top Workflow #3, persona 1 weekly, no cross-pub competitor |
| 6 | Bilingual pair tracker | `posts pair` / `posts pairs --missing` | 7/10 | Local `post_pairs(en_id, de_id)` + missing-twin query | User Vision (EN+DE), persona 1, unique to bilingual owners |
| 7 | Full-text grep across years | `grep "query" [--scope] [--publication] [--since]` | 8/10 | FTS5 over body_markdown + notes.body + comments.body, bm25 ranking + snippet | Brief Top Workflow #5, persona 3 Friday, Substack search shallow |
| 8 | Scheduled posts board | `schedule board` | 6/10 | Reads cached `scheduled_at` across pubs, ASCII 30-day calendar | post_management endpoints, persona 1 planning, no cross-pub competitor |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| drift (30d sparkline) | Pretty chart but persona 2 wants names+reasons, churn already delivers the actionable cut | churn |
| archive-all | Thin for-loop over absorbed `archive <pub>` | absorbed #37 (archive) |
| note-streak | Vanity metric, not a weekly action | grep |
| reactions-given | Niche audit, derivable from cache one-off | grep |
| top-engagers | Overlaps churn + cross-sell | churn + cross-sell |
| paywall-audit | One-off hygiene, set at write-time | best (already shows paywall flag) |
| section-balance | Quarterly task at best | best |
| note-thread | Single-thread reconstruction obtainable via `notes list` + `comments list` | grep |
