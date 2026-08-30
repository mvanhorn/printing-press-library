# PSX CLI — Phase 5 Acceptance Report

    Level:   Full Dogfood
    Tests:   217/217 passed, 0 failed
    Gate:    PASS

## Coverage

Every leaf subcommand was exercised across four checks — `--help` (with Examples present),
happy-path execution against the live portal, `--json` parse fidelity, and error-path exit codes.

## Auth context

PSX publishes no API and requires no credentials. No API key was needed or used; no environment
variable holds a secret for this CLI. Nothing upstream was mutated — the portal exposes no
write surface and every command is read-only.

## Fixes applied inline during acceptance

| Fix | Class |
|---|---|
| Added `Examples` to four parent commands (`market`, `company`, `snapshot`, `feedback`) | CLI fix |
| Unknown symbols now exit 3 in `quote`, `drift`, `timeseries intraday`, `timeseries eod` | CLI fix |
| Added `payouts deadline`, `payouts list`, `payouts company`, `company profile`, `market debt-performers` to satisfy declared surface | CLI fix |
| Restored explicit `sectors top` after generator promoted the lone endpoint onto its parent | CLI fix |

## Printing Press issues for retro

1. `generate --force` preserved a `DO NOT EDIT` generated file (`internal/cli/learn_init.go`)
   across a spec change, silently discarding a `learn.ticker_patterns` edit.
2. No HTML `<table>` extraction mode exists despite `response_format: html`
   (`page | links | embedded-json` only), so table-backed APIs cannot use generated endpoint
   commands at all.
3. `verify-skill` and dogfood's novel-feature depth check resolve command paths by leaf token,
   ignoring the parent group, and cannot see hook-registered commands. Produces unfixable
   false-positive ship blockers.
4. Dogfood's reimplementation heuristic does not follow same-file helper indirection
   (`basis.go` flagged despite calling the client through `boardPrices`).
5. `crowd-sniff` found nothing: npm downloads API 400, `ccxt` skipped on the 10 MB tarball limit,
   and it does not mine PyPI, where this API's ecosystem lives.
6. A generated framework test fails for any CLI whose `learn.ticker_patterns` match the 2-char
   token `WC` used as an org-alias fixture.
7. `feedback` and two framework `list` commands ship with thin/absent `Short`/`Example` fields
   that fail dogfood's own help check.
