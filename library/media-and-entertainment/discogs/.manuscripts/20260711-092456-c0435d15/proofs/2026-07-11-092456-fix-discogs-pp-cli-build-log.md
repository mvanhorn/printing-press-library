# Discogs CLI — Build Log

Manifest transcendence rows: 7 planned, 0 built. Phase 3 will not pass until all 7 ship.

Planned transcendence commands (all hand-code): fills, portfolio, undervalued, comps, sell-plan, identify, pressings.
Substrate hand-build: internal/store/discogs_migrations.go (price_snapshots, wantlist_limits, discogs_meta) + internal/cli/discogs_shared.go + hand-built `sync` + `limit` commands.

## Progress

Manifest transcendence rows: 7 planned, 7 built.

- Foundation: internal/store/discogs_migrations.go (price_snapshots, wantlist_limits, discogs_meta + Insert/Set/Meta helpers) + internal/cli/discogs_shared.go (store open, username resolution via /oauth/identity w/ caching, marketplace-stats parse, snapshot capture/read, trailing median, data-source guard).
- Hand-built populate: `sync` (wantlist/collection/inventory → resources table; bounded price snapshots; dogfood-curtailed) + `limit` set/list/rm (per-release max price).
- 7 transcendence commands: fills (flagship), portfolio, undervalued, comps, sell-plan, identify, pressings.
- Verified: build clean; go test ./internal/cli ok; all 7 novel leaves resolve as Cobra commands; novel_features_check planned=7 found=7 missing=none skipped=false; flagship `fills` returns a real live fill token-less (marketplace stats is public).
- Deferred (honest non-goals, not stubs): move-instance, edit-custom-field-value, edit-profile, multipart CSV upload-write.
