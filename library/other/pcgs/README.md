# PCGS CLI

**The first CLI for the PCGS Public API — cert lookup, full CoinFacts extraction, and the 1,000-call-per-day budget enforced for you.**

PCGS gives you 1,000 API calls per day per token. This CLI tracks every call locally, forecasts batch cost before you spend a single one, syncs only mutable market fields on refresh, and ingests CSV / JSON / JSONL / plain-text cert lists straight into a local SQLite cache. No community wrapper exists. This is the only PCGS CLI.

Printed by [@vinnyp](https://github.com/vinnyp) (Vinny Pasceri).

## Install

The recommended path installs both the `pcgs-pp-cli` binary and the `pp-pcgs` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install pcgs
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install pcgs --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install pcgs --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install pcgs --agent claude-code
npx -y @mvanhorn/printing-press-library install pcgs --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/pcgs/cmd/pcgs-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/pcgs-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-pcgs --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-pcgs --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-pcgs skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-pcgs. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/pcgs-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `PCGS_AUTH_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "pcgs": {
      "command": "pcgs-pp-mcp",
      "env": {
        "PCGS_AUTH_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Set `PCGS_AUTH_TOKEN` to the bearer token generated at https://www.pcgs.com/publicapi. Run `pcgs-pp-cli doctor` to confirm reachability. All commands authenticate identically — there is one auth mode.

## Quick Start

```bash
# Confirm the token is set and the API is reachable before spending quota.
pcgs-pp-cli doctor --json

# See today's used / remaining / reset before any batch.
pcgs-pp-cli --quota --json

# Single-cert verification + full CoinFacts extraction (IsValidRequest = true iff the cert is legit).
pcgs-pp-cli coin facts-cert 53972744 --json

# Forecast batch cost against remaining quota with zero live calls.
pcgs-pp-cli coin batch --file examples-pcgs-coin-list.csv --dry-run --json

# Real batch ingest, resumable across UTC days if the file is larger than the daily budget.
pcgs-pp-cli coin batch --file examples-pcgs-coin-list.csv --resumable --checkpoint pcgs.ckpt --json

# Refresh only Population / PriceGuide / Auction / Images / CoinFactsNotes — never overwrite cert identity.
pcgs-pp-cli refresh --all --older 7d --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Quota-aware orchestration
- **`coin batch`** — Parse a CSV / JSON wrapper / JSONL / plain-text cert list and look up every cert against PCGS. --dry-run forecasts cost (live calls, cache hits, %-of-quota) without spending a single call. --resumable + --checkpoint split a list larger than the 1,000/day cap across UTC days. --list-certs emits the parsed cert list to stdout without calling. Non-cert input columns round-trip to output as `_keep.<col>` for downstream re-keying.

  _Use this any time you have more than one cert to look up. The dry-run mode answers 'does this fit today's quota?' for free; --resumable makes multi-day batches idempotent._

  ```bash
  pcgs-pp-cli coin batch --file ./pcgs-coin-list.csv --dry-run --json
  ```
- **`refresh`** — Refresh cached coins by updating only the fields PCGS can actually change — Population, PopHigher, PriceGuideValue, AuctionList, Images, CoinFactsNotes — while leaving cert identity fields untouched. Emits a per-field diff. --dry-run --older 30d --field price-guide lists which cached coins need refresh without calling the API.

  _This is the safe way to keep market data current without putting cert identity at risk. Pair --dry-run --older with --field to find what to refresh before spending quota._

  ```bash
  pcgs-pp-cli refresh --all --older 7d --json
  ```

### PCGS-specific content patterns
- **`coin pop-curve`** — Pull every grade 1–70 (and PlusGrade variants, plus the 82–98 Details codes when --include-details) for one PCGSNo in one command, persist the full population curve to local store, and print the scarcity table.

  _Use this when you need scarcity context across grades — dealer pricing, key-date analysis, or seeing where Details-grade coins fit relative to numerical grades._

  ```bash
  pcgs-pp-cli coin pop-curve 7356 --plus --include-details --json
  ```
- **`order hydrate`** — Take a PCGS submission number, fetch the order, then fan out CoinFacts (and optionally images) for every cert in the order. Respects cache and refuses to start when remaining quota is less than the cert count.

  _Use this the moment a PCGS submission posts. It turns one submission number into a fully-cached, fully-hydrated set of coins in one command._

  ```bash
  pcgs-pp-cli order hydrate 12345678 --with-images --json
  ```

### Local state that compounds
- **`audit`** — Query the lookup_log table directly: every API call, its endpoint, IsValidRequest, ServerMessage, and request hash. Aggregate by day, by endpoint, or by cert; filter to failed calls only.

  _Reach for this when quota usage looks off, when you want to spot which certs return IsValidRequest=false, or when triaging a sync diff._

  ```bash
  pcgs-pp-cli audit --since 7d --failed --by-endpoint --json
  ```
- **`search`** — FTS5 + numeric filters over your local cache: search by Name, Country, SeriesName, Category, Designer, MintLocation, or variety fields, plus range filters on Year, Grade, PriceGuideValue, Population, PopHigher, Mintage, Weight, Diameter. --max-pop and --top-pct flags expose continuous rarity slicing (e.g. --top-pct 5 = top 5% rarest in the scoped cohort) — no API call.

  _Use this any time you need to slice your cached collection without burning quota. --top-pct 1 surfaces apex-rarity coins; --max-pop N for explicit thresholds._

  ```bash
  pcgs-pp-cli search --text "morgan dollar" --year 1881 --top-pct 5 --json --select Name,Year,Grade,Population,PriceGuideValue
  ```

## Usage

Run `pcgs-pp-cli --help` for the full command reference and flag list.

## Commands

### banknote

PCGS Banknote lookups: facts and images. Same shape as coins, separate endpoint family.

- **`pcgs-pp-cli banknote facts-cert`** - Full banknote metadata for one PCGS Banknote cert. Optional language code for translated text.
- **`pcgs-pp-cli banknote facts-grade`** - Banknote snapshot for a (PCGSNo, GradeNo) tuple.
- **`pcgs-pp-cli banknote images`** - Image URLs for one PCGS Banknote cert.

### coin

PCGS-graded coin lookups: CoinFacts metadata, Auction Prices Realized (APR), and images.

- **`pcgs-pp-cli coin apr-barcode`** - Auction Prices Realized by holder barcode with optional date window.
- **`pcgs-pp-cli coin apr-cert`** - Auction Prices Realized for one PCGS cert number.
- **`pcgs-pp-cli coin apr-grade`** - Auction Prices Realized for a (PCGSNo, GradeNo, PlusGrade) tuple with optional date window and result limit.
- **`pcgs-pp-cli coin facts-barcode`** - CoinFacts metadata by holder barcode. Supports PCGS and competitor-service barcodes (NGC, ANACS, ICG, SEGS).
- **`pcgs-pp-cli coin facts-cert`** - Full CoinFacts metadata for one PCGS cert number. The IsValidRequest + ServerMessage envelope tells you if the cert is legitimate.
- **`pcgs-pp-cli coin facts-grade`** - CoinFacts snapshot for a (PCGSNo, GradeNo, PlusGrade) tuple. Used by pop-curve to fan grades 1-70 + Plus + Details (82-98).
- **`pcgs-pp-cli coin images`** - TrueView and stock images for one PCGS cert (URLs only; no binary download).

### order

PCGS submission and order lookups for submitters.

- **`pcgs-pp-cli order range`** - Orders within a date window (paginated).
- **`pcgs-pp-cli order submission`** - Orders associated with one PCGS submission number.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
pcgs-pp-cli banknote facts-cert mock-value

# JSON for scripting and agents
pcgs-pp-cli banknote facts-cert mock-value --json

# Filter to specific fields
pcgs-pp-cli banknote facts-cert mock-value --json --select id,name,status

# Dry run — show the request without sending
pcgs-pp-cli banknote facts-cert mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
pcgs-pp-cli banknote facts-cert mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
pcgs-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/pcgs-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `PCGS_AUTH_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `pcgs-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $PCGS_AUTH_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **IsValidRequest = false with ServerMessage 'Invalid CertNo'** — The cert number was not in the expected format. Confirm digits only and the right length; pcgs-pp-cli coin batch --file <file> --list-certs normalizes plus-grade slab IDs like 7130.67/51225377 down to the bare cert number.
- **IsValidRequest = true but ServerMessage = 'No data found'** — PCGS has no record for that cert number — it is not a typo, the API just has no row. Exit code 3 distinguishes this from invalid input. Confirm against pcgs.com/cert before treating it as suspect.
- **Exit code 7 with 'PCGS daily quota exceeded'** — 1,000-call budget burned for today. Run pcgs-pp-cli --quota --json to see the reset time; use pcgs-pp-cli coin batch --resumable to pick up tomorrow without re-spending today's calls.
- **HTTP 500 from the API** — PCGS returns 500 for invalid credentials AND for genuine server errors. Run pcgs-pp-cli doctor --json; if doctor passes, retry once.
- **HTTP 204 with empty body** — PCGS returns 204 for empty request data (a required param was missing on the wire). Re-run with --verbose to see the constructed request URL.
- **Cached row's Year or Mintage looks wrong after sync** — sync never writes identity fields. The cached identity came from the original lookup. Re-run pcgs-pp-cli coin facts-cert <cert> --no-cache --json to compare; if they still differ, file a PCGS data-correction issue — the cache is not at fault.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**Cfomodz/what-bot**](https://github.com/Cfomodz/what-bot) — Python (3 stars)
- [**BobdaFett/pcgs-inv-gui**](https://github.com/BobdaFett/pcgs-inv-gui) — Python
- [**BobdaFett/pcgs-inv**](https://github.com/BobdaFett/pcgs-inv) — C#
- [**Cfomodz/PCGS-slab-picture-to-listing-tool**](https://github.com/Cfomodz/PCGS-slab-picture-to-listing-tool) — Python
- [**pixiitech/lustre**](https://github.com/pixiitech/lustre) — Ruby
- [**evansminotwood/Aureus**](https://github.com/evansminotwood/Aureus) — Go
- [**Arunavanag1/anags-bullion-tracker**](https://github.com/Arunavanag1/anags-bullion-tracker) — TypeScript
- [**PhillyG76/AuctionEye**](https://github.com/PhillyG76/AuctionEye) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
