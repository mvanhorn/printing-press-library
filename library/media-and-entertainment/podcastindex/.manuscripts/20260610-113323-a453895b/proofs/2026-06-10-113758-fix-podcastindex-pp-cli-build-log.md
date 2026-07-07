Manifest transcendence rows: 1 planned, 1 built. Phase 3 passes (tgrep shipped).

# PodcastIndex CLI — Phase 3 Build Log

## Auth (the load-bearing build)
- Hand-authored SHA1 signer wired so EVERY request (generated endpoint commands + tgrep) authenticates:
  - `internal/client/podcastindex_signer.go` (durable) — computes X-Auth-Key, X-Auth-Date=unix-now, Authorization=sha1hex(key+secret+date), User-Agent per request.
  - `internal/config/podcastindex_secret.go` (durable) — PodcastindexSecret() (PODCASTINDEX_SECRET → client_secret fallback) + PodcastindexAuthKey().
  - One seam line in `internal/client/client.go` doInternal (before headerOverrides) calling c.signPodcastIndex(req). Documented; overrides the templated static composed-auth block. Non-durable single line (flagged for regen).
- Generator's `composed` auth emits a static header and cannot compute per-request SHA1 — hence the signer. Verified the static modelling (4 apiKey schemes) is fully overridden.

## Built & live-verified (real API, signed)
- Core 3: `find search-byterm` (Batman University ✓), `find search-byperson` (Adam Curry ✓), `episodes byfeedid` (Gotham/Lego Batman ✓).
- Parity surface (generator-emitted endpoint commands, all signed): find search-*, episodes by*/random/live, podcasts by*/trending/bytag/bymedium/dead, recent feeds/episodes/newfeeds/soundbites, categories, stats, lookup, value by*, hub.
- Transcendence: `tgrep` (hand-authored, `internal/cli/tgrep.go`) — live-verified against Podcasting 2.0 (feed 920666): scanned 4 episodes, 4 transcripts, 4 matches for "value" with accurate snippets. Bounded by --max-feeds/--max-episodes/--max-scan; dogfood-curtailed; partial-failure accounting; --case-sensitive; SRT/VTT/JSON-segment/plain extraction. Unit-tested (`tgrep_test.go`, 7 tests pass).

## Manifest reconciliation
Absorbed parity commands ship under their generated endpoint paths (e.g. `find search-byterm`, `episodes byfeedid`, `podcasts byfeedid`, `value byfeedid`) rather than the friendlier `find term`/`episodes by-feed` names in the absorb manifest. Functionality is identical; only the command spelling differs. The live-API search parent was renamed `find` (x-pp-resource) to avoid colliding with the framework's offline `search` FTS command.

## Deferred (per gate decision)
dead-watch, drift, resurrect, cadence, guest-graph, value-rank, dedup — product-weak; not built.

## Notes
- Cache extractor warns "no extractable ID field" on find results (feeds[].id is nested under results.feeds); live queries unaffected. Candidate for Phase 4 store-wiring polish.
- add/* (publisher), static/* (file dumps), hub/* remain in the surface but are out of user scope.
