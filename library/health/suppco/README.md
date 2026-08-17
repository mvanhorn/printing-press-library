# SuppCo CLI

**Read-only SuppCo stack facts with a deterministic, provenance-bearing regimen snapshot export.**

Read minimum SuppCo stack and schedule facts, preserve overlapping nutrient relationships, and emit a deterministic snapshot for a later importer. Periodic bearer-token replacement is the intentionally simple authentication model.

## Install

After the package is accepted into the PrintingPress library:

```bash
npx -y @mvanhorn/printing-press-library install suppco
```

For a local checkout:

```bash
go build -o ./bin/suppco-pp-cli ./cmd/suppco-pp-cli
go build -o ./bin/suppco-pp-mcp ./cmd/suppco-pp-mcp
```

## Authentication

Provide a current bearer token through SUPPCO_ACCESS_TOKEN or pipe it to auth set-token. If it expires, replace it; automatic refresh and browser automation are not part of this package.

## Commands

All provider output is JSON.

```bash
suppco-pp-cli stack products
suppco-pp-cli stack nutrients
suppco-pp-cli schedule 2026-01-15
suppco-pp-cli regimen snapshot 2026-01-15
```

- `stack products` projects only provider product ID, label, and optional serving-size text.
- `stack nutrients` flattens each product's ingredient rows, derives immediate `parent_id` links from provider ancestry, and adds sorted `component_ids`. Parent and child amounts overlap and must not be summed naively.
- `schedule <date>` returns minimized configured activities, scheduled products, and reminder state for one `YYYY-MM-DD` date.
- `regimen snapshot <date>` makes one stack read and one schedule read and emits a normalized snapshot.

The snapshot keeps `provider_schedule` separate, leaves `user_override` absent, sets `effective_source` to `provider_schedule`, and copies that schedule to `effective_regimen` without adherence or cadence inference. Trainer Core may later apply manual truth as a separate downstream concern.

## Provenance and freshness

Standalone reads return:

```json
{
  "data": [],
  "provenance": {
    "provider": "suppco",
    "path": "/api/users/me_compact/",
    "observed_at": "2026-01-15T12:00:00Z"
  }
}
```

Snapshots include separate stack and schedule observations. Their top-level `as_of` is the local completion time after both non-transactional reads; it is not a claim that both provider surfaces were observed atomically.

## Privacy and safety

- Provider requests are GET-only and pinned to `https://api.supp.co`.
- Cross-origin redirects and disabled TLS verification are refused.
- Provider reads bypass the generated HTTP cache and local database.
- `sync`, `search`, export, webhook delivery, profiles, feedback, and mutation commands are not exposed.
- Raw `/me_compact` responses are never returned. Unrelated profile fields and the aggregate top-level nutrient view are discarded in memory; product-bound components come from `products[].ingredients`.
- The package does not log credential values or accept them on the command line.

Redirect stdout explicitly if you choose to retain an output. Treat any resulting file as private health-adjacent data.

## MCP server

The MCP server is stdio-only and exposes exactly five read-only tools:

- `stack_products`
- `stack_nutrients`
- `schedule_show`
- `regimen_snapshot`
- `context`

Set `SUPPCO_ACCESS_TOKEN` in the MCP host environment. The MCP server uses the same provider service and normalization rules as the CLI.

## Development

```bash
go test ./...
go vet ./...
go build ./...
```

Fixtures under `internal/provider/testdata` are synthetic. Do not add captured account responses, tokens, cookies, screenshots, or personal supplement values.

## License

Apache-2.0. See [LICENSE](LICENSE).

## Quick Start

```bash
# Project the minimum current product identity fields.
suppco-pp-cli stack products

# Inspect parent and component relationships without summing overlap.
suppco-pp-cli stack nutrients

# Build a dated normalized import candidate with provenance.
suppco-pp-cli regimen snapshot 2026-01-15

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Private provider normalization
- **`regimen snapshot`** — Combines the current stack and one dated provider schedule into deterministic JSON while preserving nutrient relationships and separate observation times.

  _Use this when an agent needs a bounded import candidate rather than raw SuppCo account payloads._

  ```bash
  suppco-pp-cli regimen snapshot 2026-01-15
  ```

## Recipes

### List products

```bash
suppco-pp-cli stack products
```

Returns the minimum product projection and observation provenance.

### Inspect nutrient hierarchy

```bash
suppco-pp-cli stack nutrients
```

Preserves parent and child rows so downstream code can avoid naive totals.

### Build a snapshot

```bash
suppco-pp-cli regimen snapshot 2026-01-15
```

Combines two bounded reads without introducing manual regimen truth.
