# Instagram CLI Build Log

Manifest transcendence rows: 7 planned, 7 built. Phase 3 gate passed (planned==found, missing none).

## Built
- Data layer: hand-authored `internal/store/instagram_analytics.go` (ig_brands, ig_account_snapshots, ig_brand_media, ig_tracked_competitors, ig_competitor_snapshots, ig_tracked_hashtags, ig_hashtag_snapshots) + idempotency test.
- Brand registry + collector: `internal/cli/instagram_brands.go` — `brands add/list/rm/discover/track-rival/track-hashtag` + top-level `pull` (live collector, dogfood-capped, parallel media-insight fan-out with partial-failure accounting). Wired into root.go.
- 12 absorbed endpoints generator-emitted from the internal YAML spec (accounts, account-insights, media incl. publish, stories, comments, hashtags, business-discovery, tags).
- 7 transcendence commands (hand-authored, store-backed, `pp:data-source local`, mcp:read-only): compare, growth, best-time, top-posts, formats, rivals, hashtag-perf. Each: verify-friendly RunE, NULL-safe scans, empty-store honest note, behavioral tests with content assertions.

## Generator note
- v4.21.0+ pulls in tinygo CoreBluetooth (cbgo) which SIGABRTs on macOS 26.5.1; pinned generation to v4.20.1 (above the 4.19.1 currency floor, no BLE dep). Filed for retro.

## Fix applied during Phase 4
- growth.go: replaced strict time.RFC3339 parse with tolerant `parseIGTime` so the Graph `+0000` (no-colon) timestamp format is handled — weeks_covered was silently 0; now correct.

## Deferred
- Live Phase 5 dogfood: no INSTAGRAM_ACCESS_TOKEN available; deferred (skip marker). Structural + behavioral (seeded-store) verification done.
