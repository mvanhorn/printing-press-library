# Lexis+ CLI Brief

## API Identity
- Domain: Legal research (LexisNexis Lexis+, plus.lexis.com). Subscription web app; no public spec. Separate licensed "LexisNexis API" exists but is out of scope (user has web subscription only).
- Users: Practicing attorneys (here: California litigator) who retrieve known authorities by citation.
- Data profile: Case opinions (California Reports / Cal.App / Cal.Rptr reporters), California codes (annotated: Deering's California Codes Annotated is the Lexis-published annotated code). Documents are long rich-text; delivery service renders to DOCX/PDF/RTF.
- Homepage: https://plus.lexis.com/zhome (302 to signin.lexisnexis.com/lnaccess — paywall confirmed by probe).

## Reachability Risk
- High (structural): every endpoint requires an authenticated session minted at signin.lexisnexis.com; cookies expire; Lexis may change internal endpoints without notice. No community wrapper exists to check for 403 reports (only a defunct Selenium scraper for the retired LexisNexis Academic, yc-lexisnexis-scraper).
- Terms of use: LexisNexis subscriber terms restrict automated/bulk access. CLI must be on-demand, single-document, user-initiated, rate-gentle. No bulk sync / crawling.

## Top Workflows
1. "Get a document" by citation: `lexis case get "51 Cal.4th 1"` -> resolve to the document -> download as .docx.
2. Statute by citation: `lexis statute get "Cal Lab Code 1102.5"` -> Deering's annotated section -> download .docx WITH annotations (notes of decisions / case notes, research references).
3. Batch: read a list of citations (file or stdin) and download each to a folder — e.g., all authorities in a brief's TOA.
4. Search/disambiguate: when a cite resolves ambiguously, show candidate list (title, court, date, cite) and let user pick by index.
5. Session management: `auth login` imports cookies from the user's logged-in Chrome; `auth status` validates.

## Table Stakes
- Lexis+ native "Download" dialog: format (MS Word/PDF/RTF/WordPerfect), options (cover page, include annotations for statutes, footnotes, embedded links, bold search terms). CLI must expose the same format choice and the statute "annotations" toggle.
- Lexis for Microsoft Office add-in: "Get Cited Docs" from a Word document — a batch-from-citations workflow; our batch command mirrors it.

## Data Layer
- Primary entities: Document (lexis doc id / urn:contentItem, title, citation, court/code, date, content type), Download (path, format, options, timestamp).
- Sync cursor: none — no bulk sync (ToS). Local store is a cache/log of retrieved citations -> doc ids -> downloaded files, so re-requests are instant and offline-listable.
- FTS/search: local FTS over titles/cites of previously retrieved docs only.

## Product Thesis
- Name: lexis-plus-pp-cli
- Why it should exist: Pulling 20 authorities for a brief is 20 x (paste cite, wait, Download, pick Word, tick annotations, save). The CLI does it from a cite list in one command with consistent filenames.

## Build Priorities
1. Cookie auth from user's logged-in browser + session validity check.
2. Citation -> document resolution (cases, CA statutes).
3. Delivery/download as DOCX, with annotations option for statutes.
4. Batch from file/stdin with naming template.
5. Local retrieval log + offline list/search.

## Reachability Gate
- Decision: PASS
- Evidence: authenticated GET /f/Configuration/regionoptions -> 200 using press-auth session cookies (unauthenticated -> 302 to sign-in). Cookie replay works outside the browser.
