# Lexis+ Browser-Sniff Report

## 1. User Goal Flow
- Goal: Retrieve a California case and a California statute (annotated) by citation and download each as MS Word.
- Steps completed: (1) home search box: typed `27 Cal.3d 167` -> wordwheel suggest; (2) Enter -> /search/ -> single-document redirect to Tameny; (3) Download dialog -> Word -> job -> poll -> file; (4) typed `Cal Lab Code 1102.5` -> Deering's Cal Lab Code § 1102.5; (5) Download with "Include annotations" -> Word.
- Coverage: 5/5. Both test downloads verified as Word 2007+ files; statute contains "Annotations" and "Research References".

## 2. Pages & Interactions
- /zhome — typed citation in search box, pressed Enter.
- /search/ — server embeds searchId in page model; client polls queue; isSdrResult -> navigate to /document.
- /document (Tameny) — clicked Download, kept defaults (Microsoft Word), clicked Download.
- /api/document/collection/statutes-legislation/id/<id>?context=1530671 — deep link to statute; Download, ticked Include annotations, Download.

## 3. Browser-Sniff Configuration
- Backend: Claude chrome-MCP in user's logged-in Chrome (fresh tab, closed after). Chosen over browser-use because the user was logged in to their daily Chrome and quitting Chrome to transfer the profile was unnecessary; probe showed no anti-bot wall.
- Body capture: in-page fetch/XHR interceptor; for the search page, a same-origin hidden iframe with early XHR hook. Extension output filters forced shape-only extraction (no raw query strings / tokens left the browser).
- Pacing: ~1 req/s for agent-initiated fetches. No 429s.
- Proxy-envelope: not detected.

## 4. Endpoints Discovered
| Method | Path | Status | Content-Type | Auth |
|---|---|---|---|---|
| POST | /r/searchbox/1phtk/wordwheel (form body) | 200 | json | auth-required |
| GET | /search/?pdmfid&pdsearchterms&pdtypeofsearch&pdsearchtype&pdpsf&pdqttype&ecomp&earg | 200 | html (embeds "searchId":"<guid>-1") | auth-required |
| GET | /f/Search/queue/response/{searchId} | 200 | json (isSdrResult, sdrResult.queryReferences[].searchResultLinks[].href) | auth-required |
| GET | /api/document/collection/{cases|statutes-legislation}/id/{docId}?context=1530671 | 302->200 | html | auth-required |
| POST | /f/DeliveryToolbar/multiDocDialogModel/1 | 200 | json | auth-required |
| POST | /f/DeliveryToolbar/CreateMultiDocJobAsync | 200 | json {jobId, outputFormat} | auth-required |
| GET | /f/delivery/{jobId}/status | 200 | json {status: 6 success} | auth-required |
| GET | /f/delivery/{jobId} | 200 | json {downloadUri: /shared/storagearea/scratchpad/{ssaId}} | auth-required |
| GET | /f/SharedStorage/sharedStorageDelivery?ssaId&mediaType&fileName | 200 | octet-stream (docx) | auth-required |
| DELETE | /f/DeliveryDialogCart/Coreapp/allItems | 200 | json | auth-required (UI cart cleanup; not needed by CLI) |

Document descriptors observed:
- Case: contentType urn:hlct:5, productContentTypeId urn:pct:30, activeContentName "Cases", docFullPath /shared/document/cases/urn:contentItem:<id>
- CA statute (Deering's): contentType urn:hlct:15, productContentTypeId urn:pct:83, activeContentName "Codes", docFullPath /shared/document/statutes-legislation/urn:contentItem:<id>
- retrieveoptions.format "docx"; statutes: includeAnnotations "true"; cases: caseDocumentOnly/caseSummary/showHeadnotes/showShepardSignal.

## 5. Traffic Analysis
- Protocols: rest_json (/f/*), html with embedded JSON (/search/), binary file delivery.
- Auth signals: session cookies only (names not recorded). No Authorization header, no CSRF header observed on /f/* calls.
- Protection: none. probe-reachability -> standard_http (stdlib 200, surf 200).
- Warnings: binary response weak schema (expected — the docx file).

## 6. Coverage Analysis
- Covered: citation resolution (cases + statutes), Word delivery with options, annotated statute delivery.
- Not covered: multi-result search lists (items[] shape only partially observed), PDF option verification, Shepard's.

## 7. Response Samples
- queue/response (case): {"isSdrResult":true,"sdrResult":{"queryReferences":[{"referenceType":"token","submittedForm":"27 Cal.3d 167","available":true,"singleDoc":true,"contentComponentId":4861,"searchResultLinks":[{"linkType":"alternate","href":"/shared/document/cases/urn:contentItem:3S11-S000-003C-R0KM-00000-00","searchLinkRequest":{"linkText":"27 Cal. 3d 167"}}]}]},...}
- CreateMultiDocJobAsync: {"courierItemStatus":0,"courierJobStatus":0,"downloadUri":"","jobId":"301678313","outputFormat":"docx"}
- delivery status: {"id":"301678313","status":6,"statusText":"success"}
- file: Binary response: application/octet-stream, 49247 bytes (case), 60452 bytes (annotated statute)

## 8. Rate Limiting Events
- None. ~1 req/s agent-initiated.

## 9. Authentication Context
- Authenticated session: user's logged-in Chrome via chrome-MCP. No cookie values were read or stored by discovery.
- All endpoints auth-required (unauthenticated GET /zhome -> 302 to signin.lexisnexis.com).
- Printed-CLI auth: cookie (press-auth companion, domain plus.lexis.com). Cookie-replay validation: see Step 2d result appended below.
