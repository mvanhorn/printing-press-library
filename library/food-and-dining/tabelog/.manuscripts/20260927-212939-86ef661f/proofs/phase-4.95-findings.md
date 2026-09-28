# Local code review

Eleven findings were fixed in one repair pass and verified across two review rounds. Authoritative changes and focused evidence are in [cluster A](phase17-cluster-a-fixes.md), [cluster B](cluster-b-fixes.md), the product's `.printing-press-patches/phase17-cluster-*` records, and the run's `toolchain/proofs/phase17-generator-toolchain-proof.json`.

Review path: direct subagent dispatch, using gpt-6-sol with max reasoning for correctness, security, and maintainability/performance. All three reviewers reviewed the combined changes in round 2. The available thread limit required reusing existing agents; correctness was independent of product implementation, and the other reviewers emphasized modules they had not authored.

- [Correctness round 2](phase17-correctness-r2.md): PASS.
- [Security round 2](phase17-security-r2.md): PASS.
- [Maintainability/performance round 2](phase17-maintainability-r2.md): PASS.

Convergence: no actionable product findings remain after round 2. Native `boundCtx` entrypoints were scanned before dispatch; the two obsolete contextless checkpoint writes identified during review were removed. Normalized persistence errors remain visible.

## Template-origin records

The [security review](phase17-security-r1.md) retains exact affected lines, realistic triggers, and upstream emitter paths for MCP positional flag injection, unbounded generated nonbinary reads, and predictable delivery temporary files. The [maintainability review](phase17-maintainability-r1.md) retains the generated projection-warning/error interaction. These were repaired in the local generator templates as well as corresponding printed code, with fresh-generation tests and separate patches. They were not hidden with printed-only changes. No public issue or PR was opened.

The response-size repair covers every emitted Tabelog nonbinary path. Optional OAuth-specific branches are not emitted for this no-auth CLI and remain outside this repair's scope. Binary streaming remains separate.

One documented framework limitation remains: global flag-parser errors before Args/RunE can be plain text even under `--agent`. The exact generated root/template locations and reproduction are in the [correctness review](phase17-correctness-r1.md). Exit status remains nonzero, no source requests occur, and README/SKILL explicitly tell callers to handle this boundary. Domain validation, source failures, projection errors and partial-refresh failures retain their structured diagnostic contract. This template-origin interface limitation is excluded from product convergence; it is not an accuracy or data-preservation failure.

Reserved `internal/cliutil` and `internal/mcp/cobratree` were not patched. No additional out-of-scope finding was observed.

## Integrated verification

The [integrated summary](phase17-integrated-summary.json) records one complete `go test ./...` run after all fixes: 12 tested packages, 670 test pass events, zero failures; `go vet ./...` also passed. Pass-event counts include parent/subtest events and should not be represented as 670 independent end-to-end workflows. The E2E package recorded 92 pass events across 36 top-level tests. Focused regression artifacts retain failing pre-fix and passing post-fix evidence.

The fact-complete five-result fixture now measures 1,247 tokens / 3,897 bytes. The proposed token target was revised from 1,200 to 1,300 to retain payment/smoking facts that the earlier summary had incorrectly omitted. The ten-result ID/name/rating projection remains 403 tokens / 1,199 bytes. Live website acceptance and promotion remain separate subsequent gates.
