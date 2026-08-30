# Shipcheck: synology-pp-cli

Command: `cli-printing-press shipcheck --dir . --spec ../../research/synology-spec.yaml --research-dir ../..`

## Result

| Leg | Result | Exit | Elapsed |
| --- | --- | --- | --- |
| verify | PASS | 0 | 40.136s |
| validate-narrative | PASS | 0 | 2.875s |
| dogfood | PASS | 0 | 3.085s |
| workflow-verify | PASS | 0 | 219ms |
| apify-audit | PASS | 0 | 291ms |
| verify-skill | PASS | 0 | 3.06s |
| scorecard | PASS | 0 | 9.346s |

Verdict: PASS, 7/7 legs.

## Before and after

| Metric | Before | After |
| --- | --- | --- |
| Shipcheck legs passing | 6/7 (scorecard failed: no `research.json`) | 7/7 |
| verify pass rate | 100% (32/32) | 100% (32/32) |
| scorecard total | 87/100, Grade A | 87/100, Grade A |
| Sample output probe | 2/2 | 2/2 |

The scorecard total is unchanged because the blockers fixed between the two runs were correctness and security defects, not the dimensions the scorecard scores. Its two soft spots stay: Cache Freshness 5/10 and Data Pipeline Integrity 7/10. Three dimensions are omitted from the denominator - MCP Surface Strategy, Auth Protocol and Live API Verification - the last of which is omitted precisely because live calls are impossible on this host.

## Blockers found and fixed

1. **scorecard leg failed** with "loading research for manifest scorecard persistence: open ...\research.json: file not found". Step 1.5e of Phase 1.5 had never written the file. Authored it to the documented schema, including `narrative` and `novel_features`; the leg then passed and the dogfood leg rewrote the file in canonical form with `novel_features_built` confirming both novel features.
2. **Seven security findings** from the Phase 4.95 review, three of them HIGH and all of them password- or session-credential leaks. All seven fixed and verified; see `proofs/phase-4.95-findings.md`.
3. **Two README correctness errors** from the Phase 4.9 audit: a blanket "read-only, does not mutate remote resources" claim contradicted by the `files` group's File Station task commands, and an empty template placeholder where the platform-default config path belonged. Both fixed, and the same read-only claim was corrected in the two places SKILL.md repeats it.
4. **One SKILL.md correctness error** from the Phase 4.8 review: the `storage smart` entry claimed the command reads the scheduled SMART test configuration, which is a separate command (`storage smart-schedule`). Corrected in SKILL.md, README.md, `research.json` and `.printing-press.json` so a regeneration cannot reintroduce it.
5. **One unregistered command** from the dogfood leg: `auth set-token`. DSM has no API token to save and the command was never registered, so it was dead code. Removed along with its `credentialSavePath` helper.

## Warnings accepted, not fixed

Two naming-convention warnings: `files info` and `system info` use the verb `info` where the convention wants `get`. Renaming them would make both commands read worse - `files get` suggests fetching a file rather than reading File Station's capability limits, and `system get` says less than `system info` - and would break the command names already published in README.md, SKILL.md and the MCP tool surface. The convention loses to legibility here. The dogfood leg still reports PASS with these warnings present.

## Gaps carried forward

- **Live API verification is impossible on this host.** No Go binary can reach the network through ProtonVPN's routing; see `proofs/phase-4.85-skip.md` for the reproducer. This blocks Phase 4.85 and Phase 5's live dogfood matrix. Every structural check - help, flags, exit codes, dry-run request shape, JSON envelopes - passes; no live response payload has ever been seen.
- **`govulncheck` never ran.** It hangs in this environment even on a trivial scratch module, so the dependency vulnerability gate is unverified. Documented in the build log.
- **37 framework test failures** in `internal/cli`, `internal/cliutil`, `internal/config`, `internal/learn`, `internal/mcp`, all environmental on Windows (HOME versus USERPROFILE path resolution, and ACL ownership the permission tests expect). None touches the Synology spec surface or the hand-authored DSM layer. `go build ./...` and `go vet ./...` are clean, and `go test ./internal/client/` passes including the new hand-authored redaction test.

## Verdict

**ship** - with the live-verification gap stated plainly. Everything that can be verified without reaching the NAS has been verified; nothing that could be verified was left unverified.
