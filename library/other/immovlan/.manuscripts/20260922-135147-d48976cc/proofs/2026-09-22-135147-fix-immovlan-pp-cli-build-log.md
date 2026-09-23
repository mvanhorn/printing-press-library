Manifest transcendence rows: 6 planned, 0 built. Phase 3 will not pass until all 6 ship.

# immovlan-pp-cli build log (2026-09-22-135147)

## Slice 1 — foundation (2026-09-22, paused by user, no internet)
- Done: `internal/immovlan/` (criteria, Brussels table, HTML parser for cards + detail, tests green on redacted fixtures), `internal/store/immovlan_store.go` (vlan_* tables, upsert, detail, queries, saved/seen/hidden/shortlist, AddrKey/PhotoHash).
- Next slice: `internal/cli/immovlan_common.go` (crit flags, fetch page via client.GetNoCache, open store), then find/show/photos/saved/watch/lists/drops/locations, then the 6 novel scaffolds (enrich, peb-trap, same-as, split-candidates, relisted, agencies), then validate-narrative + shipcheck.
- Resume: `source $SP/env.sh` (scratchpad env.sh holds RUN_ID=20260922-135147-d48976cc and all paths); phase 11 receipt is open; lock held for immovlan-pp-cli.

## Slice 2 — commands (2026-09-22)
- Absorbed rows built: find, show, photos, saved add/list/remove, watch, hide, shortlist add/list/remove, dump (csv/geojson/jsonl), locations, drops. All resolve as Cobra leaves (`Usage: immovlan-pp-cli <leaf> [flags]`), all pass --dry-run and --agent live.
- Transcendence rows built: enrich, peb-trap, same-as, split-candidates, relisted, agencies (6/6). dogfood novel_features_check: planned 6, found 6.
- Live findings folded in: (1) Immovlan ignores `municipals=`; only `towns=` works and it accepts bare postcodes → Criteria.Params emits towns from postcodes (spec updated, regenerated with --force, 20 hand-written files preserved). (2) The generated client rejects HTML unless `X-Printing-Press-HTML-Response: true` is sent → fetchHTML sets it. (3) Headless browsers get 403; plain HTTP with the Chrome UA gets 200 — no browser transport shipped.
- Tests: internal/immovlan (5 funcs on redacted fixtures), internal/store/immovlan_store_test.go (3 funcs), generated scaffold smoke tests. `go test ./...` green.
- Deferred/known: generated `listings search` (links mode) returns link objects whose name is the card ribbon text ("Best of") — the promoted `find` is the real search; dogfood dead helpers/flag (handleBinaryResponseDelivery, hasChangedLocalFlags, isDryRunResponseForClient, readSecretFromStdin, successfulNoop, maxAge) and "sync uses generic Upsert only" are generator-emitted, left for retro.

Manifest transcendence rows: 6 planned, 6 built.
