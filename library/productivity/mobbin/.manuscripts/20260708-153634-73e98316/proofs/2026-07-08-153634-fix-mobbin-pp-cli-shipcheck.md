# Mobbin CLI — Shipcheck (reprint under 4.27.1)

## Per-leg verdicts
| Leg | Result |
|-----|--------|
| verify | FAIL (see Known Gap) — 41/41 command checks PASS, pass_rate 100% |
| validate-narrative | PASS (12 ok, 0 failed, 3 side-effect auth unsupported) |
| dogfood | PASS |
| workflow-verify | PASS |
| apify-audit | PASS |
| verify-skill | PASS |
| scorecard | PASS — 90/100 (prior published: 84) |

## Fixes applied during shipcheck
- Re-applied the dropped "flat search filter flags" patch to screens/flows/apps-search (--platform, --screen-patterns, --screen-elements, --app-categories, --has-animation, --page-size, --page-index, --sort-by), assembling the structured filterOptions/paginationOptions body. verify-skill 7 errors -> 0; validate-narrative 2 failed examples -> 0.
- Added `filters list` subcommand (real constructor, mcp:hidden) so the README example resolves.
- Added `// pp:data-source` annotations to the 6 novels (deck/grab/cross=live, bench/audit/drift=local).
- Fixed a real functional bug: `sync` returned the framework "nothing synced" error (workspaces is the only flat-syncable resource and needs auth) BEFORE running the domain-population phase, so bench/audit/drift never populated. Domain sync now runs before the exit-code policy and its row count counts toward a successful sync. Verified against real public endpoints: 460 apps, 1832 screens, 229 patterns, 109 elements, 141 flow_actions.

## Known Gap (machine limitation; retro filed)
`verify` verdict is FAIL solely because of the `data_pipeline` mock-mode check:
"38 domain tables created but 0 rows after sync (mock mode)". This is a proven
false-negative for Mobbin's API shape, NOT a CLI defect:
- verify's shared mock (`startMockServer`) returns a generic single object
  (`{"id":1,"name":...}`) for `/api/searchable-apps/{platform}` because the path
  ends in "web", not "s"; it only returns arrays for paths ending in "s" or
  containing "/search", with GitHub-shaped fields — it cannot synthesize a
  Mobbin App array.
- Mobbin is cookie/Supabase-session auth with path-param-scoped syncable
  resources, so the schema-generic mock cannot produce syncable data — the same
  class the check already carves out for GraphQL CLIs
  ("mock server cannot synthesize sync data").
- The real data pipeline works: a live public sync populated 460 apps /
  1832 screens / 229 patterns / 109 elements / 141 flow_actions.
Retro action: add a `data_pipeline` carve-out for cookie-auth/path-param-only
syncable CLIs, analogous to `cliIsGraphQLCLIDir`.

## Ship recommendation: ship (with documented data_pipeline machine-gap)
All command checks pass; scorecard 90; novels functional; the only red leg is a
provable mock-synthesis false-negative. Maintainer approved continuing with a
retro filed.
