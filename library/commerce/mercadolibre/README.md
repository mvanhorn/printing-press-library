# Mercadolibre CLI

CLI cross-platform para la API de MercadoLibre. Acceso a catalogo de productos, paises, sitios, categorias, perfiles, publicaciones, preguntas y ventas. Cobertura LATAM (AR, BR, MX, CL, CO, UY, etc.). Endpoints publicos no requieren auth; resto via OAuth 2.0 (token en MERCADOLIBRE_ACCESS_TOKEN).

Printed by [@LeaCast](https://github.com/LeaCast) (Leandro Castagno).

## Install

The recommended path installs both the `mercadolibre-pp-cli` binary and the `pp-mercadolibre` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install mercadolibre
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install mercadolibre --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install mercadolibre --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install mercadolibre --agent claude-code
npx -y @mvanhorn/printing-press-library install mercadolibre --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/mercadolibre-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-mercadolibre --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-mercadolibre --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-mercadolibre skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-mercadolibre. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/mercadolibre-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `MERCADOLIBRE_ACCESS_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "mercadolibre": {
      "command": "mercadolibre-pp-mcp",
      "env": {
        "MERCADOLIBRE_ACCESS_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your access token from your API provider's developer portal, then store it:

```bash
mercadolibre-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set it via environment variable:

```bash
export MERCADOLIBRE_ACCESS_TOKEN="your-token-here"
```

### 3. Verify Setup

```bash
mercadolibre-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
mercadolibre-pp-cli catalog get mock-value
```

## Unique Features

These capabilities aren't available in any other tool for this API.
- **`watch`** — Poll a saved catalog search at interval; emit JSON line on new product appearance (diff vs prior run). Cron-friendly resale-opportunity radar.
- **`compare`** — Token-bag fingerprint dedup of catalog search results. Collapses near-duplicate listings to one exemplar per fingerprint.
- **`ml-analytics`** — Local SQLite analytics over synced catalog: price percentiles, IQR outlier trim, top sellers by category, std-dev distribution.

## Usage

Run `mercadolibre-pp-cli --help` for the full command reference and flag list.

## Commands

### catalog

Busqueda y detalle del catalogo canonico de productos (universo cross-vendor). Requiere OAuth.

- **`mercadolibre-pp-cli catalog get`** - Detalle completo de un producto canonico (atributos, fotos, descripcion)
- **`mercadolibre-pp-cli catalog search`** - Buscar productos canonicos por keyword en un sitio (ej iphone en MLA)

### categories

Operaciones sobre categorias (taxonomia, atributos requeridos)

- **`mercadolibre-pp-cli categories get`** - Detalle de categoria con path desde root, atributos requeridos, total items
- **`mercadolibre-pp-cli categories list-by-site`** - Categorias raiz de un sitio (top-level)

### countries

Operaciones sobre paises (endpoint publico, no requiere auth)

- **`mercadolibre-pp-cli countries get`** - Detalle de un pais (estados, geografia)
- **`mercadolibre-pp-cli countries list`** - Lista todos los paises soportados por MercadoLibre (publico, sin OAuth)

### items

Operaciones sobre publicaciones individuales (un item especifico en el marketplace)

- **`mercadolibre-pp-cli items <item_id>`** - Detalle completo de una publicacion (precio, stock, fotos, descripcion, vendedor)

### questions

Preguntas y respuestas en publicaciones (gestion de Q&A)

- **`mercadolibre-pp-cli questions answer`** - Responder una pregunta especifica
- **`mercadolibre-pp-cli questions list`** - Listar preguntas en publicaciones de un vendedor

### sites

Operaciones sobre sitios de MercadoLibre (AR, BR, MX, etc.)

- **`mercadolibre-pp-cli sites`** - Lista todos los sitios MercadoLibre (codigo de pais + currency)

### users

Operaciones sobre usuarios y vendedores (perfil publico, reputacion)

- **`mercadolibre-pp-cli users get`** - Perfil de usuario/vendedor (nick, reputacion, fecha registro)
- **`mercadolibre-pp-cli users items`** - Publicaciones de un vendedor (con filtros de estado)


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
mercadolibre-pp-cli catalog get mock-value

# JSON for scripting and agents
mercadolibre-pp-cli catalog get mock-value --json

# Filter to specific fields
mercadolibre-pp-cli catalog get mock-value --json --select id,name,status

# Dry run — show the request without sending
mercadolibre-pp-cli catalog get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
mercadolibre-pp-cli catalog get mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
mercadolibre-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/mercadolibre-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `MERCADOLIBRE_ACCESS_TOKEN` | per_call | No | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `mercadolibre-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $MERCADOLIBRE_ACCESS_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
