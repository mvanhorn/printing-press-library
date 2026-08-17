# intercom-pp-cli reprint build log (press 4.29.0)

Manifest transcendence rows: 4 planned, 4 built. Phase 3 gate PASS (dogfood novel_features_check planned=4 found=4).

## Built (ported from prior published 4.14.0 CLI, proven code)
- conversations incident-tag (dry-run guarded bulk-tag; live search)
- conversations sla (SQL over local mirror; ported AS-IS, statistics.* reframe deferred)
- contact 360 (mirror cross-entity join; PATCH contact-360-sql-pushdown)
- articles pull / articles push (help-center git round-trip)
- calls/* subtree (hand-built, 2.13 spec lacks /calls; PATCH intercom-calls-v2-14)

## Inline patches re-applied to generated files
- intercom-region (config.go/root.go/doctor.go): --region us|eu|au + INTERCOM_REGION -> regional base URL. PRIORITY. Verified: --region eu -> api.eu.intercom.io.
- intercom-version (client.go): pin Intercom-Version: 2.13.
- json-string-control-bytes (helpers.go).
- intercom-search-query-json-parse: DEST already parses JSON --query natively; marker-only.

## Generator wins (4.29.0)
- Cloudflare MCP pattern auto-applied (133 endpoints > 50): code orchestration, endpoint_tools hidden, transport [stdio,http].
- Self-learning loop default-on (seedless: Intercom entity vocab is per-workspace).

## Known follow-ups (for fix loop / polish)
- dead code: ticketHasContact in contact_360.go unused (ported verbatim; SQL pushdown made it redundant).
- SLA statistics.* reframe deferred (proven conversation_parts logic kept).
