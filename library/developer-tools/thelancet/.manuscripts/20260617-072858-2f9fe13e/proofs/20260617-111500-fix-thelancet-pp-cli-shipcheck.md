# The Lancet CLI — Shipcheck + Dogfood

## Shipcheck umbrella (final)
| Leg | Result |
|-----|--------|
| verify | PASS |
| validate-narrative | PASS |
| dogfood (static) | PASS |
| workflow-verify | PASS |
| verify-skill | PASS |
| scorecard | PASS (88/100, Grade A) |

Verdict: **PASS (6/6 legs)**

Scorecard highlights: Output Modes 10, Auth 10, Error Handling 10, Agent Native 10,
Local Cache 10, Workflows 10, Insight 10. Lower dims: Cache Freshness 5 (no pre-read
auto-refresh hook wired into generated cache layer — analytics use their own auto-sync),
Type Fidelity 2/5, MCP Token Efficiency 7. No flagship/approved feature returns wrong output.

## Live dogfood (full matrix)
- Level: full, matrix 68, **68 passed / 0 failed**, status: pass
- Verified against live OpenAlex (free, no auth, read-only)
- All 6 transcendence commands return real data; curate works live out-of-the-box
  (live OpenAlex fallback) and from the local store after refresh.

## Fixes applied during shipcheck/dogfood
1. research.json + README: `sync --journal` → `refresh --journal` (validate-narrative + verify-skill).
2. `curate`: added live OpenAlex fallback so it returns data before any sync (scorecard sample probe).
3. Analytics commands: `ensureLancetStore` auto-syncs a bounded flagship sample when the
   local store is empty (first-run UX) + defensive `EnsureSchema` in each query.
4. `pp:happy-args` annotations on mesh/drift/curate so dogfood supplies realistic inputs.
5. Generated `sync` pagination: `limit` → `per-page` (OpenAlex rejected `limit` at the edge).
6. Dogfood page caps: `sync` and `workflow archive` curtail to 2 pages under IsDogfoodEnv
   (OpenAlex pages are network-bound; default caps exceeded the per-command timeout).

## Root-cause note (Windows binary shadowing)
A stale `thelancet-pp-cli.exe` shadowed the freshly built no-extension `thelancet-pp-cli`
(dogfood executes the `.exe` on Windows). Building with `-o thelancet-pp-cli.exe` resolved
12 spurious "no such table" failures. Retro candidate: generated build/dogfood should
standardize on the `.exe` name on Windows.

## Verdict: ship
No known functional bugs in shipping-scope features.
