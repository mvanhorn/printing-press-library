# here.now CLI — Shipcheck Report

## Umbrella verdict: PASS (exit 0, all 6 legs green)

| Leg | Result |
|---|---|
| verify | PASS |
| validate-narrative | PASS (9 narrative commands resolved, full examples dry-run clean) |
| verify-skill | PASS (all checks + canonical-sections) |
| dogfood | PASS (novel_features_check: 7 planned == 7 built) |
| workflow-verify | PASS |
| scorecard | PASS |

## Scorecard: 88 / 100 — Grade A
- live-check findings: **none** (novel-feature output sampled against the real API — no broken flagship)

## Toolchain
- `go build ./...` exit 0
- `go vet ./...` exit 0
- `gofmt -l internal/` clean
- `go test ./...` — 336 passed in 11 packages

## Fixes applied this phase (2 loops)
**Loop 1** — verify-skill: SKILL.md/README.md referenced a dropped feature (`search --type site-data`) and the shorthand `publish ./site --anon` (real command is `publish dir`). Reconciled docs to real command paths; replaced the site-data-search recipe with `publishes data list-site-records`.

**Loop 2** — validate-narrative + verify-skill regression:
- `research.json` narrative still said `publish ./site`, `drive sync`/`drive diff`. Corrected to `publish dir`, `drives sync`/`drives diff` across novel_features, novel_features_built, quickstart, recipes, value_prop, when_to_use; re-ran dogfood to resync rendered SKILL/README/root-help blocks; rebuilt.
- `usage` exited 4 without auth — wrong for a free-plan-first CLI. Confirmed the no-auth path already degrades gracefully (local site/publish stats real, drive stats noted "auth required", exit 0); added unit + e2e tests covering it.
- `publish dir ... --dry-run` errored when the target dir didn't exist (validate-narrative runs from a cwd without `./site`). Dry-run now treats a missing dir as a preview (`dir_exists:false`) and exits 0.

## Behavioral correctness (live, witnessed)
All 7 novel features verified against https://here.now (paid test account):
- `publish dir --anon` → live site HTTP 200 with marker content; claim token vaulted; finalize completed.
- `claims` / `claims expiring --within 48h` (count=1) / `claims redeem` (success).
- `publish resume` → reports already-finalized.
- `drives sync` (sha256 diff: 1 uploaded → 0 uploaded/1 skipped on rerun) / `drives diff` (read-only).
- `usage` → sites/drives/drive_bytes/publishes vs free-plan limits; degrades gracefully with no key.

## Ship recommendation: **ship**
All ship-threshold conditions met. No known functional bugs in shipping-scope features.

## Known scope note (not a gap)
Cross-site Site Data search was dropped from scope: the here.now API has no endpoint to enumerate a site's Site Data collections (you can only read a known slug+collection pair), so "search across every collection" is not implementable. Site Data is fully covered by the generated `publishes data` record CRUD commands. Documented in the build log; removed from all narrative surfaces.

## Retro candidates (for /printing-press-retro)
- `profile` API resource hard-fails generation (reserved template collision) instead of auto-suffixing like `analytics`/`auth`; needed manual `x-pp-resource: profile_resource`.
- Generator scaffolds novel features but registered `drives sync`/`drives diff` under a new singular `drive` group and `sites stale`/`publish resume` at top level instead of under their resource parents; needed root.go re-parenting.
- `validate-narrative --full-examples` fails any filesystem-walking novel command whose dry-run reads the path before the dir exists (template-class issue).
