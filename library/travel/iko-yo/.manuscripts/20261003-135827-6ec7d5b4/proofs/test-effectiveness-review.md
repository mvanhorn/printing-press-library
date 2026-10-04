# Iko-yo Trip test-effectiveness recheck

**PASS — same sole reviewer; narrow review of the two test changes. No actionable finding remains.** Earlier source/artifact snapshots are preserved.

`TestComparisonTablesPreserveFailedIdentities` now seeds one parsed minimized source record in an isolated local store and executes actual `RootCmd trip compare` in JSON, CSV and plain modes with a missing second reference. It checks real requested/successful counts, actual records/failure identities, command warnings and parsed table row counts/status/error/unknown decision fields. Its denominator assertions observe command output rather than a by-value fixture passed to a formatter.

`TestDiscoverRejectsUnobservedPrefecture` now distinguishes a missing prefecture ID from the requested ID observed under the wrong region. The latter case supplies a genuine parsed area-map entry and requires rejection by the canonical geography guard, so an ID-only acceptance path cannot satisfy the test.

Independent focused run passed: `go test ./internal/cli ./internal/trip -run 'TestComparisonTablesPreserveFailedIdentities|TestDiscoverRejectsUnobservedPrefecture' -count=1 -v`. Both geography subcases ran and passed. No full review or broad tests were repeated.

Direct SHA256 hashes for the two current test files are in test-effectiveness-snapshot.json. The five other focused-fix files, both preserved version-stamp source files and both installed peers match their prior hashes exactly. This test-only signoff does not replace the final new-head automated gates.
