# DriveThruRPG CLI

Search the public DriveThruRPG catalog and download purchased files from your authenticated library.

## Install

Install the CLI and its companion agent skill:

```bash
npx -y @mvanhorn/printing-press install drivethrurpg
```

CLI only:

```bash
npx -y @mvanhorn/printing-press install drivethrurpg --cli-only
```

Pre-publish local build:

```bash
go build -o bin/drivethrurpg-pp-cli ./cmd/drivethrurpg-pp-cli
```

## Authentication

Public catalog commands do not need credentials.

Library and download commands need a DriveThruRPG Library App Application Key with **My Library Access** enabled.

Preferred setup:

```bash
export DRIVETHRURPG_APPLICATION_KEY="<application-key>"
drivethrurpg-pp-cli auth login
drivethrurpg-pp-cli auth status
```

For ephemeral agents or MCP hosts that already have an exchanged token:

```bash
export DRIVETHRURPG_DTRPG_TOKEN="<exchanged-token>"
```

Do not print tokens to verify setup. Use `auth status` or `doctor --json`.

## Quick Start

```bash
drivethrurpg-pp-cli doctor
drivethrurpg-pp-cli search "Cyberpunk" --json --limit 5 --data-source live
drivethrurpg-pp-cli products search --keyword "Legend of the Five Rings" --page-size 5 --json
drivethrurpg-pp-cli library --page-size 10 --json
drivethrurpg-pp-cli download <libraryProductId> <fileIndex> --output-dir ~/Downloads
```

## Commands

### Catalog

| Command | Purpose |
| --- | --- |
| `search <query>` | Search public products through the live API or local synced data. |
| `products search --keyword <term>` | Search product cards with pagination, publisher, and sort filters. |
| `products get <productId>` | Fetch public product details. |
| `search-ahead --keyword <term>` | Search ahead across products, publishers, categories, and filters. |
| `publishers <publisherId>` | Fetch public publisher details. |
| `categories list` / `categories get-category <categoryId>` | Browse public category metadata. |
| `filters list` / `filters get <filterId>` | Browse public filter metadata. |
| `reviews --product-id <productId>` | List public reviews for a product. |
| `special-offers` | List active public special offers. |

### Library

| Command | Purpose |
| --- | --- |
| `auth login` | Exchange a Library App Application Key for a saved API token. |
| `auth status` | Show whether credentials are configured without printing secrets. |
| `library` | List products in your DriveThruRPG library, including file indexes for downloads. |
| `download <libraryProductId> [fileIndex]` | Prepare, poll, and save a purchased file. |
| `library prepare-download <libraryProductId> --index <n>` | Advanced: prepare a download URL only. |
| `library check-download <libraryProductId> --index <n>` | Advanced: poll a prepared download URL. |

### Local Data

| Command | Purpose |
| --- | --- |
| `sync` | Sync API data to local SQLite. |
| `analytics` | Summarize locally synced data. |
| `export <resource>` | Export local/API data to JSON or JSONL. |
| `workflow archive` | Archive resources to the local store. |
| `workflow status` | Show local archive state. |

### Utilities

| Command | Purpose |
| --- | --- |
| `which <query>` | Resolve a natural-language capability to a CLI command. |
| `agent-context --pretty` | Emit machine-readable command, auth, and discovery metadata. |
| `profile` | Save and reuse named flag sets. |
| `feedback` | Record local feedback about the CLI. |
| `completion <shell>` | Generate shell completions. |

## Output Formats

```bash
# Human-readable table in a terminal
drivethrurpg-pp-cli products search --keyword "Mothership"

# JSON for scripts and agents
drivethrurpg-pp-cli products search --keyword "Mothership" --json

# Keep only selected fields
drivethrurpg-pp-cli library --json --select results.data.attributes.name,results.data.attributes.files

# Show the request without sending it
drivethrurpg-pp-cli products search --keyword "Mothership" --dry-run

# Agent defaults: JSON, compact output, no prompts, no color, yes to confirmations
drivethrurpg-pp-cli search "Cyberpunk" --agent --limit 5 --data-source live
```

## Cookbook

```bash
# Find a core rulebook in the public store
drivethrurpg-pp-cli search "Legend of the Five Rings core rulebook" --json --limit 5 --data-source live

# Store search with product filters
drivethrurpg-pp-cli products search --keyword "Cyberpunk" --page-size 5 --json

# Inspect one product
drivethrurpg-pp-cli products get 257004 --json

# Search-ahead for autocomplete-style matches
drivethrurpg-pp-cli search-ahead --keyword "Mothership" --page-size 5 --json

# List the first page of your library
drivethrurpg-pp-cli library --page-size 10 --json

# Download the first file from a product in your library
drivethrurpg-pp-cli download <libraryProductId> 0 --output-dir ~/Downloads

# Prepare a URL without saving the file
drivethrurpg-pp-cli download <libraryProductId> 0 --url-only --json

# List public reviews for a product
drivethrurpg-pp-cli reviews --product-id 257004 --json

# Find current sale campaigns
drivethrurpg-pp-cli special-offers --page-size 10 --json

# Let an agent discover the right command
drivethrurpg-pp-cli which "download a purchased file" --json
```

## Agent Usage

This CLI is designed for AI agent consumption:

- `--agent` expands to `--json --compact --no-input --no-color --yes`.
- `--json` writes structured data to stdout and errors to stderr.
- `--select` trims large responses for context efficiency.
- `--dry-run` previews requests and masks secret-like parameters.
- `agent-context --pretty` exposes command metadata, auth options, and recommended workflows.
- Public catalog commands work without auth; library and download commands require credentials.

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## MCP Usage

Install the MCP binary from the published public-library entry or a release bundle, then configure one of these environment variables:

```bash
DRIVETHRURPG_APPLICATION_KEY=<application-key>
# or
DRIVETHRURPG_DTRPG_TOKEN=<exchanged-token>
```

Application keys are preferred because they match the official DriveThruRPG Library App flow. Tokens are useful for hosts that cannot run `auth login`.

Stdio remains the default transport:

```bash
drivethrurpg-pp-mcp
```

For remote-capable MCP hosts, run streamable HTTP:

```bash
drivethrurpg-pp-mcp --transport http --listen 127.0.0.1:8080
```

The HTTP endpoint defaults to `/mcp`; override it with `--endpoint /mcp` or `DRIVETHRURPG_MCP_ENDPOINT`.

In addition to typed endpoint tools, the MCP server exposes user-job tools for common goals:

| Tool | Purpose |
| --- | --- |
| `job_catalog_search` | Search public products and return compact product cards. |
| `job_catalog_price_scan` | Search public products and sort compact results by price. |
| `job_product_brief` | Fetch one public product as a concise brief. |
| `job_library_list` | List authenticated library purchases as compact rows. |
| `job_library_find` | Search owned purchases by title, id, or filename. |
| `job_file_manifest` | Show downloadable file indexes for an owned product. |
| `job_prepare_file_download` | Prepare a download URL for a purchased file. |
| `job_cache_status` | Inspect local cache counts and sync freshness. |

## Health Check

```bash
drivethrurpg-pp-cli doctor
drivethrurpg-pp-cli doctor --json
drivethrurpg-pp-cli doctor --fail-on error
```

`doctor` checks config loading, auth state, API reachability, and cache state without printing credential values.

## Configuration

Config file: `~/.config/drivethrurpg-pp-cli/config.toml`

Environment variables:

| Name | Required | Description |
| --- | --- | --- |
| `DRIVETHRURPG_APPLICATION_KEY` | No | Preferred Library App Application Key. Used for token exchange on authenticated commands. |
| `DRIVETHRURPG_DTRPG_TOKEN` | No | Already-exchanged API token sent as the `Authorization` header. |
| `DRIVETHRURPG_BASE_URL` | No | Override the API base URL for tests or mocks. |
| `DRIVETHRURPG_CONFIG` | No | Override the config file path. |

At least one of `DRIVETHRURPG_APPLICATION_KEY`, `DRIVETHRURPG_DTRPG_TOKEN`, or a saved config token is required for `library` and `download`.

## Troubleshooting

**Authentication errors (exit code 4)**

- Confirm the Application Key has **My Library Access** enabled.
- Run `drivethrurpg-pp-cli auth login --no-save --json` to validate the key without saving tokens.
- Run `drivethrurpg-pp-cli doctor --json` to inspect auth state without printing secrets.

**Download times out**

- Run `drivethrurpg-pp-cli library prepare-download <libraryProductId> --index <n> --json --no-cache` to inspect the raw prepare status.
- Increase `--max-wait` if DriveThruRPG is still watermarking the file.

**No local search results**

- Run `drivethrurpg-pp-cli sync --latest-only --json` first, or force live search with `--data-source live`.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

## Unique Features

These capabilities aren't available in any other tool for this API.

### Catalog discovery
- **`search`** — Search DriveThruRPG's public product catalog by keyword while keeping JSON output agent-friendly.

  _Agents can answer store-discovery questions before asking the user for credentials._

  ```bash
  drivethrurpg-pp-cli search "Cyberpunk" --agent --limit 5 --data-source live
  ```

### Authenticated library
- **`library`** — List products in your DriveThruRPG library, including file indexes for downloads.

  _Agents can inspect owned purchases and choose the right file index without scraping website pages._

  ```bash
  drivethrurpg-pp-cli library --agent --page-size 10
  ```
- **`download`** — Prepare, poll, and save a purchased DriveThruRPG file from a single command.

  _Agents can execute the full authenticated file retrieval workflow without hand-rolling polling logic._

  ```bash
  drivethrurpg-pp-cli download LIBRARY_PRODUCT_ID --dry-run --agent
  ```
