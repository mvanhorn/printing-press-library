# Serply CLI Absorb Manifest

Sources surveyed: serply-inc/mcp (official MCP server), serply-inc/serply-python (official SDK), crewAI Serply tools (SerplyWebSearchTool, SerplyNewsSearchTool, SerplyScholarSearchTool, SerplyJobSearchTool), serply-inc/notifications (scheduled SERP alerts), serply-inc/examples.

## Absorb Manifest

### Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Google web search (q, num, start, hl, gl, tbs) | serply-inc/mcp google_search | serply-pp-cli web | --json/--select, --dry-run, typed exit codes |
| 2 | Per-country proxy and device | serply-inc/mcp proxy_location, device | (behavior in serply-pp-cli web) --x-proxy-location / --x-user-agent headers | Same flags on every vertical |
| 3 | Google News search with edition (ceid) | serply-inc/mcp google_news_search | serply-pp-cli news | Pipeable, --select |
| 4 | Google Scholar search | serply-inc/mcp google_scholar_search, crewAI SerplyScholarSearchTool | serply-pp-cli scholar | Pipeable, --select |
| 5 | Bing web search | serply-inc/mcp bing_search | serply-pp-cli bing | Cross-check index |
| 6 | Google Images search | serply-python | serply-pp-cli images | --json |
| 7 | Google Video search | serply-inc/mcp video | serply-pp-cli videos | --json |
| 8 | Google Jobs search | serply-inc/mcp jobs, crewAI SerplyJobSearchTool | serply-pp-cli job-search | --json |
| 9 | Google Shopping product search | serply-inc/mcp shopping | serply-pp-cli products | --json |
| 10 | Google Maps place search (path form) | serply-inc/mcp maps | serply-pp-cli maps | --json |
| 11 | MCP server exposing every vertical | serply-inc/mcp | (behavior in serply-pp-cli web) exposed as an MCP tool by serply-pp-mcp, as is every command | Local stdio server, no session id needed |

Not absorbed: serply-inc/mcp `scrape_url` and Reddit tools are outside the documented REST search spec this CLI is generated from.

### Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Domain rank check | rank | hand-code | Scans the live result list for a domain and reports its position, a question no endpoint answers directly | Use this command to find where one domain ranks for a query. Do NOT use it to list all results; use 'web' instead. |
| 2 | SERP change diff | serp diff | hand-code | Compares against a locally stored snapshot of the same query and location; the API keeps no history | Use this command to see what changed in a SERP since the last run. Do NOT use it for a one-off search; use 'web' instead. |
| 3 | Cited research brief | research | hand-code | Fans out to three verticals concurrently and merges them into one deduplicated, numbered citation list | Use this command for a multi-source cited brief on a topic. Do NOT use it when one vertical is enough; call 'web', 'news' or 'scholar' directly. |

Scope note: three transcendence features instead of the default minimum of five, by operator direction (2-4 genuinely useful features; credits cost money).
