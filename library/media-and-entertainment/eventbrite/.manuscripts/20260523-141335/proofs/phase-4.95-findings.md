# Phase 4.95 Local Code Review — Findings

Review path: direct subagent dispatch (general-purpose) over hand-authored files only (internal/cli/eventbrite_*.go). Generated files out of scope.

## Autofixed in-place (1 round)
- MEDIUM sales-velocity tickets_per_day denominator: windowed first-order vs all-time `sold` mismatch → denominator switched to event lifetime (created→now). commit e739680.
- LOW ebRound2 negative rounding → math.Round. commit e739680.

## Left as documented intent (not fixed)
- LOW readers `continue` on rows.Scan error: deliberate NULL-safe row handling; COALESCE defaults make scan errors near-impossible; documented in eventbrite_store.go header. No silent-corruption risk for the happy path the reader tests cover.
- INFO money int64 minor-units assumption: correct per Eventbrite API contract today.

## Template-shape / out-of-scope retro candidates
- None in hand-authored code. (Generator-side: backtick-in-struct-tag bug noted in build log for end-of-run retro — out of scope here.)

## Convergence: cleared at round 1 (no in-scope findings remain).
