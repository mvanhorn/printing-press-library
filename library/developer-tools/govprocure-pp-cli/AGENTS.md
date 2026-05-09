# govprocure-pp-cli — Agent Operating Guide

This document is for AI agents (Claude Code, Stephanie, MCP clients) consuming govprocure-pp-cli as a tool. If you are a human developer, see README.md.

---

## Quick Reference

| Item | Value |
|---|---|
| Binary | `govprocure-pp-cli` |
| Config (Linux/Mac) | `~/.config/govprocure-pp-cli/config.toml` |
| Config (Windows) | `%APPDATA%\govprocure-pp-cli\config.toml` |
| Database (Linux/Mac) | `~/.local/share/govprocure-pp-cli/data.db` |
| Database (Windows) | `%LOCALAPPDATA%\govprocure-pp-cli\data.db` |
| Tables | `grants`, `sam_opportunities`, `awards`, `sync_log` |
| FTS5 tables | `grants_fts`, `sam_fts`, `awards_fts` |

**Always run `doctor` first in a new session.** It checks API reachability, SAM.gov auth, and DB freshness in one call.

---

## Exit Codes

| Code | Meaning | Agent Action |
|---|---|---|
| 0 | Success | Proceed |
| 2 | Usage error (bad flags/args) | Fix command syntax |
| 3 | Not found | Query returned 0 results — try broader terms |
| 4 | Auth error | Run `auth set-key --sam KEY` or check key validity |
| 5 | API error | Log error, retry once after 10s, then escalate |
| 7 | Rate limited | Wait 60 seconds before retry |

---

## Agent Workflow — Procurement Research

Follow this sequence for reliable results. Skipping steps wastes API quota and may return stale data.

```bash
# Step 1: verify state (fast, no API calls if DB is fresh)
govprocure-pp-cli doctor

# Step 2: sync if DB is stale (check sync_log output from doctor)
govprocure-pp-cli sync --all

# Step 3: search locally (fast, zero API calls)
govprocure-pp-cli grants search "your query" --agent --compact

# Step 4: cross-reference all three sources
govprocure-pp-cli compound pipeline "your query" --agent

# Step 5: check SDVOSB opportunities specifically
govprocure-pp-cli sam set-asides --type SDVOSB --agent --compact
```

---

## Token Optimization

Token cost is the primary constraint in agent loops. Apply these rules in order:

**Rule 1: Always use `--compact`**
Strips long descriptions, removes metadata fields not needed for decisions, collapses nested objects. Typical savings: 60–80% fewer tokens.

```bash
govprocure-pp-cli grants search "literacy" --agent --compact
```

**Rule 2: Use `--select` for field projection**
When you only need specific fields, request only those. The DB projection happens before serialization.

```bash
govprocure-pp-cli grants search "AI" --agent --compact \
  --select id,title,close_date,amount,cfda_number
```

**Rule 3: Filter at the CLI, not in your prompt**
Use `jq` or CLI flags to pre-filter before passing results to a reasoning step.

```bash
# Only grants closing after June 1
govprocure-pp-cli grants search "cybersecurity" --agent \
  | jq '.[] | select(.close_date > "2026-06-01")'

# Only results above $500K
govprocure-pp-cli compound pipeline "AI consulting" --agent \
  | jq '.[] | select(.amount > 500000)'
```

**Rule 4: Use `--data-source local` to skip all API calls**
When you know the DB is fresh, skip the API entirely.

```bash
govprocure-pp-cli grants search "workforce" --data-source local --agent --compact
```

---

## Rate Limit Handling

| Source | Limit (no key) | Limit (with key) | Reset window |
|---|---|---|---|
| grants.gov | 25 req/min | 25 req/min | 60 seconds |
| SAM.gov | 1 req/sec | 100 req/min | 60 seconds |
| USASpending.gov | 60 req/min | 60 req/min | 60 seconds |

Exit code 7 = rate limited. **Do not retry immediately.** Wait 60 seconds.

For large syncs, control throughput:
```bash
govprocure-pp-cli sync --source sam --batch-size 25 --delay-ms 500
```

SAM.gov without a key is effectively unusable for bulk operations (1 req/sec = 60 records/min). Get the free key at [api.sam.gov](https://api.sam.gov) before running any SAM sync.

---

## Common Agent Patterns

### SDVOSB opportunities closing this month

```bash
govprocure-pp-cli sam set-asides --type SDVOSB --compact --agent \
  | jq '.[] | select(.close_date <= "2026-05-31")'
```

### Full cross-source pipeline for a query

```bash
govprocure-pp-cli compound pipeline "AI machine learning federal" --agent --compact
```

### Agency award history

```bash
govprocure-pp-cli usaspending awards --agency "Dept of Education" \
  --agent --compact --select award_id,recipient,amount,naics,fiscal_year
```

### Stale grant detection (low competition signal)

Grants that are open but have no recent award history in USASpending suggest either a new program or low industry awareness — both mean less competition.

```bash
govprocure-pp-cli compound stale --agent --compact
```

### Agency procurement profile

Before pursuing any opportunity, pull the agency profile to understand their award patterns and active pipeline:

```bash
govprocure-pp-cli compound profile "Dept of Veterans Affairs" --agent
```

### Check DB sync freshness without running a sync

```bash
govprocure-pp-cli doctor --agent | jq '.sync_log'
```

### Export to CSV for external processing

```bash
govprocure-pp-cli grants list --status open --export csv --output /tmp/grants.csv
```

---

## MCP Tool Names

When govprocure-pp-cli is running as an MCP server (`govprocure-pp-cli mcp-server`), the following tools are exposed:

| MCP Tool Name | CLI Equivalent | Notes |
|---|---|---|
| `govprocure_grants_search` | `grants search "QUERY"` | Accepts `query`, `compact`, `select` params |
| `govprocure_sam_search` | `sam search "QUERY"` | Accepts `query`, `naics`, `agency` params |
| `govprocure_sam_setasides` | `sam set-asides --type X` | Accepts `type`: SDVOSB, 8A, WOSB, HUBZone, SB |
| `govprocure_usaspending_awards` | `usaspending awards` | Accepts `agency`, `naics`, `recipient`, `keyword` |
| `govprocure_compound_pipeline` | `compound pipeline "QUERY"` | Joins all three sources |
| `govprocure_doctor` | `doctor` | Run before any research session |

All MCP tools return JSON. The `compact` parameter maps to `--compact`. All tools respect the local DB when available.

---

## Error Patterns to Watch

| Symptom | Likely Cause | Fix |
|---|---|---|
| `exit 4` on any SAM command | No API key set | `govprocure-pp-cli auth set-key --sam KEY` |
| `exit 7` during sync | SAM rate limit hit | Add `--batch-size 25 --delay-ms 1000` |
| `exit 3` on known opportunities | DB is stale | `govprocure-pp-cli sync --source sam` |
| Doctor reports grants > 2 days stale | Sync job missed | Run manual sync; check cron if automated |
| Empty compound results | All three sources return 0 | Try broader query terms; check each source individually |
| FTS5 tokenization mismatch | Query too specific | Remove quotes; try root terms (e.g. "literacy" not "literacy program support") |

---

## Notes for Stephanie (Help Desk Agent)

- Use `govprocure_doctor` before any procurement research session to confirm API health
- For SDVOSB queries, always check both `govprocure_sam_setasides` (active) and `govprocure_usaspending_awards` (historical SDVOSB awards) — the compound pipeline does this automatically
- Grant deadlines are Eastern Time (federal standard). The `close_date` field is always stored as ET
- USASpending data lags 30–90 days from agencies. "No award history" does not mean no awards exist — it means no reported awards in the current dataset window
- Always include the sync timestamp in any procurement research output so the user knows data age
