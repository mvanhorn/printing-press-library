# Novel features brainstorm — immoweb (subagent output, first print)

## Customer model
- Léa — expat relocating to Brussels, renting a 2-bed in Ixelles/Saint-Gilles/Etterbeek < €1,500. Today: three Immoweb email alerts, dozens of tabs, duplicates across overlapping searches, half already under option, no idea if €1,450 is normal. Ritual: daily morning scan + same-day calls to 3–5 agencies. Frustration: alerts fire on everything and rank nothing; best flats go under option before she calls.
- Thomas & Inès — first-time buyers, ~€350k house in Walloon Brabant/Namur. Today: one saved search + spreadsheet; hand-compute €/m²; open each listing for EPC/renovation obligation/cadastral income; "new price" badge without old price; no days-on-market. Ritual: weekend review, shortlist, visits, offer debate. Frustration: judging the price.
- Marc — buy-to-let investor, Liège/Charleroi apartments. Today: Apify/one-shot CSV into spreadsheet; second rental search to eyeball rents; watches viager/public sales; price map is gone. Ritual: Sunday screen of 5–6 postcodes, yield estimate, motivated-seller hunt. Frustration: no join of rents to sale prices; repeated cuts/stale listings invisible.
- Sophie — independent agent / local analyst (Leuven + Brussels periphery). Today: types filters into the count widget commune by commune; spreadsheet of competitor listings and disappearances. Ritual: Monday market snapshot. Frustration: no market statistics since the price map died; misses disappearances between snapshots.

## Candidates (pre-cut)
C1 market (keep) · C2 compare (merge into market) · C3 deal (keep) · C4 comps (section of deal) · C5 history (section of deal) · C6 drops (keep) · C7 lingering (column of drops; `stale` reserved) · C8 triage (keep; deterministic reframe of LLM relevance scoring) · C9 digest (watch + triage) · C10 yield (keep) · C11 epc-gap (market --by epc + deal line) · C12 cost (unverifiable tax tables, harmful if wrong) · C13 relisted (heuristic, no evidence) · C14 agencies (analytics --group-by covers counts; niche) · C15 hot (one detail call per listing, throttle risk; signal in deal/triage --enrich) · C16 gone (watch + market column + deal comparables).

## Survivors and kills
### Survivors
| # | Feature | Command | Score | Buildability | Persona |
|---|---------|---------|-------|--------------|---------|
| 1 | Commune market snapshot (+compare) | market | 10/10 (D3 P3 F2 R2) | hand-code | Sophie, Léa |
| 2 | Listing deal verdict | deal | 9/10 (D3 P3 F2 R1) | hand-code | Thomas & Inès |
| 3 | Saved-search triage | triage | 9/10 (D3 P3 F2 R1) | hand-code | Léa |
| 4 | Price-cut leaderboard | drops | 8/10 (D3 P2 F2 R1) | hand-code | Marc, Thomas & Inès |
| 5 | Rental yield estimator | yield | 7/10 (D2 P2 F2 R1) | hand-code | Marc |

Pass-3 answers: all five are weekly-or-daily rituals for their persona, none is a single-endpoint wrapper (each needs local SQLite joins or cross-transaction joins), all Long descriptions reference surviving siblings (market, deal, yield, watch).

### Killed candidates
| Feature | Kill reason | Closest surviving sibling |
|---|---|---|
| compare | Same data/output as market with several communes | market |
| comps | Section of deal | deal |
| history | Thin view already inside deal | deal |
| lingering | Column/filter of drops; `stale` is reserved | drops |
| digest | watch across searches + triage ranking | triage |
| epc-gap | market --by epc + deal EPC line | market |
| cost | Buyer-dependent regional tax rules, unverifiable, harmful if wrong | yield |
| relisted | Heuristic matching, no evidence of demand | deal |
| agencies | analytics --group-by covers counts; niche | market |
| hot | One detail call per listing (throttle/DataDome risk) | deal |
| gone | Covered by watch + market + deal | market |
