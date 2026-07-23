---
name: pp-qbo
description: Fixture-first, read-only QBO CLI for agents.
author: Jeff DeBolt
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/accounting/qbo/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# pp-qbo

Use `qbo-pp-cli` to inspect local fixture data for the QBO Printing Press CLI candidate.

This candidate is read-only and fixture-only. Do not use it for live OAuth or accounting mutations.

## Examples

```bash
qbo-pp-cli status
qbo-pp-cli accounts list --fixture testdata/fixtures/qbo/accounts.json
qbo-pp-cli reports trial-balance --fixture testdata/fixtures/qbo/trial_balance.json
```
