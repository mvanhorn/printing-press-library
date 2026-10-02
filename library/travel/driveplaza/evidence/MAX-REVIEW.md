# Independent MAX review — Drive Plaza CLI

Reviewer: the single dedicated fresh-context reviewer requested for this build. No additional agents, implementation fixes, GitHub writes, publication, account changes, or purchases were performed. Round 3 is complete. All C1–C7 and D1–D3 findings, plus the limited round 3 wording clarification, are closed. The initial findings are retained below as audit history; their status is superseded by the final resolutions.

## Final verdicts — round 3

| Surface | Verdict | Basis |
| --- | --- | --- |
| Skill | PASS | All seven semantic checks pass. Verified feature set and recipes match; unsupported offline guidance/exclusivity claims are removed and the reference projection field is corrected. |
| Docs | PASS | Rechecked commands, flags, exits, examples, source assumptions, installation caveat and anti-triggers. Invalid input now exits 2, production HTTP paths are bounded, and MCP guidance describes actual workflows. |
| Output | PASS | Independently examined all five eligible actual passing `output_sample` values. No plausibility findings in those five samples. The separately reproduced MCP HTML failure is a code/interface finding, not an invented sixth passing sample. |
| Code | PASS | No remaining concrete findings. Source contract failures reject incompatible data, all shipping client paths have response limits, MCP returns parsed references and routes to the working SA/PA tool, and consequential regressions pass. |

## Round 3 limited polish recheck

**Skill PASS · Docs PASS · Output PASS · Code PASS.** No remaining concrete findings in the reviewed final source.

Reviewed the hand-authored Short changes in `driveplaza_commands.go`, `driveplaza_catalog.go` and `sapa.go` against the actual flags, request requirements, output fields and computed/live behavior. Route now names its required English IC/JST inputs and units; stop/list/detail/facility descriptions identify their respective scope; notices retain unknown current status; computed catalogs remain honest about source assumptions and handoff-only behavior; the SA/PA parent clearly advertises usage rather than returning stop data. One new interchanges wording clarification was requested and corrected within this round: Japanese-only code lookup no longer carries an unqualified promise of bilingual names. Its Long description retains the explicit limitation and partial-language warning behavior.

The additional SKILL format/envelope edits accurately describe complete compactly encoded domain results, field selection, positional local-learning queries, the actual live provenance fields, separate computed handoff/reference/local-learning shapes, and an explicitly illustrative rather than captured response. Prior seven skill checks and five eligible sample output verdicts remain PASS. No parser, transport or output behavior changed in this limited polish delta, so provider discovery/live requests were not repeated.

The existing 29-tool catalog was examined. Runtime agent-context confirmed the polished summaries; the root binary initially predated the very last interchanges wording adjustment, which was reported to the builder for the normal rebuild/catalog/package refresh before promotion. Review approval applies to the corrected final source; final build/Press/package verification remains the builder's next step. Recommendation remains to proceed with required polish acceptance and local promotion.

## Round 2 resolutions and recommendation

| Finding | Independent final recheck |
| --- | --- |
| C1 | Current real-source exclusion quote succeeds with `roadType1=on`/`roadType2=on`; source checkbox guard rejects ignored options. |
| C2 | Saved search form and malformed identities now fail explicitly; ordinary no-match filters, empty collection projection and tabular empty outputs remain correct. |
| C3 | Light/departure-kind mismatch and missing schedule markers are rejected in independent overlays; refreshed live light/arrival/all vehicle-class quotes remain accepted. |
| C4 | Current stdio `reference_rest_form`, `reference_route_form`, and `reference_schedule` return parsed canonical metadata/filtered official links, with no raw HTML preview. |
| C5 | Production root and MCP constructor paths both apply the preserved hook. Independent 2 MiB+1 replay now fails; encoded/decoded cap regressions pass and cache is disabled. |
| C6 | Current root and refreshed stage CLI return exit 2 for overlong names/six waypoints before I/O. |
| C7 | Context now reflects default10/max30/offset/source workflows. A remaining stale `mcp_tool` field was found and fixed within round 2; current live context and regression assert all three identifiers resolve to `sapa_list`/`sapa list`. |
| D1–D3 | Current docs remove unsupported sync/search and exclusivity guidance, and the executable reference example uses `canonical_url`. |

Recommendation: **accept this local build for the remaining required Press checks, polish and promotion. No remaining concrete review findings.** No publication has been performed or recommended as an authorized action.

Independent round 2 evidence: `MAX-round2-live.json` (current MCP context plus three parsed public reference responses and source exclusion quote), `MAX-round2-invalid-input.json`, and `MAX-round2-package.json`. Focused domain/CLI/client/MCP suites pass; the final changed context regression was run uncached. Temporary overlay checks additionally reject malformed source records, incompatible/missing quote conditions and oversized hooked responses while preserving typed 429/deadline propagation and empty JSON projection.

Reviewed the refreshed 17-check/20-request source acceptance, eight supplemental variants and four-call MCP acceptance. The five eligible Press output samples remain semantically correct, with unchanged source summary/unknown/provenance contracts. The refreshed package contains both companion binaries; independent SHA-256 comparison matches each ZIP entry to its current stage binary. Stage's formerly stale invalid-input behavior is corrected.

## Initial findings retained as history

### C1 — P1: road exclusions were ignored by the source

Location: `internal/driveplaza/source.go:193` and `:196` in the initially reviewed source.

`ValidateRoute` sent `roadType1=1` and `roadType2=1`. The captured native form defines both checkbox values as `on`. Two independent live public requests for NERIMA → SENDAI-MINAMI on 2026-10-10 08:00 confirmed that `1` leaves the returned checkboxes unchecked, while `on` checks both. The CLI consequently attached exclusions=true to a quote produced without those exclusions.

Evidence: `evidence/MAX-wire-check.json`; fixture `route.html` checkbox fields; the two public requests returned HTTP 200 with distinct response sizes (186383 vs 200036 decoded characters).

Smallest fix: serialize `on`, verify the returned checkbox state, and test that ignored exclusions are rejected.

Status: CLOSED. Independent live CLI exclusion quote succeeds with both `on` values; returned-state validation is present. See `MAX-round2-live.json`.

### C2 — P2: malformed SA/PA source results can become a genuine empty set

Location: `internal/driveplaza/parse.go:236` and `:244` in the currently inspected source (`:223` and `:231` initially).

The initial parser accepted the saved **search form** as a successful empty result because it contained h1 and HIGHWAY. The builder now checks the actual result heading, which independently rejects that saved form. However, an actual 82-box result response with changed/missing stop hrefs is still silently reduced to zero records: the invalid-identity branch simply continues. This makes parser drift look like a real empty source result, contrary to the domain invariants.

Reproduction: replace `/sapa/1040/` with `/changed-sapa/1040/` in saved `sapa-list.html`, then call `parseStops`. `TestMAXReviewMalformedStopIdentities` logs `parsed=0 error=<nil>` despite 82 result boxes. This uses a real captured result structure, with only the identity href contract changed.

Smallest fix: reject a result box with a missing/invalid identity or name, or explicitly propagate a partial result and dropped-record warning. Preserve legitimate zero-box result pages and local no-match filters as empty arrays.

Status: CLOSED. Independent saved-form and altered-82-box tests now return explicit source-contract errors; legitimate local no-match sets and empty projections remain empty.

### C3 — P2: quote assumptions can conflict with the returned source conditions

Location: `internal/driveplaza/source.go:594` through `:615` in the currently inspected source (`:580` through `:601` initially).

Date/time comparison only runs when the source selectors are nonempty. Vehicle and departure/arrival mode are not checked. Replaying captured `route.html`, whose displayed source conditions are standard vehicle/departure 08:00, accepts the same prices while attaching requested `vehicle=light` or `time_kind=arrival`. Removing the txt-date and txt-time class markers also leaves the quote accepted. A fallback response or layout change can therefore be relabeled as the requested quote.

Evidence: temporary overlay `TestMAXReviewQuoteEcho`, run against the saved first-party route capture, logs `accepted=true` for all three cases. Native captured fields include txt-type=`standard size vehicle`, txt-departure=`Departure`, txt-date=`2026/10/10`, and txt-time=`08:00`.

Smallest fix: require the source schedule identity markers, compare vehicle and time-kind echoes, and reject incompatible or unidentifiable quotes before emitting attached assumptions. Normal live light/arrival responses must continue to pass.

Status: CLOSED in review round 2; see the resolution matrix below.

### C4 — P2: typed MCP reference tools bypass HTML extraction

Location: `internal/mcp/tools.go:45` through `:75`, and `:370`.

All three typed reference tools request raw HTML and send it to the generic endpoint result renderer. They never run the CLI's HTML extractor. A real stdio call to `reference_rest_form` returned a 5086-byte JSON preview of the HTML head with `_pp_truncated:true` and `original_bytes:83954`, rather than the advertised ReferencePage fields. Form controls and useful links fall outside that preview. Small HTML pages similarly return raw HTML instead of structured results.

Evidence: `evidence/MAX-mcp-reference.json`, from the actual built MCP server and one public read-only form fetch. The companion CLI's reference command demonstrably calls `extractHTMLResponse` with page/links mode.

Smallest fix: expose these references through the parsed companion CLI mirror, or apply shared page/links extraction before MCP bounding. Verify the stdio results contain parsed titles/canonical URLs/links, with the schedule returning its official filtered handoffs.

Status: CLOSED. Preserved custom reference handlers override the generated registrations after startup and invoke parsed CLI extraction through the public command runner; all three current stdio tools were independently verified.

### C5 — P2: generated reference HTTP responses are not size-bounded

Location: `internal/client/client.go:1338`; documented universal bound at `README.md:46`.

The domain client limits bodies to 2 MiB, but generated reference commands and typed MCP tools use unbounded `io.ReadAll(resp.Body)`. Their later output limit does not bound allocation during retrieval. An independent transport replay accepted a 2097153-byte HTML response with no error. The decoded-body limit is also 32 MiB rather than the documented 2 MiB.

Reproduction: overlay `TestMAXReviewHTMLBodyCap` invokes `GetWithHeadersNoCache` with the public HTML response marker and a body of `(2<<20)+1` bytes; observed `bytes=2097153 error=<nil>`.

Smallest fix: cap the non-streaming HTTP read before allocation and align the decoded cap with this CLI's intended 2 MiB contract. Retain separate streaming semantics if needed; these reference endpoints are HTML, not binary streams.

Status: CLOSED. The preserved `ApplyDrivePlazaLimits` hook decorates all shipping CLI/MCP clients, bounds encoded and decoded reads to 2 MiB and disables caching. Direct framework SDK construction is not a shipping command path; independent hooked overflow replay now rejects the body.

### C6 — P2: invalid route names/waypoint count return transport exit 5

Location: `internal/driveplaza/source.go:157`; `internal/cli/driveplaza_commands.go:112` through `:116` (initially `:97` through `:101`).

The overlong-name/six-waypoint validation error starts with `IC names`, while dpRun recognizes usage errors by a leading `--` or two specific other prefixes. Consequently six `--via` values and a 101-byte `--from` return exit 5, contradicting the documented invalid-input exit 2 and suggesting a provider/parser failure.

Evidence: `evidence/MAX-invalid-input.json`. Both native invocations fail before any provider request, with `Error: IC names must be at most100 bytes; --via accepts at most5 waypoints` and exit 5.

Smallest fix: return a typed validation error or make this error explicitly flag-prefixed; add an exit-2/no-I/O regression for both cases.

Status: CLOSED in review round 2; see the resolution matrix below.

### C7 — P2: MCP context directs agents to unsupported workflows and a help-only SA/PA tool

Location: `internal/mcp/tools.go:867` through `:871`, and `:878`.

The actual `context` response advertises cursor/after paging, default limit 100, sync, and search. The runtime has no sync or search command/tool, and domain pages use offset with default 10/max 30. Its directional facility capability points at `sapa`, which emits subcommand help, instead of the verified `sapa list` command and corresponding MCP tool. An agent following context is therefore routed away from the working data workflow.

Evidence: `evidence/MAX-mcp-reference.json` contains the real context response; `evidence/MAX-runtime-help.json` and `agent-context` confirm the runtime tree; `research.json` lists verified command `sapa list`.

Smallest fix: replace generic query tips with the actual source/offset/summary/detail guidance and map this capability to `sapa list`. This is a command/routing correction; do not alter the truthful feature description.

Status: CLOSED in review round 2; see the resolution matrix below.

### D1 — warning: SKILL advertises nonexistent offline sync/search

Location: `SKILL.md:167`, `:169`, and `:181`.

The skill's agent guidance discusses sync progress and claims offline sync/search support. Neither command exists. Live domain commands reject `--data-source local`; optional local learning is already documented separately. Remove these generic claims and describe the actual live domain and computed catalog behavior.

Status: CLOSED. Generic offline sync/search claims are removed; live source work, embedded catalogs and optional local learning are described accurately.

### D2 — warning: unsupported exclusivity claim

Location: `README.md:108`; `SKILL.md:53`.

“These capabilities aren't available in any other tool for this API” is an unverified marketing claim. The focused absorb manifest explicitly calls the work improvements to the provider workflow rather than unique technology. Remove this line or describe the five concrete capabilities without exclusivity.

Status: CLOSED. The exclusivity sentence is absent from both current README and SKILL.

### D3 — warning: reference projection example asks for a nonexistent field

Location: `SKILL.md:178`.

The worked example uses `--select url,title,links`. Reference page extraction exposes `canonical_url`, not `url`. Running extraction against saved SAPAServiceEN HTML prints `warning: --select "url" matched no fields; valid fields: canonical_url, description, image_url, links, title`.

Smallest fix: use `canonical_url,title,links`, matching the actual extractor.

Status: CLOSED. The example now selects `canonical_url,title,links`, matching actual parsed reference results.

## Checks and examined invariants

- Read the applicable workspace AGENTS.md, batch2 brief, Printing Press review phases/output-review skill, writing-for-agents guidance, README, SKILL, absorb manifest, planned/built research feature sets, browser discovery provenance, saved endpoint contract, initial implementation diff, and shipcheck/live acceptance evidence.
- Examined the complete newly authored source/parser and domain command wiring, all five constructors and consequential behavior tests. Reviewed generated root invocation/config/timeout/projection/error paths, client transport/retry/cache/redirect paths, MCP registration/result/context/SQL paths, and store read-only opening, SQL parameter binding, migration/transaction and local learning persistence paths. No concrete injection, credential leakage, account mutation, or writable SQL finding identified in those examined paths.
- Existing `go test ./internal/driveplaza ./internal/cli ./internal/client ./internal/mcp ./internal/store` passed independently. Targeted temporary overlays reproduced the additional failures without changing production source. Temporary overlay path is recorded in `evidence/MAX-overlay-path.txt`.
- Domain/source checks covered actual summary toll columns versus section/detail prices; distinct standard/ETC/ETC2.0; nullable prices versus zero; km/minutes; requested versus predicted timing; date and ten-minute validation; vehicle/time-kind/waypoint/exclusion wire values; directional stop IDs; green/gray/missing availability; absolute Japanese hrefs; branded Pasar name extraction; detail header versus erroneous breadcrumb; source weekday hours and nearby directional links; source non-East 2006-03-31 warning; RSS dates/release/postponement titles with active restriction null; canonical official handoffs; output paging/scan limits; provenance-preserving nested projection; typed 429 and deadline propagation through optional Japanese enrichment; oversized/domain response failure; dry-run before I/O.
- Inspected runtime help for every domain leaf, reference leaves, and framework branches; saved the bounded local help inventory. Agent-context auth is `none` with no required credential environment variables. Local unpublished installation caveat and anti-triggers are present. No hidden domain stub or required browser/auth setup found.
- Existing live acceptance covers 17 behavior checks and 20 provider requests separately from fixtures, with roughly 19–1611 ms latency and 19–31 MiB peak RSS across recorded commands. Additional independent real-source checks consisted of two exclusion-contract HTTP requests and a stdio MCP form call. Sandbox DNS failure was an execution-environment issue; authorized escalated public HTTP succeeded without cookies or login.
- Provider discovery used native Chrome after IAB was unavailable. No Playwright runtime/fallback was used by this reviewer. The limited schedule CDP capture and saved HTTP replay evidence are documented separately; no HAR or live-active restriction claims were inferred.

## Seven skill checks

1. Triggers: PASS — toll comparison, Tohoku rest stops, IC resolution, and explicit CLI use map to supported commands.
2. Verified set: PASS — exactly route, interchanges, sapa list, notices, handoff match the five built features.
3. Feature descriptions: PASS for the shipped CLI semantics; the exclusion/echo correctness defects are separately recorded as code findings.
4. Stubs/gates: PASS — no concealed stub, authentication gate, or browser dependency; unsupported English code-entry lookup is disclosed.
5. Authentication: PASS — public HTTP and no auth commands are claimed.
6. Recipes: PASS — the four primary recipes and corrected reference projection example match actual output intent and fields.
7. Marketing: PASS — the unsupported exclusivity claim is removed.

## Five eligible passing sample output review

Reviewed every `live_check.features[].output_sample` with `status:pass` in the run's `proofs/output-review-livecheck.json`:

| Sample | Intent/format/source ordering result |
| --- | --- |
| route | Standard NERIMA → SENDAI-MINAMI assumptions remain attached. Three source alternatives preserve summary fees, distinct ETC2.0, time-priority order, km/minutes, movement warning and conditional/predicted timing labels. |
| interchanges | Correct stable NERIMA ID and Japanese 練馬/ねりま with its Kan-Etsu road. `<redacted>` is a privacy scrub, not a format defect. Two-source provenance is present. |
| sapa list | Five Tohoku up records match the request and source order, retain Japanese names/directional IDs/URLs, distinguish false facility icons, preserve dated/operator/hour caveat and next offset. |
| notices | Five source advisory records retain canonical PDF/content URLs and dated release/postponement/plan titles with null active status. Source feed order is plausible; no active-closure inventory inference. |
| handoff | Eight official canonical destinations have precise purpose labels; zero HTTP requests, observed date, null retrieval time and handoff-only state accurately describe embedded output. |

No raw entity, mojibake, malformed canonical URL, silent requested-source drop, misleading query relevance, or implausible source ordering found in these five eligible samples. The skill's output-review calibration treats source numeric/freshness validation separately; those were also checked against the saved captures and independent live evidence above.

---OUTPUT-REVIEW-RESULT---
status: PASS
findings: []
---END-OUTPUT-REVIEW-RESULT---
