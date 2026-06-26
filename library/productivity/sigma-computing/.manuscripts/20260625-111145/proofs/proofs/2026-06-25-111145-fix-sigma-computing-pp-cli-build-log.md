# Sigma Computing CLI — Build Log

## What was built
- **Generator (Priority 0/1):** Full data layer + sync/search/SQL + 252 absorbed operations across 27 interfaces, reachable via `api <interface>` browse (workbooks/members/teams/connections/dataModels/grants/workspaces/tenants/user-attributes/etc.). OAuth2 client-credentials auth fully wired (config + auth cmd + MCP code_orch), env vars SIGMA_COMPUTING_CLIENT_ID/_SECRET/_BASE_URL, token refresh. MCP Cloudflare pattern (stdio+http transport, code orchestration, hidden endpoint tools) — ~270 tools collapsed to a thin search+execute pair.
- **Hand-built (Priority 2):** All 7 transcendence features implemented fully (no stubs) in their scaffold files + `internal/cli/novel_shared.go`:
  1. `grant audit <type> <id>` — store join grants→members + team expansion (offline via teams_members join table)
  2. `access review <email>` — reverse join member→teams→grants→resources
  3. `workbook stale --days N` — store threshold on updated_at, joined to owner email
  4. `workbook copy <id> --to <member>` — POST copy + PATCH /v2/files/{inode} ownerId reassign (bug fix bundled)
  5. `member offboard <email> --transfer-to <member>` — list owned files + reassign + PATCH member isArchived
  6. `member provision --from <csv>` — idempotent createMember + team-assign + user-attribute, CSV-driven
  7. `export bulk --query <fts> --format` — offline search resolves set, loops POST export

## Verified API field names (from spec, not guessed)
- createMember: email/firstName/lastName/memberType
- deactivate: PATCH /v2/members/{id} isArchived:true
- copy: {name, destinationFolderId}; response id = workbookId
- ownership: PATCH /v2/files/{inodeId} {ownerId}
- team assign: PATCH /v2/teams/{teamId}/members {add:[memberId]}
- user-attr: POST /v2/user-attributes/{id}/users {assignments:[...]}
- export: POST /v2/workbooks/{id}/export {format:{type}}; pdf adds {layout}

## Tests
14 new table-driven tests across the 7 features + novel_shared. go build / go vet / go test ./internal/cli/... all green.

## Deferred / notes
- Phase 5 live smoke skipped (no credentials provided this run).
- Env-var prefix is slug-derived SIGMA_COMPUTING_* (not community SIGMA_*); auth login is primary path. Aliasing to SIGMA_* is a possible polish item, not a blocker.
- Generator warnings (benign): oneOf/anyOf body fallbacks on a few POST/PATCH; auth resource renamed to sigma-computing-public-auth to avoid shadowing framework `auth`.

## Phase 3 Completion Gate: PASS
- Per-row Cobra resolution: all 7 leaves resolve (exit 0, leaf in Usage).
- Deterministic backstop: dogfood novel_features_check planned=7 found=7, missing=[].
- Test presence: all 7 _test.go files present with real assertions.
