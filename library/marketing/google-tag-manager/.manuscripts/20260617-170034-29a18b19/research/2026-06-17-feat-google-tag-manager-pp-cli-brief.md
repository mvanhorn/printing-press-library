# Google Tag Manager CLI Brief

## API Identity
- Domain: Tag management / martech measurement infrastructure. **Google Tag Manager API v2** (`tagmanager.googleapis.com`), official, OpenAPI via apis-guru (`googleapis.com/tagmanager/v2`).
- Users: Marketing/analytics engineers, agencies, and ops teams running web/app measurement across one or many GTM containers.
- Data profile: Hierarchical — Account → Container → Workspace → {Tags, Triggers, Variables, Built-in Variables, Folders, Templates, Clients, Zones, GTAG Config} → Versions → Environments. Plus `user_permissions`, `destinations`.
- **Scope (this build): READ-ONLY.** 22 GET endpoints. No create/update/delete/publish. Enforced at the spec level — the 28 POST / 1 PUT / 1 DELETE operations were stripped before generation, so the binary *cannot* mutate a container.

## Reachability Risk
- **None.** Live probe `GET /tagmanager/v2/accounts` → HTTP 401 "Request is missing required authentication credential. Expected OAuth 2 access token." Textbook auth-required PASS; the API is live and reachable.
- Auth: OAuth2, read scope `https://www.googleapis.com/auth/tagmanager.readonly`. **No API key exists for GTM.** Modeled as bearer token (`Authorization: Bearer <token>`), env `GTM_ACCESS_TOKEN`. Token minted out-of-band:
  - `gcloud auth application-default login --scopes=https://www.googleapis.com/auth/tagmanager.readonly` then `gcloud auth application-default print-access-token`
  - or a service-account JSON whose SA email is granted Viewer access in GTM admin.

## Top Workflows
1. **Audit** a container's hygiene — dead tags, unused variables, orphan triggers, tags missing consent settings.
2. **Diff** — workspace vs live, version vs version, container vs container.
3. **Export** container config to version control (git-friendly JSON).
4. **Graph queries** — what fires on trigger X; which triggers/variables a tag depends on.
5. **Search** all config for an ID/URL/string (hardcoded GA4 IDs, stray pixels, leftover URLs).

## Table Stakes (parity with API/SDK + ecosystem)
- Full read coverage of the API: list accounts/containers/workspaces and every workspace child resource; version headers + live version; environments; user permissions; container lookup + snippet.
- A container-export equivalent (what community export-parsing tools assume as input).
- `--json` / `--select` / `--csv` / offline SQLite store / typed exit codes / agent-native output.

## Data Layer
- Primary entities: account, container, workspace, tag, trigger, variable, builtInVariable, folder, template, client, zone, gtagConfig, versionHeader, liveVersion, environment, userPermission.
- Sync cursor: snapshot-based. `pull` walks a workspace or the live version into a named local snapshot; re-pull to refresh.
- FTS/search: FTS5 over flattened entity JSON (name, type, parameters, notes, fingerprint).

## Source Priority
- Single source (official Google API). N/A — no combo ordering.

## Codebase Intelligence
- Official client parity: `google.golang.org/api/tagmanager/v2` exposes the List/Get methods that equal our typed command surface.
- Hierarchical composite resource names (`accounts/X/containers/Y/workspaces/Z`) are awkward as raw flags; the transcendence `pull` command takes `--account/--container/--workspace` and assembles the parent path, then everything downstream queries the local store by friendly id.
- apis-guru collapses all individual-resource `{path}` GETs onto one slot, so only `.list` endpoints survive as distinct typed commands. Individual-resource fetches (e.g. a specific historical version) are reached by hand-coded transcendence commands via the generic client.

## Product Thesis
- Name: `google-tag-manager-pp-cli` (display: **Google Tag Manager**)
- Why it should exist: The GTM console can't diff, can't bulk-query, can't export to version control, and can't cross-reference the tag↔trigger↔variable graph. Mirror a container into local SQLite and all of that becomes one command. **Read-only by construction** — safe to point at a production container.

## Build Priorities
1. **P0 foundation** — data layer for all entities + `pull` (walk a container into SQLite).
2. **P1 absorb** — all 22 typed GET endpoint commands + container export.
3. **P2 transcend** — audit, diff, fires (graph), search, consent-report, export.
