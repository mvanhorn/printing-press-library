# DNS Made Easy CLI — Absorb Manifest

Sources surveyed: official `DNSMadeEasy/dme-go-client` SDK, `kigster/dnsmadeeasy` (Ruby `dme` CLI, richest competitor), `jswank/dnsme` (Go CLI), `mhenderson-so/godnsmadeeasy`, `john-k/dnsmadeeasy`, `soniah/dnsmadeeasy`, `DNSMadeEasy/dme` Terraform provider, `go-acme/lego` DNS provider.

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List managed domains | all wrappers | (generated endpoint) domains list | Local store, --json/--select, offline after sync |
| 2 | Get managed domain by id | all wrappers | (generated endpoint) domains get | Typed exit codes, --select |
| 3 | Get domain by name | jswank, kigster get_id_by_domain | dnsmadeeasy-pp-cli domains find | Resolves name→id offline from mirror |
| 4 | Create managed domain | all wrappers | (generated endpoint) domains create | --dry-run, agent-native |
| 5 | Create multiple domains | kigster create_domains | (generated endpoint) domains create-multi | Batch, idempotent |
| 6 | Update managed domain (soa/vanity/template/folder/gtd) | dme-go-client | (generated endpoint) domains update | --dry-run |
| 7 | Delete managed domain | all wrappers | (generated endpoint) domains delete | --dry-run confirm |
| 8 | Delete multiple domains | kigster | (generated endpoint) domains delete-multi | Batch |
| 9 | List records in a domain (filter type/name) | all wrappers | (generated endpoint) records list | Local FTS, cross-zone search superset |
| 10 | Get single record | all wrappers | (generated endpoint) records get | --select |
| 11 | Create record | all wrappers | (generated endpoint) records create | --dry-run, typed fields |
| 12 | Typed record creators (A/AAAA/CNAME/MX/TXT/SRV/NS/PTR/SPF/CAA) | kigster create_*_record | (behavior in dnsmadeeasy-pp-cli records create) --type sets typed fields | One command, all types, validated |
| 13 | Update record | all wrappers | (generated endpoint) records update | --dry-run |
| 14 | Delete record | all wrappers | (generated endpoint) records delete | --dry-run |
| 15 | Create multiple records | dme-go-client createMulti | (generated endpoint) records create-multi | Batch from --stdin/JSON |
| 16 | Update multiple records | dme-go-client updateMulti, kigster update_records | (generated endpoint) records update-multi | Batch |
| 17 | Delete multiple records | dme-go-client, kigster delete_records | (generated endpoint) records delete-multi | Batch by ids |
| 18 | Delete all records in a zone | kigster delete_all_records | (behavior in dnsmadeeasy-pp-cli records delete-multi) --all flag | Guarded by --dry-run |
| 19 | List/get/create/update/delete secondary domains | dme-go-client, kigster | (generated endpoint) secondary * | Full CRUD |
| 20 | List/get/create/update/delete secondary IP sets | dme-go-client, kigster | (generated endpoint) ipsets * | Full CRUD |
| 21 | Templates CRUD | dme-go-client | (generated endpoint) templates * | Full CRUD |
| 22 | Template records CRUD | dme-go-client model_template_record | (generated endpoint) template-records * | Full CRUD |
| 23 | SOA records CRUD | dme-go-client modelSOA | (generated endpoint) soa * | Full CRUD |
| 24 | Vanity DNS CRUD | dme-go-client modelVanity | (generated endpoint) vanity * | Full CRUD |
| 25 | Transfer ACL CRUD | dme-go-client model_acl | (generated endpoint) acl * | Full CRUD |
| 26 | Folders CRUD | dme-go-client modelFolder | (generated endpoint) folders * | Full CRUD |
| 27 | Contact lists CRUD | dme-go-client model_contact_list | (generated endpoint) contact-lists * | Full CRUD |
| 28 | Failover/monitor get/update per record | dme-go-client model_failover | (generated endpoint) monitor * | Cross-zone rollup via health |
| 29 | Usage/reports query | api docs | (generated endpoint) usage get | --json |
| 30 | Multi-credential config (API key + secret) via env/file | kigster dme | (behavior in dnsmadeeasy-pp-cli doctor) + config | Two-var env + config.toml |
| 31 | Full-account mirror to local SQLite | printing-press framework | dnsmadeeasy-pp-cli sync | Rate-limit aware, page pagination |
| 32 | Offline search / SQL / analytics over synced data | printing-press framework | dnsmadeeasy-pp-cli search / sql / analytics | Cross-zone, offline |
| 33 | Health/auth check | printing-press framework | dnsmadeeasy-pp-cli doctor | Verifies HMAC signing + rate-limit budget |

## Foundation (Priority 0 — not a feature row but the gate to everything)
- **HMAC-SHA1 request signing** applied to ALL requests (generated + hand-coded) via a signing http.RoundTripper wired into the generated client. Two-credential config (DNSMADEEASY_API_KEY + DNSMADEEASY_API_SECRET). Rate-limit-aware (respect x-dnsme-requestsRemaining, retry on 429).

## Transcendence (only possible with our approach — all hand-code)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Cross-zone value provenance | where-used | hand-code | Local SQLite join across every zone's records; the API is one-zone-at-a-time and rate-limited | Use to READ where an IP/value/target appears across all zones (audit, migration planning). Do NOT use it to change records; use 'bulk-apply' instead. |
| 2 | Snapshot drift detection | drift | hand-code | Requires per-record sync snapshots in local SQLite; the API has no diff/history | none |
| 3 | Zone export (BIND/JSON) | export | hand-code | Serializes a zone from the local mirror to a standards BIND zone file with zero API calls | none |
| 4 | DNS-hygiene audit | health | hand-code | Mechanical rule checks over the whole-account local mirror (dangling CNAME, TTL band, missing SPF/DMARC/CAA, dupes) | none |
| 5 | Cross-zone bulk record change | bulk-apply | hand-code | Selects targets across all zones locally, then applies via real updateMulti per zone; respects rate limits | Use to WRITE the same value change across many zones (IP migration). To only find affected records without changing anything, use 'where-used'. |
| 6 | Stale ACME challenge cleanup | acme-purge | hand-code | Finds _acme-challenge TXT across all zones from the mirror and deletes stale/orphaned ones via deleteMulti | none |

Transcendence rows: 6 planned, all hand-code, all scored >= 7/10.
