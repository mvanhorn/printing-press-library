# amazon-pp-cli Build Log

## What got built

Following the instacart-pp-cli pattern in `~/projects/printing-press-library/library/commerce/instacart/`.

### Packages

| Package | Files | Lines | Tests |
|---|---|---|---|
| `internal/config` | config.go | 215 | implicit via CLI dogfood |
| `internal/store` | store.go, store_test.go | 310 + 110 | 5 tests, all passing |
| `internal/auth` | auth.go, jar.go, auth_test.go | 270 + 11 + 70 | 5 tests, all passing |
| `internal/amazon` | client.go, parse.go, parse_test.go | 250 + 130 + 85 | 6 tests, all passing |
| `internal/history` | import.go, import_test.go | 175 + 75 | 3 tests, all passing |
| `internal/cli` | root.go, profiles.go, auth.go, doctor.go, history.go, cart.go, add.go, reorder.go, checkout.go, helpers.go | ~1100 | dogfooded via binary |
| `cmd/amazon-pp-cli` | main.go | 15 | binary entrypoint |
| `cmd/amazon-pp-mcp` | main.go | 25 | stub for v0.1 |

### Commands (in shipped order from `--help`)

```
amazon-pp-cli
├── add <query>            history-first repurchase (refuses unknown SKUs)
├── auth
│   ├── login              kooky Chrome cookie import
│   ├── paste              Cookie header paste (always works)
│   ├── import-file <path> Chrome Cookie-Editor JSON import
│   ├── status             session diagnostic
│   └── logout             delete cookies.json
├── cart
│   └── show               GET /gp/cart/view.html + parse line items
├── checkout               POST /gp/buy/spc/handlers/display.html (--yes gated)
├── doctor                 auth + DB + amazon.com reachability check
├── history
│   ├── import <path>      JSONL → SQLite (orders + items + FTS rebuild)
│   ├── list               most-purchased rollup
│   ├── search <query>     FTS5 search
│   └── stats              row counts + last-purchased timestamp
├── profiles
│   ├── list               named profiles + which is active
│   ├── add <name>         register a new profile
│   ├── use <name>         set the default active profile
│   └── paths              show on-disk paths for the active profile
└── reorder-last           re-add every line from the most recent order
```

Every command supports `--json`, `--profile <name>`, and (where it mutates state) `--dry-run`.

### Exit codes
- 0 OK
- 2 usage
- 3 auth (no session or expired)
- 4 not found (no match in history, missing profile)
- 5 conflict (duplicate profile)
- 7 transient (5xx / network)
- 10 confirmation required (`checkout` without `--yes`)

## Intentionally deferred

- Stdio MCP server (`amazon-pp-mcp`) is a stub. Nanoclaw integrates by shelling out to the CLI binary and consuming `--json` output, which mirrors how the rest of nanoclaw's tools work.
- New-item search/discovery. Repurchase-only is a design feature, not a missing one.
- `history sync` (live) — Amazon has no machine-readable order history endpoint; the JSONL dumper at `docs/dumper.js` is the supported backfill path.

## Generator limitations encountered

- `printing-press generate` requires a spec source (OpenAPI, internal YAML, HAR). Amazon has none of these usable for buyer-side operations. The instacart pattern (hand-built, no generator emit) was followed instead — exactly as the user's pre-answered design decision specified.

## Build / test results

```bash
go build ./...      # PASS
go test ./...       # PASS (19 tests total across 4 packages)
go vet ./...        # PASS, no findings
gofmt -l .          # PASS, no diffs
```

Binary size: ~16.5 MB (modernc.org/sqlite is most of it; pure-Go SQLite is worth it for cross-compile + Cgo-free deploys).
