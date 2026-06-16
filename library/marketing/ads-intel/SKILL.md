# Ads Intel Skill

Use `ads-intel-pp-cli` when an agent needs paid media audit context across Google Ads, Meta Ads, and Amazon Ads. It is read-only by default; the only live mutation is the hardened Google Ads exact-match negative keyword apply path.

## Rules

- Default to dry-run. Do not run a live apply unless the human supplies `--live-approved` and the exact typed `--confirm` string printed by the command contract.
- Prefer `--agent` for JSON, compact, no-input output.
- Run `sync` before reports if local data is missing.
- Run `confidence` before trusting audit recommendations.
- Treat `negative_keyword_drafts` as local draft artifacts until `apply negative-keyword` dry-runs them.
- `apply negative-keyword` supports only Google Ads exact-match negatives from confirmed wasters: spend >$10 and 0 conversions.
- Live apply requires an account allowlist (`--allow-account` or apply policy), max-changes cap, idempotency check, advisory lock, pre-write snapshot, read-after-write verification, reversal registry, and append-only audit log.
- `apply undo` is best-effort only, not a guaranteed rollback; negative keywords can change auction eligibility.
- Never recommend edits while a campaign is in active learning.

## Commands

- `agent-context` — machine-readable context and source plan.
- `doctor` — local readiness.
- `sources doctor` — child CLI availability and read-capability notes.
- `profile save/list` — local profile metadata.
- `sync` — embedded fixture or local DataSet import.
- `account-status` — account/CID, status, date range, tracking confidence, mode.
- `confidence` — tracking confidence checks.
- `audit` — deterministic structural audit from the check catalog.
- `quick-wins` — high/critical findings that take under 15 minutes.
- `budget-shift` — read-only cross-channel spend/revenue view.
- `apply negative-keyword --draft <path> --allow-account <cid>` — dry-run the exact Google Ads negative keyword mutation and inverse op; add `--live-approved --confirm "APPLY GOOGLE ADS EXACT NEGATIVES <cid>"` only after human approval.
- `apply undo --reversal-id <id> --allow-account <cid>` — dry-run a recorded best-effort inverse op; live undo requires `--live-approved --confirm "UNDO GOOGLE ADS EXACT NEGATIVE <cid>"`.
