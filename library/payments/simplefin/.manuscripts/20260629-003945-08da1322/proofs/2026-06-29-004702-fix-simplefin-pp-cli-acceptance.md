# SimpleFIN CLI Acceptance Report

Level: Full Dogfood (live, public demo Access URL)
Tests: 69/69 passed, 0 failed, 36 skipped (inapplicable error_path/json_fidelity)
Gate: PASS

Auth: SimpleFIN access-URL (no API key); credential resolved from SIMPLEFIN_ACCESS_URL.
Real-world data exercised: 3 demo accounts (savings w/ AAPL holding, checking, empty), 342 transactions, 1 connection.

Fixes applied this phase: 3 (accounts date-400, categorize stdout pollution, export --json). All re-verified pass.
Printing Press issues for retro: 1 (cliutil credential-test base64 substring assertion).

No PII in this report: demo data only; no real account names, emails, or balances beyond synthetic demo values.
