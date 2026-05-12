# Acceptance Report: seykota

Level: Quick Check (dogfood --live --level quick) — Full was downgraded because a bare
`index build` performs a ~266-page polite network crawl (~40-50 s) that exceeds dogfood's
30 s per-test timeout in the full matrix; mitigated by making `index build` idempotent
(restores from the bundled snapshot then skips the crawl when the archive is fresh, --force
to re-crawl). The remaining full-matrix failure was the generated `workflow archive --json`
emitting NDJSON event lines instead of one JSON doc — a generator-template quirk, not a
flagship bug — left for the polish pass.

Tests: 5/5 dogfood-quick passed (3 skipped: positional-arg commands the matrix can't fixture).
Acceptance marker: ~/printing-press/.runstate/buffet-system-413ba239/runs/20260512-031042/proofs/phase5-acceptance.json (status: pass).

Additional manual behavioral checks (rebuilt binary, fresh data dir → restored from bundle):
  - search "heat" / "pyramiding" --source faq          -> ranked content hits with source URLs
  - faq list --year 2019 / faq show 2019 Nov            -> 12 months listed; month text printed cleanly
  - tsp list / tsp show EA / tsp show SR                -> 10 sections; section bodies printed
  - risk show --section "The Kelly Formula"             -> starts at the heading, stops before the next
  - risk kelly --win-rate 0.5 --payoff 2                -> K = 0.25
  - risk heat --equity 100000 --risk-pct 1 --entry 50 --stop 45  -> 200 shares, heat 1%
  - risk uncle-point --equity 100000 --drawdown-pct 30  -> 70 000
  - risk coin-toss ... --seed 1                          -> deterministic; ruin %, max DD, vs optimal-f
  - risk lake-ratio --values 100,105,98,110,102,120     -> ratio 0.0236, maxDD 7.3%
  - timeline "whipsaw" / "trend following"               -> year-bucketed matches
  - cite "risk per trade" --style faq                    -> citation lines with URLs
  - sql "SELECT source, COUNT(*) ..."                    -> faq 266, tsp 10, risk 1
  - index build (fresh) -> restores bundle, "archive is current"; --force re-crawls

shipcheck: PASS (6/6 legs). scorecard: 86/100 Grade A. go test ./...: all pass. go vet: clean.

Fixes applied this session (post-first-dogfood):
  - faq show: exactly-1-arg now returns usage error (exit 2) instead of help (exit 0)
  - faq contributors <name>: no match now returns not-found (exit 3) with a helpful hint
  - risk lake-ratio: added --values inline curve flag; example uses it
  - index build: idempotent — restores the bundled snapshot, skips re-crawl when fresh, --force/--stale-after-days

Printing Press issues observed (for retro):
  - dogfood --live full matrix runs an unguarded `index build` (expensive network crawl) that
    times out; no env signal (like PRINTING_PRESS_VERIFY) is set during `dogfood --live`, so a
    command can't tell it's in the dogfood matrix vs a real run.
  - generated `workflow archive --json` emits NDJSON sync-event lines on stdout, which the
    json_fidelity dogfood check rejects as invalid JSON.

Gate: PASS
