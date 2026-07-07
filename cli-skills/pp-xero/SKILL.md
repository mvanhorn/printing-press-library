---
name: pp-xero
description: Fixture-first, read-only XERO CLI for agents.
author: Jeff DeBolt
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/accounting/xero/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# pp-xero

Use `xero-pp-cli` to inspect local fixture data for the XERO Printing Press CLI candidate.

This candidate is read-only and fixture-only. Do not use it for live OAuth or accounting mutations.

## Examples

```bash
xero-pp-cli status
xero-pp-cli accounts list --fixture testdata/fixtures/xero/accounts.json
xero-pp-cli reports trial-balance --fixture testdata/fixtures/xero/trial_balance.json
```
