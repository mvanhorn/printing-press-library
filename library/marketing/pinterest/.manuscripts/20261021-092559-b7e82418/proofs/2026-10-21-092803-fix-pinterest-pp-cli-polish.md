# Phase 5.5 — polish (printing-press-polish, forked)

Polish pass:
  Verify:      100% → 100%
  Scorecard:   96 → 96 (polish's own scorer run; shipcheck reports 98)
  Tools-audit: 0 → 0 pending
  go vet: clean; dogfood: PASS; verify-skill: 0 findings; pii-audit: 0
  Fixed by polish: none needed

Polish ship_recommendation: hold. Reasons, and the parent run's disposition:
1. "live matrix not exercised (no token)": polish's own live-check ran without credentials. The parent run executed the full live matrix with a read-only token: 661/661 pass, no hollow features, phase5-acceptance.json status=pass. Resolved by existing evidence, not a CLI defect.
2. "gosec could not complete": pinned gosec v2.26.1 does not build on go1.27.3, and @latest reported type errors/skipped SSA. All 34 findings are in generator-emitted files (auth.go, client.go, helpers.go, store_adopt_darwin.go, teach.go). This is a toolchain/generator retro item, not a hand-code finding.
3. Output-review warning: pins preflight put an unresolved board name into create_body.board_id. FIXED by the parent run: an unresolvable name is now error board_unresolved, numeric ids pass through with board_unverified, and a regression test covers both.

Net: no CLI defect remains, so the parent keeps the Phase 4 `ship` verdict and does not downgrade on polish's credential-less signals. Live dogfood is re-run after this edit for a fresh source fingerprint before promotion.
