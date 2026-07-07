# Worten CLI

Local OpenAPI seed extracted from the working housebuy Worten CLI.
This does not claim to describe the full Worten platform.
It only captures the load-bearing endpoints currently used by the housebuy operator.

Created by [@asantos00](https://github.com/asantos00) (Alexandre Santos).

## Install

Install from the Printing Press library:

```bash
npx -y @mvanhorn/printing-press-library install worten --cli-only
```

Source build fallback:

```bash
git clone https://github.com/emmassist-co/worten-pp-cli.git
cd worten-pp-cli
go install ./cmd/worten-pp-cli
```

For `housebuy`, either keep `worten-pp-cli` on `PATH` or point the workflow wrapper at it directly:

```bash
export WORTEN_PP_BIN=/absolute/path/to/worten-pp-cli
```

To preserve `housebuy`'s existing repo-local cache and snapshot files while using this binary:

```bash
export WORTEN_PP_SNAPSHOT_DIR=/path/to/housebuy/data/worten-snapshots
export WORTEN_PP_ID_CACHE_PATH=/path/to/housebuy/evals/worten-id-cache.json
```

## Current Boundary

This binary is now the intended source of truth for Worten retailer behavior:

- `resolve`
- `product`
- `buyer`
- `stock`
- `specs`
- `suggest`
- `search`
- `snapshot`

The generated `worten-api` subtree remains available as the raw endpoint layer, but `housebuy` should consume the normalized commands above.

## Quick Start

### 1. Verify Setup

```bash
worten-pp-cli doctor
```

This checks the base configuration and connectivity assumptions.

### 2. Try the normalized commands

```bash
worten-pp-cli resolve 11111111-1111-1111-1111-111111111111
worten-pp-cli product 11111111-1111-1111-1111-111111111111
worten-pp-cli buyer 11111111-1111-1111-1111-111111111111
worten-pp-cli snapshot 11111111-1111-1111-1111-111111111111 --refresh
```

## Usage

Run `worten-pp-cli --help` for the full command reference and flag list.

## Commands

### normalized

- **`worten-pp-cli resolve`** - Resolve a Worten product URL or slug to a canonical product identifier.
- **`worten-pp-cli product`** - Fetch the normalized retailer view for a product.
- **`worten-pp-cli buyer`** - Fetch the normalized buyer view used by `housebuy`.
- **`worten-pp-cli stock`** - Fetch normalized stock, shipping, seller, and optional nearby-store context.
- **`worten-pp-cli specs`** - Fetch Worten technical specifications with a stable wrapper shape.
- **`worten-pp-cli suggest`** - Fetch normalized search suggestions.
- **`worten-pp-cli search`** - Search Worten products with explicit contexts.
- **`worten-pp-cli snapshot`** - Persist and return normalized snapshot payloads.

### worten-api

Manage worten api

- **`worten-pp-cli worten-api get-offer-stock`** - Fetch nearby store pickup stock for an offer.
- **`worten-pp-cli worten-api get-product-details`** - Fetch product details by Worten product identifier.
- **`worten-pp-cli worten-api get-search-suggestions`** - Fetch search suggestions for a text query.
- **`worten-pp-cli worten-api get-technical-specifications`** - Fetch technical specifications for a Worten product.
- **`worten-pp-cli worten-api search-products`** - Search Worten products by query and context.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
worten-pp-cli worten-api get-offer-stock --offer-id 550e8400-e29b-41d4-a716-446655440000 --search-query example-value --radius 42

# JSON for scripting and agents
worten-pp-cli worten-api get-offer-stock --offer-id 550e8400-e29b-41d4-a716-446655440000 --search-query example-value --radius 42 --json

# Filter to specific fields
worten-pp-cli worten-api get-offer-stock --offer-id 550e8400-e29b-41d4-a716-446655440000 --search-query example-value --radius 42 --json --select id,name,status

# Dry run — show the request without sending
worten-pp-cli worten-api get-offer-stock --offer-id 550e8400-e29b-41d4-a716-446655440000 --search-query example-value --radius 42 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
worten-pp-cli worten-api get-offer-stock --offer-id 550e8400-e29b-41d4-a716-446655440000 --search-query example-value --radius 42 --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
worten-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/worten-reverse-engineered-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
