# Independent Sunflower Ferry review — round 1

Reviewed 2026-10-02 by the one authorized fresh-context MAX reviewer. No extra agents, browser/CDP sessions, implementation edits, cabin selection, accounts, holds, standby registration, payments, or GitHub writes were performed. This combines Printing Press phases 14–17 under the user's one-reviewer limit.

## Scope and evidence

Reviewed the batch build brief/preflight, research brief, absorb manifest, research.json, native browser evidence, browser-sniff report, source index, sanitized portal forms, live pilot and all five pilot JSON outputs. Inspected the hand-written ferry client/parsers/commands/constructor registration, generated CLI/client/MCP contract paths, README, SKILL, and AGENTS.

Independent deterministic validation passed: `go test ./internal/ferry ./internal/cli -run 'TestTimetables|TestSourceQuote|TestDaytimeArrival|TestCalendarDirection|TestCabinAndPort|TestConditionsDerive|TestValidationAndReservation|TestRateLimitAndBody|TestNovel' -count=1`.

Independent real provider checks used normal approved network access after the sandbox DNS failure was identified as a local tool restriction:

- CLI car/child recipe failed after 4.98 seconds, exit 5: `proofs/reviewer-car-child.json` and `.stderr`.
- Exact known anonymous three-stage HTTP replay returned HTTP 200 at Reserve1020 and nine real cabin fares plus a vehicle indicator: `proofs/reviewer-car-child-source.json`. Only displayed headings/table text were saved; no hidden values or cookies were retained.
- Fresh isolated reviewer CLI/MCP builds succeeded. Current-source MCP tool schemas expose every domain flag and proper read-only hints: `proofs/reviewer-mcp-current-tools.json`; route result: `proofs/reviewer-mcp-current-routes.json`.
- Real public timetable reads expose the generated CLI/MCP HTML mismatch: `proofs/reviewer-mcp-source-timetable.json`, `proofs/reviewer-cli-source-timetable.json`.

The initial pilot contains five real passing commands: quote-foot, calendar, cabins, ports, conditions. Those are eligible output samples; the independent failed car/child invocation is not a passing output sample. Fixtures were reviewed only as deterministic parser evidence and never accepted as live proof. The eventual scorecard JSON and expanded six-direction/vehicle/daytime live matrix remain builder gate work.

## Actionable findings

Line references below identify the initially reviewed code; concurrent builder fixes may shift them. Each finding was sent to the builder as soon as confirmed. Reverification is required after fixes.

### F1 — P1 / error: informational links prevent valid sailing parsing

`internal/ferry/operations.go:193` matches the entire h2 against an end-anchored regex. The real Kobe→Oita car/child response appends `About Classes About Ferries`, so the approved README recipe fails with no parseable sailing. Extract sailing heading text without informational link captions and keep strict date/route/time checks.

Evidence: h2 `10/15(Thu) Kobe 19:00 Departure >> Oita 06:20 Arrival （SUNFLOWER PEARL） About Classes About Ferries`; Reserve1020 contained Deluxe 85440 JPY and Tourist 45590 JPY for two adults, one child, one <5 m car. See `reviewer-car-child-source.json`.

### F2 — P2 / error: declared data-source strategies are not enforced

`internal/cli/ferry_commands.go:204` and sibling RunE handlers ignore `--data-source`. `quote --data-source local` attempts the booking GET; `routes list --data-source live` returns static data. Call `validateDataSourceStrategy` before transport/local reads for every hand-written command, and return a clear incompatible-source error.

### F3 — P2 / error: car/bike availability is discarded

`internal/ferry/operations.go:234` only retains six-cell cabin rows. The actual car result also has `Vehicle ○`, which is omitted despite quote text referring to vehicle availability. Preserve the source vehicle symbol/status on the sailing, with snapshot caveats and an explicit unknown when absent.

### F4 — P2 / error: route search omits advertised terminal identities

`internal/ferry/model.go:72` searches route text and English port names only. `routes search osaka-terminal2` returns an empty result although that stable terminal ID is emitted by routes list; Japanese port names are omitted too. Search both port IDs, Japanese names, English names, and line IDs.

### F5 — P2 / error: agent provenance disagrees with the documented contract

`internal/cli/ferry_commands.go:38` overwrites agent `meta.source` with the same provider phrase for static and dynamic commands. `SKILL.md:209` instructs agents to distinguish `live|local` there. Preserve the declared source strategy as `meta.source`; put provider wording in a separate field.

### F6 — P1 / artifact error: staged MCP contains obsolete scaffolds

The initial `build/stage/bin/sunflower-ferry-pp-mcp` exposes quote with only date/route, omits routes/handoff, and marks domain commands destructive. A fresh isolated build exposes correct current-source schemas. Rebuild both staged binaries and refresh the MCPB before promotion; verify the final bundle's tools list.

Evidence: `reviewer-mcp-tools.json` (stale) versus `reviewer-mcp-current-tools.json` (current source).

### F7 — P2 / documentation error: irrelevant generated operating claims remain

`SKILL.md:194,218–219` and `README.md:352` promise sync/search store workflows, credentials, cookies, and jobs absent from this no-auth CLI. `README.md:367` has an empty config path. Narrow these sections to the actual public planning and local learning/profile state, and let doctor supply platform paths.

### F8 — P2 / documentation error: executable examples still contain placeholders

Printing Press phase 15 explicitly prohibits placeholder literals in executable examples. Examples at `README.md:334`, `SKILL.md:139,363,380–382,520`, and `AGENTS.md:17,18,24,30,31,52` use angle brackets, including shell redirection syntax. Replace executable examples with concrete ferry queries/IDs/paths; keep schematic syntax outside executable blocks.

### F9 — P3 / documentation warning: doctor dry run does not inspect configuration

`research.json:narrative.quickstart[0]` and `README.md:154–155` describe doctor dry run as configuration inspection. The actual output is only `{"dry_run":true,"action":"doctor","would":"run doctor; no changes made"}`. Change the source narrative to describe a preview, or use the real doctor command for inspection.

## Printing Press template retro candidates

These are generated/template issues, separated from hand-written domain fixes under phase 17's template escape hatch. They must be recorded/disclosed rather than hidden by broad patches to generated packages.

### T1 — P2 / warning: typed MCP HTML tools skip the CLI extraction contract

`internal/mcp/tools.go:84–91,391` sends raw HTML into a generic result handler; the matching CLI at `internal/cli/source_timetable.go:53` applies `html_extract:page`. MCP returns HTML while CLI returns structured page metadata. Feed the same extraction configuration into typed MCP generation, or mirror the CLI HTML endpoint commands upstream.

Source/template family: generated MCP endpoint registration and generic handler. Runtime evidence is in the paired timetable proofs. Domain MCP command mirrors are correct and remain the recommended interface.

### T2 — P2 / warning: generated source-client body reads are unbounded

`internal/client/client.go:1330` performs `io.ReadAll(resp.Body)` before any bound on ordinary HTML. Only compressed decoded data has the separate 32 MiB limit. Generated low-level `source` commands therefore do not inherit the native ferry client's 2 MiB cap. Add a bounded ordinary-body read in the Printing Press client template.

Source/template family: shared generated client HTTP response reader. The hand-written ferry client separately uses a 2 MiB LimitReader and a request budget; those protections passed review.

## Reviewed safeguards and domain accuracy

- Only the three Kansai–Kyushu routes and six source direction codes are in scope; distinct Osaka terminal identities are retained.
- The HTTP allowlist restricts hosts, HTTPS, allowed GET resources, the two observed read-only POST steps, redirects, request counts, and body bytes. Reserve1020/MoveNext, account, reservation, and payment paths are unreachable through the ferry client. Anonymous cookies/tokens stay in memory.
- Portal hidden values and sensitive request-header/body names are scrubbed. The only remaining non-redacted hidden value in the sampled fare response is the non-secret Language field.
- School-stage categories, car length bounds, bike counts, medium pet-cage count limits, and phone-only qualifications are stated. Source fares are preserved without arithmetic, with explicit unknown fuel/tax/fee breakdown and inventory caveats.
- Normal timetable rowspan and six directions, next-morning and year rollover, explicit calendar direction/coverage/missing dates, E daytime warning, and shared-section occupancy are covered by consequential parser tests.
- Baggage distinguishes 30 kg carried-item definition from 20 kg aggregate free allowance. Normal cancellation derives the common Japanese day-before band and 200 JPY minimums, while warning about ticket-specific and vehicle conditions.
- Current-source CLI/MCP domain schemas and read-only hints agree. Final staged executable/bundle verification is still required.

## Phase outcomes

Phase 14: six unique capability commands and trigger/anti-trigger scope align with research; F1 and F9 affect executable narrative claims.

Phase 15: F5, F7, F8, and F9 require correction or explicit template acceptance. No unsupported booking/account/payment/auth commands are promised in the new planning section.

Phase 16: assessed the five real passing pilot JSON outputs. No semantic-query, format, source aggregation, or ordering/ranking finding in those samples. The failed car/child command is a correctness finding under phase 17. Reassess the final scorecard/expanded live matrix after fixes.

---OUTPUT-REVIEW-RESULT---
status: PASS
findings: []
---END-OUTPUT-REVIEW-RESULT---

Phase 17: initial actionable domain/artifact findings F1–F6 were reported for small fixes. T1/T2 are template retro candidates. No review convergence or overall ship approval is claimed before fix and final live/bundle verification.
