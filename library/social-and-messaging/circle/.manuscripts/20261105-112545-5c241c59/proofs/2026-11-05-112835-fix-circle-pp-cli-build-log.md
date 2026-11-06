<!-- slop-gate: off -->
Manifest transcendence rows: 9 planned, 9 built. Phase 3 will not pass until all 9 ship.

# circle-pp-cli build log

Code: every novel command diff was reviewed, and the run included tests plus live checks against a live Circle community on pp-test- objects.

## Built (task, commit, live proof)
- A seats + founding: payer dedupe across subscriptions and charges, typed exit 0/2. Live: 0 paid, NO-GO exit 2. Fix round: --db default resolved at runtime, partial_refunded counts, hint once.
- B inbox: local-only triage, 6 sections + counts. Live against synced mirror: 1 new member, rejects --data-source live.
- C onboard: preview default, --yes applies, skip_invitation default. Live: tag + member created, rerun sees existing, space membership added (proofs/onboard-lifecycle.md). Fixes: member_tags needs display_format; search needs status=all for inactive members.
- D spaces apply: plan/--apply/--prune-report. Live: create, rerun noop, one-field update, read-back (proofs/spaces-apply-lifecycle.md). Fixes: POST /spaces wraps id under "space"; ids decoded as float printed 2.94e+06.
- E course push/pull: folders + lesson markdown, media via direct_uploads, reorder. Live: 2 sections/3 lessons/1 upload, rerun noop, pulled folder pushes noop, one-field edit (proofs/course-push-pull-lifecycle.md). Fixes: Circle normalizes body_html; API never exposes lesson order, so push records the order it sent.
- F show-tell schedule/recap: duplicate + reschedule as quiet drafts; recap from event, attendees, live room transcript. Live: 4 draft occurrences, rerun all "exists" (proofs/show-tell-lifecycle.md). Fixes: PUT /events needs top-level space_id; drafts never listed so a local ledger tracks created ids; POST /posts wraps id under "post".
- sync/search/analytics: framework. Raw SQL is the MCP `sql` tool (no CLI `sql` command exists).

## Spec overlay (research artifact, not code)
servers -> https://app.circle.so/api/admin/v2 (path prefix moved); dropped fake `host` security scheme; x-auth-value-prefix Token;
x-auth-env-vars CIRCLE_API_TOKEN; collapsed community_setting, event_setting_attributes, attendee_invite_attributes to JSON objects
(flattened MCP keys > 64 chars hard-failed generate); /spaces path-level x-resource-id: id (spaces was missing from default sync);
x-rate-class monthly (Admin API monthly quota); x-learn seeds.

## Deferred
- spaces pull (survived the brainstorm cut, not on the approved list).
- recap post body renders markdown as one paragraph: pending fix.

## Press findings (retro candidates)
See scratch retro list: 64-char MCP key hard-fail on deep body flattening; operation-level x-resource-id ignored silently; image-posts
dependent 404 on non-image spaces; no OpenAPI default-sync opt-out; empty records page warns "no extractable ID"; concurrent runs wipe
the shared isolated GOCACHE; macOS /private/var test failure on origin/main.
