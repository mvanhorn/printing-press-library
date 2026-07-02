Acceptance Report: thepointsguy
  Level: Full Dogfood (live, thepointsguy.com, no auth)
  Tests: 63/63 passed (0 failed; 46 probes legitimately skipped)
  Gate: PASS

  Fixes applied during Phase 5:
    - suggest: added pp:happy-args=amex and pp:no-error-path-probe=true
      (any string is a valid suggestion query; unknown terms yield an empty
      but successful result, so there is no error path to probe).

  Behavioral spot-checks (live):
    - valuations: 34 programs, correctly typed (airline/hotel/transferable), July 2026.
    - worth / redeem-check / portfolio: correct points math over live valuations.
    - cards get/list/best/compare: structured card terms; fuzzy + partial failure handling.
    - search (+ --select), suggest, latest, since, read, browse, glossary: all return
      correct structured output; typed exit codes verified (2 usage / 3 not-found / 0 ok).

  Printing Press issues (for retro):
    - 3 unused generated helpers in helpers.go (formatCLIParamValue,
      responsePayloadParentAtPath, writeNoop) — template-shape dead code.
    - Static tools-manifest.json lists only spec endpoints (2) while the runtime
      cobratree mirror correctly exposes all 23 commands.
