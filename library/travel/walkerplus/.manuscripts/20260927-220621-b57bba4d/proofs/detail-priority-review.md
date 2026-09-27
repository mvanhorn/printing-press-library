# Bounded detail selection review

The second review identified a valid bounded-discovery defect: candidates with missing listing city/category facts could sort ahead of fully matching candidates and exhaust the detail budget.

Detail selection now stably prioritizes fully listing-supported requested location/category filters, then uses remaining budget for unknowns. Requested sorting is preserved within each tier; returned events retain the requested sort, including original listing order for source sorting. Coverage explains this choice when both tiers occur. Strict final filters and all request caps are unchanged.

Independent regressions reproduced the failure, then passed six cases across relevance/start/source sorts with one- and two-detail budgets. They check selected IDs, actual requested pages, exact request counts, concurrency, truncation, final location/category facts and confirmed attendance. Core tests also cover partial evidence and end-date sorting.

Final integrated verification: all 11 test packages pass, six further packages have no tests, vet and both builds pass. Fresh uncached live E2E passes eight tests plus 16 subtests in 29.230 seconds. Full Printing Press live gate passes 50/50 executed checks with zero failures and 40 separately disclosed framework/inapplicable skips. Publication validation passes every check, including the fresh source fingerprint and reachable vulnerability check.

The local workspace and canonical library contain the reviewed source and refreshed CLI/MCP bundles. Original artifacts remain preserved locally; public evidence omits host paths and raw cache/capture payloads.
