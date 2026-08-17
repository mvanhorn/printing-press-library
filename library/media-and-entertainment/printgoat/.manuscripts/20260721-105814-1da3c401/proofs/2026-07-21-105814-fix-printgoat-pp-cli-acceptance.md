# printgoat Phase 5 Acceptance Report

Level: Full Dogfood
Tests: 150/150 passed (121 skipped as not applicable — e.g. error-path checks on commands with no positional args)
Gate: PASS

## Fix loop (3 iterations)

**Round 1 (16/147 failed):**
- 12 novel commands ("designer stats", "duplicates", "feed", "follow designer list", "formats gaps", "history diff", "job download", "job resume", "library doctor", "license audit", "log fail", "similar", "snapshot verify") failed their happy-path test with "missing runnable example". Root cause: their Cobra `Example:` fields were written as bare argument fragments (e.g. `"gopro mount --agent"`) instead of the canonical full-invocation format generated commands use (`"  printgoat-pp-cli <command> <args>"`), which dogfood's example-extraction parser requires to synthesize a realistic happy-path call. Fixed all 12 (plus `snapshot create`, `follow designer add`, `job resume` for consistency) to the canonical format.
- `search-things` (generated command) failed its error-path check expecting a non-zero exit for a nonsense search term; Thingiverse's API legitimately returns HTTP 200 with an empty `{"total":0,"hits":[]}` envelope for unmatched terms rather than an error. Fixed with `pp:no-error-path-probe` annotation.
- `sync`/`workflow archive`/`workflow status` failed with a real bug: the generated sync engine's `syncResourcePath` table stored a bare relative path (`/categories`) for a resource that actually belongs to the Thingiverse spec in this multi-spec merge, so sync resolved it against the merged CLI's primary (Printables) base URL and 404'd. Fixed by storing the full absolute Thingiverse URL in that table (the same fix pattern as the tier_routing/host_auth.go issue from Phase 3 — cli-printing-press's multi-spec merge doesn't preserve per-resource base-URL context in every generated code path).

**Round 2 (1/151 failed):** `designer stats`'s error-path check expected a non-zero exit for an unknown designer name; since this command is a pure local SQLite aggregation (not an API lookup), a designer with zero logged outcomes is a legitimate empty result, not an error. Fixed with `pp:no-error-path-probe`.

**Round 3 (150/150 passed):** clean. Also fixed a stale Thingiverse example ID (`2409854`, which returns HTTP 403 "Thing has not been published" — a real but non-canonical thing) to the confirmed-working Benchy ID (`763622`) in `research.json`, `SKILL.md`, `README.md`, and the `similar` command's own `Example:` field, eliminating a recurring false-negative in the scorecard's live-check sample probe.

## Full shipcheck re-verified after every round: PASS (7/7 legs) throughout.

## Live-tested this session (beyond the dogfood matrix)

- Real search against Printables (anonymous) and Thingiverse (Bearer token) — both return real, current data.
- Real resumable download of 24 files from a Thingiverse model via the auth-requiring proxied download URL — confirmed the Phase 4.95 auth fix works end-to-end (`bytes_downloaded == file_size` for every file).
- Real `browse category`/`browse user` against Thingiverse.
- Cults3D: reachability confirmed (401 without credentials, as expected per its auth-required-for-everything design); full authenticated live testing not possible this session — only the API key was provided, not the account handle (`CULTS3D_USERNAME`) HTTP Basic Auth also requires.

## Known gaps (documented, not silent)

- Cults3D file downloads: permanently unsupported by upstream API design (search/metadata only).
- `job download`/`job resume`: only Thingiverse files fetch real bytes today; Printables/Cults3D files in the same job are recorded as `unsupported_source`.
- Cults3D live authentication untested this session (missing account handle) — search/metadata code paths are implemented and degrade gracefully on auth failure, but have not been confirmed against a real authenticated Cults3D account.

Gate: **PASS**. Proceeding to Phase 5.5 (Polish).
