# foodpanda CLI — Phase 5 Acceptance Report

    Level:   Full Dogfood (live)
    Tests:   102 / 107 passed  (183 probes incl. skips)
    Gate:    FAIL by marker; all 5 residual failures triaged below as
             environmental or generator-side, each verified by hand.

## Failures (5) — triage

| # | Test | Cause | Verified by hand |
|---|------|-------|------------------|
| 1 | `addresses` happy_path | dogfood sandboxes HOME and overrides `FOODPANDA_*`, so the cookie session cannot reach the subprocess | `addresses --json` → rc=0, returns the real saved addresses |
| 2 | `addresses` json_fidelity | same | same |
| 3 | `home` happy_path | same (depends on `addresses`) | `home --sort fee --limit 25 --agent` → rc=0 |
| 4 | `home` json_fidelity | same | same |
| 5 | `reviews` error_path | foodpanda returns HTTP 200 + empty list for unknown vendor codes; the spec-level `no_error_path_probe` flag is not honored for generated endpoint commands | `digest` (hand-written) opted out successfully via `pp:no-error-path-probe`; `reviews` cannot because it is generated |

Failures 1–4 are the documented cookie-auth-no-harness-session condition: the runner
starts each subprocess in a sandboxed HOME, and `FOODPANDA_CONFIG` + `FOODPANDA_DATA_DIR`
injected from the parent are overridden. Both commands pass locally with a live session.

Failure 5 is a generator gap, filed as a retro candidate. It is not a product defect:
the command returns a valid empty result, which is the honest answer for an API that
cannot distinguish "bad code" from "no reviews".

## Fixes applied during Phase 5 (8 failures → 5)

1. `digest` error_path — annotated `pp:no-error-path-probe` because unknown vendor
   codes return 200 + empty upstream.
2. `sync` was broken out of the box. Two compounding causes:
   - `defaultSyncResources()` contained only `customer` (auth-required) and omitted
     `vendors` entirely, so a bare `sync` synced nothing useful.
   - Sync resolves resource paths against the ROOT `base_url`, ignoring the resource's
     `base_url` override, so it requested
     `disco.deliveryhero.io/api/v5/customers/addresses` and got a Cloudflare 530
     (`zone: dh-discovery-live.net`, origin DNS error).

   Resolved by removing `customer` as a spec resource and shipping `addresses` as a
   hand-written command instead. `sync --resources vendors --global-param latitude=…
   --global-param longitude=… --global-param country=pk` now syncs **543 vendors**.
3. Documented the `search` auto-mode trap: the framework `search` defaults to
   `--data-source auto`, tries the live endpoint, and fails without coordinates.
   Troubleshooting now points at `search --data-source local` or `find`.

## Novel commands — behavioral verification (all 9, live)

| Command | Evidence |
|---|---|
| `home` | Saved address "Home" (Lahore). Real per-vendor fees: Savour Foods 49, KFC 70, others 229. 15/15 `fee_source: vendor-detail` |
| `dish` | `biryani` → 60 matches; cheapest real biryani PKR 126. Nonsense → 0. `--name-only` → 46 strict |
| `menu-diff` | First run records baseline and fabricates no drift; second reports no change; arrays `[]` not null |
| `posture` | 10 ad-buyers / 192 scanned; commission disclaimer present |
| `coverage` | Lahore vs Karachi vendor sets verified disjoint |
| `fees` | Discloses priced ratio; sorted ascending by true total |
| `digest` | Pizza Bake split: food 2.83 (53% low) vs rider 4.64, where the site shows one blended 4.6 |
| `market-compare` | pk 119 / sg 66 / my 160, with avg rating and ad share |
| `find` | `sushi` → 1 strong of 74; `zzzqqqnonsense` → 0 strong of 51, honest note |

Exit-code contract verified across all 9: `--dry-run` → 0 with envelope, bare → help,
missing required input → 2, auth failure → 4.

## Printing Press issues for retro

1. Spec-level `no_error_path_probe` on an endpoint is not propagated to the generated
   command's `pp:no-error-path-probe` annotation.
2. `sync` ignores resource-level and endpoint-level `base_url` overrides, sending every
   resource path to the root host. Silent until the wrong host happens to answer.
3. `defaultSyncResources()` selected an auth-required resource while omitting the
   primary public one, apparently because the public resource has required params.
4. Numeric JSON fields typed `int` in a spec fail to decode when the API returns
   `25.0`. Worth decoding numerics leniently, or warning at generation time.
