# DNS Made Easy — Novel Features Brainstorm (subagent audit trail)

## Customer model
- Maya — Platform SRE, ~180 zones, needs "where does IP X appear across all zones" and pre-cutover TTL sanity. Frustration: API is one-zone-at-a-time, rate-limited.
- Devin — Hosting-provider ops, thousands of zones, bulk IP migrations, zone export/backup, mail-security audits. Frustration: no way to QUERY the account, only fetch it; no local mirror.
- Priya — ACME DNS-01 automation. Frustration: orphaned _acme-challenge TXT records with no cross-account cleanup.
- Sam — Release engineer, drift/change review. Frustration: API has no diff/history; out-of-band changes invisible.

## Survivors (>=5/10, all hand-code)
1. where-used <ip-or-value> — 10/10 — cross-zone value/target provenance from local mirror.
2. drift [--since <snapshot>] [zone] — 9/10 — diff two most recent sync snapshots.
3. export <zone> --format bind|json — 9/10 — serialize zone from local mirror.
4. health [--zone <z>] — 8/10 — DNS hygiene audit (dangling CNAME, TTL band, missing SPF/DMARC/CAA, dupes).
5. bulk-apply --match <v> --set <v> [--type] [--dry-run] — 8/10 — cross-zone bulk change via updateMulti.
6. acme-purge [--older-than 24h] [--dry-run] — 7/10 — delete stale _acme-challenge TXT via deleteMulti.

## Killed candidates
- ip-report (→ where-used), zone-diff (→ drift), cname-audit (→ health), failover-report (→ health),
  template-reconcile (→ drift), ddns-update (dropped — needs external IP service or thin wrapper).
