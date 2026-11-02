# Security and privacy review — round 3

Result: **clear** for this reviewer's supported personal-Mac scope. No concrete remaining privacy, security or data-integrity finding was established. Findings S1, S2 and S3 from this reviewer's earlier rounds are addressed.

The final defensive review inspected the full original source scope, with particular attention to `internal/voice/privacy_unix.go`, `privacy_darwin_test.go`, `session.go`, `mirror.go`, `sync.go`, `remote.go`, and the custom CLI/MCP integration. No implementation files were changed, no exploit development or live probing was performed, and no real credential, message or HAR file was opened.

Verified privacy protections:

- The private leaf directory must belong to the current user with no group/world mode access. Canonical ancestors must have trusted ownership and no foreign POSIX write access, with a root-owned sticky-directory exception for normal temporary paths. Effective Darwin ACL checks apply to the leaf and ancestors; foreign write/delete ACLs are refused.
- `SaveSession` validates that directory boundary and verifies the empty temporary file's owner/mode/effective ACL before writing credential bytes, then syncs and atomically renames the file.
- SQLite verifies the file's private owner/mode/effective ACL, validates the private directory boundary, and rechecks both the directory and regular-file identity after the schema-lock wait. It also compares identity before reading the account binding. The previous foreign-directory ACL scenario is rejected.
- Final-file symlinks are refused, profile names reject traversal, and resolved private paths retain the Dropbox guard. Session secrets remain excluded from ordinary command output and HTTP error bodies.

The remaining original security boundaries remain intact: exact fixed-host three-read RPC allowlisting, redirect refusal, allowlisted credential/header forwarding, pinned-account verification before thread reads, singleton mirror ownership, transactional page/checkpoint writes, cancellable whole-sync serialization, read-only/query-only local SQLite reads, MCP response-cache bypass, and the `GOOGLE_VOICE_NO_LOCAL_DATA=1` collection guard before private session/database IO.

The new fixture hook only supplies existing `--home` and `--no-learn` arguments through runtime test annotations when explicitly configured with an absolute, whitespace-free fixture path. It adds no authentication bypass and embeds no private credential or message values. Windows private storage deliberately fails closed; this review does not certify Windows support or live Google authorization behavior.

Existing synthetic regression checks passed:

```text
go test ./internal/voice ./internal/cli -run 'TestDarwinPrivateWrites|TestMirrorRefusesForeignWritableDirectoryACL|TestPrivateImport|TestMirrorTransactionsAndQueries|TestSyncLockCancellationAndAccountSingleton|TestGatewayIdentityUnknownReadAndRateStatus|TestGatewayRejectsInvalidCountBeforeCredentials|TestVoiceNoLocalDataBlocksCollectionBeforeIO' -count=1
ok google-voice-pp-cli/internal/voice
ok google-voice-pp-cli/internal/cli
```

No new generator-template or generator-reserved-package retro candidate is filed by this reviewer. Review converged at round 3 for this reviewer; the parent must merge all reviewers' results before recording phase-level convergence.
