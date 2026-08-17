# DNS Made Easy CLI — Live Acceptance (read-only)

Level: FULL (read-only). Live API: https://api.dnsmadeeasy.com/V2.0. Gate: PASS (10/10).
Credentials: user-supplied API key + secret (env only; never written to any artifact). Redacted below.

| # | Command | Result |
|---|---------|--------|
| 1 | doctor --json | api reachable, auth configured — HMAC-SHA1 signing verified live |
| 2 | domains list | 41 zones returned |
| 3 | sync-records | 41 zones, 366 records mirrored, 0 failed zones, snapshot written |
| 4 | health --json | scanned 40 zones / 366 records; 96 findings (40 missing-caa, 32 missing-dmarc, 24 missing-spf) |
| 5 | where-used <shared-ip> | 53 records across multiple zones share one hosting IP — flagship cross-zone provenance works |
| 6 | export <zone> --format bind | valid BIND zone file; MX priority + trailing-dot formatting correct |
| 7 | drift | correctly reports <2 snapshots (need 2 to diff) after a single sync |
| 8 | bulk-apply --match .. --set .. (PREVIEW) | built plan, "preview only — re-run with --apply"; NO writes |
| 9 | acme-purge (PREVIEW) | no _acme-challenge TXT records found; NO deletes |
| 10 | records list <domainId> (generated) | 76 records returned via signed request |

All client domain names and IPs redacted from this report per PII rules (they were the user's own account data).
No write/delete operation was performed. bulk-apply/acme-purge preview-by-default confirmed live.

Verdict: PASS — live smoke clear. Ready to publish on user approval.
