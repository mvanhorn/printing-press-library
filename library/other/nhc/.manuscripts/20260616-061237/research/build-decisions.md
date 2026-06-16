# nhc-pp-cli — Locked Build Decisions

Authoritative spec for the NHC hurricane CLI. Derived from: the user's pasted NHC
integration notes, the (de-bunked) "Authoritative API Guide" PDF, live endpoint
verification, and explicit user decisions. This file is the source of truth that
feeds the Printing Press generation and the final repo.

## Identity
- **CLI binary:** `nhc-pp-cli` · **MCP server:** `nhc-pp-mcp` · **Claude Code skill** + **OpenClaw skill**
- **Repo:** `github.com/abe238/nhc-pp-cli` (public)
- **Local build dir:** `/Users/abediaz/ghostex/chats/nhc-pp`
- **Mission:** give AI agents the most *credible, real-time* National Hurricane Center
  information, text-first, with link-outs for deeper/GIS data.

## Philosophy (the anti-PDF spine)
The PDF that seeded this project claimed it verified endpoints returned `200 OK` while
every captured sample was actually a DNS failure. **Every source in this CLI is verified
against live HTTP + real fixtures, never asserted.** Credibility is the product.

## Scope (text-first, confirmed by user)
- **IN:** NHC informational products (text), some images, NWS tropical alerts.
- **OUT (link-out only):** GIS / ArcGIS MapServer polygons, PostGIS ingestion, shapefiles.
  The CLI surfaces the MapServer URLs so users "can always link out for more info."
- **Basins:** ALL — Atlantic (`at`), Eastern Pacific (`ep`), Central Pacific (`cp`).

## Verified data sources
| Source | URL / pattern | Use |
|---|---|---|
| Active storms index | `https://www.nhc.noaa.gov/CurrentStorms.json` | spine; per-storm products + graphic URLs |
| Public Advisory (TCP) | from CurrentStorms / `archive/<yr>/<id>/...public...` | current coords, winds, watches/warnings |
| Forecast Discussion (TCD) | from CurrentStorms / `...discus...` | meteorologist narrative + confidence |
| Forecast/Marine Advisory (TCM) | from CurrentStorms / `...fstadv/marine...` | detailed forecast points |
| Tropical Weather Outlook (TWO) | text `MIATWOAT` / `MIATWOEP` / **`HFOTWOCP`** (NOT `MIATWOCP`, 404); graphics `two_atl` / **`two_pac`** (NOT `two_epac`) / `two_cpac` `_2d0/7d0.png` | **always-on** value when 0 storms |
| RSS index | `index-at.xml`, `index-ep.xml`, `index-cp.xml` | discovery / fallback |
| NWS tropical alerts | `https://api.weather.gov/alerts/active?event=<...>` | Hurricane + Tropical Storm + Storm Surge × Warning/Watch |
| GIS (LINK-OUT ONLY) | `https://mapservices.weather.noaa.gov/tropical/rest/services/tropical/NHC_tropical_weather/MapServer` | cite layer ids for "more info" |

- **Auth:** descriptive `User-Agent` required (NWS rejects blank). Use
  `nhc-pp-cli/<ver> (github.com/abe238/nhc-pp-cli)`.
- **Quiet-season contract:** `CurrentStorms.json` → `{activeStorms:[]}` (verified live today,
  2026-06-15). With zero active storms the CLI MUST still be useful: clean "no active storms"
  result that points to the Tropical Weather Outlook.

## Command surface (agent-native)
- `storms` — list active storms across basins (id, name, classification, intensity, position, movement).
- `storm <id>` — full detail for one storm + product URLs + graphic URLs.
- `advisory <id> --type tcp|tcd|tcm` — fetch the actual product text (clean body, not HTML).
- `outlook --basin at|ep|cp` — Tropical Weather Outlook text + graphic links (works when quiet).
- `alerts [--area <ST>]` — active NWS tropical alerts (full event set), text + areas.
- `graphics <id>` — image URLs; **`--download <dir>`** saves PNGs; **`--open`** opens in default viewer.
- `gis-links <id>` — print ArcGIS MapServer query URLs (link-out, not ingested).
- `brief [--basin ...]` — **compound:** one call returns JSON bundle (active storms + TCP/TCD +
  outlook + alerts); **`--markdown`** also renders a human situational briefing.
- `credits` / `about` — version, sources, the NHC thank-you, and the disclaimer.

### Output
- **JSON-first** (default for agents / `--json`), plus human/markdown mode. Token-efficient.
- Local SQLite cache where it helps (press convention) so compound queries are fast.

## Tone & the NHC gratitude note (user-requested)
- The skill should read as **genuinely trying to help** — warm, calm, useful to people who may
  be in harm's way.
- **Thank-you to the humans at the National Hurricane Center** — the forecasters, meteorologists,
  hurricane hunters, and support staff who sacrifice so much for the benefit of so many. Place it:
  1. Prominently near the top of the **README**.
  2. In the **Claude Code / OpenClaw skill** description text.
  3. As a dedicated **`credits`** command.
  4. As a short footer line in **`brief --markdown`** output.
- **Disclaimer (everywhere the thank-you appears):** this is an *unofficial* tool; the NHC / NWS
  is the authoritative source; in an emergency follow official watches, warnings, and evacuation
  orders from local authorities.

## Build approach (hybrid, user-chosen)
1. Dynamic workflow verifies live endpoints + captures real 2024 storm fixtures (Beryl/Helene/Milton)
   → `research/` docs + `fixtures/` (the independent acceptance tests).
2. `/printing-press` generates the Go CLI + MCP + skills, fed this spec.
3. Dynamic workflow runs the fixture-based acceptance tests against the generated CLI
   (storm-path commands MUST work against a real storm, not just the empty live case).
4. Publish: (a) into the local `nhc-pp` folder, (b) public repo `abe238/nhc-pp-cli`,
   (c) PR to `mvanhorn/printing-press-library`.
