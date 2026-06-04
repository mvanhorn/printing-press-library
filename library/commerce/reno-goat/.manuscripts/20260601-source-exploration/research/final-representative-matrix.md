# Reno Goat final representative matrix

Date: 2026-06-04

This matrix was run after the `moen shower valve` and `medicine cabinet` filter fixes. It uses the printed CLI command path with active/auto source routing and search-result offer fallback disabled:

```bash
./reno-goat-pp-cli model-intel "<query>" --sources auto --search-offers=false --json
```

The goal is to prove the renovation-middle pattern through live priced source rows, not through manufacturer-only discovery or generic search snippets.

| Query | Artifact | Rows | Priced rows | Sources | Result |
| --- | --- | ---: | ---: | --- | --- |
| `thermostat` | `final-matrix/model-intel-thermostat-response.json` | 4 | 4 | Sylvane | Pass: HVAC thermostat rows only. |
| `floor register` | `final-matrix/model-intel-floor-register-response.json` | 12 | 12 | Rejuvenation, Hardware Hut | Pass: register/louver rows only. |
| `ceiling fan` | `final-matrix/model-intel-ceiling-fan-response.json` | 12 | 12 | 1000Bulbs, Bees Lighting, Lightology | Pass: ceiling-fan rows only. |
| `linear drain` | `final-matrix/model-intel-linear-drain-response.json` | 12 | 12 | Floor & Decor, KBAuthority, Vintage Tub | Pass: linear/shower-drain rows only. |
| `shower valve` | `final-matrix/model-intel-shower-valve-response.json` | 12 | 12 | FaucetDepot, Floor & Decor, KBAuthority, Vintage Tub | Pass: valve trim, tub/shower valve, and shower-set rows; no tile row. |
| `shower door` | `final-matrix/model-intel-shower-door-response.json` | 12 | 12 | FaucetDepot, Floor & Decor, KBAuthority | Pass: shower-door/screen rows only. |
| `shower head` | `final-matrix/model-intel-shower-head-response.json` | 12 | 12 | FaucetDepot, Floor & Decor, PlumbTile | Pass: shower-head/spray rows only. |
| `medicine cabinet` | `final-matrix/model-intel-medicine-cabinet-response.json` | 12 | 12 | Floor & Decor, IKEA, Lighting New York, PlumbTile | Pass: cabinet/mirror-cabinet rows only; riser accessory removed. |
| `vanity light` | `final-matrix/model-intel-vanity-light-response.json` | 12 | 12 | Bees Lighting, KBAuthority, Lighting New York | Pass: bathroom vanity light rows only. |
| `picture light` | `final-matrix/model-intel-picture-light-response.json` | 12 | 12 | Bees Lighting, Lighting New York | Pass: picture-light rows only. |
| `cabinet pull` | `final-matrix/model-intel-cabinet-pull-response.json` | 12 | 12 | Hardware Hut | Pass: cabinet-pull rows only. |
| `door hinge` | `final-matrix/model-intel-door-hinge-response.json` | 10 | 10 | Hardware Hut, Rejuvenation | Pass: architectural/screen door hinge rows only. |
| `grab bar` | `final-matrix/model-intel-grab-bar-response.json` | 12 | 12 | Floor & Decor, Vintage Tub | Pass: grab-bar rows only. |
| `soap dispenser` | `final-matrix/model-intel-soap-dispenser-response.json` | 12 | 12 | FaucetDepot, KBAuthority, PlumbTile, Vintage Tub | Pass: soap/lotion dispenser rows only. |

## Outcome

The final matrix supports reprint with documented residual source gaps. The remaining gaps are source breadth and transport/API extraction work, not representative model-intel quality:

- Appliance/local showroom gaps: ABW, ADU catalog, AJ Madison, Costco, Spencer's, Grand Appliance, Warners' Stellian. Best Buy, Abt, and Homewise Appliance are now promoted and remain only as regression/breadth checks.
- Bath showroom gaps: HomePerfect, DecorPlanet, Build.com. Product gaps are shower valves/controls, shower systems, and bathroom vanities from those showroom catalogs. QualityBath, Signature Hardware, and Vintage Tub are now promoted and remain only as regression/breadth checks.
- Hardware breadth: `interior door lever`, `passage door lever`, and `privacy door lever` now return priced Hardware Hut door-entry, passage, privacy, and lever-set rows.
- Big-box gaps: Home Depot, Lowe's, Menards, and Ace-style coverage remain incomplete or not cleanly replayable.
- Transport follow-up: GHST-293 remains open for shared browser-like/impersonating transport and curl/browser parity.
