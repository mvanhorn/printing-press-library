# Etherpad CLI Brief

## API Identity
- **Domain:** Real-time collaborative document editing — pads, groups, authors, sessions, per-pad chat
- **Users:** Self-hosted Etherpad operators, application integrators embedding Etherpad, AI agents driving pads programmatically
- **Data profile:** Pads (text + revision history + attribute pool + chat) × authors × groups. Live collab is via socket.io (out of scope here); HTTP API is the management/RPC surface

## Reachability Risk
- **LOW** — Etherpad is Apache-2.0, self-hosted, established (15+ year project). HTTP API documented at https://etherpad.org and served from every instance at `/api/openapi.json`. The cleaned spec (post ether/etherpad#7714) is publicly verifiable at https://pad-dev.etherpad.org/api/openapi.json — anyone can `curl` it to confirm the surface this CLI was generated against. Auth is `apikey` (query, header, or `Authorization: Bearer`); rate limits are operator-configured.

## Top Workflows
1. **Bulk pad export / instance backup** — sysadmins archiving an entire instance
2. **Cross-pad search and analytics** — "pads not edited in N days", "count pads per author"
3. **Migration in/out** — JSONL → pads, pads → markdown for git, instance-to-instance moves
4. **Agent-driven scripted edits** — LLMs creating/updating/querying pads programmatically (where ep_ai_mcp's in-pad MCP isn't applicable)
5. **Continuous polling for change** — agents watching pads without socket.io (Etherpad has no webhooks)

## Table Stakes
- Pad CRUD (create, get text/html, set text/html, delete)
- Group/author/session lifecycle
- Authorship enumeration; per-pad chat
- Revision operations (count, last-edited, save/restore/list saved)
- HTML diff between revisions
- Read-only ID resolution; public-status toggle
- Server-wide stats; token check
- JSON output mode

## Data Layer
- **Primary entities:** pads, authors, groups, sessions, pad-revisions, pad-chats
- **Sync cursor:** `getLastEdited(padID)` per pad; no global "since" — local store remembers per-pad timestamps for incremental sync
- **FTS/search:** FTS5 on pad text + chat (post-sync)
- **Relations:** authors ↔ pads many-to-many via attribute pool; groups → pads one-to-many; sessions = (group, author) tuples with TTL
- **Key tables:** `pads`, `authors`, `groups`, `sessions`, `pad_authors`, `pad_chats`, `pad_revisions`

## Codebase Intelligence
- **Source:** [ether/etherpad](https://github.com/ether/etherpad) (TypeScript/Node.js, Apache-2.0)
- **Auth:** API key from `APIKEY.txt` on the server; passed as `apikey=` query, `apikey:` header, or `Authorization: Bearer`. OAuth2/OIDC also supported when `authenticationMethod=sso`.
- **Data model:** Pads are revision-keyed text + attribute pool (rich-text annotations as integer references); changesets are operational-transform deltas. HTTP API exposes high-level state but not the OT layer (that's socket.io). Authors are first-class identities decoupled from authentication. Groups multiplex pads for multi-tenant deployments. Sessions tie an author to a group with `validUntil`.
- **Spec source:** Live `https://pad-dev.etherpad.org/api/openapi.json` (Etherpad's public develop deploy). Post #7714 the spec is OpenAPI 3.0.2, top-level tagged (`pad/author/session/group/chat/server`), 50 operations, POST-only (runtime keeps GET+POST for back-compat).
