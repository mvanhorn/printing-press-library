---
name: pp-qbo
description: Fixture-first, read-only QBO CLI for agents.
author: Jeff DeBolt
---

# pp-qbo

Use `qbo-pp-cli` to inspect local fixture data for the QBO Printing Press CLI candidate.

This candidate is read-only and fixture-only. Do not use it for live OAuth or accounting mutations.

## Examples

```bash
qbo-pp-cli status
qbo-pp-cli accounts list --fixture testdata/fixtures/qbo/accounts.json
qbo-pp-cli reports trial-balance --fixture testdata/fixtures/qbo/trial_balance.json
```
