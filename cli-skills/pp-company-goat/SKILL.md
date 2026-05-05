---
name: pp-company-goat
description: "Look up startups across SEC Form D, GitHub, Hacker News, Companies House, YC, and Wikidata in one command — including the SEC fundraising data hidden behind paid Crunchbase tiers. Trigger phrases: `look up this startup`, `research <company>`, `what does <company> do`, `form D for <company>`, `is <company> still active`, `compare <a> and <b>`, `use company-goat`, `run company-goat-pp-cli`."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - company-goat-pp-cli
    install:
      - kind: go
        bins: [company-goat-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/developer-tools/company-goat/cmd/company-goat-pp-cli
---

# Company GOAT — Printing Press CLI

company-goat-pp-cli fans out across seven free authoritative sources to research small and midsize startups in seconds. The killer feature is SEC Form D extraction: most US startups raising priced rounds file Form D with the SEC, and almost nobody outside finance knows this is queryable for free. After one snapshot, you know whether they raised, are shipping code, are talked about on HN, and are a real legal entity — without eight browser tabs.

## When to Use This CLI

Use Company GOAT when an agent or human needs structured company-research data on a small or midsize startup — fundraising, engineering signal, mention timeline, legal entity, domain age — without paying for Crunchbase Pro. Best for one-company-at-a-time deep research. Not for bulk discovery (sources don't scale to thousands of queries) and not for global private-market funding-graph queries (no source has that for free).

### The killer feature: SEC Form D, free

Most US startups raising priced equity rounds (Series Seed → Series C, Reg D 506(b)/506(c)) file **Form D** with the SEC within 15 days of first sale. The filing names the issuer, the offering amount, the exemption claimed, and the related persons (officers, directors, promoters). Crunchbase Pro charges $999/year for a wrapper around this same source; this CLI extracts it directly from EDGAR for free. Reach for `funding` when an agent needs to verify a US startup's actual fundraising history rather than quote Crunchbase Free's empty "undisclosed" rows.

### Coverage at a glance

- **SEC Form D / `funding` / `funding-trend`** — US-only. Reg D priced rounds. Pre-Series-A SAFEs are not covered. Empty filings + a `coverage_note` is the expected response for non-US companies, pre-priced-round companies, and companies that just haven't filed yet. When Form D is empty, `funding` falls back to a broader EDGAR full-text search and surfaces mentions binned by signal class: subsidiary (10-K parent mentions), debt (Venture Lending and Leasing portfolio mentions), acquisition (8-K parent disclosures), and other. `funding --who` also searches across all form types, not just Form D. **Disambiguation:** EDGAR's full-text search matches by name fragment, so "Notion" hits both Notion Labs and Notion Capital VC. The result includes `cik_summaries` and `is_ambiguous: true` whenever multiple distinct CIKs match. Re-call with `--cik <id>` (CIK from the summary) to filter to the correct entity. The compound commands (`snapshot`, `compare`, `signal`) propagate the ambiguity flag through their output and refuse to synthesize a side-by-side number when ambiguous — the agent decides which CIK is correct using broader context (Wikidata founders, GitHub org alignment, the user's intent) and re-runs `funding --cik` to confirm. `signal` surfaces `Form D match is ambiguous` as its own signal class so cross-source consistency checks aren't trusted blindly.
- **Companies House / `legal --region uk`** — UK Ltd / PLC only. Requires `COMPANIES_HOUSE_API_KEY` (free at developer.companieshouse.gov.uk).
- **GitHub / `engineering`** — Any public org. Optional `GITHUB_TOKEN` raises rate limits from 60/hr to 5000/hr.
- **Hacker News / `mentions` / `launches`** — Algolia full-text search across HN since 2007. No auth. `mentions` returns both a year-month histogram AND the top-N stories sorted by points (use `--top N` to widen, default 5). `launches` is the Show-HN-only flavor — use it for "did this company ever post a Show HN" questions; use `mentions` for "what do people say about this company on HN."
- **YC directory / `yc`** — YC-backed companies only.
- **Wikidata / `wiki`** — Sparse on early-stage; mostly useful for established companies.
- **RDAP / DNS / `domain`** — Any registered domain.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

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

## Command Reference

**filings** — SEC EDGAR Form D filings — the primary data source for US private fundraising disclosure

- `company-goat-pp-cli filings` — Fetch all SEC submissions for a given CIK (Central Index Key). Used as the seed call when resolving a company's...


**Hand-written commands**

- `company-goat-pp-cli resolve <name-or-domain>` — Resolve a company name to a canonical domain. Returns numbered candidates if ambiguous; --pick N or --domain to...
- `company-goat-pp-cli funding <co>` — SEC EDGAR Form D filings + YC batch lookup. Shows offering amount, filing date, exemption claimed, related entities...
- `company-goat-pp-cli engineering <co>` — GitHub org metadata: repo count, contributor count, commit cadence, top languages.
- `company-goat-pp-cli launches <co>` — Show HN posts about this company, sorted by points. Includes launch year for spotting dead vs. active launches.
- `company-goat-pp-cli mentions <co>` — Hacker News mention timeline: monthly histogram of mentions over time via Algolia full-text search.
- `company-goat-pp-cli legal <co>` — Legal entity lookup. UK via Companies House (optional COMPANIES_HOUSE_API_KEY); US via SEC EDGAR Form D issuer...
- `company-goat-pp-cli yc <co>` — Y Combinator directory entry if the company was YC-backed: batch, status, location, description.
- `company-goat-pp-cli wiki <co>` — Wikidata company facts: founded date, founders, HQ, industry, key people. Sparse on early-stage startups.
- `company-goat-pp-cli domain <co>` — Domain age via RDAP/WHOIS, DNS records, and CNAME-based hosting hint (Vercel/Netlify/Heroku/Cloudflare Pages/AWS/GCP).
- `company-goat-pp-cli snapshot <co>` — Fan out across all 7 sources in parallel and render a unified summary. The headline command. Uses cliutil.FanoutRun...
- `company-goat-pp-cli compare <a> <b>` — Two snapshots side-by-side, aligned by section. For evaluating which of two startups looks healthier.
- `company-goat-pp-cli search <query>` — Search the YC directory by free text + `--batch` and `--industry` filters. Cross-source FTS5 over the synced store is on the v1 roadmap; today this is YC-only.
- `company-goat-pp-cli signal <co>` — Cross-source consistency check. Flags suspicious patterns like 'raised in 2024 but no GitHub commits since 2022'....
- `company-goat-pp-cli funding-trend <co>` — Time series of Form D filings over time. Useful for charting a startup's fundraising cadence.
- `company-goat-pp-cli sync` — Pull syncable resources (YC directory and any other configured) into local SQLite. Use `--resources` to pick a subset. Subsequent reads with `--data-source local` query offline.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
company-goat-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Quick due diligence on a US startup

```bash
company-goat-pp-cli snapshot --domain anthropic.com --json
```

Fan out across all 7 sources and render a unified summary. Use `--domain` to skip name resolution and go straight to the canonical entity. Each source returns `{status, data, note, elapsed}` so partial failures are visible.

### Find every SEC filing for a founder

```bash
company-goat-pp-cli funding --who 'Patrick Collison' --json
```

Returns every Form D filing where this person is named — useful for mapping serial founders and repeat investors.

### Compare two competing startups

```bash
company-goat-pp-cli compare ramp.com brex.com
```

Aligns snapshots side-by-side; agent-readable. Use for tradeoff analysis.

### Build a research database

```bash
company-goat-pp-cli sync && company-goat-pp-cli search 'fintech'
```

Pulls the YC directory and any other syncable resources into local SQLite, then full-text searches the synced index. Subsequent reads can use `--data-source local` to query offline.

### Surface suspicious patterns

```bash
company-goat-pp-cli signal --domain acme-corp.com --json
```

Cross-source consistency check — flags cases like 'raised $X in 2024 but GitHub silent since 2022' that no single source would catch.

## Auth Setup

No required keys. Three optional environment variables expand coverage and rate limits:

- `COMPANY_PP_CONTACT_EMAIL=you@example.com` — Sent in the SEC EDGAR User-Agent header to comply with EDGAR's fair-access policy. Recommended for any non-trivial use of `funding` / `funding-trend` / `snapshot`.
- `GITHUB_TOKEN` — Raises GitHub API rate limit from 60/hr to 5000/hr. `gh auth token` works. Used by `engineering`, `snapshot`.
- `COMPANIES_HOUSE_API_KEY` — Required for `legal --region uk`. Register free at developer.companieshouse.gov.uk and create a REST application.

SEC EDGAR requests are paced and retried automatically. The CLI honors `Retry-After`, backs off on 429/5xx responses, and exits with code 7 if SEC continues throttling after retries.

Run `company-goat-pp-cli doctor` to verify which of these are detected.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  company-goat-pp-cli filings --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
company-goat-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
company-goat-pp-cli feedback --stdin < notes.txt
company-goat-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.company-goat-pp-cli/feedback.jsonl`. They are never POSTed unless `COMPANY_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `COMPANY_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
company-goat-pp-cli profile save briefing --json
company-goat-pp-cli --profile briefing filings
company-goat-pp-cli profile list --json
company-goat-pp-cli profile show briefing
company-goat-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `company-goat-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → CLI installation
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.23+)
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/developer-tools/company-goat/cmd/company-goat-pp-cli@latest
   
   # If `@latest` installs a stale build (Go module proxy cache lag), install from main:
   GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
     go install github.com/mvanhorn/printing-press-library/library/developer-tools/company-goat/cmd/company-goat-pp-cli@main
   ```
3. Verify: `company-goat-pp-cli --version`
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/developer-tools/company-goat/cmd/company-goat-pp-mcp@latest
   
   # If `@latest` installs a stale build (Go module proxy cache lag), install from main:
   GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
     go install github.com/mvanhorn/printing-press-library/library/developer-tools/company-goat/cmd/company-goat-pp-mcp@main
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add company-goat-pp-mcp -- company-goat-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which company-goat-pp-cli`
   If not found, offer to install (see CLI Installation above).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   company-goat-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `company-goat-pp-cli <command> --help`.
