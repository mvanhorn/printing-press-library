# govprocure-pp-cli

Go CLI + MCP server for U.S. government procurement intelligence.
Wraps grants.gov, SAM.gov, and USASpending.gov into one binary with SQLite local mirror.

## Stack
- Go 1.22+ with Cobra + Viper
- SQLite via modernc.org/sqlite (pure Go, no CGO)
- FTS5 full-text search across all three data sources
- Three API namespaces + compound cross-source queries

## Build

```bash
go build ./...
go vet ./...
go build -o govprocure-pp-cli.exe .   # Windows
go build -o govprocure-pp-cli .        # Linux/Mac
```

## Config
- Windows: `%APPDATA%\govprocure-pp-cli\config.toml`
- Linux/Mac: `~/.config/govprocure-pp-cli/config.toml`
- SAM API key: `govprocure-pp-cli auth set-key --sam YOUR_KEY`

## Database
- Windows: `%LOCALAPPDATA%\govprocure-pp-cli\data.db`
- Linux/Mac: `~/.local/share/govprocure-pp-cli/data.db`
- Tables: `grants`, `sam_opportunities`, `awards`, `sync_log`
- FTS5 virtual tables: `grants_fts`, `sam_fts`, `awards_fts`

## APIs
- grants.gov: `POST https://apply07.grants.gov/grantsws/rest/opportunities/search/`
- SAM.gov: `GET https://api.sam.gov/opportunities/v2/search` (requires SAM_API_KEY)
- USASpending.gov: `POST https://api.usaspending.gov/api/v2/search/spending_by_award/`

## Key Commands

```bash
govprocure-pp-cli doctor
govprocure-pp-cli auth set-key --sam KEY
govprocure-pp-cli sync --all
govprocure-pp-cli grants search "literacy"
govprocure-pp-cli sam set-asides --type SDVOSB
govprocure-pp-cli compound pipeline "AI consulting small business"
```

## Agent-Native Flags (all commands)

| Flag | Effect |
|---|---|
| `--agent` | Force JSON output to stdout |
| `--compact` | Minimal fields only (60–80% fewer tokens) |
| `--select id,title,close_date` | Field projection |
| `--data-source local` | Skip API calls, use DB only |
| `--dry-run` | Show what would happen, no API calls |
| `--output table\|json\|csv` | Output format override |

## Exit Codes

| Code | Meaning |
|---|---|
| 0 | Success |
| 2 | Usage error |
| 3 | Not found |
| 4 | Auth error (missing/invalid API key) |
| 5 | API error |
| 7 | Rate limited |

## Sync Schedule (Production)
- Grants + SAM: daily at 3 AM Pacific / 6 AM Eastern (government deadlines are ET)
- USASpending: weekly Sunday 2 AM Pacific

Cron example:
```cron
0 3 * * * govprocure-pp-cli sync --source grants --source sam
0 2 * * 0 govprocure-pp-cli sync --source usaspending
```

## MCP Server

Run as MCP server for agent access:
```bash
govprocure-pp-cli mcp-server
```

Add to Claude Code `settings.json`:
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

MCP tools: `govprocure_grants_search`, `govprocure_sam_search`, `govprocure_sam_setasides`,
`govprocure_usaspending_awards`, `govprocure_compound_pipeline`, `govprocure_doctor`

## Status
v1.0.0 — functional, all three APIs live, doctor all-green.
Pending: rate-limit backoff polish, MCP server, library submission.

## Set-Aside Types (SAM.gov)
- `SDVOSB` — Service-Disabled Veteran-Owned Small Business
- `8A` — 8(a) Small Disadvantaged Business
- `WOSB` — Women-Owned Small Business
- `HUBZone` — Historically Underutilized Business Zone
- `SB` — All small business set-asides
