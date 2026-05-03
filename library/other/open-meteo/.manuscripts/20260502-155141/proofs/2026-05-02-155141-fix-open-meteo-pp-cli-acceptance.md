# Acceptance Report — open-meteo-pp-cli

**Level:** Full Dogfood
**API:** Open-Meteo (free tier, no auth required)
**Run ID:** 20260502-155141

## Tests
**70/70 passed (100%)**

Coverage by family:
- Sanity: 4/4 (doctor, version, doctor --json, doctor JSON validity)
- forecast: 4/4 (help, happy, JSON fidelity, missing-lat error)
- archive: 4/4 (help, happy, JSON fidelity, missing-dates error)
- marine: 3/3
- air-quality: 3/3
- flood: 3/3
- climate: 3/3
- ensemble: 3/3
- seasonal: 3/3
- elevation: 4/4 (help, single, batch, JSON)
- geocode: 4/4 (search, get, JSON fidelity)
- panel: 5/5 (help, single place, multi-place, JSON, bad-place error)
- compare: 3/3
- normals: 4/4 (help, happy, JSON, missing-month error)
- weather-mix: 4/4 (help, happy, JSON, missing-dates error)
- forecast diff: 4/4 (help, first run, second run, JSON)
- accuracy: 4/4 (help, missing-date error, happy, JSON)
- is-good-for: 8/8 (help, hiking, running, biking, surfing, skiing, unknown-activity, JSON)

## Failures
None.

## Content correctness spot-checks
- Seattle forecast: realistic temperatures (~21-22°C in May)
- Berlin air quality: realistic European AQI (~30-54)
- compare anomaly: produces meaningful classification ("much_higher_than_normal" with proper anomaly + normal_mean)
- Customer-tier routing (with `OPEN_METEO_API_KEY=test-key-xyz`): URL correctly rewrites to `customer-api.open-meteo.com/v1/forecast?apikey=test-key-xyz`
- Free-tier (no env var): URL stays at `api.open-meteo.com/v1/forecast`
- Geocoding integration: place names like "Seattle", "Berlin", "Tokyo", "Half Moon Bay, CA", "Reykjavik, IS", "Paris, FR" all resolve correctly
- Multi-location batch (`panel --place Seattle,Berlin`): single batched API call returns paired per-location results

## Fixes applied during shipcheck (Phase 4 → 4 revisited)
- README/SKILL/research.json narrative: dropped `forecast --place Seattle` and `archive --place Seattle` examples (those commands take `--latitude/--longitude` only); replaced with `panel --place Seattle` for city-name use cases and explicit lat/lon for forecast/archive.
- "Mavericks, CA" example replaced with "Half Moon Bay, CA" (Mavericks isn't a populated place in the gazetteer; Half Moon Bay is the nearby real city).

## Printing Press issues to retro
- **Path validity scored 0/10** in scorecard despite all 9 hosts returning HTTP 200 in reachability and 100% live verification pass. The scorecard's path checker may not handle per-resource `base_url` overrides (multi-host APIs like Open-Meteo). Investigation candidate.
- **MCP token efficiency 4/10** — typical for spec-derived endpoint mirrors with many parameters. Potential candidate for the `mcp:` orchestration field.
- **Place-name parameter integration**: spec-derived commands could optionally accept a `--place` flag with auto-resolution via the Open-Meteo geocoding endpoint when the spec is detected as Open-Meteo. Currently a printed-CLI extension only.

## Gate
**PASS** — proceeding to Phase 5.5 polish, Phase 5.6 promote/archive, Phase 6 publish offer.
