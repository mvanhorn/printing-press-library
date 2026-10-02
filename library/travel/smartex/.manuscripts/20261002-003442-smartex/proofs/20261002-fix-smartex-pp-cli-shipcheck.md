# smartEX shipcheck

Printing Press4.32.5 umbrella reached PASS on all seven legs: verify, narrative, dogfood, primary live workflow, apify audit, skill audit and scorecard. Verify100% (22/22); scorecard80/A. `shipcheck.json` holds the binary-owned result. API-specific fixes: EUC-JP wave dash handling, literal per-command Cobra/flag bindings, distinct reference baggage-guide/timetable-links names. Public scope uses no credentials or resident browser.

Separate full Go checks caught stale generated API-command fixture names after the reference-leaf rename; this is fixed before review/promotion, not accepted from the umbrella alone. Domain/live correctness proof is in domain-tests.log, live-e2e-fares and live-e2e. Independent MAX review and final live matrix remain required before local promotion.
