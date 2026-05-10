---
date: 2026-05-09
topic: ot-akamai-waf-resilience
---

# OpenTable Akamai WAF Resilience

## Summary

A self-contained CLI fix for OpenTable's probabilistic Akamai WAF: cache successful `RestaurantsAvailability` responses on disk, singleflight-dedupe concurrent calls for the same key, throttle to a 30-second per-process floor between calls, extend the retry to a second attempt with longer backoff, and document `HTTPS_PROXY` as a power-user escape hatch. Goal: a user running 1-2 queries every few minutes never sees the WAF escalate against them.

---

## Problem Frame

OpenTable's `/dapi/fe/gql?optype=query&opname=RestaurantsAvailability` endpoint sits behind an Akamai WAF rule that scores requests by IP and session context. The rule is probabilistic — sometimes blocks, sometimes lets through — and rapid-fire calls escalate the score until the session is hard-blocked for several minutes.

The CLI hits this aggressively in two patterns:
1. **Multi-venue queries** — `earliest 'opentable:a,opentable:b,opentable:c'` fires N parallel calls, each contributing to the score
2. **Multi-day windows** — the post-May-2026 GraphQL gateway is single-day-only (`forwardDays: 0`), so a `--within 7d` scan loops 7 calls back-to-back

A real user running `earliest` once now and again 5 minutes later is fine; the same user running `earliest` for 10 venues across a 7-day window in one shot is not. The CLI today has a single 750ms retry inside `do429Aware`, which catches one transient flake but compounds session reputation when the WAF has already escalated.

The cost is concrete: when escalation kicks in, every OT row in cross-network commands degrades to a "venue exists, book directly at <URL>" message until Akamai's score cools (typically 5-15 min). Tock-side calls and OT slug→ID resolution stay healthy throughout — only `RestaurantsAvailability` is gated. The fix should be invisible at human pace and graceful when escalation does happen.

---

## Requirements

**Cache layer**

- R1. The CLI caches successful `RestaurantsAvailability` responses on disk under a per-user cache directory, keyed by `(restaurantID, date, time, partySize)`. The cache is read before the network call and written on a successful response.
- R2. Cached entries are valid for a TTL (default 10 minutes). Reads after the TTL miss the cache and fire a fresh network call.
- R3. The CLI exposes an opt-out flag (e.g., `--no-cache`) and a corresponding env var so users who want guaranteed-fresh data can bypass the cache. The flag bypasses the read but still writes the response on success.
- R4. When the network call returns a typed `BotDetectionError` AND a stale-but-valid (not-yet-corrupted) cached entry exists for the same key, the CLI returns the stale entry rather than the error. The response carries a marker (e.g., `cached_at` + `stale: true` field in the row) so downstream JSON consumers can see the data is older than TTL.

**Concurrency control**

- R5. Concurrent in-process calls for the same `(restID, date, time, partySize)` key are coalesced via singleflight: the first caller fires the network request and subsequent callers wait for and receive the same response.
- R6. Outbound `RestaurantsAvailability` calls go through an adaptive rate limiter (existing `cliutil.AdaptiveLimiter` already attached to the OT client) — 2s baseline, ramps up after success streaks, halves on rate-limit signals. No new fixed throttle floor. The initial rate is overridable via env var so paranoid users can configure aggressive spacing without code changes. Cached or singleflighted-coalesced reads bypass the limiter entirely.

**Retry policy**

- R7. On a 403 response from the OT GraphQL endpoint (operation-specific WAF rule), the client retries up to two additional times with backoff: first retry at 750ms, second retry at 5 seconds. After the second retry fails, the call returns a `BotDetectionError` (which then triggers the stale-cache fallback in R4).
- R8. Retries do not contribute to the throttle floor in R6 — the floor governs distinct "logical" calls, not retry attempts within one logical call.

**Escape hatch**

- R9. The CLI honors the standard `HTTPS_PROXY` / `HTTP_PROXY` environment variables (Go's `net/http` honors these by default; this requirement is to ensure no code path overrides them and to document the behavior).
- R10. The README and the `--help` output for affected commands document `HTTPS_PROXY` as a supported escape hatch for users who want to route OT calls through a personal proxy or Tor SOCKS5 endpoint.

**Configurability**

- R11. The AdaptiveLimiter initial rate (R6) and cache TTL (R2) are overridable via env vars. Defaults are documented; values outside reasonable ranges are clamped or rejected with a clear error.

---

## Acceptance Examples

- AE1. **Covers R1, R2, R5.** Given the cache is empty, when the user runs `earliest 'opentable:goldfinch tavern' --party 4 --date 2026-05-09 --within 1d` and the network returns slot data, the response includes the slot times AND a cache file is written. When the same command is re-run within 10 minutes, the response is identical AND no network call fires (verified via debug logging or a network-call counter). After 10 minutes, the next run fires a fresh network call.
- AE2. **Covers R6.** Given a 7-day multi-venue query (`earliest 'opentable:a,opentable:b' --within 7d`) on a cold cache while Akamai is healthy, all 14 OT GraphQL calls go through the existing AdaptiveLimiter at its 2s baseline. Total wall time is on the order of ~30s, not several minutes. If Akamai 403s mid-run, the AdaptiveLimiter halves its rate (4s, then 8s, then 16s) and recovers on success streaks; latency increases only when the upstream pushes back, not preemptively.
- AE3. **Covers R4.** Given a cached entry exists for `(restID=25606, date=2026-05-09, time=19:00, party=4)` written 8 minutes ago, when the user re-runs the same query and the network returns 403 (WAF block), the CLI returns the cached slot data with `stale: false` (still within TTL) and surfaces the cache hit in the row's reason. If the cached entry is 12 minutes old (past TTL), the CLI returns the cached data with `stale: true` and a reason note that the network is currently blocked.
- AE4. **Covers R7.** Given the OT GraphQL endpoint returns 403 for a fresh (uncached) call, the client retries at 750ms (still 403), retries again at 5s (still 403), then returns `BotDetectionError`. Total wall time is ~6 seconds, not 30+ seconds.
- AE5. **Covers R3.** Given a cached entry exists, when the user runs the same command with `--no-cache`, the CLI skips the cache read and fires a fresh network call. The fresh response overwrites the cache entry (or writes a new entry if the response succeeded).
- AE6. **Covers R9, R10.** Given `HTTPS_PROXY=socks5://localhost:9050` is set in the environment, when the user runs `earliest 'opentable:goldfinch'`, the OT GraphQL call routes through the SOCKS5 proxy. The CLI does not override or strip the env var.

---

## Success Criteria

- A user running `earliest 'opentable:<slug>' --party N --date <today> --within 2d` once now and again 5 minutes later sees real slot data both times, no escalation, ~instant on the second call (cache hit).
- A user running `earliest 'opentable:a,opentable:b,opentable:c' --within 7d` (21 cold-cache calls) completes successfully — slow (~10 min wall time due to throttle) but not 403'd into a cooldown.
- When Akamai DOES escalate against the user (e.g., during a busy session of testing) and the cache is warm, the user gets data anyway (R4 stale fallback) with a clear marker that it's cached.
- The fix does not require any new external dependency, paid service, or background process.
- A downstream agent (planner or implementer) can read this doc and know exactly which file owns each mechanism without inventing the cache key shape, throttle scope, or retry timing.

---

## Scope Boundaries

- TLS fingerprint rotation between Chrome and Firefox impersonation (Approach B in the brainstorm). Rejected because the WAF rule appears keyed on (IP, session, opname) — same Chrome browser keeps working from the same IP, so JA3 alone isn't the gate.
- Long-running daemon or background process to keep a Chrome session warm. Rejected because the CLI's identity is "self-contained executable"; daemons add lifecycle and state-directory burden the user explicitly opted out of.
- Auto-rotating free proxy lists or bundled Tor. Rejected because most public proxies are flagged by Akamai's IP intelligence and the CLI shouldn't ship questionable IP rotation by default.
- Caching of `Autocomplete`, `Bootstrap`, or any other OT operation. Out of scope — only `RestaurantsAvailability` is gated by the WAF rule and only that endpoint warrants the cache layer's complexity.
- Caching or throttling of any Tock-side calls. Tock has no equivalent WAF rule and works without these mechanisms.
- Implementing a "smart concurrency" model that learns the WAF's behavior over time. Out of scope; static throttle + cache is sufficient.
- A persistent on-disk cooldown that survives across CLI invocations specifically for the per-call retry path. The existing session-wide cooldown (cooldown.go) covers Bootstrap-path 403s; per-call retry remains in-process.

---

## Key Decisions

- **Cache TTL = 10 minutes by default.** Long enough that "tonight or tomorrow" queries fired a few minutes apart hit the cache; short enough that humans don't perceive stale data as wrong. Configurable.
- **Throttle floor = 30 seconds by default, per-process not persisted.** A fresh CLI invocation gets a clean budget; a user who runs the CLI once and waits 5 minutes never feels it. Inside a single invocation (e.g., a multi-venue query), the floor enforces human-pace spacing.
- **Stale cache trumps fresh 403.** When the network is blocked but a cached entry exists, return the cached entry with a `stale` marker. Better than failing — the slot times don't change minute-to-minute, and a stale answer is more useful than no answer.
- **Retry at 750ms then 5s, then surface BotDetectionError.** Two attempts catch transient WAF flakes without compounding score; total wall time bounded at ~6s; users still get the stale-cache safety net beyond that.
- **`HTTPS_PROXY` as documentation, not implementation.** Go's stdlib already honors it. The decision is just to not break it, and to document the affordance for power users.

---

## Dependencies / Assumptions

- The existing `BotDetectionError` typed error in `internal/source/opentable/cooldown.go` is the right signal to trigger stale-cache fallback. Verified.
- The CLI already uses `golang.org/x/sync/singleflight` for `Bootstrap()` deduplication. Reusing the same package for `RestaurantsAvailability` is consistent. Verified.
- Cache directory location follows the same pattern as the existing OT cooldown file (`~/.cache/table-reservation-goat-pp-cli/...` via `os.UserCacheDir()`). Verified by reading `internal/source/opentable/cooldown.go`.
- Disk write durability for the cache is best-effort: a corrupt cache file should fall through to the network path, not crash the CLI. The existing cooldown file already follows this pattern.
- The `RestaurantsAvailability` response shape is stable enough to cache as JSON for 10 minutes. The schema rotated in May 2026 (forced the recent `cbcf4838…` hash refresh), so cache entries from before a rotation must be invalidated; cache the response keyed by the persisted-query hash version OR by a schema fingerprint to make this safe across releases.

---

## Outstanding Questions

### Deferred to Planning

- [Affects R1, R2][Technical] How exactly should the cache file layout look on disk? One JSON-per-key file, a single SQLite db, or a single JSON file with a map? Implementation choice; planner should pick based on file-count vs read-cost trade-off.
- [Affects R6][Technical] Where should the throttle live — at the `gqlCall` level, the `do429Aware` level, or a new `RestaurantsAvailability`-specific wrapper? Likely the wrapper, but planner should validate against existing rate-limiter integration.
- [Affects R11][User decision deferred to planning] Exact env var names — there's no project convention yet for OT-specific config. Planner should pick names consistent with existing `TABLE_RESERVATION_GOAT_OT_CHROME_DEBUG_URL`-style prefixes.
- [Affects R4][Technical] How is "stale-but-valid" defined precisely — any cache file regardless of age, or only entries within 24 hours? Likely the latter (a week-old cached slot is genuinely stale) but planner should validate.
