# FedEx CLI Absorb Manifest

## Summary

- **51 absorbed endpoints** across 14 FedEx REST APIs (Ship, Rate, Track, Address Validation, Pickup, Locations, Postal Code, Service Availability, Global Trade, End-of-Day Close, Open Ship, Consolidation, Freight LTL, OAuth)
- **12 transcendence features** that no FedEx wrapper has today (rate-shop, bulk-CSV-ship, track-diff, address-cache, multi-account, ETD-aware ship, end-of-day manifest, archive search, webhook receiver, etc.)
- **Total: 63 commands**

**Why a CLI shaped like this beats every existing tool:** Every direct competitor (python-fedex, happyreturns/fedex, asrx/go-fedex-api-wrapper, fedex-nodejs) is SOAP-only and goes dark when FedEx retires SOAP on June 1, 2026. The lone REST-aware option (karrio) is Python and multi-carrier — no FedEx-specific affordances. The lone FedEx MCP server (markswendsen-code/mcp-fedex) wraps Playwright scraping the public tracking page, not the API. We ship a single static Go binary with the full FedEx REST surface plus a SQLite ledger that compounds across calls.

## Source Tools Surveyed

| Tool | Lang | Status | Features Absorbed |
|---|---|---|---|
| python-fedex (python-fedex-devs) | Python | SOAP-only, last release 2020 | Track, Rate, Ship, AddressValidation, Pickup, Locator, ServiceAvailability methods |
| happyreturns/fedex | Go | SOAP-only, 3 stars | Track, GroundClose, Pickup |
| asrx/go-fedex-api-wrapper | Go | SOAP-only | Track |
| karrio (purplship) | Python | Multi-carrier REST | Rate-shop pattern, normalized response shape |
| markswendsen-code/mcp-fedex | TS | Playwright scraping | Track, Rate, Locations, Pickup tools (web-scraped) |
| Pipedream FedEx MCP | hosted | SaaS | Track, Rate, Ship |
| FedEx Postman (devrel) | — | Vendor-curated | All 51 endpoints, request shapes, sandbox examples |

## Absorbed Features (51)

### Address (1)
| # | Feature | Endpoint | Source |
|---|---|---|---|
| 1 | Validate one or more addresses | POST /address/v1/addresses/resolve | python-fedex, karrio, FedEx Postman |

### Rate (2)
| # | Feature | Endpoint | Source |
|---|---|---|---|
| 2 | Quote rates for a shipment (Express/Ground) | POST /rate/v1/rates/quotes | python-fedex, karrio, mcp-fedex, FedEx Postman |
| 3 | Quote rates for freight (LTL) | POST /rate/v1/freight/rates/quotes | FedEx Postman |

### Ship (6)
| # | Feature | Endpoint | Source |
|---|---|---|---|
| 4 | Create shipment + label | POST /ship/v1/shipments | python-fedex, karrio, FedEx Postman |
| 5 | Validate shipment package | POST /ship/v1/shipments/packages/validate | FedEx Postman |
| 6 | Get shipment results (after async) | POST /ship/v1/shipments/results | FedEx Postman |
| 7 | Create tag (return label / Ground call tag) | POST /ship/v1/shipments/tag | python-fedex, FedEx Postman |
| 8 | Cancel shipment | PUT /ship/v1/shipments/cancel | python-fedex, FedEx Postman |
| 9 | Cancel tag | PUT /ship/v1/shipments/tag/cancel | FedEx Postman |

### Track (6)
| # | Feature | Endpoint | Source |
|---|---|---|---|
| 10 | Track by tracking number(s) | POST /track/v1/trackingnumbers | python-fedex, karrio, mcp-fedex, FedEx Postman |
| 11 | Track by reference number | POST /track/v1/referencenumbers | FedEx Postman |
| 12 | Track by Transportation Control Number | POST /track/v1/tcn | FedEx Postman |
| 13 | Track associated shipments (multi-piece) | POST /track/v1/associatedshipments | FedEx Postman |
| 14 | Get tracking documents (signature POD) | POST /track/v1/trackingdocuments | FedEx Postman |
| 15 | Configure tracking notifications/webhook | POST /track/v1/notifications | FedEx Postman |

### Pickup (6)
| # | Feature | Endpoint | Source |
|---|---|---|---|
| 16 | Schedule Express/Ground pickup | POST /pickup/v1/pickups | python-fedex, mcp-fedex (hand-authored from docs; mislabeled in collections) |
| 17 | Check Express/Ground pickup availability | POST /pickup/v1/pickups/availabilities | python-fedex (hand-authored) |
| 18 | Cancel Express/Ground pickup | PUT /pickup/v1/pickups/cancel | python-fedex (hand-authored) |
| 19 | Schedule freight pickup | POST /pickup/v1/freight/pickups | FedEx Postman |
| 20 | Check freight pickup availability | POST /pickup/v1/freight/pickups/availabilities | FedEx Postman |
| 21 | Cancel freight pickup | PUT /pickup/v1/freight/pickups/cancel | FedEx Postman |

### Locations (1)
| # | Feature | Endpoint | Source |
|---|---|---|---|
| 22 | Find FedEx pickup/dropoff locations | POST /location/v1/locations | mcp-fedex, FedEx Postman |

### Postal Code (1)
| # | Feature | Endpoint | Source |
|---|---|---|---|
| 23 | Validate postal code (serviceable?) | POST /country/v1/postal/validate | FedEx Postman |

### Service Availability (3)
| # | Feature | Endpoint | Source |
|---|---|---|---|
| 24 | Get available services for an OD pair | POST /availability/v1/packageandserviceoptions | FedEx Postman |
| 25 | Get special service options (e.g., DG, alcohol) | POST /availability/v1/specialserviceoptions | FedEx Postman |
| 26 | Get transit times | POST /availability/v1/transittimes | FedEx Postman |

### Global Trade (1)
| # | Feature | Endpoint | Source |
|---|---|---|---|
| 27 | Get regulatory details (HS codes, restrictions) | POST /globaltrade/v1/shipments/regulatorydetails/retrieve | FedEx Postman |

### End of Day Close (2)
| # | Feature | Endpoint | Source |
|---|---|---|---|
| 28 | Submit Ground end-of-day close (manifest) | POST /ship/v1/endofday/ | happyreturns/fedex, FedEx Postman |
| 29 | Re-submit / modify end-of-day close | PUT /ship/v1/endofday/ | FedEx Postman |

### Open Ship (multi-piece progressive) (9)
| # | Feature | Endpoint | Source |
|---|---|---|---|
| 30 | Create open shipment (multi-piece base) | POST /ship/v1/openshipments | FedEx Postman |
| 31 | Add packages to open shipment | POST /ship/v1/openshipments/packages | FedEx Postman |
| 32 | Retrieve open shipment | POST /ship/v1/openshipments/retrieve | FedEx Postman |
| 33 | Get open shipment results | POST /ship/v1/openshipments/results | FedEx Postman |
| 34 | Retrieve specific package | POST /ship/v1/openshipments/packages/retrieve | FedEx Postman |
| 35 | Modify open shipment | PUT /ship/v1/openshipments | FedEx Postman |
| 36 | Modify open shipment package | PUT /ship/v1/openshipments/packages | FedEx Postman |
| 37 | Delete open shipment | PUT /ship/v1/openshipments/delete | FedEx Postman |
| 38 | Delete open shipment package | PUT /ship/v1/openshipments/packages/delete | FedEx Postman |

### Consolidation (9)
| # | Feature | Endpoint | Source |
|---|---|---|---|
| 39 | Create consolidation | POST /ship/v1/consolidations | FedEx Postman |
| 40 | Add shipments to consolidation | POST /ship/v1/consolidations/shipments | FedEx Postman |
| 41 | Confirm consolidation (locks rates) | POST /ship/v1/consolidations/confirmations | FedEx Postman |
| 42 | Get consolidation confirmation results | POST /ship/v1/consolidations/confirmationresults | FedEx Postman |
| 43 | Get consolidation results | POST /ship/v1/consolidations/results | FedEx Postman |
| 44 | Retrieve consolidation | POST /ship/v1/consolidations/retrieve | FedEx Postman |
| 45 | Modify consolidation | PUT /ship/v1/consolidations | FedEx Postman |
| 46 | Delete consolidation | PUT /ship/v1/consolidations/delete | FedEx Postman |
| 47 | Delete consolidated shipment | PUT /ship/v1/consolidations/shipments/delete | FedEx Postman |

### Freight LTL Ship (1)
| # | Feature | Endpoint | Source |
|---|---|---|---|
| 48 | Create freight shipment | POST /ship/v1/freight/shipments | FedEx Postman |

### OAuth (1)
| # | Feature | Endpoint | Source |
|---|---|---|---|
| 49 | Mint access token (client_credentials) | POST /oauth/token | (foundation; hand-built; FedEx Postman) |

### Doctor / Health (auto-generated)
| # | Feature | Source |
|---|---|---|
| 50 | Doctor command — verify auth, sandbox/prod, BAG status | (generator + Phase 3 augmentation) |
| 51 | Auth status — show env vars detected and expiry | (generator) |

## Transcendence Features (12)

These are the commands that make a power user say "I need this." None of the surveyed competitors offer any of them. All hinge on the local SQLite store (compound use), the multi-account config, or both.

| # | Feature | Command | Why only we can do this |
|---|---|---|---|
| T1 | Rate-shop across service types ranked by cost/transit | `fedex rate shop --from 90210 --to 10001 --weight 5lb` | Quotes Priority/Standard/Ground/Home/Express/Ground Economy in parallel, ranks lowest-cost-per-day, persists to ledger |
| T2 | Bulk-ship from CSV with adaptive rate limiting | `fedex ship bulk --csv orders.csv --service GROUND` | cliutil.AdaptiveLimiter respects 1 req/s burst limit; resumable with `--continue-from`; per-row PASS/FAIL ledger |
| T3 | Track-diff (only what changed since last poll) | `fedex track diff --since 1h` | Local event store dedupes against last poll; emits only new milestones |
| T4 | Track watch (long-poll daemon) | `fedex track watch --tracking 12345 --interval 10m` | Background polling with state file; emits new events to stdout / webhook / file |
| T5 | Address-validation cache | `fedex address validate --cache` | SHA-256-keyed local cache; same lookup never hits API twice; saves money on repeat addresses |
| T6 | Multi-account config | `fedex --account warehouse-east ship create ...` | `~/.config/fedex/accounts.toml` with named profiles; flag selects which credentials/account-number to use |
| T7 | ETD-aware ship (one-shot Electronic Trade Documents) | `fedex ship etd --invoice invoice.pdf --orig CN --dest US ...` | Uploads document → captures docId → stitches into create shipment call. Native API needs 3 separate calls. |
| T8 | End-of-day manifest report | `fedex manifest --date 2026-05-02 --pdf` | Pulls every shipment created today from local store, formats as printable manifest, optionally calls EOD close |
| T9 | Shipment archive search (FTS) | `fedex archive "warehouse 47" --service GROUND` | SQLite FTS5 over recipient name, address, reference, tracking number, service type |
| T10 | SQL composability | `fedex sql "select count(*) from shipments where created_at > date('now','-7 days')"` | Direct SQLite query against the local archive |
| T11 | Webhook receiver (HMAC-verified push notifications) | `fedex webhook serve --port 9090` | Receives FedEx push notifications, verifies signature, stores events to local store, exits 0 |
| T12 | Doctor with FedEx-specific checks | `fedex doctor` | Verifies auth, sandbox vs prod, BAG approval status (via probe), label format compatibility, account number format, time-skew |

## Stubbed Features (none)

Every feature above ships fully implemented or is auto-generated by the printing-press from the spec. No `(stub)` rows.

## Why this CLI exists

Three forces converge over the next 30 days:

1. **June 1, 2026 SOAP retirement** — every existing FedEx wrapper goes dark. The migration window is closing.
2. **No REST-native FedEx-specific Go CLI exists.** karrio is Python multi-carrier; mcp-fedex is Playwright scraping; everyone else is SOAP. The field is wide open.
3. **Compounding local data.** No competitor has a SQLite ledger of shipments + rate quotes + tracking events that compounds across calls. That ledger is the foundation for rate-shop history, address cache, archive search, and end-of-day manifest — all of which are impossible without it.
