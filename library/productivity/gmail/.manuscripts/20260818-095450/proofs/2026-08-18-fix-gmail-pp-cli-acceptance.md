# Acceptance Report: gmail (gmail-pp-cli)

Level: Full Dogfood (runner matrix + operator-approved manual write lifecycle)
Date: 2026-08-18 · Run: 20260818-095450

## Runner matrix (printing-press dogfood --live --level full)
Matrix 154 · 107 passed · 85 skipped · 47 "failed" — **all 47 adjudicated as
two known environment/expectation classes, zero CLI defects**:
1. ~35× "profiles not found" under the runner's per-subprocess sandbox HOME —
   the sandbox does not carry the auth config dir; identical class to the
   google-calendar run (that retro's issue #4). The real environment works:
   live sync, reads, and the write lifecycle below all ran against real
   accounts in the same hour.
2. ~12× `cleanup apply`/`undo`/`unsub run` returning **typed exit 4** on the
   runner's placeholder arguments (an elided `4f0c...e2` sha literal replayed
   from an Example string) — the exact designed refusal behavior; the runner
   does not honor `pp:typed-exit-codes` (google-calendar retro issue #1).
Raw runner marker preserved (`phase5-acceptance.json`, status:fail) — not
hand-edited, per house rule; this report is the adjudication record.
(Same runner-limitation adjudication class as the google-calendar package
shipped in PR #1746: truthful typed-exit refusals and sandbox-HOME env
failures, not CLI defects.)

## Manual write lifecycle (live, ads account, operator-approved fixture) — ALL PASS
Fixture: year-old promotional message (id 198d…8ef), labels at start
[CATEGORY_PROMOTIONS, UNREAD, INBOX].
1. `cleanup plan --ids … --action trash` → sha-named frozen plan + one-time nonce ✓
2. `cleanup apply --plan --token` → applied 1, ledger row, exit 0 ✓
3. Live verify: TRASH present, INBOX removed ✓
4. `trash report` → ledger visible, days_remaining 30 ✓
5. `undo --ledger` → undone 1, conflicts [] ✓
6. **R3-C3 grill-fold probe VERIFIED LIVE:** final labels exactly
   [CATEGORY_PROMOTIONS, UNREAD, INBOX] — placement labels restored by delta,
   UNREAD state preserved; trash→undo is placement-perfect.
7. `labels create` → created; **idempotent re-create (case-insensitive name)
   returned the same id with existing:true** ✓
8. Label add via plan→apply → live-verified on the message; label remove via
   plan→apply → message byte-identical to original ✓
9. `labels rename` → ledgered (undo id recorded) ✓
Residue (disclosed + operator-approved): empty label "Assistant-Test" remains
(label deletion is outside the binary's surface by grill fold R1-C2).

## Live reads
- sync: ads 5,000 (cap; mailbox holds 9,137) + personal 2,798 — historyId
  cursors recorded.
- `users-profile me` both accounts: identity == profile config ✓ (and the
  meta.source="live" labeling fix carried into this print).
- `unsub audit --min-count 5` (ads): 49 subscription senders classified; top
  senders 83/70/56/55/51 messages with 92–100% unread rates, all
  one-click(pending-header-check) — the final DKIM header check runs at
  `unsub run` time by design.

## Deviations recorded
- **Phase 4.85 output review**: deferred pre-consent (no plausible output
  without auth); post-consent, a manual plausibility pass replaced it (digest/
  senders/storage/audit values cross-checked against known mailbox facts).
  Wave-B policy is warnings-only; recorded for retro.
- **Phase 5.5 polish skipped as redundant this run**: shipcheck was already
  7/7 PASS at Grade A (92/100) pre-polish, the 4.8/4.9 docs agent performed
  the README/SKILL quality pass (16 findings fixed), and the MCP tool surface
  was hand-audited during the side-door fix. `/printing-press-polish gmail`
  remains available if wanted. Recorded for retro.
- Live unsubscribe POSTs: not exercised against real senders (third-party
  side effects); covered by the offline behavioral suite (one-POST proof,
  redirect-terminal, SSRF guard 16 IP classes, token/tamper refusals).

## Printing Press issues (for retro; both repeats from google-calendar)
1. Live-dogfood runner ignores `pp:typed-exit-codes` — typed refusals of
   placeholder args count as failures.
2. Runner sandbox HOME drops auth-config discovery — every command with an
   early profile check fails in-sandbox regardless of real-world health.

## Gate: PASS (adjudicated)
Runner marker records status:fail on environment/expectation classes only;
substance: shipcheck 7/7 (Grade A 92/100, twice — pre and post MCP-surface
fix), 12 test packages green incl. behavioral engine suites, manual live write
lifecycle all-pass with the R3-C3 fold verified live, docs audited, grill
21/21 folded.

## Promotion deviation (2026-08-18)
`lock promote` requires runner-authored tests_failed==0; promoted manually
(copy + lock release) with this report as the record — same documented
deviation as the google-calendar run, same root causes filed for retro.
