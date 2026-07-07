# Is It Agent Ready CLI Brief

## API Identity
- **Domain:** Cloudflare's "Is Your Site Agent-Ready?" scanner (isitagentready.com). Audits a website against emerging AI-agent standards and returns a 0-5 readiness level plus prioritized, copy-paste fix advice.
- **Surface:** ONE endpoint. `POST /api/scan` with body `{"url": "<site>"}`. No auth, no key, public, read-only. ~3-7s per scan, ~25-29KB JSON.
- **Users:** developers, SEO/GEO teams, platform/devrel engineers making sites discoverable and usable by AI agents (Claude, ChatGPT, Cloudflare Agents, etc.).
- **Data profile:** per-scan report. Top-level `{url, scannedAt, level, levelName, checks{}, nextLevel{}, isCommerce, commerceSignals[]}`. The `checks` object is keyed by 5 categories; each check carries `status` (pass|fail|neutral), `message`, `evidence[]` (the literal requests/responses the scanner ran), `durationMs`, and sometimes `details`. `nextLevel.requirements[]` carries the FIX ADVICE: `{check, description, shortPrompt, prompt, specUrls[], skillUrl}`.

## Reachability Risk
- **None.** Live probes returned HTTP 200. `POST /api/scan {url:"https://example.com"}` -> level 0; `{url:"https://isitagentready.com"}` -> level 4. Missing `url` -> 400 `{"error":"Missing required field: url"}`. Unreachable target -> 200 with a `siteError{httpStatus,statusText,bodyPreview,retryAfter,server}` block (the scan endpoint itself succeeds; the TARGET site errored). `skillUrl` fix guides return 200 text/markdown.
- Request-side `checks[]` / `profile` params are **ignored** (server always runs all checks). Filtering is therefore a client-side CLI concern.
- Phase 1.9 reachability gate: **PASS** (already satisfied by these probes).

## The 21 checks (5 categories) — confirmed from live response
- **Discoverability:** robotsTxt, sitemap, linkHeaders, dnsAid (DNS for AI Discovery)
- **Content Accessibility:** markdownNegotiation (Cloudflare markdown-for-agents)
- **Bot Access Control:** robotsTxtAiRules, contentSignals, webBotAuth
- **API / Auth / MCP Discovery:** apiCatalog, oauthDiscovery, oauthProtectedResource (RFC 9728), authMd, mcpServerCard, a2aAgentCard, agentSkills (agentskills.io), webMcp (webmcp.org)
- **Commerce:** x402, mpp (mpp.dev), ucp (ucp.dev), acp (Agentic Commerce Protocol), ap2 (Agent Payments Protocol; present in API, not shown in web UI)

## Readiness level model (confirmed)
- Gate-based ladder (sequential implementations, not point accumulation): 0 Not Ready, 1 Basic Web Presence, 2 Bot-Aware, 3 Agent-Readable, 4 Agent-Integrated, 5 Agent-Native. Commerce checks (x402/mpp/ucp/acp/ap2) are tracked but do NOT affect the level. The API returns `level` + `levelName`, so the CLI never hardcodes the names. `nextLevel` names the gap to the next rung and lists the requirements to get there.
- Benchmark context (research, 62 domains): category averages Discoverability 65, Content 42, Bot Access Control 28, API/Auth/MCP 12, Commerce 8. Most sites are weakest in the discovery/auth/commerce categories, which is where the fix advice concentrates.

## Top Workflows
1. **Scan + fix:** scan a URL, see the level, and get the prioritized fixes to reach the next level (the headline workflow; advice is the point).
2. **Fetch fix guides:** each failing requirement links a real `skillUrl` SKILL.md; pull and render it so the fix is actionable in-terminal.
3. **Track over time:** re-scan a site and see whether the score went up/down and which checks flipped (regression detection).
4. **Portfolio / batch scan:** scan many sites (a company's whole web estate, or competitors) and rank them.
5. **CI gating:** fail a build when readiness drops below a threshold or a previously-passing check regresses.
6. **Competitor comparison:** scan us vs. them, diff which standards each implements.

## Table Stakes (match the web UI, then beat it)
- Scan a URL and show the level + per-check pass/fail across all 5 categories.
- Show the "how to improve" fix prompts (the web UI's "Improve the score" sheet).
- Category/check filtering (web UI does this client-side via the Customize panel; CLI does it with `--category` / `--only-failing`).
- Link to the spec URLs and SKILL.md fix guides per requirement.

## Competitor Landscape (research)
- **No standalone CLI wraps isitagentready.com.** Only programmatic path today is the official MCP server at `/.well-known/mcp.json` (single `scan_site` tool). Ours is the first dedicated terminal client, and it ships a richer MCP surface (local-store tools the official one lacks).
- **Adjacent readiness/GEO CLIs prove the workflows the web UI lacks:** searchstack-aeo (86 stars; CI, batch, audits), Cognitic-Labs/geoskills (geo-audit/fix/compare/monitor), Auriti-Labs/geo-optimizer-skill (GEO + MCP), BartWaardenburg/isagentready-skills (wraps the web UI, generates fix workflows), makeitagentready.com. These confirm demand for CI gating, batch/portfolio, history/diff, fix-export, compare, and monitor, none of which the isitagentready web UI offers. That gap is our transcendence surface (research-backed).
- **Out of scope (different products, not absorbable into a scan wrapper):** llms.txt *generation* (AnswerDotAI/llms-txt, llms-txt-hub, llms-txt-toolkit, raphaelstolt/llms-txt-php), and AI-citation tracking across ChatGPT/Perplexity (searchstack-aeo's other half). We absorb the scan+fix surface, not these.

## Data Layer (local SQLite — the transcendence engine)
- **Primary entity:** `scan` (one row per scan run): url, scannedAt, level, levelName, isCommerce, plus the full raw JSON report and a denormalized per-check status set for fast querying.
- No upstream "list" endpoint exists, so the store is populated by the CLI's own scan command on each run (history accrues locally). Cache-freshness opt-in is therefore OFF (no syncable read path); manual scan + history is the model.
- **FTS/search:** over check messages, requirement prompts, and evidence summaries so a user can grep advice across all past scans.
- This local history is exactly what the web UI cannot do: it is stateless and shows only the current scan.

## User Vision (captured at briefing)
- Bobe: **the fix advice after a scan is the priority** ("los consejos que dan después de escanear"). The CLI must make fix instructions first-class: surface the prioritized next-level prompts, fetch/render the linked SKILL.md guides, and (via local history) track which advice has been actioned across rescans.

## Product Thesis
- **Name:** Is It Agent Ready (`isitagentready-pp-cli`).
- **Why it should exist:** the web scanner gives a one-shot score; it has no memory, no batch, no CI, no diff, and you can't pipe its advice into your workflow. A CLI turns "is my site agent-ready?" into a repeatable, scriptable, fix-driven loop: scan -> see exactly what to implement -> apply -> rescan -> watch the level climb, across one site or a whole portfolio, with the fix prompts ready to paste into a coding agent.

## Build Priorities
1. **Foundation:** scan command that calls `POST /api/scan`, persists the report to local SQLite, and prints a clean level + per-category summary. Honest handling of `siteError` (target unreachable) vs API error.
2. **Advice-first (Bobe's priority):** a fix/advice command that prints the prioritized next-level requirements (description + prompt + spec links), plus a guide command that fetches and renders the `skillUrl` SKILL.md for a given check.
3. **Absorb:** category/check filtering, evidence inspection, raw JSON passthrough, level explanation — everything the web UI shows, made scriptable with `--json`/`--select`/`--compact`.
4. **Transcend (local-store only):** scan history, score-over-time diff, batch/portfolio scan with ranking, competitor comparison, CI gating, regression watch, "what advice is still open across all my sites." (Authoritative list from the novel-features subagent in Phase 1.5c.5.)
