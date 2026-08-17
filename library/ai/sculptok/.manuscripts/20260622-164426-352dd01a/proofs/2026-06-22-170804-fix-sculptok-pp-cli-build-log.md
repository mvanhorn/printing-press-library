# SculptOK CLI — Phase 3 Build Log

Manifest transcendence rows: 7 planned (research.json novel_features), 7 built. Phase 3 gate passed.

## What was built
- **internal/sculptok/** (sibling client): envelope-aware ({code,msg,data}, HTTP-200) client with multipart upload, submit, status/poll, paged list, AdaptiveLimiter + RateLimitError. Tested (httptest).
- **internal/store/** (modernc.org/sqlite, pure-Go): jobs / credit_events / drawings tables; upsert/list/search/analytics/reconcile; NULL-safe scans. Tested.
- **Transcendence commands (hand-code):**
  - generate depthmap | stl | threed | restore <image> — upload + (optional --restore-first) + submit + poll + persist + optional --out download; --batch over a directory; credit preflight; side-effect guard (no spend under --dry-run / verify / dogfood).
  - cost <kind> — credit preflight vs live balance.
  - search --type jobs|credits|drawings — offline FTS-ish over local mirror.
  - analytics --type credits --group-by actionType|remarks|day — local spend aggregation.
  - reconcile — local join of credit spend vs recorded jobs.
  - sync --resources credits,drawings — backfill the mirror from free history endpoints.
- **Absorbed (generated endpoints):** credits balance/history, draw depthmap/restore/threed/stl/status, drawings list — all resolve.

## Generator gaps found (for retro)
1. Generator emitted calls to `truncateJSONArray` in generated list commands (credits_history.go, promoted_drawings.go) but did not emit the helper -> compile failure at the govulncheck gate. Added internal/cli/truncate_json_array.go as a hand-authored file.
2. No SQLite store / sync / sql / stale emitted for a list-only spec (no get-by-id mirror model). The novel-feature commands the generator scaffolds from research.json are pure TODO stubs with no store foundation. Hand-built the entire store layer (modernc sqlite) + sync to deliver the approved store-dependent features.

## Notes
- Envelope: generated 1:1 commands use response_path: data; hand-built workflow/store commands check code != 0 explicitly (typed errors, exit 2).
- Multipart upload owned by the sibling client (outside the scalar param set).
- Live generation costs credits; guarded against accidental spend in verify/dogfood.
