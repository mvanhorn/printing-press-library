# seats-aero reprint — Phase 3 build log (2026-09-05)

Manifest transcendence rows: 5 planned, 5 built (00:55 2026-09-06). Phase 3 completion gate: per-row Cobra resolution PASS (13 paths), dogfood novel_features_check planned=5 found=5, no stubs — PASS.

Rows: new-since, calendar, direct-scan, reach, recheck (all hand-code). Codex mode via `coding-dispatch.sh codex` (workshop policy — never raw `codex exec`); gate per task: `go build ./... && go vet ./... && go test -count=1 ./...`.

## Findings that shaped the plan
- Typed tables `availability` and `awards` share one row shape; upsert overwrites `created_at`, so first-seen needs a side table + AFTER INSERT triggers (extras.go, the generator-provided hook).
- Default `sync` includes `availability` but omits its REQUIRED `source` query param (dry-run shows `/availability?include_filtered=false&min_cabin_pct=100&skip=0&take=500`) → users must pass `--resource-param availability:source=<program>`. Machine retro candidate: profiler marks a list endpoint with a required enum query param as default-syncable without enum fan-out.
- `doctor.go` has no extension hook → the quota report (absorb row 8) is relocated into the novel commands (`recheck` prints it; `reach` includes it in meta) via a separate-file client helper; doctor is left generated. Recorded, not silent.
- `Client` exposes `BaseURL`, `Config`, `HTTPClient`; `authHeader` is unexported but reachable from a same-package file → `internal/client/seats_aero_quota.go`.

## Task ledger
| Task | Files | Status |
|---|---|---|
| T0 store extras + quota helper | internal/store/extras.go, internal/store/extras_test.go, internal/client/seats_aero_quota.go, internal/client/seats_aero_quota_test.go | done (121210c) |
| T0b view explicit columns + `--db` flag | internal/store/extras.go, internal/cli/novel_store.go, internal/cli/new_since.go | done |
| T0c isolateNovelTest helper + regression test | internal/cli/novel_test_helpers_test.go | done (45cd6fd) |
| T1 new-since | internal/cli/new_since.go, new_since_test.go, novel_store.go | done (b3bae45); 1 of 5 built |
| T2 calendar | internal/cli/calendar.go, calendar_test.go | done (ac4a6e9→a366a27); 2 of 5 built |
| T3 direct-scan | internal/cli/direct_scan.go, direct_scan_test.go | done (fbe43eb); 3 of 5 built |
| T4 reach | internal/cli/reach.go, reach_test.go | done (ad2cb28); 4 of 5 built |
| T5 recheck | internal/cli/recheck.go, recheck_test.go | done (771c79e); 5 of 5 built |

## Incident 2026-09-06 00:05 — live store migrated by a smoke test
Smoke-testing `new-since --json` on the fresh build (no HOME override) let the `x-cache` auto-refresh hook run `sync` in write mode against the operator's live store: schema v1 → v9, installed 2026.6.1 refused to open it. Restored from the verified pre-reprint snapshot (digest e61999ede70d4f74 matches; old binary OK); migrated copy kept as `data.db.migrated-v9-20260906-000827`. Also surfaced a real bug: upgraded stores carry a legacy 86-column `availability` table, so the `SELECT *` UNION view fails → T0b uses an explicit 78-column list + `DROP VIEW IF EXISTS`. Logged to the workshop issues log; memory reinforced. Rule from here: every pre-promotion invocation of the new binary runs with `HOME=<scratch>` or `--db <copy>`.

## Isolation rule adopted 2026-09-06 00:15
The live store was migrated a SECOND time (v1→v9) between restore #1 (00:08:27) and 00:11, while T0b's gate (`go test ./...`) and codex exploration ran. Restored again (copy kept as `data.db.migrated-v9-20260906-001250`) and the live file is now chmod 0444 until promotion. The new CLI honours `XDG_*_HOME` (cliutil.envDir → `XDG_<KIND>_HOME`), so every further dispatch, gate, and smoke run exports XDG_DATA_HOME/XDG_STATE_HOME/XDG_CONFIG_HOME/XDG_CACHE_HOME → `<home>/printing-press/.runstate/printing-press-workshop-e51dd43c/runs/20260905-230744-c6d96a39/pipeline/xdg/{data,state,config,cache}`. Forensic question left open: which process wrote it (a generated test with a non-isolated default path, or codex running the fresh `./seats-aero-pp-cli`).

## 00:14–00:20 — third migration, root cause, regen, .git loss
- **Root cause of migrations #2/#3 (confirmed):** codex-written novel tests (`executeNewSince`, calendar equivalent) execute `RootCmd()` without isolating HOME/XDG; the root `PersistentPreRunE` auto-refresh hook (x-cache `commands: new-since/calendar/…`) runs a write-mode `sync` on the REAL default store before RunE — so `go test ./internal/cli` in a dispatch gate migrates the operator's live store (and the store's private-file mode resets my chmod 0444 to 0600). #3 at 00:14:55 = T2's gate.
- **Design fix:** `availability` cannot be auto-refreshed (sync omits the required `source`) → x-cache reduced to `routes: 168h`, no `commands`. Regenerated with `--force` (cross-spec → novel-only preservation; dropped only generated `auto_refresh.go`/`doctor.go` value drift, as intended).
- **Regen side effect:** the stage-and-swap dropped the working tree's `.git` (commits 9e171e5…ac4a6e9 lost; content preserved). Same failure family as `lock promote` dropping the library `.git` → machine retro candidate. Re-initialised git with a new baseline after verifying every hand-authored file survived.
- **Live store hold:** the (third) migrated copy is parked as `data.db.migrated-v9-20260906-001735`; the live path is EMPTY until promotion, when the verified snapshot (`e61999ede70d4f74`) is restored. The installed 2026.6.1 CLI has no store until then.
- **Isolation rule (all further gates/dispatches):** export `XDG_DATA_HOME/STATE/CONFIG/CACHE` → `pipeline/xdg/*` and `SEATS_AERO_NO_AUTO_REFRESH=1`; novel tests must call a shared `isolateNovelTest(t)` helper (`testenv.Isolate` + opt-out env) — T0c.

## What was built (Phase 3 summary)
- Store: `availability_first_seen` side table + AFTER INSERT triggers on `availability`/`awards`; `availability_all` view (explicit 78-col list, DROP+CREATE); route/date indexes. Client: `ProbeQuota` (X-RateLimit-* via a 1-call `/destinations?origin_airport=JFK` probe).
- Novel: `new-since` (local), `calendar` (local), `direct-scan` (local), `reach` (auto; `--confirm-live` opt-in), `recheck` (auto; print-only by default, `--apply` spends credits, harness-refused). All take `--db`; all tests isolated via `isolateNovelTest`.
- Intentionally deferred: quota report inside `doctor` (no extension hook — lives in `recheck`); live `/search` fallback in `reach` is opt-in only (quota); README troubleshooting for `sync --resources availability --resource-param availability:source=<program>` to be added in Priority 3 after shipcheck's README sync.
- Generator limitations found: see the Isolation/Incident sections above + RESUME checkpoint 2.

## Phase 4 notes (2026-09-06 ~01:10)
- shipcheck #1/#2: 6/7 legs PASS, scorecard HOLD on `live_api_verification` — verify stayed `Mode: mock` even with `--env-var SEATS_AERO_API_KEY` (both via shipcheck passthrough and standalone). `--api-key "$SEATS_AERO_API_KEY"` switches to `Mode: live` (43/43, PASS). **Retro candidate:** `verify --env-var` does not enable live mode.
- Standalone `scorecard` after live verify: Live API Verification 10/10 but path_validity/auth_protocol flipped to "unverified" outside the umbrella → re-ran the full `shipcheck --api-key` so legs run in canonical order (#3).
- Generated dead helper `handleBinaryResponseDelivery` (Dead Code 4/5) is generator-owned → retro, not patched here.

## Fix wave after the review gate (2026-09-06 01:30–02:10)
- T6a (4bd40dd): recheck cutoff `datetime(synced_at) <= datetime(?)` with a `YYYY-MM-DD HH:MM:SS` bind; reads `availability_all`; fail-closed quota guard (`--ignore-quota` override); `classifyAPIError` on POST; `--data-source local` skips the probe and refuses `--apply`; ProbeQuota sends the client's User-Agent/Accept and treats 401/403 as errors. Tests seed `synced_at` through the real RFC3339 encoding + the `CURRENT_TIMESTAMP` form. **Mutation-verified:** dropping `datetime()` fails `recheck_test.go:133` (`same-day-8h` missing).
- T6b (47275f0 + fix-up a88748a): reach — no local mode (usage error), `--top` ≤ 50, ≤ 10 live checks, no null `quota`, `sql.NullString` scans; uniform `YYYY-MM-DD` dates; calendar inclusive end + index map; new-since `datetime()` guard; `executeRoot` test helper; helper test asserts `IsNotExist`. **Mutation-verified:** flipping the effective-miles CASE fails `TestDirectScanFiltersLocalAwards`. Process slip: a mutation-check `git checkout` run BEFORE the T6b commit reverted codex's `direct_scan.go`; restored by hand from the dispatch-log diff (rule: never mutate an uncommitted tree).
- T7: docs patch recovered from `.git/coding-dispatch-last-fail.patch` (the dispatch failed only on the then-broken direct-scan test); `validate-narrative --strict --full-examples` OK, `verify-skill` OK.
- Deferred to machine retro: `runLearnInitOnce` RW-opens the default store for every command; DSN `?` splitting; MCP mirror does not block `--db`; `awards` Short truncation; `meta.source` hardcoded "local" for hand-written commands.

## Phase 5 (2026-09-06 07:00–07:20) — live dogfood PASS
Full matrix 101/101, 93 structural skips, no hollow features after 4 runs (recheck classification → stale-binary → `Example` still carried `--dry-run`). Acceptance marker: `proofs/phase5-acceptance.json` status=pass (runner-written). Report: `proofs/2026-09-06-071500-fix-seats-aero-pp-cli-acceptance.md`. Next: 5.5 polish → 5.6 promote (restore live store from snapshot FIRST; back up library .git; refresh archive; repoint runstate current) → session report → publish.
