# ti-pp-cli shipcheck + review — 2026-07-23

## Verdict: ship (anonymous lane live-proven; gated lane code-complete, not yet HIL-proven)

## Shipcheck: PASS 7/7 (verify, validate-narrative, dogfood, workflow-verify, apify-audit, verify-skill, scorecard). Scorecard 83/100 Grade A.

## Live-proven (anonymous, plain HTTP)
- part compliance TUSB320RWBR -> RoHS=Yes REACH=Yes (matches oracle)
- part compliance TPS53355DQPT -> RoHS=Exempt REACH=Affected (rich vocab)
- part compliance TPS54360BDDA -> Yes/Yes
- part compliance BADPART999 -> fail-closed error
- part coc -> all 5 signed statement PDFs downloaded (szzq088/119/087/077/195)

## Commands
- part compliance <opn> [--coc-dir]  (anonymous, lane contract)
- part coc --out-dir                 (anonymous, manufacturer-CoC evidence)
- part ratings <opn> --cookies       (gated: RoHS cell+exemptions, REACH, SVHC)
- part fmd <opn> --cookies --out-dir (gated: IPC-1752A Class-D XML)
- part evidence <opn> [--cookies]    (full ladder, degrades honestly)
- auth check --cookies               (cookie-freshness probe)

## Adversarial review (reviewer subagent) — findings fixed
- ERROR 1 JS injection via opn into credentialed eval  -> FIXED (ValidateOPN charset gate + json.Marshal literal + encodeURIComponent; pinned by validate_test.go)
- WARN 2 path traversal via opn in FMD filename        -> FIXED (ValidateOPN rejects ../ and slashes segments; validated in fmd/evidence before filename build)
- WARN 3 auth check bare invocation failed open exit 0 -> FIXED (removed help short-circuit; now exit 2 "cookies required")
- WARN 4 Browserbase session leaked on timeout/Ctrl-C  -> FIXED (Close uses fresh background ctx)
- WARN 5 undocumented BROWSERBASE_API_KEY/agent-browser -> FIXED (README Authentication prerequisites block)
- WARN 6 phantom --verbose troubleshoot                -> FIXED (removed from research.json + README)
- WARN 7 phantom sync/search/offline claims            -> FIXED (removed offline-store line from README)
- WARN 8 truncated literature description              -> FIXED (restored full szzq list in promoted_literature.go + SKILL)
- WARN 9 placeholder literals / TODOs                  -> FIXED (literature szzq087 example, TODO comments removed)
- WARN 10 cookie value in run() error interpolation    -> FIXED (redactArgs masks cookies-set value)
- WARN 11 ctx-ignoring sleeps                          -> FIXED (sleepCtx)
- WARN 12 JSON null -> "null" bypassing fail-closed     -> FIXED (null/None normalized to empty for rohs/reach)

## Open (not blocking anonymous ship)
- Gated commands (ratings/fmd/auth check/part evidence) NOT live-proven — need a real HIL myTI cookie snapshot.
- Findings 13/14/15 (single-ld-block parse, exit-code granularity, coc-failure discards verdicts) left as-is: latent/low, documented.

## Login-gated lane — LIVE PROVEN 2026-07-23 (real myTI HIL login via Browserbase)
Flow: created BB session via agent-browser -p browserbase → got debuggerFullscreenUrl from BB /sessions/{id}/debug → human logged into myTI in the live view → dumped 52 ti.com cookies (auth_session, tiSessionID, userType, user_pref_*) via `agent-browser --json cookies get` → CLI injected them into a FRESH BB session.
- auth check      -> {alive:true, cookies_injected:41}  exit 0
- part ratings TUSB320RWBR -> resolved login-gated report (pcid=320159), rohs_cell=Yes reach_cell=Yes  exit 0
- part fmd TUSB320RWBR     -> IPC-1752A v2.0 Class-D XML, 12206 bytes, real MainDeclaration + BusinessInfo(Authorizer "Randy Rath", Texas Instruments Inc) + 14 HomogeneousMaterial  exit 0
- part evidence  -> gated rungs (ratings, fmd) OK; anonymous rung hit transient TI 503 (IP throttled after a full session of /product hits)
Cookie reuse across a separate BB session CONFIRMED — validates the two-phase HIL-login design.
Post-fix: fetchProductPage now retries 429/503 with backoff and surfaces "rate-limited after N attempts" instead of a bare 503.
Secrets scrubbed: cookie JSON files shredded; BB session released.
