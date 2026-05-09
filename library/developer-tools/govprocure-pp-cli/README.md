# govprocure-pp-cli

![Go Version](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go) ![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg) ![APIs](https://img.shields.io/badge/APIs-grants.gov%20%7C%20SAM.gov%20%7C%20USASpending.gov-blue) ![Printed by](https://img.shields.io/badge/Printed%20by-%40silentnw-green)

**The only open-source CLI for U.S. government procurement intelligence. Search grants, contracts, and award history from three federal APIs — offline-first, agent-native, and built for the Printing Press ecosystem.**

---

## Quick Start

```bash
# Install via Printing Press
npx -y @mvanhorn/printing-press install govprocure

# Set your SAM.gov API key (free at api.sam.gov)
govprocure-pp-cli auth set-key --sam YOUR_KEY

# Verify all three APIs are reachable
govprocure-pp-cli doctor

# Sync data locally
govprocure-pp-cli sync --all

# Search
govprocure-pp-cli grants search "AI consulting small business"
govprocure-pp-cli sam set-asides --type SDVOSB
govprocure-pp-cli compound pipeline "cybersecurity SDVOSB"
```

---

## Why This Exists

Federal procurement data is spread across three agencies, each with different APIs, different authentication models, and incompatible schemas:

- **grants.gov** — grant opportunities, open to anyone, no auth required, but the search API is underdocumented and the result schema changes without notice
- **SAM.gov** — contract opportunities, set-asides, and vendor registrations; requires a free API key and rate-limits unauthenticated requests to 1 req/sec
- **USASpending.gov** — historical award data aggregated from all federal agencies; no auth, but POST-only search, complex filters, and 30–90 day lag from agencies

This CLI unifies all three into one binary with a consistent interface:

| Problem | Solution |
|---|---|
| Three different APIs, three different auth models | Single `auth` namespace handles all credentials |
| Can't search across sources at once | `compound` commands join all three in one query |
| Live APIs are slow for iteration | SQLite local mirror with FTS5: 50,000+ opportunities searchable in milliseconds |
| AI agents burn tokens on verbose JSON | `--compact` flag cuts token cost 60–80%; typed exit codes for programmatic control |
| No way to connect agents to procurement data | MCP server included: Stephanie and any MCP-compatible agent can query directly |

---

## Command Reference

### Top-Level Commands

| Command | Description |
|---|---|
| `govprocure-pp-cli doctor` | Check API connectivity, auth status, and DB health |
| `govprocure-pp-cli sync --all` | Sync all three data sources into local SQLite |
| `govprocure-pp-cli sync --source grants` | Sync only grants.gov |
| `govprocure-pp-cli sync --source sam` | Sync only SAM.gov opportunities |
| `govprocure-pp-cli sync --source usaspending` | Sync only USASpending.gov awards |
| `govprocure-pp-cli auth set-key --sam KEY` | Store SAM.gov API key |
| `govprocure-pp-cli auth status` | Show current auth and key presence |
| `govprocure-pp-cli version` | Show version and build info |

### grants Namespace

| Command | Description |
|---|---|
| `grants search "QUERY"` | FTS5 full-text search across synced grants |
| `grants search "QUERY" --live` | Search grants.gov live API (slower, bypasses local DB) |
| `grants list --status open` | List open grants, sorted by close date |
| `grants list --closing-in 7` | Grants closing within N days |
| `grants show OPPORTUNITY_ID` | Full detail for one grant |
| `grants export --format csv` | Export results to CSV |

### sam Namespace

| Command | Description |
|---|---|
| `sam search "QUERY"` | FTS5 search across synced SAM.gov opportunities |
| `sam search "QUERY" --live` | Search SAM.gov live API |
| `sam set-asides --type SDVOSB` | Filter by set-aside type |
| `sam set-asides --type 8A` | 8(a) Small Disadvantaged Business opportunities |
| `sam set-asides --type WOSB` | Women-Owned Small Business opportunities |
| `sam set-asides --type HUBZone` | HUBZone Small Business opportunities |
| `sam set-asides --type SB` | All small business set-asides |
| `sam show NOTICE_ID` | Full detail for one opportunity |
| `sam list --naics 541511` | Filter by NAICS code |
| `sam list --agency "Dept of Defense"` | Filter by agency |

### usaspending Namespace

| Command | Description |
|---|---|
| `usaspending awards --agency "AGENCY"` | Historical awards for an agency |
| `usaspending awards --naics 541511` | Awards by NAICS code |
| `usaspending awards --recipient "COMPANY"` | Awards received by a specific company |
| `usaspending awards --keyword "QUERY"` | Keyword search across award descriptions |
| `usaspending show AWARD_ID` | Full detail for one award |
| `usaspending trends --naics 541511` | Spending trend by NAICS (by fiscal year) |

### compound Namespace

| Command | Description |
|---|---|
| `compound pipeline "QUERY"` | Search all three sources and merge results |
| `compound stale` | Grants/contracts with no recent award history = low competition |
| `compound profile "AGENCY"` | Agency's grant history + active opportunities + awards |

---

## API Sources

| Source | Auth | Rate Limit | What It Covers |
|---|---|---|---|
| grants.gov | None | 25 req/min | Federal grant opportunities (all agencies) |
| SAM.gov | Free API key | 100 req/min (with key) / 1 req/sec (without) | Federal contract opportunities, set-asides, vendor data |
| USASpending.gov | None | 60 req/min | Historical award data, spending trends, recipient analysis |

Get your free SAM.gov API key at [api.sam.gov](https://api.sam.gov). No payment required. Approval is typically same-day.

---

## Compound Commands

The `compound` namespace joins data across all three sources in a single query — something impossible through the live APIs alone because they have no shared schema or identifiers.

### `compound pipeline`

Runs parallel searches across all three sources, deduplicates by CFDA/NAICS where possible, and returns a unified ranked result set.

```bash
govprocure-pp-cli compound pipeline "cybersecurity SDVOSB"
```

Example output:
```
SOURCE       TYPE        TITLE                                     CLOSE/AWARD     AMOUNT
grants.gov   grant       Cybersecurity Workforce Dev (84.215G)     2026-06-15      $2.5M
SAM.gov      contract    SDVOSB Cyber Services IDIQ (N0001426R...)  2026-05-22      $4.0M
SAM.gov      contract    Cyber Defense SOC Support (W15QKN-26...)   2026-06-01      $800K
USASpending  award       Cyber Assessment Services (DISA, FY25)    (closed)        $1.2M

4 results across 3 sources. Synced 2026-05-08 03:14 PT.
```

### `compound stale`

Identifies open grant and contract opportunities that have no matching award history in USASpending — a signal of low competition or a new program area.

```bash
govprocure-pp-cli compound stale --agent
```

### `compound profile`

Pulls an agency's complete procurement picture: active opportunities, grant history, and historical award spend by NAICS.

```bash
govprocure-pp-cli compound profile "Dept of Education"
```

---

## Agent Usage

`govprocure-pp-cli` is designed to work as a subcommand in AI agent loops. Key behaviors:

- **`--agent` flag**: enables machine-readable JSON output on stdout, structured errors on stderr
- **`--compact` flag**: strips descriptions and metadata, returns only decision-relevant fields (60–80% fewer tokens)
- **`--select FIELDS`**: comma-separated field projection, e.g. `--select id,title,close_date,amount`
- **Typed exit codes**: 0=success, 2=usage error, 3=not found, 4=auth error, 5=API error, 7=rate limited

Example agent usage with Claude Code:

```bash
# Check health before any agent workflow
govprocure-pp-cli doctor

# Compact search for agent loop
govprocure-pp-cli grants search "literacy small business" --agent --compact

# Cross-source pipeline with minimal output
govprocure-pp-cli compound pipeline "AI consulting SDVOSB" --agent --compact \
  --select id,title,close_date,amount,source
```

### MCP Server Setup

To connect govprocure-pp-cli to Claude Code as an MCP server, add to your `settings.json`:

```json
{
  "mcpServers": {
    "govprocure": {
      "command": "govprocure-pp-cli",
      "args": ["mcp-server"]
    }
  }
}
```

Available MCP tools:
- `govprocure_grants_search` — search grants.gov (local or live)
- `govprocure_sam_search` — search SAM.gov opportunities
- `govprocure_sam_setasides` — filter by set-aside type
- `govprocure_usaspending_awards` — query historical awards
- `govprocure_compound_pipeline` — cross-source unified search
- `govprocure_doctor` — check API health before a research session

---

## Government Readiness Badge

Opportunities returned by `compound pipeline` are scored on a simple Government Readiness scale:

| Score | Label | Criteria |
|---|---|---|
| A | Ready | Set-aside match + open deadline + award history in NAICS |
| B | Promising | Set-aside match or keyword match, deadline > 14 days |
| C | Research | Keyword match only, or historical data only |
| D | Stale | Deadline < 7 days or no matching set-aside |

Use `--badge` flag to include the score column in any output.

---

## Data Freshness

Data quality matters for procurement — missing a deadline has no recovery.

| Source | Recommended Sync | Reason |
|---|---|---|
| grants.gov | Daily | Deadlines are firm; missing a day = missed opportunity |
| SAM.gov | Daily | Some contracts close within 3–7 days of posting |
| USASpending.gov | Weekly | Historical data only; agencies have 30–90 day reporting lag |

The `sync_log` table in the local DB tracks the last successful sync per source. `doctor` reads this and warns when data is stale:

```
[WARN] grants last synced 3 days ago — run: govprocure-pp-cli sync --source grants
[OK]   sam last synced 18 hours ago
[OK]   usaspending last synced 5 days ago
```

Recommended cron (Linux/Mac):
```cron
0 3 * * * govprocure-pp-cli sync --source grants --source sam
0 2 * * 0 govprocure-pp-cli sync --source usaspending
```

---

## Contributing

This CLI is part of the [Printing Press library](https://github.com/mvanhorn/printing-press) — a curated collection of Go CLIs for real-world data sources.

- See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup and PR guidelines
- New data sources should follow the `internal/source` interface
- FTS5 schema changes require a migration in `internal/db/migrate.go`
- All commands must support `--agent`, `--compact`, and typed exit codes

---

## License

MIT — see [LICENSE](LICENSE)

Printed by [@silentnw](https://github.com/silentnw) for the Printing Press ecosystem.
