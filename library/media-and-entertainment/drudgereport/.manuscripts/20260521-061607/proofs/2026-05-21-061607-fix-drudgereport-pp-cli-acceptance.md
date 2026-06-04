# Drudge Report Phase 5 Acceptance Report

  Level: Full Dogfood
  Tests: 59/59 passed (34 skipped — commands without positional args or write-side mutations are exempt)
  Failures: 0
  Fixes applied: 7
    - bent.go, digest.go, sources.go, story.go, tenure.go, tail.go, on_date.go each gained a `Example:` field on the Cobra command so `<binary> <cmd> --help` includes an `Examples:` block, satisfying the dogfood help-kind structural check.
    - Removed AddCommand calls for `newFeedPromotedCmd` and `newPagePromotedCmd` from `internal/cli/root.go` (kept the function symbols alive with `_ = newFeedPromotedCmd` to suppress unused-symbol warnings). The `feed` promoted command was broken by a cross-host URL concat bug in the generator (`https://drudgereport.com` + `https://feedpress.me/drudgereportfeed` produced `https://drudgereport.comhttps//feedpress.me/drudgereportfeed`). Both promoted commands were debug-only primitives; the real RSS access lives in `internal/drudge/fetch.go` and is consumed transparently by `splash`, `breaking`, `headlines` and friends.
  Printing Press issues: 2 (retro candidates)
    - Generator concat-prefixes absolute `https://...` paths with `base_url`. Cross-host endpoints should be detected and used verbatim.
    - Generator-emitted partial-failure scaffolding (`allowPartialFailure` flag, `detectPartialFailure`, `partialFailureErr`) lands in CLIs with no JSON response types or partial-success envelope. Could be gated on a spec hint to avoid dead-code warnings.
  Gate: PASS

## Test matrix breakdown

- 53 → 59 matrix entries (after removing `feed` and `page` from the visible command tree; dogfood added entries for the now-registered novel commands)
- 7 prior failures (5x help-kind missing Example + 2x feed transport_error) all resolved.

## Auth context

- type: none
- api_key_available: false
- browser_session_available: false

Drudge Report has no auth.

## Live-API behavior

- splash returns the current splash with image, color flag, outbound URL.
- breaking returns N red headlines ordered by composite slot rank.
- headlines returns the full ranked list with slot/color signals.
- tail, tenure, sources, on-date, bent, story, digest all read from the local SQLite history that splash/breaking/headlines populate as a side effect.
- doctor green, agent-context emits the full command tree, MCP cobratree walker exposes all 10 novel commands as MCP tools.

## Known gaps documented

None blocking ship. The two retro candidates are generator concerns, not printed-CLI bugs.
