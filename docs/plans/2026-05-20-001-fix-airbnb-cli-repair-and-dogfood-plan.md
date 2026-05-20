---
title: Airbnb CLI end-to-end repair + adaptive rate-limit + dogfood matrix
type: fix
status: active
created: 2026-05-20
depth: deep
related_pr: https://github.com/mvanhorn/printing-press-library/pull/712
target_repo: mvanhorn/printing-press-library
target_cli: library/travel/airbnb
---

# Airbnb CLI end-to-end repair + adaptive rate-limit + dogfood matrix

**Target repo:** `mvanhorn/printing-press-library` (paths below are repo-relative)
**Target CLI:** `library/travel/airbnb/` (binary name `airbnb-pp-cli`)
**Related work:** PR #712 (open) ships F1 (SSR extractor) + F2 (X-Airbnb-API-Key scrape). This plan picks up F3 and everything past it.

---

## Summary

The airbnb-pp-cli does not currently return real Airbnb price data end-to-end. PR #712 fixed the SSR search extractor and the rejected API key, but immediately surfaced a third bug: the `StaysPdpBookItQuery` persisted-query hash baked into source is stale, so the pricing GraphQL call now returns ValidationError. Two adjacent hashes (`wishlistIndexHash`, `wishlistItemsHash`) sit in the same source file under the same time-bomb. Underneath that, repo research surfaced a structural gap the user did not know about: the `--rate-limit` flag is wired to one client, but the actual Airbnb scrape goes through a separate package-global `defaultClient` that never sees the flag and runs at a hard-coded 0.5 rps. There is also no 403 or datadome detection anywhere in the airbnb source path, so bot challenges silently degrade into generic 4xx errors with truncated bodies.

This plan does four things in sequence:

1. Move all three persisted-query hashes to runtime-scrape with constant fallback, mirroring the exact pattern PR #712 used for the API key.
2. Plumb `--rate-limit` from cobra root flags through to the actual scrape client so the flag does what the docs say.
3. Add adaptive backoff (jitter on top of `cliutil.Backoff`) and a `BotChallengeError` sentinel that detects datadome and Akamai signatures, so the limiter knows when to slow down and when to surface a remediation hint instead of retrying blindly.
4. Build a live-network dogfood matrix that exercises every documented command + auxiliary surface, captures pass/fail per command into a structured artifact, and gates the CLI's continued presence in the library on that evidence.

If the dogfood matrix fails to clear the strict bar (all in-scope commands return real data AND the rate-limit guard demonstrably fires under hammering), the plan branches to a deletion path: the CLI is removed from the library rather than left in a broken-but-published state.

---

## Problem Frame

**Who is affected:** Agents and users invoking `airbnb-pp-cli` for the headline arbitrage path (`cheapest`, `plan`, `compare`, `watch`) and for the authenticated wishlist surfaces (`wishlist list`, `wishlist items`, `wishlist diff`). Today every one of these returns null prices or a ValidationError. The CLI also exposes a `--rate-limit` flag that does nothing on the real traffic path, which will produce silent 403s and IP blocks when users follow the docs.

**Current state of the failure modes:**

| Surface | Current state | Root cause |
|---|---|---|
| `airbnb-listing search` | Fixed in PR #712 (not yet merged) | SSR extractor regex updated |
| `airbnb-listing get` price | ValidationError | Stale bookItHash persisted-query hash |
| `airbnb-listing get` everything else | Works | n/a |
| `airbnb_wishlist list` | At-risk (hash rotation imminent) | Stale wishlistIndexHash |
| `airbnb_wishlist items` | At-risk | Stale wishlistItemsHash |
| `cheapest` / `plan` / `compare` / `watch` | Broken | All depend on `get` price |
| `host extract` / `find-twin` / `fingerprint` | Untested in production | Unknown |
| `--rate-limit` flag on scrape | No-op | Wired to wrong client |
| 403 / datadome challenge | Surfaced as raw HTTP-status string | No typed detection |
| `match` (VRBO branch) | ErrDisabled by design | Akamai workaround deferred |

**Why fix vs delete:** the CLI's headline value is host-direct arbitrage (`cheapest`). Without working price quotes, none of the unique-capability commands deliver value. Either we restore that path and prove the CLI works, or it gets removed from the library to keep the catalog honest.

---

## Scope Boundaries

### In scope

- Replace three baked-in GraphQL persisted-query hashes with runtime scrape + constant fallback (`bookItHash`, `wishlistIndexHash`, `wishlistItemsHash`)
- Add `SetRate(rps float64)` to configure the package-global `defaultClient` in `internal/source/airbnb/client.go` and call it from `rootCmd.PersistentPreRunE` in `internal/cli/root.go` so `--rate-limit` actually applies to every scrape and GraphQL call
- Add jittered exponential backoff on top of `cliutil.Backoff` for the retry loop
- Introduce `BotChallengeError` typed sentinel and detect datadome + Akamai signatures in the airbnb response path
- Build a Go integration test (`internal/dogfood/dogfood_live_test.go`) gated on `AIRBNB_PP_DOGFOOD_LIVE=1` that exercises every documented command and writes a `dogfood-results.json` artifact
- Execute the dogfood matrix end-to-end and gate the CLI's continued presence in the library on the pass/fail bar
- Update `SKILL.md`, `README.md`, and `.printing-press-patches.json` to reflect the new behavior

### Deferred to Follow-Up Work

- VRBO re-enable (still Akamai-blocked at the cookie warmup tier, separate workaround landing path)
- Fundamental refactor to consolidate `internal/source/airbnb` and `internal/client` into one client surface (the present plan keeps both but plumbs the flag correctly)
- Persisted-query hash auto-discovery via response capture (current scope: scrape from SSR HTML at startup, not from network observation)
- Headed-browser fallback when SSR scrape itself gets datadome-challenged (the operational note documents the escape hatch but does not implement it here)

### Outside this product's identity

- The CLI does not become a "browser" or a JS execution environment. Tier escalation to chromedp / Chrome MCP is recorded as the next-tier remediation when bot defenses harden further, not built into this CLI.
- The CLI does not synthesize fake data. When backoff exhausts retries, it surfaces a typed error pointing the user at remediation (refresh cookies, slow down, try later), never a fabricated fallback. This is a deliberate carry-forward from the 2026-05-03 quarantine plan.

---

## Requirements

| ID | Requirement | Source |
|---|---|---|
| R1 | `airbnb-listing get <id> --checkin X --checkout Y --adults N` returns a real numeric `price_breakdown.total` for at least one known-good listing ID | User: "pull up Airbnb results" |
| R2 | `cheapest <airbnb-url>` returns a comparable struct (OTA total + direct total + savings) for at least one known-good listing | Skill doc unique capabilities |
| R3 | `--rate-limit 0.5` (or any user-supplied float) demonstrably caps the actual scrape request rate when observed on the wire | User: "build in the rate limit stuff" |
| R4 | When Airbnb returns a 429 or a datadome challenge, the CLI backs off (delay > 0, increasing with attempt) instead of retrying immediately or surfacing a raw HTTP-status string | User: "so it knows not to be too aggressive" |
| R5 | Every documented command in `SKILL.md` plus every auxiliary surface (`doctor`, `agent-context`, `profile`, `--deliver`, `--data-source`, `--dry-run`) has a pass/fail record in `dogfood-results.json` after the matrix runs | User: "dog food the crap out of this" |
| R6 | If `cheapest` (the headline value prop) fails to return real data on a known-good listing AND the failure is rooted in the work this plan covers (not a transient bot challenge or an unrelated side-surface regression), the plan executes the deletion path. Failure of other Tier-1 commands (search, get, plan, compare, host extract) is recorded as a Tier-2 finding and does NOT gate deletion. | User: "prove that this thing works, or we need to delete it from the library"; P-2/P-4 doc review |
| R7 | All three persisted-query hashes survive a future Airbnb hash rotation without source-code change (runtime scrape with constant fallback that logs a warning when fallback fires) | Repo research: same time-bomb in 3 places |
| R8 | Rate-limit semantics are documented in SKILL.md so users know the flag applies to scrape traffic, not just the (mostly-unused) generated client path | Repo research: silent gap today |

---

## Key Technical Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Hash discovery pattern | Mirror PR #712 verbatim: per-hash `sync.Once` + regex + pure `parseXHash([]byte) string` extractor + `resolveXHash(ctx) string` integration + constant fallback | The pattern just shipped in this same source file. Forking it would be needless variation. |
| SSR scrape target for hash extraction | `airbnb.com/s/` search results page (the same page already scraped for the API key in PR #712) | Anonymous-accessible, already in the scrape allowlist, contains all three operation hashes in its inlined Apollo Niobe state. Wishlist page requires auth and adds a dependency. |
| Rate-limit plumbing | Add `func SetRate(rps float64)` to `source/airbnb` package and call from `rootCmd.PersistentPreRunE`. Keep `defaultClient` as the singleton; do not refactor every call site. | 10-line patch vs ~30-file refactor. Alternative considered + rejected below. |
| Jitter formula | `cliutil.Backoff(attempt) + time.Duration(rand.Int63n(int64(cliutil.Backoff(attempt)/4)))` (25% jitter on top of bare exponential) | Template comment in `cliutil/ratelimit.go` explicitly invites callers to add jitter. 25% is conventional and small enough to preserve the Backoff cap semantics. |
| Bot-challenge sentinel | New `BotChallengeError struct { ChallengeType string; Remediation string }` in `internal/cliutil/errors.go` alongside `RateLimitError`. (Confirmed by repo research: `RateLimitError` already lives in cliutil; no shared-template concern.) | Symmetry with `RateLimitError`. Lets callers `errors.As` and surface the remediation hint in CLI output without coupling to detection logic. |
| Bot-challenge detection signatures | datadome: `set-cookie: datadome=*`, `server: dd-*`, response body contains `"url":"https://geo.captcha-delivery.com/captcha"` or `Please enable JS and disable any ad blocker`. Akamai: title contains `bot or not`, body contains `captcha-pwa` (matches existing VRBO `isBotChallenge` in `internal/source/vrbo/extract.go:206`) | Documented signatures from datadome and Akamai public docs. The VRBO match keeps the two source/<vendor> packages internally consistent. |
| Dogfood matrix shape | Go integration test in `internal/dogfood/dogfood_live_test.go`, gated on `AIRBNB_PP_DOGFOOD_LIVE=1`. Per-command subtests, each captures exit code + stdout shape + stderr summary, writes to `dogfood-results.json` matrix entries. | Repeatable, in-repo, gated so CI does not hit Airbnb. Mirrors the existing `dogfood-results.json` schema convention. |
| Proof bar enforcement | Tier-1 commands (search, get, cheapest, plan, compare, host extract) must return real data. Rate-limit guard must demonstrably fire under hammering (observable by per-request timestamps in the test log). Failure of any Tier-1 triggers the deletion fork. | User-confirmed strict bar at Phase 0.7. |
| Hash regex location | Per-hash extractor lives in `internal/source/airbnb/graphql.go` next to the hash constants it replaces. Tests in `internal/source/airbnb/graphql_test.go`. | Co-locates the runtime-scrape logic with the constants it supersedes. Keeps the diff narrow. |

---

## High-Level Technical Design

This section communicates the intended shape of the runtime hash-resolution path, the rate-limit plumbing, and the bot-challenge detection branching. It is directional guidance for review, not implementation specification.

### Request lifecycle after fixes

```
┌──────────────────────────────────────────────────────────────────────────┐
│ rootCmd.PersistentPreRunE                                                │
│   reads --rate-limit float                                               │
│   calls source/airbnb.SetRate(rate)                                      │
└──────────────────────────────────────────────────────────────────────────┘
                                  │
                                  ▼
┌──────────────────────────────────────────────────────────────────────────┐
│ source/airbnb.defaultClient.do(req)                                      │
│   limiter.Wait()                  ◄──  rate cap (was hardcoded 0.5)      │
│   resp = httpClient.Do(req)                                              │
│                                                                          │
│   if resp.StatusCode == 429:                                             │
│     limiter.OnRateLimit()                                                │
│     sleep cliutil.RetryAfter(resp) + jitter   ◄── NEW jitter             │
│     retry (up to 3 attempts)                                             │
│                                                                          │
│   if isBotChallenge(resp, body):  ◄── NEW detector                       │
│     limiter.OnRateLimit()  (treat like 429 for rate-limit ramp)          │
│     return BotChallengeError{type, remediation}                          │
│                                                                          │
│   if resp.StatusCode >= 400: surface error (existing behavior)           │
│   limiter.OnSuccess()                                                    │
│   return body                                                            │
└──────────────────────────────────────────────────────────────────────────┘
                                  │
                                  ▼
┌──────────────────────────────────────────────────────────────────────────┐
│ source/airbnb.BookingPrice(ctx, listingID, ...)                          │
│   hash := resolveBookItHash(ctx)   ◄── NEW: sync.Once scrape + fallback  │
│   url := airbnbBase + "/api/v3/StaysPdpBookItQuery/" + hash              │
│   call graphQLGet(ctx, url, ...)                                         │
└──────────────────────────────────────────────────────────────────────────┘
```

### Hash discovery pattern (per hash)

Each of `bookItHash`, `wishlistIndexHash`, `wishlistItemsHash` follows the exact same shape that PR #712 used for the API key:

```
var (
    bookItHashOnce  sync.Once
    bookItHashValue string
    bookItHashRe    = regexp.MustCompile(`StaysPdpBookItQuery[^"]*","persistedQueryName":"[^"]+","sha256Hash":"([a-f0-9]{64})"`)
)

const bookItHashFallback = "<current-hash-here>"

func parseBookItHash(html []byte) string {
    m := bookItHashRe.FindSubmatch(html)
    if len(m) < 2 {
        return ""
    }
    return string(m[1])
}

func resolveBookItHash(ctx context.Context) string {
    bookItHashOnce.Do(func() {
        body, err := defaultClient.fetchHomepage(ctx)
        if err != nil {
            log.Printf("airbnb: hash discovery failed, using fallback: %v", err)
            bookItHashValue = bookItHashFallback
            return
        }
        h := parseBookItHash(body)
        if h == "" {
            bookItHashValue = bookItHashFallback
            return
        }
        bookItHashValue = h
    })
    return bookItHashValue
}
```

Regex variants for the other two hashes follow the same shape, anchored on `WishlistIndexPageQuery` and `WishlistItemsAsyncQuery`. Directional only.

### Bot-challenge detection decision matrix

| Signal in response | Classification | Action |
|---|---|---|
| `status == 429` | RateLimitError (existing) | `limiter.OnRateLimit()` + `Backoff(attempt) + jitter` + retry |
| `set-cookie: datadome=` present | BotChallengeError{datadome, "wait, refresh cookies, or slow down"} | `limiter.OnRateLimit()` + return sentinel (no retry) |
| `server: dd-*` header | BotChallengeError{datadome, ...} | same |
| body contains `geo.captcha-delivery.com` | BotChallengeError{datadome, ...} | same |
| `<title>` contains "bot or not" | BotChallengeError{akamai, "warmup + slow down"} | same |
| body contains `captcha-pwa` | BotChallengeError{akamai, ...} | same |
| `status >= 400` and no match above | generic error (existing) | surface error |
| `status == 200` | success | `limiter.OnSuccess()` + return body |

---

## Output Structure

No new directory hierarchy. All changes live under existing paths:

```
library/travel/airbnb/
├── internal/
│   ├── cli/
│   │   └── root.go                          # MODIFY: pre-run wires --rate-limit to source/airbnb.SetRate()
│   ├── cliutil/
│   │   ├── ratelimit.go                     # KEEP: primitives reused; comment update only
│   │   └── errors.go                        # MODIFY: add BotChallengeError sentinel (if cliutil is local; else add to source/airbnb/errors.go)
│   ├── dogfood/                             # NEW
│   │   ├── dogfood_live_test.go             # NEW: live-network matrix gated on env var
│   │   ├── fixtures.go                      # NEW: known-good listing ID + dates per command
│   │   └── README.md                        # NEW: how to run the matrix, how to interpret results
│   └── source/
│       └── airbnb/
│           ├── client.go                    # MODIFY: SetRate(); bot-challenge detect in do(); jitter in retry loop
│           ├── graphql.go                   # MODIFY: replace 3 hash consts with sync.Once + scrape + fallback
│           ├── graphql_test.go              # MODIFY: tests for parseBookItHash, parseWishlistIndexHash, parseWishlistItemsHash
│           ├── extract.go                   # NO CHANGE expected
│           └── client_test.go               # MODIFY: tests for SetRate(), isBotChallenge(), jitter bounded
├── SKILL.md                                 # MODIFY: rate-limit docs, BotChallengeError exit semantics, dogfood matrix mention
├── README.md                                # MODIFY: rate-limit + bot-challenge notes
├── dogfood-results.json                     # OVERWRITE during U7
└── .printing-press-patches.json             # APPEND: amend entries for U2, U3, U4, U5
```

---

## Implementation Units

### U1. Coordinate with PR #712 and resolve branch base

**Goal:** Determine whether the work in this plan rebases on PR #712's branch or lands sequentially after #712 merges. Eliminate the merge-conflict risk before any U2-U8 work starts.

**Requirements:** Preflight to all of R1-R8.

**Dependencies:** none.

**Files:**
- `library/travel/airbnb/internal/source/airbnb/graphql.go` (read-only inspect)
- `.printing-press-patches.json` (read-only inspect)

**Approach:**
- Check `gh pr view 712` status. If merged: branch from main. If open: branch from `fix/airbnb-ssr-extractor-and-pricing-key` (PR #712's head).
- Verify PR #712's head matches the local working state of the managed clone at `/Users/mvanhorn/printing-press/.publish-repo-mvanhorn-eb6a05f2/`.
- Confirm the parallel-session memory rule applies: this work happens in the same checkout as ongoing `feat/alaska-airlines-award-search` work. Use ce-worktree if any risk of collision; otherwise stage-before-switch.

**Patterns to follow:** memory note `feedback_parallel_session_git_collision`.

**Execution note:** This is a preflight unit, not a feature-bearing unit. No tests.

**Test scenarios:** Test expectation: none (preflight + branch coordination, no behavior change).

**Verification:** New feature branch `fix/airbnb-hash-scrape-and-rate-limit` (or similar) exists locally with the right base. `git log --oneline -5` shows the expected ancestor commits.

---

### U2. Fix BookingPrice variables shape to match the real schema

**Goal:** Eliminate the F3 ValidationError by sending the correct GraphQL variables shape for `StaysPdpBookItQuery`. The persisted-query hash is NOT stale — empirically verified during U2 execution by comparing the CLI's outgoing variables against a real captured request in the HAR fixture. The schema validator rejects the CLI's flat `{checkin, checkout, adults}` shape because the operation expects `{dateRange, guestCounts, includeXxxFragment}` objects.

**Pivot from plan-as-written:** The earlier plan called for runtime-scraping hashes from SSR HTML with a sync.Once + regex + fallback. During U2 execution, an empirical check against the live airbnb.com homepage AND a listing detail page showed that NEITHER inlines the GraphQL operation hashes in HTML. Apollo Client loads them from the JS bundle, which a non-JS-executing CLI cannot reach. The runtime-scrape approach is not feasible and is no longer needed: the actual F3 root cause is the variables shape, not the hash. Hash rotation, if it ever happens, becomes a maintenance PR (capture fresh hash via browser DevTools, update the constant).

**Requirements:** R1.

**Dependencies:** U1.

**Files:**
- `library/travel/airbnb/internal/source/airbnb/graphql.go` (rewrite BookingPrice variables map)
- `library/travel/airbnb/internal/source/airbnb/graphql_test.go` (add a variables-shape regression test if a test seam exists)

**Approach:**
- Rewrite the `variables` map in `BookingPrice` from `{id, checkin, checkout, adults}` to:
  ```go
  variables := map[string]any{
      "id": RelayListingID(listingID),
      "dateRange": map[string]any{
          "startDate": checkin,
          "endDate":   checkout,
      },
      "guestCounts": map[string]any{
          "numberOfAdults": guests,
      },
      "includePdpMigrationBookItCalendarSheetFragment": false,
      "includePdpMigrationBookItFloatingFooterFragment": false,
      "includePdpMigrationBookItNavFragment": false,
      "includeOverviewMerchandisingTipsFragment": false,
  }
  ```
- Keep the existing `bookItHash` constant — it is current, not stale.
- Add a one-line source comment near the variables map explaining the shape was derived from `.manuscripts/20260502-210359/discovery/airbnb/airbnb-capture.har` so future maintainers can reproduce the audit.
- Wishlist variables shape audit (`WishlistList`, `WishlistItems`): defer to U7 dogfood time. Only audit when those surfaces are exercised and fail. Likely they are also misshapen but they may not be — the call has different ergonomics and was probably less hand-edited.

**Patterns to follow:** Existing variables shapes inside the airbnb codebase that already work (e.g., the search SSR scrape's request structure). The HAR fixture at `.manuscripts/20260502-210359/discovery/airbnb/airbnb-capture.har` is the source of truth for shape audits.

**Execution note:** none. The fix is a struct rewrite based on captured ground truth.

**Test scenarios:**
- A unit test calling `BookingPrice` with a stubbed graphQLGet that captures the variables map; assertion: variables contains keys `id`, `dateRange`, `guestCounts`, and the four `include*Fragment` booleans; `dateRange.startDate` and `dateRange.endDate` are the strings passed in; `guestCounts.numberOfAdults` is the int passed in.
- A regression-style test that fails if the variables shape drifts back to the flat `{checkin, checkout, adults}` form.

**Verification:** Live run `airbnb-pp-cli airbnb-listing get 18413186 --checkin 2026-05-26 --checkout 2026-05-29 --adults 1` returns a non-null `price_breakdown.total`. (Reproducible because the plan's known-good listing 18413186 was previously listed and is anonymously accessible.)

---

### U3. Thread `--rate-limit` flag through to the source/airbnb scrape client

**Goal:** Make `--rate-limit N` actually apply to scrape and GraphQL traffic. Today the flag is plumbed only to `internal/client/client.go` (which the unique-capability commands do not use); the real upstream traffic uses a package-global `defaultClient` with a hard-coded 0.5 rps.

**Requirements:** R3, R8.

**Dependencies:** U1. Can land in parallel with U2.

**Files:**
- `library/travel/airbnb/internal/source/airbnb/client.go` (add `SetRate(rps float64)`)
- `library/travel/airbnb/internal/cli/root.go` (call `airbnb.SetRate(rateLimit)` from PersistentPreRunE)
- `library/travel/airbnb/internal/source/airbnb/client_test.go` (test for SetRate effect on limiter)

**Approach:**
- Add `SetRate(rps float64)` METHOD on `cliutil.AdaptiveLimiter`. It mutates `l.rate` (and `l.floor` if applicable) under the existing `l.mu`. No pointer swap on `defaultClient.limiter`, no atomic.Pointer, no new mutex on the airbnb client. The limiter instance stays put; only its rate changes.
- Add `func SetRate(rps float64)` to `internal/source/airbnb` as a thin pass-through that calls `defaultClient.limiter.SetRate(rps)`.
- `rps == 0` means "disable" (drain limiter); any positive float sets that as the per-second cap.
- Update `rootCmd.PersistentPreRunE` to call `airbnb.SetRate(rateLimit)` ONLY when the user explicitly passed the flag (`cmd.Flags().Changed("rate-limit")`). When the flag is unset, the existing hardcoded 0.5 rps baseline in `defaultClient` is preserved. This is the non-regressive path: today's behavior continues for users who don't touch the flag.
- Document the new behavior in SKILL.md (handled in U8): "`--rate-limit N` overrides the default 0.5 rps baseline for scrape and GraphQL traffic. Default (flag unset) is 0.5 rps. Pass `--rate-limit 0` to disable."

**Patterns to follow:** existing `cliutil.AdaptiveLimiter` semantics. `internal/client/client.go:53-64,166-286` shows how the same flag is honored on the generated-client side.

**Execution note:** none.

**Test scenarios:**
- `SetRate(2.0)` followed by 10 sequential `do()` calls completes in approximately 5 seconds plus overhead (rate of 2 rps observed)
- `SetRate(0.5)` produces approximately 20 seconds for 10 calls (matches former hardcoded behavior)
- `SetRate(0)` produces approximately zero wait between calls
- `SetRate` called concurrently from 3 goroutines does not race (run with `-race`); rate value is updated under the limiter's own `l.mu`, not by pointer swap
- A `do()` call in flight while `SetRate` runs does not panic and does not lose the limiter's `OnRateLimit`/`OnSuccess` history — the limiter instance is preserved across the rate change (no pointer swap means no history loss)
- Default invocation (no `--rate-limit` flag passed) keeps the existing 0.5 rps baseline. Verified by checking `cmd.Flags().Changed("rate-limit")` before calling `airbnb.SetRate`.

**Verification:** `airbnb-pp-cli airbnb-listing search "Mercer-Island--Washington--United-States" --checkin 2026-05-26 --checkout 2026-05-29 --rate-limit 2.0 --agent` completes its scrape in noticeably less wall-clock time than the same call with `--rate-limit 0.5`. Measured by timestamp diff between first and last network request in `read_network_requests` capture, or by wall-clock of the full invocation.

---

### U4. Add jittered exponential backoff to the retry loop

**Goal:** When the existing retry loop in `source/airbnb/client.go:175-213` sleeps after a 429 or (post-U5) bot challenge, wrap `cliutil.Backoff(attempt)` with random jitter so a fleet of retrying clients does not synchronize.

**Requirements:** R4.

**Dependencies:** U1. Can land in parallel with U2, U3.

**Files:**
- `library/travel/airbnb/internal/source/airbnb/client.go` (modify retry sleep)
- `library/travel/airbnb/internal/source/airbnb/client_test.go` (jitter bounds test)

**Approach:**
- The existing retry loop at `internal/source/airbnb/client.go:203` calls `time.Sleep(cliutil.RetryAfter(resp))`. There is no `cliutil.Backoff(attempt)` call in the current source. Two changes land in U4:
  1. Wrap the existing `RetryAfter` sleep with 25% jitter: `time.Sleep(cliutil.RetryAfter(resp) + jitter(cliutil.RetryAfter(resp)))` where `jitter(d) = time.Duration(rand.Int63n(int64(d/4)))`.
  2. When `RetryAfter` returns the default (no `Retry-After` header), fall back to `cliutil.Backoff(attempt)` with the same jitter wrapper, so subsequent retries actually grow rather than re-sleeping the same default each time.
- Decision: when both a `Retry-After` header and an exponential `Backoff(attempt)` could apply, `RetryAfter` wins (server-stated wait beats client-chosen wait).
- Seed `rand` package-locally via `rand.New(rand.NewSource(time.Now().UnixNano()))` (Go 1.20+ has implicit seed but pinning explicit makes tests deterministic with a stub source).

**Patterns to follow:** `cliutil/ratelimit.go` template comment: *"Callers needing jitter add their own; the bare exponential keeps the contract deterministic."*

**Execution note:** none.

**Test scenarios:**
- For attempt = 0..5, the actual sleep duration is in `[Backoff(attempt), Backoff(attempt) * 1.25)`
- For attempt = 0, sleep is non-negative and does not panic
- 1000 invocations at attempt = 3 produce a distribution where the mean is approximately `Backoff(3) * 1.125` (sanity check on randomness)
- Sleep duration never exceeds `cliutil.MaxBackoff + (MaxBackoff / 4)` regardless of attempt

**Verification:** `client_test.go -race` passes. Live run that triggers a 429 (via aggressive search loop) shows variable retry delays in the test log timestamps rather than identical delays.

---

### U5. BotChallengeError sentinel + datadome and Akamai detection

**Goal:** Detect 403/datadome and Akamai bot challenges as a typed error class, separate from generic 4xx. Branch the retry loop to back off on bot challenges and surface a remediation hint to the user. Currently any non-429 4xx including datadome challenges is collapsed into `fmt.Errorf("GET %s returned HTTP %d: %s", ...)` with a 300-char body truncation, which is silent failure for downstream callers.

**Requirements:** R4.

**Dependencies:** U1, U4 (jitter needs to be in place for the backoff branch).

**Files:**
- `library/travel/airbnb/internal/source/airbnb/errors.go` (new file or extend existing) for `BotChallengeError`
- `library/travel/airbnb/internal/source/airbnb/client.go` (detect in `do()`, branch retry loop)
- `library/travel/airbnb/internal/source/airbnb/client_test.go` (fixture-based detection tests)

**Approach:**
- Define `type BotChallengeError struct { ChallengeType string; Remediation string; StatusCode int; ResponseSnippet string }` with `Error() string` returning a structured one-line message.
- Add `func isBotChallenge(resp *http.Response, body []byte) (BotChallengeError, bool)` that returns the typed error + true when any of the documented signatures match (see decision matrix above).
- In `client.go:do()`, after reading the response body, call `isBotChallenge` BEFORE the generic `>= 400` branch. On true: call `limiter.OnRateLimit()`, sleep `Backoff(attempt) + jitter`, retry up to N times. After N attempts: return the sentinel (do not synthesize fallback).
- Mirror VRBO's `isBotChallenge` shape from `internal/source/vrbo/extract.go:206-210` so the two source packages stay consistent.

**Patterns to follow:** `cliutil.RateLimitError` sentinel pattern. VRBO `isBotChallenge` + `Warmup` in `internal/source/vrbo/client.go:47-62`.

**Execution note:** none.

**Test scenarios:**
- `isBotChallenge` returns true + datadome type for a response with `set-cookie: datadome=abc123; Path=/`
- `isBotChallenge` returns true + datadome type for a response with `server: dd-13`
- `isBotChallenge` returns true + datadome type for a 403 body containing `geo.captcha-delivery.com/captcha`
- `isBotChallenge` returns true + akamai type for body containing `captcha-pwa`
- `isBotChallenge` returns true + akamai type for title containing `bot or not` (case-insensitive)
- `isBotChallenge` returns false for a 403 with no challenge signatures (generic forbidden)
- `isBotChallenge` returns false for a 200 with happens-to-contain-captcha-in-prose body (negative test, body markers anchored)
- The retry loop calls `limiter.OnRateLimit` on bot-challenge same as on 429
- After max retries on bot challenge, the loop returns a `BotChallengeError` (errors.As succeeds)
- After max retries, no fake fallback data is emitted

**Verification:** Synthesize a 403 datadome response via httptest server, run a scrape against it, confirm the returned error has type BotChallengeError and the `Remediation` field carries actionable text. Verify with `errors.As(err, &target)` in the test.

---

### U6. Dogfood matrix: live-network integration test

**Goal:** Exercise every documented airbnb-pp-cli command + auxiliary surface against live Airbnb (gated on `AIRBNB_PP_DOGFOOD_LIVE=1`) and emit a structured `dogfood-results.json` artifact recording pass/fail per command. The matrix is the evidence that supports R5 and the proof bar for R6.

**Requirements:** R5, R6.

**Dependencies:** U1, U2, U3, U4, U5 (all the fixes must be in before the matrix is meaningful).

**Files:**
- `library/travel/airbnb/internal/dogfood/dogfood_live_test.go` (new)
- `library/travel/airbnb/internal/dogfood/fixtures.go` (new: known-good listing IDs, dates, wishlist IDs, host names)
- `library/travel/airbnb/internal/dogfood/README.md` (new: how to run, expected outputs, interpretation)
- `library/travel/airbnb/dogfood-results.json` (overwritten by test run)

**Approach:**
- Single `TestDogfoodMatrix` with `t.Run(commandName, func(t *testing.T){...})` per surface. Each subtest invokes the CLI as a subprocess (`exec.Command`), captures stdout + stderr + exit code, asserts on shape (not exact values, since prices vary).
- Surfaces to cover (Tier-1 must pass for the strict bar; Tier-2 nice-to-have records pass/fail but does not gate):
  - **Tier-1 (gating):** `airbnb-listing search`, `airbnb-listing get`, `cheapest`, `plan`, `compare`, `host extract`, `--rate-limit` actually applied (separate dedicated subtest with timestamp assertions)
  - **Tier-2 (recorded but non-gating):** `find-twin`, `match` (VRBO branch expected to ErrDisabled), `watch add`, `watch check`, `wishlist list`, `wishlist items`, `wishlist diff`, `host portfolio`, `fingerprint`
  - **Auxiliary:** `doctor`, `agent-context`, `profile save/list/show/delete`, `which`, `feedback`, `--deliver file:`, `--deliver webhook:`, `--data-source auto/live/local`, `--dry-run`, `--agent`, `--select`, `auth status` (do not run `auth login --chrome` from test — assume the session was logged in once via the doctor preflight)
- Each subtest writes a JSON entry to a temp results file with: `{name, tier, exit_code, ok, duration_ms, observed_error_type, stdout_shape}` where `stdout_shape` records the JSON FIELD NAMES present in the response, NEVER field values. Cookie headers, response-snippet bodies, wishlist titles/cities, and any other session-bearing or PII-shaped content are stripped before serialization (security S-3).
- At test teardown, merge entries into `dogfood-results.json` at the CLI root.
- Add a dedicated subtest `TestRateLimitGuardFires` that hammers the search endpoint at `--rate-limit 100` (intentionally aggressive) and asserts that either (a) the limiter caps the actual wire rate well below 100/sec via timestamp deltas, or (b) Airbnb returns 429 / bot challenge and the limiter halves the rate via `OnRateLimit`.

**Patterns to follow:** the existing `dogfood-results.json` schema convention. The Go subprocess pattern in any of the existing CLIs' integration tests (look for `exec.Command(binPath, ...)` usage). Memory note `feedback_check_binary_freshness_before_filing_bug` — the test must rebuild the binary fresh before running, not assume `$GOPATH/bin/airbnb-pp-cli` is current.

**Execution note:** Test-first is not appropriate here (the test is the matrix; it does not pre-drive design). Run the matrix once during development to discover failures, fix them, repeat until Tier-1 is green.

**Test scenarios:**
- Tier-1: `cheapest` on the known-good Mercer Island listing (id 18413186, checkin 2026-05-26, checkout 2026-05-29) returns a non-null `airbnb.total` and a non-null `direct.total` (or, if direct lookup fails, an `errors.As(err, &HostNotFoundError)` typed error rather than a panic)
- Tier-1: `plan "Mercer Island" --checkin 2026-05-26 --checkout 2026-05-29 --guests 1 --budget 500 --agent` returns at least one ranked candidate with non-null prices
- Tier-1: `compare 18413186 <other-id> --checkin ... --checkout ... --agent` returns a paired struct with non-null totals
- Tier-1: `--rate-limit` test demonstrates either a wire-rate cap or `OnRateLimit` engagement
- Tier-2: `wishlist list` either returns the user's wishlist (when authenticated) or `ErrAuthRequired` (clean typed error) — not a 401 raw HTML body
- Tier-2: `match <airbnb-url>` either returns a cross-platform listing or `ErrDisabled` (VRBO disabled by design) — not a panic
- Auxiliary: `doctor` returns exit code 0 and prints `OK API: reachable` line
- Auxiliary: `--dry-run` on any command exits 0 without firing a real request (verify via test http server with hit counter)

**Verification:** `AIRBNB_PP_DOGFOOD_LIVE=1 go test ./internal/dogfood/... -timeout 10m -v` produces a passing Tier-1 subtest set and a populated `dogfood-results.json`. Each Tier-1 entry has `ok: true`.

---

### U7. Execute the dogfood matrix and triage

**Goal:** Run the matrix end-to-end against live Airbnb, triage failures, and produce the artifact that proves the CLI works or justifies deletion.

**Requirements:** R5, R6.

**Dependencies:** U6 (and through it, U2-U5).

**Files:**
- `library/travel/airbnb/dogfood-results.json` (overwritten with live-run results)
- `library/travel/airbnb/internal/dogfood/README.md` (append triage notes)

**Approach:**
- Build a fresh binary from the feature branch (`go install ./cmd/airbnb-pp-cli` and verify `--version` matches the branch HEAD).
- Run `AIRBNB_PP_DOGFOOD_LIVE=1 go test ./internal/dogfood/... -timeout 10m -v` and capture full stderr/stdout to `library/travel/airbnb/internal/dogfood/dogfood-run-2026-05-20.log`.
- For any Tier-1 failure: file a per-failure plan amendment (either inline fix attempt or deferred to a follow-up PR). User-confirmed strict bar means ANY Tier-1 failure is a delete-from-library trigger; do not paper over.
- For Tier-2 failures: record in `dogfood-results.json` and note in README; non-gating but worth surfacing.
- Generate a one-page summary: pass count / fail count / surfaces with no real-data backing / actual observed bot-challenges (if any) / wall-clock for full run / rate-limit-firing evidence.

**Patterns to follow:** the 2026-05-03 quarantine plan's evidence discipline. Memory note `feedback_evidence_every_pr`.

**Execution note:** none. This unit is execution + triage, no new code unless inline fixes turn out to be small.

**Test scenarios:** Test expectation: none (this unit IS the test).

**Verification:**
- `dogfood-results.json` has entries for every documented command + every auxiliary surface from U6.
- Tier-1 pass count equals Tier-1 total count (strict bar) OR triggers U8 deletion fork.
- `TestRateLimitGuardFires` is in the passing set.
- Triage notes in README cite specific command + observed error for any Tier-2 fail.

---

### U8. Documentation update and deletion fork (conditional)

**Goal:** Reflect all behavior changes in user-facing docs. If U7 fails the strict bar, execute the deletion path instead.

**Requirements:** R6, R8.

**Dependencies:** U7.

**Files (pass path):**
- `library/travel/airbnb/SKILL.md` (rate-limit semantics, BotChallengeError exit semantics, dogfood matrix mention, known-good listing IDs for reproducibility)
- `library/travel/airbnb/README.md` (rate-limit + bot-challenge notes, link to dogfood-results.json)
- `library/travel/airbnb/.printing-press-patches.json` (append amend entries for U2, U3, U4, U5)
- `library/travel/airbnb/agent-context.json` (if it carries rate-limit semantics or command status; check the file before assuming)

**Files (delete fork):**
- `library/travel/airbnb/**` (remove directory)
- `registry.json` at repo root (remove airbnb entry)
- `library/README.md` or equivalent catalog (remove airbnb mention)
- `.claude/skills/pp-airbnb/SKILL.md` (remove from skill index if registered there)
- Add a one-paragraph entry to `docs/solutions/` documenting why the CLI was removed: "Bot defenses on airbnb.com proved too aggressive to keep the headline arbitrage path working without a headed-browser tier. Quarantined until/unless a Chrome-tier rewrite lands."

**Approach (pass path):**
- Update SKILL.md to describe the new `--rate-limit` semantics: "Applies to all scrape and GraphQL traffic. Default 0.5 rps. Set 0 to disable (not recommended)."
- Add a new exit-code row to the SKILL.md exit-code table: `8 | Bot challenge (datadome/Akamai). Wait + refresh cookies via 'airbnb-pp-cli auth login --chrome'.`
- Document the dogfood matrix in README: how to run it, what the artifact says, when to re-run.
- Update `.printing-press-patches.json` with amend entries naming the four bug-fix units.

**Approach (Tier-1 failure fork — three-option modal):**

When U7 surfaces a Tier-1 failure (specifically: `cheapest` returns null/error on a known-good listing for reasons rooted in this plan's work), present the user with a three-option blocking modal:

1. **Delete from library.** Remove the directory, update `registry.json`, remove from `library/README.md`, deregister the skill, write a post-mortem entry in `docs/solutions/`. Open a PR titled `chore(airbnb): remove from library; deferred to chrome-tier rewrite` with the dogfood-results.json link as evidence.
2. **Quarantine via `--experimental`.** Keep the source on disk. Gate the broken Tier-1 commands behind a `--experimental` flag (mirrors the 2026-04-30 ebay honest-capabilities pattern). Update `agent-context.json` to mark affected commands as experimental. Update SKILL.md "Known Limitations" section with the dogfood-results.json evidence. PR title: `chore(airbnb): quarantine broken commands behind --experimental flag`.
3. **Keep shipping with documented caveat.** Update SKILL.md and README to document the Tier-1 limitation prominently. No code gate, but the user must read about the failure before invoking. PR title: `docs(airbnb): document <command> regression`.

The modal is the ONLY visible interface for this decision (per the "modal-is-the-only-visible-thing" memory rule). Do not present the choices in prose ABOVE the modal — every option must be a clickable modal option. Prose context can appear within the modal question stem or option descriptions.

**Patterns to follow:** the 2026-04-30 ebay honest-capabilities plan (chose hide-not-delete; this plan chose strict-bar delete per user direction). Memory notes `feedback_no_process_in_pr_body`, `feedback_evidence_every_pr`.

**Execution note:** none.

**Test scenarios:**
- (pass path) SKILL.md renders cleanly via the existing skill-render check
- (pass path) `.printing-press-patches.json` is valid JSON and the new entries follow the existing schema
- (delete fork) registry no longer mentions airbnb; `library/README.md` reflects the removal; the skill is no longer registered

**Verification:**
- Pass path: PR opened with all docs updates + a green dogfood-results.json link. PR body cites the dogfood matrix results and references this plan.
- Delete fork: PR opened with the removal commit + post-mortem entry in docs/solutions/. User has explicitly confirmed before push.

---

## System-Wide Impact

| Affected surface | Impact | Mitigation |
|---|---|---|
| `--rate-limit` flag semantics | Behavior change: was no-op for scrape, now actually applies | Document explicitly in SKILL.md (R8). Mention in PR body. |
| External agents calling airbnb-pp-cli | New `BotChallengeError` exit code (8). Existing 429 behavior preserved. | Document in SKILL.md exit-code table. |
| Other PP CLIs that share `internal/cliutil/ratelimit.go` | No source-code change to that file (jitter added at caller). Comment unchanged. | None needed. |
| Library registry / catalog | If U8 deletion fork fires, airbnb is removed from `registry.json` and `library/README.md`. | One-line user confirmation gate. |
| Open PR #712 | New plan rebases on or sequences after #712. Hash work touches the same file PR #712 touched. | U1 resolves the base branch explicitly. |
| `~/.osc/projects.json` | New PR URL needs to be tracked per memory note `feedback_always_track_submitted_prs`. | Tracked automatically by the PR open step in U8. |

---

## Phased Delivery

**Phase 1 (single PR, U1-U5):** Fix the three hashes (U2), thread the rate-limit flag (U3), add jitter (U4), add BotChallengeError + detection (U5). Ship as one cohesive bug-fix PR. Dependency on PR #712 resolved by U1.

**Phase 2 (separate PR, U6-U7):** Build and run the dogfood matrix. Either green-light the CLI for continued life in the library, or trigger Phase 3.

**Phase 3 (conditional, U8 deletion fork):** Only fires if Phase 2 fails the strict bar. Removes the CLI from the library with an evidence-backed post-mortem.

Phases 1 and 2 may share a single PR if the dogfood matrix runs cleanly the first time. The split is conceptual; PR boundaries are an implementation-time decision.

---

## Risk Analysis & Mitigation

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Airbnb rotates the API key + hashes during this work; scraped values become stale before PR lands | Medium | High | The runtime-scrape pattern is the mitigation. Even if the constants are stale at merge time, the first user invocation refreshes them. Worst case: write a single-line script that re-captures all three from a fresh HTML pull and updates the fallback constants. |
| SSR scrape itself gets datadome-challenged before the hash resolver can run | Medium | High | The constant fallback covers this case. Log the failure loudly so users know to refresh cookies or escalate. Headed-browser tier is documented as the next escalation, not built here. |
| Parallel-session git collision wipes uncommitted edits (per memory note) | Medium | Medium | Use ce-worktree for this work, or commit aggressively (every U-unit completion). Repeat the lesson the amend agent learned the hard way. |
| Dogfood matrix triggers an IP block during the run that contaminates subsequent commands | Medium | High | `TestRateLimitGuardFires` runs LAST in the subtest ordering, after the data-correctness checks. Use a conservative `--rate-limit 0.5` as the default for all non-rate-limit-test subtests. If a bot challenge fires mid-run, the test logs it and surfaces it as a Tier-2 finding rather than failing the whole matrix. |
| Hash regex matches partial data (e.g., a related operation name that happens to contain `BookItQuery` as substring) | Low | High | Anchor the regex on the full operation name + the `persistedQueryName` field + `sha256Hash` field structure. Test the negative case (regex must not match a near-miss substring). |
| `SetRate` race with in-flight `do()` causes a missed limiter wait | Low | Low | Use the same mutex that `AdaptiveLimiter` already holds for `Wait()`. Test with `-race`. |
| Wishlist tests in dogfood matrix fail because the test account's session is not loaded | Medium | Low | Mark wishlist subtests as `t.Skip` when `auth status` shows unauthenticated. Document the auth-login step in the dogfood README. |
| Cookie session captured in `~/.config/airbnb-pp-cli/config.toml` expires mid-dogfood | Low | Medium | `auth login --chrome` is a one-line refresh. Add a doctor-style preflight to the dogfood test that exits early with a clear remediation if cookies are stale. |
| Cookie file world-readable due to default Go file permissions (security S-1) | Medium | High | `auth login --chrome` must write `~/.config/airbnb-pp-cli/config.toml` with mode 0600 explicitly. Dogfood subprocess capture (`exec.Command`) MUST NOT include the file contents in `dogfood-results.json` or any logged artifact. Redact `Cookie:` headers from stderr summaries. |
| `--deliver webhook:<url>` exfiltration risk (security S-2) | Medium | High | Dogfood matrix exercises `--deliver webhook:` ONLY against a test-fixture `127.0.0.1:<port>` httptest server. Document explicitly that the existing webhook sink does no URL-scheme allowlisting; SSRF posture is "user-controlled URL, no allowlist," and that's the existing behavior the dogfood test is NOT widening. Recommend a follow-up unit to add scheme + host validation. |
| `dogfood-results.json` captures session-bearing snippets / personal wishlist data (security S-3) | Medium | Medium | Restrict the dogfood JSON schema to: `{name, tier, exit_code, ok, duration_ms, observed_error_type}` plus a `stdout_shape` field that records FIELD NAMES present, never field values. Strip `BotChallengeError.ResponseSnippet` before serialization. Wishlist subtests log only listing counts, not titles/cities. |

---

## Alternative Approaches Considered

### Alternative A: Refactor `internal/source/airbnb` to drop the `defaultClient` package-global entirely

Pros: cleaner long-term architecture; matches the per-call-constructor pattern used elsewhere; makes future testability easier (each command holds its own client).

Cons: ~30-file refactor touching every command in `internal/cli/`. High blast radius. Introduces conflict risk with any parallel work in the CLI. Not justified by the rate-limit problem alone.

Rejected for this plan. Recorded for a future refactor PR if the architecture decision earns the work later.

### Alternative B: Hardcode-only fix for the three hashes (no runtime scrape)

Pros: one-line patch per hash. Trivial diff.

Cons: hashes rotate every few months at Airbnb's pace. The fix would rot the same way F3 did. Does not satisfy R7.

Rejected.

### Alternative C: Skip BotChallengeError detection; treat all 4xx as generic errors with a longer Retry-After cap

Pros: smaller change.

Cons: silent misclassification persists. Users still see truncated HTML bodies as their error message. Does not satisfy R4's user-facing intent.

Rejected.

### Alternative D: Hide the broken commands behind `--experimental` flag (mirrors the 2026-04-30 ebay honest-capabilities plan)

Originally framed as "rejected; natural fallback if user changes mind." Upgraded after doc-review feedback (P-3): the user-confirmation modal in U8 now offers this path as one of three first-class options, not as an implicit fallback. The plan still names "prove or delete" as the headline framing, but the modal lets the user pick a middle path at decision time without backing out and re-running the workflow.

---

## Success Metrics

| Metric | Target |
|---|---|
| Tier-1 commands passing the dogfood matrix | 100% (strict bar) |
| `--rate-limit` flag observable on the wire | Yes, via timestamp deltas in `read_network_requests` capture |
| `BotChallengeError` typed sentinel returns when datadome fires | Yes, via httptest synthetic 403 in unit test |
| Live `airbnb-pp-cli airbnb-listing get <id> --checkin X --checkout Y` returns numeric `price_breakdown.total` | Yes |
| Live `airbnb-pp-cli cheapest <url>` returns non-null savings struct | Yes |
| All three hashes resolve via runtime scrape on a fresh install (no constant fallback used) | Yes, verified by log inspection |
| Number of integration tests in `internal/dogfood/` | At least 14 (matches documented command count) + 1 rate-limit-guard test |

---

## Documentation Plan

- SKILL.md updates: rate-limit semantics, BotChallengeError exit code, dogfood matrix run instructions, known-good listing IDs for reproducible smoke test
- README.md updates: same surface, user-facing tone
- `.printing-press-patches.json` updates: append entries for each behavior change (U2, U3, U4, U5)
- `library/travel/airbnb/internal/dogfood/README.md`: new file, explains how to run the matrix, interpret results, refresh auth cookies
- `docs/solutions/` entry: only if U8 deletion fork fires; documents the why for future reference

---

## Operational / Rollout Notes

- The PR for Phase 1 lands as a bug-fix; backward-compatible behavior change (rate-limit flag becomes effective on a path where it was previously ignored). One-line note in PR body flagging it.
- The PR for Phase 2 includes the dogfood-results.json artifact as evidence. Per memory note `feedback_evidence_every_pr`, the PR body links to the artifact and cites Tier-1 pass count.
- The PR for Phase 3 (deletion, conditional) requires explicit user confirmation before the destructive commit. Per memory note `feedback_kooky_fork_private` and general carefulness around destructive operations, no auto-merge.
- Refreshing the constant fallbacks: if the runtime scrape fails repeatedly in the wild (visible via log search), the maintainer captures a fresh `airbnb.com/s/` response and updates the four `*Fallback` constants in one tiny PR. This is the documented maintenance path.
- Cookie session refresh path: `airbnb-pp-cli auth login --chrome` (existing command). Document in SKILL.md and in the dogfood README as the remediation when BotChallengeError surfaces.
- Headed-browser escalation: if datadome challenges persist after this plan ships, the next-tier remediation is to swap the SSR scrape path to drive a real Chrome session via the claude-in-chrome MCP. Tracked as a deferred follow-up, not part of this plan.

---

## Deferred Items / Open Questions

- Should the rate-limit `SetRate` calls propagate to the (currently unused) `internal/source/vrbo` client too? Probably yes for symmetry, even though VRBO is ErrDisabled today. Defer the decision to the implementer; cheap to do; safer if VRBO ever re-enables.
- Should `dogfood-results.json` be committed to the repo or treated as a build artifact only? Convention check needed during U6. Default: commit it, so the latest evidence travels with the source.
- Should the bot-challenge `Remediation` field localize? No, deferred. English-only for this plan.
- If U7 fails Tier-1 but the user does not confirm deletion, what is the holding state? Document the CLI as `status: quarantined` in `registry.json` with a one-line explanation. Same pattern as the 2026-05-03 VRBO quarantine, applied at the CLI level.

---

## Origin Trace

This plan was generated by `/ce-plan` from a direct user request on 2026-05-20. There was no upstream `ce-brainstorm` requirements document; the planning bootstrap captured intent directly. Phase 0.7 scoping synthesis was confirmed by the user with three blocking choices:

- Rate-limit: adaptive backoff + default cap
- Dogfood breadth: all documented surfaces
- Proof bar: strict (real data + rate-limit fires)

The deletion fork in U8 exists because the user explicitly named that branch in the original request ("prove that this thing works, or we need to delete it from the library").
