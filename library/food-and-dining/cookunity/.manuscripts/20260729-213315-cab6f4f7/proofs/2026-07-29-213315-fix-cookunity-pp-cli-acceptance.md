# CookUnity CLI Acceptance Report (Phase 5 — Full Live Dogfood)

## Gate: PASS
- Level: Full Dogfood (binary-owned live matrix)
- Tests: 92/92 passed, 0 failed
- Auth: api_key (COOKUNITY_TOKEN, raw Auth0 token + platform:web header)
- Acceptance marker: proofs/phase5-acceptance.json (status: pass)

## Live behavioral verification against the real CookUnity API
- **sync**: 221 real meals synced for the delivery week (endpoint is date-lenient; both
  2026-08-03 and 2026-08-04 return the week's menu). This IS the user's primary goal —
  the complete meal list mirrored locally for offline planning.
- **sync is atomic**: two consecutive syncs keep the count at one week (221), not doubled
  (upsert-then-prune-by-delivery-date; table never empty).
- **meals** (filtered): `--diet "high protein" --min-protein 40 --max-calories 600 --sort protein`
  returns high-protein meals sorted by protein desc, all matching the constraints. Relevant.
- **plan** (flagship / user vision): `--protein-min 300 --calories-max 700 --count 8 --diet "gluten free"`
  built an 8-meal set totaling 364g protein / 4350 cal / $78.82, meets_protein=true. Works.
- **value**: protein-per-dollar leaderboard tops out at chicken breasts (5.46 g/$). Relevant.
- **chefs**: real chef leaderboard (Maribel Rivero 19 dishes, avg $12.87); cuisines split correctly.
- **search**: FTS returns "Chicken & Dumplings" as top hit for "chicken". Relevant.
- **drift**: runs cleanly across two synced snapshots (0/0/0 between two dates in the same
  delivery week — correct; mechanism verified).
- **sql**: read-only SELECT works, including literals containing keyword substrings ("%Grill%").
- **doctor**: config ok, auth configured, env var present, API reachable.

## Fixes applied this phase (all from Phase 4.95 review + dogfood, verified live)
1. client.go cluster 401/403 now hard-errors (no silent catalog wipe).
2. client.go returns error when clusters found but zero meals extracted.
3. sync.go atomic upsert-then-prune-by-delivery-date.
4. cookunity_sql.go dropped false-positive keyword blocklist (read-only handle is the barrier);
   threads ctx via QueryContext.
5. chefs.go splits joined cuisines.
6. Removed the raw-SDUI `menu` command group (menu view/cluster): `menu cluster` required a
   live per-menu reference UUID (HTTP 500 "Reference parameter is required"), returned raw SDUI,
   and duplicated the sync/meals path. Removed from CLI, root.go, MCP tools list, README, SKILL,
   and the spec. This resolved the only 2 live-dogfood failures (95/97 -> 92/92).

## Printing Press issues (for retro)
- Windows generated-test isolation (HOME/USERPROFILE) and NTFS-DACL credential-perm tests
  (filed as a retro task).

## Verdict: ship
