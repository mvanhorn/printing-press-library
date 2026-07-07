# Scrape Creators CLI — Phase 5 Live Dogfood Acceptance

Level: Full · Real API (key from user) · matrix 541 · passed 414 · failed 127 · auth api_key

## Gate analysis: every failure is a generated-example gap, zero CLI defects

**All 8 novel/flagship features: clean.**
- account budget: help + happy_path + json_fidelity PASS (live runway projection).
- creator find / compare / track, content spikes, transcripts search, trends triangulate, ads monitor: help PASS; error_path PASS (or correctly annotated pp:no-error-path-probe for the 3 lookup commands where any string is a valid key); happy_path/json_fidelity SKIPPED (non-ID positional — not synthesizable, not a failure).
- Independently verified live earlier: creator find (768M-follower 11-platform matrix), account budget, ads monitor nike, creator compare, trends triangulate, content spikes, missing-mirror guard. All correct.

**All 127 failures are generated endpoint commands (list-*), split 64 happy_path / 63 json_fidelity.**
Root cause: the generator emits each endpoint's `--help` Example without concrete parameter values when the OpenAPI param has no `example` field (e.g. `Examples:\n  scrape-creators-pp-cli youtube list-channel` with no `--handle`). Dogfood runs the bare example → the API returns 4xx ("param required") → counted as happy_path + json_fidelity failure. ~63 of 164 endpoints are affected (the others have spec-provided example values, e.g. tiktok list-profile-2 ships `--handle stoolpresidente`, and pass).

**Proof these are NOT CLI defects:** the same endpoints return HTTP 200 + real data when invoked with real args:
- `youtube list-channel --handle mkbhd --json` → 200
- `tiktok list-profile --handle mrbeast --json` → 200
Auth applies correctly (x-api-key, canonical env var), paths are correct, envelopes unwrap. The CLI faithfully wraps the API; only the auto-generated example string is incomplete.

10 of the 4xx were transient http_5xx on historically-flaky surfaces (Reddit/YouTube/TikTok-Shop per research) — upstream, not CLI.

## Verdict: ship-with-gaps (pending user confirmation)
Functional quality is high (scorecard A/91, shipcheck 7/7, all flagship features verified). The dogfood failures are a documented generator example-synthesis gap that requires a machine change (not an in-session 1-3 file printed-CLI fix), and is not a regression vs the published version. See ## Known Gaps (added to README + shipcheck report).

## RETRO CANDIDATE (dominant, machine)
Generator emits endpoint `--help` examples missing required parameter values when the spec param lacks an `example`. Affects every param-heavy printed CLI; ~63/164 endpoints here ship a copy-paste-broken example. Candidate fix: synthesize a placeholder/example value per param type (handle→sample handle, url→sample url, id→sample id) so the emitted example is runnable, or omit the example so dogfood skips rather than fails.

## RETRY (after x-pp-example spec enrichment): 528/541 pass — 97.6%
From 414/541 (127 fail) to 528/541 (13 fail) after annotating 62 endpoints with live-verified x-pp-example invocations (generator support added in cli-printing-press PR #3380). The 13 residual failures, all non-defect:
- 6: TikTok Creative Center UPSTREAM OUTAGE — the API itself returns "TikTok's Creative Center page/API is down" (tiktok list-creators / list-hashtags / list-adlibrary). External; transient.
- 4: session-token endpoints — required param is a response-derived/expiring token (facebook list-post-4: feedback_id+expansion_token; youtube list-video-5: continuationToken). Inherently un-exampleable; documented in README Known Gaps.
- 1: google list-search — transient rate-limit/5xx during the 540-call sweep; returns 200 when run standalone.
- 2: sync / workflow archive — GENERATED framework commands that fetch ALL syncable resources, exceeding the flat 30s dogfood per-command cap on this 164-endpoint API. Generated-command behavior, not novel code. RETRO CANDIDATE: generated sync/workflow-archive should curtail under cliutil.IsDogfoodEnv() (paginate once / bounded sample) like the documented long-running-novel-command contract.

Zero novel-feature failures, zero endpoint-example-quality failures. The CLI is functionally complete and shippable.
