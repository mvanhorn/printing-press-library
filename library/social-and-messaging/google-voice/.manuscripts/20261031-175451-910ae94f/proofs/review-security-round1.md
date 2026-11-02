# Security and data-integrity review — round 1

Target: `working/google-voice-pp-cli`; direct reviewer dispatch under Printing Press phase 17. Review performed on 2026-10-31. Two actionable custom-code findings remain. No source implementation changes were made.

All runtime checks used synthetic account IDs, message records and cookie values. No real credentials, messages or HAR files were opened; no remote requests, external messages or operational deletions occurred. The temporary probe and its private temporary fixtures were removed.

## S1 — P1: Private Voice writes do not enforce macOS ACL privacy

Locations:

- `internal/voice/session.go:127-137` (`SaveSession` creates/chmods the directory and delegates publication without verifying the newly created file's effective ACL).
- `internal/voice/mirror.go:68-85` (`OpenMirror` checks only regular-file and POSIX mode bits, then creates/opens SQLite).

Impact: A macOS extended ACL can grant non-owner access despite modes 0600/0700. Existing SQLite files with an everyone-read ACL are accepted and can receive additional private messages. A newly saved session inherits an everyone-read file ACL from its containing directory, and `SaveSession` reports success even though the project's own `ReadPrivate` subsequently rejects that exact session as non-private. In an otherwise traversable shared path, the inherited ACL exposes Google session cookies or message contents. Independently of external traversability, the successful import produces an unusable session and inaccurately reports success.

Synthetic reproduction on Darwin:

1. Create a synthetic mirror using `OpenMirror(tempPath, "synthetic-account", true)`, close it, and add `chmod +a 'everyone allow read' tempPath`.
2. Open that file with both `ReadPrivate` and `OpenMirror(..., false)`. The first refuses its foreign ACL, while the second accepts it.
3. Point `GOOGLE_VOICE_CONFIG_DIR` / `GOOGLE_VOICE_DATA_DIR` to private temporary paths; create the synthetic profile directory. Apply `chmod +a 'everyone allow read,file_inherit' profileDirectory`.
4. Call `SaveSession` with fake cookies, then `ReadPrivate(sessionPath)`. Save succeeds, resulting mode is 0600, and the read refuses its inherited ACL.

Observed probe output:

```text
acl: mode=0600 ReadPrivate_rejects=true OpenMirror_rejects=false
session_acl_inheritance: SaveSession_succeeded=true mode=0600 ReadPrivate_rejects=true
```

Minimal fix: Enforce owner/mode/effective-ACL checks for the SQLite file, including a newly created empty file before SQLite writes any message data. For session publication, ensure the temporary file is verified private before writing secret bytes, and reject unsafe inherited directory/file ACLs. Reuse the existing exported permission checks where appropriate, with no changes to generator-reserved `internal/cliutil`. Merely checking after secret bytes have been published leaves an exposure window; merely chmodding does not remove macOS ACLs.

Classification: Custom integration fix. `ReadPrivate` already uses the robust shared read verifier; the handwritten session/database writers omit equivalent enforcement. `cliutil.AtomicWritePrivateFile` explains the session trigger but is out of scope and should not be patched locally. A machine-wide hardening follow-up for that helper may be useful, but it does not replace fixing the Voice privacy boundary.

## S2 — P2: Concurrent sync runs break the arrivals watermark

Locations:

- `internal/voice/mirror.go:222-226` assigns each run ID before its remote page work and does not serialize syncs across database handles/processes.
- `internal/voice/mirror.go:157` records the run ID on first message insertion.
- `internal/voice/history.go:105` filters arrivals with `first_run > afterRun`.

Impact: Run IDs describe start order, while records become visible in commit order. If an older run is delayed while a later run completes, the caller can save the later terminal run as its arrivals watermark. A message committed afterward by the delayed older run has a lower `first_run`, so every subsequent `arrivals --after-run <later-run>` permanently omits it. The SQLite page transaction does not solve this ordering problem, and MCP hosts can invoke sync tools concurrently.

Deterministic synthetic reproduction:

1. Open the same synthetic database through two `Mirror` handles and insert one pending thread.
2. Start sync A with a fake `Remote`; let its account call finish and block its thread response after it has inserted run 1.
3. Run sync B against the second handle. Return a different fake message for the same thread and allow run 2 to finish.
4. Release A's thread response with its unique fake message, allowing run 1 to commit after run 2 is terminal.
5. The database has both messages; `Arrivals(ctx, 2, "", 100)` returns neither. In particular, the message committed after run 2 is absent from that incremental feed.

Observed probe output:

```text
concurrent: slow_run=1 fast_terminal_run=2 total_messages=2 arrivals_after_fast=0
```

Minimal fix: Hold a process-safe per-mirror synchronization lock around the complete sync run, starting before run-ID allocation and ending after the terminal-status update. Waiting for that lock must honor cancellation/timeouts. This also avoids stale competing checkpoint updates. Alternatively, redesign the arrivals watermark to use a committed sequence and account for all in-flight earlier runs; serialization is the smaller change for this implementation. Do not silently ignore older `running` rows left by a crashed process if the chosen watermark still depends on run ordering.

Classification: Custom Voice mirror/history implementation; not a generator template candidate.

## Boundaries and checks that passed

- The three-RPC map is used at both direct Remote and generated Gateway boundaries. Gateway requires POST, HTTPS, exact host and exact path. Remote constructs the fixed origin itself, filters imported header/cookie names and refuses redirects through its client.
- HTTP error handling does not include response bodies, cookies or authorization signatures. The adapter's normal output does not return session secrets.
- Profile names reject traversal. `privatePath` resolves existing symlink ancestors before enforcing the Dropbox component guard. Final credential symlinks are refused; regular private credential reads also reject foreign ACLs.
- Mirror SQL values are parameterized. Page inserts, source-ID validation, FTS updates and checkpoints share a transaction; a conflicting thread ID rolls the page back.
- The CLI's `GOOGLE_VOICE_NO_LOCAL_DATA=1` mirror-sync guard runs before session/database access and the synthetic regression test passes. Local query commands use read-only/query-only SQLite connections. Sync and HAR import are annotated as may-write; live cookie rotation is documented ancillary session maintenance rather than an additional finding here.
- Typed MCP API calls bind through the custom Gateway hook and disable the response cache. The reviewed custom live CLI paths bind their context before the first request.

Focused synthetic regression checks passed:

```text
go test ./internal/voice ./internal/cli -run 'TestPrivateImport|TestRemoteAndGateway|TestMirrorTransactionsAndQueries|TestVoiceNoLocalDataBlocksCollectionBeforeIO' -count=1
ok google-voice-pp-cli/internal/voice
ok google-voice-pp-cli/internal/cli
```

No independently established generator-template or `internal/mcp/cobratree` finding is filed by this reviewer. This report does not claim live authorization behavior, cross-platform Windows permission behavior, or exhaustive absence of other defects.
