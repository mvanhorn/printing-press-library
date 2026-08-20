# GISIS Landscape Research — 2026-05-27

## Existing GISIS scrapers/wrappers

Effectively none mature. Tangential repos:
- `followthemoney/vessel_research` (9 stars, 4 commits, GFW-focused; namedrops GISIS)
- `snosan-tools/srr-mrcc-france` (one-off 2018 SAR extract, not Ships/MCI)
- `interreg-speed/IMO-VESSEL-NAMES` (static list, not a scraper)

Generic ship scrapers target VesselFinder, not GISIS.
**Conclusion: greenfield for GISIS-specific tooling.**

## MCP competition for vessel data

- `vessel-api/vesselapi-mcp` — 23 tools: vessels, positions, ETA, ownership,
  emissions, inspections, casualties. Wraps commercial VesselAPI.
- `tools-mcp/vessel-traffic-mcp` — identity lookup, AIS positions, port calls
- `garrettXu/ShipXY` — Chinese AIS coverage
- Kpler announced enterprise MCP for gov/defense

None front GISIS specifically — all wrap commercial AIS/intel APIs. A
GISIS-backed MCP fills a distinct **authoritative-registry** niche.

## Reference design: Equasis CLI

`rhinonix/equasis-cli` (17 stars, v2.0.0 Oct 2025) is the closest UX/auth shape:
- Python + Click + Rich
- Modular client/parser/formatter
- Credentials at `~/.config/equasis-cli/credentials.json`
- 1s throttle + exponential backoff
- Commands: `vessel /imo`, `search /name`, `fleet /company`, batch
- Output: JSON / CSV / table
- Bellingcat lists it in their OSINT toolkit

**This is the UX shape to mirror for gisis-pp-cli.**

## Expected data fields (per Maritimeducation, Marineinsight articles)

Ship Particulars module returns:
- Name, IMO number, flag, ship type, gross tonnage
- Owner / operator, registered owner
- Classification society
- (potentially) keel-laid date, builder, status

Marine Casualties and Incidents module returns:
- Ship details, casualty type, date, location
- Investigation status, narrative
- Excel/CSV export supported on the web UI

These need verification once authenticated browser-sniff captures real responses.

## Known issues / reachability risk

- No GitHub issues about "GISIS 403 / captcha / rate limit" — likely because no
  scrapers big enough to hit limits
- ASP.NET WebForms is fragile: ViewState rotates, session cookies short-lived
- Plan: session refresh on 401/302, polite throttling (mirror Equasis 1s baseline),
  single auth-cookie cache
- Cloudflare Turnstile on login — programmatic login blocked, must use Chrome
- Equasis explicitly forbids bulk harvest in ToS; GISIS Disclaimer should be reviewed
