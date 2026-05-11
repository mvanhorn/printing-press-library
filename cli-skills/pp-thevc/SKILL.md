---
name: pp-thevc
description: "Korean startup investment database from theVC.kr (더브이씨). Company profiles, funding rounds, investor tracking, rankings, and news from Korea's leading startup data platform. No API key required. Trigger phrases: `Korean startup`, `look up Korean company`, `search thevc`, `run thevc-pp-cli`, `theVC.kr`, `한국 스타트업`."
author: "Allen Lee"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - thevc-pp-cli
    install:
      - kind: go
        bins: [thevc-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/commerce/thevc/cmd/thevc-pp-cli
---

# THE VC — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `thevc-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install thevc --cli-only
   ```
2. Verify: `thevc-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

## Core Workflows

### Get Startup Rankings
```
thevc-pp-cli interaction --kind ALL --json | jq '.results.DATE[] | {name, profilePage}'
```

### Get Company Profile
```
thevc-pp-cli information get-organization-profile rebellions --json | jq '.results'
```

### Sync All Data to Local SQLite
```
thevc-pp-cli sync
thevc-pp-cli sql "SELECT name, org_type, founded_date FROM companies ORDER BY founded_date DESC"
```

### Agent-Friendly Mode
Use `--agent` flag for JSON + compact + non-interactive output:
```
thevc-pp-cli interaction --agent
```

## About THE VC
더브이씨 (THE VC) is Korea's leading startup investment database. Tracks funding rounds, investors, products, employee counts, patents, and financial data. Public API — no authentication required.
