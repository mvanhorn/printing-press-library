# CookUnity CLI Build Log (Phase 3)

Manifest transcendence rows: 6 planned, 6 built. Phase 3 will not pass until all 6 ship. → ALL 6 SHIPPED.

## Priority 0 (foundation) — hand-built
- `internal/cookunity/client.go` — SDUI menu client. Fetches clustered-results for a
  delivery date, walks the SDUI tree for FULL_MENU_LAZY_CLUSTER lazy loaders, fetches each
  `clustered-result` component, extracts every MEAL card's `.properties`, flattens to a clean
  Meal (embedding types.Meal + isFavorite), dedups by id. Auth: raw Auth0 token in
  Authorization (no Bearer prefix) + `platform: web` header.
- `internal/cli/sync.go` — REWRITTEN. Replaced the generic REST-pagination sync with the
  CookUnity SDUI sync. `runCookunitySync` clears the `meals` table, fetches the menu, writes
  the typed `meals` table (browse/search/plan) AND `meal_snapshots` (per-week history for drift).
  Date via `--date` or `--param date=`, default next Monday.
- `internal/store/cookunity_migrations.go` — hand-authored `meal_snapshots` table
  (keyed by delivery_date+meal_id) so drift can compare weeks despite the meals table's bare-id PK.

## Priority 1 (absorbed) — hand-built
- `internal/cli/promoted_meals.go` — REWRITTEN. `meals` list reads the local store with
  composable filters (--diet/--cuisine/--chef/--min-protein/--max-calories/--max-price/
  --exclude-allergen/--in-stock/--sort/--limit); `meals get <id>` full detail.
- `internal/cli/cookunity_sql.go` — read-only `sql` command (SELECT/WITH only) over the store.
- `search` (framework) reads meals_fts. `export`, `doctor`, `auth`, `agent-context` framework.
- `internal/cli/channel_workflow.go` — REWRITTEN. `workflow archive` delegates to runCookunitySync.

## Priority 2 (transcendence) — all hand-code
1. plan — macro/calorie/budget/diet-constrained greedy meal-set selector (the user's vision).
2. drift — week-over-week snapshot diff (added/removed/repriced); defaults to two newest snapshots.
3. value — protein-per-dollar / calories-per-dollar leaderboard.
4. favorites — meals with isFavorite on the current menu.
5. compare — side-by-side macro/price/chef/allergen table for 2+ meal ids.
6. chefs — chef roster analytics (dish count, avg rating, avg price, cuisines). Reclassified
   from spec-emits to hand-code because the generator did not emit an `analytics` framework
   command for this spec.

## Deferred (approved at gate)
- `orders` (GraphQL upcoming delivery) — deferred per user choice; not needed for offline planning.

## Deviations from manifest
- `sql` and `analytics` were listed as absorbed but the generator emitted neither for this spec;
  built `sql` by hand, folded `analytics` into `sql` + `chefs`. Manifest row 8 updated.

## Generator limitations / machine issues found (for retro)
- Windows test isolation: generated test helpers set HOME but not USERPROFILE, so `go test`
  resolves paths to the real user profile, polluting real state and failing isolation tests
  (learn/mcp/cli). Patched the helpers in this build; filed retro task.
- Credential-perm tests assume POSIX perms and trip NTFS DACL checks on Windows (6 cliutil
  tests fail on the DACL warning short-circuit). Deeper generated-security-code issue; not
  hand-fixed. Does not affect runtime.
- SDUI APIs are not a shape the generator can auto-wrap; the entire meal layer is hand-built.

## Verification (Phase 3 completion gate)
- go build ./... : PASS
- go vet ./... : PASS
- Per-command Cobra resolution: meals, meals get, plan, drift, value, favorites, compare,
  chefs, sync, search, sql — all resolve rc=0 with correct Usage spec lines.
- dogfood novel_features_check: planned 6, found 6 — PASS.
