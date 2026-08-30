# Benzinga CLI — Novel Features Brainstorm (audit trail)

## Customer model

**Maya — watchlist swing trader.** Trades a 40-ticker watchlist. Today: before open, clicks through the ratings calendar manually eyeballing which names got upgrades/downgrades/PT changes overnight, cross-checks each against news one ticker at a time. Weekly ritual: 8:00–9:15 ET rebuild "what changed on my names overnight." Frustration: no single view of "everything that moved on MY tickers since I last looked"; REST can't join ratings+news+signals for a ticker set in one call.

**Devin — options/event trader.** Sits on unusual-options + halt feeds intraday. Today: when a ticker spikes flips between option_activity, block-trade, halt/resume, ratings, newsfeed — five calls — to assemble why it's moving. Ritual: multiple times daily "X ripped 9% — what's the catalyst?" needs answer in <1 min. Frustration: five endpoints, no chronology; nothing stitches sweep+halt+rating+headline into one ordered story.

**Priya — earnings-season research desk analyst.** Covers ~120 names. Today: pulls calendar/earnings for the week, exports to sheet, hand-computes beat/miss vs estimates, manually finds matching call+transcript. Ritual: Sunday + nightly during earnings season build the grid, flag surprises, queue calls. Frustration: earnings endpoint returns estimates+actuals as separate fields but no surprise %, no beat/miss flag, no link to call/transcript.

**Carlos — analyst-signal-quality trader.** Trades off rating changes but only trusts accurate analysts. Today: sees "Firm X upgrades TICKER" but no fast way to ask "is Firm X any good on this name?" Ritual: vets each fresh rating change against the firm's/analyst's historical hit rate before sizing. Frustration: accuracy data (ratings/analysts) exists but unranked and unjoined to the live rating stream.

## Candidates (pre-cut)
(Full Pass-2 list — see Survivors/Killed below for verdicts. Codebase Intelligence present; User Vision absent.)
1. `watch` — KEEP. 2. `why <T>` — KEEP. 3. `catalysts` — KEEP. 4. `analyst-accuracy` — KEEP. 5. `earnings-season` — KEEP. 6. `insider-cluster` — KEEP (marginal). 7. `consensus-divergence` — CUT (overlaps analyst-accuracy). 8. `econ-week` — CUT (thin). 9. `congress` — CUT (wrapper). 10. `options-leaderboard` — CUT (= analytics group-by). 11. `movers-why` — CUT (sibling: why). 12. `news-digest` — CUT (= search --type news). 13. `erx-surprise-gap` — CUT (sibling: earnings-season).

## Survivors (transcendence set — all hand-code, all >=5/10)

| # | Feature | Command | Persona | Score | Buildability | Buildability proof | Long Description |
|---|---------|---------|---------|-------|--------------|--------------------|------------------|
| 1 | Overnight watchlist change scan | `watch <tickerset> [--since last-open]` | Maya | 9/10 | hand-code | Joins local ratings/news/signal tables filtered to a ticker set and `updated >` cursor — multi-entity diff REST can't express in one call | Use this for a multi-ticker "what changed on my names since I last looked" diff across ratings+news+signals. Do NOT use it to deep-dive one ticker's intraday move — use `why`; for upcoming events use `catalysts`. |
| 2 | Single-ticker move explainer | `why <TICKER> [--window 1d]` | Devin | 9/10 | hand-code | Merges local option_activity/block_trade/halt_resume/ratings/news for one symbol into one time-ordered timeline (mechanical sort, no LLM) | Use this to assemble one ticker's catalyst timeline. Do NOT use it for a watchlist sweep (use `watch`) or forward events (use `catalysts`). |
| 3 | Unified forward catalyst agenda | `catalysts <tickerset> [--ahead 14d]` | Maya/Carlos | 8/10 | hand-code | Unions local earnings/dividends/splits/ipos/fda/conference-calls/guidance/ma into one forward-dated agenda keyed by (date,ticker) | Use this for upcoming dated events across calendar families. Do NOT use it for past changes (use `watch`) or computed beat/miss (use `earnings-season`). |
| 4 | Analyst/firm accuracy scorecard | `analyst-accuracy [--ticker T] [--today]` | Carlos | 9/10 | hand-code | Ranks ratings/analysts rows by Benzinga's `ratings_accuracy` field and left-joins today's calendar/ratings to tag each issuer's hit rate | none |
| 5 | Earnings surprise tracker | `earnings-season [--from --to]` | Priya | 9/10 | hand-code | Computes EPS/revenue beat-miss + surprise % from local earnings rows, joins conference-calls + transcript availability, ranked by surprise | Use this for retrospective beat/miss + surprise ranking with linked calls/transcripts. Do NOT use it for the forward schedule — use `catalysts`. |
| 6 | Clustered insider/congress buying | `insider-cluster [--window 30d] [--min 3]` | Devin/Carlos | 8/10 | hand-code | Groups local insider-transactions + congress-trades by ticker, flags symbols with >=N distinct buyers in the window (distinct-owner cluster logic beyond group-by count) | none |

## Killed candidates
| Feature | Kill reason | Closest surviving sibling |
|---|---|---|
| consensus-divergence | Overlaps analyst-accuracy; demand speculative | analyst-accuracy |
| econ-week | Thin over economics endpoint + importance sort API already supports | catalysts / analytics |
| congress | Single-endpoint rename of absorbed congress-trades | insider-cluster |
| options-leaderboard | = `analytics --type option-activity --group-by ticker` | analytics (framework) |
| movers-why | Redundant with `why` per mover | why |
| news-digest | = `search --type news` | search (framework) |
| erx-surprise-gap | Narrow; surprise side lives in earnings-season | earnings-season |
