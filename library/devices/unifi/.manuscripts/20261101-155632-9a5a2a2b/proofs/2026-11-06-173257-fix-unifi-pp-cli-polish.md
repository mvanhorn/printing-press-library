# Polish: unifi-pp-cli

Scorecard 97 to 97. Verify 99.47% to 99.47%. Tools-audit 0 pending. Gosec hand-owned 5 to 0.
Ship recommendation: ship, conditional on a live re-run, which passed afterwards (367/367).

Fixes: 5 narrow #nosec G304 comments; `change` Short text; `change apply` help in which.go; docs corrected to one voucher create and delete on one test console; gofmt on which.go.
No runtime logic changed. Console pin, confirm gates, secret sealing, voucher names and UNIFI_READ_ONLY untouched.
Skipped: verify `raw` mock probe (raw's path check correctly refuses it); 61 gosec findings in generated files; output-review sub-skill (its live check could not be guarded), so output was reviewed by hand against live data in Phase 18 instead.
