# NameThatUI CLI Brief

## API Identity
- Domain: public visual dictionary for UI elements, component anatomy, platform API symbols, visual styles, and implementation prompts.
- Users: coding agents, designers, frontend engineers, and native-app engineers who can describe what they see but do not know the canonical term.
- Data profile: 71 element records on the homepage, 14 style records, 111 sitemap URLs, 60+ AppKit-to-SwiftUI translations, comparison guides, RSS updates, and a public `POST /api/search` semantic reranker.

## Reachability Risk
- Low. Direct stdlib HTTP and Surf both returned HTTP 200; `probe-reachability` classified the runtime as `standard_http` with 0.95 confidence.
- Cloudflare headers and challenge-support scripts are present, but the useful HTML, RSS, sitemap, and search endpoint replay successfully without browser state.

## Users
- **Coding agents drafting or repairing UI:** They receive imprecise requests such as "the little pill behind the menu-bar icon" and need canonical component names, exact platform symbols, anatomy, and paste-ready implementation/debug prompts before editing code.
- **Frontend engineers choosing a pattern:** They repeatedly decide between visually similar controls such as select, combobox, dropdown, popover, and tooltip; they need concise decision rules, states, accessibility cautions, and ARIA/API mappings.
- **Native macOS engineers translating design language:** They move between product language, AppKit, and SwiftUI terminology and need a deterministic lookup for the same concept across frameworks.
- **Designers and design-system authors documenting intent:** They identify visual styles and component parts, compare common confusions, and share source-backed references with implementers or agents.

## Top Workflows
1. Describe an element badly and identify its canonical name, platform, API symbol, and relevant part.
2. Fetch a complete component brief: aliases, anatomy, implementation prompt, debug prompt, code mappings, and related components.
3. Describe a visual treatment and identify the style, its defining signals, common confusions, accessibility cautions, and implementation starting points.
4. Translate a macOS UI concept between plain language, AppKit, and SwiftUI terminology.
5. Sync the public catalog locally, then search, filter, compare, and retrieve guidance offline or in deterministic agent workflows.

## Table Stakes
- Fuzzy search by colloquial description and synonym.
- Browse/filter by platform, category, component, style, and API symbol.
- Component definitions, anatomy, aliases, related entries, and usage guidance.
- Accessibility and implementation guidance with copyable agent prompts.
- Cross-system/framework terminology comparison.
- Structured JSON, field selection, bounded output, and stable agent-facing commands.

## Competitor Baseline
- UX Components: its public MCP provides natural-language `lookup`, scenario-based `recommend`, side-by-side `compare`, and `smart_query` intent routing, with Markdown/JSON/XML output. Results include anatomy, states, when-to-use/avoid guidance, cross-design-system names, alternatives, watch-outs, related components, ARIA mappings, and design-system/icon directories.
- The Component Gallery: canonical component names and aliases, real-world design-system examples, usage/accessibility/code references, and Pagefind search.
- NameThatUI's advantage is narrower but sharper: colloquial-description resolution, platform API symbols, component-part identification, debugging prompts, macOS translation, and a governed visual-style atlas.

## User Pain Points
- People can see an interface element but cannot name it precisely enough to search for guidance or prompt a coding agent.
- Similar-looking patterns are routinely conflated: popover/dropdown/tooltip, modal/drawer/sheet, badge/chip/pill/tag, glassmorphism/Liquid Glass.
- Designers and engineers use different names across plain language, ARIA/HTML, AppKit, SwiftUI, and component libraries.
- Generic AI answers can hallucinate terminology; agents need source-backed wording and exact API symbols.

## Data Layer
- Primary entities: elements, element parts, API symbols, styles, style signals, translations, comparisons, related links, and catalog snapshots.
- Sync cursor: sitemap URL plus response metadata/ETag; RSS publication timestamps help identify new entries.
- FTS/search: names, aliases, fuzzy phrases, taglines, descriptions, parts, API symbols, prompts, debug prompts, style signals, and framework mappings.

## User Vision
- No authentication. Agents should call the public site to obtain design guidance and references for UI components and styles.
- The CLI must produce compact structured results suitable for coding-agent context, while preserving links back to NameThatUI as the source.

## Product Thesis
- Name: NameThatUI CLI
- Why it should exist: turn vague visual language into exact, source-backed component/style terminology and paste-ready implementation guidance, with a local searchable mirror for fast and deterministic agent use.

## Build Priorities
1. `identify` and `style identify`: replay the site's search/rerank flow and enrich hits from canonical public pages.
2. `sync`, `search`, `component get/list`, `style get/list`, and local SQLite/FTS persistence.
3. `prompt`, `debug-prompt`, `anatomy`, `translate`, `compare`, and compact agent views.
4. Freshness/diff commands that detect newly added or materially changed guidance.

## Browser-Sniff Decision
- Pre-approved via the website choice; anonymous capture only.
- Browser-Sniff found replayable HTML, RSS, sitemap, Next.js RSC, and `POST /api/search` surfaces. The shipped runtime does not need a resident browser.

## Crowd-Sniff Decision
- Skipped. No NameThatUI SDK or community wrapper was found; browser capture plus public HTML/RSS/sitemap provides the authoritative contract.

## Ecosystem Search
- Direct searches for a NameThatUI CLI, SDK, npm/PyPI client, Claude plugin/skill, MCP server, automation script, or public GitHub implementation returned no matching tool. The official Claude external-plugin listing also contained no NameThatUI integration.
- UX Components is the closest agent-facing competitor. Its public no-auth MCP exposes four tools (`lookup`, `recommend`, `compare`, `smart_query`) and supports Markdown, JSON, and XML responses. Live calls confirmed recommendation reasoning, states, cross-system names, alternatives, watch-outs, and side-by-side decision rules.
- The Component Gallery is the closest broad human reference: canonical names and aliases, real-world examples across design systems, technology/feature filters, usage guidance, accessibility references, and code-example links.
- Magic UI MCP is an adjacent component-code registry with list/search/get/install-source workflows. It is intentionally out of scope: this CLI identifies and explains patterns from NameThatUI; it does not install third-party UI code.
- No NameThatUI source repository or SDK/MCP repository was discovered, so MCP source extraction and DeepWiki analysis were not applicable. Component Gallery's site identifies a Puppeteer screenshot helper, but not a public repository for its underlying catalog.
