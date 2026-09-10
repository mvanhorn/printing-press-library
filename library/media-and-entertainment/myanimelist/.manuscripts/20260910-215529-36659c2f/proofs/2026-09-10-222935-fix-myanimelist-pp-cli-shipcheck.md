# MyAnimeList CLI — Phase 4 Shipcheck

Run: `20260910-215529-36659c2f` · binary `myanimelist-pp-cli` · verdict: **ship**

## Umbrella result

```
LEG                  RESULT  EXIT
verify               PASS    0
validate-narrative   PASS    0
dogfood              PASS    0
workflow-verify      PASS    0
apify-audit          PASS    0
verify-skill         PASS    0
scorecard            PASS    0
Verdict: PASS (7/7 legs passed)
```

Invocation included live verification against the real site
(`--env-var PRINTING_PRESS_LIVE`), which is what moved the scorecard's
`live_api_verification` dimension from unscored to 10/10.

## Scores

| Metric | Value |
|---|---|
| verify pass rate | 99% (87/88), verdict PASS, 0 critical after fixes |
| scorecard total | **96/100 — Grade A** |
| live API verification | 10/10 |
| live sample probe | 7/7 novel features produced plausible output (100%) |
| dogfood dead flags | 0 |
| dogfood dead functions | 0 |
| dogfood novel features | 7/7 survived |
| MCP surface | 48 tools, readiness `full` |

## Blockers found and fixed

1. **`verify-skill` FAIL — `instant search` did not exist.** The spec's single-endpoint
   `instant` resource is *promoted* to a bare `instant` command with `--query` /
   `--entity-type` flags, but the spec example, `README.md` (3 sites), and `SKILL.md`
   (2 sites) still described `instant search --query …`. Fixed at the source (spec +
   `research.json`) and in the rendered docs; `verify-skill` now reports
   "All checks passed".
2. **`scorecard` HOLD — `live_api_verification` unscored.** The dimension is only
   scored when `verify` runs against the real API rather than the spec-derived mock
   server. Re-ran shipcheck with live verification enabled; the dimension scored
   10/10.
3. **Build break from a doc fix.** Renaming a cache-command key to the promoted
   `instant` path produced a duplicate map key in `internal/cli/auto_refresh.go`;
   de-duplicated and rebuilt.
4. **`resource-path:export` static probe (non-blocking).** The probe expects the
   framework's generated `export` to call `resourceReadPath(...)`. This CLI's
   `export` was re-implemented to emit the local watch library as MyAnimeList
   import XML (the anonymous-scope bridge), so it does not use that resolver. The
   `export` command itself scores 3/3 on help/dry-run/exec, and the verify leg
   passes; recorded here rather than faked by inserting an unused resolver call.

## Before/after

| | Before first shipcheck | After fixes |
|---|---|---|
| verify | 99% WARN (1 critical static probe) | 99% PASS |
| verify-skill | 7 errors | 0 errors |
| scorecard | 95/100 Grade A, HOLD (live unscored) | **96/100 Grade A, no hold** |
| shipcheck | FAIL (1/7 legs) | **PASS (7/7 legs)** |

## Behavioral correctness spot-check (live, real MyAnimeList)

- `anime divisive 5114` → Fullmetal Alchemist: Brotherhood, **broadly loved, index 7.9**
  (love 76%, hate 2.4%) — the semantics were corrected during the build after an
  early version mislabelled a universally-loved title as "bitterly split".
- `anime drop-risk 21` → One Piece, **elevated**, dropped 8.1%, on-hold 12%
  (`completed: 26` is genuinely what MAL reports for an airing series).
- `anime consistency 52991` → Frieren, slope **+0.4 (improves)**, worst episode 1.
- `adaptation 52991` → links the anime to its manga entry and reports the
  "no chapter count published" limitation honestly instead of inventing a number.
- `franchise gap` → found Frieren 2nd Season as an unstarted **Sequel**.
- `week` → converted `Fridays at 23:00 (JST)` to **Friday 19:30 IST**.
- `export --format mal-xml` → emitted MAL's own import schema.

## Final ship recommendation

**ship.** All seven ship-threshold conditions hold: shipcheck exits 0 with every leg
PASS; verify is PASS with zero critical failures; dogfood wiring checks pass with no
dead flags or functions; workflow-verify reports `workflow-pass`; verify-skill exits 0;
and the scorecard is 96/100 with every novel feature producing plausible live output.
