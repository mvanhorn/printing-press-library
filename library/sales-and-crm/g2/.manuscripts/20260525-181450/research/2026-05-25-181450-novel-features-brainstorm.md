# G2 Novel-Features Brainstorm

## Customer model

### Persona 1: Priya, PMM at a Series-B SaaS company

**Today (without this CLI):** Priya logs into G2's web dashboard each morning to check overnight reviews on her product. She has tabs open to her product page, her two main competitors' G2 pages, the buyer-intent dashboard, and a Google Sheet where she manually copies "new high-intent companies." Once a week she opens a Postman collection to hit `/api/v2/buyer_intent` because the dashboard view truncates the activity feed and doesn't filter by `signal_type=Alternatives`. She can't answer "which review verbatims this quarter mention latency?" because G2's site search is sentiment-only — there's no full-text grep across review bodies.

**Weekly ritual:** Monday morning: pull last week's new reviews on her product and her top three competitors; diff star ratings; copy "alternatives" buyer-intent rows into the sales team's Slack; write a one-paragraph "G2 update" for the exec channel.

**Frustration:** Cross-product diff is manual. There's no command that says "show me reviews and rating shifts for these four products since last Monday" — she does it tab by tab.

### Persona 2: Marcus, Sales Ops lead

**Today (without this CLI):** Marcus runs a daily 7am script that pulls the Tray.ai buyer-intent export, dedupes against yesterday's list, enriches with Clearbit, and pushes high-intent companies into Salesforce as leads. The script burns G2 credits unpredictably — he's been throttled twice mid-month and didn't know until reps complained leads stopped flowing. He keeps a separate Sheet of "credit burn by day" he updates by hand from the G2 admin UI.

**Weekly ritual:** Daily buyer-intent triage at 7am; weekly credit-burn audit on Friday; monthly forecast of whether the team will run out of API credits before month-end.

**Frustration:** Credit metering is a black box. He'd pay real money for "tell me my remaining credits and projected month-end burn rate before I run this sync."

### Persona 3: Dana, Competitive Intel analyst

**Today (without this CLI):** Dana tracks 12 competitor products across 3 categories. She maintains a Notion page with manual screenshots of each competitor's rating, review count, and recent badge wins, updated every Friday. To find "what are reviewers complaining about for Competitor X this quarter," she scrolls competitor review pages and copies the 1-and-2-star reviews into a doc. She has no way to do "show me every review across these 12 products that mentions 'API rate limit' in the cons section."

**Weekly ritual:** Friday afternoon competitive-intel report: who gained/lost stars, who got new badges, what's the top complaint per competitor, who's a rising challenger in each category.

**Frustration:** Full-text search across multi-product review corpora doesn't exist. She has the reviews but they're scattered across 12 web pages with no grep.

### Persona 4: Sam, IT buyer evaluating a category

**Today (without this CLI):** Sam is shortlisting payroll software. He reads G2 reviews but the site optimizes for sentiment ("4.5 stars, loved!"). He needs to know "of the 8 vendors in this category, which have reviewers complaining about SAML/SCIM/compliance issues?" He opens 8 product pages, scrolls reviews, ctrl-Fs for "SAML," gives up after the third product.

**Weekly ritual:** During a category evaluation (every few months, not weekly), pulls 5-10 products' reviews and tries to find technical objections that the marketing copy hides.

**Frustration:** The G2 surface is sentiment-shaped, not technical-fit-shaped. Buyer-side review grep across a category is impossible.

## Candidates (pre-cut)

(See subagent output; 14 candidates generated, 9 survived the cut.)

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence |
|---|---------|---------|-------|--------------|--------------|----------|
| 1 | Cross-product weekly diff | `g2-pp-cli watch products --product <s,s,s> --since 7d` | 10/10 | hand-code | Snapshots `products` + new `reviews` rows since `--since`, joins against prior snapshot in local SQLite, emits star delta + new-review count + badge changes per product | Brief Top Workflow #1 + #3; persona Priya's Monday ritual |
| 2 | Credit-burn forecast | `g2-pp-cli credits forecast` | 10/10 | hand-code | Reads local `credit_ledger` (mirror of `/credit_deductions`), projects month-end spend from trailing 14-day average, gates `sync` under `--budget-check`; calls `hintIfUnsynced(cmd, db, "credit_deductions")` | Brief User Pain Point: credit metering opaque |
| 3 | Multi-product review FTS | `g2-pp-cli search --type reviews "rate limit" --product <s,s,s>` | 10/10 | spec-emits | Framework `search` over SQLite FTS5 index on `reviews.title/body/pros/cons` with structured filters | Brief Top Workflow #5; HN sentiment about G2 surface |
| 4 | Alternatives-signal switching threats | `g2-pp-cli alt-track <my-product> --since 30d` | 10/10 | hand-code | Filters local `buyer_intent_events` where `subject_product_id = <my-product>` AND `signal_type=Alternatives`, ranks companies by visit count × employee size, exports CSV | Brief Build Priority #10 ("novel beyond table-stakes") |
| 5 | Category rising-challenger detector | `g2-pp-cli analytics --type products --group-by category --metric review-velocity-30d` | 9/10 | hand-code | Joins `products` × `categories` × `reviews`, computes trailing-30d new-review counts per product per category, flags top-quartile growth not yet top-quartile by absolute count | Brief Top Workflow #4 |
| 6 | Buyer-intent triage CSV | `g2-pp-cli buyer-intent list --since 24h --min-score 50 --csv` | 10/10 | hand-code | Reads local `buyer_intent_events`, filters and flattens nested firmographics into flat CSV | Brief Top Workflow #2 + Build Priority #6 |
| 7 | Reviews × competitors top-cons | `g2-pp-cli analytics --type reviews --group-by product --filter "rating<=3" --product <s,s,s>` | 9/10 | hand-code | Local SQLite aggregation: lowest-rated reviews' `cons` field verbatims per product, JSON output for piping | Brief Top Workflow #3 |
| 8 | Syndication-eligible review filter | `g2-pp-cli reviews list --product <slug> --syndication-eligible --since 7d` | 8/10 | hand-code | Filters synced `reviews` rows for syndication-eligible flag, scoped by product and `--since` | Brief Top Workflow #1 |
| 9 | Market-signal weekly diff | `g2-pp-cli watch market-signals --category <cat> --since 7d` | 7/10 | hand-code | Diffs locally synced `market_signals` snapshots, emits intent-score delta and visits-count delta per category | Brief Top Workflow #3 |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|--------------------------|
| Category-wide review FTS | Collapses into multi-product FTS when user pipes `categories show` product list | Multi-product review FTS (#3) |
| Hashed-user identity hint command | One-off explanation, not a weekly query | None (fold into `reviews show` help) |
| My-vs-competitor velocity comparison | Strict subset of category rising-challenger | Category rising-challenger (#5) |
| Doctor / scope discovery | Table-stakes auth diagnostic, not novel | None (ships as table-stakes) |
| Cron installer | Scope creep, multi-platform edge cases, one-time setup | Buyer-intent triage CSV (#6 is cron-able by user's own scheduler) |
