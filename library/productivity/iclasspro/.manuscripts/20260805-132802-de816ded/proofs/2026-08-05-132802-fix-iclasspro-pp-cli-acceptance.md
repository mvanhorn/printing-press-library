# iClassPro CLI — Phase 5 Acceptance Report

```
Acceptance Report: iclasspro
  Level: Full Dogfood
  Tests: 107/107 passed
  Failures: none
  Fixes applied: 2 (both before the passing run; see below)
  Printing Press issues: 4 (for retro)
  Gate: PASS
```

Marker: `proofs/phase5-acceptance.json` — `status: pass`, `level: full`, `matrix_size: 107`, `tests_passed: 107`, `auth_context.type: none`.

## What was exercised

The binary-owned live matrix walked every leaf subcommand in the shipped tree and ran, where applicable: `--help`, happy path against the real portal, JSON parse validation, output-mode fidelity, and an error path. No credentials were involved — the iClassPro Open API answers plain HTTP — so the entire surface was testable, including every generated endpoint command and all eight novel commands.

Live tenants used: `scottsdalegymnastics` (classes, camps, parties, 124 entities), `scaq` (27 classes), and `nadoclub` (the user's own, sign-in-gated).

## Failures found and fixed

The first full run was **102/107**. All five failures were the same defect class, and it was a real one.

### Local-only commands answered for accounts they knew nothing about

`calendar`, `drift`, `fill-rate`, `lint`, and `opens-soon` each accepted an unknown account and returned an empty result with **exit 0**.

The worst case was `lint`, which printed `Clean: 0 entities checked, no findings.` for an account that had never been synced. That is false reassurance a user could act on — someone auditing a client's catalog before a website launch would read "clean" and ship. `calendar` was similar: it emitted a syntactically valid but empty `.ics` file rather than saying it had nothing to export.

This is precisely the failure mode this CLI was built to eliminate. The `tenant` command exists because iClassPro itself conflates "empty" with "blocked"; shipping the same conflation in our own commands would be indefensible.

**Fix:** `icpNoLocalData` — when a local-only command has no records for the requested account, it writes the structured payload (so an agent still gets parseable output) and returns a typed **exit 3** with `no local data for account "<x>"; run 'iclasspro-pp-cli sync <x>' first`.

The distinction preserved: an account **with** local data whose answer is genuinely empty still exits 0. `lint scottsdalegymnastics` reporting no findings across 124 entities means the catalog is clean. `lint some-unsynced-gym` now says it cannot answer.

**Verified both directions:** all five commands exit 3 with parseable JSON for an unknown account, and all five exit 0 for a synced one.

### Second run: 107/107.

## Behavioral spot-checks beyond the matrix

Flagship features were sampled against real data rather than trusted on exit code alone:

| Command | Result |
|---|---|
| `tenant nadoclub` | Correctly classifies classes, camps, and appointments as `sign-in-required` while locations, booking-menu, class-programs, and levels report `open` |
| `classes list … --q ninja --openings 1` | Returns Parkour & Ninja classes with openings — server-side search confirmed working |
| `calendar` | 316 VEVENTs from 124 entities, 0 skipped; spot-checked `DTSTART`/`DTEND`/`URL` against the source records |
| `opens-soon --days 60` | 12 windows, correctly ordered by proximity, matching `registrationEndDate` on the live records |
| `drift` after two syncs | 124 entities compared, 0 changes (correct — nothing changed in the interval) |
| `fill-rate --programs 589` | 13 entities with history, 13 trends — matches the 13 classes upstream reports for that program |
| `compare` across two accounts | 3 buckets, per-account aggregates consistent with each account's own listing |
| `watch --class 8357` | Watches exactly 1 entity, records the observation, reports no change on a repeat check |

## Deliberate deviation from skill guidance

The skill's missing-mirror pattern prescribes `return nil` (exit 0) with an empty payload when the local store has no data. This CLI exits 3 instead, for the reason above: for `lint` and `calendar` specifically, a successful-looking empty answer is materially misleading. The structured-payload half of the guidance is preserved — machine consumers always get valid JSON on stdout regardless of exit code.

## Printing Press issues for retro

1. **No syncable resources when every endpoint carries a positional path parameter.** A multi-tenant API keyed by a path slug gets no `sync`/`search`/`sql`/`stale`/`tail` and no `internal/syncer`, though `internal/store` is emitted and the resources are perfectly syncable per tenant. The entire local-store stack had to be hand-authored.
2. **`browser-sniff` CAPTCHA false positive.** The org-settings field `recaptchaPublic` was read as an observed challenge, yielding `reachability.mode: browser_required` at 0.9 confidence — a HOLD verdict under Phase 1.9. `probe-reachability` returned `standard_http` at 0.95 and all 13 captured entries were HTTP 200 with no challenge sentinel.
3. **`browser-sniff` path templating on multi-tenant APIs.** Static segments became identifiers (`bookings/{booking_id}` for `{locationId}`, `levels/active/{active_id}`, `parties/create/{create_id}`) and the tenant slug was baked into every path instead of being parameterized, making the sniffed spec unusable as a generation input.
4. **`validate-narrative --full-examples` passes examples that cannot work.** Three shipped examples were wrong (a required positional omitted, an unsupported `--resources` value, and an id belonging to a different tenant). All passed because `--dry-run` short-circuits before argument validation. Executing the documented strings for real is what caught them.

## Gate

**PASS.** 107/107, no known functional defects in shipping-scope features.
