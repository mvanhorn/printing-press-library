# Pangolin CLI — Absorb Manifest

## Ecosystem reality check (Step 1.5a)
- Official CLI: **none**
- Official MCP server: **none**
- Claude plugins / skills: **none**
- Community CLIs on GitHub: **none meaningful**
- npm wrappers: **none**
- PyPI wrappers: **none**
- Adjacent server-side tools: Newt (client-only connect), Gerbil (gateway, not management)

This is **greenfield**. There is no incumbent to absorb. The "match everything that exists" obligation is therefore satisfied by exposing the full 157-endpoint OpenAPI surface as typed commands (which the generator does mechanically). The competitive moat is in transcendence, not parity.

## Absorbed Features (generator-emitted typed endpoint surface)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | All 157 spec endpoints | Pangolin OpenAPI v1 | (generated endpoint) typed `<resource> <verb>` commands across 14 resource groups | Offline SQLite mirror, --json/--select/--csv/--dry-run, agent-native errors |
| 2 | List orgs | Pangolin dashboard | (generated endpoint) `orgs list` | --json, paginated, indexed in local store |
| 3 | Get/create/update/delete org | Pangolin dashboard | (generated endpoint) `org get/create/update/delete` | --dry-run on mutations, store-backed reads |
| 4 | Site management (3 ops) | Pangolin dashboard | (generated endpoint) `site get/update/delete` + org-scoped variants | Cross-org listing via local store |
| 5 | Resource CRUD (26 ops) | Pangolin dashboard | (generated endpoint) `resource <verb>` and `org/{orgId}/resource <verb>` | FTS over name/niceId/ssl/http surfaces |
| 6 | Site-resource CRUD (15 ops) | Pangolin dashboard | (generated endpoint) `site-resource <verb>` | Role and user attachment as scriptable subcommands |
| 7 | Target CRUD (3 ops) | Pangolin dashboard | (generated endpoint) `target <verb>` | Drift detection via local-store join |
| 8 | Client management (8 ops) | Pangolin dashboard | (generated endpoint) `client <verb>` | Archive workflow as one command |
| 9 | IdP management (9 ops) | Pangolin dashboard | (generated endpoint) `idp <verb>` (incl. oidc subops) | Org-coverage view across local store |
| 10 | Role CRUD (4 ops) | Pangolin dashboard | (generated endpoint) `role <verb>` and add-user subcommand | Access-graph foundation |
| 11 | User CRUD + 2FA + role assignment (5 ops) | Pangolin dashboard | (generated endpoint) `user <verb>` | Bulk audit via local store |
| 12 | Certificate management (2 ops) | Pangolin dashboard | (generated endpoint) `certificate <verb>` and org-scoped cert get | Foundation for cert-watch transcendence |
| 13 | Domain namespace check + list (3 ops) | Pangolin dashboard | (generated endpoint) `domain check-namespace-availability`, `domains namepaces` | Scriptable in pre-provision flows |
| 14 | Access token revocation | Pangolin dashboard | (generated endpoint) `access-token delete` | Auditable; logs which tokens revoked |
| 15 | Maintenance info | Pangolin server | (generated endpoint) `maintenance info` | Wraps server health into `doctor` |

## Transcendence Features (hand-built; nobody else has these)

Customer model: a homelab operator running Pangolin on a VPS with 5–50 resources spread across 1–5 orgs. Pain points: dashboard-only audits, no DR backup, no scriptable provisioning, hard to answer "who can reach what." Plus: AI agents (Claude) want to drive Pangolin programmatically — they need a CLI shape that returns small, typed payloads, not whole dashboard JSON.

| # | Feature | Command | Buildability | Why Only We Can Do This |
|---|---------|---------|--------------|--------------------------|
| 1 | Cross-org health audit — stale targets, orphaned resources, missing roles | `pangolin audit [--org <id>]` | hand-code | Requires joining resources × targets × roles across all orgs in local SQLite; no single Pangolin endpoint returns this |
| 2 | Cert expiry watch (days-until-expiry, sorted, JSON-able) | `pangolin cert-watch [--days 30]` | hand-code | Requires fetching certs per org+domain and joining with local-store metadata to sort by expiry |
| 3 | Full config backup as version-controllable JSON | `pangolin backup [--out pangolin-backup.json]` | hand-code | Walks all 14 entity types, exports normalized tree the user can diff in git |
| 4 | Config restore / re-apply from backup against fresh install | `pangolin restore <backup.json> [--dry-run]` | hand-code | Mutating ordered walk; dependencies (org → site → resource → target) require manual sequencing |
| 5 | Access graph — "who can reach what" join across users × roles × resources × orgs | `pangolin access-graph [--user <id>] [--resource <id>]` | hand-code | Pure local-store join across 4 entity types; impossible from a single API call |
| 6 | One-shot expose: site (if needed) + resource + target + role bind | `pangolin expose <subdomain> --target <host:port> [--site <id>] [--role <id>] [--dry-run]` | hand-code | Sequences 3–5 API calls atomically with rollback-on-error; replaces 5-tab dashboard dance |
| 7 | Doctor with base-URL discovery hint when /v1 returns 404 | `pangolin doctor` | hand-code | Probes both `/v1` and `/api/v1` and reports the correct mount path — Pangolin's two real-world mount conventions trip everybody up |

### Adversarial cut (what was considered and rejected)
- **`tail` — live resource access logs.** Pangolin has no log streaming endpoint exposed in the integration API. Drop.
- **`benchmark` — synthetic load against a resource.** Out of scope; users have wrk/k6 already.
- **`migrate` — move resources between orgs.** Pangolin's data model treats org as part of the resource identity; not a simple move. Defer.

## User Vision
User has not volunteered specific features (briefing was "let's go"). Inferred from project memory: Pangolin is part of the user's HomeLab/VPS stack. The transcendence list above is tuned to that operator profile.

## Stub list
**None.** All 7 transcendence features are shipping scope. The CLI ships with all 7 implemented or it returns to this gate with a revised manifest.

## Scope summary
- **Absorbed:** 157 spec endpoints → ~157 typed commands (generator handles)
- **Transcendence:** 7 hand-coded commands (audit, cert-watch, backup, restore, access-graph, expose, doctor)
- **Hand-code count:** 7 features, each ~80–200 LoC + root.go wiring
