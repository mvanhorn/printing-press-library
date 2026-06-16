# Ads Intel Skill

Use `ads-intel-pp-cli` when an agent needs read-only paid media audit context across Google Ads, Meta Ads, and Amazon Ads.

## Rules

- Phase 4 is read-only. Do not mutate ad accounts.
- Prefer `--agent` for JSON, compact, no-input output.
- Run `sync` before reports if local data is missing.
- Run `confidence` before trusting audit recommendations.
- Treat `negative_keyword_drafts` as local draft artifacts only.
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
