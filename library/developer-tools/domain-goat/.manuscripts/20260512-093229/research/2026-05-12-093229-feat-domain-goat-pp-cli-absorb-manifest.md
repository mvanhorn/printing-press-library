# domain-goat Absorb Manifest

## Tools Catalogued
- `whois` (BSD CLI), `dnstwist`, `domainr-cli` (archived), `openrdap/rdap`, `dig/host/drill` (DNS-only)
- MCP servers: saidutt46/domain-check, kolontsov/domain-mcp, simplebytes-com/domaindetails-mcp, dorukardahan/domain-search-mcp, bharathvaj-ganesan/whois-mcp, patrickdappollonio/mcp-netutils
- Web tools: Domainr, Instant Domain Search, NameMesh, Lean Domain Search, ExpiredDomains.net, NameBoy, Panabee
- npm: whoiser, node-whois, whois-json
- Registrar SDKs (read-only): Namecheap, Porkbun, Name.com, GoDaddy

## Absorbed (match-or-beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | WHOIS lookup (single) | likexian/whois | `whois <domain>` via Go `likexian/whois` + `whois-parser` | `--json`, `--raw`, cached in SQLite |
| 2 | WHOIS recursive referral | BSD `whois -R` | follow=registry+registrar | `--follow=registry\|registrar\|all` |
| 3 | RDAP lookup | openrdap/rdap | `rdap <domain>` via `openrdap/rdap` | `--json`, cached, source tagged |
| 4 | Bootstrap RDAP server map | openrdap/rdap | `tlds sync` syncs IANA dns.json | Local cache, offline TLD lookup |
| 5 | Availability check (single) | Domainr `/v2/status` | `check <domain>` — RDAP → WHOIS → DNS | Provenance per result, no API key needed |
| 6 | Availability check (bulk) | Name.com bulk + Namecheap bulk | `check <d1> <d2> <d3>...` parallel | No registrar key needed (RDAP path) |
| 7 | TLD matrix expansion | dnstwist `--tld` | `check <name> --tlds com,io,ai,dev` | One name × N TLDs in one call |
| 8 | File-driven bulk | Instant Domain Search CSV | `check --file names.txt --tlds ...` | Stdin support, parallel, CSV output |
| 9 | Permutation: addition | dnstwist `addition` | `similar <fqdn> --types addition` | Same shape, Go impl |
| 10 | Permutation: bitsquatting | dnstwist `bitsquatting` | `similar --types bitsquatting` | |
| 11 | Permutation: homoglyph | dnstwist `homoglyph` | `similar --types homoglyph` | IDN/Unicode aware |
| 12 | Permutation: hyphenation | dnstwist `hyphenation` | `similar --types hyphenation` | |
| 13 | Permutation: insertion | dnstwist `insertion` | `similar --types insertion` | |
| 14 | Permutation: omission | dnstwist `omission` | `similar --types omission` | |
| 15 | Permutation: repetition | dnstwist `repetition` | `similar --types repetition` | |
| 16 | Permutation: replacement | dnstwist `replacement` | `similar --types replacement` | |
| 17 | Permutation: subdomain | dnstwist `subdomain` | `similar --types subdomain` | |
| 18 | Permutation: transposition | dnstwist `transposition` | `similar --types transposition` | |
| 19 | Permutation: vowel-swap | dnstwist `vowel-swap` | `similar --types vowel-swap` | |
| 20 | Permutation: tld-swap | dnstwist `tld-swap` | `similar --types tld-swap` | |
| 21 | Dictionary mash | NameMesh `Mix` | `gen mix --seeds <a> <b>` | Bundled common-noun dict |
| 22 | Prefix/suffix combos | Lean Domain Search | `gen affix --seed brand --prefixes get,my --suffixes -hq,-ly,-app` | |
| 23 | Hack-style domains | Domainr | `gen hack <word>` — split-on-TLD (`del.icio.us`, `kub.es`) | |
| 24 | Portmanteau | NameMesh `Common`/`New` | `gen blend --seeds a b` — overlap-merge | |
| 25 | Rhyme / phonetic | NameBoy | `gen rhyme <word>` | Metaphone-based |
| 26 | Length filter | many | `--min-len`, `--max-len` global | |
| 27 | TLD whitelist | many | `--tlds com,io` everywhere | |
| 28 | Exclude hyphens/digits | NameMesh | `--no-hyphens --no-digits` | |
| 29 | Available-only filter | Lean Domain Search | `--available-only` | |
| 30 | Registered-only filter | dnstwist `--registered` | `--registered-only` | |
| 31 | Premium-flag detection | Domainr `marketed` + Porkbun | `check --show-premium` | Marks `premium=true` in output |
| 32 | Marketplace listing flag | Domainr `marketed` | Same as #31 | |
| 33 | Pricing lookup (registration) | Porkbun `/pricing/get` | `pricing get <tld>` — sync into SQLite | Public no-auth endpoint |
| 34 | Pricing renewal/transfer | Porkbun | Same as #33 — all three columns | |
| 35 | Cross-TLD pricing compare | TLD-List.com (scrape only) | `pricing compare <tld1> <tld2>...` | Local table, instant |
| 36 | TLD list / metadata | Namecheap `getTldList` + GoDaddy `tlds` | `tlds list` — gTLD/ccTLD, has-RDAP | Synced from IANA bootstrap |
| 37 | TLD info | dorukardahan `tld_info` | `tlds info <tld>` | Registry + RDAP + WHOIS server + has-RDAP flag |
| 38 | Whois TLD record | bharathvaj `whois_tld` | `whois --tld <tld>` | |
| 39 | Whois IP record | bharathvaj `whois_ip` | `whois --ip <ip>` | likexian also supports |
| 40 | Whois ASN | whoiser `asn` | `whois --asn <asn>` | |
| 41 | DNS A/AAAA/NS/MX/SOA | dig | `dns <domain> [--type A,NS,...]` | One-shot summary |
| 42 | DNS heuristic availability | many | RDAP fallback chain | Tagged as `dns-heuristic` |
| 43 | TLS cert check | patrickdappollonio | `cert <domain>` — issuer + SANs + expiry | Connect, read, close |
| 44 | Reverse PTR | dig `-x` | `dns --reverse <ip>` | |
| 45 | Score: length | most | brandability scorer: length component | |
| 46 | Score: syllable count | NameBoy, NameMesh | scorer: syllable component | |
| 47 | Score: pronounceability | Brand-name papers | scorer: n-gram phoneme freq | |
| 48 | Score: vowel-consonant ratio | many | scorer | |
| 49 | Score: dictionary-word match | most | scorer + boolean flag | |
| 50 | Score: hack-style | Domainr | scorer + boolean flag | |
| 51 | Score: palindrome / ABBA | rare | scorer flag | |
| 52 | Score: repeat-letter count | rare | scorer | |
| 53 | Score: hyphen/digit count | many | scorer | |
| 54 | Score: TLD prestige tier | manual | scorer (tier table .com>.io>.ai>.app>...) | |
| 55 | Brandability composite | DomainScore.ai | `score <fqdn>` — weighted composite | Configurable weights |
| 56 | Drop / expiry watch | ExpiredDomains.net | `watch add/list/run` | Self-hosted, cron-friendly |
| 57 | Drop window query | ExpiredDomains pending-delete | `drops --soon 30` | Local watchlist |
| 58 | Save / favorite list | Lean Domain Search | `lists create/add/show` | SQLite-backed |
| 59 | List notes + tags | rare | `lists annotate --tag <tag> --note ...` | |
| 60 | List CSV export | Instant Domain Search | `--csv` everywhere; `lists export` | |
| 61 | List JSON export | many | `--json` everywhere; `lists export --json` | |
| 62 | Social handle check | Lean Domain Search | `socials <name>` — Twitter/GitHub/IG presence (HEAD requests) | Read-only, no auth |
| 63 | Twitter handle length | Lean Domain Search | `socials --twitter <name>` | length validation |
| 64 | Suggestion engine (smart) | dorukardahan `suggest_domains_smart` | `gen suggest --seed <topic>` — runs all generators, scored | |
| 65 | Bulk suggest | many | `gen suggest --seeds-file ...` | |
| 66 | Project analyzer | dorukardahan `analyze_project` | `analyze <description>` — extract keywords → suggestions | |
| 67 | Hunt mode | dorukardahan `hunt_domains` | `hunt --topic "<idea>" --tlds ...` — generate + filter + score | |
| 68 | Compare registrars | dorukardahan `compare_registrars` | `pricing compare --registrars porkbun,namecheap,namedotcom` | |
| 69 | IDN / punycode | most | `--idn` everywhere, `golang.org/x/net/idna` | Unicode display, ASCII storage |
| 70 | Wildcard / pattern search | rare | `search "f*o" --tlds com,io` over local store | FTS5 |
| 71 | Show full RDAP entity tree | openrdap | `rdap --raw` | |
| 72 | Bulk RDAP for nameserver / entity | openrdap | `rdap --type nameserver/entity` | |
| 73 | Doctor / connectivity check | many MCPs | `doctor` — RDAP reachability, WHOIS port 43, DNS, Porkbun pricing | |
| 74 | Quiet / agent-friendly output | none | `--agent`, `--compact`, `--select` standard | |
| 75 | Dry-run | none | `--dry-run` on any mutation | |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Why Only We Can Do This |
|---|---------|---------|-------|------------------------|
| T1 | Top-N finalist promotion | `shortlist promote --top 10 --by combined` | 8/10 | Local join `candidates × pricing_snapshots × rdap_records`, ranks by `score - price_penalty + availability_bonus`; nobody persists candidates AND pricing AND RDAP in one store |
| T2 | 5-year true-cost filter | `budget --max-renewal 50 --years 5 --list current` | 8/10 | Computes `registration + (years-1) × renewal` from Porkbun pricing snapshots — registrar UIs hide the year-2 jump until checkout |
| T3 | Side-by-side compare | `compare <fqdn> <fqdn>...` | 9/10 | One-row-per-domain join across every table (score, length, TLD prestige, prices, RDAP status, drop flag) — no current CLI offers this |
| T4 | Drop-timeline by score | `drops timeline --days 30 --min-score 7 --tld io,ai` | 9/10 | Reads persisted RDAP `events_json` for `pendingDelete`/`redemptionPeriod`, joins to candidate scores — ExpiredDomains.net is web-only and doesn't score |
| T5 | Why-killed audit | `why-killed <fqdn>` | 7/10 | FTS5 over `candidates.notes + tags`, joins last pricing/RDAP snapshot — answers "did we kill this for trademark or for length?" weeks later |
| T6 | Pricing arbitrage radar | `pricing-arbitrage --by renewal-delta` | 6/10 | Aggregates `pricing_snapshots` per TLD, surfaces year-1-trap TLDs and prestige/price outliers — Porkbun is the only no-auth source with both registration & renewal |
| T7 | Drop re-release window | `drop-bid-window <fqdn>` | 7/10 | Reads RDAP `events_json`, adds RFC pending-delete grace → exact UTC drop window — ICANN-deterministic but unsurfaced |
| T8 | Seed → TLD affinity | `tld-affinity <seed>` | 6/10 | Joins `tlds × pricing × candidates`, scores TLD fit by suffix-semantics + historical availability + price tier — grounds the gut-feel "should I look at .ai or .studio?" question |

## Stubs / Deferred
None. Every absorbed and transcendent feature is shipping scope.

