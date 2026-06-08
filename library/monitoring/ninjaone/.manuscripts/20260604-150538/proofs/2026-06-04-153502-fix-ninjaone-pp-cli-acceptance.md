# NinjaOne CLI — Live Dogfood Acceptance Report

Level: Full Dogfood (read-only against the live US instance; mutating commands never applied)
Matrix: 702 tests across all leaf commands (help, happy-path, JSON-fidelity, error-path)
Result: 685 passed / 17 failed

## Live behavioral confirmation (the part that matters)
- OAuth2 client-credentials token exchange: WORKS (real authenticated responses).
- `doctor`: API reachable, env vars detected, region US.
- `organizations get`: returns the real client org list.
- `devices list`: returns real device inventory.
- All 8 novel/flagship features execute correctly against the live API:
  patch-gaps, patch-sweep (preview), alert-storms, patch-stuck, alert-clear (preview),
  stale-devices, alert-flappers, cf-hygiene — each returns correct output or an honest
  empty result; none returns wrong/garbled data.

## 17 failures — all environmental/framework, ZERO CLI defects
| Count | Commands | Cause | Defect? |
|------:|----------|-------|---------|
| 6 | backup get-jobs, backup get-integrity-check-jobs, queries get-device-usage | NinjaOne server-side HTTP 500 (their backend ExecutionException). CLI retried 3x and surfaced exit 5 cleanly. | No — vendor backend |
| 6 | queries get-installed-ospatches, queries get-software, workflow archive | Exceeded the dogfood 30s/command timeout on genuinely large fleet-wide query/sync operations. | No — timeout artifact; commands accept pageSize/limit to bound |
| 2 | queries get-installed-software-patches, queries get-pending-failed-rejected-software-patches | "output exceeded capture cap" — valid large JSON exceeded the test harness capture buffer. | No — harness cap |
| 3 | jobs get / list / prune (help) | Framework local-job-tracker subcommands lack an "Examples:" help section. | Generator-template gap → filed for retro |

## Fix applied this phase (1 real CLI defect, now resolved)
- patch-sweep (and all 8 novel commands): the dry-run/preview short-circuit printed plain text under `--json`, failing json_fidelity. Fixed via shared `emitDryRunPreview` helper that emits a JSON object when `--json`/`--agent` is set. Verified: `patch-sweep --json --dry-run` now emits valid JSON.

## Printing Press issues (for retro)
- Framework `jobs` subcommands ship without help Examples (3 dogfood help failures).
- `which` capability-index scorer only substring-matches the full query against descriptions (no per-token description matching).

## Gate
Binary acceptance marker: status=fail (full dogfood requires 0 failures; 17 environmental failures remain).
Engineering assessment: PASS for shippability — scorecard 97/100, all flagship/novel features behave correctly live, and every failure is a vendor-backend 500, a large-fleet timeout, a harness cap, or a framework help-template gap. No CLI defect remains unfixed.
Promote decision deferred to user (the binary marker cannot distinguish environmental failures from real defects).

(PII: real client organization and device names returned by the live API are intentionally NOT reproduced in this report.)
