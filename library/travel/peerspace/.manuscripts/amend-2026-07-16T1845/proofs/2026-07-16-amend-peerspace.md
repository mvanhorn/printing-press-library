---
date: 2026-07-16
target_cli: peerspace-pp-cli
amend_run_id: amend-2026-07-16T1845
scope_tier: bugs+features
findings_count: 2
published_status: local-only
---

# Amend plan: peerspace favorites write path

## Findings active

### F1 — create favorite board (`add-command`, feature, user-ask)
- Evidence: "creating a 'favorite board'" + HAR POST /v1/projects/attachments
- HAR body: `{"ns":"FAV_BOARD","value":"<listing_id>","project":{"name","activity","location"}}`
- Target: `shortlist create-board` (+ docs/which/MCP catalog)
- Expected: cookie-auth POST creates board and attaches listing; returns attachment+project JSON

### F2 — favorite a space onto a board (`add-command`, feature, user-ask)
- Evidence: "favoriting a space" + HAR listing-saved analytics + attachment schema (`project_id`,`value`,`ns`)
- Body (inferred for existing board): `{"ns":"FAV_BOARD","value":"<listing_id>","project_id":"<board_id>"}`
- Target: `shortlist add`
- Expected: cookie-auth POST attaches listing to existing board

## Risks
- F2 body shape for existing boards was not captured as a separate POST in HAR (only create-board+save). If live API rejects `project_id`, follow-up HAR needed.
- create-board in browser always includes a listing id; require `--listing-id`.

## Tests
- Help wiring smoke for both commands
- Payload builder unit tests for create vs add bodies
