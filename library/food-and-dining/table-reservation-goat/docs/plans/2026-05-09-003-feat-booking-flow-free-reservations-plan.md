---
title: "feat: book and cancel for free reservations on OpenTable and Tock"
type: feat
status: completed
date: 2026-05-09
deepened: 2026-05-09
completed: 2026-05-09
origin: docs/brainstorms/2026-05-09-booking-flow-free-reservations-requirements.md
---

# feat: book and cancel for free reservations on OpenTable and Tock

## Summary

Six implementation units add top-level `book` and `cancel` CLI commands to complete the cross-network reservation lifecycle for free reservations on both OpenTable and Tock. Live chrome-MCP discovery (U1) gates per-network booking clients (U2 OT, U3 Tock — parallel-safe) with explicit go/no-go scope-blocks for lock+commit-required, CSRF-needs-JS, and OT-WAF-blocking discoveries. CLI commands (U4 book, U5 cancel) layer four safety mechanisms: `cliutil.IsVerifyEnv()` floor (catches verifier mock-mode), `TRG_ALLOW_BOOK=1` env-var commit gate for book, filesystem advisory lock for concurrent-invocation safety, and CLI-boundary error sanitization (raw upstream chains never reach stdout JSON). Idempotency orchestration lives in the CLI; network clients stay single-responsibility. Discovered API shapes land inline as schema-comment blocks at the top of each `booking.go`, mirroring the existing `calendar.go` / `search.go` pattern.

---

## Problem Frame

The brainstorm (see `origin`) established that the CLI's existing transcendence commands stop short of actually booking, breaking the "agent completes the loop" UX when an LLM agent finds a slot on the user's behalf. Slot tokens are now returned by `RestaurantsAvailability` (OT) and `SearchCity` (Tock); what's missing is the surface that fires the book / cancel calls safely under the slot-token race, with verify-before-retry idempotency and a test-budget guardrail. This plan focuses on the HOW of executing that work in this codebase against the established source-client + transcendence-command patterns.

---

## Requirements

- R1. CLI exposes a top-level `book` command accepting network-prefixed venue (`opentable:<slug>` / `tock:<slug>`), with `--date`, `--time`, `--party` as required arguments.
- R2. `book` works for free reservations on both networks; payment-required venues return a typed v0.3-pointer error.
- R3. `book` commits in a single CLI call by default; no mid-call user prompting.
- R4. CLI exposes a top-level `cancel` command accepting network-prefixed reservation ID.
- R5. Pre-flight idempotency check via list-upcoming-reservations on the network; matching reservation returns existing ID with `matched_existing: true`.
- R6. Live commit gated by `TRG_ALLOW_BOOK=1` env var; unset returns dry-run envelope plus enable hint.
- R7. `cancel` ungated by `TRG_ALLOW_BOOK` (recovery action) but still respects the verify-mode floor (R12).
- R8. Cancellation-window deadline surfaced in `book` confirmation JSON.
- R9. `book --dry-run` returns would-book envelope without firing book call. Pre-flight idempotency check (read-only) still runs in dry-run mode so the envelope reflects whether an existing match exists.
- R10. `book`/`cancel` return agent-friendly JSON to stdout. Raw upstream error strings (which may contain cookies, session tokens, or PII) are stripped before stdout — only typed error category + sanitized one-line message reach stdout JSON `details`. Raw error chains may be logged to stderr. No separate audit-log file.
- R11. Network errors (slot taken, auth, 4xx/5xx, bot-detection) surface as typed JSON; never silent zero-result success.
- R12. **Verify-mode floor.** Both `book` and `cancel` short-circuit to a dry-run envelope when `cliutil.IsVerifyEnv()` returns true (i.e., `PRINTING_PRESS_VERIFY=1`). This is the floor that catches verifier mock-mode regardless of `TRG_ALLOW_BOOK` state, per the AGENTS.md side-effect-command rule. Independent of (and stricter than) R6's commit gate.
- R13. **Idempotency normalization.** Pre-flight match comparison normalizes: time → 24h `HH:MM`, slug → lowercase trimmed of network suffix, party → integer. Mismatched-format reservations on the network side that semantically equal the request must produce a `matched_existing: true` hit, not a false negative leading to a duplicate book.
- R14. **Required-arg validation order.** `--date`, `--time`, `--party` are validated inside `RunE` AFTER the verify-mode (R12), dry-run (R9), and `TRG_ALLOW_BOOK` (R6) guards have decided whether to short-circuit. Cobra's `MarkFlagRequired` (which fires before `RunE`) is NOT used — it would conflict with `printing-press verify`'s `--dry-run` probe of hand-written commands.

**Origin actors:** A1 (CLI user), A2 (LLM agent), A3 (OpenTable), A4 (Tock)
**Origin flows:** F1 (agent-initiated book), F2 (agent-initiated cancel), F3 (dry-run preview)
**Origin acceptance examples:** AE1–AE6 (see `origin`)

---

## Scope Boundaries

- Payment / prepayment flows (Tock prepaid tasting menus, OT paid experiences) — separate v0.3 brainstorm.
- Auto-book on `watch tick` hits — explicit non-goal in v0.2.
- Two-step lock-then-commit pattern — revisit in v0.3 when payment makes locking necessary.
- General "list my upcoming reservations" user-facing command — pre-flight is internal-only in v0.2.
- Reservation modification (date/time/party change without cancel + rebook) — not in v0.2.
- Multi-network parallel booking — `book` operates on one network at a time; agent orchestrates if needed.
- Persistent local SQLite of bookings — confirmation JSON to stdout is the surface.
- Card-on-file and payment-method management — moot for v0.2.
- Caching layer for upcoming-reservations lookup — pre-flight fires every book call; caching adds invalidation complexity not justified yet.
- Group bookings, large-party special handling, dietary-restriction propagation — not in v0.2.

### Deferred to Follow-Up Work

- chrome-MCP capture transcripts and protocol notes — kept in U1 working notes during execution; not committed to repo as a durable artifact.

---

## Context & Research

### Relevant Code and Patterns

- **`internal/source/tock/calendar.go`** — capture-protos-from-bundle technique, `each(...)` walker, schema-comment-block-at-top discipline. The closest precedent for "reverse-engineer a Tock endpoint and document it inline." `booking.go` for Tock follows the same pattern.
- **`internal/source/tock/search.go`** — most recent precedent (this session); reuses `FetchReduxState` for SSR-rendered data, hand-rolls JSON-walker for one specific subtree, comment-block at top documents the JSON path read. The closest fit for "GET an SSR page, extract user-state from `$REDUX_STATE`" — the upcoming-reservations lookup likely reads `state.purchase` or similar from a `/me`-style page.
- **`internal/source/opentable/client.go`** — `gqlCall`, `Bootstrap`, `RestaurantIDFromQuery`, `RestaurantsAvailability` as the GraphQL-persisted-query precedent. OT booking is almost certainly another GraphQL persisted-query.
- **`internal/source/opentable/avail_cache.go`** — singleflight, atomic disk write, schema-versioned + hash-versioned cache invalidation. Not needed for booking (don't cache writes), but the path-traversal-safe filename hashing is referenceable.
- **`internal/source/opentable/cooldown.go`** — `BotDetectionError`, disk-persisted cooldown, exponential backoff. Book commits should respect cooldown (fast-fail) but not auto-retry.
- **`internal/cli/goat.go`** — top-level transcendence command shape, `<network>:<slug>` prefix routing, `Network: "opentable"` / `"tock"` result-row pattern. The closest precedent for `book`/`cancel` CLI shape.
- **`internal/cli/earliest.go`** — `--no-cache`, env-var override (`TRG_OT_NO_CACHE`), agent-friendly JSON output. Pattern for `TRG_ALLOW_BOOK` and `--dry-run` flag.
- **`internal/cli/root.go`** — `rootCmd.AddCommand(...)` registration site for new top-level commands.
- **`internal/source/auth/auth.go`** — `Session.HTTPCookies(network)` is the cookie-passing path; cookies imported via `auth login --chrome` carry through.

### Institutional Learnings

- No matching `docs/solutions/` entries yet. Booking work extends the `cross-network-source-clients` patch line (current at v0.1.15 after this session's SearchCity work).
- This session's SearchCity discovery pattern (live chrome-MCP capture before bundle archaeology) is now the documented best practice — applied here for U1.

### External References

- None used. The technique stack is fully repo-local; OT and Tock book/cancel endpoints have no public docs and require browser-sniff discovery.

---

## Key Technical Decisions

- **Discovery (U1) gates encoder/handler work, with explicit go/no-go thresholds.** Live chrome-MCP capture against the live OT and Tock origins produces concrete request/response shapes for book, cancel, and list-upcoming-reservations on both networks. Without this gate, U2/U3 would invent shapes and fail at U6 dogfood. U1 outputs schema-comment blocks at the top of each `booking.go`. **Hard scope-block conditions** (any single one triggers plan re-scope, NOT silent absorption): (a) Tock free-reservation flow requires a separate slot-lock POST before commit (lock+commit pattern explicitly deferred to v0.3 per origin); (b) write paths require CSRF / JWT material that kooky-imported cookies cannot produce without browser JS execution; (c) OT booking opname is not on Akamai's WAF allowlist and gqlCall returns 403 — chromedp-attach fallback (mirroring `chrome_avail.go`) becomes in-scope for U2.
- **Verify-mode floor for write commands.** Per AGENTS.md side-effect-command rule, `book` and `cancel` short-circuit to a dry-run envelope when `cliutil.IsVerifyEnv()` returns true (`PRINTING_PRESS_VERIFY=1`). This is the only safety check stricter than `TRG_ALLOW_BOOK`'s opt-in; it ensures `printing-press verify` mock-mode subprocesses cannot fire real commits even if the user's shell exports `TRG_ALLOW_BOOK=1`. Cancel especially needs this floor since R7 leaves it ungated by `TRG_ALLOW_BOOK`.
- **`book` fetches the slot token internally, not from a CLI argument.** The CLI surface accepts `<network>:<slug> --date --time --party` only. Network clients' `Book(ctx, slug, date, time, party, lat, lng) (*BookResponse, error)` re-fetches availability and books with the fresh token in one call. Rationale: (a) slot-tokens-as-CLI-args would surface in shell history, `ps aux`, and MCP tool-call argument logs — short-lived but worth not exposing; (b) re-fetching narrows the slot-token race window to the time between the internal availability fetch and the immediately-following book call; (c) keeps the CLI surface stable and predictable. Cost: one extra round-trip per book call. Acceptable.
- **Concurrency safety: filesystem advisory lock around pre-flight + book.** A single CLI process is not the only failure mode; an agent can fire `book` twice in 100ms and both processes pass pre-flight before either commits. To prevent double-booking, U4 acquires a per-key filesystem advisory lock (e.g., flock on `<UserCacheDir>/table-reservation-goat-pp-cli/book-locks/<sha256>(network|slug|date|time|party).lock`) BEFORE pre-flight runs and releases AFTER book completes (success or error). The lock is OS-level and cross-process. If U1 surfaces an Idempotency-Key header on either network's endpoint, prefer that over filesystem lock; the lock is the cross-network fallback.
- **One booking-client unit per network.** OT and Tock have separate `internal/source/{opentable,tock}/` packages and zero shared booking infrastructure. Splitting into U2 (OT) and U3 (Tock) makes them parallel-safe and mirrors the existing package boundary.
- **Idempotency orchestration lives in the CLI command, not the network client.** Network clients expose `Book(...)`, `Cancel(...)`, `ListUpcomingReservations(...)` as single-responsibility methods. The CLI's `book` command runs `acquire-lock → ListUpcoming → normalize+match? → Book → release-lock` orchestration. Keeps clients reusable, idempotency policy explicit, and tests can mock at either layer.
- **Book POSTs distinguish response-received from response-never-received before deciding retry.** No auto-retry when an HTTP response (any 2xx/4xx/5xx) was received — the server saw the request, retry might double-book. ON connection-reset / read-timeout / TLS error before response headers arrive — at-most-once retry IF U1 confirms an Idempotency-Key header is supported by the endpoint; otherwise surface the ambiguity to the user with a "verify on opentable.com / exploretock.com before re-running" hint. Cancel uses `do429Aware` as-is (one 429 retry total via the shared helper); no extra retry layer.
- **`TRG_ALLOW_BOOK=1` is the commit gate for `book`, not a `--commit` flag.** Per origin Key Decision: env var resists tab-completion and shell-history accidents; flag would not. Cancel ungated per origin R7 — `IsVerifyEnv` floor (above) is its safety net.
- **Discovery output is inline schema-comment block in each `booking.go`.** Same pattern as `calendar.go` and `search.go`. A separate `docs/research/...` doc would rot; the comment block stays adjacent to the code that depends on it and surfaces in diff when the upstream API rotates. **Note:** captured CSRF / auth-token VALUES are NEVER committed (would be live secrets); only field names, header names, and lookup paths.
- **MCP annotations: minimal and per AGENTS.md.** Neither `book` nor `cancel` sets `mcp:read-only` (both write). Neither sets a custom `destructiveHint` annotation — the cobratree walker doesn't read one today, and the AGENTS.md default ("missing annotation = could write or delete") makes MCP hosts prompt. This is the conservative choice: prompt-on-every-call is acceptable UX for write commands; suppressing the prompt would require explicit work (typed MCP tool entries) and the wrong claim is worse than missing.
- **No caching of upcoming-reservations lookup in v0.2.** Pre-flight fires every book call. Caching adds invalidation complexity (when does the cache know about a fresh booking made via website?) not justified for the call volume.
- **Cancel does not perform pre-flight discovery.** The reservation ID returned by `book` is the unambiguous handle; cancel takes it directly.
- **`book` and `cancel` parse the network-prefix slug at the CLI layer.** Centralized in a small helper (matching the existing `<network>:<slug>` precedent in `earliest.go` and `watch add`); each command then dispatches to the right network client.
- **Error sanitization at the CLI boundary.** Network clients return typed errors with raw error chains (which may carry response bodies, headers, or upstream PII). The CLI layer maps typed errors to a sanitized JSON shape: error category + safe one-line message. Raw error chains never appear in stdout JSON `details`. The CLI may emit raw chains to stderr or a structured local log when verbose mode is on, but stdout is reserved for sanitized output.

---

## Open Questions

### Resolved During Planning

- **Where does idempotency orchestration live?** CLI command (U4), not network client.
- **Does `book` retry on errors?** No retry when an HTTP response was received. At-most-once retry on connection-reset-before-headers ONLY if U1 confirms an Idempotency-Key header. Cancel uses `do429Aware` as-is (one 429 retry shared with reads); no extra retry layer.
- **Does discovery output get its own research doc?** No. Inline schema-comment block in each `booking.go`. Captured auth-token values are never committed.
- **How is the slot token sourced?** `book` fetches availability internally and uses the fresh token in the immediately-following book call. CLI surface accepts `<network>:<slug> --date --time --party` only.
- **What's the verify-mode floor?** Both `book` and `cancel` short-circuit on `cliutil.IsVerifyEnv()`. `TRG_ALLOW_BOOK` is a separate, narrower gate that applies only to `book` commit.
- **MCP annotations for book and cancel?** Neither sets `mcp:read-only` or a custom `destructiveHint`. Default ("could write or delete") makes MCP hosts prompt — the safe-by-default position.
- **How is concurrent invocation handled?** Filesystem advisory lock keyed on `(network, slug, date, time, party)` around pre-flight + book in U4.
- **Does dry-run skip pre-flight?** No. Pre-flight (read-only `ListUpcomingReservations`) always runs so the dry-run envelope reflects existing matches. Pre-flight failure aborts (does not fall through).
- **Are the required args validated via Cobra `MarkFlagRequired`?** No. Validated inside `RunE` after the verify-mode and dry-run guards, so `printing-press verify`'s `--dry-run` probe still reaches the short-circuit.

### Deferred to Implementation

- **Exact OT book/cancel persisted-query hashes and variable shapes.** U1 captures these; U2 encodes them.
- **Exact Tock book/cancel endpoint paths and request bodies.** U1 captures; U3 encodes.
- **OT list-upcoming-reservations endpoint shape.** Likely a GraphQL persisted-query under the consumer GraphQL gateway. U1 confirms.
- **Tock list-upcoming-reservations location.** Likely a `state.purchase.upcoming` slice (or equivalent) under `/profile` or `/account/upcoming`. U1 confirms.
- **Cancellation-window deadline shape.** Per-venue convention varies; U1 confirms whether networks return a deadline timestamp in the book response. If not returned, surface `cancellation_deadline: null` + `note` referring to venue policy.
- **Slot-taken discriminator pattern.** U1 prefers HTTP status code → GraphQL error extension → message substring (last resort); U2/U3 add `ErrCanaryUnrecognizedBody` for drift-detection.
- **Slot-token TTL for each network.** U1 measures (t=0/60/180s); informs whether the in-`Book` re-fetch + immediate-commit pattern is sufficient or whether explicit re-lock is needed.
- **Idempotency-Key header support.** U1 probes; if supported, `Book(...)` accepts a key parameter and the filesystem-advisory-lock fallback is belt-and-suspenders.
- **Booking auth requirements beyond imported cookies.** U1 enumerates and captures field names (NEVER values). If browser JS execution is required (Gate 2), U2/U3 scope-expand to chromedp-attach OR the affected network is dropped from v0.2.

---

## Implementation Units

- U1. **Live chrome-MCP discovery + capture for both networks**

**Goal:** Capture concrete request/response shapes for `book`, `cancel`, and `list-upcoming-reservations` against the live OT and Tock origins via chrome-MCP. Document field names, auth requirements, error shapes, and cancellation-deadline conventions. Output: schema-comment blocks ready to drop into U2 and U3's `booking.go` files.

**Requirements:** R1, R2, R4, R5, R8, R11 (gating; subsequent units encode discoveries)

**Dependencies:** None (entry point)

**Files:**
- No code files created in this unit. Outputs land as schema-comment blocks committed in U2 (`internal/source/opentable/booking.go`) and U3 (`internal/source/tock/booking.go`). Working notes during execution may live in a scratch file but are not committed.

**Approach:**
- Open chrome-MCP, log into OT and Tock with the user's existing browser session (cookies imported via `auth login --chrome` are the same the CLI uses).
- For each network, walk through three flows in chrome-MCP, intercepting XHRs and capturing request URL, method, headers, body, and response shape:
  1. List upcoming reservations (browse to the user's account/upcoming-reservations page; may be SSR-hydrated `$REDUX_STATE` rather than XHR for Tock; an XHR for OT).
  2. Book a free reservation (free OT venue or non-prepay Tock venue) — capture the full sequence including any pre-book lock or create-cart calls. Capture cancellation-deadline location in the response.
  3. Cancel that reservation (capture endpoint + ID handling).
  4. Trigger an error case where possible (e.g., book a slot that's already taken, or use an invalid party size) to capture the slot-taken-vs-other-4xx discriminator.
- Document each network's shapes in the schema-comment block destined for U2/U3's `booking.go`: endpoint URL/method, required headers (including any CSRF/JWT/build-number conventions; field NAMES only, never values), request body field-by-field, response body field names for confirmation/reservation ID/cancellation deadline, error response patterns, slot-taken discriminator, and slot-token TTL.
- **Slot-token TTL measurement.** For each network, capture availability and attempt book at t=0s, t=60s, t=180s. Record the failure boundary so U2/U3 know the actual TTL. Determines whether `book`'s internal availability re-fetch is sufficient or whether explicit relock is needed for v0.2.
- **Idempotency-Key probe.** Try sending an `Idempotency-Key: <uuid>` header on book and observe whether the network honors it (same key → same reservation, idempotent retries). If supported, `Book(...)` accepts an idempotency key parameter and the filesystem-advisory-lock fallback is belt-and-suspenders rather than the only line of defense.

**Discovery gates (any single trigger forces plan re-scope, NOT silent absorption):**

- **Gate 1 (lock+commit required for free Tock).** If the captured Tock free-reservation flow shows a separate slot-lock POST followed by a confirm POST (the lock+commit pattern explicitly deferred to v0.3 in origin), STOP. Do not silently grow U3 to chain them. Surface a plan re-scope: either expand v0.2 to include lock+commit (and update Scope Boundaries) or defer Tock booking entirely to v0.3 alongside payment.
- **Gate 2 (CSRF/auth requires JS execution).** If write paths require auth material (CSRF tokens, X-Tock-Authorization JWTs, build-number nonces) that kooky-imported cookies cannot reproduce without browser JS execution, STOP. Either the affected network's booking is implementable behind chromedp-attach (mirroring `chrome_avail.go`) — and U2/U3 scope-expand to include the chromedp helper — or v0.2 ships only the network whose write paths work with cookies alone.
- **Gate 3 (OT WAF blocks booking opname).** If OT's booking persisted-query is blocked by Akamai's WAF allowlist (the same class of failure that affects `RestaurantsAvailability` per v0.1.11–v0.1.13 patches), STOP. Either chromedp-attach fallback for booking is in scope for U2 (mirroring `chrome_avail.go`'s pattern), or OT booking ships only via the chromedp path with a clear documented requirement.
- **Gate 4 (slot-token TTL too short).** If TTL < ~60s on either network, the agent → CLI → network round-trip may not fit. Either accept higher first-attempt-failure rate (and document in README) or scope-expand to add an explicit re-lock-just-before-commit step (which is the v0.3 lock-flow shape, leaking into v0.2).

**Patterns to follow:**
- This session's SearchCity capture pattern: open the SPA URL, watch network panel, decode `$REDUX_STATE` for SSR-rendered data, filter XHRs to `/api/v2/consumer/...` paths.
- `internal/source/tock/calendar.go` schema-comment block for inline-documenting captured shapes.

**Test scenarios:**
- Test expectation: none — discovery work. Verification is the schema-comment blocks landing in U2/U3.

**Verification:**
- Each of OT and Tock has a documented shape for: book request, book response (with confirmation ID + cancellation deadline location), cancel request, cancel response, list-upcoming-reservations request, list-upcoming-reservations response.
- Slot-taken-vs-auth-vs-network-error discriminator identified for each network. Preferred discriminator hierarchy: HTTP status code → GraphQL error extension code → error-message substring (last resort, with a fallback canary error when an unrecognized 4xx body shape arrives).
- Book auth requirements (CSRF, JWT, headers) enumerated and confirmed reachable via fresh kooky-imported cookies (no manual cookie-paste needed) — OR Gate 2 has fired and the plan has been re-scoped.
- Slot-token TTL measured for each network — OR Gate 4 has fired and the plan has been re-scoped.
- Idempotency-Key support probed for each network's book endpoint — outcome documented (supported / not-supported) for U4 to decide between Idempotency-Key + lock or filesystem-lock-only.
- All four discovery gates have been evaluated explicitly; none have fired silently.

---

- U2. **OpenTable booking client (`Book`, `Cancel`, `ListUpcomingReservations`)**

**Goal:** Implement OT-side network methods on `opentable.Client` for `Book`, `Cancel`, and `ListUpcomingReservations`, using the GraphQL persisted-query path established by `RestaurantsAvailability`. Includes typed error categories (slot-taken, auth-failed, payment-required, network) feeding R10's CLI sanitization.

**Requirements:** R1, R2, R4, R5, R8, R10, R11

**Dependencies:** U1

**Files:**
- Create: `internal/source/opentable/booking.go`
- Create: `internal/source/opentable/booking_test.go`

**Approach:**
- Add three methods to `*Client`: `Book(ctx, slug, date, time string, party int, lat, lng float64) (*BookResponse, error)`, `Cancel(ctx, reservationID string) (*CancelResponse, error)`, `ListUpcomingReservations(ctx) ([]UpcomingReservation, error)`. The book signature takes only the user-facing parameters; the slot-token is fetched internally by `Book` (via `RestaurantsAvailability`) and used in the immediately-following book POST — the slot-token never appears in the CLI surface or in MCP tool-call arguments.
- All three methods route through the existing `gqlCall` (or REST helper if U1 reveals non-GraphQL endpoints) so they inherit `do429Aware`, cooldown checks, and Akamai-cookie freshness. **If U1's Gate 3 fired** (booking opname blocked by WAF), `Book` routes through a chromedp-attach fallback mirroring `chrome_avail.go`.
- Typed error categorization: define `ErrSlotTaken`, `ErrPaymentRequired`, `ErrAuthExpired` as sentinel errors. `Book` wraps the discriminator pattern from U1 to return one of these (or a generic `*BotDetectionError` for cooldown). When the discriminator pattern is error-message substring (last-resort hierarchy), `Book` ALSO returns a sentinel canary error when an unrecognized 4xx body shape arrives — surfaces in U6 dogfood as a known-shape signal rather than silent miscategorization.
- **Retry semantics.** `Book` does NOT auto-retry on any HTTP response received (any 2xx/4xx/5xx). For connection-reset / read-timeout / TLS errors before response headers arrive: at-most-once retry IF U1 confirms an Idempotency-Key header is supported by OT's endpoint; otherwise return a typed "ambiguous network failure" error with a "verify on opentable.com before re-running" hint. `Cancel` uses `do429Aware` as-is (one 429 retry total via the shared helper); no extra retry layer.
- Raw error chains may include response-body fragments containing session cookies, persisted-query hashes, or PII. Network-client errors carry these in the wrapped error; the CLI layer (U4) sanitizes before stdout output. **Network clients do NOT pre-sanitize** — the typed error category + safe metadata is the boundary contract.
- Schema-comment block at top of `booking.go` documents the captured persisted-query hashes (operation names, hash values), GraphQL operation names, request/response field IDs, header names (NOT values), and the slot-taken discriminator from U1.

**Patterns to follow:**
- `internal/source/opentable/client.go` `gqlCall`, `RestaurantsAvailability` for GraphQL shape.
- `internal/source/opentable/cooldown.go` `BotDetectionError` for typed-error pattern.
- `internal/source/tock/calendar.go` schema-comment block for inline schema docs.

**Test scenarios:**
- Happy path (book): `Book(ctx, validRequest)` against an `httptest.Server` returning a captured success body produces a `*BookResponse` with non-empty `ReservationID`, `CancellationDeadline`, `RestaurantName`. Covers AE1.
- Happy path (cancel): `Cancel(ctx, "R12345")` against a fixture returns `*CancelResponse` with non-zero `CanceledAt`. Covers AE5.
- Happy path (list-upcoming): returns the user's upcoming reservations as a typed slice with `RestaurantSlug`, `Date`, `Time`, `Party`, `ReservationID`.
- Edge case: `Book` request with empty/zero slot-token → returns a typed validation error before firing HTTP.
- Edge case: `ListUpcomingReservations` with zero results → returns empty slice, not nil.
- Error path: book request returns OT's "slot taken" error → method returns `ErrSlotTaken`. Covers AE6.
- Error path: book request returns OT's "payment required" error (e.g., paid-experience venue) → method returns `ErrPaymentRequired`. Covers AE4.
- Error path: HTTP 403 with bot-detection markers → returns `*BotDetectionError`, cooldown disk file is updated.
- Error path: malformed response body → returns parse error wrapped with context, not silent empty success.
- Error path: ctx cancellation mid-request → context-aware abort.

**Verification:**
- Unit tests pass against fixture httptest server.
- Schema-comment block at top of `booking.go` matches captured shapes from U1.
- `go vet`, `go build`, `go test ./internal/source/opentable/` all clean.

---

- U3. **Tock booking client (`Book`, `Cancel`, `ListUpcomingReservations`)**

**Goal:** Implement Tock-side network methods on `tock.Client` mirroring U2's surface. Uses the existing `FetchReduxState` for SSR-hydrated data (likely upcoming-reservations) and `do429Aware` POSTs for book/cancel. Includes typed error categories feeding R10's CLI sanitization.

**Requirements:** R1, R2, R4, R5, R8, R10, R11

**Dependencies:** U1

**Files:**
- Create: `internal/source/tock/booking.go`
- Create: `internal/source/tock/booking_test.go`

**Approach:**
- Same method signatures as U2 (`Book(ctx, slug, date, time, party, lat, lng) (*BookResponse, error)`, `Cancel(ctx, reservationID) (*CancelResponse, error)`, `ListUpcomingReservations(ctx) ([]UpcomingReservation, error)`) but on `*tock.Client`. Slot token fetched internally via `SearchCity` or per-venue calendar.
- `ListUpcomingReservations` likely uses `c.FetchReduxState(ctx, "/profile/upcoming")` (or whatever path U1 surfaces) and walks `state.purchase.upcoming` (or equivalent) — same JSON-extract-from-SSR pattern as `search.go`. **Sentinel error when the expected SSR slice is missing** (Tock SPA-refactor canary, mirroring `search.go`'s pattern). The CLI layer treats this sentinel as abort, NOT empty-result.
- `Book` posts to whatever endpoint U1 captures (Tock has used `/api/consumer/book`, `/api/v2/consumer/book`, or similar in adjacent surfaces). **If U1's Gate 1 fired** (lock+commit required for free Tock), this unit is scope-blocked — do NOT silently chain lock+commit; the plan re-scope already happened at U1 gate.
- Typed error categorization mirrors U2 (`ErrSlotTaken`, `ErrPaymentRequired`, `ErrAuthExpired`); same canary-on-unrecognized-shape rule when discriminator is substring-based.
- Same retry semantics as U2: no auto-retry on response-received; conditional retry on connection-reset only if U1 confirms Idempotency-Key. `Cancel` uses `do429Aware` as-is.
- Same error-chain handling: network client returns typed errors with raw chains; sanitization happens at the CLI boundary, not here.
- Schema-comment block at top of `booking.go` documents the captured endpoints, field IDs, JSON paths, and discriminator pattern from U1.

**Patterns to follow:**
- `internal/source/tock/calendar.go` for hand-rolled wire-format / inline schema docs.
- `internal/source/tock/search.go` for `FetchReduxState` + JSON-subtree-extraction pattern.
- `internal/source/tock/client.go` `do429Aware` for shared retry/cooldown.

**Test scenarios:**
- Happy path (book): `Book(ctx, validRequest)` against fixture httptest returns `*BookResponse` with `ReservationID`, `CancellationDeadline`. Covers AE1.
- Happy path (cancel): `Cancel(ctx, validID)` returns `*CancelResponse`. Covers AE5.
- Happy path (list-upcoming): `ListUpcomingReservations` parses fixture HTML/JSON, returns typed slice.
- Edge case: empty slot-token → typed validation error before HTTP.
- Edge case: `ListUpcomingReservations` against a `$REDUX_STATE` present and hydrated but `state.purchase.upcoming` (or equivalent) is an empty array → returns empty slice, no error (genuinely no upcoming reservations).
- Edge case: `ListUpcomingReservations` against `$REDUX_STATE` missing the expected slice entirely (Tock SPA-refactored the path) → returns a typed `ErrUpcomingShapeChanged` sentinel error; does NOT return empty slice (which would silently break the idempotency guarantee). Same canary pattern as `search.go`.
- Edge case: `cuisines`-style polymorphic field in the upcoming-reservations response → handle string-or-array variants.
- Error path: Tock's "slot taken" response → returns `ErrSlotTaken`. Covers AE6.
- Error path: Tock prepay-required venue → returns `ErrPaymentRequired`. Covers AE4.
- Error path: HTTP 4xx/5xx → wrapped error with context.
- Error path: Tock returns a recognized 4xx but with an unrecognized body shape (shape drift) → returns `ErrCanaryUnrecognizedBody` so the discriminator drift surfaces loudly rather than silently miscategorizing.
- Error path: ctx cancellation → context-aware abort.

**Verification:**
- Unit tests pass against fixture httptest server.
- Schema-comment block matches captured U1 shapes.
- `go vet`, `go build`, `go test ./internal/source/tock/` all clean.

---

- U4. **`book` CLI command with idempotency orchestration**

**Goal:** Add the top-level `book` command. Parses network-prefixed slug, validates required args inside RunE (after safety guards), acquires per-key filesystem advisory lock, runs idempotency pre-flight with normalization, applies the verify-mode floor + `TRG_ALLOW_BOOK` gate, dispatches to network client, returns sanitized agent-friendly JSON.

**Requirements:** R1, R2, R3, R5, R6, R8, R9, R10, R11, R12, R13, R14

**Dependencies:** U2, U3

**Files:**
- Create: `internal/cli/book.go`
- Create: `internal/cli/book_test.go`
- Modify: `internal/cli/root.go` (register `newBookCmd(flags)`)

**Approach:**
- Cobra command shape: `book <network>:<slug> --date YYYY-MM-DD --time HH:MM --party N [--dry-run]`. Positional arg required (cobra `Args: cobra.ExactArgs(1)`); flags **NOT** marked required via Cobra (would conflict with verifier's `--dry-run` probe — see R14). Validation happens in `RunE` after the safety guards.
- **Guard order in `RunE` (each short-circuits before the next):**
  1. **Verify-mode floor (R12):** if `cliutil.IsVerifyEnv()` returns true, build a deterministic dry-run envelope from input args and return immediately. Don't reach the network.
  2. **Required-arg validation (R14):** if any of `--date`/`--time`/`--party` is empty/zero, return a typed JSON `{"error": "missing_required_args", ...}` with exit code 2.
  3. **`<network>:<slug>` parse:** invalid prefix → typed JSON error.
  4. **Acquire filesystem advisory lock** (key `sha256(network|slug|date|time|party)`) BEFORE pre-flight runs. If lock acquisition times out (e.g., another process holds it), return a typed `{"error": "concurrent_invocation", "hint": "another book is in flight; retry after"}`. Released after step 8 regardless of outcome.
  5. **Idempotency pre-flight (R5, R13):** call `ListUpcomingReservations` for the network. Apply normalized comparison (time → 24h `HH:MM`, slug → lowercase trim, party → int) against (slug, date, time, party). If match: emit `{"matched_existing": true, "reservation_id": ..., ...}` JSON and exit 0. Pre-flight failure (network error, sentinel-shape-changed) aborts: emit typed error and exit non-zero; do NOT fall through to book.
  6. **Dry-run / commit decision:** if `--dry-run` set OR `TRG_ALLOW_BOOK` unset, build the would-book envelope and return. Pre-flight match-status is included in the envelope. When triggered by missing env var, include hint `"set TRG_ALLOW_BOOK=1 to commit"`.
  7. **Commit:** call the network's `Book(...)`. Token is fetched internally by the network client; CLI does not handle slot tokens.
  8. **Result mapping:** on success, emit confirmation JSON. On typed error, map and **sanitize**: `ErrSlotTaken` → `{"error": "slot_taken", "hint": "try `earliest` for a fresh slot"}`; `ErrPaymentRequired` → `{"error": "payment_required", "hint": "Tock prepaid / OT paid experiences are deferred to v0.3"}`; `*BotDetectionError` → `{"error": "bot_detection_cooldown", "retry_after": "..."}`; `ErrCanaryUnrecognizedBody` → `{"error": "discriminator_drift", "hint": "API error shape may have changed; please report"}`; ambiguous-network-failure → `{"error": "network_error_ambiguous", "hint": "verify on opentable.com / exploretock.com before re-running"}`; generic → `{"error": "network_error"}` (NO raw error chain in `details`; raw chain logged to stderr only).
- **Slot token never touches the CLI surface.** Network client's `Book` re-fetches availability and uses the fresh token internally. No `--slot-token` flag, no slot token in shell history or MCP logs.
- Cobra `Annotations`: `mcp:read-only` is NOT set (write command); per AGENTS.md default, MCP hosts will prompt. `pp:typed-exit-codes` set to `0,2` so verifier accepts non-zero exits as control flow when commit is gated/blocked.
- Confirmation JSON shape includes: `network`, `reservation_id`, `restaurant_name`, `restaurant_slug`, `date`, `time`, `party`, `cancellation_deadline` (or null + `note` referring to venue policy), `matched_existing` (bool), `source` ("book" or "matched_existing"). Fields are stable; nullable when unknown.

**Patterns to follow:**
- `internal/cli/goat.go` for top-level transcendence-command shape, agent-friendly JSON, prefix routing.
- `internal/cli/earliest.go` for `--no-cache` env-var override pattern (mirror as `TRG_ALLOW_BOOK` handling), agent-friendly JSON output, network dispatch.
- Existing `printJSONFiltered` helper for JSON emission with `--agent` defaults.

**Test scenarios:**
- Happy path: network=tock, no existing matching reservation, `TRG_ALLOW_BOOK=1` set, `Book` call mocked to return success → JSON output contains `matched_existing: false`, valid `reservation_id`, `cancellation_deadline`. Covers AE1.
- Happy path: idempotency match, `ListUpcomingReservations` mock returns a matching reservation → JSON output contains `matched_existing: true`, `Book` is NOT called. Covers AE2.
- Edge case: `--dry-run` set → pre-flight (`ListUpcomingReservations`) DOES run (read-only, safe), `Book` does NOT run, dry-run envelope reflects whether pre-flight found a match. Covers AE3, R9.
- Edge case: `TRG_ALLOW_BOOK` unset, no `--dry-run` → pre-flight runs, returns dry-run envelope plus enable hint, `Book` not called. Covers AE3, R6.
- Edge case: `PRINTING_PRESS_VERIFY=1` set (verifier mock-mode), `TRG_ALLOW_BOOK=1` also set → CLI returns dry-run envelope built from args alone, NEITHER `ListUpcomingReservations` nor `Book` fires. Covers R12.
- Edge case: missing required arg (e.g., no `--date`) → typed JSON `{"error": "missing_required_args"}`, exit code 2. Validation runs after the safety guards (R14), not via Cobra `MarkFlagRequired`.
- Edge case: malformed `<network>:<slug>` (no colon, unknown network prefix) → typed JSON error, exit code non-zero.
- Edge case: idempotency pre-flight fails with `*BotDetectionError` → returns the error to the user; does NOT fall through and book without checking. Pre-flight failure is an abort, not a skip.
- Edge case: idempotency normalization (R13) — pre-flight returns a reservation with time `"7:00 PM"` while the book request is `--time 19:00` → normalized comparison matches; `matched_existing: true` is returned. Verifies the normalization rule, not just exact-string match.
- Edge case: concurrent invocation — second `book` process attempts to acquire the same advisory lock while first is mid-flight → second gets `{"error": "concurrent_invocation"}` and aborts; first completes normally. Validates the lock prevents race-double-book.
- Error path: `Book` returns `ErrSlotTaken` → JSON `{"error": "slot_taken", ...}`, exit non-zero. Covers AE6.
- Error path: `Book` returns `ErrPaymentRequired` → JSON `{"error": "payment_required", "hint": "v0.3 work"}`, exit non-zero. Covers AE4.
- Error path: `Book` returns `*BotDetectionError` → JSON includes `retry_after`, exit non-zero.
- Integration: full flow with `httptest.Server` standing in for OT (and Tock) — assert request URLs, headers, and body match captured shapes; assert response JSON matches confirmation shape.

**Verification:**
- All test scenarios above pass.
- `--help` lists the command and the three required flags.
- `--dry-run` and missing-env-var paths are observably indistinguishable from each other in JSON output except for the hint string.
- `printing-press verify` passes for the new command.

---

- U5. **`cancel` CLI command**

**Goal:** Add the top-level `cancel` command. Parses network-prefixed reservation ID, applies the verify-mode floor (the only safety check on cancel), dispatches to network client's `Cancel(...)`, returns sanitized JSON confirmation. No `TRG_ALLOW_BOOK` gate (recovery action per R7); `IsVerifyEnv` floor is the load-bearing safety net.

**Requirements:** R4, R7, R10, R11, R12

**Dependencies:** U2, U3

**Files:**
- Create: `internal/cli/cancel.go`
- Create: `internal/cli/cancel_test.go`
- Modify: `internal/cli/root.go` (register `newCancelCmd(flags)`)

**Approach:**
- Cobra: `cancel <network>:<reservation-id>`. Single positional arg; no flags.
- **Verify-mode floor (R12) is the FIRST guard in `RunE`.** If `cliutil.IsVerifyEnv()` returns true, build a deterministic dry-run envelope (would-cancel: network, reservation-id) and return immediately. This is the only safety check on cancel since R7 leaves it ungated by `TRG_ALLOW_BOOK`. Critical: cancel is irreversible AND ungated, so the verify-mode floor is the load-bearing safety net for `printing-press verify` mock-mode subprocesses.
- Parse `<network>:<reservation-id>` via the same prefix helper U4 uses; invalid prefix → typed JSON error.
- Network dispatch: opentable / tock as in U4.
- Cancel JSON shape: `network`, `reservation_id`, `canceled_at`, `restaurant_slug` (when network returns it).
- Typed-error mapping mirrors U4 with sanitization at the CLI boundary: cancel-specific errors (e.g., past-deadline) get their own JSON shape `{"error": "past_cancellation_window", "hint": "..."}`. Generic mapping returns `{"error": "network_error"}` with NO raw error chain in `details`; raw chain logged to stderr only. Same `ErrCanaryUnrecognizedBody` handling as U4.
- No `TRG_ALLOW_BOOK` gate per R7. `IsVerifyEnv` floor (above) is the safety net.
- Cancel uses `do429Aware` as-is (one 429 retry shared with reads); no extra retry layer.
- Cobra `Annotations`: `mcp:read-only` not set (write command); per AGENTS.md default, MCP hosts will prompt. `pp:typed-exit-codes` set to `0,2`.

**Patterns to follow:**
- U4's command shape (mirror as much as possible).
- `internal/cli/goat.go` JSON output and prefix routing.

**Test scenarios:**
- Happy path: `cancel opentable:R12345` → fixture `Cancel` mock returns success → JSON output with non-zero `canceled_at`. Covers AE5.
- Happy path: `cancel tock:T67890` → same shape from Tock client.
- Edge case: `PRINTING_PRESS_VERIFY=1` set → CLI returns dry-run envelope, `Cancel` does NOT fire against any network. Covers R12. **Critical safety test** — this is the only floor on cancel.
- Edge case: malformed argument (no colon, unknown network prefix) → typed JSON error.
- Error path: reservation ID not found → typed JSON error (`{"error": "not_found"}`).
- Error path: past cancellation window → typed JSON error with hint.
- Error path: `*BotDetectionError` → JSON includes `retry_after`.
- Error path: generic 4xx with raw response body containing PII or session cookies → CLI emits sanitized `{"error": "network_error"}` to stdout; raw chain to stderr only. Verifies error sanitization at CLI boundary.

**Verification:**
- All test scenarios above pass.
- `--help` documents the single argument.
- `printing-press verify` passes.

---

- U6. **PATCH manifest, README mention, live dogfood, ship**

**Goal:** Append v0.2.0 entry to `.printing-press-patches.json`, document the new commands in README, live-dogfood the matrix from origin AE1–AE6, and confirm the cross-network booking lifecycle works end-to-end on both networks.

**Requirements:** R1–R14 (verification gate)

**Dependencies:** U4, U5

**Files:**
- Modify: `.printing-press-patches.json` (append v0.2.0 entry under `cross-network-source-clients`; add the four new files to the `files` list).
- Modify: `README.md` (one-paragraph mention of `book` and `cancel` under the existing OT/Tock section, plus the `TRG_ALLOW_BOOK` knob in the power-user section).

**Approach:**
- v0.2.0 entry (note the minor-version bump from v0.1.x — this is the first feature that adds write-to-network surfaces, justifying v0.2.0 over v0.1.16): summarize the four new files, the typed errors, the env-var guardrail, and the dogfood transcript.
- README: mention `book opentable:<slug> --date YYYY-MM-DD --time HH:MM --party N` and `cancel <network>:<id>` under "OpenTable / Tock". Add `TRG_ALLOW_BOOK` to the power-user knobs section.
- **Pre-flight check.** Before starting the dogfood matrix, run `book --dry-run` for both networks to confirm `ListUpcomingReservations` returns empty (no leftover reservations from prior dogfood runs). If non-empty, cancel the leftovers first.
- **Live dogfood matrix (test budget: ≤2 successful per platform, with cancel between):**
  1. `book opentable:<known-free-venue> --date <date> --time <time> --party <N>` with `TRG_ALLOW_BOOK=1` — first OT book; verify confirmation JSON shape (matches R10) and that idempotency-hit is false.
  2. Same command without `TRG_ALLOW_BOOK` — verify dry-run envelope (R6), no second commit.
  3. Same command WITH `TRG_ALLOW_BOOK=1` again — verify idempotency hit: `matched_existing: true`, no second book against OT (R5).
  4. `cancel opentable:<reservation-id>` — verify cancellation. Post-cancel: re-run `book --dry-run` → confirm reservation no longer appears in upcoming.
  5. Optional second OT book + cancel cycle — verifies post-cancel rebook works. This is the second of 2 budgeted OT bookings.
  6. Same flow for Tock with a known free (non-prepay) venue.
  7. `PRINTING_PRESS_VERIFY=1 book opentable:<venue> --date X --time Y --party Z` with `TRG_ALLOW_BOOK=1` also set → verify dry-run envelope; no `ListUpcoming` or `Book` fires (R12). Verifier-mode floor exercise — NOT counted against test budget.
  8. `PRINTING_PRESS_VERIFY=1 cancel opentable:<id>` → verify dry-run envelope; no `Cancel` fires (R12). Floor exercise for the ungated cancel path.
  9. Concurrent invocation: fire two `book` processes with the same args within ~100ms → first commits, second receives `concurrent_invocation` error, no double-book (lock test). NOT counted against test budget if the second never commits.
  10. Paid-venue rejection: choose a Tock prepay venue whose paid status is observable from `SearchCity` venue metadata BEFORE submitting (so the rejection happens in the network client without the POST firing). With `TRG_ALLOW_BOOK=1`, run `book tock:<prepay-venue> --...` → verify `ErrPaymentRequired` JSON without a commit. If pre-POST detection is not possible per U1's findings, move this test into a fixture-only unit test rather than live.
  11. Sanitization spot-check: trigger a 4xx in any error path during the dogfood (e.g., expired session) → verify stdout JSON shows `{"error": "<category>"}` only, no raw cookies / IDs / response bodies. (Stderr may contain raw chain.)

**Patterns to follow:**
- `.printing-press-patches.json` v0.1.15 entry (this session's SearchCity work) for entry shape and detail level.

**Test scenarios:**
- Test expectation: none — manual dogfood. Unit tests in U2–U5 cover correctness.

**Verification:**
- Patches manifest contains v0.2.0 entry referencing the four new files and the lifecycle outcomes.
- Dogfood transcript pasted into the PR description shows: free OT book + cancel + idempotency-hit + dry-run + payment-required-error all working; same for Tock.
- README mentions `book`, `cancel`, and `TRG_ALLOW_BOOK`.
- Total successful bookings during shipping: ≤2 per platform; account remains in good standing (no rate-limit warnings, no bot-detection escalation).

---

## System-Wide Impact

- **Interaction graph:** New `book` / `cancel` commands fire network POSTs that the existing read paths don't. The OT WAF resilience envelope (cooldown, AdaptiveLimiter) wraps writes too — book commits respect cooldown for fast-fail. The Tock side uses `do429Aware` similarly. `auth login --chrome` cookies are the sole auth source. **If U1 Gate 3 fired** (OT WAF blocks booking opname), `book` routes through chromedp-attach mirroring `chrome_avail.go`.
- **Concurrency safety:** Filesystem advisory lock in U4 keyed on `(network, slug, date, time, party)` serializes pre-flight + book across processes. Lock acquisition timeout returns a typed `concurrent_invocation` error rather than silently waiting. If U1 surfaces an Idempotency-Key header on book endpoints, `Book(...)` accepts a key parameter and the lock becomes belt-and-suspenders.
- **Verify-mode safety:** `cliutil.IsVerifyEnv()` floor in both `book` and `cancel` short-circuits to dry-run regardless of `TRG_ALLOW_BOOK`. This is the load-bearing safety net for `printing-press verify` mock-mode subprocesses, especially for cancel (which R7 leaves ungated by `TRG_ALLOW_BOOK`).
- **Error propagation and sanitization:** Network clients return typed errors with raw error chains. The CLI layer (U4/U5) maps typed errors to a sanitized JSON shape (category + safe one-line message); raw chains never appear in stdout `details`. Raw chains may be emitted to stderr for debugging. JSON consumers see one error category at a time; no silent zero-result success.
- **State lifecycle risks:** Book is a write. Idempotency pre-flight + filesystem lock prevent double-booking on sequential retry AND concurrent invocation. The remaining ambiguity is connection-reset before response headers — handled by the at-most-once retry only when Idempotency-Key is supported (per U1); otherwise surfaces as a typed `network_error_ambiguous` error pointing the user at the website to verify before re-running.
- **API surface parity:** Both `book` and `cancel` exposed as Cobra commands AND as MCP tools (via `cobratree` walker). MCP annotations: neither sets `mcp:read-only` (both write). Neither sets a custom `destructiveHint` annotation — the cobratree walker doesn't read one today, and the AGENTS.md default ("missing annotation = could write or delete") makes MCP hosts prompt on every call. Conservative-by-default — adding suppression of the prompt would require typed MCP tool entries (out of v0.2 scope) and the wrong claim would be worse than missing.
- **Integration coverage:** End-to-end book → cancel cycle is exercised in U6 dogfood; unit tests in U2–U5 use fixture servers and don't touch real networks.
- **Unchanged invariants:** `goat`, `earliest`, `watch`, `drift`, `auth`, `restaurants`, `availability`, `sync`, `search` all unchanged. The OT/Tock client packages add new methods but don't modify existing ones. `internal/cli/root.go` gains two `AddCommand` calls; nothing else changes there.

---

## Risks & Dependencies

| Risk | Mitigation |
|------|------------|
| U1 discovery reveals Tock free reservations require lock+commit (origin defers this to v0.3). | **Hard scope-block.** U1 Gate 1 fires — STOP and re-scope. Either expand v0.2 (and update Scope Boundaries to remove the lock-then-commit deferral) or defer Tock booking entirely to v0.3 alongside payment. Do NOT silently chain lock+commit in U3. |
| OT WAF allowlist blocks the booking persisted-query opname (same class as `RestaurantsAvailability` per v0.1.11–v0.1.13). | **Hard scope-block at U1 Gate 3.** Either chromedp-attach fallback for `Book` is in scope for U2 (mirroring `chrome_avail.go`), or OT booking ships only via the chromedp path with a clear documented requirement. The plan's existing assumption (gqlCall + cookie freshness suffices) is invalidated by prior history; verify in U1 before U2 begins. |
| Booking write path requires CSRF / JWT material kooky cookies cannot reproduce without browser JS execution. | **Hard scope-block at U1 Gate 2.** Either chromedp-attach is wired for the affected network's booking, or v0.2 ships only the network whose write paths work with cookies alone. |
| Slot-token TTL too short for the agent → CLI → network round-trip. | **U1 Gate 4** measures TTL explicitly. If TTL < ~60s, accept higher first-attempt-failure rate (and document) OR scope-expand to add an explicit re-lock-just-before-commit step (which is the v0.3 lock-flow shape leaking into v0.2). `book`'s internal availability re-fetch narrows the race window to seconds, mitigating most cases. |
| Concurrent agent invocations (e.g., agent fires `book` twice within 100ms) bypass idempotency pre-flight and double-book. | Filesystem advisory lock keyed on `(network, slug, date, time, party)` in U4 serializes pre-flight + book across processes. Concurrent attempt aborts with typed `concurrent_invocation` error. If U1 surfaces an Idempotency-Key header, `Book` also passes that for cross-process safety belt-and-suspenders. |
| Idempotency pre-flight returns reservations in a format mismatching the request (e.g., time `7:00 PM` vs request `19:00`); naive string-compare fails and double-books. | R13 normalization rule: time → 24h `HH:MM`, slug → lowercase trimmed, party → int. U2/U3 fixtures explicitly include polymorphic-format examples. Tested in U4 normalization edge case. |
| Account ban during dogfood from too many bookings. | Hard-cap test-budget at 2 successful per platform with cancel between. `TRG_ALLOW_BOOK=1` env-var prevents accidental commit during dev; `IsVerifyEnv` floor blocks verifier mock-mode regardless. Test cases use fixture httptest servers; only U6 dogfood touches real networks. Pre-dogfood check: assert `ListUpcomingReservations` is empty for both networks before starting; post-cancel assert: reservation no longer appears. |
| Verifier mock-mode subprocess fires real book/cancel because `TRG_ALLOW_BOOK=1` leaked from user's shell. | `cliutil.IsVerifyEnv()` floor in both U4 and U5. R12 makes this floor stricter than `TRG_ALLOW_BOOK` and applies to cancel even though cancel is otherwise ungated. |
| Pre-flight idempotency fails (network error, sentinel-shape-changed). | Aborts the book attempt with a clear error — does NOT fall through and book without idempotency check. Otherwise a dropped pre-flight + successful retry equals a double book. |
| Connection-reset / TLS error before response headers arrive — was the request acknowledged or not? | If U1 confirms the network supports an Idempotency-Key header, at-most-once retry IS safe. Otherwise return `network_error_ambiguous` typed error with hint to verify on the website before re-running. Do NOT auto-retry without an idempotency key — risks double-book. |
| Booking response doesn't include cancellation deadline. | Surface `cancellation_deadline: null` with `note: "see venue policy at <URL>"` in JSON; do not synthesize a deadline. v0.3 may add a per-venue policy lookup. |
| OT GraphQL hash drift on book/cancel persisted-queries. | Same pattern as `RestaurantsAvailability` — schema-comment block documents the hash, error path returns a clear `PersistedQueryNotFound` typed error pointing to which hash needs refresh. |
| Tock SPA refactors `state.purchase.upcoming` away. | `ErrUpcomingShapeChanged` sentinel error mirrors `search.go`'s pattern. CLI treats sentinel as abort, NOT empty-result. U6 dogfood is the canary. |
| Slot-taken-vs-other-4xx discriminator is brittle (e.g., error-message-substring drifts when network changes copy). | U1 prefers HTTP status code → GraphQL error extension → message substring (last resort) hierarchy. U2/U3 also surface `ErrCanaryUnrecognizedBody` when an unrecognized 4xx body shape arrives, so drift surfaces loudly rather than silent miscategorization. |
| Auth-cookie freshness for write paths is stricter than reads. | If U1 surfaces this, U2/U3 inherit the existing `RefreshAkamaiCookies` / kooky-import paths. May require new freshness-on-write logic; flag scope expansion if so. |
| Cancel of a reservation after the cancellation window passes returns ambiguous error. | U1 captures the response shape; U2/U3 map to typed `ErrPastCancellationWindow`; CLI surfaces a clear hint. |
| Stdout error JSON leaks raw upstream content (response bodies, session cookies, persisted-query hashes, PII). | R10 mandates sanitized JSON output: typed category + safe one-line message only. Raw error chains stay on stderr. Network clients return raw chains; CLI is the sanitization boundary. Tested via "generic 4xx with raw response body containing PII" scenario in U5 tests (mirror in U4). |
| Paid-venue dogfood step (U6 step 7) initiates partial payment flow before discriminator returns `ErrPaymentRequired`. | Test against a venue whose paid status can be confirmed BEFORE submitting a book POST (e.g., readable from `RestaurantsAvailability` / `SearchCity` venue metadata). If pre-validation isn't possible, move the test to a fixture-only unit test rather than live dogfood. |
| MCP tool annotations for write commands cause permission prompts that interrupt agent flow. | Accepted — prompt-on-every-write is the safe default per AGENTS.md. Neither `book` nor `cancel` sets a custom `destructiveHint` (the walker doesn't read one today and the wrong-claim risk exceeds the prompt-fatigue cost). Suppression of prompts requires explicit typed MCP tool entries — out of v0.2 scope. |

---

## Documentation / Operational Notes

- README mention of `book` + `cancel` commands plus the relevant env knobs (`TRG_ALLOW_BOOK`, `PRINTING_PRESS_VERIFY` floor) in U6.
- `.printing-press-patches.json` v0.2.0 entry under `cross-network-source-clients` (in U6).
- No monitoring / rollout coordination needed — printed CLI; users update by re-installing.
- A cookie-hygiene note in README: booking writes may require freshly-imported Chrome cookies; recommend `auth login --chrome` shortly before booking attempts. (Final shape decided by U1's auth-requirements capture.)
- Note in README that MCP hosts will prompt on every `book` and `cancel` invocation — this is intentional safety, not a bug. Users who want suppressed prompts will need to wait for typed MCP tool entries (out of v0.2 scope).

---

## Sources & References

- **Origin document:** [docs/brainstorms/2026-05-09-booking-flow-free-reservations-requirements.md](../brainstorms/2026-05-09-booking-flow-free-reservations-requirements.md)
- Related code:
  - `internal/source/opentable/client.go` (`gqlCall`, `RestaurantsAvailability`, `Bootstrap`)
  - `internal/source/opentable/cooldown.go` (`BotDetectionError`, exponential backoff)
  - `internal/source/tock/calendar.go` (capture-protos-from-bundle, schema-comment-block-at-top)
  - `internal/source/tock/search.go` (FetchReduxState + JSON-subtree-extraction; this session's most recent precedent)
  - `internal/cli/goat.go`, `internal/cli/earliest.go` (transcendence-command shape, prefix routing, agent-friendly JSON)
- Patches manifest: `.printing-press-patches.json` (v0.1.15 documents the SearchCity work; v0.2.0 added in U6).
