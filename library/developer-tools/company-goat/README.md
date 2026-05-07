# Company GOAT CLI

**Multi-source startup research from the terminal — including the SEC Form D fundraising data hidden behind paid Crunchbase tiers.**

company-goat-pp-cli fans out across seven free authoritative sources to research small and midsize startups in seconds.

## The killer feature: SEC Form D, free

Most US startups raising priced equity rounds (Series Seed → Series C, Reg D 506(b)/506(c)) file **Form D** with the SEC within 15 days of first sale. The filing names the issuer, the offering amount, the exemption claimed, and the related persons (officers, directors, promoters). It's free, public, and structured as XML — but almost nobody outside securities lawyers and investment bankers knows it's queryable.

Crunchbase Pro charges $999/year for what's substantially a wrapper around this same source. This CLI extracts it directly from EDGAR and gives you the structured filing — including offering amount and related-person graph — in one command:

```bash
company-goat-pp-cli funding stripe --json
company-goat-pp-cli funding-trend stripe --since 2018      # cadence over years
company-goat-pp-cli funding --who 'Patrick Collison'        # serial-founder graph
```

After one `snapshot`, you know whether they raised (Form D), are shipping code (GitHub), are talked about (HN), and are a legitimate legal entity (Companies House + EDGAR issuer fields) — without eight browser tabs.

## Coverage at a glance

| Source | Coverage | Auth |
|--------|----------|------|
| SEC EDGAR Form D | US private companies raising priced rounds (Reg D) | Contact email recommended (fair-access) |
| GitHub | Any public org | None required; GITHUB_TOKEN raises 60/hr → 5000/hr |
| Hacker News (Algolia) | Posts and comments since 2007 | None |
| Companies House | UK Ltd / PLC entities only | COMPANIES_HOUSE_API_KEY required |
| Y Combinator directory | YC-backed companies only | None |
| Wikidata SPARQL | Notable companies (sparse on early-stage) | None |
| RDAP / DNS | Any registered domain | None |

Form D is **US-only**. Non-US companies (Monzo, Klarna, etc.) won't appear in `funding` — use `legal --region uk` for UK entities. Pre-priced-round US startups (typically pre-Series A SAFE / convertible note) also won't appear, since SAFEs aren't covered by Reg D. The CLI prints a `coverage_note` whenever this is the likely reason for an empty result.

### When Form D is empty: broader EDGAR fallback

A company that raised only via SAFE or convertible notes, or that has been acquired, won't have Form D filings under its own name. But it may still appear extensively in EDGAR under other forms. Examples:

- **Acquired startup**: appears in the parent's 10-K Item 21 / EX-21 subsidiary list (e.g. June Life Inc. shows up across Weber Inc.'s post-acquisition 10-K filings).
- **Venture-debt portfolio company**: appears in venture-debt holders' 10-Q / 10-K filings (e.g. Venture Lending and Leasing VII / VIII portfolio reports).
- **Acquisition announcement**: appears in the parent's 8-K with the deal disclosure.

When all stem variants come back empty on Form D, `funding` automatically falls back to a broader EDGAR full-text search and bins the hits by signal class (`subsidiary`, `debt`, `acquisition`, `other`). Form D stays the headline; the broader fallback is what lights up companies that the killer feature alone would miss.

```bash
company-goat-pp-cli funding --domain junelife.com --json
# Form D: 0 filings
# Subsidiary signal: Weber Inc. 10-K (2021-12-14, EX-21)
# Debt signal: Venture Lending and Leasing VII, Inc. 10-Q (2019-08-14)
```

## Install

The recommended path installs both the `company-goat-pp-cli` binary and the `pp-company-goat` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install company-goat
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install company-goat --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/company-goat/cmd/company-goat-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/company-goat-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

## Authentication

No API keys required for the killer feature. Optional GITHUB_TOKEN raises engineering-signal rate limits from 60/hr to 5000/hr (works with `gh auth token`). Optional COMPANIES_HOUSE_API_KEY enables UK legal entity lookup; without it, `legal --region uk` prints setup instructions and other commands work normally.

SEC EDGAR's fair-access policy requires a contact email in the User-Agent header. Set `COMPANY_PP_CONTACT_EMAIL=you@example.com` (or run `company-goat-pp-cli config set sec.user_agent_email you@example.com`) so EDGAR can reach you if your traffic looks abusive. Without it, requests still work but EDGAR may rate-limit you faster.

## Quick Start

```bash
# The headline command — fan out across all 7 sources for a known domain.
company-goat-pp-cli snapshot --domain stripe.com

# The killer feature — pull SEC Form D filings (offering amount, date, exemption).
company-goat-pp-cli funding --domain stripe.com --json

# Engineering signal as JSON — feed to jq or pipe into another tool.
company-goat-pp-cli engineering --domain vercel.com --json

# Side-by-side comparison; great for due diligence.
company-goat-pp-cli compare ramp.com brex.com --json

# Cross-source consistency check — flags zombies and stale fundraising stories.
company-goat-pp-cli signal --domain ramp.com
```

Bare names (`snapshot stripe`) work too — when a name resolves to multiple plausible domains the CLI prints numbered candidates and exits with code 2; rerun with `--pick N` or pass `--domain` directly to skip resolution.

## Unique Features

These capabilities aren't available in any other tool for this API.

### Multi-source orchestration
- **`snapshot`** — Look up a company across SEC Form D, GitHub, Hacker News, Companies House, YC, Wikidata, and DNS in one command — rendered as a unified terminal summary in seconds.

  _When you need to evaluate a startup quickly, this is the one command that tells you whether they raised, are shipping code, are talked about, and are legitimate — without 8 browser tabs._

  ```bash
  company-goat-pp-cli snapshot --domain stripe.com --json
  ```
- **`signal`** — Surface suspicious cross-source patterns. Example: 'Form D says raised $5M in 2024 but GitHub org has 0 commits since 2022' or 'YC entry says Active but website domain expired'.

  _When deciding whether to engage with a startup, this command flags zombie companies and stale fundraising stories without you having to open every tab._

  ```bash
  company-goat-pp-cli signal --domain acme-corp.com --json
  ```

### Killer feature
- **`funding`** — See structured SEC Form D filings for a US private company — offering amount, filing date, exemption claimed (Reg D 506(b) vs 506(c)), related entities, and state of incorporation.

  _When an agent needs to verify a US startup's actual fundraising history, this is the single most reliable free signal. Avoid agents quoting Crunchbase Free's empty 'undisclosed' rows._

  ```bash
  company-goat-pp-cli funding --domain stripe.com --json
  ```
- **`funding-trend`** — Time series of Form D filings for a company across years — shows fundraising cadence and gaps. Useful for spotting 'they haven't raised since 2022' silently.

  _Use this when an agent needs to summarize a company's fundraising arc, not just the latest round._

  ```bash
  company-goat-pp-cli funding-trend --domain stripe.com --since 2018 --json
  ```
- **`funding --who`** — Show every Form D filing that names a given person (officer, large holder). Reveals serial founders, repeat advisors, prolific investors.

  _Use when an agent needs to map who's behind a constellation of startups, or verify a founder's actual filing history._

  ```bash
  company-goat-pp-cli funding --who 'Patrick Collison' --json
  ```

### Local state that compounds
- **`search`** — Search the YC directory by free text + --batch and --industry filters. Free-text matches against name, one-liner description, industry, and location.

  _An agent tasked with 'find a YC fintech with "agent" in the description' has a one-shot query rather than scrolling the YC directory._

  ```bash
  company-goat-pp-cli search 'agent' --industry fintech
  ```
- **`compare`** — Two snapshots aligned column-by-column for direct comparison. Free in this CLI; paid feature elsewhere.

  _When evaluating two competing startups, this is the one-shot comparison that doesn't require flipping between tabs._

  ```bash
  company-goat-pp-cli compare stripe.com adyen.com --json
  ```

## Usage

Run `company-goat-pp-cli --help` for the full command reference and flag list.

## Commands

### filings

SEC EDGAR Form D filings — the primary data source for US private fundraising disclosure

- **`company-goat-pp-cli filings list`** - Fetch all SEC submissions for a given CIK (Central Index Key). Used as the seed call when resolving a company's filing history.


## Output Formats

```bash
# Human-readable summary (default in terminal, JSON when piped)
company-goat-pp-cli snapshot --domain stripe.com

# JSON for scripting and agents
company-goat-pp-cli snapshot --domain stripe.com --json

# Keep only specific top-level fields
company-goat-pp-cli snapshot --domain stripe.com --json --select sources,domain

# Dry run — exit early with no remote calls
company-goat-pp-cli funding --domain stripe.com --dry-run

# Agent mode — JSON + compact + no prompts in one flag
company-goat-pp-cli funding --domain stripe.com --agent
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
npx skills add mvanhorn/printing-press-library/cli-skills/pp-company-goat -g
```

Then invoke `/pp-company-goat <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/company-goat/cmd/company-goat-pp-mcp@latest
```

Then register it:

```bash
claude mcp add company-goat company-goat-pp-mcp
```

</details>

<!-- pp-hermes-install-anchor -->
## Install via Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-company-goat --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-company-goat --force
```

## Install via OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-company-goat skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-company-goat. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/company-goat-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle, install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/company-goat/cmd/company-goat-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "company-goat": {
      "command": "company-goat-pp-mcp"
    }
  }
}
```

</details>

## Cookbook

```bash
# Pull SEC Form D filings for a US private company.
company-goat-pp-cli funding --domain stripe.com --json

# Time series of fundraising over years.
company-goat-pp-cli funding-trend --domain stripe.com --since 2018 --json

# Map a serial founder's filing trail across companies.
company-goat-pp-cli funding --who 'Patrick Collison' --json

# Engineering signal: GitHub org metrics + top repos.
company-goat-pp-cli engineering --domain vercel.com --json

# UK legal entity (requires COMPANIES_HOUSE_API_KEY).
company-goat-pp-cli legal --domain monzo.com --region uk --json

# US legal entity from SEC Form D issuer fields.
company-goat-pp-cli legal --domain stripe.com --region us --json

# Domain age + DNS + hosting hint via RDAP/WHOIS.
company-goat-pp-cli domain --domain anthropic.com --json

# YC directory entry.
company-goat-pp-cli yc --domain stripe.com --json

# Wikidata facts (founders, founded date, HQ).
company-goat-pp-cli wiki --domain stripe.com --json

# Hacker News mention timeline (year-month histogram).
company-goat-pp-cli mentions --domain stripe.com --json

# Show HN posts about a company, sorted by points.
company-goat-pp-cli launches --domain vercel.com --json

# Fan out across all 7 sources; partial-failure tolerant.
company-goat-pp-cli snapshot --domain ramp.com --json

# Side-by-side comparison.
company-goat-pp-cli compare ramp.com brex.com --json

# Cross-source signal check (zombie startups, stale fundraising).
company-goat-pp-cli signal --domain ramp.com --json

# Search the YC directory by industry tag.
company-goat-pp-cli search 'agent' --industry fintech --json

# Resolve a name to a canonical domain (or get disambiguation candidates).
company-goat-pp-cli resolve stripe --json

# Pipe Form D filings into jq for custom analysis.
company-goat-pp-cli funding --domain stripe.com --json | jq '.form_d_filings[] | {date: .filing_date, amount: .offering_amount}'
```

## Health Check

```bash
company-goat-pp-cli doctor
```

Verifies configuration and connectivity to the upstream sources (SEC, GitHub, HN, RDAP, Wikidata, YC, Companies House if keyed).

## Configuration

Config file: `~/.config/company-goat-pp-cli/config.toml`

Environment variables (all optional):

| Variable | Used by | Effect |
|----------|---------|--------|
| `COMPANY_PP_CONTACT_EMAIL` | `funding`, `funding-trend`, `snapshot` | Sets the SEC EDGAR User-Agent contact email (fair-access policy). Recommended for any non-trivial use. |
| `GITHUB_TOKEN` | `engineering`, `snapshot` | Raises GitHub rate limit from 60/hr to 5000/hr. `gh auth token` works. |
| `COMPANIES_HOUSE_API_KEY` | `legal --region uk` | Required for UK legal entity lookup. Free at developer.companieshouse.gov.uk. |
| `COMPANY_FEEDBACK_ENDPOINT` / `COMPANY_FEEDBACK_AUTO_SEND` | `feedback` | Opt-in upstream POST for collected feedback (off by default; entries stay local). |

SEC EDGAR calls use conservative client-side pacing and adaptive retry behavior: the CLI honors `Retry-After`, backs off on 429/5xx responses, and lowers its per-process request rate after throttling.

## Troubleshooting

Exit codes: `0` success, `2` ambiguous (rerun with `--pick N` or `--domain`), `3` not found, `4` no candidates resolved (pass `--domain`), `5` API error / no filings for resolved entity, `7` rate limited, `10` config error.

| Symptom | Cause | Fix |
|---------|-------|-----|
| `funding stripe` returns "ambiguous" with candidates (exit 2) | Resolver found multiple plausible domains | Rerun with `--pick N` (1-indexed) or `--domain stripe.com` |
| `funding` says "no Form D filings found" + a `coverage_note` | Form D is US-only and Reg-D-priced-rounds-only | Expected for non-US companies (Monzo, Klarna), pre-Series-A SAFE companies, or companies that just haven't filed yet |
| SEC EDGAR returns 403 or repeated 429s | EDGAR fair-access policy requires a contact email, and may throttle IP/User-Agent bursts | `export COMPANY_PP_CONTACT_EMAIL=you@example.com`; retry after the printed cooldown if exit code is 7 |
| GitHub commands say "rate limit exceeded" | Unauthenticated GitHub allows 60 req/hr | `export GITHUB_TOKEN=$(gh auth token)` raises to 5000/hr |
| `legal --region uk` prints "requires COMPANIES_HOUSE_API_KEY" | Companies House requires registration | Register free at developer.companieshouse.gov.uk, then `export COMPANIES_HOUSE_API_KEY=...` |
| `snapshot stripe` returns "no candidates found" (exit 4) | Name resolution couldn't find a matching domain | Pass `--domain` explicitly: `snapshot --domain stripe.com` |
| `wiki` returns sparse / empty data | Wikidata is sparse on early-stage companies | Expected for non-notable startups; wikidata only covers the well-known |

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**yc-oss/api**](https://github.com/yc-oss/api) — TypeScript
- [**edgartools**](https://github.com/dgunning/edgartools) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
