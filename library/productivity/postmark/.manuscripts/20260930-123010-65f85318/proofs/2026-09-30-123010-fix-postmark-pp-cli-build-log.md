Manifest transcendence rows: 5 planned, 5 built. Phase 3 will not pass until all 5 ship.
<!-- slop-gate: off -->

# postmark-pp-cli build log

## Generation
- Spec: research/specs/postmark-merged.openapi.yaml built by research/specs/build_spec.py (official server + account Swagger 2.0 merged to OpenAPI 3.0.3, 21 doc-only ops + opens/readtimes added, drift fixed, clean operationIds and x-pp-resource buckets).
- Auth modeled as an optional AND pair (serverToken + accountToken) so both env vars and config fields are emitted; --auth-preference serverToken.
- Generator applied the Cloudflare MCP pattern (88 endpoints > 50).
- Generator limitation: no per-endpoint credential selection; composed sibling headers go on every request. Live probe: both headers on GET /server returns 401 ErrorCode 10. Printed-CLI routing patch planned (user-approved option A).

## Slices

### Slice A: foundation (done)
Files: internal/cli/postmark_servers.go, internal/cli/postmark_servers_test.go (hand-authored; regen-preserved, backups in handcode-backup/).
- Transport wrapper via registerClientHook: routes X-Postmark-Server-Token vs X-Postmark-Account-Token by path family; masks ApiTokens in /server and /servers* responses unless --show-tokens; adds offset=0 to paged GETs that carry count but no offset (generated offset sync omits the first-page cursor; Postmark requires offset).
- Global --server <name|id> (env POSTMARK_SERVER) resolves the server token through GET /servers with the account token; in-process memo; internal lookups bypass masking and the response cache.
- Global --sandbox swaps in POSTMARK_API_TEST. Global --show-tokens.
- servers use [name] [--from] [--stream] [--show] [--clear]: saved default server + per-server send defaults (servers.json, mode 600); PersistentPreRunE wrapper fills --from/--message-stream when unset.
- Spec fixes found during this slice: single-object responses with one typed array were printed as that array (GET /server printed only ApiTokens, i.e. the server token) -> items stripped from non-envelope arrays; messages regrouped into messages/inbound/opens/clicks with `list`/`get`; stats and bulk list typed so default sync skips them; x-resource-id moved to path items (composite for opens/clicks).
Live evidence (read-only unless noted): account-only servers list 200; server get --server "Main App" 200 (was 401 with both headers); bad name lists valid servers; sandbox send "Test job accepted" (POSTMARK_API_TEST, not delivered); sync Helm: messages 31, servers 6 (tokens masked), domains 3, senders 4, templates 1, streams 3, suppressions 8, zero errors; Staging messages 300 distinct across 3 offset pages; no unmasked tokens in store or HTTP cache.
Tests: go test -run Postmark ./internal/cli PASS.
Retro candidates (Printing Press): (1) singleArrayPropertyRef treats resource objects with one array field as list envelopes; (2) offset-style sync omits offset on the first page; (3) composed sibling apiKey headers sent on every request with no per-operation credential selection; (4) since-param detection misses `fromdate`; (5) novel-host guard rejects placeholder emails in examples.

### Slice B: templates, bounce ops, suppressions check, service status (done, merged)
Files: internal/cli/templates_check.go (+test), slice_b_template_layout.go, slice_b_templates_fs.go, slice_b_bounces.go, slice_b_suppressions_check.go, slice_b_service_status.go (each +_test.go; 38 tests).
Commands: templates pull/push/render/check, bounces reactivate, bounces resend-blocked, suppressions check, service-status. Transcendence row 4 (templates check) built.
Evidence: gate go build/vet/test exit 0 in slice and after merge; live read-only: templates pull wrote official layout; push plan unchanged 1 / writes 0, name edit -> exactly one update; check exit 0 on server, exit 3 on broken local dir; reactivate plan 0 writes (Staging bounces are Transient); suppressions check finds a real HardBounce suppression on outbound; service-status operational (19 components). httptest proves push --yes request set and no writes without --yes; resend-blocked --send refused under harness.
Deviation: service-status uses status.postmarkapp.com/api/v1/{status,components,notices} (documented at status.postmarkapp.com/api); the postmarker 1.0 endpoints return 404.
Generator note: generated .golangci.yml lacks a v2 `version` key.

### Slice C: novel commands, diagnose, overview, health (done, merged)
Files: internal/cli/pulse.go, recipient_domains.go, email_send_once.go, servers_bootstrap.go (+tests), slice_c_shared.go, slice_c_diagnose.go, slice_c_overview.go, slice_c_health.go (+tests), internal/store/postmark_ledger.go (+test).
Transcendence rows built: pulse, email send-once, recipient-domains, servers bootstrap (templates check came from slice B) -> 5/5.
Evidence: gate exit 0; live read-only: pulse 6 rows with window/baseline; overview 6 servers; diagnose delivered/not_found (bounced, spam_complaint, suppressed also observed); recipient-domains 22 domains summing to 411 synced recipients; domains health 3 domains/4 senders; streams health flags spam 0.107% over 120d; send-once --sandbox --send twice -> second duplicate:true from ledger. httptest: webhooks health flags; bootstrap --apply exact 10-request sequence, idempotent rerun, token never in stdout, env file 0600.

### Merge and completion gate
- Merged B and C into working/; restored `[email]` / `[name]` positionals in diagnose/bootstrap Use lines (+ test expectations).
- De-identified account names in examples/tests (server names -> Main App/Staging/Marketing Site/Billing; real server ID -> fake).
- Health commands registered explicitly so dogfood wiring sees them; bootstrap examples use spec-documented hosts; documented hosts recorded in research/specs/documented-urls.txt.
- Manifest command names normalized to the final tree (messages regroup).
- Per-row Cobra resolution: 22/22 PASS. dogfood novel_features_check found 5/5, no stubs, reimplementation clean; remaining issue: 1 dead generated helper (handleBinaryResponseDelivery).
- go test ./... exit 0.
Deferred: none of the approved rows. Generator limitations logged as retro candidates above plus: agent-context ignores x-auth-vars descriptions; integer path examples dropped; host-guard artifact roots expect research-dir/../research.
