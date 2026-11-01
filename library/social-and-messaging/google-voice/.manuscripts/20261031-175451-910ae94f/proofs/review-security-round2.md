# Security and data-integrity review — round 2

Target: `working/google-voice-pp-cli`; full original security scope re-reviewed after the merged round-1 fixes. One residual custom-code finding remains. No implementation files were changed by this reviewer.

The review covered session/HAR handling, remote signing and fixed-host three-RPC allowlisting, redirects, profile paths and privacy, account isolation, transactional synchronization and arrivals, CLI hooks and local-write/NO_LOCAL_DATA boundaries, typed MCP handlers and the custom gateway. Generated `internal/cliutil` and `internal/mcp/cobratree` were read only where needed to establish behavior and remain excluded from local fixes.

## S3 — P2: SQLite opens a different pathname target after privacy validation

Location: `internal/voice/mirror.go:98-122`, especially the gap between privacy verification/canonicalization and the lazy SQLite connection's first access inside the schema-lock callback. The containing directory is chmodded at lines 85-89 but its effective write/delete ACL is not validated.

Impact: `OpenMirrorContext` verifies a private regular file and resolves its pathname, then can wait indefinitely for the schema lock. SQLite opens the pathname only afterward. Replacing that previously checked pathname with a symlink during the wait bypasses both the regular-file/symlink guard and the 0600/effective-ACL guard. The call succeeds and initializes a mode-0644 target. A subsequent sync can therefore write private records to a file that would have been refused had the actual opened target been checked. A non-owner actor can perform such replacement when the containing directory or an unpinned ancestor grants foreign write/delete access through a macOS ACL; chmod0700 does not remove that ACL.

Reproduction: [sanitized standalone probe](review-security-round2-probe.go.txt). The probe uses only fresh temporary files, a synthetic account ID, and no HTTP. It grants the temporary parent `everyone allow add_file,delete_child,search`, creates an empty mode0600 database and an empty mode0644 target, holds the database's `.schema` lock, starts `OpenMirrorContext`, then replaces the validated database path with a symlink before releasing the lock.

Observed output:

```text
symlink_swap: OpenMirror_accepted=true error=<nil>
target: mode=0644 bytes_written=90112
parent_foreign_ACL_detected_by_existing_helper=true
```

Minimal fix: Establish a trusted containing-directory/path boundary before relying on filename verification, including ownership, POSIX write/delete access, and foreign effective ACLs. Keep that boundary stable across lock waits and the SQLite open; use a no-follow/opened-object check if supported rather than relying solely on a pre-wait pathname check. Validate mutable ancestors or pin the trusted parent so replacing an ancestor cannot redirect the open. Rechecking the filename after the lock removes this deterministic long gap but does not by itself close an active replacement race in a foreign-writable directory.

Existing helpers: exported `cliutil.CheckDarwinPrivateFileACL(directory, "Voice directory")` works against a directory and detects the probe ACL; its file-scope policy is stricter than the internal directory policy and is suitable for a directory promised to be owner-only. `cliutil.VerifyCredsPerms` requires regular files, so it cannot be reused for directory ownership/mode validation. The shared directory verifiers (`verifyUnixCredentialDir`, `verifyPinnedDataDir`) are private; do not modify generator-reserved cliutil. A small Voice-specific directory trust verifier is a custom integration fix. Account for root-owned sticky system temporary directories when checking ancestors; a blanket ban on any writable ancestor would reject normal private temporary directories.

Classification: custom Voice path/privacy integration, not an independent generator-template finding.

## Prior findings and full-scope checks

- S1's direct ACL failures are addressed: SQLite now calls the shared private-file verifier before SQL access, and `SaveSession` verifies its empty temporary file before writing credentials. The new Darwin regression test passes. S3 is a remaining pathname/containing-directory race around the SQLite check, not a recurrence of the direct file-ACL omission.
- S2's ordinary concurrent-sync failure is addressed: the whole sync run is serialized under a cancellable cross-process lock, with terminal-status update inside the lock. Interrupted runs are marked terminal before starting the next run. The lock-cancellation and singleton-account tests pass.
- The singleton metadata index and post-insert reread prevent concurrent initialization from accepting two different mirror accounts.
- `List`/`Get` now verify the pinned account through the Remote before returning data. The mismatch test confirms no data endpoint is called for a different account.
- Exact fixed-host/path/method validation and redirect refusal remain intact. Credential filtering, bounded response reads, sanitized errors and response-cache bypass remain intact.
- Page writes, conflict detection, message/FTS updates and checkpoint commits remain transactional. No SQL interpolation from query values was found.
- CLI live-request paths retain bound contexts. Sync/import are may-write; local queries remain read-only. `GOOGLE_VOICE_NO_LOCAL_DATA=1` still blocks collection before credential/database IO. Typed MCP handlers still reach the same fixed-host Gateway hook.

Focused synthetic regression run passed:

```text
go test ./internal/voice ./internal/cli -run 'TestDarwinPrivateWrites|TestSyncLockCancellation|TestGatewayIdentity|TestVoiceNoLocalData|TestPrivateImport|TestRemoteAndGateway|TestMirrorTransactionsAndQueries' -count=1
ok google-voice-pp-cli/internal/voice
ok google-voice-pp-cli/internal/cli
```

The runnable probe copy was removed from the working source tree; its sanitized source is retained alongside this report at the parent's request. The probe cleaned only its own temporary fixtures. No real credentials/messages/HAR files were read, and no remote calls, external messages or operational deletions occurred. No additional concrete security finding or independent generator retro candidate was established in this round.
