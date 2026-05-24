# Trustpilot CLI — Novel Features Brainstorm (full subagent transcript)

## Customer model

**Persona A: Riley — last30days research agent operator**
- **Today (without this CLI):** Riley runs `last30days thriftbooks` to build a narrative across Reddit/HN/X/YouTube. Trustpilot is the missing primary source for consumer sentiment — Riley either skips it (story has a gap) or opens trustpilot.com manually, copies a couple of recent reviews, and pastes them in. There is no way to ask "give me 5 fresh good and 5 fresh bad reviews as JSON" from the terminal.
- **Weekly ritual:** 3-5 last30days runs per week against consumer brands. Every run wants a balanced "what are real customers saying right now" slice that an agent can quote with a date and a star count.
- **Frustration:** The single most impossible part is the balance: a recency-only pull from Trustpilot is dominated by 5-star solicited reviews, and a sort-by-relevance pull is stale. There is no "most useful 5 good + 5 bad from the last 30 days" call anywhere.

**Persona B: Sam — competitive/landscape analyst at a DTC brand**
- **Today (without this CLI):** Sam tracks their own company plus 3-6 competitors on Trustpilot weekly. They open six tabs every Monday, eyeball TrustScore, scroll the first page of recent 1-stars, and copy-paste into a Notion doc. Cannot answer "is our 1-star rate worse than competitor X this month?" without manual counting.
- **Weekly ritual:** Monday morning landscape sweep — pull TrustScore, recent review volume, and 1-star themes for the cohort.
- **Frustration:** No side-by-side comparison surface; no week-over-week drift; no signal when a competitor spikes in 1-stars (which usually precedes a Reddit thread Sam wants to be ready for).

**Persona C: Jules — pre-purchase due-diligence consumer / journalist**
- **Today (without this CLI):** Jules is evaluating a vendor before signing a contract or filing a story. Opens the Trustpilot page, sorts by 1-star, reads 4-5 reviews, sorts by 5-star, reads 4-5 more. Cannot tell if a recent rash of 1-stars is a localized incident or a baseline, cannot tell if 5-stars look solicited, cannot search reviews for a specific term (e.g., "refund", "chargeback") without scrolling.
- **Weekly ritual:** 2-3 deep-dives per week on individual companies, ad hoc.
- **Frustration:** Trustpilot's on-page filter UI is anemic — no full-text search across all reviews of a company, no way to see how the 1-star rate has changed over the last 12 weeks, no way to surface reviews that got a business reply vs. those that were ignored.

## Candidates (pre-cut)

| # | Name | Command | One-liner | Persona | Source | Kill/keep verdict |
|---|------|---------|-----------|---------|--------|---|
| C1 | Balanced recent slice | `top-recent <domain> --window 30d --good N --bad N` | Returns N freshest highly-rated + N freshest lowly-rated reviews in one call | Riley | (a)/(e) User vision | Keep — mechanical (filter by star + date), serves headline integration |
| C2 | Agent bundle | `agent-bundle <domain>` | One JSON payload: company metadata + TrustScore + Trustpilot AI summary + top 5 good + top 5 bad + 5-bin histogram | Riley | (e) User briefing | Keep — pure local+API composition, zero LLM, ideal MCP single-tool surface |
| C3 | TrustScore + sentiment drift | `drift <domain> --weeks 12` | Week-over-week trustScore, 1-star %, 5-star %, total review volume from synced data | Sam, Jules | (b)/(c) Trustpilot content pattern | Keep — local SQLite aggregation over synced reviews |
| C4 | Competitor comparison | `compare <a> <b> [<c>...] --window 90d` | Side-by-side TrustScore, review velocity, 1/5-star mix, top categories | Sam | (b)/(c) | Keep — joins across companies in local store |
| C5 | Review-velocity surge detection | `surge <domain> [--baseline 90d --window 7d]` | Flags when recent volume or 1-star rate exceeds rolling baseline by Z-score | Sam | (b) Content pattern | Keep — mechanical statistical test on synced rows |
| C6 | Offline full-text review search | `search-reviews <domain> "<term>" [--stars 1 --window 90d]` | FTS5 search across synced reviews/title/businessReply with star+date+lang filters | Jules | (a) Persona frustration | Keep — FTS5 is in data layer, distinct from `search` (company-name resolver) |
| C7 | Business-reply gap analysis | `replies <domain> [--unreplied --stars 1]` | Stats: reply rate overall and per star bucket; lists 1-stars with no reply | Sam, Jules | (b) Content pattern | Keep — local aggregation + filter on existing field |
| C8 | Reviewer-overlap across competitors | `overlap <a> <b> [<c>...]` | Reviewers (by consumerName + country fingerprint) appearing across 2+ companies in local store | Sam | (c) Cross-entity join | Soft kill — verifiability fails. Cut. |
| C9 | Country-of-reviewer mix | `geo <domain> [--window 90d]` | Distribution by consumerCountry plus 1-star/5-star rate per country | Sam, Jules | (b) Content pattern | Keep |
| C10 | Language-split sentiment | `lang-split <domain>` | Per-language counts and avg rating | Sam | (b) Content pattern | Soft kill — weaker than C9. Cut. |
| C11 | Topic AI-summary passthrough | `topics <domain>` | Surfaces Trustpilot's own pre-computed topic AI summaries verbatim | Riley, Jules | (b) Content pattern | Keep |
| C12 | Verified-vs-unverified split | `verified-split <domain>` | Counts + avg rating + 1-star/5-star rate for verified vs unverified | Jules | (b) Content pattern | Soft kill — fold into `info`. Cut. |
| C13 | Similar-companies trustscore sweep | `similar-sweep <domain>` | For 8 similarBusinessUnits, fetch each's TrustScore + total reviews and rank | Sam | (b)/(c) | Keep |
| C14 | "What changed since last sync" | `whats-new <domain>` | Lists reviews added since sync_cursor, grouped by star bucket | Sam, Riley | (c) Cross-entity (sync state) | Keep |
| C15 | One-star pile-on detector | `pile-on <domain> [--window 7d]` | Detects clustered 1-star bursts | Sam | (b) Content pattern | Soft kill — fold as `surge --stars 1`. Cut. |
| C16 | Category leaderboard | `category-top <slug> [--limit 25]` | Top companies in a Trustpilot category by TrustScore + review volume | Sam (prospecting) | (b) Category endpoint | Keep |

## Survivors and kills

Adversarial cut applied — see full reasoning in conversation transcript. Final 12 survivors all score ≥ 6/10.

### Survivors

| # | Feature | Command | Persona | Score | How It Works | Evidence |
|---|---------|---------|---------|-------|-------------|----------|
| 1 | Balanced recent slice | `top-recent <domain> --window 30d --good N --bad N --json` | Riley | 10/10 | Two filtered `/_next/data/{buildId}/review/<domain>.json` pulls (`stars=4..5` + `stars=1..2`) merged and date-filtered locally; no single Trustpilot endpoint returns "good+bad in one call" | User Vision in brief explicitly names last30days pairing as the headline workflow; no existing scraper exposes a balanced-mix surface |
| 2 | One-shot agent bundle | `agent-bundle <domain> --json` | Riley | 10/10 | Composes `info` + `top-recent` + Trustpilot AI summary + 5-bin histogram into one JSON payload; readable by an external CLI in one MCP call | User Vision emphasizes "agent-native is the point" + MCP exposure first-class; no scraper / no MCP server exists for Trustpilot |
| 3 | TrustScore + sentiment drift | `drift <domain> --weeks 12 --json` | Sam, Jules | 8/10 | Local SQLite aggregation over `reviews.publishedAt` bucketed by ISO week; computes per-week count, 1-star %, 5-star %, mean rating; Trustpilot exposes only current TrustScore | Brief Top Workflow #5; Scrapfly + ScrapeOps guides note Trustpilot has no historical-trustscore endpoint |
| 4 | Multi-company comparison | `compare <a> <b> [<c>...] --window 90d` | Sam | 8/10 | Joins `companies` + windowed `reviews` tables in the local store across N domains; emits side-by-side TrustScore, review velocity, 1/5-star mix | Brief Top Workflow #6; no existing scraper supports multi-company queries |
| 5 | Review-velocity surge | `surge <domain> [--baseline 90d --window 7d --stars 1]` | Sam | 7/10 | Z-score of recent-window review count or 1-star count vs rolling baseline computed in SQL over synced rows | Trustpilot product pattern (review pile-ons precede news cycles); no scraper surfaces this |
| 6 | Offline review FTS | `search-reviews <domain> "<term>" [--stars 1 --window 90d --lang en]` | Jules, Riley | 9/10 | FTS5 query against `reviews_fts(title,text,businessReplyText)` filtered by rating/language/date columns | Data Layer in brief explicitly designs reviews_fts; Trustpilot has no per-company review-text search in its UI |
| 7 | Business-reply gap | `replies <domain> [--unreplied --stars 1]` | Sam, Jules | 7/10 | GROUP BY `rating` with `businessReplyAt IS NULL` predicate over synced reviews; lists unreplied 1-stars | Every scraper carries the reply field; none aggregates reply rate. Sam's Monday-ritual frustration |
| 8 | Country-of-reviewer mix | `geo <domain> [--window 90d]` | Sam, Jules | 6/10 | GROUP BY `consumerCountry` on synced rows; per-country count, avg rating, 1-star rate | DTC brands track country-level sentiment; consumerCountry is in every review payload but never aggregated by existing tools |
| 9 | Trustpilot topic AI passthrough | `topics <domain> --json` | Riley, Jules | 8/10 | Reads `props.pageProps.topicAiSummaries[]` straight from the cached `__NEXT_DATA__` blob; no LLM, no synthesis | Brief Data domain calls out "topic AI summaries" as first-class; absorb manifest covers only top-level AI summary, not per-topic |
| 10 | Similar-companies sweep | `similar-sweep <domain>` | Sam | 7/10 | Fans out the 8 `similarBusinessUnits` from `info` into parallel `info` calls; ranks by TrustScore/totalReviews | Builds on absorbed `info --similar`; Trustpilot site requires 9 manual page loads to assemble this |
| 11 | Sync delta | `whats-new <domain> [--since <iso> --json]` | Riley, Sam | 8/10 | Compares `reviews.publishedAt` against `sync_cursors.lastSyncedAt`; emits new reviews bucketed by star rating | sync_cursors is in Data Layer; supports agent polling pattern, no equivalent on Trustpilot site |
| 12 | Category leaderboard | `category-top <slug> [--limit 25 --min-reviews 100]` | Sam | 6/10 | Calls absorbed `categories show <slug>` and ranks results by TrustScore (with optional totalReviews floor); local sort over an API page | Absorbed `categories` returns a navigation list, not a ranking; Sam's prospecting frustration |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| Reviewer overlap (C8) | Trustpilot consumer identity isn't stable enough (no public user id, display names collide); verifiability fails the rubric | C4 compare |
| Language split (C10) | Strictly weaker than geo (English speakers span many countries); fold the language counts into `info` filters instead | C9 geo |
| Verified-vs-unverified split (C12) | Interesting once, not weekly; collapses to two extra fields on `info` rather than its own command | C7 replies |
| One-star pile-on (C15) | Pure subset of surge with `--stars 1`; sibling kill keeps the command surface lean | C5 surge |
