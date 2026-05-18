# PVGIS CLI Absorb Manifest

## Source landscape
- **pvgis-mcp (local Python MCP)** — ground truth. 7 tools, file cache, field-alias tolerance. The CLI must equal-or-beat every feature.
- **PVGIS official web UI** (https://re.jrc.ec.europa.eu/pvg_tools/en/) — interactive map + form, exports to CSV/PDF/JSON. Not scriptable; no API parity to claim beyond what the JRC API itself exposes.
- **pvlib-python** — large research library that includes a PVGIS adapter (`pvlib.iotools.get_pvgis_hourly`, `get_pvgis_tmy`, `get_pvgis_horizon`). The adapter wraps the same five endpoints; the CLI matches its surface but adds offline cache + agent-native JSON.
- **No widespread CLI competitor exists.** Public-search returns no first-class single-binary CLI; just notebook examples and Python scripts.

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | Monthly irradiance — H(h), H(i), H(opt), T2m | pvgis-mcp `get_irradiance` / pvlib `get_pvgis_monthly` | `pvgis radiation monthly --lat --lon --tilt --azimuth` | SQLite cache, `--json`, `--select`, `--csv`, field-alias robust |
| 2 | Monthly PV production | pvgis-mcp `get_production_timeseries (hourly=false)` / pvlib equivalent | `pvgis production monthly --lat --lon --pnom --system-loss --tilt` | Cache, dry-run, agent-native |
| 3 | Hourly PV production timeseries | pvgis-mcp `get_production_timeseries (hourly=true)` / pvlib `get_pvgis_hourly` | `pvgis production hourly --lat --lon --pnom --system-loss --startyear --endyear` | Cache (30-day TTL for time series), stream JSON, `--limit` |
| 4 | TMY hourly weather | pvgis-mcp `get_weather_data` / pvlib `get_pvgis_tmy` | `pvgis weather tmy --lat --lon` | Cache (30-day), summary view + full hourly |
| 5 | Horizon profile | pvgis-mcp `calculate_shading` / pvlib `get_pvgis_horizon` | `pvgis horizon profile --lat --lon` | Cache, `--json` array |
| 6 | Obstacle-based local shading heuristic | pvgis-mcp `calculate_shading` | `pvgis horizon shading --lat --lon --obstacle-height --obstacle-distance --obstacle-azimuth` | Same heuristic, plus terrain horizon merge |
| 7 | Solar factsheet (horizontal + 35° + optimal) | pvgis-mcp `get_solar_factsheet` | `pvgis radiation factsheet --lat --lon` | Three-call orchestration, cached, single JSON |
| 8 | Optimal tilt sweep | pvgis-mcp `pv_module_optimal_tilt` | `pvgis production optimal-tilt --lat --lon --azimuth` | Cache, configurable step + range |
| 9 | Coverage / database reference | pvgis-mcp `get_pvgis_coverage_area` | `pvgis info coverage` | Static fast path, no API hit |
| 10 | Database choice forcing (SARAH3/NSRDB/ERA5) | pvlib `get_pvgis_*` `--raddatabase` | `--raddatabase` flag on every command | Surface in JSON, store in cache key |

## Transcendence (only possible with our approach)

| # | Feature | Command | Why Only We Can Do This |
|---|---|---|---|
| 1 | Multi-site rank | `pvgis sites rank --input sites.csv --tilt 30 --pnom 5 --system-loss 14` | Batch loop with shared cache; no public PVGIS endpoint ranks N sites in one call. Requires local store. |
| 2 | Tilt/azimuth heatmap | `pvgis production sweep --lat --lon --tilt-min 0 --tilt-max 60 --azimuth-min -90 --azimuth-max 90` | Caches the 2D grid; subsequent interpolation queries (`pvgis production at --tilt X --azimuth Y`) hit local interp, no API call. |
| 3 | Database comparator | `pvgis production compare --lat --lon --pnom 1 --system-loss 14 --tilt 30` | Issues SARAH3 / NSRDB / ERA5 in parallel, joins results, reports delta. Requires local join. |
| 4 | Yield delta vs reference site | `pvgis sites diff --baseline 45,9 --target 41,12 --pnom 5` | Both sites cached; local-side diff, no API call after first sync. |
| 5 | TMY climate fingerprint search | `pvgis weather similar --to 45,9 --within sites.csv --top 5` | Computes a 12-month T/GHI signature for every cached TMY; ranks similarity. Pure local; no PVGIS endpoint supports this. |

## Stubs
None planned. All 5 transcendence features are shippable from the local store; the 10 absorbed features are direct endpoint wraps.

## Prose Showcase (for the user)

We have 10 absorbed features (every PVGIS endpoint × every option the existing Python MCP exposes) plus 5 transcendence features (multi-site rank, tilt/azimuth heatmap, database comparator, site diff, climate fingerprint). The CLI's differentiator vs the Python MCP is the SQLite cache + ability to compose results via `pvgis sql` and to fan out across many sites without re-hitting the API.

Worth knowing before approving:
- Rate limit: PVGIS does not publish one. We add a conservative 1 req/sec adaptive limiter; transcendence commands that fan out (sites rank, db comparator) honor it.
- The Python MCP's `calculate_shading` includes a heuristic obstacle-shading model. We port it as-is — it's a quick estimate, not a substitute for PVSyst.
- v5_3 vs v5_2: we target v5_3 (current default). The CLI exposes `--base-url` for users who need to pin to v5_2.

No risky dependencies, no expensive endpoints (PVGIS is free), no low-confidence ideas.
