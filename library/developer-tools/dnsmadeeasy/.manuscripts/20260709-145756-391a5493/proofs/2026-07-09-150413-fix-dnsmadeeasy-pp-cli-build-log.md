# DNS Made Easy CLI — Build Log

Manifest transcendence rows: 6 planned, 6 built. Phase 3 will not pass until all 6 ship.

## Priority 0 (foundation)
- HMAC-SHA1 signing transport (internal/client/dnsmadeeasy_signing.go) + one-line hook in client.New — DONE. Covers all requests (generated + novel). Tests pass (signing contract + all-3-headers + no-cred passthrough).
- Environment note: press binary pins go1.26.4 for its govulncheck gate; go1.26.4 stdlib has GO-2026-5856 (crypto/tls ECH, fixed 1.26.5). Generated code is clean under go1.26.5 (build+vet+govulncheck: "No vulnerabilities found"). Not a CLI defect.

## Priority 1 (absorbed) — DONE
- Full single-resource CRUD generated for 12 resources (domains, records, secondary, ipsets, templates, template-records, soa, vanity, acl, folders, contact-lists, monitor). Framework sync mirrors flat resources; records are hierarchical (custom sync-records covers them).
- Multi-record ops: updateMulti ships via bulk-apply, deleteMulti via acme-purge. Standalone records create-multi/update-multi/delete-multi commands NOT shipped as separate endpoints (raw-array bodies don't map to flag-based generated commands); capability is covered by the transcendence write commands. Documented limitation.
- 'usage' account-report endpoint dropped from spec (exact path unconfirmed; not shipping a guessed endpoint).
## Priority 2 (transcendence) — DONE (6/6)
- where-used (local mirror), drift (snapshot diff), export (BIND/JSON), health (hygiene rules), bulk-apply (live updateMulti, preview-by-default + --apply), acme-purge (live deleteMulti, preview-by-default + --apply).
- Custom store tables zone_records + record_snapshots (internal/store/dnsmadeeasy_migrations.go); shared live-fetch helpers (internal/cli/dnsmadeeasy_mirror.go); new sync-records command populates the cross-zone mirror + drift snapshots.
- Behavioral tests: signing contract, BIND formatting, value application, zone matching, mirror round-trip. All pass. vet clean.
