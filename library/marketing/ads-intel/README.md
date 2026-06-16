# ads-intel-pp-cli

Paid media intelligence CLI for local-first Google Ads, Meta Ads, and Amazon Ads auditing.

The CLI is read-only by default. It syncs local fixture or imported JSON, preserves provenance snapshots, reports tracking confidence first, runs a deterministic structural audit from a versioned JSON check catalog, writes negative-keyword candidates as local draft artifacts, and provides a read-only cross-channel budget-shift view.

Phase 5 adds one narrow live mutation: Google Ads exact-match negative keyword adds from confirmed wasted-spend drafts only. Live apply is blocked unless the account is allowlisted, the run fits the max-change cap, confidence is not Low/Broken, the draft proves spend >$10 with 0 conversions, `--live-approved` is present, and the typed confirmation matches the target account.

## Usage

```bash
ads-intel-pp-cli --agent agent-context
ads-intel-pp-cli sync --profile account
ads-intel-pp-cli account-status --profile account
ads-intel-pp-cli confidence --profile account
ads-intel-pp-cli audit --profile account
ads-intel-pp-cli quick-wins --profile account
ads-intel-pp-cli budget-shift --profile account
ads-intel-pp-cli apply negative-keyword --profile account --draft ~/.ads-intel-pp-cli/drafts/account/google_negative_keyword_candidate.json --allow-account <google-customer-id>
```

Every report output includes an account-status header with account/CID, active/suspended/dormant status, date range used, tracking confidence, and mode.

## Apply safety model

`apply negative-keyword` defaults to dry-run and prints the resolved Google Ads target, exact create operation, inverse operation, snapshot path, and best-effort undo warning without writing to the ad account. It prefers ad-group-scoped negatives when an ad group is available, then campaign scope.

Live execution requires all of:

- `--allow-account <cid>` or `apply-policy.json` containing `allowed_google_ads_accounts`.
- `--max-changes-per-run` within the batch size.
- confidence level Medium or High.
- `--live-approved`.
- `--confirm "APPLY GOOGLE ADS EXACT NEGATIVES <cid>"`.

The live path takes an advisory account lock, snapshots current negatives before writing, skips existing exact negatives idempotently, mutates one operation at a time through the Google Ads child CLI, re-queries to verify the negative exists, stores a best-effort reversal record, and appends successful live ops to `audit/apply.log`.

The platform-agnostic safety controls are implemented through the shared `library/internal/intelcli` apply core so later intel CLIs reuse the same audited dry-run, live-approval, lock, snapshot, reversal, and audit primitives.

`apply undo --reversal-id <id>` dry-runs the recorded inverse by default. Live undo uses the same allowlist and typed-confirm pattern, and remains best-effort rather than a clean rollback.

## Child CLI posture

`sources doctor` inspects `google-ads-pp-cli`, `meta-ads-pp-cli`, and `amazon-ads-pp-cli` availability. Only the Google Ads child CLI campaign/ad-group criteria mutate and GAQL search surfaces are used for Phase 5 apply; pause, budget, creative, Shopify, Meta, and Amazon writes remain out of scope.

## Audit rules

The audit catalog is data in `internal/cli/audit_catalog.json`. It defines stable check IDs, severity multipliers, platform category weights, and PASS/WARN/FAIL bands. Tests assert catalog weight coverage and deterministic scoring.

Implemented heuristics include wasted spend >$10 with 0 conversions, zero-conversion keyword detection, legacy BMM broad+manual-CPC exclusion, brand classification support through keyword text, shared negative-list counting, Meta fatigue, and active-learning edit discipline.
