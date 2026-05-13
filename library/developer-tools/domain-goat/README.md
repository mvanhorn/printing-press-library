# Domain Goat CLI

**Identify domains worth buying — across RDAP, WHOIS, and Porkbun pricing — without ever leaving the terminal.**

domain-goat absorbs everything dnstwist, openrdap, whois, and the abandoned domainr-cli ever did, then transcends with a local SQLite shortlist that knows your scores, your notes, your watch list, and 5-year renewal cost for every TLD — so you can find domains worth registering instead of just typing names into instant-domain-search.com five at a time.

## Install

The recommended path installs both the `domain-goat-pp-cli` binary and the `pp-domain-goat` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install domain-goat
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install domain-goat --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/domain-goat-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-domain-goat --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-domain-goat --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-domain-goat skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-domain-goat. The skill defines how its required CLI can be installed.
```

## Authentication

No auth required for the headline path. RDAP, WHOIS port 43, and the Porkbun public pricing endpoint are all open.

The optional Namecheap adapter enriches live availability and pricing when you export the following before invoking `namecheap` commands:

- `NAMECHEAP_USERNAME` (or legacy `NAMECHEAP_API_USER`)
- `NAMECHEAP_API_KEY`
- `NAMECHEAP_CLIENT_IP` — your whitelisted client IP (required by Namecheap)

Set `NAMECHEAP_SANDBOX=1` to hit the sandbox endpoint. Verify configuration with `domain-goat-pp-cli doctor`.

## Quick Start

```bash
# Pull the IANA RDAP bootstrap and Porkbun pricing snapshot once — everything offline after this.
domain-goat-pp-cli tlds sync


# RDAP-native availability with WHOIS fallback for the three TLDs you're stuck on.
domain-goat-pp-cli check kindred.io kindred.ai kindred.studio --json


# Generate 50 brandable variants, filter to only available ones, all offline.
domain-goat-pp-cli gen suggest --seeds kindred,studio --tlds com,io,ai,studio --available-only --count 50


# Save survivors to a shortlist with optional notes and tags.
domain-goat-pp-cli lists add ai-startup kindred.ai lumen.ai novella.ai


# Rank by combined score/price/availability and promote the top 10 to a finalist sub-list.
domain-goat-pp-cli shortlist promote --list ai-startup --top 10 --by combined


# Side-by-side: scores, prices, RDAP status, drop flags.
domain-goat-pp-cli compare kindred.ai lumen.ai novella.ai --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`shortlist promote`** — Promote the top-N candidates from a list into a finalist sub-list ranked by combined score, price, and availability.

  _When an agent needs to converge on a buyable shortlist, this collapses 'fetch scores, fetch prices, fetch availability, sort, persist' into one deterministic call._

  ```bash
  domain-goat-pp-cli shortlist promote --list ai-startup --top 10 --by combined --agent
  ```
- **`budget`** — Filter candidates whose 5-year total cost (registration + 4 renewals) is under your ceiling, sorted ascending.

  _Avoid the $12-year-1 / $4,800-renewal trap before falling in love with a name._

  ```bash
  domain-goat-pp-cli budget --list ai-startup --max-annual-cost 50 --years 5 --available-only
  ```
- **`compare`** — One row per domain with score, length, TLD prestige, cross-registrar price, RDAP status, and drop flag.

  _Lets an agent or user finalize a shortlist in one command instead of cross-referencing 5 tabs._

  ```bash
  domain-goat-pp-cli compare kindred.io kindred.ai kindred.studio --json --select status,price,score
  ```
- **`why-killed`** — Show why a domain is no longer on the active shortlist: status, score, notes, tag history, last price.

  _Agency teams kill the same name twice in a sprint without this — and an agent re-suggesting a killed name burns user trust._

  ```bash
  domain-goat-pp-cli why-killed kindred.studio --json
  ```
- **`tld-affinity`** — Given a seed keyword, rank TLDs by suffix-semantics fit, historical availability rate in local history, and price tier.

  _Helps an agent pick generative TLDs before running expensive permutation passes._

  ```bash
  domain-goat-pp-cli tld-affinity kindred --top 10 --json
  ```

### Reachability + persistence
- **`drops timeline`** — Time-axis view of every watched domain hitting pendingDelete/redemptionPeriod, filtered by brandability score and TLD.

  _Drop-catchers and brand investors want to know 'what's queued for next week that's worth bidding on?' This answers it._

  ```bash
  domain-goat-pp-cli drops timeline --days 30 --min-score 7 --tld io,ai --agent
  ```
- **`pricing-arbitrage`** — Rank TLDs by renewal-delta (year-1 trap risk) or by prestige-to-price ratio.

  _Helps users and agents avoid TLDs where year-2 pricing destroys the deal._

  ```bash
  domain-goat-pp-cli pricing-arbitrage --by renewal-delta --top 20 --agent
  ```
- **`drop-bid-window`** — Compute the exact UTC re-release window for a domain in pendingDelete (RDAP event + 5-day grace).

  _Drop-catchers need minute-level timing; this replaces hand-parsed WHOIS regex._

  ```bash
  domain-goat-pp-cli drop-bid-window expiring.io --json
  ```

## Usage

Run `domain-goat-pp-cli --help` for the full command reference and flag list.

## Commands

### Availability & lookup

- **`check`** — Check whether one or more domains are available (RDAP → WHOIS → DNS fallback).
- **`rdap`** — RDAP lookup (RFC 7480-7484) via IANA bootstrap.
- **`whois`** — WHOIS lookup (RFC 3912, TCP/43) with parsed output.
- **`dns`** — DNS lookups (A/AAAA/NS/MX/SOA) — fast availability pre-filter.
- **`cert`** — Inspect the TLS certificate for a domain.

### Generate & score

- **`gen`** — Offline generators (suggest, mix, affix, blend, hack, rhyme) that produce candidate names from seed words.
- **`similar`** — Generate typosquat / similar-name variations (dnstwist-style).
- **`score`** — Score a domain on brandability (length, syllables, dictionary, TLD prestige).
- **`socials`** — Check whether a handle is taken on common social platforms.

### Shortlists & comparison

- **`lists`** — Manage candidate shortlists (saved domain sets with notes and tags).
- **`shortlist`** — Promote and manage finalist shortlists from candidate lists.
- **`compare`** — Side-by-side: score, length, TLD prestige, price, RDAP status, drop flag.
- **`why-killed`** — Show why a domain is no longer on the active shortlist.
- **`budget`** — Filter candidates by 5-year true cost (registration + N renewals).

### Drops & watching

- **`drops`** — Surface domains in their expiry / pendingDelete / redemption window.
- **`drop-bid-window`** — Compute exact UTC re-release window for a domain in pendingDelete.
- **`watch`** — Drop / expiry watch — periodically re-check domains and persist status.

### Pricing & TLDs

- **`pricing`** — Manage Porkbun TLD pricing snapshots (no auth required).
- **`pricing-arbitrage`** — Rank TLDs by year-1-trap risk (renewal-delta) or prestige-to-price ratio.
- **`tlds`** — Manage the local TLD table (IANA RDAP bootstrap + metadata).
- **`tld-affinity`** — Rank TLDs by fit for a seed keyword (suffix-semantics + price tier + historical availability).
- **`namecheap`** — Namecheap registrar adapter for live availability + pricing.

### Sync & local data

- **`sync`** — Sync API data to local SQLite for offline search and analysis.
- **`import`** — Import data from a JSONL file via API create/upsert calls.

### Agent & utility

- **`doctor`** — Check CLI health (auth + connectivity).
- **`agent-context`** — Emit structured JSON describing this CLI for agents.
- **`api`** — Browse all API endpoints by interface name.
- **`which`** — Find the command that implements a capability.
- **`workflow`** — Compound workflows that combine multiple API operations.
- **`profile`** — Save and apply named sets of flags.
- **`feedback`** — Record feedback about this CLI.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
domain-goat-pp-cli pricing show com

# JSON for scripting and agents
domain-goat-pp-cli pricing show com --json

# Filter to specific fields
domain-goat-pp-cli compare kindred.io kindred.ai --json --select status,price,score

# Dry run — show the request without sending
domain-goat-pp-cli pricing sync --dry-run

# Agent mode — JSON + compact + no prompts in one flag
domain-goat-pp-cli compare kindred.io kindred.ai --agent
```

## Cookbook

Recipes that show why this CLI beats `curl` + `whois` + a spreadsheet.

```bash
# 1. One-shot health + offline-data warmup.
domain-goat-pp-cli doctor
domain-goat-pp-cli tlds sync
domain-goat-pp-cli pricing sync

# 2. Bulk availability across multiple TLDs (RDAP → WHOIS → DNS fallback).
domain-goat-pp-cli check kindred --tlds com,io,ai,studio,app --json

# 3. Read names from a file, attach price + score per result, agent-friendly.
domain-goat-pp-cli check --file names.txt --tlds com,io --include-price --include-score --agent

# 4. Generate 100 brandable variants from two seeds, available-only.
domain-goat-pp-cli gen suggest --seeds kindred,studio --tlds com,io,ai --available-only --count 100

# 5. Score a single domain on brandability (length, syllables, dictionary, TLD prestige).
domain-goat-pp-cli score kindred.ai --json

# 6. Build a shortlist with notes + tags, then dump it.
domain-goat-pp-cli lists create ai-startup
domain-goat-pp-cli lists add ai-startup kindred.io lumen.ai novella.ai --notes "founder favorites" --tags short,brandable
domain-goat-pp-cli lists show ai-startup --json

# 7. Promote the top-10 by combined score/price/availability into a finalists list.
domain-goat-pp-cli shortlist promote --list ai-startup --top 10 --by combined --dest ai-startup-finalists

# 8. Filter candidates by 5-year true cost — surface the year-2 renewal trap.
domain-goat-pp-cli budget --list ai-startup --years 5 --max-annual-cost 50 --available-only --top 20

# 9. Side-by-side comparison with selected fields.
domain-goat-pp-cli compare kindred.io kindred.ai kindred.studio --json --select status,price,score

# 10. Why is this domain no longer on the active list?
domain-goat-pp-cli why-killed kindred.studio --json

# 11. Rank TLDs by year-1-trap risk vs prestige-to-price ratio.
domain-goat-pp-cli pricing-arbitrage --by renewal-delta --top 20 --json
domain-goat-pp-cli pricing-arbitrage --by prestige-value --top 20 --json

# 12. Best-fit TLDs for a seed keyword (semantic + price + availability).
domain-goat-pp-cli tld-affinity kindred --top 10 --json

# 13. Drops timeline — what's worth bidding on this month?
domain-goat-pp-cli drops timeline --days 30 --min-score 7 --tld io,ai --agent

# 14. Exact UTC re-release window for a domain in pendingDelete.
domain-goat-pp-cli drop-bid-window expiring.io --json

# 15. Watch a set of domains over time and persist status changes.
domain-goat-pp-cli watch add kindred.io --cadence 12
domain-goat-pp-cli watch add kindred.ai --cadence 12
domain-goat-pp-cli watch run --json
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

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-domain-goat -g
```

Then invoke `/pp-domain-goat <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add domain-goat domain-goat-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/domain-goat-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "domain-goat": {
      "command": "domain-goat-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
domain-goat-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/domain-goat-pp-cli/config.toml` (override via `DOMAIN_GOAT_CONFIG`).

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

- `DOMAIN_GOAT_CONFIG` — path to an alternate config file.
- `DOMAIN_GOAT_BASE_URL` — override the Porkbun API base URL (e.g., a self-hosted proxy).
- `DOMAIN_GOAT_FEEDBACK_ENDPOINT` — upstream URL for `feedback` to send to when opted in.
- `DOMAIN_GOAT_FEEDBACK_AUTO_SEND` — set to `1` / `true` to auto-send feedback without prompting.
- `NAMECHEAP_USERNAME` (or legacy `NAMECHEAP_API_USER`) — Namecheap API user.
- `NAMECHEAP_API_KEY` — Namecheap API key.
- `NAMECHEAP_CLIENT_IP` — whitelisted client IP for the Namecheap API.
- `NAMECHEAP_SANDBOX` — set to `1` / `true` to hit Namecheap's sandbox endpoint.
- `NO_COLOR` — disable colored output (any non-empty value).

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **RDAP returns 5xx or no bootstrap entry for a ccTLD** — domain-goat-pp-cli falls back to WHOIS port 43 automatically; verify with `--source rdap` vs `--source whois`.
- **Verisign WHOIS port 43 tarpitting your bulk run** — pass `--source rdap` (RDAP at rdap.verisign.com is rate-limit safer for .com/.net) or lower concurrency with `--parallel 2`.
- **Namecheap returns 1011102 (invalid IP)** — export `NAMECHEAP_CLIENT_IP=<the IP whitelisted in your Namecheap API settings>`; the IP that hits Namecheap must match what you registered exactly.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**dnstwist**](https://github.com/elceef/dnstwist) — Python (5500 stars)
- [**likexian/whois**](https://github.com/likexian/whois) — Go (700 stars)
- [**whoiser**](https://github.com/LayeredStudio/whoiser) — JavaScript (600 stars)
- [**openrdap/rdap**](https://github.com/openrdap/rdap) — Go (250 stars)
- [**domainr-cli (archived)**](https://github.com/MichaelThessel/domainr-cli) — Go (25 stars)
- [**dorukardahan/domain-search-mcp**](https://github.com/dorukardahan/domain-search-mcp) — TypeScript
- [**saidutt46/domain-check**](https://github.com/saidutt46/domain-check) — TypeScript
- [**bharathvaj-ganesan/whois-mcp**](https://github.com/bharathvaj-ganesan/whois-mcp) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
