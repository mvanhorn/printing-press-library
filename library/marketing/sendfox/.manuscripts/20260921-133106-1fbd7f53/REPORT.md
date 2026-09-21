# SendFox CLI local upgrade — verified, unpublished

Run: `20260921-133106-1fbd7f53`. Official OpenAPI v1.4.0; installed Printing Press v4.32.4 / skill v3.0.0. Recovered generation by normalizing ten multiline operation descriptions/summaries only in the derived generation spec. Immutable official source SHA-256 remains `e9c1a272f79238e2f94f857092a932002c64f21299304b930e33f5ca2e79b991`.

## Deliverable

- Local library: `/Users/cathrynlavery/printing-press/library/sendfox`
- Working tree: `/Users/cathrynlavery/printing-press/.runstate/printing-press-c52790a1/runs/20260921-133106-1fbd7f53/working/sendfox-pp-cli`
- CLI: `/Users/cathrynlavery/printing-press/library/sendfox/sendfox-pp-cli`
- MCP: `/Users/cathrynlavery/printing-press/library/sendfox/sendfox-pp-mcp`; bundle: `/Users/cathrynlavery/printing-press/library/sendfox/build/sendfox-pp-mcp-darwin-arm64.mcpb`
- Manuscripts: `/Users/cathrynlavery/printing-press/manuscripts/sendfox/20260921-133106-1fbd7f53`
- Proofs and exact phase ledger: `/Users/cathrynlavery/printing-press/.runstate/printing-press-c52790a1/runs/20260921-133106-1fbd7f53/proofs` and `/Users/cathrynlavery/printing-press/.runstate/printing-press-c52790a1/runs/20260921-133106-1fbd7f53/pipeline/phase-receipts.jsonl`
- Prior 11-operation library preserved at `/Users/cathrynlavery/printing-press/.runstate/printing-press-c52790a1/runs/20260921-133106-1fbd7f53/working/sendfox-library-before-promotion` (103 files, hashes recorded). Original failed emission also preserved.

## Coverage and product surface

**60/60 documented operations**, up from 11 (+49), with request/response schema metadata and runtime request validation. Families: automation_emails: 2, automations: 6, campaigns: 9, contact_fields: 5, contact_tags: 5, contacts: 13, domains: 5, forms: 5, lists: 8, me: 1, unsubscribe: 1.

**139 command nodes, 123 leaves, 60 distinct endpoint commands; 41 MCP tools.** MCP exposes thin `sendfox_search`, full-schema `sendfox_get`, guarded `sendfox_execute`, campaign-performance intent and seven explicit evidence intents. Sixty endpoint mirrors are hidden; other tools provide local data and agent workflows. Scorecard's “60 tools” wording counts contract endpoint metadata; actual stdio tools/list counts 41.

Contacts, lists/membership, tags, custom fields, forms, domains, automations/emails, campaigns/stats/engagement and async bulk actions are covered. Preserved/replaced prior workflows include CSV audit/reconcile/import/onboard, list membership alias, auth setup, form generation, account-snapshot, audience-map, hygiene-report, campaign-digest and launch-plan. Obsolete campaign-write-unavailable assumptions were removed. Webhooks return an explicit capability gap.

Mutations require `--yes`; sensitive sends/schedules/activation/forms/domains/deletion additionally require `--approve-sensitive`. MCP mutations preview unless explicitly confirmed. Defaults are draft/inactive/count-preview; writes are never automatically replayed. Local limiter stays below one request/second; upstream quota is 60/minute shared across clients. Bounded async polling, validated requests, stable JSON, --agent/--select output, local SQLite sync/search/export/analytics and read-only SQL are available.

## Seven approved novel workflows

| Workflow | Built behavior | Verification |
|---|---|---|
| campaign-preflight | Draft content, links, targeting/exclusions, suppression and sender evidence | Unit/adversarial tests + local command/MCP sample PASS |
| campaign-review | Authoritative stats, link performance, engagement cohort and unscheduled resend plan | Unit/negative tests + local sample PASS |
| audience-health | Invalid/duplicate/suppressed/unassigned contacts and relationship/field checks | Unit/negative tests + local sample PASS |
| contact-dossier | Exact identity, memberships/tags/activity and ambiguity gate | Unit/negative tests + local sample PASS |
| automation-plan | JSON/YAML/Markdown definition to dependent inactive API steps | Unit/negative tests + local sample PASS |
| export-bundle | Fifteen evidence scopes, counts, completeness and stable hashes | Unit/negative tests + local sample PASS |
| migration-readiness | Mapping, reconciliation, both-provider suppression audit, delta, pilot checks, cutover checklist and rollback limits | Unit/negative tests + local sample PASS; actual migration/delivery unverified |

These are substantive local evidence workflows, not live-account certifications. Snapshots must use the documented normalized schema; direct ingestion of arbitrary native Kit CSV export layouts is not implemented. Migration reports remain unready where real delivery, current domains or sequence progress are unknown.

## Verification

- Full SendFox `go test ./...`: PASS (16 tested packages; additional packages have no tests). `go vet ./...`: PASS. CLI and MCP builds PASS.
- Actual CLI loopback HTTP matrix: **60/60**, correct methods and parsed JSON. Shared safety, schema validation, write retry/approval tests PASS.
- Help **139/139**; MCP stdio tool discovery/schema/search/workflow probes PASS. Seven approved workflows **7/7** execution and semantic sample checks PASS.
- Pagination/local pipeline: two pages, 21 stored rows, search/analytics/read-only SQL PASS; SQL mutation rejected.
- Printing Press verify **85/85 (100%)**, data pipeline PASS.
- Final umbrella shipcheck: **7/7 legs PASS**. Dogfood structural warning: manifest inline flag/value tokens are not recognized by its command mapper; actual workflow-verify passes all seven. Using WorkflowStep.Args is not a workaround because v4.32.4 executeStep ignores it.
- Scorecard **97/100, A**. Live API, MCP description and token-efficiency dimensions are explicitly unscored; this is not a live quality score.
- Tools audit: **0 pending**, 2 accepted generated framework descriptions. PII audit: **0 findings**. Gosec: raw 40 → 35, **0 unresolved novel-code findings**; generated findings triaged in gosec-triage.json. Govulncheck: **0 reachable vulnerabilities** (one unused required-module advisory).
- Output/code/docs reviews complete. Source suppression, incomplete delta, bounded evidence reads, ambiguous batch acknowledgement and --agent request-body retention have regression tests.
- Auth-aware live dogfood **SKIPPED**, per user's prohibition on live credentials; valid phase5-skip.json. No live acceptance pass was fabricated. All phases closed locally; no publish validation or publication attempted.

## Reproduction commands and evidence

Source `runstate/sendfox-bootstrap/run.env` from `/Users/cathrynlavery/printing-press`. The run's `scripts/safe-run.sh` strips inherited credentials, isolates SendFox config and defaults HTTP to a closed loopback port. Mock scripts select their own loopback server and synthetic fixture token.

```sh
cd "$CLI_WORK_DIR"
"$API_RUN_DIR/scripts/safe-run.sh" env -u SENDFOX_BASE_URL go test ./...
"$API_RUN_DIR/scripts/safe-run.sh" go vet ./...
"$API_RUN_DIR/scripts/safe-run.sh" go build -o sendfox-pp-cli ./cmd/sendfox-pp-cli
"$API_RUN_DIR/scripts/safe-run.sh" go build -o sendfox-pp-mcp ./cmd/sendfox-pp-mcp
python3 "$API_RUN_DIR/scripts/mock-operations.py"
python3 "$API_RUN_DIR/scripts/mock-sync.py"
python3 "$API_RUN_DIR/scripts/mcp-smoke.py"
python3 "$API_RUN_DIR/scripts/help-matrix.py"
"$API_RUN_DIR/scripts/safe-run.sh" "$API_RUN_DIR/working/printing-press-verifier/cli-printing-press-repaired" shipcheck --dir "$CLI_WORK_DIR" --spec "$RESEARCH_DIR/sendfox-enriched.yaml" --research-dir "$RESEARCH_DIR" --no-fix --no-live-check --json
```

Primary evidence: `final-tests.log`, `final-vet.log`, `mock-operations.json`, `mock-sync.json`, `help-matrix.json`, `mcp-smoke.json`, `output-review-final-samples.json`, `shipcheck-final-local.json`, `phase-4.85-findings.md`, `phase-4.95-findings.md`, `20260921-fix-sendfox-pp-cli-polish.md`, `phase5-skip.json`, `promotion.json`, `prior-library-preservation.json` under the proof directory above. Scripts and fixtures are retained for reproduction.

## Remaining limits and follow-up

Public API gaps versus Kit remain: webhook management/event delivery; purchases/orders; reusable snippets/templates; public posts/feed; equivalent aggregate growth analytics; richer behavioral filtering; automation timing/enrollment/sequence-progress parity. No CLI can manufacture those capabilities. Rollback cannot unsend messages or restore unsupported sequence state.

Before real operation or publication, a separately authorized live gate must verify entitlement/auth, real response/pagination/error behavior, final recipient selection, domain/DNS and delivery. No production readiness or migration parity claim is made. Normalize native Kit exports before readiness comparison. Prefer stdio MCP; generated HTTP transport still needs upstream ReadHeaderTimeout hardening.

A local Printing Press verifier repair parses SQL JSON rows/counts and separates stderr, avoiding a false 62-table report. Targeted regressions and real SendFox verification pass. Patch: `proofs/printing-press-verifier-local.patch`; installed Printing Press is unchanged. Copied-source full/golden upstream validation is **not green**: missing module-distribution fixtures, generator timeout and macOS Keychain failures/stall. Do not publish that patch without a full-checkout validation pass.

**No live SendFox/Kit credentials were used, no account migration or live account changes occurred, and nothing was published, pushed, PR'd or added to either catalog.** Local deliverable is ready for inspection; publication/live-operation gate remains unverified and outside this authorization.
