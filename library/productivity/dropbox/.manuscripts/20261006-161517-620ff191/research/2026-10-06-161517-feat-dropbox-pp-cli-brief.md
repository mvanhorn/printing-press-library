# Dropbox CLI Brief

## API Identity
- Domain: Dropbox HTTP API v2 (files, sharing, users, file_requests). Official, stable, OAuth2 PKCE. Hosts: api.dropboxapi.com (RPC), content.dropboxapi.com (upload/download, args in `Dropbox-API-Arg` header), notify.dropboxapi.com (longpoll, no auth header).
- Users:
  - Long-time personal account holders (10+ years) whose Dropbox accumulated duplicates, "conflicted copy" files, Camera Uploads sprawl, loose files at root, and stale public shared links. They want an agent to clean up and reorganize safely. (Forum evidence: Duplicate File Cleaner idea thread; thousands-of-conflicted-copies threads; Camera Uploads move threads.)
  - Freelancers/consultants with years of per-client shared folders who need to reorganize without breaking shares (forum: "Organizing shared folders").
  - Photographers dumping phones into Camera Uploads who want year/month organization (forum Photographers board).
  - Privacy-minded users auditing years of "anyone with the link" shares; there is no bulk revoke in the UI (forum idea "Delete shared links in bulk"; users paste browser-console JS).
- Data profile: hierarchical metadata, 100k-1M+ entries for old accounts. Every FileMetadata carries `content_hash` (Dropbox 4MB-block SHA-256), `size`, `client_modified`, `server_modified`, `rev`, `id`. Paths are case-insensitive; root is "".

## Reachability Risk
- None. Official, documented, actively maintained API (Stone spec pushed 2026-10-05; Python SDK 12.2.2 2026-09-22; JS SDK 10.47.0 2026-09-23). No 2025-2026 issues report blocking. Long-lived tokens ended 2021-09-30: must use short-lived access tokens (4h) + offline refresh token. Python SDK #518: token refresh needs its own 5xx retry.
- Probe-safe endpoint: `POST /2/check/user` (echo) and `POST /2/users/get_current_account`.

## Top Workflows
1. **First look**: "what is in my Dropbox and where is the space going" (space usage + per-folder/type/year breakdown).
2. **Dedupe**: find byte-identical files by `content_hash` without downloading, pick keepers, soft-delete the rest. Dropbox's own Find duplicates is web-only, no API.
3. **Conflicted-copy cleanup**: pair "(X's conflicted copy YYYY-MM-DD)" files with originals; delete identical ones, flag different ones. Web UI caps bulk delete at 1000 selections.
4. **Reorganize**: rule-based or agent-authored bulk moves (Downloads -> Documents/{year}, Camera Uploads -> Photos/{year}/{month}), via `move_batch_v2` to avoid `too_many_write_operations` lock contention, with an undo path.
5. **Shared-link hygiene**: list every link, flag public/no-expiry/dangling, bulk revoke.

## Table Stakes
- ls/list folder, metadata, search (server-side search_v2), upload, download, mv, cp, rm (soft), mkdir, revisions + restore, shared link create/list/revoke, space usage, account info, file requests list. (dbxcli, Dropbox official files MCP, Dropbox-Uploader, ngs/dropbox-mcp-server, amgadabdelhafez/dbx-mcp-server.)
- JSON output and refresh-token OAuth are now expected (dbxcli has both).

## Data Layer
- Primary entities: `files` (files + folders, keyed by `path_lower`, with id/rev/size/content_hash/client_modified/server_modified/parent), `shared_links`, `journal_batches` + `journal_ops` (applied changes, for undo), `index_state` (per-root cursor).
- Sync cursor: `files/list_folder` (recursive, limit 2000) -> `list_folder/continue` with the cursor in the POST body; persist cursor per top-level root; incremental runs use `continue` on the saved cursor (deltas incl. `deleted` tags); 409 `reset` => full re-crawl of that root. Generated sync cannot follow a POST-body cursor on a sibling continue route, so `index` is hand-coded.
- FTS/search: FTS5 over `name` and `path_display` so framework `search` works on paths.

## Codebase Intelligence
- Source: Dropbox Stone spec (github.com/dropbox/dropbox-api-spec @ 2994fb74, cloned to research/stone/). Route attrs give host/style/auth/scope mechanically. Deprecated routes to avoid: copy, move, delete, create_folder, move_batch, copy_batch, search v1, properties/*, alpha/*.
- Auth: OAuth2 authorization code + PKCE (S256), `token_access_type=offline` query param for refresh token; token endpoint https://api.dropboxapi.com/oauth2/token; authorize https://www.dropbox.com/oauth2/authorize. No client secret needed with PKCE. Bearer header.
- Errors: 409 endpoint errors with `error_summary` (match by prefix, trailing dots vary); 429 `too_many_requests` with Retry-After; 429 `too_many_write_operations` = namespace lock contention (serialize writes per namespace, use batch routes); 401 re-auth; 422 bad Path-Root.
- Batches: move_batch_v2/copy_batch_v2/delete_batch (<=1000 entries) return `async_job_id`, poll `*/check(_v2)`; status is a `.tag` union (in_progress/complete/failed) with per-entry results. The generator's async detection keys on job_id/status fields and will not detect this; batch polling is hand-coded.
- `Dropbox-API-Arg` header must escape non-ASCII as \uXXXX.
- Business accounts: team folders need `Dropbox-API-Path-Root: {".tag":"root","root":"<root_namespace_id>"}` from get_current_account.

## User Vision
Approved plan: a local plan document (not published). Summary:
- Thesis: the agent decides where things go; the CLI makes it cheap to look and safe to change.
- Look (local store): `index`, `overview`, `tree`, `dupes`, `conflicts`, `mess`, `cold`, `links audit`, `trash`.
- Change: every change is a plan file -> `plan check` -> dry-run `apply` -> `apply --yes`, batched, journaled, reversible with `undo <batch-id>`; `journal` lists batches. Plan builders: `dupes --plan`, `conflicts --plan`, `mess --plan`, `organize --match --under --to '{year}'`.
- Soft deletes approved by the user. Never permanently delete (no permanently_delete endpoint at all).
- Account type unknown (user thinks personal): detect via users/get_current_account; restore window 30 days (Basic/Plus) vs 180+ (Professional/Business) drives undo warnings; Business gets Path-Root header.
- Safety: refuse apply when source rev changed; per-run op cap; cross-shared-folder moves flagged and require explicit opt-in.
- User has created a Dropbox app (Full Dropbox, scoped) and provided the app key; bring-your-own app key until publish.

## Product Thesis
- Name: dropbox-pp-cli ("Dropbox")
- Why it should exist: dbxcli (revived 2026-09, agent JSON) and the official Dropbox MCP cover per-file CRUD. Nobody covers organizing at scale: hash dedupe without downloads, folder usage reports, conflicted-copy cleanup, bulk shared-link audit/revoke, and batched moves with lock-contention retry, all from a local index, behind plan/dry-run/apply/undo safety that makes it reasonable to let an agent loose on a decade of files.

## Build Priorities
1. Spec + auth: internal YAML from Stone for the v1 endpoint set; OAuth PKCE with offline refresh; 409/429 error handling; Dropbox-API-Arg escaping for download/upload.
2. `index` into SQLite (resumable, incremental), then the read-only analyses (`overview`, `tree`, `dupes`, `conflicts`, `mess`, `cold`, `trash`).
3. Plan/check/apply/undo/journal with batch polling and journaling.
4. `links audit` + bulk revoke via plan.

## Reachability Gate
- Decision: PASS
- Base check: unauthenticated `POST https://api.dropboxapi.com/2/check/user` returned 400 (missing auth header), the expected auth-required response; no bot protection.
- OAuth grant probe: authorize URL with the user's real app key, PKCE S256, `token_access_type=offline`, redirect `http://127.0.0.1:8085/callback` returned 200 on www.dropbox.com with no error page; a fake client_id returned Dropbox's error page (negative control). Redirect URI is not validated before login (an unregistered port also loaded), so redirect correctness is confirmed at the first live `auth login`.
