# Acceptance Report: hostex (reprint, press 4.33.0, spec 3.15.0)

Level: Full Dogfood, read-only. Mutating commands ran dry-run only (no --allow-destructive); no write endpoint was called on the live account.
Tests: 278/278 passed (435 skipped: mutating commands in dry-run-only mode, interactive and unverifiable rows).
Gate: PASS (phase5-acceptance.json written by the runner).

## Live checks beyond the matrix (read-only)
- Sync on a clean database: 23/23 resources, 222 records, 0 warnings.
- 3.15.0 surface: `transactions query --reservation-code` and the deprecated `--stay-code` alias both accepted by the live API; `reservations create --channel-id` appears in the request body (dry-run).
- `reservations query` with one check-in bound returns the documented HTTP 400; both bounds required.
- `which "update prices"` ranks the three listing write commands first; `which "check listing price drift"` returns price-parity and no write command.
- MCP server starts and lists 37 tools (Cloudflare-style orchestration: hostex_search / hostex_get / hostex_execute plus the novel commands), down from 86 endpoint tools.

## Fixes applied during dogfood
- Novel commands and jobs list/prune returned prose or a bare array under --dry-run; they now emit the dry_run JSON envelope.
- Added `pp:happy-args` for transactions query, pricing-ratios (needs a property id) and stay-brief so the matrix reaches a real happy path.

## Printing Press issues (for retro)
- jobs list/prune templates ship without the dry_run JSON envelope the 4.33.0 dogfood requires.
- OpenAPI-sourced CLIs cannot declare the cache-freshness block, capping Cache Freshness at 5/10.
- Generator-emitted helper handleBinaryResponseDelivery is dead for JSON-only specs and trips dogfood.
- regen-merge cannot synthesize its base with an older press under a newer Go toolchain (enetx/http2 build error), forcing a manual --base.
- validate-narrative lists the dry-run price-push recipe as an MCP intent tool.
