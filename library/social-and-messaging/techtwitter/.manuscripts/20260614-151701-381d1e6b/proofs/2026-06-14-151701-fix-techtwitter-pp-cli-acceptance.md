# Tech Twitter CLI — Live Dogfood Acceptance Report

- **Level:** Full Dogfood
- **Auth context:** none (public, read-only API — fully testable, no key, no side effects)
- **Tests:** 101/101 passed (matrix grew from 98 after adding the `trending` command)
- **Gate:** PASS

## Fixes applied during dogfood (3, all fix-now)
1. `tweets get <non-uuid>` — added UUID validation (was leaking ~100KB homepage HTML at exit 0; now exit 2 usage error). Tagged: **CLI fix** (consequence of the site's tweet-detail allowlist) + retro candidate (generator could reject path params that fail a known id pattern).
2. `time-travel <invalid-date>` — added date validation (today/yesterday/latest/YYYY-MM-DD). **CLI fix.**
3. `tweets monthly` — realistic example + `pp:happy-args` (`--year=2026 --month=6`); dogfood was synthesizing month 42 which the API correctly 400s. **CLI fix** + retro candidate (generator could synthesize in-range values for month-like int params, or read spec param bounds).

## Printing Press issues for retro
- Generated endpoint commands with `{id}` path params that fail a documented id pattern
  silently return redirect-target HTML as a fake 200 success. A path-param shape guard
  (or treating 307→HTML as an error) would catch this class generically.
- Dogfood synthesizes `42` for required int params; for bounded params (month 1-12) this
  guarantees a happy-path failure. Reading spec `default`/bounds or a per-type sane default
  would help.

## Manual spot-checks (all correct)
- `since 24h` → 20 curated tweets within window (offline).
- `momentum`/`narrative` → baseline on first snapshot; movement + 2 supporting tweets each
  when a prior snapshot exists (verified with injected prior).
- `author-rank --window 7d` → engagement leaderboard (paulg #1), best tweet per author.
- `digest --window 24h --agent` → top tweets + articles + authors, cited.
- `evidence read-list --agent --select evidence.title,evidence.canonicalUrl` → exactly those keys.
- `time-travel 2026-06-07` (live) and `--data-source local` (offline) both return curated tweets.
- `trending` live + offline-ranked.
