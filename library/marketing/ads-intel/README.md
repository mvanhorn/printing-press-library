# ads-intel-pp-cli

Read-only paid media intelligence CLI for local-first Google Ads, Meta Ads, and Amazon Ads auditing.

Phase 4 is deliberately read-only. It syncs local fixture or imported JSON, preserves provenance snapshots, reports tracking confidence first, runs a deterministic structural audit from a versioned JSON check catalog, writes negative-keyword candidates as local draft artifacts only, and provides a read-only cross-channel budget-shift view.

## Usage

```bash
ads-intel-pp-cli --agent agent-context
ads-intel-pp-cli sync --profile account
ads-intel-pp-cli account-status --profile account
ads-intel-pp-cli confidence --profile account
ads-intel-pp-cli audit --profile account
ads-intel-pp-cli quick-wins --profile account
ads-intel-pp-cli budget-shift --profile account
```

Every output includes an account-status header with account/CID, active/suspended/dormant status, date range used, tracking confidence, and `read-only` mode.

## Child CLI posture

`sources doctor` inspects `google-ads-pp-cli`, `meta-ads-pp-cli`, and `amazon-ads-pp-cli` availability and records the read-oriented commands ads-intel can consume later. Presence is not treated as mutation capability. Phase 4 does not call write commands and never mutates ad accounts.

## Audit rules

The audit catalog is data in `internal/cli/audit_catalog.json`. It defines stable check IDs, severity multipliers, platform category weights, and PASS/WARN/FAIL bands. Tests assert catalog weight coverage and deterministic scoring.

Implemented heuristics include wasted spend >$10 with 0 conversions, zero-conversion keyword detection, legacy BMM broad+manual-CPC exclusion, brand classification support through keyword text, shared negative-list counting, Meta fatigue, and active-learning edit discipline.
