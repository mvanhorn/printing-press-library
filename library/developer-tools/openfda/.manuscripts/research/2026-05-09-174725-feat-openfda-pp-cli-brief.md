# OpenFDA CLI Brief

## API Identity
- **Domain:** Drug/device/food safety — adverse events, recalls, product labeling, regulatory data
- **Users:** Pharmacovigilance researchers, medical device professionals, journalists, patients/caregivers, legal/insurance analysts, public health academics
- **Data profile:** 21 REST endpoints across 6 categories (Drug, Device, Food, Animal/Veterinary, Tobacco, Other). 4.9M+ drug adverse event reports since 2003, 67K+ drug labels, device MAUDE reports, recalls across all categories. All public, no auth required. Elasticsearch-backed with `search=`, `count=`, `limit=`, `skip=` query params.
- **Base URL:** `https://api.fda.gov`
- **Rate limits:** Without key: 40 req/min, 1K/day. With key: 240 req/min, 120K/day. Key is free.

## Reachability Risk
- **None.** Public API, no auth required, no bot protection, no rate-limit issues for reasonable use. The official FDA/openfda GitHub repo has zero open issues about API access.

## Endpoint Map (21 total)

### Drug (6)
| Endpoint | Path | Description |
|----------|------|-------------|
| Adverse Events | `/drug/event.json` | FAERS — side effects, medication errors, product quality (4.9M+ reports) |
| Product Labeling | `/drug/label.json` | Prescribing info, black box warnings, indications (67K+) |
| NDC Directory | `/drug/ndc.json` | National Drug Code directory |
| Recall Enforcement | `/drug/enforcement.json` | Drug recall reports |
| Drugs@FDA | `/drug/drugsfda.json` | Approved drugs since 1939 |
| Drug Shortages | `/drug/shortages.json` | Current and historical shortages |

### Device (9)
| Endpoint | Path | Description |
|----------|------|-------------|
| Adverse Events | `/device/event.json` | MAUDE/MDR device adverse events |
| 510(k) Clearances | `/device/510k.json` | Substantial equivalence clearances |
| Classification | `/device/classification.json` | Device names, product codes, classes |
| Recall Enforcement | `/device/enforcement.json` | Device recall reports |
| Premarket Approval | `/device/pma.json` | Class III PMA decisions |
| Recalls | `/device/recall.json` | Device recall actions |
| Registration/Listing | `/device/registrationlisting.json` | Manufacturer registrations |
| Unique Device ID | `/device/udi.json` | GUDID device identification |
| COVID-19 Serology | `/device/covid19serology.json` | Serology test evaluations |

### Food (2)
| Endpoint | Path | Description |
|----------|------|-------------|
| Recall Enforcement | `/food/enforcement.json` | Food recall reports |
| CAERS Reports | `/food/event.json` | Food/supplement adverse events |

### Animal & Veterinary (1)
| Endpoint | Path | Description |
|----------|------|-------------|
| Adverse Events | `/animalandveterinary/event.json` | Animal drug adverse events |

### Tobacco (1)
| Endpoint | Path | Description |
|----------|------|-------------|
| Problem Reports | `/tobacco/problem.json` | Tobacco product problems |

### Other (2)
| Endpoint | Path | Description |
|----------|------|-------------|
| NSDE | `/other/nsde.json` | Non-standardized drug entities |
| Substance Data | `/other/substance.json` | Substance information |

## Query Interface (shared across all endpoints)
- `search=field:term` — Elasticsearch query syntax
- `search=field:term+AND+field:term` — boolean AND
- `search=field:term+field:term` — boolean OR (implicit)
- `count=field.exact` — faceted counts on exact field values
- `limit=N` — max 1000 per request
- `skip=N` — pagination offset, max 25000
- `sort=field:asc|desc` — sort results
- `.exact` suffix — match whole phrases instead of individual words

## Top Workflows
1. **Drug safety check** — "Is my medication safe?" Search adverse events by drug name, see reaction frequencies
2. **Recall monitoring** — "What's being recalled?" Check recent recalls across drug/device/food
3. **Signal detection** — Track adverse event counts over time for a drug; detect acceleration
4. **Drug comparison** — Compare adverse event profiles of similar drugs (e.g., two statins)
5. **Manufacturer audit** — Check a company's full recall and adverse event history

## Table Stakes (from competing tools)
- Search drug adverse events by drug name, reaction, date range, seriousness (MCP has this)
- Search drug labels by brand/generic name, ingredient, indication (MCP has this)
- Search NDC directory by product code, name, dosage form (MCP has this)
- Search recalls by firm, classification, status, date range (MCP has this)
- Search 510(k) clearances by device name, applicant, product code (MCP has this)
- Search device adverse events by device name, manufacturer, event type (MCP has this)
- Pagination across all endpoints (MCP has this)
- Rate-limit-aware requests (MCP has this)

## Gaps in Existing Tools
- **No food, animal, tobacco, or "other" endpoint coverage** (MCP only covers drug + device)
- **No offline caching** — every query hits the API
- **No time-series analysis** — can't track how adverse event counts change over time
- **No cross-dataset correlation** — can't join recalls with adverse events
- **No watchlist/alerting** — can't monitor specific drugs/devices
- **No full-text search across synced data**
- **No count distribution caching** — can't compare "top reactions last quarter vs this quarter"
- **No CLI at all** — no scriptable, composable, agent-native interface

## Data Layer
- **Primary entities:** adverse_events (drug + device + food + animal), recalls (drug + device + food), labels, ndc, drugsfda, shortages, device_510k, device_classification, device_pma, device_recall, device_registration, device_udi, tobacco_problems, substances
- **Sync cursor:** `receiptdate` for adverse events, `report_date` for recalls, `effective_time` for labels. Incremental sync by date range.
- **FTS/search:** Full-text across all synced entities. Drug names, reactions, manufacturer names, product descriptions.

## Product Thesis
- **Name:** `openfda-pp-cli` — "The FDA safety data terminal"
- **Why it should exist:** OpenFDA has the richest public drug safety dataset in the world, but its web UI only supports single-query searches with no historical context. A CLI with local SQLite cache transforms this into a personal pharmacovigilance workstation: track drugs over time, detect signals, compare drugs, monitor recalls, and correlate adverse events with regulatory actions — all offline, all composable, all agent-native.

## Build Priorities
1. **Data layer for ALL 21 endpoints** — sync, store, search, SQL
2. **Full endpoint coverage** — every search parameter the MCP server supports, plus food/animal/tobacco/other that nobody covers
3. **Temporal analysis** — trend, signal, compare commands that only work with cached data
4. **Cross-entity correlation** — recalls that follow adverse event spikes
5. **Watchlist** — monitor specific drugs/devices/manufacturers
