# MCP Market Browser-Sniff Discovery Report

**Tool used:** chrome-devtools MCP (user pre-approved in initial invocation). Primary goal walked: "search MCP Market catalog for a server/client/skill and view its detail" (read-only discovery/browse flow — MCP Market has no cart/checkout; "download" is a skill snapshot the CLI should surface, not silently execute).

## Reachability
`cli-printing-press probe-reachability` against `mcpmarket.com` (root, `/server/<slug>`, `/server?page=2`) consistently returns:
- stdlib HTTP: `403`/`429` with `x-vercel-mitigated: challenge` (Vercel Security Checkpoint)
- surf-chrome (Chrome TLS fingerprint): `200`
- **mode: `browser_http`** → printed CLI ships Surf transport, no live browser / clearance cookie needed at runtime.

`docs.mcpmarket.com` and `app.mcpmarket.com` are not behind the checkpoint (plain `curl` gets 200 / 401 respectively).

## Confirmed replayable surfaces

| Path pattern | Method | Shape | Auth |
|---|---|---|---|
| `/server/<slug>` | GET | HTML + JSON-LD `SoftwareApplication` | none |
| `/client/<slug>` | GET | HTML + JSON-LD `SoftwareApplication` | none |
| `/tools/skills/<slug>` | GET | HTML + JSON-LD `SoftwareApplication` + `FAQPage`; SKILL.md frontmatter+body in DOM (`role=tabpanel`) | none |
| `/search?q=<query>` | GET | HTML + JSON-LD `SearchResultsPage`→`ItemList` | none |
| `/categories/<slug>` | GET | HTML + JSON-LD `ItemList` (category members) | none |
| `/leaderboards` | GET | HTML + JSON-LD `ItemList` (top 100 servers, `position` 1-100) | none |
| `/tools/skills/leaderboard` | GET | HTML + JSON-LD `ItemList` (top 100 skills) | none |
| `/daily`, `/daily/skills` | GET | HTML + JSON-LD `ItemList` (today's trending) | none |
| `/api/similar-tools?slug=&category=&type=server\|skill` | GET | `application/json` array of `{id,name,slug,description,owner:{name,avatar,url}}` | none |
| `app.mcpmarket.com/api/v1/me` | GET | `application/json` (401 unauthenticated; documented) | Bearer `sk_user_...` |

## JSON-LD sample (server detail, `/server/firecrawl`)
```json
{"@context":"https://schema.org","@type":"SoftwareApplication","name":"Firecrawl","description":"Empowers LLMs with advanced web scraping capabilities...","url":"https://firecrawl.dev","applicationCategory":"ServerApplication","keywords":"web scraping, ...","featureList":["Firecrawl Scrape","Firecrawl Map","Firecrawl Search","Firecrawl Crawl","Firecrawl Check Crawl Status","Firecrawl Extract"],"interactionStatistic":{"userInteractionCount":4200},"author":{"name":"mendableai","url":"https://github.com/mendableai"},"isRelatedTo":[...]}
```

## Not captured (out of shipping scope this run)
Authenticated org/toolkit/team/skill-install-sync management endpoints beyond `GET /me` — real (confirmed live, Bearer-token gated) but undocumented past the one example, and no test account/session was available to capture live traffic for them. Scoping the CLI to the public catalog (read-only, no auth) plus a thin `auth`/`doctor` check against `/me` for users who do have a token avoids inventing endpoint shapes.

## Runtime implication for Phase 2
`response_format: html` endpoints with `html_extract` targeting the JSON-LD `<script type="application/ld+json">` blocks (parse as JSON, not link-scrape) for server/client/skill/category/leaderboard/daily/search. One plain `response_format: json` endpoint for `similar-tools`. No clearance cookie, no live browser at CLI runtime — Surf/Chrome-fingerprint HTTP transport throughout.
