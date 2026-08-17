# Open Library Print Notes

- Keep every command read-only: no borrowing, patron/account actions, list mutation, or catalog edits.
- Use Open Library JSON endpoints, not HTML scraping.
- Keep requests bounded with `--limit`; do not turn this print into a bulk harvester.
- Preserve the optional contact-header setup through `OPEN_LIBRARY_USER_AGENT` and `OPEN_LIBRARY_CONTACT_EMAIL`.
- Keep the Subjects API caveat visible because Open Library documents it as experimental.
- Run `go test ./...` and `cli-printing-press publish validate --dir . --json` before publishing changes.
