# Phase 4.85 — agentic output review (immoweb-pp-cli)

Status: WARN (5 warnings) — all 5 fixed in place, then re-verified live and by shipcheck (PASS 7/7, 89/100).

| # | Check | Finding | Resolution |
|---|---|---|---|
| 1 | output-matches-query-intent | Commune names resolved to Immoweb "(all localities)" sets: Ixelles = 1000+1050 (740/1361 rows were City of Brussels), Saint-Gilles = 1050+1060; `market ixelles saint-gilles` compared near-identical data. | Brussels-19 ownership table (`immo.BrusselsPostcodes`, `CommuneMatch`): Ixelles → 1050, Saint-Gilles → 1060, City of Brussels → 1000/1020/1120/1130. `market` warns when requested communes share postcodes. Live: Ixelles 621 rentals (1050), Saint-Gilles 209 (1060), medians 18 vs 17 €/m². |
| 2 | ranking-makes-sense | Kots and per-room lets compared as whole flats; triage ranked a KOT 2nd; Liège 4+ yield 1.58%. | `immo.IsRoomLet` (subtype KOT, rent < 175 €/bedroom for 3+ bedrooms, shared-housing title words). Excluded from market, yield, triage (`--include-rooms` opt-in); deal compares rooms only with rooms. Live: Liège 4+ yield 5.28%, 86 room lets excluded; triage excludes 19 in Ixelles. |
| 3 | silent-empty-result | `drops --commune <never pulled>` returned `[]` silently. | stderr hint naming the `pull` command when the area has no stored listings; Brussels communes filter by their postcodes. |
| 4 | format-bugs | Raw locality strings ("1050 IXELLES", "Bruxelles  1"); bedroom bands sorted as text. | `immo.CleanLocality` at ingest and display (`locLabel`); bands ordered studio,1,2,3,4+,unknown. Language variants (Bruxelles/Brussel) are advertiser text and stay. |
| 5 | output-matches-feature-promise | `market` promised days listed but search results carry no creationDate. | Market help/short/docs no longer promise it unconditionally; output note explains the missing median and how to get dates (deal/show/triage --enrich). Triage freshness falls back to the last-update date, labelled as such. |
