# SEC Chrome DevTools capture
## User Goal Flow
Search public filing text for water. Opened fresh SEC search tab, opened DevTools Network, cleared log, entered water and submitted, exported sanitized HAR.
## Pages & Interactions
https://www.sec.gov/edgar/search/ -> keyword search water.
## Browser-Sniff Configuration
Native Chrome DevTools controlled via computer-use. Anonymous; no proxy envelope.
## Endpoints Discovered
GET https://efts.sec.gov/LATEST/search-index : HTTP200, application/json. Autocomplete keysTyped; full-text q/startdt/enddt.
## Traffic Analysis
Six requests and six usable bodies; one endpoint family. No empty-response warning in capture. Standard HTTP replay passed in prior live sample.
## Coverage Analysis
Keyword/date search covered. Submissions and companyfacts/concept contract supplied by official docs and independent live samples. Form-filtered fulltext live sample also passed. Filing-document downloads remain live dogfood work.
## Response Samples
Full-text response keys: took, timed_out, _shards, hits, aggregations, query. Captured body 61945 bytes. Samples retained beside sec-sniff spec.
## Rate Limiting Events
No 429 observed. Interactive typing generated autocomplete GETs; production CLI will send explicit query only and pace conservatively under SEC limit.
## Authentication Context
No authenticated session. Contact provided only through live SEC User-Agent requests; no contact persisted in source/spec/proofs.
