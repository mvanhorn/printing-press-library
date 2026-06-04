# GISIS CLI Brief

## API Identity
- Domain: Maritime authoritative-registry (ship particulars, casualty records, 28 IMO public modules total)
- Users: Maritime due-diligence analysts, classification societies, insurers, port state control, journalists/OSINT, shipping intelligence platforms
- Data profile: Authoritative IMO registry data — vessel name/IMO/flag/type/GT, owner/operator, classification society, casualty records, all sourced directly from IMO member state submissions
- Surface kind: ASP.NET WebForms server-rendered site (`.aspx`), HTML scraping rather than REST

## Reachability Risk
- **High — auth gates all data endpoints.** Anonymous GET to `/Public/SHIPS/*` returns HTTP 200 but body is the WebLogin form (not module content). Confirmed against `/Public/SHIPS/Default.aspx`, `/Public/SHIPS/ShipDetails.aspx`, `/Public/MCI/Default.aspx`, `/Public/MCI/Search.aspx`.
- **Cloudflare Turnstile** on the login flow (`hfTurnstileToken` hidden input on WebLogin.aspx). Programmatic login is impractical; user must log in via real browser.
- **No published rate limits.** ASP.NET WebForms; respect 1 req/2-3s default per execution log.
- robots.txt allows all under `/Public/` except `/Public/Shared/` (static assets only).
- Probe-safe endpoint used (logged-in shape): `GET /Public/SHIPS/ShipDetails.aspx?IMONumber=<imo>` — only requires session cookie; no postback complexity for read-by-IMO.

## Top Workflows
1. **Look up vessel particulars by IMO number** (#1 — drives Vessel MCP `vessel get <imo>` from Phase 3 orchestrator)
2. **Look up vessel particulars by name** (requires `__VIEWSTATE` postback on /SHIPS/Default.aspx — deferred to v0.2)
3. **Browse casualty records by IMO or date range** (MCI module — deferred to v0.2; not captured in HAR)
4. Pull company particulars (owner/operator details from companion module) — secondary
5. Cross-link a ship to its known casualty history — synthesizes 1 + 3

## Table Stakes
From Equasis CLI (`rhinonix/equasis-cli` — the reference UX shape):
- `vessel /imo` lookup → JSON / CSV / table output
- `--json` for agent consumption
- Polite throttle baseline (1s)
- Persistent credential store
- Local SQLite cache of looked-up records

From VesselAPI MCP (closest MCP competitor on coverage):
- Vessels, positions, ownership, inspections, casualties as separate MCP tools
- Identity-by-IMO is the foundational tool

## Data Layer
- Primary entities:
  - `ship` (IMO, name, flag, type, gross_tonnage, deadweight, year_built, classification_society, registered_owner, operator, status, etc.)
  - `company` (deferred — companion module not captured in v1)
  - `casualty` (deferred — MCI module not captured in v1)
- Sync cursor: none (lookup-only — no bulk feed; sync is per-IMO on demand)
- FTS/search: Full-text on `ship.name`, `ship.registered_owner`, `ship.operator` for local browse after lookups accumulate

## Codebase Intelligence
- Source: HAR capture of authenticated user session (254 entries, 182 to gisis.imo.org)
- Auth: ASP.NET session cookie (`ASP.NET_SessionId` + likely a GISIS-specific form-auth cookie); Brave stripped exact cookie names from HAR, will reconfirm via press-auth login flow
- Data model: Each module is an `.aspx` page; detail pages are GET with query string (e.g. `ShipDetails.aspx?IMONumber=9966233`); search/filter forms are POST with `__VIEWSTATE` + `__EVENTTARGET`
- Rate limiting: none observed; respect 1 req/2-3s self-imposed
- Architecture: Server-rendered HTML, no XHR/JSON API, no REST surface

## User Vision
- Ship Particulars by IMO is the v1 critical path (Phase 3 orchestrator dependency: `vessel get <imo>` must return particulars)
- MCI (Marine Casualties) is "secondary" per execution log — defer to v0.2
- Other 25 GISIS modules added later via `/printing-press-amend` if needed
- Politeness over speed (1 req/2-3s baseline)

## Source Priority
- Single source: gisis.imo.org public modules. No combo CLI; no priority gate.

## Product Thesis
- Name: `gisis-pp-cli` (binary), `GISIS — IMO Global Integrated Shipping Information System CLI`
- Why it should exist:
  - **Greenfield niche.** No existing Go/Python/Node CLI specifically wraps GISIS (verified via GitHub search). VesselAPI MCP wraps commercial AIS data; nothing fronts the authoritative IMO registry.
  - **Agent-native authoritative lookup.** Other vessel CLIs wrap commercial intel; this is the canonical IMO ship-particulars source, suitable as the spine of the Vessel MCP project.
  - **Local cache + cross-reference.** Once we have a SQLite cache of looked-up ships, downstream MCPs (Equasis, GFW, AIS Stream) can cross-link without re-fetching, and the orchestrator can issue `vessel get` cheaply.

## Build Priorities
1. **`ship get <imo>` (P0)** — GET /SHIPS/ShipDetails.aspx?IMONumber=<imo> → HTML parse → JSON envelope with all standard particulars fields. This is the Phase 3 orchestrator dependency.
2. **Cookie auth via press-auth companion (P0)** — recommend press-auth install; spec declares `login_url`, `login_complete_selector`, `jwt_carrier_cookie` so `auth login --chrome` is one-command for the user
3. **Local SQLite cache (P0)** — `ships` table with IMO as PK; `sync` no-op (lookup-only API); future commands query cache
4. **`doctor` (P0)** — verifies cookies present and session live (test ping to /SHIPS/Default.aspx, check for WebLogin redirect)
5. **MCP scaffolding (P0)** — auto-generated by press; orchestrator can call `gisis_ship_get` as an MCP tool
6. **`ship search <name>` (P1 — deferred to v0.2)** — needs `__VIEWSTATE` POST round-trip; defer
7. **`casualty *` from MCI module (P1 — deferred to v0.2)** — needs second HAR capture
8. **Other 25 modules** — via `/printing-press-amend` later
