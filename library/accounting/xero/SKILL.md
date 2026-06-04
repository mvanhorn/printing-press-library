---
name: pp-xero
description: Fixture-first, read-only XERO CLI for agents.
author: Jeff DeBolt
---

# pp-xero

Use `xero-pp-cli` to inspect local fixture data for the XERO Printing Press CLI candidate.

This candidate is read-only and fixture-only. Do not use it for live OAuth or accounting mutations.

## Examples

```bash
xero-pp-cli status
xero-pp-cli accounts list --fixture testdata/fixtures/xero/accounts.json
xero-pp-cli reports trial-balance --fixture testdata/fixtures/xero/trial_balance.json
```
