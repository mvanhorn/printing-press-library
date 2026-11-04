# SEC EDGAR requested scope addition (research draft)
User explicitly requested direct SEC EDGAR search, filings and company financial facts. The original California and Data.gov catalogs retain equal prominence; SEC is a separate direct source under edgar commands.

## Official contract evidence
- https://www.sec.gov/search-filings/edgar-application-programming-interfaces : public read APIs require no authentication or API key.
- Submissions: GET https://data.sec.gov/submissions/CIK##########.json . Ten-digit zero-padded CIK. Recent filings column arrays plus filings.files for older history. Do not claim recent array is complete history.
- Company facts: GET https://data.sec.gov/api/xbrl/companyfacts/CIK##########.json . Standard taxonomy entity-wide facts; preserve taxonomy, concept, units, period, form, filed date, accession and amendments. No invented canonical/latest financial statement aggregation.
- Company concept: GET https://data.sec.gov/api/xbrl/companyconcept/CIK##########/{taxonomy}/{concept}.json . Fact arrays separated by units.
- Filing documents and indexes: https://www.sec.gov/search-filings/edgar-search-assistance/accessing-edgar-data . Archive documents free to download. Exact accession and document names must come from upstream evidence.
- Fair access: maximum 10 requests/second across machines, descriptive User-Agent with contact email. Default conservative pacing and cache; contact requested from user, do not invent one or reuse unrelated account information.

## Proposed baseline additions
1. Company lookup by ticker or CIK with explicit resolution.
2. Filing search filtered by company, form and date, traversing older submission files within bounded limits.
3. Filing metadata, document listing and explicit selected document download.
4. Company XBRL facts and concept selection with units/period/filing provenance.
5. Full-text filing search: research SEC search surface separately before promise; public data.sec.gov APIs alone do not supply full-text search. No substitution with a paid third-party API.

## Pending
User-Agent contact before live SEC requests; official source reachability; current full-text search contract and browser discovery gate; revised absorb manifest and approval. Prior 14-capability plan has not been approved and generation has not started.

## Live read-only proof
Company submissions, companyconcept Assets, companyfacts and EFTS full-text search all returned HTTP 200 with expected JSON structures. Real user-authorized contact supplied in request header only. Config env: SEC_USER_AGENT.
