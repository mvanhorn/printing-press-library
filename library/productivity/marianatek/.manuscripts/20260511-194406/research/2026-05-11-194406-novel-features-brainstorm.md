# Novel Features Brainstorm — Mariana Tek

## Customer model

**Persona 1: Maya, the boutique-fitness regular (multi-tenant member)**

*Today:* Maya holds memberships at three studios — kolmkontrast (her main sauna+pilates), CycleBar (her Tuesday/Thursday spin), and a Y7 yoga drop-in pack. Each studio has its own iframe widget bolted into its website. She bounces between three browser tabs every Sunday night to plan the week, screenshotting class times into Apple Notes so she can compare them. There is no single calendar view across her three brands.

*Weekly ritual:* Sunday 7pm planning session. Open each tenant's site, scan the 7-day grid, mentally diff against her work calendar, manually pick non-overlapping slots, book each one. By Wednesday she usually has at least one schedule conflict she didn't catch because two of her tenants are in different time zones.

*Frustration:* Sold-out classes. The 6:30am Wednesday vinyasa fills 48 hours in advance; she refreshes the page every few hours hoping someone cancels. She has missed the spot to a faster refresher more than once. She also lets credits expire — kolmkontrast sells 10-packs that lapse after 90 days and the dashboard buries the expiry deep under each pack's detail page.

**Persona 2: Jordan, the cancellation-hunter (single-tenant power booker)**

*Today:* Jordan goes to one studio (a Solidcore franchise) where the instructor he wants — Lauren — sells out within an hour of the schedule dropping. He sets browser-tab reminders to refresh `/schedule` and stalks the page. He has never gotten into Lauren's Saturday class on first try.

*Weekly ritual:* Friday 9am: schedule drops, books whatever Lauren class he can. Friday-Sunday: refresh the sold-out class he actually wants every 30-60 minutes. Sometimes he catches a cancellation on Saturday morning while standing in the studio doing the class he settled for.

*Frustration:* No waitlist signal exists in the Mariana Tek consumer flow at his tenant — the brand disabled it. He needs a poller. He'd run a cron job if the tool existed, but spinning up a custom OAuth client for a fitness studio is more friction than the problem deserves.

**Persona 3: Sam, the integration developer / agentic-scheduler builder**

*Today:* Sam runs a Claude agent that manages their calendar, reads emails, and books things. They want their agent to keep an eye on a few class slots and book around their work calendar autonomously. Without a CLI/MCP, the agent has to drive a browser, which breaks every time Mariana Tek tweaks the iframe widget.

*Weekly ritual:* Tells Claude "watch the 7am vinyasa for next Tuesday and book it if it opens, but only if I don't have a meeting that runs past 6:30am." Currently this is impossible — there's no machine-readable interface.

*Frustration:* Mariana Tek has an OpenAPI spec and OAuth2, but every consumer-facing surface is a hosted iframe. The gap between the documented API and a usable agent tool has never been closed.

## Candidates (pre-cut)

(See subagent output — 16 candidates, 8 killed inline, 8 promoted to survivors below.)

## Survivors and kills

### Survivors

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 1 | Watch-for-cancellation | `marianatek watch <class-id\|--filter "instructor=Lauren K,location=Soho"> --interval 60s` | 10/10 | Polls `GET /classes/{id}` and `GET /classes/{id}/payment_options` (or `GET /locations/{id}/schedule` when filter mode); diffs available-spot count against last tick; emits NDJSON on transition to >0; `--auto-book` calls `POST /me/reservations` in same tick. | Brief Build Priority 2 names this as the killer feature; brief User Vision calls out "watch-for-cancellation is the killer feature — sold-out sessions in popular slots are the everyday pain point"; community client `bigkraig/marianatek` has no equivalent. |
| 2 | Multi-tenant unified schedule | `marianatek schedule --any-tenant --type vinyasa --before 07:00 --window 7d` | 10/10 | Local SQL over per-tenant cached `class_sessions` table (populated by `marianatek sync` per tenant); filters in SQL; emits time-sorted rows with `tenant` column. | Brief notes single-user-multi-tenant is the boutique-fitness reality ("a single user often belongs to multiple tenants"); no upstream client or iframe surface offers cross-tenant view. |
| 3 | Regulars / personal affinity | `marianatek regulars [--by instructor\|type\|time\|day\|location] [--top 5]` | 8/10 | `GROUP BY` on local `reservations` joined to `class_sessions`+`instructors`+`locations`; orders by count desc and recency. | `/me/metrics/top_instructors` returns one dimension; brief Build Priority 2 cites "no API offers this aggregation." |
| 4 | Expiring credits + memberships | `marianatek expiring --within 30d` | 8/10 | Local query: `credits` and `memberships` where `expires_at BETWEEN now AND now+30d`; joins remaining balance and lists candidate sessions per credit via `class_payment_options`. | Brief Build Priority 2 lists this verbatim ("credits/memberships about to expire that the API surfaces as raw dates but never aggregates"); brief Data Layer entity list includes Credit and Membership. |
| 5 | Cross-tenant + ICS conflicts | `marianatek conflicts <date> [--ics <path>] [--buffer 30m]` | 7/10 | Reads `reservations` rows across all logged-in tenants for the date; optionally parses local ICS file (Go stdlib + simple parser); flags overlapping intervals and pairs within `--buffer`. | Brief Build Priority 2 lists "overlapping reservations + buffer-time conflicts across tenants"; Maya persona's documented multi-tenant pain. |
| 6 | Catalog FTS5 search | `marianatek search "vinyasa soho morning"` | 8/10 | SQLite FTS5 virtual table over `class_sessions` joined to instructors/locations/regions/class_types; ranks by BM25; returns time-sorted matches. | Brief Data Layer calls out "FTS5 search across class names, instructors, locations, session types — power-user 'earliest barre class at any location' queries." |
| 7 | Book-regular compound | `marianatek book-regular --slot "tue-7am-vinyasa" [--auto]` | 8/10 | Resolves slot key against `regulars` table → finds next matching upcoming `class_session` → calls `GET /classes/{id}/payment_options` → calls `POST /me/reservations`. | Brief workflow #5 (regulars rebook) implicit in "book the same regulars (no shortcut)" pain stated under Product Thesis. |
| 8 | Doctor (multi-tenant health) | `marianatek doctor` | 6/10 | Iterates `~/.config/marianatek/*.token.json`; for each tenant: shows token expiry, last sync timestamp, table row counts, one live `GET /classes?page_size=1` probe to confirm reachability. | Multi-tenant architecture (brief Codebase Intelligence) makes silent-staleness risky; brief Build Priority 0 commits CLI to manage per-tenant tokens which a `doctor` command exposes. |

### Killed candidates

| Feature | Kill reason | Closest survivor |
|---------|-------------|------------------|
| C8 earliest-available | Subset of C2 `schedule --earliest --limit 1`; doesn't justify a separate command. | C2 multi-tenant schedule |
| C9 streak breakdown | Overlaps C3 regulars and the API already returns the headline scalar via `/me/metrics/longest_weekly_streak`. | C3 regulars |
| C11 instructor-watch | Subsumed by C1 `watch --filter "instructor=..."`; same poll mechanism. | C1 watch |
| C12 mood-based recommender | LLM dependency with no useful mechanical fallback; user can `regulars` + `search` themselves. | C3 regulars + C7 search |
| C13 milestone summarizer | LLM dependency (prose recap); user can pipe `/me/metrics/*` JSON to Claude. | — |
| C14 smart-cancel | Scope creep and brittle policy; trivially scripted on top of typed `cancel_penalty` + `swap_spot` commands. | — |
| C15 leaderboard scraping | External service (web scraping with no API equivalent); fragile and out-of-spec. | — |
| C16 price-compare across tenants | Used twice a year — not a weekly ritual; doesn't clear the survivor bar against stronger candidates. | — |
