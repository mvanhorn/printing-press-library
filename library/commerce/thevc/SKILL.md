---
name: thevc
description: Korean startup investment database from theVC.kr (더브이씨). Company profiles, funding rounds, investor tracking, rankings, and news. No API key required.
metadata:
  openclaw:
    writablePaths: ["~/.local/share/thevc-pp-cli/"]
    security: "reads theVC.kr public API, no auth required"
---

# thevc — Korean Startup Data CLI

Query Korea's leading startup investment database from the terminal.

## Quick Start

\`\`\`bash
# Install
npx @mvanhorn/printing-press install thevc

# Sync all company data locally
thevc-pp-cli sync

# Search for startups
thevc-pp-cli interaction --kind ALL --json | jq '.results.DATE[] | {name, profilePage}'

# Get full company profile
thevc-pp-cli information get-organization-profile rebellions --json | jq '.results'

# Run SQL on local data
thevc-pp-cli sql "SELECT name, org_type, founded_date FROM companies ORDER BY founded_date DESC LIMIT 10"
\`\`\`

## Agent Usage

Use the MCP server for Claude Code integration:

\`\`\`json
{
  "mcpServers": {
    "thevc": {
      "command": "thevc-pp-mcp"
    }
  }
}
\`\`\`

## API

All endpoints are public — no authentication, no API key, no Cloudflare.

- Rankings: \`GET /api/interaction/hits/organizations/rankings/{ALL|STARTUP}\`
- Profiles: \`GET /api/information/organizations/profiles/{slug}\`
- News: \`GET /api/information/organizations/profiles/{slug}/news-article/items\`

## About theVC.kr

더브이씨 (THE VC) is Korea's leading startup investment database. Tracks funding rounds, investors, products, employee counts, patents, and financial data. Founded in 2013, it covers the entire Korean startup ecosystem.
