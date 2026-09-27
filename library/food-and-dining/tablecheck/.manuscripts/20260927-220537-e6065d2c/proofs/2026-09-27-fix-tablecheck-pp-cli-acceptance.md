# Acceptance report: TableCheck

Level: Full Dogfood, already authorized in approved scope. Gate: PASS.
Binary-owned matrix: 100/100 mandatory checks passed,0failed. 74 auxiliary rows skipped/unverified by runner rules. All8approved planning leaves pass real happy-path and JSON-fidelity checks. Skips cover generated framework/local-write paths, non-ID arguments, hidden raw required fixtures and the raw200-empty venue error probe; no approved planning capability relies on a skipped live happy path.

Fixes: standardized dry-run action/would metadata without I/O; raw source venue's known200-empty contract receives a narrow error-probe opt-out. Normalized missing venue remains typed NotFound. No booking/cart/hold/payment/waitlist mutation exercised.

Focused independent-source verification:11/11cases,129assertions,11HTTPattempts. Includes venue identity, cuisine/geography/budget/date/party,cursor,exact course price/conditions,party2vs20calendarbooleans,default18:00window,canonicalhandoff and4rows in2venue×2date partialfailure. Allpass.

Efficiency:5cold/warm commands plusrefresh pass;12HTTPattempts total. Warm0HTTP; processwall19–39ms versus cold451–1632ms. Outputbytes,responsebytes,latency andpeakRSS are in planning-bench-result.json.

PrintingPress findings: generic scorecard fallback omitted required arguments (research now supplies realexamples); boilerplate docsync exclusivity claim removed; generated no-op broad sync/unusedhelpers are outside focused workflow. Root reviewed correctness/security/API contract and output; implementation/tests bySolmax.
