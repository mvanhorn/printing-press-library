# MUFAP + SBP — MASTER HANDOFF (7 Sep 2026, 11:4x PKT)
# Written because the session ran out of context. A new session can start from this file alone.

RUN_ID:  20260906-153304-50daea87
Run dir: ~/printing-press/.runstate/personal-74f0f171/runs/20260906-153304-50daea87
CLI:     $RUNDIR/working/mufap-pp-cli   (binary ./mufap-pp-cli, builds clean, go test green)
Press:   $HOME/go/bin/cli-printing-press  v4.31.7
LOCK:    still HELD (phase shipcheck, stale >12h). Release or reclaim:
           cli-printing-press lock release --cli mufap-pp-cli

## ============ WHAT IS DONE ============

### 1. MUFAP CLI — Phase 5 GREEN, polished, NOT YET PROMOTED
  shipcheck 6/7 legs PASS; scorecard 85/100 Grade A; live sample probe 10/10
  verify 97% (38/39) 0 critical; live dogfood FULL 117/117 100% verdict PASS
  acceptance marker: proofs/phase5-acceptance.json  status "pass", level "full"
  MARKER IS VALID: newest .go = 22:48:07, marker = 22:50:39, 0 files newer. Promote will pass.
  Polish ran: 85 -> 85, tools-audit 2 -> 0 pending, ship_recommendation ship-with-gaps,
  further_polish_recommended: no.
  10 novel commands, all hand-written, all shipped:
    rates exposure backfill panel coverage verify universe dispersion freshness dump

### 2. research.db — THREE MUFAP TABLES + SBP, all committed (owner-approved)
  policy_rates      35 rows   2016-05-21 .. 2026-04-27   21 hikes 13 cuts
  mufap_rates     3,669 rows  2016-08-22 .. 2026-09-07   2,483/2,483 trading days joined
  mufap_panel   981,688 rows  2016-08-22 .. 2026-09-07
  mufap_exposure     67 rows  2016-08 .. 2026-07   **NOT CONTIGUOUS: gap 2022-01..2026-05**
  BACKUP: data/research.db.bak_20260907_mufap  (1.0G)
  Loader: ~/psx-research/bin/mufap_load.py  (preview|load|exposure|status; --commit to write)
    It SHELLS OUT to the CLI on purpose so the accounting-negative decode, the composite row
    key and the outlier flags have ONE implementation and cannot drift.
    `status` prints an explicit NON-CONTIGUITY gap report -- do not read MIN..MAX as a series.

### 3. SBP — rates.py is no longer paste-fed
  ~/psx-research/bin/rates.py gained `fetch`, reading DMMD circulars (HTML, "from X% to Y%").
  MUST run under a modern TLS stack:  uv run --python 3.11 python bin/rates.py fetch
  It REFUSES on LibreSSL with the remedy. Full evidence: ~/psx-research/SBP_REACHABILITY_FINDING.md

### 4. Tax hypothesis — data half answered, research half UNFINISHED
  ~/psx-research/mufap-tax-analysis/FINDINGS.md   <- READ THIS, it is self-contained
  Verdict: NOT supported as primary driver; the rate cycle is sufficient.
    net sales vs policy rate r=+0.704 (n=25); 85% of flow magnitude is fixed-income;
    in the negative months money-market bled WHILE EQUITY TOOK INFLOWS.
  RESEARCH LANDED 7 Sep 12:0x and CHANGED THE VERDICT to **NOT SEPARABLE**. The tax is REAL
  but misdated: Finance Act **2024**, effective **1 Jul 2024**, and DIFFERENTIAL — debt-heavy
  fund dividend tax 15% -> 25%, equity funds unchanged. That is exactly the category the
  outflow sits in, so my earlier "wrong asset class" argument was over-concluded and is
  retracted in FINDINGS.md. SBP's first cut was June 2024, three weeks before the tax, and
  net sales starts 2024-05 = TWO pre-treatment months. No DiD possible.
  Raw research: ~/psx-research/mufap-tax-analysis/tax-research-raw.json (+ journal.jsonl)
  ONLY CLEAN DISCRIMINATOR LEFT: tax keys on DEBT SHARE (50% bright line), rates key on
  YIELD — different axes. Classify funds by debt share (mufap_panel + allocation) and test
  which one flows track. tab=payout also gives per-fund distribution dates.

## ============ IMMEDIATE NEXT STEPS, IN ORDER ============
1. `cli-printing-press lock release --cli mufap-pp-cli` (stale lock from this session)
2. Promote MUFAP. No library copy exists (`$PRESS_LIBRARY/mufap` absent) so it is Path A:
     cli-printing-press lock promote --cli mufap-pp-cli --dir <RUNDIR>/working/mufap-pp-cli
   Then archive manuscripts per Phase 5.6, then Phase 6 publish if wanted.
   EXPECT ONE PUBLISH BLOCKER: coverage_hollow ["backfill"] = upstream #4539, the same gate
   that blocked nccpl. It blocks `publish validate`, NOT the CLI. Do not mis-annotate
   backfill as read-only to get past it.
3. Re-run the tax-timeline research (item 4 above).
4. File the retro. 8 machine bugs are queued below and NONE are filed.

## ============ SETTLED FACTS — DO NOT RE-DERIVE ============
TRANSPORT: no Surf, no browser, no cf_clearance. Plain Go stdlib + one header set already in
  the spec's required_headers: browser Accept (GET 403s on */*) plus
  X-Requested-With/Origin/Referer (POST 403s without).
SBP: NOT Cloudflare-blocked. Headers are necessary but NOT sufficient -- LibreSSL 2.8.3
  (macOS python3, /usr/bin/curl) 403s; OpenSSL 3.5.7 gets 200. My earlier "TLS is irrelevant"
  claim was WRONG and is corrected in SBP_REACHABILITY_FINDING.md.
CHART_LEVELS in rates.py is the CEILING rate (policy+100bp), not the policy rate: 16/17 match
  as ceiling vs 11/17 as policy. The control was comparing two different series.
ACCOUNTING NEGATIVES: MUFAP writes "(4.97)" for -4.97 and NEVER uses a minus. ~25% of daily
  cells. Undecoded, this inverted the sign of the equity market (dispersion +3.15 vs true -3.70).
  Handled in mufap.ParseNumber. Verified across a decade: negative share 1.7% (2016) ->
  43.6% (2018) -> 2.6% (2025), tracking market conditions.
ROW KEY is Sector|Category|Fund Name. Fund name alone is NOT unique: 49 of 388 collide because
  VPS pension funds reuse one name across three sub-funds.
OUTLIERS are FLAGGED (Tukey fences), NEVER deleted. A fixed +/-200pp band was tried and
  rejected: it deleted 8 of 9 real VPS-Equity funds on "3 Years" and kept only the artefact.
FIVE date encodings: YYYY-MM-DD (html query) / "Mon DD, YYYY" (Validity Date) / M-YYYY
  (allocation POST) / YYYY (unit-holder POST) / Month=&Year= (net sales partials, NOT in the CLI).
tab=payout has NO "Validity Date" column -- its date column is "Payout Date".
tab=pricing and tab=ter are current reference data, NOT date-filtered panels.
`dump` not `export`, and the file is dump.go, on purpose: a static check keys on the filename
  export.go and demands an API-path resolver a local-store dumper cannot honour.
The five promoted single-endpoint commands use the COLLAPSED path (`allocation`, not
  `allocation get`).
vps group is NON-PRODUCTIVE UPSTREAM: retired-cash/withdrawals return empty Tables,
  age-wise returns placeholder data. Disclosed in README Known Gaps item 7.
BACKFILL OPERATIONAL TRAP: it walks the whole range inside ONE invocation bounded by root
  --timeout (default 1m). A year-walk silently stopped at 33 of 131 dates. Always pass
  --timeout 6h and NEVER --quiet (which collapses the summary to a bare number and hid it).

## ============ MACHINE BUGS (retro candidates) — 8, NONE FILED ============
1. `http_transport: browser-http` in an internal YAML spec silently ignored by generate.
2. `probe-reachability` stdlib probe omits a browser Accept header -> false browser_http.
3. `doctor` calls an HTML-serving API "unreachable" on HTTP 200 (worked around: mufapHealthGet).
4. dogfood's novel-feature depth check compares the advertised FULL PATH to Cobra's leaf
   Name(), so any nested novel command reports a false mismatch.
5. `scorecard --live-check` SIGBUSes intermittently when the binary is rebuilt under it.
6. `resource-path:export` static check keys on FILENAME and assumes any export.go is an
   API-reading dumper; a local-store dumper cannot satisfy it and is failed as CRITICAL.
7. Single-endpoint resources get their path COLLAPSED but the generator keeps the two-word
   form in its emitted Example. All five promoted commands shipped a broken example that
   "works" only because Cobra swallows the stray positional.
8. `--quiet` can suppress a truncation error on a write command (see BACKFILL trap above).
Plus: 2 tools-audit thin-short findings in DO-NOT-EDIT generated files are generator-template
candidates; 28 gosec findings are all generated/platform, zero in hand-authored MUFAP code.

## ============ NOT DONE / PERMISSIONS ============
- No browser was ever driven. No browser credentials read.
- research.db writes were owner-approved and are additive (new tables only).
- Net sales is NOT in the CLI; adding `backfill netsales` is the reproducibility gap.
- The investor-wise net-sales partial is reachable but unharvested. It is the SHARPEST
  remaining test of the tax story: individuals redeeming while institutions do not.

## ############################################################################
## SESSION UPDATE — 7 Sep 2026, 14:0x PKT. THREE CLAIMS ABOVE WERE WRONG.
## ############################################################################

### CORRECTION 1 — "MARKER IS VALID ... 0 files newer" was FALSE when written.
The line above compared MTIMES. The marker stores CONTENT HASHES for 175 files, and 4 of
them had changed after it was written: internal/cli/backfill.go, dispersion.go,
freshness.go, and spec.yaml (the morning polish edits at 10:48-11:02). Three of those were
BEHAVIOURAL, not cosmetic:
  backfill.go   -- new DatesNeedFetch / DatesForwardDated / CarriedRowsDropped fields, and a
                   changed commit rule: carried ragged rows are now REFUSED, not stored.
  dispersion.go -- zero-fund dates are now KEPT in the series with fund_count 0 (they were
                   dropped, which made a real observation look like missing data).
  freshness.go  -- lag re-anchored to the MODAL validity date, signed lag, partition fixed.
So the 22:50 live matrix did NOT cover the shipped code. VERIFY MARKERS BY HASH, NOT MTIME.

### CORRECTION 2 — the live matrix takes ~50 SECONDS, not an hour.
"do NOT waste an hour re-running the 117-test live matrix" is wrong by two orders of
magnitude. A full `dogfood --live --level full` completed in ~50s. It has been re-run twice
today. The marker at proofs/phase5-acceptance.json is now GENUINELY valid:
  status pass, level full, 117/117, 175/175 file hashes match, 0 .go files newer.
Re-running it is cheap. Do it whenever source changes.

### CORRECTION 3 — the coverage_hollow gate blocks PROMOTE, not just publish.
`lock promote` fails outright:
  Error: promoting CLI: phase5 gate failed: phase5 acceptance has hollow coverage for: backfill
ROOT CAUSE (established, not guessed). "Hollow" means a novel feature was never EXECUTED.
The live matrix runs a locally-mutating command with --dry-run ONLY, so `backfill`'s real
write path never runs, so the feature is scored as never executed.
WHAT WAS RULED OUT, so nobody repeats it:
  * Renaming the advertised command to a leaf that DID run ("backfill daily") -> still
    hollow, now reported as hollow_features:["backfill daily"]. Not a name-matching bug.
  * `--allow-destructive` -> DOES NOT WORK. Re-ran the whole matrix with it; the backfill
    tests still carried --dry-run and the marker still came back hollow. That flag governs
    endpoints classified destructive-at-AUTH, not locally-mutating commands.
  * No env override exists. The binary's only PRINTING_PRESS_* vars are CLIENT_PROFILE,
    DOGFOOD, HOME, LIBRARY_PUBLIC, MCP_BOUND_PROFILE, REPO_ROOT, SCOPE, VERIFY,
    VERIFY_LIVE_HTTP. `lock promote` takes only --cli and --dir. `library` has only list
    and migrate -- there is no add path.
  * phase5-skip.json is not an escape: its schema requires status=="skip" and the press
    rejects a pass marker there. It would also downgrade a real 117/117 full pass.
CONSEQUENCE: on press v4.31.7 a CLI whose novel feature is a locally-mutating command is
STRUCTURALLY UNPROMOTABLE by any honest route. backfill is already annotated correctly
(mcp:read-only=false, mcp:local-write=true, pp:parent-group=true). The only way through is
to mis-annotate it read-only, which is forbidden and would be a lie about a command that
writes. NOT DONE. The marker was NOT hand-edited either.
STATUS: MUFAP remains in the run dir, unpromoted, and the library has no mufap entry. The
CLI ITSELF IS FINE: builds clean, go vet clean, go test green, 117/117 live, marker valid,
scorecard 85/100 Grade A. Only the library promotion is deferred, pending an upstream fix.

### ALSO MEASURED — pp:happy-args cannot carry a timeout, and a truncated run scores as pass.
`backfill allocation --help` says "Expect it to be slow and to need a raised --timeout" and
its example uses --timeout 30m, but its pp:happy-args carries no timeout, so the matrix runs
it at the CLI root default of 1m. Measured under exactly that condition: truncated mid-fanout
at 60s, dates_stored 0 of 1, 93 rows discarded, errors[] populated -- and EXIT 0. The harness
would score that truncated no-op as a pass. Filed as part of the retro.

### IN FLIGHT AS OF 14:0x
- mufap_exposure gap fill 2022-01..2026-05 (53 months) RUNNING, owner-approved.
    script  exposure-gap-2022-2026.sh      log  proofs/exposure-gap-2022-2026.log
    Quarter chunks, 3 attempts each with 180s backoff, 45s between chunks (the 2021 lesson:
    a year-sized run 403s after sustained fetching).
    Rate observed on 2022-Q1: 5m37s for 3 months (~1.9 min/month), 0 403s.
    ETA ~16:30-17:00 PKT; per-month cost rises with reporting-fund count (195 in 2016 vs
    551 by 2026), so treat 1.9 min/month as a floor, not the average.
- Tax research re-run (the 4 cached angles replay free) and the retro classification are both
  running as workflows.

### TAX RESEARCH — CLOSED. See FINDINGS.md, which now has the full write-up.
Narrow question ANSWERED NO: no tax measure touching funds has an effective date in
2025-10-01..2026-04-30. Strong negative, not a failed search: FBR's own consolidated Income
Tax Ordinance "amended upto 20 Feb 2026" -- a snapshot from INSIDE the window -- is textually
identical to the 31 Jul 2025 consolidation on all 42 mutual-fund passages.
The real tax event is FINANCE ACT 2024, effective 1 JUL 2024, differential: dividend on funds
deriving >=50% of income from profit on debt went 15%->25% (filer), equity funds unchanged at
15%. INDEPENDENTLY VERIFIED against EY and Crowe FY2024-25 commentaries, not just agent output.
TWO CORRECTIONS I ADDED that the agents' own rewrite missed, both in FINDINGS.md:
  (1) The fund-vs-BANK-DEPOSIT penalty NARROWED from 10pp to 5pp on 1 Jul 2025, because
      FA2025 raised s.7B deposit tax 15%->20% (verified). So the fund's relative tax
      disadvantage was WIDEST from 1 Jul 2024 -- when flows were strongly POSITIVE -- and had
      HALVED six months before the outflow began. Second independent timing argument against
      tax. Caveat: NSS and govt securities stayed at 15%, so the fund-vs-govt-securities gap
      did NOT narrow.
  (2) The 50% bright line existed for EXACTLY 12 MONTHS. FA2025 replaced the cliff with a
      PROPORTIONAL split, which has no discontinuity. So the RD window is 2024-07..2025-06,
      and the 2026-01..04 outflow CANNOT be tested with the bright line. Do not write a
      discontinuity variable for 2026 dates.
CAUTION ON PROVENANCE: FINDINGS.md and this HANDOFF were edited at 12:03-12:04 by a research
agent mid-flight, BEFORE the workflow's own verify and settle stages had run. That text was
not reviewed by the verification it claimed to rest on. I have since hand-checked its
load-bearing claims against primary/secondary sources and they hold, but the episode is why
tax-research-raw.json and tax-research-journal.jsonl exist in the tax-analysis dir: the agents
wrote those themselves, unrequested.

## ############################################################################
## PROMOTED — 7 Sep 2026 16:1x PKT. "STRUCTURALLY UNPROMOTABLE" IS NOW OBSOLETE.
## ############################################################################
    cli-printing-press lock promote --cli mufap-pp-cli --dir <RUNDIR>/working/mufap-pp-cli
    -> {"promoted": true, "library_dir": "$HOME/printing-press/library/mufap"}
Marker: status pass, level full, 117/117, coverage_hollow ABSENT, fingerprint valid
(175 files, 0 drifted). backfill: 0 tests with --dry-run, 6 REAL write invocations passing.
Lock released. Manuscripts re-archived to match. Embedded proof at
  library/mufap/.manuscripts/20260906-153304-50daea87/proofs/phase5-acceptance.json

### HOW, and it took TWO fixes, not one
1. THE PRESS IS NOW LOCALLY PATCHED. press-fix-4539.diff applied to a clean v4.31.7
   checkout (commit a4c1e26), installed over ~/go/bin/cli-printing-press.
     stock binary backed up : ~/printing-press/cli-printing-press.bak_4.31.7_20260907
     patch + diff + notes   : ~/printing-press/press-patched-4539/PROVENANCE.md  <- READ IT
     WARNING: `--version` still prints 4.31.7. It is NOT stock. sha256 differs:
       stock   9cac15aa2e1a4d27c1d62577d3a097aa8ded78c46939aa35b8987d8f4d749755
       patched 39a10b8d523b2579b7f2e66d392eb16dc70e9cbbb9e0bf0b6bbb1f6830545fa0
     v4.32.0 does NOT contain this fix (verified against the tag: localWrite appears 0
     times, the old useDryRun line is still at live_dogfood.go:1640). ANY PRESS UPGRADE
     WILL REGRESS THIS. Re-apply the patch until upstream lands #4539.
     Verified before install: go vet clean, `go test ./internal/pipeline/` ok (126s),
     full `go build ./...` clean.
2. THE FEATURE IS NOW DECLARED AT ITS EXECUTABLE LEAF. The patch alone was NOT enough:
   it removed the forced --dry-run (6 real write invocations appeared) but coverage stayed
   hollow, because the feature was declared as the bare parent group `backfill`, which is
   never itself invoked. That is the SECOND half of the defect (the dogfood command walker,
   upstream psx WU-3). Repointed novel_features/novel_features_built to `backfill daily`
   in BOTH research.json and .printing-press.json. That string is the feature's OWN
   documented `example`, so it is the accurate executable path, not a dodge.

### WHAT WAS REJECTED ON THE WAY, so nobody retries it
  --allow-destructive          : does nothing here (destructive-at-AUTH, not local writes)
  renaming to a leaf, unpatched: still hollow (dry-run was the binding constraint then)
  omitting --research-dir      : DOES yield a marker with no coverage_hollow key at all
                                 (the nccpl/psx shape, which is how all 8 earlier CLIs were
                                 promoted) -- but it certifies NOTHING about novel-feature
                                 coverage. Offered and DECLINED in favour of the real fix.
  hand-editing the marker      : never done.
  mis-annotating read-only     : never done. backfill remains mcp:local-write=true.

### RETRO — DONE analytically, NOTHING FILED (needs owner confirmation for GitHub)
Final slate: 4 Do-P2, 1 Skip, 3 Drop, ZERO P1.
  Do-P2: doctor HTML-200 misread | resource-path filename-keyed check |
         collapsed-path stale Example | dogfood novel-command walker
  Skip : quiet-suppresses-truncation (Step B: 2 APIs, not 3)
  Drop : spec-http-transport-ignored  -- DISPROVEN by disassembly. `browser-http` is a real
           wired transport meaning "stdlib with HTTP/2 disabled"; no spec in this run ever
           declared it (all three say `standard`). Handoff bug #1 was wrong. The operator
           carried probe-reachability's MODE name `browser_http` into the README as if it
           were a spec value.
         scorecard-live-check-sigbus -- falsified experimentally; the guard already ships.
         stale-acceptance-marker-unwarned -- MY OWN finding from this session, and it is
           FALSE. The press already fingerprints and validates at the point of use and
           reports `changed source files`. I inferred "nothing warns" from seeing a stale
           marker without ever testing whether promote would refuse. It would have.
Also refuted: my pp:happy-args-cannot-carry-a-timeout claim. It CAN
(parseHappyArgsAnnotation + overlayLiveDogfoodFlags accept any --flag=value), so
`backfill allocation` truncating at the 1m default is a ONE-LINE annotation fix in this
CLI's own backfill.go, not a machine bug. Worth doing: add --timeout=30m to its
pp:happy-args so the harness stops scoring a truncated no-op as a pass.
One unresolved disagreement: the adversarial reviewer put novel-feature-accounting at SKIP
(Step D -- already psx WU-3 + upstream #4539) while the prioritizer listed it Do-P2 and
claimed zero overrules. Trust the SKIP.

### RETRO FILED 7 Sep 2026 ~16:3x (owner-approved). Task 4 COMPLETE.
Retro doc: manuscripts/mufap/20260906-153304-50daea87/proofs/20260907-162500-retro-mufap-pp-cli.md
  (273 lines, scrub-clean; copy in the run's proofs/)
Issues: #4612 doctor HTML-200 | #4613 resource-path filename-keyed | #4614 collapsed-path Example
Comment on #4539 (NOT a new ticket, per Step D): blocks promote not just publish;
  --allow-destructive is a red herring; v4.32.0 lacks the fix; the patch is necessary but NOT
  sufficient (bare parent group stays hollow); the gate is INERT without --research-dir, which
  is why it never fired for the 8 earlier CLIs.

### research.db: NEW TABLE policy_rates_clean (owner-approved, ADDITIVE — raw table untouched)
33 rows: 27 basis='dmmd-circular' (verbatim), 5 basis='derived-period-shift' (LABELLED, not
primary), 1 basis='sbp-mps-primary' (the 2025-12-16 = 10.5 cut, verified against MPS-DEC-2025).
Acceptance: r(net_sales, policy_rate) from policy_rates_clean = +0.7078, reproducing the
published +0.704; from raw policy_rates it is +0.6427. USE THE CLEAN TABLE for anything
rate-related. The raw table's period_end rows return the PREVIOUS rate and it carries a
fabricated 15.0 on 2025-12-16.

### EXPOSURE FILL — TWO OPERATIONAL TRAPS FOUND 7 Sep 16:4x. BOTH ARE MINE. READ BEFORE RERUNNING.

**TRAP A — a 12h --timeout turns a dead TCP connection into a multi-hour hang.**
`mufap_load.py` invokes the CLI as `exposure ... --timeout 12h`. At 16:09 the 2025-04..06
chunk opened a connection to MUFAP and stalled. Measured at 16:39, 30 minutes in:
    CPU 2.13 -> 2.20s over 90 seconds        (frozen; a working fetcher burns far more)
    same socket, same local port, 12+ min    (one ESTABLISHED connection, no new ones)
    netstat Send-Q = 105 bytes STUCK unsent  (Cloudflare not ACKing)
    data.db mtime still 16:09                (zero rows written in 32 minutes)
With a 12h request timeout there is nothing to break the stall. The documented backfill trap
was that --timeout is too SHORT by default; the inverse is just as dangerous. A per-request
timeout of a few minutes plus retry is what this needs, not a giant global one.

**TRAP B — my wrapper's success check was not chunk-scoped, so a failed chunk read as success.**
`exposure-gap-2022-2026.sh` used:
      if tail -8 "$LOG" | grep -q "WROTE"; then ok=1; break; fi
After I killed the stalled child, `tail -8` still contained the PREVIOUS chunk's
"WROTE mufap_exposure 3 row(s)" line, so grep matched, the retry loop broke on a stale signal,
and the wrapper advanced. **2025-04, 2025-05 and 2025-06 were silently skipped with no retry.**
Same defect class as the PSX control-factor lesson: never test a global signal for a local
condition. Verify against the DATABASE, not a log.

**RECOVERY, running:** `exposure-cleanup.sh` (log `proofs/exposure-cleanup.log`). It waits for
the original wrapper to exit (two writers on one SQLite file with busy_timeout=0 is how you
lose a commit), then fills whatever months are still absent, **one month at a time**, with a
13-minute wall-clock WATCHDOG that kills the stalled CLI child, and success verified by
`SELECT COUNT(*) ... WHERE month=?` rather than by grepping a log. A month that fails all 3
attempts is left MISSING and logged as such — never interpolated.

### POST-PROMOTION VERIFICATION DONE 7 Sep 16:5x — the SHIPPED artifact, not the run copy
Smoke-tested `library/mufap/mufap-pp-cli` (the promoted binary): **9/9 read-only local-store
commands pass** — panel, rates, coverage, universe, dispersion, freshness, verify allocation,
dump, exposure. `dispersion` emits its Tukey warning, so this morning's behavioural change is
live in the shipped artifact. `verify allocation --month 7-2026` returns real data
(assets_percent 100.58, deviation 0.40).
**`verify allocation` IS correctly nested** — its usage line reads
`mufap-pp-cli verify allocation [flags]`. So dogfood's depth-mismatch WARN is confirmed a
FALSE POSITIVE from the runtime side, independently of the source-side analysis.
**MCP server verified end-to-end from the promoted tree**: builds clean, initialize handshake
OK (protocol 2024-11-05, serverInfo Mufap), `tools/list` returns **36 tools**, and the
`readOnly` annotations are RIGHT — `backfill_daily`, `backfill_allocation` and
`backfill_monthly` all carry `readOnly=False`. That is the very annotation the forbidden
read-only hack would have falsified, so the honest labelling survived into the MCP surface.
Checked and NOT findings (verified press-wide, not MUFAP defects): every library binary
reports `--version 0.0.0-dev` (built without version ldflags); and declared
`mcp_tool_count` (9) counts endpoint-derived tools only, not the 36-tool runtime surface —
declared counts across the library run 1/3/9/22/42/235, so it is definitional.

### TWO REMAINING IMPROVEMENTS — DO THEM AS ONE BATCHED CYCLE, NOT SEPARATELY
Both change source/metadata, so each one on its own would invalidate the marker fingerprint and
force a full regenerate-and-re-promote. Batch them:
  1. **`backfill allocation` pp:happy-args needs a timeout.** Its own help says "Expect it to be
     slow and to need a raised --timeout" and its example uses `--timeout 30m`, but the
     annotation carries none, so the live matrix runs it at the CLI root default of 1m.
     MEASURED: truncates mid-fanout at 60s, `dates_stored` 0 of 1, 93 rows discarded,
     errors[] populated, and **exit 0** — the harness scores a truncated no-op as a pass.
     Fix: add `--timeout=30m` to the `pp:happy-args` annotation in internal/cli/backfill.go.
     (`pp:happy-args` DOES accept arbitrary --flag=value; that was verified.)
  2. **Declare `mcp.intents`.** `cli-printing-press mcp-audit` returns for mufap alone:
     `"recommend": "declare mcp.intents for common workflows"` (intent_count 0, endpoint_count
     10). Every other library CLI gets "ok", so this is triggered by surface size and is a
     genuine MUFAP-specific improvement. Natural intents: backfill-then-query the panel;
     build the rate series; audit coverage before trusting a series.
THE CYCLE (must run when MUFAP is NOT under load from an exposure fill — step 3 does a full
551-fund fanout that takes ~7 min):
    1. edit internal/cli/backfill.go + spec/.printing-press.json intents
    2. go build ./... && go vet ./... && go test ./...
    3. XDG-isolated: cli-printing-press dogfood --dir <run>/working/mufap-pp-cli --live
       --level full --research-dir <run> --timeout 30m --write-acceptance <run>/proofs/phase5-acceptance.json
    4. confirm marker: status pass, coverage_hollow ABSENT, 0 files drifted, backfill args
       contain NO --dry-run
    5. cli-printing-press lock promote --cli mufap-pp-cli --dir <run>/working/mufap-pp-cli
    6. re-copy research.json / marker / HANDOFF.md into manuscripts/mufap/<run>/
REMINDER: the installed press is the LOCAL PATCH (see press-patched-4539/PROVENANCE.md). Step 3
only clears coverage_hollow on that binary; stock 4.31.7 will re-hollow it.

## ############################################################################
## EXPOSURE FILL COMPLETE — 7 Sep 2026 17:20:58. THE NON-CONTIGUITY WARNING IS
## NOW OBSOLETE. mufap_exposure IS A FULL 120-MONTH CONTIGUOUS SERIES.
## ############################################################################
    53/53 target months committed (2022-01..2026-05)
    FULL TABLE: 120 rows, 2016-08..2026-07, 0 duplicates, **ZERO GAPS**
    Verified by recursive-month diff, not by reading MIN..MAX.
Anything earlier in this file saying "NOT CONTIGUOUS: gap 2022-01..2026-05" or telling you not
to read MIN..MAX as a series is SUPERSEDED. You can now read it as a series — subject to the
one caveat below.

### RECOVERY WORKED. The corrected cleanup did what the buggy wrapper could not:
    17:12:17  wrapper exited; cleanup queried the DB and found 2025-04/05/06 missing
    17:16:xx  2025-04 COMMITTED (verified in db)
    17:18:38  2025-05 COMMITTED after 120s
    17:20:38  2025-06 COMMITTED after 100s
    17:20:58  DONE. still missing: (none)
Every success was confirmed with `SELECT COUNT(*) ... WHERE month=?`, never by grepping a log —
which is precisely the defect that lost those three months in the first place.

### THE ONE REAL CAVEAT: 73 of 120 months EXCLUDE some funds, 771 funds in total.
`mufap_load.py` drops a fund whose fetch failed and stores the count, so a short month cannot
silently read as complete. But `stocks_pkr_mn` and `equity_share` are biased DOWNWARD in those
months. Severity distribution:
    clean (0 excluded)   47 months
    trivial (<2%)        46
    minor (2-10%)        21
    MATERIAL (>=10%)      6   <-- treat these as not comparable
THE SIX MATERIAL MONTHS ARE ALL IN THE OLD 2016-2021 BLOCK, none in what was filled today:
    2020-10  96 failed / 31.5% of reporting funds   equity_share 0.2451
    2019-09  58 / 22.2%   0.3591
    2019-08  45 / 18.3%   0.3497
    2020-08  44 / 15.5%   0.2926
    2016-09  34 / 15.5%   0.4574
    2021-08  14 / 10.6%   0.2484
Gate on `failed_funds` before using `equity_share` cross-sectionally. The 2022-2026 block just
filled is clean: 0-8 excluded per month, all <=2.6%.

### Sanity of the new block (equity share collapse then partial recovery)
    2022-01 0.2489 (275 funds)   2023-01 0.1252 (306)   2024-01 0.0982 (355)
    2025-01 0.1118 (385)         2026-01 0.1890 (448)   2026-05 0.1752 (480)
Consistent with the documented fall from ~52% (2017) toward ~17% (2026), with a 2026 rebound.

### postclose 17:00 RAN CLEAN — the SQLite contention never bit.
    17:00:08 start [session_today=yes tick_age=True eod_today=True capture_log=True]
    17:19:54 done
Zero lock / busy / ERROR / WARN lines. harvest completed 745 symbols, fail=0. The overlap risk
with the exposure cleanup (both writing research.db with busy_timeout=0) did not materialise —
postclose finished at 17:19:54, 44 seconds before the cleanup's last commit at 17:20:38.
All 7 designed PSX capture slots also captured today; the extra 16:00 plist firing captured as
`adhoc` and the 16:25 one skipped cleanly on the holiday guard (exit 0, not a failure).

### TIMEOUT FIX SHIPPED — re-promoted 7 Sep 2026 17:4x. The batched cycle is now HALF done.
`backfill allocation`'s annotation is now
    "pp:happy-args": "--from=2026-07-31;--to=2026-07-31;--timeout=30m"
and its first Example line carries `--timeout 30m` too. `monthly` (line ~701) is deliberately
UNCHANGED — it is a single cheap request and needs no timeout. Measured evidence is recorded as
a code comment above the annotation so it does not get "simplified" away.
Verified: go build / go vet / `go test ./internal/cli/...` clean; binary rebuilt; live matrix
117/117 PASS, marker status pass level full, **coverage_hollow ABSENT**, **0 of 175 files
drifted**, **0 backfill tests carrying --dry-run**, both allocation tests pass with the timeout
in argv. `lock promote` -> promoted: true. Manuscripts re-synced.

**GOTCHA FOR THE NEXT GATE YOU WRITE.** The harness parses `--timeout=30m` out of
pp:happy-args and re-emits it as SEPARATE argv tokens: the test args read
`--timeout 30m`, not `--timeout=30m`. My first gate string-matched the `=` form and
false-failed a perfectly good run. Match `--timeout[= ]30m`.
Second gotcha: do NOT gate on `dates_stored==1`. The whole matrix finished in 36s because the
allocation fanout served from the CLI's response cache (populated by an earlier manual fetch),
so the month was already covered and nothing was written. That is correct behaviour and not a
promotion criterion.

### STILL OPEN (the other half): declare `mcp.intents`. DEFERRED ON PURPOSE, not forgotten.
`cli-printing-press mcp-audit` returns for mufap alone:
`"recommend": "declare mcp.intents for common workflows"` (intent_count 0, endpoint_count 10).
Every other library CLI gets "ok", so it is triggered by surface size.
WHY DEFERRED: `spec.yaml` has **no `mcp:` section at all**; `mcp-audit` reports
**intent_count 0 for all nine** library CLIs, so there is no worked example anywhere on disk;
and the press publishes no spec schema (`cli-printing-press schema` exposes only phase5-marker,
phase5-skip, traffic-analysis). All that is known of the shape comes from two validator strings
in the binary: `mcp.intents[%d]: description is required` and
`mcp.intents[%d] (%s): name %q appears more than once` — i.e. entries have at least `name` and
`description`, and names must be unique. Required fields beyond those are UNKNOWN.
Declaring it blind on an already-promoted CLI risks failing spec validation and burning a
verify+promote cycle. Get the shape first (upstream docs, or a generator template), then add
intents such as: backfill-then-query the panel; build the market-implied rate series; audit
coverage before trusting a series. Then re-run the same cycle documented above.

### mcp.intents SHIPPED — re-promoted 7 Sep 19:0x. THE BATCHED CYCLE IS NOW COMPLETE.
mcp-audit on the promoted library copy went from
  {tool_design: "endpoint-mirror", intent_count: 0, recommend: "declare mcp.intents..."}
to
  {tool_design: "intent", intent_count: 3, recommend: "ok"}
Marker: status pass, level full, 117/117, coverage_hollow ABSENT, **176** source files
(intents.go added), 0 drift. MCP surface 36 -> **39 tools**. Manuscripts re-synced.

**I WAS WRONG EARLIER AND THIS CORRECTS IT.** I told the owner the mcp.intents schema was
undocumented, had no worked example anywhere, and that declaring intents meant guessing.
FALSE. The v4.31.7 source (cloned into the scratchpad for the #4539 patch) contains the
complete `Intent` / `IntentParam` / `IntentStep` structs at internal/spec/spec.go:2236+, the
full `validateIntents` rules at :5873, AND a worked YAML fixture in
`TestMCPIntentsParse` (internal/spec/spec_test.go:4118). I had the answer on disk and did not
look. If the press source is ever unavailable, the same facts are recoverable from the
validator strings in the binary.

SCHEMA (authoritative):
  mcp.intents[].name          snake_case, unique, required
  mcp.intents[].description   required
  mcp.intents[].params[]      name (must match the MCP property-key regex — an illegal key
                              "bricks the calling agent session"), type ∈ {string, integer,
                              boolean} ONLY, required, description
  mcp.intents[].steps[]       >=1 required; endpoint = dotted path that must resolve against
                              spec resources; bind = param -> ${input.<name>} or
                              ${capture.<field>} (capture must come from a PRIOR step; a
                              non-${ value passes through as a literal); capture unique,
                              never "input"
  mcp.intents[].returns       must match a step capture; defaults to the last step's capture

**THE STEP EVERYONE WILL MISS: editing spec.yaml ALONE DOES NOTHING.**
Intents are GENERATED Go code. `HasMCPIntents` (generator.go:5594) gates emission of
`internal/mcp/intents.go`, and `mcp-audit` counts `mcplib.NewTool(` occurrences IN THAT FILE
(mcp_audit.go:148-150) — it never reads the spec. MUFAP's MCP surface is a runtime Cobra-tree
walk, so a spec-only edit is INERT: it parses, builds, tests green, and produces zero tools.
The wiring command is:
      cli-printing-press mcp-sync <cli-dir>
Verified safe: run on a throwaway copy first (I did). It created intents.go with exactly the
3 declared tools, the build stayed clean, and all 10 hand-written novel commands survived.
It also rewrites .printing-press.json — check that novel_features still says
`"command": "backfill daily"` afterwards (it did).
ALSO NOTE: mcp-audit scans INSTALLED LIBRARY CLIs, not a run dir. It will keep reporting
intent_count 0 until you promote. That is not a defect; I misread it once.

**DESIGN CHOICE, deliberate:** every bind uses ${input.<name>} only, never
${capture.<field>}. The validator accepts a capture field, but these endpoints return LISTS
and the fan-out semantics of binding one field out of a list response are unspecified. Binding
only declared inputs keeps each intent deterministic. `endpoint_tools` was NOT set to
`hidden` — that would strip the 10 per-endpoint tools, a breaking change; intents are additive.

**NEW MACHINE DEFECT FOUND, NOT YET FILED — generated intent tools carry NO MCP annotations.**
`internal/mcp/intents.go` emits `mcplib.NewTool("amc_fund_roster", ...)` with no
ReadOnly/Destructive hints, so all three intents report `readOnlyHint=false` even though every
endpoint they compose is read-only (verified: amcs_list, funds_by-amc, allocation_get,
dates_list, unitholders_get, payouts_list all carry mcp:read-only in tools.go). An MCP host
must therefore treat a purely-read composition as possibly mutating, and may gate it behind
write approval. The generator already knows each step's endpoint and each endpoint's read-only
flag, so the fix is the conjunction: readOnlyHint = AND(all steps read-only).
EVIDENCE-BAR CAVEAT, stated honestly: this fails the retro's Step B, because MUFAP is the
FIRST CLI in the library to declare any intent (all 9 others were intent_count 0), so only one
API exhibits it. It is not a vendor quirk though — the annotation-free path is unconditional
generator template code, so recurrence is certain for any CLI that ever declares an intent.
Owner's call whether that clears the bar.
