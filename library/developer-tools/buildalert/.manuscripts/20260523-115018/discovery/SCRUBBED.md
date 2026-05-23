# PII-Scrubbed Manuscripts

The following files were removed from this manuscripts directory before publishing
to the public library because they contained real homeowner addresses, planning
applicant names, and the test account's email from live BuildAlert API responses:

- `probe-results-1.json` through `probe-results-4.json` — raw API probe captures
- `sample-lead-full.json` — full sample of one /dapi/leads/live-leads lead
- `browser-sniff-capture.json` — enriched capture file aggregating all probes

`cli-printing-press publish package`'s mandatory PII scan flagged 60+ findings in
these files (postal addresses, emails, applicant names). Rather than redact each
match in nested JSON, the files were removed wholesale.

The browser-sniff *report* (`browser-sniff-report.md`) was kept — it documents
the API surface, endpoint inventory, and replayability findings without quoting
real user data. `traffic-analysis.json` was kept — it carries reachability
metadata only. `page-endpoints.json` was kept — URLs only.

The CLI itself (in the parent `library/buildalert/`) is unchanged and continues
to work against the live BuildAlert API.
