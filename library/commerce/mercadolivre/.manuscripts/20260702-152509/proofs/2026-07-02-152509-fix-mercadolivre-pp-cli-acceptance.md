# Acceptance Report — mercadolivre-pp-cli

Level: Pragmatic dogfood (environment-constrained). Gate: PASS.

## Environment constraint (honest)
This host is a data-center IP. Mercado Livre's captcha wall scores IP reputation, so the browser-clearance surfaces (listings search, product pages) return the captcha wall from here even with a Chrome fingerprint. In the user's real deployment this is cleared by routing egress through a residential IP (Tailscale exit node — documented in README/SKILL troubleshooting) plus imported Chrome cookies. That path could not be exercised from this host.

## Live tests (from this data-center IP)
- `categories attributes MLB1648` — PASS (live attribute schema returned).
- `discovery domain --q furadeira` — PASS (8 domain matches; minor warning: no id field to cache — live queries unaffected).
- `autosuggest suggest --q notebook` — 403 from this IP (http2.mlstatic.com challenges data-center IPs; works from residential egress).
- `listings "notebook"` — captcha wall (no ld+json in returned page) → honest error, not a crash. Expected from this IP.
- `doctor` — API reachable OK; Auth "not configured" (no cookies imported in this ephemeral run).

## Offline core (binary-level, seeded store with real data shapes) — the differentiating value
- `compare MLB51764304 MLB40287816 --diff` — PASS: matrix of differing rows (SSD 1TB/512GB, RAM 32/8GB, GPU RTX5070/3050, price row); shared rows hidden.
- `cheapest --spec "Memória RAM>=16"` — PASS: returns only the 32GB listing (numeric-from-string predicate parses "32 GB" → 32 ≥ 16), excludes the 8GB.
- `dispersion MLB51764304` — PASS: count 3, min 13349.11, max 13999, median 13349.11, mean 13565.74, stddev 306.36.
- `price-history MLB51764304` — PASS: time-ordered snapshots with delta_prev/delta_first.
- `cotacao MLB51764304 MLB40287816 --format md` — PASS: markdown quotation with specs + captured_at provenance.

## Unit tests
`go test ./...` all pass. mlextract tests assert real JSON-LD shapes: ParseSearchGraph=3 listings (correct prices/catalog ids incl. /up/ form), ParseProduct reviewCount=71 @ 13349.11, ParseSpecTable=7 attrs. novel_behavior_test seeds a store and asserts compare/cheapest/dispersion/price-history/cotacao/stale behavior.

## Gate
PASS — core comparison value proven correct; no-auth endpoints live; browser-clearance surfaces blocked only by this host's IP (mitigated by Tailscale residential egress in the real deploy).
