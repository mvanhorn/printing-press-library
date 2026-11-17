# Autom novel-features brainstorm (subagent output)

## Customer model

**Persona 1: Claire, freelance SEO consultant (rank tracking)**
- **Today (without this CLI):** Claire tracks 50 to 150 keywords per client for three to five clients, often across France, Belgium and Canada. She has a paid SaaS rank tracker open in one tab, Google in incognito with a VPN in another, and a spreadsheet where she pastes positions by hand. When a client asks "where do we rank in Belgium compared with France?" or "what moved this week across everything?", she cannot answer without opening each keyword one by one.
- **Weekly ritual:** Every Monday she re-runs her keyword list, checks where the client's domain ranks per country, notes the gains and losses, and writes a short report naming the competitors that overtook the client.
- **Frustration:** Going from one row of history per keyword to a single list of what actually moved across the whole portfolio, by country, without reading every keyword's history.

**Persona 2: Malik, growth and market-research analyst (cross-engine and brand monitoring)**
- **Today (without this CLI):** Malik runs the same query by hand on Google, Bing and DuckDuckGo to see whether his brand and its rivals show up the same way everywhere. Brave and Yahoo are usually skipped. For brand news he uses a Google Alerts inbox full of duplicates. He cannot say which domains appear only on non-Google engines, or which news items he has already seen.
- **Weekly ritual:** He checks how the brand and three competitors appear across engines for a handful of category queries, then scans the week's news about the brand and the competitors.
- **Frustration:** Comparing five engines means five tabs and a manual overlap count, and news monitoring keeps showing articles he has already read.

**Persona 3: Priya, AI agent builder (grounding and cost control)**
- **Today (without this CLI):** Priya wires Autom into an agent through the SDK or MCP for grounded web results. She checks the Autom dashboard to see her remaining quota, packs and credits, and greps her logs to see which jobs spent credits. When Bing 500s on `page` or a 401 "Rate limit exceeded" appears, she has to tell it apart from a bad key by hand.
- **Weekly ritual:** She runs batches of grounding queries for evals and prompt changes, then checks how much credit the week burned and whether the balance will last until the subscription renews.
- **Frustration:** `/v1/usage` shows the balance but not where the credits went (which keyword set, which command, which engine), so she cannot predict burn or find the expensive workload.

## Candidates (pre-cut)

| # | Name | Command | Description | Persona | Source | Long Description | Inline kill/keep verdict |
|---|------|---------|-------------|---------|--------|------------------|--------------------------|
| C1 | Cross-engine compare | `engines compare "<query>" [--engines google,bing,brave,duckduckgo,yahoo] [--domain x.com] [--save]` | Runs one query on each engine and prints the domain overlap matrix, the domains unique to each engine, and the position of `--domain` per engine. | Malik | (a) Malik frustration; (b) Autom's multi-engine coverage | Use this command to run ONE query on SEVERAL engines and compare overlap and per-engine positions. Do NOT use this command for one engine across several countries; use 'geo compare' instead. Do NOT use it for a cited source list to answer a question; use 'ground' instead. | KEEP. Uses real API calls (one per engine) with the same auth. Costs N credits, shown by `--dry-run`. A failure on one engine (for example a 500) is reported per engine, not hidden. |
| C2 | Geo compare | `geo compare "<query>" --countries fr,be,ca [--engine google] [--domain x.com] [--save]` | Runs the same query on one engine for several countries or locations and prints the domain's position per country plus the overlap of the top-N results across countries. | Claire | (a) Claire frustration; (b) Google gl/hl/location params plus free finders | Use this command to run ONE query on ONE engine across SEVERAL countries or locations. Do NOT use this command to compare engines; use 'engines compare' instead. Do NOT use it to track one domain's positions over time; use 'rank history' instead. | KEEP. Real API calls. Country codes are checked with the free google-countries finder before any billed call. |
| C3 | Rank movers | `rank movers [--since 7d] [--domain x.com] [--min-delta 3] [--country fr]` | Lists, from local snapshots across every tracked keyword, the biggest position gains and losses, plus entries into and exits from the top N, between the latest run and the run before the cutoff. | Claire | (a) Claire frustration; (c) cross-snapshot local join | Use this command to see what moved across ALL tracked keywords since a cutoff. Do NOT use this command for the full trend of one keyword; use 'rank history' instead. Do NOT use it for a domain's overall footprint; use 'visibility' instead. | KEEP. Local SQLite only, no API call. |
| C4 | Domain visibility | `visibility <domain> [--vs a.com,b.com] [--top 10] [--country fr]` | For a domain, and optionally rival domains, joins every saved snapshot to list the keywords where it appears, its average and best position, and its count of top-3 and top-10 placements, side by side with the rivals. | Claire, Malik | (c) cross-query join over snapshots | Use this command for one domain's footprint across EVERY saved keyword, optionally against rival domains. Do NOT use this command for who sits in the top N for one keyword; use 'rank competitors' instead. Do NOT use it for changes since the last run; use 'rank movers' instead. | KEEP. Local SQLite only. |
| C5 | News watch | `news watch "<brand or keyword>" [--country fr] [--since 7d]` | Calls Google News once, stores the articles, and prints only articles not seen in earlier watches (deduplicated by canonical URL and by source plus title). | Malik | (a) Malik frustration; (b) news as a content type (brief workflow 5) | Use this command for NEW news articles about a brand or keyword since the last watch. Do NOT use this command for a cited, deduplicated source list that answers a question; use 'ground' instead. | KEEP after descope. A background monitor was cut to a single poll per run that cron can schedule. |
| C6 | Credit ledger | `credits [--by keyword\|command\|engine] [--since 30d]` | Joins the live balance from `/v1/usage` (quota, packs, credits, drawn in that order) with a local ledger of billed calls the CLI made, and prints spend by keyword set, command or engine, burn rate per day, and estimated days left. | Priya | (a) Priya frustration; brief Table Stakes "credit visibility" | Use this command for remaining balance PLUS where credits went and the burn rate. Do NOT use this command for the raw account and key payload; use 'usage' instead. Do NOT use it to check the key or connectivity; use 'doctor' instead. | KEEP. `/v1/usage` is free. The ledger records only calls this CLI made (2xx sync responses and completed async jobs) and says so in its output. |
| C7 | SERP volatility | `serp volatility [--since 30d] [--top 10]` | Scores each tracked keyword by top-N churn across its snapshots (rank-change sum, Jaccard of the URL sets) and flags volatile keywords. | Claire | (c) local snapshot join | none | Flagged on verifiability: the thresholds are arbitrary. Sent to Pass 3. |
| C8 | SERP replay | `serp replay <snapshot-id>` | Prints a saved snapshot offline as it was returned. | Claire | (b) snapshots | none | Weak: a raw read of one local row. Sent to Pass 3. |
| C9 | SERP anatomy | `serp anatomy "<query>"` | Lists which SERP features (news, videos, shopping, maps, people also ask) appear for a query. | Claire | (b) Google result blocks | none | Flagged on verifiability: which feature blocks Autom returns is not documented in the brief. |
| C10 | Keyword expand | `keywords expand "<seed>"` | Runs autocomplete recursively ("seed a" through "seed z") to build a keyword file for `batch run`. | Claire | (b) autocomplete endpoint | none | Flagged: 26 or more billed calls per seed, and no research evidence. |
| C11 | Engine health board | `engines health` | Calls `/v1/stats` for every engine and prints a health table. | Priya | (a) Priya frustration (engine 500s) | none | Weak: N calls to a free endpoint, overlaps `doctor`. |
| C12 | Share of voice | `share-of-voice --domains a.com,b.com` | Weights top-10 positions by a click-through-rate curve to give each domain a percentage. | Malik | (c) local join | none | Flagged on LLM-free but invented data: the CTR curve is not in the spec or the brief. |
| C13 | Cannibalization | `rank cannibal <domain>` | Finds keywords where two or more URLs from the same domain rank. | Claire | (c) local join | none | Weak: niche, and no evidence in the research. |
| C14 | Location resolve | `geo resolve "Paris"` | Turns a free-text place into a canonical location plus gl/hl. | Claire | (b) finders | none | Weak: a thin wrapper of the absorbed finder commands. |
| C15 | Cross-source news digest | `news digest "<topic>"` | Merges news from Google News with news-type results from other engines. | Malik | (b) news | none | Weak: only Google has a news endpoint, so this overlaps `news watch` and `ground`. |
| C16 | Batch cost estimate | `batch estimate <file>` | Estimates credits for a keyword file before running it. | Priya | (a) Priya | none | Already covered: `batch run` (absorbed #21) checks the credit budget against `/v1/usage`. |

## Survivors and kills

### Pass 3 answers per survivor

**engines compare**
1. Weekly use: yes. Malik's weekly category check is exactly this.
2. Wrapper vs leverage: leverage. It fans out to five or more engine endpoints and normalizes domains into one overlap matrix.
3. Transcendence proof: a cross-source join (engines) using a service-specific pattern, since Autom's selling point is breadth of engines. `--save` feeds the local store.
4. Sibling kill: C11 `engines health` was killed because engine health is a doctor concern and not a weekly analyst ritual.
5. Buildability: hand-code (`// pp:data-source live`).
6. Long-description validity: 'geo compare' and 'ground' both exist.

**geo compare**
1. Weekly use: yes. Claire's multi-country clients get this every Monday.
2. Wrapper vs leverage: leverage. It fans out over countries, does a finder pre-check, and computes per-country positions and overlap.
3. Transcendence proof: a cross-location join that exploits Google's gl/hl/location parameters plus the free finders.
4. Sibling kill: C14 `geo resolve` was killed as a thin wrapper of the finder.
5. Buildability: hand-code (`// pp:data-source live`).
6. Long-description validity: 'engines compare' and 'rank history' both exist.

**rank movers**
1. Weekly use: yes. It is the core of Claire's Monday report.
2. Wrapper vs leverage: no API call. It is a pure local SQLite join across snapshots.
3. Transcendence proof: local SQLite. It compares the latest snapshot per (keyword, engine, country) against the snapshot at the cutoff.
4. Sibling kill: C7 `serp volatility` was killed because it measures whole-SERP churn with arbitrary thresholds, while movers answers the client's question.
5. Buildability: hand-code (`// pp:data-source local`). It calls `hintIfUnsynced(cmd, db, "snapshots")` / `hintIfStale`, and drains the rows before running per-row follow-up queries.
6. Long-description validity: 'rank history' (absorbed) and 'visibility' both exist.

**visibility**
1. Weekly use: yes. Claire and Malik both compare a client or brand against rivals weekly.
2. Wrapper vs leverage: no API call. It joins across queries in the local store.
3. Transcendence proof: local SQLite. It aggregates over every saved keyword for a domain; no single API call can do that.
4. Sibling kill: C12 `share-of-voice` was killed because its CTR weighting would be invented data. Visibility gives raw placement counts instead.
5. Buildability: hand-code (`// pp:data-source local`). It uses the sync hint helpers and the drain-first pattern.
6. Long-description validity: 'rank competitors' (absorbed) and 'rank movers' both exist.

**news watch**
1. Weekly use: yes. Malik scans brand news weekly, and the command can be scheduled.
2. Wrapper vs leverage: leverage. It makes one Google News call, dedupes against stored articles, and outputs only new items.
3. Transcendence proof: local SQLite dedupe against a service-specific content type (news).
4. Sibling kill: C15 `news digest` was killed because only Google exposes a news endpoint.
5. Buildability: hand-code (`// pp:data-source auto`). The live call comes first, then the seen-set is read and fully drained, then new articles are written outside any open read cursor, with no `store.Upsert` inside an open write transaction.
6. Long-description validity: 'ground' (absorbed) exists.

**credits**
1. Weekly use: yes. Priya checks burn after each eval batch week.
2. Wrapper vs leverage: leverage. Free `/v1/usage` is joined with a local ledger of billed calls, which `usage` alone cannot attribute.
3. Transcendence proof: a cross-source join of the live balance and local SQLite spend, using Autom's billing rules (sync 200 billed, async billed on completion, failures free).
4. Sibling kill: C16 `batch estimate` was killed because `batch run` already budgets against `/v1/usage`.
5. Buildability: hand-code (`// pp:data-source auto`). It needs a `credit_ledger` table written by the shared client hook.
6. Long-description validity: 'usage' (generated endpoint command) and 'doctor' (absorbed) both exist.

### Survivors

| # | Feature | Command | Score | Persona served | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|----------------|--------------|--------------|----------|------------------|
| 1 | Cross-engine compare | `engines compare "<query>" --engines google,bing,brave,duckduckgo,yahoo --domain x.com` | 9/10 | Malik | hand-code | This uses the Google, Bing, Brave, DuckDuckGo and Yahoo search endpoints to compute a domain overlap matrix, the domains unique to each engine, and per-engine positions, with no external dependencies. | Brief Top Workflow 3 ("same query on Google, Bing, Brave, DuckDuckGo, Yahoo; overlap and unique domains"); Build Priority 3; Product Thesis ("go further on cross-engine comparison") | Use this command to run ONE query on SEVERAL engines and compare overlap and per-engine positions. Do NOT use this command for one engine across several countries; use 'geo compare' instead. Do NOT use it for a cited source list to answer a question; use 'ground' instead. |
| 2 | Rank movers | `rank movers --since 7d --domain x.com` | 9/10 | Claire | hand-code | This uses locally saved SERP snapshots (from `rank check` and `batch run`) to compute position gains and losses and top-N entries and exits across all tracked keywords, with no external dependencies. | Brief Top Workflow 1 ("how did it move since last time"); Build Priority 2 names "movers"; Decodo report and SerpTrail in the absorb manifest show demand for trends | Use this command to see what moved across ALL tracked keywords since a cutoff. Do NOT use this command for the full trend of one keyword; use 'rank history' instead. Do NOT use it for a domain's overall footprint; use 'visibility' instead. |
| 3 | Geo compare | `geo compare "<query>" --countries fr,be,ca --domain x.com` | 8/10 | Claire | hand-code | This uses the Google search endpoint with per-country gl/hl/location (validated with the free google-countries finder) to compute the domain's position per country and the top-N overlap across countries, with no external dependencies. | Brief Build Priority 3 (`geo compare`); Top Workflow 1 ("by country and language"); Product Thesis ("geo comparison") | Use this command to run ONE query on ONE engine across SEVERAL countries or locations. Do NOT use this command to compare engines; use 'engines compare' instead. Do NOT use it to track one domain's positions over time; use 'rank history' instead. |
| 4 | Credit ledger | `credits --by keyword --since 30d` | 8/10 | Priya | hand-code | This uses the free `/v1/usage` endpoint plus a local ledger of billed calls the CLI made to compute remaining balance by source, spend by keyword, command or engine, burn per day, and days left, with no external dependencies. | Brief Table Stakes ("credit visibility before a batch"); Build Priority 5 ("`credits` summary"); docs billing rules (sync 200 billed, async billed on completion, failures free) | Use this command for remaining balance PLUS where credits went and the burn rate. Do NOT use this command for the raw account and key payload; use 'usage' instead. Do NOT use it to check the key or connectivity; use 'doctor' instead. |
| 5 | Domain visibility | `visibility x.com --vs a.com,b.com --top 10` | 8/10 | Claire, Malik | hand-code | This uses locally saved SERP snapshots to compute, for each domain, the keywords where it appears, its average and best position, and its top-3 and top-10 counts side by side, with no external dependencies. | Brief Data Layer ("queries by domain and query for rank history"); Top Workflow 1; `rank competitors` (absorbed) covers one keyword only | Use this command for one domain's footprint across EVERY saved keyword, optionally against rival domains. Do NOT use this command for who sits in the top N for one keyword; use 'rank competitors' instead. Do NOT use it for changes since the last run; use 'rank movers' instead. |
| 6 | News watch | `news watch "<brand>" --country fr` | 7/10 | Malik | hand-code | This uses the Google News endpoint plus a local news table to compute the articles not seen in earlier watches (deduplicated by canonical URL and by source plus title), with no external dependencies. | Brief Top Workflow 5 ("news watch for a brand or keyword with dedupe against the local store"); Build Priority 4 (`news watch`) | Use this command for NEW news articles about a brand or keyword since the last watch. Do NOT use this command for a cited, deduplicated source list that answers a question; use 'ground' instead. |

Scores (Domain Fit / User Pain / Build Feasibility / Research Backing):
- engines compare: 3/2/2/2 = 9
- rank movers: 3/3/2/1 = 9
- geo compare: 3/2/2/1 = 8
- credits: 3/2/1/2 = 8
- visibility: 3/2/2/1 = 8
- news watch: 2/2/2/1 = 7

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| SERP volatility (`serp volatility`) | Whole-SERP churn with arbitrary thresholds, not run weekly, and hard to verify in dogfood; per-domain movement already answers the persona's question. | rank movers |
| SERP replay (`serp replay`) | A raw read of one local snapshot; already covered by `serp diff`, framework `search`, and `--json` on saved results. | rank movers |
| SERP anatomy (`serp anatomy`) | The SERP feature blocks in Autom payloads are not documented in the brief, so output can't be verified. | engines compare |
| Keyword expand (`keywords expand`) | Each seed costs 26 or more billed autocomplete calls, and no research evidence shows demand. | geo compare |
| Engine health board (`engines health`) | N calls to free `/v1/stats` that duplicate `doctor`'s job, not a weekly persona ritual. | credits |
| Share of voice (`share-of-voice`) | The CTR weighting curve isn't in the spec or brief, so it fabricates data; raw placement counts are kept instead. | visibility |
| Cannibalization (`rank cannibal`) | Niche SEO check with no evidence in the brief or the absorbed sources. | visibility |
| Location resolve (`geo resolve`) | A thin wrapper of the absorbed google-locations, google-countries and google-languages finder commands. | geo compare |
| Cross-source news digest (`news digest`) | Only Google exposes a news endpoint, so it adds nothing beyond `news watch` and `ground`. | news watch |
| Batch cost estimate (`batch estimate`) | Already covered by `batch run`'s credit budget check against `/v1/usage`. | credits |
