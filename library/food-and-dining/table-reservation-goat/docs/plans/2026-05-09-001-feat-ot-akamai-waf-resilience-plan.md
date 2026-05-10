---
title: feat: OT Akamai WAF resilience (cache + dedupe + adaptive-rate-limiter + retry)
type: feat
status: active
date: 2026-05-09
origin: docs/brainstorms/2026-05-09-ot-akamai-waf-resilience-requirements.md
---

# feat: OT Akamai WAF resilience

## Summary

Wraps `RestaurantsAvailability` with a three-layer resilience envelope — disk cache, singleflight dedupe, two-additional-attempt retry (3 total attempts) — plus a stale-cache fallback when Akamai escalates. Reuses the existing `AdaptiveLimiter` already in the OT client for outbound rate limiting (2s baseline, auto-adjusts on 403s) rather than adding a new fixed-floor throttle. Self-contained: no daemons, no proxies, no external services. The CLI surface stays unchanged except for one opt-out flag and a documented `HTTPS_PROXY` escape hatch.

---

## Problem Frame

OT's `RestaurantsAvailability` GraphQL endpoint sits behind a probabilistic Akamai WAF rule that scores by IP+session and escalates under rapid-fire load. The CLI hits this aggressively in two natural patterns: multi-venue queries and multi-day windows (the new gateway is single-day-only, so `--within 7d` loops 7 calls). Real users at 1-2 queries every few minutes never escalate; bursty CLI traffic does. The fix lives entirely in the boundary between the CLI and that one endpoint. (See origin: `docs/brainstorms/2026-05-09-ot-akamai-waf-resilience-requirements.md`.)

---

## Requirements

- R1. Successful availability responses are cached on disk keyed by `(restaurantID, date, time, partySize)` with default 10-minute TTL; reads check cache before network.
- R2. Cache TTL is configurable via env var; reads after TTL miss and re-fetch.
- R3. `--no-cache` flag and corresponding env var bypass cache reads but still write on success.
- R4. When the network returns `BotDetectionError` AND a cached entry exists for the same key, return the cached entry with a `stale: true` marker (when past TTL) so JSON consumers can see it's older.
- R5. Concurrent in-process calls for the same key are coalesced via singleflight.
- R6. Outbound `RestaurantsAvailability` calls go through the existing `cliutil.AdaptiveLimiter` attached to the OT client (2s baseline, auto-ramps on 403s, halves on 429s). No new fixed throttle floor. Cached or singleflighted-coalesced reads do NOT call through the limiter.
- R7. On 403 from the OT GraphQL endpoint, retry up to two additional times: first at 750ms, second at 5s; after the second failure, return `BotDetectionError`.
- R8. Retry attempts within one logical call do not call through the AdaptiveLimiter (the limiter governs distinct logical calls; retries are bounded by their own backoff).
- R9. The CLI honors standard `HTTPS_PROXY` / `HTTP_PROXY` env vars (no code path overrides them).
- R10. README and command help document `HTTPS_PROXY` as the supported escape hatch for personal proxy / Tor SOCKS5.
- R11. Cache TTL and AdaptiveLimiter initial rate are env-overridable; out-of-range values are clamped or rejected with a clear error.

**Origin acceptance examples:** AE1 (covers R1, R2, R5), AE2 (R6), AE3 (R4), AE4 (R7), AE5 (R3), AE6 (R9, R10).

---

## Scope Boundaries

- TLS fingerprint rotation between Chrome/Firefox impersonation. Out of scope per origin.
- Long-running daemon, bundled Tor, auto-rotating free proxies. Out of scope per origin.
- Caching of `Autocomplete`, `Bootstrap`, or any other OT operation — only `RestaurantsAvailability` warrants this layer. Tock-side calls excluded.
- Persistent on-disk cooldown for the per-call retry path. The existing session-wide cooldown (in `internal/source/opentable/cooldown.go`) keeps owning bootstrap-path 403s; per-call retry stays in-process.

---

## Context & Research

### Relevant Code and Patterns

- **Disk-cache pattern with atomic write + TTL gating.** `internal/source/auth/chrome.go` already implements `loadAkamaiCacheRaw` / `saveAkamaiCache` via `os.UserCacheDir()` + `os.WriteFile(.tmp, …)` + `os.Rename` at mode `0600`. Same pattern, different cache file. **This is the canonical reference for cache-dir convention** — `internal/source/opentable/cooldown.go` uses a hardcoded `~/.cache/...` path that is XDG-only and breaks on macOS / Windows; do not follow that pattern.
- **Singleflight for dedupe.** `internal/source/opentable/client.go` already uses `golang.org/x/sync/singleflight` in `Bootstrap()` (`bootstrapSF.Do("csrf", …)`). Reuse the same package — same key naming convention but per-call key, e.g., `fmt.Sprintf("avail:%d:%s:%s:%d", restID, date, time, party)`.
- **Existing AdaptiveLimiter rate-limit pattern.** `internal/cliutil/ratelimit.go` `AdaptiveLimiter` is already constructed in `opentable.New()` at 0.5 calls/sec and called from `do429Aware`. U2 extends this by making the initial rate env-overridable; no new component is needed.
- **Typed `BotDetectionError`.** `internal/source/opentable/cooldown.go` defines `*BotDetectionError`. Use `opentable.IsBotDetection(err)` to test for it; the stale-cache fallback (R4) keys on this type.
- **Existing `do429Aware` retry path.** `internal/source/opentable/client.go` already retries 429s once and 403s once (after 750ms). U4 extends the 403 path to add a second retry at 5s before surfacing the typed error.

### Institutional Learnings

No matching `docs/solutions/` entries — the CLI is a recent printed CLI without curated learnings yet. The patches manifest at `.printing-press-patches.json` is the closest equivalent and already documents the cooldown/akamai-cache shape this plan extends.

### External References

None used. Local patterns cover everything.

---

## Key Technical Decisions

- **Cache file layout: one JSON file per key, under `<UserCacheDir>/table-reservation-goat-pp-cli/ot-avail/`.** Mirrors the shape `auth/chrome.go` uses for Akamai cookies and avoids SQLite for a small TTL'd cache. File naming uses a SHA-256 hash of the canonicalized key string `<restID>|<date>|<time>|<party>|<forwardMinutes>|<backwardMinutes>` truncated to 16 hex chars, with `.json` suffix. Hashing avoids path-traversal vectors from user-influenced `--date`/`--time` inputs and keeps filenames a fixed length regardless of input shape. The full key is stored inside the file (alongside `Hash`, `SchemaVersion`, `FetchedAt`, `CachedAt`, and the response payload) for verification on read and human inspection.
- **Cache key includes the GraphQL request window (`forwardMinutes`, `backwardMinutes`).** Different callers pass different windows — `earliest` uses 210/210, `watch` uses 150/150 in its current code path — and the gateway returns different slot ranges per request. Keying without these fields would let one caller's response be served to another's request. The singleflight key uses the same canonical string so dedupe and cache stay consistent.
- **Cache invalidation on persisted-query hash rotation OR cache-schema bump.** Cache file embeds both the `RestaurantsAvailabilityHash` constant value AND a `SchemaVersion: 1` integer at write time. Reads compare both to current values and treat any mismatch as a cache miss. Prevents stale entries from surviving a gateway-side query rotation OR a CLI-side cache-shape change.
- **Default cache TTL = 3 minutes (down from initial 10-minute proposal).** OT slot tokens are described as "short-lived (~minutes)" in `internal/source/opentable/client.go`. A cache TTL longer than the slot lifetime means cached responses contain unbookable tokens. 3 minutes is conservatively under the bound; users who want longer can override via `TRG_OT_CACHE_TTL`. Clamp range is `[1m, 24h]`.
- **Reuse the existing `cliutil.AdaptiveLimiter` for rate limiting; do NOT add a new fixed-floor throttle.** The OT client already constructs `c.limiter = NewAdaptiveLimiter(0.5)` (2s baseline) and routes outbound requests through `c.limiter.Wait()` inside `do429Aware`. The AdaptiveLimiter's design — start low, ramp up on success streaks, halve on rate-limit signals — is exactly the adaptive behavior we want against a probabilistic WAF. Adding a fixed 30s floor on top would compound waits unnecessarily; cold `--within 7d` would inflate to ~3min for marginal Akamai-defense gain. The empirical signal from this session (escalation triggered at ~10-15 calls in 2 minutes, suggesting the danger zone is "more than 1 call per 8-12 sec sustained") is well within the AdaptiveLimiter's natural ramp range. Reuse beats invent.
- **Initial rate is overridable via `TRG_OT_THROTTLE_RATE` (calls per second).** Default stays at the existing `0.5`. Power users wanting more aggressive throttling can set `TRG_OT_THROTTLE_RATE=0.1` (10s spacing). Users on a private proxy can set `TRG_OT_THROTTLE_RATE=2.0`. The AdaptiveLimiter still ramps and halves around whatever initial value is set.
- **`--no-cache` semantics: bypass reads, still write on success.** A user who wants guaranteed-fresh data is also willing to update what's cached. Skipping writes would force the next caller to also fetch fresh, wasting requests. Settled here so U6 doesn't defer this question.
- **`RestaurantsAvailability` signature: add `noCache bool` as a positional parameter.** No need for an option pattern at v1; only one new flag. Existing callers (`internal/cli/earliest.go`, `internal/cli/watch.go`) update both call sites. If future flags arrive, refactor to options then.
- **Stale-cache fallback uses a `source` field, not the `stale` field.** When a cache entry is returned because the network 403'd, the row carries `source: "cache_fallback"` regardless of the entry's age. The `stale: true` flag separately indicates the entry is past TTL. This separation lets JSON consumers distinguish "served from a fresh cache hit" / "served from a not-yet-expired fallback" / "served from an expired-but-recent fallback" without overloading one boolean.
- **Retry placement: scope the second retry (5s) to the RestaurantsAvailability call site, not in shared `do429Aware`.** `do429Aware` is also called by `Bootstrap` and `Autocomplete`. Widening its retry path silently extends those operations too — out of declared scope and risks compounding their session reputation. The first retry (750ms) stays in `do429Aware` (it's the existing behavior); the second retry lives in the U3 wrapper around `RestaurantsAvailability` only. Total worst-case wall time stays ~6s for `RestaurantsAvailability`; other operations are unchanged.
- **All retry sleeps must be ctx-aware.** The existing 750ms `time.Sleep` in `do429Aware` is upgraded to `select { case <-time.After(...): case <-ctx.Done(): }` as part of U4. The new 5s retry uses the same shape. The existing `AdaptiveLimiter.Wait()` does NOT take ctx and uses bare `time.Sleep`; that's a known limitation we're inheriting (out of scope for this plan — fix it as a separate `cliutil` improvement if it bites).
- **Env var prefix: `TRG_OT_*`.** Shorter than the existing `TABLE_RESERVATION_GOAT_OT_CHROME_DEBUG_URL` precedent. Covers `TRG_OT_CACHE_TTL`, `TRG_OT_THROTTLE_RATE`, `TRG_OT_NO_CACHE`. Document in README.

---

## Open Questions

### Resolved During Planning

- **Cache file shape (one-per-key vs single-DB).** Resolved: one-per-key JSON files with hashed filenames. Cheap to read/write/invalidate, no schema concerns, mirrors existing `auth/chrome.go` precedent. SQLite would add a dep with no upside for ≤100 entries.
- **Where rate limiting lives.** Resolved: in the existing AdaptiveLimiter that's already inside `do429Aware`. No new component. The brainstorm initially considered a separate fixed-floor throttle, but cold-start UX cost (3+ minutes for `--within 7d`) outweighed the marginal additional WAF defense over the existing adaptive approach.
- **Stale-cache age cap.** Resolved: 24-hour hard cap on stale-cache fallback. Beyond 24 hours, treat the cache entry as expired-not-stale and surface `BotDetectionError` honestly. Avoids returning week-old slot times.
- **Cache key shape.** Resolved: include `forwardMinutes` and `backwardMinutes` (the GraphQL request window) in addition to `(restID, date, time, party)`. Different callers pass different windows; without these the cache cross-contaminates.
- **Default cache TTL.** Resolved: 3 minutes (down from initial 10-minute proposal). Slot tokens are "~minutes" per the existing client code; a TTL longer than slot lifetime risks serving unbookable tokens. Configurable via `TRG_OT_CACHE_TTL`.
- **`--no-cache` write semantics.** Resolved: bypass reads, still write on success. A user wanting fresh data is also willing to update what's cached; skipping writes wastes future requests.
- **`RestaurantsAvailability` signature.** Resolved: add `noCache bool` as a positional parameter. Only one new flag at v1; option pattern would be premature.
- **Retry placement (single vs split layers).** Resolved: first retry (750ms) stays in `do429Aware` so all OT ops keep their existing single-retry behavior; second retry (5s) lives in U3's wrapper around `RestaurantsAvailability` so only that operation pays the longer budget. Keeps Autocomplete and other ops out of the new behavior.
- **Floor=0 vs TTL=0 asymmetry.** Resolved: floor=0 is a documented power-user value meaning "no spacing" (for users with their own external rate-limiter); TTL=0 is invalid (clamps to default). Documented in env var section.

### Deferred to Implementation

- Exact JSON encoding of `cached_at` (RFC3339Nano vs Unix epoch) — pick whatever feels native at code-write time; downstream `printJSONFiltered` consumers don't care.
- Stale-cache-fallback ordering vs retry sequence — current plan retries first (~6s) then falls back to cache. An optimization could check stale cache BEFORE retries on the second day of a within-loop after day 1 already 403'd (saves ~6s per remaining day). Defer as a v1.1 perf opportunity; v1 retries-first keeps the failure path uniform.
- Session-fingerprint embedding in cache entries to handle the cross-session slot-token problem (a stronger fix than the 3m TTL reduction). 3m TTL handles most cases since slot tokens live "~minutes"; a session-fingerprint check is a deeper guarantee but adds complexity. Defer as a v1.1 hardening if cache-served tokens fail at booking time in the wild.

---

## Implementation Units

- U1. **Disk cache for RestaurantsAvailability**

**Goal:** Add a key-keyed JSON cache layer for OT availability responses with TTL gating, atomic writes, and schema-versioned invalidation.

**Requirements:** R1, R2, R11

**Dependencies:** None

**Files:**
- Create: `internal/source/opentable/avail_cache.go`
- Test: `internal/source/opentable/avail_cache_test.go`

**Approach:**
- Define `availCacheKey` (restID, date, time, party, forwardMinutes, backwardMinutes) and `availCacheEntry` (Key, FetchedAt, CachedAt, Hash, SchemaVersion, Response `[]RestaurantAvailability`). The key includes the GraphQL request window so different callers' responses don't collide (per Key Technical Decisions).
- Cache file path: `<os.UserCacheDir()>/table-reservation-goat-pp-cli/ot-avail/<keyhash>.json` where `keyhash` is the first 16 hex chars of `sha256(restID|date|time|party|forwardMinutes|backwardMinutes)`. Validate `date` matches `^\d{4}-\d{2}-\d{2}$` and `time` matches `^\d{2}:\d{2}$` before key construction; reject malformed input with a clear error rather than encoding it into a filename.
- `loadAvailCache(key, currentHash) (*availCacheEntry, bool)` — returns entry + freshness flag (true = within TTL, false = past TTL but still readable for stale fallback up to 24h). Returns nil when missing, corrupt, hash-drifted, schema-version-drifted, or past 24h. On read, compare the cache entry's embedded `Hash` to `currentHash` (the active `RestaurantsAvailabilityHash` constant) AND `SchemaVersion` to the current schema version; mismatch on either treats the entry as a cache miss.
- `saveAvailCache(key, entry)` — write-then-rename atomic, mode `0600`, mkdir parent. Stamps `CachedAt = time.Now()` so U5 can surface it on stale fallback.
- TTL read from `TRG_OT_CACHE_TTL` env var (Go `time.ParseDuration`), default `3m` (chosen to stay under OT's documented "~minutes" slot-token lifetime). Clamp to `[1m, 24h]`; values outside range fall back to default with a stderr warning.

**Patterns to follow:**
- `internal/source/auth/chrome.go` — `loadAkamaiCacheRaw`, `saveAkamaiCache`, `akamaiCachePath` (mirror file/dir handling and atomic-rename pattern verbatim).
- `internal/source/opentable/cooldown.go` — error envelope and best-effort disk semantics.

**Test scenarios:**
- Happy path: write entry, read it back within TTL → returns entry with `fresh=true`.
- Edge case: read past TTL but within 24h → returns entry with `fresh=false` (stale-but-valid).
- Edge case: read past 24h → returns nil (treat as missing).
- Edge case: read with mismatched persisted-query hash → returns nil (cache invalidated by rotation).
- Edge case: read with mismatched `SchemaVersion` → returns nil (cache invalidated by CLI cache-shape change).
- Edge case: corrupt JSON file → returns nil without crashing; logs nothing user-visible.
- Edge case: `os.UserCacheDir()` fails → load and save are best-effort no-ops; caller falls through to network path.
- Edge case: `TRG_OT_CACHE_TTL=0`, `TRG_OT_CACHE_TTL=-1m`, `TRG_OT_CACHE_TTL=48h` → all fall back to 3m default with stderr warning.
- Edge case: malformed `--date` (e.g., `2026-13-99`) or `--time` (e.g., `25:99`, `../etc/passwd`) → key construction returns an error; load/save are not called.
- Happy path: filesystem-portable filenames — output is hex-only (`[a-f0-9]{16}.json`) regardless of input shape; works on case-sensitive and case-insensitive filesystems.
- Happy path: same logical request through different key permutations (e.g., key only differs by `forwardMinutes`) produces different cache files — earlier keying issues do not collide.

**Verification:**
- Cache files appear under the expected directory after a successful call.
- Re-running the same query within the TTL window returns identical bytes without firing a network call.

---

- U2. **Configurable initial rate for the existing AdaptiveLimiter**

**Goal:** Expose `TRG_OT_THROTTLE_RATE` as an env-var override of the existing `cliutil.NewAdaptiveLimiter(0.5)` initial rate. No new throttle component — the existing AdaptiveLimiter (already attached to the OT client and already wired through `do429Aware`) handles outbound rate limiting and adaptive backoff.

**Requirements:** R6, R11

**Dependencies:** None

**Files:**
- Modify: `internal/source/opentable/client.go` (the `New()` constructor that initializes `c.limiter`)
- Test: extend `internal/source/opentable/client_test.go` (or create) — env-var read + clamp logic.

**Approach:**
- In `opentable.New()`, replace the hardcoded `NewAdaptiveLimiter(0.5)` with `NewAdaptiveLimiter(readThrottleRate())`.
- `readThrottleRate()` reads `TRG_OT_THROTTLE_RATE` (Go `strconv.ParseFloat`). Default `0.5` (preserves existing behavior). Clamp to `[0.01, 5.0]` calls/sec — values outside fall back to default with stderr warning. `0.01` ≈ 100s spacing (paranoid mode); `5.0` = 200ms spacing (private-proxy mode).
- The AdaptiveLimiter's existing ramp-up (1.25× after 10 successes) and halve-on-rate-limit behavior is unchanged — `OnSuccess` and `OnRateLimit` are already called from `do429Aware` and `gqlCall`.

**Patterns to follow:**
- Existing env-var read pattern in `internal/source/auth/chrome.go` — `os.Getenv` + `strconv.ParseDuration` + clamp + default fallback.

**Test scenarios:**
- Happy path: env var unset → limiter starts at `0.5` (existing behavior preserved).
- Happy path: `TRG_OT_THROTTLE_RATE=0.1` → limiter starts at 10s spacing.
- Edge case: `TRG_OT_THROTTLE_RATE=0`, `TRG_OT_THROTTLE_RATE=-1`, `TRG_OT_THROTTLE_RATE=10`, `TRG_OT_THROTTLE_RATE=abc` → all fall back to `0.5` default with stderr warning.

**Verification:**
- Setting `TRG_OT_THROTTLE_RATE=0.1` produces measurable 10s+ spacing on cold-cache calls; default behavior is unchanged from today.

---

- U3. **Wire cache + singleflight into RestaurantsAvailability**

**Goal:** Compose U1 + a singleflight group around the existing `RestaurantsAvailability` method. The existing AdaptiveLimiter inside `do429Aware` handles rate limiting; no separate throttle integration needed at this layer. Add the `--no-cache` opt-out at the client API level.

**Requirements:** R1, R5, R6, R8

**Dependencies:** U1 (cache infra). U2 (env-var rate override) is independently landable.

**Files:**
- Modify: `internal/source/opentable/client.go`
- Test: `internal/source/opentable/client_avail_test.go`

**Approach:**
- Add `availSF singleflight.Group` field on `Client` (mirror existing `bootstrapSF`).
- Add `noCache bool` as a positional parameter to `RestaurantsAvailability` signature (decision recorded in Key Technical Decisions). Both call sites in `internal/cli/earliest.go` and `internal/cli/watch.go` update accordingly.
- Wrap the existing `RestaurantsAvailability` body. New flow:
  1. If `noCache=false`, check disk cache. Hit + fresh → return cached entry; record cache-hit on the client (no throttle, no network).
  2. Compute singleflight key `fmt.Sprintf("avail:%d:%s:%s:%d:%d:%d:%t", restID, date, time, party, forwardMinutes, backwardMinutes, noCache)` — same canonical fields as the cache key plus `noCache` so callers with `--no-cache` don't piggyback on cache-allowed leaders' fresh-fetch results unless their flags align.
  3. `availSF.Do(key, func() { ... })` — followers wait for the leader; leader fires the network call (which goes through `do429Aware` → `c.limiter.Wait()` for the existing AdaptiveLimiter pacing).
  4. On success: write cache (regardless of `noCache` per Key Technical Decisions; bypassing reads doesn't bypass writes), return data.
  5. On `BotDetectionError`: return error to followers and leader; the stale-cache fallback in U5 catches it at the caller layer.

**Patterns to follow:**
- `internal/source/opentable/client.go` `Bootstrap()` — singleflight integration pattern (`bootstrapSF.Do("csrf", func() …)`).
- Existing `RestaurantsAvailability` signature — keep backward compatibility on positional args or add a new method `RestaurantsAvailabilityWithOptions(...)`.

**Test scenarios:**
- Happy path: first call → network fires, cache written, AdaptiveLimiter records the request.
- Happy path: second call (same key, within TTL) → cache hit, no network call, AdaptiveLimiter is NOT consulted.
- **Covers AE1.** Edge case: seed a cache entry with `FetchedAt` set to TTL+1m past; call `RestaurantsAvailability` for the same key → cache miss; exactly one network call fires; the new response overwrites the cache entry.
- Happy path: two concurrent goroutines, same key → singleflight coalesces; only one network call fires; both get the same result.
- Happy path: two concurrent goroutines, different keys → both fire (singleflight only dedupes same-key); the AdaptiveLimiter (in `do429Aware`) paces them naturally.
- Edge case: two concurrent goroutines for the same logical request, one with `noCache=true` and one with `noCache=false` → singleflight key differs (per noCache), both fire independent flights.
- Edge case: `noCache=true` → skips cache read, fires network, writes cache anyway.
- Integration: cache write failure (disk full simulated) — call still returns successfully; warning logged.
- Integration: AdaptiveLimiter paces sequential network calls (existing behavior); cache hits skip the limiter entirely (verified by counting limiter.Wait() invocations against network-call counter).

**Verification:**
- A test running 5 concurrent `RestaurantsAvailability` calls for the same `(restID, date, time, party)` records exactly 1 network invocation against a mock transport.

---

- U4. **Extend retry policy — first retry stays in `do429Aware`, second retry scoped to RestaurantsAvailability**

**Goal:** Add a second retry attempt at 5s before surfacing `BotDetectionError`, but ONLY for `RestaurantsAvailability` calls — not for sibling operations like Autocomplete that share `do429Aware` and are out of declared scope. Also upgrade the existing 750ms `time.Sleep` to ctx-aware so the test scenarios for ctx cancellation pass.

**Requirements:** R7, R8

**Dependencies:** None (composable with U3; the second-retry hook lives in U3's wrapper)

**Files:**
- Modify: `internal/source/opentable/client.go` (the existing `do429Aware` 403 branch — upgrade sleep to ctx-aware; the second 5s retry lives in the U3 wrapper around `RestaurantsAvailability`)
- Test: extend `internal/source/opentable/client_avail_test.go` or create `internal/source/opentable/client_retry_test.go`

**Approach:**
- **Audit first.** Before editing, scan `gqlCall` (in `internal/source/opentable/client.go` around the GraphQL POST) for any existing 403 retry behavior layered on top of `do429Aware`'s retry. If a duplicate retry block exists, consolidate: leave retry behavior in `do429Aware` (the lower layer), remove from `gqlCall`. The plan's "~6s worst case" budget assumes a single-source-of-truth retry layer — two layers would multiply attempts and break it. Document the audit outcome in the PR description.
- **Preserve the existing `req.URL.Path != bootstrapPath` guard.** Bootstrap-path 403s continue to flow through the session-wide cooldown branch in `internal/source/opentable/cooldown.go`; this U4 only modifies the non-bootstrap branch.
- **Upgrade the existing 750ms sleep to ctx-aware.** Replace `time.Sleep(750 * time.Millisecond)` with `select { case <-time.After(750 * time.Millisecond): case <-ctx.Done(): return ctx.Err() }`. The current implementation hangs the full 750ms even when ctx is cancelled — the AE4 test scenario "ctx cancellation during sleep returns ctx.Err() promptly" cannot pass without this change.
- **Second retry (5s) lives in the U3 wrapper, NOT in `do429Aware`.** When `do429Aware` returns a `BotDetectionError` for a `RestaurantsAvailability` call, the U3 wrapper sleeps 5s (ctx-aware) and retries the entire call once. If that also returns `BotDetectionError`, surface it. This keeps the second-retry behavior scoped to RestaurantsAvailability — Autocomplete and other ops keep the original single-retry behavior.
- Total worst-case wall time before `BotDetectionError` for `RestaurantsAvailability`: 750ms + 5s = ~5.75s. For Autocomplete or other ops: 750ms (unchanged from today).

**Patterns to follow:**
- The existing single-retry block in `do429Aware`'s 403 branch — same `req.Clone + req.GetBody` choreography, just twice.

**Test scenarios:**
- Happy path: RestaurantsAvailability — server returns 403 once, then 200 → retry succeeds at 750ms (in `do429Aware`); client returns the 200 response.
- Happy path: RestaurantsAvailability — server returns 403 twice, then 200 → second retry succeeds at 5s (in U3 wrapper); client returns the 200 response.
- Error path: RestaurantsAvailability — server returns 403 three times → client returns `BotDetectionError` after ~5.75s wall time.
- Error path: ctx cancellation during 750ms sleep → returns ctx.Err() promptly, not after the full sleep.
- Error path: ctx cancellation during 5s sleep → returns ctx.Err() promptly, not after the full sleep.
- Scope check: Autocomplete with server returning 403 once, then 200 → retry succeeds at 750ms (in `do429Aware`); Autocomplete does NOT pay the 5s second retry.
- Scope check: Autocomplete with server returning 403 twice → returns `BotDetectionError` after ~750ms (single retry); does NOT escalate to a second retry.
- Edge case: `req.GetBody == nil` (rare for our request shape, but possible) → fall back to old behavior (no retry possible) and surface error.
- Verifies that the bootstrap path is NOT affected (still goes to the existing session-cooldown branch).

**Verification:**
- A unit test driving the 403 → 403 → 200 sequence against a mock transport confirms exactly 3 outbound calls, ~5.75s total wall time within tolerance, and a 200 response surfaced.

---

- U5. **Stale-cache fallback on BotDetectionError**

**Goal:** When the network path returns `BotDetectionError` after retries, look up the disk cache for the same key (even if past TTL but within 24h) and return it with `stale: true` markers; fall through to the typed error only if no cache entry exists at all.

**Requirements:** R4

**Dependencies:** U1, U3 (uses cache infra; lives at the seam between client.RestaurantsAvailability and the caller)

**Files:**
- Modify: `internal/source/opentable/client.go` (around the network-call return in the wrapper from U3)
- Modify: `internal/cli/earliest.go` (surface `cached_at` + `stale` fields on the row when present)
- Modify: `internal/cli/earliest.go` (the `earliestRow` struct gets two new optional JSON fields)
- Test: `internal/source/opentable/client_stale_cache_test.go`

**Approach:**
- In the U3 wrapper: on `BotDetectionError` from the network call (after both retries from U4 failed), do a second cache load with the "stale-but-valid" flag. If hit: enrich the cached `[]RestaurantAvailability` with metadata and return as success. If miss: return the original `BotDetectionError`.
- Metadata transport: extend `RestaurantAvailability` with three optional fields:
  - `CachedAt time.Time `json:"cached_at,omitempty"`` — when the entry was originally fetched.
  - `Stale bool `json:"stale,omitempty"`` — true ONLY when the entry is past TTL (i.e., past 3min by default). Independent of how the entry was reached.
  - `Source string `json:"source,omitempty"`` — `"cache_fallback"` when the data came from the BotDetectionError fallback path; empty when fresh-from-network or fresh-cache-hit. Lets JSON consumers distinguish "served from a fresh cache hit" / "served from a within-TTL fallback" / "served from an expired-but-recent fallback" without overloading `stale`.
- `earliestRow` adds `CachedAt string`, `Stale bool`, `Source string` fields, populated when the OT branch's response carries them. Reason field gets a `"(served from cache fallback; data N minutes old)"` suffix when `Source == "cache_fallback"`.

**Patterns to follow:**
- The existing chrome-fallback pattern in `internal/cli/earliest.go` — the OT branch already detects `BotDetectionError` via `opentable.IsBotDetection(err)` and falls back to `c.ChromeAvailability(...)`. Stale-cache fallback should slot in **before** the chrome fallback (cheaper, no headless Chrome spawn).

**Test scenarios:**
- Covers AE3. Happy path: cache hit + network 403 + cache age 2min (TTL is 3m) → stale fallback returns cached data; row shows `source: "cache_fallback"`, `stale: false` (within TTL), `cached_at` set.
- Covers AE3. Happy path: cache hit + network 403 + cache age 12min → stale fallback returns cached data with `source: "cache_fallback"`, `stale: true` (past TTL), and a "served from cache fallback; data 12m old" reason note.
- Edge case: cache hit + network 403 + cache age 25h → returns `BotDetectionError` (cache too stale to trust per 24h hard cap).
- Edge case: no cache + network 403 → returns `BotDetectionError` unchanged (existing behavior preserved); chrome-fallback in earliest.go fires next.
- Edge case: fresh cache hit (no network call) → `source` and `stale` fields are empty/false; `cached_at` may be set for diagnostic but no `cache_fallback` marker.
- Integration: end-to-end via earliest.go — JSON output for a cross-network query (`tock:canlis,opentable:goldfinch`) where OT is blocked but cached: Tock row shows fresh data, OT row shows cached data with `source: "cache_fallback"`. Verify the chrome-fallback is NOT invoked when stale-cache hits (cheaper tier).

**Verification:**
- Forcing a 403 on the network path with a warm cache returns slot data plus the cached-marker row; the chrome fallback is NOT invoked.

---

- U6. **CLI surface: --no-cache flag, env vars, README**

**Goal:** Expose the user-facing knobs and document `HTTPS_PROXY`. Wire the flag through commands that call OT.

**Requirements:** R3, R9, R10, R11

**Dependencies:** U3 (the flag passes through the `RestaurantsAvailability` API)

**Files:**
- Modify: `internal/cli/earliest.go` (add `--no-cache` flag, plumb through to `c.RestaurantsAvailability(...)`)
- Modify: `internal/cli/watch.go` (same — `watch tick` also fires availability)
- No changes needed: `internal/cli/goat.go` does not call `RestaurantsAvailability` (verified in System-Wide Impact)
- Modify: `README.md` — section on OT availability mentions `HTTPS_PROXY` env var and the new `TRG_OT_*` vars
- Modify: `internal/cli/earliest.go` `Long` description of the cobra command — brief mention of `--no-cache` and `HTTPS_PROXY`
- Test: `internal/cli/earliest_test.go` (extend existing tests if present, or create) — flag plumbing for earliest
- Test: `internal/cli/watch_test.go` (extend existing tests if present, or create) — flag plumbing for watch

**Approach:**
- `--no-cache` is a cobra `BoolVar` with default `false` and corresponding env var fallback `TRG_OT_NO_CACHE=1`. Match the flag-binding pattern other flags in earliest.go use.
- Document env vars as a brief table in README under the OpenTable section: `TRG_OT_CACHE_TTL` (default 3m), `TRG_OT_THROTTLE_RATE` (default 0.5 calls/sec; lower for paranoid pacing, higher for private-proxy use), `TRG_OT_NO_CACHE` (default 0), `HTTPS_PROXY` (any standard HTTP proxy URL).
- Add nothing for `HTTPS_PROXY` in code — Go's stdlib already honors it. Just verify no code path overrides it (grep for `Transport`, `DialContext`, custom `RoundTripper` overrides). The Surf client should pass through; verify in code review.

**Patterns to follow:**
- Existing `cmd.Flags().BoolVar(&tonight, "tonight", false, "...")` shape in `internal/cli/earliest.go`.

**Test scenarios:**
- Happy path: `earliest --no-cache` sets the bypass; downstream client receives `noCache=true`.
- Happy path: env var `TRG_OT_NO_CACHE=1` sets the bypass without the flag.
- Happy path: both unset → `noCache=false`.
- Happy path: `watch tick --no-cache` sets the bypass on the watch path; downstream client receives `noCache=true`.
- Edge case: `TRG_OT_NO_CACHE=` (empty) → `noCache=false`.

**Verification:**
- `./table-reservation-goat-pp-cli earliest --help` output mentions `--no-cache`.
- `./table-reservation-goat-pp-cli watch tick --help` output mentions `--no-cache`.
- README has a clear "Power-user knobs" section listing all four env vars.

---

- U7. **PATCH manifest + dogfood pass**

**Goal:** Update `.printing-press-patches.json` to record this work, and re-run a representative dogfood query end-to-end to confirm the resilience envelope holds together.

**Requirements:** All (verification gate)

**Dependencies:** U1–U6

**Files:**
- Modify: `.printing-press-patches.json` (extend the `cross-network-source-clients` patch's `validated_outcome` with a v0.1.14 entry summarizing the cache + AdaptiveLimiter-rate-config + retry + stale-fallback)
- No code files

**Approach:**
- Append a v0.1.14 entry to `validated_outcome` describing the cache layout, AdaptiveLimiter reuse, retry sequence, and stale-fallback. Mention the new env vars (TRG_OT_CACHE_TTL / TRG_OT_THROTTLE_RATE / TRG_OT_NO_CACHE) and the `HTTPS_PROXY` documentation note.
- Add `internal/source/opentable/avail_cache.go` to the patch's `files` list.
- **Live dogfood (cache + retry + fallback paths):**
  1. Clear caches: `rm -rf ~/Library/Caches/table-reservation-goat-pp-cli/ot-avail/`.
  2. Run `earliest 'opentable:goldfinch tavern at four seasons' --party 4 --date 2026-05-09 --within 2d`. Verify slot times return AND cache files appear under `ot-avail/`. Cold wall time should be on the order of a few seconds (2-3 calls × existing 2s AdaptiveLimiter spacing).
  3. Re-run the same query immediately. Verify second run returns identical data in well under 1 second (cache hit; no network call).
  4. Re-run with `--no-cache`. Verify fresh network call fires.
  5. Wait until past TTL (3min), then re-run. Verify a new network call fires and cache is refreshed (covers AE1).
  6. **Stale-fallback test:** seed a cache entry, then induce Akamai 403 (rapid-fire calls until escalation). Verify `earliest` returns the cached data with `source: "cache_fallback"` rather than the chrome-fallback path. Confirm the AdaptiveLimiter halves its rate after the 403 (visible in subsequent calls' wall time).
  7. **Cache-size sanity:** after the dogfood, run `du -sh ~/Library/Caches/table-reservation-goat-pp-cli/ot-avail/`. Paste the number into the PR description so reviewers can validate the "~1-2KB per entry" estimate from System-Wide Impact.

**Test scenarios:**
- Test expectation: none — manual dogfood, not unit tests.

**Verification:**
- Patches manifest mentions v0.1.14 with all the new behavior described.
- Dogfood transcript pasted into the PR description shows: first call cold-fetch, second call cache-hit (timed < 100ms), `--no-cache` re-fetches, expired-cache re-fetches.

---

## System-Wide Impact

- **Interaction graph:** `RestaurantsAvailability` is called from `internal/cli/earliest.go` and `internal/cli/watch.go`. Both must accept the new `--no-cache` flag (or skip if the cobra command doesn't expose it). `goat.go` doesn't call this op directly. Bootstrap, Autocomplete, and ChromeAvailability paths are unaffected — the wrapper sits only on `RestaurantsAvailability`.
- **Error propagation:** `BotDetectionError` continues to be the typed signal; new behavior = it can now be silently absorbed when a stale-cache fallback succeeds. Callers using `opentable.IsBotDetection(err)` see the error less often.
- **State lifecycle risks:** Cache files accumulate over time. Mitigation: TTL gate prevents reads of stale entries; 24-hour hard cap prevents stale-fallback abuse. No automated cache eviction in v1 (`du -sh` will be small even after months — ~1-2KB per entry, capped by venue+date+party diversity).
- **API surface parity:** `--no-cache` should be added to any command that ultimately fires OT availability. Watch out: `goat` doesn't, `earliest` does, `watch tick` does. Verify by grepping for `RestaurantsAvailability(` in the cli package before finalizing U6.
- **Integration coverage:** The combination of singleflight + cache + retry + stale-fallback (with the existing AdaptiveLimiter underneath) is most fragile at the seams. U3 must include a multi-goroutine-same-key test AND a multi-goroutine-different-key test to catch races between singleflight followers and the network layer.
- **Unchanged invariants:** `gqlCall`, `Bootstrap`, `Autocomplete`, `ChromeAvailability`, the persisted-query hash, the request body shape, the GraphQL response shape — all unchanged. The chrome fallback path in `earliest.go` remains as a tier below stale-cache (cache-hit > stale-cache > chrome-attach > chrome-spawn > URL-only).

---

## Risks & Dependencies

| Risk | Mitigation |
|---|---|
| Cache files survive a persisted-query hash rotation and serve incompatible data | Cache entries embed both the hash AND a `SchemaVersion` integer at write time; reads with mismatched hash OR schema-version return cache miss. |
| AdaptiveLimiter starts too aggressive and triggers WAF on cold cache | Default initial rate (0.5/s = 2s spacing) is the existing baseline already in use; the AdaptiveLimiter halves to 1s on rate-limit signals. If real-world experience shows escalation at 0.5/s, lower the default to 0.2/s (5s spacing) — a one-line change that doesn't restructure the plan. Power users with private proxies can raise it via `TRG_OT_THROTTLE_RATE`. |
| AdaptiveLimiter ramp-up never recovers after a single 403 | The existing `OnSuccess` codepath fires after every successful HTTP call; ramp-up after 10 consecutive successes is built in. As long as cached responses don't disable `OnSuccess` calls (verify in U3 — the limiter only fires for actual network calls, so cache hits and singleflight followers don't reset the success counter), the recovery path works. |
| Stale-cache fallback masks a real upstream outage | 24-hour hard cap on stale fallback; row carries `source: "cache_fallback"` plus `stale` + `cached_at` so JSON consumers can detect and surface the staleness. |
| Cached slot tokens may exceed their lifetime and fail at booking | Default TTL reduced to 3min to stay under OT's documented "~minutes" slot-token lifetime. Stronger session-fingerprint defense deferred to v1.1 if booking failures appear. |
| Singleflight key collision between unrelated calls | Key includes `restID, date, time, party, forwardMinutes, backwardMinutes, noCache` — different windows or different cache-bypass intent route to different flights. |
| Retry sleeps hang past ctx-cancellation | All sleeps in U2, U3, U4 use ctx-aware `select { case <-time.After(...): case <-ctx.Done(): }`. The existing 750ms `time.Sleep` in `do429Aware` is upgraded as part of U4. |
| U4 retry change accidentally widens Autocomplete or Bootstrap retry behavior | Second retry (5s) lives only in U3's RestaurantsAvailability wrapper, NOT in shared `do429Aware`. Bootstrap-path guard (`req.URL.Path != bootstrapPath`) preserved. Test scenarios in U4 explicitly verify Autocomplete keeps its single-retry shape. |
| Existing `gqlCall` may have a duplicate 403-retry layer that compounds with U4 | U4 includes an explicit audit step before editing. If a duplicate layer exists, consolidate retries in `do429Aware` (the lower layer); document the audit outcome in the PR. |
| Path-traversal via user-influenced `--date`/`--time` flags in cache filenames | Cache filenames are SHA-256 hashes of canonicalized keys, not raw inputs. Date and time inputs are validated against `^\d{4}-\d{2}-\d{2}$` and `^\d{2}:\d{2}$` regexes before key construction. |
| `os.UserCacheDir()` differs across platforms (Linux XDG, macOS `~/Library/Caches`, Windows `%LOCALAPPDATA%`) | Follow `auth/chrome.go` (uses `os.UserCacheDir()` correctly); avoid the `cooldown.go` hardcoded `~/.cache/...` pattern, which is XDG-only and broken on macOS/Windows. |
| Env var prefix `TRG_OT_*` may collide with future CLI features | New convention; document in README. If future features need a registry, can be added at that point. |

---

## Documentation / Operational Notes

- README "Power-user knobs" section: list `TRG_OT_CACHE_TTL`, `TRG_OT_THROTTLE_RATE`, `TRG_OT_NO_CACHE`, `HTTPS_PROXY`. One sentence per knob.
- `earliest --help` mentions `--no-cache` flag and references the README for env vars.
- Patches manifest v0.1.14 entry under `cross-network-source-clients`.
- No monitoring or rollout coordination needed — this is a printed CLI; users update by re-installing.

---

## Sources & References

- **Origin document:** `docs/brainstorms/2026-05-09-ot-akamai-waf-resilience-requirements.md`
- Disk-cache pattern: `internal/source/auth/chrome.go` (`loadAkamaiCacheRaw`, `saveAkamaiCache`, `akamaiCachePath`)
- Cooldown / typed error pattern: `internal/source/opentable/cooldown.go`
- Singleflight integration: `internal/source/opentable/client.go` `Bootstrap()`
- TOCTOU-safe rate-limit pattern: `internal/cliutil/ratelimit.go` `AdaptiveLimiter.Wait()`
- Existing 403 retry path: `internal/source/opentable/client.go` `do429Aware` (current single 750ms retry; U4 extends to two attempts)
- Current chrome fallback (sits below stale-cache fallback in the tier order): `internal/source/opentable/chrome_avail.go`, `internal/cli/earliest.go`
