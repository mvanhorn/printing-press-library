# Crocoblock CLI

**Operate Crocoblock data and review changes across your WordPress workflows.**

Inspect the installed JetPlugins, edit supported data with request previews, and compare local snapshots before a site release. Keep schemas, form observations, reservations and filter maintenance evidence tied to the site and plugin versions that produced them.

## Install

Use the Printing Press catalog to install the CLI and agent skill. The source-build instructions also work directly from this module. A new catalog entry becomes installable after its PR is merged and the catalog workflows finish.

### Build from source

From this CLI module directory, use Go 1.26.11:

```bash
GOTOOLCHAIN=go1.26.11 go build -o ./crocoblock-pp-cli ./cmd/crocoblock-pp-cli
GOTOOLCHAIN=go1.26.11 go install ./cmd/crocoblock-pp-cli ./cmd/crocoblock-pp-mcp
crocoblock-pp-cli --version
```

`go install` writes to `GOBIN`, or `$(go env GOPATH)/bin` when `GOBIN` is unset. Put that directory on the invoking agent's `PATH`. The local MCP binary uses the same WordPress environment variables as the CLI.

### Catalog installation

The recommended path installs both the `crocoblock-pp-cli` binary and the `pp-crocoblock` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install crocoblock
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install crocoblock --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install crocoblock --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install crocoblock --agent claude-code
npx -y @mvanhorn/printing-press-library install crocoblock --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.11 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/crocoblock/cmd/crocoblock-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/crocoblock-current). After saving it as `./crocoblock-pp-cli`, clear the Gatekeeper quarantine on macOS with `xattr -d com.apple.quarantine ./crocoblock-pp-cli`. On Unix, mark it executable with `chmod +x ./crocoblock-pp-cli`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install crocoblock --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-crocoblock --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-crocoblock --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install crocoblock --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

The release packaging supports an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

For a published release only:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/crocoblock-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Configure the WordPress base URL, username and Application Password. For the current local build, use the manual environment configuration below.

Requires Claude Desktop 1.0.0 or later. The generated packaging targets pre-built bundles for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/crocoblock/cmd/crocoblock-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS, `%APPDATA%\Claude\claude_desktop_config.json` on Windows):

```json
{
  "mcpServers": {
    "crocoblock": {
      "command": "crocoblock-pp-mcp",
      "env": {
        "CROCOBLOCK_BASE_URL": "https://wordpress.example/wp-json",
        "CROCOBLOCK_USERNAME": "<wordpress-username>",
        "CROCOBLOCK_APPLICATION_PASSWORD": "<wordpress-application-password>"
      }
    }
  }
}
```

</details>

## Authentication

Set CROCOBLOCK_BASE_URL to the authorized HTTPS WordPress REST URL ending in /wp-json, CROCOBLOCK_USERNAME to the WordPress user, and CROCOBLOCK_APPLICATION_PASSWORD to its Application Password. Keep credentials in private environment/configuration, never in scripts, reports or source. Run crocoblock-pp-cli doctor --json and crocoblock-pp-cli discover --agent to check connectivity, installed modules and permission gaps. Your Crocoblock account session is only for downloading entitled plugins. Filter reindexing additionally requires an explicit SSH host and WordPress installation path for the same site. The SSH identity file is optional; omit it to use the existing SSH configuration or agent. Use CROCOBLOCK_DB or the command-specific database option to choose a site-specific local snapshot.

## Quick Start

```bash
# Inspect the health-check plan without credentials or network calls.
crocoblock-pp-cli doctor --dry-run

# After configuring the site credentials, inspect supported plugins and permissions.
crocoblock-pp-cli discover --agent

# Read the observable JetEngine structure with secret fields redacted.
crocoblock-pp-cli engine configuration --agent

```

## Recipes

### Review structure changes

```bash
crocoblock-pp-cli schema diff --before baseline.db --after candidate.db --agent
```

Compare complete snapshots captured before and after a site change.

### Inspect form field names

```bash
crocoblock-pp-cli forms fields 42 --agent --select fields
```

Keep a field-schema inspection small while the adapter handles published forms and drafts.

### Compare CCT content

```bash
crocoblock-pp-cli cct reconcile rooms --before baseline.db --after candidate.db --key external_id --fields external_id,title --agent
```

Use an explicit unique key and selected fields to compare data across revisions.

### Review filter maintenance candidates

```bash
crocoblock-pp-cli filters impact --db candidate.db --since-event 1 --agent
```

Limit the report to recorded writes and verified dependencies rather than assuming all site changes were observed.

## Unique Features

### Revisión de entregas
- **`schema diff`** — Muestra qué tipos, campos y definiciones cambiaron entre dos revisiones del sitio.

  _Úsalo antes de una entrega para revisar cambios estructurales comprobables._

  ```bash
  crocoblock-pp-cli schema diff --before baseline.db --after candidate.db --agent
  ```
- **`cct reconcile`** — Compara registros entre instantáneas por una clave única y muestra diferencias en campos seleccionados.

  _Úsalo para comprobar que dos revisiones conservan los datos esperados._

  ```bash
  crocoblock-pp-cli cct reconcile rooms --before baseline.db --after candidate.db --key external_id --fields external_id,title --agent
  ```
- **`relations audit`** — Detecta aristas duplicadas y ausencias comprobables en inventarios CCT completos sin filtros. Los extremos WordPress ausentes quedan unknown porque el inventario excluye papelera y puede omitir objetos restringidos.

  _Úsalo después de cambios de contenido para revisar relaciones, sin interpretar un permiso denegado como un huérfano._

  ```bash
  crocoblock-pp-cli relations audit --db candidate.db --agent
  ```

### Mantenimiento de formularios y filtros
- **`forms field-usage`** — Compara los campos actuales de un formulario con los nombres presentes en registros seleccionados.

  _Úsalo para encontrar nombres históricos o campos nunca observados en la muestra, sin inferir errores de validación._

  ```bash
  crocoblock-pp-cli forms field-usage 42 --db candidate.db --agent
  ```
- **`filters impact`** — Identifica candidatos de reindexación por cambios registrados para filtros custom_fields de JetSmartFilters 3.8.6.2 con _query_var y _source_custom_field válidos; otros proveedores o versiones quedan unknown.

  _Úsalo después de una edición para decidir qué mantenimiento del índice requiere revisión._

  ```bash
  crocoblock-pp-cli filters impact --db candidate.db --since-event 1 --agent
  ```

### Revisión de reservas
- **`reservations overlaps`** — Señala solapamientos por recurso y evalúa capacidad con una política explícita y ambas capturas completas de JetBooking 4.1.3.1 y JetAppointment 2.5.3. Sólo admite citas slot/range registradas; no expande recurrencias.

  _Úsalo para revisar posibles choques sin confundir un solapamiento con una reserva inválida._

  ```bash
  crocoblock-pp-cli reservations overlaps --db candidate.db --policy policy.json --agent
  ```

## Usage

Run `crocoblock-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `CROCOBLOCK_CONFIG_DIR`, `CROCOBLOCK_DATA_DIR`, `CROCOBLOCK_STATE_DIR`, or `CROCOBLOCK_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `CROCOBLOCK_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export CROCOBLOCK_HOME=/srv/crocoblock
crocoblock-pp-cli doctor
```

Under `CROCOBLOCK_HOME=/srv/crocoblock`, the four dirs resolve to `/srv/crocoblock/config`, `/srv/crocoblock/data`, `/srv/crocoblock/state`, and `/srv/crocoblock/cache`.

On Windows, set the same variable in PowerShell:

```powershell
$env:CROCOBLOCK_HOME = "C:\srv\crocoblock"
crocoblock-pp-cli doctor
```

Under `$env:CROCOBLOCK_HOME = "C:\srv\crocoblock"`, the four dirs resolve to `C:\srv\crocoblock\config`, `C:\srv\crocoblock\data`, `C:\srv\crocoblock\state`, and `C:\srv\crocoblock\cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "crocoblock": {
      "command": "crocoblock-pp-mcp",
      "env": {
        "CROCOBLOCK_HOME": "/srv/crocoblock"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `CROCOBLOCK_DATA_DIR` overrides an explicit `--home` for that kind. Use `CROCOBLOCK_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `CROCOBLOCK_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `crocoblock-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

The adapter exposes **61 service commands: 55 base operations and six local analyses**. The tables list actual command paths and positional arguments; prepend `crocoblock-pp-cli`. Framework utilities such as `doctor`, `agent-context`, `which`, profiles and the learning loop are additional commands.

Run a concrete command's help, such as `crocoblock-pp-cli engine configuration --help`, for current defaults and constraints. Common flags include `--agent`, `--json`, `--select`, `--dry-run` and `--data-source`. Wherever `--input FILE` is shown, the command also accepts `--input -` or `--data JSON`; supply exactly one payload source. Read-only native JetEngine commands accept argument objects, not arbitrary writes. Relation read limits trim an upstream unpaginated collection; they do not reduce the upstream fetch.

### Discovery and capture

| Command | Arguments and behavior |
| --- | --- |
| `discover` | Inspect plugin versions, registered routes and permission gaps. |
| `sync` | `--db FILE --resources CSV --full --strict`; optional `--cct TYPE --fields CSV`, `--record-form ID`, `--object-types CSV`, `--max-pages N`. |

### JetEngine

| Command | Arguments and behavior |
| --- | --- |
| `engine configuration` | Optional `--input FILE` to select native configuration parts. |
| `engine website` | Optional `--input FILE` to select native website configuration parts. |
| `engine macros` | Optional `--exclude CSV` or native argument object via `--input FILE`. |
| `engine modules` | `--operation list / activate / deactivate`; use `--modules CSV` for changes, or `--input FILE`. |
| `engine create-cct` | `--input FILE` with the native CCT definition. |
| `engine create-cpt` | `--input FILE` with the native custom post type definition. |
| `engine create-taxonomy` | `--input FILE` with the native taxonomy definition. |
| `engine create-meta-box` | `--input FILE` with the native metadata box definition. |
| `engine create-query` | `--input FILE` with the native query definition. |
| `engine create-listing` | `--input FILE` with the native listing definition. |
| `engine create-glossary` | `--input FILE` with the native glossary definition. |

### CCT data

| Command | Arguments and behavior |
| --- | --- |
| `cct list <type>` | `--limit N` (1–500), `--offset N`, `--order asc / desc`, `--order-by FIELD`, `--search TEXT`, `--search-by CSV`, repeatable `--where field=value`. |
| `cct get <type> <id>` | Read one item. |
| `cct create <type>` | `--input FILE` with declared CCT fields. |
| `cct update <type> <id>` | `--input FILE` with fields to update. |
| `cct delete <type> <id>` | Delete the selected item. |

### Relations

| Command | Arguments and behavior |
| --- | --- |
| `relations definitions` | `--limit N` (1–500), `--offset N`. |
| `relations list <relation-id>` | `--limit N` (1–500), `--offset N`. |
| `relations parents <relation-id> <object-id>` | `--limit N` (1–500), `--offset N`. |
| `relations children <relation-id> <object-id>` | `--limit N` (1–500), `--offset N`. |
| `relations update <relation-id>` | `--context child / parent --mode append / replace / disconnect --input FILE`. |

### Forms and submitted records

| Command | Arguments and behavior |
| --- | --- |
| `forms list` | `--limit N` (1–100), `--page N`, `--status STATUS` (default `any`). |
| `forms get ID` | Read one form with sensitive metadata redacted. |
| `forms create` | `--input FILE`; draft by default. |
| `forms update ID` | `--input FILE`; preserve unrelated content and metadata. |
| `forms trash ID` | Move the selected form to trash. |
| `forms fields ID` | Read declared fields; use `--select fields` to project the field list. |
| `records list` | `--form ID`, `--limit N` (1–100), `--page N`, `--sort ASC / DESC`. Use `--form` for a verified form association. |
| `records fields ID` | `--fields CSV`, `--limit N` (1–10000); opt-in `--include-values` requires selected fields. |

### Bookings and appointments

| Command | Arguments and behavior |
| --- | --- |
| `bookings list` | `--filters JSON`, `--limit N` (1–1000), `--offset N`, `--mode all / upcoming / past`. |
| `bookings config RESOURCE_ID` | Read configuration for one booking resource. |
| `bookings create` | `--input FILE`. |
| `bookings update ID` | `--input FILE`. |
| `bookings delete ID` | Delete the selected booking. |
| `appointments list` | `--filters JSON`, `--limit N` (1–1000), `--offset N`, `--mode all / upcoming / past`. |
| `appointments get ID` | Read one appointment. |
| `appointments create` | `--input FILE --timezone IANA_OR_UTC`. |
| `appointments update ID` | `--input FILE --timezone IANA_OR_UTC`. |
| `appointments delete ID [ID...]` | Delete the explicitly selected appointment IDs. |
| `appointments providers SERVICE_ID` | Read providers for one service. |
| `appointments slots SERVICE_ID` | `--date YYYY-MM-DD --timezone IANA_OR_UTC`; optional `--provider ID`. May persist excluded days. |

### Filters and REST-exposed metadata

| Command | Arguments and behavior |
| --- | --- |
| `filters list` | `--limit N` (1–100), `--page N`, `--search TEXT`, `--status publish / draft / trash`. |
| `filters get <id>` | Read one filter. |
| `filters create` | `--input FILE`. |
| `filters update <id>` | `--input FILE`. |
| `filters duplicate <id>` | Duplicate the selected filter. |
| `filters trash <id>` | Move the selected filter to trash. |
| `filters restore <id>` | Restore the selected trashed filter. |
| `filters reindex` | `--ssh user@host --wp-path /absolute/path`; optional `--ssh-key FILE`. Site check and local receipt required. |
| `fields list` | `--post-type SLUG` (default `post`); compare declared metadata with the REST schema. |
| `fields get ID` | `--fields CSV` required; optional `--post-type SLUG`. |
| `fields update ID` | `--input FILE --snapshot PRIVATE_FILE`; optional `--post-type SLUG`. |
| `fields restore` | `--snapshot PRIVATE_FILE`; checks current selected values before restore. |

### Six local analyses

| Command | Arguments and behavior |
| --- | --- |
| `schema diff` | `--before FILE --after FILE`; optional `--resources CSV`, repeatable `--identity resource=FIELD_PATH`, `--limit N` (default 500). |
| `cct reconcile <type>` | `--before FILE --after FILE --key FIELD --fields CSV`; `--limit N` (1–500, default 100). |
| `relations audit [relation-id]` | `--db FILE`; `--limit N` (1–500, default 100). |
| `forms field-usage ID` | `--db FILE`; `--limit N` (1–10000, default 100); totals cover the full captured sample. |
| `reservations overlaps` | `--db FILE --policy FILE`; `--limit N` (default 1000); all captured rows are analyzed. |
| `filters impact` | `--db FILE`; `--since-event N` is exclusive; `--limit N` (default 500). |

### Data sources and operational limits

| Surface | Source | What the result establishes |
| --- | --- | --- |
| `discover`, `engine`, CCT CRUD, relation reads/updates, forms CRUD/fields, `records`, `bookings`, `appointments`, filter CRUD/reindex, `fields` | Live WordPress or explicitly configured SSH; use `--data-source live` | The response from the selected site at execution time, subject to permissions and plugin contracts |
| `sync` | Live reads saved to local SQLite; use `--data-source live` | Captured resources with site, plugin versions, scope, timestamp and completeness |
| `schema diff`, `cct reconcile`, `relations audit`, `forms field-usage`, `reservations overlaps`, `filters impact` | Existing SQLite captures; use `--data-source local` | Analysis of the captured sample and journal; never an automatic live refresh |

The six analyses reject `--data-source live`. Missing, denied, capped, incompatible or incomplete coverage is unknown/error, not a clean result or an empty collection. Check capture timestamps, site, scope and plugin versions. Collection limits can truncate a response; inspect totals and coverage rather than interpreting the displayed rows as the full site.

Use `--input private.json` or `--input -` for JSON payloads when the command exposes it. These avoid placing personal data or secrets in command arguments and shell history. `--data` is available but has that exposure. Form action secrets are redacted. `records fields` reports names and presence by default; values require both `--fields` and `--include-values`, and secret keys remain redacted. CCT capture values are restricted to explicitly selected `--fields`; treat the resulting database as private.

A dry run performs no network or payload-file access. It verifies command/flag routing and previews the operation; it does not validate a fixture, permission, payload schema or current remote state. Inspect help, preview, execute once, and verify the result. Never automatically repeat a write after a timeout, ambiguous transport response, or a `mutation_applied`/receipt failure: the remote mutation may already have succeeded. Shared retry-related flags do not make plugin writes replay-safe.

`forms create` defaults to a draft. Updates preserve unrelated raw blocks and metadata; `forms trash` moves the form to trash. `forms fields` can extract declared fields from stored draft blocks without publishing. `fields` handles only JetEngine-declared post metadata that WordPress exposes in its REST schema; unknown or unregistered keys are rejected. `fields update --snapshot private.json` saves a private before/after file separate from SQLite. Restore compares fresh selected values with the recorded after-state before writing. This is a concurrency check, not atomic compare-and-swap (CAS); another writer can still race the check.

`appointments slots` can cause the provider plugin to persist excluded days while calculating availability. Treat it as potentially mutating, and supply the explicit date and timezone. Availability output is not a commercial guarantee. `filters reindex` checks the remote WordPress site URL against the configured site before executing the native indexer over SSH/WP-CLI, then records a verified receipt locally. A successful index rebuild with a failed receipt must not be blindly repeated.

### MCP surface and local file workflows

The CLI retains all 61 Crocoblock capabilities. In normal mode the MCP server exposes 56 of them plus 19 framework tools (75 tools total); read-only server mode exposes a smaller set. MCP callers cannot choose server-side input files or output destinations. Five Crocoblock commands therefore remain CLI-only: `schema diff`, `cct reconcile`, `reservations overlaps`, `fields update` and `fields restore`. The framework `import` command is also CLI-only because it has no inline payload alternative.

Use the local CLI for snapshot comparisons, reservation policy files and field backup/restore. Through MCP, `forms field-usage`, `filters impact` and `relations audit` read the database selected by the server's `CROCOBLOCK_DB`; `sync` populates that same configured store. File input is omitted from MCP mutation tools; pass a JSON object inline through their data parameter. Filter reindexing can use the SSH identity already configured for the server account. No file-path bypass is supported.

### Capture workflow

`sync --full --strict` captures the selected collections. Both flags default to true; this adapter has no incremental cursor mode. `--strict` returns a nonzero result if any selected resource fails. `--max-pages` defaults to 10; hitting the cap leaves coverage incomplete. Disabling strict failure reporting does not make incomplete data complete.

Set `CROCOBLOCK_DB` before both mutations and captures when they must share a journal. Passing `--db` to `sync` or an analysis only selects that invocation's SQLite file; it does not redirect later write commands. This distinction is necessary for `filters impact` to see the same site's captured filters, write events and reindex receipts.

```bash
export CROCOBLOCK_DB="$PWD/candidate.db"

# Default resources: schema, forms, filters, bookings, appointments, relations.
crocoblock-pp-cli sync --db "$CROCOBLOCK_DB" --full --strict --agent

# Capture selected CCT values, including the reconciliation business key.
crocoblock-pp-cli sync --resources cct --cct rooms --fields external_id,title --db "$CROCOBLOCK_DB" --full --strict --agent

# Capture declared fields and record presence for one explicit form; no submitted values.
crocoblock-pp-cli sync --resources forms,records --record-form 42 --db "$CROCOBLOCK_DB" --full --strict --agent

# Capture accessible WordPress IDs needed for relation endpoint checks.
crocoblock-pp-cli sync --resources relations,objects --object-types post --db "$CROCOBLOCK_DB" --full --strict --agent
```

Replace `rooms`, field names, form ID and object types with identifiers discovered on the authorized site. `--resources` accepts `schema,forms,filters,bookings,appointments,relations,cct,records,objects`. CCT rows require `--cct` and explicit `--fields`; records require `--record-form`; object inventories require `--object-types`. Relation audits need complete inventories for the endpoint types in their definitions; a `post` inventory cannot establish the existence of a CCT or another object type.

The default `schema diff` compares relations as well as configuration, CCT definitions, post types, taxonomies, meta boxes, queries, forms and filters. For a focused capture in each comparison database, use `sync --resources schema,forms,filters,relations --full --strict`; omitting relations leaves the default comparison incomplete.

Capture baseline and candidate runs into separate files for `schema diff` and `cct reconcile`. Sync replaces the selected resource captures in its target database, so reuse of the baseline path would erase the comparison point. Reads across pages and resources are not an atomic database snapshot. Even complete coverage records an observation window, not a transactionally consistent instant; concurrent changes can invalidate comparison assumptions.

`forms field-usage` reports declared, observed and obsolete names with an explicit captured-record denominator. A complete capture with zero records is an empty sample; a declared field without observed presence is not a recommendation to delete it. It cannot infer retention or submissions never captured. `reservations overlaps` requires an explicit `--policy` JSON file for timezone, statuses and capacity; its results describe that policy and sample. `filters impact` observes only this CLI's journal and verified receipts, not external edits or global index freshness.

### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`crocoblock-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`crocoblock-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`crocoblock-pp-cli learnings list`** - Inspect taught rows
- **`crocoblock-pp-cli learnings forget <query>`** - Undo a teach
- **`crocoblock-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`crocoblock-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`crocoblock-pp-cli teach-pattern`** - Install a query/resource template up front
- **`crocoblock-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `CROCOBLOCK_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `crocoblock-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

Curated commands emit structured JSON. Use `--agent` for a compact JSON envelope, or `--json` for the full payload. `--select` projects payload fields before the agent envelope is added.

```bash
crocoblock-pp-cli discover --json
crocoblock-pp-cli discover --agent --select versions
crocoblock-pp-cli forms fields 42 --agent --select fields
crocoblock-pp-cli appointments list --dry-run --agent
```

With `--agent`, read `.results` for data. Live operations identify a live source; the six local analyses use `meta.source="computed"` and `meta.transport="local"` and include capture provenance in their result. Use an explicit `--select` when compact output would omit fields needed for the task.

`--csv` prefixes a leading `'` on cells and header keys that start with `=`, `+`, `-`, `@`, a tab, or a line break (including fullwidth `＝＋－＠`) so a spreadsheet does not run them as formulas. Plain signed numbers such as `-5` and `+2.5e3` are left unchanged.

## Agent Usage

- Verify the installed binary with `--version`, then run `doctor --agent` and inspect the relevant `--help` before using a capability.
- Select `--data-source live` for base operations and `--data-source local` for the six analyses. Sync requires live access and writes the capture file.
- Use `--agent` for non-interactive JSON and `--select` to keep the fields required for the decision. Errors go to stderr.
- Use `--input FILE` or `--input -` for supported write payloads. A dry run previews the command without opening files or contacting the site; it is not live validation.
- Confirm the authorized target and side effects before a mutation. `--agent` does not imply `--yes`. Never retry an ambiguous write automatically; inspect the remote result and journal first.
- Read coverage, provenance, totals and truncation. An incomplete capture cannot establish that the site is clean.

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `6` partial failure, `7` rate limited, `10` config error. A nonzero result after a mutation can mean verification or journaling failed after the change was applied.

## Health Check

```bash
crocoblock-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `crocoblock-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/crocoblock-pp-cli/config.toml` on macOS/Linux and `%USERPROFILE%\.config\crocoblock-pp-cli\config.toml` on Windows; `--home`, `CROCOBLOCK_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers` in `config.toml`. The curated plugin commands do not expose a per-invocation header override flag.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `CROCOBLOCK_BASE_URL` | connection | Yes | Authorized HTTPS WordPress REST base URL, including `/wp-json`. |
| `CROCOBLOCK_USERNAME` | per_call | Yes | WordPress username. |
| `CROCOBLOCK_APPLICATION_PASSWORD` | per_call | Yes | WordPress Application Password for that user. |
| `CROCOBLOCK_DB` | local path | No | SQLite capture and write-journal file; set it consistently for the selected site. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `crocoblock-pp-cli doctor` reports `agentcookie: detected` and `crocoblock-pp-cli auth status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `crocoblock-pp-cli doctor` to check credentials
- Verify all three WordPress environment variables in the invoking runtime without printing the password.
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **A plugin route is absent or returns 403.** — Run crocoblock-pp-cli discover --agent and check the installed module, REST exposure and WordPress capability; do not treat a denial as an empty collection.
- **The JetFormBuilder field endpoint returns 204 for a draft.** — Use crocoblock-pp-cli forms fields 42 --agent; the draft adapter reads stored blocks without publishing the form.
- **Filter reindexing has no configured transport.** — Configure SSH host, identity and the WordPress installation path for the selected site; the command verifies the site URL before using WP-CLI.
- **A local comparison reports incomplete coverage.** — Capture the required resources with sync --full --strict into separate --db snapshots and resolve denied or missing resources before comparing.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**wp-jetengine-skills**](https://github.com/EPEAKSTUDIO/wp-jetengine-skills) — PHP
- [**JetFormBuilder**](https://github.com/Crocoblock/jetformbuilder) — PHP
- [**WPMCP**](https://github.com/wpmcp/wpmcp) — PHP

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

Field writes retain a local intent before transport. If verification or snapshot finalization fails, matching filter impact remains `unknown`; a covering reindex receipt alone does not prove that an in-flight write finished. Inspect WordPress before retrying an ambiguous write.
