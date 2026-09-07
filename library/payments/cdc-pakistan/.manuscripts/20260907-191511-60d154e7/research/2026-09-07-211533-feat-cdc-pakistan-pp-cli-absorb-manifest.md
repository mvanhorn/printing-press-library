# CDC Pakistan — Absorb Manifest

## Absorbed (match or beat everything that exists)

**EMPTY BY MEASUREMENT.** There is no incumbent to absorb.

Evidence (4 parallel research agents, ~180 tool calls):
- GitHub repo search: 0 results across 23 query formulations (`cdcpakistan`, `cdcaccess`,
  `central depository company pakistan`, `CDS eligibility pakistan`, `free float pakistan`, ...).
- GitHub **code** search: `"update_posts_by_year"` -> **0 hits anywhere on GitHub, any language**.
  `"Share Percentage in CDS"` -> 0.
- npm / PyPI / CRAN / pkg.go.dev / RubyGems: no package reads any CDC surface.
- MCP registries (lobehub, mcpmarket, fastmcp, glama, smithery): no CDC server.
- Closest consumer on earth: `adanos-software/free-ticker-database` (*24, 2.5GB) cites the CDC
  ISIN-with-CFI PDF as a Tier-1 provenance source for **one row**, hand-pasted. No fetcher,
  no parser, no cadence. Demonstrated demand, zero automation.
- Adjacent Pakistani tooling (177 PSX repos, pp-psx, pp-nccpl, psx-data-reader, psxdata,
  pakistan-mutual-funds-*) covers PSX/NCCPL/MUFAP surfaces. The "free float" they expose is
  PSX's computed float or NCCPL's margin-eligibility float — **definitionally different** from
  CDS custody penetration. Zero overlap with any CDC surface.

Consequence: every row below is transcendence. There are no table stakes to match.

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Row-level extraction audit | verify rows | hand-code | Asserts the report's own `%` column equals shares_available/paid_up_incl*100 per row, plus in-row invariants (excl<=incl, CDS<=incl) and ISO-6166 check digits; residual SIGNATURES separate separator-loss from column-shift from isolated misreads. No reference parser for these PDFs exists anywhere, so this is the only correctness bar. | Use this to prove the PDF extraction is trustworthy before any analysis. Do NOT use it to compare vintages; use 'verify schema' for that. |
| 2 | Identity ledger + point-in-time resolve | identity ledger | hand-code | Diffs security-list vintages and folds 145 symbol/name-change events into typed identity intervals; resolves (symbol, date) to the identity in force and returns NOT_IN_VINTAGE on a gap rather than guessing. Catches symbol recycling — the failure that silently splices two issuers into one return series. | Use this before joining CDC data to any symbol-keyed price series. Do NOT use it for eligibility state; use 'eligibility state'. |
| 3 | Three-way float reconciliation | float triangulate | hand-code | Puts CDC custody penetration beside PSX free-float shares and NCCPL free-float %, with the denominator named per row. The CDC leg exists in no package or dataset on earth; the other two are already local. Custody penetration != tradable float != margin-eligibility float, and the command must never average them. | Use this to see where the three float definitions disagree. It deliberately does not produce a single blended number. |
| 4 | Coverage map + paginator integrity | coverage map | hand-code (LOCAL WRITE) | Walks every (term, year, paged) bucket storing per-block content hashes, so the URL-pagination repeat defect and any future regression are caught mechanically; records tri-state per bucket: found / not-found-at-source / not-probed. A coverage map without persisted negatives is a lie. | Use this first, before trusting any other command's completeness. This is the only command that writes the document index. |
| 5 | CDS eligibility state machine | eligibility state | hand-code | Folds 7,639 dated notices/circulars into a per-ISIN 6-state lifecycle (declared -> intention-to-suspend -> suspended -> extension -> removal | revoked | terminated) with effective_date, notice_url and classifier_confidence. No state endpoint exists; state is only the fold of a complete 20-year crawl. | Use this for CDS-eligibility history. Note CDS eligibility is NOT the same as PSX trading suspension — it is not a tradability gate. |
| 6 | Schema + seam gate | verify schema | hand-code | Fingerprints each vintage (header text, column count, part count, rows-per-part) and blocks a load on drift until acknowledged. Proven necessary: Jan-2024 has 1,206 rows WITH Market Value and no STATUS/% columns; Nov-2025 part A has 29,350 rows WITHOUT Market Value and WITH STATUS/%. A month missing a part is recorded incomplete, never emitted. | Use this whenever a new vintage lands. It will refuse silently-incompatible data rather than blend it. |
| 7 | Statistics store (append-only) | stats history | spec-emits | Append-only snapshots of the 16-metric aggregate table with first_seen/last_seen and provenance(live|archive). The page is OVERWRITTEN monthly, so month M's scalar ceases to exist after M+1 unless snapshotted. Label drift is real (Sahulat row deleted; Securities split Listed/Unlisted). | Use this to track CDS aggregates forward. Historical rows are provenance='archive' and NON-CONTIGUOUS — never read them as a series. |
| 8 | Per-symbol GoP stake (level) | gop stake | hand-code | paid_up_incl_GoP - paid_up_excl_GoP = state-held capital per security. No Pakistani source publishes per-symbol state ownership in machine-readable form, and PSX carries heavy SOE weight. LEVEL ONLY — the delta series is killed for want of comparable vintages. | Use this for a current cross-section of state ownership. There is no time series; do not ask for deltas. |

Minimum-5 requirement: met (8 rows).

## KILLED on measured evidence (recorded so nobody re-proposes them)
- `float asof` (point-in-time panel) — needs >=2 schema-comparable vintages. We have ZERO
  comparable pairs: 1 live vintage; the one archived vintage available for diff has a
  different column set and a 24x smaller universe.
- `panel reconcile` (presence/step change detection) — same reason. Longitudinal by definition.
- `gop stake --delta` — the delta half of #8, same reason.
- Anything holder-tier, per-security investor-class, or resident/non-resident — CDC publishes
  Individual vs Corporate at AGGREGATE level only.
- Any promise of pre-CLI statistics history beyond a labelled provenance='archive' flag.
- `--compare-market tw|id` (TDCC/KSEI) — new transport, new schemas, zero CDC content.
- Ingest verbs (`downloads sync`, `master pull`, `statistics snapshot`) — table stakes, not
  features, and deliberately not counted in the feature list.

## Stubs
NONE. No row above ships as a stub.

## Required substrate (not a feature)
Features 1, 4, 5, 6 all write to one `data_quality_findings` table keyed
(surface, vintage, isin|term, check_name, severity). `--strict` on any emit path must refuse a
vintage carrying an unexplained finding. Per-command checks nobody reads are decoration.

## Promotion-gate note (upstream #4539)
`coverage map` is the ONLY novel feature that writes to the local store, and it is already an
EXECUTABLE LEAF (two words), which is the configuration the patched press requires to clear
`coverage_hollow`. It gets `mcp:local-write=true` (never mis-annotated read-only) and
`pp:happy-args` carrying `--timeout=30m`. Match `--timeout[= ]30m` when gating, never the `=` form alone.
