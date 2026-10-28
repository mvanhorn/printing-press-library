# normattiva-pp-cli shipcheck (2026-10-28)

## Loop 1
- verify PASS 98% (50/51, 0 critical), validate-narrative PASS, dogfood PASS, workflow-verify workflow-pass, apify-audit PASS, scorecard 95/100 A
- verify-skill FAIL: README troubleshoot line put `cerca` and `testo --codice/--data-gu` on one line, so the scanner attributed the flags to `cerca`.

## Fix
- research.json troubleshoot rewritten to a single `testo --codice 090G0294 --data-gu 1990-08-18` command.

## Loop 2
- All 7 legs PASS. Verify 98% (mock mode; live commands score exec FAIL against spec-derived mock bodies, expected). Scorecard 95/100 Grade A (Cache Freshness 5/10, Dead Code 4/5 from two generated helpers).
- Live sample probe: 3/3 pass.

## Behavioral checks run in Phase 3 (live)
- verifica: ok / abrogato / non_trovato classified correctly; exit 1 on invalid; --al 2010 flags modificato_dopo for art. 13 d.lgs. 196/2003.
- diff: art. 13 d.lgs. 196/2003 2010→2015 non-identical (+65/-1 words); art. 2043 c.c. identical (negative case).
- novita: empty watchlist → exit 0 with hint; positive case (D.L. 153/2026, simulated stale snapshot) reported act + changed article; second run quiet.
- fascicolo: 2 norms pinned to 2021-05-10, markdown with front matter and permalinks.

## Verdict: ship
