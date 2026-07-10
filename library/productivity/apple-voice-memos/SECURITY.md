# Security Policy

## Supported versions

Only the latest release is supported.

## Reporting a vulnerability

Please report security issues privately through GitHub Security Advisories for this repository. Do not open a public issue containing private Voice Memos metadata, recording titles, transcripts, filesystem paths, crash dumps, or database files.

A useful report includes:

- CLI version and macOS version
- the command that failed, with personal values replaced
- minimal reproduction steps using synthetic data where possible
- expected and observed behavior

Never attach `CloudRecordings.db` or a real `.m4a` recording to a public report.

## Security model

The CLI:

- opens Apple’s Voice Memos SQLite database in read-only and query-only mode
- makes no application-level network requests
- does not modify or delete recordings
- writes only when `export` copies a selected recording to a user-provided directory
- may ask macOS to run `voicememod`
- may launch Voice Memos hidden during an explicit freshness operation, then terminates only the process it launched

Apple’s iCloud synchronization is performed by macOS, outside this program.
