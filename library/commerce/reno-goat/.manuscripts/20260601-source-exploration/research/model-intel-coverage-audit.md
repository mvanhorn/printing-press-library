# Reno Goat model-intel coverage audit

Date: 2026-06-02

This audit summarizes the current live evidence for Reno Goat's non-appliance `model-intel` coverage. It is intended to answer whether the "beyond appliances" pattern is now strong enough to support a reprint, and what still needs work before claiming the CLI is broadly sourceable.

## Reprint threshold

Reno Goat does not need every blocked local showroom or big-box source to be solved before reprint. It does need:

1. Active readable sources across the renovation "middle": plumbing, electrical, HVAC, flooring, hardware, materials/decor, and appliances.
2. Auto category routing for representative installed selections so users do not need to know source categories.
3. Query-aware filtering that keeps selection-level rows and removes cheap false positives such as appliance fallback, component parts, generic IKEA storage, bulbs, hooks, and kitchen rails.
4. Evidence artifacts showing priced rows from live sources, not hardcoded or manufacturer-only payloads.
5. Explicit release notes for deferred WAF/browser-only sources and incomplete local-showroom breadth.

Current status: requirements 1-5 are satisfied for reprint closeout. The prior `moen shower valve` false positive has been fixed and recaptured, the final representative matrix passed after a `medicine cabinet` accessory filter fix, and `reprint-closeout-notes.md` explicitly documents deferred WAF/browser-only sources and incomplete local-showroom breadth.

## Covered matrix

| Area | Covered queries | Evidence |
| --- | --- | --- |
| HVAC and comfort | `thermostat`, `floor register`, `ceiling fan`, `floor warming`, `towel warmer` | Saved after artifacts return priced HVAC, register, fan, floor-warming, and towel-warmer rows. |
| Electrical and lighting | `recessed light`, `vanity light`, `lighted mirror`, `picture light`, `leviton dimmer` | Saved after artifacts show electrical routing, priced lighting/mirror rows, and Leviton spec enrichment. |
| Plumbing fixtures and systems | `linear drain`, `shower niche`, `shower door`, `shower head`, `shower panel`, `bidet seat`, `pot filler`, `bathroom faucet` | Saved after artifacts return priced plumbing/system rows while filtering trim, handles, broad assemblies, toilets, and appliance fallback. |
| Bath accessories | `grab bar`, `robe hook`, `toilet paper holder`, `towel bar`, `towel ring`, `soap dispenser` | Saved after artifacts return priced bath accessory rows and document why hardware/decor broadening is sometimes avoided. |
| Storage and mirrors | `medicine cabinet`, `lighted mirror` | Saved after artifacts return priced medicine/mirror cabinet and LED/lighted mirror rows while filtering plain mirrors and generic cabinets. |
| Hardware and finish | `cabinet pull`, `door hinge`, `floor register` | Saved after artifacts return priced pulls, architectural door hinges, and registers while filtering screws, stops, slides, cabinet hinges, and generic IKEA rows. |
| Manufacturer/spec enrichment | Broan-NuTone discovery, Moen enrichment, Leviton enrichment | Broan remains discovery-only paired with retailer fallback; Moen/Leviton enrich already-priced rows with product pages and PDFs. |

## Evidence count

The current evidence set has 25 saved `model-intel-*-after-response.json` artifacts plus the final representative matrix under `final-matrix/`. Most return 8-12 rows; known lower-count but acceptable rows are:

- `thermostat`: 4 rows, all relevant HVAC thermostat rows.
- `towel warmer`: 3 rows, all relevant towel warmer/radiator rows.
- `door hinge`: 8 rows, all relevant architectural/screen door hinge rows.
- `leviton dimmer`: 8 rows; this is primarily a spec-enrichment artifact rather than a broad dimmer-shopping proof.

## Reprint closeout items

1. `moen shower valve` is fixed and recaptured. The saved after artifact now returns 12 priced valve/trim/rough-in rows from FaucetDepot, KBAuthority, Moen enrichment, and Vintage Tub, with the prior `Moon River II Honed Marble Tile` false positive removed.
2. `medicine cabinet` is fixed and recaptured in the final matrix. The matrix initially admitted a West Elm medicine-cabinet riser accessory; the query filter now rejects risers/organizer-style rows while preserving cabinet and mirror-cabinet rows.
3. `door lever` is now covered for current model-intel purposes. Auto mode routes `interior door lever`, `passage door lever`, and `privacy door lever` to hardware. Hardware Hut returns real Sure-Loc, Emtek, Brass Accents, Schlage, and Grandeur door-entry, passage, privacy, and lever-set rows with prices and selected spec/install documents.
4. The final representative matrix passed. It covers `thermostat`, `floor register`, `ceiling fan`, `linear drain`, `shower valve`, `shower door`, `shower head`, `medicine cabinet`, `vanity light`, `picture light`, `cabinet pull`, `door hinge`, `grab bar`, and `soap dispenser` with priced live rows and no inspected semantic false positives.

## Deferred source breadth

These are not blockers if documented as deferred source candidates, but they are not solved:

- Local showroom and appliance breadth: ABW, ADU catalog, AJ Madison, Costco, Spencer's, Grand Appliance, Warners' Stellian. PC Richard has been promoted through Demandware product tiles, Appliance Factory has been promoted through AVB/LINQ REST replay, Best Buy has been promoted through Next/Apollo SSR product extraction, Abt has been promoted through HTTP/1.1 search/product-schema extraction, and Homewise Appliance has been promoted through API Gateway/Bloomreach extraction; these sources are no longer deferred.
- Big-box breadth: Home Depot, Lowe's, Menards, Ace-style coverage is still incomplete or not cleanly replayable.
- Bath showroom depth: HomePerfect, DecorPlanet, Build.com. Vintage Tub has been promoted through Searchspring, Signature Hardware has been promoted through Demandware suggestions, and QualityBath has been promoted through React Query hydration extraction; these sources are no longer deferred.
- Transport gap: GHST-293 remains open because hand-authored fan-out helpers still need a shared browser-like/impersonating transport for WAF/challenge-heavy sources.

## Recommendation

Stop expanding into more one-off accessory examples unless a reprint reviewer identifies a representative gap. The pattern is proven beyond appliances. Reprint can proceed with documented residual gaps, and the next source cycles should stay on the priority ledger: appliance/local-showroom breadth plus bath-showroom depth.

1. Use `reprint-closeout-notes.md` as the release/reprint note source.
2. Use `priority-source-gaps.md` as the shortlist for future source extraction.
3. Keep GHST-293 open as the transport follow-up rather than blending it into this printed-CLI reprint.
