---
name: pp-catastro
description: "Every Catastro feature, plus offline SQLite, bulk RC enrichment, polygon export and change detection no other... Trigger phrases: `consulta el catastro de`, `qué hay en esta referencia catastral`, `descarga las parcelas del municipio`, `enriquece estas direcciones con catastro`, `compara mi padrón contra catastro`, `use catastro`, `run catastro`."
author: "user"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - catastro-pp-cli
---

# Catastro — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `catastro-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install catastro --cli-only
   ```
2. Verify: `catastro-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

`catastro-pp-cli` unifica la Oficina Virtual del Catastro (OVC Callejero + Coordenadas) y los servicios INSPIRE (WMS/WFS/ATOM) en un único binario Go. Añade un store SQLite local con FTS5 sobre parcelas y calles, comandos pipe-friendly para enriquecer listas de RC o direcciones, recorte por polígono arbitrario (no solo por bbox o municipio) y un comando `reconcile` que compara tu tabla local contra Catastro y reporta drift.

## When to Use This CLI

Úsalo cuando necesites trabajar con datos del Catastro español a escala mayor que una consulta puntual: enriquecer listas de RC/direcciones, mantener un padrón sincronizado con drift detection, exportar parcelas por polígono arbitrario, o exponer el Catastro a un agente vía MCP. Para una consulta puntual la web `sedecatastro.gob.es` ya basta; este CLI gana cuando hay batch, comparación contra estado local, o composición de varios endpoints.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`reconcile`** — Compara una tabla local (CSV/SQLite) de parcelas contra Catastro y reporta drift (superficie, uso, año, dirección) parcela a parcela, con exit code != 0 si hay cambios.

  _Para administradores de fincas/urbanizaciones que mantienen padrones desactualizados, este es el comando central._

  ```bash
  catastro-pp-cli reconcile parcelas.csv --rc-col rc --fields surface,use,year,address --agent
  ```
- **`watch`** — Re-sincroniza un municipio y reporta deltas (superficie, uso, año, dirección) por RC respecto al último sync.

  _Único modo de saber si una parcela mutó (reforma, segregación) sin re-cargar 80M parcelas a mano._

  ```bash
  catastro-pp-cli watch --kind parcels --since 2026-01-01 --json
  ```
- **`export parcels`** — Exporta las parcelas dentro de un polígono arbitrario (GeoJSON) a GeoPackage/GeoJSON/SHP, reproyectando al EPSG pedido.

  _Quita el paso QGIS de la rutina del GIS analyst; convierte 30 min de fontanería en un comando._

  ```bash
  catastro-pp-cli export parcels --polygon ./urbanizacion.geojson --to ./parcels.gpkg --epsg 25830
  ```
- **`search`** — Búsqueda full-text sobre la tabla local de parcels (rc, address, use) y streets (nombre, municipio), con filtros opcionales por código INE.

  _Buscar por trozo de calle es lo que el visor web hace lentísimo; offline + FTS lo hace instantáneo._

  ```bash
  catastro-pp-cli search "calle alcalá" --json --agent
  ```
- **`stale`** — Lista RCs/municipios cuyo último sync supera la ventana --older-than, ordenados por antigüedad.

  _Sin esto no hay manera de saber qué parcelas tienen data caducada en el store local._

  ```bash
  catastro-pp-cli stale --older-than 180d --agent
  ```
- **`analyze-area`** — Para un polígono, devuelve número de parcelas, hectáreas totales, histograma de usos, año medio de construcción y RCs únicas.

  _Permite caracterizar una zona (potencial, antigüedad, usos) en un solo comando, sin pre-cargar QGIS._

  ```bash
  catastro-pp-cli analyze-area --polygon ./poligono.geojson --json --agent
  ```
- **`coverage`** — Resumen del estado local: por provincia/municipio, qué hay sincronizado y desde cuándo, con conteos por tipo (parcels/addresses/buildings).

  _Antes de un trabajo, saber qué tienes en local evita re-descargas innecesarias._

  ```bash
  catastro-pp-cli coverage --provincia 28 --json
  ```

### Agent-native plumbing
- **`enrich`** — Lee RCs o direcciones por stdin y emite JSONL con la property card completa de cada uno, con cache en disco y rate-limiting.

  _Sustituye el script pycatastro de un-solo-uso que cada due-diligence escribe; agentes pueden enriquecer listas en una llamada._

  ```bash
  cat refs.txt | catastro-pp-cli enrich --json --agent
  ```

### Service-specific compositions
- **`report`** — Dada una RC, compone un directorio (data.json, parts.csv, coords.json, neighbors.json, map.png, map-url.txt) con la parcela, sus vecinas, sub-parts y un PNG WMS — listo para anexar a un expediente técnico. Cada archivo se reporta en `files[]` solo si su escritura tuvo éxito, y los fallos parciales aparecen en `warnings[]`.

  _Reemplaza la fontanería de 4 herramientas (pycatastro + R CatastRo + QGIS + screenshot) del técnico de expedientes._

  ```bash
  catastro-pp-cli report 0545206VK4704F0001RE --neighbors --to ./expediente-A12/
  ```
- **`neighbors`** — Para una RC, devuelve las parcelas vecinas dentro del radio dado (típico 50 m) con sus datos DNPRC ya unidos.

  _Vecinas es el dato crítico en segregaciones/agrupaciones y disputas de lindes._

  ```bash
  catastro-pp-cli neighbors 0545206VK4704F0001RE --radius 50 --fetch --json
  ```

## Command Reference

**municipalities** — Listado de municipios por provincia

- `catastro-pp-cli municipalities` — Lista los municipios de una provincia (devuelve nombre + código INE)

**numbers** — Numeración de portales en una vía catastral

- `catastro-pp-cli numbers` — Lista los números de portal disponibles en una vía

**property** — Consulta de propiedades catastrales urbanas y rústicas

- `catastro-pp-cli property by-address` — Datos no protegidos de una finca urbana por dirección (DNPLOC). Devuelve referencia catastral + datos de propiedad.
- `catastro-pp-cli property rural` — Datos no protegidos de finca rústica por polígono + parcela (DNPPP).
- `catastro-pp-cli property show` — Datos no protegidos por referencia catastral (DNPRC). Devuelve la finca completa con sus sub-parts.

**provinces** — Listado oficial de provincias del Catastro español

- `catastro-pp-cli provinces` — Lista las 48 provincias donde el Catastro tiene competencia (excluye País Vasco y Navarra)

**streets** — Callejero oficial del Catastro por municipio

- `catastro-pp-cli streets` — Lista las vías de un municipio (calle, plaza, avenida, etc.) con sus códigos


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
catastro-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Auditar un padrón de urbanización contra Catastro

```bash
catastro-pp-cli reconcile parcelas-urbanizacion.csv --rc-col rc --fields surface,use,year,address --json --agent
```

Para cada RC en el CSV llama a Consulta_DNPRC, compara los campos declarados y emite un informe JSONL con el drift; exit 3 si encuentra mismatches (útil para pre-commit/CI).

### Enriquecer 200 direcciones con RC + datos catastrales

```bash
cut -f1 addresses.tsv | catastro-pp-cli enrich --addresses --rate 3 --json --select rc,surface,use,year > out.jsonl
```

Por cada dirección llama a DNPLOC, cachea en SQLite, emite una línea JSON por entrada. `--select` reduce el payload para que el agente no se sature; `--rate 3` evita 429.

### Recortar parcelas dentro de un polígono y exportar a EPSG:25830

```bash
catastro-pp-cli export parcels --polygon ./urbanizacion.geojson --to ./parcels.gpkg --epsg 25830
```

Sincroniza el municipio (si no está), aplica point-in-polygon sobre `parcels.geom_wkt`, reproyecta y escribe GeoPackage listo para CAD/QGIS sin un comando intermedio.

### Detectar parcelas cambiadas en el último trimestre

```bash
catastro-pp-cli watch --kind parcels --municipio 28079 --since 2026-02-01 --json --agent
```

Re-sincroniza Madrid (28079), joinea contra `parcel_history` y emite por cada RC los campos que han cambiado; agentes pueden alertar sobre superficie/uso modificados.

### Bundle expediente para un RC

```bash
catastro-pp-cli report 0545206VK4704F0001RE --neighbors --to ./expediente/
```

Compone DNPRC + RCCOOR_Distancia + WMS GetMap en un directorio con data.json, parts.csv, coords.json, neighbors.json, map.png y map-url.txt — listo para anexar a un informe técnico. Los archivos que fallen aparecen en un campo `warnings[]` del JSON de salida.

## Auth Setup

No authentication required.

Run `catastro-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  catastro-pp-cli municipalities --provincia example-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
catastro-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
catastro-pp-cli feedback --stdin < notes.txt
catastro-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.catastro-pp-cli/feedback.jsonl`. They are never POSTed unless `CATASTRO_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `CATASTRO_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
catastro-pp-cli profile save briefing --json
catastro-pp-cli --profile briefing municipalities --provincia example-value
catastro-pp-cli profile list --json
catastro-pp-cli profile show briefing
catastro-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `catastro-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add catastro-pp-mcp -- catastro-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which catastro-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   catastro-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `catastro-pp-cli <command> --help`.
