# SuppCo Printed CLI Agent Guide

This directory is a generated `suppco-pp-cli` PrintingPress package with a deliberately narrow handwritten provider layer. Systemic framework fixes belong upstream; SuppCo-specific changes must stay small and be recorded in `.printing-press-patches/suppco-provider.md`.

## Runtime contract

The supported CLI reads are:

```bash
suppco-pp-cli stack products
suppco-pp-cli stack nutrients
suppco-pp-cli schedule YYYY-MM-DD
suppco-pp-cli regimen snapshot YYYY-MM-DD
```

These commands emit the complete normalized JSON contract. `--agent` is accepted for non-interactive agent use, but alternate compact, CSV, plain, quiet, selected-field, and rich-human output modes are intentionally unavailable.

The MCP binary uses stdio and exposes exactly `stack_products`, `stack_nutrients`, `schedule_show`, `regimen_snapshot`, and `context`. The CLI and MCP handlers share `internal/provider` extraction and `internal/regimen` normalization.

Nutrient components are flattened from `products[].ingredients`; the final non-empty `ancestry` segment is the immediate parent. The top-level aggregate nutrient view is not an exposure total and is not emitted. Provider schedules use minimized `activities` with scheduled products and reminder state, sorted by documented stable keys without cadence inference.

## Authentication and privacy

Authentication is a user-supplied bearer token. A periodically replaced token is an accepted operating assumption; this package does not implement OAuth refresh or browser automation.

```bash
printf '%s' "$SUPPCO_ACCESS_TOKEN" | suppco-pp-cli auth set-token
suppco-pp-cli auth status
suppco-pp-cli doctor
```

Never place tokens in argv, examples, logs, fixtures, screenshots, reports, or Git. Never persist or print raw SuppCo responses. Synthetic fixtures are the only committed provider data.

## Ownership boundary

This package is read-only. Do not add intake logging, schedule edits, product mutations, sync storage, output delivery, clinical interpretation, exposure calculation, coaching semantics, or Trainer Core behavior. Provider schedule is preserved as provider fact; downstream software owns any manual override.

Before unfamiliar work, read `README.md`, `SKILL.md`, and `.printing-press-patches/suppco-provider.md`. Run focused tests, `go test ./...`, `go vet ./...`, `go build ./...`, PrintingPress shipcheck, privacy scans, and an exact diff against a fresh print before packaging.

## Release ledger

`CHANGELOG.md` and `.printing-press-release.json` are stamped only by the public library workflow after merge. Do not hand-bump release versions.
