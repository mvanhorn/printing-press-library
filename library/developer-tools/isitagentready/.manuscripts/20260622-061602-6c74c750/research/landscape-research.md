# Is It Agent Ready - Landscape Research (Phase 1)

## No existing CLI wrapper
No standalone CLI wraps isitagentready.com as of research. Only programmatic path:
official MCP server at https://isitagentready.com/.well-known/mcp.json (single `scan_site` tool,
Streamable HTTP, readOnlyHint). Our CLI would be the first dedicated terminal client.

## Competing tools (absorb workflow features; most are different products)
- searchstack-aeo (Python, 86 stars) - 22 commands: AI citation tracking, llms.txt gen/validate, GSC, onpage audit, CI.
- Cognitic-Labs/geoskills - geo-audit, geo-fix-content, geo-fix-schema, geo-fix-llmstxt, geo-compare, geo-monitor.
- Auriti-Labs/geo-optimizer-skill (Python) - 47 GEO methods, MCP server, llms.txt/robots gen.
- BartWaardenburg/isagentready-skills - wraps isitagentready.com via web UI; 42 checkpoints; generates fix workflows.
- makeitagentready.com (TS) - robots/llms/sitemap/MCP/OAuth checks.
- llms.txt tooling: AnswerDotAI/llms-txt, thedaviddias/llms-txt-hub (+CLI), abovefear/llms-txt-toolkit, raphaelstolt/llms-txt-php, mcp-llms-txt-explorer.
- GEOAudit (Chrome ext), Otterly AI (web), vdalhambra/siteaudit-mcp.
Takeaway: the cross-cutting CLI workflows (CI gate, batch/portfolio, history/diff, fix-export, compare, monitor, SARIF)
are proven in adjacent tools but ABSENT from the isitagentready web UI. That gap = our transcendence surface.

## 5-level gate model (confirmed)
0 Not Ready | 1 Basic Web Presence | 2 Bot-Aware | 3 Agent-Readable | 4 Agent-Integrated | 5 Agent-Native.
Gate-based (sequential implementations, not point accumulation). Commerce checks (x402/mpp/ucp/acp/ap2) tracked but
do NOT affect level. Real-world category averages across 62 domains: Discoverability 65, Content 42,
Bot Access Control 28, API/Auth/MCP 12, Commerce 8.

## Standard spec URLs (for accurate help/advice)
robots.txt RFC 9309 | sitemap sitemaps.org/protocol | Link headers RFC 8288 | DNS-AID draft-mozleywilliams-dnsop-dnsaid |
Markdown negotiation developers.cloudflare.com/fundamentals/reference/markdown-for-agents | Content Signals contentsignals.org |
Web Bot Auth draft-meunier-web-bot-auth-architecture (HTTP Message Sigs RFC 9421) | API Catalog RFC 9727 |
OAuth discovery RFC 8414 | OAuth Protected Resource RFC 9728 | auth.md (community proposal) |
MCP Server Card SEP-1649/2127 | A2A Agent Card google.github.io/A2A | Agent Skills agentskills.io |
WebMCP webmcp.org | llms.txt llmstxt.org | x402 x402.org | MPP mpp.dev | UCP ucp.dev | ACP agenticcommerce.dev | ap2 (Agent Payments Protocol).

## Power-user workflows (terminal CLI)
CI gate per PR (exit nonzero below min-level); portfolio/agency batch scan + rank; score-over-time diff;
fix-prompt export for coding agents; competitor comparison; regression monitor (cron); standard-specific deep-dive;
bulk fix-plan generation.

## Pain points the web UI does NOT solve
No history/memory; no CI integration/exit codes/JSON; no batch; no bulk raw-response inspection;
fix advice locked in the browser (cannot pipe to a coding agent or save).
