# polymarket-pp-cli — Novel Features Brainstorm

Three-pass brainstorm: customer model, candidate pool with kill/keep checks inline, then an adversarial cut leaving 6 survivors.

---

## Customer model

Polymarket users in the brief group into four concrete personas. I drop the generic "retail trader" archetype because their needs are already saturated by the official Rust CLI's `markets list/get/book/price` chain. The personas below are the ones for whom the official CLI leaves obvious money or time on the table.

### Persona 1 — Andri, the directional retail bettor

- **Today:** Has $2k of USDC on Polygon, runs a Polymarket account through MetaMask EOA. Browses Polymarket.com via mobile during news cycles (Fed meeting, election debate). Owns ~6 open positions across politics + Bitcoin price markets. Has hit "redeem" on the website maybe four times in his life.
- **Weekly ritual:** Sunday night — opens the website, manually scrolls Activity tab, copies P&L into a notes app, decides whether to roll positions or close. Tuesday morning — checks if any positions resolved overnight and need redeeming. Friday — picks 1-2 new markets to enter based on whatever news he read.
- **Frustration:**
  - The "Portfolio" page on polymarket.com shows current value but doesn't tell him *which markets will resolve in the next 7 days*, so he repeatedly misses redemption deadlines and lets winning tokens sit idle (no yield, no liquidity).
  - He can't see *how his average entry price drifted* over the holding period without screenshotting daily.
  - He doesn't know if a position is *cheap to exit right now* (tight spread + deep book) vs *trapped* (wide spread + thin book) — he just hits sell and accepts slippage.

### Persona 2 — Maya, the liquidity-rewards farmer

- **Today:** Runs a single Python script that places bid/ask quotes 1-2 ticks off mid on ~15 reward-eligible markets, harvesting Polymarket's `clobRewards` daily payouts. ~$30k working capital, targets 0.4-0.8% daily on deployed capital.
- **Weekly ritual:** Monday — pulls the previous week's `earnings-markets` per-market to figure out which markets were actually profitable after slippage. Daily — checks `reward-percentages` and `current-rewards` to rebalance which markets to quote. Continuously — watches the WebSocket book for top-of-book to re-quote.
- **Frustration:**
  - There is *no ranked list of markets by expected reward yield per dollar of inventory at risk* — she has to download `rewards`, `reward-percentages`, `market-reward`, and her own historical fills, then build her own join in pandas every Monday.
  - When she gets filled, she doesn't know if she's *the only maker* or *one of many* on that price level (depth-share matters for the scoring algorithm) — `order-scoring` returns her own score but not the contextual breakdown.
  - Cloudflare blocking when she runs her bot from a Hetzner box — she had to manually rotate to residential proxy and *still* lost a day of rewards.

### Persona 3 — Pak Rian, the news-trading arbitrageur

- **Today:** Watches CNN/BBC/Twitter for news that should move a Polymarket contract; cross-checks against Kalshi if a parallel market exists. Trades 5-30 minute windows around news events. Holds positions overnight only when a binary catalyst (court ruling, debate, FOMC) is imminent.
- **Weekly ritual:** Built a Twitter list of ~80 accounts. When a tweet drops he Cmd+Tabs to polymarket.com, searches the topic, places a taker order if the implied probability looks stale. Once a week, reviews which news → market move pairs paid off and which were noise.
- **Frustration:**
  - The official CLI's `markets search` returns markets by recency, not by *recent price velocity* — he can't ask "which markets had the biggest implied-probability swing in the last 60 minutes."
  - When a market's price snaps 10c in 5 minutes, there's no way to know if it was *one whale* or *broad order flow* without manually opening the activity feed and counting trades.
  - No diff against the last sync — he wants "show me markets where the close price today differs by >5c from yesterday."

### Persona 4 — Dewi, the political-research analyst

- **Today:** Works at a small consulting firm. Cites Polymarket implied probabilities in client decks ("the market currently prices a 68% chance of X"). Doesn't trade. Wants reproducible snapshots she can defend.
- **Weekly ritual:** Pulls implied probabilities for ~40 tracked political/macro markets every Monday morning. Pastes into a Google Sheet that auto-renders charts for the Friday client memo. Once a quarter, writes a long-form report that cites historical odds for resolved markets.
- **Frustration:**
  - `markets list` doesn't return *time-stamped historical implied probability* for events that have already moved or resolved — she has to hit `price-history` per token, then dedup, then attribute outcomes back to event titles.
  - When a market resolves she loses the historical context because the official UI dims the page; she wants a *frozen snapshot bundle* of every market she was tracking, queryable forever.
  - She can't `grep` Polymarket from the terminal — the website is a SPA, search is fuzzy, and she's already a CLI-native data person.

---

## Candidates (pre-cut)

Generated from sources `(a)` persona pain, `(b)` prediction-market-specific patterns the absorb manifest does not touch (resolution lifecycle, liquidity rewards game-theory, multi-outcome correlation, on-chain settlement timing), `(c)` cross-entity SQLite joins on the 11-entity store, and `(e)` the user's "full trading surface + wallet PK" vision. Each candidate is checked against the rubric inline.

### C1 — Resolution radar

- **What:** `polymarket-pp-cli radar resolutions --within 7d [--wallet ADDR] [--min-value 10]` — list every market whose `end_date` falls in the window, ranked by (a) user position value if `--wallet` set, else (b) global volume. Distinguishes "accepting_orders=false but not yet redeemable" vs "resolved, redeem now."
- **Source:** persona-driven (Andri P1: missed redemption deadlines, idle winning tokens).
- **Buildability proof:** uses local `markets.end_date` + `markets.closed` + Data API `/positions` for the wallet filter; pure SQL over already-synced data.
- **Kill/keep:** mechanical (no LLM), no external service, no extra auth (read-only Data API for wallet). KEEP.

### C2 — Reward-yield ranker

- **What:** `polymarket-pp-cli rewards rank --capital 10000 --days 7 --min-spread 0.02` — joins `clobRewards` config + recent `earnings-markets` + current `book` depth to compute *expected daily payout per $1k capital* if you quote within `--min-spread` of mid. Output sorted descending.
- **Source:** persona-driven (Maya P2: no ranked yield list; competitor gap — neither IQAI nor guangxiang MCP does this).
- **Buildability proof:** local SQLite join of `rewards`, `markets` (for `clobRewards` JSON), `token_outcomes` (mid), plus a live `book` call per top-N candidate for current depth; pure arithmetic. No prediction model, just a published formula.
- **Kill/keep:** mechanical, uses spec endpoints, no extra auth. KEEP.

### C3 — Position drift snapshot

- **What:** `polymarket-pp-cli portfolio drift --wallet ADDR --since 7d` — for every position, show: entry price (from earliest TRADE in `activity`), current mid (from `token_outcomes`), price-path summary (min/max/last over window using `snapshots` time series), unrealized P&L delta. Tagged "thawed" (tight spread + deep book = easy to exit) vs "frozen" (wide spread + thin book).
- **Source:** persona-driven (Andri P1: doesn't know entry drift; doesn't know exit-ability).
- **Buildability proof:** SQL join over `positions` + `activity` (entry detection: side=BUY rows pre-now) + `snapshots` (min/max) + live `book` call for current spread/depth on the held token. All endpoints are absorbed.
- **Kill/keep:** mechanical, uses spec data, no auth beyond what Data API needs. KEEP.

### C4 — Whale activity tail

- **What:** `polymarket-pp-cli activity whales --market <slug> --min-size 50000 --since 24h` — query Data API `/activity` filtered for trades above size threshold, then group by `taker` address. Surfaces concentration ("one wallet did 60% of volume in last hour").
- **Source:** persona-driven (Rian P3: "was it one whale or broad flow"); cross-entity (joins `activity` + `markets`).
- **Buildability proof:** single Data API call + GROUP BY taker in SQL.
- **Kill/keep:** mechanical, no LLM, no external service. KEEP.

### C5 — Implied-probability diff

- **What:** `polymarket-pp-cli diff prices --since yesterday --min-move 0.05` — compares latest snapshot of `token_outcomes.current_price` to the previous day's snapshot for the same token; outputs every token where |delta| ≥ threshold, joined back to market question + event title.
- **Source:** persona-driven (Rian P3: "biggest implied-probability swings"; Dewi P4: "diff vs yesterday's snapshot").
- **Buildability proof:** SQL window function over `snapshots` table; no API call beyond sync.
- **Kill/keep:** mechanical, local-data only, no auth. KEEP.

### C6 — Frozen research bundle

- **What:** `polymarket-pp-cli bundle export --tag election2026 --out ./bundle.zip` — exports every market matching `--tag` (or `--event` or `--ids`) as a portable bundle: markets + events + tags + full price-history per token + last book snapshot + holders, all as JSONL. Named for reproducibility (timestamp + content hash). `bundle import` rehydrates into a new SQLite.
- **Source:** persona-driven (Dewi P4: defendable snapshots for client decks; CLI-native).
- **Buildability proof:** read-only SELECT over local entities + price-history per token (already absorbed endpoint A16) + write a zip.
- **Kill/keep:** mechanical, no service deps, no auth. Slight scope-creep risk — single command path, ~150 LOC. KEEP.

### C7 — Pre-flight order validator

- **What:** `polymarket-pp-cli order preflight --market <id> --side BUY --price 0.62 --size 100` — runs every check a maker bot needs *before* submitting: tick-size compliance, min-size, current best bid/ask (estimated fill price), reward-eligibility under those params, projected on-chain allowance burn, predicted slippage if market order, expected EIP-712 hash for verification. No actual order sent.
- **Source:** persona-driven (Maya P2: gets filled then realizes she undercharged for the inventory risk; the official CLI sends `create-order` blind); service-pattern (prediction-market quirk: tick sizes vary per market via `clobRewards`).
- **Buildability proof:** local validation + 2 API calls (`/book/{token}` + `/markets/{id}`); no signing required, no PK needed.
- **Kill/keep:** mechanical, no extra auth (read-only). KEEP.

### C8 — Cross-venue arbitrage scout

- **What:** `polymarket-pp-cli arb scan --venue kalshi --min-edge 0.03` — for each Polymarket market with a Kalshi-mapped counterpart, compute implied-prob gap. Requires a static mapping file shipped with the CLI (`kalshi_map.json`, ~50 hand-curated overlapping markets, refresh manually).
- **Source:** persona-driven (Rian P3: cross-checks Kalshi); brief Product Thesis bullet 3.
- **Kill/keep:** **KILL.** Kalshi has its own API + auth; that's an external service the brief does not provide a key for. Static map drifts and would need maintenance outside scope. Reframe-as-local: too thin — without live Kalshi data, the command degenerates to "show me Polymarket prices for markets in the map." DROP.

### C9 — Cloudflare-aware doctor

- **What:** `polymarket-pp-cli doctor --verbose` — already in `A63` baseline, but enriched to *distinguish* the four failure modes the brief calls out: API down, Cloudflare-blocked-IP, auth-tier mismatch, rate-limited. Each gets a specific remediation: "switch `--surf`", "set `POLYMARKET_PRIVATE_KEY`", "wait until $reset".
- **Source:** brief Product Thesis bullet 4 + Reachability Risk section.
- **Buildability proof:** sequence of probes (Gamma GET → CLOB GET → CLOB authed GET → check rate-limit headers) with distinct exit codes per mode.
- **Kill/keep:** **REFRAME.** This is an *enrichment of* the absorbed A63 doctor, not a separate command. It belongs in the build plan for A63 itself, not as a novel feature. Surface it in the implementation notes for A63 instead. DROP as a novel candidate (logged for impl note).

### C10 — Stale-order janitor

- **What:** `polymarket-pp-cli orders sweep --older-than 24h [--dry-run]` — list (and optionally cancel) every open order older than threshold, grouped by market. Many makers leave dust orders alive that count against their per-IP order quota.
- **Source:** persona-driven (Maya P2 housekeeping).
- **Buildability proof:** `GET /orders` (A30) + filter by created timestamp + batch `DELETE /order/{id}` (A26).
- **Kill/keep:** **KILL — overlaps with absorbed.** A29 cancel-all + A26 cancel-by-id already cover the destructive primitive; adding a `--older-than` filter is a one-line argparse change to the absorbed cancel-orders command, not a new novel feature. DROP.

### C11 — Multi-outcome correlation pack

- **What:** `polymarket-pp-cli event correlations <event-slug>` — for events with N>2 markets (e.g., "2028 nominee" event has 12 candidate markets), show the implied-probability correlation matrix over the last `--window`. Useful for hedging: "if I bet YES on candidate A, what's typically happening to candidate B?"
- **Source:** service-specific (multi-market events are a Polymarket primitive); cross-entity (events ↔ markets ↔ snapshots).
- **Buildability proof:** SQL pivot of `snapshots.current_price` by token within event, compute Pearson over the price series.
- **Kill/keep:** mechanical (Pearson is arithmetic, not ML). Verifiability concern: meaningful correlation needs many snapshots. **KEEP but flag low-confidence** for events without enough sync history.

### C12 — Redemption batch executor

- **What:** `polymarket-pp-cli redeem all [--dry-run] [--min-value 1]` — discover every position the wallet holds in resolved markets (CTF tokens in markets with `closed=true`), and execute `ctf redeem` for each in a batch, skipping dust below `--min-value`. Outputs total USDC claimed and gas spent.
- **Source:** persona-driven (Andri P1: hits redeem maybe 4 times a lifetime); brief Top Workflow #4.
- **Buildability proof:** wraps absorbed A54 (`ctf redeem`) in a loop over positions returned by A57; uses already-required wallet PK.
- **Kill/keep:** mechanical, uses absorbed primitives, no new auth. KEEP.

### C13 — Auth bootstrap walkthrough

- **What:** `polymarket-pp-cli auth bootstrap` — interactive (or `--json` non-interactive) flow that: (1) verifies `POLYMARKET_PRIVATE_KEY` env var is set, (2) computes EOA address, (3) calls absorbed `derive-api-key` (A44), (4) writes the L2 creds to config, (5) verifies by listing api-keys (A43), (6) runs absorbed `approve set` (A51) if not already approved. One command to go from "PK env var" to "fully trading-ready."
- **Source:** user vision ("fill in the PK later"; doctor must explain auth tiers); persona-driven (every persona that wants to trade needs this).
- **Kill/keep:** **REFRAME.** The brief's "User Vision" already calls for `auth derive` to do this — this is mostly an orchestration of absorbed A44 + A51 + A43 with logging, not a new feature. Surface as enrichment of A44 in the build plan. DROP as a separate novel candidate.

### C14 — Market depth aggregator

- **What:** `polymarket-pp-cli book depth --token <id> --levels 10 --usd 5000` — given a target USD trade size, project the average fill price by walking the book, output slippage vs midpoint, and flag "this trade would move the market by X bps." Live `book` call, no auth.
- **Source:** persona-driven (Andri P1: doesn't know if a position is cheap to exit); service-specific (CLOB books on Polymarket are often shallow on long-tail markets).
- **Buildability proof:** single `GET /book/{token}` (A12) + arithmetic walk.
- **Kill/keep:** mechanical, single endpoint, no auth. KEEP.

### C15 — News-shock detector

- **What:** Watch a list of markets; surface ones where price moved >N% in the last M minutes vs prior baseline; optionally annotate with the active rate-limit budget.
- **Source:** persona-driven (Rian P3: news trading).
- **Kill/keep:** **MERGE/DROP.** This is effectively C5 (implied-probability diff) with a tighter time window and a watchlist. Adding `--window 60m --watch <slugs>` flags to C5 covers it without a separate command. DROP as redundant.

### C16 — Liquidity-rewards leaderboard

- **What:** `polymarket-pp-cli rewards leaderboard --market <id>` — top N earners on a given market for the last reward day; uses Data API holders + rewards endpoints.
- **Source:** service-specific (rewards game has a public scoreboard dimension).
- **Kill/keep:** **KILL.** No public endpoint in the absorb manifest returns per-user rewards per market — only the user's own scoring (`order-scoring`). Reimplementing it would require scraping or guessing. Fails the **External service / Auth the user doesn't have** check. DROP.

---

## Survivors and kills

Pre-cut pool: 16 candidates. Cuts applied: C8 (external service), C9 (enrichment of absorbed), C10 (overlap with absorbed), C13 (enrichment of absorbed), C15 (merged into C5), C16 (no available data source). Survivors: 10. To hit the "drop ~half" target, I additionally cut C4 and C11 (rationale below) and merge C7's "preflight" scope into a tighter form, leaving **6 survivors**.

### Survivors

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|--------------|----------|
| 1 | Resolution radar | `radar resolutions --within 7d [--wallet ADDR] [--min-value 10]` | 9/10 | Local SQL over `markets.end_date` + `markets.closed`; joins absorbed Data API `/positions` (A57) when `--wallet` set. Single command answers "what resolves soon and what do I need to redeem." | Persona Andri P1 explicit pain (missed deadlines, idle winners); no competing MCP/CLI surfaces resolution-window queries (verified across IQAI 19-tool inventory + guangxiang 8-tool inventory) |
| 2 | Reward-yield ranker | `rewards rank --capital 10000 --days 7 --min-spread 0.02` | 9/10 | Local SQL join of `rewards`, `markets.clobRewards` JSON, `token_outcomes` (mid), plus N live `book` calls (absorbed A12) for current depth. Pure arithmetic — published `clobRewards.rewardsConfig.scoreShare` formula. | Persona Maya P2 explicit pain (Monday pandas joins); Polymarket docs publish the scoring formula but no tool computes the inverse "best market for my capital" — gap across Polymarket Rust CLI + all 4 MCP servers |
| 3 | Position drift snapshot | `portfolio drift --wallet ADDR --since 7d` | 8/10 | SQL join over `positions` + `activity` (earliest BUY = entry) + `snapshots` (min/max over window) + live `book` for current spread/depth on each held token. | Persona Andri P1 explicit ("doesn't know if a position is cheap to exit"); guangxiang `get_user_positions` returns current value only — no drift, no entry attribution, no exit-ability flag |
| 4 | Implied-probability diff (with `--watch`) | `diff prices --since yesterday --min-move 0.05 [--watch <slugs>] [--window 60m]` | 7/10 | SQL window over `snapshots.current_price` for the same token; joins back to `markets.question` + `events.title`. `--watch` filters to user's tracked slugs. Absorbs the news-shock detector use case. | Persona Rian P3 + Persona Dewi P4 both explicit; brief Top Workflow #6 (research-grade pulls) implies diffability |
| 5 | Frozen research bundle | `bundle export --tag <tag-or-event> --out ./bundle.zip` and `bundle import` | 7/10 | SELECT * across local entities filtered by tag/event/ids + per-token full price-history (absorbed A16) + zip. Importable into a fresh SQLite — fully reproducible. | Persona Dewi P4 explicit ("defendable snapshots for client decks"); brief Top Workflow #6 (research-grade pulls); first-of-its-kind across the surveyed competing tools |
| 6 | Redemption batch executor | `redeem all [--dry-run] [--min-value 1]` | 8/10 | Wraps absorbed `ctf redeem` (A54) in a loop over positions where `markets.closed=true`; uses the same `POLYMARKET_PRIVATE_KEY` already required for trading. Returns total USDC claimed + gas. | Persona Andri P1 ("redeem 4 times a lifetime"); brief Top Workflow #4 — official Rust CLI requires manual per-position invocation; no MCP server batches redemptions |

**4-question force-answer per survivor:**

1. **Resolution radar — Real user pain?** Yes (Andri leaves winning tokens idle). **Spec covers it?** Yes (markets.end_date in Gamma; positions in Data API). **Verifiable?** Yes (dogfood: sync, then assert end_date < now+7d returns ≥1 row). **Differentiates?** Yes — no surveyed tool surfaces a resolution window.
2. **Reward-yield ranker — Pain?** Yes (Maya rebuilds every Monday). **Spec?** Yes (clobRewards, rewards, book endpoints all absorbed). **Verifiable?** Yes (assert top result has positive expected yield given known math). **Differentiates?** Yes — official Rust CLI returns raw rewards, never inverts to "where should I put capital."
3. **Position drift — Pain?** Yes (no exit-ability signal anywhere). **Spec?** Yes (activity + book covers it). **Verifiable?** Yes (dogfood with a known wallet: drift values are deterministic). **Differentiates?** Yes — every competing tool returns positions but none classify thawed/frozen.
4. **Diff prices — Pain?** Yes (two personas explicit). **Spec?** Yes (snapshots are local). **Verifiable?** Yes (synthetic snapshot diff is mechanical). **Differentiates?** Yes — `markets list` only sorts by recency.
5. **Frozen bundle — Pain?** Yes (Dewi needs reproducibility). **Spec?** Yes (all data already absorbed). **Verifiable?** Yes (export → import → row count match). **Differentiates?** Yes — first-of-its-kind.
6. **Redeem batch — Pain?** Yes (workflow #4 in brief). **Spec?** Yes (ctf redeem A54 absorbed). **Verifiable?** Yes (`--dry-run` lists the exact set; live run on testnet wallet). **Differentiates?** Yes — no surveyed tool batches.

### Killed candidates

| # | Candidate | Kill reason |
|---|-----------|-------------|
| C4 | Whale activity tail | Useful but tangential to the four core personas (closest fit is Rian, but C5 with `--watch` already answers his "was it one whale" question via the daily price diff trail). Score would land at ~5 — keeping it dilutes the cut. **Reframe note:** if user pushes back, surface as a `--by taker` flag on the absorbed Data API `activity` command (A59) rather than a separate command. |
| C7 | Pre-flight order validator | Borderline. Strong feature for Maya, but its core checks (tick-size, min-size, slippage estimate) overlap heavily with what the absorbed `create-order` (A21) already validates server-side; the unique value-add (reward-eligibility preview) collapses into C2 (reward-yield ranker) once that exists. Cut to avoid duplicate scope; **reframe note:** add a `--preflight` flag to absorbed `create-order` that prints what the server would reject before signing. |
| C8 | Cross-venue arbitrage (Kalshi) | External service (Kalshi) not in brief, no auth; static map would drift. Hard cut. |
| C9 | Cloudflare-aware doctor | Enrichment of absorbed A63 `doctor`, not a separate feature. Belongs in A63 impl notes. |
| C10 | Stale-order janitor | Overlap with absorbed A26 + A29; reduces to a one-flag enhancement of `cancel-orders`. |
| C11 | Multi-outcome correlation pack | Verifiability low (needs many snapshots before output is meaningful); persona fit only marginal (Maya might use it but won't drive adoption). Score lands at ~5 with low confidence — cut to keep the survivor set lean. |
| C13 | Auth bootstrap walkthrough | The user vision already specifies absorbed `derive-api-key` (A44) as the entry point; this is an orchestration enrichment of A44 + A51, not a novel feature. Surface in A44 impl notes. |
| C15 | News-shock detector | Folded into C4 via `--watch` and `--window` flags; no separate command needed. |
| C16 | Liquidity-rewards leaderboard | No public endpoint exposes per-user-per-market rewards; would require scraping. Fails the external-service / auth-the-user-doesn't-have check. |

Six survivors, each scoring ≥7/10, each tied to a named persona, each buildable from absorbed endpoints + the 11-entity local store, none requiring external services or LLM inference.
