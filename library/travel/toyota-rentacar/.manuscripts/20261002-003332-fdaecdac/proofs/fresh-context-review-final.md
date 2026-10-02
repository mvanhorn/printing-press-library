# Toyota Rent a Car fresh-context review

Reviewed 2026-10-02 (JST), by the sole dedicated reviewer. This is a new implementation; the initial baseline was only the handoff and discovery evidence. I reviewed the custom domain, parsers, anonymous client, quote and policy flows, command constructors, exposed CLI/MCP, README/SKILL, spec/absorb manifest, patch index, and authoritative latest live evidence. No implementation files were changed. Review-only regressions were injected with a Go overlay from `/private/tmp/toyota-fresh-review/`.

Round one: **four P2 findings and one nonblocking P3 observation**. The builder has reported a rate-limit fix in its isolated working tree; that fix is not yet verified in this snapshot. The other findings below also need fix re-review.

**Current verdict: round two converged. R1–R5 are resolved; no open review findings or new product regressions remain. The round-one findings below are retained as an audit trail.**

## Standards

### R1 — P2: One-way context validation accepts a different shop with an overlapping name

Location: `internal/toyota/quote.go:276–280` (especially line 277).

The final calculator checks `strings.Contains(actual, expectedShopName)` rather than confirming the complete shop name. A source response naming `From Tokyo Nihonbashi AnnexStore` and `To Hatchobori AnnexStore` passes for the requested Tokyo Nihonbashi and Hatchobori shops. The result then attributes the other route's surcharge to the requested composite IDs with `status: calculated`.

This is a violation of the identity/context contract in `AGENTS.md`: shop identities must remain distinct, and a source context change must not become a valid planning result. The dated quote path correctly compares the four composite-ID parts; the one-way path does not provide an equivalent check.

Reproduction: the review-only `TestReviewOneWayRejectsChangedShopName` replays the normal anonymous sequence using the existing deterministic transport and changes only the final displayed shop names. Running:

```bash
go test -count=1 -overlay=/private/tmp/toyota-fresh-review/overlay.json \
  -run '^TestReviewOneWayRejectsChangedShopName$' ./internal/toyota
```

fails with `accepted a changed route: status=calculated fee=5500 pickup=Tokyo Nihonbashi Shop return=Hatchobori Shop`.

This is a deterministic context-drift reproduction, **not a claim that Toyota currently returns those Annex names**. Normalize the known source wrappers (`From`/`To`, terminal `Store`, whitespace), then require exact full-name equality. Compare echoed composite IDs wherever available, and keep the fee withheld if identity cannot be established. Add a regression with overlapping names in addition to completely different names.

No subjective style or abstraction findings were raised. The first-party origin restriction, in-memory anonymous state, body/request caps, exact dated shop checks, explicit unknown total, and human-only booking boundary are implemented coherently.

## Spec and exposed behavior

### R2 — P2: Live planning commands ignore the user's request-rate ceiling

Locations: `internal/toyota/client.go:62–66`; owned call sites include `internal/cli/rental_options.go:20`, `internal/cli/cars_quote.go:29`, `internal/cli/shops_search.go:26`, `internal/cli/oneway_quote.go:25`, and `internal/cli/toyota_commands.go:84,137`. The advertised global flag is at `internal/cli/root.go:318`.

Every custom live constructor calls `toyota.NewClient()`, whose limiter is fixed to 2 requests/second. A positive root `--rate-limit` is never passed through. This violates a user-selected lower politeness ceiling, through both CLI and mirrored MCP tools.

Live reproduction, with isolated local state and learning disabled:

```bash
TOYOTA_RENTACAR_NO_LEARN=true TOYOTA_RENTACAR_HOME=/private/tmp/toyota-fresh-review-home \
  /private/tmp/toyota-fresh-review/toyota-rentacar-pp-cli rental options \
  --agent --rate-limit 0.01 --timeout 10s
```

Observed exit 0 in **0.623 seconds**, metadata `upstream_requests: 2`, `elapsed_ms: 611`, `response_bytes_read: 52517`. At 0.01 requests/second, the second request should wait about 100 seconds and the operation should instead hit its 10-second deadline after one source request.

Pass the configured rate into the owned Toyota client and preserve the advertised negative/auto, zero/disabled, and positive/hard-ceiling behavior. Test that success responses or source rate-limit headers cannot increase an explicit lower ceiling. Builder reports this fix is implemented; re-review remains pending.

### R3 — P2: Quiet output emits non-identity values for shop detail and class offers

Locations: `internal/cli/toyota_commands.go:89`; `internal/cli/cars_quote.go:45`; `internal/toyota/domain.go:150–159`. The documented contract is `SKILL.md:177`.

Shop detail puts its ID inside `shop` and passes the whole wrapper into the generic output pipeline. Quiet mode sees a wrapper without a top-level identity and selects the only scalar, its caveat:

```bash
toyota-rentacar-pp-cli shops get --id 63601:01V --quiet
```

Live observed stdout (exit 0):

```text
Operating hours do not establish dated vehicle inventory.
```

Expected: `63601:01V`.

Class rows likewise expose `class` but none of the generic identity keys (`id`, `code`, name, etc.). A review overlay passes a normal one-class `QuoteResult` through exactly the command's output pipeline thirty times. It observed `11990`, `5`, `C1`, `Rental Price`, `available`, and `false` for the same C1 row, depending on Go map iteration. Run:

```bash
go test -count=1 -overlay=/private/tmp/toyota-fresh-review/overlay.json \
  -run '^TestReviewQuietClassIdentity$' ./internal/cli
```

This can break scripts that pipe quiet output into subsequent class/shop flags. Project the actual shop/class identities in the owned command output adapter for quiet mode, or supply stable explicit identity fields. Preserve JSON context/assumptions and the existing select contract; do not repair this with a one-off change to reserved machine helpers. Add focused output tests for both paths.

### R4 — P2: The computed handoff rejects real alphabetic Toyota class codes

Location: `internal/toyota/domain.go:223–224`.

The validation regex requires every class to end in a number. Toyota's live [one-way calculator](https://rent.toyota.co.jp/eng/service/oneway/simulation.aspx) explicitly names **LXC** and **LXP** premium classes, and `oneway quote` itself explains which family covers them. Both real codes fail the computed handoff:

```bash
toyota-rentacar-pp-cli booking handoff --pickup-shop 63601:01V \
  --pickup 2026-10-20T09:00 --dropoff 2026-10-21T09:00 --class LXC --agent
# Same result with --class LXP.
```

Observed exit 2: `--class must be a Toyota class code such as C1, W2 or SUV1`.

The handoff is an unchecked checklist and does not select or promise inventory, so it should preserve these valid codes. Explicitly accept LXC/LXP in the bounded format validation, or use a source-grounded set of class formats. Keep invalid/injection-like values rejected. Add one test for each alphabetic class.

### R5 — P3, nonblocking: Required inputs are absent from custom MCP schemas

Owned locations: flag declarations in `internal/cli/cars_quote.go:47–50`, `internal/cli/booking_handoff.go:34–37`, `internal/cli/shops_search.go:32`, `internal/cli/toyota_commands.go:91`, `internal/cli/oneway_quote.go:31–32`. The machine walker only recognizes Cobra's required-flag annotation (`internal/mcp/cobratree/typemap.go:126–130`).

Actual stdio `tools/list` returns `required: []` for every custom planning tool. That misdescribes mandatory IDs/periods/keyword as optional. Runtime guards correctly reject missing values; this is an API discoverability issue rather than incorrect rental data.

Required inputs should be represented accurately in project-owned MCP schema/metadata where possible while preserving the intentional input-free CLI dry-run. If the framework cannot express both contracts, retain it as a concrete Printing Press retro candidate and document mandatory inputs in the tool descriptions. Do not modify reserved `internal/mcp/cobratree` just for this CLI.

## Verification and boundaries

Passed independently:

- `go test -count=1 ./internal/toyota ./internal/cli ./internal/mcp`.
- `go vet ./internal/toyota ./internal/cli ./cmd/...`.
- Build both real CLI and MCP executables from restored entrypoints.
- Actual stdio MCP initialize/discovery: all seven planning paths present, each with `readOnlyHint: true`; a computed handoff tool call succeeded.
- Native isolated Chrome inspection of current driving-document guidance, selected-shop controls, and one-way vehicle families. The visible foreign-license translation list contains the six countries emitted by this implementation; newer names in HTML comments are not rendered guidance.
- Live minivan quote (2026-11-10–11, 09:00 JST): W1/W2/W3, 6 upstream requests, 1,043,888 response bytes, 13,999 ms.
- Live SUV quote for the same period with 4WD plus infant and booster seats: SUV1–SUV4 and fully-booked SP1, echoed requested options, 6 requests, 1,033,332 bytes, 16,031 ms.
- Live distinct-shop compact quote with UTC input normalized to JST: `63601:01V` → `63601:095`, 2026-10-20 09:00 → 2026-10-21 09:30 JST, 24.5 hours; C1/C2 available and C0 fully booked, 7 requests, 1,315,241 bytes, 12,034 ms.
- Live policy fees remain thirteen source-derived entries; no final inclusive total is invented.

The initial review snapshot had an empty `cmd/` directory, so its README build commands initially failed. The builder restored both entrypoints during review, and the real binaries then built successfully. That copy defect is resolved in this snapshot; final promotion must preserve source directories when excluding binary files.

The authoritative latest `live-operating-windows.json` and `output-review-livecheck.json` support the existing happy paths. Earlier failures were treated as audit history. These passing gates did not cover the findings above.

No bookings, customer details, terms acceptance, payments, account writes, or publication were attempted. No anonymous cookies or ASP.NET session/form state were logged or saved. The documented final class-selection HTTP network-error boundary remains accepted: full confirmed totals and post-class equipment stock stay unknown, and the canonical handoff only prefills pickup.

Review regressions and overlay currently reside at `/private/tmp/toyota-fresh-review/{review-cli_test.go,review-toyota_test.go,overlay.json}`; they are not product source. Their expected failures substantiate R1 and R3. After fixes, rebuild the snapshot and re-run only the affected checks plus the required project gates.

Standards: 1 P2 finding, exact route context is the highest consequence. Spec/exposed behavior: 3 P2 findings and 1 nonblocking P3 observation; ignored rate ceilings, invalid quiet identities, and rejected valid classes each affect an advertised workflow.


## Round two — convergence

Reviewed the refreshed stable candidate, `evidence/review-round2-fixes.diff`, and the final adjustments in `evidence/review-round2-final-adjustments.diff`. The same dedicated reviewer performed this verification. No product source was edited.

**Accepted: all four P2 findings and the P3 MCP observation are resolved. No new product findings remain.**

| Finding | Resolution and independent evidence | Status |
|---|---|---|
| R1, route context | Final calculator labels require anchored `From `/`To ` prefixes and `Store` suffixes, followed by exact normalized full-name equality. Original overlapping-name review regression now passes; owned tests separately reject pickup and return Annex variants. A real Tokyo Nihonbashi → Hatchobori calculation still returned 0 JPY, `calculated`, and explicitly unknown dated inventory. | Resolved |
| R2, rate ceiling | All six live constructors pass the root rate mode to `NewClient`. A live `rental options --rate-limit 0.01 --timeout 2s` returned exit 5 at 2.014 seconds with `context deadline exceeded` on the second policy request, rather than fetching both pages immediately. The final transport observes valid source rate-limit headers; an independent controlled transport verified auto pacing follows the header budget, positive 0.1 rps remains capped under a high header budget, and zero remains disabled. | Resolved |
| R3, quiet identities | Every owned planning command uses the domain-aware adapter. Live shop detail prints exactly `63601:01V`; live compact quote prints exactly `C1`, `C2`, `C0`, one per line, with no caveat/status/price leakage. Owned tests cover all seven output types. Explicit field selection continues through the established selection pipeline. | Resolved |
| R4, premium codes | Both real CLI `LXC`/`LXP` handoffs now succeed and preserve the code, `inventory_checked: false`, and a null confirmed total. Injection-like `LXP&evil` remains exit 2. An actual MCP handoff with LXP also succeeds. | Resolved |
| R5, MCP required inputs | Actual stdio discovery now declares keyword for shop search, id for shop get, pickup-shop/pickup/dropoff for quote and handoff, and both shops for one-way. Option and eligibility tools correctly require none. Every planning tool retains `readOnlyHint: true`. All seven CLI paths still accept an input-free `--dry-run --agent`; missing real required inputs remain exit 2. | Resolved |

The calculator's final result exposes no known composite-ID inputs, as recorded in `evidence/native-oneway-identity-contract.json`. Exact anchored names are therefore the available final-page echo contract; composite IDs remain preserved in selection and output. The implementation does not claim to verify unavailable final hidden identity fields.

The source MCP descriptions now accurately promise **page metadata and links**, and direct users to the structured planning tools. Actual `tools/list` confirms the sanctioned sidecar descriptions were applied to all five source tools.

I noted that the initial 20 ms limiter assertion would also pass for the original 2 rps defect. The builder changed it to 750 ms, above that original limiter's 500 ms interval and below the requested 0.1 rps limiter's 10-second interval. The stronger test passes. This observation is closed.

The final additional quote assumption accurately states that the anonymous search does not select or verify a driver/license profile and that the applicable booking rate must be reconfirmed. It preserves the existing null full-total and human-booking boundaries.

Final verification:

- Package tests passed for `./internal/toyota ./internal/cli ./internal/mcp ./cmd/...` after the initial round-two fixes.
- Following the final client/header/test/assumption adjustments, the complete Toyota package passed with the scoped review overlay, including the original Annex regression, independent header-rate transport checks, and the strengthened owned limiter test.
- `go vet ./internal/toyota ./internal/cli ./cmd/...` passed on the final snapshot.
- Both final real binaries rebuilt successfully under `/private/tmp/toyota-fresh-review-round2/`.
- Actual stdio MCP discovery, required schemas, source descriptions, and premium handoff passed.
- Live fixed shop/class quiet output, explicit slow rate behavior, and strict-name one-way quote passed. The live one-way result used 9 requests, 827,745 response bytes, and 4,990 ms; this remained within the unchanged 14-request cap.

The round-two overlay is `/private/tmp/toyota-fresh-review-round2/overlay-round2.json` and contains **only Toyota transport and route-context review tests**. The old round-one CLI overlay, which calls the generic printer directly and bypasses the new owned adapter, was intentionally retired rather than treated as a current command regression.

Standards: R1 resolved, no open findings. Spec/exposed behavior: R2–R5 resolved, no open findings. Review convergence is complete; the builder can proceed with its required acceptance matrix and local promotion. No booking, customer data, terms, payment, account mutation, or publication occurred during re-review.


## Final metadata addendum

Accepted the amendment in `evidence/source-example-spec-fix.diff`. Its sole generated-command change adds one `Example` literal to `source_locations.go`; the matching endpoint example is preserved in `spec.yaml`. The keyword, flag names, and mode are valid: `--shop-mode 0` means **pickup search** (return search is mode 1). The synced MCP description and parameter metadata remain consistent with that meaning.

Independently ran `go run ./cmd/toyota-rentacar-pp-cli source locations --help`: exit 0, with the expected `Examples:` section and quoted Kyoto Station command. No business logic or custom source changed, and no new findings were raised. The review remains converged. The full live matrix was not rerun by this reviewer, as requested.
