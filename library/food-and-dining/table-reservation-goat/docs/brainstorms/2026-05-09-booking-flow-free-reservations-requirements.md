---
date: 2026-05-09
topic: booking-flow-free-reservations
---

# Booking flow — free reservations on OpenTable and Tock

## Summary

v0.2 adds top-level `book` and `cancel` commands to complete the cross-network reservation lifecycle for free reservations on both OpenTable and Tock. Single-step commit-by-default with verify-before-retry idempotency, gated by an opt-in environment variable so dogfood can't accidentally burn the test budget. Payment / prepayment flows are explicitly deferred to v0.3.

---

## Problem Frame

The CLI's existing transcendence commands (`goat`, `earliest`, `watch`, `drift`) help an LLM agent find and track openings but stop short of actually booking. When the agent surfaces a slot via `earliest` or a `watch tick` hit, the only way to complete the reservation today is for the user to copy the venue URL and book through the website manually. That breaks the "agent completes the loop" UX: the user is pulled out of chat into a browser session, and the agent loses the thread on whether the booking succeeded — including whether retries would double-book.

The technical groundwork for booking now exists: `RestaurantsAvailability` (OT, v0.1.13) and `SearchCity` (Tock, v0.1.15) both return slot tokens that are valid inputs to OT's `make-reservation` and Tock's `BookDetailsExperienceSlotLock`. What's missing is the CLI surface that fires those calls safely under the slot-token race, verifies success before retrying, and provides a matching `cancel` for both the test loop and the agent's full lifecycle authority.

A second pressure: the user's account is the test substrate. Burning more than 1–2 successful bookings per platform during shipping risks being flagged as a bot. The feature must be testable without overrunning that budget, which constrains how the dev workflow exercises live commit.

---

## Actors

- A1. **CLI user** — runs `book`/`cancel` directly in a terminal; expects clear confirmations and an explicit safety story.
- A2. **LLM agent** — invokes `book`/`cancel` on the user's behalf, typically via MCP tool calls inside a chat. The dominant caller in practice; agent-friendly JSON output is the priority surface.
- A3. **OpenTable** — external reservation system; receives `make-reservation` calls and the cancel mutation; returns confirmation or error. Has Akamai bot-detection constraints already handled by existing infrastructure.
- A4. **Tock** — external reservation system; receives slot-lock + book + cancel calls. Has Cloudflare bot-detection already handled by existing infrastructure.

---

## Key Flows

- F1. **Agent-initiated book (free reservation)**
  - **Trigger:** LLM agent calls `book` with explicit network-prefixed venue, date, time, party.
  - **Actors:** A2, A3 or A4
  - **Steps:** (1) CLI runs idempotency pre-flight against the network: list user's upcoming reservations and check for an existing match. (2) If matched, return existing reservation ID flagged as a duplicate hit; do not fire a second book call. (3) If no match and the env-var guardrail is set, fire the network's book call with the slot token. (4) Return confirmation JSON: reservation ID, restaurant info, slot details, cancellation deadline.
  - **Outcome:** A new (or matched-existing) reservation is held under the user's account; agent gets a single JSON response that the user's chat can present.
  - **Covered by:** R1, R2, R3, R5, R6, R8, R10

- F2. **Agent-initiated cancel**
  - **Trigger:** LLM agent calls `cancel` with network-prefixed reservation ID.
  - **Actors:** A2, A3 or A4
  - **Steps:** (1) CLI fires the cancel call against the network. (2) Returns success/failure JSON with the cancellation timestamp.
  - **Outcome:** Reservation is canceled; agent confirms to the user.
  - **Covered by:** R4, R7, R10

- F3. **Dry-run preview**
  - **Trigger:** Caller passes `--dry-run` to `book`, OR the env-var guardrail is unset.
  - **Actors:** A1 or A2
  - **Steps:** (1) Run idempotency pre-flight as in F1. (2) Build the would-book envelope from the slot token and venue metadata. (3) Return the envelope without firing the book call. (4) When triggered by missing env-var, also surface a hint about how to enable commit.
  - **Outcome:** Caller sees what would happen; nothing is booked.
  - **Covered by:** R6, R9

---

## Requirements

**Booking**
- R1. CLI exposes a top-level `book` command accepting network-prefixed venue (`opentable:<slug>` or `tock:<slug>`), with `--date`, `--time`, `--party` as required arguments. No silent defaults that could book the wrong slot on a typo.
- R2. `book` works for free reservations on both OpenTable and Tock. Reservations requiring payment (Tock prepaid tasting menus, OT paid experiences) return a clear typed error pointing to v0.3 rather than a partial commit or silent skip.
- R3. By default, `book` commits the reservation in a single CLI call and returns the confirmation JSON. No mid-call user prompting on the free path.

**Canceling**
- R4. CLI exposes a top-level `cancel` command accepting a network-prefixed reservation ID (`opentable:<id>` or `tock:<id>`). Returns success/failure JSON with the cancellation timestamp.

**Idempotency and safety**
- R5. Before firing a book call, `book` performs a pre-flight check against the user's upcoming reservations on that network. If a reservation matching network + slug + date + time + party already exists, `book` returns the existing reservation ID flagged as an idempotency hit and does not fire a second book call.
- R6. Live commit fires only when the environment variable `TRG_ALLOW_BOOK=1` is set in the calling environment. When unset, `book` returns the dry-run envelope plus a clear hint about how to enable commit. The guardrail is deliberately env-var, not a CLI flag, to prevent tab-completion or shell-history reuse from firing accidentally during dogfood.
- R7. `cancel` is not gated by `TRG_ALLOW_BOOK`. Canceling is a recovery action; gating it would block the operator from undoing a mistake.
- R8. The cancellation-window deadline (e.g., "must cancel by 2026-05-13T18:00 to avoid a no-show fee") is surfaced in the `book` confirmation JSON so the agent and user can decide on cancel timing without a follow-up call.

**Output shape**
- R9. `book --dry-run` returns the would-book envelope (network, slug, date, time, party, slot token if available, expected restaurant info) without firing the book call. Same envelope shape whether the dry-run was explicit or triggered by missing env-var.
- R10. `book` and `cancel` return agent-friendly JSON to stdout with stable field names: confirmation/reservation ID, restaurant slug + name, party, date, time, cancellation deadline (book only), and an idempotency-hit flag. No separate audit-log file in v0.2 — agent or user keeps records via chat history or shell redirect.
- R11. Errors from the network (slot taken between read and write, account not authorized, network 4xx/5xx, bot-detection cooldown) surface as typed error JSON with enough context for the agent to retry, escalate, or abandon. They never result in silent zero-result success.

---

## Acceptance Examples

- AE1. **Covers R1, R3, R5, R6, R10.** Given `TRG_ALLOW_BOOK=1` is set and the user has no existing reservation at Canlis on 2026-05-13 at 19:00 for 4. When the agent runs `book tock:canlis --date 2026-05-13 --time 19:00 --party 4`, the CLI fires Tock's book call, the response contains a confirmation number and cancellation deadline, and the JSON output includes `matched_existing: false`.

- AE2. **Covers R5.** Given the user already has a confirmed reservation at Canlis on 2026-05-13 at 19:00 for 4. When the agent runs `book tock:canlis --date 2026-05-13 --time 19:00 --party 4` (regardless of `TRG_ALLOW_BOOK`), the CLI returns the existing reservation ID with `matched_existing: true` and does not fire a second book call.

- AE3. **Covers R6, R9.** Given `TRG_ALLOW_BOOK` is unset and no matching existing reservation. When the agent runs `book opentable:goldfinch-tavern-... --date 2026-05-13 --time 19:00 --party 4`, the CLI returns the dry-run envelope with a clear hint that the env-var must be set to commit, and no book call is fired.

- AE4. **Covers R2.** Given a Tock venue that requires prepayment (e.g., a tasting-menu venue). When the agent runs `book tock:<paid-venue> --date X --time Y --party Z` with `TRG_ALLOW_BOOK=1`, the CLI returns a typed error identifying the venue as payment-required and pointing at v0.3 — never a partial book.

- AE5. **Covers R4, R7.** Given a confirmed reservation with ID `R12345` on OpenTable. When the agent runs `cancel opentable:R12345` (no `TRG_ALLOW_BOOK` required), the CLI fires the cancel call and returns success JSON with the cancellation timestamp.

- AE6. **Covers R11.** Given the slot has been taken between availability fetch and book commit. When `book` fires and the network returns a slot-unavailable response, the CLI returns a typed error JSON explaining the slot is gone and suggesting `earliest` to find a fresh one. The CLI does not silently report zero results or partial success.

---

## Success Criteria

- An LLM agent in a chat can complete the find → book → confirm loop without context-switching the user to a browser.
- An accidental retry of `book` (network blip, agent re-invocation, transient error) does not produce a duplicate reservation.
- A booking made via the CLI is cancelable via the CLI, closing the test loop without manual website intervention.
- During shipping, the test transcript shows ≤2 successful bookings per platform with cancel between, no account flags, and the full lifecycle proven on both OT and Tock.
- Downstream `ce-plan` can take this requirements doc and produce a plan that doesn't need to invent confirmation UX, idempotency story, or cancel scope.

---

## Scope Boundaries

- Payment / prepayment flows (Tock prepaid tasting menus, OT paid experiences) — separate v0.3 brainstorm.
- Auto-book on `watch tick` hits — explicit non-goal in v0.2; preserves the one-shot intentional trigger model.
- Two-step lock-then-commit pattern — overkill for free reservations; revisit in v0.3 when payment makes locking necessary.
- General "list my upcoming reservations" user-facing command — the pre-flight idempotency lookup is internal-only in v0.2; surface as a user command only if real demand arises.
- Reservation modification (date/time/party change without cancel + rebook) — out of v0.2; cancel + rebook covers the lifecycle.
- Multi-network parallel booking — `book` operates on one network at a time; agent orchestrates if needed.
- Persistent local SQLite of bookings — confirmation JSON to stdout is the surface in v0.2.
- Card-on-file and payment-method management — moot for v0.2; relevant only when payment lands.
- Group bookings, large-party special handling, dietary-restriction propagation — not in v0.2.

---

## Key Decisions

- **Single-step commit-by-default rather than dry-run-by-default or two-step lock-then-commit.** Matches the user's stated need that free reservations should "happen without much back-and-forth." Dry-run remains available via `--dry-run`; locking is deferred to v0.3 when payment makes it genuinely necessary.
- **Test-discipline guardrail as `TRG_ALLOW_BOOK=1` env var rather than a `--commit` flag.** Flags can be tab-completed or sticky in shell history; an env var is a deliberate per-shell gesture that matches the user's "don't burn my test budget by accident" concern.
- **Cancel by reservation ID rather than slug + date + time match.** The network-issued ID is unambiguous and recovers cleanly even if the user has multiple reservations at the same venue or has manually changed something via the website.
- **Idempotency via pre-flight upcoming-reservations check rather than agent-supplied idempotency keys.** Works without requiring agent cooperation, and is safe even if the agent regenerates a key on retry. Adds one round-trip per book call; acceptable for free-reservation latency.
- **No persistent local store of bookings in v0.2.** Agent/user keeps records via chat history or shell redirect. Adding a local store is a v0.3+ consideration if real demand surfaces.
- **No audit-log file in v0.2.** Stdout JSON is the audit. If the user wants persistent logs, they pipe to file.

---

## Dependencies / Assumptions

- OT's book / cancel and Tock's book / cancel endpoints both accept the kooky-imported Chrome session cookies the rest of the CLI already uses for read paths. (Strong assumption — proven for read paths; book/cancel may have additional CSRF requirements that planning needs to confirm.)
- Free OpenTable reservations do not require payment-method-on-file. (Most don't; some "experience" venues do — those are out of scope per R2.)
- Tock free reservations do not require a card-on-file or prepayment. (True for non-tasting-menu venues; v0.2 explicitly avoids prepay venues per R2.)
- Slot tokens returned from `RestaurantsAvailability` (OT) and `SearchCity` (Tock) are valid inputs to the corresponding book calls, with TTLs long enough to survive the agent → CLI → network round-trip.
- Both networks return a stable reservation ID in the book response that's usable directly by their cancel endpoints.
- Cancel endpoints exist on both networks and don't require additional bootstrapping beyond the same session cookies.

---

## Outstanding Questions

### Resolve Before Planning

- (None — all blockers resolved during brainstorm.)

### Deferred to Planning

- [Affects R5][Needs research] What's the most reliable shape of the "list user's upcoming reservations" call on OpenTable? GraphQL operation or REST? Browser-sniff likely needed.
- [Affects R5][Needs research] Same question for Tock — what call returns the user's upcoming reservations?
- [Affects R1][Technical] Exact slot-token-to-book request shape for both networks, including required CSRF headers and persisted-query hashes.
- [Affects R4][Technical] Cancel endpoint shape for OpenTable — GraphQL mutation or REST DELETE? What params does it take?
- [Affects R4][Technical] Cancel endpoint shape for Tock — same question.
- [Affects R8][Needs research] Cancellation-window deadline — returned in the book response or requires a separate fetch?
- [Affects R6][Technical] Should `TRG_ALLOW_BOOK=1` be one-shot (per CLI invocation, the natural env-var behavior) or persistent (e.g., a config-file toggle)? Recommend one-shot per Bash semantics.
- [Affects R10][Technical] Final naming for the idempotency-hit field — `matched_existing`, `idempotent_hit`, etc. Decide against existing JSON shape conventions during planning.
- [Affects R11][Technical] How does the CLI distinguish "slot taken between read and write" from other 4xx errors on each network? Both networks may use generic error codes; planning needs to identify the discriminator.
