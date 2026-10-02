# Airport Limousine independent review

Status: initial review complete; fixes and final verified-feature alignment pending. Dedicated fresh-context MAX reviewer; no additional agents, implementation edits, bookings, login, payment, account actions, or GitHub writes.

Reviewed source: `working/airport-limousine-pp-cli/internal/limousine/*.go`, typed domain commands under `internal/cli/`, provider HTTP adapter, relevant generated output and discovery helpers, README/SKILL/AGENTS, research brief/absorb manifest/spec/research.json, captured first-party Svelte data and browser/replay contracts.

## Actionable findings

### P2 — live-only typed commands ignore incompatible local data source

Locations: `internal/cli/fare.go:27–30`, `internal/cli/timetable.go:30–33`, `internal/cli/travel_times.go:29–32`, `internal/cli/transfers.go:42–50`, `internal/cli/conditions.go:27–37`, `internal/cli/limousine_discovery.go:35–38,89–92,129–132,165–168`.

Reproduction: `airport-limousine-pp-cli fare --data-source local --agent --no-learn` still attempted `GET https://www.limousinebus.co.jp/en/timetable/detail/Haneda-Narita/__data.json?d=2026-10-03&dir=1`. In the review sandbox, it exited 5 with DNS failure rather than a clear incompatible-source usage error. These commands instantiate `limousine.New` directly; `pp:data-source=live` annotations alone do not enforce the requested source.

Impact: an explicit local/offline request silently reaches the network, contradicting the local AGENTS Novel Command Data Sources contract. Add shared live-only source validation before fetching in all typed domain commands and verify agent/json invocations reject local without a provider request.

### P2 — compact agent conditions lose the numerical limits

Location: `internal/limousine/conditions.go:84–85`; emitted by `internal/cli/conditions.go:78`. The framework's compact list frequency rule at `internal/cli/helpers.go:2326–2348` explains the loss; a domain fix avoids a framework patch.

Reproduction: existing live `proofs/live/conditions.json`, produced by `conditions --topic all --agent`, contains rows for `checked_bag_count`, `checked_bag_dimensions`, `checked_bag_weight`, and `child_rounding` with neither `value` nor `unit`. Their summaries also omit the actual numbers. The source facts are 2 checked pieces/person, 50 × 60 × 120 cm each, 30 kg/piece, and 10 JPY rounding. Optional value/unit keys appear in fewer than 80% of the mixed rows, so generic compact output strips them.

Impact: the recommended agent command cannot answer the primary baggage-limit question despite the extractor finding the correct facts. Always serialize structured value/unit fields (null/empty for qualitative facts is sufficient), or otherwise preserve them in the domain output. Verify both `--topic all --agent` and `--topic baggage --agent`, including `--select` projections.

### P3 — domain discovery commands absent from curated capability lookup

Location: `internal/cli/which.go:28–34` and generated research feature index.

Reproduction: `which routes --agent` and `which handoff --agent` exit 2 with no matches; `which 'stop search' --agent` returns only `timetable`. `agent-context` confirms `routes`, `stops find`, `stops get`, and `handoff` are real runnable commands.

Impact: the documented AGENTS capability-discovery workflow misses useful first-class domain commands. Include these capabilities through the research metadata and normal generator/sync path if feasible; do not patch only generated index text.

### P3 — README emits an empty config path

Location: `README.md:347`.

Reproduction: “The platform-default config path is ``”. Reviewer doctor resolves a real config path under the isolated home and succeeds.

Impact: the displayed path is unusable. Remove the empty claim and point to doctor/runtime path resolution, or correctly populate the generator input. This is a template-shaped documentation defect, rather than a provider-contract issue.

## Builder-known issue independently observed

Initial `proofs/live/timetable.json`, `reverse-timetable.json`, and `conditions.json` have `{meta,results:{meta,results,...}}` because context fields outside the canonical envelope prevented the generated helper from recognizing it. The builder reported this before the reviewer found it and is moving contextual fields under meta. Final verification must assert the public envelope is exactly `{meta,results}` and `results` is the expected list.

## Verification performed

- Independently ran the relevant domain/CLI tests with `-count=1`: dated exact-terminal pair, known and unknown/mixed trip fares, invalid party, reversed stop roles, ambiguous cutoff, midnight rollover/extended hours, unknown times, returned date mismatch, current/standard arithmetic and unavailable states, route/stop IDs/Japanese names, streamed guide parsing, corrupted/error Svelte data, ephemeral public search state, HTTP 429, response body cap and cancellation. Passed. The first sandbox run could not bind the loopback httptest listener; the authorized normal-access retry passed.
- Inspected real first-party browser-sniff, replay and contract artifacts. The stop search contract is anonymous POST keyword plus ephemeral private cookies and a following GET; no credentials/session cookies imported.
- Ran independent live checks at 2026-10-02 15:32 UTC, saved under `proofs/reviewer/`: doctor; Haneda–Shinjuku from-airport and to-airport on 2026-10-03; Narita–T-CAT on the same date; nonexistent stop keyword; Haneda Airport terminal search. All exited 0. Individual observations used 1–2 requests and completed in 0.56–0.86 s; stdout was 505–18,680 bytes (doctor 1,615 bytes).
- Live Shinjuku data retained 14 forward station columns and 12 reverse station columns with correct boarding/alighting roles. Live airport search returned separate Terminal 1, Terminal 2 and Terminal 3 IDs. Negative stop query returned a genuine empty list.
- Existing builder live evidence has both airport-transfer directions, independent schedule/current-estimate fields, 3600/1800 JPY units and 9000 JPY for two adults plus one child. Source current travel clock has no verified date or exact terminal; outputs retain this distinction and unavailable states.
- Reviewed all five planned novel commands against help, implementation, absorb scope and README/SKILL claims. Triggers and anti-triggers are appropriate; no novel implementation stubs found. Authentication says none and agrees with doctor, adapter and source evidence. No resident browser process required.
- Domain source enforces 2 MiB response reads, Svelte expansion/depth limits, ID validation, HTTPS origin restrictions on redirects, typed 429 errors, limiter use, per-request timeout and CLI bound contexts. Domain read paths have no booking/payment/account mutation.
- Timetable JSON is a typed projection; captured inventory/reservation fields are excluded by the fixture test. Japanese station names, stop identity, source time values/types and selected JST date are preserved. Unknown -1 fare is null, and mixed trip fares require a selected trip.

## Final gate still pending

At initial review, `research.json` has the five planned `novel_features` but no `novel_features_built`. README Unique Features and SKILL Unique Capabilities match the planned five. The reviewer must compare their final command sets to the actual dogfood-produced verified set after synchronization and verify fixes above against refreshed source/live output. Required Press final gates and promotion were still underway; this review does not claim them complete.

