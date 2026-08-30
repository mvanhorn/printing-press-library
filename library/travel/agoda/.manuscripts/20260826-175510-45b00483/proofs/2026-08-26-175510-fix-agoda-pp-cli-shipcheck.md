# Agoda CLI shipcheck

## Final result
```
LEG                 RESULT  EXIT
verify              PASS    0
validate-narrative  PASS    0
dogfood             PASS    0
workflow-verify     PASS    0
apify-audit         PASS    0
verify-skill        PASS    0
scorecard           PASS    0

Verdict: PASS (7/7 legs passed)
```

- verify pass rate: **100% (36/36, 0 critical)**
- scorecard: **85/100, Grade A**, 0 unverified dimensions
- dogfood novel_features_check: **planned 8, found 8, 0 stubs**
- Sample Output Probe: 7/8 (the single miss is `vip delta` correctly exiting 4
  when no session cookie is present, which is the honest result, not a defect)

## Before / after
| Metric | First run | Final |
|---|---|---|
| verify pass rate | 100% | 100% |
| scorecard total | 81 | 85 |
| verify-skill | FAIL (2 findings) | PASS |
| Live API Verification | N/A (HOLD) | 10/10 |
| Insight | 7/10 | 10/10 |
| Sample probe | 6/8 | 7/8 |

## Blockers found and fixed
1. **verify-skill: `destinations resolve Tokyo` did not exist.** The generator
   promotes a single-endpoint resource to the resource name itself, so the real
   form is `destinations --search-text Tokyo`. The narrative in `research.json`
   and the spec's `example` both claimed the sub-command form, which would have
   shipped a copy-paste-broken first command in the README Quick Start.
   Fixed at source in `research.json` + spec, then regenerated.
2. **verify-skill: `--city-id` attributed to `destinations`.** A troubleshooting
   line named `--city-id` in the same sentence as the `destinations` command,
   which does not declare that flag. Reworded so the flag sits next to
   `hotels search`, which does.
3. **`compare` probe failed with exit 2.** The verifier synthesizes an
   invocation without positionals, which tripped the two-id guard. Added
   `pp:happy-args` so the probe supplies real ids and a destination.
4. **`vip delta` used the wrong typed exit code.** A missing session is an
   authentication state, so it now exits 4 (auth required) rather than 3
   (not found), with the annotation updated to match.
5. **Duplicate `search` command.** Adding `search` to `novel_features` made the
   generator emit a TODO scaffold that competed with the hand-written
   implementation. The implementation was moved into the generator's expected
   file and constructor (`internal/cli/search.go`, `newNovelSearchCmd`) so
   regeneration preserves it as an implemented body.
6. **Description drift across surfaces.** `root.Short` came from the spec's
   `cli_description` while every other surface derived from
   `narrative.headline`. Removed `cli_description` so a single source feeds all
   surfaces, and replaced the headline ("The only Agoda tool that...") with a
   capability statement rather than marketing phrasing.
7. **Dead code of my own.** Removed a `var _ = cliutil.CleanText` import guard
   left behind in `prices_cheapest.go`.

## Remaining dogfood notes (accepted, with reasons)
- **`3 dead helper functions found`** — `collectionItemsForOutput`,
  `paginatedGetWithResponsePath`, `successfulNoop`. All three are
  generator-emitted framework helpers, not hand-written code. Per the
  template-shape escape hatch these are filed as retro candidates rather than
  patched: editing them in the printed CLI would hide the machine-level issue
  and they would be re-emitted on the next regeneration. This holds Dead Code at
  2/5.
- **`sync uses generic Upsert only`** — this CLI has no `sync` command. Agoda
  exposes no bulk catalog endpoint, so there is nothing to bulk-sync; the local
  corpus is instead accumulated from live searches. The finding is a heuristic
  that does not apply to this API's shape.
- **`2/8 novel features look reimplemented`** (`prices cheapest`, `vip delta`) —
  a false positive. Both call the hand-written sibling client
  (`internal/agoda`), not the generated `internal/client`, because Agoda's
  surface is a GraphQL operation with a 30KB query document that the generated
  client cannot express. The heuristic looks for the generated client
  specifically.
- **Cache Freshness 3/10** — deliberate. `cache.enabled` is false because there
  is no upstream bulk-refresh path to make a pre-read refresh meaningful, and
  the guidance is explicit that the helpers should not be emitted as dead code
  just to raise a score.

## Ship threshold check
- shipcheck exits 0, all legs PASS ..................... met
- verify verdict PASS with 0 critical ................. met
- dogfood clean (no spec/binary/example failures) ...... met
- workflow-verify not `workflow-fail` ................. met
- verify-skill exit 0 ................................. met
- scorecard >= 65 ..................................... met (85)
- no flagship feature returns wrong or empty output .... met (verified live)

## Final verdict: **ship**
No known functional bugs in shipping-scope features. Every novel feature was
exercised against the live API with content assertions, not just exit codes.
