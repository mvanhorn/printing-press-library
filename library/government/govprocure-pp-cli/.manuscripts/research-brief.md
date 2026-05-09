# Research Brief — govprocure-pp-cli

## Problem

Small businesses and veteran-owned firms pursuing U.S. federal contracts and grants have no unified offline tool for searching across the three primary government procurement data sources:

- **grants.gov** — federal grant opportunities (~20,000 active at any time)
- **SAM.gov** — contract solicitations, set-asides, entity registration (~50,000 active notices)
- **USASpending.gov** — historical award data (~$6T in tracked spending)

Existing solutions require:
- Browser-only interfaces (no CLI, no automation)
- Separate accounts and sessions per API
- Internet connection for every search
- No cross-source compound queries ("find grants closing this week at agencies that awarded contracts to firms like mine")

## Solution

`govprocure-pp-cli` mirrors all three sources into a local SQLite database with FTS5 full-text search. Once synced, all queries run offline in milliseconds.

## Key Capabilities

- **`govprocure sync --all`** — pulls latest records from all three APIs into local SQLite
- **`govprocure grants search <query>`** — FTS5 search across grants, filtered by eligibility, agency, deadline
- **`govprocure sam search <query>`** — FTS5 search across SAM notices, filtered by NAICS code, set-aside type
- **`govprocure compound pipeline <query>`** — cross-source intelligence: grants + SAM + award history in one query
- **`govprocure mcp`** — MCP stdio server with 9 tools for agent integration (Claude Desktop, etc.)
- **`govprocure doctor`** — validates all API connections and local DB health

## Target Users

- Small business owners pursuing federal contracts
- Service-Disabled Veteran-Owned Small Businesses (SDVOSB)
- Grant writers at nonprofits and research institutions
- Government procurement consultants
- AI agents that need structured federal procurement data

## Technical Choices

- **Go** — single binary, cross-platform, no runtime dependencies
- **SQLite + FTS5** — offline-capable, fast, zero infrastructure
- **MCP stdio transport** — agent-native, works with Claude Desktop and any MCP client
- **MIT license** — open source, forkable, commercially usable

## Build Verification

```
go build ./...   ✅
go vet ./...     ✅
doctor all-green ✅ (grants.gov + USASpending.gov; SAM.gov requires free API key)
```

## Author

Don Alexander, Silent Northwest LLC — Service-Disabled Veteran-Owned Small Business (SDVOSB), Arlington WA.
