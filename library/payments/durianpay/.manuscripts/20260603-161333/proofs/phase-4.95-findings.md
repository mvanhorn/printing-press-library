# Phase 4.95 Local Code Review — durianpay-pp-cli

## Review path chosen
Direct subagent dispatch via Agent tool: correctness + security + maintainability reviewers (always-on set), run in parallel per round.

## Autofix summary
12 findings autofixed in-place across 3 rounds (round 1: 11 fixes via fix-agent — unified SNAP envelope classifier w/ 202=success semantics, consolidated client constructor, bearer-token masking behind --show-token, typed ErrMissingCredentials + classifySNAPCallError (429→exit 7), pay body-builder dedup, shared snapAmount/snapAutoRef/snapTxnDate, resolveRoute legacy guard, expiresIn integer-seconds parse, snapDataSourceGuard ordering, --key doc note; round 3: payout rewired to canonical body builders). All verified by build/vet/tests + behavioral acceptance runs.

## Pre-4.95 fixes from earlier review phases (also in-place)
- body_sha256 timestamp-colon parsing bug in snap.SignedRequest (found by Phase 4.8 reviewer, fixed with regression test).
- SKILL description truncation, README config-path error, missing DURIANPAY_SNAP_PARTNER_ID doc, SNAP credential-gating note (Phases 4.8/4.9).

## Template-shape retro candidates
- Generated novel scaffolds emit Annotations {"mcp:read-only": "false"} (string false) — misleading default; an unset key would be cleaner. (generator template)
- Generated runSnapPost-style hand-code guidance should funnel transport errors through classifyAPIError so exit-code classification (429→7) is uniform — the press's Phase 3 templates could state this explicitly.
- SKILL.md description frontmatter is rendered truncated mid-sentence when narrative.headline is long ("...and a local Trigger phrases:") — generator should close the headline cleanly before appending trigger phrases.

## Out-of-scope retro candidates
- internal/cli/deliver.go (generated) allows plaintext http:// webhook delivery sinks without a warning. (security reviewer, severity low)

## Surface-to-user findings
None — no real-tradeoff findings; all closures were mechanical.

## Convergence outcome
Findings cleared at round 3. Round 2 verdicts: correctness PASS-converged, security PASS-converged, maintainability PASS with one residual duplication finding (payout inline bodies), fixed mechanically in round 3 and verified (build/vet/tests + dry-run; zero inline beneficiary bodies remain in payout.go).

## Post-fix simplification note
/simplify pass intentionally satisfied by the review rounds themselves: two dedicated maintainability passes drove the dedup/consolidation work (single constructor, single classifier, single body-builder per endpoint, shared primitives), and `deadcode` reported zero unreachable functions in scope. A further generic simplify pass would re-cover completed ground.
