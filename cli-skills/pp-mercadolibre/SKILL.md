---
name: pp-mercadolibre
description: "Printing Press CLI for Mercadolibre. CLI cross-platform para la API de MercadoLibre."
author: "Leandro Castagno"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - mercadolibre-pp-cli
---

# Mercadolibre — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `mercadolibre-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install mercadolibre --cli-only
   ```
2. Verify: `mercadolibre-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

CLI cross-platform para la API de MercadoLibre. Acceso a catalogo de productos, paises, sitios, categorias, perfiles, publicaciones, preguntas y ventas. Cobertura LATAM (AR, BR, MX, CL, CO, UY, etc.). Endpoints publicos no requieren auth; resto via OAuth 2.0 (token en MERCADOLIBRE_ACCESS_TOKEN).

## Unique Capabilities

These capabilities aren't available in any other tool for this API.
- **`watch`** — Poll a saved catalog search at interval; emit JSON line on new product appearance (diff vs prior run). Cron-friendly resale-opportunity radar.
- **`compare`** — Token-bag fingerprint dedup of catalog search results. Collapses near-duplicate listings to one exemplar per fingerprint.
- **`ml-analytics`** — Local SQLite analytics over synced catalog: price percentiles, IQR outlier trim, top sellers by category, std-dev distribution.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**catalog** — Busqueda y detalle del catalogo canonico de productos (universo cross-vendor). Requiere OAuth.

- `mercadolibre-pp-cli catalog get` — Detalle completo de un producto canonico (atributos, fotos, descripcion)
- `mercadolibre-pp-cli catalog search` — Buscar productos canonicos por keyword en un sitio (ej iphone en MLA)

**categories** — Operaciones sobre categorias (taxonomia, atributos requeridos)

- `mercadolibre-pp-cli categories get` — Detalle de categoria con path desde root, atributos requeridos, total items
- `mercadolibre-pp-cli categories list-by-site` — Categorias raiz de un sitio (top-level)

**countries** — Operaciones sobre paises (endpoint publico, no requiere auth)

- `mercadolibre-pp-cli countries get` — Detalle de un pais (estados, geografia)
- `mercadolibre-pp-cli countries list` — Lista todos los paises soportados por MercadoLibre (publico, sin OAuth)

**items** — Operaciones sobre publicaciones individuales (un item especifico en el marketplace)

- `mercadolibre-pp-cli items <item_id>` — Detalle completo de una publicacion (precio, stock, fotos, descripcion, vendedor)

**questions** — Preguntas y respuestas en publicaciones (gestion de Q&A)

- `mercadolibre-pp-cli questions answer` — Responder una pregunta especifica
- `mercadolibre-pp-cli questions list` — Listar preguntas en publicaciones de un vendedor

**sites** — Operaciones sobre sitios de MercadoLibre (AR, BR, MX, etc.)

- `mercadolibre-pp-cli sites` — Lista todos los sitios MercadoLibre (codigo de pais + currency)

**users** — Operaciones sobre usuarios y vendedores (perfil publico, reputacion)

- `mercadolibre-pp-cli users get` — Perfil de usuario/vendedor (nick, reputacion, fecha registro)
- `mercadolibre-pp-cli users items` — Publicaciones de un vendedor (con filtros de estado)


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
mercadolibre-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Run `mercadolibre-pp-cli auth setup` for the URL and steps to obtain a token (add `--launch` to open the URL). Then store it:

```bash
mercadolibre-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `MERCADOLIBRE_ACCESS_TOKEN` as an environment variable.

Run `mercadolibre-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  mercadolibre-pp-cli catalog get mock-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

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
mercadolibre-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
mercadolibre-pp-cli feedback --stdin < notes.txt
mercadolibre-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/mercadolibre-pp-cli/feedback.jsonl`. They are never POSTed unless `MERCADOLIBRE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `MERCADOLIBRE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
mercadolibre-pp-cli profile save briefing --json
mercadolibre-pp-cli --profile briefing catalog get mock-value
mercadolibre-pp-cli profile list --json
mercadolibre-pp-cli profile show briefing
mercadolibre-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `mercadolibre-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add mercadolibre-pp-mcp -- mercadolibre-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which mercadolibre-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   mercadolibre-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `mercadolibre-pp-cli <command> --help`.
