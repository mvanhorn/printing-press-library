# Open Food Facts Print Notes

- Keep every command read-only: no product edits, image uploads, login/session flows, account actions, or write-side contribution routes.
- Use documented JSON API endpoints, not HTML scraping.
- Prefer API v3 for product reads and API v2 for structured search.
- Keep requests bounded; do not turn this print into a bulk harvester or search-as-you-type backend.
- Preserve custom `User-Agent` guidance through `OPEN_FOOD_FACTS_USER_AGENT` and `OPEN_FOOD_FACTS_CONTACT_EMAIL`.
- Keep Open Food Facts' voluntary data-quality caveat visible in downstream output.
- Run `go test ./...` and `cli-printing-press publish validate --dir . --json` before publishing changes.
