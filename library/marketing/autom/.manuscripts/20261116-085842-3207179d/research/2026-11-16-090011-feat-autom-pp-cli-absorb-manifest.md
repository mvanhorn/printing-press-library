# Autom CLI Absorb Manifest

Sources: Autom hosted MCP (mcp.autom.dev), Pipedream Autom MCP (Google/Bing/Brave), Composio Autom, the user's Autom integrations (LangChain, crewAI, Dify, Haystack), Serply printed CLI (peer SERP API), seo-position-tracker (Python CLI), Decodo rank-tracker (Python CLI), SerpTrail (SerpApi self-hosted rank tracker).

### Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Google web search (organic, location, gl/hl/lr, num, page, safe, time range) | Autom MCP + Pipedream Autom "Google Search" + Serply `web` | (generated endpoint) google search | Every documented param as a flag, --json/--select, `fields` projection, --dry-run |
| 2 | Google Search Light (fast, fewer fields) | Autom MCP | (generated endpoint) google search-light | Cheaper default for agents, same flags |
| 3 | Google autocomplete suggestions | Autom MCP | (generated endpoint) google autocomplete | Keyword ideas from the terminal |
| 4 | Google News | Autom MCP + Dify Autom "Google News" + Serply `news` | (generated endpoint) google news | Time range, country, language flags |
| 5 | Google Images | Autom MCP + Serply `images` | (generated endpoint) google images | Same |
| 6 | Google Videos | Autom MCP + Serply `videos` | (generated endpoint) google videos | Same |
| 7 | Google Shopping | Autom MCP | (generated endpoint) google shopping | Price/merchant fields selectable |
| 8 | Google Maps places | Autom MCP + Serply `maps` | (generated endpoint) google maps | Same |
| 9 | Google Resolve (redirect/URL resolution) | Autom MCP | (generated endpoint) google resolve | Scriptable |
| 10 | Bing search | Autom MCP + Pipedream Autom "Bing" + Serply `bing` | (generated endpoint) bing | Known `page` server bug surfaced as a clear error |
| 11 | Brave search, images, videos | Autom MCP + Pipedream Autom "Brave" | (generated endpoint) brave search / images / videos | Same |
| 12 | DuckDuckGo search | Autom MCP + seo-position-tracker engine | (generated endpoint) duckduckgo | Same |
| 13 | Yahoo search | Autom MCP + seo-position-tracker engine | (generated endpoint) yahoo | Same |
| 14 | Multi-engine fallback search | Autom MCP + Dify/LangChain Autom "Web Search" | (generated endpoint) search | 404 "no engine had a result" reported distinctly |
| 15 | Account usage (quota, packs, credits, rate limits, key) | Autom MCP + Autom docs best practice | (generated endpoint) usage | Free; drives credit budgets |
| 16 | Engine health stats (last hour) | Autom MCP | (generated endpoint) stats | `engine`/`path` enforced client-side (server requires one) |
| 17 | Country / language / location finders | Autom MCP | (generated endpoint) finder google-countries / google-languages / google-locations | Offline cache of finder answers |
| 18 | Async jobs (`is_async`, poll `/v1/jobs/{id}`) | Autom docs (global params) | (behavior in autom-pp-cli jobs get) `--is-async` on search commands returns a job id; `jobs get --wait` polls free | No hand-rolled polling |
| 19 | Field projection (`fields`) | Autom docs (global params) | (behavior in autom-pp-cli google search) root `--fields` sends Autom's server-side projection on every endpoint command; `--select` filters locally | Smaller payloads for agents |
| 20 | Rank of a domain for a keyword, multi-engine, CSV/JSON/TXT export | seo-position-tracker + Serply `rank` | autom-pp-cli rank check | Any engine Autom covers, saved to SQLite, CSV via --csv |
| 21 | Keyword file batch with concurrency and delay | Decodo rank-tracker `check --keywords --concurrency --delay` | autom-pp-cli batch run | Credit budget checked against /v1/usage first |
| 22 | Rank history and trend per keyword | Decodo `report --keyword`, SerpTrail history | autom-pp-cli rank history | Local, offline |
| 23 | Competitors in the top N and diff vs last run (entrants, exits, movers) | Decodo `competitors`, SerpTrail competitors | autom-pp-cli rank competitors | Any engine |
| 24 | SERP snapshot diff since last run | Serply `serp diff` | autom-pp-cli serp diff | Diff any two saved snapshots, not only the last |
| 25 | Cited research brief across web/news/scholar | Serply `research` | autom-pp-cli ground | Across Google, Bing, Brave, DuckDuckGo and News, deduplicated and numbered |
| 26 | Health check (key, DB, API, staleness, lifetime calls) | Decodo `doctor` | (behavior in autom-pp-cli doctor) config, paths, key presence and API reachability; key validity via the free `usage` command | 401 key vs rate-limit classification (exit 4 vs 7) |
| 27 | Export history CSV/JSON, prune old results | Decodo `export`, `prune` | (behavior in autom-pp-cli rank history) --csv/--json export; prune via framework store commands | No extra tooling |
| 28 | MCP tools for agents | Autom MCP, Pipedream, Composio | (behavior in autom-pp-cli agent-context) every Cobra command, novel ones included, is mirrored as an MCP tool by the autom-pp-mcp binary | Local, includes the novel commands |

### Transcendence (only possible with our approach)
| # | Feature | Command | Score | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|-------|--------------|------------------------|------------------|
| 1 | Cross-engine compare | engines compare | 9/10 | hand-code | Fans one query out to the Google, Bing, Brave, DuckDuckGo and Yahoo endpoints and joins the results into a domain overlap matrix, unique domains per engine and per-engine positions; `--save` stores the snapshots | Use this command to run ONE query on SEVERAL engines and compare overlap and per-engine positions. Do NOT use this command for one engine across several countries; use 'geo compare' instead. Do NOT use it for a cited source list to answer a question; use 'ground' instead. |
| 2 | Rank movers | rank movers | 9/10 | hand-code | Local SQLite join across saved snapshots: position gains and losses, top-N entries and exits across every tracked keyword since a cutoff | Use this command to see what moved across ALL tracked keywords since a cutoff. Do NOT use this command for the full trend of one keyword; use 'rank history' instead. Do NOT use it for a domain's overall footprint; use 'visibility' instead. |
| 3 | Geo compare | geo compare | 8/10 | hand-code | One query on one engine across several countries (codes checked with the free finder first): per-country position and top-N overlap | Use this command to run ONE query on ONE engine across SEVERAL countries or locations. Do NOT use this command to compare engines; use 'engines compare' instead. Do NOT use it to track one domain's positions over time; use 'rank history' instead. |
| 4 | Credit ledger | credits | 8/10 | hand-code | Joins the free /v1/usage balance with a local ledger of billed calls this CLI made: spend by keyword, command or engine, burn per day, days left | Use this command for remaining balance PLUS where credits went and the burn rate. Do NOT use this command for the raw account and key payload; use 'usage' instead. Do NOT use it to check the key or connectivity; use 'doctor' instead. |
| 5 | Domain visibility | visibility | 8/10 | hand-code | Local SQLite aggregate over every saved keyword: keywords where a domain appears, average and best position, top-3/top-10 counts, side by side with rival domains | Use this command for one domain's footprint across EVERY saved keyword, optionally against rival domains. Do NOT use this command for who sits in the top N for one keyword; use 'rank competitors' instead. Do NOT use it for changes since the last run; use 'rank movers' instead. |
| 6 | News watch | news watch | 7/10 | hand-code | One Google News call deduplicated against the local news table (canonical URL, source plus title); prints only new articles | Use this command for NEW news articles about a brand or keyword since the last watch. Do NOT use this command for a cited, deduplicated source list that answers a question; use 'ground' instead. |

Brainstorm audit trail (personas, 16 candidates, 10 kills): see the novel-features brainstorm file in this research directory.
