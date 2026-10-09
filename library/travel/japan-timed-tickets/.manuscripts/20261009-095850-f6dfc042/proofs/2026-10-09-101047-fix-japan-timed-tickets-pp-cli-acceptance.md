# Acceptance Report: japan-timed-tickets

Level: Full Dogfood (chosen by the orchestrator; source pacing kept; no Queue-it; no Lawson stock pages)
Tests: 37/37 passed (runner matrix; 38 skipped by the runner: profile/feedback/which need free-text positionals, mutating or no-dry-run rows)
Runner report: 2026-10-09-101047-dogfood-results.json; marker: phase5-acceptance.json (status pass, level full)

Supplementary live checks (run by hand against the built binary, 2026-10-09 JST):
- Error paths, all with the documented exit code: unknown sight in availability (2), invalid date (2), past date (2), --slots over 7 dates (2), unknown sight in sights (3), invalid --tz (2), bad --party key (2), 121-day range (2). Bare `onsale` prints help (0).
- onsale, all 7 sights, 2026-11-09..10: 0 warnings, 0 fetch failures, 20 requests. teamLab rows exact; SHIBUYA SKY opens-at 00:00 JST 14 days ahead; Ghibli closed (the museum calendar shows a 4-16 November closure, checked against the official page).
- onsale --sights sky --date 2026-11-24 --tz America/Los_Angeles: sale 2026-11-10 00:00 JST (2026-11-09 07:00 PST), computed sunset 16:29:52 JST, target slots from 15:40 at the web price of 3,400 JPY.
- onsale --sights ghibli --date 2026-12-05: opens-at 2026-11-10 10:00 JST, exact, official rule confirmed on the museum page.
- availability teamlab-planets teamlab-borderless --date today --slots --party adults=2: slots that already started are left out; slots_fit_party counts the fitting slots.
- availability teamlab-kyoto --slots --party adults=2: 30-minute slots with stock and adult price.
- onsale --ics (SKY + Planets, 3 dates): 3 VEVENTs at 15:00Z (00:00 JST); Planets already on sale, so no event.
- doctor: all 7 sights OK with the real parsers; 2 INFO boundaries (SHIBUYA SKY Queue-it, Lawson login).

Failures: none
Fixes applied in this phase: 0 (Phase 4.95 fixes were already applied and tested before this run)
Printing Press issues for retro:
- The runner skipped error_path for onsale/availability ("no positional argument") although availability takes sight positionals; error paths were covered by hand.
- Generated `isCobraUsageError` and exit mapping are fine; note that the dead `--max-age` flag and 8 helpers remain (dogfood WARN).

Gate: PASS
