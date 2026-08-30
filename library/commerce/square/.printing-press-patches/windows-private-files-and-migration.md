# Preserve Windows private-file and migration behavior

## Reprint guard

Preserve these generated-tree changes unless the upstream Printing Press templates provide equivalent behavior:

- Serialize `AtomicWritePrivateFile` operations so concurrent Windows rename operations do not intermittently fail with `Access is denied`.
- On Windows, secure config, fingerprint, database, and backup files with an owner-only protected DACL; on Unix, retain mode `0600`.
- Test private-file protection through the platform-specific security check instead of asserting Unix mode bits on Windows.
- Format Windows SQLite read-only migration URIs as `file:///C:/...`; `file:C:/...` is parsed incorrectly by modernc SQLite.
- Keep the migration source-verification errors contextual enough to identify whether opening, pinging, or reading metadata failed.

## Verification

Run `go test -count=1 ./internal/platform ./internal/cliutil ./internal/cli` on Windows and `go test -count=1 ./...` before release.
