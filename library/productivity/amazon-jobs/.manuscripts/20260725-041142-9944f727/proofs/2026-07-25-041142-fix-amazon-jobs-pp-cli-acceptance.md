# Acceptance Report: amazon-jobs

Level: Full Dogfood (live, no auth)
Tests: 81/81 passed
Gate: PASS

## Matrix
Binary-owned live dogfood matrix (cli-printing-press dogfood --live --level full) across every
leaf subcommand: help, happy-path, JSON-fidelity, and error-path. All 81 checks passed against
the real amazon.jobs API.

## Failures fixed inline (2, both CLI fixes)
- get / skills error_path probes expected a non-zero exit for an obviously-invalid arg, but both
  correctly return exit 0: amazon.jobs returns HTTP 200 + empty for any unmatched query, and
  neither command can distinguish bad input from a valid empty result (get accepts id-or-keyword;
  skills' zero-match is a legitimate "no reqs demand this"). Resolution per skill guidance:
  annotate both with pp:no-error-path-probe (not a fake error heuristic). Matrix -> 81, all pass.

## Printing Press issues for retro: 0 blocking
- (informational, for retro) v4.29 emits no framework sync/search/sql commands and the promoted
  endpoint's paginated read path ignores response_path; the whole data layer had to be hand-written.
  Worth confirming whether this is intended for single-wrapper-endpoint specs.

## PII
None. Amazon job listings are public; no org/user/email data handled. Acceptance report carries
no live-response PII.
