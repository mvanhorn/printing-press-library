# Agent notes — amazon-pp-cli

Brief contract notes for agents (and reviewers) working on this CLI.

## Architecture
Hand-built, no generator. Mirrors `library/commerce/instacart/` structure:
- `cmd/amazon-pp-cli/` — cobra root, single binary entrypoint
- `cmd/amazon-pp-mcp/` — stdio MCP shim (stub for v0.1; nanoclaw integrates via the CLI binary directly)
- `internal/config/` — multi-profile state (~/.config/amazon-pp-cli/config.json + profiles/<name>/)
- `internal/auth/` — Chrome cookie import (kooky), paste, file import
- `internal/store/` — SQLite per-profile DB with FTS5 over purchased_items.title
- `internal/amazon/` — replayable HTTP client against amazon.com (cart view, add, checkout)
- `internal/history/` — JSONL importer from `docs/dumper.js`
- `internal/cli/` — cobra commands

## Invariants worth preserving

1. **Repurchase-only.** `add` MUST exit 4 on no-history-match. There is no live search fallback.
2. **Confirmation gate.** `checkout` MUST exit 10 without `--yes`. The gate runs BEFORE session loading so users see the clear refusal even with stale cookies.
3. **Per-profile isolation.** Every profile has its own SQLite file. There is no cross-profile read path.
4. **Most-frequent tiebreak.** `add` resolves via SQL `ORDER BY purchase_count DESC, last_purchased_at DESC`; most-frequent wins (not most-recent).

## Don't add

- A search-Amazon-for-new-items command. The safety rail depends on its absence.
- A returns / cancellation surface. Same rationale — mutating existing orders should require manual confirmation on amazon.com.
- A persistent browser sidecar. The whole point is direct HTTP replay.

## Pre-commit checks
```bash
go vet ./... && gofmt -l . && go build ./... && go test ./...
```
All four must pass.
