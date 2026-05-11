---
name: pp-thevc
description: Korean startup investment database from theVC.kr (더브이씨). Company profiles, funding rounds, investor tracking, rankings, and news. No API key required.
---

# pp-thevc — Korean Startup Data

Query Korea's leading startup investment database from the terminal. No API key required — all endpoints are public.

## Commands

```bash
# Get startup rankings by popularity
thevc-pp-cli interaction --kind ALL --json

# Get full company profile
thevc-pp-cli information get-organization-profile rebellions --json

# Sync all data to local SQLite
thevc-pp-cli sync

# Run ad-hoc SQL analysis
thevc-pp-cli sql "SELECT name, org_type, founded_date FROM companies ORDER BY founded_date DESC"
```

## Agent Usage

```bash
# Agent-friendly output (--agent = --json --compact --no-input --no-color --yes)
thevc-pp-cli interaction --agent

# Script integration
thevc-pp-cli information get-organization-profile rebellions --json | jq '.results.name'
```

## About theVC.kr

더브이씨 (THE VC) is Korea's leading startup investment database. Tracks funding rounds, investors, products, employee counts, patents, and financial data for the Korean startup ecosystem.
