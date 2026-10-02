# Airport Limousine fix verification and final semantic review

Review status: previous functional findings fixed and independently verified. One newly reproduced P2 request-budget defect and one reintroduced P3 README defect remain open. Canonical shipcheck is not yet passing. No implementation source edits, additional agents, CDP sessions, account actions, bookings or GitHub writes were performed by the reviewer.

## Closed findings

- **Live-only mode enforcement:** current binary rejects `--data-source local` with usage exit 2 and the live-only explanation for all nine domain leaves: routes, stops find, stops get, timetable, travel-times, transfers, fare, conditions, handoff. In the network-restricted review sandbox every invocation returned that error before any attempted provider request. All guard call sites occur before provider construction/fetch. Evidence: `proofs/reviewer/fix-runtime-assertions.json`.
- **Condition numbers in agent mode:** Condition always serializes value/unit; numerical summaries now contain their facts. Both refreshed `all` and `baggage` samples preserve 2 pieces/person, [50, 60, 120] cm, 30 kg/piece, and the all-topic sample retains the 10 JPY child-rounding unit.
- **Capability discovery:** independent runtime checks resolve routes, handoff, stop search → stops find, boarding location → stops get. The isolated hand-authored capability extension avoids changing generator-owned index source.
- **Canonical output envelope:** all 16 final live samples have exactly `{meta, results}`, with list results. Timetable context and six exact terminal columns reside in meta.context/meta.stations. No double envelope remains.

## Remaining P2 — redirects bypass the eight-request ceiling

Locations: `internal/limousine/client.go:44–51,64–65,70–71,83`.

The Fetch counter and limiter cover the original request only. The HTTP client permits two same-origin redirects in each Fetch; those additional requests are neither counted nor paced. Three supported reads can therefore send nine requests while reporting three. This violates the explicit eight-request requirement and the newly added README/SKILL claim.

Independent deterministic reproduction uses the real Provider.New redirect policy and a fake RoundTripper, with no network and no implementation edits. The overlay test is saved under `proofs/reviewer/`:

```bash
python3 .manuscripts/20261002-221309-644a5be9/proofs/reviewer/replay_redirect_review.py
```

Current result: `wire_requests=9 reported_requests=3 last_error=<nil>`; test fails. The transport follows only redirects allowed by the existing HTTPS-origin policy. Count and pace redirects under the same budget before permitting them, or reject redirects if that matches the supported source contract. Reuse this overlay test to verify the fix.

## Remaining P3 — final doc sync restored the empty config-path claim

Location in the latest reviewed README: `README.md:352`.

The sentence again reads “The platform-default config path is ``”. The builder had removed it before shipcheck, but normal dogfood/doc synchronization appears to have restored it. Fix the final generated artifact or its source input so the empty claim does not survive the last sync.

## Independent checks after the fixes

- Relevant domain, output, capability and CLI regressions rerun with `-count=1`; both packages passed. This includes consequential parser/clock/fare/unknown-state logic and local HTTP search/body-limit/cancellation tests.
- Independently inspected all sixteen live-final result documents. A separate reviewer assertion pass checked canonical envelopes, live provenance, request metadata, JST clocks, forbidden inventory fields, exact terminal/date context, selected forward/reverse pairs, known fare arithmetic, unknown current-duration state/arithmetic, negative searches, selected field projection and both transfer directions. **Passed with zero issues; 143 emitted clock objects checked.** Evidence: `proofs/reviewer/final-sample-assertions.json`.
- Independently fetched three post-fix public workflows with the current binary: conditions all in agent mode; baggage agent output selecting key/value/unit and observed time; Haneda–Narita timetable selecting ID, exact departure/arrival timestamps, scheduled duration and date context. **All passed.** The selected timetable returned 2026-10-03 08:25 → 10:00 JST and 95 scheduled minutes. The baggage selection retained the structured numbers. Elapsed times were 1.209 s, 0.664 s and 0.488 s; outputs were 5,899, 668 and 481 bytes. Evidence: `proofs/reviewer/independent-fix-live-assertions.json` and the corresponding JSON/stdout/stderr artifacts.
- Broader existing live evidence covers Haneda–Shinjuku and Narita–Shinjuku, exact airport-stop searches and provider negatives. Independent earlier reviewer live checks also cover both Haneda–Shinjuku directions and Narita–T-CAT.
- Source/fixture evidence still supports the anonymous public search contract, devalue/streamed-data decoding, exact Japanese identities, explicit rollover and unknown times/fares, unit-based party totals, source-clock date uncertainty, independent schedule/live-estimate semantics and canonical handoff. Typed outputs omit source inventory/reservation fields.

## Printing Press 14–17 contract review

- **Verified set:** research.json now contains five novel_features_built commands: timetable, travel-times, transfers, fare, conditions. README Unique Features and SKILL Unique Capabilities match this set exactly; no planned-only command is claimed in those sections.
- **Semantic claims:** trigger phrases, anti-triggers, command behavior, auth type none and no-resident-browser explanation agree with help, source and live evidence. No implementation stub or undisclosed login/booking dependency found.
- **Recipes/output:** stop discovery and exact IDs, terminal detail, dated transfer comparison and baggage recipes produce their described domain data. Agent field projection is verified against fresh reads.
- **Limits/source caveats:** README/SKILL explain live-only mode, terminal identity, JST date rollover, contextual metadata, unknown live states and source-date uncertainty. Body, parser expansion, deadline and current request-counter code are bounded; the redirect gap above must be closed to meet the advertised eight-request ceiling.
- **Security/read-only:** HTTPS origin restriction and TLS verification remain enabled; public keyword cookies are ephemeral; no user auth cookies are imported or persisted. Domain commands perform public reads/search and user handoff only. No unsupported inventory, exact terminal identity for realtime labels, source date for the HH:MM clock, or arrival guarantee is invented.

## Canonical gate status

At the latest reviewed `proofs/shipcheck.json` (started 2026-10-02T15:39:24Z), verdict is FAIL / exit 3: validate-narrative exited 1 and dogfood exited 3. Verify, workflow-verify, apify-audit, verify-skill and scorecard passed. The five-command built-set alignment is independently checked, but this is **not** a successful canonical shipcheck or promotion claim. The builder is continuing those gates.

Final review clearance requires the redirect-budget fix, stable removal of the empty README config path, and verification of the required canonical gate result.


The public overlay contains portable `<cli-dir>` and `<proof-dir>` tokens. The replay command expands them from its own location, writes a private temporary overlay, runs both independent regressions, and deletes the expanded overlay afterward. Run the command from this CLI module directory.
