# Acceptance report: app-store

Gate: **PASS** after two fix loops: one before polish and one after it (see "After polish" below).

- **Level:** Full Dogfood. This is the press's live matrix, `cli-printing-press dogfood --live --level full`, run against the real App Store on 2026-10-24. No auth is needed.
- **Tests:** 202 of 202 passed in runs 2, 3 and 4. In run 1, 202 of 205 passed. The publish-time gate ran the same full matrix twice: before PR #2374 opened, and again after the fixes for its first review. Both passed 202 of 202, and the second wrote the current `phase5-acceptance.json`.
- **Not run:** 88 tests were skipped by the runner. Its notes say why: framework commands that need a fixture (profile, learnings, feedback, export, teach), dry-run-only mutating framework commands, and the error-path probe on commands that take no positional argument.
- **Coverage:** every App Store command has a passing happy-path and a passing JSON-fidelity test: app, top, reviews, ratings, compare, prices, iap, privacy, versions, events, similar, developer, featured, search-store, suggest, keyword-rank, categories list/games, and the local readers (traction, impact, changes, keyword-share, breakouts, movers, rank-history, keyword-history, watch-listing, review-digest, search).
- **Real data, checked by hand** (the runner's own output samples):
  - Spotify's listing, ratings (us, 42.7M ratings, version 9.1.94) and similar apps;
  - Candy Crush's in-app events ("Spooky Season", live until 2026-11-07) and in-app purchase prices;
  - the US games chart;
  - the "spotify" search, where Spotify ranks #1 of 177 results.
- **Throttling:** none. The harness wait budget (24 s) and the governor's pacing (pages 20 s apart, shared across the matrix) kept every request polite.

## Failures in run 1, fixed

- **keyword-rank, search-store, suggest: the error_path probe.** The runner passed `__printing_press_invalid__` as the search term and expected a non-zero exit. For these commands any text is a valid term, so the run correctly returned an empty or broad result with exit 0. **CLI fix** (commits 149cc1d and 867e9d0): the three commands declare `pp:no-error-path-probe` with a comment, the same pattern keyword-share already used.

## After polish: runs 3 and 4

Polish changed the source, which made the run-2 marker stale, so the matrix ran again before promotion.

- **Run 3** (10:02 UTC, 91 s): 202 of 202 passed. Its `reviews` sample caught Apple's most-recent reviews feed for Spotify in us returning a well-formed empty page. One paced probe confirmed that the most-helpful feed still served 50 reviews, and run 2 had read 50 recent reviews that morning. The CLI's note blamed "no written reviews in this feed, or the app is not sold there". Its own Lookup contradicted that: 42.7M ratings in us.
- **CLI fix** (commit 07d5690):
  - When a Lookup ran in an empty storefront and counts ratings there, the note now says so and suggests running it again later, or `--sort both` when only one sort was asked for.
  - With both sorts, a sort whose first page came back empty while the other sort had reviews is now named in a note. It used to be dropped silently.
  - Three tests cover this, and each fails on the old code.
- **Run 4** (10:11 UTC, after the fix): 202 of 202 passed. The `reviews` sample now reads "Apple returned no reviews for app 324684580 in us (verified empty), but Lookup counts 42729984 ratings in us: Apple's reviews feeds are sometimes empty for a while, so run it again later or try --sort both".
- **Throttling:** none in either run.
- **Privacy:** the saved results file replaces reviewer nicknames in output samples with `[reviewer]`.

## Fixes applied

1. The `pp:no-error-path-probe` annotation on the three free-text search commands (see above).
2. Honest empty-feed notes in `reviews` (see "After polish").

## Printing Press issues

None new. The error-path probe treats any positional argument as invalidatable, and the press's annotation handles that.

## Not done, by design

- **The TLS-fingerprint experiment** planned earlier: whether a Chrome TLS fingerprint lowers Apple's page throttling. It was not run, because the paced governor drew no 429 in either live matrix run, and testing the idea would mean deliberately provoking Apple.
