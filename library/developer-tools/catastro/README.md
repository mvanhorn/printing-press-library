# Catastro CLI

**Every Catastro feature, plus offline SQLite, bulk RC enrichment, polygon export and change detection no other Catastro tool has.**

catastro-pp-cli unifica la Oficina Virtual del Catastro (OVC Callejero + Coordenadas) y los servicios INSPIRE (WMS/WFS/ATOM) en un único binario Go. Añade un store SQLite local con FTS5 sobre parcelas y calles, comandos pipe-friendly para enriquecer listas de RC o direcciones, recorte por polígono arbitrario (no solo por bbox o municipio) y un comando `reconcile` que compara tu tabla local contra Catastro y reporta drift.

Printed by [@rsolanilla](https://github.com/rsolanilla).

## Install

The recommended path installs both the `catastro-pp-cli` binary and the `pp-catastro` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press install catastro
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install catastro --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press install catastro --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press install catastro --agent claude-code
npx -y @mvanhorn/printing-press install catastro --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/catastro-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-catastro --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-catastro --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-catastro skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-catastro. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/catastro-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "catastro": {
      "command": "catastro-pp-mcp"
    }
  }
}
```

</details>

## Authentication

No requiere API key. Catastro publica los Webservices Libres en `ovc.catastro.meh.es` con acceso anónimo. El cliente respeta un rate-limit suave (~5 req/s) y cachea respuestas en `~/.cache/catastro-pp-cli/cache.sqlite`.

## Quick Start

```bash
# Smoke-check: lista las 48 provincias del Catastro (sin auth)
catastro-pp-cli provinces list --json


# Ficha completa por referencia catastral (DNPRC) — ejemplo: edificio de Calle Alcalá 1, Madrid
catastro-pp-cli property show 0545206VK4704F0001RE --json


# De coordenadas WGS84 a referencias catastrales cercanas (Consulta_RCCOOR)
catastro-pp-cli geocode reverse --lon -3.70379 --lat 40.41678 --json


# Sincroniza el catálogo de provincias al store local SQLite
catastro-pp-cli sync --resources provinces


# Enriquecimiento masivo: por cada RC en stdin, una línea JSON con la ficha completa
cat refs.txt | catastro-pp-cli enrich --json > enriched.jsonl


# Diff entre tu CSV local y Catastro: exit code != 0 si encuentra drift
catastro-pp-cli reconcile parcelas.csv --rc-col rc --fields surface,use,year

```

## Unique Features

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

## Usage

Run `catastro-pp-cli --help` for the full command reference and flag list.

## Commands

### municipalities

Listado de municipios por provincia

- **`catastro-pp-cli municipalities`** - Lista los municipios de una provincia (devuelve nombre + código INE)

### numbers

Numeración de portales en una vía catastral

- **`catastro-pp-cli numbers`** - Lista los números de portal disponibles en una vía

### property

Consulta de propiedades catastrales urbanas y rústicas

- **`catastro-pp-cli property by-address`** - Datos no protegidos de una finca urbana por dirección (DNPLOC). Devuelve referencia catastral + datos de propiedad.
- **`catastro-pp-cli property rural`** - Datos no protegidos de finca rústica por polígono + parcela (DNPPP).
- **`catastro-pp-cli property show`** - Datos no protegidos por referencia catastral (DNPRC). Devuelve la finca completa con sus sub-parts.

### provinces

Listado oficial de provincias del Catastro español

- **`catastro-pp-cli provinces`** - Lista las 48 provincias donde el Catastro tiene competencia (excluye País Vasco y Navarra)

### streets

Callejero oficial del Catastro por municipio

- **`catastro-pp-cli streets`** - Lista las vías de un municipio (calle, plaza, avenida, etc.) con sus códigos


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
catastro-pp-cli municipalities --provincia example-value

# JSON for scripting and agents
catastro-pp-cli municipalities --provincia example-value --json

# Filter to specific fields
catastro-pp-cli municipalities --provincia example-value --json --select id,name,status

# Dry run — show the request without sending
catastro-pp-cli municipalities --provincia example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
catastro-pp-cli municipalities --provincia example-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
catastro-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/catastro-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **`property show` devuelve `RC no encontrada`** — La RC debe tener 14, 18 o 20 caracteres. Verifica con `catastro-pp-cli property show <RC> --dry-run` que la validación local pasa antes de llamar a OVC
- **`429 Too Many Requests` en `enrich` masivo** — Reduce con `--rate 2` (req/s). El default es 5; OVC empieza a 429ear sobre 10
- **`watch` no detecta cambios aunque sabe que los hay** — OVC actualiza diariamente pero el ATOM municipal solo se refresca bi-anualmente. Para cambios diarios usa `enrich` sobre la lista de RCs en lugar de re-sync ATOM
- **`export parcels --polygon` da `municipality not synced`** — El polígono recorta sobre el store local. Ejecuta antes `catastro-pp-cli sync --resources provinces` y luego repuebla con `enrich` sobre los RCs del polígono, hasta que el sync ATOM municipal esté implementado
- **Coordenadas devuelven XML en lugar de JSON** — Es esperado: OVCCoordenadas.asmx solo expone REST con respuesta XML; el cliente lo parsea internamente. Si llamas al endpoint directo, usa `Accept: application/xml`

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**gisce/pycatastro**](https://github.com/gisce/pycatastro) — Python
- [**rOpenSpain/CatastRo**](https://github.com/rOpenSpain/CatastRo) — R
- [**geomatico/cidownloader**](https://github.com/geomatico/cidownloader) — Python
- [**sigdeletras/Spanish_Inspire_Catastral_Downloader**](https://github.com/sigdeletras/Spanish_Inspire_Catastral_Downloader) — Python
- [**MrCabss69/Python-Catastro**](https://github.com/MrCabss69/Python-Catastro) — Python
- [**sperea/catastro-lib-python**](https://github.com/sperea/catastro-lib-python) — Python
- [**ibonkonesa/api-catastro**](https://github.com/ibonkonesa/api-catastro) — JavaScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
