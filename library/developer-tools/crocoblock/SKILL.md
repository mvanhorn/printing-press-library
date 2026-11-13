---
name: pp-crocoblock
description: "Operate Crocoblock data and review changes across your WordPress workflows. Trigger phrases: `revisa los campos de JetEngine`, `compara los datos de este CCT`, `revisa los registros de JetFormBuilder`, `busca solapamientos en las reservas`, `revisa qué filtros necesitan reindexarse`, `use crocoblock`, `run crocoblock`."
author: "bobe"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - crocoblock-pp-cli
    install:
      - kind: go
        bins: [crocoblock-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/developer-tools/crocoblock/cmd/crocoblock-pp-cli
---

# Crocoblock — Printing Press CLI

## Build from source

Use the Printing Press catalog to install the CLI and agent skill. The source-build instructions also work directly from this module. A new catalog entry becomes installable after its PR is merged and the catalog workflows finish.

### Build from source

From this CLI module directory, use Go 1.26.11:

```bash
GOTOOLCHAIN=go1.26.11 go build -o ./crocoblock-pp-cli ./cmd/crocoblock-pp-cli
GOTOOLCHAIN=go1.26.11 go install ./cmd/crocoblock-pp-cli ./cmd/crocoblock-pp-mcp
crocoblock-pp-cli --version
```

`go install` writes to `GOBIN`, or `$(go env GOPATH)/bin` when `GOBIN` is unset. Put that directory on the invoking agent's `PATH`. The local MCP binary uses the same WordPress environment variables as the CLI.


**Installation selection:** install a missing binary using **Prerequisites: Install the CLI**, or use **Build from source** when operating from a source checkout.

## Prerequisites: Install the CLI

This skill drives the `crocoblock-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install crocoblock --cli-only
   ```
2. Verify: `crocoblock-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.11 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/crocoblock/cmd/crocoblock-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Inspect the installed JetPlugins, edit supported data with request previews, and compare local snapshots before a site release. Keep schemas, form observations, reservations and filter maintenance evidence tied to the site and plugin versions that produced them.

## When to Use This CLI

Use this CLI to inspect and operate supported JetEngine, JetFormBuilder, JetBooking, JetAppointment and JetSmartFilters contracts on an authorized WordPress site. Use local comparisons to review schema and data changes, relation integrity, observed form-field names, reservation overlaps and recorded filter maintenance needs. Check provenance and coverage before treating a local result as current.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to buy or activate Crocoblock licenses or manage the vendor account.
- Do not use this CLI for universal WordPress administration, visual page design or unrelated JetPlugins.
- Do not use this CLI to submit live payment or mail forms or to assume a reservation is commercially available.
- Do not use a local audit with incomplete coverage to certify production integrity.

## Unique Capabilities

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

## Command Reference

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

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
crocoblock-pp-cli which "compare schema"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query. `--json` (and other machine formats) keep that exit-2 contract and write `{"matches":[]}` on stdout so agents can inspect the envelope without treating a miss as success.

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

## Auth Setup

Set CROCOBLOCK_BASE_URL to the authorized HTTPS WordPress REST URL ending in /wp-json, CROCOBLOCK_USERNAME to the WordPress user, and CROCOBLOCK_APPLICATION_PASSWORD to its Application Password. Keep credentials in private environment/configuration, never in scripts, reports or source. Run crocoblock-pp-cli doctor --json and crocoblock-pp-cli discover --agent to check connectivity, installed modules and permission gaps. Your Crocoblock account session is only for downloading entitled plugins. Filter reindexing additionally requires an explicit SSH host and WordPress installation path for the same site. The SSH identity file is optional; omit it to use the existing SSH configuration or agent. Use CROCOBLOCK_DB or the command-specific database option to choose a site-specific local snapshot.

Run `crocoblock-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` for `--json --compact --no-input --no-color`. Commands remain subject to site permissions and their own contracts. `--agent` does not imply `--yes` or authorize a side effect.

- JSON goes to stdout and errors go to stderr. Use `--json` without `--compact` for a full payload.
- `--select` projects payload fields before the envelope is added and takes precedence over compact output. Dotted paths traverse objects and arrays. For form declarations use `--select fields`.
- `--dry-run` does not open payload files or contact the network. It verifies command/flag routing, not the current data, credentials or payload schema.
- `--input FILE` or `--input -` supplies JSON without sensitive values in arguments. Use it only on commands whose help exposes that flag.
- Base operations are live; the six analyses read snapshots and reject `--data-source live`. Sync needs network access to create or refresh those captures.
- Never automatically retry a mutation after an ambiguous result, verification failure or journal/receipt failure. An applied change can accompany a nonzero exit.
- `--csv` and `--plain` produce tabular output; `--quiet` emits identity values. CSV protects formula-like cells and headers with a leading apostrophe.

```bash
crocoblock-pp-cli discover --data-source live --agent --select versions
crocoblock-pp-cli forms fields 42 --data-source live --agent --select fields
crocoblock-pp-cli forms field-usage 42 --db candidate.db --data-source local --agent
```

### Response envelope

With `--agent`, parse `.results` for data and `.meta` for source metadata. Live adapters identify a live source. The six local analyses use `meta.source="computed"` and `meta.transport="local"`; capture timestamps, scope, versions and coverage are in their results. Select the fields needed for the decision explicitly if compact output omits them. Plain `--json` produces the payload without requiring the agent envelope.

A report can be limited to a captured sample even when its algorithm scanned every stored row. Check the command's total/truncation fields and capture coverage. Unknown coverage is not zero findings.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `CROCOBLOCK_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `CROCOBLOCK_CONFIG_DIR`, `CROCOBLOCK_DATA_DIR`, `CROCOBLOCK_STATE_DIR`, `CROCOBLOCK_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `CROCOBLOCK_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `crocoblock-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

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

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `CROCOBLOCK_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `CROCOBLOCK_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, pass the question as an argv or MCP tool argument to `recall --agent`. Do not interpolate user-controlled text into a shell command line.

Quoted `recall "<question>"` breaks on an apostrophe, which is ordinary English. A quoted heredoc breaks when a body line equals the delimiter, and that delimiter is published in these docs. Write the question with a non-shell file-writing tool, then read it back as data:

```bash
# Write the question verbatim with your file-writing tool (no shell involved).
# Command substitution on a file only ever yields data — the shell never
# parses the file's bytes as syntax.
QUERY=$(cat /path/to/question.txt)
crocoblock-pp-cli recall "$QUERY" --agent
```

Prefer MCP: pass the question as the tool's query argument. `"$QUERY"` after a file read is argv-safe; putting the question itself in the command text is not.

The response envelope:

```json
{
  "query": "...",
  "normalized": "<normalized form>",
  "query_entities": ["..."],
  "found": true | false,
  "match_score": 0.0,
  "results": [
    { "resource_id": "...", "resource_type": "...", "venue": "...",
      "confidence": 2, "entity_match": "exact|partial|unknown",
      "source": "taught|preseed|pattern", "warnings": ["..."] }
  ],
  "mismatches": [ /* only when --debug-mismatches */ ],
  "warnings": [ /* top-level */ ],
  "candidates": [
    { "id": 12, "class": "flag_alias | playbook_candidate",
      "summary": "...", "sightings": 3, "last_seen": "...",
      "rationale": "...",
      "next_action": ["<trial command>", "crocoblock-pp-cli learnings confirm 12"] }
  ],
  "playbook": {
    "query_family": "...",
    "playbook": {
      "steps": [ { "cmd": "<command with {slot} substitution>", "purpose": "..." } ],
      "entity_slots": ["$ENTITY"],
      "expected_tool_calls": 3
    },
    "slots_resolved": { "$ENTITY": { "token": "<live token>", "canonical": "<canonical>" } },
    "notes": "<workarounds + gotchas for this query family>"
  },
  "notes": "<duplicate surface for non-playbook callers>"
}
```

Empty-store short-circuit: if the store has no learnings, playbooks, or candidates yet (recall finds nothing and `learnings list` and `learnings candidates` are both empty), skip recall for the rest of this session instead of taxing every query; resume recall-first once something has been taught.

### Step 2: decision tree

Read `candidates`, `playbook`, `notes`, `results[0]`, and warnings in that order:

```
if Candidates present (warnings include "candidates_present"):
    -> candidates are try-then-confirm, never facts. Follow each candidate's
       two-step next_action verbatim: run the trial command first, then run
       `learnings confirm <id>` only after the trial verified the behavior.
       Reject a wrong candidate with `learnings reject <id>`.
    -> NEVER re-teach something recall surfaced as a candidate; confirm or
       reject that candidate instead of teaching a duplicate.
    -> candidates ride alongside playbooks and resource hits, not instead of
       them; continue with the branches below after acting on them.

if Playbook present:
    -> READ Playbook.notes verbatim FIRST (workarounds + gotchas the CLI surface doesn't expose)
    -> replay Playbook.steps in order, substituting Playbook.slots_resolved entries
       for the entity slot tokens. If a step's slot is unresolved, fall back to
       discovery for that step only.
    -> the Playbook's expected_tool_calls is a budget; if you find yourself running
       materially more, record the divergence via `crocoblock-pp-cli playbook amend`
       at end-of-session.

elif Notes present (no Playbook):
    -> read Notes verbatim before any discovery step; they carry known gotchas
       for this query family even when no structured choreography exists yet.

elif Found AND Results[0].EntityMatch == "exact" AND Results[0].Confidence >= 2:
    -> skip discovery; fetch live data for Results[*].ResourceID in parallel

elif Found AND Results[0].EntityMatch == "partial":
    -> candidate hint, NOT a hit; read the resource title to validate before trusting

elif (any row in Mismatches[] when --debug-mismatches was passed):
    -> treat as cold start; the stored learning is for a different entity
       (different canonical resolved from query_entities)

else:  // Found == false, no playbook, no notes
    -> cold start; run discovery normally; teach the answer afterward (Step 4).
       If the family has no playbook yet, that teach auto-synthesizes a
       playbook candidate from this session's journal - you do not need to
       record one by hand.
```

Playbook and Notes are orthogonal to the per-resource path. A recall response can carry both a Playbook AND a `Results[]` hit - use both: the Playbook tells you which choreography to run; the resource hits short-circuit specific steps. Default to skipping `mismatches`; pass `--debug-mismatches` only when investigating cold-start surprises.

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `crocoblock-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `crocoblock-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately. Pass the query the same way as recall — argv/MCP, or file-then-`$QUERY`. Do not splice the question into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
crocoblock-pp-cli teach --query "$QUERY" --resource-type forms --resource 42
# (append shell `&` to background it)
```

The form ID `42` is illustrative; replace it with the resource ID actually resolved for the question. Repeat `--resource` for multiple verified IDs. Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
QUERY=$(cat /path/to/question.txt)
crocoblock-pp-cli teach \
  --query "$QUERY" \
  --resource-type forms \
  --resource 42 \
  --playbook-file ~/playbooks/form-inspection.json \
  --playbook-notes-file ~/playbooks/form-inspection-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
QUERY=$(cat /path/to/question.txt)
crocoblock-pp-cli teach-playbook \
  --query "$QUERY" \
  --playbook-file ~/playbooks/form-inspection.json \
  --notes-file ~/playbooks/form-inspection-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`. Pass the query and note as argv/MCP arguments, or write each with a non-shell file tool and read them back (`QUERY=$(cat ...)`, `NOTE=$(cat ...)`). Do not interpolate either string into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
NOTE=$(cat /path/to/note.txt)
crocoblock-pp-cli playbook amend \
  --query "$QUERY" \
  --add-note "$NOTE"
# (append shell `&` to background it)
```

What counts as worth amending: a behavior you OBSERVED this session that future-you would benefit from knowing. Examples worth amending:

- A workaround for a CLI surface that silently drops or misorders a flag.
- An undocumented endpoint shape (response wrapped in `{meta, results}`, payload nested two levels deeper than the docs claim).
- Observed schema drift (a field renamed, an index that shifted between seasons, a category label that the API now returns lower-cased).

What does NOT belong in notes:

- The year-specific or entity-specific answer to the user's question. That's the response, not a learning.
- Per-team / per-athlete / per-row data the playbook already retrieves at runtime.
- Statements that paraphrase what the existing notes already say.

The amend command appends to the family's existing notes with a timestamped marker (`[amend YYYY-MM-DDTHH:MMZ]: <text>`). Multiple amends accumulate; the audit trail is visible. If no playbook exists yet for the family, amend creates a notes-only one (so cold-start corrections still land).

#### PII discipline for amend notes

`playbook amend` notes are designed to potentially flow upstream as shared knowledge in future versions of the Printing Press. Keep them clean of user-identifying content so the upstream-contribution path stays open without retroactive scrubbing:

- **Do NOT embed** paths to user filesystems, personal API keys or tokens, user email addresses, user GitHub handles, or specific query histories tied to a single user.
- **Acceptable**: endpoint shapes, undocumented field names, API gotchas, observed schema drift, workarounds for CLI surfaces, generalizable pagination or retry tactics.

If a correction is only meaningful with user-specific context, it belongs in a personal note, not in the playbook amend.

### Measuring the loop

`crocoblock-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `CROCOBLOCK_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
crocoblock-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
crocoblock-pp-cli feedback --stdin < notes.txt
crocoblock-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `CROCOBLOCK_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `CROCOBLOCK_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` as a private 0600 file (exclusive randomly named temp in the destination directory, then rename; a planted symlink is not followed). Binary-response commands write decoded payload bytes (not the base64 JSON envelope) and print a small JSON receipt on stdout; `--json`/`--csv` do not refuse when this sink is set. |
| `webhook:<url>` | POST the output body to the URL (`application/json`). Redirects are not followed; a 3xx response fails the delivery and the body is not sent to the `Location`. |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures, including redirects, return non-zero and log the URL and HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

```
crocoblock-pp-cli profile save briefing --json
crocoblock-pp-cli --profile briefing appointments list
crocoblock-pp-cli profile list --json
crocoblock-pp-cli profile show briefing
crocoblock-pp-cli profile delete briefing --yes
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
| 6 | Partial failure |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `crocoblock-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP server from the catalog or build `./cmd/crocoblock-pp-mcp` from this module. Supply all three WordPress environment variables to the MCP host; the Crocoblock browser session is not runtime authentication.

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/developer-tools/crocoblock/cmd/crocoblock-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add crocoblock-pp-mcp -- crocoblock-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which crocoblock-pp-cli`
   If not found, use **Prerequisites: Install the CLI**, or **Build from source** when operating from a source checkout.
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   crocoblock-pp-cli discover --agent
   ```
   This example inspects site capabilities; choose the concrete command and arguments that match the request.
4. If ambiguous, drill into the relevant subcommand help, for example `crocoblock-pp-cli forms --help`.
