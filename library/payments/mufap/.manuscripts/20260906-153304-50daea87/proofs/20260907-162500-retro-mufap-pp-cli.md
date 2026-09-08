# Printing Press Retro: mufap

## Session Stats
- API: mufap (Mutual Funds Association of Pakistan — HTML-table industry data, no JSON API)
- Spec source: internal YAML (`spec_format: internal`, `http_transport: standard`), auth: none
- Printing Press version: **4.31.7** (later locally patched with `press-fix-4539.diff` — see below)
- Scorecard: **85/100 (A)**; polish ran 85 → 85, `further_polish_recommended: no`
- shipcheck: 6/7 legs PASS; `verify` 97% (38/39), 0 critical
- Live dogfood: **117/117, verdict PASS**, level full — re-run five times today
- `go test ./...`: green; `go vet`: clean
- 10 hand-written novel commands: rates exposure backfill panel coverage verify universe
  dispersion freshness dump
- Outcome: **PROMOTED** to `library/mufap` — but only after patching the press. See F4.
- Retro method: 8 candidates, each classified then attacked by an adversarial "maintainer
  who wants to close it", then prioritized. **Half the slate died**, three at the premise.

## Findings

### F1. `doctor`'s reachability probe discards the HTML body, so a healthy 200 reads as "unreachable" (Bug)
Component: `generator`. Issue type: `bug`. Priority: **P2**.

A generated CLI's `doctor` reports `api: "unreachable: ... expected JSON, API returned HTML
instead of JSON"` for an upstream that answered **HTTP 200 with a valid HTML body**, and
false-fails `doctor --fail-on=error` (exit 1) on a healthy site. The same nil-body error path
starves the doctor's own 200-interstitial detector, so a Cloudflare challenge page is also
reported as "unreachable" rather than "blocked by Cloudflare interstitial" — the shipped
`looksLikeDoctorInterstitial` is dead code on its primary documented case. The credentials
step then reports `skipped (API unreachable)` off the same error.

Why it is a machine defect and not a per-CLI fix: `health_check_path` is a bare string in the
spec type with **no response-format field**, so this is the one HTML fetch in the machine that
a spec author cannot declare their way out of. The generator *does* correctly wire the HTML
opt-out for endpoints declaring `response_format: html` — just not for the health path.

Evidence: mufap `spec.yaml:7` `health_check_path "/"` on an HTML site, worked around here by a
hand-written `internal/cli/mufap_health.go`; zameen `spec.yaml:6` health path `/` on a
200-text/html site with the identical blind probe at `internal/cli/doctor.go:187` and zero
`HTMLResponseHeader` occurrences; google-maps `spec.yaml:8` health path `/maps`, same probe,
same absence. nccpl carries the client-side HTML guard but escapes only because its health
path happens to point at a JSON endpoint — an accident of one spec, not a template property.

Fix: in `doctor.go.tmpl`, switch the Step-1 probe to `GetWithHeaders(..., {HTMLResponseHeader:
"true"})` — the same generated function already does this for its Step-2 credential probe, so
no new plumbing. Do **not** weaken the client-side HTML guard; it is deliberate policy and
prevents HTML error pages being sanitized into `json.RawMessage`. Preserve the lost signal as a
non-error note (`reachable (HTML body at <path>)`) that does not match `doctorExitForFailOn`'s
"unreachable"/"error" substrings, and fix the same-error cascade on the Step-2 branch.
Separately: `internal/cliutil.ProbeReachable` exists explicitly to prevent doctor-vs-fetch
probe drift and ships into all 9 CLIs with **zero non-test callers** — wire it or stop emitting it.

Honest weakness: exactly one API trips this today and it is this run's own subject. zameen and
google-maps are latent-on-reprint (both `doctor.go` files carry the DO-NOT-EDIT header, so a
reprint regenerates the defect), and "fix it at reprint" is a defensible maintainer answer. What
carries it over the bar is that any *new* CLI with an HTML health path breaks on first print.

### F2. `resource-path:export` infers a command's data source from its FILENAME (Scorer bug)
Component: `scorer`. Issue type: `bug`. Priority: **P2**.

`runResourcePathContractChecks` treats the mere existence of
`internal/cli/{tail,export,import}.go` as proof the file is an API-reading command, and scores
it **0/3 CRITICAL** unless its text contains the emitted resource-path resolver. A hand-written
command that dumps the CLI's own local store cannot satisfy a contract about API paths.

Consequence on this run: the machine forced a **user-facing naming decision**. The command was
named `dump` and its file `dump.go` purely to dodge a filename-keyed check. That is the machine
dictating vocabulary, which is exactly backwards.

Correction to the original candidate: it is **not** a hard shipping gate. `verify` returns an
error only on verdict FAIL; one critical with a high pass rate yields WARN, which exits 0 and
passes shipcheck's verify leg. nccpl is the proof — published with this condition.

Fix: in `internal/pipeline/runtime_resource_paths.go`, classify the file by what it *does*
(does it call the API client?) rather than what it is named, reusing the sibling classifier
already in the same package. Measured guard on real files: 11-vs-5 split, zero overlap, so the
contrast set (amazon-jobs, foodpanda, hubspot, peekaboo, psx) is unaffected.

### F3. Collapsed single-endpoint commands ship a stale two-word Example (Bug)
Component: `generator`. Issue type: `bug`. Priority: **P2**.

Promoted (collapsed single-endpoint) commands ship a Cobra `Example` naming a
`<resource> <endpoint>` subcommand that was never registered. Because no promoted command
declares an `Args` constraint, the wrong example either exits 0 with correct output — Cobra
silently swallows the stray positional — or errors confusingly. All five promoted MUFAP
commands shipped a broken example.

Census across the library: **19 of 22 promoted commands affected**, 4 published CLIs verified at
file level, 2 confirmed at runtime on their shipped binaries.

Correction to the original candidate, which matters for whoever implements it: the stated root
cause ("the collapse is the machine's own transformation, so the machine knows both forms") is
**wrong**. `promotedExampleLine` echoes the spec author's hand-written `example:` verbatim. So
the durable fix is **detect and refuse, not repair**: at generate time, tokenize the example's
command path and check it resolves against the registered command tree; fail generation when it
does not. Detection-only has zero blast radius, and the three APIs a careless auto-repair would
damage (foodpanda menu/reviews, zameen listings) are safe under a token-equals-endpoint-name
condition.

### F4. dogfood cannot see the generator's own novel-command registration, and the coverage gate makes local-write features hollow by construction (Scorer bug)
Component: `scorer`. Issue type: `bug`. Priority: **P2**. **See the bucket disagreement below.**

Two halves, both reproduced on shipped CLIs on current main.

(a) The command walker cannot parse the generator's own novel-command registration. New fact
nobody had stated: an advertised novel feature **deleted outright** (registration removed AND
implementation file deleted) is still certified `8/8 survived` at exit 0, with the only signal a
WARN that misdescribes its own cause as a depth mismatch. The depth check compares an advertised
FULL PATH to Cobra's leaf `Name()`, so any nested novel command reports a false mismatch.

(b) The phase5 `coverage_hollow` gate marks a novel feature "never executed" when the live matrix
only ever ran it with `--dry-run` — and the matrix forces `--dry-run` on every mutating command.
**Measured consequence: a CLI whose novel feature is a locally-mutating command cannot be
promoted at all.** `lock promote` refuses outright:
`phase5 gate failed: phase5 acceptance has hollow coverage for: backfill`.
`backfill` was annotated honestly (`mcp:read-only=false`, `mcp:local-write=true`,
`pp:parent-group=true`) and still could not pass. The perverse incentive is explicit: the only
route past the gate on a stock binary is to **mislabel a writing command as read-only**.

Ruled out experimentally, so nobody repeats them: `--allow-destructive` (governs
destructive-at-*auth* endpoints, not local writes — re-ran the full matrix with it, backfill
still carried `--dry-run`); renaming the advertised command to a leaf that did run (still
hollow while dry-run was the binding constraint); any env override (none of the 9
`PRINTING_PRESS_*` vars bypasses it); `phase5-skip.json` (schema requires `status=="skip"` and
would downgrade a genuine 117/117 full pass).

Resolution used here: applied the existing `press-fix-4539.diff` to a clean v4.31.7 checkout and
rebuilt. Its own comment states the bug verbatim — *"Forcing it made every local-write novel
feature hollow by construction, so the only way to pass the phase5 gate was to mislabel the
command mcp:read-only."* That removed the forced `--dry-run` (6 real write invocations appeared)
but coverage **stayed hollow**, because the feature was declared as the bare parent group
`backfill`, which is never itself invoked — i.e. half (a) is load-bearing for half (b). Declaring
it at its executable leaf `backfill daily` (the feature's own documented `example`) cleared it:
`coverage_hollow` absent, pass/full, 117/117.

**Note for the maintainer: v4.32.0 does NOT contain this fix** (verified against the tag —
`localWrite` appears 0 times, the old `useDryRun` line is still at `live_dogfood.go:1640`).

**Bucket disagreement, recorded rather than resolved.** The adversarial reviewer placed this at
**Skip** on Step D as dispositive: half (a) is already psx WU-3 (filed 2026-08-20, P2,
unimplemented, whose acceptance criterion names this exact symptom) and was raised again as
nccpl WU-4; half (b) duplicates open upstream **#4539**, which already has a complete 54-line
patch on disk at `manuscripts/nccpl/.../proofs/press-fix-4539.diff`. The prioritizer listed it
Do-P2 while claiming zero overrules — an internal inconsistency. **Trust the Skip:** the correct
action is to raise WU-3's priority and attach the new false-negative evidence to it, and to land
`press-fix-4539.diff` — not to open a third ticket in the same walker family.

Two implementation warnings: completing the paths map activates `matchPath`'s collision
protection for the first time and could convert today's silent false-passes into a wave of
false-failures (`dogfood.go:599-603` documents the leaf fallback as deliberate false-negative
protection), so retain the fallback for genuinely unreconstructable trees. And nccpl's escape
from the hollow branch is explained: **the gate only computes when `--research-dir` is
supplied.** Verified directly — omitting it yields a marker with no `coverage_hollow` key at
all, which is the shape of every one of the 8 previously-promoted CLIs. That means the gate has
been silently inert for most of the library.

## Prioritized Improvements

### P1 — High priority
None. Zero P1 this run: no finding leaves a printed CLI broken, unsafe, or emitting silently
wrong data to a user.

### P2 — Medium priority
- F1 doctor HTML-200 misread (`comp:generator`, `bug`)
- F2 resource-path filename-keyed inference (`comp:scorer`, `bug`)
- F3 collapsed-path stale Example (`comp:generator`, `bug`)
- F4 novel-command walker + coverage_hollow gate (`comp:scorer`, `bug`) — **but see Skip**

### Skip
- **F4** per the adversarial reviewer: Step D. Already filed as psx WU-3 and upstream #4539.
  Attach evidence to those; do not open a third ticket.
- **`--quiet` collapses a summary object to one nondeterministic scalar.** Under `--quiet`, a
  generated command whose payload is a single summary object with no identity key has stdout
  collapsed to ONE arbitrarily-chosen scalar field (Go map iteration order), exit 0 — so a
  summary's `errors[]` / `failed` / `incomplete` fields vanish. Genuine, independently
  reproduced, Step D clean. Fails **Step B at two APIs, not three**: only mufap (this run's own
  subject, which the bar exists to discount) and nccpl define `printQuiet` at all; the other
  seven library CLIs `return nil` under `--quiet` and print nothing, which is honest. One part
  survives as a cheap safe change worth attaching as evidence: iterate `sort.Strings`-ordered
  keys in the generated `quietRowValue` so identical input yields identical stdout.

### Dropped at triage
- **`http_transport: browser-http` silently ignored** — **DISPROVEN, not merely unsupported.**
  `browser-http` is in the validated enum, is a live switch arm in `EffectiveHTTPTransport`, and
  drives distinct emitted output via `UsesHTTP2DisabledTransport` (its own `newHTTPClient` branch
  setting `NextProtos` to `http/1.1` and clearing `TLSNextProto`, plus its own README section).
  It means "stdlib with HTTP/2 disabled"; Surf is bound only by `browser-chrome*`. The one
  hypothesis that could have saved the finding — `generate` silently downgrading a declared
  value — was closed by disassembly: `applyHTTPTransportDefault` early-returns on any declared
  value and its only two writes are the 17-byte `browser-chrome-h2`/`-h3` constants, so it
  cannot write `standard`. And the premise has no artifact: **all three specs in the run declare
  `standard`**, and no spec anywhere in `PRESS_LIBRARY` declares `browser-http`. Root cause was
  human: `probe-reachability` classified MUFAP as reachability *mode* `browser_http`
  (underscore), and that mode name was carried into the README narrative as if it were a spec
  value. All four cited APIs turned out to be counter-examples.
- **`scorecard --live-check` SIGBUS** — falsified experimentally. `go build -o` rename-replaces
  the output (inode changes), so a running process keeps its mapped text pages; the child exited
  0 with clean stderr, and the only way to kill it was truncating the file by hand, which nothing
  in the pipeline does. The requested guard (`snapshotLiveCheckBinary` /
  `refreshLiveCheckStageBinary` / `rebuildLiveCheckBinary`) **already ships in v4.31.7**, the exact
  build that ran. No crash artifact exists against three live dogfoods at 117/117. The one proven
  SIGBUS in this codebase is the printed CLI's SQLite mmap, already filed as #3739 and already
  fixed. Filing this would reopen a closed bug under a wrong root cause.
- **Nothing warns when post-acceptance edits invalidate the marker** — **premise false.** This was
  raised *by me* during this session after observing a stale marker (4 of 175 file hashes drifted,
  3 changes behavioural). But `CaptureSourceFingerprint` + `validatePhase5SourceFingerprint`
  already exist, run unconditionally at both points of use, refuse with the drifted filenames, and
  are covered by six tests including edit/add/delete. The capability was specified and shipped in
  v4.31.7 after an earlier amazon-jobs retro. I inferred "nothing warns" from seeing the stale
  marker without ever testing whether promote would refuse — **it would have.** The safety claim
  ("a stale-but-non-hollow marker would have promoted silently") is provably inverted.

## Work Units

### WU-1: Make `doctor`'s reachability probe HTML-aware (from F1)
`comp:generator`, `bug`, P2. Switch the Step-1 probe to `GetWithHeaders` with
`HTMLResponseHeader`, add a non-error `reachable (HTML body)` note that does not trip
`--fail-on`, fix the Step-2 `credentials: skipped (API unreachable)` cascade, and resolve
`ProbeReachable` (wire it or delete it). Acceptance: a 200/text-html health path yields
`reachable` and exit 0; a Cloudflare interstitial yields `blocked by Cloudflare interstitial`.

### WU-2: Classify by behaviour, not filename, in the resource-path contract (from F2)
`comp:scorer`, `bug`, P2. Test whether the file calls the API client before applying the
resolver contract. Acceptance: a local-store dumper named `export.go` scores clean; an
API-reading `export.go` missing its resolver still fails.

### WU-3: Refuse a generated Example that does not resolve against the command tree (from F3)
`comp:generator`, `bug`, P2. Detect at generate time and fail; do not auto-repair. Acceptance:
the 19-of-22 affected promoted commands are caught at generation; foodpanda/zameen unaffected.

### WU-4: Attach evidence to psx WU-3 and land press-fix-4539.diff (from F4)
`comp:scorer`, `bug`, P2 — **not a new ticket.** Raise psx WU-3's priority, attach the
deleted-feature false-negative, and land #4539. Retain the leaf fallback for unreconstructable
trees. Also note the gate is inert without `--research-dir`, which is why 8 CLIs never saw it.

## Anti-patterns

- **Verifying an acceptance marker by mtime instead of content hash.** This run's own handoff
  asserted "marker is valid, 0 files newer" from an mtime comparison. Four of 175 hashes had
  drifted and three of the changes were behavioural, so the live matrix on record did not cover
  the shipped code. The marker stores every hash needed to check this properly.
- **Carrying a diagnostic MODE name into documentation as a configuration value.** `browser_http`
  (a `probe-reachability` verdict) became `browser-http` (a spec field value) in the README
  narrative, and generated a whole false finding. The shipped README still tells a blocked user
  to "confirm `http_transport` is `browser-http`" while the spec says `standard`.
- **Asserting a machine gap without testing it.** Two of this session's three dropped findings
  were mine, and both were speculation from an observation rather than a reproduction. Cost:
  real reviewer time. The bar "did you actually run the failing path?" would have killed both.
- **Passing verbose agent output through a fixed truncation.** The first prioritization pass
  silently received 4 of 8 verdicts because the input was sliced at a byte limit. It reported the
  gap honestly, which is the only reason it was caught. Project verdicts down to
  decision-relevant fields before passing them on, and log the count.
- **Letting a static check dictate user-facing vocabulary.** `dump` vs `export` was chosen to
  dodge a filename-keyed check, not for the user.

## What the Printing Press Got Right

- **The fingerprint machinery works.** It hashes the right set, persists per-file hashes, refuses
  at both points of use, and names the drifted files. It survived an adversarial attempt to prove
  otherwise — mine.
- **The transport enum is real and honest.** `browser-http` is validated, decoded, and drives
  genuinely distinct output. The generator honours a declared value over provenance-based
  defaulting, verified on a sniffed-provenance spec (google-maps).
- **The mmap SIGBUS class was already found and fixed** in the generator, with generator-emitted
  regression assertions in psx, nccpl and mufap.
- **`pp:happy-args` is more capable than assumed.** It accepts arbitrary `--flag=value` pairs
  (`parseHappyArgsAnnotation` + `overlayLiveDogfoodFlags`), so `backfill allocation` truncating at
  the 1m default is a one-line annotation fix in this CLI, not a machine defect. Recommended
  follow-up on the promoted CLI: add `--timeout=30m` to its `pp:happy-args`, so the harness stops
  scoring a truncated no-op (0 dates stored, 93 rows discarded, exit 0) as a pass.
- **The `coverage_hollow` gate, once patched, does exactly what it should.** It refused to certify
  a novel feature whose real path had never run, and it was right to. The defect was the forced
  dry-run, not the gate.
- **The live matrix is cheap.** A full 117-test live dogfood completes in ~50 seconds. The handoff
  claimed an hour and discouraged re-running it; it was re-run five times in one afternoon.

## Filed (7 Sep 2026, owner-approved)
- **WU-1** doctor HTML-200 misread → https://github.com/mvanhorn/cli-printing-press/issues/4612
- **WU-2** resource-path filename-keyed inference → https://github.com/mvanhorn/cli-printing-press/issues/4613
- **WU-3** collapsed-path stale Example → https://github.com/mvanhorn/cli-printing-press/issues/4614
- **WU-4** NOT filed as a new ticket, per the Step-D finding. Evidence added to the existing
  upstream issue instead:
  https://github.com/mvanhorn/cli-printing-press/issues/4539#issuecomment-5569930371
  — covering: it blocks `lock promote` not just `publish validate`; `--allow-destructive` does
  not help; **v4.32.0 does not contain the fix**; the patch is necessary but NOT sufficient
  (a bare parent group stays hollow even patched, entangling this with the command-walker
  defect); and the gate is **inert without `--research-dir`**, which is why it never fired for
  the 8 previously-promoted CLIs.
- Labels could not be applied (`viewerPermission: READ`); type/component/priority are stated in
  each issue body instead.
