# Scientific Consensus CLI Brief

## API Identity
- **Domain:** Scientific research intelligence — aggregate multiple scholarly databases and transform large paper collections into actionable *evidence summaries* answering "What does the scientific evidence currently say?"
- **Users:** Medical researchers, clinicians, public-health professionals, PhD/university students, data scientists, journalists, science communicators, evidence-based-medicine (EBM) practitioners.
- **Data profile:** Scholarly works (title, abstract, year, venue, type, DOI/PMID), citation counts, authorships + institutions, topics/concepts, MeSH publication types, funders, TLDRs. High-gravity entities: **Work**, **Author**, **Institution**, **Source/Journal**, **Topic**, **Funder**.

## Source Priority (combo CLI — confirmed via Multi-Source Priority Gate)
- **Primary: OpenAlex** — spec state: documented public REST API (keyless); auth: free. Provides the base spec + headline commands. Richest structured metadata: `cited_by_count`, `authorships[].institutions`, `primary_topic`/`topics`, `type`, `abstract_inverted_index`, `referenced_works`, `counts_by_year`. **Confirmed live**: `works?search=` → 1.37M hits for "vitamin d"; `group_by=type` → article/review/preprint/letter/editorial/... enum.
- **Secondary: PubMed E-utilities** — esearch + esummary/efetch. **Gold source for evidence classification**: `pubtype` returns authoritative `Systematic Review`, `Meta-Analysis`, `Randomized Controlled Trial`, `Review`, `Editorial`, etc. Also MeSH terms. Keyless (10 req/s with `api_key`, 3/s without). Confirmed live.
- **Secondary: Crossref** — `is-referenced-by-count`, `type`, **`funder[]`** (funding analysis), license, references. Keyless (polite pool via `mailto`). Confirmed live.
- **Secondary: Europe PMC** — full-text + biomedical `abstractText`, `pubTypeList`, open-access status. Keyless. Confirmed live (569K hits "vitamin d").
- **Tertiary/optional: Semantic Scholar** — `tldr` (AI summary), `publicationTypes`, citation intents. **Confirmed rate-limited: HTTP 429 keyless.** Per spec ("when available") → best-effort, graceful degradation, opt-in `SEMANTIC_SCHOLAR_API_KEY`.
- **Economics:** All sources free/keyless. No paid tiers. Optional AI keys (`OPENAI_API_KEY`/`ANTHROPIC_API_KEY`/`GEMINI_API_KEY`) enhance summarization only — every command works without them.
- **Inversion risk:** OpenAlex has no single OpenAPI doc but is the richest source and the confirmed primary; do NOT let a cleaner secondary spec invert the ordering. Generate base from a hand-authored OpenAlex internal spec.

## Reachability Risk
- **None** for OpenAlex/PubMed/Crossref/Europe PMC — all returned 2xx with expected fields on live probe (2026-06-20).
- **Low/known** for Semantic Scholar — 429 keyless. Mitigation: adaptive rate limiter, treat as optional enrichment, never block a command on S2.
- Probe-safe endpoints used: `GET /works` (OpenAlex), `GET /search` (Europe PMC, Crossref), `esearch.fcgi` (PubMed). All read-only.

## Top Workflows
1. **"What does the evidence say about X?"** — run `consensus "<claim>"`: fan out across sources, classify each study's stance (supports / refutes / mixed / inconclusive) + evidence tier, return a Consensus Score + Confidence + evidence pyramid.
2. **Triage a topic into a reading list** — `search`/`curate`/`landmark`: rank by citations/relevance/recency, dedupe across sources by DOI, export Markdown/BibTeX.
3. **Map the evidence base** — `evidence "<topic>"`: distribution across study designs (meta-analysis → case report), so a clinician sees whether a claim rests on RCTs or case series.
4. **Track how a field moves** — `timeline`/`trends`/`emerging`/`drift`: publication velocity, milestone years, emerging vs declining subtopics.
5. **Find disagreement & gaps** — `controversies` (conflicting conclusions), `gaps` (understudied populations, missing RCTs/replication).
6. **People & institutions** — `authors`/`institutions`: citation leaders, collaboration networks, rising research centers.

## Table Stakes (must match incumbents)
- Cross-source search + dedup (vs Consensus.app, Elicit, Scite, PubMed, Europe PMC).
- Citation counts + ranking (vs Semantic Scholar, Google Scholar).
- Study-type / evidence classification (vs Scite, Consensus "study types").
- Stance/consensus meter (vs **Consensus.app** consensus meter; **Scite.ai** supporting/contrasting citations).
- TLDR/AI summaries (vs Semantic Scholar, Elicit) — but optional & keyless-degrading.
- Export to BibTeX/CSV/JSON/Markdown (vs Zotero/reference managers, Litmaps).
- Offline persistence + repeat queries (NO incumbent does this — local SQLite is our moat).

## Data Layer
- **Primary entities (SQLite):** works (id, doi, pmid, title, abstract, year, type, venue, cited_by, source_provenance, raw JSON), authors, institutions, sources/journals, topics, funders. Join tables: work_authors, work_topics, work_funders.
- **Derived/cache tables:** consensus_runs (query, scores, generated_at), evidence_classifications (work_id, design, tier, method, confidence), citations_by_year.
- **Sync cursor:** per-source incremental (`from_publication_date` / cursor paging on OpenAlex; date ranges on others).
- **FTS:** FTS5 over title+abstract for offline `search`/`sql`.

## Codebase Intelligence (reference CLIs)
- Reuse architecture from `thelancet` and `nejm` (both OpenAlex-backed, already in `$PRESS_LIBRARY`): command style, `--agent`/`--json`/`--select`/`--compact` flags, `data-source auto|live|local`, doctor, sync, SQL/search, profiles, deliver sinks.
- Auth: none required (keyless). Optional `*_API_KEY` env vars (PubMed, Semantic Scholar, AI providers) raise limits / enable summarization.
- Rate limiting: OpenAlex polite pool via `mailto`; PubMed 3/s keyless; S2 adaptive backoff. Use `cliutil.AdaptiveLimiter` per-source.

## Evidence Engine (design — key research finding)
- OpenAlex `type` is coarse (article/review/preprint/letter/editorial) — **insufficient** for the spec's 10-tier study taxonomy.
- **Classification cascade per work:**
  1. If PMID available → PubMed `pubtype` (authoritative: Meta-Analysis, Systematic Review, Randomized Controlled Trial, Observational Study, Review, Editorial, ...).
  2. Else if Semantic Scholar `publicationTypes` available → use it.
  3. Else → title/abstract keyword heuristics (regex tiers: "umbrella review" > "meta-analysis" > "systematic review" > "randomi[sz]ed controlled trial" > "cohort" > "case-control" > "cross-sectional" > "case series" > "case report" > "narrative review").
- Evidence tier ranking (pyramid weight): umbrella/meta-analysis/systematic-review (top) → RCT → cohort → case-control → cross-sectional → case series → case report → narrative review/editorial (bottom).

## Consensus Engine (design)
- For each retrieved work, derive a **stance** toward the query claim:
  - Lexical/heuristic baseline (keyless): sentiment + negation cues in title/abstract ("no association", "not effective", "reduces risk", "improves", "no significant difference") → supports / refutes / mixed / inconclusive.
  - Optional AI-enhanced stance (when an AI key is set): per-abstract classification.
- **Consensus Score** = evidence-tier-weighted net stance (supports − refutes) / total weighted; **Confidence** = f(study count, top-tier presence, citation mass, source agreement); **Evidence Strength** = highest tier present × volume. Report study/publication/citation counts.

## User Vision (from original spec, prior session)
- "The most powerful open-source scientific evidence and consensus CLI in the Printing Press ecosystem." Functionality researchers normally need multiple commercial platforms (Consensus.app + Scite + Elicit + Litmaps) to obtain. Must work fully keyless; AI optional. Architecture must allow future expansion to all scientific disciplines.

## Product Thesis
- **Name:** scientific-consensus (binary: `scientific-consensus`).
- **Why it should exist:** Consensus.app, Scite, and Elicit are paywalled, closed, web-only, and not agent-native. No free tool aggregates OpenAlex + PubMed + Crossref + Europe PMC, classifies evidence by study design, scores consensus, AND persists everything to a local SQLite store you can query offline with `--json` for agents. This is that tool.

## Build Priorities
1. **Foundation:** OpenAlex base spec → generate; SQLite data layer for works/authors/institutions/sources/topics; sync + FTS search + SQL.
2. **Absorb (match incumbents):** search, authors, institutions, journals, timeline, trends, curate, landmark, export, cited-by — across sources with DOI dedup.
3. **Transcend (the engines):** `consensus`, `evidence` (+ pyramid), `compare`, `gaps`, `controversies`, `emerging`, `drift`, `reproducibility`, `quality`, `funding`, `watch` — only possible because everything is normalized into one local store.
4. **Polish:** optional AI summarization layer, exports (MD/JSON/CSV/HTML), doctor (per-source health + key detection).

## Reachability Gate
- Decision: PASS
- Evidence: OpenAlex GET /works returned 200 with full metadata on 2026-06-20 live probe (count=1.37M for "vitamin d"). PubMed/Crossref/Europe PMC also 200. Semantic Scholar 429 (optional, degrades).
