# judgementTW CLI Brief

A Go CLI for Taiwan's judicial system, spanning two surfaces:
- **FJUD** (`judgment.judicial.gov.tw`) — public judgment search, 30+ courts, civil/criminal/administrative/disciplinary/constitutional
- **FJUDKM** (`fjudkm.judicial.gov.tw`) — Judicial Knowledge Base, curated case law commentary, legal research analyses

Plus the **official Open Data API** at `data.judicial.gov.tw/jdg/api/` (Auth + JList + JDoc), which is the legally-safest data layer.

## API Identity
- **Domain:** Public legal information / case law research / judicial transparency
- **Users:** Lawyers, judges, paralegals, legal researchers, academics, journalists, policy analysts, citizens checking case history
- **Data profile:** Court judgments (text + PDF), JID-keyed (e.g., `TPHM,110,毒抗,1212,20210831,1`); case law commentary categorized by civil/criminal/administrative law; static reference

## Reachability Risk
**Low for official API; Medium for site browser-replay (legal grey, not technical).**

- Official open data API: **0:00–6:00 AM Taipei service window** (5 hours/night). Token-based (6hr TTL). Requires opendata.judicial.gov.tw account. Daily JList covers ~7-day rolling window.
- FJUD search interface: ASP.NET WebForms (`Default_AD.aspx`) — heavy ViewState, requires browser-replay or session emulation; reachable 200 OK from `curl` with browser UA.
- FJUDKM: ASP.NET WebForms, login NOT required for browse, hierarchical navigation (體系查詢) + full-text search (全文檢索).
- **Lawsnote precedent (2024):** Two founders convicted, 4yr/2yr prison + NT$105M damages. **Critical clarification:** the conviction was for scraping a *private competitor* (法源資訊, Fasource), NOT for scraping the government site. But the precedent established "public data ≠ free commercial use" and broad copyright on editorial compilations. This CLI must:
  1. Default to the official open data API where possible
  2. Mark site browser-replay as opt-in with a clear `--accept-tos` style guard
  3. Auto-purge "查無資料" judgments (the API explicitly requires deleted cases be removed from caches for privacy)
  4. Document non-commercial intent in README
  5. Not redistribute corpora; fetch on-demand only

## Top Workflows
1. **Find a specific judgment** — by JID, by case number (e.g., 「110年度毒抗字第1212號」), or by parties + year
2. **Track recent rulings** — daily/weekly digest of new judgments matching keywords (e.g., `--watch "毒品危害防制條例"`)
3. **Bulk download for research** — all rulings on a given topic/court/year for empirical legal research
4. **Citation/precedent lookup** — given a case, find the lower-court ruling, the appeal, related cases by reason
5. **Court analytics** — sentencing patterns, conviction rates, timeline analysis, judge productivity (transcendence)
6. **Knowledge base browse** — find FJUDKM commentary on a legal question or case topic

## Table Stakes (must match competitors)
- Search by court + year + case type + keyword (FJUD's search UI provides this)
- Fetch full judgment text + PDF attachments (JDoc API + JFile)
- Filter by case type: 民事(V) civil / 刑事(M) criminal / 行政(A) administrative / 懲戒(P) discipline / 憲法(C) constitutional
- Date-range search (民國 calendar)
- Export to JSON, plain text, CSV
- 30+ courts: 最高、高等、地方、行政、智慧財產、商業、憲法 etc.

## Data Layer
- **Primary entities:** `judgments` (JID-keyed), `courts`, `case_types`, `judges` (extracted from text), `parties` (anonymized), `attachments`, `change_log`, `knowledge_articles` (FJUDKM)
- **Sync cursor:** JList daily array `{date: YYYY-MM-DD, list: [JID...]}` — store latest fetched date per court
- **FTS/search:** SQLite FTS5 over `JFULLCONTENT`, `JTITLE`, `JCASE`. Supports CJK tokenization (use `unicode61` tokenizer with `remove_diacritics 0` and a Chinese-friendly token rule, or trigram fallback for Chinese text)
- **Lifecycle:** Same JID across days = update (overwrite); error `查無資料` = privacy purge (auto-DELETE row + audit log entry)

## Codebase Intelligence (existing tools)
- **GOV-TW/Judicial-OD** — official open data examples; documents the JList/JDoc endpoints. **Note:** the README says "no auth", but the actual PDF spec says auth is required (the docs lag behind reality)
- **samttoo22-MewCat/judgement-scrawler** — Python+SeleniumBase scraper for >500 records; uses the FJUD search UI directly
- **whiskyinsulo/Judicial_Judgements** — open data parser/cleaner
- **biglawtw/biglaw** — open law judgment intelligent search (broader analytics platform)
- **0xyd/SunnyJudge** — wraps the *Sunny Judge* (司法陽光網) API, a different but related Taiwan judicial transparency project
- **Lawsnote (commercial)** — major incumbent, AI-powered legal Q&A + judgment search, but *currently subject to the 2024 verdict*

None of these is a single, agent-native CLI that bridges (a) the official API for legally-safe bulk and (b) the browser-replay search for advanced queries with (c) a local SQLite store for offline FTS, dedup, and analytics. That gap is the product opportunity.

## User Vision
None volunteered at briefing.

## Source Priority
- Two sources, weighted as **peers**.
- README lead: **FJUD** (judgment search) — most users will reach for it first; FJUDKM is a secondary "research mode."
- Primary data layer: **official open data API** (`data.judicial.gov.tw`), free, auth required, 0–6 AM service window.
- Secondary live-search path: **FJUD `Default_AD.aspx` browser-replay**, opt-in, fills the search-by-court-year-keyword gap that the official API does not cover.
- Tertiary reference: **FJUDKM** browse/search, opt-in, supplies curated commentary not in the raw judgment corpus.
- **Economics:** All sources are free (no paid keys). The opendata account is no-cost government registration.
- **Inversion risk:** None — neither source has a paid tier or a richer competitor spec that would tempt a swap.

## Product Thesis
**Name:** `judgementtw-pp-cli` — Taiwan judgment & judicial knowledge CLI

**Why it should exist:**
- The official open data API is **technically restrictive** (0–6 AM only, no search) — a local-store + offline-FTS CLI turns it into a 24/7 search engine.
- The Lawsnote case has chilled commercial scraping; an MIT-licensed, non-commercial, agent-native, on-demand CLI that defaults to the official API is uniquely safe.
- No existing tool combines bulk API + browser-replay search + FTS5 store + agent JSON output. Existing scrapers are research one-offs (Python+Selenium), not user/agent CLIs.
- Compound queries ("show me all 毒品危害防制條例 cases from 110-113 in High Courts that referenced the 109/01/15 amendment") need a local store — impossible with the API or website alone.

## Build Priorities
1. **Auth + sync via official API** — `auth login`, `sync` (JList → store), `get <jid>` (JDoc), with the 0–6 AM service-window guard and respectful retry/backoff
2. **Local FTS5 search** — query synced corpus offline; CJK-aware tokenization
3. **Browser-replay site search** (opt-in `--site` mode) — POST to FJUD search to fill the search-by-court-year-keyword gap; rate-limited; clear TOS warning
4. **Bulk export** — JSON/CSV/text, `--select` for field projection
5. **Privacy purge** — auto-handle `查無資料` errors; `purge --orphans`
6. **Knowledge base lookup** — FJUDKM browse + search by legal topic
7. **Transcendence (Phase 1.5c.5):** citation graph, court analytics, judge sentencing patterns, daily-digest watchlist, related-case discovery, statute-cite extraction
