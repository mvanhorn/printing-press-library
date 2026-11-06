<!-- slop-gate: off -->
# circle-pp-cli shipcheck

Round 1: FAIL 2/7 (dogfood: 30% example coverage; verify-skill: 8 positional-arg mismatches). Scorecard 94/100.
Fixes: Press static dogfood now excludes intentionally omitted examples (Press commit 1f836054); Fix task G declared positionals in Use for course push/pull and show-tell recap; goldmark bumped to v1.7.17 (GO-2026-5320) after generate govulncheck caught it; operationId overlay renamed export/refund/cancel/resume/payout/lesson reorder/image post commands.

Round 2:
```
Shipcheck Summary
=================
  LEG               RESULT  EXIT      ELAPSED
  verify            PASS    0         29.843s
  validate-narrative  PASS    0         697ms
  dogfood           PASS    0         5.072s
  workflow-verify   PASS    0         23ms
  apify-audit       PASS    0         70ms
  verify-skill      PASS    0         19.714s
  scorecard         PASS    0         7.004s

Verdict: PASS (7/7 legs passed)
  Total: 94/100 - Grade A
```

Sample probe: founding --go-no-go 10 exits 2 (NO-GO, 0 payers): designed typed exit, declared in pp:typed-exit-codes.

Verdict: ship
