# Verification Proofs

Structural validation performed on 2026-06-23:

- `go test ./...` from `library/ai/context-dev`: passed.
- `go vet ./...` from `library/ai/context-dev`: passed.
- `go build ./...` from `library/ai/context-dev`: passed.
- `cli-printing-press verify-skill --dir library/ai/context-dev --json`: passed with no findings.
- `cli-printing-press shipcheck --dir library/ai/context-dev --json`: passed.

Shipcheck legs:

- `verify`: passed.
- `validate-narrative`: passed.
- `dogfood`: passed.
- `workflow-verify`: passed.
- `apify-audit`: passed.
- `verify-skill`: passed.
- `scorecard`: passed.

Help-surface proof:

- `context-dev-pp-cli --help` lists generated `brand` and `web` command groups plus first-class `doctor-discover`, `clinic-enrich`, `scrape`, `crawl`, `extract`, `styleguide`, and `screenshot`.

Live-credit dogfood status:

- Not run in this workspace because `CONTEXT_DEV_API_KEY`, `CONTEXT_API_KEY`, and `CONTEXT_DEV_BEARER_AUTH` were all absent, and `context-dev-pp-cli auth status --json` reported no stored credentials.
- `context-dev-pp-cli doctor --json` still confirmed the API base is reachable and reported missing `CONTEXT_DEV_API_KEY` without exposing any secret.
