# Cloudflare Dry-Run Smoke Proofs

Run date: 2026-06-08

Live Cloudflare credentials were available through 1Password for a scoped Workers API token. The token verified successfully and listed live zones. Token-management permission-group lookup and billing usage endpoints returned structured 403 responses because the token is scoped and does not carry those permissions. Mutating workflows were verified in `--dry-run --json` mode only.

Commands exercised:

- `go test ./...`
- `go build -o ./cloudflare-pp-cli ./cmd/cloudflare-pp-cli`
- `go build -o ./cloudflare-pp-mcp ./cmd/cloudflare-pp-mcp`
- `cloudflare-pp-cli token recipe pages-static --dry-run --json`
- `cloudflare-pp-cli token recipe agent-admin --json`
- `cloudflare-pp-cli token create --recipe agent-admin --dry-run --json`
- `cloudflare-pp-cli token doctor --recipe pages-static --account acct_123 --json`
- `cloudflare-pp-cli agent admin --dry-run --json`
- `cloudflare-pp-cli project launch ./internal --project my-site --account acct_123 --domain app.example.com --dry-run --json`
- `cloudflare-pp-cli project preview ./internal --project my-site --account acct_123 --branch pr-123 --dry-run --json`
- `cloudflare-pp-cli domain connect app.example.com --target pages --project my-site --account acct_123 --zone zone_123 --dry-run --json`
- `cloudflare-pp-cli worker secret put API_KEY --account acct_123 --script my-worker --value supersecret --dry-run --json`
- `cloudflare-pp-cli rag bootstrap --account acct_123 --name memory --dry-run --json`
- `cloudflare-pp-cli agent memory bootstrap --account acct_123 --name memory --dry-run --json`
- `CLOUDFLARE_API_TOKEN=<from 1Password> cloudflare-pp-cli auth doctor --json`
- `CLOUDFLARE_API_TOKEN=<from 1Password> cloudflare-pp-cli zones get --json --select id,name,status`

Secret smoke output redacted the value as `<redacted:11 chars>`.
