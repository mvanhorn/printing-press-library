Manifest transcendence rows: 3 planned, 3 built. Phase 3 will not pass until all 3 ship.

# coinlocker-navi-pp-cli build log (run 20261009-095811-aff6d8f7)

## Built
- Priority 0: internal/locker — rate-limited client (per-host cliutil.AdaptiveLimiter at 1 req/s, cookie jar, 4 MB body cap, typed ErrNotFound / HTTPError / cliutil.RateLimitError), coinlocker-navi HTML parsers (search list, GPS nearest, detail page, Maihama map + ajax blocks), Multi Ekicube ph2 JSON parser with JA/EN merge, hours and window logic, gate/live join rules.
- Priority 1 (absorbed rows 1-6): `near <keyword>`, `near --lat/--lon` (filters --size, --ic, --walk-up-only), `locker <id>`, `vacancy <keyword>|--lat/--lon`, `vacancy --station maihama`. Area browse pages are covered by the keyword path (approved "less is more").
- Priority 2 (transcendence 1-3): `near --live` (N1), `near --open-during HH:MM-HH:MM [--strict]` (N2), `near --gate inside|outside` (N3).
- Priority 3: tests in internal/locker (model, hours, navi, ekicube, join, maihama; fixtures scrubbed of CSRF tokens), dry-run output names the planned requests, README Commands + Data Honesty sections, SKILL command reference + result-reading rules.

## Gate rule (N3) as built
Gate is copied from Multi Ekicube inside_ticket_gate/outside_ticket_gate only when exactly one bank is within 15 m (gate_source multiecube_single_bank_within_15m). No text guessing. --gate drops unknown-gate rows and counts them in meta.excluded.gate_unknown.

## Honesty rules as built
meta.fetched_at + "no per-record update date" note; Maihama source_as_of; 情報なし -> null; "0 reservable does not mean full" caveat on every Ekicube bank and in vacancy notes; ¥500 reservation fee split from usage fee; live proximity match labelled "nearest_bank_within_30m_may_be_neighbouring_bank"; on-demand only, no sync.

## Deferred / cut (approved at Phase 8)
- Hankyu Umeda live page (broken template JSON) — cut.
- JR East Tokyo area maps — retired at source.

## Skipped body fields
None (all requests are GET or small form POSTs).

## Generator limitations found
- Generated go.mod failed govulncheck (Go 1.27.1 stdlib, x/net v0.56.0). Fixed by x/net v0.60.0 + toolchain go1.27.2; generated with --validate=false then re-ran gates.
- Dogfood WARN: dead root flag maxAge and 8 dead generated helpers (endpoint/query helpers unused by an internal-yaml HTML spec). Generator emission; left in place.
- README/SKILL template text assumed a SQLite store and used `source search --q example-value` examples; rewritten.

## Phase 3 completion gate
- near/locker/vacancy --help Usage lines resolve as leaf commands (exit 0).
- dogfood novel_features_check: planned 3, found 3, missing none.
- validate-narrative --strict --full-examples: 7/7 OK. verify-skill: all checks passed.
Phase 18 blocked at depth question; resume with phase-receipt enter --resume
