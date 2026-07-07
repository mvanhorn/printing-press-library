# PokerDB Local Print Notes

- Keep this print local-only. Do not add Hendon Mob, PokerDB, or other network API calls.
- Do not add scraping, Cloudflare bypasses, or browser automation against pokerdb.thehendonmob.com.
- Use `POKERDB_FILE` only as a local CSV/JSON file path.
- Keep imported data user-supplied and portable; do not commit personal exports or cached poker data.
- Run `go test ./...`, `go vet ./...`, and `go build ./cmd/pokerdb-pp-cli` before publishing changes.
