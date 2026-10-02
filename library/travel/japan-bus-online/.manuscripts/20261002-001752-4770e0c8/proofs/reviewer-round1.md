# Dedicated review — round 1

Reviewer: the single fresh-context gpt-6.1-sol MAX reviewer assigned to this build. No additional agents, `codex exec`, source edits, bookings, passenger submission, payment, account operations, or GitHub writes were used.

**Result: changes requested.** Five P2 contract/readiness findings and one P3 documentation warning were identified. The builder started applying reported corrections during this review; the entries below retain the behavior observed before those fixes. They are not considered closed until this same reviewer confirms the final source and evidence.

Reviewed the five approved workflows, provider implementation/tests, CLI output helpers and command wiring, runtime MCP catalog/context, README/SKILL/AGENTS, spec/provenance, approved research/absorb manifest, and preserved live/browser evidence. Combined Printing Press phases 14–17 and the output-review sub-skill in this one reviewer, as required by the user.

## Actionable source and contract findings

### F1 — P2 / error: validate returned service identities, not only the date input

- **Location:** `internal/jbo/client.go:566` in the reviewed `Services` implementation, and its date guard at original lines 582–589; `internal/jbo/quote.go:152` consumes these rows.
- **Trigger:** the dated response echoes `SelectDate=10/10/2026`, but a service row has `data-depdate=20261011`. The request is for `2026-10-10`, within the sale window.
- **Observed:** the independent overlay fixture returned requested/effective date `2026-10-10`, status `inventory_reported`, and a service departing `2026-10-11`. Quote can then follow the wrong service day. This is a deterministic contract reproduction, not a claim that the current live source returned this response.
- **Expected:** no wrong-day service may be represented as requested-day inventory. Validate row route, direction, and original service-day identity before returning inventory or following a quote.
- **Suggested fix:** reject inconsistent row identities explicitly, or report a source substitution without requested-day rows. Preserve the existing outside-window/no-services distinctions: suppress outside-window fallback rows before applying the in-window row identity guard.
- **Verification:** `go test -overlay=/private/tmp/jbo-review-probes/overlay.json ./internal/jbo -run '^TestReviewer' -count=1 -v`. Before the fix this failed with `requested_date=2026-10-10 effective_source_date=2026-10-10 status=inventory_reported returned departure_date=2026-10-11`. The overlay adds only a virtual test file; source files remain unchanged. Add permanent coverage for wrong route/direction/day, date-input substitution, and outside-window fallback rows.

### F2 — P2 / error: MCP context describes unsupported provider workflows

- **Location:** `internal/mcp/tools.go:830`, `internal/mcp/tools.go:836`, `internal/mcp/tools.go:844`.
- **Trigger:** call MCP `context`, which tells agents to call it first.
- **Observed:** it advertises typed endpoint tools, a syncable resource named `en`, cursor/`after` paging, default limit 100, and `sync`/`search`. The fresh runtime catalog has 23 tools, five provider command mirrors, no typed `en` endpoint tool, and no sync/search workflow. Lists actually use offset/limit with default 20 and live provider data.
- **Expected:** context must describe the shipped five workflows, actual paging/defaults, anonymous English source, and absence of a provider inventory cache.
- **Suggested fix:** adapt the context taxonomy and query tips to the provider implementation; retain accurate local learning-store explanations separately. Preserve the three verified novel capability descriptions from research.json.
- **Verification:** stdio JSON-RPC `initialize`, `tools/list`, then `tools/call` with `{"name":"context","arguments":{}}`; compare every advertised provider workflow and paging parameter with the catalog and CLI help.

### F3 — P2 / error: capability discovery cannot find conditions or common fare intent

- **Location:** `internal/cli/which.go:28`–33; README.md:60 and SKILL.md:71 recommend `which` for discovery.
- **Trigger:** `japan-bus-online-pp-cli which conditions --json` or `which baggage --json`; `which fares --json` is another natural request.
- **Observed:** all three exited 2 with `{"matches":[]}`. `bus conditions` exists and is approved, but is absent from the curated index; fare terminology also fails to resolve the implemented quote command.
- **Expected:** the five supported workflows should be discoverable using their command name and central domain terms.
- **Suggested fix:** add `bus conditions` with baggage/boarding/cancellation wording; make fare intent resolve `bus quote`. Keep generated description changes synchronized from research.json where applicable, and add a small discovery regression check.
- **Verification:** repeat those three `which` calls; assert conditions/baggage resolve `bus conditions` and fares resolves `bus quote`, then ensure `which --json` includes all five provider workflows.

### F4 — P2 / error: provider adapter ignored the advertised request-rate flag

- **Location:** original `internal/jbo/client.go:64`, `internal/cli/bus_commands.go:47`, `internal/cli/promoted_en.go:28`; advertised contract at `internal/cli/root.go:318`.
- **Trigger:** run any provider workflow with an explicit rate such as `--rate-limit 0.5`, or `--rate-limit 0`.
- **Observed:** all provider paths constructed `jbo.New(language)` with a hardcoded 2 requests/second limiter; `flags.rateLimit` never reached the adapter. The help promises an explicit maximum, disabled pacing at zero, and automatic header pacing by default.
- **Expected:** all five provider workflows honor the declared rate option while keeping timeout and 429 handling intact.
- **Suggested fix:** pass the root rate setting into provider construction; preserve auto pacing, explicit ceilings, zero semantics, and server-header observation. The builder has begun wiring this change; confirmation is pending.
- **Verification:** test explicit rate/zero/auto constructor behavior, a bounded fake-transport request sequence under a restrictive rate, header observation, and typed 429 propagation. Inspect both `busClient` and routes-list/legacy catalog constructors to ensure they share the setting.

### F5 — P2 / error: exported MCP manifest advertises a nonexistent tool

- **Location:** `tools-manifest.json:6`, `tools-manifest.json:13`; `.printing-press.json:21`–22.
- **Trigger:** an agent or installation consumer uses the shipped tools manifest to choose a tool.
- **Observed:** the manifest lists only `en_list-routes`, which is absent from runtime `tools/list`, and describes `browser-chrome-h2` although provider requests use the `jbo` Go HTTP adapter. Runtime provider tools are `routes_list`, `bus_route`, `bus_services`, `bus_quote`, and `bus_conditions`. The provenance counts remain 1 and need an explicit definition or reconciliation.
- **Expected:** a runtime tool manifest must match registered names and supported input/read contracts. Historical spec/provenance metadata must be labeled clearly if retained separately.
- **Suggested fix:** regenerate or reconcile the runtime manifest for the five provider mirrors, with correct transport and read hints; distinguish original endpoint-generation counts from current public runtime counts.
- **Verification:** compare manifest tool names against stdio `tools/list`; an advertised tool must exist. Confirm all five provider tools carry `readOnlyHint=true` and `destructiveHint=false`, and preserve the same schema flags/defaults as CLI help.

### W1 — P3 / warning: unsupported exclusivity claim in generated documentation

- **Location:** `SKILL.md:81`, `README.md:95`.
- **Trigger:** read the opening sentence of Unique Capabilities/Unique Features.
- **Observed:** both assert that these capabilities are unavailable in any other tool. Research has `novelty_score: 0` and `alternatives: null`; it provides no comparative evidence for that claim.
- **Expected:** concrete descriptions of verified capabilities.
- **Suggested fix:** remove the exclusivity sentence while preserving the supported command descriptions and the byte-exact installation block. This is generated boilerplate and is also an upstream template improvement candidate.
- **Verification:** compare documentation and research after sync; no unsupported exclusivity claim should remain.

## SKILL semantic review — all seven required checks

| Check | Result | Evidence or finding |
| --- | --- | --- |
| Trigger phrases match capabilities | PASS | The frontmatter covers route/stops, dated services/fares, conditions, and booking handoff; those workflows exist. |
| Verified-set alignment | PASS | Unique Capabilities exactly contains `bus services`, `bus quote`, and `bus route`, matching research.json `novel_features_built`. |
| Feature descriptions match command help | PASS | Each description matches its runnable help and the implemented intent. F1 concerns a guard defect, not an invented capability. |
| Stub/gated disclosure | PASS | No stub or auth-gated workflow is claimed. Local unpublished installation is disclosed before future distribution instructions. Current scheduled maintenance is external temporary state. |
| Auth narrative | PASS | Anonymous English GET workflows use memory-only cookies; no auth command is promised. |
| Recipe output claims | PASS | The Hamamatsu route shortlist and nested projection are supported by real preserved output and resolving flags. |
| Marketing-copy smell | WARN | W1: remove the unsupported exclusivity sentence at SKILL.md:81. |

README/SKILL/AGENTS otherwise match read-only scope, anti-triggers, verified English language, source Japanese-name nulls, one-way fare arithmetic, unknown discounts/infant fares/seat feasibility, selected-pair capacity and transaction limits, defaults, live-only provider data, and public booking handoff. No executable placeholder or nonexistent auth operation was found. The SKILL Prerequisites section is **byte-for-byte identical** to `evidence/canonical-install-section.txt` (1159 bytes); preserve it through fixes.

## Output plausibility and source evidence

The preserved pre-maintenance outputs are plausible and consistent with the native Chrome source check: three Hamamatsu catalog matches; both directions and mapped timetable stops; service `0001` on 2026-10-10 arriving 2026-10-11; default selected-pair Adult 6300 JPY/Child(6-12) 3150 JPY, party total 15750 JPY; stop 8 → 9 Adult 6100 JPY/Child 3050 JPY, total 15250 JPY; stop 8 at source 25:00 normalized to **2026-10-11T01:00:00+09:00**. Positive selected counts remain lower bounds, zero means no seats, and the 4-ticket transaction limit remains separate. Canonical route/detail/map URLs and Unicode names are intact.

Output-review checks: query relevance PASS for preserved catalog samples; formatting/canonical URLs PASS; multi-source aggregation not applicable; ranking not applicable because no ranked recommendation is claimed. Existing negative-query, nested select, CSV, agent source, date-window, party-cap, and timeout evidence is coherent. Numeric accuracy was additionally cross-checked against the supplied native browser evidence, beyond the output-review sub-skill's normal numeric blind spot.

### T1 — P3 / upstream tooling warning: outage errors are classified as passing samples

- **Location:** Press run `proofs/output-review-livecheck.json`, `live_check.features[2]` (bus route).
- **Trigger:** scorecard sampling encounters the provider outage before the new maintenance classifier was in its sampled binary.
- **Observed:** its sole `status: pass` entry is `graceful empty: Error: route ID not found or direction layout changed`; the output is an error, not a route or inventory result. Services and quote samples failed.
- **Expected:** nonzero-exit/parser/outage errors cannot establish successful provider output or inventory plausibility.
- **Suggested fix:** distinguish genuine successful empty results from failed commands in Printing Press live-check eligibility, then rerun sampling after real provider reopening. This is a Printing Press retro candidate, not a provider parser defect and not an instruction to edit reserved generated packages.
- **Verification:** inspect sample exit/status/output together; eligible success samples must contain a real successful result. Do not infer an inventory pass from this artifact.

```text
---OUTPUT-REVIEW-RESULT---
status: WARN
findings:
- check: eligible sampled output
  severity: warning
  description: The current sole passing sample contains an API/layout error during provider maintenance; it does not establish successful route output.
  suggestion: Exclude failed commands from passing samples and rerun against the reopened provider; retain the separate successful pre-window evidence.
---END-OUTPUT-REVIEW-RESULT---
```

## Verification performed and limits

- Fresh review binaries built successfully in `/private/tmp/jbo-review-bin/` using `go build` for the CLI and MCP. These builds did not change source or the staged release binaries.
- `go test ./internal/jbo ./internal/cli ./internal/mcp` passed after escalation for local loopback fixture binding. The initial sandbox bind refusal was environmental, not a source failure. The builder separately reports its full suite and vet passing; final required checks remain the builder's responsibility after fixes.
- All five commands resolved `--help` and successful `--dry-run --agent`; all five rejected `--data-source local` with exit 2. Flags/defaults were compared with source. The provider commands all use `boundCtx` before requests and shared output helpers; list limits are 1–100 with nonnegative offsets.
- A fresh stdio MCP process completed initialize/tools-list/context, quote dry run, and unknown-parameter rejection. All five provider tools had read-only/non-destructive hints. The probe process was terminated cleanly.
- Reviewed GET-only request construction, memory-only cookies, same-provider HTTPS redirects, response/redirect/time limits, typed 429 behavior, source ID handling, selected-pair capacity/arithmetic, optional cancellation failures, and exclusion of reservation/passenger/payment/account mutations. No additional booking mutation or secret persistence issue was found in the provider paths.
- Reserved `internal/cliutil/` and `internal/mcp/cobratree/` were inspected only to understand behavior; no source-edit recommendation targets those packages. Generator framework audit warnings are separate from these provider findings.

**Current external wait:** native browser evidence confirms scheduled maintenance served as HTTP 200 for 2026-10-02 02:01–04:59 JST. The typed maintenance classifier and its fixture correctly separate outage from inventory/layout change. This review does not invent a new current live pass. The full real live matrix, fresh sampled outputs, security/tool final audits, receipt closure, and local promotion must complete after actual reopening and fixes. No final acceptance or promotion is asserted here.

## Confirmation requested from the same reviewer

After the builder finishes fixes, send the touched files, exact test/evidence paths, and final source state back to this reviewer. Recheck every finding together, run the relevant deterministic checks and MCP introspection, preserve the installation block, and report whether findings are cleared. Final live inventory acceptance still requires real reopened-source evidence.
