# edgar-pp-cli — Novel-features brainstorm (full audit trail)

Source: Phase 1.5c.5 novel-features subagent run on 2026-05-13.

## Customer model

### Persona 1: "Iris" — LODESTAR `/$research` agent (first-time thesis construction)

**Today (without this CLI):** Iris is the Claude Code agent invoked when an analyst types `/$research`. To build Gate 1 (unique capability), Gate 2 (execution validation), and Gate 3 (asymmetric structure), she currently chains 8-12 `WebFetch` calls: ticker→CIK lookup, submissions index, the latest 10-K body (100KB-10MB of HTML), four 10-Qs, every 8-K from the last 90 days, a 12-month Form 4 history, the most recent DEF 14A, plus XBRL companyfacts for revenue/margin trajectory. Each HTML fetch costs ~30-100K tokens. She frequently runs out of context budget before finishing Gate 2.

**Weekly ritual:** Two to four fresh research workflows per week, each touching one ticker. The shape is fixed: identify the issuer → pull primary sources → enumerate insider transactions over 12 months → check recent material disclosures → reconcile XBRL facts against management's narrative claims.

**Frustration:** Form 4 transaction history is the single most token-expensive thing she fetches relative to its signal density. She has to read every transaction in detail to figure out which were code-S (signal) vs code-F (noise) — and even then, she can't tell at a glance whether the reporter was a senior officer or some VP of South-American HR. Without that distinction the entire insider-buying narrative is unusable.

### Persona 2: "Quinn" — LODESTAR `/$recheck` agent (quarterly re-validation)

**Today (without this CLI):** Quinn runs against an existing watchlist thesis file (e.g., `watchlist/<ticker>.md`) with a `last_rechecked` timestamp. Her job is to find what changed since that timestamp: any new 8-K, any new Form 4, any 10-Q, any DEF 14A amendment. Today she has no cursor — she fetches the full submissions index and diffs in-context, which means she re-pays the token cost of filings she's already seen.

**Weekly ritual:** Rechecks 3-5 tickers per week. The query is always "what's new since DATE." She does not want full filing bodies; she wants the enumeration with form type, filed-at, and a one-line title — and only the bodies of items she hasn't seen before.

**Frustration:** No delta primitive. Today she gets every filing every time and has to reason about novelty herself. Worse, when an 8-K is a stale amendment (Item 9.01 exhibits-only) versus a material disclosure (Item 5.02 officer departure, Item 4.02 restatement), she can't tell without reading the body.

### Persona 3: "Pax" — Quant pulling XBRL for cross-sectional screens

**Today (without this CLI):** Pax wants `Revenues`, `NetIncomeLoss`, `Assets`, and `OperatingCashFlow` for a list of 50 tickers across the last 8 quarters to feed a quality screen. Today he hits `companyfacts` 50 times, parses 50 deeply-nested JSON documents, normalizes concept aliases (`Revenues` vs `RevenueFromContractWithCustomerExcludingAssessedTax`), and pivots himself.

**Weekly ritual:** Weekly screen refresh. Always tabular, always comparable across issuers.

**Frustration:** XBRL concept aliasing — same economic quantity reported under 3-4 different US-GAAP tags depending on the filer's accountant. No wrapper resolves this transparently.

## Candidates (pre-cut)

_(See subagent return for full 16-candidate list. Survivors and kills below.)_

## Survivors and kills

### Survivors (8, all ≥7/10)

| # | Feature | Command | Score | Persona | One-line description |
|---|---------|---------|-------|---------|----------------------|
| 1 | Recheck delta cursor | `since <TICKER> --as-of TS` | 9/10 | Quinn | Local SQLite delta primitive — only filings filed after the supplied timestamp |
| 2 | 8-K material-item enumeration | `eightk-items <TICKER> --since DATE --material-only` | 9/10 | Quinn, Iris | Parse 8-K Item numbers; flag material vs exhibits-only refilings |
| 3 | Insider follow-through join | `insider-followthrough <TICKER>` | 8/10 | Iris | Join senior-officer code-S sales × subsequent 8-K material items within 90d |
| 4 | Cross-sectional XBRL pivot | `xbrl-pivot --tickers A,B,C --concepts ...` | 8/10 | Pax | Multi-ticker pivot with concept-alias resolution |
| 5 | Item-scoped 10-K/10-Q sections | `sections <TICKER> --form 10-K --items 1A,7,7A` | 8/10 | Iris | Byte-offset Item extraction; token-efficient |
| 6 | Ownership-cross enumeration | `ownership-crosses <TICKER>` | 7/10 | Iris | 13D/G filings against the issuer with percent owned |
| 7 | Governance red flags | `governance-flags <TICKER>` | 8/10 | Iris | Auditor changes (8-K 4.01) + restatements (4.02) + NT-10-K bundled |
| 8 | Offline FTS over cached bodies | `fts <QUERY> --ticker TICK --form 10-K` | 7/10 | Iris, Quinn | FTS5 over `filings.body_text`; never re-hits efts.sec.gov |

### Killed candidates

| Feature | Kill reason | Closest survivor |
|---------|-------------|------------------|
| `insider-cluster` | Duplicates `insider-followthrough` + `insider-net` | `insider-followthrough` |
| `insider-net` | Absorbed into `insider-summary` compound | `insider-summary` (absorbed) |
| `late-filer` / `auditor-changes` / `restatements` | Merged into `governance-flags` | `governance-flags` |
| `xbrl-trend` | Single-ticker case is `companyfacts --concept` + client math | absorbed `companyfacts` |
| `buybacks` | Issuer-specific HTML, fails verifiability | `sections` |
| `proxy-comp` | DEF 14A Summary Comp Table is filer-specific; fails verifiability | absorbed DEF 14A compact |
| `watch` | Duplicates `primary-sources` + `since` composition | `since` + compound commands |
