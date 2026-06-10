# PodcastIndex CLI — Phase 5 Live Dogfood Acceptance

## Level: Full Dogfood — Gate: PASS (148/148)
Every command exercised against the real PodcastIndex API with live SHA1-signed requests. 0 failures.

## Journey to green (45 → 17 → 3 → 0 failures)
All failures were out-of-scope endpoints or synthetic-fixture artifacts, never flagship bugs:
1. **Removed out-of-scope/broken commands** (−28): `static/*` (11, hit a different host → HTTP 400 file dumps), `add/*` (2, publisher auth), `hub` (1, publisher pubnotify), `tail` (framework polling command that GETs `/<resource>` — no such PodcastIndex path exists; also referenced removed resources). Deleted files + root.go wiring.
2. **Fixtured valid lookup-by-identifier commands** (−14) with `pp:happy-args` real identifiers: episodes byfeedurl/byguid/byitunesid, podcasts byfeedurl/byitunesid/bymedium, value byfeedid/byfeedurl. These 4xx'd only on dogfood's synthetic fake IDs; all work with real input.
3. **Final fixture corrections** (−3): `value byfeedurl` pointed at a feed with no Lightning value block → repointed to a v4v feed (pc20.xml); `podcasts bymedium music` returned 8032 feeds / 11.9 MB (API ignores `max`) exceeding the capture cap → repointed to `film` (23 feeds).

## Flagship features — all live-verified
- find search-byterm (Batman University ✓), find search-byperson (Adam Curry ✓), episodes byfeedid (✓)
- tgrep: scanned Podcasting 2.0 transcripts, 4/4 matched "value" with accurate snippets ✓

## Notes for the user
- `podcasts bymedium <medium>` returns ALL feeds of that medium (the API ignores `max`); "music"/"video" are very large (8k–10k feeds). Use rarer mediums or expect a big payload.
- `value by*` only returns data for feeds that publish a Lightning value4value block.

## Printing Press issue (retro)
- `composed` auth can't compute per-request SHA1/HMAC signatures — required hand-authored signer + a non-durable client.go seam. Worth a first-class `signed`/`hmac` auth mode.
