# domain-goat CLI Brief

## API Identity
- **Domain**: domain-name discovery & availability research (NOT purchase/transaction)
- **Users**: founders, marketers, brand researchers, drop-catchers, agents (LLM-driven) hunting names
- **Data profile**: candidate domains, TLDs, registrars, WHOIS/RDAP records, premium listings, watch lists, pricing snapshots

## Reachability Risk
- **Low** for the headline path (RDAP). IANA bootstrap covers all legacy gTLDs and the bulk of new gTLDs. RDAP 404 = unregistered (reliable for gTLDs).
- **Medium** for some ccTLDs — RDAP coverage ~60% in IANA bootstrap; Verisign tarpits port-43 WHOIS aggressively. Fall back: RDAP → WHOIS port 43 → DNS heuristic.
- **Known dead**: Freenom (.tk/.ml/.cf/.ga) collapsed 2023; Squarespace (ex-Google Domains) no API; Dan.com API deprecated post-GoDaddy acquisition.
- **GoDaddy API gating** (since 2023): new keys silently 403 unless account holds ≥10 domains. Skip as primary; offer as opt-in.

## Top Workflows
1. **Bulk check** a list of candidate names × TLD matrix → CSV/JSON with availability + price + premium flag.
2. **Generate names** from seed keywords: prefix/suffix combos, portmanteaus, hack-style TLD splits (`del.icio.us`, `kub.es`), dictionary mash-ups.
3. **Typosquat / similar-name** discovery (dnstwist-style permutations: homoglyph, omission, transposition, vowel-swap, bitsquatting).
4. **Watch list / drop-watch** — track candidate expiry/status across time, alert on `pendingDelete` / `redemptionPeriod`.
5. **Score & shortlist** — score candidates (length, brandability, dictionary match, vowel-consonant pattern, keyword presence), tag, annotate, export shortlist.
6. **Cross-registrar price compare** — Porkbun pricing endpoint (free, no-auth, ~600 TLDs) as the public price source; per-registrar prices when configured.

## Table Stakes
- WHOIS lookup (RFC 3912)
- RDAP lookup (RFC 7480-7484) with IANA bootstrap
- Availability check (single + bulk)
- TLD list with metadata (gTLD/ccTLD, RDAP base, WHOIS server, has-RDAP)
- Pricing lookup (Porkbun public endpoint)
- IDN handling (`golang.org/x/net/idna`)
- DNS heuristic (A/NS/MX) as fast pre-filter
- `--json` + `--csv` everywhere

## Data Layer
- **Primary entities**:
  - `tlds` — TLD, type, rdap_base, whois_server, has_rdap, price snapshot
  - `domains` — FQDN, ascii, idn, length, tld_id, score, status, registrar_status, created_at, expires_at, last_checked_at
  - `candidates` — domain_id, list_id, score, notes, tags, added_at
  - `lists` — name, description, created_at
  - `watches` — domain_id, cadence (hours), last_run_at, last_status
  - `searches` — saved generator recipe (seed, transforms, tlds, filters)
  - `suggestions` — generated candidate with provenance (generator, score)
  - `whois_records` — domain_id, raw, parsed_json, source, fetched_at
  - `rdap_records` — domain_id, raw_json, status, events_json, fetched_at
  - `pricing_snapshots` — tld, registrar, registration_price, renewal_price, transfer_price, fetched_at
- **Sync cursor**: last_pricing_fetched_at per registrar
- **FTS5/search**: across domain.fqdn, candidates.notes, candidates.tags, lists.name

## Codebase Intelligence
- **Recommended Go deps**:
  - `github.com/likexian/whois` + `github.com/likexian/whois-parser` (active, the Go WHOIS standard)
  - `github.com/openrdap/rdap` (Go RDAP standard, ships its own `rdap` CLI as reference)
  - `golang.org/x/net/idna` (stdlib-adjacent IDN encoding)
- **Don't**: avoid `domainr-cli` (abandoned 2018); avoid rolling our own RDAP parser; avoid hitting Verisign port-43 in tight loops (rate-limit hell).
- **Architecture**: source-priority resolver — RDAP → WHOIS → DNS → registrar API → cached. Each result tagged with provenance.

## User Vision
> "I just want to identify domains to purchase, we don't need to actually purchase them as part of this."

Therefore: no checkout flow, no cart, no add-funds, no DNS-management commands. Everything is read-heavy discovery, scoring, and persistence of a shortlist the user later buys manually.

## Product Thesis
- **Name**: `domain-goat-pp-cli`
- **Why it should exist**: existing tools split between dumb-pipe (`whois`), abandoned (`domainr-cli`), or web-only (instant-domain-search.com, Domainr.com). None offer offline-persisted candidate shortlisting, batch generation engines, RDAP-native lookups, cross-registrar price comparison, AND an MCP surface for agent-driven discovery sessions. `domain-goat` is the first CLI that combines the data layer + generation engine + RDAP + price comparison + agent surface in one binary.

## Build Priorities
1. **Data layer + RDAP/WHOIS resolver** — tlds + IANA bootstrap sync + RDAP lookup with WHOIS fallback. Cache results.
2. **Bulk availability check** — `check <name>...` and `check --file names.txt --tlds com,io,ai,dev`.
3. **Generation engine** — prefix/suffix, portmanteau, typosquat, hack-style, dictionary combos. Offline.
4. **Candidate shortlist** — `lists create/add/show`, score, notes, tags, export.
5. **Pricing** — sync Porkbun pricing endpoint; show cross-TLD comparison.
6. **Watch lists** — drop-watch with cadence, status-change detection.
7. **Premium/aftermarket flag** — Domainr API integration (optional, requires RapidAPI key).
8. **Transcendence**: NOI commands that only work because everything is in SQLite — `compare`, `shortlist score`, `domain-goat similar <fqdn>` (typosquat over a seed), `domain-goat drops --soon 30` (expiry timeline), `domain-goat brandscore <name>`.
